# Casebook — 状況 → 技（実在の市販ゲーム逆アセンで裏打ちした事例集）

`cookbook.md` が「**作りたいゲーム型 → 標準レシピ**」（前向き）なのに対し、本書は「**実在の市販ゲームが、ある状況をどの技で解いたか**」の**事例カタログ**（逆向き・エビデンス駆動）。各エントリは **状況 → 採用技 → なぜそれが効くか → 出典（マニュアル＋逆アセン著者）** を持つ。著述ループ（`authoring-protocol.md`）の retrieve で「この状況、実ゲームではどう解いている？」を引くための索引。

> 本書は**読んで学ぶ**（受動）。**手を動かして学ぶ**（能動＝実ゲームを1メカニクスずつ自分で再現し実ROMに数値照合）は対の `build-to-learn.md` を参照。

> **作り方（3層ケーススタディ）**：商用ゲームを「マニュアル(spec)×逆アセンブル(impl)×Claude再構築(rehearsal)」で学び、**再構築と実装の差分＝能力ギャップ**を一般化散文として昇格する（[[project-casebook-3layer]]）。**クリーンルーム厳守＝逆アセンのコードは転載せず、一般化した散文と出典のみ**。生ペアリングは非リポ `reference/disassemblies/<game>/` 配下の `_casestudies/`。
>
> **Layer1 spec はマニュアルだけで足りない＝実プレイ（実ROM観察）を必ず足す**：マニュアルは目的・操作・スコアの「意図」は書くが、画面の実際の動き・物量・描画 craft・手触りは伝えない。`load_rom`→`step_frame`→`get_screen_annotated` で挙動を観察し spec に足す（Fishing Derby では魚=1行1体・斜め釣り糸・水面シマーが全て実走観察で判明＝マニュアルの穴）。[[feedback-verification-standard]]。

## 索引
| ゲーム | 年/設計 | サイズ/型 | 状況→技 エントリ |
|---|---|---|---|
| Fishing Derby | 1980 Activision / David Crane（逆アセン Dennis Debro） | 2K / 単画面スポーツ | 大型不定形・斜めの線・多ターゲットのオブジェクト経済・対向スコア・同種衝突・無コスト演出 |
| Breakout | 1978 Atari / Brad Stewart（逆アセン Dennis Debro） | 2K / 単画面アクション | **build-to-learn 初実装**（[[build-to-learn]]）＝自作で「書けた」技：多領域PFカーネル・RAM駆動の破壊可能PF・BL/P0位置決め・キーパドル・位置ベース衝突・サーブ/残球のゲーム状態 |
| PONG | 2026 in-house capstone | 4K / 単画面スポーツ | 対戦AIの4パラダイム（倒せる設計込み）・不完全さの調律（誤差/遅延）・排他パスの共有末尾スキップで予算捻出・**AI強さの非推移性（単一基準ベンチ≠総当たり）** |
| Combat | 1977 Atari / Larry Wagner (disasm Roger Williams) | 2K / 27-variant 2-player | PF-only dual score (players are tank-only) · multi-frame wall-normal bounce solver · stir hit-reaction state machine · 27-variant bit-packed config selector + DDR input-gating |

## Breakout（Atari 1978）— build-to-learn の worked example（「書けた」技）
`build-to-learn.md` の手順で、マニュアル＋逆アセン＋実ROM寸法スペックから**自作で8段（rung1-8）を実装し遊べる1人用 Breakout を完成**。各段は実ROMに数値照合。＝「説明できる」が「自分で書ける」に変わった実証。出典＝`reference/disassemblies/_casestudies/breakout/`（impl-map/fixtures/method-diff/layout-compare）＋`roms/breakout/`（自作ROM・steps/に各段スナップショット）。

- **状況：単画面に複数の縦領域（スコア帯／ブロック帯／プレイ域）** → **領域ごとに COLUBK/PF の役割を切替える多領域カーネル**（壁＝PF端 or COLUBK、ブロック＝PF＋行COLUPF）。
- **状況：壊せるブロック壁** → **PF値をRAM(`brkPF0/1/2`)に持ち、毎走査線リロードして描画。当たったビットをクリア＝破壊**（反射PFはミラー破壊の簡略／非対称PFが上位）。
- **状況：1px のボール／横長パドルを任意X位置に** → **÷15 coarse(RESxx)＋HMxx fine を1 HMOVEで複数オブジェクト同時**位置決め。レンダ位置＝RAM値−offset（read_rowで実測較正）。
- **状況：操作（キー）** → SWCHA でパドル±、INPT4 でサーブ。
- **状況：衝突** → 今回は位置ベース（学習用に `CXBLPF` ハード衝突への置換を method-diff に記録）。
- **状況：ゲーム進行** → `ballLive`(サーブ待ち/プレイ)・`lives`(5球)・`gameOver` の小さな状態機械。
- **★craft 較正の作法**：色も寸法も **read_row/get_screen_annotated でオリジナルに数値収束**（目測だけだとパドルを「白24px」と誤認→実測「赤16px」）。[[build-to-learn]]・`layout-compare.ja.md`。

## 次ゲームの選定基準（scale-up）
合成難易度の**昇順**で keystone を増やす。選定キー：
1. **ROMサイズ／バンク切替（主・機械判定）**：2K → 4K → バンク切替（8K+）。dissect 注釈も 2K/4K が綺麗。
2. **アーキタイプ**：単画面 → 迷路/スポーツ/固定シューター → スクロール/多画面 → プラットフォーマー/3D。
3. **マニュアルの詳しさ（spec の濃さ・ユーザー指摘 2026-06-15）**：説明が厚いゲームほど Layer1 spec が濃く、diff が豊かになる＝学びが大きい。例＝**Asteroids は説明が非常に厚い**（ただし 8K バンク切替＝バンク keystone 帯）。マニュアル濃度は良い選定信号だが、難易度昇順（1）と両立させる。
4. **逆アセンの入手性**（完全注釈・実ROM一致が望ましい）。
- **必須工程**：どのゲームでも **Layer1 spec＝マニュアル＋実ROM観察の二本立て**（[[feedback-verification-standard]]）。マニュアルだけでは挙動を把握し切れない。

---

## Fishing Derby（Activision, 1980, David Crane）
出典＝マニュアル `reference/disassemblies/_casestudies/fishing-derby/manual/`（archive.org 原本）＋ Dennis Debro 完全注釈逆アセン（実ROM完全一致）。検証＝`build/fishing_derby.bin` を Gopher2600 実走（2026-06-15）。

- **状況：8px より大きい単体の不定形クリーチャを出したい（魚より大きいサメ等）**
  → **1個の player を per-走査線 NUSIZ（サイズ/コピー）＋HMOVE テーブルで“引き伸ばして”成形**。GRP は小さいまま横 ~40clock の不定形になる。色は単色割り切り。**フリッカも追加オブジェクトも不要**。実走で確認。→ 原則 `design-principles.md`「8px 超の単体不定形」。

- **状況：2点を結ぶ動く細い斜め線（釣り糸/テザー/ロープ/レーザー）が要る**
  → **missile/ball を縦に出し、slope を整数+分数で持って毎走査線 `adc`、桁上りで `HMMx`/`HMBL` に ±1px HMOVE**（Bresenham を HMOVE で実装）。右糸=BL・左糸=M1。実走で傾きを確認。→ 原則「任意傾きの1px 直線」。

- **状況：単画面に多数の同種ターゲットを置きたい（"6 rows of fish"）**
  → **「1行1体 × バンド再利用」が定石**。全6匹を P0 を7バンドで使い回して1匹ずつ描画し、**操作対象（掛かって巻上中の魚）は P1 専任**に分ける。マニュアルの名詞（"rows of fish"）から同時表示数を過大に見積もらない＝**先に「TIA 6枠×バンド再利用で実際に置ける数」を算出**してからゲーム性を当てる。〔Claude 再構築は「群れ＝P0×3+P1×3」と誤認→実走で反証〕

- **状況：左右対向の2桁スコア×2**
  → **player2枠（P0=十/P1=一）＋数字フォント表＋1走査線内 re-strobe（`Waste18Cycles` を挟み GRP 描き直し）**。PF score は単一/左右対称向き、対向2スコアは player 方式。既存技 `docs/techniques/score-kernel.md` に該当。実走で「スコア行の PF にフォント無し＝数字は player」を確認。

- **状況：同種2オブジェクトの接触判定（ハザードが標的を奪う）**
  → **`CXPPMM`（P0×P1）一発**。座標計算に逃げない。サメ(P0)×掛かり魚(P1)接触を検出して魚を消す。

- **状況：背景に安い“ゆらぎ”の質感（水面/砂嵐/星）**
  → **既に回している LFSR/randomSeed のビットを帯ごとに `COLUBK` へ流すだけ**（専用RAM不要・ほぼ無コスト）。→ 原則 色節「背景のゆらぎ」。

### このケースが定量化した能力ギャップ（→ `capability-gap-audit.md`／`roms/EVALUATION.md`）
Claude の封印再構築 vs 実装の差分＝**衝突・入力・難易度の“ロジック”は読めるが、TIA を絞り切る描画 craft（1スプライト成形・斜線・多重利用・無コスト演出）で実ベテランに劣る**。詳細台帳＝`reference/disassemblies/_casestudies/fishing-derby/diff-gaps.ja.md`。

---

## PONG（in-house capstone, 2026）— 対戦AIの4パラダイムと「強さ」の測り方
市販ゲーム逆アセンでなく、**自作 capstone（1枚画像→完成PONG）で実測裏取りした gameplay 事例**（Breakout 同様「書けた」側のエントリ）。4変種は本流から**AIコードのみ**差替（物理/english/サーブ/音/スコアは完全同一＝純粋比較）・全本で実機予算検証済（全物理行≤76cy・900f over:false）。出典＝`sandbox/practice/pong/ai-variants/`（README＝4種設計・`bench/README.md`＝客観ベンチ＋総当たり実測・2026-07）。

- **状況：倒せる対戦相手AI（パドル系）が要る**
  → 古典PONG-AIの**4大パラダイム**から選ぶ。鍵＝**攻略口は後付けでなく設計入力**（各型に構造的な負け筋を残す）：
  | 型 | 仕組み | 攻略口（designed beatability） |
  |---|---|---|
  | 追従 tracker | 現在Yを遅延追従（8fに1回再サンプル・3px） | 再サンプルの一拍遅れを速球・角度変化で抜く |
  | 予測迎撃 predictive | 影ボール（実ボールの2倍速で前進＋**壁反射込み**）で着弾Yを確定→先回り待機 | 着弾確定後の角度変化（english/WHAMMY急球）・注入した誤読（1/16で逆読み） |
  | 先読みリード anticipatory | 線形外挿 target=BallRow+4×BallDY・毎f連続2px（滑らか） | **壁を読まない**＝バウンド球に見当違いの先行 |
  | ラバーバンド rubberband | スコア差で**誤差幅**を変調（負け=締める/勝ち=甘い）・速度2px固定 | リードすると緩む＋人間の瞬間速度優位（WHAMMY±3 > AI2px） |
- **状況：AIの「らしさ」＝不完全さを調律したい**
  → 速度でなく**誤差と遅延**で作る：狙い誤差 AIErr の注入（生成は余裕のある行へ移設）・反応遅延（再サンプル間隔）・速度上限。難易度可変は「**誤差幅の変調**」（速度変調は見た目でバレる・誤差変調はバレにくい＝ゲームAIの定石）。
- **状況：カーネル予算が足りず精度を上げられない**
  → **排他パスで不要な共有末尾をスキップして予算を捻出**：v2 は予測フェーズ中パドルが動かない＝共有末尾 PaddleR_End 再計算（10cy）が不要→`jmp OverEnt` で housekeeping 行へ直行。**浮いた10cyで影ボールの傾きを近似（X速3固定）→正確（2×BallDX＝全速度で正確）へ強化**。design-principles「物理行の間借り」の変種＝パス固有に不要な共有処理を見つけて省き、浮いた分を精度に回す。
- **状況：AIの強さを測りたい（評価・バランス調整）**
  → **単一基準ベンチは1つのレンズにすぎず、順位を正反対に誤り得る（実測）**。固定基準AI（決定論1px追従）相手の11点先取では v4 11-0／v1 11-1／v3 1-11／v2 1-11＝「v4>v1>v3≈v2」。だが**総当たり（head-to-head・AIを左パドルへ移植し左右反転で side bias 排除）の真実は v3≈v4 ＞ v2 ＞ v1**——v1 は 0勝3敗の最弱（8f再サンプル遅延を実AIに突かれる）・v3/v4 は 37000f 走らせても 0-0 の完全膠着。基準を混ぜると **v1→基準→{v2,v3}→v1 の循環＝強さは非推移（ジャンケン）**。さらに**客観ベンチ≠人間相手の難易度**：v3 は完璧追従の基準には 1-11 で「弱」だが、追従が不完全な人間には「中・なんとか勝てる」＝本流採用。教訓＝**AI の強さは相手依存で一次元でない。真の評価は総当たり＋多様な相手モデル**（→ harness backlog の gameplay-verification フロンティア）。

---

## Combat (Atari, 1977, Larry Wagner) — dual score with zero player slots, multi-frame bounce solving, a bit-packed 27-variant shell
Source = the disassembly `reference/disassemblies/.../Combat.asm` (Larry Wagner original / Roger Williams 2002 annotation), read as the casebook "disassembly-reading" layer; distilled in `sandbox/studies/combat/comparison-structure-vs-original.ja.md` + `diff-gaps.ja.md`. Clean-room: generalized prose + routine names only, no code transcribed. Cross-checked against the self-built `combat_mine`.

- **Situation: two independent 2-digit scores, but BOTH players (P0/P1) are permanently occupied by the moving tanks**
  → **draw both scores from the PLAYFIELD, never from a player.** Combat sets `CTRLPF #$02` (SCORE mode, NOT reflect) so the left half takes `COLUP0` and the right half `COLUP1` = two independently-colored scores out of one repeated field, carried by a SINGLE `PF1` register **recycled** across the kernel (the previous line's `NUMG` is computed while `PF1` is reused), `PF1` write pinned to cycle 9. Digit glyph offset = `SCROT` turns each BCD nibble into a ×5 index with `ASL`/`ASL`/`ADC` (clean NMOS BCD nibble), 5-line font. **The object-slot decision follows "what permanently owns the screen": tanks own P0/P1 → score goes to PF; PONG's thin paddles let P0/P1 double as score (`docs/techniques/score-kernel.md`).** Contrast: Combat = SCORE-mode per-half coloring + one recycled PF1; `docs/techniques/asymmetric-pf-score.md` = repeat-mode, PF1+PF2 twice/line for 4 fields. 〔`SCROT`/`NUMG0/1`/`CTRLPF #$02`/`NUMBERS`; diff-gaps GAP-2, comparison §2.5〕
- **Situation: a bullet must reflect off a maze wall, but the TIA collision latch reports only "hit playfield" — never WHICH wall face**
  → **reconstruct the wall normal over multiple frames with a trial-and-error state machine (`MxPFcount`), zero geometry math.** frame0 try vertical reflect → frame1 (still inside) flip 180° = horizontal → frame2 wait (grace to clear) → frame3+ corner reflect, held until the bullet escapes; each frame re-observes "still in the wall?" and picks the next hypothesis — the missing hardware fact (which face) is recovered by hypothesis-testing across time. `COLcount` ignores contacts under ~4 frames (a resting tank isn't shoved). **Lesson: when the hardware under-reports, replace the one-shot formula with an observe-then-choose state machine that spends frames to disambiguate.** 〔`MxPFcount`/`COLcount`/`BounceCount`/`CXM0FB`; diff-gaps GAP-4, comparison §2.6〕
- **Situation: the "got hit" reaction needs game-feel, not just a blink**
  → **a dedicated `StirTimer` state machine seizes the loser's tank**: spin it like a top with `INC DIRECTN` every frame, decaying explosion via `BoomSnd` (AUDV/AUDC/AUDF ramp-down), knockback via `RushTank`; the winner's engine sound is silenced. Score-on-hit seeds `StirTimer` with the loser's ID. **The hit reaction is its own mode that suppresses input — even a 1977 title invests in feel; don't model "hit" as a mere flash + respawn.** 〔`StirTimer`/`DIRECTN`/`BoomSnd`/`RushTank`; diff-gaps GAP-6, comparison §2.6〕
- **Situation: one 2K ROM must be 27 game variations toggled by the console switches**
  → **one bit-packed descriptor byte per variation → fast flag bytes; gate joysticks through the port DDR.** `GSGRCK` reads `SWCHB` (RESET=new game; SELECT+debounce=advance, wrapping to game 1); `VARMAP` holds one packed byte per game, fanned out into 4 flag bytes (`$82-$85`: PF_PONG/GUIDED/BILLIARD/GAMSHP) tested with cheap `BIT`; `ClearMem` reuses ONE wrap loop for 4 erase ranges by re-seeding X. Notably **`GameOn` is written into `SWCHB`'s DDR to physically gate joystick input** (enable/disable by making the port drive rather than read). **Steal when adding variants: one bit-packed descriptor per config → fast BIT flags, plus DDR-gating for hardware input on/off.** 〔`GSGRCK`/`VARMAP`/`GAMVAR`/`ClearMem`/`SWCHB` DDR/`GameOn`; diff-gaps GAP-E, comparison §2.8〕
