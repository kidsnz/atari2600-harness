# Technique — 24-character text line (50% flicker, two blocks)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** double the text width to 24 characters by alternating two 12-character blocks across
frames — the left 12 on one field, the right 12 on the next, so the eye reads a continuous
24-char line at 50% flicker (CRT/venetian-blind friendly).

Demo: `roms/techniques/text24.asm` ("ABCDEFGHIJKLMNOPQRSTUVWX").
CI: `scenarios/text24.json` (both block positions, packed buffers, 262, golden).
Lineage: za2600 (Zelda port) `text24.asm` — studied from source
(`reference/2600-technique-sources/za2600/`); this is the supercat "two groups" realization
(za2600 interleaves at medium NUSIZ; we use two contiguous 12-blocks at close NUSIZ, simpler and
position-verifiable). Builds directly on `text12.md` (flicker-free 12).

## The technique
- Reuse text12 wholesale: 4×5 font packed 2-chars/byte, column-major zp buffers, the 48px VDEL
  6-store kernel.
- Split the 24-char string into **first 12 (bufE)** and **last 12 (bufO)**.
- **Even frame**: draw bufE at the left block (P0=39). **Odd frame**: draw bufO at the right
  block (P0=87 = left + 48px = exactly 12 char-cells). The per-frame position is set by a
  frame-dependent pre-RESP delay (measured: 39 vs 87).
- Together the two fields span 39→135 = 96px = 24 contiguous characters.

## Verified
Left block P0=39, right block P0=87 (48px apart, contiguous), both legible on the annotated
screen, packed buffers non-zero, 262 lines, golden-pinned.

## Notes / variants
- 50% flicker is unavoidable for 24 on one line without RESP re-strobing (the 32-char route,
  candidate ⑨). On LCDs column/block flicker often looks better than CRTs (supercat).
  **The re-strobe is measured — see [`restrobe-copies.md`](restrobe-copies.md) (technique #36)**:
  how many slots a mid-line strobe adds (**3 + k** per player at 6/7/8 cycles of spacing), where the
  ladder is **flat** (3 and 5 cycles), and where the copies land (off the multiple-of-three grid at
  spacing 8, on it at 6). Read it before concluding that flicker is the only way past 12.
- For genuinely interleaved single characters (za2600's look), switch to NUSIZ medium and offset
  by 8px instead of 48 — same skeleton, different position constants.
- Per-frame color staging works in the gaps for two-color text.
- **Alternating by scanline and by frame at once (Cited only, not verified).** Rob Kudla, 2000, from
  what he saw on screen (*"I haven't looked at any of this code"*): Stellar Track and Dark Mage
  *"appear to simulate a 12-character display"* of 8-pixel players, drawing alternate cells on
  alternate scanlines and swapping them on the next frame (*"crisscrossing every other scanline"*);
  Suicide Mission only alternates columns every frame and *"might appear more flickery as a result"*
  〔stella-list `200001/msg00025`〕. The scanline half is `venetian-blinds.md`'s interleave; Manuel
  Polik's 2003 reading that Stellar Track's text routine shifts its letters every other line is in
  `design-principles.md`. Opinions of that text differed in 2003: Clay Halliwell called it *"the
  flicker-interlace mess of the Stellar Track routine"*, Thomas Jentzsch *"the best you can get on a
  2600"* (of Stellar Track or Dark Mage/FotR) 〔`200301/msg00444`, `200301/msg00449`〕.
- **Three frames, several rows (Cited only, not verified).** Andrew Davie, 2003, posted a multi-line
  text screen: *"This is really just displaying 3 time-separated frames... so its nothing too
  difficult.   Flickers, but how does it look on hardware, guys?"* 〔stella-list `200301/msg00427`〕.
  Adam Thornton put the demo at 18x6; Davie said *"I can extend the sytstem/resolution to 18 x 12,
  actually, with the same flicker"*, and in a later post that it takes 12 bytes of RAM per text line 〔`200301/msg00428`,
  `200301/msg00431`, `200301/msg00434`〕. Paul Slocum's answer to the hardware question: *"Personally I
  think the flicker is way too much even on real hardware."* Replying to Slocum's idea, which he did not know how to fit in RAM, of a
  variable-width font in the 48-pixel sprite, Clay Halliwell proposed instead what this
  page does, *"2 left-right flickering 48-pixel sprites, at 4 bits per character"* for *"a generous 24
  characters per line"* 〔`200301/msg00442`, `200301/msg00444`〕.
