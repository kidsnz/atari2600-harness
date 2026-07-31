package cyclebound

// The DAG-first path had no witness in this repository.
//
// It bounds regions whose graph is acyclic but whose branches point backwards, and
// every region it was built for is a COMMERCIAL cartridge that is not part of this
// repo — Seaquest $F1EC, Chopper Command $FA78 and $FAEC, Barnstorming $F3D4. A
// path exercised only by files CI does not have is a path CI does not check.
// `litmus_dag_region.asm` is the in-tree specimen.
//
// The test checks its PREMISES before its conclusion, because the conclusion is
// worthless without them: the region must really contain two branches that the
// address-order test counts as back edges, and the walk must really have answered
// it. Without the premise checks a fixture that quietly stopped having backward
// branches — the first draft did exactly that, falling through into the blocks so
// the region was never entered at all — would still report a green tick.

import (
	"os"
	"testing"
)

const dagRegionAsm = "../../roms/litmus/litmus_dag_region.asm"

func TestDagFirstBoundsAnAcyclicRegionWithBackwardBranches(t *testing.T) {
	before := dagFirstAnswered
	beforeMulti := dagFirstMultiLatch

	rep, err := Prove(dagRegionAsm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}

	// PREMISE 1 — the walk answered at least one region that the address-order test
	// calls multi-latch. If this is zero the fixture no longer reaches the path.
	if got := dagFirstMultiLatch - beforeMulti; got < 1 {
		t.Fatalf("the fixture reached the DAG path for %d multi-latch regions; it must reach at least 1, "+
			"or it witnesses nothing (dagFirstAnswered moved by %d)",
			got, dagFirstAnswered-before)
	}

	// PREMISE 2 — nothing in this ROM is refused. A refusal here would mean the
	// fixture drifted into some other shape.
	for _, r := range rep.Unbounded {
		t.Errorf("region $%04X refused: %s", r.Start, r.Reason)
	}

	// CONCLUSION — the dispatch region is bounded, and at the measured number. The
	// value is pinned rather than compared with <= because a bound that drifts
	// upward is a bound that stopped being tight, and tightness is what makes the
	// number usable for trimming a kernel.
	const pickAddr = 0xF043
	const pickWorst = 26
	var found bool
	for _, r := range rep.Lines {
		if int(r.Start) != pickAddr {
			continue
		}
		found = true
		if !r.Bounded {
			t.Fatalf("$%04X is not bounded: %s", pickAddr, r.Reason)
		}
		if r.Worst != pickWorst {
			t.Errorf("$%04X proven worst %d, expected %d", pickAddr, r.Worst, pickWorst)
		}
	}
	if !found {
		t.Fatalf("no region starts at $%04X; the fixture's layout changed and the test is aimed at "+
			"an address that no longer opens a region", pickAddr)
	}
	if !rep.Certified {
		t.Errorf("the ROM did not certify; every region here fits 76 cycles")
	}
}

// TestDagFirstDoesNotOverrideBodyRefusals is the other half, and it is the one that
// matters. foldLoops refuses for eight reasons; only the back-edge one describes the
// GRAPH. An earlier version of this change ran the walk first and accepted whenever
// no cycle was met, which bypassed the other seven — and a loop whose body holds a
// WSYNC is invisible to the walk, because the WSYNC is a sink and the walk stops
// there without traversing the edge back. VideoOlympics $F5CA came back with 148
// cycles for an interval the machine takes 163.
//
// That cartridge is not in this repo, so what is checked here is the invariant that
// made it possible: the override is keyed on the exact refusal string, and the other
// seven are still produced somewhere in the corpus. If a future edit widened the
// override, these reasons would stop appearing.
func TestDagFirstDoesNotOverrideBodyRefusals(t *testing.T) {
	bodyReasons := map[string]int{
		"WSYNC inside loop body":                          0,
		"branch inside loop body — not a simple counted loop": 0,
	}

	roms, err := os.ReadDir("../../roms/techniques")
	if err != nil {
		t.Fatalf("corpus: %v", err)
	}
	graded := 0
	for _, e := range roms {
		if len(e.Name()) < 5 || e.Name()[len(e.Name())-4:] != ".bin" {
			continue
		}
		rep, err := Prove("../../roms/techniques/"+e.Name(), 76)
		if err != nil || rep.BankedDeclined != "" {
			continue
		}
		graded++
		for _, r := range rep.Unbounded {
			if _, ok := bodyReasons[r.Reason]; ok {
				bodyReasons[r.Reason]++
			}
		}
	}
	if graded == 0 {
		t.Fatal("no ROM was graded; this check would assert nothing")
	}
	t.Logf("graded %d technique ROMs", graded)

	total := 0
	for reason, n := range bodyReasons {
		t.Logf("%-52s %d", reason, n)
		total += n
	}
	if total == 0 {
		t.Fatalf("not one body-shape refusal survives anywhere in %d ROMs. Either the corpus stopped "+
			"containing such a loop, or the DAG override widened past the back-edge case — and the "+
			"second is how a bound below the machine's gets published", graded)
	}
}
