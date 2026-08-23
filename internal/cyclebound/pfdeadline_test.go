package cyclebound

import (
	"strings"
	"testing"
)

const (
	pfOnTime = "../../roms/litmus/pf_ontime.asm"
	pfLate   = "../../roms/litmus/pf_late.asm"
)

// The witness. pf_late puts three cycles of index arithmetic at the TOP of the line, and
// every playfield write in that line lands after the beam has already drawn the pixels it
// governs. This is the defect that shipped: the picture came out shifted right with the
// previous line's right edge wrapping in at the left.
func TestALateWriteIsCaughtAndItsShiftIsQuantified(t *testing.T) {
	rep, err := CheckPFDeadlines(pfLate)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Late) == 0 {
		t.Fatalf("pf_late has no late writes, but every store in its line is three cycles "+
			"behind the beam; checked %d, not-ours %d, unjudged %d", rep.Checked, rep.NotOurs, rep.Unjudged)
	}
	w := rep.Late[0]
	if w.Reg != "PF1" || w.Deadline != 16 {
		t.Errorf("worst late write is %s against a deadline of %d; it should be PF1 against 16, "+
			"the clock the beam reaches column 4", w.Reg, w.Deadline)
	}
	if w.LateBy != 15 {
		t.Errorf("worst write is %d colour clocks late; it measured 15 when this witness was "+
			"built, and the number is the whole point of the check", w.LateBy)
	}
	// The report has to say what the author will SEE, not just that a number is large.
	if got := rep.Summary(); got == "" || !strings.Contains(got, "shifts the picture right by 4 column(s)") {
		t.Errorf("summary does not translate the miss into columns of shift: %q", got)
	}
}

// The negative control. pf_ontime does the SAME arithmetic on the SAME data and draws the
// same 40 columns -- it just does it in the line's tail. If this fails, the check is
// flagging the shape of the kernel rather than the timing, and it would condemn every
// asymmetric playfield in the tree.
func TestTheSameKernelWithTheArithmeticMovedIsClean(t *testing.T) {
	rep, err := CheckPFDeadlines(pfOnTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Late) != 0 {
		t.Fatalf("pf_ontime reported %d late write(s); the first is %s at clock %d against %d. "+
			"This build meets every deadline and a false positive here makes the check unusable",
			len(rep.Late), rep.Late[0].Reg, rep.Late[0].MaxClock, rep.Late[0].Deadline)
	}
	if rep.Checked < 6 {
		t.Fatalf("only %d write(s) were checked; a 40-column line writes PF0/PF1/PF2 twice, "+
			"so a low count means the check passed by looking at nothing", rep.Checked)
	}
	if rep.Tightest == nil {
		t.Error("no tightest write reported; without it a build that is one clock from failing " +
			"looks the same as one with room to spare")
	}
}

// The reason this file exists at all: the EXISTING prover cannot tell these two apart.
// Both fit inside 76 cycles, so both are CERTIFIED. If this ever fails -- if Prove starts
// rejecting pf_late -- the deadline check is redundant and should be deleted rather than
// maintained.
func TestTheCycleBudgetProverPassesBothIncludingTheBrokenOne(t *testing.T) {
	for _, asm := range []string{pfOnTime, pfLate} {
		rep, err := Prove(asm, 76)
		if err != nil {
			t.Fatalf("%s: %v", asm, err)
		}
		if !rep.Certified {
			t.Errorf("%s is not certified by the cycle-budget prover; this test asserts that "+
				"fitting in 76 cycles is a DIFFERENT question from landing before the beam, "+
				"and it only means anything while both builds fit", asm)
		}
	}
}

// The deadlines are the machine's geometry, so they are pinned rather than left to drift
// with whatever the code happens to say.
func TestTheDeadlineTableMatchesThePlayfieldGeometry(t *testing.T) {
	for _, c := range []struct {
		reg  string
		nth  int
		want int
		ok   bool
	}{
		{"PF0", 1, 0, true},  // columns 0-3   start at clock 0
		{"PF1", 1, 16, true}, // columns 4-11  start at clock 16
		{"PF2", 1, 48, true}, // columns 12-19 start at clock 48
		{"PF0", 2, 80, true}, // the right half repeats at 80
		{"PF1", 2, 96, true},
		{"PF2", 2, 128, true},
		{"PF0", 3, 0, false}, // a third write is beyond what this can judge
		{"COLUPF", 1, 0, true},
		{"COLUPF", 2, 0, false},
		{"GRP0", 1, 0, false}, // objects are not on a column grid; not this check's business
	} {
		got, ok := pfDeadlineFor(c.reg, c.nth)
		if ok != c.ok {
			t.Errorf("pfDeadlineFor(%s, %d) checkable = %v, want %v", c.reg, c.nth, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("pfDeadlineFor(%s, %d) = %d, want %d", c.reg, c.nth, got, c.want)
		}
	}
}

// Writes past the second, and registers with no rule, must be COUNTED as unchecked rather
// than folded into the pass. A summary that says "all land in time" while silently
// ignoring half the writes is the exact failure mode this whole session was about.
func TestUncheckedWritesAreReportedNotAbsorbed(t *testing.T) {
	rep, err := CheckPFDeadlines(pfOnTime)
	if err != nil {
		t.Fatal(err)
	}
	if rep.NotOurs == 0 && rep.Unjudged == 0 {
		t.Skip("this build happens to have no unjudgeable writes")
	}
	sum := rep.Summary()
	if rep.NotOurs > 0 && !strings.Contains(sum, "no playfield deadline governs") {
		t.Errorf("%d write(s) fall outside these rules but the summary does not say so: %q",
			rep.NotOurs, sum)
	}
	if rep.Unjudged > 0 && !strings.Contains(sum, "NOT JUDGED") {
		t.Errorf("%d PLAYFIELD write(s) were not judged but the summary does not say so: %q",
			rep.Unjudged, sum)
	}
}

// The two counts must not be summed back together, whatever the wording. They mean opposite
// things to a reader: NotOurs is "this rule does not apply here", Unjudged is "this rule applies
// and nobody checked". A combined figure was read as a coverage hole in the playfield check on
// 2026-08-23, and the first attempt to reword it called the whole count "non-playfield" — which
// is flatly false of a third PF0 write. The classifier is what keeps them apart, so it is what
// this test pins.
func TestNotOursAndUnjudgedAreDifferentQuestions(t *testing.T) {
	for _, c := range []struct {
		reg       string
		playfield bool
	}{
		{"PF0", true}, {"PF1", true}, {"PF2", true}, {"COLUPF", true}, {"COLUBK", true},
		{"GRP0", false}, {"GRP1", false}, {"ENAM1", false}, {"NUSIZ0", false}, {"COLUP1", false},
	} {
		if got := isPlayfieldReg(c.reg); got != c.playfield {
			t.Errorf("isPlayfieldReg(%s) = %v, want %v", c.reg, got, c.playfield)
		}
	}
	// The pair that matters: a third PF0 write has no deadline AND is a playfield write, so it
	// must land in Unjudged. If it ever lands in NotOurs, the verdict says "not our business"
	// about a playfield write it did not check.
	if _, ok := pfDeadlineFor("PF0", 3); ok {
		t.Fatal("a third PF0 write is judgeable now; this test's premise is stale")
	}
	if !isPlayfieldReg("PF0") {
		t.Error("a third PF0 write would be counted as somebody else's register")
	}
}
