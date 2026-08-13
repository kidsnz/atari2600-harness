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
| **job total** | **446 s (7m26)** | **624 s (10m24)** |

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

**15 minutes of job wall-clock.** Currently 10m24, so there is 4.5 minutes of headroom.

15 was chosen against the only constraint that binds: a push-to-green loop long enough that the
author stops waiting and context-switches is a loop that stops being run before pushing. GitHub's
job limit is six hours and is irrelevant here.

**When a run exceeds 15 minutes, the next commit must do one of three things and say which:**

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

There was no pressure to ship it: at 10m24 against a 15-minute ceiling there is 4.5 minutes of
headroom, so the honest position is that **no cut is needed yet**. Whoever picks this up needs a
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
# 1. Worktrees — anything but the two real checkouts is debris.
git -C harness worktree list
git -C roms worktree list
git -C sandbox worktree list

# 2. Untracked files, per repo. Look for stray .bin, scratch _test.go, agent leftovers.
for r in harness roms sandbox; do echo "== $r"; git -C $r status --porcelain; done

# 3. All FOUR repositories, not three (see below).
for r in harness roms sandbox ../260811_cover-demos; do
  echo "== $r  $(git -C $r log --oneline -1)  unpushed=$(git -C $r log --oneline @{u}.. 2>/dev/null | wc -l)"
done
```

### There are four repositories, not three

Every handoff in `STATUS.md` counts `harness` / `roms` / `sandbox`. There is a fourth:
**`../260811_cover-demos`** (created 2026-08-11, one commit, pushed to
`github.com/kidsnz/260811_cover-demos`) — a single self-contained page carrying every build of the
jacket piece as base64 with a javatari.js emulator, kept out of search results by `robots.txt` and
a `noindex` tag.

It is **published**, which makes it the one artefact here with an audience, and it appeared in no
board, no index and no memory file until it was found by accident on 2026-08-13. Its `index.html`
is *generated* from `roms/technojacket/tools/mkpage.py` and embeds the ROMs at generation time, so
rebuilding a ROM does not update the page — the page is a snapshot, and a stale one is
indistinguishable from a current one to anybody visiting the URL.
