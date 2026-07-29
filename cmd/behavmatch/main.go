// behavmatch — behavioral reproduction diff (Tool B core).
//
// Drives a TARGET ROM and YOUR build through identical scripted input scenarios,
// records every object's per-frame trajectory, and reports where a MECHANIC
// differs (movement speed, clamp range, fire→freeze coupling) as numbers — the
// behavioural sibling of `vismatch`.
//
//	go run ./cmd/behavmatch -target Outlaw.bin -mine build/outlaw.asm -scenario all
//
// Exit 1 when any scenario's behaviour differs; 0 on match.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/build"
)

func resolve(path string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".asm") {
		return path, nil
	}
	bin := build.BinPathFor(path)
	if out, err := build.Assemble(path, bin); err != nil {
		return "", fmt.Errorf("assemble %s failed:\n%s", path, out)
	}
	return bin, nil
}

func main() {
	target := flag.String("target", "", "reference ROM .bin or .asm (required)")
	mine := flag.String("mine", "", "your build ROM .bin or .asm (required)")
	spec := flag.String("spec", "NTSC", "TV spec")
	scenario := flag.String("scenario", "all", "scenario name or 'all'")
	tWarmup := flag.Int("target-warmup", 0, "frames to run the TARGET (no input) before the scenario — skip a title screen that auto-advances to gameplay")
	mWarmup := flag.Int("mine-warmup", 0, "frames to run YOUR build (no input) before the scenario")
	tol := flag.Float64("tol", 0.6, "px tolerance for 'same' on speed/range")
	dump := flag.Bool("dump", false, "also print each traced object's raw X series")
	ramGate := flag.Bool("ram-gate", false, "also gate on RAM state: report the first frame+address where the build's RAM stops matching the target's")
	scenFile := flag.String("scenarios", "", "load scenarios from a .json file or a directory of them, instead of the built-in library — a scenario is an input script and a list of objects to watch, so a game can carry its own next to its source")
	exportScen := flag.Bool("export-scenarios", false, "print the built-in library as JSON and exit — the starting point for a new game, so the file format need not be derived from the parser")
	ramMask := flag.String("ram-mask", "live", "which bytes the RAM gate compares: 'live' (bytes the target exercised, minus the stack's reach) or 'full' (all 128)")
	flag.Parse()
	if *exportScen {
		b, err := behavmatch.ExportBuiltins()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println(string(b))
		os.Exit(0)
	}

	if *target == "" || *mine == "" {
		fmt.Fprintln(os.Stderr, "usage: behavmatch -target ref.bin -mine build.asm [-scenario all]")
		fmt.Fprintln(os.Stderr, "scenarios:", strings.Join(behavmatch.ScenarioNames(), ", "))
		os.Exit(2)
	}
	tBin, err := resolve(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	mBin, err := resolve(*mine)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	library := behavmatch.Library
	names := behavmatch.ScenarioNames()
	if *scenFile != "" {
		lib, order, err := behavmatch.LoadScenarios(*scenFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "scenarios:", err)
			os.Exit(2)
		}
		library, names = lib, order
	}
	if *scenario != "all" {
		if _, ok := library[*scenario]; !ok {
			fmt.Fprintln(os.Stderr, "unknown scenario:", *scenario)
			os.Exit(2)
		}
		names = []string{*scenario}
	}

	allMatch := true
	for _, name := range names {
		scn := library[name]
		tt, err := behavmatch.Record(tBin, *spec, scn, *tWarmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, name, "target:", err)
			os.Exit(2)
		}
		mt, err := behavmatch.Record(mBin, *spec, scn, *mWarmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, name, "mine:", err)
			os.Exit(2)
		}
		d := behavmatch.CompareTraces(tt, mt, scn.Objects, *tol)
		fmt.Printf("== scenario %s ==\n", name)
		for _, l := range d.Lines {
			fmt.Println(l)
		}
		// Freeze coupling for the fire scenario.
		if name == "p0-fire-freeze" {
			tb, tf, tm2 := behavmatch.FreezeReport(tt, 0, 1)
			mb, mf, mm2 := behavmatch.FreezeReport(mt, 0, 1)
			fmt.Printf("  freeze: target[bulletFrames=%d frozen=%d moved=%d]  mine[bulletFrames=%d frozen=%d moved=%d]\n",
				tb, tf, tm2, mb, mf, mm2)
			// Coupling verdict: shooter frozen most frames the bullet is out
			// (the "no Getaway" rule). Threshold 0.7 admits the small tail where
			// the shooter takes its first post-fire step as the bullet expires.
			couplingOK := func(b, f int) bool { return b > 0 && float64(f) >= 0.7*float64(b) }
			if couplingOK(tb, tf) != couplingOK(mb, mf) {
				fmt.Println("  **DIFF** freeze coupling (frozen-while-bullet) differs")
				d.Match = false
			}
		}
		if *ramGate {
			// The mask comes from the TARGET's own recording — which bytes it
			// exercised, and how far its stack reached — so it is measured rather
			// than declared. GateRAM prints what it excluded either way.
			mask := behavmatch.LiveMask(tt)
			if *ramMask == "full" {
				mask = behavmatch.FullMask()
			}
			g := behavmatch.GateRAM(tt, mt, mask)
			fmt.Print(g.String())
			if !g.Pass() {
				d.Match = false
			}
		}
		if *dump {
			for _, idx := range scn.Objects {
				fmt.Printf("  target %s.X: %s\n", []string{"P0", "M0", "P1", "M1", "BL"}[idx], behavmatch.FormatTraceX(tt, idx))
				fmt.Printf("  mine   %s.X: %s\n", []string{"P0", "M0", "P1", "M1", "BL"}[idx], behavmatch.FormatTraceX(mt, idx))
			}
		}
		if d.Match {
			fmt.Println("  RESULT: behaviour matches")
		} else {
			fmt.Println("  RESULT: behaviour differs")
			allMatch = false
		}
	}
	if allMatch {
		os.Exit(0)
	}
	os.Exit(1)
}
