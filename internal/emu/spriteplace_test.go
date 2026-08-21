package emu

import "testing"

// TestSpritePlacementPhysics machine-locks where a RESP and a RESM strobe put their object, how
// late a GRP write can be and still reach the copy it feeds, and what neither of them does on the
// line it runs on. Grades roms/litmus/litmus_sprite_place.bin (built by
// scripts/gen_litmus_sprite_place.py); the rules are stated there and in
// docs/techniques/sprite-placement.md.
//
// WHY THIS EXISTS. These are the numbers a kernel author needs before deciding whether a shape
// wider than eight pixels can be drawn at all, and until 2026-08-21 none of them was written down
// anywhere in this repository. They were re-derived with throwaway probe ROMs twice in one
// session, the second time after the author asked whether the harness was using what the
// community already knows — it was not, because it had never been told. The catalogue covers what
// to DO with sprites (nusiz-shaping, dynamic-multisprite, bitmap48); this covers where they land.
//
// Two of these rules turned an eight-letter row into a ten-letter one in the piece that prompted
// them, which is a fair measure of what not knowing them costs.
func TestSpritePlacementPhysics(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_sprite_place.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	type run struct{ clock, len int }
	// runsOn returns, per element, where it was drawn on that scanline and for how long.
	runsOn := func(line int) map[string][]run {
		out := map[string][]run{}
		rs, _, err := e.DecomposeRow(line)
		if err != nil {
			return out
		}
		for _, r := range rs {
			if r.Element == "P0" || r.Element == "M0" {
				out[r.Element] = append(out[r.Element], run{r.Clock, r.Len})
			}
		}
		return out
	}
	starts := func(line int) map[string][]int {
		out := map[string][]int{}
		for el, rs := range runsOn(line) {
			for _, r := range rs {
				out[el] = append(out[el], r.clock)
			}
		}
		return out
	}

	// Band 0's first line draws nothing on purpose, so the first LIT line is band 0 line 1 and the
	// band grid starts one above it. Finding it beats assuming a VisibleTop: a change to the
	// blanking above the picture then cannot silently shift every case.
	base := -1
	for line := 20; line < 140 && base < 0; line++ {
		if len(starts(line)) > 0 {
			base = line - 1
		}
	}
	if base < 0 {
		t.Fatal("the litmus drew nothing in scanlines 20..140")
	}
	line := func(band, k int) int { return base + 4*band + k }

	eq := func(what string, got, want []int) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: drawn at %v, want %v", what, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: copy %d drawn at clock %d, want %d", what, i, got[i], want[i])
			}
		}
	}

	// ---- rules 1 and 2: where a strobe puts a player and where it puts a missile ----
	for _, c := range []struct {
		band int
		elem string
		want []int
		why  string
	}{
		{0, "P0", []int{12}, "player x = 3c - 60, c = 24"},
		{1, "P0", []int{60}, "player x = 3c - 60, c = 40"},
		{2, "M0", []int{11}, "missile x = 3c - 61, c = 24"},
		{3, "M0", []int{59}, "missile x = 3c - 61, c = 40"},
	} {
		eq(c.why, starts(line(c.band, 1))[c.elem], c.want)
	}

	// ---- rule 3: three cycles later lands the missile at player+8, not player+9 ----
	// The gap two RESP strobes cannot close: three cycles is the shortest distance between two
	// stores and three cycles is nine colour clocks. This is what lets a 12 px shape be an 8 px
	// player with a 4 px missile abutting it, positioned by strobes alone and no HMOVE.
	eq("RESP0 at 30", starts(line(4, 1))["P0"], []int{30})
	eq("RESM0 three cycles later", starts(line(4, 1))["M0"], []int{38})

	// ---- rule 4: the missile follows its player's NUSIZ copies ----
	eq("NUSIZ0=$26 copies the player", starts(line(5, 1))["P0"], []int{30, 62, 94})
	eq("and the missile with it", starts(line(5, 1))["M0"], []int{38, 70, 102})

	// ---- rule 5: a strobe does not draw the NEW position on its own line ----
	// P0 is lit across its own RESP0 (write cycle 40, so x=60). The strobe line must still show
	// the position it had before, and 60 must not appear until the line after.
	eq("the strobe line", starts(line(6, 2))["P0"], []int{30})
	eq("the line after the strobe", starts(line(6, 3))["P0"], []int{60})

	// ---- rule 6: a GRP write takes effect at x = 3w - 64 ----
	// Copy 0 at x=30 has already drawn when GRP0 drops to $00; copy 1 at x=62 is the question.
	// At write cycle 43 the write lands at x=61 and takes effect at 65, so copy 1 keeps THREE
	// pixels of $FF — 65-62, exactly. At 42 and 41 it is clean. Four clocks of margin is enough
	// and one is not; six, which this repository's kernels assumed for a while, is two too many.
	for _, c := range []struct {
		k, leak int
		why     string
	}{
		{1, 3, "GRP0<-0 on cycle 43 lands at x=61, effective at 65"},
		{2, 0, "GRP0<-0 on cycle 42 lands at x=58, effective at 62"},
		{3, 0, "GRP0<-0 on cycle 41 lands at x=55, effective at 59"},
	} {
		var leak int
		for _, r := range runsOn(line(7, c.k))["P0"] {
			if r.clock >= 62 && r.clock < 94 {
				leak += r.len
			}
		}
		if leak != c.leak {
			t.Errorf("%s: copy 1 kept %d px of the old byte, want %d", c.why, leak, c.leak)
		}
		if got := runsOn(line(7, c.k))["P0"]; len(got) == 0 || got[0].clock != 30 || got[0].len != 8 {
			t.Errorf("%s: copy 0 came out %v, want one 8 px run at 30 — the control for this band "+
				"is that the write is LATE, not that it never happened", c.why, got)
		}
	}

	// ---- rule 7: a normal-width player cannot be strobed left of x=3 ----
	// Write cycles 17 and 21 both land on 3, and 3 = 3*21 - 60 is where the clamp and the formula
	// meet; 22 and 25 are above it and obey 3c - 60. This is what fixes where a picture as wide as
	// the screen has to start.
	eq("RESP0 write cycle 17, clamped", starts(line(8, 1))["P0"], []int{3})
	eq("RESP0 write cycle 21, clamped", starts(line(8, 3))["P0"], []int{3})
	eq("RESP0 write cycle 22", starts(line(9, 1))["P0"], []int{6})
	eq("RESP0 write cycle 25", starts(line(9, 3))["P0"], []int{15})

	// ---- rule 8: a strobe cancels the FIRST copy only ----
	// P0 is parked at 42 with copies at 42, 74, 106 and gets strobed to x=24 while the beam is at
	// 16 — ahead of all three. Neither "it just moves" (24, 56, 88 would draw) nor "it goes quiet"
	// (nothing would) is what happens: copy 0 is cancelled and the copies at +32 and +64 land
	// anyway, because they are triggered from the reset position. Skipping a blank line's clear on
	// the strength of the strobe therefore does not work, which cost a rebuild to find out.
	eq("parked at 42", starts(line(10, 1))["P0"], []int{42, 74, 106})
	eq("the strobe line keeps copies 1 and 2", starts(line(10, 2))["P0"], []int{56, 88})
	eq("and the line after has all three", starts(line(10, 3))["P0"], []int{24, 56, 88})
}
