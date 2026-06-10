# Changelog

このプロジェクトの変更履歴。形式は [Keep a Changelog](https://keepachangelog.com/)、
バージョンは [Semantic Versioning](https://semver.org/) に従う。

## [Unreleased]

### 追加予定
- 実ゲーム制作（ハーネスを使った本番。Pong 再挑戦など）
- `step_scanline|clock` / `watch|trap` ツールの拡充

## [0.22.0] - 2026-06-10

### 変更
- **物理 spinoff：基盤を独立リポ `atari2600-harness` に分離（ロムは別リポ `atari2600-roms` へ）.** 傘フォルダ
  `260609_atari2600-dev/` の下に `harness/`（この repo・既存履歴を維持）と `roms/`（新 repo）を兄弟配置し
  `go.work` で束ねる。`roms/frogger` を harness から除去し roms repo へ移設、`roms/litmus` は基盤の持ち物として残留。
  **harness の game 依存を根絶**：scenario/emu のユニットテストが参照していた frogger ROM を litmus へ付け替え、
  新フィクスチャ `roms/litmus/scenarios/golden.json`(+`.golden`) を追加。`.mcp.json`/`.claude` は傘直下へ
  （Claude Code のプロジェクトルートで読まれるため）。CLAUDE.md の構成・開発環境節を spinoff 後の実態へ更新。
  検証：harness `go vet`/`go test` 緑・litmus 4 シナリオ PASS、roms 側 `gen`＋frogger 3 シナリオ PASS。
- **Go モジュール名を `github.com/kidsnz/atari2600-dev` → `github.com/kidsnz/atari2600-harness` にリネーム
  （spinoff 準備）.** 基盤が独立リポ `atari2600-harness` になる前提に合わせ、`go.mod` と import 9 ファイルを
  一括置換。build/vet/test 緑、全シナリオ PASS。
- **`internal/playfield` → `pkg/playfield` へ公開昇格（spinoff 準備）.** Go は `internal/` を別モジュールから
  import できないため、playfield エンコーダ（Atari 2600 普遍知識）を公開パッケージ化。唯一の越境 importer
  `roms/frogger/gen` の import を書換。全シーンを再生成（header コメントのみ差分）。build/vet/test 緑、
  全シナリオ（frogger 3＋litmus 3）PASS で裏取り。これで roms を別モジュールに切り出してもエンコーダを共有できる。
- **ドキュメント鮮度監査（spinoff 前段）.** `README.md` を v0.21.0 の実態へリライト（旧構成図＝`cmd/probe`＋
  `internal/emu` のみ → cmd 4本・internal 6本・roms/<game>・MCP 19ツール・欠落A〜E全閉を反映、smoke.asm パスを
  `roms/litmus/` に修正）。軽微 stale も是正：`improvement-roadmap` に「基盤系は全クローズ」注記、`mcp-tools` の
  Phase 表記を現状化、`tool-landscape` の Gopher2600 行を「ライブラリ埋め込みに確定（terminal 不要）」へ、
  `roms/frogger/gen/asmgen.go` の `cmd/genpf`（統合済で不在）コメントを `roms/frogger/gen` に。

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

## [0.21.0] - 2026-06-10

### 追加 / 変更
- **シナリオの `rom` に `.asm` ソースを直接指定可能に（欠落E を完全クローズ）。** scenario の `rom` が `.asm` なら
  実行前に dasm でアセンブルしてから走る ＝ **「ソース 1 枚 → アセンブル → 実行 → 数値アサート → 合否」が
  1 コマンド**（`go run ./cmd/scenario foo.json`）。欠落E（反復コスト）の理想形に到達。
- **dasm 実行を `internal/build` に集約（DRY）。** `assemble_and_load`(harness) とシナリオの `.asm` 直指定が
  同じ `build.Assemble`/`build.BinPathFor` を共有。アセンブル失敗は握り潰さずエラー（失敗行を含む dasm 出力）。
  - サンプル: `roms/litmus/scenarios/smoke_src.json`（`rom: smoke.asm`）。
  - 検証: `scenario_test.go` — .asm 直指定で assemble→run→pass、壊れた .asm はエラー。

## [0.20.0] - 2026-06-10

### 追加
- **横位置 X(N) の自動キャリブレーション（B-4 / 欠落B を完全クローズ）。** litmus を「一度きりの手作業」から
  「掃引→自動フィットで再現可能」へ。協調 ROM（`litmus_pos`: 遅延 `DELAY=$80`・SBC/BCS=5 CPU サイクル/ユニット）
  の遅延を poke で振り、各フレームの `player0.ResetPixel` を実測 → 直線回帰して傾きとオフセットを数値で復元。
  - **実装**: `internal/calibrate`（`Sweep` = poke 掃引＋ResetPixel 実測 / `Fit` = 純関数の直線回帰）。
    `Fit` は **160 折返しと左端飽和に頑健**：mod-160 デルタの中央値で 1 ユニット前進量を推定し、その前進を保つ
    **最長連続区間だけ**を unwrap して最小二乗（strobe が有効域外で X=3 に張り付く飽和点を除外）。
    `cmd/calibrate [rom] [lo] [hi]` で表＋傾き＋R² を表示。
  - **実測結果（litmus_pos）**: DELAY 2..11 で X = 12,27,…,147（+15px/ユニット）→ **slope = 3.0000 px/CPU-cycle**
    （実機権威値 3 と一致）・R²=1.0・kernel offset（unwrap 後 DELAY=0 外挿）= −18。
  - **検証**: `calibrate_test.go` — 折返し＋飽和混在の合成データで slope 復元、実 ROM 掃引で 3 px/cycle 再現、
    無変化データはエラー。

## [0.19.0] - 2026-06-10

### 追加
- **ゴールデンフレーム回帰（P2 D-3 / 欠落D を完全クローズ）。** シナリオに `checks.golden_frame: true` を
  足すと、タイムラインの**描画フレーム連鎖ハッシュ**を `<scenario>.golden` と照合する＝「描画ピクセルが
  変わってないか」をピクセル単位で回帰検知（D-1/D-2 のロジック/タイミング回帰を補完）。
  - **実装**: Gopher2600 の exported `digest.Video`（`NewVideo`/`Hash`/`ResetDigest`、フレーム毎 sha1 連鎖）を
    `internal/emu` に配線（`EnableVideoDigest`/`ResetVideoDigest`/`VideoHash`、任意・冪等）。`internal/scenario`
    は golden 有効時に digest を有効化 → warmup 後にリセット（warmup を除外し決定的化）→ タイムライン →
    副作用計測の前にハッシュ確定 → `.golden` と照合。`cmd/scenario -update` で基準を記録/更新。
  - **サンプル**: `roms/frogger/scenarios/golden.json` ＋ committed `golden.golden`。
  - **検証**: `scenario_test.go` — 同一 ROM/入力でハッシュ決定的（再現）、committed 基準と一致（陽性）、
    基準が違えば fail（陰性）。
  - 注: golden を使うシナリオのみ per-frame sha1 コスト。CLI のみ＝`bin/harness`（MCP）は不変。

## [0.18.0] - 2026-06-10

### 追加
- **シナリオ・ランナー（P2 / 欠落D = 検証自動化の第一歩。D-1 アサーション + D-2 入力リプレイ）。**
  「入力タイムライン＋数値アサーション」を 1 つの JSON で宣言し、ROM に対して自動 pass/fail する。
  `go run ./cmd/scenario <file.json> ...`（全 pass で exit 0／失敗で exit 1）＝ **MCP 不要で CI に乗る**回帰土台。
  - **設計の肝**: アサーションの語彙（`field` 文字列）が `internal/emu` の read 系メソッドに 1 対 1 で対応
    ＝今日まで積んだ観測ツールをそのまま回帰の語彙として使う（ドッグフーディング）。
    瞬時フィールド: `frame`/`scanline`/`clock`/`cycles_total`/`cpu.*`/`ram.0xNN`/`tia.<obj>.<reset|hmoved>_pixel`/
    `tiareg.<obj>.<reg>`/`collisions.<pair>`/`audio.<ch>.<reg>`。未知フィールドはエラー（タイポを握り潰さない）。
    副作用のある計測は run 全体の `checks{ntsc_frame_lines, max_line_budget}` に分離（評価順の破壊を回避）。
  - **構成**: `internal/scenario`（パース＋語彙解決＋Run、ROM 非依存）/ `cmd/scenario`（薄い CLI、`cmd/probe` 規約）。
  - **サンプル**: `roms/litmus/scenarios/`（smoke=`ram.0x80==$42`+262行+予算、collide=`bl_pf==1`）、
    `roms/frogger/scenarios/`（boot=FrogY初期144+残機3+262行+予算、**hop=`up`入力で FrogY 144→128**＝入力タイムライン
    が実ゲームで効くことを実証）。
  - **検証**: `internal/scenario/scenario_test.go` — 全サンプル pass（陽性）、故意に外したアサーションで
    `Result.Pass=false` を検出（陰性）、演算子テーブル、未知/不正フィールドはエラー。
  - 注: 本機能は CLI のみで `bin/harness`（MCP）は不変＝MCP 再接続は不要。

## [0.17.0] - 2026-06-10

### 追加
- **`read_audio` MCP ツール（R-2 / 音声検証経路）。** TIA 音声レジスタ AUDC(control)/AUDF(freq)/AUDV(volume)
  の現在値を両チャンネル分、数値で返す。`read_tia`/`read_row` は映像のみで音声に検証経路が無かった
  （鉄則1「判定は数値」を音声領域へ拡張）。Gopher2600 の exported `Audio.PeekChannels()` を使うため
  外部クローンの改変は不要（channel0/1 自体は unexported だが PeekChannels で取れる）。
  - **実装**: `internal/emu` `ReadAudio()` + 型（`AudioChannel`/`AudioState`）。`cmd/harness/main.go` に
    handler + `AddTool`。
  - **検証ロム**: `roms/litmus/litmus_audio.asm`（ch0=AUDC$0C/AUDF$14/AUDV$0A, ch1=AUDC$04/AUDF$1F/AUDV$08）。
  - **検証**: `emu_audio_test.go` で既知書込みと完全一致。MCP e2e でも両チャンネル一致を確認。

## [0.16.0] - 2026-06-10

### 追加
- **`assemble_and_load` MCP ツール（P3 / ビルドループ短縮）。** asm パスを受け `dasm -f3` を `os/exec` で
  実行し、成功なら出力 .bin を即ロードする 1 ショット（`edit→dasm→load_rom` の多段を畳む）。失敗時は MCP
  エラーにせず `ok=false` ＋ `dasm_output`（失敗行＋理由を含む）で構造化返却＝Claude がその場で直せる。
  `bin_path` 省略時は asm 拡張子を `.bin` に。`cmd/harness` 内に閉じる。
  - **検証**: MCP e2e で成功（smoke.asm → `ok=true`/`loaded=true`）と失敗（不正 asm → `ok=false`、
    `dasm_output` に `"... (3): error: Unknown Mnemonic 'lda'."`）の両経路を確認。

## [0.15.0] - 2026-06-10

### 追加
- **`step_instruction` / `step_scanline` MCP ツール（B-2 / フレーム内粒度）。** これまでフレーム単位
  （`step_frame`）でしか進められず kernel の途中状態を覗けなかった。
  - `step_instruction`: ちょうど 1 つの CPU 命令を実行（保留中の WSYNC stall を消化してから）。返却=その
    命令のサイクル数＋座標。`read_cycles` と対で 1 命令ずつ追える。
  - `step_scanline`: TV の scanline が 1 つ進むまで（フレーム境界では次フレーム scanline 0）。返却=その
    scanline 区間で実行した CPU サイクル＋座標。ライン単位で kernel 状態を検分。
  - **実装**: `internal/emu` `StepInstruction`/`StepScanline`（共通 `stepInstr` プリミティブの上）。
    `cmd/harness/main.go` に handler 2 つ + `AddTool`。
  - **検証**: `emu_step_test.go` — litmus_cycles で各 step の LastCycles が 2(NOP)/3(JMP)、累積がちょうど
    その分進む。smoke で scanline がちょうど +1（フレーム境界で 0 折返し）・各ラインでサイクル消費 > 0。
  - 注: 色クロック単位の `step_clock` は `Step` が命令単位のため未実装（finer hook が要る、将来）。

## [0.14.0] - 2026-06-10

### 追加
- **`read_tia_registers` MCP ツール（P1 / 欠落A の残りを閉じる）。** 書込専用 TIA レジスタ
  （COLUP0/1・COLUPF・COLUBK・NUSIZ・CTRLPF・PF0/1/2・REFP・VDEL・ENAM/ENABL・GRP 等）の現在値を
  Gopher2600 内部保持から直接数値返却。「`sta COLUP0` は本当に効いたか」を `read_row` の色推論でなく
  実測で確かめられる。`internal/emu` に exported 型（`PlayerRegs`/`MissileRegs`/`BallRegs`/`PlayfieldRegs`/
  `TIARegisters`）と `ReadTIARegisters()`。確認済みシンボル: `TIA.Video.{Player0/1,Missile0/1,Ball,Playfield}`
  の各 exported フィールド（`player.go`/`missile.go`/`ball.go`/`playfield.go`）。
  - 検証: smoke の `COLUBK=$1E`、litmus_pf の PF 非ゼロを実測一致。MCP 実測で **PF0=$F0**（PF0 は上位
    ニブルのみ＝実 TIA 挙動）も確認。
- **`read_collisions` MCP ツール（P1）。** 8 本の衝突ラッチ（CXxx, `$30–$37`, 各 D7/D6・sticky）を
  名前付き真偽ペア（`p0_p1`/`m0_p0`/`p0_pf`/`bl_pf` …）に構造化。Frogger の OnPad 判定が使っていた
  raw peek の置換。ビット割当は Gopher2600 `collisions.go` の `tick()` で裏取り（純関数 `decodeCollisions`）。
  - 検証: `decodeCollisions` の D7/D6 全ペア単体テスト、無スプライト ROM で all-false、新規
    `roms/litmus/litmus_collide.asm`（PF 全点灯＋ボール有効）で **BL-PF を陽性検出**。

## [0.13.0] - 2026-06-10

### 追加
- **`assert_line_budget` MCP ツール（欠落B の本丸 / B-3 = per-scanline サイクル予算ガード）。** Pong v2 を
  黙って殺した失敗モード（per-scanline サイクル超過 → 画面ロール、検知不能）を数値で捕まえる。最大
  `max_frames` 走らせ、ある論理ライン（= WSYNC ストローブの間隔）が `budget`（既定 76cy = 1 ライン）を
  超えて余分なスキャンラインを食い込んだ瞬間に停止。返却 `over` / `at_scanline`（超過ラインの開始）/
  `line_cycles`（消費した概算 machine cycle）。多ライン・カーネル（2LK 等）は `budget` を 152 等へ。
  - **検出原理**: WSYNC ストローブ = CPU `RdyFlg` の true→false 遷移（WSYNC だけが RDY を落とす,
    `tia.go:195`）。WSYNC は必ず次スキャンライン境界まで stall するので、連続ストローブ間の **scanline 差 =
    その論理ラインが消費した物理ライン数**（work が 76cy に収まれば 1、超えれば ≥2）。machine-cycle 差は
    隣接ライン依存で誤検知しやすいため scanline 差を採用。debugger の `watch|trap`（型 unexported）に
    触れず、exported `RdyFlg` + ビーム座標だけで `internal/emu` の自前 step ループに実装（v0.3.0 の
    「debugger driver を外す」決定と整合、roadmap G-1 の限界どおり）。
  - **実装**: `internal/emu/emu.go` `RunUntilBudget`。`cmd/harness/main.go` `handleBudgetGuard` + `AddTool`。
    起動直後はリセット／VSYNC 同期が乱れる（実測: frame 0 で strobe が scanline 22→30 と飛ぶ）ため、
    計測前に 2 フレーム空走して安定させる。
  - **検証ロム**: `roms/litmus/litmus_overrun.asm`（可視中央に WSYNC 前 ~100cy のビジーループを 1 本だけ仕込み、
    その論理ラインが 2 物理ラインを食う）。
  - **検証**: `internal/emu/emu_budget_test.go` — overrun ROM で `over=true`・`line_cycles=152`（2 ライン消費）・
    `at_scanline` が可視域、正常 ROM（smoke / **frogger 実ゲーム**）で `over=false`（誤検知なし）。frogger は
    60 フレーム走らせても無検知。MCP end-to-end でも overrun→over=true(132,152) / frogger→over=false を確認。

## [0.12.1] - 2026-06-10

### 修正
- **`read_cycles` が WSYNC stall 中の空転を多重カウントしていた（v0.12.0 のバグ）。** CPU は WSYNC stall 中
  （`RdyFlg=false`）だと命令を実行せず `cycleCallback` を 1 回呼ぶだけで返り `LastResult` を据え置く
  （Gopher2600 `cpu.go:614`）。旧実装は命令境界ごとに `LastResult.Cycles` を加算していたため、stall の各
  Step で直前命令のサイクル数を重複加算し、WSYNC を使う全 ROM で過大計上になっていた（litmus_cycles は
  WSYNC 不使用のため初回検証をすり抜けた）。
  - **修正**: 進行を `stepInstr()` プリミティブへ統一。**Step 直前に `RdyFlg` が true だった時だけ**加算する
    （＝実命令が走ったステップのみ）。`cpuCycles` の意味は「実行した命令サイクルの総和（WSYNC 空転は含めない）」。
    `RunFrames`/`RunUntilBeam` を `RunForFrameCount` から自前 step ループへ置換し全経路で一貫させた。
  - **回帰テスト**: `TestCycleCounterExcludesWsyncStall`（smoke.bin で 1 フレームの実行サイクルが
    マシン時間 lines×76 の 1/4 未満であることを検証＝旧バグなら超過して落ちる）。実測 = 1 フレーム 2336 cy
    （マシン 19912 の約 1/8）。

## [0.12.0] - 2026-06-10

### 追加
- **`read_cycles` MCP ツール（欠落B＝タイミングを実制作ループに通す P0 第1段 / B-1）。** CPU サイクルを
  シミュレータから数値で取得する（鉄則2を litmus 限定から実ループへ初めて実体化）。返却＝直近1命令の
  サイクル数 `last_instruction_cycles`・直近 mark 以降の `cycles_since_mark`・ROM ロード以降の `total_cycles`。
  `reset=true` で区間計測の基準点を今に揃える。
  - **実装**: `internal/emu/emu.go` に累積カウンタ `cpuCycles` と `LastCycles/TotalCycles/CyclesSinceMark/
    MarkCycles`。`CPU.LastResult.Cycles`（PageFault/分岐の +1 込み実サイクル）を命令境界で加算。全進行経路で
    一貫させるため `RunFrames`/`RunUntilBeam` の continueCheck（命令完了ごと, `run.go`）と `StepFrame` の
    自前ループ双方に hook。`cmd/harness/main.go` に `handleReadCycles` + `AddTool`。
  - **検証ロム**: `roms/litmus/litmus_cycles.asm`（WSYNC 不使用の無限ループ＝CPU 無停止）。命令境界では
    「実行 CPU サイクル × 3 == 進んだカラークロック数」が普遍則として厳密成立する性質を利用。
  - **検証**: white-box テスト `internal/emu/emu_cycles_test.go` が上記普遍則を全命令境界で照合（PASS）。
    MCP end-to-end でも 1 フレーム = 263 lines × 76 cy = `total_cycles 19988` を確認（無停止 263×228/3 と一致）。

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
