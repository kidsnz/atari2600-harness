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
