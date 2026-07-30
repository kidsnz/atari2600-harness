package ramtrace

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/behavmatch"
)

// The behaviour-reproduction plan's central premise — each RAM byte determined by
// itself, the input and at most two companions — was recorded as SUPPORTED BY
// MEASUREMENT on figures of the shape "live 43 / resolved 38 / unresolved 5".
//
// Those figures were carried by a clock. $DA cycles 0..255 every frame (4266
// transitions, deltas only +1 and -255), and keying on a frame counter explains
// any byte perfectly — it memorises a timetable rather than finding a rule. The
// detector missed it because it required the value never to repeat, and a counter
// that wraps repeats.
//
// The corrected figures are pinned HERE rather than left in prose, because prose
// is exactly what let the old ones survive a year of being wrong: four separate
// documents still carried "38/43" today, one of them stating the plan needed no
// rework. A number that decides a plan has to be something a machine re-checks.
func TestOutlawArityFiguresAreWhatThePlanRestsOn(t *testing.T) {
	const rom = "../../../sandbox/studies/outlaw/Outlaw.bin"
	prov, err := NewProvenance("ramtrace arity", rom, "NTSC", "all", 20)
	if err != nil {
		t.Skipf("commercial ROM unavailable: %v", err)
	}
	traces := map[string]*behavmatch.Trace{}
	for _, n := range behavmatch.ScenarioNames() {
		tr, err := behavmatch.Record(rom, "NTSC", behavmatch.Library[n], 20)
		if err != nil {
			t.Skipf("could not record %s: %v", n, err)
		}
		traces[n] = tr
	}
	rep := Arity(prov, traces)

	// The premise is about how MUCH of RAM is explained, so the counts are the
	// claim. Pinned at the measured values; a change means the plan's basis moved
	// and the documents that quote it have to move with it.
	if rep.Live != 44 || rep.Resolved != 23 || rep.Unresolved != 21 {
		t.Errorf("live/resolved/unresolved = %d/%d/%d, want 44/23/21. If this is an IMPROVEMENT, update "+
			"this test AND the four documents that quote the figure (STATUS.md and "+
			"BEHAVIOUR-REPRO-MASTER-PLAN.md x3) — the old 38/43 survived precisely because nothing "+
			"forced them to be updated together.", rep.Live, rep.Resolved, rep.Unresolved)
	}

	// And the reason the old number was wrong: the clock must be named. A silent
	// clock is what made 27 of 35 "resolved" bytes memorisation.
	found := false
	for _, a := range rep.FrameIndexLike {
		if a == "$DA" {
			found = true
		}
	}
	if !found {
		t.Errorf("$DA is a free-running frame counter and must be reported as clock-like; got %v. "+
			"Without that flag a resolution keyed on it reads as a discovered rule.", rep.FrameIndexLike)
	}
	if rep.Memorised != 0 {
		t.Errorf("memorised = %d; a resolution where every key was seen once is not evidence of a rule",
			rep.Memorised)
	}
	t.Logf("live %d / resolved %d / unresolved %d, clock-like %v",
		rep.Live, rep.Resolved, rep.Unresolved, rep.FrameIndexLike)
}
