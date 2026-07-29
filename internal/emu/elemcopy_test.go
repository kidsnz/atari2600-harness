package emu

import "testing"

// Which COPY of a sprite drew a pixel is not a refinement of which OBJECT drew
// it — without it, three copies of a player and one player stretched to four
// times its width are the same observation, because both are an unbroken stretch
// of the same colour attributed to P0. Every NUSIZ mode therefore looked alike to
// anything reading DecomposeRow, and a generated reproduction of an all-modes ROM
// missed 2666 cells and then blamed sprite multiplexing for it.
//
// The oracle is the hardware's own definition of the eight NUSIZ modes, and
// litmus_nusiz_all runs all eight in 24-line bands with $FF graphics, so each
// band's answer is fixed by the mode and nothing is inferred from a formula:
// the copy count and each copy's width ARE the mode.
func TestDecomposeRowReportsWhichCopyDrewThePixel(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_all.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(20); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()

	// P0 sits well left of P1, so its copies stay on screen in every mode; P1 is
	// placed further right deliberately and its wider modes run off the 160-clock
	// edge, which is why only P0 carries the assertion.
	want := []struct {
		nusiz  int
		copies int
		width  int
		desc   string
	}{
		{0, 1, 8, "one copy"},
		{1, 2, 8, "two copies, close"},
		{2, 2, 8, "two copies, medium"},
		{3, 3, 8, "three copies, close"},
		{4, 2, 8, "two copies, wide"},
		{5, 1, 16, "one copy, double width"},
		{6, 3, 8, "three copies, medium"},
		{7, 1, 32, "one copy, quad width"},
	}
	for _, w := range want {
		y := w.nusiz*24 + 12 // middle of the band, clear of the mode-setting line
		runs, _, err := e.DecomposeRow(top + y)
		if err != nil {
			t.Fatalf("NUSIZ %d: %v", w.nusiz, err)
		}
		byCopy := map[int]int{}
		for _, r := range runs {
			if r.Element == "P0" {
				byCopy[r.Copy] += r.Len
			}
		}
		if len(byCopy) != w.copies {
			t.Errorf("NUSIZ %d (%s): P0 drawn as %d distinct copies, want %d — got %v",
				w.nusiz, w.desc, len(byCopy), w.copies, byCopy)
			continue
		}
		for cpy, n := range byCopy {
			if n != w.width {
				t.Errorf("NUSIZ %d (%s): P0 copy %d is %d pixels wide, want %d — got %v",
					w.nusiz, w.desc, cpy, n, w.width, byCopy)
			}
		}
		// The copies must be numbered from 0 upwards with no gaps, or "how many
		// copies" and "which copy" disagree and only one of them can be right.
		for i := 0; i < w.copies; i++ {
			if _, ok := byCopy[i]; !ok {
				t.Errorf("NUSIZ %d (%s): copy index %d missing from %v", w.nusiz, w.desc, i, byCopy)
			}
		}
	}
}

// The copy index must not invent a distinction where the hardware makes none: a
// mode with one copy has to report exactly one, or every consumer that counts
// copies would see multiplication that never happened.
func TestSingleCopyModesReportExactlyOneCopy(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_all.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(20); err != nil {
		t.Fatal(err)
	}
	_, top := e.Snapshot()
	for _, nusiz := range []int{0, 5, 7} { // one copy; plain, double, quad width
		runs, _, err := e.DecomposeRow(top + nusiz*24 + 12)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[int]bool{}
		for _, r := range runs {
			if r.Element == "P0" {
				seen[r.Copy] = true
			}
		}
		if len(seen) != 1 || !seen[0] {
			t.Errorf("NUSIZ %d draws one copy, but the attribution reports copies %v", nusiz, seen)
		}
	}
}
