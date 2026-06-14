package design

// LineCycles は 1 走査線で使える CPU サイクルの天井（228 color clock / 3 = 76cy）。
// 実行時の超過検出は MCP の assert_line_budget が行う。本関数は「作る前」の静的見積り。
// 〔design-principles.md「76cy/line が天井」/ CLAUDE.md 定数〕
const LineCycles = 76

// LineBudget は per-feature（PF描画/スプライト/色替え…）のサイクルコスト合計と、
// それが 76cy に収まるかを返す。設計時に「この配置は1ラインに収まるか」を即判定する。
func LineBudget(costs ...int) (total int, fits bool) {
	for _, c := range costs {
		total += c
	}
	return total, total <= LineCycles
}

// RemainingCycles は使用済みサイクルを引いた 1 ラインの残予算（負なら超過）。
func RemainingCycles(used int) int {
	return LineCycles - used
}
