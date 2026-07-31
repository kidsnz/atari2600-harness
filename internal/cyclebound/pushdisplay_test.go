package cyclebound

import (
	"strings"
	"testing"
)

// regionAt returns the visible/blank region opened at the named source location.
func regionAt(t *testing.T, rep *Report, loc string) (Region, string) {
	t.Helper()
	for _, r := range rep.Lines {
		if strings.HasPrefix(r.StartLoc, loc) {
			return r, "visible"
		}
	}
	for _, r := range rep.BlankLines {
		if strings.HasPrefix(r.StartLoc, loc) {
			return r, "blank"
		}
	}
	t.Fatalf("no region opens at %q", loc)
	return Region{}, ""
}

// TestPushThatCanReachTheDisplayKeepsItsRegionVisible witnesses `pushMissesDisplay`'s
// "SP can reach $0100/$0101" branch, which had run ZERO times across 129 ROMs.
//
// A PHA writes to $0100|SP, and page 1 mirrors the addresses the console decodes, so a
// program that points SP at the bottom of the stack turns a push into a write to VSYNC
// or VBLANK — the Stack Trick. That is the entire reason the prover tracks SP here
// instead of treating every push as display-touching, and it was the half of the
// predicate nothing exercised: the one ROM that reached it (rts_dispatch) took the
// "proved to miss" path, and litmus_stack_trick — the fixture written for this very
// hazard — never reached the predicate at all.
//
// The consequence is not cosmetic. A region whose display is off and which touches
// nothing is classified BLANK and skipped, so a push wrongly judged harmless removes a
// region's cost from the budget check.
//
// The twins differ by one immediate operand (the value TXS loads), so the change in
// classification is attributable to the SP range and nothing else. Their worst-case
// cost is identical, which is the premise check: the code is the same, only the verdict
// about it moved.
func TestPushThatCanReachTheDisplayKeepsItsRegionVisible(t *testing.T) {
	const pushRegion = "OS+5"

	danger, err := Prove("../../roms/litmus/cb_pushdisplay.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	safe, err := Prove("../../roms/litmus/cb_pushsafe.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}

	dReg, dKind := regionAt(t, danger, pushRegion)
	sReg, sKind := regionAt(t, safe, pushRegion)

	if dKind != "visible" {
		t.Errorf("the push lands on $0101 (VBLANK) with SP=1, but its region was classified %q — "+
			"a blank region is skipped, so the cost of everything in it stops being checked", dKind)
	}
	if sKind != "blank" {
		t.Errorf("the twin's push lands at $01FF, ordinary stack RAM, and its region came back %q "+
			"instead of blank — then the danger ROM's classification proves nothing", sKind)
	}

	// Premise: same code, same cost. If these ever differ the twins have drifted apart
	// and the comparison is measuring something other than the SP range.
	if dReg.Worst != sReg.Worst {
		t.Errorf("the twins' region costs differ (%d vs %d cy) — they are supposed to be the same "+
			"instructions with one immediate changed", dReg.Worst, sReg.Worst)
	}
	if danger.Regions != safe.Regions {
		t.Errorf("region counts differ (%d vs %d): the twins are no longer the same kernel",
			danger.Regions, safe.Regions)
	}
	// And the classification really did move a region between the buckets.
	if danger.Blank != safe.Blank-1 {
		t.Errorf("blank-region counts are %d (danger) and %d (safe); the push should move exactly "+
			"one region out of the blank bucket", danger.Blank, safe.Blank)
	}
	t.Logf("push region %s: danger=%s safe=%s, both %d cy; blank regions %d vs %d",
		pushRegion, dKind, sKind, dReg.Worst, danger.Blank, safe.Blank)
}
