# M3 「2ゾーン2要素」最小プロトタイプ設計（de-risk・実装はユーザー GO 待ち）

## なぜ（収束点）
[[research-w5-m3-codegen]]（ゾーン合成型の方針）と [[research-w9-real-kernel-patterns]]（実ゲームは「固定ループ断片＋WSYNC/HMOVE 境界縫合」・型I〜IV）が**同じ最大不確実性**に収束：
**「走査線ゾーンの境界で RESP 再配置＋HMOVE＋色/NUSIZ 切替を、76cy を割らず・HMOVE後24cy制約を守って吐けるか」**。
Pizza Boy 実機カーネル（`reference/pizza-boy/dissection.ja.md`：`RepoKernel` が各 multisprite Y端で RESP1 再ストローブ＝型III/IV）でも実証済の構造。これを **bespoke 生成＋数値検証**で再現＝bB を超える（[[project-deliverable-beyond-bb]]）第一歩。
**M3 全体を作る前に、最小の境界縫合 1 個だけを手書きで通す**＝リスクをここに閉じ込める。

## プロトの仕様（最小・2ゾーン×別要素構成）
NTSC 192 可視を 2 ゾーンに割る：
- **ゾーンA（scanline 0–95）**：PF 背景（repeat・1色）＋ **P0 を左寄り**（1色 COLUP0）。要素＝{PF, P0}。
- **境界（scanline 96 の縫合ブロック）**：**P0 を RESP0 で右寄りへ再配置**（DivideBy15＋FineAdjustTable→HMOVE）＋ **COLUP0 を別色へ**＋（任意）NUSIZ0 を 1x→2x。＝w9「型III/IV：RESP 付き2ライン縫合」。
- **ゾーンB（scanline 96–191）**：PF 背景（同）＋ 右寄り 2x の P0 ＋ **走査線ごとに COLUP0 を変える（縦多色 colorPerRow）**。要素＝{PF, P0(別位置/別NUSIZ/縦多色)}。
→ 「ゾーンで要素構成（位置・NUSIZ・色テーブル）が変わる」最小例。bB 固定カーネルでは不可能な自由度を1個だけ示す。

## 受け入れ基準（数値・ハーネス直結）
1. **`assert_line_budget`**：全 192 ライン、WSYNC 間隔が 76cy を超えない（境界ラインも）。超過ゼロが必須。
2. **HMOVE規律**：HMOVE は WSYNC 直後、HMxx 書込みは HMOVE 後 24cy 以内に行わない（`trace_clocks` で境界の HMOVE 前後を実測確認）。
3. **`read_row`/`get_screen_annotated`**：ゾーンA で P0 が左・ゾーンB で右＋2x、色がゾーンで切替＋ゾーンB は走査線ごとに色が変わる、を数値＋視覚で確認。
4. **`read_tia`**：境界後の P0 `HmovedPixel` が狙い位置（litmus 流儀＝目で数えない）。

## 実装手順（GO 後・小ステップ＝CLAUDE.md 鉄則）
1. `roms/proto/zone2.asm` を新規（litmus と同じ最小骨格＝VSYNC/VBLANK/192可視/overscan）。
2. ゾーンA ループ（PF+P0）を書く→`assemble_and_load`→`assert_line_budget`→緑を確認。
3. 境界縫合ブロックを足す（RESP0+HMOVE+色/NUSIZ）→`trace_clocks` で 76cy/24cy 規律を実測→緑。
4. ゾーンB ループ（再配置P0＋colorPerRow 色テーブル）→`assert_line_budget`→`get_screen_annotated` で視覚確認。
5. **これが通れば M3 の核（境界生成）が実証**＝以後 codegen はこの手書きパターンを「ゾーン記述→生成」するだけ（[[research-w5-m3-codegen]] のテンプレ穴埋め＋境界生成）。

## 失敗時の逃げ（型を落とす）
境界が 76cy に収まらなければ w9 の型を下げる：型IV(2ライン RESP)→型II(色/NUSIZ のみ・位置据置=1ライン)→型I(色のみ)。**位置再配置を諦めれば必ず収まる**＝段階的フォールバックを codegen の選択肢に持たせる根拠。

## スコープ外（このプロトでは扱わない）
P1 multisprite の flickersort・48px・ball/missile・スコア帯。それらは別プロトで個別に。まず**境界縫合1個**だけを潰す。
