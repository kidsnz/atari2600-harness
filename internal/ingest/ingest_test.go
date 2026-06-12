package ingest

import (
	"image"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// 疑似 Stella 化: TIA 画像を sx×sy の整数スケールで拡大する。
func upscale(src *image.RGBA, sx, sy int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*sx, b.Dy()*sy))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := src.RGBAAt(b.Min.X+x, b.Min.Y+y)
			for dy := 0; dy < sy; dy++ {
				for dx := 0; dx < sx; dx++ {
					dst.SetRGBA(x*sx+dx, y*sy+dy, c)
				}
			}
		}
	}
	return dst
}

// ラウンドトリップ（正解既知）: litmus_pf_async を Gopher2600 で描画 → 2×1 拡大で
// Stella 形状を模す → Normalize がスケールを正しく当て、ピクセルが完全往復すること。
func TestRoundTripNormalize(t *testing.T) {
	e, err := emu.New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_pf_async.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	truth, _ := e.Snapshot()
	fake := upscale(truth, 2, 1) // Stella F12 と同じ 320 幅

	q := NewNTSCQuantizer()
	n, err := Normalize(fake, q)
	if err != nil {
		t.Fatal(err)
	}
	if n.ScaleX != 2 || n.ScaleY != 1 {
		t.Fatalf("scale = %dx%d, want 2x1", n.ScaleX, n.ScaleY)
	}
	if n.Height != truth.Bounds().Dy() {
		t.Fatalf("height = %d, want %d", n.Height, truth.Bounds().Dy())
	}
	if n.AvgDist != 0 {
		t.Fatalf("avg palette dist = %f, want 0 (same palette must round-trip exactly)", n.AvgDist)
	}
	diff := 0
	for y := 0; y < n.Height; y++ {
		for x := 0; x < tiaWidth; x++ {
			if n.TIA.RGBAAt(x, y) != truth.RGBAAt(truth.Bounds().Min.X+x, truth.Bounds().Min.Y+y) {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Fatalf("%d pixels differ after round-trip", diff)
	}
}

// 2×2 など他の整数スケールも当てられること。
func TestScaleDetection2x2(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/litmus/litmus_pf_async.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(10); err != nil {
		t.Fatal(err)
	}
	truth, _ := e.Snapshot()
	fake := upscale(truth, 2, 2)
	q := NewNTSCQuantizer()
	n, err := Normalize(fake, q)
	if err != nil {
		t.Fatal(err)
	}
	if n.ScaleX != 2 || n.ScaleY != 2 {
		t.Fatalf("scale = %dx%d, want 2x2", n.ScaleX, n.ScaleY)
	}
	if n.AvgDist != 0 {
		t.Fatalf("avg dist %f", n.AvgDist)
	}
}

func TestQuantizerExactness(t *testing.T) {
	q := NewNTSCQuantizer()
	// パレット内の全色は距離 0 で自分自身に戻る
	for _, code := range q.codes {
		got, d := q.Nearest(q.RGB(code))
		if d != 0 {
			t.Fatalf("code $%02X: dist %d, want 0", code, d)
		}
		if q.RGB(got) != q.RGB(code) {
			t.Fatalf("code $%02X mapped to different RGB", code)
		}
	}
}
