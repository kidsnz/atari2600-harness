package audioingest

import (
	"fmt"
	"math"
)

// Reading a note from ONE bar of a record does not work, and the failure is quiet.
//
// Measured on "Bassline": the same sixteenth read B1 in one bar, C2 in the next and
// 63.9 Hz in a third, at confidences between 0.55 and 0.64 — high enough to look like
// answers and far enough apart to be three different notes. A bass note under a kick,
// a pad and a room is not a clean signal, and a single 120 ms window of it is not
// enough evidence to name a pitch.
//
// So this stacks. It takes the SAME sixteenth from many bars, computes each one's
// autocorrelation, and adds them up before looking for a peak. Autocorrelation because
// it is phase-invariant: the waveform of a note is not the same from bar to bar even
// when the pitch is, so averaging the samples themselves would cancel the note and
// leave the noise. What survives 48 bars of addition is what is actually there every
// bar; what does not is what a single-bar read was inventing.

// StackedNote is one sixteenth of the bar, measured over every occurrence of it.
type StackedNote struct {
	Step  int     // which sixteenth
	Hz    float64 // the pitch that survived stacking
	Conf  float64 // the accumulated peak's height, 0..1
	Bars  int     // occurrences that contributed
	Cents float64 // distance from the nearest 12-TET note, filled in by the caller
	Note  string
}

// StackNote reads sixteenth `step` from every bar in the file and returns the pitch that
// survives. loHz..hiHz is the range to search; skipSec delays the window past a
// transient (a kick on the same sixteenth will otherwise dominate the first 30 ms).
func StackNote(samples []float64, rate int, beatSec, phaseSec float64, step int,
	loHz, hiHz, skipSec, winSec float64) (StackedNote, error) {

	if beatSec <= 0 || loHz <= 0 || hiHz <= loHz {
		return StackedNote{}, fmt.Errorf("audioingest: beat %.4f s, range %.1f-%.1f Hz is not usable",
			beatSec, loHz, hiHz)
	}
	sixteenth := beatSec / 4
	barSec := beatSec * 4
	win := int(winSec * float64(rate))
	if win < 4*int(float64(rate)/loHz) {
		return StackedNote{}, fmt.Errorf("audioingest: a %.0f ms window holds fewer than four "+
			"cycles at %.1f Hz, which is not enough to autocorrelate", winSec*1000, loHz)
	}
	// Band-limit generously around the search range: the fundamental is what is being
	// looked for, and letting the whole spectrum in lets a hat decide a bass note.
	x := bandLimit(samples, loHz*0.6, hiHz*3, float64(rate))

	loLag := int(float64(rate) / hiHz)
	hiLag := int(float64(rate) / loHz)
	acc := make([]float64, hiLag+1)
	bars := 0
	for b := 0; ; b++ {
		t := phaseSec + float64(b)*barSec + float64(step)*sixteenth + skipSec
		i0 := int(t * float64(rate))
		if i0 < 0 || i0+win+hiLag > len(x) {
			break
		}
		w := x[i0 : i0+win+hiLag]
		// energy-normalise each bar, so a loud bar does not outvote the rest
		e := 0.0
		for i := 0; i < win; i++ {
			e += w[i] * w[i]
		}
		if e <= 0 {
			continue
		}
		for lag := loLag; lag <= hiLag; lag++ {
			s := 0.0
			for i := 0; i < win; i++ {
				s += w[i] * w[i+lag]
			}
			acc[lag] += s / e
		}
		bars++
	}
	if bars < 8 {
		return StackedNote{}, fmt.Errorf("audioingest: only %d bar(s) of material; stacking needs "+
			"enough occurrences to out-vote a single bad one", bars)
	}

	best := loLag
	for lag := loLag; lag <= hiLag; lag++ {
		if acc[lag] > acc[best] {
			best = lag
		}
	}
	// Parabolic interpolation on the peak, so the answer is not quantised to whole
	// samples -- at 60 Hz one sample of lag is already 3 cents.
	pos := float64(best)
	if best > loLag && best < hiLag {
		y0, y1, y2 := acc[best-1], acc[best], acc[best+1]
		if d := y0 - 2*y1 + y2; d != 0 {
			pos = float64(best) - 0.5*(y2-y0)/d
		}
	}
	n := StackedNote{Step: step, Hz: float64(rate) / pos, Bars: bars,
		Conf: acc[best] / float64(bars)}
	n.Note, n.Cents = nearest12TET(n.Hz)
	return n, nil
}

// nearest12TET names the closest equal-tempered note to a frequency and says how far
// away it is, in cents. A440.
func nearest12TET(hz float64) (string, float64) {
	if hz <= 0 {
		return "", 0
	}
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	semis := 12 * math.Log2(hz/440)
	k := int(math.Round(semis))
	exact := 440 * math.Pow(2, float64(k)/12)
	idx := ((k+9)%12 + 12) % 12
	oct := (k + 9 + 12*10) / 12 // A4 is octave 4; +9 puts C at the boundary
	return fmt.Sprintf("%s%d", names[idx], oct-6), 1200 * math.Log2(hz/exact)
}
