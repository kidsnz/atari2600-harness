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
| **VV-2** | **Static per-scanline cycle-budget PROVER** (`cmd/cyclebound`) | **proof over ALL paths** (∀) vs observe-one-run — the only ∀-claim member; attacks gap B | C-1 | M | ★1 |
| **VV-3** | **PC/branch coverage map** (`cmd/cover`) → **coverage-guided fuzzing** | test-adequacy axis + AFL-style feedback fuzz (today's fuzz is blind) | D-1→D-2 | S→M | ★1 |
| **VV-4** ✅v1.79.0 | **Motion-smoothness / jerk metric** (`cmd/motion` + `read_motion` MCP + `checks.motion`) | per-frame motion-jerk NUMBER = "judder/ブルブル" automated (closes the Breakout hand-trace) | E-1 | S–M | ★1 |
| **VV-5** | **Temporal-logic trace assertions** (`temporal` block: always/eventually-within-K/response/never-for-N) | properties over a **sequence** of frames (per-frame invariants can't) | F-1 | M | ★1 |
| **VV-6** | **MAME headless cross-oracle** (`cmd/mamecheck`) | a **3rd independent** full-system oracle, **fully headless** (no keypress unlike Stella) | A1 | M | 2 |
| **VV-7** | **perfect6502 hardware-grade CPU oracle** (`cmd/cpucheck`) + **N-oracle majority vote** (`cmd/oraclevote`) | silicon-netlist truth (catches bugs ALL hand-written emulators share); fuses oracles into one verdict | A2/A3/B-C3 | M | 2 |
| **VV-8** | **Behavioral trajectory diff vs original ROM** (`cmd/trajdiff`) | full **time-extended** state-trajectory diff (refdiff is a static snapshot) | F-2 | M | 2 |
| **VV-9** | **Score/lives OCR semantic oracle** (displayed digits == RAM) | ties **display ↔ program meaning** (template-match, pure-Go, no Python) | E-2 | M | 2 |
| **VV-10** | **HW-divergence trap detectors** (timer-wrap=G8, HMOVE-latch, uninit-RAM-read) | runtime monitors for "passes-in-emu / fails-on-HW" (siblings of `assert_line_budget`) | F-3 | M | 2 |
| **VV-11** | **State-coverage matrix** (zone/VDEL-parity/NUSIZ/bank) + **coverage-aware mutation** | did tests exercise every TIA mode; honest mutation kill-rate (closes the playbook's 5–20% thread) | D-3/D-4 | S–M | 3 |
| **VV-12** | **SSIM / pHash tolerant frame compare** | magnitude+locality "how wrong, and where" (exact golden is boolean) | E-3 | S–M | 3 |
| **VV-13** | **Audio spectral (FFT) + RMS-envelope diff** | frequency-domain timbre check (out-resolves `golden_audio` on V2-14 inverted twins) | E-4 | S–M | 3 |
| **VV-14** | `cmd/cpucert` owned certificate · **ILP/SMT (Z3)** prover upgrade · external TIA/Sim2600 ROMs | citable cert; infeasible-path tightening + value-range proofs; silicon-TIA tie-breaker | B-C2/C-2/B-C3 | M–L | 3 (defer) |

## Tier ★1 — recommended pilots (highest value × feasibility, substrate mostly in-tree)
- **VV-1 ✅ DONE (v1.78.0):** the suites are run via the full import path `go test github.com/jetsetilly/gopher2600/hardware/cpu/tests/{klaus2m5,thomharte}/...` (resolved through go.mod's `replace`; `cd Gopher2600` is wrong — go binds the harness module). **Klaus** always-on (embedded .bin committed upstream, no provisioning). **Harte** runs a 12-opcode smoke subset in CI — `a9 69 e9 d0 4c 6c 20 b1 9d fe 00 ca` — fetched on demand from SingleStepTests (the 1GB corpus is `.gitignore`'d upstream); full 256 is local-only (`scripts/check_cpu_conformance.sh full`). New `scripts/check_cpu_conformance.sh` (+ `--selftest`) + two CI steps. **Self-test (mandatory):** corrupt one expected `final.a` in a Harte case → the gate must go RED (proven live, not vacuous). **Src:** Klaus2m5 repo; SingleStepTests/65x02 (MIT).
- **VV-2** *(flagship new class — proof, attacks the #1 gap B):* `cmd/cyclebound`+`internal/cyclebound` builds a 6502 CFG between `STA WSYNC` strobes, costs each block from in-tree `instructions.Definitions` (exact cycles + page/branch penalty), proves longest reachable path ≤76cy (DAG longest-path, no solver). Reuse `internal/build.AssembleWithListing` + `internal/srcmap`. Out-of-scope = unbounded loops → reported, never silently passed. Self-test: a litmus that overruns only on one branch must be FLAGGED though a lucky run passes; observed max ≤ proven bound = CI assertion. **Src:** Li&Malik IPET DAC'95; Ballabriga&Cassé WCET'08.
- **VV-3** *(unlocks a whole adequacy axis cheaply):* add `pcSeen`/`branchEdges` recorder inside `emu.stepInstr()` (uses `LastResult.Address`/`IsBranch()`/`BranchSuccess`); `cmd/cover` emits dead-code + one-sided-branch map; then `RunGuidedFuzz` keeps inputs hitting new edges. Self-test: planted unreachable branch reads as uncovered; guided fuzz reaches a deeply-guarded state blind fuzz misses. **Src:** Zalewski AFL whitepaper; Go native fuzzing.
- **VV-4 ✅ DONE (v1.79.0):** `internal/motion` (pure `Analyze` + `TrackObject`) tracks an object's exact X (`Markers().HmovedPixel`) and rendered top (column scan over a uniform-background window) over N frames → velocity / accel / **jerk_rms** (RMS of the 2nd difference; 0 = constant velocity) + `max_accel`/`monotonic` (glitch vs benign staircase). Shipped 3 ways: `cmd/motion` CLI, **`read_motion` MCP tool** (interactive — used live on the Breakout ball: vertical jerk 0, horizontal jerk 1 = the benign 1px/2-frame staircase, not a bug), and scenario **`checks.motion: max_jerk_rms`** (regression gate). Litmus `motion_glide` (clean +1/frame → jerk 0) + `motion_stutter` (+2,0,+2,0 → jerk 2). Self-test = Go `TestMotionSelfTest` (glide jerk 0 vs stutter ≫) + scenario probe. **Validated against the user's own perception: motion_stutter run in Stella reproduced their exact "ブルブル" symptom.** **Src:** Flash & Hogan min-jerk 1985.
- **VV-5** *(one file, every game needs it):* `temporal` block in `internal/scenario` with 4 O(1)–O(K) LTL₃/bounded-MTL monitors (always / eventually-within-K / response{after,then,within} / never-P-for-N), reusing the existing condition struct + `resolve()`; "inconclusive" reported distinctly so liveness can't be vacuously green. Self-test: `eventually` fails on a never-written cell; `response` with `within` one frame too tight fails, true latency passes. **Src:** Bauer/Leucker/Schallhart TOSEM 2011; STL RV'15.

## Tier 2 — high value, more dependent or larger
- **VV-6 / VV-7 (cross-oracle):** MAME a2600 runs `-video none -autoboot_script <lua> -seconds_to_run` = a genuinely independent, **hands-free** 3rd oracle (catches corners Gopher2600+Stella both get wrong); perfect6502 is the **silicon-netlist** CPU tier (CPU-only; shelled-out to keep the main build CGO-free); `oraclevote` fuses Gopher2600+Stella+MAME(+perfect6502) into a majority verdict that surfaces "all software agrees but the hardware-grade member dissents" = the project's reason to exist. FPGA/real-2600 = manual escalation tier only (human cost). Extract shared `internal/oracle/` from today's `cmd/stellacheck`. **Src:** MAME luascript docs; mist64/perfect6502 (from visual6502).
- **VV-8 (trajectory diff):** `cmd/trajdiff` steps original vs authored ROM in lockstep on one input timeline, reports first-divergence frame+field (reuses `scenario.ResolveField`). The strongest oracle for a reproduction task. Self-test: identity = MATCH (also a determinism guard); a one-byte mutant diverges at the right frame; a dead-byte mutant = MATCH (diffs behavior, not bytes). **Src:** Martignoni TOSEM'13; EXAMINER ASPLOS'22; McKeeman 1998.
- **VV-9 (score OCR):** template-match 2600 fixed-bitmap digits (learn templates from the ROM font table; Hamming match) → assert displayed == decode(RAM score); catches display-kernel/BCD/font-index bugs exact hashes miss. `internal/ocr` + scenario `checks.score_equals_ram`. Pure-Go. Self-test: mutate one font byte (garbled glyph) without touching RAM → displayed≠RAM caught. **Src:** pHash Hamming primitive.
- **VV-10 (HW-trap detectors):** siblings of `assert_line_budget` in `internal/emu`: T-1 RIOT timer-wrap (=G8; Gopher2600 `timer.go` models the 1T flip and cites AtariAge 303277 in-source), T-2 HMOVE-then-HMxx<24cy, T-3 uninitialized-RAM read (shadow-memory mask). Each with a planted-trap ROM vs a clean twin, both directions CI-locked. Start with T-1/G8. **Src:** known-traps.md §A/§D; Valgrind Memcheck (shadow memory).

## Tier 3 — polish / softer / defer
- **VV-11** state-coverage matrix + coverage-filtered mutation (honest kill-rate; discharges the playbook's flagged 5–20%). **VV-12** SSIM/pHash tolerant lane (adds magnitude+locality; does **not** replace exact golden). **VV-13** audio FFT/RMS-envelope (new modality; audio rarer on the roadmap). **VV-14** `cmd/cpucert` citable certificate, the Z3/ILP prover upgrade (infeasible-path + value-range invariants — defer until C-1 proves out or a kernel demands it), and external TIA/Sim2600 silicon tie-breaker ROMs (ROM-licensing sensitive). C-3 ESIL/radare2 symbolic exec was **assessed and declined** (foreign Python+r2 runtime vs pure-Go; unvetted 6502 timing model) — on record so it isn't re-litigated.

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
