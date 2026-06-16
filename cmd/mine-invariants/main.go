// Command mine-invariants discovers likely invariants from a driven run
// (Daikon-lite) and prints them as scenario invariants/monotonic fragments to
// seed a spec. docs/testing-playbook.md (Ernst et al., Daikon).
//
//	go run ./cmd/mine-invariants -rom x.bin -frames 600 -seed 7 -actions left,right,fire
//
// By default prints only "interesting" candidates (non-zero constants, monotonic,
// or non-trivial ranges); -all includes always-zero cells.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/mine"
)

func main() {
	rom := flag.String("rom", "", "ROM .bin (required)")
	frames := flag.Int("frames", 600, "frames to observe")
	seed := flag.Int64("seed", 1, "PRNG seed for the driving input")
	actionsCSV := flag.String("actions", "", "comma-separated input pool to drive with (e.g. left,right,fire); empty = free-run")
	fieldsCSV := flag.String("fields", "", "comma-separated fields to observe (default: all RAM 0x80..0xFF)")
	player := flag.Int("player", 0, "input player port")
	all := flag.Bool("all", false, "include always-zero cells")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: mine-invariants -rom x.bin [-frames N -seed S -actions a,b -fields f,g -all]")
		os.Exit(2)
	}
	var actions, fields []string
	if *actionsCSV != "" {
		actions = strings.Split(*actionsCSV, ",")
	}
	if *fieldsCSV != "" {
		fields = strings.Split(*fieldsCSV, ",")
	}

	cands, err := mine.Mine(*rom, *frames, *seed, actions, *player, fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	var invs, monos []string
	for _, c := range cands {
		if !*all && c.Kind == "const" && c.Value == 0 {
			continue // 常に 0 ＝未使用、退屈なので既定で省く
		}
		switch c.Kind {
		case "const", "range":
			invs = append(invs, "    "+c.JSON())
		case "monotonic-up", "monotonic-down":
			monos = append(monos, "    "+c.JSON())
		}
	}
	fmt.Printf("// mined from %s over %d frames (seed=%d)\n", *rom, *frames, *seed)
	fmt.Printf("\"invariants\": [\n%s\n],\n", strings.Join(invs, ",\n"))
	fmt.Printf("\"monotonic\": [\n%s\n]\n", strings.Join(monos, ",\n"))
}
