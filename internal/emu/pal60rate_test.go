package emu

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// TestPAL60DeclaresOneLineRateAndUsesAnother measures an inconsistency in the vendored engine
// that this repository is currently insulated from **by accident rather than by design**, and
// pins both halves so the insulation cannot quietly end.
//
// `specifications.go` builds each spec as a literal and then overwrites `RefreshRate` with a
// division. Four of the five divide their OWN fields:
//
//	SpecNTSC.RefreshRate  = SpecNTSC.HorizontalScanRate  / SpecNTSC.ScanlinesTotal
//	SpecPAL.RefreshRate   = SpecPAL.HorizontalScanRate   / SpecPAL.ScanlinesTotal
//	SpecPAL_M.RefreshRate = SpecPAL_M.HorizontalScanRate / SpecPAL_M.ScanlinesTotal
//	SpecSECAM.RefreshRate = SpecSECAM.HorizontalScanRate / SpecSECAM.ScanlinesTotal
//
// PAL60 does not:
//
//	SpecPAL60.RefreshRate = SpecNTSC.HorizontalScanRate / SpecNTSC.ScanlinesTotal
//
// So `SpecPAL60.HorizontalScanRate` is 15625.00 while the rate its own `RefreshRate` implies is
// 15734.26 — a gap of 0.4170 Hz, 0.70%. The engine's arithmetic is right and the declared field
// is the wrong one: PAL60 is PAL colour on 60 Hz timing, and **PAL-M, which is the same 262-line
// 60 Hz geometry, declares 15734.26**. The two cannot both be correct.
//
// **Why this matters here even though nothing breaks.** `internal/ceiling/palette.go` resolves
// `SpecPAL60` — but only to reach its colour generator. `HorizontalScanRate` and `RefreshRate`
// are read **nowhere** in `internal/`, `pkg/` or `cmd/`; timing in this harness comes from
// scanline counts, not from hertz. That is the only reason the bad field costs nothing, and it
// is not a decision anyone made. The second assertion below is what makes the situation
// falsifiable: if a later change starts reading these fields, or if upstream corrects the
// literal, this test fails and whoever is standing there reads this comment.
//
// Found by the mailing-list distillation (helper-2), who raised the discrepancy as a decision
// for the author; measured here, where it turns out not to need one.
func TestPAL60DeclaresOneLineRateAndUsesAnother(t *testing.T) {
	// Every spec's declared rate over its own line count.
	own := func(s specification.Spec) float32 {
		return s.HorizontalScanRate / float32(s.ScanlinesTotal)
	}

	for _, s := range []specification.Spec{
		specification.SpecNTSC,
		specification.SpecPAL,
		specification.SpecPAL_M,
		specification.SpecSECAM,
	} {
		if got, want := s.RefreshRate, own(s); got != want {
			t.Errorf("%s: RefreshRate %.4f is not its own %.2f/%d = %.4f — this spec used to be "+
				"self-consistent, so either the engine changed or this test's premise did",
				s.ID, got, s.HorizontalScanRate, s.ScanlinesTotal, want)
		}
	}

	// PAL60 is the exception, and the exception is the point.
	p := specification.SpecPAL60
	if p.RefreshRate == own(p) {
		t.Fatalf("PAL60 is now self-consistent (%.4f Hz from its own %.2f/%d). Upstream has "+
			"corrected the declared HorizontalScanRate or the division; delete this test and "+
			"the note in docs/fundamentals-audit.md rather than leaving a stale warning",
			p.RefreshRate, p.HorizontalScanRate, p.ScanlinesTotal)
	}
	if p.RefreshRate != own(specification.SpecNTSC) {
		t.Errorf("PAL60 RefreshRate %.4f no longer equals NTSC's %.4f — the division has been "+
			"changed to something else again, so re-measure before trusting either field",
			p.RefreshRate, own(specification.SpecNTSC))
	}

	// The size of the gap, so a future reader does not have to recompute it to decide whether
	// it matters. 0.70% is small enough to hide in a frame-count check and large enough to
	// matter to anything expressed in hertz.
	gap := p.RefreshRate - own(p)
	if gap < 0.41 || gap > 0.42 {
		t.Errorf("the PAL60 gap is %.4f Hz, not the 0.4170 measured on 2026-09-07 — the numbers "+
			"moved, so the note that describes them is stale too", gap)
	}

	// PAL-M is the internal witness that 15625.00 is the wrong half: same geometry, correct rate.
	m := specification.SpecPAL_M
	if m.ScanlinesTotal != p.ScanlinesTotal {
		t.Fatalf("PAL-M is %d lines and PAL60 is %d; they are no longer the same geometry, so "+
			"PAL-M stops being evidence about PAL60's declared rate",
			m.ScanlinesTotal, p.ScanlinesTotal)
	}
	if m.HorizontalScanRate == p.HorizontalScanRate {
		t.Errorf("PAL-M and PAL60 now declare the same %.2f — the disagreement this test exists "+
			"to record is gone", m.HorizontalScanRate)
	}
}
