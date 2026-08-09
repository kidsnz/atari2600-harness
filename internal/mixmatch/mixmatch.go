// Package mixmatch compares the SPECTRAL BALANCE of a reference recording against a
// ROM's own mixer output, band by band, and says by how many dB each part is out.
//
// Why this exists. "The melody is too heavy" is a real observation and an unactionable
// one: a TIA channel has a 4-bit volume, no EQ and no dynamics, so the only lever is
// which integer goes into which AUDV. Turning that observation into a number — "the
// 200-1200 Hz band is 4.9 dB louder than the record's" — makes it a search with an
// answer, and the answer turned out to be one step on each of three knobs.
//
// The measurement is a RATIO, not a level. Absolute loudness is meaningless here: the
// record is mastered and the ROM's output is a 4-bit sum, so they are compared against
// one of their own bands and only the balance between bands is judged.
//
// What it cannot fix. A band that is out because of the WAVEFORM rather than the mix
// stays out. AUDC 1 is a harmonically rich saw and its harmonics band ran 3.2 dB hot
// against the record no matter what the volumes were; the tool reports that and it is
// a property of the machine, not an error to tune away.
package mixmatch

import (
	"fmt"
	"math"
	"sort"
)

// Band is one frequency range to judge, named for the part that lives in it.
type Band struct {
	Name   string
	Lo, Hi float64
}

// Default is the split that separates a kick-and-bass record's parts on this machine:
// the sub where the kick and a low pedal sit, the register a bassline's fundamentals
// occupy, the harmonics above them, and the top where hats live.
func Default() []Band {
	return []Band{
		{"low 30-60", 30, 60},
		{"line 60-200", 60, 200},
		{"harm 200-1200", 200, 1200},
		{"top 3k-14k", 3000, 14000},
	}
}

// Profile is the band balance of one signal, in dB relative to Ref.
type Profile struct {
	Ref  string             // the band everything is measured against
	DB   map[string]float64 // band name -> dB relative to Ref (Ref itself is 0)
	RMS  map[string]float64 // raw RMS, kept so a caller can see absolute levels
	Bands []Band
}

// Measure band-limits the signal with an FFT and reports each band's RMS, then
// normalises to the band named ref. A zero-energy reference band is an error rather
// than a silent divide: it means the split does not fit the material.
func Measure(x []float64, rate float64, bands []Band, ref string) (*Profile, error) {
	if len(x) < 1024 {
		return nil, fmt.Errorf("mixmatch: %d samples is too short to band-limit", len(x))
	}
	n := 1
	for n < len(x) {
		n <<= 1
	}
	// Hann window before the transform. Without it a pure tone leaks across every bin
	// -- measured: a 45 Hz sine left -19.5 dB in the 3-14 kHz band, which would have
	// been read as "the record has hats" when it has silence there.
	re := make([]float64, n)
	im := make([]float64, n)
	for i, v := range x {
		re[i] = v * (0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(len(x)-1)))
	}
	fft(re, im, false)

	p := &Profile{Ref: ref, DB: map[string]float64{}, RMS: map[string]float64{}, Bands: bands}
	for _, b := range bands {
		lo := int(b.Lo * float64(n) / rate)
		hi := int(b.Hi * float64(n) / rate)
		fr := make([]float64, n)
		fi := make([]float64, n)
		for k := lo; k <= hi && k < n/2; k++ {
			fr[k], fi[k] = re[k], im[k]
			fr[n-k], fi[n-k] = re[n-k], im[n-k]
		}
		fft(fr, fi, true)
		s := 0.0
		for i := 0; i < len(x); i++ {
			s += fr[i] * fr[i]
		}
		p.RMS[b.Name] = math.Sqrt(s / float64(len(x)))
	}
	r, ok := p.RMS[ref]
	if !ok {
		return nil, fmt.Errorf("mixmatch: no band named %q", ref)
	}
	if r <= 0 {
		return nil, fmt.Errorf("mixmatch: the reference band %q has no energy; the band split does not fit this material", ref)
	}
	for k, v := range p.RMS {
		p.DB[k] = 20 * math.Log10(v/r+1e-15)
	}
	return p, nil
}

// Error is how far a candidate is from the reference, per band.
type Error struct {
	Band string
	DB   float64 // positive = the candidate is LOUDER than the reference here
}

// Compare returns the per-band error, worst first by absolute value.
func Compare(ref, got *Profile) []Error {
	var out []Error
	for _, b := range ref.Bands {
		out = append(out, Error{b.Name, got.DB[b.Name] - ref.DB[b.Name]})
	}
	sort.Slice(out, func(i, j int) bool { return math.Abs(out[i].DB) > math.Abs(out[j].DB) })
	return out
}

// Score collapses the per-band error into one number a search can minimise. Weights let
// a caller discount a band it cannot control -- the harmonics of a fixed waveform, for
// instance -- rather than chase it and spoil the bands it can.
func Score(errs []Error, weight map[string]float64) float64 {
	s := 0.0
	for _, e := range errs {
		w := 1.0
		if x, ok := weight[e.Band]; ok {
			w = x
		}
		s += w * math.Abs(e.DB)
	}
	return s
}

// ---- a small in-place radix-2 FFT, so this package needs nothing outside the repo ----

func fft(re, im []float64, inverse bool) {
	n := len(re)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		ang := 2 * math.Pi / float64(l)
		if !inverse {
			ang = -ang
		}
		wr, wi := math.Cos(ang), math.Sin(ang)
		for i := 0; i < n; i += l {
			cr, ci := 1.0, 0.0
			for k := 0; k < l/2; k++ {
				ur, ui := re[i+k], im[i+k]
				vr := re[i+k+l/2]*cr - im[i+k+l/2]*ci
				vi := re[i+k+l/2]*ci + im[i+k+l/2]*cr
				re[i+k], im[i+k] = ur+vr, ui+vi
				re[i+k+l/2], im[i+k+l/2] = ur-vr, ui-vi
				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
	if inverse {
		for i := range re {
			re[i] /= float64(n)
			im[i] /= float64(n)
		}
	}
}
