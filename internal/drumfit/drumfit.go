// Package drumfit measures a drum hit in a reference recording and fits TIA envelope
// tables to it, so a kick on this machine can be the RECORD's kick rather than the
// generic 2600 one every demo uses.
//
// The 2600's drum is a pitch sweep with a volume decay: one AUDC, and a pair of tables
// stepped once a frame. Those tables are normally written by ear. They do not have to
// be. A kick in a record is a short event with a measurable amplitude envelope and a
// measurable falling fundamental, both sampled at 60 Hz because that is the only rate
// the machine can change them at, and both quantisable to the 4 and 5 bits the TIA has.
//
// The fit is deliberately shallow. It matches the ENVELOPE -- how loud and how low,
// frame by frame -- and nothing else. A TIA square at divisor 31 is not a sampled kick
// and no table will make it one; what a table can do is give it the record's shape,
// which is what makes two kicks sound like the same instrument at different fidelity
// rather than two different instruments.
//
// Averaging over many hits is not an optimisation. A single kick in a mix carries
// whatever else was playing under it; the same kick averaged over thirty beats keeps
// what repeats and cancels what does not.
package drumfit

import (
	"fmt"
	"math"

	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// Hit is one measured drum, frame by frame from its onset.
type Hit struct {
	Frames int
	Amp    []float64 // linear amplitude, normalised so the peak is 1
	Hz     []float64 // dominant frequency in the band, 0 where it could not be found
	Conf   []float64 // 0..1 confidence in Hz. READ THIS. At 30-120 Hz one frame holds
	                 // between half a cycle and two, which is not enough to fix a pitch:
	                 // measured on a real kick, the per-frame track jumped 35 -> 18 ->
	                 // 60 -> 13 Hz with errors past 1400 cents. The amplitude envelope
	                 // from the same measurement was clean. Pitch needs PitchWin > 1.
}

// Fitted is the same hit expressed as TIA tables.
type Fitted struct {
	AUDC int
	EnvV []int     // AUDV per frame, 0..15
	EnvF []int     // AUDF per frame, 0..31
	Hz   []float64 // what those AUDF values actually produce
	Cents []float64 // how far each is from the measured pitch
}

// Measure averages the envelope of a drum across many onsets.
//
// onsets are in seconds; band is the frequency range the drum occupies (30..120 Hz for
// a kick, 3k..14k for a hat). frames is how many 1/60 s steps to follow it for.
func Measure(x []float64, rate float64, onsets []float64, band [2]float64, frames int) (*Hit, error) {
	return MeasureWin(x, rate, onsets, band, frames, 1)
}

// MeasureWin is Measure with an explicit pitch window: pitch is estimated over pitchWin
// frames centred on each point, while amplitude stays per-frame. They need different
// windows because they change at different rates -- a kick's level moves every frame and
// its pitch does not -- and using one window for both makes the pitch unusable.
func MeasureWin(x []float64, rate float64, onsets []float64, band [2]float64, frames, pitchWin int) (*Hit, error) {
	if len(onsets) < 3 {
		return nil, fmt.Errorf("drumfit: %d onsets is not enough to average; a single hit carries the rest of the mix with it", len(onsets))
	}
	if frames < 2 {
		return nil, fmt.Errorf("drumfit: %d frames is not an envelope", frames)
	}
	bp := bandpass(x, rate, band[0], band[1])
	step := rate / 60.0
	if pitchWin < 1 {
		pitchWin = 1
	}
	h := &Hit{Frames: frames, Amp: make([]float64, frames), Hz: make([]float64, frames),
		Conf: make([]float64, frames)}
	counted := make([]int, frames)
	for _, t := range onsets {
		for f := 0; f < frames; f++ {
			a := int(t*rate) + int(float64(f)*step)
			b := a + int(step)
			if a < 0 || b > len(bp) {
				continue
			}
			seg := bp[a:b]
			s := 0.0
			for _, v := range seg {
				s += v * v
			}
			h.Amp[f] += math.Sqrt(s / float64(len(seg)))
			pa := a - int(float64(pitchWin-1)/2*step)
			pb := pa + pitchWin*int(step)
			if pa >= 0 && pb <= len(bp) {
				if hz, cf := dominantConf(bp[pa:pb], rate, band[0], band[1]); hz > 0 {
					h.Hz[f] += hz
					h.Conf[f] += cf
				}
			}
			counted[f]++
		}
	}
	peak := 0.0
	for f := 0; f < frames; f++ {
		if counted[f] == 0 {
			continue
		}
		h.Amp[f] /= float64(counted[f])
		h.Hz[f] /= float64(counted[f])
		h.Conf[f] /= float64(counted[f])
		if h.Amp[f] > peak {
			peak = h.Amp[f]
		}
	}
	// "No energy" has to be judged RELATIVE to the signal, not against exact zero: a
	// band-pass of a pure low tone leaves a numerically tiny but non-zero residue up in
	// the hat band, and comparing against zero would accept it and report an envelope
	// made of rounding error.
	full := 0.0
	for _, v := range x {
		full += v * v
	}
	full = math.Sqrt(full / float64(len(x)))
	if peak <= full*1e-3 {
		return nil, fmt.Errorf("drumfit: %.0f-%.0f Hz holds %.2e against a broadband %.2e -- "+
			"there is no drum in that band", band[0], band[1], peak, full)
	}
	for f := range h.Amp {
		h.Amp[f] /= peak
	}
	return h, nil
}

// Fit quantises a measured hit onto the TIA: amplitude to a 0..15 volume and the
// dominant frequency to the nearest AUDF on the given waveform.
//
// The volume is quantised on a LINEAR amplitude scale, not in dB. The TIA's AUDV is a
// linear attenuator, so matching the shape means matching amplitude; converting to dB
// first and rounding there produces a curve that decays visibly too slowly.
func Fit(h *Hit, audc int, peakVol int, baseClock float64) *Fitted {
	f := &Fitted{AUDC: audc}
	for i := 0; i < h.Frames; i++ {
		v := int(math.Round(h.Amp[i] * float64(peakVol)))
		if v < 0 {
			v = 0
		}
		if v > 15 {
			v = 15
		}
		f.EnvV = append(f.EnvV, v)

		best, bestF := math.Inf(1), 0
		if h.Hz[i] > 0 {
			for a := 0; a <= 31; a++ {
				g := audio.Freq(audc, a, baseClock)
				if g <= 0 {
					continue
				}
				if e := math.Abs(1200 * math.Log2(g/h.Hz[i])); e < best {
					best, bestF = e, a
				}
			}
		}
		if math.IsInf(best, 1) {
			// no pitch measured this frame: hold the previous AUDF rather than jump to 0,
			// which would be the highest note the divisor can make
			if len(f.EnvF) > 0 {
				bestF = f.EnvF[len(f.EnvF)-1]
			}
			best = 0
		}
		f.EnvF = append(f.EnvF, bestF)
		got := audio.Freq(audc, bestF, baseClock)
		f.Hz = append(f.Hz, got)
		if h.Hz[i] > 0 {
			f.Cents = append(f.Cents, 1200*math.Log2(got/h.Hz[i]))
		} else {
			f.Cents = append(f.Cents, 0)
		}
	}
	// Trim the tail once the volume has reached zero: a table that keeps stepping after
	// silence only costs the envelope cursor frames it could spend on the next hit.
	for len(f.EnvV) > 1 && f.EnvV[len(f.EnvV)-1] == 0 && f.EnvV[len(f.EnvV)-2] == 0 {
		f.EnvV = f.EnvV[:len(f.EnvV)-1]
		f.EnvF = f.EnvF[:len(f.EnvF)-1]
		f.Hz = f.Hz[:len(f.Hz)-1]
		f.Cents = f.Cents[:len(f.Cents)-1]
	}
	return f
}

func bandpass(x []float64, rate, lo, hi float64) []float64 {
	n := 1
	for n < len(x) {
		n <<= 1
	}
	re := make([]float64, n)
	im := make([]float64, n)
	copy(re, x)
	fft(re, im, false)
	l, h := int(lo*float64(n)/rate), int(hi*float64(n)/rate)
	for k := 0; k < n/2; k++ {
		if k < l || k > h {
			re[k], im[k] = 0, 0
			re[n-1-k], im[n-1-k] = 0, 0
		}
	}
	fft(re, im, true)
	return re[:len(x)]
}

// dominant returns the strongest frequency in [lo,hi] by autocorrelation, which holds
// up at 40 Hz where an FFT bin is wider than a semitone.
func dominant(seg []float64, rate, lo, hi float64) float64 {
	hz, _ := dominantConf(seg, rate, lo, hi)
	return hz
}

func dominantConf(seg []float64, rate, lo, hi float64) (float64, float64) {
	if len(seg) < 32 {
		return 0, 0
	}
	m := 0.0
	for _, v := range seg {
		m += v
	}
	m /= float64(len(seg))
	x := make([]float64, len(seg))
	for i, v := range seg {
		x[i] = v - m
	}
	r0 := 0.0
	for _, v := range x {
		r0 += v * v
	}
	if r0 == 0 {
		return 0, 0
	}
	loLag, hiLag := int(rate/hi), int(rate/lo)
	if hiLag >= len(x) {
		hiLag = len(x) - 1
	}
	if loLag < 1 || loLag >= hiLag {
		return 0, 0
	}
	// Normalise by the OVERLAP, not by r0. A frame is 524 samples and a 45 Hz lag is
	// 699 of them, so the long lags this band needs sum far fewer terms than r0 does;
	// dividing by r0 makes every low note look weak and the detector returns nothing.
	// Measured: a clean 90 Hz sweep reported 0 Hz on every frame before this.
	best, bestLag := -1.0, 0
	for lag := loLag; lag <= hiLag; lag++ {
		s, cnt := 0.0, 0
		for i := 0; i+lag < len(x); i++ {
			s += x[i] * x[i+lag]
			cnt++
		}
		if cnt == 0 {
			continue
		}
		s /= float64(cnt)
		if s > best {
			best, bestLag = s, lag
		}
	}
	conf := 0.0
	if r0 > 0 {
		conf = best / (r0 / float64(len(x)))
	}
	if bestLag == 0 || conf < 0.2 {
		return 0, conf
	}
	return rate / float64(bestLag), conf
}

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
