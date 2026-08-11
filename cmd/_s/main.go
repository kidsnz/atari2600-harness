package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM(os.Args[1]); err != nil { panic(err) }
	for i := 0; i < 60; i++ { e.StepFrame() }
	const s = 2
	var shots []*image.RGBA
	prev := uint8(255)
	for f := 0; f < 900 && len(shots) < 7; f++ {
		e.StepFrame()
		spi, _ := e.PeekRAM(0x8C)
		k0i, _ := e.PeekRAM(0x83)
		if spi != prev && k0i < 3 {
			img, _ := e.Snapshot()
			c := image.NewRGBA(image.Rect(0, 0, 160, 192))
			for y := 0; y < 192; y++ {
				for x := 0; x < 160; x++ { c.Set(x, y, img.RGBAAt(x, 8+y)) }
			}
			shots = append(shots, c)
		}
		prev = spi
	}
	out := image.NewRGBA(image.Rect(0, 0, (160*s+6)*len(shots), 192*s))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.RGBA{40, 40, 40, 255}), image.Point{}, draw.Src)
	for i, c := range shots {
		xo := i * (160*s + 6)
		for y := 0; y < 192*s; y++ {
			for x := 0; x < 160*s; x++ { out.Set(xo+x, y, c.RGBAAt(x/s, y/s)) }
		}
	}
	f, _ := os.Create(os.Args[2]); png.Encode(f, out); f.Close()
	fmt.Printf("%s  %d 枚\n", os.Args[2], len(shots))
}
