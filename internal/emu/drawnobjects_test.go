package emu

// A marker is a claim that an object is on screen, so it has to be measured.
//
// `Markers` read the TIA's position registers and returned all five objects
// unconditionally. Every TIA object always HAS a position — it is a counter, it does
// not stop existing when nothing is drawn — so on a playfield-only kernel the
// annotated screenshot showed five labelled vertical lines for five objects that
// painted nothing. That image is the primary channel the user reads a picture
// through, which makes a phantom object there a false statement about the ROM rather
// than a cosmetic blemish.
//
// `DrawnObjects` answers from the per-pixel attribution buffer instead: it looks for
// the object IN THE FRAME. Both directions are asserted here, because a function that
// answered "false" for everything would satisfy the playfield case on its own and
// silently erase every real sprite.

import (
	"testing"
)

func TestDrawnObjectsSeparatesPaintedFromMerelyPositioned(t *testing.T) {
	for _, c := range []struct {
		name string
		rom  string
		// want is P0, M0, P1, M1, BL — Markers' order.
		want    [5]bool
		because string
	}{
		{
			name: "playfield-only kernel draws no movable object",
			rom:  "../../roms/litmus/litmus_pf_allcols.bin",
			want: [5]bool{},
			because: "this ROM lights one playfield column per band and never writes GRP/ENAM/ENABL, " +
				"so every marker on its annotated screenshot was a phantom",
		},
		{
			name:    "a player kernel draws P0",
			rom:     "../../roms/litmus/litmus_pos.bin",
			want:    [5]bool{true, false, false, false, false},
			because: "litmus_pos positions and draws P0 only",
		},
		{
			// The expected values here were WRONG when first written, guessed from
			// the ROM's name: "objsizes" was read as covering players too. It does
			// not — its header says so, and the measurement said so first. Kept as
			// a case precisely because the two readings of the buffer agreed with
			// each other and disagreed with the author, which is the arrangement
			// that makes a wrong expectation visible instead of a wrong tool.
			name: "the missile/ball width litmus draws the missiles and the ball, and no player",
			rom:  "../../roms/litmus/litmus_objsizes.bin",
			want: [5]bool{false, true, false, true, true},
			because: "it sweeps every missile and ball WIDTH plus the ball's vertical delay; it never " +
				"writes GRP0 or GRP1, so neither player appears",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := New("NTSC")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.LoadROM(c.rom); err != nil {
				t.Fatalf("%s: %v", c.rom, err)
			}
			if err := e.RunFrames(4); err != nil {
				t.Fatal(err)
			}

			got := e.DrawnObjects()
			// DERIVE the labels from Markers() rather than restating them. DrawnObjects'
			// idxOf exists for exactly one reason — to line up with the Markers() literal
			// beside it — so a test that hard-codes the same order agrees with idxOf even
			// when idxOf has stopped agreeing with Markers. Reading them back makes the
			// two impossible to reorder independently.
			var labels [5]string
			ms := e.Markers()
			if len(ms) != len(labels) {
				t.Fatalf("Markers() returned %d markers, want %d — DrawnObjects indexes "+
					"this slice and the two have gone out of step", len(ms), len(labels))
			}
			for i, m := range ms {
				labels[i] = m.Label
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("%s: drawn=%v, want %v — %s", labels[i], got[i], c.want[i], c.because)
				}
			}

			// Cross-check against DecomposeRow, which is what a caller would use to
			// verify by hand. Both read the same buffer, so agreement is not
			// independent evidence — but a disagreement would mean one of them
			// indexes it wrongly, and that is worth catching.
			seen := map[string]bool{}
			for y := e.cap.frameInfo.VisibleTop; y <= e.cap.frameInfo.VisibleBottom; y++ {
				runs, _, err := e.DecomposeRow(y)
				if err != nil {
					continue
				}
				for _, r := range runs {
					seen[r.Element] = true
				}
			}
			for i, label := range labels {
				if seen[label] != got[i] {
					t.Errorf("%s: DrawnObjects says %v but DecomposeRow %s find it — the two readings "+
						"of the attribution buffer disagree", label, got[i],
						map[bool]string{true: "does", false: "does not"}[seen[label]])
				}
			}
		})
	}
}

// TestMarkersCarryTheMeasurement wires the two together: the annotated image drops a
// marker when Drawn is false, so if Markers stopped propagating it every phantom
// would come back while DrawnObjects still tested green on its own.
func TestMarkersCarryTheMeasurement(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pf_allcols.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}
	ms := e.Markers()
	if len(ms) != 5 {
		t.Fatalf("expected 5 markers, got %d — the JSON still lists every object; only the IMAGE "+
			"drops the ones that drew nothing", len(ms))
	}
	for _, m := range ms {
		if m.Drawn {
			t.Errorf("%s is marked as drawn on a playfield-only ROM", m.Label)
		}
	}

	// And the other way, so this cannot pass by returning false always.
	e2, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e2.LoadROM("../../roms/litmus/litmus_pos.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e2.RunFrames(4); err != nil {
		t.Fatal(err)
	}
	var any bool
	for _, m := range e2.Markers() {
		if m.Drawn {
			any = true
		}
	}
	if !any {
		t.Fatal("no marker is drawn on a ROM that draws a player; the measurement would erase every " +
			"real sprite from the annotated screenshot")
	}
}
