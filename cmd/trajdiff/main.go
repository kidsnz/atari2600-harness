// Command trajdiff diffs the runtime behavior of two ROMs (VV-8): it steps an
// original and a candidate ROM in lockstep on the same input timeline and
// reports the first frame+field where their observable state diverges, or MATCH.
// It compares behavior over time (default = the 128-byte RAM each frame), not
// bytes — the strongest oracle for a reproduction task.
//
//	go run ./cmd/trajdiff -a original.bin -b candidate.bin -frames 120
//	go run ./cmd/trajdiff -a a.bin -b b.bin -fields ram.0x85,ram.0x9B -held fire
//
// Prints JSON; exit 1 on divergence, 0 on MATCH.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/trajdiff"
)

func main() {
	a := flag.String("a", "", "original ROM (.bin, required)")
	b := flag.String("b", "", "candidate ROM (.bin, required)")
	frames := flag.Int("frames", 60, "frames to compare in lockstep")
	warmup := flag.Int("warmup", 2, "frames to settle before comparing (both ROMs)")
	fields := flag.String("fields", "", "comma-separated fields to compare (default: all 128 RAM bytes)")
	held := flag.String("held", "", "comma-separated actions held every frame on both ROMs")
	player := flag.Int("player", 0, "input port for -held")
	flag.Parse()

	if *a == "" || *b == "" {
		fmt.Fprintln(os.Stderr, "usage: trajdiff -a original.bin -b candidate.bin [-frames 60] [-fields ram.0x85,...] [-held fire]")
		os.Exit(2)
	}

	o := trajdiff.Options{
		Frames: *frames,
		Warmup: *warmup,
		Fields: splitTrim(*fields),
		Held:   splitTrim(*held),
		Player: *player,
	}
	res, err := trajdiff.Compare(*a, *b, o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
	if !res.Match {
		os.Exit(1)
	}
}

func splitTrim(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
