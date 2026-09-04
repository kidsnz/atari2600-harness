// Package crt approximates what a television does to a 2600 picture, for LOOKING at artwork —
// never for measuring it.
//
// WHY THIS EXISTS. Everything else in this repository sees pixel-exact output that no console ever
// produced: `internal/emu` imports the engine's `hardware` tree and nothing from its GUI, where the
// real CRT model lives (OpenGL shaders, `gui/sdlimgui/gl32_crtseq*.go`), and there is no headless
// path through those. `docs/ingest.md` even instructs turning Stella's TV effects off, which is
// correct for extraction and wrong for judgement. Meanwhile the machine's own visual style depends on
// the blur — stella-list, 1997: *"the TV screen seems to act as an anti-aliasing device … especially
// true for the 2600 because its games made a massive use of colour-striping effects, that look much
// better on TV."*
//
// WHAT THIS IS NOT. It is not the engine's CRT model, not a signal simulation, and not evidence.
// Two effects, chosen because they are the two that change what a *design decision* should be:
//
//   - Bleed — a horizontal low-pass. A real NTSC set carries chroma at a fraction of luma bandwidth,
//     so adjacent colour clocks of different hue merge while brightness edges stay comparatively
//     sharp. Approximated here by blurring more strongly in the colour axes than in luminance.
//   - Persist — averaging successive frames, which is what makes a flickered object read as a dim
//     steady one rather than as blinking.
//
// Neither is calibrated against a television. **Nothing in this package may be used to decide whether
// a ROM is correct**; the pixel-exact comparisons stay the authority. Its only job is to let the
// person drawing see roughly what the drawing will look like before they call it finished.
package crt

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// yuv converts to a luma/chroma space so the two can be blurred by different amounts, which is the
// whole point: a television smears colour far more than brightness.
func yuv(c color.RGBA) (y, u, v float64) {
	r, g, b := float64(c.R), float64(c.G), float64(c.B)
	y = 0.299*r + 0.587*g + 0.114*b
	u = -0.14713*r - 0.28886*g + 0.436*b
	v = 0.615*r - 0.51499*g - 0.10001*b
	return
}

func rgb(y, u, v float64) color.RGBA {
	clamp := func(f float64) uint8 {
		if f < 0 {
			return 0
		}
		if f > 255 {
			return 255
		}
		return uint8(math.Round(f))
	}
	return color.RGBA{
		R: clamp(y + 1.13983*v),
		G: clamp(y - 0.39465*u - 0.58060*v),
		B: clamp(y + 2.03211*u),
		A: 255,
	}
}

// Bleed low-passes each row horizontally, chroma harder than luma. lumaTaps and chromaTaps are the
// half-widths in source pixels; chromaTaps should be the larger of the two or this is not modelling
// anything a television does. Taps of 0 leave that channel alone.
func Bleed(src *image.RGBA, lumaTaps, chromaTaps int) (*image.RGBA, error) {
	if src == nil {
		return nil, fmt.Errorf("nil image")
	}
	if lumaTaps < 0 || chromaTaps < 0 {
		return nil, fmt.Errorf("taps must not be negative (%d, %d)", lumaTaps, chromaTaps)
	}
	if chromaTaps < lumaTaps {
		return nil, fmt.Errorf("chromaTaps (%d) < lumaTaps (%d): a television smears colour MORE "+
			"than brightness, so this would be modelling the opposite of a television",
			chromaTaps, lumaTaps)
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var sy, su, sv float64
			var ny, nc float64
			for d := -chromaTaps; d <= chromaTaps; d++ {
				xx := x + d
				if xx < b.Min.X || xx >= b.Max.X {
					continue
				}
				cy, cu, cv := yuv(src.RGBAAt(xx, y))
				if d >= -lumaTaps && d <= lumaTaps {
					sy += cy
					ny++
				}
				su += cu
				sv += cv
				nc++
			}
			if ny == 0 {
				ny = 1
			}
			if nc == 0 {
				nc = 1
			}
			dst.SetRGBA(x, y, rgb(sy/ny, su/nc, sv/nc))
		}
	}
	return dst, nil
}

// Persist averages frames, approximating phosphor persistence. It is what turns a two-frame flicker
// into a dim steady object instead of a blinking one — the single most misleading difference between
// a still capture and a screen.
func Persist(frames []*image.RGBA) (*image.RGBA, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames")
	}
	b := frames[0].Bounds()
	for i, f := range frames {
		if f == nil {
			return nil, fmt.Errorf("frame %d is nil", i)
		}
		if f.Bounds() != b {
			return nil, fmt.Errorf("frame %d is %v, frame 0 is %v", i, f.Bounds(), b)
		}
	}
	dst := image.NewRGBA(b)
	n := float64(len(frames))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var r, g, bl float64
			for _, f := range frames {
				c := f.RGBAAt(x, y)
				r += float64(c.R)
				g += float64(c.G)
				bl += float64(c.B)
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
		}
	}
	return dst, nil
}
