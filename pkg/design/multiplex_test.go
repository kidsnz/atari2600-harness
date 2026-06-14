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
