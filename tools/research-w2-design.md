# research-w2-design — Atari 2600 ビジュアル設計の原則 & 作画 ground-truth

W2 web リサーチ担当の成果。Claude のデザイン力強化 + TIA Studio のテンプレ/フィジビリティ判定の素材。
収集日: 2026-06-13。全項目に出典 URL を付す。**「ルール化できる原則」を最優先で箇条書き化**。

---

## 1. 2600 作画 / グラフィック設計の原則・チュートリアル

主出典: Andrew Davie *"2600 Programming for Newbies"* (randomterrain 版), Steven Hugg
*Making Games for the Atari 2600* (8bitworkshop), splendidnut *2600 Display Kernels*,
Bumbershoot Software のスプライト多重化記事。

### ルール化できる設計原則（スプライト / プレイヤー）
- **P1: プレイヤーは 8 ドット幅・1 レジスタ**。GRP0/GRP1 が各 8 ビット = 横 8 ピクセルの形状。
  プレイヤーピクセルはプレイフィールドの 1/4 幅（= 1 color clock 相当の細さ）。
  〔Davie S21〕
- **P2: スプライトの色は COLUP0/COLUP1 で1個ずつ**。1 スプライトにつき同時に 1 色。多色スプライトは
  **スキャンラインごとに COLUPx を書き換える**ことで縦方向に色を足す（横方向は 1 色のまま）。
  〔Davie S21 / Hugg "Color Sprites"〕
- **P3: 横位置は「描き始め」で決める**。RESP0/RESP1 へ書いた瞬間にそのスプライトの描画が始まる。
  座標を直接代入するのではなく、ビーム位置に合わせて strobe する。粒度は **6502 1 サイクル = 3 TIA
  ピクセル**。〔Davie S21/S22〕
- **P4: 粗位置 = 15-color-clock ループ（÷15）→ 微調整 = HMOVE**。`dex`(2)+`bne`(3)=5 cy ループ =
  15 color clock 進む。X ピクセル位置 ≒ X/15 回ループ。残り 0–14px を HMOVE 微動で詰める（2 段階法）。
  〔Davie S22〕（※我々の litmus は `X = 3N − 54`(player) でこれを数値検証済み）
- **P5: 48 ピクセルの大型画像はカーネルで作る**。NUSIZ0=NUSIZ1=$03（3 close copies）+ P1 を P0 の
  8px 右に置く → 隙間なく 48px 幅。**VDELP0/VDELP1 を立てて二重バッファ化**し、描画中に GRP0/GRP1 を
  順次差し替える。書き換え順は厳密にサイクル管理（例 P0=55,P1=63 で cycle 7/14/21/44/47/50/53 に
  sta GRP0/GRP1 を交互）。タイトル/大型ボス絵の定番。〔WoodgrainWizard 48-pixel Image Routine〕
- **P6: 2 個より多いオブジェクトは「多重化（multiplexing）」**。同一フレーム内で各スプライトを別々の
  Y 帯に置き、帯の切れ目で再 strobe する。**横の再位置決めは丸々 1 スキャンライン消費**するのが原則。
  〔Bumbershoot "Successfully Multiplexing Sprites"〕
- **P7: 多重化スプライト間には必ず「空の Y レーン」を 1 本挟む**。アニメで Y が動いても、再位置決め中の
  ちらつき/破綻を防ぐためオブジェクト間に常時空帯を確保する。レーン制（例 16 レーン × 8 ライン）で X/Y を
  管理。〔Bumbershoot〕
- **P8: 多重化の代償はちらつき（30Hz 化）**。1 フレームに収まらない数を出すと交互フレーム描画 = 30Hz
  フリッカ。出せる数の現実的上限はカーネルの CPU 予算で決まる。〔Bumbershoot / splendidnut〕

### ルール化できる設計原則（プレイフィールド / 背景）
- **PF1: PF 解像度は横 40 ピクセル**（= 20 ビット × 左右複製）。各 PF ピクセル = **4 color clock 幅**
  （160/40）。縦は任意（スキャンライン単位で書き換え可）。〔Davie S13〕
- **PF2: 左右対称が「ただ」、非対称は「高い」**。CTRLPF D0 = 0:repeat（右が左のコピー）/ 1:reflect
  （鏡像）。**非対称 PF はスキャンラインの途中で PF0/PF1/PF2 を 2 回書く**必要があり、CPU を食う。
  〔Davie S13/S17, CLAUDE.md 既知定数と一致〕
- **PF3: 非対称 PF の書き換え窓（color clock）**:
  PF0 = 84–147、PF1 = 116–163、PF2 = 148–195 の間に右側用の値を書く（左側は cycle 68 より前に書く）。
  PF0 窓は約 60 clock ≒ **20 CPU サイクル**しかない厳しさ。1 ラインで PF を計 6 回書くと CPU の大半を
  消費。〔Davie S17〕
- **PF4: 縦方向はコスト最小の「絵の伸ばし所」**。横は 40px で粗いので、**背景の表現力は縦のリズム
  （スキャンラインごとの PF / 色変化）で稼ぐ**のがセオリー（地形・水平線・グラデーション）。
  〔Davie S13–S20 から総合〕

### ルール化できる設計原則（色 / カーネル全体）
- **C1: 色は「色相 上位ニブル × 輝度 下位ニブル」**。レジスタ値 $HL の H=hue(0–15)、L=lum(0–15, 偶数)。
  例 $25 = hue2 lum5。〔Davie S11〕
- **C2: 輝度は奇数ビット無効 = 実質 8 段**。色レジスタ下位ニブルの bit0 は無視され、$x0/$x2/…/$xE の
  **8 段の輝度**しか出ない。〔TIA 構造 / 2k6specs〕
- **C3: スキャンライン数を変えると色が変わる**。NTSC/PAL/SECAM で色マップが別物。カーネルのライン数を
  いじると同じ $値でも色が変わるので、色は **必ず実機/エミュで確認**する（机上で決めない）。〔Davie S11〕
- **C4: カーネルの CPU 予算が画づくりの天井**。1 ライン = 76 CPU サイクル（228 clock / 3）。HMOVE・PF
  二度書き・GRP 差し替え・色替えはすべてこの予算を食い合う。**「何ライン使うか」を先に決め、残予算で
  作画機能を割り振る**のが設計順序。〔Davie S22 / splendidnut〕
- **C5: 表示カーネルの行解像度を決める**。1-line kernel（毎スキャンライン更新＝高精細・高負荷）か
  2-line kernel（2 ラインで 1 更新＝省 CPU・縦半解像度）。splendidnut は DPC+ 等で RAM/データフェッチを
  足し 5-line 級の多色 single-line スプライトまで実現。〔splendidnut "2600 Display Kernels"〕

出典:
- Davie S11 https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-11.html
- Davie S13 https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-13.html
- Davie S17 https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-17.html
- Davie S21 https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-21.html
- Davie S22 https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-22.html
- Hugg 書籍 https://8bitworkshop.com/docs/books/ （章: Playfield Graphics / Players and Sprites / Color Sprites / Sprite Fine Positioning / NUSIZ and Other Delights）, errata http://8bitworkshop.com/docs/books/errata.html
- splendidnut https://forums.atariage.com/blogs/entry/18176-2600-display-kernels/
- 48-pixel routine http://www.taswegian.com/WoodgrainWizard/tiki-index.php?page=48-pixel-Image-Routine
- Bumbershoot 多重化 https://bumbershootsoft.wordpress.com/2024/10/05/atari-2600-successfully-multiplexing-sprites/
- Bumbershoot 縦配置 https://bumbershootsoft.wordpress.com/2018/09/04/atari-2600-vertical-sprite-placement/

---

## 2. NTSC パレットの ground-truth（RGB 出典と差分）

### 重要発見: 我々の `palette_stella.go` と Stella 公式静的配列は別物
我々の `harness/internal/ingest/palette_stella.go` は「Stella 7.x デフォルト設定で実走し savesnap から
採取」した値（= 現代 Stella が **YIQ 位相演算で生成**したパレットに gamma/contrast/saturation 調整が
乗ったもの）。一方 Stella ソースの**静的フォールバック配列 `ourNTSCPalette`** は古典的固定テーブルで、
値が体系的に異なる。

| TIA code | 我々 palette_stella.go | Stella `ourNTSCPalette`(静的) | Stella `ourNTSCPaletteZ26` |
|---|---|---|---|
| $00 | `06 06 06` | `00 00 00` | `00 00 00` |
| $02 | `34 34 34` | `4a 4a 4a` | `50 50 50` |
| $04 | `5C 5C 5C` | `6f 6f 6f` | `64 64 64` |
| $06 | `88 88 88` | `8e 8e 8e` | `78 78 78` |
| $08 | `B8 B8 B8` | `aa aa aa` | `8c 8c 8c` |

→ **グレースケール段で最大 ~0x20 (32) のズレ**。出典が違えば RGB は普通に十数〜数十違う。
「唯一の正解 RGB」は存在せず、**①TIA の YIQ 由来生成値（位相シフト/gamma 依存）と ②各エミュの固定
テーブルが併存**するのが実態。**我々の用途（Stella savesnap の量子化逆引き）には我々の実測テーブルが正**。
Gopher2600 のパレットとも微差あり（go ファイル冒頭コメント既知）。

### ground-truth ソース一覧（在処）
- **Stella 公式ソース（最も権威ある一次）**: `src/common/PaletteHandler.cxx`
  - 静的配列 `ourNTSCPalette`（古典 128 色, 24bit hex）+ `ourNTSCPaletteZ26`（z26 由来の別系）。
  - 現代の既定は配列直読みでなく `adjustedPalette()` が **位相 myPhaseNTSC / hue / saturation /
    contrast / gamma（"match PC 2.2 gamma to TV 2.65 gamma"）から動的生成**。→ ユーザー設定で RGB が
    変わる = 「設定込みで初めて確定」が公式の立場。
  - https://github.com/stella-emu/stella/blob/master/src/common/PaletteHandler.cxx
- **Stella `ourNTSCPalette` 全 128 色（採取済み・下記 §2-appendix に転記）**。
- **biglist stella-list TIA colorchart**（Glenn Saunders / Trebor 由来, ツールチップ内に RGB）
  https://www.biglist.com/lists/stella/archives/200109/msg00285.html
- **randomterrain インタラクティブ TIA color chart**（Trebor の値, クリックで hex 表示）
  https://www.randomterrain.com/atari-2600-memories-tia-color-charts.html
- **Lospec**（コミュニティ標準パレット。NTSC 128 色, PAL/Wikipedia 版もあり, .pal/.hex/.gpl DL 可）
  https://lospec.com/palette-list/atari-2600-palette-ntsc-version
  https://lospec.com/palette-list/atari-2600-tia-ntsc
- **gimp-palettes (denilsonsa)** HW-Atari-2600-NTSC.gpl
  https://github.com/denilsonsa/gimp-palettes/blob/master/palettes/HW-Atari-2600-NTSC.gpl
- **Wikipedia / HandWiki / Grokipedia "video game console palettes"**（YIQ 由来の解説 + 表）
  https://en.wikipedia.org/wiki/List_of_video_game_console_palettes
- **TIA の色生成原理**: 3.579545MHz color clock の位相シフトで hue、振幅で luminance。NTSC = 8 hue ×
  16 lum（うち lum bit0 無効で実質 8 段）= 128 色。PAL=104, SECAM=8。
  〔randomterrain S11 / Wikipedia〕

### RGB 差の結論
- **複数ソースで RGB は確実に違う**（生成方式・想定 CRT・gamma が違うため）。グレー段で ~32 差を実測。
- **検証時は「どのソースか」を必ず明記**。我々の `palette_stella.go`（実走 savesnap）は Stella 既定
  設定の現実値を捉えており、Stella ピクセル照合（CLAUDE.md の 100% 照合）にはこれが正。
- 公式静的 `ourNTSCPalette` は「設定無依存の素のテーブルが欲しい」場合の基準として併記価値あり。

### §2-appendix: Stella `ourNTSCPalette` 静的配列（全 128 色, 採取 2026-06-13）
TIA code 順 $00,$02,…($x0..$xE が各行 8 個）。値は 0xRRGGBB。
```
gray : 000000 4a4a4a 6f6f6f 8e8e8e aaaaaa c0c0c0 d6d6d6 ececec
gold : 484800 69690f 86861d a2a22a bbbb35 d2d240 e8e84a fcfc54
orange:7c2c00 904811 a26221 b47a30 c3903d d2a44a dfb755 ecc860
red-o: 901c00 a33915 b55328 c66c3a d5824a e39759 f0aa67 fcbc74
red  : 940000 a71a1a b83232 c84848 d65c5c e46f6f f08080 fc9090
purple:840064 97197a a8308f b846a2 c659b3 d46cc3 e07cd2 ec8ce0
violet:500084 68199a 7d30ad 9246c0 a459d0 b56ce0 c57cee d48cfc
blue1: 140090 331aa3 4e32b5 6848c6 7f5cd5 956fe3 a980f0 bc90fc
blue2: 000094 181aa7 2d32b8 4248c8 545cd6 656fe4 7580f0 8490fc
blue3: 001c88 183b9d 2d57b0 4272c2 548ad2 65a0e1 75b5ef 84c8fc
cyan : 003064 185080 2d6d98 4288b0 54a0c5 65b7d9 75cceb 84e0fc
teal : 004030 18624e 2d8169 429e82 54b899 65d1ae 75e7c2 84fcd4
green: 004400 1a661a 328432 48a048 5cba5c 6fd26f 80e880 90fc90
grn-y: 143c00 355f18 527e2d 6e9c42 87b754 9ed065 b4e775 c8fc84
ylw-g: 303800 505916 6d762b 88923e a0ab4f b7c25f ccd86e e0ec7c
brown: 482c00 694d14 866a26 a28638 bb9f47 d2b656 e8cc63 fce070
```
（注: 列順は luminance 低→高。行ラベルは hue $0x..$Fx の便宜名）

---

## 3. 名作の作画分解・ギャラリー（「良い作画」の実例ソース）

### ルール化できる「良い作画」の傾向（実例から逆算）
- **多色化はハードを足して買う**: Pitfall II は **DPC（Display Processor Chip）** を積み、メモリ +25%
  でより複雑なグラフィックと BGM を実現。市販で群を抜く画は専用チップ前提が多い。
  〔Wikipedia Pitfall II〕
- **「見栄え」の定番は色数 × スプライト密度**: Demon Attack が「最も見栄えする」と評されたのは**カラフル
  なエイリアン スプライト**。色替えカーネルで縦に色を足すのが効く（§1 C2/P2）。〔racketboy〕
- **homebrew は市販を超えた**: SpiceWare の **Stay Frosty / Draconian** は作画で高評価。AtariAge
  Homebrew Awards に「Best Graphics (Original)」部門が常設され、現役で技術が競われている。
  〔AtariAge Homebrew Awards〕

### 実例ソース（在処）
- AtariAge **Best Graphics 部門**（4th Annual Homebrew Awards、ノミネート＝現代の作画上限の見本）
  https://forums.atariage.com/topic/330066-atari-2600-best-graphics-original-4th-annual-atari-homebrew-awards/
- retrostack「Atari Homebrew Awards Nominees and Winners」
  https://retrostack.substack.com/p/atari-homebrew-awards-nominees-and
- retrostack「Top 40 Atari 2600 Homebrew Developers」（SpiceWare 等の作画派が並ぶ）
  https://retrostack.substack.com/p/top-40-atari-2600-homebrew-developers
- racketboy "Games That Defined the Atari 2600"（Demon Attack 等の作画評）
  https://racketboy.com/retro/best-games-that-defined-the-atari-2600
- Wikipedia Pitfall II（DPC による画の強化）
  https://en.wikipedia.org/wiki/Pitfall_II:_Lost_Caverns
- The Spriters Resource Pitfall（スプライト素材ギャラリー）
  https://www.spriters-resource.com/mobile/pitfall/
- ※ DaveC "Pizza Boy" は web インデックス上で確証取れず（同名別物多数）。AtariAge 内の DaveC 投稿を
  直接当たる必要あり（W2 では未確定として残す）。

---

## 4. スプライト / PF のデータ形式・寸法の正典（一次ソース）

主出典: Stella Programmer's Guide, problemkaputt 2k6specs, atarihq TIA_HW_Notes, Davie tutorials,
6502.org。**TIA Studio のフィジビリティ判定 = 寸法/ビット順の正典**。

### GRP（プレイヤー形状）
- **8 ビット = 横 8 ドット**。`1`=プレイヤー色、`0`=透明。
- **既定の描画は MSB first（bit7 = 左端ピクセル）**。〔2k6specs〕
- **REFP0/REFP1 の bit3 = Reflect**: 0=Normal(MSB first) / 1=Mirror(LSB first = 左右反転)。〔2k6specs〕
- データを置くとき、左を向くキャラ/右を向くキャラは REFP で 1 枚を使い回せる（設計上の節約）。

### NUSIZ0/NUSIZ1（Number-Size, D2-D0 全表）
| 値(D2D1D0) | プレイヤー複製/サイズ | パターン |
|---|---|---|
| 0 (000) | 1 copy | `X..........` |
| 1 (001) | 2 copies close | `X.X........` |
| 2 (010) | 2 copies medium | `X...X......` |
| 3 (011) | 3 copies close | `X.X.X......` ← **48px カーネルで使用** |
| 4 (100) | 2 copies wide | `X.......X..` |
| 5 (101) | double-size player (16px幅) | `XX.........` |
| 6 (110) | 3 copies medium | `X...X...X..` |
| 7 (111) | quad-size player (32px幅) | `XXXX.......` |
- **bit4-5 = Missile size**: 0..3 = **1,2,4,8 px 幅**。〔2k6specs〕
- 〔出典 2k6specs / Stella PG / atariarchives〕。我々 CLAUDE.md の 48px=「three copies close」と一致。

### VDELP0/VDELP1（縦遅延）
- **bit0 = Vertical Delay**: 0=遅延なし / 1=GRP への書き込みをバッファし、相方の GRP 書き込み時に表示。
- 48px カーネルや 1-line kernel の二重バッファ手段。〔2k6specs / 48-pixel routine〕

### Ball / Missile サイズ
- **CTRLPF bit4-5 = Ball size**: 0..3 = 1,2,4,8 px 幅。〔2k6specs〕
- Missile は NUSIZ bit4-5（上記）。

### 寸法・座標の正典（我々の litmus と整合）
- 1 スキャンライン = **228 color clock**（HBLANK 68 + visible 160）= **76 CPU サイクル**（3 clk/cy）。
- 横位置粒度 = **3 px / CPU サイクル**。粗 = ÷15（5cy ループ）、微 = HMOVE（上位ニブル, 2の補数,
  正=左 / 負=右, 範囲 +7..−8, 1px 粒度）。〔Davie S22 / CLAUDE.md litmus 検証済〕
- PF = 40 列 × 4 color clock。PF0 上位ニブルのみ(col0→D4..col3→D7)、PF1 MSB first(col4→D7..),
  PF2 LSB first(col12→D0..)。CTRLPF D0: 0=repeat / 1=reflect。〔CLAUDE.md / Davie S13–S20〕

### 一次ソース URL
- Stella Programmer's Guide (HTML) https://www.atariage.com/2600/programming/2600_101/docs/stella.html
- problemkaputt 2k6specs https://problemkaputt.de/2k6specs.htm
- atarihq TIA Hardware Notes https://www.atarihq.com/danb/files/TIA_HW_Notes.txt
- atariarchives TIA description https://www.atariarchives.org/dev/tia/description.php
- 48-pixel routine http://www.taswegian.com/WoodgrainWizard/tiki-index.php?page=48-pixel-Image-Routine
- 6502.org（CPU タイミング, 既知定数の出典）

---

## 付記: TIA Studio への含意（テンプレ/フィジビリティ判定の素材）
- **テンプレの寸法既定値**: player=8px(×2/×4 で 16/32px), missile/ball=1/2/4/8px, PF=40 列×4clk,
  48px 合体スプライト=NUSIZ$03+8px オフセット+VDEL。これらを「置ける形/サイズの選択肢」として固定。
- **フィジビリティ判定の軸**: ①横は 3px 粒度（1px 単位の自由配置は不可、HMOVE で詰める）②同一ラインに
  プレイヤーは実質 2 体（多重化で増やすと 1 ライン/再配置＋ちらつき）③非対称 PF は 1 ライン CPU 大量消費
  （PF0 窓 ~20cy）④色はスキャンライン単位でしか替えられない（横方向の多色は GRP 差し替え 48px 技のみ）。
- **色の扱い**: 量子化/照合は `palette_stella.go`（実走値）を正とし、素のテーブルが要る時は Stella
  静的 `ourNTSCPalette`（§2-appendix）を併記。RGB は「出典依存で十数〜32 差」を前提に判定。
