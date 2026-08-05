package design

// 作画 craft（絵作り）の数値化できるルール。判断系の craft（字形の誤読・サムネ可読性など
// 人/画像が要るもの）はコード化せず docs/design-principles.md の「機械判定不能」節に集約する。

// プレビューを信じてはいけない＝実機アスペクトで絵を決める。〔design-principles.md / 採掘 326595〕
// PixelAspectRatio is the width:height of one 2600 pixel on screen — the pixel is
// WIDE, so a square shape needs more scanlines than columns.
//
// ★ THIS VALUE IS KNOWN TO BE TOO LARGE, and is left at 2 deliberately until someone
// decides which display to design for. Measured against the mined sources
// (docs/design-principles.md, threads 190154 / 169128 / 208810 / 172161 / 334673):
// 5:3 = 1.67, 12:7 = 1.71, 20:11 = 1.82. **2.0 is above all of them.**
//
// The spread is not noise. A pixel's aspect is (visible width / visible height)
// divided by the display's own 4:3, and the VISIBLE HEIGHT is the free variable —
// 192 lines of a 262-line frame is a different picture from 210 or 228, and each
// source picked a different one. So no single measurement arbitrates it; choosing a
// value inside 1.67–1.82 is a decision about which overscan to design for, and it
// belongs to whoever is drawing, not to whichever thread was read last.
//
// Anything that consumes this is over-tall by 10–20% today. Raising the number is a
// one-line change; deciding WHICH number is the part that is not the code's to make.
const PixelAspectRatio = 2

// ScanlinesForSquare は、幅 widthPx のスプライト/アイコンを画面上で正方に見せるのに必要な
// スキャンライン数を返す（横長補正＝縦に 2 倍積む）。〔採掘 326595〕
func ScanlinesForSquare(widthPx int) int { return widthPx * PixelAspectRatio }

// WalkFrame は歩行アニメの 2 フレーム 50:50 切替を返す（0 or 1）。フレームカウンタの 1 ビット
// （speedBit）で等間隔・リセット不要に交互させる。移動中だけ counter を進めること。
// speedBit を上げるほど切替が遅くなる（bit3 = 8 フレーム毎）。〔design-principles.md / 採掘 301861〕
func WalkFrame(counter byte, speedBit uint) int { return int((counter >> speedBit) & 1) }

// BackgroundSpec は背景アートを決める 4 軸（このまま背景テンプレの入力パラメータになる）。
// 〔design-principles.md「背景アートは4軸で先に決める」/ 採掘 319884 atari-background-builder〕
type BackgroundSpec struct {
	WidthPx   int  // 幅: 48 (反射の片側) or 96 (全幅相当)
	Colors    int  // 色数: 1 or 2（2 は score-bit or per-band COLUPF）
	Reflect   bool // PF 対称性: true=反射 / false=非対称（非対称は高コスト）
	RowHeight int  // 行高: 1〜16 ライン/行（精細度 vs 負荷のトレードオフ）
}

// Feasible は各軸が文書化された範囲に収まっているかを返す。
func (s BackgroundSpec) Feasible() bool {
	if s.WidthPx != 48 && s.WidthPx != 96 {
		return false
	}
	if s.Colors != 1 && s.Colors != 2 {
		return false
	}
	if s.RowHeight < 1 || s.RowHeight > 16 {
		return false
	}
	return true
}
