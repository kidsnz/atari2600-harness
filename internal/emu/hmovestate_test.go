package emu

import "testing"

// HMOVE is the 2600's most reliable source of "the sprite landed somewhere I did
// not predict", and the only thing observable about it used to be the final
// position. The machinery producing that position — the strobe's delay, and the
// ripple counter that hands extra ticks to the sprites — was invisible, so a
// wrong landing could be guessed at but not watched.
//
// The counter's behaviour is fixed by the hardware, not by anything computed
// here: after the strobe the ripple counts down from 15 and rests at 255 (−1).
// So the assertions are against the TIA's own definition.
func TestHmoveRippleIsRecordedPerColourClock(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}

	var got HmoveSpan
	for sl := 0; sl < 262; sl++ {
		sp, err := e.HmoveOnScanline(sl)
		if err == nil && sp.Latched {
			got = sp
			break
		}
	}
	if !got.Latched {
		t.Fatal("no scanline reported an HMOVE latch; litmus_hmove strobes HMOVE every frame, so " +
			"either the premise is broken or the state is not being recorded")
	}

	// The headline claim: HMOVE is NOT instantaneous. If this were a single-clock
	// event, reading a sprite position right after the strobe would be safe, and
	// every "it moved to the wrong place" bug would need another explanation.
	if got.RippleTicks < 30 {
		t.Errorf("the ripple counter was active for only %d colour clocks; HMOVE hands out ticks over "+
			"a span, and a span this short would mean the recording is missing clocks (%+v)",
			got.RippleTicks, got)
	}
	if got.StrobeClock == -999 {
		t.Error("the strobe's delay was never observed pending; the gap between the HMOVE write and " +
			"the ripple starting is the part that is invisible from the position alone")
	}
	if got.StrobeClock > got.RippleStart {
		t.Errorf("the strobe delay (%d) must be observed before the ripple starts (%d)",
			got.StrobeClock, got.RippleStart)
	}

	// The counter must count DOWN, and reach the bottom. A counter stuck near its
	// start would still produce a plausible-looking span.
	if len(got.Values) < 8 {
		t.Fatalf("ripple took only %d distinct values %v; it counts 15 down to 0", len(got.Values), got.Values)
	}
	prev := int(got.Values[0]) + 1
	for _, v := range got.Values {
		if v == 255 { // the idle/-1 resting value, only legitimate at the end
			continue
		}
		if int(v) >= prev {
			t.Errorf("ripple values are not strictly descending: %v", got.Values)
			break
		}
		prev = int(v)
	}
	if last := got.Values[len(got.Values)-1]; last != 255 && last > 1 {
		t.Errorf("ripple stopped at %d; it should run down to 0/1 and rest at 255 — %v", last, got.Values)
	}
}

// A ROM that never strobes HMOVE must never show the machinery running.
// Without this, a recorder that latched a constant would satisfy the test above
// and tell every caller the same lie.
func TestHmoveIdleWhenNothingStrobesIt(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	// litmus_nusiz_all places its sprites with RESPx only and never writes HMOVE.
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_all.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}
	sampled := 0
	for sl := 0; sl < 262; sl++ {
		sp, err := e.HmoveOnScanline(sl)
		if err != nil || !sp.Recorded {
			continue
		}
		sampled++
		if sp.Latched || sp.RippleTicks > 0 {
			t.Fatalf("scanline %d reports HMOVE activity (%+v) on a ROM that never strobes it — "+
				"the recorder is not reading real state", sl, sp)
		}
	}
	if sampled == 0 {
		t.Fatal("no scanline was recorded at all, so this proves nothing")
	}
	t.Logf("%d scanlines recorded, none showing HMOVE activity", sampled)
}

// A scanline that was never sampled must say so rather than report a calm line.
// Absence of data and absence of activity are different answers.
func TestHmoveSpanDistinguishesUnrecordedFromIdle(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_hmove.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	// No frames run: nothing has been sampled yet.
	sp, err := e.HmoveOnScanline(100)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Recorded {
		t.Errorf("scanline 100 reported as recorded before any frame ran: %+v", sp)
	}
	if sp.Latched || sp.RippleTicks != 0 {
		t.Errorf("an unrecorded scanline must not report activity: %+v", sp)
	}
}
