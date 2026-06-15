# Build-to-learn — 実ゲームを「読む」から「自分で書く」へ変える再利用テンプレ

> **これは叩き台＝再利用可能な方法論テンプレ**（どのセッションからでも参照可能）。`casebook.md`（実ゲームを*読んで*"状況→技"を学ぶ＝受動）の対になる**能動版**＝実ゲームのメカニクスを**1個ずつ自分で asm に再現し、実ROMに数値照合**して、「説明できる」を「書ける」に変える。初出＝Breakout（2026-06-15）。著述ループ `authoring-protocol.md` の実地版。

## いつ使うか
- casebook study で「自分の描画/実装 craft が弱い」と判明したとき（[[feedback-mastery-not-just-passing]]）。
- 紙の再構築でなく**手を動かして技を身体化**したいとき。狙い＝**小さな成功体験の積み上げ**で能力を底上げ。

## 前提＝3素材（すべて出典記録・[[feedback-provenance-always]]）
| 素材 | 役割 | 入手 | 検証 |
|---|---|---|---|
| 検証済み ROM | 挙動の ground-truth | atarimania（[[reference-atarimania-roms]]）／padはdd抽出 | md5 が正規版と一致 |
| 公式マニュアル | spec（仕様） | archive.org（PDF＋OCR `_djvu.txt`） | 公式版を primary（Sears等の別ブランドに注意） |
| 注釈付き逆アセン | impl（実装の答え） | AtariAge（Debro 等）／無ければ distella で自前 | **`dasm -f3`→実ROMとバイト完全一致**を確認 |
- マニュアルだけでは挙動を把握し切れない＝**必ず実プレイ観察を足す**（[[feedback-play-the-rom-not-just-manual]]）。

## Phase 0 — 念入り精査（書く前に必ず）
1. **マニュアル↔コード対応マップ**（`_casestudies/<game>/impl-map.ja.md`・クリーンルーム散文のみ）：マニュアル各節→逆アセンのルーチン/RAM/テーブルへ対応づけ。書式＝表（節｜振る舞い｜コード｜RAM）。
2. **ground-truth fixtures**（`_casestudies/<game>/fixtures.ja.md`）：実ROMから**数値を1回採取**して各段の照合基準に固定（色のTIA値・座標clock・走査線範囲・初期値）。採取は `peek`(RAM)／`read_row`／`read_tia_registers`。**判定は数値**（Iron rule 1）。

## 制作戦略＝ボトムアップ・ラダー（既定）
**display→静的要素→動く要素→入力→衝突→ゲーム状態** を1メカニクスずつ。常に動くROMが残り、各段で確実な数値合格＝成功体験を最大化。
- 代替：**B メカニクス先行**（対話コアが早いが難所が前倒し）／**C 並行2トラック**（描画＋物理サンドボックスを同時→統合）。Cの並行性は**高リスク段の "spike" 先行**として既定Aに取り込む（失敗案も技の知見として記録＝「ダメな方法も学び」）。

## 1段の回し方（毎段・小ステップ）
1. **DoD を数値で先に定義**（verification-first）＝「何が出れば合格か」を fixtures 基準で。scenario も先に書き `roms/<game>/scenarios/` に回帰として残す。
2. **sealed で自分で挑戦**（簡単な段）。難段は挑戦→詰まったら**逆アセンの方法を読んで技を学ぶ**→**自分で書く**（クリーンルーム＝転載しない）。
3. assemble（`assemble_and_load`）→実走→**数値照合**（`read_row`/`read_tia_registers`/`read_collisions`/`step_frame`/`set_input`）→**1コミット**（[[feedback-fine-grained-commits]]）。おかしければ**即 revert**（Iron rule 3）。
4. **差分を記録**（自分の方法 vs 逆アセンの方法）＝`diff-gaps.ja.md`＝能力ギャップ＝学び。

## エンジニアリングの作法
- fixtures に固定照合（主観で判定しない）／cycles は sim から（rule 2）／litmus 該当なら数値裏取り（rule 4）。
- risk-first：高リスク段は使い捨て spike で技を先に数値確定。
- 既存 harness 資産を再利用（新規実装を増やさない）：`assemble_and_load`/`load_rom`/`get_screen_annotated`/`read_row`/`read_tia_registers`/`read_collisions`/`step_frame`/`set_input`/`assert_line_budget`/`run_scenario`/`cmd/scenario`/`cmd/dissect`/distella。

## 成果物と配置
- **自作 ROM（作品）**：`roms/<game>/<game>.asm`＋`scenarios/*.json`（git管理）。段ごとに育てる。
- **study（非リポ）**：`reference/disassemblies/_casestudies/<game>/{impl-map,fixtures,diff-gaps}.ja.md`＋`manual/`。逆アセン原本＝`reference/disassemblies/<Game>_<author>/`。
- **昇格（完了後・出典付き・lint緑）**：`casebook.md`（状況→技エントリ）／新技あれば `design-principles.md`／`roms/EVALUATION.md`（"書けた"の採点）。`check_wiring`/`check_provenance` 緑・`CHANGELOG`・push/tagは確認。

## 複利（なぜ続けると効くか）
各ゲームの diff が casebook/design-principles を厚くし、次のゲームが楽になる。受動(casebook)＝技の地図、能動(build-to-learn)＝技の手。両輪で [[project-roadmap-to-pong-capstone]] の capstone（1枚画像→オリジナル制作）の土台を作る。

## 実例（worked example）
**Breakout（Atari 1978）** が初出。8段ラダー（安定枠→左右壁→6色ブロック壁[非対称PF]→スコア→ボール反射→パドル[容量読取]→ブロック衝突→ゲーム状態）。詳細＝承認プラン `~/.claude/plans/cheerful-noodling-origami.md` ＋ `_casestudies/breakout/`。
