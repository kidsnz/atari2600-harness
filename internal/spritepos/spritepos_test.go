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

// TestAchieveSweepLog is exploratory (measure, don't guess): print X(A) so the
// slope-1 + constant-offset model behind Solve is visible in the record.
func TestAchieveSweepLog(t *testing.T) {
	p, err := NewPositioner()
	if err != nil {
		t.Fatal(err)
	}
	for a := 0; a < 160; a += 8 {
		x, err := p.Achieve("P0", a)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("A=%3d -> X=%3d", a, x)
	}
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
