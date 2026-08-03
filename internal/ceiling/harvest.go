package ceiling

import (
	"fmt"
	"image"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// PaletteROM is the sweep cartridge the empirical harvest runs: 4 white marker
// scanlines, then 128 scanlines of COLUBK = $00,$02,..,$FE, one colour per line.
// Source: roms/litmus/litmus_palette.asm (written 2026-06-12 for exactly this).
const PaletteROM = "roms/litmus/litmus_palette.bin"

// HarvestPalette MEASURES the renderer's palette instead of asking it: it runs
// litmus_palette.bin, snapshots a frame, and reads one colour off each of the
// 128 sweep scanlines. This is the empirical twin of PaletteFor, and the two are
// asserted equal by TestHarvestedPaletteEqualsDerivedPalette.
//
// Both exist on purpose. PaletteFor is cheap and is what the metric uses;
// HarvestPalette is the thing that can catch a renderer change PaletteFor would
// silently follow into the wrong answer — e.g. a gamma/colour-generation change
// applied on the way to the framebuffer but not in Spec.GetColor. Whichever way
// such a change went, the equality test would go red.
//
// romPath is the path to litmus_palette.bin (usually PaletteROM, resolved
// relative to the harness root).
func HarvestPalette(romPath, spec string) (Palette, error) {
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return Palette{}, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return Palette{}, err
	}
	if err := e.RunFrames(8); err != nil {
		return Palette{}, err
	}
	img, _ := e.Snapshot()

	// The ROM's first four kernel lines are white ($0E) so the sweep can be
	// located without assuming where VBLANK ended. Find that marker band rather
	// than trusting a VisibleTop constant.
	want, err := PaletteFor(spec)
	if err != nil {
		return Palette{}, err
	}
	white := want.Colors[0x0E>>1]

	b := img.Bounds()
	markerTop := -1
	for y := b.Min.Y; y+4+PaletteSize <= b.Max.Y; y++ {
		if uniformRow(img, y) == white && uniformRow(img, y+1) == white &&
			uniformRow(img, y+2) == white && uniformRow(img, y+3) == white {
			markerTop = y
			break
		}
	}
	if markerTop < 0 {
		return Palette{}, fmt.Errorf("ceiling: %s: no 4-line white marker band found in the %dx%d frame — is this litmus_palette.bin?",
			romPath, b.Dx(), b.Dy())
	}

	p := Palette{Spec: spec}
	for i := 0; i < PaletteSize; i++ {
		y := markerTop + 4 + i
		row := uniformRow(img, y)
		if row == ([3]int{-1, -1, -1}) {
			return Palette{}, fmt.Errorf("ceiling: %s: sweep row %d (colour code $%02X) is not a single flat colour", romPath, y, i*2)
		}
		p.Colors[i] = row
	}
	return p, nil
}

// LooksUnrendered reports whether the frame is the CLEARED framebuffer rather
// than a picture — every pixel pure (0,0,0).
//
// This is a measurement, not a guess: internal/emu's capture clears to (0,0,0)
// and paints blanked pixels with Spec.GetColor(signal.ZeroBlack), which on NTSC
// is (6,6,6) — the same colour as code $00. So pure black is a value the
// renderer never writes, and a frame full of it means no frame has been drawn.
// Grading one is not merely useless, it is actively misleading: (0,0,0) is 108
// squared units from the nearest TIA colour, so every rung comes back at rmse
// 6.00 and the ladder looks like a real, flat, slightly-imperfect answer.
func LooksUnrendered(img *image.RGBA) bool {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return false
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i] != 0 || img.Pix[i+1] != 0 || img.Pix[i+2] != 0 {
				return false
			}
		}
	}
	return true
}

// uniformRow returns the row's colour if every pixel in it is the same, and
// {-1,-1,-1} otherwise. A sweep line that is not flat means the ROM is not the
// one we think it is, which must be reported rather than averaged away.
func uniformRow(img *image.RGBA, y int) [3]int {
	b := img.Bounds()
	if y < b.Min.Y || y >= b.Max.Y {
		return [3]int{-1, -1, -1}
	}
	var first [3]int
	for x := b.Min.X; x < b.Max.X; x++ {
		i := img.PixOffset(x, y)
		c := [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
		if x == b.Min.X {
			first = c
		} else if c != first {
			return [3]int{-1, -1, -1}
		}
	}
	return first
}
