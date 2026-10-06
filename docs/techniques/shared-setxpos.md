# Technique — `shared_setxpos`: position all 5 objects with one shared routine

**Source:** AtariAge topic 115690 *New way for repositioning all 5 objects in 1 shared code* (`reference/atariage/115690-*/notes.ja.md`; technique candidate ㉒).

**Goal:** Place all five movable TIA objects (P0, P1, M0, M1, BL) at arbitrary X with a
**single shared code path** instead of five hand-written positioning routines. The standard
initial-placement idiom of real 2600 games (AtariAge #115690, LS_Dracon / robert-m, 2007).

**Demo:** `roms/techniques/shared_setxpos.asm` — five color-coded objects at distinct X
(P0=20, P1=55, M0=84, M1=109, BL=139), all positioned by one `SetXPos` indexed by object
number. CI-locked by `scenarios/shared_setxpos.json` + `.golden`.

## Why one routine can serve all five

The TIA register file is laid out so the relevant registers are **consecutive**:

| obj # | strobe (RESxx) | motion (HMxx) |
|---|---|---|
| 0 P0 | `$10` | `$20` |
| 1 P1 | `$11` | `$21` |
| 2 M0 | `$12` | `$22` |
| 3 M1 | `$13` | `$23` |
| 4 BL | `$14` | `$24` |

So with the object number in `X`, `RESP0,x` selects the right strobe and `HMP0,x` the right
motion register — **one code path positions any object**. (robert-m's correction in the thread:
the X register's true role is *object selection*, not a cycle-filler.)

## The shared loop

```asm
; X coords in RAM, laid out in RESxx order: P0,P1,M0,M1,BL @ $80..$84
PositionAllObjects:
        ldx #4              ; BL down to P0
PosLoop:
        lda xpos,x          ; this object's intended X
        jsr SetXPos         ; the ONE shared routine
        dex
        bpl PosLoop
        sta WSYNC
        sta HMOVE           ; apply all 5 fine adjustments at once
        rts

SetXPos:                    ; A = target X, X = object number (0..4)
        sec
        sta WSYNC           ; fresh line => deterministic coarse timing
WaitObj:
        sbc #15             ; 2  div15 coarse step (15px granularity)
        bcs WaitObj         ; 2/3 loop until borrow = coarse position
        eor #7              ; 2  remainder -> HMOVE nibble correction
        asl                 ; 2 \ shift low nibble to high (HMxx uses upper nibble)
        asl                 ; 2  |
        asl                 ; 2  |
        asl                 ; 2 /
        sta HMP0,x          ; 4  stage fine adjustment for THIS object (indexed)
        sta.w RESP0,x       ; 5  strobe RESxx (coarse). .w forces 5cy fixed timing
        rts
```

**Design rule:** lay the X coordinates in RAM at **consecutive addresses in RESxx order**
(P0,P1,M0,M1,BL). Then a `DEX/BPL` loop + one trailing `WSYNC`+`HMOVE` positions all five.
Each `SetXPos` does its own `WSYNC`, so it costs **one scanline per object, or two when the next
`WSYNC` misses the line**, plus the final HMOVE line. In this loop an input of 90 or more pushes the
next call's `WSYNC` onto the following line, so the demo's five objects take 7 lines, not 5 (measured
below). Two is the most: at input 255 the next `WSYNC` starts at 5q+46 = 131 (q = 17, the count
below), inside the second line. Measured on copies of the demo with BL's input changed (2026-10-01,
`cmd/scenario` `ntsc_frame_lines`): 89 gives 261 lines (BL's extra line gone, M1's left), 90 and 255
give 262, as 140 does; M1 at 80 and BL at 75 give 260. The last object, P0, is followed by
`rts`/`dex`/`bpl`/`sta WSYNC` rather than another call, so it keeps to one line up to input 134: with
M1 at 80 and BL at 75, P0 at 134 gives 260 lines and P0 at 135 gives 261 (same method).

**The one-line cost holds only while the next `WSYNC` is reached in time.** The wait grows by 5
cycles per 15 px, and `SetXPos` returns right after its strobe, so it is the *next* call's `WSYNC`
that must still land on the same line. RMundschau's 2004 routine, which ends with its own `WSYNC`,
states the contract in its header: *"If control comes on or before cycle 73 then 1 scanline is
consumed. If control comes after cycle 73 then 2 scanlines are consumed"* and *"control is returned
on cycle 6 of the next scanline"* 〔`200403/msg00260`〕. Counted with the cycle table
(`Gopher2600/hardware/cpu/instructions/definitions.json`: `sec`/`sbc #`/`eor #`/`asl`/`dex` 2,
`bcs`/`bpl` taken 3, `sta zp,x`/`lda zp,x` 4, `sta abs,x` 5, `jsr`/`rts` 6), the strobe ends at cycle
5q+23 after the `WSYNC` (q = ⌊input/15⌋, the value passed in A) and the next call's `sta WSYNC`
starts at 5q+46 — past 73 from q = 6, input ≥ 90, where that object costs two lines. Measured on the
demo with `trace_clocks` (2026-09-30; the `.asm` passes inputs 20/55/85/110/140, which land at the
`hmoved_pixel` 20/55/84/109/139 above): the next `sta WSYNC` starts at cycle 15 of the following line
after BL (input 140) and at cycle 5 of the following line after M1 (input 110), but at cycle 71 after
M0 (input 85) and 61 after P1 (input 55). The frame is 3 VSYNC + the line the loop is called from + 7
+ 29 VBLANK (the first of them the HMOVE line) + 192 + 30 = 262, matching the scenario — the loop
takes 7 lines for 5 objects, not 5. `cmd/framegen` measured the same effect on its copy of
`SetXPos` — *"Combat (P1 at clock 145, input 139) spends one line more than Outlaw does"* — and
calibrates the frame length for it. Shrek met the inline form in 2004: with code added to the
positioning lines, *"if a sprite was too far on the right, it didn't fit and dropped on the next
line"*; he moved the code, and it came back when a monster walked past its rightmost point,
*"prolonging the positioning line and throwing the following few instructions to the next line, thus
shifting the platform one pixel down"* 〔`200410/msg00142`, `200410/msg00144`〕. **Cited only, not
verified.**

**A table instead of the division, as priced in 2001.** Manuel Polik's Gunfight 2600 positioned from
a table, `HorzTable` (161 bytes as posted), each byte holding the HMxx value in its upper nibble and
the coarse wait count in its lower (`lda HorzTable,X` / `sta HMP0,Y` / `and #$0F` / `tax` /
`dex`-`bpl` / `sta RESP0,Y`). In 2001 Andrew Davie pointed him at doing it *"with a routine instead
of a table (MUCHO bytes savings... about 128 bytes, approx)"*, naming Qb's `PositionSprites`.
Polik's answer: *"Check the number of WSYNCS in both routines. You see the difference? :-)"* and
*"I've mid-screen repositioning in mind, so saving a whole scanline might make a BIG difference in
finding the right spot for a repositioning - well worth 128 bytes"* 〔`200102/msg00152`,
`200102/msg00163`〕. Neither routine's line count is counted here; `divtable.md` (*The coarse
position is time*) has Bob Colbert's 1997 remark that precalculating into a table saves a line.
**Cited only, not verified.**

**Why `ldx #4`, not `#5`.** The five objects are indices 0..4. A sixth pass writes the next
register in each run instead: `RESP0+5` = `$15` = `AUDC0` and `HMP0+5` = `$25` = `VDELP0`
(`internal/beamtrace/beamtrace.go`) — an audio register and P0's vertical delay, not a position. It
has happened on AtariAge at least twice: `ldx #5` in Nosehair (karl-g: *"when the X register is 5,
you are stomping on AUDC0"* 〔AtariAge `topic/312285`〕), and a `cpx #$06` up-count whose sixth pass, per
Omegamatrix, *"ultimately writes to VDELP0 and AUDC0!!"* 〔AtariAge `topic/255357`〕. The second case adds the
other half of the design rule: that loop read `PlayerX0,x`, so the RAM *declaration order* decides
which variable positions which object, and the sixth byte it read was `PlayerY0S`, a variable that
had simply been declared next. **Cited only, not verified.**

## div15 math
- `sbc #15` loop = the known divide-by-15 coarse step (15px granularity).
  Why 15: one taken pass of `sbc #15`/`bcs` is 2 + 3 = 5 CPU cycles = 15 colour clocks, the time the
  beam takes to cross 15 pixels, so the same loop divides X and waits out the distance. RMundschau:
  *"our timing loop takes 5 cycles * 3 color clocks = 15 pixels. SBC #15 takes 2 cycles the same as
  DEY or DEX"* 〔`200403/msg00260`〕; SpiceWare's `PosObject` says it in a comment: *"each time thru
  this loop takes 5 cycles, which is the same amount of time it takes to draw 15 pixels"* 〔AtariAge
  `topic/342613`〕.
- The leftover remainder is mapped to the HMOVE nibble via `eor #7` then `asl`×4 (HMxx uses the
  upper nibble; two's-complement, positive = left).
- The nibbles this gives run from +6 (left 6) to −8 (right 8), not ±7: after the loop A is −15..−1,
  its low nibble 1..15, and `eor #7` maps that to 6..0 then −1..−8. RMundschau's table, and the
  table-free form he gives beside it (`eor #%00000111` / `adc #1` / `asl`×4; the carry is clear after
  the loop, so `adc #1` adds exactly 1), give +7..−7 〔`200403/msg00260`〕. Both are 15 consecutive
  steps, so either reaches every pixel once calibrated (`road.md`: *"Calibrate, don't copy"*); they
  differ by a one-pixel offset: both of RMundschau's forms use left 7 (his table's first entry is
  *"Left 7"*), and only `eor #7` alone uses right 8. **Not verified** in the emulator — arithmetic over
  the 15 remainders.
- HMOVE fires **once at the end**, applying every object's pre-latched HMxx simultaneously.
- `sta.w RESP0,x` forces the 16-bit absolute form = 5 fixed cycles, stabilizing the strobe.
  The 4-cycle zero-page `sta RESP0,x` is not variable either (the cycle table above); what the extra
  cycle changes is when the strobe lands, because a store reaches the TIA when the instruction
  completes, so `.w` puts the `RESxx` write one CPU cycle (3 colour clocks) later. That was the
  list's answer in 2001 when Andrew Davie needed exactly one more cycle before a write: Kurt Woloch
  suggested `sta.w GRP0` (*"if that works"*), and Davie, asked by Chris Wilkson whether GRP timing
  depends on when the instruction starts or finishes, replied *"When the instruction completes,
  unfortunately"* and *"You don't need to use the mirror, just sta.w GRP0 will do the job"*
  〔`200102/msg00154`, `200102/msg00156`, `200102/msg00159`〕 (also `restrobe-copies.md`,
  `kernel-micro-idioms.md`). **Cited only, not verified.**
- That depends on the assembler keeping the index. In 2017 two DASM builds both numbered 2.20.11
  disagreed: kylearan's (20140304) assembled `sta.w RESP0,x` to `STA.wx RESP0,x`, Thomas Jentzsch's
  (*"DASM 2.20.11 unofficial RevEng 20140124"*) to `STA.w RESP0` — the `,x` dropped without a word,
  so the strobe always hit `RESP0` and *"the ball and P1 are moving horizontally between frames"*.
  kylearan's suggestion for that build was to write `sta.wx RESP0,x` 〔AtariAge `topic/264527`〕. The
  opcodes differ (`$9D` `sta abs,x`, 5 cycles, against `$8D` `sta abs`, 4). Nothing here reads the
  opcode, and CI installs whatever `dasm` apt provides (`.github/workflows/ci.yml`); but a dropped
  `,x` would send every strobe to `RESP0`, so the demo's four `hmoved_pixel` asserts for P1, M0, M1
  and BL should catch it indirectly (inference, not tried; the local `.bin` has `$9D`).
  **Cited only, not verified** for the DASM builds.
- A load can buy the cycle instead of the store. RMundschau's `PosObject` makes its table read,
  `lda fineAdjustTable,Y`, always cross a page: the table is ORGed at `$F000`, the label sits 241
  bytes (`%11110001`) below it, and `Y` holds the loop's leftover `$F1`..`$FF`, so every effective
  address is on the page after the base's. His comments: *"Consume 5 cycles by guaranteeing we cross
  a page boundary"*, *"In your own code you may wish to consume only 4"*, and the table is at the
  top of a page *"to guarantee the processor will cross a page boundary and waste a cycle I need to
  waste in order to be at the precise position I want the RESP0,X to happen at"*
  〔`200403/msg00260`〕. That is one crossing per call, outside the wait loop, not the per-pass
  crossing inside it that `hmove-two-step.md` warns against. It is the same +1 Chris Wilkson
  suggested to Andrew Davie in 2001, *"Can you align the code so the the LDA instruction crosses a
  page boundary? That'll give you an extra cycle"* 〔`200102/msg00154`〕 (our reading; Davie took it
  as a branch crossing, *"utilising the extra cycle when branching over a page boundary is certainly
  something worth looking at"*, and noted that his branch back to the top would *"take an additional
  cycle, too"* 〔`200102/msg00159`〕). Eric Ball's table under *Variants* uses the same 241-byte
  offset the other way, ORGed at `$xxF1` so the index never leaves the page. **Cited only, not
  verified.**

## Variants

- **Moving by a difference instead of placing at X** 〔AtariAge `topic/212690`, svolli〕: `sec` /
  `lda oldpos` / `sbc newpos` / `asl`×4 / `sta HMxx`. old − new is positive for a move to the left,
  the sign HMxx uses, so the shifted difference is the nibble — no table, no re-strobe. It does not
  handle the wrap at the screen edge (old 2 → new 158 wants left 4), and one HMOVE reaches only +7
  left .. −8 right (`known-traps.md`, *HMOVE range*), so a longer move is split across frames.
  Outlaw gets the sign without the subtraction, by Manuel Polik's reading of David Crane's code
  (*"I think I know the answer now"*): it counts horizontal coordinates *"left to right from 160 to
  zero"*, so a bullet moving 4 pixels right adds `$FC` (−4) to its coordinate, and the four `asl`
  before `sta HMM0,X` give, per his comment, `$C0`, right 4 (in the posted fragment the shifts act on
  the sum after `adc`, not on `$FC` itself) — *"I think that's how horizontal movement
  was _supposed_ to work"* 〔`200110/msg00139`〕.
  **Cited only, not verified.**
- **Two players 8 px apart from one X, on one line** 〔AtariAge `topic/119524`, cybergoth's
  Seawolf code as quoted there〕: after the `sbc #$0F`/`bcs` wait, `and #$0F`/`tax`/`lda hmovetab,x` is P1's nibble
  (a 16-entry table), and `sbc #$0F` on it — the carry is clear after the loop, so this subtracts
  `$10`, one step further right — is P0's; then `sta RESP0`/`sta RESP1` back to back. The strobes land
  9 colour clocks apart and the one-step nibble difference closes that to 8 — the correction
  `sprite-placement.md` (*What rule 3 buys*) takes from two shifted tables, here derived from one. The
  code needs a `clc` ahead of the wait loop; without it the pair shifts one pixel right when it moves.
  Eric Ball posted the same pair of nibbles and back-to-back strobes on the list in 2002 (*"Update
  Manuel's magic with Eric's 2 in 1 magic"*) 〔stella-list `200212/msg00193`〕, as a reply in a thread
  whose first post is not in this corpus. Compared with Manuel Polik's Star Fire version, which
  opens with `sta WSYNC` / `sta HMOVE`, masks the leftover with `and #$0F` and reads a 16-entry
  table 〔`200211/msg00165`〕, his differs in two ways. His 15-entry table is ORGed at `$xxF1` and
  read as `hmovetab-241,Y` with the loop's leftover in `Y`: *"Figured out an additional tweak: if
  you put the hmovetab at the very end of a page, then the AND #$0F isn't required. This saves a
  couple of additional cycles"* (by hand: the base is then the page's first byte and `$F1`..`$FF`
  stays inside the page). And his line opens with `sta WSYNC` alone, keeping only the closing
  `sta HMOVE`: *"I've also removed the STA HMOVE since there isn't any drawing on the line anyway
  and it could cause problems since the STA HMP1 is less than 24 cycles after (although it might be
  necessary for HMOVE bars)"*.
  **Cited only, not verified** — the Seawolf form is taken from the mining notes; that thread's own
  text is not in the corpus.
- **Battlezone's routine** 〔stella-list `200210/msg00281`, Manuel Polik, 2002〕: the same `sbc #$0F`
  / `bcs` wait and `eor #$07` / `asl`×4 as `SetXPos`, but it strobes with the 4-cycle `sta RESP0,X`,
  leaves the nibble in `Y` (`tay`) for the caller to store, and does `sta WSYNC` / `sta HMOVE` both
  before the wait and after the strobe (`divtable.md`, *The coarse position is time*, has the wait
  itself). Ahead of it sits an entry step, `cmp #$11` / `bcs` / `sbc #$04` / `bcs` / `adc #$A5`. By
  hand, inputs of 17 and up pass unchanged, 5..16 enter the wait as 0..11 and 0..4 as 160..164, with
  the carry set on every path (**Not verified**). Polik guessed at its purpose — *"The add/sub stuff
  on entry of the routine seems to fix some troubles with early RESPX"* — without knowing what range
  Battlezone feeds it: *"Just a few guessings"* 〔`200211/msg00017`〕. He said it *"uses half the
  bytes"* of the routine Robert Colbert had explained, that *"you can call this wherever you want,
  even midscreen"*, and *"I think the routine presented could be even tweaked to reposition an
  object in a single scannline - without using any table!"*, leaving open whether it is precise at
  the borders and *"how/if the algorithm works without"* Battlezone's HMOVE lines
  〔`200210/msg00284`〕. Dennis Debro ran it in VBLANK on Climber's main player (*"it works great"*,
  with *"a little wall on the left where the player seems to bounce off"*) and then dropped the
  `tay` in his kernel, where *"it seems to work fine without those 2 cycles"*; Polik's question
  whether that gave all 160 positions has no answer in the thread 〔`200210/msg00285`,
  `200210/msg00286`, `200211/msg00017`〕. In 2004 Thomas Jentzsch: *"Looks like Space Jockey was the
  first game using it"* 〔`200403/msg00262`〕. **Cited only, not verified.**
- **Pre-packed coordinates (the Air-Sea Battle form)** 〔AtariAge `topic/89293`〕: each object's X is
  converted in VBLANK, all objects together, to one byte with the HMxx value in the upper nibble and
  the coarse count in the lower, kept in a RAM array; the positioning line then loads it,
  `sta WSYNC`, `sta HMP1`, `and #$0F` / `tay`, waits in a `dey` / `bpl` loop, strobes `RESP1` and
  ends with `sta WSYNC` / `sta HMOVE`, so the kernel does no division. `HorzTable` above has the
  same byte layout in ROM, indexed by X. On the list, Erik Mooney in 1997, discussing a conversion
  routine, weighed storing only the standard X against storing *"both the standard X and FC_X"* when
  short of RAM 〔`199704/msg00051`〕, and in 2004 said the Air-Sea Battle routine *"at least gets a
  visible understandable byte with the fine/coarse numbers"* 〔`200404/msg00298`〕. Dennis Debro,
  moving Climber from what he called *"the old Air-Sea Battle positioning routine"* to Battlezone's,
  expected *"7 more bytes of RAM"*; the thread does not say what those bytes held
  〔`200210/msg00285`, `200210/msg00286`〕. **Cited only, not verified** — the VBLANK conversion, the
  nibble order and the kernel-line sequence are taken from the mining notes; that thread's own text
  is not in the corpus.

## CI

`go run ./cmd/scenario roms/techniques/scenarios/shared_setxpos.json` (exit 0):
five `tia.<obj>.hmoved_pixel` asserts (one per object), `ntsc_frame_lines:262`,
`golden_frame:true`.

## Verified facts
- One shared `SetXPos` (indexed by object number via the consecutive `RESP0,x`/`HMP0,x` layout)
  positioned all five objects at **distinct intended X**: P0=20, P1=55, M0=84, M1=109, BL=139
  (measured `hmoved_pixel`).
- Players land at X (= 3N−54), missiles/ball at X−... (3N−55); the 1px player offset shows as
  the +1 difference between the player targets and the missile/ball targets that share the same
  coarse math — exactly the documented hardware offset.
- All five are visibly rendered at distinct X with distinct colors (`read_row` at scanline 100:
  P0 yellow @20, P1 red @55, M0 @84, M1 @109, BL cyan @139).
- 262 lines/frame; golden frame hash stable.
