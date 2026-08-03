package cyclebound

// A `dex; bne` counter entering at zero runs 256 times, not none.
//
// `determineBound` took the counter's entry range and returned its upper bound, on the
// reasoning that more iterations cost more. For BNE that is wrong at exactly one
// point: the trip count as a function of the entry value v is `v` for v > 0 and **256**
// for v = 0, because the decrement wraps to $FF. The function is not monotone, so `Hi`
// is not the maximum whenever zero is reachable — and the analysis answered for the
// smallest possible loop while the machine could run the largest.
//
// Measured on this fixture before the fix: proven **60** against a machine that spends
// **2319 across 31 scanlines** — 38.7x under, with `certified: true` and
// `roll_free: true`.
//
// The repair returns 256 rather than refusing, so the region stays BOUNDED. An author
// gets a number to act on instead of a refusal that says nothing, and the number is
// honest: it is what the hardware does on the path the analysis cannot rule out.
//
// It was found and deliberately left when the BPL sibling was fixed, because a census
// over the five commercial cartridges the gate then graded showed 3 instances and no
// violation. Re-censused once the gate stopped grading a hand-picked five: **14 folds
// across three shipped cartridges** (Seaquest x3, Bermuda Triangle x6, Vanguard x5, all
// [0,15]). The census was right about the observed runs and wrong about the exposure.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestBneCounterEnteringAtZeroWraps(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bnezero.asm"

	before := bneZeroWrapAdjusted
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}

	// PREMISE 1 — the fixture must actually reach the wrap path. Without this the
	// equality below could be satisfied by a fixture whose range stopped including
	// zero, and the test would grade a different branch while reading green.
	if got := bneZeroWrapAdjusted - before; got < 1 {
		t.Fatalf("the fixture reached the bne-wrap path %d times; it must reach it at least once, or "+
			"it witnesses nothing", got)
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

	// The danger region: bounded, and EXACT against the machine. Equality rather than
	// `>=` because the machine takes the zero arm, so 256 iterations is not a safe
	// over-estimate here — it is the true count, and a proof that cannot match a
	// fully determined loop has lost something.
	danger, ok := region("DangerRow")
	if !ok {
		t.Fatal("no DangerRow region; the fixture's labels moved")
	}
	if !danger.Bounded {
		t.Fatalf("DangerRow is refused (%s). The wrap count is knowable, so refusing costs an author a "+
			"number they could have acted on", danger.Reason)
	}
	row, hit := measured[danger.Start]
	if !hit {
		t.Fatal("DangerRow produced no measured interval; a bound on a region the machine never runs " +
			"demonstrates nothing")
	}
	if row.WorstCycles < 1000 {
		t.Errorf("DangerRow measured only %d cycles; it is written so the machine takes the ZERO arm and "+
			"wraps (2319 when authored), so it is no longer the shape this test is about", row.WorstCycles)
	}
	if danger.Worst != row.WorstCycles {
		t.Errorf("DangerRow: proven %d, machine %d — the counter enters at 0 and wraps for 256 "+
			"iterations, which is fully determined, so the two must agree exactly",
			danger.Worst, row.WorstCycles)
	}

	for _, c := range []struct{ name, why string }{
		{"PosCtl", "its joined range is [3,5], so zero is NOT reachable and the upper bound is still the " +
			"maximum; losing it means the repair fires on any joined range rather than on zero"},
		{"ConstCtl", "a plain constant countdown — losing it means the repair keys on the latch being a " +
			"BNE instead of on zero being reachable"},
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
			t.Errorf("%s produced no measured interval", c.name)
			continue
		}
		if r.Worst < row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — an under-approximation", c.name, r.Worst, row.WorstCycles)
		}
		// ConstCtl's cost is fully determined; PosCtl's is not (its range is [3,5] and
		// the machine takes the 3 arm), so only the first is required to be exact.
		if c.name == "ConstCtl" && r.Worst != row.WorstCycles {
			t.Errorf("ConstCtl: proven %d, machine %d — a constant countdown is fully determined and "+
				"must stay exact", r.Worst, row.WorstCycles)
		}
	}
}
