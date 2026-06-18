// cpucert — VV-14 citable per-scanline cycle-budget certificate
// (docs/capability-gap-audit.md). Wraps the VV-2 static prover (internal/cyclebound)
// in a reproducible, attestable proof artifact: the proven per-region worst-case
// bounds and verdict, the multi-line declarations (`@lines`) the proof relies on,
// and full provenance (prover version, Gopher2600 pin, DASM version, and SHA-256
// of both the source and the assembled ROM).
//
// "Citable" = attach it to a ROM as evidence ("prover vX proved every visible
// scanline <= N cycles; ROM sha256 H"). Deterministic (same ROM => same cert,
// modulo timestamp) and falsifiable (tamper the ROM => hash changes; any region
// over budget or unbounded => NOT certified, exit 1).
//
//	go run ./cmd/cpucert -asm roms/litmus/smoke.asm           # text
//	go run ./cmd/cpucert -asm roms/litmus/smoke.asm -json     # JSON artifact
//
// Exit 0 = certified; 1 = not certified; 2 = error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
)

func main() {
	asm := flag.String("asm", "", "kernel .asm to certify (required)")
	budget := flag.Int("budget", cyclebound.DefaultBudget, "per-visible-scanline CPU-cycle budget")
	asJSON := flag.Bool("json", false, "emit the certificate as JSON")
	flag.Parse()
	if *asm == "" {
		fmt.Fprintln(os.Stderr, "usage: cpucert -asm kernel.asm [-budget 76] [-json]")
		os.Exit(2)
	}

	cert, err := cyclebound.Certify(*asm, *budget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	if *asJSON {
		b, _ := json.MarshalIndent(cert, "", "  ")
		fmt.Println(string(b))
	} else {
		printText(cert)
	}
	if !cert.Certified {
		os.Exit(1)
	}
}

func printText(c *cyclebound.Certificate) {
	status := "CERTIFIED"
	if !c.Certified {
		status = "NOT CERTIFIED"
	}
	fmt.Printf("CPU cycle-budget certificate — %s\n", status)
	fmt.Printf("  kernel:        %s\n", c.Asm)
	fmt.Printf("  budget/line:   %d cycles (NTSC visible scanline)\n", c.Budget)
	fmt.Printf("  regions:       %d analysed (%d blank/setup skipped)\n", c.Regions, c.Blank)
	if c.Certified {
		fmt.Printf("  PROVEN:        every visible region's worst case <= its budget (max worst = %d)\n", c.MaxWorst)
	}
	if len(c.LineDecls) > 0 {
		fmt.Printf("  relies on @lines declarations:\n")
		for _, d := range c.LineDecls {
			fmt.Printf("     line %d: region spans %d scanlines (budget x%d)\n", d.Line, d.Lines, d.Lines)
		}
	}
	for _, v := range c.Violations {
		fmt.Printf("  VIOLATION:     %s worst=%d > budget=%d\n", v.StartLoc, v.Worst, v.Budget)
	}
	for _, u := range c.Unbounded {
		fmt.Printf("  UNBOUNDED:     %s — %s\n", u.StartLoc, u.Reason)
	}
	p := c.Provenance
	fmt.Printf("  provenance:    %s\n", p.ProverVersion)
	if p.GopherPin != "" {
		fmt.Printf("                 gopher2600 %s\n", p.GopherPin)
	}
	if p.DasmVersion != "" {
		fmt.Printf("                 %s\n", p.DasmVersion)
	}
	fmt.Printf("                 asm sha256 %s\n", p.AsmSHA256)
	fmt.Printf("                 rom sha256 %s\n", p.BinSHA256)
	fmt.Printf("                 generated %s\n", p.GeneratedAt)
}
