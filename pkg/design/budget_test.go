package design

import "testing"

// TestLineBudget は合計と 76cy 収まり判定。
func TestLineBudget(t *testing.T) {
	t.Run("fits", func(t *testing.T) {
		total, fits := LineBudget(20, 30, 26) // =76 ちょうど
		if total != 76 || !fits {
			t.Errorf("got total=%d fits=%v want 76,true", total, fits)
		}
	})
	t.Run("over", func(t *testing.T) {
		total, fits := LineBudget(40, 40) // =80 > 76
		if total != 80 || fits {
			t.Errorf("got total=%d fits=%v want 80,false", total, fits)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if total, fits := LineBudget(); total != 0 || !fits {
			t.Errorf("got total=%d fits=%v want 0,true", total, fits)
		}
	})
}

// TestRemainingCycles は残予算（負=超過）。
func TestRemainingCycles(t *testing.T) {
	if got := RemainingCycles(47); got != 29 { // PFprimer 例: 47cy 使用→残29
		t.Errorf("RemainingCycles(47)=%d want 29", got)
	}
	if got := RemainingCycles(80); got != -4 {
		t.Errorf("RemainingCycles(80)=%d want -4", got)
	}
}
