package emu

import "testing"

// TestCoverageLogic は記録ロジックを純データで検証する（VV-3 の falsifiable 自己テスト）。
// 片側分岐の判定と「踏んでいないアドレス＝未踏」を固定する。決定的・ROM 非依存。
func TestCoverageLogic(t *testing.T) {
	c := newCoverage()
	c.record(0, 0x0100, false, false) // 非分岐
	c.record(0, 0x0200, true, true)   // 分岐: taken だけ → 片側
	c.record(0, 0x0300, true, false)  // 分岐: fall-through
	c.record(0, 0x0300, true, true)   // 同じ分岐の taken も → 両側踏破

	if c.PCCount() != 3 {
		t.Fatalf("PCCount=%d want 3", c.PCCount())
	}
	if c.BranchCount() != 2 {
		t.Fatalf("BranchCount=%d want 2", c.BranchCount())
	}
	if c.EdgeCount() != 3 { // {0x200 taken, 0x300 taken, 0x300 not}
		t.Fatalf("EdgeCount=%d want 3", c.EdgeCount())
	}
	if !c.Seen(0x0100) {
		t.Fatalf("0x0100 should be seen")
	}
	if c.Seen(0x0999) { // planted: 記録していないアドレスは未踏
		t.Fatalf("0x0999 was never recorded, must read as uncovered")
	}
	os := c.OneSidedBranches()
	if len(os) != 1 || os[0] != 0x0200 {
		t.Fatalf("OneSidedBranches=%v want [0x0200] (0x0300 is two-sided)", os)
	}
}

// TestCoverageThroughRun は stepInstr→record の配線を smoke.bin で end-to-end 検証する。
func TestCoverageThroughRun(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/smoke.bin"); err != nil {
		t.Fatal(err)
	}
	e.EnableCoverage()
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	cov := e.Coverage()
	if cov == nil || cov.PCCount() == 0 {
		t.Fatalf("expected non-empty coverage after running, got %+v", cov)
	}
	if cov.Seen(0x0000) { // RAM/データ域＝命令として実行されない＝未踏のはず
		t.Fatalf("0x0000 is not an executed instruction address; must be uncovered")
	}
	// 健全性: 踏んだエッジ数は分岐数の 2 倍を超えない。片側分岐は分岐集合の部分集合。
	if cov.EdgeCount() > 2*cov.BranchCount() {
		t.Fatalf("EdgeCount %d > 2*BranchCount %d", cov.EdgeCount(), cov.BranchCount())
	}
}
