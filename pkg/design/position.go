package design

// 横位置の設計時ヘルパ。スプライトの絵そのもの（GRP/NUSIZ）は pkg/sprite が持つ。
// ここは「目標 X に置くには粗位置ループ何回＋HMOVE 何 px か」という設計時の見積りだけを扱う。
//
// ⚠ 絶対 N（RESPx までの CPU サイクル数）はカーネルの prologue 長に依存する＝ここでは扱わない。
// 最終的な横位置の判定は必ず実機で `read_tia` の HmovedPixel(可視 0–159) を測る（CLAUDE.md 鉄則1）。
// 本ファイルは「粗÷15 → 微 HMOVE」の二段位置決めを設計段階で割り付けるためのもの。
// 〔design-principles.md「横位置 = 2段階」/ Davie S22 / litmus 一致〕

// PositionGranularityPx は横位置の最小粒度（1 CPU サイクル = 3px）。
const PositionGranularityPx = PixelsPerCycle

// CoarseStepPx は粗位置決めの 1 ステップ移動量。÷15 ループ（5cy ループ）の 1 反復 = 15px。
const CoarseStepPx = 15

// HMove の微調整可動域（px）。HMOVE は上位ニブルのみ・2の補数で +7(左)〜−8(右)。
const (
	HMovePxMax = 7  // 左へ最大
	HMovePxMin = -8 // 右へ最大
)

// CoarseIterations は目標 X に最も近い「15px グリッド」へ載せる粗ループ反復回数を返す。
// 残差は HMOVE が吸収する（PositionSplit 参照）。
func CoarseIterations(targetX int) int {
	// 最も近い 15 の倍数へ丸める（四捨五入）。
	return (targetX + CoarseStepPx/2) / CoarseStepPx
}

// PositionSplit は目標 X を「粗ループ反復回数」と「HMOVE 微調整(px)」に分解する。
// hmovePx は必ず HMovePxMin..HMovePxMax に収まる（15 > 7+8 なので 0–159 のどの X も到達可能）。
func PositionSplit(targetX int) (coarseIters, hmovePx int) {
	coarseIters = CoarseIterations(targetX)
	hmovePx = targetX - coarseIters*CoarseStepPx
	return
}

// HMoveReachable は残差 px が 1 回の HMOVE で吸収できる範囲かを返す。
func HMoveReachable(px int) bool { return px >= HMovePxMin && px <= HMovePxMax }
