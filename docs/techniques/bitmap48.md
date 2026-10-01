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
