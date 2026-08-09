// Command drumfit measures a drum in a reference recording and prints TIA envelope
// tables fitted to it, so a kick on this machine can be the RECORD's kick rather than
// the generic 2600 one.
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	drumfit -wav track.wav -bpm 124 -t0 0.2571 -every 1 -band 30,120 -audc 6 -frames 14
//
// -t0/-bpm/-every give the onsets: the first beat, the tempo, and how many beats apart
// the hits are. Read them off cmd/audioingest, which fits them to the kick track.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/internal/drumfit"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

func main() {
	wav := flag.String("wav", "", "16-bit PCM WAV to measure")
	bpm := flag.Float64("bpm", 0, "tempo, for placing the onsets")
	t0 := flag.Float64("t0", 0, "time of the first onset, seconds")
	every := flag.Float64("every", 1, "beats between hits (1 = four-on-the-floor, 2 = backbeat)")
	hits := flag.Int("hits", 32, "how many hits to average")
	bandS := flag.String("band", "30,120", "the drum's frequency band, lo,hi")
	audc := flag.Int("audc", 6, "the TIA waveform to fit")
	frames := flag.Int("frames", 14, "frames of envelope to follow")
	peak := flag.Int("peak", 15, "the volume the loudest frame maps to")
	name := flag.String("name", "Kick", "label for the emitted tables")
	pwin := flag.Int("pitchwin", 3, "frames of context for the PITCH estimate. 1 is unusable below ~120 Hz: one frame holds under two cycles there.")
	flag.Parse()
	if *wav == "" || *bpm <= 0 {
		flag.Usage()
		os.Exit(2)
	}
	var lo, hi float64
	fmt.Sscanf(strings.ReplaceAll(*bandS, ",", " "), "%f %f", &lo, &hi)

	blob, err := os.ReadFile(*wav)
	must(err)
	x, rate, err := audioingest.DecodeWAV(blob)
	must(err)

	beat := 60.0 / *bpm
	var onsets []float64
	for i := 0; i < *hits; i++ {
		t := *t0 + float64(i)**every*beat
		if int(t*float64(rate)) >= len(x) {
			break
		}
		onsets = append(onsets, t)
	}
	h, err := drumfit.MeasureWin(x, float64(rate), onsets, [2]float64{lo, hi}, *frames, *pwin)
	must(err)
	f := drumfit.Fit(h, *audc, *peak, audio.BaseClockNTSC)

	fmt.Printf("%s: %d onsets averaged, %.0f-%.0f Hz, AUDC %d\n\n", *name, len(onsets), lo, hi, *audc)
	fmt.Printf("%-6s %9s %9s %6s   %5s %5s %9s %8s\n", "frame", "amp", "Hz", "conf", "AUDV", "AUDF", "TIA Hz", "cents")
	for i := range f.EnvV {
		hz := "-"
		if h.Hz[i] > 0 {
			hz = fmt.Sprintf("%.1f", h.Hz[i])
		}
		fmt.Printf("%-6d %9.3f %9s %6.2f   %5d %5d %9.2f %+8.0f\n", i, h.Amp[i], hz, h.Conf[i], f.EnvV[i], f.EnvF[i], f.Hz[i], f.Cents[i])
	}
	fmt.Printf("\n; ---- fitted from %s ----\n", *wav)
	fmt.Printf("%sEnd = %d\n", *name, len(f.EnvV))
	fmt.Printf("EnvV:    byte %s\n", join(f.EnvV))
	fmt.Printf("EnvF:    byte %s\n", join(f.EnvF))
}

func join(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(s, ",")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "drumfit:", err)
		os.Exit(1)
	}
}
