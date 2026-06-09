package emu

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/jetsetilly/gopher2600/hardware/television/frameinfo"
	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// capture は television.PixelRenderer を実装し、最新フレームを image.RGBA に取り込む。
// 実装は Gopher2600 の thumbnailer/image.go の確立パターンを踏襲（signal→RGB 変換、
// frameInfo.Crop() で可視域クロップ）。
//
// クロップ画像の座標規約（litmus / Crop() 実装で裏取り済み）:
//   - x = 可視 clock 0..159（スプライトの HmovedPixel と同一）
//   - y = 絶対 scanline − VisibleTop（可視先頭が y=0）
type capture struct {
	img       *image.RGBA
	cropImg   *image.RGBA
	frameInfo frameinfo.Current
}

func newCapture() *capture {
	c := &capture{}
	c.img = image.NewRGBA(image.Rect(0, 0, specification.ClksScanline, specification.AbsoluteMaxScanlines))
	c.clear()
	// 初期クロップを NTSC で確定（force=true）。以後フレーム仕様が安定したら更新。
	c.resize(frameinfo.NewCurrent(specification.SpecNTSC), true)
	return c
}

// clear は全ピクセルを黒・alpha=255 に。SetPixels は RGB のみ書くため alpha を先に立てる。
func (c *capture) clear() {
	for i := 0; i < len(c.img.Pix); i += 4 {
		c.img.Pix[i], c.img.Pix[i+1], c.img.Pix[i+2], c.img.Pix[i+3] = 0, 0, 0, 255
	}
}

func (c *capture) resize(fi frameinfo.Current, force bool) {
	if c.frameInfo.IsDifferent(fi) && (force || fi.Stable) {
		c.cropImg = c.img.SubImage(fi.Crop()).(*image.RGBA)
	}
	c.frameInfo = fi
}

// --- television.PixelRenderer 実装 ---

func (c *capture) NewFrame(fi frameinfo.Current) error {
	c.resize(fi, false)
	return nil
}

func (c *capture) NewScanline(int) error { return nil }

func (c *capture) SetPixels(sig []signal.SignalAttributes, last int) error {
	var offset int
	for i := range sig {
		var col color.RGBA
		if sig[i].VBlank || sig[i].Index == signal.NoSignal {
			col = c.frameInfo.Spec.GetColor(signal.ZeroBlack)
		} else {
			col = c.frameInfo.Spec.GetColor(sig[i].Color)
		}
		s := c.img.Pix[offset : offset+3 : offset+3]
		s[0], s[1], s[2] = col.R, col.G, col.B
		offset += 4
	}
	return nil
}

func (c *capture) Reset()              { c.clear() }
func (c *capture) EndRendering() error { return nil }

// snapshot は最新フレームの可視域を独立した image.RGBA コピーで返す（以後の駆動で
// 上書きされない）。visibleTop は縦座標マッピング用（クロップ y=0 の絶対 scanline）。
func (c *capture) snapshot() (img *image.RGBA, visibleTop int) {
	src := c.cropImg
	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst, c.frameInfo.VisibleTop
}
