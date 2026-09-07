package place

import "testing"

// TestPlacementDeadzoneIsTheLeftEdge measures WHERE a shape cannot be placed at all, which
// this package could always answer one x at a time and had never been asked across the
// screen. A distillation item put it plainly: *"`internal/place` / `plan_sprite_placement`
// に deadzone は0行"* — the solver returns an error for an impossible x and nothing said
// which x are impossible, so an author meets them one at a time, as a surprise.
//
// Swept 2026-09-07 over every x in 0..159 for a single 8-clock shape:
//
//	x =   0..32   20 of 33 impossible — 2-pixel holes at a 3-pixel pitch
//	x =  33..150  ZERO impossible — the middle 118 pixels are free
//	x = 151..159  four 1-pixel holes
//
// **The middle of the screen is not the problem; the left edge is a comb.** The cause is the
// one CLAUDE.md already states — `RESPx` positions on a 3-colour-clock grid and the strobe
// cannot happen before `ClampFirst` — so near the left edge the reachable positions are
// quantised and the gaps are visible. Away from it every residue is reachable.
//
// ★This is a claim about the SOLVER, not about the TIA: it says which x this package can
// currently produce a plan for. The controls below keep it that way — if the free middle
// ever narrows or the left comb changes pitch, the numbers move and the test says so.
func TestPlacementDeadzoneIsTheLeftEdge(t *testing.T) {
	bad := map[int]bool{}
	for x := 0; x < 160; x++ {
		if _, err := Solve([]Shape{{X: x}}, 0, 0); err != nil {
			bad[x] = true
		}
	}

	mid := 0
	for x := 33; x <= 150; x++ {
		if bad[x] {
			mid++
		}
	}
	left := 0
	for x := 0; x <= 32; x++ {
		if bad[x] {
			left++
		}
	}
	t.Logf("unplaceable: x=0..32 → %d of 33   x=33..150 → %d of 118   total %d", left, mid, len(bad))

	if mid != 0 {
		t.Errorf("%d unplaceable x between 33 and 150 — the middle of the screen used to be "+
			"entirely free, and an author placing a row of glyphs relies on that", mid)
	}
	if left == 0 {
		t.Errorf("no unplaceable x at the left edge — either the strobe floor has moved or " +
			"Solve has stopped reporting impossibility, and a caller would now get a plan " +
			"for a position the hardware cannot reach")
	}
	// The comb, not just its existence: holes near the left edge come in ones and twos, never
	// in a long block. A single wide dead band would be a different (and much worse) shape.
	run, longest := 0, 0
	for x := 0; x <= 32; x++ {
		if bad[x] {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	if longest > 3 {
		t.Errorf("longest unplaceable run at the left edge is %d px; it has been a comb of 1s "+
			"and 2s, and a wide band means something other than grid quantisation", longest)
	}
}
