package design

import (
	"math"
	"testing"
)

// TestFlickerRateLadder pins the half of the flicker decision `NeedsFlicker` does not carry.
//
// `NeedsFlicker` is a yes or no: it returns the same answer for three objects and for twenty.
// `FlickerRateHz` is how much, and the archive supplies both ends of the ladder — Glenn Saunders'
// *"never really necessary to drop below 30hz"* 〔`199709/msg00139`〕 and Piero Cavina's
// *"'Adventure' must be the king of flicker"* five days later 〔`199709/msg00218`〕.
func TestFlickerRateLadder(t *testing.T) {
	for _, c := range []struct {
		objects int
		subsets int
		hz      float64
	}{
		{1, 1, 59.9227},
		{2, 1, 59.9227}, // both players, no flicker at all
		{3, 2, 29.9614}, // the first rung, and the one Saunders calls sufficient
		{4, 2, 29.9614},
		{12, 6, 9.9871},
		{24, 12, 4.9936}, // Cavina's "5hZ, maybe?" for a crowded Adventure room
	} {
		if got := SubsetsFor(c.objects); got != c.subsets {
			t.Errorf("%d objects need %d subsets, want %d", c.objects, got, c.subsets)
		}
		if got := FlickerRateHz(c.objects); math.Abs(got-c.hz) > 0.01 {
			t.Errorf("%d objects draw at %.4f Hz, want %.4f", c.objects, got, c.hz)
		}
	}

	// Two objects must not flicker at all — that is the whole point of the two player slots, and a
	// rate below the frame rate there would mean the ladder starts in the wrong place.
	if r := FlickerRateHz(DistinctPlayerSprites); r != NTSCFrameRateHz {
		t.Errorf("%d objects draw at %.4f Hz, want the full %.4f — NeedsFlicker says they do not "+
			"need flicker, so the rate must agree with it", DistinctPlayerSprites, r, NTSCFrameRateHz)
	}
	// And the first rung must be exactly half, or "30 Hz" is not what this function means by it.
	if r := FlickerRateHz(DistinctPlayerSprites + 1); math.Abs(r-NTSCFrameRateHz/2) > 0.001 {
		t.Errorf("one object past the slots draws at %.4f Hz, want half the frame rate %.4f",
			r, NTSCFrameRateHz/2)
	}
	// Nonsense in, zero out, rather than a division by zero dressed as a rate.
	if FlickerRateHz(0) != 0 || FlickerRateHz(-3) != 0 {
		t.Error("a non-positive object count should give 0 Hz, not a number that looks like a rate")
	}

	// The constant must stay the 2600's own frame rate — not the nominal 60, and not 60.0544, the
	// broadcast line rate over 262 lines (15734.26 / 262, the 227.5-colour-clock line). subpixel-velocity.md's conversion factor
	// is derived from the same number and they must not drift apart. The check goes the other way
	// round from the constant: the 1,193,182 Hz CPU clock over 76 cycles per line over 262 lines.
	if cpu := 1193182.0 / 76 / 262; math.Abs(NTSCFrameRateHz-cpu) > 0.001 {
		t.Errorf("NTSCFrameRateHz is %.4f but 1193182/76/262 is %.4f — the colour clock moved, "+
			"or this was set to the broadcast line rate over 262 lines (60.0544) or rounded to 60", NTSCFrameRateHz, cpu)
	}
}
