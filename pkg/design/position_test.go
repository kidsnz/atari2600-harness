package design

import "testing"

// TestPositionSplit は粗÷15＋微HMOVEの分解が、残差を必ずHMOVE可動域に収めること。
func TestPositionSplit(t *testing.T) {
	for x := 0; x <= 159; x++ {
		coarse, hmove := PositionSplit(x)
		if !HMoveReachable(hmove) {
			t.Fatalf("X=%d: residual %dpx outside HMOVE range [%d,%d]", x, hmove, HMovePxMin, HMovePxMax)
		}
		if got := coarse*CoarseStepPx + hmove; got != x {
			t.Fatalf("X=%d: coarse(%d)*15 + hmove(%d) = %d, want %d", x, coarse, hmove, got, x)
		}
	}
}

// TestCoarseIterations は代表点の粗反復回数（最近傍15グリッド）。
func TestCoarseIterations(t *testing.T) {
	cases := []struct{ x, want int }{
		{0, 0}, {7, 0}, {8, 1}, {15, 1}, {30, 2}, {68, 5}, {159, 11},
	}
	for _, c := range cases {
		if got := CoarseIterations(c.x); got != c.want {
			t.Errorf("CoarseIterations(%d)=%d want %d", c.x, got, c.want)
		}
	}
}
