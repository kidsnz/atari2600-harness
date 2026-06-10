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
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "asym":
		genAsymTest()
	case "anim":
		genMonetAnim()
	case "sprite":
		genMonetSprite()
	case "full":
		genMonetFull()
	case "collide":
		genMonetCollide()
	case "frogger":
		genFrogger()
	default:
		genMonetM1()
	}
}

// genFrogger は遊べる Monet Frogger → roms/frogger.asm。睡蓮の川レーン=scanline 80-87。
func genFrogger() {
	_, water48, _ := buildMonetScene()
	bg := make([]byte, 192)
	for y := 0; y < 192; y++ {
		switch {
		case y < 28: // 上＝ゴール帯（金）
			bg[y] = 0x1E
		case y >= 148: // 下＝スタート岸（緑）
			bg[y] = 0xC4
		default: // 中＝Monet 水
			bg[y] = water48[y/4]
		}
	}
	pad := []byte{0x3C, 0x7E, 0xFF, 0xFF, 0xFF, 0xFF, 0x7E, 0x3C}
	frog := []byte{0x24, 0x7E, 0xFF, 0xFF, 0xBD, 0x7E, 0x24, 0x42}
	grp0 := make([]byte, 192)
	for i := range pad {
		grp0[80+i] = pad[i] // 川レーン scanline 80-87（frog の FrogY=80 と一致）
	}
	src, err := playfield.GenerateFroggerASM(bg, grp0, frog, 0x03, 0xC8, 0x1C, 0xF0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/frogger.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (playable Monet Frogger)\n", out)
}

// genMonetCollide は衝突検証用: カエルを睡蓮と同じレーン(scanline76-83)に置く → 横で重なると CXPPMM D7。
func genMonetCollide() {
	_, water48, _ := buildMonetScene()
	bg := make([]byte, 192)
	for y := 0; y < 192; y++ {
		bg[y] = water48[y/4]
	}
	pad := []byte{0x3C, 0x7E, 0xFF, 0xFF, 0xFF, 0xFF, 0x7E, 0x3C}
	frog := []byte{0x24, 0x7E, 0xFF, 0xFF, 0xBD, 0x7E, 0x24, 0x42}
	grp0 := make([]byte, 192)
	grp1 := make([]byte, 192)
	for i := range pad {
		grp0[76+i] = pad[i]
		grp1[76+i] = frog[i] // カエルも同じレーンに（衝突検証）
	}
	src, err := playfield.GenerateMonetFullASM(bg, grp0, grp1, 0x03, 0x00, 0xC8, 0x1C, 0xF0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/monet_collide.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (frog in lily lane — collision test)\n", out)
}

// genMonetFull はフルシーン: Monet 水面＋流れる睡蓮(player0)＋操作カエル(player1) → roms/monet_full.asm。
func genMonetFull() {
	_, water48, _ := buildMonetScene()
	bg := make([]byte, 192)
	for y := 0; y < 192; y++ {
		bg[y] = water48[y/4]
	}
	pad := []byte{0x3C, 0x7E, 0xFF, 0xFF, 0xFF, 0xFF, 0x7E, 0x3C}
	frog := []byte{0x24, 0x7E, 0xFF, 0xFF, 0xBD, 0x7E, 0x24, 0x42}
	grp0 := make([]byte, 192) // 睡蓮レーン scanline 76..83
	for i, b := range pad {
		grp0[76+i] = b
	}
	grp1 := make([]byte, 192) // カエル scanline 150..157（手前）
	for i, b := range frog {
		grp1[150+i] = b
	}
	// player0=3コピー緑(右1px/f drift), player1=単体カエル黄緑
	src, err := playfield.GenerateMonetFullASM(bg, grp0, grp1, 0x03, 0x00, 0xC8, 0x1C, 0xF0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/monet_full.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (water + flowing lilies + controllable frog)\n", out)
}

// genMonetSprite は「Monet 水面（per-scanline 色帯）の上を流れる睡蓮スプライト」→ roms/monet_sprite.asm（M3 step2 統合）。
func genMonetSprite() {
	_, water48, _ := buildMonetScene() // 48 行ぶんの水色 wash を流用
	bg := make([]byte, 192)
	for y := 0; y < 192; y++ {
		bg[y] = water48[y/4] // 各行を ×4 に展開（行高4）
	}
	// 睡蓮パッド 8px。縦の一帯（scanline 88..95）にだけ非ゼロ。
	pad := []byte{
		0x3C, // 00111100
		0x7E, // 01111110
		0xFF, 0xFF, 0xFF, 0xFF,
		0x7E,
		0x3C,
	}
	grp0 := make([]byte, 192)
	const padTop = 88
	for i, b := range pad {
		grp0[padTop+i] = b
	}
	src, err := playfield.GenerateMonetSpriteASM(bg, grp0, 0x03, 0xC8, 0xF0) // NUSIZ=3コピー, 緑, 右1px/frame
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/monet_sprite.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (Monet water + flowing lily sprite)\n", out)
}

// buildMonetScene は Monet 睡蓮シーンを返す（静止/アニメで共有）。
// art=40列の有機的・非対称パッド散布、water=per-row 水色の wash（巡回パレットにも流用）、lily=定数緑。
func buildMonetScene() (art []string, water []byte, lily byte) {
	const rows = 48
	seed := uint32(0x2600)
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed >> 16 }

	water = make([]byte, rows)
	hues := []byte{0x70, 0x80, 0x90, 0x50, 0x60, 0xB0, 0xC0} // blue..violet..cyan-green..green
	for y := 0; y < rows; y++ {
		hue := hues[(y/3+int(next()%2))%len(hues)]
		lum := byte(4 + (next()%5)*2) // 4..12
		water[y] = hue | lum
	}
	lily = 0xC8 // 睡蓮パッド（定数 COLUPF・緑）

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
	art = make([]string, rows)
	for y := range grid {
		art[y] = string(grid[y])
	}
	return art, water, lily
}

// genMonetAnim は Monet 静止画を「水面きらめき」アニメ化（M2 ステップ1）→ roms/monet_anim.asm。
func genMonetAnim() {
	art, water, lily := buildMonetScene()
	src, err := playfield.GenerateAsymmetricShimmerASM(art, water, lily,
		playfield.SceneOpts{LineHeight: 4, Speed: 5})
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	const out = "roms/monet_anim.asm"
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (asymmetric Monet, shimmer speed 5)\n", out)
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

// genMonetM1 は非対称 Monet 静止画 → roms/monet_m1.asm。
func genMonetM1() {
	art, water, lily := buildMonetScene()
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
	fmt.Printf("wrote %s (asymmetric Monet still, %d rows)\n", out, len(art))
}
