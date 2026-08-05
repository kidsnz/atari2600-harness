package design

// 作画 craft（絵作り）の数値化できるルール。判断系の craft（字形の誤読・サムネ可読性など
// 人/画像が要るもの）はコード化せず docs/design-principles.md の「機械判定不能」節に集約する。

// プレビューを信じてはいけない＝実機アスペクトで絵を決める。〔design-principles.md / 採掘 326595〕
// The pixel-aspect pair that used to live here — `PixelAspectRatio = 2` and
// `ScanlinesForSquare(w) = w * 2` — was DELETED on 2026-08-04. It had no caller
// anywhere: not in this repository, not in the umbrella `sandbox/` tree that holds
// the 54 authored PONG sources and the Pizza Boy reproduction. The only references
// were its own definition and a test asserting that definition.
//
// It was also wrong. Measured against the mined sources, a 2600 pixel's width:height
// lands somewhere in 1.67–1.82 (5:3 from a 4:3 screen at 200 visible lines, verified
// on a real TV by 24 PF px reading square against 160 lines; 12:7 from Stella's
// 320x210; 20:11 from NTSC's own 10:11 pixel ratio doubled). **2 is above all of
// them**, so anything that had adopted it would have drawn 10–20% over-tall.
//
// Dead code carrying a wrong constant is worse than no code: the next reader would
// have trusted it. The measurement itself is worth keeping and lives in
// docs/design-principles.md, where the three derivations are set out with their
// sources. The author works in Photoshop at a 1:2 grid and has decided not to chase
// the remaining ~16%, which on a 2600 sprite is one dot either way.

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
