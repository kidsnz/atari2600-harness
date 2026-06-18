// statecov — VV-11 state-coverage matrix (docs/capability-gap-audit.md).
//
// Reports which TIA MODES a ROM exercised over a multi-frame run: NUSIZ copies,
// missile/ball size, vertical-delay flags, playfield reflect/score/priority, and
// bank switches. An axis stuck at its reset value is a verification blind spot.
// This is orthogonal to PC/branch coverage (VV-3, cmd/cover).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/statecov"
)

func main() {
	rom := flag.String("rom", "", "ROM (.bin) to profile (required)")
	frames := flag.Int("frames", 8, "frames to sample")
	spec := flag.String("spec", "NTSC", "TV spec")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: statecov -rom x.bin [-frames 8]")
		os.Exit(2)
	}

	m, err := statecov.Run(*rom, *spec, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	type axisRep struct {
		Seen     []int   `json:"seen"`
		Distinct int     `json:"distinct"`
		Domain   int     `json:"domain,omitempty"`
		Coverage float64 `json:"coverage,omitempty"`
	}
	rep := map[string]axisRep{}
	for _, ax := range statecov.Axes {
		a := axisRep{Seen: m.Values(ax), Distinct: m.Count(ax)}
		if d, ok := statecov.Domain(ax); ok {
			a.Domain = d
			a.Coverage = float64(a.Distinct) / float64(d)
		}
		rep[ax] = a
	}
	out := struct {
		Rom     string             `json:"rom"`
		Frames  int                `json:"frames"`
		Samples int                `json:"samples"`
		Axes    map[string]axisRep `json:"axes"`
	}{*rom, *frames, m.Samples, rep}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
