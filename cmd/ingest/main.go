// ingest — スクリーンショット → TIA データの解析 CLI。
// 使い方: go run ./cmd/ingest -in shot.png -out report_dir/
// 出力: report_dir/overlay.png（TIA 実座標グリッド付き）+ report_dir/report.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"strings"

	"github.com/kidsnz/atari2600-harness/internal/ingest"
)

func main() {
	in := flag.String("in", "", "input screenshot(s), comma-separated for multi-frame (png/jpeg)")
	out := flag.String("out", "ingest_out", "output directory")
	scale := flag.Int("scale", 3, "overlay upscale factor")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: ingest -in shot.png [-out dir]")
		os.Exit(2)
	}
	if err := run(*in, *out, *scale); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(in, out string, scale int) error {
	q := ingest.NewNTSCQuantizer()
	var frames []*ingest.Normalized
	for _, path := range strings.Split(in, ",") {
		f, err := os.Open(strings.TrimSpace(path))
		if err != nil {
			return err
		}
		src, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		n, err := ingest.Normalize(src, q)
		if err != nil {
			return err
		}
		frames = append(frames, n)
	}
	n := frames[0]
	var rep *ingest.Report
	var multi *ingest.MultiReport
	if len(frames) > 1 {
		var err error
		multi, err = ingest.AnalyzeFrames(frames, q)
		if err != nil {
			return err
		}
		rep = multi.Static
		fmt.Printf("multi-frame: %d frames, unresolved %.2f%%\n", multi.NumFrames, multi.UnresolvedShare*100)
		for i, fr := range multi.Frames {
			fmt.Printf("  frame %d: %d sprites, fidelity %.2f%%\n", i, len(fr.Sprites), fr.Fidelity*100)
		}
		for i, u := range multi.Union {
			fl := ""
			if u.Flicker {
				fl = " FLICKER"
			}
			fmt.Printf("  union %d: %s %dx%d seen=%v%s\n", i, u.Kind, u.W, u.H, u.SeenFrames, fl)
		}
	} else {
		rep = ingest.Analyze(n, q)
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	ov, err := os.Create(filepath.Join(out, "overlay.png"))
	if err != nil {
		return err
	}
	defer ov.Close()
	if err := png.Encode(ov, ingest.OverlayReport(n, rep, scale)); err != nil {
		return err
	}
	js, err := os.Create(filepath.Join(out, "report.json"))
	if err != nil {
		return err
	}
	defer js.Close()
	enc := json.NewEncoder(js)
	enc.SetIndent("", "  ")
	if multi != nil {
		if err := enc.Encode(multi); err != nil {
			return err
		}
	} else if err := enc.Encode(rep); err != nil {
		return err
	}

	fmt.Printf("normalized %dx%d (scale %dx%d), avg palette dist %.1f\n",
		rep.Width, rep.Height, rep.ScaleX, rep.ScaleY, rep.AvgPaletteDist)
	for _, w := range rep.Warnings {
		fmt.Println("WARN:", w)
	}
	top := rep.Colors
	if len(top) > 6 {
		top = top[:6]
	}
	for _, c := range top {
		fmt.Printf("  color $%02X (#%s) %.1f%%\n", c.Code, c.Hex, c.Share*100)
	}
	if len(rep.Playfield) > 0 {
		fmt.Printf("  playfield bands: %d\n", len(rep.Playfield))
	}
	for i, s := range rep.Sprites {
		fmt.Printf("  sprite %d: %s x=%d y=%d %dx%d copies=%d\n", i, s.Kind, s.X, s.Y, s.W, s.H, s.Copies)
	}
	fmt.Println("wrote", filepath.Join(out, "overlay.png"), "and report.json")
	return nil
}
