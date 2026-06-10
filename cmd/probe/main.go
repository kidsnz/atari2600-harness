// Command probe は MCP に包む前の配管検証 CLI。Gopher2600 を自前 Go から headless
// 駆動できることを「数値で」確認する（鉄則: 判定はスクショでなく数値）。
//
//	go run ./cmd/probe [rom.bin]   (default: roms/litmus/smoke.bin)
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

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
	rom := "roms/litmus/smoke.bin"
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

	annotated := e.Annotated(3)
	writePNG("bin/frame_annotated.png", annotated)

	fmt.Printf("Snapshot    : %dx%d  visibleTop=%d  → bin/frame.png\n", b.Dx(), b.Dy(), visTop)
	fmt.Printf("Annotated   : %dx%d  → bin/frame_annotated.png\n",
		annotated.Bounds().Dx(), annotated.Bounds().Dy())
}
