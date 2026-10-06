# Technique — 48px bitmap zone with window scrolling

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** a logo / picture / message band of arbitrary height drawn with the verified 48px
6-store choreography, plus a **window** into a taller bitmap so the band can scroll vertically
(or jump between frames of a larger image).

Demo: `roms/techniques/bitmap48.asm` (a 48×24 emblem inside a 48×48 bitmap, window bouncing).
CI: `scenarios/bitmap48.json` (window offset animation incl. the bounce, positions, 262, golden).
Lineage: RevEng's Bitmap Minikernel (AtariAge topic/168603) — "point the six score pointers at
bitmap column slices"; window indexing is what turns it into scrolling text/menus/room names.

## The technique
- **Bitmap = six column tables** (one per 8px column of the 48px band), stored bottom-row-first
  within one ROM page (fixed pointer high bytes, no page-cross penalty — store timing stays
  deterministic).
- **Window**: the six zero-page pointers are `ColK + offset` recomputed per frame; the kernel
  shows `WINDOW` rows starting there. Scrolling is just `offset ± 1` (here every 2nd frame,
  bouncing across `BMH-WINDOW`).
- **Kernel** = the score6/text12 choreography verbatim, one row per scanline.

Together with `score-kernel.md` (digits) and `text12.md` (text) this completes the 48px family:
**same verified choreography, three data feeds** (font-by-value, packed text buffer, bitmap
columns with a window).

## Verified
Window offset animates and bounces exactly as asserted (7@f10 → 22@f40 → direction flip →
4@f100), positions 87/95, 262 lines every frame, golden-pinned.

## Wider than 48 — what it was said to cost

`sprite-placement.md` carries spiceware's claim that the missiles and the ball can widen a 48-pixel
display. A tool that did it was posted in 2006: *"a 52-pixel wide sprite. It works by combining the
48 pixel sprite with the missiles and the ball. One of the missiles becomes a 2-pixel sprite with a
combination of HMM0, NUSIZ0, and ENAM0. The larger image takes a lot of ROM space, and is not as
movable as the 48 sprite. I'll leave it to you to weigh the costs and benefits."* (Zach M,
〔stella-list `200603/msg00001`〕). He does not break the 52 down or put a number on the ROM, and the
picture uses up all three remaining objects. The tool and its output were not looked at here —
Cited only, not verified.

## What else the six pointers can carry

The kernel reads wherever the pointers point; the frame code decides the picture. Two uses from
the list and the forum, none built here — Cited only, not verified.

- **Game state as the offset.** From the thread that took *Defender II* apart: a RAM vector draws the
  mountains and the starfield, and another byte, when non-zero, *"alters the LSB of this vector,
  counting upward to `$1C`"*, so *"`$A9 = 0` for planet OK, `$A9 < $1C` currently being destroyed,
  `$A9 = $1C` fully destroyed"* (AtariAge `topic/289892`; quoted from our notes, the posts themselves
  are not on disk). That vector is not a 48 px band; our reading is that this page's window offset
  could hold such a state the same way, with the kernel unchanged.
- **Columns shared between animation frames.** For a 48 px fighter, Eckhard Stolberg: *"since some of
  the collumns would look the same in different animation frames, maybe we should store the data
  column wise and use a table, that tells us which collumn to use for which animation frame"*
  〔stella-list `199804/msg00009`〕, and on being misread, *"With collumns I meant the 6 bytes, which
  make up the sprite"* — set the pointers from that table *"at the beginning of a displayed frame"*
  (`msg00014`). Erik Mooney: the colour "column" can be shared too, and *"there'd be no need to keep one
  frame's data within a page"* (`msg00015`). Asked whether pointers into different pages cost cycles
  *"As long as we never index across a page boundary"*, Stolberg: *"That's right"*, the columns of a
  set of frames having to sit in one 2K bank *"since we can't switch the bank during display"*
  (`msg00022`). Our reading: for timing, this page's one-page rule is stronger than needed — what
  costs a cycle is an `(ind),Y` read crossing a page, measured by `TestPageCrossPenaltyRules`.

## Row height from a table

This changes the kernel's row index, not the pointers. Answering Andrew Davie's double-height
six-digit score, Thomas Jentzsch: *"You could also use a table to create displays of any size you
want"* — `ldx count1` / `ldy table,x` with `table .byte 0,0,0,1,1,1,2,...` for triple height — *"But
this will cost you some extra ROM too."* 〔stella-list `200102/msg00058`〕. The next day, a different
height per row: `.byte 0,0,0,1,1,2,3...`, *"result might look funny :)"* (`msg00066`). Not tried on
this page's loop — Cited only, not verified.

## Cycles left over

Eckhard Stolberg, posting a movable 48-pixel demo: *"I used the standard vdel method in ROM which has
6 cycles to spare. If it were put in RAM like Robin's version, it would have 14 cycles left."*
〔stella-list `199803/msg00201`〕. His loop is not this page's; the loop this page shares with
`score-kernel.md` was measured at 4 cycles left there (2026-09-07). For a double-scanline version of
the big sprite, Stolberg: *"there are 28 cycles left to spare for both lines together, although you
only get 22 of them in a row"* (`199804/msg00014`). Erik Mooney, fitting a branchless 14-cycle ball
and the sprite colours into it: *"the PLA can go in the six separate, and the other 22 are filled"*
(`msg00023`); he added that the ball code *"should be doing a CLC before the SBC, but there just
aren't the cycles to do it"* (`msg00026`). Cited only, not verified.

## Is it 48 pixels?

Thomas Jentzsch on the *Thrust* score routine, which parks a byte in the stack pointer (`TXS`/`TSX`):
*"The result is actually only a \*47\* pixel routine, because the right pixel of the first and the third
digit (if i remember it correct) are always the same. That works for me, the score display has blank
pixels there, and THRUST too (except for the long T-line)."* 〔stella-list `200008/msg00024`〕. Of
another score routine: *"If you try use bit 0 in your data, you will see, that the last column of first
digit displays the data of the third digit. If you want real 48 pixels you have to use the VDELPx
version."* (`200102/msg00066`). Both Cited only, not verified; `score-kernel.md` has related accounts
from Stolberg and Harbron of six-store routines without VDEL. Our reading of the *Thrust* loop in
`msg00024` (last stores at cycles 43/46/51/54, ending `sty GRP0`) is that it has the VDEL shape too.
`litmus_48px6` cannot show this error: its test row's first and third bytes are `$AA` and `$F0`, both
with bit 0 clear. Whether this page's kernel has it is Not verified.

## Moving the band

This band is parked. Moving a 48 px band broke `multicolor48`'s picture (`capability-gap-audit.md`);
HMOVE's 8 px blank is in `sprite-placement.md`. On the list: *"the movable 6char trick, which was
used for the dragons in Dragonfire & the cars in Dragster"*, recreated by Eckhard Stolberg in *'Move
This'* and *'Kung Fu Sprites'* and reused in Rob Kudla's *Boing* (Manuel Polik, 〔stella-list
`200209/msg00062`〕). Kudla treated the Move This code *"almost as a black box"*: *"I actually don't
understand the flow of the 6-character sprite kernels, to the extent of Boing actually only having a
5-character sprite worth of data. I just wrote a blank sprite to the sixth copy because when I stopped
repositioning it weird visual artifacts showed up"* (`msg00065`). Stolberg in the same thread: *"There
is so much overhead that you can't start the positioning routine directly after a WSYNC"*; *"go through
the actual display without using any WSYNCs during those lines"*, and in his first karate demo a
background colour change *"would always happen in the middle of a scanline, depending on where the
fighter was positioned"* (`msg00074`). Stolberg in 1998, of the big-sprite kernel: because *"the
players are moveable, the writes to PFx or ENABL would occur at different cycles in the scanline"*, so
*"graphics left of the player would be affected one line earlier than graphics right of the player"*
〔stella-list `199804/msg00022`〕. The Stella FAQ posted to the list in 2002: *"When moving these
objects, a very precise time-wasting kernel is employed to adjust the positions and rewrite-time of
the kernel. This severely limits any sort of playfield manipulations."* (〔stella-list
`200210/msg00268`〕, of *"the drag racers in Dragster"*). Cited only, not verified.
