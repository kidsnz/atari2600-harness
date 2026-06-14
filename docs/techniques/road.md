# Technique — pseudo-3D road (missiles = shoulders, ball = centre line)

**Source:** clean-room re-implementation studying the 8bitworkshop `road.asm` (Steven Hugg,
*Making Games for the Atari 2600*; `reference/docs_atari/8bitworkshop_samples/road.asm`). This was the
single gap found by `docs/8bitworkshop-crosscheck.md` (technique candidate ㉓).

**Goal:** draw a perspective road with three one-pixel-wide objects — the two missiles M0/M1 as the
left/right **shoulders**, the ball BL as the dashed **centre line** — converging near the horizon and
**widening toward the bottom** of the screen. This is the racing-game primitive and a direct application
of "a 1-px object = a vertical line".

Demo: `roms/techniques/road.asm` (joystick steers the road centre left/right; the shoulders fan out
band-by-band, the centre line dashes and scrolls downward).
CI: `scenarios/road.json` (centre position, foreground shoulder spread, on-screen TIA positions, steering,
262 lines, golden).

## The pattern

- **Bands, not per-row.** The visible area is split into `NBAND` (8) perspective bands. Each band has one
  **half-width** from a monotone table `WidthTab` (4 → 44 px, horizon → foreground). The road opens out
  because the table grows with depth.
- **Per band**: `leftX = curveX − half`, `rightX = curveX + half`, centre = `curveX`. Position M0 at
  `leftX`, M1 at `rightX`, BL at `curveX` with one `PosObj` (divide-by-15 coarse + `eor #7` fine + HMOVE)
  each, then hold the band lit for `BANDROWS` scanlines.
- **Indexed strobes.** `RESM0/RESM1/RESBL` are consecutive ($12/$13/$14) and `HMM0/HMM1/HMBL` are
  consecutive ($22/$23/$24), so one `PosObj` does all three objects via `sta HMM0,y / sta RESM0,y` with
  `Y = 0/1/2`. No per-object code duplication.
- **Dashed centre.** `ENABL` is toggled inside the hold loop by `(row + dashCt) & 4` — a 4-on/4-off
  vertical dash whose phase advances with `dashCt` each frame, so the dashes scroll down (fake forward
  motion).
- **Steering.** The joystick increments/decrements `curveX`; the whole wedge shifts left/right, the road
  "curves".

## Verified facts (scenario road.json, frame after 3 warmup)

- Centre `curveX` ($82) = 78; foreground band mirrors leftMir/rightMir/ctrMir ($88/$89/$8A) = **34 / 122 / 78**
  → foreground shoulder spread = 88 px around centre 78 (half-width 44 = `WidthTab[7]`).
- On-screen TIA positions (last band): **M0 hmoved_pixel = 38, M1 = 126, BL = 86** — i.e. the rendered
  shoulders are 88 px apart, not a blank screen.
- Widening is real, measured with `read_row`: scanline 50 shoulder spread ≈ 7 px, scanline 180 ≈ 63 px.
- Steering: holding `right` 5 frames moves curveX 78 → 83 and the foreground mirrors 34/122 → 39/127
  (the whole road translates).
- `ntsc_frame_lines == 262`, `golden_frame` matches.

## Hard-won notes

- **Line budget.** Each band costs 6 WSYNC for the three `PosObj` calls plus `BANDROWS` hold rows. With
  8 bands that is `8 × (6 + BANDROWS)` visible lines; `BANDROWS = 18` + VSYNC 3 / VBLANK 37 / overscan 23
  lands exactly on 262. Changing the band count or hold rows shifts the frame line total — retune the
  overscan loop, don't hardcode "262".
- **Positioning lines still render.** The 6 WSYNCs spent repositioning between bands are themselves visible
  scanlines with the objects enabled, so the road draws through the whole visible region; the stair-step
  you see at band boundaries is exactly the per-band HMOVE jump (the perspective quantisation), not a gap.
- **Calibrate, don't copy.** `PosObj` fine adjust is `eor #7`; the absolute X→pixel offset is
  kernel-specific. The asserts here lock the measured `hmoved_pixel` values rather than a formula.
- `tiareg` has `ball.enabled` but **no `missileN.enabled`** field — assert missile presence via its
  `hmoved_pixel` position instead.
