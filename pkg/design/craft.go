package design

// 作画 craft（絵作り）の数値化できるルール。判断系の craft（字形の誤読・サムネ可読性など
// 人/画像が要るもの）はコード化せず docs/design-principles.md の「機械判定不能」節に集約する。

// PixelAspectRatio は 2600 の画面上 1 ピクセルの 横:縦 比（≈ 2:1・横長）。正方ピクセルの
// プレビューを信じてはいけない＝実機アスペクトで絵を決める。〔design-principles.md / 採掘 326595〕
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
