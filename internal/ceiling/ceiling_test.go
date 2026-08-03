package ceiling

import (
	"image"
	"os"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

const litmusDir = "../../roms/litmus/"

// frameOf renders a ROM and returns its visible frame.
func frameOf(t *testing.T, rom string, frames int) *image.RGBA {
	t.Helper()
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatalf("emu.New: %v", err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Fatalf("LoadROM %s: %v", rom, err)
	}
	if err := e.RunFrames(frames); err != nil {
		t.Fatalf("RunFrames %s: %v", rom, err)
	}
	img, _ := e.Snapshot()
	return img
}

func ntscPalette(t *testing.T) Palette {
	t.Helper()
	p, err := PaletteFor("NTSC")
	if err != nil {
		t.Fatalf("PaletteFor: %v", err)
	}
	return p
}

func ladder(t *testing.T, img *image.RGBA, pal Palette) *Analysis {
	t.Helper()
	a, err := Compute(img, pal, Options{})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return a
}

// playfieldOnlyROMs are the self-test subjects. Each qualifies because its SOURCE
// contains no reference to any movable-object register at all — grepping
// roms/litmus/<name>.asm for GRP0/GRP1/ENAM0/ENAM1/ENABL/RESP0/RESP1/RESM0/RESM1/
// RESBL/COLUP0/COLUP1/NUSIZ returns zero hits in all five — so every pixel each of
// them puts on screen came from PF0/PF1/PF2 against COLUPF/COLUBK. That is C1's
// constraint set exactly, which makes the frame achievable BY CONSTRUCTION and its
// C1 ceiling necessarily zero.
var playfieldOnlyROMs = []string{
	"litmus_pf_allcols.bin",   // 20 bands, one playfield column lit per band
	"litmus_pf.bin",           // columns 0/4/12
	"litmus_pf_async.bin",     // asymmetric playfield (PF written twice per line)
	"score2.bin",              // playfield-drawn score digits, 14 distinct scanlines
	"litmus_title_then_play.bin",
}

// THE SELF-TEST, and it is the reason any of these numbers are worth anything.
// A frame a real 2600 ROM produced is achievable by construction, so it MUST
// score perfectly under a constraint set that describes it. The prototype's
// first version returned 9.95 here because it quantised Gopher2600 frames
// against Stella's palette; 7 of a 14-colour frame's colours were not in that
// table at all, off by up to 40 RGB units. A percentage computed against the
// wrong palette is noise with a decimal point.
//
// The assertion is on the RAW SQUARED ERROR being exactly zero, not on a
// rounded rmse: "0.00" printed to two decimals would also cover an rmse of
// 0.004, which on 34240 pixels is ~1600 units of real error.
func TestPlayfieldOnlyROMsAreExactlyReachableUnderC1(t *testing.T) {
	pal := ntscPalette(t)
	graded := 0
	for _, name := range playfieldOnlyROMs {
		t.Run(name, func(t *testing.T) {
			a := ladder(t, frameOf(t, litmusDir+name, 8), pal)
			var c1 RungResult
			for _, r := range a.Result.Rungs {
				if r.Rung == C1 {
					c1 = r
				}
			}
			if c1.SumSq != 0 {
				t.Errorf("%s: playfield-only frame is achievable by construction, so C1 must be exactly 0; "+
					"got sum_sq=%.0f rmse=%.4f over %d px (%d distinct scanlines). "+
					"The first suspect is the PALETTE: is it the renderer's own?",
					name, c1.SumSq, c1.RMSE, a.Result.Pixels, a.Result.UniqueLines)
			}
			graded++
			t.Logf("%s: C1 sum_sq=%.0f rmse=%.4f  (flat reference rmse=%.2f, %d distinct scanlines)",
				name, c1.SumSq, c1.RMSE, a.Result.Flat.RMSE, a.Result.UniqueLines)
		})
	}
	if graded != len(playfieldOnlyROMs) {
		t.Errorf("self-test graded %d of %d playfield-only ROMs", graded, len(playfieldOnlyROMs))
	}
	t.Logf("self-test denominator: %d in-tree playfield-only ROMs", graded)
}

// BOTH DIRECTIONS. A metric that returned 0 for everything would pass the
// self-test alone, so the self-test is only half a check: content the playfield
// cannot draw must score materially worse.
//
// The subjects are chosen so the reason is legible rather than merely large:
//   - litmus_missile: a missile is ONE colour clock wide and the playfield's
//     finest brush is four. Its C3 (no grid, same two colours per line) is 0.00,
//     so every unit of its C1 error is the column grid and none of it is the
//     colour count. That is the C1->C3 delta doing exactly the job it was
//     designed for.
//   - litmus_nusiz_all: the worst C1 in the whole litmus corpus.
func TestSpriteContentScoresMateriallyWorseUnderC1ThanPlayfieldContent(t *testing.T) {
	pal := ntscPalette(t)

	base := ladder(t, frameOf(t, litmusDir+"litmus_pf_allcols.bin", 8), pal)
	baseC1, _ := base.Result.RMSEOf(C1)

	for _, c := range []struct {
		rom    string
		minC1  float64
		reason string
	}{
		{"litmus_missile.bin", 15, "a 1-clock missile against a 4-clock grid"},
		{"litmus_nusiz_all.bin", 30, "players, copies and sizes — the corpus's worst C1"},
		{"litmus_collide_mp.bin", 20, "missile and player overlapping off-grid"},
	} {
		t.Run(c.rom, func(t *testing.T) {
			a := ladder(t, frameOf(t, litmusDir+c.rom, 8), pal)
			c1, _ := a.Result.RMSEOf(C1)
			c3, _ := a.Result.RMSEOf(C3)
			flat := a.Result.Flat.RMSE
			if c1 <= baseC1 {
				t.Errorf("%s: C1 rmse %.2f is not worse than the playfield-only frame's %.2f", c.rom, c1, baseC1)
			}
			if c1 < c.minC1 {
				t.Errorf("%s: C1 rmse %.2f below the expected floor %.2f (%s) — the metric has gone soft",
					c.rom, c1, c.minC1, c.reason)
			}
			t.Logf("%s: C1=%.2f C3=%.2f flat=%.2f  (playfield-only reference C1=%.2f) — %s",
				c.rom, c1, c3, flat, baseC1, c.reason)
		})
	}
}

// THE PLANTED DEFECT. Quantise against the wrong palette and the self-test must
// break — otherwise the self-test is decoration. Two wrong palettes are planted:
// a real one (PAL's colour generator applied to an NTSC frame — a plausible
// mis-configuration) and a synthetic one shifted by 40 RGB units, which is the
// magnitude actually measured between Stella's palette and Gopher2600's.
func TestPlantedWrongPaletteBreaksTheSelfTest(t *testing.T) {
	good := ntscPalette(t)
	palPAL, err := PaletteFor("PAL")
	if err != nil {
		t.Fatalf("PaletteFor(PAL): %v", err)
	}
	planted := []struct {
		name string
		pal  Palette
	}{
		{"PAL palette on an NTSC frame", palPAL},
		{"NTSC palette shifted +40 RGB", good.Shifted(40)},
		{"NTSC palette shifted -40 RGB", good.Shifted(-40)},
	}

	frames := map[string]*image.RGBA{}
	for _, name := range playfieldOnlyROMs {
		img := frameOf(t, litmusDir+name, 8)
		a := ladder(t, img, good)
		if r, _ := a.Result.RMSEOf(C1); r != 0 {
			t.Fatalf("control: %s must score 0 under the correct palette before a defect means anything, got %.4f", name, r)
		}
		frames[name] = img
	}

	for _, p := range planted {
		detected, blind := 0, []string{}
		for _, name := range playfieldOnlyROMs {
			a, err := Compute(frames[name], p.pal, Options{Rungs: []Rung{C1}})
			if err != nil {
				t.Fatalf("%s / %s: %v", name, p.name, err)
			}
			got, _ := a.Result.RMSEOf(C1)
			if got > 0 {
				detected++
			} else {
				blind = append(blind, name)
			}
			t.Logf("planted %-30s on %-28s -> C1 rmse %8.4f (correct palette: 0.0000)", p.name, name, got)
		}
		if detected == 0 {
			t.Errorf("planted defect %q is INVISIBLE on all %d playfield-only frames — the self-test proves nothing about the palette",
				p.name, len(playfieldOnlyROMs))
		}
		if len(blind) > 0 {
			// Not a failure: a frame drawn only in greys cannot see a chroma-only
			// palette error. Recording which frames are blind to which defect is
			// the honest form of "the self-test has teeth".
			t.Logf("  %q detected on %d/%d frames; blind on %v", p.name, detected, len(playfieldOnlyROMs), blind)
		}
	}
}

// The rendered ceiling must BE the picture whose error was reported. If Render
// drew anything else, the image a reader looks at to decide "the playfield can do
// the scenery and cannot do the actors" would be arguing from a different number
// than the table beside it.
func TestRenderedCeilingReproducesTheReportedError(t *testing.T) {
	pal := ntscPalette(t)
	for _, rom := range []string{"litmus_nusiz_all.bin", "litmus_missile.bin", "litmus_pf_allcols.bin"} {
		img := frameOf(t, litmusDir+rom, 8)
		a := ladder(t, img, pal)
		for _, r := range a.Result.Rungs {
			pic, err := a.Render(r.Rung)
			if err != nil {
				t.Fatalf("%s %s: Render: %v", rom, r.Rung, err)
			}
			got := float64(sumSqDiff(img, pic))
			if got != r.SumSq {
				t.Errorf("%s %s: rendered picture scores %.0f but the ladder reported %.0f", rom, r.Rung, got, r.SumSq)
			}
		}
		t.Logf("%s: all %d rendered rungs match their reported error exactly", rom, len(a.Result.Rungs))
	}
}

func sumSqDiff(a, b *image.RGBA) int64 {
	ba, bb := a.Bounds(), b.Bounds()
	var s int64
	for y := 0; y < ba.Dy(); y++ {
		for x := 0; x < ba.Dx(); x++ {
			i := a.PixOffset(ba.Min.X+x, ba.Min.Y+y)
			j := b.PixOffset(bb.Min.X+x, bb.Min.Y+y)
			for c := 0; c < 3; c++ {
				d := int64(a.Pix[i+c]) - int64(b.Pix[j+c])
				s += d * d
			}
		}
	}
	return s
}

// Ladder invariants. Every rung is an OPTIMUM under a constraint set, and the
// sets nest: a flat line is a special case of C1 (both colours equal), C1 is a
// special case of C2 (the object colour equal to the playfield's), and C1 is a
// special case of C3 (a C3 solution may repeat a colour across four clocks). So
// flat >= C1 >= C2 and flat >= C1 >= C3 must hold for EVERY frame, and a
// violation means an optimiser missed a solution it was supposed to find.
func TestLadderRungsNestAsTheirConstraintSetsDo(t *testing.T) {
	pal := ntscPalette(t)
	roms, err := os.ReadDir(litmusDir)
	if err != nil {
		t.Fatalf("read litmus dir: %v", err)
	}
	graded := 0
	for _, f := range roms {
		if len(f.Name()) < 4 || f.Name()[len(f.Name())-4:] != ".bin" {
			continue
		}
		img := frameOf(t, litmusDir+f.Name(), 8)
		a, err := Compute(img, pal, Options{})
		if err != nil {
			t.Fatalf("%s: %v", f.Name(), err)
		}
		flat := a.Result.Flat.RMSE
		c1, _ := a.Result.RMSEOf(C1)
		c2, _ := a.Result.RMSEOf(C2)
		c3, _ := a.Result.RMSEOf(C3)
		if c1 > flat+1e-9 {
			t.Errorf("%s: C1 %.4f > flat %.4f — a flat line is a C1 solution with both colours equal", f.Name(), c1, flat)
		}
		if c2 > c1+1e-9 {
			t.Errorf("%s: C2 %.4f > C1 %.4f — C1 is a C2 solution with the object colour equal to the playfield's", f.Name(), c2, c1)
		}
		if c3 > c1+1e-9 {
			t.Errorf("%s: C3 %.4f > C1 %.4f — every C1 picture is also a C3 picture", f.Name(), c3, c1)
		}
		graded++
	}
	if graded < 100 {
		t.Errorf("nesting invariant graded only %d ROMs; the litmus corpus has ~112", graded)
	}
	t.Logf("ladder nesting (flat >= C1 >= C2, flat >= C1 >= C3) holds on %d litmus frames", graded)
}

func TestComputeRejectsInputsItCannotGradeHonestly(t *testing.T) {
	pal := ntscPalette(t)
	for _, c := range []struct {
		name string
		img  *image.RGBA
		opts Options
	}{
		{"width not a whole multiple of the column count", image.NewRGBA(image.Rect(0, 0, 163, 10)), Options{}},
		{"empty image", image.NewRGBA(image.Rect(0, 0, 0, 0)), Options{}},
		{"unknown rung", image.NewRGBA(image.Rect(0, 0, 160, 4)), Options{Rungs: []Rung{"C9"}}},
	} {
		if _, err := Compute(c.img, pal, c.opts); err == nil {
			t.Errorf("%s: Compute returned no error", c.name)
		}
	}
	if _, err := PaletteFor("VHS"); err == nil {
		t.Error("PaletteFor accepted a TV spec that does not exist")
	}
}

// The cleared framebuffer must be recognisable as "no frame has been drawn"
// rather than graded. Measured: internal/emu's capture clears to pure (0,0,0)
// while the renderer's own blank is (6,6,6) = colour code $00, so pure black is a
// value no rendered frame contains — and it sits 108 squared units from the
// nearest TIA colour, which makes every rung come back at exactly rmse 6.00. That
// is the most dangerous shape a wrong answer can take: flat, small, and plausible.
func TestClearedFramebufferIsDistinguishableFromAPicture(t *testing.T) {
	cleared := image.NewRGBA(image.Rect(0, 0, 160, 192))
	for i := 3; i < len(cleared.Pix); i += 4 {
		cleared.Pix[i] = 255
	}
	if !LooksUnrendered(cleared) {
		t.Error("a framebuffer of pure black was not recognised as unrendered")
	}
	pal := ntscPalette(t)
	a, err := Compute(cleared, pal, Options{Rungs: []Rung{C1}})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	got, _ := a.Result.RMSEOf(C1)
	if got == 0 {
		t.Error("pure black is not a TIA colour, so grading it must not return 0")
	}
	t.Logf("an ungraded cleared framebuffer scores C1 rmse %.4f on every rung — which is why it is refused, not reported", got)

	real := frameOf(t, litmusDir+"litmus_pf_allcols.bin", 8)
	if LooksUnrendered(real) {
		t.Error("a genuinely rendered frame was reported as unrendered")
	}
}
