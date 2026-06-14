package design

import "testing"

// TestMinColorBandWidthPx は「色帯最小幅 = 書込みサイクル×3px」（採掘 170018）の基本値。
func TestMinColorBandWidthPx(t *testing.T) {
	cases := []struct {
		name   string
		cycles int
		want   int
	}{
		{"STA_zp_3cy", 3, 9},
		{"STA_abs_4cy", 4, 12}, // design-principles「STx.w = 12px」と一致
		{"generic_color_6cy", 6, 18},
		{"zero", 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MinColorBandWidthPx(c.cycles); got != c.want {
				t.Errorf("MinColorBandWidthPx(%d)=%d want %d", c.cycles, got, c.want)
			}
		})
	}
}

// TestCheckColorBands は先頭帯は無コスト・2番目以降に最小幅(writeCycles=4→12px)を課すこと。
func TestCheckColorBands(t *testing.T) {
	t.Run("all_fit", func(t *testing.T) {
		// 先頭8は免除、12・20 は min(12) 以上。
		if bad := CheckColorBands([]int{8, 12, 20}, 4); len(bad) != 0 {
			t.Errorf("want feasible, got %v", bad)
		}
	})
	t.Run("second_too_narrow", func(t *testing.T) {
		// index1=10 < 12 が1件だけ違反。
		bad := CheckColorBands([]int{20, 10, 12}, 4)
		if len(bad) != 1 || bad[0].Index != 1 || bad[0].WidthPx != 10 || bad[0].MinPx != 12 {
			t.Errorf("want [{1,10,12}], got %v", bad)
		}
	})
	t.Run("first_exempt", func(t *testing.T) {
		// 先頭4は狭くても免除。
		if bad := CheckColorBands([]int{4, 12}, 4); len(bad) != 0 {
			t.Errorf("first band should be exempt, got %v", bad)
		}
	})
}
