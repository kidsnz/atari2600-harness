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
	water := make([]byte, rows)
	for r := 0; r < rows; r++ {
		start := r * 36 / rows                                    // 0..35 を掃引
		row := []byte("........................................") // 40 dots
		for dx := 0; dx < 4 && start+dx < 40; dx++ {
			row[start+dx] = 'X'
		}
		art[r] = string(row)
		water[r] = 0x84 // 青（位置が読みやすいよう単色背景）
	}
	const lily = 0x0E // 白の前景ストライプ（定数 COLUPF）
	src, err := playfield.GenerateAsymmetricASM(art, water, lily, playfield.SceneOpts{LineHeight: 4})
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

	// 水: per-row COLUBK。上→下にブルー→バイオレット→グリーンへ漂う wash＋輝度揺らぎ。
	// （非対称ループは予算が無く per-row 色は1チャンネルのみ → 面積の大きい水を per-row に。）
	water := make([]byte, rows)
	hues := []byte{0x70, 0x80, 0x90, 0x50, 0x60, 0xB0, 0xC0} // blue, blue, lightblue, violet, blue-violet, cyan-green, green
	for y := 0; y < rows; y++ {
		hue := hues[(y/3+int(next()%2))%len(hues)]
		lum := byte(4 + (next()%5)*2) // 4..12
		water[y] = hue | lum
	}
	const lily = 0xC8 // 睡蓮パッド（定数 COLUPF・緑）

	// アート: 40 列の有機的・非対称な睡蓮パッド散布（左右独立。中央 col~20 をまたぐパッドも）。
	grid := make([][]byte, rows)
	for y := range grid {
		grid[y] = []byte("........................................") // 40 dots
	}
	pads := []struct{ row, col, w, h int }{
		{2, 5, 4, 2}, {3, 18, 5, 2}, {6, 28, 4, 1}, {8, 9, 3, 2},
		{11, 33, 5, 2}, {13, 1, 4, 1}, {15, 20, 6, 3}, {18, 12, 4, 2},
		{21, 30, 4, 2}, {24, 3, 5, 2}, {26, 22, 4, 1}, {30, 15, 5, 3},
		{33, 35, 3, 2}, {35, 7, 4, 2}, {38, 25, 5, 2}, {41, 17, 4, 2},
		{44, 2, 3, 1},
	}
	for _, p := range pads {
		for dy := 0; dy < p.h && p.row+dy < rows; dy++ {
			for dx := 0; dx < p.w && p.col+dx < 40; dx++ {
				grid[p.row+dy][p.col+dx] = 'X'
			}
		}
	}
	art := make([]string, rows)
	for y := range grid {
		art[y] = string(grid[y])
	}

	src, err := playfield.GenerateAsymmetricASM(art, water, lily, playfield.SceneOpts{LineHeight: 4})
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
