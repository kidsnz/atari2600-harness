package audioingest

import (
	"math"
	"testing"
)

// The 233 ms error, in miniature: a file that does not start at the music.
func TestLeadingSilenceFindsWhereTheMusicStarts(t *testing.T) {
	const rate = 44100
	for _, gap := range []float64{0, 0.05, 0.233, 1.0} {
		x := make([]float64, int(3*rate))
		for i := int(gap * rate); i < len(x); i++ {
			x[i] = math.Sin(2 * math.Pi * 200 * float64(i) / rate)
		}
		got := LeadingSilence(x, rate, 0.02)
		if math.Abs(got-gap) > 0.01 {
			t.Errorf("gap %.3f s: measured %.4f", gap, got)
		}
	}
}

// The negative control: a quiet recording is not a silent lead-in. An absolute threshold would
// call the whole of this file silence, which is why the measure is relative to the peak.
func TestAQuietFileIsNotAllSilence(t *testing.T) {
	const rate = 44100
	x := make([]float64, 2*rate)
	for i := range x {
		x[i] = 0.0005 * math.Sin(2*math.Pi*200*float64(i)/rate) // -66 dBFS throughout
	}
	if got := LeadingSilence(x, rate, 0.02); got > 0.01 {
		t.Errorf("a uniformly quiet file reported %.3f s of leading silence", got)
	}
}

// two-bar and one-bar loops, built so the answer is known
func loop(rate int, bars int, barSec float64, total int, alt bool) []float64 {
	n := int(float64(total) * barSec * float64(rate))
	x := make([]float64, n)
	step := barSec / 16
	for b := 0; b < total; b++ {
		for s := 0; s < 16; s++ {
			hit := s%4 == 0
			if alt && b%bars == 1 {
				hit = s%4 == 2 // the odd bar puts its hits somewhere else
			}
			if !hit {
				continue
			}
			i0 := int((float64(b)*barSec + float64(s)*step) * float64(rate))
			for i := i0; i < i0+rate/40 && i < n; i++ {
				x[i] = 1
			}
		}
	}
	return x
}

func TestPatternBarsFindsATwoBarLoop(t *testing.T) {
	const rate = 44100
	x := loop(rate, 2, 1.9, 16, true)
	got, scores := PatternBars(x, rate, 0, 1.9, 8)
	if got != 2 {
		t.Errorf("two-bar loop read as %d bars (scores %v)", got, scores)
	}
}

// The negative control that matters: a gate reporting 2 for everything would pass the test
// above. A one-bar loop must read as one bar.
func TestPatternBarsFindsAOneBarLoop(t *testing.T) {
	const rate = 44100
	x := loop(rate, 1, 1.9, 16, false)
	got, scores := PatternBars(x, rate, 0, 1.9, 8)
	if got != 1 {
		t.Errorf("one-bar loop read as %d bars (scores %v)", got, scores)
	}
}

// It must report the SMALLEST true period, not a multiple of it — a two-bar pattern also
// correlates at four and eight, and answering eight would be true and useless.
func TestPatternBarsPrefersTheSmallestTruePeriod(t *testing.T) {
	const rate = 44100
	got, _ := PatternBars(loop(rate, 2, 1.9, 24, true), rate, 0, 1.9, 8)
	if got != 2 {
		t.Errorf("got %d, want the smallest true period 2", got)
	}
}

// BandPass must pass what is in the band and stop what is not, and must not shift events in
// time — the output is used to locate hits, so a phase shift would move the grid.
func TestBandPassSelectsAndDoesNotShiftInTime(t *testing.T) {
	const rate = 44100
	n := rate
	inb, outb := make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		inb[i] = math.Sin(2 * math.Pi * 200 * float64(i) / rate)
		outb[i] = math.Sin(2 * math.Pi * 5000 * float64(i) / rate)
	}
	rms := func(v []float64) float64 {
		var s float64
		for _, x := range v[rate/10 : n-rate/10] {
			s += x * x
		}
		return math.Sqrt(s / float64(len(v[rate/10:n-rate/10])))
	}
	if g := rms(BandPass(inb, rate, 85, 1000)); g < 0.5 {
		t.Errorf("200 Hz through an 85-1000 band came out at %.3f rms, want ~0.7", g)
	}
	if g := rms(BandPass(outb, rate, 85, 1000)); g > 0.1 {
		t.Errorf("5000 Hz through an 85-1000 band came out at %.3f rms, want ~0", g)
	}
	// a click, filtered, must still peak where it was
	click := make([]float64, n)
	click[n/2] = 1
	f := BandPass(click, rate, 85, 1000)
	best, bi := 0.0, 0
	for i, v := range f {
		if math.Abs(v) > best {
			best, bi = math.Abs(v), i
		}
	}
	if d := bi - n/2; d < -50 || d > 50 {
		t.Errorf("a click at %d came out peaking at %d — the filter shifts events in time", n/2, bi)
	}
}

// FirstOnset is the calibration for "where does the part actually START" -- t0 for the whole
// audio chain, since audioingest takes -from and drumfit takes -t0 and both are read off this.
// A wrong t0 propagates through every later measurement with nothing downstream to catch it,
// which is how a grid sat two sixteenths out of phase for two days. Hand-built envelope with
// the peaks at known samples, so the expected answer is arithmetic rather than a recording.
func TestFirstOnsetReturnsTheTimeOfThePeakInItsWindow(t *testing.T) {
	const rate = 1000 // one sample per millisecond, so an index IS a time in ms
	env := make([]float64, rate)
	env[100] = 0.4 // an earlier, smaller bump
	env[350] = 1.0 // THE onset
	env[700] = 0.9

	for _, c := range []struct {
		after, within, want float64
		why                 string
	}{
		{0, 1.0, 0.350, "the largest peak in the whole second"},
		{0.5, 0.5, 0.700, "after= excludes the 350 ms peak, leaving 700 ms"},
		{0, 0.3, 0.100, "within= excludes both, leaving the 100 ms bump"},
	} {
		if got := FirstOnset(env, rate, c.after, c.within); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("after=%.2f within=%.2f: got %.4f s, want %.4f s (%s)",
				c.after, c.within, got, c.want, c.why)
		}
	}
}

// The negative control. An envelope with no peak must report the start of the window it was
// told to search rather than some index the loop happened to stop on: a detector that always
// answers is worse than one that answers nothing, because the number looks the same.
func TestFirstOnsetOnAFlatEnvelopeReportsTheWindowStart(t *testing.T) {
	if got := FirstOnset(make([]float64, 1000), 1000, 0.2, 0.5); math.Abs(got-0.2) > 1e-9 {
		t.Errorf("a flat envelope searched from 0.2 s reported %.4f s", got)
	}
}
