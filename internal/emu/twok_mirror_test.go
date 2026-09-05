package emu

import "testing"

// TestThe2KMirrorMakesTwoSeparateTrapsNotOne resolves a pair of archive reports that look like they
// contradict each other and do not.
//
// `capability-gap-audit.md` CMB-6 says Combat "squats live game data on the CPU IRQ/BRK vector slot"
// and that "a 2K cart mirrors $F000-$F7FF into $F800-$FFFF, so $F7FE/$F7FF *are* the $FFFE/$FFFF
// vector ... This booby-traps a 2K->4K port." The archive says something that sounds like the same
// thing pointing the other way:
//
//	1997-03, Chris Pepin: "That's one of the problems I was having with games not working.
//	                       When I doubled them up to 4k, they worked fine."
//	2003-10: a 2K game's BRK reads its vector from $FFFE/$FFFF, which on a Supercharger is
//	                       $1FF8/$1FF9 — the control hotspots.
//
// "2K breaks, 4K fixes it" against "2K->4K is the trap". ★Measured here: **it is two traps, and the
// address decides.** Under the same 2K mirror the hazards sit on different byte pairs, six apart:
//
//	file offset $7FE/$7FF -> $F7FE/$F7FF == $FFFE/$FFFF   the IRQ/BRK vector      (CMB-6)
//	file offset $7F8/$7F9 -> $F7F8/$F7F9 == $FFF8/$FFF9   the Supercharger hotspot
//
// So a 2K image can trip either, both or neither, and "double it to 4K" only helps the second: at
// 4K the top half stops being a mirror of the bottom, so the bytes that were landing on $1FF8/$1FF9
// stop doing so — while the vector slot at $FFFE/$FFFF still holds whatever the author put there.
// **The two reports do not contradict each other.** Found by the mailing-list distillation
// (helper-3), who confirmed both verbatims and explicitly declined to judge which was right.
func TestThe2KMirrorMakesTwoSeparateTrapsNotOne(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_2k_mirror.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	if r[0x00] != 0x5C || r[0x02] != 0x5C {
		t.Errorf("$F7F8 reads $%02X and $FFF8 reads $%02X, want $5C for both — the 2K mirror is not "+
			"folding the low half onto the high half, and every claim below rests on that fold",
			r[0x00], r[0x02])
	}
	if r[0x01] != 0xA3 || r[0x03] != 0xA3 {
		t.Errorf("$F7FE reads $%02X and $FFFE reads $%02X, want $A3 for both — the vector pair is not "+
			"mirrored, which is the half of CMB-6 that makes a 2K->4K port a booby trap",
			r[0x01], r[0x03])
	}
	if r[0x04] != 1 {
		t.Error("the two markers came back equal, so this ROM proves nothing: the whole point is that " +
			"$FFF8/$FFF9 and $FFFE/$FFFF are DIFFERENT locations six bytes apart, one carrying the " +
			"Supercharger hotspot hazard and the other the vector hazard. If they read the same, the " +
			"ROM's own markers have been placed wrongly and the distinction is untested")
	}
}
