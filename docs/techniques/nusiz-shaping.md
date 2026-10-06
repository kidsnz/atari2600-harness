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
- **The colour is single in this kernel.** It writes no `COLUPx` inside the loop, so a shape made of
  copies is one colour across its whole width on any given line. That is this kernel's budget, not
  the hardware's limit (cited, not measured: "Per-copy colours mid-line" below). Within this kernel,
  a second colour costs the second player. SpiceWare's dragon in Medieval Mayhem: *"each player set
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

**Reported in other games, not measured here.** Nukey Shay, 2019, on Pete Rose Baseball: *"HMOVE is
hit on every scanline, which is why there is a black left border"*, and when pitching, the catcher,
batter and pitcher *"have their horizontal motion register and size set on every scanline...makes it
appear that the sprites are wider than 8 bits. If you examine each scanline of sprites, you can see
that no more than 8 bits are used on any of them."* Elsewhere, by his account: M-Network uses it
*"to draw missiles and make them appear as if they are the 8-bit sprites"*, Texas Chainsaw Massacre
for Leatherface's weapon, and Seaquest's divers and bubbles are *"Just the 1-pixel ball sprite
shifted and stretched"* (AtariAge `topic/293976`). **Cited only, not verified** — none of these ROMs
was examined here.

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
  Removing a copy through NUSIZ is older. Piero Cavina, 1997, on groups of three identical aliens:
  rightmost hit, two copies close; leftmost, two close and *"shift it 16 color clocks to the right!"*; centre, two
  copies medium; one left, one copy, moved. He set aside blanking the dead copy's pattern as
  *"much more difficult to me, as it involves cycle counting"*, and wrote *"these are only my
  thoughts, I haven't checked the code of any game"*. Nick Bensema's reply: there is *"no NUSIZ
  register for "nothing shows up""* 〔stella-list `199703/msg00116`, `msg00123`〕. Erik Mooney, 1998,
  computing each bullet copy against each plane copy rather than reading the collision registers,
  needs the same 16: *"we move the object's X-coordinate sixteen pixels to the right (because the
  middle object became the leftmost object.)"* His plan was a ROM table indexed by NUSIZ values and
  the 8-pixel step of the X difference, 1296 bytes if limited to 64 pixels either way 〔stella-list
  `199805/msg00310`〕. **Cited only, not verified**.
- **Two 2x players shifted line by line, as one picture.** SpiceWare, 2024: *"Dolphin and Medieval
  Mayhem use the players in 2x size mode, and moves them left-right over successive scanlines"*, and
  *"Shifting the players line-by-line eliminates the blocky look you normally get when using 2x
  sized players"*. Asked whether Dragonfire's dragon is the same: *"Not the same. Dragonfire uses
  both players in duplicate mode, similar to how a 6-digit score is drawn"*, which shows in debug
  colour mode *"where Red = player0 and yellow = player1"* (AtariAge `topic/361594`). In 2022 he
  called the dragon (the colour bullet above) *"Just 2x with line-by-line shifting"*, said size
  and/or line-by-line shifting *"has been used to great effect for a long time, such as in Boxing and
  Dolphin"*, and that he used it for *"the largest asteroids in Space Rocks, and the space stations
  in Draconian"*. He converted the dragon with his web-based Two Color Sprite Converter; its output
  is two 8-bit bitmaps and a table of one byte per line holding both players' moves, one per nibble,
  plus a second table of the opposite moves for the other direction. The asker's reading of why it
  works: *"shifting the next scanline so you still have illusion of 1 pixel resolution"* (AtariAge
  `topic/344242`; our copy holds 18 of the thread's 23 posts). **Cited only, not verified**.
- **A mirrored body with an independent centre.** SpiceWare, 2007, posting a kernel from a Care
  Bears prototype: P0 at two copies with P1 between them, `REFP0` set before the right copy of P0 is
  drawn and cleared again before the next line, so one graphic makes a symmetric body while P1
  carries the centre (nose, ears) in its own shape and colour. An indirect `JMP` into a run of
  `NOP`s is a variable delay for placing the flip (AtariAge `topic/105254`; **Cited only, not
  verified** — read from distilled notes, not the thread).
- **Borrowing the far player's missile for its width.** A missile's width is in its player's
  `NUSIZx` (`missiles-bullets.md`). Rob, 2003, brainstorming a port, would draw a figure with *"the
  missile for the player on the other side of the screen (so you can mess with the width in
  mid-scanline:)"*. In our reading the far side is the point: that player is not being drawn where
  the write lands. A proposal, not a build 〔stella-list `200307/msg00107`〕; **Cited only, not
  verified**.
- **The ball as a brush.** Fabrizio Zavagli, 2002, drew a bouncing ball with the ball object,
  *"changing the size and horizontal position on each line (that's why it doesn't look all that
  good, but it's still better than just a square I think :)"*, on the free line of a kernel that
  alternates its tile graphics between odd and even lines each frame, cycle-counted with no `WSYNC`
  〔stella-list `200209/msg00107`〕. The ball's size is `CTRLPF` D4-D5, beside the playfield bits
  (`pf-modes.md`). **Cited only, not verified**.
- **Quad width can cost a character its moves.** freshbrood, building Ninja Kombat, dropped one
  character's punch and grab because *"the sprites look way too awful quad stretched"*, and gave it a
  shoulder smash instead (AtariAge `topic/316451`; **Cited only, not verified** — read from distilled
  notes, not the thread). One author's judgement; the 4-clock pixel is in the bullet on two quad
  players above.
- **Per-copy colours mid-line.** splendidnut, 2022: *"Switching colors during a scanline is indeed
  possible. Just requires careful planning"* (AtariAge `topic/344242`). Happy_Dude's Master Mind
  Deluxe kernel, 2004, gives five close-spaced pegs on one line five colours with five `COLUPx`
  writes per line, the last three while the pegs are drawn. Manuel Rotschkar on why it holds: *"I
  even think it only works because the circles are less than 8 pixels wide. (The last color change
  would be 9 pixels wide.)"* — each write has to fall between copies or on blank bits (our reading).
  Its author had run it only in Stella 〔stella-list `200404/msg00103`, `msg00114`〕; **Cited only,
  not verified**. `restrobe-copies.md` has a medium-spaced four-colour version (karl-g) that reports
  close spacing leaves too little time; that one also rewrites `GRPx` mid-line, and this one writes
  only colours (our reading).
- **Copies and width combined mid-line (Meltdown).** Thomas Jentzsch, 2004: the size is *"usually
  set to two copies wide, and during displaying the player the size is set to double size. Timing is
  critical, since you must chance the size exactly at the start of the copy being displayed. If you
  are too early, the copy won't show at all, if you are too late, the first pixel(s) will still have
  single size"* (his spelling). Quad size or three copies could also be used, he found, *"But since
  the timing has to be pixel perfect and the CPU clock runs at 1/3rd of the pixel clock the timing
  for either the 2nd or 3rd copy can't be perfect"*; he suggested scaled graphics *"with 64 pixel
  (2x2 player, double sized)"*, and asked by Manuel Rotschkar whether Meltdown's code runs every
  line or once: *"AFAIK every single scanline."* Eckhard Stolberg recalled the trick from an earlier
  demo of Andrew Towers', adding that *"his timing might have been different"*. Towers' demo has
  four double-width players on a line from P0 and P1, with `VDELP` pre-loading the right-hand
  graphics because otherwise there was no time to switch both NUSIZ and load new graphics; he
  *"couldn't get it to work in any emulator"*, and by his account Christopher Tumber found it works
  on hardware *"if you move two pixels to the right (using the joystick)"* 〔stella-list
  `200408/msg00085`, `msg00087`, `msg00089`, `msg00091`, `msg00095`〕. In 2014 Jentzsch still named
  *"the RESPx /NUSIZx tricks used in Meltdown"* among what makes the 2600 hard to emulate (AtariAge
  `topic/225603`); `stella-oracle.md` records Stella's later fix to its NUSIZ modelling for
  Meltdown. `known-traps.md` lists this trick as not rendered by Gopher2600, so it has not been
  measured here; **Cited only, not verified**.
