package behavmatch

import (
	"strings"
	"testing"
)

// trace builds a Trace whose objects move exactly as described, so a role can be
// asserted against something known rather than against a ROM.
func trace(name string, moves map[int][2]bool) *Trace {
	tr := &Trace{Scenario: name}
	for f := 0; f < 40; f++ {
		s := Sample{X: map[int]int{}, YTop: map[int]int{}, Height: map[int]int{},
			Present: map[int]bool{}}
		for idx, mv := range moves {
			x, y := 50, 60
			if mv[0] {
				x += f
			}
			if mv[1] {
				y += f
			}
			s.X[idx], s.YTop[idx], s.Height[idx], s.Present[idx] = x, y, 8, true
		}
		tr.Samples = append(tr.Samples, s)
	}
	return tr
}

func TestRolesAreDerivedFromWhatAnObjectDoes(t *testing.T) {
	tr := trace("t", map[int][2]bool{
		0: {false, true},  // vertical -- a paddle
		1: {true, false},  // horizontal
		2: {true, true},   // free -- a ball
		3: {false, false}, // static
	})
	got := ClassifyRoles(tr, []int{0, 1, 2, 3})
	for idx, want := range map[int]Role{0: RoleVert, 1: RoleHoriz, 2: RoleFree, 3: RoleStatic} {
		if got[idx] != want {
			t.Errorf("object %d classified %q, want %q", idx, got[idx], want)
		}
	}
}

// THE WITNESS, and the case that actually occurred. Video Olympics puts the ball on the
// BALL object and the paddles on the two players; the reproduction puts the ball on
// PLAYER 0. Comparing P0 with P0 compares a paddle with a ball, and every axis line
// comes back "MECHANIC DIFF" -- eight confident statements, all false, in a shape that
// reads exactly like a finding.
func TestAPaddleIsNotComparedWithABall(t *testing.T) {
	target := trace("s", map[int][2]bool{0: {false, true}, 4: {true, true}}) // paddle on P0, ball on BL
	mine := trace("s", map[int][2]bool{0: {true, true}, 4: {false, false}})  // ball on P0, BL unused

	mism := CompareRoles(target, mine, []int{0, 4})
	if len(mism) != 2 {
		t.Fatalf("%d role mismatch(es) between a paddle-on-P0 ROM and a ball-on-P0 ROM, want 2: %+v", len(mism), mism)
	}

	d := CompareTraces(target, mine, []int{0, 4}, 0.6)
	if d.Match {
		t.Error("the comparison passed two ROMs that use the objects for different things")
	}
	if len(d.RoleMismatch) == 0 {
		t.Error("Diff does not carry the role mismatch, so a caller cannot tell this refusal " +
			"apart from a real behavioural difference")
	}
	joined := strings.Join(d.Lines, "\n")
	if !strings.Contains(joined, "NOT COMPARABLE") {
		t.Errorf("the report does not say the comparison is impossible:\n%s", joined)
	}
	// and it must NOT go on to print the per-axis table, which is the whole point
	if strings.Contains(joined, "MECHANIC DIFF") {
		t.Errorf("the per-object differences were printed anyway — that table is the falsehood "+
			"this gate exists to suppress:\n%s", joined)
	}
}

// The negative control. Two ROMs that DO agree on what each object is must still get
// the full comparison: a gate that refuses everything is worth nothing.
func TestMatchingRolesStillGetTheFullComparison(t *testing.T) {
	target := trace("s", map[int][2]bool{0: {false, true}, 4: {true, true}})
	mine := trace("s", map[int][2]bool{0: {false, true}, 4: {true, true}})

	if mism := CompareRoles(target, mine, []int{0, 4}); len(mism) != 0 {
		t.Fatalf("identical traces reported %d role mismatch(es): %+v", len(mism), mism)
	}
	d := CompareTraces(target, mine, []int{0, 4}, 0.6)
	if !d.Match {
		t.Errorf("identical traces did not match:\n%s", strings.Join(d.Lines, "\n"))
	}
	joined := strings.Join(d.Lines, "\n")
	if !strings.Contains(joined, "mechanic ok") {
		t.Errorf("the per-object table was not produced for two ROMs that agree:\n%s", joined)
	}
}

// An object one ROM draws and the other never draws is the same failure in its starkest
// form, and it must not read as "both are at rest".
func TestAnUnusedObjectIsAMismatchAndNotAgreement(t *testing.T) {
	target := trace("s", map[int][2]bool{0: {false, true}})
	mine := trace("s", map[int][2]bool{}) // draws nothing at all

	mism := CompareRoles(target, mine, []int{0})
	if len(mism) != 1 {
		t.Fatalf("an object the build never draws was not flagged: %+v", mism)
	}
	if mism[0].Mine != RoleAbsent {
		t.Errorf("mine's role is %q, want %q", mism[0].Mine, RoleAbsent)
	}
}
