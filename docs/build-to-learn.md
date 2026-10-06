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
- **Someone else's source may not assemble here even when it assembled for them.** None of this
  repository's `.asm` files uses `SEG` (`rg -l --no-ignore -g '*.asm' -g '!**/Gopher2600/**' -g
  '!**/third_party/**' -e '\bSEG\b' .` from `harness/` → 0 of 211, 2026-10-01), so a segmented source
  is the unfamiliar case. Andrew Davie on a source that placed `ORG $F000` before its uninitialised-RAM
  segment: *"move the ORG $F000 to AFTER the uninitialised RAM segment, just after the SEG"*, *"Each
  segment should really be given an ORG"*, and *"It wouldn't even assemble for me without this change.
  I don't know how you've managed to make it work so far!"* 〔stella-list `200402/msg00021`〕. He also
  names reading *"the segment tables at the end of assembly"*. Why it built for one and not the other
  is not said (a different DASM version is a guess). Cited only, not verified.
- **A ROM with no recorded origin may still name its author in its own bytes.** A demo circulating
  only as "Scroller Water Demo" was asked about on AtariAge; djmips, noting the question seemed
  answered, asked whether it was Paul Slocum of Tree Wave and added that the binary carries an
  embedded Spanish sentence which, translated, credits Tree Wave and claims the effect was done on an
  unmodified 2600 〔AtariAge `topic/128547`; held here only as distilled notes, so the wording is not
  checked〕. Reading the printable text in the image is a step to try when the md5 matches no
  catalogued release (our reading). Cited only, not verified.
- **Byte-identical proves the bytes, not the labels.** Labels and comments do not change the bytes, so
  the table's `dasm -f3` check passes whatever they say. Dennis Debro, having started disassembling
  other games, ran his own game Climber 5 through DiStella: *"If I were to label and comment on the
  code I got from Distella, I don't think I would have got it right. I would have especially missed
  the part of the code where I figured out if the climber could ascend or descend a ladder"*
  〔stella-list `200407/msg00019`〕. Read an annotated disassembly's comments as its annotator's
  reading (our reading). Cited only, not verified.
- **Heavy comments are the exception.** SpiceWare, 2022, of his tutorial Collect: *"Since it's a
  tutorial I went overboard with the comments, to an extent that you most likely will not see in other
  source code or disassemblies."* The same source says why its labels end in a colon: *"The : is
  optional. However, if you remember to include the : in all of your labels you can then easily find
  where something is defined by including : in the search. Find "Score:" will bring you here, find
  "Score" will locate all places that the variable Score is used."* 〔AtariAge `topic/342613`〕. Cited
  only, not verified.
- **Changing a disassembly without losing the byte-identical build.** Manuel Polik, 2002, on the NTSC
  titles he and Fabrizio Zavagli were converting to PAL: *"for A-Team there's compile switches in it
  that compile either into a bit-perfect NTSC original or fixed scannline PAL/PAL60 versions. Even
  creating a fixed scannline NTSC version is supported"* 〔stella-list `200212/msg00138`〕. A switch that
  still assembles the original keeps the table's `dasm -f3` check runnable after your edits (our
  reading). Cited only, not verified.

## Reading a game that has no labels yet (your own distella listing, and its screen)
- **Where to start.** Piero Cavina asked Thomas Jentzsch how he disassembles a game as complex as
  Pitfall!. Roger Williams answered first: look for a *"Rosetta Stone"*, such as *"references to the
  joystick ports, or the very universal snippets used for vertical sync which bracket a kernel"*
  〔stella-list `200110/msg00172`〕. Jentzsch, replying to him, gave his own order: divide the code into
  the four sections *"by looking for accesses at VSYNC, VBLANK, TIM64T and INTIM"*; look for `LDA
  (ZP),Y` to find the pointers to the graphics data, then where they are loaded, which gives the
  pointer tables; from there the score display (the 48-pixel routine) and the score and lives
  variables; *"verifing the new variables by looking for collision register access before they are
  changed"*; HMxy and RESxy for the position variables; then playfield tables and the other TIA
  registers. After that *"it depends on the game"*. Searching for the still-unknown RAM addresses is
  *"the most frustrating step"*, and *"Very often, there is a combination of variables that has to be
  identified together"*. He names labels and tables only once the variables are identified
  〔stella-list `200110/msg00173`〕. He adds that it got easier after his first, because the Activision
  games he did were from the same period and some routines are *"(nearly) identical"*, and that his
  Surround disassembly was *"quite hard and frustrating, because the coding was sometimes very obscure
  (everything ws quite new then) and I found it hard to find the expected code patterns"*. Cited only,
  not verified.
- **DiStella gives an address one name; the TIA's read and write registers share addresses.** `$08`
  is COLUPF when written and INPT0 when read. David Galloway, 2005: *"I found it a pain to have to
  search through the output to find the loads from COLUPF and change them to loads from INPT0"*; B.
  Watson: *"If it's a load, it must be INPT0. If it's a store, it's COLUPF"*. In the same thread Dennis
  Debro asked whether DiStella could be told that a ROM uses `$42` instead of `$02` for WSYNC; the only
  game he had seen do it was Miner 2049'er, and Eckhard Stolberg answered: *"All games and demos with
  3F bankswitching access the TIA through it's mirror at $0040 - $007F. This is because the
  bankswitching logic checks for accesses to $003F and below."* 〔stella-list `200506/msg00077`,
  `msg00081`, `msg00082`, `msg00083`, `msg00089`〕. Whether today's DiStella handles either case is
  not checked here. Cited only, not verified. (`cmd/dissect`'s own store trace folds a target below
  `$80` onto its register with `target&0x3F`, for stores to nine drawing registers only —
  `cmd/dissect/main.go`.)
- **The HMOVE comb shows where the kernel strobes HMOVE.** An `HMOVE` right after `WSYNC` blanks
  colour clocks 0-7 of that line (`known-traps.md`, "HMOVE comb on a visible line"). Piero Cavina,
  1997, in the thread he had opened asking why the line appears: *"What is sure is that when you see
  such a line, a sprite is being positioned on that scanline"*, and *"it tells you a lot about how the
  program is working! Some games have a lot of lines - see "Spiderman" for example, if I remember
  correctly - there's one of them every 2 or 3 scanlines"* 〔stella-list `199703/msg00211`〕. A comb
  marks a line whose HMOVE was strobed early in its HBLANK (right after WSYNC is the usual case;
  `known-traps.md`'s mid-line HMOVE row records that a strobe at cycle 75 re-raises the comb), not
  necessarily a repositioned sprite (our reading): in the same thread Nick Bensema notes that *"some games
  purposefully create this blankness down the entire screen for aesthetic purposes"* 〔stella-list
  `199703/msg00199`〕. Cited only, not verified.

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
- The clean-room line in step 2 of "How to run one rung" is about code. Measuring the original's
  picture is part of this template (Phase 0's dimensional spec; `vismatch -genpf` generates
  playfield tables from the target, `reproduce-loop.md`); what this document does not say is
  whether another game's graphics bytes (sprite bitmaps, animation frames) may be copied in. The
  question has come up on the list from the art side: Eckhard Stolberg, to Andrew Davie about his
  large-sprite demo Fu Kung!, that *"Someone actually has to draw all those animation frames"*, and
  Clay Halliwell in reply: *"Crazy idea-- why not copy the sprites from Karateka? Most of them are
  about the right size for this sprite system. In fact, why not do a straight port of Karateka?"*
  〔stella-list `200301/msg00151`, `msg00156`〕. No later message in the thread as archived takes the
  suggestion up. Cited only, not verified.
- Reuse the existing harness assets (do not add new implementations): `assemble_and_load`/`load_rom`/`get_screen_annotated`/`read_row`/`read_tia_registers`/`read_collisions`/`step_frame`/`set_input`/`assert_line_budget`/`run_scenario`/`cmd/scenario`/`cmd/dissect`/distella.

## Deliverables and where they go
- **The self-built ROM (the artifact)**: `roms/<game>/<game>.asm` + `scenarios/*.json` (under git). Grow it rung by rung.
- **The study (outside the repo)**: `reference/disassemblies/_casestudies/<game>/{impl-map,fixtures,diff-gaps}.ja.md` + `manual/`. The disassembly original = `reference/disassemblies/<Game>_<author>/`.
- **Promotion (after completion, with sources, lint green)**: `casebook.md` (a situation→technique entry) / `design-principles.md` if there is a new technique / `sandbox/EVALUATION.md` (scoring "could write it"). `check_wiring`/`check_provenance` green, `CHANGELOG` updated, push/tag confirmed.

## Compounding (why it pays to keep going)
Each game's diff thickens casebook / design-principles and makes the next game easier. Passive (casebook) = the map of techniques; active (build-to-learn) = the hands for them. Together they build the foundation for the capstone in [[project-roadmap-to-pong-capstone]] (one image → an original production).

## Worked example
**Breakout (Atari 1978)** was the first. An 8-rung ladder (stable frame → left and right walls → a 6-colour brick wall [asymmetric PF] → score → ball reflection → paddle [capacitance read] → brick collision → game state). Details = the approved plan `~/.claude/plans/cheerful-noodling-origami.md` + `_casestudies/breakout/`.
