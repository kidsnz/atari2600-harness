package guidedfuzz

import (
	"fmt"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// EmuSnapshotEvaluator は EmuEvaluator と同じ意味の Evaluator を、**ROM を毎回ロードし直さずに**
// 作る。ウォームアップ後の状態を 1 度だけスナップショットしておき、評価ごとにそこへ復元する。
//
// なぜ要るか（実測 2026-07-23, roms/litmus/motion_glide.bin, 30 評価の平均）:
//
//	warmup=0   reload 版 6.1ms   /評価
//	warmup=60  reload 版 189.7ms /評価
//	warmup=200 reload 版 625.9ms /評価
//	          snapshot 版 6.2ms /評価  ← warmup に依存しない
//
// タイトル画面を抜けてからでないと面白い挙動が始まらないゲーム（= 商用 ROM のほとんど）を
// fuzz するとき、reload 版は毎評価ウォームアップを焼き直すため warmup に比例して遅くなる。
// warmup=200 では **約 100 倍**の差になる。
//
// 決定性は保つ: RestoreState は RAM・ビーム座標・サイクル計数・フレームバッファまで戻し、
// 同一スナップショットは何度でも復元できる（emu の TestRestoreDeterministicAndReusable）。
// ただし Coverage は「加算されるだけの記録器」で復元されないので、復元のたびに Reset して
// 1 評価ぶんの標識だけを取る。等価性は TestSnapshotEvaluatorMatchesReload で担保する。
func EmuSnapshotEvaluator(spec, romPath string, warmup, player int) (Evaluator, error) {
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	if warmup > 0 {
		if err := e.RunFrames(warmup); err != nil {
			return nil, err
		}
	}
	e.EnableCoverage()
	start := e.SaveState()

	return func(seq []Action) (map[uint64]bool, error) {
		if err := e.RestoreState(start); err != nil {
			return nil, fmt.Errorf("snapshot evaluator: %w", err)
		}
		e.Coverage().Reset() // 前の評価の標識を持ち越さない

		for _, a := range seq {
			if err := e.SetInput(player, a.Name, a.Pressed); err != nil {
				return nil, err
			}
			if err := e.RunFrames(1); err != nil {
				return nil, err
			}
		}
		if len(seq) == 0 { // reload 版と同じ約束: 入力ゼロでも 1 フレームは走らせる
			if err := e.RunFrames(1); err != nil {
				return nil, err
			}
		}
		return e.Coverage().Signature(), nil
	}, nil
}
