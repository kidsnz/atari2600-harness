package vismatch

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
)

const glideROM = "../../roms/litmus/motion_glide.bin"

// The problem, stated as a measurement. motion_glide moves a ball down one
// scanline per frame, so the SAME ROM compared at two different frames reports a
// difference in the ball — and the difference is entirely about when it was
// looked at, not about the ball.
func TestUnmatchedFramesReportAMovingObjectAsDifferent(t *testing.T) {
	a, err := ExtractROM(glideROM, "NTSC", 10, false)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	b, err := ExtractROM(glideROM, "NTSC", 20, false)
	if err != nil {
		t.Fatal(err)
	}
	rep := Diff(a, b)
	if rep.Match {
		t.Fatal("premise broken: the ball should be somewhere else after ten more frames")
	}
	if rep.Missing["BL"] == 0 && rep.Extra["BL"] == 0 {
		t.Errorf("expected the BALL to be what differs; got missing=%v extra=%v", rep.Missing, rep.Extra)
	}
}

// And the fix, stated the same way: drive both through the same script first and
// the same comparison is exact. Whatever remains after that is position
// fidelity rather than a difference in game state.
func TestMatchedStateMakesTheSameROMExact(t *testing.T) {
	scn := behavmatch.Library["p0-right"]
	ea, err := behavmatch.RunScenario(glideROM, "NTSC", scn, 0)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	eb, err := behavmatch.RunScenario(glideROM, "NTSC", scn, 0)
	if err != nil {
		t.Fatal(err)
	}
	rep := Diff(GridFrom(ea), GridFrom(eb))
	if !rep.Match {
		t.Errorf("the same ROM driven through the same script should be pixel-exact; "+
			"missing=%v extra=%v", rep.Missing, rep.Extra)
	}
}

// Matched state must not flatten a real difference into agreement. Two ROMs whose
// pictures genuinely differ still have to come out different after the drive —
// otherwise the feature would turn every comparison green.
func TestMatchedStateStillSeesARealDifference(t *testing.T) {
	scn := behavmatch.Library["p0-right"]
	ea, err := behavmatch.RunScenario(glideROM, "NTSC", scn, 0)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	eb, err := behavmatch.RunScenario("../../roms/litmus/litmus_shift_base.bin", "NTSC", scn, 0)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	rep := Diff(GridFrom(ea), GridFrom(eb))
	if rep.Match {
		t.Error("two different ROMs came out pixel-exact after a matched drive")
	}
}
