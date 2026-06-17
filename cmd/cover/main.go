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

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

type report struct {
	Rom            string   `json:"rom"`
	Frames         int      `json:"frames"`
	PCExecuted     int      `json:"pc_executed"`      // distinct instruction addresses reached
	Branches       int      `json:"branches"`         // distinct branch instructions observed
	EdgesExercised int      `json:"edges_exercised"`  // taken+fall-through edges hit (max = branches*2)
	EdgeCoverage   float64  `json:"edge_coverage"`    // edges_exercised / (branches*2)
	OneSided       []string `json:"one_sided_branches"` // branch PCs (hex) with only one edge taken
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
	edgeCov := 0.0
	if cov.BranchCount() > 0 {
		edgeCov = float64(cov.EdgeCount()) / float64(cov.BranchCount()*2)
	}
	rep := report{
		Rom:            *rom,
		Frames:         *frames,
		PCExecuted:     cov.PCCount(),
		Branches:       cov.BranchCount(),
		EdgesExercised: cov.EdgeCount(),
		EdgeCoverage:   edgeCov,
		OneSided:       oneHex,
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(b))
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(2)
}
