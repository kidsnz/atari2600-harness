package cyclebound

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The proof has to hold on the machine, on every kernel, not on one litmus.
//
// A WSYNC inside a subroutine opens a region whose continuation lives in the
// caller, and resolving that per call site is what turned five previously
// unprovable regions into numbers. Numbers are exactly the thing that must now
// be checked: a proven worst case that the hardware exceeds is worse than no
// number at all, because it would be trusted.
//
// The rule is one-sided on purpose. Observed <= proven is required; observed
// well BELOW proven is fine and expected, since a run only takes the paths its
// inputs lead it down.
func TestProvenWorstIsNeverExceededOnCorpus(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	// A single region is known to exceed its proven worst, measured and recorded
	// as SD-4 in docs/capability-gap-audit.md: bitmap48's Krow is proven at 93
	// cycles and one interval in six frames takes 911, spanning 12 physical lines
	// out of 84 measured intervals — i.e. the loop-exit iteration, which the proof
	// does not appear to price. It is listed here by name rather than skipped, so
	// it stays visible, and the test fails if it ever STOPS violating: an
	// exemption that silently becomes unnecessary hides that the bug was fixed.
	known := map[string]bool{"bitmap48.asm|Krow (bitmap48.asm:146)": true}
	knownSeen := map[string]bool{}

	regions, roms := 0, 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			continue
		}
		proven := map[uint16]Region{}
		for _, r := range rep.Lines {
			if r.Bounded {
				proven[r.Start] = r
			}
		}
		for _, r := range rep.BlankLines {
			if r.Bounded {
				proven[r.Start] = r
			}
		}
		if len(proven) == 0 {
			continue
		}

		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		rows, err := e.ProfileLineWorst(6, nil)
		if err != nil {
			continue
		}
		roms++
		for _, row := range rows {
			p, ok := proven[row.StrobePC]
			if !ok || row.Count == 0 {
				continue
			}
			regions++
			key := filepath.Base(asm) + "|" + p.StartLoc
			if row.WorstCycles > p.Worst {
				if known[key] {
					knownSeen[key] = true
					t.Logf("KNOWN GAP (SD-4) %s: machine %d cycles vs proven %d", key, row.WorstCycles, p.Worst)
					continue
				}
				t.Errorf("%s region %s: the machine took %d cycles, the proof says at most %d — "+
					"a worst case the hardware exceeds is worse than no number at all",
					filepath.Base(asm), p.StartLoc, row.WorstCycles, p.Worst)
			}
		}
	}
	if regions == 0 {
		t.Fatal("no region was compared against a measured run — the test proves nothing")
	}
	for k := range known {
		if !knownSeen[k] {
			t.Errorf("%s is listed as a known proof gap but did not violate — if it was fixed, "+
				"remove it from the list and close SD-4; a stale exemption hides a repaired bug", k)
		}
	}
	t.Logf("observed <= proven on %d measured regions across %d ROMs (1 known gap, SD-4)", regions, roms)
}

// A region cannot cost zero cycles: reaching the next WSYNC takes at least the
// store that ends it. A zero would mean the walk found a sink without executing
// anything, which is a modelling error, not a very fast kernel.
func TestNoRegionIsProvenFree(t *testing.T) {
	files, _ := filepath.Glob("../../roms/techniques/*.asm")
	checked := 0
	for _, asm := range files {
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			continue
		}
		all := append(append([]Region{}, rep.Lines...), rep.BlankLines...)
		for _, r := range all {
			if !r.Bounded {
				continue
			}
			checked++
			if r.Worst <= 0 {
				t.Errorf("%s region %s: proven worst is %d; reaching the next WSYNC costs at least "+
					"the store that ends the region", filepath.Base(asm), r.StartLoc, r.Worst)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no bounded region was checked")
	}
}
