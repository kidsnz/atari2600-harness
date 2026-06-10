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
	"github.com/jetsetilly/gopher2600/debugger/govern"
	"github.com/jetsetilly/gopher2600/environment"
	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/coords"
	"github.com/jetsetilly/gopher2600/setup"

	"github.com/kidsnz/atari2600-dev/internal/annotate"
)

// Emu は 1 台の VCS とその TV を保持する。
type Emu struct {
	TV  *television.Television
	VCS *hardware.VCS
	cap *capture // 最新フレームを image.RGBA に取り込む PixelRenderer
}

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

// RunFrames は n フレーム実行する（条件停止なし）。
func (e *Emu) RunFrames(n int) error {
	return e.VCS.RunForFrameCount(n, nil)
}

// StepFrame はちょうど 1 フレーム分カラークロック単位でステップし、そのフレームに
// 含まれた scanline 数を返す（タイミング検証点。NTSC なら 262 を期待）。
func (e *Emu) StepFrame() (int, error) {
	start := e.VCS.TV.GetCoords().Frame
	maxScanline := 0
	for {
		if err := e.VCS.Step(nil); err != nil {
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

// PeekRAM は副作用なしでメモリを読む（read_ram / peek の土台）。
func (e *Emu) PeekRAM(addr uint16) (uint8, error) {
	return e.VCS.Mem.Peek(addr)
}

// Poke はメモリへ 1 バイト書き込む（poke ツール）。
func (e *Emu) Poke(addr uint16, val uint8) error {
	return e.VCS.Mem.Poke(addr, val)
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

// RunUntilBeam は最大 maxFrames フレーム実行し、ビームが (scanline, clock) に達したら
// 早期停止する。条件で止まったとき halted=true（breakif の土台）。
func (e *Emu) RunUntilBeam(maxFrames, scanline, clock int) (halted bool, err error) {
	check := func() (govern.State, error) {
		c := e.VCS.TV.GetCoords()
		if c.Scanline == scanline && c.Clock == clock {
			halted = true
			return govern.Ending, nil
		}
		return govern.Running, nil
	}
	err = e.VCS.RunForFrameCount(maxFrames, check)
	return halted, err
}
