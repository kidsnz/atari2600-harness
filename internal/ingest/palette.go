// Package ingest は「スクリーンショット → TIA データ」の逆方向パイプライン。
// 入力 PNG（主対象: Stella の F12 スナップショット）を TIA 実座標 160×H に正規化し、
// ハーネス描画と同一のパレット（Gopher2600 specification.Spec.GetColor）へ量子化、
// playfield / スプライトの素材データへ落とす。判定が原理的に曖昧な要素は
// confidence 付き候補として report する（確定を装わない）。
package ingest

import (
	"image/color"
	"sort"

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

// Canonical は「同一 RGB を持つ最小の TIA コード」を返す。パレットには同色衝突がある
// （例: 本パレットでは $0C と $0E が同一 RGB）。量子化結果は常にこの正準値で報告される。
func (q *Quantizer) Canonical(code uint8) uint8 {
	c, _ := q.Nearest(specification.SpecNTSC.GetColor(signal.ColorSignal(code)))
	return c
}

// RGB は TIA 色コードのパレット RGB を返す（overlay の再描画用）。
func (q *Quantizer) RGB(code uint8) color.RGBA {
	return specification.SpecNTSC.GetColor(signal.ColorSignal(code))
}

// Aliases returns the groups of TIA colour codes this quantiser cannot tell apart: every entry is
// two or more codes that share one RGB value. The reverse lookup necessarily returns just one of
// them, and until now nothing said which information was being thrown away.
//
// Measured on the Stella NTSC table (2026-09-04): **128 codes, 91 distinct colours, 28 alias groups,
// 37 codes that no image can ever ask for.** Four codes ($26 $28 $F6 $F8) are one colour; the
// adjacent greys $08/$0A and $0C/$0E are pairs.
//
// Why this is worth a function rather than a comment. Artwork for this project is drawn in
// Photoshop and ingested here, so an artist can pick two swatches that look different, get two
// different TIA codes out of a table, and see one colour on the screen — with no warning anywhere in
// the chain. `Nearest` cannot warn, because by the time it runs the two swatches have already
// collapsed. A caller that wants to warn has to ask this first.
//
// **The count belongs to this table, and measurement says so sharply.** Run the same two functions
// over the engine's own palette (`NewNTSCQuantizer`) and the answer is **126-127 distinct colours with
// 1-2 alias groups** — the exact figure depends on the host, because the palette is generated in
// floating point and its last bit rounds differently on arm64 (two colliding pairs) than on x86-64
// (one). The gap to Stella's 91 is not host-dependent, and that gap is the finding — against Stella's 91 and 28. The collapse is therefore overwhelmingly a property
// of the *measured Stella table*, not of the TIA's colour generation, and the person who measured
// that palette said as much in 2001: the colours in Stella "seem to be idealized a bit". Three
// layers exist — the colour code, an emulator's RGB, and a real television — and this measures the
// middle one of one emulator. Do not read "91 colours" as a hardware limit.
func (q *Quantizer) Aliases() [][]uint8 {
	byRGB := map[color.RGBA][]uint8{}
	for i, p := range q.rgbs {
		byRGB[p] = append(byRGB[p], q.codes[i])
	}
	var out [][]uint8
	for _, codes := range byRGB {
		if len(codes) > 1 {
			out = append(out, codes)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// DistinctColours returns how many different colours this quantiser can actually produce, which is
// smaller than the number of codes whenever Aliases is non-empty.
func (q *Quantizer) DistinctColours() int {
	seen := map[color.RGBA]bool{}
	for _, p := range q.rgbs {
		seen[p] = true
	}
	return len(seen)
}
