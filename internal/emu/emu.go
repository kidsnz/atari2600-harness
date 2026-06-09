// Package emu は Gopher2600 をライブラリとして自プロセスに埋め込み、headless で
// 駆動する薄いラッパ。MCP ツール群（cmd/harness）と配管検証 CLI（cmd/probe）の
// 共通土台。低レベルの terminal/PushedFunction は使わず hardware.VCS を直接叩く
// （より決定的・単純・高速）。
package emu

import (
	"github.com/jetsetilly/gopher2600/cartridgeloader"
	"github.com/jetsetilly/gopher2600/environment"
	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/coords"
	"github.com/jetsetilly/gopher2600/setup"
)

// Emu は 1 台の VCS とその TV を保持する。
type Emu struct {
	TV  *television.Television
	VCS *hardware.VCS
}

// New は指定 TV 仕様（"NTSC" / "PAL" / "AUTO" 等）で headless な VCS を作る。
func New(spec string) (*Emu, error) {
	tv, err := television.NewTelevision(spec)
	if err != nil {
		return nil, err
	}
	tv.SetFPSLimit(false) // headless: スロットルしない

	vcs, err := hardware.NewVCS(environment.MainEmulation, tv, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Emu{TV: tv, VCS: vcs}, nil
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
