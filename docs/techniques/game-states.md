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
  - The other layout keeps the state in **bit 7** (a second one in bit 6): `BIT state` then
    `BMI`/`BPL` or `BVS`/`BVC`, no `cmp` and no mask, and the low six bits stay free — SpiceWare:
    *"allowing them to be used for something else such as which game variation the player has
    selected"* (AtariAge `topic/292204`). That `BIT` copies the memory's bit 7 into N and bit 6 into V
    is measured here (`internal/emu/bitflags_test.go`); the layout itself is **Cited only, not
    verified** — the demo uses the values 0/1/2.
- **Input snapshots + edge detection**: read INPT4 + SWCHB once per frame into "current" cells,
  compare with "previous" cells; transitions fire on edges only (hold-to-repeat bugs gone).
  - **RESET needs this too.** An init that runs on every frame RESET is down keeps re-initialising for
    as long as it is held; in one 2019 homebrew that showed as a screen roll with artifacts, which its
    author wondered might be his bankswitching (*"Perhaps"*; AtariAge `topic/287799` — a diagnosis
    by two replies, not confirmed in the thread). nukey-shay's two cures there: a RAM bit remembering the switch (what
    `prevRe` does here), or the cold start placed at the top of overscan so a whole frame is drawn
    before the reset test runs again. **Cited only, not verified.**
  - **Shorter ways to write the same test** — the demo keeps one previous cell per input
    (`prevFi` / `prevRe` / `prevSe`) and tests them one by one. **Cited only, not verified:**
    - `EOR` the current read against the saved one and a 1 is left exactly where something changed:
      every direction bit of SWCHA in one test, `beq` when nothing moved (SpiceWare, AtariAge
      `topic/306641`); on SWCHB, whether any console switch moved at all — nukey-shay: *"you can
      clear the score and whatever if ANY switch has changed position"* (`topic/168437`).
    - One button's previous state can live in one bit and travel through the carry (nukey-shay,
      `topic/286661`): `lsr flag` / `lda INPT4` / `bcc held` / `bmi up` / *(on press)* /
      `held:` `up:` `rol` / `rol flag`. The carry is clear afterwards; for a flag in bit 7, the first
      `lsr` becomes `asl` and the last `rol` becomes `ror`. The middle `rol` takes the button from bit 7
      of A, so the press code must not leave D7 set — **Not verified**. The same
      thread's other form keeps INPT4's masked D7 in a byte and `cmp`s it, which needs masking if that
      byte's other bits are in use.
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
- **Adding a menu to a game built without one** — SpiceWare's order, from a 2019 thread about old
  games (AtariAge `topic/291009`): make room in ROM (easy from 2K; beyond that it means adding
  bankswitching), find the RAM that holds the variation and the RAM the menu can borrow, write the
  menu for that game, point the cartridge's init at the menu, give the game a new init that
  re-initialises the borrowed RAM, and send game-over back to the menu — and let the last score stay
  visible, since a menu that replaces it at once hides it. A wrapper that only hands values to an
  unmodified game cannot take over SELECT, because the game tracks SELECT every frame.
  **Cited only, not verified.**
