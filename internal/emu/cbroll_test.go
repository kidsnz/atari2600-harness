package emu

import (
	"fmt"
	"strings"
	"testing"
)

// visibleRowSignatures renders `frames` frames and returns the length of the frame
// that follows them plus one signature string per visible scanline.
func visibleRowSignatures(t *testing.T, rom string, frames int) (lines int, rows []string) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < frames; i++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}
	start := e.Coords().Frame
	maxLine := 0
	for e.Coords().Frame == start {
		if _, err := e.stepInstr(); err != nil {
			t.Fatal(err)
		}
		if l := e.Coords().Scanline; l > maxLine {
			maxLine = l
		}
	}
	for sl := 0; sl < 192; sl++ {
		runs, _, err := e.ReadRow(sl)
		if err != nil {
			rows = append(rows, fmt.Sprintf("ERR:%v", err))
			continue
		}
		var b strings.Builder
		for _, r := range runs {
			fmt.Fprintf(&b, "%d+%d=%s,", r.Clock, r.Len, r.Hex)
		}
		rows = append(rows, b.String())
	}
	return maxLine + 1, rows
}

// TestCbRollIsOneRowFromCbClean puts a machine behind the claim that justifies the
// static line-budget prover.
//
// cb_roll's header and docs/capability-gap-audit.md both said cb_roll and cb_clean
// render PIXEL-IDENTICALLY, "verified 2026-06-17" — by hand, with nothing checking
// it since. Re-measured 2026-07-30 it is not true, and the real number is the
// better argument: of the 192 visible scanlines exactly ONE differs. The stolen
// scanline duplicates the stripe above it at scanline 133 ($060606 where cb_clean
// has $380774); every other row matches.
//
// One row in 192 is not something anyone catches by eye. 263 scanlines against 262
// is unambiguous. That gap between what the eye can see and what the numbers say is
// the entire reason a static prover exists, so it is worth a test rather than a
// remembered observation. cb_roll was also one of only two litmus ROMs referenced by
// nothing at all — no scenario, no test — which is how the claim went stale.
func TestCbRollIsOneRowFromCbClean(t *testing.T) {
	rollLines, rollRows := visibleRowSignatures(t, "../../roms/litmus/cb_roll.bin", 8)
	cleanLines, cleanRows := visibleRowSignatures(t, "../../roms/litmus/cb_clean.bin", 8)

	if cleanLines != 262 {
		t.Errorf("cb_clean is %d scanlines, want 262 — the clean counterpart is not clean", cleanLines)
	}
	if rollLines != 263 {
		t.Errorf("cb_roll is %d scanlines, want 263 — the heavy line is no longer stealing one", rollLines)
	}
	if rollLines == cleanLines {
		t.Fatal("both ROMs have the same frame length; the pair no longer demonstrates anything")
	}

	if len(rollRows) != len(cleanRows) || len(rollRows) != 192 {
		t.Fatalf("compared %d and %d rows, want 192 each", len(rollRows), len(cleanRows))
	}
	var differing []int
	for i := range rollRows {
		if rollRows[i] != cleanRows[i] {
			differing = append(differing, i)
		}
	}
	if len(differing) != 1 {
		t.Errorf("%d visible scanlines differ, want exactly 1 — the pair is documented as differing by a "+
			"single duplicated stripe, and that number is what makes 'you cannot see it' true. differing: %v",
			len(differing), differing)
	}
	if len(differing) == 1 && differing[0] != 133 {
		t.Errorf("the differing scanline is %d, want 133", differing[0])
	}
	// The difference must be a DUPLICATE of the row above, not arbitrary noise: that
	// is what "the heavy line ate a scanline" means.
	if len(differing) == 1 {
		d := differing[0]
		if rollRows[d] != rollRows[d-1] {
			t.Errorf("scanline %d does not repeat the row above it in cb_roll; the difference is not a "+
				"stolen line.\n above: %s\n   at : %s", d, rollRows[d-1], rollRows[d])
		}
	}
	t.Logf("cb_roll %d lines / cb_clean %d lines; %d of 192 visible rows differ",
		rollLines, cleanLines, len(differing))
}
