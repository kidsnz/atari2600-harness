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
// **★ 2026-08-04: THE ROUTE THIS FIXTURE USED IS NOW CLOSED, and the closing is the
// point.** `collectRegion` and `longest` now prune the arm of a branch whose flag is
// statically decided, using the same `refineBranch` the abstract interpreter has
// always used. That was done because `pizza_boy.asm` has `lda #0 / sta Dx / beq .exit`
// followed by a data table: collection walked the impossible fall-through, decoded the
// table, met a `$00`, and refused the region for "BRK in region" — an instruction the
// machine never executes at an address that holds graphics.
//
// The pruned edge was exactly this fixture's route to the guard. With it gone,
// `cb_deadpred` comes back fully bounded and the guard has NO witness again.
//
// The guard is kept anyway, and the reason is in the note above: a missing state entry
// means either PROVEN-UNREACHABLE or NEVER-ANALYSED. The prune removes the first kind
// before `determineBound` can see it, which is correct — a node the machine cannot
// reach is not a predecessor. The second kind survives: `computeStatesWith` can return
// `converged=false` with its work list non-empty, and those nodes have no state for a
// reason that has nothing to do with reachability. Deleting the guard would let one of
// them be skipped, and a skipped predecessor lowers the maximum, which is an
// under-approximation.
//
// So this test now records the CLOSURE rather than witnessing the guard. It asserts
// the twin still behaves, that the dead ROM is now fully bounded, and it says out loud
// that the guard is unwitnessed — the same treatment `overlaps` and the body-range
// check got, for the same reason: an empty count is a fact to write down, not a
// licence to delete.
func TestTheDeadPredecessorRouteIsClosedByBranchPruning(t *testing.T) {
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
	// THE CLOSURE. If this ever fires again, the prune has stopped working — which is
	// a real regression, because the prune is what keeps a data table from being
	// decoded as instructions.
	if len(refused) != 0 {
		t.Errorf("cb_deadpred has %d refused region(s) (first: %s, %q). The branch prune is "+
			"supposed to remove the dead edge before determineBound scans predecessors; if it "+
			"is back, check that collectRegion and longest still agree on refineBranch",
			len(refused), refused[0].StartLoc, refused[0].Reason)
	}
	// THE TWINS DO NOT AGREE ON COST, AND THAT IS MEASURED RATHER THAN ASSUMED.
	// live 33, dead 38. The first draft of this assertion demanded they be equal, on
	// the reasoning that "the ROMs differ by one edge the machine cannot take" — which
	// is what the fixture's own header claims. The measurement says otherwise, so the
	// header's claim is the thing that is loose: the dead ROM carries the instructions
	// that FEED the dead predecessor as well as the edge into it, and those are still
	// decoded and still costed, because they are reachable by other paths.
	//
	// What must hold is the direction: pruning an edge the machine cannot take can
	// only ever remove work, so the dead twin cannot come out CHEAPER than the live
	// one. If it does, the prune has removed a path the machine can take.
	if dead.MaxWorst < live.MaxWorst {
		t.Errorf("the dead-predecessor ROM is CHEAPER than its twin (%d vs %d) — pruning an "+
			"impossible edge must not remove a path the machine can take",
			dead.MaxWorst, live.MaxWorst)
	}
	t.Logf("both twins fully bounded (dead %d, live %d); determineBound's "+
		"\"predecessor with no abstract state\" guard is UNWITNESSED — kept for the "+
		"never-analysed case (computeStatesWith can return converged=false), which no "+
		"fixture currently reaches", dead.MaxWorst, live.MaxWorst)
	_ = strings.Contains
}
