package emu

import "testing"

// TestCoarseAdjustSweepIsWhatTheDocSays turns docs/litmus-results.md's coarse-adjust
// table into a machine-checked fact. It was a table of numbers with no test behind
// it, and one row had drifted from the hardware.
//
// MEASURED 2026-07-30, and the FIRST thing that had to be fixed was the protocol.
// The document prescribes "poke $80 (DELAY) -> step_frame -> read_tia" starting from
// a freshly loaded ROM. That does not work: the poke lands before the kernel has
// initialised, so the value is overwritten and every DELAY reads back the same
// position (measured: 107 for DELAY 0, 1, 2, 3 and 6 alike). Running one frame
// first, then poking, is stable and gives the same answer whether one, two or three
// frames follow.
//
// Under that protocol the table is right except for DELAY=0, which the document
// records as 72 with the note "minimal delay; boundary artifact of a deep-HBLANK
// strobe". Measured, DELAY=0 gives 3 — the same left clamp as DELAY=1 — and 72 is
// what DELAY=6 produces. There is no protocol in which 0 yields 72.
func TestCoarseAdjustSweepIsWhatTheDocSays(t *testing.T) {
	// DELAY -> P0 HmovedPixel. 0 and 1 both clamp; 2 is the HBLANK->visible
	// boundary; from 3 on it is linear at +15 (one SBC#1+BCS iteration = 5 CPU
	// cycles = 15 colour clocks).
	want := map[int]int{0: 3, 1: 3, 2: 12, 3: 27, 4: 42, 5: 57, 6: 72, 7: 87, 8: 102, 9: 117, 10: 132}

	pos := func(delay int) int {
		t.Helper()
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_pos.bin"); err != nil {
			t.Fatal(err)
		}
		e.StepFrame() // let the kernel initialise, or the poke is overwritten
		if err := e.Poke(0x80, byte(delay)); err != nil {
			t.Fatal(err)
		}
		e.StepFrame()
		x, ok := e.ObjectX("P0")
		if !ok {
			t.Fatal("no P0")
		}
		return x
	}

	for delay := 0; delay <= 10; delay++ {
		if got := pos(delay); got != want[delay] {
			t.Errorf("DELAY=%d: P0 at %d, doc table says %d", delay, got, want[delay])
		}
	}

	// The linear step is the number the positioning code depends on. Checking it
	// separately means a table edited to match a broken build still fails here.
	for delay := 4; delay <= 10; delay++ {
		if d := want[delay] - want[delay-1]; d != 15 {
			t.Fatalf("DELAY %d->%d is %d px, want 15 (5 CPU cycles x 3)", delay-1, delay, d)
		}
	}

	// And the protocol itself: poking before the first frame must be visibly
	// useless, or a future reader will "simplify" the test back into a measurement
	// that reads the same number for every input.
	unstable := map[int]bool{}
	for _, delay := range []int{0, 3, 6} {
		e, _ := New("NTSC")
		if err := e.LoadROM("../../roms/litmus/litmus_pos.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.Poke(0x80, byte(delay)); err != nil {
			t.Fatal(err)
		}
		e.StepFrame()
		x, _ := e.ObjectX("P0")
		unstable[x] = true
	}
	if len(unstable) != 1 {
		t.Errorf("poking before the first frame now distinguishes DELAY values (%v) — the protocol "+
			"note in docs/litmus-results.md needs re-measuring", unstable)
	}
}
