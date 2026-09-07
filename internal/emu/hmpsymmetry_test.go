package emu

import (
	"testing"
)

// TestHMP0AndHMP1MoveByTheSameAmount tests a 1998 report this repository could neither confirm nor
// dismiss, and records which of those two it managed.
//
// Brad Mott, in one sentence, never corroborated anywhere else in the archive:
//
//	"The only bad thing I see is that the meaning of HMP0 isn't quite the same as it is for HMP1.
//	 When the high nibble of HMP0 is F it moves P0 by -8 instead of -7 :-("
//	〔stella-list `199804/msg00193`〕
//
// `known-traps.md` already records that mid-line HMOVE behaviour differs by **TIA revision**. This
// would be a different axis entirely: the same chip, and the two players disagreeing with each other.
//
// `roms/litmus/litmus_hmp0_vs_hmp1.asm` places both players on one line, writes the SAME nibble to
// both registers, and strobes HMOVE once inside HBLANK. Measured 2026-09-07 across all sixteen
// nibbles: **P0 and P1 move by the same amount every time**, on the standard signed table —
// `$0`..`$7` give 0..−7 and `$8`..`$F` give +8..+1.
//
// ★**So the asymmetry does not reproduce here, and that is a statement about this instrument.** The
// engine applies one motion table to both objects, so it could not show a difference whatever the
// hardware does; and the Stella oracle compares `HMP0`/`HMP1` as register **values**, which are equal
// by construction in this litmus, not as motion. Both cross-checks are blind to Mott's claim in the
// same way. What is recorded is therefore "not reproducible here", in the shape
// `internal/emu/timerdiv_test.go` uses for the same situation — **not** "Mott was wrong".
//
// ★★Note also that `$F` moves **+1** here, not −8 or −7. Mott is counting in the other direction, so
// even his baseline uses a different convention from the engine's; a future measurement on hardware
// has to settle the convention before it can settle the asymmetry.
//
// Found by the mailing-list distillation (helper-2), who flagged that they had no independent second
// source for the sentence.
func TestHMP0AndHMP1MoveByTheSameAmount(t *testing.T) {
	// The standard HMOVE table, as this engine applies it: $0..$7 move left by the nibble,
	// $8..$F move right by 16 minus it.
	want := map[int]int{
		0x0: 0, 0x1: -1, 0x2: -2, 0x3: -3, 0x4: -4, 0x5: -5, 0x6: -6, 0x7: -7,
		0x8: +8, 0x9: +7, 0xA: +6, 0xB: +5, 0xC: +4, 0xD: +3, 0xE: +2, 0xF: +1,
	}

	moved := 0
	for nib := 0; nib < 16; nib++ {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_hmp0_vs_hmp1.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(0x81, uint8(nib)<<4); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		v := e.VCS.TIA.Video
		d0 := v.Player0.HmovedPixel - v.Player0.ResetPixel
		d1 := v.Player1.HmovedPixel - v.Player1.ResetPixel

		if d0 != d1 {
			t.Errorf("HM=$%X0 moves P0 by %+d and P1 by %+d. That is the asymmetry Brad Mott "+
				"reported in 1998 and it has never been reproducible here — if this fires, the "+
				"engine has started modelling it and the note in this test is out of date",
				nib, d0, d1)
		}
		if d0 != want[nib] {
			t.Errorf("HM=$%X0 moves by %+d, want %+d. The motion table itself changed, so the "+
				"symmetry above is being measured against a different mechanism", nib, d0, want[nib])
		}
		if d0 != 0 {
			moved++
		}
	}

	// Without this the test would pass on a litmus where HMOVE does nothing at all.
	if moved != 15 {
		t.Fatalf("only %d of the 16 nibbles moved the players; 15 should (every value but $0). "+
			"A litmus that never moves anything cannot show two objects moving by the same amount",
			moved)
	}
}
