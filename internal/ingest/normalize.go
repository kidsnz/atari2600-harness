package ingest

import (
	"fmt"
	"image"
	"image/color"
)

// Normalized は TIA 実座標へ正規化された画面。
type Normalized struct {
	TIA      *image.RGBA // 160 × Height（1 ピクセル = 1 カラークロック × 1 走査線）
	Height   int         // 可視走査線数（画像相対。絶対 scanline は画像からは原理的に不明）
	ScaleX   int         // 検出した横スケール（例: Stella 320px → 2）
	ScaleY   int
	Warnings []string

	Codes   [][]uint8 // [y][x] TIA 色コード（量子化後）
	AvgDist float64   // 量子化の平均 RGB 二乗距離（0=Gopher2600 由来、Stella は微差）
}

const tiaWidth = 160

// plausibleH は可視走査線数としてもっともらしい範囲（NTSC 可視 ~192 + 上下マージン）。
const plausibleHMin, plausibleHMax = 120, 300

// Normalize は入力画像のスケールを検出し、TIA 160×H 格子（セル=多数決色）へ落とし、
// パレット量子化まで行う。非整数スケール等の劣化入力は警告を積んで続行する（拒否しない）。
func Normalize(src image.Image, q *Quantizer) (*Normalized, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < tiaWidth {
		return nil, fmt.Errorf("image width %d < %d (TIA width)", w, tiaWidth)
	}
	n := &Normalized{}

	// 横スケール: 160 の整数倍が原則（Stella F12 は 320）。割り切れなければ最近整数＋警告。
	n.ScaleX = w / tiaWidth
	if w%tiaWidth != 0 {
		n.Warnings = append(n.Warnings,
			fmt.Sprintf("width %d is not a multiple of 160 — assuming scale %d (re-shoot with Stella F12 for exactness)", w, n.ScaleX))
	}

	// 縦スケール: H/sy がもっともらしい範囲に入る sy のうち、セル内一様性が最大のもの。
	bestSy, bestScore := 1, -1.0
	for sy := 1; sy <= 8; sy++ {
		hh := h / sy
		if hh < plausibleHMin || hh > plausibleHMax {
			continue
		}
		score := uniformity(src, n.ScaleX, sy)
		if score > bestScore {
			bestSy, bestScore = sy, score
		}
	}
	n.ScaleY = bestSy
	if h%bestSy != 0 {
		n.Warnings = append(n.Warnings,
			fmt.Sprintf("height %d is not a multiple of scaleY %d — bottom remainder ignored", h, bestSy))
	}
	if bestScore >= 0 && bestScore < 0.95 {
		n.Warnings = append(n.Warnings,
			fmt.Sprintf("cell uniformity %.2f < 0.95 — image may be filtered/resized (TV effects? non-integer scale?)", bestScore))
	}
	n.Height = h / n.ScaleY

	// セル多数決で 160×H に縮約。
	n.TIA = image.NewRGBA(image.Rect(0, 0, tiaWidth, n.Height))
	for y := 0; y < n.Height; y++ {
		for x := 0; x < tiaWidth; x++ {
			n.TIA.SetRGBA(x, y, cellMode(src, x*n.ScaleX, y*n.ScaleY, n.ScaleX, n.ScaleY))
		}
	}

	// パレット量子化。
	n.Codes = make([][]uint8, n.Height)
	total := 0
	for y := 0; y < n.Height; y++ {
		n.Codes[y] = make([]uint8, tiaWidth)
		for x := 0; x < tiaWidth; x++ {
			code, d := q.Nearest(n.TIA.RGBAAt(x, y))
			n.Codes[y][x] = code
			total += d
			// 量子化済みのパレット RGB に貼り直す（以後の処理は全て TIA コード基準）
			n.TIA.SetRGBA(x, y, q.RGB(code))
		}
	}
	n.AvgDist = float64(total) / float64(n.Height*tiaWidth)
	return n, nil
}

// uniformity は (sx,sy) セル分割したとき「セル内の全ピクセルが同色」な割合（サンプリング）。
func uniformity(src image.Image, sx, sy int) float64 {
	if sx < 1 || sy < 1 {
		return -1
	}
	b := src.Bounds()
	cells, uniform := 0, 0
	for cy := 0; cy < b.Dy()/sy; cy += 3 { // 1/3 サンプリングで十分
		for cx := 0; cx < tiaWidth; cx += 3 {
			first := rgbaAt(src, b.Min.X+cx*sx, b.Min.Y+cy*sy)
			same := true
			for dy := 0; dy < sy && same; dy++ {
				for dx := 0; dx < sx && same; dx++ {
					if rgbaAt(src, b.Min.X+cx*sx+dx, b.Min.Y+cy*sy+dy) != first {
						same = false
					}
				}
			}
			cells++
			if same {
				uniform++
			}
		}
	}
	if cells == 0 {
		return -1
	}
	return float64(uniform) / float64(cells)
}

// cellMode はセル内の最頻色（多数決）。
func cellMode(src image.Image, x0, y0, sx, sy int) color.RGBA {
	b := src.Bounds()
	counts := map[color.RGBA]int{}
	var best color.RGBA
	bestN := 0
	for dy := 0; dy < sy; dy++ {
		for dx := 0; dx < sx; dx++ {
			c := rgbaAt(src, b.Min.X+x0+dx, b.Min.Y+y0+dy)
			counts[c]++
			if counts[c] > bestN {
				best, bestN = c, counts[c]
			}
		}
	}
	return best
}

func rgbaAt(src image.Image, x, y int) color.RGBA {
	r, g, b, _ := src.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
}
