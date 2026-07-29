package vismatch

import (
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// The generated table has to equal what the ROM actually puts in the TIA.
//
// Checking the generator against its own decoding would agree with itself no
// matter how wrong the bit order or the half-mapping were. The independent
// oracle is the machine: at a given scanline the PF0/PF1/PF2 registers hold
// definite values, and a table derived from the rendered picture must reproduce
// them exactly. Bit order, column width, which half PF2 covers, and the
// LSB-first/MSB-first inversion between PF1 and PF2 are all wrong-able, and all
// of them show up here.
//
// Only repeat/reflect ROMs are compared: an asymmetric kernel rewrites PF within
// the line, so "the register value at this scanline" is not one number and the
// comparison would be meaningless rather than failing.
func TestGeneratedPFMatchesTIARegisters(t *testing.T) {
	files, err := filepath.Glob("../../roms/techniques/*.asm")
	if err != nil || len(files) == 0 {
		t.Skip("technique corpus unavailable")
	}
	romsChecked, linesChecked, skipped, unreadable := 0, 0, 0, 0
	for _, asm := range files {
		bin := build.BinPathFor(asm)
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Logf("assemble %s: %s", asm, out)
			continue
		}
		g, mode, bands, ok := measureROM(bin)
		if !ok || mode == PFAsymmetric || len(bands) == 0 {
			continue
		}

		// Re-run and sample the registers at the end of each measured scanline.
		e, err := emu.New("NTSC")
		if err != nil {
			continue
		}
		if err := e.LoadROM(bin); err != nil {
			continue
		}
		if err := e.RunFrames(4); err != nil {
			continue
		}
		byLine := map[int]PFBand{}
		for _, b := range bands {
			for sl := b.ScanlineLo; sl <= b.ScanlineHi; sl++ {
				byLine[sl] = b
			}
		}
		// One pass down the frame, sampling the registers at each scanline. Stepping
		// instructions to hunt one line at a time costs a whole frame whenever the
		// beam has already passed it, which thinned the comparison to a handful of
		// lines per ROM.
		mismatch := 0
		for {
			c := e.Coords()
			if c.Scanline >= g.Top+g.H {
				break
			}
			if b, want := byLine[c.Scanline]; want {
				// The register snapshot is only the value that DREW this line if the
				// kernel did not rewrite PF during it. Sample early and late and compare
				// only when they agree; a kernel that writes the next line's playfield
				// part-way through this one (a two-line kernel does) would otherwise be
				// judged against a value that never touched these pixels.
				early, okE := pfAt(e, c.Scanline, 4)
				late, okL := pfAt(e, c.Scanline, 150)
				if !okE || !okL {
					// The line ended before both samples could be taken — a WSYNC in the
					// middle of it. Counted, not silently dropped: an invisible skip is
					// how a test ends up proving nothing while reporting a pass.
					unreadable++
					delete(byLine, c.Scanline)
					if err := e.StepScanline(); err != nil {
						break
					}
					continue
				}
				if early != late {
					skipped++
					delete(byLine, c.Scanline)
					if err := e.StepScanline(); err != nil {
						break
					}
					continue
				}
				linesChecked++
				if early.PF0 != b.PF0 || early.PF1 != b.PF1 || early.PF2 != b.PF2 {
					mismatch++
					if mismatch <= 3 {
						t.Errorf("%s scanline %d: generated PF0/1/2 = %02X %02X %02X, "+
							"the machine held %02X %02X %02X all line",
							filepath.Base(asm), c.Scanline, b.PF0, b.PF1, b.PF2,
							early.PF0, early.PF1, early.PF2)
					}
				}
				delete(byLine, c.Scanline)
			}
			if err := e.StepScanline(); err != nil {
				break
			}
		}
		romsChecked++
	}
	if linesChecked == 0 {
		t.Fatal("no scanline was compared against the machine — the test proves nothing")
	}
	t.Logf("generated PF bytes matched the TIA registers on %d scanlines across %d repeat/reflect ROMs "+
		"(%d skipped: PF changed within the line so the snapshot is not what drew it; "+
		"%d unreadable: a WSYNC ended the line before both samples)",
		linesChecked, romsChecked, skipped, unreadable)
}

// measureROM renders a frame and measures its playfield across the visible area.
func measureROM(bin string) (*Grid, PFMode, []PFBand, bool) {
	g, err := ExtractROM(bin, "NTSC", 4, false)
	if err != nil || g == nil || g.H == 0 {
		return nil, "", nil, false
	}
	bands, mode := MeasurePF(g, g.Top, g.Top+g.H-1)
	return g, mode, bands, true
}

// pfAt advances within the given scanline to the given clock and returns the
// playfield registers there.
func pfAt(e *emu.Emu, sl, clk int) (emu.PlayfieldRegs, bool) {
	for i := 0; i < 200; i++ {
		c := e.Coords()
		if c.Scanline != sl {
			return emu.PlayfieldRegs{}, false
		}
		if c.Clock >= clk {
			return e.ReadTIARegisters().Playfield, true
		}
		if err := e.StepInstruction(); err != nil {
			return emu.PlayfieldRegs{}, false
		}
	}
	return emu.PlayfieldRegs{}, false
}
