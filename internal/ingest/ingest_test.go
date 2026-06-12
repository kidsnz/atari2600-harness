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

// --- M2: playfield 抽出のラウンドトリップ（正解既知） ---

// litmus_pf: PF0=$10 PF1=$80 PF2=$01・repeat・白 $0E が全可視行で出続ける。
func TestExtractLitmusPF(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/litmus/litmus_pf.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(10)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	bands, _ := ExtractPlayfield(n)
	if len(bands) == 0 {
		t.Fatal("no PF bands found")
	}
	total := 0
	for _, b := range bands {
		if b.Mode != "repeat" || b.PF0 != 0x10 || b.PF1 != 0x80 || b.PF2 != 0x01 {
			t.Fatalf("band %+v, want repeat $10/$80/$01", b)
		}
		want := q.Canonical(0x0E) // パレット同色衝突（$0C≡$0E）は正準値で比較
		if b.ColorLeft != want || b.ColorRight != want {
			t.Fatalf("band colors $%02X/$%02X, want $%02X", b.ColorLeft, b.ColorRight, want)
		}
		total += b.Height
	}
	if total < 180 {
		t.Fatalf("PF rows total %d, want >=180", total)
	}
}

// pf_modes: score-mode（PF1=$66 左右同パターン・別色）と壁（PF2=$10）が抽出できる。
func TestExtractPFModes(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/techniques/pf_modes.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(10)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	bands, _ := ExtractPlayfield(n)
	score, wall := false, false
	for _, b := range bands {
		if b.PF1 == 0x66 && b.ScoreMode && b.ColorLeft == 0x44 && b.ColorRight == 0x86 {
			score = true
		}
		if b.PF2 == 0x10 && b.PF0 == 0 && b.PF1 == 0 && b.Mode == "repeat" {
			wall = true
		}
	}
	if !score {
		t.Fatalf("score-mode band not found in %d bands", len(bands))
	}
	if !wall {
		t.Fatalf("wall band not found in %d bands", len(bands))
	}
}

// Exerciser 山脈: reflect 判定＋抽出バイトが RAM の band データ（実行時の正解）と一致。
func TestExtractReflectMountains(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/exerciser/exerciser.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(5)
	e.Poke(0x80, 4) // scene=proc
	e.Poke(0x83, 0) // lastScene≠4 → 進入初期化（山生成）
	e.RunFrames(20)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	bands, _ := ExtractPlayfield(n)
	// RAM の band 三つ組（mPF0/mPF1/mPF2 = $C0/$CA/$D4 起点 ×10）
	type triple struct{ p0, p1, p2 uint8 }
	ram := map[triple]bool{}
	for i := 0; i < 10; i++ {
		a, _ := e.PeekRAM(uint16(0xC0 + i))
		b, _ := e.PeekRAM(uint16(0xCA + i))
		c, _ := e.PeekRAM(uint16(0xD4 + i))
		ram[triple{a & 0xF0, b, c}] = true // PF0 は上位ニブルのみ表示＝表示真実でマスク
	}
	found := 0
	for _, b := range bands {
		if b.Mode != "reflect" || b.Height < 6 {
			continue
		}
		if !ram[triple{b.PF0, b.PF1, b.PF2}] {
			t.Fatalf("reflect band $%02X/$%02X/$%02X not in RAM ground truth", b.PF0, b.PF1, b.PF2)
		}
		found++
	}
	if found < 3 {
		t.Fatalf("only %d reflect mountain bands matched, want >=3", found)
	}
}

// --- M3: スプライト抽出のラウンドトリップ（正解既知） ---

// vertical_pos のボール: GRP 行が Art 定数とビット単位で一致、X=80、色は青 $86（正準）。
func TestExtractSpriteBall(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/techniques/vertical_pos.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(30)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	_, residual := ExtractPlayfield(n)
	sprites := ExtractSprites(n, residual)
	if len(sprites) != 1 {
		t.Fatalf("found %d sprites, want 1", len(sprites))
	}
	s := sprites[0]
	want := []uint8{0x3C, 0x7E, 0xFF, 0xDB, 0xFF, 0xE7, 0x7E, 0x3C}
	if s.Kind != "player" || s.X != 80 || s.H != 8 {
		t.Fatalf("sprite %+v, want player at x=80 h=8", s)
	}
	for i, b := range want {
		if s.GRP[i] != b {
			t.Fatalf("GRP[%d] = %%%08b, want %%%08b", i, s.GRP[i], b)
		}
		if s.Colors[i] != q.Canonical(0x86) {
			t.Fatalf("row color $%02X, want canonical $86", s.Colors[i])
		}
	}
}

// sprite_anim の歩行者: 行4倍化（h=32）込みで GRP がフェーズの絵と一致。
func TestExtractSpriteWalker(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/techniques/sprite_anim.bin"); err != nil {
		t.Fatal(err)
	}
	// phase==0（$80）かつ右向き（$83==0）のフレームまで進める
	for i := 0; i < 200; i++ {
		e.RunFrames(1)
		ph, _ := e.PeekRAM(0x80)
		dir, _ := e.PeekRAM(0x83)
		tmr, _ := e.PeekRAM(0x81)
		if ph == 0 && dir == 0 && tmr == 4 {
			break
		}
	}
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	_, residual := ExtractPlayfield(n)
	sprites := ExtractSprites(n, residual)
	if len(sprites) != 1 {
		t.Fatalf("found %d sprites, want 1", len(sprites))
	}
	s := sprites[0]
	phase0 := []uint8{0x18, 0x18, 0x3C, 0x78, 0x3C, 0x24, 0x42, 0x81}
	if s.Kind != "player" || s.H != 32 {
		t.Fatalf("sprite %+v, want player h=32 (row-quadrupled)", s)
	}
	for i := 0; i < 32; i++ {
		if s.GRP[i] != phase0[i/4] {
			t.Fatalf("GRP[%d] = %%%08b, want %%%08b (art row %d)", i, s.GRP[i], phase0[i/4], i/4)
		}
	}
}

// litmus_nusiz_copies: 3 コピー近接（間隔16）が 1 件 copies=3 に畳まれる。
func TestExtractNusizCopies(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/litmus/litmus_nusiz_copies.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(10)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	_, residual := ExtractPlayfield(n)
	sprites := ExtractSprites(n, residual)
	for _, s := range sprites {
		if s.Copies == 3 && s.Spacing == 16 {
			return // 期待どおり
		}
	}
	t.Fatalf("no 3-copy/spacing-16 group found in %+v", sprites)
}
