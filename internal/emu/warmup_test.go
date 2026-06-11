package emu

import "testing"

// warmupStable は電源投入過渡を抜けるまでフレームを進める（安定フレーム=260行以上 が2連続で完了）。
//
// 背景: Gopher2600 の電源投入直後の数フレームは、TV 同期の安定化過程がプラットフォーム/プロセスに
// 敏感で、CI(linux) でのみ稀に「短い再同期フレーム」(例: 26行) が観測された（macOS では 40 回反復でも
// 再現せず）。scenario runner は warmup 後に digest をリセットして過渡を除外しており一度も flake して
// いない——同じ教訓を unit テストに適用する。固定の RunFrames(2) で「フレーム3は安定」と仮定しない。
func warmupStable(t *testing.T, e *Emu) {
	t.Helper()
	stable := 0
	for i := 0; i < 15; i++ {
		lines, err := e.StepFrame()
		if err != nil {
			t.Fatal(err)
		}
		if lines >= 260 && lines <= 320 {
			stable++
			if stable >= 2 {
				return
			}
		} else {
			stable = 0
		}
	}
	t.Fatalf("TV did not stabilise within 15 frames after power-on")
}
