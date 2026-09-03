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

// TestHueLuminance は色レジスタ分解（D7..D4 hue / D3..D1 lum / D0 無効）。
func TestHueLuminance(t *testing.T) {
	cases := []struct {
		reg     byte
		wantHue byte
		wantLum byte
	}{
		{0x00, 0, 0},
		{0x1E, 1, 7},  // hue1, lum 段7（黄の最大輝度）
		{0x46, 4, 3},  // hue4(赤), lum 段3
		{0x8A, 8, 5},  // hue8(青), lum 段5
		{0xCF, 12, 7}, // hue12(緑), D0 立っても lum 段7（bit0 無効）
		{0xCE, 12, 7}, // 0xCF と同段（D0 を落とす）
	}
	for _, c := range cases {
		if got := Hue(c.reg); got != c.wantHue {
			t.Errorf("Hue($%02X)=%d want %d", c.reg, got, c.wantHue)
		}
		if got := Luminance(c.reg); got != c.wantLum {
			t.Errorf("Luminance($%02X)=%d want %d", c.reg, got, c.wantLum)
		}
	}
}

// TestHueName は文書化された hue アンカーの色名。
func TestHueName(t *testing.T) {
	for hue, want := range map[byte]string{1: "yellow", 4: "red", 8: "blue", 12: "green", 15: "yellow"} {
		if got := HueName[hue]; got != want {
			t.Errorf("HueName[%d]=%q want %q", hue, got, want)
		}
	}
	if got, ok := HueName[2]; ok {
		t.Errorf("hue2 should be unnamed, got %q", got)
	}
}

// TestWashoutRisk は VividMaxLuminance(5) 超で白っぽくなる判定。
func TestWashoutRisk(t *testing.T) {
	for lum := byte(0); lum <= 5; lum++ {
		if WashoutRisk(lum) {
			t.Errorf("lum %d should be vivid-safe", lum)
		}
	}
	for _, lum := range []byte{6, 7} {
		if !WashoutRisk(lum) {
			t.Errorf("lum %d should be washout risk", lum)
		}
	}
}

// TestGradientSameHue は同一 hue・輝度のみ変化のグラデを可、色相混在を不可とする。
func TestGradientSameHue(t *testing.T) {
	if !GradientSameHue([]byte{0x82, 0x84, 0x88, 0x8C}) { // 全て hue8、輝度だけ上昇
		t.Error("same-hue gradient should be valid")
	}
	if GradientSameHue([]byte{0x82, 0x44}) { // hue8 と hue4 が混在
		t.Error("mixed-hue gradient should be invalid")
	}
	if !GradientSameHue([]byte{0x40}) || !GradientSameHue(nil) {
		t.Error("0/1-element gradient should be trivially valid")
	}
}

// TestSameLuminance — ★2026-09-03 に InterlaceColorsSafe から改名。★関数は【同一輝度か】
// しか言わない。★出典 176987 は同じノートで「完全均一は不可」と反論しており、★「Safe」を
// 名乗れる根拠は無い。★★閾値（識別に足りる最小の輝度差）は5層すべてで0件＝未測定。
func TestSameLuminance(t *testing.T) {
	if !SameLuminance(0x48, 0xC8) { // hue違い・同輝度(段4)
		t.Error("equal-luminance interlace should be safe")
	}
	if SameLuminance(0x42, 0x4C) { // 同hue・輝度差
		t.Error("luminance-differing interlace should be unsafe")
	}
}

// TestPFAlignedBandsAreMultiplesOfFour pins the composition the design doc's colour
// rule states, because that rule used to weld the two numbers with an "=" that is
// false ("multiples of 4 colour clocks (= 12px)").
//
// They are different quantities. WHERE a playfield-aligned boundary may fall is a
// property of the TIA — one PF pixel is 4 colour clocks, 40 columns x 4 = 160 — and is
// pinned at the pixel by emu's TestEveryPlayfieldColumnLandsWhereTheTableSays. HOW WIDE
// the narrowest band can be is a property of the 6502 and the beam: 3 colour clocks per
// CPU cycle, pinned by cmd/calibrate at R^2 = 1.000000. This test pins what falls out of
// putting them together, which is the part an author actually uses:
//
//	a 4-cycle STx.w buys 12 clocks = exactly 3 PF pixels, and lands on the grid
//	a 3-cycle STA zp buys 9 clocks, which is not a multiple of 4 and cannot
//
// So "use the absolute form for a PF-aligned band" is a consequence, not a style note.
func TestPFAlignedBandsAreMultiplesOfFour(t *testing.T) {
	const pfPixelClocks = 4 // one playfield pixel; 40 columns x 4 = 160 visible clocks
	for _, c := range []struct {
		cycles  int
		wantPx  int
		aligned bool
		why     string
	}{
		{3, 9, false, "STA zp — 9 clocks is not a multiple of 4, so a band this narrow cannot start and end on the PF grid"},
		{4, 12, true, "STx.w — 12 clocks is exactly 3 PF pixels"},
		{5, 15, false, "15 clocks straddles the grid the same way 9 does"},
		{6, 18, false, "the ~6cy arbitrary-colour band the doc quotes is NOT PF-aligned either"},
		{8, 24, true, "24 clocks is 6 PF pixels"},
	} {
		got := MinColorBandWidthPx(c.cycles)
		if got != c.wantPx {
			t.Errorf("MinColorBandWidthPx(%d) = %d, want %d", c.cycles, got, c.wantPx)
		}
		if aligned := got%pfPixelClocks == 0; aligned != c.aligned {
			t.Errorf("%dcy -> %d clocks: PF-aligned = %v, want %v — %s",
				c.cycles, got, aligned, c.aligned, c.why)
		}
	}
	// The 160 visible clocks really are 40 PF pixels of 4, which is what makes the
	// modulo above mean anything.
	if ColorClockPerColumn != pfPixelClocks {
		t.Errorf("ColorClockPerColumn = %d, want %d — the whole alignment argument rests on it",
			ColorClockPerColumn, pfPixelClocks)
	}
}
