# MCP tools — implementation spec

The settled spec for the tools exposed by the `cmd/harness` MCP server. **Every API here is verified
against the installed SDK / Gopher2600 nightly** (zero guesswork).
(Note: this document was written during the numeric-tools implementation era; the image tool
`get_screen_annotated` was implemented in v0.5.0.)

## SDK bootstrap (go-sdk v1.6.1)

```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

server := mcp.NewServer(&mcp.Implementation{
    Name: "atari2600-harness", Version: "0.3.0",
}, nil)

mcp.AddTool(server, &mcp.Tool{Name: "load_rom", Description: "..."}, handleLoadROM)
// ... each tool ...

server.Run(context.Background(), &mcp.StdioTransport{})
```

- Generic signature: `mcp.AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out])`.
- Handler signature: `func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)`.
- **Make `Out` a typed struct and the SDK auto-generates the JSON Schema and auto-fills `StructuredContent`.**
  `*mcp.CallToolResult` may return `nil` (with a typed Out the SDK fills Content too).
- The `In` struct tag `jsonschema:"..."` becomes the property description. Argument-less tools use `In = struct{}`.

## State management

- Hold a single `*emu.Emu` (`internal/emu`) as a package global, protected by a `sync.Mutex`.
- `load_rom` rebuilds from `emu.New(spec)` and attaches each time (deterministic reset).
- Return an error if read/step tools are called before a ROM is loaded.

## Shared return component

```go
type Coords struct {
    Frame    int `json:"frame"`
    Scanline int `json:"scanline"`
    Clock    int `json:"clock"`     // ★ Gopher2600 convention: HBLANK −68..−1 / visible 0..159
}
```
**★ Important (settled by real-hardware verification):** `GetCoords().Clock` is the internal clock minus
`ClksHBlank(68)`. So the start of a scanline (start of HBLANK) is **clock = −68**, the first visible pixel
is **clock = 0**, and the right edge of the visible region is **clock = 159**. A sprite's `HmovedPixel`
uses the same visible pixel coordinate (0–159), so the horizontal-position litmus test can compare the two
directly. (The "0–159 color clocks" notation in resources.md / CLAUDE.md matches this 0-origin, not 0–227.)
Fill `Coords` into every tool's Out (following mcp-gameboy's "observe the result of what you did" numerically).
Source: `emu.VCS.TV.GetCoords()` → `{Frame, Scanline, Clock}`.

## Tool list (minimal prototype)

### 1. `load_rom`
- In: `{ Path string `jsonschema:"path to .bin ROM"`; TVSpec string `jsonschema:"NTSC|PAL|AUTO (default NTSC)"` }`
- Behavior: if `TVSpec` is empty, `"NTSC"`. `emu.New` → `emu.LoadROM(Path)`. Store into the global.
- Out: `{ Coords; Message string }`

### 1b. `assemble_and_load`  ★ build-loop shortening (P3, v0.16.0)
- In: `{ AsmPath string; BinPath string (defaults asm→.bin); TVSpec string (default NTSC) }`
- Behavior: `exec.Command("dasm", asm, "-f3", "-o"+bin).CombinedOutput()`. On success, `emu.New`+`LoadROM` to load immediately.
- Out: `{ Ok bool; BinPath string; DasmOutput string; Loaded bool; Coords }`.
  - On failure, **do not raise an MCP error**; return `Ok=false` + `DasmOutput` (the failing `"file (N): error: ..."` line) so
    the model can fix and resubmit on the spot. Self-contained in `cmd/harness` (dasm assumed on PATH).
- Verification: MCP e2e — smoke.asm loads OK / a broken asm gives Ok=false + the failing line.

### 2. `step_frame`
- In: `{ Count int `jsonschema:"frames to run (default 1)"` }`
- Behavior: if `Count<=0`, 1. `emu.RunFrames(Count)`.
- Out: `{ Coords }`
- Note: to return a scanline count, the last single frame can be measured with `emu.StepFrame()` (optional).

### 2b. `step_instruction` / `step_scanline`  ★ intra-frame granularity (B-2, v0.15.0)
- `step_instruction` In: `struct{}` / Behavior: `emu.StepInstruction` (drain a pending WSYNC stall, then execute one instruction) /
  Out: `{ LastInstructionCycles int; Coords }`. Pair with `read_cycles` to follow one instruction at a time.
- `step_scanline` In: `struct{}` / Behavior: `emu.StepScanline` (until scanline +1; at a frame boundary, stop at scanline 0 of the next frame) /
  Out: `{ CyclesConsumed int64; Coords }`.
  - Note: because stepping is per-instruction, `CyclesConsumed` runs to the first instruction boundary at the next scanline's start
    (may slightly overshoot by one instruction). A color-clock-granular `step_clock` is unimplemented since `Step` is per-instruction.
- Verification: `internal/emu/emu_step_test.go` (2/3 cy on litmus_cycles; scanline +1 wrap on smoke).

### 3. `read_cpu`
- In: `struct{}`
- Behavior: `cpu := emu.VCS.CPU`.
- Out:
  ```go
  type CPUState struct {
      PC uint16 // cpu.PC.Value()
      A, X, Y uint8 // cpu.A.Value() etc.
      SP uint8 // uint8(cpu.SP.Address())  ← low byte
      Status uint8 // cpu.Status.Value()
      Flags struct{ N,V,B,D,I,Z,C bool } // cpu.Status.Sign, .Overflow, .Break, .DecimalMode, .InterruptDisable, .Zero, .Carry
      Coords
  }
  ```

### 3b. `read_cycles`  ★ brings rule 2 into the real loop (B-1, v0.12.0)
- In: `{ Reset bool }` (with `reset=true`, align the interval baseline to now = `cycles_since_mark` starts at 0)
- Behavior: return the CPU cycles `emu` accumulates at instruction boundaries. Source = `CPU.LastResult.Cycles`
  (real cycles including the +1 for PageFault/branch). To stay consistent across all progress paths, call `e.accumCycle()` from both
  the continueCheck of `RunFrames`/`RunUntilBeam` (per instruction completion, `run.go`) and `StepFrame`'s own loop.
- Out:
  ```go
  type ReadCyclesOut struct {
      LastInstructionCycles int   // cycles of the most recent instruction (= LastResult.Cycles)
      CyclesSinceMark       int64 // accumulated since the last MarkCycles
      TotalCycles           int64 // accumulated since ROM load
      Coords
  }
  ```
  - **Verification basis**: on a WSYNC-free ROM (`roms/litmus/litmus_cycles.bin`), since the CPU never stalls, the invariant
    "executed cycles × 3 == color clocks advanced" holds exactly at instruction boundaries. Cross-checked in
    `internal/emu/emu_cycles_test.go`. One frame = 263 lines × 76 cy = `TotalCycles 19988`.

### 4. `read_ram`
- In: `struct{}`
- Behavior: 128 bytes of RAM = `$80–$FF`. Loop `emu.PeekRAM(addr)` over 0x80..0xFF.
- Out: `{ Base uint16 (=0x80); Hex string (concatenated 256-char hex); Coords }`

### 5. `read_tia`  ★ the core of the litmus test
- In: `struct{}`
- Behavior: `v := emu.VCS.TIA.Video`. Read sprite positions and collisions.
- Out:
  ```go
  type Sprite struct { ResetPixel, HmovedPixel int }
  type TIAState struct {
      Player0, Player1, Missile0, Missile1, Ball Sprite
      // each v.Player0.ResetPixel / v.Player0.HmovedPixel (int)
      Hblank bool   // emu.VCS.TIA.Hblank
      Coords
  }
  ```
  - Collisions are `v.Collisions` (`*video.Collisions`). Omit in the minimal version; if there's room, decode CXxx to bool.
  - **Judgment basis**: whether the player's `HmovedPixel` matches `X = 3N−54` (horizontal-position verification).

### 5b. `read_tia_registers`  ★ closes the rest of gap A (P1, v0.14.0)
- In: `struct{}`
- Behavior: read the current values of the write-only TIA registers directly from exported fields of `e.VCS.TIA.Video`
  (`emu.ReadTIARegisters`). Instead of inferring color (`read_row`), confirm by measurement whether `sta COLUPx` took effect.
- Out: `emu.TIARegisters` + `Coords`. Breakdown:
  - Player0/1 (`PlayerRegs`): `color`(COLUP) / `nusiz` / `size_and_copies` / `gfx_new` / `gfx_old`(GRP) /
    `reflected`(REFP) / `vertical_delay`(VDELP).
  - Missile0/1 (`MissileRegs`): `color` / `nusiz` / `size` / `copies` / `enabled`(ENAM) / `reset_to_player`(RESMP).
  - Ball (`BallRegs`): `color` / `size` / `enabled`(ENABL) / `vertical_delay`(VDELBL).
  - Playfield (`PlayfieldRegs`): `pf0`/`pf1`/`pf2` / `foreground_color`(COLUPF) / `background_color`(COLUBK) /
    `ctrlpf` / `reflected`(D0) / `priority`(D2) / `scoremode`(D1).
- Note: PF0 holds only the upper nibble (write `$FF` → read `$F0`) = real TIA behavior.
- Verification: smoke's COLUBK=$1E / litmus_pf's nonzero PF. `internal/emu/emu_tia_test.go`.

### 5d. `read_audio`  ★ audio verification path (R-2, v0.17.0)
- In: `struct{}`
- Behavior: read the current TIA audio register values from `e.VCS.TIA.Audio.PeekChannels()` (exported) (`emu.ReadAudio`).
  read_tia/read_row are video-only; this fills the missing verification path for audio.
- Out: `emu.AudioState` (`channel0`/`channel1`, each `control`(AUDC) / `freq`(AUDF) / `volume`(AUDV)) + `Coords`.
- Verification: exact match on `litmus_audio.bin` (ch0=$0C/$14/$0A, ch1=$04/$1F/$08). `internal/emu/emu_audio_test.go`.

### 5c. `read_collisions`  ★ CXxx structured (P1, v0.14.0)
- In: `struct{}`
- Behavior: peek (side-effect-free) the collision registers `$30–$37` (each a D7/D6 latch, sticky, held until CXCLR) and decode to
  named boolean pairs (`emu.ReadCollisions` / pure function `decodeCollisions`).
- Out: `emu.Collisions` (`p0_p1`/`m0_m1`/`m0_p0`/`m0_p1`/`m1_p0`/`m1_p1`/`p0_pf`/`p0_bl`/`p1_pf`/`p1_bl`/
  `m0_pf`/`m0_bl`/`m1_pf`/`m1_bl`/`bl_pf`) + `Coords`. Bit assignment verified against Gopher2600 `collisions.go` `tick()`.
- Verification: unit tests for all D7/D6 pairs, all-false with no sprites, BL-PF positive on `litmus_collide.bin` (fully-lit PF + ball).

### 6. `peek` / `poke`
- `peek` In: `{ Addr uint16 }` → Out: `{ Value uint8; Coords }` (`emu.VCS.Mem.Peek`).
- `poke` In: `{ Addr uint16; Value uint8 }` → Out: `{ Coords }` (`emu.VCS.Mem.Poke`).

### 7. `breakif` (conditional run)
- In: `{ MaxFrames int; UntilScanline int; UntilClock int }` (minimal version: stop at a beam position)
- Behavior: in the `continueCheck` of `emu.VCS.RunForFrameCount(MaxFrames, continueCheck)`, return `govern.Ending`
  once `GetCoords()` reaches the target (Scanline, Clock).
  (`govern` = `github.com/jetsetilly/gopher2600/govern`, not `.../hardware/television/...`. Import the same
  state type as `RunForFrameCount`'s return.)
- Out: `{ Coords; Halted bool }`
- Note: stopping on RAM value or collision is a later extension. First just get beam-position stopping working.

### 7b. `assert_line_budget`  ★ the crux of gap B (B-3, v0.13.0)
- In: `{ MaxFrames int (default 1); Budget int (default 76 = CPU cycles for one line) }`
- Behavior: stop the moment a logical line (= the interval between WSYNC strobes) exceeds its budget and eats into extra scanlines.
  Numerically catches the "per-scanline overrun → roll" that silently killed Pong v2. `emu.RunUntilBudget`.
  - **Detection**: a WSYNC strobe = a true→false transition of the CPU `RdyFlg` (only WSYNC lowers RDY). Since WSYNC stalls until the
    next scanline boundary, the scanline delta between consecutive strobes = the physical lines consumed by that logical line
    (OK if it stays at 1, overrun if ≥2). Stop if it exceeds `maxLines = budget/76` (default 1).
  - Free-run 2 frames before measuring (right after boot, VSYNC sync is disturbed and causes false positives). Raise the budget for multi-line kernels.
- Out:
  ```go
  type BudgetOut struct {
      Over       bool // true = exceeded budget (a roll cause) and stopped
      AtScanline int  // start scanline of the overrunning logical line
      LineCycles int  // approximate machine cycles that line consumed (physical lines consumed × 76)
      Coords
  }
  ```
  - **Verification**: `roms/litmus/litmus_overrun.bin` (one heavy line of ~100cy before WSYNC) gives `Over=true`,
    `LineCycles=152`. smoke / frogger give `Over=false` (no false positives). `internal/emu/emu_budget_test.go`.

## Acceptance check

1. `go build ./cmd/harness` passes.
2. Over stdio, JSON-RPC `initialize` → `tools/list` returns the tools (the prototype returned 7; the harness now exposes 19).
3. `load_rom roms/litmus/smoke.bin` → `step_frame {Count:10}` → `read_ram` shows `42` at `$80`.
4. `read_cpu`'s Coords.Frame is around 10.

For a manual check, pipe the `initialize`/`tools/call` JSON into harness over stdin, or call it via an MCP
client (registered in Claude Code's `.mcp.json`).
