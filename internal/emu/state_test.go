package emu

import (
	"bytes"
	"testing"
)

// romAnim は「自走してフレームごとに RAM も絵も変わる」ROM。フレームバッファ復元の検証には
// 静止画 ROM では不十分（保存時と分岐時の絵が同一だと差が出ない）ため animation を使う。
// 本リポジトリ自身の litmus のみを参照する（harness は game repo に依存しない = CLAUDE.md の不変則）。
const romAnim = "../../roms/litmus/motion_glide.bin"

func newLoaded(t *testing.T, rom string, frames int) *Emu {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Skipf("ROM unavailable (%s): %v", rom, err)
	}
	if err := e.RunFrames(frames); err != nil {
		t.Fatal(err)
	}
	return e
}

func ramOf(t *testing.T, e *Emu) []uint8 {
	t.Helper()
	out := make([]uint8, 0, 128)
	for a := uint16(0x80); a <= 0xff; a++ {
		v, err := e.PeekRAM(a)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func pixOf(e *Emu) []uint8 {
	img, _ := e.Snapshot()
	out := make([]uint8, len(img.Pix))
	copy(out, img.Pix)
	return out
}

// TestSaveRestoreRoundTrip: 保存→分岐→復元 で RAM・座標・サイクル・**画面**が保存時に戻る。
func TestSaveRestoreRoundTrip(t *testing.T) {
	e := newLoaded(t, romAnim, 60)

	st := e.SaveState()
	ramA, pixA, cycA := ramOf(t, e), pixOf(e), e.TotalCycles()
	cA := e.Coords()

	if err := e.RunFrames(60); err != nil { // 分岐
		t.Fatal(err)
	}
	ramMid, pixMid := ramOf(t, e), pixOf(e)
	if bytes.Equal(ramA, ramMid) {
		t.Fatal("precondition failed: RAM did not change over 60 frames — cannot test restore")
	}
	if bytes.Equal(pixA, pixMid) {
		t.Fatal("precondition failed: screen did not change over 60 frames — cannot test framebuffer restore")
	}

	if err := e.RestoreState(st); err != nil {
		t.Fatal(err)
	}

	if got := ramOf(t, e); !bytes.Equal(ramA, got) {
		n := 0
		for i := range ramA {
			if ramA[i] != got[i] {
				n++
			}
		}
		t.Errorf("RAM not restored: %d/128 bytes differ", n)
	}
	// ★ これが回帰の本体: television.Plumb は PixelRenderer を触らないので、
	// RestoreState が pix を戻さなくなった瞬間ここが「分岐後の絵」で落ちる。
	if got := pixOf(e); !bytes.Equal(pixA, got) {
		t.Error("framebuffer not restored (get_screen would return the diverged frame)")
	}
	if c := e.Coords(); c.Frame != cA.Frame || c.Scanline != cA.Scanline || c.Clock != cA.Clock {
		t.Errorf("coords not restored: got %v want %v", c, cA)
	}
	if got := e.TotalCycles(); got != cycA {
		t.Errorf("cpu cycle counter not restored: got %d want %d", got, cycA)
	}
}

// TestRestoreDeterministicAndReusable: 復元後のリプレイは決定的で、同一 State は再利用できる
// （分岐探索が成立する条件そのもの）。
func TestRestoreDeterministicAndReusable(t *testing.T) {
	e := newLoaded(t, romAnim, 30)
	st := e.SaveState()

	if err := e.RunFrames(45); err != nil {
		t.Fatal(err)
	}
	wantRAM, wantPix := ramOf(t, e), pixOf(e)

	for trial := 0; trial < 3; trial++ {
		if err := e.RestoreState(st); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(45); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(wantRAM, ramOf(t, e)) {
			t.Errorf("trial %d: replayed RAM differs", trial)
		}
		if !bytes.Equal(wantPix, pixOf(e)) {
			t.Errorf("trial %d: replayed screen differs", trial)
		}
	}
}

// TestRestoreUndoesPoke: poke → 復元 で書き換えが消える（probe_ram_semantics の非破壊性の土台）。
func TestRestoreUndoesPoke(t *testing.T) {
	e := newLoaded(t, romAnim, 40)
	st := e.SaveState()

	before, err := e.PeekRAM(0x90)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Poke(0x90, before^0xff); err != nil {
		t.Fatal(err)
	}
	if v, _ := e.PeekRAM(0x90); v == before {
		t.Fatal("precondition failed: poke did not change RAM")
	}
	if err := e.RestoreState(st); err != nil {
		t.Fatal(err)
	}
	if v, _ := e.PeekRAM(0x90); v != before {
		t.Errorf("poke not undone by restore: got $%02X want $%02X", v, before)
	}
}

func TestRestoreNil(t *testing.T) {
	e := newLoaded(t, romAnim, 1)
	if err := e.RestoreState(nil); err == nil {
		t.Error("expected an error restoring a nil state")
	}
}
