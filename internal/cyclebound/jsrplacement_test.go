package cyclebound

import (
	"os"
	"path/filepath"
	"testing"
)

// placementROM builds a kernel whose row either does the positioning inline or calls it.
func placementROM(t *testing.T, viaJSR bool) string {
	t.Helper()
	body, sub := `        lda #$70
        sta HMP0
        sta RESP0
        sta HMOVE
        sta HMCLR`, ""
	if viaJSR {
		body, sub = "        jsr Place", `Place:  lda #$70
        sta HMP0
        sta RESP0
        sta HMOVE
        sta HMCLR
        rts`
	}
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUP0=$06
COLUBK=$09
RESP0=$10
GRP0=$1B
HMP0=$20
HMOVE=$2A
HMCLR=$2B
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #0
        sta COLUBK
        lda #$FF
        sta GRP0
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #33
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldy #95
Krow:   sta WSYNC
` + body + `
        dey
        bpl Krow
        lda #2
        sta VBLANK
        ldx #37
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
` + sub + `
        org $FFFC
        .word Start
        .word Start
`
	p := filepath.Join(t.TempDir(), "p.asm")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestFoldingPlacementIntoAJSRCostsTwelveCycles puts a number on a choice the sprite-placement work
// has to make: whether the positioning steps live inline in the kernel row or behind a call.
//
// A subroutine costs `JSR` 6 cycles plus `RTS` 6, and unlike most costs it lands on the **worst
// case** rather than the average: the call happens on every row whatever the data does.
//
// Measured 2026-09-07 with `prove_line_budget` on two kernels identical but for the fold:
//
//	positioning inline    Krow worst 28 of 76
//	positioning via JSR   Krow worst 40 of 76      +12
//
// ★**Twelve cycles is about 1.7 graphics stores** at the seven cycles a `lda (zp),y` + `sta GRPx`
// pair costs, so the fold is not free and it is not catastrophic either — it is one and a half
// sprites' worth of the line, charged unconditionally.
//
// Both kernels certify here, which is the point worth keeping: the budget prover will not object to
// the fold in a row with room, and it will object in a row without. **The number is what decides,
// not the gate.** Raised by the mailing-list distillation (helper-2), who predicted +12 exactly.
func TestFoldingPlacementIntoAJSRCostsTwelveCycles(t *testing.T) {
	krow := func(viaJSR bool) int {
		r, err := Prove(placementROM(t, viaJSR), 76)
		if err != nil {
			t.Fatalf("prove: %v", err)
		}
		for _, l := range r.Lines {
			if len(l.StartLoc) >= 4 && l.StartLoc[:4] == "Krow" {
				return l.Worst
			}
		}
		t.Fatal("no Krow region in the report — the generated kernel changed shape")
		return 0
	}

	inline := krow(false)
	viaJSR := krow(true)

	if inline == 0 || viaJSR == 0 {
		t.Fatal("one of the kernels measured zero cycles, so the difference below is meaningless")
	}
	if d := viaJSR - inline; d != 12 {
		t.Errorf("the call costs %d cycles (inline %d, via JSR %d), want 12 — `JSR` 6 plus `RTS` 6. "+
			"If this moved, either the cycle model changed or the generated kernels stopped being "+
			"identical apart from the fold", d, inline, viaJSR)
	}
	// Two-sided: the fold must still fit, or the measurement is about a broken kernel rather than
	// about a choice anyone would make.
	if viaJSR > 76 {
		t.Errorf("the folded kernel needs %d of 76 cycles — it no longer fits, so this test is "+
			"measuring an overrun and not the price of a call", viaJSR)
	}
}
