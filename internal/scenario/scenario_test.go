package scenario

import (
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

// TestRunSamples は同梱サンプルシナリオが実 ROM で全 pass することを確認する（陽性）。
// ROM パスはリポジトリルート相対なので、テストはルートへ chdir して CLI と同じ前提で走らせる。
func TestRunSamples(t *testing.T) {
	t.Chdir("../..")
	for _, f := range []string{
		"roms/litmus/scenarios/smoke.json",
		"roms/litmus/scenarios/collide.json",
		"roms/frogger/scenarios/boot.json",
		"roms/frogger/scenarios/hop.json",
	} {
		t.Run(filepath.Base(f), func(t *testing.T) {
			s, err := Load(f)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Run(s)
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
	res, err := Run(s)
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
	if _, err := Run(s); err == nil {
		t.Fatalf("expected error for malformed field, got nil")
	}
	s2 := &Scenario{
		Rom:     "roms/litmus/smoke.bin",
		Asserts: []Assert{{AtFrame: 0, Field: "bogus.field", Op: "==", Value: 0}},
	}
	if _, err := Run(s2); err == nil {
		t.Fatalf("expected error for unknown field, got nil")
	}
}
