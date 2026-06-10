package sprite

import (
	"testing"

	"github.com/kidsnz/atari2600-harness/pkg/playfield"
)

// TestEncodeRow_BitOrder は col0=最左→D7 の実機標準ビット順を固定する。
func TestEncodeRow_BitOrder(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want byte
	}{
		{"leftmost_col0_D7", "X.......", 0x80},
		{"rightmost_col7_D0", ".......X", 0x01},
		{"all_on", "XXXXXXXX", 0xFF},
		{"all_off", "........", 0x00},
		// ↓ 既存の手書きスプライト（Monet Frogger 睡蓮パッド pad[0]）と一致＝設計の裏取り。
		{"frogger_pad_3C", "..XXXX..", 0x3C},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EncodeRow(playfield.ParseASCIIRow(c.row))
			if got != c.want {
				t.Errorf("row %q: got %02X, want %02X", c.row, got, c.want)
			}
		})
	}
}

// TestEncode_TopToBottom は戻りが設計順（gfx[0]=最上段）であることを固定する。
func TestEncode_TopToBottom(t *testing.T) {
	art := []string{"X.......", "........", ".......X"}
	got := Encode(art)
	want := []byte{0x80, 0x00, 0x01}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("gfx[%d] = %02X, want %02X", i, got[i], want[i])
		}
	}
}

// TestReflect は左右反転が対称になることを確認する（reflect(reflect(b)) == b）。
func TestReflect(t *testing.T) {
	cases := []struct{ in, want byte }{
		{0x80, 0x01}, // D7 → D0
		{0x01, 0x80},
		{0x3C, 0x3C}, // 左右対称はそのまま
		{0xB1, 0x8D}, // 10110001 → 10001101
	}
	for _, c := range cases {
		if got := Reflect(c.in); got != c.want {
			t.Errorf("Reflect(%02X) = %02X, want %02X", c.in, got, c.want)
		}
		if rr := Reflect(Reflect(c.in)); rr != c.in {
			t.Errorf("Reflect twice on %02X = %02X, want identity", c.in, rr)
		}
	}
}
