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
	out := struct {
		Rom        string   `json:"rom"`
		Frames     int      `json:"frames"`
		Oracles    []string `json:"oracles"`
		Majority   bool     `json:"majority"`
		Dissenters []string `json:"dissenters,omitempty"`
		RAM0x80    int      `json:"ram_0x80"`
	}{*rom, *frames, names, ok, dissenters, int(maj[0])}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))

	if len(oracles) == 1 {
		fmt.Fprintln(os.Stderr, "note: only Gopher2600 available — install mame for an independent cross-check")
	}
	if !ok || len(dissenters) > 0 {
		fmt.Fprintf(os.Stderr, "DISSENT: %v disagree with the majority\n", dissenters)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "AGREE: %d oracle(s) unanimous\n", len(oracles))
}
