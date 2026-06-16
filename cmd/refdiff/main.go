// Command refdiff diffs a ROM's layout fingerprint against a reference ROM (the
// original game = the oracle): left/right wall position and ball width. Catches
// "wrong vs the original" that golden self-regression never sees.
// docs/testing-playbook.md (differential testing).
//
//	go run ./cmd/refdiff -rom mine.bin -ref original.bin
//
// exit 0 if every feature matches the reference, 1 if any differs.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/refdiff"
)

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	ref := flag.String("ref", "", "reference/original ROM (.bin, required)")
	warmup := flag.Int("warmup", 10, "frames to settle before measuring")
	flag.Parse()
	if *rom == "" || *ref == "" {
		fmt.Fprintln(os.Stderr, "usage: refdiff -rom mine.bin -ref original.bin [-warmup N]")
		os.Exit(2)
	}

	got, err := refdiff.Extract(*rom, *warmup)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR rom: %v\n", err)
		os.Exit(2)
	}
	want, err := refdiff.Extract(*ref, *warmup)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR ref: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("rom : %s\n", got)
	fmt.Printf("ref : %s\n", want)
	ds := refdiff.Compare(got, want)
	for _, d := range ds {
		mark := "ok  "
		if !d.Match {
			mark = "DIFF"
		}
		fmt.Printf("    %s %-18s rom=%d ref=%d\n", mark, d.Feature, d.Got, d.Want)
	}
	if !refdiff.AllMatch(ds) {
		fmt.Println("MISMATCH — the ROM diverges from the original")
		os.Exit(1)
	}
	fmt.Println("MATCH — every measured feature equals the original")
}
