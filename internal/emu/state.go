package emu

import (
	"fmt"
	"image"

	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/frameinfo"
	"github.com/jetsetilly/gopher2600/rewind"
)

// State は「この瞬間の機械まるごと」のスナップショット。SaveState で作り RestoreState で戻す。
// 用途は分岐探索 = 同一局面から N 通りの入力/RAM 値を試して結果を比べる（毎回 load_rom からの
// 再実行を不要にする）。同一 State は何度でも復元できる（Plumb 側が内部で再スナップショットする）。
//
// 中身は 3 層:
//   - vcs / tv : Gopher2600 の hardware.State / television.State（CPU・RAM・TIA・RIOT・カート・TV）
//   - pix      : capture の最新フレーム画像（下記の理由で必須）
//   - counters : 本ラッパが持つ CPU サイクル累積（read_cycles の出典）
//
// ★ pix を持つ理由（実測 2026-07-23, sandbox/experiments/monet-frogger/monet_anim.bin）:
// television.Plumb は tv.state を差し替えるだけで **PixelRenderer には一切触らない**
// （Gopher2600 hardware/television/television.go）。実測でも復元時に capture.Reset() は呼ばれず、
// 復元直後のフレームバッファには **分岐先で描かれた古い絵がそのまま残る**（ハッシュが保存時と
// 不一致・分岐時と一致）。つまり pix を戻さないと「RAM は戻ったのに get_screen だけ未来の絵」
// という再現しにくい嘘になる。ここで一緒に戻すことで復元は画面まで含めて一貫する。
//
// 覆わないもの（正直な限界。いずれも「加算されるだけの記録器」で、機械の状態ではない）:
//   - EnableVideoDigest / EnableAudioDigest の連鎖ハッシュ（巻き戻らない）
//   - EnableCoverage の PC/分岐カバレッジ（巻き戻らない = 累積カバレッジのまま）
//   - EnableAudioCapture の生サンプル列（巻き戻らない）
type State struct {
	vcs *hardware.State
	tv  *television.State

	pix       []uint8           // capture.img.Pix の独立コピー
	frameInfo frameinfo.Current // クロップ矩形の出典
	cropRect  image.Rectangle   // 復元時に cropImg を張り直すための矩形

	cpuCycles     int64
	cycleMark     int64
	paddlePlugged [2]bool
}

// Coords は State が指す TV 座標（frame/scanline/clock）。保存時点の識別に使う。
func (s *State) Coords() (frame, scanline, clock int) {
	c := s.tv.GetCoords()
	return c.Frame, c.Scanline, c.Clock
}

// SaveState は現在の機械状態を丸ごと保存する。emulator は進めない（副作用なし）。
func (e *Emu) SaveState() *State {
	pix := make([]uint8, len(e.cap.img.Pix))
	copy(pix, e.cap.img.Pix)

	cropRect := e.cap.frameInfo.Crop()
	if e.cap.cropImg != nil {
		cropRect = e.cap.cropImg.Bounds()
	}

	return &State{
		vcs:           e.VCS.Snapshot(),
		tv:            e.VCS.TV.Snapshot(),
		pix:           pix,
		frameInfo:     e.cap.frameInfo,
		cropRect:      cropRect,
		cpuCycles:     e.cpuCycles,
		cycleMark:     e.cycleMark,
		paddlePlugged: e.paddlePlugged,
	}
}

// RestoreState は SaveState で作った状態へ戻す。同一 State から何度でも戻せる。
func (e *Emu) RestoreState(s *State) error {
	if s == nil {
		return fmt.Errorf("restore state: nil state")
	}
	if len(s.pix) != len(e.cap.img.Pix) {
		return fmt.Errorf("restore state: framebuffer size mismatch (%d != %d)", len(s.pix), len(e.cap.img.Pix))
	}

	rewind.Plumb(e.VCS, &rewind.State{VCS: s.vcs, TV: s.tv}, false)

	// ★ フレームバッファを戻す（上のコメント参照）。cropImg は img.Pix を共有する SubImage
	// なので、img.Pix を書き戻せば同時に戻る。矩形だけは frameInfo ごと張り直す。
	copy(e.cap.img.Pix, s.pix)
	e.cap.frameInfo = s.frameInfo
	e.cap.cropImg = e.cap.img.SubImage(s.cropRect).(*image.RGBA)

	e.cpuCycles = s.cpuCycles
	e.cycleMark = s.cycleMark
	e.paddlePlugged = s.paddlePlugged
	return nil
}
