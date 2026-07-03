# Technique — HMOVE two-step positioning (beyond the ÷15 budget wall)

**Goal:** place a sprite at the far right edge (clock ~150-159) when the classic per-frame
÷15 positioning loop can't reach it — the loop's iteration count grows with X, and past
X≈149 the positioning line overruns 76 cycles (measured 152cy = 2 scanlines = roll).

**Status:** verified **in-game** (PONG `sandbox/practice/pong/steps/pong_top_paddle_pf2_02_flyoff-right.asm`
and later); standalone demo ROM + CI scenario = TODO (candidate for the techniques pipeline).
**Source:** in-house invention, PONG fly-off work 2026-07-02 (session fa891501). Budget wall measured
with `assert_line_budget` (BallX≥150 → 152cy); fix verified with `read_row` (ball at clock 158-159).

## The problem
`A = X; loop: sbc #15 / bcs loop` positions by *burning cycles proportional to X*. On a shared
76-cycle line, iterations = floor(X/15)+1; at X≥150 the 11th iteration plus epilogue
(`eor #7`, 4×`asl`, `sta HMPx`, `sta.w RESPx`) no longer fits. The budget wall is a property of
the POSITIONING METHOD, not the screen — the beam can draw to 159 but ÷15 can't strobe there in time.

## The pattern (two-step)
1. **Clamp the ÷15 input** into budget: for targets in the overflow zone (150..157),
   feed the loop `X − 8` (≤149 = 10 iterations, fits).
2. **Second HMOVE on the NEXT (black) line** shifts the sprite the remaining 8px right
   (`HMPx = $80` = right 8). For normal targets store `$00` = no-op second HMOVE.
   ```
   ; line A (prep, black):  clamp calc → HMX2 = $00 or $80 (or $10 = left 1 for the LEFT edge)
   ; line B (black):        ÷15 loop → HMPx/RESPx
   ; line C (black):        HMOVE #1  → wait ≥24cy → HMPx ← HMX2
   ; line D (black):        HMOVE #2  (comb lands here = invisible)
   ; line E (visible band): no HMOVE = no comb notch
   ```
3. Same trick mirrors to the LEFT edge: position at clock 1 via ÷15, then `HMX2=$10`
   (left 1) reaches clock 0 — the true first pixel.

## Constraints / gotchas (all measured)
- **HMxx write must wait ≥24 CPU cycles after HMOVE #1** (known trap) — pad before storing HMX2.
- Both HMOVEs must land on **black lines** or the 8px comb notches a visible row
  (see known-traps "HMOVE comb on a visible line").
- Other objects' HMxx must be 0 at HMOVE #2 (HMCLR earlier, or they drift every frame).
- Keep the ÷15 loop **page-aligned** (known trap: page-cross +1cy/iteration = judder + budget).

## Verified numbers (PONG)
- Right: BallX=157 → ball pixels at clock **158-159** (`read_row` run len 2), exact screen edge.
- Left: BallX=0 + `HMX2=$10` → pixels **0-1**.
- Budget: `assert_line_budget budget=76` over=false with the drift path exercised (50f frozen at
  the max, then 600f free-run).
