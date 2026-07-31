package emu

import "testing"

// TestHmcleardoesNotClearCollisions machine-locks three claims CLAUDE.md files under
// "constants you must never get wrong": the CXxx latches are STICKY, CXCLR clears them,
// and **HMCLR does not** — "HMCLR = clear the motion registers (a different thing)".
//
// The bit assignment already had a pure-function test. These three did not: stickiness
// appeared only in a comment, and nothing checked the HMCLR distinction at all — which
// is the one a reader can actually get wrong, since the two names differ by two letters
// and both read as "clear something". Confusing them leaves every collision latched
// forever, and a game that tests collisions then sees them fire on the first frame and
// never stop.
//
// litmus_cxclr collides P0 with a fully lit playfield, then snapshots CXP0FB into RAM
// at three points: before any clear, after HMCLR, after CXCLR. RAM because these are
// write-only strobes — poking them from outside does not persist.
func TestHmclearDoesNotClearCollisions(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_cxclr.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}
	ram, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	const d7 = 0x80
	before, afterHM, afterCX := ram[0x00], ram[0x01], ram[0x02] // $80,$81,$82

	if before&d7 == 0 {
		t.Fatalf("no collision latched ($80 = $%02X) — P0 sits on a solid playfield for the whole "+
			"visible area, so the fixture has stopped colliding and the rest proves nothing", before)
	}
	if afterHM&d7 == 0 {
		t.Errorf("HMCLR cleared the collision latch ($81 = $%02X): HMCLR clears the MOTION registers, "+
			"a different thing, and code that relies on the distinction would silently lose its "+
			"collision", afterHM)
	}
	if afterHM != before {
		t.Errorf("HMCLR changed CXP0FB from $%02X to $%02X — it should not touch it at all",
			before, afterHM)
	}
	if afterCX&d7 != 0 {
		t.Errorf("CXCLR left the collision latch set ($82 = $%02X)", afterCX)
	}
	t.Logf("CXP0FB: $%02X collided -> $%02X after HMCLR (sticky) -> $%02X after CXCLR",
		before, afterHM, afterCX)
}
