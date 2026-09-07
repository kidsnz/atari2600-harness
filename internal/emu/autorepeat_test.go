package emu

import (
	"testing"
)

// TestHeldInputRepeatsOnlyIfSomethingMakesIt measures the third use of the input machinery, beside
// the two this repository already records.
//
// Glenn Saunders, 1996, on a Tetris in progress: *"When moving the joystick left and right, **when
// you HOLD the joystick, it should keep moving the piece. It should not require extra taps.** Perhaps
// some extra **'grace time' when a piece is lying flat** so you can 'slide' L-shaped pieces into
// place after they are on the ground. That's how the arcade one work[s]"* 〔`199612/msg00012`〕.
//
// `game-states.md` records edge detection with *"hold-to-repeat bugs gone"* — a record of **removing**
// repetition — and `design-principles.md` records deliberate throttling (a turn-rate governor).
// Neither is deliberate repetition, and a held direction needs exactly that.
//
// `roms/litmus/litmus_autorepeat.asm` runs both policies off the same button, so the difference is
// the policy and nothing else. Measured 2026-09-07 with `DELAY = 16` and `REPEAT = 8`:
//
//	held for  1 frame    edge-only 1   auto-repeat 1
//	held for 16 frames   edge-only 1   auto-repeat 1     <- one frame short of the delay
//	held for 17 frames   edge-only 1   auto-repeat 2     <- the first repeat, exactly at DELAY+1
//	held for 25 frames   edge-only 1   auto-repeat 3     <- and every REPEAT frames after
//	held for 33 frames   edge-only 1   auto-repeat 4
//	held for 60 frames   edge-only 1   auto-repeat 7
//
// ★**Edge detection alone never repeats, however long the button is down.** That is correct for a
// menu and wrong for a direction, and the two policies differ by nine lines of kernel.
//
// ★★**The two numbers are the design**, and they are here so a scenario can assert them instead of a
// person judging them by feel: `DELAY` is how long a deliberate hold has to last before the game
// decides it was deliberate, and `REPEAT` is how fast it then goes.
//
// ★★★**Not measured here: the grace window.** Saunders' second sentence asks for input to keep being
// accepted for some frames *after* a piece lands. That is the same counter with a different trigger —
// a state change rather than a button edge — and this litmus has no landing to hang it on. Named
// rather than left implied.
//
// Found by the mailing-list distillation (helper-1), who noted that harness's record is of a bug
// being removed, so adding repetition has to distinguish the deliberate kind from the accidental one.
func TestHeldInputRepeatsOnlyIfSomethingMakesIt(t *testing.T) {
	const (
		delay  = 16
		repeat = 8
	)

	hold := func(frames int) (edge, rep, held int) {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_autorepeat.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 70; f++ {
			if err := e.SetInput(0, "fire", f < frames); err != nil {
				t.Fatal(err)
			}
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		a, _ := e.PeekRAM(0x80)
		b, _ := e.PeekRAM(0x81)
		c, _ := e.PeekRAM(0x82)
		return int(a), int(b), int(c)
	}

	// The boundary: one frame short of the delay is still one step; one past it is two.
	for _, c := range []struct{ frames, wantRep int }{
		{1, 1},
		{delay, 1},
		{delay + 1, 2},
		{delay + 1 + repeat, 3},
		{delay + 1 + 2*repeat, 4},
	} {
		edge, rep, held := hold(c.frames)
		if held != c.frames {
			t.Fatalf("held for %d frames but the ROM counted %d — the input is not reaching it and "+
				"nothing below means anything", c.frames, held)
		}
		if edge != 1 {
			t.Errorf("held for %d frames, edge-only stepped %d times, want 1. Edge detection is "+
				"supposed to ignore how long a button is down; that is what \"hold-to-repeat bugs "+
				"gone\" means", c.frames, edge)
		}
		if rep != c.wantRep {
			t.Errorf("held for %d frames, auto-repeat stepped %d times, want %d (first at once, "+
				"then after DELAY=%d, then every REPEAT=%d)", c.frames, rep, c.wantRep, delay, repeat)
		}
	}

	// And the long hold, so the rate is checked over several repeats rather than at one boundary.
	_, rep60, _ := hold(60)
	want60 := 1 + 1 + (60-(delay+1))/repeat
	if rep60 != want60 {
		t.Errorf("held for 60 frames, auto-repeat stepped %d times, want %d", rep60, want60)
	}
}
