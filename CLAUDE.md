# CLAUDE.md — atari2600-dev

このファイルは**毎セッション自動で全文ロードされる唯一の常時文脈**。ここには「不変の前提・確定した決定・
絶対に取り違えてはいけない定数・どの作業でどの doc を読むか」だけを置く。深掘りは `docs/`（下のルーティング表）。
ここに無いものは読まれていないと思え。常に成り立つべき事実は doc に"だけ"置かず、ここかメモリに焼く。

## 不変の前提
- 目的: Claude が Atari 2600 を 6502 アセンブリで的確に制作できる**検証ハーネス**を作る（ゲーム生成専用アプリではない）。
- **主たる作者は Claude。** ユーザーはアセンブリを読まない。環境は Claude の制作ループの精度・速度を最適化する。
- **最優先は欠落 B（タイミング）。** 過去の Pong は全放棄が「未検証のタイミング／位置決め」で死んだ。

## 鉄則（毎回守る）
1. **判定は数値。スクショは補助。** 横位置の最終判定は TIA レジスタ値、縦は scanline 整数。目視のピクセル数えで決めない。
2. **サイクルはシミュレータから取る**（Gopher2600 / sim65）。DASM のリストにも頭の暗算にも頼らない。
3. **小ステップ。** 編集→アセンブル→実行→数値確認→commit。失敗したら前ステップへ revert。一括変更しない。
4. **litmus test:** スプライトを任意 X に置く／1px 動かすが `X = 3N − 55` どおりに通ること。これが通れば環境は本物。

## 確定アーキテクチャ
- エンジン = **Gopher2600**(Go)。薄い **Go MCP**（公式 `modelcontextprotocol/go-sdk`, stdio）で包む。
  設計は mcp-gameboy の「**全ツールが最新フレーム画像を返す**（やったこと＝結果の観測を一体化）」を踏襲。
- 回帰 = **Gopher2600 の `regress` + 録画/再生**。純 6502 のサイクル = sim65 / 6502profiler。
- 照合オラクル & 注釈スクショ = **Stella**（`-sssingle -ss1x`, `-tia.dbgcolors roygbp`, `-dbg.script`+`dump`）。
- 画像オーバーレイ = **Go 内製**（`image/draw` + `fogleman/gg`）。ImageMagick へシェルアウトしない。
- アセンブラ = **DASM**（`-f3`）。**BizHawk は macOS 不可で不採用。**
- MCP ツール(予定): `load_rom` / `step`(clock|scanline|frame) / `read_cpu|ram|tia|riot` / `peek|poke` /
  `breakif|watch|trap` / `get_screen_annotated`。

## 絶対に取り違えてはいけない定数（出典: `docs/resources.md`）
**フレーム** — 1 ライン = 228 カラークロック（HBLANK 68 + 可視 160）= **76 CPU サイクル**（3 クロック/サイクル）。
NTSC **262** = VSYNC 3 / VBLANK 37 / 可視 **192** / Overscan 30。PAL・SECAM 312 = 3/45/228/36。
実ゲームは逸脱するので「厳密 262」を決め打ちせず範囲＋警告で扱う。

**横位置** — ミサイル/ボール `X = 3N − 55`、**プレイヤーは +1px → `X = 3N − 54`**
（N = 同期点から RESPx ストローブまでの CPU サイクル数）。最左 X=2（プレイヤー 3）。
ズレの正体 = TIA 約 5 カラークロック遅延 + HBLANK 68。粒度 3px。粗調整は divide-by-15（5 サイクルループ）。

**HMOVE** — 上位ニブルのみ・2 の補数・**正=左 / 負=右**・範囲 +7〜−8。動くのは HMOVE ストローブ時のみ。HMOVE は **WSYNC 直後**。

**衝突(CXxx)** — 各 D7/D6 に 2 ラッチ・sticky。`BIT CXxx` → `BMI`(D7)/`BVS`(D6)。
**CXCLR**=全衝突クリア、**HMCLR**=動きレジスタクリア（別物）。

**ハード** — RAM 128 バイト。ROM `$F000`(4K)、ベクタ `$FFFA`。

**注釈スクショ** — グリッドは **XY 二次元**（横 0–159 カラークロック / 縦 0–191 scanline）。X 専用ではない、両軸常時。

## ルーティング表（作業前に読む）
| 作業 | 先に読む |
|---|---|
| なぜこの設計か / 失敗の構造 | `docs/gap-analysis.md` |
| ツール選定理由・代替案 | `docs/tool-landscape.md` |
| 実装仕様（Gopher2600 API / MCP / Stella flags）・定数の出典 | `docs/resources.md` |
| 決定の経緯・変更履歴 | `CHANGELOG.md` |

## 開発環境（macOS / Apple Silicon）
`brew install dasm cc65 sdl2` / Gopher2600: `go build -tags=release .` / Stella: `brew install --cask stella`。
ビルド: `dasm x.asm -f3 -ox.bin`。

## バージョン管理
意味ある変更ごとに `CHANGELOG.md`（Keep a Changelog）へ追記し SemVer でタグ。決定は CHANGELOG の「決定」節に残す。
