package ceiling

import (
	"fmt"
	"image"
)

// Render draws a rung's optimal picture — the best the constraint set can do.
//
// This is not decoration. Rendering the C1 ceiling is how the ladder decision was
// actually made: on Chopper Command the landscape survives nearly intact while
// the helicopter, the score digits and the ACTIVISION logo collapse into 4-clock
// smears, which says in a picture "the playfield can do the scenery and cannot do
// the actors". A single rmse never says that.
//
// The returned image is byte-identical to the picture whose error Compute
// reported for that rung, so `Render` output vs target is the same measurement.
func (a *Analysis) Render(r Rung) (*image.RGBA, error) {
	if !a.want[r] {
		return nil, fmt.Errorf("ceiling: rung %s was not computed", r)
	}
	img := image.NewRGBA(image.Rect(0, 0, a.w, a.h))
	set := func(x, y, v int) {
		c := a.pal.RGBA(v)
		i := img.PixOffset(x, y)
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	for y := 0; y < a.h; y++ {
		s := a.rows[y]
		switch r {
		case C1:
			a.paintGrid(img, y, s.c1a, s.c1b, set)
		case C2:
			a.paintGrid(img, y, s.c2a, s.c2b, set)
			// The object window. Two things happen here that the plain grid does
			// not do, and both are part of the optimum rather than cosmetic:
			// the covered columns re-choose their playfield colour knowing the
			// object will cover part of them, and each colour clock inside the
			// window independently takes the object colour when that is closer —
			// which is exactly what a player's 8 GRP bits control.
			lo := s.c2k * a.cellW
			hi := (s.c2k + SpriteColumns) * a.cellW
			for c := s.c2k; c < s.c2k+SpriteColumns; c++ {
				var ea, eb int64
				for x := c * a.cellW; x < (c+1)*a.cellW; x++ {
					tgt := a.target(x, y)
					dt := dist2(tgt, a.pal.Colors[s.c2t])
					ea += int64(minI32(dist2(tgt, a.pal.Colors[s.c2a]), dt))
					eb += int64(minI32(dist2(tgt, a.pal.Colors[s.c2b]), dt))
				}
				v := s.c2a
				if eb < ea {
					v = s.c2b
				}
				for x := c * a.cellW; x < (c+1)*a.cellW; x++ {
					set(x, y, v)
				}
			}
			for x := lo; x < hi; x++ {
				i := img.PixOffset(x, y)
				under := [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
				tgt := a.target(x, y)
				if dist2(tgt, a.pal.Colors[s.c2t]) < dist2(tgt, under) {
					set(x, y, s.c2t)
				}
			}
		case C3:
			for x := 0; x < a.w; x++ {
				tgt := a.target(x, y)
				v := s.c3a
				if dist2(tgt, a.pal.Colors[s.c3b]) < dist2(tgt, a.pal.Colors[s.c3a]) {
					v = s.c3b
				}
				set(x, y, v)
			}
		}
	}
	return img, nil
}

// paintGrid fills one line with the two-colour playfield picture: each column
// takes whichever of the pair costs less over the whole column, which is the
// choice C1's optimum was computed with.
func (a *Analysis) paintGrid(img *image.RGBA, y, pa, pb int, set func(x, y, v int)) {
	for c := 0; c < a.cols; c++ {
		var ea, eb int64
		for x := c * a.cellW; x < (c+1)*a.cellW; x++ {
			tgt := a.target(x, y)
			ea += int64(dist2(tgt, a.pal.Colors[pa]))
			eb += int64(dist2(tgt, a.pal.Colors[pb]))
		}
		v := pa
		if eb < ea {
			v = pb
		}
		for x := c * a.cellW; x < (c+1)*a.cellW; x++ {
			set(x, y, v)
		}
	}
}

// SetTarget hands Render the frame Compute measured, so it can pick per-column
// and per-pixel exactly as the optimum did. Compute calls it; callers do not.
func (a *Analysis) target(x, y int) [3]int {
	i := a.src.PixOffset(a.srcOrigin.X+x, a.srcOrigin.Y+y)
	return [3]int{int(a.src.Pix[i]), int(a.src.Pix[i+1]), int(a.src.Pix[i+2])}
}
