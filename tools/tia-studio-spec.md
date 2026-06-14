# tia-studio — 仕様ドラフト（ビジョン・2026-06-13）

ユーザー発案。既存ツール（[[tia-studio-references.md]]）を超える「2600 シーン設計スタジオ」。
個別要素エディタ＋**全要素を1画面に合成して配置できるコンポジット・キャンバス**＋**ゲーム別テンプレート**。

## ユーザーのコア・ビジョン
1. **独立した要素エディタ**：Sprite0 / Sprite1 / Playfield（／Missile0,1 / Ball）を別々に編集。
2. **走査線ごとの色指定**：各要素の各ラインの横軸に色を置く（per-scanline COLUP）。
3. **★新規＝コンポジット・キャンバス**：それら要素が**全部1画面内に配置された大きめのキャンバス**。
   **スプライト位置をボタンで移動でき、配置場所を選べる**（＝実 TIA 座標での WYSIWYG 画面合成）。
4. **テンプレート**：ゲームごとにスプライト数・要素構成が違う → **いろんな組合せパターンがテンプレ化**されて選べる。

なぜ新しいか：既存ツールは要素を**単体**で編集する（PlayerPal=スプライト、PlayfieldPal=PF）。
**全要素を実画面に合成して位置決めする WYSIWYG は存在しない**。これは 2600 の本質（1画面＝P0+P1+M0+M1+BL+PF を
走査線ごとに合成＋位置）にそのまま対応する。

## Claude の追加アイデア（ここが我々だけの強み）
- **A. harness 裏打ちのプレビュー（killer feature）**：コンポジット・キャンバスの出力は単なる絵バイトでなく**シーン**
  （要素グラフィック＋位置＋走査線毎色）＝カーネルが描くものそのもの。だから **シーン→カーネル自動生成→Gopher2600 で実走→
  スクショ→キャンバスと差分**まで回せる。**プレビューが JS 近似でなく"実エミュレータ"で裏打ちされる**＝既存ツールに原理的に不可能。
- **B. テンプレート＝我々の検証済みカーネル技**：テンプレは恣意的でなく zone_multiplex / dyn_multisprite / two_line_kernel /
  score6 / bitmap48 等の**カーネル骨格**に対応。テンプレ選択＝カーネル骨格選択、スタジオが絵・位置・色を流し込む。
  ＝**設計スタジオが techniques カタログの上に乗る**。
- **C. サイクル予算フィジビリティ**：各テンプレは予算を持つ。配置・走査線数が 76cy/行を超えたら `assert_line_budget` 連動で警告。
  **設計と実現可能性が同じ画面**（移動コスト＝RESP+HMOVE のサイクルも可視化、calibrate/X(N) 連動）。
- **D. ingest インポート**：Photoshop モック等を drop → cmd/ingest → キャンバスの要素に自動分解（あなたの視覚意図→スタジオ）。
- **E. コード駆動シーンモデル**：シーン＝JSON。**私が関数でシーンを生成・反復**できる（設計生成→canvas→スクショ→判断→反復）。
- **F. アニメ**：シーン横断のフレーム（sprite_anim 技）。

## マイルストーン（各段が単体で有用＝大作業を分割）
- **M1**：単一スプライト GRP エディタ＋走査線毎カラー＋Stella パレット描画＋`.byte`/ラベル配列 export（＝最初の小試作）。
- **M2**：PF エディタ（対称/非対称・reflect/repeat）＋要素セット（P0/P1/M0/M1/BL）。
- **M3（新規の核）**：コンポジット・キャンバス＝要素を TIA 座標に配置・ボタン移動・走査線毎色を統合。
- **M4**：テンプレート（＝検証済みカーネル技）＋予算フィジビリティ。
- **M5（killer）**：シーン→カーネル生成→harness 実走→キャンバスと差分（実機裏打ちプレビュー）。
- **M6**：ingest インポート・アニメ・save/load(JSON)・color コメント無損失往復。

## 技術スタック（buildable 架構・全 MIT・研究で確定 2026-06-13）
ゼロから書かず、**MIT の既存を土台に組む**（legal に embed/fork 可）。詳細＝`research-w1-tooling.md`。
- **編集レイヤ＝spritemate（Esshahn・MIT）を fork**：埋め込み可能なバニラ JS canvas スプライトエディタ。ピクセル編集/undo/パレットの土台。
- **ライブプレビュー＝6502.ts / Stellerator-embedded（DirtyHairy・MIT）を embed**：`new Stellerator(canvas).run(rom,ntsc)`→`getImageData` で画素読み。
- **配線の参考＝vcs-game-maker（haroldo-ok・MIT）**：ブラウザ内コンパイル→プレビューの完全パイプライン（bB だが「シーン→ROM→プレビュー」の流れを学ぶ）。
- **権威ある検証＝我々の Gopher2600（GPL・サーバ側）**：M5 の正本。ブラウザ即時(Stellerator)＋権威(Gopher2600)の二段。
- **描画忠実度＝Stella 実測パレット（`internal/ingest/palette_stella.go`）＋PF ビット順（CLAUDE.md）**。
- ライセンス禁則：javatari(AGPL=embed 罠・次点)／8bitworkshop・background-builder・Stella(GPL=study専用)／atari2600-wasm・masswerk・aloan(不可/概念のみ)。

## 正直なリスク／方針
- 規模は大きい。だが**層で出す**（M1 から各段が単体で使える）。novelty は M3＋M5。
- **M3 novelty の精緻化（研究後）**：合成スクリーンエディタ＋codegen は**1つ実在**（lucienEn の **ScreenEditor2600**・Win .NET・closed・58.5MB／
  reference/atariage/349056）。だが **Web/canvas でない・OSS/embed 不可・harness 非連携・コード駆動不可**。
  → 我々の差別化＝**「Web・OSS で組める・実機検証・私がコードで駆動できる」合成エディタ**。需要と実現性は裏付けられた。
- 既存ツールへの敬意：描画の正しさは既存も達成済み（[[tia-studio-references.md]] 参照）。我々の価値は
  **合成WYSIWYG（M3）＋実機裏打ち（M5）＋テンプレ=技（M4）＋コード駆動（E）**。
- 設計原則は `../docs/design-principles.md`（テンプレ寸法・フィジビリティ4軸の根拠）。
- 1ファイル HTML から始め、肥大したら分割。export 形式は roms/techniques と cmd/ingest 準拠。
