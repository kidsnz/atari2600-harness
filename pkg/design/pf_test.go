package design

import "testing"

// TestAsymRightWindow は woodgrain definitive table の右半窓（repeated）。
func TestAsymRightWindow(t *testing.T) {
	cases := []struct {
		reg        PFReg
		start, end int
	}{
		{PF0, 27, 48},
		{PF1, 37, 53},
		{PF2, 48, 64},
	}
	for _, c := range cases {
		s, e := AsymRightWindow(c.reg)
		if s != c.start || e != c.end {
			t.Errorf("reg %d: got [%d,%d] want [%d,%d]", c.reg, s, e, c.start, c.end)
		}
	}
}

// TestFitsAsymRightWrite は窓の境界（両端OK・外NG）。
func TestFitsAsymRightWrite(t *testing.T) {
	cases := []struct {
		name  string
		reg   PFReg
		cycle int
		want  bool
	}{
		{"PF0_start_ok", PF0, 27, true},
		{"PF0_end_ok", PF0, 48, true},
		{"PF0_before", PF0, 26, false},
		{"PF0_after", PF0, 49, false},
		{"PF2_48_ok", PF2, 48, true},
		{"PF2_47_no", PF2, 47, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FitsAsymRightWrite(c.reg, c.cycle); got != c.want {
				t.Errorf("FitsAsymRightWrite(%d,%d)=%v want %v", c.reg, c.cycle, got, c.want)
			}
		})
	}
}

// TestPFTotalColorClocks は 40 列 × 4clk = 可視 160 と一致すること（playfield.FullWidth 再利用）。
func TestPFTotalColorClocks(t *testing.T) {
	if got := PFTotalColorClocks(); got != 160 {
		t.Errorf("PFTotalColorClocks()=%d want 160", got)
	}
}

// TestScoreModeTwoColor は CTRLPF の score ビット(D1)検出。
func TestScoreModeTwoColor(t *testing.T) {
	if !ScoreModeTwoColor(0x02) || !ScoreModeTwoColor(0x06) {
		t.Error("D1 set should be score mode")
	}
	if ScoreModeTwoColor(0x00) || ScoreModeTwoColor(0x01) || ScoreModeTwoColor(0x04) {
		t.Error("D1 clear should not be score mode")
	}
}

// TestScrollScanlinesConstant は総ライン一定・PAL偶数の鉄則。
func TestScrollScanlinesConstant(t *testing.T) {
	if !ScrollScanlinesConstant([]int{262, 262, 262}, false) {
		t.Error("constant NTSC line counts should pass")
	}
	if ScrollScanlinesConstant([]int{262, 263}, false) {
		t.Error("varying line counts should fail")
	}
	if !ScrollScanlinesConstant([]int{264, 264}, true) {
		t.Error("constant even PAL line counts should pass")
	}
	if ScrollScanlinesConstant([]int{263, 263}, true) {
		t.Error("odd PAL line count should fail")
	}
	if !ScrollScanlinesConstant(nil, true) {
		t.Error("empty should be trivially constant")
	}
}
