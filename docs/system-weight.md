# System weight — the CI budget, and the debris sweep

Every gate here measures whether the code is right. Nothing measured whether the *system* was
getting heavier faster than it was getting better. This page is that second measurement: a
declared ceiling on CI wall-clock, and a shutdown sweep for the residue a session leaves behind.

Baseline taken 2026-08-13. Numbers here are measurements with a date on them, not targets someone
liked the sound of.

## 1. The CI budget

### What CI actually costs

Measured from the GitHub Actions API, per step, on the runs at `3f148ad` (2026-08-07) and
`65cbc9d` (2026-08-13):

| Step | 2026-08-07 | 2026-08-13 |
|---|---|---|
| setup (checkout, Go, DASM, clone engine, assemble) | 38 s | 38 s |
| **`go build` + `go vet` + `go test -p 1 ./...`** | **342 s** | **512 s** |
| regression scenarios | 55 s | 64 s |
| CPU conformance (Klaus + Tom-Harte) | 3 s | 4 s |
| the five gates | 3 s | 1 s |
| **job total** | **446 s (7m26)** | **624 s (10m24)** — but see the range below |

**82% of CI is one step**, and that step took +50% in a single session. Nothing else moved.

### Where the growth went — and the premise that was wrong

The session that produced the growth recorded it as "almost all of it is the audio sweep". That
was checked against a local before/after run at the same two commits, and it is not right. Per
package, artefacts removed:

| Package | delta | share | audio? |
|---|---|---|---|
| `internal/emu` | +52.7 s | 41% | mostly (the 512-point pitch sweep is 41.8 s of it) |
| `internal/cyclebound` | +31.2 s | 24% | no — `pf_deadlines` + forced refusal classes |
| `internal/audioingest` | +20.3 s | 16% | yes |
| `internal/scenario` | +17.3 s | 13% | no — blank-region overrun |
| `internal/behavmatch` | +4.6 s | 4% | no |
| `internal/ceiling` | +3.2 s | 2% | no |

**Audio is 54% of the growth, not "almost all"** — and 18% of the suite. The single pitch sweep is
32% of the growth by itself, so the instinct about *that test* was right; the generalisation to
"the audio work" was not, and it would have aimed the first cut at the wrong 46%.

> The comparison run needed correcting twice, which is worth recording because both errors are the
> house speciality. First, `go test ./...` served most packages from **cache**, so the initial
> per-package timings measured nothing; CI has no cache, so `-count=1` is the only honest local
> mirror. Second, the baseline worktree lacked `bin/p6502step` and the umbrella tree, so
> `internal/cpudiff` and `internal/ramtrace` **skipped** there and appeared to have grown 60× —
> +56 s of pure artefact in packages whose diff between the two commits is empty. Both were caught
> by asking why a number was surprising, not by reading carefully.

### The ceiling

**20 minutes of job wall-clock, and it is a LOOK-AT-IT line, not a budget.** Nothing enforces it:
`ci.yml` has no `timeout`, so a run over the line does not fail — it asks a human to go and look.
Calling it a "budget" is what made 2026-08-24 read a 41-second gap as a cliff and spend an hour
diagnosing a cause that did not exist.

**Measured 2026-08-24 over 97 successful runs: min 4.37, median 7.42, max 14.45 minutes, and
ZERO runs above 15.** Three windows counted independently (19 / 39 / 97 runs) all land on the same
ceiling of 14.45, and the top six cluster at 14.18-14.45. The line sits 5.55 minutes outside that
ceiling on purpose: **a breach means something actually changed, because nothing that has ever run
here has come close.**

The old line was 15 minutes with "1m45 of headroom", read off the slowest run then visible. That
framing measures the runner's mood: **the same commit, run four times on 2026-08-24, took 488 /
793 / 799 / 851 seconds — 6.0 minutes of spread on byte-identical work, with 2-5 seconds of queue.**
A single-run line inside that spread fires on luck. A "gap to the line" computed from one run is
not a quantity.

That claim was a **single sample, and the fastest one** — the same defect
`check_instruments.py` was extended to forbid on the very day this page was written. Measured
properly over the five runs since the growth landed, on essentially the same workload: 623 s,
623 s, 721 s, 790 s, 795 s. The spread is runner variance, not code — the run at 790 s differs
from the run at 624 s by two sub-second calibrations. **A CI budget has to be set against the
distribution, because the run that breaches the ceiling is the slow one.**

15 was chosen against the only constraint that binds: a push-to-green loop long enough that the
author stops waiting and context-switches is a loop that stops being run before pushing. GitHub's
job limit is six hours and is irrelevant here.

**When a run exceeds 20 minutes, the next commit must do one of three things and say which.**
**Do NOT start by guessing the cause — the cause is already measured and recorded**: 84% of the
wall clock is the single `go test -p 1 ./...` step, `-p 1` is there because packages share `.bin`
fixtures (`ci.yml:48`), and the entry point to fixing that is `t.TempDir()` (§ below). Start there:

1. **Make the heavy thing faster.** First resort, because it costs no coverage. A test that is slow
   because it is serial is not a test that is slow.
2. **Drop it to `-short`, and record what CI stopped checking.** A `-short` skip is a coverage
   decision wearing a performance costume. It needs the same sentence a deleted test would need.
3. **Raise the ceiling, with the reason and the new number.** Legitimate, but it must be a decision
   with a date on it rather than a drift.

What is explicitly *not* an acceptable response: trimming a sweep's point count. The reason this
project sweeps 512 pairs is that four spot checks passed for a year against a broken instrument.
Sampling a sweep re-creates the defect it was built to find.

### The first attempt at (1), measured and NOT shipped

The pitch sweep is 330 independent emulator runs, and `buildAudioROM` writes into its own
`t.TempDir()` without invoking dasm, so none of the shared-`.bin` races that keep CI on `-p 1`
apply. Taken concurrently it ran **41.8 s → 9.7 s, measuring the same 330 pairs with the same
result** — the serial and concurrent runs agree pair for pair.

It was reverted anyway, on two findings:

- **`go test -race` reported a race on the first run** ("race detected during execution of test").
  It has not reproduced since: 2 further `-race` runs of the sweep, plus 15 runs of a probe that
  drives 8 concurrent emulators, all clean. The report was captured through a `tail` that swallowed
  the WARNING block, so the racing addresses are not known. **One positive detection is not
  cancelled by clean runs** — the detector does not invent happens-before violations — so the
  correct reading is an unlocated intermittent race, not an absent one.
- **Independently fatal: the loop body calls `t.Fatal` from a worker goroutine**, via
  `warmupStable` and `buildAudioROM`. That is documented misuse — it calls `runtime.Goexit()`, so
  it kills the worker rather than the test, the deferred `wg.Done()` still fires, and the pair is
  left unmeasured while the aggregation counts it as measured. A concurrency change whose failure
  mode is a silently short sweep is the wrong change to make to the test whose entire reason for
  existing is that a short sweep once passed for a year.

There was no pressure to ship it at the time. That has narrowed since: against the measured
10m23-13m15 range the headroom is ~1m45, so **the sweep is the first place to come back to** —
safely this time. Whoever picks this up needs a
`warmupStable`/`buildAudioROM` pair that return errors instead of touching `t`, and a located
explanation of the race — in that order.

## 2. The debris sweep

Sessions leave residue, and residue is not inert. Measured 2026-08-13: a subagent's **34 MB git
worktree** was left inside the harness directory, and `check_instruments.py` — which walks the
tree — scanned it as a second copy of the repository and reported **three uncalibrated instruments
that do not exist**. A hardening tool tripped over the debris of the session that added it.

The rule that follows: **a worktree used for measurement goes outside the repository.** The
baseline run for the CI numbers above used one at `/private/tmp/.../wt-3f148ad`, where no gate that
walks `harness/` can see it, and it was removed when the measurement was done.

### Shutdown checklist

Run before reporting a session complete:

```sh
# Run from the umbrella root.

# 1. Worktrees — anything but the one real checkout per repo is debris.
for r in harness roms sandbox; do echo "== $r"; git -C $r worktree list; done

# 2. Untracked files, per repo. Look for stray .bin, scratch _test.go, agent leftovers.
for r in harness roms sandbox; do echo "== $r"; git -C $r status --porcelain; done

# 3. ENUMERATE the repositories — do not recite a number (see below).
#    NOTE THE ABSENCE OF `2>/dev/null` ON BOTH LINES. It is load-bearing.
find . -maxdepth 2 -name .git -type d          # inside the umbrella
find .. -maxdepth 2 -name .git -type d         # SIBLINGS — this is where the fourth one was

# 4. Each repository's head, and what is not on a remote.
#    "no upstream" is a THIRD state, not zero: sandbox is local-only by design, so
#    nothing but this disk holds it. Printing `fatal:` on every run would train the
#    reader to ignore step 4, so the state is named instead.
for r in harness roms sandbox; do
  if git -C $r rev-parse --abbrev-ref @{u} >/dev/null 2>&1; then
    n="unpushed=$(git -C $r log --oneline @{u}.. | wc -l | tr -d ' ')"
  else
    n="NO UPSTREAM — this disk is the only copy"
  fi
  echo "== $r  $(git -C $r log --oneline -1)  $n"
done
```

**Why step 3 must not swallow stderr, learned by shipping it wrong earlier the same day.** The
first version of this checklist ran `find /Users/shinji/Documents/2D -name .git 2>/dev/null`. On
this machine macOS refuses to list that directory — and `find` prints the refusal to stderr **and
exits 0**. With stderr discarded the step printed an empty list under a successful exit, which
reads exactly like "there are no other repositories". A step that cannot tell *nothing is there*
from *I was not allowed to look* is worse than reciting a number, because it looks like evidence.

Run without the redirect, the sibling sweep fails loudly (`Operation not permitted`, exit 1),
which is the correct outcome: it reports that it could not check. **Treat that refusal as an
unfinished step, not a pass.** To make it actually run, grant the terminal Full Disk Access in
System Settings → Privacy & Security. The umbrella-internal sweep works either way and finds the
three.

And the sibling line is the one that matters: `260811_cover-demos` lived **beside** the umbrella,
not inside it, so an enumeration that only descends from here would have missed it exactly as the
handoffs did.

### Count the repositories, do not recite them

For two days there were **four**, and every handoff said three. `260811_cover-demos` (created
2026-08-11) published a browser-playable page of the technojacket builds at
`kidsnz.github.io/260811_cover-demos/`, and it appeared in no board, no index and no memory file
until it was found by accident on 2026-08-13 — while being the only artefact here with an
audience.

**It is three again as of 2026-08-13**: the page had served its purpose, and the repository was
retired. The page itself could not be rebuilt — `roms/260809_technojacket/tools/mkpage.py` embeds the ROMs at generation
time, and three ROMs had landed since — so it is archived at
`roms/260809_technojacket/_archive/2026-08-11-cover-demos/` with the exact served bytes, a `git bundle`
of its history, and a `PROVENANCE.md` carrying the restore procedure. The bytes were nearly lost
to a subtlety worth remembering: the identical local copy at `roms/260809_technojacket/tools/preview.html` is **gitignored
on purpose**, so it had never entered any git history.

The lesson is not the number. It is that **the shutdown sweep must enumerate what exists rather
than repeat a figure**, because the figure was wrong for two days and the wrong one was written
in five places by the session that first measured it — including this page.
