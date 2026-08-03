// Package ceiling computes the VISUAL CEILING of a target frame: the best any
// Atari 2600 kernel could reach for that picture under a stated constraint set.
//
// Why it exists. `vismatch` compares a build against another ROM, so when a
// picture is wrong there is no way to separate "the kernel is wrong" from "the
// hardware cannot do this" — the missing-denominator defect the capability audit
// records repeatedly, in visual form. A ceiling supplies the denominator.
//
// Settled by measurement 2026-08-03 (docs/8bitworkshop-crosscheck.md, the two
// Dithertron sections; prototype in the local-only sandbox/experiments/visual-ceiling):
//
//   - A ceiling is a property of (image, CONSTRAINT SET), not of an image. Scoring
//     Chopper Command under a playfield-only ceiling gives a number that says
//     nothing about its kernel, because Chopper does not draw with the playfield
//     alone. A denominator that does not match the constraints the kernel works
//     under is a lie dressed as a percentage.
//   - So the deliverable is a LADDER (C1/C2/C3) and the DIFFERENCES between its
//     rungs, not any single rung. See ceiling.go for the rung definitions.
//   - Detecting the constraint set from the build was REJECTED: it makes the
//     author's own choices the denominator, so the score is high by construction
//     and never says "you left a resource unused".
//
// Source: Dithertron (Steven Hugg, GPL-3.0) supplied the idea and the
// constraint-cell formulation — one whole scanline of 40 playfield columns, two
// colours — and is described in docs/8bitworkshop-crosscheck.md. No Dithertron
// source is used or vendored here; the algorithm is a different one (exhaustive
// per-line optimum, not its iterated histogram-and-diffuse fixpoint).
package ceiling

import (
	"fmt"
	"image/color"

	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/television/specification"
)

// PaletteSize is the number of distinct TIA colours. The colour registers are
// 8-bit but D0 is ignored, so codes $00,$02,..,$FE are the whole set.
const PaletteSize = 128

// Palette is the colour table of the RENDERER THAT PRODUCED THE FRAMES.
//
// This is not a convenience: it is the trap the prototype's self-test caught.
// The first version quantised Gopher2600 frames against internal/ingest's
// measured STELLA palette; 7 of a 14-colour frame's colours were not in that
// table at all, off by up to 40 RGB units — the same order as the signal being
// measured — and a frame that is achievable by construction scored rmse 9.95
// instead of 0. A percentage computed against the wrong palette is noise with a
// decimal point. Nothing here is hardcoded; every entry is asked of the renderer.
type Palette struct {
	Spec   string             // "NTSC" / "PAL" / "PAL60" / "PAL-M" / "SECAM"
	Colors [PaletteSize][3]int // index = TIA colour code >> 1
}

// specByID resolves a TV spec name to Gopher2600's own Spec value — the same
// object internal/emu's capture calls GetColor on when it paints a frame.
func specByID(id string) (specification.Spec, error) {
	switch id {
	case "", "NTSC":
		return specification.SpecNTSC, nil
	case "PAL":
		return specification.SpecPAL, nil
	case "PAL60":
		return specification.SpecPAL60, nil
	case "PAL-M", "PAL_M", "PALM":
		return specification.SpecPAL_M, nil
	case "SECAM":
		return specification.SpecSECAM, nil
	}
	return specification.Spec{}, fmt.Errorf("unknown TV spec %q (want one of %v)", id, specification.SpecList)
}

// PaletteFor derives the palette from the renderer itself: every entry is the
// value Gopher2600's colour generator returns for that colour code, which is
// literally the function internal/emu's capture uses to paint each pixel
// (capture.SetPixels -> frameInfo.Spec.GetColor). Derived, never transcribed.
//
// TestHarvestedPaletteEqualsDerivedPalette proves the two agree by running
// roms/litmus/litmus_palette.bin — a 128-colour sweep, one colour per scanline —
// through the emulator and reading the colours back off the rendered frame.
func PaletteFor(spec string) (Palette, error) {
	s, err := specByID(spec)
	if err != nil {
		return Palette{}, err
	}
	p := Palette{Spec: s.ID}
	for i := 0; i < PaletteSize; i++ {
		c := s.GetColor(signal.ColorSignal(uint8(i * 2)))
		p.Colors[i] = [3]int{int(c.R), int(c.G), int(c.B)}
	}
	return p, nil
}

// RGBA returns entry i as a colour value (for rendering a ceiling image).
func (p *Palette) RGBA(i int) color.RGBA {
	c := p.Colors[i]
	return color.RGBA{uint8(c[0]), uint8(c[1]), uint8(c[2]), 255}
}

// Code returns the TIA colour code (COLUPF/COLUBK value) of entry i.
func (p *Palette) Code(i int) uint8 { return uint8(i * 2) }

// Shifted returns a copy with every channel of every entry moved by delta and
// clamped to 0..255. It exists ONLY to plant the palette defect in a test: the
// measured Stella-vs-Gopher2600 divergence was up to 40 RGB units, so
// Shifted(40) reproduces a wrong palette of the same magnitude as the real one
// and TestPlantedPaletteDefectBreaksTheSelfTest asserts the self-test then fails.
func (p Palette) Shifted(delta int) Palette {
	q := p
	q.Spec = fmt.Sprintf("%s+shift(%d)", p.Spec, delta)
	for i := range q.Colors {
		for c := 0; c < 3; c++ {
			v := q.Colors[i][c] + delta
			if v < 0 {
				v = 0
			}
			if v > 255 {
				v = 255
			}
			q.Colors[i][c] = v
		}
	}
	return q
}
