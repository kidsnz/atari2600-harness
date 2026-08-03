// ceiling — what is the BEST the Atari 2600 could do for this picture?
//
// Prints the visual-ceiling LADDER for one frame: for each constraint set (rung),
// the smallest error any kernel working under that set could reach against the
// target, plus the flat-colour reference to normalise against. The DIFFERENCES
// between rungs are the deliverable — C1->C2 answers "what would one sprite buy
// here", C1->C3 answers "how much is the 4-clock column grid costing".
//
//	go run ./cmd/ceiling -target roms/litmus/litmus_pf_allcols.bin
//	go run ./cmd/ceiling -target shot.png -rungs C1,C3 -out ceiling.png -rung-out C1
//
// The number is NOT a score for a kernel. It is the denominator a kernel's own
// error (cmd/vismatch, cmd/framesim) should be read against.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/kidsnz/atari2600-harness/internal/ceiling"
	"github.com/kidsnz/atari2600-harness/internal/emu"
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
		if r, ok := img.(*image.RGBA); ok {
			return r, nil
		}
		b := img.Bounds()
		r := image.NewRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r.Set(x, y, img.At(x, y))
			}
		}
		return r, nil
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
	target := flag.String("target", "", "target frame: ROM .bin (rendered after -frames) or .png (required)")
	frames := flag.Int("frames", 8, "frames to render for a .bin target")
	spec := flag.String("spec", "NTSC", "TV spec (the palette is derived from this renderer spec)")
	rungs := flag.String("rungs", "C1,C2,C3", "comma-separated rungs to compute")
	cols := flag.Int("columns", ceiling.DefaultColumns, "playfield columns")
	out := flag.String("out", "", "write the rendered ceiling picture here (PNG)")
	rungOut := flag.String("rung-out", "C1", "which rung -out renders")
	harvest := flag.Bool("harvest-palette", false, "MEASURE the palette by running roms/litmus/litmus_palette.bin instead of deriving it from the renderer spec (slower; the cross-check)")
	paletteROM := flag.String("palette-rom", ceiling.PaletteROM, "sweep ROM used by -harvest-palette")
	palSpec := flag.String("palette-spec", "", "quantise against THIS spec's palette instead of -spec's. Only useful for showing what a wrong palette does to the numbers: the frames come from -spec's renderer, so anything else here is deliberately wrong.")
	jsonOut := flag.Bool("json", false, "print JSON instead of a table")
	flag.Parse()

	if *target == "" {
		fmt.Fprintln(os.Stderr, "usage: ceiling -target frame.bin|frame.png [-rungs C1,C2,C3] [-out ceiling.png]")
		os.Exit(2)
	}

	quantSpec := *palSpec
	if quantSpec == "" {
		quantSpec = *spec
	}
	var pal ceiling.Palette
	var err error
	if *harvest {
		pal, err = ceiling.HarvestPalette(*paletteROM, quantSpec)
	} else {
		pal, err = ceiling.PaletteFor(quantSpec)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: palette: %v\n", err)
		os.Exit(2)
	}

	img, err := loadFrame(*target, *spec, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load %s: %v\n", *target, err)
		os.Exit(2)
	}

	var want []ceiling.Rung
	for _, r := range strings.Split(*rungs, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			want = append(want, ceiling.Rung(r))
		}
	}

	t0 := time.Now()
	a, err := ceiling.Compute(img, pal, ceiling.Options{Columns: *cols, Rungs: want})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}
	elapsed := time.Since(t0)

	if *out != "" {
		pic, err := a.Render(ceiling.Rung(*rungOut))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: render: %v\n", err)
			os.Exit(2)
		}
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		if err := png.Encode(f, pic); err != nil {
			f.Close()
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		f.Close()
	}

	if *jsonOut {
		j, _ := json.MarshalIndent(a.Result, "", "  ")
		fmt.Println(string(j))
		return
	}

	r := a.Result
	fmt.Printf("ceiling: %s  (%dx%d = %d px, %d distinct scanlines, palette=%s%s, %d columns)\n",
		*target, r.Width, r.Height, r.Pixels, r.UniqueLines, pal.Spec,
		map[bool]string{true: " [harvested]", false: " [derived]"}[*harvest], r.Columns)
	if quantSpec != *spec {
		fmt.Printf("  WARNING: frames rendered as %s but quantised against the %s palette — these numbers are deliberately wrong.\n", *spec, quantSpec)
	}
	fmt.Printf("  %-6s rmse %7.2f   %s\n", "flat", r.Flat.RMSE, r.Flat.Model)
	for _, x := range r.Rungs {
		fmt.Printf("  %-6s rmse %7.2f   %s\n", x.Rung, x.RMSE, x.Model)
	}
	if len(r.Deltas) > 0 {
		fmt.Println("  deltas (the deliverable):")
		for _, d := range r.Deltas {
			fmt.Printf("    %s->%s  %6.2f rmse   %s\n", d.From, d.To, d.RMSEDrop, d.Question)
		}
	}
	fmt.Printf("  computed in %v (worst-line C2 pair candidates: %d of 8256)\n", elapsed.Round(time.Millisecond), a.C2Candidates())
	if *out != "" {
		fmt.Printf("  wrote %s (%s)\n", *out, *rungOut)
	}
}
