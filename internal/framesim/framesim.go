// Package framesim is a TOLERANT frame comparator (VV-12,
// docs/capability-gap-audit.md). The exact golden-frame check (scenario
// golden_frame) answers a boolean "identical?"; framesim answers "how wrong,
// and where" — magnitude (SSIM, perceptual-hash distance) and locality (the
// worst-matching block). It complements, and does not replace, the exact golden:
// a 1-pixel jitter that flips the exact hash still scores SSIM ~1.0, while a
// genuinely corrupted frame scores far lower.
//
// Pure Go, no external dependencies.
package framesim

import (
	"image"
	"image/color"
	"math"
)

// luma returns the Rec.601 luma of an RGBA pixel at (x,y).
func luma(img *image.RGBA, x, y int) float64 {
	i := img.PixOffset(x, y)
	r := float64(img.Pix[i])
	g := float64(img.Pix[i+1])
	b := float64(img.Pix[i+2])
	return 0.299*r + 0.587*g + 0.114*b
}

// SSIMResult holds the structural-similarity verdict.
type SSIMResult struct {
	Mean       float64         // mean SSIM over all blocks, 1.0 = identical
	Worst      float64         // lowest block SSIM (the most-divergent region)
	WorstBlock image.Rectangle // location of that block (in image coords)
	Blocks     int
}

const (
	ssimBlock = 8
	ssimL     = 255.0
)

var (
	ssimC1 = (0.01 * ssimL) * (0.01 * ssimL)
	ssimC2 = (0.03 * ssimL) * (0.03 * ssimL)
)

// SSIM computes the windowed structural similarity of two same-size images over
// 8x8 luma blocks. Returns mean + worst block (magnitude + locality). The images
// must have identical bounds.
func SSIM(a, b *image.RGBA) (SSIMResult, bool) {
	ra, rb := a.Bounds(), b.Bounds()
	if ra.Dx() != rb.Dx() || ra.Dy() != rb.Dy() {
		return SSIMResult{}, false
	}
	w, h := ra.Dx(), ra.Dy()
	res := SSIMResult{Mean: 1, Worst: 1}
	var sum float64
	n := 0
	for by := 0; by+ssimBlock <= h; by += ssimBlock {
		for bx := 0; bx+ssimBlock <= w; bx += ssimBlock {
			s := blockSSIM(a, b, ra.Min.X+bx, ra.Min.Y+by, rb.Min.X+bx, rb.Min.Y+by)
			sum += s
			n++
			if s < res.Worst {
				res.Worst = s
				res.WorstBlock = image.Rect(bx, by, bx+ssimBlock, by+ssimBlock)
			}
		}
	}
	if n == 0 {
		return SSIMResult{}, false
	}
	res.Mean = sum / float64(n)
	res.Blocks = n
	return res, true
}

// Resize returns a nearest-neighbor rescale of src to w×h. TIA frames are blocky
// (no anti-aliasing), so nearest-neighbor preserves hard edges without inventing
// interpolated colors that would skew SSIM.
func Resize(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if sw == 0 || sh == 0 || w == 0 || h == 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(sb.Min.X+x*sw/w, sy))
		}
	}
	return dst
}

// NormalizeSize rescales a and b to a common size so frames captured at different
// SCALES — e.g. a 1× ROM render (160×N) vs a 2× Stella screenshot (320×M) — can be
// compared instead of erroring on a bounds mismatch. The common size is the
// per-axis minimum (downscale only — never invent detail). It returns the rescaled
// pair and the common size. NOTE: this normalizes scale, NOT vertical framing — for
// a meaningful score both frames should cover the same visible region (differing
// VBLANK/overscan margins shift content; aligning that is a separate concern).
func NormalizeSize(a, b *image.RGBA) (*image.RGBA, *image.RGBA, image.Point) {
	wa, ha := a.Bounds().Dx(), a.Bounds().Dy()
	wb, hb := b.Bounds().Dx(), b.Bounds().Dy()
	if wa == wb && ha == hb {
		return a, b, image.Pt(wa, ha)
	}
	w, h := min(wa, wb), min(ha, hb)
	return Resize(a, w, h), Resize(b, w, h), image.Pt(w, h)
}

// DiffStats localizes where two frames differ (lit-state mismatch). Mismatch =
// pixels lit in exactly one frame. AOnly = lit in A but not B (A has extra),
// BOnly = lit in B but not A (A is MISSING what B shows). RowMiss[y] = mismatches
// on that row, so a band with many BOnly says "the target draws something here that
// I don't" (e.g. a missing score/paddle/ball).
type DiffStats struct {
	W, H     int
	Mismatch int
	AOnly    int
	BOnly    int
	BBox     image.Rectangle
	RowMiss  []int
}

// Diff compares two SAME-SIZE frames pixel-by-pixel (call NormalizeSize first) and
// returns a visualization + localization: a diff image (match-dark / both-lit grey /
// A-only red / B-only blue) and per-row + bbox stats. "Lit" = luma > 128. This is
// the autonomous compare→localize step: render → Diff vs target → fix every flagged
// region → repeat, instead of eyeballing.
func Diff(a, b *image.RGBA) (*image.RGBA, DiffStats, bool) {
	ra, rb := a.Bounds(), b.Bounds()
	if ra.Dx() != rb.Dx() || ra.Dy() != rb.Dy() {
		return nil, DiffStats{}, false
	}
	w, h := ra.Dx(), ra.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	st := DiffStats{W: w, H: h, RowMiss: make([]int, h), BBox: image.Rect(w, h, 0, 0)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			la := luma(a, ra.Min.X+x, ra.Min.Y+y) > 128
			lb := luma(b, rb.Min.X+x, rb.Min.Y+y) > 128
			var c color.RGBA
			diff := false
			switch {
			case la && lb:
				c = color.RGBA{60, 60, 60, 255} // both lit
			case !la && !lb:
				c = color.RGBA{0, 0, 0, 255} // both dark
			case la && !lb:
				c, diff = color.RGBA{255, 0, 0, 255}, true // A-only (mine extra)
				st.AOnly++
			default:
				c, diff = color.RGBA{0, 128, 255, 255}, true // B-only (target shows, mine missing)
				st.BOnly++
			}
			out.SetRGBA(x, y, c)
			if diff {
				st.Mismatch++
				st.RowMiss[y]++
				if x < st.BBox.Min.X {
					st.BBox.Min.X = x
				}
				if y < st.BBox.Min.Y {
					st.BBox.Min.Y = y
				}
				if x+1 > st.BBox.Max.X {
					st.BBox.Max.X = x + 1
				}
				if y+1 > st.BBox.Max.Y {
					st.BBox.Max.Y = y + 1
				}
			}
		}
	}
	if st.Mismatch == 0 {
		st.BBox = image.Rectangle{}
	}
	return out, st, true
}

func blockSSIM(a, b *image.RGBA, ax, ay, bx, by int) float64 {
	var sa, sb, saa, sbb, sab float64
	const nn = ssimBlock * ssimBlock
	for dy := 0; dy < ssimBlock; dy++ {
		for dx := 0; dx < ssimBlock; dx++ {
			va := luma(a, ax+dx, ay+dy)
			vb := luma(b, bx+dx, by+dy)
			sa += va
			sb += vb
			saa += va * va
			sbb += vb * vb
			sab += va * vb
		}
	}
	ma := sa / nn
	mb := sb / nn
	va := saa/nn - ma*ma
	vb := sbb/nn - mb*mb
	cov := sab/nn - ma*mb
	return ((2*ma*mb + ssimC1) * (2*cov + ssimC2)) /
		((ma*ma + mb*mb + ssimC1) * (va + vb + ssimC2))
}

// --- perceptual hash (DCT-based pHash) ---

const phashN = 32 // working resolution before DCT

// PHash returns a 64-bit DCT perceptual hash. Robust to small shifts/scaling:
// two frames that look alike hash close (small Hamming distance) even if not
// bit-identical.
func PHash(img *image.RGBA) uint64 {
	// downsample to phashN x phashN luma via box sampling
	var g [phashN][phashN]float64
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	for y := 0; y < phashN; y++ {
		for x := 0; x < phashN; x++ {
			sx := r.Min.X + x*w/phashN
			sy := r.Min.Y + y*h/phashN
			g[y][x] = luma(img, sx, sy)
		}
	}
	// 2D DCT-II
	var d [phashN][phashN]float64
	for u := 0; u < 8; u++ { // only need the top-left 8x8 low-frequency block
		for v := 0; v < 8; v++ {
			var s float64
			for x := 0; x < phashN; x++ {
				for y := 0; y < phashN; y++ {
					s += g[x][y] *
						math.Cos((2*float64(x)+1)*float64(u)*math.Pi/(2*phashN)) *
						math.Cos((2*float64(y)+1)*float64(v)*math.Pi/(2*phashN))
				}
			}
			d[u][v] = s
		}
	}
	// median of the 8x8 low-freq block, excluding the DC term (0,0)
	var vals []float64
	for u := 0; u < 8; u++ {
		for v := 0; v < 8; v++ {
			if u == 0 && v == 0 {
				continue
			}
			vals = append(vals, d[u][v])
		}
	}
	med := median(vals)
	var hash uint64
	bit := 0
	for u := 0; u < 8; u++ {
		for v := 0; v < 8; v++ {
			if u == 0 && v == 0 {
				continue
			}
			if d[u][v] > med {
				hash |= 1 << uint(bit)
			}
			bit++
		}
	}
	return hash
}

func median(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	// copy + insertion sort (n=63, tiny)
	c := make([]float64, n)
	copy(c, xs)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && c[j-1] > c[j]; j-- {
			c[j-1], c[j] = c[j], c[j-1]
		}
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// HammingDistance is the number of differing bits between two pHashes (0..64).
func HammingDistance(a, b uint64) int {
	x := a ^ b
	c := 0
	for x != 0 {
		x &= x - 1
		c++
	}
	return c
}
