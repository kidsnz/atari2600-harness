// Command voicefit answers "which of the TIA's waveforms sounds most like THIS?" by
// comparing harmonic series, not by ear and not by tuning.
//
//	ffmpeg -i track.mp3 -ac 1 -ar 44100 -c:a pcm_s16le track.wav
//	voicefit -wav track.wav -at 1.83 -band 85,1000
//	voicefit -harmonics 1.0,0.40,0.16,0.06,0.03      # if you already measured them
//
// WHY IT EXISTS. Choosing a voice by TUNING and choosing it by TIMBRE are different searches
// and they disagree. cmd/keyfit answers the first: which key can this machine play the figure
// in. Nothing answered the second. On the Transistor Dub work a lead was fitted by tuning
// alone, landed on AUDC 12, and the author's first words on hearing it were that the timbre
// was nothing like the record — AUDC 12 is a squarewave with NO even harmonics, and the
// record's line rolls off 1.00 .40 .16 .06, so it was the furthest of the eight from what was
// wanted. cmd/mixmatch caught the same class of error a second time, from the other end, by
// band balance. The numbers to prevent it existed all along, measured, in a _test.go file
// that no tool could import; they now live in audio.MeasuredSpectra and this reads them.
//
// WHAT IT DOES NOT DO. It ranks timbre and says nothing about pitch: the best-sounding
// waveform may not reach the note. Run keyfit for that and expect to trade. And a spectrum
// measured from a mix is the mix's, not the instrument's — band-limit to the part, and read
// the f0 warning, because a harmonic series measured on the wrong fundamental is a series of
// the wrong thing.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/audioingest"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

func main() {
	wav := flag.String("wav", "", "16-bit PCM WAV holding the reference")
	at := flag.Float64("at", 0, "seconds into the file")
	win := flag.Float64("win", 0.12, "analysis window in seconds")
	band := flag.String("band", "85,1000", "where to look for the fundamental, lo,hi in Hz")
	hs := flag.String("harmonics", "", "instead of a WAV: the reference series as a1,a2,a3,... (any scale)")
	n := flag.Int("n", 8, "harmonics to compare (audio.MeasuredSpectra holds 8)")
	flag.Parse()

	var ref []float64
	var f0 float64
	switch {
	case *hs != "":
		for _, p := range strings.Split(*hs, ",") {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "-harmonics: %v\n", err)
				os.Exit(2)
			}
			ref = append(ref, v)
		}
	case *wav != "":
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
		i0, i1 := int(*at*float64(rate)), int((*at+*win)*float64(rate))
		if i0 < 0 || i1 > len(samples) {
			fmt.Fprintf(os.Stderr, "-at %.3f +%.3fs is outside the file\n", *at, *win)
			os.Exit(2)
		}
		w := samples[i0:i1]
		d := audioingest.F0Checked(w, float64(rate), lo, hi, 0)
		if d.Hz <= 0 {
			fmt.Fprintln(os.Stderr, "no pitch found in that window")
			os.Exit(1)
		}
		f0 = d.Hz
		// The f0 warning is printed BEFORE the ranking, not after it, because a harmonic
		// series measured on the second harmonic is a series of a different thing and every
		// number below it would be wrong in a way that looks fine.
		if d.Suspect() {
			fmt.Printf("!! the fundamental is suspect: a better period sits at %.2f Hz", d.SubHz)
			if d.BelowRange {
				fmt.Printf(", BELOW your -band lo of %.0f", lo)
			}
			fmt.Println(". Run f0check before trusting the ranking below.")
			fmt.Println()
		}
		ref = audio.HarmonicsF(w, float64(rate)/d.Hz, *n)
		if ref == nil {
			fmt.Fprintln(os.Stderr, "window too short to hold a cycle at that pitch")
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "give -wav or -harmonics")
		os.Exit(2)
	}

	if f0 > 0 {
		nm, c := audio.NearestNote(f0)
		fmt.Printf("reference: %s at %.3f s, f0 %.2f Hz (%s%+.0fc)\n", *wav, *at, f0, nm, c)
	} else {
		fmt.Println("reference: given on the command line")
	}
	fmt.Printf("  %s\n", series(ref, *n))
	fmt.Println()

	type row struct {
		audc int
		d    float64
	}
	var rows []row
	for _, c := range audio.PitchedWaveforms {
		rows = append(rows, row{c, audio.SpectrumDistance(ref, audio.MeasuredSpectra[c])})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].d < rows[j].d })

	fmt.Println("  closest first — distance is RMS dB per harmonic, normalised to the fundamental")
	fmt.Printf("  %-4s %-9s %8s   %s\n", "AUDC", "name", "dist", "its own spectrum")
	for _, r := range rows {
		fmt.Printf("  %-4d %-9s %6.1f dB   %s\n", r.audc, audio.Name(r.audc), r.d,
			series(audio.MeasuredSpectra[r.audc], *n))
	}
	fmt.Println()
	fmt.Printf("  %s (AUDC %d) is the closest timbre. It may not reach the note — that is\n",
		audio.Name(rows[0].audc), rows[0].audc)
	fmt.Println("  cmd/keyfit's question, and the two answers routinely disagree.")
}

// series prints a spectrum normalised so the fundamental reads 1.00, which is how the ear
// hears it and how SpectrumDistance compares it. Printing the raw normalised-to-total figures
// would rank a flat waveform as having a quiet fundamental when it has none.
func series(a []float64, n int) string {
	if len(a) == 0 || a[0] <= 0 {
		return "(no fundamental to normalise by)"
	}
	if n > len(a) {
		n = len(a)
	}
	var b strings.Builder
	for k := 0; k < n; k++ {
		fmt.Fprintf(&b, "%5.2f ", a[k]/a[0])
	}
	return strings.TrimSpace(b.String())
}

func twoFloats(s string) (float64, float64, error) {
	p := strings.Split(s, ",")
	if len(p) != 2 {
		return 0, 0, fmt.Errorf("want lo,hi — got %q", s)
	}
	a, e1 := strconv.ParseFloat(strings.TrimSpace(p[0]), 64)
	b, e2 := strconv.ParseFloat(strings.TrimSpace(p[1]), 64)
	if e1 != nil || e2 != nil || a <= 0 || b <= a {
		return 0, 0, fmt.Errorf("want 0 < lo < hi — got %q", s)
	}
	_ = math.Abs
	return a, b, nil
}
