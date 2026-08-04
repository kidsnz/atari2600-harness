#!/usr/bin/env python3
"""check_wiring.py — 知識の「持ち腐れ」を構造的に防ぐ配線チェック。

検査は2本立て:
  ① docs/*.md が入口（CLAUDE.md routing / authoring-protocol.md）から辿れるか。
  ② roms/litmus/*.asm が「回帰の網」に入っているか＝シナリオ or コード/テストから参照されているか。
  ③ docs/verified-coverage.md が名指しする ROM が実在し、②の網に入っているか。

②の理由（2026-07-30 実測）: litmus は 91 本あり、42 本にシナリオが無い。うち 40 本は Go テストが
直接使っているので健全だが、**2 本（cb_roll / litmus_color）はどこからも参照されていなかった**。
cb_roll はその間に主張が腐った——「cb_clean と画素完全一致」と書いてあったが実測では 192 行中 1 行違う。
検証用 ROM が誰にも実行されないのは、検証していないのと同じ。

ルール（[[knowledge-activation-architecture]]）：harness の知識は**入口から辿れて初めて機能する**。
`docs/*.md`（公開・英語）が **CLAUDE.md の routing** か **docs/authoring-protocol.md（制作の背骨）** の
どちらかから参照されていなければ「孤立 doc ＝発火しない持ち腐れ予備軍」とみなして CI を落とす。
＝「知識を足したら必ず入口に繋ぐ」を機械強制する（provenance/traps lint と同型）。

使い方:
    cd harness && python3 scripts/check_wiring.py
"""
import glob
import os
import re
import sys

HARNESS = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))

# 入口ファイル（ここから辿れれば「配線済み」）。
ENTRYPOINTS = ["CLAUDE.md", os.path.join("docs", "authoring-protocol.md")]
# 入口自身・索引/履歴系は対象外（参照する側）。
SKIP = {"authoring-protocol.md", "mining-digest.md", "provenance.md"}


# 検証 ROM を置くディレクトリ。roms/carts は「TIA でなくカートリッジ形式を検査する」別コーパス
# （Stella TIA オラクルの対象外＝roms/carts/README.md に理由）。別コーパスにしたことで配線検査から
# 外れては本末転倒なので、ここで同じ網に入れる。
ROM_DIRS = ["litmus", "carts"]


def litmus_orphans():
    """回帰の網の外にいる検証 ROM を返す。網＝シナリオ or コード/テスト/スクリプトからの参照。"""
    rom_dirs = [os.path.join(HARNESS, "roms", d) for d in ROM_DIRS]
    rom_dirs = [d for d in rom_dirs if os.path.isdir(d)]
    if not rom_dirs:
        return [], 0, 0, 0

    # 参照側テキストを一度だけ集める（ROM 自身と CHANGELOG は除く＝履歴は「使っている」ではない）。
    # THIS FILE IS EXCLUDED, and the reason is a defect it had for ten minutes: the
    # docstring above names cb_roll and litmus_color as examples, the scan read every
    # *.py including this one, and both ROMs came back "referenced". A checker that
    # satisfies itself by explaining what it checks reports 0 orphans forever.
    me = os.path.abspath(__file__)
    refs = ""
    for pat in ("**/*.go", "**/*.py", "**/*.sh", "**/*.json"):
        for f in glob.glob(os.path.join(HARNESS, pat), recursive=True):
            if os.path.abspath(f) == me:
                continue
            if os.sep + "roms" + os.sep + "litmus" + os.sep in f and not f.endswith(".json"):
                continue
            try:
                refs += open(f, encoding="utf-8", errors="ignore").read()
            except OSError:
                pass

    orphans, via_scenario, via_code, total = [], 0, 0, 0
    for d in rom_dirs:
        rel = "roms/" + os.path.basename(d) + "/"
        asms = sorted(glob.glob(os.path.join(d, "*.asm")))
        total += len(asms)
        for f in asms:
            base = os.path.basename(f)[:-4]
            short = base[len("litmus_"):] if base.startswith("litmus_") else base
            if os.path.isfile(os.path.join(d, "scenarios", base + ".json")) or \
               os.path.isfile(os.path.join(d, "scenarios", short + ".json")):
                via_scenario += 1
                continue
            if base in refs:
                via_code += 1
                continue
            orphans.append(rel + base + ".asm")
    return orphans, total, via_scenario, via_code


def coverage_doc_roms():
    """docs/verified-coverage.md が名指しする litmus ROM を返す（表の 2 列目）。

    この表は冒頭で「全項目が litmus ROM で検証され、シナリオで回帰固定され、push ごとに CI で
    走る」と断言している。実測（2026-07-30）では 35 本中 7 本がシナリオではなく Go テストで
    守られていた——無防備な項目は 0 だったが、断言のほうが事実と違っていた。ここが検査するのは
    「表が名指しした ROM が実在し、②の網に入っていること」＝将来この表に、誰も走らせない ROM が
    書き足されたら落ちる。
    """
    path = os.path.join(HARNESS, "docs", "verified-coverage.md")
    if not os.path.isfile(path):
        return []
    out = []
    for line in open(path, encoding="utf-8"):
        if not line.startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 3:
            continue
        m = re.fullmatch(r"`([a-z0-9_]+)`", cells[1])
        if m:
            out.append(m.group(1))
    return sorted(set(out))


def main():
    entry_text = ""
    for e in ENTRYPOINTS:
        p = os.path.join(HARNESS, e)
        if os.path.isfile(p):
            entry_text += open(p, encoding="utf-8").read()

    orphans = []
    for p in sorted(glob.glob(os.path.join(HARNESS, "docs", "*.md"))):
        fn = os.path.basename(p)
        if fn.endswith(".ja.md") or fn in SKIP:
            continue
        # ファイル名（拡張子有/無）で参照されているか。
        if fn in entry_text or fn[:-3] in entry_text:
            continue
        orphans.append("docs/" + fn)

    if orphans:
        print("WIRING ORPHANS — these docs aren't reachable from CLAUDE.md routing or the authoring protocol:")
        for o in orphans:
            print("  ✗", o)
        print("\nWire each into CLAUDE.md's routing table (or link from docs/authoring-protocol.md) so the")
        print("knowledge actually fires at authoring time. (rule: knowledge-activation-architecture)")
        sys.exit(1)
    lit, total, via_scenario, via_code = litmus_orphans()
    if lit:
        print("LITMUS ORPHANS — these verification ROMs are run by nothing (no scenario, no test, no tool):")
        for o in lit:
            print("  ✗", o)
        print("\nA litmus nobody runs is not verification. Give it a scenario in roms/litmus/scenarios/,")
        print("or a Go test that names it. Measured cost of skipping this: cb_roll sat unreferenced and its")
        print("header's claim (\"pixel-identical to cb_clean\") drifted to false — 1 of 192 rows differ.")
        sys.exit(1)

    named = coverage_doc_roms()
    missing = [r for r in named if not os.path.isfile(os.path.join(HARNESS, "roms", "litmus", r + ".asm"))]
    unnetted = [r for r in named if "roms/litmus/" + r + ".asm" in lit]
    if missing or unnetted:
        print("VERIFIED-COVERAGE ORPHANS — docs/verified-coverage.md names ROMs that do not exist or that "
              "nothing runs:")
        for r in missing:
            print("  ✗", r, "(no such ROM)")
        for r in unnetted:
            print("  ✗", r, "(exists, but nothing runs it)")
        print("\nThe table's own opening says every behaviour in it is locked for regression. A row whose")
        print("ROM nobody runs makes that sentence false for the reader who trusts it.")
        sys.exit(1)

    print("wiring OK — every docs/*.md is reachable from an entrypoint (no orphaned knowledge).")
    print(f"litmus OK — {total} ROMs in the regression net: {via_scenario} via scenario, "
          f"{via_code} via a test or tool, 0 orphaned.")
    print(f"verified-coverage OK — all {len(named)} ROMs the table names exist and are in the net.")


if __name__ == "__main__":
    main()
