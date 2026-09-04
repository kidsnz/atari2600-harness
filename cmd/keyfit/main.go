// Command keyfit answers the question this machine forces on anyone reproducing music:
// WHICH KEY can the TIA play this figure in, and on which waveform?
//
//	keyfit -degrees 0,3,5,7 -lo 55 -octaves 3        ; sweep tonics, report worst degree
//	keyfit -degrees 0,3,5,7 -tonic 92.5              ; one tonic, in detail
//	keyfit -intune 55,220 -tol 25                    ; every pitch inside 25 cents, as a ladder
//	keyfit -degrees 0,3,5,7 -lo 55 -octaves 3 -detune 50 -step 10
//	                                                 ; leave the semitone grid: try tonics
//	                                                   +/-50 cents off in 10-cent steps
//
// The TIA's reachable notes are a fixed, uneven ladder — a figure trivially in tune at one
// tonic can have three unusable degrees a semitone away. That is hardware, not taste, and it
// has to be measured before a note is chosen.
//
// WHY THIS FILE EXISTS AT ALL. internal/keyfit is 502 lines with tests and was written after
// the question had been hand-rolled three times for one record. It had no command and no
// importer, so nothing could reach it — while harness/CLAUDE.md described it as one of three
// pillars of audio reproduction. Measured 2026-08-15: 2 of 34 internal packages were
// unreachable, and this was one. `cmd/drumfit` is the sibling this follows.
//
// It does NOT choose. It reports the error at every tonic and leaves the musical decision —
// drop a degree, displace it an octave, accept the register change — to whoever can hear it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/keyfit"
)

// ntscClock is the TIA audio base clock (3.579545 MHz / 114). Overridable for PAL work.
const ntscClock = 3579545.0 / 114

func parseDegrees(s string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("degree %q: %w", f, err)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no degrees given")
	}
	return out, nil
}

func main() {
	degrees := flag.String("degrees", "", "semitones above the tonic that the figure uses, e.g. 0,3,5,7")
	tonic := flag.Float64("tonic", 0, "report ONE tonic in detail, in Hz (skips the sweep)")
	lo := flag.Float64("lo", 55, "sweep: lowest tonic to try, Hz")
	octaves := flag.Int("octaves", 3, "sweep: how many octaves of tonics to try")
	detune := flag.Int("detune", 0, "sweep: also try tonics up to +/-N cents off the semitone grid")
	step := flag.Int("step", 10, "sweep: cent step when -detune is used")
	intune := flag.String("intune", "", "instead: list every reachable pitch in LO,HI Hz within -tol cents of a semitone")
	tol := flag.Float64("tol", 25, "-intune: how many cents off counts as in tune")
	oneVoice := flag.Bool("one-voice", false, "rank by the best SINGLE waveform rather than best-per-degree (one voice cannot change AUDC per note)")
	top := flag.Int("top", 8, "how many tonics to print")
	clock := flag.Float64("clock", ntscClock, "TIA audio base clock (NTSC default; PAL is 3546894/114)")
	asJSON := flag.Bool("json", false, "emit JSON instead of a table")
	waves := flag.String("waves", "", "restrict to these AUDC values, e.g. 6 — the figure then uses ONE timbre")
	fine := flag.Bool("fine", false, "-degrees: sweep tonics CONTINUOUSLY between -lo and -hi instead of on the semitone grid")
	hi := flag.Float64("hi", 0, "-fine: highest tonic to try, Hz (default -lo x 2^-octaves)")
	cstep := flag.Float64("cstep", 5, "-fine: tonic resolution in cents")
	flag.Parse()

	switch {
	case *intune != "":
		var a, b float64
		if _, err := fmt.Sscanf(*intune, "%f,%f", &a, &b); err != nil {
			fmt.Fprintln(os.Stderr, "bad -intune (want LO,HI):", err)
			os.Exit(2)
		}
		p := keyfit.InTune(a, b, *tol, *clock)
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(p)
			return
		}
		fmt.Printf("pitches within %.0f cents of a semitone, %.1f-%.1f Hz: %d\n", *tol, a, b, len(p))
		fmt.Printf("  %-5s %-5s %9s %8s  %s\n", "AUDC", "AUDF", "Hz", "cents", "note")
		for _, x := range p {
			fmt.Printf("  %-5d %-5d %9.3f %+8.1f  %s\n", x.AUDC, x.AUDF, x.Hz, x.Cents, x.Note)
		}
		// A short list IS the finding: it means the register has no usable notes there.
		if len(p) == 0 {
			fmt.Println("  none — this band has no pitch the machine can play in tune. Transpose.")
		}
		return

	case *degrees == "":
		flag.Usage()
		os.Exit(2)
	}

	deg, err := parseDegrees(*degrees)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var wv []int
	if *waves != "" {
		for _, part := range strings.Split(*waves, ",") {
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				fmt.Fprintf(os.Stderr, "-waves: %v\n", err)
				os.Exit(2)
			}
			wv = append(wv, v)
		}
	}
	var fits []keyfit.Fit
	switch {
	case *tonic > 0 && wv != nil:
		fits = []keyfit.Fit{keyfit.FitTonicVoices(*tonic, deg, wv, *clock)}
	case *tonic > 0:
		fits = []keyfit.Fit{keyfit.FitTonic(*tonic, deg, *clock)}
	case *fine || wv != nil:
		// Continuous, and restricted if asked. Both are needed for the same question — "one
		// type of sound only" — because the best tonic for a SINGLE waveform is generally not
		// on the semitone grid: for AUDC 6 over {0,3,4,5,10,12,15} it is 38.41 Hz, D#1 minus 21
		// cents, which no grid sweep reaches. Before this, that fit had to be hand-rolled, and
		// it was, twice in one session.
		top := *hi
		if top <= 0 {
			top = *lo * math.Pow(2, float64(*octaves))
		}
		fits = keyfit.SweepVoices(*lo, top, *cstep, deg, wv, *clock)
	case *detune > 0:
		fits = keyfit.SweepDetuned(*lo, *detune, *step, deg, *clock)
	default:
		fits = keyfit.Sweep(*lo, *octaves, deg, *clock)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(fits)
		return
	}

	best := keyfit.Best(fits, *oneVoice)
	fmt.Printf("figure %s over %d tonic(s); ranked by %s\n", *degrees, len(fits),
		map[bool]string{true: "best SINGLE waveform", false: "best per degree"}[*oneVoice])
	fmt.Printf("  %-9s %-7s %10s %8s %8s %10s  %s\n", "tonic", "Hz", "worst", "spread", "mean", "1-voice", "detune")
	shown := 0
	// spread and mean sit next to worst deliberately. A key displaced uniformly is IN TUNE WITH
	// ITSELF and a listener hears intervals, not absolute pitch -- so ranking by `worst` alone
	// prefers a key that straddles zero over one that is uniformly sharp, which is backwards for
	// anything played on its own. See internal/keyfit's Fit.Spread for the 1998 source and the
	// worked example from this repository's own pitch-dither figures. Neither number is applied
	// automatically; a piece played against an external reference wants `worst` after all.
	for _, f := range fits {
		if shown >= *top {
			break
		}
		fmt.Printf("  %-9s %-7.2f %+9.1fc %7.1fc %+7.1fc %+9.1fc  %+.0fc\n",
			f.TonicName, f.TonicHz, f.Worst, f.Spread, f.Mean, f.OneWorst, f.Detune)
		shown++
	}
	if len(fits) > shown {
		fmt.Printf("  ... %d more (raise -top, or -json for all)\n", len(fits)-shown)
	}
	fmt.Printf("\nbest: %s (%.2f Hz), worst degree %+.1f cents, spread %.1f, mean %+.1f",
		best.TonicName, best.TonicHz, best.Worst, best.Spread, best.Mean)
	if *oneVoice {
		fmt.Printf(" on one waveform (AUDC %d, %+.1f cents)", best.OneVoice, best.OneWorst)
	}
	fmt.Println()
	fmt.Println("This reports; it does not choose. Dropping a degree, displacing it an octave or")
	fmt.Println("accepting the register change are musical decisions and belong to whoever can hear it.")
}
