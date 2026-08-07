package design

import "testing"

// TestMultiplexConstants は可動オブジェクト数・multisprite 上限。
func TestMultiplexConstants(t *testing.T) {
	if MovableObjects != 5 {
		t.Errorf("MovableObjects=%d want 5", MovableObjects)
	}
	if DistinctPlayerSprites != 2 {
		t.Errorf("DistinctPlayerSprites=%d want 2", DistinctPlayerSprites)
	}
}

// TestNeedsFlicker は 2体までフリッカ無し・3体以上で要フリッカ。
func TestNeedsFlicker(t *testing.T) {
	cases := []struct {
		n    int
		want bool
	}{
		{1, false},
		{2, false},
		{3, true},
		{5, true},
	}
	for _, c := range cases {
		if got := NeedsFlicker(c.n); got != c.want {
			t.Errorf("NeedsFlicker(%d)=%v want %v", c.n, got, c.want)
		}
	}
}

// TestNeedsEmptyYLane は再配置に空Yレーンが要る条件（3体以上）と再配置コスト。
func TestNeedsEmptyYLane(t *testing.T) {
	if NeedsEmptyYLane(2) {
		t.Error("2 distinct sprites need no empty lane")
	}
	if !NeedsEmptyYLane(3) {
		t.Error("3 distinct sprites need an empty lane for repositioning")
	}
	if RepositionCostScanlines != 1 {
		t.Errorf("RepositionCostScanlines=%d want 1", RepositionCostScanlines)
	}
}

// TestHardwareCollisionUsable pins the consequence flicker has on collision detection,
// because it is the kind of rule that gets rediscovered as a bug. Two objects drawn on
// alternate frames can be overlapping and never share a frame, so the TIA's latches
// never fire — the failure is a silent MISS, not a wrong answer, and a design that
// picks flicker has thereby picked software collision too.
func TestHardwareCollisionUsable(t *testing.T) {
	if !HardwareCollisionUsable(false) {
		t.Error("an object drawn every frame CAN use CXxx — the latches are exactly what they are for")
	}
	if HardwareCollisionUsable(true) {
		t.Error("a flickered object cannot rely on CXxx: colliding objects may never share a frame, " +
			"so the latch never fires and the collision is silently missed")
	}
}
