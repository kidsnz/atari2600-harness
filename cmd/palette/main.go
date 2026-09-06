// Command palette writes the machine's own colours out in the formats an artist can load.
//
// ★Why it exists. The drawing side of this project had no palette of its own: the author was
// using one found on the web, of unknown origin. That is the exact hazard the list recorded in
// 2001. Jake Patterson built a Photoshop palette by taking "sixteen screenshots of the colors.bin
// running in Mac Stella", published it, and only then learned that what he had captured included
// luminances the machine cannot make — *"I actually used a couple of the non-spec ones in my little
// demo thing, it was necesssary to get the colors to look right"* 〔stella-list `200109/msg00148`〕.
// The answer came the next day: *"On a real VCS there are only 8 luminance levels. The lowest bit
// isn't used ... If you are seeing more than that, then it must be a bug in the emulator"*
// 〔`200109/msg00159`, Eckhard Stolberg〕. He republished it *"with the odd numbered colors ...
// removed"* 〔`200109/msg00278`〕 — a complete arc of an accident and its repair, twenty-five years
// before this command.
//
// ★★So the palette is MEASURED, not transcribed: `ceiling.HarvestPalette` runs
// `roms/litmus/litmus_palette.bin` and reads back the colour of the pixels the renderer actually
// painted. It is checked against the spec-derived table on every run, and a mismatch is an error
// rather than a warning — if those two disagree, neither is worth handing to an artist.
//
// ★★★The swatch NAMES carry the TIA code ("$1E"), which is the point of the whole exercise: the
// artist picks a colour in Photoshop and the register value is on screen, so "what number is this
// colour" stops being a question anyone has to ask.
//
// Usage:
//
//	go run ./cmd/palette -out DIR [-spec NTSC]
//
// Writes TIA-<spec>-128.aco (Photoshop swatches, named), .act (raw 256×RGB, no names), .txt
// (code R G B) and .png (a chart, 16 hues × 8 luminances).
//
// Found by the mailing-list distillation (helper-2), who located the 2001 thread and noticed that
// this repository's ingest side is already safe (`internal/ingest` uses the 128 EVEN codes, and
// `pkg/design/color_test.go` machine-locks D0 as invalid) while the drawing side had nothing.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"unicode/utf16"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/kidsnz/atari2600-harness/internal/ceiling"
)

func main() {
	out := flag.String("out", "", "directory to write into (required)")
	spec := flag.String("spec", "NTSC", "television specification")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: palette -out DIR [-spec NTSC]")
		os.Exit(2)
	}
	if err := run(*out, *spec); err != nil {
		fmt.Fprintln(os.Stderr, "palette:", err)
		os.Exit(1)
	}
}

func run(dir, spec string) error {
	measured, err := ceiling.HarvestPalette(ceiling.PaletteROM, spec)
	if err != nil {
		return fmt.Errorf("harvest: %w", err)
	}
	derived, err := ceiling.PaletteFor(spec)
	if err != nil {
		return fmt.Errorf("derive: %w", err)
	}
	// ★Two independent paths to the same table. Handing an artist a palette that only one of them
	// agrees with would be handing them a guess.
	for i := 0; i < ceiling.PaletteSize; i++ {
		if measured.Colors[i] != derived.Colors[i] {
			return fmt.Errorf("code $%02X: the palette measured by rendering (%v) and the one "+
				"derived from the specification (%v) disagree — neither is safe to hand to an "+
				"artist until that is explained", measured.Code(i), measured.Colors[i],
				derived.Colors[i])
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, "TIA-"+spec+"-128")

	var txt []byte
	for i := 0; i < ceiling.PaletteSize; i++ {
		c := measured.Colors[i]
		txt = append(txt, []byte(fmt.Sprintf("%02X %3d %3d %3d\n", measured.Code(i), c[0], c[1], c[2]))...)
	}
	if err := os.WriteFile(base+".txt", txt, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(base+".aco", aco(measured), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(base+".act", act(measured), 0o644); err != nil {
		return err
	}
	if err := writePNG(base+".png", measured, spec); err != nil {
		return err
	}

	distinct := map[[3]int]bool{}
	for i := 0; i < ceiling.PaletteSize; i++ {
		distinct[measured.Colors[i]] = true
	}
	fmt.Printf("wrote %s.{aco,act,txt,png} — 128 codes, %d distinct colours "+
		"(the machine has 8 luminances, so only the EVEN codes exist)\n", base, len(distinct))
	return nil
}

// aco writes an Adobe Color Swatch file: version 1 (colours) followed by version 2 (colours with
// names), which is what gives every swatch its TIA code in the Photoshop panel.
func aco(p ceiling.Palette) []byte {
	var b []byte
	put16 := func(v int) { var t [2]byte; binary.BigEndian.PutUint16(t[:], uint16(v)); b = append(b, t[:]...) }
	body := func(named bool) {
		for i := 0; i < ceiling.PaletteSize; i++ {
			c := p.Colors[i]
			put16(0) // RGB
			put16(c[0] * 257)
			put16(c[1] * 257)
			put16(c[2] * 257)
			put16(0)
			if named {
				name := fmt.Sprintf("$%02X", p.Code(i))
				u := utf16.Encode([]rune(name))
				var t [4]byte
				binary.BigEndian.PutUint32(t[:], uint32(len(u)+1))
				b = append(b, t[:]...)
				for _, r := range u {
					put16(int(r))
				}
				put16(0)
			}
		}
	}
	put16(1)
	put16(ceiling.PaletteSize)
	body(false)
	put16(2)
	put16(ceiling.PaletteSize)
	body(true)
	return b
}

// act writes the raw 256×RGB table. It carries no names, so it is the fallback rather than the
// point; the remaining 128 entries are zero because the machine has no colours to put there.
func act(p ceiling.Palette) []byte {
	b := make([]byte, 768)
	for i := 0; i < ceiling.PaletteSize; i++ {
		c := p.Colors[i]
		b[i*3], b[i*3+1], b[i*3+2] = byte(c[0]), byte(c[1]), byte(c[2])
	}
	return b
}

// writePNG draws the chart: 16 hue rows by 8 luminance columns, each cell labelled with its code.
func writePNG(path string, p ceiling.Palette, spec string) error {
	const cw, ch, pad, top, left = 96, 74, 2, 34, 46
	w := left + 8*(cw+pad) + 12
	h := top + 16*(ch+pad) + 12
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(r image.Rectangle, c color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Set(x, y, c)
			}
		}
	}
	fill(img.Bounds(), color.RGBA{24, 24, 26, 255})
	label := func(x, y int, s string, c color.RGBA) {
		d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13,
			Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	grey := color.RGBA{150, 150, 155, 255}
	label(12, 20, "Atari 2600 "+spec+" - 128 codes, measured from the machine (litmus_palette + HarvestPalette)",
		color.RGBA{235, 235, 235, 255})
	for lum := 0; lum < 8; lum++ {
		label(left+lum*(cw+pad)+4, top-8, fmt.Sprintf("lum %d", lum), grey)
	}
	for hue := 0; hue < 16; hue++ {
		y := top + hue*(ch+pad)
		label(8, y+ch/2, fmt.Sprintf("%X_", hue), grey)
		for lum := 0; lum < 8; lum++ {
			i := (hue*16 + lum*2) / 2
			c := p.Colors[i]
			x := left + lum*(cw+pad)
			fill(image.Rect(x, y, x+cw, y+ch), color.RGBA{uint8(c[0]), uint8(c[1]), uint8(c[2]), 255})
			// ★Ink chosen by luminance so the code stays readable on both ends of the ramp; a
			// chart whose labels vanish on the bright rows is a chart of half the palette.
			ink := color.RGBA{255, 255, 255, 255}
			if (c[0]*299+c[1]*587+c[2]*114)/1000 > 128 {
				ink = color.RGBA{0, 0, 0, 255}
			}
			label(x+6, y+18, fmt.Sprintf("$%02X", p.Code(i)), ink)
			label(x+6, y+ch-8, fmt.Sprintf("%d %d %d", c[0], c[1], c[2]), ink)
		}
	}
	_ = draw.Src
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
