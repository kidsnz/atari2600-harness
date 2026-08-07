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
	nz0, nz1         []uint8    // per-visible-line NUSIZ, measured (see measureLines)
	tgtElem          [][]string // target's per-visible-line object attribution (for content-shift search)
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
	// bx[i][y] is object i's reset X on visible line y with the NUSIZ size shift
	// removed, or notDrawn when it drew nothing there. objFacts collapses the same
	// measurement into a histogram, which cannot say WHERE a position changed — and
	// where is the whole question for a zone-structured kernel: a target that takes
	// six positions in six separated bands can be followed, one that takes six while
	// drawing continuously cannot.
	bx [5][]int
	// lx[i][y] is the LEFTMOST clock object i actually drew a pixel at on visible
	// line y, or notDrawn. It exists because the reset position and the drawn window
	// are not the same number: measured on zone_multiplex, the same 8-pixel-wide
	// sprite line reports reset X 49 with span 49..56 in one band, reset X 117 with
	// span 116..123 in another, and reset X 4 with span 4..11 in a third — the
	// marker is off by -1, 0 or +1 depending on the fine HMOVE nibble that put the
	// object there. A kernel calibrated against the marker is therefore up to a pixel
	// out, and a graphics byte sampled at the marker reads the wrong 8 clocks. Both
	// sides of the comparison are anchored on drawn pixels instead.
	lx   [5][]int
	kern []writeBlock
	// zones is the reproduction plan when the target repositions down the frame:
	// nil means the historical single-position kernel. See planZones.
	zones   []zone
	zfollow [5]bool // objects the zone plan repositions per zone
	// zdrop[i] is the measured reason the zone plan could NOT follow object i, kept
	// so the final RESULT line can name it. Without it the report says only "this
	// kernel strobes RESPx once" — true, but it hides that a zone kernel WAS costed
	// against the target's own gaps and came up short by a counted number of lines.
	zdrop [5]string
}

// notDrawn marks a visible line on which an object drew no pixels at all. A real
// reset X is 0..159, so a sentinel is needed: "drawn at clock 0" and "not drawn"
// are different facts and collapsing them would invent a position.
const notDrawn = -1

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

// xRun is one maximal band of consecutive visible lines on which an object was
// drawn at ONE reset X, plus the number of blank lines immediately before it.
//
// This is the measurement the per-zone kernel stands on. `objFacts.distinctX`
// counts HOW MANY positions a target used; it cannot say whether they are
// reachable, because repositioning costs whole scanlines (SetXPos opens with `sta
// WSYNC`) and those scanlines have to come from somewhere. A band structure with
// blank lines between the bands has room; a continuously-drawn object that slides
// every few lines does not, and the difference is `gap`.
type xRun struct {
	start, end int // inclusive visible-line indices
	x          int // reset X, size shift removed
	gap        int // blank lines (object not drawn) immediately before start
}

func (r xRun) lines() int { return r.end - r.start + 1 }

// xRuns folds a per-line reset-X series into bands. A band breaks when the object
// stops being drawn OR when its position changes while it is still being drawn —
// the second case is recorded as a band with gap 0, which is exactly the case a
// zone kernel cannot serve, so it must not be smoothed over.
func xRuns(bx []int) []xRun {
	var out []xRun
	blank := 0
	for y := 0; y < len(bx); y++ {
		if bx[y] == notDrawn {
			blank++
			continue
		}
		if n := len(out); n > 0 && blank == 0 && out[n-1].end == y-1 && out[n-1].x == bx[y] {
			out[n-1].end = y
			continue
		}
		out = append(out, xRun{start: y, end: y, x: bx[y], gap: blank})
		blank = 0
	}
	return out
}

// runList renders the measured bands so every claim about zone structure arrives
// with the lines, the position and the gap that were counted. Long series are
// truncated with the count of what was left out — a report that scrolls off the
// screen is a report nobody reads, but silently dropping rows would hide evidence.
func runList(rs []xRun, max int) string {
	if len(rs) == 0 {
		return "(never drawn)"
	}
	n := len(rs)
	if max > 0 && n > max {
		n = max
	}
	parts := make([]string, 0, n+1)
	for _, r := range rs[:n] {
		parts = append(parts, fmt.Sprintf("L%d-%d(%dL)@x%d gap%d", r.start, r.end, r.lines(), r.x, r.gap))
	}
	if n < len(rs) {
		parts = append(parts, fmt.Sprintf("...+%d more bands", len(rs)-n))
	}
	return strings.Join(parts, " ")
}

// heldRun counts consecutive target lines ending at y that a positioning block could
// occupy: lines the HELD registers already reproduce.
//
// The planner used to demand background-only lines, and on this corpus every failure
// reported have=0 — real kernels draw something on every line, so the requirement was
// almost never satisfiable. But a positioning line does not need a blank target. During
// the block the replay loop is not running, so GRP0/GRP1/PF hold whatever the last
// replayed line left, and the target matches iff those lines are IDENTICAL to that line.
// A repeated row is exactly that, and repeated rows are everywhere: Fishing Derby's water
// gives 7 of them where the old test saw 0.
//
// This generalises the old rule rather than replacing it — an all-background run is a run
// of identical lines, so everything blankRun accepted, this accepts too.
//
// The anchor is included on purpose. The block owns [y-n+1, y] and the registers it holds
// come from the line before it, so every line in the block AND that source line have to
// agree; counting the run back from y is what makes "n >= posLines+1" say that.
func (fd *frameData) heldRun(y int) int {
	if y < 0 || y >= len(fd.tgtElem) {
		return 0
	}
	n := 1
	for p := y - 1; p >= 0; p-- {
		if len(fd.tgtElem[p]) != len(fd.tgtElem[y]) {
			break
		}
		same := true
		for i := range fd.tgtElem[y] {
			if fd.tgtElem[p][i] != fd.tgtElem[y][i] {
				same = false
				break
			}
		}
		if !same {
			break
		}
		n++
	}
	return n
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
	// The zone plan is indexed by the SAME y the tables are, so it has to travel
	// with them: out[y] = table[y+s] means the band that sat at target lines
	// [a,b] is emitted at kernel lines [a-s, b-s]. Leaving the boundaries behind
	// would reposition the sprite s lines away from the band it belongs to.
	for i := range c.bx {
		c.bx[i] = shiftInt(fd.bx[i], s, notDrawn)
	}
	if fd.zones != nil {
		c.zones = make([]zone, len(fd.zones))
		for k, z := range fd.zones {
			z.start -= s
			z.end -= s
			if z.posStart >= 0 {
				z.posStart -= s
			}
			c.zones[k] = z
		}
		// The loops have to cover the whole frame however the content moved, so the
		// outer edges are pinned rather than shifted. Both edges are inside a blank
		// region on any target this plan accepts (a boundary needs background-only
		// lines), so a line added or dropped there changes no pixel.
		c.zones[0].start = 0
		c.zones[len(c.zones)-1].end = fd.h - 1
	}
	return &c
}

// shiftInt is shiftUint8 for the per-line position series, with its own fill
// value: 0 is a legal reset X, so an edge has to read "not drawn", not "drawn at
// the left edge".
func shiftInt(a []int, s, fill int) []int {
	out := make([]int, len(a))
	for i := range out {
		j := i + s
		if j >= 0 && j < len(a) {
			out[i] = a[j]
		} else {
			out[i] = fill
		}
	}
	return out
}

// zonesValid reports that every zone still replays at least one line and that no
// boundary has been pushed outside the frame. A vertical-shift candidate that
// breaks the plan is skipped rather than emitted: the emitted loop would run
// `cpy` against a bound it never reaches.
func (fd *frameData) zonesValid() bool {
	if fd.zones == nil {
		return true
	}
	prev := -1
	for k, z := range fd.zones {
		if z.start > z.end || z.start < 0 || z.end >= fd.h {
			return false
		}
		if z.start <= prev {
			return false
		}
		if k > 0 && (z.posStart <= fd.zones[k-1].end || z.posStart >= z.start) {
			return false
		}
		prev = z.end
	}
	return true
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
		fd.bx[objP0] = append(fd.bx[objP0], lineBaseX(runsPerLine[y], "P0", ls.nz0, ls.x0))
		fd.bx[objP1] = append(fd.bx[objP1], lineBaseX(runsPerLine[y], "P1", ls.nz1, ls.x1))
		// Missiles and the ball are measured from the pixels they DREW rather than
		// from a register read: a missile has no graphics byte, so its run in the
		// attribution is its position and its width directly, and reading it that way
		// needs no second measurement pass to disagree with the first. NUSIZ is
		// passed as 0 because the size shift countRuns removes applies to a player's
		// graphics start, not to a missile — a missile's run starts where it starts.
		for i, name := range []string{"M0", "M1", "BL"} {
			bx := notDrawn
			if x, w, ok := firstRun(runsPerLine[y], name); ok {
				countRuns(&fd.obj[objM0+i], runsPerLine[y], name, 0, x)
				fd.obj[objM0+i].widths[w]++
				bx = x
			}
			fd.bx[objM0+i] = append(fd.bx[objM0+i], bx)
		}
		fd.en0 = append(fd.en0, enableByte(runsPerLine[y], "M0"))
		fd.en1 = append(fd.en1, enableByte(runsPerLine[y], "M1"))
		fd.enb = append(fd.enb, enableByte(runsPerLine[y], "BL"))
	}
	for i := range fd.obj {
		fd.obj[i].distinctX = len(fd.obj[i].baseX)
		fd.lx[i] = leftmostDrawn(fd.tgtElem, objNames[i])
	}
	fd.msz0 = sizeBits(fd.obj[objM0])
	fd.msz1 = sizeBits(fd.obj[objM1])
	fd.bsz = sizeBits(fd.obj[objBL])

	return fd, nil
}

// buildGRP samples the two players' graphics bytes. It runs AFTER planZones,
// because where the pattern has to be read from depends on where the kernel will
// have put the sprite.
//
// For a player the kernel places ONCE, the byte is sampled at the line's own pixel
// width from the END-OF-FRAME reset position, corrected only by that line's size
// shift. It deliberately does NOT sample at the per-line HmovedPixel: the position
// the kernel strobes to is calibrated against the end-of-frame marker, so reading
// the pattern from anywhere else draws the right shape in the wrong column.
// Measured cost of getting that wrong: sampling at the per-line position turned
// flicker_multiplex (0 cells wrong) into 78, sprite_anim (0) into 84 and text24
// (162) into P0/P1 absent entirely — the end-of-frame marker and the visible-line
// position are not the same number on a target that parks its sprites during
// overscan.
//
// For a player the zone plan FOLLOWS, the opposite is true and for the same
// reason: the kernel puts the sprite at the band's own measured position, so the
// pattern must be read from that position. Sampling such a target at the
// end-of-frame marker reads 8 bits of background — zone_multiplex's P0 ends its
// frame at reset X 99 while its six bands sit at 52, 82, 119, 22, 11 and 99.
func (fd *frameData) buildGRP() {
	fd.grp0, fd.grp1 = nil, nil
	sh0 := nusizStartShift(fd.nusiz0)
	sh1 := nusizStartShift(fd.nusiz1)
	for y := 0; y < fd.h; y++ {
		x0 := fd.p0x - sh0 + nusizStartShift(fd.nz0[y])
		x1 := fd.p1x - sh1 + nusizStartShift(fd.nz1[y])
		k := -1
		if fd.zones != nil {
			k = fd.zoneOf(y)
		}
		b0 := grpByte(fd.tgtElem[y], x0, nusizWidth(fd.nz0[y]), "P0")
		b1 := grpByte(fd.tgtElem[y], x1, nusizWidth(fd.nz1[y]), "P1")
		if fd.zfollow[objP0] {
			b0 = fd.zoneGRP(y, k, objP0, x0)
		}
		if fd.zfollow[objP1] {
			b1 = fd.zoneGRP(y, k, objP1, x1)
		}
		fd.grp0 = append(fd.grp0, b0)
		fd.grp1 = append(fd.grp1, b1)
	}
}

// zoneGRP reads a followed player's byte for one line from the anchor its own zone
// was measured at. A line no zone replays (one a positioning block consumed) and a
// zone the object never appears in both give 0 — the kernel is not writing GRP
// there at all, and inventing a byte would paint a sprite the target has no pixel
// for. `fallback` is only used when there is no zone plan for this object.
func (fd *frameData) zoneGRP(y, k, i int, fallback int) uint8 {
	if k < 0 || fd.lx[i][y] == notDrawn {
		return 0
	}
	anchor := fd.zones[k].lt[i]
	if anchor == notDrawn {
		return 0
	}
	nz := fd.nz0[y]
	if i == objP1 {
		nz = fd.nz1[y]
	}
	return grpByte(fd.tgtElem[y], anchor, nusizWidth(nz), objNames[i])
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

// leftmostDrawn reads, per visible line, the leftmost clock at which an element
// actually appears in an attribution grid, or notDrawn. Taken off the picture, so
// the same function serves the target and the clone and the two numbers are
// commensurable — which the reset markers are not, being up to a pixel out.
func leftmostDrawn(grid [][]string, name string) []int {
	out := make([]int, len(grid))
	for y, row := range grid {
		out[y] = notDrawn
		for x, e := range row {
			if e == name {
				out[y] = x
				break
			}
		}
	}
	return out
}

// zoneLeftmost is the leftmost clock an object was drawn at anywhere inside a
// zone — the anchor the band's graphics bytes are read from and the position the
// clone is driven to. Taking the minimum over the band matters: a sprite whose top
// line is 2 pixels wide and whose middle line is 8 (measured on zone_multiplex:
// widths 2,4,6,8,8,6,4 down every band) reveals its left edge only on the widest
// lines, and anchoring on a narrow line would shift the whole band.
func zoneLeftmost(lx [5][]int, z zone, i int) int {
	best := notDrawn
	if lx[i] == nil {
		return best
	}
	for y := z.start; y <= z.end && y < len(lx[i]); y++ {
		if y < 0 || lx[i][y] == notDrawn {
			continue
		}
		if best == notDrawn || lx[i][y] < best {
			best = lx[i][y]
		}
	}
	return best
}

// lineBaseX is the reset X countRuns would record for this line, or notDrawn.
// Kept in one place so the per-line series and the histogram can never disagree
// about what "the position on this line" means.
func lineBaseX(runs []emu.ElemRun, name string, nz uint8, x int) int {
	for _, r := range runs {
		if r.Element == name {
			return x - nusizStartShift(nz)
		}
	}
	return notDrawn
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
func renderGrid(src, spec string, frames int) (grid [][]string, top int, obj [5]objFacts, bx [5][]int, err error) {
	for i := range obj {
		obj[i] = newObjFacts(objNames[i])
	}
	dir, err := os.MkdirTemp("", "framegen")
	if err != nil {
		return nil, 0, obj, bx, err
	}
	asm, bin := dir+"/c.asm", dir+"/c.bin"
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		return nil, 0, obj, bx, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return nil, 0, obj, bx, fmt.Errorf("assemble:\n%s", out)
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, 0, obj, bx, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, 0, obj, bx, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, 0, obj, bx, err
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
	for i := range bx {
		bx[i] = make([]int, h)
		for y := range bx[i] {
			bx[i][y] = notDrawn
		}
	}
	if st, merr := measureLines(e); merr == nil {
		for y := 0; y < h; y++ {
			ls, ok := st[vt+y]
			if !ok {
				continue
			}
			countRuns(&obj[0], runsPerLine[y], "P0", ls.nz0, ls.x0)
			countRuns(&obj[1], runsPerLine[y], "P1", ls.nz1, ls.x1)
			bx[objP0][y] = lineBaseX(runsPerLine[y], "P0", ls.nz0, ls.x0)
			bx[objP1][y] = lineBaseX(runsPerLine[y], "P1", ls.nz1, ls.x1)
		}
	}
	for y := 0; y < h; y++ {
		for i, name := range []string{"M0", "M1", "BL"} {
			if x, w, ok := firstRun(runsPerLine[y], name); ok {
				countRuns(&obj[objM0+i], runsPerLine[y], name, 0, x)
				obj[objM0+i].widths[w]++
				bx[objM0+i][y] = x
			}
		}
	}
	for i := range obj {
		obj[i].distinctX = len(obj[i].baseX)
	}
	return grid, vt, obj, bx, nil
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
// The block COUNT is capped at what `cyclebound` can certify. It used to be capped
// LOWER than that, and the gap cost a write block on every scanline.
//
// Every table is `align 256` and the index never exceeds 255, so `lda TABLE,y`
// cannot cross a page and costs 4. This code charged 5 anyway and capped at 8
// blocks, because the prover "cannot assume the alignment" — true when it was
// written, and fixed in the prover on 2026-08-01: `pagePenalty` now returns 0 for an
// absolute,X/absolute,Y read whose base has a zero low byte, which needs no index
// analysis at all (a 6502 index is 0..255, so $NN00+idx stays in $NN's page).
//
// Re-measured 2026-08-05 before changing anything: the eight-block Fishing Derby
// clone certifies at **worst 66 cycles**, which is 3+7*8+7 — the prover was already
// counting the blocks at 7. The constant was the only thing still counting them at
// 8, and it was throwing away the ninth block against a limitation that no longer
// existed. Nine blocks are 3+7*9+7 = 73 of 76.
//
// The caution that produced the old cap stays worth keeping: emit only kernels this
// repo's own prover accepts, because a generated artifact that trips the line-budget
// gate is a landmine, and RL-7b exists because exactly that shipped once. So the cap
// is still the prover's number — it is just no longer a stale copy of it.
const (
	kernBlockCycles = 7 // lda abs,y (4, tables are page-aligned) + sta zp (3)
	kernFixedCycles = 3 + 7
	// kernBlockCeiling is the most blocks a scanline can hold: 3+7*9+7 = 73 of 76.
	kernBlockCeiling = (76 - kernFixedCycles) / kernBlockCycles
)

// kernMaxBlocks is SEARCHED, not fixed, because more blocks is not monotonically
// better and that was measured rather than assumed.
//
// Raising the cap from 8 to the prover's true ceiling of 9 on Fishing Derby bought
// M1 outright — 0 of 43 cells to 43 of 43 — and cost 2,025 background cells, because
// the ninth slot let a playfield write be scheduled past the beam it governs: PF went
// from drawing 6,872 cells to 8,829 against a target of 6,888. Net element match fell
// 33,637 -> 31,680. On other targets the ninth block helps; the old comment here
// recorded a nine-block clone whose picture improved.
//
// So the count joins the content shift, the VBLANK top and the frame length as
// something this tool CALIBRATES against the target instead of deriving. Both
// candidates certify under cyclebound (66 and 73 cycles against 76), so the choice is
// purely about the picture, and the search reports what it picked and what it beat.
var kernMaxBlocks = kernBlockCeiling

func blockLands(i int) int { return 21*(i+1) - 68 } // i is 0-based

// placeable reports whether this kernel can put the object where the target has
// it: it must be drawn at all, and either sit at ONE reset X for the whole frame
// or be followed by the zone plan (fd.zfollow), because outside a zone block the
// kernel strobes each RESxx once. An object that moves and cannot be followed is
// measured, reported and not drawn — never approximated to its first position,
// which would put pixels somewhere the target never had them.
func (fd *frameData) placeable(i int) bool {
	return fd.placeableWith(i, fd.zfollow)
}

func (fd *frameData) placeableWith(i int, follow [5]bool) bool {
	o := fd.obj[i]
	if i == objP0 || i == objP1 {
		return true // the players are always placed; their X is the calibrated marker
	}
	return o.lines > 0 && (o.distinctX == 1 || follow[i])
}

// wantX is the position the calibration drives this object to OUTSIDE a zone
// plan. Players use the end-of-frame marker the whole kernel is calibrated
// against; missiles and the ball use the single reset X measured from the pixels
// they drew.
func (fd *frameData) wantX(i int) int {
	switch i {
	case objP0:
		return fd.p0x
	case objP1:
		return fd.p1x
	}
	return fd.obj[i].onlyX
}

// zone is one band of visible lines the replay loop runs over without moving
// anything, together with the reset X every followed object holds during it.
//
// A zone boundary costs whole scanlines, which is the entire reason the plan is
// measured rather than assumed: repositioning is `sta WSYNC` + a div-15 subtract
// loop per object plus one line for the HMOVE strobe, and the lines it eats have
// to be lines the target draws nothing on — the replay loop is not running on
// them, so the only picture reproducible there is no picture.
type zone struct {
	start, end int    // inclusive visible-line indices this zone's loop replays
	posStart   int    // first visible line eaten by the positioning block before it (-1 for zone 0)
	x          [5]int // reset X each followed object holds here (notDrawn = never drawn in this zone)
	lt         [5]int // leftmost clock the target actually drew it at in this zone (notDrawn = not drawn)
}

// zoneWantX is the position the calibration drives object i to in zone k, as the
// leftmost clock the target drew it at there. A followed object inherits the
// previous zone's anchor when it draws nothing in its own (the position is a
// register, it does not stop existing); everything else keeps its single
// frame-wide target, measured the way it always was.
func (fd *frameData) zoneWantX(k, i int) int {
	if fd.zones == nil || !fd.zfollow[i] {
		return fd.wantX(i)
	}
	for ; k >= 0; k-- {
		if v := fd.zones[k].lt[i]; v != notDrawn {
			return v
		}
	}
	// Never drawn up to and including zone k: fall forward to the first zone that
	// does draw it, so the object is parked somewhere it was actually measured.
	for _, z := range fd.zones {
		if z.lt[i] != notDrawn {
			return z.lt[i]
		}
	}
	return fd.wantX(i)
}

// zoneOf is the zone whose replay loop covers visible line y, or -1 when the line
// is one a positioning block consumed.
func (fd *frameData) zoneOf(y int) int {
	for k, z := range fd.zones {
		if y >= z.start && y <= z.end {
			return k
		}
	}
	return -1
}

// zonePosLines is what one positioning block costs in scanlines: one per placed
// object (the inline block opens with `sta WSYNC`, so it owns its line) plus one
// for the HMOVE strobe line.
//
// EVERY placeable object is repositioned in every block, including ones that did
// not move. That is not waste, it is correctness: HMOVE applies whatever the HMxx
// registers hold, so an object left out of the block would be moved a SECOND time
// by its stale prologue nibble. Rewriting all of them costs a line and needs no
// HMCLR, which would otherwise have to run before the first div-15 loop and push
// its `sta.w RESxx` past the end of the scanline.
func (fd *frameData) zonePosLines(follow [5]bool) int {
	n := 1
	for i := range fd.obj {
		if fd.placeableWith(i, follow) {
			n++
		}
	}
	return n
}

// tryZones builds the zone plan for one candidate follow-set, or reports the
// object and the measured numbers that make it impossible.
func tryZones(fd *frameData, follow [5]bool) ([]zone, int, string) {
	posLines := fd.zonePosLines(follow)
	var zs []zone
	cur := zone{start: 0, end: 0, posStart: -1}
	for i := range cur.x {
		cur.x[i] = notDrawn
	}
	pin := func(z *zone, y int) int {
		for i := 0; i < 5; i++ {
			if !follow[i] {
				continue
			}
			v := fd.bx[i][y]
			if v == notDrawn {
				continue
			}
			if z.x[i] == notDrawn {
				z.x[i] = v
				continue
			}
			if z.x[i] != v {
				return i
			}
		}
		return -1
	}
	for y := 0; y < fd.h; y++ {
		conflict := pin(&cur, y)
		if conflict < 0 {
			cur.end = y
			continue
		}
		// The block has to finish before line y, so it owns [y-posLines, y-1], and
		// the line before it must be background-only too: that is the last line the
		// replay loop runs, and it is what leaves GRP0/GRP1/PF at 0 so the
		// positioning lines show background rather than the previous zone's picture.
		ps := y - posLines
		if have := fd.heldRun(y - 1); have < posLines+1 {
			return nil, posLines, fmt.Sprintf("%s changes reset X from %d to %d at visible line %d, "+
				"but only %d of the %d holdable lines the move needs are there (%d objects to place at one "+
				"scanline each + 1 HMOVE line + 1 line whose registers the block then holds). A holdable line is "+
				"one the target repeats, since the replay loop is stopped during the block",
				objNames[conflict], cur.x[conflict], fd.bx[conflict][y], y, have, posLines+1, posLines-1)
		}
		if ps-1 < cur.start {
			return nil, posLines, fmt.Sprintf("%s changes reset X from %d to %d at visible line %d, "+
				"and the %d-line positioning block would start at line %d, before this zone's own first line %d — "+
				"the zone would replay nothing",
				objNames[conflict], cur.x[conflict], fd.bx[conflict][y], y, posLines, ps, cur.start)
		}
		cur.end = ps - 1
		zs = append(zs, cur)
		nz := zone{start: y, end: y, posStart: ps}
		for i := range nz.x {
			nz.x[i] = notDrawn
		}
		if c := pin(&nz, y); c >= 0 {
			// Two positions on ONE line: no amount of blank lines can serve that.
			return nil, posLines, fmt.Sprintf("%s is drawn at two reset X on the single visible line %d, "+
				"which no per-zone kernel can express", objNames[c], y)
		}
		cur = nz
	}
	cur.end = fd.h - 1
	zs = append(zs, cur)
	for k := range zs {
		for i := range zs[k].lt {
			zs[k].lt[i] = zoneLeftmost(fd.lx, zs[k], i)
		}
	}
	return zs, posLines, ""
}

// planZones decides which objects the kernel will follow per zone, and reports
// every one it drops with the measurement that ruled it out.
//
// Only an object measured at MORE THAN ONE reset X can force a zone boundary, so
// a target with one position per object produces no zones at all and the
// historical single-position kernel is emitted unchanged — that is the property
// that keeps 21 pixel-exact clones byte-identical.
func planZones(fd *frameData) ([]zone, [5]bool, []string) {
	var follow [5]bool
	any := false
	for i := range fd.obj {
		if fd.obj[i].lines > 0 && fd.obj[i].distinctX > 1 {
			follow[i] = true
			any = true
		}
	}
	if !any {
		return nil, follow, nil
	}
	var notes []string
	for {
		zs, posLines, bad := tryZones(fd, follow)
		if bad == "" {
			var desc []string
			for k, z := range zs {
				desc = append(desc, fmt.Sprintf("z%d L%d-%d", k, z.start, z.end))
			}
			notes = append(notes, fmt.Sprintf("zone kernel: %d zones (%s), repositioning %d objects + 1 HMOVE line = %d scanlines per boundary, taken from measured background-only gaps",
				len(zs), strings.Join(desc, " "), posLines-1, posLines))
			return zs, follow, notes
		}
		// Drop the object the plan failed on and retry: losing one multiplexed object
		// does not have to cost the others. A dropped player reverts to the single
		// position it was always given; a dropped missile or ball is not drawn at all,
		// which the coverage table then reports as structurally absent.
		drop := -1
		for i := range follow {
			if follow[i] && strings.HasPrefix(bad, objNames[i]+" ") {
				drop = i
				break
			}
		}
		if drop < 0 {
			for i := range follow {
				if follow[i] {
					drop = i
					break
				}
			}
		}
		if drop < 0 {
			return nil, follow, notes
		}
		follow[drop] = false
		fd.zdrop[drop] = bad
		notes = append(notes, fmt.Sprintf("NOT REPRODUCED: per-zone X for %s. Measured: %s. Bands: %s",
			objNames[drop], bad, runList(xRuns(fd.bx[drop]), 8)))
		still := false
		for _, f := range follow {
			still = still || f
		}
		if !still {
			return nil, follow, notes
		}
	}
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

// resetRegs/moveRegs name the strobe and motion register of each object. RESP0..
// RESBL are $10-$14 and HMP0..HMBL are $20-$24, which is why SetXPos can index
// both from one base — but an INLINE block writes them by name, because the two
// cycles an `ldx #n` costs move the RESxx strobe 6 pixels and would have to be
// calibrated out again.
var resetRegs = [5]string{"RESP0", "RESP1", "RESM0", "RESM1", "RESBL"}
var moveRegs = [5]string{"HMP0", "HMP1", "HMM0", "HMM1", "HMBL"}

// A zone positioning block places one object per scanline with a FIXED-cost
// delay chain, not with the prologue's div-15 subtract loop, and the reason is a
// measured scanline:
//
//	sta WSYNC | n x nop 2n | sta.w RESxx 4 | lda #nib 2 | sta HMxx 3 | sta WSYNC 3
//
// so the RESxx write lands on cycle 2n+3 and the block's CLOSING `sta WSYNC`
// writes on cycle 2n+11, which has to be <= 75 for the block to own exactly one
// scanline. A div-15 block costs 5k+19 to the strobe and 5k+22 to that closing
// write, i.e. it needs k <= 10 — and k <= 10 only reaches reset X 6..155, measured.
// zone_multiplex asks for X=4. Pushing to k=11 to reach it made one block spend
// two scanlines instead of one, which showed up as a 263-line frame and a picture
// the shift search wanted to move 2 lines: 72 cells wrong that were not a
// positioning error at all.
//
// The delay chain also has no branch, so there is no page-crossing cycle to
// account for; `nop` is one byte and two cycles wherever it sits.
const (
	zoneNopCycles   = 2 // one nop
	zoneStrobeFixed = 3 // sta.w RESxx write, relative to the end of the chain
	zoneTailCycles  = 8 // lda #nib 2 + sta HMxx 3 + sta WSYNC 3
	zoneMaxNops     = (75 - zoneStrobeFixed - zoneTailCycles) / zoneNopCycles
	// zoneInputMax: the input is in pixels (see zoneCoarseFine), 6 per nop plus the
	// 0..5 the HMxx nibble covers.
	zoneInputMax = zoneMaxNops*6 + 5
)

// zoneCoarseFine splits a position input into the delay-chain length and the HMxx
// nibble. One nop moves the RESxx strobe 2 CPU cycles = 6 colour clocks, and the
// remaining 0..5 are made up by moving the object RIGHT with the motion register:
// HMxx is upper-nibble two's complement with positive = LEFT, so a right move of f
// is the nibble 16-f ($F0 = right 1 ... $B0 = right 5). Input therefore reads in
// pixels — +1 input is +1 pixel right — the same way the prologue's input does.
func zoneCoarseFine(in int) (nops int, nib uint8) {
	nops = in / 6
	f := in % 6
	return nops, uint8((16-f)&0x0F) << 4
}

func zoneStrobeCycle(in int) int {
	n, _ := zoneCoarseFine(in)
	return zoneNopCycles*n + zoneStrobeFixed
}

// zonePositionCode is one positioning block: one fixed-cost placement per
// placeable object, then the HMOVE strobe on its own line, then the replay index
// stepped over the lines the block consumed so the tables stay single arrays.
func zonePositionCode(fd *frameData, k int, in [5]int) string {
	z := fd.zones[k]
	var b strings.Builder
	fmt.Fprintf(&b, "    ; --- reposition for zone %d. Target lines %d-%d draw nothing at all\n"+
		"    ; (measured), so the replay loop gives them up: %d objects at one scanline\n"+
		"    ; each + 1 HMOVE line. Line %d is the last line replayed, and it is blank,\n"+
		"    ; which is what leaves GRP0/GRP1/PF at 0 across the block. ---\n",
		k, z.posStart, z.start-1, z.start-z.posStart-1, z.posStart-1)
	for i := range fd.obj {
		if !fd.placeable(i) {
			continue
		}
		nops, nib := zoneCoarseFine(in[i])
		// "leftmost drawn clock", not "reset X": the two differ by up to a pixel with
		// the fine nibble (measured), and this number is the one the calibration
		// actually drove the clone onto — the leftmost pixel the target drew here.
		fmt.Fprintf(&b, "    sta WSYNC           ; %s -> leftmost drawn clock %d: %s strobes on cycle %d of 76\n",
			objNames[i], fd.zoneWantX(k, i), resetRegs[i], zoneStrobeCycle(in[i]))
		if nops > 0 {
			fmt.Fprintf(&b, "    REPEAT %d\n    nop\n    REPEND              ; %d nop = %d cycles of coarse delay\n",
				nops, nops, zoneNopCycles*nops)
		}
		fmt.Fprintf(&b, "    sta.w %s\n    lda #$%02X\n    sta %-6s      ; fine move %d px right\n",
			resetRegs[i], nib, moveRegs[i], in[i]%6)
	}
	fmt.Fprintf(&b, "    sta WSYNC\n    sta HMOVE\n    ldy #%d              ; skip the %d lines the block consumed\n",
		z.start, z.start-z.posStart)
	return b.String()
}

// kernelBody emits the replay loop. With no zone plan it emits exactly the single
// loop it always did, byte for byte; with one it emits a loop per zone separated
// by positioning blocks, all sharing one set of page-aligned tables indexed by the
// SAME y that keeps counting across the boundaries.
func kernelBody(fd *frameData, zin [][5]int, kb string) string {
	if fd.zones == nil {
		return fmt.Sprintf("    ; --- visible: replay %d scanlines ---\n    ldy #0\nKern:\n    sta WSYNC\n%s    iny\n    cpy #%d\n    bne Kern\n",
			fd.h, kb, fd.h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "    ; --- visible: replay %d scanlines as %d zones, repositioning between them ---\n    ldy #0\n", fd.h, len(fd.zones))
	for k, z := range fd.zones {
		if k > 0 {
			b.WriteString(zonePositionCode(fd, k, zin[k]))
		}
		fmt.Fprintf(&b, "Kern%d:\n    sta WSYNC\n%s    iny\n    cpy #%d\n    bne Kern%d\n", k, kb, z.end+1, k)
	}
	return b.String()
}

// zoneEquates are the strobe/motion registers only an inline zone block names.
// Emitted ONLY when there is a zone plan: the single-position kernel's source must
// stay byte-identical to what it has always been, and an unused equate is a diff.
func zoneEquates(fd *frameData) string {
	if fd.zones == nil {
		return ""
	}
	return "RESM0  = $12\nRESM1  = $13\nRESBL  = $14\nHMM0   = $22\nHMM1   = $23\nHMBL   = $24\n"
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
	if !vary0 && !vary1 && len(extra) == 0 && fd.zones == nil {
		// Historical order, unchanged: GRP0, PF left, GRP1, PF right.
		return []writeBlock{g0, pf[0], pf[1], pf[2], g1, pf[3], pf[4], pf[5]}, nil
	}
	// A zone plan invalidates the historical order, and the number that says so is
	// the deadline. GRP1 sits fifth there and lands at visible clock +37, which is
	// early enough for a player parked on the right — but zone_multiplex moves P1 to
	// reset X 4, and a store at +37 leaves that band drawing the previous line's
	// byte. Measured: 64 P1 cells wrong with the historical order, and the
	// deadline-sorted layout below puts GRP1 in a slot that lands at -26.

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
		notes = append(notes, fmt.Sprintf("NOT REPRODUCED: per-line %s. The kernel would need %d write blocks and only %d fit — 3+%d*n+7 CPU cycles against a 76-cycle scanline, so %d blocks cost %d and %d would cost %d. The tables are `align 256`, so `lda TABLE,y` cannot cross a page and is costed at 4 by both the hardware and (since 2026-08-01) this repo's prover; the cap is the prover's own number, not a safety margin on top of it",
			strings.Join(dropped, " and "), needed, kernMaxBlocks, kernBlockCycles,
			kernMaxBlocks, 3+kernBlockCycles*kernMaxBlocks+7,
			kernMaxBlocks+1, 3+kernBlockCycles*(kernMaxBlocks+1)+7))
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

// emit builds the full clone source. zin[k] holds the position-routine inputs for
// zone k (zin[0] = the prologue's SetXPos calls) and vblankAdj shifts the VBLANK
// line count — all self-calibrated by the caller.
func emit(fd *frameData, zin [][5]int, vblankAdj, osAdj int) string {
	in := zin[0]
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
%sHMOVE  = $2A
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
%s    sta WSYNC           ; end the last visible line BEFORE clearing anything:
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

`, zoneEquates(fd), fd.p0col, fd.p1col, fd.pfcol, fd.bgcol,
		fd.nusiz0|fd.msz0, fd.nusiz1|fd.msz1, fd.bsz, ctrlpfNote, staticEnables(fd),
		positionCode(fd, in), vblank, kernelBody(fd, zin, kb.String()), nusizRestore, overscan)

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
	// WRAP, do not clamp. The div-15 routine positions modulo the 160-clock line, so
	// an input below the reachable floor has an equivalent 160 higher — and clamping
	// to 0 instead does not merely lose precision, it STALLS the calibration at a
	// wrong position and then reports the residue as a cause "this tool has not
	// measured".
	//
	// Measured cost of getting this wrong: litmus_nusiz_all went from 2 mismatched
	// cells to 1868 when the clamp was introduced, because P1's target reset X of 4
	// needs input -3 and the clamp pinned it at 0 (calibration stopped at
	// "P1 7(want 4,d-3)"). The regression went unseen for the same reason it was
	// possible: the corpus sweep globs roms/techniques/*.asm and this litmus lives in
	// roms/litmus/, so the ROM added BECAUSE it lights axes nothing else does was
	// outside the regression set.
	for v < 0 {
		v += 160
	}
	for v > 255 {
		v -= 160
	}
	return v
}

func renderClonePositions(src, spec string, frames int) (x [5]int, lx [5][]int, top int, err error) {
	p0, p1, t, err := renderClone(src, spec, frames)
	if err != nil {
		return x, lx, 0, err
	}
	x[objP0], x[objP1] = p0, p1
	grid, _, obj, _, err := renderGrid(src, spec, frames)
	if err != nil {
		return x, lx, t, err
	}
	for i := range lx {
		lx[i] = leftmostDrawn(grid, objNames[i])
	}
	for i := objM0; i <= objBL; i++ {
		if obj[i].lines == 0 {
			x[i] = -1 // never drawn: distinct from "drawn at clock 0"
			continue
		}
		x[i] = obj[i].onlyX
	}
	return x, lx, t, nil
}

// wrapZoneInput keeps an inline zone block inside its own scanline. Unlike the
// prologue's SetXPos — whose overrun the frame-length calibration absorbs — a zone
// block that runs long steals a scanline from the picture, so the bound is hard.
//
// It WRAPS rather than clamping, because clamping cannot reach the left edge and
// wrapping can. The div-15 loop always runs at least once, so input 0 is the
// leftmost coarse step with the largest available left nibble, and it was measured
// at reset X 6 on zone_multiplex — two pixels short of the band at 4, and clamping
// left those 7 lines permanently wrong. The object counter wraps at 160, so
// input+160 asks for the same column from the other side; the input range 0..164
// is 165 values wide, one more than a full wrap, so every column is reachable.
func wrapZoneInput(v int) int {
	for v < 0 {
		v += 160
	}
	for v > zoneInputMax {
		v -= 160
	}
	if v < 0 {
		return 0
	}
	return v
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
	// The band structure, printed whenever an object took more than one position.
	// This is the measurement the zone plan is built from, so it is shown before the
	// plan rather than only when the plan fails: L<a>-<b>(<n>L)@x<X> gap<G> reads as
	// "drawn on lines a..b at reset X, with G lines drawn-nothing immediately before".
	for i := range fd.obj {
		if fd.obj[i].distinctX > 1 {
			rs := xRuns(fd.bx[i])
			fmt.Printf("  bands %s: %d bands over %d lines — %s\n",
				objNames[i], len(rs), fd.obj[i].lines, runList(rs, 12))
		}
	}
	// The zone plan has to be decided before anything that depends on where the
	// sprites will be: which objects are placeable, and where the graphics bytes are
	// sampled from.
	zoneBlocks, zoneFollow, planNotes := planZones(fd)
	fd.zones, fd.zfollow = zoneBlocks, zoneFollow
	fd.buildGRP()
	kern, kernNotes := planKernel(fd)
	fd.kern = kern
	planNotes = append(planNotes, kernNotes...)
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
	// One input row per zone; with no zone plan there is exactly one row and this is
	// the loop it always was. A zone row is calibrated independently because the
	// clone is the only authority on where a given input lands, but it starts from
	// the same 40 so a target with one zone and a target with six begin identically.
	nz := 1
	if fd.zones != nil {
		nz = len(fd.zones)
	}
	zin := make([][5]int, nz)
	for k := range zin {
		zin[k] = [5]int{40, 40, 40, 40, 40}
	}
	stuck := make([][5]bool, nz)
	misses := make([][5]int, nz)
	vblankAdj, osAdj := 0, 0
	lastSig := ""
	for iter := 0; iter < 24; iter++ {
		src := emit(fd, zin, vblankAdj, osAdj)
		got, gotlx, top, err := renderClonePositions(src, *spec, *frames)
		if err != nil {
			fmt.Fprintln(os.Stderr, "calibrate:", err)
			os.Exit(2)
		}
		dtop := fd.top - top
		done := dtop == 0
		var parts []string
		for k := 0; k < nz; k++ {
			for i := range fd.obj {
				if !fd.placeable(i) || stuck[k][i] {
					continue
				}
				want := fd.zoneWantX(k, i)
				// With no zone plan the measurement is the frame-boundary marker, as it
				// always was. With one, both sides are the LEFTMOST CLOCK THE OBJECT WAS
				// DRAWN AT inside that zone — the marker is up to a pixel away from the
				// drawn window (measured: the same sprite line reads reset X 49 with span
				// 49..56, and reset X 117 with span 116..123), so driving the clone's
				// marker onto the target's marker leaves the picture a pixel out.
				have := got[i]
				label := objNames[i]
				if fd.zones != nil {
					have = zoneLeftmost(gotlx, fd.zones[k], i)
					label = fmt.Sprintf("z%d%s", k, objNames[i])
					if !fd.zfollow[i] && k > 0 {
						// Not followed: one position for the whole frame. The later zones
						// re-place it to the same input, so calibrating it once is enough
						// and calibrating it per zone would fight itself.
						zin[k][i] = zin[0][i]
						continue
					}
				}
				if have < 0 {
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
					misses[k][i]++
					if misses[k][i] >= 4 {
						stuck[k][i] = true
					}
					done = false
					parts = append(parts, fmt.Sprintf("%s NOT-DRAWN(want %d, miss %d)", label, want, misses[k][i]))
					continue
				}
				misses[k][i] = 0
				d := want - have
				if d != 0 {
					done = false
				}
				if fd.zones != nil && k > 0 {
					zin[k][i] = wrapZoneInput(zin[k][i] + d)
				} else {
					zin[k][i] = clampInput(zin[k][i] + d) // +1 input ≈ +1px right near the target
				}
				parts = append(parts, fmt.Sprintf("%s %d(want %d,d%+d)", label, have, want, d))
			}
		}
		fmt.Printf("  calib iter %d: %s top %d(want %d,d%+d) in=%v vbAdj=%d\n",
			iter, strings.Join(parts, " "), top, fd.top, dtop, zin, vblankAdj)
		if done {
			break
		}
		vblankAdj += dtop // more VBLANK lines → visible top moves later (down)
		// Stop when nothing moved. The loop is deterministic — same source, same
		// render — so an iteration that changes no input and no VBLANK count cannot
		// be followed by one that does, and the remaining rounds only cost time. The
		// fixed point, and therefore the emitted source, is identical either way;
		// what is left over is reported as a residual difference, not iterated at.
		sig := fmt.Sprint(zin, vblankAdj)
		if sig == lastSig {
			fmt.Printf("  calib: fixed point reached with a residual difference (see the report below)\n")
			break
		}
		lastSig = sig
	}

	// Content vertical-shift search: after the visible top matches, nudge the
	// replay tables ±lines so the rendered picture aligns line-for-line with the
	// target (kills the residual constant offset the top-match can't see).
	bestS, bestM := 0, -1
	for s := -4; s <= 4; s++ {
		sh := fd.shifted(s)
		if !sh.zonesValid() {
			// A shift that pushes a zone boundary out of the frame would emit a loop
			// whose `cpy` bound it never reaches. Skipped, and said so.
			fmt.Printf("  content shift %+d: skipped — the zone boundaries would leave the frame\n", s)
			continue
		}
		src := emit(sh, zin, vblankAdj, osAdj)
		grid, _, _, _, err := renderGrid(src, *spec, *frames)
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

	// Write-block count: the same measure-don't-derive treatment. The ceiling is what
	// the prover certifies; whether the last block HELPS is a property of the target,
	// so try each count and keep the picture that matches best.
	bestB, bestBM := kernMaxBlocks, bestM
	seenPlan := map[int]bool{len(fd.kern): true}
	for b := kernBlockCeiling; b >= 4; b-- {
		if b == kernMaxBlocks {
			fmt.Printf("  block cap %d: element match %d / %d (ceiling; kernel uses %d)\n", b, bestM, fd.h*160, len(fd.kern))
			continue
		}
		// A cap above what the target actually needs plans the same kernel, so only
		// caps that change the PLAN are worth rendering. Without this the loop spends
		// five renders re-scoring one kernel on every target that needs 8 or fewer.
		{
			savedCap := kernMaxBlocks
			kernMaxBlocks = b
			probe, _ := planKernel(fd)
			kernMaxBlocks = savedCap
			if seenPlan[len(probe)] {
				continue
			}
			seenPlan[len(probe)] = true
		}
		// planKernel is what CONSUMES the cap, and it runs once before this search
		// and stores its answer in fd.kern — so the count has to be re-planned, not
		// just re-emitted. Getting that wrong is silent: every candidate renders the
		// kernel that was already chosen and scores identically, which is exactly
		// what the first version of this loop reported before the plan was moved in.
		savedCap, savedKern := kernMaxBlocks, fd.kern
		kernMaxBlocks = b
		kern, _ := planKernel(fd)
		fd.kern = kern
		m := -1
		if src := emit(fd.shifted(bestS), zin, vblankAdj, osAdj); true {
			if grid, _, _, _, err := renderGrid(src, *spec, *frames); err == nil {
				m = matchCount(fd.tgtElem, grid)
			}
		}
		kernMaxBlocks, fd.kern = savedCap, savedKern
		if m < 0 {
			fmt.Printf("  block cap %d: did not render\n", b)
			continue
		}
		fmt.Printf("  block cap %d: element match %d / %d (kernel uses %d)\n", b, m, fd.h*160, len(kern))
		// TIES GO TO THE SMALLER KERNEL. The loop counts DOWN from the ceiling, so
		// `>=` hands a tie to the lower count — same picture, fewer cycles spent. On
		// the technique corpus most targets score identically at 9 and 8, and taking
		// the ceiling there would emit a 73-cycle kernel to draw what a 66-cycle one
		// draws, handing the author 7 cycles less headroom for nothing.
		if m >= bestBM {
			bestBM, bestB = m, b
		}
	}
	kernMaxBlocks = bestB
	if kern, _ := planKernel(fd); true {
		fd.kern = kern
	}
	// Report the CAP and the kernel it produced separately. They are not the same
	// number and conflating them reads as a regression: `bullets` plans 8 blocks at
	// 66 cycles whatever the cap is, and printing "chosen kernel blocks: 9" for it
	// says the kernel got bigger when nothing about it changed.
	fmt.Printf("  chosen block cap: %d — kernel uses %d write blocks, %d of 76 CPU cycles (element match %d / %d)\n",
		bestB, len(fd.kern), kernFixedCycles+kernBlockCycles*len(fd.kern), bestBM, fd.h*160)

	src := emit(fd.shifted(bestS), zin, vblankAdj, osAdj)

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
		src = emit(fd.shifted(bestS), zin, vblankAdj, osAdj)
	}

	// What did it actually reproduce? Measured per element against the clone's
	// own rendered frame, because "pixel-exact" on a target that draws no
	// missiles says nothing about a target that does.
	var cov []elemCoverage
	var cloneObj [5]objFacts
	cloneObj[0], cloneObj[1] = newObjFacts("P0"), newObjFacts("P1")
	if grid, _, co, _, err := renderGrid(src, *spec, *frames); err == nil {
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
	why := diagnose(cov, fd.obj, cloneObj, fd.kern, fd)
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
func diagnose(cov []elemCoverage, tgt, clone [5]objFacts, blocks []writeBlock, fd *frameData) string {
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
		switch {
		case t.distinctX > 1 && !fd.zfollow[i]:
			w := fmt.Sprintf("its reset X takes %d distinct values across the frame %s and this kernel strobes RESP%d once",
				t.distinctX, t.xList(), i)
			if fd.zdrop[i] != "" {
				w += fmt.Sprintf(" — a per-zone kernel was costed against the target's own gaps and declined: %s", fd.zdrop[i])
			}
			why = append(why, w)
		case t.distinctX > 1:
			// The kernel DID follow it. Saying "one X cannot follow a multiplexed
			// target" here would be the RL-7c failure again — a true statement about a
			// kernel that is not the one emitted. What is measurable is where the clone
			// landed per zone, so that is what is reported.
			var off []string
			for k, z := range fd.zones {
				if z.lt[i] == notDrawn {
					continue
				}
				off = append(off, fmt.Sprintf("z%d L%d-%d want x%d", k, z.start, z.end, z.lt[i]))
			}
			why = append(why, fmt.Sprintf("the kernel repositions it in %d zones (%s) and its cells still differ, "+
				"so the residue is not the single-position limit", len(fd.zones), strings.Join(off, " ")))
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
			// Reached only when nothing differs and nothing is over-drawn — i.e. the
			// clone matches. Saying "the difference is in BG cells only" there asserts a
			// difference that does not exist. Measured: no ROM in the 31-ROM corpus
			// reaches it today, so this is a sentence with no witness rather than a
			// visible defect; it is corrected instead of left as a trap for the first
			// target that does reach it.
			return "Every element matches and nothing is over-drawn; this diagnosis has no difference to " +
				"explain, so any cell count reported above comes from somewhere this function does not model."
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
