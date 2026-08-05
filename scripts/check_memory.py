#!/usr/bin/env python3
"""check_memory.py — structural check of the persistent memory (~/.claude/.../memory/).

Why it is needed (measured 2026-07-30): `docs/` had check_wiring and check_provenance, and both
of them actually found holes. **memory had no check at all.** 38 files / 1,786 lines were being
loaded every session with nobody able to verify them. Tidying (consolidating, rewriting) should
start here, and rewriting with no gate in place is exactly the "whatever is outside the net
breaks" failure this project had spent a night stamping out.

What is checked:
  (1) Does each [[wiki-link]] point at a memory file that exists **or** a harness/docs file?
  (2) Does every file have **exactly one** line in MEMORY.md, and does each index line point at
      a file that exists?
  (3) Is the frontmatter complete (name / description / metadata.type) and name == filename?
  (4) The per-file line ceiling (to prevent re-bloat).
  (6) Do the [[link]]s from harness/docs and CLAUDE.md point at live memory files?
     — measured 2026-07-30: right after one memory file was moved to _archive/, the
       [[feedback-authoring-loop-system]] link at docs/authoring-protocol.md:5 was left dangling.
       The first version missed it because it looked only at memory->docs and not the reverse
       direction. A one-way check is half a check.
  (5) ★ Are the canonical rule files still holding onto their INCIDENTS — that is, do they cite a
      minimum number of concrete things (filenames, test names, numbers)?
     — what makes a rule bite is the concrete example, not the abstraction. Tidying tends to leave
       the headings and drop the cases. This stops that mechanically.

Usage:
    cd harness && python3 scripts/check_memory.py
    MEMORY_DIR=/path/to/memory python3 scripts/check_memory.py
On a machine with no memory directory this skips (exit 0).
"""
import os
import re
import sys

DEFAULT = os.path.expanduser(
    "~/.claude/projects/-Users-shinji-Documents-2D-260609-atari2600-dev/memory")
MEM = os.environ.get("MEMORY_DIR", DEFAULT)

# The per-file ceiling. Measured (2026-07-30), the largest was project-next-session-todo.md=321
# and the next feedback-verification-standard.md=197. The ceiling is not drawn between those two
# but at "a length that still reads as a canonical rule".
MAX_LINES = 250

# Explicit exceptions to the ceiling. **You do not get one without writing the reason**
# (no silent loosening). Debt is kept greppable rather than hidden — the same convention as
# `@rom-write-ok` on the roms side.
OVERSIZE_EXEMPT = {
    # 2026-07-30: project-next-session-todo.md, the only exception there was, is resolved
    # (322 -> 34 lines). The 17 history sections were stored in
    # memory/_archive/project-next-session-todo-history.md and the 3 sections that were not in
    # STATUS.md were moved into STATUS.md (all 20 sections were accounted for one by one; none
    # went missing). The description, which had run to 8,361 characters, was put back to one line.
    # The table is left empty deliberately: it shows where to write the reason next time one
    # exceeds the ceiling.
}

# The canonical rules. This is the side that must NOT be thinned out.
CANONICAL = [
    "feedback-verification-standard.md",
    "feedback-goal-standard.md",
    "feedback-execution-discipline.md",
    "feedback-work-tracking.md",
]
MIN_CONCRETE = 3  # the minimum number of concrete things a canonical rule must cite

# A "concrete thing" = something that really exists in the repo, a test name, or a measured value.
# The metric that fails a canonical rule which has become pure abstraction.
#
# NOTE: the unit alternatives below are deliberately Japanese. This regex is the one thing in
# scripts/ that reads a NON-repo tree: the memory files under ~/.claude/.../memory/ are the
# author's own notes and are written in Japanese by design. Translating these alternatives would
# stop the check finding measured values and silently weaken gate (5).
CONCRETE = re.compile(
    r"`[^`]*\.(?:asm|go|py|json|md|bin)`"      # a filename
    r"|`?Test[A-Z][A-Za-z0-9_]+`?"              # a Go test name
    r"|\b[0-9a-f]{7}\b"                         # a commit hash
    r"|\b\d+\s*(?:件|本|行|回|cy|サイクル|px)\b"  # a measured value with its unit
)
LINK = re.compile(r"\[\[([^\]]+)\]\]")
HARNESS_DOCS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs")


def INDEX_ENTRY(fname):
    """One index entry = `- [Title](file.md)` at the start of a line. A different shape from a
    reference in the prose."""
    return re.compile(r"^- \[[^\]]*\]\(" + re.escape(fname) + r"\)", re.M)

FM_NAME = re.compile(r"^name:\s*(\S+)\s*$", re.M)
FM_DESC = re.compile(r"^description:\s*\S", re.M)
FM_TYPE = re.compile(r"^\s*type:\s*(user|feedback|project|reference)\s*$", re.M)


def inbound(stem, files, index_text, repo):
    """Count and return the places that point at stem, excluding the MEMORY.md index line
    (every file has exactly one, so it carries no information).

    A measurement on 2026-07-30 showed this was needed: the inbound references to one memory file
    were reported as "one line in one canonical file" and it was about to be archived on that
    basis, when in fact 3 places referenced it. Unless you **count them all mechanically** before
    consolidating, the references nobody re-pointed are left dangling.
    """
    hits = []
    pats = [re.compile(r"\[\[" + re.escape(stem) + r"\]\]"),
            re.compile(r"\(" + re.escape(stem) + r"\.md\)")]
    srcs = [(os.path.join(MEM, f), "memory/" + f) for f in files if f[:-3] != stem]
    srcs.append((os.path.join(repo, "CLAUDE.md"), "harness/CLAUDE.md"))
    for base, _, fs in os.walk(os.path.join(repo, "docs")):
        for x in fs:
            if x.endswith(".md"):
                srcs.append((os.path.join(base, x), os.path.relpath(os.path.join(base, x), repo)))
    for path, label in srcs:
        try:
            body = open(path, encoding="utf-8").read()
        except OSError:
            continue
        n = sum(len(pat.findall(body)) for pat in pats)
        if n:
            hits.append(f"{label}x{n}" if n > 1 else label)
    # In the index, count only cross-references in the prose (every file has an entry line, so
    # those carry zero information).
    body = index_text
    n = sum(len(pat.findall(body)) for pat in pats) - len(INDEX_ENTRY(stem + ".md").findall(body))
    if n > 0:
        hits.append(f"MEMORY.md(prose)x{n}")
    return hits


def report():
    """The table used to pick consolidation candidates. Zero inbound references only means a file
    COULD possibly be dropped; that it holds no unique information has to be confirmed separately,
    one file at a time (do not make the count a target)."""
    files = sorted(f for f in os.listdir(MEM) if f.endswith(".md") and f != "MEMORY.md")
    index = open(os.path.join(MEM, "MEMORY.md"), encoding="utf-8").read()
    repo = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    rows = []
    for f in files:
        lines = open(os.path.join(MEM, f), encoding="utf-8").read().count("\n") + 1
        rows.append((len(inbound(f[:-3], files, index, repo)), lines, f,
                     inbound(f[:-3], files, index, repo)))
    rows.sort()
    print(f"{'refs':>4} {'lines':>5}  file")
    for n, lines, f, hits in rows:
        mark = "  ← 0 inbound" if n == 0 else ""
        print(f"{n:>4} {lines:>5}  {f}{mark}")
        if hits:
            print(f"            {', '.join(hits)}")
    print(f"\n{len(files)} files; {sum(1 for r in rows if r[0]==0)} with no inbound reference")
    return 0


def main():
    if "--report" in sys.argv:
        return report()
    if not os.path.isdir(MEM):
        print(f"memory not present ({MEM}) — skipped")
        return 0
    index_path = os.path.join(MEM, "MEMORY.md")
    files = sorted(f for f in os.listdir(MEM)
                   if f.endswith(".md") and f != "MEMORY.md")
    if not files:
        print("no memory files — skipped")
        return 0

    errors = []
    index = open(index_path, encoding="utf-8").read() if os.path.isfile(index_path) else ""
    if not index:
        errors.append("MEMORY.md is missing or empty — the index is what gets loaded every session")

    stems = {f[:-3] for f in files}
    concrete_counts = {}

    for f in files:
        path = os.path.join(MEM, f)
        text = open(path, encoding="utf-8").read()
        lines = text.count("\n") + 1

        # ③ frontmatter
        m = FM_NAME.search(text)
        if not m:
            errors.append(f"{f}: no `name:` in frontmatter")
        elif m.group(1) != f[:-3]:
            errors.append(f"{f}: frontmatter name `{m.group(1)}` does not match the filename")
        if not FM_DESC.search(text):
            errors.append(f"{f}: no `description:` — recall picks files by this line")
        if not FM_TYPE.search(text):
            errors.append(f"{f}: no `metadata.type:` of user/feedback/project/reference")

        # (4) the ceiling
        if lines > MAX_LINES and f not in OVERSIZE_EXEMPT:
            errors.append(f"{f}: {lines} lines, over the {MAX_LINES} cap — split it rather than let "
                          f"one file accrete (one fact per file). If it must stay, add it to "
                          f"OVERSIZE_EXEMPT WITH A MEASURED REASON.")
        if f in OVERSIZE_EXEMPT and lines <= MAX_LINES:
            errors.append(f"{f}: is exempt from the {MAX_LINES}-line cap but is now {lines} lines — "
                          f"the debt was paid, so drop the exemption rather than leave a licence "
                          f"nobody needs")

        # (1) links. harness/docs is as legitimate a destination as another memory file, so this
        # does not forbid it — it checks that the target exists in ONE OF THE TWO (measured:
        # feedback-verification-standard used [[known-traps]] to point at docs/known-traps.md).
        for target in LINK.findall(text):
            if target in stems:
                continue
            if any(os.path.isfile(os.path.join(HARNESS_DOCS, sub, target + ".md"))
                   for sub in ("", "techniques")):
                continue
            errors.append(f"{f}: [[{target}]] resolves to neither a memory nor a harness doc")

        # (2) the index. Count **index entry lines** only. Cross-references in the prose
        # (e.g. "canonical = [x](x.md)") are a legitimate way to write and are not counted.
        n = len(INDEX_ENTRY(f).findall(index))
        if n == 0:
            errors.append(f"{f}: absent from MEMORY.md — a memory nobody indexes is not recalled")
        elif n > 1:
            errors.append(f"{f}: appears {n} times in MEMORY.md; the index holds one line per memory")

        concrete_counts[f] = len(set(CONCRETE.findall(text)))

    # (6) the reverse direction: links from the harness docs / CLAUDE.md that point at memory
    repo = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    sources = [os.path.join(repo, "CLAUDE.md")]
    for base, _, fs in os.walk(os.path.join(repo, "docs")):
        sources += [os.path.join(base, x) for x in fs if x.endswith(".md")]
    doc_stems = set()
    for base, _, fs in os.walk(os.path.join(repo, "docs")):
        doc_stems |= {x[:-3] for x in fs if x.endswith(".md")}
    for src in sources:
        if not os.path.isfile(src):
            continue
        try:
            body = open(src, encoding="utf-8").read()
        except OSError:
            continue
        for target in set(LINK.findall(body)):
            if target in stems or target in doc_stems:
                continue
            rel = os.path.relpath(src, repo)
            arch = os.path.isfile(os.path.join(MEM, "_archive", target + ".md"))
            why = "it was archived" if arch else "no such memory or doc"
            errors.append(f"{rel}: [[{target}]] resolves to nothing ({why})")

    # does the index point at a file that does not exist?
    for ref in re.findall(r"\(([a-z0-9-]+\.md)\)", index):
        if ref not in files:
            errors.append(f"MEMORY.md links to {ref}, which does not exist")

    # (5) concreteness of the canonical rules
    for c in CANONICAL:
        if c not in files:
            errors.append(f"canonical rule file {c} is missing")
            continue
        if concrete_counts[c] < MIN_CONCRETE:
            errors.append(
                f"{c}: only {concrete_counts[c]} concrete citations (files, test names, commit "
                f"hashes, measured values); a rule bites because of the incident behind it, and a "
                f"tidy-up that keeps the heading and drops the evidence silently stops it working")

    if errors:
        print("MEMORY CHECK FAILED:")
        for e in errors:
            print("  ✗", e)
        return 1

    total = sum(open(os.path.join(MEM, f), encoding="utf-8").read().count("\n") + 1 for f in files)
    if OVERSIZE_EXEMPT:
        print(f"note: {len(OVERSIZE_EXEMPT)} file(s) exempt from the {MAX_LINES}-line cap: "
              f"{', '.join(sorted(OVERSIZE_EXEMPT))}")
    print(f"memory OK — {len(files)} files / {total} lines, index complete, all [[links]] resolve, "
          f"{len(CANONICAL)} canonical rules each citing >= {MIN_CONCRETE} concrete artefacts "
          f"({', '.join(f'{c.split(chr(46))[0][:22]}:{concrete_counts[c]}' for c in CANONICAL)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
