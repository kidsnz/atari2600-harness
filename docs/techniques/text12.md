# Technique — 12-character text line (flicker-free)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** readable text for menus, messages, and titles: 12 characters per line, flicker-free,
using the same hardware-verified 48px VDEL 6-store choreography as the score kernel — with a
4×5 font packed two characters per player byte.

Demo: `roms/techniques/text12.asm` ("HELLO WORLD!" / "ATARI 2600.." on two lines).
CI: `scenarios/text12.json` (positions, packed-buffer bytes, 262 lines, golden).
Lineage: David Crane's 12-characters-per-line routine (Basic Programming, 1979) — the ancestor
of every 2600 text display; researched via the AtariAge 32-character-display thread, where the
width ladder is 12 (flicker-free) → 24 (column flicker) → 28 (Jentzsch) → 32 (interleaved
RESP re-strobing, solidcorp 2011). 12 is the sweet spot: zero flicker, no re-strobe timing
hazards, reuses the score kernel verbatim.

★**That ladder counts CHARACTERS, not width — the character width never changes.** Measured
2026-09-07 by reading the picture (`internal/emu/textwidth_test.go`): `text12` puts its twelve
characters in **clock 87..134 = 48 px**, and `text24` puts the *same* 48-px block at **two** X
positions — 39..86 and 87..134 — on alternate frames. **48 px is one 6-store sprite block, every rung
uses the same 4×5 font, and 48 / 12 = 4 px per character at every rung.** Climbing the ladder does not
narrow the letters; it adds another block at another X and pays in flicker. What a rung buys is how
much of the 160-px line carries text; what it costs is how often each block is drawn.

★★**So a glyph wider than 4 px is not a character in any of these kernels.** A 10-px letter needs the
48-px block used as a *picture* (`bitmap48.md`), where it is one of about four shapes on the line
rather than one of twelve. That is a different technique with a different cycle budget, not a wider
setting of this one.

★★★**Sampling one frame hides half of this.** One frame of `text24` shows one 48-px band and invites
the conclusion that 24 characters are squeezed into the same span at 2 px each. Both phases have to be
read — the same trap `scripts/phase_probe.py` exists for. Raised by the mailing-list distillation
(helper-2) from a 2003 thread where three people took apart David Crane's routine and disagreed over
whether letters could be 7 px or 8 px wide 〔`200309/msg00212`, `msg00216`, `msg00218`〕.

**Before designing anything wider, read [`restrobe-copies.md`](restrobe-copies.md) (technique #36).**
The rungs above 24 all use the mechanism named in the line above — a mid-line `RESP` re-strobe — and
#36 is where this harness measured it: a player in a copy mode draws **3 + k** slots with k mid-line
strobes at 6, 7 or 8 cycles of spacing, the ladder is **flat** at 3 and 5 cycles, and it climbs faster
at 12. This page's "12 is the sweet spot" is a statement about cost, not about a ceiling. A work in
the private `roms/` repository re-derived that ceiling from scratch in 2026-08 without reading #36,
and lost days to it; the one-way link (#36 pointed here, nothing pointed there) is why.

## The technique

- **Font**: 4×5 glyphs (39 chars: space, A-Z, 0-9, !, .), one nibble per row (bit 3 = leftmost),
  stored bottom-row-first (the kernel walks Y=4..0). 200 bytes of ROM.
- **Build (once, or per string change)**: for each of 6 character pairs, compose
  `Font[left]<<4 | Font[right]` per row into a **column-major zero-page buffer**
  (`buf[pair*5+row]`, 30 bytes per text line). Strings are stored pre-encoded as glyph indices.
- **Kernel**: exactly the score6 choreography — six `(zp),y` pointers set to `buf+pair*5`,
  4-burst stores completing at 55/58/61/64 cy, position P0=87/P1=95. Each text row is drawn on
  2 scanlines (the second line re-runs the store sequence with the same Y) → a text line is
  12 chars × 10 scanlines.

## Verified
- Both demo lines render legibly on first run (annotated screenshot), buffer composition is
  byte-exact (`buf[0] = H‹4|E = $9F` asserted), 262 lines every frame, golden-pinned.

## Notes / variants
- Per-line color: set COLUP0/1 before each text line (the choreography leaves room outside the
  burst). Scrolling: feed the window through the buffer build (the bitmap-minikernel idea).
- Wider displays (24/32 chars) need column flicker or RESPx re-strobing — recorded as catalog
  candidates with the measured constraints (9px strobe granularity, RESP-vs-GRP write conflicts)
  from the research thread; implement when a game needs them.
