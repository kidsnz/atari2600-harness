// Package ocr verifies that the digits a ROM RENDERS match the value it holds in
// RAM (VV-9) — tying the display back to program meaning, which exact frame
// hashes can't (they'd also pass a kernel that draws the wrong glyph from a
// consistent-but-wrong font index). It reads the rendered pixels (not the
// registers), matches each glyph against templates rendered from a ground-truth
// font (the spec), and decodes the displayed number. A display-kernel / BCD-split
// / font-index bug makes the rendered glyph diverge from the spec and is caught.
package ocr

import "image"

// Font is the ground-truth glyph table: 10 digits (0-9), 8 rows each, one byte
// per row (bit pattern). This is the SPEC; the ROM's own font table is the
// implementation under test.
type Font [10][8]byte

// digitCells turns a font byte into 8 left-to-right on/off cells for a playfield
// register. PF1 renders MSB-first (col4=D7..col11=D0); PF2 renders LSB-first
// (col12=D0..col19=D7). (CLAUDE.md playfield bit order, hardware-verified.)
func digitCells(b byte, pf2 bool) [8]bool {
	var c [8]bool
	for i := 0; i < 8; i++ {
		if pf2 {
			c[i] = b&(1<<uint(i)) != 0 // LSB-first
		} else {
			c[i] = b&(1<<uint(7-i)) != 0 // MSB-first
		}
	}
	return c
}

// region is the horizontal pixel span of one digit and its PF bit order.
type region struct {
	x0, x1 int // visible-x span (inclusive..exclusive)
	pf2    bool
}

// Default 2-digit layout for the score2 litmus: tens in PF1 (clocks 16-47),
// ones in PF2 (clocks 48-79). Each digit is 8 cells of 4 clocks.
var defaultRegions = []region{
	{16, 48, false}, // tens, PF1
	{48, 80, true},  // ones, PF2
}

// linesPerRow is how many scanlines the score2 kernel holds each glyph row.
const linesPerRow = 8

// bandRows finds the band TOP (first row with any playfield-on pixel in x[16,80))
// and samples the middle of each of the 8 glyph rows at the fixed kernel spacing.
// Detecting the top (then a known row height) is robust to blank glyph rows — a
// blank final row would otherwise shorten an extent-based split and misalign all
// rows.
func bandRows(img *image.RGBA) ([]int, bool) {
	b := img.Bounds()
	on := func(x, y int) bool {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return (r>>8)+(g>>8)+(bl>>8) > 120
	}
	top := -1
	for y := 0; y < b.Dy() && top < 0; y++ {
		for x := 16; x < 80; x++ {
			if on(x, y) {
				top = y
				break
			}
		}
	}
	if top < 0 || top+7*linesPerRow+linesPerRow/2 >= b.Dy() {
		return nil, false
	}
	ys := make([]int, 8)
	for r := 0; r < 8; r++ {
		ys[r] = top + r*linesPerRow + linesPerRow/2
	}
	return ys, true
}

// extractGlyph reads an 8(row)x8(cell) on/off bitmap for one region.
func extractGlyph(img *image.RGBA, reg region, ys []int) [8][8]bool {
	b := img.Bounds()
	on := func(x, y int) bool {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return (r>>8)+(g>>8)+(bl>>8) > 120
	}
	cellW := (reg.x1 - reg.x0) / 8
	var g [8][8]bool
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			x := reg.x0 + c*cellW + cellW/2
			g[r][c] = on(x, ys[r])
		}
	}
	return g
}

// template renders one font digit to the same 8x8 cell bitmap for a region.
func template(font Font, digit int, pf2 bool) [8][8]bool {
	var g [8][8]bool
	for r := 0; r < 8; r++ {
		g[r] = digitCells(font[digit][r], pf2)
	}
	return g
}

func hamming(a, b [8][8]bool) int {
	d := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if a[r][c] != b[r][c] {
				d++
			}
		}
	}
	return d
}

// matchDigit returns the nearest font digit (by Hamming) and its distance.
func matchDigit(g [8][8]bool, font Font, pf2 bool) (digit, dist int) {
	best, bestD := -1, 1<<30
	for d := 0; d < 10; d++ {
		if h := hamming(g, template(font, d, pf2)); h < bestD {
			best, bestD = d, h
		}
	}
	return best, bestD
}

// Result is the OCR outcome for a 2-digit display.
type Result struct {
	Tens, Ones int  // decoded digits (nearest font template)
	TensDist   int  // Hamming distance of the best match (0 = exact)
	OnesDist   int
	OK         bool // band found and both digits decoded
}

// ReadScore2 decodes the two rendered digits using `font` as the ground-truth
// template set. visTop is unused (image is already the visible crop) but kept
// for signature symmetry with emu.Snapshot.
func ReadScore2(img *image.RGBA, font Font) Result {
	ys, ok := bandRows(img)
	if !ok {
		return Result{}
	}
	t, td := matchDigit(extractGlyph(img, defaultRegions[0], ys), font, false)
	o, od := matchDigit(extractGlyph(img, defaultRegions[1], ys), font, true)
	return Result{Tens: t, Ones: o, TensDist: td, OnesDist: od, OK: true}
}

// ExpectedBCD is the displayed value as a packed BCD byte (tens<<4 | ones).
func (r Result) ExpectedBCD() byte { return byte(r.Tens<<4 | r.Ones) }
