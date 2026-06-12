package ingest

import (
	"fmt"
	"image"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/annotate"
)

// ColorCount は画面内の色の出現統計（report 用）。
type ColorCount struct {
	Code  uint8   `json:"tia_color"` // COLUxx に書く値（hue/lum、D0 無視）
	Hex   string  `json:"hex"`       // パレット RGB（参考表示用）
	Share float64 `json:"share"`     // 画面内の占有率
}

// Report は M1 時点の解析レポート（M2/M3 で playfield/sprites フィールドが増える）。
type Report struct {
	SourceW, SourceH int      `json:"-"`
	Width            int      `json:"width"`  // 160
	Height           int      `json:"height"` // 可視走査線数（画像相対）
	ScaleX           int      `json:"scale_x"`
	ScaleY           int      `json:"scale_y"`
	AvgPaletteDist   float64  `json:"avg_palette_dist"` // 0=Gopher2600 由来。Stella は数十程度の微差
	Warnings         []string `json:"warnings,omitempty"`
	Colors           []ColorCount `json:"colors"` // 出現順（多い順）
}

// BuildReport は正規化結果から統計レポートを作る。
func BuildReport(n *Normalized, q *Quantizer) *Report {
	r := &Report{
		Width: tiaWidth, Height: n.Height,
		ScaleX: n.ScaleX, ScaleY: n.ScaleY,
		AvgPaletteDist: n.AvgDist,
		Warnings:       n.Warnings,
	}
	counts := map[uint8]int{}
	for y := 0; y < n.Height; y++ {
		for x := 0; x < tiaWidth; x++ {
			counts[n.Codes[y][x]]++
		}
	}
	total := float64(n.Height * tiaWidth)
	for code, cnt := range counts {
		c := q.RGB(code)
		r.Colors = append(r.Colors, ColorCount{
			Code:  code,
			Hex:   fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B),
			Share: float64(cnt) / total,
		})
	}
	sort.Slice(r.Colors, func(i, j int) bool { return r.Colors[i].Share > r.Colors[j].Share })
	return r
}

// Overlay は TIA 実座標グリッド付きのオーバーレイ画像を返す（annotate を再利用）。
// visibleTop=0（画像相対座標）。ユーザーはこの画像の座標をそのまま指せる。
func Overlay(n *Normalized, scale int) *image.RGBA {
	if scale < 1 {
		scale = 3
	}
	return annotate.Render(n.TIA, 0, scale, nil)
}
