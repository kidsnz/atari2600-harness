// audiospec — VV-13 frequency-domain audio comparison (docs/capability-gap-audit.md).
//
// Captures a channel's PCM from two ROMs and reports their spectral distance
// (FFT magnitude) and RMS-envelope distance, plus each one's dominant frequency.
// The spectral axis separates "inverted twins" — sounds with the same loudness
// envelope (and the same coarse golden_audio intent) but different pitch/timbre.
//
//	go run ./cmd/audiospec -a cand.bin -b ref.bin -frames 30 -ch 0
//
// Exit 1 if spectral distance exceeds -max (a regression gate); 0 otherwise.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/audiospec"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const tiaRate = 31400.0 // NTSC TIA sample rate

func capture(rom, spec string, frames, ch int) ([]float64, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(rom); err != nil {
		return nil, err
	}
	if err := e.EnableAudioCapture(); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	ch0, ch1 := e.AudioSamples()
	raw := ch0
	if ch == 1 {
		raw = ch1
	}
	return audiospec.ToFloat(raw), nil
}

func main() {
	a := flag.String("a", "", "ROM A (.bin, required)")
	b := flag.String("b", "", "ROM B (.bin, required)")
	frames := flag.Int("frames", 30, "frames of audio to capture")
	ch := flag.Int("ch", 0, "TIA channel (0 or 1)")
	spec := flag.String("spec", "NTSC", "TV spec")
	max := flag.Float64("max", -1, "fail (exit 1) if spectral distance > this")
	flag.Parse()
	if *a == "" || *b == "" {
		fmt.Fprintln(os.Stderr, "usage: audiospec -a x.bin -b y.bin [-frames 30 -ch 0 -max 0.2]")
		os.Exit(2)
	}

	sa, err := capture(*a, *spec, *frames, *ch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: capture %s: %v\n", *a, err)
		os.Exit(2)
	}
	sb, err := capture(*b, *spec, *frames, *ch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: capture %s: %v\n", *b, err)
		os.Exit(2)
	}

	specDist := audiospec.SpectralDistance(sa, sb)
	envDist := audiospec.EnvelopeDistance(sa, sb, 256, 128)
	out := struct {
		A             string  `json:"a"`
		B             string  `json:"b"`
		Channel       int     `json:"channel"`
		Samples       int     `json:"samples"`
		SpectralDist  float64 `json:"spectral_distance"`
		EnvelopeDist  float64 `json:"envelope_distance"`
		DominantFreqA float64 `json:"dominant_freq_a"`
		DominantFreqB float64 `json:"dominant_freq_b"`
	}{*a, *b, *ch, len(sa), specDist, envDist,
		audiospec.DominantFreq(sa, tiaRate), audiospec.DominantFreq(sb, tiaRate)}
	j, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(j))

	if *max >= 0 && specDist > *max {
		fmt.Fprintf(os.Stderr, "FAIL: spectral distance %.4f > max %.4f\n", specDist, *max)
		os.Exit(1)
	}
}
