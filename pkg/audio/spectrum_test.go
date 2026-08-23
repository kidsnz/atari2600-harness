package audio

import (
	"math"
	"testing"
)

// A waveform must be closest to itself. Without this a metric that ranked by anything at all
// would look plausible on one example.
func TestEveryWaveformIsClosestToItself(t *testing.T) {
	for _, c := range PitchedWaveforms {
		best, bestC := math.Inf(1), -1
		for _, d := range PitchedWaveforms {
			if v := SpectrumDistance(MeasuredSpectra[c], MeasuredSpectra[d]); v < best {
				best, bestC = v, d
			}
		}
		if bestC != c {
			t.Errorf("AUDC %d (%s) is closest to AUDC %d (%s), not itself",
				c, Name(c), bestC, Name(bestC))
		}
		if best != 0 {
			t.Errorf("AUDC %d against itself is %.3f, want 0", c, best)
		}
	}
}

// AUDC 4 and 12 are one timbre in two registers — the comment on Harmonics says so and the
// measured spectra agree to .02. The metric must reflect that, or it is not measuring timbre.
func TestSquareAndLeadAreTheSameTimbre(t *testing.T) {
	d := SpectrumDistance(MeasuredSpectra[4], MeasuredSpectra[12])
	if d > 3 {
		t.Errorf("square and lead are %.1f dB apart; they are meant to be one instrument", d)
	}
	// and they must be further from everything else than from each other
	for _, c := range PitchedWaveforms {
		if c == 4 || c == 12 {
			continue
		}
		if o := SpectrumDistance(MeasuredSpectra[4], MeasuredSpectra[c]); o <= d {
			t.Errorf("square is %.1f dB from %s but %.1f dB from lead", o, Name(c), d)
		}
	}
}

// The failure that prompted this: a source rolling off 1.00 .40 .16 .06 was reproduced on
// AUDC 12, which has NO even harmonics. The metric must place 12 far from such a source, and
// must place a strong-fundamental voice near it.
func TestARollingOffSourceIsNotMatchedToASquarewave(t *testing.T) {
	ref := []float64{1.00, 0.40, 0.16, 0.06, 0.03, 0.00, 0.01, 0.01}
	lead := SpectrumDistance(ref, MeasuredSpectra[12])
	bass := SpectrumDistance(ref, MeasuredSpectra[6])
	if bass >= lead {
		t.Errorf("bass %.1f dB, lead %.1f dB: a source with a strong 2nd harmonic must not "+
			"rank a waveform that has none above one that does", bass, lead)
	}
}

// Absolute loudness is a separate 4-bit control on this machine, so scaling a spectrum must
// not move it. A metric that failed this would rank waveforms by how loud they happen to be.
func TestScalingTheReferenceDoesNotChangeTheRanking(t *testing.T) {
	ref := []float64{1.00, 0.40, 0.16, 0.06, 0.03, 0.00, 0.01, 0.01}
	for _, k := range []float64{0.001, 0.5, 7, 1000} {
		scaled := make([]float64, len(ref))
		for i, v := range ref {
			scaled[i] = v * k
		}
		for _, c := range PitchedWaveforms {
			a := SpectrumDistance(ref, MeasuredSpectra[c])
			b := SpectrumDistance(scaled, MeasuredSpectra[c])
			if math.Abs(a-b) > 1e-9 {
				t.Errorf("scaling by %g moved AUDC %d from %.4f to %.4f", k, c, a, b)
			}
		}
	}
}

// HarmonicsF is the ONE piece of arithmetic shared by a spectrum measured off a reference
// recording and one measured off the TIA -- that sharing is the whole basis for comparing
// them, so a drift here would make the two incomparable while both still looked reasonable.
// Calibrated against a signal whose harmonic content is known BY CONSTRUCTION rather than
// against the emulator, because checking an instrument against the machine it reads proves
// only that they agree.
func TestHarmonicsFOnASignalWhoseSpectrumIsKnown(t *testing.T) {
	const period = 64.0
	build := func(f func(th float64) float64) []float64 {
		x := make([]float64, 8*int(period))
		for i := range x {
			x[i] = f(2 * math.Pi * float64(i) / period)
		}
		return x
	}
	// The result is normalised to sum 1, so amplitudes 1 and 0.5 become 2/3 and 1/3.
	for _, c := range []struct {
		name string
		x    []float64
		want []float64
	}{
		{"pure sine", build(math.Sin), []float64{1, 0, 0, 0}},
		{"1 + 0.5 second harmonic",
			build(func(th float64) float64 { return math.Sin(th) + 0.5*math.Sin(2*th) }),
			[]float64{2.0 / 3, 1.0 / 3, 0, 0}},
		{"1 + 1 third harmonic",
			build(func(th float64) float64 { return math.Sin(th) + math.Sin(3*th) }),
			[]float64{0.5, 0, 0.5, 0}},
	} {
		got := HarmonicsF(c.x, period, 4)
		for i := range c.want {
			if math.Abs(got[i]-c.want[i]) > 1e-9 {
				t.Errorf("%s: harmonic %d = %.9f, want %.9f (all: %.6f)",
					c.name, i+1, got[i], c.want[i], got)
			}
		}
	}
}

// The negative control, in the shape that actually happens: a constant signal is not a
// spectrum of equal parts. The mean is removed before the transform, so a DC level has no
// harmonics at all -- and the function must say zero rather than normalise nothing into a
// flat quarter each, which is what a divide-by-total would do unguarded.
func TestHarmonicsFOnADCSignalReportsNoHarmonics(t *testing.T) {
	dc := make([]float64, 8*64)
	for i := range dc {
		dc[i] = 3.5
	}
	for i, v := range HarmonicsF(dc, 64, 4) {
		if v != 0 {
			t.Errorf("a constant signal reported harmonic %d = %g", i+1, v)
		}
	}
}
