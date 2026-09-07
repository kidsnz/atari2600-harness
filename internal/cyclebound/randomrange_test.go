package cyclebound

import (
	"os"
	"path/filepath"
	"testing"
)

// rangeROM builds a kernel whose row narrows a random byte to a range, with or without the `clc`.
func rangeROM(t *testing.T, withCLC bool) string {
	t.Helper()
	body := "        lsr\n        adc #9\n        sta $84"
	if withCLC {
		body = "        clc\n" + body
	}
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUBK=$09
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldy #95
Krow:   sta WSYNC
        lda $83
` + body + `
        dey
        bpl Krow
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`
	p := filepath.Join(t.TempDir(), "r.asm")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestUniformRandomRangeCostsTwoCycles prices a line B. Watson posted in 2005 and an improvement he
// did not mention.
//
// His narrowing, chosen for taking the same time whatever the byte is:
//
//	jsr getRandomByte  ; byte is 0-255
//	lsr a              ; now it's 0-127, carry may or may not be set
//	adc #9             ; now it's 9-137, done
//
//	"This has the advantage of running in a constant amount of time."  〔`200505/msg00172`〕
//
// The range is right and the distribution is not uniform: `lsr` leaves the byte's bit 0 in the carry
// and `adc` adds it, so the sum reaches 137 only from one input and 9 only from one. Verified over
// all 256 inputs:
//
//	as written      9..137, every value twice except BOTH ENDS, which appear once
//	with `clc`      9..136, every value exactly twice
//
// ★So `clc` buys a flat distribution for **two cycles and the top value of the range**. Measured with
// `prove_line_budget` on two otherwise identical kernels: the row goes from 14 to 16 cycles of 76.
//
// ★★Whether that matters is a design question, not a technical one: for a tetromino out of seven the
// edge bias is invisible, and for a damage roll it is a value that comes up half as often as its
// neighbour. The point of pricing it is that **the fix is two cycles, so the question is never
// "can I afford it"**.
//
// Found by the mailing-list distillation (helper-1), who worked the distribution out by hand across
// all 256 inputs; it was re-derived by machine here before being written down.
func TestUniformRandomRangeCostsTwoCycles(t *testing.T) {
	krow := func(withCLC bool) int {
		r, err := Prove(rangeROM(t, withCLC), 76)
		if err != nil {
			t.Fatalf("prove: %v", err)
		}
		for _, l := range r.Lines {
			if len(l.StartLoc) >= 4 && l.StartLoc[:4] == "Krow" {
				return l.Worst
			}
		}
		t.Fatal("no Krow region — the generated kernel changed shape")
		return 0
	}

	plain, uniform := krow(false), krow(true)
	if plain == 0 {
		t.Fatal("the plain kernel measured zero cycles")
	}
	if d := uniform - plain; d != 2 {
		t.Errorf("`clc` costs %d cycles (%d then %d), want 2. If this moved, the cycle model changed "+
			"or the two kernels differ by more than the one instruction", d, plain, uniform)
	}

	// The distribution, re-derived here rather than quoted, because the cycle price only means
	// something if it buys what the comment says it buys.
	tally := func(withCLC bool) map[int]int {
		out := map[int]int{}
		for b := 0; b < 256; b++ {
			a := b >> 1
			c := 0
			if !withCLC {
				c = b & 1 // lsr leaves bit 0 in the carry; adc adds it
			}
			out[(a+9+c)&0xFF]++
		}
		return out
	}
	as, cl := tally(false), tally(true)

	if as[9] != 1 || as[137] != 1 {
		t.Errorf("as written, 9 appears %d times and 137 %d — both ends are supposed to be reachable "+
			"from exactly one input, which is the bias the `clc` removes", as[9], as[137])
	}
	if len(cl) != 128 {
		t.Errorf("with `clc` the range holds %d values, want 128 (9..136)", len(cl))
	}
	for v, n := range cl {
		if n != 2 {
			t.Fatalf("with `clc`, value %d appears %d times, want 2 for every value — the point of "+
				"the two cycles is that the result is flat", v, n)
		}
	}
	if _, ok := cl[137]; ok {
		t.Error("with `clc` the range still reaches 137; the top value is what the uniformity costs " +
			"and if it is still there the trade being described is not the one happening")
	}
}
