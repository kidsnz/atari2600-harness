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
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
)

// perBank renders the per-bank instruction counts so a reader can see that a bank
// was not silently skipped.
func perBank(r cyclebound.LintResult) string {
	if r.Declined != "" {
		return "NOTHING ANALYSED"
	}
	banks := make([]int, 0, len(r.PerBank))
	for b := range r.PerBank {
		banks = append(banks, b)
	}
	sort.Ints(banks)
	parts := make([]string, 0, len(banks))
	for _, b := range banks {
		parts = append(parts, fmt.Sprintf("bank %d: %d", b, r.PerBank[b]))
	}
	return strings.Join(parts, ", ")
}

func main() {
	asm := flag.String("asm", "", "kernel .asm to lint (required)")
	asJSON := flag.Bool("json", false, "emit warnings as JSON")
	flag.Parse()
	if *asm == "" {
		fmt.Fprintln(os.Stderr, "usage: timinglint -asm kernel.asm [-json]")
		os.Exit(2)
	}
	res, err := cyclebound.LintDetail(*asm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	ws := res.Warnings
	if *asJSON {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else if len(ws) == 0 {
		// Always state the denominator. "no timing warnings" over a program that was
		// never decoded reads identically to a clean bill of health, and that is not a
		// hypothetical: on an 8K cartridge this tool used to read zero instructions.
		fmt.Printf("timinglint: %s — no timing warnings (read %d instructions across %d bank(s): %s)\n",
			*asm, res.Instructions, res.Banks, perBank(res))
	} else {
		fmt.Printf("timinglint: %s — %d warning(s) (read %d instructions across %d bank(s): %s)\n",
			*asm, len(ws), res.Instructions, res.Banks, perBank(res))
		for _, x := range ws {
			fmt.Printf("  [%s] %s\n      %s\n      hint: %s\n", x.Rule, x.Loc, x.Msg, x.Hint)
		}
	}
	if len(ws) > 0 {
		os.Exit(1)
	}
}
