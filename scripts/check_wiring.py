#!/usr/bin/env python3
"""check_wiring.py — wiring check that structurally prevents knowledge from rotting unused.

The inspection has four parts:
  (1) Is every docs/*.md reachable from an entrypoint (CLAUDE.md routing / authoring-protocol.md)?
  (2) Is every roms/litmus/*.asm inside the "regression net" = referenced by a scenario or by code/tests?
  (3) Do the ROMs that docs/verified-coverage.md names exist, and are they inside the net of (2)?
  (4) Does every scripts/check_*.py have a row in docs/gate-ledger.md, and does that row's
      "Runs in" column match where the gate is ACTUALLY invoked (ci.yml / pre-push)?

Reason for (4) (measured 2026-08-13): six gates ran on this repository and nothing recorded what
any of them had ever caught, so a gate that earns its place could not be told from one that only
looks like it does. Building that ledger immediately found that **check_memory.py was run by
nothing** — 275 lines, real catches to its name, absent from both ci.yml and the pre-push hook,
and its only mention outside its own source was a line in an ARCHIVED status file. A gate nobody
runs is the same defect as a litmus nobody runs, one level up, and prose alone would have gone on
claiming otherwise. So the ledger's "Runs in" column is checked against the two files that do the
running, rather than believed.

Reason for (2) (measured 2026-07-30): there are 91 litmus ROMs, 42 with no scenario. 40 of those are
used directly by Go tests, so they are healthy, but **2 of them (cb_roll / litmus_color) were referenced
by nothing at all**. In the meantime cb_roll's claim rotted — its header said "pixel-identical to
cb_clean", but measurement shows 1 of 192 rows differs. A verification ROM that nobody runs is the
same as not verifying.

Rule ([[knowledge-activation-architecture]]): harness knowledge **only functions once it is reachable
from an entrypoint**. If a `docs/*.md` (public, English) is referenced by neither **CLAUDE.md's
routing** nor **docs/authoring-protocol.md (the authoring backbone)**, it is treated as an "orphaned
doc = knowledge that will never fire" and CI fails.
= Machine-enforces "when you add knowledge, always wire it to an entrypoint" (same shape as the
provenance/traps lints).

Usage:
    cd harness && python3 scripts/check_wiring.py
"""
import glob
import os
import re
import sys

HARNESS = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))

# Entrypoint files (reachable from here = "wired").
ENTRYPOINTS = ["CLAUDE.md", os.path.join("docs", "authoring-protocol.md")]
# The entrypoints themselves and index/history docs are exempt (they are the referencing side).
SKIP = {"authoring-protocol.md", "mining-digest.md", "provenance.md"}


# Directories holding verification ROMs. roms/carts is a separate corpus that "inspects cartridge
# formats rather than the TIA" (outside the Stella TIA oracle's scope = reason in roms/carts/README.md).
# It would defeat the purpose if making it a separate corpus dropped it out of the wiring check, so it
# goes into the same net here.
ROM_DIRS = ["litmus", "carts"]


def litmus_orphans():
    """Return the verification ROMs outside the regression net. Net = a scenario, or a reference from code/tests/scripts."""
    rom_dirs = [os.path.join(HARNESS, "roms", d) for d in ROM_DIRS]
    rom_dirs = [d for d in rom_dirs if os.path.isdir(d)]
    if not rom_dirs:
        return [], 0, 0, 0

    # Collect the referencing-side text once (excluding the ROMs themselves and the CHANGELOG =
    # history is not "using").
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
    """Return the litmus ROMs that docs/verified-coverage.md names (2nd column of the table).

    That table opens by asserting "every entry is verified by a litmus ROM, pinned for regression
    by a scenario, and runs in CI on every push". Measured (2026-07-30): 7 of the 35 were guarded
    by a Go test rather than a scenario — zero entries were unguarded, but the assertion itself
    did not match the facts. What this inspects is "the ROMs the table names exist and are inside
    the net of (2)" = it fails if a ROM that nobody runs is ever added to this table in the future.
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


def gate_wiring():
    """Check every scripts/check_*.py against its row in docs/gate-ledger.md.

    Returns a list of complaint strings; empty means the ledger matches reality.

    "Runs in" must be a list of BACKTICKED tokens drawn from `ci`, `pre-push`, `none`; any prose
    beside them is ignored. Free-text parsing was tried first and was wrong within the hour: the
    reason written next to check_memory's row is "absent in CI, skips there", and a bare search
    for the word matched it, so a gate that runs nowhere near CI was read as claiming CI. A
    column that a checker has to interpret is a column the checker will interpret wrongly.
    """
    ledger = os.path.join(HARNESS, "docs", "gate-ledger.md")
    gates = sorted(os.path.basename(p) for p in glob.glob(os.path.join(HARNESS, "scripts", "check_*.py")))
    if not gates:
        return []
    if not os.path.isfile(ledger):
        return ["docs/gate-ledger.md is missing, but %d gates exist: %s"
                % (len(gates), ", ".join(gates))]

    text = open(ledger, encoding="utf-8").read()
    ci = os.path.join(HARNESS, ".github", "workflows", "ci.yml")
    hook = os.path.join(HARNESS, "scripts", "git-hooks", "pre-push")
    ci_text = open(ci, encoding="utf-8").read() if os.path.isfile(ci) else ""
    hook_text = open(hook, encoding="utf-8").read() if os.path.isfile(hook) else ""

    # Row shape: | `check_x.py` | forbids | runs in | cost | ... |
    rows = {}
    for line in text.splitlines():
        if not line.startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 3:
            continue
        m = re.fullmatch(r"`(check_\w+\.py)`", cells[0])
        if m:
            rows[m.group(1)] = cells[2]

    out = []
    for g in gates:
        if g not in rows:
            out.append("%s has no row in docs/gate-ledger.md — a gate with no record of what it "
                       "has caught cannot be told from one that has caught nothing" % g)
            continue
        tokens = {t.lower() for t in re.findall(r"`([^`]+)`", rows[g])}
        unknown = tokens - {"ci", "pre-push", "none"}
        if unknown:
            out.append("%s: the ledger's 'Runs in' has backticked token(s) %s; only `ci`, "
                       "`pre-push` and `none` are meaningful" % (g, ", ".join(sorted(unknown))))
            continue
        claims_ci, claims_hook, claims_none = "ci" in tokens, "pre-push" in tokens, "none" in tokens
        if not (claims_ci or claims_hook or claims_none):
            out.append("%s: the ledger's 'Runs in' says %r, which backticks neither `ci`, "
                       "`pre-push` nor `none`" % (g, rows[g]))
            continue
        really_ci, really_hook = g in ci_text, g in hook_text
        if claims_ci and not really_ci:
            out.append("%s: the ledger says it runs in CI, but .github/workflows/ci.yml never "
                       "names it" % g)
        if claims_hook and not really_hook:
            out.append("%s: the ledger says it runs in the pre-push hook, but "
                       "scripts/git-hooks/pre-push never names it" % g)
        if not claims_ci and really_ci:
            out.append("%s: ci.yml runs it, but the ledger's 'Runs in' does not say `ci`" % g)
        if not claims_hook and really_hook:
            out.append("%s: the pre-push hook runs it, but the ledger's 'Runs in' does not say "
                       "`pre-push`" % g)
        if claims_none and (really_ci or really_hook):
            out.append("%s: the ledger says `none`, but it is actually invoked" % g)
        if not (really_ci or really_hook) and not claims_none:
            out.append("%s: nothing invokes it — not ci.yml, not the pre-push hook. Wire it, or "
                       "say `none` in the ledger with the reason" % g)
    return out


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
        # Is it referenced by file name (with or without the extension)?
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

    gate_problems = gate_wiring()
    if gate_problems:
        print("GATE LEDGER — docs/gate-ledger.md does not match what actually runs:")
        for p in gate_problems:
            print("  ✗", p)
        print("\nA gate nobody runs is the same defect as a litmus nobody runs. check_memory.py sat")
        print("unwired from the day it was written until 2026-08-13, with real catches to its name.")
        sys.exit(1)

    print("wiring OK — every docs/*.md is reachable from an entrypoint (no orphaned knowledge).")
    print(f"litmus OK — {total} ROMs in the regression net: {via_scenario} via scenario, "
          f"{via_code} via a test or tool, 0 orphaned.")
    print(f"verified-coverage OK — all {len(named)} ROMs the table names exist and are in the net.")
    n_gates = len(glob.glob(os.path.join(HARNESS, "scripts", "check_*.py")))
    print(f"gate ledger OK — all {n_gates} gates have a row, and every 'Runs in' matches ci.yml "
          f"and the pre-push hook.")


if __name__ == "__main__":
    main()
