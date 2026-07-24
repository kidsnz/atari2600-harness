package emu

import "testing"

// TestObjectYExtentTracksBall locks the numeric vertical-position readout (spritey):
// on motion_glide (a BL that glides straight down 1px/frame over a uniform field),
// ObjectYExtent(BL) must (a) find the ball by its own colour and (b) see its y_top
// advance as the ball descends — otherwise the readout is vacuous. This is the
// object-Y primitive read_tia (X only) and read_motion (rendered-top, unreliable
// for a small object against a border) do not provide. (motion_glide.bin is
// assembled by CI before `go test`.)
func TestObjectYExtentTracksBall(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/motion_glide.bin"); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := e.RunFrames(8); err != nil { // settle the first-frame transient
		t.Fatal(err)
	}
	const BL = 4
	top1, bot1, h1, ok1 := e.ObjectYExtent(BL)
	if !ok1 {
		t.Fatal("ObjectYExtent(BL) not present on motion_glide (the ball should be drawn)")
	}
	if h1 <= 0 || bot1 < top1 || top1 < 0 || bot1 > 260 {
		t.Fatalf("implausible extent: top=%d bot=%d h=%d", top1, bot1, h1)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	top2, _, _, ok2 := e.ObjectYExtent(BL)
	if !ok2 {
		t.Fatal("ObjectYExtent(BL) lost the ball after 10 more frames")
	}
	// The glide moves the ball straight down, so its top must have descended.
	if top2 <= top1 {
		t.Fatalf("VACUOUS: BL y_top did not descend over 10 glide frames (%d -> %d)", top1, top2)
	}
}
