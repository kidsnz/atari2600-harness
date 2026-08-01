package cyclebound

// A page-aligned table cannot be crossed, and the proof is now allowed to say so.
//
// `pagePenalty` reached its conservative +1 whenever the index range was not
// provable, which is precisely when a kernel aligns its tables. The rule that
// settles it needs no index analysis at all: a 6502 index register holds 0..255, so
// `$NN00 + idx` is at most `$NNFF` and stays in the base's page for every possible
// index.
//
// The branch had NO witness in this repository, and that absence was itself a
// measurement. Across the 135 ROMs in `roms/` the case "aligned base, unknown index"
// fires zero times — because 24 of the 31 technique kernels draw no playfield, so
// none of them is a table-driven picture kernel, and a picture kernel is what aligns
// tables. The first one written produced eight wasted charges on its first run and
// proved 74 against a machine that takes 66. It lives in another repository, hence
// `litmus_pagealign.asm`.
//
// The test asserts EQUALITY against the machine for the aligned region. A bound that
// is merely safe is not the goal here — the whole complaint is that a loose bound
// tells an author to trim work that was never over budget.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"os"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

func TestPageAlignedBaseIsNotChargedForACrossing(t *testing.T) {
	const asm = "../../roms/litmus/litmus_pagealign.asm"

	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}

	proven := map[string]Region{}
	for _, r := range rep.Lines {
		switch {
		case r.StartLoc != "" && len(r.StartLoc) >= 6 && r.StartLoc[:6] == "AlnRow":
			proven["aligned"] = r
		case r.StartLoc != "" && len(r.StartLoc) >= 6 && r.StartLoc[:6] == "SplRow":
			proven["split"] = r
		}
	}
	for _, label := range []string{"aligned", "split"} {
		if _, ok := proven[label]; !ok {
			t.Fatalf("no region named %s in the report; the fixture's labels moved and this test is "+
				"aimed at something that no longer exists", label)
		}
	}

	// PREMISE — the aligned region must really contain page-aligned indexed reads
	// whose index the analysis cannot pin. Without this the equality below could be
	// satisfied by a fixture that stopped exercising the branch at all.
	alignedReads, unknownIndex := countAlignedUnknownIndexReads(t, asm, proven["aligned"].Start)
	if alignedReads < 4 {
		t.Fatalf("the aligned region holds %d page-aligned indexed reads; the fixture is written with 4, "+
			"and with none it witnesses nothing", alignedReads)
	}
	if unknownIndex < 4 {
		t.Fatalf("%d of the aligned region's reads have a PROVABLE index range; the branch under test is "+
			"the one where the index is unknown, so this fixture would pass through the old code path",
			alignedReads-unknownIndex)
	}

	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(build.BinPathFor(asm)); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	if lines, err := e.StepFrame(); err != nil {
		t.Fatal(err)
	} else if lines != 262 {
		t.Fatalf("frame is %d lines, not 262", lines)
	}
	rows, _, err := e.ProfileLineWorst(10, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := map[uint16]emu.LineWorst{}
	for _, row := range rows {
		if row.Count > 0 {
			measured[row.StrobePC] = row
		}
	}

	al := proven["aligned"]
	alRow, ok := measured[al.Start]
	if !ok {
		t.Fatalf("the aligned region ($%04X) produced no measured interval — the comparison would be "+
			"vacuous, which is how a costing bug survives", al.Start)
	}
	if al.Worst != alRow.WorstCycles {
		t.Errorf("aligned region: proven %d, machine %d — a page-aligned base cannot be crossed by any "+
			"index, so these must agree exactly. A difference of %d is the phantom charge this change "+
			"removes (or, if negative, an under-approximation, which is worse)",
			al.Worst, alRow.WorstCycles, al.Worst-alRow.WorstCycles)
	}

	// The unaligned twin must STILL be charged, or the shortcut has widened into
	// "never charge" and the under-approximation this package forbids is back. It is
	// asserted on the costing function directly rather than on the region total,
	// because at run time SWCHA reads $FF and SplitTbl+255 really does cross — the
	// region's number is correct there either way, so the region cannot tell the two
	// implementations apart. The function can.
	sp := proven["split"]
	spRow, ok := measured[sp.Start]
	if !ok {
		t.Fatalf("the split region ($%04X) produced no measured interval", sp.Start)
	}
	if sp.Worst < spRow.WorstCycles {
		t.Errorf("split region: proven %d, machine %d — an under-approximation", sp.Worst, spRow.WorstCycles)
	}
	splitAligned, splitCharged := countUnalignedUnknownIndexCharges(t, asm, sp.Start)
	if splitAligned != 0 {
		t.Errorf("the split region holds %d PAGE-ALIGNED reads; it is meant to hold none", splitAligned)
	}
	if splitCharged < 4 {
		t.Errorf("only %d of the split region's unaligned reads are charged the crossing cycle; with an "+
			"unknown index every one of them must be, or the shortcut stopped looking at the base's low "+
			"byte and an unprovable crossing goes uncosted", splitCharged)
	}
	t.Logf("aligned proven %d == machine %d (tight); split proven %d >= machine %d with %d reads still charged",
		al.Worst, alRow.WorstCycles, sp.Worst, spRow.WorstCycles, splitCharged)
}

// countUnalignedUnknownIndexCharges is the mirror of the premise helper: in the given
// region, how many indexed reads have a NON-aligned base, and how many of those does
// pagePenalty still charge. The second number is the guard against a shortcut that
// grew too wide.
func countUnalignedUnknownIndexCharges(t *testing.T, asm string, regionStart uint16) (aligned, charged int) {
	t.Helper()
	bin := build.BinPathFor(asm)
	rom, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read %s: %v", bin, err)
	}
	units, decline := analysisUnits(rom, bin)
	if decline != "" {
		t.Fatalf("fixture declined: %s", decline)
	}
	decodes, instrs, entries, _ := decodeUnits(units)
	sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
	widen, _ := unmodelledLandings(instrs, sw)
	states, _ := computeStates(instrs, entries, romByBank(decodes), sw, widen)

	for at, in := range instrs {
		if in.Addr < regionStart || in.Addr > regionStart+0x20 {
			continue
		}
		d := in.Def
		if !d.PageSensitive || d.IsBranch() {
			continue
		}
		if d.AddressingMode != instructions.AbsoluteX && d.AddressingMode != instructions.AbsoluteY {
			continue
		}
		if int(in.Operand)&0xFF == 0 {
			aligned++
			continue
		}
		if in.pagePenalty(states[at]) == 1 {
			charged++
		}
	}
	return aligned, charged
}

// countAlignedUnknownIndexReads re-derives the fixture's premise from the same
// decode the prover uses: how many indexed reads in the region start at a
// page-aligned base, and of those, how many have an index the analysis cannot pin.
// Both numbers matter — an aligned base with a PROVABLE index was already free under
// the old code, so a fixture made only of those would witness nothing new.
func countAlignedUnknownIndexReads(t *testing.T, asm string, regionStart uint16) (aligned, unknown int) {
	t.Helper()
	bin := build.BinPathFor(asm)
	rom, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read %s: %v", bin, err)
	}
	units, decline := analysisUnits(rom, bin)
	if decline != "" {
		t.Fatalf("fixture declined: %s", decline)
	}
	decodes, instrs, entries, _ := decodeUnits(units)
	sw := switchModel{banked: len(units) > 1, banks: map[int]bool{}}
	widen, _ := unmodelledLandings(instrs, sw)
	states, _ := computeStates(instrs, entries, romByBank(decodes), sw, widen)

	// The region runs from its opening strobe to the next one; the fixture's two
	// rows are adjacent, so a short forward window covers it without needing the
	// solver's own collection.
	for at, in := range instrs {
		if in.Addr < regionStart || in.Addr > regionStart+0x20 {
			continue
		}
		d := in.Def
		if !d.PageSensitive || d.IsBranch() {
			continue
		}
		if d.AddressingMode != instructions.AbsoluteX && d.AddressingMode != instructions.AbsoluteY {
			continue
		}
		if int(in.Operand)&0xFF != 0 {
			continue
		}
		aligned++
		st := states[at]
		idx := st.X
		if d.AddressingMode == instructions.AbsoluteY {
			idx = st.Y
		}
		if !st.valid || idx.Top {
			unknown++
		}
	}
	return aligned, unknown
}
