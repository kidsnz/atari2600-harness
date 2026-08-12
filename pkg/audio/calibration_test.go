package audio

import (
	"math"
	"testing"
)

// CALIBRATION, not agreement. Every instrument below is fed an input this file BUILDS,
// whose answer is arithmetic rather than an emulator reading, and asserted against it.
//
// The distinction is the whole point and it cost this project a year. MeasurePeriod was
// "verified" against the machine at four operating points and agreed with it at all four
// — while returning a clean FRACTION of the period on five of the nine waveforms the TIA
// has. An instrument checked against the machine it reads proves the two agree; only a
// known answer proves it is right.

// square builds n samples of a two-level wave with the given period and duty, so the
// period is known by construction and not by measurement.
func square(period, n int, duty float64) []uint8 {
	out := make([]uint8, n)
	hi := int(float64(period) * duty)
	for i := range out {
		if i%period < hi {
			out[i] = 15
		}
	}
	return out
}

func TestMeasureFundamentalRecoversAPeriodItWasNeverTold(t *testing.T) {
	for _, p := range []int{2, 7, 30, 93, 310} {
		got, corr := MeasureFundamental(square(p, p*40, 0.5), 2, p*3)
		if got != p {
			t.Errorf("a square of period %d measured %d (r=%.3f)", p, got, corr)
		}
		if corr < 0.9 {
			t.Errorf("period %d: correlation %.3f on an exactly periodic signal", p, corr)
		}
	}
	// 13:18 is AUDC 6's real duty, measured on the machine — an asymmetric pulse must
	// come back as one period, not two.
	if got, _ := MeasureFundamental(square(31, 31*40, 13.0/31.0), 2, 93); got != 31 {
		t.Errorf("a 13:18 pulse of period 31 measured %d", got)
	}
	// It must refuse rather than invent when there is nothing to measure.
	if got, _ := MeasureFundamental(make([]uint8, 200), 2, 60); got != 0 {
		t.Errorf("a constant signal returned period %d; DC has no period", got)
	}
	// AND IT MUST REFUSE lo=1. A two-level signal of long runs correlates at r>0.98 with
	// itself at lag 1, so a search from 1 returns 1 for everything. This calibration found
	// that on its first run: period 310 measured 1 at r=0.987.
	if got, _ := MeasureFundamental(square(310, 310*8, 0.5), 1, 930); got != 0 {
		t.Errorf("lo=1 was accepted and returned %d; every long-run signal answers 1 there", got)
	}
}

// MeasurePeriod's limit, pinned exactly — and the first draft of this test had it WRONG,
// which is worth recording. It asserted that an asymmetric 13:18 pulse breaks MeasurePeriod;
// the calibration ran and MeasurePeriod returned 31.00, correct. Asymmetry is not the
// problem. TRANSITIONS PER CYCLE are: mean-interval-times-two is the period whenever a
// cycle has exactly two runs, however lopsided, and it is off by (runs/2) when it has more.
// Saw has 8 runs, pitfall and buzz 16, engine 128 — those are the failures.
func TestMeasurePeriodIsRightOnTwoRunCyclesAndWrongOnMore(t *testing.T) {
	if got := MeasurePeriod(square(30, 1200, 0.5)); math.Abs(got-30) > 0.5 {
		t.Errorf("a 50%% square of period 30 measured %.2f", got)
	}
	if got := MeasurePeriod(square(31, 1240, 13.0/31.0)); math.Abs(got-31) > 1 {
		t.Errorf("a 13:18 pulse of period 31 measured %.2f — two runs a cycle is two runs "+
			"a cycle whether or not they are equal", got)
	}
	// four runs a cycle: the same period, reported half.
	four := make([]uint8, 1200)
	for i := range four {
		if m := i % 40; m < 10 || (m >= 20 && m < 30) {
			four[i] = 15
		}
	}
	if got := MeasurePeriod(four); math.Abs(got-20) > 1 {
		t.Errorf("a four-run cycle of period 40 measured %.2f, want 20 — off by runs/2, "+
			"which is the whole failure mode and the reason MeasureFundamental exists", got)
	}
}

func TestHarmonicsPutsTheEnergyWhereTheArithmeticSaysItIs(t *testing.T) {
	// A 50% square has ODD harmonics only, falling as 1/n. Both facts are arithmetic.
	h := Harmonics(square(64, 64*20, 0.5), 64, 8)
	if h == nil {
		t.Fatal("no spectrum from an exactly periodic square")
	}
	for _, even := range []int{1, 3, 5, 7} { // indices 1,3,5,7 = harmonics 2,4,6,8
		if h[even] > 0.02 {
			t.Errorf("harmonic %d holds %.3f of a 50%% square; a symmetric square has no "+
				"even harmonics at all", even+1, h[even])
		}
	}
	if h[0] < 0.5 {
		t.Errorf("the fundamental holds %.3f of a 50%% square, want over half", h[0])
	}
	if r := h[0] / h[2]; math.Abs(r-3) > 0.4 {
		t.Errorf("fundamental/third = %.2f, want 3 (a square's harmonics fall as 1/n)", r)
	}
	// A pulse whose duty is 1/2 of nothing: at 25% duty the FOURTH harmonic is a null,
	// which is a property of the shape and not of the machine.
	p := Harmonics(square(64, 64*20, 0.25), 64, 8)
	if p[3] > 0.03 {
		t.Errorf("harmonic 4 holds %.3f of a 25%% pulse; that harmonic is a null there", p[3])
	}
	if got := Harmonics(make([]uint8, 100), 64, 8); got != nil && got[0] != 0 {
		t.Errorf("a constant signal reports %.3f in its fundamental", got[0])
	}
}
