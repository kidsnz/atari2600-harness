// Command audioingest reads a reference recording and prints the numbers needed to
// reproduce its bassline on the TIA: tempo, the sixteenth grid, and the pitch in each
// step already mapped onto the hardware's own uneven pitch ladder.
//
// It is the audio counterpart of analyze_image. Everything else here compares a ROM
// against something; this is the only path that goes from a recording TOWARDS a ROM.
//
//	# 16-bit PCM only -- convert first, so no half-written decoder can mis-read a file
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	audioingest -wav track.wav -from 96 -bars 2 -emit pattern-bassline
//
// -from picks where the loop starts, in seconds. Pick it by ear (or from a DAW) and
// pass it: finding "the main part" is a musical judgement, not a signal-processing one,
// and a tool that guessed would be guessing about the part that matters most.
//
// READ THE CONFIDENCE AND CENTS COLUMNS. A step with low confidence is the analyser
// saying it could not hear a bass note there, and a large cents figure is the TIA
// saying it cannot play the one that is there. Both are findings, not noise.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

var names = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func noteName(f float64) string {
	if f <= 0 {
		return "-"
	}
	n := 12 * math.Log2(f/440.0)
	k := int(math.Round(n))
	return fmt.Sprintf("%s%d", names[((k+9)%12+12)%12], 4+int(math.Floor(float64(k+9)/12)))
}

func main() {
	wavPath := flag.String("wav", "", "16-bit PCM WAV to analyse")
	from := flag.Float64("from", 0, "start of the loop, in seconds")
	bars := flag.Int("bars", 1, "bars to analyse from -from")
	bpmFlag := flag.Float64("bpm", 0, "tempo override; 0 = estimate it")
	waves := flag.String("waves", "1", "comma-separated AUDC values the pattern may use")
	rest := flag.Float64("rest", 0.25, "a step quieter than this fraction of the loudest is a rest")
	emit := flag.String("emit", "", "also print a DASM pattern table with this name")
	census := flag.String("census", "", "instead of reading notes, census a BAND across the whole file: \"lo-hi\" in Hz (e.g. 6000-14000). Answers whether a part EXISTS and where, before anyone reproduces it")
	per := flag.Int("per", 8, "bars per census section")
	flag.Parse()
	if *wavPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	blob, err := os.ReadFile(*wavPath)
	must(err)
	samples, rate, err := audioingest.DecodeWAV(blob)
	must(err)
	fmt.Printf("%s: %.1f s, %d Hz\n", *wavPath, float64(len(samples))/float64(rate), rate)

	bpm, strength := *bpmFlag, 1.0
	if bpm == 0 {
		env := audioingest.OnsetEnvelope(samples, rate)
		bpm, strength = audioingest.EstimateTempo(env, rate, 90, 170)
		if bpm == 0 {
			fmt.Fprintln(os.Stderr, "audioingest: no tempo could be estimated")
			os.Exit(1)
		}
		fmt.Printf("tempo: %.2f BPM (autocorrelation strength %.2f%s)\n", bpm, strength,
			map[bool]string{true: " -- WEAK, do not trust this", false: ""}[strength < 0.2])
	} else {
		fmt.Printf("tempo: %.2f BPM (given)\n", bpm)
	}

	if *census != "" {
		var lo, hi float64
		if _, err := fmt.Sscanf(*census, "%g-%g", &lo, &hi); err != nil {
			fmt.Fprintf(os.Stderr, "audioingest: -census wants \"lo-hi\" in Hz, got %q\n", *census)
			os.Exit(2)
		}
		beat := 60.0 / bpm
		c, err := audioingest.SlotCensus(samples, rate, beat, *from, [2]float64{lo, hi}, *per)
		must(err)
		fmt.Printf("\ncensus: %.0f-%.0f Hz, %d-bar sections, phase %.3f s\n", lo, hi, *per, *from)
		fmt.Printf("%-6s %7s  %s  %6s\n", "bar", "at", "0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15", "off/on")
		for _, sec := range c.Sections {
			fmt.Printf("%-6d %6.1fs ", sec.Bar0, sec.StartSec)
			for _, v := range sec.Slot {
				fmt.Printf("%4.2f ", v)
			}
			fmt.Printf(" %5.2f\n", sec.Offbeat())
		}
		on := c.Onsets()
		fmt.Printf("%-6s %7s ", "ONSET", "rise")
		for _, v := range on {
			fmt.Printf("%4.2f ", v)
		}
		fmt.Println()
		var hits []int
		for k, v := range on {
			if v >= 0.25 {
				hits = append(hits, k)
			}
		}
		fmt.Printf("       note onsets (rise >= 0.25 of the largest): %v  -- %d per bar\n", hits, len(hits))

		fmt.Printf("\nlift = offbeat eighths (2,6,10,14) vs the neighbouring sixteenths; >1.15 is a part\n")
		fmt.Printf("%s\n", c.Verdict())

		// The grid is only as good as the phase it hangs on. Check it against the drum
		// that DEFINES the phase, every time, rather than trusting -from. The first real
		// run of this census was two sixteenths out and produced a coherent, false
		// reading of the high band.
		kc, err := audioingest.SlotCensus(samples, rate, beat, *from, [2]float64{30, 60}, *per)
		if err != nil {
			fmt.Printf("\nphase check: could not measure the 30-60 Hz band (%v)\n", err)
			return
		}
		ks, conf := kc.KickSlot()
		switch {
		case ks == 0 && conf >= 1.5:
			fmt.Printf("phase check: OK -- the 30-60 Hz drum is on sixteenth 0 of the beat (%.2fx an average slot)\n", conf)
		case conf < 1.5:
			fmt.Printf("phase check: INCONCLUSIVE -- nothing in 30-60 Hz stands out (best sixteenth %d at %.2fx). "+
				"This grid's phase is unverified.\n", ks, conf)
		default:
			fmt.Printf("phase check: WRONG -- the 30-60 Hz drum is on sixteenth %d of the beat, not 0 (%.2fx an "+
				"average slot). Every slot above is rotated by %d. Re-run with -from %.6f\n",
				ks, conf, ks, *from+float64((4-ks)%4)*(beat/4))
		}
		return
	}

	var wv []int
	for _, s := range strings.Split(*waves, ",") {
		var v int
		fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
		wv = append(wv, v)
	}

	stepSec := 60.0 / bpm / 4
	steps := 16 * *bars
	notes := audioingest.BassNotes(samples, rate, *from, stepSec, steps, wv, *rest)

	fmt.Printf("\ngrid: %d steps of %.4f s from %.2f s\n", steps, stepSec, *from)
	fmt.Printf("%-5s %8s %-5s %5s   %-6s %-5s %8s %7s\n",
		"step", "Hz", "note", "conf", "AUDC", "AUDF", "TIA Hz", "cents")
	for _, n := range notes {
		if n.Hz == 0 {
			fmt.Printf("%-5d %8s %-5s %5s   %-6s %-5s %8s %7s\n", n.Step, "-", "rest", "-", "-", "-", "-", "-")
			continue
		}
		flag := ""
		if n.Confidence < 0.3 {
			flag = "  <- low confidence"
		}
		if math.Abs(n.Cents) > 25 {
			flag += "  <- the TIA cannot play this note"
		}
		fmt.Printf("%-5d %8.2f %-5s %5.2f   %-6d %-5d %8.2f %+7.1f%s\n",
			n.Step, n.Hz, noteName(n.Hz), n.Confidence, n.AUDC, n.AUDF,
			audio.Freq(n.AUDC, n.AUDF, audio.BaseClockNTSC), n.Cents, flag)
	}

	// The tempo the ROM will actually run at, which is not the tempo of the record:
	// a slot length is a whole number of frames, so the engine spreads the remainder
	// across the four sixteenths of a beat.
	beatFrames := 59.92 * 60 / bpm
	base := int(beatFrames / 4)
	long := int(math.Round(beatFrames)) - base*4
	fmt.Printf("\nframe grid for %.2f BPM: beat = %.2f frames -> sixteenths of ", bpm, beatFrames)
	for i := 0; i < 4; i++ {
		if i >= 4-long {
			fmt.Printf("%d ", base+1)
		} else {
			fmt.Printf("%d ", base)
		}
	}
	fmt.Printf("= %d frames/beat = %.2f BPM\n", base*4+long, 59.92*60/float64(base*4+long))

	if *emit != "" {
		fmt.Printf("\n; ---- src/%s.asm ----\n", *emit)
		fmt.Printf("; GENERATED by cmd/audioingest from %s at %.2f s, %.2f BPM.\n", *wavPath, *from, bpm)
		fmt.Printf("        align 16\nBassTab:")
		for i, n := range notes {
			if i%16 == 0 && i > 0 {
				fmt.Printf("\n        byte ")
			} else if i == 0 {
				fmt.Printf(" byte ")
			} else {
				fmt.Printf(",")
			}
			if n.AUDF < 0 {
				fmt.Printf("$FF")
			} else {
				fmt.Printf("%3d", n.AUDF)
			}
		}
		fmt.Println()
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "audioingest:", err)
		os.Exit(1)
	}
}
