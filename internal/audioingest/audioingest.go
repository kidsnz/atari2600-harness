// Package audioingest reads a reference RECORDING and returns the numbers an author
// needs to reproduce it on the TIA: its tempo, its sixteenth-note grid, and the bass
// note sitting in each step, already mapped onto the pitches the hardware can make.
//
// Why this exists. The repo could already COMPARE audio -- audiospec measures spectral
// distance, pcmcheck grades sample fidelity, golden_audio hashes the register chain --
// but every one of those needs the ROM to exist first. There was no path in the other
// direction, no audio counterpart to analyze_image: nothing that takes a recording and
// produces something authorable. Reproducing a piece of music therefore depended on
// somebody transcribing it by ear, which is not a capability this harness has.
//
// What it does NOT do. It reports one monophonic pitch per step from the low band, so
// it recovers a BASSLINE and not an arrangement; chords, melody over the bass, and
// anything above ~300 Hz are outside what it looks at. Every note carries a confidence
// and a cents error, and both are meant to be read: the TIA's pitch grid is uneven, so
// "the nearest note the hardware can play" is sometimes 30 cents away, and that is a
// property of the machine that the caller has to decide about, not something to bury.
package audioingest

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/kidsnz/atari2600-harness/internal/audiospec"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// ---------------------------------------------------------------- WAV decoding

// DecodeWAV reads a 16-bit PCM RIFF/WAVE file and returns mono samples in -1..1.
// Only 16-bit PCM is accepted, and deliberately so: anything else should be converted
// by the caller (`ffmpeg -i in.mp3 -ac 1 -ar 44100 -c:a pcm_s16le out.wav`) rather than
// handled by a half-implemented decoder here that would silently mis-read a format.
func DecodeWAV(b []byte) (samples []float64, rate int, err error) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("audioingest: not a RIFF/WAVE file")
	}
	var channels, bits int
	var data []byte
	for pos := 12; pos+8 <= len(b); {
		id := string(b[pos : pos+4])
		n := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		body := b[pos+8:]
		if n > len(body) {
			n = len(body)
		}
		switch id {
		case "fmt ":
			if n < 16 {
				return nil, 0, fmt.Errorf("audioingest: fmt chunk is %d bytes, want >= 16", n)
			}
			if f := binary.LittleEndian.Uint16(body[0:2]); f != 1 {
				return nil, 0, fmt.Errorf("audioingest: format %d is not PCM; convert with ffmpeg first", f)
			}
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			rate = int(binary.LittleEndian.Uint32(body[4:8]))
			bits = int(binary.LittleEndian.Uint16(body[14:16]))
		case "data":
			data = body[:n]
		}
		pos += 8 + n
		if n%2 == 1 {
			pos++ // RIFF chunks are word-aligned
		}
	}
	if bits != 16 {
		return nil, 0, fmt.Errorf("audioingest: %d-bit samples; only 16-bit PCM is read", bits)
	}
	if channels < 1 || rate <= 0 || data == nil {
		return nil, 0, fmt.Errorf("audioingest: incomplete WAV (channels=%d rate=%d data=%v)", channels, rate, data != nil)
	}
	n := len(data) / 2 / channels
	samples = make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for c := 0; c < channels; c++ {
			v := int16(binary.LittleEndian.Uint16(data[(i*channels+c)*2:]))
			sum += float64(v) / 32768.0
		}
		samples[i] = sum / float64(channels)
	}
	return samples, rate, nil
}

// ---------------------------------------------------------------- tempo

const fluxHop = 512 // ~11.6 ms at 44.1 kHz

// OnsetEnvelope returns half-wave-rectified spectral flux, one value per fluxHop
// samples. Flux rather than plain energy because a kick landing under a sustained pad
// changes the SHAPE of the spectrum without changing its loudness much, and an energy
// envelope walks straight past it.
func OnsetEnvelope(samples []float64, _ int) []float64 {
	const win = 1024
	if len(samples) < win*2 {
		return nil
	}
	var env []float64
	var prev []float64
	for start := 0; start+win <= len(samples); start += fluxHop {
		mag := audiospec.MagnitudeSpectrum(samples[start : start+win])
		if prev != nil {
			f := 0.0
			for i := range mag {
				if d := mag[i] - prev[i]; d > 0 {
					f += d
				}
			}
			env = append(env, f)
		}
		prev = mag
	}
	return env
}

// EstimateTempo autocorrelates the onset envelope and returns the best BPM in
// [minBPM, maxBPM] together with a 0..1 strength. Strength is the point: a track with
// no steady pulse still produces SOME maximum, and reporting it without saying how
// weak it was would turn noise into a tempo.
func EstimateTempo(env []float64, sampleRate int, minBPM, maxBPM float64) (bpm, strength float64) {
	if len(env) < 16 {
		return 0, 0
	}
	mean := 0.0
	for _, v := range env {
		mean += v
	}
	mean /= float64(len(env))
	d := make([]float64, len(env))
	for i, v := range env {
		d[i] = v - mean
	}
	hopSec := float64(fluxHop) / float64(sampleRate)
	r0 := 0.0
	for _, v := range d {
		r0 += v * v
	}
	if r0 == 0 {
		return 0, 0
	}
	best, bestLag := 0.0, 0
	loLag := int(60.0 / maxBPM / hopSec)
	hiLag := int(60.0 / minBPM / hopSec)
	if hiLag >= len(d) {
		hiLag = len(d) - 1
	}
	for lag := loLag; lag <= hiLag; lag++ {
		s := 0.0
		for i := 0; i+lag < len(d); i++ {
			s += d[i] * d[i+lag]
		}
		s /= float64(len(d) - lag)
		if s > best {
			best, bestLag = s, lag
		}
	}
	if bestLag == 0 {
		return 0, 0
	}
	return 60.0 / (float64(bestLag) * hopSec), best / (r0 / float64(len(d)))
}

// BeatPhase returns the offset in seconds of the first beat, by sliding a pulse train
// of the given period over the onset envelope and taking the best-scoring alignment.
func BeatPhase(env []float64, sampleRate int, beatSec float64) float64 {
	if len(env) == 0 || beatSec <= 0 {
		return 0
	}
	hopSec := float64(fluxHop) / float64(sampleRate)
	period := beatSec / hopSec
	best, bestOff := -1.0, 0.0
	for off := 0.0; off < period; off += 0.25 {
		s := 0.0
		for k := 0; ; k++ {
			i := int(math.Round(off + float64(k)*period))
			if i >= len(env) {
				break
			}
			s += env[i]
		}
		if s > best {
			best, bestOff = s, off
		}
	}
	return bestOff * hopSec
}

// ---------------------------------------------------------------- bass pitch

// Note is one step of the recovered pattern.
type Note struct {
	Step       int     // index within the analysed span
	StartSec   float64 //
	Hz         float64 // measured fundamental, 0 when the step is a rest
	Confidence float64 // 0..1 autocorrelation peak height; below ~0.3 do not trust it
	AUDC, AUDF int     // the nearest pitch the TIA can make (-1, -1 for a rest)
	Cents      float64 // how far that pitch is from the measurement
}

// lowpass is a two-pole Butterworth-ish smoother, run forwards then backwards so it
// adds no phase shift -- a shifted envelope would move every note off its step.
func lowpass(x []float64, cutoff, rate float64) []float64 {
	a := math.Exp(-2 * math.Pi * cutoff / rate)
	y := make([]float64, len(x))
	prev := 0.0
	for i, v := range x {
		prev = (1-a)*v + a*prev
		y[i] = prev
	}
	prev = 0
	for i := len(y) - 1; i >= 0; i-- {
		prev = (1-a)*y[i] + a*prev
		y[i] = prev
	}
	return y
}

// F0 returns the fundamental of one window by autocorrelation, searching loHz..hiHz,
// plus the normalised peak height as a confidence. Autocorrelation rather than an FFT
// peak because a 60 Hz fundamental needs a window long enough that an FFT's bin
// spacing gets coarse relative to a semitone down here.
func F0(w []float64, rate, loHz, hiHz float64) (hz, conf float64) {
	if len(w) < 64 {
		return 0, 0
	}
	mean := 0.0
	for _, v := range w {
		mean += v
	}
	mean /= float64(len(w))
	x := make([]float64, len(w))
	for i, v := range w {
		x[i] = v - mean
	}
	r0 := 0.0
	for _, v := range x {
		r0 += v * v
	}
	if r0 == 0 {
		return 0, 0
	}
	// Ceil, not truncate. int(44100/800) is 55 and 44100/55 is 801.8 Hz, so truncating the
	// short end lets the search return a frequency above the hiHz it was given. Rounding the
	// lag DOWN in frequency at both ends keeps the answer inside the range the caller asked
	// for, which is the whole contract of taking a range.
	loLag, hiLag := int(math.Ceil(rate/hiHz)), int(rate/loHz)
	if hiLag >= len(x) {
		hiLag = len(x) - 1
	}
	best, bestLag := 0.0, 0
	for lag := loLag; lag <= hiLag; lag++ {
		s := 0.0
		for i := 0; i+lag < len(x); i++ {
			s += x[i] * x[i+lag]
		}
		if s > best {
			best, bestLag = s, lag
		}
	}
	if bestLag == 0 {
		return 0, 0
	}
	// Parabolic interpolation around the peak: one lag at 4 kHz is ~2 semitones at
	// 100 Hz, so taking the integer lag would quantise the answer more coarsely than
	// the thing being measured.
	// Interpolate only when the peak has a lag on each side INSIDE the search range. On the
	// edge there is no summit to interpolate, only a slope: den goes to nothing and the
	// correction runs away. Measured, a window whose true period lay outside loHz..hiHz
	// returned -488 Hz that way, and clamping the correction to half a sample still let it
	// return 809 Hz from a search that was told to stop at 800. A frequency outside the
	// range the caller asked for is not a near miss; it is a value no caller can defend
	// against. At the edge the integer lag is the honest answer, and F0Checked is what says
	// the answer is suspect.
	delta := 0.0
	if bestLag > loLag && bestLag < hiLag {
		y1, y2, y3 := acf(x, bestLag-1), best, acf(x, bestLag+1)
		if den := y1 - 2*y2 + y3; den != 0 {
			delta = 0.5 * (y1 - y3) / den
			if delta > 0.5 {
				delta = 0.5
			} else if delta < -0.5 {
				delta = -0.5
			}
		}
	}
	return rate / (float64(bestLag) + delta), best / r0
}

func acf(x []float64, lag int) float64 {
	if lag <= 0 || lag >= len(x) {
		return 0
	}
	s := 0.0
	for i := 0; i+lag < len(x); i++ {
		s += x[i] * x[i+lag]
	}
	return s
}

// NearestTIA finds the (AUDC, AUDF) whose frequency is closest to hz in log space,
// searching only the given waveforms. Returns the cents error, signed: positive means
// the hardware plays SHARP of the target.
func NearestTIA(hz float64, waveforms []int, baseClock float64) (audc, audf int, cents float64) {
	best := math.Inf(1)
	audc, audf = -1, -1
	for _, c := range waveforms {
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

// BassNotes slices the span into `steps` equal windows starting at startSec and reports
// the bass pitch in each. A window whose RMS falls below restRatio of the loudest
// window is called a rest rather than being given the pitch of whatever leaked in.
func BassNotes(samples []float64, rate int, startSec, stepSec float64, steps int,
	waveforms []int, restRatio float64) []Note {

	const dsRate = 4000.0
	lp := lowpass(samples, 320, float64(rate))
	factor := int(float64(rate) / dsRate)
	if factor < 1 {
		factor = 1
	}
	ds := make([]float64, 0, len(lp)/factor)
	for i := 0; i < len(lp); i += factor {
		ds = append(ds, lp[i])
	}
	effRate := float64(rate) / float64(factor)

	out := make([]Note, 0, steps)
	rms := make([]float64, steps)
	windows := make([][]float64, steps)
	for s := 0; s < steps; s++ {
		a := int((startSec + float64(s)*stepSec) * effRate)
		b := int((startSec + float64(s+1)*stepSec) * effRate)
		if a < 0 {
			a = 0
		}
		if b > len(ds) {
			b = len(ds)
		}
		if a >= b {
			windows[s] = nil
			continue
		}
		w := ds[a:b]
		windows[s] = w
		e := 0.0
		for _, v := range w {
			e += v * v
		}
		rms[s] = math.Sqrt(e / float64(len(w)))
	}
	loudest := 0.0
	for _, v := range rms {
		if v > loudest {
			loudest = v
		}
	}
	for s := 0; s < steps; s++ {
		n := Note{Step: s, StartSec: startSec + float64(s)*stepSec, AUDC: -1, AUDF: -1}
		if windows[s] != nil && loudest > 0 && rms[s] >= restRatio*loudest {
			hz, conf := F0(windows[s], effRate, 35, 300)
			if hz > 0 {
				n.Hz, n.Confidence = hz, conf
				n.AUDC, n.AUDF, n.Cents = NearestTIA(hz, waveforms, audio.BaseClockNTSC)
			}
		}
		out = append(out, n)
	}
	return out
}

// ---- octave safety -------------------------------------------------------------
//
// WHY THIS EXISTS. F0 searches loHz..hiHz and cannot return anything outside it. That is
// correct behaviour and it is also a trap: if the caller's band excludes the real
// fundamental, F0 returns a HARMONIC of it, confidently and with no way for the caller to
// tell. On the Transistor Dub work a lead line was measured over 110-800 Hz for two days
// and read 194 Hz; its fundamental is 96.9 Hz, below the band, and 194 Hz was its second
// harmonic. Two independent methods were needed to find that out. This makes it a
// property the code reports rather than a mistake a person has to remember not to make.
//
// THE TEST. Autocorrelation at twice the found lag is high for ANY periodic signal, so
// "energy at 2L" proves nothing on its own. What separates the two cases is which lag
// correlates BETTER once the comparison is made lag-fair: for a signal whose period really
// is L, ncc(2L) <= ncc(L); for one whose period is 2L, ncc(2L) > ncc(L) strictly. The
// normalised cross-correlation below divides by the energy of the two overlapping segments
// so that it does not decay with lag on its own, which the plain sum does.

// F0Detail is what F0 found, together with the evidence for whether it is really the
// fundamental or a harmonic of one the search range could not reach.
type F0Detail struct {
	Hz   float64 // what F0 returns
	Conf float64 // its normalised autocorrelation peak

	// SubHz is a LOWER fundamental that fits the same window better. Zero when none was
	// found. SubRatio is Hz/SubHz when that is close to a whole number (2 = an octave down,
	// 3 = a twelfth) and 0 when the relationship is not a simple one.
	SubHz    float64
	SubRatio int
	SubNCC   float64 // the lag-fair correlation there
	NCC      float64 // the lag-fair correlation at Hz, for comparison

	// BelowRange is the finding that matters: the lower fundamental lies under loHz, so the
	// caller's own search range is what hid it.
	BelowRange bool
}

// Suspect reports whether the caller should not trust Hz as a fundamental.
func (d F0Detail) Suspect() bool { return d.SubHz > 0 }

// ncc is the normalised cross-correlation of x with itself at lag, in -1..1. Unlike the
// plain sum it does not fall off with lag merely because fewer samples overlap, which is
// what makes correlations at L and 2L comparable.
func ncc(x []float64, lag int) float64 {
	if lag <= 0 || lag >= len(x) {
		return 0
	}
	var num, ea, eb float64
	for i := 0; i+lag < len(x); i++ {
		num += x[i] * x[i+lag]
		ea += x[i] * x[i]
		eb += x[i+lag] * x[i+lag]
	}
	if ea <= 0 || eb <= 0 {
		return 0
	}
	return num / math.Sqrt(ea*eb)
}

// F0Checked runs F0 and then goes looking BELOW the search range for a period that fits
// the same window better. subMargin is how much better the lower lag must correlate before
// it is believed; 0.02 rejects the ordinary jitter of an in-range answer.
//
// It searches rather than testing integer multiples of the found lag, because the found
// peak is not always a harmonic of the true fundamental: a squarewave read from above its
// fundamental leaves partials 3f, 5f and 7f, whose in-band autocorrelation peak sits at no
// simple ratio to f at all. floorHz bounds how far down it looks; pass 0 for loHz/4, and
// note that the window must be long enough to hold a couple of cycles at floorHz or there
// is nothing there to find.
func F0Checked(w []float64, rate, loHz, hiHz, floorHz float64) F0Detail {
	d := F0Detail{}
	d.Hz, d.Conf = F0(w, rate, loHz, hiHz)
	if d.Hz <= 0 {
		return d
	}
	if floorHz <= 0 {
		floorHz = loHz / 4
	}
	if floorHz < 20 {
		floorHz = 20
	}
	mean := 0.0
	for _, v := range w {
		mean += v
	}
	mean /= float64(len(w))
	x := make([]float64, len(w))
	for i, v := range w {
		x[i] = v - mean
	}
	d.NCC = ncc(x, int(math.Round(rate/d.Hz)))

	const subMargin = 0.02
	// Everything from just under the search range down to floorHz. Two cycles must fit, or
	// the correlation is measured on too little overlap to mean anything.
	//
	// Decimated by DEC first. This search is for LOW frequencies, so throwing away three of
	// every four samples costs nothing it is looking for and makes it sixteen times cheaper
	// — the loop is O(samples x lags) and undecimated it took 18 s on the package's own
	// tests, against a CI budget with under two minutes of slack.
	const dec = 4
	dx := make([]float64, len(x)/dec)
	for i := range dx {
		dx[i] = x[i*dec]
	}
	drate := rate / dec
	from := int(drate/loHz) + 1
	to := int(drate / floorHz)
	if to > len(dx)/2 {
		to = len(dx) / 2
	}
	dNCC := ncc(dx, int(math.Round(drate/d.Hz)))
	if dNCC > d.NCC {
		dNCC = d.NCC // compare like with like; never let decimation flatter the sub-search
	}
	bestNCC, bestLag := dNCC+subMargin, 0
	for lag := from; lag <= to; lag++ {
		if c := ncc(dx, lag); c > bestNCC {
			bestNCC, bestLag = c, lag
		}
	}
	if bestLag == 0 {
		return d
	}
	// Refine at full rate around the decimated answer, so SubHz is not quantised by dec.
	d.SubHz = drate / float64(bestLag)
	fl := int(math.Round(rate / d.SubHz))
	for lag := fl - dec; lag <= fl+dec; lag++ {
		if lag > 0 && lag < len(x) {
			if c := ncc(x, lag); c > d.SubNCC {
				d.SubNCC, d.SubHz = c, rate/float64(lag)
			}
		}
	}
	_ = bestNCC
	// Report the ratio only when it really is close to a whole number; "the fundamental is
	// an octave down" and "there is a better period down there somewhere" are different
	// findings and saying the first when only the second is true would be a guess.
	r := d.Hz / d.SubHz
	if math.Abs(r-math.Round(r)) < 0.06 {
		d.SubRatio = int(math.Round(r))
	}
	d.BelowRange = d.SubHz < loHz
	return d
}

// ---- the grid, measured rather than supplied ------------------------------------------
//
// WHY THIS EXISTS. audioingest takes -from and drumfit takes -t0: the start of the loop is an
// INPUT to every audio tool here, and drumfit's own documentation says to read it off
// audioingest. So one wrong value propagates through the whole chain with nothing to catch it.
// Measured cost: two delivered mp3s each carry about 233 ms of digital silence before the
// first sample, a grid was built without accounting for it, and for two days it sat two
// sixteenths out of phase — which turned four-on-the-floor into "the bass is on the offbeat"
// and made every note reading coherent and wrong.

// LeadingSilence returns the seconds before the first short-term window whose RMS rises above
// frac of the file's loudest window. It is deliberately relative: an absolute threshold cannot
// tell a quiet recording from a silent lead-in, and a delivered file's level is not knowable
// in advance. frac <= 0 uses 0.02, which is 34 dB below peak.
func LeadingSilence(samples []float64, rate int, frac float64) float64 {
	if len(samples) == 0 || rate <= 0 {
		return 0
	}
	if frac <= 0 {
		frac = 0.02
	}
	win := rate / 200 // 5 ms — finer than a sixteenth by two orders of magnitude
	if win < 8 {
		win = 8
	}
	var peak float64
	rms := make([]float64, 0, len(samples)/win+1)
	for i := 0; i+win <= len(samples); i += win {
		var s float64
		for _, v := range samples[i : i+win] {
			s += v * v
		}
		r := math.Sqrt(s / float64(win))
		rms = append(rms, r)
		if r > peak {
			peak = r
		}
	}
	if peak == 0 {
		return float64(len(samples)) / float64(rate)
	}
	for i, r := range rms {
		if r >= frac*peak {
			return float64(i*win) / float64(rate)
		}
	}
	return float64(len(samples)) / float64(rate)
}

// PatternBars reports how many bars long the repeating unit is, by comparing each bar's
// per-sixteenth energy profile with the bars that follow it.
//
// WHY IT IS NEEDED. "Reproduce the first bar" and "reproduce the loop" are the same request
// only when the loop is one bar. On the material this was written for the lead alternates
// between two shapes about a minor third apart at eleven of sixteen steps, and a one-bar loop
// would have been a different piece of music. Nothing measured that until it was asked for
// specially.
//
// scores[p-1] is the mean correlation between bars p apart, so a reader can see the margin
// rather than trust the verdict. The answer is the SMALLEST period within margin of the best
// score: a two-bar pattern also correlates well at four and eight bars, and reporting eight
// would be true and useless.
func PatternBars(samples []float64, rate int, t0, barSec float64, maxBars int) (int, []float64) {
	if barSec <= 0 || rate <= 0 || maxBars < 1 {
		return 1, nil
	}
	step := barSec / 16
	var bars [][]float64
	for b := 0; ; b++ {
		v := make([]float64, 16)
		ok := true
		for s := 0; s < 16; s++ {
			i0 := int((t0 + float64(b)*barSec + float64(s)*step) * float64(rate))
			i1 := i0 + int(step*float64(rate))
			if i0 < 0 || i1 > len(samples) {
				ok = false
				break
			}
			var e float64
			for _, x := range samples[i0:i1] {
				e += x * x
			}
			v[s] = math.Sqrt(e / float64(i1-i0))
		}
		if !ok {
			break
		}
		bars = append(bars, v)
	}
	if len(bars) < 2 {
		return 1, nil
	}
	if maxBars > len(bars)-1 {
		maxBars = len(bars) - 1
	}
	scores := make([]float64, maxBars)
	for p := 1; p <= maxBars; p++ {
		var sum float64
		var n int
		for i := 0; i+p < len(bars); i++ {
			sum += corr(bars[i], bars[i+p])
			n++
		}
		if n > 0 {
			scores[p-1] = sum / float64(n)
		}
	}
	best := 0.0
	for _, s := range scores {
		if s > best {
			best = s
		}
	}
	// 0.02 of correlation: enough that a genuinely better period wins, small enough that the
	// smallest true period is not passed over for a multiple of itself that scores a hair more.
	for p, s := range scores {
		if s >= best-0.02 {
			return p + 1, scores
		}
	}
	return 1, scores
}

func corr(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var ma, mb float64
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(len(a))
	mb /= float64(len(b))
	var num, va, vb float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		num += da * db
		va += da * da
		vb += db * db
	}
	if va <= 0 || vb <= 0 {
		return 0
	}
	return num / math.Sqrt(va*vb)
}

// FirstOnset returns the time of the strongest onset in the first `within` seconds at or after
// `after`. It is how a downbeat is found when a beat PHASE is not enough: phase is known only
// modulo a beat, so it says where the beats are and not which one starts the bar.
//
// Measured need: on a real track the phase estimate and the first audible hit disagreed by
// about 0.2 s — most of a beat — and the phase was the one that was wrong. An electronic track
// starts on a hit, so the hit is the better anchor and the phase is the check on it.
func FirstOnset(env []float64, sampleRate int, after, within float64) float64 {
	if len(env) == 0 || sampleRate <= 0 {
		return after
	}
	// OnsetEnvelope is one value per sample of the original signal.
	i0 := int(after * float64(sampleRate))
	i1 := i0 + int(within*float64(sampleRate))
	if i0 < 0 {
		i0 = 0
	}
	if i1 > len(env) {
		i1 = len(env)
	}
	if i0 >= i1 {
		return after
	}
	best, bi := env[i0], i0
	for i := i0; i < i1; i++ {
		if env[i] > best {
			best, bi = env[i], i
		}
	}
	return float64(bi) / float64(sampleRate)
}

// BandPass filters in place-safe fashion with a 2nd-order Butterworth band-pass, applied
// forward then backward so the result has no phase shift — which matters here because the
// output is used to locate events in TIME.
func BandPass(x []float64, rate int, lo, hi float64) []float64 {
	if lo <= 0 || hi <= lo || rate <= 0 {
		out := make([]float64, len(x))
		copy(out, x)
		return out
	}
	f0 := math.Sqrt(lo * hi)
	q := f0 / (hi - lo)
	w := 2 * math.Pi * f0 / float64(rate)
	alpha := math.Sin(w) / (2 * q)
	b0, b1, b2 := alpha, 0.0, -alpha
	a0, a1, a2 := 1+alpha, -2*math.Cos(w), 1-alpha
	b0, b1, b2 = b0/a0, b1/a0, b2/a0
	a1, a2 = a1/a0, a2/a0

	pass := func(in []float64) []float64 {
		out := make([]float64, len(in))
		var x1, x2, y1, y2 float64
		for i, v := range in {
			y := b0*v + b1*x1 + b2*x2 - a1*y1 - a2*y2
			out[i] = y
			x2, x1 = x1, v
			y2, y1 = y1, y
		}
		return out
	}
	fwd := pass(x)
	rev := make([]float64, len(fwd))
	for i := range fwd {
		rev[i] = fwd[len(fwd)-1-i]
	}
	rev = pass(rev)
	out := make([]float64, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out
}
