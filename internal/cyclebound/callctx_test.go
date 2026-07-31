package cyclebound

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// commercialROMs are graded by the corpus gate alongside the kernels in this repo.
//
// They are here because the corpus WE wrote could not find the defect that was in the
// prover. determineBound gave a dex/dey countdown latched by BPL the same trip count
// as one latched by BNE, though the bpl form exits on NEGATIVE and runs one more
// iteration; the bound came out one body plus one taken branch short. Of the 7 bpl
// folds across the 140 images analysed, only 4 were in our own kernels and NOT ONE
// could expose it: rts_dispatch's $F036 and zone_multiplex's $F033 produce no
// ProfileLineWorst row at all, so the gate compared nothing there, and
// shared_setxpos's $F054 was proven 83 against a measured 36 — 47 cycles of slack to
// absorb the 15 cycles of error. Seaquest's $F1FC is a tight 75-cycle line and showed
// the gap immediately: proven 66, machine 75.
//
// The corpus was the corpus we happened to write. That is the same shape of defect
// docs/capability-gap-audit.md records over and over, and the repair is to grade code
// nobody here authored.
//
// Nothing is READ from these cartridges but their own bytes — no published
// disassembly, no third-party RAM map. They live in the umbrella `reference/` tree,
// which is NOT part of this repo, so CI will not have them: the gate skips them when
// absent and asserts on them when present. A skip that can quietly become permanent is
// itself a defect this project keeps finding, so the count of images actually graded
// is logged, and anything between "none" and "all of them" fails.
var commercialROMs = []string{
	"../../../reference/roms-study/VideoOlympics.bin",
	"../../../reference/pizza-boy/Samples for Pizza Boy/Adventure.bin",
	"../../../reference/pizza-boy/Samples for Pizza Boy/Seaquest.bin",
	"../../../reference/pizza-boy/Samples for Pizza Boy/Chopper Command.bin",
	"../../../reference/pizza-boy/Samples for Pizza Boy/Star Wars - The Empire Strikes Back.bin",
}

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
	// "on every kernel" was written above while this ran on the 31 technique ROMs,
	// about a quarter of the images in the tree. The litmus and exerciser corpora
	// are kernels too, and one of them is where the last two gradings found their
	// defects.
	for _, pat := range []string{"../../roms/litmus/*.asm", "../../roms/exerciser/*.asm"} {
		more, _ := filepath.Glob(pat)
		files = append(files, more...)
	}
	// …and every kernel in the tree is still a kernel we wrote. See commercialROMs.
	commercialPresent := 0
	for _, bin := range commercialROMs {
		if _, err := os.Stat(bin); err == nil {
			commercialPresent++
			files = append(files, bin)
		}
	}
	if commercialPresent != 0 && commercialPresent != len(commercialROMs) {
		t.Fatalf("%d of %d commercial cartridges are present — a PARTIAL corpus is the failure this "+
			"extension exists to prevent, since the missing one is exactly where the next gap will be. "+
			"Restore the umbrella reference/ tree or remove it entirely", commercialPresent, len(commercialROMs))
	}
	// No known exceptions. One was listed here — bitmap48's Krow, proven at 93
	// cycles and apparently taking 911 — and the guard below caught its own
	// obsolescence within the hour: the discrepancy was in the MEASURING
	// instrument, not the proof (SD-4 in the audit). Keeping the list and the
	// guard, because the next real gap should be recorded the same way rather
	// than skipped.
	known := map[string]bool{}
	knownSeen := map[string]bool{}

	regions, roms := 0, 0
	commercialGraded, commercialPairs := 0, 0
	for _, asm := range files {
		// A raw cartridge has no source to assemble and no listing, so nothing names its
		// regions; and because it is the half of the corpus we did not write, its errors
		// are reported rather than skipped past. The in-tree loop keeps its `continue`s:
		// those files are covered by other gates that would notice.
		isBin := strings.EqualFold(filepath.Ext(asm), ".bin")
		fail := t.Logf
		if isBin {
			fail = t.Errorf
		}
		rep, err := Prove(asm, DefaultBudget)
		if err != nil {
			fail("prove %s: %v", asm, err)
			continue
		}
		// Keyed on (bank, address). An 8K image decodes the SAME addresses in both
		// banks, so an address-only key can pair a region proven in one bank with a
		// measured row from the other. That direction matters here: an accidental
		// pairing that happens to satisfy observed <= proven HIDES a real gap, which
		// is the failure this test exists to catch. banked_game is in the corpus.
		type key struct {
			bank int
			addr uint16
		}
		proven := map[key]Region{}
		add := func(r Region) {
			if !r.Bounded {
				return
			}
			bk := 0
			if r.BankValid {
				bk = r.Bank
			}
			proven[key{bk, r.Start}] = r
		}
		for _, r := range rep.Lines {
			add(r)
		}
		for _, r := range rep.BlankLines {
			add(r)
		}
		if len(proven) == 0 {
			fail("%s: no region was proven at all — nothing to grade", asm)
			continue
		}

		bin := asm
		if !isBin {
			bin = build.BinPathFor(asm)
			if out, err := build.Assemble(asm, bin); err != nil {
				t.Logf("assemble %s: %s", asm, out)
				continue
			}
		}
		e, err := emu.New("NTSC")
		if err != nil {
			fail("emu %s: %v", asm, err)
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			fail("load %s: %v", bin, err)
			continue
		}
		// A commercial title spends its first frames in an attract/init path, so 6
		// frames leaves whole kernels unentered — measured on Chopper Command, which
		// yields ZERO rows at 6 frames and 18 at 30. A ROM present but contributing no
		// pair is a silent skip, which is what the count below refuses to allow.
		frames := 6
		if isBin {
			frames = 30
		}
		rows, _, err := e.ProfileLineWorst(frames, nil)
		if err != nil {
			fail("profile %s: %v", asm, err)
			continue
		}
		roms++
		pairs := 0
		for _, row := range rows {
			rowBank := 0
			if row.BankValid {
				rowBank = row.Bank
			}
			p, ok := proven[key{rowBank, row.StrobePC}]
			if !ok || row.Count == 0 {
				continue
			}
			regions++
			pairs++
			// A raw image has no labels, so name the region by the address the strobe
			// actually sits at rather than printing an empty location.
			where := p.StartLoc
			if where == "" {
				where = fmt.Sprintf("$%04X", p.Start)
			}
			key := filepath.Base(asm) + "|" + where
			if row.WorstCycles > p.Worst {
				if known[key] {
					knownSeen[key] = true
					t.Logf("KNOWN GAP (SD-4) %s: machine %d cycles vs proven %d", key, row.WorstCycles, p.Worst)
					continue
				}
				t.Errorf("%s region %s: the machine took %d cycles, the proof says at most %d — "+
					"a worst case the hardware exceeds is worse than no number at all",
					filepath.Base(asm), where, row.WorstCycles, p.Worst)
			}
		}
		if isBin {
			commercialPairs += pairs
			if pairs > 0 {
				commercialGraded++
			}
			t.Logf("commercial %s: %d proven regions, %d measured rows, %d graded pairs",
				filepath.Base(asm), len(proven), len(rows), pairs)
		}
	}
	if regions == 0 {
		t.Fatal("no region was compared against a measured run — the test proves nothing")
	}
	// 0 = the umbrella reference/ tree is absent (CI); 5 = all of them graded. Anything
	// between is a skip in the making: a cartridge that is present, gets loaded, and
	// then quietly contributes no comparison looks exactly like a passing test.
	switch commercialGraded {
	case 0:
		t.Logf("commercial cartridges: none present (umbrella reference/ tree not in this repo) — "+
			"%d in-tree kernels graded instead", roms)
	case len(commercialROMs):
		t.Logf("commercial cartridges: %d/%d graded, %d region<->row pairs",
			commercialGraded, len(commercialROMs), commercialPairs)
	default:
		t.Errorf("only %d of %d commercial cartridges produced a graded pair (%d pairs total) — "+
			"a ROM that is present but compares nothing is a skip wearing a pass",
			commercialGraded, len(commercialROMs), commercialPairs)
	}
	for k := range known {
		if !knownSeen[k] {
			t.Errorf("%s is listed as a known proof gap but did not violate — if it was fixed, "+
				"remove it from the list and close SD-4; a stale exemption hides a repaired bug", k)
		}
	}
	t.Logf("observed <= proven on %d measured regions across %d ROMs, no exceptions", regions, roms)
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
