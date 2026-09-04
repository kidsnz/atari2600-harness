package emu

import (
	"fmt"
	"testing"
)

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

	// **A hardware report from 2001, reproduced.** Someone running a game on a PAL woody through a
	// Cuttle Cart: *"It works fine on my PAL woody, although **the red cards are green** (changed RED
	// to `$62` and RED is red for PAL)."* So `$62` should be red under PAL and something else under
	// NTSC. Measured: NTSC `521196` -- purple -- and PAL `7C0A15`, red by any reading.
	//
	// This is the strongest check in the file, and not because it is the largest difference. Every
	// other assertion here compares one emulator against itself under two settings; **this one
	// agrees with a person who owned the console.** known-traps.md records the case where three
	// emulators agreed with each other and were all wrong, so a datum from outside the models is
	// worth more than another from inside them.
	//
	// It does not establish a mapping. Eckhard Stolberg, 1997: "there is not necessarily a
	// corresponding hue for each hue in the other system." One point is one point.
	if n62, ok := sample("NTSC", 0x62); ok && n62 != "521196" {
		t.Errorf("$62 renders %s on NTSC, want 521196 (purple) -- the 2001 report's premise was "+
			"that this value is NOT red on NTSC", n62)
	}
	if p62, ok := sample("PAL", 0x62); ok {
		var r, g, b int
		if _, err := fmt.Sscanf(p62, "%02x%02x%02x", &r, &g, &b); err != nil {
			t.Fatalf("cannot parse %q: %v", p62, err)
		}
		if !(r > 100 && r > 3*g && r > 3*b) {
			t.Errorf("$62 renders %s on PAL; the 2001 hardware report says this is the value that "+
				"made red look red on a PAL console, so it should be dominantly red (R=%d G=%d B=%d)",
				p62, r, g, b)
		}
	}

	// **How many colours each standard actually has**, counted 2026-09-04 by rendering all 128 even
	// values and collecting distinct results:
	//
	//	NTSC  126 of 128
	//	PAL   104
	//	SECAM   8
	//
	// A 1997 post says PAL has *"half the colours"*, hedged with *"I vaguely remember"* — the hedge was
	// right and the claim was not: 104 is 83% of 126, not 50%. **SECAM is the one that deserves the
	// alarm**: eight colours, because it carries luminance only and maps each level to a fixed hue.
	// A picture designed here does not degrade on SECAM, it is replaced.
	count := func(spec string) int {
		e, err := New(spec)
		if err != nil {
			return -1
		}
		if err := e.LoadROM("../../roms/litmus/litmus_swacnt.bin"); err != nil {
			return -1
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for v := 0; v <= 0xFE; v += 2 {
			if err := e.Poke(0x81, uint8(v)); err != nil {
				t.Fatal(err)
			}
			if err := e.RunFrames(2); err != nil {
				t.Fatal(err)
			}
			runs, _, err := e.ReadRow(100)
			if err != nil || len(runs) == 0 {
				continue
			}
			seen[runs[0].Hex] = true
		}
		return len(seen)
	}
	for _, tc := range []struct {
		spec string
		want int
	}{{"NTSC", 126}, {"PAL", 104}, {"SECAM", 8}} {
		if got := count(tc.spec); got != tc.want {
			if got < 0 {
				t.Logf("%s unavailable", tc.spec)
				continue
			}
			t.Errorf("%s renders %d distinct colours from 128 values, want %d — the palette budget "+
				"for that standard has changed and anything designed against the old number needs "+
				"re-checking", tc.spec, got, tc.want)
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
