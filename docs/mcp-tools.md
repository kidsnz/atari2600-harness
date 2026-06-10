# MCP Tools — 実装仕様（Phase 2.2）

`cmd/harness` の MCP サーバが露出するツールの確定仕様。**ここに書いた API は
全て installed SDK / Gopher2600 nightly で裏取り済み**（推測ゼロ）。Sonnet がこの
通りに実装すれば動く。数値ファースト＝画像（`get_screen_annotated`）は Phase 2.3 へ繰り延べ。

## SDK ブートストラップ（go-sdk v1.6.1）

```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

server := mcp.NewServer(&mcp.Implementation{
    Name: "atari2600-harness", Version: "0.3.0",
}, nil)

mcp.AddTool(server, &mcp.Tool{Name: "load_rom", Description: "..."}, handleLoadROM)
// ... 各ツール ...

server.Run(context.Background(), &mcp.StdioTransport{})
```

- ジェネリック署名: `mcp.AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out])`。
- ハンドラ署名: `func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)`。
- **`Out` を typed struct にすれば SDK が JSON Schema を自動生成し、`StructuredContent` を自動充填**。
  `*mcp.CallToolResult` は `nil` を返してよい（typed Out があれば SDK が Content も埋める）。
- `In` の struct タグ `jsonschema:"..."` がプロパティ説明になる。引数なしツールは `In = struct{}`。

## 状態管理

- パッケージグローバルに 1 台の `*emu.Emu`（`internal/emu`）を保持。`sync.Mutex` で保護。
- `load_rom` で毎回 `emu.New(spec)` から作り直してアタッチ（決定的リセット）。
- 未ロード時に read/step 系を呼ばれたらエラーを返す。

## 共通の戻り値部品

```go
type Coords struct {
    Frame    int `json:"frame"`
    Scanline int `json:"scanline"`
    Clock    int `json:"clock"`     // ★Gopher2600規約: HBLANK −68..−1 / 可視 0..159
}
```
**★重要（実機検証で確定）:** `GetCoords().Clock` は内部 clock から `ClksHBlank(68)` を
引いた値。よってスキャンライン先頭（HBLANK 開始）は **clock = −68**、可視領域の先頭ピクセルが
**clock = 0**、可視右端が **clock = 159**。スプライトの `HmovedPixel` も同じ可視ピクセル座標
（0–159）なので、横位置 litmus test では両者を直接比較できる。
（resources.md / CLAUDE.md の「0–159 カラークロック」表記はこの 0 起点と一致。0–227 ではない。）
全ツールの Out に `Coords` を埋める（mcp-gameboy 流「やったこと＝結果の観測」を数値で踏襲）。
取得元: `emu.VCS.TV.GetCoords()` → `{Frame, Scanline, Clock}`。

## ツール一覧（最小プロトタイプ）

### 1. `load_rom`
- In: `{ Path string `jsonschema:"path to .bin ROM"`; TVSpec string `jsonschema:"NTSC|PAL|AUTO (default NTSC)"` }`
- 動作: `TVSpec` 空なら `"NTSC"`。`emu.New` → `emu.LoadROM(Path)`。グローバルへ格納。
- Out: `{ Coords; Message string }`

### 2. `step_frame`
- In: `{ Count int `jsonschema:"frames to run (default 1)"` }`
- 動作: `Count<=0` なら 1。`emu.RunFrames(Count)`。
- Out: `{ Coords }`
- 補足: scanline 数を返したいなら最後の 1 フレームだけ `emu.StepFrame()` で計測可（任意）。

### 3. `read_cpu`
- In: `struct{}`
- 動作: `cpu := emu.VCS.CPU`。
- Out:
  ```go
  type CPUState struct {
      PC uint16 // cpu.PC.Value()
      A, X, Y uint8 // cpu.A.Value() 等
      SP uint8 // uint8(cpu.SP.Address())  ← 下位バイト
      Status uint8 // cpu.Status.Value()
      Flags struct{ N,V,B,D,I,Z,C bool } // cpu.Status.Sign, .Overflow, .Break, .DecimalMode, .InterruptDisable, .Zero, .Carry
      Coords
  }
  ```

### 3b. `read_cycles`  ★鉄則2 を実ループへ（B-1, v0.12.0）
- In: `{ Reset bool }`（`reset=true` で区間計測の基準点を今に揃える＝以後 `cycles_since_mark` は 0 から）
- 動作: `emu` が命令境界で累積する CPU サイクルを返す。累積源 = `CPU.LastResult.Cycles`
  （PageFault/分岐の +1 込み実サイクル）。全進行経路で一貫させるため `RunFrames`/`RunUntilBeam` の
  continueCheck（命令完了ごと, `run.go`）と `StepFrame` 自前ループ双方で `e.accumCycle()` を呼ぶ。
- Out:
  ```go
  type ReadCyclesOut struct {
      LastInstructionCycles int   // 直近 1 命令のサイクル数（= LastResult.Cycles）
      CyclesSinceMark       int64 // 直近 MarkCycles 以降の累積
      TotalCycles           int64 // ROM ロード以降の累積
      Coords
  }
  ```
  - **検証根拠**: WSYNC 不使用 ROM（`roms/litmus/litmus_cycles.bin`）では CPU 無停止のため命令境界で
    「実行サイクル × 3 == 進んだカラークロック数」が普遍則として厳密成立。`internal/emu/emu_cycles_test.go`
    で照合済み。1 フレーム = 263 lines × 76 cy = `TotalCycles 19988`。

### 4. `read_ram`
- In: `struct{}`
- 動作: RAM 128 バイト = `$80–$FF`。`emu.PeekRAM(addr)` を 0x80..0xFF でループ。
- Out: `{ Base uint16 (=0x80); Hex string (256 文字の連結 hex); Coords }`

### 5. `read_tia`  ★litmus test の中核
- In: `struct{}`
- 動作: `v := emu.VCS.TIA.Video`。スプライト位置と衝突を読む。
- Out:
  ```go
  type Sprite struct { ResetPixel, HmovedPixel int }
  type TIAState struct {
      Player0, Player1, Missile0, Missile1, Ball Sprite
      // 各 v.Player0.ResetPixel / v.Player0.HmovedPixel（int）
      Hblank bool   // emu.VCS.TIA.Hblank
      Coords
  }
  ```
  - 衝突は `v.Collisions`（`*video.Collisions`）。最小版では省略可、余裕あれば CXxx を bool 化。
  - **判定基準**: プレイヤー `HmovedPixel` が `X = 3N−54` に一致するか（横位置検証）。

### 5b. `read_tia_registers`  ★欠落A の残りを閉じる（P1, v0.14.0）
- In: `struct{}`
- 動作: 書込専用 TIA レジスタの現在値を `e.VCS.TIA.Video` の exported フィールドから直接読む（`emu.ReadTIARegisters`）。
  色推論（`read_row`）でなく実測で「`sta COLUPx` は効いたか」を確かめる。
- Out: `emu.TIARegisters` ＋ `Coords`。内訳:
  - Player0/1（`PlayerRegs`）: `color`(COLUP) / `nusiz` / `size_and_copies` / `gfx_new` / `gfx_old`(GRP) /
    `reflected`(REFP) / `vertical_delay`(VDELP)。
  - Missile0/1（`MissileRegs`）: `color` / `nusiz` / `size` / `copies` / `enabled`(ENAM) / `reset_to_player`(RESMP)。
  - Ball（`BallRegs`）: `color` / `size` / `enabled`(ENABL) / `vertical_delay`(VDELBL)。
  - Playfield（`PlayfieldRegs`）: `pf0`/`pf1`/`pf2` / `foreground_color`(COLUPF) / `background_color`(COLUBK) /
    `ctrlpf` / `reflected`(D0) / `priority`(D2) / `scoremode`(D1)。
- 注: PF0 は上位ニブルのみ保持（`$FF` 書込→読みは `$F0`）＝実 TIA 挙動。
- 検証: smoke の COLUBK=$1E / litmus_pf の PF 非ゼロ。`internal/emu/emu_tia_test.go`。

### 5c. `read_collisions`  ★CXxx 構造化（P1, v0.14.0）
- In: `struct{}`
- 動作: 衝突レジスタ `$30–$37`（各 D7/D6 ラッチ・sticky・CXCLR まで保持）を副作用なし peek して名前付き
  真偽ペアへデコード（`emu.ReadCollisions` / 純関数 `decodeCollisions`）。
- Out: `emu.Collisions`（`p0_p1`/`m0_m1`/`m0_p0`/`m0_p1`/`m1_p0`/`m1_p1`/`p0_pf`/`p0_bl`/`p1_pf`/`p1_bl`/
  `m0_pf`/`m0_bl`/`m1_pf`/`m1_bl`/`bl_pf`）＋ `Coords`。ビット割当は Gopher2600 `collisions.go` `tick()` 裏取り。
- 検証: D7/D6 全ペア単体テスト、無スプライトで all-false、`litmus_collide.bin`（PF全点灯＋ball）で BL-PF 陽性。

### 6. `peek` / `poke`
- `peek` In: `{ Addr uint16 }` → Out: `{ Value uint8; Coords }`（`emu.VCS.Mem.Peek`）。
- `poke` In: `{ Addr uint16; Value uint8 }` → Out: `{ Coords }`（`emu.VCS.Mem.Poke`）。

### 7. `breakif`（条件実行）
- In: `{ MaxFrames int; UntilScanline int; UntilClock int }`（最小版：ビーム位置で停止）
- 動作: `emu.VCS.RunForFrameCount(MaxFrames, continueCheck)` の `continueCheck` で
  `GetCoords()` が目標 (Scanline, Clock) に達したら `govern.Ending` を返す。
  （`govern` = `github.com/jetsetilly/gopher2600/hardware/television/...` ではなく
  `github.com/jetsetilly/gopher2600/govern`。`RunForFrameCount` の戻り state 型と同じものを import。）
- Out: `{ Coords; Halted bool }`
- 補足: RAM 値や衝突での停止は次イテレーションで拡張。まずビーム位置停止だけ通す。

### 7b. `assert_line_budget`  ★欠落B の本丸（B-3, v0.13.0）
- In: `{ MaxFrames int (default 1); Budget int (default 76 = 1 ライン分の CPU サイクル) }`
- 動作: ある論理ライン（= WSYNC ストローブ間隔）が予算を超えて余分なスキャンラインを食い込んだ瞬間に停止。
  Pong v2 を黙って殺した「per-scanline 超過 → ロール」を数値で捕まえる。`emu.RunUntilBudget`。
  - **検出**: WSYNC ストローブ = CPU `RdyFlg` の true→false 遷移（WSYNC のみが RDY を落とす）。WSYNC は次
    スキャンライン境界まで stall するので、連続ストローブ間の scanline 差 = その論理ラインが消費した物理ライン数
    （1 に収まれば OK、≥2 で超過）。`maxLines = budget/76`（既定 1）を超えたら停止。
  - 計測前に 2 フレーム空走（起動直後は VSYNC 同期が乱れ誤検知するため）。多ライン・カーネルは budget を上げる。
- Out:
  ```go
  type BudgetOut struct {
      Over       bool // true=予算超過（ロール要因）で停止
      AtScanline int  // 超過した論理ラインの開始 scanline
      LineCycles int  // そのラインが消費した概算 machine cycle（消費物理ライン数 × 76）
      Coords
  }
  ```
  - **検証**: `roms/litmus/litmus_overrun.bin`（WSYNC 前 ~100cy の重いライン 1 本）で `Over=true`・
    `LineCycles=152`。smoke / frogger は `Over=false`（誤検知なし）。`internal/emu/emu_budget_test.go`。

## 動作確認（受け入れ条件）

1. `go build ./cmd/harness` が通る。
2. stdio で JSON-RPC `initialize` → `tools/list` が 7 ツールを返す。
3. `load_rom roms/smoke.bin` → `step_frame {Count:10}` → `read_ram` で `$80` 位置が `42`。
4. `read_cpu` の Coords.Frame が 10 付近。

手動確認は `initialize`/`tools/call` の JSON を harness に標準入力でパイプするか、
MCP クライアント（Claude Code の `.mcp.json` 登録）で叩く。
