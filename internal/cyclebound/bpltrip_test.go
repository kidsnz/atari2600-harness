package cyclebound

import (
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// A dex/BPL countdown runs one MORE iteration than a dex/BNE one, and the prover
// gave both the same trip count.
//
// `dex; bne L` from X=6 leaves when the decrement produces zero: 6 iterations.
// `dex; bpl L` from X=6 leaves only when the decrement produces a NEGATIVE value, so
// it runs the body once more with X=0 and exits on X=$FF: 7 iterations. The bound was
// short by one body plus one taken branch — on the real Seaquest cartridge, region
// $F1FC, exactly 9 cycles: proven 66 against a machine-measured 75, carried out on
// Bounded=true. An under-approximation is the one answer this package must never give.
//
// The assertion here is EQUALITY, not `observed <= proven` like the corpus gate.
// Tightness is the whole point: best+1 is the EXACT trip count for the bpl exit
// condition, not a conservative cushion, and a bound that is merely safe would send an
// author trimming cycles out of a line that was never over budget. So a fix that
// over-corrected — +2, or unconditionally +1 on BNE as well — has to fail here too.
//
// litmus_bpl_trip.asm carries the fixture; its header explains why the ROM exists and
// why the standing corpus gate could not find this on the kernels we wrote.
func TestBplCountdownTripCountMatchesTheMachine(t *testing.T) {
	const asm = "../../roms/litmus/litmus_bpl_trip.asm"

	// Machine-measured, both regions, 10 frames (950 and 960 intervals). Recomputed by
	// hand from the instruction costs so the numbers are checkable without running
	// anything:
	//   VisX  ldx #6 (2) + 7*(sta zp,x 4 + dex 2) + 6*(bpl taken 3) + (bpl exit 2)
	//         + dec zp (5) + bne taken (3) + the WSYNC that closes the region (3) = 75
	//   VisY  ldy #6 (2) + 7*(sty zp 3 + dey 2) + 6*3 + 2 + 5 + 3 + 3            = 68
	// At the OLD (BNE) trip count of 6 they come out 66 and 60.
	want := map[string]int{"VisX": 75, "VisY": 68}

	// PREMISE, checked before the conclusion. If Prove never folds a bpl latch down
	// the `dec:` path, the equalities below could hold for reasons that have nothing
	// to do with what this test claims to witness. The counter is incremented at the
	// exact line that applies the correction.
	before := bplTripCountAdjusted

	rep, err := Prove(asm, DefaultBudget)
	if err != nil {
		t.Fatalf("prove %s: %v", asm, err)
	}
	if folds := bplTripCountAdjusted - before; folds < 2 {
		t.Fatalf("the `dec:` path folded %d bpl latch(es) while proving %s; the fixture has two "+
			"(dex/bpl at FillX, dey/bpl at FillY) and this test witnesses nothing without them",
			folds, asm)
	} else {
		t.Logf("premise: %d bpl trip counts adjusted while proving the fixture", folds)
	}

	proven := map[string]Region{}
	for _, r := range append(append([]Region{}, rep.Lines...), rep.BlankLines...) {
		for label := range want {
			if strings.HasPrefix(r.StartLoc, label+" ") {
				proven[label] = r
			}
		}
	}
	for label := range want {
		r, ok := proven[label]
		if !ok {
			t.Fatalf("no region opens at %s — the fixture changed shape and this test is grading "+
				"something else", label)
		}
		if !r.Bounded {
			t.Fatalf("region %s is unbounded (%s); the fixture is meant to be provable", label, r.Reason)
		}
		if r.Kind != "visible" {
			t.Errorf("region %s is %q, not visible — a blank region is skipped by the budget check "+
				"and would witness nothing", label, r.Kind)
		}
	}

	bin := build.BinPathFor(asm)
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("assemble %s: %s", asm, out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	// 262 lines: if a region overran its scanline the "worst cycles" below would be
	// measuring a roll rather than a fold.
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	if lines, err := e.StepFrame(); err != nil {
		t.Fatal(err)
	} else if lines != 262 {
		t.Fatalf("frame is %d lines, not 262 — both fixture regions are meant to fit one scanline", lines)
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

	for label, expect := range want {
		r := proven[label]
		row, ok := measured[r.Start]
		if !ok {
			t.Fatalf("region %s ($%04X) produced no measured interval — the gate would compare "+
				"nothing here, which is how this bug survived on our own kernels", label, r.Start)
		}
		if row.WorstLines != 1 {
			t.Errorf("region %s took %d scanlines; the fixture is built to stay inside one",
				label, row.WorstLines)
		}
		if row.WorstCycles != expect {
			t.Errorf("region %s: the machine took %d cycles over %d intervals, expected %d — "+
				"recheck the fixture before the prover", label, row.WorstCycles, row.Count, expect)
		}
		if r.Worst != expect {
			delta := expect - r.Worst
			t.Errorf("region %s: proven worst %d, machine %d (%d intervals) — a gap of %d cycles. "+
				"The loop is a dex/dey countdown latched by BPL, which exits on NEGATIVE and so runs "+
				"best+1 = 7 iterations, not best = 6; one body plus one taken branch is exactly what "+
				"is missing. A proven bound BELOW the machine is unsound",
				label, r.Worst, row.WorstCycles, row.Count, delta)
			continue
		}
		t.Logf("%s ($%04X): proven %d == machine %d over %d intervals, %d line",
			label, r.Start, r.Worst, row.WorstCycles, row.Count, row.WorstLines)
	}
}
