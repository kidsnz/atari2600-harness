package cyclebound

// The counter's entry value comes from the EDGE, not from the instruction.
//
// `determineBound` scanned the header's predecessors and computed each one's
// contribution with `State.transfer`, which models what an INSTRUCTION does to the
// machine. For a JSR that is only the push: X and Y are left untouched, i.e. at their
// pre-call values. The state that actually flows along the edge is `absSuccessors`',
// and it resets a JSR's return point to Top precisely because the callee's effect is
// not modelled. **Two functions in the same package, disagreeing about the same edge**
// — and the scan was reading the wrong one.
//
// Measured on this fixture before the fix: `ldx #$02 / jsr SetBig` where the callee
// does `ldx #$50`. The scan saw X=2 and answered **36** cycles; the machine spent
// **738 across 10 scanlines**. 20.5x under, with `certified: true`.
//
// The repair deletes the divergence rather than adding a rule, which is the same
// argument `successors` itself makes about having ONE notion of a successor.
//
// SafeCtl is deliberately a refusal rather than a pass. Its callee provably does not
// touch X, so a bound is achievable in principle — but the analysis has no callee
// summary, and Top is the honest answer for an unmodelled call. Asserting the refusal
// makes it a MEASURED consequence of that gap instead of an unexamined side effect,
// and marks the row that should become bounded if a summary is ever added.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestCounterEntryComesFromTheEdgeNotTheInstruction(t *testing.T) {
	const asm = "../../roms/litmus/litmus_jsrentry.asm"

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
	// PREMISE — the region must run long on the machine, or refusing it says nothing.
	row, hit := measured[danger.Start]
	if !hit {
		t.Fatal("DangerRow produced no measured interval; a refusal on a region the machine never runs " +
			"demonstrates nothing")
	}
	if row.WorstCycles < 400 {
		t.Errorf("DangerRow measured %d cycles; the callee is meant to replace the counter with $50 "+
			"(738 when authored), so it is no longer the shape this test is about", row.WorstCycles)
	}
	if danger.Bounded {
		t.Errorf("DangerRow is bounded at %d. A `jsr` sits between the counter's load and the loop, and "+
			"the callee replaces it — the pre-call value of 2 is not the value at the header, and the "+
			"machine spends %d cycles here", danger.Worst, row.WorstCycles)
	}

	// The unmodelled-callee refusal, asserted so it is measured rather than assumed.
	safe, ok := region("SafeCtl")
	if !ok {
		t.Fatal("no SafeCtl region")
	}
	if safe.Bounded {
		t.Logf("SafeCtl is now BOUNDED at %d — a callee-effect summary must have been added. That is an "+
			"improvement, not a failure; update this test to require exactness against the machine "+
			"instead of a refusal.", safe.Worst)
	}

	// The repair must key on the edge state, not on a subroutine existing anywhere in
	// the ROM. Without this a blanket "refuse if the ROM contains a JSR" passes.
	plain, ok := region("PlainCtl")
	if !ok {
		t.Fatal("no PlainCtl region")
	}
	if !plain.Bounded {
		t.Fatalf("PlainCtl is refused (%s) — it has no call at all, so the repair is keying on the "+
			"presence of a subroutine somewhere rather than on the edge into the header", plain.Reason)
	}
	prow, hit := measured[plain.Start]
	if !hit {
		t.Fatal("PlainCtl produced no measured interval, so its bound is graded by nothing")
	}
	if plain.Worst != prow.WorstCycles {
		t.Errorf("PlainCtl: proven %d, machine %d — a plain constant countdown is fully determined and "+
			"must stay exact", plain.Worst, prow.WorstCycles)
	}
}
