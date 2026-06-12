package emu

import "testing"

// two_line_vdel: 2-line kernel でも P0 の上端が「毎フレームちょうど 1 走査線」動く
// （VDELP0=偶奇ビット）。VDEL 偶奇技の数値証明。
func TestVDELOddEven(t *testing.T) {
	e, _ := New("NTSC")
	if err := e.LoadROM("../../roms/techniques/two_line_vdel.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(20)
	_, top := e.cap.snapshot()
	find := func() int {
		for sl := top; sl < top+200; sl++ {
			runs, _, _ := e.ReadRow(sl)
			for _, r := range runs {
				if r.Hex == "FFFF29" { // P0 黄 $1E
					return sl
				}
			}
		}
		return -1
	}
	prev := find()
	if prev < 0 {
		t.Fatal("P0 not found")
	}
	for i := 0; i < 6; i++ {
		e.RunFrames(1)
		cur := find()
		if cur != prev+1 {
			t.Fatalf("frame %d: top %d -> %d, want exactly +1 (VDEL odd/even)", i, prev, cur)
		}
		prev = cur
	}
}
