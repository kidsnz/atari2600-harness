// framegen — reproduce a target ROM's static visible frame FROM SCRATCH as a
// new, self-contained DASM source. It renders the target, reads (per visible
// scanline) which TIA object drew each pixel (`emu.DecomposeRow`), re-encodes
// the playfield into left/right PF register bytes and the two players into
// GRP0/GRP1 bytes, reads the colours/NUSIZ/positions, and emits a data-driven
// per-scanline replay kernel. It then self-calibrates the two sprite X positions
// by assembling + rendering its own output and nudging the position inputs until
// the players land on the target's columns.
//
//	go run ./cmd/framegen -rom Outlaw.bin -reset -frames 28 -out clone/outlaw_clone.asm
//
// Verify the result with: vismatch -target Outlaw.bin -mine clone/outlaw_clone.asm
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// nusizWidth maps NUSIZ size-and-copies bits to pixels-per-GRP-bit (1/2/4).
//
// The five COPY modes (1,2,3,4,6) are 1 pixel per bit — the copies are hardware
// replication of the SAME 8-bit byte, not a wider byte — so returning 1 for them
// is correct, and it is measured rather than assumed: with NUSIZ held constant,
// `DecomposeRow` on roms/litmus/litmus_nusiz_all's per-mode probe reports run
// LENGTHS of 8 for modes 0,1,2,3,4,6, 16 for mode 5 and 32 for mode 7 on a $FF
// byte. framegen reproduces all eight constant modes pixel-exact on a P0-only
// probe (864/864, 1728/1728, 1728/1728, 2592/2592, 1728/1728, 1728/1728,
// 2592/2592, 3456/3456 P0 cells matched for modes 0..7) — so a wrong copy count
// in a clone is NOT this function.
func nusizWidth(sc uint8) int {
	switch sc & 0x07 {
	case 0x05:
		return 2
	case 0x07:
		return 4
	}
	return 1
}

// nusizCopies maps NUSIZ size-and-copies bits to the number of copies the TIA
// draws. Measured, not tabulated from a document: litmus_nusiz_all holds $FF in
// both players and steps NUSIZ 0..7 in 24-line bands, and `DecomposeRow` counts
// P0 runs per band at these start clocks —
//
//	0: 24 | 1: 24,40 | 2: 24,56 | 3: 24,40,56 | 4: 24,88 | 5: 25 | 6: 24,56,88 | 7: 25
//
// i.e. 1,2,2,3,2,1,3,1 copies for modes 0..7.
func nusizCopies(sc uint8) int {
	switch sc & 0x07 {
	case 0x01, 0x02, 0x04:
		return 2
	case 0x03, 0x06:
		return 3
	}
	return 1
}

// nusizStartShift is how far right the TIA starts drawing a player when the size
// bits widen it. Measured on eight otherwise-identical probe ROMs whose only
// difference is the NUSIZ value: `Markers()[0].Clock` (HmovedPixel) reads 24 for
// modes 0,1,2,3,4,6 and 25 for modes 5 and 7, and the first drawn pixel agrees
// with it in all eight. So the shift is +1 for the two size modes and 0 for the
// rest, and HmovedPixel already carries it — a per-line X read from the machine
// needs no correction, but comparing two lines' positions does.
func nusizStartShift(sc uint8) int {
	if nusizWidth(sc) > 1 {
		return 1
	}
	return 0
}

// lineState is the per-scanline TIA state a per-scanline replay kernel could
// carry for the two players: NUSIZ (size+copies) and the reset position.
//
// framegen used to read both ONCE, at the end of the rendered frame, and apply
// that single value to every scanline. On a target that changes NUSIZ down the
// frame that is not an approximation, it is the wrong picture: litmus_nusiz_all
// ends its frame in mode $07 (quad width), so every one of its 214 lines was
// reproduced quad-width and 2666 cells came out wrong.
type lineState struct {
	nz0, nz1 uint8
	x0, x1   int
}

// measureLines steps ONE whole frame instruction by instruction and records, for
// every scanline, the NUSIZ and reset position each player ENDS that scanline
// with. A scanline with no instruction boundary of its own inherits the previous
// line's state, which is what the hardware does too.
//
// End of line, not end of HBLANK, and that is a measurement rather than a
// preference. The engine applies a NUSIZ write through a delayed event, so the
// register READBACK lags the store: traced on litmus_nusiz_all scanline 37, the
// band header's `sta NUSIZ1` completes at clock -41 and `Player1.SizeAndCopies`
// still reads the previous band's $07 at every instruction boundary through clock
// -2, only reading $00 at clock +7 — while the picture on that line shows P1 drawn
// in the NEW mode ($00: one copy, 8 pixels wide, starting at clock 3, not $07's
// quad width starting at 4). A cutoff at the end of HBLANK therefore reads a value
// the scanline never used; it cost 2 of litmus_nusiz_all's 214 lines (37 and 181,
// the two whose band header straddles the boundary) and 18 P1 cells.
//
// The cost of the late sample is stated rather than hidden: a target that changes
// NUSIZ part-way ALONG a scanline has that change attributed to the whole line.
// This kernel carries one NUSIZ per scanline, so it could not reproduce such a
// line under any sampling rule; the difference surfaces as unexplained cells in
// the final report instead of being quietly absorbed.
func measureLines(e *emu.Emu) (map[int]lineState, error) {
	regs := e.ReadTIARegisters()
	mk := e.Markers()
	cur := lineState{regs.Player0.SizeAndCopies, regs.Player1.SizeAndCopies, mk[0].Clock, mk[2].Clock}
	out := map[int]lineState{}
	start := e.Coords().Frame
	maxSl := 0
	for {
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		c := e.Coords()
		if c.Frame != start {
			break
		}
		if c.Scanline > maxSl {
			maxSl = c.Scanline
		}
		regs := e.ReadTIARegisters()
		mk := e.Markers()
		cur = lineState{regs.Player0.SizeAndCopies, regs.Player1.SizeAndCopies, mk[0].Clock, mk[2].Clock}
		out[c.Scanline] = cur
	}
	// Fill the gaps forward: a scanline whose HBLANK held no instruction boundary
	// still ran with whatever the previous line left in the registers.
	var last lineState
	seeded := false
	for s := 0; s <= maxSl; s++ {
		if v, ok := out[s]; ok {
			last, seeded = v, true
			continue
		}
		if seeded {
			out[s] = last
		}
	}
	return out, nil
}

// pfBytes encodes 20 playfield columns (each 4 clocks) into PF0/PF1/PF2 with the
// hardware bit order (PF0 D4-D7 = cols 0-3; PF1 D7-D0 = cols 4-11; PF2 D0-D7 =
// cols 12-19). lit[c] = column c (0..19) is playfield.
func pfBytes(lit [20]bool) (pf0, pf1, pf2 uint8) {
	for i := 0; i < 4; i++ {
		if lit[i] {
			pf0 |= 1 << (4 + i)
		}
	}
	for i := 0; i < 8; i++ {
		if lit[4+i] {
			pf1 |= 1 << (7 - i)
		}
	}
	for i := 0; i < 8; i++ {
		if lit[12+i] {
			pf2 |= 1 << i
		}
	}
	return
}

type frameData struct {
	top, h           int
	p0col, p1col     uint8
	pfcol, bgcol     uint8
	nusiz0, nusiz1   uint8 // end-of-frame NUSIZ (what the clone restores after its kernel)
	p0x, p1x         int
	pf0L, pf1L, pf2L []uint8
	pf0R, pf1R, pf2R []uint8
	grp0, grp1       []uint8
	nz0, nz1         []uint8     // per-visible-line NUSIZ, measured (see measureLines)
	tgtElem          [][]string  // target's per-visible-line object attribution (for content-shift search)
	// obj holds the measured facts for all five movable objects, indexed by
	// objIndex: P0, P1, M0, M1, BL. The missiles and the ball were absent from
	// this array for as long as the kernel could not draw them, which is exactly
	// backwards — a tool has to measure what it cannot yet reproduce, or it cannot
	// say what it is missing.
	obj [5]objFacts
	// en0/en1/enb are per-visible-line ENAM0/ENAM1/ENABL bytes ($02 on, $00 off).
	en0, en1, enb []uint8
	// msz0/msz1/bsz are the measured missile and ball widths (NUSIZ bits 4-5 and
	// CTRLPF bits 4-5), taken from the run lengths the machine actually drew.
	msz0, msz1, bsz uint8
	kern []writeBlock
}

// objIndex names the five movable objects in the order fd.obj holds them, and in
// the order SetXPos's X register selects them: RESP0=$10, RESP1=$11, RESM0=$12,
// RESM1=$13, RESBL=$14 are consecutive, and so are HMP0..HMBL at $20..$24, which
// is why one routine can place any of them.
const (
	objP0 = iota
	objP1
	objM0
	objM1
	objBL
)

var objNames = [5]string{"P0", "P1", "M0", "M1", "BL"}

// objFacts is what the machine says one player DOES over the target's frame, as
// opposed to what a single end-of-frame register read says it is. Every field is
// counted, never inferred, because the report built on it used to assert a cause
// ("a per-zone multiplexed target") that it had not measured — and that was wrong
// for litmus_nusiz_all, whose two players are placed once before the frame loop
// and never moved.
type objFacts struct {
	name      string
	lines     int           // visible lines on which the target draws this element at all
	maxRuns   int           // most separate runs of it seen on one line
	modes     map[uint8]int // per-line NUSIZ value -> number of visible lines with it
	maxCopies int           // most copies any of those modes asks the TIA for
	baseX     map[int]int   // reset X with the NUSIZ size shift removed -> line count
	distinctX int           // len(baseX): >1 is the only thing that means multiplexing
	onlyX     int           // the single base X, meaningful only when distinctX == 1
	// widths counts the run lengths the object was drawn at. For a missile or the
	// ball that IS its size register (1/2/4/8 colour clocks); more than one entry
	// means the width changes down the frame, which one register write per frame
	// cannot reproduce.
	widths map[int]int
}

func newObjFacts(name string) objFacts {
	return objFacts{name: name, modes: map[uint8]int{}, baseX: map[int]int{}, widths: map[int]int{}}
}

// modeList renders the measured NUSIZ modes with the line count each ran for, so
// "up to 3 copies" is always accompanied by the evidence for it.
// widthList renders the measured run widths with the line count for each, so
// "8 clocks wide" always arrives with the evidence.
func (f objFacts) widthList() string {
	if len(f.widths) == 0 {
		return "(none)"
	}
	var ks []int
	for k := range f.widths {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	parts := make([]string, 0, len(ks))
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%d(%d lines)", k, f.widths[k]))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func (f objFacts) modeList() string {
	if len(f.modes) == 0 {
		return "(never drawn)"
	}
	keys := make([]int, 0, len(f.modes))
	for k := range f.modes {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("$%02X(%dx%d copies on %d lines)",
			k, nusizWidth(uint8(k)), nusizCopies(uint8(k)), f.modes[uint8(k)]))
	}
	return strings.Join(parts, " ")
}

// xList renders the measured reset positions, size shift removed.
func (f objFacts) xList() string {
	if len(f.baseX) == 0 {
		return "[]"
	}
	keys := make([]int, 0, len(f.baseX))
	for k := range f.baseX {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d(%d lines)", k, f.baseX[k]))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// shiftUint8 returns a copy of a shifted by s (out[i]=a[i+s], edges clamp to 0).
func shiftUint8(a []uint8, s int) []uint8 {
	out := make([]uint8, len(a))
	for i := range out {
		j := i + s
		if j >= 0 && j < len(a) {
			out[i] = a[j]
		}
	}
	return out
}

// shifted returns a copy of fd with all replay tables shifted vertically by s.
func (fd *frameData) shifted(s int) *frameData {
	c := *fd
	c.pf0L = shiftUint8(fd.pf0L, s)
	c.pf1L = shiftUint8(fd.pf1L, s)
	c.pf2L = shiftUint8(fd.pf2L, s)
	c.pf0R = shiftUint8(fd.pf0R, s)
	c.pf1R = shiftUint8(fd.pf1R, s)
	c.pf2R = shiftUint8(fd.pf2R, s)
	c.grp0 = shiftUint8(fd.grp0, s)
	c.grp1 = shiftUint8(fd.grp1, s)
	// The enable tables are picture data too. Leaving them behind shifts the
	// playfield and players while the missiles and ball stay put: measured on
	// motion_glide, the ball came out drawn on the wrong 4 scanlines, 8 cells wrong
	// where it had simply been absent before.
	c.en0 = shiftUint8(fd.en0, s)
	c.en1 = shiftUint8(fd.en1, s)
	c.enb = shiftUint8(fd.enb, s)
	// The NUSIZ tables are per-scanline replay data like the rest: shifting the
	// picture without them would move a band's graphics off its own copy mode.
	c.nz0 = shiftKeepEdges(fd.nz0, s)
	c.nz1 = shiftKeepEdges(fd.nz1, s)
	return &c
}

// shiftKeepEdges shifts like shiftUint8 but clamps to the first/last element
// instead of 0. NUSIZ has no "off" value — shifting a zero into the edge would
// silently order one copy where the target asks for three.
func shiftKeepEdges(a []uint8, s int) []uint8 {
	out := make([]uint8, len(a))
	if len(a) == 0 {
		return out
	}
	for i := range out {
		j := i + s
		if j < 0 {
			j = 0
		}
		if j >= len(a) {
			j = len(a) - 1
		}
		out[i] = a[j]
	}
	return out
}

// extract renders the target and pulls every per-scanline replay byte.
func extract(rom, spec string, frames int, reset bool) (*frameData, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(rom); err != nil {
		return nil, err
	}
	if reset {
		_ = e.SetPanel("reset", true)
		if err := e.RunFrames(8); err != nil {
			return nil, err
		}
		_ = e.SetPanel("reset", false)
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	img, top := e.Snapshot()
	h := img.Bounds().Dy()
	regs := e.ReadTIARegisters()
	mk := e.Markers()
	fd := &frameData{
		top: top, h: h,
		p0col: regs.Player0.Color, p1col: regs.Player1.Color,
		pfcol: regs.Playfield.ForegroundColor, bgcol: regs.Playfield.BackgroundColor,
		nusiz0: regs.Player0.SizeAndCopies, nusiz1: regs.Player1.SizeAndCopies,
		p0x: mk[0].Clock, p1x: mk[2].Clock,
	}
	// Pass 1: object attribution for every visible line. It has to run before the
	// state pass, which steps a further frame and overwrites the element buffer.
	runsPerLine := make([][]emu.ElemRun, h)
	for y := 0; y < h; y++ {
		sl := top + y
		elem := make([]string, 160)
		for i := range elem {
			elem[i] = "BG"
		}
		if runs, _, err := e.DecomposeRow(sl); err == nil {
			runsPerLine[y] = runs
			for _, r := range runs {
				for x := r.Clock; x < r.Clock+r.Len && x < 160; x++ {
					elem[x] = r.Element
				}
			}
		}
		var litL, litR [20]bool
		for c := 0; c < 20; c++ {
			litL[c] = elem[4*c] == "PF"
			litR[c] = elem[80+4*c] == "PF"
		}
		l0, l1, l2 := pfBytes(litL)
		r0, r1, r2 := pfBytes(litR)
		fd.pf0L = append(fd.pf0L, l0)
		fd.pf1L = append(fd.pf1L, l1)
		fd.pf2L = append(fd.pf2L, l2)
		fd.pf0R = append(fd.pf0R, r0)
		fd.pf1R = append(fd.pf1R, r1)
		fd.pf2R = append(fd.pf2R, r2)
		fd.tgtElem = append(fd.tgtElem, elem)
	}

	// Pass 2: per-scanline NUSIZ and reset position, measured off the next frame
	// (framegen reproduces a STATIC frame, so the next one repeats this one).
	st, err := measureLines(e)
	if err != nil {
		return nil, err
	}
	for i := range fd.obj {
		fd.obj[i] = newObjFacts(objNames[i])
	}
	for y := 0; y < h; y++ {
		ls, ok := st[top+y]
		if !ok {
			ls = lineState{fd.nusiz0, fd.nusiz1, fd.p0x, fd.p1x}
		}
		fd.nz0 = append(fd.nz0, ls.nz0)
		fd.nz1 = append(fd.nz1, ls.nz1)
		countRuns(&fd.obj[objP0], runsPerLine[y], "P0", ls.nz0, ls.x0)
		countRuns(&fd.obj[objP1], runsPerLine[y], "P1", ls.nz1, ls.x1)
		// Missiles and the ball are measured from the pixels they DREW rather than
		// from a register read: a missile has no graphics byte, so its run in the
		// attribution is its position and its width directly, and reading it that way
		// needs no second measurement pass to disagree with the first. NUSIZ is
		// passed as 0 because the size shift countRuns removes applies to a player's
		// graphics start, not to a missile — a missile's run starts where it starts.
		for i, name := range []string{"M0", "M1", "BL"} {
			if x, w, ok := firstRun(runsPerLine[y], name); ok {
				countRuns(&fd.obj[objM0+i], runsPerLine[y], name, 0, x)
				fd.obj[objM0+i].widths[w]++
			}
		}
		fd.en0 = append(fd.en0, enableByte(runsPerLine[y], "M0"))
		fd.en1 = append(fd.en1, enableByte(runsPerLine[y], "M1"))
		fd.enb = append(fd.enb, enableByte(runsPerLine[y], "BL"))
	}
	for i := range fd.obj {
		fd.obj[i].distinctX = len(fd.obj[i].baseX)
	}
	fd.msz0 = sizeBits(fd.obj[objM0])
	fd.msz1 = sizeBits(fd.obj[objM1])
	fd.bsz = sizeBits(fd.obj[objBL])

	// The graphics byte is sampled at the line's OWN pixel width, from the same
	// end-of-frame reset position as before, corrected only by that line's size
	// shift. It deliberately does NOT sample at the per-line HmovedPixel: this
	// kernel strobes RESPx once, and the position it strobes to is calibrated
	// against the end-of-frame marker, so reading the pattern from anywhere else
	// draws the right shape in the wrong column. Measured cost of getting that
	// wrong: sampling at the per-line position turned flicker_multiplex (0 cells
	// wrong) into 78, sprite_anim (0) into 84 and text24 (162) into P0/P1 absent
	// entirely — the end-of-frame marker and the visible-line position are not the
	// same number on a target that parks its sprites during overscan.
	sh0 := nusizStartShift(fd.nusiz0)
	sh1 := nusizStartShift(fd.nusiz1)
	for y := 0; y < h; y++ {
		x0 := fd.p0x - sh0 + nusizStartShift(fd.nz0[y])
		x1 := fd.p1x - sh1 + nusizStartShift(fd.nz1[y])
		fd.grp0 = append(fd.grp0, grpByte(fd.tgtElem[y], x0, nusizWidth(fd.nz0[y]), "P0"))
		fd.grp1 = append(fd.grp1, grpByte(fd.tgtElem[y], x1, nusizWidth(fd.nz1[y]), "P1"))
	}
	return fd, nil
}

// countRuns folds one visible line into an object's measured facts.
// firstRun returns the clock and length of the leftmost run of the named element
// on a line, or ok=false when it drew nothing there.
func firstRun(runs []emu.ElemRun, name string) (clock, length int, ok bool) {
	best := 1 << 30
	for _, r := range runs {
		if r.Element == name && r.Clock < best {
			best, length, ok = r.Clock, r.Len, true
		}
	}
	return best, length, ok
}

// enableByte is the ENAMx/ENABL value that reproduces this line: bit 1 set when
// the object drew, clear when it did not.
func enableByte(runs []emu.ElemRun, name string) uint8 {
	for _, r := range runs {
		if r.Element == name {
			return 0x02
		}
	}
	return 0
}

// sizeBits turns the measured run width into the register bits that produce it —
// NUSIZ bits 4-5 for a missile, CTRLPF bits 4-5 for the ball, both encoding
// 1/2/4/8 colour clocks as 0/1/2/3 shifted left by four.
//
// The width is taken from what the machine DREW, not from a register read, so a
// target whose missile changes width down the frame shows up here as more than
// one entry rather than as whichever value happened to survive to the last line —
// the same end-of-frame trap that made every NUSIZ mode replay as quad width.
func sizeBits(f objFacts) uint8 {
	w := 0
	for k := range f.widths {
		if k > w {
			w = k
		}
	}
	switch {
	case w >= 8:
		return 0x30
	case w >= 4:
		return 0x20
	case w >= 2:
		return 0x10
	}
	return 0
}

func countRuns(f *objFacts, runs []emu.ElemRun, name string, nz uint8, x int) {
	n := 0
	for _, r := range runs {
		if r.Element == name {
			n++
		}
	}
	if n == 0 {
		return // the object draws nothing here; its position says nothing about the picture
	}
	f.lines++
	if n > f.maxRuns {
		f.maxRuns = n
	}
	f.modes[nz&0x07]++
	if c := nusizCopies(nz); c > f.maxCopies {
		f.maxCopies = c
	}
	// Remove the size-mode start shift before counting positions, so a target that
	// only switches between 1x and 2x is not reported as having moved its sprite.
	b := x - nusizStartShift(nz)
	f.baseX[b]++
	f.onlyX = b
}

// renderGrid assembles src, renders it, and returns the per-visible-line object
// attribution grid + the Snapshot top (aligned to absolute scanline top+y), plus
// the SAME measured per-object facts that are taken off the target. Measuring the
// clone the same way the target is measured is what lets the final report compare
// copy counts and reset positions instead of asserting a cause.
// mk carries the clone's end-of-frame P0/P1 markers alongside the per-object
// facts: the players' calibration target is the marker the target was measured
// with, and a missile's is the reset X its pixels were drawn at. Comparing a
// player against a base X instead would be comparing two different numbers.
func renderGrid(src, spec string, frames int) (grid [][]string, top int, obj [5]objFacts, err error) {
	for i := range obj {
		obj[i] = newObjFacts(objNames[i])
	}
	dir, err := os.MkdirTemp("", "framegen")
	if err != nil {
		return nil, 0, obj, err
	}
	asm, bin := dir+"/c.asm", dir+"/c.bin"
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		return nil, 0, obj, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return nil, 0, obj, fmt.Errorf("assemble:\n%s", out)
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, 0, obj, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, 0, obj, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, 0, obj, err
	}
	img, vt := e.Snapshot()
	h := img.Bounds().Dy()
	runsPerLine := make([][]emu.ElemRun, h)
	for y := 0; y < h; y++ {
		row := make([]string, 160)
		for i := range row {
			row[i] = "BG"
		}
		if runs, _, err := e.DecomposeRow(vt + y); err == nil {
			runsPerLine[y] = runs
			for _, r := range runs {
				for x := r.Clock; x < r.Clock+r.Len && x < 160; x++ {
					row[x] = r.Element
				}
			}
		}
		grid = append(grid, row)
	}
	if st, merr := measureLines(e); merr == nil {
		for y := 0; y < h; y++ {
			ls, ok := st[vt+y]
			if !ok {
				continue
			}
			countRuns(&obj[0], runsPerLine[y], "P0", ls.nz0, ls.x0)
			countRuns(&obj[1], runsPerLine[y], "P1", ls.nz1, ls.x1)
		}
	}
	for y := 0; y < h; y++ {
		for i, name := range []string{"M0", "M1", "BL"} {
			if x, w, ok := firstRun(runsPerLine[y], name); ok {
				countRuns(&obj[objM0+i], runsPerLine[y], name, 0, x)
				obj[objM0+i].widths[w]++
			}
		}
	}
	for i := range obj {
		obj[i].distinctX = len(obj[i].baseX)
	}
	return grid, vt, obj, nil
}

// elemCoverage is one TIA element's share of the reproduction: how many visible
// cells the target draws with it, how many of those the clone draws the same
// way, and whether the clone draws that element ANYWHERE in its frame.
type elemCoverage struct {
	Elem     string
	Target   int // cells the target draws with this element
	Matched  int // of those, cells the clone agrees on
	CloneAny int // cells the clone draws with this element, anywhere
}

// coverage measures the reproduction per element rather than as one number.
//
// A single match percentage hides the failure that matters: this generator's
// kernel emits PF + GRP0/GRP1 and nothing else, so on a target that draws no
// missiles it is pixel-exact, and on a target that draws bullets it silently
// omits every one of them while still scoring high. `CloneAny == 0` with
// `Target > 0` is that case stated as a number — the element is not slightly
// wrong, it is structurally absent.
func coverage(tgt, clone [][]string) []elemCoverage {
	target, matched, cloneAny := map[string]int{}, map[string]int{}, map[string]int{}
	h := len(tgt)
	if len(clone) < h {
		h = len(clone)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < 160 && x < len(tgt[y]) && x < len(clone[y]); x++ {
			target[tgt[y][x]]++
			cloneAny[clone[y][x]]++
			if tgt[y][x] == clone[y][x] {
				matched[tgt[y][x]]++
			}
		}
	}
	// Fixed order so two runs are diffable, and so an element the target never
	// draws is absent rather than reported as a vacuous 0/0 pass.
	var out []elemCoverage
	for _, e := range []string{"BG", "PF", "P0", "P1", "M0", "M1", "BL"} {
		if target[e] == 0 && cloneAny[e] == 0 {
			continue
		}
		out = append(out, elemCoverage{Elem: e, Target: target[e], Matched: matched[e], CloneAny: cloneAny[e]})
	}
	return out
}

// missingElements names the elements the target draws and the clone never does.
func missingElements(cov []elemCoverage) []string {
	var miss []string
	for _, c := range cov {
		if c.Target > 0 && c.CloneAny == 0 {
			miss = append(miss, c.Elem)
		}
	}
	return miss
}

// matchCount counts element cells that agree between target grid and clone grid,
// both aligned to the same absolute scanline (tgtTop==cloneTop assumed).
func matchCount(tgt, clone [][]string) int {
	n := 0
	h := len(tgt)
	if len(clone) < h {
		h = len(clone)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < 160 && x < len(tgt[y]) && x < len(clone[y]); x++ {
			if tgt[y][x] == clone[y][x] {
				n++
			}
		}
	}
	return n
}

// grpByte reads an 8-bit player pattern at x, sampling every `w` clocks (NUSIZ
// width). MSB (D7) is the leftmost pixel.
func grpByte(elem []string, x, w int, name string) uint8 {
	var b uint8
	for i := 0; i < 8; i++ {
		c := x + i*w
		if c >= 0 && c < 160 && elem[c] == name {
			b |= 1 << (7 - i)
		}
	}
	return b
}

func byteTable(label string, data []uint8) string {
	var sb strings.Builder
	sb.WriteString("    align 256\n")
	fmt.Fprintf(&sb, "%s:\n", label)
	for i := 0; i < len(data); i += 8 {
		sb.WriteString("    .byte ")
		for j := i; j < i+8 && j < len(data); j++ {
			if j > i {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, "$%02X", data[j])
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// writeBlock is one per-scanline table→TIA-register store in the replay kernel:
// `lda TABLE,y` (4 cycles) + `sta REG` (3) = 7 CPU cycles. `deadline` is the
// visible clock of the FIRST pixel that register governs — land the store after
// it and that pixel still shows the previous line's value.
type writeBlock struct {
	table    string
	reg      string
	deadline int
	data     func(*frameData) []uint8
}

// Timing model of the loop, all in CPU cycles from the scanline's start. WSYNC
// releases the CPU at colour clock 0 of the new line, so block i (1-based)
// finishes at cycle 7i = colour clock 21i, i.e. visible clock 21i-68. One
// iteration costs `sta WSYNC` (3) + 7 per block + `iny`/`cpy #n`/`bne` (7).
//
// The block COUNT is capped at what `cyclebound` can certify, not at what the
// hardware would tolerate, and the two differ. Every table is `align 256` and the
// index never exceeds 255, so `lda TABLE,y` never crosses a page and really costs
// 4 — nine blocks run in 3+7*9+7 = 73 of the 76 cycles, and a nine-block clone was
// measured at 262 scanlines with its picture improved. But the static prover
// cannot assume the alignment, bounds `lda abs,y` at 5, and scores the same kernel
// at 3+8*9+7 = 82 against a 76 budget: `certified:false`, a violation on the
// visible `Kern` region. Eight blocks score 74 and certify (measured on the Outlaw
// clone). Emitting only kernels the project's own prover accepts is worth the slot
// — a generated artifact that trips the repo's line-budget gate is a landmine, and
// RL-7b exists because exactly that kind of unseen overrun shipped once already.
const (
	kernBlockCycles     = 7 // real cost: lda abs,y (4, tables are page-aligned) + sta zp (3)
	kernProvedBlockCost = 8 // cyclebound's conservative cost: lda abs,y bounded at 5
	kernFixedCycles     = 3 + 7
	kernMaxBlocks       = (76 - kernFixedCycles) / kernProvedBlockCost
)

func blockLands(i int) int { return 21*(i+1) - 68 } // i is 0-based

// placeable reports whether this kernel can put the object where the target has
// it: it must be drawn at all, and it must sit at ONE reset X for the whole
// frame, because the kernel strobes each RESxx once. An object that moves per
// zone is measured, reported and not drawn — never approximated to its first
// position, which would put pixels somewhere the target never had them.
func (fd *frameData) placeable(i int) bool {
	o := fd.obj[i]
	if i == objP0 || i == objP1 {
		return true // the players are always placed; their X is the calibrated marker
	}
	return o.lines > 0 && o.distinctX == 1
}

// wantX is the position the calibration drives this object to. Players use the
// end-of-frame marker the whole kernel is calibrated against; missiles and the
// ball use the single reset X measured from the pixels they drew.
func (fd *frameData) wantX(i int) int {
	switch i {
	case objP0:
		return fd.p0x
	case objP1:
		return fd.p1x
	}
	return fd.obj[i].onlyX
}

// alwaysOn reports that the object drew on EVERY visible line, so its enable can
// be written once before the kernel instead of costing a per-line write block.
// Measured on shared_setxpos: M0, M1 and the ball are all on for all 214 lines,
// so reproducing them there costs zero kernel cycles.
func (fd *frameData) alwaysOn(i int) bool { return fd.obj[i].lines == fd.h }

// positionCode emits the SetXPos calls for every placeable object.
func positionCode(fd *frameData, in [5]int) string {
	var b strings.Builder
	for i := range fd.obj {
		if !fd.placeable(i) {
			continue
		}
		fmt.Fprintf(&b, "    ldx #%d              ; %s\n    lda #%d\n    jsr SetXPos\n", i, objNames[i], in[i])
	}
	return b.String()
}

// staticEnables writes, once before the frame loop, the enable bit of every
// missile or ball that the target draws on EVERY visible line.
//
// An object that never turns off does not need a per-line table, and on a kernel
// with eight write slots that distinction is the difference between reproducing
// it and reporting it as absent: shared_setxpos draws M0, M1 and the ball on all
// 214 lines, so all three cost zero kernel cycles here.
func staticEnables(fd *frameData) string {
	var b strings.Builder
	for _, m := range []struct {
		idx int
		reg string
	}{{objM0, "ENAM0"}, {objM1, "ENAM1"}, {objBL, "ENABL"}} {
		if fd.placeable(m.idx) && fd.alwaysOn(m.idx) {
			fmt.Fprintf(&b, "    lda #2              ; %s on for all %d visible lines\n    sta %s\n",
				objNames[m.idx], fd.h, m.reg)
		}
	}
	return b.String()
}

// planKernel picks the per-scanline writes the replay kernel will carry.
//
// The eight-block PF+GRP layout is kept EXACTLY as it was whenever the target's
// NUSIZ never changes down the frame, so every clone framegen used to get right
// is still emitted byte for byte. Only a target that moves NUSIZ needs more
// blocks than fit, and only then is the layout re-planned: playfield writes the
// target provably does not need are dropped (both halves all zero, or the right
// half identical to the left, both measured over every line), and what remains is
// ordered by deadline. If two NUSIZ tables still do not fit, they are dropped and
// the caller reports it rather than emitting a kernel that runs long and rolls.
func planKernel(fd *frameData) (blocks []writeBlock, notes []string) {
	pf := []writeBlock{
		{"PF0LT", "PF0", 0, func(f *frameData) []uint8 { return f.pf0L }},
		{"PF1LT", "PF1", 16, func(f *frameData) []uint8 { return f.pf1L }},
		{"PF2LT", "PF2", 48, func(f *frameData) []uint8 { return f.pf2L }},
		{"PF0RT", "PF0", 80, func(f *frameData) []uint8 { return f.pf0R }},
		{"PF1RT", "PF1", 96, func(f *frameData) []uint8 { return f.pf1R }},
		{"PF2RT", "PF2", 128, func(f *frameData) []uint8 { return f.pf2R }},
	}
	// A missile or the ball that is on for EVERY visible line needs no per-line
	// write at all — its enable is set once before the kernel — so it costs zero
	// cycles. Only one that comes and goes needs a table. Measured: shared_setxpos
	// has all three on for all 214 lines (free); Fishing Derby's M1 and ball are on
	// for 42 and 58 of 214 (one block each).
	var extra []writeBlock
	for _, m := range []struct {
		idx        int
		table, reg string
		data       func(*frameData) []uint8
	}{
		{objM0, "EN0T", "ENAM0", func(f *frameData) []uint8 { return f.en0 }},
		{objM1, "EN1T", "ENAM1", func(f *frameData) []uint8 { return f.en1 }},
		{objBL, "ENBT", "ENABL", func(f *frameData) []uint8 { return f.enb }},
	} {
		if !fd.placeable(m.idx) || fd.alwaysOn(m.idx) {
			continue
		}
		extra = append(extra, writeBlock{m.table, m.reg, fd.obj[m.idx].onlyX, m.data})
	}

	g0 := writeBlock{"GRP0T", "GRP0", fd.obj[0].minX(fd.p0x), func(f *frameData) []uint8 { return f.grp0 }}
	g1 := writeBlock{"GRP1T", "GRP1", fd.obj[1].minX(fd.p1x), func(f *frameData) []uint8 { return f.grp1 }}
	n0 := writeBlock{"NZ0T", "NUSIZ0", g0.deadline, func(f *frameData) []uint8 { return f.nz0 }}
	n1 := writeBlock{"NZ1T", "NUSIZ1", g1.deadline, func(f *frameData) []uint8 { return f.nz1 }}

	vary0 := !constBytes(fd.nz0)
	vary1 := !constBytes(fd.nz1)
	if !vary0 && !vary1 && len(extra) == 0 {
		// Historical order, unchanged: GRP0, PF left, GRP1, PF right.
		return []writeBlock{g0, pf[0], pf[1], pf[2], g1, pf[3], pf[4], pf[5]}, nil
	}

	want := []writeBlock{g0, g1}
	want = append(want, extra...)
	if vary0 {
		want = append(want, n0)
	}
	if vary1 {
		want = append(want, n1)
	}
	// Playfield pruning, decided per REGISTER and only where the drop is provably
	// output-equivalent. A register whose two halves are 0 on every line is left at
	// the 0 the reset clear already wrote; a register whose right-half bytes equal
	// its left-half bytes on every line needs no mid-line rewrite, because CTRLPF is
	// 0 (repeat) and the right half then copies the left. Dropping a LEFT write on
	// its own is never sound — the following line would inherit the right half's
	// value — so the two are only ever dropped together.
	dropZero, dropRight := 0, 0
	for i := 0; i < 3; i++ {
		l, r := pf[i], pf[i+3]
		lb, rb := l.data(fd), r.data(fd)
		switch {
		case allZero(lb) && allZero(rb):
			dropZero++
		case sameBytes(lb, rb):
			want = append(want, l)
			dropRight++
		default:
			want = append(want, l, r)
		}
	}
	if dropZero > 0 {
		notes = append(notes, fmt.Sprintf("playfield: %d of the 3 PF registers are 0 on both halves of all %d visible lines, so the kernel does not write them (%d cycles freed)", dropZero, fd.h, dropZero*2*kernBlockCycles))
	}
	if dropRight > 0 {
		notes = append(notes, fmt.Sprintf("playfield: %d of the 3 PF registers have right-half bytes equal to their left half on all %d visible lines, so CTRLPF repeat reproduces them and the mid-line rewrite is dropped (%d cycles freed)", dropRight, fd.h, dropRight*kernBlockCycles))
	}
	if len(want) > kernMaxBlocks {
		// Drop in priority order and RE-CHECK after each removal, never a picture
		// write: an over-long kernel does not render a slightly wrong frame, it
		// desynchronises and rolls. Dropping one class and assuming that was enough
		// is how Fishing Derby came out with a 10-block, 80-cycle kernel against a
		// 76-cycle line and a 50-scanline frame — the tool printed its own "only 8
		// fit" note while emitting 10.
		//
		// NUSIZ goes before the enables: losing a size mis-draws an object that is
		// still there, losing an enable removes it from the picture entirely.
		needed := len(want)
		var dropped []string
		for _, reg := range []string{"NUSIZ0", "NUSIZ1", "ENABL", "ENAM1", "ENAM0"} {
			if len(want) <= kernMaxBlocks {
				break
			}
			for i, b := range want {
				if b.reg == reg {
					want = append(want[:i:i], want[i+1:]...)
					dropped = append(dropped, reg)
					break
				}
			}
		}
		kept := want
		notes = append(notes, fmt.Sprintf("NOT REPRODUCED: per-line %s. The kernel would need %d write blocks and only %d fit — 3+%d*n+7 CPU cycles against a 76-cycle scanline, counting each block at %d because a static prover bounds `lda abs,y` at 5 rather than assume the tables' page alignment (the hardware would run 9 blocks at 3+%d*9+7 = %d, but the emitted clone would then fail cyclebound at 82)",
			strings.Join(dropped, " and "), needed, kernMaxBlocks, kernProvedBlockCost, kernProvedBlockCost, kernBlockCycles, 3+kernBlockCycles*9+7))
		want = kept
	}
	// Earliest deadline first: every write has to land before the pixels it
	// governs, and PF0-left (deadline 0) is the tightest of them.
	sortStableByDeadline(want)
	return want, notes
}

// minX is the leftmost reset position the object was measured at on any line it
// drew on — the tightest deadline its register writes have to meet. Falls back to
// the end-of-frame marker when the object never drew.
func (f objFacts) minX(fallback int) int {
	if len(f.baseX) == 0 {
		return fallback
	}
	m := 1 << 30
	for x := range f.baseX {
		if x < m {
			m = x
		}
	}
	return m
}

func sortStableByDeadline(b []writeBlock) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j].deadline < b[j-1].deadline; j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}

func constBytes(a []uint8) bool {
	for i := 1; i < len(a); i++ {
		if a[i] != a[0] {
			return false
		}
	}
	return true
}

func allZero(a []uint8) bool {
	for _, v := range a {
		if v != 0 {
			return false
		}
	}
	return true
}

func sameBytes(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// kernelHas reports whether the planned kernel replays reg per scanline.
func kernelHas(blocks []writeBlock, reg string) bool {
	for _, b := range blocks {
		if b.reg == reg {
			return true
		}
	}
	return false
}

// emit builds the full clone source. p0in/p1in are the SetXPos routine inputs
// and vblankAdj shifts the VBLANK line count (all self-calibrated by the caller).
func emit(fd *frameData, in [5]int, vblankAdj, osAdj int) string {
	nlines := fd.h
	// Positioning costs one scanline per placed object (SetXPos opens with a WSYNC)
	// plus one for the HMOVE line. That used to be the constant 3 — two players and
	// HMOVE — and stayed 3 when missiles and the ball became placeable, which
	// overshot the overscan count into `ldx #257` on Fishing Derby: an immediate
	// operand that does not assemble, so the clone could not even be built.
	posLines := 1
	for i := range fd.obj {
		if fd.placeable(i) {
			posLines++
		}
	}
	vblank := fd.top - 3 - posLines + vblankAdj
	if vblank < 1 {
		vblank = 1
	}
	// Count what this function actually emits rather than what the frame nominally
	// contains: 3 VSYNC + 3 positioning + vblank + nlines + 1 cleanup + overscan,
	// with vblank = top-6+vblankAdj. Solving for 262 total leaves
	//   overscan = 261 - top - nlines - vblankAdj.
	// The vblankAdj term is the one that was missing: self-calibration adds lines
	// to VBLANK to slide the picture down and nothing took them back off the end,
	// so generated frames ran 264 scanlines — measured, and out of NTSC spec.
	overscan := 261 - fd.top - nlines - vblankAdj + osAdj
	// Clamped to what an immediate operand can hold. The frame-length
	// self-calibration corrects the count by measurement anyway, but it cannot run
	// at all if the first emitted source refuses to assemble.
	if overscan < 1 {
		overscan = 1
	}
	if overscan > 255 {
		overscan = 255
	}
	blocks := fd.kern
	// The kernel body, one 7-cycle block per line, annotated with the beam clock
	// the store lands at and the first visible clock it has to beat. Those are the
	// numbers that decide whether the reproduction is even possible, so they belong
	// in the file rather than only in the terminal that generated it.
	var kb strings.Builder
	for i, b := range blocks {
		lands := blockLands(i)
		fmt.Fprintf(&kb, "    lda %s,y\n    sta %-6s      ; lands at visible clock %+d, needs <= %d\n",
			b.table, b.reg, lands, b.deadline)
	}
	ctrlpfNote := "repeat mode: independent halves via per-line rewrite"
	if !kernelHas(blocks, "PF0") {
		ctrlpfNote = "repeat mode; the kernel rewrites only what the target needs"
	}
	// Put the end-of-frame NUSIZ back after the kernel: framegen calibrates the
	// sprite X by comparing the target's and the clone's HmovedPixel at the frame
	// boundary, and that marker moves 1 pixel with the size bits (measured: 24 at
	// 1x, 25 at 2x/4x) — so leaving the last scanline's NUSIZ in place would make
	// the two markers describe different things.
	nusizRestore := ""
	if kernelHas(blocks, "NUSIZ0") || kernelHas(blocks, "NUSIZ1") {
		nusizRestore = fmt.Sprintf("    lda #$%02X\n    sta NUSIZ0\n    lda #$%02X\n    sta NUSIZ1\n", fd.nusiz0, fd.nusiz1)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `; ==========================================================================
; Auto-generated by cmd/framegen from a target ROM — a from-scratch, self-
; contained pixel reproduction of the target's static visible frame via a
; per-scanline PF(left/right) + GRP0/GRP1 replay kernel. DO NOT hand-edit;
; regenerate with framegen. Verify with: vismatch -target <rom> -mine this.asm
; ==========================================================================
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
NUSIZ0 = $04
NUSIZ1 = $05
COLUP0 = $06
COLUP1 = $07
COLUPF = $08
COLUBK = $09
CTRLPF = $0A
REFP0  = $0B
REFP1  = $0C
PF0    = $0D
PF1    = $0E
PF2    = $0F
RESP0  = $10
RESP1  = $11
GRP0   = $1B
GRP1   = $1C
ENAM0  = $1D
ENAM1  = $1E
ENABL  = $1F
HMP0   = $20
HMP1   = $21
HMOVE  = $2A
    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
clr:
    sta $00,x
    dex
    bne clr
    lda #$%02X
    sta COLUP0
    lda #$%02X
    sta COLUP1
    lda #$%02X
    sta COLUPF
    lda #$%02X
    sta COLUBK
    lda #$%02X
    sta NUSIZ0
    lda #$%02X
    sta NUSIZ1
    lda #$%02X
    sta CTRLPF          ; %s
    lda #0
    sta REFP0
    sta REFP1
%s

Main:
    lda #2
    sta VSYNC
    sta WSYNC
    sta WSYNC
    sta WSYNC
    lda #0
    sta VSYNC
    lda #2
    sta VBLANK
    ; --- position the objects (SetXPos: div-15 coarse + HMOVE fine) ---
%s    sta WSYNC
    sta HMOVE
    ; --- pad VBLANK to the target's first visible scanline ---
    ldx #%d
vb:
    sta WSYNC
    dex
    bne vb
    lda #0
    sta VBLANK
    ; --- visible: replay %d scanlines ---
    ldy #0
Kern:
    sta WSYNC
%s    iny
    cpy #%d
    bne Kern
    sta WSYNC           ; end the last visible line BEFORE clearing anything:
    lda #0              ; without it the cleanup runs inside that line and clears
    sta GRP0            ; GRP0 at beam clock +133 and GRP1 at +142 (measured),
    sta GRP1            ; erasing any sprite pixel right of there on that one line
    sta PF0
    sta PF1
    sta PF2
%s    lda #2
    sta VBLANK
    ldx #%d
os:
    sta WSYNC
    dex
    bne os
    jmp Main

; SetXPos: X=object index, A=input. div-15 coarse RESPx + eor#7 HMOVE nibble.
SetXPos:
    sta WSYNC
    sec
.w:
    sbc #15
    bcs .w
    eor #7
    asl
    asl
    asl
    asl
    sta HMP0,x
    sta.w RESP0,x
    rts

`, fd.p0col, fd.p1col, fd.pfcol, fd.bgcol,
		fd.nusiz0|fd.msz0, fd.nusiz1|fd.msz1, fd.bsz, ctrlpfNote, staticEnables(fd),
		positionCode(fd, in), vblank, nlines, kb.String(), nlines, nusizRestore, overscan)

	// Only the tables the kernel actually reads (a dropped playfield write would
	// otherwise leave 256 bytes of dead data behind), in a fixed order so two runs
	// stay diffable no matter how the kernel was scheduled.
	for _, name := range []string{"PF0LT", "PF1LT", "PF2LT", "PF0RT", "PF1RT", "PF2RT", "GRP0T", "GRP1T", "NZ0T", "NZ1T", "EN0T", "EN1T", "ENBT"} {
		for _, b := range blocks {
			if b.table == name {
				sb.WriteString(byteTable(b.table, b.data(fd)))
				break
			}
		}
	}
	sb.WriteString("\n    org $FFFC\n    .word Reset\n    .word Reset\n")
	return sb.String()
}

// renderClone assembles src, renders it, and returns the clone's P0/P1 landing X
// and its Snapshot visible top (for vertical calibration).
// renderClonePositions renders the generated source and reports where each of the
// five objects actually landed, plus the visible top. The positions come from the
// same measurement as the target's, so the calibration compares like with like.
// clampInput keeps a SetXPos argument inside the immediate operand's range. The
// routine's div-15 loop wraps the position anyway, so a value above 255 is not a
// larger move, it is a source file that does not assemble.
func clampInput(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func renderClonePositions(src, spec string, frames int) (x [5]int, top int, err error) {
	p0, p1, t, err := renderClone(src, spec, frames)
	if err != nil {
		return x, 0, err
	}
	x[objP0], x[objP1] = p0, p1
	_, _, obj, err := renderGrid(src, spec, frames)
	if err != nil {
		return x, t, err
	}
	for i := objM0; i <= objBL; i++ {
		if obj[i].lines == 0 {
			x[i] = -1 // never drawn: distinct from "drawn at clock 0"
			continue
		}
		x[i] = obj[i].onlyX
	}
	return x, t, nil
}

func renderClone(src, spec string, frames int) (p0x, p1x, top int, err error) {
	dir, err := os.MkdirTemp("", "framegen")
	if err != nil {
		return 0, 0, 0, err
	}
	asm := dir + "/c.asm"
	bin := dir + "/c.bin"
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		return 0, 0, 0, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return 0, 0, 0, fmt.Errorf("assemble clone:\n%s", out)
	}
	e, err := emu.New(spec)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := e.LoadROM(bin); err != nil {
		return 0, 0, 0, err
	}
	if err := e.RunFrames(frames); err != nil {
		return 0, 0, 0, err
	}
	mk := e.Markers()
	_, vt := e.Snapshot()
	return mk[0].Clock, mk[2].Clock, vt, nil
}

func main() {
	rom := flag.String("rom", "", "target ROM .bin (required)")
	out := flag.String("out", "clone.asm", "output asm path")
	spec := flag.String("spec", "NTSC", "TV spec")
	frames := flag.Int("frames", 28, "frames to render the target")
	reset := flag.Bool("reset", false, "press console RESET to start the target")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: framegen -rom target.bin [-reset] -out clone.asm")
		os.Exit(2)
	}
	fd, err := extract(*rom, *spec, *frames, *reset)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(2)
	}
	fmt.Printf("extracted: visible %d lines (top %d), P0col=$%02X P1col=$%02X PFcol=$%02X BGcol=$%02X NUSIZ0=$%02X NUSIZ1=$%02X P0x=%d P1x=%d\n",
		fd.h, fd.top, fd.p0col, fd.p1col, fd.pfcol, fd.bgcol, fd.nusiz0, fd.nusiz1, fd.p0x, fd.p1x)
	for i := range fd.obj {
		o := fd.obj[i]
		if o.lines == 0 {
			continue // the target never draws it; saying nothing beats printing zeros
		}
		if i >= objM0 {
			// A missile and the ball have no graphics byte: their run IS their width,
			// so copies and NUSIZ modes would be noise here.
			fmt.Printf("  measured %s: drawn on %d of %d lines, %d distinct reset X %s, width(s) %s\n",
				o.name, o.lines, fd.h, o.distinctX, o.xList(), o.widthList())
			continue
		}
		fmt.Printf("  measured %s: drawn on %d lines, NUSIZ %s, up to %d copies, %d run(s) max per line, %d distinct reset X %s\n",
			o.name, o.lines, o.modeList(), o.maxCopies, o.maxRuns, o.distinctX, o.xList())
	}
	var planNotes []string
	fd.kern, planNotes = planKernel(fd)
	names := make([]string, len(fd.kern))
	for i, b := range fd.kern {
		names[i] = b.reg
	}
	fmt.Printf("  kernel: %d write blocks (%s), %d of 76 CPU cycles per scanline\n",
		len(fd.kern), strings.Join(names, ","), kernFixedCycles+kernBlockCycles*len(fd.kern))
	for _, n := range planNotes {
		fmt.Println("  " + n)
	}

	// Self-calibrate: the two SetXPos inputs (players land on the target's
	// columns — the landing offset is kernel-specific) AND the VBLANK line count
	// (the clone's visible top matches the target's, so content aligns vertically).
	// osAdj corrects the overscan count once the frame length can be measured; it
	// stays 0 through the X/VBLANK calibration, which does not depend on it.
	// One position input per placeable object. The two players were calibrated this
	// way already; missiles and the ball go through the same routine because
	// RESP0..RESBL ($10-$14) and HMP0..HMBL ($20-$24) are consecutive, so SetXPos's
	// X register selects any of the five.
	in := [5]int{40, 40, 40, 40, 40}
	var stuck [5]bool
	var misses [5]int
	vblankAdj, osAdj := 0, 0
	for iter := 0; iter < 24; iter++ {
		src := emit(fd, in, vblankAdj, osAdj)
		got, top, err := renderClonePositions(src, *spec, *frames)
		if err != nil {
			fmt.Fprintln(os.Stderr, "calibrate:", err)
			os.Exit(2)
		}
		dtop := fd.top - top
		done := dtop == 0
		var parts []string
		for i := range fd.obj {
			if !fd.placeable(i) || stuck[i] {
				continue
			}
			want := fd.wantX(i)
			if got[i] < 0 {
				// Not drawn THIS round is not the same as cannot be drawn. Every object
				// starts at the same input, so they pile up on each other, and TIA
				// priority hides the lower ones: measured on shared_setxpos, M1 was
				// invisible until M0 moved off it, and freezing on the first miss left
				// M1 stranded at its starting position with all 1712 of its cells wrong.
				// So hold the input still, keep iterating, and give up only after
				// several rounds — the others move away and it reappears.
				//
				// Chasing the full delta instead walks the input out of range in two
				// rounds (measured 40 -> 149 -> 258, and `lda #258` does not assemble).
				misses[i]++
				if misses[i] >= 4 {
					stuck[i] = true
				}
				done = false
				parts = append(parts, fmt.Sprintf("%s NOT-DRAWN(want %d, miss %d)", objNames[i], want, misses[i]))
				continue
			}
			misses[i] = 0
			d := want - got[i]
			if d != 0 {
				done = false
			}
			in[i] = clampInput(in[i] + d) // +1 input ≈ +1px right near the target
			parts = append(parts, fmt.Sprintf("%s %d(want %d,d%+d)", objNames[i], got[i], want, d))
		}
		fmt.Printf("  calib iter %d: %s top %d(want %d,d%+d) in=%v vbAdj=%d\n",
			iter, strings.Join(parts, " "), top, fd.top, dtop, in, vblankAdj)
		if done {
			break
		}
		vblankAdj += dtop // more VBLANK lines → visible top moves later (down)
	}

	// Content vertical-shift search: after the visible top matches, nudge the
	// replay tables ±lines so the rendered picture aligns line-for-line with the
	// target (kills the residual constant offset the top-match can't see).
	bestS, bestM := 0, -1
	for s := -4; s <= 4; s++ {
		src := emit(fd.shifted(s), in, vblankAdj, osAdj)
		grid, _, _, err := renderGrid(src, *spec, *frames)
		if err != nil {
			continue
		}
		m := matchCount(fd.tgtElem, grid)
		fmt.Printf("  content shift %+d: element match %d / %d\n", s, m, fd.h*160)
		// Strictly better, or equal at no shift. A tie means the offset explains
		// nothing — motion_glide scores 34232 at all nine — and the scan starting
		// at -4 made "explains nothing" come out as "shift the picture up 4 lines".
		if m > bestM || (m == bestM && s == 0) {
			bestM, bestS = m, s
		}
	}
	fmt.Printf("  chosen content shift: %+d\n", bestS)

	src := emit(fd.shifted(bestS), in, vblankAdj, osAdj)

	// Self-calibrate the frame LENGTH, for the same reason X and VBLANK are
	// calibrated rather than computed: the prologue's cost is not a constant.
	// `SetXPos` is a div-15 subtract loop, so a player far to the right takes
	// longer to place than one on the left and can run past its own scanline —
	// Combat (P1 at clock 145, input 139) spends one line more than Outlaw does.
	// Deriving the overscan count from a formula therefore cannot be right for
	// every target; measuring the emitted frame and correcting the difference is.
	want := 262
	if strings.EqualFold(*spec, "PAL") || strings.EqualFold(*spec, "SECAM") {
		want = 312
	}
	for iter := 0; iter < 8; iter++ {
		got, err := cloneFrameLines(src, *spec)
		if err != nil {
			fmt.Fprintln(os.Stderr, "frame-length calibration:", err)
			break
		}
		fmt.Printf("  frame length iter %d: %d lines (want %d) osAdj=%d\n", iter, got, want, osAdj)
		if got == want {
			break
		}
		osAdj += want - got
		src = emit(fd.shifted(bestS), in, vblankAdj, osAdj)
	}

	// What did it actually reproduce? Measured per element against the clone's
	// own rendered frame, because "pixel-exact" on a target that draws no
	// missiles says nothing about a target that does.
	var cov []elemCoverage
	var cloneObj [5]objFacts
	cloneObj[0], cloneObj[1] = newObjFacts("P0"), newObjFacts("P1")
	if grid, _, co, err := renderGrid(src, *spec, *frames); err == nil {
		// fd.tgtElem, not the shifted copy: shifted() moves the REPLAY tables, and
		// the target's own attribution is the reference the shift was chosen against.
		cov = coverage(fd.tgtElem, grid)
		cloneObj = co
	} else {
		fmt.Fprintln(os.Stderr, "coverage: could not render the clone:", err)
	}
	// Frame length is not something the picture comparison can see: a clone can be
	// pixel-exact and still emit 264 scanlines, which rolls on a real television.
	// It went unnoticed for exactly that reason, so it is measured here every run.
	lines, lerr := cloneFrameLines(src, *spec)

	miss := missingElements(cov)
	why := diagnose(cov, fd.obj, cloneObj, fd.kern)
	src = annotate(src, cov, miss, planNotes, why)

	if err := os.WriteFile(*out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(2)
	}
	fmt.Printf("wrote %s (%d bytes source)\n", *out, len(src))

	switch {
	case lerr != nil:
		fmt.Println("frame length: could not measure —", lerr)
	case lines != want:
		fmt.Printf("frame length: %d scanlines, want %d  <-- OUT OF SPEC (would roll on hardware)\n", lines, want)
	default:
		fmt.Printf("frame length: %d scanlines (correct for %s)\n", lines, strings.ToUpper(*spec))
	}

	fmt.Println("reproduction by element (target cells / matched / drawn anywhere in the clone):")
	for _, c := range cov {
		note := ""
		switch {
		case c.Target > 0 && c.CloneAny == 0:
			note = "  <-- NOT REPRODUCED: this kernel never draws this element"
		case c.Target > 0 && c.Matched < c.Target:
			note = fmt.Sprintf("  (%d cells differ)", c.Target-c.Matched)
		}
		fmt.Printf("  %-2s target %6d  matched %6d  clone %6d%s\n", c.Elem, c.Target, c.Matched, c.CloneAny, note)
	}
	if len(miss) > 0 {
		fmt.Printf("RESULT: partial reproduction — the target draws %s and the generated kernel emits "+
			"no %s writes at all, so every one of those pixels is absent from the clone. "+
			"Use it as a PF/player validator, not as a full-frame reproduction.\n",
			strings.Join(miss, "/"), strings.Join(enableRegs(miss), "/"))
		os.Exit(1)
	}
	// An element the clone draws SOMEWHERE but in the wrong cells is a different
	// finding from one it never draws, and it must not fall through to a success
	// line. Which of the two structural limits caused it — copies the kernel does
	// not order, or a position it cannot follow — is decided by the measurements
	// above, never asserted: the line that used to stand here blamed "a per-zone
	// multiplexed target" unconditionally, and on litmus_nusiz_all (both players
	// placed once before the frame loop, 1 distinct reset X measured for each) that
	// was simply false.
	if lerr == nil && lines != want {
		fmt.Printf("RESULT: the picture matches but the frame is %d scanlines, not %d — a clone that "+
			"reproduces every pixel and the wrong frame length still rolls on hardware.\n", lines, want)
		os.Exit(1)
	}
	short := 0
	for _, c := range cov {
		short += c.Target - c.Matched
	}
	if short > 0 {
		fmt.Printf("RESULT: differences remain — %d of %d visible cells are drawn by a different element "+
			"than the target's. %s\n", short, len(fd.tgtElem)*160, why)
		os.Exit(1)
	}
	fmt.Println("RESULT: pixel-exact — every visible cell is drawn by the same element as the target")
}

// diagnose says why the clone's cells differ, from counted quantities only.
//
// Two structural limits of this kernel shape can put a present element in the
// wrong cells, and they are not interchangeable:
//
//   - COPIES — the target asks the TIA for 2 or 3 hardware copies of a player and
//     the clone draws fewer. Measured as copies-per-line on both sides.
//   - MULTIPLEXING — the target moves the sprite down the frame and the clone,
//     which strobes RESPx once, cannot follow. Measured as the number of DISTINCT
//     reset positions (size-mode shift removed, because a 1x→2x switch moves the
//     first drawn pixel by 1 without the sprite having moved).
//   - A LATE WRITE — the kernel's own store for that object lands after the beam
//     has already passed the object's leftmost pixel, so the line renders the
//     PREVIOUS line's byte. Measured against the emitted block schedule, not
//     assumed: the kernel is a fixed sequence of 7-cycle blocks and each one's
//     landing clock is arithmetic on its position in that sequence.
//
// Anything else is named as unexplained rather than attributed to whichever of
// the three happens to sound plausible.
func diagnose(cov []elemCoverage, tgt, clone [5]objFacts, blocks []writeBlock) string {
	differ := map[string]int{}
	over := map[string]int{}
	for _, c := range cov {
		differ[c.Elem] = c.Target - c.Matched
		over[c.Elem] = c.CloneAny - c.Target
	}
	var clauses []string
	for i, name := range []string{"P0", "P1"} {
		d := differ[name]
		if d <= 0 {
			continue
		}
		t, c := tgt[i], clone[i]
		var why []string
		if t.maxCopies > c.maxCopies {
			why = append(why, fmt.Sprintf("the target orders up to %d copies (NUSIZ %s) and the clone draws up to %d",
				t.maxCopies, t.modeList(), c.maxCopies))
		}
		if t.distinctX > 1 {
			why = append(why, fmt.Sprintf("its reset X takes %d distinct values across the frame %s and this kernel strobes RESP%d once",
				t.distinctX, t.xList(), i))
		}
		for _, l := range lateWrites(blocks, fmt.Sprintf("%d", i)) {
			why = append(why, l)
		}
		if len(why) == 0 {
			why = append(why, fmt.Sprintf("copies %d target vs %d clone, reset X %d distinct target vs %d clone, "+
				"and every write for it lands before its leftmost pixel — none of copies, multiplexing or a late "+
				"write explains these cells, so the cause is not one this tool has measured",
				t.maxCopies, c.maxCopies, t.distinctX, c.distinctX))
		}
		clauses = append(clauses, fmt.Sprintf("%s (%d cells): %s", name, d, strings.Join(why, "; and ")))
	}
	if d := differ["PF"]; d > 0 {
		clauses = append(clauses, fmt.Sprintf("PF (%d cells)", d))
	}
	if len(clauses) == 0 {
		// Every cell the target draws with an object is drawn the same way, so the
		// remainder is the clone painting over background the target left empty —
		// the signature of a register the target changed part-way ALONG a scanline,
		// which a kernel that writes each register once per line in HBLANK cannot
		// express at any byte value.
		for _, e := range []string{"P0", "P1", "PF"} {
			if over[e] > 0 {
				clauses = append(clauses, fmt.Sprintf("the clone draws %d %s cells the target leaves as background, "+
					"while matching every %s cell the target does draw", over[e], e, e))
			}
		}
		if len(clauses) == 0 {
			return "Every element is present and every object cell matches; the difference is in BG cells only."
		}
	}
	return "Measured: " + strings.Join(clauses, ". ") + "."
}

// cloneFrameLines assembles the generated source and asks the machine how many
// scanlines one of its frames actually takes.
func cloneFrameLines(src, spec string) (int, error) {
	dir, err := os.MkdirTemp("", "framegen-lines")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	asm, bin := dir+"/c.asm", dir+"/c.bin"
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		return 0, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return 0, fmt.Errorf("assemble:\n%s", out)
	}
	e, err := emu.New(spec)
	if err != nil {
		return 0, err
	}
	if err := e.LoadROM(bin); err != nil {
		return 0, err
	}
	// Settle first: the frames right after reset are not representative.
	if err := e.RunFrames(8); err != nil {
		return 0, err
	}
	return e.StepFrame()
}

// lateWrites lists the kernel blocks for object suffix ("0" or "1") whose store
// lands after the first pixel it governs. The kernel is a fixed chain of 7-cycle
// blocks after WSYNC, so block i finishes at CPU cycle 7i = colour clock 21i =
// visible clock 21i-68 — no estimate is involved.
func lateWrites(blocks []writeBlock, suffix string) []string {
	var out []string
	for i, b := range blocks {
		if !strings.HasSuffix(b.reg, suffix) || (!strings.HasPrefix(b.reg, "GRP") && !strings.HasPrefix(b.reg, "NUSIZ")) {
			continue
		}
		if lands := blockLands(i); lands > b.deadline {
			out = append(out, fmt.Sprintf("the kernel's %s store is block %d of %d and lands at visible clock %+d, "+
				"past the object's leftmost drawn pixel at %d, so those lines render the previous line's value",
				b.reg, i+1, len(blocks), lands, b.deadline))
		}
	}
	return out
}

// wrapComment folds a long report line into an assembler comment block so the
// generated source stays inside a readable width.
func wrapComment(s string) string {
	const width = 74
	var out strings.Builder
	col := 0
	for _, w := range strings.Fields(s) {
		if col > 0 && col+1+len(w) > width {
			out.WriteString("\n; ")
			col = 0
		} else if col > 0 {
			out.WriteString(" ")
			col++
		}
		out.WriteString(w)
		col += len(w)
	}
	return out.String()
}

// enableRegs names the TIA register that would have to be written for each
// missing element, so the report says what is absent from the source, not just
// what is absent from the picture.
func enableRegs(elems []string) []string {
	reg := map[string]string{"M0": "ENAM0", "M1": "ENAM1", "BL": "ENABL", "P0": "GRP0", "P1": "GRP1", "PF": "PF0/1/2"}
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		if r, ok := reg[e]; ok {
			out = append(out, r)
		} else {
			out = append(out, e)
		}
	}
	return out
}

// annotate burns the coverage measurement into the generated source's banner.
// The file outlives the terminal it was generated in; a clone that is missing
// every bullet must say so where it will still be read months later.
func annotate(src string, cov []elemCoverage, miss, notes []string, why string) string {
	var b strings.Builder
	b.WriteString("; --- reproduction coverage (measured against the target's own frame) ---\n")
	for _, c := range cov {
		fmt.Fprintf(&b, "; %-2s target %6d  matched %6d  clone %6d\n", c.Elem, c.Target, c.Matched, c.CloneAny)
	}
	if len(miss) > 0 {
		fmt.Fprintf(&b, "; NOT REPRODUCED: %s — this kernel emits no %s, so those pixels are\n"+
			"; absent by construction. This file is a PF/player reproduction, not the whole frame.\n",
			strings.Join(miss, "/"), strings.Join(enableRegs(miss), "/"))
	}
	// The kernel-planning decisions and the measured cause of any residual
	// difference belong in the file, for the same reason the coverage table does:
	// the file outlives the terminal that produced it.
	for _, n := range notes {
		fmt.Fprintf(&b, "; %s\n", wrapComment(n))
	}
	if why != "" {
		fmt.Fprintf(&b, "; %s\n", wrapComment(why))
	}
	b.WriteString("; ---------------------------------------------------------------------\n")
	const anchor = "; ==========================================================================\n    processor 6502\n"
	if i := strings.Index(src, anchor); i >= 0 {
		return src[:i+len("; ==========================================================================\n")] +
			b.String() + src[i+len("; ==========================================================================\n"):]
	}
	return b.String() + src
}
