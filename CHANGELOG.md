# Changelog

このプロジェクトの変更履歴。形式は [Keep a Changelog](https://keepachangelog.com/)、
バージョンは [Semantic Versioning](https://semver.org/) に従う。

## [Unreleased]

### 追加
- **ハーネス配管検証（Phase 2.1）成功。** Gopher2600 をライブラリとして自プロセスに
  埋め込み、完全 headless で数値駆動できることを実 ROM で確認。
  - Go モジュール `github.com/kidsnz/atari2600-dev`。ローカル Gopher2600 へ `replace`。
  - `internal/emu`: 駆動ラッパ（New/LoadROM/Coords/RunFrames/StepFrame/PeekRAM）。
  - `cmd/probe`: 数値検証 CLI。`roms/smoke.asm`（NTSC 262 ライン・RAM `$80`=sentinel `$42`）で
    `ScanlinesPF=262` / `RAM[$80]=$42` / CPU 実行（PC=F024）を確認。

### 決定
- **駆動は terminal/PushedFunction でなく `hardware.VCS` 直接埋め込み。** 実 API 調査の結果、
  `hardware`/`television`/`setup` は SDL/cgo 非依存の純 Go であり、ライブラリ埋め込みの方が
  決定的・単純・高速。研究ドキュメント（resources.md）が想定した terminal 駆動は不要だった。
- Gopher2600 は `replace => ./Gopher2600`（nightly clone）で固定。clone 自体は `.gitignore`。

### 追加予定
- 最小 MCP プロトタイプ（`load_rom` + `step_frame` + `read_cpu/ram/tia` + `breakif`）
- 横スプライト位置決め（litmus test）でハーネスの有効性を検証

## [0.2.0] - 2026-06-09

### 追加
- **macOS / Apple Silicon 環境セットアップ完了。** 全ツールの導入・疎通確認済み。
  - Go 1.26.4（arm64）
  - cc65 2.19 / sim65 V2.18（純 6502 サイクル計測層）
  - pkgconf 2.5.1（SDL2 リンク用）
  - Gopher2600 ビルド済み（`go build -tags=release .`、27MB バイナリ）
  - DASM 2.20.14.1 / Stella.app / SDL2 は前フェーズから継続

### 決定
- Gopher2600 は `go build -tags=release .` でビルド。`--version` フラグ無し、起動確認はヘルプ表示で代替。

## [0.1.0] - 2026-06-09

### 追加
- プロジェクト発足。目的を「Claude が Atari 2600 を 6502 アセンブリで的確に制作できる環境の構築」と定義。
- `docs/gap-analysis.md`: Claude が 2600 アセンブリで失敗する構造を欠落 A〜E に分解。
  過去 Pong 制作の post-mortem による実証データを反映（全放棄が「未検証のタイミング／位置決め」で死亡）。
- `docs/tool-landscape.md`: 各欠落に当てるツール／資料の地図（macOS 前提で裏取り済み）。
- `docs/resources.md`: 必要資料の棚卸し（既存／新規）＋ リサーチ結果。
  横位置の閉形式公式 `X = 3N − 55`（プレイヤー +1）とオフセットの正体（TIA 約 5 クロック遅延＋HBLANK 68）、
  HMOVE ニブル表、フレーム予算、衝突レジスタ、検証資料（Gopher2600 の録画/再生 + `regress`）、
  および Phase 1〜2 の実装仕様（Gopher2600 Go API / MCP SDK / Stella 仕様 / 画像合成）を収録。
- `README.md`, `CHANGELOG.md`。

### 決定
- **エンジンを Gopher2600 に決定。** macOS でプログラム駆動でき、CPU + color-clock（ビーム位置）単位で
  検査できる唯一の高精度 2600 エミュであるため。これを薄い Go 製 MCP で包む。
- **BizHawk を不採用。** Lua socket server は魅力だが macOS ポートが廃止されており Apple Silicon で実質不可。
- **回帰層は sim65 / 6502profiler**（純 6502 のサイクル計測・CI）、**照合は Stella**、**最優先欠落は B（タイミング）**。
- **MCP SDK は公式 `modelcontextprotocol/go-sdk`**（stdio・型付き）。設計は mcp-gameboy の「全ツールが最新フレーム画像を返す」を踏襲。
- **画像オーバーレイは Go 内製**（`image/draw` + `fogleman/gg`）。ImageMagick へのシェルアウトはしない（非決定性回避）。
- **回帰は Gopher2600 の録画/再生 + `regress`** を軸に（欠落 D の既製解）。

### 変更
- ディレクトリ名を `Stella-MCP` から `atari2600-dev` に変更。
  理由: エンジンが Stella に限定されない（Gopher2600 / BizHawk が候補）こと、
  成果物が単一の MCP に留まらず環境一式であること。
