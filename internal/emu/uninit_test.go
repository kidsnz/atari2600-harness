package emu

import "testing"

// TestUninitReadDetector は VV-10 T-3 の falsifiable 自己テスト：
//   - clean ツイン（indexed クリアループで全 RAM を書いてから読む）は NO hit
//     （実効アドレス追跡が indexed 書き込みを取りこぼさない＝誤検出ゼロ）。
//   - trap（クリアせず未初期化 RAM を読む）は HIT。
func TestUninitReadDetector(t *testing.T) {
	run := func(rom string) *UninitReadHit {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		hit, err := e.WatchUninitRead(2)
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}

	if hit := run("../../roms/litmus/uninit_clean.bin"); hit != nil {
		t.Fatalf("clean kernel must NOT be flagged, got read of 0x%04X @pc 0x%04X (indexed-clear false positive?)", hit.Addr, hit.PC)
	}
	if hit := run("../../roms/litmus/uninit_trap.bin"); hit == nil {
		t.Fatalf("trap (reads uninitialized RAM) must be flagged, got no hit")
	} else if hit.Addr != 0x0090 {
		t.Fatalf("expected the uninit read at 0x0090, got 0x%04X", hit.Addr)
	}
}
