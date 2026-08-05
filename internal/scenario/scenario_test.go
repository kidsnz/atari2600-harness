package scenario

import (
	"os"
	"path/filepath"
	"strings"
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

// TestFuzz は seed 付き乱数入力で走らせても invariant が保たれ、クラッシュせず、
// かつ決定論的（同一 seed → 同一結果）であることを確認する。
func TestFuzz(t *testing.T) {
	t.Chdir("../..")
	mk := func() *Scenario {
		return &Scenario{
			Rom:        "roms/litmus/smoke.bin",
			Fuzz:       &Fuzz{Seed: 42, Frames: 30, Actions: []string{"left", "right", "fire"}},
			Invariants: []Invariant{{Field: "ram.0x80", Op: "==", Value: 66}},
		}
	}
	r1, err := Run(mk(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Pass {
		for _, a := range r1.Asserts {
			if !a.Pass {
				t.Errorf("fuzz unexpectedly failed: %s (got %d)", a.Desc, a.Got)
			}
		}
	}
	// 決定論: 同一 seed の再走は同一の合否（再現可能 replay の土台）。
	r2, err := Run(mk(), false)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Pass != r2.Pass || len(r1.Asserts) != len(r2.Asserts) {
		t.Fatalf("fuzz not deterministic: %+v vs %+v", r1.Asserts, r2.Asserts)
	}
}

// TestFuzzNeedsActions は空の actions プールがエラーになることを確認する。
func TestFuzzNeedsActions(t *testing.T) {
	t.Chdir("../..")
	s := &Scenario{Rom: "roms/litmus/smoke.bin", Fuzz: &Fuzz{Seed: 1, Frames: 5}}
	if _, err := Run(s, false); err == nil {
		t.Fatalf("expected error for empty fuzz actions")
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
		"roms/litmus/scenarios/beamrace_clean.json",
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

// TestBeamRaceScenario locks the no_beam_race check (AT-3) both directions through
// the scenario engine: the clean fixture (P0 updated in HBLANK) passes; the late
// fixture (P0 graphics written deep into the visible line) fails.
func TestBeamRaceScenario(t *testing.T) {
	t.Chdir("../..")
	clean := &Scenario{
		Rom:    "roms/litmus/beamrace_clean.asm",
		Checks: &Checks{NoBeamRace: &BeamRaceCheck{Object: "P0", LineFrom: 40, LineTo: 54, Frames: 1, Warmup: 2}},
	}
	if res, err := Run(clean, false); err != nil {
		t.Fatal(err)
	} else if !res.Pass {
		t.Errorf("clean fixture must pass no_beam_race: %+v", res.Asserts)
	}
	late := &Scenario{
		Rom:    "roms/litmus/beamrace_late.asm",
		Checks: &Checks{NoBeamRace: &BeamRaceCheck{Object: "P0", LineFrom: 40, LineTo: 54, Frames: 1, Warmup: 2}},
	}
	if res, err := Run(late, false); err != nil {
		t.Fatal(err)
	} else if res.Pass {
		t.Errorf("late fixture must FAIL no_beam_race: %+v", res.Asserts)
	}
}

// TestFrameLinesStable locks frame_lines_stable in both directions against the
// framelines_clean / framelines_trap pair, which differ by ONE instruction: a `sta
// WSYNC` the trap spends on every 128th frame, putting that frame at 263 lines.
//
// The check originally used real ROMs — a sweep of all 156 in this repo found the fault
// already present in 4 of them — but every one of those was then repaired, and a gate
// whose only witness has been fixed is a gate that certifies nothing. Hence a fixture
// pair that stays broken on purpose.
//
// The third case is the one that matters most: the SAME trap ROM PASSES over a 60-frame
// window, because its period is 128. A stability gate certifies only the frames it
// measured, and pinning that here is what stops someone later "fixing" a red check by
// shrinking the window. The real defect this stands in for had a 120-frame period.
func TestFrameLinesStable(t *testing.T) {
	t.Chdir("../..")

	// Negative control: the twin without that one instruction must not trip the check.
	stable := &Scenario{
		Rom:    "roms/litmus/framelines_clean.asm",
		Checks: &Checks{FrameLinesStable: &FrameLinesStableCheck{Frames: 130, Lines: 262}},
	}
	if res, err := Run(stable, false); err != nil {
		t.Fatal(err)
	} else if !res.Pass {
		t.Errorf("the clean twin must pass frame_lines_stable: %+v", res.Asserts)
	}

	// Positive control: a window that contains the trap's 128-frame period.
	breathing := &Scenario{
		Rom:    "roms/litmus/framelines_trap.asm",
		Checks: &Checks{FrameLinesStable: &FrameLinesStableCheck{Frames: 130}},
	}
	res, err := Run(breathing, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pass {
		t.Errorf("framelines_trap spends an extra line every 128th frame and must FAIL: %+v", res.Asserts)
	}
	if got := res.Asserts[len(res.Asserts)-1].Got; got != 2 {
		t.Errorf("got = distinct line counts: want 2 (262 and 263), got %d", got)
	}

	// A window shorter than the ROM's period passes the same ROM. This is the vacuity
	// the doc comment warns about, measured rather than asserted.
	short := &Scenario{
		Rom:    "roms/litmus/framelines_trap.asm",
		Checks: &Checks{FrameLinesStable: &FrameLinesStableCheck{Frames: 60}},
	}
	if res, err := Run(short, false); err != nil {
		t.Fatal(err)
	} else if !res.Pass {
		t.Errorf("a 60-frame window cannot reach the 128-frame trap, so it must pass: %+v", res.Asserts)
	}

	// Stable at the wrong number is still a failure when `lines` is declared.
	wrong := &Scenario{
		Rom:    "roms/litmus/framelines_clean.asm",
		Checks: &Checks{FrameLinesStable: &FrameLinesStableCheck{Frames: 20, Lines: 261}},
	}
	if res, err := Run(wrong, false); err != nil {
		t.Fatal(err)
	} else if res.Pass {
		t.Errorf("stable at 262 must FAIL a declared lines:261: %+v", res.Asserts)
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

// TestEvalTemporal は時相判定ロジックを純関数として planted トレースで検証する（VV-5 の
// falsifiable 自己テスト）。pass / fail / inconclusive を 3 演算子すべてで固定する。フレーム
// base やエミュに依存しないので決定的。
func TestEvalTemporal(t *testing.T) {
	tt := func(kind, field string, within, n int) Temporal {
		return Temporal{Kind: kind, Field: field, Op: "==", Value: 1, Within: within, N: n}
	}
	cases := []struct {
		name string
		mon  Temporal
		p    []bool
		a    []bool
		want bool // 期待 pass
		incl bool // desc に INCONCLUSIVE を含むべきか
	}{
		// eventually: 窓内に成立=pass / 窓を観測しても不成立=fail / 窓未観測=inconclusive。
		{"eventually-holds", tt("eventually", "x", 5, 0), []bool{false, false, true, false, false}, nil, true, false},
		{"eventually-misses", tt("eventually", "x", 5, 0), []bool{false, false, false, false, false}, nil, false, false},
		{"eventually-too-tight", tt("eventually", "x", 2, 0), []bool{false, false, true}, nil, false, false},
		{"eventually-inconclusive", tt("eventually", "x", 5, 0), []bool{false, false, false}, nil, false, true},
		// never_for: N 連続で成立=fail / 未満=pass。
		{"never-violated", tt("never_for", "x", 0, 3), []bool{true, true, true}, nil, false, false},
		{"never-ok", tt("never_for", "x", 0, 4), []bool{true, true, true}, nil, true, false},
		{"never-ok-broken-run", tt("never_for", "x", 0, 3), []bool{true, true, false, true, true}, nil, true, false},
		// response: 期限内に応答=pass / 期限切れ=fail / 窓が run 終了をまたぐ=inconclusive。
		{"response-ok", Temporal{Kind: "response", Field: "x", Op: "==", Value: 1, AField: "a", AOp: "==", AValue: 1, Within: 2},
			[]bool{false, false, false, true, false}, []bool{false, true, false, false, false}, true, false},
		{"response-too-tight", Temporal{Kind: "response", Field: "x", Op: "==", Value: 1, AField: "a", AOp: "==", AValue: 1, Within: 1},
			[]bool{false, false, false, true, false}, []bool{false, true, false, false, false}, false, false},
		{"response-inconclusive", Temporal{Kind: "response", Field: "x", Op: "==", Value: 1, AField: "a", AOp: "==", AValue: 1, Within: 3},
			[]bool{false, false, false, false, false}, []bool{false, false, false, false, true}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			desc, pass := evalTemporal(c.mon, c.p, c.a)
			if pass != c.want {
				t.Fatalf("pass=%v want %v (%s)", pass, c.want, desc)
			}
			incl := strings.Contains(desc, "INCONCLUSIVE")
			if incl != c.incl {
				t.Fatalf("inconclusive=%v want %v (%s)", incl, c.incl, desc)
			}
		})
	}
}

// TestTemporalThroughRun は resolve→観測→evalTemporal の配線を smoke.bin で end-to-end 検証する。
// ram.0x80 は定数 66（base 非依存）なので always-true / never-true の命題を作れる。
func TestTemporalThroughRun(t *testing.T) {
	t.Chdir("../..")

	// 陽性: 即成立する eventually ＋ 決して成立しない never_for。
	pass := &Scenario{
		Rom:    "roms/litmus/smoke.bin",
		Frames: 6,
		Temporal: []Temporal{
			{Kind: "eventually", Field: "ram.0x80", Op: "==", Value: 66, Within: 3},
			{Kind: "never_for", Field: "ram.0x80", Op: "==", Value: 999, N: 3},
		},
	}
	res, err := Run(pass, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got: %+v", res.Asserts)
	}

	// 陰性: 窓を完全に観測しても決して成立しない eventually。
	fail := &Scenario{
		Rom:      "roms/litmus/smoke.bin",
		Frames:   6,
		Temporal: []Temporal{{Kind: "eventually", Field: "ram.0x80", Op: "==", Value: 999, Within: 3}},
	}
	if res, err := Run(fail, false); err != nil {
		t.Fatal(err)
	} else if res.Pass {
		t.Fatalf("expected fail, got pass: %+v", res.Asserts)
	}

	// inconclusive ≠ 緑: 窓 (within 10) が run (3 フレーム) より長い→未確定で pass にしない。
	incl := &Scenario{
		Rom:      "roms/litmus/smoke.bin",
		Frames:   3,
		Temporal: []Temporal{{Kind: "eventually", Field: "ram.0x80", Op: "==", Value: 999, Within: 10}},
	}
	res3, err := Run(incl, false)
	if err != nil {
		t.Fatal(err)
	}
	if res3.Pass {
		t.Fatalf("inconclusive must not be a vacuous pass: %+v", res3.Asserts)
	}
	if !strings.Contains(res3.Asserts[len(res3.Asserts)-1].Desc, "INCONCLUSIVE") {
		t.Fatalf("expected INCONCLUSIVE in desc, got: %+v", res3.Asserts)
	}

	// 不正な定義は run 前に拒否。
	bad := &Scenario{Rom: "roms/litmus/smoke.bin", Frames: 3,
		Temporal: []Temporal{{Kind: "eventually", Field: "ram.0x80", Op: "==", Value: 1, Within: 0}}}
	if _, err := Run(bad, false); err == nil {
		t.Fatalf("expected error for within<=0")
	}
}
