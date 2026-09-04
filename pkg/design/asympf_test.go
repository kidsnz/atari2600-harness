package design

import "testing"

// TestAsymPFRepositionBudget locks the derivation behind Thomas Jentzsch's sentence in stella-list
// 200409/msg00258: "With assymetrical, non striped playfields, you won't be able to reposition at
// all." Found by the mailing-list distillation (helper-2), who noticed harness held both halves of
// the arithmetic — the six rewrite windows and the one strobe cycle rule 1 demands — and had never
// written the consequence down.
//
// The table below is the whole finding, and the interesting row is the FIRST one: with the
// playfield and nothing else, every position is still reachable with 23 cycles to spare. **The
// playfield alone does not cost the reposition.** It is the graphics writes on top of it that close
// the window, and they close it fast — one object costs 4 positions, two costs 37, three costs all
// 53. Jentzsch was writing Jumpman, which puts several objects on a line, so his "at all" is exact
// for his kernel and an overstatement for a bare playfield. That distinction is not in the source
// and it is the part worth having: it says which escape buys what.
func TestAsymPFRepositionBudget(t *testing.T) {
	want := map[int]int{0: 53, 1: 49, 2: 16, 3: 0}
	for n, count := range want {
		got := AsymPFReachableX(n)
		if len(got) != count {
			t.Errorf("with %d graphics writes: %d reachable positions, want %d (%v)",
				n, len(got), count, got)
		}
	}

	// Three objects is the sentence, literally: nothing left.
	if got := AsymPFReachableX(3); len(got) != 0 {
		t.Errorf("three graphics writes must leave NO reachable position — that is the claim being "+
			"reproduced — got %v", got)
	}

	// Two objects is the interesting middle: what survives is all on the right. A strobe late in
	// the line is the only one that misses every playfield window, so the reachable positions are
	// the ones a late strobe produces.
	two := AsymPFReachableX(2)
	if len(two) == 0 {
		t.Fatal("two graphics writes must still leave some reachable positions")
	}
	if two[0] < 100 {
		t.Errorf("with two graphics writes the surviving positions should all be on the RIGHT "+
			"(a late strobe is the only one that clears the windows); leftmost is %d: %v", two[0], two)
	}

	// Rule 1's grid must survive: every reachable x is 3c-60 for an integer c.
	for _, x := range AsymPFReachableX(0) {
		if (x+60)%3 != 0 {
			t.Errorf("x=%d is not on the 3-clock strobe grid", x)
		}
	}

	// Negative control on the cost model rather than on the answer: if a playfield write were free,
	// the budget would not bind and three objects would fit. This fails if someone "simplifies"
	// AsymPFLineFits into something that ignores its inputs and always says yes.
	if AsymPFLineFits(12, 40) {
		t.Error("twelve graphics writes cannot fit in 76 cycles; AsymPFLineFits is not counting")
	}
	// And the other direction: an empty line must fit at a cycle inside every playfield window.
	if !AsymPFLineFits(0, 40) {
		t.Error("a bare asymmetric playfield plus one strobe must fit; it uses 53 of 76 cycles")
	}
}
