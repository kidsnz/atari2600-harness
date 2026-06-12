# Technique #2 — Sprite animation (frame cycling + REFP reflection)

**Goal:** make a sprite *move like a creature, not a token*: cycle its GRP bitmap through a few
frames on a timer (walk cycles, blinking, spinning wheels), and flip it horizontally for free with
**REFP0/REFP1** so you only store art for one facing direction. Ubiquitous — almost every 2600 game
animates something this way.

Learned from (clean-room, ideas only — implementation is our own): Darrell Spice Jr.,
*Let's Make a Game* Step 14 (`reference/docs_atari/spiceware_tutorial/`). Demo:
`roms/techniques/sprite_anim.asm`, locked in CI by `roms/techniques/scenarios/sprite_anim.json`.

## The technique

1. **Animation clock.** A frame counter divides the 60 Hz TV rate down to the animation rate:
   every `ANIM_RATE` frames advance `phase = (phase + 1) & (NPHASES-1)`. Powers of two keep the
   wrap a single `and`.
2. **Frame storage and selection.** All phases live contiguously in ROM (`NPHASES × height`
   bytes). With small sprites, skip pointers entirely: precompute `frameBase = phase * height`
   once per frame (three `asl` for height 8), and the kernel indexes `Frames + frameBase + row`
   with `tax` / `lda Frames,x`. Pointer tables (`lda (ptr),y`) only pay off for tall sprites or
   page-crossing art.
3. **Row doubling/quadrupling.** The kernel derives the art row from the line counter
   (`tya / lsr / lsr` = 4 TV lines per art row) — bigger sprite, no extra bytes, and the indexed
   load stays well inside the 76-cycle line budget (~25 cycles).
4. **Free horizontal flip.** `REFP0` bit D3 mirrors the bit order the TIA scans out — write it
   from the facing direction once per frame in VBLANK. Draw the art with an asymmetric detail
   (our walker's forward arm) so the flip reads on screen. No second art set, no CPU cost.
5. **Decouple logic from drawing.** Movement/animation logic, REFP+frameBase staging, and the
   horizontal reposition each get their own VBLANK line, closed by `WSYNC` — the visible kernel
   only consumes precomputed state. The whole frame owns its 262 WSYNCs explicitly
   (3+37+192+30) rather than letting any logic line spill.

## Verified here (Gopher2600, locked in CI)

- 4-phase, 8-frame-per-phase walk cycle; phase asserted by RAM at fixed frames.
- Ping-pong X 10⇔140; **the applied horizontal mapping is calibrated so `pos(v) = v` exactly**
  (`XCAL = -8` on the divide-by-15 + HMOVE-table positioner; swept organically across the range).
- `REFP0` asserted via `tiareg.player0.reflected` after the turn; the mirrored pixels confirmed
  by row reads.
- 262 lines every frame; line budget clean (`assert_line_budget`).

## Measurement subtlety (worth remembering)

`tia.player0.hmoved_pixel` sampled **at the frame boundary** reflects the *previous* frame's
reposition (the new frame's positioning lines haven't run yet): while walking right it reads
`xpos−1`, walking left `xpos+1`. This is an observation-time artifact, not a positioning error —
the drawn pixels match the intended `xpos` for the frame being displayed. Scenario asserts encode
the lagged values on purpose. Also: poking state mid-stream from a test harness interacts with
frame-boundary anatomy — calibrate with *organic* runs, not pokes (we mis-measured ±2 px twice
before learning this).

## Reuse checklist

- `ANIM_RATE` per object; several objects can share one frame counter with different masks.
- Keep each phase's bytes in one page (or align `Frames`) to dodge `+1cy` page-cross surprises
  in timed kernels (not an issue at this demo's budget).
- For tall/many-phase art switch to per-phase pointers staged in VBLANK (`lda (p),y` is 5 cycles
  in-kernel — budget it).
