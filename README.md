# atari2600-dev

**目的:** Claude が Atari 2600 のゲームを 6502 アセンブリで「的確に」制作できる**検証ハーネス**を整えること。
ゲーム生成専用アプリではなく、Claude が毎反復で叩ける「アセンブル → 実行 → 数値で確認」のループ基盤。

## このプロジェクトの前提

- **主たる作者は Claude である。** ユーザー（プロジェクトオーナー）はアセンブリを読まない。
  したがってこの環境は、人間の可読性ではなく、**Claude の制作ループの精度と速度**を最適化する。
- 過去に Pong 等を作らせて分かったのは、失敗の原因がコード生成能力ではなく、
  **実行結果のフィードバックと、サイクル厳密なタイミングの検証手段が無いこと**だった。
- よって作るべきは「ゲーム生成専用アプリ」ではなく、**Claude が毎反復で叩ける検証ハーネス**である。

## 設計の骨格 — 5 つの欠落（A〜E）

Claude が 2600 アセンブリで失敗する構造を 5 つの欠落に分解し、各欠落を埋めるツール／資料を当てていく。
詳細は [`docs/gap-analysis.md`](docs/gap-analysis.md)。**v0.21.0 時点で A〜E すべてクローズ済み。**

| | 欠落 | 一言 | 状態 |
|---|------|------|---|
| A | 知覚 | 実行結果が見えない（数値状態が要る） | ✅ 閉 |
| B | タイミング計算 | サイクル／ビーム位置を頭で数えられない（★最重要） | ✅ 閉 |
| C | 知識 | 6502/TIA の定数・挙動を取り違える | ✅ 閉（B に従属） |
| D | 検証 | 再現性・回帰テストが無い | ✅ 閉 |
| E | 摩擦 | build→run→inspect が1コマンドになっていない | ✅ 閉 |

## アーキテクチャ

エンジン = **Gopher2600**(Go) を**ライブラリとして自プロセスに埋め込み**、薄い **Go MCP サーバ**
（`modelcontextprotocol/go-sdk` v1.6.1, stdio）で包む。`hardware`/`television`/`setup` は SDL 非依存の
純 Go なので headless 数値駆動が成立。各ツールは結果を**数値（typed JSON）**で返す。

- アセンブラ = **DASM**（`-f3`）／純 6502 サイクル = sim65・6502profiler。
- 照合オラクル = **Stella**／画像オーバーレイ = Go 内製（`image/draw` + `fogleman/gg`）。

### MCP ツール（19・`cmd/harness`）

`load_rom` / `step_frame` / `step_scanline` / `step_instruction` / `assemble_and_load` /
`read_cpu` / `read_ram` / `read_tia` / `read_tia_registers` / `read_cycles` / `read_collisions` /
`read_row` / `read_audio` / `peek` / `poke` / `breakif` / `set_input` / `assert_line_budget` /
**`get_screen_annotated`**（画像＋数値を同時返却＝ユーザー↔Claude の主要通信回線）。

実装仕様は [`docs/mcp-tools.md`](docs/mcp-tools.md)。

## 構成

```
atari2600-dev/
├── CLAUDE.md               # 毎セッション常時ロードされる開発憲法（前提・鉄則・確定定数）
├── README.md / CHANGELOG.md
├── go.mod                  # Go モジュール（ローカル Gopher2600 へ replace）
├── Gopher2600/             # 外部依存（git 管理外。下記手順で取得）
├── cmd/                    # 実行可能（システム＝全ゲームで再利用）
│   ├── harness/            #   MCP サーバ（19 ツール）
│   ├── probe/              #   配管検証 CLI
│   ├── scenario/           #   シナリオ回帰ランナー（入力タイムライン＋数値アサート）
│   └── calibrate/          #   横位置 X(N) 掃引フィット
├── internal/               # 基盤ライブラリ（game にゼロ依存）
│   ├── emu/                #   Gopher2600 駆動ラッパ（headless・数値駆動）
│   ├── annotate/           #   注釈スクショ生成
│   ├── build/              #   dasm 呼び出し
│   ├── scenario/           #   シナリオ回帰（ROM 非依存）
│   ├── calibrate/          #   位置キャリブレーション
│   └── playfield/          #   playfield エンコーダ（Atari 2600 普遍知識）
├── roms/                   # ロム成果物（game → harness の一方通行）
│   ├── litmus/             #   基盤の検証 ROM（litmus_* / smoke）
│   └── frogger/            #   Monet Frogger（+ gen/ で playfield を import してカーネル生成）
└── docs/                   # 深掘りドキュメント（ルーティングは CLAUDE.md 参照）
```

`.bin`/`bin/`/`preview/`/`Gopher2600/` は git 管理外（`.gitignore`）。次セッションは dasm / `go build` / シナリオで再生成する。

## 開発セットアップ（macOS / Apple Silicon）

```sh
brew install dasm cc65 pkg-config go                       # アセンブラ・6502シミュ・ビルド依存
brew install --cask stella                                  # 照合オラクル（任意）
git clone https://github.com/JetSetIlly/Gopher2600.git      # エンジン（repo ルートへ。git 管理外）
go mod tidy                                                  # 依存解決

dasm roms/litmus/smoke.asm -f3 -oroms/litmus/smoke.bin      # テスト ROM をアセンブル
go run ./cmd/probe                                          # 配管検証（数値出力）
go run ./roms/frogger/gen                                   # ロム生成（playfield → DASM ソース）
go run ./cmd/scenario roms/frogger/scenarios/*.json         # 回帰シナリオ（全 pass で exit 0）
go run ./cmd/calibrate                                      # 横位置較正（slope 3 px/CPU-cycle 再現）
```

`Gopher2600/` は `replace` で参照するため repo ルート直下に clone する。

### Claude Code から MCP ツールとして使う

`.mcp.json` がハーネスバイナリ（`bin/harness`）を MCP サーバとして登録している。
初回はバイナリをビルドしてから Claude Code を再起動すると、`get_screen_annotated` 等のツールが使える。

```sh
go build -o bin/harness ./cmd/harness   # .mcp.json が参照するバイナリを用意
# → Claude Code を再起動して MCP サーバ "atari2600" をロード
```

## ドキュメント

| 作業 | 読む |
|---|---|
| なぜこの設計か / 失敗の構造 | [`docs/gap-analysis.md`](docs/gap-analysis.md) |
| ツール選定理由・代替案 | [`docs/tool-landscape.md`](docs/tool-landscape.md) |
| 実装仕様・定数の出典 | [`docs/resources.md`](docs/resources.md) |
| MCP ツール実装仕様 | [`docs/mcp-tools.md`](docs/mcp-tools.md) |
| シナリオ回帰の形式 | [`docs/scenarios.md`](docs/scenarios.md) |
| litmus 実測値（横位置・HMOVE） | [`docs/litmus-results.md`](docs/litmus-results.md) |
| 改善方針・次の打ち手 | [`docs/improvement-roadmap.md`](docs/improvement-roadmap.md) |
| 決定の経緯・変更履歴 | [`CHANGELOG.md`](CHANGELOG.md) |

## 来歴

旧名 `Stella-MCP`。エンジンが Stella に限らない（Gopher2600 を採用）こと、および成果物が単一の MCP に
留まらない（環境一式である）ことから、目的ベースの名前に変更した。
