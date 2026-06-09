# Resource Plan — このプロジェクトに必要な資料

`docs_atari/`（Pong 用の学習コーパス）は「ゲームを書いて学ぶ」ために集めたもの。
本プロジェクトは「**検証ハーネスを作る + 制約を蒸留する**」ので、必要な資料の種類が変わる。
ここで**改めて必要なものを棚卸し**し、「既に有る／新規に要る」を分類してリサーチ対象を定める。

凡例: ✅=docs_atari に既に有る / 🔍=新規にリサーチ / ⭐=過去の失敗に直結（優先）

---

## カテゴリ 1 — 制作の核心定数（CLAUDE.md へ蒸留する用）

欠落 C の対策は「収集」ではなく「蒸留」。**記憶に頼らず常時文脈に置く簡潔な決定版**が要る。

- ⭐🔍 **横位置の正確な数式とハード固有オフセット** — RESPx ストローブのサイクルと表示ピクセルの関係、
  `P1 = 160 - P0 - 幅` が合わない理由、divide-by-15 粗調整、HMxx ニブル → ±ピクセルの対応表。
  （＝Pong の失敗 #1「魔法定数の総当たり」を二度とやらないための核心。権威ある数値で。）
- ⭐🔍 **HMOVE の挙動** — HMOVE comb（左端の黒線）、early HBLANK、HMOVE は WSYNC 直後必須、毎ライン HMOVE。
- 🔍 **フレーム予算の確定値** — NTSC/PAL/SECAM の総ライン・VSYNC/VBLANK/可視/オーバースキャン・
  228 カラークロック/76 CPU サイクル/68 HBLANK。
- ✅🔍 **TIA レジスタ完全ビットマップ** と **6502/6507 命令サイクル表**（分岐・ページ跨ぎペナルティ込み）の
  蒸留向けチートシート。（Stella Programmer's Guide にはあるが「1 枚物」が要る）
- 🔍 **衝突レジスタ（CXxx）の読み方**。

## カテゴリ 2 — ハーネス構築資料（ほぼ全て新規。Pong では不要だった）

- 🔍 **Gopher2600 開発者ドキュメント** — Gopher2600-Docs wiki、debugger terminal の全コマンド、
  `PushedFunction` でのプログラム駆動、フレームバッファ取り出し、color-clock 検査。
- 🔍 **Go の MCP SDK** — 公式 go-sdk と mark3labs/mcp-go、2026 時点の推奨、ツール露出と stdio。
- 🔍 **参考実装** — mcp-gameboy（画面を画像で返す形）と vice-mcp（埋込 MCP の形）のアーキテクチャ。
- 🔍 **Stella の正確な仕様** — 1x 単発スナップショットの CLI フラグ、`-dbg.script`/`dump` の出力形式、
  **Fixed Debug Colors**（どの色がどのオブジェクトか）の有効化方法。
- 🔍 **画像オーバーレイ手法** — 160×192 スナップに XY グリッド＋軸ラベル＋マーカーを合成（Go image vs ImageMagick）。

## カテゴリ 3 — 検証・テスト資料（大半が新規）

- 🔍 **テスト ROM・既知状態のリファレンス** — ハーネス（エミュ）自体の校正・正しさ確認用。
- 🔍 **決定的な入力リプレイ** — Stella / Gopher2600 でのムービー／入力スクリプト再生の方法（欠落 D）。
- 🔍 **2600 回帰テストの事例** — homebrew コミュニティの自動テスト手法。
- ✅🔍 **sim65 / 6502profiler の具体的テスト記述例**。

## カテゴリ 4 — 既存資料の再評価

- ✅ `docs_atari/` の棚卸しは post-mortem 済み（`gap-analysis.md` 参照）。
  本プランは「新視点で足りないもの」を上記 1〜3 で補完する位置づけ。

---

## リサーチ・ストリーム（並行実行）

- **Stream A（ハーネス構築）= カテゴリ 2** — ✅ `完了 2026-06-09`（末尾）。
- **Stream B（ドメイン定数＋検証）= カテゴリ 1・3** — ✅ `完了 2026-06-09`（下記）。

> 結果は本ファイルに反映し、CLAUDE.md 蒸留（Phase 4）とハーネス実装（Phase 2）の入力にする。

---

## Stream B 結果（ドメイン定数＋検証）2026-06-09

蒸留用にそのまま CLAUDE.md / `docs/2600-constants.md` へ落とせる確定値。

### ⭐ 横位置（失敗 #1 の解毒剤）= NEW・最重要
- **公式:** ミサイル/ボール `X = 3N − 55`（N = 同期点から RESPx ストローブまでの CPU サイクル数）、
  **プレイヤーは +1px → `X = 3N − 54`**。最左 X = 2（プレイヤー 3）。
- **オフセットの正体:** RESPx はストローブで、ストア完了後 TIA が描画開始するまで **約 5 カラークロックの遅延**。
  これ ＋ **HBLANK 68 クロック**が「`160 − P0 − 幅` が合わない」理由。HBLANK 中のストローブは最左端に置かれる。
- **粒度:** 3 クロック/CPU サイクル → RESPx は **3px 刻み**。残りは HMOVE で詰める。
- **divide-by-15 粗調整:** ターゲット X を 15 で割り、**5 CPU サイクル（=15 カラークロック）ループ**で時間を潰す。
  `SBC #15 / BCS loop`。余り(0–14)で HMOVE 微調整値をページ境界揃えのテーブルから引く。

### ⭐ HMOVE ニブル表（罠: 正=左／負=右）
上位ニブル(D7–D4)のみ・**2 の補数**・範囲 **+7（左7）〜 −8（右8）**。動くのは **HMOVE ストローブ時のみ**。
```
$70=左7 $60=左6 $50=左5 $40=左4 $30=左3 $20=左2 $10=左1 $00=0
$F0=右1 $E0=右2 $D0=右3 $C0=右4 $B0=右5 $A0=右6 $90=右7 $80=右8
```

### HMOVE comb / タイミング
- HMOVE は**その行の HBLANK を 8 クロック延長**（LRHB デコード）→ 左 8px が黒くなり、不均一だと「櫛」状。
- **毎ライン HMOVE** で櫛を左 8px の**実線バー**に（Pitfall! 等）。**cycle 73–74 で HMOVE** すると黒線を消せる。
- HMOVE は **WSYNC 直後**に打つ。低レベルの定本は Andrew Towers "TIA Hardware Notes"。

### フレーム予算（確定値）
- **1 ライン = 228 カラークロック（HBLANK 68 + 可視 160）= 76 CPU サイクル**（3 クロック/サイクル）。
- NTSC **262** = VSYNC 3 / VBLANK 37 / 可視 **192** / Overscan 30。
- PAL・SECAM **312** = 3 / 45 / 可視 **228** / 36。
- **注意:** 実ゲームは逸脱する（NTSC 248–286 等）。ハーネスは「厳密 262」を決め打ちしない（範囲＋警告で）。

### 衝突レジスタ（CXxx）
8 個の読み取り専用、各 **D7/D6** に 2 ラッチ・**sticky**。判定は `BIT CXxx` → `BMI`(D7)/`BVS`(D6)。
```
CXM0P : D7=M0-P1 D6=M0-P0    CXM1P : D7=M1-P0 D6=M1-P1
CXP0FB: D7=P0-PF D6=P0-BL    CXP1FB: D7=P1-PF D6=P1-BL
CXM0FB: D7=M0-PF D6=M0-BL    CXM1FB: D7=M1-PF D6=M1-BL
CXBLPF: D7=BL-PF (D6 未使用)  CXPPMM: D7=P0-P1 D6=M0-M1
```
- **CXCLR**(書込strobe)=全衝突ラッチをクリア。 **HMCLR**(書込strobe)=動きレジスタ(HMxx)を 0 に（衝突とは別物）。

### 検証・テスト資料（カテゴリ 3）= NEW
- **Klaus Dormann 6502 functional test** — CPU 正しさのゴールド基準（成功で PC が既知アドレスに停止）。Gopher2600 も使用。
- ⭐ **Gopher2600 は録画/再生 + 組込み `regress`（回帰テスト DB）を持つ** — フレームハッシュのゴールデン画像差分が既製。欠落 D の本命。
- **差分テスト = Stella をオラクルに**（Gopher2600 作者も Stella を精度基準に使用）。
- **Visual6502 / perfect6502**（トランジスタ級）= 事実が割れた時の最終真実。
- 既存 TIA テスト ROM は HMOVE comb・late-HMOVE・HBLANK 中 RESPx 等の端を**網羅しない** → Stella＋実機キャプチャで校正。

### 参照（蒸留向けチートシート）
- 6502 命令/サイクル: masswerk `6502_instruction_set`（分岐: 非成立 2 / 成立同ページ 3 / ページ跨ぎ 4。`abs,X`/`abs,Y`/`(ind),Y` の跨ぎ +1）。
- TIA レジスタ表: Stella Programmer's Guide / Computer Archeology / NO\$ `2k6specs` / `vcs.h`。

### 要・手動確認（subagent は WebFetch 不可だった）
- masswerk の HMOVE 表のビット表記（要約器が符号ビットを落とした。Stella Guide 値で代用・複数ソースで裏取り済みだが念のため）。
- **Gopher2600 `regress` の正確なコマンド構文**（Stream A でも未回収。回帰層着手時に確認）。

---

## Stream A 結果（ハーネス構築）2026-06-09

Phase 1〜2 の実装仕様。最新 Gopher2600 **v0.56.0**（2026-06）、Apple Silicon 公式対応。

### Gopher2600 駆動（エンジン）
- **terminal コマンド:** `STEP` / `QUANTUM`（`CPU` または `CLOCK` ★CLOCK で**カラークロック単位ステップ**＝ビーム単位）/
  `SCANLINE` / `FRAME` / `PEEK` / `POKE` / `CPU` `RAM` `TIA` `RIOT` `TV`（各サブシステム表示）/ `WATCH`（読/書でhalt、シンボル可）/
  `BREAK` / `TRAP`（変化でhalt）/ `REWIND` / `SCRIPT`（記録・再生）/ `ONSTEP`・`ONHALT`・`ONTRACE`（毎回自動実行）。
  起動スクリプトは config dir の **debuggerInit**。
- **Go API:** `hardware.VCS{ CPU, Mem, RIOT, TIA, Input, TV, Clock }`。
  ビーム位置 `vcs.TV.GetCoords() → coords{Frame, Scanline, Clock}`。
  ステップ `vcs.Step(onColorClock func(bool)error)` / `vcs.RunForFrameCount(n, ...)`。
  save/restore `Snapshot()/Plumb()`（**TV ビーム状態は含まない**点に注意）。
- **外部駆動:** `debugger/terminal.Terminal` を自前実装（非対話）＋ `ReadEvents.PushedFunction` /
  `PushedFunctionImmediate` にクロージャを流し込み、エミュループと同期して状態取得・コマンド実行。
- **フレームバッファ:** `PixelRenderer` を実装し `tv.AddPixelRenderer`。`SetPixels` で 2D ビットマップ、
  `television/colourgen` で col-lum→RGB、可視幅 ~160 → `image.RGBA` で 1x キャプチャ。
- **ビルド(macOS):** `brew install sdl2`、Go（最小版は go.mod 参照、wiki の 1.16 は古い）、`go build -tags=release .`。

### MCP サーバ（Go）
- **公式 `github.com/modelcontextprotocol/go-sdk` を採用**（stdio・型付きツール・struct から JSON schema 自動生成）。
  `mcp-go` は HTTP/SSE が要る時のみ。
- パターン: `mcp.NewServer` → `mcp.AddTool[In,Out](server, &Tool{...}, handler)` → `server.Run(ctx, &mcp.StdioTransport{})`。
  画面は `ImageContent{Data []byte, MIMEType:"image/png"}` で返す（SDK が base64 化）。

### 参考実装パターン（踏襲）
- ⭐ **action-returns-screenshot**（mcp-gameboy）: 全ツールが最新フレーム画像を返す → 「やったこと」と「結果の観測」を一体化。
- 低レベル駆動（Gopher2600 terminal/PushedFunction）を MCP ツールの背後に隠蔽し、構造化データ（hex 文字列・レジスタ）を返す。
- 細粒度ツール ＋ `execute_batch` 風のまとめ実行でラウンドトリップ削減（ViceMCP は ~10x）。
- `BREAK`/`WATCH`/`TRAP` を checkpoint 系ツールに対応。

### Stella（オラクル + 注釈スクショ）
- 1x 単発: `stella -snapsavedir DIR -sssingle -ss1x -snapname rom ROM`。
- `-dbg.script FILE`（読込順 `autoexec.script` → `<rom>.script` → `-dbg.script`）。
  `dump START [END] FLAGS`（**1=メモリ / 2=CPU / 4=入力**、加算で `7`=全部）。
- **Fixed Debug Colors:** `-tia.dbgcolors roygbp` = **P0=赤 / M0=橙 / P1=黄 / M1=緑 / PF=青 / BL=紫**（順序 P0,M0,P1,M1,PF,BL 固定）。

### 画像オーバーレイ
- **Go 内製**（`image`/`image/draw`/`image/png` ＋ `fogleman/gg` で線・テキスト）。**ImageMagick へシェルアウトしない**
  （外部依存・非決定性を避ける＝検証ハーネスでは重要）。160×192 は可読性のため整数倍（×3–4, nearest）に拡大して描画、
  1x 原本は判定の基準として保持。

### 要・確認（UNVERIFIED）
- `QUANTUM` の引数綴り（CPU/CLOCK/VIDEO）と `BREAK`/`TRAP` の条件文法 → in-app `HELP`。
- Gopher2600 最小 Go バージョン → `go.mod`。 / `regress` サブコマンド構文 → 回帰層着手時。
- Stella `dump` の正確な出力レイアウト、manpage の文言（オプション名・色順は裏取り済み、文言は未）。
