package drumfit

import (
	"math"
	"math/rand"
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

const rate = 31440.0

// synthKick builds a drum whose envelope is known exactly: an exponentially decaying
// sine whose frequency falls from f0 to f1 over `dur` seconds, repeated at `period`.
func synthKick(hits int, period, dur, f0, f1, tau float64) ([]float64, []float64) {
	n := int(float64(hits) * period * rate)
	x := make([]float64, n)
	var onsets []float64
	for h := 0; h < hits; h++ {
		t0 := float64(h) * period
		onsets = append(onsets, t0)
		start := int(t0 * rate)
		ph := 0.0
		for i := 0; i < int(dur*rate); i++ {
			t := float64(i) / rate
			f := f0 + (f1-f0)*(t/dur)
			ph += 2 * math.Pi * f / rate
			if start+i < n {
				x[start+i] += math.Exp(-t/tau) * math.Sin(ph)
			}
		}
	}
	return x, onsets
}

func TestMeasureRecoversTheDecayShape(t *testing.T) {
	// tau 0.05 s: after one frame (16.7 ms) the amplitude should be e^-0.33 = 0.72 of
	// the peak, and after five frames e^-1.67 = 0.19.
	x, on := synthKick(20, 0.5, 0.25, 90, 45, 0.05)
	h, err := Measure(x, rate, on, [2]float64{30, 150}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if h.Amp[0] < 0.9 {
		t.Errorf("frame 0 amplitude %.2f; the envelope is normalised so the peak is 1", h.Amp[0])
	}
	for _, c := range []struct {
		f    int
		want float64
	}{{1, 0.72}, {3, 0.37}, {5, 0.19}} {
		if math.Abs(h.Amp[c.f]-c.want) > 0.15 {
			t.Errorf("frame %d amplitude %.2f, want about %.2f", c.f, h.Amp[c.f], c.want)
		}
	}
	if h.Amp[9] >= h.Amp[1] {
		t.Errorf("the envelope is not decaying: frame 1 %.2f, frame 9 %.2f", h.Amp[1], h.Amp[9])
	}
}

func TestMeasureRecoversTheFallingPitch(t *testing.T) {
	x, on := synthKick(20, 0.5, 0.25, 90, 45, 0.20)
	h, err := Measure(x, rate, on, [2]float64{30, 150}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if h.Hz[0] < 70 || h.Hz[0] > 100 {
		t.Errorf("frame 0 pitch %.1f Hz, want near 90", h.Hz[0])
	}
	if h.Hz[7] >= h.Hz[0] {
		t.Errorf("the sweep is not falling: frame 0 %.1f Hz, frame 7 %.1f Hz", h.Hz[0], h.Hz[7])
	}
}

func TestFitQuantisesOntoTheHardware(t *testing.T) {
	x, on := synthKick(20, 0.5, 0.25, 90, 45, 0.05)
	h, _ := Measure(x, rate, on, [2]float64{30, 150}, 10)
	f := Fit(h, 6, 15, audio.BaseClockNTSC)

	if f.EnvV[0] != 15 {
		t.Errorf("peak volume %d, want 15", f.EnvV[0])
	}
	for i := 1; i < len(f.EnvV); i++ {
		if f.EnvV[i] > f.EnvV[i-1] {
			t.Errorf("volume rose at frame %d (%d after %d); the source decays monotonically",
				i, f.EnvV[i], f.EnvV[i-1])
		}
	}
	for i := range f.EnvF {
		if f.EnvF[i] < 0 || f.EnvF[i] > 31 {
			t.Fatalf("frame %d AUDF %d is outside 0..31", i, f.EnvF[i])
		}
	}
	// The pitch has to land somewhere real: divisor 31 is coarse but not useless in this
	// register, and anything worse than a semitone would mean the wrong waveform.
	worst := 0.0
	for i, c := range f.Cents {
		if h.Hz[i] > 0 && math.Abs(c) > worst {
			worst = math.Abs(c)
		}
	}
	if worst > 100 {
		t.Errorf("worst pitch error %.0f cents; AUDC 6 should track a 45-90 Hz sweep inside a semitone", worst)
	}
}

func TestFitTrimsTheSilentTail(t *testing.T) {
	// 30 frames requested, but the drum is over in about 6. The table must not carry
	// two dozen zero entries -- the envelope cursor would spend them doing nothing.
	x, on := synthKick(12, 0.7, 0.12, 90, 45, 0.02)
	h, _ := Measure(x, rate, on, [2]float64{30, 150}, 30)
	f := Fit(h, 6, 15, audio.BaseClockNTSC)
	if len(f.EnvV) >= 30 {
		t.Errorf("table is %d frames long; the tail of zeros should have been trimmed", len(f.EnvV))
	}
	if len(f.EnvV) < 3 {
		t.Errorf("table trimmed to %d frames, which is not an envelope", len(f.EnvV))
	}
	if f.EnvV[len(f.EnvV)-1] != 0 {
		t.Errorf("the table does not end at silence (last value %d)", f.EnvV[len(f.EnvV)-1])
	}
}

// The negative control. White noise has no drum in it; the fitter must not return a
// confident envelope for one.
func TestNoiseDoesNotProduceAFallingSweep(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	x := make([]float64, int(6*rate))
	for i := range x {
		x[i] = rng.NormFloat64()
	}
	var on []float64
	for i := 0; i < 12; i++ {
		on = append(on, float64(i)*0.5)
	}
	h, err := Measure(x, rate, on, [2]float64{30, 150}, 10)
	if err != nil {
		t.Fatal(err)
	}
	// A real kick loses most of its level within ten frames. Noise does not decay at all.
	if h.Amp[9] < 0.5 {
		t.Errorf("white noise decayed to %.2f of peak in 10 frames; it should stay near 1, "+
			"and if it does not, this fitter will invent envelopes out of anything", h.Amp[9])
	}
}

func TestMeasureRefusesTooFewOnsets(t *testing.T) {
	x, on := synthKick(2, 0.5, 0.25, 90, 45, 0.05)
	if _, err := Measure(x, rate, on, [2]float64{30, 150}, 10); err == nil {
		t.Fatal("two onsets were accepted; a single hit carries whatever else was in the mix")
	}
	if _, err := Measure(x, rate, []float64{0, 0.5, 1.0}, [2]float64{6000, 14000}, 10); err == nil {
		t.Fatal("a band with no energy in it was accepted")
	}
}
