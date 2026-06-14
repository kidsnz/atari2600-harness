package design

import "testing"

// TestScanlinesForSquare は横長補正（縦2倍）で正方表示になること。
func TestScanlinesForSquare(t *testing.T) {
	if got := ScanlinesForSquare(8); got != 16 {
		t.Errorf("ScanlinesForSquare(8)=%d want 16", got)
	}
	if got := ScanlinesForSquare(0); got != 0 {
		t.Errorf("ScanlinesForSquare(0)=%d want 0", got)
	}
}

// TestWalkFrame は選んだビットで 50:50 に交互すること。
func TestWalkFrame(t *testing.T) {
	// speedBit=0 → 毎フレーム交互（0,1,0,1,...）
	for c := 0; c < 8; c++ {
		if got, want := WalkFrame(byte(c), 0), c&1; got != want {
			t.Errorf("WalkFrame(%d,0)=%d want %d", c, got, want)
		}
	}
	// speedBit=3 → 8 フレーム毎に切替（0..7=0, 8..15=1）
	if WalkFrame(0, 3) != 0 || WalkFrame(7, 3) != 0 || WalkFrame(8, 3) != 1 || WalkFrame(15, 3) != 1 {
		t.Error("speedBit=3 should toggle every 8 frames")
	}
}

// TestBackgroundSpecFeasible は背景4軸の範囲判定。
func TestBackgroundSpecFeasible(t *testing.T) {
	if !(BackgroundSpec{WidthPx: 48, Colors: 1, Reflect: true, RowHeight: 8}).Feasible() {
		t.Error("nominal spec should be feasible")
	}
	bad := []BackgroundSpec{
		{WidthPx: 60, Colors: 1, RowHeight: 8},  // 幅が 48/96 でない
		{WidthPx: 96, Colors: 3, RowHeight: 8},  // 色数が 1/2 でない
		{WidthPx: 96, Colors: 2, RowHeight: 0},  // 行高 < 1
		{WidthPx: 96, Colors: 2, RowHeight: 17}, // 行高 > 16
	}
	for i, s := range bad {
		if s.Feasible() {
			t.Errorf("case %d should be infeasible: %+v", i, s)
		}
	}
}
