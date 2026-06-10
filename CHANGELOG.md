# Changelog

このプロジェクトの変更履歴。形式は [Keep a Changelog](https://keepachangelog.com/)、
バージョンは [Semantic Versioning](https://semver.org/) に従う。

## [Unreleased]

### 追加
- **改善ロードマップ文書（`docs/improvement-roadmap.md`）。** 「制作をさらに正確にする」次の打ち手を
  あらゆる角度から優先度付きで整理。中心的所見＝**位置の litmus は閉じたが、タイミング*予算*の検証は
  開いたまま**（欠落B が実制作ループでは最大の穴）。P0=サイクル露出＋per-scanline 予算ガード、
  P1=TIA シャドウ／衝突レジスタ読み、P2=検証自動化、P3=ビルドループ短縮。各項目に検証済み
  Gopher2600 API シンボル（`CPU.LastResult.Cycles`・`TIA.Video.*`・`Collisions`）を併記。
  ルーティング表にも追加。実装は伴わないため tag は打たない。
  - **「参照資料の未採掘脈」節を追記。** 旧 Pong の `docs_atari` トローブ（`tool-landscape.md` で
    カタログ済）を Frogger 段階で再評価。R-1 Freeway アーキテクチャ移植（lane/複数オブジェクト/lane単位衝突の
    実証済み設計）・R-2 音声レシピ（Slocum ガイド＋`za2600/audio.asm`、音声を「範囲外」→「着手可能」へ格上げ、
    `TIA.Audio` シャドウ読みを P1 に同梱提案）・R-3 サイクルコスト表（Bensema、書く側の予測を補強）・
    R-4 実ゲーム構造の索引化。新規発見ではなく未採掘脈の採掘という位置づけ。
  - **「外部リサーチ（GitHub/web）」節を追記。** 最大の発見＝**埋め込み済 Gopher2600 自体に最難項目が
    ライブラリ実装済**（debugger driver は外したまま `recorder`/`regression`/`tracker`/`reflection`/`digest`/
    `rewind`(deeppoke) を単体利用可。P2/R-2 が「作る」→「配線する」に縮小。exported API 確認済）。
    `debugger/halt_*` は unexported でパターン参照どまり、License=GPL-3.0 も明記。G-2 C64 MCP 群
    （vice-mcp 等。**2600 は皆無＝我々が最初**）と sim6502 の pluggable backend DSL、G-3 テスト DSL 先行例
    （64spec/sim65/Klaus2m5）、G-4 オーサリングツール連携（PlayerPal、注釈の paint→register 化）。

### 追加予定
- 実ゲーム制作（ハーネスを使った本番。Pong 再挑戦など）
- `step_scanline|clock` / `watch|trap` ツールの拡充
- **【TODO】制作システム（ハーネス）の独立プロジェクト化。** ハーネスは特定ロムに依存せず汎用の制作・検証
  システムとして育てる方針。北極星ロム（Monet Frogger）が一段落したら、`internal/emu`・`cmd/harness`・MCP
  ツール群・`internal/playfield` 等を**独立リポジトリ/プロジェクトへ切り出す**。ロム制作物（`roms/`・各シーン
  generator）と制作基盤を分離し、基盤側を単体で進化させる。

## [0.11.0] - 2026-06-10

### 変更
- **モノレポ再編：root=ハーネス基盤 / `roms/<game>/`=ロム（独立化 Phase 1）。** 依存が game→harness の
  一方通行（harness は game にゼロ依存）であることを実証し、外科手術なしで分離。
  - **共通(root)**: `cmd/harness`・`cmd/probe`・`internal/emu`・`internal/annotate`・`internal/playfield/playfield.go`
    （汎用エンコーダ `EncodeSymmetric` 等）。
  - **ゲーム固有**: `cmd/genpf` ＋ `internal/playfield/asmgen.go`（カーネル生成）を **`roms/frogger/gen/`（package main、
    `playfield` を import）** へ統合移動。`roms/frogger/*.asm`、検証用は `roms/litmus/`。
  - `cmd/probe` の既定 ROM・`.gitignore`（`/roms/**/*.bin` 等）・CLAUDE.md 構成節・roadmap のパス参照を更新。
  - **検証:** `go build/test/vet ./...` 全グリーン、`go run ./roms/frogger/gen frogger` で再生成、全 15 ROM を
    新パスでアセンブル、`litmus_pf` を新パスからロード→read_row が再編前と完全一致（基盤無傷）。
  - 今後のゲームは `roms/<name>/`(＋`gen/`)の規約。次フェーズ＝望む場所への移動（要メモリ移行＋再起動）。

## [0.10.1] - 2026-06-10

### 追加
- **Frogger の磨き（A）。** 自プレイで検証。
  - **ゲームオーバー/リスタート:** Lives が 0 になったら全リスタート（Lives=3・Score=0・start へ）。
    検証: poke で Lives=1/Score=5 にして溺れる → Lives=3・Score=0 にリセットを確認。
  - **視覚ゾーン:** 上 28 scanline=ゴール帯（金）/ 下 148-191=スタート岸（緑）/ 中=Monet 水（川）。
    Frogger の構造（岸→川→ゴール）が一目で読めるように。`cmd/genpf` の BG 生成で zone 分け。

## [0.10.0] - 2026-06-10

### 追加
- **🎉 遊べる Monet Frogger 完成（M5）。** Monet 水面の上で、流れる睡蓮を足場にカエルが川を渡る。
  Claude が `set_input`＋`peek/read_tia` で**自分でプレイして全メカニクスを数値裏取り**。
  - `internal/playfield.GenerateFroggerASM`: 状態機械で ride/drown/win/lives を処理する完全な game kernel。
    水面(per-scanline COLUBK)＋睡蓮(player0/NUSIZ/HMP0 ドリフト)＋カエル(player1/可変 FrogY)。衝突は CXPPMM。
    `cmd/genpf frogger` → `roms/frogger.asm`。RAM: FrogY/Lives/Score/OnPad/PrevY を peek で観測可能。
  - **検証（自プレイ）:** 川で葉を外す→Lives 3→2・start へ（drown）／葉に着地→OnPad=128・frog が葉と +1px/frame
    で流れる・無死（ride）／上端到達→Score 0→1・start へ（win）。
  - **自プレイでバグ発見・修正:** 「川に入った瞬間、前フレーム(川の手前)の衝突=0 で即溺れ→絶対渡れない」致命的
    タイミングバグを、自分でプレイして発見。着地 1 フレームの猶予（`PrevY`）で修正。

## [0.9.3] - 2026-06-09

### 追加
- **カエルの縦ジャンプ（hop）。Frogger 4 大メカニクスが揃う。** `roms/frog_hop.asm`: player0 を可変 scanline
  `FrogY`（RAM）に描画。SWCHA の up(D4)/down(D5) を**エッジ検出**し、押した瞬間に FrogY を ±16（1レーン）
  離散ジャンプ。押しっぱなしは連射しない（離す→押すで次の hop）。
  - **検証:** set_input で up 押下→FrogY 92→76、保持で 76 のまま（連射せず）、離して再押下で →60、down で →76。
    peek($81)で数値確認。`set_input`＋`peek/read_tia/read_row/screenshot` で Claude が自分で操作・観測・判定する
    閉ループが成立（ヘッドレスで遊んで裏取りできる）。
  - 補足: エッジ検出入力は「離す→1フレーム→押す」と、間にフレームを挟んで遷移を観測させる必要がある。

## [0.9.2] - 2026-06-09

### 追加
- **衝突判定（Frogger の核：カエルが葉の上か水中か）。** `GenerateMonetFullASM` に毎フレーム `CXCLR`
  ストローブを追加（衝突ラッチを毎フレーム新規取得）。衝突は `peek $37`（CXPPMM）で読める＝新ツール不要。
  - `cmd/genpf collide` → `roms/monet_collide.asm`（カエルを睡蓮レーンに置いた検証シーン）。
  - **検証:** frog をレーン内で動かし、葉と重なった frame33 で CXPPMM=128（D7=P0-P1 衝突）、離れた frame45 で
    0。CXCLR が効き毎フレーム正しく set/clear。CXPPMM セット=葉に乗っている / クリア=水中、の判定が成立。
  - 本番 `roms/monet_full.asm` も CXCLR 入りで再生成。

## [0.9.1] - 2026-06-09

### 追加
- **フルシーン統合（Frogger の舞台）。** Monet 水面（per-scanline COLUBK）の上に、流れる睡蓮(player0)と
  操作できるカエル(player1)を同居。
  - `internal/playfield.GenerateMonetFullASM`: per-scanline で GRP0/GRP1/COLUBK を各 1 バイト読む2スプライト
    版。player0=睡蓮（NUSIZ コピー・HMP0 で一定ドリフト）、player1=カエル（SWCHA を読んで HMP1）。HMOVE 1 回で
    両者に別々の動きを適用。`cmd/genpf full` → `roms/monet_full.asm`。
  - **検証:** frame5→13 でカエル 111→125（+2px/f, 右入力中）/ 睡蓮 3→11（+1px/f, 自動ドリフト）。
    1 画面で水・流れる足場・操作キャラの3要素が独立に動くことを数値実証。

## [0.9.0] - 2026-06-09

### 追加
- **`set_input` ツール＝ジョイスティック入力注入（Frogger 操作系の土台）。** ヘッドレスで入力を与える経路。
  `poke` は入力に効かない（RIOT ポートが毎フレーム駆動するため SWCHA が $FF に戻る）ことが判明 → 正攻法として
  Gopher2600 の `Ports.HandleInputEvent` 経由で注入。`internal/emu.SetInput(player, action, pressed)`。
  action=left/right/up/down/fire/center、pressed で押下保持/解除（次に変えるまで持続）。
- **カエル操作の検証 ROM（`roms/frog_control.asm`）。** player0 をカエルとして表示、SWCHA を読んで HMOVE で
  左右移動。**検証:** set_input 右 hold で 21→39（+2px/frame）、center で停止、左 hold で 41→23（−2px/frame）。
  「入力→カエルが動く」をヘッドレスで数値実証。

### 修正
- `set_input` の jsonschema タグが `0=…`/`true=…` で始まり go-sdk が WORD= タグと誤認して AddTool でパニック →
  タグ文言を修正（起動時パニックの回避）。サーバ版 0.6.0→0.9.0。

## [0.8.1] - 2026-06-09

### 追加
- **Monet 水面＋流れる睡蓮スプライトの統合（M3 ステップ2）。** 背景(per-scanline 色帯)と前景(HMOVE で
  流れるスプライト)を同居。
  - `internal/playfield.GenerateMonetSpriteASM`: per-scanline COLUBK(水)＋per-scanline GRP0(睡蓮)を
    各 scanline で 1 バイトずつテーブルから読む＝両方 HBLANK 内に確定し、サイクル臨界を回避。
    `NUSIZ0` でコピー数、`HMP0` を毎フレーム HMOVE strobe で drift。
  - `cmd/genpf sprite` → `roms/monet_sprite.asm`（水=Monet 勾配を192に展開, 睡蓮=scanline88-95, 3コピー）。
  - **検証:** HmovedPixel が frame5=133→frame17=145＝統合カーネルでも +1px/frame。画面は水の色帯の上を
    緑の睡蓮3つが流れる。Monet の2本柱（色帯の水＋滑らかに流れる足場）が合体。

## [0.8.0] - 2026-06-09

### 追加
- **M2/M3 アニメーション着手。** 2 つの動きの土台を実機裏取り。
  - **per-frame 色テーブル・アニメ（`GenerateAsymmetricShimmerASM`）。** 水色テーブルを RAM 化し毎フレーム
    VBLANK でスクロール再充填。VBLANK/Overscan を **TIM64T タイマー方式**へ（計算量に依存せずライン安定）。
    検証: scanline100 の水色が frame 9/19/29 で青→藤→緑と循環。可視ループ(サイクル臨界)は不変。
    ※ ただし「テーブル剛体シフト＝色帯が一方向に流れる」見た目は Monet の狙いと違うと判断（保留。将来は
    その場明滅=twinkle / 前後ゆらぎ=sway で作り直す）。`cmd/genpf anim` → `roms/monet_anim.asm`。
  - **スプライトの滑らかな横移動＝水流（`roms/sprite_flow.asm`, M3 ステップ1）。** player0 を睡蓮パッドとして
    表示、`NUSIZ0=$03` で 3 コピー、`HMP0=$F0` を毎フレーム HMOVE strobe＝**累積で +1px/frame ドリフト**。
    検証: HmovedPixel が frame5=133→frame15=143＝ちょうど +1px/frame の連続移動。HMOVE は VBLANK 内で撃ち
    comb を可視域に出さない。**2600 で滑らかな横移動はスプライト(HMOVE)の仕事**＝playfield 横スクロール（粗い
    事前計算フェーズ）を避けた正攻法。Frogger の足場・カエル移動の土台。
  - `cmd/genpf`: `buildMonetScene()` で静止/アニメの Monet シーンを共有化。`SceneOpts.Speed` 追加。

## [0.7.2] - 2026-06-09

### 変更
- **Monet 静止画(M1)を非対称版へ格上げ（有機的な睡蓮）。** 左右独立 playfield ＋ per-row 水色帯。
  - `GenerateAsymmetricASM` を **per-row 水(COLUBK)＋定数睡蓮(COLUPF)** へ改良（シグネチャ
    `(art, water []byte, lily byte, opts)`）。非対称ループは予算が無く per-row 色は1チャンネルのみ
    → 面積の大きい水を色帯に回す（Monet の横反射に合致）。タイミングは ABB 転写のまま不変。
  - `cmd/genpf`: デフォルト=非対称 Monet（40列・有機配置・中央またぎ可）、`asym`=非対称 litmus。
  - **検証:** read_row(scanline58) で緑パッド clock132-151（右半 cols33-37）のみ＝設計一致・片側のみ＝
    非対称確定。水の横色帯（青/シアン/マゼンタ/紫/緑）の上に睡蓮が左右非対称で浮かぶ。
  - トレードオフ記録: per-row 色1チャンネル制約で当面ピンクの花は不可（将来 2-line kernel で両立余地）。

## [0.7.1] - 2026-06-09

### 追加
- **非対称(左右独立) playfield 能力を獲得・実機裏取り（アニメ前の検証）。** 有機的・非対称な水面/睡蓮に必要。
  - `internal/playfield.GenerateAsymmetricASM`: 1 ライン内で PF0/1/2 を A(左)→B(右) に詰め替える
    ABB(kirkjerk) の「repeated」非対称カーネルを逐語転写（72サイクル/ライン、`tay`/`sty` で間合い）。
    `EncodeAsymmetric` のビット配置が ABB repeated と一致することを利用。
  - `cmd/genpf asym`: 幅4ブロックが40列を上→下に掃引する対角ストライプ検証シーン → `roms/asym_test.asm`。
  - **検証:** read_row(scanline20)=白 clock8-23（左半のみ）／(scanline172)=白 clock120-135（右半のみ）。
    片側だけ点灯＝reflect では不可能＝左右独立描画の決定的証明。画面も中心で折れない 1 本の対角。

## [0.7.0] - 2026-06-09

### 追加
- **M1「静かな池」達成＝描画パイプライン端から端まで開通。** 北極星 ROM「Monet 睡蓮 Frogger」の最初の
  マイルストーン。`ASCIIアート＋色設計 → EncodeSymmetric → asmgen(kernel) → dasm → load_rom → read_row照合`。
  - `internal/playfield/asmgen.go`: `GenerateSymmetricASM` — 対称(reflect) playfield 静止画の自己完結 DASM
    ソースを生成。ABB(kirkjerk) の対称カーネルを土台に、水背景の per-row COLUBK を追加。レジスタ設定順
    COLUBK→PF0→PF1→PF2→COLUPF（PF を just-in-time で書き glitch 回避、水背景は全幅正しく）。
  - `cmd/genpf`: Monet 池シーンを設計（青/紫/緑の per-row 水 wash＋睡蓮パッド散布）→ `roms/monet_m1.asm`。
  - **検証:** read_row(scanline30) で睡蓮パッド col2-5(clock8-23, 緑COLUPF)＋鏡像(clock136-151)＝設計と完全一致、
    水(COLUBK青)も正しい。Monet の横色帯＋対称睡蓮が実機 Gopher2600 に出た。

## [0.6.0] - 2026-06-09

### 追加
- **`read_row` ツール（playfield 点灯列 / per-scanline 色を数値で読む）。** 注釈スクショの目視に頼らず、
  指定した可視 scanline の 1 ライン分のピクセル色を横方向に連長エンコード(RLE) `{clock,len,hex}` で返す。
  鉄則#1「判定は数値、スクショは補助」を playfield/色にも適用するための汎用プリミティブ。
  - `internal/emu/emu.go`: `Emu.ReadRow(scanline)` + `RowRun` 型。`Snapshot()` の可視クロップ RGBA を RLE 化。
  - 北極星 ROM「Monet 睡蓮 Frogger」の gap 0（PF ビット順 litmus）と gap 1（per-scanline 色）の検証土台。
- **playfield ビット順 litmus（`roms/litmus_pf.asm`）＝gap 0 合格。** ABB / falukropp の 2 ソースから抽出した
  「列→PF レジスタ＋ビット順」変換表を実機 Gopher2600 で `read_row` 裏取り。各レジスタ最左列のみ点灯
  （PF0=$10→clock0-3 / PF1=$80→16-19 / PF2=$01→48-51、右半に反復）。各 4clock 幅ぴったりで完全一致。
  確定表を `docs/resources.md`・`CLAUDE.md` に焼いた。副産物: TIA 書込専用レジスタは poke 非持続の癖を記録。
- **per-scanline 色 litmus（`roms/litmus_color.asm`）＝gap 1 合格。** 毎ライン `stx COLUBK` で縦グラデーション。
  `read_row` で scanline 20/100/180 が各々別の単色（全幅160の単一 run）＝Monet の横色帯の核を実証。
- **`internal/playfield` パッケージ＝gap 2 核。** ASCIIグリッド→PF0/PF1/PF2 変換（`EncodeSymmetric` /
  `EncodeAsymmetric` /`ParseASCIIRow`）。検証済みビット順を実装し、**litmus の実機値と一致することを go test で自己検証**。

## [0.5.1] - 2026-06-09

### 追加
- **`get_screen_annotated` が PNG をファイルへも保存（通信回線の実用化）。** インライン画像を描画しない
  クライアント（CLI ターミナル等）でもユーザーが最新フレームを開けるよう、毎回固定パスへ上書き保存する。
  - 保存先は env `ATARI2600_SCREEN_PATH`（未設定なら OS temp の `atari2600_screen.png`）。
  - `.mcp.json` で `<project>/preview/screen.png` を指定。VS Code の画像プレビューはファイル変更で自動リロード
    ＝タブを開きっぱなしで「Claude が呼ぶ→画面が更新される」往復が成立する。
  - structured Out に `png_path` を追加し、保存先の絶対パスを毎回返す。
  - `.gitignore` に `/preview/` を追加（生成物）。サーバ版 0.3.0→0.5.1。

## [0.5.0] - 2026-06-09

### 追加
- **`get_screen_annotated` 実装（ユーザー↔Claude の通信回線）。** 一級市民として完成。
  - `internal/emu/capture.go`: `PixelRenderer` 実装でフレームを `image.RGBA` 捕捉（thumbnailer パターン）。
    座標規約: クロップ画像 x = 可視 clock 0..159（= `HmovedPixel`）、y = 絶対 scanline − visibleTop。
  - `internal/annotate`: TIA 実座標の XY グリッド＋軸ラベル＋スプライトマーカー（Fixed Debug Colors）を
    nearest-neighbor ×3 拡大で描画（`fogleman/gg` + `basicfont`）。ラベルは clock 順ソートで 2 段化し重なり回避。
  - MCP ツールは **画像（`ImageContent` PNG）＋ 数値（structured Out のスプライト位置）を同時返却**。
    JSON-RPC 往復で base64 無損を確認。litmus_pos の白スプライトが clock 72 のマーカーと一致。
  - これでユーザーが画像を見て「P0 を clock 80 へ」と座標指示 → Claude が register に直訳する往復が成立。

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
