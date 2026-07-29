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

// VerticalShift is the single number a human currently derives by hand from a
// band diff: how far the whole picture has moved.
//
// When every band is off by the same amount, the band-by-band report is N ways
// of saying one thing, and reading it back into that one thing is exactly where
// a 3-pixel error crept in during field use (a band model was built at -11 when
// the truth was -8). The search is cheap and the answer is a number, so there is
// no reason for a person to be doing it.
type VerticalShift struct {
	// Shift is added to a TARGET scanline to find the matching row in MINE, so a
	// positive value means mine sits LOWER on the screen.
	Shift int `json:"shift"`

	MismatchAtZero int     `json:"mismatch_at_zero"` // differing cells with no shift
	MismatchAtBest int     `json:"mismatch_at_best"` // differing cells at Shift
	Removed        float64 `json:"removed"`          // fraction of the mismatch the shift explains
}

// FindVerticalShift searches shifts in [-maxShift,+maxShift] for the one that
// explains the most element mismatch. It reports the count at zero alongside the
// best, because a shift that removes little is not a finding — two pictures can
// be wrong in ways that no alignment fixes, and a bare "best shift" would present
// that as an explanation.
func FindVerticalShift(target, mine *Grid, maxShift int) VerticalShift {
	count := func(s int) int {
		n := 0
		for y := 0; y < target.H; y++ {
			my := y + (target.Top - mine.Top) + s
			if my < 0 || my >= mine.H {
				continue
			}
			for x := 0; x < target.W && x < mine.W; x++ {
				if target.Elem[y][x] != mine.Elem[my][x] {
					n++
				}
			}
		}
		return n
	}
	zero := count(0)
	best, bestN := 0, zero
	for s := -maxShift; s <= maxShift; s++ {
		if s == 0 {
			continue
		}
		if n := count(s); n < bestN {
			best, bestN = s, n
		}
	}
	removed := 0.0
	if zero > 0 {
		removed = float64(zero-bestN) / float64(zero)
	}
	return VerticalShift{Shift: best, MismatchAtZero: zero, MismatchAtBest: bestN, Removed: removed}
}

// Describe renders the shift as the one line it exists to replace.
func (v VerticalShift) Describe() string {
	if v.Shift == 0 {
		return fmt.Sprintf("no global vertical shift explains the difference (%d mismatched cells stand)",
			v.MismatchAtZero)
	}
	dir := "LOWER"
	n := v.Shift
	if n < 0 {
		dir, n = "HIGHER", -n
	}
	return fmt.Sprintf("best global vertical shift: mine sits %d scanline(s) %s than the target "+
		"(removes %.0f%% of the mismatch: %d -> %d cells)",
		n, dir, 100*v.Removed, v.MismatchAtZero, v.MismatchAtBest)
}

// GameplayStart is where a ROM's picture settles after its opening screen.
type GameplayStart struct {
	Frame     int  `json:"frame"`      // first frame of the settled picture
	Found     bool `json:"found"`      // a change followed by stability was seen
	Changed   bool `json:"changed"`    // the playfield ever differed from frame 0's
	StableFor int  `json:"stable_for"` // frames of stability the answer required
	Searched  int  `json:"searched"`   // frames examined
}

// FindGameplayStart runs a ROM and reports the first frame whose PLAYFIELD both
// differs from the opening frame's and then holds still for `stable` frames.
//
// It exists because comparing two ROMs from frame 0 measures one game's title
// against another's gameplay, and every mechanic then reads as different for a
// reason that has nothing to do with mechanics. The warmup that avoids that is
// currently a number someone tunes by hand.
//
// The playfield is the signal rather than the whole picture because sprites move
// during play — a settled game still has a still background, while a title
// screen that auto-advances changes it exactly once. A ROM whose playfield never
// changes reports Found=false and Frame=0 rather than inventing a transition:
// no title is a perfectly ordinary thing for a ROM to have.
func FindGameplayStart(romPath, spec string, maxFrames, stable int) (GameplayStart, error) {
	if maxFrames < 2 {
		maxFrames = 2
	}
	if stable < 1 {
		stable = 8
	}
	e, err := emu.New(spec)
	if err != nil {
		return GameplayStart{}, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return GameplayStart{}, fmt.Errorf("load %s: %w", romPath, err)
	}
	// The first frames are the machine booting: RAM is still being cleared and the
	// ROM has not drawn its picture yet, so their playfield differs from every
	// later one. Taking a baseline there reports a transition on EVERY cartridge,
	// including ones with no title at all — measured, before this settle existed.
	const bootSettle = 2
	if err := e.RunFrames(bootSettle); err != nil {
		return GameplayStart{}, err
	}
	sigs := make([]string, 0, maxFrames)
	for f := 0; f < maxFrames; f++ {
		if err := e.RunFrames(1); err != nil {
			return GameplayStart{}, err
		}
		sigs = append(sigs, pfSignature(e))
	}
	out := GameplayStart{StableFor: stable, Searched: maxFrames}
	first := sigs[0]
	for i := 1; i < len(sigs); i++ {
		if sigs[i] == first {
			continue
		}
		out.Changed = true
		// Require the new picture to hold still, so a one-frame flash is not
		// mistaken for the game starting.
		end := i + stable
		if end > len(sigs) {
			break
		}
		steady := true
		for j := i + 1; j < end; j++ {
			if sigs[j] != sigs[i] {
				steady = false
				break
			}
		}
		if steady {
			out.Frame, out.Found = i+bootSettle, true
			return out, nil
		}
	}
	return out, nil
}

// pfSignature is a compact description of which cells the playfield drew.
func pfSignature(e *emu.Emu) string {
	img, top := e.Snapshot()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var sb strings.Builder
	for y := 0; y < h; y++ {
		runs, _, err := e.DecomposeRow(top + y)
		if err != nil {
			continue
		}
		for _, r := range runs {
			if r.Element == "PF" {
				fmt.Fprintf(&sb, "%d:%d-%d;", y, r.Clock, r.Clock+r.Len)
			}
		}
	}
	_ = w
	return sb.String()
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
