// framesim — VV-12 tolerant frame comparison (docs/capability-gap-audit.md).
//
// Compares two frames by structural similarity (SSIM) and perceptual hash
// distance — "how wrong, and where" — complementing the exact golden_frame
// check. Each side is a ROM (.bin, rendered after -frames frames) or a PNG.
//
//	go run ./cmd/framesim -a cand.bin -b ref.bin -frames 8
//	go run ./cmd/framesim -a cand.bin -b golden.png
//
// Exit 1 if SSIM falls below -min (a tolerant regression gate); 0 otherwise.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/framesim"
)

func loadFrame(path, spec string, frames int) (*image.RGBA, error) {
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, err
		}
		if rgba, ok := img.(*image.RGBA); ok {
			return rgba, nil
		}
		b := img.Bounds()
		rgba := image.NewRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				rgba.Set(x, y, img.At(x, y))
			}
		}
		return rgba, nil
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(path); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	img, _ := e.Snapshot()
	return img, nil
}

func main() {
	a := flag.String("a", "", "frame A: ROM .bin or .png (required)")
	b := flag.String("b", "", "frame B: ROM .bin or .png (required)")
	frames := flag.Int("frames", 8, "frames to render for .bin inputs")
	spec := flag.String("spec", "NTSC", "TV spec")
	min := flag.Float64("min", 0, "fail (exit 1) if SSIM mean < this")
	flag.Parse()
	if *a == "" || *b == "" {
		fmt.Fprintln(os.Stderr, "usage: framesim -a x.bin -b y.bin [-frames 8] [-min 0.95]")
		os.Exit(2)
	}

	ia, err := loadFrame(*a, *spec, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load %s: %v\n", *a, err)
		os.Exit(2)
	}
	ib, err := loadFrame(*b, *spec, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load %s: %v\n", *b, err)
		os.Exit(2)
	}

	// Normalize scale so a 1× ROM render and a 2× screenshot compare instead of
	// erroring (the common size is the per-axis min — downscale only).
	na, nb, sz := framesim.NormalizeSize(ia, ib)
	ss, ok := framesim.SSIM(na, nb)
	if !ok {
		fmt.Fprintf(os.Stderr, "ERROR: cannot compare frames (normalized to %dx%d)\n", sz.X, sz.Y)
		os.Exit(2)
	}
	ham := framesim.HammingDistance(framesim.PHash(na), framesim.PHash(nb))

	out := struct {
		A          string  `json:"a"`
		B          string  `json:"b"`
		Normalized string  `json:"normalized"`
		SSIMMean   float64 `json:"ssim_mean"`
		SSIMWorst  float64 `json:"ssim_worst"`
		WorstBlock string  `json:"worst_block"`
		PHashHam   int     `json:"phash_hamming"`
	}{*a, *b, fmt.Sprintf("%dx%d", sz.X, sz.Y), ss.Mean, ss.Worst, ss.WorstBlock.String(), ham}
	j, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(j))

	if *min > 0 && ss.Mean < *min {
		fmt.Fprintf(os.Stderr, "FAIL: SSIM %.4f < min %.4f\n", ss.Mean, *min)
		os.Exit(1)
	}
}
