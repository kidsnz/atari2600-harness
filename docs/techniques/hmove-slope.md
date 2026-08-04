# Technique — fractional-HMOVE slope (an arbitrary-angle 1px line from a missile or ball)

**Goal:** draw a thin straight line at **any** angle — a fishing line, a tether, a rope, a laser, a
tow cable — using one missile or the ball, one pixel wide, with no playfield and no per-line graphic
table.

**Status:** ✅ hardware-verified and CI-locked. Fixture `roms/litmus/litmus_hmove_slope.asm`;
grading `internal/emu/hmoveslope_test.go` (drawn x against the line equation: **max error 0 px over
160 scanlines**, on two slopes in opposite directions).
**Source:** the situation and the idea come from the Fishing Derby entry in `docs/casebook.md`
(capability gap **G9(b)** in `docs/capability-gap-audit.md`) — the pattern Claude's sealed
reconstruction of that game had no counterpart for. Re-derived clean-room here from the hardware
rules and measured, not transcribed. Independently confirmed on the original cartridge by raw pixel
attribution — see "What the original actually does" below.

## The problem

A missile or the ball can be one colour clock wide, which is exactly what a thin line needs. Moving
it down the screen is free — it is drawn on every scanline. Moving it *sideways* is the problem:
`HMOVE` shifts an object by a **whole** colour clock or not at all, so per-line HMOVE gives you
slopes of 0 and ±1 px per scanline and nothing between. A line from a rod tip to a hook 3 pixels
across and 8 scanlines down is not expressible.

## The pattern

Keep an 8-bit **error accumulator** per object. Add the fractional slope every line; the carry out
of `adc` is "one whole pixel is owed", and it is the only thing that decides whether that line gets
a move. This is Bresenham with HMOVE as the plotter.

```
        ; setup: HMCLR, accumulator = 0
SL:     sta WSYNC
        sta HMOVE               ; 0-2   this line's move, decided by the PREVIOUS line
        lda acc                 ; 3-5
        clc
        adc #NUM                ; the fraction; C=1 means a whole pixel is owed
        sta acc
        ldx #$00
        bcc noMove
        ldx #$F0                ; right 1   ($10 = left 1)
noMove: ...pad to cycle >=26...
        stx HMBL                ; ~cycle 35: clear of the 24-cycle post-HMOVE hazard
```

**Choosing NUM.** The slope is `NUM/256` pixels per scanline, so `NUM = round(256 × slope)` and the
position of the n-th line of the band is exactly

```
x(n) = x(0) ± floor(n × NUM / 256)
```

| line you want | slope | NUM |
|---|---|---|
| 3 px over 8 scanlines | 0.375 | `$60` (96) |
| 1 px over 2 scanlines | 0.5 | `$80` (128) |
| 1 px over 3 scanlines | 0.333… | `$55` (85) |
| 1 px over 10 scanlines | 0.1 | `$1A` (26) |
| `dx` over `dy` | dx/dy | `(dx*256+dy/2)/dy`, clamped to 255 |

For a line between two *moving* endpoints, recompute `NUM` once per frame in the housekeeping line
(`NUM = 256·dx/dy` needs a divide; `docs/techniques/divtable.md` has the reciprocal-multiply form)
and reset `acc` at the top of the band so the line always starts from the anchor.

Slopes steeper than 1 px per scanline need the integer part too: move by `int` clocks every line
(`HM` nibble = the whole part, up to 8) and let the accumulator add the extra one. Beyond ±8 px per
line a single HMOVE cannot keep up and the object needs re-strobing.

**This is the same DDA as `docs/techniques/subpixel-velocity.md`**, aimed at a different consumer:
there the accumulator spills into a *position byte once per frame*, here into an *HMOVE once per
scanline*. The sign trap documented there applies here too — the carry always means "+1", so a
leftward line loads a left nibble rather than adding.

## Constraints / gotchas (all measured)

- **`sta HMOVE` must be the first instruction after `WSYNC`.** Measured while building the sibling
  fixture `roms/litmus/litmus_nusiz_shape.asm`: with the strobe at CPU cycle 10, objects gained +1
  clock per line **even with `HM=$00`**, i.e. a phantom slope of exactly 1 px/scanline underneath the
  intended one. Keep a zero-motion object on screen as a control; it is the only thing that catches
  this, because every slope graded relative to its own first line survives it.
- **Write `HMxx` late in the line**, ≥24 cycles after the strobe (`docs/known-traps.md`, fixture
  `roms/litmus/lint_r3_hazard.asm`). Consequence: the value HMOVE consumes was computed on the
  previous line, which is what fixes the equation's phase — `x(0)` gets no move.
- **Two objects fit comfortably, three is the practical limit.** One accumulator is ~19 cycles
  (load, add, store, branch, load nibble, store); the measured fixture runs two accumulators plus a
  static third object in ~49 of the 76 cycles.
- **HMOVE on a visible line blanks visible clocks 0-7.** Strobe on every line of the band so the
  margin is uniform, and do not let the line reach clock 8.
- **A 1-pixel object inks one clock LEFT of where a player would**, given the same RESxx timing.
  Measured at three positions on this fixture: the ÷15 delay units that put a player at 12 / 87 / 147
  put the ball and missiles at 11 / 86 / 146.
- **Crossing lines occlude.** TIA priority is P0/M0 > P1/M1 > BL/PF, so where two of these lines
  cross, one disappears for the lines they share — and a measurement that identifies objects by
  colour reads that as "the object vanished". Lay the paths out so they do not cross, or measure with
  `decompose_row` instead of `read_row`.
- **The line is one colour**, and the ball takes COLUPF, so a ball line changes colour with the
  playfield.

## Verified numbers

`roms/litmus/litmus_hmove_slope.asm` runs three 1-pixel objects down one 160-line band from a single
HMOVE strobe per line:

| object | NUM | slope | x(0) → x(159) | agreement with `x(n) = x(0) ± floor(n·NUM/256)` |
|---|---|---|---|---|
| ball | `$60` | 96/256 = 3/8 = 0.375 right | 11 → 70 (+59) | **160 of 160 scanlines exact, max error 0 px** |
| missile 1 | `$55` | 85/256 = 0.332… left | 146 → 94 (−52) | **160 of 160 scanlines exact, max error 0 px** |
| missile 0 | — | 0 (control) | 86 → 86 | held clock 86 on **160 of 160** |

85/256 is deliberately not a dyadic fraction: that angle is unreachable by any doubling or halving
scheme, so "arbitrary" is a measurement rather than a manner of speaking. The two slopes run in
opposite directions on the same strobes, so a symmetric sign error cannot hide.

Negative controls (the tests were watched failing, then restored):

- accumulator step `$60` → `$61` (0.379 instead of 0.375) → *"max error 1 px over 160 scanlines (120
  of 160 exact); worst at line 37, drawn at 25, the line equation says 24"* and *"travelled +60 px
  over the band, the slope says +59"*. A one-bit change in the numerator is caught.
- giving the static control the ball's move → *"the zero-motion control moved on 157 of 160
  scanlines (first at line 3)"*, and the crossing paths trip the separability guard by name.

## What the original actually does

Measured on the Fishing Derby cartridge under `sandbox/studies/fishing-derby/` (umbrella-only; not
part of this repository and not in CI) with `emu.DecomposeRow`, 2026-08-04, one frame of live play:
the right-hand line is the **ball**, one pixel wide, drawn on **110 consecutive scanlines**, moving
clock 103 → 94 = **−0.0826 px per scanline** — a slope no whole-clock-per-line HMOVE can produce.
Its x holds for 9, 15, 10, 11, 10, 12, 12, 8, 12 and 11 scanlines in turn: the moves are **not on a
fixed period**, which is the observable signature of an accumulator rather than a fixed divider. The
left-hand line is `M1` and is vertical in that frame (43 scanlines, 0 px of travel), consistent with
only the reeling player's line being under tension. No disassembly was consulted.
