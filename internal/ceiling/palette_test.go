package ceiling

import "testing"

// The palette must come from the renderer that produced the frames. PaletteFor
// asks Gopher2600's colour generator; HarvestPalette MEASURES what the renderer
// actually painted, by running roms/litmus/litmus_palette.bin — 4 white marker
// scanlines then 128 lines of COLUBK = $00,$02,..,$FE — and reading one colour
// off each sweep line of the rendered frame.
//
// The two must agree on all 128 entries EXACTLY. If they ever diverge, the
// derived table has stopped describing the pixels (a colour-generation or
// gamma step applied on the way to the framebuffer but not in Spec.GetColor,
// say), and every ceiling number computed from it is quantised against a
// palette the frames are not drawn in — which is the exact defect that made the
// prototype's self-test read 9.95 instead of 0.
func TestHarvestedPaletteEqualsDerivedPalette(t *testing.T) {
	derived, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatalf("PaletteFor: %v", err)
	}
	harvested, err := HarvestPalette("../../"+PaletteROM, "NTSC")
	if err != nil {
		t.Fatalf("HarvestPalette: %v", err)
	}
	worst, worstAt := 0, -1
	mismatch := 0
	for i := 0; i < PaletteSize; i++ {
		d := int(dist2(derived.Colors[i], harvested.Colors[i]))
		if d != 0 {
			mismatch++
			if d > worst {
				worst, worstAt = d, i
			}
		}
	}
	if mismatch != 0 {
		t.Errorf("derived and measured palettes disagree on %d of %d entries; worst is code $%02X "+
			"(derived %v, measured %v, squared RGB distance %d). The ceiling metric quantises against the "+
			"DERIVED table, so it is no longer describing the pixels the renderer paints.",
			mismatch, PaletteSize, worstAt*2, derived.Colors[worstAt], harvested.Colors[worstAt], worst)
	}
	t.Logf("all %d TIA colours agree exactly between Spec.GetColor and the rendered %s sweep", PaletteSize, PaletteROM)
}

// A wrong palette must be visible as a wrong palette, not merely as a wrong
// score. Shifted is the defect injector the self-test is checked with, so its
// own behaviour is pinned: every entry moves by delta, clamped.
func TestShiftedPaletteMovesEveryEntryAndClamps(t *testing.T) {
	p, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatalf("PaletteFor: %v", err)
	}
	s := p.Shifted(40)
	moved := 0
	for i := 0; i < PaletteSize; i++ {
		for c := 0; c < 3; c++ {
			want := p.Colors[i][c] + 40
			if want > 255 {
				want = 255
			}
			if s.Colors[i][c] != want {
				t.Fatalf("code $%02X channel %d: got %d want %d", i*2, c, s.Colors[i][c], want)
			}
			if s.Colors[i][c] != p.Colors[i][c] {
				moved++
			}
		}
	}
	if moved == 0 {
		t.Error("Shifted(40) moved nothing — the planted defect would be a no-op")
	}
	t.Logf("Shifted(40) moved %d of %d channels (the rest were already at the 255 clamp)", moved, PaletteSize*3)
}
