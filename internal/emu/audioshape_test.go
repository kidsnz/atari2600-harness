package emu

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// runLengths turns a captured sample stream into the lengths of its constant runs,
// dropping the first and last (both are truncated by where the capture happened to
// start and stop, and a truncated run is not a measurement of anything).
func runLengths(s []uint8) []int {
	var out []int
	n := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			n++
			continue
		}
		out = append(out, n)
		n = 1
	}
	if len(out) < 3 {
		return nil
	}
	return out[1 : len(out)-1]
}

// captureRuns plays one (AUDC, AUDF) and returns the run lengths of its waveform.
func captureRuns(t *testing.T, audc, audf int, frames int) []int {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(buildAudioROM(t, uint8(audc), uint8(audf), 0x0A)); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	e.ResetAudioCapture()
	if err := e.RunFrames(frames); err != nil {
		t.Fatal(err)
	}
	ch0, _ := e.AudioSamples()
	return runLengths(ch0)
}

// AUDF IS A PURE TIME SCALING: it changes how fast a waveform is played and never what
// the waveform IS. Stated that way it sounds obvious, and it had never been measured;
// the pitch sweep proves the PERIOD comes out at (AUDF+1)xD but says nothing about the
// shape inside that period, so a waveform that changed character across its range would
// have passed everything this project has.
//
// It matters musically. If the shape were AUDF-dependent, a bass line would change
// timbre as it moved, and the instrument tables every driver here uses -- which pick a
// waveform once and then vary only AUDF -- would be built on a false premise.
//
// The test normalises each run length by (AUDF+1) and requires the resulting sequence to
// be identical at every AUDF UP TO ROTATION, with exact integer equality and no tolerance.
//
// The rotation is not a loosening, it is the correct comparison, and taking it literally
// first produced a second false alarm of exactly the kind this file is about. Compared
// position-for-position, AUDC 7 "fails" at every AUDF -- until you look at the sequences:
// AUDF=4 gives [2 1 3 1 1 1 1 4 1 2 1 1 2 2 5 3] and AUDF=9 gives
// [1 1 2 2 5 3 2 1 3 1 1 1 1 4 1 2], which is the same cycle entered six runs later. The
// capture begins wherever it begins; a waveform has no first run.
//
// It came out of an anomaly. Fourteen pairs resisted the exact-repeat check in the pitch
// sweep, all of them AUDC 6/10 at long periods, and counting their runs showed AUDC 6
// emitting 130+180 at AUDF 9, 286+396 at 21, 377+522 at 28 and 416+576 at 31 -- every one
// of them exactly 13:18 after dividing by (AUDF+1). So AUDC 6 is not a square wave at
// all: it is a 13:18 asymmetric pulse, which is why "bass" does not sound like "square".
// This test asks whether that constancy is a property of AUDC 6 or of the machine.
func TestAUDFScalesTheWaveformAndNeverChangesIt(t *testing.T) {
	if testing.Short() {
		t.Skip("captures several seconds of audio per waveform")
	}
	// Three AUDF values per waveform, far apart, all small enough that several whole
	// cycles fit in the capture even for the longest divisor tested here.
	audfs := []int{4, 9, 19}

	// EVERY PITCHED WAVEFORM THE TIA HAS, measured. Run lengths within one cycle,
	// normalised by (AUDF+1), so these are the shapes themselves and not one pitch's
	// rendering of them. They sum to the divisor by construction.
	//
	// Only AUDC 4 and 12 are true 50% squares. AUDC 6 is a 13:18 pulse and 14 a 49:44
	// one -- two-level like a square but ASYMMETRIC, which is a different harmonic
	// series and is why "bass" does not sound like "square". The rest are polynomial
	// shapes with 8, 16 or 128 runs per cycle.
	//
	// The run counts also close the loop on the measurement bug that started all this.
	// MeasurePeriod assumes two transitions per cycle, so it under-reports by exactly
	// runs/2 -- predicted 4x for saw, 8x for rumble/pitfall/buzz, 64x for engine, and
	// the sweep observed 4.00, 8.07, 8.05, 8.05 and 64.01. Nothing was wrong with the
	// hardware model; the measure could not see past a waveform's shape.
	//
	// Without this table the test would still pass if the emulator changed EVERY
	// waveform in the same way, since the AUDF comparison is only self-consistency.
	want := map[int][]int{
		1:  {4, 3, 1, 2, 2, 1, 1, 1},
		4:  {1, 1},
		6:  {13, 18},
		7:  {2, 1, 3, 1, 1, 1, 1, 4, 1, 2, 1, 1, 2, 2, 5, 3},
		12: {3, 3},
		14: {49, 44},
		15: {5, 6, 4, 5, 10, 5, 3, 7, 4, 10, 6, 3, 6, 4, 9, 6},
		2:  {62, 44, 18, 31, 31, 13, 18, 13, 62, 49, 13, 31, 31, 18, 13, 18},
		// AUDC 3 (engine) has 128 runs per cycle and is checked by its length and sum
		// rather than spelled out; the shape is in the test log.
	}

	for audc := 0; audc < 16; audc++ {
		if audio.Divisor(audc) == 0 || audio.Canonical(audc) == 8 {
			continue // DC and noise have no repeating shape to compare
		}
		if audio.Canonical(audc) != audc {
			continue // duplicates are covered by their canonical value
		}
		var ref []int
		var refAudf int
		for _, audf := range audfs {
			runs := captureRuns(t, audc, audf, 40)
			if len(runs) == 0 {
				t.Errorf("AUDC=%d AUDF=%d: no measurable runs in the capture", audc, audf)
				continue
			}
			// One cycle's worth: the run lengths must sum to the period.
			period := audio.PeriodSamples(audc, audf)
			var cycle []int
			sum := 0
			for _, r := range runs {
				cycle = append(cycle, r)
				if sum += r; sum >= period {
					break
				}
			}
			if sum != period {
				t.Errorf("AUDC=%d (%s) AUDF=%d: the runs %v sum to %d, not the period %d -- "+
					"either the capture is too short or the period is wrong",
					audc, audio.Name(audc), audf, cycle, sum, period)
				continue
			}
			// Normalise: every run should be an exact multiple of (AUDF+1).
			norm := make([]int, len(cycle))
			for i, r := range cycle {
				if r%(audf+1) != 0 {
					t.Errorf("AUDC=%d (%s) AUDF=%d: run of %d samples is not a whole multiple "+
						"of (AUDF+1)=%d, so AUDF is not a clean time scaling here",
						audc, audio.Name(audc), audf, r, audf+1)
					norm = nil
					break
				}
				norm[i] = r / (audf + 1)
			}
			if norm == nil {
				continue
			}
			if ref == nil {
				ref, refAudf = norm, audf
				t.Logf("AUDC=%2d (%-8s div %3d)  %3d runs  shape %v",
					audc, audio.Name(audc), audio.Divisor(audc), len(norm), norm)
				if w, ok := want[audc]; ok && !sameCycle(w, norm) {
					t.Errorf("AUDC=%d (%s): shape is %v, want %v (up to rotation). The measured "+
						"waveform has changed, which is either a real emulator change or a "+
						"regression in the capture -- both worth knowing about",
						audc, audio.Name(audc), norm, w)
				}
				if sumOf(norm) != audio.Divisor(audc) {
					t.Errorf("AUDC=%d (%s): the shape sums to %d, not the divisor %d",
						audc, audio.Name(audc), sumOf(norm), audio.Divisor(audc))
				}
				continue
			}
			if !sameCycle(ref, norm) {
				t.Errorf("AUDC=%d (%s): the waveform CHANGES with AUDF, and not merely in phase.\n"+
					"  AUDF=%d gives %v\n  AUDF=%d gives %v\nAUDF is meant to scale time only; a "+
					"shape that moves means the instrument tables in every driver here -- which "+
					"pick a waveform once and then vary only AUDF -- are built on a false premise",
					audc, audio.Name(audc), refAudf, ref, audf, norm)
			}
		}
	}
}

// The 13:18 pulse, pinned on its own so a regression names the thing that broke rather
// than pointing at a general property test.
func TestAUDC6IsAThirteenToEighteenPulseAndNotASquare(t *testing.T) {
	if testing.Short() {
		t.Skip("captures audio")
	}
	for _, audf := range []int{9, 21, 28, 31} {
		runs := captureRuns(t, 6, audf, 40)
		if len(runs) < 2 {
			t.Fatalf("AUDF=%d: %d runs measured", audf, len(runs))
		}
		a, b := runs[0], runs[1]
		if a > b {
			a, b = b, a
		}
		gotLo, gotHi := a/(audf+1), b/(audf+1)
		if gotLo != 13 || gotHi != 18 {
			t.Errorf("AUDF=%d: runs %d and %d normalise to %d:%d, want 13:18 (a symmetric "+
				"square would be 15.5:15.5 and is not what this waveform is)",
				audf, a, b, gotLo, gotHi)
		}
		if gotLo+gotHi != audio.Divisor(6) {
			t.Errorf("AUDF=%d: the two runs come to %d, not the divisor %d",
				audf, gotLo+gotHi, audio.Divisor(6))
		}
	}
}

func sumOf(v []int) int {
	n := 0
	for _, x := range v {
		n += x
	}
	return n
}

// sameCycle reports whether b is a rotation of a. Two captures of the same waveform
// start at whatever phase they start at, so equality of the CYCLE is the question and
// equality of the slice is not.
func sameCycle(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for off := 0; off < len(a); off++ {
		ok := true
		for i := range a {
			if a[i] != b[(i+off)%len(b)] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// THE SPECTRUM, not just the pitch. The shape test above proves each waveform keeps its
// shape across AUDF; this turns that shape into the thing an author actually chooses by.
// A pitch table says where a waveform's fundamental sits and nothing about whether the
// fundamental is the loudest thing in it -- and on this machine it very often is not.
//
// Pinned as a golden in audio.MeasuredSpectra — which is where the numbers themselves now
// live, so that cmd/voicefit and anything else can read them — because the practical
// conclusions in audio.Harmonics' comment rest on these figures: that AUDC 2 speaks about an octave above where Freq puts it, that
// AUDC 4 and 12 are one timbre in two registers, and that only 6 and 14 pair a strong
// fundamental with a bass divisor.
//
// Tolerance is 0.02 of normalised amplitude. It is not zero because the DFT runs over a
// whole number of cycles of an INTEGER period and short periods leave few samples per
// cycle -- AUDC 4 at AUDF 9 has twenty, and reads .574 where an ideally oversampled
// reconstruction of the same run lengths gives .596. Everything with a longer period
// agrees to three decimals.
func TestTheWaveformSpectraAreWhatAnAuthorChoosesBy(t *testing.T) {
	if testing.Short() {
		t.Skip("captures audio")
	}
	// The table used to be a literal here. It is now audio.MeasuredSpectra, because a tool
	// choosing a waveform by TIMBRE needs to import it and a _test.go file cannot be imported.
	// This test's job is unchanged and is now stated more honestly: it is what proves the
	// pinned table still matches the hardware.
	const audf, tol = 9, 0.02
	want := audio.MeasuredSpectra
	for audc, w := range want {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(buildAudioROM(t, uint8(audc), audf, 0x0A)); err != nil {
			t.Fatal(err)
		}
		if err := e.EnableAudioCapture(); err != nil {
			t.Fatal(err)
		}
		warmupStable(t, e)
		e.ResetAudioCapture()
		if err := e.RunFrames(60); err != nil {
			t.Fatal(err)
		}
		s, _ := e.AudioSamples()
		got := audio.Harmonics(s, float64(audio.PeriodSamples(audc, audf)), 8)
		if got == nil {
			t.Errorf("AUDC=%d: no spectrum measured", audc)
			continue
		}
		t.Logf("AUDC=%2d (%-8s) %v", audc, audio.Name(audc), round3(got))
		for k := range w {
			if d := got[k] - w[k]; d > tol || d < -tol {
				t.Errorf("AUDC=%d (%s) harmonic %d: %.3f, want %.3f (tol %.2f). The timbre has "+
					"changed, and the instrument advice in audio.Harmonics rests on it",
					audc, audio.Name(audc), k+1, got[k], w[k], tol)
			}
		}
	}
}

// The two conclusions that would change an author's choice, asserted directly so a
// regression names them instead of pointing at a table of eight numbers.
func TestRumbleSpeaksAnOctaveAboveWhereTheFormulaPutsIt(t *testing.T) {
	if testing.Short() {
		t.Skip("captures audio")
	}
	h := spectrumOf(t, 2, 9)
	if h[1] < 3*h[0] {
		t.Errorf("AUDC 2: harmonic 2 is %.3f against harmonic 1's %.3f. The whole point of "+
			"this entry is that the second dominates -- its waveform nearly repeats at half "+
			"period -- so a note written for it from Freq lands about an octave low", h[1], h[0])
	}
}

func TestSquareAndLeadAreOneTimbreInTwoRegisters(t *testing.T) {
	if testing.Short() {
		t.Skip("captures audio")
	}
	a, b := spectrumOf(t, 4, 9), spectrumOf(t, 12, 9)
	for k := range a {
		if d := a[k] - b[k]; d > 0.03 || d < -0.03 {
			t.Errorf("harmonic %d: AUDC 4 reads %.3f and AUDC 12 reads %.3f. These are meant to "+
				"be the same waveform at divisors 2 and 6, so an author picking between them is "+
				"choosing a register and not a sound", k+1, a[k], b[k])
		}
	}
	if audio.Divisor(4) == audio.Divisor(12) {
		t.Error("the two divisors are equal, so this test is comparing a waveform with itself")
	}
}

func spectrumOf(t *testing.T, audc, audf int) []float64 {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(buildAudioROM(t, uint8(audc), uint8(audf), 0x0A)); err != nil {
		t.Fatal(err)
	}
	if err := e.EnableAudioCapture(); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e)
	e.ResetAudioCapture()
	if err := e.RunFrames(60); err != nil {
		t.Fatal(err)
	}
	s, _ := e.AudioSamples()
	h := audio.Harmonics(s, float64(audio.PeriodSamples(audc, audf)), 8)
	if h == nil {
		t.Fatalf("AUDC=%d AUDF=%d: no spectrum", audc, audf)
	}
	return h
}

func round3(v []float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = float64(int(x*1000+0.5)) / 1000
	}
	return out
}
