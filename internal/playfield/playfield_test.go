package playfield

import "testing"

// cellsAt は指定列だけ点灯した width 長のセル列を作る。
func cellsAt(width int, lit ...int) []bool {
	c := make([]bool, width)
	for _, i := range lit {
		c[i] = true
	}
	return c
}

// TestEncodeHalf_LitmusValues は litmus_pf + read_row で実機裏取りした既知値と一致するか検証する。
// PF0=$10→col0(clock0-3) / PF1=$80→col4(clock16-19) / PF2=$01→col12(clock48-51)。
func TestEncodeHalf_LitmusValues(t *testing.T) {
	cases := []struct {
		name          string
		col           int
		pf0, pf1, pf2 byte
	}{
		{"col0_PF0_D4", 0, 0x10, 0x00, 0x00},
		{"col3_PF0_D7", 3, 0x80, 0x00, 0x00},
		{"col4_PF1_D7", 4, 0x00, 0x80, 0x00},
		{"col11_PF1_D0", 11, 0x00, 0x01, 0x00},
		{"col12_PF2_D0", 12, 0x00, 0x00, 0x01},
		{"col19_PF2_D7", 19, 0x00, 0x00, 0x80},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pf0, pf1, pf2 := encodeHalf(cellsAt(HalfWidth, c.col))
			if pf0 != c.pf0 || pf1 != c.pf1 || pf2 != c.pf2 {
				t.Errorf("col %d: got PF0=%02X PF1=%02X PF2=%02X, want %02X %02X %02X",
					c.col, pf0, pf1, pf2, c.pf0, c.pf1, c.pf2)
			}
		})
	}
}

// TestEncodeHalf_AllOn は全20列点灯で PF0=$F0,PF1=$FF,PF2=$FF（左半フル）になるか。
// litmus の all-on poke で PF2 全域(clock48-79)が連続点灯したのと整合。
func TestEncodeHalf_AllOn(t *testing.T) {
	all := make([]bool, HalfWidth)
	for i := range all {
		all[i] = true
	}
	pf0, pf1, pf2 := encodeHalf(all)
	if pf0 != 0xF0 || pf1 != 0xFF || pf2 != 0xFF {
		t.Errorf("all-on: got PF0=%02X PF1=%02X PF2=%02X, want F0 FF FF", pf0, pf1, pf2)
	}
}

// TestEncodeHalf_Empty は全消灯で 0,0,0。
func TestEncodeHalf_Empty(t *testing.T) {
	pf0, pf1, pf2 := encodeHalf(make([]bool, HalfWidth))
	if pf0 != 0 || pf1 != 0 || pf2 != 0 {
		t.Errorf("empty: got %02X %02X %02X, want 00 00 00", pf0, pf1, pf2)
	}
}

// TestEncodeAsymmetric_LeftRightIndependent は左右が独立に符号化されるか。
func TestEncodeAsymmetric_LeftRightIndependent(t *testing.T) {
	// 左 col0 点灯（→PF0A=$10）、右 col20(=右半col0) 点灯（→PF0B=$10）。
	cells := cellsAt(FullWidth, 0, 20)
	r := EncodeAsymmetric(cells)
	if r.PF0A != 0x10 || r.PF1A != 0 || r.PF2A != 0 {
		t.Errorf("left: got PF0A=%02X PF1A=%02X PF2A=%02X, want 10 00 00", r.PF0A, r.PF1A, r.PF2A)
	}
	if r.PF0B != 0x10 || r.PF1B != 0 || r.PF2B != 0 {
		t.Errorf("right: got PF0B=%02X PF1B=%02X PF2B=%02X, want 10 00 00", r.PF0B, r.PF1B, r.PF2B)
	}
}

// TestParseASCIIRow は '.'/'X' のパース。
func TestParseASCIIRow(t *testing.T) {
	cells := ParseASCIIRow("X..X")
	want := []bool{true, false, false, true}
	if len(cells) != 4 {
		t.Fatalf("len=%d want 4", len(cells))
	}
	for i := range want {
		if cells[i] != want[i] {
			t.Errorf("cell %d: got %v want %v", i, cells[i], want[i])
		}
	}
}
