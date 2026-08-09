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
	"sort"

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
	// PONG-C3: per-line worst table, worst first — "trim by the exact margin".
	lines := append([]cyclebound.Region(nil), rep.Lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].Worst > lines[j].Worst })
	for _, r := range lines {
		mark := "  "
		switch {
		case !r.Bounded:
			mark = "??"
		case r.Over:
			mark = "!!"
		}
		if r.Bounded {
			fmt.Fprintf(os.Stderr, "%s %3dcy /%3d  %s\n", mark, r.Worst, r.Budget, r.StartLoc)
		} else {
			fmt.Fprintf(os.Stderr, "%s   ?  /%3d  %s: %s\n", mark, r.Budget, r.StartLoc, r.Reason)
		}
	}
	// Bank accounting. A per-bank line is what stops a barely-decoded bank from
	// reading as a checked one, and the seed list names the cross-bank entry points
	// the decode was closed over (see cyclebound.CrossBankSeed).
	if rep.Banks > 0 {
		for _, c := range rep.BankCoverage {
			fmt.Fprintf(os.Stderr, "   bank %d: %d instrs / %d regions (%d seeded entry point(s))\n",
				c.Bank, c.Instructions, c.Regions, c.SeededEntries)
		}
		for _, s := range rep.CrossBankSeeds {
			fmt.Fprintf(os.Stderr, "   seed: %s\n", s.Desc)
		}
		if rep.CrossBankSeedCapped {
			fmt.Fprintf(os.Stderr, "   WARNING: cross-bank seeding stopped at its %d-round cap — the "+
				"decode is INCOMPLETE by an unknown amount\n", rep.CrossBankSeedRounds)
		}
		for _, u := range rep.UnresolvedHotspots {
			fmt.Fprintf(os.Stderr, "   unresolved hotspot: %s\n", u)
		}
		// The modelled-edge count prints beside the refusal count so "0 refused" cannot be
		// read as "the crossing was checked": a cartridge that crossed nothing reports 0 too.
		fmt.Fprintf(os.Stderr, "   cross-bank flow: %d modelled edge(s), %d region(s) refused for a switch "+
			"this analysis does not model\n", rep.ModelledSwitchEdges, rep.UnmodelledSwitches)
		if rep.SwitchWidenedSites > 0 {
			fmt.Fprintf(os.Stderr, "   %d site(s) forced to an unknown value state as a possible landing of "+
				"an unmodelled switch\n", rep.SwitchWidenedSites)
			for _, w := range rep.SwitchWidenReasons {
				fmt.Fprintf(os.Stderr, "     because: %s\n", w)
			}
		}
		if rep.SourceAnnotations != "" {
			fmt.Fprintf(os.Stderr, "   source annotations %s\n", rep.SourceAnnotations)
		}
		if rep.UnresolvableSwitchAccesses > 0 {
			// State the count and what it does NOT tell you. Claiming "the regions
			// holding them are refused" asserts a coupling this line does not check,
			// and it prints directly above the CERTIFIED verdict where a reader is
			// deciding how much to trust that word. The refusal count is a separate
			// number and is printed as one.
			fmt.Fprintf(os.Stderr, "   %d access(es) whose target could not be resolved: each MAY select "+
				"a bank and none could be seeded. Whether the region holding one was refused is the "+
				"unmodelled-switch count, not this one (%d refused).\n",
				rep.UnresolvableSwitchAccesses, rep.UnmodelledSwitches)
		}
	}
	if rep.Certified {
		// `Certified` covers the VISIBLE regions only. A blank region over budget is
		// not a visible tear, but it is not harmless either: the WSYNC after it waits
		// for the next line and the frame comes out one scanline long. Printing a bare
		// "CERTIFIED" while the report's own BlankOver list is non-empty told a reader
		// the ROM was fine when the tool had already found the defect -- measured
		// 2026-08-09 on technojacket, where a 77-cycle VBLANK line made 5 frames in
		// 300 come out at 263 lines and only ntsc_frame_lines caught it.
		if len(rep.BlankOver) > 0 {
			fmt.Fprintf(os.Stderr, "NOT ROLL-FREE: every visible region is within budget, but %d BLANK region(s) "+
				"exceed it (worst %d cy of %d). A blank overrun pushes the following WSYNC to the next line, "+
				"so the frame gains a scanline.\n", len(rep.BlankOver), rep.BlankMaxWorst, rep.Budget)
			for _, v := range rep.BlankOver {
				fmt.Fprintf(os.Stderr, "  BLANK OVER %d>%d @ %s\n", v.Worst, v.Budget, v.StartLoc)
			}
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "CERTIFIED: %d regions within budget (base %d cy/line; worst region %d cy visible, %d cy blank)\n",
			rep.Regions, rep.Budget, rep.MaxWorst, rep.BlankMaxWorst)
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
