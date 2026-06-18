// spritepos — forward sprite-position solver (authoring aid). Given a target X
// (0..159) it returns the routine input, the div-15 coarse/HMOVE-fine decomposition,
// a paste-able SetXPos snippet, and the position the hardware ACTUALLY reaches
// (HmovedPixel, measured by running the kernel — the formula offset is
// kernel-specific, so the answer is verified, not trusted).
//
//	go run ./cmd/spritepos -x 96 -object P0
//	go run ./cmd/spritepos -x 96 -json
//	go run ./cmd/spritepos -object BL -all      # solve every X for one object
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/spritepos"
)

func main() {
	x := flag.Int("x", -1, "target X position 0..159 (required unless -all)")
	object := flag.String("object", "P0", "object: P0 P1 M0 M1 BL")
	all := flag.Bool("all", false, "solve every X 0..159 for -object (table)")
	asJSON := flag.Bool("json", false, "emit JSON")
	flag.Parse()

	p, err := spritepos.NewPositioner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	if *all {
		var sols []spritepos.Solution
		for tx := 0; tx <= 159; tx++ {
			s, err := p.Solve(*object, tx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(2)
			}
			sols = append(sols, s)
		}
		if *asJSON {
			b, _ := json.MarshalIndent(sols, "", "  ")
			fmt.Println(string(b))
			return
		}
		miss := 0
		for _, s := range sols {
			if !s.Exact {
				miss++
				fmt.Printf("  X=%3d  inputA=%3d  achieved=%3d  MISS\n", s.TargetX, s.InputA, s.AchievedX)
			}
		}
		fmt.Printf("spritepos -all %s: 160 targets, %d exact, %d miss\n", *object, 160-miss, miss)
		return
	}

	if *x < 0 || *x > 159 {
		fmt.Fprintln(os.Stderr, "usage: spritepos -x <0..159> [-object P0] [-json]")
		os.Exit(2)
	}
	s, err := p.Solve(*object, *x)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	if *asJSON {
		b, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(b))
		return
	}
	verdict := "EXACT (emulator-verified)"
	if !s.Exact {
		verdict = fmt.Sprintf("achieved %d (off by %+d)", s.AchievedX, s.AchievedX-s.TargetX)
	}
	fmt.Printf("spritepos: %s -> X=%d\n", s.Object, s.TargetX)
	fmt.Printf("  routine input  : lda #%d   (jsr SetXPos)\n", s.InputA)
	fmt.Printf("  decomposition  : %d coarse 15px steps + HMOVE nibble %+d\n", s.CoarseSteps, s.HMOVENibble)
	fmt.Printf("  achieved X     : %s\n\n", verdict)
	fmt.Println(spritepos.Snippet(s.Object, s.InputA))
}
