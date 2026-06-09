// Command probe は MCP に包む前の配管検証 CLI。Gopher2600 を自前 Go から headless
// 駆動できることを「数値で」確認する（鉄則: 判定はスクショでなく数値）。
//
//	go run ./cmd/probe [rom.bin]   (default: roms/smoke.bin)
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/kidsnz/atari2600-dev/internal/annotate"
	"github.com/kidsnz/atari2600-dev/internal/emu"
)

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create png:", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, "encode png:", err)
		os.Exit(1)
	}
}

func main() {
	rom := "roms/smoke.bin"
	if len(os.Args) > 1 {
		rom = os.Args[1]
	}

	e, err := emu.New("NTSC")
	if err != nil {
		fmt.Fprintln(os.Stderr, "new:", err)
		os.Exit(1)
	}
	if err := e.LoadROM(rom); err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}

	// 数フレーム回して安定させる
	if err := e.RunFrames(10); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}

	// ちょうど 1 フレーム分ステップして scanline 数を測る（タイミング検証）
	lines, err := e.StepFrame()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stepframe:", err)
		os.Exit(1)
	}

	c := e.Coords()
	cpu := e.VCS.CPU
	sentinel, _ := e.PeekRAM(0x80)

	fmt.Printf("ROM         : %s\n", rom)
	fmt.Printf("Frame       : %d\n", c.Frame)
	fmt.Printf("ScanlinesPF : %d   (expect 262 NTSC)\n", lines)
	fmt.Printf("CPU         : PC=%04X A=%02X X=%02X Y=%02X SP=%04X\n",
		cpu.PC.Value(), cpu.A.Value(), cpu.X.Value(), cpu.Y.Value(), cpu.SP.Address())
	fmt.Printf("RAM[$80]    : $%02X   (expect $42)\n", sentinel)

	// フレーム捕捉（A1）と注釈描画（A3）の目視確認用 PNG
	img, visTop := e.Snapshot()
	b := img.Bounds()
	writePNG("bin/frame.png", img)

	v := e.VCS.TIA.Video
	markers := []annotate.Marker{
		{Label: "P0", Clock: v.Player0.HmovedPixel, Col: color.RGBA{230, 60, 60, 255}},  // 赤
		{Label: "M0", Clock: v.Missile0.HmovedPixel, Col: color.RGBA{235, 140, 40, 255}}, // 橙
		{Label: "P1", Clock: v.Player1.HmovedPixel, Col: color.RGBA{230, 215, 50, 255}},  // 黄
		{Label: "M1", Clock: v.Missile1.HmovedPixel, Col: color.RGBA{70, 200, 70, 255}},  // 緑
		{Label: "BL", Clock: v.Ball.HmovedPixel, Col: color.RGBA{180, 90, 210, 255}},     // 紫
	}
	annotated := annotate.Render(img, visTop, 3, markers)
	writePNG("bin/frame_annotated.png", annotated)

	fmt.Printf("Snapshot    : %dx%d  visibleTop=%d  → bin/frame.png\n", b.Dx(), b.Dy(), visTop)
	fmt.Printf("Annotated   : %dx%d  → bin/frame_annotated.png\n",
		annotated.Bounds().Dx(), annotated.Bounds().Dy())
}
