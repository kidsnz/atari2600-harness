# Technique #4 — The 2-line kernel

**Goal:** buy CPU headroom inside the visible region. A single-line kernel must finish *all* TIA
updates in 76 cycles per line — with two sprites, a playfield and colors, you run out. The 2-line
kernel stretches each art row over **two scanlines** and splits the work: line A updates one set
of objects, line B the other. Each job now has a whole 76-cycle line to itself. This is the
backbone of most real games (combined with #1/#10 multiplexing). The price: vertical resolution
halves — positions move in 2-line steps.

Learned from (clean-room): Darrell Spice Jr. *Let's Make a Game* Step 4; `multisprite.inc`
discussions. Demo: `roms/techniques/two_line_kernel.asm`, locked in CI by
`scenarios/two_line_kernel.json`.

## The technique

96 pairs × 2 lines = 192. Per pair (Y = pair index, sprite coords in *pair units*):

- **Line A:** vertical-compare + GRP0 store for P0 (#3's idiom, ~21 cy), then a background
  gradient `COLUBK` update (~14 cy) — two jobs, still half the budget free.
- **Line B:** vertical-compare + GRP1 store for P1, loop control. In a real game this is where
  game logic or the missile/ball updates go.

### Positioning two players: one shared HMOVE
Set `HMP0+RESP0` on one line, `HMP1+RESP1` on the next, then strobe **HMOVE once** after the
final WSYNC — it applies every loaded HMxx register simultaneously.
**Pitfall (cost us +3 px, found by `read_tia`):** strobing HMOVE after *each* positioning line
re-applies the earlier sprite's HMxx a second time (the registers keep their values until HMCLR
or rewrite). One strobe, after everything is staged.

### Known refinements (documented)
- **VDEL odd/even trick:** writing GRPx through the vertical-delay shadow register lets a sprite
  start on an odd scanline inside a 2-line kernel = 1-px vertical granularity back. We have VDEL
  hardware-verified (litmus_vdel, Exerciser 48px) but haven't built the odd/even form yet.
- Carry hygiene in shared lines: an `adc` after the sprite compare inherits its carry/`lsr`
  residue — our gradient flickered at stripe edges until the add became an `ora` (valid since
  the operands can't overlap). Constant-input ops beat flag-dependent ones inside kernels.

## Verified here (Gopher2600, locked in CI)
- P0 (diamond, X=60) and P1 (frame, X=100) bounce independently in pair units over a striped
  gradient; RAM/`hmoved_pixel` asserted at fixed frames; 262 lines every frame; budget clean
  (A ≈ 45 cy / B ≈ 40 cy); golden frame.
