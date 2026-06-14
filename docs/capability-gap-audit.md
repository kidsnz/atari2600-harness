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
