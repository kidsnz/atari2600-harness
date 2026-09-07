package emu

import (
	"testing"
)

// TestPaintingTheBackgroundColourIsNotErasing settles a 1998 question and shows which instrument here
// can answer it.
//
// Ruffin Bailey, playing someone else's game: *"why can I see a **silhouetted tank** over on the far
// right side of the ground in one player games?"* Piero Cavina, who wrote it: *"it might be the
// second player's sprite, which is **always there, but painted in black** in 1-player games. But **I
// don't see it here**.."* 〔`199801/msg00038` and `msg00045`〕. The author could not reproduce what a
// third party saw — the signature of something that depends on the display rather than the program,
// which is the direction `known-traps.md` names as this harness's blind one.
//
// What can be settled is what the program does. `roms/litmus/litmus_paint_as_background.asm` draws
// three bands: a visible player, the same player painted `COLUBK`'s colour, and no player at all.
// Measured 2026-09-07:
//
//	band            ReadRow (pixels)              DecomposeRow (elements)
//	visible         BG x129, WHITE x8, BG x23     BG, P1, BG
//	painted as BG   BG x160                       BG, P1, BG      <- still there
//	not drawn       BG x160                       BG
//
// ★**The middle two rows are pixel-identical and element-different.** Every colour comparison in this
// repository — `vismatch`, a golden frame, `cmd/still`'s diff — reads "painted the background colour"
// as "not drawn". `DecomposeRow` does not, and that is what it is for.
//
// ★★**So "paint it out" is not "remove it".** The object still costs its store, still occupies a
// player slot, and still sets collision latches (measured separately: an object placed as decor
// latches `CXxx` exactly as a live one does). A kernel that hides a sprite this way has not freed
// anything — it has only stopped the author from seeing what is still running.
//
// ★★★And the third party who DID see it is outside what this measurement can reach. A television is
// not a pixel comparator; the 1998 report is evidence about a display, and nothing here reproduces
// displays. Recorded rather than dismissed.
//
// Found by the mailing-list distillation (helper-1), who first eliminated the obvious hypothesis by
// reading the engine: `COLUP0`, `COLUP1`, `COLUPF` and all three of their objects mask with `& 0xfe`
// identically, so there is no bit-0 asymmetry between a player's colour and the background's.
func TestPaintingTheBackgroundColourIsNotErasing(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_paint_as_background.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}

	pixelRuns := func(line int) int {
		runs, _, err := e.ReadRow(line)
		if err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		return len(runs)
	}
	hasP1 := func(line int) bool {
		elems, _, err := e.DecomposeRow(line)
		if err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		for _, r := range elems {
			if r.Element == "P1" {
				return true
			}
		}
		return false
	}

	const (
		visible = 45 // band A
		painted = 65 // band B
		absent  = 85 // band C
	)

	// The control: a visible player must be visible, or the two rows below are about a ROM that
	// never draws anything.
	if pixelRuns(visible) < 3 || !hasP1(visible) {
		t.Fatalf("the visible band shows %d pixel runs and P1 present=%v; it should show three runs "+
			"and a P1", pixelRuns(visible), hasP1(visible))
	}

	// The measurement: painted-as-background and not-drawn are identical to the pixels.
	if pixelRuns(painted) != 1 || pixelRuns(absent) != 1 {
		t.Errorf("painted-as-background gives %d pixel runs and not-drawn %d; both should be 1, "+
			"because a sprite in the background's colour is invisible to any colour comparison",
			pixelRuns(painted), pixelRuns(absent))
	}

	// And different to the elements.
	if !hasP1(painted) {
		t.Error("DecomposeRow reports no P1 on the painted-as-background band. That is the one " +
			"instrument here that can tell it from an empty line, and if it cannot, nothing can")
	}
	if hasP1(absent) {
		t.Error("DecomposeRow reports a P1 on the band where GRP1 is zero — it is reporting the " +
			"object's existence rather than its drawing, and the distinction above collapses")
	}
}
