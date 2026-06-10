// Command genpf は北極星 ROM「Monet 睡蓮 Frogger」の M1（静止画）シーンを設計し、
// internal/playfield で DASM ソース（roms/monet_m1.asm）に変換する。
// 「ASCIIアート＋色テーブル → ROM」配管の最初の本番＝対称(reflect) playfield 静止画。
package main

import (
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-dev/internal/playfield"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "asym" {
		genAsymTest()
		return
	}
	genMonetM1()
}

// genAsymTest は非対称 playfield の検証シーン。幅4の点灯ブロックが 40 列を上→下に掃引する対角
// ストライプ。上の行は左半(col<20)だけ・下の行は右半(col>=20)だけ点灯＝reflect では不可能な絵。
// read_row で「上の行は左半のみ lit / 下の行は右半のみ lit」を数値確認すれば非対称が証明される。
func genAsymTest() {
	const rows = 48
	art := make([]string, rows)
	lily := make([]byte, rows)
	for r := 0; r < rows; r++ {
		start := r * 36 / rows                                    // 0..35 を掃引
		row := []byte("........................................") // 40 dots
		for dx := 0; dx < 4 && start+dx < 40; dx++ {
			row[start+dx] = 'X'
		}
		art[r] = string(row)
		lily[r] = 0x0E // 白（位置が読みやすいよう単色）
	}
	const water = 0x84 // 青
	src, err := playfield.GenerateAsymmetricASM(art, lily, water, playfield.SceneOpts{LineHeight: 4})
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/asym_test.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d rows, diagonal sweep 0..35)\n", out, rows)
}

func genMonetM1() {
	const rows = 48

	// 決定的ジッタ（再現可能な「broken color」揺らぎ用 LCG）。
	seed := uint32(0x2600)
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed >> 16 }

	water := make([]byte, rows)
	lily := make([]byte, rows)

	// 水: 上→下にブルー→バイオレット→グリーンへ漂う wash＋行ごとの輝度揺らぎ（NTSC: 上位=色相/下位=輝度）。
	hues := []byte{0x70, 0x80, 0x90, 0x50, 0x60, 0xB0, 0xC0} // blue, blue, lightblue, violet, blue-violet, cyan-green, green
	for y := 0; y < rows; y++ {
		hue := hues[(y/3+int(next()%2))%len(hues)]
		lum := byte(4 + (next()%5)*2) // 4..12
		water[y] = hue | lum

		lhue := byte(0xC0) // 既定: 緑のパッド
		switch next() % 6 {
		case 0:
			lhue = 0x40 // ピンクの花
		case 1:
			lhue = 0xF0 // 黄のハイライト
		}
		lily[y] = lhue | byte(6+(next()%4)*2) // 輝度 6..12
	}

	// アート: 一面の水（'.'）に睡蓮パッド（'X'）を散布。左半 20 列、reflect で左右対称になる。
	grid := make([][]byte, rows)
	for y := range grid {
		grid[y] = []byte("....................") // 20 dots
	}
	pads := []struct{ row, col, w int }{
		{4, 2, 4}, {7, 11, 3}, {12, 5, 5}, {16, 14, 4},
		{21, 1, 3}, {25, 8, 4}, {30, 12, 5}, {34, 3, 4},
		{39, 9, 3}, {43, 15, 4},
	}
	for _, p := range pads {
		for dy := 0; dy < 2 && p.row+dy < rows; dy++ { // 縦に少し厚みを
			for dx := 0; dx < p.w && p.col+dx < 20; dx++ {
				grid[p.row+dy][p.col+dx] = 'X'
			}
		}
	}
	art := make([]string, rows)
	for y := range grid {
		art[y] = string(grid[y])
	}

	src, err := playfield.GenerateSymmetricASM(art, water, lily, playfield.SceneOpts{LineHeight: 4})
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/monet_m1.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d rows, height 4 → %d visible scanlines)\n", out, rows, rows*4)
}
