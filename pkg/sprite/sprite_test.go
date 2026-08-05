package sprite

import (
	"os"
	"regexp"
	"strconv"
	"strings"
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

// TestSplitWide は 16 列設計が P0(左8)・P1(右8)に正しく割れることを固定する。
func TestSplitWide(t *testing.T) {
	art := []string{
		"XXXXXXXXXXXXXXXX", // solid 16  -> P0=$FF P1=$FF
		"XXXXXXXX........", // left half -> P0=$FF P1=$00
		"........XXXXXXXX", // right half-> P0=$00 P1=$FF
		"X..............X", // far edges -> P0=$80 P1=$01
		"X...............", // P0 のみ最左 -> P0=$80 P1=$00
	}
	wantP0 := []byte{0xFF, 0xFF, 0x00, 0x80, 0x80}
	wantP1 := []byte{0xFF, 0x00, 0xFF, 0x01, 0x00}
	p0, p1 := SplitWide(art)
	for i := range art {
		if p0[i] != wantP0[i] || p1[i] != wantP1[i] {
			t.Errorf("row %d %q: got P0=%02X P1=%02X, want P0=%02X P1=%02X",
				i, art[i], p0[i], p1[i], wantP0[i], wantP1[i])
		}
	}
}

// TestNUSIZ は player size / missile size の合成が正しいことを固定する。
func TestNUSIZ(t *testing.T) {
	if got := NUSIZPlayer(OneCopy); got != 0x00 {
		t.Errorf("OneCopy = %02X, want 00", got)
	}
	if got := NUSIZPlayer(DoubleWidth); got != 0x05 {
		t.Errorf("DoubleWidth = %02X, want 05", got)
	}
	if got := NUSIZPlayer(QuadWidth); got != 0x07 {
		t.Errorf("QuadWidth = %02X, want 07", got)
	}
	// 3 コピー近接 + ミサイル 8px = $30 | $03 = $33
	if got := NUSIZ(ThreeCopiesClose, Missile8px); got != 0x33 {
		t.Errorf("NUSIZ(ThreeCopiesClose, Missile8px) = %02X, want 33", got)
	}
	// 倍幅 + ミサイル 2px = $10 | $05 = $15
	if got := NUSIZ(DoubleWidth, Missile2px); got != 0x15 {
		t.Errorf("NUSIZ(DoubleWidth, Missile2px) = %02X, want 15", got)
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

// DigitFont: 10 桁・各行は 6px 幅（下位 2bit 空白）・全桁に点灯行がある。
func TestDigitFont(t *testing.T) {
	f := DigitFont()
	for d, rows := range f {
		lit := false
		for r, b := range rows {
			if b&0x03 != 0 {
				t.Errorf("digit %d row %d uses low 2 bits: %02X", d, r, b)
			}
			if b != 0 {
				lit = true
			}
		}
		if !lit {
			t.Errorf("digit %d is blank", d)
		}
	}
}

// TestDigitFontMatchesScore6 turns DigitFont's own doc comment into a check. It claims
// the glyphs are identical to the score6 technique's and that it returns them top-first,
// while score6.asm stores them bottom-first for a Y=7->0 kernel — so the two tables must
// be exact reverses of each other, digit for digit.
//
// Nothing asserted that before, and digit 9 was wrong: it held score6's raw bottom-first
// bytes while the other nine were reversed, which draws an upside-down glyph. The
// existing TestDigitFont could not see it (low-2-bits-clear and not-blank are both true
// of a flipped glyph), and the package has no importer, so no ROM was ever going to
// notice either. Parsing the .asm rather than restating its bytes here is deliberate: a
// second hand-copied table would be a third copy to drift.
func TestDigitFontMatchesScore6(t *testing.T) {
	const asmPath = "../../roms/techniques/score6.asm"
	src, err := os.ReadFile(asmPath)
	if err != nil {
		t.Fatalf("read %s: %v", asmPath, err)
	}
	// Font table rows look like: `        byte $78,$CC,...,$78   ; 0`
	row := regexp.MustCompile(`(?m)^\s*byte\s+((?:\$[0-9A-Fa-f]{2},){7}\$[0-9A-Fa-f]{2})\s*;\s*(\d)`)
	ms := row.FindAllStringSubmatch(string(src), -1)
	if len(ms) != 10 {
		t.Fatalf("parsed %d font rows from %s, want 10 — the table's shape changed and this "+
			"test would silently stop checking", len(ms), asmPath)
	}
	got := DigitFont()
	for _, m := range ms {
		d, err := strconv.Atoi(m[2])
		if err != nil || d < 0 || d > 9 {
			t.Fatalf("bad digit label %q", m[2])
		}
		var asm [8]byte
		for i, f := range strings.Split(m[1], ",") {
			v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(f), "$"), 16, 8)
			if err != nil {
				t.Fatalf("digit %d field %q: %v", d, f, err)
			}
			asm[i] = byte(v)
		}
		// score6 is bottom-first; DigitFont documents top-first.
		var want [8]byte
		for i := range asm {
			want[i] = asm[7-i]
		}
		if got[d] != want {
			t.Errorf("digit %d: DigitFont()=%X, reversed score6=%X — one of the two copies drifted",
				d, got[d], want)
		}
	}
}
