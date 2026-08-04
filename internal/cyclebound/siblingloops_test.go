package cyclebound

// Two loops in a region are not necessarily nested, and the refusal was named after
// the rarest of the three shapes.
//
// `foldLoops` refused any region with more than one back edge, under the name
// "multiple back-edges (nested/complex loops)". Measured across the sixteen-cartridge
// corpus, of the regions carrying exactly TWO latches:
//
//	22   siblings — the intervals do not overlap
//	 9   overlapping, irreducibly
//	 1   nested
//
// The name describes the rarest one. Nesting is rare for a reason that is obvious
// once stated: a region is one WSYNC-to-WSYNC interval, so a nest would have to fit
// two levels of iteration inside a scanline.
//
// Siblings are two plain counted loops one after the other — each a fold the code
// already computes, into a `s.folds` map that is keyed by header and holds as many as
// it is given. So the refusal was about the COUNT of back edges rather than about
// anything the analysis could not do.
//
// WHAT THIS BOUGHT ON THE CORPUS: nothing, measured. Zero regions gained, zero lost.
// The diagnosis of WHY is the more useful half of this change, and it is recorded in
// TestMultiLatchRefusalsRoundToMultipleBackEdgesSoTheDAGCanOverride below.

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestSiblingLoopsFoldAndNestsStillRefuse(t *testing.T) {
	const asm = "../../roms/litmus/litmus_siblingloops.asm"

	before := siblingFolds
	rep, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	// PREMISE — the fixture must actually reach the multi-loop path. Across the whole
	// commercial corpus this counter stays at zero (every multi-latch region fails for
	// a body reason first), so this fixture is the ONLY witness that the code under
	// test runs at all. Without this check, deleting the whole change would leave the
	// rest of this test passing on the DAG walk's answer.
	if got := siblingFolds - before; got < 1 {
		t.Fatalf("no region took the multi-loop fold; the fixture witnesses nothing")
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
	// The fixture's first version wound two counters backwards through $FF, so every
	// interval crossed a frame boundary, every one was dropped, and the whole ROM
	// measured NOTHING — which is indistinguishable from measuring agreement.
	if len(measured) == 0 {
		t.Fatal("no interval was measured at all; the machine side of this test is empty")
	}

	// THE ROW THE CHANGE IS FOR.
	sib, ok := region("SiblingRow")
	if !ok {
		t.Fatal("no SiblingRow region")
	}
	if !sib.Bounded {
		t.Fatalf("SiblingRow is refused (%s) — two counted loops side by side are each "+
			"a fold this code already computes", sib.Reason)
	}
	row, hit := measured[sib.Start]
	if !hit {
		t.Fatal("SiblingRow produced no measured interval, so its bound is graded by nothing")
	}
	if sib.Worst < row.WorstCycles {
		t.Errorf("SiblingRow: proven %d, machine %d — an UNDER-approximation. Relaxing a "+
			"refusal is exactly the kind of change that can introduce one",
			sib.Worst, row.WorstCycles)
	}

	// THE CONTROLS. Both must stay refused: folding a nest's two loops independently
	// would charge the inner one once when the machine runs it once per outer
	// iteration, and an irreducible overlap has no order in which folding one leaves
	// the other a simple counted loop.
	for _, c := range []struct{ name, why string }{
		{"NestedRow", "the inner loop runs once per OUTER iteration, so folding the two " +
			"independently charges it once instead of three times"},
		{"OverlapRow", "neither interval contains the other, so there is no order in which " +
			"folding one leaves the other a simple counted loop"},
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

	// The single-loop path must be untouched: every folded region in the corpus takes
	// it, and this fixture's row proves 36 both before and after.
	if s, ok := region("SingleCtl"); !ok || !s.Bounded || s.Worst != 36 {
		t.Errorf("SingleCtl: bounded=%v worst=%d, expected a bound of 36 — the "+
			"single-loop path must not move", ok && s.Bounded, s.Worst)
	}
}

// TestMultiLatchRefusalsRoundToMultipleBackEdgesSoTheDAGCanOverride carries the
// diagnosis this change produced, which is worth more than the change.
//
// After lifting the latch-count limit, the corpus gained ZERO regions. Every
// multi-latch region still fails, and the census of why says the graph shape was
// never the obstacle:
//
//	branch inside loop body        the large majority, at every latch count
//	trip count unknown             14 of the two-latch regions
//	WSYNC inside loop body         next
//	call or jump inside loop body  next
//
// So "multiple back-edges" was a refusal named after a property that is real but not
// load-bearing. The regions behind it are waiting on `branch inside loop body`, which
// the corpus census counts separately and which this test pins as the true blocker so
// that a future repair there is measured against the right expectation.
//
// WHAT THIS TEST PINS is the rounding, which is counter-intuitive and was learned the
// expensive way. A multi-latch region that cannot fold reports `multipleBackEdges`
// even when the code KNOWS the specific reason — "WSYNC inside loop body", say. The
// first version reported the specific reason, because it is more informative, and it
// cost **6 proven regions**: Barnstorming $F3D4, Chopper Command $FA78 and $FAEC,
// Planet of the Apes $F8B9, Seaquest $F1EC, Stampede $F1A5.
//
// `multipleBackEdges` is the ONE refusal `analyzeRegion` lets the DAG walk override,
// and it is matched by identity. A more precise string is a string the override does
// not match, so six regions the walk had been answering stopped being answered.
// Precision in the message is worth less than the answer it suppresses.
func TestMultiLatchRefusalsRoundToMultipleBackEdgesSoTheDAGCanOverride(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_siblingloops.asm", 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range rep.Unbounded {
		for _, name := range []string{"NestedRow", "OverlapRow"} {
			if len(r.StartLoc) >= len(name) && r.StartLoc[:len(name)] == name {
				seen[name] = true
				if r.Reason != multipleBackEdges {
					t.Errorf("%s is refused for %q, expected exactly multipleBackEdges — the "+
						"DAG override matches that string by identity, and reporting the "+
						"specific body reason instead cost 6 proven regions on the corpus",
						name, r.Reason)
				}
			}
		}
	}
	for _, name := range []string{"NestedRow", "OverlapRow"} {
		if !seen[name] {
			t.Errorf("%s never appeared among the refused regions", name)
		}
	}
}

// TestOverlapsSeparatesSiblingsFromNests pins `overlaps` directly, because no ROM can
// reach it.
//
// Disabling the function (`return false`) leaves every fixture row unchanged: if two
// loops share an instruction then one body holds the other's latch, a latch is a
// branch, and `loopShape` refuses a body with a branch first. So the negative control
// for this guard does not fire, and a test written through a ROM would pass with the
// guard deleted.
//
// That is a reason to test it directly, not a reason to remove it. `branch inside loop
// body` is the largest refusal left on the corpus, and the repair that lifts it makes
// this the only thing that stops a nest from being folded as two independent loops —
// which would charge the inner one ONCE for iterations the machine runs many times.
func TestOverlapsSeparatesSiblingsFromNests(t *testing.T) {
	at := func(a uint16) site { return site{addr: a} }
	mk := func(latchAddr, headerAddr uint16, bodyAddrs ...uint16) loopShape {
		body := map[site]bool{}
		for _, a := range bodyAddrs {
			body[at(a)] = true
		}
		return loopShape{
			latch:  Instr{Addr: latchAddr},
			header: at(headerAddr),
			body:   body,
		}
	}
	for _, c := range []struct {
		name string
		a, b loopShape
		want bool
		why  string
	}{
		{
			name: "siblings: disjoint bodies, one after the other",
			a:    mk(0xF010, 0xF000, 0xF000, 0xF002, 0xF004),
			b:    mk(0xF030, 0xF020, 0xF020, 0xF022),
			want: false,
			why:  "two counted loops in sequence share nothing and each folds on its own",
		},
		{
			name: "nest: the inner loop lives inside the outer body",
			a:    mk(0xF030, 0xF000, 0xF000, 0xF002, 0xF010, 0xF012, 0xF020),
			b:    mk(0xF012, 0xF010, 0xF010),
			want: true,
			why: "folding these independently charges the inner loop once per REGION " +
				"instead of once per outer iteration",
		},
		{
			name: "overlap: B's header is inside A's body, B's latch is not",
			a:    mk(0xF020, 0xF000, 0xF000, 0xF010),
			b:    mk(0xF030, 0xF010, 0xF010, 0xF020),
			want: true,
			why:  "neither contains the other, so no order of folding leaves a simple loop",
		},
		{
			name: "shared header, two exits",
			a:    mk(0xF010, 0xF000, 0xF000),
			b:    mk(0xF020, 0xF000, 0xF000, 0xF010),
			want: true,
			why:  "one header cannot carry two independent trip counts",
		},
		{
			name: "one loop's latch is the other's body instruction",
			a:    mk(0xF010, 0xF000, 0xF000, 0xF002),
			b:    mk(0xF030, 0xF020, 0xF020, 0xF010),
			want: true,
			why:  "A's back edge sits inside B's body, so B's straight-line cost is a lie",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.overlaps(c.b); got != c.want {
				t.Errorf("overlaps = %v, want %v — %s", got, c.want, c.why)
			}
			// The relation must be symmetric: which loop is examined first depends on
			// a map iteration order the caller sorts, and a one-directional answer
			// would make the refusal depend on that sort.
			if got := c.b.overlaps(c.a); got != c.want {
				t.Errorf("overlaps is not symmetric: b.overlaps(a) = %v, want %v", got, c.want)
			}
		})
	}
}
