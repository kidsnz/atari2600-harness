package emu

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/jetsetilly/gopher2600/hardware/television/frameinfo"
	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// capture implements television.PixelRenderer and takes the latest frame into an image.RGBA.
// The implementation follows the established pattern of Gopher2600's thumbnailer/image.go
// (signal→RGB conversion, crop to the visible area with frameInfo.Crop()).
//
// Coordinate convention of the cropped image (corroborated against litmus and the Crop()
// implementation):
//   - x = visible clock 0..159 (identical to a sprite's HmovedPixel)
//   - y = absolute scanline − VisibleTop (the first visible line is y=0)
type capture struct {
	img       *image.RGBA
	cropImg   *image.RGBA
	frameInfo frameinfo.Current
}

func newCapture() *capture {
	c := &capture{}
	c.img = image.NewRGBA(image.Rect(0, 0, specification.ClksScanline, specification.AbsoluteMaxScanlines))
	c.clear()
	// Fix the initial crop as NTSC (force=true). Updated later, once the frame spec is stable.
	c.resize(frameinfo.NewCurrent(specification.SpecNTSC), true)
	return c
}

// clear sets every pixel to black with alpha=255. SetPixels writes only RGB, so alpha is
// raised up front.
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

// --- television.PixelRenderer implementation ---

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

// colorRGBA converts a TIA colour-register value (COLUPx/COLUM/COLUBL/COLUPF/COLUBK,
// 0..255) into the exact RGBA the renderer used for it — the SAME palette as the
// captured frame, so a value read from ReadTIARegisters matches Snapshot pixels.
func (c *capture) colorRGBA(code uint8) color.RGBA {
	return c.frameInfo.Spec.GetColor(signal.ColorSignal(code))
}

// snapshot returns the visible area of the latest frame as an independent image.RGBA copy
// (later driving does not overwrite it). visibleTop is for mapping vertical coordinates (the
// absolute scanline of crop y=0).
func (c *capture) snapshot() (img *image.RGBA, visibleTop int) {
	src := c.cropImg
	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst, c.frameInfo.VisibleTop
}
