package audioingest

import (
	"math"
	"math/rand"
	"testing"
)

// figure builds a bar-length bass figure: notes[step] = Hz, 0 = nothing. noise is added
// per bar so no two bars are the same waveform, which is the condition stacking has to
// survive and averaging the samples would not.
func figure(bars int, beat float64, notes map[int]float64, amp, noise float64, seed int64) []float64 {
	barSec := beat * 4
	sixteenth := beat / 4
	n := int(float64(bars)*barSec*crate) + crate
	x := make([]float64, n)
	rng := rand.New(rand.NewSource(seed))
	for i := range x {
		x[i] = rng.NormFloat64() * noise
	}
	for b := 0; b < bars; b++ {
		for step, hz := range notes {
			if hz <= 0 {
				continue
			}
			t0 := float64(b)*barSec + float64(step)*sixteenth
			at := int(t0 * crate)
			ph := rng.Float64() * 2 * math.Pi // a different phase every bar
			for i := 0; i < samps(0.20) && at+i < n; i++ {
				s := float64(i) / crate
				x[at+i] += amp * math.Exp(-s/0.10) *
					(math.Sin(2*math.Pi*hz*s+ph) + 0.4*math.Sin(4*math.Pi*hz*s+ph))
			}
		}
	}
	return x
}

// The witness. Five notes at known pitches, buried in noise as loud as they are, read
// back to within a few cents once 32 bars are stacked.
func TestStackingRecoversPitchesFromNoisyBars(t *testing.T) {
	beat := 0.5
	want := map[int]float64{4: 51.913, 6: 61.735, 7: 69.296, 9: 73.416, 11: 82.407}
	x := figure(32, beat, want, 1.0, 1.0, 5)
	for step, hz := range want {
		n, err := StackNote(x, crate, beat, 0, step, 45, 130, 0.005, 0.115)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if e := math.Abs(1200 * math.Log2(n.Hz/hz)); e > 25 {
			t.Errorf("step %d: stacked %.2f Hz over %d bars, want %.2f (%.0f cents out)",
				step, n.Hz, n.Bars, hz, e)
		}
	}
}

// The reason stacking exists, made explicit: ONE bar of the same material is not enough.
// If a single-bar read were as good, all of this would be ceremony.
func TestOneBarIsWorseThanThirtyTwo(t *testing.T) {
	beat := 0.5
	const step, hz = 6, 61.735
	x := figure(32, beat, map[int]float64{step: hz}, 1.0, 1.4, 11)

	stacked, err := StackNote(x, crate, beat, 0, step, 45, 130, 0.005, 0.115)
	if err != nil {
		t.Fatal(err)
	}
	stackedErr := math.Abs(1200 * math.Log2(stacked.Hz/hz))

	// the same window, one bar at a time, through the single-window estimator
	sixteenth := beat / 4
	barSec := beat * 4
	worst := 0.0
	for b := 0; b < 32; b++ {
		i0 := int((float64(b)*barSec + float64(step)*sixteenth + 0.005) * crate)
		got, _ := F0(x[i0:i0+samps(0.115)], crate, 45, 130)
		if got <= 0 {
			continue
		}
		if e := math.Abs(1200 * math.Log2(got/hz)); e > worst {
			worst = e
		}
	}
	t.Logf("stacked over 32 bars: %.1f cents out. Worst single bar: %.1f cents out.", stackedErr, worst)
	if worst <= stackedErr {
		t.Errorf("the worst single bar (%.1f c) is no worse than the stack (%.1f c); on the real "+
			"record single bars disagreed by a whole semitone, and this fixture has to reproduce "+
			"that or it is not testing the thing stacking was written for", worst, stackedErr)
	}
	if stackedErr > 25 {
		t.Errorf("the stack itself is %.1f cents out", stackedErr)
	}
}

// A sixteenth with NOTHING on it must not come back with a confident pitch. Absence is
// the answer that a five-note figure depends on being able to give.
func TestAnEmptySixteenthDoesNotProduceAConfidentNote(t *testing.T) {
	beat := 0.5
	x := figure(32, beat, map[int]float64{4: 51.913}, 1.0, 1.0, 7)
	full, err := StackNote(x, crate, beat, 0, 4, 45, 130, 0.005, 0.115)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := StackNote(x, crate, beat, 0, 13, 45, 130, 0.005, 0.115)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("a sixteenth with a note: conf %.3f | one with only noise: conf %.3f", full.Conf, empty.Conf)
	if empty.Conf >= full.Conf {
		t.Errorf("noise scores %.3f against a real note's %.3f; the confidence cannot separate "+
			"a note from nothing, so every step would read as a note", empty.Conf, full.Conf)
	}
}

func TestStackRefusesWhatItCannotJudge(t *testing.T) {
	x := figure(4, 0.5, map[int]float64{0: 60}, 1, 0.1, 3)
	if _, err := StackNote(x, crate, 0.5, 0, 0, 45, 130, 0, 0.115); err == nil {
		t.Error("four bars were accepted; stacking needs enough occurrences to out-vote a bad one")
	}
	big := figure(32, 0.5, map[int]float64{0: 60}, 1, 0.1, 3)
	if _, err := StackNote(big, crate, 0.5, 0, 0, 45, 130, 0, 0.02); err == nil {
		t.Error("a 20 ms window was accepted for a 30 Hz search: it holds well under four cycles")
	}
	if _, err := StackNote(big, crate, 0, 0, 0, 45, 130, 0, 0.115); err == nil {
		t.Error("a zero beat length was accepted")
	}
}

func TestNoteNamingIsRight(t *testing.T) {
	for _, c := range []struct {
		hz   float64
		name string
	}{{440, "A4"}, {46.249, "F#1"}, {61.735, "B1"}, {73.416, "D2"}, {82.407, "E2"}, {92.499, "F#2"}} {
		got, cents := nearest12TET(c.hz)
		if got != c.name || math.Abs(cents) > 1 {
			t.Errorf("nearest12TET(%.3f) = %s %+.1f c, want %s within a cent", c.hz, got, cents, c.name)
		}
	}
}
