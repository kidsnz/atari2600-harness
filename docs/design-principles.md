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
- **★TIA リビジョン差は実機照合の落とし穴**：HMOVE の「追加クロック」効果（Cosmic Ark の星）は **post-1989 TIA で挙動が反転**するなど、**同じ ROM がリビジョンで違う絵になる**。harness のピクセル照合は**TIA リビジョン/エミュを固定**して比較する（どのリビジョンで検証したか記録）。〔採掘 191061 Cosmic Ark stars〕
- **最小バイト初期化＋ホットスポット配置**：狭い 2K/4K では Omegamatrix の8バイト自己書換 init（`bne .loop+1` で operator/operand 間を跳ぶ→`#$0A` が ASL として実行）で A=0/X=0/SP=$FF/carry clear を得る。バンク・ホットスポットは**最上位アドレス（既使用の割込みベクタ付近）に置く**と空きチャンクが最大化（ZP ホットスポット=Tigervision 3F は1ROMバイト＋1cy/切替を節約）。〔採掘 blog 12061, 11811〕

## 「良い作画」の経験則
- 見栄え ≒ 色数 × スプライト密度。多色化はハードを足して買う（Pitfall II=DPC）。〔Demon Attack, Stay Frosty/Draconian〕
- 見本＝AtariAge Homebrew Awards「Best Graphics」部門。**最有力の ground-truth＝ユーザー本人が全作画した homebrew "Pizza Boy"**（Photoshop デザイン・制限は DaveC と確認）。外部スレ採掘より精度が高い＝設計判断（色帯/NUSIZ/フリッカ許容）を本人に直接当てる。
- **実作の裏書き（Pizza Boy 解剖）**：プロ級の見栄えは **標準カーネル（batari Basic multisprite＝可動5体 P1 flickersort＋P0＋M0/M1/BL＋6桁スコア）の上の craft** で達成されていた。exotic なコード技ではなく、**役割分担（建物＝静的非対称PF／可動体＝スプライト）＋窓リズム（PF 行で solid/窓 を交互＝縦の窓表現）＋色・密度設計**が効く。→「デザイナーが標準カーネルの上で画面を組む」という TIA Studio の前提を実作が裏書き。詳細 `reference/pizza-boy/dissection.ja.md`〔Pizza Boy, bB multisprite kernel〕
- 作る前にモックアップで実現性検証（色予算＋走査線数＋多重化を机上で）。

## 作画 craft（スプライト/文字の絵作り＝⑥craft の具体ルール）
- **サムネイル可読性を起点**：1ドット相当まで縮小して識別できるかを**先に**検証してから細部を足す。縮小は補間なし（nearest・半分ずつ）。〔326595, 106110〕
- **2600 ピクセルは横長（横 ≒ 縦の約 1/2・≈2:1）**：正方ドットのプレビューを信じない。実機アスペクトで字形/絵を決める（player=横1px間引き、PF=縦3–4倍で密度を稼ぐ）。**→ プレビューは非正方ピクセルで描く**。〔326595〕 `→ design.PixelAspectRatio/ScanlinesForSquare`
- **★画像→タイトルの正典ルート（プロの実作ワークフロー）**：SpiceWare は**先に Photoshop でモック→それからカーネル**を作る。ロゴ/タイトルは**フリッカ無し2色48pxカーネル**で「設計した48px画像→安定した画面表示」を実現（SF2 の実例）。＝本proj の Photoshop→2600 の道筋そのもの。`multicolor48`/`bitmap48` がこの実装基盤。〔採掘 blog SpiceWare 10640, 10515〕
  - **⚠ 精密値は要実測（codified 2:1 は過大）**：複数源が「2600 1px は横長」と一致するが**値は割れる**＝5:3≈1.67(190154,172161,334673) / 12:7≈1.71(169128) / 20:11≈1.82(208810,Stella 91%)。**現コード `design.PixelAspectRatio=2`(2.0) は全源より大きい＝確実に過大**だが、**正解は表示前提で 1.67〜1.82 に割れる**。Photoshop モック→2600 のユーザー主ワークフロー直結なので、**forum 値どれかで上書きせず、既知正方ROMを Stella で実測して definitive 値を1つ決めてから `pkg/design` を更新**（[[feedback-verification-first]]）。色も非RGB＝Stella パレットが照合基準（306508, 300805）。〔採掘 190154, 169128, 208810, 172161〕
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
