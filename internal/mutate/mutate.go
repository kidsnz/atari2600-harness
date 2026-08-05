// Package mutate implements mutation testing for ROM scenario suites: inject a
// fault into the ROM (flip a byte) and confirm the scenario suite catches it
// (= mutant killed). A surviving mutant means the checks are too weak to detect
// that regression. This grades the *tests*, not the ROM. docs/testing-playbook.md.
// Source: DeMillo/Lipton/Sayward 1978; Offutt.
package mutate

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/scenario"
)

// Mutation is a fault injection that rewrites one ROM byte to Value.
type Mutation struct {
	Offset int
	Value  byte
}

// Result is the evaluation result of one mutant.
type Result struct {
	Mutation Mutation
	OrigByte byte
	Killed   bool   // at least one of the scenario set failed / could not run (= the checks caught it)
	By       string // path of the scenario that killed it (empty if survived)
}

// writeMutant copies base and writes a temp file with the byte at Offset set to Value.
func writeMutant(base []byte, m Mutation, dir string) (string, error) {
	b := make([]byte, len(base))
	copy(b, base)
	b[m.Offset] = m.Value
	p := filepath.Join(dir, fmt.Sprintf("mutant_%d_%02x.bin", m.Offset, m.Value))
	return p, os.WriteFile(p, b, 0o644)
}

// EvalOne evaluates one mutation against all scenarios; killed if any of them fails or cannot
// run. The scenario's Rom is swapped for the mutant's .bin (so even if the original is a .asm,
// the post-assembly image can be mutated).
func EvalOne(base []byte, m Mutation, scenarioPaths []string, dir string) (Result, error) {
	res := Result{Mutation: m, OrigByte: base[m.Offset]}
	mp, err := writeMutant(base, m, dir)
	if err != nil {
		return res, err
	}
	for _, sp := range scenarioPaths {
		s, err := scenario.Load(sp)
		if err != nil {
			return res, fmt.Errorf("load %s: %w", sp, err)
		}
		s.Rom = mp
		r, runErr := scenario.Run(s, false)
		if runErr != nil { // mutant could not run = detected (kill)
			res.Killed, res.By = true, sp+" (run error)"
			return res, nil
		}
		if !r.Pass { // some assertion failed = kill
			res.Killed, res.By = true, sp
			return res, nil
		}
	}
	return res, nil
}

// EvalRandom generates and evaluates count random mutations from seed (bytes differing from the
// existing value). The returned Result slice yields the kill rate = the strength of the check suite.
func EvalRandom(romPath string, count int, seed int64, scenarioPaths []string) ([]Result, error) {
	base, err := os.ReadFile(romPath)
	if err != nil {
		return nil, err
	}
	if len(base) == 0 {
		return nil, fmt.Errorf("empty rom %s", romPath)
	}
	dir, err := os.MkdirTemp("", "mutate")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	rng := rand.New(rand.NewSource(seed))
	out := make([]Result, 0, count)
	for i := 0; i < count; i++ {
		off := rng.Intn(len(base))
		val := byte(rng.Intn(256))
		if val == base[off] { // always force an actual change
			val ^= 0xFF
		}
		r, err := EvalOne(base, Mutation{Offset: off, Value: val}, scenarioPaths, dir)
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// CoveredOffsets runs romPath for frames frames (with PC coverage enabled) and returns the set
// of ROM file offsets where instructions actually executed (VV-11). Address → offset is mapped
// via a mask of the ROM length (a power of two). Instruction fetches are always in ROM space, so
// this is safe.
func CoveredOffsets(romPath, spec string, frames int) (map[int]bool, error) {
	base, err := os.ReadFile(romPath)
	if err != nil {
		return nil, err
	}
	if len(base) == 0 {
		return nil, fmt.Errorf("empty rom %s", romPath)
	}
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	e.EnableCoverage()
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	cov := e.Coverage()
	out := map[int]bool{}
	// The file offset of an executed instruction is bank*4K + its offset in the
	// window — NOT `addr & (len(rom)-1)`, which folds every $Fxxx into the LAST 4K
	// image no matter which bank ran. Measured 2026-07-30 with the old fold: on
	// exerciser all 278 covered offsets landed in $1000-$1FFF and not one in
	// $0000-$0FFF, so "restrict fault injection to code that actually executed"
	// was injecting into bank 1's bytes while bank 0 was the half that ran. A 4K
	// image is the one-bank case of this and is unaffected.
	for _, site := range cov.SeenSites() {
		bank, addr := site[0], site[1]
		off := bank*0x1000 + (addr & 0x0FFF)
		if off < len(base) {
			out[off] = true
		}
	}
	return out, nil
}

// EvalRandomCovered is coverage-filtered mutation testing (VV-11). Unlike EvalRandom, it
// restricts fault injection to offsets of "code that actually executed" = the **honest kill
// rate**. A mutant in code that never runs (dead padding etc.) is unkillable in principle and
// unfairly drags down the naive kill rate. frames is the frame count of the baseline run that
// builds the covered set.
func EvalRandomCovered(romPath string, count int, seed int64, scenarioPaths []string, frames int) ([]Result, error) {
	base, err := os.ReadFile(romPath)
	if err != nil {
		return nil, err
	}
	covered, err := CoveredOffsets(romPath, "NTSC", frames)
	if err != nil {
		return nil, err
	}
	if len(covered) == 0 {
		return nil, fmt.Errorf("no covered code in %s over %d frames", romPath, frames)
	}
	offs := make([]int, 0, len(covered))
	for o := range covered {
		offs = append(offs, o)
	}
	sort.Ints(offs)

	dir, err := os.MkdirTemp("", "mutate")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	rng := rand.New(rand.NewSource(seed))
	out := make([]Result, 0, count)
	for i := 0; i < count; i++ {
		off := offs[rng.Intn(len(offs))]
		val := byte(rng.Intn(256))
		if val == base[off] {
			val ^= 0xFF
		}
		r, err := EvalOne(base, Mutation{Offset: off, Value: val}, scenarioPaths, dir)
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// KillRate is the kill rate (0.0〜1.0) of a Result slice.
func KillRate(rs []Result) float64 {
	if len(rs) == 0 {
		return 0
	}
	k := 0
	for _, r := range rs {
		if r.Killed {
			k++
		}
	}
	return float64(k) / float64(len(rs))
}
