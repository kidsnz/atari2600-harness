package cyclebound

// A loop body with an if/else in it is still a counted loop.
//
// `loopShape` walked the body as a STRAIGHT CHAIN and refused the moment it met a
// branch. Measured across the sixteen-cartridge corpus, of the branches that tripped
// that refusal:
//
//	89   forward, target still inside the body   (an if/else skip)
//	29   forward, target outside the body        (an early exit)
//	 1   backward, inside the body               (an inner loop)
//
// Three quarters are a skip, most often `bcc` (64 of 118) — "add, and if it did not
// carry, skip the fixup". Such a body is not a chain but a small acyclic graph with
// two ways through that rejoin before the latch, and it has a LONGEST PATH, which is
// a sound cost for one iteration: whichever way the machine goes, it cannot spend
// more.
//
// WHAT IT BOUGHT ON THE CORPUS: one loop. See
// TestTheBranchWallHidARaftOfLargerWalls for the measurement and what it means.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestBranchInLoopBodyIsCostedByItsLongestPath(t *testing.T) {
	const asm = "../../roms/litmus/litmus_branchbody.asm"

	before := branchBodyFolds
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// PREMISE — the fixture must actually cost a body as a graph. The corpus yield is
	// ONE loop, so this fixture is effectively the only witness that the new path runs
	// at all; without this check, reverting the change would leave the rest of the
	// test passing on whatever else answered the region.
	if got := branchBodyFolds - before; got < 1 {
		t.Fatalf("no loop body was costed as a graph; the fixture witnesses nothing")
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

	// THE ROW THE CHANGE IS FOR. Its two arms differ by the four cycles of the two
	// skipped `nop`s, and the bound must be the longer one.
	br, ok := region("BranchRow")
	if !ok {
		t.Fatal("no BranchRow region")
	}
	if !br.Bounded {
		t.Fatalf("BranchRow is refused (%s) — a body whose two arms rejoin before the "+
			"latch has a longest path, and that path is a sound cost for one iteration",
			br.Reason)
	}
	row, hit := measured[br.Start]
	if !hit {
		t.Fatal("BranchRow produced no measured interval, so its bound is graded by nothing")
	}
	if br.Worst < row.WorstCycles {
		t.Errorf("BranchRow: proven %d, machine %d — an UNDER-approximation. Relaxing a "+
			"refusal is exactly the kind of change that introduces one",
			br.Worst, row.WorstCycles)
	}

	// THE CONTROLS, each refused for its own reason.
	for _, c := range []struct{ name, why string }{
		{"ExitCtl", "the branch LEAVES the loop, so the trip count is no longer the " +
			"counter's range and the cost after the exit belongs to another part of the region"},
		{"InnerCtl", "the branch goes BACKWARD inside the body: an inner loop, which one " +
			"longest path charges once for iterations the machine runs many times"},
	} {
		r, ok := region(c.name)
		if !ok {
			t.Errorf("no %s region", c.name)
			continue
		}
		if r.Bounded {
			t.Errorf("%s is now bounded at %d — %s", c.name, r.Worst, c.why)
		}
	}

	// A chain body has exactly one path, so the DAG walk must reproduce the old
	// running sum instruction for instruction. This is what keeps every folded region
	// in the corpus at the bound it already had (measured: 0 lost, 0 raised, 0
	// lowered over 958 regions).
	if s, ok := region("SingleCtl"); !ok || !s.Bounded || s.Worst != 36 {
		t.Errorf("SingleCtl: bounded=%v worst=%d, expected a bound of 36 — a body with no "+
			"branch must cost exactly what the chain walk gave it", ok && s.Bounded, s.Worst)
	}
}

// TestTheBranchWallHidARaftOfLargerWalls records what the repair actually bought, and
// it is the more useful half.
//
// Allowing a branch in the body moved ONE loop across the corpus. The reason is not
// that the 89 skips were miscounted — it is that a body walk stops at its FIRST
// obstacle, so a census of first obstacles is a census of what is nearest, not of
// what is blocking. With the branch wall gone, the walks run further and hit what was
// always behind it. Measured over single-latch loops, after the change:
//
//	105   body fully understood        (of which just 1 needed the graph)
//	 53   body understood, TRIP COUNT unknown
//	 41   WSYNC inside loop body
//	 13   branch (early exit or inner loop — the shapes that stay refused)
//	 13   call or jump inside loop body
//
// So `branch inside loop body` fell from 118 first-hits to 13, and `WSYNC inside loop
// body` rose to 41 — the same loops, now failing further along. The largest obstacle
// is no longer a body shape at all: 53 loops have a body this package understands
// completely and a trip count it cannot establish.
//
// This is the third refusal in a row measured to be a name rather than a cause
// (`multiple back-edges` before it, and `SD-9's proxy` before that). The pattern is
// worth more than any of them individually: A CENSUS OF REFUSAL REASONS IS A CENSUS
// OF FIRST OBSTACLES, and removing the first obstacle mostly reveals the second.
func TestTheBranchWallHidARaftOfLargerWalls(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_branchbody.asm", 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// The fixture's own controls are the local instance of the finding: ExitCtl and
	// InnerCtl both hold a branch, both are still refused, and the reasons name the
	// shape rather than the branch. If either starts reporting plain "branch inside
	// loop body" with no qualifier, the successor test stopped distinguishing an
	// early exit from a skip and the fold is one step from being unsound.
	want := map[string]string{
		"ExitCtl":  "branch inside loop body",
		"InnerCtl": multipleBackEdges,
	}
	seen := map[string]bool{}
	for _, r := range rep.Unbounded {
		for name, reason := range want {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				seen[name] = true
				if !hasSub(r.Reason, reason) {
					t.Errorf("%s is refused for %q, expected %q", name, r.Reason, reason)
				}
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s never appeared among the refused regions", name)
		}
	}
}

func hasSub(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestTheBodyRangeCheckHasNoNegativeControl records a guard whose control does not
// fire, in the same spirit as `overlaps` in siblingloops_test.go.
//
// Deleting the `[header, latch]` test on successors leaves both fixture controls
// refused exactly as before — ExitCtl for "WSYNC inside loop body", because the walk
// follows the escape and a region is bounded by strobes, and InnerCtl for
// multipleBackEdges, because a branch below the header IS a back edge and the region
// then carries two latches.
//
// So the guard is currently redundant with two accidents. It is kept because neither
// accident states the premise pass 2 rests on — that every collected site lies in
// [header, latch], which is what makes ascending address a topological order. A body
// walk that wanders below the header would produce a longest path computed in the
// wrong order, and the answer would be wrong rather than refused.
//
// This test pins the two reasons, so that if either accident disappears the fixture
// says which one went rather than quietly resting on the other.
func TestTheBodyRangeCheckHasNoNegativeControl(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_branchbody.asm", 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	got := map[string]string{}
	for _, r := range rep.Unbounded {
		for _, name := range []string{"ExitCtl", "InnerCtl"} {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				got[name] = r.Reason
			}
		}
	}
	if r, ok := got["ExitCtl"]; !ok || !hasSub(r, "branch inside loop body") {
		t.Errorf("ExitCtl is refused for %q; with the range check in place it must be the "+
			"branch test that refuses it, not the WSYNC test standing in", r)
	}
	if r, ok := got["InnerCtl"]; !ok || r != multipleBackEdges {
		t.Errorf("InnerCtl is refused for %q, expected multipleBackEdges — an inner loop is "+
			"a second back edge before it is anything else", r)
	}
}
