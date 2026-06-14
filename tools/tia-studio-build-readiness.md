# TIA Studio — Build-Readiness（着手台帳）

このノート = w1〜w9 の研究 + Pizza Boy 知見を「ビルドを始めるための単一の着手台帳」に畳んだもの。
研究の詳細は各 `research-w*.md` と `reference/pizza-boy/dissection.ja.md` を参照。
**このノートを読めば「今日どのファイルを触るか」がわかる** 状態を保つこと。

作成日: 2026-06-13。
仕様の正本: `tia-studio-spec.md`（M1〜M6 ビジョン）。
設計原則の正本: `../docs/design-principles.md`。

---

## 横断テーマ（全マイルストーンを貫く方針）

| テーマ | 内容 | 出典 |
|---|---|---|
| **bB を超える** | 固定カーネルでなくゾーン合成生成（走査線帯×要素×色を自由に設計）。Pizza Boy は design の ground-truth・技術の天井ではない | `design-principles.md`・`dissection.ja.md` |
| **D2 ループ（二系統）** | ①abb 経路（atari-background-builder .abb.json → `cmd/ingest -abb` → DASM）②ROM 直接 ingest（load_rom→analyze_screen、grade-A 実証済）。Photoshop → TIA バイト往復の正本 | w7・w8・`dissection.ja.md` |
| **二段プレビュー** | ブラウザ = Stellerator 即時（見た目確認）、サーバ = Gopher2600 権威（pixel 照合の正本）。getImageData は直接不可（WebGL）だが ImageData は VideoEndpointInterface.newFrame で 5〜8行で取得可（M5 まで保留） | w3・w4 |
| **Pizza Boy を受け入れテストに** | `pizza-boy-20220804.bin` を実走し grade-A ingest 照合（avg_dist=0, fidelity=1 達成済）→ TIA Studio が生成したカーネルを同じ基準で検証 | `dissection.ja.md` |
| **色はレジスタ値で持つ** | spritemate の palette index ではなく TIA レジスタ値（hue 上位 × lum 下位ニブル）で内部表現。量子化変換は `palette_stella.go`（実走実測・Stella 100% 一致） | w2・w4・`design-principles.md` |
| **出荷規律** | harness コードは branch→test→tag。serverInfo 版番号も上げる | `../CLAUDE.md`・STATUS.md |

---

## マイルストーン別着手台帳

### M1：単一スプライト GRP エディタ＋走査線毎カラー＋Stella パレット＋DASM export＋Stellerator プレビュー

| 項目 | 内容 |
|---|---|
| **目的** | 最初の動くもの。spritemate fork を TIA 用に改修し、GRP + per-row COLUP を DASM `.byte` で吐き、Stellerator で即時確認できるブラウザエディタ |
| **確定済み（decided）** | spritemate fork の現行スタック = TypeScript + Vite（w3 で vanilla-JS から訂正済み）。`SpriteData.color:number` → `colorPerRow:number[]`（TIA レジスタ値）が M1 データモデルの正本（w4 確定）。改修は 9 ファイル（5 変更 + 1 追加 + 1 新規 + worker 配置）に局所化済み（w6）。Stellerator embed は `run(romBytes, ntsc)` で無改造・Stellerator worker は public/ に同梱（w3・w4）。パレット正本は `palette_stella.go`（128色・TIA レジスタ値 index 規則 = TIA_reg / 2）（w2・w6）。Export は `create_tia_assembly()` を新規追加、旧 C64 `create_assembly` は残置（w6）。PlayerPal の「行末 color コメントで無損失往復」が per-row color の前例（w4）。 |
| **未解決の de-risk** | Stellerator worker の同一ドメイン配置・CORS 確認は実機のみ（npm run dev では public/ 参照で OK のはず）。Stella 実測パレットと Stellerator 表示色の一致未確認（色照合正本はサーバ側なので影響は限定的）。 |
| **何を出荷するか** | `tia-studio/` フォルダ（spritemate fork）が `npm run dev` で立ち上がり、8×16 グリッドを編集・行ごと色ストリップで COLUP を設定・DASM export・.bin を file input で Stellerator canvas に表示できる状態 |
| **着手順（最初に書くファイル）** | 1. `git clone/fork Esshahn/spritemate → tia-studio/` 2. `src/js/config.ts`：sprite_x=8/sprite_y=16 + Stella 128色パレット（`scripts/gen_stella_palette.js` で `palette_stella.go` から抽出）3. `src/js/SpriteTypes.ts`：`colorPerRow:number[]` 4. `src/js/Sprite.ts`：flip-Y/shift-Y に colorPerRow 同期 5. `src/js/Editor.ts` + `Window_Controls.ts`：塗り色 colorPerRow[y] + color strip UI 6. `src/js/Export-Base.ts`：`create_tia_assembly()` 追加 7. `src/js/TiaPreview.ts`（新規）+ Stellerator worker 配置 |

---

### M2：PF エディタ + 要素セット（P0/P1/M0/M1/BL）

| 項目 | 内容 |
|---|---|
| **目的** | M1 の GRP エディタに PF（左右 20 列、対称/非対称/reflect/repeat）と Missile/Ball の幅設定を追加し、要素セットを揃える |
| **確定済み（decided）** | PF ビット順の正典確定済み（PF0 上ニブル col0→D4、PF1 MSB first col4→D7、PF2 LSB first col12→D0）、litmus 100% 一致（CLAUDE.md）。`pkg/playfield.EncodeSymmetric/EncodeAsymmetric` が正典エンコーダ（w8 で abb との一致も確認）。非対称 PF の書換え締切（PF2 = cy45 厳守・nop1個で破綻）は `design-principles.md` に記録済み。masswerk の「ラベル付き配列 per PF レジスタ」export 形式がクリーンな出力形（w1 確認・コードは non-OSS なので概念のみ）。vcs-game-maker の `matrixToPlayfield`（`X/.` 行列↔ テキスト変換）が PF 表現の参考（w3・w5）。 |
| **未解決の de-risk** | spritemate の `Sprite.ts` を PF 用（40列幅・1bit/cell）に再利用するか、別コンポーネントを新規作成するかは M1 の実装経験次第で判断。M1 完了後に決定。 |
| **何を出荷するか** | PF エディタ（40列グリッド、対称/非対称/reflect/repeat 切替、per-row COLUPF）+ GRP/PF の合計 export（`create_tia_assembly()` に PF テーブルを追加） |
| **着手順** | M1 完了後。`src/js/Playfield.ts`（spritemate に既存だが C64 プレイフィールド用）を TIA 40列 PF 用に改修するか、新規 `TiaPlayfield.ts` を作成。Export に `<name>_pf0/_pf1/_pf2/_pfcol` テーブルを追加。 |

---

### M3（新規の核）：コンポジット・キャンバス = 要素を TIA 座標に配置・ゾーン合成カーネル生成

| 項目 | 内容 |
|---|---|
| **目的** | 全要素（P0/P1/M0/M1/BL/PF）を1画面に TIA 座標で配置・ボタン移動できる WYSIWYG キャンバス + それらからゾーン合成型 DASM カーネルを自動生成する（既存ツールに存在しない新規領域） |
| **確定済み（decided）** | codegen 推奨アーキ = **ゾーン合成型**：シーン JSON → ①ゾーン分割（境界 = 要素 Y 範囲の変化点）→ ②ゾーン毎にカーネル断片テンプレ選択 → ③断片内穴埋め（grfx/PF/colorPerRow/位置データ注入）→ ④ゾーン間縫合コード生成（RESP/HMOVE/VDEL/色切替）→ ⑤マスターテンプレ（VSYNC/VBLANK/Overscan + データ表ラベル + ゾーン列）（w5）。中間表現 2 層 = 要素グラフィック層（pixels + colorPerRow）+ シーン配置層（TIA座標 + ゾーン境界）（w5）。実ゲーム 5 本（RiverRaid/Pitfall/Defender/Adventure/Carnival）が全例「固定ループ断片 + 境界縫合」構造を採用し w5 を裏付け（w9）。境界縫合の型は 4 種（I=クリアのみ/II=色+NUSIZ 切替 1ライン/III=RESP 再配置 2ライン/IV=RESP+HMOVE 微調整 2ライン）、前後ゾーンの TIA 要素 diff から自動選択（w9）。最小ゾーン高さ = 4 ライン（Pitfall Kernel 2 実測）（w9）。シーン = JSON（コード駆動 spec E に対応）。生成器は Go 側（harness）に置くと assemble_and_load + assert_line_budget と同プロセスで回せて最良（w5）。ScreenEditor2600（lucienEn・Win/.NET/closed）が唯一の先行例だが Web/OSS/harness 非連携のため差別化確定（w1・tia-studio-spec）。 |
| **未解決の de-risk（最大の不確実性）** | **ゾーン境界縫合コード（型 III/IV・RESP 再配置付き 2ライン）が 76cy 制約を守れるか未実走**。w5 §7 + w9 §5-A 共通の最高優先 de-risk。`assert_line_budget` で実走数値確認が必要。非対称 PF を含むゾーンで他要素（GRP/色更新）と同一走査線に共存できるか未確認（cy45 一点厳守 + 残 29cy）。生成器の置き場（ブラウザ JS vs Go harness）は M3 着手時に確定要（ハイブリッド = ブラウザがシーン JSON 吐き、Go がカーネル生成+検証が有力）。 |
| **何を出荷するか** | 「2ゾーン・2要素の最小プロトタイプ」：シーン JSON（上帯 P0 のみ + 下帯 P0+PF 対称）→ ゾーン分割器 + 境界縫合 1 箇所 → DASM → `assemble_and_load` → `assert_line_budget`（76cy 内確認）→ `read_tia`（位置・色確認）→ `.bin` → Stellerator 即時プレビュー。これが通れば「ヘテロ合成の核」成立。 |
| **着手順** | 1. Go 側に `cmd/tiastudio/` または `internal/codegen/` を新設 2. シーン JSON schema 定義（要素グラフィック層 + シーン配置層の 2 層）3. ゾーン分割器（境界検出）4. 断片テンプレ 2 種（P0 のみ帯 + P0+PF 対称帯）5. 型 II 境界縫合コード生成 6. マスターテンプレ（VSYNC/VBLANK/Overscan 定型）7. `assemble_and_load` → `assert_line_budget` で実走検証 |

---

### M4：テンプレート（= 検証済みカーネル技）+ 予算フィジビリティ

| 項目 | 内容 |
|---|---|
| **目的** | ゾーン断片テンプレをカタログ化（= `docs/techniques/` の検証済み技に 1 対 1 対応）し、「テンプレ選択 = カーネル骨格選択」にする。+ 各テンプレの予算メタで 76cy 超過をユーザーに警告 |
| **確定済み（decided）** | テンプレ優先度：`score6`（全ゲーム共通・NUSIZ THREE_COPIES + VDEL + 走査線毎 GRP、最優先）・`two_line_kernel`・`zone_multiplex`・`dyn_multisprite`・`bitmap48`（w9 §4-E + tia-studio-spec）。vcs-game-maker の Handlebars マスターテンプレ + 名前付きスロット穴埋めパターンが M4 テンプレ配線の参考（w3・w5）。batari Basic の固定カーネル（`std_kernel.asm`/`multisprite_kernel.asm`）は「ゾーン内容は固定パターン + 表差し替え」の長所を持つが手書き品質に届かない制限も実走ソースで確認（w5 §2）。predict cycle budget は `assert_line_budget` 連動（CLAUDE.md 既存ツール）。位置コスト可視化は calibrate/X(N) 連動。 |
| **未解決の de-risk** | 初期テンプレカタログのセット確定（`docs/techniques/` の実在技と 1 対 1 棚卸しが要る）。RESP 再配置コストをテンプレのメタに自動で持たせる設計。 |
| **何を出荷するか** | `score6` テンプレを最初に実装・実走検証（型 III 境界縫合のリファレンス実装も兼ねる）。UI は「テンプレ選択肢リスト」として M3 コンポジット・キャンバスに統合。 |
| **着手順** | M3「2ゾーン最小プロト」完成後。`score6` 断片テンプレ（命令列テンプレ + データ表ポインタ）を最初に実装。その後 `two_line_kernel`、`zone_multiplex` の順で追加。 |

---

### M5（killer）：シーン → カーネル生成 → harness 実走 → キャンバスと差分（実機裏打ちプレビュー）

| 項目 | 内容 |
|---|---|
| **目的** | M3/M4 の codegen パイプラインを Gopher2600 実走につなぎ、キャンバスとスクショの pixel-level diff を自動実行する。既存ツールに原理的に不可能な「実エミュレータ裏打ちプレビュー」 |
| **確定済み（decided）** | 照合の正本 = サーバ側 Gopher2600（w3・w4 で確定・ブラウザ Stellerator は即時見た目専任）。ブラウザ側 pixel 読出しは `VideoEndpointInterface.newFrame: Event<ImageData>` が WebGL 前に ImageData を持つため 5〜8行の fork で実現可（preserveDrawingBuffer 不要）、ただし M5 まで保留（w4 §Q2-b）。`assemble_and_load`（dasm→load 一発）→ `get_screen_annotated`（スクショ + TIA 座標）→ `assert_line_budget` が既存ツールで完備（CLAUDE.md）。Pizza Boy ROM 実走 grade-A 照合（fidelity=1）が D2 ループの実証済み基準（`dissection.ja.md`）。 |
| **未解決の de-risk** | ブラウザ Stellerator 側の ImageData フック（5〜8行の fork）の実機動作確認。M3/M4 が安定してから着手。 |
| **何を出荷するか** | 「シーン JSON → 生成カーネル → Gopher2600 実走 → スクショ → キャンバスと pixel diff レポート」の自動ループ。ユーザーが1クリックで設計の実機正確性を確認できる。 |
| **着手順** | M4 完了後。`assemble_and_load` + `get_screen_annotated` を HTTP エンドポイントとしてラップ（or Vite proxy で転送）。キャンバス出力の PNG と `get_screen_annotated` の PNG を diff して diff 画像 + 数値 (avg_dist, max_dist) をブラウザに返す。 |

---

### M6：ingest インポート・アニメ・save/load（JSON）・color コメント無損失往復

| 項目 | 内容 |
|---|---|
| **目的** | Photoshop モック（PNG / .abb.json）→ キャンバスへの自動分解、スプライトアニメーション（sprite_anim 技）、JSON save/load、color コメント無損失往復 |
| **確定済み（decided）** | D2 ループの最短経路 = abb 経路（`.abb.json` → `cmd/ingest -abb` → DASM PF テーブル）。schema 確定・関数シグネチャ案まで設計済み（w8）。`pkg/playfield.EncodeSymmetric/EncodeAsymmetric` を再利用（litmus 検証済み・abb と規約一致）。PNG 順方向変換のアルゴリズム = `readSquare()`（atari-background-builder 流・2値化 + コントラスト自動チューニング）+ Dithertron `TwoColor_Canvas`（ヒストグラム 2色選択・max 100 イテレーション）（w7）。Dithertron の弱点（per-line COLUPF を ASM export しない）は abb 経路には存在しない（colorGrid を転記するだけ）（w8）。PlayerPal の「行末 color コメントで無損失往復」が M6 color コメント往復の直接前例（w4）。アニメ = vcs-game-maker `generateAnimations()` の「フレーム配列 + reverse + %bin + frame カウンタ SM」が実装参考（w3・w5）。spritemate `Animation.ts` が再利用可（ノータッチ、w6 §3）。 |
| **未解決の de-risk** | `cmd/ingest -abb` の Assym 非対称 PF 対応（height 異常の旧版 .abb.json）。Dithertron per-line COLUPF の ASM 取り出し（fork か自前）。GRP 順方向変換（8px スプライトへの任意画像縮小）の精度。 |
| **何を出荷するか** | `.abb.json` drop → PF テーブル自動生成（`cmd/ingest -abb`）。PNG drop → GRP + PF 自動分解（ingest 順方向モード）。アニメフレーム編集。JSON save/load。 |
| **着手順** | abb 経路（`cmd/ingest -abb`）が最短・最高優先。1. `internal/ingest/abb.go`（LoadABB + ToBands + DASMFromABB）2. `cmd/ingest -abb` 分岐 3. pb_04.abb.json → `assemble_and_load` → `read_row` で PF バイト照合。 |

---

## D2 ループ（二系統）整理

```
[経路①] Photoshop PSD
  → atari-background-builder (.abb.json = yxGrid + colorGrid + mode + spl)
  → cmd/ingest -abb  [未実装・w8 で設計完了]
  → DASM ASM (PFData/PFColu/PFColuBk/PFHeight テーブル)
  → assemble_and_load → harness 検証

[経路②] 実 ROM
  → load_rom → analyze_screen (grade-A 実証済・fidelity=1)
  → GRP/PF/色テーブル抽出
  → TIA Studio のキャンバスに逆インポート  [M6 スコープ]
```

**Pizza Boy 受け入れテスト**：`pizza-boy-20220804.bin` を実走 → source 定数（BK$6c/ビル$ae/P0$ce/タクシー$1e/木$c6・PFバイト）と 100% 一致達成済。M3/M4/M5 で生成したカーネルを同じ基準で検証する。

---

## 今すぐ着手できる順序付きタスク列

以下は M1 から M3 最小プロトまでの着手可能な順序。**各タスクは単体で検証が閉じる小ステップ**。

| 順 | タスク | 出荷 | 確認方法 |
|---|---|---|---|
| **1** | spritemate を `tia-studio/` に fork + `npm run dev` 動作確認（C64 版が起動することを確認） | 基線確保 | http://localhost:5173 |
| **2** | `config.ts`：sprite_x=8/sprite_y=16 化 | 8×16 グリッド | Editor が 8×16 グリッドになる |
| **3** | `scripts/gen_stella_palette.js` で `palette_stella.go` から 128色 hex 抽出 → `config.ts` に差し替え | Stella パレット | Palette ウィンドウに 128色表示 |
| **4** | `SpriteTypes.ts`：`color:number` → `colorPerRow:number[]` + `createSprite` 初期化変更 | データモデル確定 | `npm run build` でコンパイルエラー一覧 → 次タスクの改修対象確認 |
| **5** | `Sprite.ts`：flip-Y / shift-Y に colorPerRow 同期追加 | flip/shift 正常動作 | 縦反転で color strip も一緒に反転 |
| **6** | `Editor.ts` + `Window_Controls.ts`：colorPerRow[y] で塗り色 + color strip UI 追加 | per-row 色描画 | 行ごと色ストリップをクリックで行色変更 |
| **7** | `Export-Base.ts`：`create_tia_assembly()` 新規追加（GRP MSB-first + colorPerRow → COLUPx テーブル） | DASM export | DASM で `.bin` 生成できる |
| **8** | Stellerator worker 配置 + `TiaPreview.ts` 新規作成 + `index.html` に canvas 追加 | Stellerator プレビュー | file input で .bin → canvas に GRP 描画 |
| **9（M1 完成）** | 手動で export → DASM → `.bin` → file input → Stellerator canvas に色付き GRP 確認 | M1 動作確認 | 視覚確認 + harness assemble_and_load + get_screen_annotated |
| **10** | `cmd/ingest -abb` 実装：`internal/ingest/abb.go`（LoadABB・ToBands・DASMFromABB）| abb 経路 | pb_04.abb.json → read_row で PF バイト照合 |
| **11（D2 最短路完成）** | `pb_04.abb.json` → `cmd/ingest -abb` → `assemble_and_load` → `read_row` で PF バイト数値照合 | D2 abb 経路 | read_row の各列 bool が yxGrid と一致 |
| **12（M3 最大 de-risk）** | 「2ゾーン最小プロト」を手書きで作成：上帯 P0 のみ / 下帯 P0+PF 対称 → `assemble_and_load` → `assert_line_budget` で 76cy + HMOVE 後 24cy を数値確認 | ゾーン縫合の実走証明 | assert_line_budget が両ゾーンで PASS |

タスク 1〜9 は M1 の完全な着手手順書が w6 に存在する（ファイル名・行番号・コード差分まで記載済み）。

---

## ユーザー GO 待ちの判断点

| 判断点 | 内容 | 状態 |
|---|---|---|
| **M1 着手 GO** | `tia-studio/` として spritemate fork を始める。tia-studio リポを `260609_atari2600-dev/` 配下に作る | **GO 待ち（研究完了・手順書完備）** |
| **M3 最小プロト GO** | 「2ゾーン最小プロト」の実装着手。codegen Go コードを harness に追加（branch→test→tag 規律）| **GO 待ち（設計完了・最大 de-risk が明確）** |
| **`cmd/ingest -abb` GO** | Pizza Boy abb.json → DASM パイプラインの実装着手。harness の branch 運用 | **GO 待ち（設計・schema・関数シグネチャ完備・w8）** |
| **M5 ブラウザ ImageData フック** | Stellerator.ts への 5〜8行 fork（`VideoEndpointInterface.newFrame` 購読）。MIT なので可 | M5 まで保留 |

---

## 出典一覧

| 主題 | ドキュメント |
|---|---|
| M1〜M6 ビジョン・技術スタック確定 | `tia-studio-spec.md` |
| ツール棚卸し・ライセンス判定 | `research-w1-tooling.md` |
| TIA 設計原則（GRP/PF/色/予算）・パレット ground-truth | `research-w2-design.md` |
| spritemate/Stellerator/vcs-gm の実装 API（TS fork ・WebGL 訂正） | `research-w3-buildable-apis.md` |
| M1 設計判断確定（colorPerRow / framebuffer 二段構成） | `research-w4-m1-open-questions.md` |
| M3 codegen 推奨アーキ（ゾーン合成型・w5 の核） | `research-w5-m3-codegen.md` |
| M1 着手手順書（7コミット・ファイル名・行・コード差分） | `research-w6-m1-fork-plan.md` |
| D2 ループ・画像→2600 変換パイプライン（abb 最短路） | `research-w7-image-to-2600-pipeline.md` |
| abb.json schema 確定・`cmd/ingest -abb` 実装設計 | `research-w8-abb-pipeline.md` |
| 実ゲーム 5 本のゾーン分割・境界縫合 4 型・score6 最優先 | `research-w9-real-kernel-patterns.md` |
| 設計原則正本（TIA Studio フィジビリティ判定の根拠） | `../docs/design-principles.md` |
| Pizza Boy 解剖・grade-A 照合・D2 実証・bB を超える方針 | `reference/pizza-boy/dissection.ja.md` |
