# research-w3-buildable-apis — TIA Studio buildable 架構の「実装API地図」(W3)

目的: TIA Studio の M1 試作の技術リスクを潰すため、確定 3 OSS（spritemate / 6502.ts-Stellerator /
vcs-game-maker）の **データモデル・公開API・拡張ポイント**をソース実読で文書化し「どう繋ぐか」を確定する。
収集日 2026-06-13。全項目に GitHub 出典（パス・行・確認手段）を付す。**[確認]=ソース実読 / [推定]=未実装の方針**。

## 既存 w1/w2 との差分（このノートの担当範囲）
- **w1-tooling** = どの OSS を採るか（ライセンス・候補棚卸し）。**w2-design** = 2600 作画の設計原則・寸法・色。
- **w3（本書）= 実装API特化**: 採用済み 3 OSS を GitHub で実読し、`SpriteData` の形・`create_assembly` の
  fork 点・`Stellerator.run()` の I/O・`bbasic.js` の codegen 配線まで「接続できるレベル」で記述。
- w1 の記述で**更新が必要だった点**（本書で訂正）:
  - spritemate は w1 で「modular **vanilla-JS**」とあるが、現行 main は **TypeScript + Vite** に移行済み
    （`src/js/*.ts`, `tsconfig.json`, `vite.config.js`）。型定義 `SpriteTypes.ts` あり=fork が**より容易**。
  - 6502.ts の Stellerator 出力 canvas は **WebGL（2D ではない）** かつ `preserveDrawingBuffer` 未設定
    = ブラウザ側 `getImageData()` 直読は**そのままでは不可**（w1/spec の「getImageData で画素読み」は要訂正。後述§2-D）。

---

## 1. spritemate — 編集レイヤ（fork 対象）

- リポ: https://github.com/Esshahn/spritemate （live: spritemate.com）
- ライセンス: **MIT** [確認] `LICENSE` = "MIT License / Copyright (c) 2017 Ingo Hinterding"
- 最終更新: **2026-05-22**（活発）。stars 150。default branch `main`。
- スタック: **TypeScript + Vite**。`src/js/*.ts`（28 ファイル）。中央 `App` クラス + `window.app` グローバルで各モジュールを仲介。

### データモデル [確認] `src/js/SpriteTypes.ts`
```ts
interface SpriteData {
  name: string;
  color: number;          // pen 1 の個別色（パレット index 0-15）
  multicolor: boolean;
  double_x: boolean; double_y: boolean; overlay: boolean;
  pixels: number[][];     // ★2D配列 [y][x]、値 0=透明 1=個別色 2=mc1 3=mc2
  animation?: AnimationData;
}
interface SpriteCollection {
  version: string; filename: string;
  colors: { 0:number; 2:number; 3:number };  // 0=背景/透明, 2=mc1, 3=mc2（全sprite共有）
  sprites: SpriteData[];
  current_sprite: number; pen: number;        // pen = 0|1|2|3
  animation: {...};
}
```
- 実体は `src/js/Sprite.ts`（`class Sprite`、`this.all` が `SpriteCollection`）。これが**純データモデル+操作**
  （new/clear/fill/flip/shift/floodfill/invert/undo-redo backup/copy-paste）。undo は `backup[]` に deepClone を積む方式 [確認]。
- `Sprite.ts` には既に `this.all.playfield = { backgroundColor, sprites:[] }` が存在 [確認] = レイアウト概念の素地あり（C64 文脈だが構造は流用可）。
- 寸法は config 駆動: `src/js/config.ts` `sprite_x:24, sprite_y:21`（C64）。**TIA は `sprite_x:8` 系に差し替え**れば
  Editor/Preview/Export が全部追従する（Editor は `config.sprite_x * grid_width` で canvas を組む [確認] `Editor.ts`）。
- パレットも config: `palettes.{colodore,pepto,...}.values[16]`（C64 hex）。**ここを Stella 実測16階調群に差し替え**れば描画色が我々のパイプラインと一致。

### import/export（fork して TIA 出力に差し替える場所）[確認] `src/js/Export-Base.ts`
- **`create_assembly(format, encode_as_binary)`** が出力の心臓。`sprites[j].pixels` を `[].concat.apply` で flatten →
  8ビットずつ走査 → multicolor は 2bit ペア（00/10/01/11）、singlecolor は 1bit（0/1）で `byte` 文字列を組み立て →
  `$hex` または `%binary` で `!byte`（ACME）/`.byte`（KickAss）行に出力。各 sprite 末尾に color byte を付す。
- **TIA への fork 方針 [推定]**: このビット詰めループを差し替える。
  - GRP: multicolor 分岐を捨て、各行 8px を **MSB-first 1byte/scanline** に（w2 の GRP 正典どおり）。sprite 高さ=行数ぶん `.byte %xxxxxxxx`。
  - 色: per-scanline COLUP は spritemate の単一 `color` では持てない → **データモデル拡張が必要**（後述 M1 接続）。
  - PF 出力: `pixels` を 40 列モデルにし PF0/PF1/PF2 のビット順（w2 正典）で別ルーチン化。
  - DASM(`-f3`)向けに行頭は `.byte`、ラベルは `name`（既存の `label_suffix` 機構を DASM 用に足すだけ）。
- 既存 export 形式: ACME/KickAss asm（`create_assembly`）, C64 BASIC（`create_basic`）, spritesheet PNG（`Export-Spritesheet.ts`）。
- **import**: `Import.ts`（.spm JSON / C64 .spd, .prg）, `Load.ts`（プロジェクト JSON）, **`ImportPNG.ts`**（PNG→sprite、`<input accept=".png">`→量子化）[確認]。
  PNG import 経路あり = 我々の cmd/ingest（Photoshop モック取込）と思想が一致。

### UI とデータの分離度（再利用しやすさ）[確認]
- **高い**。`Sprite`（データ）/ `Editor`（canvas 描画、config 駆動）/ `App`（仲介・イベント）/ `Export-*`（出力）/ `Palette`/`Preview` が
  モジュール分離。`App.ts` が全 window コンポーネントを束ね `window.app` で参照させる素朴 MVC。
  Editor は `pixels[y][x]` を読んで矩形を塗るだけ（描画に C64 固有ロジックなし）。
- 結論: **spritemate fork は現実的**。差し替え対象は config(寸法/パレット) と Export(ビット詰め)、追加が per-scanline 色モデルのみ。
  UI/undo/floodfill/flip/PNG import はそのまま再利用できる。

---

## 2. 6502.ts / Stellerator embedded — ライブプレビュー（embed 対象）

- リポ: https://github.com/6502ts/6502.ts （typedoc: 6502ts.github.io/typedoc/stellerator-embedded/）
- ライセンス: **MIT** [確認] 各 `.ts` 冒頭ヘッダ "Copyright (c) 2014--2020 Christian Speckner ... MIT"。
- 最終更新: **2025-01-26**（spritemate/vcs-gm より古いが安定。2600 コア=枯れている）。stars 64。default branch `master`。
- 公式 doc: `doc/stellerator_embedded.md` [確認]。NPM: `6502.ts`、frontend のみ（worker は別途ホスト要）。

### embed 最小手順 [確認] `doc/stellerator_embedded.md` + `src/web/embedded/stellerator/Stellerator.ts`
```js
// prebuilt: グローバル $6502 / npmは import Stellerator from '6502.ts/lib/web/embedded/stellerator/Stellerator'
const st = new Stellerator(canvasElement, 'js/stellerator.js' /* worker URL */, {
  tvEmulation: Stellerator.TvEmulation.none,   // composite/svideo/none
  scalingMode: Stellerator.ScalingMode.qis, phosphorLevel:0, scanlineLevel:0, gamma:1
});
await st.run(romBytes, Stellerator.TvMode.ntsc);  // ROM=配列/TypedArray/base64文字列
```
- **構造**: frontend（`Stellerator` クラス）+ **emulation core が web worker 別スレッド**で走る2部構成 [確認]。
  worker スクリプト(`stellerator.js`)は同一ドメインにホストが必須。

### 公開API（runtime 制御）[確認] `Stellerator.ts`（メソッドは全て async/Promise<State>）
- ライフサイクル: `start(rom,tvMode,cfg)`（paused で起動）/ `run(...)`（即実行）/ `pause()` / `resume()` / `stop()` / `reset()`。
- 取得: `getState()`→`State.{running,paused,stopped,error}` / `lastError()` / `getControlPanel()`（select/reset/difficulty/color スイッチ）。
- 映像設定: `setGamma/setScalingMode/setTvEmulation/setPhosphorLevel/setScanlineLevel/resize/toggleFullscreen`。
- 音: `setVolume/getVolume/audioEnabled`。canvas 差替: `setCanvas/releaseCanvas`。
- **イベント（microevent）**: `frequencyUpdate: Event<number>` / `stateChange: Event<State>` / `asyncIOMessage`。
  TvMode/ScalingMode/TvEmulation/State/ControllerType は `Stellerator.*` namespace の enum [確認]。

### M5「ライブプレビュー」の I/O = ROM bytes in → framebuffer
- **ROM bytes in**: `run(romBytes, ntsc)` に TypedArray を渡すだけ。我々の codegen → DASM → .bin の bytes を直接投入できる [確認]。
- **frame out（重要リスク）[確認]**:
  - 映像は private `VideoDriver`(`src/web/driver/Video.ts`)が **WebGL** で canvas に描く。`getContext('webgl', {alpha:false,depth:false,antialias:false})`
    = **`preserveDrawingBuffer` 未設定** [確認 L185-195]。frame は worker から RGBA bytes(`frame.get()`)が来て `texImage2D` でテクスチャ化→GPU 描画 [確認 L238]。
  - **公開された「per-frame ピクセル callback / framebuffer 取得 API は無い」** [確認]（events は frequency/stateChange/asyncIO のみ）。
  - 帰結: **ブラウザ側で `canvas.getContext('2d').getImageData()` は使えない**（WebGL canvas）。`gl.readPixels()` も preserveDrawingBuffer 無しでは
    requestAnimationFrame 後に空になりがちで不安定。**=「ブラウザで実機ピクセル照合」は Stellerator 素のままでは確実に取れない**。

### M1/M3 への接続方針
- **M1（単一スプライト試作）**: ライブプレビューは「絵が動く」確認用途。`run()` で codegen ROM を流し canvas に出すだけで十分=**低リスク [確認]**。
- **M3/M5（合成→実機裏打ち）**: **ピクセル差分の正本は我々の Gopher2600（サーバ側）に置く**＝spec の二段構成は正しい [確認で裏付け]。
  ブラウザ Stellerator は「即時の見た目プレビュー」専任に限定。どうしてもブラウザ側読みが要るなら選択肢は:
  (a) fork して VideoDriver の WebGL を `preserveDrawingBuffer:true` 化＋`gl.readPixels` を生やす（MIT なので可・小改造）、
  (b) worker の `VideoEndpointInterface`(`frame.get()` の RGBA)を `asyncIO`/独自 event で外に出す改造、
  (c) ブラウザ照合を諦めサーバ Gopher2600 一本（**推奨・追加実装ゼロ**）。

---

## 3. vcs-game-maker — 配線（codegen）の参考

- リポ: https://github.com/haroldo-ok/vcs-game-maker （itch: haroldo-ok.itch.io/vcs-game-maker）
- ライセンス: **MIT** [確認] repo license=MIT。codegen 各ファイルは Apache-2.0 ヘッダ（Blockly 由来の派生で混在）。
- 最終更新: **2026-01-21**（活発）。stars 60。default branch `master`。スタック: **Vue2 + Vuetify + Blockly**。

### codegen 構造（中間表現 → カーネル）[確認] `src/generators/bbasic.js` + `bbasic.bb.hbs`
パイプラインは**「2系統の中間表現」を Handlebars マスターテンプレに流し込む**形:
1. **ロジック中間表現 = Blockly workspace**。`src/blocks/*`（block 定義）→ `src/generators/bbasic/*`（各 block の bB 生成器、
   sprites/background/color/collision/input/sound/score/logic/loops/math …）。`Blockly.BBasic` が Generator。
2. **グラフィック中間表現 = プロジェクト storage**（Vue ref）。`hooks/project.js` の `useBackgroundsStorage/usePlayer0Storage/usePlayer1Storage`。
   pixel は **行列**（`number[][]` 相当、`utils/pixels.js` の `playfieldToMatrix/matrixToPlayfield` で `X/.` テキスト⇔行列）。
3. **合成 = `Blockly.BBasic.finish(code)`** [確認]: `handlebarsTemplate({...})` に名前付きスロットを埋める。
   - `generateBackgrounds()` → 各 bg を `playfield:` ブロック（`matrixToPlayfield`）＋ `if newbackground<>id then goto` で切替。
   - `generateAnimations()` → player の各フレームを `player0:`＋`%xxxxxxxx`（行を reverse して下→上）＋ frame カウンタの
     ステートマシン（`if player0frame > limit then goto ...`）として吐く [確認]。
   - イベントスロット: `system_start/title_start/title_update/gameplay_start/gameover_*` を `@label` で差し込む。
- **マスターテンプレ** `bbasic.bb.hbs` [確認]: `set tv ntsc` / `set romsize 4k` + 固定の init（COLUBK, player0x/y, NUSIZ…）+
  `{{{ generatedBody }}}` 等のスロット。**= 手書きカーネル骨格 + 穴埋めスロット**という構造そのもの。
- **コンパイル/プレビュー配線** [確認]: deps `batari-basic`（bB コンパイラの JS 版）で bB→ROM bytes、プレビューは **Javatari**
  （`src/App.vue` L187/224 `javatari-target-container`, `Javatari.compiledResult.output` を Blob 化して `compiled-rom.bin` DL）。
  ※我々の spec は **この Javatari(AGPL) を Stellerator(MIT) に差し替える**方針＝ここが最大の流用上の相違点。

### M3 への接続方針（ヘテロ全要素を1画面合成→手書きカーネル生成）
- **流用できる配線パターン [確認→推定]**:
  1. **「固定カーネル骨格テンプレ + 名前付きスロット穴埋め」**（Handlebars `bbasic.bb.hbs`）= 我々の M4「テンプレ=検証済みカーネル技」の
     実装形にそのまま使える発想。我々は bB でなく **DASM カーネル骨格 .asm.hbs（または Go text/template）** にスロットを切る。
  2. **「グラフィック storage（pixel 行列）」と「ロジック/配置」を別中間表現に分け、finish() で1本化** = 我々の
     シーン JSON（要素グラフィック + 位置 + per-scanline 色）→ codegen と同型。spritemate の `SpriteData.pixels` を pixel 行列として使える。
  3. **アニメ=フレーム配列＋frame カウンタのステートマシン生成** = 我々の sprite_anim 技の codegen 雛形になる。
- **我々が変える点**: ①出力先=bB ではなく **手書き 6502/DASM カーネル**（vcs-gm は bB 標準カーネルに限定＝多スプライト合成が弱い、w1 既述）。
  ②プレビュー= Javatari→Stellerator。③検証= 我々の Gopher2600/harness（vcs-gm に無い実機裏打ち）。
- 結論: vcs-gm は **「中間表現2系統 → テンプレ穴埋め → bytes → preview」の配線教科書**として価値大。コードは MIT だが **Vue/Blockly/bB 前提**なので
  コピーではなく**配線構造を移植**する（codegen の出力ターゲットを DASM に替える）。

---

## 4. 未解決 / 要実機確認

- **[要実機/要試作] Stellerator のブラウザ側ピクセル読み**: WebGL `preserveDrawingBuffer` 未設定のため `getImageData/readPixels` が
  安定して取れるか未検証。**M5 ブラウザ照合を諦めれば不要**（サーバ Gopher2600 が正本）。やる場合は fork 小改造（§2 (a)/(b)）の実機検証が要る。
- **[要設計] per-scanline COLUP のデータモデル**: spritemate `SpriteData` は sprite あたり単一 `color`。w2 の「縦に色を足す（走査線毎 COLUPx）」を
  表すには `color` を `colorPerRow: number[]` に拡張する必要（M1 のデータモデル決定事項）。
- **[要確認] batari-basic(js) の素性**: vcs-gm は npm `batari-basic@0.0.1` 依存。我々は bB を使わない方針なので影響薄だが、配線を真似る際の参照に留める。
- **[要確認] Stellerator worker のバンドル**: prebuilt の `stellerator.js`(worker) を harness/tools に同梱しホストする運用（CORS/同一ドメイン制約）。M1 で実機確認。
- **[未検証] Stella 実測パレットと Stellerator 表示色の一致**: Stellerator の TvEmulation/gamma を切れば素の TIA 色に近づくが、我々の `palette_stella.go` と
  完全一致するかは未検証（w2 の「出典で RGB が十数〜32 差」と同根）。**色照合の正本はサーバ側**で担保。

## 5. M1 着手時に最初に読むべきソース（優先順位）
1. **spritemate `src/js/SpriteTypes.ts` + `Sprite.ts`** — データモデルを TIA 用に確定（pixels[y][x] 流用 + per-scanline color 拡張）。
2. **spritemate `src/js/Export-Base.ts` `create_assembly`** — fork して GRP(8bit/MSB-first/1byte/scanline) + DASM `.byte` 出力に。
3. **spritemate `src/js/config.ts` + `Editor.ts`** — `sprite_x/y` と `palettes` を TIA 寸法・Stella 実測色に差し替え（描画一致）。
4. **6502.ts `doc/stellerator_embedded.md` + `Stellerator.ts`(run/start/pause/state)** — `run(rom,ntsc)` で codegen ROM を canvas に出す最小プレビュー。
5. **vcs-game-maker `src/generators/bbasic.js`(finish) + `bbasic.bb.hbs`** — M4 テンプレ穴埋め配線の参照（出力を DASM に置換する設計）。
6. （M5 で必要時のみ）**6502.ts `src/web/driver/Video.ts`** — ブラウザ側読みを足す場合の改造点。

---

### 最重要発見（M1 リスクをどれだけ潰せたか）
1. **spritemate fork は現実的**: 純データモデル `SpriteData.pixels:number[][]` と config 駆動の寸法/パレット、出力1点 `create_assembly` という
   理想的な分離。fork 改修は「config 差し替え＋Export のビット詰めを GRP/PF に＋per-scanline color 拡張」に局所化でき、UI/undo/PNG import は無改造で再利用可。
2. **ライブプレビューは低リスク・ピクセル照合は要注意**: `Stellerator.run(romBytes, ntsc)` で codegen ROM を canvas に出すのは確実（M1 OK）。
   ただし **canvas は WebGL で preserveDrawingBuffer 未設定 → ブラウザ側ピクセル読みは素のままでは不可**。spec の「getImageData で読む」は要訂正、
   **照合の正本はサーバ Gopher2600**（spec の二段構成が正解と裏付け）。
3. **codegen 配線図が手に入った**: vcs-gm の「pixel 行列 storage ＋ Blockly ロジック → Handlebars マスターテンプレの名前付きスロット穴埋め → bytes → preview」が
   そのまま M3/M4 の設計図。我々は出力ターゲットを bB → 手書き DASM カーネルに替えるだけで同型の配線を組める。
