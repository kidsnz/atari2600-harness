// fieldtest — ROM 自走フィールドテスト（R6, v1.34.0）。
// ROM を Gopher2600 で自走させ、複数フレームを採取してマルチフレーム解析（静的/動的分離・
// union トラック・flicker）まで全自動で行う。スクリーンショット不要＝「ROM を inbox に
// 置けば解析一式が出てくる」入力契約 v3 の実体。
//
// 使い方:
//   go run ./cmd/fieldtest -rom game.bin [-warmup 120] [-shots 4] [-gap 2] [-out dir]
//                          [-press right@60,fire@90]   ; フレーム指定の入力注入
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/ingest"
)

func main() {
	rom := flag.String("rom", "", "ROM (.bin)")
	warmup := flag.Int("warmup", 120, "frames to run before the first shot")
	shots := flag.Int("shots", 4, "frames to capture")
	gap := flag.Int("gap", 2, "frames between shots")
	out := flag.String("out", "", "output dir (default: alongside the ROM, <name>_fieldtest/)")
	press := flag.String("press", "", "input injection: action@frame[,action@frame...] (left|right|up|down|fire)")
	flag.Parse()
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: fieldtest -rom game.bin [...]")
		os.Exit(2)
	}
	if err := run(*rom, *warmup, *shots, *gap, *out, *press); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(rom string, warmup, shots, gap int, out, press string) error {
	if out == "" {
		base := strings.TrimSuffix(filepath.Base(rom), filepath.Ext(rom))
		out = filepath.Join(filepath.Dir(rom), base+"_fieldtest")
	}
	type inj struct {
		frame  int
		action string
	}
	var injs []inj
	if press != "" {
		for _, p := range strings.Split(press, ",") {
			parts := strings.SplitN(strings.TrimSpace(p), "@", 2)
			if len(parts) != 2 {
				return fmt.Errorf("bad -press entry %q (want action@frame)", p)
			}
			f, err := strconv.Atoi(parts[1])
			if err != nil {
				return err
			}
			injs = append(injs, inj{f, parts[0]})
		}
	}

	e, err := emu.New("AUTO")
	if err != nil {
		return err
	}
	if err := e.LoadROM(rom); err != nil {
		return err
	}
	q := ingest.NewNTSCQuantizer()
	var frames []*ingest.Normalized
	frame := 0
	step := func(n int) error {
		for i := 0; i < n; i++ {
			for _, ij := range injs {
				if ij.frame == frame {
					// 1 フレーム押して放す（押しっぱなしは action@f を連続指定）
					if err := e.SetInput(0, ij.action, true); err != nil {
						return err
					}
				}
				if ij.frame == frame-1 {
					if err := e.SetInput(0, ij.action, false); err != nil {
						return err
					}
				}
			}
			if err := e.RunFrames(1); err != nil {
				return err
			}
			frame++
		}
		return nil
	}
	if err := step(warmup); err != nil {
		return err
	}
	for s := 0; s < shots; s++ {
		img, _ := e.Snapshot()
		n, err := ingest.Normalize(img, q)
		if err != nil {
			return err
		}
		frames = append(frames, n)
		if s < shots-1 {
			if err := step(gap); err != nil {
				return err
			}
		}
	}

	mr, err := ingest.AnalyzeFrames(frames, q)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	ov, err := os.Create(filepath.Join(out, "overlay.png"))
	if err != nil {
		return err
	}
	if err := png.Encode(ov, ingest.OverlayReport(frames[0], mr.Static, 3)); err != nil {
		ov.Close()
		return err
	}
	ov.Close()
	js, _ := os.Create(filepath.Join(out, "report.json"))
	enc := json.NewEncoder(js)
	enc.SetIndent("", "  ")
	enc.Encode(mr)
	js.Close()
	title := fmt.Sprintf("%s (fieldtest: warmup %d, %d shots, gap %d)", filepath.Base(rom), warmup, shots, gap)
	os.WriteFile(filepath.Join(out, "report.txt"), []byte(ingest.TextReportMulti(mr, title)), 0o644)

	fmt.Printf("fieldtest %s: %d frames, unresolved %.2f%%, union %d objects\n",
		filepath.Base(rom), mr.NumFrames, mr.UnresolvedShare*100, len(mr.Union))
	for i, fr := range mr.Frames {
		fmt.Printf("  frame %d: %d sprites, fidelity %.2f%%\n", i, len(fr.Sprites), fr.Fidelity*100)
	}
	fmt.Println("wrote", out+"/")
	return nil
}
