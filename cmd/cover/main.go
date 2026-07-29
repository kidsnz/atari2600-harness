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
}

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	frames := flag.Int("frames", 120, "frames to run")
	warmup := flag.Int("warmup", 2, "frames to settle before recording")
	inputs := flag.String("inputs", "", "comma-separated actions held every frame (e.g. left,fire)")
	flag.Parse()

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
	for f := 0; f < *frames; f++ {
		for _, a := range held {
			if err := e.SetInput(0, strings.TrimSpace(a), true); err != nil {
				fail(fmt.Errorf("input %q: %w", a, err))
			}
		}
		if err := e.RunFrames(1); err != nil {
			fail(err)
		}
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
