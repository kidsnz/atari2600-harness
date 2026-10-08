# Technique ⑱ — RTS-stack modular kernel dispatch

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ0 = $06` — three copies at the wide spacing — so a base past ~96 puts the last copy over the edge. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

> Also known as: **modular kernel** (vitoco, AtariAge topic 313777). The **dynamic** sibling of zone
> multiplexing: the screen is composed of selectable vertical zones whose order is **data in RAM**, and the
> beam walks them by chaining zone routines through the **RTS-stack trick** at a constant ~6-cycle cost.

## Goal
A screen built from N vertical **zones**, where *which* zones appear and *in what order* is a **RAM list of
zone IDs** — not a fixed, hand-unrolled kernel. With many zone *types* a fixed kernel suffers a combinatorial
explosion; here a single dispatcher chains arbitrary zones with **no `case`/branch ladder** and a **constant**
per-zone switching cost. This is the dynamic generalization of `zone_multiplexing` (whose zones are fixed).

## Demo (`roms/techniques/rts_dispatch.asm`)
Four zone types, each a self-contained routine that draws a fixed `ZONE_H = 40` scanlines and ends in `RTS`:

| ID | routine | what renders (numerically verified, `read_row`) |
|----|---------|---------------------------------------------------|
| 0 | `ZoneSolid`  | solid blue band (`COLUBK $84` → `1D20CA`) |
| 1 | `ZonePF`     | yellow/red striped playfield band (`FFFF30`/`EC3333`) |
| 2 | `ZoneSprite` | brown band with 3 white diamond sprites (NUSIZ0 3-copy; `FFFFFE` columns over `FFAD37`) |
| 3 | `ZoneStripe` | per-scanline color-cycling band (`CD32A6` mid-band) |

The active screen is the RAM list `zonelist` (`$90..$93`). It boots as `[0,1,2,3]` (top→bottom) and is
**rewritten to `[3,2,1,0]` at frame 60** to prove the screen is genuinely data-driven — the rewrite is
reflected in both the dispatch trace and the rendered pixels.

## The RTS-dispatch mechanism (the actual trick)
On 6502, **`JSR` pushes `(return_addr − 1)`** (hi then lo) and **`RTS` pulls two bytes and adds 1** into PC.
So if you **push `(target − 1)` yourself** and execute `RTS`, you land on `target` — a computed jump in
**6 cycles**, no register clobber, no comparison ladder.

Per frame the dispatcher (clean-room reimplementation of vitoco's idea):

```
        lda #>(EndKernel-1)     ; 1) push the FINAL landing (last RTS returns here)
        pha
        lda #<(EndKernel-1)
        pha
        ldx #NLIST-1            ; 2) push each zone's (addr-1), list walked in REVERSE
PushLoop:
        lda zonelist,x         ;    zone ID
        asl                    ;    *2 -> word index into ZoneTbl
        tay
        lda ZoneTbl+1,y        ;    hi of (routine-1)
        pha
        lda ZoneTbl,y          ;    lo of (routine-1)
        pha
        dex
        bpl PushLoop
        rts                    ; 3) launch: RTS jumps to list[0]
```

Because the list is pushed **in reverse**, the stack pops **front-to-back**: `RTS` → `list[0]`; that zone's
trailing `RTS` → `list[1]`; … `list[NLIST-1]`'s `RTS` → `EndKernel`. Every zone-to-zone transition is one
`RTS` = constant ~6 cy regardless of which zone runs next. `ZoneTbl` stores `routine − 1` so the `RTS` `+1`
lands exactly on the routine. **Cost:** 2 stack bytes per zone (here 4 zones = 8 bytes) + 2 for the terminator.

The −1 is a 16-bit subtraction. A routine starting at `$xx00` needs `$(xx−1)FF` in its entry; take
one from the low byte alone and that routine is sent a page astray. Rob Mundschau: *"If the Lo byte is
$00, then you must subtract 1 from the Hi byte as the borrow for the subtraction"* 〔stella-list
`200301/msg00031`〕. The demo writes `.word ZoneSolid-1` and `#>(EndKernel-1)`, one expression each, so
the assembler takes the borrow; a table built at run time, or `#<` and `#>` halves taken of different
expressions, has to take it itself (our reading, not assembled). **Cited only, not verified**.

## How total lines stay 262 (the hard part)
Variable zones could make the frame breathe; this demo nails it three ways:
1. **Fixed zone count & height.** The list is always `NLIST = 4` entries of `ZONE_H = 40` lines → visible
   zone area is always `4×40 = 160` lines whatever the IDs are. Different IDs change *appearance*, never *line
   count*.
2. **Constant-time frame logic.** The per-frame list rewrite runs inside **one dedicated `WSYNC`-bounded
   logic line** (`sta WSYNC` reserved every frame), so the swap frame costs exactly the same scanlines as any
   other — no ±1 jitter.
3. **Fixed budget = VSYNC 3 + VBLANK(logic 1 + 33) + zones 160 + EndKernel 63 = 262.** Verified by the
   scenario's `ntsc_frame_lines: 262`, including the swap-crossing run.

(vitoco's original keeps the arena height constant with **elastic blank spacer zones** — shrink an enemy
zone, grow a blank zone — so sprite vertical motion doesn't change the line total. This demo uses the simpler
fixed-height form; the elastic-spacer form is a documented extension.)

## CI / how the harness verifies it
`roms/techniques/scenarios/rts_dispatch.json` (golden + 9 asserts, exit 0):
- **Dispatch order trace.** Each zone writes its ID into `evid[slot]` (`$94..$97`) as it runs. Frame 0 asserts
  `evid == [0,1,2,3]` → the RTS chain executed every zone, in the list order.
- **RAM rewrite reflected.** Frame 57 (after the frame-60 swap) asserts `evid == [3,2,1,0]` → rewriting the
  zone-list RAM changes which zones render and in what order.
- **Distinct pixels locked.** `golden_frame: true` hashes the rendered frame chain; the four zones above were
  cross-checked numerically with `read_row` (distinct color/PF per zone range) and visually with
  `get_screen_annotated` (4 visually distinct horizontal bands — not blank).
- **Constant frame.** `ntsc_frame_lines: 262`.

## Verified facts
- **`JSR` pushes `addr−1`, `RTS` pops+`+1`** ⇒ pushing `(target−1)` + `RTS` = a 6-cycle computed jump
  (table stores `routine−1`).
- **Constant dispatch cost:** one `RTS` per zone transition, independent of zone type — no branch ladder.
- **Constant frame:** fixed `NLIST×ZONE_H` zone area + a `WSYNC`-bounded logic line keep the frame at **262**
  even when the RAM list is rewritten mid-run (verified across the swap frame).
- **Cost:** 2 bytes of stack per zone in the list (+2 for the terminator).

## Neighbours — the same stack for other jobs, and the list as something else

- **`RTS` or `JMP (ptr)`: the price is RAM, not ROM.** Rob Mundschau's single dispatch is `LDA
  TableHi,X / PHA / LDA TableLo,X / PHA / RTS`, tables holding address−1; he wrote that it is 2 bytes
  of ROM smaller than storing a vector and `JMP (Vector)`, and one cycle slower 〔stella-list
  `200301/msg00031`〕. By the opcode table (zero-page vector) the `RTS` form is 9 bytes and 20 cycles
  against 13 bytes and 19 cycles — 4 bytes, not 2 (**Not verified** — a hand count, not assembled).
  Thomas Jentzsch named the real difference: *"The little advantage of the JMP() version is, that it
  doesn't need extra stack RAM. Instead you can re(!)use two other currently unused bytes (e.g. a
  pointer)"* 〔`msg00039`〕. The stack bytes are claimed at that moment; the vector can be two bytes
  that mean something else outside the kernel. In this page's chain every zone's address sits on the
  stack for the whole kernel (the **Cost** line above). **Cited only, not verified**.
- **The same pair five years earlier, with the doubling counted.** Andrew Davie, 1998, from a `lda
  value / asl a / tax` start: Piero Cavina's `JMP (temp)` form, *"16 bytes and 26 cycles"*; the `PHA /
  PHA / RTS` form, *"13 bytes and 27 cycles - saves 3 bytes on the earlier implementation, at the cost
  of a single cycle"*; and with `value` already doubled, *"11 bytes, 23 cycles"* 〔stella-list
  `199805/msg00052`〕. Three minutes later he got the 11/23 form without the doubling by splitting the
  table into low and high byte tables 〔`msg00053`〕, which Bob Colbert was about to post 〔`msg00058`〕.
  By the opcode table (zero-page `value` and `temp`, no page crossed) the first form is 17 bytes, so
  the saving is 4, as in Mundschau's pair above; his other figures match (**Not verified** — a hand
  count, not assembled). His posted code pushes the low byte first, and he warned *"memory fails me as
  to if the high byte or low byte should be pushed onto the stack first"*: high goes first, the order
  this page's dispatcher uses and its scenario's dispatch trace depends on. His tables hold no −1 either.
  **Cited only, not verified**.
- **Four answers to one question, 2012.** Asked for an address table, an AtariAge thread gave four
  (read from our distilled notes; the thread text was not kept) 〔AtariAge `topic/198867`〕: tokumaru's word
  table, index doubled, copied into a zero-page `Pointer` for `JMP (Pointer)` (two bytes of RAM); Joe
  Musashi's `PHA / PHA / RTS` from separate low and high tables of `address−1` (no pointer, up to 256
  entries); omegamatrix's table of whole `JMP` instructions — `.byte $4C` and a `.word` per entry —
  entered by `JMP (indirectAddr)` with the index tripled and the vector's high byte set beforehand, 19
  cycles, 14 when the index is already a multiple of three; and omegamatrix's `(indirect),Y` pointers,
  which pick DATA for one piece of code rather than which code runs — a difference the thread itself
  blurred. Both cycle figures agree with the opcode
  table once the `JMP` inside the table is counted (our count; **Not verified**). **Cited only, not
  verified**.
- **Half the table: low bytes only.** Greg Troutman, 1997, answering a compare-and-branch lookup: *"I
  *usually* try and build a table only with the low bytes, and put all the target addresses into the
  same page of memory"* — `ldx variable / lda jmpTable,x / sta jmpWord`, the high byte written to
  `jmpWord+1` — it *"might need to be loaded only once when program inits, unless you are recycling this memory with other
  routines"*, then `jmp (jmpWord)` 〔stella-list `199709/msg00368`〕. Seven hours later he gave the same shape
  for data pointers, `.byte` instead of `.word`, with an `ALIGN` to force the page *"if you have enough
  ROM available"* 〔`msg00374`〕. In the RTS form the high byte would be pushed as a constant, and a
  routine at the first byte of that page would need the previous page's high byte for its −1 (our
  reading). **Cited only, not verified**.
- **A vector that is set once.** Christopher Tumber, 2003, answering a beginner's book that called
  indirect `JMP` useless, listed its uses: an enemy-AI routine picked by a random number through
  `LSB`/`MSB` tables; one vector per player, computer or human, set at game start (`casebook.md` has
  that one); a level-drawing vector instead of *"a bunch of CMPs"*; and scheduling during vertical
  blank or overscan — *"Check if there's enough time left … if there is call the next routine from a
  list"*, carrying on down the list the next time. His condition: *"if you're not hurting for RAM many
  sequences of related CMP branches can be replaced with this kind of branching, particularly if the
  value being tested does not change often so you can setup the JMP vector once and leave it be"*
  〔stella-list `200305/msg00012`〕. Set once, the four-instruction build leaves the hot path and each
  dispatch is the `JMP (vector)` alone, 5 cycles by the opcode table; the `RTS` form has no such split,
  because the jump consumes the address it pulls (our reading; **Not verified**). **Cited only, not
  verified**.
- **A list that keeps its place across frames.** Four months before Tumber's 2003-05 post, Manuel Polik (`cybergoth`, the Manuel Rotschkar of
  the *Jumpman* item below), 2003-01, over-running vertical blank in *Star Fire*, wanted to respawn *"only when there is still enough time
  left that particular frame"* behind `LDA INTIM / CMP #$05 ; more than 6*64 cycles left?`, and asked
  whether that would work on a real console. Thomas Jentzsch: *"Yes it works. I'm doing this in Thrust
  and recently found out that Vanguard has those "emergency exits" too"* 〔stella-list `200301/msg00038`,
  `msg00040`〕. Christopher Tumber proposed a table of low-priority routines whose index *"would only be
  reset when the end of the table is reached. So calling the routines could be spread out over several
  frames"*, against a check at the head of each routine, where the first routines might run every frame
  while the last wait several 〔`msg00048`〕. Polik answered *"This is a really excellent idea!"* but
  kept his own scheme for *Star Fire*, where *"A great deal of routines I do are really required every
  frame"* 〔`msg00051`〕. Replying to that sentence the same day, Tumber put the list on the stack itself,
  each next routine called by an `RTS` and the last one resetting SP to the top of the list
  〔`msg00062`〕. Both posted guards skip the work when `INTIM` reads 5 or more and run it below that
  (our reading): `CMP` sets carry when `INTIM` is 5 or more, so Polik's `BCC Continue` falls through to
  `RTS` and Tumber's `BCS end_VBLANK` leaves the table — the reverse of their comment, and a timer that
  has already run out and wrapped to `$FF` also reads as plenty; Tumber had written his *"off the top of
  my head, may be buggy as hell"*. With `TIM64T`, an `INTIM` of 5 promises only a little over 4×64
  cycles before it reaches 0, not the comment's 6×64 (our reading).
  **Cited only, not verified**.
- **A third table for the bank.** sunpazed, writing a first game, gave each character a state list
  whose states *"are then jumped to via a look up table"*. Thomas Jentzsch uses state lists in his
  Elite demo; his dispatch reads three parallel tables — `TaskPtrTblLo,y` and `TaskPtrTblHi,y` into a
  `jmpVec`, then `ldx TaskBankTbl,y / beq .runTaskInBank / jmp RunTask` — under the comment *"enough
  time left, start current task"*, the last line annotated `;14 = 40`. SplendidNut runs the menu system
  and title-screen sequence of ChaoticGrill on a state machine 〔AtariAge `topic/382726`〕. The 2012
  address-table thread above gives the same low/high/bank tables for the *E.T. Book Cart* 〔AtariAge
  `topic/198867`〕. **Cited only, not verified**.
- **One line per entry for both the ID and the table.** Andrew Davie, 2003, on a vector table whose
  entry names were hand-kept equates that had to follow the table's order: a DASM macro, `TOKEN`, that
  defines `TOKEN_{1} equ TOK`, bumps `TOK`, and emits `.word Animate{1}`, so *"you never need worry
  about the values of tokens, or adding/removing or reordering entries in the vector table"*. His
  catches: entries must be unique, and the target labels must share one format, or the macro takes two
  parameters 〔stella-list `200302/msg00049`〕. This page's `ZoneTbl` and its zone IDs are kept in step
  by hand; for the `RTS` form the emitted word would be `Animate{1}-1` (our reading, not assembled).
  **Cited only, not verified**.
- **A variable delay, entered by `JMP (ptr)` and left by `RTS`.** shazz, timing a 48-px sprite into
  place, pushes the continuation and jumps into a run of `nop`s: `STA WSYNC / LDA #>ScanLineLoop / PHA /
  LDA #<ScanLineLoop-1 / PHA / JMP (DelayRoutine)`; the run ends in `RTS`, which lands on
  `ScanLineLoop` 〔AtariAge `topic/215637`, 2013〕. Where `DelayRoutine` points into the run is the
  number of cycles spent — 2 per `nop`, so he keeps separate even and odd tables. The push-(target−1)
  -then-`RTS` above, used once to come back rather than to chain zones; `SLEEP` elsewhere in this
  repository is a delay fixed at assembly time. **Cited only, not verified**.
- **Rewrite the jump instead of stacking it.** Glenn Saunders, 2004, to Eric Ball about a Lode
  Runner-style game, as a way to *"fully exploit the SC RAM"*: *"You can basically copy the kernel code
  to RAM (or have a bunch of different kernel modes and overwrite JMP addresses to script out which to
  run and when) on the fly between frames. Assuming there is enough CPU time inbetween frames, that
  would allow for as much flexibility as you can do in a static title screen since nothing really has
  to be evaluated inside the kernel anymore"* 〔stella-list `200404/msg00004`〕. Ball had not planned to
  use SC-RAM for code changes — he had found a way to do without the self-modifying code he had tried —
  and was spending it on data, four pages for the playfield background and two for player graphics
  〔`msg00023`〕. Here the chain's targets are data on the stack; in that suggestion the jump's own operand
  is RAM (our reading). The thread has no measurement of the copy against the time between frames.
  **Cited only, not verified**.
- **One kernel, or one per section: which resource is short.** Andrew Davie, 2001, in his Qb
  post-mortem: *"Initially, I recall, I had something like 4 kernels in there... all doing similar, but
  not the same, things. They were all rather massive, and I started to run seriously out of ROM
  space"*; he generalised them into one kernel drawing the target area, the playfield area and the
  sliding cube coming on to the screen, and *"compressing the multiple kernels into a single one meant
  that I 'd saved many hundreds of much-needed ROM bytes"* 〔stella-list `200103/msg00196`〕. Manuel
  Rotschkar's *Jumpman* went the other way, hard-coded kernels per screen section: *"Wastes tons of ROM,
  but I don't want to waste any cycles on conditional logic or JSR/RTS combos at the moment"*
  〔`200409/msg00264`〕. The same trade from its two ends, settled by whichever of ROM and cycles was
  short (our reading; neither of them links the two). **Cited only, not verified**.
  Later in that thread Rotschkar put those kernels into macros: *"I also started packing all those
  kernels into macros, so I can quickly reuse all learned techniques and it also will help a lot when
  it's getting more complex (sprites, missiles), as all code has to be touched only once - per
  macro..."* 〔`200409/msg00266`〕, and *"The kernel macros surely speed up the programming process"*,
  one level taking *"only ~ 80 minute to create"* 〔`200409/msg00270`〕. His progress reports from 19
  September to 24 October 2004 then list *"Kernel macros"* and *"Used ROM thus far"*: 7 macros and 3K
  with 7 levels transferred, 8 and 6K with 15, 9 and 10K with 25 — the last also holding the title,
  demo and secret level structures 〔`200409/msg00273`, `200410/msg00065`, `200410/msg00178`〕. The
  second report adds *"At the moment I'm no longer sure wether the macro approach will help or hurt me
  later... ;-)"* 〔`200410/msg00065`〕. Two days after it, of one level, *"by far the most complicated
  level to transfer so far"*: *"you've seen one of the 9(!) macro calls"* 〔`200410/msg00101`〕 — nine
  calls in that level, which is a different count from the 8 and 9 macros of the reports. What he had
  shown is, by our reading, in his DASM bug-report thread of the day before, where two lines each
  invoke a macro named `CUSTOMPF0PF1PF2JUMPMANKERNEL` with eight arguments (our count) built from
  `FOLLOW_` labels. Of the first he wrote *"I got the impression that DASM can't handle code lines
  that get longer than \*cough\* 255 chars..."* and *"it can't compile this line"*
  〔`200410/msg00086`〕; Thomas Mathys pointed at `#define MAXLINE 256` in DASM's `main.c`, *"don't
  know if this fixes the problem"* 〔`200410/msg00090`〕; and Rotschkar's reply has the second line,
  *"working around this, by pre-calculating some values"* 〔`200410/msg00091`〕. Nobody in that thread
  reports whether raising the limit cured it. No macro body is in the mails read for this entry, and
  the attached sources were not read here.
  **Cited only, not verified**.
- **The list as a timeline.** The zone list here is spatial — one entry per band of the screen. In a
  2025 thread on growing stalactites drawn in the playfield, two replies made the list temporal.
  SplendidNut: treat *"each PF byte as its own unique stack of bitmap changes"*, each entry a pattern
  and *"the countdown to the next change"* (16 bytes per PF column, 64 for a 32-pixel display).
  Bit Expander: about 32 bytes, *"each one being an opcode to be executed by the kernel"* — 3 bits of
  blocks to wait, 5 bits naming the PF bit to clear 〔AtariAge `topic/385470`〕. Thomas Jentzsch, who
  asked, used neither: he stored the plain playfield, *"32 (columns) / 8 * 16 (rows) = 64 bytes"*.
  **Cited only, not verified**.
- **Zone seams show in the picture.** Each zone sets its own colours, and anything drawn across a zone
  boundary shows the change. IanAjax's Impossible Mission mock-up (2026) staggers the side walls so
  platform zones and object zones alternate, and because every zone is the same height the colour
  steps read as *"a bit more like some design on the wall"*. Objects that use the ball also have a
  playfield version, so one of them can share a level with another ball object 〔AtariAge
  `topic/388301`〕. A mock-up, not a kernel; two days later the walls became a gradient
  pattern that changes the playfield colours only 12 times, *"12 zones to switch between"*.
  **Cited only, not verified**.

## See also
- `zone-multiplexing.md` — the **static** form (fixed zones); this is its data-driven generalization.
- `dynamic-multisprite.md` — sort/position/display + flicker (the general multi-sprite kernel).
- `roadmap.md` — full candidate list (this is candidate ⑱).

## References
- vitoco, *Tips and tricks for a modular kernel* — AtariAge topic **313777** (2020).
  Distilled notes: `reference/atariage/313777-modular-kernel/notes.ja.md`.
- 6502 `JSR`/`RTS` stack semantics (6502.org): `JSR` pushes return−1; `RTS` pulls and increments.
