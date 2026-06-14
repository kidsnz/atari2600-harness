# research-w7 — 画像→2600 変換パイプライン（OSS・アルゴリズム調査）

調査日: 2026-06-13。
目的: **D2 ループ（Photoshop モック → TIA バイト）の帯域を太くする**。  
既存 harness の `analyze_image`/`ingest` が既にやっている事との差分を切り分け、ingest 強化への具体示唆を導く。

---

## 0. 狙い：D2 ループとは何か

Pizza Boy 実績ワークフロー:

```
Photoshop PSD
  → (手作業で 40列×H行 に縮小・白黒化)
  → atari-background-builder (abb) でビットマップ編集
  → .abb.json (yxGrid + colorGrid) エクスポート
  → (手動で DASM ASM へ転記)
  → harness で検証
```

現状の「設計→バイト」の変換は **手作業が多い・色の選択が手動**。  
本調査は「Photoshop の PNG からどのくらい自動で TIA バイトに落とせるか」を整理する。

---

## 1. 既存 ingest との差分（先に切り分け）

| 能力 | harness `analyze_image` (ingest) | 本調査の対象 |
|---|---|---|
| **入力** | Stella F12 スナップ or ROM （エミュ出力） | **任意画像（Photoshop PNG, モックアップ）** |
| **目的方向** | 逆方向：TIA 画面 → GRP/PF バイトへ解析 | 順方向：**デザイン画 → TIA バイトへ変換** |
| **色** | 実 TIA 画面から Gopher2600 パレット量子化（逆引き） | デザイン画の任意 RGB → Atari パレット量子化（前処理） |
| **PF** | 既存 TIA 画面から PF バンド抽出（litmus 100% 一致） | **デザイン画（非 TIA）から PF ビットマップ生成** |
| **GRP** | 既存 TIA 画面からスプライト抽出 | **デザイン画から GRP バイト生成** |
| **per-line 色** | 静的層として抽出 | 任意画像の per-line 2 色選択（要実装） |
| **減色** | 実施しない（TIA 画面は既に量子化済） | **任意 RGB → 128 色 NTSC パレットへの量子化** |

**結論**: ingest は「TIA 画面 ↔ データ」の逆方向解析に最適化されており、  
「**デザイン画（Photoshop モック）→ TIA データ**」の **順方向変換は既存 ingest の範囲外**。  
この gap を埋めるのが本調査。

---

## 2. ツール表

### A. atari-background-builder（Kirk Israel / kirkjerk）

| 項目 | 内容 |
|---|---|
| **URL** | https://github.com/kirkjerk/atari-background-builder |
| **Live** | https://alienbill.com/2600/atari-background-builder/ |
| **ライセンス** | GPL-3.0 |
| **Stars / 更新** | 3 stars / 2025-11-16（活発） |
| **入力** | JPG/PNG（ブラウザ `<input>` 経由）または手描き |
| **出力** | `.abb.json`（yxGrid[y][x] bool 40列 + colorGrid[y] hex + mode）、DASM ASM、batari Basic |
| **量子化手法** | **なし（単色ビットマップのみ）**。色は手動選択（`openAtariColorPicker()`）。PNG は**グレースケール平均しきい値（2値化）**で 40列 PF ビットマップへ縮小 |
| **縮小アルゴリズム** | `readSquare(x,y,contrast)`: 各 PF セル内ピクセルの `(R+G+B)/3` 平均を contrast 閾値と比較→ bool。contrast は `findGridWithMostContrastingNeighbors()` で自動チューニング |
| **PF バイト順** | CLAUDE.md の正典と一致（PF0 上ニブル col0→D4、PF1 MSB先 col4→D7、PF2 LSB先 col12→D0） |
| **制約** | PF のみ（GRP なし）。1 色（COLUPF）のみ per-line 設定可、多色グラデーションは手動。非対称 PF 対応（asym/mirror/repeat モード）。 |
| **採用判断** | GPL→コード流用不可。**アルゴリズムと JSON 形式は研究・取り込み可。PNG→PF の縮小＋2値化ロジックを harness の `cmd/ingest` 相当に移植する価値あり。** |

**実証済み実ファイル**: `inbox/211207_Pizza Boy/Designs/Playfield/atari-background-builder/pb_04.abb.json` – `mode:"SymPFMirrored"`, `height:40`, `FGColor:"7C"`, `BGColor:"4A"`, `yxGrid[y]` が 40要素 bool 配列, `colorGrid[y]` が TIA hex 文字列, `currentScanlinesPer:"4"`（複数スキャンラインで 1 PF 行を共有）。これがユーザーの実ワークフロー出力の正本。

---

### B. Dithertron（Steven Hugg / sehugg）

| 項目 | 内容 |
|---|---|
| **URL** | https://github.com/sehugg/dithertron |
| **Live** | https://8bitworkshop.com/dithertron/ |
| **ライセンス** | GPL-3.0（出力コードは CC0） |
| **Stars / 更新** | 101 stars / 2026-05-23（最も活発） |
| **入力** | JPG/PNG（ブラウザアップロード）、任意解像度 |
| **出力** | PNG（変換後画像）、バイナリ（.bin）、8bitworkshop IDE で表示できる ASM サンプル |
| **Atari 2600 対応** | `vcs`（40×192、2色/ライン）、`vcs.color`（VCSColorPlayfield_Canvas）、`vcs.48`（48×48ビットマップ） |
| **量子化パレット** | `VCS_RGB`（128色 NTSC）—ソースは独自テーブル（Stella や Gopher2600 とは微差あり） |
| **per-line 色選択** | `TwoColor_Canvas` を継承した `VCSColorPlayfield_Canvas`（`w=40, h=1`）: **1ライン1ブロック**として処理。2色はヒストグラム（現在色を重み 100、最近傍候補を重み 1 で積算）の上位 2 候補を選択。収束まで最大 100 イテレーション |
| **ディザリング** | Floyd-Steinberg, Atkinson, Sierra2, Sierra-Lite, Stucki, False Floyd（6種）+ 独自カーネル。誤差拡散は `diffuse=0.8` 係数＋ネイバーへのカーネル重み配分 |
| **距離関数** | `getRGBAErrorPerceptual`（知覚的色距離、RGB ユークリッドより優れる） |
| **ASM 出力** | `src/export/asm/vcs.asm`：非対称 PF（PFBitmap0-2=左, 3-5=右）。`COLUPF`/`COLUBK` は**フレーム全体で固定**（per-line 色変化は出力しない！） |
| **制約** | PF のみ（GRP スプライト出力なし）。per-line 2 色は内部で選択されるが **ASM 出力への反映がない**（バイナリとして埋め込まれるのみ、DASM テーブルとして取り出せない）。減色にはイテレーション収束まで数十秒。 |
| **採用判断** | **PF 全画面画像変換のベストツール**。live ツールとして D2 ループの「全画面背景デザイン確認」ステップとして使う。GRL は別途 PlayerPal か手動。**per-line COLUPF テーブルが欲しければ独自実装が必要**（既にアルゴリズムは解析済み）。 |

---

### C. vcsconv（g012）

| 項目 | 内容 |
|---|---|
| **URL** | https://github.com/g012/vcsconv |
| **ライセンス** | MIT（最も自由に使える） |
| **Stars / 更新** | 0 stars / 2017-04-18（放棄・参照のみ） |
| **入力** | 8 ビットインデックス PNG（任意サイズ）。減色は**外部ツールに委ねる** |
| **出力** | DASM `dc.b $XX,...`、CA65 `.byte $XX,...`、K65、バイナリ |
| **PF バイト順** | harness の正典と完全一致（PF1/PF4 逆順、PF0/2/3/5 正順）。ソース `vcsconv.c` の `cmd_playfield` で確認 |
| **GRP 出力** | `cmd_sprite`：8ピクセル幅、MSB=左端（`1<<i` i=7→0）→ DASM `dc.b` |
| **per-line 色** | `cmd_linecol`：各スキャンラインの「最初の非ゼロピクセル色」を 1 テーブルとして出力（COLUPF/COLUBK 用） |
| **制約** | PNG は 8bit インデックス形式のみ（true-color 不可）。量子化は外部依存（MtPaint 等）。2017 年以降更新なし。 |
| **採用判断** | MIT だが古くて入力形式が厳しい。**アルゴリズムの参照実装として有価値**（PF ビット順・GRP 符号化の C 実装）。実用は Dithertron か自前実装が優先。 |

---

### D. PlayerPal / PlayfieldPal（Kirk Israel）—既所有

- https://alienbill.com/2600/playerpal.html / playerpalnext.html
- **PNG インポートなし**（手描きのみ）。GRP バイト + DASM ASM を出力。Web ツール、ライセンス不明。
- **D2 ループでの役割**: スプライト設計（手描き）→ DASM コピー。PNG ルートへの橋渡しは別途必要。

---

### E. masswerk Tiny VCS Sprite/Playfield Editors—既所有

- https://www.masswerk.at/vcs-tools/
- **PNG インポートなし**。GRP バイト（MSB 先）と PF バイト（ラベル付き配列形式）を DASM ASM で出力。色なし（パターンのみ）。著作権保護（OSS ライセンスなし）。
- **D2 ループでの役割**: パターン確認・DASM テンプレート参照。

---

### F. Lospec / Photoshop ACO パレット

- https://lospec.com/palette-list/atari-2600-palette-ntsc-version（128色, ASE/GPL/PNG ダウンロード可）
- https://bywilliamjmeyer.itch.io/atari-2600-palette（ACO フォーマット、Photoshop Swatches へ直接 import 可）
- **D2 ループでの役割**: Photoshop で作業する際にスウォッチとして読み込む → デザイン段階から Atari パレットで作画 → PNG エクスポートの際に色はすでに最近傍量子化済み。**これが D2 ループの「前段フィルタ」**として最も安価。

---

## 3. 減色＋走査線毎色割当のアルゴリズム整理

### 3-1. 全体の処理ステップ

```
入力 PNG（任意 RGB）
  ↓
[Step 1] スケール縮小 → 40×192（PF）or 8×H（GRP）ピクセルへ
  ↓
[Step 2] 各ピクセルを NTSC 128色 への最近傍量子化
  ↓
[Step 3] 2600 制約へ落とす
    PF: 1ライン = 最大2色（COLUPF + COLUBK）→ ライン毎に2色選択
    GRP: 1スプライト = 1色（COLUP0/COLUP1）→ ライン毎に色変更で縦多色
  ↓
[Step 4] PF: PF0/PF1/PF2 バイト生成（harness pkg/playfield と同一ビット順）
         GRP: 8bit/行 GRP バイト生成（MSB=左端）
  ↓
[Step 5] DASM ASM 出力 or harness ingest への直接フィード
```

### 3-2. Step 2：NTSC 量子化

**選択肢（優れた順）:**

1. **Gopher2600 パレット（harness 推奨）**: `specification.SpecNTSC.GetColor()` から生成する Quantizer（`internal/ingest/palette.go`の `NewNTSCQuantizer()`）。**Stella F12 スナップとの照合で最も精度が高い**（avg_dist ≈ 0）。既に実装済み。
2. **Dithertron の `VCS_RGB`**: 独自テーブル（出典不明）。Gopher2600 とは微差あり（最大 ~32/255 チャンネル）。
3. **Lospec/Wikipedia 由来テーブル**: コミュニティ標準だが生成方式不明。

**距離関数:**
- RGB ユークリッド: `sqrt(dR²+dG²+dB²)` — 最もシンプル。harness の `Nearest()` がこれを使用。
- 知覚的距離: `getRGBAErrorPerceptual`（Dithertron）— 視覚的には優れるが複雑。
- **推奨**: harness 量子化器（RGB ユークリッド × Gopher2600 パレット）を順方向変換にも流用する。差 0 が保証されており、後段の ingest との一貫性が最も高い。

### 3-3. Step 3：走査線毎 2 色選択（PF の核心）

Dithertron の実装（実読確認済み、`src/dither/basecanvas.ts` `TwoColor_Canvas`）:

```
for each scanline:
    hist[c] = 0 (256 要素)
    for each pixel:
        hist[current_indexed_color] += 100   // 現在の色を優遇
        hist[closest_palette_match] += 1 + noise
    [c1, c2] = top2(hist)  // 上位 2 エントリを選択
```

**この2色選択の本質**: ヒストグラム頻度＋現在値への慣性（100:1 の重み）。これは k-means 的なクラスタリングではなく、収束を促すためのソフトな制約。最大 100 回反復して変化がゼロになるまで繰り返す。

**2600 制約との対応:**

| 2600 制約 | アルゴリズムの対応 |
|---|---|
| 1ライン = 背景色（COLUBK）+ 前景色（COLUPF）| `h=1` ブロックで 2 色選択 |
| 横方向は全ライン 1 色（横多色不可） | ブロック幅 `w=40`（全 PF 幅）で共通 2 色 |
| 縦方向は毎ライン色を変更可能 | 1 ライン = 1 ブロック = 独立した 2 色選択 |
| PF は 2 値ビットマップ（on/off） | 2 色選択後に最近傍でどちらかへ量子化 |

### 3-4. Step 3：GRP の縦多色（スプライト色）

GRP は 1 スプライト = 1 色（COLUP0/COLUP1）の制約がある（W2 の P2 ルール）。  
縦多色を出すには「スキャンラインごとに COLUPx を書き換える」しかない。

**順方向変換での実装案:**
- 入力画像の各スキャンラインの「スプライト領域内で最頻出の NTSC 色」を COLUP の候補とする
- GRP バイトは 8 ビット（on=COLUP色, off=背景）— 2 値化は同一ライン内の色の二乗距離で判定

### 3-5. Design-principles 整合チェック

W2 の原則との整合:
- **PF1（40列解像度）**: 縮小時に 40列へのリサンプリングが必要（4 color clock = 1列）。Dithertron の `scaleX:6` は 240px 入力に相当。atari-background-builder の `readSquare()` はセル平均。
- **C4（予算天井）**: per-line 色変化＋非対称 PF は CPU を食う。順方向変換ツールが出力する ASM はこの予算を考慮しない — 生成 ASM はあくまで**素材**であり、カーネルへの組み込みは手動。
- **PF2（対称/非対称）**: Dithertron は非対称 PF を強制出力。atari-background-builder は mode 選択可（SymPFMirrored, SymPFRepeated, asymmetric）。対称を選ぶと ROM バイトが半分になる。

---

## 4. harness ingest への推奨示唆

### 示唆 1（最優先）: abb.json を ingest の入力形式として正式サポート

**現状**: ユーザーは abb.json を手作業で DASM に転記。  
**示唆**: `cmd/ingest` に `-abb input.abb.json` フラグを追加し、  
`yxGrid` + `colorGrid` + `mode` + `currentScanlinesPer` から直接 DASM ASM を生成。  
PF ビット符号化は既存 `pkg/playfield.EncodeSymmetric/EncodeAsymmetric` をそのまま流用できる。  
Pizza Boy の実ファイル（`pb_04.abb.json`）でゼロ移植できる筋。

### 示唆 2: 順方向 ingest モード（PNG → TIA データ）

**現状**: `analyze_image` は TIA 画面（逆方向）を対象とする。  
**示唆**: `cmd/ingest -forward input.png` を追加し、  
任意 PNG を 40×H ビットマップに縮小 → Gopher2600 量子化器で per-line 2 色選択 → PF バイト生成。  
アルゴリズムは atari-background-builder の `readSquare()`（2値化）＋ Dithertron の `TwoColor_Canvas`（ヒストグラム 2 色選択）の組み合わせで実装可能。

### 示唆 3: per-line COLUPF テーブル生成

**現状**: Dithertron は per-line 2 色を内部で選択するが ASM テーブルとして出力しない。  
**示唆**: 順方向 ingest の出力に `COLUPF[y]` と `COLUBK[y]` の配列を追加し、  
DASM `dc.b` テーブルとして直接生成する。これで Photoshop モック → per-line 色テーブル＋PF バイトが 1 コマンドで揃う。

### 示唆 4: Photoshop パレットの統一

**現状**: ユーザーが Photoshop で作業する際のパレットと harness の量子化パレットが一致する保証がない。  
**示唆**: harness の Gopher2600 パレット 128 色から `palette.aco`（Photoshop 用）または `palette.gpl`（GIMP 用）を生成するコマンド（`cmd/genpalette` など）を追加。Photoshop 作業時点から Gopher2600 パレットで作画できれば、後段の量子化誤差がゼロになる。

---

## 5. 未解決・要実機確認

| 項目 | 状況 |
|---|---|
| Dithertron の `VCS_RGB` パレットと Gopher2600 パレットの差分量 | 未測定。実測が必要（最大差が画質に影響するか） |
| Dithertron の per-line 2 色をどう ASM に取り出すか | 内部 `params[]` 配列はアクセス可能だが、export 関数が実装されていない。fork か自前実装が必要 |
| atari-background-builder の `currentScanlinesPer` の DASM への反映 | abb.json に `"4"` があるが、生成 ASM でどう扱うか（1 PF バイト = N スキャンライン）の実装は未確認 |
| GRP 順方向変換（8px スプライトへの任意画像縮小）の精度 | 縦多色スプライトの色割当て精度は未検証。PlayerPal が現実的な上限を示しているが自動化は未実装 |
| PF ブロック境界のアーチファクト（Dithertron）| `vcs.color` モードで実際に生成した ROM の見栄えを Gopher2600 で確認していない |

---

## 6. 最初に試すべき 1 ツール

**Dithertron（https://8bitworkshop.com/dithertron/）のブラウザ版**。

理由:
1. Photoshop から PNG を書き出し、ドラッグ&ドロップで即変換できる
2. `vcs.color` モードで per-line 2 色の PF 変換を視覚的に確認できる
3. ソースが実読済みで、出力が PF バイト（non-ASM binary）として取り出せる構造を把握済み
4. 出力 ASM を harness で `assemble_and_load` してすぐ確認できる

**ただし**：COLUPF/COLUBK の per-line テーブルが ASM に出力されないのが最大の欠点。この欠点を克服する最短路は「示唆 1」の abb.json サポート（ユーザーは既に atari-background-builder で色も設定済みの .abb.json を持っている）。

---

## 出典 URL

- atari-background-builder ソース（`splash.js` / `ataricolors.js`）: https://github.com/kirkjerk/atari-background-builder
- Dithertron システム定義: https://github.com/sehugg/dithertron/blob/master/src/settings/systems.ts
- Dithertron TwoColor_Canvas（ヒストグラム 2 色選択）: https://github.com/sehugg/dithertron/blob/master/src/dither/basecanvas.ts
- Dithertron VCS ASM テンプレート: https://github.com/sehugg/dithertron/blob/master/src/export/asm/vcs.asm
- vcsconv PF/GRP ビット順・DASM 出力: https://github.com/g012/vcsconv/blob/master/vcsconv.c
- Lospec NTSC パレット（ASE/GPL 配布）: https://lospec.com/palette-list/atari-2600-palette-ntsc-version
- Photoshop ACO パレット: https://bywilliamjmeyer.itch.io/atari-2600-palette
- harness `pkg/playfield` ビット順（litmus 検証済み）: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/pkg/playfield/playfield.go
- harness `internal/ingest/palette.go`（Gopher2600 量子化器）: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/internal/ingest/palette.go
- Pizza Boy abb.json 実ファイル: /Users/shinji/Documents/2D/260609_atari2600-dev/inbox/211207_Pizza Boy/Designs/Playfield/atari-background-builder/pb_04.abb.json
- Atari Projects / Dithertron チュートリアル: https://atariprojects.org/2020/12/06/display-a-digital-photo-on-an-atari-2600-10-15-mins/
