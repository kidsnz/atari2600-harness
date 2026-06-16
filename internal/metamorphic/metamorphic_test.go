package metamorphic

import "testing"

// TestEval は同一シナリオ間の関係（決定論 MR）で == が成立し < が違反になることを確認する。
// smoke.bin は ram.0x80 が定数 66 なので 66==66 / 66<66 が決定的。
func TestEval(t *testing.T) {
	t.Chdir("../..")
	const s = "roms/litmus/scenarios/smoke.json"

	hold, err := Eval(s, s, "ram.0x80", "==")
	if err != nil {
		t.Fatal(err)
	}
	if !hold.Pass {
		t.Fatalf("expected == to hold: %s", hold.Desc)
	}
	if hold.AVal != 66 || hold.BVal != 66 {
		t.Fatalf("metric capture wrong: %+v", hold)
	}

	viol, err := Eval(s, s, "ram.0x80", "<")
	if err != nil {
		t.Fatal(err)
	}
	if viol.Pass {
		t.Fatalf("expected < to be violated (66<66): %s", viol.Desc)
	}

	if _, err := Eval(s, s, "ram.0x80", "~~"); err == nil {
		t.Fatalf("expected error for unknown relation")
	}
}
