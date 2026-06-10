// Package playfield は「ASCIIグリッド（点灯セル列）→ TIA の PF0/PF1/PF2 バイト」変換を担う。
// ビット順は ABB(kirkjerk) と falukropp の2ソース独立実装が一致し、さらに Gopher2600 上で
// litmus_pf + read_row により数値裏取り済み（docs/resources.md / CLAUDE.md）。
//
// 検証済み変換表（画面 左→右に40列、各列=4カラークロック幅）:
//
//	col:  0 1 2 3 | 4 5 6 7 8 9 10 11 | 12 13 14 15 16 17 18 19
//	reg:  PF0     | PF1               | PF2
//	bit:  4 5 6 7 | 7 6 5 4 3 2 1  0  | 0  1  2  3  4  5  6  7
//
// PF0=上ニブルのみ col0→D4..col3→D7 / PF1=MSB先 col4→D7..col11→D0 / PF2=LSB先 col12→D0..col19→D7。
package playfield

// HalfWidth は片側 playfield の列数（左半＝可視 clock 0..79、20列）。
const HalfWidth = 20

// FullWidth は非対称 playfield の総列数（左20＋右20）。
const FullWidth = 40

// encodeHalf は 20 セル（col 0..19）を検証済みビット順で PF0/PF1/PF2 へ符号化する。
// cells が 20 未満なら残りは消灯(0)、20 超は無視。
func encodeHalf(cells []bool) (pf0, pf1, pf2 byte) {
	n := len(cells)
	if n > HalfWidth {
		n = HalfWidth
	}
	for col := 0; col < n; col++ {
		if !cells[col] {
			continue
		}
		switch {
		case col <= 3: // PF0 上ニブル: col0→D4 .. col3→D7
			pf0 |= 1 << uint(4+col)
		case col <= 11: // PF1 MSB先: col4→D7 .. col11→D0
			pf1 |= 1 << uint(11-col)
		default: // PF2 LSB先: col12→D0 .. col19→D7
			pf2 |= 1 << uint(col-12)
		}
	}
	return
}

// EncodeSymmetric は 1 行 20 セルを 1 組の PF0/PF1/PF2 に符号化する。
// 表示側 CTRLPF D0 で repeat（右半＝左半の複製）/ reflect（鏡像）を選ぶ。
// ハードが右半を自動生成するので右半データは不要＝最も安価なカーネル。
func EncodeSymmetric(cells []bool) (pf0, pf1, pf2 byte) {
	return encodeHalf(cells)
}

// AsymRow は非対称 playfield 1 行ぶんの 6 バイト。A=左半(col0..19)、B=右半(col20..39)。
// 表示には 1 ライン内で PF0/1/2 を A→B に詰め替える非対称カーネルが要る（ABB の例カーネル参照）。
type AsymRow struct {
	PF0A, PF1A, PF2A byte // 左半（可視 clock 0..79）
	PF0B, PF1B, PF2B byte // 右半（可視 clock 80..159）
}

// EncodeAsymmetric は 1 行 40 セルを左右独立の 6 バイトに符号化する。
// 左右で別の絵を出せる（有機的・非対称な水面/睡蓮向き）。cells が 40 未満なら残りは消灯。
func EncodeAsymmetric(cells []bool) AsymRow {
	left := cells
	if len(left) > HalfWidth {
		left = cells[:HalfWidth]
	}
	pf0a, pf1a, pf2a := encodeHalf(left)

	var right []bool
	if len(cells) > HalfWidth {
		right = cells[HalfWidth:]
	}
	pf0b, pf1b, pf2b := encodeHalf(right)

	return AsymRow{
		PF0A: pf0a, PF1A: pf1a, PF2A: pf2a,
		PF0B: pf0b, PF1B: pf1b, PF2B: pf2b,
	}
}

// ParseASCIIRow は 1 文字 = 1 列の文字列を点灯セル列へ変換する。
// on に含まれる文字（既定 'X' と '#'）が点灯、それ以外（'.' 等）は消灯。
func ParseASCIIRow(s string, on ...rune) []bool {
	onSet := map[rune]bool{'X': true, '#': true}
	for _, r := range on {
		onSet[r] = true
	}
	cells := make([]bool, 0, len(s))
	for _, r := range s {
		cells = append(cells, onSet[r])
	}
	return cells
}
