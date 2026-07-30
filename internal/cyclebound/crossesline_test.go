package cyclebound

import (
	"path/filepath"
	"testing"
)

// findWrite returns the proven window for one PC in one ROM.
func findWrite(t *testing.T, asm, pc string) BeamWrite {
	t.Helper()
	br, err := BeamIntervals(asm)
	if err != nil {
		t.Fatalf("%s: %v", asm, err)
	}
	for _, rg := range br.Regions {
		for _, w := range rg.Writes {
			if w.PC == pc {
				return w
			}
		}
	}
	t.Fatalf("%s: no write at %s", asm, pc)
	return BeamWrite{}
}

// TestInvertedWindowMeansTheWriteCrossesALine pins the one direction of crosses_line
// that can be checked without restating the formula that computes it.
//
// Folding a window into per-line clocks subtracts the SAME multiple of 228 from both
// endpoints as long as they sit on one line, so within a line the folded pair stays in
// order. Therefore min_clock > max_clock is possible only when the endpoints are on
// different lines — an inverted window is a proof of a crossing, and must be flagged.
//
// It was not. crosses_line was computed as (minAbs+68)/228 != (maxAbs+68)/228, which
// puts the boundary at clock 92 of each line instead of at the line's start. Measured
// over 127 ROMs and 1016 writes: 43 flags raised, 35 of them wrong, and 11 real
// crossings missed — 81% of everything this field said was false, in both directions.
func TestInvertedWindowMeansTheWriteCrossesALine(t *testing.T) {
	var roms []string
	for _, d := range []string{"../../roms/techniques", "../../roms/litmus", "../../roms/exerciser"} {
		m, _ := filepath.Glob(d + "/*.asm")
		roms = append(roms, m...)
	}
	if len(roms) < 50 {
		t.Fatalf("only %d ROMs found — the corpus this is graded on is missing", len(roms))
	}

	inverted, unflagged := 0, 0
	for _, rom := range roms {
		br, err := BeamIntervals(rom)
		if err != nil || br == nil {
			continue // bank-switched images decline; not this test's subject
		}
		for _, rg := range br.Regions {
			for _, w := range rg.Writes {
				if w.MinClock <= w.MaxClock {
					continue
				}
				inverted++
				if !w.CrossesLine {
					unflagged++
					t.Errorf("%s %s %s: window [%d..%d] is inverted, so its endpoints are on "+
						"different scanlines, but crosses_line is false (min_abs=%d max_abs=%d)",
						filepath.Base(rom), w.PC, w.Reg, w.MinClock, w.MaxClock, w.MinAbs, w.MaxAbs)
				}
			}
		}
	}
	// Non-vacuity: if the corpus stops containing inverted windows this test proves
	// nothing, and would go on passing forever.
	if inverted < 10 {
		t.Fatalf("only %d inverted windows in the whole corpus — too few for this to be a check",
			inverted)
	}
	t.Logf("%d inverted windows across %d ROMs, %d unflagged", inverted, len(roms), unflagged)
}

// TestCrossesLineOnMeasuredCases pins one write from each direction of the old bug, by
// address, so a future change to the boundary arithmetic has to break a named case
// rather than a summary statistic.
func TestCrossesLineOnMeasuredCases(t *testing.T) {
	// Was MISSED: the window runs off the end of the line, and the folded pair inverts.
	crossing := findWrite(t, "../../roms/techniques/bullets.asm", "$F108")
	if crossing.MinAbs != 198 || crossing.MaxAbs != 276 {
		t.Fatalf("bullets $F108 window moved to [%d..%d] — repin this case",
			crossing.MinAbs, crossing.MaxAbs)
	}
	if !crossing.CrossesLine {
		t.Errorf("bullets $F108 %s: min_abs=198 max_abs=276 straddles the 228 boundary "+
			"(clock [%d..%d]), and it is not flagged", crossing.Reg, crossing.MinClock, crossing.MaxClock)
	}
	if crossing.MinClock <= crossing.MaxClock {
		t.Errorf("bullets $F108: folded window [%d..%d] is no longer inverted — repin",
			crossing.MinClock, crossing.MaxClock)
	}

	// Was FALSELY flagged: the whole window sits inside one line. Warning about this
	// one tells a kernel author the scanline depends on the path when it does not.
	sameLine := findWrite(t, "../../roms/techniques/bullets.asm", "$F0FB")
	if sameLine.MinAbs != 150 || sameLine.MaxAbs != 222 {
		t.Fatalf("bullets $F0FB window moved to [%d..%d] — repin this case",
			sameLine.MinAbs, sameLine.MaxAbs)
	}
	if sameLine.CrossesLine {
		t.Errorf("bullets $F0FB %s: min_abs=150 max_abs=222 is entirely within the first line "+
			"(clock [%d..%d], in order), but it is flagged as crossing one",
			sameLine.Reg, sameLine.MinClock, sameLine.MaxClock)
	}
}
