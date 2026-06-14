# Atari 2600 ビジュアル設計原則（design-principles）

採掘（AtariAge）＋web 研究で得た「ルール化できる作画設計の原則」の正典。用途＝(1) Claude のデザイン判断の
明示ルール（roms/EVALUATION.md の⑥craft）(2) TIA Studio のテンプレ寸法・フィジビリティ判定の根拠。
詳細出典＝`tools/research-w2-design.md` ＋ `reference/atariage/*/notes.ja.md`。

## 色（最重要）
- **色は RGB でなく「レジスタ値／象徴名（hue 上位ニブル × lum 下位ニブル）」で持つ**。生 hex をばら撒かない。
  輝度は実質 8 段（bit0 無効）。PAL/NTSC は 1行で切替できる二系統（N_xx/P_xx）を設計目標に。〔Davie S11, symbolic-color-names〕
- **多色は「縦に足す」＝走査線ごとに COLUPx 書換え**（横は1色）。**横多色は高い**（PF score/Chronocolour/フリッカー/重ねの擬似のみ）。〔Hugg, Davie S21〕
  - **横多色の色帯 最小幅 = ストア命令サイクル × 3px**。PF 整列の色は 4色クロック（=12px, `STx.w`）の倍数、任意色は ~6cy/帯（1行で約8帯が上限）。SP（`txs`/`tsx`）を 4本目の色レジスタに流用する手も。〔170018 multiple-colors-per-scanline〕
- **「唯一正しい RGB」は存在しない**：Stella は YIQ 動的生成、同一レジスタ値でもエミュ/設定で十数〜0x20 差。
  我々は実走テーブル `internal/ingest/palette_stella.go` が正（Stella 照合100%）。〔rgb-color-values, 118495〕
- **hue↔色の地図**：hue1=黄 / hue4=赤 / hue8=青 / hue12=緑（hue15≈hue1）。黄は hue1 が定石。〔132561〕
- **高輝度ほど彩度が落ち白っぽくなる**（特に明るい青は識別が消える）→ **鮮やかに見せたい色は中〜低輝度**で置く。彩度と輝度はトレードオフ。〔132561〕

## スプライト（P0/P1）
- 8 ドット幅・1 レジスタ（GRP 8bit, MSB=左端）。幅 NUSIZ 1x/2x/4x。〔2k6specs, Davie S21〕
- **横位置 = 2段階**：粗 ÷15（5cy ループ）→ 微 HMOVE。粒度 3px/CPUサイクル（litmus 一致）。〔Davie S22〕
- **48px** = NUSIZ$03（3 copies）＋ P1 を 8px 右 ＋ VDEL 二重バッファで GRP 時間差差替え。score/bitmap48 を転用。〔48px-positioning〕
- **絵を先に決めて割当しない**。順序＝色予算→割当表→不足は「色共有・オブジェクト兼用・レイアウト変更」で交渉。
- missile/ball = 線・縁・縦枠、player = 倍幅/複数コピー/4x で面。1つの見た目を複数オブジェクトの重ねで構成。

## プレイフィールド
- 横 40px × 4clk/px。表現力は縦のリズムで稼ぐ。〔Davie S13〕
- **PF スクロール背景は「盤面RAM＋表示バッファ＋delta更新」3層**＋タイル単位スクロール（tearing 回避）。鉄則＝**総スキャンライン数をフレーム間で一定に保つ**（PAL は偶数必須・安全圏 262/264）。スクロール帯は上下 10〜16 ライン。〔200972 tile-scrolling-engines, Boulder Dash 型〕
- **HUD/テキストは「出す文字数」で技法が決まる**：48px=12字 / venetian blinds=32字（ただし 3px 幅専用）。HUD は画面モード分離 or ゾーン隔離＋スコア枠の多目的再利用。〔197162 text-hud〕
- **非対称 PF は高コスト**（走査線途中で PF0/1/2 を2回書く、PF0 窓は ~20cy のみ）。妥協＝中央32px/1行おき+倍高/venetian/RAM自己書換。〔Davie S17, castlevania-port〕
- **非対称PFの書込み締切（実測サイクル）**：左半分を表示しつつ同一走査線で右半分へ書き換える時、書込みは「その PF が**もう見えていない**瞬間」を狙う。古典カーネルの実値＝
  1回目 PF0[cy7] / PF1[cy14] / PF2[cy21]（左半分用、可視前に間に合わせる）→ 右半分用に
  **PF0 再書込み cy31 / PF1 cy38 / PF2 は“ちょうど cy45”**（早くても遅くても崩れる＝nop 1個追加で破綻）。
  残り 76−47≒**29cy/line がスプライト等の自由予算**。横多色PFの実現性は「この 45cy 一点を外さず、かつ残29cy に他処理が収まるか」で判定する。〔Williams/Saunders "Asymmetric Reflected Playfield" tutorial〕
- **タダで2色PF＝CTRLPF D1（score bit）**：bit1 を立てると **左半分PF=COLUP0・右半分PF=COLUP1** で独立色になる（非対称書込みタイミング不要）。スコア表示の定番だが、背景の左右色分けにも使える安価な2色化。〔w11/Asym2scrol〕

## 多重化・フリッカー
- 2体超は Y 帯で多重化、横再配置は1走査線消費、**空 Y レーン必須**、代償 30Hz ちらつき。〔Bumbershoot〕
- **フリッカは最終手段／短命限定**。大面積禁止。エミュを信用せず複数フレーム合成で検証。〔flicker-to-enhance-graphics〕
  - ただし**意図的フリッカを演出に転化**する手もある：点滅する目標物で Game&Watch 味を出しつつ、可動5スプライト制限を回避＝欠点を美観に変えた実例。〔Pizza Boy 329673〕
  - **時間混色（2フレーム交互色）を使うなら、両色を同一輝度にして hue だけで分ける**＝フリッカー知覚は輝度差に比例するので激減する（例 lum4 の黄緑 vs 青緑）。〔176987 interlaced-multicolor〕

## カーネル予算・状態
- **76cy/line が天井**。ライン数を先に決め残予算で機能配分。〔splendidnut〕
- 状態＝1個の GameState 変数＋状態別カーネル。タイトル絵は上下パディング＋中央PFテーブル、終端で GRP/PF クリア。〔title-to-game-transition〕
- 省サイクル＝ISC/ISB 非公式オペコード＋SP をラインカウンタ流用（要 litmus 裏取り）。〔5cycle-color-cycling, illegal-opcodes〕

## 「良い作画」の経験則
- 見栄え ≒ 色数 × スプライト密度。多色化はハードを足して買う（Pitfall II=DPC）。〔Demon Attack, Stay Frosty/Draconian〕
- 見本＝AtariAge Homebrew Awards「Best Graphics」部門。**最有力の ground-truth＝ユーザー本人が全作画した homebrew "Pizza Boy"**（Photoshop デザイン・制限は DaveC と確認）。外部スレ採掘より精度が高い＝設計判断（色帯/NUSIZ/フリッカ許容）を本人に直接当てる。
- **実作の裏書き（Pizza Boy 解剖）**：プロ級の見栄えは **標準カーネル（batari Basic multisprite＝可動5体 P1 flickersort＋P0＋M0/M1/BL＋6桁スコア）の上の craft** で達成されていた。exotic なコード技ではなく、**役割分担（建物＝静的非対称PF／可動体＝スプライト）＋窓リズム（PF 行で solid/窓 を交互＝縦の窓表現）＋色・密度設計**が効く。→「デザイナーが標準カーネルの上で画面を組む」という TIA Studio の前提を実作が裏書き。詳細 `reference/pizza-boy/dissection.ja.md`〔Pizza Boy, bB multisprite kernel〕
- 作る前にモックアップで実現性検証（色予算＋走査線数＋多重化を机上で）。

## 作画 craft（スプライト/文字の絵作り＝⑥craft の具体ルール）
- **サムネイル可読性を起点**：1ドット相当まで縮小して識別できるかを**先に**検証してから細部を足す。縮小は補間なし（nearest・半分ずつ）。〔326595, 106110〕
- **2600 ピクセルは横長（横 ≒ 縦の約 1/2・≈2:1）**：正方ドットのプレビューを信じない。実機アスペクトで字形/絵を決める（player=横1px間引き、PF=縦3–4倍で密度を稼ぐ）。**→ TIA Studio エディタは非正方ピクセルで表示すべき**（M1 実装要件）。〔326595〕
- **字形の誤読ペアを潰す**：L/I/T・U/W・M/H/N・O/0/D。作者は自分の誤読に気づけない→**他者/読み上げで検証**、最終調整は単一ピクセル単位。〔294306, 326595（重複確認＝強い原則）〕
- **8px モノクロは輪郭に全予算**：識別力が最大の1パーツ（帽子/ヒゲ等）に集中。足りなければ倍幅＋ベネチアン縞で密度。〔106110〕
- **歩行アニメは最小2フレーム 50:50**：フレームカウンタの1ビット（`and #2^n`）で等間隔・リセット不要・**移動中のみ**回す。〔301861〕
- **風景グラデは同一 hue・輝度のみ段階変化**（色相を混ぜない）。BG=奥／PF=手前の2層で奥行き。〔160655〕（色節の「高輝度→低彩度」則と整合）
- **背景アートは4軸で先に決める**：幅(48/96px)・色数(1/2)・PF対称性(反射/非対称)・行高(1〜16ライン/行＝精細度 vs 負荷)。**これは TIA Studio 背景テンプレの入力パラメータそのもの**。〔319884 atari-background-builder（=ユーザーが Pizza Boy で使ったツール）〕

## TIA Studio への落とし込み
- テンプレ寸法・フィジビリティ4軸（色/走査線/多重化/予算）の既定値は `tools/research-w2-design.md` 末尾に詳細。
- テンプレ＝検証済みカーネル技（zone_multiplex/dyn_multisprite/score6/bitmap48/two_line_kernel…）に対応。
- フィジビリティ＝assert_line_budget/read_cycles/calibrate 連動で「この配置は 76cy に収まるか」を即判定。
