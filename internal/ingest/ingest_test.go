package ingest

import (
	"image"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/pkg/playfield"
)

func playfieldEncode(cells []bool) (uint8, uint8, uint8) {
	return playfield.EncodeSymmetric(cells)
}

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
	bands, _, _ := ExtractPlayfield(n)
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
	bands, _, _ := ExtractPlayfield(n)
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
	bands, _, _ := ExtractPlayfield(n)
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
	_, residual, _ := ExtractPlayfield(n)
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
		if s.GRP[i] != int(b) {
			t.Fatalf("GRP[%d] = %%%08b, want %%%08b", i, s.GRP[i], b)
		}
		if s.Colors[i] != int(q.Canonical(0x86)) {
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
	_, residual, _ := ExtractPlayfield(n)
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
		if s.GRP[i] != int(phase0[i/4]) {
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
	_, residual, _ := ExtractPlayfield(n)
	sprites := ExtractSprites(n, residual)
	for _, s := range sprites {
		if s.Copies == 3 && s.Spacing == 16 {
			return // 期待どおり
		}
	}
	t.Fatalf("no 3-copy/spacing-16 group found in %+v", sprites)
}

// --- M5: 忠実度（自前 ROM は完全再構成できなければバグ） ---

func fidelityOf(t *testing.T, romPath string, frames int) float64 {
	t.Helper()
	e, _ := emu.New("NTSC")
	if err := e.LoadROM(romPath); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(frames)
	truth, _ := e.Snapshot()
	q := NewNTSCQuantizer()
	n, err := Normalize(upscale(truth, 2, 1), q)
	if err != nil {
		t.Fatal(err)
	}
	rep := Analyze(n, q)
	return rep.Fidelity
}

func TestFidelityOwnROMs(t *testing.T) {
	cases := []struct {
		rom    string
		frames int
		min    float64
	}{
		{"../../roms/litmus/litmus_pf.bin", 10, 1.0},
		{"../../roms/techniques/pf_modes.bin", 10, 0.999}, // 優先度領域は再構成が sprite-over-PF 仮定（誤差数px）
		{"../../roms/techniques/vertical_pos.bin", 30, 1.0},
		{"../../roms/techniques/sprite_anim.bin", 30, 1.0},
		{"../../roms/litmus/litmus_nusiz_copies.bin", 10, 1.0},
	}
	for _, c := range cases {
		f := fidelityOf(t, c.rom, c.frames)
		if f < c.min {
			t.Errorf("%s: fidelity %.4f < %.4f", c.rom, f, c.min)
		}
	}
}

// --- M6: 文脈つき降格（スプライトの水平ストロークを PF に取られない） ---
// 4clk 整列の上下棒を持つリング3つ（"000" 型）が、棒も含めて 3 つの完全な成分になること。
func TestContextDemotionRings(t *testing.T) {
	q := NewNTSCQuantizer()
	orange := q.RGB(q.Canonical(0x3C))
	img := image.NewRGBA(image.Rect(0, 0, 160, 24))
	draw := func(x, y int) { img.SetRGBA(x, y, orange) }
	for d := 0; d < 3; d++ {
		x0 := 72 + d*8 // 上下棒は col18/20/22 に 4clk 整列（列数3 > 2 ＝旧規則をすり抜ける形）
		for dx := 0; dx < 4; dx++ {
			draw(x0+dx, 6)  // 上棒
			draw(x0+dx, 13) // 下棒
		}
		for y := 7; y <= 12; y++ { // 両壁（1px ＝ 4clk 列として不均一 → residual に落ちる側）
			draw(x0, y)
			draw(x0+3, y)
		}
	}
	n, err := Normalize(img, q)
	if err != nil {
		t.Fatal(err)
	}
	bands, residual, _ := ExtractPlayfield(n)
	if len(bands) != 0 {
		t.Fatalf("rings leaked into %d PF bands: %+v", len(bands), bands)
	}
	sprites := ExtractSprites(n, residual)
	if len(sprites) != 3 {
		t.Fatalf("found %d sprites, want 3 rings", len(sprites))
	}
	for _, s := range sprites {
		if s.H != 8 || s.W != 4 {
			t.Fatalf("ring %+v, want 4x8 complete (top/bottom bars included)", s)
		}
	}
}

// --- M7: 重なり修復（正解既知の合成画像） ---
// 繰り返すビル風 PF（3周期）の中央周期にスプライトを重ね、
// ①スプライト GRP が重なり前の定義とビット単位一致 ②PF が参照周期どおりに修復
// ③fidelity 100% を assert する。
func TestRepairOverlap(t *testing.T) {
	q := NewNTSCQuantizer()
	cyan := q.Canonical(0x9E)
	green := q.Canonical(0xCE)
	img := image.NewRGBA(image.Rect(0, 0, 160, 40))
	// PF: 屋根行(cols2-8)と窓行(cols2,4,6,8)を4行ずつ、8行周期×3（y=4..27）、repeat
	pfRow := func(y int, cols []int) {
		for _, c := range cols {
			for _, base := range []int{0, 80} {
				for dx := 0; dx < 4; dx++ {
					img.SetRGBA(base+c*4+dx, y, q.RGB(cyan))
				}
			}
		}
	}
	roof := []int{2, 3, 4, 5, 6, 7, 8}
	win := []int{2, 4, 6, 8}
	for cyc := 0; cyc < 3; cyc++ {
		for r := 0; r < 4; r++ {
			pfRow(4+cyc*8+r, roof)
			pfRow(4+cyc*8+4+r, win)
		}
	}
	// スプライト: 8px 枠 ($FF,$81,$BD,$A5,$A5,$BD,$81,$FF) を x=16, y=12（中央周期に重なる）
	art := []uint8{0xFF, 0x81, 0xBD, 0xA5, 0xA5, 0xBD, 0x81, 0xFF}
	for r, g := range art {
		for bit := 0; bit < 8; bit++ {
			if g&(1<<(7-uint(bit))) != 0 {
				img.SetRGBA(16+bit, 12+r, q.RGB(green)) // PF を上書き＝sprite over PF
			}
		}
	}
	n, err := Normalize(img, q)
	if err != nil {
		t.Fatal(err)
	}
	rep := Analyze(n, q)
	// ① スプライト完全復元
	if len(rep.Sprites) != 1 {
		t.Fatalf("found %d sprites, want 1: %+v", len(rep.Sprites), rep.Sprites)
	}
	s := rep.Sprites[0]
	if s.X != 16 || s.Y != 12 || s.W != 8 || s.H != 8 {
		t.Fatalf("sprite bbox %+v, want (16,12) 8x8", s)
	}
	for i, want := range art {
		if s.GRP[i] != int(want) {
			t.Fatalf("GRP[%d] = %%%08b, want %%%08b", i, s.GRP[i], want)
		}
	}
	// ② 修復: 全バンドが屋根 or 窓のどちらかのバイト列（汚染パターンが残っていない）
	wantRoof := bandBytes(roof)
	wantWin := bandBytes(win)
	for _, b := range rep.Playfield {
		got := [3]uint8{b.PF0, b.PF1, b.PF2}
		if got != wantRoof && got != wantWin {
			t.Fatalf("contaminated band survived: %+v", b)
		}
	}
	// ③ 完全再構成
	if rep.Fidelity != 1.0 {
		t.Fatalf("fidelity %.4f, want 1.0", rep.Fidelity)
	}
}

func bandBytes(cols []int) [3]uint8 {
	cells := make([]bool, 20)
	for _, c := range cols {
		cells[c] = true
	}
	pf0, pf1, pf2 := playfieldEncode(cells)
	return [3]uint8{pf0, pf1, pf2}
}

// --- M8: マルチフレーム分離（正解既知・自前 ROM で多フレーム生成） ---

func captureFrames(t *testing.T, romPath string, warmup, count int) []*Normalized {
	t.Helper()
	e, _ := emu.New("NTSC")
	if err := e.LoadROM(romPath); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(warmup)
	q := NewNTSCQuantizer()
	var out []*Normalized
	for i := 0; i < count; i++ {
		img, _ := e.Snapshot()
		n, err := Normalize(upscale(img, 2, 1), q)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
		e.RunFrames(1)
	}
	return out
}

// flicker_multiplex: 連続2フレームで 4 オブジェクト全部が union に揃い、flicker 判定が立つ。
func TestMultiFrameFlicker(t *testing.T) {
	frames := captureFrames(t, "../../roms/techniques/flicker_multiplex.bin", 40, 2)
	q := NewNTSCQuantizer()
	mr, err := AnalyzeFrames(frames, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(mr.Union) != 4 {
		t.Fatalf("union has %d objects, want 4 (got %+v)", len(mr.Union), mr.Union)
	}
	colors := map[int]bool{}
	for _, u := range mr.Union {
		if !u.Flicker {
			t.Fatalf("object %+v should be flagged flicker (appears in 1 of 2 frames)", u.Sprite)
		}
		if u.Kind != "player" || u.H != 8 {
			t.Fatalf("object %+v, want 8-row player", u.Sprite)
		}
		for _, c := range u.Colors {
			colors[c] = true
		}
	}
	if len(colors) != 4 {
		t.Fatalf("union colors %v, want 4 distinct", colors)
	}
	// 静的層は空（PF なし・駐機物なし）、各フレーム fidelity 100%
	if len(mr.Static.Playfield) != 0 || len(mr.Static.Sprites) != 0 {
		t.Fatalf("static layer should be empty: pf=%d static=%d", len(mr.Static.Playfield), len(mr.Static.Sprites))
	}
	for i, fr := range mr.Frames {
		if fr.Fidelity != 1.0 {
			t.Fatalf("frame %d fidelity %.4f, want 1.0", i, fr.Fidelity)
		}
	}
}

// sprite_anim: 歩行者が両フレームに（移動して）出る＝flicker ではない。静的層クリーン。
func TestMultiFrameWalker(t *testing.T) {
	frames := captureFrames(t, "../../roms/techniques/sprite_anim.bin", 30, 2)
	q := NewNTSCQuantizer()
	mr, err := AnalyzeFrames(frames, q)
	if err != nil {
		t.Fatal(err)
	}
	for i, fr := range mr.Frames {
		if len(fr.Sprites) != 1 {
			t.Fatalf("frame %d: %d sprites, want 1", i, len(fr.Sprites))
		}
		if fr.Fidelity != 1.0 {
			t.Fatalf("frame %d fidelity %.4f", i, fr.Fidelity)
		}
	}
	// 位置が 1px 進んでいる（同一オブジェクトの移動）
	if mr.Frames[1].Sprites[0].X != mr.Frames[0].Sprites[0].X+1 {
		t.Fatalf("walker did not advance: %d -> %d", mr.Frames[0].Sprites[0].X, mr.Frames[1].Sprites[0].X)
	}
	for _, u := range mr.Union {
		if u.Flicker && len(mr.Union) == 1 {
			t.Fatalf("moving walker misflagged as flicker")
		}
	}
}

// pf_modes（静止シーン）×2: 静的層の PF が単一フレーム解析と一致＝退行なし。
func TestMultiFrameStaticScene(t *testing.T) {
	frames := captureFrames(t, "../../roms/techniques/pf_modes.bin", 10, 2)
	q := NewNTSCQuantizer()
	mr, err := AnalyzeFrames(frames, q)
	if err != nil {
		t.Fatal(err)
	}
	single := Analyze(frames[0], q)
	if len(mr.Static.Playfield) != len(single.Playfield) {
		t.Fatalf("PF bands %d != single-frame %d", len(mr.Static.Playfield), len(single.Playfield))
	}
	for i := range single.Playfield {
		a, b := mr.Static.Playfield[i], single.Playfield[i]
		if a.Top != b.Top || a.Height != b.Height || a.Mode != b.Mode ||
			a.PF0 != b.PF0 || a.PF1 != b.PF1 || a.PF2 != b.PF2 ||
			a.ColorLeft != b.ColorLeft || a.ColorRight != b.ColorRight {
			t.Fatalf("band %d differs: %+v vs %+v", i, a, b)
		}
	}
	// P0 柱（静止物）は static_* 側に出る（動的層には何も出ない）
	for _, s := range mr.Static.Sprites {
		if len(s.Kind) < 7 || s.Kind[:7] != "static_" {
			t.Fatalf("static-layer sprite without static_ prefix: %+v", s)
		}
	}
	if len(mr.Static.Sprites) == 0 {
		t.Fatalf("static P0 column not found in static layer")
	}
	for i, fr := range mr.Frames {
		if len(fr.Sprites) != 0 {
			t.Fatalf("frame %d has %d dynamic sprites, want 0 in a static scene", i, len(fr.Sprites))
		}
	}
	if mr.UnresolvedShare != 0 {
		t.Fatalf("unresolved %.4f, want 0 for a static scene", mr.UnresolvedShare)
	}
}

// --- M-G: 動的マルチスプライト kernel（technique #10 完全形）の機能証明 ---
// 5 オブジェクト全色が描画され（=ソート＋動的割当＋画面中再配置が機能）、262 を維持する。
func TestDynMultisprite(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/techniques/dyn_multisprite.bin"); err != nil {
		t.Fatal(err)
	}
	for f := 0; f < 30; f++ {
		lines, _ := e.StepFrame()
		if f >= 2 && lines != 262 {
			t.Fatalf("frame %d: %d lines", f, lines)
		}
	}
	q := NewNTSCQuantizer()
	colors := map[int]bool{}
	for i := 0; i < 6; i++ {
		img, _ := e.Snapshot()
		n, _ := Normalize(upscale(img, 2, 1), q)
		rep := Analyze(n, q)
		for _, s := range rep.Sprites {
			for _, c := range s.Colors {
				colors[c] = true
			}
		}
		e.RunFrames(3)
	}
	for _, want := range []int{0x1E, 0x56, 0x9A, 0xCA, 0x44} {
		if !colors[int(q.Canonical(uint8(want)))] {
			t.Fatalf("object color $%02X never seen (colors=%v)", want, colors)
		}
	}
}

// アニメ PF（星空スクロール）は動的層で animated_pf? ヒント付きになり、山は静的 PF に残る。
func TestMultiFrameAnimatedPF(t *testing.T) {
	e, _ := emu.New("NTSC")
	if err := e.LoadROM("../../roms/exerciser/exerciser.bin"); err != nil {
		t.Fatal(err)
	}
	e.RunFrames(5)
	e.Poke(0x80, 4)
	e.Poke(0x83, 0)
	e.RunFrames(20)
	q := NewNTSCQuantizer()
	var frames []*Normalized
	for i := 0; i < 3; i++ {
		img, _ := e.Snapshot()
		n, _ := Normalize(upscale(img, 2, 1), q)
		frames = append(frames, n)
		e.RunFrames(1)
	}
	mr, err := AnalyzeFrames(frames, q)
	if err != nil {
		t.Fatal(err)
	}
	// 山（reflect バンド）が静的層に残る
	reflect := 0
	for _, b := range mr.Static.Playfield {
		if b.Mode == "reflect" && b.Height >= 6 {
			reflect++
		}
	}
	if reflect < 3 {
		t.Fatalf("mountains not in static layer: %d reflect bands", reflect)
	}
	// 動的層に animated_pf? ヒントの幅広成分が出る
	hinted := 0
	for _, fr := range mr.Frames {
		for _, s := range fr.Sprites {
			if s.Hint == "animated_pf?" {
				hinted++
			}
		}
	}
	if hinted == 0 {
		t.Fatalf("no animated_pf? hints in dynamic layers")
	}
}

// --- R3: 行中 COLUPF（color_writes）---
// 左半に2色の PF が並ぶ行 → ColorWrites として表現され fidelity 100%。
func TestColorWrites(t *testing.T) {
	q := NewNTSCQuantizer()
	a := q.Canonical(0xD6) // 葉
	b := q.Canonical(0x42) // 赤
	img := image.NewRGBA(image.Rect(0, 0, 160, 12))
	for y := 4; y < 8; y++ {
		for c := 1; c <= 8; c++ { // colsA
			for dx := 0; dx < 4; dx++ {
				img.SetRGBA(c*4+dx, y, q.RGB(a))
			}
		}
		for c := 12; c <= 18; c++ { // colsB（同じ左半・別色）
			for dx := 0; dx < 4; dx++ {
				img.SetRGBA(c*4+dx, y, q.RGB(b))
			}
		}
	}
	n, err := Normalize(img, q)
	if err != nil {
		t.Fatal(err)
	}
	rep := Analyze(n, q)
	if len(rep.Playfield) != 1 {
		t.Fatalf("bands=%d want 1: %+v", len(rep.Playfield), rep.Playfield)
	}
	band := rep.Playfield[0]
	if len(band.ColorWrites) < 2 {
		t.Fatalf("no color_writes: %+v", band)
	}
	if band.ColorWrites[1].Clock != 48 || band.ColorWrites[1].Color != int(b) {
		t.Fatalf("write[1]=%+v want clock48 color $%02X", band.ColorWrites[1], b)
	}
	if rep.Fidelity != 1.0 {
		t.Fatalf("fidelity %.4f", rep.Fidelity)
	}
}
