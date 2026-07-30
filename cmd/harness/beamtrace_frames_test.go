package main

import (
	"context"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestBeamtraceReturnsEveryFrameItPaidFor pins that `frames` frames traced means
// `frames` frames returned.
//
// It did not. Until 2026-07-30 the handler traced N frames, advanced the emulator by
// all N, and returned only the earliest — measured over the wire: frames=4 starting at
// frame 5 left the machine at frame 9 and handed back frame 5 alone. The three
// discarded frames were unreachable by any other route, because calling a second time
// advances the machine again; there is no way to see frame 5 and frame 6 of the same
// run except in one call.
//
// The check has to be more than a count. Returning the SAME frame N times would satisfy
// a count, so the test reads a register whose value provably changes every frame:
// motion_xclamp repositions P0 by +2px per frame, so the HMOVE nibble staged into HMP0
// on the positioning line steps 96 -> 64 -> 32 as the fine adjustment walks. Three
// distinct values is the witness that these are three different frames.
func TestBeamtraceReturnsEveryFrameItPaidFor(t *testing.T) {
	const (
		posLine = 34   // the scanline SetXPos strobes RESP0 on
		hmp0    = 0x20 // HMP0 — the fine-adjust nibble, restaged every frame
		want    = 3
	)

	mu.Lock()
	e, err := emu.New("NTSC")
	if err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/motion_xclamp.bin"); err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	if err := e.RunFrames(5); err != nil { // past the power-on transient
		mu.Unlock()
		t.Fatal(err)
	}
	current = e
	before := e.Coords().Frame
	mu.Unlock()

	sl := posLine
	_, out, err := handleBeamtrace(context.Background(), nil, BeamtraceIn{Frames: want, Scanline: &sl})
	if err != nil {
		t.Fatal(err)
	}

	if len(out.Frames) != want {
		t.Fatalf("traced %d frames, got %d back — the tool advances the emulator by every one of "+
			"them, so a dropped frame is a frame nothing can ever look at", want, len(out.Frames))
	}

	mu.Lock()
	after := e.Coords().Frame
	mu.Unlock()
	if got := after - before; got != want {
		t.Errorf("emulator advanced %d frames for a %d-frame trace", got, want)
	}

	seenFrame := map[int]bool{}
	values := []int{}
	for _, f := range out.Frames {
		if seenFrame[f.Frame] {
			t.Fatalf("frame %d reported twice", f.Frame)
		}
		seenFrame[f.Frame] = true
		if len(f.Rows) != 1 {
			t.Fatalf("frame %d: asked for one scanline, got %d rows", f.Frame, len(f.Rows))
		}
		if f.Rows[0].Scanline != posLine {
			t.Fatalf("frame %d: reported scanline %d, asked for %d", f.Frame, f.Rows[0].Scanline, posLine)
		}
		found := false
		for _, w := range f.Rows[0].Writes {
			if w.Reg == hmp0 {
				values = append(values, int(w.Value))
				found = true
			}
		}
		if !found {
			t.Fatalf("frame %d, scanline %d: no HMP0 write — the ROM restages the fine adjustment "+
				"every frame, so this row is the wrong one to read", f.Frame, posLine)
		}
	}

	// The anti-vacuity half: N copies of one frame would pass everything above.
	distinct := map[int]bool{}
	for _, v := range values {
		distinct[v] = true
	}
	if len(distinct) != len(values) {
		t.Fatalf("HMP0 across the returned frames = %v — the sprite moves every frame, so equal "+
			"values mean the same frame came back more than once", values)
	}
	t.Logf("frames %v, HMP0 staged %v", keysOf(seenFrame), values)
}

func keysOf(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
