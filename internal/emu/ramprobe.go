package emu

import (
	"fmt"
	"image"
	"sort"
)

// --- RAM semantics probe (C1) ---
//
// Measures, by brute force, "if I rewrite $XX, what on the screen moves and how". A tool
// for pulling the meaning of RAM (is this byte a sprite's X coordinate / its Y coordinate /
// its appearance / does it never reach the screen) out of a commercial ROM with no source,
// **by measurement rather than by guessing**. The entry point to the casebook (learning by
// reverse-analysing commercial games).
//
// Provenance: the method itself is a generalisation of play.py in kisonecat/deep-atari
// (which uses ALE's setRAM to hit Freeway's $B3 = chicken Y and $CD..$D7 = the car X of the
// 6 lanes, then looks at the screen).
// https://github.com/kisonecat/deep-atari (2022-02-15, a personal repository, not
// GPL-family; method only)
//
// Procedure (SaveState/RestoreState exist, so this is **completely non-destructive**: after
// the call the machine is back where it started):
//
//	S0 = SaveState()
//	base = restore(S0) → advance frames → screen
//	each addr, each value: restore(S0) → poke(addr,value) → advance frames → diff the screen against base
//	restore(S0)
//
// The verdict is "how far the centroid of the changed pixels travelled against the probe
// value". For an X-coordinate byte the centroid drifts sideways, for a Y-coordinate byte it
// drifts vertically. The centroid is the centroid of the CHANGED REGION (old position ∪ new
// position), so it is not the object's absolute position itself, but it **moves
// monotonically with value**, which is enough to tell a position byte apart. Every sample's
// numbers are returned, so you can check the raw data instead of trusting the
// classification.

// ProbeOptions is the input to probe_ram_semantics.
type ProbeOptions struct {
	Addrs  []uint16 // empty = all of $80..$FF
	Values []uint8  // empty = DefaultProbeValues
	Frames int      // frames to advance after the poke (default DefaultProbeFrames)
	MinPix int      // fewer changed pixels than this is noise, reported as "none" (default 1)
}

// DefaultProbeValues is the default set of probe values. It steps 0..255 at roughly even
// intervals (a coordinate byte then travels the screen end to end, so the centroid shows a
// slope).
var DefaultProbeValues = []uint8{0x00, 0x30, 0x60, 0x90, 0xC0, 0xF0}

// DefaultProbeFrames is the default number of frames to advance after the poke.
// It is 3 rather than 1 by measurement: Fishing Derby's score at $BD/$BE is drawn only after
// being converted from BCD into digit graphics, so at frames=1 it never reaches the screen
// (affected=0), at frames=2 one of the two does, and **at frames=3 both** were detected
// (matching the two addresses ALE's RomSettings records as my_score/oppt_score). Rather than
// misjudge "does not show in 1 frame = no effect", the default is the one that misses
// nothing even though it is 3x slower.
var DefaultProbeFrames = 3

// ProbeSample is one measured point for one (addr,value).
type ProbeSample struct {
	Value   uint8   `json:"value"`
	Changed int     `json:"changed_pixels"`
	BBox    [4]int  `json:"bbox"` // x0,y0,x1,y1 (bounding box of the changed pixels; all 0 when nothing changed)
	CX      float64 `json:"centroid_x"`
	CY      float64 `json:"centroid_y"`
}

// ProbeResult is the conclusion for one address.
type ProbeResult struct {
	Addr       uint16        `json:"addr"`
	AddrHex    string        `json:"addr_hex"`
	Class      string        `json:"class"` // x_position | y_position | appearance | none
	MaxChanged int           `json:"max_changed_pixels"`
	SpanX      float64       `json:"centroid_span_x"` // swing of centroid X (max−min across the probe values)
	SpanY      float64       `json:"centroid_span_y"`
	Samples    []ProbeSample `json:"samples"`
}

// ProbeRAMSemantics brute-forces addrs×values and measures the effect each address has on
// the screen. Afterwards the emulator is back in the state it was in at the call
// (non-destructive).
func (e *Emu) ProbeRAMSemantics(opt ProbeOptions) ([]ProbeResult, error) {
	addrs := opt.Addrs
	if len(addrs) == 0 {
		for a := uint16(0x80); a <= 0xFF; a++ {
			addrs = append(addrs, a)
		}
	}
	values := opt.Values
	if len(values) == 0 {
		values = DefaultProbeValues
	}
	frames := opt.Frames
	if frames <= 0 {
		frames = DefaultProbeFrames
	}
	minPix := opt.MinPix
	if minPix <= 0 {
		minPix = 1
	}

	s0 := e.SaveState()
	defer e.RestoreState(s0) // return to the starting state whatever happens

	if err := e.RestoreState(s0); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, fmt.Errorf("probe baseline: %w", err)
	}
	base, _ := e.Snapshot()

	out := make([]ProbeResult, 0, len(addrs))
	for _, a := range addrs {
		if a < 0x80 || a > 0xFF {
			return nil, fmt.Errorf("probe: address $%04X outside user RAM $80-$FF", a)
		}
		r := ProbeResult{Addr: a, AddrHex: fmt.Sprintf("$%02X", a), Class: "none"}
		minCX, maxCX := 0.0, 0.0
		minCY, maxCY := 0.0, 0.0
		seen := false

		for _, v := range values {
			if err := e.RestoreState(s0); err != nil {
				return nil, err
			}
			if err := e.Poke(a, v); err != nil {
				return nil, fmt.Errorf("probe poke $%02X: %w", a, err)
			}
			if err := e.RunFrames(frames); err != nil {
				return nil, fmt.Errorf("probe run $%02X=$%02X: %w", a, v, err)
			}
			img, _ := e.Snapshot()
			s := diffSample(base, img)
			s.Value = v
			r.Samples = append(r.Samples, s)

			if s.Changed < minPix {
				continue
			}
			if s.Changed > r.MaxChanged {
				r.MaxChanged = s.Changed
			}
			if !seen {
				minCX, maxCX, minCY, maxCY, seen = s.CX, s.CX, s.CY, s.CY, true
				continue
			}
			minCX, maxCX = minf(minCX, s.CX), maxf(maxCX, s.CX)
			minCY, maxCY = minf(minCY, s.CY), maxf(maxCY, s.CY)
		}

		if seen {
			r.SpanX, r.SpanY = maxCX-minCX, maxCY-minCY
			r.Class = classify(r.SpanX, r.SpanY)
		}
		out = append(out, r)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].MaxChanged > out[j].MaxChanged })
	return out, e.RestoreState(s0)
}

// classify infers the role from how the centroid swings. Basis for the threshold: a 2600
// sprite is 8px wide, so a centroid that travels 8px or more means "the same picture
// appeared somewhere else" = it can be taken as a position byte. The axis is asserted only
// when one axis moved at least 2x as far as the other; otherwise it does not commit to
// calling it a position ("appearance").
func classify(spanX, spanY float64) string {
	const move = 8.0
	switch {
	case spanX >= move && spanX > 2*spanY:
		return "x_position"
	case spanY >= move && spanY > 2*spanX:
		return "y_position"
	default:
		return "appearance"
	}
}

// diffSample counts the pixels that differ between two frames and returns the bounding box
// and the centroid.
func diffSample(a, b *image.RGBA) ProbeSample {
	var s ProbeSample
	w := a.Bounds().Dx()
	h := a.Bounds().Dy()
	if b.Bounds().Dx() != w || b.Bounds().Dy() != h {
		return s // resolution changed (frame spec changed) = not comparable. Treated as 0 changes
	}
	x0, y0, x1, y1 := w, h, -1, -1
	var sx, sy float64
	for y := 0; y < h; y++ {
		ra := a.PixOffset(a.Bounds().Min.X, a.Bounds().Min.Y+y)
		rb := b.PixOffset(b.Bounds().Min.X, b.Bounds().Min.Y+y)
		for x := 0; x < w; x++ {
			i, j := ra+x*4, rb+x*4
			if a.Pix[i] == b.Pix[j] && a.Pix[i+1] == b.Pix[j+1] && a.Pix[i+2] == b.Pix[j+2] {
				continue
			}
			s.Changed++
			sx += float64(x)
			sy += float64(y)
			if x < x0 {
				x0 = x
			}
			if x > x1 {
				x1 = x
			}
			if y < y0 {
				y0 = y
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if s.Changed == 0 {
		return s
	}
	s.BBox = [4]int{x0, y0, x1, y1}
	s.CX = sx / float64(s.Changed)
	s.CY = sy / float64(s.Changed)
	return s
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
