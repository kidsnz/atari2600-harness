# Technique — per-scanline NUSIZ + HMOVE shaping (one player, an irregular shape wider than 8px)

**Goal:** draw a single creature that is wider than eight pixels and not a rectangle — a shark, a
whale, a boss, a vehicle — out of **one** player object, with no flicker and no second sprite.

**Status:** ✅ hardware-verified and CI-locked. Fixture `roms/litmus/litmus_nusiz_shape.asm`;
grading `internal/emu/nusizshape_test.go` (the intended outline matches the drawn pixels on 40 of
40 scanlines, and on 120 of 120 control scanlines).
**Source:** the situation and the idea come from the Fishing Derby entry in `docs/casebook.md`
(capability gap **G9(a)** in `docs/capability-gap-audit.md`) — the pattern Claude's sealed
reconstruction of that game had no counterpart for. Re-derived clean-room here from the hardware
rules and measured, not transcribed. Independently confirmed on the original cartridge by raw pixel
attribution — see "What the original actually does" below.

## The problem

A player object is eight bits of graphics. `NUSIZ` can widen it (double = 16 clocks, quad = 32) or
copy it (2 or 3 copies at 16 / 32 / 64 clock spacing), but if you set NUSIZ **once per frame** every
scanline of the object gets the same treatment: you get a rectangle of copies, not a silhouette. The
usual escapes are expensive — flicker two objects at 30 Hz, or spend a second player you needed for
something else.

## The pattern

Set `NUSIZ0` **and** strobe `HMOVE` on *every line of the kernel*. NUSIZ decides that line's width
and copy pattern; the accumulated HMOVE decides that line's left edge. The outline is then a
per-line pair (width, edge) and the graphic underneath can stay constant.

```
        ; before the band: position the object, prime HMP0 with line 0's move
SL:     sta WSYNC
        sta HMOVE               ; 0-2    FIRST instruction after WSYNC. Not negotiable — see below.
        lda NusizTab,y          ; 3-6
        sta NUSIZ0              ; 7-9    lands in HBLANK, ahead of the beam reaching the object
        iny
        lda HmTab,y             ; the NEXT line's move
        nop                     ; \
        nop                     ;  > spacing: an HMxx write must be >=24 cycles after HMOVE
        nop                     ; /
        sta HMP0                ; ~cycle 28
        cpy #rows
        bne SL
```

**Designing the outline.** Write the silhouette you want as a table of (left edge, width) per band,
then convert:

| you want | you write |
|---|---|
| the width/copy pattern of band *b* | `NusizTab[b]` |
| left edge of band *b* | `HmTab[b]` = the **delta** from band *b−1*, as an HM nibble |

and the position the hardware reaches on band *b* is

```
left(b) = X0 + Σ(HM deltas up to b) + sizeDelay(NUSIZ)
sizeDelay = 1 for double ($x5) and quad ($x7) width, 0 for every other mode
```

`HmTab` entries are the standard nibbles (`$10` = left 1 … `$70` = left 7, `$80` = right 8 …
`$F0` = right 1, `$00` = no move), so one line moves at most 8 clocks: a steep edge needs a band per
step. Put `$00` on the lines inside a band and the whole band shares one edge.

## Constraints / gotchas (all measured)

- **`sta HMOVE` must be the first instruction after `WSYNC`.** Measured on this fixture while it was
  being built: with the strobe at CPU cycle 10 instead of cycle 0, every object gained **+1 clock per
  line even with `HM=$00`** — 39 clocks of drift over 40 lines. Every band still looked plausible;
  only a deliberately motionless control row caught it. This is why the fixture carries one.
- **The next line's `HMxx` has to be prepared on this line, ≥24 cycles after the strobe** (the known
  post-HMOVE hazard, `docs/known-traps.md`, fixture `roms/litmus/lint_r3_hazard.asm`). That inverts
  the natural order: HMOVE consumes what the *previous* line computed.
- **Double and quad width start one clock later than the 1x modes.** Measured on
  `roms/litmus/litmus_nusiz_all.asm`: modes 0-4 and 6 ink from clock 24, modes 5 and 7 from clock 25.
  Forget it and a body that switches between quad and 1x is 1px wrong on every quad line.
- **The NUSIZ write must land before the beam reaches the object.** In the kernel above it retires at
  colour clock ~30, so anything from visible clock ~10 rightwards is safe. An object near the left
  edge needs the write earlier, which means fewer cycles for everything else.
- **HMOVE on a visible line blanks visible clocks 0-7** (the comb). Strobe on *every* line so the
  blank is a uniform left margin rather than a ragged notch, and keep the shape clear of it.
- **Cost is about 36 cycles per scanline** for one object, which makes this a one-line kernel with
  room for a little else — not something to run for two objects at once without a 2-line kernel.
- **The colour is single.** COLUPx is per-line at best; a shape made of copies is one colour across
  its whole width on any given line.

## Verified numbers

`roms/litmus/litmus_nusiz_shape.asm` runs the same 40-line kernel four times over the same tables,
changing only two zero-page masks, so each register's contribution can be measured alone:

| block | NUSIZ | HMOVE | what it proves | result |
|---|---|---|---|---|
| 0 shaped | on | on | the outline is the intended one | **40 of 40 scanlines**, 840 px of ink against 840 intended |
| 1 nusiz-only | on | off | widths are right, edge never moves | 40 of 40 |
| 2 hmove-only | off | on | edges are right, width never changes | 40 of 40 |
| 3 flat | off | off | 40 zero-motion strobes displace nothing | static on 40 of 40; differs from block 0 on 40 of 40 |

The shaped block is additionally graded **without any table at all**: its runs must equal block 1's
runs translated by block 2's displacement, on all 8 bands. That relation catches the axes
interfering with each other, which no comparison against a table can.

Negative controls (the tests were watched failing, then restored):

- deleting the single `sta NUSIZ0` → *"the outline matches at 5 of 40 scanlines (ink drawn 320 px,
  intended 840 px)"*, plus *"widest band is 8 px of ink over an 8-clock span"*.
- zeroing **one** `HmTab` entry (band 3's right-4) → *"the outline matches at 15 of 40 scanlines"*,
  naming rows 53-62 and the exact spans.
- making block 1 stop being the width oracle → the table tests still pass and the metamorphic
  relation fails on 7 of 8 bands, which is what that relation is for.

## What the original actually does

Measured on the Fishing Derby cartridge under `sandbox/studies/fishing-derby/` (umbrella-only; not
part of this repository and not in CI) with `emu.DecomposeRow`, 2026-08-04, one frame of live play:
**P0 is drawn on 103 scanlines with 13 distinct per-row ink widths** (1, 2, 3, 4, 5, 6, 7, 8, 10,
11, 12, 24 and 28 px), reaching a **28-clock extent on a single line out of an 8-bit graphics
register**, with the copy count changing from two copies to one wide copy inside four scanlines and
the left edge stepping 44 → 43 → 42 on consecutive lines. That is this technique, read off the
pixels — no disassembly was consulted.
