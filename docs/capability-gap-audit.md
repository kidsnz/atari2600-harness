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
- **G5:** mid-line HMOVE / RESP pipeline = *observed* via `trace_clocks`, not litmus-locked.
- **G6:** oracle sub-frame phase offset for per-frame-mutating RAM.
- **G7:** collision trap (`watch_ram` is RAM-only); `step_clock` (parked).
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
| **VV-14** ◑v1.96.0 | **`cmd/cpucert`** ✅ · `@lines` real kernels ✅ · **interprocedural JSR/RTS + divide-loop bounding** ✅ (2A/2B) · ILP/SMT(Z3) + external TIA/Sim2600 ROMs (defer) | citable cert + prover scope expansion done; no false-positive violations remain (14/31 certified); rest are honest UNBOUNDED scope limits | B-C2/C-2/B-C3 | M–L | 3 (partial) |

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
  why this prover exists:** a *small* per-scanline overrun (one heavy line = 262→263 scanlines) is **visually
  invisible** — the TV's auto-sync absorbs a one-line slip, so `cb_roll` (over) and `cb_clean` (clean) look
  pixel-identical (verified 2026-06-17). Visual checking is unfit for this class of defect; only the numbers
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
  opcodes agree across many seeds **except 11 illegal/unstable opcodes** (ANC/ALR/ARR/ANE/LXA/SH*/LAS), which form
  a classified allow-list. Main build stays **CGO-free** (perfect6502 is an external binary, shelled out;
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
  technique kernels** (first sweep's 6 hits were two detector gaps — missed indexed `sta HMP0,x` stores, and a
  benign zero-clear — both fixed; a latent hazard false-negative in the cycle accounting also fixed). Litmus
  `lint_r1/_r2/_r3` + `lint_clean`; `TestLint*` lock both directions + the corpus guard. Pure Go, CLI only (no
  MCP/reconnect). **AT-2/3/4 + a single batched MCP exposure remain.**

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

## Housekeeping backlog (docs/repo, not a harness capability)
- **DOC-EN — translate the JA-heavy canonical docs to English** to finish the public-repo English-only pass
  (`design-principles.md` ~98 JA lines, `casebook.md` ~39, `build-to-learn.md` ~33). Deferred from the
  2026-06-17 docs cleanup (which dropped the 13 `.ja.md` duplicate files); these `.md` bodies are the *only*
  copy so they were left intact, but the user asked that the English-ization "reliably happen later" — tracked
  here so it is not dropped. `mining-digest.md` is **excluded** (generated from Japanese-source thread data —
  translating would break source fidelity). Size: medium; needs review. Separate approval.
