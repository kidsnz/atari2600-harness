// Command calibrate は横位置の式 X(N) を掃引実測でフィットする（B-4）。
// 協調 ROM（litmus_pos: DELAY=$80, 5 CPU サイクル/ユニット）の遅延を poke で振り、
// player0 ResetPixel を read_tia で測って傾き(px/cycle)とオフセットを数値で復元する。
//
//	go run ./cmd/calibrate [rom.bin] [lo] [hi]
//	  default: roms/litmus/litmus_pos.bin 2 14
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/kidsnz/atari2600-dev/internal/calibrate"
	"github.com/kidsnz/atari2600-dev/internal/emu"
)

const cyclesPerUnit = 5 // litmus_pos の SBC(2)+BCS(3) ループ

func main() {
	rom := "roms/litmus/litmus_pos.bin"
	lo, hi := 2, 14
	if len(os.Args) > 1 {
		rom = os.Args[1]
	}
	if len(os.Args) > 3 {
		lo, _ = strconv.Atoi(os.Args[2])
		hi, _ = strconv.Atoi(os.Args[3])
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

	pts, err := calibrate.Sweep(e, 0x80, lo, hi)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sweep:", err)
		os.Exit(1)
	}
	res, err := calibrate.Fit(pts, cyclesPerUnit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fit:", err)
		os.Exit(1)
	}

	fmt.Printf("ROM            : %s\n", rom)
	fmt.Printf("DELAY sweep    : %d..%d  (%d CPU cycles/unit)\n", lo, hi, cyclesPerUnit)
	fmt.Println("DELAY -> X (player0 ResetPixel):")
	for _, p := range res.Points {
		fmt.Printf("   %2d -> %3d\n", p.Delay, p.X)
	}
	fmt.Printf("slope/unit     : %.4f px  (expect ~15)\n", res.SlopePerUnit)
	fmt.Printf("slope/cycle    : %.4f px/CPU-cycle  (real-hw authority = 3)\n", res.SlopePerCycle)
	fmt.Printf("intercept X    : %.2f  (unwrapped X at DELAY=0)\n", res.InterceptX)
	fmt.Printf("R^2            : %.6f  (1.0 = perfectly linear)\n", res.R2)
}
