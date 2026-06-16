package mine

import "testing"

// TestMineSmoke は smoke.bin（ram.0x80 が定数 66、frame が単調増）から、
// 期待どおりの候補不変条件が出ることを確認する。
func TestMineSmoke(t *testing.T) {
	t.Chdir("../..")
	cands, err := Mine("roms/litmus/smoke.bin", 20, 1, nil, 0, []string{"ram.0x80", "frame"})
	if err != nil {
		t.Fatal(err)
	}
	byField := map[string]Candidate{}
	for _, c := range cands {
		byField[c.Field] = c
	}
	if c := byField["ram.0x80"]; c.Kind != "const" || c.Value != 66 {
		t.Errorf("ram.0x80: want const 66, got %+v", c)
	}
	if c := byField["frame"]; c.Kind != "monotonic-up" {
		t.Errorf("frame: want monotonic-up, got %+v", c)
	}
}
