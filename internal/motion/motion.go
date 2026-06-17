// Package motion turns "does this object judder / ブルブル between frames" into
// numbers. For a TIA object it tracks, over N frames, the exact horizontal
// position (HmovedPixel) and the rendered vertical top (scanned from the frame),
// then reports velocity (1st difference), acceleration (2nd difference), and the
// RMS of the 2nd difference as a smoothness score: a constant-velocity glide
// scores 0; a stutter or a one-frame snap-back scores high. This automates the
// hand frame-by-frame trace we did on the Breakout ball.
//
// Provenance: smoothness as (the integral of) squared jerk — Flash & Hogan,
// "The coordination of arm movements", J. Neurosci. 1985 (minimum-jerk model);
// here the discrete analogue = RMS of the 2nd difference of position.
package motion

import (
	"fmt"
	"image/color"
	"math"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Stats summarizes the smoothness of one 1-D position track.
type Stats struct {
	Positions []int   `json:"positions"`
	Velocity  []int   `json:"velocity"`  // 1st difference (len = n-1)
	Accel     []int   `json:"accel"`     // 2nd difference (len = n-2)
	JerkRMS   float64 `json:"jerk_rms"`  // RMS of the 2nd difference; 0 = constant velocity
	MaxStep   int     `json:"max_step"`  // max |velocity| (largest single-frame move)
	MaxAccel  int     `json:"max_accel"` // max |accel| (largest velocity change = a snap)
	Monotonic bool    `json:"monotonic"` // velocity never changes sign (no back-and-forth)
}

// Analyze computes the smoothness stats of a position sequence.
func Analyze(pos []int) Stats {
	s := Stats{Positions: pos}
	for i := 1; i < len(pos); i++ {
		s.Velocity = append(s.Velocity, pos[i]-pos[i-1])
	}
	for i := 1; i < len(s.Velocity); i++ {
		s.Accel = append(s.Accel, s.Velocity[i]-s.Velocity[i-1])
	}
	for _, v := range s.Velocity {
		if a := iabs(v); a > s.MaxStep {
			s.MaxStep = a
		}
	}
	var sumsq float64
	for _, a := range s.Accel {
		if aa := iabs(a); aa > s.MaxAccel {
			s.MaxAccel = aa
		}
		sumsq += float64(a) * float64(a)
	}
	if len(s.Accel) > 0 {
		s.JerkRMS = math.Sqrt(sumsq / float64(len(s.Accel)))
	}
	s.Monotonic = monotonic(s.Velocity)
	return s
}

func monotonic(v []int) bool {
	sign := 0
	for _, x := range v {
		switch {
		case x > 0:
			if sign < 0 {
				return false
			}
			sign = 1
		case x < 0:
			if sign > 0 {
				return false
			}
			sign = -1
		}
	}
	return true
}

func iabs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Track is the result of following one object over N frames.
type Track struct {
	Object  string `json:"object"`
	Frames  int    `json:"frames"`
	X       Stats  `json:"x"`      // horizontal HmovedPixel (exact, no vision)
	Top     Stats  `json:"top"`    // rendered top scanline (vision)
	Height  []int  `json:"height"` // rendered height per frame (px)
	Missing int    `json:"missing"` // frames where the object was not found in the window
}

var objects = []string{"P0", "M0", "P1", "M1", "BL"}

func markerIndex(object string) int {
	for i, o := range objects {
		if o == object {
			return i
		}
	}
	return -1
}

// TrackObject steps the (already-loaded, already-positioned) emulator N frames,
// recording the object's exact X (HmovedPixel) and its rendered vertical top +
// height each frame, then analyzes both axes. [yTop,yBot] bound the scanline
// search window (grid-y, same coordinate as ReadRow / the annotated grid). The
// rendered top = the topmost scanline in the window whose colour at the object's
// X column differs from that column's background (its most common colour over the
// window). Robust over a uniform background; over the textured playfield the
// vision Top is unreliable — use the exact X axis there.
func TrackObject(e *emu.Emu, object string, frames, yTop, yBot int) (Track, error) {
	idx := markerIndex(object)
	if idx < 0 {
		return Track{}, fmt.Errorf("unknown object %q (want one of P0 M0 P1 M1 BL)", object)
	}
	if frames < 3 {
		return Track{}, fmt.Errorf("need at least 3 frames to form a 2nd difference, got %d", frames)
	}
	t := Track{Object: object, Frames: frames}
	xs := make([]int, 0, frames)
	tops := make([]int, 0, frames)
	for f := 0; f < frames; f++ {
		if _, err := e.StepFrame(); err != nil {
			return Track{}, err
		}
		x := e.Markers()[idx].Clock
		xs = append(xs, x)
		top, h, ok := renderedTop(e, x, yTop, yBot)
		if !ok {
			t.Missing++
			if len(tops) > 0 { // carry last known so the series stays aligned
				top = tops[len(tops)-1]
			} else {
				top = yTop
			}
			h = 0
		}
		tops = append(tops, top)
		t.Height = append(t.Height, h)
	}
	t.X = Analyze(xs)
	t.Top = Analyze(tops)
	return t, nil
}

// renderedTop finds the object's topmost rendered scanline at column x by scanning
// one frame snapshot. Background = the most common colour in column x over the
// window; top = the first scanline whose colour differs; height = the contiguous
// run from there.
func renderedTop(e *emu.Emu, x, yTop, yBot int) (top, height int, ok bool) {
	img, visTop := e.Snapshot()
	b := img.Bounds()
	if x < 0 || x >= b.Dx() {
		return 0, 0, false
	}
	type sample struct {
		sl int
		c  color.RGBA
	}
	var col []sample
	count := map[color.RGBA]int{}
	for sl := yTop; sl <= yBot; sl++ {
		row := sl - visTop
		if row < 0 || row >= b.Dy() {
			continue
		}
		c := img.RGBAAt(x, row)
		col = append(col, sample{sl, c})
		count[c]++
	}
	if len(col) == 0 {
		return 0, 0, false
	}
	var bg color.RGBA
	best := -1
	for c, n := range count {
		if n > best {
			best, bg = n, c
		}
	}
	for i, p := range col {
		if p.c != bg {
			top, height = p.sl, 1
			for j := i + 1; j < len(col) && col[j].c != bg; j++ {
				height++
			}
			return top, height, true
		}
	}
	return 0, 0, false
}
