package emu

import "testing"

// romCollide is a litmus ROM from this repo that puts objects on screen; any ROM
// that draws overlapping objects will latch collisions.
const romCollide = "../../roms/litmus/litmus_collide_all.bin"

func collisionPairs(c Collisions) map[string]bool {
	return map[string]bool{
		"m0p1": c.M0P1, "m0p0": c.M0P0, "m0pf": c.M0PF, "m0bl": c.M0BL,
		"m1p0": c.M1P0, "m1p1": c.M1P1, "m1pf": c.M1PF, "m1bl": c.M1BL,
		"p0pf": c.P0PF, "p0bl": c.P0BL, "p1pf": c.P1PF, "p1bl": c.P1BL,
		"blpf": c.BLPF, "p0p1": c.P0P1, "m0m1": c.M0M1,
	}
}

// The load-bearing invariant, and the one that validates the copied bit masks:
// whatever is still LATCHED in CXxx at the end of a frame must be a subset of
// what the watcher says actually HAPPENED during that frame. A latched-but-not-
// witnessed collision means a mask bit is wrong or the accumulation is missing
// video cycles.
//
// The converse must NOT hold — the watcher is expected to see collisions the
// latches no longer show, because the ROM clears CXxx mid-frame. That asymmetry
// is the entire reason this exists.
func TestFrameWatchAgreesWithLatches(t *testing.T) {
	for _, rom := range []string{romCollide, romAnim} {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Skipf("ROM unavailable (%s): %v", rom, err)
		}
		if err := e.RunFrames(20); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 30; f++ {
			e.StartFrameWatch()
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
			happened, _ := e.FrameWatch()
			latched, err := e.ReadCollisions()
			if err != nil {
				t.Fatal(err)
			}
			hp, lp := collisionPairs(happened), collisionPairs(latched)
			for name, wasLatched := range lp {
				if wasLatched && !hp[name] {
					t.Fatalf("%s frame %d: %s is latched in CXxx but the watcher never saw it — "+
						"a CollisionEvent mask bit is wrong or accumulation is missing cycles", rom, f, name)
				}
			}
		}
	}
}

// The stack pointer at a frame boundary is nearly always back at $FF, so a
// boundary sample cannot tell you which RAM the stack trampled. Measured on the
// real target, boundary sampling reported a low-water mark of $FF on every frame
// — excluding exactly zero bytes from a gate that is supposed to exclude the
// stack. Watching inside the frame must do better on any ROM that uses JSR.
// romJSR calls subroutines, so its stack demonstrably descends into RAM. Using a
// ROM that never pushes would make this test pass while proving nothing — the
// first draft did exactly that (both numbers came out $FF).
const romJSR = "../../roms/litmus/cb_jsr.bin"

func TestFrameWatchSeesStackBelowFrameBoundary(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(romJSR); err != nil {
		t.Skipf("ROM unavailable (%s): %v", romJSR, err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	deepest := uint8(0xFF)
	boundaryMin := uint8(0xFF)
	for f := 0; f < 20; f++ {
		e.StartFrameWatch()
		if _, err := e.StepFrame(); err != nil {
			t.Fatal(err)
		}
		_, low := e.FrameWatch()
		if low < deepest {
			deepest = low
		}
		if sp := uint8(e.VCS.CPU.SP.Address()); sp < boundaryMin {
			boundaryMin = sp
		}
	}
	if deepest >= boundaryMin {
		t.Errorf("in-frame low-water $%02X is not below the frame-boundary minimum $%02X — "+
			"on a ROM that calls subroutines the watcher must see the stack descend inside the frame",
			deepest, boundaryMin)
	}
	t.Logf("stack low-water: in-frame $%02X vs frame-boundary $%02X", deepest, boundaryMin)
}

// Watching must be observation-only: enabling it may not change a single value
// the emulator produces. Otherwise the measurement changes the thing measured.
func TestFrameWatchDoesNotPerturb(t *testing.T) {
	run := func(watch bool) ([RAMSize]uint8, int64) {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(romAnim); err != nil {
			t.Skipf("ROM unavailable (%s): %v", romAnim, err)
		}
		for f := 0; f < 40; f++ {
			if watch {
				e.StartFrameWatch()
			}
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
		}
		ram, err := e.CurrentRAM()
		if err != nil {
			t.Fatal(err)
		}
		return ram, e.cpuCycles
	}
	ramOff, cycOff := run(false)
	ramOn, cycOn := run(true)
	if ramOff != ramOn {
		t.Error("RAM differs with the frame watch enabled — observation is perturbing the run")
	}
	if cycOff != cycOn {
		t.Errorf("cycle count differs with the frame watch enabled: %d vs %d", cycOff, cycOn)
	}
}
