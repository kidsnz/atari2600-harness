package emu

import "testing"

// TestTIMINTsTwoBitsAreNotBothMeasurements separates a number this repository has been pinning as
// one thing.
//
// `docs/fundamentals-audit.md` records the RIOT timer's expiry as "TIMINT $285 D7 = expired
// ($93=$C0, D7+D6 set)" and `roms/litmus/scenarios/timer.json` locks `ram.0x93 == 192`. Both are
// true of this engine and neither says what D6 IS.
//
// It is not a second expiry bit. The 6532 datasheet — transcribed to the mailing list by Dan Boris
// in 199708 from Rockwell's AIM trainer book — gives the register two independent flags:
//
//	Read and Clear Interrupt Flag ... Bit 7 = Timer IRQ flag
//	                                  Bit 6 = PA7 IRQ flag
//
// and the engine implements exactly that, as two separate booleans (`hardware/riot/timer/timer.go`):
//
//	timintExpired = 0b10000000        if tmr.expired { v |= timintExpired }
//	timintPA7     = 0b01000000        if tmr.pa7     { v |= timintPA7 }
//
// ★So why is D6 set in a ROM that never touches PA7? Because `Timer.Reset()` opens with
// `tmr.pa7 = true`, unconditionally — before the `RandomState` branch, so it is not even part of
// the randomised power-on state. **Half of `$C0` is a hardware fact and half is this emulator's
// initial condition, and the scenario pins the sum.** A sum cannot say which half moved.
//
// Measured here, reading TIMINT before any timer is written and before anything could expire:
//
//	$80  $40  at boot — D6 set with no PA7 edge anywhere, D7 clear
//	$81  $00  reading it cleared the PA7 flag
//	$82  $80  after a real expiry: D7 alone. D6 does NOT come back, because no edge came
//	$84  $00  and reading INTIM clears it, which is the behaviour the audit already describes
func TestTIMINTsTwoBitsAreNotBothMeasurements(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_timint_pa7.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	if got := r[0x00]; got != 0x40 {
		t.Errorf("TIMINT at boot is $%02X, want $40. D6 alone is the whole claim here: the PA7 flag "+
			"is SET before the ROM has done anything, and D7 is clear because nothing has expired. "+
			"If this is $00 the engine has changed its power-on convention and both `$93=$C0` in "+
			"fundamentals-audit.md and `ram.0x93 == 192` in scenarios/timer.json are now wrong — "+
			"silently, because a scenario that pins a sum cannot report which half moved", got)
	}
	if got := r[0x01]; got != 0x00 {
		t.Errorf("reading TIMINT twice gives $%02X the second time, want $00 — the read is supposed "+
			"to clear the PA7 flag, and if it does not then D6 is a level rather than a latch", got)
	}
	if got := r[0x02]; got != 0x80 {
		t.Errorf("TIMINT after a real expiry is $%02X, want $80: D7 alone. Getting $C0 here would "+
			"mean D6 came back without any PA7 edge, which would make the two bits not independent "+
			"after all — and that is the assumption the whole separation rests on", got)
	}
	if got := r[0x04]; got != 0x00 {
		t.Errorf("TIMINT after the INTIM read is $%02X, want $00 — fundamentals-audit.md says "+
			"reading INTIM clears TIMINT, and this is that claim measured rather than restated", got)
	}
	// The INTIM value is recorded so a change in the timer's rate shows up here rather than only
	// in the assertions above, which are about flags and would not notice.
	if got := r[0x03]; got != 0xEF {
		t.Errorf("INTIM reads $%02X at the point TIMINT was sampled, want $EF — the flag results "+
			"above are only meaningful if the timer got where this ROM thinks it did", got)
	}
}
