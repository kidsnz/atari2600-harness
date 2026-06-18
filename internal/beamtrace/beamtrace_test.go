package beamtrace

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestTimelineSpans locks the pure span logic with synthetic writes (no emulator):
// writes are ordered by beam clock, each governs pixels until the next write to the
// SAME register, an HBLANK write clamps to pixel 0, and a write superseded during
// HBLANK governs an empty span.
func TestTimelineSpans(t *testing.T) {
	ws := []Write{
		{Frame: 1, Scanline: 10, Clock: 80, Reg: 0x09, Name: "COLUBK", Kind: KindColor, Value: 0x42, HasValue: true},
		{Frame: 1, Scanline: 10, Clock: 20, Reg: 0x09, Name: "COLUBK", Kind: KindColor, Value: 0x0E, HasValue: true},
		{Frame: 1, Scanline: 10, Clock: 50, Reg: 0x1B, Name: "GRP0", Kind: KindGraphics, Value: 0xFF, HasValue: true},
		{Frame: 1, Scanline: 10, Clock: -30, Reg: 0x0E, Name: "PF1", Kind: KindGraphics, Value: 0x01, HasValue: true},
		{Frame: 1, Scanline: 10, Clock: -10, Reg: 0x0E, Name: "PF1", Kind: KindGraphics, Value: 0x02, HasValue: true},
		{Frame: 1, Scanline: 11, Clock: 5, Reg: 0x09, Name: "COLUBK", Kind: KindColor, Value: 0x99, HasValue: true}, // other line
		{Frame: 2, Scanline: 10, Clock: 5, Reg: 0x09, Name: "COLUBK", Kind: KindColor, Value: 0x88, HasValue: true}, // other frame
	}
	row := Timeline(ws, 1, 10)
	want := []struct {
		clk, from, to int
		reg           uint16
	}{
		{-30, 0, 0, 0x0E},   // PF1 superseded by the -10 PF1 (both HBLANK) => empty span
		{-10, 0, 160, 0x0E}, // PF1 governs from pixel 0 (HBLANK clamp), no later PF1
		{20, 20, 80, 0x09},  // COLUBK until the next COLUBK at 80
		{50, 50, 160, 0x1B}, // GRP0, no later GRP0
		{80, 80, 160, 0x09}, // COLUBK to end of line
	}
	if len(row.Writes) != len(want) {
		t.Fatalf("got %d writes, want %d: %+v", len(row.Writes), len(want), row.Writes)
	}
	for i, w := range row.Writes {
		if w.Clock != want[i].clk || w.VisFrom != want[i].from || w.VisTo != want[i].to || w.Reg != want[i].reg {
			t.Errorf("write %d: got clk=%d span[%d,%d) reg=$%02X, want clk=%d span[%d,%d) reg=$%02X",
				i, w.Clock, w.VisFrom, w.VisTo, w.Reg, want[i].clk, want[i].from, want[i].to, want[i].reg)
		}
	}
}

func mustBuildLoad(t *testing.T, asm string) *emu.Emu {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "out.bin")
	if out, _, _, err := build.AssembleWithListing(asm, bin); err != nil {
		t.Fatalf("assemble %s:\n%s", asm, out)
	}
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestTraceGRP0Marker is the integration test: the fixture writes GRP0=$A5 exactly
// once per frame at a visible clock. The trace must surface that single write with
// the right value/kind, on exactly one scanline, and deterministically.
func TestTraceGRP0Marker(t *testing.T) {
	const asm = "../../roms/litmus/beamtrace_grp0.asm"
	e := mustBuildLoad(t, asm)
	if err := e.RunFrames(1); err != nil { // warm up past reset
		t.Fatal(err)
	}
	ws, err := Trace(e, 1)
	if err != nil {
		t.Fatal(err)
	}

	var grp0 []Write
	for _, w := range ws {
		if w.Reg == 0x1B {
			grp0 = append(grp0, w)
		}
	}
	if len(grp0) != 1 {
		t.Fatalf("expected exactly one GRP0 write in the frame, got %d: %+v", len(grp0), grp0)
	}
	m := grp0[0]
	if !m.HasValue || m.Value != 0xA5 {
		t.Errorf("marker value: got HasValue=%v value=$%02X, want $A5", m.HasValue, m.Value)
	}
	if m.Kind != KindGraphics || m.Name != "GRP0" {
		t.Errorf("marker classification: got %s/%s, want GRP0/graphics", m.Name, m.Kind)
	}
	if m.Clock < 0 {
		t.Errorf("marker landed in HBLANK (clk=%d); fixture intends a visible clock", m.Clock)
	}

	// Localized: the marker is on its scanline and absent from the neighbours.
	if got := len(Timeline(ws, m.Frame, m.Scanline).Writes); got == 0 {
		t.Errorf("timeline for the marker's scanline %d is empty", m.Scanline)
	}
	for _, sl := range []int{m.Scanline - 1, m.Scanline + 1} {
		for _, w := range Timeline(ws, m.Frame, sl).Writes {
			if w.Reg == 0x1B {
				t.Errorf("GRP0 marker leaked onto neighbour scanline %d", sl)
			}
		}
	}

	// Deterministic: a fresh run yields an identical write stream.
	e2 := mustBuildLoad(t, asm)
	if err := e2.RunFrames(1); err != nil {
		t.Fatal(err)
	}
	ws2, err := Trace(e2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws2) != len(ws) {
		t.Fatalf("non-deterministic: %d writes vs %d", len(ws2), len(ws))
	}
	for i := range ws {
		if ws[i] != ws2[i] {
			t.Fatalf("non-deterministic at write %d: %+v vs %+v", i, ws[i], ws2[i])
		}
	}
}
