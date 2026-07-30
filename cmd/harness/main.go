// Command harness は Gopher2600 ベースの Atari 2600 検証ハーネスを MCP (stdio) で
// 露出する。Claude が load_rom → step → read で「やったこと＝結果」を数値で観測する。
// 仕様は docs/mcp-tools.md（全 API 裏取り済み）。数値ファースト＝画像は Phase 2.3。
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kidsnz/atari2600-harness/internal/beamrace"
	"github.com/kidsnz/atari2600-harness/internal/beamtrace"
	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/cyclebound"
	"github.com/kidsnz/atari2600-harness/internal/emu"
	"github.com/kidsnz/atari2600-harness/internal/ingest"
	"github.com/kidsnz/atari2600-harness/internal/motion"
	"github.com/kidsnz/atari2600-harness/internal/scenario"
	"github.com/kidsnz/atari2600-harness/internal/spritepos"
	"github.com/kidsnz/atari2600-harness/internal/srcmap"
	"github.com/kidsnz/atari2600-harness/internal/version"
	"github.com/kidsnz/atari2600-harness/pkg/audio"
)

// --- グローバル状態（stdio は逐次だが念のため mutex 保護）---

// curMap は assemble_and_load 経由ロード時の PC→ソース行対応（load_rom ではクリア）。
var curMap *srcmap.Map

// curROMPath は現在ロード中の ROM ファイル（patch オプションの復元先）。
var curROMPath string

// locate は PC をソース位置文字列へ（対応なしは空文字）。
func locate(pc uint16) string { return curMap.Locate(pc) }

var (
	mu      sync.Mutex
	current *emu.Emu
)

// get はロード済みの Emu を返す。未ロードならエラー。
func get() (*emu.Emu, error) {
	if current == nil {
		return nil, fmt.Errorf("no ROM loaded: call load_rom first")
	}
	return current, nil
}

// --- 共通戻り値 ---

type Coords struct {
	Frame    int `json:"frame"`
	Scanline int `json:"scanline"`
	Clock    int `json:"clock"` // Gopher2600規約: HBLANK −68..−1 / 可視 0..159（可視px0=clock0）
}

func coordsOf(e *emu.Emu) Coords {
	c := e.Coords()
	return Coords{Frame: c.Frame, Scanline: c.Scanline, Clock: c.Clock}
}

// --- load_rom ---

type LoadROMIn struct {
	Path   string `json:"path" jsonschema:"path to .bin ROM"`
	TVSpec string `json:"tv_spec,omitempty" jsonschema:"NTSC|PAL|AUTO (default NTSC)"`
}
type LoadROMOut struct {
	Coords  Coords `json:"coords"`
	Message string `json:"message"`
}

func handleLoadROM(ctx context.Context, req *mcp.CallToolRequest, in LoadROMIn) (*mcp.CallToolResult, LoadROMOut, error) {
	mu.Lock()
	defer mu.Unlock()

	spec := in.TVSpec
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, LoadROMOut{}, fmt.Errorf("new emu: %w", err)
	}
	if err := e.LoadROM(in.Path); err != nil {
		return nil, LoadROMOut{}, fmt.Errorf("load rom: %w", err)
	}
	current = e
	resetSlots() // 別 ROM の状態を復元させない
	curMap = nil // .bin 直ロードはソース対応なし
	curROMPath = in.Path
	return nil, LoadROMOut{
		Coords:  coordsOf(e),
		Message: fmt.Sprintf("loaded %s (%s)", in.Path, spec),
	}, nil
}

// --- assemble_and_load（P3: edit→dasm→load を 1 ショット化）---

type AssembleIn struct {
	AsmPath string `json:"asm_path" jsonschema:"path to .asm source"`
	BinPath string `json:"bin_path,omitempty" jsonschema:"output .bin path (default: asm path with .bin extension)"`
	TVSpec  string `json:"tv_spec,omitempty" jsonschema:"NTSC|PAL|AUTO (default NTSC)"`
}
type AssembleOut struct {
	Ok         bool   `json:"ok"`          // dasm 成功
	BinPath    string `json:"bin_path"`    // 出力 .bin
	DasmOutput string `json:"dasm_output"` // dasm の stdout+stderr（失敗時は失敗行を含む）
	Loaded     bool   `json:"loaded"`      // 成功して VCS にロードしたか
	Coords     Coords `json:"coords"`      // ロード時のみ有効
}

func handleAssembleAndLoad(ctx context.Context, req *mcp.CallToolRequest, in AssembleIn) (*mcp.CallToolResult, AssembleOut, error) {
	mu.Lock()
	defer mu.Unlock()

	if in.AsmPath == "" {
		return nil, AssembleOut{}, fmt.Errorf("asm_path is required")
	}
	bin := in.BinPath
	if bin == "" {
		bin = build.BinPathFor(in.AsmPath)
	}

	// dasm -f3（-l/-s 付き）。失敗行を含む診断をそのまま返す。
	out, lst, sym, err := build.AssembleWithListing(in.AsmPath, bin)
	if err != nil {
		// アセンブル失敗は MCP エラーにせず Ok=false＋dasm 出力で構造化返却（Claude が失敗行を見て直す）。
		return nil, AssembleOut{Ok: false, BinPath: bin, DasmOutput: out}, nil
	}
	curMap = srcmap.Parse(lst, sym, in.AsmPath) // U-M9: 以後のツール出力に at を併記

	spec := in.TVSpec
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, AssembleOut{}, fmt.Errorf("new emu: %w", err)
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, AssembleOut{Ok: true, BinPath: bin, DasmOutput: out}, fmt.Errorf("assembled ok but load failed: %w", err)
	}
	current = e
	resetSlots() // 別 ROM の状態を復元させない
	curROMPath = bin
	return nil, AssembleOut{Ok: true, BinPath: bin, DasmOutput: out, Loaded: true, Coords: coordsOf(e)}, nil
}

// --- step_frame ---

type StepFrameIn struct {
	Count int `json:"count,omitempty" jsonschema:"frames to run (default 1)"`
}
type StepFrameOut struct {
	Coords Coords `json:"coords"`
}

func handleStepFrame(ctx context.Context, req *mcp.CallToolRequest, in StepFrameIn) (*mcp.CallToolResult, StepFrameOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, StepFrameOut{}, err
	}
	n := in.Count
	if n <= 0 {
		n = 1
	}
	if err := e.RunFrames(n); err != nil {
		return nil, StepFrameOut{}, fmt.Errorf("run frames: %w", err)
	}
	return nil, StepFrameOut{Coords: coordsOf(e)}, nil
}

// --- step_instruction / step_scanline（B-2: フレーム内粒度）---

type StepInstructionOut struct {
	LastInstructionCycles int    `json:"last_instruction_cycles"`
	Coords                Coords `json:"coords"`
}

func handleStepInstruction(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, StepInstructionOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, StepInstructionOut{}, err
	}
	if err := e.StepInstruction(); err != nil {
		return nil, StepInstructionOut{}, fmt.Errorf("step instruction: %w", err)
	}
	return nil, StepInstructionOut{LastInstructionCycles: e.LastCycles(), Coords: coordsOf(e)}, nil
}

type StepScanlineOut struct {
	CyclesConsumed int64  `json:"cycles_consumed"` // この scanline 区間で実行した CPU サイクル
	Coords         Coords `json:"coords"`
}

func handleStepScanline(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, StepScanlineOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, StepScanlineOut{}, err
	}
	before := e.TotalCycles()
	if err := e.StepScanline(); err != nil {
		return nil, StepScanlineOut{}, fmt.Errorf("step scanline: %w", err)
	}
	return nil, StepScanlineOut{CyclesConsumed: e.TotalCycles() - before, Coords: coordsOf(e)}, nil
}

// --- read_cpu ---

type CPUFlags struct {
	N bool `json:"n"`
	V bool `json:"v"`
	B bool `json:"b"`
	D bool `json:"d"`
	I bool `json:"i"`
	Z bool `json:"z"`
	C bool `json:"c"`
}

// --- read_bank（bankswitch ROM の現在バンク。4K 非バンクでは常に 0/false）---

type ReadBankOut struct {
	Bank   int    `json:"bank" jsonschema:"current cartridge bank at PC"`
	IsRAM  bool   `json:"is_ram,omitempty" jsonschema:"true when PC is executing from cartridge RAM"`
	Coords Coords `json:"coords"`
}

func handleReadBank(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadBankOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadBankOut{}, err
	}
	n, isRAM := e.Bank()
	return nil, ReadBankOut{Bank: n, IsRAM: isRAM, Coords: coordsOf(e)}, nil
}

type ReadCPUOut struct {
	PC     uint16   `json:"pc"`
	At     string   `json:"at,omitempty"` // ソース位置（assemble_and_load 経由時のみ）
	A      uint8    `json:"a"`
	X      uint8    `json:"x"`
	Y      uint8    `json:"y"`
	SP     uint8    `json:"sp"`
	Status uint8    `json:"status"`
	Flags  CPUFlags `json:"flags"`
	Coords Coords   `json:"coords"`
}

func handleReadCPU(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadCPUOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadCPUOut{}, err
	}
	cpu := e.VCS.CPU
	sr := cpu.Status
	return nil, ReadCPUOut{
		PC:     cpu.PC.Value(),
		At:     locate(cpu.PC.Value()),
		A:      cpu.A.Value(),
		X:      cpu.X.Value(),
		Y:      cpu.Y.Value(),
		SP:     uint8(cpu.SP.Address()),
		Status: sr.Value(),
		Flags: CPUFlags{
			N: sr.Sign, V: sr.Overflow, B: sr.Break, D: sr.DecimalMode,
			I: sr.InterruptDisable, Z: sr.Zero, C: sr.Carry,
		},
		Coords: coordsOf(e),
	}, nil
}

// --- read_cycles（鉄則2: サイクルはシミュレータから取る）---

type ReadCyclesIn struct {
	Reset bool `json:"reset,omitempty" jsonschema:"mark a new measurement baseline before reading (cycles_since_mark resets to 0)"`
}
type ReadCyclesOut struct {
	LastInstructionCycles int    `json:"last_instruction_cycles"` // 直近 1 命令のサイクル数
	CyclesSinceMark       int64  `json:"cycles_since_mark"`       // 直近 mark 以降の累積
	TotalCycles           int64  `json:"total_cycles"`            // ROM ロード以降の累積
	Coords                Coords `json:"coords"`
}

func handleReadCycles(ctx context.Context, req *mcp.CallToolRequest, in ReadCyclesIn) (*mcp.CallToolResult, ReadCyclesOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadCyclesOut{}, err
	}
	if in.Reset {
		e.MarkCycles()
	}
	return nil, ReadCyclesOut{
		LastInstructionCycles: e.LastCycles(),
		CyclesSinceMark:       e.CyclesSinceMark(),
		TotalCycles:           e.TotalCycles(),
		Coords:                coordsOf(e),
	}, nil
}

// --- read_ram ---

type ReadRAMOut struct {
	Base   uint16 `json:"base"` // 0x80
	Hex    string `json:"hex"`  // 256 hex chars, $80..$FF
	Coords Coords `json:"coords"`
}

func handleReadRAM(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadRAMOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadRAMOut{}, err
	}
	var sb strings.Builder
	for addr := 0x80; addr <= 0xFF; addr++ {
		b, err := e.PeekRAM(uint16(addr))
		if err != nil {
			return nil, ReadRAMOut{}, fmt.Errorf("peek %02X: %w", addr, err)
		}
		fmt.Fprintf(&sb, "%02x", b)
	}
	return nil, ReadRAMOut{Base: 0x80, Hex: sb.String(), Coords: coordsOf(e)}, nil
}

// --- read_tia (litmus test の中核) ---

type Sprite struct {
	ResetPixel  int `json:"reset_pixel"`
	HmovedPixel int `json:"hmoved_pixel"`
}
type ReadTIAOut struct {
	Player0  Sprite `json:"player0"`
	Player1  Sprite `json:"player1"`
	Missile0 Sprite `json:"missile0"`
	Missile1 Sprite `json:"missile1"`
	Ball     Sprite `json:"ball"`
	Hblank   bool   `json:"hblank"`
	Coords   Coords `json:"coords"`
}

func handleReadTIA(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadTIAOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadTIAOut{}, err
	}
	v := e.VCS.TIA.Video
	return nil, ReadTIAOut{
		Player0:  Sprite{v.Player0.ResetPixel, v.Player0.HmovedPixel},
		Player1:  Sprite{v.Player1.ResetPixel, v.Player1.HmovedPixel},
		Missile0: Sprite{v.Missile0.ResetPixel, v.Missile0.HmovedPixel},
		Missile1: Sprite{v.Missile1.ResetPixel, v.Missile1.HmovedPixel},
		Ball:     Sprite{v.Ball.ResetPixel, v.Ball.HmovedPixel},
		Hblank:   e.VCS.TIA.Hblank,
		Coords:   coordsOf(e),
	}, nil
}

// --- read_tia_registers（P1: 書込専用レジスタの現在値を実測）---

type ReadTIARegsOut struct {
	emu.TIARegisters
	Coords Coords `json:"coords"`
}

func handleReadTIARegisters(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadTIARegsOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadTIARegsOut{}, err
	}
	return nil, ReadTIARegsOut{TIARegisters: e.ReadTIARegisters(), Coords: coordsOf(e)}, nil
}

// --- read_audio（R-2: 音声レジスタを数値で）---

type ChannelNote struct {
	Note  string  `json:"note,omitempty"`  // 12 平均律の最近接音名（無音/非楽音は空）
	Cents float64 `json:"cents,omitempty"` // その音名からの誤差
}
type ReadAudioOut struct {
	emu.AudioState
	Note0  ChannelNote `json:"note0"` // ch0 の音名（A-1 回収: 耳でなく名前で議論できる）
	Note1  ChannelNote `json:"note1"`
	Coords Coords      `json:"coords"`
}

func noteOf(ch emu.AudioChannel) ChannelNote {
	if ch.Volume == 0 {
		return ChannelNote{}
	}
	f := audio.Freq(int(ch.Control), int(ch.Freq), audio.BaseClockNTSC)
	name, cents := audio.NearestNote(f)
	return ChannelNote{Note: name, Cents: cents}
}

func handleReadAudio(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadAudioOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadAudioOut{}, err
	}
	st := e.ReadAudio()
	return nil, ReadAudioOut{AudioState: st, Note0: noteOf(st.Channel0), Note1: noteOf(st.Channel1), Coords: coordsOf(e)}, nil
}

// --- read_audio_trace（音の時系列＝read_audio の read_motion 版。音の包絡を一括で採る）---

type AudioTraceIn struct {
	Frames int `json:"frames,omitempty" jsonschema:"frames to trace (default 40); ADVANCES the emulator this many frames"`
}
type AudioChannelTrace struct {
	// ★[]int（[]uint8 は Go が JSON で base64 文字列にエンコード＝配列にならず schema 検証に落ちる）
	Control []int `json:"control"` // AUDC per frame（波形/音色）
	Freq    []int `json:"freq"`    // AUDF per frame（分周＝音程）
	Volume  []int `json:"volume"`  // AUDV per frame（音量＝包絡）
}
type AudioTraceOut struct {
	Frames   int               `json:"frames"`
	Channel0 AudioChannelTrace `json:"channel0"`
	Channel1 AudioChannelTrace `json:"channel1"`
	Coords   Coords            `json:"coords"`
}

func handleReadAudioTrace(ctx context.Context, req *mcp.CallToolRequest, in AudioTraceIn) (*mcp.CallToolResult, AudioTraceOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, AudioTraceOut{}, err
	}
	frames := in.Frames
	if frames == 0 {
		frames = 40
	}
	var c0, c1 AudioChannelTrace
	for f := 0; f < frames; f++ {
		if _, err := e.StepFrame(); err != nil {
			return nil, AudioTraceOut{}, err
		}
		st := e.ReadAudio()
		c0.Control = append(c0.Control, int(st.Channel0.Control))
		c0.Freq = append(c0.Freq, int(st.Channel0.Freq))
		c0.Volume = append(c0.Volume, int(st.Channel0.Volume))
		c1.Control = append(c1.Control, int(st.Channel1.Control))
		c1.Freq = append(c1.Freq, int(st.Channel1.Freq))
		c1.Volume = append(c1.Volume, int(st.Channel1.Volume))
	}
	return nil, AudioTraceOut{Frames: frames, Channel0: c0, Channel1: c1, Coords: coordsOf(e)}, nil
}

// --- read_ram_trace（任意 RAM の時系列＝step+peek ループを1発に。AI位置/タイマ/モード等の推移を数値で採る）---

type RamTraceIn struct {
	Addrs  []int `json:"addrs" jsonschema:"RAM addresses to trace, each $80-$FF (128-255); 1-16 addresses"`
	Frames int   `json:"frames,omitempty" jsonschema:"frames to trace (default 60); ADVANCES the emulator this many frames"`
}
type RamTraceOut struct {
	// ★[]int（[]uint8 は Go が JSON で base64 文字列にエンコード＝配列にならず schema 検証に落ちる）
	Frames int     `json:"frames"`
	Addrs  []int   `json:"addrs"`  // echoed, same order as traces
	Traces [][]int `json:"traces"` // traces[i][f] = value of Addrs[i] at frame f (0-based from the call)
	Coords Coords  `json:"coords"`
}

func handleReadRAMTrace(ctx context.Context, req *mcp.CallToolRequest, in RamTraceIn) (*mcp.CallToolResult, RamTraceOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, RamTraceOut{}, err
	}
	if len(in.Addrs) == 0 {
		return nil, RamTraceOut{}, fmt.Errorf("addrs required (1-16 RAM addresses $80-$FF)")
	}
	if len(in.Addrs) > 16 {
		return nil, RamTraceOut{}, fmt.Errorf("too many addrs (%d); max 16", len(in.Addrs))
	}
	for _, a := range in.Addrs {
		if a < 0x80 || a > 0xFF {
			return nil, RamTraceOut{}, fmt.Errorf("addr %d out of RAM range $80-$FF (128-255)", a)
		}
	}
	frames := in.Frames
	if frames == 0 {
		frames = 60
	}
	if frames < 1 || frames > 4000 {
		return nil, RamTraceOut{}, fmt.Errorf("frames %d out of range (1-4000)", frames)
	}
	traces := make([][]int, len(in.Addrs))
	for i := range traces {
		traces[i] = make([]int, 0, frames)
	}
	for f := 0; f < frames; f++ {
		if _, err := e.StepFrame(); err != nil {
			return nil, RamTraceOut{}, err
		}
		for i, a := range in.Addrs {
			b, err := e.PeekRAM(uint16(a))
			if err != nil {
				return nil, RamTraceOut{}, fmt.Errorf("peek %02X: %w", a, err)
			}
			traces[i] = append(traces[i], int(b))
		}
	}
	return nil, RamTraceOut{Frames: frames, Addrs: in.Addrs, Traces: traces, Coords: coordsOf(e)}, nil
}

// --- read_collisions（P1: CXxx を構造化）---

type ReadCollisionsOut struct {
	emu.Collisions
	Coords Coords `json:"coords"`
}

func handleReadCollisions(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ReadCollisionsOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadCollisionsOut{}, err
	}
	cx, err := e.ReadCollisions()
	if err != nil {
		return nil, ReadCollisionsOut{}, err
	}
	return nil, ReadCollisionsOut{Collisions: cx, Coords: coordsOf(e)}, nil
}

// --- read_row（playfield 点灯列 / per-scanline 色を数値で読む）---

type ReadRowIn struct {
	Scanline int `json:"scanline" jsonschema:"visible scanline (0-based, same y as the annotated grid)"`
}
type ReadRowOut struct {
	Scanline int          `json:"scanline"`
	Width    int          `json:"width"` // 可視幅（通常 160）
	Runs     []emu.RowRun `json:"runs"`  // 横方向の連長エンコード {clock,len,hex}
	Coords   Coords       `json:"coords"`
}

func handleReadRow(ctx context.Context, req *mcp.CallToolRequest, in ReadRowIn) (*mcp.CallToolResult, ReadRowOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadRowOut{}, err
	}
	runs, width, err := e.ReadRow(in.Scanline)
	if err != nil {
		return nil, ReadRowOut{}, err
	}
	return nil, ReadRowOut{
		Scanline: in.Scanline,
		Width:    width,
		Runs:     runs,
		Coords:   coordsOf(e),
	}, nil
}

// --- decompose_row（各ピクセルを "どの TIA オブジェクトが描いたか" で分解。AT-5）---

type DecomposeRowIn struct {
	Scanline int `json:"scanline" jsonschema:"visible scanline (0-based, same y as read_row / the annotated grid)"`
}
type DecomposeRowOut struct {
	Scanline int           `json:"scanline"`
	Width    int           `json:"width"` // 可視幅（通常 160）
	Runs     []emu.ElemRun `json:"runs"`  // 連長エンコード {clock,len,element}: BG/PF/P0/P1/M0/M1/BL
	Coords   Coords        `json:"coords"`
}

func handleDecomposeRow(ctx context.Context, req *mcp.CallToolRequest, in DecomposeRowIn) (*mcp.CallToolResult, DecomposeRowOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, DecomposeRowOut{}, err
	}
	runs, width, err := e.DecomposeRow(in.Scanline)
	if err != nil {
		return nil, DecomposeRowOut{}, err
	}
	return nil, DecomposeRowOut{
		Scanline: in.Scanline,
		Width:    width,
		Runs:     runs,
		Coords:   coordsOf(e),
	}, nil
}

// --- read_motion（オブジェクトの動きの滑らかさ＝judder/ブルブルを数値化。VV-4）---

type ReadMotionIn struct {
	Object string `json:"object,omitempty" jsonschema:"object to track: P0 M0 P1 M1 BL (default BL)"`
	Frames int    `json:"frames,omitempty" jsonschema:"frames to track (default 40); ADVANCES the emulator this many frames"`
	YTop   int    `json:"y_top,omitempty" jsonschema:"scanline search window top (grid-y; default 0)"`
	YBot   int    `json:"y_bot,omitempty" jsonschema:"scanline search window bottom (grid-y; default 260)"`
}
type ReadMotionOut struct {
	Track  motion.Track `json:"track"`
	Coords Coords       `json:"coords"`
}

func handleReadMotion(ctx context.Context, req *mcp.CallToolRequest, in ReadMotionIn) (*mcp.CallToolResult, ReadMotionOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ReadMotionOut{}, err
	}
	obj := in.Object
	if obj == "" {
		obj = "BL"
	}
	frames := in.Frames
	if frames == 0 {
		frames = 40
	}
	yBot := in.YBot
	if yBot == 0 {
		yBot = 260
	}
	tr, err := motion.TrackObject(e, obj, frames, in.YTop, yBot)
	if err != nil {
		return nil, ReadMotionOut{}, err
	}
	return nil, ReadMotionOut{Track: tr, Coords: coordsOf(e)}, nil
}

// --- spritey（オブジェクトの縦位置 Y を数値で。read_tia=Xのみ・read_motion top=小missile不可 の穴埋め。弾の軌道/リコシェを数値化）---

type SpriteYIn struct {
	Object string `json:"object,omitempty" jsonschema:"object: P0 M0 P1 M1 BL (default M0)"`
	Frames int    `json:"frames,omitempty" jsonschema:"samples to return (default 1 = current frame only, no advance). >1 ADVANCES the emulator frames-1 steps and returns the per-frame Y trajectory — trace a bullet's ricochet (y_top rises then falls at a bounce) as numbers"`
}
type SpriteYSample struct {
	Frame   int  `json:"frame"`
	X       int  `json:"x"`       // HmovedPixel (0..159)
	YTop    int  `json:"y_top"`   // topmost drawn scanline (grid-y); -1 if absent
	YBot    int  `json:"y_bot"`   // bottommost drawn scanline; -1 if absent
	Height  int  `json:"height"`  // y_bot - y_top + 1
	Present bool `json:"present"` // false = disabled / off-screen
}
type SpriteYOut struct {
	Object  string          `json:"object"`
	Samples []SpriteYSample `json:"samples"`
	Coords  Coords          `json:"coords"`
}

var spriteYIndex = map[string]int{"P0": 0, "M0": 1, "P1": 2, "M1": 3, "BL": 4}

func handleSpriteY(ctx context.Context, req *mcp.CallToolRequest, in SpriteYIn) (*mcp.CallToolResult, SpriteYOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, SpriteYOut{}, err
	}
	obj := strings.ToUpper(strings.TrimSpace(in.Object))
	if obj == "" {
		obj = "M0"
	}
	idx, ok := spriteYIndex[obj]
	if !ok {
		return nil, SpriteYOut{}, fmt.Errorf("unknown object %q (want P0 M0 P1 M1 BL)", obj)
	}
	frames := in.Frames
	if frames < 1 {
		frames = 1
	}
	out := SpriteYOut{Object: obj}
	for f := 0; f < frames; f++ {
		if f > 0 {
			if _, err := e.StepFrame(); err != nil {
				return nil, SpriteYOut{}, err
			}
		}
		yt, yb, h, present := e.ObjectYExtent(idx)
		s := SpriteYSample{Frame: f, X: e.Markers()[idx].Clock, Present: present}
		if present {
			s.YTop, s.YBot, s.Height = yt, yb, h
		} else {
			s.YTop, s.YBot, s.Height = -1, -1, 0
		}
		out.Samples = append(out.Samples, s)
	}
	out.Coords = coordsOf(e)
	return nil, out, nil
}

// --- set_input（ジョイスティック注入。poke は入力に効かない）---

type SetInputIn struct {
	Player  int     `json:"player,omitempty" jsonschema:"player port (0 left/P0 default, 1 right/P1)"`
	Action  string  `json:"action" jsonschema:"joystick: left|right|up|down|fire|center|paddle; console panel switches: reset|select|color|p0pro|p1pro"`
	Pressed bool    `json:"pressed,omitempty" jsonschema:"press/hold when set, release when unset (ignored for center/paddle)"`
	Value   float64 `json:"value,omitempty" jsonschema:"paddle position 0.0..1.0 (action=paddle only; plugs the paddle peripheral on first use)"`
}
type SetInputOut struct {
	Coords Coords `json:"coords"`
}

func handleSetInput(ctx context.Context, req *mcp.CallToolRequest, in SetInputIn) (*mcp.CallToolResult, SetInputOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, SetInputOut{}, err
	}
	switch in.Action {
	case "paddle":
		if err := e.SetPaddle(in.Player, in.Value); err != nil {
			return nil, SetInputOut{}, err
		}
	case "reset", "select", "color", "p0pro", "p1pro": // コンソールパネルのスイッチ
		if err := e.SetPanel(in.Action, in.Pressed); err != nil {
			return nil, SetInputOut{}, err
		}
	default:
		if err := e.SetInput(in.Player, in.Action, in.Pressed); err != nil {
			return nil, SetInputOut{}, err
		}
	}
	return nil, SetInputOut{Coords: coordsOf(e)}, nil
}

// --- peek / poke ---

type PeekIn struct {
	Addr uint16 `json:"addr" jsonschema:"memory address"`
}
type PeekOut struct {
	Value  uint8  `json:"value"`
	Coords Coords `json:"coords"`
}

func handlePeek(ctx context.Context, req *mcp.CallToolRequest, in PeekIn) (*mcp.CallToolResult, PeekOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, PeekOut{}, err
	}
	val, err := e.PeekRAM(in.Addr)
	if err != nil {
		return nil, PeekOut{}, fmt.Errorf("peek: %w", err)
	}
	return nil, PeekOut{Value: val, Coords: coordsOf(e)}, nil
}

type PokeIn struct {
	Addr  uint16 `json:"addr" jsonschema:"memory address"`
	Value uint8  `json:"value" jsonschema:"byte to write"`
}
type PokeOut struct {
	Coords Coords `json:"coords"`
}

func handlePoke(ctx context.Context, req *mcp.CallToolRequest, in PokeIn) (*mcp.CallToolResult, PokeOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, PokeOut{}, err
	}
	if err := e.Poke(in.Addr, in.Value); err != nil {
		return nil, PokeOut{}, fmt.Errorf("poke: %w", err)
	}
	return nil, PokeOut{Coords: coordsOf(e)}, nil
}

// --- breakif（ビーム位置で停止）---

type BreakIfIn struct {
	MaxFrames     int `json:"max_frames,omitempty" jsonschema:"upper bound on frames to run (default 1)"`
	UntilScanline int `json:"until_scanline" jsonschema:"halt when beam reaches this scanline"`
	UntilClock    int `json:"until_clock" jsonschema:"halt when beam reaches this color clock (0-227)"`
}
type BreakIfOut struct {
	Halted bool   `json:"halted"` // true=条件で停止 / false=フレーム上限に到達
	Coords Coords `json:"coords"`
}

func handleBreakIf(ctx context.Context, req *mcp.CallToolRequest, in BreakIfIn) (*mcp.CallToolResult, BreakIfOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, BreakIfOut{}, err
	}
	maxFrames := in.MaxFrames
	if maxFrames <= 0 {
		maxFrames = 1
	}
	halted, err := e.RunUntilBeam(maxFrames, in.UntilScanline, in.UntilClock)
	if err != nil {
		return nil, BreakIfOut{}, fmt.Errorf("run until beam: %w", err)
	}
	return nil, BreakIfOut{Halted: halted, Coords: coordsOf(e)}, nil
}

// --- assert_line_budget（B-3: per-scanline サイクル予算ガード）---

// PatchSpec は測定専用の一時 ROM パッチ（PONG-C2: XTable 差替儀式の構造的解消）。
// symbol は直近 assemble_and_load のシンボル表から解決。パッチは ROM のコピーに適用され、
// 測定後に元 ROM を再ロードして復元する（復元忘れという事故クラスを消す）。
type PatchSpec struct {
	Symbol string `json:"symbol,omitempty" jsonschema:"DASM symbol from the last assemble_and_load (e.g. XTable)"`
	Addr   int    `json:"addr,omitempty" jsonschema:"absolute ROM address (e.g. 61522 = $F052); used when symbol is empty"`
	Bytes  string `json:"bytes" jsonschema:"hex byte string to write at the location (e.g. 0e0e090e0e)"`
}

// PokeSpec は（再）起動直後・実行前に適用する RAM poke。
type PokeSpec struct {
	Addr  int `json:"addr" jsonschema:"RAM address"`
	Value int `json:"value" jsonschema:"byte value"`
}

type BudgetIn struct {
	MaxFrames int         `json:"max_frames,omitempty" jsonschema:"upper bound on frames to run (default 1)"`
	Budget    int         `json:"budget,omitempty" jsonschema:"CPU-cycle budget per WSYNC interval (default 76 = one scanline)"`
	Patch     []PatchSpec `json:"patch,omitempty" jsonschema:"temporary ROM byte patches applied to a COPY for this measurement only (fresh boot); the original ROM is reloaded afterwards"`
	Pokes     []PokeSpec  `json:"pokes,omitempty" jsonschema:"RAM pokes applied after boot, before running (only with patch)"`
}
type BudgetOut struct {
	Over       bool   `json:"over"`         // true=ある論理ラインが予算超過（ロール要因）で停止
	At         string `json:"at,omitempty"` // 停止時 PC のソース位置（assemble_and_load 経由時のみ）
	AtScanline int    `json:"at_scanline"`  // 超過した論理ラインの開始 scanline（over=true 時）
	LineCycles int    `json:"line_cycles"`  // そのラインが消費した概算 machine cycle（消費ライン数×76）
	Coords     Coords `json:"coords"`
}

// applyTempPatch は現 ROM のコピーへ patch を適用して差し替えロードし、復元関数を返す。
// 呼び出し側は必ず defer restore() すること（＝復元忘れの構造的防止が存在理由）。
func applyTempPatch(e emuLike, patches []PatchSpec) (restore func(), err error) {
	if curROMPath == "" {
		return nil, fmt.Errorf("patch: no ROM path tracked (load via load_rom/assemble_and_load first)")
	}
	rom, err := os.ReadFile(curROMPath)
	if err != nil {
		return nil, fmt.Errorf("patch: read rom: %w", err)
	}
	base := 0x10000 - len(rom) // 4K→$F000 / 2K→$F800
	for _, p := range patches {
		addr := p.Addr
		if p.Symbol != "" {
			a, ok := curMap.Symbol(p.Symbol)
			if !ok {
				return nil, fmt.Errorf("patch: symbol %q not found (assemble_and_load required for symbols)", p.Symbol)
			}
			addr = int(a)
		}
		b, err := hex.DecodeString(strings.TrimPrefix(p.Bytes, "$"))
		if err != nil {
			return nil, fmt.Errorf("patch: bytes %q: %w", p.Bytes, err)
		}
		off := addr - base
		if off < 0 || off+len(b) > len(rom) {
			return nil, fmt.Errorf("patch: addr $%04X (+%d bytes) outside ROM ($%04X-$%04X)", addr, len(b), base, base+len(rom)-1)
		}
		copy(rom[off:], b)
	}
	tmp, err := os.CreateTemp("", "harness-patch-*.bin")
	if err != nil {
		return nil, fmt.Errorf("patch: temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(rom); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("patch: write temp: %w", err)
	}
	tmp.Close()
	if err := e.LoadROM(tmpPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("patch: load patched rom: %w", err)
	}
	orig := curROMPath
	return func() {
		_ = e.LoadROM(orig) // 元 ROM を必ず復元（フレッシュブート状態になる）
		os.Remove(tmpPath)
	}, nil
}

// emuLike は applyTempPatch が必要とする最小 interface。
type emuLike interface{ LoadROM(path string) error }

func handleBudgetGuard(ctx context.Context, req *mcp.CallToolRequest, in BudgetIn) (*mcp.CallToolResult, BudgetOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, BudgetOut{}, err
	}
	maxFrames := in.MaxFrames
	if maxFrames <= 0 {
		maxFrames = 1
	}
	if len(in.Patch) > 0 { // PONG-C2: 一時パッチ（コピーに適用→測定→自動復元）
		restore, err := applyTempPatch(e, in.Patch)
		if err != nil {
			return nil, BudgetOut{}, err
		}
		defer restore()
		// フレッシュブートの Reset 初期化＋VSYNC 安定化を先に済ませてから pokes を適用する
		// （順序を誤ると Reset の RAM クリアと RunUntilBudget の warmup が pokes を食い潰す）。
		if err := e.RunFrames(2); err != nil {
			return nil, BudgetOut{}, fmt.Errorf("warmup: %w", err)
		}
		for _, pk := range in.Pokes {
			if err := e.Poke(uint16(pk.Addr), uint8(pk.Value)); err != nil {
				return nil, BudgetOut{}, fmt.Errorf("poke $%04X: %w", pk.Addr, err)
			}
		}
	}
	over, atScanline, lineCycles, err := e.RunUntilBudget(maxFrames, in.Budget)
	if err != nil {
		return nil, BudgetOut{}, fmt.Errorf("run until budget: %w", err)
	}
	out := BudgetOut{Over: over, AtScanline: atScanline, LineCycles: lineCycles, Coords: coordsOf(e)}
	if over {
		out.At = locate(e.PC())
	}
	return nil, out, nil
}

// --- profile_line_budget（PONG-C3: assert_line_budget の定量版＝行別ワースト実測）---

type ProfileLinesIn struct {
	MaxFrames int         `json:"max_frames,omitempty" jsonschema:"frames to profile (default 30)"`
	Watch     []string    `json:"watch,omitempty" jsonschema:"RAM locations to snapshot at the OPENING strobe of each line's worst interval — DASM symbols (assemble_and_load required) or addresses ($A8 / 0x84 / 132). These are the arg values the worst path read."`
	Top       int         `json:"top,omitempty" jsonschema:"return only the N worst lines (default all)"`
	Patch     []PatchSpec `json:"patch,omitempty" jsonschema:"temporary ROM byte patches applied to a COPY for this measurement only (fresh boot); the original ROM is reloaded afterwards"`
	Pokes     []PokeSpec  `json:"pokes,omitempty" jsonschema:"RAM pokes applied after boot, before profiling (only with patch)"`
}
type ProfileLine struct {
	At            string         `json:"at,omitempty"` // 開き STA WSYNC のソース位置（assemble_and_load 経由時）
	PC            string         `json:"pc"`           // 開き STA WSYNC の PC（hex）
	WorstCycles   int            `json:"worst_cycles"` // 実測ワースト CPU cy（≤76 なら1ラインに収まっている）
	WorstLines    int            `json:"worst_lines"`  // ワースト時の消費物理ライン数
	Count         int            `json:"count"`        // 計測区間数
	WorstFrame    int            `json:"worst_frame"`
	WorstScanline int            `json:"worst_scanline"`
	Watch         map[string]int `json:"watch,omitempty"` // ワースト区間開始時点の watch RAM 値
	// Bank は開き strobe を実行したバンク（バンク切替カートのみ）。ポインタなので
	// 平坦 ROM では欄自体が出ず、"バンク0" と "バンクの概念なし" が混ざらない。
	Bank *int `json:"bank,omitempty"`
}
type ProfileLinesOut struct {
	Lines  []ProfileLine `json:"lines"`
	Coords Coords        `json:"coords"`
	// CrossFrameDropped は座標式が成り立たないため集計しなかった区間数。0 でも出す：
	// 欄が無いことを「そんな区間は無かった」と読まれるのを防ぐ。
	CrossFrameDropped int `json:"cross_frame_dropped"`
}

// resolveRAMRef は "$A8" / "0x84" / "132" / DASMシンボル を RAM アドレスへ解決する。
func resolveRAMRef(s string) (uint16, error) {
	if a, ok := curMap.Symbol(s); ok {
		return a, nil
	}
	t := strings.TrimPrefix(strings.TrimPrefix(s, "$"), "0x")
	if v, err := strconv.ParseUint(t, 16, 16); err == nil && (strings.HasPrefix(s, "$") || strings.HasPrefix(s, "0x")) {
		return uint16(v), nil
	}
	if v, err := strconv.ParseUint(s, 10, 16); err == nil {
		return uint16(v), nil
	}
	return 0, fmt.Errorf("watch %q: not a known symbol (assemble_and_load required for symbols) nor an address", s)
}

func handleProfileLines(ctx context.Context, req *mcp.CallToolRequest, in ProfileLinesIn) (*mcp.CallToolResult, ProfileLinesOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ProfileLinesOut{}, err
	}
	maxFrames := in.MaxFrames
	if maxFrames <= 0 {
		maxFrames = 30
	}
	watchAddrs := make([]uint16, len(in.Watch))
	for i, w := range in.Watch {
		a, err := resolveRAMRef(w)
		if err != nil {
			return nil, ProfileLinesOut{}, err
		}
		watchAddrs[i] = a
	}
	if len(in.Patch) > 0 { // C2 と同じ一時パッチ規律（コピーに適用→測定→自動復元）
		restore, err := applyTempPatch(e, in.Patch)
		if err != nil {
			return nil, ProfileLinesOut{}, err
		}
		defer restore()
		if err := e.RunFrames(2); err != nil {
			return nil, ProfileLinesOut{}, fmt.Errorf("warmup: %w", err)
		}
		for _, pk := range in.Pokes {
			if err := e.Poke(uint16(pk.Addr), uint8(pk.Value)); err != nil {
				return nil, ProfileLinesOut{}, fmt.Errorf("poke $%04X: %w", pk.Addr, err)
			}
		}
	}
	rows, crossFrame, err := e.ProfileLineWorst(maxFrames, watchAddrs)
	if err != nil {
		return nil, ProfileLinesOut{}, fmt.Errorf("profile lines: %w", err)
	}
	if in.Top > 0 && len(rows) > in.Top {
		rows = rows[:in.Top]
	}
	out := ProfileLinesOut{Coords: coordsOf(e), Lines: make([]ProfileLine, len(rows)),
		// Intervals that straddle a frame boundary have no valid cycle count from the
		// beam coordinates, so they are not aggregated. Reporting how many were
		// dropped is the difference between a table that is complete and one that
		// merely looks it — and a bank-switch trampoline is usually placed at the end
		// of overscan, i.e. exactly in the dropped set.
		CrossFrameDropped: crossFrame}
	for i, r := range rows {
		pl := ProfileLine{
			At: locate(r.StrobePC), PC: fmt.Sprintf("$%04X", r.StrobePC),
			WorstCycles: r.WorstCycles, WorstLines: r.WorstLines, Count: r.Count,
			WorstFrame: r.WorstFrame, WorstScanline: r.WorstScanline,
		}
		if r.BankValid {
			pl.Bank = &r.Bank
		}
		if len(r.Watch) == len(in.Watch) && len(in.Watch) > 0 {
			pl.Watch = map[string]int{}
			for j, name := range in.Watch {
				pl.Watch[name] = int(r.Watch[j])
			}
		}
		out.Lines[i] = pl
	}
	return nil, out, nil
}

// --- assert_edge_coincidence（PONG-C1: Nエッジ同一Y整列の worst-path fuzz）---
// エッジ比較カーネル（cpy <edge> の束）の真の worst path＝「全エッジ変数が同じ Y に揃う」
// を能動的に作って予算をassertする。free-run では数百フレーム踏まないことがある
// 1cy 超過（known-traps "N-edge coincidence"）を数秒で網羅検出する。

type EdgeCoinIn struct {
	Addrs      []int       `json:"addrs" jsonschema:"zero-page RAM addresses of the edge variables to align (poked to Y+offset each case)"`
	Offsets    []int       `json:"offsets,omitempty" jsonschema:"per-addr offsets (same length as addrs, default all 0): addr[i] is poked to Y+offsets[i] — use to keep coupled variables consistent (e.g. ball_end=Y, ball_top=Y-4, paddle_end=Y+height) so the sweep only visits REACHABLE worst cases"`
	YMin       int         `json:"y_min,omitempty" jsonschema:"sweep start Y (default 0)"`
	YMax       int         `json:"y_max,omitempty" jsonschema:"sweep end Y inclusive (default 182)"`
	YStep      int         `json:"y_step,omitempty" jsonschema:"sweep step (default 1)"`
	FramesPerY int         `json:"frames_per_y,omitempty" jsonschema:"frames to run per alignment (default 2 = the poked frame renders once)"`
	Budget     int         `json:"budget,omitempty" jsonschema:"CPU-cycle budget per WSYNC interval (default 76)"`
	Patch      []PatchSpec `json:"patch,omitempty" jsonschema:"optional temporary ROM patches (e.g. lightweight positioning table) applied for the sweep, auto-restored"`
}
type EdgeCoinOut struct {
	Over       bool   `json:"over"`                  // true=少なくとも1つの整列Yで予算超過
	FailYs     []int  `json:"fail_ys,omitempty"`     // 超過した整列Y（最大32個まで記録）
	TestedYs   int    `json:"tested_ys"`             // 試行した整列Yの数
	FirstAt    string `json:"first_at,omitempty"`    // 最初の超過のソース位置
	FirstY     int    `json:"first_y,omitempty"`     // 最初の超過Y
	LineCycles int    `json:"line_cycles,omitempty"` // 最初の超過ラインの消費（消費ライン×76）
	Coords     Coords `json:"coords"`
}

func handleEdgeCoincidence(ctx context.Context, req *mcp.CallToolRequest, in EdgeCoinIn) (*mcp.CallToolResult, EdgeCoinOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, EdgeCoinOut{}, err
	}
	if len(in.Addrs) == 0 {
		return nil, EdgeCoinOut{}, fmt.Errorf("addrs is required (the edge variables to align)")
	}
	if len(in.Offsets) > 0 && len(in.Offsets) != len(in.Addrs) {
		return nil, EdgeCoinOut{}, fmt.Errorf("offsets length %d != addrs length %d", len(in.Offsets), len(in.Addrs))
	}
	yMax := in.YMax
	if yMax <= 0 {
		yMax = 182
	}
	yStep := in.YStep
	if yStep <= 0 {
		yStep = 1
	}
	framesPerY := in.FramesPerY
	if framesPerY <= 0 {
		framesPerY = 2
	}
	if len(in.Patch) > 0 {
		restore, err := applyTempPatch(e, in.Patch)
		if err != nil {
			return nil, EdgeCoinOut{}, err
		}
		defer restore()
	}
	// フレッシュブート安定化を掃引前に済ませる（各Yの poke を Reset/warmup に食わせない）
	if err := e.RunFrames(2); err != nil {
		return nil, EdgeCoinOut{}, fmt.Errorf("warmup: %w", err)
	}
	out := EdgeCoinOut{}
	for y := in.YMin; y <= yMax; y += yStep {
		for i, a := range in.Addrs {
			v := y
			if len(in.Offsets) > 0 {
				v += in.Offsets[i]
			}
			if err := e.Poke(uint16(a), uint8(v)); err != nil {
				return nil, EdgeCoinOut{}, fmt.Errorf("poke $%04X: %w", a, err)
			}
		}
		over, _, lineCycles, err := e.RunUntilBudget(framesPerY, in.Budget)
		if err != nil {
			return nil, EdgeCoinOut{}, fmt.Errorf("run (Y=%d): %w", y, err)
		}
		out.TestedYs++
		if over {
			if !out.Over {
				out.Over = true
				out.FirstY = y
				out.FirstAt = locate(e.PC())
				out.LineCycles = lineCycles
			}
			if len(out.FailYs) < 32 {
				out.FailYs = append(out.FailYs, y)
			}
		}
	}
	out.Coords = coordsOf(e)
	return nil, out, nil
}

// --- prove_line_budget（VV-2: 静的に全到達パスの per-scanline 予算を証明＝assert_line_budget の ∀ 版）---

type ProveLineBudgetIn struct {
	AsmPath string `json:"asm_path" jsonschema:"path to the kernel .asm to prove (relative to the harness working dir)"`
	Budget  int    `json:"budget,omitempty" jsonschema:"per-WSYNC-interval CPU-cycle budget (default 76 = one scanline)"`
}
type ProveLineBudgetOut struct {
	Report *cyclebound.Report `json:"report"`
}

func handleProveLineBudget(ctx context.Context, req *mcp.CallToolRequest, in ProveLineBudgetIn) (*mcp.CallToolResult, ProveLineBudgetOut, error) {
	// Static analysis on a source file; touches no emulator state, so no lock/get.
	if in.AsmPath == "" {
		return nil, ProveLineBudgetOut{}, fmt.Errorf("asm_path is required")
	}
	rep, err := cyclebound.Prove(in.AsmPath, in.Budget)
	if err != nil {
		return nil, ProveLineBudgetOut{}, err
	}
	return nil, ProveLineBudgetOut{Report: rep}, nil
}

// --- static def-use / beam-interval proofs (SD-1, SD-2) ---

type DefUseIn struct {
	AsmPath string `json:"asm_path" jsonschema:"path to the .asm to assemble and analyse"`
}
type DefUseOut struct {
	Report *cyclebound.DefUseReport `json:"report"`
}

// handleDefUse answers, over ALL paths, which instruction writes which address.
func handleDefUse(ctx context.Context, req *mcp.CallToolRequest, in DefUseIn) (*mcp.CallToolResult, DefUseOut, error) {
	if in.AsmPath == "" {
		return nil, DefUseOut{}, fmt.Errorf("asm_path is required")
	}
	rep, err := cyclebound.DefUse(in.AsmPath, 0)
	if err != nil {
		return nil, DefUseOut{}, err
	}
	return nil, DefUseOut{Report: rep}, nil
}

type BeamIntervalsIn struct {
	AsmPath string `json:"asm_path" jsonschema:"path to the .asm to assemble and analyse"`
}
type BeamIntervalsOut struct {
	Report *cyclebound.BeamReport `json:"report"`
}

// handleBeamIntervals proves, per TIA write, the beam window it can land in.
func handleBeamIntervals(ctx context.Context, req *mcp.CallToolRequest, in BeamIntervalsIn) (*mcp.CallToolResult, BeamIntervalsOut, error) {
	if in.AsmPath == "" {
		return nil, BeamIntervalsOut{}, fmt.Errorf("asm_path is required")
	}
	rep, err := cyclebound.BeamIntervals(in.AsmPath)
	if err != nil {
		return nil, BeamIntervalsOut{}, err
	}
	return nil, BeamIntervalsOut{Report: rep}, nil
}

// --- authoring aids: beamtrace timeline / beam_race advisory / spritepos solver ---

type BeamtraceIn struct {
	Scanline *int `json:"scanline,omitempty" jsonschema:"scanline to report (omit = every scanline that has writes)"`
	Frames   int  `json:"frames,omitempty" jsonschema:"frames to trace (default 1); ADVANCES the emulator"`
}
type BeamtraceOut struct {
	Frame  int             `json:"frame"`
	Rows   []beamtrace.Row `json:"rows"`
	Coords Coords          `json:"coords"`
}

// handleBeamtrace returns the write→visible-pixel timeline for the current ROM:
// per scanline, every TIA write with the beam clock it lands at and the visible
// span it governs (until the next write to the same register).
func handleBeamtrace(ctx context.Context, req *mcp.CallToolRequest, in BeamtraceIn) (*mcp.CallToolResult, BeamtraceOut, error) {
	mu.Lock()
	defer mu.Unlock()
	e, err := get()
	if err != nil {
		return nil, BeamtraceOut{}, err
	}
	frames := in.Frames
	if frames < 1 {
		frames = 1
	}
	ws, err := beamtrace.Trace(e, frames)
	if err != nil {
		return nil, BeamtraceOut{}, err
	}
	frs := beamtrace.Frames(ws)
	if len(frs) == 0 {
		return nil, BeamtraceOut{Coords: coordsOf(e)}, nil
	}
	frame := frs[0]
	var rows []beamtrace.Row
	if in.Scanline != nil {
		rows = append(rows, beamtrace.Timeline(ws, frame, *in.Scanline))
	} else {
		for _, sl := range beamtrace.Scanlines(ws, frame) {
			rows = append(rows, beamtrace.Timeline(ws, frame, sl))
		}
	}
	return nil, BeamtraceOut{Frame: frame, Rows: rows, Coords: coordsOf(e)}, nil
}

type BeamRaceIn struct {
	Frames int `json:"frames,omitempty" jsonschema:"frames to scan (default 1); ADVANCES the emulator"`
}
type BeamRaceOut struct {
	Events []beamrace.Event `json:"events"`
	Coords Coords           `json:"coords"`
}

// handleBeamRace returns the advisory beam-race map for the current ROM: per
// pixel-data write (GRP0/1, ENAM0/1, ENABL), the object's X and whether the write
// reached the beam in time. FACTUAL, no verdict — a "late" line may be an intended
// next-line pre-load; use a scenario no_beam_race check when you can state intent.
func handleBeamRace(ctx context.Context, req *mcp.CallToolRequest, in BeamRaceIn) (*mcp.CallToolResult, BeamRaceOut, error) {
	mu.Lock()
	defer mu.Unlock()
	e, err := get()
	if err != nil {
		return nil, BeamRaceOut{}, err
	}
	frames := in.Frames
	if frames < 1 {
		frames = 1
	}
	ev, err := beamrace.Trace(e, frames)
	if err != nil {
		return nil, BeamRaceOut{}, err
	}
	return nil, BeamRaceOut{Events: ev, Coords: coordsOf(e)}, nil
}

type SpriteposIn struct {
	X      int    `json:"x" jsonschema:"target X position 0..159"`
	Object string `json:"object,omitempty" jsonschema:"object: P0 P1 M0 M1 BL (default P0)"`
}
type SpriteposOut struct {
	Solution spritepos.Solution `json:"solution"`
	Snippet  string             `json:"snippet"`
}

// handleSpritepos solves a target X to the SetXPos routine input and verifies the
// achieved position on its own calibration emulator (independent of the loaded
// ROM), so the answer is measured, not trusted. Returns a paste-able snippet.
func handleSpritepos(ctx context.Context, req *mcp.CallToolRequest, in SpriteposIn) (*mcp.CallToolResult, SpriteposOut, error) {
	// Self-contained calculator: builds its own template ROM, does not touch the
	// shared emulator, so it takes no lock.
	obj := in.Object
	if obj == "" {
		obj = "P0"
	}
	p, err := spritepos.NewPositioner()
	if err != nil {
		return nil, SpriteposOut{}, err
	}
	sol, err := p.Solve(obj, in.X)
	if err != nil {
		return nil, SpriteposOut{}, err
	}
	return nil, SpriteposOut{Solution: sol, Snippet: spritepos.Snippet(obj, sol.InputA)}, nil
}

// --- get_screen_annotated（ユーザー↔Claude 通信回線）---

type ScreenIn struct {
	Scale int `json:"scale,omitempty" jsonschema:"integer upscale factor (default 3)"`
}
type SpritePos struct {
	Label string `json:"label"`
	Clock int    `json:"clock"` // HmovedPixel, 可視 0..159
}
type ScreenOut struct {
	Width   int         `json:"width"`
	Height  int         `json:"height"`
	Sprites []SpritePos `json:"sprites"` // 各オブジェクトの横位置（画像と同じ数値）
	Coords  Coords      `json:"coords"`
	PNGPath string      `json:"png_path"` // 人間が開ける固定パス（毎回上書き）
}

func handleScreenAnnotated(ctx context.Context, req *mcp.CallToolRequest, in ScreenIn) (*mcp.CallToolResult, ScreenOut, error) {
	mu.Lock()
	defer mu.Unlock()

	e, err := get()
	if err != nil {
		return nil, ScreenOut{}, err
	}
	scale := in.Scale
	if scale <= 0 {
		scale = 3
	}
	img := e.Annotated(scale)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, ScreenOut{}, fmt.Errorf("encode png: %w", err)
	}

	// 人間が開ける固定パスへ毎回上書き保存（ユーザー↔Claude 通信回線）。
	// MCP のインライン画像を描画しないクライアントでも、このファイルを開けば最新フレームが見られる。
	// VS Code の画像プレビューはファイル変更で自動リロード＝タブを開きっぱなしで往復可能。
	// 保存先は env ATARI2600_SCREEN_PATH で指定（未設定なら OS temp）。
	pngPath := os.Getenv("ATARI2600_SCREEN_PATH")
	if pngPath == "" {
		pngPath = filepath.Join(os.TempDir(), "atari2600_screen.png")
	}
	if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
		return nil, ScreenOut{}, fmt.Errorf("mkdir for png: %w", err)
	}
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		return nil, ScreenOut{}, fmt.Errorf("write png: %w", err)
	}

	sprites := make([]SpritePos, 0, 5)
	for _, m := range e.Markers() {
		sprites = append(sprites, SpritePos{Label: m.Label, Clock: m.Clock})
	}

	// 画像（人間向け）＋ 数値（Claude 向け structured Out）を両方返す。
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: buf.Bytes(), MIMEType: "image/png"},
		},
	}
	out := ScreenOut{
		Width:   img.Bounds().Dx(),
		Height:  img.Bounds().Dy(),
		Sprites: sprites,
		Coords:  coordsOf(e),
		PNGPath: pngPath,
	}
	return result, out, nil
}

// --- analyze_image: スクリーンショット → TIA データ（ingest パイプライン） ---

type AnalyzeImageIn struct {
	Path  string   `json:"path,omitempty"`  // 解析する PNG（A ランク = Stella F12 無加工）
	Paths []string `json:"paths,omitempty"` // 複数枚（同一シーンの連続 F12）→ マルチフレーム分離
	Scale int      `json:"scale,omitempty"` // overlay の拡大率（既定 3）
}

type AnalyzeImageOut struct {
	Report      *ingest.Report      `json:"report"`          // 単一フレーム or 静的層の解析
	Multi       *ingest.MultiReport `json:"multi,omitempty"` // 複数枚のとき: フレーム毎+union+flicker
	OverlayPath string              `json:"overlay_path"`    // グリッド付きオーバーレイの固定パス（毎回上書き）
}

func handleAnalyzeImage(ctx context.Context, req *mcp.CallToolRequest, in AnalyzeImageIn) (*mcp.CallToolResult, AnalyzeImageOut, error) {
	paths := in.Paths
	if len(paths) == 0 && in.Path != "" {
		paths = []string{in.Path}
	}
	if len(paths) == 0 {
		return nil, AnalyzeImageOut{}, fmt.Errorf("path or paths is required")
	}
	q := ingest.NewNTSCQuantizer()
	var frames []*ingest.Normalized
	for _, pth := range paths {
		f, err := os.Open(pth)
		if err != nil {
			return nil, AnalyzeImageOut{}, err
		}
		src, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, AnalyzeImageOut{}, fmt.Errorf("decode %s: %w", pth, err)
		}
		nn, err := ingest.Normalize(src, q)
		if err != nil {
			return nil, AnalyzeImageOut{}, err
		}
		frames = append(frames, nn)
	}
	n := frames[0]
	var rep *ingest.Report
	var multi *ingest.MultiReport
	if len(frames) > 1 {
		var err error
		multi, err = ingest.AnalyzeFrames(frames, q)
		if err != nil {
			return nil, AnalyzeImageOut{}, err
		}
		rep = multi.Static
	} else {
		rep = ingest.Analyze(n, q)
	}

	scale := in.Scale
	if scale <= 0 {
		scale = 3
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, ingest.OverlayReport(n, rep, scale)); err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	ovPath := os.Getenv("ATARI2600_INGEST_PATH")
	if ovPath == "" {
		ovPath = filepath.Join(os.TempDir(), "atari2600_ingest.png")
	}
	if err := os.MkdirAll(filepath.Dir(ovPath), 0o755); err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	if err := os.WriteFile(ovPath, buf.Bytes(), 0o644); err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: buf.Bytes(), MIMEType: "image/png"},
		},
	}
	return result, AnalyzeImageOut{Report: rep, Multi: multi, OverlayPath: ovPath}, nil
}

// --- run_scenario: 回帰シナリオを live ループから実行 ---

type RunScenarioIn struct {
	Paths []string `json:"paths"` // scenario JSON のパス（複数可）
}

type ScenarioResult struct {
	Path   string   `json:"path"`
	Pass   bool     `json:"pass"`
	Detail []string `json:"detail,omitempty"` // 失敗アサーションの説明
}

type RunScenarioOut struct {
	AllPass bool             `json:"all_pass"`
	Results []ScenarioResult `json:"results"`
}

func handleRunScenario(ctx context.Context, req *mcp.CallToolRequest, in RunScenarioIn) (*mcp.CallToolResult, RunScenarioOut, error) {
	if len(in.Paths) == 0 {
		return nil, RunScenarioOut{}, fmt.Errorf("paths is required")
	}
	out := RunScenarioOut{AllPass: true}
	for _, p := range in.Paths {
		sc, err := scenario.Load(p)
		if err != nil {
			return nil, RunScenarioOut{}, fmt.Errorf("%s: %w", p, err)
		}
		res, err := scenario.Run(sc, false)
		if err != nil {
			return nil, RunScenarioOut{}, fmt.Errorf("%s: %w", p, err)
		}
		r := ScenarioResult{Path: p, Pass: res.Pass}
		if !res.Pass {
			out.AllPass = false
			for _, a := range res.Asserts {
				if !a.Pass {
					r.Detail = append(r.Detail, fmt.Sprintf("%s (got %d)", a.Desc, a.Got))
				}
			}
		}
		out.Results = append(out.Results, r)
	}
	return nil, out, nil
}

// --- analyze_screen: 現在のエミュレータフレームに ingest を直接適用（ファイル不要） ---

type AnalyzeScreenIn struct {
	Scale int `json:"scale,omitempty"`
}

func handleAnalyzeScreen(ctx context.Context, req *mcp.CallToolRequest, in AnalyzeScreenIn) (*mcp.CallToolResult, AnalyzeImageOut, error) {
	mu.Lock()
	e, err := get()
	if err != nil {
		mu.Unlock()
		return nil, AnalyzeImageOut{}, err
	}
	snap, _ := e.Snapshot()
	mu.Unlock()
	q := ingest.NewNTSCQuantizer()
	n, err := ingest.Normalize(snap, q)
	if err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	rep := ingest.Analyze(n, q)
	scale := in.Scale
	if scale <= 0 {
		scale = 3
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, ingest.OverlayReport(n, rep, scale)); err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	ovPath := os.Getenv("ATARI2600_INGEST_PATH")
	if ovPath == "" {
		ovPath = filepath.Join(os.TempDir(), "atari2600_ingest.png")
	}
	if err := os.WriteFile(ovPath, buf.Bytes(), 0o644); err != nil {
		return nil, AnalyzeImageOut{}, err
	}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.ImageContent{Data: buf.Bytes(), MIMEType: "image/png"}},
	}
	return result, AnalyzeImageOut{Report: rep, OverlayPath: ovPath}, nil
}

// --- watch_ram: RAM 変化トラップ ---

type WatchRAMIn struct {
	Addr      int `json:"addr"`                 // 監視する RAM アドレス（$80-$FF）
	MaxFrames int `json:"max_frames,omitempty"` // 打ち切り（既定 60）
}

type WatchRAMOut struct {
	Changed bool   `json:"changed"`
	Old     int    `json:"old"`
	New     int    `json:"new"`
	PC      string `json:"pc,omitempty"` // 変化を起こした命令のアドレス
	At      string `json:"at,omitempty"` // 同・ソース位置（assemble_and_load 経由時のみ）
	Coords  Coords `json:"coords"`
}

func handleWatchRAM(ctx context.Context, req *mcp.CallToolRequest, in WatchRAMIn) (*mcp.CallToolResult, WatchRAMOut, error) {
	mu.Lock()
	defer mu.Unlock()
	e, err := get()
	if err != nil {
		return nil, WatchRAMOut{}, err
	}
	mf := in.MaxFrames
	if mf <= 0 {
		mf = 60
	}
	changed, oldV, newV, pc, err := e.WatchRAM(uint16(in.Addr), mf)
	if err != nil {
		return nil, WatchRAMOut{}, err
	}
	out := WatchRAMOut{Changed: changed, Old: int(oldV), New: int(newV), Coords: coordsOf(e)}
	if changed {
		out.PC = fmt.Sprintf("$%04X", pc)
		out.At = locate(pc)
	}
	return nil, out, nil
}

// --- trace_clocks: 命令毎のビーム解剖（step_clock の観測版） ---

type TraceClocksIn struct {
	MaxInstructions int `json:"max_instructions,omitempty"` // 既定 16
}

type TraceClocksOut struct {
	Trace  []emu.InstrTrace `json:"trace"`
	Coords Coords           `json:"coords"`
}

func handleTraceClocks(ctx context.Context, req *mcp.CallToolRequest, in TraceClocksIn) (*mcp.CallToolResult, TraceClocksOut, error) {
	mu.Lock()
	defer mu.Unlock()
	e, err := get()
	if err != nil {
		return nil, TraceClocksOut{}, err
	}
	n := in.MaxInstructions
	if n <= 0 {
		n = 16
	}
	if n > 200 {
		n = 200
	}
	tr, err := e.TraceClocks(n)
	for i := range tr {
		var pc uint16
		fmt.Sscanf(tr[i].PC, "$%04X", &pc)
		tr[i].At = locate(pc)
	}
	if err != nil {
		return nil, TraceClocksOut{}, err
	}
	return nil, TraceClocksOut{Trace: tr, Coords: coordsOf(e)}, nil
}

// --- main ---

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "atari2600-harness",
		Version: version.Harness,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "load_rom", Description: "Load a .bin ROM and reset the VCS (TV spec NTSC/PAL/AUTO)."}, handleLoadROM)
	mcp.AddTool(server, &mcp.Tool{Name: "assemble_and_load", Description: "Assemble a .asm with DASM (-f3) and, on success, load the resulting ROM in one shot. On failure returns ok=false with the DASM output (including the failing line) instead of an error — collapses the edit->dasm->load loop."}, handleAssembleAndLoad)
	mcp.AddTool(server, &mcp.Tool{Name: "step_frame", Description: "Run the emulator for N frames."}, handleStepFrame)
	mcp.AddTool(server, &mcp.Tool{Name: "step_instruction", Description: "Execute exactly one CPU instruction (consuming any pending WSYNC stall first). Returns its cycle count and beam coords — pairs with read_cycles to step through a kernel one instruction at a time."}, handleStepInstruction)
	mcp.AddTool(server, &mcp.Tool{Name: "step_scanline", Description: "Advance until the TV scanline increments once (stops at the next scanline, or scanline 0 of the next frame). Returns CPU cycles consumed across that scanline and beam coords — for inspecting kernel state line by line."}, handleStepScanline)
	mcp.AddTool(server, &mcp.Tool{Name: "read_cpu", Description: "Read 6507 registers, status flags, and beam coords."}, handleReadCPU)
	mcp.AddTool(server, &mcp.Tool{Name: "read_bank", Description: "Read the current cartridge bank at PC (bankswitched ROMs; 0 for flat 2K/4K). Returns is_ram=true when executing from cartridge RAM."}, handleReadBank)
	mcp.AddTool(server, &mcp.Tool{Name: "read_cycles", Description: "Read CPU cycle counts from the simulator (rule #2: never count cycles by hand): the last instruction's cycles, cycles since the last mark, and total cycles since ROM load. Set reset=true to mark a new measurement baseline (cycles_since_mark restarts at 0)."}, handleReadCycles)
	mcp.AddTool(server, &mcp.Tool{Name: "read_ram", Description: "Dump the 128 bytes of RAM ($80-$FF) as hex."}, handleReadRAM)
	mcp.AddTool(server, &mcp.Tool{Name: "read_tia", Description: "Read TIA sprite positions (ResetPixel/HmovedPixel) and HBLANK. Authoritative for horizontal-position checks."}, handleReadTIA)
	mcp.AddTool(server, &mcp.Tool{Name: "read_tia_registers", Description: "Read the current values of the write-only TIA registers (COLUP0/1, COLUPF, COLUBK, NUSIZ, CTRLPF, PF0/1/2, REFP, VDEL, ENAM/ENABL, GRP, etc.) straight from emulator state. Use this to confirm a 'sta COLUP0' actually took effect instead of inferring from pixel colors."}, handleReadTIARegisters)
	mcp.AddTool(server, &mcp.Tool{Name: "read_collisions", Description: "Read the 8 TIA collision latches (CXxx, $30-$37; sticky until CXCLR) as named boolean pairs (p0_p1, m0_p0, p0_pf, bl_pf, ...). Structured replacement for raw peeks of the collision registers."}, handleReadCollisions)
	mcp.AddTool(server, &mcp.Tool{Name: "read_audio", Description: "Read the current TIA audio register values for both channels: control (AUDC, waveform), freq (AUDF, divider), volume (AUDV). Lets you verify sound numerically — read_tia/read_row only cover video."}, handleReadAudio)
	mcp.AddTool(server, &mcp.Tool{Name: "read_audio_trace", Description: "Trace the TIA audio registers (AUDC control / AUDF freq / AUDV volume) for BOTH channels over N frames, returning the per-frame time-series (control[]/freq[]/volume[] per channel) — the audio analog of read_motion. NOTE: ADVANCES the emulator N frames, so trigger the sound first (fire / hit / start moving), then call. Captures a whole sound envelope (the attack/decay of a fire or explosion, an engine pitch change) in one call instead of stepping frame-by-frame with read_audio by hand."}, handleReadAudioTrace)
	mcp.AddTool(server, &mcp.Tool{Name: "read_ram_trace", Description: "Trace up to 16 RAM addresses ($80-$FF) over N frames, returning each address's per-frame value time-series (traces[i][f]) — the read_motion of arbitrary game state. NOTE: ADVANCES the emulator N frames (input set via set_input sticks across the trace). Collapses a manual step_frame+peek loop into one call: measure as NUMBERS how a tank's X/Y, an AI mode/timer, a score, or any RAM byte evolves over time (e.g. frames-to-escape a region, a decay curve, a stuck oscillation) instead of hand-stepping."}, handleReadRAMTrace)
	mcp.AddTool(server, &mcp.Tool{Name: "read_row", Description: "Read one visible scanline's pixel colors as run-length runs {clock,len,hex} across visible clock 0..159. Numerical readout for playfield lit-columns and per-scanline color (judge by data, not by eyeballing the screenshot)."}, handleReadRow)
	mcp.AddTool(server, &mcp.Tool{Name: "decompose_row", Description: "Decompose one visible scanline into WHICH TIA OBJECT drew each pixel: run-length runs {clock,len,element} across visible clock 0..159, element in {BG,PF,P0,P1,M0,M1,BL}. The attribution sibling of read_row (colours) and beamtrace (register writes) — answers 'is this part of the picture playfield, a player, a missile, or the ball?'. Essential for reverse-engineering how a running ROM composes its screen (sprite vs missile vs ball vs playfield use per scanline). Same absolute scanline coordinate as read_row."}, handleDecomposeRow)
	mcp.AddTool(server, &mcp.Tool{Name: "read_motion", Description: "Track a TIA object (P0/M0/P1/M1/BL) over N frames and report how smoothly it moves: per-frame velocity (1st difference), acceleration (2nd difference), and jerk_rms (RMS of the 2nd difference; 0 = constant velocity, high = judder/stutter). Tracks the exact horizontal HmovedPixel (x) and the rendered vertical top. Turns 'does it judder / ブルブル' into a number — automates the hand frame-by-frame trace. NOTE: ADVANCES the emulator N frames, so set up the state (serve/step) first; track the rendered top over a window where the object moves on a uniform background (x is always exact). For the exact vertical position of a small missile/ball (a bullet) — where the rendered-top scan latches onto a border — use spritey instead."}, handleReadMotion)
	mcp.AddTool(server, &mcp.Tool{Name: "spritey", Description: "Numeric VERTICAL position of a TIA object (P0/M0/P1/M1/BL): its drawn scanline extent (y_top/y_bot/height in grid-y, same coord as read_row) + X (HmovedPixel), found by matching the object's OWN colour at its X column. Fills the gap read_tia (X only) and read_motion (rendered-top, unreliable for a 1-2px missile against a bordered playfield) leave. frames=1 (default) reports the CURRENT frame without advancing; frames>1 ADVANCES the emulator and returns the per-frame Y trajectory — trace a bullet's ricochet as numbers (y_top rises then falls at each top/bottom bounce). present=false = disabled/off-screen. Caveat: matches by colour, so another same-colour object within ~8px right (e.g. a just-fired missile still overlapping its player) can widen the extent until they separate."}, handleSpriteY)
	mcp.AddTool(server, &mcp.Tool{Name: "set_input", Description: "Inject controller input or a console panel switch (the headless input path; poke does NOT affect input). player 0=P0/left port, 1=P1/right. Joystick actions: left|right|up|down|fire|center|paddle. Console panel switches: reset|select|color|p0pro|p1pro (pressed=true is the active state; reset/select are momentary so press then release across frames to start a game). pressed=true holds, false releases (sticks). action=paddle uses value 0.0..1.0 and plugs the paddle peripheral on first use. center releases all directions."}, handleSetInput)
	mcp.AddTool(server, &mcp.Tool{Name: "peek", Description: "Read one byte of memory without side effects."}, handlePeek)
	mcp.AddTool(server, &mcp.Tool{Name: "poke", Description: "Write one byte of memory."}, handlePoke)
	mcp.AddTool(server, &mcp.Tool{Name: "breakif", Description: "Run up to max_frames, halting when the beam reaches (until_scanline, until_clock)."}, handleBreakIf)
	mcp.AddTool(server, &mcp.Tool{Name: "assert_line_budget", Description: "Run up to max_frames and halt the moment a logical line (the interval between WSYNC strobes) overruns its CPU-cycle budget and eats extra scanlines — the failure mode that silently rolls the screen. budget defaults to 76 (one scanline); raise it for multi-line kernels. Returns over=true with at_scanline (the overrunning line's start) and line_cycles (machine cycles it consumed). Observes ONE run (∃) — for an all-paths proof use prove_line_budget. Optional patch=[{symbol|addr,bytes}] applies temporary ROM byte patches to a COPY for this measurement only (e.g. a lightweight positioning table), fresh-boots it, and ALWAYS reloads the original ROM afterwards — replaces the hand-edit→assemble→assert→restore ritual and its forget-to-restore failure mode. pokes=[{addr,value}] seeds RAM after the patched boot."}, handleBudgetGuard)
	mcp.AddTool(server, &mcp.Tool{Name: "profile_line_budget", Description: "Quantitative sibling of assert_line_budget (PONG-C3): profile `max_frames` frames and return, PER logical line (keyed by the PC of the STA WSYNC that OPENS it — stable even when overscan physics rows drift across scanlines), the measured WORST CPU cycles between WSYNC strobes (exact, beam-derived; <=76 fits one scanline), the physical lines it consumed, how often it ran, and the frame/scanline where the worst hit. Replaces trim-by-guess-and-assert with 'row4 worst = 78cy → trim 2'. Optional watch=[symbols/addrs] snapshots those RAM values at the opening strobe of each line's worst interval = the arg values the worst path read. Optional patch/pokes as in assert_line_budget (temporary, auto-restored). ADVANCES the emulator."}, handleProfileLines)
	mcp.AddTool(server, &mcp.Tool{Name: "assert_edge_coincidence", Description: "Worst-path fuzz for edge-compare kernels (PONG-class): the true per-line worst case is ALL edge variables (ball top/bottom, paddle tops/bottoms...) landing on the SAME Y (+~5cy per extra hit) — an overrun free-running tests can miss for hundreds of frames. Pokes every listed zero-page address to the same Y, runs frames_per_y frames under assert_line_budget semantics, sweeps Y over [y_min..y_max], and reports every failing alignment. Optional patch (auto-restored) for lightweight positioning tables."}, handleEdgeCoincidence)
	mcp.AddTool(server, &mcp.Tool{Name: "defuse", Description: "Statically answer, over ALL paths, WHICH INSTRUCTION WRITES WHICH ADDRESS — the forall sibling of watch_ram/read_ram_trace, which report only what the runs you did happened to do. Assembles asm_path, walks the CFG, and per WSYNC-to-WSYNC region lists each address's writer PCs and reader PCs (with source locations), plus the whole-program may-write set. Targets are resolved through the EFFECTIVE address, so an indexed store is attributed to the register it actually reaches and a PHA lands wherever SP points (the stack trick writes a TIA register that way, and nothing else here can see it). Also reports UNINITIALISED READS: reads of RAM that no path from reset has definitely written first, which on hardware is power-on rubbish while an emulator hands out a defined value and the bug never surfaces in testing. Use it when a byte is being written from more than one place, when a value looks stale, or before trusting a cell that setup was supposed to fill. DECLINES a bank-switched image outright rather than describing a program that does not exist: this analysis keys on a flat address and reports per flat PC, and on a bank-switched cartridge that produced an empty may-write set for a ROM that demonstrably writes RAM — empty only because the flat 8K fold decodes almost nothing."}, handleDefUse)
	mcp.AddTool(server, &mcp.Tool{Name: "beam_intervals", Description: "PROVE where every TIA write lands on the scanline, on EVERY path — the forall sibling of beamtrace, which answers for one execution. Assembles asm_path and carries elapsed cycles as an interval through each WSYNC-to-WSYNC region, so each write comes back with the earliest and latest beam clock it can reach (same coordinates as read_row: HBLANK -68..-1, visible 0..159), plus exact=true when its position does not depend on the path at all. A WIDE window is a finding, not a gap: it means the write's position varies with the branch taken, which is a bug in a kernel meant to be cycle-exact; crosses_line marks the worse case where even the SCANLINE depends on the path. Reach for it whenever a colour or graphics register is written after a branch, when one column of one line looks wrong, or to confirm a positioning write is genuinely cycle-exact before building on it. Nothing else in the 2600 ecosystem computes this — the state of the art is hand-counting a single path. Declines a bank-switched image: beam windows are computed from a flat-address decode of one 4K fold, and a confidently wrong beam clock looks exactly like a right one."}, handleBeamIntervals)
	mcp.AddTool(server, &mcp.Tool{Name: "prove_line_budget", Description: "Statically PROVE a kernel's per-scanline cycle budget over ALL reachable paths (∀) — the static sibling of assert_line_budget, which observes only one run (∃). Assembles asm_path, decodes from the reset/IRQ/NMI vectors, cuts the CFG at every STA WSYNC, and proves each WSYNC-to-WSYNC region's worst-case CPU cycles <= budget (default 76). Returns certified plus any over-budget regions (each with a cycle-by-cycle worst path + source location) and any regions it could not bound (unbounded loop / JSR / indirect JMP), reported honestly rather than passed. Run it BEFORE executing a kernel to catch a branch path that overruns only sometimes — the timing trap that rolls the screen on hardware while a lucky run looks fine. Handles a BANK-SWITCHED cartridge as one merged program keyed on (bank, address): an instruction whose data access reaches a bank-switch hotspot continues at the same address in the bank that hotspot's own mapper symbol names, so a WSYNC-to-WSYNC region that crosses banks gets a real proven worst case (measured: litmus_bank 54cy, _f6 72cy, _f4 128cy — each equal to what the emulator measures for the same interval). Still refused and counted in unmodelled_switches, which blocks certification: an instruction whose own bytes span a hotspot, a jmp/jsr INTO one, an unresolvable indirect access under a hotspot-bearing mapper, a hotspot symbol that does not name a bank, and any mapper whose banks are not the whole 4K window at $F000. On a banked image @lines/@amax cannot be read (DASM's listing addresses are physical ROM offsets there) and source_annotations says so."}, handleProveLineBudget)
	mcp.AddTool(server, &mcp.Tool{Name: "trace_clocks", Description: "Execute the next N instructions and return each one's beam anatomy: PC, opcode, CPU cycles (WSYNC stalls visible as large counts), and start/end (scanline, color clock). Sub-instruction OBSERVATION granularity — the practical recovery of step_clock (Gopher2600 cannot suspend mid-instruction; see docs/mcp-tools.md)."}, handleTraceClocks)
	mcp.AddTool(server, &mcp.Tool{Name: "beamtrace", Description: "Write→visible-pixel timeline (authoring aid): trace `frames` frames and return, per scanline, every TIA write with the beam clock it lands at, the register name/kind, the value, and the visible-pixel span [vis_from,vis_to) it governs (until the next write to the same register). Answers 'where on the line does this sta GRP0 actually paint?'. Omit `scanline` for all scanlines that have writes. ADVANCES the emulator `frames` frames — set up the state first."}, handleBeamtrace)
	mcp.AddTool(server, &mcp.Tool{Name: "beam_race", Description: "Advisory beam-race report (authoring aid): for every pixel-data write (GRP0/GRP1/ENAM0/ENAM1/ENABL) over `frames` frames, the controlled object's X and whether the write reached the beam in time (before_beam = clock<=X; otherwise that line draws the PREVIOUS value = one-line lag). FACTUAL, no verdict — a 'late' line may be an intended next-line pre-load; use scenario checks.no_beam_race when you can declare intent. ADVANCES the emulator `frames` frames."}, handleBeamRace)
	mcp.AddTool(server, &mcp.Tool{Name: "spritepos", Description: "Forward sprite-position solver (authoring aid): given target X (0..159) return the SetXPos routine input, the div-15-coarse/HMOVE-fine decomposition, a paste-able snippet, and the position the hardware ACTUALLY reaches (achieved_x = HmovedPixel, measured by running a calibration kernel — exact=true when it equals the target). The X(N) offset is kernel-specific, so the answer is verified, not trusted. Self-contained: does not use or disturb the loaded ROM."}, handleSpritepos)
	mcp.AddTool(server, &mcp.Tool{Name: "watch_ram", Description: "Run instruction-by-instruction until RAM[addr] CHANGES (returns old/new value and the PC of the writing instruction), bounded by max_frames. Granularity is per-instruction (Gopher2600 cannot suspend mid-instruction); same-value stores are invisible to change detection."}, handleWatchRAM)
	mcp.AddTool(server, &mcp.Tool{Name: "run_scenario", Description: "Run regression scenario JSON files (input timeline + numeric assertions) in-process and return pass/fail with failing assertion details — the cmd/scenario verdict from the live loop."}, handleRunScenario)
	mcp.AddTool(server, &mcp.Tool{Name: "analyze_screen", Description: "Run the ingest analyzer on the CURRENT emulator frame (no file needed): playfield bands as PF bytes, sprite candidates with GRP bytes + per-row colors, groups, fidelity, plus the TIA-grid overlay. The reverse-direction read of whatever is on screen right now."}, handleAnalyzeScreen)
	mcp.AddTool(server, &mcp.Tool{Name: "analyze_image", Description: "Ingest a game screenshot (grade A = Stella F12 PNG, unmodified, TV effects off; any integer multiple of the 160-clock raster) and return TIA-space analysis: normalized raster + palette quantization to real COLUxx values, playfield bands as PF0/PF1/PF2 bytes (repeat/reflect/asymmetric, score-mode flag), sprite candidates with GRP bytes + per-row colors + NUSIZ copy folding, plus a TIA-grid overlay image. Ambiguous elements carry confidence; one screenshot is one frame of truth (flicker objects appear partially)."}, handleAnalyzeImage)
	mcp.AddTool(server, &mcp.Tool{Name: "get_screen_annotated", Description: "Return the latest frame as a PNG with an XY grid in real TIA coordinates (x=clock 0..159, y=scanline) and labelled sprite-position markers. The primary visual channel: the user can point at it and instruct by coordinate. Also returns sprite positions numerically."}, handleScreenAnnotated)
	registerStateTools(server) // save_state / restore_state / probe_ram_semantics (cmd/harness/tools_state.go)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "harness:", err)
		os.Exit(1)
	}
}
