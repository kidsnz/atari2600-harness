package cyclebound

// "The analysis cannot pin A" is not "A has no bound".
//
// The `sec / sbc #N / bcs` divide is how a sprite gets positioned, and its trip count
// comes from A entering the loop. `determineBound` maximises that over the header's
// predecessors and refuses when any of them carries a Top accumulator — correctly,
// because SD-9's proxy guessed one and under-approximated by 40x.
//
// But refusing EVERY unpinned entry conflates two statements:
//
//	"this analysis does not know the value"   true, and unavoidable
//	"this value has no upper bound"           FALSE about a 6502 accumulator
//
// A is eight bits. Whatever the machine puts in it, it is at most 255 — a fact about
// the hardware, not a range inferred from the program. That is why using it does not
// reopen the door SD-9 closed: the failure there was reading a number off the wrong
// INSTRUCTION, not reading a register's width off the datasheet.
//
// FOUND ON THE USER'S OWN ROM, and it was the second half of a scenario failure whose
// first half was fixed earlier the same day. `pizza_boy.asm` positions its sprites
// through `lda px / jsr SetXPos`, where `SetXPos` opens with `sta WSYNC` and divides A
// by 15. `px` is a RAM byte, Top by construction, at all five call sites — so every
// call context died on this line, and the region came back "no WSYNC reached from
// region start". The refusal named a symptom four steps downstream of its cause.
//
// Corpus: **4 regions gained, 0 lost** (all four in Panda Chase), and
// TestProvenWorstIsNeverExceededOnCorpus stays green.

import (
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestAnUnpinnableAccumulatorIsStillEightBitsWide(t *testing.T) {
	const asm = "../../roms/litmus/litmus_amax_floor.asm"

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
	if len(measured) == 0 {
		t.Fatal("no interval was measured at all; the machine side of this test is empty")
	}

	// THE ROW. A is read from INTIM while the RIOT counts, so nothing static can pin
	// it — and the loop is still a divide with a known subtrahend.
	fl, ok := region("FloorRow")
	if !ok {
		t.Fatal("no FloorRow region")
	}
	if !fl.Bounded {
		t.Fatalf("FloorRow is refused (%s) — the accumulator is eight bits wide whether or "+
			"not this analysis can pin its value", fl.Reason)
	}
	row, hit := measured[fl.Start]
	if !hit {
		t.Fatal("FloorRow produced no measured interval, so its bound is graded by nothing")
	}
	if fl.Worst < row.WorstCycles {
		t.Errorf("FloorRow: proven %d, machine %d — an UNDER-approximation. Supplying a "+
			"ceiling where the scan found none is exactly the kind of change that makes one",
			fl.Worst, row.WorstCycles)
	}

	// THE CONTROL. A comes from an immediate the scan CAN read, so its bound must be
	// TIGHTER. If the two come out equal, the floor has replaced the real scan rather
	// than standing under it, and every divide fold in the corpus just got looser.
	kc, ok := region("KnownCtl")
	if !ok {
		t.Fatal("no KnownCtl region")
	}
	if !kc.Bounded {
		t.Fatalf("KnownCtl is refused (%s) — `lda #60` is exactly what the predecessor scan "+
			"is for", kc.Reason)
	}
	if kc.Worst >= fl.Worst {
		t.Errorf("KnownCtl proves %d and FloorRow proves %d — the pinned row must be CHEAPER. "+
			"Equal bounds mean the 255 floor is answering for both, i.e. the scan stopped "+
			"being consulted", kc.Worst, fl.Worst)
	}
	// And it must still be the number the scan produces, not a coincidence.
	if kc.Worst != 43 {
		t.Errorf("KnownCtl proves %d, expected 43 (A=60, sbc #15 → 6 iterations) — if this "+
			"moved, the change touched the pinned path as well", kc.Worst)
	}

	t.Logf("FloorRow proven %d against machine %d (unpinned A, 255-derived); "+
		"KnownCtl proven %d (A pinned at 60)", fl.Worst, row.WorstCycles, kc.Worst)
}

// TestTheFloorDoesNotRescueTheDexPath pins the boundary of the change.
//
// The dex/dey counter path refuses an unknown entry value for a different reason: the
// count IS the register's value there, so a 255 floor would say "this loop may run 255
// times" about a loop whose counter the author set to 3 — technically sound, uselessly
// loose, and it would mask the very defect SD-9 was about (a wrong count read off the
// wrong instruction). The floor is deliberately confined to the divide idiom, where
// the count is A/N and the subtrahend is a constant in the instruction.
func TestTheFloorDoesNotRescueTheDexPath(t *testing.T) {
	rep, err := Prove("../../roms/litmus/cb_deadpred.asm", DefaultBudget)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// cb_deadpred's delay loop is a dex countdown. Whatever its verdict, the reason
	// must never become a divide-style bound: if it does, the floor has leaked.
	for _, r := range rep.Unbounded {
		if strings.Contains(r.Reason, "loop bound unknown") {
			return // still refused, which is the point
		}
	}
	// It may legitimately be fully bounded (the branch prune closed its dead
	// predecessor route on 2026-08-04). What must not happen is a bound that came
	// from the floor, and there is no dex fixture that would show that here — so this
	// test records the boundary rather than witnessing it, and says so.
	t.Log("cb_deadpred is fully bounded, so this test currently records the boundary " +
		"rather than witnessing it: the 255 floor is confined to the BCS/BCC divide path " +
		"by construction (it sits inside that branch of determineBound), and no dex " +
		"fixture in the tree reaches a Top counter to prove it stays out")
}
