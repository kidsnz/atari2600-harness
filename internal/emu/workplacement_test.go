package emu

import "testing"

// TestFramePhaseDoesNotChangeWhatTheKernelSees tests a plausible explanation and finds it wrong.
//
// `prove_line_budget` proves the worst case over all paths WITHIN a scanline. Nothing in this
// repository says anything about the frame: NTSC's 262 lines are 3 VSYNC + **37 VBLANK before the
// picture** + 192 visible + **30 overscan after it**, and which of the two non-kernel regions you
// use had never been written down as making a difference.
//
// The list treats it as a fix. Andrew Davie, 200103/msg00056, on a game losing vertical sync: *"the
// amount of time required to move and draw all the cubes is > the available cycles that I can
// provide … **I moved the routine from the overscan to the vertical bl[ank]**"*. Dennis Debro, 2004,
// moved a console-switch check **the other way** for the same class of symptom.
//
// **The obvious explanation is ordering, and it is wrong.** VBLANK runs before the picture and
// overscan after it, so it looks as though work done in overscan should reach the screen a frame
// late. Measured here with the same work in both places: **the kernel sees the same value on the
// same frame either way.** The reason is that the kernel reads the variable at a fixed moment; a
// value written in overscan of frame N and one written in VBLANK of frame N+1 are both simply "what
// is there when frame N+1 draws". There is no phase penalty to find.
//
// **So the fix in the archive is about capacity, not about ordering** — 37 lines against 30 is
// **7 lines, 532 cycles** more room before the beam catches up, which is the whole of it. That
// matters for how the advice should be written: *"move it to VBLANK"* is not a latency trick, it is
// "use the bigger of the two rooms", and the corollary is that a routine which does not fit in 37
// will not fit in 30 either — at which point the remaining moves are the ones the same threads name:
// spread the work across frames, or gate it on the timer (`litmus_askdontwait`), or blank a frame.
//
// Recorded because a plausible mechanism that nobody tests becomes folklore. Found by the
// mailing-list distillation (helper-3), who proposed the ordering hypothesis and marked it as
// unmeasured.
func TestFramePhaseDoesNotChangeWhatTheKernelSees(t *testing.T) {
	const (
		addrPhase = 0x80
		addrXpos  = 0x81
		addrDrawn = 0x82
	)
	run := func(t *testing.T, phase uint8) []int {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_workplacement.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(addrPhase, phase); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(addrXpos, 0); err != nil {
			t.Fatal(err)
		}
		var seen []int
		for f := 0; f < 5; f++ {
			if err := e.RunFrames(1); err != nil {
				t.Fatal(err)
			}
			d, err := e.PeekRAM(addrDrawn)
			if err != nil {
				t.Fatal(err)
			}
			seen = append(seen, int(d))
		}
		return seen
	}

	inVBlank := run(t, 0)
	inOverscan := run(t, 1)

	for i := range inVBlank {
		if inVBlank[i] != inOverscan[i] {
			t.Errorf("frame %d: the kernel used %d when the work ran in VBLANK and %d when it ran in "+
				"overscan. If these ever differ, moving work between the two regions IS a latency "+
				"change and the advice in `fundamentals-audit` needs rewriting", i, inVBlank[i], inOverscan[i])
		}
	}

	// Control: the fixture must actually be animating, or "no difference" is the difference between
	// two frozen pictures and means nothing.
	if inVBlank[0] == inVBlank[len(inVBlank)-1] {
		t.Errorf("the value never changed across five frames (%v) — the fixture is not doing the work "+
			"whose placement is under test", inVBlank)
	}
	for i := 1; i < len(inVBlank); i++ {
		if inVBlank[i] != inVBlank[i-1]+1 {
			t.Errorf("the work should advance the value by exactly one per frame; got %v", inVBlank)
			break
		}
	}
}
