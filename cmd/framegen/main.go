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
func emit(fd *frameData, p0in, p1in, vblankAdj int) string {
	nlines := fd.h
	vblank := fd.top - 3 - 3 + vblankAdj // 3 lines used for positioning (P0,P1,HMOVE) below
	if vblank < 1 {
		vblank = 1
	}
	overscan := 262 - fd.top - nlines
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
    lda #0
    sta GRP0
    sta GRP1
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
	p0in, p1in, vblankAdj := 40, 40, 0
	for iter := 0; iter < 16; iter++ {
		src := emit(fd, p0in, p1in, vblankAdj)
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
		src := emit(fd.shifted(s), p0in, p1in, vblankAdj)
		grid, _, err := renderGrid(src, *spec, *frames)
		if err != nil {
			continue
		}
		m := matchCount(fd.tgtElem, grid)
		fmt.Printf("  content shift %+d: element match %d / %d\n", s, m, fd.h*160)
		if m > bestM {
			bestM, bestS = m, s
		}
	}
	fmt.Printf("  chosen content shift: %+d\n", bestS)

	src := emit(fd.shifted(bestS), p0in, p1in, vblankAdj)
	if err := os.WriteFile(*out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(2)
	}
	fmt.Printf("wrote %s (%d bytes source)\n", *out, len(src))
}
