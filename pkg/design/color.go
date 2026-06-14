// Package design は design-principles.md の設計ルールを実行可能なフィジビリティ判定に
// 「吸収」する。prose の craft/予算ルールを数値判定に落とし、**Claude が asm を書く前の
// 設計判断**の土台にする（凍結した TIA Studio の予算フィジビリティにも流用可）。
//
// 出典: docs/design-principles.md ／ 採掘ノート reference/atariage/*/notes.ja.md。
// ハード定数（1 CPU cycle = 3 color clock = 3 可視px）は docs/resources.md / CLAUDE.md。
package design

// PixelsPerCycle は CPU 1 サイクルが進む可視ピクセル数（1 cycle = 3 color clock = 3px）。
const PixelsPerCycle = 3

// MinColorBandWidthPx は、色替えの書込みに writeCycles サイクルかかる時に、その色が
// 「見える」最小の横幅(px)を返す。横多色は色帯ごとに最低この幅が要る。
// 例: STA zp(3cy)→9px ／ STA abs(4cy)→12px ／ 任意色(LDA#imm+STA≈6cy)→18px。
// 〔design-principles.md「横多色の色帯 最小幅 = ストア命令サイクル × 3px」/ 採掘 170018〕
func MinColorBandWidthPx(writeCycles int) int {
	return writeCycles * PixelsPerCycle
}

// NarrowBand は CheckColorBands が返す「狭すぎて描けない帯」。
type NarrowBand struct {
	Index   int // 帯の番号（左から 0 始まり）
	WidthPx int // 指定された幅
	MinPx   int // 必要な最小幅
}

// CheckColorBands は、左→右に並ぶ横方向の色帯の幅(px)列が、1帯=writeCycles サイクルの
// 色替えコストで実現可能かを判定する。先頭の帯は走査線開始時の既定色＝色替え不要(0)と
// みなし、2番目以降に最小幅を課す。狭すぎる帯の一覧を返す（空 slice なら実現可能）。
func CheckColorBands(widthsPx []int, writeCycles int) []NarrowBand {
	min := MinColorBandWidthPx(writeCycles)
	var bad []NarrowBand
	for i, w := range widthsPx {
		if i == 0 {
			continue // 先頭は既定色＝追加の色替え不要
		}
		if w < min {
			bad = append(bad, NarrowBand{Index: i, WidthPx: w, MinPx: min})
		}
	}
	return bad
}

// --- TIA color register の分解（COLUPx/COLUBK/COLUPF 共通）------------------
// TIA 色レジスタは D7..D4 = hue(0–15) / D3..D1 = luminance(0–7) / D0 無効。
// 色は RGB でなく「(hue, luminance) のレジスタ値」で持つ（生 hex をばら撒かない）。
// 〔design-principles.md「色（最重要）」/ 採掘 symbolic-color-names, 118495〕

// LuminanceLevels は実用上の輝度段数（bit0 無効 → 8 段）。
const LuminanceLevels = 8

// VividMaxLuminance は「鮮やかに見せたい色」を置ける輝度の上限段。これより明るいと
// 彩度が落ちて白っぽくなる（特に明るい青は識別が消える）。〔採掘 132561〕
const VividMaxLuminance = 5

// Hue は色レジスタ値の hue（D7..D4, 0–15）を返す。
func Hue(reg byte) byte { return reg >> 4 }

// Luminance は色レジスタ値の輝度段（D3..D1, 0–7）を返す。D0 は無効なので落とす。
func Luminance(reg byte) byte { return (reg >> 1) & 0x07 }

// HueName は文書化された hue アンカーの色名（hue1=黄/hue4=赤/hue8=青/hue12=緑、
// hue15≈hue1）。アンカー以外は "" を返す（中間色は名前を断定しない）。〔採掘 132561〕
var HueName = map[byte]string{
	1:  "yellow",
	4:  "red",
	8:  "blue",
	12: "green",
	15: "yellow", // hue15 ≈ hue1
}

// WashoutRisk は輝度段 lum がその色を白っぽく(低彩度に)してしまう領域かを返す。
// 鮮やかに見せたい色は VividMaxLuminance 以下に置く。〔採掘 132561〕
func WashoutRisk(lum byte) bool { return lum > VividMaxLuminance }

// GradientSameHue は色帯（風景グラデ等）が全て同一 hue で、輝度のみ変化しているかを返す。
// 色相を混ぜたグラデは不可（奥行きは BG=奥/PF=手前 の輝度段で出す）。〔採掘 160655〕
func GradientSameHue(regs []byte) bool {
	if len(regs) <= 1 {
		return true
	}
	h := Hue(regs[0])
	for _, r := range regs[1:] {
		if Hue(r) != h {
			return false
		}
	}
	return true
}

// InterlaceColorsSafe は2フレーム交互の時間混色 a/b が「低ちらつき」かを返す。
// フリッカ知覚は輝度差に比例するので、両色を同一輝度にして hue だけで分けると激減する。
// 〔採掘 176987 interlaced-multicolor〕
func InterlaceColorsSafe(a, b byte) bool { return Luminance(a) == Luminance(b) }
