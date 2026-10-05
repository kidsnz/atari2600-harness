# Technique #3 — Vertical positioning (any-Y sprite placement)

**Goal:** put a sprite at *any* vertical position, smoothly, 1 px per frame. Horizontal position
has hardware (RESPx + HMOVE); **vertical has none** — the TIA only knows "what's in GRP0 right
now". Vertical position is therefore a *software illusion*: every scanline the kernel asks
"is the beam inside my sprite?" and feeds GRP0 either an art row or zero.

Learned from (clean-room, ideas only): Darrell Spice Jr. *Let's Make a Game* Step 5; the
`skipDraw` idiom discussion (Davie/AtariAge). Demo: `roms/techniques/vertical_pos.asm`
(a ball bouncing Y 4⇔180 at fixed X=80), locked in CI by `scenarios/vertical_pos.json`.

## The technique

Per visible line (Y register = line number):

```
        tya             ; current line
        sec
        sbc sprY        ; A = row inside the sprite
        cmp #SPRITE_H   ; carry clear ⇔ 0 <= row < height
        bcc .draw
        lda #0          ; outside: blank
        beq .store      ; (always taken — keeps both paths near-equal cycles)
.draw:  tax
        lda Art,x       ; inside: art row
.store: sta GRP0
```

- The subtraction underflows to large values above the sprite, so a single unsigned `cmp`
  covers both "above" and "below" — no second compare.
- Both paths re-converge on one `sta GRP0` (~21 cycles in) — early enough in the line for any
  sprite X ≳ 8, and the line stays far under the 76-cycle budget (~30 cycles total).
- Movement logic runs once per frame in VBLANK; the kernel only reads `sprY`.
- **Exactly equal paths, from the list.** Where `beq .store` above gets the two paths near-equal,
  Andrew Davie's 2001 form pads the skip path to the draw path's length:
  `tya / sec / sbc SpriteEnd / adc #SPRITE_HEIGHT / bcs .Draw / nop / nop / sec / bcs .skipDraw`,
  then `.Draw: lda (Sprite),y / sta PLAYER0SHAPE` — 11 cycles from the branch to `.skipDraw` on
  either path by our count of the listed opcodes (zero-page operands, no page crossed). The post's
  own total reads 18, where the same count gives 20 〔stella-list `200102/msg00256`〕. Manuel Polik's *Gunfight* skip path matches the carry as well
  as the cycles and gives its filler a job: `SEC ; Same carry state after both branches` / `NOP` /
  `LDA #$00` / `STA.w COLUP0 ; Special trick, blacks the bullets` / `BEQ Continue` — *"no matter
  which branch is taken, I'm exactly on the same cycle when hitting 'Continue'"*, where one
  `STA GRP0` takes either the shape or `00` 〔`200110/msg00272`〕. (COLUP0 colours missile 0 as well
  as player 0, which is how a store on the skip path can black the bullets.) **Cited only, not
  verified.**

### skipDraw / DCP variant — **verified** (v1.23.0, `vertical_pos_dcp.asm`)
The classic undocumented-opcode idiom: per line `lda #H-1` / `DCP sprDraw` ($C7 zp = DEC+CMP) /
`bcs draw`; `sprDraw` initialized to `sprY+H` each frame counts down through 0..H-1 for exactly
H lines; art is stored bottom-up and indexed by the counter. DASM 2.20.14.1 assembles the illegal
mnemonics (`dcp $80` → `C7 80`; `lax`, `sax` likewise), so `dcp` can be written as is; the kernel's `.byte $C7` assembles the same. **Measured on this kernel: max line 40→38 cycles, sprite line 31→30**
(2–3 cycles — modest here; the idiom's real value is freeing A/X/Y pressure: no `tya`, so Y
stays available for other per-line work). Pixel-identical to the compare version (CI-locked).
The 2001 post that offered the DCP form lists three advantages: the state of the carry no longer
matters (*"may save 2 cycles"*), A stays constant (*"could be useful for a 2nd sprite"*), and the
counter's content can index the sprite data instead of Y 〔stella-list `200102/msg00282`〕. This
kernel uses the third; it draws one sprite, so the second is not exercised here. Thomas Jentzsch's
own counts for the idiom alone, range test through `STA GRPx` with `LDA (GfxPtr),Y`, are **19/12**
cycles (drawn/skipped) for the legal `TYA / SEC / SBC / ADC / BCC` form, 17/10 when the carry is
already known, and **17/10** for `LDA #H-1 / DCP / BCC` 〔`200110/msg00229`〕 — his annotations, over
a different stretch than the 20/17 WSYNC→GRP0 (drawn/skipped) measured on this kernel in `fundamentals-audit.md`.
**Cited only, not verified.**
- **Pointer pre-offset:** set `sprPtr = Art − sprY` in VBLANK and `lda (sprPtr),y` in-kernel;
  pairs naturally with masking tables for tall sprites.
  It has three preconditions; a 2004 cookbook attempt written without them showed the right graphic
  only while Y was below the sprite height 〔`200404/msg00285`〕. Y starts at the kernel height and
  counts down each line (each pair of lines in a 2LK) 〔`200404/msg00289`〕; the
  pointer is set outside the kernel so it points at the art exactly when the right lines are
  reached; and *"To avoid page penalties, this also requires the graphics to start at (align 256 +
  kernel-height)."* 〔`200404/msg00292`〕 The asker's reply: *"most of the other descriptions I've seen
  don't get into what all the preconditions (set up in the VBLANK) are."* 〔`200404/msg00293`〕
  **Cited only, not verified** — the demos here index with X (`Art,x` / `ArtRev,x`), not `(sprPtr),y`.
- Combine with #2 (animation): `Art` becomes `Frames + frameBase`.

### The rest of the family — cited, not built here

No demo or test here covers this subsection; every cycle count is the source's own annotation.
**Cited only, not verified.**

- **Zero padding takes the range test out of the kernel.** Pad the art with zero bytes above and
  below and offset the pointer outside the kernel, so the line counter indexes the art directly and
  out-of-range lines read a zero: the draw is `lda (ptr),y / sta GRP0`, **8 cycles a sprite** with no
  branch (no page crossed), against 17–18 for a clipped draw — SpiceWare's padded form in *Medieval
  Mayhem* (AtariAge `topic/88607`). Two sprites fit in 18: `lda (gfx1),y / tax / lda (gfx0),y /
  sta GRP0 / stx GRP1` (`topic/296173`). The price is ROM for the padding and a vertical range bounded
  by it — the trade `fundamentals-audit.md` measured for SwitchDraw's 256-byte table (17 cycles on
  every line, 248 bytes a sprite), paid here with a pointer instead of an index. **When the kernel is
  tight, always draw and let a zero hide it, rather than branch to decide whether to draw.**
- **What the padding costs, as the list counted it.** Indexing a 192-line kernel straight into the
  art needs 192 − height zero bytes before and after it, for every frame of animation (the blanks
  can overlap somewhat), plus the extra cycle when the index crosses a page (Erik Mooney,
  stella-list `200110/msg00226`). Lee Fastenau page-aligned each sprite and filled the rest of its
  page with zeros, reading it with `lda ballShape0,x` and no skipdraw: 256 bytes a sprite, and his
  two-sprite ball *"was chomping up an eighth of my 4K"* (`200409/msg00002`, `200409/msg00006`).
  Paul Slocum, who reads *Maze Craze* as doing the same (*"best I can tell"*), expected the zeros
  to *"limit the sprite animation I can do"*; his plan to shrink them was to re-point once per
  13-line band, at the art or at *"a small zero'd out piece of memory"* (`200202/msg00167`) — a
  plan, not a result, and the same shape as just-jeff's periodic reset below. Thomas Jentzsch's reply pointed to skipDraw as *"the fastest way I know
  without wasting memory"* (`200202/msg00168`): the padding pays in ROM, skipDraw in cycles. Erik
  Mooney's *RRampage* shares the zeros instead — at the end of an object (a check run once every
  4 lines) the pointer's high byte moves to *"an entire ROM page full of just zeroes"*, so enemies
  can sit any vertical distance apart: *"ROM is plentiful, cycles aren't."* (`200404/msg00269`; his
  demo did not yet move anything vertically).
  For blanking after the last row, B. Watson's answer is a zero as each shape's last byte, one
  byte a shape (`200110/msg00236`); Manuel Polik's is to clear in the skip path — *"you'd only lose
  two bytes _once_ in each skipdraw"*, with no cycles lost because that path is the faster one —
  since in *Gunfight* the byte a shape would be RAM (`200110/msg00244`). And a kernel whose draw
  starts and stops only every fourth raster can pad a few zeros above and below, letting the start
  slide against that alignment, and advance the fetch index on the other lines without a test
  (AtariAge `topic/136387`).
- **Two more ways to drop the range test** (AtariAge `topic/288362`): pad the art with zeros (e.g.
  50 bytes either side) and reset the pointer periodically inside the kernel — `dey / tya / sbc
  ObjectY` compares against the sprite, and when it is out of range the pointer's low byte is moved
  onto the zeros, which needs the zeros and the art in one page; the padding costs ROM on both sides
  (just-jeff). Or draw from a RAM buffer one kernel tall, cleared by pointing the stack at its end and pushing zeros with `PHA`, then filled
  with the art — no padding at all, because RAM data can be rewritten where a ROM table can only be
  pointed at (jeremiahk). Erik Mooney proposed the same for Supercharger or Superchip RAM in 2001:
  copy the art offscreen into the right place in a zeroed 100-byte block, so every kernel line is
  an indexed load and a zero-page store — and noted that *"even Superchip RAM isn't enough to do
  that for more than one player object"* (stella-list `200108/msg00598`).
- **Names for the rest of the SkipDraw family** (Verdant's 2024 catalogue, AtariAge `topic/363349`).
  **MaskDraw** (SpiceWare) is the masked draw in `design-principles.md` (*Masked sprite drawing*):
  `lda (pattern),y / and (mask),y / sta GRPx` is 13 cycles, 21 with the colour pair, and the mask is
  an `align 256` block of `SPACE_BEFORE` zeros, `SPRITE_HEIGHT` `$FF`s and `SPACE_AFTER` zeros.
  **FlipDraw** (Manuel Rotschkar) lets the `DCP` counter itself become the art index once in range
  (`ldy Sprite_Y / lda (ptr),y`), 20 cycles on both paths with the draw on the fall-through — the
  balanced-path move `fundamentals-audit.md` describes. Verdant rebuilt it from one mailing-list post
  and could not find it in a ROM. That post (2005) gives it as `LDA #SPRITEHEIGHT / DCP spriteOffset /
  BCC FlipDraw / LDY spriteOffset / LDA (spritePtr),Y` and prices it: *"2/3 cycles more than
  skip/switchdraw, but it neither requires a line counter (save lotsa cycles here!) nor occupies a
  register"*; the setup is `spriteOffset = spriteY` and a plain pointer to the art, no bias
  (stella-list `200508/msg00049`).
- **The counting-up form uses `ISB` where this page uses `DCP`.** Steven Hugg's *Making Games for the
  Atari 2600* draws with `lda #SpriteHeight / isb YP0 / bcs .DoDraw / lda #0` (INC, then SBC). Andrew
  Davie's reading is that YP0 has to start as the **negative** of the sprite's Y; he says he has never
  used it: *"It seems to me that .DoDraw will receive a non-zero index only when scanline >= -YP0, and
  it will be valid for SpriteHeight+1 lines."* — and next, *"I might be out a line or two but maybe
  that's how it works..."* (AtariAge `topic/296245`). SpiceWare had only seen `DCP` used for DoDraw.
  On the mailing list in 2004, Aaron gave a reason to pick it: `lda #SPRITEHEIGHT / sec /
  isb SpriteEnd / bcc .skipDraw` *"leaves the a register with useful information"*, where *"the
  version of skipdraw with dcp just throws that result away since dcp is based on cmp"* and the
  value has to be reloaded from `SpriteEnd`; the costs are the `SEC` (*"which you might not even need"*) and storing *"the negative of
  the distance from the top of the screen"* (`200411/msg00005`, `200411/msg00017`).
- **A price list by method** (eshu, AtariAge `topic/136387`): `LDA Sprite,y / STA GRP0` is 7 cycles
  but limits the sprite to 128 lines and wastes a lot of memory; `LDA Sprite,y / AND Mask,y / STA
  GRP0` is 11 and wastes less, but the sprite data has to sit in the middle of a page; SkipDraw
  costs more and has fewer constraints.
- **Five names** (Chris Walton, stella-list `200505/msg00069`): SkipDraw, SkipDraw in its
  illegal-opcode version, SwitchDraw, a Stack Trick and a JSR/BRK Trick — *"There are lots of subtle
  issues in these algorithms that took me a long time to work out!"* The first two are this page and
  SwitchDraw is measured in `fundamentals-audit.md`; the post does not describe the last two, and
  the Stack Trick is probably the `PHP`-to-ENAM trick in `missiles-bullets.md` (our reading); nothing here
  describes a JSR/BRK Trick.
- **When every row of the art is the same byte, load it as an immediate.** Manuel Polik's rewrite of
  Ruffin Bailey's kernel: `LDX #$00 / TYA / SBC yposP0 / ADC #$10 / BCC RuffinDraw1 / LDX #%10100101`,
  then `RuffinDraw1: STX GRP0` — no table and no pointer (stella-list `200201/msg00035`).
- **Reuse the range test's result as the row index.** The compare version above already does this
  for the sprite (A after `sbc sprY` is the row). Thomas Jentzsch applied it to a low-resolution
  playfield island: after `bcc NoPF`, `lsr / lsr / tax / lda island,x / sta PF2`. The asker had
  re-derived the row from Y and saw *"the island starts jump every 4 frames I think"*; Thomas Jentzsch's diagnosis: *"the height of the first row of the island differs between 2 and
  8, because it only depends on the current y-value, but not on the starting row of the island"*
  (`200107/msg00025`, `200107/msg00040`). Shifts give power-of-two row heights only (our reading).
- **When an object never moves vertically, skip the question.** For a castle game where nothing but
  the ball and the playfield needs vertical positioning, Manuel Polik sketched a top kernel and a
  bottom kernel; the bottom one's *"line counter is already indexing all data tables and you never
  have to answer the "to draw or not to draw" question - you just always draw"* (`200307/msg00112`)
  — a sketch from a brainstorming thread, not a built kernel.
- **One line ahead through a byte of RAM.** Kirk Israel's 2002 stopgap: *"the first thing I do at
  the start of my kernel is to load that buffer into GRP0, then I do the calculations for the "next"
  line of the player during the rest of the visible line, and finally load up the buffer right
  before the WSYNC"* — *"no cycle counting or page aliging"* (`200207/msg00037`). The price is a byte
  of RAM an object; Ruffin Bailey counted the per-line part as a zero-page `LDA` and `STA`, 6 cycles
  an object (`200207/msg00041`).
- **The decision ahead of time.** Erik Mooney's 2001 invaders kernel: *"How do you make a
  store/no-store decision in zero cycles? Make the decision ahead of time by self-modifying code in
  RAM."* His decision is which invaders of an 11×5 grid are drawn, not a vertical range
  (`200103/msg00184`).
- **Counting down versus VDEL.** Roger Williams's 2001 demos, *"tested on a real 2600"*: *"The
  method I use to count down the screen to set GRPx is not compatible with VDEL, but it does allow
  the sprites to scroll smoothely off the top and bottom of the screen."* That is his method; the
  post does not say which part of it conflicts (`200110/msg00291`).
- **A second counter, folded into a table.** Bob Montgomery's kernel counted lines in Y and blocks
  in X: `dey / tya / cmp BlockChange,X / bne / dex / bpl` — *"That takes 15 cycles"*
  (`200507/msg00031`). Swapping the registers for `cpx BlockChange,y` is not an option: `CPX` and
  `CPY` have only immediate, zero-page and absolute modes, as Bob pointed out and Manuel Rotschkar
  conceded (`200507/msg00033`, `200507/msg00035`). The answer Thomas Jentzsch quoted from Manuel's
  earlier post indexes a table by the line instead — `DEY / LDX LargerButFunkierTable,Y / BPL
  .nextLine` (`200507/msg00040`); the table's name is the price.

## Verified here (Gopher2600, locked in CI)

- Ball bounces Y 4⇔180; `sprY`/direction asserted by RAM at fixed frames; X pinned at 80 via
  `tia.player0.hmoved_pixel`; 262 lines; line budget clean; golden frame.
- Pixel-level: `read_row` confirms 8 contiguous rows of P0 color at the expected grid rows,
  matching the art **bit-for-bit** (the `%11011011` row reads back as 2-2-2 runs).
- **Calibration is kernel-specific, again:** this ROM's positioning prologue is `lda #imm`
  (2 cy) where sprite_anim's is `lda zp` (3 cy) — 1 CPU cycle = 3 px, so `XCAL` here is −5,
  not −8. Never copy a calibration constant between kernels; re-measure (`read_tia`).

## Harness fix shipped with this technique

`read_row`'s y-coordinate was off by `visibleTop` (~29 lines) from the annotated-grid labels it
promises to match — static-content checks (playfield) were self-consistent, but cross-referencing
a screenshot coordinate missed. Fixed in v1.4.0 (`internal/emu/emu.go ReadRow` now subtracts
`visibleTop`); the grid y you see is now exactly what you pass.
