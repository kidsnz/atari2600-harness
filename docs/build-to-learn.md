# Build-to-learn — the reusable template that turns "reading" a real game into "writing" one yourself

> **This is a starting point = a reusable methodology template** (referable from any session). It is the **active counterpart** to `casebook.md`, which learns "situation → technique" by *reading* real games (passive): reproduce a real game's mechanics **one at a time in asm yourself and match the numbers against the real ROM**, turning "can explain" into "can write". First use = Breakout (2026-06-15). The field version of the authoring loop `authoring-protocol.md`.

## When to use it
- When a casebook study shows that "my own drawing / implementation craft is weak" ([[feedback-goal-standard]]).
- When you want to **embody a technique by doing** rather than reconstructing it on paper. The aim = raise capability by **accumulating small successes**.

## Prerequisites = 3 materials (all with recorded provenance · [[feedback-provenance-always]])
| Material | Role | Where to get it | Verification |
|---|---|---|---|
| a verified ROM | ground truth for behaviour | atarimania ([[reference-atarimania-roms]]) / extract past any padding with dd | md5 matches the canonical release |
| the official manual | spec | archive.org (PDF + OCR `_djvu.txt`) | take the official edition as primary (beware other brands such as Sears) |
| an annotated disassembly | impl (the implementation's answer) | AtariAge (Debro and others) / roll your own with distella if none exists | confirm **`dasm -f3` → byte-identical to the real ROM** |
- The manual alone never captures the whole behaviour = **always add observation of real play** ([[feedback-verification-standard]]).

## Phase 0 — thorough scrutiny (always, before writing)
1. **A manual ↔ code correspondence map** (`_casestudies/<game>/impl-map.ja.md`, clean-room prose only): map each section of the manual onto the disassembly's routines / RAM / tables. Format = a table (section | behaviour | code | RAM).
2. **ground-truth fixtures** (`_casestudies/<game>/fixtures.ja.md`): **take the numbers off the real ROM once** and freeze them as the comparison standard for every rung (TIA colour values, coordinate clocks, scanline ranges, initial values). Take them with `peek` (RAM) / `read_row` / `read_tia_registers`. **The verdict is numeric** (Iron rule 1).
3. **★Fix the dimensional spec first (mandatory in Phase 0 — never defer it)** (`_casestudies/<game>/layout-compare.ja.md`): with `get_screen_annotated` (an "eye" calibrated to real TIA coordinates), **measure the position and size (Y / clock / width / scanline count) of every element of the original up front and make that the target spec**. Build each rung to that spec. **Why = the layout ties directly to kernel regions, scanline allocation and positions, so fixing it after production is under way means starting over** (= "measure, then build", the same as for colour and RAM). Once your own version exists, check and converge the difference with annotated (read_row / annotated). 〔user's point 2026-06-15: measure the layout first of all〕

## Production strategy = the bottom-up ladder (default)
**display → static elements → moving elements → input → collision → game state**, one mechanic at a time. A working ROM always remains, and every rung passes a firm numeric check = successes are maximised.
- Alternatives: **B, mechanics-first** (the interactive core arrives sooner but the hard parts come early) / **C, two parallel tracks** (drawing plus a physics sandbox at once → integrate). C's parallelism is folded into the default A as **a "spike" ahead of a high-risk rung** (record the failed approaches as technique knowledge too = "a method that does not work is also learning").

## How to run one rung (every rung, small steps)
1. **Define the DoD numerically up front** (verification-first) = "what has to appear to pass", against the fixtures. Write the scenario first as well and leave it as a regression in `roms/<game>/scenarios/`.
2. **Attempt it yourself, sealed** (the easy rungs). For a hard rung: attempt it → when stuck, **read the disassembly's method and learn the technique** → **write it yourself** (clean room = never transcribe).
3. assemble (`assemble_and_load`) → run it → **match the numbers** (`read_row`/`read_tia_registers`/`read_collisions`/`step_frame`/`set_input`) → **one commit** ([[feedback-execution-discipline]]). If anything looks wrong, **revert immediately** (Iron rule 3).
4. **Record the difference** (your method vs the disassembly's method) = `diff-gaps.ja.md` = the capability gap = the learning.

## Engineering discipline
- Compare against the frozen fixtures (never judge subjectively) / take cycles from the simulator (rule 2) / where a litmus applies, back it numerically (rule 4).
- risk-first: pin a high-risk rung's technique numerically with a throwaway spike first.
- Reuse the existing harness assets (do not add new implementations): `assemble_and_load`/`load_rom`/`get_screen_annotated`/`read_row`/`read_tia_registers`/`read_collisions`/`step_frame`/`set_input`/`assert_line_budget`/`run_scenario`/`cmd/scenario`/`cmd/dissect`/distella.

## Deliverables and where they go
- **The self-built ROM (the artifact)**: `roms/<game>/<game>.asm` + `scenarios/*.json` (under git). Grow it rung by rung.
- **The study (outside the repo)**: `reference/disassemblies/_casestudies/<game>/{impl-map,fixtures,diff-gaps}.ja.md` + `manual/`. The disassembly original = `reference/disassemblies/<Game>_<author>/`.
- **Promotion (after completion, with sources, lint green)**: `casebook.md` (a situation→technique entry) / `design-principles.md` if there is a new technique / `sandbox/EVALUATION.md` (scoring "could write it"). `check_wiring`/`check_provenance` green, `CHANGELOG` updated, push/tag confirmed.

## Compounding (why it pays to keep going)
Each game's diff thickens casebook / design-principles and makes the next game easier. Passive (casebook) = the map of techniques; active (build-to-learn) = the hands for them. Together they build the foundation for the capstone in [[project-roadmap-to-pong-capstone]] (one image → an original production).

## Worked example
**Breakout (Atari 1978)** was the first. An 8-rung ladder (stable frame → left and right walls → a 6-colour brick wall [asymmetric PF] → score → ball reflection → paddle [capacitance read] → brick collision → game state). Details = the approved plan `~/.claude/plans/cheerful-noodling-origami.md` + `_casestudies/breakout/`.
