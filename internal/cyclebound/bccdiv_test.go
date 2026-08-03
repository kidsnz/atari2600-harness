package cyclebound

// BCC counts UP, so `amax` is the wrong variable for it.
//
// The divide idiom's two latches run the loop in opposite directions:
//
//	`sbc #N / bcs L`  loops while NO borrow (A >= N). A FALLS by N each time, so a
//	                  larger entry value means more iterations — `amax` bounds it.
//	`sbc #N / bcc L`  loops while there IS a borrow (A < N). The subtraction wraps, so
//	                  A RISES by (255 - N) until it reaches N. A larger entry value
//	                  means FEWER iterations, and `amax` bounds nothing at all.
//
// One formula was applied to both. It agrees only while N is small: at N = 15 it is
// loose and safe, at N = 200 it answers 2 for a loop the machine runs 5 times.
// Measured on this fixture: **proven 16 cycles, machine 31 — 1.9x under**, with
// `certified: true`. The smallest of the nine unsound bounds the premise audit found,
// the same direction as all of them, and the last one.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestBccDivideCountsUpwardsNotDownwards(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bccdiv.asm"

	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	region := func(name string) (Region, bool) {
		for _, r := range append(append([]Region{}, rep.Lines...), rep.Unbounded...) {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				return r, true
			}
		}
		return Region{}, false
	}

	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(binFor(asm)); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(12, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := map[uint16]emu.LineWorst{}
	for _, r := range rows {
		if r.Count > 0 {
			measured[r.StrobePC] = r
		}
	}

	for _, c := range []struct{ name, why string }{
		{"DangerRow", "a BCC with N=200: A enters at 0 and rises by 55 a step, so it takes five " +
			"iterations to reach 200 — the BCS formula answers amax/N + 2 = 2"},
		{"BcsCtl", "the BCS form, where `amax` genuinely is the governing variable — losing or moving " +
			"it means the fix keyed on the divide idiom rather than on the latch"},
		{"SmallCtl", "a BCC with N=15, where the old formula was already safe — losing it means the " +
			"repair refuses the common case"},
	} {
		r, ok := region(c.name)
		if !ok {
			t.Errorf("no %s region", c.name)
			continue
		}
		if !r.Bounded {
			t.Errorf("%s is refused (%s) — %s", c.name, r.Reason, c.why)
			continue
		}
		row, hit := measured[r.Start]
		if !hit {
			t.Errorf("%s produced no measured interval, so its bound is graded by nothing", c.name)
			continue
		}
		if r.Worst < row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — an UNDER-approximation. %s",
				c.name, r.Worst, row.WorstCycles, c.why)
		}
	}

	// The BCS control must keep the SAME number, not merely stay sound. A fix that
	// changed both latches would leave the fixture green while quietly re-deriving a
	// bound that was already correct.
	if bcs, ok := region("BcsCtl"); ok && bcs.Bounded && bcs.Worst != 36 {
		t.Errorf("BcsCtl proves %d, expected 36 — the BCS path must be untouched", bcs.Worst)
	}
}

// TestBccIterationBoundIsExactAtTheEdges pins the arithmetic itself, including the case
// the corpus cannot reach: N = 255 does not terminate at all, because 255 - 255 = 0
// leaves A where it was. A number there would be a bound on a loop with no end.
func TestBccIterationBoundIsExactAtTheEdges(t *testing.T) {
	// ceil(N/(255-N)) + 2, and 0 (refuse) when A cannot rise.
	bound := func(sub int) int {
		up := 255 - sub
		if up <= 0 {
			return 0
		}
		return (sub+up-1)/up + 2
	}
	for _, c := range []struct {
		sub, want int
		why       string
	}{
		{15, 3, "A rises by 240, so one wrap clears 15; +1 for the exit, +1 for an untracked entry carry"},
		{128, 4, "A rises by 127, so two wraps are needed to pass 128"},
		{200, 6, "A rises by 55: ceil(200/55) = 4 wraps, and the machine takes 5 iterations"},
		{254, 256, "A rises by one a step, so reaching 254 from zero takes 254 wraps"},
		{255, 0, "A cannot rise at all — the loop does not terminate and must be refused"},
	} {
		if got := bound(c.sub); got != c.want {
			t.Errorf("sub=%d: bound %d, want %d — %s", c.sub, got, c.want, c.why)
		}
	}
}
