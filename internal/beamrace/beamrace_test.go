package beamrace

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestCheckEvalPure locks the verdict logic: only late writes (Clock > ObjectX) for
// the named object within the line range are flagged; in-time and out-of-range
// writes pass.
func TestCheckEvalPure(t *testing.T) {
	ev := []Event{
		{Object: "P0", Scanline: 50, Clock: -40, ObjectX: 20, BeforeBeam: true},  // HBLANK, in time
		{Object: "P0", Scanline: 51, Clock: 80, ObjectX: 20, BeforeBeam: false},  // late, in range -> flag
		{Object: "P0", Scanline: 99, Clock: 80, ObjectX: 20, BeforeBeam: false},  // late, OUT of range
		{Object: "P1", Scanline: 51, Clock: 80, ObjectX: 20, BeforeBeam: false},  // wrong object
		{Object: "P0", Scanline: 52, Clock: 20, ObjectX: 20, BeforeBeam: true},   // exactly at X, in time
	}
	v := Check{Object: "P0", LineFrom: 50, LineTo: 60}.Eval(ev)
	if len(v) != 1 || v[0].Scanline != 51 {
		t.Fatalf("expected exactly one violation on scanline 51, got %+v", v)
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
	if err := e.RunFrames(1); err != nil { // warm up past reset
		t.Fatal(err)
	}
	return e
}

// band returns the inclusive scanline span over which `obj` had pixel-data writes.
func band(ev []Event, obj string) (lo, hi, n int) {
	lo = 1 << 30
	for _, e := range ev {
		if e.Object != obj {
			continue
		}
		n++
		if e.Scanline < lo {
			lo = e.Scanline
		}
		if e.Scanline > hi {
			hi = e.Scanline
		}
	}
	return
}

// TestBeamraceCleanPasses — the clean fixture updates P0 in HBLANK every band line:
// every event is in time and a check over the band finds no violation.
func TestBeamraceCleanPasses(t *testing.T) {
	e := mustBuildLoad(t, "../../roms/litmus/beamrace_clean.asm")
	ev, err := Trace(e, 1)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi, n := band(ev, "P0")
	if n == 0 {
		t.Fatal("no P0 pixel-data writes observed")
	}
	for _, x := range ev {
		if x.Object == "P0" && !x.BeforeBeam {
			t.Errorf("clean fixture: P0 write at scanline %d clk %d (X=%d) is NOT before beam", x.Scanline, x.Clock, x.ObjectX)
		}
	}
	if v := (Check{Object: "P0", LineFrom: lo, LineTo: hi}).Eval(ev); len(v) != 0 {
		t.Fatalf("clean fixture must pass no_beam_race over lines %d..%d, got %+v", lo, hi, v)
	}
}

// TestBeamraceLateFails — the late fixture writes P0 graphics deep in the visible
// line, right of P0: every band event is late and a check over the band fails.
func TestBeamraceLateFails(t *testing.T) {
	e := mustBuildLoad(t, "../../roms/litmus/beamrace_late.asm")
	ev, err := Trace(e, 1)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi, n := band(ev, "P0")
	if n == 0 {
		t.Fatal("no P0 pixel-data writes observed")
	}
	for _, x := range ev {
		if x.Object == "P0" && x.BeforeBeam {
			t.Errorf("late fixture: P0 write at scanline %d clk %d (X=%d) unexpectedly before beam", x.Scanline, x.Clock, x.ObjectX)
		}
	}
	v := (Check{Object: "P0", LineFrom: lo, LineTo: hi}).Eval(ev)
	if len(v) == 0 {
		t.Fatalf("late fixture must FAIL no_beam_race over lines %d..%d", lo, hi)
	}
	t.Logf("late fixture: %d violations over lines %d..%d (e.g. %s)", len(v), lo, hi, v[0].Detail)
}
