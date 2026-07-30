package scenario

import (
	"strings"
	"testing"
)

// TestMotionSmoothnessAlonePassesOnAFrozenSprite pins the hole `min_span` closes, and
// keeps the hole itself on the record.
//
// jerk_rms is the RMS of the position's 2nd difference: 0 for constant velocity, and 0
// for an object that never moves at all. A gate on smoothness alone therefore certifies
// nothing — measured on litmus_pos, whose P0 is pinned at one X for the whole run, the
// check reports jerk_rms 0 and PASSES. The judder regression it exists to catch and a
// completely dead kernel are indistinguishable to it.
//
// So the span is now reported unconditionally (a scenario that has been gating a frozen
// object says "span 0" in its own output) and gated when min_span is set.
func TestMotionSmoothnessAlonePassesOnAFrozenSprite(t *testing.T) {
	t.Chdir("../..")

	// litmus_pos holds P0 at a fixed X for the entire run.
	frozen := func(minSpan int) *Scenario {
		return &Scenario{
			Rom: "roms/litmus/litmus_pos.asm",
			Checks: &Checks{Motion: &MotionCheck{
				Object: "P0", Axis: "x", Frames: 40, Warmup: 3,
				MaxJerkRMS: 0.5, MinSpan: minSpan,
			}},
		}
	}

	res, err := Run(frozen(0), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("ungated smoothness on a frozen sprite unexpectedly failed — if this ROM now "+
			"moves P0, the fixture is wrong and this test proves nothing: %+v", res.Asserts)
	}
	if len(res.Asserts) != 1 || !strings.Contains(res.Asserts[0].Desc, "span 0") {
		t.Errorf("the measured span must appear in the output even when nobody gated it, so a "+
			"scenario quietly certifying a motionless object shows it: %+v", res.Asserts)
	}

	res, err = Run(frozen(10), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pass {
		t.Errorf("min_span=10 passed on a sprite that travelled 0px: %+v", res.Asserts)
	}

	// And the gate must not fire on an object that really does move: motion_glide's ball
	// descends 1px/frame, so it clears any sane span while staying perfectly smooth.
	moving := &Scenario{
		Rom: "roms/litmus/motion_glide.asm",
		Checks: &Checks{Motion: &MotionCheck{
			Object: "BL", Frames: 40, Warmup: 5, YTop: 40, YBot: 230,
			MaxJerkRMS: 0.5, MinSpan: 30,
		}},
	}
	if res, err := Run(moving, false); err != nil {
		t.Fatal(err)
	} else if !res.Pass {
		t.Errorf("a genuinely gliding object failed the span gate: %+v", res.Asserts)
	}
}
