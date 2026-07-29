package emu

import "testing"

// watch_ram traps a RAM change and names the instruction that made it. Collisions
// had no equivalent (G7): CXxx latches are sticky and a game clears them with
// CXCLR every frame, so sampling at a frame boundary answers "did it happen" and
// nothing else — and by the time an instruction retires the beam has moved on, so
// WHERE it happened is gone.
//
// WatchCollision records the beam position at the colour clock each latch is first
// set. litmus_collide_all is built to make every pair fire, which is what makes it
// the positive control; a ROM whose objects never touch is the negative one, and
// both directions are asserted because a trap that fires on everything is as
// useless as one that fires on nothing.
func TestCollisionTrapReportsWhereItHappened(t *testing.T) {
	t.Run("every pair on a ROM built to collide", func(t *testing.T) {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_collide_all.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		hits, err := e.WatchCollision(nil, 6)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != len(CollisionPairNames()) {
			t.Errorf("litmus_collide_all is built to fire every pair; %d of %d reported",
				len(hits), len(CollisionPairNames()))
		}
		for _, h := range hits {
			// The site is the point of the tool. A zeroed one would mean the position
			// was reconstructed after the fact rather than captured at the colour clock.
			if h.At.PC == 0 {
				t.Errorf("%s: no PC recorded, so the trap did not capture the moment", h.Pair)
			}
			if h.At.Clock < -68 || h.At.Clock > 159 {
				t.Errorf("%s: clock %d is outside the beam's coordinate range", h.Pair, h.At.Clock)
			}
		}
	})

	t.Run("silent on a ROM whose objects never touch", func(t *testing.T) {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		// shared_setxpos parks two players, two missiles and the ball at separate
		// fixed columns and never moves them.
		if err := e.LoadROM("../../roms/techniques/shared_setxpos.bin"); err != nil {
			t.Skipf("ROM unavailable: %v", err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		hits, err := e.WatchCollision(nil, 6)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("%d collision(s) reported on a ROM whose objects sit at separate columns: %+v",
				len(hits), hits)
		}
	})

	t.Run("a misspelt pair is an error, not silence", func(t *testing.T) {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM("../../roms/litmus/litmus_collide_all.bin"); err != nil {
			t.Skipf("litmus unavailable: %v", err)
		}
		if _, err := e.WatchCollision([]string{"p0_pf", "m0_qq"}, 2); err == nil {
			t.Error("an unknown pair name returned no error; an empty result would then read as " +
				"'nothing collided' when the name was simply wrong")
		}
	})
}
