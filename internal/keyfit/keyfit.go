// Package keyfit answers the question this machine forces on anyone reproducing music:
// WHICH KEY can the TIA play this figure in, and on which waveform?
//
// The TIA's pitch is one clock divided by (AUDF+1) and again by the waveform's divisor,
// so its reachable notes are a fixed, uneven ladder — not a scale, and not the same
// ladder in every octave. A figure that is trivially in tune at one tonic can have three
// unusable degrees a semitone away. That is not a tuning preference, it is a property of
// the hardware, and it has to be MEASURED before a note is chosen rather than discovered
// afterwards by ear.
//
// This was hand-rolled three times while reproducing one record — for a bassline, for
// an arpeggio, and for a melody — and each time it changed the answer: the source key
// was unusable and the transposition it picked was not the obvious one. It belongs in
// the toolbox.
//
// What it does NOT do: choose. It reports the error at every tonic and leaves the
// musical decision (drop a degree, displace it an octave, accept the register change)
// to the person who can hear it.
package keyfit

import (
	"fmt"
	"math"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// Note names, sharps only — the TIA has no opinion about enharmonics.
var names = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// Name renders a frequency as the nearest 12-TET note plus its octave.
func Name(hz float64) string {
	if hz <= 0 {
		return "-"
	}
	v := 12 * math.Log2(hz/440)
	k := int(math.Round(v))
	return fmt.Sprintf("%s%d", names[((k+9)%12+12)%12], 4+int(math.Floor(float64(k+9)/12)))
}

// Choice is one degree placed on the hardware.
type Choice struct {
	Semitone int     `json:"semitone"` // offset above the tonic
	WantHz   float64 `json:"want_hz"`
	AUDC     int     `json:"audc"`
	AUDF     int     `json:"audf"`
	GotHz    float64 `json:"got_hz"`
	Cents    float64 `json:"cents"` // positive = the hardware plays SHARP
}

// Fit is one candidate tonic, scored.
type Fit struct {
	TonicHz    float64  `json:"tonic_hz"`
	TonicName  string   `json:"tonic_name"`
	Worst      float64  `json:"worst_cents"`       // signed, the degree furthest out
	WorstDeg   int      `json:"worst_semitone"`    //
	Choices    []Choice `json:"choices"`           // best per degree, any waveform
	OneVoice   int      `json:"one_voice_audc"`    // the single waveform that fits best
	OneWorst   float64  `json:"one_voice_worst"`   // its worst degree
}

// Playable is every waveform worth considering: divisor > 0, and not the noise voice,
// which has no pitch at all.
func Playable() []int {
	var out []int
	for c := 0; c <= 15; c++ {
		if c == 8 || audio.Divisor(c) == 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Nearest finds the (AUDC, AUDF) closest to hz in log space among the given waveforms.
// Returns cents signed: positive means the hardware is SHARP of the target.
func Nearest(hz float64, waves []int, baseClock float64) (audc, audf int, cents float64) {
	best := math.Inf(1)
	audc, audf = -1, -1
	for _, c := range waves {
		for f := 0; f <= 31; f++ {
			g := audio.Freq(c, f, baseClock)
			if g <= 0 {
				continue
			}
			e := 1200 * math.Log2(g/hz)
			if math.Abs(e) < math.Abs(best) {
				best, audc, audf = e, c, f
			}
		}
	}
	if math.IsInf(best, 1) {
		return -1, -1, 0
	}
	return audc, audf, best
}

// FitTonic places every degree at one tonic and scores the result.
func FitTonic(tonicHz float64, degrees []int, baseClock float64) Fit {
	waves := Playable()
	f := Fit{TonicHz: tonicHz, TonicName: Name(tonicHz), OneVoice: -1}
	for _, d := range degrees {
		hz := tonicHz * math.Pow(2, float64(d)/12)
		c, af, e := Nearest(hz, waves, baseClock)
		f.Choices = append(f.Choices, Choice{d, hz, c, af, audio.Freq(c, af, baseClock), e})
		if math.Abs(e) > math.Abs(f.Worst) {
			f.Worst, f.WorstDeg = e, d
		}
	}
	f.OneWorst = math.Inf(1)
	for _, c := range waves {
		w := 0.0
		for _, d := range degrees {
			_, _, e := Nearest(tonicHz*math.Pow(2, float64(d)/12), []int{c}, baseClock)
			if math.Abs(e) > math.Abs(w) {
				w = e
			}
		}
		if math.Abs(w) < math.Abs(f.OneWorst) {
			f.OneWorst, f.OneVoice = w, c
		}
	}
	return f
}

// Sweep scores every semitone tonic from loHz upward for `octaves` octaves.
// The result is in tonic order, not sorted — the caller usually wants to see the shape
// of the ladder, and sorting by score hides that neighbouring keys differ wildly.
func Sweep(loHz float64, octaves int, degrees []int, baseClock float64) []Fit {
	var out []Fit
	for st := 0; st < 12*octaves; st++ {
		out = append(out, FitTonic(loHz*math.Pow(2, float64(st)/12), degrees, baseClock))
	}
	return out
}

// InTune lists every (AUDC, AUDF) inside [loHz, hiHz] that lands within tol cents of
// 12-TET. This is the other direction: not "can I play this figure" but "what can this
// machine play at all", which is where a piece written FOR the hardware starts.
type Pitch struct {
	AUDC, AUDF int
	Hz, Cents  float64
	Note       string
}

func InTune(loHz, hiHz, tol, baseClock float64) []Pitch {
	var out []Pitch
	for _, c := range Playable() {
		for f := 0; f <= 31; f++ {
			hz := audio.Freq(c, f, baseClock)
			if hz < loHz || hz > hiHz {
				continue
			}
			v := 12 * math.Log2(hz/440)
			cents := (v - math.Round(v)) * 100
			if math.Abs(cents) <= tol {
				out = append(out, Pitch{c, f, hz, cents, Name(hz)})
			}
		}
	}
	return out
}
