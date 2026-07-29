package vismatch

import "testing"

// litmus_title_then_play draws one playfield for frames 0..29 and a different
// one from frame 30 onward. The switch frame is 30 by construction, so the
// detector is graded against a number it cannot have influenced — unlike one
// inferred from its own output, which would agree with whatever it reported.
const titleROM = "../../roms/litmus/litmus_title_then_play.bin"

func TestFindGameplayStartRecoversAPlantedTitleLength(t *testing.T) {
	gs, err := FindGameplayStart(titleROM, "NTSC", 90, 8)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	if !gs.Found {
		t.Fatalf("no transition found on a ROM that changes its playfield at frame 30: %+v", gs)
	}
	// RunFrames(1) lands on frame 1's picture at index 0, so the reported index is
	// one behind the ROM's own counter; allow that single offset and no more.
	if gs.Frame < 29 || gs.Frame > 31 {
		t.Errorf("transition reported at frame %d, the ROM switches at 30: %+v", gs.Frame, gs)
	}
}

// A ROM with no title must not be given one. Reporting a transition that did not
// happen is worse than reporting none, because the number would be used as a
// warmup and skip real gameplay.
func TestFindGameplayStartFindsNoTitleWhenThereIsNone(t *testing.T) {
	gs, err := FindGameplayStart("../../roms/litmus/litmus_shift_base.bin", "NTSC", 60, 8)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	if gs.Found {
		t.Errorf("a static ROM was reported as having a title ending at frame %d: %+v", gs.Frame, gs)
	}
	if gs.Frame != 0 {
		t.Errorf("with no transition the answer must be frame 0, got %d", gs.Frame)
	}
}

// Stability is what separates a title from a flicker. A search that stops before
// the required run of steady frames must report not-found rather than the first
// change it happened to see.
func TestFindGameplayStartNeedsTheStabilityItClaims(t *testing.T) {
	// Search only as far as the transition itself: there is no room to observe the
	// stability, so the honest answer is not-found.
	gs, err := FindGameplayStart(titleROM, "NTSC", 31, 8)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	if gs.Found {
		t.Errorf("claimed a settled picture without room to observe %d stable frames: %+v",
			gs.StableFor, gs)
	}
	if !gs.Changed {
		t.Error("the playfield does change within 31 frames; Changed should say so")
	}
}
