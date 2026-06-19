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
	diffOut := flag.String("diff", "", "also write a per-pixel diff image here (red=A-only, blue=B-only) and report differing row-bands")
	align := flag.Bool("align", false, "crop both to lit-content bbox before comparing (align wall-to-wall, not by frame edges) — for ROM vs screenshot with different margins")
	up := flag.Bool("up", false, "rescale to the common MAX (upscale the low-res side) instead of min — keeps a hi-res screenshot's sharp edges so the diff tracks real structure, not downscale blur")
	spans := flag.Bool("spans", false, "the per-element ruler: print each content-aligned row's lit clock-spans for A and B (native precision, no resize), marking rows that differ. Use to measure exact element bboxes (net/score/ball/paddle/wall) target-vs-yours.")
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

	// -spans: the exact per-element ruler. Print each content-aligned row's lit
	// clock-spans for A and B (measured natively, no resize), marking diffs. Stands
	// alone (no SSIM) — this is for "where exactly does each element sit, target vs mine".
	if *spans {
		printSpans(*a, ia, *b, ib)
		return
	}

	// A height mismatch makes the resized -align diff ±1-row sensitive (false full-row
	// diffs at the top/bottom edge); say so and point at -spans for exact work.
	if *align {
		ha, hb := framesim.ContentBBox(ia).Dy(), framesim.ContentBBox(ib).Dy()
		if ha != hb {
			d := ha - hb
			if d < 0 {
				d = -d
			}
			fmt.Fprintf(os.Stderr, "WARN: content heights differ (A=%d B=%d) — the resized -align diff carries ~%dpx of edge noise; use -spans for exact per-element measurement.\n", ha, hb, d)
		}
	}

	// Normalize so a 1× ROM render and a 2× screenshot compare instead of erroring.
	// -align also crops to lit content first (wall-to-wall), absorbing differing
	// VBLANK/overscan margins; otherwise just scale-normalize (downscale to min).
	var na, nb *image.RGBA
	var sz image.Point
	switch {
	case *align && *up:
		na, nb, sz = framesim.NormalizeAlignedUp(ia, ib)
	case *align:
		na, nb, sz = framesim.NormalizeAligned(ia, ib)
	case *up:
		na, nb, sz = framesim.NormalizeSizeMax(ia, ib)
	default:
		na, nb, sz = framesim.NormalizeSize(ia, ib)
	}
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

	if *diffOut != "" {
		dimg, ds, _ := framesim.Diff(na, nb)
		if f, e := os.Create(*diffOut); e == nil {
			_ = png.Encode(f, dimg)
			f.Close()
		}
		fmt.Printf("diff: %d mismatched px (A-only/red=%d, B-only/blue=%d) bbox=%v -> %s\n",
			ds.Mismatch, ds.AOnly, ds.BOnly, ds.BBox, *diffOut)
		inBand, start := false, 0
		for y := 0; y <= ds.H; y++ {
			hot := y < ds.H && ds.RowMiss[y] > 2
			if hot && !inBand {
				inBand, start = true, y
			} else if !hot && inBand {
				inBand = false
				sum := 0
				for r := start; r < y; r++ {
					sum += ds.RowMiss[r]
				}
				fmt.Printf("  rows %3d-%3d (%d): %d diff px\n", start, y-1, y-start, sum)
			}
		}
	}

	if *min > 0 && ss.Mean < *min {
		fmt.Fprintf(os.Stderr, "FAIL: SSIM %.4f < min %.4f\n", ss.Mean, *min)
		os.Exit(1)
	}
}

func fmtSpans(s []framesim.Span) string {
	if len(s) == 0 {
		return " (none)"
	}
	out := ""
	for _, sp := range s {
		out += fmt.Sprintf(" clk[%d-%d]", sp.Lo, sp.Hi)
	}
	return out
}

// printSpans tabulates each content-aligned row's lit clock-spans for A and B,
// marking rows whose spans differ — the exact per-element ruler.
func printSpans(aName string, a *image.RGBA, bName string, b *image.RGBA) {
	ha, hb := framesim.ContentBBox(a).Dy(), framesim.ContentBBox(b).Dy()
	fmt.Printf("spans (content-aligned, clock 0..159 · A=%s rows=%d · B=%s rows=%d):\n", aName, ha, bName, hb)
	n := ha
	if hb < n {
		n = hb
	}
	diff := 0
	for ar := 0; ar < n; ar++ {
		sa, sb := framesim.ContentRowSpans(a, ar), framesim.ContentRowSpans(b, ar)
		mark := ""
		if !framesim.SpansEqual(sa, sb) {
			mark, diff = "   <-- differ", diff+1
		}
		fmt.Printf("  ar %3d: A%-28s | B%s%s\n", ar, fmtSpans(sa), fmtSpans(sb), mark)
	}
	fmt.Printf("summary: %d/%d content rows differ", diff, n)
	if ha != hb {
		fmt.Printf(" (heights differ A=%d B=%d — extra rows beyond %d not shown)", ha, hb, n)
	}
	fmt.Println()
}
