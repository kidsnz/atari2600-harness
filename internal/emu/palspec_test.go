package emu

import "testing"

// TestSameColourByteIsADifferentColourOnPAL measures a fact this project's own tooling does not
// carry: **the same COLUxx value is a different hue on a PAL console**, often unrecognisably so.
//
// Eckhard Stolberg, stella-list `the-demo-image-series-7` (2003-02), looking at someone's demo on
// his own television:
//
//	"I tried this demo out on my TV … But what is funny is that the NTSC RED you chose comes out as
//	 GREEN on a PAL VCS."
//
// Reproduced here, and it is not a shift, it is a different colour:
//
//	COLU   NTSC        PAL
//	$36    EA5928 orange   ->  51C00C green      <- Stolberg's sentence, in our engine
//	$C6    34A334 green    ->  904AFF violet
//	$86    2D32EA blue     ->  E044B5 pink
//	$1A    FFFF2F yellow   ->  CFCFCF grey
//	$D4    3C6F1E dark green -> 3A34FC blue
//
// Ten values sampled, ten differ. Only the luminance-only column ($0E white) stays close.
//
// **Why this is a test and not a note.** `internal/ingest` holds exactly one palette —
// `palette_stella.go`, `stellaNTSC` — and `NewStellaNTSCQuantizer` is the only quantiser there is.
// Artwork ingested through it is mapped to NTSC colour bytes with no record that the choice is
// spec-specific. Nothing in `internal/` or `pkg/` mentions a PAL palette at all, though the engine
// ships five specs (`SpecList`: NTSC, PAL, PAL60, PAL-M, SECAM). So a picture designed here is a
// picture designed *for NTSC*, and this test is what makes that statement falsifiable: if some
// future change makes the two specs agree, the premise behind "choose colours for one spec" has
// changed and someone should be told.
func TestSameColourByteIsADifferentColourOnPAL(t *testing.T) {
	sample := func(spec string, colu uint8) (string, bool) {
		e, err := New(spec)
		if err != nil {
			return "", false
		}
		if err := e.LoadROM("../../roms/litmus/litmus_swacnt.bin"); err != nil {
			return "", false
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		// The ROM copies RAM $81 to COLUBK every frame, so poking RAM chooses the background.
		if err := e.Poke(0x81, colu); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		runs, _, err := e.ReadRow(100)
		if err != nil || len(runs) == 0 {
			return "", false
		}
		return runs[0].Hex, true
	}

	// Hue-carrying values: every one of these must differ between the specs.
	for _, c := range []uint8{0x36, 0xC6, 0x86, 0x1A, 0xD4, 0x42, 0x44, 0x46, 0x9A} {
		n, ok1 := sample("NTSC", c)
		p, ok2 := sample("PAL", c)
		if !ok1 || !ok2 {
			t.Skip("both specs required")
		}
		if n == p {
			t.Errorf("COLU $%02X renders as %s on both NTSC and PAL. If the specs now agree, then "+
				"colour choices are no longer spec-specific and `internal/ingest`'s single NTSC "+
				"palette has stopped being a limitation worth naming", c, n)
		}
	}

	// The control: a luminance-only value has no hue to get wrong, so it must NOT differ much.
	// Without this the test would pass on a build where PAL simply returned garbage.
	n, ok1 := sample("NTSC", 0x0E)
	p, ok2 := sample("PAL", 0x0E)
	if !ok1 || !ok2 {
		t.Skip("both specs required")
	}
	if n[:2] != p[:2] {
		t.Errorf("white ($0E) reads %s on NTSC and %s on PAL — a luminance-only value carries no "+
			"hue, so a large difference here means PAL is not rendering, not that the palettes "+
			"differ, and the assertions above prove nothing", n, p)
	}
}
