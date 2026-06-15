#!/usr/bin/env python3
"""check_wiring.py — 知識の「持ち腐れ」を構造的に防ぐ配線チェック。

ルール（[[knowledge-activation-architecture]]）：harness の知識は**入口から辿れて初めて機能する**。
`docs/*.md`（公開・英語）が **CLAUDE.md の routing** か **docs/authoring-protocol.md（制作の背骨）** の
どちらかから参照されていなければ「孤立 doc ＝発火しない持ち腐れ予備軍」とみなして CI を落とす。
＝「知識を足したら必ず入口に繋ぐ」を機械強制する（provenance/traps lint と同型）。

使い方:
    cd harness && python3 scripts/check_wiring.py
"""
import glob
import os
import sys

HARNESS = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))

# 入口ファイル（ここから辿れれば「配線済み」）。
ENTRYPOINTS = ["CLAUDE.md", os.path.join("docs", "authoring-protocol.md")]
# 入口自身・索引/履歴系は対象外（参照する側）。
SKIP = {"authoring-protocol.md", "mining-digest.md", "provenance.md"}


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
    print("wiring OK — every docs/*.md is reachable from an entrypoint (no orphaned knowledge).")


if __name__ == "__main__":
    main()
