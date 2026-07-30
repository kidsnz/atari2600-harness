package emu

import "testing"

// rippleHistogram samples every recorded colour clock over `frames` frames and
// returns, per ripple value, how often it appeared and how it was classified.
func rippleHistogram(t *testing.T, rom string, frames int) (count map[uint8]int, naiveWrongActive, naiveWrongIdle, justEnded int) {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	count = map[uint8]int{}
	for f := 0; f < frames; f++ {
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
		for i := range e.hmRipple {
			fl := e.hmFlags[i]
			if fl&hmRecorded == 0 {
				continue
			}
			r := e.hmRipple[i]
			active := fl&hmRippleActive != 0
			count[r]++
			if fl&hmJustEnded != 0 {
				justEnded++
			}
			// The "obvious" activity test a reader would reach for.
			naive := r != 255
			if naive && !active {
				naiveWrongActive++
			}
			if !naive && active {
				naiveWrongIdle++
			}
		}
	}
	return count, naiveWrongActive, naiveWrongIdle, justEnded
}

// TestRippleActiveIsNotRippleNot255 puts a test behind the claim in HmoveState's
// doc comment: that `Ripple != 255` is wrong at BOTH ends of the ripple.
//
// The comment asserted it and nothing checked it, so a reader could "simplify" a
// consumer to the obvious predicate and no test would notice. Measured on
// litmus_hmove over 6 frames: 28 samples sit at ripple 0 with the counter NOT active
// (the naive test would call them active) and 28 samples sit at ripple 255 WITH it
// active because the ripple expired on that very clock (the naive test would call
// them idle). Both ends are real and both are one full HMOVE's worth of clocks.
func TestRippleActiveIsNotRippleNot255(t *testing.T) {
	count, wrongActive, wrongIdle, justEnded := rippleHistogram(t, "../../roms/litmus/litmus_hmove.bin", 6)
	if count[0] == 0 {
		t.Fatal("ripple never reached 0 — the fixture is not exercising HMOVE and this test proves nothing")
	}
	if wrongActive == 0 {
		t.Errorf("no sample where `ripple != 255` would wrongly report ACTIVE; the doc comment on " +
			"HmoveState.RippleActive claims this end exists (ripple 0 idle), and it is unverified")
	}
	if wrongIdle == 0 {
		t.Errorf("no sample where `ripple != 255` would wrongly report IDLE; the doc comment claims " +
			"this end exists (the clock the counter expires on), and it is unverified")
	}
	if wrongIdle != justEnded {
		t.Errorf("the ripple==255-but-active samples (%d) should be exactly the just-ended ones (%d)",
			wrongIdle, justEnded)
	}
	t.Logf("`ripple != 255` misclassifies %d samples as active and %d as idle over 6 frames",
		wrongActive, wrongIdle)
}

// TestRippleObservableSeriesStartsAt14 pins what the per-colour-clock recorder can
// actually see. HmoveState.Ripple was documented as counting "15 down to 0"; the
// engine does load 15, but it is gone before the next sample point, so an observer
// sees 14 -> 0. Measured across four HMOVE-using ROMs: 15 appears exactly once in
// each while every other value appears once per HMOVE.
//
// This matters because 15 is the engine's own "just started" marker
// (hmove.justStarted is Ripple == 15). A consumer that waits for it through this
// interface would miss almost every ripple.
func TestRippleObservableSeriesStartsAt14(t *testing.T) {
	for _, rom := range []string{
		"../../roms/litmus/litmus_hmove.bin",
		"../../roms/techniques/hscroll.bin",
		"../../roms/techniques/multicolor48.bin",
		"../../roms/techniques/venetian.bin",
	} {
		count, _, _, _ := rippleHistogram(t, rom, 6)
		if count[14] == 0 {
			t.Errorf("%s: ripple 14 never observed — no HMOVE ran, so nothing here is checked", rom)
			continue
		}
		// The series must be complete: every value from 14 down to 0 seen the same
		// number of times. A gap would mean the recorder is skipping clocks.
		for v := uint8(0); v <= 14; v++ {
			if count[v] != count[14] {
				t.Errorf("%s: ripple %d seen %d times, ripple 14 seen %d — the sampled series has a gap",
					rom, v, count[v], count[14])
			}
		}
		if count[15] >= count[14] {
			t.Errorf("%s: ripple 15 seen %d times vs 14 seen %d — 15 has become observable, so the "+
				"comment on HmoveState.Ripple (sampled series is 14..0) needs re-measuring",
				rom, count[15], count[14])
		}
		t.Logf("%-40s ripple 15 seen %d, 14..0 seen %d each", rom, count[15], count[14])
	}
}
