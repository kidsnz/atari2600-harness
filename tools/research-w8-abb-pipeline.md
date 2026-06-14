# research-w8 — abb.json → DASM 実装詳細化（`cmd/ingest -abb`）

調査日: 2026-06-13。
前提: `research-w7-image-to-2600-pipeline.md` の結論「ユーザーの `.abb.json`（atari-background-builder
エクスポート）を `cmd/ingest -abb` で直接 DASM に変換するのが最短 D2 ルート」を **実装できるレベルまで詰める**。

実装はしない（自走中は出荷しない方針）。本書は schema 確定／マッピング設計／関数シグネチャ案／
最初に書く最小コードの順序まで。

---

## 1. `.abb.json` schema 確定（実ファイル＋GitHub 根拠）

### 1-1. GitHub 根拠（エクスポート構造）

`splash.js` の save ロジック（WebFetch 確認、kirkjerk/atari-background-builder master）:

```js
const project = {
  project: currentProjectname,
  mode: currentKernelModeName,
  height: H,
  tvmode: currentTVMode,
  FGColor: currentFGColor,
  BGColor: currentBGColor,
  yxGrid,        // [y][x] bool, 40 列固定
  colorGrid,     // [y] hex 文字列（前景 COLUPF/row）
  colorBgGrid,   // [y] hex 文字列（背景 COLUBK/row）。未使用時は []
  currentScanlinesPer
};
```

mode 文字列の全集合（`splash-modes.js` の `modes` オブジェクトのキー、WebFetch 確認）:
`player48color` / `player48mono` / `SymPFMirrored` / `SymPFRepeated` / `AssymPFRepeated` /
`AssymPFMirrored` / `bbPFDPCcolors` / `bBTitle_48x2`。
本パイプラインが対象とするのは **PF 4 モード**: `SymPFMirrored` / `SymPFRepeated` /
`AssymPFRepeated` / `AssymPFMirrored`（player48* は GRP 48px スプライト＝別件）。

### 1-2. 実ファイルで確定した schema（Pizza Boy `pb_04.abb.json`）

```
project:  "pb_04"
mode:     "SymPFMirrored"
height:   40
tvmode:   "ntsc"
FGColor:  "7C"          ← 既定前景色（TIA hex 生値）
BGColor:  "4A"          ← 既定背景色（TIA hex 生値）
yxGrid:   [40 行][40 列] bool        ← 行数 == height
colorGrid:[75] hex 文字列            ← 行毎前景色。先頭 height 個が有効、残りは editor 余白
colorBgGrid: []                      ← 空 = 行毎背景上書きなし（BGColor 一律）
currentScanlinesPer: "4"             ← 1 PF 行 = 4 走査線（文字列）
```

**寸法**: `yxGrid` は常に 40 列固定（PF の 40 列＝各 4 color clock 幅）。行数は `height`。
PF 4 モードでは実測上 **`len(yxGrid) == height`**（07/05=44 行/44, pb_01..04=40/40）。

> **異常値の警告**: `Title/220814/title_220814_01.abb.json`（`AssymPFRepeated`, height 44）は
> `yxGrid` が 256 行で、非空行は index 2..42 の 39 行のみ。height と行数が一致しない旧版／
> canvas パディング込みの出力。**実装は `height` を信頼せず `len(yxGrid)` を一次採用し、
> 末尾の全 false 行は trim、height と不一致なら WARN を吐く**のが安全。PF サンプル（pb_*）は
> 全て一致しているのでまず pb_* で通す。

**色の持ち方（確定）**: `colorGrid`/`FGColor`/`BGColor` は **TIA レジスタ生値の 16 進文字列**
（独自 index ではない）。根拠:
- 値が全て偶数 hex（`7C`,`4A`,`0E`,`AC`,`6A`,`00`）＝ COLUxx の D0（未使用ビット）が 0。
- ABB の `openAtariColorPicker()` が TIA 128 色を直接選ばせる UI。
- harness の playfield 規約（COLUPF に書く値＝hue/lum、D0 無視）と一致。

**colorGrid の意味**: per-row 前景色。pb_04 では先頭 40 個が全て `7C`（＝この絵は前景単色）。
index 40..74 の `0E` は **editor の余白行の色**で出力データではない（height で切る）。
**`colorGrid[y]` は前景＝COLUPF[y]、`colorBgGrid[y]` があれば背景＝COLUBK[y]**。
`colorBgGrid` が `[]` の場合は全行 `BGColor` 一律。

**mode → CTRLPF / 対称性 マッピング**（命名＋AtariAge 説明「Asymmetric, Mirrored, or Repeated」より）:

| mode 文字列 | 対称性 | CTRLPF D0 | yxGrid 使用列 | harness エンコーダ |
|---|---|---|---|---|
| `SymPFMirrored`  | 対称 | 1 = reflect | 左 20 列（col0..19）| `EncodeSymmetric` |
| `SymPFRepeated`  | 対称 | 0 = repeat  | 左 20 列（col0..19）| `EncodeSymmetric` |
| `AssymPFRepeated`| 非対称| 0 = repeat※ | 40 列全部 | `EncodeAsymmetric` |
| `AssymPFMirrored`| 非対称| 1 = reflect※| 40 列全部 | `EncodeAsymmetric` |

> ※ Assym では左右を別データで描く（6 バイト/行）ので CTRLPF D0 はハードのミラー機能としては
> 効かない。Assym*Mirrored/Repeated の差は ABB 内部の右半生成方法の名残で、出力 DASM では
> 左右 6 バイトが両方入る。**実装上は Assym = `EncodeAsymmetric`（CTRLPF D0 は 0 固定）で良い**。
> Sym の Mirrored/Repeated の差だけが CTRLPF D0 に直結する（ここが重要）。

**実測の対称性検証**（pb_04, `SymPFMirrored`）: 全 40 行で `right == reverse(left)`＝鏡像。
左 20 列だけ取れば `EncodeSymmetric` でゼロロスで符号化できる（右半はハードが reflect で生成）。

---

## 2. マッピング設計（abb json → pkg/playfield → DASM）

### 2-1. 走査線ゾーン（縦多色）の持たせ方

design-principles「PF は横 1 色・縦に色を足す」と完全整合する。abb は **per-row 前景色
（colorGrid）と任意の per-row 背景色（colorBgGrid）を既に持っている** ので、
連続する同色・同パターンの行を **band（PFBand）にまとめる**だけでよい。

```
abb 行 y:  (yxGrid[y] の左20列 or 40列) + colorGrid[y] (+ colorBgGrid[y])
   │  連続行で (pfbytes, fg, bg) が同一なら 1 band に畳む
   ▼
band: { topRow, rowCount, pf0/1/2 (+pf0b..), fg=COLUPF, bg=COLUBK }
   │  各 band の走査線高 = rowCount × currentScanlinesPer
   ▼
DASM: PF 3(or6) バイトテーブル + COLUPF テーブル + COLUBK テーブル + 高さテーブル
```

`currentScanlinesPer`（pb は "4"）は **1 abb 行が画面上 4 走査線を占める** という縮尺。
DASM 側はカーネルのループ回数（band 高 = rowCount×4 走査線）に反映する。テーブル自体は
abb 行解像度（1 エントリ＝4 走査線）で持ち、カーネルが各エントリを 4 回 WSYNC するのが素直。

### 2-2. マッピング表

| abb フィールド | 変換先 | 備考 |
|---|---|---|
| `yxGrid[y][0..19]` (Sym) | `EncodeSymmetric` → PF0/PF1/PF2 | 左半 20 列。右半はハード reflect/repeat |
| `yxGrid[y][0..39]` (Assym)| `EncodeAsymmetric` → AsymRow(6byte)| 左右独立 6 バイト |
| `mode` Sym*Mirrored | CTRLPF |= REFLECT (D0=1) | 一度だけ書く（全 band 共通） |
| `mode` Sym*Repeated | CTRLPF D0=0 | repeat |
| `mode` Assym* | CTRLPF D0=0、左右 6byte で描画 | ミラーはデータで実現 |
| `FGColor` / `colorGrid[y]` | COLUPF (per-row → band で畳む) | TIA 生値。colorGrid 優先、無効域は FGColor |
| `BGColor` / `colorBgGrid[y]`| COLUBK (per-row → band) | colorBgGrid 空なら BGColor 一律 |
| `currentScanlinesPer` | band 高 = rowCount × spl 走査線 | カーネルループ回数 |
| `height` | 検証用（len(yxGrid) と照合、不一致は WARN）| 一次は len(yxGrid) |

### 2-3. per-line COLUPF の結論（w7 の Dithertron 弱点への回答）

**abb は per-row 前景色 `colorGrid` を持つので、Dithertron が抱えた「per-line 色を
ASM テーブルに出せない」弱点は abb ルートには存在しない**。abb json から COLUPF[y] /
COLUBK[y] をそのまま DASM `byte` テーブルに落とせる（行内 timed-write は不要＝PF 横 1 色制約に
合致、縦は band 毎に COLUPF を書き換えるだけ）。
→ **追加の per-line 色推定アルゴリズムは不要**。abb の color フィールドを忠実に転記すれば足りる。
（ingest 側 PFBand の `ColorWrites`＝行中書換は abb には対応物が無いので使わない。）

---

## 3. `cmd/ingest -abb` 実装設計（設計のみ）

### 3-1. 入出力

```
go run ./cmd/ingest -abb path/to/foo.abb.json -out out_dir/
  入力: .abb.json 1 枚
  出力: out_dir/foo.asm   … PF データ + 色テーブル + 高さテーブル + CTRLPF 設定コメント
        （overlay.png は不要。-abb は順方向データ生成なので report.json/overlay は出さない）
```

既存 `-in`（スクショ逆解析）とは排他のサブ経路。`-abb` 指定時は別パス（`runABB`）へ分岐し、
画像 normalize/quantize を一切通さない（abb は既に TIA 生値を持つため量子化器不要）。

### 3-2. 既存資産の流用点

| 既存 | 流用 |
|---|---|
| `pkg/playfield.EncodeSymmetric([]bool) (pf0,pf1,pf2)` | Sym モードの行符号化（そのまま） |
| `pkg/playfield.EncodeAsymmetric([]bool) AsymRow` | Assym モードの行符号化（そのまま） |
| `internal/ingest.PFBand` 構造体 | band 表現を再利用（Top/Height/Mode/PF*/ColorLeft/ColorRight）|
| `internal/ingest.DASMPlayfield([]PFBand) string` | 既存 DASM emitter を **拡張 or 並置** |

`EncodeSymmetric`/`EncodeAsymmetric` は CLAUDE.md のビット順検証済み（litmus_pf + read_row 100%）。
abb の yxGrid も同じ「左→右 40 列、col0 が最左」規約（w7 で ABB=harness 一致を確認済）なので
**列の並べ替え不要・ゼロ移植**。

### 3-3. 必要な最小コード（関数シグネチャ案）

新規ファイル `internal/ingest/abb.go`:

```go
// ABBProject は .abb.json のデコード先（PF 4 モード用フィールドのみ）。
type ABBProject struct {
    Project   string     `json:"project"`
    Mode      string     `json:"mode"`      // SymPFMirrored 等
    Height    int        `json:"height"`
    TVMode    string     `json:"tvmode"`
    FGColor   string     `json:"FGColor"`   // TIA hex "7C"
    BGColor   string     `json:"BGColor"`
    YXGrid    [][]bool   `json:"yxGrid"`    // [row][40col]
    ColorGrid []string   `json:"colorGrid"` // per-row 前景 hex（>=height、余白込み）
    ColorBg   []string   `json:"colorBgGrid"`
    Scanlines string     `json:"currentScanlinesPer"` // "4"
}

// LoadABB は .abb.json を読み、行数 trim と height 照合（不一致は warn）を行う。
func LoadABB(path string) (*ABBProject, []string /*warnings*/, error)

// ABBToBands は abb プロジェクトを PFBand 列へ畳む。
// - Sym: 各行 EncodeSymmetric(left20)、CTRLPF はモードで決定（呼び出し側で 1 回設定）
// - Assym: 各行 EncodeAsymmetric(40)、PF*B も埋める
// - colorGrid[y]/colorBgGrid[y] を COLUPF/COLUBK に、無効域は FG/BGColor
// - 連続する同 (pfbytes,fg,bg) 行を 1 band に統合。Height は abb 行数（×spl は DASM 側）
func (p *ABBProject) ToBands() (bands []ingest.PFBand, reflect bool, spl int, err error)

// DASMFromABB は bands + メタを貼れる DASM へ。
// CTRLPF コメント、PF テーブル、COLUPF[]/COLUBK[]/HEIGHT[] テーブルを出力。
func DASMFromABB(p *ABBProject, bands []ingest.PFBand, reflect bool, spl int) string

// 色 hex → uint8（"7C" → 0x7C）。D0 はそのまま（COLUxx は D0 無視で表示）。
func parseTIAHex(s string) (uint8, error)
```

`cmd/ingest/main.go` 側の最小追加:

```go
abb := flag.String("abb", "", "atari-background-builder .abb.json → DASM PF")
...
if *abb != "" { return runABB(*abb, *out) } // 既存 run() より前で分岐
```

### 3-4. 出力 DASM の形（イメージ）

```asm
; from pb_04.abb.json  mode=SymPFMirrored  spl=4  rows=40  -> 160 scanlines
; CTRLPF: REFLECT (D0=1)   COLUBK base=$4A
PFData0  byte $F0,$F0,...   ; PF0 per band (上ニブルのみ)
PFData1  byte $00,...       ; PF1
PFData2  byte $0F,...       ; PF2
PFColu   byte $7C,$7C,...   ; COLUPF per band
PFColuBk byte $4A,$4A,...   ; COLUBK per band
PFHeight byte 4,4,...       ; 各 band の走査線数（rowCount*spl）
```

既存 `DASMPlayfield` は band 列を `byte $xx,$xx,$xx` で吐く形なので、**色テーブルと高さテーブルを
足した abb 専用 emitter（`DASMFromABB`）を新設**するのが衝突がなく安全（既存 emitter は逆解析用に温存）。

---

## 4. 最初に書く最小コードの順序

1. `internal/ingest/abb.go` に `ABBProject` 構造体 + `LoadABB`（JSON decode・行 trim・height 照合 warn）。
   → **まず `pb_04.abb.json` を読んで構造体に入る**ことだけ確認（テストで dump）。
2. `parseTIAHex` + 単体テスト（"7C"→0x7C, "0E"→0x0E）。
3. `ToBands`（Sym のみ先行：`EncodeSymmetric(left20)` を全行→colorGrid を畳んで band 化、
   reflect=（mode に "Mirrored"）。**pb_04 で band 数・PF バイトを golden 照合**）。
4. `DASMFromABB`（テーブル emit）。**出力 .asm を `assemble_and_load` で組んで通す**。
5. `cmd/ingest -abb` 分岐 + `runABB`。pb_01..04・07 を一括で .asm 化して全部 dasm 緑を確認。
6. （後追い）Assym 対応：`EncodeAsymmetric` 経路 + 6 バイト emit。title 系の height 不一致を warn 処理。

**検証の起点**: Sym × pb_04 が「band 化 → DASM → assemble → Gopher2600 で `read_row` が
yxGrid と一致」まで通れば、abb→DASM は本物（litmus 流の数値裏取り）。

---

## 5. 出典

- abb save 構造（splash.js `const project = {...}`）: https://github.com/kirkjerk/atari-background-builder/blob/master/splash.js
- mode 文字列集合（SymPFMirrored 等）: https://github.com/kirkjerk/atari-background-builder/blob/master/splash-modes.js
- mode 説明「Asymmetric, Mirrored, or Repeated」: https://forums.atariage.com/topic/319884-re-introducing-the-atari-background-builder-formerly-splash-o-matic/
- 実ファイル pb_04（mode=SymPFMirrored, height40, FG7C/BG4A, spl4, 鏡像対称を実測）:
  /Users/shinji/Documents/2D/260609_atari2600-dev/inbox/211207_Pizza Boy/Designs/Playfield/atari-background-builder/pb_04.abb.json
- 実ファイル群（mode/height/spl 確認）: 同 dir pb_01..03, 05, 07（Sym*）／ Title/220814/title_220814_01（Assym, height≠rows 異常）
- 流用エンコーダ（ビット順 litmus 検証済）: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/pkg/playfield/playfield.go:46,59
- 既存 DASM emitter / PFBand: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/internal/ingest/emit.go:199, segment.go:8
- cmd/ingest フラグ構造: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/cmd/ingest/main.go:21
- w7 前提調査: /Users/shinji/Documents/2D/260609_atari2600-dev/harness/tools/research-w7-image-to-2600-pipeline.md
