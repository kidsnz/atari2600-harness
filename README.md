# atari2600-dev

**目的:** Claude が Atari 2600 のゲームを 6502 アセンブリで「的確に」制作できる開発環境を整えること。

## このプロジェクトの前提

- **主たる作者は Claude である。** ユーザー（プロジェクトオーナー）はアセンブリを読まない。
  したがってこの環境は、人間の可読性・操作性ではなく、**Claude の制作ループの精度と速度**を最適化する。
- 過去に Pong 等を Claude に作らせて分かったのは、失敗の原因がコード生成能力ではなく、
  **実行結果のフィードバックと、サイクル厳密なタイミングの検証手段が無いこと**だった。
- よって作るべきは「ゲーム生成専用アプリ」ではなく、**Claude が毎反復で叩ける検証ハーネス**である。

## 設計の骨格

Claude が 2600 アセンブリで失敗する構造を 5 つの欠落（A〜E）に分解し、各欠落を埋めるツール／資料を当てていく。詳細は [`docs/gap-analysis.md`](docs/gap-analysis.md)。

| | 欠落 | 一言 |
|---|------|------|
| A | 知覚 | 実行結果が見えない（数値状態が要る） |
| B | タイミング計算 | サイクル／ビーム位置を頭で数えられない（★最重要） |
| C | 知識 | 6502/TIA の定数・挙動を取り違える |
| D | 検証 | 再現性・回帰テストが無い |
| E | 摩擦 | build→run→inspect が1コマンドになっていない |

ツール候補とマッピングは [`docs/tool-landscape.md`](docs/tool-landscape.md)、必要資料の棚卸しと実装仕様・リサーチ結果は [`docs/resources.md`](docs/resources.md)。

## 構成

```
atari2600-dev/
├── CLAUDE.md               # 毎セッション常時ロードされる開発憲法（前提・鉄則・確定定数）
├── README.md
├── CHANGELOG.md
├── go.mod                  # Go モジュール（ローカル Gopher2600 へ replace）
├── Gopher2600/             # 外部依存（git 管理外。下記手順で取得）
├── internal/
│   └── emu/                # Gopher2600 駆動ラッパ（headless・数値駆動）
├── cmd/
│   └── probe/              # 配管検証 CLI（数値で状態を確認）
├── roms/                   # テスト ROM（.asm をコミット、.bin は git 管理外）
└── docs/
    ├── gap-analysis.md      # 欠落A〜Eの分析（この環境の仕様の土台）
    ├── tool-landscape.md    # 欠落に当てるツール／資料の地図
    └── resources.md         # 必要資料の棚卸し＋実装仕様・リサーチ結果
```

## 開発セットアップ（macOS / Apple Silicon）

```sh
brew install dasm cc65 pkg-config go        # アセンブラ・6502シミュ・ビルド依存
git clone https://github.com/JetSetIlly/Gopher2600.git   # エンジン（repo ルートへ）
go mod tidy                                  # 依存解決
dasm roms/smoke.asm -f3 -oroms/smoke.bin     # テスト ROM をアセンブル
go run ./cmd/probe                           # 配管検証（数値出力）
```

`Gopher2600/` は `replace` で参照するため repo ルート直下に clone する（git 管理外）。

## 来歴

旧名 `Stella-MCP`。エンジンが Stella に限らない（Gopher2600 / BizHawk 等が候補）こと、
および成果物が単一の MCP に留まらない（環境一式である）ことから、目的ベースの名前に変更した。
