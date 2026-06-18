// Package spritepos is a forward sprite-position solver (authoring aid): given a
// target X (0..159) it returns the coarse/fine decomposition, a paste-able snippet,
// and — the part that makes it trustworthy — the position the hardware ACTUALLY
// reaches, measured by running the kernel (HmovedPixel). Per CLAUDE.md the X(N)
// offset is kernel-specific, so we never trust the arithmetic alone: Solve verifies
// every answer against the emulator (the same ground truth `calibrate` regresses).
//
// It is built clean-room on the verified `roms/techniques/shared_setxpos.asm`
// idiom: div-15 coarse positioning via the RESPx strobe timing, with the remainder
// turned into an HMOVE fine nibble by the classic `eor #7; asl x4` trick.
package spritepos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// objIndex maps an object name to the index used by `RESP0,x` / `HMP0,x`.
var objIndex = map[string]int{"P0": 0, "P1": 1, "M0": 2, "M1": 3, "BL": 4}

// Objects lists the supported object names in index order.
var Objects = []string{"P0", "P1", "M0", "M1", "BL"}

// Decompose mirrors the runtime arithmetic of SetXPos for input A = x: how many
// 15px coarse steps the divide loop runs and the signed HMOVE nibble (-8..+7) the
// remainder becomes. Pure — for author insight; the achieved position is verified
// separately. (Coarse steps = ⌈x/15⌉ … the loop runs until A goes negative.)
func Decompose(x int) (coarseSteps int, hmoveNibble int) {
	a := x & 0xFF
	for {
		coarseSteps++
		borrow := a < 15 // sec; sbc #15: carry clears (bcs falls through) iff a < 15
		a = (a - 15) & 0xFF
		if borrow { // carry clear -> loop ends; A now holds remainder-15 ($F1..$FF)
			break
		}
	}
	// remainder-15 in A; eor #7 then take the high nibble after asl x4.
	n := (a ^ 0x07) & 0x0F
	if n >= 8 {
		hmoveNibble = n - 16 // upper-nibble two's complement: +7..-8
	} else {
		hmoveNibble = n
	}
	return coarseSteps, hmoveNibble
}

// romTemplate positions the object selected by RAM cell objCell at the X in RAM
// cell inCell, every frame, using the verified SetXPos idiom. Both cells are poked
// at run time (the init clear runs once, so post-warmup pokes stick).
const objCell = 0x81
const inCell = 0x80

const romTemplate = `        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
RESP0   = $10
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
inX     = $80
objIdx  = $81
        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
ClearM: sta $00,x
        dex
        bne ClearM
Main:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        sta HMCLR
        ldx objIdx
        lda inX
        jsr SetXPos
        sta WSYNC
        sta HMOVE
        ldx #200
Vis:    sta WSYNC
        dex
        bne Vis
        jmp Main
SetXPos:
        sec
        sta WSYNC
WaitObj:
        sbc #15
        bcs WaitObj
        eor #7
        asl
        asl
        asl
        asl
        sta HMP0,x
        sta.w RESP0,x
        rts
        org $FFFC
        .word Start
        .word Start
`

// Positioner runs the calibrated template ROM and measures achieved positions.
type Positioner struct {
	e *emu.Emu
}

// NewPositioner assembles + loads the template and warms past the one-time init
// clear (so subsequent pokes to the input cells persist).
func NewPositioner() (*Positioner, error) {
	dir, err := os.MkdirTemp("", "spritepos")
	if err != nil {
		return nil, err
	}
	asm := filepath.Join(dir, "spritepos.asm")
	bin := filepath.Join(dir, "spritepos.bin")
	if err := os.WriteFile(asm, []byte(romTemplate), 0o644); err != nil {
		return nil, err
	}
	if out, aerr := build.Assemble(asm, bin); aerr != nil {
		return nil, fmt.Errorf("assemble template:\n%s", out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, err
	}
	if err := e.RunFrames(1); err != nil { // run the init clear once
		return nil, err
	}
	return &Positioner{e: e}, nil
}

// Achieve positions `object` with routine input A = inputA and returns the X the
// hardware actually reaches (HmovedPixel, 0..159) — the ground truth.
func (p *Positioner) Achieve(object string, inputA int) (int, error) {
	idx, ok := objIndex[object]
	if !ok {
		return 0, fmt.Errorf("unknown object %q", object)
	}
	if err := p.e.Poke(objCell, uint8(idx)); err != nil {
		return 0, err
	}
	if err := p.e.Poke(inCell, uint8(inputA&0xFF)); err != nil {
		return 0, err
	}
	if err := p.e.RunFrames(2); err != nil { // settle the new position
		return 0, err
	}
	x, _ := p.e.ObjectX(object)
	return x, nil
}

// Solution is a solved placement for a target X.
type Solution struct {
	Object      string `json:"object"`
	TargetX     int    `json:"target_x"`
	InputA      int    `json:"input_a"`       // value to load before SetXPos to land on TargetX
	AchievedX   int    `json:"achieved_x"`    // measured HmovedPixel (== TargetX when Exact)
	Exact       bool   `json:"exact"`         // AchievedX == TargetX
	CoarseSteps int    `json:"coarse_steps"`  // div-15 loop iterations for InputA
	HMOVENibble int    `json:"hmove_nibble"`  // signed fine adjust (-8..+7)
}

// Solve finds the routine input that lands `object` on targetX in this kernel and
// verifies it on the emulator. The X(A) map is A→X with slope 1 and a kernel
// constant offset (CLAUDE.md), so we measure the offset once and invert it, then
// re-run to confirm — never trusting the math. Returns the verified Solution.
func (p *Positioner) Solve(object string, targetX int) (Solution, error) {
	if targetX < 0 || targetX > 159 {
		return Solution{}, fmt.Errorf("targetX %d out of range 0..159", targetX)
	}
	// Measure the constant offset c: X(A0) = (A0 + c) mod 160 at a mid probe.
	const a0 = 80
	x0, err := p.Achieve(object, a0)
	if err != nil {
		return Solution{}, err
	}
	c := ((x0-a0)%160 + 160) % 160
	inputA := ((targetX-c)%160 + 160) % 160
	got, err := p.Achieve(object, inputA)
	if err != nil {
		return Solution{}, err
	}
	cs, nib := Decompose(inputA)
	return Solution{
		Object: object, TargetX: targetX, InputA: inputA, AchievedX: got,
		Exact: got == targetX, CoarseSteps: cs, HMOVENibble: nib,
	}, nil
}

// Snippet returns paste-able 6502 that positions `object` using the verified
// SetXPos idiom with the given routine input. Re-verify in your own kernel — the
// landing X has a kernel-specific constant offset (use Solve to calibrate it).
func Snippet(object string, inputA int) string {
	idx := objIndex[object]
	return fmt.Sprintf(`        ldx #%d              ; %s
        lda #%d
        jsr SetXPos
        sta WSYNC
        sta HMOVE
        ; --- SetXPos: div-15 coarse (RESPx) + eor#7 HMOVE-nibble fine ---
SetXPos:
        sec
        sta WSYNC
.wait:  sbc #15
        bcs .wait
        eor #7
        asl
        asl
        asl
        asl
        sta HMP0,x
        sta.w RESP0,x
        rts`, idx, object, inputA)
}
