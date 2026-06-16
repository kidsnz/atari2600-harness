package refdiff

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestWallsFromRow は壁抽出ロジック（背景=最広 run、両端の細い非背景 run 2本）を確認する。
func TestWallsFromRow(t *testing.T) {
	// 自作レイアウト: 端2px黒 → 左壁(2..9) → 黒 → 右壁(152..159)。
	mine := []emu.RowRun{
		{Clock: 0, Len: 2, Hex: "060606"},
		{Clock: 2, Len: 8, Hex: "888888"},
		{Clock: 10, Len: 142, Hex: "060606"},
		{Clock: 152, Len: 8, Hex: "888888"},
	}
	if l, r, ok := wallsFromRow(mine, 160); !ok || l != 2 || r != 159 {
		t.Errorf("mine: got (%d,%d,%v), want (2,159,true)", l, r, ok)
	}

	// 原版レイアウト: 左壁が clock0 から（端ピッタリ）。
	orig := []emu.RowRun{
		{Clock: 0, Len: 8, Hex: "888888"},
		{Clock: 8, Len: 144, Hex: "060606"},
		{Clock: 152, Len: 8, Hex: "888888"},
	}
	if l, r, ok := wallsFromRow(orig, 160); !ok || l != 0 || r != 159 {
		t.Errorf("orig: got (%d,%d,%v), want (0,159,true)", l, r, ok)
	}

	// 全面ブロック行など（端の壁2本でない）は壁ラインとして拒否。
	notWalls := []emu.RowRun{
		{Clock: 0, Len: 2, Hex: "060606"},
		{Clock: 2, Len: 8, Hex: "888888"},
		{Clock: 10, Len: 60, Hex: "A9A81D"},
		{Clock: 70, Len: 82, Hex: "060606"},
		{Clock: 152, Len: 8, Hex: "888888"},
	}
	if _, _, ok := wallsFromRow(notWalls, 160); ok {
		t.Errorf("a row with a brick run should not be taken as a clean wall line")
	}
}

// TestCompare は特徴一致/不一致の判定を確認する。
func TestCompare(t *testing.T) {
	got := Fingerprint{LeftWallClock: 2, RightWallEnd: 159, BallWidthClocks: 1}
	want := Fingerprint{LeftWallClock: 0, RightWallEnd: 159, BallWidthClocks: 2}
	ds := Compare(got, want)
	if AllMatch(ds) {
		t.Fatal("expected mismatch")
	}
	if Compare(want, want); !AllMatch(Compare(want, want)) {
		t.Fatal("identical fingerprints should match")
	}
}
