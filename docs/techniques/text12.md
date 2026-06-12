# Technique — 12-character text line (flicker-free)

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
