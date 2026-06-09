// Package annotate は捕捉フレーム（160×可視高さ）に TIA 実座標のグリッド・軸ラベル・
// スプライト位置マーカーを重ね、人間可読サイズに拡大した画像を作る。
//
// これは Claude 専用の補助ではなく**ユーザー↔Claude の通信回線**。ユーザーが画像を見て
// 「P0 を clock 80 へ」と指示でき、その clock 値が register 操作へ直結するよう、グリッドは
// TIA 実座標（横 clock 0..159 / 縦 可視 scanline）で校正する。
package annotate

import (
	"fmt"
	"image"
	"image/color"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/basicfont"
	xdraw "golang.org/x/image/draw"
)

// Marker は 1 オブジェクトの横位置マーカー。Clock は可視 0..159（HmovedPixel）。負なら描かない。
type Marker struct {
	Label string
	Clock int
	Col   color.RGBA
}

const (
	leftMargin = 30 // y 軸ラベル用
	topMargin  = 16 // x 軸ラベル用
	rightPad   = 10
	botPad     = 16 // マーカーラベル用
)

var (
	gridMinor = color.RGBA{255, 255, 255, 30}
	gridMajor = color.RGBA{255, 255, 255, 70}
	labelCol  = color.RGBA{205, 215, 225, 255}
)

// Render は注釈付き画像を返す。scale は整数倍率（×3〜4 推奨）。visibleTop は
// 縦ラベルを絶対 scanline で出すための起点（クロップ y=0 の絶対 scanline）。
func Render(frame *image.RGBA, visibleTop, scale int, markers []Marker) *image.RGBA {
	fw := frame.Bounds().Dx() // 160
	fh := frame.Bounds().Dy()
	up := upscale(frame, scale)

	W := leftMargin + fw*scale + rightPad
	H := topMargin + fh*scale + botPad

	dc := gg.NewContext(W, H)
	dc.SetRGB(0.11, 0.11, 0.13)
	dc.Clear()
	dc.DrawImage(up, leftMargin, topMargin)
	dc.SetFontFace(basicfont.Face7x13)

	cx := func(clock int) float64 { return float64(leftMargin + clock*scale) }
	cy := func(row int) float64 { return float64(topMargin + row*scale) }
	top := float64(topMargin)
	bottom := float64(topMargin + fh*scale)
	left := float64(leftMargin)
	right := float64(leftMargin + fw*scale)

	// 縦グリッド（clock）。10 刻み・40 ごとに強調＋ラベル。右端 159 もラベル。
	for c := 0; c <= fw; c += 10 {
		major := c%40 == 0
		setLine(dc, c == 0 || major)
		dc.DrawLine(cx(c), top, cx(c), bottom)
		dc.Stroke()
		if c == 0 || major {
			dc.SetColor(labelCol)
			dc.DrawStringAnchored(fmt.Sprintf("%d", c), cx(c), top-4, 0.5, 1)
		}
	}
	dc.SetColor(labelCol)
	dc.DrawStringAnchored("159", right, top-4, 0.5, 1)

	// 横グリッド（可視 scanline 行）。20 刻み・40 ごとに強調＋絶対 scanline ラベル。
	for r := 0; r <= fh; r += 20 {
		major := r%40 == 0
		setLine(dc, major)
		dc.DrawLine(left, cy(r), right, cy(r))
		dc.Stroke()
		if major {
			dc.SetColor(labelCol)
			dc.DrawStringAnchored(fmt.Sprintf("%d", visibleTop+r), left-3, cy(r), 1, 0.5)
		}
	}

	// スプライトマーカー（縦線＋数値ラベル）
	for _, m := range markers {
		if m.Clock < 0 || m.Clock > fw {
			continue
		}
		dc.SetColor(m.Col)
		dc.SetLineWidth(1.5)
		dc.DrawLine(cx(m.Clock), top, cx(m.Clock), bottom)
		dc.Stroke()
		dc.DrawStringAnchored(fmt.Sprintf("%s:%d", m.Label, m.Clock), cx(m.Clock), bottom+8, 0.5, 0.5)
	}

	return dc.Image().(*image.RGBA)
}

func setLine(dc *gg.Context, major bool) {
	dc.SetLineWidth(1)
	if major {
		dc.SetColor(gridMajor)
	} else {
		dc.SetColor(gridMinor)
	}
}

// upscale は nearest-neighbor で整数倍拡大（ピクセルを鮮明に保つ）。
func upscale(src *image.RGBA, scale int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}
