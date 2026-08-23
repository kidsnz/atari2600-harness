package audioingest

import (
	"math"
	"testing"
)

// tone builds n samples of a harmonic series on f0: amps[i] is the amplitude of the
// (i+1)th harmonic, so amps[0]=0 is a missing fundamental.
func tone(rate int, secs, f0 float64, amps []float64) []float64 {
	n := int(float64(rate) * secs)
	w := make([]float64, n)
	for i := range w {
		t := float64(i) / float64(rate)
		for k, a := range amps {
			w[i] += a * math.Sin(2*math.Pi*f0*float64(k+1)*t)
		}
	}
	return w
}

// The failure this whole file exists for: a line whose fundamental is 96.9 Hz, measured
// over 110-800 Hz. F0 must return the 2nd harmonic — it has no choice — and F0Checked must
// say so and name the fundamental the band excluded.
func TestAFundamentalUnderTheSearchRangeIsReported(t *testing.T) {
	const rate = 44100
	// harmonics 1..5 with the roll-off measured on the record: 1.00 .40 .16 .06 .03
	w := tone(rate, 0.5, 96.9, []float64{1.0, 0.40, 0.16, 0.06, 0.03})

	// What F0 returns here is not the point and is not asserted: with the fundamental out
	// of reach it returns SOMETHING inside 110-800 and every choice it could make is wrong.
	// (Autocorrelation is in fact tougher than an FFT peak on this material — the real
	// failure on the record was an FFT argmax landing on the 2nd harmonic. F0 does not have
	// to be fooled for the caller to be: it is fooled by being asked the wrong question.)
	d := F0Checked(w, rate, 110, 800, 0)
	if d.Hz < 110 || d.Hz > 800 {
		t.Fatalf("F0 returned %.1f Hz, outside the range it was given", d.Hz)
	}
	if !d.Suspect() {
		t.Fatalf("the fundamental is below the band and was not flagged: %+v", d)
	}
	if math.Abs(d.SubHz-96.9) > 2 {
		t.Errorf("SubHz = %.2f, want ~96.9", d.SubHz)
	}
	if !d.BelowRange {
		t.Errorf("BelowRange = false, but %.1f Hz is under the -lo of 110", d.SubHz)
	}
}

// The negative control, and the one that matters: an implementation that flagged
// everything would pass the test above. Given a band that DOES contain the fundamental,
// the same signal must come back clean.
func TestTheSameSignalIsCleanWhenTheBandContainsTheFundamental(t *testing.T) {
	const rate = 44100
	w := tone(rate, 0.5, 96.9, []float64{1.0, 0.40, 0.16, 0.06, 0.03})

	d := F0Checked(w, rate, 85, 1000, 0)
	if math.Abs(d.Hz-96.9) > 2 {
		t.Fatalf("F0 over 85-1000 should find the fundamental, got %.1f Hz", d.Hz)
	}
	if d.Suspect() {
		t.Errorf("clean signal flagged as suspect: %+v", d)
	}
}

// A pure sine correlates just as well at 2L as at L, so a test that only asked "is there
// correlation at twice the lag" would call every sine an octave error. It must not.
func TestAPureToneIsNotAnOctaveError(t *testing.T) {
	const rate = 44100
	for _, f := range []float64{110, 220, 440} {
		d := F0Checked(tone(rate, 0.5, f, []float64{1.0}), rate, 80, 1000, 0)
		if math.Abs(d.Hz-f) > 2 {
			t.Fatalf("%.0f Hz sine read as %.2f", f, d.Hz)
		}
		if d.Suspect() {
			t.Errorf("%.0f Hz sine flagged as an octave error: sub %.2f ncc %.3f vs %.3f",
				f, d.SubHz, d.SubNCC, d.NCC)
		}
	}
}

// A squarewave has no even harmonics. Read from above its fundamental the surviving
// partials are 3f, 5f and 7f, whose in-band autocorrelation peak sits at no simple ratio to
// f — which is why the check searches below the band rather than testing integer multiples
// of what it found. This is the case that broke the first implementation.
func TestAThirdHarmonicCaseReportsRatioThree(t *testing.T) {
	const rate = 44100
	w := tone(rate, 0.5, 100, []float64{0, 0, 1.0, 0, 0.6, 0, 0.4})
	d := F0Checked(w, rate, 250, 1200, 60)
	if !d.Suspect() {
		t.Fatalf("odd-harmonics-only signal not flagged: %+v", d)
	}
	if math.Abs(d.SubHz-100) > 3 {
		t.Errorf("SubHz = %.2f, want ~100", d.SubHz)
	}
}

// The bug this found in F0 itself: with the true period outside loHz..hiHz the peak lands
// on the edge of the search range, where the parabola it interpolates is a slope rather
// than a summit and the correction runs away. Before the clamp this returned -488 Hz.
func TestF0NeverReturnsAFrequencyOutsideItsOwnRange(t *testing.T) {
	const rate = 44100
	for _, c := range []struct {
		f0       float64
		amps     []float64
		lo, hi   float64
	}{
		{96.9, []float64{1.0, 0.40, 0.16, 0.06, 0.03}, 110, 800},
		{50, []float64{1.0, 0.5, 0.25}, 200, 900},
		{440, []float64{1.0, 0.3}, 60, 120},
	} {
		hz, _ := F0(tone(rate, 0.4, c.f0, c.amps), rate, c.lo, c.hi)
		if hz != 0 && (hz < c.lo*0.98 || hz > c.hi*1.02) {
			t.Errorf("f0=%.0f searched %.0f-%.0f: returned %.2f Hz, outside its own range",
				c.f0, c.lo, c.hi, hz)
		}
	}
}
