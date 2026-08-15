// Command still renders a ROM to a PNG, choosing WHICH FRAME by the machine's own
// state rather than by an index: either one picture, or a side-by-side pair taken
// at two different values of a chosen RAM byte (a beat-synced glitch, an animation
// phase, a mode flag).
//
// The reason it is a tool and not a one-liner is that both of the obvious ways to
// grab a frame are wrong, and both were shipped to a user before being measured.
//
// Two things this tool gets right that the first version did not, both of them
// measured rather than assumed:
//
//  1. FRAME CHOICE. The picture does not exist yet at frame 1. Measured on a
//     technojacket cover ROM, the mean luminance of the picture band is 6.00 at
//     frame 1 and 52.39 from frame 7 on, so a tool that grabs frame 1 writes a
//     uniformly near-black PNG -- which is exactly what the user was shown.
//     Picking the frame by a RAM byte is not automatically better: the first
//     version watched a byte that reads $00 at EVERY frame boundary, so it
//     selected frame 1 every time and never found its second frame at all.
//     A trigger byte has to be one that MOVES across frame boundaries; -trigger
//     names it and -lo/-hi are the two values to catch, so the choice is stated
//     and can be checked.
//
//  2. THE PICTURE BAND. The captured frame is 160x214 and the 192 picture lines
//     sit at y=8..199 -- 8 rows above and 14 below. Cropping at height-192 is two
//     rows low.
//
// The snapshot copying the old version did was NOT a bug: emu.Snapshot already
// returns an independent image (capture.snapshot draws into a fresh RGBA), and a
// src-vs-copy measurement showed identical mean/max and distinct backing arrays.
//
// -frame N covers the case the trigger mechanism cannot: a ROM with NO moving
// state. A still picture has no phase to select, so there is no byte that moves
// and -trigger has nothing to watch -- it fails, correctly, with "pick a trigger
// byte that actually moves". Naming the frame outright is the right answer there,
// but it reopens trap 1, so -frame carries its own guard: the picture band's
// luminance SPREAD is measured and a frame with nothing on it is REFUSED with that
// number, rather than written out as a black PNG. See bandStats for why the spread
// and not the brightness -- the obvious measure fails here, and was tried.
//
// Usage:
//
//	still [-single] [-scale N] [-trigger 0x83] [-lo 3] [-hi 6] <rom.bin> <out.png>
//	still -frame N [-scale N] <rom.bin> <out.png>
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const (
	// Picture band inside the 160x214 capture.
	bandTop = 8
	bandH   = 192
	bandW   = 160

	warmup = 20 // frames to let the picture and the music settle
	search = 80 // frames to look through for the two frames we want
)

// grab runs the ROM and returns (clean, glitch) pictures plus the frame numbers
// they came from. clean is a frame with the kick envelope well past its head;
// glitch is the first frame with k0i < 3.
func grab(rom string, trigger, lo, hi int) (clean, glitch *image.RGBA, cf, gf int, err error) {
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
		k := int(ram[trigger-0x80])
		img, _ := e.Snapshot() // already an independent copy
		switch {
		case k < lo && glitch == nil:
			glitch, gf = img, f
		case k >= hi && clean == nil:
			clean, cf = img, f
		}
		if clean != nil && glitch != nil {
			break
		}
	}
	if clean == nil {
		// Saying "no frame matched" beats writing whatever frame happened to be
		// last: a near-black PNG is indistinguishable from a ROM that draws nothing.
		return nil, nil, 0, 0, fmt.Errorf("%s: no frame where RAM $%02X >= %d in %d frames -- "+
			"pick a trigger byte that actually moves", rom, trigger, hi, warmup+search)
	}
	return clean, glitch, cf, gf, nil
}

// bandStats returns the mean and the standard deviation of the 192 picture lines'
// luminance, on the same 0..255 scale the PNG is written at.
//
// THE DEVIATION IS THE ONE THAT ANSWERS THE QUESTION, and the mean was tried first.
// "Did we grab the picture or the frame before it existed" looks like a brightness
// question, and this file's own package comment quotes a mean of 6.00 for the
// undrawn frame against 52.39 for the picture. But 6.00 is not that ROM's number:
// frame 1 of litmus_pf_allcols reads 6.00 too, and so does frame 1 of litmus_48px.
// It is the emulator's undrawn frame, the same value for every ROM, so a mean floor
// set under it passes exactly the frame it was written to catch, and a floor set
// over it would reject any genuinely dark picture.
//
// An undrawn frame is UNIFORM, and that is what distinguishes it from a dark one.
// Measured: frame 1 reads sd 0.00 on both ROMs above, against 52.49 and 39.58 once
// the picture is there. A picture of one flat colour would also read 0.00 and would
// also be refused -- correctly, since nothing is on it.
func bandStats(img *image.RGBA) (mean, sd float64) {
	n := float64(bandH * bandW)
	var sum, sumsq float64
	for y := bandTop; y < bandTop+bandH; y++ {
		for x := 0; x < bandW; x++ {
			c := img.RGBAAt(x, y)
			l := (float64(c.R) + float64(c.G) + float64(c.B)) / 3
			sum += l
			sumsq += l * l
		}
	}
	mean = sum / n
	v := sumsq/n - mean*mean
	if v < 0 {
		v = 0
	}
	return mean, math.Sqrt(v)
}

// blankFloor is the luminance standard deviation below which a frame is refused --
// far enough above the measured 0.00 of an undrawn frame to survive a rounding
// difference, far enough below the measured 43.79 of a real one to be nowhere near it.
const blankFloor = 0.5

// grabFrame runs the ROM to exactly frame n and returns that frame. Unlike grab it
// asks the machine nothing -- which is the point, because a still picture has no
// state to ask about -- so the check that the picture is actually there has to be
// made on the pixels.
func grabFrame(rom string, n int) (*image.RGBA, error) {
	img, err := grabFrameUnguarded(rom, n)
	if err != nil {
		return nil, err
	}
	if mean, sd := bandStats(img); sd < blankFloor {
		return nil, fmt.Errorf("%s: frame %d's picture band is UNIFORM (luminance sd %.2f, mean %.2f) -- "+
			"nothing is on it. This is the frame-1 trap, where the picture is not drawn yet, and it "+
			"reads the same for every ROM. Try a later frame", rom, n, sd, mean)
	}
	return img, nil
}

// grabFrameUnguarded is grabFrame without the blank check, so a test can look at the
// frame the guard exists to reject.
func grabFrameUnguarded(rom string, n int) (*image.RGBA, error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(rom); err != nil {
		return nil, err
	}
	for f := 1; f <= n; f++ {
		if _, err := e.StepFrame(); err != nil {
			return nil, err
		}
	}
	img, _ := e.Snapshot() // already an independent copy
	return img, nil
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
//
// READ THIS AS COLOUR PLUS GEOMETRY, NEVER AS GEOMETRY. On the technojacket covers
// COLUPF follows the drum envelope in EVERY build, glitched or not, so the clean
// control still reports 6136 differing pixels between its two frames. A raw pixel
// diff therefore cannot tell "the picture changed shape" from "the picture changed
// brightness", and the per-element attribution in emu.DecomposeRow is what answers
// the first question. This number is a sanity check on the capture, not a measure of
// an effect.
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
	single := flag.Bool("single", false, "write only the -hi frame")
	scale := flag.Int("scale", 2, "integer upscale factor")
	// These three used to default to 0x83 / 3 / 6, which is TECHNOJACKET's kick-envelope index
	// (k0i = $83, roms/260809_technojacket/src/prologue.asm). "Which byte selects the frame" is a
	// property of the ROM being rendered and cannot have a default; the old one silently picked
	// another work's variable, and the tool only notices when that byte never moves -- not when
	// it moves and means something else. Required since 2026-08-15 (unless -frame is used).
	trigger := flag.Int("trigger", -1, "zero-page RAM address whose value selects the frames (REQUIRED unless -frame; technojacket's is 0x83)")
	lo := flag.Int("lo", 3, "the right-hand frame is the first with RAM[trigger] < this")
	hi := flag.Int("hi", 6, "the left-hand frame is the first with RAM[trigger] >= this")
	frame := flag.Int("frame", 0, "render exactly this frame instead of choosing one by -trigger, "+
		"for a ROM with no moving state; a blank frame is refused, not written")
	flag.Parse()
	usingFrame := *frame > 0
	if flag.NArg() != 2 || *frame < 0 || (!usingFrame && (*trigger < 0x80 || *trigger > 0xFF)) ||
		(usingFrame && *trigger != -1 && (*trigger < 0x80 || *trigger > 0xFF)) {
		fmt.Fprintln(os.Stderr, "usage: still -trigger 0xNN [-lo N] [-hi N] [-single] [-scale N] <rom.bin> <out.png>")
		fmt.Fprintln(os.Stderr, "       still -frame N [-scale N] <rom.bin> <out.png>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "-trigger has NO default. It names the zero-page byte whose value selects the")
		fmt.Fprintln(os.Stderr, "frame, which is a property of the ROM. It defaulted to 0x83 until 2026-08-15;")
		fmt.Fprintln(os.Stderr, "that is technojacket's kick-envelope index and means nothing in another ROM.")
		os.Exit(2)
	}
	rom, outPath := flag.Arg(0), flag.Arg(1)
	s := *scale

	if *frame > 0 {
		img, err := grabFrame(rom, *frame)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		out := image.NewRGBA(image.Rect(0, 0, bandW*s, bandH*s))
		blit(out, img, 0, s)
		if err := writePNG(outPath, out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		mean, sd := bandStats(img)
		fmt.Printf("%s  frame=%d  band luminance mean %.2f sd %.2f\n", outPath, *frame, mean, sd)
		return
	}

	clean, glitch, cf, gf, err := grab(rom, *trigger, *lo, *hi)
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
		fmt.Printf("%s  left=frame %d ($%02X>=%d)  right=frame %d ($%02X<%d)  diff=%d px (colour+geometry)\n",
			outPath, cf, *trigger, *hi, gf, *trigger, *lo, diffPixels(clean, glitch))
	}

	if err := writePNG(outPath, out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
