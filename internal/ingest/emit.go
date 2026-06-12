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
	Playfield        []PFBand     `json:"playfield,omitempty"`     // M2: PF バンド（上→下）
	PlayfieldASM     string       `json:"playfield_asm,omitempty"` // M2: そのまま貼れる DASM データ片
	Sprites          []Sprite     `json:"sprites,omitempty"`       // M3: スプライト候補
	SpritesASM       string       `json:"sprites_asm,omitempty"`   // M3: GRP テーブル等の DASM 片
}

// Analyze は正規化結果からフルレポートを作る（統計＋playfield。M3 でスプライトが加わる）。
func Analyze(n *Normalized, q *Quantizer) *Report {
	r := BuildReport(n, q)
	bands, residual := ExtractPlayfield(n)
	r.Playfield = bands
	r.PlayfieldASM = DASMPlayfield(bands)
	r.Sprites = ExtractSprites(n, residual)
	r.SpritesASM = DASMSprites(r.Sprites)
	return r
}

// DASMSprites はスプライト候補を DASM に貼れる GRP テーブル＋色テーブルで出力する。
func DASMSprites(sprites []Sprite) string {
	if len(sprites) == 0 {
		return ""
	}
	out := "; sprites (extracted by cmd/ingest — window anchored at leftmost lit pixel)\n"
	for i, s := range sprites {
		out += fmt.Sprintf("; sprite %d: kind=%s x=%d y=%d w=%d h=%d", i, s.Kind, s.X, s.Y, s.W, s.H)
		if s.Copies > 1 {
			out += fmt.Sprintf(" copies=%d spacing=%d (NUSIZ)", s.Copies, s.Spacing)
		}
		if s.Confidence < 1.0 {
			out += fmt.Sprintf(" conf=%.2f", s.Confidence)
		}
		out += "\n"
		if len(s.GRP) > 0 {
			out += fmt.Sprintf("Spr%dGfx:\n", i)
			for r, b := range s.GRP {
				out += fmt.Sprintf("        byte %%%08b ; color $%02X\n", uint8(b), uint8(s.Colors[r]))
			}
		}
	}
	return out
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

// DASMPlayfield は PF バンド列を DASM にそのまま貼れる byte テーブル＋コメントで出力する。
func DASMPlayfield(bands []PFBand) string {
	if len(bands) == 0 {
		return ""
	}
	out := "; playfield bands (extracted by cmd/ingest — rows are image-relative)\n"
	for i, b := range bands {
		out += fmt.Sprintf("; band %d: rows %d-%d (%d lines) mode=%s colorL=$%02X colorR=$%02X",
			i, b.Top, b.Top+b.Height-1, b.Height, b.Mode, b.ColorLeft, b.ColorRight)
		if b.ScoreMode {
			out += " SCORE-MODE?"
		}
		if b.Confidence < 1.0 {
			out += fmt.Sprintf(" conf=%.2f", b.Confidence)
		}
		out += "\n"
		if b.Mode == "asymmetric" {
			out += fmt.Sprintf("        byte $%02X,$%02X,$%02X, $%02X,$%02X,$%02X ; PF0A,PF1A,PF2A, PF0B,PF1B,PF2B\n",
				b.PF0, b.PF1, b.PF2, b.PF0B, b.PF1B, b.PF2B)
		} else {
			out += fmt.Sprintf("        byte $%02X,$%02X,$%02X ; PF0,PF1,PF2 (CTRLPF D0=%d)\n",
				b.PF0, b.PF1, b.PF2, map[string]int{"repeat": 0, "reflect": 1}[b.Mode])
		}
	}
	return out
}

// Overlay は TIA 実座標グリッド付きのオーバーレイ画像を返す（annotate を再利用）。
// visibleTop=0（画像相対座標）。ユーザーはこの画像の座標をそのまま指せる。
func Overlay(n *Normalized, scale int) *image.RGBA {
	if scale < 1 {
		scale = 3
	}
	return annotate.Render(n.TIA, 0, scale, nil)
}
