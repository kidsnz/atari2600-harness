package emu

import "testing"

// TestPlacementIsTheSameOnPAL measures whether `sprite-placement.md`'s rules — measured entirely on
// NTSC — hold under PAL. They do, exactly, and that matters more than it sounds: this repository's
// placement machinery (`plan_sprite_placement`, `cmd/place`, `internal/place`, the whole rule table)
// carried no statement about which TV standard it was true of.
//
// The question came from a 2004 report on stella-list (`200407/msg00015`, Kroko), on a PAL console:
// *"the game is cool when I play it with the emulator. For some reason, there are problems when I
// play it on the 2600 … when I shoot, then the shots **always start at the same position (a little
// right of the middle of the screen)** and not where the spaceship is. That is not so on the
// emulator, but it runs in NTSC mode, i guess."* Horizontal placement going wrong under PAL is one
// explanation, and this test removes it.
//
// Running `litmus_resp_pair` unchanged under both specs:
//
//	band | NTSC        | PAL         |
//	A    | 69 / 78     | 69 / 78     | one background pixel between the players
//	B    | 62 / 71     | 62 / 71     | both pulled left 7
//	C    | 69 / 77     | 69 / 77     | joined, 16 px continuous
//	D    | 69 / 79     | 69 / 79     | two background pixels
//
// Identical to the clock. **Two things do differ and neither touches the horizontal:** the colours
// (`FFFFFE`→`FFFEFF`, `CC2121`→`A44108` — see `palspec_test.go`), and the line numbers, which shift
// by +19 because PAL's vertical blanking is longer. So a picture designed here keeps its geometry on
// a PAL console and loses its palette.
//
// What this does NOT settle: the 2004 symptom itself. Two variables moved at once in that report —
// PAL *and* real hardware versus NTSC *and* an emulator — and helper-3, who found it, said so rather
// than picking one. This closes the horizontal-placement branch of the explanation and leaves the
// rest open.
func TestPlacementIsTheSameOnPAL(t *testing.T) {
	// (NTSC line, PAL line) for the four bands; PAL's picture starts 19 lines later.
	bands := []struct {
		name              string
		ntscLine, palLine int
		wantP0, wantP1    int
	}{
		{"A", 46, 65, 69, 78},
		{"B", 60, 79, 62, 71},
		{"C", 74, 93, 69, 77},
		{"D", 88, 107, 69, 79},
	}
	// pair returns the start clocks of the two non-background runs on a line.
	pair := func(t *testing.T, spec string, line int) (int, int, int) {
		e, err := New(spec)
		if err != nil {
			t.Skipf("%s unavailable: %v", spec, err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_resp_pair.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		runs, _, err := e.ReadRow(line)
		if err != nil {
			t.Fatalf("%s row %d: %v", spec, line, err)
		}
		if len(runs) == 0 {
			t.Fatalf("%s row %d: no runs", spec, line)
		}
		bg := runs[0].Hex // the leftmost run is always background in this fixture
		var obj []int
		for _, r := range runs {
			if r.Hex != bg && r.Len == 8 {
				obj = append(obj, r.Clock)
			}
		}
		if len(obj) != 2 {
			t.Fatalf("%s row %d: found %d eight-wide objects, want 2 (%v)", spec, line, len(obj), runs)
		}
		return obj[0], obj[1], len(runs)
	}

	for _, b := range bands {
		n0, n1, _ := pair(t, "NTSC", b.ntscLine)
		p0, p1, _ := pair(t, "PAL", b.palLine)
		if n0 != b.wantP0 || n1 != b.wantP1 {
			t.Errorf("band %s on NTSC: %d/%d, want %d/%d — the fixture itself moved and the "+
				"comparison below means nothing", b.name, n0, n1, b.wantP0, b.wantP1)
		}
		if p0 != n0 || p1 != n1 {
			t.Errorf("band %s: NTSC puts the pair at %d/%d and PAL at %d/%d. **sprite-placement.md's "+
				"rules were all measured on NTSC**; if the specs disagree, every placement number in "+
				"this repository needs a standard attached to it", b.name, n0, n1, p0, p1)
		}
	}

	// Control: something MUST differ between the specs, or the two runs above are not actually
	// different machines and the agreement is vacuous. The colour is the thing that differs.
	e1, err1 := New("NTSC")
	e2, err2 := New("PAL")
	if err1 != nil || err2 != nil {
		t.Skip("both specs required")
	}
	for _, e := range []*Emu{e1, e2} {
		if err := e.LoadROM("../../roms/litmus/litmus_resp_pair.bin"); err != nil {
			t.Skip("litmus unavailable")
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
	}
	rn, _, _ := e1.ReadRow(46)
	rp, _, _ := e2.ReadRow(65)
	if len(rn) > 1 && len(rp) > 1 && rn[1].Hex == rp[1].Hex {
		t.Errorf("the players render as %s under both specs — the two emulators are not behaving "+
			"differently at all, so 'placement agrees' is not evidence of anything", rn[1].Hex)
	}
}
