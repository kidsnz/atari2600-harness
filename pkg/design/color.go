// Package design は design-principles.md の設計ルールを実行可能なフィジビリティ判定に
// 「吸収」する。prose の craft/予算ルールを数値判定に落とし、TIA Studio M4（予算
// フィジビリティ）と Claude の設計判断の土台にする。
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
