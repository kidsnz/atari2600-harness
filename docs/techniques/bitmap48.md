# Technique — 48px bitmap zone with window scrolling

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
