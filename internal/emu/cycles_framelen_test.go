package emu

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// TestCyclesLitmusHasNoStableFrameLength pins what litmus_cycles actually does,
// because two documents described it as a fixed-length frame and it is not one.
//
// `docs/mcp-tools.md` and `docs/verified-coverage.md` both carried
// "1 frame = 263 lines x 76 cy = TotalCycles 19988". litmus_cycles emits no VSYNC
// (that is the point — the CPU never stops, so the cycles*3 == color-clocks
// invariant holds at every instruction boundary), which means the FRAME length is
// not decided by the ROM at all. Measured over consecutive frames after warm-up:
//
//	263 -> 290 -> 319 -> 350 -> 350 -> 350 ...
//
// It grows until it hits specification.AbsoluteMaxScanlines (350), the engine's hard
// cap, and rests there. 263 is a transient value from the first frames only, and the
// steady state is 350 lines / ~26600 cycles (measured 26598-26601, so not even
// exactly lines*76 every frame).
//
// The invariant test above is unaffected — it re-bases at every frame boundary — so
// the figure was decorative. It is pinned here rather than deleted so that "263"
// cannot quietly come back as if it were a property of the ROM.
func TestCyclesLitmusHasNoStableFrameLength(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_cycles.bin"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5000; i++ {
		if _, err := e.stepInstr(); err != nil {
			t.Fatal(err)
		}
	}

	var lines []int
	for f := 0; f < 10; f++ {
		start := e.Coords().Frame
		maxLine := 0
		for e.Coords().Frame == start {
			if _, err := e.stepInstr(); err != nil {
				t.Fatal(err)
			}
			if l := e.Coords().Scanline; l > maxLine {
				maxLine = l
			}
		}
		lines = append(lines, maxLine+1)
	}
	t.Logf("frame lengths: %v", lines)

	same := true
	for _, n := range lines {
		if n != lines[0] {
			same = false
			break
		}
	}
	if same {
		t.Errorf("every frame was %d lines — this ROM emits no VSYNC, so a constant frame length "+
			"means the measurement is not seeing what it thinks it is", lines[0])
	}
	for _, n := range lines[len(lines)-4:] {
		if n != specification.AbsoluteMaxScanlines {
			t.Errorf("steady state is %d lines, want the engine cap %d — the frame length of a "+
				"VSYNC-less ROM is set by the engine, not the ROM", n, specification.AbsoluteMaxScanlines)
		}
	}
	for _, n := range lines {
		if n == 263 && lines[len(lines)-1] == 263 {
			t.Error("263 is a transient early-frame value, not this ROM's frame length")
		}
	}
}
