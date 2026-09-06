package ceiling

import "testing"

// ntscGolden is the NTSC palette this repository actually paints with: 128 codes (the even ones —
// D0 is unused by the TIA) and the RGB triple the renderer produces for each.
//
// ★Why a golden and not a comparison. `TestHarvestedPaletteEqualsDerivedPalette` checks
// `PaletteFor` against `HarvestPalette` — one derived from the spec, one measured by rendering
// `litmus_palette.bin`. That is a good test of the harvest path and it CANNOT see a change to the
// colour generator, because both sides go through `Spec.GetColor` and a generator change moves them
// together. Measured 2026-09-05 by probing the vendored engine: setting
// `colourgen.SetDefaults`'s `LegacyAdjust.Saturation` from **0.963 to 0.500** moves code `$46` from
// `[236 51 51]` to `[157 69 69]` — a red turning to brick, plainly visible — and the twin test still
// reported `ok`. The five playfield-only ROMs whose C1==0 is asserted elsewhere are blind to it for
// the same reason. Found by the mailing-list distillation (helper-2), who closed the population of
// engine defaults and named `colourgen` as the one with no guard; the probe was run and reverted
// here.
//
// ★★What the numbers depend on, which is the point of pinning them. `colourgen/legacy.go` starts
// from 128 RGB literals *"taken from Stella 7.0 file common/PaletteHandler.cxx"*, converts them to
// YIQ, applies **LegacyEnabled true, Brightness 1.196, Contrast 1.000, Saturation 0.963, Hue 0.0,
// NTSCPhase 0.0** and a gamma, and converts back. So this table is not "the Stella palette" — it is
// the Stella palette put through that round trip with those six numbers. Upstream knows the source
// table moves: `colourgen/legacy_test.go`'s `TestStellaComparisons` holds **five** Stella palettes
// and asserts where one of them changed. Upstream guards the input; this guards the output.
//
// ★★★The aliasing is visible in the table and is not a defect: `$0C` and `$0E` are both
// `[255 255 254]`. `internal/ingest/aliases_test.go` measures how much of the palette is
// distinguishable and why — 91 distinct colours out of 128 against the Stella table, 126 against
// this generator — which is the number an artist needs before picking swatches.
//
// ★★★★If this test fails, the renderer's colours have moved. Do not update the table to make it
// pass without saying what moved them and why: every ceiling measurement, every pixel comparison
// against the Stella oracle, and every piece of artwork ingested against these codes is quantised
// against exactly these values. The precedent for pinning a measured table rather than a derived
// one is `pkg/audio.MeasuredSpectra`.
var ntscGolden = [PaletteSize]struct {
	Code    uint8
	R, G, B int
}{
	{0x00, 6, 6, 6}, {0x02, 52, 52, 52}, {0x04, 92, 92, 92}, {0x06, 136, 136, 136},
	{0x08, 184, 184, 184}, {0x0A, 227, 227, 227}, {0x0C, 255, 255, 254}, {0x0E, 255, 255, 254},
	{0x10, 50, 50, 7}, {0x12, 84, 84, 13}, {0x14, 123, 123, 21}, {0x16, 169, 168, 29},
	{0x18, 216, 215, 38}, {0x1A, 255, 255, 47}, {0x1C, 255, 255, 48}, {0x1E, 255, 255, 41},
	{0x20, 106, 28, 7}, {0x22, 136, 50, 14}, {0x24, 166, 77, 23}, {0x26, 199, 107, 33},
	{0x28, 230, 140, 43}, {0x2A, 255, 173, 55}, {0x2C, 255, 209, 66}, {0x2E, 255, 243, 79},
	{0x30, 134, 19, 7}, {0x32, 166, 38, 16}, {0x34, 199, 61, 27}, {0x36, 234, 89, 40},
	{0x38, 255, 119, 54}, {0x3A, 255, 152, 70}, {0x3C, 255, 185, 86}, {0x3E, 255, 219, 103},
	{0x40, 139, 7, 7}, {0x42, 172, 18, 18}, {0x44, 204, 33, 33}, {0x46, 236, 51, 51},
	{0x48, 255, 71, 71}, {0x4A, 255, 94, 94}, {0x4C, 255, 117, 117}, {0x4E, 255, 141, 141},
	{0x50, 116, 7, 77}, {0x52, 146, 18, 104}, {0x54, 175, 32, 135}, {0x56, 205, 50, 166},
	{0x58, 234, 68, 197}, {0x5A, 255, 90, 229}, {0x5C, 255, 112, 255}, {0x5E, 255, 135, 255},
	{0x60, 56, 7, 116}, {0x62, 82, 17, 150}, {0x64, 109, 32, 183}, {0x66, 140, 49, 220},
	{0x68, 170, 68, 254}, {0x6A, 202, 90, 255}, {0x6C, 234, 111, 255}, {0x6E, 255, 135, 255},
	{0x70, 14, 7, 132}, {0x72, 33, 18, 164}, {0x74, 55, 33, 197}, {0x76, 83, 51, 231},
	{0x78, 113, 71, 255}, {0x7A, 146, 93, 255}, {0x7C, 181, 116, 255}, {0x7E, 217, 141, 255},
	{0x80, 7, 7, 138}, {0x82, 17, 18, 170}, {0x84, 29, 32, 202}, {0x86, 45, 50, 234},
	{0x88, 62, 70, 255}, {0x8A, 81, 93, 255}, {0x8C, 101, 116, 255}, {0x8E, 122, 140, 255},
	{0x90, 7, 18, 121}, {0x92, 17, 39, 154}, {0x94, 30, 64, 188}, {0x96, 46, 96, 224},
	{0x98, 63, 129, 255}, {0x9A, 82, 165, 255}, {0x9C, 102, 204, 255}, {0x9E, 124, 242, 255},
	{0xA0, 7, 30, 76}, {0xA2, 17, 57, 112}, {0xA4, 30, 89, 149}, {0xA6, 46, 126, 191},
	{0xA8, 64, 165, 233}, {0xAA, 83, 207, 255}, {0xAC, 103, 250, 255}, {0xAE, 125, 255, 255},
	{0xB0, 7, 42, 30}, {0xB2, 17, 75, 55}, {0xB4, 30, 115, 85}, {0xB6, 47, 160, 118},
	{0xB8, 64, 208, 154}, {0xBA, 83, 255, 191}, {0xBC, 104, 255, 231}, {0xBE, 116, 255, 255},
	{0xC0, 7, 45, 7}, {0xC2, 18, 80, 18}, {0xC4, 33, 119, 33}, {0xC6, 52, 163, 52},
	{0xC8, 72, 212, 72}, {0xCA, 95, 255, 95}, {0xCC, 118, 255, 118}, {0xCE, 139, 255, 139},
	{0xD0, 15, 39, 7}, {0xD2, 35, 72, 17}, {0xD4, 60, 111, 30}, {0xD6, 92, 157, 47},
	{0xD8, 127, 207, 64}, {0xDA, 164, 255, 84}, {0xDC, 204, 255, 104}, {0xDE, 209, 255, 101},
	{0xE0, 31, 36, 7}, {0xE2, 57, 66, 16}, {0xE4, 90, 101, 29}, {0xE6, 127, 142, 43},
	{0xE8, 166, 185, 59}, {0xEA, 208, 230, 77}, {0xEC, 252, 255, 95}, {0xEE, 255, 255, 95},
	{0xF0, 49, 28, 7}, {0xF2, 84, 55, 15}, {0xF4, 122, 86, 26}, {0xF6, 168, 124, 39},
	{0xF8, 214, 164, 52}, {0xFA, 255, 206, 67}, {0xFC, 255, 252, 82}, {0xFE, 255, 255, 81},
}

func TestNTSCPaletteMatchesTheMeasuredGolden(t *testing.T) {
	derived, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatalf("PaletteFor: %v", err)
	}
	harvested, err := HarvestPalette("../../"+PaletteROM, "NTSC")
	if err != nil {
		t.Fatalf("HarvestPalette: %v", err)
	}

	// Both paths are checked against the golden, not against each other — that is the whole point.
	for _, side := range []struct {
		name string
		p    Palette
	}{{"derived (PaletteFor)", derived}, {"measured (HarvestPalette)", harvested}} {
		bad, worst, worstAt := 0, 0, -1
		for i := 0; i < PaletteSize; i++ {
			g := ntscGolden[i]
			if uint8(i*2) != g.Code {
				t.Fatalf("golden row %d carries code $%02X, expected $%02X — the table is misaligned "+
					"and every comparison below would be against the wrong colour", i, g.Code, i*2)
			}
			got := side.p.Colors[i]
			d := (got[0]-g.R)*(got[0]-g.R) + (got[1]-g.G)*(got[1]-g.G) + (got[2]-g.B)*(got[2]-g.B)
			if d != 0 {
				bad++
				if d > worst {
					worst, worstAt = d, i
				}
			}
		}
		if bad != 0 {
			g := ntscGolden[worstAt]
			t.Errorf("%s disagrees with the measured golden on %d of %d codes; worst is $%02X "+
				"(golden [%d %d %d], now %v, squared RGB distance %d). The renderer's colours have "+
				"moved — most likely one of colourgen's six adjustment values, which nothing else "+
				"here guards. Say what moved them and why before updating this table: the ceiling "+
				"metric, the Stella pixel oracle and every ingested artwork are quantised against "+
				"exactly these numbers",
				side.name, bad, PaletteSize, g.Code, g.R, g.G, g.B, side.p.Colors[worstAt], worst)
		}
	}

	// A witness against the table being trivially satisfiable: if the golden were all zeros, or the
	// palette all one colour, the loops above would still run. Require the table to be a real
	// palette — more than half its entries distinct.
	seen := map[[3]int]bool{}
	for _, g := range ntscGolden {
		seen[[3]int{g.R, g.G, g.B}] = true
	}
	if len(seen) < PaletteSize/2 {
		t.Fatalf("the golden holds only %d distinct colours out of %d rows — that is not a palette, "+
			"and a test comparing against it would pass for the wrong reason", len(seen), PaletteSize)
	}
	t.Logf("128 codes pinned, %d distinct colours", len(seen))
}
