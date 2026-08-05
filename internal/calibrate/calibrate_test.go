package calibrate

import (
	"math"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestFitWrapAndSaturation checks that the slope is recovered correctly even from a point series mixing wraparound (160) and left-edge saturation.
func TestFitWrapAndSaturation(t *testing.T) {
	var pts []Point
	for d := 2; d <= 11; d++ {
		pts = append(pts, Point{Delay: d, X: 15*d - 18}) // 12,27,...,147
	}
	pts = append(pts, Point{Delay: 12, X: (15*12 - 18) % 160}) // 162 -> 2 (wraparound)
	pts = append(pts, Point{Delay: 13, X: 3})                  // saturated
	pts = append(pts, Point{Delay: 14, X: 3})                  // saturated

	r, err := Fit(pts, 5)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.SlopePerUnit-15) > 1e-6 {
		t.Errorf("SlopePerUnit = %v, want 15", r.SlopePerUnit)
	}
	if math.Abs(r.SlopePerCycle-3) > 1e-6 {
		t.Errorf("SlopePerCycle = %v, want 3", r.SlopePerCycle)
	}
	if math.Abs(r.R2-1) > 1e-9 {
		t.Errorf("R2 = %v, want 1 (linear run is collinear)", r.R2)
	}
}

func TestFitDegenerate(t *testing.T) {
	pts := []Point{{2, 50}, {3, 50}, {4, 50}} // no movement
	if _, err := Fit(pts, 5); err == nil {
		t.Errorf("expected error for no movement")
	}
}

// TestSweepFitLitmus sweep-fits the hardware-verified litmus_pos and reproduces on a real ROM
// that the horizontal-position slope is 3 px/CPU-cycle (authoritative value,
// docs/litmus-results.md) — the whole point of B-4: making the calibration reproducible.
func TestSweepFitLitmus(t *testing.T) {
	t.Chdir("../..")
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("roms/litmus/litmus_pos.bin"); err != nil {
		t.Fatal(err)
	}
	pts, err := Sweep(e, 0x80, 2, 14)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Fit(pts, 5)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.SlopePerCycle-3) > 1e-6 {
		t.Fatalf("SlopePerCycle = %v, want 3 (litmus authority)", r.SlopePerCycle)
	}
	if math.Abs(r.R2-1) > 1e-6 {
		t.Fatalf("R2 = %v, want ~1", r.R2)
	}
}
