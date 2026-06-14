# tia-studio — 参考にする既存ツール（canvas プロジェクト用・2026-06-13 ユーザー提供）

自前の「2600 スプライト＋PF デザインスタジオ」（harness/tools/tia-studio.html・予定）を作る前に
**徹底調査する既存ツール**。良いとこ取りして、我々のパイプライン直結＋私がコードで駆動できる版を作る。
方針＝[[../docs/techniques]] の作画版。
**差別化（正直版・2026-06-13 訂正）**：既存の良いツール（PlayerPal/masswerk/background-builder）は
ビットレベルで**正確に描画している**（「近似」は言い過ぎだった）。自前を作る理由は"忠実度"ではなく：
- (a) **export が roms/techniques と cmd/ingest にそのまま乗る**（パイプライン直結）。
- (b) **JSON モデル＋関数を私がコードで駆動できる**（設計生成→canvas→スクショ→判断→反復。既存はマウス専用）。
- 補足: 描画は我々も Stella 実測パレット（`internal/ingest/palette_stella.go`）＋PFビット順(CLAUDE.md)を使い、
  **我々のパイプラインと色が一致**するようにする（既存ツールのパレットが一致するかは未検証＝もし違えば色がズレうる、という
  "一貫性"の話。CRT の非正方ピクセル/にじみはどのツールも我々も再現しない＝そこに優位は無い）。

## スプライト系
- **PlayerPal Next (v2.2a)** — https://alienbill.com/2600/playerpalnext.html （Kirk Israel・JS・**定番**）
  機能: ピクセル描画／**多フレームアニメ（cut/paste）**／**走査線毎カラー**／幅 1x/2x/4x／1-4ライン kernel／NTSC・PAL／
  collapsible パネル・サムネイル・flip/rotate。**export = ASM（色コメント埋込で lossless 往復）＋ batariBASIC**。
  ★採用: **色コメント埋込で export↔import 無損失**・アニメフレーム列・走査線毎カラー・幅プレビュー。
- **PlayerPal classic** — （上の前身。playerpal.html）
- **spred (SprEd)** — https://bocianu.gitlab.io/spred/ （※**Atari 8-bit 向け**・2600 ではない）
  参考になる UX: **キーボード駆動（0-6 色・space で再生）**・フレームアニメ＋遅延・undo/redo・
  **export 形式を HEX/DEC/BIN＋bytes-per-line＋ラベル接頭辞で選べる**・GIF 出力。2600 用に色制約を単純化して流用。

## プレイフィールド系
- **alienbill background-builder** — https://alienbill.com/2600/atari-background-builder/ （Kirk Israel・大PF）
  機能: 非対称/mirror/repeat（正しいビット反転）・**48px スプラッシュ minikernel**・**画像インポート(PNG/JPG→2600・縮小最適化)**・
  走査線毎 FG/BG 色（NTSC/PAL＋スポイト）・描画ツール(pen/line/rect/ellipse/fill/invert)・kernel 高 1-20・**project save/load**。
  ★採用: 画像インポート（我々は cmd/ingest と接続）・描画ツール一式・save/load・48px タイトル。
- **PlayfieldPal** — https://alienbill.com/2600/playfieldpal.html （PlayerPal の PF 版）
- **masswerk Tiny Playfield Editor** — https://www.masswerk.at/vcs-tools/TinyPlayfieldEditor/
  対称/非対称＋repeat/mirror。**export を「行順」か「PF0/1/2 ラベル配列」で選べる**柔軟性。★採用: この二択 export。
- **masswerk Tiny Sprite Editor** — https://www.masswerk.at/vcs-tools/TinySpriteEditor/
  最小プレイヤー編集・asm 入出力。★採用: ミニマル UI の潔さ。
- **randomterrain batari BASIC Playfield Editor** — https://www.randomterrain.com/bb-playfield-editor.html
  32列キャンバス・行毎に FG/BG 塗り（NTSC チャート）・kernel 種別（DPC+ 176/88/44/22/11・標準 11・multisprite）・
  **multisprite は中央 mirror**・"no_blank_lines" で前行色がにじむ。**export = batariBASIC（playfield: の X/. ＋ pfcolors/bkcolors 配列）**。
  ★採用: 行毎カラーの UX・kernel 高の選択肢・X/. テキスト表現。
- **2600 Screen Editor v0.7**（AtariAge スレ）— https://forums.atariage.com/topic/349056-2600-screen-editor-v07/
  ※フォーラムスレ。背景採掘エージェントが採掘候補。スプライト＋PF を1画面で扱う統合型の可能性。

## GitHub/web 探索の追加発見（2026-06-13・準備フェーズ）

### ブラウザ内 2600 エミュレータ（M5「実機裏打ち」をブラウザ完結する選択肢）
- **★Stellerator-embedded（6502.ts）** — github.com/6502ts/6502.ts（**MIT**・DirtyHairy・TS・2026-04 活発・64★）。
  **埋め込み用に設計された TS 製 2600 エミュ**。MIT＝**採用が最も楽＝M5 の最有力候補**。stellerator-embedded API で
  canvas に組み込み→生成カーネルを即実走の見込み。**javatari の AGPL を避けられる。**
- **M3 合成画面の最も近い既存（コンセプト先行・我々の差別化の裏付け）**: **aloan "Atari Graphics Simplified"**
  （aloan.neocities.org/atari_graphics_simplified）＝7要素を1画面合成・flicker強制・公式パレット・8pxグリッド。
  **だが Clickteam Fusion 製で .exe を吐く"2600風"フェイク**（本物のROM/asm でない）→ **本物の合成エディタは存在しない＝M3 が新規**。
- **javatari.js** — github.com/ppeccin/javatari.js（**AGPL-3.0**・235★・2026 も保守・HTML5/JS・次点）。
  **CRT phosphor 効果**あり（＝唯一「CRT の見た目」を再現）。JS API で ROM ロード＋console 制御可。
  フレームバッファ readback は未文書化だが **canvas の `getImageData` で画素を読める**はず。
  ★ライセンス：AGPL-3.0（強コピーレフト）。**ただしユーザーが作者(ppeccin/Paulo Peccin)とコンタクト有り＝最悪直接許諾を確認できる**
  （2026-06-13 ユーザー談）→ **AGPL は実質ブロッカーでない**。よって2択を両方活かせる：
  - **(i) ブラウザ内ライブプレビュー＝javatari.js を embed**（CRT 見た目つき・getImageData で画素読み）。採用時は作者に license 確認。
  - **(ii) 権威ある検証＝我々の Gopher2600（GPL・自前制御）でサーバ側実行**（M5 の正本）。
  推奨構成：**(ii) を正本、(i) を任意の即時プレビュー**。両立可。
- jsAtari（docmarionum1・GPL-3.0・13★）／html5atari（jstoudt・**ライセンス無し=不採用**）。

### ★ブラウザ内 IDE / 学習基盤（ユーザー注目・恒久リファレンス）
- **8bitworkshop** — 8bitworkshop.com ／ github.com/sehugg/8bitworkshop（**GPL-3.0**・581★・**2026-06 も活発**）。
  ブラウザ内の 2600 IDE＝**エディタ＋アセンブラ＋エミュレータ＋ビジュアルデバッガ**（ビーム/メモリ/スコープ可視化）。
  Steven Hugg 著『Making Games for the Atari 2600』が土台＝**設計・カーネル・作画の体系的教材**。**ユーザーは自分のリポ
  (kidsnz/test) で実際に使用**（＝ユーザーの実開発環境の一部）。GPL なので harness と license 相性良。
  学べるもの：(a) ブラウザ内 emulator 統合の作法（canvas の M5 ライブプレビュー参照）(b) ビジュアルデバッグ UX
  (c) 例題ソースで 2600 技/作画の定石 (d) 教材本＝design-principles.md の一次資料。
- **sehugg/awesome-8bitgamedev**（180★）＝8bit ゲーム開発の資源カタログ（ツール/エミュ/教材の総覧）。探索の出発点に。
- **javatari.js**（再掲・JS製＝コードを読んで学べる）＝CRT 効果つき 2600 エミュレータ。embed 候補＋実装の教材。

### study/fork できる editor ソース
- **VCS Graphics Editor**（SourceForge・Java・オープン）。pfed gist（gist.github.com/ocarneiro/d4dd29af…）。
- bocianu/spred（GitLab・8bit だがオープン・UX 参考）。masswerk（ソース公開状況は要確認）。

### 自前 canvas を書くための一般コードパターン（私が実装する型）
- **Eloquent JavaScript "A Pixel Art Editor"**（eloquentjavascript.net/19_paint.html）＝canvas ピクセルエディタを
  ゼロから作る定番チュートリアル（state/undo/ツール構成）。**私が tia-studio を書く時の骨組みの参照**。
- **Piskel**（オープン・パレット制限あり）／**Pixelorama**（オープン）／**Pixellate**（HTML canvas JS・最小）。UX/コード参考。

### 設計原則の定番チュートリアル（design-principles.md の素材）
- **randomterrain "Atari 2600 Programming for Newbies"（Andrew Davie）** — Session 20 非対称PF / 21 スプライト /
  23 縦スプライト配置。**カーネル設計の正典**。
- **splendidnut "2600 Display Kernels" ブログ**（AtariAge）＝カーネル合成の参照。
- **Bumbershoot Software "Vertical Sprite Placement"**。

## 採用方針サマリ（自前ツールに入れる）
1. スプライト: 大キャンバス＋**アニメフレーム列**＋**走査線毎カラー**（実 Stella パレット＋スポイト）＋幅1/2/4＋reflect。
2. PF: 対称/非対称＋reflect/repeat（正しいビット反転）＋行毎カラー。
3. **export 複数形式**: DASM 行順 `.byte %xxxxxxxx` ／ PF0/1/2 ラベル配列 ／（任意）X/. テキスト。**色コメント埋込で無損失往復**。
4. **画像インポート** = cmd/ingest と接続（PNG→TIA）。**project save/load** = JSON。
5. 我々だけの強み: **harness 忠実描画**・**JSON モデルを私がコードで駆動**（設計生成→canvas→スクショ→判断→反復）・P0+P1+M+BL 合成プレビュー。
