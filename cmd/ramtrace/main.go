// ramtrace — record and describe a ROM's RAM as a per-frame time series.
//
// The measurement half of behavioural reproduction: before any of a game's logic
// can be re-authored, its state has to be observed. ramtrace drives a ROM through
// a scripted input scenario and records all 128 RAM bytes, the held input, the
// collision latches and the stack pointer on every frame — then describes what
// each byte did, without fitting anything to it.
//
//	ramtrace record   -rom Outlaw.bin -scenario p0-right -out trace.json
//	ramtrace activity -rom Outlaw.bin -scenario all
//	ramtrace arity    -rom Outlaw.bin -scenario all
//
// It is ROM-blind on purpose: the input is a .bin and an input script, never a
// symbol map or a disassembly.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/ramtrace"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage: ramtrace <record|activity|arity> -rom ROM [flags]

  record    write the full per-frame RAM time series as JSON
  activity  per-byte descriptive statistics (what each byte did)
  arity     smallest feature set that determines each byte's next value

flags:
  -rom PATH        ROM .bin or .asm (required)
  -scenario NAME   scenario name, or 'all' (default: all)
  -warmup N        frames to run with no input before the scenario
                   (a title screen that auto-advances would otherwise eat the script)
  -spec SPEC       TV spec (default NTSC)
  -out PATH        write JSON here (default: stdout text for activity/arity)
  -json            force JSON output for activity/arity

scenarios: `+strings.Join(behavmatch.ScenarioNames(), ", "))
}

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
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	rom := fs.String("rom", "", "ROM .bin or .asm (required)")
	spec := fs.String("spec", "NTSC", "TV spec")
	scenario := fs.String("scenario", "all", "scenario name, comma-separated list, or 'all'")
	exclude := fs.String("exclude", "", "comma-separated scenario names to leave out")
	warmup := fs.Int("warmup", 0, "frames with no input before the scenario")
	out := fs.String("out", "", "write JSON to this path")
	asJSON := fs.Bool("json", false, "JSON output for activity/arity")
	skip := fs.Int("skip", 0, "arity: ignore this many frames at the start of each recording (power-on initialisation is a bulk RAM clear, not a per-byte rule)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *rom == "" {
		usage()
		os.Exit(2)
	}
	bin, err := resolve(*rom)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	names := behavmatch.ScenarioNames()
	if *scenario != "all" {
		names = nil
		for _, n := range strings.Split(*scenario, ",") {
			n = strings.TrimSpace(n)
			if _, ok := behavmatch.Library[n]; !ok {
				fmt.Fprintln(os.Stderr, "unknown scenario:", n)
				fmt.Fprintln(os.Stderr, "known:", strings.Join(behavmatch.ScenarioNames(), ", "))
				os.Exit(2)
			}
			names = append(names, n)
		}
	}
	if *exclude != "" {
		drop := map[string]bool{}
		for _, n := range strings.Split(*exclude, ",") {
			drop[strings.TrimSpace(n)] = true
		}
		var kept []string
		for _, n := range names {
			if !drop[n] {
				kept = append(kept, n)
			}
		}
		names = kept
	}
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "no scenarios selected")
		os.Exit(2)
	}

	switch cmd {
	case "record":
		if len(names) != 1 {
			fmt.Fprintln(os.Stderr, "record needs a single -scenario (one trace per file)")
			os.Exit(2)
		}
		scn := behavmatch.Library[names[0]]
		tr, err := behavmatch.Record(bin, *spec, scn, *warmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		prov, err := ramtrace.NewProvenance("ramtrace record", bin, *spec, scn.Name, *warmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		doc := ramtrace.FromTrace(prov, tr, scn.Objects)
		if err := ramtrace.WriteJSON(*out, doc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if *out != "" {
			fmt.Printf("wrote %s — %s, %d frames\n", *out, scn.Name, doc.Frames)
		}

	case "activity", "arity":
		traces := map[string]*behavmatch.Trace{}
		for _, n := range names {
			tr, err := behavmatch.Record(bin, *spec, behavmatch.Library[n], *warmup)
			if err != nil {
				fmt.Fprintln(os.Stderr, n, ":", err)
				os.Exit(2)
			}
			traces[n] = tr
		}
		prov, err := ramtrace.NewProvenance("ramtrace "+cmd, bin, *spec, strings.Join(names, ","), *warmup)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		var doc any
		var text string
		if cmd == "activity" {
			r := ramtrace.Analyse(prov, traces)
			doc, text = r, r.Text()
		} else {
			ramtrace.SkipFrames = *skip
			r := ramtrace.Arity(prov, traces)
			doc, text = r, r.Text()
		}
		if *out != "" || *asJSON {
			if err := ramtrace.WriteJSON(*out, doc); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if *out != "" {
				fmt.Println("wrote", *out)
			}
			return
		}
		fmt.Print(text)

	default:
		usage()
		os.Exit(2)
	}
}
