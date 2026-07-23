// Command guidedfuzz runs coverage-guided (AFL-style) input search against a ROM
// and reports the coverage markers it found and the input sequence that reached
// the most (VV-3). Unlike the blind scenario `fuzz`, it keeps a corpus and grows
// it whenever a mutation reveals new coverage, so it climbs toward guarded states.
//
//	go run ./cmd/guidedfuzz -rom x.bin -iters 4000 -maxlen 16 -actions left,right,fire
//
// Prints JSON. Deterministic for a fixed -seed (the found sequence replays).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/guidedfuzz"
)

func main() {
	rom := flag.String("rom", "", "ROM under test (.bin, required)")
	iters := flag.Int("iters", 2000, "mutation iterations")
	maxlen := flag.Int("maxlen", 16, "max input-sequence length (frames driven)")
	warmup := flag.Int("warmup", 2, "frames to settle before recording")
	seed := flag.Int64("seed", 1, "RNG seed (deterministic)")
	player := flag.Int("player", 0, "player port for inputs")
	actions := flag.String("actions", "left,right,fire", "comma-separated action pool")
	blind := flag.Bool("blind", false, "run the blind baseline instead (for comparison)")
	snapshot := flag.Bool("snapshot", false, "reuse one emulator and restore a post-warmup snapshot per evaluation instead of reloading the ROM (identical signatures, ~100x faster at warmup=200; measured in internal/guidedfuzz)")
	flag.Parse()

	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: guidedfuzz -rom x.bin [-iters 2000] [-maxlen 16] [-actions left,right,fire] [-seed 1] [-blind]")
		os.Exit(2)
	}

	cfg := guidedfuzz.Config{
		Seed:       *seed,
		Iterations: *iters,
		MaxLen:     *maxlen,
		Actions:    splitTrim(*actions),
	}
	eval := guidedfuzz.EmuEvaluator("NTSC", *rom, *warmup, *player)
	if *snapshot {
		snapEval, err := guidedfuzz.EmuSnapshotEvaluator("NTSC", *rom, *warmup, *player)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		eval = snapEval
	}

	run := guidedfuzz.RunGuided
	mode := "guided"
	if *blind {
		run = guidedfuzz.RunBlind
		mode = "blind"
	}
	res, err := run(cfg, eval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	best := make([]string, len(res.Best))
	for i, a := range res.Best {
		st := "down"
		if !a.Pressed {
			st = "up"
		}
		best[i] = a.Name + ":" + st
	}
	out := struct {
		Rom        string   `json:"rom"`
		Mode       string   `json:"mode"`
		Iterations int      `json:"iterations"`
		CorpusSize int      `json:"corpus_size"`
		Markers    int      `json:"markers"`
		Best       []string `json:"best_input"`
	}{*rom, mode, res.Iterations, res.CorpusSize, res.Markers, best}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
