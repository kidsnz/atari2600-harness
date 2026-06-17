package emu

import "testing"

// TestTimerWrapDetector は VV-10 T-1 の falsifiable 自己テスト：
//   - clean ツイン（TIM64T を 0 まで正しくポーリングし wrap 前に抜ける）は NO hit。
//   - trap（TIM1T で 0 を取り逃し wrap 後の値を読み続ける）は HIT。
// 両方向を固定する（誤検出ゼロ・見逃しゼロ）。
func TestTimerWrapDetector(t *testing.T) {
	run := func(rom string) *TimerWrapHit {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil { // 起動を越える
			t.Fatal(err)
		}
		hit, err := e.WatchTimerWrap(3)
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}

	if hit := run("../../roms/litmus/timerwrap_clean.bin"); hit != nil {
		t.Fatalf("clean kernel must NOT be flagged, got hit @frame %d pc 0x%04X", hit.Frame, hit.PC)
	}
	if hit := run("../../roms/litmus/timerwrap_trap.bin"); hit == nil {
		t.Fatalf("trap kernel (reads INTIM after wrap) must be flagged, got no hit")
	}
}
