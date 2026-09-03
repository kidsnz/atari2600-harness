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

// TestScrollBackgroundFitsRAM guards the gap the distillation found: the three-layer
// scrolling structure is prescribed in design-principles.md and nothing checked whether
// it fits the 2600's 128 bytes. Source 〔200972:14〕 puts the practical ceiling at a
// "120-byte-class" world on internal RAM, with SuperChip/CBS needed above that.
func TestScrollBackgroundFitsRAM(t *testing.T) {
	cases := []struct {
		name                                  string
		board, buffer, delta, stack int
		want                                  bool
	}{
		{"a 120-byte-class world with a shallow stack", 60, 40, 12, 6, true},
		{"exactly the 128 bytes", 60, 40, 22, 6, true},
		{"one byte over", 60, 40, 23, 6, false},
		{"the stack is what tips it", 60, 40, 22, 8, false},
		{"a large board alone", 128, 0, 0, 0, true},
		{"a large board plus any buffer", 128, 1, 0, 0, false},
		{"negative is refused rather than wrapped", -1, 0, 0, 0, false},
	}
	for _, c := range cases {
		if got := ScrollBackgroundFitsRAM(c.board, c.buffer, c.delta, c.stack); got != c.want {
			t.Errorf("%s: board %d + buffer %d + delta %d + stack %d = %d of %d bytes; got %v want %v",
				c.name, c.board, c.buffer, c.delta, c.stack,
				c.board+c.buffer+c.delta+c.stack, RAM2600, got, c.want)
		}
	}
	// The constant must match what the emulator models, or the budget is fiction.
	if RAM2600 != 128 {
		t.Errorf("RAM2600 is %d; the 2600 has 128 bytes at $80-$FF", RAM2600)
	}
}
