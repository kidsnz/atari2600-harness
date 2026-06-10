# 改善ロードマップ — 制作精度を上げる次の打ち手

このプロジェクトの検証ハーネスは成熟し、Monet Frogger（v0.10.1）は**過去で一番正確**に作れている。
本文書は「ここからさらに正確にする」ための**優先度付きの研究ロードマップ**。実装は別セッション。
各項目に**触る場所（実ファイル・検証済み Gopher2600 API シンボル）**を併記し、着手時に憶測なく入れるようにする。

## 中心的所見 — 位置は閉じた、タイミング予算は開いている

最優先と宣言する**欠落B（タイミング）が、実制作ループでは最大の穴のまま**。
*位置*の litmus は完全クローズ（`read_tia` の `HmovedPixel` で任意 X を数値配置できる、`docs/litmus-results.md`）。
だが*タイミング予算*の検証は未着手＝**「この kernel 区間は 76 サイクルに収まったか？」を数値で問う手段が無い**。
これは Pong v2 を黙って殺した唯一の未クローズ失敗モード（per-scanline サイクル超過 → 画面ロール、検知不能）と直結する。
鉄則2「サイクルはシミュレータから取る」は、litmus では使ったが**実制作ループにはまだ通っていない**。

### 欠落タクソノミ 現状ステータス（`docs/gap-analysis.md` 参照）

| 欠落 | 内容 | 状態 | 残り |
|---|---|---|---|
| A 知覚 | 実行結果が見えない | **閉** | `read_tia_registers`/`read_collisions`（P1, v0.14.0）で書込専用レジスタ・衝突を実測。色推論を廃した |
| B タイミング | サイクル計算が合わない | **閉** | 位置・サイクル露出（B-1, v0.12）・予算ガード（B-3, v0.13）・フレーム内粒度（B-2, v0.15）・kernel定数自動較正（B-4, v0.20）すべて実装済 |
| C 知識 | 6502/TIA 詳細の誤り | **閉** | kernel 依存定数（missile式 N・HBLANK境界）が未formalize |
| D 検証 | 再現・回帰が無い | **閉** | シナリオランナー（D-1 アサーション v0.18 + D-2 入力リプレイ v0.18 + D-3 ゴールデンフレーム回帰 v0.19）実装済 |
| E 摩擦 | edit→run→inspect が多段 | **閉** | `assemble_and_load`（P3, v0.16）＋ シナリオ回帰（P2, v0.18-19）＋ scenario の `.asm` 直指定（v0.21）で「ソース1枚→合否」が1コマンド |

---

## P0 — 欠落B: タイミング検証を実制作ループに通す（最優先・最大インパクト）

### B-1. `read_cycles` ツール（サイクル数の露出）　✅ 実装済 v0.12.0（バグ修正 v0.12.1）
- **問題:** `step_frame` はサイクルを一切返さない。Claude は kernel の重さを数値で知れない。
- **提案:** 直近命令／区間の CPU サイクルを数値返却する MCP ツール。
- **触る場所:** Gopher2600 は `mc.LastResult.Cycles`（`Gopher2600/hardware/cpu/cpu.go:52` の
  `LastResult execution.Result`、フィールドは `cpu/instructions/definitions.go:31` 系）で**既に追跡済**。
  `internal/emu/emu.go` に `e.VCS.CPU.LastResult.Cycles` を読むメソッドを足し、`cmd/harness/main.go` で公開。
- **検証:** 既知サイクルの命令列 ROM（例 `LDA#`=2cy ×N）で累積が一致するか。`cpu/cpu_test.go:554` が前例。
- **規模:** 小。最小の追加で鉄則2を実ループに初めて実体化。

### B-2. `step_scanline` / `step_clock` ツール（フレーム内粒度）　✅ step_scanline + step_instruction 実装済 v0.15.0（step_clock は色クロック粒度で未着手）
- **問題:** 現状フレーム単位ステップのみ。kernel の途中状態を覗けない。
- **提案:** scanline+1 / color-clock+1 で前進。CLAUDE.md・CHANGELOG で「未実装(予定)」明記済。
- **触る場所:** `emu.go` に既にある `RunUntilBeam(maxFrames, scanline, clock)`（`emu.go:197`）と
  低レベル `e.VCS.Step(nil)`（`StepFrame` が使用、`emu.go:102`）を土台に、現在 Coords +1 で停止する薄いラッパ。
- **検証:** ステップ後 `Coords()` が期待 scanline/clock に一致。
- **規模:** 小〜中。

### B-3. per-scanline サイクル予算ガード（本丸 — Pong v2 を殺した失敗を直接検知）　✅ 実装済 v0.13.0（`assert_line_budget`）
- **問題:** ある可視ラインが 76 CPU サイクルを超えても**黙って画面が崩れる**だけで、検知手段が無い。
- **提案:** WSYNC 間（= 1 ライン）のサイクルを計測し、**> 76 で halt＋該当 scanline を報告**。
  `breakif` を「ビーム位置条件」から「予算超過条件」へ広げる（未実装の `watch|trap` の中核ユースケース）。
- **触る場所:** `emu.go` の step ループで WSYNC ストローブ間の `LastResult.Cycles` 累積を監視。
  TV の scanline 遷移（`GetCoords().Scanline` 変化）を境界に使う。
- **検証:** わざと重い kernel ライン（cycle over）を仕込んだ ROM で halt が発火、正常 ROM で発火しないこと。
- **規模:** 中。**インパクト最大** — 「タイミングが合っているか」を制作中に常時数値保証できる。

### B-4. kernel 依存定数の自動キャリブレーション（欠落C と接続）　✅ 実装済 v0.20.0（`cmd/calibrate` / `internal/calibrate`。litmus_pos で slope 3 px/cyc・offset −18 を再現）
- **問題:** missile/ball の `X = 3N − 55` の **N 絶対値は kernel 固有**、DELAY 0–2 は HBLANK 境界で非線形
  （`docs/litmus-results.md` 記載の未解明点）。毎回 read_tia 実測が必要。
- **提案:** 現 kernel の RESPx タイミングを掃引して `X(N)` を自動フィットし、その kernel のオフセット定数を返す helper。
- **規模:** 中。litmus を「一度きりの手作業」から「kernel ごとに再現可能」へ。

---

## P1 — TIA シャドウレジスタ読み（欠落A の残り）

### `read_tia_registers`（書込専用レジスタの現在値を実測）　✅ 実装済 v0.14.0
- **問題:** COLUP0/1・COLUPF・COLUBK・NUSIZ0/1・CTRLPF・PF0/1/2・REFP・HMxx は**書込専用**で、
  現状 `poke` も持続しない（CLAUDE.md「poke の癖」）。Claude は `read_row` の**色から推論**するしかない
  ＝「`sta COLUP0` は本当に効いたか？」を間接的にしか確かめられない。
- **提案:** Gopher2600 が内部保持する現在値を**直接数値返却**。色推論を実測へ置換。
- **触る場所（検証済みシンボル）:** `e.VCS.TIA.Video`（`emu.go:60` で既にアクセス）配下に揃っている：
  - Player: `Player0.Color` / `.Nusiz` / `.SizeAndCopies` / `.Reflected` / `.HmovedPixel`
    （`Gopher2600/hardware/tia/video/player.go:96-108`）。
  - Playfield: `Playfield.PF0/.PF1/.PF2` / `.ForegroundColor` / `.BackgroundColor` / `.Reflected` /
    `.Priority` / `.Scoremode`（`tia/video/playfield.go:44-81`）。
  - Video 構造体は `Playfield *Playfield` / `Player0/1` / `Missile0/1` / `Ball` を持つ（`tia/video/video.go:78-85`）。
- **検証:** ROM で `sta COLUP0,#$1C` → `read_tia_registers` が `0x1C` を返す。`read_row` の色と突き合わせ。
- **規模:** 小〜中。**Gap A を「推論ゼロ」へ。**

### `read_collisions`（CXxx の構造化）　✅ 実装済 v0.14.0
- **問題:** Frogger の `OnPad` 判定は CXPPMM を **raw peek（$30–$37）** で読んでいる。
- **提案:** 衝突ラッチを構造化 JSON（P0-P1 / M0-P0 等のペア真偽）で返す。
- **触る場所:** `Gopher2600/hardware/tia/video/collisions.go:25` の `Collisions` 構造体／`chipbus.CXPPMM` 等。
- **規模:** 小。P1 と同系統で同時実装が自然。

---

## P2 — 欠落D: 検証自動化（手動 MCP 連打の置換）

### D-1. アサーション仕様ファイル + ランナー　✅ 実装済 v0.18.0（`cmd/scenario` / `internal/scenario`, `docs/scenarios.md`）
- **問題:** 全チェックが手動 MCP 呼び出し。回帰が人手依存。
- **提案:** ROM ごとに宣言ファイル（JSON）で `scanline == 262` / `P0.HmovedPixel <= 159` /
  `FrogY ∈ [24,160]` 等を記述 → 自動実行・pass/fail 報告。
- **触る場所:** 既存 `internal/playfield/playfield_test.go` の自己検証パターンを **ROM レベル**へ拡張。
- **規模:** 中。

### D-2. 入力リプレイのタイムライン　✅ 実装済 v0.18.0（シナリオの `inputs[]`。frogger hop で実証）
- **問題:** `set_input`（`emu.go:129`）は**単発イベント**。シナリオ再現ができない。
- **提案:** 「フレーム N で up、N+30 で fire」等のスクリプトを流して hop→drown→win を再現テスト化。
- **規模:** 中。

### D-3. ゴールデンフレーム回帰　✅ 実装済 v0.19.0（`digest.Video` 配線、`checks.golden_frame`、`cmd/scenario -update`）
- **提案:** Gopher2600 の `regress` + 録画/再生を配線（フレームハッシュ）。
- **未確認:** `regress` の CLI 構文（`docs/resources.md` でも未確認扱い）→ 着手時に要検証。
- **規模:** 中〜大。

---

## P3 — 欠落E: ビルドループ短縮　✅ クローズ（v0.16 assemble_and_load ＋ v0.18-19 シナリオ回帰 ＋ v0.21 scenario の .asm 直指定＝ソース1枚→合否が1コマンド）

### `assemble_and_load`（多段を 1 ショット化）　✅ 実装済 v0.16.0
- **問題:** `edit asmgen` → `go run ./roms/<game>/gen` → `dasm -f3` → `load_rom` の多段。摩擦が反復速度を削る。
- **提案:** asm パスを受け、`os/exec` で `dasm` 実行 → **エラーをツール面に構造化表面化**（失敗行ハイライト）→
  成功なら即ロード。`cmd/harness` 内に閉じる。
- **規模:** 小。反復の往復を最短化。

---

## その他の角度（中期・要観察）

- **`asmgen.go` のモノリス化:** 5 つの `Generate*ASM`（symmetric/asymmetric/sprite/full/frogger）が
  kernel boilerplate を重複保持。再利用可能な kernel フラグメント／マクロへ。
  → メモリ `[[project-harness-spinoff-todo]]`（基盤の独立リポジトリ化）と接続して整理するのが筋。
- **注釈スクショの拡張:** 現状 sprite X マーカーのみ。**衝突状態・RAM 由来オブジェクト位置（FrogY 等）・
  per-scanline サイクル予算「定規」**をオーバーレイすれば、ユーザー↔Claude の主回線（一級市民）がさらに濃くなる。
  併せて Gopher2600 注釈ピクセル vs Stella オラクルの整合 cross-check（`docs/gap-analysis.md` 指摘の弱点）。
- **音声（AUDC/AUDF/AUDV）が完全欠落:** 検証経路ゼロ。だが資料（Slocum ガイド＋動作ドライバ）が揃っており
  **→ 下記 R-2 で「範囲外」から「着手可能」へ格上げ。**
- **欠落C の formalize:** HBLANK 境界（DELAY 0–2 非線形）・missile 式の kernel 依存 N を `docs/litmus-results.md` に追記。

---

## 参照資料の未採掘脈（docs_atari — カタログ済だが Frogger 段階で未活用）

出典: 旧 Pong プロジェクトの参照トローブ `/Users/shinji/Documents/2D/260304_Claude-Code-Pong/docs_atari`。
**これらは新規発見ではない** — 既に `docs/tool-landscape.md` でカタログ済（Bensema cycle guide＝「★B/C の核心」、
`sethorizpos.asm`、`game_disassembly/`「21本」、`za2600/`、各サンプル）。`docs/gap-analysis.md` も
「参照資料はユーザーが既にほぼ揃えている」と明記。**価値は新ファイルでなく、Frogger 段階に対して
"未採掘の脈"を掘ること。** 設計の根拠（litmus・位置決め）には使ったが、以下は今の北極星にまだ活きていない。

### R-1. Freeway アーキテクチャ移植（最優先・制作直結）
- **問題:** 現 Frogger は lane／複数オブジェクト／衝突を**独自に再発明**している。Freeway は同型
  （横断ゲーム）の**実証済みコマーシャル設計**がそのまま転用できる、最も近い参照。
- **資料:** `game_disassembly/freeway.asm`（1683行）。確認済みの中核構造:
  `LaneNumber` / `CarMotionTimers[10]` / `CarMotions[10]` / `CarXCoords[10]` /
  `Chick0LaneCollide`・`Chick1LaneCollide`（**lane 単位の衝突**）/ `CarShapePtr` / `ZCarPatterns[10]`（NUSIZ で車列）。
- **採掘ポイント:** ① per-lane オブジェクト配列を1ループで回す kernel ② オブジェクト毎の移動タイマー（速度差）
  ③ Y→lane 逆算＋X 重なりによる lane 単位衝突（CXxx に依存しない判定）④ NUSIZ 複数コピーで「車列／睡蓮列」。
- **効果:** Frogger の状態機械（ride/drown/win）と lane 移動を、実機ゲームの設計で裏取り。
- **注:** 逆アセンブルの**構造のみ**学ぶ（コードはコピーしない＝クリーンルーム的に発想だけ移植）。

### R-2. 音声レシピ（"範囲外" → "着手可能" へ格上げ）　🔶 検証経路 `read_audio` 実装済 v0.17.0（音声「制作」side＝効果音はこれから）
- **これまで:** ロードマップは音声を「範囲外」と明記。だが資料は揃っている。
- **資料:** `2600_music_guide.txt`（Paul Slocum: AUDC/AUDF/AUDV の意味、常用 8 音色 =
  Saw / Engine / Square / Bass / Pitfall / Noise / Lead / Buzz、ドラム設定、音程表）、
  `za2600/audio.asm`（**動作するドライバ**: `Tone`/`Freq`/`Vol`・seq notes/durs・`songCur`）、
  8bitworkshop `musicplayer` / `wavetable` / `fracpitch`。
- **採掘ポイント:** 最小の効果音から（hop=短い Square、drown=下降 Noise、win=上昇アルペジオ）。
  VBLANK / Overscan の余り cycle で AUDV/AUDF/AUDC を叩く。
- **ハーネス含意（重要）:** 音声には**数値検証経路が無い**（`read_tia` は映像のみ）。
  → P1 の `read_tia_registers` に **TIA Audio シャドウ読み（AUDC/AUDF/AUDV 現在値）を同梱**すべき。
  確認済み: Gopher2600 は `TIA.Audio *audio.Audio`（`Gopher2600/hardware/tia/tia.go:75`）を持ち、各チャンネルの
  `registers.Control / .Freq / .Volume`（`tia/audio/channels.go`・`tia/audio/registers.go`）が AUDC/AUDF/AUDV 相当。
  ただし `channel0/1` は unexported のため、`read_audio` には小さなアクセサ追加が要る。
- **効果:** Frogger に効果音を足し、かつ「音も数値で検証」へと原則（鉄則1）を音声領域に拡張。

### R-3. サイクルコスト表の蒸留（P0 の "書く側" を補強）
- **相補性:** P0 は Gopher2600 で**測る(measure)**。だが書く前に**予測(predict)**できれば往復が減る。両者は二刀流。
- **資料:** `cycle_counting_guide.html`（Nick Bensema / Random Terrain）。命令カテゴリ別サイクル
  （Fast math / Storage / Weenie / Slow math / Stack / Branch）、`X=(CYCLES−20)×3`、DEY-BNE ループ、
  `(indirect),Y` のページ跨ぎ +1、分岐成立 +1・ページ跨ぎ +2。
- **採掘ポイント:** 常用命令のサイクル早見表を CLAUDE.md か doc に蒸留し、kernel を書く瞬間に参照。
- **効果:** 鉄則2「サイクルはシミュレータから取る」を保持しつつ、初稿のタイミング精度を上げ往復削減。

### R-4. 実ゲーム構造一般（拡散しやすいので薄く・"索引"として）
- **資料:** `spiceware_tutorial/`（14ステップで Collect を完成）、`nanochess_samples/`（Programming Games book）、
  `game_disassembly/`（Pitfall / Kaboom / RiverRaid / Adventure 他）、`8bitworkshop_samples/`
  （multisprite・complexscene・fullgame=LFSR 乱数・score6/BCD・collisions）。
- **採掘ポイント（必要時に引く索引）:** 6桁スコア/BCD、LFSR 乱数、マルチスプライト多重化、
  タイトル/テキスト表示（`za2600/text24.asm`・`*.chr` フォント）。
- **注意:** **拡散リスク大。北極星(Frogger)に必要になった時だけ該当ファイルを引く**運用に留め、索引化以上はしない。

---

## 外部リサーチ — さらに進化させるアイデア（GitHub/web, 2026-06）

GitHub/web を横断調査した結果。**最大の発見が2つ:**
(1) 我々が埋め込んでいる **Gopher2600 自体に、本ロードマップの最難項目が既にライブラリとして実装済**。
`hardware.VCS` を直接埋め込み debugger driver を外した決定（v0.3.0・決定的/単純で正しい）を**壊さずに**、
その"下"の package 群を単体利用できる ＝ 多くの P2/R 項目は「作る」でなく**「配線する」**。
(2) **C64 には emulator-MCP が複数あるが Atari 2600 は皆無 ＝ 本プロジェクトが最初。**

### G-1. Gopher2600 の未使用パッケージを"昇格"（最優先・最接地・インパクト最大）
debugger driver は外したまま、下記ライブラリを単体利用（exported API は実コードで確認済）：

| package | exported API（確認済） | 充たすロードマップ項目 |
|---|---|---|
| `recorder` | `NewRecorder(transcript, *hardware.VCS)` / `NewPlayback(transcript)` | **D-2 入力リプレイ** |
| `regression` | `RegressAdd` / `RegressRun`（video-hash＋Playback＋Log の3種テスト） | **D-3 ゴールデン回帰**（CLAUDE.md 既述・未配線） |
| `tracker` | `Entry`/`Distortion`/`MusicalNote`/`NoteToPianoKey`（`audio.Tracker` 実装） | **R-2 音声検証** — AUDx を**音名/音色名**へ変換 |
| `reflection` | `NewReflector(*hardware.VCS)`（per-video-step の element 帰属＋`Hmove` comb） | **注釈拡張**（どのオブジェクトが描いたか） |
| `digest` | `NewVideo(tv)` / `NewAudio(tv)` | フレーム/音声ハッシュ＝ゴールデン土台 |
| `rewind` | `PokeHook`（**deeppoke**）/ `ComparisonState` | **CLAUDE.md「poke の癖」を解決**（持続 poke）＋状態 diff |

- **正直な限界:** `debugger/halt_watches|traps|breakpoints` も存在するが**型が unexported**で debugger loop に結合
  → `watch|trap`（P0 B-3）は**パターン参照**にとどまる（recorder 等のようなドロップイン不可）。
- **配線コスト差:** `recorder`/`digest`/`regression` はほぼドロップイン。`tracker`/`reflection` は毎 video-cycle ステップや
  FrameTrigger 登録などの配線が要る。
- **ライセンス:** Gopher2600 = **GPL-3.0**（既に `go.mod` の `replace` で内包・同条件運用）。利用形態に留意。
- **効果:** P2(D-2/D-3)・R-2 が「実装」から「既存ライブラリ配線」へ縮小 ＝ 最小コストで回帰・入力リプレイ・音声検証が入る。

### G-2. C64 MCP エコシステムからの借用＋位置づけ
- **発見:** C64 には emulator-MCP が複数 ——
  [`barryw/vice-mcp`](https://github.com/barryw/vice-mcp)（~17k 行 C を VICE に直埋込。breakpoint/sprite/**SID レジスタ読み**/
  screenshot/step を JSON-RPC で）、[`chrisgleissner/c64bridge`](https://github.com/chrisgleissner/c64bridge)、
  [`axewater/mcp-vice-emu`](https://github.com/axewater/mcp-vice-emu)、[`cliffhall/mcp-c64`](https://github.com/cliffhall/mcp-c64) 他。
  **Atari 2600 は皆無 ＝ 本プロジェクトが最初。**
- **借用アイデア:**
  ① vice-mcp の「**SID レジスタ読み**」＝我々の TIA 音声シャドウ読み（R-2 / G-1 `tracker`）の裏付け（音もレジスタで読むのが標準）。
  ② [`barryw/sim6502`](https://github.com/barryw/sim6502) の **pluggable backend テスト DSL**（速い純CPU＋cycle-accurate を同一DSLで切替）
  → P2 を **「sim65＝純CPU高速」＋「Gopher2600＝cycle/TIA 正確」の二層**に（CLAUDE.md「純6502=sim65」と整合）。
  ③ [LLM→6502 パイプライン](https://hackaday.com/2024/11/07/using-ai-to-help-with-assembly/)（Amazon Q 他、RAG コーパス＋自動コンパイラ feedback）
  → P3 `assemble_and_load` の **DASM エラー構造化・即時差し戻し**を強化。我々は CLAUDE.md＋docs が事実上のコーパス。
- **位置づけの価値:** 「2600 未開拓・我々が最初」を `gap-analysis.md` / README に1行明記 ＝ 独立化(spinoff)時の対外的意義が立つ。

### G-3. テスト DSL の先行例（P2 を独自発明しない）
- [`barryw/sim6502`](https://github.com/barryw/sim6502)（DSL＋複数バックエンド）/ [`64bites/64spec`](https://github.com/64bites/64spec)
  （KickAssembler の describe-it spec）/ sim65（cc65、サイクル表示＋トレース）/
  [`AsaiYusuke/6502_test_executor`](https://github.com/AsaiYusuke/6502_test_executor)（cc65 ベース）/
  [`Klaus2m5/6502_65C02_functional_tests`](https://github.com/Klaus2m5/6502_65C02_functional_tests)（全 opcode）。
- **採掘:** P2 D-1 のアサーション仕様は、これらの DSL 形（`expect A == $1C` / `cycles <= 76`）を借りる（独自発明しない）。
  sim65 は確定アーキ既述だが**未配線** → TIA 非依存ロジック（スコア計算・LFSR 乱数 等）は sim65 で高速 CI、TIA 絡みは Gopher2600 と役割分担。
  Klaus2m5 は Gopher2600 CPU 自体の正しさ担保にも使える。

### G-4. 制作（オーサリング）ツール連携（中期・任意）
- [`PlayerPal 2.2`](https://atariage.com/forums/topic/318184-tool-update-playerpal-22/)（マルチカラー sprite エディタ→ASM/batari 出力）/
  [masswerk VCS tools](https://www.masswerk.at/vcs-tools/) / Tiny 8-bit sprite editor / batari の PF・sprite・music エディタ。
- **採掘アイデア:** ① これらの ASM/データ出力形式を import して `roms/<game>/gen` のスプライト表に流し込む。
  ② 野心案: **注釈スクショを"逆向き"に使う** ＝ ユーザーが画像上で塗る → GRP/register データへ
  （CLAUDE.md「主回線」を**入力方向へ拡張** ＝ paint→register エディタ）。
- **注意:** 北極星(Frogger)に必要になってからでよい。拡散リスクがあるので索引化に留める。

---

## 推奨着手順

1. ~~**B-1 `read_cycles`** → **B-3 予算ガード**（本丸）~~ ✅ v0.12.0–v0.13.0。欠落B が実ループで閉じた。
2. ~~**P1 `read_tia_registers` + `read_collisions`**（推論ゼロ化）~~ ✅ v0.14.0。欠落A クローズ。
3. ~~**B-2 `step_scanline`**（+ `step_instruction`）~~ ✅ v0.15.0（`step_clock` は色クロック粒度で未着手）。
4. ~~**P3 `assemble_and_load`**（摩擦低減）~~ ✅ v0.16.0。
5. ~~**P2 検証自動化**（回帰の土台）~~ ✅ D-1+D-2 (v0.18.0) + D-3 ゴールデン回帰 (v0.19.0)。**欠落D 完全クローズ**。
6. ~~**B-4 kernel 定数 自動較正**~~ ✅ v0.20.0（欠落B 完全クローズ）。残り: **R-2 音声"制作"側** / **R-1 Freeway 移植** / **ハーネス独立化**（＝制作 or spinoff フェーズへ）。

> 制作（Frogger）側では **R-1 Freeway アーキテクチャ**が最も即効。音声検証 **R-2** は P1
> （`read_tia_registers`）に Audio シャドウを同梱する形で一緒に入れるのが自然。
> **P2/R-2 は自前実装せず G-1 の Gopher2600 既存パッケージ（`recorder`/`regression`/`tracker`）配線が最短路。**
> 実装時は CLAUDE.md「Smoke-test harness before reconnect」に従い、`bin/harness` 改修後は
> MCP `initialize` でスモークテストしてから再接続を依頼すること（`AddTool` 起動パニック対策）。
