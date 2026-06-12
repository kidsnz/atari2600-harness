// fieldtest — ROM 自走フィールドテスト（R6→S1 で v2）。
// ROM を Gopher2600 で自走させ、複数フレームを採取してマルチフレーム解析まで全自動。
//
//	go run ./cmd/fieldtest -rom game.bin [-warmup N -shots K -gap G] [-auto]
//	                        [-press right@60,fire@90,reset@30] [-out dir]
//	go run ./cmd/fieldtest -inbox ../inbox       ; 整理モード（*.bin → 各フォルダ＋解析一式）
//
// -auto: 動的オブジェクトが採れるまで RESET → fire → fire+右入力保持 の順に試す
// （タイトル画面・アトラクト・待機キャラ対策。何で開始できたかを報告）。
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
	warmup := flag.Int("warmup", 120, "frames before the first shot")
	shots := flag.Int("shots", 4, "frames to capture")
	gap := flag.Int("gap", 3, "frames between shots")
	out := flag.String("out", "", "output dir (default: <romdir>/<name>_fieldtest)")
	press := flag.String("press", "", "input injection: action@frame[,...] (left|right|up|down|fire|reset|select)")
	auto := flag.Bool("auto", false, "auto-start: escalate RESET/fire/hold-right until dynamic objects appear")
	inboxDir := flag.String("inbox", "", "batch: organize <dir>/*.bin into per-ROM folders and analyze each")
	flag.Parse()
	if *inboxDir != "" {
		if err := runInbox(*inboxDir, *warmup, *shots, *gap); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		return
	}
	if *rom == "" {
		fmt.Fprintln(os.Stderr, "usage: fieldtest -rom game.bin [...] | -inbox dir")
		os.Exit(2)
	}
	if err := run(*rom, *warmup, *shots, *gap, *out, *press, *auto); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

// runInbox: 直下の各 X.bin → X/ フォルダへ移動し、その中へ解析一式を出力（散らかり防止の標準構造）。
func runInbox(dir string, warmup, shots, gap int) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	n := 0
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".bin") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		sub := filepath.Join(dir, base)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		dst := filepath.Join(sub, name)
		if err := os.Rename(filepath.Join(dir, name), dst); err != nil {
			return err
		}
		if asm := base + ".asm"; fileExists(filepath.Join(dir, asm)) {
			os.Rename(filepath.Join(dir, asm), filepath.Join(sub, asm))
		}
		fmt.Printf("== %s ==\n", base)
		if err := run(dst, warmup, shots, gap, sub, "", true); err != nil {
			fmt.Printf("  analysis failed: %v (ROM organized into %s/)\n", err, sub)
		}
		n++
	}
	fmt.Printf("organized %d ROM(s) under %s\n", n, dir)
	return nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func run(rom string, warmup, shots, gap int, out, press string, auto bool) error {
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
	frame := 0
	apply := func(action string, pressed bool) error {
		if action == "reset" || action == "select" {
			return e.SetPanel(action, pressed)
		}
		return e.SetInput(0, action, pressed)
	}
	step := func(n int) error {
		for i := 0; i < n; i++ {
			for _, ij := range injs {
				if ij.frame == frame {
					if err := apply(ij.action, true); err != nil {
						return err
					}
				}
				if ij.frame == frame-1 {
					if err := apply(ij.action, false); err != nil {
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
	capture := func(holdRight bool) (*ingest.MultiReport, []*ingest.Normalized, error) {
		var fs []*ingest.Normalized
		if holdRight {
			apply("right", true)
			defer apply("right", false)
		}
		for s := 0; s < shots; s++ {
			img, _ := e.Snapshot()
			n, err := ingest.Normalize(img, q)
			if err != nil {
				return nil, nil, err
			}
			fs = append(fs, n)
			if s < shots-1 {
				if err := step(gap); err != nil {
					return nil, nil, err
				}
			}
		}
		mr, err := ingest.AnalyzeFrames(fs, q)
		return mr, fs, err
	}
	pulse := func(action string) error {
		if err := apply(action, true); err != nil {
			return err
		}
		if err := step(2); err != nil {
			return err
		}
		if err := apply(action, false); err != nil {
			return err
		}
		return step(40)
	}

	if err := step(warmup); err != nil {
		return err
	}
	mr, frames, err := capture(false)
	if err != nil {
		return err
	}
	startedBy := "as-is"
	if auto && len(mr.Union) == 0 {
		for _, at := range []struct {
			name, action string
			hold         bool
		}{{"reset", "reset", false}, {"fire", "fire", false}, {"fire+hold-right", "fire", true}} {
			if err := pulse(at.action); err != nil {
				return err
			}
			mr2, fs2, err := capture(at.hold)
			if err != nil {
				return err
			}
			if len(mr2.Union) > 0 {
				mr, frames = mr2, fs2
				startedBy = at.name
				break
			}
			startedBy = "no-dynamics"
		}
	}
	if auto {
		fmt.Printf("  auto-start: %s (union=%d)\n", startedBy, len(mr.Union))
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
	title := fmt.Sprintf("%s (fieldtest: warmup %d, %d shots, gap %d, start=%s)",
		filepath.Base(rom), warmup, shots, gap, startedBy)
	os.WriteFile(filepath.Join(out, "report.txt"), []byte(ingest.TextReportMulti(mr, title)), 0o644)

	fmt.Printf("fieldtest %s: %d frames, unresolved %.2f%%, union %d objects\n",
		filepath.Base(rom), mr.NumFrames, mr.UnresolvedShare*100, len(mr.Union))
	for i, fr := range mr.Frames {
		fmt.Printf("  frame %d: %d sprites, fidelity %.2f%%\n", i, len(fr.Sprites), fr.Fidelity*100)
	}
	fmt.Println("wrote", out+"/")
	return nil
}
