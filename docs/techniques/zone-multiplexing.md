# Technique #1 — Sprite multiplexing (vertical zones)

> Also known as: a **multi-sprite kernel**. DaveC's term **"zone"** is the common descriptive name for the
> vertical-band form used here. Our demo is the **static-zones** form (see "Forms" below).

**Goal:** show **more than the hardware's 2 players** on one frame. The TIA only has 2 player objects, but
its restrictions are **per scanline, not per frame** — so by **re-using P0/P1 in different vertical bands**
(reposition X, reload graphics/color as the beam descends) you can show many players. N bands → up to 2·N.
This is the standard 2600 trick behind crowded screens (rows of cars, enemies, lily pads).

Sources studied (clean-room — the idea is universal 2600 knowledge; the implementation here is our own):
DaveC's `landscape.asm` (AtariAge; `reference/dave/`) and the 8bitworkshop multisprite kernels
(`reference/docs_atari/8bitworkshop_samples/multisprite*.asm`). See **References** below.

## Formal name & taxonomy
- The umbrella term is **sprite multiplexing**: "reusing the same sprite slots more than once per frame or
  scan line" (Wikipedia). On the 2600 it's an *undocumented* trick — you reset the player objects mid-frame.
- The timing-sensitive display loop that does it is **the kernel**; one that draws many sprites is a
  **multi-sprite kernel**.
- **Hard limit: at most 2 player-objects per scanline.** Bands stack *vertically*; within any single scanline
  you still get only two. Going beyond two *on the same line* needs **flicker** (below) or the missile/ball/PF.
- **Single-line vs 2-line kernel.** A *single-line* kernel updates the TIA every scanline (almost no spare
  CPU). A *2-line (double-line) kernel* repeats each sprite line over 2 scanlines, buying CPU time for logic —
  the more common choice for real games. Ours is effectively single-line.

## Forms (ours vs the general one)
- **Static zones (this demo):** fixed bands, exactly P0+P1 per band, positions in RAM. Simple, deterministic,
  no per-frame sorting. Good when objects live in known rows (Frogger lanes).
- **General multi-sprite kernel:** a *sort → position → display* pipeline that Y-sorts an arbitrary set of
  objects each frame, allocates the nearest two to P0/P1, and (when a 3rd collides on a line) **flickers**
  them with a priority counter so they blink instead of vanishing. More flexible, more code. (Roadmap item.)

## How our demo works
Per-band X lives in RAM (`zx0`/`zx1`); the kernel walks bands top→bottom and per band:
1. **Reposition P0/P1** with the harness-verified coarse+fine method: a divide-by-15 loop (`sec`/`sbc #15`/
   `bcs`, 5 cyc = 15 color clocks coarse) then the remainder indexes an HMOVE-nibble table → `HMPx` + strobe
   `RESPx`, with `HMOVE` right after a `WSYNC`. (8bitworkshop calls this routine `SetHorizPos`.)
2. **Set the band color** (`COLUBK`) in HBLANK so it doesn't shift the positioning.
3. **Draw** the sprite for the band's height (`GRP0`/`GRP1` from a table; `cpy #SPRITE_H` height guard).

## Refinements & limits (documented — to verify if we rely on them)
- **Positioning costs scanlines.** Each band spends its first 1–2 lines on positioning; two sprites whose
  tops are too close vertically can clash (the lower one may be dropped). The general kernel mitigates via the
  priority counter.
- **Flicker** is the accepted way past the 2-per-line wall: alternate which objects get P0/P1 each frame; a
  priority counter gives the longest-unshown object precedence so motion stays legible.
- **2-line kernel** is usually worth it (CPU headroom for game logic); cost is half vertical sprite resolution.
- **Page alignment** of the kernel and the HMOVE table matters (a mid-loop page cross adds a cycle and shears
  the picture); a timer (`TIM64T`) keeps VBLANK stable regardless of per-frame work.

## Cycle-level craft (verified in our build)
- HMOVE table placed to avoid a page-cross on the positioning line (`LOOKUP = TABLE_END - 256`, negative index).
- Every line budgeted to 76 CPU cycles; the per-frame position-update loop is absorbed by retuning VBLANK to
  keep the frame at 262 lines.

## How the harness verifies it
Building blocks are hardware-verified (`litmus_pos` = positioning, `litmus_hmove` = HMOVE, `litmus_sprite` =
GRP bit order). The **composite** is locked by `roms/techniques/scenarios/zone_multiplex.json` (golden frame),
run in CI; `get_screen_annotated` shows all 12 and `read_ram` shows the motion (position bytes change frame to
frame). Cross-checked in Stella.

## Status — ✅ verified
- `roms/techniques/zone_multiplex.asm`: **12 moving sprites** (6 bands × P0+P1) from a 2-player machine, with
  per-band X in RAM updated each frame (P0 right, P1 left, wrap `and #$7F`) and per-band background colors
  (a landscape look). Verified on Gopher2600 + cross-checked in Stella; CI-locked.

## See also
- **48-pixel sprite** ("Six-Digit Score Trick" / Staugas kernel) — a *different* wide-sprite trick (3-copies
  + VDEL shadow registers), for titles/scores. (Roadmap.)
- **Venetian Blinds** (Bob Whitehead, *Video Chess* 1979) — a *different*, older flavor: horizontal reuse +
  **vertical interlacing** (every other line) of the same object, which flickers/looks striped. (Roadmap.)
- **General multi-sprite kernel** (sort/position/display + flicker) — the dynamic form of this technique. (Roadmap.)
- The full candidate list: [`roadmap.md`](roadmap.md).

## References
- Wikipedia — *Sprite multiplexing*: https://en.wikipedia.org/wiki/Sprite_multiplexing
- 8bitworkshop multisprite kernels (single- & 2-line, sort/position/display, flicker):
  `reference/docs_atari/8bitworkshop_samples/multisprite{1,2,3}.asm`, `multisprite.inc`
- Darrell Spice Jr., *Let's Make a Game* (Step 4 = 2-line kernel): `reference/docs_atari/spiceware_tutorial/`
- Andrew Davie, *2600 Programming for Newbies* (Sessions 21–23, vertical placement):
  `reference/docs_atari/Atari_2600_Programming_for_Newbies.txt`; https://www.randomterrain.com/atari-2600-memories-tutorial-andrew-davie-23.html
- Bumbershoot Software — *Successfully Multiplexing Sprites*: https://bumbershootsoft.wordpress.com/2024/10/05/atari-2600-successfully-multiplexing-sprites/
- AtariAge — *multi-sprite kernel strategies or examples* (topic 347667); splendidnut, *2600 Display Kernels* (blog)
- DaveC's `landscape.asm` (`reference/dave/`) — the "zone" form studied here.
