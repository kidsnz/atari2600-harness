// Package vismatch does palette-INDEPENDENT visual reproduction diffing: it
// renders two ROMs (a target and your build), reads WHICH TIA object drew each
// pixel (emu.DecomposeRow → BG/PF/P0/P1/M0/M1/BL) on every visible scanline, and
// reports every element-level difference — plus a per-element "band diff" that
// groups differing scanlines into vertical bands with the exact lit clock-spans
// on each side. This is the tool that catches, in one shot, the kind of 1-2px
// playfield-band boundary errors that manual sparse-sampling misses.
//
// Palette independence is the whole point: two different ROMs use different
// colour palettes, so a raw RGB diff is useless. Comparing object attribution
// (PF-vs-PF, P0-vs-P0, …) is exact and palette-free.
package vismatch

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Grid is a per-pixel TIA-object attribution of one rendered frame's visible
// area. Elem[y][x] is the object that drew visible pixel (clock=x) on absolute
// scanline (Top+y): one of BG/PF/P0/P1/M0/M1/BL/none. Hex[y][x] is that pixel's
// RGB (kept for the overlay / reference only — never diffed across ROMs).
type Grid struct {
	Top  int        // absolute scanline of row 0
	W, H int        // width (160) and visible height
	Elem [][]string // [H][W] object name
	Hex  [][]string // [H][W] "RRGGBB"
}

// ExtractROM renders romPath and returns its element grid. When reset is true it
// performs the console RESET dance (press RESET, run 8 frames, release, run
// `frames`) — many original cartridges only start the game after RESET. When
// reset is false it simply runs `frames` frames (self-starting builds).
func ExtractROM(romPath, spec string, frames int, reset bool) (*Grid, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, fmt.Errorf("load %s: %w", romPath, err)
	}
	if reset {
		if err := e.SetPanel("reset", true); err != nil {
			return nil, err
		}
		if err := e.RunFrames(8); err != nil {
			return nil, err
		}
		if err := e.SetPanel("reset", false); err != nil {
			return nil, err
		}
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	img, top := e.Snapshot()
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	g := &Grid{Top: top, W: w, H: h, Elem: make([][]string, h), Hex: make([][]string, h)}
	for y := 0; y < h; y++ {
		sl := top + y
		erow := make([]string, w)
		hrow := make([]string, w)
		for x := range erow {
			erow[x] = "BG"
		}
		if eruns, _, err := e.DecomposeRow(sl); err == nil {
			for _, r := range eruns {
				for x := r.Clock; x < r.Clock+r.Len && x < w; x++ {
					erow[x] = r.Element
				}
			}
		}
		if cruns, _, err := e.ReadRow(sl); err == nil {
			for _, r := range cruns {
				for x := r.Clock; x < r.Clock+r.Len && x < w; x++ {
					hrow[x] = r.Hex
				}
			}
		}
		g.Elem[y] = erow
		g.Hex[y] = hrow
	}
	return g, nil
}

// Report is the outcome of Diff.
type Report struct {
	ScanlineLo, ScanlineHi int            // absolute-scanline overlap range compared
	Width                  int            //
	Missing                map[string]int // target has element E here, mine does not  (per element)
	Extra                  map[string]int // mine has element E here, target does not
	Bands                  []BandDiff     // per-element grouped vertical band mismatches
	Match                  bool           // true when every compared element cell agrees
}

// BandDiff is a maximal run of consecutive scanlines over which one element's
// lit clock-spans differ between target and mine. TargetSpan/MineSpan are
// human-readable clock-run descriptions (e.g. "72-75,80-91").
type BandDiff struct {
	Element                string
	ScanlineLo, ScanlineHi int
	Height                 int
	TargetSpan             string
	MineSpan               string
}

// allElems is the fixed object set (BG excluded from band diffs — it's the
// negative space, already captured by the other elements' presence).
var allElems = []string{"PF", "P0", "P1", "M0", "M1", "BL"}

// Diff compares two grids by object attribution, aligned on ABSOLUTE scanline
// (both are NTSC/PAL with the same visible top, but we align defensively). It
// tallies per-element missing/extra cells and builds a per-element band diff.
func Diff(target, mine *Grid) *Report {
	lo := target.Top
	if mine.Top > lo {
		lo = mine.Top
	}
	hi := target.Top + target.H
	if mine.Top+mine.H < hi {
		hi = mine.Top + mine.H
	}
	w := target.W
	if mine.W < w {
		w = mine.W
	}
	r := &Report{ScanlineLo: lo, ScanlineHi: hi - 1, Width: w,
		Missing: map[string]int{}, Extra: map[string]int{}, Match: true}

	for sl := lo; sl < hi; sl++ {
		ta := target.Elem[sl-target.Top]
		mi := mine.Elem[sl-mine.Top]
		for x := 0; x < w; x++ {
			if ta[x] == mi[x] {
				continue
			}
			r.Match = false
			if ta[x] != "BG" {
				r.Missing[ta[x]]++
			}
			if mi[x] != "BG" {
				r.Extra[mi[x]]++
			}
		}
	}

	// Per-element band diff: for each element, per-scanline lit-clock signature,
	// group consecutive differing scanlines that share the same (target,mine) pair.
	for _, el := range allElems {
		var cur *BandDiff
		flush := func() {
			if cur != nil {
				cur.Height = cur.ScanlineHi - cur.ScanlineLo + 1
				r.Bands = append(r.Bands, *cur)
				cur = nil
			}
		}
		for sl := lo; sl < hi; sl++ {
			tsig := elemSpans(target.Elem[sl-target.Top], el)
			msig := elemSpans(mine.Elem[sl-mine.Top], el)
			if tsig == msig {
				flush()
				continue
			}
			if cur != nil && cur.TargetSpan == tsig && cur.MineSpan == msig {
				cur.ScanlineHi = sl
				continue
			}
			flush()
			cur = &BandDiff{Element: el, ScanlineLo: sl, ScanlineHi: sl, TargetSpan: tsig, MineSpan: msig}
		}
		flush()
	}
	sort.SliceStable(r.Bands, func(i, j int) bool {
		if r.Bands[i].ScanlineLo != r.Bands[j].ScanlineLo {
			return r.Bands[i].ScanlineLo < r.Bands[j].ScanlineLo
		}
		return r.Bands[i].Element < r.Bands[j].Element
	})
	return r
}

// elemSpans renders the lit clock-runs of one element on a scanline as a compact
// string like "72-75,80-91" ("-" when the element is absent on this scanline).
func elemSpans(row []string, el string) string {
	var parts []string
	x := 0
	for x < len(row) {
		if row[x] != el {
			x++
			continue
		}
		lo := x
		for x < len(row) && row[x] == el {
			x++
		}
		parts = append(parts, fmt.Sprintf("%d-%d", lo, x-1))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// Overlay renders a scale× object-attribution difference image over the compared
// scanline range: green = both sides agree (non-BG), dim = both BG, red =
// element in TARGET only (missing in yours), blue = element in MINE only (extra).
func (r *Report) Overlay(target, mine *Grid, scale int) *image.RGBA {
	if scale < 1 {
		scale = 1
	}
	h := r.ScanlineHi - r.ScanlineLo + 1
	out := image.NewRGBA(image.Rect(0, 0, r.Width*scale, h*scale))
	green := color.RGBA{70, 210, 110, 255}
	red := color.RGBA{230, 60, 60, 255}
	blue := color.RGBA{70, 120, 235, 255}
	bg := color.RGBA{22, 22, 28, 255}
	for i := 0; i < h; i++ {
		sl := r.ScanlineLo + i
		ta := target.Elem[sl-target.Top]
		mi := mine.Elem[sl-mine.Top]
		for x := 0; x < r.Width; x++ {
			var c color.RGBA
			switch {
			case ta[x] == mi[x] && ta[x] == "BG":
				c = bg
			case ta[x] == mi[x]:
				c = green
			case ta[x] != "BG" && mi[x] == "BG":
				c = red
			case mi[x] != "BG" && ta[x] == "BG":
				c = blue
			default: // both non-BG but different objects
				c = color.RGBA{235, 200, 60, 255} // amber = object-type mismatch
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					out.Set(x*scale+dx, i*scale+dy, c)
				}
			}
		}
	}
	return out
}
