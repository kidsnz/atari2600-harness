package mutate

import "testing"

// TestEvalRandomKillsSome は smoke.bin にランダム故障を注入すると、smoke シナリオ
// （ram.0x80==66 ＋ 262 ライン）が少なくとも一部を捕まえる（kill 率 > 0）こと、
// かつ同一 seed が決定論的（再現可能）であることを確認する。
func TestEvalRandomKillsSome(t *testing.T) {
	t.Chdir("../..")
	scen := []string{"roms/litmus/scenarios/smoke.json"}
	rs1, err := EvalRandom("roms/litmus/smoke.bin", 30, 1, scen)
	if err != nil {
		t.Fatal(err)
	}
	if KillRate(rs1) <= 0 {
		t.Fatalf("expected the suite to catch at least one mutation, kill rate = 0")
	}
	// 決定論: 同一 seed は同一結果。
	rs2, err := EvalRandom("roms/litmus/smoke.bin", 30, 1, scen)
	if err != nil {
		t.Fatal(err)
	}
	if KillRate(rs1) != KillRate(rs2) {
		t.Fatalf("mutation not deterministic: %.3f vs %.3f", KillRate(rs1), KillRate(rs2))
	}
}
