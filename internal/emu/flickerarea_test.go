package emu

import "testing"

// TestFlickerAreaCountsElementsAndNotColours pins the measure that `diffPixels` could not give.
//
// The negative control is the whole point: a ROM that draws the same picture every frame must read
// **exactly zero**. `cmd/still`'s pixel diff reports 6136 differing pixels on its own clean control
// — a fifth of the picture — because a colour register sweeps every frame in every build. Comparing
// the drawing ELEMENT instead removes that entirely, and the zero below is what proves it.
//
// Measured 2026-09-06: `litmus_pal` (a static picture) 0 px; `zone_multiplex` 126; and the whole
// two-frame comparison costs about **4 ms**, which is why it can sit in a scenario assert.
func TestFlickerAreaCountsElementsAndNotColours(t *testing.T) {
	area := func(rom string) int {
		t.Helper()
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		e.EnableElementCapture()
		if err := e.RunFrames(4); err != nil {
			t.Fatal(err)
		}
		a, err := e.FlickerArea()
		if err != nil {
			t.Fatal(err)
		}
		return a
	}

	// ★The negative control. A static picture must read zero — not "small", zero. If this ever
	// becomes non-zero the measure has started counting something other than element changes, and
	// every threshold set against it is measuring that something instead.
	if got := area("../../roms/litmus/litmus_pal.bin"); got != 0 {
		t.Errorf("a static picture reports %d flickering pixels, want 0 — FlickerArea is counting "+
			"something that is not a change of drawing object, which is exactly the failure that "+
			"makes a raw pixel diff useless here", got)
	}

	// ★★And the positive one, so a measure that always returns zero cannot pass.
	multiplexed := area("../../roms/techniques/zone_multiplex.bin")
	if multiplexed <= 0 {
		t.Errorf("a zone-multiplexed kernel reports %d flickering pixels, want more than zero — "+
			"a measure that answers zero for everything would pass the control above", multiplexed)
	}
	t.Logf("static=0  zone_multiplex=%d px of %d visible", multiplexed, 160*192)
}
