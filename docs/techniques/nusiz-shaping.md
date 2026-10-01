# Technique — per-scanline NUSIZ + HMOVE shaping (one player, an irregular shape wider than 8px)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture drives `NUSIZ` from a table and sweeps every mode, so it is the most exposed of all. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

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
  A second colour costs the second player. SpiceWare's dragon in Medieval Mayhem: *"each player set
  for 2x size / each player is a different color / each player shifted left/right over successive
  scanlines"*, and Circus Convoy *"did add changing the colors of the players over successive
  scanlines"*. alex_79 on why the pair reads as one sprite: *"Objects with lower priority will only
  be seen through the "holes" of the ones that are on top of them, creating what seems a high res
  multicolor sprite"* (AtariAge `topic/344242`; **Cited only, not verified**). That is two objects,
  so the cost bullet above applies to it.

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

## Neighbouring uses of NUSIZ (cited, not built here)

- **A black quad-width player as a mask.** omegamatrix, hiding a railing where it wraps at the right
  edge: *"position P0 at pixel 143. Make COLUP0 black, and use Quad Size for NUSIZ0. P0 will be large
  enough to cover up the railing if you make its length a few pixels shorter."* *"This works because
  P0 always has priority over P1 and M1"* — the order `invisible-probe.md` reads from the engine's
  `video.go`. He calls it *"a crappy one because you lose P0 an probably M0, and restrict the color"*
  (his spelling; AtariAge `topic/233831`; **Cited only, not verified**). A black cover hides only
  against a black background (our reading). His *"I seem to recall the positioning gets delay 1 or 2
  pixels in quad size"* is measured here as one clock — the double/quad bullet above.
- **Two quad-width players as one picture.** kiwi drew a forum avatar with them: *"I used 2 quad size
  player to make the icon"* (AtariAge `topic/300645`; **Cited only, not verified** — read from
  distilled notes, not the thread). A quad player is
  32 clocks for 8 bits (`litmus_nusiz_quad`), so the picture is 4-clock pixels; the 1x pair joined
  without a seam is `litmus_p0p1`.
- **Removing one copy of three.** ZackAttack, 2018: *"I'd view the three enemies as a single enemy.
  This aggregate enemy would have 8 states corresponding to which of the three copies is active. (3
  copies, 2 states, 2^3=8 states total)"*, with a transition table of *"only 24 bytes of ROM"*, a
  table for *"the NUSIZ1 value for each state (8 ROM bytes), and the horizontal offset for each
  transition (24 ROM bytes). Still it's only taking up 56 bytes of ROM total"*. The code in his later
  sketch he marked *"untested"*. Moving the
  base when the left copy goes is `sprite-placement.md`'s "Move the variable, not the object". Nukey
  Shay's alternative in the same thread: *"always draw 3 copies...but draw blank bitmaps in place of
  the one(s) hit"*, with *"NUSIZ registers are only altered as bordering columns are removed
  completely"* (AtariAge `topic/274546`; **Cited only, not verified**). That puts a GRP write between
  two copies, and when such a write takes effect is `sprite-placement.md`'s rule 6, measured in
  `internal/emu/spriteplace_test.go`.
