package ingest

import (
	"fmt"
	"image"
	"sort"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/basicfont"

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
	RowBG            []int        `json:"-"`                       // M5: 行毎の背景色（逆描画用）
	Fidelity         float64      `json:"fidelity"`                // M5: 再構成一致率（0..1）
	Groups           []Group      `json:"groups,omitempty"`        // M6: 同行・同色の並び（スコア/ゲージ等）
}

// Group は意味的な並び（スコア桁・ライフゲージなど）。Members は Sprites のインデックス。
type Group struct {
	Label   string `json:"label"` // "row-group"
	Members []int  `json:"members"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
}

// Analyze は正規化結果からフルレポートを作る（統計＋playfield。M3 でスプライトが加わる）。
func Analyze(n *Normalized, q *Quantizer) *Report {
	r := BuildReport(n, q)
	bands, residual, rowBG := ExtractPlayfield(n)
	r.Playfield = bands
	r.PlayfieldASM = DASMPlayfield(bands)
	r.Sprites = ExtractSprites(n, residual)
	r.SpritesASM = DASMSprites(r.Sprites)
	for _, bg := range rowBG {
		r.RowBG = append(r.RowBG, int(bg))
	}
	r.Groups = groupRows(r.Sprites)
	r.Fidelity = Fidelity(r, n)
	return r
}

// groupRows は「Y 範囲が重なり・色が交差・X 間隙 ≤8」の連鎖を row-group に束ねる。
func groupRows(sprites []Sprite) []Group {
	n := len(sprites)
	used := make([]bool, n)
	var groups []Group
	for i := 0; i < n; i++ {
		if used[i] || len(sprites[i].GRP) == 0 {
			continue
		}
		members := []int{i}
		used[i] = true
		for {
			grew := false
			for j := 0; j < n; j++ {
				if used[j] || len(sprites[j].GRP) == 0 {
					continue
				}
				for _, m := range members {
					a, b := sprites[m], sprites[j]
					yOverlap := a.Y < b.Y+b.H && b.Y < a.Y+a.H
					gapX := b.X - (a.X + a.W)
					if gapX < 0 {
						gapX = a.X - (b.X + b.W)
					}
					if yOverlap && gapX >= 0 && gapX <= 8 && colorsShared(a.Colors, b.Colors) {
						members = append(members, j)
						used[j] = true
						grew = true
						break
					}
				}
			}
			if !grew {
				break
			}
		}
		if len(members) >= 2 {
			g := Group{Label: "row-group", Members: members}
			g.X, g.Y = sprites[members[0]].X, sprites[members[0]].Y
			maxX, maxY := 0, 0
			for _, m := range members {
				s := sprites[m]
				if s.X < g.X {
					g.X = s.X
				}
				if s.Y < g.Y {
					g.Y = s.Y
				}
				if s.X+s.W > maxX {
					maxX = s.X + s.W
				}
				if s.Y+s.H > maxY {
					maxY = s.Y + s.H
				}
			}
			g.W, g.H = maxX-g.X, maxY-g.Y
			groups = append(groups, g)
		}
	}
	return groups
}

func colorsShared(a, b []int) bool {
	set := map[int]bool{}
	for _, c := range a {
		set[c] = true
	}
	for _, c := range b {
		if set[c] {
			return true
		}
	}
	return false
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
// rep があればスプライトの bbox＋番号を重ね描きする（answer-check 用）。
func Overlay(n *Normalized, scale int) *image.RGBA { return OverlayReport(n, nil, scale) }

func OverlayReport(n *Normalized, rep *Report, scale int) *image.RGBA {
	if scale < 1 {
		scale = 3
	}
	img := annotate.Render(n.TIA, 0, scale, nil)
	if rep == nil {
		return img
	}
	dc := gg.NewContextForRGBA(img)
	dc.SetFontFace(basicfont.Face7x13)
	for i, s := range rep.Sprites {
		x0 := float64(annotate.LeftMargin + s.X*scale)
		y0 := float64(annotate.TopMargin + s.Y*scale)
		w := float64(s.W * scale)
		if s.Copies > 1 {
			w = float64((s.Spacing*(s.Copies-1) + s.W) * scale)
		}
		dc.SetRGBA(1, 0.35, 0.35, 0.9)
		dc.SetLineWidth(1.5)
		dc.DrawRectangle(x0-1, y0-1, w+2, float64(s.H*scale)+2)
		dc.Stroke()
		dc.DrawStringAnchored(fmt.Sprintf("%d", i), x0+3, y0-6, 0, 0.5)
	}
	return img
}
