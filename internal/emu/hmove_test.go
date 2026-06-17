package emu

import "testing"

// TestHMOVEHazardDetector は VV-10 T-2 の falsifiable 自己テスト：
//   - clean ツイン（HMxx は VBLANK で設定・HMOVE は WSYNC 直後・窓外）は NO hit。
//   - trap（HMOVE の ~3cy 後に HMP0 書き込み）は HIT。
func TestHMOVEHazardDetector(t *testing.T) {
	run := func(rom string) *HMOVEHazardHit {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		hit, err := e.WatchHMOVEHazard(2)
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}

	if hit := run("../../roms/litmus/hmove_clean.bin"); hit != nil {
		t.Fatalf("clean HMOVE pattern must NOT be flagged, got hit @frame %d pc 0x%04X (%dcy after)", hit.Frame, hit.PC, hit.CyclesAfter)
	}
	if hit := run("../../roms/litmus/hmove_trap.bin"); hit == nil {
		t.Fatalf("trap (HMxx within 24cy of HMOVE) must be flagged, got no hit")
	} else if hit.CyclesAfter >= 24 {
		t.Fatalf("hazard reported %dcy after HMOVE, expected < 24", hit.CyclesAfter)
	}
}
