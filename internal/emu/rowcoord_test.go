package emu

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/annotate"
)

// TestRowCoordinateSystemIsOne pins the promise the annotated screenshot makes:
// the number printed beside a row is the number you pass to read_row and
// decompose_row.
//
// That promise spans three pieces of code that had no test between them —
// annotate.Render draws the label, emu.ReadRow converts it back to an image row,
// emu.DecomposeRow indexes a separate per-colour-clock buffer with it — and it has
// been broken before (v1.4.0: ReadRow took the cropped image row, so every y was off
// by visibleTop). The conversion now exists once, in annotate.GridScanline/GridRow,
// and this test measures that all three agree on real frames.
func TestRowCoordinateSystemIsOne(t *testing.T) {
	roms := []string{
		"../../roms/litmus/litmus_color.bin",
		"../../roms/litmus/litmus_lastline.bin",
		"../../roms/techniques/maze.bin",
		"../../roms/techniques/multicolor48.bin",
	}
	for _, rom := range roms {
		name := filepath.Base(rom)
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		e.EnableElementCapture()
		for i := 0; i < 4; i++ {
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		img, visibleTop := e.cap.snapshot()
		h := img.Bounds().Dy()

		// The label the grid would draw for the first and last image rows.
		firstLabel := annotate.GridScanline(visibleTop, 0)
		lastLabel := annotate.GridScanline(visibleTop, h-1)

		// ReadRow must accept exactly that span and nothing outside it.
		if _, _, err := e.ReadRow(firstLabel); err != nil {
			t.Errorf("%s: ReadRow rejected the grid's first label %d: %v", name, firstLabel, err)
		}
		if _, _, err := e.ReadRow(lastLabel); err != nil {
			t.Errorf("%s: ReadRow rejected the grid's last label %d: %v", name, lastLabel, err)
		}
		if _, _, err := e.ReadRow(firstLabel - 1); err == nil {
			t.Errorf("%s: ReadRow accepted %d, one above the grid's first label", name, firstLabel-1)
		}
		if _, _, err := e.ReadRow(lastLabel + 1); err == nil {
			t.Errorf("%s: ReadRow accepted %d, one below the grid's last label", name, lastLabel+1)
		}

		// DecomposeRow must accept the SAME span. A row you can attribute but cannot
		// read the colours of, or the reverse, means the two tools disagree about
		// which line the user is pointing at.
		rrTop, rrBot, drTop, drBot := -1, -1, -1, -1
		for sl := 0; sl < 300; sl++ {
			if _, _, err := e.ReadRow(sl); err == nil {
				if rrTop < 0 {
					rrTop = sl
				}
				rrBot = sl
			}
			if _, _, err := e.DecomposeRow(sl); err == nil {
				if drTop < 0 {
					drTop = sl
				}
				drBot = sl
			}
		}
		if rrTop != drTop || rrBot != drBot {
			t.Errorf("%s: ReadRow accepts %d..%d but DecomposeRow accepts %d..%d — the two per-row "+
				"observables disagree about which scanlines exist", name, rrTop, rrBot, drTop, drBot)
		}
		if rrTop != firstLabel || rrBot != lastLabel {
			t.Errorf("%s: ReadRow accepts %d..%d, the grid labels %d..%d", name, rrTop, rrBot,
				firstLabel, lastLabel)
		}

		// And the content must come from the row the grid points at, not merely from
		// a row in range: the pixel ReadRow reports for label Y must be the image
		// pixel at GridRow(visibleTop, Y).
		for _, y := range []int{firstLabel, (firstLabel + lastLabel) / 2, lastLabel} {
			runs, _, err := e.ReadRow(y)
			if err != nil || len(runs) == 0 {
				t.Errorf("%s: ReadRow(%d) gave nothing: %v", name, y, err)
				continue
			}
			c := img.RGBAAt(0, annotate.GridRow(visibleTop, y))
			want := hexOf(c.R, c.G, c.B)
			if runs[0].Hex != want {
				t.Errorf("%s: ReadRow(%d) starts with %s but image row %d starts with %s",
					name, y, runs[0].Hex, annotate.GridRow(visibleTop, y), want)
			}
		}
		t.Logf("%-24s grid labels %d..%d (%d rows); ReadRow and DecomposeRow both accept exactly that",
			name, firstLabel, lastLabel, h)
	}

	// GridScanline and GridRow must be inverses, or the single definition is still
	// two definitions.
	for _, vt := range []int{0, 29, 40} {
		for _, r := range []int{0, 1, 100, 213} {
			if got := annotate.GridRow(vt, annotate.GridScanline(vt, r)); got != r {
				t.Errorf("GridRow(GridScanline(%d,%d)) = %d", vt, r, got)
			}
		}
	}
}

func hexOf(r, g, b uint8) string {
	const hexd = "0123456789ABCDEF"
	out := make([]byte, 6)
	for i, v := range []uint8{r, g, b} {
		out[i*2] = hexd[v>>4]
		out[i*2+1] = hexd[v&0x0F]
	}
	return string(out)
}
