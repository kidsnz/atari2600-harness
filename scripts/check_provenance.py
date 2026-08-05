#!/usr/bin/env python3
"""check_provenance.py — check that every harness element records its ORIGIN (CI / local lint).

Rule ([[feedback-provenance-always]]): every element put into the harness must record, as a
source citation, which original it came from. The point is that when something misbehaves during
real authoring you can **go back to the original source and look it up again**.

What is checked, and what passes (containing any one provenance marker is a pass):
  - docs/techniques/<name>.md (excluding README/roadmap): an external reference (Source/Learned
    from/topic/<id>/8bitworkshop/Spice/Davie etc.) or our own verification basis
    (litmus_*/Hardware basis/Foundation).
  - pkg/design/<name>.go (excluding _test): a mining number / a design-principles reference /
    a 〔source〕 comment.
  - docs/design-principles.md: at least a set number of 〔...〕 source tags.

Usage:
    cd harness && python3 scripts/check_provenance.py        # check everything (exit 1 if any are missing)
    cd harness && python3 scripts/check_provenance.py --list  # regenerate docs/provenance.md (element -> origin index)
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
HARNESS = os.path.normpath(os.path.join(HERE, ".."))

# Markers that count as provenance (case-insensitive): an external reference, our own
# verification basis, or the citation bracket.
#
# `出典` ("source") is kept as a LEGACY accept, not as a live spelling. The repo is
# English-only and every citation now reads `Source:` or 〔…〕, so nothing currently passes
# on this alternative alone — it stays because dropping a marker can only turn a passing
# file red, and a gate that goes red on a rename teaches people to delete the gate.
MARKERS = re.compile(
    r"source:|出典|learned from|reference:|based on|derived from|〔|"
    r"litmus_|hardware basis|hardware-verified|foundation:|"
    r"reference/|topic/\d|8bitworkshop|spice|davie|staugas|whitehead|hugg",
    re.IGNORECASE,
)

SKIP_DOCS = {"README.md", "roadmap.md", "roadmap.ja.md", "README.ja.md"}


def techniques_docs():
    d = os.path.join(HARNESS, "docs", "techniques")
    for fn in sorted(os.listdir(d)):
        # Only the published canonical copy (the English .md) is checked. *.ja.md are
        # gitignored local translations = out of scope.
        if fn.endswith(".md") and not fn.endswith(".ja.md") and fn not in SKIP_DOCS:
            yield os.path.join(d, fn)


def pkg_design_files():
    d = os.path.join(HARNESS, "pkg", "design")
    for fn in sorted(os.listdir(d)):
        if fn.endswith(".go") and not fn.endswith("_test.go"):
            yield os.path.join(d, fn)


def has_marker(path):
    try:
        with open(path, encoding="utf-8") as f:
            return bool(MARKERS.search(f.read()))
    except OSError:
        return False


def rel(p):
    return os.path.relpath(p, HARNESS)


def first_marker_line(path):
    """Return the first line that states provenance (the short source sentence, for the index)."""
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                if MARKERS.search(line):
                    return line.strip().lstrip("# ").strip()
    except OSError:
        pass
    return ""



# ─────────────────────────────────────────────────────────────────────────────
# A citation nobody can follow is not provenance.
#
# The marker check above asks whether a doc SAYS where something came from. It
# never asked whether the thing it names is there. Two independent readers — an
# agent and the main session — both concluded on 2026-08-04 that
# `docs/techniques/asymmetric-pf-score.md`'s "verified in-game (PONG
# `sandbox/practice/pong/steps/...`)" pointed at nothing, and both were WRONG:
# `sandbox/` and `reference/` live in the UMBRELLA directory one level above
# `harness/`, not inside it, and documentation paths are written relative to the
# umbrella. The evidence was there; the readers looked in one of the two roots.
#
# So this resolves every cited path against BOTH roots, and reports the ones that
# resolve against neither. That answers the question provenance exists for —
# "when this breaks, can I get back to the original?" — instead of the question
# the marker check answers, which is "does this file contain the word Source:".
KNOWN_ABSENT = {
    # path -> why it is cited but not present. Each entry is a claim someone must
    # eventually make true or retract; the point of listing them is that they are
    # COUNTED rather than invisible.
    "reference/disassemblies/_casestudies/breakout/":
        "casebook.md says the 8-rung Breakout was completed and cites this as its "
        "output; only _casestudies/outlaw exists. The raw material is there "
        "(reference/disassemblies/Breakout_debro) but the case study is not.",
    "roms/breakout/":
        "casebook.md cites the self-built Breakout ROM and its per-rung snapshots. Absent.",
    "reference/disassemblies/_casestudies/fishing-derby/manual/":
        "casebook.md cites the archive.org manual scans. Absent; "
        "sandbox/studies/fishing-derby and reference/disassemblies/Fishing_Derby_debro exist.",
    "reference/disassemblies/_casestudies/fishing-derby/diff-gaps.ja.md":
        "casebook.md cites this as the detailed ledger of Claude-vs-master gaps. Absent.",
    "docs/2600-constants.md":
        "resources.md offers it as a distillation target. Never created; the values "
        "live in CLAUDE.md instead.",
}
# MCP protocol method names, not paths.
NOT_PATHS = {"tools/call", "tools/list"}

CITE_ROOTS = ("sandbox/", "roms/", "reference/", "pkg/", "internal/", "cmd/",
              "scripts/", "docs/", "third_party/", "tools/")
CITE_RE = re.compile(r"`([A-Za-z0-9_./*-]+)`")
FILE_EXT = re.compile(r"\.(asm|bin|go|md|json|py|txt|png|jsonl|sh)$")


UMBRELLA = os.path.dirname(HARNESS)

# Roots that live in the UMBRELLA, not in this repository. `reference/` is the
# clean-room source material and `sandbox/` is a separate git repo of authored ROMs;
# neither is part of a checkout of the harness, so CI has no way to resolve a citation
# into them.
UMBRELLA_ROOTS = ("sandbox/", "reference/")


def umbrella_present():
    """Whether the umbrella tree is on disk. Env var forces it off, for testing the
    CI path without moving the user's files."""
    if os.environ.get("HARNESS_NO_UMBRELLA"):
        return False
    return any(os.path.isdir(os.path.join(UMBRELLA, r.rstrip("/"))) for r in UMBRELLA_ROOTS)


def _resolves(path):
    """True if `path` exists under the harness OR under the umbrella above it.

    A `.bin` is a BUILD PRODUCT and is gitignored, so it is absent from any fresh
    checkout and present on any machine that has run the build — which makes its
    existence a fact about the working copy, not about the citation. What is
    followable is its SOURCE, so a cited `.bin` is resolved through the `.asm` beside
    it. (Found the third time this check turned CI red for citing something real:
    first the umbrella, then the commercial corpus, then this.)"""
    import glob as _glob
    if path.endswith(".bin"):
        path = path[:-4] + ".asm"
    for root in (HARNESS, UMBRELLA):
        full = os.path.join(root, path)
        if "*" in path:
            if _glob.glob(full):
                return True
        elif os.path.exists(full):
            return True
    return False


def cited_paths():
    """Yield (doc, path) for every repo-relative path cited in backticks in docs/."""
    import glob as _glob
    docs = os.path.join(HARNESS, "docs")
    for fn in sorted(_glob.glob(os.path.join(docs, "**", "*.md"), recursive=True)):
        if fn.endswith(".ja.md"):
            continue
        try:
            body = open(fn, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        for m in CITE_RE.finditer(body):
            path = m.group(1)
            if not path.startswith(CITE_ROOTS) or "..." in path or path in NOT_PATHS:
                continue
            # `pkg/sprite.DigitFont` is a Go symbol, not a file.
            base = path.rsplit("/", 1)[-1]
            if "." in base and not FILE_EXT.search(base):
                continue
            yield rel(fn), path


def check_citations():
    """Return (unresolved, stale_known, skipped) — cited-but-absent, entries in
    KNOWN_ABSENT that now resolve, and the count of citations not checked because the
    umbrella is absent.

    CI CHECKS OUT THIS REPOSITORY ALONE. `sandbox/` and `reference/` are the umbrella's,
    so on a CI runner every citation into them resolves to nothing — not because the
    trail is broken but because the tree it leads into was never fetched. The first
    version of this check did not know that and turned GitHub Actions red on 11 real,
    followable citations. Checking them there is not a stricter check, it is a
    different and wrong one.

    The count of what went unchecked is RETURNED rather than swallowed, so a run that
    verified 756 citations and a run that verified 69 fewer do not print the same
    thing."""
    have_umbrella = umbrella_present()
    unresolved = []
    skipped = 0
    for doc, path in cited_paths():
        if _resolves(path):
            continue
        if not have_umbrella:
            # WITHOUT THE UMBRELLA, "does not resolve" carries no information: the
            # path may be perfectly good and simply live in a tree this checkout does
            # not contain. Keying on the root prefix is not enough — `scripts/` exists
            # on BOTH sides, and `docs/techniques/asymmetric-pf-score.md` cites the
            # umbrella's `scripts/pong_font_gen_pf.py`. So when the umbrella is
            # absent, anything the harness alone cannot resolve is counted and passed
            # over rather than called broken.
            skipped += 1
            continue
        if path in KNOWN_ABSENT:
            continue
        unresolved.append((doc, path))
    stale = []
    if have_umbrella:
        # With no umbrella there is no way to tell a stale entry from an unfetchable
        # one, and claiming staleness would delete a real entry from KNOWN_ABSENT.
        stale = [p for p in KNOWN_ABSENT if _resolves(p)]
    return unresolved, stale, skipped

def write_list():
    """docs/provenance.md = the consolidated element -> origin list (generated, never hand-written;
    the index you fall back to in the worst case)."""
    out = ["# Provenance map — every harness element → its origin\n",
           "Auto-generated by `scripts/check_provenance.py --list`. A low-traffic reference: open it only\n"
           "when something built on a technique/rule misbehaves and you need to go back to the original source.\n",
           "Recording is the strict part (CI-enforced); this list is just the consolidated lookup.\n",
           "\n## Techniques (`docs/techniques/<name>.md` → source)\n",
           "| technique | recorded source |\n|---|---|\n"]
    for p in techniques_docs():
        out.append("| `%s` | %s |\n" % (os.path.basename(p), first_marker_line(p) or "—"))
    out.append("\n## Other provenance layers\n")
    out.append("- **Design rules** → `docs/design-principles.md` (each rule ends with `〔source〕`).\n")
    out.append("- **`pkg/design` functions** → source comment in each `pkg/design/*.go`.\n")
    out.append("- **Mined AtariAge threads** → `docs/mining-digest.md` (topic_id + URL → what it feeds).\n")
    out.append("- **Raw per-thread notes** → `reference/atariage/<id>-*/notes.ja.md` (provenance, not committed).\n")
    open(os.path.join(HARNESS, "docs", "provenance.md"), "w", encoding="utf-8").write("".join(out))
    print("wrote docs/provenance.md")


def main():
    if "--list" in sys.argv:
        write_list()
        return
    missing = []
    for p in techniques_docs():
        if not has_marker(p):
            missing.append(rel(p))
    for p in pkg_design_files():
        if not has_marker(p):
            missing.append(rel(p))

    # design-principles is checked coarsely, by counting source tags.
    dp = os.path.join(HARNESS, "docs", "design-principles.md")
    if os.path.isfile(dp):
        with open(dp, encoding="utf-8") as f:
            tags = f.read().count("〔")
        if tags < 20:
            missing.append("docs/design-principles.md (citations 〔〕=%d, want >=20)" % tags)

    # Every cited path must be followable — from the harness or from the umbrella.
    unresolved, stale, skipped = check_citations()
    if skipped:
        print("citation check: %d citation(s) NOT checked — they do not resolve inside this "
              "repository and the umbrella tree that holds them is not present, so this run "
              "cannot tell a broken trail from an unfetched one" % skipped)
    for doc, path in unresolved:
        missing.append("%s cites `%s`, which exists under neither the harness nor the "
                       "umbrella above it" % (doc, path))
    for path in stale:
        missing.append("KNOWN_ABSENT lists `%s`, but it now resolves — delete the entry "
                       "so the list keeps meaning something" % path)
    if KNOWN_ABSENT:
        print("citations known to be absent (%d) — each is a claim to make true or retract:"
              % len(KNOWN_ABSENT))
        for k in sorted(KNOWN_ABSENT):
            print("  ·", k)

    if missing:
        print("PROVENANCE MISSING — these need a Source:/〔…〕/litmus_ citation:")
        for m in missing:
            print("  ✗", m)
        print("\nrule: [[feedback-provenance-always]] — every harness element records its origin.")
        sys.exit(1)
    print("provenance OK — every technique doc / pkg/design / design-principles cites a source, "
          "and every cited path resolves from the harness or the umbrella.")


if __name__ == "__main__":
    main()
