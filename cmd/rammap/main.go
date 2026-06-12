// rammap — ROM の RAM 使用マップを自動監査する（V2-18, U-M13）。
// N フレーム実行しながらフレーム毎に RAM ($80-$FF) を差分採取し、
// 「どの番地が・何回・どの値域で変化したか」を Markdown 表で出力する。
// docs/ram-maps.md の自動生成元（実ゲームの変数マップ調査・自作 ROM の RAM 監査の両方に）。
//
//	go run ./cmd/rammap -rom game.bin [-frames 300] [-warmup 10]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	rom := flag.String("rom", "", "ROM (.bin)")
	frames := flag.Int("frames", 300, "frames to observe")
	warmup := flag.Int("warmup", 10, "frames before observation")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: rammap -rom game.bin [-frames 300]")
		os.Exit(2)
	}
	if err := run(*rom, *warmup, *frames); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(rom string, warmup, frames int) error {
	e, err := emu.New("AUTO")
	if err != nil {
		return err
	}
	if err := e.LoadROM(rom); err != nil {
		return err
	}
	if err := e.RunFrames(warmup); err != nil {
		return err
	}
	read := func() ([128]uint8, error) {
		var r [128]uint8
		for i := 0; i < 128; i++ {
			v, err := e.PeekRAM(uint16(0x80 + i))
			if err != nil {
				return r, err
			}
			r[i] = v
		}
		return r, nil
	}
	prev, err := read()
	if err != nil {
		return err
	}
	var changes [128]int
	var minV, maxV [128]uint8
	for i := range minV {
		minV[i], maxV[i] = prev[i], prev[i]
	}
	for f := 0; f < frames; f++ {
		if err := e.RunFrames(1); err != nil {
			return err
		}
		cur, err := read()
		if err != nil {
			return err
		}
		for i := 0; i < 128; i++ {
			if cur[i] != prev[i] {
				changes[i]++
			}
			if cur[i] < minV[i] {
				minV[i] = cur[i]
			}
			if cur[i] > maxV[i] {
				maxV[i] = cur[i]
			}
		}
		prev = cur
	}
	fmt.Printf("# RAM map — %s (%d frames after %d warmup)\n\n", filepath.Base(rom), frames, warmup)
	fmt.Println("| addr | changes/frame | range | note |")
	fmt.Println("|---|---|---|---|")
	used := 0
	for i := 0; i < 128; i++ {
		if changes[i] == 0 && minV[i] == maxV[i] && minV[i] == 0 {
			continue // 終始 0 = 未使用とみなす
		}
		used++
		note := ""
		switch {
		case changes[i] == 0:
			note = "constant (init only)"
		case changes[i] >= frames:
			note = "every frame (counter/timer?)"
		}
		fmt.Printf("| $%02X | %.2f | $%02X-$%02X | %s |\n",
			0x80+i, float64(changes[i])/float64(frames), minV[i], maxV[i], note)
	}
	fmt.Printf("\n%d/128 bytes in use.\n", used)
	return nil
}
