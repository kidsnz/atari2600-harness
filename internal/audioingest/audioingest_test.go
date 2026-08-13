package audioingest

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

// synth builds a WAV of a bassline: one note per step, a click on every 4th step.
// Every test below grades the package against a signal whose answer is known exactly,
// because the whole point of the tool is to be trusted on a signal whose answer is not.
func synth(rate int, bpm float64, notesHz []float64, clickEvery int) []byte {
	stepSec := 60.0 / bpm / 4
	n := int(float64(len(notesHz)) * stepSec * float64(rate))
	x := make([]float64, n)
	for s, hz := range notesHz {
		a := int(float64(s) * stepSec * float64(rate))
		b := int(float64(s+1) * stepSec * float64(rate))
		if b > n {
			b = n
		}
		if hz > 0 {
			for i := a; i < b; i++ {
				t := float64(i-a) / float64(rate)
				// fundamental plus a second harmonic: a pure sine is easier to track
				// than anything real, so the fixture is made slightly harder.
				x[i] += 0.6*math.Sin(2*math.Pi*hz*t) + 0.2*math.Sin(4*math.Pi*hz*t)
			}
		}
		if clickEvery > 0 && s%clickEvery == 0 {
			for i := a; i < a+rate/100 && i < n; i++ {
				x[i] += 0.9 * math.Exp(-float64(i-a)/float64(rate/300))
			}
		}
	}
	return wav(x, rate)
}

func wav(x []float64, rate int) []byte {
	b := []byte("RIFF")
	put32 := func(v uint32) { b = binary.LittleEndian.AppendUint32(b, v) }
	put16 := func(v uint16) { b = binary.LittleEndian.AppendUint16(b, v) }
	put32(uint32(36 + len(x)*2))
	b = append(b, "WAVEfmt "...)
	put32(16)
	put16(1)
	put16(1)
	put32(uint32(rate))
	put32(uint32(rate * 2))
	put16(2)
	put16(16)
	b = append(b, "data"...)
	put32(uint32(len(x) * 2))
	for _, v := range x {
		if v > 1 {
			v = 1
		}
		if v < -1 {
			v = -1
		}
		put16(uint16(int16(v * 32000)))
	}
	return b
}

func TestDecodeWAVReadsRateAndLength(t *testing.T) {
	b := synth(44100, 120, []float64{80, 80, 0, 0}, 4)
	s, rate, err := DecodeWAV(b)
	if err != nil {
		t.Fatalf("DecodeWAV: %v", err)
	}
	if rate != 44100 {
		t.Errorf("rate %d, want 44100", rate)
	}
	if want := int(4 * (60.0 / 120 / 4) * 44100); math.Abs(float64(len(s)-want)) > 4 {
		t.Errorf("%d samples, want about %d", len(s), want)
	}
}

func TestDecodeWAVRejectsWhatItCannotRead(t *testing.T) {
	if _, _, err := DecodeWAV([]byte("not a wav at all")); err == nil {
		t.Error("garbage was accepted as a WAV")
	}
	b := synth(44100, 120, []float64{80}, 0)
	b[20] = 3 // format 3 = IEEE float, which this decoder does not read
	if _, _, err := DecodeWAV(b); err == nil {
		t.Error("a non-PCM format was accepted; it would have been read as 16-bit ints")
	}
}

func TestEstimateTempoRecoversTheClickTempo(t *testing.T) {
	for _, bpm := range []float64{110, 124, 140} {
		notes := make([]float64, 64)
		for i := range notes {
			notes[i] = 90
		}
		s, rate, err := DecodeWAV(synth(44100, bpm, notes, 4))
		if err != nil {
			t.Fatal(err)
		}
		got, strength := EstimateTempo(OnsetEnvelope(s, rate), rate, 90, 170)
		if math.Abs(got-bpm) > 3 {
			t.Errorf("bpm %.1f, want %.0f (strength %.2f)", got, bpm, strength)
		}
		if strength < 0.2 {
			t.Errorf("bpm %.0f recovered but strength only %.2f", bpm, strength)
		}
	}
}

func TestEstimateTempoReportsWeakStrengthOnNoise(t *testing.T) {
	// The negative control. Noise has no pulse, so SOME lag still wins the
	// autocorrelation -- the tool must say the answer is weak rather than confident.
	rng := rand.New(rand.NewSource(1))
	x := make([]float64, 44100*6)
	for i := range x {
		x[i] = rng.NormFloat64() * 0.3
	}
	s, rate, err := DecodeWAV(wav(x, 44100))
	if err != nil {
		t.Fatal(err)
	}
	_, strength := EstimateTempo(OnsetEnvelope(s, rate), rate, 90, 170)
	if strength > 0.15 {
		t.Errorf("white noise reported tempo strength %.2f; it should be near zero", strength)
	}
}

func TestBassNotesRecoversThePitchesAndTheRests(t *testing.T) {
	// F2 87.31, G#2 103.83, C3 130.81, rest.
	want := []float64{87.31, 87.31, 0, 103.83, 130.81, 0, 87.31, 87.31}
	s, rate, err := DecodeWAV(synth(44100, 124, want, 0))
	if err != nil {
		t.Fatal(err)
	}
	stepSec := 60.0 / 124 / 4
	got := BassNotes(s, rate, 0, stepSec, len(want), []int{1}, 0.25)
	for i, w := range want {
		if w == 0 {
			if got[i].Hz != 0 {
				t.Errorf("step %d: reported %.1f Hz, want a rest", i, got[i].Hz)
			}
			continue
		}
		if got[i].Hz == 0 {
			t.Errorf("step %d: reported a rest, want %.1f Hz", i, w)
			continue
		}
		cents := 1200 * math.Log2(got[i].Hz/w)
		if math.Abs(cents) > 50 {
			t.Errorf("step %d: %.2f Hz is %.0f cents from %.2f", i, got[i].Hz, cents, w)
		}
		if got[i].Confidence < 0.3 {
			t.Errorf("step %d: confidence %.2f is too low to act on", i, got[i].Confidence)
		}
	}
}

func TestNearestTIAPicksTheMeasuredGridPoint(t *testing.T) {
	// The saw voice's F2 is AUDF 23 at 87.22 Hz, 1.7 cents flat of 87.31 (cmd/_tune).
	c, f, cents := NearestTIA(87.31, []int{1}, 3579545.0/114)
	if c != 1 || f != 23 {
		t.Errorf("got AUDC %d AUDF %d, want AUDC 1 AUDF 23", c, f)
	}
	if math.Abs(cents+1.7) > 1.0 {
		t.Errorf("cents %.1f, want about -1.7", cents)
	}
	// And the case that decides the key: F#2 on this voice is badly out of tune.
	_, _, e := NearestTIA(92.50, []int{1}, 3579545.0/114)
	if math.Abs(e) < 20 {
		t.Errorf("F#2 came out %.1f cents off; the measured grid says it is ~28 flat, "+
			"and the whole transposition decision rests on that", e)
	}
}

// TestBeatPhaseFollowsAnOffsetItWasNeverTold calibrates BeatPhase by MOVING the beat and
// checking the reading moves with it. The previous version measured one click track and
// asserted only that the answer was outside the band 0.08..0.42 -- which `return 0`
// satisfies, so a BeatPhase that never searched at all passed the whole package (verified
// by mutation). One state cannot separate an instrument from a constant; this is the
// second axis check_instruments.py now enforces.
//
// The known answer is constructed, not read back: silence of a known length is prepended
// to a click track, so the beat is displaced by exactly that much and the reading must
// follow it one-for-one.
//
// MEASURED AND PINNED, rather than assumed away: the reading sits a CONSTANT ~27 ms early.
// That is the group delay of the spectral-flux envelope it is handed -- OnsetEnvelope hops
// in 512-sample frames, and the flux peak does not land on the transient sample. The bias
// is a property worth having in writing; what must not drift is the SLOPE, which is 1.
func TestBeatPhaseFollowsAnOffsetItWasNeverTold(t *testing.T) {
	const beatSec = 0.5
	notes := make([]float64, 64)
	for i := range notes {
		notes[i] = 90
	}
	s, rate, err := DecodeWAV(synth(44100, 120, notes, 4))
	if err != nil {
		t.Fatal(err)
	}
	// The search steps by a quarter of a 512-sample hop, so no reading can be finer.
	step := 0.25 * 512 / float64(rate)

	// Wrap a phase difference into (-beatSec/2, +beatSec/2]: a phase is modular, and
	// "0.4731 against a prepend of 0" is -0.027, not +0.473.
	wrap := func(d float64) float64 {
		for d > beatSec/2 {
			d -= beatSec
		}
		for d <= -beatSec/2 {
			d += beatSec
		}
		return d
	}

	var biases []float64
	for _, offSec := range []float64{0, 0.05, 0.125, 0.25, 0.375} {
		pad := make([]float64, int(offSec*float64(rate)))
		got := BeatPhase(OnsetEnvelope(append(pad, s...), rate), rate, beatSec)
		bias := wrap(got - offSec)
		biases = append(biases, bias)
		// Slope 1: displacing the beat by offSec displaces the reading by offSec. This is
		// the clause `return 0` cannot satisfy -- at offSec 0.05 it reports a bias of
		// -0.05 while the others report -0.027.
		if math.Abs(bias) > 0.04 {
			t.Errorf("prepending %.3f s of silence moved the reading to %.4f s, a residual "+
				"of %+.4f s; the reading is supposed to follow the beat one-for-one",
				offSec, got, bias)
		}
	}
	// The bias is constant, not merely small: spread within two search steps. A reading
	// that wandered by a tenth of a beat while staying under the bound above would still
	// be useless for hanging a sixteenth grid on.
	lo, hi := biases[0], biases[0]
	for _, b := range biases {
		lo, hi = math.Min(lo, b), math.Max(hi, b)
	}
	if hi-lo > 2*step {
		t.Errorf("bias ranged %.4f..%.4f s (spread %.4f) across five offsets; more than "+
			"two search steps (%.4f s) means it is not a fixed group delay", lo, hi, hi-lo, 2*step)
	}
	if hi > 0 || lo < -0.04 {
		t.Errorf("bias %.4f..%.4f s is outside the measured envelope; BeatPhase reads ~27 ms "+
			"EARLY because the flux peak trails the transient, and a sign flip there would "+
			"put every note on the wrong side of its step", lo, hi)
	}
}
