package audiospec

import (
	"math"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const tiaRate = 31400.0 // NTSC TIA sample rate (~2 samples/scanline)

func sine(freq, rate float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Sin(2 * math.Pi * freq * float64(i) / rate)
	}
	return out
}

// TestInvertedTwins is the VV-13 core: two tones at the SAME amplitude (same RMS
// envelope) but DIFFERENT pitch are indistinguishable by loudness yet clearly
// separated by spectrum. The spectral axis must resolve what the envelope axis
// (and, like it, the golden_audio hash's coarse intent) cannot.
func TestInvertedTwins(t *testing.T) {
	const n = 8192
	a := sine(2000, tiaRate, n)
	b := sine(6000, tiaRate, n)

	env := EnvelopeDistance(a, b, 256, 128)
	spec := SpectralDistance(a, b)
	t.Logf("envelope distance=%.4f  spectral distance=%.4f", env, spec)

	if env > 0.02 {
		t.Errorf("equal-amplitude tones should have ~0 envelope distance, got %.4f", env)
	}
	if spec < 0.5 {
		t.Errorf("different pitches should be spectrally far apart, got %.4f", spec)
	}
	if spec <= env {
		t.Fatalf("spectral axis failed to out-resolve the envelope: spec %.4f <= env %.4f", spec, env)
	}
}

// TestIdentity: a signal compared with itself is zero on both axes.
func TestIdentity(t *testing.T) {
	a := sine(3000, tiaRate, 4096)
	if d := SpectralDistance(a, a); d > 1e-9 {
		t.Errorf("self spectral distance = %g, want 0", d)
	}
	if d := EnvelopeDistance(a, a, 256, 128); d > 1e-9 {
		t.Errorf("self envelope distance = %g, want 0", d)
	}
}

// TestDominantFreq: the FFT recovers a known tone's frequency within bin
// resolution.
// TestDominantFreq calibrates across LENGTH as well as frequency, and the length is the
// axis that matters. The bin width is sampleRate/nextPow2(len), which equals
// sampleRate/len for exactly one class of input: a power of two. This test used to pass
// only 8192 samples, so `float64(n)` -> `float64(len(samples))` was a mutant it could
// never see -- while cmd/audiospec, the only production caller, passes an emulator
// capture that is never a power of two (30,955 samples on a 60-frame run). Measured with
// that mutant in place: exact at 8192, and 249.1 Hz read as 263.7 Hz -- 100 cents -- at
// the length production actually uses.
func TestDominantFreq(t *testing.T) {
	cases := []struct {
		freq float64
		n    int
		why  string
	}{
		{4000, 8192, "power of two: nextPow2(len) == len, so a length bug is invisible here"},
		{4000, 30955, "the length cmd/audiospec passes for 60 frames"},
		{249, 30955, "low pitch at the production length: 65 bins from DC, where a scale error is largest"},
		{7000, 12000, "a third length, none of them a power of two"},
	}
	// The tolerance is in CENTS, not hertz. 50 Hz is a quarter of a bin at 4000 and 348
	// cents at 249, so an absolute window lets the low case through unexamined -- under
	// the mutant above it passed, while the two neighbouring cases failed.
	for _, c := range cases {
		got := DominantFreq(sine(c.freq, tiaRate, c.n), tiaRate)
		off := 1200 * math.Log2(got/c.freq)
		if math.Abs(off) > 25 {
			t.Errorf("dominant freq at n=%d = %.1f Hz, want ~%.0f (%.0f cents off) (%s)",
				c.n, got, c.freq, off, c.why)
		}
	}
}

// TestRealCapture exercises the full capture→spectrum pipeline on a real ROM:
// the stream is non-empty and self-comparison is zero.
func TestRealCapture(t *testing.T) {
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/techniques/sfx_demo.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(30); err != nil {
		t.Fatal(err)
	}
	ch0, _ := e.AudioSamples()
	if len(ch0) < 1024 {
		t.Fatalf("captured only %d audio samples, want >=1024", len(ch0))
	}
	sig := ToFloat(ch0)
	if d := SpectralDistance(sig, sig); d > 1e-9 {
		t.Errorf("self spectral distance on real capture = %g, want 0", d)
	}
}
