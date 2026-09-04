package emu

import "testing"

// TestFlickerCollisionAttribution settles what fundamentals-audit called "a verifiable
// pattern once we do flicker". We have had flicker since technique #10, and
// roms/techniques/flicker_multiplex.asm touches no collision register at all, so the
// pattern was never built — the mark was waiting on a condition that had already been
// met, which is a different kind of stale than a wrong number.
//
// The claim worth machine-locking is not "a collision latches" (litmus_cxclr has that).
// It is that when two objects share one slot on alternate frames, the latch you read
// belongs to whichever object was drawn in THAT frame — and that this is true only
// because CXCLR runs every frame.
//
// Proving the "only because" needs a control where CXCLR does not run, and that
// control's expected result depends on a latch surviving a frame boundary. Nothing
// measured that. litmus_cxclr takes all three of its snapshots inside one frame (its
// own header says so) and strobes CXCLR every frame, so a latch never gets the chance
// to cross a boundary there. So group 1 below measures it here, first, and the control
// then rests on our own measurement rather than on an assumption.
//
// Group 3 is not a separate fixture: it is the negative control, and the test fails if
// it ever agrees with group 2, because two identical columns would prove nothing about
// CXCLR. Group 4 inverts the phase so that "the latch set on even frames" cannot be
// mistaken for the result.
//
// Every cell is normalised to 0 or 1 by the ROM. Raw CXP0FB must never be stored: only
// D7/D6 are driven and the rest of the byte is the last value the CPU put on the bus,
// which is why scenarios/litmus_cxclr.json pins 130 and 2 instead of 128 and 0. Pinning
// a raw read pins the preceding instruction too.
func TestFlickerCollisionAttribution(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_flicker_attrib.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(32); err != nil {
		t.Fatal(err)
	}
	ram, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	at := func(a int) byte { return ram[a-0x80] }
	col := func(base int) [8]byte {
		var v [8]byte
		for i := range v {
			v[i] = at(base + i)
		}
		return v
	}

	// The ROM must have run all 28 measured frames, or the columns below are partly
	// whatever the RAM clear left behind.
	if n := at(0x80); n < 28 {
		t.Fatalf("only %d frames ran; the ROM needs 28 to fill every column", n)
	}

	// Group 1 — the premise, measured here rather than assumed.
	// F0 clear+lit, F1 lit (no clear), F2 blank (no clear), F3 clear+blank.
	g1 := [4]byte{at(0x90), at(0x91), at(0x92), at(0x93)}
	if g1 != [4]byte{1, 1, 1, 0} {
		t.Fatalf("group 1 = %v, want [1 1 1 0]: a set latch must survive into the next frame "+
			"(cell 2 is read with the playfield blanked, so a 0 there means it did not) and "+
			"CXCLR must be what clears it (cell 3)", g1)
	}

	// Group 2 — with CXCLR every frame, the latch tracks what was drawn this frame.
	want := [8]byte{1, 0, 1, 0, 1, 0, 1, 0}
	latch, drawn := col(0xA0), col(0xB0)
	if latch != want {
		t.Fatalf("group 2 latches = %v, want %v", latch, want)
	}
	if drawn != want {
		t.Fatalf("group 2 cause = %v, want %v — the ROM's own record of which object it drew "+
			"disagrees with the pattern, so the fixture is not doing what the test claims", drawn, want)
	}
	for i := range latch {
		if latch[i] != drawn[i] {
			t.Fatalf("frame %d: latch %d but drew %d — attribution does not follow the frame",
				i, latch[i], drawn[i])
		}
	}

	// Group 3 — the negative control. Same alternation, CXCLR only on the first frame.
	g3 := col(0xC0)
	for i, v := range g3 {
		if v != 1 {
			t.Fatalf("group 3 = %v: cell %d is 0, so the latch cleared without CXCLR and the "+
				"control proves nothing", g3, i)
		}
	}
	if g3 == latch {
		t.Fatal("group 3 is identical to group 2, so nothing here depends on CXCLR at all")
	}

	// Group 4 — phase inverted. Rules out "it sets on even frames" as the explanation.
	g4, wantInv := col(0xD0), [8]byte{0, 1, 0, 1, 0, 1, 0, 1}
	if g4 != wantInv {
		t.Fatalf("group 4 = %v, want %v: with the alternation inverted the latches must invert "+
			"too, otherwise frame parity is the cause rather than what was drawn", g4, wantInv)
	}
	if g4 == latch {
		t.Fatal("group 4 matches group 2 despite the inverted phase — the result tracks the " +
			"frame number, not the object")
	}
}
