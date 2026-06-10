// Command harness は Gopher2600 ベースの Atari 2600 検証ハーネスを MCP (stdio) で
// 露出する。Claude が load_rom → step → read で「やったこと＝結果」を数値で観測する。
// 仕様は docs/mcp-tools.md（全 API 裏取り済み）。数値ファースト＝画像は Phase 2.3。
package main

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kidsnz/atari2600-dev/internal/emu"
)

// --- グローバル状態（stdio は逐次だが念のため mutex 保護）---

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
		bin = strings.TrimSuffix(in.AsmPath, filepath.Ext(in.AsmPath)) + ".bin"
	}

	// dasm -f3（DASM の生バイナリ出力）。CombinedOutput で失敗行を含む診断をそのまま返す。
	out, err := exec.Command("dasm", in.AsmPath, "-f3", "-o"+bin).CombinedOutput()
	if err != nil {
		// アセンブル失敗は MCP エラーにせず Ok=false＋dasm 出力で構造化返却（Claude が失敗行を見て直す）。
		return nil, AssembleOut{Ok: false, BinPath: bin, DasmOutput: string(out)}, nil
	}

	spec := in.TVSpec
	if spec == "" {
		spec = "NTSC"
	}
	e, err := emu.New(spec)
	if err != nil {
		return nil, AssembleOut{}, fmt.Errorf("new emu: %w", err)
	}
	if err := e.LoadROM(bin); err != nil {
		return nil, AssembleOut{Ok: true, BinPath: bin, DasmOutput: string(out)}, fmt.Errorf("assembled ok but load failed: %w", err)
	}
	current = e
	return nil, AssembleOut{Ok: true, BinPath: bin, DasmOutput: string(out), Loaded: true, Coords: coordsOf(e)}, nil
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
type ReadCPUOut struct {
	PC     uint16   `json:"pc"`
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
	CyclesSinceMark       int64  `json:"cycles_since_mark"`        // 直近 mark 以降の累積
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

// --- set_input（ジョイスティック注入。poke は入力に効かない）---

type SetInputIn struct {
	Player  int    `json:"player,omitempty" jsonschema:"player port (0 left/P0 default, 1 right/P1)"`
	Action  string `json:"action" jsonschema:"one of left|right|up|down|fire|center"`
	Pressed bool   `json:"pressed,omitempty" jsonschema:"press/hold when set, release when unset (ignored for center)"`
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
	if err := e.SetInput(in.Player, in.Action, in.Pressed); err != nil {
		return nil, SetInputOut{}, err
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

type BudgetIn struct {
	MaxFrames int `json:"max_frames,omitempty" jsonschema:"upper bound on frames to run (default 1)"`
	Budget    int `json:"budget,omitempty" jsonschema:"CPU-cycle budget per WSYNC interval (default 76 = one scanline)"`
}
type BudgetOut struct {
	Over       bool   `json:"over"`        // true=ある論理ラインが予算超過（ロール要因）で停止
	AtScanline int    `json:"at_scanline"` // 超過した論理ラインの開始 scanline（over=true 時）
	LineCycles int    `json:"line_cycles"` // そのラインが消費した概算 machine cycle（消費ライン数×76）
	Coords     Coords `json:"coords"`
}

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
	over, atScanline, lineCycles, err := e.RunUntilBudget(maxFrames, in.Budget)
	if err != nil {
		return nil, BudgetOut{}, fmt.Errorf("run until budget: %w", err)
	}
	return nil, BudgetOut{Over: over, AtScanline: atScanline, LineCycles: lineCycles, Coords: coordsOf(e)}, nil
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

// --- main ---

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "atari2600-harness",
		Version: "0.9.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "load_rom", Description: "Load a .bin ROM and reset the VCS (TV spec NTSC/PAL/AUTO)."}, handleLoadROM)
	mcp.AddTool(server, &mcp.Tool{Name: "assemble_and_load", Description: "Assemble a .asm with DASM (-f3) and, on success, load the resulting ROM in one shot. On failure returns ok=false with the DASM output (including the failing line) instead of an error — collapses the edit->dasm->load loop."}, handleAssembleAndLoad)
	mcp.AddTool(server, &mcp.Tool{Name: "step_frame", Description: "Run the emulator for N frames."}, handleStepFrame)
	mcp.AddTool(server, &mcp.Tool{Name: "step_instruction", Description: "Execute exactly one CPU instruction (consuming any pending WSYNC stall first). Returns its cycle count and beam coords — pairs with read_cycles to step through a kernel one instruction at a time."}, handleStepInstruction)
	mcp.AddTool(server, &mcp.Tool{Name: "step_scanline", Description: "Advance until the TV scanline increments once (stops at the next scanline, or scanline 0 of the next frame). Returns CPU cycles consumed across that scanline and beam coords — for inspecting kernel state line by line."}, handleStepScanline)
	mcp.AddTool(server, &mcp.Tool{Name: "read_cpu", Description: "Read 6507 registers, status flags, and beam coords."}, handleReadCPU)
	mcp.AddTool(server, &mcp.Tool{Name: "read_cycles", Description: "Read CPU cycle counts from the simulator (rule #2: never count cycles by hand): the last instruction's cycles, cycles since the last mark, and total cycles since ROM load. Set reset=true to mark a new measurement baseline (cycles_since_mark restarts at 0)."}, handleReadCycles)
	mcp.AddTool(server, &mcp.Tool{Name: "read_ram", Description: "Dump the 128 bytes of RAM ($80-$FF) as hex."}, handleReadRAM)
	mcp.AddTool(server, &mcp.Tool{Name: "read_tia", Description: "Read TIA sprite positions (ResetPixel/HmovedPixel) and HBLANK. Authoritative for horizontal-position checks."}, handleReadTIA)
	mcp.AddTool(server, &mcp.Tool{Name: "read_tia_registers", Description: "Read the current values of the write-only TIA registers (COLUP0/1, COLUPF, COLUBK, NUSIZ, CTRLPF, PF0/1/2, REFP, VDEL, ENAM/ENABL, GRP, etc.) straight from emulator state. Use this to confirm a 'sta COLUP0' actually took effect instead of inferring from pixel colors."}, handleReadTIARegisters)
	mcp.AddTool(server, &mcp.Tool{Name: "read_collisions", Description: "Read the 8 TIA collision latches (CXxx, $30-$37; sticky until CXCLR) as named boolean pairs (p0_p1, m0_p0, p0_pf, bl_pf, ...). Structured replacement for raw peeks of the collision registers."}, handleReadCollisions)
	mcp.AddTool(server, &mcp.Tool{Name: "read_row", Description: "Read one visible scanline's pixel colors as run-length runs {clock,len,hex} across visible clock 0..159. Numerical readout for playfield lit-columns and per-scanline color (judge by data, not by eyeballing the screenshot)."}, handleReadRow)
	mcp.AddTool(server, &mcp.Tool{Name: "set_input", Description: "Inject joystick input (the headless input path; poke does NOT affect input). player 0=P0/left port, 1=P1/right. action left|right|up|down|fire|center. pressed=true holds, false releases; state persists until changed. center releases all directions."}, handleSetInput)
	mcp.AddTool(server, &mcp.Tool{Name: "peek", Description: "Read one byte of memory without side effects."}, handlePeek)
	mcp.AddTool(server, &mcp.Tool{Name: "poke", Description: "Write one byte of memory."}, handlePoke)
	mcp.AddTool(server, &mcp.Tool{Name: "breakif", Description: "Run up to max_frames, halting when the beam reaches (until_scanline, until_clock)."}, handleBreakIf)
	mcp.AddTool(server, &mcp.Tool{Name: "assert_line_budget", Description: "Run up to max_frames and halt the moment a logical line (the interval between WSYNC strobes) overruns its CPU-cycle budget and eats extra scanlines — the failure mode that silently rolls the screen. budget defaults to 76 (one scanline); raise it for multi-line kernels. Returns over=true with at_scanline (the overrunning line's start) and line_cycles (machine cycles it consumed)."}, handleBudgetGuard)
	mcp.AddTool(server, &mcp.Tool{Name: "get_screen_annotated", Description: "Return the latest frame as a PNG with an XY grid in real TIA coordinates (x=clock 0..159, y=scanline) and labelled sprite-position markers. The primary visual channel: the user can point at it and instruct by coordinate. Also returns sprite positions numerically."}, handleScreenAnnotated)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "harness:", err)
		os.Exit(1)
	}
}
