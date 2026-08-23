// Command f0check answers one question about a recording: is the pitch you just measured
// the FUNDAMENTAL, or a harmonic of one your analysis band excluded?
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	f0check -wav track.wav -at 1.09 -band 110,800
//
// WHY IT EXISTS. On the Transistor Dub work a lead line was measured over 110-800 Hz and
// read 194 Hz for two days. Its fundamental is 96.9 Hz — below the band — and 194 Hz was
// its second harmonic. Nothing in the toolchain could say so: cmd/audioingest takes the
// band and the grid as INPUTS and reports what it finds inside them, which is the correct
// contract and also a trap, because a band that excludes the fundamental produces a
// confident wrong answer that looks exactly like a right one. It took two independent
// analyses to catch. This makes it one command.
//
// WHAT IT REPORTS, and why both numbers are printed. Autocorrelation and an FFT peak fail
// differently, so seeing them disagree IS the finding:
//   - the autocorrelation answer (internal/audioingest.F0), which is robust to a weak or
//     missing fundamental but cannot return anything outside the band it was given;
//   - the FFT peak, which is what most eyes and most tools reach for first and is the one
//     that actually lied on this material;
//   - the harmonic ladder at the candidate, so a reader can see whether the odd multiples
//     are present — nothing can sit at 1.5x f0 if f0 really is the fundamental;
//   - whether a better period exists BELOW the band, found by searching rather than by
//     testing multiples of the answer, because a squarewave read from above its
//     fundamental leaves partials at no simple ratio to it.
//
// Exit 1 when the fundamental is suspect, so it can stand in a script or a gate.
//
// BEFORE YOU MEASURE ANYTHING: IS THIS FILE ONE SOURCE?
//
// Band-limiting selects a FREQUENCY RANGE. It does not select an INSTRUMENT. Everything the
// record has in that range stays: the kick's body, other parts' harmonics, the reverb tail.
// Measured cost of forgetting this, on the job these tools come from — a part was "isolated"
// by cutting a mix to 110-300 Hz and then measured, reproduced and verified for days. The
// goldens were green, the negative controls fired, the prover certified the kernel and the ROM
// followed the measured line to within 36.6 cents. It was still the wrong sound, and the author
// heard it in seconds.
//
// SEPARATE BY SOURCE FIRST. It costs about ten seconds and it is not an Atari problem:
//
//	python3 -m demucs -n htdemucs_ft -d mps -o stems track.wav
//	bandsplit -files stems/htdemucs_ft/track/{bass,drums,other,vocals}.wav -out /tmp/pick.html
//	open /tmp/pick.html          # and ask which one holds the part
//
// Then measure the stem, not the mix. On that job the acid line came back in `bass` (a bassline
// separator looks where the line lives, 49-116 Hz) and `other` was empty at -24.5 dB. Separating
// took the two-bar correlation from 0.681-vs-0.963 to 0.223-vs-0.979 and the per-note pitch
// confidence from a median 0.69 to 0.77.
//
// A checker for this was written and then DELETED. Its verdict flipped with the analysis band on
// the same file (a bass stem read 99.6% "one source" over 30-400 Hz and 44.1% "a mixture" over
// 60-1000), and it called another record's full mix a single source. More to the point: if the
// right move is always to separate — and at ten seconds it is — then a tool that tells you
// whether to separate has no decision to inform. The knowledge belongs here, not in a command.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

func main() {
	wav := flag.String("wav", "", "16-bit PCM WAV to read (required)")
	at := flag.Float64("at", 0, "seconds into the file to analyse")
	win := flag.Float64("win", 0.12, "analysis window in seconds")
	band := flag.String("band", "85,1000", "search range as lo,hi in Hz — the range you would have passed to any other tool")
	floor := flag.Float64("floor", 0, "how far below the band to look for a better period; 0 = lo/4")
	strict := flag.Bool("strict", false, "exit 1 when the fundamental is suspect")
	flag.Parse()

	if *wav == "" {
		fmt.Fprintln(os.Stderr, "-wav is required")
		os.Exit(2)
	}
	lo, hi, err := twoFloats(*band)
	if err != nil {
		fmt.Fprintf(os.Stderr, "-band: %v\n", err)
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
	i0 := int(*at * float64(rate))
	i1 := i0 + int(*win*float64(rate))
	if i0 < 0 || i1 > len(samples) {
		fmt.Fprintf(os.Stderr, "-at %.3f +%.3fs is outside the file (%.3fs long)\n",
			*at, *win, float64(len(samples))/float64(rate))
		os.Exit(2)
	}
	w := samples[i0:i1]

	d := audioingest.F0Checked(w, float64(rate), lo, hi, *floor)
	fmt.Printf("%s  %.3f s +%.0f ms, searching %.0f-%.0f Hz\n", *wav, *at, *win*1000, lo, hi)
	fmt.Println()
	fmt.Printf("  autocorrelation   %8.2f Hz  %-9s  confidence %.2f\n",
		d.Hz, name(d.Hz), d.Conf)
	fp := fftPeak(w, float64(rate), lo, hi)
	fmt.Printf("  FFT peak          %8.2f Hz  %-9s  %s\n", fp, name(fp), agree(d.Hz, fp))

	if d.Hz > 0 {
		fmt.Println()
		fmt.Println("  harmonic ladder at the autocorrelation answer (dB relative to it):")
		base := toneAt(w, float64(rate), d.Hz)
		for _, m := range []float64{0.5, 1, 1.5, 2, 3, 4, 5} {
			f := d.Hz * m
			if f > float64(rate)/2 {
				continue
			}
			mark := ""
			if m == 0.5 || m == 1.5 {
				mark = "   <- must be EMPTY if this is the fundamental"
			}
			fmt.Printf("    %5.1fx  %8.2f Hz  %+7.1f dB%s\n",
				m, f, 20*math.Log10(toneAt(w, float64(rate), f)/(base+1e-30)), mark)
		}
	}

	fmt.Println()
	if d.Suspect() {
		r := ""
		if d.SubRatio > 0 {
			r = fmt.Sprintf(" (%dx below)", d.SubRatio)
		}
		fmt.Printf("  ** SUSPECT ** a better period exists at %.2f Hz %s%s\n",
			d.SubHz, name(d.SubHz), r)
		fmt.Printf("     lag-fair correlation %.3f there against %.3f at %.2f Hz\n",
			d.SubNCC, d.NCC, d.Hz)
		// How LOUD the lower period is decides what to do about it, and the correlation does
		// not say. A sub an octave down at -20 dB is a sub-oscillator layer of one instrument
		// and reproducing the upper note is right; one at -3 dB is the note itself and
		// reproducing the upper note is an octave error. Both correlate well.
		sub := 20 * math.Log10(toneAt(w, float64(rate), d.SubHz)/(toneAt(w, float64(rate), d.Hz)+1e-30))
		fmt.Printf("     %.2f Hz is %+.1f dB against %.2f Hz — ", d.SubHz, sub, d.Hz)
		switch {
		case sub > -6:
			fmt.Println("as loud or louder. Treat it as the note.")
		case sub > -15:
			fmt.Println("clearly present. Decide by ear which is the note.")
		default:
			fmt.Println("quiet. Likely a sub-oscillator under the note rather than the note.")
		}
		if d.BelowRange {
			fmt.Printf("     and %.2f Hz is BELOW your -band lo of %.0f, so the range you gave\n",
				d.SubHz, lo)
			fmt.Printf("     is what hid it. Re-run with -band %.0f,%.0f\n", d.SubHz*0.85, hi)
		}
		if *strict {
			os.Exit(1)
		}
	} else {
		fmt.Printf("  clean: no better period between %.0f Hz and the bottom of the search.\n",
			floorOf(*floor, lo))
		fmt.Println("  This says the band did not hide the fundamental. It does not say the")
		fmt.Println("  window held one note — an unpitched or polyphonic window can read clean.")
	}
}

func floorOf(f, lo float64) float64 {
	if f <= 0 {
		f = lo / 4
	}
	if f < 20 {
		f = 20
	}
	return f
}

func agree(a, b float64) string {
	if a <= 0 || b <= 0 {
		return ""
	}
	c := 1200 * math.Log2(b/a)
	switch {
	case math.Abs(c) < 50:
		return "agrees with the autocorrelation"
	case math.Abs(math.Abs(c)-1200) < 60:
		return "AN OCTAVE APART — read the ladder below"
	default:
		return fmt.Sprintf("DISAGREES by %+.0f cents", c)
	}
}

// toneAt measures one frequency directly rather than reading an FFT bin, so the answer is
// not quantised by the window length — at 100 Hz in a 120 ms window a bin is most of a
// semitone wide, which is coarser than the thing being judged.
func toneAt(w []float64, rate, f float64) float64 {
	var re, im float64
	n := float64(len(w))
	for i, v := range w {
		h := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/n) // Hann
		p := 2 * math.Pi * f * float64(i) / rate
		re += v * h * math.Cos(p)
		im -= v * h * math.Sin(p)
	}
	return math.Hypot(re, im) / n
}

// fftPeak is deliberately the naive thing — the strongest frequency inside the band — because
// that is the measurement that fails, and printing it next to the robust one is the point.
func fftPeak(w []float64, rate, lo, hi float64) float64 {
	best, bf := 0.0, 0.0
	for f := lo; f <= hi; f += 0.25 {
		if m := toneAt(w, rate, f); m > best {
			best, bf = m, f
		}
	}
	return bf
}

func name(hz float64) string {
	if hz <= 0 {
		return ""
	}
	n, c := audio.NearestNote(hz)
	return fmt.Sprintf("%s%+.0fc", n, c)
}

func twoFloats(s string) (float64, float64, error) {
	p := strings.Split(s, ",")
	if len(p) != 2 {
		return 0, 0, fmt.Errorf("want lo,hi — got %q", s)
	}
	a, err := strconv.ParseFloat(strings.TrimSpace(p[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.ParseFloat(strings.TrimSpace(p[1]), 64)
	if err != nil {
		return 0, 0, err
	}
	if a <= 0 || b <= a {
		return 0, 0, fmt.Errorf("want 0 < lo < hi — got %q", s)
	}
	return a, b, nil
}
