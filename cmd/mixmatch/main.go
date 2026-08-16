// Command mixmatch measures how far a ROM's spectral BALANCE is from the record's, band
// by band, so "the melody is too heavy" becomes a number a volume can be chosen against.
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le ref.wav
//	mixmatch -ref ref.wav -rom roms/260809_technojacket/build/cover-bl-3.asm
//	mixmatch -ref ref.wav -got mine.wav          ; two recordings, no emulator
//
// The TIA gives four bits of volume and no EQ, so the only lever an author has is which
// integer goes in which AUDV. That makes "which band is too loud, and by how much" the
// whole question — and it is answerable only in dB, not by ear on a laptop speaker.
//
// Everything is measured RELATIVE to one band (-refband), because absolute level depends
// on the capture and says nothing: a mix that is uniformly quiet is not wrong.
//
// WHY THIS FILE EXISTS. internal/mixmatch is 280 lines with tests and had no command and
// no importer — nothing could reach it, while harness/CLAUDE.md described it as one of
// three pillars of audio reproduction. Measured 2026-08-15: 2 of 34 internal packages were
// unreachable, and this was the other one. `cmd/drumfit` is the sibling this follows.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/internal/audiospec"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/mixmatch"
)

const tiaRate = 31400.0 // NTSC TIA sample rate, ~2 samples per scanline

// captureROM assembles (if given an .asm) and runs a ROM headless, returning its mixed
// audio. Both channels are summed: a value-only look at channel 0 was how golden_audio
// missed half of every piece for seven scenarios (CHANGELOG, golden_mix).
func captureROM(path, spec string, frames int) ([]float64, float64, error) {
	bin := path
	if strings.HasSuffix(path, ".asm") {
		bin = strings.TrimSuffix(path, ".asm") + ".bin"
		if out, err := build.Assemble(path, bin); err != nil {
			return nil, 0, fmt.Errorf("assemble %s: %v\n%s", path, err, out)
		}
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, 0, err
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, 0, err
	}
	if err := e.EnableAudioCapture(); err != nil {
		return nil, 0, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, 0, err
	}
	ch0, ch1 := e.AudioSamples()
	mixed := make([]float64, len(ch0))
	for i := range ch0 {
		v := float64(ch0[i])
		if i < len(ch1) {
			v += float64(ch1[i])
		}
		mixed[i] = v
	}
	return audiospec.ToFloat(toU8(mixed)), tiaRate, nil
}

func toU8(x []float64) []uint8 {
	out := make([]uint8, len(x))
	for i, v := range x {
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		out[i] = uint8(v)
	}
	return out
}

func loadWAV(path string) ([]float64, float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	s, rate, err := audioingest.DecodeWAV(raw)
	if err != nil {
		return nil, 0, err
	}
	return s, float64(rate), nil
}

func parseBands(s string) ([]mixmatch.Band, error) {
	if s == "" {
		return mixmatch.Default(), nil
	}
	var out []mixmatch.Band
	for _, part := range strings.Split(s, ";") {
		f := strings.Split(strings.TrimSpace(part), ",")
		if len(f) != 3 {
			return nil, fmt.Errorf("band %q: want name,lo,hi", part)
		}
		lo, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			return nil, err
		}
		hi, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, mixmatch.Band{Name: f[0], Lo: lo, Hi: hi})
	}
	return out, nil
}

func main() {
	ref := flag.String("ref", "", "the reference recording, 16-bit PCM WAV (required)")
	rom := flag.String("rom", "", "the candidate as a ROM (.asm is assembled first, .bin loaded as-is)")
	got := flag.String("got", "", "the candidate as a WAV instead of a ROM")
	spec := flag.String("spec", "NTSC", "TV spec for -rom")
	frames := flag.Int("frames", 600, "frames to capture for -rom")
	bandsS := flag.String("bands", "", "bands as name,lo,hi;name,lo,hi (default: the built-in kick/bass/mid/air split)")
	refBand := flag.String("refband", "", "the band everything is measured against (default: the first band)")
	asJSON := flag.Bool("json", false, "emit JSON instead of a table")
	flag.Parse()

	if *ref == "" || (*rom == "" && *got == "") {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "\n-ref plus one of -rom / -got is required.")
		os.Exit(2)
	}

	bands, err := parseBands(*bandsS)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	rb := *refBand
	if rb == "" {
		rb = bands[0].Name
	}

	refX, refRate, err := loadWAV(*ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reference:", err)
		os.Exit(2)
	}
	refP, err := mixmatch.Measure(refX, refRate, bands, rb)
	if err != nil {
		fmt.Fprintln(os.Stderr, "measure reference:", err)
		os.Exit(2)
	}

	var gotX []float64
	var gotRate float64
	if *rom != "" {
		gotX, gotRate, err = captureROM(*rom, *spec, *frames)
	} else {
		gotX, gotRate, err = loadWAV(*got)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "candidate:", err)
		os.Exit(2)
	}
	gotP, err := mixmatch.Measure(gotX, gotRate, bands, rb)
	if err != nil {
		fmt.Fprintln(os.Stderr, "measure candidate:", err)
		os.Exit(2)
	}

	errs := mixmatch.Compare(refP, gotP)
	score := mixmatch.Score(errs, nil)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Ref   *mixmatch.Profile `json:"reference"`
			Got   *mixmatch.Profile `json:"candidate"`
			Errs  []mixmatch.Error  `json:"errors"`
			Score float64           `json:"score"`
		}{refP, gotP, errs, score})
		return
	}

	fmt.Printf("balance against %s, everything relative to the %q band\n", *ref, rb)
	fmt.Printf("  %-10s %10s %10s %10s\n", "band", "reference", "candidate", "error")
	for _, e := range errs {
		fmt.Printf("  %-10s %+9.1f %+9.1f %+9.1f dB\n", e.Band, refP.DB[e.Band], gotP.DB[e.Band], e.DB)
	}
	fmt.Printf("\nscore %.2f dB (mean absolute band error; 0 = same balance)\n", score)
	fmt.Println("Positive error = the candidate is LOUDER than the record in that band.")
	fmt.Println("The only lever is which integer goes in which AUDV — there is no EQ on this machine.")
}
