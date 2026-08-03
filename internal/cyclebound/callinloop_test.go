package cyclebound

// A call inside a folded loop body was costed at six cycles.
//
// `foldLoops` walks the body with `nextSite()` and sums `nodeCost()`. For a JSR that
// is the RETURN address and six cycles — **the callee's cycles are dropped, once per
// iteration.** `IsBranch()` does not catch it: that is `AddressingMode == Relative &&
// Effect == Flow`, and a JSR is Absolute/Subroutine, a JMP Absolute/Flow.
//
// Measured on this fixture before the guard, with a callee of twelve `nop`s: **proven
// 48 cycles, machine 168 across 3 scanlines — 3.5x under**, with `certified: true`.
//
// The worse case is a callee containing `sta WSYNC`: the walk then steps over a REGION
// BOUNDARY, the machine's interval ends at that strobe and the proof's does not, and
// the two numbers describe different intervals. `roms/techniques/shared_setxpos.asm`
// $F054 is exactly that — `jsr SetXPos` into a routine whose second instruction is
// `sta WSYNC` — reading proven 98 against a machine 36. Sound only by accident, and
// the fixture deliberately omits the WSYNC so its own comparison stays a comparison
// rather than a category error.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestACallInsideALoopBodyIsRefused(t *testing.T) {
	const asm = "../../roms/litmus/litmus_callinloop.asm"

	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	region := func(r *Report, name string) (Region, bool) {
		for _, x := range append(append([]Region{}, r.Lines...), r.Unbounded...) {
			if len(x.StartLoc) >= len(name) && x.StartLoc[:len(name)] == name {
				return x, true
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

	danger, ok := region(rep, "DangerRow")
	if !ok {
		t.Fatal("no DangerRow region; the fixture's labels moved")
	}
	row, hit := measured[danger.Start]
	if !hit {
		t.Fatal("DangerRow produced no measured interval; refusing a region the machine never runs " +
			"demonstrates nothing")
	}
	// PREMISE — the callee must be expensive enough that dropping it is visible. If a
	// future edit shortens Burn, the fixture stops being about anything.
	if row.WorstCycles < 100 {
		t.Errorf("DangerRow measured %d cycles; the callee is twelve nops and the loop runs four times "+
			"(168 when authored), so this is no longer the shape the test is about", row.WorstCycles)
	}
	if danger.Bounded {
		t.Errorf("DangerRow is bounded at %d. Its body calls a twelve-instruction routine whose cycles "+
			"the fold drops once per iteration — the machine spends %d here",
			danger.Worst, row.WorstCycles)
	}

	for _, c := range []struct{ name, why string }{
		{"InlineCtl", "the same work written inline — losing it means the repair refuses loops rather " +
			"than calls"},
		{"StoreCtl", "ordinary memory work in the body — losing it means the repair was bought by " +
			"refusing everything that is not a register operation"},
	} {
		r, ok := region(rep, c.name)
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
		if r.Worst != row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — fully determined, so they must agree exactly",
				c.name, r.Worst, row.WorstCycles)
		}
	}
}

// TestTheLiveCallInLoopInstanceIsNowRefused pins the one real occurrence in the tree,
// so the guard is exercised by shipped code and not only by a fixture built for it.
//
// `shared_setxpos.asm` $F054 used to report 98 against a machine 36. That is not
// "conservative by 62" — the fold walked past `SetXPos`'s `sta WSYNC`, so the proof
// described an interval the machine does not have. A refusal is the honest answer
// until a callee-effect model exists.
func TestTheLiveCallInLoopInstanceIsNowRefused(t *testing.T) {
	const asm = "../../roms/techniques/shared_setxpos.asm"
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	const at = 0xF054
	for _, r := range rep.Lines {
		if int(r.Start) == at && r.Bounded {
			t.Errorf("$%04X is bounded at %d. Its loop calls SetXPos, whose second instruction is "+
				"`sta WSYNC`, so the fold walks over a region boundary and the number describes an "+
				"interval the machine does not have", at, r.Worst)
		}
	}
	var found bool
	for _, r := range rep.Unbounded {
		if int(r.Start) == at {
			found = true
			if r.Reason == "" {
				t.Errorf("$%04X is refused with no reason", at)
			}
			t.Logf("$%04X refused: %s", at, r.Reason)
		}
	}
	if !found {
		// Not a failure by itself — the region may have been restructured — but it
		// means this test is no longer watching anything, and silence about that is
		// how a check becomes decorative.
		t.Errorf("no region at $%04X in %s; this test was pinning the tree's only call-in-loop-body "+
			"instance and now grades nothing. Re-point it or delete it.", at, build.BinPathFor(asm))
	}
}
