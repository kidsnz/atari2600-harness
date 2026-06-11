// Package emu は Gopher2600 をライブラリとして自プロセスに埋め込み、headless で
// 駆動する薄いラッパ。MCP ツール群（cmd/harness）と配管検証 CLI（cmd/probe）の
// 共通土台。低レベルの terminal/PushedFunction は使わず hardware.VCS を直接叩く
// （より決定的・単純・高速）。
package emu

import (
	"fmt"
	"image"
	"image/color"

	"github.com/jetsetilly/gopher2600/cartridgeloader"
	"github.com/jetsetilly/gopher2600/digest"
	"github.com/jetsetilly/gopher2600/environment"
	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/riot/ports"
	"github.com/jetsetilly/gopher2600/hardware/riot/ports/plugging"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/coords"
	"github.com/jetsetilly/gopher2600/hardware/tia/video"
	"github.com/jetsetilly/gopher2600/setup"

	"github.com/kidsnz/atari2600-harness/internal/annotate"
)

// Emu は 1 台の VCS とその TV を保持する。
type Emu struct {
	TV  *television.Television
	VCS *hardware.VCS
	cap *capture // 最新フレームを image.RGBA に取り込む PixelRenderer

	cpuCycles int64 // ROM ロード以降に実行した CPU サイクルの累積（命令完了ごとに加算）
	cycleMark int64 // 区間計測の基準点（MarkCycles で現在の cpuCycles に揃える）

	vdigest *digest.Video // ゴールデンフレーム回帰用の連鎖ハッシュ（任意・EnableVideoDigest で有効化）
	adigest *digest.Audio // ゴールデン音声回帰用の連鎖ハッシュ（任意・EnableAudioDigest で有効化）
}

// EnableVideoDigest はフレームの連鎖ハッシュ（描画の指紋）を取り始める（D-3 ゴールデン回帰）。
// per-frame sha1 のコストがあるため任意。冪等。
func (e *Emu) EnableVideoDigest() error {
	if e.vdigest != nil {
		return nil
	}
	d, err := digest.NewVideo(e.TV) // TV に PixelRenderer として自己登録する
	if err != nil {
		return err
	}
	e.vdigest = d
	return nil
}

// ResetVideoDigest はハッシュ連鎖をゼロから取り直す（warmup を除外して決定的にするため）。
func (e *Emu) ResetVideoDigest() {
	if e.vdigest != nil {
		e.vdigest.ResetDigest()
	}
}

// VideoHash は現在までのフレーム連鎖ハッシュを返す（未有効なら ""）。
func (e *Emu) VideoHash() string {
	if e.vdigest == nil {
		return ""
	}
	return e.vdigest.Hash()
}

// EnableAudioDigest は音声の連鎖ハッシュ（音の指紋）を取り始める（A-2 ゴールデン音声回帰）。
// 映像 digest と同型・別チャンネル。冪等。
func (e *Emu) EnableAudioDigest() error {
	if e.adigest != nil {
		return nil
	}
	d, err := digest.NewAudio(e.TV) // TV に AudioMixer として自己登録する
	if err != nil {
		return err
	}
	e.adigest = d
	return nil
}

// ResetAudioDigest は音声ハッシュ連鎖をゼロから取り直す（warmup 除外で決定的化）。
func (e *Emu) ResetAudioDigest() {
	if e.adigest != nil {
		e.adigest.ResetDigest()
	}
}

// AudioHash は現在までの音声連鎖ハッシュを返す（未有効なら ""）。
func (e *Emu) AudioHash() string {
	if e.adigest == nil {
		return ""
	}
	return e.adigest.Hash()
}

// stepInstr は VCS を 1 ステップ進め、実際に 1 命令が実行された場合だけその実サイクル数を
// 累積へ加えて executed=true を返す。
//
// 肝: CPU は WSYNC stall 中（RdyFlg=false）だと ExecuteInstruction が命令を進めず
// cycleCallback を 1 回呼ぶだけで返り、LastResult は据え置かれる（Gopher2600 cpu.go:614）。
// よって「Step 直前に RdyFlg が true だった時だけ」が実命令の実行ステップ＝加算すべき点。
// stall ステップを数えると直前命令のサイクル数を多重加算してしまう（WSYNC を使う全 ROM で過大）。
// この規約で cpuCycles は「実行した命令サイクルの総和」になる（WSYNC の空転は含めない）。
func (e *Emu) stepInstr() (executed bool, err error) {
	ready := e.VCS.CPU.RdyFlg
	if err := e.VCS.Step(nil); err != nil {
		return false, err
	}
	if ready {
		e.cpuCycles += int64(e.VCS.CPU.LastResult.Cycles)
		return true, nil
	}
	return false, nil
}

// LastCycles は直近に完了した 1 命令のサイクル数を返す。
func (e *Emu) LastCycles() int { return e.VCS.CPU.LastResult.Cycles }

// TotalCycles は ROM ロード以降に実行した CPU サイクルの累積を返す。
func (e *Emu) TotalCycles() int64 { return e.cpuCycles }

// CyclesSinceMark は直近の MarkCycles 以降に実行した CPU サイクル数を返す（区間計測）。
func (e *Emu) CyclesSinceMark() int64 { return e.cpuCycles - e.cycleMark }

// MarkCycles は区間計測の基準点を現在に揃える（以後 CyclesSinceMark は 0 から数え直す）。
func (e *Emu) MarkCycles() { e.cycleMark = e.cpuCycles }

// New は指定 TV 仕様（"NTSC" / "PAL" / "AUTO" 等）で headless な VCS を作る。
func New(spec string) (*Emu, error) {
	tv, err := television.NewTelevision(spec)
	if err != nil {
		return nil, err
	}

	cap := newCapture()
	tv.AddPixelRenderer(cap) // NewVCS の前に接続（thumbnailer 同様）
	tv.SetFPSLimit(false)    // headless: スロットルしない

	vcs, err := hardware.NewVCS(environment.MainEmulation, tv, nil, nil)
	if err != nil {
		return nil, err
	}
	// 決定化（regression 用）: 既定では電源投入時状態が乱数化され（vcs.Env.Random、CPU.Reset で使用）、
	// 起動直後のサイクル/タイミングが run ごとに揺れて一部テストが CI で flaky になる。Normalise() は
	// Gopher2600 公式の「毎回同じ初期状態にする」メソッド（Random.ZeroSeed=true ＋ prefs デフォルト）。
	// AttachCartridge（LoadROM）でのリセット前に立てるので、以後の状態は決定的になる。
	vcs.Env.Normalise()
	return &Emu{TV: tv, VCS: vcs, cap: cap}, nil
}

// Snapshot は最新フレームの可視域（160×可視高さ）を独立コピーで返す。
// visibleTop はクロップ y=0 に対応する絶対 scanline（縦座標マッピング用）。
// 座標規約: 返り画像の x = 可視 clock 0..159、y = 絶対 scanline − visibleTop。
func (e *Emu) Snapshot() (img *image.RGBA, visibleTop int) {
	return e.cap.snapshot()
}

// Markers は 5 オブジェクトの横位置マーカーを Fixed Debug Colors で返す
// （P0=赤 / M0=橙 / P1=黄 / M1=緑 / BL=紫）。Clock は HmovedPixel（可視 0..159）。
func (e *Emu) Markers() []annotate.Marker {
	v := e.VCS.TIA.Video
	return []annotate.Marker{
		{Label: "P0", Clock: v.Player0.HmovedPixel, Col: color.RGBA{230, 60, 60, 255}},
		{Label: "M0", Clock: v.Missile0.HmovedPixel, Col: color.RGBA{235, 140, 40, 255}},
		{Label: "P1", Clock: v.Player1.HmovedPixel, Col: color.RGBA{230, 215, 50, 255}},
		{Label: "M1", Clock: v.Missile1.HmovedPixel, Col: color.RGBA{70, 200, 70, 255}},
		{Label: "BL", Clock: v.Ball.HmovedPixel, Col: color.RGBA{180, 90, 210, 255}},
	}
}

// Annotated は最新フレームに TIA 実座標のグリッド・軸ラベル・スプライトマーカーを重ねた
// 注釈画像を返す（ユーザー↔Claude の通信回線）。scale は整数倍率（×3〜4 推奨）。
func (e *Emu) Annotated(scale int) *image.RGBA {
	img, visTop := e.cap.snapshot()
	return annotate.Render(img, visTop, scale, e.Markers())
}

// LoadROM はファイルから ROM をロードして VCS にアタッチする。
func (e *Emu) LoadROM(path string) error {
	cartload, err := cartridgeloader.NewLoaderFromFilename(path, "AUTO", "AUTO", nil)
	if err != nil {
		return err
	}
	return setup.AttachCartridge(e.VCS, cartload, nil)
}

// Coords は現在のビーム位置（Frame/Scanline/Clock）を返す。横位置判定の出典。
func (e *Emu) Coords() coords.TelevisionCoords {
	return e.VCS.TV.GetCoords()
}

// RunFrames は n フレーム実行する（条件停止なし）。stepInstr 経由で CPU サイクルを正しく累積する
// （RunForFrameCount を使わないのは、stall ステップを除外した正確なサイクル計上を統一するため）。
func (e *Emu) RunFrames(n int) error {
	if n <= 0 {
		return nil
	}
	target := e.VCS.TV.GetCoords().Frame + n
	for {
		if e.VCS.CPU.Jammed {
			return nil // CPU jam 時は無限ループ防止（RunForFrameCount と同じガード）
		}
		if e.VCS.TV.IsFrameNum(target) {
			return nil
		}
		if _, err := e.stepInstr(); err != nil {
			return err
		}
	}
}

// StepFrame はちょうど 1 フレーム分ステップし、そのフレームに含まれた scanline 数を返す
// （タイミング検証点。NTSC なら 262 を期待）。
func (e *Emu) StepFrame() (int, error) {
	start := e.VCS.TV.GetCoords().Frame
	maxScanline := 0
	for {
		if _, err := e.stepInstr(); err != nil {
			return 0, err
		}
		c := e.VCS.TV.GetCoords()
		if c.Frame != start {
			break
		}
		if c.Scanline > maxScanline {
			maxScanline = c.Scanline
		}
	}
	return maxScanline + 1, nil
}

// StepInstruction はちょうど 1 つの CPU 命令を実行して進める。WSYNC stall 中なら stall を消化して
// 次の実命令まで進む（read_cycles と対で「1 命令ずつ覗く」フレーム内粒度。B-2）。
func (e *Emu) StepInstruction() error {
	for {
		executed, err := e.stepInstr()
		if err != nil {
			return err
		}
		if executed || e.VCS.CPU.Jammed {
			return nil
		}
	}
}

// StepScanline は TV の scanline がちょうど 1 つ進むまでステップする（フレーム境界では次フレームの
// scanline 0 で停止）。kernel の途中状態をライン単位で覗くための粒度（B-2）。
func (e *Emu) StepScanline() error {
	start := e.VCS.TV.GetCoords().Scanline
	for {
		if _, err := e.stepInstr(); err != nil {
			return err
		}
		if e.VCS.CPU.Jammed || e.VCS.TV.GetCoords().Scanline != start {
			return nil
		}
	}
}

// PeekRAM は副作用なしでメモリを読む（read_ram / peek の土台）。
func (e *Emu) PeekRAM(addr uint16) (uint8, error) {
	return e.VCS.Mem.Peek(addr)
}

// Poke はメモリへ 1 バイト書き込む（poke ツール）。
func (e *Emu) Poke(addr uint16, val uint8) error {
	return e.VCS.Mem.Poke(addr, val)
}

// SetInput はジョイスティック入力を注入する（headless ハーネスの入力経路。poke は入力に効かない）。
// player 0=PortLeft / 1=PortRight。action は left/right/up/down/fire/center。
// pressed=true で押下保持・false で解除（次に変えるまで状態は持続）。center は全方向解除。
func (e *Emu) SetInput(player int, action string, pressed bool) error {
	port := plugging.PortLeft
	if player == 1 {
		port = plugging.PortRight
	}
	var ev ports.Event
	var d ports.EventData
	switch action {
	case "center", "centre":
		ev, d = ports.Centre, nil
	case "fire":
		ev, d = ports.Fire, pressed
	case "left":
		ev = ports.Left
	case "right":
		ev = ports.Right
	case "up":
		ev = ports.Up
	case "down":
		ev = ports.Down
	default:
		return fmt.Errorf("unknown action %q (want left/right/up/down/fire/center)", action)
	}
	if ev != ports.Centre && ev != ports.Fire {
		if pressed {
			d = ports.DataStickTrue
		} else {
			d = ports.DataStickFalse
		}
	}
	_, err := e.VCS.RIOT.Ports.HandleInputEvent(ports.InputEvent{Port: port, Ev: ev, D: d})
	return err
}

// RowRun は ReadRow の連長エンコード 1 区間。可視 clock [Clock, Clock+Len) が同色 Hex。
type RowRun struct {
	Clock int    `json:"clock"` // 区間先頭の可視 clock（0..159）
	Len   int    `json:"len"`   // 連続ピクセル数
	Hex   string `json:"hex"`   // 表示 RGB（RRGGBB）。背景か前景かは色で判定
}

// ReadRow は指定した可視 scanline（注釈グリッドの y と同座標、0 起点）の 1 ライン分の
// ピクセル色を、横方向に連長エンコード(RLE)して返す。playfield の点灯列や per-scanline 色を
// 目視でなく数値で確かめるための土台。width は可視幅（通常 160）。
func (e *Emu) ReadRow(scanline int) (runs []RowRun, width int, err error) {
	img, _ := e.cap.snapshot()
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	if scanline < 0 || scanline >= h {
		return nil, w, fmt.Errorf("scanline %d out of visible range 0..%d", scanline, h-1)
	}
	hexAt := func(x int) string {
		c := img.RGBAAt(x, scanline)
		return fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B)
	}
	for x := 0; x < w; x++ {
		hx := hexAt(x)
		if len(runs) > 0 && runs[len(runs)-1].Hex == hx {
			runs[len(runs)-1].Len++
			continue
		}
		runs = append(runs, RowRun{Clock: x, Len: 1, Hex: hx})
	}
	return runs, w, nil
}

// --- P1: TIA 書込専用レジスタの現在値読み（色推論を実測へ）---

// PlayerRegs は 1 プレイヤーの書込専用レジスタ現在値（Gopher2600 内部保持の実測値）。
type PlayerRegs struct {
	Color         uint8 `json:"color"`           // COLUP0/1
	Nusiz         uint8 `json:"nusiz"`           // NUSIZ raw
	SizeAndCopies uint8 `json:"size_and_copies"` // NUSIZ 下位3bit
	GfxNew        uint8 `json:"gfx_new"`         // GRP（新）
	GfxOld        uint8 `json:"gfx_old"`         // GRP（VDEL 用 旧）
	Reflected     bool  `json:"reflected"`       // REFP
	VerticalDelay bool  `json:"vertical_delay"`  // VDELP
}

// MissileRegs は 1 ミサイルの書込専用レジスタ現在値。
type MissileRegs struct {
	Color         uint8 `json:"color"`
	Nusiz         uint8 `json:"nusiz"`
	Size          uint8 `json:"size"`
	Copies        uint8 `json:"copies"`
	Enabled       bool  `json:"enabled"`         // ENAM
	ResetToPlayer bool  `json:"reset_to_player"` // RESMP
}

// BallRegs はボールの書込専用レジスタ現在値。
type BallRegs struct {
	Color         uint8 `json:"color"`
	Size          uint8 `json:"size"`
	Enabled       bool  `json:"enabled"`        // ENABL
	VerticalDelay bool  `json:"vertical_delay"` // VDELBL
}

// PlayfieldRegs は playfield の書込専用レジスタ現在値。
type PlayfieldRegs struct {
	PF0             uint8 `json:"pf0"`
	PF1             uint8 `json:"pf1"`
	PF2             uint8 `json:"pf2"`
	ForegroundColor uint8 `json:"foreground_color"` // COLUPF
	BackgroundColor uint8 `json:"background_color"` // COLUBK
	Ctrlpf          uint8 `json:"ctrlpf"`
	Reflected       bool  `json:"reflected"` // CTRLPF D0
	Priority        bool  `json:"priority"`  // CTRLPF D2
	Scoremode       bool  `json:"scoremode"` // CTRLPF D1
}

// TIARegisters は書込専用 TIA レジスタの現在値一式（read_tia_registers の戻り）。
type TIARegisters struct {
	Player0   PlayerRegs    `json:"player0"`
	Player1   PlayerRegs    `json:"player1"`
	Missile0  MissileRegs   `json:"missile0"`
	Missile1  MissileRegs   `json:"missile1"`
	Ball      BallRegs      `json:"ball"`
	Playfield PlayfieldRegs `json:"playfield"`
}

// ReadTIARegisters は書込専用 TIA レジスタの現在値を Gopher2600 の内部保持から直接読む。
// 「sta COLUP0 は本当に効いたか」を色推論でなく実測で確かめるための窓（欠落A の残りを閉じる）。
func (e *Emu) ReadTIARegisters() TIARegisters {
	v := e.VCS.TIA.Video
	player := func(p *video.PlayerSprite) PlayerRegs {
		return PlayerRegs{
			Color: p.Color, Nusiz: p.Nusiz, SizeAndCopies: p.SizeAndCopies,
			GfxNew: p.GfxDataNew, GfxOld: p.GfxDataOld,
			Reflected: p.Reflected, VerticalDelay: p.VerticalDelay,
		}
	}
	missile := func(m *video.MissileSprite) MissileRegs {
		return MissileRegs{
			Color: m.Color, Nusiz: m.Nusiz, Size: m.Size, Copies: m.Copies,
			Enabled: m.Enabled, ResetToPlayer: m.ResetToPlayer,
		}
	}
	pf := v.Playfield
	return TIARegisters{
		Player0:  player(v.Player0),
		Player1:  player(v.Player1),
		Missile0: missile(v.Missile0),
		Missile1: missile(v.Missile1),
		Ball: BallRegs{
			Color: v.Ball.Color, Size: v.Ball.Size,
			Enabled: v.Ball.Enabled, VerticalDelay: v.Ball.VerticalDelay,
		},
		Playfield: PlayfieldRegs{
			PF0: pf.PF0, PF1: pf.PF1, PF2: pf.PF2,
			ForegroundColor: pf.ForegroundColor, BackgroundColor: pf.BackgroundColor,
			Ctrlpf: pf.Ctrlpf, Reflected: pf.Reflected, Priority: pf.Priority, Scoremode: pf.Scoremode,
		},
	}
}

// --- R-2: TIA 音声レジスタの現在値読み（音も数値で検証）---

// AudioChannel は 1 音声チャンネルの書込専用レジスタ現在値。
type AudioChannel struct {
	Control uint8 `json:"control"` // AUDC: 波形/音色（下位 4bit）
	Freq    uint8 `json:"freq"`    // AUDF: 分周（下位 5bit）
	Volume  uint8 `json:"volume"`  // AUDV: 音量（下位 4bit）
}

// AudioState は 2 チャンネルの音声レジスタ現在値（read_audio の戻り）。
type AudioState struct {
	Channel0 AudioChannel `json:"channel0"`
	Channel1 AudioChannel `json:"channel1"`
}

// ReadAudio は TIA 音声レジスタ（AUDC/AUDF/AUDV）の現在値を Gopher2600 の exported
// PeekChannels から読む。read_tia は映像のみで音声に検証経路が無かったため、鉄則1（判定は数値）を
// 音声領域へ拡張する。改変不要（Audio.PeekChannels は exported）。
func (e *Emu) ReadAudio() AudioState {
	r := e.VCS.TIA.Audio.PeekChannels()
	return AudioState{
		Channel0: AudioChannel{Control: r[0].Control, Freq: r[0].Freq, Volume: r[0].Volume},
		Channel1: AudioChannel{Control: r[1].Control, Freq: r[1].Freq, Volume: r[1].Volume},
	}
}

// --- P1: 衝突（CXxx）の構造化読み ---

// Collisions は 8 本の CXxx レジスタ（$30–$37, 各 D7/D6 ラッチ・sticky）を意味づけした真偽集合。
// CXCLR まで保持される。出典のビット割当は Gopher2600 collisions.go tick() で裏取り済み。
type Collisions struct {
	P0P1 bool `json:"p0_p1"` // CXPPMM D7
	M0M1 bool `json:"m0_m1"` // CXPPMM D6
	M0P0 bool `json:"m0_p0"` // CXM0P  D6
	M0P1 bool `json:"m0_p1"` // CXM0P  D7
	M1P0 bool `json:"m1_p0"` // CXM1P  D7
	M1P1 bool `json:"m1_p1"` // CXM1P  D6
	P0PF bool `json:"p0_pf"` // CXP0FB D7
	P0BL bool `json:"p0_bl"` // CXP0FB D6
	P1PF bool `json:"p1_pf"` // CXP1FB D7
	P1BL bool `json:"p1_bl"` // CXP1FB D6
	M0PF bool `json:"m0_pf"` // CXM0FB D7
	M0BL bool `json:"m0_bl"` // CXM0FB D6
	M1PF bool `json:"m1_pf"` // CXM1FB D7
	M1BL bool `json:"m1_bl"` // CXM1FB D6
	BLPF bool `json:"bl_pf"` // CXBLPF D7
}

// decodeCollisions は CXM0P,CXM1P,CXP0FB,CXP1FB,CXM0FB,CXM1FB,CXBLPF,CXPPMM（$30..$37 の順）の
// 生バイト 8 本から各衝突ペアを取り出す純関数（単体テスト対象）。D7=0x80 / D6=0x40。
func decodeCollisions(r [8]byte) Collisions {
	d7 := func(b byte) bool { return b&0x80 != 0 }
	d6 := func(b byte) bool { return b&0x40 != 0 }
	return Collisions{
		M0P1: d7(r[0]), M0P0: d6(r[0]), // CXM0P
		M1P0: d7(r[1]), M1P1: d6(r[1]), // CXM1P
		P0PF: d7(r[2]), P0BL: d6(r[2]), // CXP0FB
		P1PF: d7(r[3]), P1BL: d6(r[3]), // CXP1FB
		M0PF: d7(r[4]), M0BL: d6(r[4]), // CXM0FB
		M1PF: d7(r[5]), M1BL: d6(r[5]), // CXM1FB
		BLPF: d7(r[6]),                 // CXBLPF（D6 なし）
		P0P1: d7(r[7]), M0M1: d6(r[7]), // CXPPMM
	}
}

// ReadCollisions は衝突レジスタ $30–$37 を副作用なしで読み、構造化して返す。
func (e *Emu) ReadCollisions() (Collisions, error) {
	var r [8]byte
	for i := 0; i < 8; i++ {
		b, err := e.PeekRAM(uint16(0x30 + i))
		if err != nil {
			return Collisions{}, fmt.Errorf("peek CX %02X: %w", 0x30+i, err)
		}
		r[i] = b
	}
	return decodeCollisions(r), nil
}

// RunUntilBeam は最大 maxFrames フレーム実行し、ビームが (scanline, clock) に達したら
// 早期停止する。条件で止まったとき halted=true（breakif の土台）。
func (e *Emu) RunUntilBeam(maxFrames, scanline, clock int) (halted bool, err error) {
	target := e.VCS.TV.GetCoords().Frame + maxFrames
	for {
		if e.VCS.CPU.Jammed {
			return false, nil
		}
		if e.VCS.TV.IsFrameNum(target) {
			return false, nil // フレーム上限に到達（条件未成立）
		}
		if _, err := e.stepInstr(); err != nil {
			return false, err
		}
		c := e.VCS.TV.GetCoords()
		if c.Scanline == scanline && c.Clock == clock {
			return true, nil
		}
	}
}

// RunUntilBudget は最大 maxFrames フレーム走らせ、ある論理ライン（= WSYNC ストローブの間隔）が
// サイクル予算を超えて物理スキャンラインを食い込んだ瞬間に停止する。これは Pong v2 を黙って殺した
// 失敗モード（per-scanline サイクル超過 → 画面ロール、検知不能）を数値で捕まえる本丸ガード。
//
// 検出原理:
//   - WSYNC ストローブ = CPU RdyFlg の true→false 遷移（WSYNC だけが RDY を落とす, tia.go:195）。
//   - WSYNC は必ず次スキャンライン境界まで stall する。よって連続ストローブ間の「scanline 差」=
//     その論理ラインが実際に消費した物理ライン数（work が 1 ライン=76cy に収まれば 1、超えれば ≥2）。
//     scanline 差はプログラム局所で安定（machine cycle 差は隣接ライン依存で誤検知しやすく不採用）。
//
// budgetCycles は 1 WSYNC 区間あたりの CPU サイクル予算（既定 76 = 1 ライン）。多ライン・カーネル
// （例 2LK）では 152 等に上げる。over=true のとき atScanline=超過ラインの開始 scanline、
// lineCycles=そのラインが消費した概算 machine cycle（消費物理ライン数 × 76）。
func (e *Emu) RunUntilBudget(maxFrames, budgetCycles int) (over bool, atScanline, lineCycles int, err error) {
	if budgetCycles <= 0 {
		budgetCycles = 76
	}
	maxLines := budgetCycles / 76
	if maxLines < 1 {
		maxLines = 1
	}

	// 起動直後の数フレームはリセット／VSYNC 同期が安定せず WSYNC 間隔が乱れる（実測: frame 0 で
	// strobe が scanline 22→30 と飛ぶ）。誤検知を避けるため計測前に 2 フレーム空走して安定させる。
	if err := e.RunFrames(2); err != nil {
		return false, 0, 0, err
	}

	target := e.VCS.TV.GetCoords().Frame + maxFrames
	prevRdy := e.VCS.CPU.RdyFlg
	haveBaseline := false
	lastStrobeScanline := 0
	lastStrobeFrame := 0

	for {
		if e.VCS.CPU.Jammed {
			return false, 0, 0, nil
		}
		if e.VCS.TV.IsFrameNum(target) {
			return false, 0, 0, nil // フレーム上限に到達（超過なし）
		}
		if _, err := e.stepInstr(); err != nil {
			return false, 0, 0, err
		}

		rdy := e.VCS.CPU.RdyFlg
		if prevRdy && !rdy { // WSYNC ストローブ（STA WSYNC が今のステップで RDY を落とした）
			c := e.VCS.TV.GetCoords()
			if haveBaseline && c.Frame == lastStrobeFrame {
				lines := c.Scanline - lastStrobeScanline // この論理ラインが食った物理ライン数
				if lines > maxLines {
					return true, lastStrobeScanline + 1, lines * 76, nil
				}
			}
			lastStrobeScanline = c.Scanline
			lastStrobeFrame = c.Frame
			haveBaseline = true
		}
		prevRdy = rdy
	}
}
