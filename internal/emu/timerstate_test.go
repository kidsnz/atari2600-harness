package emu

import "testing"

// TestTimerStateObservability は RIOT タイマ内部状態の観測（VV-10 T-1 の土台）を検証する。
// cb_timer.bin は TIM64T を使う既知のカーネルなので、走らせると divider=64 が観測できるはず。
func TestTimerStateObservability(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/cb_timer.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	ts := e.TimerState()
	// 妥当性: divider は 1/8/64/1024 のいずれか。
	switch ts.Divider {
	case 1, 8, 64, 1024:
	default:
		t.Fatalf("divider=%d, want one of 1/8/64/1024", ts.Divider)
	}
	// cb_timer は TIM64T を使う＝64 を観測できる（実書き込みを読めている証拠）。
	if ts.Divider != 64 {
		t.Fatalf("cb_timer uses TIM64T; expected divider=64, got %d (INTIM=%d TIMINT=%#02x)", ts.Divider, ts.INTIM, ts.TIMINT)
	}
	if ts.TicksRemaining < 0 {
		t.Fatalf("ticksRemaining must be >= 0, got %d", ts.TicksRemaining)
	}
}
