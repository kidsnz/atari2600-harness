package emu

import "testing"

// TestDecodeCollisions は CXxx 8 本の生バイト→衝突ペアの割当（D7/D6, Gopher2600 tick() 裏取り）を検証する純関数テスト。
func TestDecodeCollisions(t *testing.T) {
	// 各レジスタに D7 だけ立てる → D7 側のペアが全て true、D6 側は false。
	d7only := [8]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	c := decodeCollisions(d7only)
	d7 := map[string]bool{
		"M0P1": c.M0P1, "M1P0": c.M1P0, "P0PF": c.P0PF, "P1PF": c.P1PF,
		"M0PF": c.M0PF, "M1PF": c.M1PF, "BLPF": c.BLPF, "P0P1": c.P0P1,
	}
	for k, v := range d7 {
		if !v {
			t.Errorf("D7-only: %s should be true", k)
		}
	}
	if c.M0P0 || c.M1P1 || c.P0BL || c.P1BL || c.M0BL || c.M1BL || c.M0M1 {
		t.Errorf("D7-only: a D6 pair was unexpectedly set: %+v", c)
	}

	// 各レジスタに D6 だけ → D6 側が true、D7 側は false。CXBLPF は D6 を持たない。
	d6only := [8]byte{0x40, 0x40, 0x40, 0x40, 0x40, 0x40, 0x40, 0x40}
	c = decodeCollisions(d6only)
	d6 := map[string]bool{
		"M0P0": c.M0P0, "M1P1": c.M1P1, "P0BL": c.P0BL, "P1BL": c.P1BL,
		"M0BL": c.M0BL, "M1BL": c.M1BL, "M0M1": c.M0M1,
	}
	for k, v := range d6 {
		if !v {
			t.Errorf("D6-only: %s should be true", k)
		}
	}
	if c.M0P1 || c.M1P0 || c.P0PF || c.P1PF || c.M0PF || c.M1PF || c.BLPF || c.P0P1 {
		t.Errorf("D6-only: a D7 pair was unexpectedly set: %+v", c)
	}

	// 全 0 → 全 false。
	if (decodeCollisions([8]byte{}) != Collisions{}) {
		t.Errorf("all-zero should decode to all-false")
	}
}

// TestReadTIARegisters は書込専用レジスタの実測読みが ROM の既知書込みと一致することを裏取りする。
func TestReadTIARegisters(t *testing.T) {
	// smoke は可視領域で COLUBK=$1E を書く。1 フレーム走らせれば背景色レジスタに残る。
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/smoke.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	if got := e.ReadTIARegisters().Playfield.BackgroundColor; got != 0x1E {
		t.Fatalf("COLUBK = 0x%02X, want 0x1E (smoke sets it in the visible region)", got)
	}

	// litmus_pf は PF0/1/2 を点灯パターンに設定する。少なくとも 1 本は非ゼロのはず。
	e2, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e2.LoadROM("../../roms/litmus/litmus_pf.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e2.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	pf := e2.ReadTIARegisters().Playfield
	if pf.PF0 == 0 && pf.PF1 == 0 && pf.PF2 == 0 {
		t.Fatalf("litmus_pf: PF0/PF1/PF2 all zero, expected a lit pattern (%+v)", pf)
	}
}

// TestReadCollisionsNoSprites は描画オブジェクトの無い ROM で衝突が全 false（誤検知なし）であることを確認する。
func TestReadCollisionsNoSprites(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/smoke.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	cx, err := e.ReadCollisions()
	if err != nil {
		t.Fatal(err)
	}
	if (cx != Collisions{}) {
		t.Fatalf("smoke has no sprites/PF collisions yet got %+v", cx)
	}
}

// TestReadCollisionsBallPlayfield は、PF 全点灯＋ボール有効の ROM で BL-PF 衝突が実際にラッチされ
// ReadCollisions が陽性で拾うことを裏取りする（陰性 all-false だけでなく陽性経路も検証）。
func TestReadCollisionsBallPlayfield(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_collide.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	cx, err := e.ReadCollisions()
	if err != nil {
		t.Fatal(err)
	}
	if !cx.BLPF {
		t.Fatalf("expected BL-PF collision to latch (ball over fully-lit playfield), got %+v", cx)
	}
	// プレイヤーは未描画なので player 系衝突は立たないはず。
	if cx.P0PF || cx.P1PF || cx.P0P1 {
		t.Fatalf("unexpected player collision with no players drawn: %+v", cx)
	}
}
