# Technique — horizontal playfield scroll (coarse 4px)

**Goal:** scroll a playfield pattern horizontally — the foundation of side-scrollers — verifiably,
at the natural 4-px (one-PF-pixel) coarse granularity.

Demo: `roms/techniques/hscroll.asm` (4px stripes, 32px period, scrolling in reflect mode).
CI: `scenarios/hscroll.json` (phase progression, reflect, 262, golden).
Lineage: studied from the legacy ATARI AR `Side-Scroll/scroll.asm`
(`reference/2600-technique-sources/sidescroll/`).

## The technique
- The PF pattern is stored as **8 precomputed phases** of (PF0, PF1, PF2) — each phase is the
  stripe pattern shifted by one PF pixel (the PF bit-order quirks — PF0 nibble, PF1 reversed,
  PF2 normal — are baked into the table so a clean shift falls out).
- Each scroll tick (every `scrollSpeed` frames) advances the phase by 1 → the stripes move 4px.
  `CTRLPF` reflect mirrors the left half, so both halves scroll symmetrically.
- Per scanline the kernel just holds the current phase's PF (vertical stripes).

## Verified
read_row shows the stripe edges advancing 4px per scroll tick (e.g. edge 28→24 across one tick);
reflect on; phase variable progresses 0→7 wrapping; 262 lines; golden-pinned.

## Notes / variants
- **1-px fine scroll** needs bus stuffing or asymmetric per-line PF rewrites (candidate, harder) —
  the AtariAge "Bus Stuffing Demos" (index-forum50.csv) is the route, noted in the source comments.
- For scrolling *graphics* (not stripes) the phase table generalizes to any 40-bit pattern; a
  longer level scrolls by streaming new columns into the table edge.
- Vertical scroll is independent (shift the row pointer — see bitmap48's window).

## The eight-phase table is a periodic stripe, not a rotation (2026-09-07)

It looks like this technique spends 24 bytes of ROM to avoid a runtime rotation — Manuel Polik posted
one in 2002, alternating `ROR`/`ROL` across PF2/PF1/PF0 with a wrap at the end 〔`200209/msg00126`〕.
**The two are not the same operation.**

Read out of `hscroll.asm` and reassembled into the twenty bits as the beam paints them (PF0 `D4..D7`,
PF1 `D7..D0`, PF2 `D0..D7`), measured by `internal/emu/hscrollphase_test.go`:

- each phase **is** the previous shifted left by one, and
- the bit shifted **in** alternates — `0,0,0,0,1,1,1` across the seven steps — so it is not fed back
  from the bit that left, which is what a rotation does;
- rotating the twenty-bit ring by eight does **not** return phase 0;
- the stripe's period is **8**, and **8 does not divide 20**.

★So the table holds **eight phases of an eight-periodic stripe**, not eight rotations of a playfield.
A true ring rotation repeats after twenty steps; this repeats after eight, which is why `and #7` is
right here and would be wrong for an arbitrary picture.

★★**The trade is therefore not the one it looks like.** 24 bytes buys eight phases of **one periodic**
pattern. An arbitrary twenty-bit playfield tabled the same way needs twenty phases — **60 bytes** — or
a runtime rotation and its cycles. Anyone simplifying this table into a rotation gets a different
picture, silently, for every pattern whose period does not divide 20. Raised by the mailing-list
distillation (helper-1) as a cycles-versus-ROM question; the answer is that the two sides are not
doing the same thing.
