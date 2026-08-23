# Authoring protocol — START HERE to build a 2600 ROM

**This is the single entry point for making a ROM.** It activates the whole accumulated knowledge base in
order, so nothing rots unused. The loop is self-strengthening: each production sharpens the rules/checks
(compounding). Rule: [[knowledge-activation-architecture]]. Goal:
[[project-roadmap-to-pong-capstone]].

## How a veteran builds (the mined pro workflow — A–E)
Distilled from real homebrew dev diaries (SpiceWare et al.) — the way an experienced 2600 engineer actually works:
- **A. Image-first.** Design the screen/title in Photoshop **first**, then write the kernel to it. A designed
  48-px image → on-screen title goes through the **flicker-free 2-color 48-px kernel** (`multicolor48`/`bitmap48`).
  → see `docs/cookbook.md` "title from a Photoshop mock". *(This is the project's whole reason for the harness.)*
- **B. Bottom-up build order.** Build + verify in the canonical 14-step sequence (stable display → timers →
  score → 2-line kernel → VDEL → playfield → input → variations → RNG → ball → missiles → sound → animation →
  polish). → `docs/cookbook.md`.
- **C. Know the ceiling.** Vanilla first; DPC+/ARM/CDF "beyond-bB" is a later track → technique-candidates.
- **D. Audio truths.** TIA = LFSR-pair voices (not a table); AUDF-lowering lags ≤32cy; 2 voices can cancel to
  silence; Gopher2600 noise ≠ real HW → `docs/known-traps.md` E.
- **E. Craft is cycle budgets.** Real games are won on per-line cycle math (mask-sprite 21cy, drop-PF0 frees
  12cy, flicker algorithms for >2 objects) → `docs/design-principles.md`.
- **Debug like a pro:** Stella Fixed Debug Colors (ROYGBIV per object) + the numeric verdicts (read_tia / scenario).

## The 6 steps (run for every kernel/feature)

1. **Retrieve** — before writing, pull the relevant knowledge:
   - `docs/cookbook.md` → the recipe for this game-type (technique stack + traps + checks).
   - `docs/casebook.md` → how a real commercial game solved this situation (situation → technique, evidence-backed by disassemblies).
   - `docs/mining-digest.md` / `docs/design-principles.md` → the rules for the feature at hand.
   - `docs/integration-density-playbook.md` → when the constraint is *fit* (bytes/RAM/cycles interlock):
     the density principles, the **density scorecard** to score against a reference, and the practice ladder.
   - `docs/techniques/` → the nearest verified technique to clone.
2. **Plan against checks** — run the design through `pkg/design` feasibility (budget / color bands /
   multiplex / PF windows / positioning). Reject an unworkable layout **on paper**, before asm.
   - **If the layout is a row of shapes at fixed x, run `plan_sprite_placement` FIRST** (or
     `cmd/place`). Where the objects can go is decided by three grids that do not line up, clamps
     that are windows of cycles, and copies that wrap past 160 — worked out by hand it comes back
     "impossible" for rows that place fine, which has now happened twice. It returns the bases,
     NUSIZ codes and strobe cycles, or the reason there are none. `docs/techniques/sprite-placement.md`.
3. **Author** — write the asm, cloning the nearest verified `roms/techniques/<name>.asm`.
4. **Pre-flight** — `python3 scripts/check_traps.py <file.asm>` (the static "emu-passes/HW-fails" linter,
   spec = `docs/known-traps.md`). Walk the runtime-only traps (timer wraparound, HMOVE-24cy) by hand /
   `breakif`.
5. **Verify** — `assemble_and_load` → `run_scenario` (numeric asserts + golden + 262) → `get_screen_annotated`
   (visual: not blank, the technique reads). Horizontal verdict = `read_tia` HmovedPixel; vertical = scanline.
   Choose the oracle and rigour for the claim using `docs/testing-playbook.md` (invariants / property /
   metamorphic / fuzz / mutation) — and for any *emergent* claim, demonstrate that behaviour directly
   (free-run, no poke), don't infer it from component checks. Apply the **verification standard**'s MAX
   checklist (memory `feedback-verification-standard`): continuous frame-by-frame trace, full-window reads,
   formula↔pixel cross-check, eliminate each hypothesis with data, prove the negative, present the measured table.
6. **★Feedback (the compounding core)** — when something fails or a gap surfaces, feed it back:
   - a missed *known* trap → strengthen `check_traps.py`;
   - new knowledge → distil to `design-principles.md` (with provenance);
   - a reusable pattern → promote to `docs/techniques/` or `pkg/`.
   Every production makes the next one safer and faster.

## Reproducing a reference image pixel-exact (the image-match loop)
When the task is "make the ROM look like THIS image" (a Stella snapshot of a real ROM, or a Photoshop mock —
the project's core use case, workflow A above), run this **measured convergence loop**. It is how the PONG
static frame reached ~0.1% diff. Judge by the per-element ruler, not the eye.

0. **Clean-reference contract.** Pixel-exact reproduction is capped by the target's cleanliness: use a **Stella
   F12 PNG (TV effects off, integer scale)** or the **ROM itself** — not an OS screenshot / resized / filtered
   image (non-integer scale → fuzzy measurement). Get two things from the user up front: **semantics** (which
   TIA object each element is — "net = the thin centre line, scores = players, ball is square") and **fidelity**
   ("match exactly" vs "rough mock"). They eliminate guesswork and mutual misreading.
1. **Measure the target per element** — `framesim -spans` (read column A): every element's exact extent in
   clock×scanline. This is the ruler.
2. **Author / render** the kernel.
3. **Localize** — `framesim -align -diff out.png` (where it's wrong) + `-up` (sharp/strict, no downscale blur).
4. **Measure yours per element** — `framesim -spans -a rom.bin -b target.png`: row-by-row clock-spans, target
   vs yours, differing rows marked.
5. **Fix ONE element, re-measure** (small steps — [[feedback-execution-discipline]]).
6. At convergence, **the user does a visual pass** (`get_screen_annotated` is the channel) and names any element
   still off; fix each exactly.

**Two rules for this loop (both learned the hard way on PONG, 2026-06-19):**
- **Measure per element — don't trust the global SSIM/diff alone.** A 1-row fencepost error hides in the global
  number but the eye (and `-spans`) catches it (the frame read "done" globally while 3 elements were each off a row).
- **Never call a localized diff "the floor / irreducible" without proving it.** If `-spans`/`-diff` shows a region
  off, it is a *solvable target* — exhaust the fix. If a real hardware/structural limit blocks it (object count,
  PF 4-clock granularity, a row trade-off like `docs/known-traps.md` §A PF-coverage), show that limit numerically
  before reporting "floor." (= [[feedback-verification-standard]] "prove the negative" + [[feedback-execution-discipline]].)

## Why this exists
Past Pong attempts died at step 4/5 (unverified timing). The corpus + checks turn "Claude that knows things"
into "Claude that gets better at making things." The mechanical parts (pkg/design, check_traps, CI) are
enforced; the prose steps are followed each time.

## Hooks
- CLAUDE.md iron rule 5 ("design before asm") points here.
- CI gates: `check_provenance.py` + `check_traps.py` (+ scenario regression) — green is necessary, not sufficient
  (also run the visual/`read_tia` verdict, per the iron rules).
