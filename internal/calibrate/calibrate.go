// Package calibrate obtains the slope and offset of the horizontal-position formula X(N) by
// "sweep and fit against measurements" (B-4).
//
// In an arbitrary ROM the position where RESPx is strobed is kernel-dependent, so we use a
// cooperating ROM that reads the delay amount from RAM
// (roms/litmus/litmus_pos.bin: DELAY=$80, SBC/BCS loop = 5 CPU cycles/unit); the harness sweeps
// DELAY via poke → measures read_tia's ResetPixel → fits a line by regression.
// This turns the litmus from "a one-off manual job" into "reproducible per kernel"
// (Source: B-4, delivered, see CHANGELOG.md).
package calibrate

import (
	"fmt"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Point is one measurement (the delay-unit DELAY and the resulting player0 ResetPixel).
type Point struct {
	Delay int `json:"delay"`
	X     int `json:"x"`
}

// Result is the regression result of a sweep.
type Result struct {
	Points        []Point `json:"points"`
	CyclesPerUnit int     `json:"cycles_per_unit"` // CPU cycles per delay unit (5 for litmus_pos)
	SlopePerUnit  float64 `json:"slope_per_unit"`  // ΔX / ΔDELAY (px). Expect 15 for litmus_pos
	SlopePerCycle float64 `json:"slope_per_cycle"` // px / CPU cycle. Authoritative hardware value = 3
	InterceptX    float64 `json:"intercept_x"`     // extrapolated X at DELAY=0 after unwrapping
	R2            float64 `json:"r2"`              // goodness of fit (1 for a perfect line)
}

// Sweep sweeps the cooperating ROM's delay cell (delayAddr) over lo..hi and measures player0's
// ResetPixel for each frame.
// e must already have the ROM loaded.
func Sweep(e *emu.Emu, delayAddr uint16, lo, hi int) ([]Point, error) {
	if lo > hi {
		lo, hi = hi, lo
	}
	if err := e.RunFrames(2); err != nil { // let startup settle
		return nil, err
	}
	pts := make([]Point, 0, hi-lo+1)
	for d := lo; d <= hi; d++ {
		if err := e.Poke(delayAddr, uint8(d)); err != nil {
			return nil, err
		}
		if err := e.RunFrames(1); err != nil { // draw the frame positioned with this delay
			return nil, err
		}
		x := e.VCS.TIA.Video.Player0.ResetPixel // no HMOVE used, so ResetPixel == HmovedPixel
		pts = append(pts, Point{Delay: d, X: x})
	}
	return pts, nil
}

// modDelta returns X's forward travel with the 160-pixel wraparound folded in (0..159). A normal
// step and a wrapping step yield the same forward travel (e.g. 147→2 is (2-147+160)=15).
// Left-edge saturation (…→3→3), on the other hand, comes out ~0.
func modDelta(a, b int) int { return ((b-a)%160 + 160) % 160 }

func medianInt(v []int) int {
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[len(s)/2]
}

// Fit fits a straight line to the (DELAY, X) point series. To be robust against wraparound (160)
// and saturation (the strobe pinning to the left edge outside its valid range), it first
// estimates the per-unit forward travel from the median of the mod-160 deltas, then unwraps and
// least-squares only the **longest contiguous run** that keeps that travel (excluding saturated
// points).
func Fit(pts []Point, cyclesPerUnit int) (Result, error) {
	if len(pts) < 2 {
		return Result{}, fmt.Errorf("need >= 2 points, got %d", len(pts))
	}
	if cyclesPerUnit <= 0 {
		return Result{}, fmt.Errorf("cyclesPerUnit must be > 0")
	}

	// 1) Median of adjacent mod-160 deltas = the expected step (saturated points are few, so the median rejects them).
	deltas := make([]int, len(pts)-1)
	for i := 0; i+1 < len(pts); i++ {
		deltas[i] = modDelta(pts[i].X, pts[i+1].X)
	}
	step := medianInt(deltas)
	if step == 0 {
		return Result{}, fmt.Errorf("no horizontal movement across sweep (saturated?)")
	}

	// 2) The longest contiguous run [bestLo, bestHi] (point indices) whose deltas match step.
	const tol = 2
	bestLo, bestHi, lo := 0, 0, 0
	for i := 0; i < len(deltas); i++ {
		if abs(deltas[i]-step) <= tol {
			if i-lo > bestHi-bestLo {
				bestLo, bestHi = lo, i+1
			}
		} else {
			lo = i + 1
		}
	}
	run := pts[bestLo : bestHi+1]
	if len(run) < 2 {
		return Result{}, fmt.Errorf("no linear run found (step=%d)", step)
	}

	// 3) Unwrap the run and least-squares fit.
	ys := make([]float64, len(run))
	off := 0
	for i, p := range run {
		if i > 0 && p.X < run[i-1].X {
			off += 160
		}
		ys[i] = float64(p.X + off)
	}
	n := float64(len(run))
	var sx, sy, sxx, sxy float64
	for i, p := range run {
		x := float64(p.Delay)
		sx += x
		sy += ys[i]
		sxx += x * x
		sxy += x * ys[i]
	}
	slope := (n*sxy - sx*sy) / (n*sxx - sx*sx)
	intercept := (sy - slope*sx) / n

	meanY := sy / n
	var ssTot, ssRes float64
	for i, p := range run {
		pred := slope*float64(p.Delay) + intercept
		ssRes += (ys[i] - pred) * (ys[i] - pred)
		ssTot += (ys[i] - meanY) * (ys[i] - meanY)
	}
	r2 := 1.0
	if ssTot != 0 {
		r2 = 1 - ssRes/ssTot
	}

	return Result{
		Points:        pts,
		CyclesPerUnit: cyclesPerUnit,
		SlopePerUnit:  slope,
		SlopePerCycle: slope / float64(cyclesPerUnit),
		InterceptX:    intercept,
		R2:            r2,
	}, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
