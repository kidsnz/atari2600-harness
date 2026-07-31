package emu

import "testing"

// TestEveryPlayfieldColumnLandsWhereTheTableSays machine-locks the column→register→bit
// map that CLAUDE.md lists under "constants you must never get wrong": PF0 upper nibble
// only (col0→D4 … col3→D7), PF1 MSB first (col4→D7 … col11→D0), PF2 LSB first
// (col12→D0 … col19→D7), four colour clocks per column, left half 0–79 and — with
// CTRLPF D0 = 0 — repeated in 80–159.
//
// litmus_pf checks the leftmost bit of each register: columns 0, 4 and 12. That is
// three of twenty positions, and the three easiest ones. The other seventeen bits of
// the table had nothing behind them.
//
// litmus_pf_allcols draws the whole map in one frame — 20 bands of 9 scanlines, band k
// lighting only column k — so one pass over the frame checks every entry, including
// that nothing ELSE lights up, which is what catches a bit landing in the wrong column
// rather than in no column.
func TestEveryPlayfieldColumnLandsWhereTheTableSays(t *testing.T) {
	const (
		visibleTop = 40 // first drawn scanline of this kernel
		bandHeight = 9
		bands      = 20
	)
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pf_allcols.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	checked := 0
	for k := 0; k < bands; k++ {
		line := visibleTop + k*bandHeight + bandHeight/2
		runs, _, err := e.DecomposeRow(line)
		if err != nil {
			t.Fatalf("column %d: scanline %d: %v", k, line, err)
		}
		var lit [][2]int
		for _, r := range runs {
			if r.Element == "PF" {
				lit = append(lit, [2]int{r.Clock, r.Len})
			}
		}
		want := [][2]int{{4 * k, 4}, {80 + 4*k, 4}}
		if len(lit) != 2 || lit[0] != want[0] || lit[1] != want[1] {
			t.Errorf("column %d (scanline %d): playfield lit at %v, want %v — the byte for this "+
				"column comes straight from the documented table, so a mismatch means the table is "+
				"wrong about which bit paints where", k, line, lit, want)
			continue
		}
		checked++
	}
	if checked != bands {
		t.Errorf("only %d of %d columns verified", checked, bands)
	}
	t.Logf("all %d columns land on their documented 4-clock span, and repeat at +80", checked)
}
