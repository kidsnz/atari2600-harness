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
	if MaxMultiSprite != 5 {
		t.Errorf("MaxMultiSprite=%d want 5", MaxMultiSprite)
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

// TestFitsMultiSprite は multisprite 上限(5)の境界。
func TestFitsMultiSprite(t *testing.T) {
	if !FitsMultiSprite(5) || !FitsMultiSprite(1) {
		t.Error("<=5 sprites should fit")
	}
	if FitsMultiSprite(6) {
		t.Error(">5 sprites should not fit")
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
