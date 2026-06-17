// Package guidedfuzz implements coverage-guided (AFL-style) input search for the
// 2600 (VV-3). Today's scenario `fuzz` is BLIND — it throws random inputs and
// only checks invariants. Guided fuzz instead keeps a corpus of input sequences
// and grows it whenever a mutation reveals a NEW coverage marker, so it climbs
// toward deeply-guarded states a blind search would essentially never hit.
//
// The search core is decoupled from the emulator via Evaluator (input sequence
// -> coverage markers), so it can be unit-tested deterministically with a
// synthetic "staircase" oracle. EmuEvaluator wires it to the real emu.
package guidedfuzz

import "math/rand"

// Action は 1 フレームに与える入力（行動名と押下状態）。
type Action struct {
	Name    string
	Pressed bool
}

// Evaluator は入力列を先頭から実行し、踏んだカバレッジ標識（比較可能キー）の集合を返す。
type Evaluator func(seq []Action) (map[uint64]bool, error)

// Config は探索設定。決定論的（Seed 固定で再現可能）。
type Config struct {
	Seed       int64
	Iterations int      // 変異試行回数
	MaxLen     int      // 入力列の最大長（＝走らせるフレーム数の上限）
	Actions    []string // 変異で選ぶ行動プール
}

// Result は探索の結果。
type Result struct {
	Iterations int      // 実行した試行回数
	CorpusSize int      // interesting と判定して保持した入力列の数
	Markers    int      // 発見したカバレッジ標識の総数
	Best       []Action // 最大の標識数を出した入力列（再現可能）
}

func clone(s []Action) []Action {
	c := make([]Action, len(s))
	copy(c, s)
	return c
}

// mutate は親列から子列を作る（末尾追加 / 行動差し替え / 押下反転）。
func (cfg Config) mutate(rng *rand.Rand, parent []Action) []Action {
	child := clone(parent)
	pick := func() Action {
		return Action{Name: cfg.Actions[rng.Intn(len(cfg.Actions))], Pressed: rng.Intn(2) == 1}
	}
	switch {
	case len(child) == 0 || (len(child) < cfg.MaxLen && rng.Intn(2) == 0):
		child = append(child, pick()) // 末尾追加（列を伸ばす）
	default:
		child[rng.Intn(len(child))] = pick() // 既存の 1 フレームを差し替え
	}
	return child
}

// RunGuided は coverage-guided 探索を行う。新しい標識を増やした子だけを corpus に残す。
func RunGuided(cfg Config, eval Evaluator) (Result, error) {
	rng := rand.New(rand.NewSource(cfg.Seed))
	global := map[uint64]bool{}
	corpus := [][]Action{{}} // 空列から開始（決定的なシード）

	seedCov, err := eval(nil)
	if err != nil {
		return Result{}, err
	}
	mergeInto(global, seedCov)
	best := []Action(nil)
	bestN := len(seedCov)

	for i := 0; i < cfg.Iterations; i++ {
		parent := corpus[rng.Intn(len(corpus))]
		child := cfg.mutate(rng, parent)
		cov, err := eval(child)
		if err != nil {
			return Result{}, err
		}
		if addsNew(global, cov) {
			mergeInto(global, cov)
			corpus = append(corpus, child)
		}
		if len(cov) > bestN {
			bestN = len(cov)
			best = clone(child)
		}
	}
	return Result{Iterations: cfg.Iterations, CorpusSize: len(corpus), Markers: len(global), Best: best}, nil
}

// RunBlind は比較用のベースライン：corpus フィードバック無しのランダム探索。
func RunBlind(cfg Config, eval Evaluator) (Result, error) {
	rng := rand.New(rand.NewSource(cfg.Seed))
	global := map[uint64]bool{}
	best := []Action(nil)
	bestN := -1
	for i := 0; i < cfg.Iterations; i++ {
		n := rng.Intn(cfg.MaxLen + 1)
		seq := make([]Action, n)
		for j := range seq {
			seq[j] = Action{Name: cfg.Actions[rng.Intn(len(cfg.Actions))], Pressed: rng.Intn(2) == 1}
		}
		cov, err := eval(seq)
		if err != nil {
			return Result{}, err
		}
		mergeInto(global, cov)
		if len(cov) > bestN {
			bestN = len(cov)
			best = seq
		}
	}
	return Result{Iterations: cfg.Iterations, CorpusSize: 0, Markers: len(global), Best: best}, nil
}

func mergeInto(dst, src map[uint64]bool) {
	for k := range src {
		dst[k] = true
	}
}

func addsNew(global, cov map[uint64]bool) bool {
	for k := range cov {
		if !global[k] {
			return true
		}
	}
	return false
}
