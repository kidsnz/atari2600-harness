// Package audiospec is a frequency-domain audio comparator (VV-13,
// docs/capability-gap-audit.md). golden_audio hashes the audio register chain
// and the RMS envelope tells you "how loud over time"; neither distinguishes two
// sounds with the SAME loudness envelope but DIFFERENT pitch/timbre ("inverted
// twins"). A magnitude spectrum (FFT) does. This package adds that modality:
// spectral distance + RMS-envelope distance over a captured PCM stream.
//
// Pure Go, no external dependencies. Input is the raw per-channel sample stream
// from emu.AudioSamples (TIA: ~31.4 kHz NTSC, 4-bit 0..15).
package audiospec

import "math"

// ToFloat converts raw 0..15 TIA samples to mean-centred float64 (removes the DC
// offset so the spectrum reflects the waveform, not its bias).
func ToFloat(s []uint8) []float64 {
	if len(s) == 0 {
		return nil
	}
	out := make([]float64, len(s))
	var sum float64
	for _, v := range s {
		sum += float64(v)
	}
	mean := sum / float64(len(s))
	for i, v := range s {
		out[i] = float64(v) - mean
	}
	return out
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// fft performs an in-place iterative radix-2 Cooley-Tukey FFT. len(re)==len(im)
// must be a power of two.
func fft(re, im []float64) {
	n := len(re)
	// bit-reversal permutation
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wr, wi := math.Cos(ang), math.Sin(ang)
		for i := 0; i < n; i += length {
			cr, ci := 1.0, 0.0
			for k := 0; k < length/2; k++ {
				a := i + k
				b := i + k + length/2
				tr := cr*re[b] - ci*im[b]
				ti := cr*im[b] + ci*re[b]
				re[b] = re[a] - tr
				im[b] = im[a] - ti
				re[a] += tr
				im[a] += ti
				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
}

// MagnitudeSpectrum returns the magnitude of the first N/2 FFT bins of the
// (zero-padded to a power of two) signal.
func MagnitudeSpectrum(samples []float64) []float64 {
	if len(samples) == 0 {
		return nil
	}
	n := nextPow2(len(samples))
	re := make([]float64, n)
	im := make([]float64, n)
	copy(re, samples)
	fft(re, im)
	half := n / 2
	mag := make([]float64, half)
	for i := 0; i < half; i++ {
		mag[i] = math.Hypot(re[i], im[i])
	}
	return mag
}

// DominantFreq returns the frequency (Hz) of the strongest non-DC bin.
func DominantFreq(samples []float64, sampleRate float64) float64 {
	mag := MagnitudeSpectrum(samples)
	if len(mag) < 2 {
		return 0
	}
	n := nextPow2(len(samples))
	best, bestI := 0.0, 1
	for i := 1; i < len(mag); i++ { // skip DC bin 0
		if mag[i] > best {
			best, bestI = mag[i], i
		}
	}
	return float64(bestI) * sampleRate / float64(n)
}

// RMSEnvelope returns the per-window root-mean-square amplitude (loudness over
// time). win and hop are in samples.
func RMSEnvelope(samples []float64, win, hop int) []float64 {
	if win <= 0 || hop <= 0 || len(samples) < win {
		return nil
	}
	var env []float64
	for start := 0; start+win <= len(samples); start += hop {
		var sum float64
		for i := start; i < start+win; i++ {
			sum += samples[i] * samples[i]
		}
		env = append(env, math.Sqrt(sum/float64(win)))
	}
	return env
}

// cosineDistance = 1 - cosine similarity of two non-negative vectors (0 =
// identical direction, 1 = no shared energy). Shorter vector is zero-extended.
func cosineDistance(a, b []float64) float64 {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		var av, bv float64
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		if na == 0 && nb == 0 {
			return 0
		}
		return 1
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

// SpectralDistance compares two signals by their magnitude spectra (0 =
// spectrally identical, →1 = disjoint frequency content). This is the axis that
// separates inverted twins.
func SpectralDistance(a, b []float64) float64 {
	return cosineDistance(MagnitudeSpectrum(a), MagnitudeSpectrum(b))
}

// EnvelopeDistance compares two signals by their RMS loudness envelopes
// (0 = same loudness contour). Two different pitches at equal volume have ~0
// envelope distance yet a large spectral distance.
func EnvelopeDistance(a, b []float64, win, hop int) float64 {
	return cosineDistance(RMSEnvelope(a, win, hop), RMSEnvelope(b, win, hop))
}
