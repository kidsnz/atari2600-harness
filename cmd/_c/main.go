package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	div := map[uint8]float64{1: 15, 6: 31, 12: 6}
	for _, rom := range os.Args[1:] {
		for _, ch := range []int{0, 1} {
			e, _ := emu.New("NTSC")
			if err := e.LoadROM(rom); err != nil {
				panic(err)
			}
			for i := 0; i < 40; i++ {
				e.StepFrame()
			}
			fmt.Printf("%-20s ch%d ", strings.TrimSuffix(filepath.Base(rom), ".bin"), ch)
			for i := 0; i < 26; i++ {
				e.StepFrame()
				a := e.ReadAudio()
				c := a.Channel0
				if ch == 1 {
					c = a.Channel1
				}
				if c.Volume == 0 {
					fmt.Printf("   .  ")
					continue
				}
				d, ok := div[c.Control]
				if !ok {
					fmt.Printf(" noi%d ", c.Volume)
					continue
				}
				fmt.Printf("%3.0f/%d ", 31440/d/float64(c.Freq+1), c.Volume)
			}
			fmt.Println()
		}
	}
}
