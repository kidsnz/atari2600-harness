package mixmatch

import (
	"math"
	"math/rand"
	"testing"
)

const rate = 31440.0

// tone builds a signal from sine components, so the answer is known before measuring.
func tone(n int, parts map[float64]float64) []float64 {
	x := make([]float64, n)
	for hz, amp := range parts {
		for i := range x {
			x[i] += amp * math.Sin(2*math.Pi*hz*float64(i)/rate)
		}
	}
	return x
}

func TestMeasurePutsEnergyInTheRightBand(t *testing.T) {
	x := tone(1<<15, map[float64]float64{45: 1.0}) // one tone, in the low band only
	p, err := Measure(x, rate, Default(), "line 60-200")
	if err != nil {
		t.Fatal(err)
	}
	if p.DB["low 30-60"] < 20 {
		t.Errorf("a 45 Hz tone put only %.1f dB more in the low band than in the line band", p.DB["low 30-60"])
	}
	if p.DB["top 3k-14k"] > -50 {
		t.Errorf("a 45 Hz tone left %.1f dB in the 3-14 kHz band; with a window it should be "+
			"below -50, and it was -19.5 before one was applied", p.DB["top 3k-14k"])
	}
}

func TestMeasureRecoversAKnownBalance(t *testing.T) {
	// Two tones, the upper one exactly 6 dB (a factor of 2) below the lower.
	x := tone(1<<15, map[float64]float64{100: 1.0, 6000: 0.5})
	p, err := Measure(x, rate, Default(), "line 60-200")
	if err != nil {
		t.Fatal(err)
	}
	got := p.DB["top 3k-14k"]
	if math.Abs(got+6.02) > 1.0 {
		t.Errorf("top band measured %.2f dB below the line band, want -6.02", got)
	}
}

func TestCompareReportsTheDifferenceWithASign(t *testing.T) {
	ref, _ := Measure(tone(1<<15, map[float64]float64{100: 1.0, 6000: 0.25}), rate, Default(), "line 60-200")
	got, _ := Measure(tone(1<<15, map[float64]float64{100: 1.0, 6000: 0.50}), rate, Default(), "line 60-200")
	errs := Compare(ref, got)
	if errs[0].Band != "top 3k-14k" {
		t.Fatalf("worst band is %q, want the top band -- it is the only one that moved", errs[0].Band)
	}
	if math.Abs(errs[0].DB-6.02) > 1.0 {
		t.Errorf("top band error %.2f dB, want +6.02 (the candidate is twice as loud there)", errs[0].DB)
	}
	// and the band that did NOT move must read as unchanged
	for _, e := range errs {
		if e.Band == "line 60-200" && math.Abs(e.DB) > 0.01 {
			t.Errorf("the reference band drifted by %.3f dB; it is the normalisation point and must be 0", e.DB)
		}
	}
}

// The negative control. Comparing a signal with ITSELF must be zero everywhere; if it
// is not, every number this package produces is suspect.
func TestComparingASignalWithItselfIsZero(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	x := make([]float64, 1<<15)
	for i := range x {
		x[i] = rng.NormFloat64()
	}
	a, _ := Measure(x, rate, Default(), "line 60-200")
	b, _ := Measure(x, rate, Default(), "line 60-200")
	for _, e := range Compare(a, b) {
		if math.Abs(e.DB) > 1e-9 {
			t.Errorf("band %s differs from itself by %g dB", e.Band, e.DB)
		}
	}
}

func TestScoreCanDiscountABandTheMixCannotReach(t *testing.T) {
	errs := []Error{{"low 30-60", 1}, {"harm 200-1200", 10}}
	full := Score(errs, nil)
	discounted := Score(errs, map[string]float64{"harm 200-1200": 0})
	if full <= discounted {
		t.Fatalf("weighting did nothing: full %.1f, discounted %.1f", full, discounted)
	}
	if discounted != 1 {
		t.Errorf("discounted score %.1f, want 1 -- only the low band should remain", discounted)
	}
}

func TestMeasureRefusesAnEmptyReferenceBand(t *testing.T) {
	// A band split that does not fit the material must fail loudly, not divide by zero
	// and report a balance made of nothing.
	if _, err := Measure(make([]float64, 1<<15), rate, Default(), "line 60-200"); err == nil {
		t.Fatal("normalising a silent signal against an empty band was accepted")
	}
	x := tone(1<<15, map[float64]float64{45: 1.0})
	if _, err := Measure(x, rate, Default(), "no such band"); err == nil {
		t.Fatal("an unknown reference band name was accepted")
	}
}
