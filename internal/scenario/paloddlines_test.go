package scenario

import (
	"strings"
	"testing"
)

// TestAnOddPALFrameFailsOnParityAlone is the witness the parity check did not have.
//
// `design.ScrollScanlinesConstant` requires a constant frame length and, on PAL, an EVEN one — an
// odd count makes a PAL set drop colour entirely. It was wired into `frame_lines_stable` on
// 2026-09-06, and until this test existed its reachability had been shown only by MUTATION:
// inverting the parity test turned `pal.json` red. **A branch demonstrated only by breaking it is
// not demonstrated.** `roms/litmus/pal_odd_lines.asm` is a PAL frame of 311 lines — VSYNC 3 +
// VBLANK 45 + picture 228 + overscan 35 — identical every frame.
//
// ★The separation is the point. The constancy half PASSES here (311 twenty times over) and only
// the parity half fails, so this fixture is a witness for one assertion rather than for two. The
// stability check on its own is perfectly happy with a ROM that a PAL television would render in
// black and white.
//
// ★★And the check does not need the scenario to declare `lines`. `FrameLinesStableCheck.Lines` is
// `omitempty`; every scenario in the tree currently sets it, and every value is even, so the parity
// rule has never yet had anything to catch. It would catch this one on a scenario that set nothing
// but `frames` — which is what this test does.
//
// The rule is seven years deep on the list — Eckhard Stolberg's PALLINES.BIN, 1997, and his own
// retraction of a wrong version of it ten days earlier 〔stella-list `199703/msg00258`,
// `199703/msg00204`〕. Found unwired by the mailing-list distillation (helper-3), who also asked
// for exactly this witness rather than accepting the mutation as proof.
func TestAnOddPALFrameFailsOnParityAlone(t *testing.T) {
	t.Chdir("../..")

	s := &Scenario{
		Rom:    "roms/litmus/pal_odd_lines.asm",
		TVSpec: "PAL",
		Checks: &Checks{FrameLinesStable: &FrameLinesStableCheck{Frames: 20}},
	}
	res, err := Run(s, false)
	if err != nil {
		t.Fatal(err)
	}

	var stable, parity *AssertResult
	for i := range res.Asserts {
		switch {
		case strings.HasPrefix(res.Asserts[i].Desc, "frame_lines_stable"):
			stable = &res.Asserts[i]
		case strings.HasPrefix(res.Asserts[i].Desc, "design.ScrollScanlinesConstant"):
			parity = &res.Asserts[i]
		}
	}
	if stable == nil || parity == nil {
		t.Fatalf("expected both a stability assert and a parity assert, got %d asserts: %+v",
			len(res.Asserts), res.Asserts)
	}

	if !stable.Pass {
		t.Errorf("the stability half failed (%s) — this fixture is supposed to be perfectly "+
			"stable, so it can no longer isolate the parity rule", stable.Desc)
	}
	if parity.Pass {
		t.Errorf("311 lines on PAL passed the parity rule. `design.ScrollScanlinesConstant` is no " +
			"longer reached from `frame_lines_stable`, or no longer requires an even PAL count — " +
			"and a ROM a PAL television renders in black and white now reads green here")
	}
	if res.Pass {
		t.Error("the scenario as a whole passed: a failing assert is not failing the run")
	}
}
