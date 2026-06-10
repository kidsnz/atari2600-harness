package emu

import "testing"

// TestCycleCounterMatchesBeam は read_cycles（B-1）の累積カウンタが実機の実行 CPU
// サイクルを正しく数えていることを、beam 座標との普遍則で裏取りする。
//
// litmus_cycles.bin は WSYNC を一切使わない無限ループ＝CPU が決して停止しない。
// よって命令境界では常に「実行 CPU サイクル × 3 == 進んだカラークロック数」が厳密に成立する。
// フレーム境界をまたぐと scanline がリセットされるので、フレームが変わるたびに基準点を取り直し、
// 同一フレーム内で (Δcycles)*3 == Δ(scanline*228+clock) を毎命令検証する。
func TestCycleCounterMatchesBeam(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_cycles.bin"); err != nil {
		t.Fatal(err)
	}

	// ロード直後はカウンタ 0 であること。
	if got := e.TotalCycles(); got != 0 {
		t.Fatalf("TotalCycles after load = %d, want 0", got)
	}

	// リセット直後の起動シーケンスを少し進めて安定させる。
	for i := 0; i < 2000; i++ {
		if err := e.VCS.Step(nil); err != nil {
			t.Fatal(err)
		}
		e.accumCycle()
	}

	cc := func() int { c := e.Coords(); return c.Scanline*228 + c.Clock }

	baseT := e.cpuCycles
	baseCC := cc()
	baseFrame := e.Coords().Frame

	for i := 0; i < 40000; i++ {
		if err := e.VCS.Step(nil); err != nil {
			t.Fatal(err)
		}
		e.accumCycle()

		c := e.Coords()
		if c.Frame != baseFrame {
			// フレームが変わったら基準点を取り直す。
			baseFrame = c.Frame
			baseT = e.cpuCycles
			baseCC = cc()
			continue
		}
		if dc := (e.cpuCycles - baseT) * 3; dc != int64(cc()-baseCC) {
			t.Fatalf("cycle/beam invariant broken at step %d (frame %d): cycles*3=%d, beam delta=%d",
				i, c.Frame, dc, cc()-baseCC)
		}
	}

	if e.TotalCycles() <= 0 {
		t.Fatalf("TotalCycles did not advance: %d", e.TotalCycles())
	}

	// MarkCycles / CyclesSinceMark の区間計測も確認する。
	e.MarkCycles()
	if got := e.CyclesSinceMark(); got != 0 {
		t.Fatalf("CyclesSinceMark right after MarkCycles = %d, want 0", got)
	}
	before := e.cpuCycles
	for i := 0; i < 100; i++ {
		if err := e.VCS.Step(nil); err != nil {
			t.Fatal(err)
		}
		e.accumCycle()
	}
	if got, want := e.CyclesSinceMark(), e.cpuCycles-before; got != want {
		t.Fatalf("CyclesSinceMark = %d, want %d", got, want)
	}
}
