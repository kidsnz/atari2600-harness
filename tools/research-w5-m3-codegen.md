# research-w5-m3-codegen — シーン記述 → 手書き品質 DASM カーネルの codegen 設計（W5・M3/M4 de-risk）

目的: TIA Studio の **M3＝「ヘテロな全要素（P0/P1/M0/M1/BL/PF＋走査線毎色）を1画面に合成し、手書き品質の DASM
カーネルを生成する」段階**の codegen を設計確定する。M3/M4 のこの部分は **既存 OSS に存在しない新規領域**であり、
本ノートの狙いは「実在 OSS の codegen 手法をソースで読み、流用できる配線と、流用できない壁（＝自前で書く部分）を切り分け、
推奨アーキテクチャを1本に決める」こと。収集日 2026-06-13。**[確認]=GitHub ソース/一次資料 実読 / [推定]=未実装の設計方針**。

前提（確定済み）: 編集=spritemate fork / プレビュー=6502.ts Stellerator embed / codegen 配線の参考=vcs-game-maker。
データモデルは w4 で確定（`SpriteData.pixels:number[][]` + `colorPerRow:number[]`＝走査線→TIA色値の1次元配列）。
本ノートは「そのシーン中間表現 → DASM カーネル」の変換器の設計に絞る。

---

## 0. M3 の新規性（なぜ OSS に無いか）

既存 OSS の codegen は **すべて「固定カーネル＋データ表」モデル**で、走査線ごとに要素構成（どの GRP/PF/色を出すか）が
変わる**ヘテロな1画面**を吐く機能を持たない。

- **vcs-game-maker / batari Basic** = bB の固定カーネルに **データだけ**流し込む。カーネル骨格は全ゲーム共通で、ユーザーは
  書き換えられない（後述 §2-c が一次資料で明言）。多スプライト合成・走査線毎の自由な要素構成は原理的に対象外。
- **8bitworkshop** = エディタ＋アセンブラ（DASM/bB/cc65）＋エミュ。**codegen も最適化も持たない**（後述 §3）。
- **ScreenEditor2600**（lucienEn・Win/.NET/closed・spec §正直なリスク） = 合成スクリーン→codegen を**1つだけ**実在させた
  唯一の前例だが closed・非 Web・harness 非連携・コード駆動不可。**OSS で再利用できる実装は存在しない**。

→ M3 codegen は「シーン中間表現（要素グラフィック＋TIA座標位置＋走査線毎色）→ 手書き品質の DASM カーネル」を
**自前で書く**しかない。OSS から流用できるのは **配線パターン（中間表現の分け方・テンプレ穴埋めの粒度・データ表の吐き方）**
であって、カーネル生成そのものではない。これが本ノートの核心的な切り分け。

---

## 1. vcs-game-maker — 「2系統中間表現 → Handlebars マスターテンプレ穴埋め」[確認]

リポ: https://github.com/haroldo-ok/vcs-game-maker （MIT）。default branch `master`。

### 1-a. finish() の穴埋め方式 [確認 raw 実読 `src/generators/bbasic.js`]

`Blockly.BBasic.finish(code)`（https://github.com/haroldo-ok/vcs-game-maker/blob/master/src/generators/bbasic.js）が
codegen の合流点。3つの生成器＋6つのイベントスロットを **Handlebars テンプレに名前付きで流し込む**:

```js
Blockly.BBasic.finish = function(code) {
  code = ...finish.call(this, code);             // Blockly が組んだユーザーロジック
  code = Blockly.BBasic.normalizeIndents(code);
  const generatedConfiguration = Blockly.BBasic.generateConfiguration();
  const generatedBackgrounds   = Blockly.BBasic.generateBackgrounds();   // PF データ
  const generatedAnimations    = Blockly.BBasic.generateAnimations();    // sprite フレーム
  // … system_start / title_start / title_update / gameplay_start / gameover_* の各ラベル生成 …
  return handlebarsTemplate({ generatedBody, generatedConfiguration,
    generatedBackgrounds, generatedAnimations,
    systemStartEvent, titleStartEvent, titleUpdateEvent,
    gamePlayStartEvent, gameOverStartEvent, gameOverUpdateEvent });
};
```

**名前付きスロットの粒度** = 「ロジック本体1枠 + グラフィック2枠（背景/アニメ）+ ライフサイクル・イベント6枠」の**粗粒度**。
スロットは**機能ブロック単位**（カーネルの走査線単位ではない）。これが bB が固定カーネル前提で済む理由＝走査線レベルの
合成は bB ランタイムが裏で吸収するので、テンプレはマクロな差し込みで足りる。

### 1-b. グラフィックデータの注入の仕方 [確認]

- **PF**: `src/utils/pixels.js` の `matrixToPlayfield`（https://github.com/haroldo-ok/vcs-game-maker/blob/master/src/utils/pixels.js）＝
  `matrix.map(line => line.map(p => p ? 'X' : '.').join('')).join('\n')` で **行列→`X/.` テキスト**化し、bB の `playfield:` ブロックに流す。
  逆変換 `playfieldToMatrix` で往復（編集モデル＝行列、出力＝bB DSL テキスト）。
- **スプライト**: `generateAnimations()` が各フレームの pixel 行を **`frame.pixels.slice().reverse().map(row => '  %' + row.join(''))`**＝
  **下→上に reverse して `%xxxxxxxx` バイト行**を吐く（2600 の GRP は下端が先＝reverse が必須なのが裏取れる）。
  フレーム送りは `player0frame = player0frame + 1 / if player0frame >= dur then ... / if player0frame > limit then goto`
  ＝**フレームカウンタのステートマシン**を生成。
- **複数要素を1カーネルに織り込む方法**: vcs-gm は**織り込まない**。背景は `if newbackground <> id then goto background{id}end`＝
  実行時 if-goto ディスパッチ、プレイヤーは bB ランタイムの GRP0/GRP1 に各 1 枠。**＝「1画面に最大2 player + 1 PF」という
  bB 固定カーネルの器に各要素を割り当てるだけ**で、走査線毎の自由合成はしていない。

### 1-c. マスターテンプレ `bbasic.bb.hbs` [確認 WebFetch]

https://github.com/haroldo-ok/vcs-game-maker/blob/master/src/generators/bbasic.bb.hbs ＝
**固定の init（`set tv ntsc` / `set romsize 4k` / `COLUBK` / `COLUPF` / `player0x/y`・`player1x/y` / `playerNsize=$30` /
`playerNrealcolor`）+ 4つの `{{{ slot }}}`（generatedConfiguration / generatedBody / generatedBackgrounds /
generatedAnimations）+ 6イベントスロット**。`generatedBody` は `main → commongamelogic → drawscreen` ループの
**drawscreen 直後**に置かれる＝bB の「1フレーム描画は drawscreen が全部やる」前提だからスロットがマクロで済む。

### → M3 への流用可否
- **流用できる [推定]**: ①「グラフィック中間表現（pixel 行列）とロジック/配置を別系統に分け、finish() で1本化」②「Handlebars
  マスターテンプレ + 名前付きスロット穴埋め」③「アニメ=フレーム配列 + reverse して `%bin` 行 + frame カウンタ SM」
  ④「pixel 行末や別ラベルでデータ表を吐く」配線。
- **流用できない（壁）**: vcs-gm のスロット粒度は **bB 固定カーネルの器に合わせた粗粒度**。M3 の「走査線ごとに要素構成が変わる
  ヘテロカーネル」は、この **マクロ穴埋めでは表現できない**（drawscreen に相当する魔法のランタイムが我々には無く、走査線毎の
  STA タイミングを我々自身が生成する必要がある）。→ テンプレ穴埋めの**粒度を走査線/ゾーン単位に下げる**必要（§4・§5）。

---

## 2. batari Basic — 「固定カーネルテンプレ + データ表」の長所/限界 [確認]

リポ: https://github.com/batari-Basic/batari-Basic （MIT）。カーネルは `includes/*.asm`。

### 2-a. カーネル一覧 [確認 git tree 実読]
`includes/std_kernel.asm` / `std_kernel_vertical_reflect.asm` / `multisprite_kernel.asm`（+`multispriteheader.asm`）/
`DPCplus_kernel.asm`（+`DPCplus.inc`/`DPCplusheader.asm`/`.arm`）/ `PXE_kernel.asm` / `pf_drawing.asm` /
`pf_scrolling.asm` / `score_graphics.asm`。
(https://github.com/batari-Basic/batari-Basic/tree/master/includes)

### 2-b. 標準カーネルの「固定スケルトン＋表参照」構造 [確認 raw 実読 `includes/std_kernel.asm`]
主ループ `.kerloop`（コメント "enter at cycle 59"）/ `.continuekernel`。各可視走査線で **indexed/indirect load** によって
ユーザーデータ表を引く:
- player0 graphics: **`lda (player0pointer),y` → `sta GRP0`**（Y=スプライトのY位置でグラフィック表を引く）
- player0 color: `lda (player0color),y` → `sta player0colorstore`（後で `sta COLUP0`）＝**per-scanline player color が表で可能**
- playfield: `ldy playfield+pfres*pfwidth-132,x` → `sty PF1L` 等（X=PFインデックス）
- PF color: `lda (pfcolortable),y` → `sta COLUPF`（または `COLUBK`）＝**per-scanline PF color が表で可能**

**重要な限界 [確認]**: 走査線ごとのコードは **`ifconst`/`ifnconst` のコンパイル時分岐**（`playercolors`/`PFcolors`/`readpaddle` 等の
有無）で**生成時に1回だけ枝刈り**される＝**ゲーム実行中は全走査線が同一の固定命令列**。だから per-line サイクルレイアウトは
固定（約60–70cy/line、WSYNC 始端、nop/sleep でパディング）。**＝「走査線ごとに違う要素を出す」自由は無い。表の値が変わるだけ**。

### 2-c. multisprite カーネル [確認 raw 実読 `includes/multisprite_kernel.asm`]
- 多スプライトは **flickersort アルゴリズム**: 毎フレーム Y でソート→重なり検出→重なったら `dec temp3` で**その frame は間引く（フリッカ）**。
- P1 を**画面途中で `sta RESP1` 再配置**して GRP1 を使い回す: `lda NewSpriteX,X / sta RESP1 / sta WSYNC / sta HMOVE`。
- **限界**: 最大 **5 スプライト**（ソート表サイズ）/ 縦重なりで**フリッカ必須** / 色は **`NewCOLUP1` でスプライト毎1色**（走査線毎ではない）/
  カーネル本体は**固定**（変わるのは `NewSpriteY/X`・`player1pointerlo/hi`・`spriteheight` 等の表のみ）。

### 2-d. 一次資料の限界明言 [確認 WebFetch randomterrain bB commands]
batari の標準カーネルは **「ユーザーが書き換えられない pre-written assembly framework」**。ユーザーは graphics/PF/色/座標の
**データだけ**供給。文書化された限界＝「**per-scanline のスプライト構成変更は不可**」「PF はデフォルト2-line解像度」「タイミング固定」
「オブジェクト増は別カーネル/フリッカで買う」。(https://www.randomterrain.com/atari-2600-memories-batari-basic-commands.html)

### → 固定カーネルテンプレ方式の長所/限界（手書き品質に届くか）
- **長所**: ①生成が単純（テンプレ1枚＋データ表）②サイクルが事前検証済みで安全 ③色テーブル indexed load は我々の per-scanline
  color データモデル（w4 確定）と**そのまま噛む**（`LDA table,Y / STA COLUPx`＝design-principles の Davie/2k6specs 定石と同型）。
- **限界（＝手書き品質に届かない核心）**: 固定カーネルは「**走査線ごとに要素構成（どの GRP/PF/M/BL を、どの色で、どの位置に出すか）が
  変わる**」を表現できない。bB は「全走査線同一命令列＋表の値違い」が天井で、これは design-principles の **76cy 非対称PF締切**
  （PF0再書込 cy31/PF1 cy38/PF2 ちょうど cy45）のような**走査線内のサイクル精密制御**＝手書きカーネルの本質を**捨てている**。
  → **固定カーネル単独では M3 の手書き品質には届かない**。ただし「ゾーンの中身は固定パターン」なら部分的に使える（§4-B）。

---

## 3. 8bitworkshop — codegen は無い（参考外と確定）[確認]

リポ: https://github.com/sehugg/8bitworkshop。VCS は **エディタ＋アセンブラ（DASM / batari Basic / cc65）＋エミュ（Javatari）**の
IDE であって、**VCS 向けの code generation も最適化も持たない**（README/構成上、presets はサンプルソースで生成器ではない）。
→ M3 codegen の参考価値は**低い**。bB を介する点で batari Basic（§2）に吸収される。スプライト→コード生成機能は無い。

---

## 4. 走査線ごとに要素構成が変わる「ゾーン分割カーネル」を codegen でどう吐くか

M3 の本丸。3つの生成戦略を比較する。

### 4-A. テンプレ穴埋め型（マクロ穴埋め・vcs-gm 流）
- やり方: カーネル全体を1枚の `*.asm.hbs` 固定スケルトンにし、`{{grpData}}`/`{{colorTable}}`/`{{pfData}}` 等の**データ枠だけ**穴埋め。
- 可否: **M3 には不足**。固定スケルトンは走査線内のサイクルレイアウトが1種類しか持てず、ヘテロな要素構成
  （上20行=P0+PF / 中40行=P0+P1+M0 / 下…）を**1テンプレで吐けない**。batari と同じ天井に当たる（§2-c）。
- 使い所: **ゾーン1個の中身**（同一構成が続く帯）には最適。

### 4-B. ゾーン合成型（テンプレ断片を「ゾーン記述」で組み立てる）★推奨
- やり方: シーンを **Y帯（ゾーン）に分割**し、各ゾーンの「要素構成（どの GRP/PF/M/BL/色を出すか）」に対応する**検証済みカーネル
  断片テンプレ**（design-principles の techniques＝zone_multiplex / two_line_kernel / score6 / bitmap48 …＝M4 テンプレ）を選び、
  **ゾーン列＝断片列として連結**。ゾーン間の遷移（RESP 再配置・VDEL 切替・HMOVE）は生成器が縫う。
- これは batari の **「ゾーン中身は固定パターン」**（§2-B 長所）と vcs-gm の **「中間表現→テンプレ穴埋め」**（§1）の**いいとこ取り**:
  - ゾーン内 = 固定断片テンプレ穴埋め（サイクル事前検証済み＝安全・手書き品質）。
  - ゾーン間 = 生成器が要素構成の差分を見て遷移コードを吐く（ヘテロ合成を担う＝新規部分）。
- design-principles と直結: **「ライン数を先に決め残予算で機能配分」「状態＝GameState＋状態別カーネル」**＝ゾーン列という中間表現が
  まさにこの設計法のデータ化。テンプレ＝技（spec の M4・design-principles「テンプレ＝検証済みカーネル技」）と1対1。
- 可否: **M3/M4 の本命**。novelty（ヘテロ合成）を**ゾーン境界の生成だけ**に局所化でき、ゾーン中身は検証済み資産を再利用。

### 4-C. 完全生成型（シーンから1命令ずつ最適スケジュール）
- やり方: シーンの要素・位置・色から、各走査線の 76cy 予算内で STA/LDA を**サイクル単位でスケジューリング**して全生成（テンプレ無し）。
- 可否: **理論上は最高品質**（手書きと同等以上）だが、76cy 制約下の命令スケジューリングは**サイクル正確なソルバが要る**＝
  M3 で着手するにはリスク過大。design-principles の非対称PF締切（cy45 ちょうど・nop1個で破綻）を自動満たすのは難問。
- 位置づけ: **将来（M4 以降）の最適化レイヤ**。M3 では採らない。ゾーン断片の**中身を後で 4-C に差し替える**余地として残す。

### 4-D. harness 検証（assert_line_budget / Gopher2600）との噛ませ方 [確認＝harness 既存能力]
- 各ゾーン断片テンプレは **サイクル予算メタ**を持つ（design-principles のフィジビリティ4軸：色/走査線/多重化/予算）。
- 生成 → `assemble_and_load`（dasm→load 一発）→ `assert_line_budget`（WSYNC 間隔超過＝roll原因を halt）で**ゾーン境界が 76cy を割らないか**を機械判定。
- 位置照合 = `read_tia` の `HmovedPixel`（可視0–159）/ 色 = `read_tia_registers` / 走査線 = `step_scanline`。
- **正本＝サーバ Gopher2600**（w3/w4 確定）。ブラウザ Stellerator は即時見た目専任。
- ＝**codegen の各ゾーン断片を「生成直後に harness で実走検証」できる**＝OSS に無い差別化（spec killer feature A）。
  ゾーン合成型は**断片単位で検証が閉じる**ので、合成後の破綻もゾーン境界に絞って二分できる＝デバッグが効く。

---

## 5. 推奨アーキテクチャ — シーン中間表現 → ゾーン合成カーネル生成

### 5-a. 二層の中間表現（vcs-gm の2系統分離を踏襲・粒度を走査線に下げる）
1. **要素グラフィック層**（spritemate fork 由来）: 各要素の `pixels:number[][]` + `colorPerRow:number[]`（w4 確定）。
   P0/P1=GRP 1byte/scanline、PF=40列→PF0/1/2 ビット順（CLAUDE.md 正典）、M0/M1/BL=幅・縦範囲。
2. **シーン配置層**（M3 新規＝コンポジット・キャンバスの出力）: 各要素の **TIA座標位置（clock/scanline）** と
   **Y帯（ゾーン）境界**。これが「走査線ごとの要素構成」を決める唯一の真実源。シーン＝JSON（spec のコード駆動 E）。

### 5-b. 生成パイプライン（ゾーン合成型＝§4-B を中核に）
```
シーンJSON
  → ① ゾーン分割器: scanline 0..191 を「要素構成が一定の帯」に切る（境界＝要素のY範囲の和集合の変化点）
  → ② ゾーン毎にテンプレ選択: 各帯の構成（出す要素集合）に対応する検証済みカーネル断片テンプレ（M4=技）を選ぶ
  → ③ ゾーン内穴埋め: 断片に grfx/PF/colorPerRow/位置データを注入（vcs-gm の matrix→bytes・reverse・%bin 流用）
  → ④ ゾーン間縫合: RESP/HMOVE 再配置・VDEL バッファ切替・色テーブル切替を生成器が吐く（新規部分）
  → ⑤ マスターテンプレに合流: VSYNC/VBLANK/Overscan 定型 + データ表ラベル（<name>_gfx/_col/_pf）+ ゾーン列本体
  → DASM .asm → assemble_and_load → assert_line_budget/read_tia で検証 → .bin → Stellerator.run() で即時プレビュー
```

### 5-c. どの方式をどの粒度で（結論）
- **ゾーン内 = テンプレ穴埋め型（4-A）**＝検証済み断片＋データ枠。サイクル安全・手書き品質を断片単位で担保。
- **ゾーン間 = ゾーン合成型の生成器（4-B）**＝ヘテロ合成の新規部分をここだけに局所化。
- **完全生成型（4-C）は採らない**（将来、断片中身の最適化差し替え余地として残置）。
- データ表は **vcs-gm 流に別ラベルで吐く**（`<name>_gfx`/`<name>_col`/`<name>_pf`、行頭 `.byte`、DASM `-f3`）。
- per-scanline color は **`LDA colorTable,Y / STA COLUPx`**（batari §2-b と Davie/2k6specs 定石＝design-principles を踏襲）。
- テンプレエンジンは Handlebars でも Go `text/template` でも可。**生成器本体を Go（harness 側）に置く**と検証（assemble_and_load/
  assert_line_budget）と同一プロセスで回せて噛みが最良。ブラウザ側は「シーン JSON を吐く」までに留める案が有力（要M3で判断）。

---

## 6. 未解決 / 要実機確認

- **[要試作] ゾーン境界の遷移生成**が最大の未知数。RESP 再配置＋HMOVE は HBLANK を 8clk 延ばす／HMOVE後24cy以内に HMxx 禁止
  （CLAUDE.md）等の制約があり、ゾーン跨ぎで**76cy を割らずに要素を入れ替える遷移コード**を生成器が安全に吐けるかは未検証。
  最初に潰すべき de-risk 点。
- **[要実機] 非対称PFを含むゾーン**の締切（PF0 cy31/PF1 cy38/PF2 ちょうど cy45、残29cy）を、他要素（GRP/色更新）と
  **同一走査線で共存させた断片**が成立するか＝design-principles の「45cy一点を外さず残29cyに収まるか」を生成断片で実機確認（assert_line_budget）。
- **[要設計] ゾーン分割の自動化**: 要素Y範囲の和集合から境界を切る規則（空Yレーン必須・多重化はY帯で・design-principles）を
  codegen に落とす際、フリッカ判定（5体超・重なり）まで自動でやるか、警告に留めるか未確定。
- **[要判断] 生成器の置き場**: ブラウザ（JS・vcs-gm 流）か harness 側（Go・検証同居）か。検証噛みは Go が最良だが、
  M5 即時プレビューの体験はブラウザ完結が良い。ハイブリッド（ブラウザ=シーンJSON、Go=カーネル生成+検証）が有力だが M3 で確定要。
- **[要確認] テンプレ断片カタログの初期セット**: M4 の技（zone_multiplex/two_line_kernel/score6/bitmap48/dyn_multisprite）の
  どれを「ゾーン断片テンプレ」として最初に用意するか。harness `docs/techniques/` の検証済み資産と1対1で棚卸し要。

## 7. M3 着手時に最初に作るべき最小プロトタイプ

**「2ゾーン・2要素」の最小ゾーン合成**を1本通す（縫合の de-risk が目的）:
1. シーン JSON＝「上帯: P0 のみ（per-row 色）/ 下帯: P0+PF（対称）」の2ゾーン1枚。
2. ゾーン分割器（境界1本）＋ ゾーン内テンプレ穴埋め（既存検証済み断片2種）＋ **ゾーン境界の遷移生成1箇所**（色/PF有効化の切替）。
3. マスターテンプレ（VSYNC/VBLANK/Overscan 定型）に合流 → DASM。
4. `assemble_and_load` → `assert_line_budget`（両ゾーン 76cy 内）→ `read_tia`（P0位置・色）→ `step_scanline` で境界の走査線確認。
5. `.bin` を `Stellerator.run()` でブラウザ即時プレビュー（無改造・w4 確定）。
→ これが通れば「ヘテロ合成の核（ゾーン縫合＋断片穴埋め＋実機検証）」が成立。以降は要素種別とゾーン数を増やすだけ。

---

### 報告用1行結論
- **M3 codegen 推奨アーキ**: 「シーンを Y帯（ゾーン）に分割し、ゾーン内＝検証済みカーネル断片のテンプレ穴埋め／ゾーン間＝
  生成器が RESP・HMOVE・色切替を縫う **ゾーン合成型**」。固定カーネル（batari）でも完全生成でもなく、両者の中間で novelty を
  ゾーン境界生成だけに局所化し、各断片を harness（assert_line_budget/Gopher2600）で実走検証する。
- **最大の不確実性**: **ゾーン境界の遷移生成**（RESP再配置＋HMOVE＋VDEL/色切替を、76cy を割らず・HMOVE後24cy制約を守って
  安全に吐けるか）が未検証。§7 の「2ゾーン最小プロト」で最初に潰すべき de-risk 点。
