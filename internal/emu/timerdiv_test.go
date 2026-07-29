package emu

import "testing"

// The RIOT's divider internally drops to 1T when INTIM underflows. A write to
// TIM8T/TIM64T/T1024T whose own cycles straddle that instant loses the race on
// real hardware: the requested divider does not take, the timer keeps running at
// 1T, and an interval meant to last hundreds of scanlines ends almost at once.
//
// Gopher2600 does not reproduce it — Timer.Update assigns the requested divider
// unconditionally — so the consequence is invisible here and only the HAZARD can
// be reported.
//
// What this test locks is the NEGATIVE direction, which is the one with a witness.
// litmus_timerwrap_nearmiss was built to land its store on the underflow and,
// measured, does not: at the store the timer reads INTIM=255, ticksRemaining=0,
// divider=8, putting the wrap 255*8 = 2040 cycles away against a 4-cycle store.
// A divider write shortly after a wrap is the ordinary safe shape, and a detector
// that flagged it would cry wolf on every timer-driven kernel in the corpus.
//
// The POSITIVE case has no ROM witness yet, and that is stated rather than implied
// by a passing test: arranging a store whose cycles contain the wrap needs
// cycle-level tuning of the polling exit. Until then the detector is code without a
// witness, which is why its silence must not be read as "no hazards exist".
func TestTimerDividerHazardDoesNotCryWolf(t *testing.T) {
	roms := []string{
		"../../roms/litmus/litmus_timerwrap_nearmiss.bin", // writes just AFTER a wrap
		"../../roms/techniques/game_states.bin",           // ordinary timer-driven kernel
		"../../roms/litmus/litmus_lastline.bin",           // no timer use at all
	}
	for _, rom := range roms {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM(rom); err != nil {
			t.Skipf("ROM unavailable (%s): %v", rom, err)
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		hits, err := e.WatchTimerDividerHazard(6)
		if err != nil {
			t.Fatalf("%s: %v", rom, err)
		}
		if len(hits) != 0 {
			t.Errorf("%s: %d hazard(s) reported on a ROM whose divider writes do NOT straddle the "+
				"underflow — e.g. %+v. A false positive here would flag every timer-driven kernel.",
				rom, len(hits), hits[0])
		}
	}
}

// The detector must actually be looking at the timer, not returning nil. Drive it
// over a ROM that writes a divider register and confirm the writes are seen — the
// hazard condition is then a judgement about those writes rather than about
// whether any were noticed.
func TestTimerDividerWritesAreObserved(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_timerwrap_nearmiss.bin"); err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	// Widen the condition to "any divider write" by asking for the state directly:
	// step and count stores to the divider registers, which is what the detector
	// filters. If this is zero the ROM changed and the negative test above proves
	// nothing.
	writes := 0
	start := e.Coords().Frame
	for e.Coords().Frame-start < 3 {
		if err := e.StepInstruction(); err != nil {
			break
		}
		lr := e.VCS.CPU.LastResult
		if lr.Defn == nil || lr.Defn.Effect.String() != "Write" {
			continue
		}
		if _, ok := timerDividerRegs[lr.InstructionData&0x0FFF]; ok {
			writes++
		}
	}
	if writes == 0 {
		t.Fatal("no divider write observed at all, so the negative hazard test covers nothing — " +
			"litmus_timerwrap_nearmiss is supposed to write TIM8T and T1024T every frame")
	}
	t.Logf("%d divider writes observed and none flagged as straddling the underflow", writes)
}
