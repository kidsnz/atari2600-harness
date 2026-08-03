package cyclebound

// The latch must read the counter's own flags.
//
// `determineBound` derives a trip count from "the counter decrements to zero and the
// branch exits there". That reasoning is about the DECREMENT's Z/N, so it holds only
// if those are the flags the latch tests. It never checked. Anything writing Z or N
// in between substitutes its own condition, and the derived count describes a loop
// that does not exist.
//
// Measured before the fix, on this fixture's DangerRow: proven **19** cycles against
// a machine that spends **3829 across 51 scanlines** — 201x under, reported with
// certified:true and roll_free:true. That is the one direction this package forbids,
// and it is SD-9's fortyfold proxy bug arriving by a second route.
//
// The three regions are asserted together because each rules out a different wrong
// repair:
//
//	DangerRow  must be REFUSED          — the defect itself
//	SafeRow    must stay EXACT          — refusing every counted loop would "fix" it
//	StoreRow   must stay EXACT          — requiring the decrement to be ADJACENT to
//	                                      the latch would break a sound loop
//
// Without StoreRow the cheapest repair passes, and it would have cost real precision:
// a store between the decrement and the latch preserves the flags, and Chopper
// Command $F39D is such a loop.

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestLatchMustReadTheCountersFlags(t *testing.T) {
	const asm = "../../roms/litmus/litmus_latchflags.asm"
	const bin = "../../roms/litmus/litmus_latchflags.bin"

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

	// --- the defect: a flag-writing instruction between the decrement and the latch
	danger, ok := region("DangerRow")
	if !ok {
		t.Fatal("no DangerRow region; the fixture's labels moved")
	}
	if danger.Bounded {
		t.Errorf("DangerRow is bounded at %d. Its `cpx #$02` sits between the `dex` and the `bne`, so "+
			"the branch tests the COMPARISON: X enters at 1, the decrement makes it 0, the compare "+
			"clears Z, and the counter wraps through $FF for 255 iterations. The machine spends 3829 "+
			"cycles across 51 scanlines in this interval", danger.Worst)
	}
	if rep.Certified {
		t.Error("the report certifies a ROM containing an unbounded visible region")
	}

	// --- the machine, for the two that must keep their bounds
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(10, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := map[uint16]emu.LineWorst{}
	for _, r := range rows {
		if r.Count > 0 {
			measured[r.StrobePC] = r
		}
	}

	// The danger region must still be MEASURED, or the fixture proves nothing about
	// the hardware — a refusal on a region the machine never runs is free.
	if row, hit := measured[danger.Start]; !hit {
		t.Error("DangerRow produced no measured interval; the fixture would be refusing a region the " +
			"machine never executes, which demonstrates nothing")
	} else if row.WorstCycles < 100 {
		t.Errorf("DangerRow measured only %d cycles; the fixture is meant to run away (3829 when "+
			"written), so it is no longer the shape this test is about", row.WorstCycles)
	}

	for _, c := range []struct {
		name string
		why  string
	}{
		{"SafeRow", "the latch reads the decrement's own flags, so this loop is exactly as boundable " +
			"as it ever was; losing it would mean the repair refuses every counted loop"},
		{"StoreRow", "`stx` writes memory and leaves Z/N alone, so the latch still reads the decrement. " +
			"Losing it would mean the repair demands adjacency, which rejects sound loops — Chopper " +
			"Command $F39D is one"},
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
			t.Errorf("%s: proven %d, machine %d — this region's cost is fully determined, so the two "+
				"must agree exactly; a gap means precision was lost to the repair",
				c.name, r.Worst, row.WorstCycles)
		}
	}
}

// TestPreservesZNIsAWhitelistNotAGuess pins the flag table itself. It is written out
// by hand because the engine's instruction definitions carry no flag information —
// `Effect` describes memory, not status — so nothing else in the tree can catch a
// wrong entry. An operator wrongly listed as flag-preserving reopens exactly the hole
// above.
func TestPreservesZNIsAWhitelistNotAGuess(t *testing.T) {
	// The dangerous direction: these MUST NOT be treated as preserving.
	for _, op := range clobbersZN() {
		if preservesZN(op) {
			t.Errorf("%v is listed as leaving Z/N alone, and it does not. A latch after it reads ITS "+
				"flags, not the counter's", op)
		}
	}
	// The precision direction: these must stay allowed, or sound loops get refused.
	for _, op := range keepsZN() {
		if !preservesZN(op) {
			t.Errorf("%v does not set Z or N, so refusing a loop because of it costs precision for "+
				"nothing", op)
		}
	}
}

// The two lists are spelled out rather than derived, because deriving them from the
// same source `preservesZN` reads would make the test a restatement of the code.
// Each entry is a 6502 fact: does this operator write Z or N.
func clobbersZN() []instructions.Operator {
	return []instructions.Operator{
		instructions.CPX, instructions.CPY, instructions.CMP, // the one that bit us
		instructions.LDA, instructions.LDX, instructions.LDY,
		instructions.TAX, instructions.TXA, instructions.TAY, instructions.TYA,
		instructions.TSX, // sets Z/N from S; TXS does not, and is excluded anyway
		instructions.ADC, instructions.SBC,
		instructions.AND, instructions.ORA, instructions.EOR,
		instructions.INX, instructions.INY, instructions.INC, instructions.DEC,
		instructions.ASL, instructions.LSR, instructions.ROL, instructions.ROR,
		instructions.BIT, // writes N and V from the operand
		instructions.PLA, instructions.PLP,
	}
}

func keepsZN() []instructions.Operator {
	return []instructions.Operator{
		instructions.STA, instructions.STX, instructions.STY,
		instructions.PHA, instructions.PHP,
		instructions.CLC, instructions.SEC, instructions.CLI, instructions.SEI,
		instructions.CLV, instructions.CLD, instructions.SED,
		instructions.NOP,
	}
}
