package cyclebound

// A branch whose flag is already decided has ONE successor, not two.
//
// `collectRegion` walked both arms of every branch and `longest` costed both. That
// is right when the flag is unknown and wrong when it is not — and when the dead arm
// falls into a DATA TABLE, the walk decodes the table as instructions.
//
// FOUND ON THE USER'S OWN ROM, not on the corpus.
// `sandbox/practice/pizza-boy-tokyo/build/pizza_boy.asm` line 673 has
//
//	lda #0          ; Z := 1
//	sta Dx          ; a store leaves the flags alone
//	beq .cexit      ; therefore ALWAYS taken
//
// followed immediately by the `Alley3A` snap table. The fall-through cannot happen,
// collection took it anyway, and the `$00` at $F490 — a byte of level data — decoded
// as BRK. The region was refused for "BRK in region", and that refusal is what made
// the project's own `phase0` scenario FAIL.
//
// The corpus never showed this. It ranks `BRK in region` at 16 of 332 refusals,
// below four other reasons, which is why it sat untouched while three larger-looking
// obstacles were repaired for no measurable gain. The thing blocking the user's
// actual work was five rows down the list.
//
// `refineBranch` — the same test `absSuccessors` has always applied — settles it,
// and it prunes ONLY when the flag is known.

import (
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestADecidedBranchHasOneSuccessor(t *testing.T) {
	const asm = "../../roms/litmus/litmus_deadbranch.asm"

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

	// THE ROW. `lda #0 / sta $80 / beq` cannot fall through, so the table below it is
	// not code and its $00 is not a BRK.
	dead, ok := region("DeadRow")
	if !ok {
		t.Fatal("no DeadRow region")
	}
	if !dead.Bounded {
		t.Fatalf("DeadRow is refused (%s) — the fall-through of a `beq` after `lda #0` "+
			"cannot happen, so the bytes below it are data, not instructions", dead.Reason)
	}
	row, hit := measured[dead.Start]
	if !hit {
		t.Fatal("DeadRow produced no measured interval, so its bound is graded by nothing")
	}
	if dead.Worst < row.WorstCycles {
		t.Errorf("DeadRow: proven %d, machine %d — an UNDER-approximation. Pruning an arm is "+
			"exactly the kind of change that can remove a path the machine takes",
			dead.Worst, row.WorstCycles)
	}

	// CONTROL 1 — the SAME shape on a flag the analysis cannot decide (it reads
	// SWCHB). Both arms are live, the fall-through genuinely reaches the data, and
	// the region must STAY refused. If this passes, the prune is firing on unknown
	// flags and is removing paths the machine can take.
	live, ok := region("LiveRow")
	if !ok {
		t.Fatal("no LiveRow region")
	}
	if live.Bounded {
		t.Errorf("LiveRow is bounded at %d — its branch reads SWCHB, so BOTH arms are real "+
			"and the fall-through does reach the table. Pruning here would be unsound",
			live.Worst)
	}
	if !strings.Contains(live.Reason, "BRK in region") {
		t.Errorf("LiveRow is refused for %q, expected the BRK refusal — if the reason moved, "+
			"this control is no longer testing the same thing", live.Reason)
	}

	// CONTROL 2 — a decided branch with CODE after it, bounded before and after at
	// the same number. The prune changes which arms exist, not what an arm costs.
	if p, ok := region("PlainRow"); !ok || !p.Bounded || p.Worst != 18 {
		t.Errorf("PlainRow: bounded=%v worst=%d, expected a bound of 18", ok && p.Bounded, p.Worst)
	}
}

// TestTheBRKRefusalNamesItsAddress pins a small thing that cost real time today.
//
// The refusal used to read exactly "BRK in region", with no address. The region it
// names starts at $F075, but the BRK it found was at $F490 — four hundred bytes away,
// inside a data table. Diagnosing that meant instrumenting the prover and re-running
// it, when the message could have said so.
func TestTheBRKRefusalNamesItsAddress(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_deadbranch.asm", 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	for _, r := range rep.Unbounded {
		if !strings.Contains(r.Reason, "BRK in region") {
			continue
		}
		if !strings.Contains(r.Reason, "$") {
			t.Errorf("the BRK refusal is %q — it must name the address, because the BRK is "+
				"generally nowhere near the region's own start", r.Reason)
		}
		return
	}
	t.Fatal("no region was refused for a BRK; LiveRow is supposed to be one")
}
