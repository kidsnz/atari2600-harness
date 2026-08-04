package cyclebound

import (
	"fmt"
	"testing"
)

func TestZZTW(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_timerwait.asm", 76)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	fmt.Printf("\n%-14s %-8s %6s  %s\n", "region", "bounded", "worst", "reason")
	for _, r := range append(append([]Region{}, rep.Lines...), rep.Unbounded...) {
		if seen[r.StartLoc] {
			continue
		}
		seen[r.StartLoc] = true
		fmt.Printf("%-14.14s %-8v %6d  %.62s\n", r.StartLoc, r.Bounded, r.Worst, r.Reason)
	}
}
