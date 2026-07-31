package emu

import "testing"

// TestAllSixteenHmoveNibblesMoveByOnePixelEach machine-locks the HMOVE table that
// CLAUDE.md calls a constant you must never get wrong: upper nibble only, two's
// complement, POSITIVE = LEFT and negative = right, $70 = left 7 … $00 = 0 …
// $F0 = right 1 … $80 = right 8, at 1px granularity.
//
// That table was hand-verified once, for v0.4.0, and nothing has held it true since:
// the existing HMOVE tests cover the ripple counter and the idle/unrecorded
// distinction, and litmus_hmove has no scenario. A constant nobody re-checks is a
// claim about a version, not about the code.
//
// The assertion is on the DRAWN pixel, not only on the register readout, because the
// file's own iron rule is that the verdict is where the object actually lands. Both
// are checked, and they must agree — if hmoved_pixel and the drawn span ever
// disagree, the readout has stopped describing the picture.
func TestAllSixteenHmoveNibblesMoveByOnePixelEach(t *testing.T) {
	// nibble -> documented displacement. Positive nibbles move LEFT.
	want := map[uint8]int{
		0x7: -7, 0x6: -6, 0x5: -5, 0x4: -4, 0x3: -3, 0x2: -2, 0x1: -1, 0x0: 0,
		0xF: +1, 0xE: +2, 0xD: +3, 0xC: +4, 0xB: +5, 0xA: +6, 0x9: +7, 0x8: +8,
	}

	drawnStart := func(e *Emu) (int, bool) {
		runs, _, err := e.DecomposeRow(100)
		if err != nil {
			return 0, false
		}
		for _, r := range runs {
			if r.Element == "P0" {
				return r.Clock, true
			}
		}
		return 0, false
	}

	var base int
	for nib := 0; nib < 16; nib++ {
		n := uint8(nib)
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_hmove.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(0x81, n<<4); err != nil { // HMVAL: the value written to HMP0
			t.Fatal(err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		v := e.VCS.TIA.Video
		reset, hmoved := v.Player0.ResetPixel, v.Player0.HmovedPixel
		if n == 0 {
			base = reset
			if hmoved != reset {
				t.Fatalf("$00 must not move the sprite, but hmoved_pixel %d != reset_pixel %d",
					hmoved, reset)
			}
		}
		if reset != base {
			t.Fatalf("nibble $%X0 changed the COARSE position (reset_pixel %d, expected %d) — the "+
				"fixture is no longer isolating the fine adjustment", n, reset, base)
		}
		if got := hmoved - reset; got != want[n] {
			t.Errorf("HMP0=$%X0: moved %+d px, want %+d (positive nibbles move LEFT)", n, got, want[n])
		}
		start, ok := drawnStart(e)
		if !ok {
			t.Fatalf("HMP0=$%X0: P0 is not drawn on the sampled line — nothing to verify against", n)
		}
		if start != hmoved {
			t.Errorf("HMP0=$%X0: hmoved_pixel says %d but P0 is drawn from clock %d — the numeric "+
				"readout has stopped describing the picture", n, hmoved, start)
		}
	}
}
