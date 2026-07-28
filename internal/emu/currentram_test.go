package emu

import "testing"

// CurrentRAM は「PeekRAM を $80..$FF に 128 回」と厳密に同じ値を返さねばならない。
// 一括読みが独自経路（ミラー解決の違い・オフバイワン・stale なコピー）を持たないことの証明。
func TestCurrentRAMMatchesPeekLoop(t *testing.T) {
	e := newLoaded(t, romAnim, 12)

	got, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	want := ramOf(t, e) // $80..$FF を 1 バイトずつ PeekRAM した既存経路

	if len(want) != RAMSize {
		t.Fatalf("test premise broken: peek loop yielded %d bytes, RAMSize=%d", len(want), RAMSize)
	}
	for i := 0; i < RAMSize; i++ {
		if got[i] != want[i] {
			t.Errorf("$%02X: CurrentRAM=%02X peek=%02X", RAMBase+i, got[i], want[i])
		}
	}
}

// 添字は番地−$80 でなければならない（out[0] が $80）。境界の両端を名指しで確かめる
// ＝オフバイワンだと全体が 1 バイトずれたまま上のテストも通ってしまう、を防ぐ。
func TestCurrentRAMIndexIsAddressMinusBase(t *testing.T) {
	e := newLoaded(t, romAnim, 7)

	got, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	for _, addr := range []uint16{0x80, 0x81, 0xFE, 0xFF} {
		want, err := e.PeekRAM(addr)
		if err != nil {
			t.Fatal(err)
		}
		if g := got[addr-RAMBase]; g != want {
			t.Errorf("$%02X: CurrentRAM[%d]=%02X want %02X", addr, addr-RAMBase, g, want)
		}
	}
}

// 生きた観測であることの証明。自走 ROM を進めれば少なくとも 1 バイトは変わるはずで、
// 変わらなければ「保存済みスナップショットを返しているだけ」を疑う（読取りが frame に
// 追随しないと、全RAMトレースは常に同じ値の列になり、遷移関数が測れない）。
func TestCurrentRAMTracksFrames(t *testing.T) {
	e := newLoaded(t, romAnim, 3)

	before, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(5); err != nil {
		t.Fatal(err)
	}
	after, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Errorf("RAM identical across 5 frames of a self-animating ROM (%s) — CurrentRAM may be stale", romAnim)
	}
}
