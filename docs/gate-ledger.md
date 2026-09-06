# Gate ledger — what each `scripts/check_*.py` has actually caught

Six gates run on this repository. Until 2026-08-13 nothing recorded what any of them had ever
caught, so there was no way to tell a gate that earns its place from one that only looks like it
does. **A gate that has never caught anything is not free — it is a cost plus a false sense of
coverage**, and the second half is the expensive part: a green tick is read as "this class of
defect is covered", which is a claim about the future that a gate with no catches has never
supported.

This file is that record. It is machine-checked by `scripts/check_wiring.py`, which fails if a
`scripts/check_*.py` has no row here, or if a row's **Runs in** column disagrees with
`.github/workflows/ci.yml` and `scripts/git-hooks/pre-push`. That check exists because of the
first thing this ledger found: `check_memory.py` was 275 lines, had caught real defects, and was
run by **nothing at all** — not CI, not the hook. It had been unwired since it was written, and
a prose table would have gone on saying otherwise.

## What a "catch" is here

Three columns, kept apart on purpose, because collapsing them is how a gate's value gets
overstated:

- **Catch** — the gate failed on something already in the tree, and the failure revealed a
  defect: a wrong number, a claim that had drifted, a check that covered nothing.
- **Compliance** — the gate blocked new work until it complied (a missing citation, an unwired
  doc). Real prevention, but it is not evidence that a latent defect existed.
- **Self-inflicted** — the gate itself was wrong, and fixing it cost a session or a red CI. A
  debit, recorded in the same place as the credits.

## The ledger

Cost is wall-clock for one run, measured 2026-08-13 on this machine. For scale: the whole test
suite is **444 s** and all six gates together are **2.15 s**, so no gate here is a CI-time
problem and none should be cut to save time. Cut one only if its catches do not justify the
maintenance and the false confidence.

| Gate | Forbids | Runs in | Cost | Catches | Compliance | Self-inflicted |
|---|---|---|---|---|---|---|
| `check_instruments.py` | a sample→number function with no known-answer calibration; and, since 2026-08-13, one calibrated in a single state | `ci` + `pre-push` | 0.19 s | **10** | 0 | 0 |
| `check_wiring.py` | an orphaned doc, a litmus ROM nothing runs, ★a scenario check no document names, a `verified-coverage.md` row whose ROM is absent or unrun, a gate with no ledger row, **an internal package nothing imports, a command CLAUDE.md never names** | `ci` + `pre-push` | 1.57 s | **3** | many | 1 |
| `check_tests.py` | a `TestXxx` with no failure path, or whose only failure path is its own setup | `ci` + `pre-push` | 0.10 s | **7** | 0 | 0 |
| `check_provenance.py` | a technique/rule/`pkg/design` element with no source, or a citation that resolves nowhere | `ci` + `pre-push` | 0.12 s | **6** | 4 | 3 |
| `check_memory.py` | a broken wiki-link, a missing/duplicate index line, an oversize file, a canonical rule file with no concrete evidence | `pre-push` only — reads ~/.claude/.../memory, which a CI checkout has not got | 0.07 s | **3** | many | 2 |
| `check_traps.py` | emu-passes/hardware-fails patterns in kernels: unstable illegal opcodes, `NOP $00`, stack-zone variables, missing `CLD`, reads of write-only TIA registers, stores into ROM | `ci` + `pre-push` | 0.10 s | **0 over 411 files** (was 0 over 31) | 3 | **113, all found and fixed 2026-09-05 the first time it was aimed at the works** |

### `check_instruments.py` — 10 catches, the highest yield per line here

Added `65cbc9d` (2026-08-13). Five uncalibrated instruments on its first run, **two of them
written in the session that added the gate**. Writing the missing calibrations then found three
further defects in one of those functions, in a chain: `MeasureFundamental` returns 1 for any
long-run signal when the search starts at lag 1 (adjacent samples of a two-level wave are equal
except at a transition, r=0.987); refusing `lo<2` then broke the 512-point pitch sweep on eight
pairs; the CI mirror found that one. None had ever appeared in a machine test, because every
real call happened to pass a sensible bound.

The state axis, added 2026-08-13, found two more (`DominantFreq`, `BeatPhase`) and **both were
hiding a live mutant** — see the gate's own docstring for the measurements.

It also corrected a claim rather than code: "MeasurePeriod breaks on asymmetric pulses" had been
written repeatedly and is false — it returns 31.00 for a 13:18 pulse of period 31. The defect is
transitions per cycle, not asymmetry.

### `check_wiring.py` — 3 catches, and the strongest single argument for the whole family

Added `8afb476`. The catch worth remembering is `485b031`: of 91 litmus ROMs, two (`cb_roll`,
`litmus_color`) were referenced by nothing at all — and **both of their claims had drifted while
nobody was running them**. `cb_roll`'s header said "pixel-identical to `cb_clean`" and was off by
one row of 192; `litmus_color`'s said "each line a different single colour" and was wrong by a
factor of two, because the TIA masks bit 0 off every colour register, so `stx COLUBK` stepping by
1 changes colour only every second line. That is the anti-rot thesis demonstrated: an unrun
verification ROM does not stay true.

Third catch `d7b56a6`: `verified-coverage.md` opened by asserting every row was pinned by a
scenario; 7 of its 35 were pinned by a Go test instead. Nothing was unguarded — the page whose
job is to be trusted was simply wrong about 20% of itself.

**Fifth inspection, 2026-08-15 — tool reachability.** Every `cmd/` built and every gate was green
on a tree where two of the 34 internal packages could not be called at all: `internal/keyfit` (502
lines) and `internal/mixmatch` (280) had no command and no importer, while `CLAUDE.md` described
them as two of the three pillars of audio reproduction. Six further commands — `ceiling`, `ingest`,
`metamorphic`, `mine-invariants`, `motion`, `refdiff` — built, ran, and appeared in no entrypoint,
which is worse: present, and never reached for.

The check fired on its own author within the minute, naming the two commands written to fix the
first half before they had been documented. Negative controls: a package importing nothing and a
command absent from CLAUDE.md each produce their own message.

Self-inflicted (`485b031`): the checker scanned every `*.py` including itself, and its own
docstring named `cb_roll` and `litmus_color` as examples, so both came back "referenced" and it
reported 0 orphans while one was real. A checker that satisfies itself by explaining what it
checks reports 0 forever.

### `check_tests.py` — 7 catches

Added `173b6e0`: 344 test functions, exactly one that could not fail (`TestZZProbe`, a scratch
probe swept into a docs commit and green ever since). Extended in `f5ac8f3` to reject a test whose
only failure paths are setup error checks — **it fired immediately on six scratch tests** left
behind by terminated agents. The three defects that motivated that extension were found by hand,
not by the gate, and are not counted here: a test that counted non-262-line frames and *printed*
them, one that asserted `w*2 == w*2`, and one that logged "this test currently records the
boundary rather than witnessing it" and went green either way.

### `check_provenance.py` — 6 catches, 3 self-inflicted CI reds

Added `3303938` (4 docs backfilled = compliance). The catches are `ceb198c`: six mistyped
citations, each a provenance trail that led nowhere — an EVALUATION.md cited under the wrong
repository root, twice; a technique doc cited without its `techniques/` path segment; and a
reference directory cited by half its name, twice. (The wrong paths are described rather than
quoted, because this gate reads a backticked path in prose as a citation and would flag the
description of its own catch — which it did, on the first draft of this page.) The same commit
records five citations that resolve nowhere and are listed with reasons rather than deleted.

Self-inflicted (`3f893b1`): resolving citations against both the harness and the umbrella is
right on a machine with both, and turned GitHub Actions **red three times** on a CI checkout that
has one. The rule now counts and prints what it could not check, so a run that verified
everything and one that skipped a third of it do not print the same thing.

### `check_memory.py` — 3 catches, and it was wired to nothing

Added `34fedb4`. Four findings on its first run, **two of which were the checker itself** (raw
filename counting flagged a legitimate in-prose cross-reference as a duplicate index line; banning
non-memory wiki-links flagged the one naming known-traps.md, which is a real harness doc). Of the
real two, the durable one is `1304c32`, where the gate caught its own author: an edit meant to
drop a now-unneeded exemption silently failed on a bad anchor, and the gate refused. `96a9b14`
added `--report` after an agent reported one inbound reference to a memory where three existed —
merging on the reported number would have left two links dangling.

**It ran nowhere.** Not in `ci.yml`, not in the pre-push hook; the only mention outside its own
source was one line in an archived STATUS file. Wired into the pre-push hook 2026-08-13, which is
its correct home — CI has no `~/.claude` tree, so the gate skips there and would have proved
nothing.

### `check_traps.py` — the zero was a statement about where it had been pointed

Added `d7f1abc`, extended twice (`00f2280`, `15d8375`). **It has never failed on a defect.** Every
measurement it has ever reported was a clean corpus: zero hits across 31 technique ROMs at
introduction, zero across 123 ROMs when the write-only-TIA-read detector was added, and when the
store-into-ROM detector arrived it found exactly two stores in the whole corpus — both in
`litmus_6502`, both deliberate, both now carrying `@rom-write-ok`.

Kept, on three grounds, none of which is "it might catch something someday":

1. It costs **0.10 s**, which is below the noise of the suite it runs beside.
2. It is the only mechanical thing holding the three `@rom-write-ok` declarations in place. Those
   are the compliance column, and they would rot silently.
3. It is a **pre-flight** linter, and the author it is meant to stop is Claude, mid-build. A catch
   there never reaches a commit, so this ledger structurally cannot see its best case. That is an
   argument for keeping it and *not* for claiming it works — the honest statement is that its
   value is unmeasured, not that it is proven.

If it is ever cut, cut it on ground 3 being judged not worth 0.10 s, and say so.

**2026-09-05 — the zero meant something else than it read.** Everything above is about 31 files.
Called with no arguments — which is how CI (`ci.yml:79`) and the pre-push hook both call it — the
gate globbed `roms/techniques/*.asm` and nothing else, so **the author's own works were checked by
nothing**: 121 `.asm` in the first work, 2 in the second, 4 more works' worth beside them. The
docstring said the quiet part out loud without anyone hearing it: *"Zero false positives is the top
priority (the existing roms/techniques are all clean)"* — the gate was validated on the set it was
pointed at, and never pointed anywhere else.

**This is the second occurrence of one accident, and the first is written down.** `roms/allworks_test.go`
opens with it: *"Until 2026-08-15 nothing ran the scenarios in this repo … Two directories named
'roms' is the whole of it: the deliverables sat in the one nobody walked … Discovery is a WALK, not
a list."* That fix has since carried 47 scenarios to 151 with no further edit. This gate had the
same shape — `glob(os.path.join(HARNESS, "roms", "techniques", "*.asm"))` **is** the list — and
takes the same cure.

Aimed at the works, it produced **113 findings and every one was a false positive**, in three
families, none of which could occur in `roms/techniques/`:

| family | count | what it really was |
|---|---|---|
| the line was lower-cased before the register-name match | **112** | `lda pf1` — a RAM variable, `pf1 = MUSZP+7` in the music driver — read as `lda PF1` |
| `NAME = $FC` read as a variable in the stack zone | ~16 | `COLM1 = $FC ; the gold electron` is a **colour**; a value is used as `#COLM1`, a variable as `sta COLM1` |
| "no CLD" applied to a file that is not a program | 6, then 35 more once the walk reached `src/art/` | includes: data-only tail files carrying `org $FFFC` with zero instructions, and generated kernel bodies with instructions and no vector |

All three are fixed, each with a counter-bait in `--selftest` that must stay silent, and the
selftest now has both halves: eleven detectors that must fire on the bait, and four clean samples
that must not. Six mutations — including reverting each fix and silencing rule 3 — are caught.
**The gate now walks 411 `.asm` files and the tree is clean.** That is a different sentence from
the one at the top of this entry: "0 catches" has become "0, and we looked".

**Where the coverage actually lands.** Both call sites pass no arguments, so both get the walk, but
they see different trees. CI clones only this repository, so `../roms` is absent there and the run
covers the 31 technique ROMs and says so in a note — a CI green is still a statement about 31 files.
The **pre-push hook is where the works are covered**: `core.hooksPath` is set in this repository
(measured), `scripts/git-hooks/pre-push:94` calls the gate bare, and the sibling `roms/` is on disk,
so every push of `harness` now checks all 411. The `roms` repository has a `pre-commit` and no
pre-push of its own, so a push there runs nothing — pushing the instrument is what checks the works,
which is backwards but is the coverage that exists.

Found by the mailing-list distillation (helper-2), who aimed the gate at the works, opened all 113
findings, found the 2026-08-15 precedent, and measured the repair order. Re-measured here before
each change.

## Reading this table

The gate that has caught the most was added most recently, and the gate that has caught nothing
is the oldest. That is not an argument that new gates are better. It is what a ledger looks like
when the defects a gate finds are mostly found **in the session that adds it** — the act of
stating a rule precisely enough to check it mechanically is what surfaces the violations, and
after that the gate is mostly holding a line rather than discovering one.

Which means the recurring question is not "is this gate still catching things" but "has this gate
stopped being a line worth holding". Those have different evidence, and only the first is in the
Catches column.
