# Authoring protocol — START HERE to build a 2600 ROM

**This is the single entry point for making a ROM.** It activates the whole accumulated knowledge base in
order, so nothing rots unused. The loop is self-strengthening: each production sharpens the rules/checks
(compounding). Rule: [[feedback-authoring-loop-system]] / [[knowledge-activation-architecture]]. Goal:
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
   - `docs/mining-digest.md` / `docs/design-principles.md` → the rules for the feature at hand.
   - `docs/techniques/` → the nearest verified technique to clone.
2. **Plan against checks** — run the design through `pkg/design` feasibility (budget / color bands /
   multiplex / PF windows / positioning). Reject an unworkable layout **on paper**, before asm.
3. **Author** — write the asm, cloning the nearest verified `roms/techniques/<name>.asm`.
4. **Pre-flight** — `python3 scripts/check_traps.py <file.asm>` (the static "emu-passes/HW-fails" linter,
   spec = `docs/known-traps.md`). Walk the runtime-only traps (timer wraparound, HMOVE-24cy) by hand /
   `breakif`.
5. **Verify** — `assemble_and_load` → `run_scenario` (numeric asserts + golden + 262) → `get_screen_annotated`
   (visual: not blank, the technique reads). Horizontal verdict = `read_tia` HmovedPixel; vertical = scanline.
6. **★Feedback (the compounding core)** — when something fails or a gap surfaces, feed it back:
   - a missed *known* trap → strengthen `check_traps.py`;
   - new knowledge → distil to `design-principles.md` (with provenance);
   - a reusable pattern → promote to `docs/techniques/` or `pkg/`.
   Every production makes the next one safer and faster.

## Why this exists
Past Pong attempts died at step 4/5 (unverified timing). The corpus + checks turn "Claude that knows things"
into "Claude that gets better at making things." The mechanical parts (pkg/design, check_traps, CI) are
enforced; the prose steps are followed each time.

## Hooks
- CLAUDE.md iron rule 5 ("design before asm") points here.
- CI gates: `check_provenance.py` + `check_traps.py` (+ scenario regression) — green is necessary, not sufficient
  (also run the visual/`read_tia` verdict, per the iron rules).
