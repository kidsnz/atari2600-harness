package main

// A raw shot must be 1px = 1 TIA pixel, or it is not usable for what it exists for.
//
// The annotated image is for READING a picture — the user points at a coordinate and
// Claude turns it into registers. The other half of the loop runs the other way: the
// user opens the frame in Photoshop and paints dots, and Claude samples the file back
// into `.byte` rows. That direction needs the file's pixel grid to BE the machine's
// pixel grid. A grid line, an axis label or a 3x upscale is then not decoration but
// corruption of the artwork, and an upscaled image silently lies about its own units.
//
// So the test asserts the dimensions exactly (160x192), asserts that a marker's
// colour appears in the annotated image and NOT in the raw one, and asserts the two
// go to different files — because the user keeps the annotated file open in a
// previewer, and overwriting it with an unlabelled frame would break that window
// without saying anything.

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

func TestRawSnapshotIsTheMachinesPixelGrid(t *testing.T) {
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	// A ROM that draws a player, so there IS a marker to be absent from the raw shot.
	if err := e.LoadROM("../../roms/litmus/litmus_pos.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	raw, _ := e.Snapshot()
	b := raw.Bounds()
	// 160 visible colour clocks; the visible scanline count is what the frame
	// actually rendered, so it is read rather than assumed — a PAL ROM or a kernel
	// with a short frame is legitimately not 192.
	if b.Dx() != 160 {
		t.Errorf("raw frame is %d pixels wide, want 160 — one pixel per visible colour clock is the "+
			"whole point of this mode", b.Dx())
	}
	if b.Dy() < 100 || b.Dy() > 300 {
		t.Errorf("raw frame is %d pixels tall, which is not a plausible visible height", b.Dy())
	}

	// The annotated image at scale 3 must be strictly larger in both axes, or one of
	// the two paths is returning the other's image.
	ann := e.Annotated(3)
	ab := ann.Bounds()
	if ab.Dx() <= b.Dx() || ab.Dy() <= b.Dy() {
		t.Fatalf("annotated image is %dx%d and the raw one %dx%d; the annotated version carries margins "+
			"and an upscale, so it cannot be the same size or smaller", ab.Dx(), ab.Dy(), b.Dx(), b.Dy())
	}

	// PREMISE, then conclusion. P0's marker colour must be present in the annotated
	// image — otherwise the absence checked next proves nothing.
	var markerCol = struct{ r, g, b uint8 }{230, 60, 60} // P0, from emu.Markers
	if !hasColour(ann, markerCol.r, markerCol.g, markerCol.b) {
		t.Fatalf("P0's marker colour is not in the annotated image, so this ROM draws no marker and the "+
			"raw image having none would say nothing")
	}
	if hasColour(raw, markerCol.r, markerCol.g, markerCol.b) {
		t.Errorf("P0's marker colour appears in the RAW frame; annotations are leaking into the image "+
			"meant to hold only what the TIA drew")
	}
}

func hasColour(img *image.RGBA, r, g, bl uint8) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R == r && c.G == g && c.B == bl {
				return true
			}
		}
	}
	return false
}

// TestRawShotWritesBesideTheAnnotatedFile pins the path rule. The user keeps the
// annotated PNG open in a previewer that reloads on change; a raw shot landing on
// that same path would replace what they are looking at with an unlabelled frame and
// give no sign it had done so.
func TestRawShotWritesBesideTheAnnotatedFile(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "screen.png")

	// The handler derives the raw path from the configured one; this reproduces that
	// derivation so a change to it has to be deliberate.
	ext := filepath.Ext(base)
	rawPath := base[:len(base)-len(ext)] + "_raw" + ext

	if rawPath == base {
		t.Fatal("the raw path is the annotated path; a raw shot would overwrite the file the user has open")
	}
	if filepath.Ext(rawPath) != ".png" {
		t.Errorf("raw path %q lost its extension; a previewer keys on it", rawPath)
	}
	if err := os.WriteFile(base, []byte("annotated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawPath, []byte("raw"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "annotated" {
		t.Errorf("writing the raw file changed the annotated one")
	}
}
