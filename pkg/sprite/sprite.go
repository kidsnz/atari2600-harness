// Package sprite は「ASCIIアート（8列幅の行）→ TIA プレイヤー GRP バイト」変換を担う。
// pkg/playfield の鏡。プレイヤーは横 8px、GRP は D7 が最初に描画される（= 最左ピクセル）＝MSB 先。
// REFP(reflect)=1 のときハードが LSB 先で描くが、本パッケージは未反転（REFP=0 前提）の素データを返す。
//
// ビット順（実機標準・既存の手書きスプライトとも一致）:
//
//	col:  0 1 2 3 4 5 6 7   （画面 左→右）
//	bit:  7 6 5 4 3 2 1 0   （GRP の D7..D0）
//
// 例: "..XXXX.." → 0x3C（= Monet Frogger の睡蓮パッド 1 行 `pad[0]` と同値）。
package sprite

import "github.com/kidsnz/atari2600-harness/pkg/playfield"

// Width はプレイヤースプライトの横幅（px）。
const Width = 8

// EncodeRow は 8 セル（cells[0]=最左）を 1 GRP バイトへ符号化する（col0→D7, col7→D0）。
// cells が 8 未満なら残りは消灯(0)、8 超は無視。
func EncodeRow(cells []bool) byte {
	var b byte
	for i := 0; i < Width && i < len(cells); i++ {
		if cells[i] {
			b |= 1 << (7 - uint(i)) // col0→D7 … col7→D0
		}
	}
	return b
}

// Encode は ASCII アート（各行 8 文字・最上段が先頭）を GRP バイト列へ符号化する。
// 戻りは設計順（gfx[0]=最上段 scanline）。kernel 側のテーブル並び（多くは下→上）は生成器が面倒を見る。
// 点灯文字は playfield.ParseASCIIRow と同じ（既定 'X'/'#'、'.' 等は消灯）。
func Encode(art []string) []byte {
	gfx := make([]byte, len(art))
	for i, line := range art {
		gfx[i] = EncodeRow(playfield.ParseASCIIRow(line))
	}
	return gfx
}

// WideWidth は P0+P1 連結スプライトの最大横幅（px）。P0=左8列・P1=右8列。
const WideWidth = 16

// PlayerSize は NUSIZx レジスタ下位 3bit（D2..D0）が表すプレイヤーのコピー数／横幅モード。
// 実機の number-size 表（Stella Programmer's Guide）どおり。
type PlayerSize uint8

const (
	OneCopy           PlayerSize = 0 // 1 コピー・通常幅(8px)
	TwoCopiesClose    PlayerSize = 1 // 2 コピー（近接・間隔 16）
	TwoCopiesMedium   PlayerSize = 2 // 2 コピー（中・間隔 32）
	ThreeCopiesClose  PlayerSize = 3 // 3 コピー（近接）
	TwoCopiesWide     PlayerSize = 4 // 2 コピー（広・間隔 64）
	DoubleWidth       PlayerSize = 5 // 1 コピー・2 倍幅(16px)
	ThreeCopiesMedium PlayerSize = 6 // 3 コピー（中）
	QuadWidth         PlayerSize = 7 // 1 コピー・4 倍幅(32px)
)

// MissileSize はミサイル/ボール幅（NUSIZx D5..D4 / CTRLPF・ENABL 系の D5..D4）。
type MissileSize uint8

const (
	Missile1px MissileSize = 0
	Missile2px MissileSize = 1
	Missile4px MissileSize = 2
	Missile8px MissileSize = 3
)

// NUSIZ は「プレイヤーのコピー/幅」と「ミサイル幅」を 1 つの NUSIZx バイトへ合成する。
// 下位 3bit = PlayerSize、D5..D4 = MissileSize（他ビットは 0）。
func NUSIZ(p PlayerSize, m MissileSize) byte {
	return byte(p)&0x07 | (byte(m)&0x03)<<4
}

// NUSIZPlayer は PlayerSize だけを NUSIZx バイトに（ミサイル幅 0）。
func NUSIZPlayer(p PlayerSize) byte { return byte(p) & 0x07 }

// SplitWide は 16 列幅の ASCII 設計を P0(左 col0..7)・P1(右 col8..15)の GRP バイト列へ分割する。
// 表示側で P1 を P0 のちょうど +8px に隣接配置すると、2 枚のスプライトが 1 体の最大 16px 幅キャラに見える
// （多色にしたい場合は P0/P1 に別 COLUP を与える）。各行 16 文字・最上段が先頭。p0[i]/p1[i] は設計順（上→下）。
// 行が 16 文字未満なら足りない列は消灯、8 文字以下なら p1 は全消灯。
func SplitWide(art []string) (p0, p1 []byte) {
	p0 = make([]byte, len(art))
	p1 = make([]byte, len(art))
	for i, line := range art {
		cells := playfield.ParseASCIIRow(line)
		left := cells
		if len(cells) > Width {
			left = cells[:Width]
		}
		p0[i] = EncodeRow(left)
		if len(cells) > Width {
			p1[i] = EncodeRow(cells[Width:])
		}
	}
	return p0, p1
}

// Reflect は GRP バイトのビットを左右反転する（D7↔D0, D6↔D1, …）。
// REFP を使わず素データ側で鏡像にしたい場合や、P0+P1 連結で右半を作る場合に使う。
func Reflect(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		if b&(1<<uint(i)) != 0 {
			r |= 1 << (7 - uint(i))
		}
	}
	return r
}

// DigitFont は 0-9 のスコア用フォント（U-M1, score6 技と同一の絵柄）。
// 各桁 8 行・**最上段が先頭**（[d][0]=上）。グリフは 6px 幅＋右 2px 空白
// （48px スコアカーネルではコピーが 8px ピッチで隣接するため、桁間隔をフォント側に内蔵する）。
// kernel が Y=7→0 で参照する場合（score6.asm）は逆順で .byte 化すること。
func DigitFont() [10][8]byte {
	return [10][8]byte{
		{0x78, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0x78}, // 0
		{0x30, 0x70, 0x30, 0x30, 0x30, 0x30, 0x30, 0xFC}, // 1
		{0x78, 0xCC, 0x0C, 0x18, 0x30, 0x60, 0xC0, 0xFC}, // 2
		{0x78, 0xCC, 0x0C, 0x38, 0x0C, 0x0C, 0xCC, 0x78}, // 3
		{0x1C, 0x3C, 0x6C, 0xCC, 0xFC, 0x0C, 0x0C, 0x0C}, // 4
		{0xFC, 0xC0, 0xC0, 0xF8, 0x0C, 0x0C, 0xCC, 0x78}, // 5
		{0x38, 0x60, 0xC0, 0xF8, 0xCC, 0xCC, 0xCC, 0x78}, // 6
		{0xFC, 0x0C, 0x18, 0x30, 0x60, 0x60, 0x60, 0x60}, // 7
		{0x78, 0xCC, 0xCC, 0x78, 0xCC, 0xCC, 0xCC, 0x78}, // 8
		{0x70, 0x18, 0x0C, 0x7C, 0xCC, 0xCC, 0xCC, 0x78}, // 9
	}
}
