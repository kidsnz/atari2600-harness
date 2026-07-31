# Capability Gap Audit — mined techniques × harness capabilities

> Date: 2026-06-13. Turns "can the harness be strengthened more?" into a finite, prioritized
> list. Cross-references the mined corpus (72 AtariAge threads + research `tools/research-w1..w11`
> + Pizza Boy dissection) against what the harness can currently verify/support. Status cells cite
> `docs/fundamentals-audit.md`, `docs/verified-coverage.md`, `docs/stella-oracle.md`,
> `docs/hscroll.md`, `CHANGELOG.md`.

**Key insight:** most gaps are **not** closed by mining more. They close by (1) **codifying** mined
knowledge into code, and (2) **supporting/verifying advanced cartridges**. Views-ranked mining
already exposed the high-value gaps; the remaining long tail has diminishing returns.

**Label note:** `M1–M6` = **TIA Studio milestones** (the app, **now frozen** — see
`project-pivot-author-not-tool`). `G1–G7` = **harness capability gaps** (this doc). Some G's were
originally motivated by M's (e.g. G2 → M4), but the **current consumer of the G capabilities is Claude's
own authoring loop**, not the editor. The G work stands on its own; M references are historical.

## Tier 1 — high leverage, goal-aligned

### G2 — codify design rules into a `pkg/design` feasibility checker ★ top priority
- **Techniques:** multicolor (170018), color-band width, 48px = 12 chars (197162), asymmetric-PF
  write windows, multiplex limits, background 4-axis (319884), the craft rules.
- **Status — prose only.** The only numeric judge is `assert_line_budget` (76cy ceiling). Band
  width / char count / PF windows / multiplex limits are hand-computed.
- **Gap:** turn the ~15 `design-principles.md` rules into **executable check functions**. This *is*
  "absorbing mined knowledge into harness capability" — the pre-authoring feasibility gate Claude runs
  before writing asm (it also happened to be TIA Studio M4, now frozen). **Done in v1.67.0.**

### G1 — advanced cartridge support + litmus (DPC, DPC+, ARM/ELF, 3E/3E+, bus stuffing, separate SC-RAM)
- **Techniques:** DPC+ full-screen bitmap (181816), plain Superchip 30K (224683), bus stuffing
  (258191 / 279712), DPC+ deep dive (163495), raycasting (229083).
- **Status:** only **F8/F6/F4** are litmus-verified. DPC+/3E+/etc. are *recognized* by Gopher2600 but
  have **zero harness verification**; `read_bank` untested on them. [`fundamentals-audit.md`, `hscroll.md`]
- **Gap:** can't reliably author/verify "beyond bB / full-screen bitmap" techniques. *Not* required for
  first authoring targets (vanilla + SC bespoke kernels) — this is the **advanced-track** foundation.

## Tier 2 — depth / accuracy

### G4 — Stella oracle TIA write-register compare
- **Status:** RAM (128/128) and pixels (100%) are cross-checked; **COLUPF/NUSIZ etc. write registers
  are not compared vs Stella.** [`stella-oracle.md` v2 backlog]
- **Gap:** can't authoritatively confirm a generated kernel wrote the right registers (only indirect
  via `read_tia` / pixels).

### G3 — digital speech (4-bit DAC PCM) fidelity verification
- **Techniques:** Doctor Who speech (234209), SAM2600 (309689), Tiamat micro-tuning (386896).
- **Status:** `read_audio` covers AUDC/AUDF/AUDV + note/cents/duplicate/pitch; **no PCM waveform
  fidelity check.**
- **Gap:** only needed for speech/music games — not core to graphics-first authoring.

## Tier 3 — polish
- **G5 ✅ RE-MEASURED 2026-07-30 — the entry was stale; both halves have been litmus-locked for some time,
  and locking them turned out to be the easy part.** `litmus_resp_edge` + `scenarios/resp_edge.json` (golden
  frame) pins the RESBL-vs-RESPx double-strobe rule — RESBL re-issues START (two balls on one line, clocks 38
  and 140) while RESPx does not until the next 160-clock wrap (one player, clock 107). `litmus_hmove_mid` +
  `scenarios/hmove_mid.json` pins mid-line HMOVE.
  **What the re-measurement found:** hmove_mid's four asserts read as a four-entry table (55, 60, 60, 60) and
  they are not one — three of the values coincide **for two different reasons**. Measured per frame by counting
  HMOVE latches: the control frame latches **once** (scanline 1) and sits at 60; the other three latch
  **twice** (scanline 1 and the mid-line strobe on scanline 136), and of those three **only one moves the
  object** (to 55). So a mid-line HMOVE with HM=0 shifts at one of the three strobe positions this ROM tries
  and does nothing at the other two — and the position asserts cannot tell "the strobe fired and did nothing"
  from "the strobe never fired", which is the fixture's entire subject.
  `TestHmoveMidStrobesAllFireButOnlyOneShifts` counts the strobes, so a regression that stops emitting them
  fails there even though every scenario assert would still pass. Negative control: hiding the mid-line latch
  reports "0 frames carried a mid-line strobe, expected 3".
- **G6 ✅ MEASURED AND HANDLED (2026-07-30):** oracle sub-frame phase offset for per-frame-mutating RAM.
  **"Run N frames and dump RAM" does not name a moment, and the oracles pick different ones.** New fixture
  `roms/litmus/litmus_framephase.asm` bumps a separate counter at three points in one frame — `$80` just after
  VSYNC, `$81` at the midpoint of the visible field, `$82` as the last instruction before the next VSYNC —
  so the three values say *where* an oracle read them. **Measured at frames=10: Gopher2600 gives
  `$80=10 $81=9 $82=9`** (it stops at the program's own frame boundary, just after VSYNC) **and MAME gives
  `$80=10 $81=10 $82=9`** (its frame notifier fires after the visible midpoint). On a ROM where nothing is
  wrong they disagree about `$81`, every time — and any game that updates a byte between those two moments
  produces the same false dissent.
  `oracle.ClassifyDiff(refN, refNext, other)` bounds the artefact with the reference oracle at N and N+1 and
  splits the offsets into **real** and **sampling-phase**; both are returned, because a one-off in a counter is
  also what a genuine engine bug looks like. `cmd/oraclevote` prints both counts.
  **A second defect fell out of it:** with exactly TWO oracles a disagreement has no strict majority, so `Vote`
  returned `ok=false` with an **empty** dissenters list — the tool exited 1 while naming neither the oracle nor
  the offset, and two oracles (Gopher + MAME) is the normal case here. It now reports who differed and where.
  Litmus: `real []`, `phase [1]` on the fixture; `smoke.bin` unanimous. Negative control: a classifier that
  excuses any byte the reference moved mis-sorts offset 2 and loses offset 3.
- **G7 — DONE (v1.116.0) for the collision half.** `watch_ram` traps a RAM change and names the instruction
  that made it; collisions had no equivalent. New `emu.WatchCollision` reports where each CXxx latch was
  FIRST set — beam scanline and clock in `read_row` coordinates, plus the PC executing at that colour clock.

  **The beam position is the part that cannot be recovered afterwards**, which is why it is captured inside
  the existing per-colour-clock hook rather than reconstructed: CXxx latches are sticky and a game clears
  them with CXCLR every frame, so a frame-boundary sample answers "did it happen" and nothing else — and by
  the time an instruction retires the beam has moved on.

  It reports every requested pair rather than stopping at the first: "the bullet hit the wall" and "the
  bullet hit the player" on the same frame are different answers, and a trap that stopped would hide one.
  A misspelt pair name is an **error listing the valid names**, not an empty result that would read as
  "nothing collided".

  Measured in three directions: `litmus_collide_all` fires **all 15 pairs**, each with a real PC and an
  in-range beam clock (frame 3, scanline 36, clock 2); `shared_setxpos`, whose five objects sit at separate
  fixed columns, reports **0**; an unknown pair name errors. A trap that fires on everything is as useless as
  one that fires on nothing, so both directions are asserted. `step_clock` remains parked.
- **G7 (original):** collision trap (`watch_ram` is RAM-only); `step_clock` (parked).
- **G8 — PARTIAL (v1.116.0): the detector exists, the positive case has no ROM witness.** New
  `emu.WatchTimerDividerHazard` reports every write to TIM1T/TIM8T/TIM64T/T1024T whose own cycles straddle
  the counter's underflow, folding every RIOT mirror through `memorymap.MapAddress` and taking the register
  addresses from the engine's own table (`cpubus`: `$0294`..`$0297`) rather than a literal.

  **Confirmed first that the emulator does not reproduce the bug**, which is why a detector is needed rather
  than a test: `Gopher2600/hardware/riot/timer/timer.go` `Update` assigns the requested divider
  unconditionally and resets `ticksRemaining` to 0, with no wraparound race. The consequence is invisible
  here, so only the hazard can be reported.

  **The condition took two measurements to get right, and the first was wrong.** "INTIM == 0 with the
  decrement due" fires on nothing, because at instruction granularity that instant is almost never observed:
  measured at the deliberately-hazardous store in `litmus_timerwrap_nearmiss`, the timer reads **INTIM=255,
  ticksRemaining=0, divider=8** — it had already wrapped, the polling loop's exit plus one `lda` taking longer
  than the ticks that remained. The computable condition is instead whether the store's cycles CONTAIN the
  underflow: `ticksRemaining + INTIM*divider ≤ storeCycles`. On that ROM that is **2040 against 4**.

  **So the litmus is named for what it measured as, not what it was built as.** `litmus_timerwrap_nearmiss`
  writes a divider just AFTER a wrap — the ordinary, safe shape — and is the negative control: a detector that
  flagged it would cry wolf on every timer-driven kernel. Locked, with 0 hazards on it, on `game_states` and
  on a timer-free ROM, plus a test proving **6 divider writes are actually observed** so the silence is not a
  detector returning nil.

  **Stated, not implied: the positive case is untested.** Arranging a store whose cycles straddle the wrap
  needs cycle-level tuning of the polling exit. Until such a ROM exists this detector's silence must not be
  read as "no hazard here". Original entry follows.
- **G8 (mined 2026-06-14, on-mission):** RIOT timer-wraparound roll detector. Writing `TIM64T`/`TIM1024T`
  on the exact wraparound cycle silently drops the divider to 1T → the ROM rolls on real hardware while
  Stella/emulators pass. This is precisely the "passes-in-emu / fails-on-hardware" timing trap the harness
  exists to catch (gap B). A `breakif`/assert that flags timer writes on the wraparound cycle would be a
  natural sibling to `assert_line_budget`. Source: design-principles.md (採掘 303277), diagnosed in-thread
  by the Gopher2600 author. Verify the exact behaviour against Gopher2600's RIOT model before implementing.
- **G9 (surfaced 2026-06-15, Fishing Derby casebook):** *authoring-craft* support for two patterns the
  Claude-side reconstruction missed (`docs/casebook.md`): (a) **per-scanline NUSIZ+HMOVE shaping** of one
  player into an 8px-plus irregular sprite (the shark), and (b) **fractional-HMOVE slope** drawing of an
  arbitrary-angle 1px line on a missile/ball (the fishing line). A `pkg/design` estimator or a
  `docs/techniques/` skeleton for these would stop the next sports/action build from falling in the same
  hole. Concrete-driven: build when the next ROM needs it. Source: `_casestudies/fishing-derby/diff-gaps.ja.md`.

## Tier 2b — testing/verification discipline import (2026-06-16, `docs/testing-playbook.md`) — **DONE**
Imported the established software-testing discipline (oracle problem → property/metamorphic/fuzz/mutation,
deterministic simulation testing à la FoundationDB/Antithesis) onto the existing deterministic emulator +
`internal/scenario` substrate. Source provenance in `docs/testing-playbook.md`. **All G10–G14 delivered.**
First catch: the suite immediately exposed that Breakout rendered 264 lines, not the "262" claimed by eye.

- **G10 — scenario `invariants` / `monotonic` / range operators. ✅** Conditions checked **every frame**
  (`invariants`), single-direction guards (`monotonic`), and the `in` range operator. `internal/scenario`. (QuickCheck.)
- **G11 — `fuzz` (seeded random input + invariant monitoring + replay). ✅** Deterministic seeded random
  inputs over N frames, invariants monitored each frame, CPU-jam detection, reproducible by seed.
  `internal/scenario` `fuzz`. (AFL / FoundationDB-Antithesis.)
- **G12 — `metamorphic` (two-run relation). ✅** `internal/metamorphic` + `cmd/metamorphic`: assert
  `A.field <rel> B.field` (oracle-free). (Chen / Segura survey.)
- **G13 — `mutation` (grade the tests). ✅** `internal/mutate` + `cmd/mutate`: inject a ROM-byte fault, confirm
  the suite catches it (kill) or flag a survivor. (DeMillo / Offutt.)
- **G14 — `mine-invariants` (Daikon-lite). ✅** `internal/mine` + `cmd/mine-invariants`: observe fields over a
  driven run, emit candidate `invariants`/`monotonic`/range as a spec draft. (Daikon.)

## Recommendation (concrete-driven, per project principle)
1. ~~**G2 first**~~ **DONE in v1.67.0** — mined rules codified into `pkg/design` (color/position/pf/
   multiplex/craft + the existing budget/text), each rule in `design-principles.md` cross-referenced to
   its function or marked doc-only. This was the pre-authoring feasibility gate (prose → *capability*).
2. **G1 next** — advanced-cartridge litmus = foundation of the "beyond bB" advanced track, built
   incrementally as specific techniques demand it.
3. **G4** as oracle-completion hardening, anytime.
4. **G3 / G5 / G6 / G7** only when concretely needed (avoid gold-plating; verification-first).

→ The harness **can** be strengthened more — G2 done (v1.67.0); remaining order is **G1 (advanced
carts) → G4 (oracle)**, not more mining. A **finite backlog**, not infinite mining.

---

# Verification-Variety backlog (VV-*) — 2026-06-17

> Motivated by the user's insight: *"Claude's accuracy comes down to how much it can verify itself — accurate
> changes at accurate **values** let it commit accurately to the result."* So the fastest capability lift is
> **more independent KINDS of verification**. Six parallel research agents surveyed six angles (provenance in
> each finding; raw agent scratch was consolidated here and deleted). Center of gravity = **technical +
> perceptual-numeric** (oracles the user can judge by number); subjective game-craft stays the user's call.
> Every VV item ships via the usual pipeline (litmus/numeric verify → CI → tag) and **must carry a
> planted-discrepancy self-test** (the new oracle must itself be falsifiable — like `stellacheck` proving it
> catches an injected diff). Provenance per item = `feedback-provenance-always`.

**Cross-cutting finding:** the three highest-leverage items need **almost no new infrastructure** — the
substrate is already vendored. Klaus + Tom-Harte CPU suites + their Go runners are already in the embedded
Gopher2600 tree (just never run by harness CI); the exact 256-opcode cycle table is in-tree (`instructions.
Definitions`) for the static prover; PC/branch coverage is a few lines inside the existing `emu.stepInstr()`
loop. Much of the highest-value verification is **activation + ownership**, not new building.

## Ranked backlog

| ID | Capability | New class it adds | Source | Size | Tier |
|---|---|---|---|---|---|
| **VV-1** ✅v1.78.0 | Activate vendored **Klaus + Tom-Harte 65x02** CPU suites in CI (gated) | external, exhaustive, all-256-opcode + **per-cycle bus** certification of the engine | B-C1 | S–M | ★1 |
| **VV-2** ✅v1.80.0 | **Static per-scanline cycle-budget PROVER** (`cmd/cyclebound`) | **proof over ALL paths** (∀) vs observe-one-run — the only ∀-claim member; attacks gap B | C-1 | M | ★1 |
| **VV-3** ✅v1.83.0 | **PC/branch coverage map** (`cmd/cover`) → **coverage-guided fuzzing** (`cmd/guidedfuzz`) | test-adequacy axis + AFL-style feedback fuzz (today's fuzz is blind) | D-1→D-2 | S→M | ★1 |
| **VV-4** ✅v1.79.0 | **Motion-smoothness / jerk metric** (`cmd/motion` + `read_motion` MCP + `checks.motion`) | per-frame motion-jerk NUMBER = "judder/ブルブル" automated (closes the Breakout hand-trace) | E-1 | S–M | ★1 |
| **VV-5** ✅v1.82.0 | **Temporal-logic trace assertions** (`temporal` block: eventually-within-K/response/never-for-N; `always`=existing invariant) | properties over a **sequence** of frames (per-frame invariants can't) | F-1 | M | ★1 |
| **VV-6** ✅v1.90.0 | **MAME headless cross-oracle** (`internal/oracle.Mame` + `cmd/oraclevote`) | a **3rd independent** full-system oracle, **fully headless** (no keypress unlike Stella) | A1 | M | 2 |
| **VV-7** ✅v1.91.0 | **perfect6502 silicon CPU differential** (`internal/cpudiff` + `cmd/cpucheck`) | transistor-netlist truth at the **CPU layer** (catches a CPU bug ALL software emulators could share); covers undocumented/decimal opcodes Harte (VV-1) excludes | A2/A3/B-C3 | M | 2 |
| **VV-8** ✅v1.84.0 | **Behavioral trajectory diff vs original ROM** (`cmd/trajdiff`) | full **time-extended** state-trajectory diff (refdiff is a static snapshot) | F-2 | M | 2 |
| **VV-9** ✅v1.87.0 | **Score/lives OCR semantic oracle** (displayed digits == RAM) | ties **display ↔ program meaning** (template-match, pure-Go, no Python) | E-2 | M | 2 |
| **VV-10** ✅v1.88.0 (T-1/T-2/T-3) | **HW-divergence trap detectors** (timer-wrap=G8 ✅, HMOVE-latch ✅, uninit-RAM-read ✅) | runtime monitors for "passes-in-emu / fails-on-HW" (siblings of `assert_line_budget`) | F-3 | M | 2 |
| **VV-11** ✅v1.92.0 | **State-coverage matrix** (NUSIZ/size/VDEL/PF-mode/bank; `internal/statecov`+`cmd/statecov`) + **coverage-filtered mutation** (`mutate.EvalRandomCovered`, `cmd/mutate -covered`) | did tests exercise every TIA mode; **honest** mutation kill-rate (closes the playbook's 5–20% thread — smoke: 2%→68%) | D-3/D-4 | S–M | 3 |
| **VV-12** ✅v1.93.0 | **SSIM / pHash tolerant frame compare** (`internal/framesim`+`cmd/framesim`) | magnitude+locality "how wrong, and where" (exact golden is boolean) | E-3 | S–M | 3 |
| **VV-13** ✅v1.94.0 | **Audio spectral (FFT) + RMS-envelope diff** (`internal/audiospec`+`cmd/audiospec`) | frequency-domain timbre check (out-resolves `golden_audio` on V2-14 inverted twins) | E-4 | S–M | 3 |
| **VV-14** ◑v1.96.0 | **`cmd/cpucert`** ✅ · `@lines` real kernels ✅ · **interprocedural JSR/RTS + divide-loop bounding** ✅ (2A/2B) · ILP/SMT(Z3) + external TIA/Sim2600 ROMs (defer) | citable cert + prover scope expansion done; no false-positive violations remain (**15/31 certified** — re-measured 2026-07-30; recorded as 14 and moved when SD-6 removed the two-call-shared-subroutine false positive, which greened `game_states`); rest are honest UNBOUNDED scope limits | B-C2/C-2/B-C3 | M–L | 3 (partial) |

## Tier ★1 — recommended pilots (highest value × feasibility, substrate mostly in-tree)
- **VV-1 ✅ DONE (v1.78.0):** the suites are run via the full import path `go test github.com/jetsetilly/gopher2600/hardware/cpu/tests/{klaus2m5,thomharte}/...` (resolved through go.mod's `replace`; `cd Gopher2600` is wrong — go binds the harness module). **Klaus** always-on (embedded .bin committed upstream, no provisioning). **Harte** runs a 12-opcode smoke subset in CI — `a9 69 e9 d0 4c 6c 20 b1 9d fe 00 ca` — fetched on demand from SingleStepTests (the 1GB corpus is `.gitignore`'d upstream); full 256 is local-only (`scripts/check_cpu_conformance.sh full`). New `scripts/check_cpu_conformance.sh` (+ `--selftest`) + two CI steps. **Self-test (mandatory):** corrupt one expected `final.a` in a Harte case → the gate must go RED (proven live, not vacuous). **Src:** Klaus2m5 repo; SingleStepTests/65x02 (MIT).
- **VV-2 ✅ DONE (v1.80.0; precision S0–S3 v1.81.0):** `internal/cyclebound`+`cmd/cyclebound` recursive-descent-decode the ROM from its
  reset/IRQ/NMI vectors (so inline data isn't misdecoded), cost each instruction from in-tree
  `instructions.Definitions` (exact cycles + branch-taken/page penalty), cut the CFG at every `STA WSYNC` ($02),
  and prove each WSYNC-to-WSYNC region's DAG longest path ≤ budget (default 76, no solver). Counted loops
  (`ldx/ldy #N` + `dex/dey` + `bne/bpl`) folded by bound; JSR / indirect JMP / unbounded loops reported honestly,
  never passed; over-budget regions return a cycle-by-cycle worst path + source location. Reuses
  `internal/build.AssembleWithListing` + `internal/srcmap`. Shipped 3 ways: `cmd/cyclebound` CLI, the
  **`prove_line_budget` MCP tool**, and scenario **`checks.prove_line_budget`** (`cyclebound_safe.json` certifies
  smoke). Self-test `TestCycleboundSelfTest` (planted discrepancy): `cyclebound_branch` overruns only on one
  branch (~101cy) so a live run is a lucky pass yet the proof FLAGS it (∀ catches what ∃ misses); `litmus_overrun`'s
  counted delay loop bounded + flagged (108cy); `smoke` certified (worst 19cy); a tight budget flips smoke to a
  violation (non-vacuous); the certified bound holds at runtime (observed-within-proven dual). **Precision
  S0–S3 (v1.81.0):** S0 = abstract-interpretation engine (`absint.go`, per-address value-range state); S1 =
  region recognition (VSYNC/VBLANK/timer-driven intervals classified and skipped, so a long blank region isn't
  a false over-budget); S3 = **page-cross precision** — an `abs,X`/`abs,Y` read's +1 is resolved from the
  proven index range: if `[base+lo, base+hi]` stays inside one 256-byte page the penalty is **0**, an unknown
  index or pointer-based `(ind),Y` stays conservative (+1). Loop-body costing keeps the conservative `nodeCost`
  (sound, over-approximating). Cuts false positives on real kernels without weakening the proof; self-test (no
  false-negatives) stays green. **Scope (v1 — honest over guessing, found by running it on the real technique
  kernels):** single-bank flat 2K/4K; loops bounded only via the `ldx/ldy #N`+`dex/dey`+`bne/bpl` idiom —
  **divide-by-15 coarse-positioning** (A-register `sbc/bcs`) and other A-reg/memory-counter loops are reported
  **unbounded**, not over-estimated into a false violation; a ROM with no reachable `STA WSYNC` (bank-switched
  display loop) is reported unbounded, never vacuously certified. Tightening the remaining loop idioms
  (constant-propagation to bound divide-by-15) and infeasible-path exclusion is **VV-14** territory. **Limit /
  why this prover exists:** a *small* per-scanline overrun (one heavy line = 262→263 scanlines) is
  **all but invisible** — the TV's auto-sync absorbs the one-line slip so the picture does not roll.
  **★re-measured 2026-07-30:** the older claim here was that `cb_roll` and `cb_clean` are *pixel-identical*,
  and they are not: of 192 visible scanlines **exactly one differs** (scanline 133, where the stolen line
  duplicates the stripe above it — `$060606` against `$380774`). One row in 192 is not something anyone
  catches by eye, while 263 against 262 is unambiguous, so the true number argues for the prover better than
  the false one did. Pinned by `internal/emu.TestCbRollIsOneRowFromCbClean`. Visual checking is unfit for this class of defect; only the numbers
  differ — the unseen overrun is exactly VV-2's territory. **Green-ification (2-line kernels) ✅ mechanism done
  (v1.89.0):** a `; @lines N` note on the source line that opens a WSYNC region sets that region's budget to
  N*76, so a legitimate 2-line kernel (~152cy between WSYNCs: multicolor48 / score6 / tia_pcm / exerciser) is
  certified instead of falsely flagged. Sound — it only scales a specific region's budget, never disables a
  check; an un-annotated over-76 region still flags (litmus `cb_2line` vs `cb_2line_noann`,
  `TestTwoLineBudgetAnnotation` both directions). Applying the annotations to the actual game ROMs is a
  roms-repo follow-on. (Full infeasible-path green-ification stays **VV-14**.) **Src:** Li&Malik IPET DAC'95; Ballabriga&Cassé WCET'08.
- **VV-2 scope expansion (v1.96.0, via VV-14 2A/2B):** the prover now (2A) **follows subroutine calls** —
  `longest()` threads a single-level return address (memo keyed by `(addr,ret)`), JSR descends into the callee and
  RTS returns; nested call / RTS-without-caller ⇒ UNBOUNDED (sound) — and (2B) **bounds the divide-by-15 /
  sbc-counter idiom** from A's proven loop-entry range. Self-tests `cb_jsr.asm`/`cb_divloop.asm` (both directions).
  Measured: removes the JSR blocker and bounds known-range divide loops, but only flips +1 kernel to certified by
  itself (sfx_demo, via the 2C `@lines` once 2A exposed its region) — the rest are blocked by a combination of
  honest scope limits (no-WSYNC, multi-call-site RTS context, nested loops, WSYNC-in-loop, divide loops whose
  counter lives in untracked indexed RAM). Those stay UNBOUNDED (correct), not false positives.
- **VV-2 value-range arc (v1.97.0, "諦め悪い" array-range push):** three more sound, composable absint capabilities,
  each litmus-locked — **3A** AND/ORA #imm range + reading a divide loop's entry value from the fall-through
  predecessor (header is back-edge-polluted); **3B** zero-page **RAM** array-element range (`State.ZPVal`, $80–$FF
  only — TIA/RIOT regs at $00–$7F excluded); **3D** **ROM data-table** value range (constant bytes read from the
  binary over the proven index). Litmus `cb_andloop`/`cb_arrloop`/`cb_romtable`. **Measured: +0 real kernels.** The
  recurring root limitation is now precise: real kernels' counters/indices are **loop-carried**, so they are
  over-approximated to Top at the loop header (the dec/sbc wraps on the exit edge); recovering the in-loop counter
  range (narrowing it on the loop branch) + tight table extents is an open-ended precision tail with diminishing
  per-kernel payoff. The building blocks are in for when the root fix is tackled. **Src:** Cousot&Cousot (interval
  domain); array-smashing (Blanchet et al.).
- **VV-3 ✅ DONE (v1.83.0):** opt-in coverage recorder hooked into `emu.stepInstr()` at instruction completion
  (`LastResult.Address`/`Defn.IsBranch()`/`BranchSuccess`) → `internal/emu.Coverage` (pcSeen + per-branch
  taken/fall-through edges; `OneSidedBranches`, `Signature`); nil until `EnableCoverage` = zero cost. **`cmd/cover`**
  emits reached-coverage + one-sided branches (on `cyclebound_branch` it flags `0xF036` — the very path VV-2
  statically proves overruns but runtime never takes = an independent cross-check). **`internal/guidedfuzz`** +
  **`cmd/guidedfuzz`** = AFL-style search: a corpus of input sequences grown whenever a mutation reveals a new
  coverage marker (search core decoupled from the emu via `Evaluator`, so it is unit-testable). Self-test:
  `TestCoverageLogic` (one-sided detection; an unrecorded address reads as uncovered), `TestGuidedBeatsBlind`
  (synthetic staircase oracle — guided reaches full depth 9 markers while blind stalls at 4 on the same 6000-iter
  budget, deterministically), plus emu-wiring integration tests. **Scope (honest):** full dead-code over the
  *decodable* universe (reusing the cyclebound decoder) is a follow-on; today's map is reached-coverage +
  one-sided branches. **Src:** Zalewski AFL whitepaper; Go native fuzzing.
- **VV-4 ✅ DONE (v1.79.0):** `internal/motion` (pure `Analyze` + `TrackObject`) tracks an object's exact X (`Markers().HmovedPixel`) and rendered top (column scan over a uniform-background window) over N frames → velocity / accel / **jerk_rms** (RMS of the 2nd difference; 0 = constant velocity) + `max_accel`/`monotonic` (glitch vs benign staircase). Shipped 3 ways: `cmd/motion` CLI, **`read_motion` MCP tool** (interactive — used live on the Breakout ball: vertical jerk 0, horizontal jerk 1 = the benign 1px/2-frame staircase, not a bug), and scenario **`checks.motion: max_jerk_rms`** (regression gate). Litmus `motion_glide` (clean +1/frame → jerk 0) + `motion_stutter` (+2,0,+2,0 → jerk 2). Self-test = Go `TestMotionSelfTest` (glide jerk 0 vs stutter ≫) + scenario probe. **Validated against the user's own perception: motion_stutter run in Stella reproduced their exact "ブルブル" symptom.** **Src:** Flash & Hogan min-jerk 1985.
- **VV-5 ✅ DONE (v1.82.0):** `temporal` block in `internal/scenario` with bounded LTL₃/MTL monitors, reusing the
  existing condition vocabulary (`resolve` + `condPass` + `condDesc`). Three new `kind`s — **`eventually`** (P
  within K frames; bounded liveness), **`response`** (every trigger A answered by P within K), **`never_for`**
  (P not for N consecutive frames; safety); `always` stays the existing `invariant` (not duplicated). Each
  monitor's proposition (and the response trigger) is observed into a per-frame boolean trace in the run loop,
  then the verdict is computed off the trace — liveness whose window isn't fully observed reports
  **INCONCLUSIVE** (`Pass:false`, never a vacuous green). **Scenario-only — no MCP tool, so no reconnect** (a
  deliberate low-friction choice). Self-test: `TestEvalTemporal` fixes pass/fail/inconclusive for all three on
  planted boolean traces (frame-base independent), `TestTemporalThroughRun` proves the resolve→observe→eval
  wiring on `smoke.bin` + inconclusive-is-not-green + invalid-definition rejection; sample
  `roms/litmus/scenarios/temporal.json`. **Src:** Bauer/Leucker/Schallhart TOSEM 2011; STL RV'15.

## Tier 2 — high value, more dependent or larger
- **VV-6 ✅ DONE (v1.90.0):** shared `internal/oracle` (Oracle interface = `DumpRAM`: run a ROM from power-on N
  frames → RAM $80-$FF; `Diff`; `Vote` = majority dump + named dissenters), extracted from `cmd/stellacheck`
  (now reuses `oracle.Gopher`). `oracle.Mame` runs MAME's a2600 driver `-video none -skip_gameinfo` with a lua
  autoboot script that dumps RAM after N frames — a genuinely independent, **fully hands-free** 3rd oracle
  (unlike Stella's keypress). CGO-free (shells out to the `mame` binary). `cmd/oraclevote` fuses every available
  oracle into a majority verdict that surfaces "all software agrees but the hardware-grade member dissents" =
  the project's reason to exist. Self-test (gated on MAME present): MAME agrees with Gopher2600 on all 128 RAM
  bytes of `smoke` and they vote unanimously; `TestVoteDissent` proves a planted lone dissenter is named.
- **VV-7 ✅ DONE (v1.91.0):** perfect6502 (mist64, the visual6502 transistor netlist) as a hardware-grade
  **CPU differential**. It is CPU-only (no TIA/RIOT) so it is **NOT** a member of the full-system RAM vote
  (`cmd/oraclevote`) — forcing it in would mean hand-writing a 2600 around it, defeating the point. Instead it
  cross-checks at the **instruction layer**, where Gopher2600 and MAME (both software) could share a CPU bug no
  software-vs-software vote would catch, and it is **generative** (random states, all 256 opcodes incl. the
  unstable undocumented ones Harte excludes). `internal/cpudiff/p6502step/p6502step.c` runs one instruction on the
  netlist (register injection via a measure.c-style prologue; instruction boundary from the **SYNC line**, node
  539, so it is robust even when control flow returns to the instruction; writes via memory diff). `internal/cpudiff`
  runs the SAME image+prologue on the embedded Gopher2600 CPU (`cpu.NewCPU`) — **symmetric**, so both reach
  identical pre-instruction state by construction — and diffs registers/cycles/writes with P bits 4/5 masked.
  `cmd/cpucheck` sweeps seeded random vectors and exits 1 on any **unexpected** divergence. Empirically, all 256
  opcodes agree across many seeds **except 11 illegal/unstable opcodes** (ANC/ALR/ARR/ANE/SH*/TAS/LAS), which form
  a classified allow-list. **★re-measured 2026-07-30: the count was right and the contents were not.** The list held
  **12** entries and `cmd/cpucheck` never reported which of them actually fired, so an entry could silence an opcode
  while catching nothing. Measured per entry: `$AB` (LXA/LAX #imm) was exercised **110 times across seeds 1-4 and
  diverged 0 times** — Gopher2600 and the netlist agree on it — while the other 11 all fire. `$AB` removed (a real
  engine bug there is now a failure, not a waved-through "known unstable"); cpucheck now prints
  `diverged/tested` per entry plus an explicit `allow_list_never_diverged`;
  `TestAllowListEntriesEarnTheirPlace` fails if any entry stops firing. Negative control: reinstating `$AB` fails
  it by name. Main build stays **CGO-free** (perfect6502 is an external binary, shelled out;
  `scripts/install_perfect6502.sh` fetches the pinned clone + builds `bin/p6502step`). Self-tests: always-on
  differ-logic (planted-mutant, no binary) + gated silicon differential. FPGA/real-2600 = manual escalation only.
  **Src:** mist64/perfect6502 @ 09fc542 (MIT; measure.c register-injection idiom); visual6502 SYNC node 539.
- **VV-8 ✅ DONE (v1.84.0):** `internal/trajdiff` + `cmd/trajdiff` step original vs candidate ROM in lockstep on
  one input timeline and report the first-divergence frame+field, or MATCH. Default trajectory = the 128-byte
  RAM each frame (`emu.PeekRAM`); custom fields reuse `scenario.ResolveField`. Diffs **behavior over time**, not
  bytes — the strongest oracle for a reproduction task. Pure Go, no external dependency, no reconnect. Self-test
  (`TestTrajdiffSelfTest`): identity = MATCH (also a determinism guard); a corrupted reset vector diverges
  (behavior-sensitive); a behaviorally dead-byte flip = MATCH (behavioral, not a byte compare). CLI exits 1 on
  divergence. **Src:** Martignoni TOSEM'13; EXAMINER ASPLOS'22; McKeeman 1998.
- **VV-9 ✅ DONE (v1.87.0):** `internal/ocr` reads the RENDERED digit pixels (not registers), matches each glyph
  against templates rendered from a **ground-truth font (the spec, not the ROM's own table)** — PF1=MSB-first /
  PF2=LSB-first per the verified playfield bit order — and decodes the displayed 2-digit packed-BCD score. Asserts
  displayed == `decode(RAM)`, tying display to program meaning (catches display-kernel/BCD-split/font-index bugs an
  exact hash would pass — it would also accept a consistently-wrong glyph). Band located by detecting its top then
  sampling at the kernel's fixed row spacing (robust to blank glyph rows). Pure-Go, scenario check
  `checks.score_equals_ram` (ground-truth font from `<scenario>.font`; no MCP tool / no reconnect). Litmus
  `score2.asm` (RAM $80 packed BCD '42' via PF1/PF2). Self-test `TestScoreOCRSelfTest`: genuine ROM decodes 42 ==
  RAM; a font-index mutation (glyph 8 copied over glyph 4, RAM untouched) is caught as displayed≠RAM. **Src:** pHash Hamming primitive.
- **VV-10 (HW-trap detectors):** siblings of `assert_line_budget` in `internal/emu`. **T-1 ✅ DONE (v1.85.0):**
  RIOT timer-wrap (=G8). `Emu.TimerState` exposes the timer (INTIM/TIMINT/Expired/Divider/ticks); `Emu.WatchTimerWrap`
  flags the first **read of INTIM while the timer has already wrapped (Expired)** — the real G8 signature. **Key
  finding (measured):** the naive "flag any wrap" is wrong — a *clean* kernel's timer also wraps later in the
  frame (after the poll exits at 0), but nothing reads INTIM then; so the trap is specifically *read-after-wrap*
  (and `Expired` must be sampled BEFORE the step, since reading INTIM clears it). Exposed as scenario check
  `checks.no_timer_wrap` (no MCP tool / no reconnect). Planted/clean litmus: `timerwrap_trap` (TIM1T, ~7cy poll
  overshoots 0 → reads post-wrap = hit) vs `timerwrap_clean` (TIM64T polled to 0 = no hit); `TestTimerWrapDetector`
  locks both directions. **T-2 ✅ DONE (v1.86.0):** HMOVE-then-HMxx<24cy. `Emu.WatchHMOVEHazard` flags a write to
  a motion register (HMP0/HMP1/HMM0/HMM1/HMBL or HMCLR) within 24 CPU cycles of an HMOVE strobe (Stella-PG
  "unpredictable motion"). The window is measured in **color clocks** (72 = 24 CPU cy) via `Coords`, not the
  executed-cycle counter (which excludes WSYNC stalls) — so a clean kernel that separates HMOVE from HMxx with a
  WSYNC reads as outside the window. Scenario check `checks.no_hmove_hazard`; litmus `hmove_trap`/`hmove_clean`;
  `TestHMOVEHazardDetector` locks both directions. **T-3 ⏳ deferred (honest):** uninitialized-RAM read
  (shadow-memory mask) needs the *effective* address of **every** RAM write — including indexed and `(ind),Y`
  modes (one `sta $80,x` in a clear loop writes 128 bytes). **T-3 ✅ DONE (v1.88.0):** `Emu.WatchUninitRead`
  flags the first read of a RAM byte never written since reset (passes-in-emu deterministic value /
  fails-on-HW power-up garbage). The enabler is `Emu.effectiveAddr` — full effective-address resolution for
  every operand mode: Absolute (zero-page folded in via `Defn.Bytes==2`), AbsoluteX/Y (with zp wrap), and
  `(ind,X)`/`(ind),Y` via pointer dereference — so an indexed clear loop is fully tracked and **not** a false
  positive (proven by the clean litmus). Stack push/pull are implied (no operand) = outside this operand-based
  tracker, self-consistently. Scenario check `checks.no_uninit_read` runs on a **fresh emu from reset**
  (from-reset property). Litmus `uninit_trap` (reads $90 with no clear = hit) / `uninit_clean` (indexed clear
  then read = no hit); `TestUninitReadDetector` locks both directions. **VV-10 complete (T-1/T-2/T-3).** **Src:**
  known-traps.md §A/§D; AtariAge 303277; Valgrind Memcheck (shadow memory).

## Tier 3 — polish / softer / defer
- **VV-11 ✅ DONE (v1.92.0):** `internal/statecov`+`cmd/statecov` build a **state-coverage matrix** — a coverage
  axis orthogonal to PC/branch (VV-3): which TIA *modes* the test actually drove (NUSIZ copies, missile/ball size,
  VDELP0/P1/BL, PF reflect/score/priority, bank), sampled per scanline over a multi-frame run. An axis stuck at
  its reset value = a verification blind spot (e.g. smoke moves nothing; multicolor48 drives triple-copy NUSIZ;
  banked_game shows 2 banks; pf_modes shows score/priority). `mutate.EvalRandomCovered` (`cmd/mutate -covered`)
  adds **coverage-filtered mutation** = an **honest kill rate**: restricting fault injection to executed offsets
  removes dead-code dilution that deflates the naive number — on smoke.bin the same suite scores **2% naive vs
  68% covered**, discharging the playbook's flagged 5–20% thread. Self-tests both directions (matrix must
  distinguish rich/poor ROMs; covered kill-rate must exceed naive and be non-vacuous + deterministic). Pure Go,
  no reconnect. **Src:** DeMillo/Offutt (mutation); coverage-guided testing.
- **VV-12 ✅ DONE (v1.93.0):** `internal/framesim`+`cmd/framesim` — the **tolerant** lane beside exact
  `golden_frame`. Windowed **SSIM** over 8×8 luma blocks gives magnitude (mean) + locality (worst block); a DCT
  **perceptual hash** gives a shift-tolerant distance. A 1-pixel jitter that flips the exact rendering hash still
  scores SSIM ~1.0; a corrupted frame scores far lower (mc48 vs smoke = 0.08). `cmd/framesim` compares two frames
  (each a rendered `.bin` or a `.png`), JSON report, exit 1 below `-min`. It **adds** magnitude+locality; it does
  **not** replace the boolean exact golden. Self-test both directions (identity=1.0/0; tolerant to 1px; monotonic
  in damage; worst block localises; real cross-ROM measurably lower). Pure Go. **Src:** Wang et al. SSIM 2004; pHash.
- **VV-13 ✅ DONE (v1.94.0):** `internal/audiospec`+`cmd/audiospec` add the **frequency-domain** modality.
  `golden_audio` hashes the audio register chain and an RMS envelope says "how loud over time"; neither separates
  **inverted twins** — sounds with the same loudness contour but different pitch/timbre. A pure-Go radix-2 FFT
  magnitude spectrum + cosine **spectral distance** does, alongside an **RMS-envelope distance** and a
  dominant-frequency readout, over the captured PCM (`emu.AudioSamples`). The self-test makes the point numerically:
  two equal-amplitude tones at different pitch score **envelope distance 0.0000 vs spectral distance 0.9980**. The
  CLI separates real ROMs (music_driver's 523 Hz tone vs sfx_demo). Pure Go. **Src:** Cooley-Tukey FFT.
- **VV-14 ◑ PARTIAL (v1.95.0):** scoped to ①pure-Go prover precision + ③citable certificate (②Z3/SMT and
  ④external silicon-TIA ROMs stay deferred). **Key empirical correction (measured, not assumed):** the prover's
  over-warnings on the real technique kernels are **NOT** infeasible-path or page-cross artifacts (the prior memo's
  guess) — sweeping all 30 kernels found **0** whose conservatism is infeasible-branch-caused. Every over-budget
  region is on a kernel that runs at a verified-**stable 262 scanlines/frame**, so a region with worst W>76 cy
  **legitimately spans ⌈W/76⌉ scanlines** (the frame budgets for it; no roll). The honest, sound fix is therefore
  **`@lines`** (declare the true scanline span — the v1.89.0 mechanism), applied to the 9 affected kernels
  (multicolor48/score6/hscroll/bitmap48/two_line_vdel/zone_multiplex/tia_pcm/bullets/rpgmap): 5 now fully certify;
  4 clear their false-positive *violation* but stay UNBOUNDED on other regions. **③ `cmd/cpucert`** (+
  `cyclebound.Certify`) emits a reproducible, falsifiable certificate: per-region proven bounds + verdict, the
  `@lines` lemmas relied on, and provenance (prover version, Gopher2600 pin, DASM version, asm+ROM SHA-256). Exit 1
  if not certified; self-test both directions (smoke certifies deterministically; overrun rejected; tamper changes
  the hash). **Assessed and NOT built (measured 0/low real-kernel payoff):** ① the *display-off region
  reclassification* — instrumentation showed the VSYNC→VBLANK transition has VBLANK provably-unknown (init clears
  it; the first frame can briefly run with display on), so skipping those regions would be **unsound** (reverted);
  *value-range loop bounding* (S3) — the unbounded loops are nested (zone_multiplex's divide-by-15 inside an outer
  loop), hardware-timer waits (bitmap48 `INTIM`), or **JSR/RTS subroutine timing**, none of which a divide-by-15
  bounder fixes; *infeasible-branch pruning* (S4) — 0 real kernels need it. The remaining UNBOUNDED verdicts (JSR
  subroutine timing, nested loops, timer waits) are the **correct honest** outcome, not false positives; tightening
  them is a larger future lever (subroutine-timing modeling), kept deferred. C-3 ESIL/radare2 symbolic exec was
  **assessed and declined** (foreign Python+r2 runtime vs pure-Go; unvetted 6502 timing model) — on record so it
  isn't re-litigated. **Src:** Li&Malik IPET DAC'95; Ballabriga&Cassé WCET'08.

## Authoring aids (proactive, non-VV) — sprint 2026-06-18
A user-initiated complement to the VV-* verification backlog: where VV-* *observe/measure/judge a run*, these
*read intent before the run* and *interpret timing causality*. Four planned (AT-1 static timing linter, AT-2
write→visible-pixel timeline, AT-3 beam-race/too-late-write detector, AT-4 forward sprite-position solver).
**Quality bar: zero false positives on the known-good technique corpus** (same discipline as VV-2 green-ification).
- **AT-1 ✅ DONE (v1.98.0): static TIA-timing linter** (`cmd/timinglint` + `cyclebound.Lint`). Reuses the
  cyclebound decoder/`Instr`/srcmap and the absint value-range engine. Three high-confidence rules — `hmove-without-hmxx`,
  `hmxx-without-hmove` (value-aware: only provably non-zero staged motion warns; a `lda #0; sta HMPx` clear or an
  unknown value stays silent), `hmove-hazard` (HMxx/HMCLR write starts <24cy after HMOVE on a straight-line path;
  the `sta HMOVE; ds 12; sta HMCLR` idiom at exactly 24cy is safe). **Measured: 0 false positives on all 31
  technique kernels** (**2026-07-30**: still 0, and the denominator is now genuinely 31 — it had
  drifted to 30 analysed + 1 declined, see the per-bank linting entry below) (first sweep's 6 hits were two detector gaps — missed indexed `sta HMP0,x` stores, and a
  benign zero-clear — both fixed; a latent hazard false-negative in the cycle accounting also fixed). Litmus
  `lint_r1/_r2/_r3` + `lint_clean`; `TestLint*` lock both directions + the corpus guard. Pure Go, CLI only (no
  MCP/reconnect). **A single batched MCP exposure (AT-5) remains.**
- **AT-2 ✅ DONE (v1.99.0): write→visible-pixel timeline** (`cmd/beamtrace` + `internal/beamtrace`, new thin
  `emu.LastTIAWrite` accessor). Runs the ROM instruction-by-instruction, records every TIA write with the beam
  clock it lands at, and tabulates per scanline which visible pixels each write governs — the causal map
  `trace_clocks`/`read_row` only show piecemeal. Sound by construction: a write at clock C can affect a pixel
  only if rendered at clock ≥ C, and a later write to the **same** register supersedes it → governed span
  `[C, next-same-reg-write)`. Register name+kind table; pure strobes carry no value. Validated against the real
  `multicolor48` 48px kernel (staggered GRP0/GRP1 rewrites reproduced with correct interleaved spans; an
  HBLANK-superseded write shows an empty span). `TestTimelineSpans` (pure) + `TestTraceGRP0Marker` (fixture,
  deterministic + localized). Pure Go, CLI only.
- **AT-3 ✅ DONE (v1.100.0): beam-race / too-late-write detection — a SOUND dual, not a blanket detector**
  (`internal/beamrace` + scenario `checks.no_beam_race` + `cmd/beamtrace -race` + `emu.ObjectX`). A pixel-data
  write (GRP0/GRP1/ENAM0/ENAM1/ENABL) at clock C with the object at X reaches the beam iff C ≤ X. Two pieces:
  **(a) advisory `-race` report** — factual per-object in-time/LATE map, no verdict ⇒ cannot false-positive;
  **(b) `no_beam_race` check** — the author declares `{object, line_from, line_to}` (which object must update
  before the beam on which lines) and the check fails on any late write — sound because intent is supplied.
  Generalises the hardware-fixed `no_hmove_hazard` gate. Both directions locked (`beamrace_clean.asm` passes,
  `beamrace_late.asm` fails) at unit + scenario level.
  - **Measured design decision — no fully-automatic verdict (the key finding):** whether a late write is a bug
    is intent-dependent (same late `sta GRP0` = correct as next-line pre-load, wrong if meant for this line).
    Shown on the real `multicolor48` kernel: P0 at X=87, the 48px right-side GRP0 rewrites land "LATE" at clk
    +139/+157 — **correct facts, not bugs**; an automatic detector would false-positive there. So the verdict is
    opt-in (intent supplied) and the automatic part is advisory-only.
  - **DEFERRED, NOT CLOSED (per user, 2026-06-18):** a fully-automatic *heuristic* detector (guess intent, accept
    rare false positives, tune to minimise them on the corpus) remains a live future option should a concrete
    need appear — kept on the books deliberately rather than declared done. The substrate to revisit it
    (`beamrace.Trace` events carrying object X + before/after-beam) is already in place.
- **AT-4 ✅ DONE (v1.101.0): forward sprite-position solver** (`cmd/spritepos` + `internal/spritepos`). Target
  X (0..159) → routine input + div-15-coarse / HMOVE-fine decomposition + paste-able `SetXPos` snippet + the
  position the hardware ACTUALLY reaches (HmovedPixel). Clean-room on the verified `shared_setxpos.asm` idiom.
  **Soundness via measurement, not formula:** per CLAUDE.md the X(N) offset is kernel-specific, so `Solve`
  measures the offset on the emulator, inverts it, and re-runs to confirm. Measured X(A)==A exactly (slope 1,
  offset 0) for the calibrated routine; `spritepos -object BL -all` lands 160/160 targets exactly. `TestSolve*`
  (exact landing, emulator-verified) + `TestAchieveDiscriminates` (wrong input misses ⇒ non-vacuous) + a fixed
  `Decompose` carry bug (caught by the emulator disagreeing with the math). Pure Go, CLI only.
- **AT-5 ✅ DONE (v1.102.0): one batched MCP exposure** of the interactive aids — `beamtrace` (timeline),
  `beam_race` (advisory), `spritepos` (solver) added to `cmd/harness` in a single commit → `bin/harness` rebuilt
  → `scripts/mcp_smoke.py` extended to call all three → one reconnect. The linter (AT-1) stays CLI/CI-only (no
  MCP needed — proactive source check). **Authoring-tools sprint complete (AT-1..AT-5).**

## PONG dogfooding campaign — tool findings (live, 2026-06-18+)
Building a real PONG (`sandbox/practice/pong/`, structure = `apong_2.png`) while exercising + critically
evaluating EVERY tool (log: `sandbox/practice/pong/TOOL-EVAL.md`). Findings that became backlog/improvements:
- **framesim — scale-normalized screenshot compare: ✅ DONE.** `framesim -a rom.bin -b screenshot.png` errored on
  a bounds mismatch (1× ROM 160×N vs 2× shot 320×M); added `Resize`+`NormalizeSize` (downscale both to the
  per-axis min) so a ROM compares to a target screenshot. **Open:** normalize *vertical framing* too (VBLANK/
  overscan margins shift content; SSIM is discounted when the two don't cover the same visible region).
- **ingest/analyze_image — single-frame structure recognition on a PONG-class layout (open).** On `apong_2.png`
  it split the dashed net into ~24 separate "ball" sprites, classified the two symmetric scores inconsistently
  (one player, one PF), missed the full-width walls, and quantized white to $0A. Improvements: repeated-pattern
  (net) summarisation, symmetric/same-kind recognition, thin full-width wall detection, PF↔foreground split,
  B&W quantization. (The richer path is `fieldtest` multi-frame + `dissect` on the real ROM — to be evaluated.)
- **PONG-C1 — ✅ DONE (v1.103.0, 2026-07-03).** Shipped as `assert_edge_coincidence` (addrs+offsets coupled
  sweep, optional patch). Live-proven: detects a byte-faithful replica of the historical 77cy bug at PFp1;
  fixed kernel passes 139 reachable alignments. The rollout also exposed+fixed the RunUntilBudget warmup flaw
  (see CHANGELOG). Original design note kept below for provenance.
  Original: edge-coincidence fuzz case (from the 77cy latent bug, 2026-07-02). A per-line edge-compare
  kernel's true worst path = ALL edge variables on the SAME Y (+~5cy per hit) — free-run budget testing missed
  it for hundreds of frames (known-traps "N-edge coincidence"). Capability: a scenario/fuzz primitive that
  reads a declared list of "edge variables" (or auto-mines `cpy zp` targets in the kernel PC range), pokes
  them all to one Y (sweep Y), and runs `assert_line_budget` per alignment. Substrate exists (guidedfuzz,
  scenario poke steps, assert_line_budget) — this is a targeted generator, not a new engine. Size: S-M.
  Rollout per the interactive full-integration rule (implement → MCP/scenario wiring → smoke → live-use proof).
- **PONG-C2 — ✅ DONE (v1.103.0, 2026-07-03).** Shipped as `patch`/`pokes` params on `assert_line_budget`
  (+`patch` on `assert_edge_coincidence`); symbol resolution via `srcmap.Symbol`. Live-proven: full 600f
  lightweight-table run in one call, original ROM byte-identical afterwards (peek $F000=45 + file md5).
  Original design note kept below for provenance.
  Original: `assert_line_budget` temporary-patch option (from the XTable-swap ritual). During PONG every
  budget run required: hand-edit XTable to light values → assemble → assert → restore → re-assemble (done
  ~15×; one forgotten restore = shipping a wrong ROM). Capability: `assert_line_budget` (and
  `prove_line_budget`) accept `patch: [{addr|symbol, bytes}]` applied to a COPY of the loaded ROM for the
  measurement only, auto-reverted after — kills the ritual and the restore-forgetting failure mode.
  Needs symbol lookup from the DASM listing (or raw addr). Size: S. Same interactive rollout.

- **PONG-C3 — per-line WORST cycle count, not just pass/fail (from the pf2-06 feel-pass, 2026-07-03).**
  `assert_line_budget` answers "did any line exceed budget, and where's the first one" — a boolean + the
  offender label + `line_cycles` which, on an overrun, reads `152` (= it spilled to a 2nd WSYNC line), NOT
  the real cycle count. So when a physics row lands at ~76-78cy you cannot see *how much* to trim; you binary-
  search budgets (76 over / 75 not) or hand-count cycles — and hand-counting was wrong by ±2cy repeatedly this
  session (page-cross branch, an `ld_` clobbering a flag), costing ~4 assemble/assert iterations per row. The
  root friction: **the overscan physics rows aren't at fixed scanlines** (their Y drifts frame to frame), so
  `beamtrace`/`step_scanline`/`breakif(scanline)` are awkward to aim at "row 5 on its worst frame," and
  `breakif(until_scanline)` did not halt as expected on the drifting overscan (unconfirmed — possible tool
  quirk worth a separate look). Capability: report, per **labeled line** (start label → next WSYNC), its
  **max cycles across N frames** + the frame/Y where it peaked + the arg values that produced it — i.e. the
  quantitative sibling of the boolean `assert_line_budget`. Ideal: fold into `prove_line_budget` (static ∀
  over paths already knows each line's exact worst — just surface the number per line instead of a global
  verdict). That turns "trim by guess-and-assert" into "trim by the exact margin." Size: S–M (the cycle
  accounting exists in `internal/cyclebound`/`emu`; this is mostly surfacing it per-line). Same interactive
  rollout. **This is the highest-leverage tool gap the feel-pass surfaced.**
  - **More evidence (AI-variants build, 2026-07-06):** building 3 swappable row-4 AI kernels hit the exact
    same guess-and-assert loop repeatedly — v4's rubber-band overran because a clamp-high + loose-error +
    up-move path combination landed ~2cy over (the design-time hand estimate said 61cy, real was ~78), and
    v2's predictive kernel overran when a +8cy accuracy tweak pushed one of ~6 branchy paths over. Each cost
    a full assemble→assert→re-trim cycle to find, and the fix was structural (move work to a slack row).
    A per-line worst-cycle readout (with the winning path's arg values) would have shown "row4 worst = 78cy
    on the loose+clamp-high+up path" immediately. Strengthens the case: this recurs on *every* budget-tight
    kernel, not just PONG's physics rows.

- **PONG-C4 — gameplay-behavior verification: headless match harness (AI strength / physics invariants /
  fairness) (from the C1 objective bench + round-robin, 2026-07-11).**
  The harness is strong on timing/rendering (assert/prove_line_budget, beamtrace, framesim, …) but has no
  primitives for *game behavior*: PONG's AI, serve, physics and fairness were all verified with hand-rolled
  pokes and ad-hoc free-runs. The C1 campaign (`sandbox/practice/pong/ai-variants/bench/` + `matchup/`)
  exposed both the need and the method — and its central caveat:
  - Building it by hand meant **one hand-edited ROM per pairing** (4 bench variants + 7 matchup ROMs, each
    transplanting an AI into the left-paddle input path). That's the ritual to kill.
  - A single fixed baseline opponent produced a ranking (v4>v1>v3≈v2) that the round-robin **refuted**
    (true head-to-head order v3≈v4>v2>v1; strength is non-transitive — a v1→baseline→{v2,v3}→v1 cycle). So
    the capability must support **N×N tournaments**, not a lone benchmark opponent, and the honest output is
    the matrix, not a scalar rank.
  Capability sketch — input: ROM + a declared "actor interface" (which RAM addr / input register the harness
  drives for one side), a parameterized scripted policy (k-px tracker with delay/error knobs), match rules
  (first-to-N via BCD score addrs, or fixed frames), optionally a set of ROMs/policies for round-robin.
  Output: per-pairing final scores, the tournament matrix, per-match traces (points timeline, rally lengths)
  — typed JSON numbers like every other tool. Second half: **behavioral invariants under long free-run fuzz**
  (ball-speed bounds, paddle range, score monotonicity/BCD validity, serve fairness left vs right) = the
  physics/fairness siblings of the same primitive.
  Dedup: `run_scenario` has input timelines + assertions but no closed-loop policy driving; `guidedfuzz`
  drives inputs but coverage-guided, not policy-scripted; `trajdiff` compares vs a reference ROM, not vs a
  policy. Caveat baked in from C1: any scripted opponent is ONE lens (objective strength ≠ human difficulty:
  v3 measured 1-11 vs a perfect 1px tracker yet plays "medium" for a human) — the opponent model must be an
  explicit, swappable parameter. Size: M (frame-loop input driving exists in the scenario substrate; the
  work is the actor-interface declaration + match bookkeeping + matrix reporting). Registration only —
  implementation is a separate approval.
  Provenance: `sandbox/practice/pong/ai-variants/bench/README.md` (baseline bench + round-robin tables,
  37000f v3–v4 stalemate, side-bias check via left/right transplant) + `docs/casebook.md` PONG section.

## How this stays finite & honest
Provenance is attached to every item (papers/tools/in-tree symbols). Each is de-duped against the existing
surface (testing-playbook methods, `stellacheck`, litmus V2-1…18, golden/regress/refdiff, G1–G14) — see each
angle's dedup note (now folded above). The list is **ranked and bounded (14 items, best first)**; implement
concrete-driven, top-tier first, not all at once. Implementation of any VV item is a **separate approval**.

---

# Doc-consolidation note (2026-06-17)

**This file is now the single live backlog.** The former `improvement-roadmap.md` and `hardening-roadmap.md`
were retired here (the user's "no duplicate files / 1 source" discipline): both had become ~95% "✅ DONE"
logs duplicating `CHANGELOG.md` (the canonical version history) + git. Their delivered work stays in the
CHANGELOG; the only still-open carryovers were already tracked above as **G5** (mid-line HMOVE/RESP
litmus-lock) and **G7** (`step_clock` color-clock granularity · `watch/trap` on bus/collision — the ex-
`hardening-roadmap` F-5 stubs). `improvement-roadmap`'s game-side items (Freeway port, audio recipes) are
roms-side builds, not harness capability gaps. The verified-facts catalog remains `verified-coverage.md`; the
knowledge-state audit remains `fundamentals-audit.md`.

## Combat structure-comparison — capability candidates (2026-07-22)
Three harness-capability candidates surfaced by an efficiency/structure comparison of a self-authored Combat clone (`combat_mine` 4K) against the original Wagner 2K ROM (`sandbox/studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`). Clean-room note: citing the original disassembly here is the casebook contract. These target **integration-under-budget** capability (see `design-principles.md` "Structure & efficiency rules from the Combat disassembly comparison"). **Registration only — each implementation is a separate approval.**

- **CMB-1 — structural-efficiency lint: flag inlined code that could be one `,X`-indexed loop when it runs in *blanked* (non-beam-critical) time.** The comparison measured **~250-400 recoverable ROM bytes** that are **not** a provability trade — pure duplication the original avoids by running all four moving objects through a **single `,X` path over `DIRECTN[0..3]`**. The clone instead **4×-inlines** friction/accel (~120-200B) and **duplicates** missile fly/kill ×2 + sound ×2 (~60-100B); the key qualifier is that this code runs in **overscan/VBLANK** so the 76cy budget does **not** bind → indexing it is "small **and** free." Capability: a static lint (reusing the `cyclebound` decoder + `srcmap` + absint, like `timinglint`/AT-1) that detects N near-identical straight-line blocks differing only by a base address/offset, confirms they sit in a provably-blank region (blank-region ∀ classification already exists — **v1.106.0 PONG-C3/VV-2b**, do not re-file), and advises collapsing to a `,X` loop. A **ROM-size/structure** lint (distinct from `prove_line_budget`'s cycles). Zero false positives on the known-good corpus (AT-1 discipline). Size: M. 〔Combat `DIRECTN`/`MVtable`/`MVadjA`/`MVadjB`; comparison §2.4/§4/§6①/§7, diff-gaps GAP-3〕
- **CMB-2 — `INTIM`/`TIM64T` "fixed-picture-start" detector + advisor.** The original times its VBLANK with a RIOT timer (`VCNTRL` loads `TIM64T=43`, then polls `INTIM`) so the **picture starts on a fixed line while VBLANK logic time is free to vary** — logic growth auto-absorbed. The clone relies on a **hand-tuned fixed WSYNC count + elastic pad**: correct today but "screen-dip fragile." Capability: a static/runtime advisor that (a) **detects** the fixed-WSYNC-count-plus-pad pattern (a counted `sta WSYNC` sequence framing VBLANK with no `INTIM` poll) and (b) **advises** the timer load-leveling idiom. Sibling to but distinct from **G8/VV-10 T-1** (the timer-*wrap* HW-trap detector — that guards a hazard; this is an authoring-robustness advisor). Reuses the `Emu.TimerState` exposure built for T-1. Size: S-M. 〔Combat `VCNTRL`/`INTIM`/`TIM64T=43`; comparison §2.1/§6⑥/§7, diff-gaps 追加ディテール〕
- **CMB-3 — collision-face / wall-normal estimation aid.** The TIA reports **THAT** an object hit the playfield (`CXP0FB`/`CXM0FB`) but not **WHICH FACE** — so a correct maze-wall bounce cannot be computed in one frame. The original reconstructs the normal with a **multi-frame trial-and-error solver** (`MxPFcount`: frame0 vertical → frame1 flip 180° → frame2 wait → frame3+ corner, held until clear; `COLcount` ignores sub-few-frame contacts). A genuinely hard, under-specified problem the clone never solved (its collision path is a last-safe-position restore, no reflection). Capability: a harness aid — static (a `pkg/design`/`docs/techniques` state-machine skeleton) or runtime (a scenario primitive that drives an object into a PF wall and **verifies** the reconstructed bounce direction against the geometric normal) — to help **author and verify** such a solver. Distinct from **G7** (RAM/bus collision *trap*) — this is the *semantics* of wall-normal recovery. Concrete-driven (build when a maze/bounce ROM needs it, cf. G9). Size: M. 〔Combat `MxPFcount`/`COLcount`/`CXP0FB`/`CXM0FB`; comparison §2.6/§7, diff-gaps GAP-4〕

## Combat deep-read — capability candidates (round 2, 2026-07-23)
Four more harness-capability candidates from a 5-lens deep read of the original Wagner Combat.asm (game-design / 6502-craft / audio / anti-patterns / clone-novelties), surfacing gaps the round-1 efficiency/structure comparison structurally could not see (esp. **audio** and **memory-layout traps**). Distinct from CMB-1/2/3 above. **Registration only — each implementation is a separate approval.**

- **CMB-4 (CMB-AUDIO) — temporal-audio assertion: the audio analog of `assert_line_budget`.** `read_audio_trace` (v1.104.0) *reads* AUDC/AUDF/AUDV per-frame over N frames, but there is **no executable assertion over an audio trace**. Combat's key sound behaviors are all **temporal invariants** a linter could check numerically: (1) AUDF descends monotonically across successive rally bounces (rising pitch — a gameplay counter aliased into AUDF); (2) AUDV decays monotonically after an explosion trigger (a physics countdown `LSR`×3 → AUDV, half-way delayed-onset gate); (3) engine AUDF is a pure function of (craft, velocity, player) matching the pitch curve + fixed +2-per-player detune; (4) channel-owner uniqueness (one-object-per-channel, last-writer-wins). Capability: `assert_audio_envelope(channel, direction, over=N)` + `assert_channel_owner`, over the existing `read_audio_trace` substrate. **Distinct from VV-13** (`audiospec`, two-run spectral/RMS compare) **and G3** (PCM fidelity) — this is a single-ROM temporal-invariant assertion. Planted-discrepancy self-test (VV discipline). Size: S-M. 〔Combat `BounceCount`→AUDF (MOTORS ~1022), `BoomSnd` StirTimer `LSR`×3→AUDV (~812-818), `SNDP` 3×12 + per-player detune, MisFly/MotMis/BoomSnd last-writer dispatch; `read_audio_trace` exists; deep-read harvest 2026-07-23〕
- **CMB-5 — gap-fill model for finalize-step MD5 / byte-diff.** A **byte-faithful disassembly still fails MD5** against the original cart: the unwritten bytes between end-of-data and the startup vectors are **$FF** on real hardware (unprogrammed PROM) but **DASM zero-fills them ($00)**. So a "perfect" reassembly mismatches the reference MD5 purely on gap/pad fill, not code. Live trap here because **finalize-step registers MD5s in Stella** — a correct reconstruction would be flagged wrong. Capability: a gap-fill-aware byte/MD5 differ that (a) models the **$FF-for-PROM** fill before trusting an MD5, and (b) treats a nonzero diff **confined to unwritten regions** as *expected*. Size: S. 〔Combat intro annotation (Williams: recompiled image "differs… only in the few unwritten bytes… DASM sets these to zero… in the cart they are $FF"); ties to finalize-step MD5 registration; deep-read harvest 2026-07-23〕
- **CMB-6 — assembler/linter WARN: table or data bytes placed over the $FFFA-$FFFF vector region.** Combat squats **live game data on the CPU IRQ/BRK vector slot**: it `ORG`s vectors at $F7FC (a 2K cart mirrors $F000-$F7FF into $F800-$FFFF, so $F7FE/$F7FF *are* the $FFFE/$FFFF vector) and reads its own vector bytes as a data table (`LDA AudPitch,X`). It survives only because START `SEI`s and never takes BRK. This booby-traps a 2K→4K port. Capability: a harness assembler/linter **WARN** when an `ORG`/data directive lands bytes over $FFFA-$FFFF (accounting for the 2K/4K mirror). Zero false positives on the known-good corpus (AT-1 discipline). Size: S. 〔Combat `ORG $F7FC` / `AudPitch` at $F7FE (annotator: "move AudPitch out of the interrupt vector"); deep-read harvest 2026-07-23〕
- **CMB-7 (negative / boundary fact) — Combat has ZERO self-modifying code; all mutable state in ~26 zero-page bytes.** A whole-file scan finds no classic SMC (no writes into instruction operands, no code from RAM; single `ORG $F000`, never rewritten). Nearest analogues are pure **data** ops (`MVadjA`/`MVadjB` rotated in-place via the `ROL`-MSB-in-carry idiom; `SCROFF` `INC`). Recorded as a **calibration boundary** so a future SMC-lint does not false-positive Combat — this era's density came from packing/aliasing/branchless masks, **not** SMC. Not a capability to build; a truth one must respect. 〔whole-file scan; `MVadjA`/`MVadjB`, `SCROFF` INC, single `ORG $F000`; deep-read harvest 2026-07-23〕

## Static x dynamic program model (SD-*) — 2026-07-29

A survey (8 agents: local assets, the vendored Gopher2600, DiStella/Stella, the external ecosystem, PL
techniques; then a catalogue and two adversarial critiques) produced this backlog. Two conclusions worth
recording before the items:

- **No external analyser is worth adopting.** Ghidra/SLEIGH, Binary Ninja and angr have no cycle model at
  all, so three of the five questions we care about are outside what they can express. radare2 has 6502
  cycles but its page-crossing penalty is a FIXME in the source — unsound in exactly the case a 2600 kernel
  cares about — while the Gopher2600 table already in tree is sound. The only external binary worth
  considering is z3 as an SMT-LIB subprocess, the same shell-out pattern already used for perfect6502.
- **The vendored `Gopher2600/disassembly` package is unimported** (verified by grep across cmd/ and
  internal/). It recovers a CFG from a raw .bin, separates code from data by following flow, resolves
  indirect jump targets, and carries a Decoded/Blessed/**Executed** confidence ladder — the static-vs-dynamic
  distinction we need, already modelled. Its blessed set is a heuristic, so it is a cross-check and a data
  source, never the sound denominator.

### SD-0 — Soundness and honesty repairs (blocking; do before anything is built on top)
- **SD-0a `applyStore` has no kill-set.** `internal/cyclebound/absint.go:336-369`: `storeAddr` returns a
  location only for Absolute mode, so `sta $80,x` and `sta ($90),y` leave previously-tracked `State.Mem`
  cells intact and a later absolute load reads a stale constant. That range reaches `refineBranch`,
  `determineBound` and `pagePenalty`, so a proven worst case can sit BELOW the machine's. Stack pushes are
  unmodelled and land in the same 128 bytes. Fix: invalidate the may-set of every non-Absolute store.
  Demonstrate the unsoundness with a planted case before fixing, and keep that case as a regression.
- **SD-0b `LastTIAWrite` mis-attributes indexed TIA stores.** `internal/emu/emu.go:1350` maps
  `lr.InstructionData` (the raw operand) rather than the effective address, so `sta RESP0,x` is reported as
  a write to the base register. `beamtrace` inherits this, and it breaks precisely on the multiplexed-object
  kernels the tool exists for. Fix: route through `effectiveAddr`; add a litmus with an indexed TIA store.
- **SD-0c Address canonicalisation.** DONE for the decoder. `program.canon` folds a CPU address to a
  cartridge offset, deciding cartridge space through Gopher2600's memory map first and masking only then
  (masking first would fold RAM/TIA/RIOT into ROM and decode whatever was there). Measured before/after on
  real cartridges: **Outlaw 2K went 0 entries / 0 instructions -> 1 / 931, Combat 2K 0 / 0 -> 2 / 838**;
  every 2K cartridge previously decoded to nothing at all, and reported that as an analysis. 4K unchanged
  (Frogger 152 instructions either way); the .asm path is byte-identical, 14/31 certified with a zero diff *(15/31 as of 2026-07-30 — SD-6)*.
  REMAINING: the same canonical key is not yet used by `emu.Coverage` (which records the raw PC), so a
  static-minus-dynamic subtraction still compares different address spaces. That matters when the
  "statically reachable but never executed" report is built — see SD-0e.
- **SD-0d Non-convergence is silent.** `absint.computeStates` returns after `maxIter` without signalling it;
  unconverged cells are under-approximations consumed as sound. Fix: return a converged flag and refuse to
  certify without it.
- **SD-0e `cover` divided by branches OBSERVED — DONE.** A branch never reached left the arithmetic
  entirely, so the percentage ROSE as the test got worse. Measured on this repo's own kernels before the fix:
  `divtable` reported **100% edge coverage with 12 of its 17 branches never executed**; `maze`, `hscroll` and
  `multicolor48` also read 100% with a third of theirs unreached; `game_states` read 60% where the honest
  figure is 30%.
  `cyclebound.StaticBranches` decodes the cartridge from its vectors and gives the branches the PROGRAM has.
  `cover` now divides by that, keeps the old number alongside as `edge_coverage_observed` so the difference
  is visible rather than silently corrected, and names the **unreached branches** — the actionable half.
  When an executed branch was never decoded (bank switch, computed dispatch) the denominator is too small
  and the figure is an over-estimate: `decoder_incomplete` plus a note say so out loud instead of leaving it
  to be inferred from a percentage above 100 (`banked_game` reads 150%, which is the diagnosis, not a bug in
  the arithmetic). Measured: 31 ROMs, 2 with executed-but-undecoded branches (`banked_game`,
  `rts_dispatch`); static <= observed coverage holds on all 29 fully decoded ROMs.

### SD-1 — Static def-use / reaching definitions (selected 2026-07-29)
The forall answer to "which instruction wrote which RAM/TIA address, in what order within a frame,
including read-before-write in a shared block". Both critics flagged its absence independently: every other
option either declines the question or answers it only for the runs that happened to execute. Builds on the
CFG already recovered by `cyclebound.decodeInto`, the WSYNC-to-WSYNC region partition (which is exactly the
"within a frame" ordering scope), the existing worklist fixpoint, and the store-address machinery.
May/must semantics per address. Soundness is checkable, and must be checked: the static may-write set must
CONTAIN every write observed by a real trace.

### SD-1b — Loop-aware must-write — DONE
`findSweeps` recognises the counted sweep loop (`ldx #N / sta base,x / dex / bne`) and treats the loop's
FALL-THROUGH edge — never the back edge, which would claim at the header that the whole range is already
written — as a must-write of the swept range. The fencepost is the substance: `dex/bne` leaves once the
index reaches zero, so index 0 is never stored and the range is base+1..base+N; `dex/bpl` leaves only when
the index goes negative and does include zero.

Measured across the technique corpus plus litmus: uninitialised-read reports went **3783 -> 0 false**, while
the planted case still fires. The pair `roms/litmus/litmus_uninit_read.asm` (clears $01..$3F, then reads $8A)
and `litmus_uninit_clean.asm` (identical but for the sweep bound) proves both directions — one report on the
bait, none on the twin. Cross-checked against the emulator: every uninitialised read `emu.WatchUninitRead`
observes must appear in the static set (32 ROMs run).

Also added while doing it: a bank-switched image (larger than the 4K the console addresses at once) is now
DECLINED rather than analysed. `banked_game.asm` was decoding 66 instructions from entry points $FFE0/$FFFF
and producing a confident finding about an address it had never decoded. `DefUseReport.FlatBankOnly` says so.

### (superseded) SD-1b — original note
`DefUse` computes reads that no path from reset has definitely written first, and does not report them,
because both scopes tried produce noise rather than findings — measured, not guessed:
per-WSYNC-region, 515 flagged addresses on one technique kernel (a kernel reads what its setup wrote);
whole-program from reset, 3783 on `dyn_multisprite`, 506 on `maze`, 128 on `rts_dispatch`.
The cause is single: must-write counts only exactly-targeted stores, and the RAM clear every 2600 ROM opens
with (`ldx #$FF / sta $00,x / dex / bne`) is an INDEXED store, so the analysis never learns RAM was
initialised. Fix: recognise the sweep idiom and treat the loop EXIT as a must-write of the swept range —
noting the fencepost, since `bne` leaves before storing at index 0, so the swept range is $01-$FF and not
$00-$FF. `zpInitRange` (absint.go) already detects this idiom for the value domain and is the place to start.
Until then the dynamic counterpart `emu.WatchUninitRead` answers the question for the runs it executes.

### SD-1c — CFG reach gaps found by the def-use soundness sweep
The containment check (static may-set vs what the machine actually wrote) reported 9053/9053 across the
31-kernel corpus, and as a by-product named where the CFG never arrives: 306 writes in `banked_game` and
`rts_dispatch` come from instructions the decoder never decoded. Flow from the reset/NMI/IRQ vectors cannot
follow a bank switch or an RTS-computed dispatch. Not a wrong prediction — a hole in reachability, counted
and reported separately so neither hides the other. Fixing it is the stack-modelling and bank-hotspot work
described in SD-3.

### SD-4 — RESOLVED: it was the measuring instrument, not the proof
Extending the observed-vs-proven cross-check from one litmus to the whole corpus produced what looked like
the worst possible finding: `bitmap48.asm` region `Krow` proven at 93 cycles while the machine took 911,
spanning 12 physical lines. A proven number the hardware exceeds is worse than no number, so this was
recorded as the top item — and then measured rather than believed.

`emu.ProfileLineWorst` detected a WSYNC by the RdyFlg true->false transition, i.e. by the CPU stalling. A
WSYNC whose stall is shorter than one instruction step never shows that transition, so the strobe is
invisible; the interval it should have closed runs on to the next visible strobe and is reported as one huge
region. Measured: `$F0D0` executes **192 times in 8 frames** and the transition method counted **108**
intervals — 84 strobes missed, and the longest bogus interval spanned 13 lines.

Detecting the WSYNC **store** instead fixes it, with one subtlety worth keeping: `LastResult` does not change
while the CPU is stalled, so the check must be restricted to steps that actually retired an instruction or
the same strobe is seen again on every step of its own halt (that draft closed a string of zero-length
intervals and was caught by the exact-match litmus). After the fix, `$F0D0` counts **184** = 192 executions
minus the 8 dropped at frame boundaries, and its worst is **87 cycles over 2 lines**, matching its `@lines 2`
annotation and sitting inside the proven 93.

The corpus check is now clean with no exceptions: 215 measured regions across 30 ROMs, observed <= proven.
The "fails if a listed gap stops violating" guard caught its own obsolescence within the hour, which is
exactly what it was for.

### SD-5 — Call-context resolution for a region opened inside a subroutine — DONE
(The note below is kept because the sequence matters more than the outcome.)
A WSYNC inside a callee opens a region whose continuation lives in the caller, so with no call context the
walk hits an RTS, finds no WSYNC, and the region is unbounded. `callContexts` enumerates each JSR's return
address whose callee body contains the region start, re-analyses the region per context, and takes the WORST
— every context must resolve, since a region provable from one call site and not another is not provable.

Measured: unbounded regions **29 -> 24**; certified unchanged at 14/31; observed <= proven on **223** measured
regions across 30 ROMs with no exceptions.

**This was reverted once, wrongly.** The first attempt was judged unsound because `text12`/`text24` region
`KrowA+36` was reported bounded at 110 while the profiler measured 143. The profiler was the thing that was
broken (SD-4): with WSYNC detection fixed, the same region measures **104** against the proven 110. A correct
improvement was nearly discarded on the evidence of a faulty instrument — the same instrument that had also
made the prover look unsound. Check the instrument before believing what it says about the thing under test.

### (superseded) SD-5 — original revert note
A WSYNC inside a callee opens a region whose continuation lives in the caller, so with no call context the
walk hits an RTS, finds no WSYNC, and the region is unbounded. Measured: 5 regions fail as "no WSYNC reached"
and 3 as "RTS with no caller in context", across bullets/game_states/road/shared_setxpos/text12/text24.

An implementation that enumerated each JSR's return address, re-analysed the region per context and took the
worst brought unbounded regions from 29 to 24 — and was **unsound**. The corpus cross-check caught it
immediately: `text12`/`text24` region `KrowA+36` was newly reported as bounded at 110 cycles while the
machine takes 143. Reverted the same block it was written in; the numbers are recorded here so the next
attempt starts from the failure rather than repeating it.

Likely cause to check first: taking the worst over JSR return addresses assumes the region is entered only
through a call, but these kernels re-enter the routine from a loop in the caller, so the continuation crosses
the call boundary more than once and a single-level return context under-counts it.

### SD-2 — Beam position as a tracked interval
Carry colour-clocks-since-WSYNC in the abstract domain so every TIA write gets a proven clock interval on
every path. Nothing in the community or the RE ecosystem has this; the state of the art is hand-counting a
single path. Two costs the first sketch missed: an interval needs a LOWER bound too, and no shortest-path
solver exists (only `solver.longest`); and clocks-since-WSYNC is unbounded, so it breaks the termination
argument and needs an explicit widening. SD-0d first.

### SD-3 — The limits, and how far each can actually be pushed
Recorded because three of these were previously mistaken for hard limits when they are missing features.
- **Semantic role of a byte** — recoverable by triangulation rather than by any single tool:
  `probe_ram_semantics` (poke it, see what moves) x `DecomposeRow` (which object drew those pixels) x SD-1
  (which TIA register the byte flows into). Three independent witnesses that can disagree, which is what
  makes the answer falsifiable. What stays out of reach is naming in the work's own vocabulary, not the
  functional role.
- **Stack-pointer tracking — DONE.** `absint.State.SP` is tracked through TXS/TSX/PHA/PHP/PLA/PLP/JSR/RTS,
  and a push now resolves to `$0100+SP` wherever SP is known. Before this, BOTH sides of the harness were
  blind to a write the hardware performs: `emu.LastTIAWrite` reported zero COLUBK writes for
  `roms/litmus/litmus_stack_trick.asm` while the rendered background was green, because `effectiveAddr`
  treated an implied opcode as touching no memory. Static and dynamic now agree exactly — PC `$F023`,
  COLUBK, value `$C4`, clock `-11` — and the picture arbitrates. A push with SP unknown still drops the
  whole tracked map; with SP known it drops only the cell it wrote. Prover unchanged (14/31, zero diff);
  def-use containment 9053 -> 9055/9055; beam containment 7117/7117.
  **(2026-07-30: both gradings ran on the 31 technique kernels only. Extended to the whole corpus — 128
  images — they read 32655/32655 and 19143/19143, still zero violations, for 4.7s and 3.2s. Roughly 3x
  the evidence at no cost, and the same 'denominator smaller than it looks' shape found three times that
  day: timinglint at 0/133 instructions, 38 of 95 scenarios never run, blank grading on 32 of 129 ROMs.)**
  REMAINING here: JSR/RTS move SP by two but the pushed return address is not modelled as a write, and an
  interrupt would move SP without being seen at all.
- **Self-modifying code is detectable even when not resolvable** — a store whose effective address lands in
  decoded code space is a fact, not a guess. Correct terminal state is "region suspended", never a guess.
  **Scoped by measurement 2026-07-30, and not built yet — deliberately.** Ran `DefUse`'s may-write set
  against the cartridge window over the whole corpus: **133 ROMs, 0 that write into it.** Building the
  detector now would add a branch with no witness, which is what every sweep this week has penalised, so
  the detector and its planted fixture have to be built together — a ROM that stores into its own decoded
  code space, twinned with one that does not, in the pattern `cb_deadpred`/`cb_pushdisplay` use.
  **The precondition nobody had stated — now cleared for `Prove`.** The ROMs where this matters are
  commercial images, not our own kernels, and the static tools could not read one: `Prove` and `timinglint`
  ASSEMBLE their input, so a raw `.bin` came back as "Unknown Mnemonic" (measured on Adventure, Seaquest,
  Chopper Command, VideoOlympics, Empire Strikes Back). SD-0c's raw-image work was in the DECODER and
  unreachable from any public entry. `Prove` now takes a `.bin` directly — VideoOlympics 8 regions,
  Adventure 14, Seaquest 49, Chopper Command 29, all converged, with the `.asm` path byte-identical.
  `timinglint` still assembles; same one-branch change when it is wanted.
- **Conditional bounds — DONE for the dominant case.** Measured first: of 29 unbounded regions across the
  technique corpus, **15 fail for one reason — "loop bound unknown"**. The body of such a loop is fully
  understood; only its trip count is missing, so the region's cost is still a known function of it and the
  largest count that fits the budget is computable. `Region.Conditional` now carries that:
  *"within 76 cycles provided the loop at $F126 runs at most 11 time(s) (worst 75 there)"*.
  14 of the 15 produce a statement; 5 of those report that the region misses its budget even at ONE
  iteration, which is a finding in itself rather than a refusal.
  The number is checked for TIGHTNESS by re-deriving both edges independently of the search that produced
  it (n fits, n+1 does not) — a bound that is merely safe would send an author trimming work that was never
  over budget. And it never certifies: `Bounded` stays false and 14/31 is unchanged with a zero diff,
  because an obligation is something to discharge, not a proof.
  REMAINING: the other refusal reasons — no WSYNC reached (5), RTS with no caller in context (3), WSYNC
  inside a loop body (2), multiple back-edges (2), branch inside the loop body (1). Also, discharging an
  obligation automatically (prove the trip count elsewhere, or emit a runtime assertion) is not built.

## Housekeeping backlog (docs/repo, not a harness capability)
- **DOC-EN — translate the JA-heavy canonical docs to English** to finish the public-repo English-only pass
  (`design-principles.md` ~98 JA lines, `casebook.md` ~39, `build-to-learn.md` ~33). Deferred from the
  2026-06-17 docs cleanup (which dropped the 13 `.ja.md` duplicate files); these `.md` bodies are the *only*
  copy so they were left intact, but the user asked that the English-ization "reliably happen later" — tracked
  here so it is not dropped. `mining-digest.md` is **excluded** (generated from Japanese-source thread data —
  translating would break source fidelity). Size: medium; needs review. Separate approval.

## Reproduction-loop backlog (RL-*) — 2026-07-29
The clean-room reproduction loop tools (`docs/reproduce-loop.md`): **`vismatch`** (palette-independent
per-pixel object-attribution visual diff + `-genpf` PF-table generator), **`behavmatch`** (scripted-input
behavioural diff), **`framegen`** (from-scratch pixel-exact static-frame generator). **Field-evaluated** on
a second game (Pizza Boy Tokyo) — the running log + all 7 enhancement asks are in
`sandbox/practice/pizza-boy-tokyo/TOOL-EVAL-reproduce-loop.md`, promoted here as the canonical backlog.
**Validation stands:** vismatch found a systematic **+8-scanline** vertical shift that eyeballed screenshots
missed; `-genpf` gave the exact band positions that corrected a hand-model that was **3px (−11 vs −8) wrong**
→ byte-exact PF; matched-state measurement pinned bike top at abs sl176 vs a comment that mis-stated the
offset by 13 lines. The tools do what they were built for: **numbers close holes the eye cannot.** Ranked:

- **RL-5 — `behavmatch` warmup/frame-offset flag — DONE, and this entry was the stale one (2026-07-30).**
  The original text: `behavmatch` drives both ROMs from frame 0, so a game with a **title screen that
  auto-advances to gameplay** (most commercial ROMs) is measured on its TITLE screen, not gameplay —
  apples-to-oranges, every scenario reads MECHANIC DIFF (observed live on Pizza Boy: target P0 Y=29-46 =
  title vs mine=gameplay). `vismatch` has `-target-frames`; `behavmatch` has "no equivalent".
  **It has had one.** `-target-warmup N` / `-mine-warmup N` exist in `cmd/behavmatch`, with help text naming
  the title-screen case. Verified by measurement rather than by reading the flag list, because a flag that
  exists and does nothing looks identical from the outside: running Outlaw against **itself**, equal warmups
  report `behaviour matches`; `-target-warmup 60 -mine-warmup 0` reports `behaviour differs` with
  `P1 Y` and `M1 Y` at `**MECHANIC DIFF** [pos offset -99]`. The flag changes what is measured.
  〔TOOL-EVAL V4 / idea #5〕

  **Why this one is worth recording.** Every other RL-* item is marked DONE; the single entry that was not
  is the single one that already was, and it was the one ranked ★ top priority — so it is what any reader
  picks up first, which is exactly what happened when this was checked. A backlog that says a capability is
  missing when it exists costs the same as a description that denies a capability the tool has
  (`analyze_image`, fixed the same day): the reader does not go looking. Spot-checked the other direction
  too — `-format pf012` (RL-1), `-target-until-gameplay` (RL-4) and `-export-scenarios` (RL-6) are all
  present, so the DONE marks are not the failing side.
- **RL-1 — `-genpf` full-width PF output mode — DONE.** `vismatch -genpf -format pf012` measures the
  playfield across the FULL line width and emits `PF0tab/PF1tab/PF2tab` as `ds N,$XX` runs, classifying the
  right half as repeat / reflect / asymmetric from the pixels (an asymmetric target also gets right-half
  tables, with the note that landing those writes inside the line is a cycle-budget problem the generator
  does not solve). The measurement already computed every byte; the Outlaw path simply discarded four of the
  six. Verified against an INDEPENDENT oracle rather than against its own decoding: the generated bytes must
  equal what the machine holds in PF0/PF1/PF2 while drawing that line — 108 scanlines across 27
  repeat/reflect kernels, zero mismatches. The oracle's scope is stated in the test rather than assumed:
  2837 further lines are unreadable because a WSYNC ends the line before both samples can be taken.
  FOLLOW-UP: a stronger oracle exists — `beamtrace` already computes, per write, the visible span it
  governs, which would give the drawing value for every line instead of only the ones without a mid-line
  WSYNC.
- **(original) RL-1 — `-genpf` full-width PF output mode.** `-genpf` measures band positions + per-band
  clk-spans **perfectly**, but emits **Outlaw-specific** `CacLTbl/CacRTbl` (centre-arena repeat-mode CacL/CacR
  encoding). A full-width playfield (e.g. Pizza Boy's 4 buildings across clk4-143) needs **per-scanline/band
  `PF0/PF1/PF2` tables** (`PF0tab/PF1tab/PF2tab`, reflect or repeat). The measurement already exists — only the
  emit needs a `-format pf012|arena` switch. Size: S-M. 〔TOOL-EVAL V2 / idea #1〕
- **RL-6 — `behavmatch` scenario generalisation — DONE.** Two halves, both measured.
  (a) The scripts were Outlaw mechanics; they are now ROM-AGNOSTIC and name no game variable — both players,
  every direction, tap-vs-hold fire, diagonals, aimed fire, simultaneous fire, a 900-frame duel, the console
  switches. On the real cartridge that took live bytes from 16 to 51 and revealed the RAM layout as
  consecutive per-player pairs.
  (b) They are now LOADABLE: `-scenarios file.json|dir/` and `-export-scenarios` to dump the built-ins as a
  starting point, so reaching a new game no longer puts a Go build between a person and the question. A
  scenario is an input script plus objects to watch — data, not code.
  The check is not that the JSON parses but that a scenario driven from disk moves the machine IDENTICALLY:
  round-tripped built-ins produce byte-identical traces across all 128 RAM addresses and identical held-input
  levels per frame. Malformed entries are errors, never skips (missing name/frames, a frame past the end,
  both or neither of panel/action, duplicate names, an empty file) — a scenario silently absent from a suite
  is a hole in coverage nothing would report, the same reason the coverage denominator and the RAM-gate mask
  print what they left out.
- **(original) RL-6 — `behavmatch` scenario generalisation.** Scenarios are hardcoded Outlaw mechanics in
  `internal/behavmatch/scenarios.go` (e.g. `p0-fire-freeze` = the no-Getaway rule). Make scenarios
  **per-game/loadable** (input timeline + trace targets via JSON/DSL or a per-game registry) so a new game is
  usable without editing the package. Pizza Boy needs: 4-way move / delivery (+$10 BCD trace) / crash
  (pstate/freeze). Ties to the behaviour-reproduction system (any non-Outlaw game needs this). Size: M.
  〔TOOL-EVAL V4 / idea #6〕
- **RL-2 — `vismatch` global vertical-offset auto-detect — DONE.** `vismatch.FindVerticalShift` searches
  +/-max-shift for the offset that explains the most element mismatch and the CLI prints one line:
  *"mine sits 8 scanline(s) LOWER than the target (removes 100% of the mismatch: 2560 -> 0 cells)"*.
  It reports the count at zero alongside the best, so a shift that explains little cannot be presented as
  an explanation — two pictures can differ in ways no alignment fixes.
  Graded against a shift known by construction rather than one inferred from its own output: the new pair
  `roms/litmus/litmus_shift_base.asm` and `litmus_shift_down8.asm` are the same work with eight visible
  lines moved into VBLANK, so the picture sits exactly eight lines lower and the frame still totals 262.
  The detector must return +8 and -8 depending on the argument order, must explain all of the mismatch
  there, must NOT claim an alignment between two unrelated pictures, and must report zero for a ROM against
  itself.
  Reproduced first: on that pair the old output gave two 8-line band rows and never named the offset,
  which is exactly the reading-back-by-hand that produced a 3px error in the field.
- **(original) RL-2 — `vismatch` global vertical-offset auto-detect + 1-line summary.** When N playfield bands are all
  shifted by a constant, a human currently reads the band-diff and infers it. Emit `best global vertical
  shift = −11 (removes X% of PF mismatch)` automatically (framegen already does this content-shift search
  internally — lift it into vismatch). Size: S. 〔TOOL-EVAL V3 / idea #2〕
- **RL-3 — matched-state moving-object comparison — DONE.** `vismatch -scenario NAME [-scenarios file]`
  drives BOTH ROMs through the same input script (`behavmatch.RunScenario`) and compares the picture there.
  Reproduced as a measurement first: the SAME ROM compared at frames 10 and 20 reports its ball as a
  difference, because motion_glide moves it a scanline per frame — the difference being when it was looked
  at, not where the object is. Driven through one script, the same comparison is pixel-exact; two genuinely
  different ROMs still come out different, so the feature cannot turn comparisons green.
  Caught while wiring the CLI, and worth recording because the Go test did not catch it: the first version
  reused `-target-frames`/`-mine-frames` (defaults 28 and 12) as the per-side warmup, so matched state gave
  the two sides DIFFERENT warmups and reported a ROM as differing from itself. Separate `-target-warmup` /
  `-mine-warmup` default to 0, and an asymmetry between them is printed rather than left in the flags —
  it is legitimate (one ROM has a title the other does not) and it is also exactly what makes a ROM differ
  from itself.
- **(original) RL-3 — matched-state moving-object comparison (`vismatch` × `behavmatch`).** Single-frame vismatch is
  perfect for static PF but a moving object (bike/taxi) reads as transient diff when the two ROMs are in
  different game-state. Drive both to the **same scripted state/frame (behavmatch), then vismatch** — measures
  moving-object position fidelity in one shot. (Done manually via matched-state this session; automate.)
  Size: M. 〔TOOL-EVAL V5 / idea #3〕
- **RL-4 — `-target-until-gameplay` auto frame-find — DONE.** `vismatch -target-until-gameplay` reports the
  first frame whose PLAYFIELD both differs from the settled opening frame and then holds still, so
  `-target-frames` / `-target-warmup` need not be hand-tuned. The playfield is the signal because sprites move
  during play: a settled game still has a still background, while a title that auto-advances changes it once.
  Graded against a frame known by construction — the new `roms/litmus/litmus_title_then_play.asm` switches
  pattern at frame 30 and the detector returns 30. It must also NOT invent a transition (a static ROM reports
  found=false at frame 0) and must not claim a settled picture without room to observe the stability it asks
  for.
  One measured correction on the way: the first frames are the machine booting, so a baseline taken there
  differs from every later frame and reported a title on EVERY cartridge including ones with none. A two-frame
  settle before the baseline fixes it.
  On real cartridges: Outlaw settles at frame 7; Combat and Stampede report no settling point with
  `playfield changed: false`, and the message says what that means — no title, or the game needs input to
  start — rather than guessing.
- **(original) RL-4 — `-target-until-gameplay` auto frame-find.** Auto-detect the first frame where PF/sprites stabilise
  (title exited) so the target frame count need not be hand-tuned. Complements RL-5. Size: S-M. 〔idea #4〕
- **RL-7 — `framegen` field evaluation (validator track). DONE (v1.115.0).** Evaluated on 31 technique ROMs +
  3 commercial cartridges. The self-calibration works: VBLANK, vertical shift and sprite X all converge
  without hand-iteration. What the evaluation found is that **the tool did not say what it failed to
  reproduce**, and its kernel cannot reproduce everything.

  **The defect, reproduced first.** `framegen`'s only progress line was `element match N / 34240`. On
  *Fishing Derby* that read **33172 / 34240 = 96.9%**, followed by `wrote fd.asm` — no warning of any kind.
  Measured per element, the same frame is:

  | elem | target cells | matched | drawn anywhere in the clone |
  |---|---|---|---|
  | BG | 26226 | 26172 | 27186 |
  | PF | 6912 | 6888 | 6888 |
  | P0 | 665 | **75** | 98 |
  | P1 | 337 | **37** | 68 |
  | M1 | 42 | **0** | **0** |
  | BL | 58 | **0** | **0** |

  96.9% overall while **the fisherman is 11% correct (75/665)** and the hook and line are absent outright.
  Background is 77% of the visible area, so it carries the headline number and buries everything the
  reproduction is actually for. `grep -c 'ENAM0\|ENAM1\|ENABL'` on every generated clone returns **0**: the
  emitted kernel writes PF/GRP0/GRP1 and nothing else, so missiles and the ball are missing *by
  construction*, not by a tuning error.

  **The fix** — `framegen` now measures its own output per element against the target's attribution and
  reports what it did not draw, in three places: the terminal report, a `; NOT REPRODUCED:` block burned into
  the generated `.asm` banner (the file outlives the terminal), and the **exit code** (1 = incomplete, matching
  `vismatch`/`behavmatch`). Structural absence (`clone 0`) is reported separately from misplacement
  (`clone > 0` but in the wrong cells) because they have different causes and different fixes.

  **Field results after the fix** (`-frames 28`, 31 technique ROMs):

  | verdict | count | ROMs |
  |---|---|---|
  | pixel-exact | 21 | banked_game, bullets, divtable, flicker_multiplex, game_states, maze, multicolor48, music_driver, paddle_demo, pf_modes, procgen_demo, rpgmap, sfx_demo, sound_driver, sprite_anim, tia_pcm, two_line_kernel, two_line_vdel, venetian, vertical_pos, vertical_pos_dcp |
  | placement differs | 8 | zone_multiplex 380, rts_dispatch 376, text12 298, bitmap48 212, dyn_multisprite 176, hscroll 64, score6 48, text24 162 (cells) |
  | element absent | 2 | shared_setxpos (M0 1712, M1 1712, BL 428), road (M0/M1/BL) |

  Commercial: **Outlaw pixel-exact, Combat pixel-exact** (with and without `-reset`); **Fishing Derby partial**
  (M1/BL absent). The 8 "placement differs" ROMs share one cause, now stated in the report rather than left
  to be discovered: `framegen` carries **one X per player for the whole frame**, so a kernel that repositions
  a sprite per zone cannot be followed — `zone_multiplex` loses 190 cells on each player.

  **Verdict: `framegen` is a sound visual-layer validator for BG/PF/P0/P1 on single-position kernels**, which
  covers 21/31 of the corpus and 2/3 of the cartridges tried. It is *not* a whole-frame reproducer: it draws
  no missiles or ball, and it cannot follow a multiplexed sprite. That limit is now machine-enforced (exit 1)
  instead of resting on the reader noticing. A tie in the vertical-shift search also used to resolve to the
  first candidate scanned (−4), i.e. "no offset explains anything" came out as "shift the picture up four
  lines"; ties now resolve to 0. Adding ENAMx/ENABL replay and per-zone sprite X are the follow-ups —
  filed below as RL-8.
- **RL-7b — the two defects the RL-7 evaluation exposed downstream. DONE (v1.115.0).** Running the generated
  clone through `cyclebound` and `beamtrace` — rather than only looking at its picture — found two faults that
  a pixel comparison structurally cannot see.

  **(a) The cleanup ran inside the last visible scanline.** `cyclebound` put the `Kern` region at **97 cycles
  against a 76-cycle budget**; hand-counting the loop body gives 66, and the missing 31 are the loop-exit
  cleanup, which falls through *before* the next WSYNC. `beamtrace` on the clone shows it directly:

  ```
  scanline 240:                       <- the clone's LAST visible line
      clk +133  GRP0 $00  pixels[133..160)   @$F08E
      clk +142  GRP1 $00  pixels[142..160)   @$F090
  ```

  So a sprite pixel right of clock 133 survives on all 213 earlier lines and vanishes on the last one.
  Proving it took **two attempts at the litmus**, and the failed one is the useful part: a full-width
  *playfield* shows nothing, because PF2 — the only PF register covering the right edge (clocks 128-159) — is
  cleared after the line has already ended. Only GRP0/GRP1 are cleared early enough to bite, so the planted
  quantity had to be a *player* near the right edge (`roms/litmus/litmus_lastline.asm`). Fixed with a `sta
  WSYNC` before the cleanup; after it, line 240 carries no cleanup write and the clears land in the next
  line's HBLANK at clocks −53..−17. `Kern`'s worst drops **97 → 66**, and the region violation disappears.
  *(Superseded, and left visible rather than rewritten: RL-8a added the missile and ball enables, so today
  the same region measures **74** of 76 and still certifies. Re-measured 2026-07-30 — the 66 was true when
  written and no longer is, which is precisely the shape of number this document keeps having to correct.)*

  **(b) Every clone ever generated was out of NTSC spec.** Measured across the 31-ROM corpus, the *pre-fix*
  generator emitted **267 scanlines on 30 ROMs and 268 on one — 262 on none of them**. Five to six lines
  over: a real television rolls. Nothing caught it because `vismatch` and the coverage table both compare the
  visible picture, and the picture was pixel-exact. The cause is that overscan was computed from a formula
  that ignored `vblankAdj` (self-calibration adds VBLANK lines to slide the picture down; nothing took them
  back off the end) — and, more fundamentally, that **no formula can be right here**: `SetXPos` is a div-15
  subtract loop, so placing a player far right costs more than placing one on the left and can run past its
  own scanline. Combat (P1 at clock 145, input 139) spends one prologue line more than Outlaw does. Fixed by
  *measuring* — the frame length now self-calibrates against `StepFrame()` exactly as X and VBLANK already
  did, and `framegen` reports the count every run and exits 1 when it is wrong. **After: 262 on 31/31**, with
  the pixel results unchanged (21 exact / 8 misplaced / 2 missing) — the fix corrected the frame without
  disturbing the picture. Regression-locked by `roms/litmus/scenarios/lastline.json` (`ntsc_frame_lines`).

  The lesson is the RL-7 lesson one level up: the tool that checks the picture cannot see the frame around
  it, and a clone can be pixel-perfect and unplayable at the same time.
- **RL-7c — `framegen` asserted a cause it had not measured, and replayed one frame-final NUSIZ. DONE.**
  Found on `roms/litmus/litmus_nusiz_all.bin` (the eight-NUSIZ-mode litmus added by the statecov work).
  `framegen` reported **2666 of 34240 cells wrong** and explained them with a fixed sentence: *"this is
  placement, not omission (one X per player cannot follow a per-zone multiplexed target)"*. That ROM places
  P0 and P1 **once, before the frame loop, and never moves them** — measured, 1 distinct reset X per player
  over 191/190 drawn lines. The explanation was printed unconditionally on every non-exact run, so it was
  never a finding; it was a slogan.

  **The first hypothesis was wrong too, and killing it is the load-bearing part.** The obvious suspect was
  `nusizWidth`, which maps only $05→2 and $07→4 and returns 1 for the five COPY modes. Falsified by
  experiment: eight otherwise-identical probe ROMs, one per NUSIZ mode held CONSTANT for the whole frame,
  reproduce **pixel-exact in all eight** (P0 cells matched 864/864, 1728/1728, 1728/1728, 2592/2592,
  1728/1728, 1728/1728, 2592/2592, 3456/3456 for modes 0..7). The copies are hardware replication of the
  same 8-bit byte, so 1 pixel per bit is the *correct* width for a copy mode and `grpByte` was already
  reading the right pixels. The real cause is that `extract` read NUSIZ **once, at the end of the rendered
  frame** — litmus_nusiz_all ends in mode $07, so all 214 lines were reproduced quad-width.

  **Fix, in two parts.**
  1. *Diagnosis.* Per visible line the extractor now measures, for each player, the NUSIZ in force, the reset
     position, and the number of separate runs `DecomposeRow` reports; the same measurement is taken off the
     **clone**. The RESULT line now names a cause only when the number that proves it was counted — copies
     (`target orders up to 3 copies (NUSIZ $06 on 37 lines) and the clone draws up to 1`), multiplexing
     (**only** when distinct reset X > 1, with the positions and their line counts), or a late write (the
     kernel's own store lands after the object's leftmost pixel — arithmetic on the emitted block schedule).
     When none of the three is measurable it says so instead of picking whichever sounds plausible. The size
     shift is removed before counting positions, because HmovedPixel moves ±1 on a 1x↔2x change without the
     sprite having moved (`SetNUSIZ` in the engine does exactly that, and it is measured: 24 for modes
     0,1,2,3,4,6 and 25 for 5 and 7 with an identical RESP0).
  2. *Reproduction.* When per-line NUSIZ varies, the kernel replays it from a table. Room is made by dropping
     playfield writes the target provably does not need, decided per PF register: both halves 0 on every line
     (the reset clear already leaves the register at 0), or right-half bytes equal to the left on every line
     (CTRLPF repeat reproduces it). A left write is never dropped alone — the next line would inherit the
     right half's value.

  **Result: 2666 → 2 cells** (P0 3616/3616, P1 3120/3120; the 2 are BG cells the clone paints P1). Those 2
  are reported, not swept up: the target clears GRP1 *part-way along* scanline 228, leaving a 10-pixel P1
  run, and a kernel that writes each register once per line in HBLANK can only draw 8 or 12 at quad width.

  **Where it still gives up, with the number.** `rts_dispatch` needs 6 PF + 2 GRP + 1 NUSIZ0 = 9 blocks.
  Nine blocks *run*: `lda TABLE,y` on a page-aligned table costs 4, so 3+7·9+7 = 73 of 76, and the 9-block
  clone measured 262 scanlines with its picture improved from 376 wrong cells to 8. But `cyclebound` bounds
  `lda abs,y` at 5 (it cannot assume the alignment) and scores the same kernel at **82 against a 76 budget,
  `certified:false`**. The kernel is therefore capped at the *certifiable* 8 blocks, `rts_dispatch` keeps its
  baseline 376 cells, and the tool says why — naming the mode, the copy count and the cycle arithmetic.
  Emitting an artifact that trips the repo's own line-budget gate is the RL-7b failure mode again.

  **Regression: none.** Whenever NUSIZ is constant down the frame — 30 of the 31 technique ROMs, Outlaw and
  Combat — the historical eight-block layout is emitted unchanged, verified by diffing the generated sources
  (only the new per-block deadline comments differ). Corpus after: **21 pixel-exact / 8 differ / 2 partial,
  262 scanlines on 31/31**, identical to before except `rts_dispatch`'s wording; Outlaw and Combat still
  pixel-exact with and without `-reset`. Two interim versions of this change *did* regress
  (`flicker_multiplex` 0→78, `sprite_anim` 0→84, `text24` 162→P0/P1 absent) by sampling the graphics byte at
  the per-line reset position — the end-of-frame marker and the visible-line position are not the same number
  on a target that parks its sprites during overscan. That is why the sampling position stayed put and only
  the size shift was applied to it.
- **SD-6 — `prove_line_budget` false-positived every two-call shared subroutine. DONE (v1.115.0).** Found by
  running a generated clone through the prover rather than only looking at its picture. The ordinary
  two-sprite shape — both players placed through one shared `SetXPos` — reported `certified:false` with the
  routine classified `visible`, while the **identical kernel with one call site certified**:

  | kernel | regions | blank | max_worst | certified | verdict |
  |---|---|---|---|---|---|
  | generated clone, 2 calls to `SetXPos` | 9 | 2 | 89 | **false** | `SetXPos` reported `visible` |
  | same kernel, 1 call | 9 | 3 | 74 | true | — |

  Nothing but the number of call sites differs. That shape is in this project's own Outlaw and Pizza Boy
  builds, so the prover was crying wolf on the exact kernels it exists to certify.

  **Cause.** `absSuccessors` reset a JSR's return point to full Top, discarding facts the callee cannot
  change. VBLANK went unknown at the *second* call site; that unknown flowed into the shared subroutine,
  joined with the known-on state from the first, and the routine's own entry state came out unknown — so
  `displayOff()` was false and the region could not be classified blank.

  **Fix.** Keep VSYNC/VBLANK across a call when the callee provably cannot write them. One-sided by
  construction: an unresolvable store, a push whose SP range can reach $0100/$0101 (page 1 mirrors the
  console's addresses — the Stack Trick is a real display write), or a nested call already on the stack all
  answer "not preserved". Indexed stores are resolved through the proven index range, which does not exist
  on the first pass, so `computeStates` runs twice — the second pass adds only a fact justified by a sound
  first-pass range.

  **A second, pre-existing hole fell out of the litmus.** `regionTouchesDisplay` tested the raw operand, so
  it saw only non-indexed writes: `sta VSYNC,x` is AbsoluteX and returned no address at all, letting a region
  that writes VBLANK be skipped as blank. Both checks now share one resolution path.

  **Corpus effect (31 technique ROMs).** Three false positives removed — `game_states` now certifies
  (`EnterPlay` runs between VBLANK-on and VBLANK-off and is called from two sites), `bullets` ×2 (`PosObj`
  called twice inside VBLANK) and `sfx_demo` (the overscan loop) reclassify to blank. One region moved the
  *conservative* way — `rts_dispatch` at 55 ≤ 76, no violation — because indexed stores are no longer
  invisible to the display check. The clone's real 89-cycle interval is **not lost**: it moves to
  `blank_over`, i.e. frame-line drift rather than a torn line, which `ntsc_frame_lines` owns and which
  `framegen` now self-calibrates (RL-7b).

  **Graded against the machine, not itself.** `blankclass_test.go` runs every corpus ROM and asks the
  television (`GetLastSignal().VBlank`, via new `emu.DisplayOff()`) whether the beam is really blanked each
  time execution reaches a blank-classified region's opening WSYNC: **129,936 executions across 31 ROMs, 0
  disagreements**, the 1 never-reached region reported as not covered. **Negative control**: forcing
  `displayOff()` to true makes the test fail with 28 disagreements — a test that cannot fail proves nothing.
  Twin litmus `roms/litmus/litmus_jsr_display.asm` holds three routines of identical shape differing in one
  store (`sta COLUP0,x` / `sta VSYNC,x` / `sta VBLANK`), all called from the same place with the same index
  values, so a rule that answers the same way for all three is wrong whichever answer it gives.
- **SD-7 — the bank-switching refusal was not real. DONE (v1.115.0), stage 0 of bank support.**
  The premise "cyclebound declines bank-switched cartridges" was only true of `DefUse`. Measured on
  `banked_game.asm` (8192 bytes, F8, 2 banks):

  | entry point | before |
  |---|---|
  | `DefUse` | declines (`FlatBankOnly`) |
  | `StaticBranches` | returns flat branches + `banked=true` so `cover` can mark `decoder_incomplete` |
  | **`Prove`** | **does not decline** — `regions:0, certified:false`, no decline field at all |
  | **`BeamIntervals`** | **does not decline** |
  | **`Lint`** | **does not decline**, and built its own `program` literal that did not even set `banked` |

  `Prove` was not refusing; it was analysing a program that does not exist and being saved by the incidental
  fact that whichever bank the flat fold landed in contained no reachable `STA WSYNC`, which tripped the
  "0 regions never certify" backstop. That is luck. A banked image whose flat fold *did* contain a WSYNC
  would have been analysed and reported on with confidence. The guarding test encoded the luck as its
  premise and `t.Skipf`'d the moment it changed — a test that stops testing.

  **The verdict now comes from the engine, not from `len(rom)`.** Size is not the machine: an 8192-byte
  image is F8 only unless it fingerprints as WF8/3F/E0/E7/WD/FE/UA, and a superchip variant overlays RAM on
  part of the window whatever the length says. `emu.CartInfo()` returns the mapper the cartridge actually
  fingerprinted as and its bank count, so the analysis and the emulator its own acceptance tests run on
  cannot disagree about which machine is being described. Decided once in `loadProgram`, so all four entry
  points share one verdict: measured, `F8/F8/F6/F4` correctly named on `banked_game`, `litmus_bank`,
  `litmus_bank_f6`, `litmus_bank_f4`.

  A linter that stays silent about a program it never decoded reads as an all-clear, so `Lint` returns a
  `not-analysed` warning rather than nothing. The corpus false-positive test now separates a refusal from a
  timing claim and **prints its own denominator**: "30 kernels with zero timing false positives; 1 DECLINED".
  It also fails if *nothing* was declined — a linter that analysed all 31 is reading fiction in at least one.

  Golden diff across all 31 technique ROMs + all `cb_*` litmus: **byte-identical except `banked_game`**, the
  only banked image among them.

  The full staged design for real bank support (site identity `(bank, addr)`, hotspot edges as
  "same address, other bank", phantom-read switches, per-mapper decline list, and the unsoundness risks
  including ROM tables read from the wrong bank) is researched and recorded — stages 1-4 remain.
- **SD-8 — bank-switched cartridges are analysed, one bank at a time. DONE (v1.116.0), stage 1 of bank
  support.** Rather than key every address on `(bank, addr)` — a ~20-call-site refactor across five files —
  each bank is handed to the EXISTING pipeline as the self-contained 4K image it already is: its own length
  gives the mask, the address space gives the base, and its own vectors seed the decode. A flat ROM is
  literally the one-unit case of the same loop, which is why its output is unchanged.

  | ROM | before | after |
  |---|---|---|
  | `banked_game` (F8, 2 banks) | 0 regions, no decline | **7 regions**, `certified:false`, 1 unmodelled switch |
  | `litmus_bank` (F8) | — | bank0 52 instrs/6 regions, **bank1 4 instrs/0 regions** |
  | 42 flat ROMs | — | **byte-identical JSON** |

  **Cross-bank flow is refused, not guessed.** A region whose instructions can touch a bank-switch hotspot
  continues in a bank this analysis never entered; costing the bytes that happen to follow in the *current*
  bank would be a number about a path the hardware does not take, which is worse than no number because it
  looks like one. The refusal names the mapper's own symbol: *"the access at $FF00 reaches BANK1 ($1FF9)"*.
  It cannot key on stores — `lda $FFF9` is the canonical switch — and it counts an unresolvable access under
  a hotspot-bearing mapper as a possible switch.

  **Two defects found on the way, both of the "confidently wrong" kind:**
  - **`memorymap.MapAddress` does not fold cartridge mirrors.** Measured: `$FFF9 → $FFF9`, `$3FF9 → $3FF9`.
    It identifies cartridge space but leaves the address alone, so comparing its output against a hotspot
    table keyed in the primary mirror (`$1FF9`) matched **nothing at all** — a region full of `lda $FFF9`
    reported no bank switch whatsoever. Folding now mirrors what the cartridge itself does
    (`OriginCart | (addr & CartridgeBits)`).
  - **`litmus_bank` came back `certified:true`** — a true statement about bank 0 of 2, presented as a verdict
    on the cartridge. Certification now requires `UnmodelledSwitches == 0`, for the same reason it requires
    `Converged`: "every region I looked at passed" is not a proof when the reason some were not looked at is
    that the program leaves for somewhere the analysis does not follow.

  **The residue is named, not hidden.** `BankCoverage` reports decoded instructions and regions per bank, so
  `bank1: 4 instrs / 0 regions` cannot pass for a bank that was checked. Graded against the machine
  (`banksound_test.go`): every `(bank, pc)` the emulator executes must be in the static decode. **Bank 0 —
  the bank the reset vector reaches — is complete on all four ROMs (0 missing).** The remaining absences are
  exactly the worker banks entered only across a switch (`litmus_bank`: 4 of 36), which is Stage 2's job; the
  test requires each to be reported as a barely-decoded bank carrying no region verdicts, and requires the
  report to state a cause for the residue.

  **Corpus gap recorded, not papered over:** in all four bank ROMs the banks never execute the same address,
  so `#(bank,pc) == #pc` and **none of them can catch a flat-keyed instrument**. The test says so in its own
  output rather than passing quietly. A litmus where two banks run code at one address is the missing piece.

  #### SD-8b — the decode is closed over bank switches (stage 2 of bank support)

  Stage 1's residue was the worker banks: a bank was decoded only from its OWN reset/NMI/IRQ vectors, and a
  worker bank is entered by the trampoline that switched to it, never by its own stub. **Measured before, per
  ROM, as executed `(bank,pc)` pairs absent from the decode** (`banksound_test.go`, the emulator as oracle):

  | ROM | mapper | absent before | where | absent after |
  |---|---|---|---|---|
  | `litmus_bank` | F8, 2 banks | **4** of 36 | bank 1 `$FF03/$FF05/$FF07/$FF09` | **0** |
  | `banked_game` | F8, 2 banks | **1** of 61 | bank 1 `$FF83` | **0** |
  | `litmus_bank_f6` | F6, 4 banks | 0 of 41 | — | **0** |
  | `litmus_bank_f4` | F4, 8 banks | 0 of 57 | — | **0** |

  **The fix needs no new map key.** An instruction whose memory access reaches a bank-switch hotspot continues
  at the FOLLOWING address in the bank that hotspot names — and because each bank is already analysed as its
  own self-contained 4K image, that landing address is just another decode entry point for the target unit.
  Nothing in `map[uint16]Instr` changes. Seeding is a fixpoint (seeding B can reveal a hotspot access in B
  that seeds C); measured, it closes in **2 rounds on all four ROMs** (1 productive + 1 that adds nothing),
  against a cap of 8, and `cross_bank_seed_capped` exists so a capped run can never read as a closed one.

  **The bank comes from the mapper's own symbol, and an unparseable symbol is refused, not scraped.** Read via
  `emu.BankSwitchHotspots()`, never hardcoded — measured: F8 publishes `$1FF8=BANK0 $1FF9=BANK1`, F6
  `$1FF6..$1FF9 = BANK0..BANK3`, F4 `$1FF4..$1FFB = BANK0..BANK7`. The parse insists the whole symbol be
  `BANK<digits>` because two vendored mappers publish other shapes for which "the same address in the other
  bank" is simply not where execution lands: **Parker Bros E0 publishes `B0S0`..`B7S2`** (bank-in-SEGMENT —
  only a 1K slice moves) and **M-Network publishes `RAM0`..`RAM3`** (cartridge RAM, which is not in the image
  at all). A digit-scrape would read those as banks 0 and 0..3 and invent an edge the hardware does not have,
  so they are reported in `unresolved_hotspots` and seed nothing.

  **Target addresses resolve through `cartHotspotKey`** (the SD-8 mirror-folding defect: `$FFF9`, `$1FF9` and
  `$3FF9` are one hotspot), **a READ switches as a write does** (`lda $FFF9` is the canonical form), and
  addresses are resolved under `topState()` rather than the abstract-interpretation result — the states are
  computed FROM the decode, so using them here would be circular. Top over-approximates: an unknown index
  contributes its whole 256-address footprint. Over-seeding costs precision; under-seeding would leave
  executed code out of the decode, which is the direction that lies. An access whose target cannot be
  resolved at all yields no address, therefore no symbol, therefore no bank: counted in
  `unresolvable_switch_accesses`, never guessed.

  **This closes the DECODE, not the flow.** `hotspotRefusal` is untouched: a region that can reach a hotspot
  is still refused, `UnmodelledSwitches` still gates `Certified`, and all four ROMs still report
  `unmodelled_switches: 1, certified: false`. The analysis now knows what the other bank CONTAINS, not how
  the cycles run across the boundary. That is stage 3.

  **BankCoverage, before → after** (instructions / regions; seeded entry points in brackets):

  | ROM | bank 0 | worker banks |
  |---|---|---|
  | `litmus_bank` | 52/6 → 52/6 [2] | bank1 **4/0 → 99/0** [1] |
  | `banked_game` | 67/7 → 67/7 [2] | bank1 **66/0 → 67/0** [1] |
  | `litmus_bank_f6` | 67/6 → 67/6 [2] | banks 1-3 1362/1368/1374, unchanged [1 each] |
  | `litmus_bank_f4` | 103/6 → 103/6 [2] | banks 1-7 1362..1398, unchanged [1 each] |

  `litmus_bank` bank 1 jumping 4 → 99 is over-approximation, and it is measured rather than assumed: 4 of the
  95 new instructions are the body the machine really runs (`$FF03/$FF05/$FF07/$FF09`) and the rest is the
  `$FF` filler between that body and the reset stub, decoded as a chain of undocumented `isc abs,X`. None of
  it stores WSYNC, so bank 1 still carries **0 regions** — extra decoded bytes, not extra verdicts.

  **Golden diff, mandatory:** cyclebound JSON for all 31 `roms/techniques/*.asm` + all 12 `roms/litmus/cb_*.asm`,
  before vs after: **42 of 43 byte-identical**. The one that changed is `banked_game`, the only banked image
  in the set. Flat ROMs are untouched by construction — one analysis unit means no hotspots, no seeding, and
  `TestFlatRomIsNotSeeded` asserts the flat decode, entry list and every new report field are unchanged.

  **The test was given teeth.** A containment check the seeding cannot affect would keep passing with the
  seeding deleted, so `banksound_test.go` now also (a) computes the vector-only decode and requires seeding to
  be **strictly additive** (a lost pair is a hard failure), (b) fails if the corpus stops containing code
  reachable only across a switch — **measured: 5 such executed pairs, litmus_bank 4 + banked_game 1** — and
  (c) requires every seed whose SOURCE the machine executed to have a landing address the machine also
  executed, so a seed cannot be an invented edge.
- **SD-11 — a WSYNC-to-WSYNC region that crosses a bank switch gets a real proven worst case. DONE
  (v1.117.0), stage 3 of bank support.** SD-8 analysed each bank separately, SD-8b closed the DECODE over
  switches, and SD-10 measured that a merged flat map is impossible. What was still missing was the FLOW: the
  one region where a bank-switched kernel does all of its cross-bank work had no number at all, only a
  refusal — and that region is the whole point of the cartridge.

  **Measured before/after, with the machine beside the proof** (`emu.ProfileLineWorst(6, nil)`, rows keyed
  `(bank, PC)`, `cross_frame_dropped = 5` on every ROM — one per frame boundary):

  | ROM (mapper, banks) | crossing region | before | proven after | machine | after verdict |
  |---|---|---|---|---|---|
  | `litmus_bank` (F8, 2) | bank 0 `$F02B` `Vis+0` | REFUSED | **54** | **54** / 1 line, 1152 intervals | `certified:true` |
  | `litmus_bank_f6` (F6, 4) | bank 0 `$F02B` | REFUSED | **72** | **72** / 1 line, 1152 | `certified:true` |
  | `litmus_bank_f4` (F4, 8) | bank 0 `$F02B` | REFUSED | **128** | **128** / 2 lines, 1152 | violation at 76, `certified:false` |
  | `banked_game` (F8, 2) | bank 0 `$F01B` `LvTab+2` | REFUSED (switch) | unbounded, NEW reason | 28 / 1 line, 6 | `certified:false` |

  The three litmus kernels are deterministic, so the machine walks the single path the prover costs and the
  assertion is **exact equality**, not merely proven >= measured. `litmus_bank_f4`'s eight-segment chain
  really does spill into a second scanline (its source compensates with `ldx #29` instead of 30), so it comes
  back as a **stated 128-cycle budget violation** rather than a refusal.

  `UnmodelledSwitches` went **1 -> 0 on all four**, and the new `ModelledSwitchEdges` prints beside it
  (2 / 4 / 8 / 2) because "0 refused" is also what a cartridge that crossed nothing reports.

  **THE EDGE, from the engine rather than from folklore.** An instruction whose DATA access reaches a
  bank-switch hotspot continues at the SAME ADDRESS IN THE TARGET BANK: the Atari mapper switches on the
  access and does not touch PC (`Gopher2600/hardware/memory/cartridge/mapper_atari.go` runs
  `bankswitch(addr)` and then `return cart.banks[cart.state.bank][addr]`), and the hotspot is matched on the
  12-bit-folded address, which `cartHotspotKey` already reproduces. A READ switches exactly as a write does.
  The switching instruction is charged its ordinary cost and the edge is charged nothing — the latch takes
  effect at the next fetch, so no cycle belongs to the crossing. When the access is EXACT the intra-bank
  fall-through is **REPLACED**, which is what makes 54/72/128 exact instead of merely safe: keeping it would
  cost the `$EA` filler that follows in the current bank, which the hardware never fetches. A wide footprint
  (unknown index) keeps the fall-through as well, which over-approximates in the safe direction.

  **The composite key is now real.** `Instr` carries its bank, `site{bank, addr}` is the map key for the
  decode, the region subgraph, the abstract states, the loop folds and the call contexts. `ctx{ret site;
  active bool}` replaces the `ret == 0` sentinel — `(bank 0, $0000)` is an address-shaped value and "no
  active call" must not be spelled as an address. DATA addresses stay flat, deliberately: RAM `$80-$FF`,
  TIA/RIOT `$00-$3F` and the page-1 stack mirrors are not banked, so a bank in a data key would split one
  physical cell into N and break the store kill-set, the value domain and the def-use may-sets.
  `mustWrittenAt` makes the asymmetry explicit — outer key a code site, inner set flat data addresses.

  **One oracle for both the edge and the refusal.** `switchModel.switchEdges` is the single decision point,
  and `successors` is the single successor function every walker uses (collect, longest, the abstract
  interpreter, the beam pass, `regionInstrs`, `mustWrittenAt`, `reachableWithinCallee`, `displayPreserver`,
  `determineBound`'s predecessor scan). Two predicates would eventually disagree, and a successor function
  modelling a switch the refusal does not guard is silently unsound. Every place the flow model cannot follow
  an edge returns a REFUSAL rather than skipping: a dropped successor shortens the longest path.

  **What stays refused** (counted in `UnmodelledSwitches`, still gating `Certified`): an instruction whose
  OWN BYTES span a hotspot — the opcode at the hotspot comes from the NEW bank, so the instruction decoded
  here is not the one that executes, and each further operand byte can select yet another bank; a `jmp`/`jsr`
  INTO a hotspot; an access whose target cannot be resolved at all (measured **0 such accesses on all four
  corpus bank ROMs**, so refusing costs nothing there today, and it is still counted rather than assumed
  away); a hotspot symbol that does not name a bank (`B0S0`, `RAM0`); a target bank outside the analysed set;
  a landing address that wraps past `$FFFF`. **All-or-nothing**: if any candidate of a fan-out cannot be
  resolved, the whole instruction is refused, because modelling the resolvable ones and dropping the rest
  deletes a successor.

  **The geometry is checked rather than assumed.** `analysisUnits` now reads `CartBank.Origins` (grep over the
  package before this: **0 hits**) and declines unless every bank is `len(Data) == 4096` at `Origins ==
  [$F000]`. M-Network is the trap that PARSES: `mapper_mnetwork.go` publishes `BANK0..BANK6` as
  `HotspotBankSwitch`, so `hotspotTargetBank` accepts it, while its banks are 2K mapped at two different
  origins — "the same address in the target bank" is simply false there. Under stage 2 a wrong seed only
  over-decoded; under stage 3 it is a CFG edge the longest-path walk follows, and a wrong edge can SHORTEN
  the longest path.

  **`romTableRange` was unsound on two new axes at once and is fixed on both.** It is now routed by
  `in.Bank`, because on a merged program a flat reader folds whichever bank happened to be bound and a
  `lda table,x` in bank 1 would come out bounded by bank 0's table. And a footprint containing a hotspot is
  `Top`, because the hardware switches FIRST and returns the TARGET bank's byte there. Both would produce a
  narrow, confident, wrong value range feeding a loop's trip count — SD-8c's failure mode on the bank axis.

  **SD-9's guarantee survives the bank boundary, and this was the top risk.** Merging two banks' instructions
  into one node set is exactly the condition that breaks an address-order heuristic. Three fixes:
  (a) `determineBound`'s predecessor scan follows cross-bank edges and returns 0 unless the predecessor set
  is complete — maximising over an INCOMPLETE set under-approximates the entry value, hence the trip count,
  hence the worst case; (b) both address-order filters on the BCS/BCC path became SAME-BANK comparisons,
  since addresses in different banks have no order at all; (c) the `lda #imm` "closest immediate below the
  header" proxy still live on that path — the twin of the one SD-9 deleted — is same-bank-gated and now
  COUNTED. Measured with that counter over all 31 technique ROMs, all 12 `cb_*` litmus, `litmus_bound_proxy`
  and every bank ROM: **0 hits**, so it is dead code kept visible rather than deleted blind.
  (d) `foldLoops` refuses outright any loop whose BODY contains a switching instruction: the folded cost
  `n*body + (n-1)*(latch+pen) + latch` assumes every iteration executes the same bytes, and after a switch
  iteration 2 executes different bytes at the same addresses. That is a soundness condition, not precision.

  **An unmodelled switch widens its possible LANDING sites, not just its own region.** The value domain is a
  whole-program fixpoint while a refusal is per-region, so refusing the region that CONTAINS a switch does
  nothing for the region that contains its LANDING — whose incoming-edge set is then incomplete, and a join
  over an incomplete predecessor set under-approximates. `unmodelledLandings` forces those sites to `Top`,
  per class (own bytes span a hotspot -> `[addr, addr+3]` in every analysed bank, since neither the
  instruction nor its length is known; `jmp`/`jsr` into one -> the hotspot address in every bank; anything
  unresolvable -> `in.next()` in every bank). Reported as `switch_widened_sites` + `switch_widen_reasons`.
  Measured: **5 sites on `litmus_bank`, 5 on `banked_game`, 0 on `litmus_bank_f6`/`_f4`** — all five from
  decoded-but-never-executed filler at bank 1 `$FFF6/$FFF8/$FFF9`, none inside any region's subgraph, which
  is why the blank/visible classification of all four ROMs is unchanged. A first draft widened EVERY site
  instead and that was measurably worse: `litmus_bank`'s two blank regions were reclassified visible.

  **Merged fixpoint cost, measured** (the risk list's concern). Sites and wall-clock `Prove`:
  `litmus_bank` **151 / 17 ms**, `litmus_bank_f6` **4171 / 24 ms**, `litmus_bank_f4` **9763 / 19 ms**,
  `banked_game` **134 / 47 ms** — `converged:true` on all four, against ~1.4k per-bank sites before. The
  iteration cap now scales with the program (`iterCap`, 300000 floor) rather than being a fixed number sized
  for one bank; `Converged` still gates `Certified`, so a capped run certifies nothing.

  **Three new litmus ROMs, each closing a hole no existing test could see.**
  - `roms/litmus/litmus_bank_shared_addr.asm` — the corpus gap SD-8 named and SD-10 measured. Two banks
    execute DIFFERENT code with DIFFERENT cycle costs at ONE address (`$FF10`: `lda #$A0` 2cy vs `inc $8C`
    5cy; `$FF12`: `sta $8D` 3cy vs `lda #$B1` 2cy) inside a single WSYNC-to-WSYNC region. Measured: **38
    executed `(bank,pc)` pairs over 35 distinct PCs** — the first ROM in this corpus where
    `#(bank,pc) != #pc`, so the first that can catch a flat-keyed instrument at all. Proven **58** = machine
    **58**. The executable negative control rebuilds the decode the way a flat-keyed prover would (one map
    keyed on the bare address, later banks overwriting) and requires the answer to DIFFER; it does — the flat
    fold cannot even bound the region.
  - `roms/litmus/litmus_bank_bound.asm` — the cross-bank sibling of `litmus_bound_proxy`. A `dex`/`bne` loop
    at bank 1 `$FF05` whose ONLY initialiser is `ldx #5` at bank 0 `$FF00`, so the header's only
    non-back-edge predecessor is reachable ONLY across the switch. An intra-bank predecessor scan loses the
    bound outright. Proven **70** = machine **70**.
  - `roms/litmus/litmus_bank_unmodelled.asm` — the certification gate's witness. With the crossing modelled,
    all four corpus bank ROMs report `unmodelled_switches: 0`, so the gate would pass **with the gate
    deleted**. This cartridge's switch can never become modelled: `sta (ptr),y` resolves through a RAM
    pointer, so no address, no symbol and no landing bank can ever be named. It reports
    `unmodelled_switches: 1, certified: false` and its own test fails if that stops being true.

    All three render 262 scanlines and are locked by scenarios (`bank_shared_addr.json`, `bank_bound.json`,
    `bank_unmodelled.json`) so their planted premises cannot drift unnoticed.

  **`DefUse` was fixed as well, in the direction SD-7 established.** `p.banked` used to suppress only the
  uninitialised-read pass while MayWrite/Writes/Regions were still computed from the flat 8K fold. Measured
  on `litmus_bank`: `may_write: []` beside the note — an empty may-write set for a cartridge that
  demonstrably writes `$80/$81/$82`, empty only because the flat fold decodes almost nothing. That is the
  same "saved by luck" shape SD-7 condemned in `Prove`, so it is now a real decline naming the mapper and
  the bank count. `BeamIntervals` and `Lint` keep their existing declines rather than presenting
  bank-0-only windows as the cartridge's.

  **`srcmap` was WRONG on banked images, not merely absent, and the doc now says what the code does.** DASM's
  listing address column is the PHYSICAL ROM OFFSET, not the RORG'd address (`litmus_bank`'s listing:
  `0f00 ad f9 ff lda $FFF9` for bank 0, `1f03 a9 b1 lda #$B1` for bank 1), and `Parse` drops every row below
  `$1000` — so bank 0's lines are dropped ENTIRELY and banks 1..n's offsets are stored as if they were CPU
  addresses. Consequence, measured end to end: banked `start_loc` comes back `"Vis+0"` with no `(file:line)`
  while flat `cb_clean` gives `"Main+8 (cb_clean.asm:33)"`, and therefore **`@lines` and `@amax` are INERT on
  every bank-switched kernel**. Not fixed here (it needs the parser to track each segment's ORG/RORG pair and
  key on `(offset/4096, offset%4096|base)`); instead it is STATED — new `Report.SourceAnnotations`, and
  `solver.loc` refuses to attribute a bank-1 site to a bank-0 label, printing `bank 1 $FF03` instead. This is
  load-bearing for `litmus_bank_f4`: its crossing region is over budget only for want of an `@lines 2` the
  map cannot read.

  **Golden diff, mandatory.** cyclebound JSON for all 31 `roms/techniques/*.asm`, all 12
  `roms/litmus/cb_*.asm`, `litmus_bound_proxy` and `litmus_superchip`, before vs after: **44 of 44 FLAT ROMs
  byte-identical**, and only the 4 bank-switched images changed. Every new JSON field is `omitempty` and
  every bank flag stays false on a one-unit image; `TestFlatRomIsNotSeeded` now asserts that for
  `Region.Bank/BankValid/SwitchEdges`, `Step.Bank/BankValid`, `ModelledSwitchEdges`, `SwitchWidenedSites` and
  `SourceAnnotations`. `litmus_bound_proxy` still proves 1015 (the SD-9 lock); `litmus_superchip` is still
  declined.

  **What is NOT done, with the reason.**
  - `banked_game`'s crossing region is still unbounded — but the refusal MOVED from "region can switch banks"
    to "loop bound unknown", with a conditional obligation naming *the loop at bank 1 `$F00A`*. Its bank-1
    level loader counts with `iny`/`cpy #8`/`bne`, a third trip-count idiom `determineBound` does not model.
    Adding it is a new capability that could change flat-ROM output, so it was deliberately not done under a
    change gated on a byte-identical flat golden diff. The plan predicted this region would come back bounded
    and within budget; it does not, and the crossing itself IS followed (`switch_edges: 2`).
  - `banked_game`'s other unbounded region (`KRow+0`, "WSYNC inside loop body") is untouched, as required.
  - `BeamIntervals`, `Lint` and `StaticBranches` are converted to the site key but still analyse a FLAT fold,
    so they decline (or, for `StaticBranches`, flag `banked=true`) rather than answering per bank.
  - `foldLoops` still refuses a region with more than one back edge. Measured non-issue on this corpus (every
    trampoline body is straight-line), but a real cross-bank kernel with a loop in each bank will hit it.
  - `@lines`/`@amax` stay inert on banked images until `srcmap` tracks ORG/RORG (above).
- **SD-10 — measured whether the flat key can hold two banks. It cannot.** Stage 1 kept a bare `uint16`
  code key workable by giving each bank its OWN decode map and never merging them. Cross-bank region analysis
  has to merge, so "can one flat map hold two banks?" decides whether a composite `(bank, addr)` key is
  optional or mandatory. New `DecodedAddrsPerBank` answers it by measurement rather than argument:

  | ROM | banks | decoded addresses | claimed by 2+ banks |
  |---|---|---|---|
  | `litmus_bank` | 2 | 142 | 9 (6%) |
  | `banked_game` | 2 | 123 | 11 (9%) |
  | `litmus_bank_f6` | 4 | 1403 | **1375 (98%)** |
  | `litmus_bank_f4` | 8 | 1427 | **1399 (98%)** |

  Not marginal: every bank is a 4K image occupying the same `$F000-$FFFF` window, so on the four- and
  eight-bank ROMs almost every decoded address collides. A merged flat map keeps whichever bank was inserted
  last — and the region set, the abstract states and the source locations all sit on that map. **The composite
  key is mandatory for stage 3**, and the per-bank-map design stage 1 chose was the right way to defer it.

  Locked as a test that fails if the premise changes — if the banks ever stop overlapping, it is the test's
  assumption that broke, not the code.
- **SD-9 — the loop-bound heuristic under-approximated by 40× on a `roll_free:true` verdict. DONE
  (v1.116.0).** The judged plan for cross-bank flow named this as its top risk and framed it as a hazard the
  *splice* approach would introduce. It is not: it reproduces on a **flat 4K image with no bank switching**,
  so it was already there.

  `determineBound` took a counted loop's trip count from *"the immediate LDX/LDY at the greatest address
  below the loop header"* — a proxy for "the initialiser that ran most recently", sound only while address
  order matches execution order. A backward jump breaks that.

  **New `roms/litmus/litmus_bound_proxy.asm` plants it:** the `ldx #200` the loop really runs with sits
  ABOVE the header, reached by a forward jump, while a `ldx #2` that executes and is then discarded sits
  below it. The proxy could only see the decoy.

  | | before | after |
  |---|---|---|
  | proven worst | **25** | **1015** |
  | machine measured | 1015 | 1015 |
  | `certified` / `roll_free` | true / **true** | true / **false** |
  | frame (machine) | 273 scanlines | 273 scanlines |

  A **fortyfold under-approximation carried on the `roll_free` verdict**, on a ROM the emulator runs at 1015
  cycles across 14 scanlines in a 273-line frame. That is the one direction this package forbids, and it was
  the headline claim.

  **The fix** takes the counter's entry range from the abstract state of every predecessor of the header
  except the back-edge, maximised — sound, because more iterations cost more — and returns 0 (stays
  unbounded) when any predecessor's range is unknown. The address proxy is gone; a stated refusal is worth
  more than a number that can be wrong by 40×. The BCS/BCC divide-loop path already worked this way, which
  is why only the `dex`/`bne` path was affected.

  **Precision cost: none, measured.** Golden diff over all 31 technique ROMs and every `cb_*` litmus:
  **43 of 43 byte-identical**. The abstract states were already accurate enough; only the fallback was
  unsound.

  Graded against the machine, not against the prover's own arithmetic: the test asks the emulator and
  requires proven ≥ measured. Negative control: restoring the address proxy makes it fail with 2
  disagreements. The 273-line frame is locked in `scenarios/bound_proxy.json` so the planted premise cannot
  drift unnoticed.
- **SD-8d — the ∃ oracle could not be trusted for what comes next. DONE (v1.116.0), graft G1.** The judged
  plan for full cross-bank flow analysis names this as the step that must come BEFORE any prover work: the
  runtime profiler is what the static proof is graded against, so it has to be trustworthy first.

  Two things it was silent about:

  1. **It keyed its rows on the strobe PC alone** (`rows := map[uint16]*LineWorst{}`). Two banks storing WSYNC
     at one address merge into a single row, and the containment check against the static proof then passes
     while testing half of what it claims. Measured, no ROM in the corpus executes the same address in two
     banks — so the old keying was correct *by ROM layout, not by the instrument being right*. Rows are now
     keyed `(bank, PC)` and carry the bank; flat images carry no bank field at all, so "bank 0" and "not a
     banked cartridge" cannot be confused. Measured: 6/7/6 rows tagged on the three bank ROMs, **0 on flat
     images**, so their output is unchanged.
  2. **It dropped frame-crossing intervals silently.** An interval straddling a frame boundary has no valid
     cycle count from the beam coordinates, which is the right call — but dropping it without saying so makes
     a table that is merely incomplete look complete. The count is now returned and surfaced as
     `cross_frame_dropped`, printed even when zero so an absent field cannot read as "there were none".
     Measured: **5 over 6 frames on every ROM tried — one per boundary**, flat and banked alike.

  **A claim in the plan did not survive checking.** It stated that "banked_game's cross-bank interval is
  exactly a frame-crossing one", i.e. that the trampoline fell into the dropped set. Measured against the
  static report: **every one of the 6/7/6 static regions on litmus_bank, banked_game and litmus_bank_f4 has a
  matching dynamic row — 0 static regions without a measurement.** The dropped intervals are the frame
  boundaries and nothing else. The instrument was worth fixing regardless, but not for the stated reason.

  Negative control: forcing the bank flag off makes the test fail with 2 disagreements, so it can fail.
- **SD-8c — a superchip cartridge was analysed as if its RAM were ROM. DONE (v1.116.0).** Found by the
  adversarial verification of SD-8b, which asked what `analysisUnits` does NOT check.

  A superchip overlays 128 bytes of RAM on the bottom of the cartridge window — `$F000-$F07F` write port,
  `$F080-$F0FF` read port, both reaching the same 128 cells — so **the image is not what the CPU reads
  there**. That is not cosmetic for this package: `romTableRange` folds real cartridge bytes into an *exact*
  value range, that range bounds a loop's trip count, and the trip count sets the proven worst case. Folding
  a RAM address therefore produces a narrow, confident, wrong number in the one direction the package
  forbids. Measured hole: `analysisUnits` accepted any mapper with `banks > 1` that published hotspots and
  asked nothing about RAM (`grep -c Superchip` over the package: **0**).

  Presence is **fingerprinted from the bytes**, not declared — the engine requires every bank's first page to
  mirror its own two halves — so the same image is `F8` or `F8SC` depending on the engine's decision. Which
  is exactly why the analysis asks `emu.HasSuperchip()` instead of inferring from the size.

  New `roms/litmus/litmus_superchip.asm` satisfies that fingerprint deliberately (each bank opens with 128
  bytes of `$A5` twice over, code above the overlay). Measured: the engine reports **`mapper=F8SC banks=2
  superchip=true`**, and `Prove` now declines it naming both the mapper and the reason. Locked by tests in
  both directions — the superchip image must be declined, and a plain F8 cartridge must still be analysed
  per bank with its 2 banks and 6 regions, because a guard that refuses everything is sound and useless.
- **RL-8a — `framegen` draws missiles and the ball. DONE (v1.116.0).** RL-7 measured that the generated
  kernel emitted no `ENAM0`/`ENAM1`/`ENABL` at all, so those pixels were absent by construction. They are
  now measured, positioned and drawn wherever the kernel shape allows, and reported when it does not.

  **Measured first, designed second.** What the corpus actually does with missiles:

  | ROM | M0 | M1 | BL |
  |---|---|---|---|
  | `shared_setxpos` | 214/214 lines, **1 reset X**, 8 clocks | same, X 109 | 214/214, X 139, 2 clocks |
  | Fishing Derby | — | 42/214 lines, **1 X** | 58/214, **1 X** |
  | `road` | 195/214, **21 distinct X** | 194/214, 16 X | 7/214, 3 X |

  Three cases fall out, and the cheapest is the common one: an object on for EVERY line needs no per-line
  table at all — its enable is written once before the kernel, costing **zero** of the eight write slots.
  One that comes and goes costs one slot. One whose reset X moves cannot be placed by a kernel that strobes
  RESxx once, and is reported rather than approximated to its first position.

  `shared_setxpos`: **M0 1712/1712, M1 1712/1712, BL 428/428** — 3852 structurally-absent cells to none.
  Position calibration generalised from two objects to five (RESP0..RESBL and HMP0..HMBL are consecutive,
  so one SetXPos places any of them); all five converge in 4 iterations. Corpus: **one ROM changed**, 30
  byte-identical, 31/31 still 262 scanlines, generated clones still certify (74/76).

  **Four defects found on the way, each by a measurement rather than by reading:**
  - *Priority hid an object from its own calibration.* All five inputs start equal, so the objects pile up
    and TIA priority buries the lower ones — M1 was invisible until M0 moved off it. Freezing on the first
    non-appearance stranded M1 with all 1712 of its cells wrong; it now holds position and retries, and gives
    up only after four rounds.
  - *Chasing a missing object walks the input out of range.* 40 → 149 → 258, and `lda #258` does not
    assemble. Inputs are clamped and non-appearance no longer accumulates.
  - *The vertical-shift search moved the picture but not the enables.* `shifted()` shifted PF and GRP and
    left `en0/en1/enb` behind, so `motion_glide`'s ball came out on the wrong scanlines.
  - *The block cap dropped one class and did not re-check.* Fishing Derby emitted a **10-block, 80-cycle
    kernel against a 76-cycle line** while printing its own "only 8 fit" note, and the frame collapsed to 50
    scanlines. Dropping is now iterative and ordered — NUSIZ before enables, because a lost size mis-draws an
    object that is still there while a lost enable removes it — and never touches a picture write.

  Still not reproduced, with numbers: `road` (reset X moves 21/16/3 ways) and Fishing Derby's M1/BL (its
  playfield is reflect-asymmetric, so all six PF writes are needed and no slot is left). Per-zone sprite X
  remains open as **RL-8b**.
- **RL-8b — `framegen` per-zone sprite X. DONE.** The last RL-7 limit. `framegen` carried one reset X per
  object and strobed RESxx once, so a target that repositions down the frame could not be followed:
  `zone_multiplex` lost 190 cells on each player. It now emits a ZONE-STRUCTURED kernel — a replay loop per
  zone, separated by positioning blocks that run in the target's own blank lines — and where that does not
  fit it says so with the lines it counted.

  **The measurement that decides the scope, taken first.** A repositioning kernel does not need per-line
  RESxx; it needs the position to be constant over a BAND of lines and enough blank lines between bands to
  pay for the move. So the extractor now records the per-line reset X as a series rather than a histogram
  (`objFacts.distinctX` counts how many positions a target used and cannot say whether they are reachable)
  and folds it into bands with the gap before each one. Cost of one boundary: one scanline per object placed
  (each block opens with `sta WSYNC`) + one HMOVE line + one replayed blank line to leave GRP/PF at 0, since
  the replay loop is not running on the block's lines and the only picture reproducible there is none.

  | ROM | object | bands | band structure (measured) | gap needed | fits |
  |---|---|---|---|---|---|
  | `zone_multiplex` | P0 | 6 | L11-17@x49 gap11, L27-33@x79 gap9, L43-49@x117 gap9, L59-65@x19 gap9, L75-81@x9 gap9, L91-97@x97 gap9 | 4 | **yes** |
  | `zone_multiplex` | P1 | 6 | L11-17@x71 gap11, L27-33@x41 gap9, L43-49@x4 gap9, L59-65@x34 gap9, L75-81@x66 gap9, L91-97@x94 gap9 | 4 | **yes** |
  | `dyn_multisprite` | P0 | 3 | L72-85@x48 gap72, L140-141@x48 gap54, L142-153@x78 **gap0** | 4 | no |
  | `road` | M0 | 27 | L7-8@x38 gap7, L9-10@x82 **gap0**, L11-12@x90 gap0, L13-32@x98 gap0, … | 7 | no |
  | `road` | M1 | 23 | L7-8@x126 gap7, L9@x122 **gap0**, L13-33@x91 gap3, … | 6 | no |
  | `road` | BL | 4 | L7-8@x86 gap7, L9-10@x83 **gap0**, L11-12@x80 gap0, L13@x86 gap0 | 5 | no |
  | `rts_dispatch` | P0 | — | **1 reset X** (36, on all 37 drawn lines) | — | not a zone case |
  | `bitmap48` | P0/P1 | — | **1 reset X** each (87 / 95, 19 lines) | — | not a zone case |
  | `score6` | P0/P1 | — | **1 reset X** each (87 / 95, 8 lines) | — | not a zone case |
  | `text12` | P0/P1 | — | **1 reset X** each (87 / 95, 20 lines) | — | not a zone case |
  | `text24` | P0/P1 | — | **1 reset X** each (39 / 47, 10 lines) | — | not a zone case |
  | `hscroll` | — | — | **no player drawn at all**; its 64 cells are PF | — | not a zone case |
  | `shared_setxpos` | all 5 | — | **1 reset X** each (20/55/84/109/139) | — | not a zone case |
  | Outlaw | P0/P1 | — | **1 reset X** each (6 / 135, 48 lines) | — | not a zone case |
  | Combat | P0/P1 | — | **1 reset X** each (11 / 143, 14 lines) | — | not a zone case |

  So **five of the eight "placement differs" ROMs were never a per-zone problem at all** — one X per player,
  measured — and the RL-7 verdict's "the 8 placement-differs ROMs share one cause" was wrong about them. That
  is the RL-7c lesson holding a second time: the honest scope of this feature is `zone_multiplex`, and
  `dyn_multisprite` and `road` are refused with the counted reason rather than approximated.

  **Result: `zone_multiplex` 380 → 0 cells, pixel-exact** (BG 33808/33808, P0 228/228, P1 204/204), 262
  scanlines. Corpus after, over 31 technique ROMs + Outlaw and Combat with and without `-reset`:
  **22 pixel-exact / 8 differ / 1 partial, 262 scanlines on 35/35 runs**, and the generated source is
  **byte-identical for all 34 non-zone runs** (`dyn_multisprite` and `road` gain only the new
  `NOT REPRODUCED: per-zone X` note in the banner; the code below the comments diffs clean).

  **Three defects found by measuring the clone, each of which a plausible-sounding design would have kept:**
  - *The prologue's `SetXPos` cannot be reused inside the kernel.* Its div-15 loop costs `5k+22` cycles to
    the block's closing `sta WSYNC` write, so `k=11` — needed for reset X 4, because `k<=10` only reaches
    6..155 — pushes that write past cycle 75 and the block eats TWO scanlines. Measured symptom: a 263-line
    frame and a picture the shift search wanted to move 2 lines, i.e. **72 cells wrong that were not a
    positioning error**. Replaced with a branch-free fixed-cost block: `n x nop` + `sta.w RESxx` +
    `lda #nib` + `sta HMxx`, strobing at `2n+3` and closing at `2n+11`, one scanline for every reachable
    position and no page-crossing branch cycle to account for.
  - *The reset marker is not the drawn window.* The same 8-pixel sprite line reports reset X 49 with span
    49..56 in one band, 117 with span 116..123 in another and 4 with span 4..11 in a third — the marker is
    off by −1, 0 or +1 with the fine HMOVE nibble that placed the object. Calibrating the clone's marker onto
    the target's marker therefore leaves the picture a pixel out (**12 cells**, all of them one edge pixel of
    a band's widest lines) and sampling the graphics byte at the marker reads the wrong 8 clocks. Both sides
    are now anchored on **the leftmost clock the object was actually drawn at**, minimised over the band
    because the sprite is 2,4,6,8,8,6,4 pixels wide down each band and only the widest lines show its edge.
  - *The historical block order is wrong for a moved sprite.* `GRP1` sits fifth and lands at visible clock
    +37, fine for a player parked on the right and **64 cells wrong** for `zone_multiplex`'s band at reset X
    4. A zone plan now takes the deadline-sorted layout (which also drops the 6 playfield writes this target
    provably does not need — all three PF registers 0 on both halves of all 214 lines, 42 cycles freed).

  Not reproduced, with the number: `dyn_multisprite` (P0 goes 48 → 78 at visible line 142 while it is still
  being drawn — **0 of the 4 blank lines** the move needs), `road`'s M0/M1/BL (**0 of 7 / 0 of 6 / 0 of 5**
  blank lines at their first change, line 9), and the five one-X ROMs above, whose causes are copies and the
  block budget (RL-7c) and are unchanged.

  **Gates, all measured on the 35 runs.** `cyclebound -asm` returns `certified:true` on **35/35** clones —
  `max_worst` 74/76 on the 34 single-zone ones and **66/76 on the zoned one**, whose worst region is a
  positioning line, so the fixed-cost block's own arithmetic bound (`2n+11 <= 75`) is confirmed by the repo's
  prover and not only by hand. `timinglint` reports no warnings on the zoned clone (its HMxx-then-HMOVE order
  and the 24-cycle rule), and `vismatch` — a different code path from `framegen`'s own coverage table —
  independently returns `pixel-exact, band diffs: none` for `zone_multiplex`, Outlaw and Combat.

  **Found and NOT fixed, deliberately.** Every pixel-exact clone's banner ends with *"Every element is
  present and every object cell matches; the difference is in BG cells only."* — on a clone with **zero**
  differing cells that sentence is false, and it is on all 22 of them (verified in the pre-change output too,
  so it predates this work). Correcting it would rewrite the banner of all 34 byte-identical clones, which is
  exactly the regression gate this change had to hold, so it is reported here instead of fixed here.

### SD-11a — the cross-bank rekey shipped an under-approximation, found by review after it was pushed (2026-07-29)

I verified SD-11 against the machine before accepting it — for every region the prover bounds, on all four
bank ROMs, proven ≥ measured with **0 violations** — and pushed it. That verification was sound *for what the
corpus exercises*. The adversarial pass then reported two `unsound` findings on cases the corpus does not
contain, one of them a **regression this change introduced**. Confirmed by reading the code, not taken on
report.

**The defect.** A JSR records its return point as `ctx{ret: in.nextSite()}`, which stamps the **caller's**
bank unconditionally. If the callee switches bank and returns WITHOUT switching back, the hardware resumes at
that address in the **new** bank — different bytes, different cost — and costing the caller's bytes there is an
under-approximation, the one direction this package forbids.

No ROM in the corpus does it: every trampoline switches back before returning. So there was no witness, the
machine gate could not see it, and my proven-≥-measured check passed while the hole was open. **That is the
limit of grading against a corpus, stated plainly.**

**Refused rather than modelled**, because following it properly means carrying the bank across the call, which
is more than this stage does. The refusal names both banks. Verified: all four bank ROMs still prove and none
trips the new guard (their trampolines do switch back), and the golden JSON is **43 of 43 byte-identical** —
this touches nothing that was already working.

**What this says about the process, which matters more than the fix.** The machine gate and the adversarial
pass find different things and neither substitutes for the other: the gate proves the numbers agree on what
the corpus runs, the review finds what the corpus never runs. Accepting a change on the gate alone — as I did
here, pushing before the review returned — is how an unsound path ships. The review took 2h57m against my
gate's few minutes, and it was the one that found this.

**Three of the six are now closed (SD-11b), each measured rather than taken on the review's word — and one
of its numbers did not survive.**

1. **The convergence skips were real but smaller than reported.** The review counted "six `t.Skip` sites that
   turn the machine gate off while CI stays green". Measured: **28 skip sites exist and only 2 fire today**,
   both missing commercial ROMs (`VideoOlympics`, `Stampede`), not convergence. The structural point stands:
   **5 sites skipped the whole assertion when the abstract-interpretation fixpoint failed to converge** — on
   litmus ROMs written to converge, where a failure means the premise broke, not that the test is
   inapplicable. All 5 are now `t.Fatal`. Negative control: forcing non-convergence makes **6 assertions
   fail** where they previously passed in silence.
2. **`HasSuperchip()` really is Atari-only.** `grep -rln "func.*HasSuperchip"` over the vendored cartridge
   package returns `mapper_atari.go` and the dispatcher, nothing else, so every other mapper answers false
   through the type assertion — while **3E+ and M-Network overlay cartridge RAM and set
   `banking.Information.IsRAM`**. New `emu.MapsCartridgeRAM()` asks the engine's own flag across the whole
   window (M-Network maps RAM into a SEGMENT, so a single-address sample answers differently depending on
   where you ask) and additionally declines `3E`/`3E+`/`E7`/`AR` by ID, because those map RAM in only after a
   switch and a boot-time look would answer "no" for a cartridge that maps RAM a frame later. Verified:
   `F8SC` true, plain `F8` and `4K` false, golden **43 of 43 byte-identical**.
3. **`analysisUnits` could return an empty unit list with no decline reason** — reachable when `CopyBanks`
   yields nothing while `CartInfo` reports more than one bank. Every caller reads "no decline" as permission,
   then finds nothing to analyse and reports zero regions, which the 0-region backstop turns into "not
   certified" rather than "I was handed nothing". It now declines by name, and also declines when the
   readable bank count differs from the reported one — analysing a subset would certify on whichever part
   happened to be available.

**Two more closed (SD-11c), and one of the six was already handled.**

4. **`determineBound`'s divide-loop path skipped predecessors it could not bound.** Its own comment promised
   *"Unknown => 0 (stay unbounded)"* and the code did not do it: a predecessor with no abstract state, or one
   whose `A` is `Top`, was **skipped**, and the ceiling taken as the maximum over whichever predecessors
   happened to be known. Skipping is not neutral — a maximum over a subset is a **lower** bound on the real
   maximum, so the trip count and the proven worst case both come out too small. Same function, same
   direction, as SD-9's fortyfold under-approximation on the `dex`/`dey` path. An unknown predecessor now
   forces the inferred range to be discarded (the author's `@amax` annotation may still answer; an inferred
   range may not). **Golden 43 of 43 byte-identical** — no precision lost, only the unsound path removed.
   Guarded by a new standing machine gate: **228 bounded regions across 31 ROMs, none below what the emulator
   measured.**
5. **The Atari-only cross-bank edge semantics were already guarded**, which measurement showed rather than
   argument: `analysisUnits` declines any bank that is not 4K or whose `Origins` is anything but
   `$F000` alone, so M-Network's 2K-at-two-origins layout cannot reach the edge model. Verified end to end —
   `litmus_bank`/`f6`/`f4`/`banked_game` analysed, `litmus_superchip` declined by name, `litmus_lastline`
   analysed as flat.

**Still open from the same review, recorded rather than fixed** (each needs its own measurement):
(all three are closed below).

> **★CLOSED 2026-07-30 — Stage 4: the edge model is now named per mapper.** The cross-bank edge this package
> models is Atari's — *the access selects the bank its hotspot SYMBOL names, PC is untouched, the next fetch is
> the following address in that bank* — and it was applied to any cartridge that merely LOOKED like Atari's.
> Every gate before it is **geometric** (bank count, 4K per bank, one origin at `$F000`, no mapped RAM), and
> geometry says nothing about how a mapper picks its target bank.
>
> **WF8 is the measured proof, not a hypothetical.** It is 8K, two 4K banks at `$F000`, no RAM, and it publishes
> `$1FF8:BANK0` / `$1FF9:BANK1` — it clears **every** geometric gate. Its actual switch
> (`mapper_atari_wf8.go` `wf8.bankswitch`) responds **only to `$0FF8`** and takes the target from **data bus
> bit 2**. So the old model would (a) invent an edge for `$1FF9`, an address that does nothing at all on that
> cartridge, and (b) send `$1FF8` to bank 0 on the strength of a symbol when the hardware goes to bank 0 **or**
> 1 depending on the value written. Both are edges the machine does not take, and a wrong edge can **shorten**
> the longest path.
>
> `verifiedEdgeSemantics` now lists the IDs whose rule was READ in the engine source, and the value is the
> evidence rather than a label — each cites the file and the `bankswitch` function: **F8, F6, F4, EF, BF, DF,
> JANE** (JANE takes a `data` argument and does not read it). `knownDifferentEdgeSemantics` names **WF8/WFSC**
> so the refusal says what is actually wrong instead of "unverified". Everything else declines with the list of
> what has been checked. **Of the engine's 32 mapper IDs: 7 verified, 25 declined.**
>
> The property that survives an upstream release is that the DEFAULT is decline: a mapper added to Gopher2600
> tomorrow is refused by name, not analysed under Atari's rule because nobody noticed.
> `TestEdgeSemanticsAreNamedPerMapper` pins all 32 IDs, requires WF8's refusal to state its real reason,
> requires every "verified" entry to cite a `.go` file and `bankswitch`, and fails if the table ever verifies
> nothing or declines nothing. `TestCorpusBankROMsUseVerifiedMappers` drives the real path so the table is
> exercised rather than merely present. All 122 corpus ROMs byte-identical. Negative control: restoring
> accept-everything fails with 25 named mappers.

> **★CLOSED 2026-07-30 — the empty unit list, and the answer is "not reachable, and here is the proof".**
> The concern was that `analysisUnits` could return an empty unit list with no decline reason, which every
> caller reads as permission to proceed before finding nothing to analyse and reporting zero regions — and a
> zero-region report reads as a clean one. **Measured: it cannot.** Deleting the dedicated `len(units) == 0`
> guard and re-running shows the function still declines, through `len(units) != banks` (0 ≠ 2), only with a
> vaguer reason. Since `analysisUnits` returns early for `banks <= 1`, there is no input where the count check
> lets an empty list through. The dedicated guard is redundancy that buys a specific message, not the thing
> standing between the caller and a silent empty analysis.
>
> What was genuinely missing is that **none of these guards had ever run**. They fire only on cartridge shapes
> no ROM in this repo has — an empty bank, a 2K segment, a bank at a second origin, fewer banks readable than
> reported — so they were untested code that could not be shown to work. The validation is now split out as
> `unitsFromBanks(id, banks, contents, hotspots)`, a pure function over fabricated bank images, and
> `TestUnitsFromBanksDeclinesByName` drives all five declines plus the accepting case (or a function that
> refused everything would pass). Pure refactor: all 122 corpus ROMs byte-identical.

> **★CLOSED 2026-07-30 — the absent abstract state.** The third item read: `determineBound`'s predecessor scan
> passes `absStates[pred]` where an absent entry is the zero `State`, whose ranges are `Top=false, Lo=0, Hi=0`
> — **exact zero**, not unknown. It is worse than the entry suggested, because the same shape is at EVERY call
> site: nine places index `absStates[at]` / `states[a]` and a Go map miss yields that zero state.
> `accessOf` then reads `st.SP.konst()` and `st.X`/`st.Y` from it, so a `PHA` is modelled as writing precisely
> `$0100` and `lda table,x` as reading precisely `table`, and `switchEdges` decides from that footprint whether
> the instruction can reach a bank-switch hotspot. A narrower footprint MISSES a hotspot → drops a cross-bank
> successor → shortens the predecessor set `determineBound` maximises over → under-approximates the trip count.
> Forbidden direction.
>
> **Measured before fixing** (Prove + BeamIntervals + DefUse + Lint over `roms/techniques` + `roms/litmus`):
> **1,994,520** `successors` calls · **2,572** with no usable state · **212** of those on a bank-switched
> cartridge where the state is actually read · **124** still producing a concrete address · **6** whose
> footprint genuinely differs from the sound answer. Six, not zero.
>
> Fixed at the funnel: `successors` substitutes `topState()` for any state that is not `valid`.
> **No corpus output changes** — 113 flat and 6 banked images byte-identical — so this is a hole closed before
> it bit, not a wrong number repaired, and that is precisely why it needed its own test.
> `TestAbsentAbstractStateIsTopNotZero` builds `lda $1F00,X` against a 2-bank hotspot table and pins all three
> readings: the zero state resolves to **1** address and **0** cross-bank edges, `Top` to **256** addresses and
> **2** edges, and a *proven* `X=0` must still yield only the fall-through so the fix is not "assume the worst
> everywhere". Negative control: removing the substitution makes `successors` return 1 successor instead of 3.

### The prover's reach on REAL games, measured for the first time — and the backlog was ranked on the wrong corpus (2026-07-30)

The `.bin` entry point landed, so the prover was pointed at five commercial cartridges. Nothing here had
ever been measured against code we did not write.

| ROM | regions | bounded | unbounded | over budget |
|---|---:|---:|---:|---:|
| VideoOlympics (2K) | 8 | 3 | 5 | 1 |
| Adventure | 14 | 8 | 6 | 1 |
| Seaquest | 49 | 35 | 14 | 7 |
| Chopper Command | 29 | **5** | **24** | 3 |
| Empire Strikes Back | 61 | 32 | 29 | 10 |
| **total** | **161** | **83** | **78** | 22 |

All five converge. **52% of regions in real games are bounded**, against 14/31 kernels certified in our own
corpus — a different kind of number, but the first honest one about commercial code.

**The refusal reasons are ranked completely differently from ours, and that is the finding.**

| reason | commercial | our corpus (recorded in SD-3) |
|---|---:|---:|
| multiple back-edges (nested/complex loops) | **21** | 2 |
| WSYNC inside loop body | **20** | 2 |
| branch inside loop body | **13** | 1 |
| no WSYNC reached from region start | 9 | 5 |
| BRK in region | 8 | — |
| **loop bound unknown** | **5** | **15** |
| nested subroutine call | 2 | — |

SD-3's conditional-bounds work was chosen by measuring our own kernels, where *"loop bound unknown"* was 15
of 29 refusals — "the dominant case", and it was, for us. On real games it is **5 of 78, six percent**, and
the three reasons that dominate (multiple back-edges, WSYNC inside a loop, a branch inside a loop — 54 of
78 between them) were ranked 2, 2 and 1 in our corpus and never worked on.

That is the same defect this file keeps recording, one level up: not a wrong number, but a **denominator
chosen from what was at hand**. A backlog ranked on fixtures optimises for fixtures.

**Chopper Command is the outlier worth naming**: 5 of 29 regions bounded, 17%. Whatever it does, the prover
mostly cannot follow it, and it is the cheapest available specimen for the three dominant reasons.

**The 8 "BRK in region", probed — sharper, still not settled.** The hypothesis was that the decoder walked
into data and read `$00`. Measured: the BRKs are **concentrated in one ROM**, not spread. Empire Strikes
Back decodes **20** of them; Seaquest 1; Chopper Command and Adventure **zero**. Of Empire's 20, **19 are
never executed** in a 200k-instruction attract-mode run — and **one is**. So "all phantom" is already
falsified, and "all real" was never likely. Settling it needs flow reachability from the vectors, not more
counting; recorded here so the next attempt starts from the distribution rather than from the total.

**The bigger number the same probe produced: static decode reaches about twice what execution does.**

| ROM | decoded | executed (200k instr, no input) | never executed |
|---|---:|---:|---:|
| Chopper Command | 2358 | 1108 | **1250 (53%)** |
| Empire Strikes Back | 2478 | 803 | **1675 (68%)** |
| Seaquest | 1656 | 858 | 798 (48%) |
| Adventure | 1236 | 760 | 476 (39%) |

**It is not attract mode — measured 2026-07-30.** Driving the ROMs properly changes the numbers by almost
nothing: Chopper Command 1108 idle → 1108 after RESET → 1112 with RESET plus twelve rounds of scripted
joystick input; Seaquest 858 → 858 → 860; Adventure 760 → 760 → 774. RESET is genuinely doing something —
**53 of 128 RAM bytes differ** with it versus without on Chopper Command, 25 on Seaquest, 26 on Adventure —
so the switch works and the machine moves to a different state; it moves there **through the same
instructions**. Which is how 2600 attract modes are built: the demo runs the game loop with synthetic input.
So the unexecuted half is elsewhere — other game variations, difficulty paths, death and scoring branches —
and simple scripted input will not reach it. "Never executed" is still not "unreachable", but it is no
longer explained away by the ROM sitting on a title screen.

*Two instrument failures on the way to that, both caught before it was written down.* The first run pressed
nothing: `emu.SetInput` rejects console switches — they live on `SetPanel` — and its error was returned and
ignored, so "RESET changes nothing" was measured on a RESET that never happened. The liveness probe added
that morning is what exposed it. The second: with RESET working, the coverage was again identical to the
instruction, which reads exactly like the first failure; distinguishing "the switch did nothing" from "the
switch did something and the code path is the same" needed a **different observable**, and the RAM diff is
what settled it.

But the size of the gap matters for how this project grades itself. **Every soundness grading here is dynamic
containment**: `defuse` at 32655/32655, `beam_intervals` at 19143/19143, "observed ≤ proven" at 896
regions. Those check that what the machine DID falls inside what the analysis PREDICTED. On a commercial
cartridge the machine does about half the decoded program, so such a grading would validate roughly half
the static claims and say nothing about the rest. On our own kernels — small, driven by scenarios that
exercise them deliberately — the gap is much smaller, which is why it has never shown up. A grading that
is ∃ over a corpus that is mostly unexercised is a weaker statement than its numerator suggests.

### A wrong number in "Constants you must never get wrong" (2026-07-30)

`CLAUDE.md` is the only document loaded in full every session, and the section named "Constants you must
never get wrong" carried one that was wrong — and contradicted itself while carrying it. The horizontal
position paragraph said **"Leftmost X=2 (player 3)"** and, three lines later, **"leftmost X=3"**, both
labelled hardware-verified.

Measured rather than argued, with this repo's own instruments. `cmd/calibrate` sweeps `litmus_pos`'s DELAY
and reports `12 -> 2`, `13 -> 3`, `14 -> 3` for player0's `ResetPixel` (slope 3.0000 px/CPU-cycle,
R² = 1.000000 — the slope claim holds). Confirmed at the pixel, because the file's own iron rule says the
verdict is the drawn position: at `DELAY=12`, `read_tia` gives `reset_pixel` = `hmoved_pixel` = 2 and
`decompose_row` shows **P0 occupying clock 2..9**. A player draws from clock 2.

The deeper error is the category, not the digit. Two sentences after stating a leftmost-X constant, the same
paragraph warns that the offset constant is **kernel-specific** and that the verdict must be measured — and
a leftmost reachable position is exactly that offset at its wrap. Stating it as a machine constant is the
mistake the paragraph itself warns against. The line now records the measurement and says there is no such
constant. The missile/ball figure was left alone and is marked as not re-measured, rather than corrected by
analogy.

### The bank-blind address, closed: six instances in two syntaxes, and one place that already knew (2026-07-30)

The sweep that started with two gradings ended at six, and the shape of the search mattered more than the
search did.

| # | site | what it did | found by |
|---|---|---|---|
| 1 | blank-classification grading | graded code from the other bank | `map[uint16]` scan |
| 2 | "machine never exceeds the proof" | **the hiding direction** — a chance pairing that satisfies `observed <= proven` buries a real gap | `map[uint16]` scan |
| 3 | `emu.Coverage` (all four sets) | count 12% low; `Seen()` wrong in the **flattering** direction | `map[uint16]` scan |
| 4 | `mutate -covered` offsets | all 278 covered offsets in bank 1 on an 8K image — mutating the half that did not run | **expression** scan |
| 5 | `assert_line_budget`'s `patch=` | silently rewrote bank 1; the bounds check passed | **expression** scan |
| 6 | `dissect` annotation matching | bank 0 resolved to `$Exxx`, matched no label, annotations dropped in silence | **expression** scan |

**The `map[uint16]` scan found none of 4–6.** Those write the same mistake as arithmetic — `addr & (len-1)`,
`0x10000 - len(rom) + off` — and no key-shaped search reaches them. One sweep does not close a class; the
FORM of the sweep has to change too, and that is the transferable lesson here rather than anything about
banks.

**One site already knew.** `internal/cyclebound`'s `program.canon` is correct because `newBankProgram` wraps
each bank as the self-contained 4K image it is, and the flat path *declines* a banked image outright — with
the reason recorded as a measurement in the struct comment: the flat model decoded 66 instructions from
`banked_game` and "produced a confident finding about an address it had never decoded". That comment is why
that site was not the seventh instance.

**Not every one was fixed the same way.** Where the bank was recoverable, the code now keys on it (1, 2, 3,
4, 6). Where an address genuinely cannot identify a byte and the API has nowhere to say which bank, it
declines (5) rather than guessing — `PatchSpec` carrying a bank is filed, not invented.

### The bank-blind address key, third instance: coverage itself (2026-07-30)

Two gradings were found keyed on a ROM address with no bank on the same day — the blank classification and
the "machine never exceeds the proof" check — so the pattern was swept rather than waited on. 97 uses of
`map[uint16]` exist outside the vendored engine; almost all are about a synthetic memory image or a single
run and do not care. One does.

**`internal/emu/coverage.go` keys all four of its sets on a bare address**: instructions executed, branches
seen, and the taken / not-taken edge sets. On an 8K cartridge the two banks decode the SAME addresses, so
bank 0's `$F123` and bank 1's `$F123` are one entry. Measured over 200k instructions:

| ROM | distinct `(bank,pc)` executed | `Coverage.PCCount()` reports | addresses run in BOTH banks |
|---|---:|---:|---:|
| `exerciser.bin` | 319 | **282** | **37** |
| `banked_game.bin` | 74 | **69** | **5** |
| `litmus_bank.bin` (4K logic) | 46 | 46 | 0 |

It fails in both directions at once. As a count, `PCCount` **under-reports** distinct executed instructions
— 282 of 319 on the exerciser, 12% missing. As a query, `Seen(addr)` **over-reports**: it answers "covered"
for the twin instruction in the bank that never ran, which is the flattering direction and the one that
matters, because VV-3's coverage percentage and `mutate -covered`'s "honest kill rate over executed code
only" both rest on it.

**FIXED, with the opinionated half left alone.** The worry was that keying on `(bank, addr)` changes the
meaning of the exported `Seen(addr)` and of numbers already published. Measuring split that worry in two:
on a **flat 4K image every bank is 0**, so the new key is a no-op there — verified by diffing `cmd/cover`'s
entire JSON output before and after on five flat ROMs, **byte-identical 5/5**. Only banked images move, and
there the old numbers were wrong.

So the recorder now keys on `(bank, addr)` and `Seen(addr)` **keeps its meaning** — "executed at this
address in SOME bank" — documented in place, with `SeenIn(bank, addr)` added beside it. Nothing published
changes except on banked images: `cmd/cover`'s `pc_executed` on the exerciser goes **268 → 297**. The
fetch bank is captured BEFORE the step, because a hotspot access changes the mapping as it completes and
asking afterwards attributes the switching instruction to the bank it switched TO. `Signature()` includes
the bank as well, so coverage-guided fuzzing on an 8K image can tell the two halves of an address apart
instead of concluding it has seen everything.

Witness: `TestCoverageDistinguishesTheSameAddressInTwoBanks` (319 pairs, 37 addresses in both banks) checks
its own premise — it fails if the ROM stops exercising the collision — and asserts the flattering direction
directly, that a twin in the bank which never ran does not read as covered. Negative control: forcing the
bank back to 0 fails both assertions.

### `displayPreserver` swept: the precise answer fires 61 times and has never been graded (2026-07-30)

Same method, next function on the blank-classification path. `displayPreserver(entry)` answers "does this
subroutine, and everything it calls, provably not touch VSYNC/VBLANK". A `true` makes the abstract
interpreter **carry the display state across the JSR** (`absint.go:723`); a wrong `true` therefore keeps
"display is off" alive past a routine that actually turned it on, and a later region can be classified
BLANK and dropped from the budget check. Precise side, and the consequence is a deleted check rather than
a loud failure.

| branch | hits | side |
|---|---:|---|
| walk stops at a return | 87 | precise |
| **`PRESERVES` — the answer itself** | **61** | **precise** |
| a store touches the display -> NOT | 33 | conservative |
| a callee does not preserve -> NOT | 6 | conservative |
| unfollowable switch -> NOT | 2 | conservative |
| **too deep / recursive -> NOT** | **0** | conservative |
| **a push touches the display -> NOT** | **0** | conservative |

Two unwitnessed, both refusals, so neither can under-approximate — the same shape as the display-miss
sweep, and no change is called for. The `push touches` zero is the third place the push predicate turns
out to be barely exercised; `cb_pushdisplay` does not cover it either, because its dangerous push is in a
region rather than inside a `jsr`'d routine. Deliberately NOT fixed by adding a third fixture: colouring a
conservative branch green is coverage theatre, and the sweep exists to find the opposite.

**CORRECTION (same day, before building on it): "zero verdicts" was wrong.** The first version of this
entry said `displayPreserver` has 61 executions and no verdict, on the strength of a grep showing it named
in no test. The grep was right and the conclusion was not: its only consumer's OUTPUT is graded against the
machine by `TestBlankClassificationAgreesWithTheMachine`, which runs each ROM and asserts that a region the
prover called BLANK is never reached with the display on — **144,568 executions of blank-region entry points
across 32 ROMs, 0 disagreements**, and it reports the 1 blank region never reached as explicitly not
covered. A wrong `true` from `displayPreserver` carries "display off" past a routine that turned it on, and
that shows up there. The function is unnamed in the tests; its consequence is not ungraded. Checking before
extending is what caught this — the same move that corrected the `cb_deadpred` mechanism earlier today.

**What IS missing is corpus, and trying to add it found an oracle defect.** The grading runs on
`roms/techniques/*.asm` plus one litmus — 32 of the ~129 ROMs. Extending it to the litmus and exerciser
corpora produced **6730 disagreements**, every one of them on a frame's VSYNC lines in a ROM that raises
VSYNC without also raising VBLANK. The prover's `displayOff()` is `VSync || VBlank`; the oracle,
`emu.DisplayOff()`, read **only** `sig.VBlank`. VSYNC blanks the picture as surely as VBLANK does, so the
oracle was short a term — fixed, and the error direction was false ALARMS rather than missed detections, so
nothing unsound had shipped. What it had done instead was silently cap this grading to ROMs that happen to
raise VBLANK during VSYNC, a property nobody chose.

**RESOLVED (next tick, measured): both residual groups were defects in the GRADING, not the prover, and
the corpus is now shipped at 128 ROMs.** The 776 split cleanly once probed at the exact failing PCs:

- **`litmus_bound_proxy` (336) — the sampling instant.** A TIA register write is *delayed*
  (`tia.futureVblank`), so at the region's opening `sta WSYNC` a `sta VBLANK` issued one instruction
  earlier has not reached the signal yet. Measured: `DisplayOff()` is **false at the strobe and true one
  instruction later**, for a region that is genuinely blanked. The test asked its question at the wrong
  instant — the claim is about the LINE the region opens, not about the strobe.
- **`exerciser` (440) — a bank-blind key.** `blank` was `map[uint16]bool` keyed on the region's address
  alone, and an 8K image decodes the SAME addresses in both banks, so the run matched whatever sat at
  `$Fxxx` in the *other* bank. Measured: the probe landed at scanline 36 with the picture on, nowhere near
  the frame-top region the prover had classified. **`banked_game` is in the original 32-ROM corpus**, so
  part of the old number was aimed at the wrong instructions all along.

Both fixed (key on `(bank, addr)`; sample after the strobe retires). Result: **128 ROMs, 133,684
executions, 0 disagreements**, up from 32 ROMs. The per-ROM instruction budget went 400k → 120k in the same
change, measured rather than guessed: at 400k the run costs 180s, at 120k 57s, and *blank regions never
reached* — the coverage number, the one that says what this grading does NOT see — is **1 either way**. The
extra 280k instructions bought repeat executions of entry points already covered.

**Superseded — kept for the record:** After
the fix they are confined to `litmus_bound_proxy` (336) and `exerciser` (440). Probed at the exact failing
PCs, both sit **one instruction after** the store that turns the display off, and `GetLastSignal()` there
still reports the pre-write pixel (`sig.VBlank=false sig.VSync=false` at `litmus_bound_proxy $F038`,
scanline 228 clock −41, immediately after its `sta VBLANK`). The hypothesis is therefore a sampling-point
problem in the oracle — the TV's last EMITTED signal lags the register write by a few colour clocks — and
not a prover defect. **It is a hypothesis, not a result:** the register value at that instant was not read,
because `ReadTIARegisters` does not expose VBLANK/VSYNC. Until that is settled the corpus stays at 32 ROMs
rather than shipping a red test or a weakened assertion. Next step: expose the VBLANK/VSYNC register state
and re-probe; if the registers say blanked while the signal does not, move the oracle to the registers and
extend the corpus (144,568 → 446,497 executions, 32 → 128 ROMs).

**Also filed, unchanged:** a direct grading of the predicate rather than its consequence — record every
write to VSYNC/VBLANK with its PC, and for every entry declared preserving, assert no observed write lies
inside that callee's body (`reachableWithinCallee` already answers the containment question).

### The display-miss predicates swept: 3 unwitnessed branches, all conservative — and a fixture that misses its own hazard (2026-07-30)

Next function through the witness method, chosen because it sits on the **precise** side: `storeMissesDisplay`
/ `pushMissesDisplay` / `indexedStoreMissesDisplay` decide whether a region can be classified BLANK and
skipped, so a wrong "misses the display" drops a region's cost — an under-approximation, the direction this
package forbids. Branch hits over 129 ROMs:

| branch | hits | side |
|---|---:|---|
| `store abs: TIA but not $00/$01 -> MISSES` | 1146 | precise |
| `store abs: not TIA -> MISSES` | 411 | precise |
| `store abs: IS display -> touches` | 361 | conservative |
| `store: delegate to indexed` | 55 | — |
| `idx: no state -> touches` | 18 | conservative |
| `idx: unknown range -> touches` | 17 | conservative |
| `idx: proved -> MISSES` | 17 | precise |
| `idx: reaches display -> touches` | 3 | conservative |
| `push: proved -> MISSES` | 1–2 | precise |
| **`push: unknown SP -> touches`** | **0** | conservative |
| **`push: SP reaches display -> touches`** | **0** | conservative |
| **`idx: indirect -> touches`** | **0** | conservative |

**All three unwitnessed branches are refusals.** They can only fail by refusing too much — classifying a
genuinely blank region as display-touching, which costs precision and never soundness. That is the safe
side, and unlike the `pagePenalty` sweep (where the one real defect sat on the single unwitnessed *precise*
branch) there is no precise branch without a witness here. No change made.

**RESOLVED — the fixture was written (`cb_pushdisplay` / `cb_pushsafe`).** Original finding: `pushMissesDisplay` is reached by exactly ONE ROM in the
corpus — `roms/techniques/rts_dispatch.asm` — and **`litmus_stack_trick` reaches it 0 times**. That ROM
exists because a program can aim SP at the TIA and turn a `PHA` into a register write, which is the entire
reason this predicate is not just "every push touches the display". The hazard has a fixture and the
predicate has a witness, and they are not the same ROM: nothing in the corpus exercises the branch where
**SP actually can reach $0100/$0101**. Written: `cb_pushdisplay` puts a `PHA` with **SP = 1** — so the write lands on `$0101`, VBLANK — inside an
overscan region that has VBLANK already on and no display stores, i.e. a region that would otherwise be
classified BLANK and have its cost skipped. `cb_pushsafe` is the same kernel with **one immediate changed**
(`STACKTOP = $FF`, landing at `$01FF`, ordinary stack RAM). Measured: the danger ROM's region comes back
`visible`, the twin's comes back `blank`, both at 16 cy, and the blank-region count differs by exactly one —
the code is identical, only the verdict about it moved. The branch itself was confirmed directly rather than
inferred from the classification: `reaches-display sp=1 ma=$01`. Negative controls: making every push safe,
and making every push dangerous, each fail the test.

**One fixture defect was caught before shipping.** The first version put the push in a region that ran on
past `jmp Main` into the next frame's `sta VSYNC` — regions are WSYNC-to-WSYNC — so both twins were
classified visible and the fixture proved nothing. Closing the region with a trailing `sta WSYNC` fixed it;
the ROM comment records why that WSYNC is load-bearing.

*Measurement note:* two shell aggregations of the push count disagreed (1 vs 2) and the discrepancy was in
the throwaway instrumentation, not in the harness; it was not chased because the count does not bear on the
finding — one ROM, both refusal branches at zero. Recorded rather than smoothed over.

### `dec: unknown predecessor` witnessed at last — and it took two failed fixtures to get there (2026-07-30)

The last branch left from the `determineBound` sweep. It had run **0 times across 123 ROMs**, and unlike
its shadowed sibling (`dec: successor refusal`, which a coarser region-level guard always beats to the
punch) nobody had shown it unreachable — it was simply unwitnessed, which is the worst state for a guard
to sit in, because "nothing reaches it" and "it is right" are different claims and neither was made.

**Two fixtures failed first, and both are findings.**

1. *Code hopped over by a `jmp` never becomes a predecessor at all.* The decode follows flow, so an
   unreachable instruction is never decoded, never enters the region's node map, and the scan cannot see
   it. Measured by instrumenting the scan: it listed 9 candidates and the dead instruction's address was
   not among them.
2. *Dead code in the region ABOVE the header is not seen either.* The predecessor scan is per-region, and
   the header of a WSYNC-free delay loop lives inside the region the preceding WSYNC opened — the first
   attempt put the dead instruction one region too early.

What reaches it is the **not-taken edge of a branch whose condition is statically known**. That edge IS
decoded (the decoder cannot prove which way a branch goes). `lda #0` / `beq` over a `ldy #200` that falls
into the delay loop hit the guard at `bank0 $F035 -> header $F037` on the first run.

Shipped as the twin pair `cb_deadpred` / `cb_deadpred_live`, differing by that one pruned edge and
nothing else, so the refusal is attributable: the dead one leaves its visible region unbounded ("loop
bound unknown"), the twin comes back with all 7 regions bounded and a dearer worst region (33 vs 17 cy),
which also proves the delay loop is really being counted there. Negative control: removing the guard
(silently skipping the unknown predecessor — the pre-SD-11a behaviour) makes the dead ROM come back fully
bounded, and the test says so.

**RESOLVED, and the first write-up of it was wrong.** The open question was whether the guard is
over-conservative: it refuses on `!ok || !st.valid`, and the hypothesis was that `!st.valid` marks a
**provably unreachable** edge, which cannot contribute an entry value, so skipping it would be sound and
more precise. Instrumenting the two conditions separately over 129 ROMs settles it, and against the
hypothesis:

- **`!st.valid` fires ZERO times anywhere, including in the fixture built to hit it.** A pruned edge never
  acquires a state to be invalid: `absSuccessors` emits only edges whose refined state is still valid
  (`if tk.valid` / `if nt.valid`), so the target is never pushed into the map at all. In this function
  that condition is as unreachable as the sibling branch the fixture replaced.
- **`!ok` is the live one** — one hit, `cb_deadpred` at `$F035` — and it is **not** "proven unreachable".
  A missing entry means proven-unreachable OR **never analysed**: a fixpoint that hits its iteration cap
  leaves work on the queue and returns `converged=false`, and those nodes have no entry either.

So the refusal must stay. Relaxing it would drop a real predecessor whenever the fixpoint was capped, and
under-approximate the entry value — the one direction this package forbids. **No change made, and the
reason is now measured rather than assumed.** The earlier text in this section, which named the
invalid-state route as the one the fixture takes, was corrected in place; the fixture and its twin are
right, only the mechanism named for them was wrong.

### TD-* — All 38 MCP tool descriptions audited against their handlers: 8 disagree (2026-07-30)

Three instances of "the tool's description does not match what the tool does" were found by hand in two
days — `timinglint` reading 0 of 133 instructions while reporting nothing, `spritey` advertising only the
Y half of a trajectory that always carried X, and then the *repair* of that one landing on the field tag
while leaving the tool's own Description saying Y-only. Three hand-found instances of a class is the point
at which you stop finding them by hand. All 38 tools were sharded across parallel readers, each comparing
the shipped Description and jsonschema tags against the handler and the packages it calls; every candidate
then went to an independent skeptic instructed to refute it and to default to refuted when unsure.

**10 candidates → 8 confirmed, 2 killed.** Two of the killed ones matter as much as the survivors: a
`read_cycles` finding whose harm argument ran backwards, and a `set_input` finding claiming the liveness
verdict is unadvertised when it is in the same tool definition. Both were killed by a verifier who built
the server and read the live wire schema rather than the source — which is the standard the whole sweep
was held to.

| Tool | The description says | The code does | Class |
|---|---|---|---|
| `step_scanline` | "CPU cycles consumed across that scanline" | `TotalCycles()` delta, which **excludes WSYNC stall cycles** (`emu.go:142,144,159`). Measured: `cycles_consumed=8` on twelve consecutive 76-cycle lines of `smoke.bin` | wrong measurement |
| `read_row` / `decompose_row` | tag: "visible scanline (**0-based**…)" | **Absolute** scanline; accepted range measured 29..242. `emu.go:1355` already records that "0起点" was the error and the fix landed on the Go comment only. `decompose_row`'s own Description says "absolute" while its own tag says 0-based | silent off-by-`visibleTop` |
| `breakif` | "beam reaches this color clock (**0-227**)" | Compared against `GetCoords().Clock` = **−68..159**; 160..227 is unreachable, so the call silently runs to `max_frames` and returns `halted=false` | half the domain is dead |
| `assert_line_budget` | "line_cycles (machine cycles it consumed)" | `lines * 76` (`emu.go:1683`) — a scanline delta, **always a multiple of 76**, never measured | invented arithmetic |
| `beam_intervals` | min/max "same coordinates as read_row: −68..159" | `clockAt` folds modulo 228, so a window past the line end comes back **inverted** (min > max) with `crosses_line=false` | wrong measurement |
| `beamtrace` | "trace `frames` frames and return, per scanline, every TIA write" | Traces N frames, then **keeps only the first** (`frs[0]`), while still advancing the emulator N frames irreversibly | pays N, returns 1 |
| `read_audio` | "control, freq, volume … verify sound numerically" | Also returns **`note0`/`note1` = {note, cents}**, the only register→pitch conversion in the whole tool surface, mentioned nowhere | hidden capability |
| `analyze_image` | "one screenshot is one frame of truth (flicker objects appear partially)" | Accepts **`paths[]`** and runs the multi-frame pipeline (static/dynamic split, union tracks, flicker) | the description **denies** the capability a flicker-hunter searches for |

Two were re-verified by hand before any were acted on, because a sweep that trusts its own agents is the
same failure one level up: `read_row`'s tag at `main.go:575`/`:607` and `assert_line_budget`'s
`lines * 76` at `emu.go:1683` were both read directly. Both hold.

**The sweep's own denominator was wrong — 38 of 41.** The shard list came from grepping `mcp.AddTool` in
`main.go`, and `save_state` / `restore_state` / `probe_ram_semantics` are registered from
`tools_state.go`, so three tools were never looked at while the write-up said "all 38". Swept by hand
afterwards: **clean**. `probe_ram_semantics` rejects anything outside `$80-$FF` with a named error rather
than probing it silently (`ramprobe.go:105`), its `top` really is by effect size (`sort.SliceStable` on
`MaxChanged`, `ramprobe.go:149`), and `restore_state` errors with the list of saved slots when the slot is
missing. Recording the miscount rather than quietly fixing the number: a coverage claim whose denominator
was never checked is the same defect this whole section is about — `timinglint`'s "0 false positives on all
31 kernels" was true of 30.

**`breakif` turned out to be worse than the finding said, and only measurement showed it.** The report was
"the advertised range 0-227 does not exist". Driving the built server found the real shape: the halt
condition was an *equality* on the beam clock, and observations only happen at instruction boundaries — the
CPU advances 3 colour clocks per cycle, so **one phase in three is observable at all**, and on a WSYNC
kernel a visible scanline is observed at **7 clocks, every one inside HBLANK**. The entire visible region
0..159 was unreachable. So the tool did not merely have a wrong range: for most positions a caller could
name, it ran to `max_frames` and returned `halted=false` — indistinguishable from "not yet". Fixed to halt
at or past the target, with out-of-range now an error.

**Status: 7 of 8 fixed** (`spritey`, `read_row`/`decompose_row`, `beamtrace`, `breakif`, `step_scanline`,
`assert_line_budget`, `read_audio`, `analyze_image`). **Open: `beam_intervals`** — `clockAt`
(`beaminterval.go:109`) folds modulo 228, so a window running past the end of the line returns
`min_clock > max_clock` while `crosses_line` stays false. That one is not a description fix: the honest
repair is to set `crosses_line` when the fold happens, which touches a prover whose soundness is graded
7117/7117, so it needs its own measurement first.

**The shape worth keeping.** Six of the eight are `misleads-into-wrong-measurement` and two are
`hides-an-existing-capability`, and the second kind cost real time this week: two separate attempts to
measure a horizontal trajectory were hand-rolled against the wrong tool because the right one did not say
it could do it. A description is part of the tool. An unstated capability is an absent one.

Fixes land one tool at a time, each verified against the shipped schema, not the source.

### "Hold the input, then read" does not measure a clamp — a second confident-constant trap (2026-07-30)

Found while settling a disputed measurement, minutes after wiring the liveness check, and it is the
same family: a number that is stable, plausible, and about something other than what was asked.

Measuring where Outlaw's gunman stops when you hold RIGHT, the obvious method is to hold it for a long
time and read the position. Held for 700 frames, `read_tia` returns `player0.hmoved_pixel = 7` — near
the LEFT edge. Nothing errors. The reason is that 700 frames is ~12 seconds: the round ended and the
gunman was returned to his start. **Waiting longer made the answer worse**, which is the opposite of
the intuition the method rests on.

The correct method is to sample and look for the plateau. Holding RIGHT and reading every 20 frames:

    17  27  37  47  57  57  57  59  57  57  59  57  59  57  57  59  57  57  59  57

10px per 20 frames (0.5px/frame, as documented), then a plateau at **57–59 oscillating ±1**. At that
moment the drawn extent is **clk 63–66** (`decompose_row`, y_top 101); the left clamp is `ObjectX` 4.

Two earlier readings of the same thing were 58 (right, but by luck — that run's round had not yet
ended) and 66–67 with a drawn extent of clk 69–80, which **does not reproduce** under this procedure;
the gap is ~6–9px in both observables, consistent with a different game variation or a different
sprite. Both are recorded in `sandbox/studies/outlaw/spec.ja.md` rather than one being chosen.

**The liveness check does not catch this one.** The program IS responding; the measurement is simply
of a later game state. Liveness answers "is it running", not "is it still in the situation you set
up". That is a second question, and it needs the same treatment: sample the trajectory, do not trust a
single late read.

### A stuck ROM returns a confident constant, and it fooled three measurements (2026-07-30)

Not a gap that was filed — a gap that bit, three times, in one day, and the third time it bit the
person writing the warning about it.

Outlaw sits in attract mode after power-on. Held input changes nothing, and every position accessor
keeps returning a **stable, plausible number**: `y_top` pinned at 101, `x` pinned at 7, for as many
frames as you care to run. Nothing errors. **That is the shape of the failure — a confident constant —
and it is the most dangerous shape a wrong measurement can take**, because it survives review: the
number looks like a measurement, has no error attached, and does not vary.

Three separate measurements of the gunman's movement were taken in this state today. The memory file
for that ROM already said, in bold, "ALWAYS verify liveness first". It was read, and the trap was
walked into anyway while acting on it.

`emu.RespondsToInput(player, action, frames)` answers it as a question about BEHAVIOUR, so it needs no
game-specific knowledge: run N frames with the input held, rewind to the same state, run N frames with
nothing held, and compare RAM. Identical means the program did not react.

**It is one-sided and says so.** `false` is strong — this input changed nothing. `true` is weak — an
animating title screen also changes RAM. It is a reason to REFUSE a measurement, never to bless one.

Witnessed on the ROM that caused the problem: before RESET it refuses ("changed no RAM byte at all"),
and the same test then proves the trap is real rather than asserted by showing `ObjectYExtent` handing
back an unchanged value across 30 frames of held input. After RESET it reports 3 changed bytes
(`$A0 $DC $DE`). A second test pins that the check leaves the machine exactly where it found it — a
helper that silently advances the emulator would be a trap of its own. Negative controls: removing the
refusal makes attract mode read as live; removing the final restore is caught as "the check advanced
the emulator it was asked about".

**★WIRED the same day.** `set_input` now runs the probe whenever a joystick direction or fire is
**held** — not on a release, a centre, a paddle or a panel switch, where the question is meaningless —
and returns `responds_to_input` plus the reason. Attached to the call that OPENS the trap rather than
offered as an option, because the caller who needs it is exactly the one who will not think to ask.
Measured cost ~0.3s per held input; side-effect free (the probe restores the machine, pinned by a test).

`cmd/harness` had **no test at all** before this; `TestSetInputReportsLiveness` is its first, and it
checks both directions — a held direction must carry a verdict, a release and a panel switch must not.
Negative control: removing the wiring reports "a held direction came back with no liveness verdict —
the trap is open again". The first version of that test was itself wrong: it pressed and released RESET
with no frames in between, so the switch never registered and it "proved" the game was still dead after
reset. Tenth instrument error of the day, caught by the test disagreeing with the emu-level one.

**Requires an MCP reconnect** to reach the running session: `bin/harness` rebuilt and smoke-tested
(`initialize OK 1.117.0`, 12 tools) per the standing rule of never asking for a reconnect without one.
`spritey` / `read_motion` still do not require liveness — only `set_input` reports it.

### The shadowed branches, decided: unit-witness the logic, keep the code, record the reachability gap (2026-07-30)

Two refusal branches were shown to be unreachable through any ROM this project can build, because a
coarser guard always fires first. The question left open was whether to delete them. **Decided: keep,
and witness them at the unit level instead**, because "no input reaches it" and "the logic is wrong"
are different claims and only the first was established.

`TestFoldLoopsRefusesABankSwitchInsideTheBody` builds the exact shape `foldLoops` looks for —
`lda $FFF9` / `dex` / `bne` back to the header, with a two-bank hotspot table — and asserts the refusal.
It checks its own premises first (the branch really targets the header; the planted access really
reaches a hotspot) so it cannot pass for the wrong reason, and it finishes with the SAME loop minus the
switching access, which must fold — otherwise the refusal would only prove that `foldLoops` rejects
everything. Negative control: removing the refusal makes it fall through to
"loop bound unknown (need a counted dex/dey…)", and the test names what was lost.

**What this does and does not settle.** It settles that the branch is correct when reached. It does not
make it reachable, and the test says so in place rather than reading as coverage. Deleting the branch
was rejected: the coarse guards that currently shadow it are about the REGION (multiple back-edges, no
WSYNC reached), not about bank switching, so a future change to region collection could expose this
path with nothing behind it.

The `determineBound` sibling (`dec: successor refusal`) is left as-is: it is shadowed the same way but
its guard is a plain `return 0`, so a unit witness would assert only that a function returns zero — no
information. Recorded as unverifiable rather than given a test that proves nothing.

### The witness sweep, finished: 4 soundness functions, 1 real bug, 12 unwitnessed branches (2026-07-30)

Last function: the blank-region classifier (`analyzeRegion`'s VSYNC/VBLANK test). It is the
**best-covered of the four**, and one of its branches turns out to be carrying real weight:

| branch | hits |
|---|---:|
| display off, but the region STORES to `$00`/`$01` -> **not** blank | **352** |
| blank | 331 |
| display on or unknown -> not blank | 249 |
| no abstract state at the region start | 0 |

The 352 is the finding. Without that second condition, **352 regions would have been skipped from the
visible-line budget check** on the strength of the display being off at entry, while the region itself
can turn the display back on inside itself. It fires more often than the blank verdict it guards. The
single zero is the missing-state fallback, which treats the region as visible and therefore
budget-checked — stricter, so a mistake there over-constrains.

**Sweep total across the four functions:**

| function | branches | unwitnessed | of those, refusals | real defect found |
|---|---:|---:|---:|---|
| `pagePenalty` | 6 | 1 (before the fix) | 0 | **yes — an under-approximation** |
| `switchEdges` | 10 | 4 | 4 | no (one repair witnessed for the first time) |
| `determineBound` | 12 | 4 | 4 | no (one proved unreachable) |
| blank classifier | 4 | 1 | 1 | no |

**One real bug, and it was in the only unwitnessed branch that was not a refusal.** That is the
sweep's lesson in one line: an unwitnessed CONSERVATIVE branch can only fail by staying silent, but an
unwitnessed PRECISE branch can be wrong in the forbidden direction and no outcome gate will see it.
Rank future sweeps that way — precise branches first, refusals second.

Twelve branches remain unwitnessed. Eleven are refusals; three of those were shown to be shadowed by a
coarser guard that fires first, so they are plausibly dead. Filed above, not guessed at.

### `foldLoops`, and a PATTERN: fine-grained refusals are shadowed by coarser ones (2026-07-30)

Fourth function through the witness method. Branch hits over 123 ROMs:

| branch | hits |
|---|---:|
| no back edge (not a loop) | 845 |
| WSYNC inside the loop body | 11 |
| multiple back edges (nested/complex) | 5 |
| branch inside the loop body | 1 |
| **bank switch inside the loop body** | **0** |
| **loop body leaves the latch's bank** | **0** |
| **loop body leaves the region** | **0** |
| **misaligned loop body** | **0** |

All four zeroes are refusals — they decline to fold, which leaves the region unbounded. Conservative,
so they cannot under-approximate by firing wrongly, only by failing to fire.

**Two construction attempts at the most valuable one, both blocked by a DIFFERENT earlier guard.**
`bank switch inside loop body` was added for bank support and says, correctly, that the second
iteration does not execute the same bytes. A fixture put `lda $FFF9` inside a `dex`/`bne` loop in
bank 0 of an F8 cartridge:

1. First attempt — the loop shared a region with the existing kernel loop, so `foldLoops` refused with
   **"multiple back-edges"** before it ever walked the body.
2. Second attempt — the loop was isolated between two fresh `sta WSYNC`s (and the kernel's row count
   reduced by two to keep the frame length). The region then came back **"no WSYNC reached from region
   start"**: the cross-bank edge sends the walk into bank 1, where it finds no WSYNC. Still 0.

**This is the same shape as the `determineBound` finding above**, where `dec: successor refusal` proved
unreachable because the region is refused at collection time. **★corrected 2026-07-30: TWO refusal
branches, not three** — the earlier count treated the two failed construction attempts at the same
`foldLoops` branch as separate branches. Two functions, two branches, each shadowed by a coarser
guard that fires first. Not a defect — the outcome is the same refusal — but
it means these branches are **unverifiable, and plausibly dead code**. Worth a deliberate pass later to
decide whether the fine-grained checks earn their place or should be deleted in favour of the coarse
ones they duplicate.

The fixture was deleted rather than kept. It exercised only branches that already had witnesses
(`multiple back-edges`, `no WSYNC reached`), and a witness that does not witness what its name claims
is worse than none: it reads as coverage.

### The sweep on `determineBound`: 4 branches unwitnessed, and one of them cannot be reached (2026-07-30)

Third function through the witness method. Branch hits over 123 ROMs:

| branch | hits |
|---|---:|
| `dec:` enter (dex/dey latch) | 51 |
| `div:` enter (sbc-divide latch) | 31 |
| `dec:` no dex/dey in body -> 0 | 21 |
| `div:` unknown predecessor -> discard inferred range | 19 |
| `div:` BOUNDED | 16 |
| `div:` unbounded -> 0 | 15 |
| **`div:` address-PROXY fallback** | **9** |
| `div:` `@amax` annotation used | 1 |
| **`dec:` successor refusal -> 0** | **0** |
| **`dec:` unknown predecessor -> 0** | **0** |
| **`div:` body not the canonical `sbc #const` -> 0** | **0** |
| **latch is neither BCS/BCC nor BNE/BPL -> 0** | **0** |

All four zeroes are REFUSALS (return 0 = stay unbounded), the conservative direction, so none of
them can under-approximate by firing wrongly — only by failing to fire.

**One of them is provably unreachable through the route it was written for.** A fixture was built to
witness `dec: successor refusal` — a banked cartridge with `lda ($90),y` (an unresolvable target on a
hotspot cartridge) immediately before a `dex`/`bne` loop header. Measured: the branch still ran 0
times, because the **region** is refused first, at collection time: *"region contains an access at
bank 0 $F04F whose target cannot be resolved, and this cartridge has bank-switch hotspots"*. The
coarser guard always fires before the finer one. That is not a defect — the outcome is the same
refusal — but it means the inner guard is **unverifiable and probably redundant**, and the fixture
was deleted rather than kept as a ROM that proves nothing.

**`dec: unknown predecessor -> 0` is the one worth flagging.** It is the guard SD-11a added after
finding that the predecessor scan silently skipped predecessors it could not bound, and like the
`jmp`/`jsr` case above it **has never once run**. Unlike the successor-refusal branch, no attempt has
yet shown it unreachable — it is simply unwitnessed. Filed.

**`div:` address-PROXY fallback fires 9 times.** That is the heuristic SD-9 removed from the dex/dey
path for producing a fortyfold under-approximation, still live on the divide path — gated to one bank
and counted, not trusted. It has witnesses, so it is not in the class above, but it is the remaining
place where an address stands in for execution order.

### The same sweep on `switchEdges`: 4 refusal branches had no witness, one of them a repaired one (2026-07-30)

Applying the witness method to the next soundness function. Counting which branch of
`switchEdges` each instruction takes, over 123 ROMs and all four entry points:

| branch | hits |
|---|---:|
| not a banked cartridge | 1,912,675 |
| instruction touches no memory | 88,981 |
| access reaches no hotspot | 46,852 |
| edges produced | 6,755 |
| the instruction's own bytes span a hotspot | 108 |
| access target unresolvable | 13 |
| **`jmp`/`jsr` transfers control INTO a hotspot** | **0** |
| **hotspot symbol does not name a bank** | **0** |
| **target bank outside the analysed set** | **0** |
| **landing address wraps past `$FFFF`** | **0** |

All four unexercised branches are REFUSALS, and that is precisely why an outcome gate cannot see
them: a branch that never fires produces no wrong number, it produces a **missing refusal**.

The first of them matters most because **it is a branch SD-8b judged unsound and repaired.** Gopher2600
classifies `jmp`/`jsr` as Subroutine/Flow rather than Read, so a check driven off an instruction's
data access never looks at them and `jmp $FFF9` slipped past. The fix has been in the tree since —
and had **never once run**. `roms/litmus/litmus_bank_jmphotspot.asm` plants `jmp $FFF9` in a visible
kernel region of bank 0 of an F8 cartridge; the branch now fires 5 times and the region is refused
by name: *"the jmp at bank 0 $F04D transfers control to BANK1 ($1FF9), whose instruction fetch
selects another bank"*. `TestJmpIntoHotspotIsRefused` requires the refusal to mention the operator,
the symbol, the address and the reason, so it cannot degrade into a generic unbounded region.
Negative control: removing the `jmp`/`jsr` case — i.e. the state before SD-8b — makes the refusal
disappear entirely. All 124 other ROMs byte-identical.

The remaining three (symbol not naming a bank, bank outside the analysed set, landing wrapping past
`$FFFF`) need cartridges this corpus does not contain — Parker Bros `B0S0` symbols, M-Network `RAM0`
— and are **recorded as unwitnessed rather than assumed correct**.

### Why the page-cross bug survived a passing gate: the branch had NO witness (2026-07-30)

`TestProvenWorstIsNeverExceededOnCorpus` compares proven against measured on **228 regions
across 31 ROMs** and passed the entire time the constant-index under-approximation was live. It
had to: **no corpus ROM took the wrong branch.** An outcome gate cannot see a branch nothing
reaches, and that is the general lesson, not a detail of this bug.

Measured by counting which branch of `pagePenalty` each instruction takes, over 123 ROMs:

| branch | hits | with `litmus_pagecross` removed |
|---|---:|---:|
| `not-sensitive-or-branch` | 23423 | 23423 |
| `index-unknown` (-> +1, conservative) | 111 | 111 |
| `indirect-or-other` (-> +1, conservative) | 51 | 51 |
| `index-known-no-cross` (-> 0, **precise**) | 5 | **1** |
| `index-known-CROSSES` (-> +1, **precise**) | **4** | **0** |
| `state-unknown` (-> +1, conservative) | 0 | 0 |

The bug lived in a branch with **zero** coverage, and the four hits it now has are the four reads
of the litmus written to catch it. The two PRECISE branches are the ones worth guarding — every
other branch returns the conservative `+1`, where a mistake over-approximates and is allowed.

`TestEveryPagePenaltyBranchHasAWitness` classifies every instruction in the corpus by branch and
**fails when either precise branch has no witness**. It also re-derives what `pagePenalty` should
return from the same inputs and compares, so the classification cannot drift from the function it
describes. Negative controls: removing `litmus_pagecross` fails with the branch named; restoring
the old wrong condition reports 4 instructions whose classification no longer matches the
function.

`state-unknown` is still at 0 — never reached by any corpus ROM — but it returns the conservative
`+1`, so a mistake there over-approximates. Recorded rather than guarded.

### The neighbourhood of that bug, swept: 1 defect, 3 paths measured correct (2026-07-30)

After fixing `pagePenalty` the same class of error was looked for around it, on the principle
that one under-approximation in a costing function is a reason to distrust its siblings. **Nothing
further was found**, and the three checks are recorded because a measured zero is a result:

1. **`PageSensitive` is right across all 256 opcodes.** 32 indexed opcodes carry it and every one
   is a READ. **No WRITE carries it** — `sta abs,X` is 5 cycles crossing or not, so marking it
   would over-charge. Every indexed opcode that does NOT carry it is explained: zero-page indexed
   (cannot leave page 0), a write, or read-modify-write (which always pays the extra cycle, so
   there is no conditional penalty to model). The first sweep flagged 52 "under-charged" opcodes
   and every one of them was one of those three — the instrument, again, before the finding.
2. **Branches are charged by the correct rule.** The CFG edge uses
   `(in.next()>>8) != (in.branchTarget()>>8)` — target page against the page of the instruction
   AFTER the branch, which is the hardware's own test — and takes the max of taken and not-taken.
   `pagePenalty` excludes branches so the cycle is not charged twice.
3. **The unknown paths stay conservative.** `(ind),Y`, an unknown index range, and a missing
   abstract state all return `+1`, which over-approximates. Sound.

`TestPageSensitiveTableIsWhatTheCostingAssumes` pins all of it, including the count of 32 — these
are premises about the ENGINE's opcode table, so nothing else in this repo would notice them
changing. Negative control: removing the branch exclusion from `pagePenalty` reports all 8
branches as double-charged.

### RESOLVED: the page-cross penalty was never charged for a CONSTANT index — an under-approximation (2026-07-30)

Raised while measuring the last of known-traps.md's named static traps, "bank-move misaligns
code -> page-cross". The question turned out to be about the prover rather than about a linter,
and it is **recorded unresolved** rather than guessed at.

`absStates` is commented "S3: abstract state per site, for page-cross **precision**", and the
audit says page-cross `+1` is handled by the abstract interpreter. Measured, it does not appear
to vary with the address:

A fixture kernel reads `lda Table,y` four times in one WSYNC region with `ldy #200` — a constant,
so the target address is decidable. Moving `Table`:

| `Table` at | `$F0xx + 200` | crosses a page? | `max_worst` |
|---|---|---|---|
| `$F100` | `$F1C8` | no | **35** |
| `$F0F8` | `$F1C0` | yes | **35** |
| `$F130` | `$F1F8` | no | **35** |
| `$F138` | `$F200` | yes | **35** |

Four reads, so a precise model should differ by 4 cycles between the crossing and non-crossing
layouts. Hand-counting the region gives ~30 without the penalty and ~34 with it, and the reported
35 sits at the charged end — **consistent with the penalty being applied unconditionally, which is
SOUND (an over-approximation) but not what "precision" claims**. The alternative reading, that the
penalty is never charged, would be an under-approximation and the direction this package forbids,
so the difference matters and the two are not distinguishable from the number alone.

**★RESOLVED by reading `absint.pagePenalty`.** It asked the wrong question:

```go
if (base+idx.Lo)>>8 != (base+idx.Hi)>>8 { return 1 }   // does the RANGE straddle a page?
return 0
```

The 6502 charges its extra cycle when the indexed **target** lands in a different page from the
**base** — not when the range of possible targets straddles a boundary. With a CONSTANT index
`Lo == Hi`, so the old test always compared equal and returned **0**, however far the access
reached. It was also wrong for any range lying wholly beyond the boundary. Both are
**under-approximations**, the one direction this package forbids, and the same shape as SD-9.

Fixed to `(base>>8) != ((base+idx.Hi)>>8)` — crossing is monotonic in the index for a fixed base,
so the worst case over `[Lo, Hi]` is decided by `Hi` alone. Measured on the same ROM,
`litmus_pagecross`: **FarRow 35 → 39** (four crossing reads, +1 each) with **NearRow unchanged at
30**. **123 of 123 existing ROMs byte-identical** — the bug was real and had no witness in the
corpus, which is exactly why `roms/litmus/litmus_pagecross.asm` is now permanent: it is the only
thing that can catch it coming back. `TestConstantIndexPageCrossIsCharged` pins both numbers, and
its comment warns against comparing NearRow with FarRow directly (same instructions, different
addresses; they differed by 5 even before the fix, for unrelated reasons). Negative control:
restoring the old condition reports `FarRow worst = 35, want 39`.

**Separately measured and clean:** shifting each of the 31 technique kernels by 1, 2 and 3 bytes
(`ds N` after the first `ORG`, verified to change the assembled image — three distinct SHA-1s at
the same 4096 bytes) leaves `max_worst` **unchanged on all 31**. So no corpus kernel is alignment-
fragile today, whichever way the question above resolves.

### 38 of the 95 scenarios were never run by any gate (2026-07-30)

Found by widening the orphan question from "is this ROM referenced by anything" to "is this
CHECK actually executed". Measured: **95 scenario files in three directories** — 57 under
`roms/litmus`, 31 under `roms/techniques`, 7 under `roms/exerciser`. The CI mirror ran
`cmd/scenario roms/litmus/scenarios/*.json` and nothing else, so **38 of 95 (40%) were written
and then never executed**.

All 38 pass today (`techniques` 31/31, `exerciser` 7/7), so nothing was hiding behind them.
That is the whole point: a check nobody runs reports nothing when it breaks, and these had been
sitting outside the gate for long enough that their state was unknown until it was measured.

`internal/scenario.TestEveryScenarioRuns` walks `roms/**/scenarios/*.json` and runs every one
inside `go test ./...` — **discovering** the directories rather than naming them, so a fourth
scenario directory is covered without anyone remembering to extend a command line. It reports the
per-directory denominator and fails if the walk finds only one directory, since a sweep that
silently covered one place is the state it replaces. Runtime 19s (litmus 3.3, the rest 15.5).
Negative control: breaking one `techniques` scenario — previously invisible to CI — fails it.

**The ROM corpora themselves are clean:** all 31 `roms/techniques` ROMs are referenced (18 places
glob the directory) and after the litmus gate, 0 of 92 litmus ROMs are orphaned. The gap was never
the ROMs; it was which checks the runner was pointed at.

### The timing linter had ZERO coverage of every bank-switched cartridge (2026-07-30)

Found by re-measuring AT-1's "0 false positives on all 31 technique kernels". The claim
survives, but the denominator had quietly become 30: `banked_game.asm` is answered with a
`not-analysed` refusal, and **0 of its 133 instructions were ever read**. The refusal is
honest, and it was also the whole story for any 8K+ ROM — an authoring aid with no coverage
at all of the cartridge size a real game reaches.

`cyclebound.Prove` had already gained the per-bank pipeline (SD-8/SD-11). `Lint` had not, so
two tools disagreed about the same ROM. Lint now runs the SAME path —
`analysisUnits` -> `decodeUnits` -> `switchModel` -> `computeStates` — which is why all 113
flat images produce byte-identical output: a flat ROM is the one-unit case.

Two rules needed care, in opposite directions:

- **R1/R2 must survey the UNION of banks.** Both ask "is HMxx/HMOVE used ANYWHERE", and the
  answer can live in another bank. `lint_bank_split` is the fixture: HMP0 staged in bank 0,
  HMOVE strobed in bank 1 — correct code. Measured with the survey artificially restricted:
  bank 0 alone reports `hmxx-without-hmove`, bank 1 alone reports `hmove-without-hmxx`. Both
  false. The merged survey is silent.
- **R3 must NOT cross a bank.** The straight-line walk follows fall-through addresses; after
  a hotspot access the next fetch comes from the other bank, so it now stops at any
  instruction `switchEdges` says can switch.

| ROM | before | after |
|---|---|---|
| `banked_game` (corpus) | not-analysed, **0 instructions** | silent, **134** (bank0 67 / bank1 67) |
| `lint_bank_hazard` (trap in BANK 1) | not-analysed, 0 instructions | **`hmove-hazard` at bank 1 $F00D**, 149 read |
| `lint_bank_split` (correct, split) | not-analysed, 0 instructions | silent, 138 read |

`timinglint` now prints its own denominator on every run ("read N instructions across B
bank(s): bank 0: x, bank 1: y") because "no timing warnings" over a program that was never
decoded is indistinguishable from a clean bill of health — which is precisely what it used
to print. `LintResult` carries the counts. Negative control: restoring the old decline fails
both new tests by name.

**A second defect fell out of the fixture.** The first bank-0 warning printed
`bank 0 LvTab+10` — `LvTab` is a BANK 1 label. srcmap's label list comes from the symbol
dump, where every bank's labels carry their RORG'd $F0xx address, so the two banks are
interleaved in one list and "the last label at or before this address" can be either bank's.
The existing `solver.loc` comment reasoned that bank 0 is safe because DASM's listing rows
for it are dropped; that is true of LINE NUMBERS and false of LABELS. Lint now prints
`bank N $FFxx` for every bank on a banked image. **The prover had the same hole and it fired
on a corpus ROM**: `cyclebound -asm roms/techniques/banked_game.asm` labelled two bank-0
regions `LvTab+0` / `LvTab+2`. **FIXED (next commit):** `solver.loc` now prints
`bank N $FFxx` for every bank on a banked image. Measured on banked_game, 8 locations, 0
carrying a label; across the four bank ROMs, 104 locations checked. All 113 flat images
byte-identical. `TestBankedReportNamesNoSourceLabel` walks the marshalled report for any
key ending in `loc`, so a location field added later is covered without anyone remembering.
Negative control: the old `loc` fails it, naming `bank 0 LvTab+2`. **★BUILT 2026-07-30 — `srcmap.BankMap`.** The listing's address column is the
physical ROM offset, so `bank = offset>>12` and `addr = $F000 | (offset & $0FFF)` recover per-bank source
lines exactly. **Labels do not come from the symbol table at all** — its addresses are RORG'd and interleave
the banks — they come from the SOURCE FILE: a label defined on line L appears in the listing on line L, and
that row carries the offset, which carries the bank. Measured: `banked_game` resolves **87 addresses across
2 banks** (bank 0: 64, bank 1: 23) with 12 labels placed; `litmus_bank_f4` resolves 102 across 8 banks.
`bank 0 $F017` is now `bank 0 NextFrame+4 (banked_game.asm:44)`, and `bank 1 $F000` is
`bank 1 B1Work (banked_game.asm:110)` — a label the flat map could never have reached. A label further than
one page from the address is dropped and the line printed alone (measured: `$FF86` sits in a trampoline with
no label of its own and was reported as `LvTab+3949`).
**`@lines` / `@amax` now work on a banked image too, and they were broken for EVERY bank, not just 1..n:**
bank 0's listing rows are below `$1000` and the flat parse discards them as TIA equates, so an address lookup
missed in both directions and every banked region was silently budgeted at one scanline. No banked ROM in the
corpus carries an annotation, so nothing in the reports changed — hence
`TestBankedImageReadsLineAnnotations`, which writes `@lines 3` onto a copy and measures the KRow region's
budget going **76 → 228**. `TestBankedReportNamesItsOwnBanksLabels` replaces the interim "never print a
label" rule with "never print another bank's label", checked against `BankMap.LabelBank` over 104 locations
(44 of them now named). All 114 flat images byte-identical. Negative control: restoring the flat lookup
reproduces `bank 0 LvTab+2`.

### Three more from the audit re-measurement: an off-by-one in a litmus, a stale figure, a false sentence (2026-07-30)

**1. `litmus_shift_base` and `litmus_shift_down8` ran 261 scanlines, not 262.** Both files' headers state
`NTSC 3/37/192/30 = 262`. Counted from the source, the visible run is `40 + 24 + 127 = 191`, so the frame is
**261** — confirmed independently by `cmd/scenario`'s `ntsc_frame_lines` on both ROMs. Fixed at the source
(`ldx #(128-SHIFT)`), both now 262, and the ±8 shift detection they exist for still passes in all three
directions.

**Why it survived:** these two were the only litmus ROMs with no scenario file. `lastline`, `nusiz_all` and
`objsizes` all carry `ntsc_frame_lines` checks; these did not, so nothing ever asked. Both now have one.
That is the same shape as RL-7d — a ROM outside the regression net — found in the same sweep.

**2. RL-7b's "Kern's worst drops 97 → 66" is stale.** Re-measured: the same region is **74** of 76 today and
still certifies. 66 was true when written; RL-8a then added the missile and ball enables. The entry is
annotated in place rather than rewritten, because the drift is the point.

**3. framegen could print a difference that does not exist.** `diagnose` ends with *"Every element is present
and every object cell matches; the difference is in BG cells only"* on the branch where nothing differs and
nothing is over-drawn — i.e. where the clone matches. Measured: **no ROM in the 31-ROM corpus reaches it**, so
it is a sentence with no witness rather than a visible defect. Corrected anyway, because the first target that
does reach it would be told a difference exists and where it is not.

### RL-7c regressed from 2 cells to 1868 and nothing noticed (2026-07-29)

Found by re-measuring the audit's own numbers rather than by anything failing. RL-7c records
"2666 → 2 cells" on `litmus_nusiz_all` and builds a conclusion on it — that per-line NUSIZ replay works.
That was true when written. **RL-8a then broke it to 1868 and the sentence stayed.**

| framegen version | mismatched cells |
|---|---|
| pre-RL-7c | 2666 |
| RL-7c | **2** |
| RL-8a → today | **1868** |

**Cause, measured:** RL-8a added `clampInput` to stop the position calibration walking out of the immediate
operand's range. But the div-15 routine positions modulo the 160-clock line, so an input below the floor has
an equivalent 160 higher — and clamping to 0 does not lose precision, it STALLS the calibration at the wrong
position. P1's target reset X of 4 needs input −3; the clamp pinned it at 0 and calibration stopped at
`P1 7(want 4,d-3)`. The clone then reported 1074 of P1's own cells as "a cause this tool has not measured".
Fixed by wrapping instead of clamping: **1868 → 2**, P1 reaching X 4 on the first iteration with input 157.

**Why it was invisible, which is the more important half.** The corpus regression sweep globs
`roms/techniques/*.asm`. `litmus_nusiz_all` lives in `roms/litmus/`. **The ROM added because "it lights an
axis nothing else does" was outside the regression set** — and `cmd/framegen` had no test file at all, so CI
never ran it either. A number in prose, a conclusion on top, and nothing asserting it: the same shape as the
`38/43` arity figure, in the same document, found the same way.

**Fixed structurally**, not just corrected: `cmd/framegen/regress_test.go` is framegen's first test and pins
the cell count and frame length for the ROMs that exercise the axes nothing else covers —
`litmus_nusiz_all` (2), `litmus_objsizes` (2568, partial), `zone_multiplex` (0), `shared_setxpos` (16).
Negative control: restoring the clamp makes it fail.

Two things worth keeping from writing that test. Its first version expected `litmus_objsizes` to be
pixel-exact **from assumption**, and the test caught it — measured, M0 and M1 match exactly (728/728,
720/720) while the ball is not reproduced and the missiles are drawn on more lines than the target draws them
(clone 1544 vs target 728). And the counts are pinned at their MEASURED values, not at hoped-for ones, so an
improvement shows up as a failing test to be updated rather than passing silently.

### The verification canon is outgrowing the attention it gets — trigger-bound, not scheduled (2026-07-29)

The single source of truth for verification discipline is memory `feedback-verification-standard`. Its
delivery is sound: `harness/CLAUDE.md` names it in iron rule 1 and is loaded in full every session,
`MEMORY.md` lists it first among the four behavioural standards, and 17 other memory files link to it. It
gets read.

**Its size is the problem.** Measured today: **26.3 KB, 147 lines, 9 sections — and 4 of those sections,
4321 characters or 35% of the file, were added in this one session.**

**The honest part is what that growth means.** Those four sections are "distrust a refusal", "check the
instrument twice", "an observable artefact can be correct while its surroundings are broken", and "re-measure
a number before reporting it". **Every one was written AFTER being caught by it today, not before.** The file
did not prevent them. So the growth is not evidence the canon is working — it is evidence that a 26 KB
document is not what stops these mistakes. The tool output that carries its own denominator does
(§"A RAM gate that compares one byte", §"The technique corpus is playfield-light").

**Why this is filed here and not scheduled.** Reorganising the canon has the exact property this repo keeps
finding fault with: **its effect cannot be measured.** A shorter file that prevents nothing is worse than a
long one, and "the mistakes stopped" would be indistinguishable from "no such mistake came up". A scheduled
tidy-up would also land in the 31%-completion bucket that undated TODOs go to — measured today, this audit's
own items run at 43 of 53 done (81%) against 5 of 16 (31%) for the STATUS board's TODO list.

**The trigger, so this is not "someday":** act on it the next time a mistake occurs that the canon ALREADY
warned about. That is the missing evidence — today's four sections are all cases the canon did *not* cover, so
they say nothing about whether a reader would have found the warning. One case of "it was written down and I
still walked into it" turns this from a tidiness preference into a measured retrieval failure, and *that* is
worth restructuring for.

Related, and the same shape one level up: the durable fix for any of these is to move the check into a tool's
output, where it cannot be forgotten. A ritual or a memory file is the net for what has not been moved yet —
it should shrink as the tools absorb it, and a growing canon is a signal that absorption is lagging.

### A frame counter that WRAPS was not recognised as a clock — and it had been carrying 12 "resolved" bytes (2026-07-29)

`ramtrace arity` finds the smallest feature set (self + input + companions) that determines each RAM byte's
next value. `frameIndexLike` exists because a free-running counter uniquely identifies the frame, so keying on
it "explains" every other byte perfectly and the probe reports arity 1 for all of RAM — memorisation wearing a
model's clothes. That trap was found and fixed in a previous session.

**It was only half fixed.** The test required the value to be distinct on EVERY transition, and a counter that
WRAPS does not satisfy it. Measured on Outlaw, `$DA` takes **256 distinct values, changes on all 4266
transitions, and its only deltas are +1 and −255** — a textbook frame counter that cycles. It failed the clock
test purely because the recording outlives its period, and **27 of the 35 bytes reported as resolved were
explained by keying on it**.

| Outlaw, 24 scenarios, warmup 20 | before | after |
|---|---|---|
| live bytes | 44 | 44 |
| **resolved** | **35** | **23** |
| **unresolved** | **9** | **21** |
| named as clock-like | — | **`$DA`** |

**This corrects a project-level conclusion.** The previous session recorded that the behaviour-reproduction
plan's central premise — "each byte is determined by itself + input + at most 2 companions" — was *supported by
measurement*, on figures of that shape (it recorded live 43 / resolved 38 / unresolved 5). With the clock
excluded, **nearly half the live bytes are unresolved**. The premise is far weaker than recorded, and the M-B
phase should be planned against 23/44 rather than 38/43.

The detector now also treats "visits ≥250 of its 256 possible values while changing every frame" as a clock,
and the report **names** the clock-like bytes instead of computing the list and never printing it — the
`FrameIndexLike` field existed and no caller displayed it. Asserted in three directions: a wrapping counter is
a clock, a byte cycling over four values is NOT (an ordinary state variable must stay usable as a companion),
and a constant is not.

### A RAM gate that compares one byte read as a pass (2026-07-29)

`behavmatch -ram-gate` reports the first frame and address where a build's RAM stops matching the target's,
and it was built to print what it excluded — so the exclusion bookkeeping was the thing to check first.
Measured over the first six built-in scenarios on six ROMs: **not one excluded byte actually varied in the
traces.** The mask drops nothing that mattered; that part is sound.

What the same numbers exposed is how THIN the comparison can be:

| ROM | bytes compared, of 128 |
|---|---|
| `paddle_demo` | **0** |
| `game_states` | **1** |
| `bullets` | **1** |
| `litmus_bank` | 2 |
| Outlaw | 15 |
| Combat | 45 |

`VACUOUS` fires only at zero, so a gate comparing ONE byte printed `first_divergence: none` and read as a
pass. And the mask's thickness is a property of the **scenarios**, not of the ROM — `game_states` needs
specific input to move its state, so six generic scenarios leave 127 of its bytes constant and therefore
excluded as "dead or unexercised".

Now warned, with the number stated both ways:

```
  RAM gate [p0-right]: 1/128 bytes compared over 150 frames
    THIN: only 1 of 128 bytes varied in the target's own traces, so this verdict covers almost none
    of its state — the scenarios did not exercise it, and a matching build would look identical here
    whatever else it got wrong
```

Asserted in both directions, because a warning that fires on everything is noise: the 1-byte gate must warn,
and Combat's 45-byte gate must not.

**The deeper statement, recorded rather than fixed:** the RAM gate's power is bounded by scenario coverage,
and on the current ROM-agnostic library that bound is severe for anything that needs specific input to change
state. A game carrying its own scenarios (`-scenarios file.json`, already supported) is the intended answer;
until one does, `-ram-gate` on a technique ROM is close to decorative and now says so.

### The technique corpus is playfield-light, and the PF tools are graded on 7 ROMs (2026-07-29)

`MeasurePF` decides whether a target's playfield is repeat, reflect or asymmetric, and the caller emits
tables from that decision — dropping the right half when it says repeat or reflect. A wrong decision there
silently loses half the picture, so it deserves a check against the pixels rather than against the reasoning
that produced it. (`TestGeneratedPFMatchesTIARegisters` checks the generated BYTES, but only on ROMs this same
function classified as repeat or reflect: it grades the decision using the decision.)

New `TestPFModeDecisionAgreesWithThePixels` reads the attribution grid directly: if the mode says repeat, the
right half of every covered line must equal the left. **Result: 0 disagreements over 9515 lit column
samples.** The decision is sound on what the corpus can test.

**What the corpus can test is 7 of 31 ROMs, and the first version of this check hid that.** It reported
"31 of 31 sound" — while **24 of those ROMs draw no playfield at all**, where "left half off equals right half
off" is true by default. The pass covered nothing on three quarters of its inputs. The test now counts and
prints the vacuous ones, and fails outright if none of the remaining ROMs has a playfield.

The seven with playfield content: `hscroll` (2568 lit samples), `maze` (2240, the only reflect ROM),
`divtable` (1600), `rpgmap` (1059), `banked_game` (768), `rts_dispatch` (640), `pf_modes` (640).

A guess checked and dropped: `venetian.asm` looked like a contradiction — a "venetian blind" ROM with no
playfield — but it writes no PF register at all and draws with GRP0 and COLUBK. The blinds are a sprite
technique. Assumption, not defect.

**Corpus gap, stated rather than closed:** the technique corpus is sprite-heavy and playfield-light, so every
PF-facing tool (`MeasurePF`, `-genpf`, framegen's PF pruning) is graded on seven ROMs, one of which is the
only `reflect` witness. This is a different gap from the TIA-state axes below — `statecov` reports
`pf_reflect`/`pf_score`/`pf_priority` at 2/2, but that measures MODES EXERCISED, not ROMs with playfield
CONTENT to reproduce. A ROM with a dense asymmetric full-width playfield would be the highest-value addition.

### Corpus selection is measured, not chosen by taste (2026-07-29)

The question "would more test ROMs make the tests more useful?" has a measurable answer, and `statecov`
gives it: aggregate the TIA state axes every corpus ROM exercises and read off what nothing has ever run.

Across the 31-ROM technique corpus:

| axis | covered | never exercised |
|---|---|---|
| `nusiz1_copies` | **2/8** | 1, 2, 4, 5, 6, 7 — P1 was almost never given copies or a size |
| `nusiz0_copies` | 4/8 | 1, 2, 4, 7 |
| `missile0_size` / `missile1_size` | 2/4 | the 2px and 4px widths |
| `ball_size` | 2/4 | the 4px and 8px widths |
| `vdelbl` | **1/2** | never set at all |
| `pf_reflect` / `pf_score` / `pf_priority` / `vdelp0` / `vdelp1` | 2/2 | — |

Playfield modes and player vertical delay were already saturated; missiles, the ball and P1's copy modes
were not. Two litmus ROMs close all of it — `litmus_objsizes.asm` (every missile and ball width, plus VDELBL
made observable by toggling ENABL on alternating lines) and `litmus_nusiz_all.asm` (all eight NUSIZ modes on
both players) — taking every bounded axis to full coverage.

**The coverage number is not the point; what the new ROMs broke is.** Run through `framegen`,
`litmus_nusiz_all` produced **2666 mismatched cells, the worst in the corpus** (previous worst:
`zone_multiplex`, 380) — and, worse, the tool explained it wrongly: *"one X per player cannot follow a
per-zone multiplexed target"*, on a ROM whose players never move. The real cause is that `nusizWidth`
understands only modes 5 and 7 (double/quad width) and treats the five COPY modes as a single 1× player.
A confidently wrong reason is worse than no reason; fixed separately.

The exercise also found the assertion language could not state what the new ROM exists to prove: `tiareg`
exposed no `missile0`/`missile1` object at all and no `size` field, so a scenario could say where a missile
was but not how wide. Coverage that cannot be asserted is coverage on paper. Added
`tiareg.missile0/1.{color,nusiz,size,copies,enabled,reset_to_player}` and `tiareg.ball.{size,vertical_delay}`.

**Rule going forward:** admit a ROM to the corpus when `statecov` shows it lights an axis nothing else does,
not because it is famous or complex. The corollary holds for commercial ROMs, whose value is different in
kind — they contain shapes we would not think to write (Fishing Derby is what exposed the missile gap in
RL-7) — but the same measurement decides which ones earn their place in CI.
