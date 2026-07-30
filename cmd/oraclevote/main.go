// Command oraclevote runs a ROM on every available independent oracle
// (Gopher2600 always; MAME if installed; perfect6502 later via VV-7) from
// power-on for N frames, then reports a majority RAM verdict and names any
// dissenters. This surfaces the suite's reason to exist: "all software agrees
// but the hardware-grade member dissents." Exit 1 on dissent or no majority.
//
//	go run ./cmd/oraclevote -rom x.bin -frames 10
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/oracle"
)

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	frames := flag.Int("frames", 10, "frames from power-on to compare at")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: oraclevote -rom x.bin [-frames 10]")
		os.Exit(2)
	}

	oracles := []oracle.Oracle{oracle.Gopher{}}
	if oracle.MameAvailable() {
		oracles = append(oracles, oracle.Mame{})
	}
	names := make([]string, len(oracles))
	for i, o := range oracles {
		names[i] = o.Name()
	}

	maj, dissenters, ok, err := oracle.Vote(oracles, *rom, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	// A dissent is not yet a divergence. The oracles do not read RAM at the same
	// MOMENT: measured on roms/litmus/litmus_framephase.bin at frames=10, Gopher2600
	// stops just after VSYNC ($80=10 $81=9 $82=9) while MAME's frame notifier fires
	// after the midpoint of the visible field ($80=10 $81=10 $82=9). Any byte a game
	// updates between those two points differs by one on a ROM where nothing is
	// wrong. Bounding it with the reference oracle at N and N+1 separates the two,
	// and BOTH counts are printed: a phase byte is not proof of health, it is a
	// question the tool cannot answer.
	// NOTE the condition. With exactly two oracles a disagreement has no strict
	// majority, so Vote returns ok=false and an EMPTY dissenters list — the tool
	// exited 1 while reporting nothing about who differed or where. Two oracles is
	// the normal case here (Gopher + MAME), so that was the common path.
	var realDiff, phaseDiff []int
	var differed []string
	if !ok || len(dissenters) > 0 {
		refN, e1 := oracle.Gopher{}.DumpRAM(*rom, *frames)
		refNext, e2 := oracle.Gopher{}.DumpRAM(*rom, *frames+1)
		if e1 == nil && e2 == nil {
			for _, o := range oracles {
				if o.Name() == (oracle.Gopher{}).Name() {
					continue
				}
				d, e := o.DumpRAM(*rom, *frames)
				if e != nil {
					continue
				}
				r, p := oracle.ClassifyDiff(refN, refNext, d)
				if len(r) > 0 || len(p) > 0 {
					differed = append(differed, o.Name())
				}
				realDiff = append(realDiff, r...)
				phaseDiff = append(phaseDiff, p...)
			}
		}
	}

	out := struct {
		Rom        string   `json:"rom"`
		Frames     int      `json:"frames"`
		Oracles    []string `json:"oracles"`
		Majority   bool     `json:"majority"`
		Dissenters []string `json:"dissenters,omitempty"`
		Differed   []string `json:"differed_from_gopher2600,omitempty"`
		RealDiffs  []int    `json:"real_diff_offsets,omitempty"`
		PhaseDiffs []int    `json:"sampling_phase_offsets,omitempty"`
		RAM0x80    int      `json:"ram_0x80"`
	}{*rom, *frames, names, ok, dissenters, differed, realDiff, phaseDiff, int(maj[0])}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))

	if len(oracles) == 1 {
		fmt.Fprintln(os.Stderr, "note: only Gopher2600 available — install mame for an independent cross-check")
	}
	if !ok || len(dissenters) > 0 {
		who := dissenters
		if len(who) == 0 {
			who = differed // two oracles: no strict majority, so Vote names nobody
		}
		fmt.Fprintf(os.Stderr, "DISSENT: %v differ from gopher2600 "+
			"(%d offset(s) a sampling-phase shift cannot explain: %v; %d it can: %v)\n",
			who, len(realDiff), realDiff, len(phaseDiff), phaseDiff)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "AGREE: %d oracle(s) unanimous\n", len(oracles))
}
