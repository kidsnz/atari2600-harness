package cyclebound

// Only the decrement may touch the counter.
//
// `determineBound` reads "entry value v, therefore v iterations", which needs the
// body's net effect on the counter to be exactly -1. It latched a boolean when it saw
// a `dex`/`dey`, so a body that ALSO wrote the register was indistinguishable from one
// that did not. Measured on this fixture before the fix: proven **22** against a
// machine that spends **2290 across 31 scanlines** — 104x under, with
// `certified: true` and `roll_free: true`.
//
// It is the other half of the SD-13 repair. `preservesZN` guards the window AFTER the
// decrement, where a compare substitutes its own condition; the window BEFORE it
// changes the count itself, which is worse.
//
// Three controls, each ruling out a different cheap repair — see the fixture's header.
// The one that matters most is OtherCtl: refusing whenever ANY index register is
// written would pass the danger case while rejecting every loop that walks two
// pointers, which is a common shape in real kernels.

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestOnlyTheDecrementMayWriteTheCounter(t *testing.T) {
	const asm = "../../roms/litmus/litmus_counterwrite.asm"

	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	region := func(name string) (Region, bool) {
		for _, r := range append(append([]Region{}, rep.Lines...), rep.Unbounded...) {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				return r, true
			}
		}
		return Region{}, false
	}

	danger, ok := region("DangerRow")
	if !ok {
		t.Fatal("no DangerRow region; the fixture's labels moved")
	}
	if danger.Bounded {
		t.Errorf("DangerRow is bounded at %d. Its body is `inx / inx / dex`, so the counter RISES by one "+
			"per iteration and the loop never terminates on it — the machine spends 2290 cycles across "+
			"31 scanlines in this interval", danger.Worst)
	}
	if rep.Certified {
		t.Error("the report certifies a ROM whose visible region cannot be bounded")
	}

	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(binFor(asm)); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(12, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := map[uint16]emu.LineWorst{}
	for _, r := range rows {
		if r.Count > 0 {
			measured[r.StrobePC] = r
		}
	}

	// PREMISE — the danger region must run away on the machine. A refusal on a region
	// the hardware never executes, or executes briefly, demonstrates nothing.
	if row, hit := measured[danger.Start]; !hit {
		t.Error("DangerRow produced no measured interval; the fixture proves nothing about the hardware")
	} else if row.WorstCycles < 1000 {
		t.Errorf("DangerRow measured %d cycles; it is written to run away (2290 when authored), so it "+
			"is no longer the shape this test is about", row.WorstCycles)
	}

	for _, c := range []struct{ name, why string }{
		{"PlainCtl", "the counter is untouched apart from the decrement, so this loop is as boundable as " +
			"it ever was; losing it means the repair refuses every counted loop"},
		{"StoreCtl", "`stx` writes MEMORY, not the counter, and preserves the flags — losing it means the " +
			"repair keys on position rather than on what is written (Chopper Command $F39D is real)"},
		{"OtherCtl", "`iny` moves the OTHER index register, which is not the counter — losing it means " +
			"the repair refuses any loop that walks two pointers"},
	} {
		r, ok := region(c.name)
		if !ok {
			t.Errorf("no %s region", c.name)
			continue
		}
		if !r.Bounded {
			t.Errorf("%s is refused (%s) — %s", c.name, r.Reason, c.why)
			continue
		}
		row, hit := measured[r.Start]
		if !hit {
			t.Errorf("%s produced no measured interval, so its bound is graded by nothing", c.name)
			continue
		}
		if r.Worst != row.WorstCycles {
			t.Errorf("%s: proven %d, machine %d — this region's cost is fully determined, so they must "+
				"agree exactly; a gap means precision was lost to the repair", c.name, r.Worst, row.WorstCycles)
		}
	}
}

// TestRegisterWriteTablesArePinned does for writesX/writesY what
// TestPreservesZNIsAWhitelistNotAGuess does for the flag table: the engine records no
// register effects, so a wrong entry here cannot be caught by anything else, and a
// missing one reopens the 104x hole.
func TestRegisterWriteTablesArePinned(t *testing.T) {
	for _, op := range writesXOps() {
		if !writesX(op) {
			t.Errorf("%v writes X and is not listed; a loop counter it clobbers would go uncounted", op)
		}
	}
	for _, op := range writesYOps() {
		if !writesY(op) {
			t.Errorf("%v writes Y and is not listed", op)
		}
	}
	// The precision direction: these touch neither index register, and refusing a
	// loop because of one costs precision for nothing.
	for _, op := range touchesNeitherIndex() {
		if writesX(op) || writesY(op) {
			t.Errorf("%v is listed as writing an index register and does not", op)
		}
	}
}

// Spelled out rather than derived from the code under test, so this is a statement of
// 6502 fact rather than a restatement of writesX/writesY.
func writesXOps() []instructions.Operator {
	return []instructions.Operator{
		instructions.LDX, instructions.INX, instructions.DEX,
		instructions.TAX, instructions.TSX,
	}
}

func writesYOps() []instructions.Operator {
	return []instructions.Operator{
		instructions.LDY, instructions.INY, instructions.DEY, instructions.TAY,
	}
}

func touchesNeitherIndex() []instructions.Operator {
	return []instructions.Operator{
		instructions.LDA, instructions.STA, instructions.STX, instructions.STY,
		instructions.ADC, instructions.SBC, instructions.CMP, instructions.CPX, instructions.CPY,
		instructions.AND, instructions.ORA, instructions.EOR, instructions.BIT,
		instructions.ASL, instructions.LSR, instructions.ROL, instructions.ROR,
		instructions.INC, instructions.DEC,
		instructions.TXA, instructions.TYA, instructions.TXS,
		instructions.PHA, instructions.PHP, instructions.PLA, instructions.PLP,
		instructions.CLC, instructions.SEC, instructions.CLD, instructions.SED,
		instructions.CLI, instructions.SEI, instructions.CLV, instructions.NOP,
	}
}

func binFor(asm string) string { return build.BinPathFor(asm) }
