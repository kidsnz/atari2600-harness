package emu

import (
	"bytes"
	"testing"
)

// 地の真値 (ground truth) は litmus ROM のソースから取る:
//   - roms/litmus/litmus_pos.asm  : $80 = DELAY（粗調整ループ回数・1 単位 = 15px 横移動）→ x_position
//   - roms/litmus/motion_glide.asm: $80 = posY （ボールの走査線位置・+1/frame）        → y_position
// どちらも「そのバイトが何をするか」が事前に判っているので、probe の分類を採点できる。

func TestProbeClassifiesXPosition(t *testing.T) {
	e := newLoaded(t, "../../roms/litmus/litmus_pos.bin", 10)

	// DELAY は 1 単位 = 15px。0x00..0xF0 では画面 160px を大きく超えるので実レンジで探る。
	res, err := e.ProbeRAMSemantics(ProbeOptions{
		Addrs:  []uint16{0x80},
		Values: []uint8{0, 2, 4, 6, 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	r := res[0]
	if r.Class != "x_position" {
		t.Errorf("$80 (DELAY, 15px per unit) classified %q, want x_position (span_x=%.1f span_y=%.1f)",
			r.Class, r.SpanX, r.SpanY)
	}
	if r.SpanX < 15 {
		t.Errorf("centroid_span_x = %.1f, want >= 15 (one DELAY unit is 15px)", r.SpanX)
	}
}

func TestProbeClassifiesYPosition(t *testing.T) {
	e := newLoaded(t, romAnim, 10) // motion_glide: $80 = posY

	res, err := e.ProbeRAMSemantics(ProbeOptions{
		Addrs:  []uint16{0x80},
		Values: []uint8{40, 60, 80, 100, 120},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := res[0]
	if r.Class != "y_position" {
		t.Errorf("$80 (posY) classified %q, want y_position (span_x=%.1f span_y=%.1f)",
			r.Class, r.SpanX, r.SpanY)
	}
}

// motion_glide が使う RAM は $80 だけ。それ以外を叩いても画面は変わらないはず
// （= 偽陽性を出さないことの証明。肯定だけでなく否定も測る）。
func TestProbeUnusedAddressesReportNone(t *testing.T) {
	e := newLoaded(t, romAnim, 10)

	res, err := e.ProbeRAMSemantics(ProbeOptions{Addrs: []uint16{0x90, 0xA0, 0xB0, 0xC0, 0xF0}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Class != "none" || r.MaxChanged != 0 {
			t.Errorf("%s: class=%q changed=%d — motion_glide only uses $80, so this is a false positive",
				r.AddrHex, r.Class, r.MaxChanged)
		}
	}
}

// 非破壊性: probe の前後で RAM・座標・画面が一致する（probe はツールであって副作用ではない）。
func TestProbeIsNonDestructive(t *testing.T) {
	e := newLoaded(t, romAnim, 25)

	ramBefore, pixBefore, cycBefore := ramOf(t, e), pixOf(e), e.TotalCycles()
	cBefore := e.Coords()

	if _, err := e.ProbeRAMSemantics(ProbeOptions{Addrs: []uint16{0x80, 0x81, 0x82}}); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(ramBefore, ramOf(t, e)) {
		t.Error("probe mutated RAM")
	}
	if !bytes.Equal(pixBefore, pixOf(e)) {
		t.Error("probe mutated the framebuffer")
	}
	if c := e.Coords(); c != cBefore {
		t.Errorf("probe moved the beam: got %v want %v", c, cBefore)
	}
	if got := e.TotalCycles(); got != cycBefore {
		t.Errorf("probe changed the cycle counter: got %d want %d", got, cycBefore)
	}
}

func TestProbeRejectsAddressOutsideRAM(t *testing.T) {
	e := newLoaded(t, romAnim, 5)
	if _, err := e.ProbeRAMSemantics(ProbeOptions{Addrs: []uint16{0x0040}}); err == nil {
		t.Error("expected an error for an address outside $80-$FF")
	}
}
