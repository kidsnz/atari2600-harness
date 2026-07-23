package guidedfuzz

import (
	"testing"
)

const fuzzROM = "../../roms/litmus/litmus_input.bin"

func sigEqual(a, b map[uint64]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// スナップショット版は reload 版と **同じ標識集合** を返さなければ意味がない
// （速さのために忠実さを落としていないことの証明）。入力列を変えて総当たりで突き合わせる。
func TestSnapshotEvaluatorMatchesReload(t *testing.T) {
	seqs := [][]Action{
		{},
		{{Name: "fire", Pressed: true}},
		{{Name: "fire", Pressed: true}, {Name: "fire", Pressed: false}},
		{{Name: "left", Pressed: true}, {Name: "left", Pressed: false}, {Name: "fire", Pressed: true}},
		{{Name: "right", Pressed: true}, {Name: "up", Pressed: true}, {Name: "down", Pressed: true}},
	}
	for _, warmup := range []int{0, 3, 40} {
		reload := EmuEvaluator("NTSC", fuzzROM, warmup, 0)
		snap, err := EmuSnapshotEvaluator("NTSC", fuzzROM, warmup, 0)
		if err != nil {
			t.Skipf("ROM unavailable: %v", err)
		}
		for i, seq := range seqs {
			want, err := reload(seq)
			if err != nil {
				t.Fatal(err)
			}
			got, err := snap(seq)
			if err != nil {
				t.Fatal(err)
			}
			if !sigEqual(want, got) {
				t.Errorf("warmup=%d seq#%d: snapshot signature differs (reload=%d markers, snapshot=%d)",
					warmup, i, len(want), len(got))
			}
		}
	}
}

// 復元のたびに Coverage を切らないと標識が累積して「どの入力でも同じ（全部入り）」になる。
// 同じ評価器で違う入力を順に流しても互いに汚染しないことを確かめる。
func TestSnapshotEvaluatorDoesNotAccumulate(t *testing.T) {
	// litmus_input はどの入力でも同じ 36 標識しか踏まないので、この検査には歯が立たない
	// （Reset を消しても通ってしまうことを実測で確認済み）。標識数が入力で実際に変わる
	// motion_stutter を使い、その前提自体もテスト内で検査する。
	const leakROM = "../../roms/litmus/motion_stutter.bin"
	snap, err := EmuSnapshotEvaluator("NTSC", leakROM, 3, 0)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	long := []Action{{Name: "fire", Pressed: true}, {Name: "left", Pressed: true},
		{Name: "right", Pressed: true}, {Name: "up", Pressed: true}, {Name: "down", Pressed: true}}
	short := []Action{{Name: "fire", Pressed: true}}

	first, err := snap(short)
	if err != nil {
		t.Fatal(err)
	}
	big, err := snap(long) // 汚染源: 広く踏む入力を挟む
	if err != nil {
		t.Fatal(err)
	}
	if len(big) <= len(first) { // 前提: 長い入力の方が標識が多くないと汚染を検出できない
		t.Fatalf("precondition failed: long seq covered %d markers, short covered %d — this ROM cannot expose a leak",
			len(big), len(first))
	}
	again, err := snap(short)
	if err != nil {
		t.Fatal(err)
	}
	if !sigEqual(first, again) {
		t.Errorf("same input gave different signatures across runs (%d vs %d markers) — coverage leaked between evaluations",
			len(first), len(again))
	}
}
