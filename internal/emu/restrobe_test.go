package emu

import "testing"

// TestRestrobeAddsCopies machine-locks what a mid-line RESP strobe is worth when the player is in a
// COPY mode: it does not move the copies already drawn, it ADDS another run of them, one per strobe.
// Grades roms/litmus/litmus_restrobe.bin (built by scripts/gen_litmus_restrobe.py); the rules are
// stated in docs/techniques/restrobe-copies.md.
//
// WHY THIS EXISTS. reference/atariage/180632 records solidcorp's 32-character display and files the
// mechanism as candidate ⑨, unverified. Nothing here had checked it, and the work that prompted this
// had concluded "two players, three copies each, six shaped slots a scanline" was a hardware ceiling.
// It is eight per player.
//
// The three probes that had said otherwise were built the same wrong way: the claim is about a copy
// SEQUENCE restarting, and each of them measured a player with ONE copy, where "the first copy" and
// "the only copy" are the same thing -- so sprite-placement.md rule 8 ate the result every time.
//
// NOTHING IN sprite-placement.md WAS WRONG, and this test nearly "corrected" it. The measurement that
// produced the ladder labelled its strobes one cycle high -- it padded to the store's FIRST cycle and
// called that the write cycle -- so every landing read three pixels off and rule 1 looked like it said
// 3c-60 where the machine said 3c-63. The catalogue's convention is the store's LAST cycle
// (scripts/gen_litmus_sprite_place.py:strobe pads to want-2); the fixture here now uses the same one,
// and rule 1 reproduces exactly. Two numbers that differ by one cycle are the same measurement until
// the origin is checked.
//
// THE TEST DOES NOT ASSUME WHICH SCANLINE IS WHICH. It reads every lit line and grades the SET of
// copy counts, because fixing band k to a line number graded the wrong rows and the failure looked
// like a hardware disagreement.
func TestRestrobeAddsCopies(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_restrobe.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	// p0On returns where P0 was drawn on a scanline, left to right.
	p0On := func(line int) []int {
		rs, _, err := e.DecomposeRow(line)
		if err != nil {
			return nil
		}
		var out []int
		for _, r := range rs {
			if r.Element == "P0" {
				out = append(out, r.Clock)
			}
		}
		return out
	}

	counts := map[int]bool{}
	var widest []int
	var sawPark bool
	for line := 20; line < 200; line++ {
		xs := p0On(line)
		if len(xs) == 0 {
			continue
		}
		counts[len(xs)] = true
		t.Logf("line %3d: %d copies %v", line, len(xs), xs)
		if len(xs) > len(widest) {
			widest = xs
		}
		// The parked player, untouched, draws its three copies sixteen apart from the leftmost a
		// player reaches. That line is the baseline every band is compared against.
		if len(xs) == 3 && xs[0] == 3 && xs[1] == 19 && xs[2] == 35 {
			sawPark = true
		}
	}

	if !sawPark {
		t.Error("no line shows the parked three copies at 3, 19, 35 — the fixture did not park")
	}

	// THE LADDER: k re-strobes are worth k more copies. Zero through five re-strobes must all be
	// present, and eight is where five strobes land -- not 3+2k, because a strobe puts the base at
	// 3c-63, five colour clocks ahead of the beam, and the TIA needs more than five to start: the
	// copy AT the base is lost, and of the two NUSIZ makes from it one lands where the previous
	// group's last copy already was.
	for k := 0; k <= 5; k++ {
		if !counts[3+k] {
			t.Errorf("no scanline draws %d copies of P0: the ladder is broken at %d re-strobes", 3+k, k)
		}
	}
	if len(widest) != 8 {
		t.Errorf("the widest line draws %d copies, want 8: %v", len(widest), widest)
	}

	// The first two copies are drawn before any strobe can reach them, so they never move.
	if len(widest) >= 2 && (widest[0] != 3 || widest[1] != 19) {
		t.Errorf("widest line leads with %v, want 3 and 19 — a strobe reached backwards", widest[:2])
	}

	// Every ADDED copy sits off the multiple-of-three grid, because a strobe base is 3c-63 and the
	// copies NUSIZ makes from it are +16 and +32. A row that needs a letter at x ≡ 0 (mod 3) cannot
	// get it from a re-strobe and has to shift the glyph inside its byte instead.
	for _, x := range widest[2:] {
		if x%3 == 0 {
			t.Errorf("added copy at x=%d is a multiple of three; a re-strobe cannot place one there", x)
		}
	}
}
