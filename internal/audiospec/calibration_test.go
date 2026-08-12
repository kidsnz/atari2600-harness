package audiospec

import (
	"math"
	"testing"
)

// CALIBRATION: inputs this file builds, answers that are arithmetic. Checking a reader
// against the machine it reads proves the two agree; only a known answer proves it right.
// See scripts/check_instruments.py for why that distinction is enforced.

func TestToFloatRemovesTheMeanAndNothingElse(t *testing.T) {
	got := ToFloat([]uint8{10, 20, 30, 40})
	want := []float64{-15, -5, 5, 15} // mean 25, subtracted
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("index %d: %.3f, want %.3f", i, got[i], want[i])
		}
	}
	var sum float64
	for _, v := range got {
		sum += v
	}
	if math.Abs(sum) > 1e-9 {
		t.Errorf("the output sums to %.6f; removing the mean means summing to zero", sum)
	}
	if ToFloat(nil) != nil {
		t.Error("an empty input produced a non-nil result")
	}
}

func TestMagnitudeSpectrumPutsASineInItsOwnBin(t *testing.T) {
	// 256 samples, exactly 8 cycles: the energy belongs in bin 8 and nowhere else. Both
	// the bin and the exclusivity are arithmetic, not measurements.
	const n, cycles = 256, 8
	x := make([]float64, n)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * cycles * float64(i) / n)
	}
	m := MagnitudeSpectrum(x)
	if m == nil {
		t.Fatal("no spectrum from a pure sine")
	}
	peak := 0
	for i, v := range m {
		if v > m[peak] {
			peak = i
		}
	}
	if peak != cycles {
		t.Errorf("the peak is bin %d, want %d — a sine of %d cycles in %d samples lands "+
			"in bin %d by definition", peak, cycles, cycles, n, cycles)
	}
	var off float64
	for i, v := range m {
		if i != cycles && i != 0 {
			off += v
		}
	}
	if off > m[cycles]*0.1 {
		t.Errorf("energy outside the peak bin is %.1f against %.1f in it; a whole number of "+
			"cycles leaks nowhere", off, m[cycles])
	}
	if MagnitudeSpectrum(nil) != nil {
		t.Error("an empty input produced a non-nil spectrum")
	}
}

func TestRMSEnvelopeMeasuresLevelsItWasGiven(t *testing.T) {
	// Two halves at known amplitudes: the envelope must read them, not something near them.
	x := make([]float64, 400)
	for i := range x {
		if i < 200 {
			x[i] = 3
		} else {
			x[i] = 9
		}
	}
	env := RMSEnvelope(x, 100, 100)
	if len(env) < 4 {
		t.Fatalf("%d windows over 400 samples at win=hop=100, want 4", len(env))
	}
	if math.Abs(env[0]-3) > 1e-9 || math.Abs(env[3]-9) > 1e-9 {
		t.Errorf("windows read %.3f and %.3f, want 3 and 9 — the RMS of a constant is "+
			"that constant", env[0], env[3])
	}
	if RMSEnvelope(x, 0, 10) != nil || RMSEnvelope(x, 10, 0) != nil {
		t.Error("a zero window or hop was accepted; it has no meaning and would divide by zero")
	}
	if RMSEnvelope(x, 500, 100) != nil {
		t.Error("a window longer than the signal was accepted")
	}
}
