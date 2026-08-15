package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	for _, rom := range os.Args[1:] {
		e, _ := emu.New("NTSC")
		if err := e.LoadROM(rom); err != nil {
			panic(err)
		}
		for i := 0; i < 40; i++ {
			e.StepFrame()
		}
		base, _ := e.Snapshot()
		lines := map[int]int{}
		moved := 0
		for f := 0; f < 300; f++ {
			n, _ := e.StepFrame()
			lines[n]++
			if f == 60 { // one row of scroll would be 4 frames; 60 frames is 15 rows
				img, _ := e.Snapshot()
				for y := 40; y < 160; y++ {
					for x := 0; x < 160; x += 2 {
						if img.RGBAAt(x, y) != base.RGBAAt(x, y) {
							moved++
						}
					}
				}
			}
		}
		fmt.Printf("%-20s フレーム行数 %v / 60フレーム後に動いたpx %d\n",
			strings.TrimSuffix(filepath.Base(rom), ".bin"), lines, moved)
	}
}
