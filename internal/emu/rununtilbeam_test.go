package emu

import "testing"

// TestRunUntilBeamReachesPositionsNobodyStopsOn pins that "run until the beam reaches
// (scanline, clock)" works for a clock the CPU is never observed at.
//
// It did not, and it failed silently. The original implementation halted only on
// c.Scanline == scanline && c.Clock == clock. Observations happen at instruction
// boundaries, and the CPU advances 3 colour clocks per cycle, so only ONE PHASE IN
// THREE is ever observable at all; a WSYNC kernel narrows it much further. Measured on
// motion_xclamp: a visible scanline is observed at 7 clocks, every one of them inside
// HBLANK, so the entire visible region 0..159 was unreachable. Asking for a position in
// the picture ran to max_frames and returned halted=false, with no error to say the
// request had been impossible.
//
// The test doubles as its own negative control: it requires the halt to land at a clock
// DIFFERENT from the one asked for, which an equality-matching implementation cannot
// produce.
func TestRunUntilBeamReachesPositionsNobodyStopsOn(t *testing.T) {
	e := loadXClampFixture(t)

	const line, want = 100, 80
	halted, err := e.RunUntilBeam(3, line, want)
	if err != nil {
		t.Fatal(err)
	}
	if !halted {
		t.Fatalf("never halted at (%d,%d) — a position inside the picture is exactly what a caller "+
			"asks for, and it is the region a WSYNC kernel is never observed in", line, want)
	}
	c := e.Coords()
	if c.Scanline != line {
		t.Fatalf("halted on scanline %d, wanted %d", c.Scanline, line)
	}
	if c.Clock < want {
		t.Fatalf("halted at clock %d, BEFORE the requested %d", c.Clock, want)
	}
	if c.Clock == want {
		t.Fatalf("halted at exactly %d: this ROM is never observed there, so either the fixture "+
			"changed or the check has stopped being a control", want)
	}
	if c.Clock-want > 8 { // one instruction is at most 7 cycles; 8 clocks is under 3 of them
		t.Errorf("halted %d clocks past the target — 'reaches' should be the FIRST boundary at or "+
			"past it, not any later one", c.Clock-want)
	}
	t.Logf("asked for (%d,%d), halted at (%d,%d)", line, want, c.Scanline, c.Clock)
}

// TestRunUntilBeamRejectsClocksOutsideTheRaster pins that an impossible request is an
// error. The tool's tag used to advertise "0-227"; 160..227 does not exist in this
// coordinate system (HBLANK is negative here), so those callers got halted=false and no
// way to tell "not yet" from "never".
func TestRunUntilBeamRejectsClocksOutsideTheRaster(t *testing.T) {
	e := loadXClampFixture(t)
	for _, clock := range []int{160, 200, 227, -69} {
		if halted, err := e.RunUntilBeam(2, 100, clock); err == nil {
			t.Errorf("until_clock=%d accepted (halted=%v) — out of range must be an error, not a "+
				"silent run to the frame limit", clock, halted)
		}
	}
	if _, err := e.RunUntilBeam(2, -1, 0); err == nil {
		t.Error("a negative scanline was accepted")
	}
}

// TestRunUntilBeamWaitsForTheNextFrameWhenThePositionHasPassed pins the arming rule: a
// position already behind the beam in the current frame must not match immediately —
// "reaches" means the next time the beam gets there, not "we are already past it".
func TestRunUntilBeamWaitsForTheNextFrameWhenThePositionHasPassed(t *testing.T) {
	e := loadXClampFixture(t)
	if halted, err := e.RunUntilBeam(3, 150, 40); err != nil || !halted {
		t.Fatalf("setup halt failed: halted=%v err=%v", halted, err)
	}
	frameAt150 := e.Coords().Frame

	// Now ask for a position EARLIER in the raster than where we are standing.
	halted, err := e.RunUntilBeam(3, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !halted {
		t.Fatal("never came back round to scanline 20")
	}
	c := e.Coords()
	if c.Frame == frameAt150 {
		t.Fatalf("halted in the SAME frame (%d) at scanline %d — an already-passed position must "+
			"wait for the next frame, otherwise the call returns instantly wherever it stands",
			c.Frame, c.Scanline)
	}
	if c.Scanline != 20 {
		t.Errorf("halted on scanline %d, wanted 20", c.Scanline)
	}
	t.Logf("stood at frame %d scanline 150, next (20,0) came at frame %d scanline %d",
		frameAt150, c.Frame, c.Scanline)
}

func loadXClampFixture(t *testing.T) *Emu {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/motion_xclamp.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil { // past the power-on transient
		t.Fatal(err)
	}
	return e
}
