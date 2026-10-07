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
  A mock made to try the object budget may be unfinished as a picture: bladejunker's character-select
  mock was made to find how to distribute the drawing over the objects, and *"It's not a complete
  mockup visually"* (AtariAge `topic/194635`; **Cited only, not verified** — read from distilled
  notes, not the thread).
  Image-first is one way in, not the only one. Asked how they begin a project (AtariAge `topic/259052`,
  2016), gemware-games: *"Generally, unlike most programmers, I know the exact gameplay beforehand. I
  usually write a story board"*; gauauu: *"I usually start extremely simple (ie get a sprite showing on a
  background) and very slowly iteratively transform it into the game that's in my imagination"*;
  tschak909, of Dodgeball: *"the idea knocked me upside the head"*. **Cited only, not verified.**
- **B. Bottom-up build order.** Build + verify in the canonical 14-step sequence (stable display → timers →
  score → 2-line kernel → VDEL → playfield → input → variations → RNG → ball → missiles → sound → animation →
  polish). → `docs/cookbook.md`.
  **Where each routine goes in the frame** is a separate choice. SpiceWare's starting layout: input,
  movement of what the player does not control, display prep (position the TIA objects, set up what the
  kernel needs), kernel, collisions — *"and I start out with 1-3 in Vertical blank and 5 in Overscan as
  Vertical Blank has a lot more processing time available than Overscan. As the project progresses I
  may have to shift some of the routines around"*; gauauu arrived at the same order (AtariAge
  `topic/252613`; **Cited only, not verified**). The examples in `docs/design-principles.md` put
  movement in overscan instead (Combat; the "lodging" pattern for physics lines).
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
   - **Which resource to decide first** is a different order from B's order of building. Thomas
     Jentzsch, 2004, answering Christian Bogey's question how to write an effective kernel: understand
     the hardware; *"Then usually the first step is to think about the playfield graphics. Do you need
     them and do they have to be asymmetrical? Do you have to draw them every scanline? Do you need all three PF registers (often PF0
     is not used). How much RAM will you need for drawing the PF? Then you can calculate how much time is
     left and how often you can update the sprites. This is the point where you should decide after how
     many scanlines the kernel will repeat."* Then: repositioning sprites inside the kernel, VDEL, the
     remaining cycles for missiles or colour changes, and *"sometimes"* a specialised kernel per
     horizontal stripe (Pitfall) — and *"At any of those steps you will quite often go back to the
     previous points, change your decision and restart the iteration again"* 〔stella-list
     `200405/msg00048`〕. **Cited only, not verified.**
3. **Author** — write the asm, cloning the nearest verified `roms/techniques/<name>.asm`.
   - **Scope labels with `SUBROUTINE`.** Thomas Jentzsch, 2001, to Glenn Saunders' complaint that a
     label does not show whether it starts a subroutine: *"There is a solution, use SUBROUTINE! (DASM
     rocks!) Now you can give all local labels a name starting with ".". Only the subroutine labels and
     the extra startpoints get a normal name without the point."* 〔stella-list `200110/msg00440`〕
     DASM's manual, as SpiceWare quoted it: the directive *"logically separates local labels (starting
     with a dot). This allows you to reuse label names (for example, .1 .fail) rather than think up
     crazy combinations of the current subroutine to keep it all unique"* (AtariAge `topic/291397`);
     joe-musashi quoted the same passage to a beginner (AtariAge `topic/198465`, read from distilled
     notes, not the thread). The reuse is measured with DASM 2.20.14.1 in `docs/known-traps.md`
     (*A `.label` used twice in one file is a "Label mismatch" until `SUBROUTINE` separates them*);
     the rest is **Cited only, not verified**.
   - **Write down what a routine takes and leaves.** Jentzsch, same post: *"Give every subroutine a
     comment header where you describe the input and output parameters (registers, variables and
     flags), and the functionality of course."* 〔`200110/msg00440`〕 Earlier that year, finding bytes in
     Andrew Davie's Qb, he struck out an `lda #0` after `jsr MBlock` (*"MBlock returns a=0"*) and added:
     *"If you document the state of registers and flags when returning you might find some more."*
     〔`200103/msg00019`〕 **Cited only, not verified.**
     Jentzsch's comment-header advice answered Glenn Saunders, who had written that *"there are no
     rules for how to pass parameters to assembly subroutines. You can use the stack, use the
     registers, or use general RAM."* 〔`200110/msg00435`〕 Our reading: with no convention to fall
     back on, the header is where a routine's contract has to be written down. **Cited only, not
     verified.**
   - **Write a register's bits as bits.** Rodrigo Silva, 2003, of a template he offered to beginners:
     *"I used binary notation when writing to addresses like VBLANK, to make clear that in theory im not
     writing 2, but rather setting D1 bit"* 〔`200309/msg00290`〕. Two technique ROMs here write binary
     immediates (`#%`) — `roms/techniques/shared_setxpos.asm` for the NUSIZ, CTRLPF and ENAxx bits and
     `roms/techniques/rts_dispatch.asm` for a playfield pattern — and both still write VSYNC and VBLANK
     as `#2`. Binary data is commoner: 15 `.asm` under `roms/` hold a `%` literal of four or more digits
     (`rg -l '%[01]{4,}' roms/`, 2026-10-05). **Cited only, not verified.**
   - **Have the assembler print sizes while you build.** Dennis Debro, 2003, answering Kirk Israel's
     question whether he might have a 2K ROM on his hands: `echo "***", (*-Start), " BYTES OF ROM USED"`
     before an `org` — *"The real work is done in* `(*-label)`*. This calculates the number of bytes used between the
     current position and the label specified"*, read off DASM's output or the list file
     〔`200309/msg00001`〕. Silva's template has `ECHO` lines for the RAM and ROM bytes used and left
     〔`200309/msg00290`〕. An `ECHO` prints once per pass: `docs/known-traps.md` (*`ECHO` prints once
     per pass, and a symbol defined after it makes it print once*). `internal/build.ROMBytesUsed` reads
     the finished image instead, a lower bound. **Cited only, not verified.**
   - **Put the deadline next to the cycle count.** Andrew Davie, 2001, explaining his Qb comments to
     Glenn Saunders: `instruction ;cycles ->ends@time < must_start_before (now_starting@)`, so that
     `sta PF0 ; 3 ->45 <49 (@42)` *"Tells ME that the instruction starts at cycle 42, SHOULD start
     before cycle 49 (it does), and that it ends at cycle 45"* — *"a sort of shorthand that I use to
     check that I'm doing everything allright"* 〔`200102/msg00019`〕. In the same post Davie asked
     readers not to rely on the timing in his previously submitted code: *"I've only just tightened it
     up"*. His count appears to include the `sta WSYNC` itself — after `sta WSYNC ; 3` and
     `lda #%00000000 ; 2` the next store is marked `(@5)` — so his figures may sit 3 above a count that
     starts at 0 when `WSYNC` releases (our reading). The comment is still hand arithmetic (iron rule 2): check the line with `prove_line_budget`, and
     playfield writes with scenario `checks.pf_deadlines`, which uses neither count: it judges each write
     by the beam clock it lands on. `cmd/framegen` writes a similar pair — the
     landing clock and the limit — into the source it generates. **Cited only, not verified.**
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
