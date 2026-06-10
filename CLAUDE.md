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
- エンジン = **Gopher2600**(Go) を**ライブラリとして自プロセスに埋め込み**、薄い **Go MCP**
  （公式 `modelcontextprotocol/go-sdk` v1.6.1, stdio）で包む。`hardware`/`television`/`setup` は
  SDL 非依存の純 Go なので headless 数値駆動が成立。terminal/PushedFunction は不要だった（v0.3.0 確定）。
- 各ツールは結果を**数値（typed JSON, Coords 同梱）**で返す。画像（`get_screen_annotated`）は別格＝下記注釈スクショ。
- 回帰 = **Gopher2600 の `regress` + 録画/再生**。純 6502 のサイクル = sim65 / 6502profiler。
- 照合オラクル = **Stella**（`-sssingle -ss1x`, `-tia.dbgcolors roygbp`, `-dbg.script`+`dump`）。
- 画像オーバーレイ = **Go 内製**（`image/draw` + `fogleman/gg`）。ImageMagick へシェルアウトしない。
- アセンブラ = **DASM**（`-f3`）。**BizHawk は macOS 不可で不採用。**
- MCP ツール(**実装済**, `cmd/harness`): `load_rom` / `step_frame` / `read_cpu` / `read_ram` /
  `read_tia` / `peek` / `poke` / `breakif` / **`get_screen_annotated`**(v0.5.0, 画像＋数値を同時返却)。
  未実装(予定): `step_scanline|clock` / `watch|trap`。

## 絶対に取り違えてはいけない定数（出典: `docs/resources.md`）
**フレーム** — 1 ライン = 228 カラークロック（HBLANK 68 + 可視 160）= **76 CPU サイクル**（3 クロック/サイクル）。
NTSC **262** = VSYNC 3 / VBLANK 37 / 可視 **192** / Overscan 30。PAL・SECAM 312 = 3/45/228/36。
実ゲームは逸脱するので「厳密 262」を決め打ちせず範囲＋警告で扱う。

**ビーム座標(Gopher2600 `GetCoords`, 実機裏取り済 v0.3.0)** — `Clock` 規約は **HBLANK = −68..−1 / 可視 = 0..159**
（可視先頭ピクセル = clock 0）。スプライト `ResetPixel`/`HmovedPixel` と**同一座標系**＝直接比較可。`Scanline` は 0 起点整数。

**横位置** — ミサイル/ボール `X = 3N − 55`、**プレイヤーは +1px → `X = 3N − 54`**
（N = 同期点から RESPx ストローブまでの CPU サイクル数）。最左 X=2（プレイヤー 3）。
ズレの正体 = TIA 約 5 カラークロック遅延 + HBLANK 68。粒度 3px。粗調整は divide-by-15（5 サイクルループ）。
**litmus 実機裏取り済(v0.4.0):** 傾き 3px/CPUサイクル・粗 15px/5サイクル・160 折返し・最左 X=3 を確認。
ただし式の**オフセット定数は kernel 固有**（プロローグのサイクル数を含む）→ N の絶対値は決め打ちせず、
位置の最終判定は **`read_tia` の `HmovedPixel`**（可視 0–159 座標）で実測する。HMOVE 未発火時は `ResetPixel` と一致。

**HMOVE** — 上位ニブルのみ・2 の補数・**正=左 / 負=右**・範囲 +7〜−8。動くのは HMOVE ストローブ時のみ。HMOVE は **WSYNC 直後**。
（litmus v0.4.0 で全 16 ニブル実機裏取り: `$70`=左7 … `$00`=0 … `$F0`=右1 … `$80`=右8。1px 粒度。）

**衝突(CXxx)** — 各 D7/D6 に 2 ラッチ・sticky。`BIT CXxx` → `BMI`(D7)/`BVS`(D6)。
**CXCLR**=全衝突クリア、**HMCLR**=動きレジスタクリア（別物）。

**playfield（ビット順・実機裏取り済 v0.6.0）** — 左→右に40列、各列=4カラークロック幅。**2ソース(ABB/falukropp)一致＋`read_row`実測**。
`PF0`=上ニブルのみ col0→D4..col3→D7 ／ `PF1`=MSB先 col4→D7..col11→D0 ／ `PF2`=LSB先 col12→D0..col19→D7。
左半=clock 0–79・右半=80–159。`CTRLPF` D0: 0=repeat（右半複製）/ 1=reflect（鏡像）。検証は `read_row`（数値・目視に頼らない）。

**ハード** — RAM 128 バイト。ROM `$F000`(4K)、ベクタ `$FFFA`。
**poke の癖** — `poke` は RAM 向け。TIA 書込専用レジスタ($0D PF0 等)は poke で安定して持続しない → レンダリング変更は ROM/kernel の `sta` で。

**注釈スクショ(`get_screen_annotated`)** — Claude 専用補助ではなく**ユーザー↔Claude の主要通信回線**＝一級市民。
ユーザーが画像を見て「P0 を clock 80 へ」と視覚的にデータ指示 → Claude が register に直訳する往復ループ。
よってグリッドは **TIA 実座標で校正**（横 clock 0–159 / 縦 scanline 0–191、両軸常時）＝ユーザーの座標がそのまま
register 値へ直結すること。現在位置を**数値ラベル**で焼く。人間可読性最優先（×3–4 拡大・軸ラベル）。
画像はインライン返却に加え**毎回ファイルへ上書き保存**（env `ATARI2600_SCREEN_PATH`、既定は `.mcp.json` で
`preview/screen.png`）＝VS Code プレビューが自動リロードし往復できる。`png_path` を JSON でも返す。

## ルーティング表（作業前に読む）
| 作業 | 先に読む |
|---|---|
| なぜこの設計か / 失敗の構造 | `docs/gap-analysis.md` |
| ツール選定理由・代替案 | `docs/tool-landscape.md` |
| 実装仕様（Gopher2600 API / MCP / Stella flags）・定数の出典 | `docs/resources.md` |
| MCP ツール実装仕様（go-sdk API・各ツール I/O） | `docs/mcp-tools.md` |
| litmus 実測値（横位置・HMOVE の権威データ） | `docs/litmus-results.md` |
| 決定の経緯・変更履歴 | `CHANGELOG.md` |

## 開発環境（macOS / Apple Silicon）
`brew install dasm cc65 pkg-config go` / Stella: `brew install --cask stella`。
Gopher2600 は repo ルートへ clone（git 管理外, `go.mod` の `replace` で参照）。
ROM ビルド: `dasm x.asm -f3 -ox.bin`。配管検証: `go run ./cmd/probe`。MCP サーバ: `go build -o bin/harness ./cmd/harness`。

## バージョン管理
意味ある変更ごとに `CHANGELOG.md`（Keep a Changelog）へ追記し SemVer でタグ。決定は CHANGELOG の「決定」節に残す。
