// Package annotate overlays a captured frame (160 x visible height) with a grid in real TIA
// coordinates, axis labels, and sprite position markers, and produces an image upscaled to a
// human-readable size.
//
// This is not a Claude-only aid but **the user↔Claude communication channel**. So that the user
// can look at the image and say "move P0 to clock 80" and that clock value maps directly onto
// register operations, the grid is calibrated to real TIA coordinates (horizontal clock 0..159 /
// vertical visible scanlines).
package annotate

import (
	"fmt"
	"image"
	"image/color"
	"sort"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font/basicfont"
)

// Marker is the horizontal position marker for one object. Clock is visible 0..159 (HmovedPixel). Negative means it is not drawn.
type Marker struct {
	Label string
	Clock int
	Col   color.RGBA
	// S-4: the player's current GRP bit pattern (0 = none). Reflect mirrors it horizontally,
	// Wide is the pixel multiplier (NUSIZ 1x/2x/4x). One row of "the picture currently in GRP"
	// is overlaid at actual size at the marker position in the annotated image.
	Gfx     uint8
	Reflect bool
	Wide    int
	// Drawn records whether this object painted a visible pixel in the frame, taken
	// from the per-pixel attribution buffer rather than inferred from the registers.
	// A TIA object always HAS a position; that says nothing about whether it is on
	// screen. Rendering a marker for an object that drew nothing puts a labelled
	// vertical line over a picture that does not contain it — and this image is the
	// primary channel the user reads a picture through, so that is a false statement
	// about the ROM, not a cosmetic blemish. Measured on a playfield-only sunset
	// kernel: five markers, five objects, zero of them on screen.
	Drawn bool
}

const (
	leftMargin = 30 // for y-axis labels
	topMargin  = 16 // for x-axis labels
	rightPad   = 10
	botPad     = 30 // for the two rows of marker labels
)

// LeftMargin/TopMargin are Render's drawing origin (exported so the pixel position where TIA (0,0) lands can be computed).
const (
	LeftMargin = leftMargin
	TopMargin  = topMargin
)

var (
	// Note: Go's color.RGBA is alpha-premultiplied. A channel > alpha (e.g. {255,255,255,30}) is
	// an invalid value and breaks compositing on a bright background (harmless by luck on a black
	// background = a bug that stayed hidden until v1.14.1).
	// For translucency, setLine uses dc.SetRGBA (non-premultiplied 0..1) instead.
	gridMinorA = 30.0 / 255
	gridMajorA = 70.0 / 255
	labelCol   = color.RGBA{205, 215, 225, 255}
)

// GridScanline returns the y label the grid draws for image row `row`, i.e. the absolute
// scanline. GridRow is its inverse. These two are the single definition of the argument
// convention for read_row / decompose_row; the promise "you can pass the y you saw in a
// screenshot as-is" is concentrated in this one place.
// The same formula used to be written separately in Render and emu.ReadRow (it drifted once and
// was fixed in v1.4.0).
func GridScanline(visibleTop, row int) int { return visibleTop + row }

// GridRow is the inverse transform of GridScanline.
func GridRow(visibleTop, scanline int) int { return scanline - visibleTop }

// Render returns the annotated image. scale is an integer magnification (×3〜4 recommended).
// visibleTop is the origin for printing the vertical labels as absolute scanlines (the absolute
// scanline of crop y=0).
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

	// Vertical grid lines (clock). Every 10, emphasized + labeled every 40. The right edge, 159, is labeled too.
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

	// Horizontal grid lines (visible scanline rows). Every 20, emphasized + labeled with the absolute scanline every 40.
	for r := 0; r <= fh; r += 20 {
		major := r%40 == 0
		setLine(dc, major)
		dc.DrawLine(left, cy(r), right, cy(r))
		dc.Stroke()
		if major && r != 0 { // r=0 is omitted because it collides with the top-left clock label
			dc.SetColor(labelCol)
			dc.DrawStringAnchored(fmt.Sprintf("%d", GridScanline(visibleTop, r)), left-3, cy(r), 1, 0.5)
		}
	}

	// Sprite markers (vertical line + numeric label). Sort the visible ones by clock and assign
	// labels to two rows by rank parity = labels adjacent on screen always land on different
	// rows, avoiding overlap.
	// An object that drew nothing gets no marker. It still has a position — every TIA
	// object always does — but a labelled line over a picture that does not contain
	// the object is a false statement about the ROM, and this image is how the user
	// reads the picture. Measured on a playfield-only kernel: five markers for five
	// objects, none of them on screen.
	vis := make([]Marker, 0, len(markers))
	for _, m := range markers {
		if m.Drawn && m.Clock >= 0 && m.Clock <= fw {
			vis = append(vis, m)
		}
	}
	sort.SliceStable(vis, func(i, j int) bool { return vis[i].Clock < vis[j].Clock })
	for rank, m := range vis {
		dc.SetColor(m.Col)
		dc.SetLineWidth(1.5)
		dc.DrawLine(cx(m.Clock), top, cx(m.Clock), bottom)
		dc.Stroke()
		ly := bottom + 7 + float64((rank%2)*12)
		dc.DrawStringAnchored(fmt.Sprintf("%s:%d", m.Label, m.Clock), cx(m.Clock), ly, 0.5, 0.5)
		// S-4: show the current GRP pattern at actual size just above the marker position (D7 is leftmost; REFP mirrors it)
		if m.Gfx != 0 {
			w := m.Wide
			if w <= 0 {
				w = 1
			}
			for bit := 0; bit < 8; bit++ {
				b := uint(7 - bit)
				if m.Reflect {
					b = uint(bit)
				}
				if m.Gfx&(1<<b) == 0 {
					continue
				}
				x0 := cx(m.Clock + bit*w)
				dc.DrawRectangle(x0, bottom-float64(4*scale)-2, float64(w*scale), float64(3*scale))
				dc.Fill()
			}
		}
	}

	return dc.Image().(*image.RGBA)
}

func setLine(dc *gg.Context, major bool) {
	if major {
		dc.SetRGBA(1, 1, 1, gridMajorA)
	} else {
		dc.SetRGBA(1, 1, 1, gridMinorA)
	}
	dc.SetLineWidth(1)
}

// upscale performs integer-factor nearest-neighbor scaling (keeps pixels crisp).
func upscale(src *image.RGBA, scale int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}
