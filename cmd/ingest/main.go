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

	"github.com/kidsnz/atari2600-harness/internal/ingest"
)

func main() {
	in := flag.String("in", "", "input screenshot (png/jpeg)")
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
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	q := ingest.NewNTSCQuantizer()
	n, err := ingest.Normalize(src, q)
	if err != nil {
		return err
	}
	rep := ingest.Analyze(n, q)

	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	ov, err := os.Create(filepath.Join(out, "overlay.png"))
	if err != nil {
		return err
	}
	defer ov.Close()
	if err := png.Encode(ov, ingest.Overlay(n, scale)); err != nil {
		return err
	}
	js, err := os.Create(filepath.Join(out, "report.json"))
	if err != nil {
		return err
	}
	defer js.Close()
	enc := json.NewEncoder(js)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
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
