package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		got  int64
		op   string
		want int64
		exp  bool
	}{
		{5, "==", 5, true}, {5, "==", 6, false},
		{5, "!=", 6, true}, {5, "!=", 5, false},
		{5, "<", 6, true}, {6, "<", 5, false},
		{5, "<=", 5, true}, {6, "<=", 5, false},
		{6, ">", 5, true}, {5, ">", 6, false},
		{5, ">=", 5, true}, {4, ">=", 5, false},
	}
	for _, c := range cases {
		got, err := compare(c.got, c.op, c.want)
		if err != nil {
			t.Fatalf("compare(%d,%q,%d) err: %v", c.got, c.op, c.want, err)
		}
		if got != c.exp {
			t.Errorf("compare(%d,%q,%d) = %v, want %v", c.got, c.op, c.want, got, c.exp)
		}
	}
	if _, err := compare(1, "~=", 1); err == nil {
		t.Errorf("unknown op should error")
	}
}

// TestCondPassIn は範囲演算子 in（[lo,hi] 内包）を確認する。
func TestCondPassIn(t *testing.T) {
	cases := []struct {
		got, lo, hi int64
		exp         bool
	}{
		{5, 0, 10, true}, {5, 6, 10, false}, {10, 0, 10, true}, {0, 0, 10, true}, {11, 0, 10, false},
	}
	for _, c := range cases {
		got, err := condPass(c.got, "in", 0, c.lo, c.hi)
		if err != nil {
			t.Fatalf("condPass in err: %v", err)
		}
		if got != c.exp {
			t.Errorf("%d in [%d,%d] = %v, want %v", c.got, c.lo, c.hi, got, c.exp)
		}
	}
}

// TestInvariantsMonotonic は毎フレーム監視（invariants / monotonic / range）の陽性・陰性を確認する。
// smoke.bin は ram.0x80 が定数 66、frame は毎フレーム単調増＝決定的な break ケースになる。
func TestInvariantsMonotonic(t *testing.T) {
	t.Chdir("../..")
	// 陽性: 定数の invariant・範囲 assert・frame の単調増。
	pass := &Scenario{
		Rom:    "roms/litmus/smoke.bin",
		Frames: 5,
		Asserts: []Assert{
			{AtFrame: 0, Field: "ram.0x80", Op: "in", Lo: 60, Hi: 70}, // 66 ∈ [60,70]
		},
		Invariants: []Invariant{
			{Field: "ram.0x80", Op: "==", Value: 66},
			{Field: "ram.0x80", Op: "in", Lo: 60, Hi: 70},
		},
		Monotonic: []Monotonic{{Field: "frame", Direction: "up"}},
	}
	res, err := Run(pass, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		for _, a := range res.Asserts {
			if !a.Pass {
				t.Errorf("unexpected fail: %s (got %d)", a.Desc, a.Got)
			}
		}
	}

	// 陰性: 破れる invariant ＋ 単調増の frame に対する "down"。各 break は1回だけ記録される。
	fail := &Scenario{
		Rom:        "roms/litmus/smoke.bin",
		Frames:     5,
		Invariants: []Invariant{{Field: "ram.0x80", Op: "==", Value: 1}}, // 66≠1 → break
		Monotonic:  []Monotonic{{Field: "frame", Direction: "down"}},     // frame↑ → break
	}
	res2, err := Run(fail, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Pass {
		t.Fatalf("expected fail")
	}
	breaks := 0
	for _, a := range res2.Asserts {
		if !a.Pass {
			breaks++
		}
	}
	if breaks != 2 { // 1 invariant + 1 monotonic、各々最初の破れだけ
		t.Fatalf("expected exactly 2 recorded breaks, got %d: %+v", breaks, res2.Asserts)
	}
}

// TestRunSamples は同梱サンプルシナリオが実 ROM で全 pass することを確認する（陽性）。
// ROM パスはリポジトリルート相対なので、テストはルートへ chdir して CLI と同じ前提で走らせる。
func TestRunSamples(t *testing.T) {
	t.Chdir("../..")
	for _, f := range []string{
		"roms/litmus/scenarios/smoke.json",
		"roms/litmus/scenarios/smoke_src.json",
		"roms/litmus/scenarios/collide.json",
		"roms/litmus/scenarios/golden.json",
	} {
		t.Run(filepath.Base(f), func(t *testing.T) {
			s, err := Load(f)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Run(s, false)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Pass {
				for _, a := range res.Asserts {
					if !a.Pass {
						t.Errorf("FAIL %s (got %d)", a.Desc, a.Got)
					}
				}
			}
		})
	}
}

// TestRunAsmSource は rom に .asm を指定すると実行前にアセンブルされ、ソース 1 枚から合否まで
// 走ること（欠落E のビルドループ短縮）を確認する。
func TestRunAsmSource(t *testing.T) {
	t.Chdir("../..")
	s := &Scenario{
		Rom:     "roms/litmus/smoke.asm",
		Asserts: []Assert{{AtFrame: 0, Field: "ram.0x80", Op: "==", Value: 66}},
	}
	res, err := Run(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("asm-source scenario failed: %+v", res.Asserts)
	}
}

// TestRunAsmSourceError は壊れた .asm がアセンブル段でエラーになる（握り潰さない）ことを確認する。
func TestRunAsmSourceError(t *testing.T) {
	t.Chdir("../..")
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.asm")
	if err := os.WriteFile(bad, []byte("\tprocessor 6502\n\tlda bogus_undefined_label_xyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Scenario{Rom: bad}
	if _, err := Run(s, false); err == nil {
		t.Fatalf("expected assemble error for broken .asm, got nil")
	}
}

// TestRunDetectsFailure は誤ったアサーションが fail として検出され Result.Pass=false になることを確認する（陰性）。
func TestRunDetectsFailure(t *testing.T) {
	t.Chdir("../..")
	s := &Scenario{
		Rom: "roms/litmus/smoke.bin",
		Asserts: []Assert{
			{AtFrame: 0, Field: "ram.0x80", Op: "==", Value: 66}, // 真
			{AtFrame: 0, Field: "ram.0x80", Op: "==", Value: 1},  // 偽（$42≠1）
		},
	}
	res, err := Run(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pass {
		t.Fatalf("expected overall fail, got pass")
	}
	if len(res.Asserts) != 2 || !res.Asserts[0].Pass || res.Asserts[1].Pass {
		t.Fatalf("expected [pass, fail], got %+v", res.Asserts)
	}
}

// TestUnknownFieldErrors は未知フィールドを握り潰さずエラーにすることを確認する。
func TestUnknownFieldErrors(t *testing.T) {
	t.Chdir("../..")
	s := &Scenario{
		Rom:     "roms/litmus/smoke.bin",
		Asserts: []Assert{{AtFrame: 0, Field: "ram", Op: "==", Value: 0}}, // ram.<addr> 欠落
	}
	if _, err := Run(s, false); err == nil {
		t.Fatalf("expected error for malformed field, got nil")
	}
	s2 := &Scenario{
		Rom:     "roms/litmus/smoke.bin",
		Asserts: []Assert{{AtFrame: 0, Field: "bogus.field", Op: "==", Value: 0}},
	}
	if _, err := Run(s2, false); err == nil {
		t.Fatalf("expected error for unknown field, got nil")
	}
}

// TestGoldenDeterministic は golden_frame の描画連鎖ハッシュが同一 ROM・同一入力で再現することを確認する。
func TestGoldenDeterministic(t *testing.T) {
	t.Chdir("../..")
	mk := func() *Scenario {
		return &Scenario{
			Rom:          "roms/litmus/smoke.bin",
			WarmupFrames: 2,
			Asserts:      []Assert{{AtFrame: 2, Field: "ram.0x80", Op: "==", Value: 66}},
			Checks:       &Checks{GoldenFrame: true},
		}
	}
	r1, err := Run(mk(), false)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Run(mk(), false)
	if err != nil {
		t.Fatal(err)
	}
	if r1.GoldenHash == "" {
		t.Fatalf("golden hash empty")
	}
	if r1.GoldenHash != r2.GoldenHash {
		t.Fatalf("golden hash not deterministic: %s vs %s", r1.GoldenHash, r2.GoldenHash)
	}
}

// TestGoldenSampleMatches は同梱の golden.json が committed の .golden と一致する（描画回帰なし）ことを確認する。
func TestGoldenSampleMatches(t *testing.T) {
	t.Chdir("../..")
	s, err := Load("roms/litmus/scenarios/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("golden sample did not match committed baseline (hash=%s)", res.GoldenHash)
	}
}

// TestGoldenDetectsMismatch は基準ハッシュが違えば fail することを確認する（陰性）。
func TestGoldenDetectsMismatch(t *testing.T) {
	t.Chdir("../..")
	dir := t.TempDir()
	sp := filepath.Join(dir, "x.json")
	if err := os.WriteFile(goldenPath(sp), []byte("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Scenario{
		Rom:          "roms/litmus/smoke.bin",
		WarmupFrames: 2,
		Asserts:      []Assert{{AtFrame: 2, Field: "ram.0x80", Op: "==", Value: 66}},
		Checks:       &Checks{GoldenFrame: true},
		srcPath:      sp,
	}
	res, err := Run(s, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pass {
		t.Fatalf("expected golden mismatch to fail, got pass (hash=%s)", res.GoldenHash)
	}
}
