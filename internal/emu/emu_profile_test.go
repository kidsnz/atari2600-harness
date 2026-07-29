package emu

import "testing"

// TestProfileLineWorstFindsHeavyLine は litmus_overrun（1本だけ ~100cy の重い可視ラインを
// 仕込んだ ROM）で、ProfileLineWorst が (a) その行を最ワーストとして先頭に返し
// (b) worst_cycles が 76 超・2 物理ライン消費を「厳密な cy 数」で報告することを裏取りする。
// RunUntilBudget の lineCycles=152（消費ライン×76 の近似）と違い、こちらは実測値そのもの。
func TestProfileLineWorstFindsHeavyLine(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_overrun.bin"); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no WSYNC intervals profiled")
	}
	top := rows[0]
	if top.WorstCycles <= 76 {
		t.Fatalf("worst row = %dcy, want >76 (the planted heavy line)", top.WorstCycles)
	}
	if top.WorstCycles >= 152 {
		t.Fatalf("worst row = %dcy — looks like the lines*76 approximation, want the EXACT cycle count (<152)", top.WorstCycles)
	}
	if top.WorstLines != 2 {
		t.Fatalf("worst row consumed %d physical lines, want 2", top.WorstLines)
	}
	if top.Count < 2 {
		t.Fatalf("worst row measured %d times over 3 frames, want >=2", top.Count)
	}
	// 残りの行は全部 1 ライン内（このROMの他ラインは規律正しい）。
	for _, r := range rows[1:] {
		if r.WorstCycles > 76 {
			t.Fatalf("unexpected second over-budget row %dcy at $%04X", r.WorstCycles, r.StrobePC)
		}
	}
}

// TestProfileLineWorstNoHiddenWarmup は RunUntilBudget と同じ warmup 規律
// （走行中 Frame>=2 なら余計なフレームを消費しない＝poke 状態を食い潰さない）の回帰ロック。
func TestProfileLineWorstNoHiddenWarmup(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pos.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(5); err != nil {
		t.Fatal(err)
	}
	start := e.Coords().Frame
	if _, _, err := e.ProfileLineWorst(3, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.Coords().Frame - start; got != 3 {
		t.Fatalf("frames consumed = %d, want exactly 3 (no hidden warmup when already running)", got)
	}
}

// TestProfileLineWorstWatch は watch スナップショットが「ワースト区間の開き strobe 時点」
// の RAM 値を返すことを smoke する（litmus_pos の ZP を1本 watch）。
func TestProfileLineWorstWatch(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_overrun.bin"); err != nil {
		t.Fatal(err)
	}
	rows, _, err := e.ProfileLineWorst(3, []uint16{0x80, 0x81})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	for _, r := range rows {
		if len(r.Watch) != 2 {
			t.Fatalf("watch snapshot len = %d, want 2", len(r.Watch))
		}
	}
}
