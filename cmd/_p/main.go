// Command _p renders a technojacket cover ROM to a PNG: either a single picture
// frame, or a side-by-side pair (left = a frame between kicks, right = a frame at
// the head of a kick, where the beat-synced glitch is supposed to be live).
//
// Two things this tool gets right that the first version did not, both of them
// measured rather than assumed:
//
//  1. FRAME CHOICE. The picture does not exist yet at frame 1. Measured on
//     technojacket-cover-dither.bin, the mean luminance of the picture band is
//     6.00 at frame 1 and 52.39 from frame 7 on, so a tool that grabs frame 1
//     writes a uniformly near-black PNG. The old version selected its "clean"
//     frame by looking for gmask ($9B) in {$00,$FF} and took the first such
//     frame -- but gmask reads $00 at EVERY frame boundary in every one of the
//     eight cover ROMs, so that test picked frame 1 always, and additionally
//     never found a "dirty" frame at all. The trigger here is the kick envelope
//     index k0i ($83) instead, which does vary: it runs 1,2,3..13 on a 29-frame
//     cycle. Frames are also warmed up before either is taken.
//
//  2. THE PICTURE BAND. The captured frame is 160x214 and the 192 picture lines
//     sit at y=8..199 -- 8 rows above and 14 below. Cropping at height-192 is two
//     rows low.
//
// The snapshot copying the old version did was NOT a bug: emu.Snapshot already
// returns an independent image (capture.snapshot draws into a fresh RGBA), and a
// src-vs-copy measurement showed identical mean/max and distinct backing arrays.
//
// Usage:
//
//	_p [-single] [-scale N] <rom.bin> <out.png>
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const (
	// Picture band inside the 160x214 capture.
	bandTop = 8
	bandH   = 192
	bandW   = 160

	// RAM addresses (zero-page $80..$FF).
	addrK0i = 0x83 // kick envelope index; the glitch is live while this is < 3

	warmup = 20 // frames to let the picture and the music settle
	search = 80 // frames to look through for the two frames we want
)

// grab runs the ROM and returns (clean, glitch) pictures plus the frame numbers
// they came from. clean is a frame with the kick envelope well past its head;
// glitch is the first frame with k0i < 3.
func grab(rom string) (clean, glitch *image.RGBA, cf, gf int, err error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, nil, 0, 0, err
	}
	if err := e.LoadROM(rom); err != nil {
		return nil, nil, 0, 0, err
	}
	for f := 1; f <= warmup+search; f++ {
		if _, err := e.StepFrame(); err != nil {
			return nil, nil, 0, 0, err
		}
		if f <= warmup {
			continue
		}
		ram, err := e.CurrentRAM()
		if err != nil {
			return nil, nil, 0, 0, err
		}
		k := ram[addrK0i-0x80]
		img, _ := e.Snapshot() // already an independent copy
		switch {
		case k < 3 && glitch == nil:
			glitch, gf = img, f
		case k >= 6 && clean == nil:
			clean, cf = img, f
		}
		if clean != nil && glitch != nil {
			break
		}
	}
	if clean == nil {
		return nil, nil, 0, 0, fmt.Errorf("%s: no frame with k0i>=6 in %d frames", rom, warmup+search)
	}
	return clean, glitch, cf, gf, nil
}

// blit copies the picture band of src into dst at (xo,0), scaled up by s.
func blit(dst *image.RGBA, src *image.RGBA, xo, s int) {
	for y := 0; y < bandH*s; y++ {
		for x := 0; x < bandW*s; x++ {
			dst.Set(xo+x, y, src.RGBAAt(x/s, bandTop+y/s))
		}
	}
}

// diffPixels counts pixels that differ between the picture bands of a and b.
func diffPixels(a, b *image.RGBA) int {
	n := 0
	for y := bandTop; y < bandTop+bandH; y++ {
		for x := 0; x < bandW; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

func main() {
	single := flag.Bool("single", false, "write only the between-kicks frame")
	scale := flag.Int("scale", 2, "integer upscale factor")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: _p [-single] [-scale N] <rom.bin> <out.png>")
		os.Exit(2)
	}
	rom, outPath := flag.Arg(0), flag.Arg(1)
	s := *scale

	clean, glitch, cf, gf, err := grab(rom)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	const gap = 8
	var out *image.RGBA
	if *single || glitch == nil {
		out = image.NewRGBA(image.Rect(0, 0, bandW*s, bandH*s))
		draw.Draw(out, out.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
		blit(out, clean, 0, s)
		fmt.Printf("%s  single frame=%d\n", outPath, cf)
	} else {
		out = image.NewRGBA(image.Rect(0, 0, bandW*s*2+gap, bandH*s))
		draw.Draw(out, out.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
		blit(out, clean, 0, s)
		blit(out, glitch, bandW*s+gap, s)
		fmt.Printf("%s  left=frame %d (between kicks)  right=frame %d (kick head)  diff=%d px\n",
			outPath, cf, gf, diffPixels(clean, glitch))
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := png.Encode(f, out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
