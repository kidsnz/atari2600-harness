package cyclebound

// Every way into a folded loop must be through its header.
//
// `determineBound` takes the counter's entry value by maximising over the predecessors
// of the HEADER, which is the right set only if every execution reaching the back edge
// passed through the header. An edge landing inside the body arrives at the latch
// without crossing a scanned predecessor, so the value it carries is not in the
// maximum, and the fold charges `n` iterations for a loop entered with a different `n`.
//
// Nothing stated that premise. The body walk checks the chain is straight, cheap and
// single-bank; it never asked who else can arrive in it.
//
// Measured on this fixture before the guard: the header's only scanned predecessor
// loads X=2 while a `jmp` lands one instruction past the header with X=$50 already set.
// **Proven 40 cycles, machine 733 across 10 scanlines — 18.3x under**, with
// `certified: true` and `roll_free: true`.
//
// `JoinCtl` is the control that matters. Several predecessors OF THE HEADER are
// perfectly boundable — the scan sees them all and takes the maximum — so a guard
// keyed on "more than one predecessor" would pass the danger case while refusing a
// common and sound shape. The header is excluded from the check for exactly that
// reason.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestALoopMustBeEnteredAtItsHeader(t *testing.T) {
	const asm = "../../roms/litmus/litmus_midentry.asm"

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

	danger, ok := region("DangerRow")
	if !ok {
		t.Fatal("no DangerRow region; the fixture's labels moved")
	}
	// PREMISE — the machine must really take the mid-entry arm and run long. A guard
	// that refuses a region the hardware never enters that way proves nothing.
	row, hit := measured[danger.Start]
	if !hit {
		t.Fatal("DangerRow produced no measured interval")
	}
	if row.WorstCycles < 400 {
		t.Errorf("DangerRow measured %d cycles; the fixture is built so the machine jumps into the body "+
			"with X=$50 (733 when authored), so it is no longer the shape this test is about",
			row.WorstCycles)
	}
	if danger.Bounded {
		t.Errorf("DangerRow is bounded at %d. A `jmp` lands one instruction past the header with the "+
			"counter already at $50, which no predecessor of the header carries — the machine spends "+
			"%d cycles here", danger.Worst, row.WorstCycles)
	}

	for _, c := range []struct {
		name  string
		exact bool
		why   string
	}{
		{"JoinCtl", true, "both arms enter AT the header, so the scan sees both and takes the maximum. " +
			"Losing it means the guard fires on any loop with several predecessors, which is a common " +
			"and perfectly boundable shape"},
		{"PlainCtl", true, "a single entry — losing it means the guard fires on loops with no second " +
			"way in at all"},
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
		if c.exact && r.Worst != row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — this region's cost is fully determined on the path "+
				"the machine takes, so they must agree exactly", c.name, r.Worst, row.WorstCycles)
		}
	}
}
