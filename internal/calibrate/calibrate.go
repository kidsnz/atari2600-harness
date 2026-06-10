// Package calibrate は横位置の式 X(N) の傾き・オフセットを「掃引して実測フィット」で求める（B-4）。
//
// 任意 ROM では RESPx を打つ位置が kernel 依存なので、遅延量を RAM から読む協調 ROM
// （roms/litmus/litmus_pos.bin: DELAY=$80, SBC/BCS ループ=5 CPU サイクル/ユニット）を使い、
// ハーネスが poke で DELAY を掃引 → read_tia の ResetPixel を実測 → 直線回帰する。
// litmus を「一度きりの手作業」から「kernel ごとに再現可能」へ（出典: docs/improvement-roadmap B-4）。
package calibrate

import (
	"fmt"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// Point は 1 計測（遅延ユニット DELAY と、その結果の player0 ResetPixel）。
type Point struct {
	Delay int `json:"delay"`
	X     int `json:"x"`
}

// Result は掃引の回帰結果。
type Result struct {
	Points        []Point `json:"points"`
	CyclesPerUnit int     `json:"cycles_per_unit"`  // 遅延 1 ユニットあたりの CPU サイクル（litmus_pos は 5）
	SlopePerUnit  float64 `json:"slope_per_unit"`   // ΔX / ΔDELAY（px）。litmus_pos では 15 を期待
	SlopePerCycle float64 `json:"slope_per_cycle"`  // px / CPU サイクル。実機権威値 = 3
	InterceptX    float64 `json:"intercept_x"`      // unwrap 後 DELAY=0 での外挿 X
	R2            float64 `json:"r2"`               // 当てはまりの良さ（完全直線なら 1）
}

// Sweep は協調 ROM の遅延セル(delayAddr)を lo..hi で掃引し、各フレームの player0 ResetPixel を測る。
// 事前に e は ROM ロード済みであること。
func Sweep(e *emu.Emu, delayAddr uint16, lo, hi int) ([]Point, error) {
	if lo > hi {
		lo, hi = hi, lo
	}
	if err := e.RunFrames(2); err != nil { // 起動安定
		return nil, err
	}
	pts := make([]Point, 0, hi-lo+1)
	for d := lo; d <= hi; d++ {
		if err := e.Poke(delayAddr, uint8(d)); err != nil {
			return nil, err
		}
		if err := e.RunFrames(1); err != nil { // この遅延で位置決めされたフレームを描く
			return nil, err
		}
		x := e.VCS.TIA.Video.Player0.ResetPixel // HMOVE 不使用なので ResetPixel == HmovedPixel
		pts = append(pts, Point{Delay: d, X: x})
	}
	return pts, nil
}

// modDelta は X の前進量を 160 の折返しを畳んで返す（0..159）。通常ステップも折返しステップも
// 同じ前進量になる（例: 147→2 は (2-147+160)=15）。一方、左端飽和（…→3→3）は ~0 になる。
func modDelta(a, b int) int { return ((b-a)%160 + 160) % 160 }

func medianInt(v []int) int {
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[len(s)/2]
}

// Fit は (DELAY, X) 点列を直線回帰する。折返し（160）と飽和（strobe が有効域外で左端に張り付く）に
// 頑健にするため、まず mod-160 デルタの中央値で 1 ユニットあたりの前進量を推定し、その前進量を保つ
// **最長の連続区間**だけを unwrap して最小二乗する（飽和点を除外）。
func Fit(pts []Point, cyclesPerUnit int) (Result, error) {
	if len(pts) < 2 {
		return Result{}, fmt.Errorf("need >= 2 points, got %d", len(pts))
	}
	if cyclesPerUnit <= 0 {
		return Result{}, fmt.Errorf("cyclesPerUnit must be > 0")
	}

	// 1) 隣接 mod-160 デルタの中央値 = 期待ステップ（飽和点は少数なので中央値で弾かれる）。
	deltas := make([]int, len(pts)-1)
	for i := 0; i+1 < len(pts); i++ {
		deltas[i] = modDelta(pts[i].X, pts[i+1].X)
	}
	step := medianInt(deltas)
	if step == 0 {
		return Result{}, fmt.Errorf("no horizontal movement across sweep (saturated?)")
	}

	// 2) デルタが step に一致する最長連続区間 [bestLo, bestHi]（点インデックス）。
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

	// 3) 区間を unwrap して最小二乗。
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
