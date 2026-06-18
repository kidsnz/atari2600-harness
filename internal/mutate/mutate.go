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

// Mutation は ROM の 1 バイトを Value に書き換える故障注入。
type Mutation struct {
	Offset int
	Value  byte
}

// Result は 1 つの mutant の評価結果。
type Result struct {
	Mutation Mutation
	OrigByte byte
	Killed   bool   // scenario 集合の少なくとも1つが fail / 実行不能（＝検査が捕まえた）
	By       string // killed したシナリオのパス（survived なら空）
}

// writeMutant は base をコピーして Offset のバイトを Value にした一時ファイルを作る。
func writeMutant(base []byte, m Mutation, dir string) (string, error) {
	b := make([]byte, len(base))
	copy(b, base)
	b[m.Offset] = m.Value
	p := filepath.Join(dir, fmt.Sprintf("mutant_%d_%02x.bin", m.Offset, m.Value))
	return p, os.WriteFile(p, b, 0o644)
}

// EvalOne は 1 つの mutation を全シナリオに評価し、どれかが fail/実行不能なら killed とする。
// scenario の Rom は mutant の .bin に差し替える（元が .asm でもコンパイル後を変異させられる）。
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
		if runErr != nil { // mutant が実行不能＝検出された（kill）
			res.Killed, res.By = true, sp+" (run error)"
			return res, nil
		}
		if !r.Pass { // どれかのアサーションが落ちた＝kill
			res.Killed, res.By = true, sp
			return res, nil
		}
	}
	return res, nil
}

// EvalRandom は seed から count 個のランダム mutation（既存値と異なるバイト）を生成・評価する。
// 戻りの Result 列から kill 率＝検査スイートの強さが分かる。
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
		if val == base[off] { // 必ず実際に変化させる
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

// CoveredOffsets は romPath を frames フレーム実行（PC カバレッジ有効）し、実際に命令を
// 実行した ROM ファイルオフセットの集合を返す（VV-11）。アドレス→オフセットは ROM 長の
// マスク（2の冪）で写す。命令フェッチは必ず ROM 空間なので安全。
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
	mask := len(base) - 1
	out := map[int]bool{}
	for _, pc := range cov.SeenPCs() {
		out[int(pc)&mask] = true
	}
	return out, nil
}

// EvalRandomCovered は coverage-filtered mutation testing（VV-11）。EvalRandom と違い、
// 故障注入を「実際に実行されたコード」のオフセットだけに限定する＝**正直な kill 率**。
// 一度も実行されないコード（デッドパディング等）の mutant は原理上 kill 不能で、ナイーブな
// kill 率を不当に下げる。frames は covered 集合を作る基準実行のフレーム数。
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

// KillRate は Result 列の kill 率（0.0〜1.0）。
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
