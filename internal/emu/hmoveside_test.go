package emu

import "testing"

// Grading for `roms/litmus/litmus_hmove_side.asm` — HMOVE's two side effects.
//
// The ROM has existed since V2-2 and its header records what it measured, but nothing
// graded it: it was carried only as ceiling corpus, so the numbers in that header were
// a comment, not a check. `docs/fundamentals-audit.md` marked the mechanism 📖 (its own
// legend: "stated by a document, NOT measured by our litmus ROMs") and
// `docs/design-principles.md` marked the comb half `(needs litmus)`. Both were right.
//
// Three claims, from Towers' *TIA Hardware Notes* by way of that header:
//
//   (a) HMOVE struck right after WSYNC extends HBLANK by 8 colour clocks, so the line
//       carries a black comb over its leftmost 8 pixels — even with every HMxx at zero.
//   (b) HMOVE struck mid-visible displaces nothing (the ripple counter has already run).
//   (c) HMOVE struck at the end of the line moves by the nibble PLUS 8: HMP0 = $10 is
//       one clock left normally, nine clocks left here.
//
// (c) is the one a kernel gets wrong silently: the object still moves, just eight clocks
// further than the author asked for.

const (
	hsInk = "FFFFFE" // COLUP0 = $0E, the P0 bar

	// Band geometry, measured from the rendered frame and pinned by
	// TestHmoveSideFrameGeometry. Bands A (32 lines), B (16) and C (16) run back to
	// back and share one fixed P0; band D follows and sweeps.
	hsBandsStart = 7   // first picture row the P0 bar is drawn on
	hsALines     = 32  // band A: HMOVE right after WSYNC, every second line
	hsABCLines   = 64  // A + B + C, all with HMxx zero
	// Band D's first readable pair. Rows 73-74 catch the band mid-transition (P0 reads
	// 0 there), so the sweep is counted from the first fully held pair.
	hsDStart     = 75
	// Band D's loop runs sixteen times, but its LAST strobe is not comparable with the
	// others: on that iteration `dey` clears Y, `bne` falls through (2 cycles, not 3) and
	// `sta HMCLR` follows immediately, so the strobe does not land at the same point in
	// the line. Measured, that band moves -8 instead of -9. Fourteen readable bands give
	// thirteen uniform steps, which is what is graded; the exit band is asserted
	// separately so the difference is recorded rather than hidden.
	hsDBands     = 14  // readable 2-line bands in D, excluding the loop-exit band
	hsDExitStep  = -8  // the loop-exit band, measured
	hsDStep      = -9  // nibble $10 (one clock left) + the late-HMOVE 8 = nine left
	hsFixedX     = 9   // where P0 sits while HMxx is zero
	hsCombWidth  = 8   // the comb the extended HBLANK paints
)

func loadHmoveSide(t *testing.T) (*Emu, int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove_side.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	return e, top
}

// hsRow reports whether the leftmost hsCombWidth pixels are all background, and where
// the P0 bar starts (-1 if it is not on this line).
func hsRow(t *testing.T, e *Emu, row int) (combed bool, x int) {
	t.Helper()
	runs, _, err := e.ReadRow(row)
	if err != nil {
		t.Fatalf("read row %d: %v", row, err)
	}
	combed, x = true, -1
	for _, r := range runs {
		if r.Clock < hsCombWidth && r.Hex != "000000" && r.Hex != "060606" {
			combed = false
		}
		if r.Hex == hsInk && x < 0 {
			x = r.Clock
		}
	}
	return
}

// TestHmoveSideCombAppearsOnlyOnStrobedLines is claim (a). The fixture strobes on every
// other line with every HMxx at zero, so the comb has to alternate: it is the extended
// HBLANK that paints it, not any motion.
func TestHmoveSideCombAppearsOnlyOnStrobedLines(t *testing.T) {
	e, top := loadHmoveSide(t)
	var pattern []bool
	for i := 0; i < hsALines; i++ {
		c, _ := hsRow(t, e, top+hsBandsStart+i)
		pattern = append(pattern, c)
	}
	on, breaks, firstBreak := 0, 0, -1
	for i, c := range pattern {
		if c {
			on++
		}
		if i > 0 && c == pattern[i-1] {
			breaks++
			if firstBreak < 0 {
				firstBreak = i
			}
		}
	}
	if breaks != 0 {
		t.Errorf("the comb does not alternate: %d lines repeat their neighbour (first at line %d of "+
			"the band). Every second line strobes HMOVE with HMxx = 0, so the comb must appear on "+
			"exactly those and no others", breaks, firstBreak)
	}
	if on*2 < hsALines-2 || on*2 > hsALines+2 {
		t.Errorf("%d of %d lines carry the comb; strobing every second line means about half",
			on, hsALines)
	}
	// The other half of the claim, and the reason band A alone is checked above: HMOVE
	// struck mid-visible does NOT extend HBLANK, so bands B and C carry no comb at all.
	// A test that looked for alternation across all 64 lines would fail here, and the
	// first version of this test did exactly that.
	late := 0
	for i := hsALines; i < hsABCLines; i++ {
		if c, _ := hsRow(t, e, top+hsBandsStart+i); c {
			late++
		}
	}
	if late != 0 {
		t.Errorf("%d of the %d mid-visible lines carry the comb — only a strobe that lands before "+
			"HBLANK ends extends it", late, hsABCLines-hsALines)
	}
	t.Logf("comb on %d of band A's %d lines, strictly alternating; %d of the %d mid-visible lines "+
		"carry it — the extended HBLANK paints it, with every HMxx at zero",
		on, hsALines, late, hsABCLines-hsALines)
}

// TestHmoveSideMidVisibleHmoveDisplacesNothing is claim (b), and it is also the control
// that keeps (a) honest: if the strobe were moving the object, the comb test above could
// not tell an extended HBLANK from an object that had walked off the left edge.
func TestHmoveSideMidVisibleHmoveDisplacesNothing(t *testing.T) {
	e, top := loadHmoveSide(t)
	moved, first := 0, -1
	for i := 0; i < hsABCLines; i++ {
		row := top + hsBandsStart + i
		_, x := hsRow(t, e, row)
		if x < 0 {
			continue
		}
		if x != hsFixedX {
			moved++
			if first < 0 {
				first = i
			}
		}
	}
	if moved != 0 {
		t.Errorf("P0 left clock %d on %d of %d lines (first at line %d) — bands A, B and C strobe "+
			"HMOVE after WSYNC and mid-visible, and neither may displace anything",
			hsFixedX, moved, hsABCLines, first)
	}
	t.Logf("P0 held clock %d across all %d lines of bands A, B and C", hsFixedX, hsABCLines)
}

// TestHmoveSideLateHmoveAddsEight is claim (c) — the silent one. HMP0 = $10 asks for one
// clock left; struck at the end of the line it delivers nine.
func TestHmoveSideLateHmoveAddsEight(t *testing.T) {
	e, top := loadHmoveSide(t)
	var xs []int
	for b := 0; b < hsDBands; b++ {
		_, x := hsRow(t, e, top+hsDStart+b*2)
		if x < 0 {
			t.Fatalf("band D line %d: P0 is not drawn", b)
		}
		xs = append(xs, x)
	}
	for i := 1; i < len(xs); i++ {
		if d := xs[i] - xs[i-1]; d != hsDStep {
			t.Errorf("band D step %d: P0 moved %+d, want %+d (HMP0 = $10 is %+d, and a late HMOVE "+
				"adds 8 more to the left)", i, d, hsDStep, hsDStep+8)
			break
		}
	}
	if got := xs[len(xs)-1] - xs[0]; got != hsDStep*(len(xs)-1) {
		t.Errorf("band D travelled %+d over %d strobes, the %+d-per-strobe rule says %+d",
			got, len(xs)-1, hsDStep, hsDStep*(len(xs)-1))
	}
	// The nibble alone would give -1 per strobe. If the +8 were absent the whole band
	// would fit in 14 clocks instead of 126, so this bound cannot be met by accident.
	if span := xs[0] - xs[len(xs)-1]; span < 100 {
		t.Errorf("band D spans only %d clocks; %+d per strobe over %d strobes needs %d — without "+
			"the late-HMOVE +8 the nibble alone would give %d",
			span, hsDStep, len(xs)-1, -hsDStep*(len(xs)-1), len(xs)-1)
	}
	// The loop-exit band, recorded rather than dropped: `bne` falls through there, so the
	// strobe lands one cycle earlier and the object moves one clock less.
	_, exitX := hsRow(t, e, top+hsDStart+hsDBands*2)
	if exitX < 0 {
		t.Fatalf("the loop-exit band of D is not drawn (row %d)", top+hsDStart+hsDBands*2)
	}
	if d := exitX - xs[len(xs)-1]; d != hsDExitStep {
		t.Errorf("band D's loop-exit strobe moved %+d, want %+d — the exit path is one cycle "+
			"shorter than the loop body and that difference is part of the measurement",
			d, hsDExitStep)
	}
	t.Logf("band D: %d uniform strobes, %d -> %d, exactly %+d each (nibble %+d plus the "+
		"late-HMOVE 8); the loop-exit strobe moves %+d and lands at %d",
		len(xs), xs[0], xs[len(xs)-1], hsDStep, hsDStep+8, hsDExitStep, exitX)
}

// TestHmoveSideFrameGeometry pins the window every reading above is indexed from.
func TestHmoveSideFrameGeometry(t *testing.T) {
	e, top := loadHmoveSide(t)
	n, err := e.StepFrame()
	if err != nil {
		t.Fatal(err)
	}
	if n != 262 {
		t.Errorf("frame is %d scanlines, want 262", n)
	}
	if _, x := hsRow(t, e, top+hsBandsStart-1); x >= 0 {
		t.Errorf("P0 is already drawn one row above the bands (row %d, clock %d) — every index "+
			"below is off", top+hsBandsStart-1, x)
	}
	if _, x := hsRow(t, e, top+hsDStart+hsDBands*2+4); x != 26 {
		t.Errorf("past band D, P0 sits at %d, want 26 (the last swept position, held through the "+
			"filler) — the band count is wrong", x)
	}
}
