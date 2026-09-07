package emu

import (
	"testing"
)

// TestVDELPxDecodesBitZeroOnly settles a claim made in a 2002 code comment and never checked here.
//
// Thomas Jentzsch's two-line kernel writes the parity straight into the register:
//
//	LDA P0_YFullres
//	EOR #1
//	STA VDELP0          ;don't care for the bits 1..7, VDEL ignores them
//
// 〔stella-list `200204/msg00067`, 2002-04〕. `docs/techniques/two-line-kernel.md` instead tells
// authors to write **`VDELP0 = y & 1`**, and nothing in this repository said whether the mask is
// required. If it is not, every object drops one `AND #1` per frame — **2 cycles and 1 byte each**,
// in the part of the frame where cycles are scarcest.
//
// `roms/litmus/litmus_vdel_bits.asm` writes six values into VDELP0, parks OLD := $00 and NEW := $FF
// before each, and lets the picture answer: a lit band means the NEW graphic reached the screen
// (delay OFF), a dark band means the parked OLD one did (delay ON).
//
//	A  $00  bit 0 clear                 lit    control: off is off
//	B  $01  bit 0 set                   dark   control: on is on
//	C  $02  bit 1 set, bit 0 CLEAR      lit    bit 1 ignored
//	D  $03  bits 0 and 1 set            dark   bit 0 decides
//	E  $FE  every bit but bit 0         lit    all seven ignored
//	F  $FF  every bit                   dark   bit 0 decides
//
// Measured 2026-09-07: **lit, dark, lit, dark, lit, dark** — exactly bit 0, and `$FE` with seven
// bits set behaves identically to `$00`. Jentzsch was right and the mask can go.
//
// A and B are what make C, D, E and F readable: without them a run of dark bands could mean the
// litmus never draws anything, and a run of lit ones could mean VDEL never engages.
//
// Found by the mailing-list distillation (helper-2).
func TestVDELPxDecodesBitZeroOnly(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_vdel_bits.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	// A band is "lit" when the player's 8 white pixels are on screen.
	drawn := func(line int) bool {
		runs, _, err := e.ReadRow(line)
		if err != nil {
			return false
		}
		for _, r := range runs {
			if r.Hex == "FFFFFE" {
				return true
			}
		}
		return false
	}

	bands := []struct {
		name    string
		value   string
		line    int // a line comfortably inside the band's twelve
		wantLit bool
	}{
		{"A", "$00", 36, true},
		{"B", "$01", 51, false},
		{"C", "$02", 66, true},
		{"D", "$03", 81, false},
		{"E", "$FE", 96, true},
		{"F", "$FF", 111, false},
	}

	for _, b := range bands {
		if got := drawn(b.line); got != b.wantLit {
			t.Errorf("band %s (VDELP0 = %s) at line %d: player drawn = %v, want %v. "+
				"Lit means the NEW graphic reached the screen (delay OFF); dark means the parked "+
				"OLD one did (delay ON)", b.name, b.value, b.line, got, b.wantLit)
		}
	}

	// The measurement, stated as the thing an author would act on: seven set bits and none set
	// give the SAME answer, so `AND #1` before `STA VDELPx` buys nothing.
	if drawn(96) != drawn(36) {
		t.Error("$FE and $00 disagree — bits 1..7 are decoded after all, and " +
			"two-line-kernel.md's `VDELP0 = y & 1` mask is load-bearing. Do not drop it")
	}
	if drawn(111) != drawn(51) {
		t.Error("$FF and $01 disagree, which would mean the high bits change the behaviour " +
			"only when bit 0 is set — re-measure before writing anything about this register")
	}
}
