package emu

import "testing"

// TestLitmusColorSteppingIsEveryTwoLines brings litmus_color into the regression
// net and corrects what it claimed.
//
// The ROM writes `stx COLUBK` once per visible line with x counting 191..0, and its
// header said that makes "each line a different single colour". It does not: the TIA
// masks bit 0 off every colour register — Gopher2600's video.go writes
// `setBackground(data.Value & 0xfe)` — so stepping the register by 1 changes the
// COLOUR only every second line.
//
// Measured over the readable window (absolute scanlines 29..242, 214 rows): every row
// is a single uniform run, and there are 95 distinct colours across them, not 214 —
// a ratio of 0.44, with 117 rows repeating the row above.
//
// ★The first version of this comment said "29..239, 211 rows". That was my measuring
// loop stopping at 240, not the ROM: the visible window is 29..242 and ReadRow accepts
// all of it. The conclusion (colour changes every second line) was unaffected, the
// window was not. See TestRowCoordinateSystemIsOne, which now pins the window itself.
// That halving is the fact an author needs: a vertical gradient driven by a counter
// has at most 128 steps, and stepping by 1 wastes half of them.
func TestLitmusColorSteppingIsEveryTwoLines(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_color.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	var cols []string
	nonUniform := 0
	for sl := 0; sl < 300; sl++ { // 300, not 240: the window ends at 242 and a short loop under-counts it
		runs, w, err := e.ReadRow(sl)
		if err != nil || len(runs) == 0 {
			continue
		}
		if len(runs) != 1 || runs[0].Len != w {
			nonUniform++
			continue
		}
		cols = append(cols, runs[0].Hex)
	}

	if len(cols) < 150 {
		t.Fatalf("only %d readable rows; the ROM is not rendering and this test would prove nothing",
			len(cols))
	}
	// No playfield, no sprites: every row must be one run edge to edge. That is the
	// property that makes this ROM a clean colour probe at all.
	if nonUniform != 0 {
		t.Errorf("%d rows were not a single full-width run; litmus_color must draw background only",
			nonUniform)
	}

	distinct := map[string]bool{}
	for _, c := range cols {
		distinct[c] = true
	}
	if len(distinct) < 60 {
		t.Errorf("only %d distinct colours over %d rows — the vertical gradient is not happening",
			len(distinct), len(cols))
	}
	// The halving. Allowing a wide band because the top and bottom of the window sit
	// in blanking and repeat, but a per-LINE gradient would put this near 1.0.
	ratio := float64(len(distinct)) / float64(len(cols))
	if ratio > 0.75 {
		t.Errorf("%d distinct colours over %d rows (%.2f) — that is close to one colour per line, but "+
			"the TIA masks bit 0 off COLUBK (video.go: setBackground(data.Value & 0xfe)), so a counter "+
			"stepping by 1 can only change colour every second line", len(distinct), len(cols), ratio)
	}
	if ratio < 0.25 {
		t.Errorf("%d distinct colours over %d rows (%.2f) — far fewer than the every-second-line stepping "+
			"predicts; something else is flattening the gradient", len(distinct), len(cols), ratio)
	}
	t.Logf("%d readable rows, all uniform, %d distinct colours (%.2f per row)", len(cols), len(distinct), ratio)
}
