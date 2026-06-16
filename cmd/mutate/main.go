// Command mutate is mutation testing for ROM scenario suites: inject a ROM-byte
// fault and check the scenario suite catches it (kill). Grades the *tests*.
//
//	go run ./cmd/mutate -rom x.bin -offset 100 -value 0xEA  scen.json ...   # single (exit 1 if it SURVIVES)
//	go run ./cmd/mutate -rom x.bin -count 30 -seed 1        scen.json ...   # batch: report kill rate
//
// docs/testing-playbook.md (DeMillo/Offutt). MCP-free, CI-friendly.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/mutate"
)

func main() {
	rom := flag.String("rom", "", "ROM .bin to mutate (required)")
	offset := flag.Int("offset", -1, "single mutation: byte offset")
	value := flag.Int("value", -1, "single mutation: new byte value (0-255)")
	count := flag.Int("count", 0, "batch: number of random mutations")
	seed := flag.Int64("seed", 1, "batch: PRNG seed (deterministic)")
	flag.Parse()
	scenarios := flag.Args()
	if *rom == "" || len(scenarios) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mutate -rom x.bin [-offset N -value V | -count K -seed S] <scenario.json ...>")
		os.Exit(2)
	}

	if *count > 0 { // バッチ＝検査スイートの強さ（kill 率）を採点
		rs, err := mutate.EvalRandom(*rom, *count, *seed, scenarios)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		survived := 0
		for _, r := range rs {
			if !r.Killed {
				survived++
				fmt.Printf("SURVIVED  offset=%d %02x->%02x  (no scenario caught it)\n", r.Mutation.Offset, r.OrigByte, r.Mutation.Value)
			}
		}
		fmt.Printf("kill rate: %.0f%% (%d/%d killed; %d survived)\n", mutate.KillRate(rs)*100, len(rs)-survived, len(rs), survived)
		return
	}

	if *offset < 0 || *value < 0 || *value > 255 { // 単体
		fmt.Fprintln(os.Stderr, "single mode needs -offset N -value 0..255 (or use -count for batch)")
		os.Exit(2)
	}
	base, err := os.ReadFile(*rom)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	dir, err := os.MkdirTemp("", "mutate")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	defer os.RemoveAll(dir)
	r, err := mutate.EvalOne(base, mutate.Mutation{Offset: *offset, Value: byte(*value)}, scenarios, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	if r.Killed {
		fmt.Printf("KILLED  offset=%d %02x->%02x  by %s\n", r.Mutation.Offset, r.OrigByte, r.Mutation.Value, r.By)
		return
	}
	fmt.Printf("SURVIVED  offset=%d %02x->%02x  (no scenario caught it — checks too weak)\n", r.Mutation.Offset, r.OrigByte, r.Mutation.Value)
	os.Exit(1)
}
