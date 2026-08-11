// Package audio holds universal knowledge of Atari 2600 TIA sound (public, reusable).
// Source: Paul Slocum "Atari 2600 Music And Sound Programming Guide" v1.02 (authoritative, held locally),
// Eckhard Stolberg "frequency and waveform guide", Stella Programmer's Guide.
// Verification: numbers cross-checked with the harness audio capture (raw samples -> period measurement, internal/emu) (V2-14/15).
package audio

import (
	"fmt"
	"math"
	"strings"
)

// BaseClockNTSC is the TIA audio clock (Hz). Color clock/114 = 2 samples/scanline.
const BaseClockNTSC = 3579545.0 / 114.0 // ≈ 31399.5
// BaseClockPAL is the PAL audio clock (≈13 cents lower).
const BaseClockPAL = 3546894.0 / 114.0 // ≈ 31113.1

// Name is the common name of an AUDC value (Slocum's naming). Duplicates return the canonical value's name.
func Name(audc int) string {
	switch Canonical(audc) {
	case 0:
		return "silent"
	case 1:
		return "saw"
	case 2:
		return "rumble"
	case 3:
		return "engine"
	case 4:
		return "square"
	case 6:
		return "bass"
	case 7:
		return "pitfall"
	case 8:
		return "noise"
	case 12:
		return "lead"
	case 14:
		return "low bass"
	case 15:
		return "buzz"
	}
	return "unknown"
}

// Canonical folds duplicate AUDC values ({0,11} {4,5} {6,10} {7,9} {12,13}) into the canonical value.
// Note (V2-14 measured, Gopher2600): {0,11} {4,5} {12,13} are complete duplicates [sample-identical].
// {6,10} and {7,9} are [same tuning and period but logically inverted waveforms] (hi/lo complementary) = identical to the ear, different sample sequences.
// The documents' (Stolberg/Slocum) "duplicates" are correct in the tuning sense, but at the sample level they split into two kinds.
func Canonical(audc int) int {
	switch audc & 0x0F {
	case 11:
		return 0
	case 5:
		return 4
	case 10:
		return 6
	case 9:
		return 7
	case 13:
		return 12
	}
	return audc & 0x0F
}

// Divisor is the repeat length D of the AUDC waveform (in audio-clock ticks). Frequency = base/(AUDF+1)/D.
// 0/11 is DC (silence) = 0. 8 is noise (511-bit poly, no sense of pitch).
func Divisor(audc int) int {
	switch Canonical(audc) {
	case 1:
		return 15
	case 2, 3:
		return 465
	case 4:
		return 2
	case 6, 7:
		return 31
	case 8:
		return 511
	case 12:
		return 6
	case 14, 15:
		return 93
	}
	return 0
}

// Freq is the fundamental frequency (Hz) of (AUDC, AUDF). Pitchless modes (DC/noise) return 0.
func Freq(audc, audf int, baseClock float64) float64 {
	d := Divisor(audc)
	if d == 0 || Canonical(audc) == 8 {
		return 0
	}
	return baseClock / float64(audf+1) / float64(d)
}

// PeriodSamples is the theoretical period of (AUDC, AUDF) (in samples = audio-clock ticks).
func PeriodSamples(audc, audf int) int {
	d := Divisor(audc)
	if d == 0 {
		return 0
	}
	return (audf + 1) * d
}

// MeasurePeriod measures the dominant repeat period (in samples) from a raw sample sequence
// (for square-wave-like signals: mean interval between value transitions x 2). Fewer than 3
// transitions returns 0 (silence/DC).
//
// ⚠️ SQUARE-LIKE ONLY, and the failure is silent. "Mean interval between transitions x 2"
// is the period only when there are exactly two transitions per cycle, which is true of
// AUDC 4 and 12 and of nothing else the TIA has. The polynomial waveforms switch many
// times inside one cycle, so this returns a FRACTION of the period and it looks like a
// perfectly ordinary number: measured against the formula it is 8x low for saw (AUDC 1),
// pitfall (7/9) and buzz (15), and 64x low for engine (3). Use MeasureFundamental when
// the waveform is not known to be square-like -- and note that the four spot checks that
// stood as this table's verification for a year happened to be three squares and one
// AUDC 6, the one poly waveform whose transition count coincides with its period.
func MeasurePeriod(samples []uint8) float64 {
	if len(samples) < 4 {
		return 0
	}
	var transitions []int
	for i := 1; i < len(samples); i++ {
		if samples[i] != samples[i-1] {
			transitions = append(transitions, i)
		}
	}
	if len(transitions) < 3 {
		return 0
	}
	// mean transition interval = half period
	first, last := transitions[0], transitions[len(transitions)-1]
	half := float64(last-first) / float64(len(transitions)-1)
	return half * 2
}

// MeasureFundamental measures the shortest strong repeat period (in samples) of a raw
// sample sequence, by autocorrelation over lo..hi. Returns the period and its normalised
// correlation (1.0 = the signal repeats exactly at that lag). 0 lags or too few samples
// return (0, 0).
//
// This is the measurement MeasurePeriod cannot make. Counting transitions asks "how often
// does the signal change", which is a property of the waveform's shape; autocorrelation
// asks "after how many samples does the signal look like itself again", which is the
// period regardless of how convoluted one cycle is. On the TIA's nine pitched waveforms
// it reproduces (AUDF+1)xD exactly, at r >= 0.97.
func MeasureFundamental(samples []uint8, lo, hi int) (period int, corr float64) {
	n := len(samples)
	if n < 8 || lo < 1 {
		return 0, 0
	}
	if hi > n/3 {
		hi = n / 3
	}
	if hi < lo {
		return 0, 0
	}
	mean := 0.0
	for _, v := range samples {
		mean += float64(v)
	}
	mean /= float64(n)
	x := make([]float64, n)
	var energy float64
	for i, v := range samples {
		x[i] = float64(v) - mean
		energy += x[i] * x[i]
	}
	if energy == 0 {
		return 0, 0 // DC: no period to find
	}
	best, bestR := 0, -2.0
	for lag := lo; lag <= hi; lag++ {
		var num float64
		for i := 0; i+lag < n; i++ {
			num += x[i] * x[i+lag]
		}
		if r := num / energy; r > bestR {
			best, bestR = lag, r
		}
	}
	return best, bestR
}

// IsPeriodic checks that samples repeat exactly with the given period (s[i]==s[i+period]) for at least minPeriods periods.
// Use this to verify the period of poly waveforms (saw/pitfall/engine etc. = too many transitions for MeasurePeriod).
func IsPeriodic(samples []uint8, period, minPeriods int) bool {
	if period <= 0 || len(samples) < period*(minPeriods+1) {
		return false
	}
	n := period * minPeriods
	for i := 0; i < n; i++ {
		if samples[i] != samples[i+period] {
			return false
		}
	}
	return true
}

// NoteByte is a Sequencer Kit / slocum-tracker compatible note byte (upper 3 bits = sound-type idx, lower 5 bits = AUDF).
// idx is an index into soundTypeArray (default: 4,6,7,8,15,12,1,3). $FF is a rest.
// Format-specific ambiguity: (idx=7, AUDF=31) collides with $FF = rest and is therefore unusable (a property of the format).
func NoteByte(soundTypeIdx, audf int) uint8 {
	return uint8((soundTypeIdx&0x07)<<5 | (audf & 0x1F))
}

// DecodeNoteByte converts a note byte back to (sound-type idx, AUDF). $FF is (-1, -1) (rest).
func DecodeNoteByte(b uint8) (soundTypeIdx, audf int) {
	if b == 0xFF {
		return -1, -1
	}
	return int(b >> 5), int(b & 0x1F)
}

// --- Note name -> TIA (R7: composition-session support) ---

var noteOffsets = map[string]int{
	"C": -9, "C#": -8, "DB": -8, "D": -7, "D#": -6, "EB": -6, "E": -5,
	"F": -4, "F#": -3, "GB": -3, "G": -2, "G#": -1, "AB": -1, "A": 0,
	"A#": 1, "BB": 1, "B": 2,
}

// NoteFreq converts a 12-tone equal-temperament note name ("C4", "F#3", "Bb2" etc., A4=440Hz) to a frequency.
func NoteFreq(name string) (float64, error) {
	n := strings.ToUpper(strings.TrimSpace(name))
	if len(n) < 2 {
		return 0, fmt.Errorf("bad note %q", name)
	}
	octIdx := len(n) - 1
	pitch := n[:octIdx]
	oct := int(n[octIdx] - '0')
	if oct < 0 || oct > 9 {
		return 0, fmt.Errorf("bad octave in %q", name)
	}
	off, ok := noteOffsets[pitch]
	if !ok {
		return 0, fmt.Errorf("bad pitch %q", name)
	}
	semis := float64((oct-4)*12 + off)
	return 440.0 * math.Pow(2, semis/12.0), nil
}

// FindNote returns the (AUDC, AUDF) closest to a note name (with the error in cents).
// If types is empty, it searches the practical sound types {4(square), 6(bass), 7(buzz), 12(lead)}.
func FindNote(name string, types []int, baseClock float64) (audc, audf int, cents float64, err error) {
	target, err := NoteFreq(name)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(types) == 0 {
		types = []int{4, 6, 7, 12}
	}
	best := math.Inf(1)
	for _, c := range types {
		for f := 0; f < 32; f++ {
			got := Freq(c, f, baseClock)
			if got <= 0 {
				continue
			}
			ct := 1200 * math.Log2(got/target)
			if math.Abs(ct) < math.Abs(best) {
				best, audc, audf = ct, c, f
			}
		}
	}
	return audc, audf, best, nil
}

// NearestNote returns the 12-tone equal-temperament note name closest to a frequency
// ("D6" etc., A4=440Hz) and the error in cents (the inverse of FindNote = for transcription).
// freq<=0 is treated as silence and returns the empty string.
func NearestNote(freq float64) (name string, cents float64) {
	if freq <= 0 {
		return "", 0
	}
	semis := 12 * math.Log2(freq/440.0) // semitones from A4
	n := int(math.Round(semis))
	idx := n + 57 // semitone index from C0 (A4 = 57)
	if idx < 0 {
		idx = 0
		n = -57
	}
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	exact := 440.0 * math.Pow(2, float64(n)/12.0)
	return fmt.Sprintf("%s%d", names[idx%12], idx/12), 1200 * math.Log2(freq/exact)
}

// --- SFX helpers (U-M2: generate sound effects as frame sequences, numerically verifiable) ---

// SFXFrame is the audio-register specification for one frame.
type SFXFrame struct {
	C uint8 // AUDC (sound type)
	F uint8 // AUDF (frequency divider)
	V uint8 // AUDV (volume)
}

// PitchSweep is a sweep that linearly interpolates AUDF from fStart to fEnd (larger fEnd = descending pitch).
func PitchSweep(audc, fStart, fEnd, vol, frames int) []SFXFrame {
	out := make([]SFXFrame, frames)
	for i := range out {
		f := fStart
		if frames > 1 {
			f = fStart + (fEnd-fStart)*i/(frames-1)
		}
		out[i] = SFXFrame{uint8(audc), uint8(f), uint8(vol)}
	}
	return out
}

// NoiseBurst is a decaying burst of noise (default AUDC=8). Linear decay from vol to 0.
func NoiseBurst(audc, audf, vol, frames int) []SFXFrame {
	out := make([]SFXFrame, frames)
	for i := range out {
		v := vol * (frames - i) / frames
		out[i] = SFXFrame{uint8(audc), uint8(audf), uint8(v)}
	}
	return out
}

// Blip is a single short tone.
func Blip(audc, audf, vol, frames int) []SFXFrame {
	out := make([]SFXFrame, frames)
	for i := range out {
		out[i] = SFXFrame{uint8(audc), uint8(audf), uint8(vol)}
	}
	return out
}

// Arpeggio plays a sequence of AUDF values in order, framesPer frames each (pickup sounds etc.).
func Arpeggio(audc int, audfs []int, framesPer, vol int) []SFXFrame {
	var out []SFXFrame
	for _, f := range audfs {
		out = append(out, Blip(audc, f, vol, framesPer)...)
	}
	return out
}

// EmitSFX converts an SFX frame sequence into a dasm .byte table (2 bytes/frame: AUDC<<4|AUDV, AUDF).
func EmitSFX(label string, frames []SFXFrame) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: ; %d frames (2 bytes each: AUDC<<4|AUDV, AUDF)\n", label, len(frames))
	for _, fr := range frames {
		fmt.Fprintf(&b, "        byte $%02X,$%02X\n", fr.C<<4|fr.V&0x0F, fr.F)
	}
	return b.String()
}
