// Package emu は Gopher2600 をライブラリとして自プロセスに埋め込み、headless で
// 駆動する薄いラッパ。MCP ツール群（cmd/harness）と配管検証 CLI（cmd/probe）の
// 共通土台。低レベルの terminal/PushedFunction は使わず hardware.VCS を直接叩く
// （より決定的・単純・高速）。
package emu

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"sort"

	"github.com/jetsetilly/gopher2600/cartridgeloader"
	"github.com/jetsetilly/gopher2600/digest"
	"github.com/jetsetilly/gopher2600/environment"
	"github.com/jetsetilly/gopher2600/hardware"
	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/cartridge/mapper"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
	"github.com/jetsetilly/gopher2600/hardware/peripherals/controllers"
	"github.com/jetsetilly/gopher2600/hardware/riot/ports"
	"github.com/jetsetilly/gopher2600/hardware/riot/ports/plugging"
	"github.com/jetsetilly/gopher2600/hardware/riot/timer"
	"github.com/jetsetilly/gopher2600/hardware/television"
	"github.com/jetsetilly/gopher2600/hardware/television/coords"
	"github.com/jetsetilly/gopher2600/hardware/television/signal"
	"github.com/jetsetilly/gopher2600/hardware/television/specification"
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

	vdigest       *digest.Video // ゴールデンフレーム回帰用の連鎖ハッシュ（任意・EnableVideoDigest で有効化）
	adigest       *digest.Audio // ゴールデン音声回帰用の連鎖ハッシュ（任意・EnableAudioDigest で有効化）
	acap          *audioCapture // 生音声サンプル取得（任意・EnableAudioCapture で有効化, V2-15）
	paddlePlugged [2]bool       // SetPaddle がポートへ paddle peripheral を差したか（冪等化, V2-4b）

	cov *Coverage // PC/分岐カバレッジ記録（任意・EnableCoverage で有効化, VV-3）。nil=無効でゼロコスト

	// AT-5: per-pixel の描画オブジェクト帰属（PF/P0/P1/M0/M1/BL/BG）。elemCB を毎カラー
	// クロック呼び、GetLastSignal().Index を索引に Video.LastElement を記録する。
	// 値は element+1（0=未記録）。elemCB=nil のとき VCS.Step(nil) と等価＝ゼロコスト。
	elemBuf []uint8          // 索引 = signal.Index（フルフレーム 228×scanline 空間）
	// hmRipple / hmFlags は elemBuf と同じ索引で HMOVE 機構の状態を毎カラークロック記録する。
	// 現在値の読み出しだけでは足りない：リップルはカラークロック単位で動くので、命令単位で
	// 標本化すると値が飛び、実測で 16 値中 3 値しか見えず終了も一度も捕まらなかった。
	hmRipple []uint8
	hmFlags  []uint8
	// elemCtBuf は elemBuf と同じ索引で「その要素の何番目のコピーが描いたか」。
	// player/missile なら NUSIZ 複製のコピー番号、playfield なら PF0/1/2 のどれか。
	// 0 は未記録の番兵で、格納値は実際の値 +1。
	elemCtBuf []uint8
	elemCB  func(bool) error // 事前確保クロージャ（stepInstr で毎回渡す。per-call alloc 回避）

	// フレーム内ウォッチ（StartFrameWatch / FrameWatch）。watching=false でゼロコスト。
	watching bool
	cxAccum  video.CollisionEvent // 起きた衝突の OR 蓄積（CXCLR に消されない）
	spLow    uint8                // SP が到達した最小値
	spHigh   uint8                // SP が到達した最大値（低い値に「固定」なのか「途中で降りた」のかを区別する）
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
	if err := e.VCS.Step(e.elemCB); err != nil { // elemCB=nil のとき従来どおり（ゼロコスト）
		return false, err
	}
	if e.watching {
		sp := uint8(e.VCS.CPU.SP.Address())
		if sp < e.spLow {
			e.spLow = sp
		}
		if sp > e.spHigh {
			e.spHigh = sp
		}
	}
	if ready {
		lr := e.VCS.CPU.LastResult
		e.cpuCycles += int64(lr.Cycles)
		if e.cov != nil && lr.Defn != nil { // VV-3: 命令完了時だけ記録（nil=無効でゼロコスト）
			e.cov.record(lr.Address, lr.Defn.IsBranch(), lr.BranchSuccess)
		}
		return true, nil
	}
	return false, nil
}

// EnableCoverage は PC/分岐カバレッジ記録を有効化する（以後の実行を記録）。冪等。
func (e *Emu) EnableCoverage() {
	if e.cov == nil {
		e.cov = newCoverage()
	}
}

// Coverage は記録したカバレッジを返す（EnableCoverage していなければ nil）。
func (e *Emu) Coverage() *Coverage { return e.cov }

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
	// 決定化（regression 用）。電源投入時状態には乱数源が2系統ある:
	//  (1) Random.Rewindable（座標ベース）… Normalise() の ZeroSeed=true で決定化される（RAM open-bus 等）。
	//  (2) 埋め込み *math/rand.Rand（time.Now() 種）… CPU.Reset が rnd.Intn() で PC/A/X/Y/Status/SP を
	//      ランダム初期化する経路。**ZeroSeed は (2) に効かない**ため、同一プラットフォーム同一コードでも
	//      run ごとに CPU 初期状態が変わり、smoke 系テストが稀に flaky になっていた（TIA レジスタ/衝突）。
	// → 両系統を潰す: Normalise() で (1) を、固定種の rand へ差し替えで (2) を決定化。AttachCartridge
	//    （LoadROM）でのリセット前に行うので、以後の CPU 初期状態は毎 run・全プラットフォームで同一になる。
	vcs.Env.Normalise()
	vcs.Env.Random.Rand = rand.New(rand.NewSource(0))
	e := &Emu{TV: tv, VCS: vcs, cap: cap}
	e.EnableElementCapture() // AT-5: decompose_row 用に per-pixel 帰属を常時記録（実測は不変・オーバヘッド極小）
	return e, nil
}

// EnableElementCapture は per-pixel の描画オブジェクト帰属記録を有効化する（冪等）。
// 以後の stepInstr が毎カラークロック Video.LastElement を elemBuf[signal.Index] に書く。
// エミュレーション結果（色/サイクル）は一切変えない＝観測専用。
func (e *Emu) EnableElementCapture() {
	if e.elemCB != nil {
		return
	}
	e.elemBuf = make([]uint8, specification.AbsoluteMaxClks)
	e.elemCtBuf = make([]uint8, specification.AbsoluteMaxClks)
	e.hmRipple = make([]uint8, specification.AbsoluteMaxClks)
	e.hmFlags = make([]uint8, specification.AbsoluteMaxClks)
	e.elemCB = func(isCycle bool) error {
		sig := e.VCS.TV.GetLastSignal()
		if sig.Index == signal.NoSignal {
			return nil
		}
		if sig.Index >= 0 && sig.Index < len(e.elemBuf) {
			// element(0..6) を +1 して格納（0=未記録の番兵）
			e.elemBuf[sig.Index] = uint8(e.VCS.TIA.Video.LastElement) + 1
			// WHICH copy drew the pixel, alongside WHICH object. The TIA already
			// carries it next to LastElement and we were throwing it away: for a
			// player or missile it is the sprite's copy index (NUSIZ replication),
			// for the playfield it is which of PF0/PF1/PF2 lit the pixel. Without
			// it, three copies of a sprite and one wide sprite are the same
			// observation, which is precisely the confusion that made a generated
			// clone of an all-NUSIZ-modes ROM miss 2666 cells and then blame
			// multiplexing for it.
			e.elemCtBuf[sig.Index] = uint8(e.VCS.TIA.Video.LastElementCt) + 1
			// HMOVE machinery, same clock, same index. Two bytes per colour clock —
			// the engine's own reflection package records 288 for the same moment.
			hm := &e.VCS.TIA.Hmove
			e.hmRipple[sig.Index] = hm.Ripple
			var f uint8 = hmRecorded
			if hm.Latch {
				f |= hmLatch
			}
			if hm.IsActive() {
				f |= hmRippleActive
			}
			if hm.RippleJustEnded {
				f |= hmJustEnded
			}
			if hm.Future.IsActive() {
				f |= hmDelayActive
			}
			e.hmFlags[sig.Index] = f
		}
		// フレーム内で起きた衝突の蓄積（下記 FrameWatch）。LastColorClock は
		// 毎ビデオサイクル reset されるので、ここで拾わないと二度と見えない。
		if e.watching {
			e.cxAccum |= e.VCS.TIA.Video.Collisions.LastColorClock
		}
		return nil
	}
}

// --- フレーム内ウォッチ（衝突の蓄積・スタック到達点） ---
//
// フレーム境界だけを標本化すると、この 2 つは取り逃す：
//
//   - 衝突ラッチ CXxx は sticky だが、ROM はふつう毎フレーム CXCLR で消す。フレーム末に
//     読むと「その時点で残っていたもの」しか見えず、フレーム中に起きて消された衝突は
//     観測できない。「このゲームは衝突を使っていない」を*証明*したいのに、証拠が
//     消えた後に見ていることになる。
//   - スタックポインタはフレーム末にはほぼ必ず $FF に戻っている。実測でも Outlaw の
//     stack low-water はフレーム境界サンプリングだと常に $FF ＝ RAM のどこをスタックが
//     踏んだかを 1 バイトも除外できていなかった。
//
// どちらもフレームの*中*でしか見えない。衝突は per-videocycle の LastColorClock を
// 毎カラークロックで OR 蓄積し（CXCLR と独立に、起きた事実そのものを拾う）、SP は
// 命令ごとに最小値を追う。
//
// ビット割当の出典: Gopher2600 `hardware/tia/video/collisions.go`（CollisionEvent の
// 非公開ビットマスク定数）。ここは複製なので、ズレていないことを
// TestFrameWatchAgreesWithLatches が実測で検証する。
const (
	cxeM0P1 video.CollisionEvent = 1 << 0
	cxeM0P0 video.CollisionEvent = 1 << 1
	cxeM0PF video.CollisionEvent = 1 << 2
	cxeM0BL video.CollisionEvent = 1 << 3
	cxeM1P0 video.CollisionEvent = 1 << 4
	cxeM1P1 video.CollisionEvent = 1 << 5
	cxeM1PF video.CollisionEvent = 1 << 6
	cxeM1BL video.CollisionEvent = 1 << 7
	cxeP0PF video.CollisionEvent = 1 << 8
	cxeP0BL video.CollisionEvent = 1 << 9
	cxeP1PF video.CollisionEvent = 1 << 10
	cxeP1BL video.CollisionEvent = 1 << 11
	cxeBLPF video.CollisionEvent = 1 << 12
	cxeP0P1 video.CollisionEvent = 1 << 13
	cxeM0M1 video.CollisionEvent = 1 << 14
	// bit 15 は CXCLR イベント＝衝突ではないので蓄積対象外。
)

// StartFrameWatch は蓄積をリセットして観測を開始する（毎フレームの直前に呼ぶ）。
func (e *Emu) StartFrameWatch() {
	e.watching = true
	e.cxAccum = 0
	sp := uint8(e.VCS.CPU.SP.Address())
	e.spLow, e.spHigh = sp, sp
}

// FrameWatch は StartFrameWatch 以降に**実際に起きた**衝突すべてと、SP が到達した
// 最小値を返す。衝突は CXCLR で消されていても残る＝「起きたか」を答える。
func (e *Emu) FrameWatch() (Collisions, uint8) {
	a := e.cxAccum
	has := func(m video.CollisionEvent) bool { return a&m != 0 }
	return Collisions{
		M0P1: has(cxeM0P1), M0P0: has(cxeM0P0), M0PF: has(cxeM0PF), M0BL: has(cxeM0BL),
		M1P0: has(cxeM1P0), M1P1: has(cxeM1P1), M1PF: has(cxeM1PF), M1BL: has(cxeM1BL),
		P0PF: has(cxeP0PF), P0BL: has(cxeP0BL), P1PF: has(cxeP1PF), P1BL: has(cxeP1BL),
		BLPF: has(cxeBLPF), P0P1: has(cxeP0P1), M0M1: has(cxeM0M1),
	}, e.spLow
}

// FrameWatchSPRange は StartFrameWatch 以降に SP が動いた範囲を返す。low だけでは
// 「ずっと低い値に固定」と「高い所から降りてきた」が区別できない＝スタックとして
// 使われているのか、SP を別用途に向けているのかの判別に high が要る。
func (e *Emu) FrameWatchSPRange() (low, high uint8) { return e.spLow, e.spHigh }

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
	wide := func(nusiz uint8) int { // NUSIZ サイズビット → ピクセル倍率（$05=2x, $07=4x）
		switch nusiz & 0x07 {
		case 0x05:
			return 2
		case 0x07:
			return 4
		}
		return 1
	}
	return []annotate.Marker{
		{Label: "P0", Clock: v.Player0.HmovedPixel, Col: color.RGBA{230, 60, 60, 255},
			Gfx: v.Player0.GfxDataNew, Reflect: v.Player0.Reflected, Wide: wide(v.Player0.SizeAndCopies)},
		{Label: "M0", Clock: v.Missile0.HmovedPixel, Col: color.RGBA{235, 140, 40, 255}},
		{Label: "P1", Clock: v.Player1.HmovedPixel, Col: color.RGBA{230, 215, 50, 255},
			Gfx: v.Player1.GfxDataNew, Reflect: v.Player1.Reflected, Wide: wide(v.Player1.SizeAndCopies)},
		{Label: "M1", Clock: v.Missile1.HmovedPixel, Col: color.RGBA{70, 200, 70, 255}},
		{Label: "BL", Clock: v.Ball.HmovedPixel, Col: color.RGBA{180, 90, 210, 255}},
	}
}

// ObjectColorRGBA returns object idx's rendered colour (0=P0 1=M0 2=P1 3=M1 4=BL)
// — the palette-mapped COLUPx/COLUM/COLUBL used to draw it. Pairs with
// Markers()[idx].Clock (the object's X) to find the object by its OWN colour,
// robust where a "first non-background pixel" scan latches onto the playfield
// frame/border instead of a small missile.
func (e *Emu) ObjectColorRGBA(idx int) color.RGBA {
	r := e.ReadTIARegisters()
	var code uint8
	switch idx {
	case 0:
		code = r.Player0.Color
	case 1:
		code = r.Missile0.Color
	case 2:
		code = r.Player1.Color
	case 3:
		code = r.Missile1.Color
	case 4:
		code = r.Ball.Color
	}
	return e.cap.colorRGBA(code)
}

// ObjectYExtent returns object idx's drawn vertical extent this frame — the top
// and bottom VISIBLE scanline (grid-y, same coord as ReadRow) whose pixel matches
// the object's own colour at/near its X column. present=false when the object is
// disabled / off-screen (no matching pixel). This is the numeric Y / vertical
// readout that read_tia (X only) and read_motion (rendered-top, unreliable for a
// 1-2px missile against a bordered field) lack — for tracing a bullet's trajectory
// / ricochet as numbers. Caveat: matches the object's colour, so another object of
// the SAME colour within ~8px to the right (e.g. a just-fired missile overlapping
// its player) can widen the extent; once separated it is clean.
func (e *Emu) ObjectYExtent(idx int) (yTop, yBot, height int, present bool) {
	if idx < 0 || idx >= 5 {
		return 0, 0, 0, false
	}
	x := e.Markers()[idx].Clock
	if x < 0 || x > 159 {
		return 0, 0, 0, false
	}
	want := e.ObjectColorRGBA(idx)
	img, visTop := e.Snapshot()
	b := img.Bounds()
	const span = 8 // missile occupies x..x+1; a player up to ~x+15 (left half suffices)
	yTop, yBot = 1<<30, -1
	for row := 0; row < b.Dy(); row++ {
		for dx := 0; dx < span && x+dx < b.Dx(); dx++ {
			p := img.RGBAAt(x+dx, row)
			if p.R == want.R && p.G == want.G && p.B == want.B { // RGB only (alpha differs)
				sl := row + visTop
				if sl < yTop {
					yTop = sl
				}
				if sl > yBot {
					yBot = sl
				}
				break
			}
		}
	}
	if yBot < 0 {
		return 0, 0, 0, false
	}
	return yTop, yBot, yBot - yTop + 1, true
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

// HmoveState is the TIA's horizontal-motion machinery caught mid-operation.
//
// HMOVE is not instantaneous: the strobe sets a latch after a delay, a ripple
// counter then runs 15→0 handing extra ticks to the sprites, and only when it
// expires has an object finished moving. Every "the sprite landed somewhere I did
// not predict" bug lives in that window, and until now the only observable was
// the final position — the machinery producing it was invisible, so a wrong
// answer could only be guessed at, not watched.
// Flags packed into hmFlags, one byte per colour clock. hmRecorded is the
// sentinel that separates "nothing happening here" from "never sampled here" —
// without it an unvisited clock and an idle one are the same zero, and a query
// would report calm where it has no data at all.
const (
	hmRecorded uint8 = 1 << iota
	hmLatch
	hmRippleActive
	hmJustEnded
	hmDelayActive
)

type HmoveState struct {
	Latch  bool  `json:"latch"`  // HMOVE has been triggered on this scanline (cleared at line start)
	Ripple uint8 `json:"ripple"` // counts 15 down to 0, then rests at 255 (= -1)
	// RippleActive is the TIA's OWN definition of "the counter is running", not a
	// test invented here. The obvious `Ripple != 255` is wrong at both ends: it
	// calls the idle-but-not-yet-wrapped value 0 active, and it calls the cycle the
	// counter expires on inactive.
	RippleActive    bool `json:"ripple_active"`
	RippleJustEnded bool `json:"ripple_just_ended"` // it expired on this very colour clock
	// DelayActive covers the gap between the HMOVE strobe and the ripple starting.
	// An object is committed to move during it while nothing has moved yet, which
	// is the part of the sequence that is invisible from the position alone.
	DelayActive    bool `json:"delay_active"`
	DelayRemaining int  `json:"delay_remaining"`
	Phi2           bool `json:"phi2"` // TIA phase clock is in rising Phi2 (when the ripple ticks)
}

// Hmove returns the current horizontal-motion state.
//
// HMOVE is not instantaneous: the strobe schedules a latch, a ripple counter then
// runs 15→0 handing extra ticks to the sprites, and only when it expires has an
// object finished moving. Every "the sprite landed somewhere I did not predict"
// bug lives in that window, and the only thing observable until now was the final
// position — the machinery producing it was invisible, so a wrong landing could
// be guessed at but not watched.
//
// Observation only: reading this cannot change what the machine does.
func (e *Emu) Hmove() HmoveState {
	h := &e.VCS.TIA.Hmove
	return HmoveState{
		Latch:           h.Latch,
		Ripple:          h.Ripple,
		RippleActive:    h.IsActive(),
		RippleJustEnded: h.RippleJustEnded,
		DelayActive:     h.Future.IsActive(),
		DelayRemaining:  h.Future.Remaining(),
		Phi2:            h.Clk,
	}
}

// HmoveSpan is where HMOVE acted on one scanline, in read_row beam coordinates
// (HBLANK −68..−1, visible 0..159).
type HmoveSpan struct {
	Scanline int  `json:"scanline"`
	Latched  bool `json:"latched"`  // HMOVE was triggered on this line at all
	Recorded bool `json:"recorded"` // the line was actually sampled (else every field below is meaningless)
	// StrobeClock is the first clock at which the strobe's delay was pending, i.e.
	// the earliest visible consequence of the HMOVE write. −999 when there is none.
	StrobeClock int `json:"strobe_clock"`
	// RippleStart/RippleEnd bracket the clocks over which the counter ran. Objects
	// are still moving inside this span, so a position read within it is a
	// half-finished answer — which is exactly how a "wrong" landing gets recorded.
	RippleStart int `json:"ripple_start"`
	RippleEnd   int `json:"ripple_end"`
	RippleTicks int `json:"ripple_ticks"` // colour clocks the counter was active
	// Values is every distinct ripple value seen, in the order first seen. The
	// counter runs 15→0, so anything else is a finding.
	Values []uint8 `json:"values"`
}

// HmoveOnScanline reports where HMOVE acted on the given absolute scanline.
//
// It reads the per-colour-clock recording rather than the live registers because
// the ripple counter advances every colour clock — three times per CPU cycle — so
// sampling it by instruction aliases it badly: measured on litmus_hmove, stepping
// by instruction saw 3 of the 16 counter values and never once caught the cycle
// it expired on. A caller debugging a mis-positioned sprite that way is being
// shown a value that was never the whole story.
func (e *Emu) HmoveOnScanline(scanline int) (HmoveSpan, error) {
	if e.hmFlags == nil {
		return HmoveSpan{}, fmt.Errorf("element capture not enabled")
	}
	base := scanline * specification.ClksScanline
	if base < 0 || base+specification.ClksScanline > len(e.hmFlags) {
		return HmoveSpan{}, fmt.Errorf("scanline %d out of recorded range", scanline)
	}
	sp := HmoveSpan{Scanline: scanline, StrobeClock: -999, RippleStart: -999, RippleEnd: -999}
	seen := map[uint8]bool{}
	for i := 0; i < specification.ClksScanline; i++ {
		f := e.hmFlags[base+i]
		if f&hmRecorded == 0 {
			continue
		}
		sp.Recorded = true
		clk := i - specification.ClksHBlank // read_row coordinates
		if f&hmLatch != 0 {
			sp.Latched = true
		}
		if f&hmDelayActive != 0 && sp.StrobeClock == -999 {
			sp.StrobeClock = clk
		}
		if f&hmRippleActive != 0 {
			if sp.RippleStart == -999 {
				sp.RippleStart = clk
			}
			sp.RippleEnd = clk
			sp.RippleTicks++
			if v := e.hmRipple[base+i]; !seen[v] {
				seen[v] = true
				sp.Values = append(sp.Values, v)
			}
		}
	}
	return sp, nil
}

// DisplayOff reports whether the beam is currently blanked — VSYNC or VBLANK
// asserted — as the television itself sees it, taken from the last signal the TIA
// sent rather than from any register value we kept a copy of.
//
// This is the runtime oracle for the static prover's blank/visible region
// classification: the prover claims a WSYNC-to-WSYNC region runs while the
// display is off, and that claim is only worth anything if the machine agrees.
func (e *Emu) DisplayOff() bool {
	sig := e.VCS.TV.GetLastSignal()
	return sig.VBlank
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

// 2600 の作業 RAM（RIOT・$80–$FF の 128 バイト）。
const (
	RAMBase = 0x80 // 最初の RAM 番地
	RAMSize = 128  // RAM の総バイト数
)

// CurrentRAM は 128 バイトの RAM 全域（$80–$FF）を副作用なしで一括に読む。
//
// PeekRAM を 128 回呼ぶのと値としては同じだが、呼ぶ側が「どの番地を見たいか」を
// **事前に宣言しなくてよくなる**のが本質。挙動再現（RAM トレース）では、どの番地が
// 意味を持つかは測る前には分からない＝毎フレーム全部を記録し、後から分類する。
// 添字は番地−$80（out[0] が $80）。
func (e *Emu) CurrentRAM() ([RAMSize]uint8, error) {
	var out [RAMSize]uint8
	for i := 0; i < RAMSize; i++ {
		b, err := e.VCS.Mem.Peek(uint16(RAMBase + i))
		if err != nil {
			return out, fmt.Errorf("peek RAM $%02X: %w", RAMBase+i, err)
		}
		out[i] = b
	}
	return out, nil
}

// TimerState は RIOT タイマの現在状態（VV-10 T-1 timer-wrap 検出の土台）。
type TimerState struct {
	INTIM          uint8 // 現在のカウンタ値
	TIMINT         uint8 // INSTAT/TIMINT レジスタ（bit7=expired/underflow, bit6=PA7）
	Expired        bool  // タイマが 0 を下回って wrap した（INTIM 0x00→0xFF・1T へ flip）＝G8 ハザード信号
	Divider        int   // 直近に要求された分周（1/8/64/1024）
	TicksRemaining int   // 次の減算までの CPU サイクル
}

// TimerWrapHit は read-after-wrap（G8）検出の記録。
type TimerWrapHit struct {
	Frame int    // 検出したフレーム
	PC    uint16 // INTIM を読んだ命令のアドレス
}

const intimCanonical = 0x0284 // INTIM の正規アドレス（全ミラーをここへ畳む）

// WatchTimerWrap は ROM を命令単位で frames フレーム進め、タイマが既に underflow/wrap
// 済み（Expired）の状態で INTIM を**読んだ**最初の箇所を flag する＝G8「timer-wrap」ハザード
// （区間が短すぎる／ポーリングが 0 を取り逃して wrap 後の値を消費している）。INTIM==0 で正しく
// 抜ける通常のポーリング（wrap 前に exit）は flag しない。該当が無ければ nil。
//
// 注意: INTIM の読み出しは Expired を解除する副作用がある（timer.go の reversion）。よって
// Expired は各命令の**実行前**に標本化し、「直前から Expired だった状態で INTIM を読んだ」を検出する
// （ポーリングが wrap をまたいで回り続けていれば次の反復で必ず捕捉される）。
func (e *Emu) WatchTimerWrap(frames int) (*TimerWrapHit, error) {
	start := e.Coords().Frame
	for e.Coords().Frame-start < frames {
		preExpired := e.TimerState().Expired
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		lr := e.VCS.CPU.LastResult
		if lr.Defn == nil || lr.Defn.Effect != instructions.Read {
			continue
		}
		canon, area := memorymap.MapAddress(lr.InstructionData, true)
		if area != memorymap.RIOT || canon != intimCanonical {
			continue
		}
		if preExpired {
			return &TimerWrapHit{Frame: e.Coords().Frame, PC: lr.Address}, nil
		}
	}
	return nil, nil
}

// UninitReadHit records a read of a RAM byte never written since reset (T-3).
type UninitReadHit struct {
	Frame int    // frame on which it occurred
	PC    uint16 // address of the reading instruction
	Addr  uint16 // the RAM address read before any write
}

// effectiveAddr は直近に完了した命令のメモリオペランドの実効アドレスを返す（mem=false なら
// メモリ非アクセス）。Gopher2600 は zero-page を Absolute/AbsoluteX/Y に畳む（Defn.Bytes==2 が
// zp）。X/Y は命令完了後の値（indexed load/store はインデクス用レジスタを破壊しないので不変）。
func (e *Emu) effectiveAddr() (addr uint16, mem bool) {
	lr := e.VCS.CPU.LastResult
	d := lr.Defn
	if d == nil {
		return 0, false
	}
	zp := d.Bytes == 2
	base := lr.InstructionData
	x := uint16(e.VCS.CPU.X.Value())
	y := uint16(e.VCS.CPU.Y.Value())
	peek := func(a uint16) uint16 { v, _ := e.VCS.Mem.Peek(a); return uint16(v) }
	switch d.AddressingMode {
	case instructions.Implied:
		// A push has no operand, but it certainly has an effective address: the
		// stack, at $0100+SP. On the 2600 page 1 mirrors the same addresses the
		// console decodes, so a program that aims SP at a TIA register turns PHA
		// into a store to it — the "stack trick" Combat uses for its missile
		// enable. Treating an implied opcode as touching no memory made that write
		// invisible to every tool here, while the screen showed it happening
		// (roms/litmus/litmus_stack_trick.asm turns the background green with a PHA).
		//
		// SP is read after the instruction completed and a push decrements it, so
		// the byte went one slot above where SP now points.
		switch d.Operator {
		case instructions.PHA, instructions.PHP:
			sp := uint16(e.VCS.CPU.SP.Address()) & 0xFF
			return 0x0100 | ((sp + 1) & 0xFF), true
		}
		return 0, false
	case instructions.Absolute:
		return base, true
	case instructions.AbsoluteX:
		if zp {
			return (base + x) & 0xFF, true
		}
		return base + x, true
	case instructions.AbsoluteY:
		if zp {
			return (base + y) & 0xFF, true
		}
		return base + y, true
	case instructions.PreIndexed: // (ind,X)
		ptr := (base + x) & 0xFF
		return peek(ptr) | peek((ptr+1)&0xFF)<<8, true
	case instructions.PostIndexed: // (ind),Y
		ptr := base & 0xFF
		return (peek(ptr) | peek((ptr+1)&0xFF)<<8) + y, true
	}
	return 0, false // Immediate / Implied / Relative / Indirect(JMP)
}

// WatchUninitRead は ROM を命令単位で frames 進め、reset 以降に一度も書かれていない RAM バイト
// ($80-$FF) を**読んだ**最初の箇所を flag する＝未初期化 RAM 読み（emu では決定値で通るが実機では
// 電源投入ゴミ＝"passes-in-emu / fails-on-HW"）。indexed/(ind),Y を含む実効アドレスで write を
// 追跡するので RAM クリアループ（`sta $80,x` 等＝1命令で多数書き）を誤検出しない。スタック
// (push/pull) は implied でオペランド無し＝対象外（自己整合的に push 済み）。該当が無ければ nil。
func (e *Emu) WatchUninitRead(frames int) (*UninitReadHit, error) {
	start := e.Coords().Frame
	var written [128]bool
	for e.Coords().Frame-start < frames {
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		lr := e.VCS.CPU.LastResult
		if lr.Defn == nil {
			continue
		}
		isRead := lr.Defn.Effect == instructions.Read
		if !isRead && lr.Defn.Effect != instructions.Write {
			continue
		}
		eff, isMem := e.effectiveAddr()
		if !isMem {
			continue
		}
		canon, area := memorymap.MapAddress(eff, isRead)
		if area != memorymap.RAM {
			continue
		}
		idx := canon & 0x7F
		if isRead {
			if !written[idx] {
				return &UninitReadHit{Frame: e.Coords().Frame, PC: lr.Address, Addr: canon}, nil
			}
		} else {
			written[idx] = true
		}
	}
	return nil, nil
}

// HMOVEHazardHit records an HMxx motion-register write too soon after an HMOVE (T-2).
type HMOVEHazardHit struct {
	Frame       int    // frame on which it occurred
	PC          uint16 // address of the HMxx-writing instruction
	CyclesAfter int    // CPU cycles since the HMOVE strobe (< 24 = the hazard)
}

// clockPos は現在のビーム位置を「絶対カラークロック」で返す（フレーム内 0..）。HMOVE 後の
// 危険窓は**実時間**なので CPU サイクル累積（WSYNC stall を含まない）ではなくこれで測る。
func (e *Emu) clockPos() (frame int, pos int64) {
	c := e.Coords()
	return c.Frame, int64(c.Scanline)*228 + int64(c.Clock) + 68 // Clock 規約 -68..159 を 0..227 へ
}

// WatchHMOVEHazard は ROM を命令単位で frames フレーム進め、HMOVE ストローブ後 **24 CPU
// サイクル（=72 カラークロック）以内**に動き系レジスタ（HMP0/HMP1/HMM0/HMM1/HMBL・HMCLR）へ
// 書き込んだ最初の箇所を flag する＝「動きが不定になる」既知ハザード（Stella PG）。WSYNC を挟む
// 正しいパターン（HMOVE は WSYNC 直後・HMxx は窓外）は flag しない。該当が無ければ nil。
func (e *Emu) WatchHMOVEHazard(frames int) (*HMOVEHazardHit, error) {
	start := e.Coords().Frame
	var hf int
	var hp int64
	armed := false
	for e.Coords().Frame-start < frames {
		if err := e.StepInstruction(); err != nil {
			return nil, err
		}
		lr := e.VCS.CPU.LastResult
		if lr.Defn == nil || lr.Defn.Effect != instructions.Write {
			continue
		}
		canon, area := memorymap.MapAddress(lr.InstructionData, false)
		if area != memorymap.TIA {
			continue
		}
		switch reg := canon; {
		case reg == 0x2A: // HMOVE ストローブ
			hf, hp = e.clockPos()
			armed = true
		case (reg >= 0x20 && reg <= 0x24) || reg == 0x2B: // HMP0..HMBL / HMCLR
			if armed {
				f, p := e.clockPos()
				if d := p - hp; f == hf && d >= 0 && d < 72 { // 72 clk = 24 CPU cy・同一フレーム
					return &HMOVEHazardHit{Frame: f, PC: lr.Address, CyclesAfter: int(d / 3)}, nil
				}
			}
		}
	}
	return nil, nil
}

// TimerState は副作用なしで RIOT タイマ内部状態を読む（PeekState 経由）。
func (e *Emu) TimerState() TimerState {
	t := e.VCS.RIOT.Timer
	timint := t.PeekState("timint").(uint8)
	return TimerState{
		INTIM:          t.PeekState("intim").(uint8),
		TIMINT:         timint,
		Expired:        timint&0x80 != 0,
		Divider:        int(t.PeekState("divider").(timer.Divider)),
		TicksRemaining: t.PeekState("ticksRemaining").(int),
	}
}

// Poke はメモリへ 1 バイト書き込む（poke ツール）。
func (e *Emu) Poke(addr uint16, val uint8) error {
	return e.VCS.Mem.Poke(addr, val)
}

// SetPaddle はパドル位置を注入する（V2-4b）。value は 0.0（最小=充電最速）〜1.0（最大）。
// 初回呼び出しでポートに paddle peripheral を差す（デフォルトの stick を置換・冪等）。
func (e *Emu) SetPaddle(player int, value float64) error {
	port := plugging.PortLeft
	idx := 0
	if player == 1 {
		port = plugging.PortRight
		idx = 1
	}
	if !e.paddlePlugged[idx] {
		if err := e.VCS.RIOT.Ports.Plug(port, controllers.NewPaddles); err != nil {
			return err
		}
		e.paddlePlugged[idx] = true
	}
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}
	motion := int16(value*65535 - 32768)
	_, err := e.VCS.RIOT.Ports.HandleInputEvent(ports.InputEvent{
		Port: port, Ev: ports.PaddleSet,
		D: ports.EventDataPaddle{Paddle: -1, Motion: motion, Relative: false},
	})
	return err
}

// Bank は「現在 PC が指すアドレス」のカートリッジバンク情報を返す（bankswitch ROM の検証用）。
// Gopher2600 の Cartridge.GetBank をそのまま使う（4K 非バンク ROM では常に 0, false）。
func (e *Emu) Bank() (number int, isRAM bool) {
	info := e.VCS.Mem.Cart.GetBank(e.VCS.CPU.PC.Value())
	return info.Number, info.IsRAM
}

// CartInfo reports what cartridge the engine decided this image is: the mapper
// it fingerprinted, and how many banks that mapper has.
//
// Size is not the machine. An 8192-byte image is F8 only unless it fingerprints
// as WF8/3F/E0/E7/WD/FE/UA, and a superchip variant overlays RAM on part of the
// window whatever the length says. Any analysis that infers the cartridge from
// len(rom) can therefore describe a different machine from the one its own
// acceptance tests run on.
func (e *Emu) CartInfo() (id string, banks int) {
	return e.VCS.Mem.Cart.ID(), e.VCS.Mem.Cart.NumBanks()
}

// CartBank is one bank of a cartridge as the engine hands it over for static
// analysis: the bytes, and the origins in the 4K window this bank may be mapped
// to. A 2K image answers TWICE inside that window, so Origins has two entries for
// it — which is why the origin must come from the mapper rather than be assumed.
type CartBank struct {
	Number  int
	Data    []uint8
	Origins []uint16
}

// CopyBanks returns every bank's bytes, taken through the mapper's own
// disassembly path so reading them cannot disturb the running cartridge (a plain
// cartridge access has bank-switching side effects — that is the whole hazard).
func (e *Emu) CopyBanks() ([]CartBank, error) {
	cs, err := e.VCS.Mem.Cart.CopyBanks()
	if err != nil {
		return nil, err
	}
	out := make([]CartBank, 0, len(cs))
	for _, c := range cs {
		out = append(out, CartBank{Number: c.Number, Data: c.Data, Origins: c.Origins})
	}
	return out, nil
}

// BankSwitchHotspots returns the cartridge addresses that select a bank when
// touched, keyed in the primary mirror ($1000-$1FFF), with the mapper's own
// symbol for each.
//
// Only HotspotBankSwitch entries are returned. The other kinds — function,
// register read/write, reserved — do things that are not "the next instruction
// comes from a different bank", and lumping them together would put edges in a
// control-flow graph that the hardware does not have.
//
// A nil map with ok=false means the mapper publishes no hotspot table at all.
// That is not "there are none": every mapper whose switch is driven by a data
// value, by bus timing, or by an address OUTSIDE cartridge space (3F, 3E, UA, SB,
// FE, WD) is in that group, and for those an address-hotspot model does not
// merely lose precision — it misses the switch entirely, which is the unsound
// direction. Callers must decline rather than assume an empty set.
func (e *Emu) BankSwitchHotspots() (map[uint16]string, bool) {
	bus := e.VCS.Mem.Cart.GetCartHotspotsBus()
	if bus == nil {
		return nil, false
	}
	out := map[uint16]string{}
	for addr, info := range bus.Hotspots() {
		if info.Action == mapper.HotspotBankSwitch {
			out[addr] = info.Symbol
		}
	}
	return out, true
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

// SetPanel は本体パネルスイッチ（Game Reset / Game Select）を操作する。
// 多くの市販ゲームはジョイスティックでなく RESET スイッチで開始する（fieldtest -auto の土台）。
func (e *Emu) SetPanel(switchName string, pressed bool) error {
	var ev ports.Event
	switch switchName {
	case "reset":
		ev = ports.PanelReset
	case "select":
		ev = ports.PanelSelect
	case "color": // pressed=true → カラー / false → 白黒（SWCHB D3）
		ev = ports.PanelSetColor
	case "p0pro": // pressed=true → P0 難易度 A(Pro) / false → B（SWCHB D6）
		ev = ports.PanelSetPlayer0Pro
	case "p1pro": // SWCHB D7
		ev = ports.PanelSetPlayer1Pro
	default:
		return fmt.Errorf("unknown panel switch %q (want reset/select/color/p0pro/p1pro)", switchName)
	}
	_, err := e.VCS.RIOT.Ports.HandleInputEvent(ports.InputEvent{Port: plugging.PortPanel, Ev: ev, D: pressed})
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
// 注: グリッドの y ラベルは visibleTop + 画像行（annotate.Render と同じ式）。ここでも
// visibleTop を引いて画像行へ変換する＝「グリッドで見えた y をそのまま渡せる」を守る
// （v1.4.0 修正: 以前はクロップ後の画像行を直接受けており、グリッドと ~visibleTop ずれていた）。
func (e *Emu) ReadRow(scanline int) (runs []RowRun, width int, err error) {
	img, visibleTop := e.cap.snapshot()
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	row := scanline - visibleTop
	if row < 0 || row >= h {
		return nil, w, fmt.Errorf("scanline %d out of visible range %d..%d", scanline, visibleTop, visibleTop+h-1)
	}
	hexAt := func(x int) string {
		c := img.RGBAAt(x, row)
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

// ElemRun は DecomposeRow の連長エンコード 1 区間。可視 clock [Clock, Clock+Len) を
// 同一の TIA オブジェクト Element が描いている。
type ElemRun struct {
	Clock   int    `json:"clock"`   // 区間先頭の可視 clock（0..159）
	Len     int    `json:"len"`     // 連続ピクセル数
	Element string `json:"element"` // BG/PF/P0/P1/M0/M1/BL（none=未記録）
	// Copy は「その要素の何番目のコピーが描いたか」。player/missile は NUSIZ 複製の
	// コピー番号、playfield は PF0/1/2 のどれか。未記録は -1。
	Copy int `json:"copy"`
}

// elemNames は video.Element(0..6) → 表示名。索引は elemBuf 格納値 −1。
var elemNames = [...]string{"BG", "BL", "PF", "P0", "P1", "M0", "M1"}

// DecomposeRow は指定可視 scanline（ReadRow と同じ絶対 scanline 座標）を「どの TIA
// オブジェクトが各ピクセルを描いたか」で連長エンコードして返す。ReadRow（色）・beamtrace
// （書込）に無い帰属マップ＝商用 ROM のクリーンルーム解析用（AT-5）。可視 clock x は
// elemBuf 索引 scanline*228 + 68 + x（read_row と同座標）。要 EnableElementCapture。
func (e *Emu) DecomposeRow(scanline int) (runs []ElemRun, width int, err error) {
	if e.elemBuf == nil {
		return nil, specification.ClksVisible, fmt.Errorf("element capture not enabled")
	}
	vt := e.cap.frameInfo.VisibleTop
	vb := e.cap.frameInfo.VisibleBottom
	if scanline < vt || scanline > vb {
		return nil, specification.ClksVisible, fmt.Errorf("scanline %d out of visible range %d..%d", scanline, vt, vb)
	}
	elemAt := func(x int) (string, int) {
		idx := scanline*specification.ClksScanline + specification.ClksHBlank + x
		if idx < 0 || idx >= len(e.elemBuf) {
			return "none", -1
		}
		v := e.elemBuf[idx]
		if v == 0 || int(v) > len(elemNames) {
			return "none", -1
		}
		cpy := -1
		if idx < len(e.elemCtBuf) && e.elemCtBuf[idx] > 0 {
			cpy = int(e.elemCtBuf[idx]) - 1
		}
		return elemNames[v-1], cpy
	}
	// A run breaks on the COPY as well as on the element: two adjacent copies of a
	// sprite are one uninterrupted stretch of the same colour, so merging them
	// would report a single wide object where the hardware drew two narrow ones —
	// exactly the ambiguity that let all eight NUSIZ modes look alike.
	for x := 0; x < specification.ClksVisible; x++ {
		el, cpy := elemAt(x)
		if n := len(runs); n > 0 && runs[n-1].Element == el && runs[n-1].Copy == cpy {
			runs[n-1].Len++
			continue
		}
		runs = append(runs, ElemRun{Clock: x, Len: 1, Element: el, Copy: cpy})
	}
	return runs, specification.ClksVisible, nil
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
	// ★フレッシュブート時のみ（Frame<2）。無条件 warmup は「poke で作った整列状態のフレーム」を
	// 監視前に食い潰す欠陥だった（PONG-C1 ロールアウトで実測発見：77cy 整列超過が poke+assert で
	// 再現不能だった真因。トラジェクトリ経由の超過は状態が持続するため偶然検出できていた）。
	if e.VCS.TV.GetCoords().Frame < 2 {
		if err := e.RunFrames(2); err != nil {
			return false, 0, 0, err
		}
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

// LineWorst は ProfileLineWorst の1行＝「ある STA WSYNC が開く論理ライン」の実測ワースト。
// キーは開き strobe の PC（オーバースキャン物理行のように scanline が毎フレーム漂う行でも
// コード位置で安定に集計できる）。cycles の定義は静的 prover の Region.Worst と同一＝
// 「開き WSYNC の解放（次ライン先頭）から、閉じ WSYNC ストア完了まで」。≤76 なら1ラインに収まる。
type LineWorst struct {
	StrobePC      uint16  `json:"strobe_pc"`       // 開き STA WSYNC の PC
	At            string  `json:"at,omitempty"`    // ソース位置（cmd/harness が srcmap で充填・emu は触らない）
	Count         int     `json:"count"`           // 計測できた区間数
	WorstCycles   int     `json:"worst_cycles"`    // 実測ワースト CPU サイクル（ビーム座標から厳密算出）
	WorstLines    int     `json:"worst_lines"`     // ワースト時に消費した物理ライン数（1=予算内の形）
	WorstFrame    int     `json:"worst_frame"`     // ワーストが出たフレーム
	WorstScanline int     `json:"worst_scanline"`  // ワースト区間の開始 scanline（開き strobe の次ライン）
	Watch         []uint8 `json:"watch,omitempty"` // ワースト区間の開き strobe 時点の RAM watch 値（=その行が読む引数）
}

// ProfileLineWorst は maxFrames フレーム走らせ、WSYNC 区間ごとの実測ワーストサイクルを
// 開き strobe の PC 別に集計して返す（PONG-C3: assert_line_budget の定量版）。
// assert が「超えたか/どこで」しか言えないのに対し、こちらは「各行が最悪何 cy 使い、
// あと何 cy 削ればよいか」を直接示す。watch には ZP アドレス群を渡すと、各行のワースト
// 区間開始時点の値（その行が読む引数＝ワーストパスの条件）を併記する。
//
// サイクル算出: WSYNC 解放は必ず次スキャンライン先頭（clock -68）なので、
//
//	clocks = (閉じstrobe.Scanline − (開きstrobe.Scanline+1))×228 + (閉じstrobe.Clock+68)
//	cycles = clocks/3（CPU 1cy=3clock・解放はサイクル境界に整列）
//
// フレームを跨ぐ区間（オーバースキャン最終行→次フレーム VSYNC）は集計しない。
func (e *Emu) ProfileLineWorst(maxFrames int, watch []uint16) ([]LineWorst, error) {
	// フレッシュブート時のみ安定化（RunUntilBudget と同じ規律＝poke 状態を食い潰さない）。
	if e.VCS.TV.GetCoords().Frame < 2 {
		if err := e.RunFrames(2); err != nil {
			return nil, err
		}
	}

	target := e.VCS.TV.GetCoords().Frame + maxFrames
	rows := map[uint16]*LineWorst{}

	var (
		havePrev     bool
		prevPC       uint16
		prevFrame    int
		prevScanline int
		prevWatch    []uint8
	)
	snap := func() []uint8 {
		if len(watch) == 0 {
			return nil
		}
		vals := make([]uint8, len(watch))
		for i, a := range watch {
			v, err := e.PeekRAM(a)
			if err == nil {
				vals[i] = v
			}
		}
		return vals
	}

	for {
		if e.VCS.CPU.Jammed {
			break
		}
		if e.VCS.TV.IsFrameNum(target) {
			break
		}
		pcBefore := e.VCS.CPU.PC.Address()
		executed, err := e.stepInstr()
		if err != nil {
			return nil, err
		}
		// WSYNC の検出は「RdyFlg が true→false になった瞬間」ではなく、WSYNC への
		// **ストアそのもの**で行う。停止時間が 1 命令ステップ未満で終わる WSYNC は
		// RdyFlg の遷移として観測できず、実測でその取りこぼしが起きていた：
		// bitmap48 の Krow($F0D0) は 8 フレームで 192 回実行されるのに、遷移方式では
		// 108 区間しか数えられなかった。取りこぼすと区間が次のストローブまで連結し、
		// 本来 1 行の区間が 13 行・987 サイクルとして報告される。静的証明（93）を
		// 実機が破っているように見えた原因はこれで、証明器ではなく測定器の側だった。
		// Only on a step that actually retired an instruction: LastResult — and so
		// LastTIAWrite — is unchanged while the CPU is stalled, so the same strobe
		// would otherwise be seen again on every step of its own halt, closing a
		// string of zero-length intervals.
		wsyncNow := false
		if executed {
			if w, ok := e.LastTIAWrite(); ok && w.Reg == 0x02 {
				wsyncNow = true
			}
		}
		if wsyncNow { // WSYNC ストローブ完了
			c := e.VCS.TV.GetCoords()
			if havePrev && c.Frame == prevFrame {
				clocks := (c.Scanline-(prevScanline+1))*228 + (c.Clock + 68)
				if clocks >= 0 {
					cycles := (clocks + 2) / 3 // 完了座標の端数（0..2clock）を吸収する切り上げ
					lines := c.Scanline - prevScanline
					if lines < 1 {
						lines = 1
					}
					row := rows[prevPC]
					if row == nil {
						row = &LineWorst{StrobePC: prevPC}
						rows[prevPC] = row
					}
					row.Count++
					if cycles > row.WorstCycles {
						row.WorstCycles = cycles
						row.WorstLines = lines
						row.WorstFrame = prevFrame
						row.WorstScanline = prevScanline + 1
						row.Watch = prevWatch
					}
				}
			}
			prevPC = pcBefore
			prevFrame = c.Frame
			prevScanline = c.Scanline
			prevWatch = snap()
			havePrev = true
		}
	}

	out := make([]LineWorst, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].WorstCycles != out[j].WorstCycles {
			return out[i].WorstCycles > out[j].WorstCycles
		}
		return out[i].StrobePC < out[j].StrobePC
	})
	return out, nil
}

// WatchRAM は RAM[addr] が変化するまで命令単位で実行する（watch_ram ツールの心臓部）。
// 戻り: 変化したか・旧値/新値・変化時の PC。max_frames で打ち切り。
// 注: Gopher2600 の VCS.Step は命令途中で中断できない（colorClockCallback は観測専用）ため、
// 検出粒度は命令単位。同値書込（変化なしの sta）は peek では検出できない＝change 検出に限定。
func (e *Emu) WatchRAM(addr uint16, maxFrames int) (changed bool, oldVal, newVal uint8, pc uint16, err error) {
	oldVal, err = e.PeekRAM(addr)
	if err != nil {
		return false, 0, 0, 0, err
	}
	startFrame := e.Coords().Frame
	for e.Coords().Frame < startFrame+maxFrames {
		p := e.VCS.CPU.PC.Address()
		if err := e.StepInstruction(); err != nil {
			return false, oldVal, 0, 0, err
		}
		v, err := e.PeekRAM(addr)
		if err != nil {
			return false, oldVal, 0, 0, err
		}
		if v != oldVal {
			return true, oldVal, v, p, nil
		}
	}
	return false, oldVal, oldVal, 0, nil
}

// InstrTrace は 1 命令ぶんのビーム解剖（trace_clocks ツールの単位）。
type InstrTrace struct {
	PC         string `json:"pc"`
	Opcode     uint8  `json:"opcode"`
	Cycles     int    `json:"cycles"`      // CPU サイクル（WSYNC stall 含むときは大きくなる）
	StartLine  int    `json:"start_line"`  // 開始 scanline
	StartClock int    `json:"start_clock"` // 開始 color clock（-68..159）
	EndLine    int    `json:"end_line"`
	EndClock   int    `json:"end_clock"`
	At         string `json:"at,omitempty"` // ソース位置（cmd/harness が srcmap で充填・emu は触らない）
}

// TraceClocks は次の maxInstr 命令を実行しながら、各命令のビーム上の開始/終了位置を返す。
// Gopher2600 は命令途中で停止できない（colorClockCallback は観測専用）ため、これは
// 「サブ命令粒度の観測」を提供する step_clock の実用回収版（docs/mcp-tools.md 参照）。
func (e *Emu) TraceClocks(maxInstr int) ([]InstrTrace, error) {
	var out []InstrTrace
	for i := 0; i < maxInstr; i++ {
		c0 := e.Coords()
		pc := e.VCS.CPU.PC.Address()
		op, _ := e.VCS.Mem.Read(pc)
		if err := e.StepInstruction(); err != nil {
			return out, err
		}
		c1 := e.Coords()
		out = append(out, InstrTrace{
			PC:         fmt.Sprintf("$%04X", pc),
			Opcode:     op,
			Cycles:     e.LastCycles(),
			StartLine:  c0.Scanline,
			StartClock: c0.Clock,
			EndLine:    c1.Scanline,
			EndClock:   c1.Clock,
		})
	}
	return out, nil
}

// --- dissect 用の軽量アクセサ ---

// PC は現在のプログラムカウンタ。
func (e *Emu) PC() uint16 { return e.VCS.CPU.PC.Address() }

// A/XReg/YReg はレジスタ現在値（store トレースの値取得用）。
func (e *Emu) A() uint8    { return e.VCS.CPU.A.Value() }
func (e *Emu) XReg() uint8 { return e.VCS.CPU.X.Value() }
func (e *Emu) YReg() uint8 { return e.VCS.CPU.Y.Value() }

// TIAWrite は直近 StepInstruction が行った TIA 書込（beamtrace 用）。
type TIAWrite struct {
	Reg    uint16 // 正規化した書込専用レジスタアドレス（$00-$2C）
	Val    uint8  // 書き込んだバイト（STA/STX/STY のとき HasVal=true）
	HasVal bool   // strobe / read-modify-write は値不明 → false
	PC     uint16 // 書込命令のアドレス
}

// LastTIAWrite は直近 StepInstruction が TIA 書込専用レジスタへ書いたかを返す。検出は
// WatchHMOVEHazard と同じ（Effect==Write かつ実効アドレスが TIA 領域）。値は store の
// ソースレジスタを命令後に読む（store はソースを変えないので一致）。ストローブ等は値不明。
func (e *Emu) LastTIAWrite() (TIAWrite, bool) {
	lr := e.VCS.CPU.LastResult
	if lr.Defn == nil || lr.Defn.Effect != instructions.Write {
		return TIAWrite{}, false
	}
	// The register written is the EFFECTIVE address, not the base operand.
	// `sta COLUP0,x` with x=3 reaches COLUBK, and a multiplexed kernel driving
	// several objects from one indexed loop is exactly the case a write→beam
	// timeline exists to explain — so using the operand here names the wrong
	// register in the only situation that matters. effectiveAddr resolves every
	// addressing mode; a store does not modify X/Y, so reading them after the
	// instruction gives the values it actually used.
	eff, ok := e.effectiveAddr()
	if !ok {
		return TIAWrite{}, false
	}
	canon, area := memorymap.MapAddress(eff, false)
	if area != memorymap.TIA {
		return TIAWrite{}, false
	}
	w := TIAWrite{Reg: canon, PC: lr.Address}
	switch lr.Defn.Operator {
	case instructions.STA:
		w.Val, w.HasVal = e.A(), true
	case instructions.STX:
		w.Val, w.HasVal = e.XReg(), true
	case instructions.STY:
		w.Val, w.HasVal = e.YReg(), true
	case instructions.PHA:
		w.Val, w.HasVal = e.A(), true
	case instructions.PHP:
		w.Val, w.HasVal = e.VCS.CPU.Status.Value(), true
	}
	return w, true
}

// LastMemWrite は直前の命令がメモリへ**書いた**なら、その実効アドレスを返す。
//
// 静的解析が「ここに書きうる」と主張した集合を、実際の実行が破っていないかを
// 突き合わせるための観測点。静的側の may-set は実測の書込を**包含**していなければ
// 健全でない（過大近似は許されるが、取りこぼしは誤り）。基底オペランドではなく
// 実効アドレスであることが肝で、`sta $80,x` は $80 ではなくその時の X 次第の番地を
// 書く（[[LastTIAWrite]] が同じ理由で壊れていた）。
//
// RMW（INC/DEC/シフトのメモリ形）も書込として返す。
func (e *Emu) LastMemWrite() (addr uint16, ok bool) {
	lr := e.VCS.CPU.LastResult
	d := lr.Defn
	if d == nil {
		return 0, false
	}
	writes := d.Effect == instructions.Write
	if !writes {
		switch d.Operator {
		case instructions.INC, instructions.DEC, instructions.ASL, instructions.LSR,
			instructions.ROL, instructions.ROR:
			writes = d.AddressingMode != instructions.Implied
		}
	}
	if !writes {
		return 0, false
	}
	return e.effectiveAddr()
}

// ObjectX は object の現在の水平位置（HmovedPixel・可視 0..159）を返す。obj は
// "P0"/"P1"/"M0"/"M1"/"BL"。HMOVE 適用後の実描画位置（HMOVE 未使用なら ResetPixel と同値）。
func (e *Emu) ObjectX(obj string) (int, bool) {
	v := e.VCS.TIA.Video
	switch obj {
	case "P0":
		return v.Player0.HmovedPixel, true
	case "P1":
		return v.Player1.HmovedPixel, true
	case "M0":
		return v.Missile0.HmovedPixel, true
	case "M1":
		return v.Missile1.HmovedPixel, true
	case "BL":
		return v.Ball.HmovedPixel, true
	}
	return 0, false
}

// PeekROM は副作用なしの 1 バイト読み（命令ストリームのデコード用）。
func (e *Emu) PeekROM(addr uint16) uint8 {
	v, err := e.VCS.Mem.Peek(addr)
	if err != nil {
		return 0
	}
	return v
}
