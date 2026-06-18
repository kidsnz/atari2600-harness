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
func TestDominantFreq(t *testing.T) {
	got := DominantFreq(sine(4000, tiaRate, 8192), tiaRate)
	if math.Abs(got-4000) > 50 {
		t.Errorf("dominant freq = %.1f Hz, want ~4000", got)
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
