package emu

import "testing"

// TestHmoveMidStrobesAllFireButOnlyOneShifts states what litmus_hmove_mid actually
// demonstrates, which is not what its scenario appears to say.
//
// The ROM repositions P0 to X=60 every frame the ordinary way (coarse + fine, HMOVE
// in HBLANK, HMCLR) and then, on three frames out of every four, strobes a SECOND
// HMOVE part-way down visible scanline 136 with all motion registers cleared. The
// fourth frame is the control: no mid-line strobe.
//
// scenarios/hmove_mid.json pins the resulting positions as 55, 60, 60, 60. Read
// alone that looks like a four-entry table of behaviours; measured, three of the
// four values coincide FOR TWO DIFFERENT REASONS. One 60 is the control, which
// never strobed. The other two are frames that DID strobe and moved nothing:
//
//	control      1 HMOVE latch  (scanline 1)          -> P0 = 60
//	delay A      2 HMOVE latches (scanline 1 and 136) -> P0 = 55
//	delay B, C   2 HMOVE latches (scanline 1 and 136) -> P0 = 60
//
// So a mid-line HMOVE with HM=0 shifts the object at ONE of the three strobe
// positions this ROM tries, and leaves it alone at the other two. The position
// assertions cannot tell "the strobe fired and did nothing" from "the strobe never
// fired", and the difference is the whole subject of the fixture. This test counts
// the strobes, so a regression that stops emitting them fails here even though every
// scenario assert would still pass.
func TestHmoveMidStrobesAllFireButOnlyOneShifts(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove_mid.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // the scenario's warmup
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
	}

	type obs struct {
		x       int
		latches int
		midLine int // the scanline of the second latch, -1 if there is none
	}
	var got []obs
	for f := 0; f < 8; f++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
		o := obs{midLine: -1}
		for sl := 0; sl < 262; sl++ {
			span, err := e.HmoveOnScanline(sl)
			if err != nil || !span.Recorded || !span.Latched {
				continue
			}
			o.latches++
			if o.latches == 2 {
				o.midLine = sl
			}
		}
		o.x, _ = e.ObjectX("P0")
		got = append(got, o)
	}

	// The cycle must be four frames long and repeat.
	for i := 4; i < len(got); i++ {
		if got[i] != got[i-4] {
			t.Errorf("frame %d %+v does not repeat frame %d %+v — the parity cycle is not 4 frames",
				i, got[i], i-4, got[i-4])
		}
	}

	controls, strobed, shifted := 0, 0, 0
	for i, o := range got[:4] {
		switch o.latches {
		case 1:
			controls++
			if o.x != 60 {
				t.Errorf("frame %d is the control (no mid-line strobe) but P0 is at %d, not the "+
					"60 it was placed at", i, o.x)
			}
		case 2:
			strobed++
			if o.midLine != 136 {
				t.Errorf("frame %d: the second HMOVE latched on scanline %d, expected 136 — the strobe "+
					"has moved out of the visible region and is no longer a MID-LINE HMOVE",
					i, o.midLine)
			}
			if o.x != 60 {
				shifted++
			}
		default:
			t.Errorf("frame %d latched HMOVE on %d scanlines, expected 1 (control) or 2 (strobed)",
				i, o.latches)
		}
	}

	if controls != 1 {
		t.Errorf("%d control frames in the 4-frame cycle, expected exactly 1", controls)
	}
	if strobed != 3 {
		t.Errorf("%d frames carried a mid-line strobe, expected 3 — a scenario that only reads "+
			"positions would still pass with the strobes gone", strobed)
	}
	if shifted != 1 {
		t.Errorf("%d of the 3 mid-line strobes moved the object, measured 1. If this is now 0 the "+
			"fixture demonstrates nothing; if it is 3 the engine's mid-line HMOVE behaviour has "+
			"changed", shifted)
	}
	t.Logf("4-frame cycle: %d control (1 latch), %d strobed (2 latches, scanline 136), of which %d shifted "+
		"the object; positions %d %d %d %d",
		controls, strobed, shifted, got[0].x, got[1].x, got[2].x, got[3].x)
}
