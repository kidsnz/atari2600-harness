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
