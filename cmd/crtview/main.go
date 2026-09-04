// crtview — render a ROM the way a television roughly would, for LOOKING at artwork.
//
// Not a measurement tool and not evidence: see internal/crt's package comment. The pixel-exact
// paths (vismatch, read_row, visual_ceiling, get_screen_annotated) remain the authority on whether
// a ROM is correct. This exists because all of them see output sharper than any console produced,
// and the 2600's own style leans on the blur.
//
//	go run ./cmd/crtview -rom roms/.../x.bin -out /tmp/x.png [-frames 2] [-luma 1] [-chroma 3]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/crt"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func main() {
	rom := flag.String("rom", "", "ROM to render (.bin)")
	out := flag.String("out", "", "PNG to write (optional if -ansi)")
	ansi := flag.Int("ansi", 0, "also print to the terminal as coloured half-blocks, this many columns wide (0 = off)")
	aspect := flag.Float64("aspect", 1.74, "horizontal stretch for -ansi, since a 2600 pixel is wide (see design-principles)")
	spec := flag.String("spec", "NTSC", "TV spec")
	warm := flag.Int("warmup", 6, "frames to run before capturing")
	nf := flag.Int("frames", 2, "frames to average (phosphor persistence; 2 shows a flicker as a blend)")
	luma := flag.Int("luma", 1, "horizontal blur half-width for brightness, in source pixels")
	chroma := flag.Int("chroma", 3, "horizontal blur half-width for colour (must be >= -luma)")
	flag.Parse()
	if *rom == "" || (*out == "" && *ansi == 0) {
		flag.Usage()
		os.Exit(2)
	}

	e, err := emu.New(*spec)
	check(err)
	check(e.LoadROM(*rom))
	check(e.RunFrames(*warm))

	var frames []*image.RGBA
	for i := 0; i < *nf; i++ {
		if _, err := e.StepFrame(); err != nil {
			check(err)
		}
		img, _ := e.Snapshot()
		cp := image.NewRGBA(img.Bounds())
		copy(cp.Pix, img.Pix)
		frames = append(frames, cp)
	}

	persisted, err := crt.Persist(frames)
	check(err)
	bled, err := crt.Bleed(persisted, *luma, *chroma)
	check(err)

	if *ansi > 0 {
		printANSI(bled, *ansi, *aspect)
	}
	if *out != "" {
		f, err := os.Create(*out)
		check(err)
		defer f.Close()
		check(png.Encode(f, bled))
		b := bled.Bounds()
		fmt.Printf("wrote %s (%dx%d, %d frames averaged, blur luma=%d chroma=%d)\n",
			*out, b.Dx(), b.Dy(), *nf, *luma, *chroma)
	}
	fmt.Println("this is an APPROXIMATION for looking at, not a measurement — see internal/crt")
}

// printANSI draws the frame with half-block characters and 24-bit colour, two image rows per text
// row. This exists because the author works in a terminal and is sometimes remote: `open` reaches a
// Mac that may not be in front of them, and an inline image does not reach a terminal at all. The
// escape sequence is emitted only when the colour pair changes, which matters — the naive form
// produced 102 KB for one 160x214 frame and was truncated before it could be seen.
func printANSI(img *image.RGBA, cols int, aspect float64) {
	b := img.Bounds()
	// crop to the rows that are not entirely background, then scale
	bg := img.RGBAAt(b.Min.X, b.Min.Y)
	top, bot := b.Max.Y, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != bg {
				if y < top {
					top = y
				}
				if y > bot {
					bot = y
				}
				break
			}
		}
	}
	if top > bot {
		top, bot = b.Min.Y, b.Max.Y-1
	}
	srcW, srcH := b.Dx(), bot-top+1
	rows := int(float64(srcH) * (float64(cols) / (float64(srcW) * aspect)))
	rows -= rows % 2
	if rows < 2 {
		rows = 2
	}
	at := func(cx, cy int) color.RGBA {
		sx := b.Min.X + cx*srcW/cols
		sy := top + cy*srcH/rows
		return img.RGBAAt(sx, sy)
	}
	fmt.Printf("%dx%d cells, aspect %.2f applied — an APPROXIMATION, not a measurement\n", cols, rows/2, aspect)
	for y := 0; y < rows; y += 2 {
		var sb strings.Builder
		prev := ""
		for x := 0; x < cols; x++ {
			t, bo := at(x, y), at(x, y+1)
			key := fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm", t.R, t.G, t.B, bo.R, bo.G, bo.B)
			if key != prev {
				sb.WriteString(key)
				prev = key
			}
			sb.WriteString("▀")
		}
		fmt.Println(sb.String() + "\033[0m")
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "crtview:", err)
		os.Exit(1)
	}
}
