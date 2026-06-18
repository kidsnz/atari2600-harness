// Command cyclebound STATICALLY proves an Atari 2600 kernel's per-scanline cycle
// budget over ALL reachable paths (VV-2) — the ∀ sibling of assert_line_budget's
// single-run ∃ observation. It assembles the .asm, decodes from the reset/IRQ/NMI
// vectors, cuts the CFG at every `STA WSYNC`, and proves each WSYNC-to-WSYNC
// region's worst-case cost <= budget (default 76). Over-budget regions are
// reported with a cycle-by-cycle worst path; loops it can't bound are reported
// honestly, never silently passed. See internal/cyclebound and
// docs/capability-gap-audit.md (VV-2).
//
//	go run ./cmd/cyclebound -asm roms/litmus/smoke.asm
//	go run ./cmd/cyclebound -asm roms/litmus/cyclebound_branch.asm -budget 76
//
// Prints the full report as JSON on stdout (a human summary on stderr); exits
// non-zero when the kernel is not certified (a violation or an unbounded region).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
)

func main() {
	asm := flag.String("asm", "", "kernel .asm to prove (required)")
	budget := flag.Int("budget", cyclebound.DefaultBudget, "per-WSYNC-interval CPU-cycle budget")
	flag.Parse()

	if *asm == "" {
		fmt.Fprintln(os.Stderr, "usage: cyclebound -asm kernel.asm [-budget 76]")
		os.Exit(2)
	}

	rep, err := cyclebound.Prove(*asm, *budget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	out, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(out))

	// Human summary on stderr (stdout stays clean JSON for piping).
	if rep.Certified {
		fmt.Fprintf(os.Stderr, "CERTIFIED: %d regions within budget (base %d cy/line; worst region %d cy, scaled per @lines)\n", rep.Regions, rep.Budget, rep.MaxWorst)
		return
	}
	fmt.Fprintf(os.Stderr, "NOT CERTIFIED: %d violation(s), %d unbounded region(s) (budget %d)\n",
		len(rep.Violations), len(rep.Unbounded), rep.Budget)
	for _, v := range rep.Violations {
		fmt.Fprintf(os.Stderr, "  OVER %d>%d @ %s\n", v.Worst, v.Budget, v.StartLoc)
	}
	for _, u := range rep.Unbounded {
		fmt.Fprintf(os.Stderr, "  UNBOUNDED @ %s: %s\n", u.StartLoc, u.Reason)
	}
	os.Exit(1)
}
