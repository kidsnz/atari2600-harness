package emu

import "testing"

// TestRESPPairSpacing measures what two consecutive RESP strobes on one line actually put on the
// screen, and settles an apparent conflict between `docs/techniques/sprite-placement.md` rule 1 and
// stella-list 200203/msg00074 (Manuel Polik):
//
//	"STA RESP0 / STA RESP1 in one scannline. With the second RESP, I loose one pixel, so using the
//	 same HMOVE values for both sprites would produce a one pixel gap. ... So I worked out two
//	 tables, that are shifted by one pixel."
//
// Rule 1 (x = 3c - 60) says two `sta zp` strobes 3 cycles apart land exactly 9 clocks apart, on the
// same 3-clock grid, with no one-pixel remainder anywhere — so the source reads like a contradiction.
// It is not one. Polik measures against the ADJACENCY he wants: a player is 8 wide, joining two edge
// to edge needs +8, the hardware hands him +9, and the pixel he "loses" is the single background
// pixel left between them. His second table spends one HMOVE step to close it.
//
// `litmus_p0p1` already performs that exact correction (HMP1=$10) and its comment states the +9 it
// corrects — but it asserts 69/77, the values AFTER the correction. The +9 itself was pinned only by
// inference back through the HMOVE model, so rule 1 and the source were being compared to each other
// rather than to the screen. This asserts the screen.
func TestRESPPairSpacing(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_resp_pair.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	const (
		white = "FFFFFE" // COLUP0 $0E — P0
		red   = "CC2121" // COLUP1 $44 — P1
		bg    = "060606" // COLUBK $00
	)

	// find returns the first run of colour hex on the line, or a zero Clock and false.
	find := func(line int, hex string) (clock, length int, ok bool) {
		runs, _, err := e.ReadRow(line)
		if err != nil {
			t.Fatalf("read row %d: %v", line, err)
		}
		for _, r := range runs {
			if r.Hex == hex {
				return r.Clock, r.Len, true
			}
		}
		return 0, 0, false
	}
	nruns := func(line int) int {
		runs, _, err := e.ReadRow(line)
		if err != nil {
			t.Fatalf("read row %d: %v", line, err)
		}
		return len(runs)
	}
	pair := func(band string, line int) (x0, x1 int) {
		c0, l0, ok0 := find(line, white)
		c1, l1, ok1 := find(line, red)
		if !ok0 || !ok1 {
			t.Fatalf("band %s line %d: expected both players drawn, got P0=%v P1=%v "+
				"(the band is supposed to strobe RESP0 then RESP1)", band, line, ok0, ok1)
		}
		if l0 != 8 || l1 != 8 {
			t.Fatalf("band %s line %d: players are %d and %d wide, want 8 and 8 — NUSIZ must be 0 "+
				"or the spacing below is measured against the wrong width", band, line, l0, l1)
		}
		return c0, c1
	}

	// Band A — HMP0 = HMP1 = $00. Rule 1: the second strobe is 3 cycles later, so +9.
	x0, x1 := pair("A", 49)
	if x0 != 69 {
		t.Errorf("band A: P0 at x=%d, want 69 (`sta RESP0` writes on cycle 43: 3*43-60=69, the "+
			"position litmus_p0p1 measured from the identical prelude)", x0)
	}
	if got := x1 - x0; got != 9 {
		t.Errorf("band A: the two strobes are %d apart, want 9 — rule 1 says a `sta zp` strobe 3 "+
			"cycles later lands 9 clocks right (x = 3c-60). If this is 8 or 10 the rule is wrong "+
			"and stella-list 200203/msg00074 is describing something else", got)
	}
	// Polik's lost pixel, on the screen: 8 wide + ONE background pixel + 8 wide.
	if n := nruns(49); n != 5 {
		t.Errorf("band A: %d runs on the line, want 5 (bg | P0 | ★one bg pixel | P1 | bg)", n)
	}
	if c, l, ok := find(49, bg); !ok || c != 0 {
		t.Errorf("band A: first run is not background (%d,%d,%v)", c, l, ok)
	}
	runs, _, _ := e.ReadRow(49)
	if len(runs) == 5 {
		gap := runs[2]
		if gap.Hex != bg || gap.Clock != 77 || gap.Len != 1 {
			t.Errorf("band A: the run between the players is (clock %d, len %d, %s); want exactly "+
				"one background pixel at clock 77. THAT PIXEL IS Polik's \"I loose one pixel\" — "+
				"the source and rule 1 are the same fact seen from two sides", gap.Clock, gap.Len, gap.Hex)
		}
	}

	// Band B — HMP0 = HMP1 = $70. The same nibble on both: the pair translates, the gap survives.
	bx0, bx1 := pair("B", 63)
	if bx0 != 62 || bx1 != 71 {
		t.Errorf("band B: pair at %d/%d, want 62/71 (both pulled left 7 by HMP=$70)", bx0, bx1)
	}
	if got := bx1 - bx0; got != 9 {
		t.Errorf("band B: spacing %d after an EQUAL HMOVE on both, want 9 — this is exactly why "+
			"Polik needed a second table: one table cannot close a gap it moves along with", got)
	}

	// Band C — HMP1 = $10 only. One HMOVE step of correction: 9 becomes 8 and the gap closes.
	cx0, cx1 := pair("C", 77)
	if cx0 != 69 || cx1 != 77 {
		t.Errorf("band C: pair at %d/%d, want 69/77 — the values litmus_p0p1 asserts", cx0, cx1)
	}
	if got := cx1 - cx0; got != 8 {
		t.Errorf("band C: spacing %d, want 8 — a single HMOVE step (HMP1=$10, left 1) is the whole "+
			"content of Polik's \"two tables, shifted by one pixel\"", got)
	}
	if n := nruns(77); n != 4 {
		t.Errorf("band C: %d runs, want 4 (bg | P0 | P1 | bg) — no background between the players; "+
			"this is the seam litmus_p0p1 tests, reached here by correcting the +9", n)
	}

	// Band D — HMP1 = $F0 (right 1). The spacing is a free parameter, not a fixed +9 or +8.
	dx0, dx1 := pair("D", 91)
	if dx0 != 69 || dx1-dx0 != 10 {
		t.Errorf("band D: pair at %d/%d (spacing %d), want 69/79 (spacing 10)", dx0, dx1, dx1-dx0)
	}

	// The strobe sets the base and only the nibble moves P1: A, C and D share one P0.
	if !(x0 == cx0 && cx0 == dx0) {
		t.Errorf("bands A/C/D put P0 at %d/%d/%d — they run the identical prelude, so P0 must not "+
			"move; if it does, the spacing differences above are not attributable to HMP1", x0, cx0, dx0)
	}
}
