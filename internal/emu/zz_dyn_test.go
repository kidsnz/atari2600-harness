package emu

// A frame that is not 262 scanlines is a defect, so this counts them and FAILS.
//
// It used to count them and print them. `bad` could reach 28 and the test still
// passed, because nothing ever compared it to anything — the only `t.Fatal` in the
// function was the error check on `LoadROM`, which is how it slipped past
// `check_tests.py`'s "every test function can fail" rule: that gate sees a `t.Fatal`
// and is satisfied, without asking whether the assertion is about the THING UNDER
// TEST or about the plumbing that set it up.
//
// `dyn_multisprite` is the flagship of `docs/techniques/dynamic-multisprite.md`, so
// "its frames are the right length" is exactly the claim this file should carry.

import "testing"

func TestDynMultispriteHoldsA262LineFrame(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/techniques/dyn_multisprite.bin"); err != nil {
		t.Fatal(err)
	}
	// Frames 0 and 1 are the boot transient: the ROM has not reached a steady
	// frame structure yet, and grading them would fail on every ROM in the repo.
	const settle = 2
	const frames = 30
	bad := 0
	var first string
	for f := 0; f < frames; f++ {
		lines, err := e.StepFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", f, err)
		}
		if f < settle {
			continue
		}
		if lines != 262 {
			bad++
			if first == "" {
				first = "frame " + itoa(f) + " ran " + itoa(lines) + " lines"
			}
		}
	}
	// PREMISE — grading nothing looks identical to grading agreement.
	if graded := frames - settle; graded < 20 {
		t.Fatalf("only %d frames graded; this test measures nothing at that size", graded)
	}
	if bad > 0 {
		t.Errorf("%d of %d frames are not 262 scanlines (first: %s) — a frame that runs "+
			"long adds a scanline, which is a roll on hardware", bad, frames-settle, first)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
