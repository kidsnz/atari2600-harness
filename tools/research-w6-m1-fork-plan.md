# research-w6-m1-fork-plan — M1 着手手順書（spritemate fork → TIA Studio M1）

前提研究: `research-w3-buildable-apis.md`（API地図）/ `research-w4-m1-open-questions.md`（colorPerRow 確定・framebuffer 二段構成確定）。
本書は「再調査なし」。w3/w4 の結論を**今すぐ着手できる具体的な fork 手順**に落とす。
収集日 2026-06-13。**[確認]=GitHub ソース実読 / [推定]=実装方針**。出典 URL（ファイル/行）必須。

---

## 1. リポ構造とビルド

### clone → dev サーバ起動

```bash
cd /Users/shinji/Documents/2D/260609_atari2600-dev
git clone git@github.com:YOUR_GITHUB/spritemate.git tia-studio   # fork 後の自分のリポ
cd tia-studio
npm install          # jszip のみ（devDeps は vite + ESLint + prettier + TS）
npm run dev          # http://localhost:5173 で即起動
npm run build        # dist/ に出力（vite.config.js: outDir:'dist', base:'./'）
```

出典:
- `package.json` — scripts: `"dev":"vite"` / `"build":"vite build"` [確認]
  https://github.com/Esshahn/spritemate/blob/main/package.json
- `vite.config.js` — `base:'./'`, `outDir:'dist'` [確認]

### エントリポイント

```
index.html: <script type="module" src="./src/js/App.ts"></script>
            <div id="app"></div>
```

`App.ts` 末尾（L2097付近）:
```ts
document.addEventListener("DOMContentLoaded", function () {
  window.app = new App(get_config());
});
```
[確認] https://github.com/Esshahn/spritemate/blob/main/src/js/App.ts

### ディレクトリ構成（src/js の主要28ファイルと役割）

```
tia-studio/
├── index.html              エントリ HTML
├── package.json            vite 1.2.1, jszip 依存
├── vite.config.js          base:'./', outDir:'dist', alias '@'→'./src'
├── tsconfig.json
└── src/
    └── js/
        ├── config.ts         ★改修① 寸法・パレット。get_config() が全モジュールに渡る
        ├── SpriteTypes.ts    ★改修② colorPerRow 拡張。SpriteData / SpriteCollection / SpriteHelpers
        ├── Sprite.ts         ★改修③ flip/shift/undo が colorPerRow を同期
        ├── Editor.ts         ★改修④ 塗り色 colorPerRow[y] 参照 + 行ごと色 UI
        ├── Preview.ts        ★改修⑤ render_pixels が colorPerRow[y] 参照
        ├── Window_Controls.ts ★改修⑤ render_pixels 共通ロジック（Preview の親クラス）
        ├── Export-Base.ts    ★改修⑥ create_tia_assembly() 新規追加
        ├── TiaPreview.ts     ★新規⑦ Stellerator embed
        ├── App.ts            ノータッチ（モジュール仲介のみ）
        ├── Palette.ts        ノータッチ（config.palettes 差し替えで自動追従）
        ├── ImportPNG.ts      ノータッチ
        ├── Import.ts         ノータッチ
        ├── Load.ts           ノータッチ
        ├── Save.ts           ノータッチ
        ├── Animation.ts      ノータッチ
        ├── List.ts           ノータッチ
        ├── Tools.ts          ノータッチ
        ├── Storage.ts        ノータッチ
        ├── Window.ts         ノータッチ
        ├── helper.ts         ノータッチ
        └── (About/Dialog/IconStateManager/Playfield/Snapshot/Settings/Sortable/Tooltip) ノータッチ
```

---

## 2. 改修ポイントの正確な特定（ファイル / 関数 / 行）

### 改修① `src/js/config.ts` — 寸法・パレット差し替え

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/config.ts

**変更点A: スプライト寸法（L5-6）**

```ts
// 変更前
sprite_x: 24,   // L5
sprite_y: 21,   // L6

// 変更後（TIA GRP は 8px 幅; 高さは可変、M1 初期値 = 16 scanlines）
sprite_x: 8,
sprite_y: 16,
```

`Editor.ts` は `config.sprite_x * grid_width` で canvas を組む [確認 Editor.ts L40-43]。
`Sprite.ts` はコンストラクタで `this.width = config.sprite_x` / `this.height = config.sprite_y` [確認 Sprite.ts L11-12]。
→ **寸法変更だけで Editor/Preview/List のキャンバスが全部追従**する。

**変更点B: sprite_defaults (L7-15)**

```ts
sprite_defaults: {
  background_color: 0,        // TIA $00=黒（パレット index 0）
  individual_color: 7,        // TIA $0E=白相当（index 7 = $0E / 2 = 7）
  multicolor_1: 0,
  multicolor_2: 0,
  pen: 1,
  animation_fps: 10,
  animation_mode: "restart",
},
```

**変更点C: palettes（L25-62）→ Stella NTSC 128色**

C64 の4パレットを削除し `stella_ntsc` 128色に差し替える。
正本: `harness/internal/ingest/palette_stella.go` の `stellaNTSC [128][3]uint8`（L9）
インデックス規則: `index = TIA_reg / 2`（index 0 = $00 / index 127 = $FE）

変換スクリプト（one-shot）:
```js
// scripts/gen_stella_palette.js
const fs = require('fs');
const lines = fs.readFileSync('../../harness/internal/ingest/palette_stella.go', 'utf8').split('\n');
const colors = [];
for (const l of lines) {
  const m = l.match(/\{0x([0-9A-F]{2}), 0x([0-9A-F]{2}), 0x([0-9A-F]{2})\}/i);
  if (m) colors.push(`"#${m[1]}${m[2]}${m[3]}"`);
}
console.log(colors.join(', '));  // 128エントリ出力
```

config.ts 差し替え後:
```ts
palettes: {
  stella_ntsc: {
    name: "Stella NTSC",
    values: [
      "#060606", "#343434", "#5C5C5C", /* ... 128エントリ ... */ "#FFDB67",
    ],
  },
},
selected_palette: "stella_ntsc",
color_names: ["$00","$02","$04",/* ... */,"$FE"],   // 128要素
```

**Export 時の index → TIA レジスタ変換**: `reg = colorPerRow[y] * 2`（例: index 7 → $0E）

---

### 改修② `src/js/SpriteTypes.ts` — colorPerRow 拡張

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/SpriteTypes.ts

**`SpriteData` インターフェース（L17-26）**

```ts
// 変更前（L19）
color: number;

// 変更後（L19）
colorPerRow: number[];  // length === pixels.length（sprite_y ぶん）
                        // 値 = Stella palette index（0-127）; export 時 * 2 → TIA reg
// multicolor/double_x/double_y/overlay は M1 では false 固定のため削除可（後で復活）
```

**`SpriteHelpers.createSprite`（L71-84）**

```ts
// 変更前（L77）
color: params.color ?? config.sprite_defaults.individual_color,

// 変更後
colorPerRow: params.colorPerRow ?? Array(config.sprite_y).fill(config.sprite_defaults.individual_color),
```

---

### 改修③ `src/js/Sprite.ts` — flip/shift/undo が colorPerRow を同期

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/Sprite.ts

**A. `flip_vertical()`（L109-139）else ブランチに追加**:
```ts
this.currentSprite.pixels.reverse();
this.currentSprite.colorPerRow.reverse();   // ★追加
```
selection bounds ブランチ: rows[] 反転と同じロジックで colorPerRow の部分配列も反転。

**B. `shift_vertical()`（L162-201）else ブランチに追加**:
```ts
if (direction === "down") {
  s.pixels.unshift(s.pixels.pop());
  s.colorPerRow.unshift(s.colorPerRow.pop());   // ★追加
} else {
  s.pixels.push(s.pixels.shift());
  s.colorPerRow.push(s.colorPerRow.shift());    // ★追加
}
```

**C. `save_backup()`（L355-360）/ `copy/paste`（L436-449）**: 追加コード不要。
`deepClone(this.all)` が配列を含む全プロパティを再帰複製するため `colorPerRow` は自動追従。

---

### 改修④ `src/js/Editor.ts` — 塗り色 colorPerRow[y] 参照 + 行ごと色 UI

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/Editor.ts

**A. `renderSpriteAtPosition()`（L165-194）色解決変更**:
```ts
// 変更前（L180-183）
let color: number;
if (array_entry === 1 || !sprite_data.multicolor) {
  color = sprite_data.color;
} else {
  color = all_data.colors[array_entry];
}

// 変更後（M1: 1bit/px, multicolor 不要）
let color: number;
if (array_entry === 1) {
  color = sprite_data.colorPerRow[j];  // j = y ループ変数（L172の for j）
}
this.canvas.fillStyle = this.config.colors[color];
```

**B. 行ごと色 UI（新規追加）**:
```ts
renderColorStrip(sprite_data: SpriteData): void {
  const strip_x = this.config.sprite_x * this.zoom + 4;
  for (let y = 0; y < this.config.sprite_y; y++) {
    const colorIdx = sprite_data.colorPerRow[y];
    this.canvas.fillStyle = this.config.colors[colorIdx];
    this.canvas.fillRect(strip_x, y * this.zoom, 12, this.zoom);
  }
}
// mousedown listener: strip 領域クリック → colorPerRow[y] = selectedPen → save_backup()
```

PlayerPal（https://alienbill.com/2600/playerpalnext.html）の side toggle の相当物。

---

### 改修⑤ `src/js/Window_Controls.ts` — colorPerRow[y] 参照

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/Window_Controls.ts

`Preview.ts` は `render_pixels(sprite_data, all_data)` を親クラス `Window_Controls` に委譲 [確認 Preview.ts L47-48]。
`Window_Controls.render_pixels` の色解決も改修④Aと同じ変更を適用:
```ts
// 変更前: color = sprite_data.color
// 変更後: color = sprite_data.colorPerRow[y]  (y = 行インデックス)
```

---

### 改修⑥ `src/js/Export-Base.ts` — `create_tia_assembly()` 追加

**ファイル**: https://github.com/Esshahn/spritemate/blob/main/src/js/Export-Base.ts

旧 `create_assembly`（L147-279、C64向け）は残置。DASM 出力用メソッドを新規追加:

```ts
create_tia_assembly(label_prefix: string = ""): string {
  let out = "";
  for (const sp of this.savedata.sprites) {
    const name = label_prefix + sp.name;
    const h = sp.pixels.length;   // sprite_y

    // GRP グラフィックテーブル
    out += `${name}_gfx:\n`;
    for (let y = 0; y < h; y++) {
      let bits = "";
      for (let x = 0; x < 8; x++) {
        bits += sp.pixels[y][x] === 0 ? "0" : "1";  // MSB-first, 1bit/px
      }
      out += `    .byte %${bits}   ; scanline ${y}\n`;
    }

    // COLUP カラーテーブル
    out += `${name}_col:\n`;
    for (let y = 0; y < h; y++) {
      const regVal = sp.colorPerRow[y] * 2;   // index → TIA reg
      const hex = regVal.toString(16).padStart(2, "0").toUpperCase();
      out += `    .byte $${hex}   ; scanline ${y} COLUP0\n`;
    }

    out += `${name}_h = ${h}\n\n`;
  }
  return out;
}
```

出力例（sprite_y=3）:
```asm
player0_gfx:
    .byte %01111110   ; scanline 0
    .byte %11111111   ; scanline 1
    .byte %01111110   ; scanline 2
player0_col:
    .byte $0E         ; scanline 0 COLUP0
    .byte $44         ; scanline 1 COLUP0
    .byte $0E         ; scanline 2 COLUP0
player0_h = 3
```

カーネル側の使い方（参考）:
```asm
    ldy player0_h - 1
loop:
    lda player0_gfx,y
    sta GRP0
    lda player0_col,y
    sta COLUP0
    dey
    bpl loop
```

---

## 3. 再利用できてノータッチで済む部分

| モジュール | 再利用機能 | 理由 |
|---|---|---|
| `Sprite.ts` undo/redo | deepClone backup | colorPerRow は deepClone に自動追従 |
| `Sprite.ts` floodfill / flip-X / shift-X / copy-paste | pixels 配列操作 | 色変更（flip-Y/shift-Y）以外はノータッチ |
| `ImportPNG.ts` | PNG → pixels 量子化 | pixels[y][x] 形式そのまま。Stella パレットへの量子化は config 差し替えで追従 |
| `Palette.ts` | カラー選択 UI | config.palettes 差し替えで Stella 128色が自動表示 |
| `App.ts` | 全モジュール仲介 / キーボードショートカット / 自動保存 | 完全ノータッチ |
| `Animation.ts` | アニメ再生 | pixels 配列を frame ループするだけ |
| `Storage.ts` | localStorage 永続化 | `all` オブジェクトごと保存なので colorPerRow 自動追従 |
| `Import.ts` / `Load.ts` | .spm JSON 読込 | 旧ファイルは load 時に `colorPerRow = Array(h).fill(0)` で移行 |
| `List.ts` / `Tools.ts` / `Save.ts` / `About.ts` 等 | そのまま | ノータッチ（計19ファイル） |

---

## 4. ライブプレビュー配線（Stellerator embed）

### worker 取得

```bash
# GitHub release 1.1.2 から stellerator-embedded.zip を取得
curl -L https://github.com/6502ts/6502.ts/releases/download/1.1.2/stellerator-embedded.zip \
  -o /tmp/stellerator.zip
unzip /tmp/stellerator.zip -d /tmp/stellerator
# 中身: stellerator-embedded.js (frontend) + stellerator.js (worker) + *.map
cp /tmp/stellerator/stellerator.js tia-studio/public/stellerator.js
```

[確認] https://github.com/6502ts/6502.ts/releases/download/1.1.2/stellerator-embedded.zip

npm frontend:
```bash
npm install 6502.ts   # lib/web/embedded/stellerator/Stellerator.ts が入る
```

[確認] https://github.com/6502ts/6502.ts/blob/master/src/web/embedded/stellerator/Stellerator.ts

### 最小 embed コード（新規 `src/js/TiaPreview.ts`）

```ts
import Stellerator from '6502.ts/lib/web/embedded/stellerator/Stellerator';

export class TiaPreview {
  private st: Stellerator | null = null;

  async init(canvas: HTMLCanvasElement): Promise<void> {
    this.st = new Stellerator(canvas, '/stellerator.js', {
      tvEmulation: Stellerator.TvEmulation.none,
      scalingMode: Stellerator.ScalingMode.qis,
      phosphorLevel: 0,
      scanlineLevel: 0,
      gamma: 1,
    });
  }

  async loadRom(romBytes: Uint8Array): Promise<void> {
    if (!this.st) return;
    await this.st.run(romBytes, Stellerator.TvMode.ntsc);
  }
}
```

`App.ts` への追加（既存パターンに倣い1行ずつ）:
```ts
this.tiaPreview = new TiaPreview();
await this.tiaPreview.init(document.getElementById('tia-preview') as HTMLCanvasElement);
```

### codegen ROM bytes → Stellerator の最小フロー

M1 初期（assemble API なし）:
```ts
// <input type="file"> で手動 .bin 読み込み → Stellerator に流す
input.onchange = async (e) => {
  const bytes = new Uint8Array(await (e.target as HTMLInputElement).files![0].arrayBuffer());
  await this.tiaPreview.loadRom(bytes);
};
```

将来（assemble API あり）:
```ts
const asm = this.export.create_tia_assembly();
const resp = await fetch('/api/assemble', { method:'POST', body: asm });
const romBytes = new Uint8Array(await resp.arrayBuffer());
await this.tiaPreview.loadRom(romBytes);
```

**M1 ではサーバ Gopher2600 照合は後回し**:
- プレビューは「絵が canvas に出るか」の視覚確認専任
- 照合の正本は harness の MCP ツール（`assemble_and_load` + `get_screen_annotated`）
- ブラウザ内 ImageData フック（`VideoEndpointInterface.newFrame`、5-8行）は M5 まで保留 [確認 w4 §Q2-c]

**worker ホスト制約**: same-origin 必須（CORS）。`npm run dev` では `public/stellerator.js` → `/stellerator.js` で参照可。
[確認] https://github.com/6502ts/6502.ts/blob/master/doc/stellerator_embedded.md

---

## 5. M1 作業分解（最初のコミットから動く最小形まで）

### Task 0: Fork + clone + 動作基線確認（~10分）

```bash
# GitHub で Esshahn/spritemate を fork → 自分のリポ
git clone git@github.com:YOUR/spritemate.git tia-studio
cd tia-studio && npm install && npm run dev
# C64 版スプライトエディタが http://localhost:5173 で動くことを確認
```

---

### Task 1: `config.ts` 寸法 8×16 化（コミット 1 / ~15分）

**最初に触る1ファイル: `src/js/config.ts`**

- L5: `sprite_x: 24` → `sprite_x: 8`
- L6: `sprite_y: 21` → `sprite_y: 16`
- `sprite_defaults` を TIA 初期値に

確認: Editor が 8×16 グリッドになる

---

### Task 2: `config.ts` Stella NTSC パレット差し替え（コミット 2 / ~30分）

**触るファイル: `src/js/config.ts`**（Task 1 の続き）

- `scripts/gen_stella_palette.js` を実行して 128色の hex 配列を生成
- `palettes` を `stella_ntsc` 128色に差し替え
- `selected_palette: "stella_ntsc"` に

確認: Palette ウィンドウに 128色 Stella NTSC パレットが表示される

---

### Task 3: `SpriteTypes.ts` colorPerRow 拡張（コミット 3 / ~20分）

**触るファイル: `src/js/SpriteTypes.ts`**

- `SpriteData.color:number` → `colorPerRow:number[]`
- `SpriteHelpers.createSprite` の初期化を配列化

確認: `npm run build` でコンパイルエラー一覧 → Task 4-6 の改修対象が確定する

---

### Task 4: `Sprite.ts` flip-Y / shift-Y colorPerRow 同期（コミット 4 / ~20分）

**触るファイル: `src/js/Sprite.ts`**

- `flip_vertical()` L109-139 / `shift_vertical()` L162-201 に colorPerRow 同期追加
- deepClone/copy-paste は追加不要

---

### Task 5: `Editor.ts` + `Window_Controls.ts` 塗り色 colorPerRow[y]（コミット 5 / ~30分）

**触るファイル: `src/js/Editor.ts` + `src/js/Window_Controls.ts`**

- `renderSpriteAtPosition()` L180-183 の色解決を `colorPerRow[j]` に変更
- `Window_Controls.render_pixels` にも同様の変更
- color strip UI（行ごと色帯 + クリックで色更新）を追加

確認: 各行が `colorPerRow[y]` の色で描画される。strip クリックで行色が変わる

---

### Task 6: `Export-Base.ts` `create_tia_assembly()` 追加（コミット 6 / ~30分）

**触るファイル: `src/js/Export-Base.ts`**

- `create_tia_assembly()` を新規追加（旧 `create_assembly` は残置）
- Export UI にフォーマット選択肢を追加

確認: Export → DASM フォーマットで `{name}_gfx` / `{name}_col` テーブルが出力される。手動 DASM で .bin が作れる

---

### Task 7: Stellerator embed + プレビュー確認（コミット 7 / ~45分）

**触るファイル: `src/js/TiaPreview.ts`（新規）/ `index.html`（canvas 追加）/ `App.ts`（1行追加）**

- worker ZIP 取得 → `public/stellerator.js`
- `TiaPreview.ts` 新規作成
- `<canvas id="tia-preview">` を `index.html` に追加
- `App.ts` に `TiaPreview` インスタンス化
- `<input type="file">` で .bin → `tiaPreview.loadRom(bytes)` の確認 UI

確認:
1. `create_tia_assembly()` で .asm 出力 → 手動 DASM で .bin → file input でアップロード
2. Stellerator canvas に GRP が描画される
3. 行ごと色が COLUP0 テーブルとして正しく反映される

**M1 完成** = 「単一スプライト GRP + 走査線毎 COLUP + Stella パレット描画 + DASM .byte/ラベル export + Stellerator プレビュー」が全部動く

---

### Task 8（任意拡張）: assemble API 配線（コミット 8 / ~60分）

- Vite `server.proxy` で `/api/assemble` → Go MCP harness エンドポイントに転送
- または Go 側に HTTP エンドポイント追加（`assemble_and_load` MCP ツールを HTTP ラップ）
- Task 7 の手動 .bin アップロードが自動化され、edit→preview ループが完結

M1 完成の定義外だが、連続 edit→preview ループには必要。

---

## まとめ表（全改修ファイル一覧）

| # | ファイル | 改修内容 | 規模感 |
|---|---|---|---|
| 1 | `config.ts` | sprite_x:8/y:16 / sprite_defaults / Stella 128色パレット | ~10行変更 + 128行値展開 |
| 2 | `SpriteTypes.ts` | `color` → `colorPerRow:number[]` / createSprite 初期化 | ~5行変更 |
| 3 | `Sprite.ts` | flip-Y / shift-Y の colorPerRow 同期 | ~8行追加 |
| 4 | `Editor.ts` | 塗り色 colorPerRow[y] + color strip UI | ~30行変更+新規 |
| 5 | `Window_Controls.ts` | 塗り色 colorPerRow[y] | ~5行変更 |
| 6 | `Export-Base.ts` | `create_tia_assembly()` 新規追加 | ~40行追加 |
| 7 | `TiaPreview.ts` | 新規ファイル・Stellerator embed | ~30行新規 |
| 8 | `index.html` | `<canvas id="tia-preview">` 追加 | ~2行 |
| 9 | `public/stellerator.js` | worker バイナリ配置 | コピーのみ |

ノータッチ: App.ts / Palette.ts / ImportPNG.ts / Import.ts / Load.ts / Save.ts / Animation.ts / List.ts / Tools.ts / Storage.ts / その他（計19ファイル）

---

## 出典 URL 一覧

| ファイル | URL |
|---|---|
| spritemate config.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/config.ts |
| spritemate SpriteTypes.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/SpriteTypes.ts |
| spritemate Sprite.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/Sprite.ts |
| spritemate Editor.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/Editor.ts |
| spritemate Preview.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/Preview.ts |
| spritemate Export-Base.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/Export-Base.ts |
| spritemate App.ts | https://github.com/Esshahn/spritemate/blob/main/src/js/App.ts |
| spritemate package.json | https://github.com/Esshahn/spritemate/blob/main/package.json |
| 6502.ts Stellerator.ts | https://github.com/6502ts/6502.ts/blob/master/src/web/embedded/stellerator/Stellerator.ts |
| 6502.ts VideoEndpointInterface.ts | https://github.com/6502ts/6502.ts/blob/master/src/web/driver/VideoEndpointInterface.ts |
| 6502.ts stellerator_embedded.md | https://github.com/6502ts/6502.ts/blob/master/doc/stellerator_embedded.md |
| 6502.ts release 1.1.2 worker | https://github.com/6502ts/6502.ts/releases/download/1.1.2/stellerator-embedded.zip |
| harness palette_stella.go | /Users/shinji/Documents/2D/260609_atari2600-dev/harness/internal/ingest/palette_stella.go |
