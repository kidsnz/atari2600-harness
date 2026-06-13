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
