package cyclebound

import "testing"

// TestConstantIndexPageCrossIsCharged is the only witness in the repo for a
// soundness bug found on 2026-07-30: the page-cross penalty was not charged for an
// indexed read with a CONSTANT index, however far the access reached.
//
// pagePenalty compared base+Lo against base+Hi — "does the range of possible targets
// straddle a page boundary". The 6502 charges its extra cycle on a different
// question: does the TARGET land in a different page from the BASE. With a constant
// index Lo == Hi, so the old test always compared equal and returned 0.
//
// litmus_pagecross runs `ldy #200` and four `lda Tbl,y` in each of two WSYNC regions.
// NearTbl at $F100 reaches $F1C8, inside its own page. FarTbl at $F0F8 reaches $F1C0,
// a page beyond. Measured on the same ROM:
//
//	before   NearRow 30   FarRow 35
//	after    NearRow 30   FarRow 39
//
// Four crossing reads, four cycles the hardware spends and the proof did not — an
// under-approximation, the direction this package forbids, and the same shape as
// SD-9. No corpus ROM changed (123 of 123 identical), so this fixture is the only
// thing that can catch it coming back.
//
// Do not compare NearRow against FarRow directly: the two regions hold the same
// instructions at different addresses and differ by a few cycles for unrelated
// reasons (they were 30 and 35 before the fix). Only the before/after of one region
// is meaningful, which is what the numbers below pin.
func TestConstantIndexPageCrossIsCharged(t *testing.T) {
	rep := mustProve(t, "../../roms/litmus/litmus_pagecross.asm", 76)

	worst := map[string]int{}
	for _, r := range rep.Lines {
		switch {
		case contains(r.StartLoc, "NearRow"):
			worst["near"] = r.Worst
		case contains(r.StartLoc, "FarRow"):
			worst["far"] = r.Worst
		}
	}
	if len(worst) != 2 {
		t.Fatalf("expected a NearRow and a FarRow region, found %v — the fixture no longer has the "+
			"shape this test reads", worst)
	}
	if worst["near"] != 30 {
		t.Errorf("NearRow worst = %d, want 30. This region's reads stay inside their page, so nothing "+
			"about the page-cross rule should touch it", worst["near"])
	}
	if worst["far"] != 39 {
		t.Errorf("FarRow worst = %d, want 39. It was 35 while pagePenalty asked whether the RANGE of "+
			"targets straddled a page instead of whether the target left the base's page — with a "+
			"constant index that is always false, so four crossing reads were charged nothing",
			worst["far"])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
