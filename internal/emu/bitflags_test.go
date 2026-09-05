package emu

import "testing"

// TestBITSetsThreeFlagsFromTwoPlaces closes a gap in `docs/resources.md`, which says of the
// collision registers: "8 read-only registers, each with two latches in D7/D6, sticky. Test with
// `BIT CXxx` -> `BMI`(D7)/`BVS`(D6)." True, and one flag short.
//
// Bill Heineman, stella-list 200207/msg00281:
//
//	Moves Bit #7 from the memory location and places it in the N flag ...
//	Moves Bit #6 from the memory location and places it in the V flag ...
//	[A AND memory] is placed in the Z flag ... the accumulator is NOT affected in anyway.
//	This way, you can do a quick bit test without [damaging the accumulator]
//
// So one instruction answers three questions from **two different sources**: N and V read the
// memory's own bits regardless of what the accumulator holds, and Z is a MASK TEST of A against
// that memory. Measured here, including the case that separates the two sources: with A = $00 the
// AND result is zero and Z sets, and **N still comes back set** from the memory's bit 7.
//
// ★Why it is worth having: the Z half tests an arbitrary mask without destroying the accumulator,
// where `AND` would. And it lands on the register `litmus_timint_pa7` measured — TIMINT's D7 is the
// timer flag and D6 is the PA7 flag — so `BIT TIMINT / BMI expired / BVS pa7` reads both in one
// instruction rather than two.
//
// ★★The limit belongs with it: **BIT has no immediate mode** (Chris Wilkson, 199806/msg00118), so
// the mask must live in memory. Andrew Davie corrected himself in that same thread after saying
// otherwise — "my memory got mixed up with the 65816" — which is why the limit is recorded next to
// the capability instead of being left for the next person to rediscover.
func TestBITSetsThreeFlagsFromTwoPlaces(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_bit_flags.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		addr int
		want uint8
		what string
	}{
		{0x00, 1, "N after BIT on $80 — N is the MEMORY's bit 7"},
		{0x01, 0, "V after BIT on $80 — bit 6 is clear there"},
		{0x02, 0, "N after BIT on $40 — bit 7 is clear there"},
		{0x03, 1, "V after BIT on $40 — V is the MEMORY's bit 6"},
		{0x04, 1, "Z after BIT of A=$F0 against $0F — no bits shared, so the AND is zero"},
		{0x05, 1, "Z CLEAR after BIT of A=$0F against $0F — bits shared, so the AND is not zero"},
		{0x07, 1, "N after BIT on $80 with A=$00 — N comes from memory even when the AND is zero, " +
			"which is the case that proves N and Z read different things"},
	} {
		if got := r[tc.addr]; got != tc.want {
			t.Errorf("$%02X is %d, want %d — %s", 0x80+tc.addr, got, tc.want, tc.what)
		}
	}

	if got := r[0x06]; got != 0x0F {
		t.Errorf("the accumulator is $%02X after a BIT, want $0F — BIT must not touch A. If it "+
			"does, every use of it as a non-destructive mask test in this repository is wrong", got)
	}
}
