package emu

import "testing"

// TestStepInstruction は 1 命令ずつ進める粒度を検証する。litmus_cycles は NOP(2cy)×4 + JMP(3cy) の
// ループなので、各 StepInstruction の LastCycles は 2 か 3 のいずれかになるはず。
func TestStepInstruction(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_cycles.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(1); err != nil { // 起動シーケンスを越える
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		before := e.TotalCycles()
		if err := e.StepInstruction(); err != nil {
			t.Fatal(err)
		}
		got := e.LastCycles()
		if got != 2 && got != 3 {
			t.Fatalf("step %d: LastCycles = %d, want 2 (NOP) or 3 (JMP)", i, got)
		}
		// ちょうど 1 命令ぶん累積が進む（WSYNC を使わない ROM なので stall 消化はゼロ）。
		if d := e.TotalCycles() - before; d != int64(got) {
			t.Fatalf("step %d: total advanced by %d, want %d", i, d, got)
		}
	}
}

// TestStepScanline は scanline がちょうど 1 つずつ進む（フレーム境界では 0 へ折返す）ことを検証する。
func TestStepScanline(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/smoke.bin"); err != nil {
		t.Fatal(err)
	}
	warmupStable(t, e) // 電源投入過渡を除外（CI flake 根本対策）

	var total int64
	for i := 0; i < 40; i++ {
		prev := e.Coords()
		before := e.TotalCycles()
		if err := e.StepScanline(); err != nil {
			t.Fatal(err)
		}
		cur := e.Coords()
		wrapped := cur.Frame == prev.Frame+1 && cur.Scanline == 0
		stepped := cur.Frame == prev.Frame && cur.Scanline == prev.Scanline+1
		if !wrapped && !stepped {
			t.Fatalf("step %d: scanline jumped from (f%d,s%d) to (f%d,s%d), want +1 line or frame wrap",
				i, prev.Frame, prev.Scanline, cur.Frame, cur.Scanline)
		}
		total += e.TotalCycles() - before
	}
	// 個々のラインは「純 WSYNC stall のパススルー」で 0 サイクル（命令を 1 つも実行しない）もあり得るため
	// 「毎ライン > 0」は不変条件でない（電源投入時のビーム位相で前後する）。40 ライン累積で CPU が前進している
	// ことだけを確認する＝決定性に依らず堅牢。
	if total <= 0 {
		t.Fatalf("no CPU cycles consumed across 40 scanlines (total=%d)", total)
	}
}
