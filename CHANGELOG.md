# Changelog

このプロジェクトの変更履歴。形式は [Keep a Changelog](https://keepachangelog.com/)、
バージョンは [Semantic Versioning](https://semver.org/) に従う。

## [Unreleased]

### 追加予定
- `get_screen_annotated`（XY グリッド注釈スクショ＝ユーザー↔Claude 通信回線。一級市民扱い）

## [0.4.1] - 2026-06-09

### 変更
- **核心定数を CLAUDE.md へ蒸留（Phase 4）。** 実機検証で確定した事実を常時文脈に焼いた：
  - ビーム座標規約 `GetCoords().Clock` = HBLANK `−68..−1` / 可視 `0..159`（新規・重要定数）。
  - 横位置: litmus 裏取り（3px/サイクル・粗 15px・160 折返し・最左 X=3）＋ オフセットは kernel 固有、
    最終判定は `read_tia.HmovedPixel`（可視 0–159）で実測する旨。
  - HMOVE 表に「全 16 ニブル実機裏取り済」を明記。
  - 確定アーキテクチャ: 駆動はライブラリ埋め込み（terminal 不要）、MCP 8 ツール実装済みへ更新。
  - 注釈スクショを**ユーザー↔Claude の主要通信回線・一級市民**として再定義（TIA 実座標校正・数値ラベル・人間可読性優先）。
  - ルーティング表に `docs/mcp-tools.md` / `docs/litmus-results.md`、開発環境を実態（go/pkg-config・clone+replace）へ修正。

## [0.4.0] - 2026-06-09

### 追加
- **litmus test 完全合格（Phase 3）。ハーネスが本物であることを数値で実証。** 鉄則#4 達成。
  - 粗調整（`roms/litmus_pos.asm`）: `$80`(DELAY) スイープで 1 ループ=5 CPU サイクル=**15px**、
    DELAY 3〜11 で完全線形（`ResetPixel = 15·DELAY − 18`）、可視幅 **160 折返し**、最左クランプ **X=3**。
  - 微調整（`roms/litmus_hmove.asm`）: HMP0 ニブル全 16 値スイープで `HmovedPixel` の変位が
    **CLAUDE.md の HMOVE 表と完全一致**（2の補数・正=左/負=右・範囲 +7〜−8、1px 粒度）。
  - 粗 15px ＋ 微 1px で**任意 X を数値的に予測・配置・検証可能**に。測定値は `docs/litmus-results.md`。
  - **過去 Pong の失敗 #1（魔法定数総当たり）・#3（位置決め破綻）= 欠落 B を解毒。**

## [0.3.0] - 2026-06-09

### 追加
- **ハーネス配管検証（Phase 2.1）成功。** Gopher2600 をライブラリとして自プロセスに
  埋め込み、完全 headless で数値駆動できることを実 ROM で確認。
  - Go モジュール `github.com/kidsnz/atari2600-dev`。ローカル Gopher2600 へ `replace`。
  - `internal/emu`: 駆動ラッパ（New/LoadROM/Coords/RunFrames/StepFrame/PeekRAM/Poke/RunUntilBeam）。
  - `cmd/probe`: 数値検証 CLI。`roms/smoke.asm`（NTSC 262 ライン・RAM `$80`=sentinel `$42`）で
    `ScanlinesPF=262` / `RAM[$80]=$42` / CPU 実行（PC=F024）を確認。
- **最小 MCP プロトタイプ（Phase 2.2）動作。** `cmd/harness` が stdio で 8 ツールを露出し、
  JSON-RPC 疎通を数値で確認。`load_rom`→`step_frame`→`read_ram` で `$80`=`$42`、
  `read_cpu` で PC=`$F024`（probe と一致）。
  - ツール: `load_rom` / `step_frame` / `read_cpu` / `read_ram` / `read_tia` / `peek` / `poke` / `breakif`。
  - `read_tia` は `Video.PlayerN.ResetPixel/HmovedPixel` を露出（横位置 litmus の判定値）。
  - 公式 `modelcontextprotocol/go-sdk` v1.6.1。typed Out で JSON Schema 自動生成。
  - 実装仕様 `docs/mcp-tools.md`（全 API 裏取り済みのレシピ）。

### 決定
- **駆動は terminal/PushedFunction でなく `hardware.VCS` 直接埋め込み。** 実 API 調査の結果、
  `hardware`/`television`/`setup` は SDL/cgo 非依存の純 Go であり、ライブラリ埋め込みの方が
  決定的・単純・高速。研究ドキュメント（resources.md）が想定した terminal 駆動は不要だった。
- Gopher2600 は `replace => ./Gopher2600`（nightly clone）で固定。clone 自体は `.gitignore`。
- **★ビーム clock 座標規約を実機で確定:** `GetCoords().Clock` は HBLANK=`−68..−1` / 可視=`0..159`
  （可視先頭ピクセル = clock 0）。spec の暫定記述「0–227」は誤りだった。スプライト `HmovedPixel`
  と同座標系なので litmus test で直接比較できる。→ Phase 4 で CLAUDE.md へ蒸留。
- 任意引数は json タグ `,omitempty` で optional 化（jsonschema-go は omitempty/omitzero を任意扱い）。

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
