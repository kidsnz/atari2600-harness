package scenario

import (
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
)

// A blank region over budget is a ROLL: the WSYNC that follows it waits for the next
// line, and the frame comes out one scanline long. cyclebound has always FOUND those —
// they land in BlankOver — but `Certified` covers the visible regions only, and both
// the CLI headline and this package's prove_line_budget gate passed that field
// straight through. So a ROM with a 77-cycle VBLANK line printed "CERTIFIED" and its
// scenario went green.
//
// Measured 2026-08-09 on roms/technojacket (the atari2600-roms repo): adding two
// instructions to a VBLANK line took one path to 77 cycles, five frames in three
// hundred came out at 263 lines, and ntsc_frame_lines/frame_lines_stable were the only
// checks that noticed. prove_line_budget — the ∀-over-all-paths gate, the one whose
// whole claim is that it does not need to catch the bad frame in a sample — was green.
//
// cb_blank_noamax is the standing fixture for this shape: an un-annotated divide loop
// in a blank region that proves 107 cycles against a 76-cycle budget while every
// visible region fits.
func TestProveLineBudgetFailsOnABlankOverrun(t *testing.T) {
	const fixture = "../../roms/litmus/cb_blank_noamax.asm"
	rep, err := cyclebound.Prove(fixture, 76)
	if err != nil {
		t.Fatalf("Prove(%s): %v", fixture, err)
	}

	// The fixture has to keep being the case this test is about. If someone annotates
	// the loop, the witness silently stops witnessing.
	if len(rep.BlankOver) == 0 {
		t.Fatalf("%s no longer has a blank region over budget, so it cannot witness this; "+
			"blank_max_worst=%d", fixture, rep.BlankMaxWorst)
	}
	if !rep.Certified {
		t.Fatalf("%s must still be visible-clean — otherwise the gate could pass for the "+
			"ordinary reason and this test would prove nothing about blank regions", fixture)
	}

	budget := 76
	s := &Scenario{Rom: fixture, Frames: 2, Checks: &Checks{ProveLineBudget: &budget}}
	res, err := Run(s, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Pass {
		t.Fatalf("prove_line_budget PASSED a ROM with %d blank region(s) at %d cycles against a "+
			"budget of %d. That is the defect: the frame gains a scanline and the ∀ gate says nothing.",
			len(rep.BlankOver), rep.BlankMaxWorst, budget)
	}
	var found string
	for _, a := range res.Asserts {
		if strings.HasPrefix(a.Desc, "prove_line_budget") {
			found = a.Desc
			if a.Pass {
				t.Errorf("the prove_line_budget assertion itself passed: %q", a.Desc)
			}
		}
	}
	if found == "" {
		t.Fatal("no prove_line_budget assertion was recorded at all")
	}
	if !strings.Contains(found, "BLANK") {
		t.Errorf("the verdict does not say WHY it failed: %q — a reader has to be told it is a "+
			"blank overrun, not a visible one, or they will look in the wrong place", found)
	}
}

// The negative control. The annotated twin bounds the same loop at 67 cycles, so its
// blank regions fit and the gate must go green — otherwise the check above would pass
// for any ROM at all and would be measuring nothing.
func TestProveLineBudgetStillPassesWhenTheBlankRegionFits(t *testing.T) {
	const fixture = "../../roms/litmus/cb_blank_amax.asm"
	rep, err := cyclebound.Prove(fixture, 76)
	if err != nil {
		t.Fatalf("Prove(%s): %v", fixture, err)
	}
	if len(rep.BlankOver) != 0 {
		t.Fatalf("%s is supposed to FIT; got %d blank region(s) over budget", fixture, len(rep.BlankOver))
	}
	if rep.BlankMaxWorst == 0 {
		t.Fatal("the control has no measured blank region at all, so it controls for nothing")
	}

	budget := 76
	s := &Scenario{Rom: fixture, Frames: 2, Checks: &Checks{ProveLineBudget: &budget}}
	res, err := Run(s, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Pass {
		t.Fatalf("the annotated twin must still pass; it proves %d cycles in its blank regions "+
			"against a budget of %d", rep.BlankMaxWorst, budget)
	}
}
