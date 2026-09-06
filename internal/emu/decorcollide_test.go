package emu

import "testing"

// TestDecorationStillCollides measures that an object drawn purely as SCENERY still sets
// its collision latch. The TIA compares geometry; it has no notion of what a program means
// by an object, so an object borrowed for a second job keeps colliding at its first one.
//
// This repository teaches the borrowing. `design-principles.md` has "Multi-kernel = reuse
// one object per region" and cites the list on "intelligent sprite flicker and reuse"; the
// technique catalogue is full of objects doing double duty. What it did not have, until
// this row, is the bill.
//
// The list has the bug report, written by the author of the game it happened in — Piero
// Cavina on INV 〔stella-list `199801/msg00197`, 1998-01〕:
//
//	This must happen because the program is checking missile/invaders collisions even
//	when the missile is used for the explosion.
//
// Shoot one of two adjacent invaders in the top row and BOTH explode: the missile had been
// repurposed as the explosion graphic and was still being tested against the invaders.
//
// ★The negative control is band B — same geometry, `ENAM1` cleared. Without it a TIA that
// latched unconditionally would pass, and so would a fixture whose read address was wrong.
//
// ★★The fixture itself is the second lesson. Its first version strobed RESP0 and RESM1
// three CPU cycles apart and read NO collision, which looks exactly like the trap being
// absent. `DecomposeRow` showed the objects at clocks 3..10 and 25..32 — twenty-two apart,
// never touching. The TIA was right and the measurement was wrong, and only looking at the
// pixels separated the two. The player is quad-width now so the overlap is not a near miss.
func TestDecorationStillCollides(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_decor_collides.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}

	// The objects have to actually overlap, or the bands below compare nothing. P0 is
	// quad-width and M1 is eight clocks inside it, so the row reports a single P0 run
	// wide enough to contain the missile.
	runs, _, err := e.DecomposeRow(50)
	if err != nil {
		t.Fatal(err)
	}
	var p0 *ElemRun
	for i := range runs {
		if runs[i].Element == "P0" {
			p0 = &runs[i]
		}
	}
	if p0 == nil || p0.Len < 32 {
		t.Fatalf("band A does not draw a quad-width P0 (%v) — the fixture is not posing the "+
			"question, and a no-collision result below would mean nothing", runs)
	}

	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	bandA, bandB := r[0x00], r[0x01]
	t.Logf("band A  M1 enabled, inside P0     CXM1P=$%02X  D7=%v", bandA, bandA&0x80 != 0)
	t.Logf("band B  M1 disabled, same strobes CXM1P=$%02X  D7=%v", bandB, bandB&0x80 != 0)

	if bandA&0x80 == 0 {
		t.Errorf("band A: CXM1P D7 clear ($%02X) — a missile drawn over the player did not "+
			"latch. Either the trap is not real here or the fixture stopped overlapping", bandA)
	}
	if bandB&0x80 != 0 {
		t.Errorf("band B: CXM1P D7 SET ($%02X) with ENAM1 cleared — the latch is not "+
			"measuring the missile, so band A proves nothing", bandB)
	}
}
