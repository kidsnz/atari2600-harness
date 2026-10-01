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
#
# The English phrases must start a word: `(?<![\w-])`, not `\b`, because `\b` sits between a
# hyphen and a letter. Without it, "re-derived from scratch" counted as "derived from" — a
# sentence that cites nothing — and `--list` would put it in the source column of seven technique
# rows (bitmap48, multicolor48, nusiz-shaping, rts-dispatch, score-kernel, text12, text24; found
# 2026-10-01). Paths, symbols and names are left unguarded: `gen_litmus_*`, `andrew-davie-*` still
# name the litmus fixture and the person, so a prefix there does not change what is cited.
MARKERS = re.compile(
    r"(?<![\w-])(?:source:|learned from|reference:|based on|derived from|"
    r"hardware basis|hardware-verified|foundation:)|"
    r"出典|〔|litmus_|"
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


# ★2026-09-06: a citation that resolves nowhere is not provenance.
#
# This gate's own stated purpose is that "when something misbehaves during real authoring you can
# go back to the original source and look it up again". It checked that cited PATHS resolve and
# never checked the one thing most of the newer citations are made of: a stella-list message
# number. Measured when the check was written — 41 unique `YYYYMM/msgNNNNN` references across 111
# occurrences, of which **40 resolve and one does not**: `199712/msg00194`, cited from
# `CHANGELOG.md` and `docs/known-traps.md`, in a month that holds 102 messages ending at
# `msg00101`. Found by the mailing-list distillation (helper-2); the sweep re-run here.
#
# The archive lives in the umbrella (`reference/`), so a CI checkout of this repository alone
# cannot resolve anything — in that case the check SAYS it skipped rather than passing silently.
MSGREF_RE = re.compile(r"\b((?:19|20)\d{4})/msg(\d{5})\b")


def check_message_references():
    """Every stella-list message number cited anywhere must name a file that exists."""
    archive = os.path.join(UMBRELLA, "reference", "stella-list")
    # `umbrella_present()` is how the rest of this gate simulates a CI checkout
    # (`HARNESS_NO_UMBRELLA=1`); honour it here too, or the skip path is untestable.
    if not umbrella_present() or not os.path.isdir(archive):
        print("message references: SKIPPED — %s is not present, so no citation could be "
              "resolved. This is expected in CI, which clones only this repository; it is not "
              "a pass." % archive)
        return []

    seen = {}
    excused = set()
    for root, dirs, files in os.walk(HARNESS):
        dirs[:] = [d for d in dirs if d not in ("Gopher2600", ".git", "build", "bin")]
        for f in files:
            if not f.endswith((".md", ".py", ".go", ".asm", ".txt")):
                continue
            path = os.path.join(root, f)
            # This file quotes the broken reference as its own worked example; a gate naming
            # its own test case is not making a citation.
            if os.path.abspath(path) == os.path.abspath(__file__):
                continue
            try:
                with open(path, encoding="utf-8", errors="ignore") as fh:
                    text = fh.read()
            except OSError:
                continue
            for m in MSGREF_RE.finditer(text):
                # ★A reference may be named in order to say it is WRONG — the tree carries one
                # such, and explaining it requires printing it. The escape is the word
                # "unresolved" on the same line, and the count of escapes is printed below so
                # they cannot quietly accumulate.
                line = text[text.rfind("\n", 0, m.start()) + 1:
                            (text.find("\n", m.end()) + 1 or len(text)) - 1]
                if "unresolved" in line.lower():
                    excused.add(m.group(0))
                    continue
                seen.setdefault(m.group(0), set()).add(os.path.relpath(path, HARNESS))

    bad = []
    for ref, where in sorted(seen.items()):
        ym, num = ref.split("/msg")
        if os.path.isfile(os.path.join(archive, ym, "msg%s.html" % num)):
            continue
        month = os.path.join(archive, ym)
        if os.path.isdir(month):
            nums = sorted(int(n[3:8]) for n in os.listdir(month)
                          if n.startswith("msg") and n.endswith(".html"))
            hint = ("that month holds %d messages, the last is msg%05d"
                    % (len(nums), nums[-1])) if nums else "that month is empty"
        else:
            hint = "there is no %s directory in the archive" % ym
        bad.append("%s cites stella-list `%s`, which does not exist — %s. A citation that "
                   "resolves nowhere is not provenance: this gate exists so a claim can be "
                   "looked up again. Find the right number, or say in the text that the "
                   "reference is unresolved and what was searched"
                   % (", ".join(sorted(where)), ref, hint))
    if not bad:
        note = ""
        if excused:
            note = (" (+%d marked unresolved: %s)"
                    % (len(excused), ", ".join(sorted(excused))))
        print("message references: %d cited stella-list messages, all resolve%s"
              % (len(seen), note))
    return bad


def _resolves(path):
    """True if `path` exists under the harness OR under the umbrella above it.

    A `.bin` is a BUILD PRODUCT and is gitignored, so it is absent from any fresh
    checkout and present on any machine that has run the build — which makes its
    existence a fact about the working copy, not about the citation. What is
    followable is its SOURCE, so a cited `.bin` is resolved through the `.asm` beside
    it. (Found the third time this check turned CI red for citing something real:
    first the umbrella, then the commercial corpus, then this.)"""
    import glob as _glob
    candidates = [path]
    if path.endswith(".bin"):
        # A .bin we BUILD is gitignored, so resolve it through its .asm. A .bin we did
        # NOT build — a commercial ROM image under `sandbox/studies/` or `reference/` —
        # has no .asm beside it and never will: not having Atari's source is the whole
        # point of the clean-room line, so rewriting to .asm asks for a file whose
        # absence is deliberate. Try the .bin as cited first, then its source.
        # (Fourth time this check turned CI red for citing something real: the umbrella,
        # the commercial corpus, the build-product rewrite — and now the rewrite applied
        # to a ROM that is not a build product. 2026-09-04.)
        candidates.append(path[:-4] + ".asm")
    for cand in candidates:
        for root in (HARNESS, UMBRELLA):
            full = os.path.join(root, cand)
            if "*" in cand:
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
            # umbrella's `sandbox/practice/pong/tools/pong_font_gen_pf.py`. So when the umbrella is
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

# The prose sections that follow the generated index. They were first written into
# docs/provenance.md by hand, which `--list` then silently deleted on every regeneration
# (found 2026-10-01 by diffing the generator's output against the committed file). They
# live here now, verbatim, so the file stays generated end to end. Edit them HERE.
HAND_SECTIONS = """

## How this repository cites the mailing list — and why a checker cannot follow it

**Six shapes are in use. That is five too many, and it is a harness problem, not a tool problem.**
Measured 2026-09-06, when a checker built to verify that quoted text really appears in the message it
names reported thirteen wrong sources; a human opened all thirteen and **every one was correct here**.
The tool was not weak — it was reading prose that cites in six different ways:

| # | shape | example |
|---|---|---|
| 1 | full | `〔200405/msg00275〕` |
| 2 | **continuation** — the year-month carries over from earlier in the sentence | `〔msg00286〕` |
| 3 | **month + thread name, no number at all** | ``stella-list `200011` (`more-keyboard-nonsense`)`` |
| 4 | range | `〔199803/msg00196–00199〕` |
| 5 | comma list | `〔199703/msg00258, 199703/msg00204〕` |
| 6 | **a quote from source, not from the list**, sitting beside a message number | ``hardware/memory/vcs/tia.go … *"left over from the address"*`` |

Shapes 2, 3 and 6 are the ones that mislead: a checker looking for the nearest `YYYYMM/msgNNNNN`
attaches the quote to a number that belongs to a different claim. **`check_provenance.py` does not have
this problem** — it only asks whether a cited message EXISTS, which shape 1 and 4 and 5 all satisfy
and 2, 3 and 6 simply do not trigger. The problem appears the moment anyone asks the stronger question,
*does this quote appear in that message*, which is the question worth asking.

**The rule for new prose:** cite in shape 1, `〔YYYYMM/msgNNNNN〕`, immediately after the closing quote.
A range or comma list is fine when the claim genuinely spans messages. **Do not use a bare continuation,
do not cite a thread by name without a number, and do not put a source-code quotation next to a message
number.** ★And nothing goes inside the quotation marks that the author did not write — no `[sic]`, no
bracketed completions, no silently corrected typos. Measured the same day: a note that wrote
*"the cable networks will tole[rate]"* for *"…will tolerate flicker"* was missed by a verbatim matcher,
and three notes had quietly fixed `kernal`, `positionining` and `yor`. **A corrected quote is a quote
the person did not say, and it is also a quote `rg -F` will never find again.** Put the clarification
in the surrounding prose instead, where it belongs.

★★**And nothing comes OUT of a quotation either — least of all a hedge.** Measured 2026-09-06, after
the typo sweep above had been declared finished: a second pass looking for *omissions* rather than
substitutions found **45 unmarked deletions**, and five of them had removed the author's own qualifier —
`(I think)`, `perhaps`, `(apparently)`, `(or my own)`. **A quotation with its hedge cut reads as an
assertion the person did not make**, and unlike a typo it cannot be caught by reading: the sentence is
grammatical, plausible and wrong about exactly one thing, which is how sure its author was. This
repository already treats claim strength as part of a fact — that is what the `📖` legend in
`fundamentals-audit.md` is for, marking what is documented but unmeasured. **The same discipline has to
survive the trip through a quotation.** Marked elision (`…`, `[...]`) is fine and 146 instances of it
were correctly left alone; what is not fine is silence.

★★★**A third way to make a quotation lie: stop it one clause early.** Found the same day, 31 cases
where a quotation ends mid-sentence and the original continues with a *but*. Cutting a quote is a
normal thing to do — what is not normal is cutting it where the next clause **reverses** it:

> *"…on the vintage ROMs were active high"* — and the original goes on, *"**But they are active low on
> standard EPROMs.**"* 〔`200207/msg00165`〕

**Read the quotation alone and you learn the opposite of the fact.** Another ended at *"adding another
table is impossible"* where the author's next line reports having found a working example — a solved
problem quoted as an open one. **When the continuation reverses the claim, extend the quotation.** That
is not in tension with the rule above: the rule forbids adding text the author did not write, and
restoring more of what they DID write is the same rule pointed the other way. Where the continuation
merely adds detail or social chat, leave it — and where it strengthens rather than reverses, a note
outside the quotation is enough. **The whole of this class cannot be automated**: only the *but* case
is detectable, and `tole[rate]`'s missing sentence — the strongest one in that thread — was not a
reversal and would never have been flagged.

★★★★**A fourth way, and the simplest: wear the marker without being one.** A paraphrase set in
`>` or `*"…"*` claims to be verbatim, and nothing in it can be repaired by restoring words — the whole
line is the writer's sentence. The fix is not to edit the quotation but to **stop it claiming to be
one**: drop the marker and let it be prose. This is the failure the other three are variations of, and
it is the only one where a matcher's "does not appear in the source" verdict is exactly right.

★★★★★The way these classes were found is worth as much as the fixes. The first sweep's detector compared
word against word and **had no path at all for reporting a deletion** — not an oversight but a shape:
it could not see omissions, so it reported none, and "243 typos" was the count of one class presented as
the count of all of them. It took another session pointing at it from outside. **A tool's outline is
invisible to the person holding it** — measured across nine wrong results in one day, the tool's own
shape was recovered exactly once, and that once was when someone else named it.

★★★★★★★★And the sharpest demonstration came from a session that refused to grade itself with
somebody else's instrument. Told to fix nineteen listed omissions, it **wrote a second detector** rather
than trust the list, and the first run reported **zero** across nine hundred notes. Before reporting
that, it fed the detector sixty artificial deletions: **none of them fired.** The comparison had been
reading its diff opcodes backwards — counting text present in the note and absent from the source
(fabrication) instead of the reverse (omission). Corrected, the same control fired 57 of 60, and the
real scan found **twenty-one more** than the list it had been handed. **A zero without a negative
control is not a measurement**, and that one was a sentence away from being reported with confidence.

Found by the mailing-list distillation (helper-2, who counted the shapes after their own tool was
fooled by five of them).


## Attribution without a message id is where misattribution lives (2026-09-07)

Three quotations here named the wrong person. **All three attribute by NAME with no message id**, and
of the 45 quotations that carry an id, **none is wrong**. That is the finding: the defect is not
carelessness about who said things, it is the habit of writing a name without the number that would
have checked it.

| page | said | actually |
|---|---|---|
| `design-principles.md:283` | Eric Ball | **Glenn Saunders** 〔`200401/msg00063`〕 — Ball ANSWERED it in `msg00064`, and the reply quoted the question |
| `design-principles.md:302` | Erik Mooney | **Billy Eno** 〔`200208/msg00131`〕 — whose post opens *"Erik, I searched the archives…"* |
| `sprite-placement.md:31` | Erik Mooney | **KirkIsrael** 〔`200207/msg00046`〕 |

★**Three different mechanisms, and none of them is inattention:**

1. **A reply quotes the question.** Search the archive for the sentence and you land on `msg00064`,
   whose byline is the replier's. The quoted text is `>`-marked in the source and the marker is lost
   the moment the sentence is copied out.
2. **A vocative read as a byline.** *"Erik, I searched the archives…"* is Billy Eno writing TO Erik.
   The name nearest the quotation was the addressee.
3. **A quotation without a `>`.** Bob Colbert repeats KirkIsrael's sentence unmarked in `msg00047`, so
   a matcher — and a reader — takes it for his own words.

★★The third one also lost the claim's shape. The original is a beginner asking *"since you can't read
the **Horizontal Positions** directly, **right?**"*; this file had it as *"Erik Mooney said it plainly …
you can't read the horizontal positions directly."* **A tentative question became a flat assertion by
an expert** — the hedge-cutting failure recorded above, this time carrying a name with it.

★★★**The rule, and it is cheap:** name a person and give the message id. The id is what makes the
attribution checkable; a name alone asserts something no reader can verify and no gate can catch.
Found by the mailing-list distillation (helper-3), whose detector had a 73% false-positive rate on the
eleven it flagged — six were work titles read as people — and whose **population** (26 quotations
attributed by name with no id) is the number worth keeping.

## The archive's attachments were never fetched, and nothing here depends on them (2026-09-07)

Roughly 1,200 messages in the stella-list archive carry attachments — ROMs, sources, zips. **They
were deliberately not fetched** (author's decision, 2026-09-04). This section is the measurement
that makes that decision safe to leave standing rather than a hope.

**104** stella-list message ids are cited across `docs/`, `internal/` and `CHANGELOG.md`. All 104
bodies are present in the local archive. **19 of them carry an attachment.** Each of the 19 was
opened and read:

| What the attachment is | Count |
|---|---|
| A binary, zip, or `.asm` offered **beside** the argument (`Attachment: lmnf12.bin`, `push.asm`, `songplay.zip`) | 18 |
| A body that **is** a uuencoded blob (`199702/msg00017`, *"section 1 of uuencode 5.25 of file say.bin"*) | 1 |

**In none of the 19 does this repository quote the attachment.** The eighteen are quoted from body
text — Stolberg's *"I need to revise my 5 pixel delay theory again"*, Mooney's *"different results
for both (d+3) positioning and (d-1) positioning between classic VCS and Atari JR"*, Bergstrom's
`ClearMem` loop, which he pasted inline. The nineteenth is cited only as evidence that a `wavconv`
thread existed at that date, which its subject line establishes without the blob.

★**The one that came closest** is `199901/msg00099`: *"I'm attaching ALL of my data (hits.txt)"*. The
data is genuinely gone. What this repository quotes from that message is the sentence **summarising**
the data, not a number out of it — so the citation stands and the missing file bounds what could ever
be asked of it.

★★**What this does not say.** It is not a claim that attachments hold nothing worth having; 1,200
files were not examined. It says the 104 citations already made do not rest on any of them, so
fetching is a question about future work, not a repair of existing work. Re-run it before assuming it
still holds — the ids are extractable with
`grep -rhoE "[0-9]{6}/msg[0-9]{5}" docs/ internal/ CHANGELOG.md | sort -u`.
"""


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
    out.append(HAND_SECTIONS)
    open(os.path.join(HARNESS, "docs", "provenance.md"), "w", encoding="utf-8").write("".join(out))
    print("wrote docs/provenance.md")



# --- Arcade-Pong / Video Olympics confusion guard -----------------------------------
# There is NO standalone Pong cartridge for the Atari 2600. Pong on the 2600 is one
# variant inside Video Olympics (CX2621, 1977), and that is what sandbox/practice/pong
# reproduces, measured against reference/roms-study/VideoOlympics.bin. The blog mine also
# holds DanBoris's reverse-engineering of the 1972 DISCRETE-LOGIC arcade machine, which is
# a DIFFERENT GAME with different numbers — arcade accelerates at volley 4 and 12 (three
# speeds), Video Olympics at 4/8/16 with +/-1 steps capped at +/-4.
#
# This is not hypothetical: a past session implemented the arcade thresholds in the VO
# reproduction and labelled them "原典アーケード仕様" (the original arcade spec), and the
# code ended up matching neither game. Comments rot; this does not. Any note that talks
# about the arcade machine must carry the warning banner that says which game it is about.
ARCADE_MARKERS = ("アーケード PONG", "アーケード Pong", "arcade PONG", "arcade Pong")
BANNER_MARKER = "対象の取り違え注意"


def check_arcade_pong_banners():
    """Every blog note about the ARCADE machine must say it is not the reproduction target."""
    blogs = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage", "blogs"))
    if not os.path.isdir(blogs):
        return []  # umbrella not present (CI); nothing to check, and say so upstream
    bad = []
    for name in sorted(os.listdir(blogs)):
        note = os.path.join(blogs, name, "notes.ja.md")
        if not os.path.isfile(note):
            continue
        text = open(note, encoding="utf-8", errors="ignore").read()
        if not any(m in text for m in ARCADE_MARKERS):
            continue
        if BANNER_MARKER not in text:
            bad.append("reference/atariage/blogs/%s/notes.ja.md discusses the ARCADE Pong "
                       "machine without the 対象の取り違え注意 banner — a reader will take "
                       "its numbers for Video Olympics, which is the actual reproduction "
                       "target" % name)
    return bad


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
    missing.extend(check_arcade_pong_banners())
    missing.extend(check_message_references())
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
