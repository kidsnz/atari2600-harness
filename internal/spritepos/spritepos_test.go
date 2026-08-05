package spritepos

import "testing"

// TestDecompose locks the pure coarse/fine arithmetic against hand-computed values.
func TestDecompose(t *testing.T) {
	for _, c := range []struct{ x, coarse int }{
		{0, 1}, {14, 1}, {15, 2}, {30, 3}, {96, 7}, {159, 11},
	} {
		if cs, _ := Decompose(c.x); cs != c.coarse {
			t.Errorf("Decompose(%d) coarse=%d, want %d", c.x, cs, c.coarse)
		}
	}
}

// Solve searches for an input on the assumption that X(A) is a straight line of
// slope 1 — one pixel of travel per unit of A, with a kernel-specific offset. If
// that model breaks anywhere, Solve's search is walking the wrong curve and every
// answer it gives is luck.
//
// This used to be TestAchieveSweepLog: it swept A and PRINTED X(A) so the model
// was "visible in the record", which is not a check. The line was there to be read
// by a human who never read it, and the sweep could have come back flat, wrapped,
// or off by 30 and the test would still have been green.
//
// The line is now MEASURED. Sweeping A = 0,8,…,152 on this kernel:
//
//	A =   0 → X =   0        A =  80 → X =  80
//	A =  40 → X =  40        A = 152 → X = 152
//
// so slope is exactly 1 and the offset for THIS kernel is exactly 0 — which is a
// property of litmus_setxpos's prologue, not of the machine (CLAUDE.md: the offset
// constant is kernel-specific), so it is asserted as a measured constant of the
// fixture rather than as a hardware law. A deviation at any sampled A fails.
func TestAchieveIsAStraightLineOfSlopeOne(t *testing.T) {
	p, err := NewPositioner()
	if err != nil {
		t.Fatal(err)
	}
	sampled := 0
	for a := 0; a < 160; a += 8 {
		x, err := p.Achieve("P0", a)
		if err != nil {
			t.Fatal(err)
		}
		sampled++
		if off := x - a; off != 0 {
			t.Errorf("A=%d → X=%d: offset %+d, want 0. Slope-1 with a zero offset is what "+
				"Solve's search assumes on this kernel; if the prologue moved, the offset "+
				"moved with it and Solve is searching the wrong line", a, x, off)
		}
	}
	// PREMISE — a sweep of nothing would agree with any model at all.
	if sampled < 20 {
		t.Fatalf("only %d points sampled; the line is not measured at that size", sampled)
	}
	t.Logf("X(A) is identity across %d sampled inputs (A=0..152 step 8)", sampled)
}

// TestSolveHitsTargets is the core empirical guarantee: across objects and target
// positions, Solve lands the object EXACTLY on the target, verified by measuring
// HmovedPixel (not by trusting the formula).
func TestSolveHitsTargets(t *testing.T) {
	p, err := NewPositioner()
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range []string{"P0", "P1", "M0", "BL"} {
		for _, tx := range []int{12, 31, 48, 75, 96, 123, 150} {
			sol, err := p.Solve(obj, tx)
			if err != nil {
				t.Fatal(err)
			}
			if !sol.Exact {
				t.Errorf("%s target %d: achieved %d (inputA=%d, off by %+d)", obj, tx, sol.AchievedX, sol.InputA, sol.AchievedX-tx)
			}
		}
	}
}

// TestAchieveDiscriminates is the negative direction: a deliberately wrong input
// must NOT land on the target — proving the emulator measurement discriminates and
// the exact-match guarantee above isn't vacuous.
func TestAchieveDiscriminates(t *testing.T) {
	p, err := NewPositioner()
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range []int{40, 96} {
		got, err := p.Achieve("P0", tx+5) // 5px-wrong input
		if err != nil {
			t.Fatal(err)
		}
		if got == tx {
			t.Errorf("wrong input (target+5) unexpectedly landed on target %d", tx)
		}
	}
}
