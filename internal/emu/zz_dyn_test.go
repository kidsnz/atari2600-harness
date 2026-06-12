package emu

import (
	"fmt"
	"testing"
)

func TestZZDyn(t *testing.T) {
	e, _ := New("NTSC")
	if err := e.LoadROM("../../roms/techniques/dyn_multisprite.bin"); err != nil {
		t.Fatal(err)
	}
	bad := 0
	for f := 0; f < 30; f++ {
		lines, _ := e.StepFrame()
		if f >= 2 && lines != 262 {
			bad++
			if bad < 3 {
				fmt.Printf("frame %d lines=%d\n", f, lines)
			}
		}
	}
	fmt.Println("non-262:", bad)
}
