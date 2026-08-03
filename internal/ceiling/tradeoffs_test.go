package ceiling

import (
	"image"
	"math"
	"testing"
)

// rowsOf extracts one image row as the solver sees it.
func rowsOf(img *image.RGBA, y int) [][3]int {
	b := img.Bounds()
	px := make([][3]int, b.Dx())
	for x := 0; x < b.Dx(); x++ {
		i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
		px[x] = [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
	}
	return px
}

// gridBase returns, for a fixed colour pair, the per-pixel error of the C1
// picture: each column takes whichever colour of the pair costs less over the
// whole column.
func gridBase(px [][3]int, cols int, pal *Palette, a, b int) []int64 {
	cellW := len(px) / cols
	base := make([]int64, len(px))
	for c := 0; c < cols; c++ {
		var ea, eb int64
		for x := c * cellW; x < (c+1)*cellW; x++ {
			ea += int64(dist2(px[x], pal.Colors[a]))
			eb += int64(dist2(px[x], pal.Colors[b]))
		}
		v := a
		if eb < ea {
			v = b
		}
		for x := c * cellW; x < (c+1)*cellW; x++ {
			base[x] = int64(dist2(px[x], pal.Colors[v]))
		}
	}
	return base
}

// bestObjectGain returns the largest error an 8-clock object can remove from
// one line, holding the playfield picture fixed. free=true lets the window start
// at ANY colour clock; free=false restricts it to the column grid, which is what
// the shipped C2 does.
func bestObjectGain(px [][3]int, cols int, pal *Palette, base []int64, free bool) int64 {
	cellW := len(px) / cols
	win := SpriteColumns * cellW
	step := cellW
	if free {
		step = 1
	}
	var best int64
	for t := 0; t < PaletteSize; t++ {
		imp := make([]int64, len(px))
		for x := range px {
			g := base[x] - int64(dist2(px[x], pal.Colors[t]))
			if g > 0 {
				imp[x] = g
			}
		}
		for s := 0; s+win <= len(px); s += step {
			var tot int64
			for x := s; x < s+win; x++ {
				tot += imp[x]
			}
			if tot > best {
				best = tot
			}
		}
	}
	return best
}

// THE TRADE, MEASURED. C2's object window is aligned to the playfield's column
// grid (39 positions on a 40-column line) instead of free to start at any of the
// 153 colour clocks. Restricting the search can only make the computed error
// LARGER, so it can only understate the machine — the safe direction for a bound
// whose job is to say "the hardware cannot do this" — but "safe direction" is not
// the same as "small", and the size is a fact, not an intuition.
//
// The comparison holds everything else fixed: same line, same playfield pair
// (each line's own C1 optimum), same 128 object colours, same 8-clock width. The
// only difference is where the window may start.
func TestColumnAlignedObjectWindowCostsThisMuchVersusAFreeStart(t *testing.T) {
	pal := ntscPalette(t)
	graded := 0
	for _, rom := range []string{
		"litmus_nusiz_all.bin", "litmus_missile.bin", "litmus_objsizes.bin",
		"litmus_collide_mp.bin", "litmus_sprite.bin", "litmus_hmove_side.bin",
	} {
		img := frameOf(t, litmusDir+rom, 8)
		a := ladder(t, img, pal)
		var alignedSum, freeSum, c1Sum int64
		h := img.Bounds().Dy()
		for y := 0; y < h; y++ {
			s := a.rows[y]
			px := rowsOf(img, y)
			base := gridBase(px, a.cols, &pal, s.c1a, s.c1b)
			var tot int64
			for _, v := range base {
				tot += v
			}
			c1Sum += tot
			alignedSum += tot - bestObjectGain(px, a.cols, &pal, base, false)
			freeSum += tot - bestObjectGain(px, a.cols, &pal, base, true)
		}
		if freeSum > alignedSum {
			t.Errorf("%s: a free window start scored WORSE (%d) than the column-aligned one (%d) — "+
				"the aligned positions are a subset, so this is impossible", rom, freeSum, alignedSum)
		}
		n := float64(img.Bounds().Dx()*h) * 3
		rmse := func(v int64) float64 { return math.Sqrt(float64(v) / n) }
		shipped, _ := a.Result.RMSEOf(C2)
		t.Logf("%-24s C1=%.3f  C2(aligned,fixed picks)=%.3f  C2(free start,fixed picks)=%.3f  "+
			"alignment costs %.4f rmse  |  shipped C2 (pair search + pick flips)=%.3f",
			rom, rmse(c1Sum), rmse(alignedSum), rmse(freeSum), rmse(alignedSum)-rmse(freeSum), shipped)
		if shipped > rmse(alignedSum)+1e-9 {
			t.Errorf("%s: the shipped C2 (%.4f) is worse than the fixed-pick aligned reference (%.4f) — "+
				"the shipped search is supposed to be a superset", rom, shipped, rmse(alignedSum))
		}
		graded++
	}
	t.Logf("alignment cost measured on %d frames", graded)
}

// protoC1 reimplements the PROTOTYPE's C1 (sandbox/experiments/visual-ceiling/
// upper.py): collapse each column to its mean colour, choose the pair that
// minimises the error of those MEANS, then pick each column by which pair member
// is nearer its mean, and charge the error at pixel resolution.
//
// Both steps are proxies for the thing actually wanted, which is the picture with
// the least pixel error. The shipped C1 optimises that directly, at the same
// cost — cellCost[c][v] is the true per-column price and the pair scan is the
// same 8256 iterations. So the shipped C1 must never be worse, and where it is
// better the prototype was overstating how badly the hardware does.
func protoC1(img *image.RGBA, cols int, pal *Palette) int64 {
	h := img.Bounds().Dy()
	var total int64
	for y := 0; y < h; y++ {
		px := rowsOf(img, y)
		cellW := len(px) / cols
		means := make([][3]int, cols)
		for c := 0; c < cols; c++ {
			var r, g, b int
			for x := c * cellW; x < (c+1)*cellW; x++ {
				r += px[x][0]
				g += px[x][1]
				b += px[x][2]
			}
			means[c] = [3]int{r / cellW, g / cellW, b / cellW}
		}
		bestErr, bestA, bestB := int64(math.MaxInt64), 0, 0
		for a := 0; a < PaletteSize; a++ {
			for b := a; b < PaletteSize; b++ {
				var s int64
				for c := 0; c < cols; c++ {
					s += int64(minI32(dist2(means[c], pal.Colors[a]), dist2(means[c], pal.Colors[b])))
				}
				if s < bestErr {
					bestErr, bestA, bestB = s, a, b
				}
			}
		}
		for c := 0; c < cols; c++ {
			v := bestA
			if dist2(means[c], pal.Colors[bestB]) < dist2(means[c], pal.Colors[bestA]) {
				v = bestB
			}
			for x := c * cellW; x < (c+1)*cellW; x++ {
				total += int64(dist2(px[x], pal.Colors[v]))
			}
		}
	}
	return total
}

// The shipped C1 is a TRUE optimum of its constraint set, which the prototype's
// cell-mean formulation was not. This measures how much that is worth: a ceiling
// that is too high blames the hardware for error a kernel could actually have
// avoided, which is the same missing-denominator defect in miniature.
func TestShippedC1IsAtLeastAsTightAsThePrototypeCellMeanProxy(t *testing.T) {
	pal := ntscPalette(t)
	graded := 0
	for _, rom := range []string{
		"litmus_nusiz_all.bin", "litmus_objsizes.bin", "litmus_missile.bin",
		"litmus_hmove_side.bin", "litmus_ctrlpf.bin", "litmus_pf_allcols.bin",
	} {
		img := frameOf(t, litmusDir+rom, 8)
		a := ladder(t, img, pal)
		mine, _ := a.Result.RMSEOf(C1)
		n := float64(a.Result.Pixels) * 3
		proxy := math.Sqrt(float64(protoC1(img, a.cols, &pal)) / n)
		if mine > proxy+1e-9 {
			t.Errorf("%s: shipped C1 %.4f is LOOSER than the cell-mean proxy %.4f — the exhaustive "+
				"pixel-level optimum cannot be beaten by a proxy for it", rom, mine, proxy)
		}
		t.Logf("%-24s shipped C1 %7.3f   cell-mean proxy %7.3f   tighter by %.3f rmse", rom, mine, proxy, proxy-mine)
		graded++
	}
	t.Logf("cell-mean-proxy comparison graded on %d frames", graded)
}
