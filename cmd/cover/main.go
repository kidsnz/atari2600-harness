// Command cover reports PC/branch coverage for a ROM: how many distinct
// instructions were executed, how many branch edges (taken / fall-through) were
// exercised, and which branches are ONE-SIDED (only one direction ever taken =
// the other side is untested). It is the test-adequacy axis for VV-3 — "did the
// run actually reach the code it claims to cover" — and the substrate the
// coverage-guided fuzzer reuses.
//
//	go run ./cmd/cover -rom x.bin -frames 120
//	go run ./cmd/cover -rom x.bin -frames 600 -inputs left,right,fire
//
// Prints JSON. With -inputs, a fixed action is held each frame (a cheap way to
// drive past a title screen); for real input timelines use a scenario.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

type report struct {
	Rom        string `json:"rom"`
	Frames     int    `json:"frames"`
	PCExecuted int    `json:"pc_executed"` // distinct instruction addresses reached

	// BranchesStatic is every branch the control-flow graph reaches from the
	// reset/NMI/IRQ vectors — the branches the PROGRAM has. BranchesObserved is
	// the ones this run executed.
	//
	// The distinction is the whole point of this report. Dividing by what was
	// observed makes a branch that was never reached vanish from the arithmetic,
	// so the percentage RISES as the test gets worse. Measured on this repo's own
	// kernels: divtable reported 100% edge coverage with 12 of its 17 branches
	// never executed; maze, hscroll and multicolor48 also read 100% with a third
	// of their branches unreached.
	BranchesStatic   int `json:"branches_static"`
	BranchesObserved int `json:"branches_observed"`
	EdgesExercised   int `json:"edges_exercised"`

	// EdgeCoverage divides by the static denominator. EdgeCoverageObserved is the
	// old, flattering number, kept alongside so the difference is visible rather
	// than silently corrected.
	EdgeCoverage         float64 `json:"edge_coverage"`
	EdgeCoverageObserved float64 `json:"edge_coverage_observed"`

	// UnreachedBranches are branches the program has and this run never executed —
	// the actionable half of the report.
	UnreachedBranches []string `json:"unreached_branches"`
	OneSided          []string `json:"one_sided_branches"` // executed, but only one edge taken

	// ExecutedButUndecoded are branches the machine ran that the decoder never
	// reached: flow from the vectors cannot follow a bank switch or a computed
	// dispatch. While this is non-empty the static denominator is too small and
	// the coverage figure is an over-estimate, which DecoderIncomplete says out
	// loud rather than leaving to be inferred from a percentage above 100.
	ExecutedButUndecoded []string `json:"executed_but_undecoded,omitempty"`
	DecoderIncomplete    bool     `json:"decoder_incomplete"`
	Note                 string   `json:"note,omitempty"`

	// Drive records HOW the ROM was driven. A coverage percentage is not a property
	// of a ROM, it is a property of a ROM and a driving, and two numbers taken under
	// different drivings are not comparable — so the driving travels with the number.
	Drive string `json:"drive"`
}

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	frames := flag.Int("frames", 120, "frames to run")
	warmup := flag.Int("warmup", 2, "frames to settle before recording")
	inputs := flag.String("inputs", "", "comma-separated actions held every frame (e.g. left,fire)")
	drive := flag.String("drive", "hold", "how to drive the ROM: 'hold' (keep -inputs pressed every frame) or "+
		"'explore' (cycle SELECT through the game variations, press RESET, then rotate the stick) — measured "+
		"2026-07-30 on commercial cartridges, explore reaches code hold never does: Seaquest 51%->60%, "+
		"Adventure 61%->67%, Chopper Command 46%->49%")
	flag.Parse()
	if *drive != "hold" && *drive != "explore" {
		fail(fmt.Errorf("-drive %q: want 'hold' or 'explore'", *drive))
	}

	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: cover -rom x.bin [-frames 120] [-warmup 2] [-inputs left,fire]")
		os.Exit(2)
	}

	e, err := emu.New("NTSC")
	if err != nil {
		fail(err)
	}
	if err := e.LoadROM(*rom); err != nil {
		fail(fmt.Errorf("rom: %w", err))
	}
	if *warmup > 0 {
		if err := e.RunFrames(*warmup); err != nil {
			fail(fmt.Errorf("warmup: %w", err))
		}
	}

	// Record only after warmup so boot/title code isn't conflated with play.
	e.EnableCoverage()
	var held []string
	if *inputs != "" {
		held = strings.Split(*inputs, ",")
	}
	// A 2600 attract mode runs the GAME LOOP with synthetic input, so simply letting a
	// cartridge sit there already covers most of what playing it covers — measured, RESET
	// plus a dozen rounds of stick moved Chopper Command by 4 instructions out of 2358.
	// What it does not cover are the other game VARIATIONS, and those are behind SELECT.
	// Hence two modes, and the mode is reported, because a coverage percentage taken under
	// one driving is not comparable with one taken under another.
	if err := driveROM(e, *drive, *frames, held); err != nil {
		fail(err)
	}

	cov := e.Coverage()
	one := cov.OneSidedBranches()
	oneHex := make([]string, len(one))
	for i, a := range one {
		oneHex[i] = fmt.Sprintf("0x%04X", a)
	}

	staticBr, banked, sErr := cyclebound.StaticBranches(*rom)
	staticSet := map[uint16]bool{}
	for _, a := range staticBr {
		staticSet[a] = true
	}
	var unreached []string
	for _, a := range staticBr {
		if !cov.Seen(a) {
			unreached = append(unreached, fmt.Sprintf("0x%04X", a))
		}
	}
	var undecoded []string
	for _, a := range cov.BranchAddrs() {
		if !staticSet[a] {
			undecoded = append(undecoded, fmt.Sprintf("0x%04X", a))
		}
	}

	obsCov := 0.0
	if cov.BranchCount() > 0 {
		obsCov = float64(cov.EdgeCount()) / float64(cov.BranchCount()*2)
	}
	statCov := 0.0
	if len(staticBr) > 0 {
		statCov = float64(cov.EdgeCount()) / float64(len(staticBr)*2)
	}
	rep := report{
		Rom:                  *rom,
		Frames:               *frames,
		Drive:                *drive,
		PCExecuted:           cov.PCCount(),
		BranchesStatic:       len(staticBr),
		BranchesObserved:     cov.BranchCount(),
		EdgesExercised:       cov.EdgeCount(),
		EdgeCoverage:         statCov,
		EdgeCoverageObserved: obsCov,
		UnreachedBranches:    unreached,
		OneSided:             oneHex,
		ExecutedButUndecoded: undecoded,
		DecoderIncomplete:    banked || len(undecoded) > 0 || sErr != nil,
	}
	switch {
	case sErr != nil:
		rep.Note = "the cartridge could not be decoded (" + sErr.Error() + "); edge_coverage has no denominator"
	case banked:
		rep.Note = "bank-switched image: a flat decode does not describe it, so branches_static is incomplete and edge_coverage is an over-estimate"
	case len(undecoded) > 0:
		rep.Note = "some executed branches were never decoded (flow from the vectors cannot follow a bank switch or a computed dispatch), so branches_static is too small and edge_coverage is an over-estimate"
	}
	if *warmup > 0 {
		rep.Note = strings.TrimSpace(rep.Note + " | recording starts after " + fmt.Sprint(*warmup) +
			" warm-up frame(s), so branches only reached during boot count as unreached")
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(b))
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(2)
}

// driveROM runs the ROM for `frames` frames under one of two drivings.
//
// "hold" keeps `held` pressed every frame — what this command always did. "explore"
// walks the game VARIATIONS with SELECT, presses RESET, then rotates the stick.
//
// The split exists because of a measurement, not a hunch. A 2600 attract mode runs the
// GAME LOOP with synthetic input, so a cartridge left alone already covers most of what
// playing it covers: on Chopper Command, RESET plus a dozen rounds of stick moved the
// executed-instruction count by 4 out of 2358. What sitting there does NOT cover are the
// other game variations, and those are behind SELECT — Seaquest goes 51% -> 60% of its
// decoded instructions, Adventure 61% -> 67%, Chopper Command 46% -> 49%.
//
// Panel switches are SetPanel, not SetInput; SetInput rejects them, and ignoring that
// error is how an earlier measurement "pressed" RESET without pressing anything.
func driveROM(e *emu.Emu, mode string, frames int, held []string) error {
	press := func(sw string, hold, settle int) error {
		if err := e.SetPanel(sw, true); err != nil {
			return fmt.Errorf("panel %q: %w", sw, err)
		}
		if err := e.RunFrames(hold); err != nil {
			return err
		}
		if err := e.SetPanel(sw, false); err != nil {
			return fmt.Errorf("panel %q release: %w", sw, err)
		}
		return e.RunFrames(settle)
	}

	if mode != "explore" {
		for f := 0; f < frames; f++ {
			for _, a := range held {
				if err := e.SetInput(0, strings.TrimSpace(a), true); err != nil {
					return fmt.Errorf("input %q: %w", a, err)
				}
			}
			if err := e.RunFrames(1); err != nil {
				return err
			}
		}
		return nil
	}

	budget := frames
	for i := 0; i < 12 && budget > 12; i++ {
		if err := press("select", 4, 6); err != nil {
			return err
		}
		budget -= 10
	}
	if budget > 12 {
		if err := press("reset", 4, 6); err != nil {
			return err
		}
		budget -= 10
	}
	acts := []string{"left", "right", "up", "down", "fire"}
	for i := 0; budget > 0; i++ {
		a := acts[i%len(acts)]
		if err := e.SetInput(0, a, true); err != nil {
			return fmt.Errorf("input %q: %w", a, err)
		}
		n := 4
		if n > budget {
			n = budget
		}
		if err := e.RunFrames(n); err != nil {
			return err
		}
		if err := e.SetInput(0, a, false); err != nil {
			return fmt.Errorf("input %q release: %w", a, err)
		}
		budget -= n
	}
	return nil
}
