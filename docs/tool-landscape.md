# Tool Landscape — 各欠落に当てるツール／資料の地図

[`gap-analysis.md`](gap-analysis.md) の欠落 A〜E に、ツールと資料をマッピングする。
裏取り済み（2026-06-09、macOS / Apple Silicon 前提）。

## 欠落の凡例
A=実行結果が見えない / B=サイクル・ビーム位置が数えられない / C=知識 / D=回帰・再現性 / E=反復摩擦

---

## 比較表（裏取り済み）

| ツール | 埋める欠落 | headless/スクリプト | MCP化 | macOS 導入 | ライセンス |
|--------|:---:|------|:---:|------|------|
| **Gopher2600** | A B C E | （実装は v0.3.0 で確定＝**ライブラリ埋め込み**。terminal/`PushedFunction` は不要だった） | **◎ 採用** | `brew install sdl2 pkg-config` → `go install` | GPL-3.0 |
| **Stella** | A B C E | `exec`/`autoexec`/`-dbg.script` + `dump` をファイルへ。**socket無し・headless描画無し** | △ スクリプト+ファイル経由のみ | `brew install --cask stella` | GPL-2.0 |
| **BizHawk** | A B C D | Lua socket server あり | ✕ **macOS不可** | **廃止（Apple Silicon実質不可）** | MIT(混在) |
| **8bitworkshop** | A C E | ブラウザ(Javatari)。`make tsweb`でlocal | ✕ | clone + node | GPL-3.0 |
| **sim65** (cc65) | B E | CLI、`-c`で実行サイクル出力 | ○ ラップ可 | `brew install cc65` | zlib系 |
| **6502profiler** | B D E | CLI、サイクル計測 + Lua テスト | ○ ラップ可 | `go install` | OSS |
| **6502_test_executor** | B D | CLI、JSONテスト、サイクル数アサート | ○ ラップ可 | clone + make | OSS |
| **sim6502** (barryw) | B D E | CLI、決定的 + VICE バックエンド | △ | .NET build | OSS |
| **DASM** | C E | アセンブラ。`-l`でリスト（**サイクル注記は無し**） | n/a | `brew install dasm` | GPL-2.0 |
| **Atari Dev Studio** | C E | dasm+Stella+batari 同梱。VS Code タスク駆動 | ✕ IDE結合 | VS Code 拡張 | OSS |

---

## A 知覚 / B タイミング — エミュレータ

### Gopher2600（本命エンジン）
macOS でプログラム駆動できる唯一の高精度 2600 エミュ。6507/TIA/RIOT 高精度。
**CPU と color-clock（ビーム位置）単位で状態を覗け・巻き戻せる** → racing-the-beam（欠落 B）に直撃。
Go パッケージ `debugger/terminal` が **`PushedFunction`**（自前 Go プロセスから debugger goroutine にコマンドを流す）を公開。
terminal は stdin パイプ可、`debuggerInit` で起動時スクリプト。
→ **薄い Go 製 MCP サーバで包む**のが最善。露出案: `load_rom` / `step_frame` / `step_scanline` / `step_cycle` /
`read_cpu` / `read_ram` / `read_tia` / `breakif` / `get_screen`(フレームバッファ→画像)。

### Stella（人間用ビジュアル + 照合オラクル）
**socket・headless 描画ともに無い**ことが確定。外部制御は「`-dbg.script` で `dump` をファイルに吐く→読む」の
スナップショット型のみ。ライブ MCP エンジンには不向き。
役割は (1) 人間（＝pizza boy 流）のビジュアルデバッグ、(2) Gopher2600 の結果を突き合わせる**精度の最終審判**。

### BizHawk（macOS では不採用）
Atari2600Hawk コア + Lua socket server は強力だが、**macOS ポートは廃止**（64bit WinForms 非対応）。
Apple Silicon では実質使えない。**この環境では選ばない。**

---

## B タイミング / D 回帰 — 純 6502 シミュ・テスト（TIA 非対応・CIロジック検証用）

カーネルのタイミング計算と純ロジックの回帰を、決定的・高速に回す層。いずれも TIA は持たないので、
**分離可能な 6502 ルーチンのサイクル計算とユニットテスト**に使う（2600 全体の検証は Gopher2600）。

- **sim65**（cc65 同梱）— `-c` で実行サイクル数を出力。`brew install cc65` で即。最軽量。
- **6502profiler** — クロックサイクル計測 + Lua でテストの arrange/assert。Go ビルド。
- **6502_test_executor** — JSON テスト、サイクル数アサート、cc65 ベース。
- **sim6502**（barryw）— 決定的 + VICE サイクル正確の2バックエンド。Commodore 寄り。
- **Klaus2m5 functional tests** — 6502 挙動の正しさのゴールデン基準（参照）。

> **重要:** DASM のリストファイルは**サイクル数を注記しない**（行番号/アドレス/バイト/ソースのみ）。
> サイクルは必ずシミュレータ（sim65 / 6502profiler / Gopher2600）から取る。

---

## C 知識 — 参照資料（CLAUDE.md へ蒸留する原典）

**既にユーザーが `260304_Claude-Code-Pong/docs_atari/` にほぼ収集済み**（→ 欠落 C は収集ではなく蒸留が課題）。

- **Stella Programmer's Guide**（Steve Wright, 1979）`stella_programmers_guide.{html,pdf}` — TIA の聖典
- **Guide to Cycle Counting**（Nick Bensema）`cycle_counting_guide.html` — ★B/C の核心
- **Programming for Newbies**（Andrew Davie）`Atari_2600_Programming_for_Newbies.{pdf,txt}` — 特に Session 22（横位置）
- **woodgrain wiki** `Playfield_Timing.html` / `Clock_Speeds.html` / `Memory_Map.html` / `Bank_Switching.html` / `Sound.html`
- **vcs.h / macro.h** — TIA レジスタ名定義
- **横位置の正解** `8bitworkshop_samples/sethorizpos.asm`（divide-by-15 ルーチン）
- **実ゲーム逆アセンブル** `game_disassembly/`（adventure, pitfall, kaboom 他 21 本）, `za2600/`（Zelda 再現）
- **サンプル** `8bitworkshop_samples/`, `nanochess_samples/`, `spiceware_tutorial/`
- **6502 リファレンス** `6502_reference.md`, `vcs_reference.md`, `tia_colors_ntsc.md`, `2600_music_guide.txt`

---

## E 摩擦 — 反復低減 / 足場

- **DASM** — 標準アセンブラ。`brew install dasm`。build: `dasm x.asm -f3 -ox.bin`
- **Atari Dev Studio**（VS Code）— dasm+Stella+batari 同梱。MCP には不向きだが**正しい macOS バイナリの入手元**として有用
- **batari Basic** — カーネルのタイミングを肩代わり。純 asm が詰まった時の足場/比較対象

---

## 既存の MCP / ハーネス（prior art）

- **mcp-gameboy**（mario-andreschak）— TS の MCP が `serverboy` を包む。`load_rom` / 入力 / `get_screen`→`ImageContent`。
  **2600 MCP の設計はこれを踏襲**（画面を画像で返す＝欠落 A）。
- **vice-mcp / ViceMCP**（barryw）— VICE の C コアに MCP を埋め込み 63 ツール（ブレーク/ステップ/レジスタ/メモリ/VIC-II/SID/スクショ）。
  **macOS でビルド可**。「エミュ＋埋込 MCP を Claude が完全駆動」が成立する最良の実証（ただし Commodore）。
- **CTalkobt/sim6502** — Node MCP（assemble/step/reg/mem/breakpoint）。TIA 非対応・現状プロプライエタリ。
- **Atari 2600 専用 MCP は存在しない。**（`bradleylab/stella-mcp` は無関係＝System Dynamics モデリング用）
  → **ここが空き地＝このプロジェクトの新規性。**

---

## 確定したアーキテクチャ

```
[ Claude Code ]
   │  MCP
   ▼
[ Gopher2600-backed MCP server (Go) ]   ← A/B/C/E: load_rom, step_*, read_cpu/ram/tia, breakif, get_screen
   │
   ├─ DASM (brew)                         ← アセンブル
   ├─ sim65 / 6502profiler                ← B/D: 分離ロジックのサイクル計測・回帰テスト(CI)
   └─ Stella (brew cask)                  ← 照合オラクル + 人間のビジュアル確認(-dbg.script + dump)
```

- **エンジン = Gopher2600**（macOS でビーム単位に駆動できる唯一格）。BizHawk は macOS 不可で不採用。
- **回帰層 = sim65 / 6502profiler**（純 6502 のサイクル計算と CI）。
- **照合 = Stella**（live MCP には使わない。最終精度確認と人間用）。
- **新規性:** 2600/TIA を理解する MCP は世に無い。設計は mcp-gameboy、形の実証は vice-mcp に倣う。
