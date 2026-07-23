# Atari 2600 ビジュアル設計原則（design-principles）

採掘（AtariAge）＋web 研究で得た「ルール化できる作画設計の原則」の正典。用途＝(1) Claude のデザイン判断の
明示ルール（roms/EVALUATION.md の⑥craft）(2) `pkg/design` フィジビリティ判定の根拠（凍結した TIA Studio テンプレにも流用可）。
詳細出典＝`tools/research-w2-design.md` ＋ `docs/mining-digest.md`（採掘スレ索引）＋ `reference/atariage/*/notes.ja.md`。

**実行可能な判定は `pkg/design/` に「吸収」済み**（asm を書く前に機械チェックする）。数値化できるルールは
末尾に `→ func` で対応関数を示す。数値化できない判断系は末尾「機械判定不能な判断ルール」節に集約する。
対応: 色帯/グラデ/混色=`color.go` ／ 横位置=`position.go` ／ PF窓・score2色・スクロール=`pf.go` ／
多重化=`multiplex.go` ／ 文字数=`text.go` ／ 予算=`budget.go` ／ 作画craft=`craft.go`。

## 色（最重要）
- **色は RGB でなく「レジスタ値／象徴名（hue 上位ニブル × lum 下位ニブル）」で持つ**。生 hex をばら撒かない。
  輝度は実質 8 段（bit0 無効）。PAL/NTSC は 1行で切替できる二系統（N_xx/P_xx）を設計目標に。〔Davie S11, symbolic-color-names〕
- **多色は「縦に足す」＝走査線ごとに COLUPx 書換え**（横は1色）。**横多色は高い**（PF score/Chronocolour/フリッカー/重ねの擬似のみ）。〔Hugg, Davie S21〕
  - **横多色の色帯 最小幅 = ストア命令サイクル × 3px**。PF 整列の色は 4色クロック（=12px, `STx.w`）の倍数、任意色は ~6cy/帯（1行で約8帯が上限）。SP（`txs`/`tsx`）を 4本目の色レジスタに流用する手も。〔170018 multiple-colors-per-scanline〕 `→ design.MinColorBandWidthPx/CheckColorBands`
- **「唯一正しい RGB」は存在しない**：Stella は YIQ 動的生成、同一レジスタ値でもエミュ/設定で十数〜0x20 差。
  我々は実走テーブル `internal/ingest/palette_stella.go` が正（Stella 照合100%）。〔rgb-color-values, 118495〕
- **hue↔色の地図**：hue1=黄 / hue4=赤 / hue8=青 / hue12=緑（hue15≈hue1）。黄は hue1 が定石。〔132561〕
- **高輝度ほど彩度が落ち白っぽくなる**（特に明るい青は識別が消える）→ **鮮やかに見せたい色は中〜低輝度**で置く。彩度と輝度はトレードオフ。〔132561〕 `→ design.Hue/Luminance/WashoutRisk, HueName, GradientSameHue, InterlaceColorsSafe`
- **色のデータモデルの最小単位は「走査線ごとの色」＝`colorPerRow[]`**：単一 `color` でなく走査線インデックス→COLUPx 値の配列で持つと、縦多色（最も安い多色化）がそのまま表現できる。TIA Studio M1 の設計判断もこれに収束した。〔研究 w4 / `tools/research-w4-m1-open-questions.md`〕
- **背景の「ゆらぎ/ノイズ質感」は乱数シードのビットを `COLUBK` に毎走査線流すだけ（専用RAM不要）**：水面のシマー・砂嵐・星の瞬きなどを、既に持っている LFSR/randomSeed のビットを帯ごとに `COLUBK` へ転記して**ほぼ無コスト**で出す。〔Fishing Derby `.colorWaterShimmer`＝randomSeed のビットを per-line COLUBK に流す水面演出〕

## スプライト（P0/P1）
- 8 ドット幅・1 レジスタ（GRP 8bit, MSB=左端）。幅 NUSIZ 1x/2x/4x。〔2k6specs, Davie S21〕
- **横位置 = 2段階**：粗 ÷15（5cy ループ）→ 微 HMOVE。粒度 3px/CPUサイクル（litmus 一致）。〔Davie S22〕 `→ design.PositionSplit/CoarseIterations`
- **★RESxx の内部描画遅延（位置照合の第一容疑）**：`RESxx` ストロボはカウンタを即リセットするが、**オブジェクトが実際に描き始めるのは遅れる＝player +5 / missile・ball +4 カラークロック**（RESP0 が cycle46 で終わると X≈75）。Stella ソース(`renderCounterOffset`)で実証。**「目標 X が ~5px ずれる」時はまずこれを疑う**。RESxx の粒度は 3 color-clock。〔採掘 294398, 283075, 305780, 172089, 137739, 329611, 304182〕（codified `X=3N−54/55` を裏で説明する量＝**litmus_pos で実測照合**してから定数化）
- **多オブジェクトの位置式と書込み窓**：可視中の `RESxx` は禁止（即時リセットで曲がる）＝HBLANK/前ラインで先読み。共有ループは consecutive な `RESP0,x`/`HMP0,x` を `DEX/BPL` で回す（`design.shared_setxpos` 実装済）。右端の溢れ限界は X≈134（「N オブジェクト=N+1 走査線」が真因）。〔採掘 67045, 308513, 340965, 311795（RESxx×HMOVE レース＝Gopher2600 実装済）〕
- **1サイクル潰しで RESP を狙った位置に**：粗位置決めで NOP が無い時、`sta.wx HMP0,x`（dasm `.w`/`.FORCE` で Absolute,X=5cy を強制／ZP,X は4cy）で1cy 足して RESP0 ストロボを**狙ったサイクル**に合わせる。〔採掘 blog SpiceWare 12538〕
- **マスク式スプライト描画＝21cy**（DoDraw の26cy より安い）：`lda (img),y / and (mask),y / sta GRPx / lda (color),y / sta COLUPx`＝**形のクリップと行毎色更新を同時に**。0パディングでサイズ違いを共有マスク1個に。〔採掘 blog SpiceWare 10890, 339509〕
- **div15 の微動レンジは実装依存**：素朴な div15 は **−6..8px**、`eor #15`＋`adc #((8+1)<<4)` で対称化すると **−7..8px**。起源は Decuir/Video Olympics。HMOVE 生のハード可動域(−8..+7)とは別＝ルーチンの性質。〔採掘 286698〕（要 litmus 裏取り）
- **early-HMOVE（WSYNC 前 HMOVE）の「動かさない」値 = HMPx $80（=8）であって $00 ではない**：$00 にすると同一走査線を跨ぐオブジェクトが 8px ドリフトする。15px×11 の専用カーネルで位置決めする型。〔採掘 169471〕（要 litmus）
- **HMOVE をサイクル 73–74 で撃つと左端のコーム(黒線)が出ない**：Cosmic Ark 系の既知トリック。Omegamatrix の間接ジャンプ位置決め＝HMPx 下位ニブルがジャンプ索引を兼ねる。〔採掘 165428, 183219, 319456 "HMOVE Shuffle"〕（要 litmus）
- **48px** = NUSIZ$03（3 copies）＋ P1 を 8px 右 ＋ VDEL 二重バッファで GRP 時間差差替え。score/bitmap48 を転用。〔48px-positioning〕 `→ pkg/sprite.SplitWide/NUSIZ・design.MaxChars(Text48px)`
- **絵を先に決めて割当しない**。順序＝色予算→割当表→不足は「色共有・オブジェクト兼用・レイアウト変更」で交渉。
- missile/ball = 線・縁・縦枠、player = 倍幅/複数コピー/4x で面。1つの見た目を複数オブジェクトの重ねで構成。
- **8px 超の単体不定形は「1 player を per-走査線 NUSIZ＋HMOVE テーブルで成形」＝フリッカに逃げない**：GRP は小さいまま、各走査線で NUSIZ（サイズ 1/2/4/8・コピー数）と HMOVE を切替えると、1つの player を横 ~40clock 級の不定形（魚/サメ/船/横長クリーチャ）に“引き伸ばせる”。色は単色割り切り。追加オブジェクト/フリッカ不要。〔Fishing Derby (David Crane/Debro逆アセン) SharkTraveling*NUSIZValues＝per-line NUSIZ+HMOVE のサメ・build/fishing_derby.bin 実走で ~40clock 幅を確認〕 `→ casebook.md「大型不定形」`
- **任意傾きの1px 直線は missile/ball ＋ 分数HMOVE累算（Bresenham in HMOVE）**：縦に出した M/BL を、毎走査線 slope を整数+分数で `adc` し桁上りで `HMMx`/`HMBL` に ±1px HMOVE すると、釣り糸/テザー/ロープ/レーザー等の**斜め線**になる（縦・横しか出せないという思い込みを捨てる）。〔Fishing Derby fishingLineSlope(Integer/Fraction)＋HMOVE・右糸=BL/左糸=M1、実走で右糸の傾きを確認〕 `→ casebook.md「斜めの線」`

## プレイフィールド
- 横 40px × 4clk/px。表現力は縦のリズムで稼ぐ。〔Davie S13〕
- **PF スクロール背景は「盤面RAM＋表示バッファ＋delta更新」3層**＋タイル単位スクロール（tearing 回避）。鉄則＝**総スキャンライン数をフレーム間で一定に保つ**（PAL は偶数必須・安全圏 262/264）。スクロール帯は上下 10〜16 ライン。〔200972 tile-scrolling-engines, Boulder Dash 型〕 `→ design.ScrollScanlinesConstant`
- **HUD/テキストは「出す文字数」で技法が決まる**：48px=12字 / venetian blinds=32字（ただし 3px 幅専用）。HUD は画面モード分離 or ゾーン隔離＋スコア枠の多目的再利用。〔197162 text-hud〕 `→ design.MaxChars`
- **非対称 PF は高コスト**（走査線途中で PF0/1/2 を2回書く、PF0 窓は ~20cy のみ）。妥協＝中央32px/1行おき+倍高/venetian/RAM自己書換。〔Davie S17, castlevania-port〕
- **非対称PFの書込み締切（実測サイクル）**：左半分を表示しつつ同一走査線で右半分へ書き換える時、書込みは「その PF が**もう見えていない**瞬間」を狙う。古典カーネルの実値＝
  1回目 PF0[cy7] / PF1[cy14] / PF2[cy21]（左半分用、可視前に間に合わせる）→ 右半分用に
  **PF0 再書込み cy31 / PF1 cy38 / PF2 は“ちょうど cy45”**（早くても遅くても崩れる＝nop 1個追加で破綻）。
  残り 76−47≒**29cy/line がスプライト等の自由予算**。横多色PFの実現性は「この 45cy 一点を外さず、かつ残29cy に他処理が収まるか」で判定する。〔Williams/Saunders "Asymmetric Reflected Playfield" tutorial〕
- **タダで2色PF＝CTRLPF D1（score bit）**：bit1 を立てると **左半分PF=COLUP0・右半分PF=COLUP1** で独立色になる（非対称書込みタイミング不要）。スコア表示の定番だが、背景の左右色分けにも使える安価な2色化。〔w11/Asym2scrol〕 `→ design.ScoreModeTwoColor`
- **PF0 を捨てる節約トレード**：上部プラットフォーム等で PF0 を描かないと、代わりに PF2 を **cycle 48 ちょうど**で書く必要が出るが、**12cy/line ＋ RAM 18バイトが浮き**、プレイヤーが画面両端で落下できる（反射PFの利点）。〔採掘 blog SpiceWare Stay Frosty〕
- **縦移動プラットフォームは2ゾーン相補高さ**：上下の帯の高さを「片方が伸びたら他方が同量縮む」で組むと総ライン一定＝画像が安定（不一致だとモーションブラー）。〔採掘 blog SpiceWare〕
- **PFレジスタ書込みの可視遅延**：PF0/PF1/PF2 への `sta` は**2〜3カラークロック遅れて**反映される（色レジスタは即時）。反射PFの中央境界は**ちょうど cycle 48** で完了させる（採掘 149228 の実測＝line 38 の cy45 締切と整合・クローン機は+1cy）。横多色PFのタイミング判定はこの遅延込みで行う。〔採掘 149228 PF書込みタイミング表〕

## 多重化・フリッカー
- 2体超は Y 帯で多重化、横再配置は1走査線消費、**空 Y レーン必須**、代償 30Hz ちらつき。〔Bumbershoot〕 `→ design.NeedsFlicker/NeedsEmptyYLane/RepositionCostScanlines`
- **走査線中の GRP 書換で1スプライトを多数化**：1本の player を NUSIZ で複製し、各コピーの描画直前に `STA GRPx` し直すと**全コピーを別絵に**できる（Space Invaders の編隊・6桁スコア・多彩エネミー列の共通基盤）。`STA GRPx` は HBLANK 内厳守。〔採掘 337131, 182923〕
- **マルチカーネル＝1オブジェクトを区画ごとに再利用**：Y 帯ごとに `REFP`/位置/絵を切替えて1個の player を別用途に使い回す（Stay Frosty）。「同一ラインに重ねない」配置制約と「占有列に入らない」AI を一致させればフリッカ0。〔採掘 303364, 318140, 164247〕
- **フリッカは最終手段／短命限定**。大面積禁止。エミュを信用せず複数フレーム合成で検証。〔flicker-to-enhance-graphics〕
- **>2体フリッカの2アルゴリズム**：(a)**年齢ベース**（各物体の表示回数を数え、最も古い物を次に出す）／(b)**リスト並べ替え**（SHOWN/NOT_SHOWN を毎フレーム merge）。**両プレイヤーを総フリッカに使えば最大~24体**（Frantic 実例）。`flicker_multiplex`/`dyn_multisprite` の上位設計。〔採掘 blog SpiceWare 10777, 11656〕
  - ただし**意図的フリッカを演出に転化**する手もある：点滅する目標物で Game&Watch 味を出しつつ、可動5スプライト制限を回避＝欠点を美観に変えた実例。〔Pizza Boy 329673〕
  - **時間混色（2フレーム交互色）を使うなら、両色を同一輝度にして hue だけで分ける**＝フリッカー知覚は輝度差に比例するので激減する（例 lum4 の黄緑 vs 青緑）。〔176987 interlaced-multicolor〕 `→ design.InterlaceColorsSafe`
  - **フリッカの見え方は輝度で調律する**：黒背景なら輝度 `$x4〜$x8` が安全圏（既定 `$38`）。同一輝度・異hue でちらつき最小化（上の則と整合）。264 ライン運用＝PAL は偶数必須。〔採掘 162521 StayFrosty〕

## カーネル予算・状態
- **76cy/line が天井**。ライン数を先に決め残予算で機能配分。〔splendidnut〕 `→ design.LineBudget/RemainingCycles`
- **★RIOT 6532 タイマのラップアラウンド・バグ（"Stella は通る／実機はロールする"トラップ）**：タイマがラップアラウンドする**まさにそのサイクル**に `TIM64T`/`TIM1024T` を書くと、分周器が静かに **1T** に化けてフレーム長が崩れ実機でロールする。**対策＝二重書き（double-write TIM64T）**。エミュ依存で見逃しやすい＝harness の中核ミッション(gap B)直撃。Gopher2600 作者(JetSetIlly)がこのスレで診断。〔採掘 303277 "To Roll or not to Roll"〕（harness 強化候補＝ラップアラウンド・サイクルでのタイマ書込みを検出する assert）
- **縦配分の硬い下限と失敗モードの非対称**：総スキャンライン数を一定に保つ前提で、各区間の下限＝VSYNC≥3 / Overscan≥3 / VBLANK≥15（PAL は偶数）。**Overscan を伸ばし過ぎ＝無表示／VBLANK を伸ばし過ぎ＝jitter** と失敗の出方が違う＝余りは Overscan でなく VBLANK 側で吸収しない（jitter源）。〔採掘 171270〕
- **WSYNC のセマンティクス**：`sta WSYNC` は**次の HBLANK 先頭**（68カラークロック＝22⅔ CPUサイクル）までCPUを止める。レジスタ更新遅延（色=即時／PF=2-3clk／VBLANK=+1ライン／音長=遅延）を踏まえて書込み位置を決める。〔採掘 192183 レジスタ更新遅延表〕
- 状態＝1個の GameState 変数＋状態別カーネル。タイトル絵は上下パディング＋中央PFテーブル、終端で GRP/PF クリア。〔title-to-game-transition〕
- 省サイクル＝ISC/ISB 非公式オペコード＋SP をラインカウンタ流用（要 litmus 裏取り）。〔5cycle-color-cycling, illegal-opcodes〕
- **違法(非公式)オペコードの安定性マップ**：実機で**安定して使えるのは LAX/SAX/SBX/DCP/ASR(ALR) 系**（48px/dyn_multisprite で既使用）。**LXA/XAA は不安定＝使うな**（チップ個体/温度依存）。opcode レベルのコード生成はこの可否表で gate する。〔採掘 168616 illegal-opcode stability〕
- **資源トライアングル＋レジスタ規約**：RAM(128B)／CPU(76cy)／ROM は相互排他＝1つを増やすと他が減る（＋人的コスト）。Thomas Jentzsch 規約＝カーネル中は **Y=走査線兼スプライト index・X=PF・A=その他** に役割固定すると速い。サブルーチンは「コード再利用」目的のみ（呼出コストが高い）。〔採掘 146817〕
- **カーネル用語の正典**（Andrew Davie）：「**N-scanline kernel**（1 絵行が N 走査線）」＋形状4軸＝スプライト間隔／PF 間隔／対称性(sym/asym)／反射(mirrored)。harness 内部のカーネル語彙としてこれを採用。〔採掘 320714〕
- **動き＝固定小数点サブピクセル**：位置を 8.8 固定小数で持ち `vel` を毎フレーム加算→キャリーで整数移動＝滑らかな低速・摩擦・重力・風を1枠組みで。**放物線＝X 等速 × Y 等加速**（三角関数不要）。敵追尾は `(target−pos)/16` の符号シフトで比例ホーミング（除算なし・16方向は octant＋傾き閾値）。〔採掘 178177, 270373, 107024〕（technique-candidate ㉕）
- **符号付き小量の ×2^n は `asl` 連打（掛け算不要・符号保存）**：2の補数は `asl` がそのまま ×2 なので、BallDY のような符号付き速度は `asl`×n で ×2^n（例：先読み target = BallRow + 4×BallDY ＝ `asl` 2発＋`adc`）。ただし (a) **結果の値域が広がる→bit7 が符号判定に使えなくなる**＝クランプ/折返し判定は値域で行う（外挿目標が最大~190 なら閾値は `cmp #220`。known-traps「bit7クランプ不可」の適用例）(b) シフト途中で bit7 へ溢れる入力（|値|×2^n ≥ 128）は符号が壊れる＝入力値域を先に確認。〔in-house: PONG ai-variants v3 先読みリード 2026-07〕
- **BCD スコアはデコード不要で `cmp` 大小比較できる**：有効なパック BCD バイトは2進の大小＝十進の大小（上位ニブルが支配）→ `cmp #$11`（11点先取）も ScoreR vs ScoreL の優劣判定もそのまま正しい。ただし**2進の差は十進の差ではない**（桁跨ぎで +6 膨れる：$10−$09=7）→差を「量」として使う時は飽和バケット化で粗さを無害化（v4 ラバーバンドのスコア差→誤差幅変調）。減算差の bit7 符号は |2進差|<128 の範囲でのみ有効。〔in-house: PONG ai-variants v4 2026-07〕
- **★TIA リビジョン差は実機照合の落とし穴**：HMOVE の「追加クロック」効果（Cosmic Ark の星）は **post-1989 TIA で挙動が反転**するなど、**同じ ROM がリビジョンで違う絵になる**。harness のピクセル照合は**TIA リビジョン/エミュを固定**して比較する（どのリビジョンで検証したか記録）。〔採掘 191061 Cosmic Ark stars〕
- **最小バイト初期化＋ホットスポット配置**：狭い 2K/4K では Omegamatrix の8バイト自己書換 init（`bne .loop+1` で operator/operand 間を跳ぶ→`#$0A` が ASL として実行）で A=0/X=0/SP=$FF/carry clear を得る。バンク・ホットスポットは**最上位アドレス（既使用の割込みベクタ付近）に置く**と空きチャンクが最大化（ZP ホットスポット=Tigervision 3F は1ROMバイト＋1cy/切替を節約）。〔採掘 blog 12061, 11811〕
- **★物理行の「間借り」パターン（排他パスでWSYNC行を共有）**：Overscan の物理を「1関心事=1 WSYNC行」に分けると行数が足りなくなるが、**同一フレームで排他なパス（通常/ヒット/ミス/凍結…）は同じ行を別用途に使ってよい**——各パスが行 N の WSYNC を自分で strobe し、行の中身だけ差し替える（例：行3=通常はパドル入力／ヒット時はenglish計算／ミス時はサーブ処理）。スキップされた処理（パドル1フレーム分の入力反映など）は「1フレーム古い値で描く」だけ＝不可視の妥協。**総行数はどのパスでも同一**に保つ（可変分は filler 行数で相殺）。予算が膨らむ機能追加はまず「どのパスと排他か」を問い、専有行を新設する前に間借りを検討。毎フレーム必ず走る housekeeping（LFSR/カウンタ/音長/スイッチ検査）は**全パスが合流する専用行**に集約すると安全。〔in-house: PONG pf2 物理行アーキテクチャ 2026-07-02〜03（serve間借り→hit/miss全面化→行5新設）〕

## 「良い作画」の経験則
- 見栄え ≒ 色数 × スプライト密度。多色化はハードを足して買う（Pitfall II=DPC）。〔Demon Attack, Stay Frosty/Draconian〕
- 見本＝AtariAge Homebrew Awards「Best Graphics」部門。**最有力の ground-truth＝ユーザー本人が全作画した homebrew "Pizza Boy"**（Photoshop デザイン・制限は DaveC と確認）。外部スレ採掘より精度が高い＝設計判断（色帯/NUSIZ/フリッカ許容）を本人に直接当てる。
- **実作の裏書き（Pizza Boy 解剖）**：プロ級の見栄えは **標準カーネル（batari Basic multisprite＝可動5体 P1 flickersort＋P0＋M0/M1/BL＋6桁スコア）の上の craft** で達成されていた。exotic なコード技ではなく、**役割分担（建物＝静的非対称PF／可動体＝スプライト）＋窓リズム（PF 行で solid/窓 を交互＝縦の窓表現）＋色・密度設計**が効く。→「デザイナーが標準カーネルの上で画面を組む」という TIA Studio の前提を実作が裏書き。詳細 `reference/pizza-boy/dissection.ja.md`〔Pizza Boy, bB multisprite kernel〕
- 作る前にモックアップで実現性検証（色予算＋走査線数＋多重化を机上で）。

## 作画 craft（スプライト/文字の絵作り＝⑥craft の具体ルール）
- **サムネイル可読性を起点**：1ドット相当まで縮小して識別できるかを**先に**検証してから細部を足す。縮小は補間なし（nearest・半分ずつ）。〔326595, 106110〕
- **2600 ピクセルは横長（横 ≒ 縦の約 1/2・≈2:1）**：正方ドットのプレビューを信じない。実機アスペクトで字形/絵を決める（player=横1px間引き、PF=縦3–4倍で密度を稼ぐ）。**→ プレビューは非正方ピクセルで描く**。〔326595〕 `→ design.PixelAspectRatio/ScanlinesForSquare`
- **★画像→タイトルの正典ルート（プロの実作ワークフロー）**：SpiceWare は**先に Photoshop でモック→それからカーネル**を作る。ロゴ/タイトルは**フリッカ無し2色48pxカーネル**で「設計した48px画像→安定した画面表示」を実現（SF2 の実例）。＝本proj の Photoshop→2600 の道筋そのもの。`multicolor48`/`bitmap48` がこの実装基盤。〔採掘 blog SpiceWare 10640, 10515〕
  - **⚠ 精密値は要実測（codified 2:1 は過大）**：複数源が「2600 1px は横長」と一致するが**値は割れる**＝5:3≈1.67(190154,172161,334673) / 12:7≈1.71(169128) / 20:11≈1.82(208810,Stella 91%)。**現コード `design.PixelAspectRatio=2`(2.0) は全源より大きい＝確実に過大**だが、**正解は表示前提で 1.67〜1.82 に割れる**。Photoshop モック→2600 のユーザー主ワークフロー直結なので、**forum 値どれかで上書きせず、既知正方ROMを Stella で実測して definitive 値を1つ決めてから `pkg/design` を更新**（[[feedback-verification-standard]]）。色も非RGB＝Stella パレットが照合基準（306508, 300805）。〔採掘 190154, 169128, 208810, 172161〕
  - **走査線途中の色は 3CC グリッド上**：横多色は最大 ~18 帯／3色（SAX 流用で4色）。任意4色は不可＝穴(holes)＋重ね(stacking)で代替。SCORE bit(CTRLPF D1)で PF 左右分割。〔採掘 190154〕 `→ design.MinColorBandWidthPx, ScoreModeTwoColor`
- **字形の誤読ペアを潰す**：L/I/T・U/W・M/H/N・O/0/D。作者は自分の誤読に気づけない→**他者/読み上げで検証**、最終調整は単一ピクセル単位。〔294306, 326595（重複確認＝強い原則）〕
- **8px モノクロは輪郭に全予算**：識別力が最大の1パーツ（帽子/ヒゲ等）に集中。足りなければ倍幅＋ベネチアン縞で密度。〔106110〕
- **歩行アニメは最小2フレーム 50:50**：フレームカウンタの1ビット（`and #2^n`）で等間隔・リセット不要・**移動中のみ**回す。〔301861〕 `→ design.WalkFrame`
- **風景グラデは同一 hue・輝度のみ段階変化**（色相を混ぜない）。BG=奥／PF=手前の2層で奥行き。〔160655〕（色節の「高輝度→低彩度」則と整合）
- **背景アートは4軸で先に決める**：幅(48/96px)・色数(1/2)・PF対称性(反射/非対称)・行高(1〜16ライン/行＝精細度 vs 負荷)。**これは背景テンプレ（`design.BackgroundSpec`）の入力パラメータそのもの**。〔319884 atari-background-builder（=ユーザーが Pizza Boy で使ったツール）〕 `→ design.BackgroundSpec.Feasible`

## 機械判定不能な判断ルール（doc-only・`pkg/design` に落とさない）
数値化できず Claude/人/画像の判断が要るため、あえてコード化せずここに集約する（＝全ルールに処遇を与え網羅を保証）。
- **サムネイル可読性**：縮小して識別できるか＝画像と人の目が要る。`get_screen_annotated` の縮小プレビューで判断する。〔326595, 106110〕
- **字形の誤読ペア**（L/I/T・U/W・M/H/N・O/0/D）：作者は自分の誤読に気づけない＝他者/読み上げ検証。機械化困難。〔294306, 326595〕
- **8px モノクロは輪郭に全予算**：どのパーツに識別力が宿るかは題材依存の美的判断。〔106110〕
- **missile/ball=線・player=面の役割分担**：1つの見た目をどのオブジェクトの重ねで作るかは構図判断。〔Davie〕
- **GameState は1変数＋状態別カーネル**：構造の型であって数値判定ではない。〔title-to-game-transition〕
- **ISC/ISB 違法オペコード＋SP ラインカウンタ流用**：省サイクル技。可否は litmus 実測で裏取り（コードでなく検証で担保）。〔illegal-opcodes〕
- **symbolic 命名 / PAL-NTSC 二系統(N_xx/P_xx)**：色の持ち方の規約。`design.Hue/Luminance` で値は分解できるが「象徴名で持つ」運用自体は規約であってチェック対象でない。〔symbolic-color-names〕
- **ツール実装寄りの知見（spritemate データモデル / 走査線毎色UIの実装など）は吸収しない**：著述（asm を書く）に効かないため。凍結 `tia-studio/` リポと research ノートに保全で十分。

## 実装への落とし込み（`pkg/design` ／ 凍結 TIA Studio）
- フィジビリティ＝`pkg/design` の静的見積り＋実走の assert_line_budget/read_cycles/calibrate 連動で「この配置は 76cy に収まるか」を即判定。Claude が asm を書く前のゲートに使う。
- フィジビリティ4軸（色/走査線/多重化/予算）の既定値の詳細は `tools/research-w2-design.md` 末尾。
- テンプレ＝検証済みカーネル技（zone_multiplex/dyn_multisprite/score6/bitmap48/two_line_kernel…）に対応。
- ※ TIA Studio（canvas エディタ）は**凍結**（[[project-pivot-author-not-tool]]）。これらの寸法/判定は元々その M4 想定だったが、現在の主消費者は Claude の著述ループ。テンプレ群は復活時に流用可。

## Structure & efficiency rules from the Combat (1977) disassembly comparison
Distilled from an efficiency/structure comparison of a self-authored Combat clone (`combat_mine`, 4K) vs the original Wagner 2K ROM (`sandbox/studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`). Clean-room: generalized prose + routine names only. These are **integration-under-budget** rules — how the original fits a whole 27-variant game in 2K.

- **Move ALL objects through ONE `,X`-indexed path over a bearings/state array — do NOT inline per object**: hold each object's dir/vel/pos in parallel arrays indexed by object (Combat's `DIRECTN[0..3]` drives both tanks AND both missiles down one `,X` loop with a 24-byte `MVtable`). The clone inlined friction+accel 4× (P0/P1 × X/Y ≈ +120–200 B of pure duplication). Decisive point: **movement runs in blanked overscan, so the 76cy/line budget does not apply — an index costs nothing off-beam, so the indexed loop is BOTH smaller AND free.** Before adding a per-object copy of any motion code, ask whether one indexed pass over an array does it. 〔Combat `DIRECTN`/`MVtable` — one `,X` path for 4 objects; comparison §2.4/§4/§7, diff-gaps GAP-3〕
- **Momentum = time-sliced increments, not a fractional velocity**: as an alternative to `pos += vel/frac`, dither the velocity across time. `FwdTimer` ($F0→$00, 16 steps) `ROL`s two 8-bit halves (`MVadjA`/`MVadjB`); the emerging bit nudges `XoffBase` by $10 for that one frame → faint analog acceleration over 16 frames, **no multiply**. Diagonal isotropy = **frame gating** (`MPace & $03` moves on 3 of 4 frames), not a √2 fraction (cheaper, VCS-idiomatic). A plain subpixel integrator moves correctly but can't reproduce that "faint inertia" texture — reach for time-slicing when the *feel* matters. 〔Combat `FwdTimer`/`MVadjA`/`MVadjB`/`MPace`; diff-gaps GAP-3, comparison §2.4〕
- **Rotation sprite = precompute the shape into a RAM buffer so the kernel reads a bare `LDA abs,Y` (zero per-line rotation math)**: store only **180° of shapes in ROM**; synthesize the other 180° as a **point-rotation = `REFP` hardware H-flip + a reverse-order byte copy (software V-flip)**, rendered in VBLANK into a RAM shape buffer; re-render only **one object per frame** (30 Hz each) to bound the VBLANK cost. General pattern: *don't compute in the kernel; stage the shape in VBLANK*; the table needs only 180° (symmetry supplies the rest). 〔Combat `ROT`/`SHAPES`+`REFP0/1`+reverse copy → 16B HIRES RAM; diff-gaps GAP-5, comparison §2.2〕
- **One interleaved HIRES buffer can feed BOTH players (P0 = even bytes / P1 = odd)**: a single 16-byte RAM buffer serves both sprites — pick a player's bytes with `AND #$FE` / `ORA #$01`, no shape math. Halves the RAM vs two separate buffers (~16 B) = a RAM-thrift move to hold in reserve for when 128 B is tight. 〔Combat shared 16B HIRES, P0/P1 interleaved; comparison §2.1/§2.2/§7〕
- **Fan one byte out to many duties, phase-locked, when RAM is tight**: `CLOCK` serves **5 roles** (frame timer / attract color / debounce pace / score-flash clock …) and `GameTimer` serves **3** (match clock + bit7 in-progress flag + attract period), sub-fields phase-locked so their uses never collide. Master-class RAM economy — but **only pay this when RAM is actually scarce**: packing with 43 B free just spends clarity for nothing (premature optimization). Know it; deploy it only under pressure. 〔Combat `CLOCK` (5-duty) / `GameTimer` (3-duty) / `VCNTRL`; comparison §2.7/§7〕
- **Load-level VBLANK with `TIM64T`/`INTIM` so the picture starts at a FIXED beam position — don't rely on a fixed WSYNC count + elastic filler**: arm a RIOT timer at VBLANK start, spin on `INTIM` until it expires, then begin the visible kernel = display-start **independent of how long the frame's logic ran**. A fixed WSYNC count + elastic `VBpad` tuned to today's code does NOT auto-absorb logic growth: add work and the picture dips (screen dip — the exact fragility the clone's positioner had to hand-engineer around). Prefer timer load-leveling when VBLANK work is variable or expected to grow. 〔Combat `VCNTRL`/`INTIM`/`TIM64T`; comparison §2.1/§6, diff-gaps (INTIM で VBLANK 長を計る)〕 `→ techniques/sound-driver.md · game-states.md`
- **One wrap-around clear loop, reused with 4 seed values for 4 clear extents**: `ClearMem` is a single loop whose start index (X seed) is set 4 ways to wipe 4 regions — one routine, four callers, vs four clear loops. Cheap ROM-thrift for init/reset paths that wipe several ranges. 〔Combat `ClearMem`; comparison §2.8/§7〕
- **Audit your OWN hand-tuned code for cargo-cult — hand-tuned ≠ optimal, even in a 2K master ROM**: the annotated Combat disassembly honestly inventories its own cruft (a redundant double `STA GRP0`, a stray `WSYNC`, a self-flagged "why not `LDA MVtable+1,Y`?" 2-cycle miss). Model this: keep a written inventory of your ROM's own redundancy rather than assuming your tuned code is tight. (Applied to our clone, this surfaced ~250–400 B of recoverable duplication unrelated to its provability trade.) 〔Combat — Williams' annotations; comparison §7〕
