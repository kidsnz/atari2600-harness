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
	loLag, hiLag := int(rate/hiHz), int(rate/loHz)
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
	y1, y2, y3 := acf(x, bestLag-1), best, acf(x, bestLag+1)
	den := y1 - 2*y2 + y3
	delta := 0.0
	if den != 0 {
		delta = 0.5 * (y1 - y3) / den
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
