package place

import "testing"

// row builds the shapes one PASS of a staggered row draws: every other letter, so they are twice
// the pitch apart. All of them solid on both halves, which is what a 2 px-grid glyph gives.
func row(x0, pitch, n int) []Shape {
	var out []Shape
	for k := 0; k < n; k += 2 {
		out = append(out, Shape{X: x0 + k*pitch, SolidLeft: true, SolidRight: true})
	}
	return out
}

func TestPlacesTheRowThatFitsOnTheGrid(t *testing.T) {
	// Ten shapes 16 apart starting at 3: the pass drawn on the even lines is five of them, 32
	// apart, and every one of those sits on the player grid. This is the easy case and it must
	// come out head-first, because nothing forces anything else.
	p, err := Solve(row(3, 16, 10), 0, 0)
	if err != nil {
		t.Fatalf("x0=3 should place: %v", err)
	}
	for i, s := range p.Splits {
		if s != HeadFirst {
			t.Errorf("shape %d came out %s; nothing at x0=3 needs turning round", i, s)
		}
	}
}

func TestPlacesTheRowThatStartsWhereNoPlayerCanBe(t *testing.T) {
	// The same row one pixel further left. x=2 is on the missile grid and not the player's, and it
	// is left of the player's floor as well, so the leftmost shape can only be drawn by turning it
	// round. That the row places AT ALL is the whole point of this package: worked out by hand it
	// came back "impossible" twice.
	shapes := row(2, 16, 10)
	p, err := Solve(shapes, 0, 0)
	if err != nil {
		t.Fatalf("x0=2 should place: %v", err)
	}
	if p.Splits[0] != MissileFirst {
		t.Errorf("the shape at x=2 came out %s; no player can be strobed to 2", p.Splits[0])
	}
	if err := Validate(shapes, p); err != nil {
		t.Errorf("the plan does not check out: %v", err)
	}
}

func TestRefusesWhatTheGridCannotDo(t *testing.T) {
	// Negative control. Same position, but the shape is solid on neither half, so it needs a
	// player at x=2 -- and no strobe, and no copy of one, reaches x=2 with a player.
	_, err := Solve([]Shape{{X: 2}}, 0, 0)
	if err == nil {
		t.Fatal("a shape at x=2 with no solid half has no placement, but one was returned")
	}
}

func TestClampIsAWindowNotAPoint(t *testing.T) {
	// The measurement the first version of this search was missing. If the floor were reachable
	// from one cycle only, the two strobes below would be one cycle apart and the row could not be
	// placed; the window is what makes it possible.
	cs := cyclesFor(SolidFloor, true, ClampFirst, 72)
	if len(cs) < 2 {
		t.Fatalf("the solid floor should be reachable from several write cycles, got %v", cs)
	}
	if got := cyclesFor(SolidFloor+3, true, ClampFirst, 72); len(got) != 1 {
		t.Errorf("above the floor a position is one cycle, got %v", got)
	}
}
