# research-w4-m1-open-questions — w3 の続き＝M1 ブロッカー2点（per-scanline color データモデル / ブラウザ framebuffer 読み）の決着

w3（`research-w3-buildable-apis.md`）で TIA Studio の buildable 架構（spritemate fork=編集 / 6502.ts
Stellerator embed=プレビュー / vcs-game-maker=codegen 配線）が確定。残った **M1 着手前に潰すべき2点**を
GitHub ソース実読で詰めた。収集日 2026-06-13。**[確認]=ソース/一次資料実読 / [推定]=未実装の方針**。出典 URL 付き。

w3 末尾 §4 の「未解決」がそのまま本書の Q1/Q2。両方とも **M1 の設計判断は確定した**（後述の1行結論）。

---

## Q1. per-scanline color のデータモデル（M1 の決定事項）

### Q1-a. spritemate の color 保持の現状 [確認]

ソース根拠（GitHub `Esshahn/spritemate` default branch `main`）:

- **`src/js/SpriteTypes.ts`** —
  https://github.com/Esshahn/spritemate/blob/main/src/js/SpriteTypes.ts [確認 raw 実読]
  - `interface SpriteData { name; color: number; multicolor; double_x; double_y; overlay; pixels: number[][]; animation? }`
    — **`color` は単一 `number`（pen1 の個別色・パレット index 0–15）**。スプライト1個に色1個。
  - `interface SpriteCollection { ... colors: { 0:number; 2:number; 3:number }; sprites: SpriteData[]; ... }`
    — `colors` は全スプライト共有の背景/mc1/mc2。**ここも行ごとではない**。
- **`src/js/Export-Base.ts`** —
  https://github.com/Esshahn/spritemate/blob/main/src/js/Export-Base.ts [確認 raw 実読]
  - `create_assembly()` は各スプライトの末尾に **color byte を1個**だけ吐く:
    `high_nibble = is_multicolor ? "1000" : "0000"` ＋ `low_nibble = sprites[j].color の 4bit` →
    `color_byte = "$" + hex` を `data` に追記。**＝1スプライト1色を1バイトで export**（C64 の sprite pointer+color 慣習）。
- 帰結: spritemate は **「スプライト＝固定1色」モデル**。TIA Studio M1 の「走査線ごとに COLUP 色」を表現できない。
  → **データモデル拡張が必須**（w3 §4 の指摘どおり）。

### Q1-b. 前例（per-row / per-scanline color を持つ実在ツール・技法）[確認]

| 前例 | 色の持ち方 | データ表現 | 出典 |
|---|---|---|---|
| **PlayerPal 2600**（kisrael・**already-owned**） | **行ごとに色を持つ**（"change scanline colors with the toggle on the side"） | スプライトの**各ピクセル行末に color を asm コメント**として吐き、import 時にそのコメントから色を復元（無損失往復） | https://alienbill.com/2600/playerpalnext.html [確認 WebFetch] |
| **Andrew Davie S11/S15 + 2k6specs（kernel 技法）** | COLUP0/COLUPF を**走査線ごとに書換え** | **走査線数ぶんの color テーブル**を `LDA colortable,Y / STA COLUP0` で indexed load（PF テーブルと同型） | https://problemkaputt.de/2k6specs.htm [確認]／"These tables contain the values which should be written ... for each scanline" https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-15.html [確認] |
| **batari Basic `playfield:`/多色カーネル** | playfield は1行=PF0/PF1/PF2 の行配列、色は `pfcolors:`/`COLUPF` を行ごと | **「行→値」の配列**（vcs-game-maker の `matrixToPlayfield` が `X/.` テキスト⇔行列に変換、w3 §3 で確認済） | w3 §3（`vcs-game-maker/src/utils/pixels.js`） |
| masswerk Tiny VCS Sprite editor | **色を持たない**（パターンのみ。"add color codes on your own"） | — | https://www.masswerk.at/vcs-tools/ [確認] |

**定石の結論**: 2600 の vertical multicolor は普遍的に **「走査線（行）→ COLUP 値」の1次元配列＝color テーブル**で表す。
カーネルは PF テーブルと完全に同じ `LDA table,Y / STA COLUPx` パターンで消費する。**ランレングス/ゾーン境界は不要**
（76cy 制約下では単純な行→色配列が最速・最も慣習的。圧縮は export 最適化の話で、編集モデルには持ち込まない）。
PlayerPal が「行末 color コメントで無損失往復」を既にやっており、これは tia-studio-spec M6 の「color コメント無損失往復」の直接の前例。

### Q1-c. 推奨データモデル（具体スキーマ）[推定＝M1 の決定]

spritemate の `SpriteData` を最小拡張する。**`pixels[y][x]` はそのまま流用、`color:number` を行配列に格上げ**:

```ts
interface SpriteData {
  name: string;
  // --- 拡張点: 1スプライト1色 → 走査線ごとの色 ---
  colorPerRow: number[];   // length === pixels.length（高さぶん）。各値=COLUPx に書く TIA レジスタ値
                           // 旧 color:number は廃止（後方互換が要れば colorPerRow = Array(h).fill(color) で移行）
  pixels: number[][];      // [y][x]。0=透明 / 1=描画（M1 は GRP=1bit/px なので 0|1 の2値で十分）
  multicolor: boolean;     // M1 では false 固定（TIA の P0/P1 は1bit/px。mc は将来 PF/score 用）
  double_x: boolean; double_y: boolean; overlay: boolean;
  name; animation?: AnimationData;
}
```

- **色の値域**: spritemate の「パレット index 0–15」ではなく、**TIA レジスタ値（hue 上位ニブル×lum 下位ニブル）**で持つ
  （design-principles.md「色は RGB でなく レジスタ値／象徴名で持つ」）。描画時に `palette_stella.go` 由来の実測 RGB へ変換。
  → config の `palettes` を Stella 実測 16階調群×hue に差し替える（w3 §1 で確認済の差し替え点）。
- **なぜ配列1本か**: ゾーン境界やランレングスは「同色が続く区間を圧縮」する最適化だが、編集 UI は「この行の色は何か」を
  O(1) で読み書きできる素の配列が一番素直で、undo（spritemate の deepClone backup）とも相性が良い。圧縮は export 側で任意。

### Q1-d. 影響範囲チェックリスト（`color:number` → `colorPerRow:number[]`）[確認＝ソース所在]

w3 §1 の分離度（高い）を踏まえ、触る箇所は局所:

- [ ] **SpriteTypes.ts**: `color:number` を `colorPerRow:number[]` に。`SpriteHelpers.createSprite` のデフォルト生成を
      `colorPerRow = Array(sprite_y).fill(default)` に（`createPixelGrid` の隣）。
- [ ] **Sprite.ts**（`class Sprite`・データ操作の本体）: new/clear/fill/flip(**double_y/flip-Y 時は colorPerRow も反転**)/
      shift(**縦 shift 時は色配列も同期シフト**)/copy-paste/undo backup の deepClone が `colorPerRow` を含むか確認。
      `this.all.colors`（共有色）はそのまま。https://github.com/Esshahn/spritemate/blob/main/src/js/Sprite.ts
- [ ] **Editor.ts**（canvas 描画・config 駆動）: ピクセル矩形の塗り色を「pen 色1個」から `colorPerRow[y]` 参照に。
      **新規 UI＝行ごとの color ストリップ**（PlayerPal の「side toggle」相当）を grid の左右どちらかに足す。
      https://github.com/Esshahn/spritemate/blob/main/src/js/Editor.ts
- [ ] **Preview**（spritemate の Preview）: 同じく行ごと色で描く（M1 のブラウザ即時プレビューはここ＋後述 Stellerator）。
- [ ] **Export-Base.ts `create_assembly`**: w3 §1 の方針どおり multicolor 分岐を捨て **GRP=1byte/scanline・MSB-first**。
      **加えて color テーブルを別ラベルで吐く**: `<name>_gfx: .byte %xxxxxxxx`（行ぶん）＋
      `<name>_col: .byte $xx`（`colorPerRow` をそのまま行ぶん）。DASM `-f3` 用に行頭 `.byte`。
      https://github.com/Esshahn/spritemate/blob/main/src/js/Export-Base.ts
- [ ] **config.ts**: `sprite_x:8`（TIA）化＋`palettes` を TIA レジスタ値×Stella 実測 RGB に。
      https://github.com/Esshahn/spritemate/blob/main/src/js/config.ts
- [ ] **無改造で再利用**: undo/redo（backup deepClone は構造を辿るので配列化に追従）/ floodfill / PNG import（`ImportPNG.ts`）/
      App 仲介。← w3 §1 結論どおり。

---

## Q2. ブラウザ内 framebuffer 読み出しの可否（照合の二段構成の確定）

### Q2-a. 6502.ts のレンダリング経路 [確認＝raw 実読]

GitHub `6502ts/6502.ts` default branch `master`:

1. **`src/web/driver/Video.ts`**（WebGL ドライバ）—
   https://github.com/6502ts/6502.ts/blob/master/src/web/driver/Video.ts [確認 raw 実読]
   - context 生成: `getContext('webgl', { alpha:false, depth:false, antialias:false })`
     **— `preserveDrawingBuffer` 未設定**（w3 §2-D の指摘どおり確定）。`readPixels` 呼び出しは**コード中に一切無い**。
   - フレーム供給: **`frame.get()` が `ImageData` を返し**、`gl.texImage2D(..., gl.RGBA, gl.UNSIGNED_BYTE, frame.get())` で
     テクスチャ化 → NTSC/phosphor/scanline/integer-scaling の processor チェーン → `drawArrays`。
     processor 群は `src/web/driver/video/`（`NtscProcessor/PhosphorProcessor/ScanlineProcessor/IntegerScalingProcessor/Program/shader/Capabilities`）。
2. **★決定的発見 — フレームは WebGL に入る前に既に標準 `ImageData`**:
   **`src/web/driver/VideoEndpointInterface.ts`** —
   https://github.com/6502ts/6502.ts/blob/master/src/web/driver/VideoEndpointInterface.ts [確認 raw 実読]
   ```ts
   getWidth(): number;
   getHeight(): number;
   newFrame: EventInterface<PoolMemberInterface<ImageData>>;
   ```
   — **`newFrame` イベントが `ImageData`（RGBA そのもの）を流す**。`PoolMemberInterface.get()` が `ImageData` を返す
   （https://github.com/6502ts/6502.ts/blob/master/src/tools/pool/PoolMemberInterface.ts [確認]：`get():T` / adopt / release / dispose）。
   → **ROM→1フレームの RGBA は WebGL `readPixels` を経由せず、JS 側に ImageData として既に存在する**。
3. **embed 層での到達性** — `src/web/embedded/stellerator/Stellerator.ts` [確認 raw 実読]:
   `_createVideoDriver()` 内で `this._driverManager.addDriver(this._videoDriver, ctx => this._videoDriver.bind(ctx.getVideo()))`。
   **`ctx.getVideo()` が `VideoEndpointInterface` を返す**が、`Stellerator` はこれを `private _videoDriver` に隠し、
   **公開 getter（getVideo/getVideoEndpoint 等）を一切持たない**。公開イベントは `frequencyUpdate/stateChange/asyncIOMessage` のみ。
   https://github.com/6502ts/6502.ts/blob/master/src/web/embedded/stellerator/Stellerator.ts

### Q2-b. framebuffer 取得の最小改造 [推定＝MIT で可]

w3 §2 は WebGL readback（preserveDrawingBuffer 化＋readPixels）を選択肢に挙げたが、**Q2-a の発見でそれは不要**。
RGBA は ImageData として既に存在するので、最小改造は **WebGL を一切触らず VideoEndpoint をフックするだけ**:

- **最小手順（推奨ルート）**: `Stellerator.ts` の `_createVideoDriver()` のクロージャ内で、`bind` と同じ `context.getVideo()` を
  掴み、その `newFrame` を購読して最新 `ImageData` を保持＋公開する。追加はおおよそ次の数行レベル:
  ```ts
  // _createVideoDriver の addDriver クロージャ内（bind の隣に追記）
  const video = context.getVideo();
  video.newFrame.addHandler(pm => { this._lastFrame = pm.get(); });   // ImageData を保持
  // クラスに公開 API を1つ追加:
  getLastFrame(): ImageData | undefined { return this._lastFrame; }    // ← 数行
  ```
  これで `getLastFrame()` が **RGBA（ImageData）を返す**。`getImageData`/`readPixels`/`preserveDrawingBuffer` は不要。
  改造規模＝**1ファイル（Stellerator.ts）に保持フィールド＋購読1行＋getter 1メソッド ≒ 5–8行**。MIT なので fork 可。
  ※ pool 管理されたフレームなので「保持中はコピーする（`new ImageData(new Uint8ClampedArray(pm.get().data), w, h)`）」のが安全
  （pool が再利用する前にスナップショットを取る）。
- 代替（より侵襲小だが API 設計次第）: `Stellerator` に `frameAvailable: Event<ImageData>` を生やして都度流す。
- **WebGL readback ルートは非推奨**: preserveDrawingBuffer 化＋readPixels は requestAnimationFrame 後に空になりがちで
  不安定（w3 §2-D）。ImageData フックがあるので採る理由がない。

### Q2-c. 二段構成は M1 に十分か（結論）

- **M1 のブラウザプレビューは「見せるだけ」で十分** [確認で裏付け]: M1 のゴールは「単一スプライト GRP＋per-row 色を編集→
  codegen→ROM→Stellerator.run() で canvas に絵が出る」確認。`run(romBytes, ntsc)` で出すだけ＝**無改造で達成**（w3 §2/§5）。
  ピクセル**照合の正本はサーバ Gopher2600**（harness）。tia-studio-spec の二段構成（ブラウザ即時＋権威サーバ）が正しい。
- **ブラウザ照合が要るユースケースは M1 には無い**。将来 M5（実機裏打ち）で「ブラウザ内で Stellerator 出力 vs キャンバスを
  即時 diff」をやりたくなった時に限り、Q2-b の **ImageData フック（5–8行）** を入れれば足りる。
  → **M1 では framebuffer 取得改造は不要。やるとしても M5 で軽量に追加可能**と判明（w3 §2 が懸念した「WebGL readback の不安定さ」は
  そもそも回避できる＝リスク消滅）。

---

## M1 着手時に「最初に書くコード」順序（本書＋w3 §5 を踏まえた確定版）

1. **spritemate を fork**し、`src/js/SpriteTypes.ts` の `SpriteData.color:number` → **`colorPerRow:number[]`**（Q1-c）。
   `SpriteHelpers.createSprite` のデフォルト生成も配列化。
2. **`src/js/config.ts`** で `sprite_x:8`（TIA）化＋`palettes` を **TIA レジスタ値×Stella 実測 RGB**（`palette_stella.go` 由来）に。
3. **`src/js/Editor.ts`**: ピクセル塗り色を `colorPerRow[y]` 参照に変更＋**行ごと color ストリップ UI**（PlayerPal の side toggle 相当）を追加。
4. **`src/js/Sprite.ts`**: flip-Y / 縦 shift / undo backup が `colorPerRow` を同期するよう修正（Q1-d チェックリスト）。
5. **`src/js/Export-Base.ts` `create_assembly`**: multicolor 分岐を捨て **GRP=1byte/scanline・MSB-first** ＋
   **color テーブルを別ラベル** `<name>_gfx`/`<name>_col` で DASM `.byte` 出力（Q1-d）。
6. **6502.ts Stellerator を embed**: `new Stellerator(canvas, worker).run(codegenRomBytes, ntsc)` で **無改造プレビュー**（Q2-c）。
   worker `stellerator.js` を同一ドメインに同梱（w3 §4 の運用メモ）。
7. **照合は触らない**（M1 ではサーバ Gopher2600 が正本）。ブラウザ ImageData フック（Q2-b・5–8行）は **M5 まで保留**。

---

### 1行結論（M1 の設計判断）
- **Q1（color データモデル）＝確定**: `SpriteData.color:number` を **`colorPerRow:number[]`（走査線→TIA色値の1次元配列）** に拡張する。
  前例（PlayerPal の per-row color＋行末コメント往復、Davie/2k6specs の color テーブル indexed load）が定石を裏付け。影響範囲は spritemate 5ファイルに局所。
- **Q2（ブラウザ framebuffer）＝確定**: 6502.ts はフレームを **WebGL に入る前に `ImageData` として持つ**（`VideoEndpointInterface.newFrame: ...ImageData`）ので、
  必要なら `readPixels`/`preserveDrawingBuffer` 不要・**Stellerator.ts に 5–8行のフック**で RGBA を取れる。**ただし M1 では取得不要＝サーバ Gopher2600 が照合の正本**で十分（二段構成が正しいと確定、w3 のリスクは消滅）。
