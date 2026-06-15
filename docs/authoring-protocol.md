# Authoring protocol — the loop run on every kernel/feature/game

The repeatable, self-strengthening loop Claude runs when authoring 2600 assembly, so the mined corpus is
**used at write time** (not just stored) and **each production sharpens the system** (compounding).
Rule: [[feedback-authoring-loop-system]]. Goal: [[project-roadmap-to-pong-capstone]].

## The 6 steps

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
