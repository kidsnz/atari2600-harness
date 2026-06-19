package framesim

import (
	"image"
	"image/color"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// patternImage builds a deterministic w x h test image with structure.
func patternImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*7 + y*13) & 0xFF)
			img.Set(x, y, color.RGBA{v, uint8(255 - int(v)), uint8((x ^ y) & 0xFF), 255})
		}
	}
	return img
}

func clone(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func invert(src *image.RGBA) *image.RGBA {
	dst := clone(src)
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 255 - dst.Pix[i]
		dst.Pix[i+1] = 255 - dst.Pix[i+1]
		dst.Pix[i+2] = 255 - dst.Pix[i+2]
	}
	return dst
}

// TestNormalizeSizeRescales locks the scale-normalization that lets a 1× ROM frame
// compare to a 2× screenshot: an image vs its clean 2× upscale, normalized to the
// common (min) size, scores SSIM ~1.0 instead of erroring on a bounds mismatch.
func TestNormalizeSizeRescales(t *testing.T) {
	orig := patternImage(32, 24)
	up := Resize(orig, 64, 48) // 2× nearest-neighbor, as a 2× screenshot would be
	na, nb, sz := NormalizeSize(orig, up)
	if sz.X != 32 || sz.Y != 24 {
		t.Fatalf("common size = %v, want 32x24 (per-axis min)", sz)
	}
	ss, ok := SSIM(na, nb)
	if !ok {
		t.Fatal("SSIM should succeed after NormalizeSize")
	}
	if ss.Mean < 0.99 {
		t.Errorf("an image vs its own 2× upscale should score ~1.0 after normalize, got %.4f", ss.Mean)
	}
	if _, _, s := NormalizeSize(orig, clone(orig)); s.X != 32 || s.Y != 24 {
		t.Errorf("equal-size inputs must pass through unchanged, got %v", s)
	}
}

// TestDiffLocalizes locks the diff localizer: identical frames score 0 mismatch;
// a lit block over black is reported as B-only and localized to its rows.
func TestDiffLocalizes(t *testing.T) {
	a := patternImage(32, 24)
	if _, ds, ok := Diff(a, clone(a)); !ok || ds.Mismatch != 0 {
		t.Fatalf("identical frames should have 0 mismatch, got %d", ds.Mismatch)
	}
	black := image.NewRGBA(image.Rect(0, 0, 32, 24))
	lit := clone(black)
	for y := 10; y < 14; y++ {
		for x := 5; x < 9; x++ {
			lit.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	_, ds, _ := Diff(black, lit) // black=A, lit=B → 16 B-only px in rows 10-13
	if ds.AOnly != 0 || ds.BOnly != 16 {
		t.Errorf("expected 0 A-only / 16 B-only, got AOnly=%d BOnly=%d", ds.AOnly, ds.BOnly)
	}
	if ds.RowMiss[11] != 4 || ds.RowMiss[0] != 0 {
		t.Errorf("mismatch should localize to rows 10-13 (4/row), got RowMiss[11]=%d RowMiss[0]=%d", ds.RowMiss[11], ds.RowMiss[0])
	}
}

// perturb flips the luma of the first k pixels to mid-grey.
func perturb(src *image.RGBA, k int) *image.RGBA {
	dst := clone(src)
	for i := 0; i < k && i*4+2 < len(dst.Pix); i++ {
		dst.Pix[i*4] = 128
		dst.Pix[i*4+1] = 128
		dst.Pix[i*4+2] = 128
	}
	return dst
}

// TestSSIMIdentityAndTolerance: identical => 1.0; a 1-pixel change stays ~1
// (tolerant where exact-hash would flip); a wholesale change scores far lower.
func TestSSIMIdentityAndTolerance(t *testing.T) {
	a := patternImage(64, 64)

	id, ok := SSIM(a, clone(a))
	if !ok || id.Mean < 0.9999 {
		t.Fatalf("identical SSIM mean = %.6f, want ~1.0", id.Mean)
	}

	one, _ := SSIM(a, perturb(a, 1))
	if one.Mean < 0.99 {
		t.Errorf("1-pixel change SSIM = %.4f, want tolerant (>0.99)", one.Mean)
	}
	if one.Mean >= 1.0 {
		t.Errorf("1-pixel change SSIM = %.6f, must be < 1 (it IS different)", one.Mean)
	}

	inv, _ := SSIM(a, invert(a))
	if inv.Mean > 0.5 {
		t.Errorf("inverted SSIM = %.4f, want low (<0.5)", inv.Mean)
	}
	// magnitude monotonicity: more corruption => lower SSIM
	light, _ := SSIM(a, perturb(a, 16))
	heavy, _ := SSIM(a, perturb(a, 64*32))
	if heavy.Mean >= light.Mean {
		t.Errorf("SSIM not monotonic in damage: heavy %.4f >= light %.4f", heavy.Mean, light.Mean)
	}
}

// TestSSIMLocality: the worst block must localise where the damage is.
func TestSSIMLocality(t *testing.T) {
	a := patternImage(64, 64)
	b := clone(a)
	// corrupt a known 8x8 region near (40,48)
	for y := 48; y < 56; y++ {
		for x := 40; x < 48; x++ {
			b.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	res, _ := SSIM(a, b)
	if !res.WorstBlock.Overlaps(image.Rect(40, 48, 48, 56)) {
		t.Errorf("worst block %v did not localise the (40,48) damage", res.WorstBlock)
	}
}

func TestSSIMSizeMismatch(t *testing.T) {
	if _, ok := SSIM(patternImage(64, 64), patternImage(64, 32)); ok {
		t.Fatal("SSIM should reject mismatched sizes")
	}
}

// TestPHashDistance: identical => 0; inverted/different => large.
func TestPHashDistance(t *testing.T) {
	a := patternImage(80, 64)
	if d := HammingDistance(PHash(a), PHash(clone(a))); d != 0 {
		t.Errorf("identical pHash distance = %d, want 0", d)
	}
	if d := HammingDistance(PHash(a), PHash(invert(a))); d < 10 {
		t.Errorf("inverted pHash distance = %d, want large (>=10)", d)
	}
}

// TestRealFrames: a frame is identical to itself; two different ROMs' frames
// are measurably less similar.
func TestRealFrames(t *testing.T) {
	cap := func(rom string) *image.RGBA {
		e, err := emu.New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(8); err != nil {
			t.Fatal(err)
		}
		img, _ := e.Snapshot()
		return img
	}
	mc := cap("../../roms/techniques/multicolor48.bin")
	sm := cap("../../roms/litmus/smoke.bin")

	self, ok := SSIM(mc, clone(mc))
	if !ok || self.Mean < 0.9999 {
		t.Fatalf("self SSIM = %.6f, want ~1", self.Mean)
	}
	cross, ok := SSIM(mc, sm)
	if !ok {
		t.Fatal("cross SSIM size mismatch between ROM frames")
	}
	if cross.Mean >= self.Mean {
		t.Errorf("different ROMs SSIM %.4f not below self %.4f", cross.Mean, self.Mean)
	}
	if HammingDistance(PHash(mc), PHash(sm)) == 0 {
		t.Error("different ROM frames must not share a pHash")
	}
}
