// timinglint — static TIA-timing linter (authoring aid). Reads a kernel and warns,
// BEFORE running it, about high-confidence horizontal-motion timing pitfalls:
// HMOVE strobed with no HMxx ever set, HMxx set but HMOVE never strobed, and an
// HMxx/HMCLR write within <24 cycles of an HMOVE (the motion-undefined hazard).
// Zero false positives on known-good kernels is the design bar; it complements the
// runtime checks (assert_line_budget / VV-10 HMOVE hazard) by being proactive.
//
//	go run ./cmd/timinglint -asm roms/techniques/multicolor48.asm
//
// Exit 0 = no warnings; 1 = warnings; 2 = error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
)

func main() {
	asm := flag.String("asm", "", "kernel .asm to lint (required)")
	asJSON := flag.Bool("json", false, "emit warnings as JSON")
	flag.Parse()
	if *asm == "" {
		fmt.Fprintln(os.Stderr, "usage: timinglint -asm kernel.asm [-json]")
		os.Exit(2)
	}
	ws, err := cyclebound.Lint(*asm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	if *asJSON {
		b, _ := json.MarshalIndent(ws, "", "  ")
		fmt.Println(string(b))
	} else if len(ws) == 0 {
		fmt.Printf("timinglint: %s — no timing warnings\n", *asm)
	} else {
		fmt.Printf("timinglint: %s — %d warning(s)\n", *asm, len(ws))
		for _, x := range ws {
			fmt.Printf("  [%s] %s\n      %s\n      hint: %s\n", x.Rule, x.Loc, x.Msg, x.Hint)
		}
	}
	if len(ws) > 0 {
		os.Exit(1)
	}
}
