package guidedfuzz

import "github.com/kidsnz/atari2600-harness/internal/emu"

// EmuEvaluator は入力列を headless emu で先頭から走らせ、踏んだカバレッジ標識を返す Evaluator を作る。
// emu は決定論的なので、同じ入力列は常に同じ標識集合になる（再現可能）。各評価は新しい emu で
// ROM をロードし直す＝状態は入力列だけで決まる（スナップショット不要）。
func EmuEvaluator(spec, romPath string, warmup, player int) Evaluator {
	return func(seq []Action) (map[uint64]bool, error) {
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
		for _, a := range seq {
			if err := e.SetInput(player, a.Name, a.Pressed); err != nil {
				return nil, err
			}
			if err := e.RunFrames(1); err != nil {
				return nil, err
			}
		}
		if len(seq) == 0 { // 入力ゼロでも初期カバレッジを得るため最低 1 フレーム走らせる
			if err := e.RunFrames(1); err != nil {
				return nil, err
			}
		}
		return e.Coverage().Signature(), nil
	}
}
