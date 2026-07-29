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
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// nusizWidth maps NUSIZ size-and-copies bits to pixels-per-GRP-bit (1/2/4).
func nusizWidth(sc uint8) int {
	switch sc & 0x07 {
	case 0x05:
		return 2
	case 0x07:
		return 4
	}
	return 1
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
	nusiz0, nusiz1   uint8
	p0x, p1x         int
	pf0L, pf1L, pf2L []uint8
	pf0R, pf1R, pf2R []uint8
	grp0, grp1       []uint8
	tgtElem          [][]string // target's per-visible-line object attribution (for content-shift search)
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
	return &c
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
	w0 := nusizWidth(fd.nusiz0)
	w1 := nusizWidth(fd.nusiz1)
	for y := 0; y < h; y++ {
		sl := top + y
		elem := make([]string, 160)
		for i := range elem {
			elem[i] = "BG"
		}
		if runs, _, err := e.DecomposeRow(sl); err == nil {
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
		fd.grp0 = append(fd.grp0, grpByte(elem, fd.p0x, w0, "P0"))
		fd.grp1 = append(fd.grp1, grpByte(elem, fd.p1x, w1, "P1"))
		fd.tgtElem = append(fd.tgtElem, elem)
	}
	return fd, nil
}

// renderGrid assembles src, renders it, and returns the per-visible-line object
// attribution grid + the Snapshot top (aligned to absolute scanline top+y).
func renderGrid(src, spec string, frames int) (grid [][]string, top int, err error) {
	dir, err := os.MkdirTemp("", "framegen")
	if err != nil {
		return nil, 0, err
	}
	asm, bin := dir+"/c.asm", dir+"/c.bin"
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		return nil, 0, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return nil, 0, fmt.Errorf("assemble:\n%s", out)
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, 0, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, 0, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, 0, err
	}
	img, vt := e.Snapshot()
	h := img.Bounds().Dy()
	for y := 0; y < h; y++ {
		row := make([]string, 160)
		for i := range row {
			row[i] = "BG"
		}
		if runs, _, err := e.DecomposeRow(vt + y); err == nil {
			for _, r := range runs {
				for x := r.Clock; x < r.Clock+r.Len && x < 160; x++ {
					row[x] = r.Element
				}
			}
		}
		grid = append(grid, row)
	}
	return grid, vt, nil
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

// emit builds the full clone source. p0in/p1in are the SetXPos routine inputs
// and vblankAdj shifts the VBLANK line count (all self-calibrated by the caller).
func emit(fd *frameData, p0in, p1in, vblankAdj, osAdj int) string {
	nlines := fd.h
	vblank := fd.top - 3 - 3 + vblankAdj // 3 lines used for positioning (P0,P1,HMOVE) below
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
	if overscan < 1 {
		overscan = 1
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
    lda #0
    sta CTRLPF          ; repeat mode: independent halves via per-line rewrite
    sta REFP0
    sta REFP1

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
    ; --- position the two players (SetXPos: div-15 coarse + HMOVE fine) ---
    ldx #0
    lda #%d
    jsr SetXPos
    ldx #1
    lda #%d
    jsr SetXPos
    sta WSYNC
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
    lda GRP0T,y
    sta GRP0
    lda PF0LT,y
    sta PF0
    lda PF1LT,y
    sta PF1
    lda PF2LT,y
    sta PF2
    lda GRP1T,y
    sta GRP1
    lda PF0RT,y
    sta PF0
    lda PF1RT,y
    sta PF1
    lda PF2RT,y
    sta PF2
    iny
    cpy #%d
    bne Kern
    sta WSYNC           ; end the last visible line BEFORE clearing anything:
    lda #0              ; without it the cleanup runs inside that line and clears
    sta GRP0            ; GRP0 at beam clock +133 and GRP1 at +142 (measured),
    sta GRP1            ; erasing any sprite pixel right of there on that one line
    sta PF0
    sta PF1
    sta PF2
    lda #2
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

`, fd.p0col, fd.p1col, fd.pfcol, fd.bgcol, fd.nusiz0, fd.nusiz1,
		p0in, p1in, vblank, nlines, nlines, overscan)

	sb.WriteString(byteTable("PF0LT", fd.pf0L))
	sb.WriteString(byteTable("PF1LT", fd.pf1L))
	sb.WriteString(byteTable("PF2LT", fd.pf2L))
	sb.WriteString(byteTable("PF0RT", fd.pf0R))
	sb.WriteString(byteTable("PF1RT", fd.pf1R))
	sb.WriteString(byteTable("PF2RT", fd.pf2R))
	sb.WriteString(byteTable("GRP0T", fd.grp0))
	sb.WriteString(byteTable("GRP1T", fd.grp1))
	sb.WriteString("\n    org $FFFC\n    .word Reset\n    .word Reset\n")
	return sb.String()
}

// renderClone assembles src, renders it, and returns the clone's P0/P1 landing X
// and its Snapshot visible top (for vertical calibration).
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

	// Self-calibrate: the two SetXPos inputs (players land on the target's
	// columns — the landing offset is kernel-specific) AND the VBLANK line count
	// (the clone's visible top matches the target's, so content aligns vertically).
	// osAdj corrects the overscan count once the frame length can be measured; it
	// stays 0 through the X/VBLANK calibration, which does not depend on it.
	p0in, p1in, vblankAdj, osAdj := 40, 40, 0, 0
	for iter := 0; iter < 16; iter++ {
		src := emit(fd, p0in, p1in, vblankAdj, osAdj)
		gp0, gp1, top, err := renderClone(src, *spec, *frames)
		if err != nil {
			fmt.Fprintln(os.Stderr, "calibrate:", err)
			os.Exit(2)
		}
		d0, d1, dtop := fd.p0x-gp0, fd.p1x-gp1, fd.top-top
		fmt.Printf("  calib iter %d: P0 %d(want %d,d%+d) P1 %d(want %d,d%+d) top %d(want %d,d%+d) in0=%d in1=%d vbAdj=%d\n",
			iter, gp0, fd.p0x, d0, gp1, fd.p1x, d1, top, fd.top, dtop, p0in, p1in, vblankAdj)
		if d0 == 0 && d1 == 0 && dtop == 0 {
			break
		}
		p0in += d0 // +1 input ≈ +1px right near the target
		p1in += d1
		vblankAdj += dtop // more VBLANK lines → visible top moves later (down)
	}

	// Content vertical-shift search: after the visible top matches, nudge the
	// replay tables ±lines so the rendered picture aligns line-for-line with the
	// target (kills the residual constant offset the top-match can't see).
	bestS, bestM := 0, -1
	for s := -4; s <= 4; s++ {
		src := emit(fd.shifted(s), p0in, p1in, vblankAdj, osAdj)
		grid, _, err := renderGrid(src, *spec, *frames)
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

	src := emit(fd.shifted(bestS), p0in, p1in, vblankAdj, osAdj)

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
		src = emit(fd.shifted(bestS), p0in, p1in, vblankAdj, osAdj)
	}

	// What did it actually reproduce? Measured per element against the clone's
	// own rendered frame, because "pixel-exact" on a target that draws no
	// missiles says nothing about a target that does.
	var cov []elemCoverage
	if grid, _, err := renderGrid(src, *spec, *frames); err == nil {
		// fd.tgtElem, not the shifted copy: shifted() moves the REPLAY tables, and
		// the target's own attribution is the reference the shift was chosen against.
		cov = coverage(fd.tgtElem, grid)
	} else {
		fmt.Fprintln(os.Stderr, "coverage: could not render the clone:", err)
	}
	// Frame length is not something the picture comparison can see: a clone can be
	// pixel-exact and still emit 264 scanlines, which rolls on a real television.
	// It went unnoticed for exactly that reason, so it is measured here every run.
	lines, lerr := cloneFrameLines(src, *spec)

	miss := missingElements(cov)
	src = annotate(src, cov, miss)

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
	// line: a kernel that carries one X per player cannot follow a multiplexed
	// target that moves its sprites per zone, and that shows up here as a cell
	// count, not as a missing element.
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
			"than the target's. Every element is present, so this is placement, not omission "+
			"(one X per player cannot follow a per-zone multiplexed target).\n", short, len(fd.tgtElem)*160)
		os.Exit(1)
	}
	fmt.Println("RESULT: pixel-exact — every visible cell is drawn by the same element as the target")
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
func annotate(src string, cov []elemCoverage, miss []string) string {
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
	b.WriteString("; ---------------------------------------------------------------------\n")
	const anchor = "; ==========================================================================\n    processor 6502\n"
	if i := strings.Index(src, anchor); i >= 0 {
		return src[:i+len("; ==========================================================================\n")] +
			b.String() + src[i+len("; ==========================================================================\n"):]
	}
	return b.String() + src
}
