# Technique — 24-character text line (50% flicker, two blocks)

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
- For genuinely interleaved single characters (za2600's look), switch to NUSIZ medium and offset
  by 8px instead of 48 — same skeleton, different position constants.
- Per-frame color staging works in the gaps for two-color text.
