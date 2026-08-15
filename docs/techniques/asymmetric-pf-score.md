# Technique — asymmetric-playfield two-digit score (left/right independent, PONG-style)

**Goal:** big blocky per-player scores (2 BCD digits each side) drawn with the PLAYFIELD —
players/missiles stay free for game objects — by rewriting PF1/PF2 mid-scanline so the left
and right screen halves show different digits (CTRLPF repeat mode, reflect OFF).

**Status:** verified **in-game** (PONG `sandbox/practice/pong/steps/pong_top_paddle_pf2_*.asm`,
first landed in `pf2_score-2digit-playfield` 2026-07-01); standalone demo ROM + CI scenario = TODO.
**Source:** in-house, PONG pf2 work 2026-07-01 (session fa891501), digit layout matched to the
real Video Olympics look (4-px digits above the top wall). All digit cells verified with `read_row`.

## The pattern
- **CTRLPF D0=0 (repeat):** the right half re-reads PF0/PF1/PF2 in the same order → write the
  registers twice per line (left values in HBLANK, right values mid-line after the beam passes
  the left half) = 4 independent digit fields per line.
- **Digit geometry (per side):** tens + ones, each 4 PF pixels (=16 clocks) wide with a 1-PF-px
  gap, 5 zones × 4 scanlines tall (the classic chunky look). Layout used:
  - left tens = PF1 bits 4-1 (clock 28-43), bit0 = inter-digit gap
  - left ones = PF2 bits 0-3 (clock 48-63; PF2 is LSB-left → bit-reversed glyphs)
  - right tens = PF1 bits 7-4 (clock 96-111), bit3 = gap
  - right ones **straddles registers**: PF1 bits 2-0 (clock 116-127) + PF2 bit 0 (clock 128-131)
- **5 glyph tables** (one per field; the straddling digit needs a hi/lo pair), 8 bytes/digit,
  page-aligned so `(zp),y` never page-crosses. Pointers = `digit*8 + table base`, computed in
  VBLANK from the packed BCD scores.
- **Kernel line:** `WSYNC → PF1←(ScLt),y → PF2←(ScLo),y → ~10 nop → PF1←(ScRt)|(ScRoH),y →
  PF2←(ScRoL),y` — the mid-line rewrite must complete after the beam draws the left half
  (clock >79) and before it reads PF1 for the right half (clock 96).
- **Font pipeline:** glyphs auto-generated from the hand-painted 8×20 master font
  (`sandbox/practice/pong/tools/pong_font_gen_pf.py`, OR-pair 8px→4px) so one painting feeds both the sprite-font
  and playfield-font versions.

## Verified numbers (PONG)
- All four digit fields verified per zone with `read_row` (e.g. "83 38": left-8 bar clock 28-43
  = $1E pattern, right-ones straddle at clock 116-131).
- Budget: the score-band line ≈ well under 76cy (two pointer reads per half + nop delay);
  `assert_line_budget` over=false with the full game running.
- Layout matches the real-PONG reference screenshot (digit centers symmetric about the net).
