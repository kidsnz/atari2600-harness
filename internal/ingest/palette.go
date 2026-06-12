// Package ingest は「スクリーンショット → TIA データ」の逆方向パイプライン。
// 入力 PNG（主対象: Stella の F12 スナップショット）を TIA 実座標 160×H に正規化し、
// ハーネス描画と同一のパレット（Gopher2600 specification.Spec.GetColor）へ量子化、
// playfield / スプライトの素材データへ落とす。判定が原理的に曖昧な要素は
// confidence 付き候補として report する（確定を装わない）。
package ingest

import (
	"image/color"

	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// Quantizer は RGB → TIA 色コード（hue/lum バイト）の逆引き。
// 基準表は Gopher2600 の specification.SpecNTSC.GetColor（capture.go の描画と同一の真実）。
type Quantizer struct {
	codes []uint8      // TIA 色コード（偶数 0x00..0xFE、D0 は無視ビット）
	rgbs  []color.RGBA // codes[i] に対応する RGB
}

// NewNTSCQuantizer は NTSC 全 128 色の逆引き表を作る。
func NewNTSCQuantizer() *Quantizer {
	q := &Quantizer{}
	for v := 0; v <= 0xFE; v += 2 {
		q.codes = append(q.codes, uint8(v))
		q.rgbs = append(q.rgbs, specification.SpecNTSC.GetColor(signal.ColorSignal(v)))
	}
	return q
}

// Nearest は最近色の TIA コードと距離（RGB 二乗距離）を返す。
// 距離 0 = Gopher2600 由来の画像。Stella 由来は微差が出る（report で平均距離を出す）。
func (q *Quantizer) Nearest(c color.RGBA) (code uint8, dist int) {
	best, bestD := 0, 1<<62
	for i, p := range q.rgbs {
		dr := int(c.R) - int(p.R)
		dg := int(c.G) - int(p.G)
		db := int(c.B) - int(p.B)
		d := dr*dr + dg*dg + db*db
		if d < bestD {
			best, bestD = i, d
		}
	}
	return q.codes[best], bestD
}

// RGB は TIA 色コードのパレット RGB を返す（overlay の再描画用）。
func (q *Quantizer) RGB(code uint8) color.RGBA {
	return specification.SpecNTSC.GetColor(signal.ColorSignal(code))
}
