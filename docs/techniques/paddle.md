# Technique — paddle input (dump / charge / per-line count)

**Goal:** the standard paddle-reading kernel: discharge the pot capacitor during blanking,
release at visible start, count scanlines until INPT0 D7 goes high — that count *is* the paddle
value, mapped to whatever the game controls.

Demo: `roms/techniques/paddle_demo.asm` (a P0 bar tracks the paddle).
CI: `scenarios/paddle_demo.json` (three paddle positions → exact line counts + bar X, golden).
Hardware basis: `litmus_paddle` (v0.54.0; INPT0 dump/charge transfer curve measured).

## The pattern

- **Overscan + VBLANK: `VBLANK = $82`** — D1 blanks the screen as usual, **D7 dumps the paddle
  caps** (discharge). Both functions live in one register; keep dump on through blanking.
- **Visible start: `VBLANK = 0`** — releases the dump, capacitor starts charging.
- **Kernel, each line** (cheap: ~12 cycles when already latched):
  ```
  lda padNew / cmp #$FF / bne done   ; already latched this frame?
  bit INPT0 / bpl done               ; D7 still 0 = charging
  stx padNew                         ; X = line counter → the paddle value
  ```
- End of frame: commit `padNew` → `padVal` (use $FF→191 for "never latched" = far end),
  clamp/map to the controlled quantity. The demo maps to bar X via PosObject (clamp 151).

## Verified numbers
With this frame structure, `set_input paddle` 0.1 / 0.25 / 0.5 measure **0 / 63 / 170 lines**
(matches the litmus transfer curve shifted by the dump-release line). Bar X follows exactly
(170 clamps to 151). Note the measured value depends on *when in the frame you release the
dump* — re-baseline the scenario after structural changes (ours shifted by 7 lines when the
VBLANK timer constant moved).

## Notes
- Full paddle range needs more lines than one visible frame at the far end — real paddle games
  either accept saturation (as here) or read across two frames.
- Four paddles = INPT0-3 with the same pattern; pairs share a port.
- Latch test uses **N flag (`bit`/`bpl`)**, per the verified input rules (never test low bits).
