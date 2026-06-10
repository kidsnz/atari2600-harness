package calibrate

import (
	"math"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestFitWrapAndSaturation は折返し（160）と左端飽和の混じった点列でも傾きを正しく復元することを確認する。
func TestFitWrapAndSaturation(t *testing.T) {
	var pts []Point
	for d := 2; d <= 11; d++ {
		pts = append(pts, Point{Delay: d, X: 15*d - 18}) // 12,27,...,147
	}
	pts = append(pts, Point{Delay: 12, X: (15*12 - 18) % 160}) // 162 -> 2（折返し）
	pts = append(pts, Point{Delay: 13, X: 3})                  // 飽和
	pts = append(pts, Point{Delay: 14, X: 3})                  // 飽和

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
	pts := []Point{{2, 50}, {3, 50}, {4, 50}} // 動かない
	if _, err := Fit(pts, 5); err == nil {
		t.Errorf("expected error for no movement")
	}
}

// TestSweepFitLitmus は実機検証済み litmus_pos を掃引フィットし、横位置の傾きが 3 px/CPU-cycle
// （権威値, docs/litmus-results.md）になることを実 ROM で再現する（B-4 の本旨＝再現可能化）。
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
