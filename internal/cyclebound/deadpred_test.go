package cyclebound

import (
	"strings"
	"testing"
)

// TestUnboundedWhenAPredecessorHasNoAbstractState witnesses determineBound's
// "a predecessor we know nothing about: no bound" guard, which had run ZERO times
// across the 123-ROM sweep and, unlike its shadowed sibling, had never been shown
// unreachable either. It was simply unwitnessed — the state a guard should not be
// left in, because "nothing reaches it" and "it is correct" are different claims.
//
// Reaching it took two failed attempts, and both are worth keeping:
//
//   - Code hopped over by a `jmp` never becomes a predecessor at all. The decode
//     follows flow, so an unreachable instruction is not decoded, is not in the
//     region's node map, and the scan cannot see it. Measured: the scan listed 9
//     candidates and the dead instruction's address was not among them.
//   - Dead code placed in the region ABOVE the header is not seen either: the
//     predecessor scan is per-region, and the header of a WSYNC-free delay loop is
//     inside the region that the preceding WSYNC opened.
//
// What does reach it is the NOT-TAKEN edge of a branch whose condition is statically
// known. That edge IS decoded — the decoder cannot prove which way a branch goes —
// and then it gets NO abstract state at all: `absSuccessors` emits only edges whose
// refined state is still valid, so a pruned edge's target is never pushed into the
// state map. The guard fires on `!ok` (no entry), not on `!st.valid`.
//
// That distinction was measured, and it corrects the first write-up of this fixture,
// which named the invalid-state route. Instrumenting the two conditions separately
// over 129 ROMs: `!ok` fires once (here, at $F035) and `!st.valid` fires ZERO times
// anywhere — pruned nodes never acquire a state to be invalid, so in this function
// that condition is as unreachable as the sibling branch this fixture replaced.
//
// It also settles whether the guard could be relaxed. It could not: a missing entry
// means EITHER proven-unreachable OR never-analysed, because a fixpoint that hits its
// iteration cap leaves nodes unprocessed (`computeStatesWith` returns converged=false
// and the work list non-empty). Skipping a missing predecessor would then drop a real
// one and under-approximate the entry value — the one direction this package forbids.
func TestUnboundedWhenAPredecessorHasNoAbstractState(t *testing.T) {
	dead, err := Prove("../../roms/litmus/cb_deadpred.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	live, err := Prove("../../roms/litmus/cb_deadpred_live.asm", DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}

	// The twin differs by ONE pruned branch edge and nothing else. If it does not
	// come back fully bounded, "the other one is unbounded" says nothing about the
	// dead predecessor and this test is measuring the loop shape instead.
	for _, r := range live.Lines {
		if !r.Bounded {
			t.Fatalf("the twin WITHOUT the dead predecessor must be fully bounded, but the region "+
				"at %s is not (%s) — the refusal is then not attributable", r.StartLoc, r.Reason)
		}
	}

	var refused []Region
	for _, r := range dead.Lines {
		if !r.Bounded {
			refused = append(refused, r)
		}
	}
	if len(refused) == 0 {
		t.Fatal("cb_deadpred came back fully bounded: the pruned edge is no longer reaching the " +
			"loop header, so the guard is unwitnessed again")
	}
	for _, r := range refused {
		if !strings.Contains(r.Reason, "loop bound unknown") {
			t.Errorf("region at %s was refused for %q, not for the missing loop bound — the fixture "+
				"is now proving something else", r.StartLoc, r.Reason)
		}
		if !strings.Contains(r.StartLoc, "Top") {
			t.Errorf("the refusal landed at %s; the delay loop it is built around is under Top", r.StartLoc)
		}
	}

	// Premise: the twin really does execute the delay loop, so the difference is a
	// refusal and not a loop that vanished. Its worst region must be dearer than the
	// dead ROM's, whose costliest region is one the prover gave up on and left at 0.
	if live.MaxWorst <= dead.MaxWorst {
		t.Errorf("the twin's worst region (%d cy) is not dearer than the refused ROM's (%d cy) — "+
			"the delay loop is supposed to be COUNTED there", live.MaxWorst, dead.MaxWorst)
	}
	t.Logf("dead: %d refused region(s), max_worst %d | live twin: all bounded, max_worst %d",
		len(refused), dead.MaxWorst, live.MaxWorst)
}
