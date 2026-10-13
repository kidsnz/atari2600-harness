package emu

import "testing"

// TestStackOverlapDetector: the watch must flag a byte that is both variable and stack, and
// must say nothing about the same program with the variable out of the way, about the clear
// at reset (which pushes through every byte of RAM), or about a kernel that aims the stack
// pointer at a TIA register and stores with a push.
func TestStackOverlapDetector(t *testing.T) {
	run := func(rom string) *StackOverlapHit {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		hit, err := e.WatchStackOverlap(4)
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}

	for _, clean := range []string{"stack_overlap_clean.bin", "litmus_stack_trick.bin"} {
		if hit := run("../../roms/litmus/" + clean); hit != nil {
			t.Fatalf("%s must NOT be flagged, got $%02X pushed @pc $%04X and used @pc $%04X",
				clean, hit.Addr, hit.PushPC, hit.VarPC)
		}
	}
	hit := run("../../roms/litmus/stack_overlap_trap.bin")
	if hit == nil {
		t.Fatalf("stack_overlap_trap (a variable at $FD under a two-deep call) must be flagged, got no hit")
	}
	if hit.Addr != 0x00FD {
		t.Fatalf("expected the overlap at $FD, got $%02X (pushed @pc $%04X, used @pc $%04X)", hit.Addr, hit.PushPC, hit.VarPC)
	}
}
