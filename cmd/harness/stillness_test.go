package main

import (
	"context"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func loadFor(t *testing.T, rom string, warmup int) *emu.Emu {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(warmup); err != nil {
		t.Fatal(err)
	}
	current = e
	return e
}

func spriteyOf(t *testing.T, object string, frames int) SpriteYOut {
	t.Helper()
	_, out, err := handleSpriteY(context.Background(), nil, SpriteYIn{Object: object, Frames: frames})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestStillnessDoesNotCallALiveROMDead is the test this feature exists in its current
// shape because of.
//
// The first version classified "the object did not move AND no RAM byte changed" as
// STUCK — the program is not running. Measured before it shipped: litmus_pos and smoke
// run a full kernel, draw their sprite on every frame and never write a byte of RAM
// after init, so a perfectly live ROM got a confident diagnosis of dead. That is the
// exact failure the whole stillness idea is meant to protect against, produced by the
// protection itself.
//
// So the note may say the reading is a CONSTANT — which is true and useful — and may not
// conclude anything about whether the program runs. Only an injected input can answer
// that, which is why it stays in set_input.
func TestStillnessDoesNotCallALiveROMDead(t *testing.T) {
	for _, rom := range []string{"../../roms/litmus/litmus_pos.bin", "../../roms/litmus/smoke.bin"} {
		loadFor(t, rom, 6)
		out := spriteyOf(t, "P0", 20)
		if out.Stillness == nil {
			t.Fatalf("%s: a 20-frame read got no stillness", rom)
		}
		// The fixture's whole value is that it is alive and writes no RAM. If that
		// stops being true the test proves nothing.
		if out.Stillness.RAMChanged {
			t.Fatalf("%s no longer holds still in RAM — repin this fixture, it is the one that "+
				"proves 'no RAM writes' is not 'not running'", rom)
		}
		for _, banned := range []string{"STUCK", "not running", "is dead", "jam"} {
			if strings.Contains(out.Stillness.Note, banned) {
				t.Errorf("%s: a live ROM was diagnosed with %q — no non-invasive reading can "+
					"establish that, and this one draws its sprite every frame.\nnote: %s",
					rom, banned, out.Stillness.Note)
			}
		}
		if !strings.Contains(out.Stillness.Note, "CONSTANT") {
			t.Errorf("%s: a motionless reading must still be flagged as a constant: %q",
				rom, out.Stillness.Note)
		}
	}
}

// TestStillnessStaysQuietWhenSomethingActuallyMoves pins the other side: a warning that
// fires on everything is not a warning. Both axes are covered, because reporting one
// axis's travel into the other is the mistake that would make this vacuous.
func TestStillnessStaysQuietWhenSomethingActuallyMoves(t *testing.T) {
	loadFor(t, "../../roms/litmus/motion_xclamp.bin", 6)
	x := spriteyOf(t, "P0", 20)
	if x.Stillness == nil || x.Stillness.XSpan <= 0 {
		t.Fatalf("motion_xclamp P0 travels +2px/frame; stillness reported %+v", x.Stillness)
	}
	if x.Stillness.Note != "" {
		t.Errorf("an object that moved %dpx was warned about: %s", x.Stillness.XSpan, x.Stillness.Note)
	}

	loadFor(t, "../../roms/litmus/motion_glide.bin", 6)
	y := spriteyOf(t, "BL", 20)
	if y.Stillness == nil || y.Stillness.YSpan <= 0 {
		t.Fatalf("motion_glide BL descends 1px/frame; stillness reported %+v", y.Stillness)
	}
	if y.Stillness.Note != "" {
		t.Errorf("an object that descended %dpx was warned about: %s", y.Stillness.YSpan, y.Stillness.Note)
	}
}

// TestStillnessFlagsTheClampWindowAndSkipsASingleRead pins the case it was built for —
// motion_xclamp's plateau is the Outlaw shape, a stable plausible number that means the
// situation has stopped changing — and pins that a single read gets no stillness at all,
// because a window it did not measure is not something it may report on.
func TestStillnessFlagsTheClampWindowAndSkipsASingleRead(t *testing.T) {
	loadFor(t, "../../roms/litmus/motion_xclamp.bin", 50) // past the climb, inside the clamp
	held := spriteyOf(t, "P0", 15)
	if held.Stillness == nil {
		t.Fatal("no stillness on a 15-frame read")
	}
	if held.Stillness.XSpan != 0 || held.Stillness.Note == "" {
		t.Errorf("the clamp plateau must come back flagged as a constant: %+v", held.Stillness)
	}
	if !held.Stillness.RAMChanged {
		t.Error("motion_xclamp counts frames in RAM, so RAM must change even while X is pinned — " +
			"this is the case that distinguishes a running program from an unchanging one")
	}

	if single := spriteyOf(t, "P0", 1); single.Stillness != nil {
		t.Errorf("frames=1 has no window to measure, so it must report no stillness: %+v",
			single.Stillness)
	}
}
