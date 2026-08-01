package cyclebound

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/build"
)

// TestEveryPagePenaltyBranchHasAWitness measures which branches of the page-cross
// costing the corpus actually exercises, and fails when the precise ones have no
// witness.
//
// This exists because of how the constant-index bug survived. `prove >= measured` is
// checked on 228 regions across 31 ROMs and passed the whole time, because NO CORPUS
// ROM took the branch that was wrong. Measured tonight, before litmus_pagecross was
// added, the branch "index is known and the access CROSSES a page" ran ZERO times in
// 123 ROMs; "known and does not cross" ran once. A gate over outcomes cannot see a
// branch nothing reaches, so this one counts the branches instead.
//
// It also re-derives what pagePenalty should return from the same inputs and compares,
// so the classification cannot drift away from the function it describes.
func TestEveryPagePenaltyBranchHasAWitness(t *testing.T) {
	var files []string
	for _, p := range []string{"../../roms/techniques/*.asm", "../../roms/litmus/*.asm"} {
		f, _ := filepath.Glob(p)
		files = append(files, f...)
	}
	if len(files) < 100 {
		t.Skipf("only %d ROMs found; the corpus is not present", len(files))
	}

	count := map[string]int{}
	roms := 0
	for _, asm := range files {
		bin := build.BinPathFor(asm)
		out, _, _, err := build.AssembleWithListing(asm, bin)
		if err != nil {
			t.Logf("%s: assemble failed, skipped:\n%s", filepath.Base(asm), out)
			continue
		}
		rom, err := os.ReadFile(bin)
		if err != nil || len(rom) < 6 || len(rom) > 0x10000 {
			continue
		}
		units, decline := analysisUnits(rom, bin)
		if decline != "" {
			continue
		}
		roms++
		decodes, instrs, entries, _ := decodeUnits(units)
		sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
		if sw.banked {
			sw.hotspots = units[0].hotspots
			for _, u := range units {
				sw.banks[u.bank] = true
			}
		}
		widen, _ := unmodelledLandings(instrs, sw)
		states, _ := computeStates(instrs, entries, romByBank(decodes), sw, widen)

		for at, in := range instrs {
			d := in.Def
			st := states[at]
			var branch string
			var want int
			absIndexed := d.AddressingMode == instructions.AbsoluteX ||
				d.AddressingMode == instructions.AbsoluteY
			switch {
			case !d.PageSensitive || d.IsBranch():
				branch, want = "not-sensitive-or-branch", 0
			// Mirrors pagePenalty's order: a page-aligned base settles the question
			// before either conservative bail-out, because base+255 cannot leave the
			// page and no index analysis is needed to know it.
			case absIndexed && int(in.Operand)&0xFF == 0:
				branch, want = "base-page-aligned", 0
			case !st.valid:
				branch, want = "state-unknown", 1
			case d.AddressingMode != instructions.AbsoluteX && d.AddressingMode != instructions.AbsoluteY:
				branch, want = "indirect-or-other", 1
			default:
				idx := st.X
				if d.AddressingMode == instructions.AbsoluteY {
					idx = st.Y
				}
				base := int(in.Operand)
				switch {
				case idx.Top:
					branch, want = "index-unknown", 1
				case (base >> 8) != ((base + idx.Hi) >> 8):
					branch, want = "index-known-CROSSES", 1
				default:
					branch, want = "index-known-no-cross", 0
				}
			}
			count[branch]++
			if got := in.pagePenalty(st); got != want {
				t.Errorf("%s $%04X %v %v: classified as %q (expects %d) but pagePenalty returned %d — "+
					"this test's reading of the branches has drifted from the function",
					filepath.Base(asm), in.Addr, d.Operator, d.AddressingMode, branch, want, got)
			}
		}
	}

	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("   %-24s %d", k, count[k])
	}

	// The two PRECISE branches are the ones worth guarding: everything else returns
	// the conservative +1, where a mistake over-approximates. A bug in either of
	// these under-approximates, which is the direction this package forbids.
	for _, k := range []string{"index-known-CROSSES", "index-known-no-cross"} {
		if count[k] == 0 {
			t.Errorf("no ROM in the corpus reaches the %q branch of pagePenalty. That is exactly how the "+
				"constant-index bug survived a passing prove>=measured gate: an outcome check cannot see "+
				"a branch nothing takes. Add a litmus that does (roms/litmus/litmus_pagecross.asm is the "+
				"one that covers CROSSES).", k)
		}
	}
	if count["base-page-aligned"] == 0 {
		t.Errorf("no ROM reaches the %q branch. It was added because a picture kernel aligns its tables "+
			"and the corpus had none; if litmus_pagealign stopped exercising it, the shortcut is "+
			"unwitnessed again", "base-page-aligned")
	}
	t.Logf("classified the costing of %d ROMs; both precise page-cross branches have a witness", roms)
}
