# Technique — game state machine (title / play / game-over, console switches, attract)

**Goal:** the structural skeleton every real game ships with: a state machine driving
title → play → game-over → title, console-switch handling (RESET / SELECT / difficulty), and an
attract mode — all verifiable frame-by-frame.

Demo: `roms/techniques/game_states.asm`.
CI: `scenarios/game_states.json` (full lifecycle over ~1100 frames, golden).
New hardware verification: `litmus_swchb` + `scenarios/swchb.json` — **SWCHB read side verified**
(D0 RESET / D1 SELECT active-low, D3 color/BW, D6/D7 difficulty), driven by the harness's
extended `SetPanel` (now also `color` / `p0pro` / `p1pro`) and scenario panel inputs.

## Structure

- **One state byte** (`state` = 0 title / 1 play / 2 over) asserted directly in scenarios.
- **Input snapshots + edge detection**: read INPT4 + SWCHB once per frame into "current" cells,
  compare with "previous" cells; transitions fire on edges only (hold-to-repeat bugs gone).
- **Frame logic under TIM64T in VBLANK** (the dynamic-multisprite pattern): state branches have
  wildly different lengths; the timer keeps the frame at 262 lines regardless.
- **title**: SELECT cycles the game variant (0-3); RESET *or* fire starts; 300 idle frames turn
  on attract (background pulse), any input clears it.
- **play**: a drifting sprite (HMP0=$F0 + one HMOVE per frame — the cheapest motion); the round
  timer counts double when **P0 difficulty = A/Pro** (SWCHB D6), so Pro rounds are half as long —
  the scenario discriminates this by timing (B: over at ~320f, Pro: ~160f).
- **game-over**: 120 frames back to title; RESET restarts immediately.
- **Deterministic state entry**: `EnterPlay` strobes RESP0 at a fixed cycle after WSYNC, so the
  sprite X is identical on every entry (golden-stable).

## Verified
- Full lifecycle (11 asserts over ~1100 frames): variant select, both start paths, B vs Pro round
  lengths, game-over timeout, return to title, attract flag.
- **Dogfood**: `fieldtest -auto` on this ROM reports `auto-start: reset` — the harness's title-
  screen escalation detects exactly the start method this technique implements.

## Integration notes
- A real game replaces the play-state body and keeps the dispatcher/edges/timer shell as is.
- SELECT-cycled `variant` is where game options live (number of players, speed class …).
- Attract typically swaps to a self-playing demo; the flag + idle counter here is the hook.
