package guidedfuzz

import "testing"

// staircase は「深くガードされた状態」を模した合成オラクル：入力列が secret と先頭から一致する
// 長さ k に応じて標識 0..k を公開する（正解を 1 つ進めるごとに新標識が 1 つ開く）。AFL 流の
// フィードバックが効くかを ROM 非依存・決定論で検証できる。
func staircase(secret []Action) Evaluator {
	return func(seq []Action) (map[uint64]bool, error) {
		k := 0
		for k < len(secret) && k < len(seq) && seq[k] == secret[k] {
			k++
		}
		cov := map[uint64]bool{}
		for d := 0; d <= k; d++ {
			cov[uint64(d)] = true
		}
		return cov, nil
	}
}

// TestGuidedBeatsBlind は guided 探索が深いガード状態へ到達でき、blind が同じ予算では到達
// できないこと（VV-3 の核心主張）を固定する。決定論的（Seed 固定）。
func TestGuidedBeatsBlind(t *testing.T) {
	pool := []string{"up", "down", "left", "right"}
	const depth = 8
	secret := make([]Action, depth)
	for j := range secret {
		secret[j] = Action{Name: pool[j%len(pool)], Pressed: j%2 == 0}
	}
	cfg := Config{Seed: 1, Iterations: 6000, MaxLen: 14, Actions: pool}

	guided, err := RunGuided(cfg, staircase(secret))
	if err != nil {
		t.Fatal(err)
	}
	blind, err := RunBlind(cfg, staircase(secret))
	if err != nil {
		t.Fatal(err)
	}

	// guided は全段（標識 0..depth = depth+1 個）に到達する。
	if guided.Markers != depth+1 {
		t.Fatalf("guided reached %d markers, want full depth %d (corpus=%d)", guided.Markers, depth+1, guided.CorpusSize)
	}
	// blind は同予算では浅い段で頭打ち（guided に届かない）。
	if blind.Markers >= guided.Markers {
		t.Fatalf("blind reached %d markers >= guided %d — guidance gave no advantage", blind.Markers, guided.Markers)
	}
	if len(guided.Best) < depth {
		t.Fatalf("guided.Best too short: %d (< depth %d)", len(guided.Best), depth)
	}
}

// TestEmuGuidedRuns は実機(emu)アダプタ越しに guided 探索が回り、標識を見つけることを確認
// （配線の end-to-end・非フレーキー）。
func TestEmuGuidedRuns(t *testing.T) {
	eval := EmuEvaluator("NTSC", "../../roms/litmus/smoke.bin", 2, 0)
	cfg := Config{Seed: 7, Iterations: 60, MaxLen: 6, Actions: []string{"fire", "left", "right"}}
	res, err := RunGuided(cfg, eval)
	if err != nil {
		t.Fatal(err)
	}
	if res.Markers == 0 {
		t.Fatalf("expected to discover coverage markers through the emu, got 0")
	}
}
