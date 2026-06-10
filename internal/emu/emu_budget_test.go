package emu

import "testing"

// TestBudgetGuardCatchesOverrun は、わざと 1 本だけ 76cy 予算を超える可視ラインを仕込んだ ROM で
// per-scanline 予算ガードが over=true を返し、そのラインが 2 物理スキャンライン（=152 machine cy）を
// 消費したと報告することを裏取りする。
func TestBudgetGuardCatchesOverrun(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_overrun.bin"); err != nil {
		t.Fatal(err)
	}

	over, atScanline, lineCycles, err := e.RunUntilBudget(3, 76)
	if err != nil {
		t.Fatal(err)
	}
	if !over {
		t.Fatalf("expected over=true for litmus_overrun, got false")
	}
	// 重いラインは work ~100cy > 76 ＝ 2 物理ライン消費 → lineCycles = 2*76 = 152。
	if lineCycles != 152 {
		t.Fatalf("lineCycles = %d, want 152 (one logical line eating 2 scanlines)", lineCycles)
	}
	// 重いラインは可視領域（VSYNC3+VBLANK37=40 以降）にある。雑な妥当域でガード。
	if atScanline < 40 || atScanline > 240 {
		t.Fatalf("at_scanline = %d, out of plausible visible range", atScanline)
	}
}

// TestBudgetGuardNoFalsePositive は、毎ライン規律正しく WSYNC する正常 ROM で予算ガードが
// 誤発火しない（over=false）ことを裏取りする。smoke=合成 litmus / frogger=実ゲーム。
func TestBudgetGuardNoFalsePositive(t *testing.T) {
	for _, rom := range []string{
		"../../roms/litmus/smoke.bin",
		"../../roms/frogger/frogger.bin",
	} {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		over, atScanline, lineCycles, err := e.RunUntilBudget(3, 76)
		if err != nil {
			t.Fatal(err)
		}
		if over {
			t.Fatalf("%s: unexpected budget overrun at scanline %d (%d cy) — false positive",
				rom, atScanline, lineCycles)
		}
	}
}
