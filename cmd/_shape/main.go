package main

import (
	"fmt"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	e, err := emu.New("NTSC")
	if err != nil { panic(err) }
	if err := e.LoadROM(os.Args[1]); err != nil { panic(err) }
	if err := e.RunFrames(6); err != nil { panic(err) }
	for sl := 36; sl <= 60; sl++ {
		runs, _, err := e.DecomposeRow(sl)
		if err != nil { continue }
		row := make([]byte, 160)
		for i := range row { row[i] = '.' }
		for _, r := range runs {
			ch := byte('.')
			switch r.Element {
			case "P0": ch = '0'
			case "P1": ch = '1'
			}
			for i := 0; i < r.Len; i++ {
				if r.Clock+i < 160 { row[r.Clock+i] = ch }
			}
		}
		fmt.Printf("sl%3d |%s|\n", sl, string(row[80:140]))
	}
}
