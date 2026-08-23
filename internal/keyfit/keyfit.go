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
	"sort"

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
	TonicHz   float64  `json:"tonic_hz"`
	TonicName string   `json:"tonic_name"`
	Worst     float64  `json:"worst_cents"`     // signed, the degree furthest out
	WorstDeg  int      `json:"worst_semitone"`  //
	Choices   []Choice `json:"choices"`         // best per degree, any waveform
	OneVoice  int      `json:"one_voice_audc"`  // the single waveform that fits best
	OneWorst  float64  `json:"one_voice_worst"` // its worst degree
	Detune    float64  `json:"detune_cents"`    // how far this tonic sits from the reference, in cents. SweepDetuned sets it; Sweep leaves it 0.

	// OneVoiceFundamental is how much of OneVoice's energy is in the FUNDAMENTAL
	// (audio.FundamentalStrength). READ IT WITH OneWorst, never instead of it. "In tune"
	// and "audible as a pitch" are different questions and this file answers only the
	// first: asked which single waveform plays the F# minor bass figure most accurately,
	// it returns AUDC 1, whose spectrum is .149 .146 .141 .133 -- flat enough to have no
	// fundamental at all. That answer is correct and useless, and this field is how a
	// caller sees why.
	OneVoiceFundamental float64 `json:"one_voice_fundamental"`
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
			f.OneVoiceFundamental = audio.FundamentalStrength(c)
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

// SweepDetuned is Sweep without the assumption that a piece has to start on a semitone.
//
// THE ASSUMPTION, and why it costs so much here. Sweep tries twelve tonics an octave,
// because that is where the notes of a keyboard are. The TIA is not a keyboard: its rungs
// sit wherever (AUDF+1)×D puts them, and in the bass they are 53 to 182 cents apart —
// measured on AUDC 6, 182.4 cents between AUDF 8 and 9, still 53.3 between 31 and 32.
// A ladder that coarse has no reason to line up with A440, and nothing about a cartridge
// obliges it to: a piece can sit at ANY reference pitch, and only the intervals inside it
// are audible as tuning. Anchoring to A440 throws away a whole free parameter.
//
// Measured on the F# minor bass figure this project reproduced (tonic, 4th, 5th, b6, b7,
// octave), searching a full octave of tonics:
//
//	AUDC 6 alone   best semitone tonic  43.5 cents worst  ->  best continuous  16.7
//	all waveforms  best semitone tonic  13.9 cents worst  ->  best continuous   9.2
//
// The single-waveform case is the one that matters, because a bass line usually wants one
// voice: 43.5 cents is a quarter tone and 16.7 is not.
//
// WHAT THIS IS NOT. It is not just intonation. Aiming at just ratios instead of 12-TET was
// tried first and is refuted for this machine, quantitatively: the largest 12-TET-to-just
// difference is 17.6 cents (the minor seventh), against a rung spacing of 53 to 182, so
// the target moves and the chosen AUDF does not. Measured over the same figure, every
// degree picks the identical register under both tunings.
//
// stepCents is the search resolution; spanCents the range each way. The returned Fits
// carry Detune, the offset from loHz in cents.
func SweepDetuned(loHz float64, spanCents, stepCents int, degrees []int, baseClock float64) []Fit {
	if stepCents < 1 {
		stepCents = 1
	}
	var out []Fit
	for c := -spanCents; c <= spanCents; c += stepCents {
		f := FitTonic(loHz*math.Pow(2, float64(c)/1200), degrees, baseClock)
		f.Detune = float64(c)
		out = append(out, f)
	}
	return out
}

// Best returns the fit with the smallest worst degree, judged on ONE waveform if oneVoice
// is true (OneWorst) and on any waveform otherwise (Worst). Returns the zero Fit for an
// empty list.
func Best(fits []Fit, oneVoice bool) Fit {
	best := Fit{}
	first := true
	for _, f := range fits {
		v, b := math.Abs(f.Worst), math.Abs(best.Worst)
		if oneVoice {
			v, b = math.Abs(f.OneWorst), math.Abs(best.OneWorst)
		}
		if first || v < b {
			best, first = f, false
		}
	}
	return best
}

// ---- one voice, and a tonic that need not sit on the semitone grid ---------------------
//
// WHY BOTH AT ONCE. Sweep and SweepDetuned rank tonics using the BEST waveform per degree, and
// -one-voice only REPORTS what a single waveform would cost — it cannot be told WHICH waveform
// to use. That is the wrong shape for the question an author actually asks, which on the job
// this comes from was "one type of sound only, please" after hearing a build that changed
// timbre mid-figure. And the best tonic for a single voice is generally NOT on the semitone
// grid: for AUDC 6 over the degree set {0,3,4,5,10,12,15} it is 38.41 Hz, which is D#1 minus 21
// cents, and no grid sweep reaches it. Both gaps had to be worked around by hand twice in one
// session, which is what this replaces.

// SweepVoices searches tonics continuously between loHz and hiHz at stepCents resolution,
// restricted to the given waveforms, and returns the fits sorted best-worst-degree first.
// waves nil means every playable one, which makes this SweepDetuned with a finer grid.
func SweepVoices(loHz, hiHz, stepCents float64, degrees []int, waves []int, baseClock float64) []Fit {
	if loHz <= 0 || hiHz <= loHz || len(degrees) == 0 {
		return nil
	}
	if stepCents <= 0 {
		stepCents = 5
	}
	if waves == nil {
		waves = Playable()
	}
	var out []Fit
	steps := int(1200 * math.Log2(hiHz/loHz) / stepCents)
	for i := 0; i <= steps; i++ {
		hz := loHz * math.Pow(2, float64(i)*stepCents/1200)
		f := FitTonicVoices(hz, degrees, waves, baseClock)
		if len(f.Choices) == len(degrees) {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return math.Abs(out[i].Worst) < math.Abs(out[j].Worst)
	})
	return out
}

// FitTonicVoices is FitTonic restricted to a waveform set.
func FitTonicVoices(tonicHz float64, degrees []int, waves []int, baseClock float64) Fit {
	if waves == nil {
		waves = Playable()
	}
	f := Fit{TonicHz: tonicHz, TonicName: Name(tonicHz)}
	for _, d := range degrees {
		target := tonicHz * math.Pow(2, float64(d)/12)
		c, af, cents := Nearest(target, waves, baseClock)
		if c < 0 {
			continue
		}
		f.Choices = append(f.Choices, Choice{
			Semitone: d, WantHz: target, AUDC: c, AUDF: af,
			GotHz: audio.Freq(c, af, baseClock), Cents: cents,
		})
		if math.Abs(cents) > math.Abs(f.Worst) {
			f.Worst, f.WorstDeg = cents, d
		}
		if len(waves) == 1 {
			f.OneVoice, f.OneWorst = waves[0], f.Worst
			f.OneVoiceFundamental = audio.FundamentalStrength(waves[0])
		}
	}
	return f
}
