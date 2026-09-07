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


# ★2026-09-05: the technique catalogue's table lists FEWER demo ROMs than roms/techniques/ holds.
#   Measured when the TIA-revision column went in: **18 rows against 31 ROMs**, and three of the
#   seven ROMs that turned out to be revision-dependent (`two_line_vdel`, `rpgmap`, `bitmap48`)
#   were among the thirteen with no row — so the new column was incomplete because the TABLE was.
#   ★★A doc that is silently less complete than the directory it describes is the shape this whole
#   script exists to catch, and it had a blind spot for it. The ceiling is the measured gap, so it
#   cannot grow without someone deciding to raise it.
TECHNIQUE_TABLE_GAP_CEILING = 13


def technique_table_gap():
    """Demo ROMs in roms/techniques/ that the catalogue table does not name."""
    readme = os.path.join(HARNESS, "docs", "techniques", "README.md")
    if not os.path.isfile(readme):
        return []
    text = open(readme, encoding="utf-8").read()
    listed = set(re.findall(r"roms/techniques/([a-z0-9_]+)\.asm", text))
    on_disk = {os.path.basename(p)[:-4]
               for p in glob.glob(os.path.join(HARNESS, "roms", "techniques", "*.asm"))}
    return sorted(on_disk - listed)


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


def tool_reachability():
    """Every internal/ package must be reachable, and every cmd/ must be documented.

    Two ways a tool stops being usable without stopping being green, both measured
    2026-08-15 on a tree where every cmd/ built and every gate passed:

      UNREACHABLE  internal/keyfit (502 lines) and internal/mixmatch (280) had no cmd/ and
                   no importer at all, while CLAUDE.md described them as two of the three
                   pillars of audio reproduction. Nothing could call them. This is the
                   dead-code-with-numbers family the project already knows: sprite.DigitFont
                   had its digit 9 upside down for months because its importer count was zero
                   and nobody could notice.
      UNDOCUMENTED cmd/ceiling, cmd/ingest, cmd/metamorphic, cmd/mine-invariants, cmd/motion
                   and cmd/refdiff built and ran but appeared nowhere in CLAUDE.md, the file
                   that decides what the author reaches for. Worse than missing: present, and
                   never thought of.

    Both are checkable by grep, which is the whole reason to check them.
    """
    out = []
    ipath = os.path.join(HARNESS, "internal")
    if os.path.isdir(ipath):
        for pkg in sorted(os.listdir(ipath)):
            if not os.path.isdir(os.path.join(ipath, pkg)):
                continue
            imp = 'atari2600-harness/internal/%s"' % pkg
            # Count importers OUTSIDE the package's own directory.
            reachable = False
            for f in glob.glob(os.path.join(HARNESS, "**", "*.go"), recursive=True):
                if os.sep + "Gopher2600" + os.sep in f:
                    continue
                if os.path.dirname(f) == os.path.join(ipath, pkg):
                    continue
                try:
                    if imp in open(f, encoding="utf-8", errors="ignore").read():
                        reachable = True
                        break
                except OSError:
                    pass
            if not reachable:
                out.append("internal/%s is imported by nothing — no cmd/, no MCP tool, no other "
                           "package. It cannot be called, so nothing it computes can be wrong in "
                           "a way anyone would see. Give it a cmd/, or delete it and say so."
                           % pkg)

    cpath = os.path.join(HARNESS, "cmd")
    claude = os.path.join(HARNESS, "CLAUDE.md")
    if os.path.isdir(cpath) and os.path.isfile(claude):
        text = open(claude, encoding="utf-8").read()
        for c in sorted(os.listdir(cpath)):
            if not os.path.isdir(os.path.join(cpath, c)):
                continue
            if ("cmd/" + c) not in text:
                out.append("cmd/%s exists and builds, but CLAUDE.md never names it — the author "
                           "cannot reach for a tool they do not know about." % c)
    return out


def scenario_check_docs():
    """Every scenario check must be named in docs/scenarios.md.

    ★2026-09-06. Two checks were added that day — `ram_budget` and `max_flicker_area` — and
    NEITHER appeared in any document. Measured at the time: the checks that already existed were
    named in four to eight places; the two new ones, in zero. An author reading `docs/scenarios.md`
    to see what a scenario can assert would not have learned they existed.

    This is a repeat. The 2026-08-15 sweep found six commands that built and ran and were not in
    `CLAUDE.md` — "あるのに思いつかない", present but unthinkable — and added the rule below for
    `cmd/`. The same failure came back one level down, in the scenario schema, because the rule was
    written for commands rather than for the general shape: a capability nobody can find is a
    capability nobody has. So this checks the schema too.

    The source of truth is the `json:"..."` tags on the `Checks` struct, which is what a scenario
    file is actually parsed against — not a hand-kept list that can drift from it.
    """
    src = os.path.join(HARNESS, "internal", "scenario", "scenario.go")
    doc = os.path.join(HARNESS, "docs", "scenarios.md")
    if not (os.path.isfile(src) and os.path.isfile(doc)):
        return ["internal/scenario/scenario.go or docs/scenarios.md is missing, so the scenario "
                "schema could not be checked against its documentation"]
    with open(src, encoding="utf-8") as f:
        text = f.read()
    m = re.search(r"type Checks struct \{(.*?)\n\}", text, re.S)
    if not m:
        return ["could not find `type Checks struct` in internal/scenario/scenario.go — this gate "
                "is looking at the wrong thing and would pass while checking nothing"]
    names = re.findall(r'json:"([a-z0-9_]+)', m.group(1))
    if not names:
        return ["`type Checks struct` yielded no json tags, so nothing was checked"]
    with open(doc, encoding="utf-8") as f:
        docs = f.read()
    missing = [n for n in names if n not in docs]
    if missing:
        return ["scenario check `%s` is in the schema and named nowhere in docs/scenarios.md — "
                "an author reading the format reference cannot discover it" % n for n in missing]
    print("scenario schema OK — all %d checks are named in docs/scenarios.md." % len(names))
    return design_rule_docs()


def design_rule_docs():
    """Every exported `pkg/design` rule must be named in a document.

    ★`pkg/design` is deliberately NOT called by the pipeline. `budget.go` says what it is for:
    「本関数は『作る前』の静的見積り」 — a static estimate BEFORE building, consulted at design
    time rather than run at test time. Measured 2026-09-06: 23 of its 26 exported functions have no
    non-test caller, and that is the intended shape, not a defect. (Two of the three that DO have
    callers were wired the same day, because they had runtime counterparts; the rest do not.)

    ★★Which makes findability the entire question. A design-time rule nobody can find is worse off
    than an unreached runtime rule: nothing will ever fail to remind you it exists. Measured the same
    day: 22 of the 26 were named in a document and **four were not** — `AsymPFLineFits`,
    `AsymPFReachableX`, `HMoveReachable`, `FitsText`. The convention in `design-principles.md` is a
    trailing `→ design.Fn` pointer on the principle the function computes; the four now have one.

    ★★★This is the same rule as the scenario-schema check above and as the `cmd/` check below,
    which is the point: the 2026-08-15 sweep wrote that rule for commands, and it came back twice in
    one day at other levels. **A capability nobody can find is a capability nobody has**, wherever it
    lives.
    """
    pkg = os.path.join(HARNESS, "pkg", "design")
    if not os.path.isdir(pkg):
        return ["pkg/design is missing, so the design rules could not be checked against docs"]
    names = []
    for fn in sorted(os.listdir(pkg)):
        if not fn.endswith(".go") or fn.endswith("_test.go"):
            continue
        with open(os.path.join(pkg, fn), encoding="utf-8") as f:
            names += re.findall(r"^func ([A-Z]\w*)\(", f.read(), re.M)
    if not names:
        return ["pkg/design yielded no exported functions, so nothing was checked"]
    corpus = ""
    for d in sorted(glob.glob(os.path.join(HARNESS, "docs", "*.md"))) + [os.path.join(HARNESS, "CLAUDE.md")]:
        if os.path.isfile(d):
            with open(d, encoding="utf-8") as f:
                corpus += f.read()
    missing = [n for n in names if n not in corpus]
    if missing:
        return ["design rule `design.%s` is exported and named in no document — pkg/design is a "
                "DESIGN-TIME library, so nothing will ever fail to remind an author it exists" % n
                for n in missing]
    print("design rules OK — all %d exported pkg/design functions are named in a document." % len(names))
    return []



def dangling_doc_references():
    """(1)'s missing direction: does every referenced .md actually EXIST?

    The orphan check above asks "has every doc got a reader". It never asked the
    reverse — "has every reference got a target" — so a `.md` name that no file
    answers to passes every gate here. Seven of them did, quietly, for months:
    `2600-constants.md`, `improvement-roadmap.md`, `hardening-roadmap.md`,
    `MEMORY.md`, `feedback_asm_architecture.md`, `project_pong_status.md`,
    `feedback_dev_process.md`. Two of those are roadmaps whose content was folded
    into `capability-gap-audit.md` and whose names were left pointing at nothing;
    the reader following one learns the document is missing, not that it moved.

    ★A gate that only looks one way is the same defect this repository spent a day
    finding in a text differ — it reported deletions and had no path at all for
    insertions, so it reported none. Direction is a property of an instrument, and
    an instrument only answers the direction it was built to ask.

    Found by the mailing-list distillation (helper-3), who arrived at it by
    misreporting a broken link, chasing the mistake, and noticing the gate could
    not have caught either the false one or the real ones.
    """
    # Search the umbrella, not just this repo: docs legitimately point at ../STATUS.md,
    # ../OVERVIEW.md and the sibling repos. Counting those as missing was the first
    # version's own error.
    umbrella = os.path.dirname(HARNESS)
    # ★Is the umbrella actually here? The pre-push hook runs the gates in a throwaway
    # worktree OUTSIDE the repository, where the sibling repos do not exist — so a
    # reference to `sandbox/EVALUATION.md` is unresolvable there and perfectly fine in a
    # real checkout. Judging it either way from inside the worktree would be a guess.
    #
    # ★★This is the same mistake as the CI revert on 2026-09-06, one day later and one
    # scale smaller: a check written where the environment happened to be complete, run
    # where it is not. The gate now asks whether it can see the umbrella before it decides
    # anything about the umbrella.
    siblings_present = all(os.path.isdir(os.path.join(umbrella, d)) for d in ("roms", "sandbox"))
    have = set()
    for root, dirs, files in os.walk(umbrella):
        dirs[:] = [d for d in dirs if d not in ("Gopher2600", ".git", "node_modules")]
        for f in files:
            if f.endswith(".md"):
                have.add(f)

    # Capture the WHOLE path, so `x.ja.md` yields `x.ja.md` and not the tail `ja.md`. The
    # first version split on the last dot and reported fifteen references to a file called
    # `ja.md`; the check was wrong, not the tree — this repository's most-repeated lesson,
    # met again inside the check that came out of it.
    # Only references a reader would FOLLOW: a path (`docs/x.md`, `../STATUS.md`) or a
    # markdown link. A bare file name in prose is a mention, not a link.
    #
    # ★The first version took every `*.md` token and reported 23 problems where 7 exist and
    # only ONE is a defect. Its false positives are worth naming, because each is a different
    # way of being right: `x.ja.md` split at the last dot and became fifteen references to a
    # file called `ja.md`; `~/.claude/plans/…` names a real file this gate cannot see; *"the
    # FORMER improvement-roadmap.md"* is prose about a document that was deliberately folded
    # away; and *"memory `feedback_dev_process.md`"* says where to look in the word before it.
    # **A reference is not a link, and a gate that cannot tell them apart reports history and
    # prose as rot.** Narrowed to paths and links, the tree has one.
    ref = re.compile(r"(?:\]\(|`|\s)((?:\.\.?/|[\w-]+/)[\w./-]*\.md)")
    problems = []
    for src in sorted(glob.glob(os.path.join(HARNESS, "docs", "**", "*.md"), recursive=True)):
        if os.path.basename(src).endswith(".ja.md"):
            continue
        for m in ref.finditer(open(src, encoding="utf-8", errors="ignore").read()):
            path = m.group(1)
            if path.startswith("~") or path.startswith("/") or path.endswith(".ja.md"):
                continue
            # A reference OUT of this repo can only be judged where the siblings exist.
            if not siblings_present and (path.startswith("../") or path.split("/")[0] in
                                         ("roms", "sandbox", "reference", "library")):
                continue
            if os.path.basename(path) not in have:
                problems.append("%s references `%s`, which does not exist anywhere under the umbrella"
                                % (os.path.relpath(src, HARNESS), path))
    return sorted(set(problems))


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

    gap = technique_table_gap()
    if len(gap) > TECHNIQUE_TABLE_GAP_CEILING:
        print(f"TECHNIQUE TABLE GAP — {len(gap)} demo ROMs have no row in docs/techniques/README.md, "
              f"over the ceiling of {TECHNIQUE_TABLE_GAP_CEILING}:")
        for g in gap:
            print("  ✗ roms/techniques/" + g + ".asm")
        print("\nA ROM nobody can find in the catalogue is a technique nobody will use, and any")
        print("column added to that table (hardware scope, status) is incomplete by exactly this")
        print("much. Add a row, or lower the ceiling if you removed one.")
        sys.exit(1)
    if len(gap) < TECHNIQUE_TABLE_GAP_CEILING:
        print(f"note: the technique-table gap is down to {len(gap)} (ceiling {TECHNIQUE_TABLE_GAP_CEILING}) "
              f"— lower the ceiling so it cannot drift back up")
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

    tool_problems = tool_reachability()
    if tool_problems:
        print("TOOLS THAT CANNOT BE REACHED OR CANNOT BE FOUND:")
        for t in tool_problems:
            print("  ✗", t)
        print("\nA tool nobody can call is the dead-code-with-numbers family; a tool nobody knows")
        print("about is worse, because it is present and never thought of.")
        sys.exit(1)

    gate_problems = gate_wiring()
    sc_problems = scenario_check_docs()
    if sc_problems:
        print("SCENARIO SCHEMA — a check exists that no document names:")
        for p in sc_problems:
            print("  ✗", p)
        print("\nThe 2026-08-15 sweep found six commands nobody could think of because CLAUDE.md")
        print("did not list them. This is the same defect one level down, in the scenario schema.")
        sys.exit(1)

    dangling = dangling_doc_references()
    if dangling:
        print("WIRING CHECK FAILED (dangling doc reference):")
        for d in dangling:
            print("  \u2717 " + d)
        print("  A reference with no target teaches the reader the document is missing, not that it "
              "moved. Point it at the file that absorbed it, or delete the sentence.")
        sys.exit(1)

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
    n_int = len([d for d in os.listdir(os.path.join(HARNESS, "internal"))
                 if os.path.isdir(os.path.join(HARNESS, "internal", d))])
    n_cmd = len([d for d in os.listdir(os.path.join(HARNESS, "cmd"))
                 if os.path.isdir(os.path.join(HARNESS, "cmd", d))])
    print(f"tools OK — all {n_int} internal packages are imported by something, and all {n_cmd} "
          f"commands are named in CLAUDE.md.")


if __name__ == "__main__":
    main()
