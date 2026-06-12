package emu

import "testing"

// WatchRAM: exerciser の frameCt($82) は毎フレーム inc される＝1フレーム以内に変化を捕まえる。
func TestWatchRAMChange(t *testing.T) {
	e, _ := New("NTSC")
	if err := e.LoadROM("../../roms/exerciser/exerciser.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(5); err != nil {
		t.Fatal(err)
	}
	changed, oldV, newV, pc, err := e.WatchRAM(0x82, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || newV != oldV+1 {
		t.Fatalf("changed=%v old=%d new=%d", changed, oldV, newV)
	}
	if pc == 0 {
		t.Fatal("pc not captured")
	}
	// 変化しないアドレス（$BF は exerciser 未使用）→ タイムアウト側
	changed, _, _, _, err = e.WatchRAM(0xBF, 2)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("unused address reported change")
	}
}
