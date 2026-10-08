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

## Beyond this demo — what others posted or proposed (2026-10-05)

None of this was built or measured here. Every bullet is **Cited only, not verified**; where a line
is our own arithmetic or reading, it says so.

**The clock** (extends the checklist's "one frame counter with different masks"):

- **Intervals that are not powers of two.** For *Thrust*, Thomas Jentzsch put a count-to-3 in the
  top two bits of one byte and a period-64 count in the low six; one mask per interval (the 3-count's
  mask ORed with a power-of-two mask for 6/12/48/96/192) then tests every
  1/2/3/4/6/8/12/16/32/48/64/96/192 frames with one `lda` / `and #mask` / `beq`, and advancing the
  byte cost him 28–32 cycles 〔stella-list `200103/msg00078`〕. Later he added: *"if you change the
  counter in the middle of i.E. overscan, then you get 2+1, 3+1, 4+1, 6+1, ..."*, and spreading work
  this way gains *"heaps of cpu cycles for things that have not to be updated every frame. Which
  should be almost anything but pre-calculating for the kernel"* 〔stella-list `200108/msg00039`〕.
- **The mask can come from a table.** Greg Troutman (1997) ANDs the frame counter with a mask read
  through a per-level pointer, indexed by row (`and (ptr),y`); masks 1/3/7 give every 2nd/4th/8th
  frame, so each row's speed on each level is hand-picked in data 〔stella-list `199708/msg00161`〕.
  His was movement (one pixel or none per frame); the same test gates an animation step (our
  reading).
- **Different rates per object.** Andrew Wallace's 2001 shooter already mixed single frame, 2 frames
  at 60 Hz, 2 frames at 30 Hz and 4 frames at 60 Hz — each object with *"the frame count, and
  animation speed which seemed to work best for it"*, all those objects' frames inside the 256 bytes
  allotted. Glenn Saunders had told him *"Don't have a single speed for all the animated sprites"*;
  Glenn's reason was that the 2-frame animations looked too much like *Oystron*
  〔stella-list `200112/msg00068`, `200112/msg00072`〕.
- **A hold time per cel.** Andrew Davie's *Fu-Kung!* v0.07 (January 2003) ran *"simple animation
  loops from lists of frame,duration animation"* — a duration per entry instead of one fixed
  `ANIM_RATE` 〔stella-list `200301/msg00426`〕.
- **From a frame list to a small script.** By *Fu-Kung!* v0.11 (28 January 2003), *"Interaction is
  done totally using the state-system and a simple macro programming language to control creature
  movement/animations"*. An animation is a block of lines such as `LOCK EVENT_ATTACK_THROW` (commented
  *"cause opponent to react"*), `SHOW FRAME_THROW1,4`, `MOVE -5,0` between frames, and a closing
  `GOTO ANIMATION_STAND`; an attack routine is `ANIMATION ANIM_THROW` / `HANDLER NORMAL_STAND2` /
  `rts`. *"Apart from getting the collision code in, most of the programming of the moves themselves
  are simply variations of the above."* The frame names come from the filenames of the original
  graphics, so *"it will assemble correctly, even if the frame table has extra frames added"*
  〔stella-list `200301/msg00475`〕. That `SHOW`'s second operand is the v0.07 duration is our
  reading; the post shows the macro calls, not the macros, so whether they expand to code or to data
  for an interpreter is not known, and the post calls this interaction *"just a mockup"*.
- **No clock at all.** In *Pac-Line Panic* Thomas Jentzsch writes that *"the animations are solely
  based on the positions. So we do not need any bytes for these"*, with the side effect that the
  Pac-Man animation *"is automatically in sync with the pellets he is eating"*; the death animation is
  the exception and reuses the power-pellet timer 〔AtariAge `topic/368501`〕 (the RAM budget this
  belongs to is in `input-budget.md`). He names the cost in the same thread: with hardware collisions
  for the pellets, *"the collision is detected when the mouth gets closed"* and *"Hardware detection
  causes a one frame delay for the collision"*, and he
  could not detect with the mouth open *"because if the players move e.g. every 2nd frame, the mouths
  will still be open when the pellet gets eaten"*.

**The art and its layout:**

- **A per-cel anchor.** *Fu-Kung!* v0.07: *"The babys now have centerpoint data attached, so frame
  display is correctly positioned"* 〔stella-list `200301/msg00426`〕. The demo above assumes every
  phase shares one origin (`frameBase = phase * height`); our reading is that hand-drawn cels whose
  weight shifts (a step, a turn) need such an offset per cel or they jump. The post does not say how
  many bytes it costs or where it is applied.
- **Height per sprite, not a constant.** In Aaron's 2004 six-line-kernel demo the constant
  `SPRITEHEIGHT` became a per-sprite byte from a list: the low three bits take the height's place and
  bit 3 is the reflection, so the same byte is stored to `REFP1` as it is; he notes the height *"can
  change for every sprite (it doesn't in the demo though)"* 〔stella-list `200411/msg00017`〕. That the
  other bits are harmless because `REFP1` uses only D3 is our reading.
- **Stepping only the pointer's low byte.** Ruffin Bailey (2002) keeps each frame 16 lines and adds
  `$10` to the low byte per frame; he wrote that it *"craps out after fifteen frames"* without carrying
  into the high byte, and guessed the sixteenth *"would start dealing with page boundaries"*
  〔stella-list `200207/msg00271`〕. The same post also says *"you can have only eight frames of
  animation or less"*, and Thomas Jentzsch pointed out that its counter code yields 8 frames where 4
  were meant 〔stella-list `200207/msg00272`〕. Our arithmetic: a page holds sixteen 16-byte frames, so the low
  byte alone steps through at most sixteen, and unless the first frame starts a page the last one
  straddles the boundary (the `+1cy` read of the checklist).
- **One `.word` table of cel addresses instead of two byte tables.** Manuel Polik, 2001, answering
  Tempest, who had asked how to animate *"4 frames or so"*: read the shape with `LDA (spritePointer),Y` and reload the pointer from
  two tables, `highpointer .byte #>shape1, #>shape2, #>shape3, #>shape4` and `lowpointer .byte
  #<shape1, …`, indexed by `LDA frameCounter` / `LSR` / `LSR` / `LSR` (*"This'd update every 8
  frames..."*) / `AND #$03` / `TAX` 〔stella-list `200105/msg00038`〕. Pointed back to that answer in
  2003, he updated it — *"nowadays I'd simplify this mess"* — to one table, `shapetab .word shape1,
  shape2, shape3, shape4`, with *"some access code like"* `LDA frameCounter` / `AND #%00000011` / `ASL`
  / `TAX` / `LDA shapetab,X` / `STA spritePointer` / `LDA shapetab+1,X` / `STA spritePointer+1`
  〔stella-list `200304/msg00198`〕. The 2003 listing has no `LSR`s, so copied as it stands it steps the
  cel every frame (our reading; the post does not mention them). Comparing the lookups alone, the
  single table adds the `ASL` — 1 byte and 2 cycles (our count; neither post counts anything).
  `rts-dispatch.md` has the same pair of layouts for jump tables, where splitting into low and high
  tables is what removes the doubling. **Cited only, not verified.**
- **The kernel limits which lines a cel may change.** Erik Mooney's 1997 *Invaders* (invaders drawn
  in the playfield): *"the kernel can only handle modifying the base invader shape on two consecutive
  scanlines out of the six for each invader row"* 〔stella-list `199704/msg00197`〕. The same game also
  had a cel-count rule — *"Each type of invader can cycle through 6 frames of animation. (must be 6
  frames, though it can repeat a 2-frame sequence three times.)"* 〔stella-list `199704/msg00187`〕 —
  but neither that post nor the replies give the reason for the 6.
- **Move Y instead of drawing in-betweens.** In a 1998 fighting-game demo whose kernel kept the
  fighters at one Y because a free Y *"would conflict with the background lines"* (Eckhard Stolberg
  added that the fighters did not use *"the full 42 pixels"*, so *"small jumps would be possible"*), Robin Harbron
  said he would take Y movement over a background if the background graphics had to go, and
  proposed reusing frames *"(jumping/flipping
  especially) and get a better effect, by just varying the y value while keeping the frame still. The
  movements will be still be smooth, without countless frames of animation inbetween"*
  〔stella-list `199808/msg00002`〕. Eckhard's reply: *"I don't think I'll have to remove the background
  completely. I only have to change it, so that the fighters don't jump over any lines"*, and, having
  checked, *"there are only three lines left for the jumps. That wouldn't look too good. I will
  implement y coordinates"* 〔stella-list `199808/msg00009`〕. The frame-reuse idea stays a proposal in
  the thread, not a measured result.
- **Compose a sprite from parts in RAM.** A 2009 forum answer: keep the upper and lower body in
  separate tables, have game logic copy both into a RAM buffer, and let the kernel read the one buffer
  — swapping only the legs pointer shares the upper body across every animation
  〔AtariAge `topic/143070`〕. The buffer costs a byte of RAM per line of sprite (our arithmetic). The
  rotation-sprite rule in `design-principles.md` stages shapes in RAM the same way, for rotation.
- **A RAM-buffered shape makes variants cheap.** Manuel Polik's *Gunfight 2600* (2001) knocks a hat
  off on a hit: *"Stuff like that is easy doable, when the player shapes are buffered in the RAM. Costs
  me only 8 byte instead of having a complete second set of player shapes in the ROM"*
  〔stella-list `200111/msg00001`〕.
- **A proposal: rotate a mark instead of storing cels.** Joel Park, 2002, under the heading "NEWBIE
  THOUGHT" in the *Marble Craze* thread, drew a marble with a reflection mark in one row and asked
  *"Would it be possible"* that, *"instead of setting up a new animation, just use ROL to rotate the
  mark each time the marble is redrawn. This wouldn't make it look like it was rolling, but it would
  give it the appearance of spinning. You might even use ROR and ROL in response to the Paddle
  position."* — *"Just a crazy thought"* 〔stella-list `200207/msg00028`〕. Paul Slocum's reply gives
  the precondition, a shape held in RAM: *"since my Marble data is stored in ROM and not copied to RAM,
  I can't use a ROL/ROR on it. And I don't have any cycles left to do it between reading it from ROM
  and writing it to the TIA."* 〔stella-list `200207/msg00032`〕 Neither post reports it tried. Shifting
  one RAM copy of a shape is in `integration-density-playbook.md` ("The same choice at the size of one
  shape"), and `ROL`/`ROR` go through the carry (`known-traps.md`, *ROL/ROR rotate THROUGH the carry*).
  **Cited only, not verified.**
- **Diagonal facings are drawn, not derived.** `REFP` mirrors only horizontally, so a 45° (or 22.5°)
  view is a frame of its own. Rotating 8×8 art by a non-right angle in a paint program and snapping it
  back to the grid leaves a blob; the forum advice is to draw by hand, using the rotated image or a
  rotated outline only as a guide 〔AtariAge `topic/168144`, `topic/113389`〕.

**Aside — movement, not the animation clock:**

- **Same speed on PAL.** Piero Cavina, with 16-bit positions and per-object increments: use NTSC
  increments that are multiples of 5 and multiply them by 6/5 for PAL 〔stella-list `199708/msg00155`〕.
  This is about position increments, not the animation clock; the same 6/5 rule, with Thomas
  Jentzsch's 2003 NTSC/PAL table and its catches, is in `subpixel-velocity.md` ("What the swapped
  table holds, from 2003").
