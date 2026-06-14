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
