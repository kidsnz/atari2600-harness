package vismatch

import "testing"

// litmus_shift_down8 is byte-for-byte litmus_shift_base with eight scanlines
// moved from the visible area into VBLANK, so its picture sits exactly eight
// lines lower and the frame still totals 262. The amount is known by
// construction, which is the point: a detector graded against a shift inferred
// from its own output would agree with itself whatever it reported.
const (
	shiftBase  = "../../roms/litmus/litmus_shift_base.bin"
	shiftDown8 = "../../roms/litmus/litmus_shift_down8.bin"
)

func gridOrSkip(t *testing.T, rom string) *Grid {
	t.Helper()
	g, err := ExtractROM(rom, "NTSC", 4, false)
	if err != nil || g == nil {
		t.Skipf("ROM unavailable (%s): %v", rom, err)
	}
	return g
}

// The whole reason this exists: when every band is off by the same amount, the
// band diff is N ways of saying one number, and reading it back into that number
// by hand is where a 3-pixel error crept in during field use. So the number must
// come out exactly, and it must come out with the sign the report claims.
func TestFindVerticalShiftRecoversAPlantedOffset(t *testing.T) {
	base := gridOrSkip(t, shiftBase)
	down := gridOrSkip(t, shiftDown8)

	v := FindVerticalShift(base, down, 24)
	if v.Shift != 8 {
		t.Errorf("shift = %d, want +8 (mine sits 8 scanlines lower); %+v", v.Shift, v)
	}
	if v.MismatchAtBest != 0 {
		t.Errorf("the two pictures are identical apart from the offset, so the shift should explain "+
			"ALL of the mismatch; %d cells remain", v.MismatchAtBest)
	}
	if v.MismatchAtZero == 0 {
		t.Error("premise broken: the two ROMs should differ before the shift is applied")
	}

	// And the other direction, so a sign error cannot pass.
	rev := FindVerticalShift(down, base, 24)
	if rev.Shift != -8 {
		t.Errorf("reversed shift = %d, want -8; %+v", rev.Shift, rev)
	}
}

// A shift that explains nothing must not be presented as an explanation. Two
// pictures can differ in ways no alignment fixes, and reporting a "best" shift
// there would invent a cause.
func TestFindVerticalShiftReportsWhenNothingAligns(t *testing.T) {
	base := gridOrSkip(t, shiftBase)
	other := gridOrSkip(t, "../../roms/litmus/motion_glide.bin")

	v := FindVerticalShift(base, other, 24)
	if v.Removed > 0.5 {
		t.Errorf("two unrelated pictures should not be explained by a vertical shift; "+
			"removed %.0f%% at shift %d", 100*v.Removed, v.Shift)
	}
	if v.Shift == 0 && v.MismatchAtBest != v.MismatchAtZero {
		t.Errorf("shift 0 must report the same count as the zero baseline: %+v", v)
	}
}

// A ROM against itself has no offset, and the description has to say so rather
// than naming a shift of zero as a finding.
func TestFindVerticalShiftIsZeroForIdenticalFrames(t *testing.T) {
	a := gridOrSkip(t, shiftBase)
	b := gridOrSkip(t, shiftBase)
	v := FindVerticalShift(a, b, 24)
	if v.Shift != 0 || v.MismatchAtZero != 0 {
		t.Errorf("identical ROMs: got %+v", v)
	}
	if got := v.Describe(); got == "" || v.MismatchAtZero != 0 {
		t.Errorf("describe = %q", got)
	}
}
