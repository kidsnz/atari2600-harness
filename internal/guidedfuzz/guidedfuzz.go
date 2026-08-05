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

// Action is the input given for one frame (action name and pressed state).
type Action struct {
	Name    string
	Pressed bool
}

// Evaluator runs an input sequence from the start and returns the set of coverage markers (comparable keys) it stepped on.
type Evaluator func(seq []Action) (map[uint64]bool, error)

// Config is the search configuration. Deterministic (reproducible for a fixed Seed).
type Config struct {
	Seed       int64
	Iterations int      // number of mutation attempts
	MaxLen     int      // maximum input-sequence length (= upper bound on frames run)
	Actions    []string // pool of actions mutations pick from
}

// Result is the outcome of a search.
type Result struct {
	Iterations int      // number of attempts executed
	CorpusSize int      // number of input sequences kept as interesting
	Markers    int      // total number of coverage markers discovered
	Best       []Action // the input sequence that produced the most markers (reproducible)
}

func clone(s []Action) []Action {
	c := make([]Action, len(s))
	copy(c, s)
	return c
}

// mutate derives a child sequence from a parent (append at the end / replace an action / flip pressed).
func (cfg Config) mutate(rng *rand.Rand, parent []Action) []Action {
	child := clone(parent)
	pick := func() Action {
		return Action{Name: cfg.Actions[rng.Intn(len(cfg.Actions))], Pressed: rng.Intn(2) == 1}
	}
	switch {
	case len(child) == 0 || (len(child) < cfg.MaxLen && rng.Intn(2) == 0):
		child = append(child, pick()) // append at the end (extend the sequence)
	default:
		child[rng.Intn(len(child))] = pick() // replace one existing frame
	}
	return child
}

// RunGuided performs the coverage-guided search. Only children that add new markers are kept in the corpus.
func RunGuided(cfg Config, eval Evaluator) (Result, error) {
	rng := rand.New(rand.NewSource(cfg.Seed))
	global := map[uint64]bool{}
	corpus := [][]Action{{}} // start from the empty sequence (deterministic seed)

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

// RunBlind is the baseline for comparison: random search with no corpus feedback.
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
