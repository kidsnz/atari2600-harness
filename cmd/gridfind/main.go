// Command gridfind measures the things every other audio tool here takes as INPUTS: where the
// music starts, how fast it is, and how long the repeating unit is.
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	gridfind -wav track.wav
//
// WHY IT EXISTS. `audioingest` takes `-from` and `drumfit` takes `-t0`, and drumfit's own
// documentation says to read that value off audioingest — so a single wrong start propagates
// through the entire chain with nothing anywhere to catch it. Measured cost: two delivered
// mp3s each carried about 233 ms of digital silence before the first sample. A grid built
// without accounting for it sat two sixteenths out of phase for two days, which turned
// four-on-the-floor into "the bass is on the offbeat" and made every note reading coherent and
// wrong. That is the most expensive single error on the project this came from, and nothing in
// the toolchain could have found it.
//
// WHAT IT PRINTS, and what to do with each line:
//   - LEADING SILENCE. Measured relative to the file's own peak, because an absolute threshold
//     cannot tell a quiet recording from a silent lead-in. If this is more than a few
//     milliseconds, the file does NOT start at the music and any grid anchored at zero is wrong.
//   - TEMPO, by onset-flux autocorrelation, with its strength. Low strength means it could not
//     find a beat, which is a finding and not a number to use.
//   - T0, the first downbeat: the beat phase, resolved against the leading silence.
//   - PATTERN LENGTH in bars, with the per-period correlations so the margin is visible rather
//     than trusted. "Reproduce the first bar" and "reproduce the loop" are the same request
//     only when the loop is one bar long.
//
// It reports; it does not write a pattern. Feed the numbers to audioingest and drumfit.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
)

func main() {
	wav := flag.String("wav", "", "16-bit PCM WAV to measure (required)")
	minBPM := flag.Float64("minbpm", 80, "slowest tempo to consider")
	maxBPM := flag.Float64("maxbpm", 180, "fastest tempo to consider")
	bpmOverride := flag.Float64("bpm", 0, "skip the tempo estimate and use this")
	maxBars := flag.Int("maxbars", 8, "longest repeating unit to test for")
	frac := flag.Float64("silence", 0.02, "leading silence threshold as a fraction of peak RMS")
	band := flag.String("band", "", "measure the pattern length in this band only, lo,hi in Hz — see the note it prints")
	flag.Parse()

	if *wav == "" {
		fmt.Fprintln(os.Stderr, "-wav is required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*wav)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	samples, rate, err := audioingest.DecodeWAV(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	dur := float64(len(samples)) / float64(rate)
	fmt.Printf("%s  %.2f s, %d Hz\n\n", *wav, dur, rate)

	lead := audioingest.LeadingSilence(samples, rate, *frac)
	fmt.Printf("  leading silence   %.4f s  (%.0f ms)\n", lead, lead*1000)
	if lead > 0.005 {
		fmt.Printf("                    ** the file does NOT start at the music. A grid anchored\n")
		fmt.Printf("                       at zero is %.0f ms out before anything else is measured.\n", lead*1000)
	}

	env := audioingest.OnsetEnvelope(samples, rate)
	bpm, strength := audioingest.EstimateTempo(env, rate, *minBPM, *maxBPM)
	if *bpmOverride > 0 {
		fmt.Printf("  tempo             %.2f BPM  (given, not measured; the estimate was %.2f at strength %.2f)\n",
			*bpmOverride, bpm, strength)
		bpm = *bpmOverride
	} else {
		fmt.Printf("  tempo             %.2f BPM  strength %.2f", bpm, strength)
		if strength < 0.2 {
			fmt.Printf("  ** too weak to use — pass -bpm")
		}
		fmt.Println()
	}
	if bpm <= 0 {
		fmt.Fprintln(os.Stderr, "\nno tempo; nothing further can be measured")
		os.Exit(1)
	}
	beat := 60 / bpm
	bar := beat * 4
	fmt.Printf("  beat / bar        %.5f s / %.5f s\n", beat, bar)

	// T0 is the first HIT after the silence, not the beat phase. Phase is known only modulo a
	// beat, so it says where the beats are and not which one opens the bar; measured on real
	// material the two disagreed by most of a beat and the phase was the one that was wrong.
	// The phase is kept as the check on the hit rather than as the answer.
	t0 := audioingest.FirstOnset(env, rate, lead, beat)
	phase := audioingest.BeatPhase(env, rate, beat)
	fmt.Printf("  T0 (first hit)    %.4f s\n", t0)
	off := math.Mod(t0-phase, beat)
	if off < 0 {
		off += beat
	}
	if off > beat/2 {
		off -= beat
	}
	fmt.Printf("  beat phase        %.4f s  — the first hit sits %+.0f ms off the beat grid",
		phase, off*1000)
	switch {
	case math.Abs(off) < 0.02:
		fmt.Println("  (agrees)")
	case math.Abs(off) < 0.06:
		fmt.Println("  (close; likely the attack)")
	default:
		fmt.Println("  ** THEY DISAGREE — check by ear before building a grid on either")
	}

	pat := samples
	bandNote := "full band"
	if *band != "" {
		var lo, hi float64
		if _, err := fmt.Sscanf(*band, "%f,%f", &lo, &hi); err != nil || lo <= 0 || hi <= lo {
			fmt.Fprintf(os.Stderr, "-band: want lo,hi in Hz — got %q\n", *band)
			os.Exit(2)
		}
		pat = audioingest.BandPass(samples, rate, lo, hi)
		bandNote = fmt.Sprintf("%.0f-%.0f Hz", lo, hi)
	}
	bars, scores := audioingest.PatternBars(pat, rate, t0, bar, *maxBars)
	fmt.Printf("\n  pattern measured in: %s\n", bandNote)
	fmt.Printf("  pattern length    %d bar(s)\n", bars)
	if len(scores) > 0 {
		fmt.Printf("  per-period correlation (how alike bars N apart are):\n")
		for p, s := range scores {
			mark := ""
			if p+1 == bars {
				mark = "  <-"
			}
			fmt.Printf("      %d bar apart   %+.3f%s\n", p+1, s, mark)
		}
		fmt.Println("  The answer is the SMALLEST period within 0.02 of the best score: a two-bar")
		fmt.Println("  pattern also correlates well at four and eight, and saying eight would be")
		fmt.Println("  true and useless.")
		if *band == "" {
			fmt.Println()
			fmt.Println("  ** THIS IS THE WHOLE MIX, so it is the DRUMS' pattern length: they carry")
			fmt.Println("     most of the energy in every sixteenth. Parts can and do differ — on the")
			fmt.Println("     material this was written for the drums repeat every bar and the lead")
			fmt.Println("     every two. Re-run with -band around the part you are reproducing.")
		}
	}

	fmt.Printf("\n  feed these on: audioingest -wav %s -from %.4f -bars %d\n", *wav, t0, bars)
	fmt.Printf("                 drumfit    -wav %s -t0 %.4f -bpm %.2f\n", *wav, t0, bpm)
}
