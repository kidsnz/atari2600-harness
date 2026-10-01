# Technique #1 — Sprite multiplexing (vertical zones)

> Also known as: a **multi-sprite kernel**. DaveC's term **"zone"** is the common descriptive name for the
> vertical-band form used here. Our demo is the **static-zones** form (see "Forms" below).

**Goal:** show **more than the hardware's 2 players** on one frame. The TIA only has 2 player objects, but
its restrictions are **per scanline, not per frame** — so by **re-using P0/P1 in different vertical bands**
(reposition X, reload graphics/color as the beam descends) you can show many players. N bands → up to 2·N.
This is the standard 2600 trick behind crowded screens (rows of cars, enemies, lily pads).

Sources studied (clean-room — the idea is universal 2600 knowledge; the implementation here is our own):
DaveC's `landscape.asm` (AtariAge; `reference/files-dave/`) and the 8bitworkshop multisprite kernels
(`reference/docs_atari/8bitworkshop_samples/multisprite*.asm`). See **References** below.

## Formal name & taxonomy
- The umbrella term is **sprite multiplexing**: "reusing the same sprite slots more than once per frame or
  scan line" (Wikipedia). On the 2600 it's an *undocumented* trick — you reset the player objects mid-frame.
- The timing-sensitive display loop that does it is **the kernel**; one that draws many sprites is a
  **multi-sprite kernel**.
- **Hard limit: at most 2 player-objects per scanline.** Bands stack *vertically*; within any single scanline
  you still get only two. Going beyond two *on the same line* needs **flicker** (below), the
  missile/ball/PF, **or a third route this line used to omit: interleave the two players at a wide
  `NUSIZ` so each sits in the other's gaps.** Glenn Saunders describes it on the list in 2000 —
  P1 P2 P1 P2 P1 P2 across one line, **six figures and no flicker**, paid for in width rather than
  in frames. **And a fourth: use a missile as a CHARACTER** — vary its width (`NUSIZ` bits 4-5) and
  its colour every line and `HMOVE` it every line, and it draws a shape rather than a dot 〔stella
  1997, Erik Mooney, with a tank sketched in ASCII〕. Added 2026-09-03; neither is measured here.
  **★And a fifth, added 2026-09-06 and this one IS measured: pay in RAM.** Reserve a strip of RAM
  per player, draw the shape into it at the right vertical offset, and the kernel line becomes
  `lda P0strip,y / sta GRP0` — *"No skipdraw trickery, and to have multiple players vertically, you
  just draw multiple players in the RAM strip. YOu can have as many as you want, and they can
  overlap fine, and they won't flicker when vertically overlapping"* 〔stella-list `200305/msg00000`,
  Andrew Davie, 2003-05-01, in a thread titled "What would you do with more RAM?"〕.
  **Price: 7 cycles a line per player, not the 8 the list says** — `sta GRP0` is a store to $1B and
  the TIA is in the zero page, so it is `sta zp` at three, not four (measured,
  `internal/emu/ramstrip_test.go`: load 4 + store 3). **8 is reachable and at exactly the proposed
  size**: a 256-byte strip spans a page, so the indexed read crosses one whenever the index carries,
  and those lines cost 8. Page-align the strip and you keep the cycle — a little over five scanlines
  a frame for two players. **Price in RAM: one byte per KERNEL LINE per player — not the 256 the
  proposal names.** The list had said so a year before the proposal was made: *"Even with your
  current code, the full 256 bytes make no sense, because the values for Y are not ranging from
  0..255 when drawing the ball.  You initialize Y with 156, so that's the maximum of bytes you
  should have to waste"* 〔`200207/msg00025`, Thomas Jentzsch, 2002-07-03〕. 256 is the ceiling of
  the index, not the requirement. **That correction cuts both ways and the second way is the useful
  one:** at 156 the strip fits INSIDE a page with 100 bytes of slack, so the page-alignment above
  costs nothing to arrange, where a 256-byte strip has to be exactly page-aligned or it spans two by
  construction. Against `design.RAM2600 = 128` a 156-line strip still needs a SuperChip — but the
  figure that decides that is now the kernel's height, so **a short enough zone fits in the stock
  128 and this route stops being SuperChip-only**, which was not visible while the price read 256.
  The other side of that was priced on the list too: *"Sure, it wastes RAM on lines the sprites don't appear in, but it's worth it …
  all those RAM strips do add up"* 〔`200309/msg00071`, Glenn Saunders〕.
- **Single-line vs 2-line kernel.** A *single-line* kernel updates the TIA every scanline (almost no spare
  CPU). A *2-line (double-line) kernel* repeats each sprite line over 2 scanlines, buying CPU time for logic —
  the more common choice for real games. Ours is effectively single-line.
  **The choice can be made per band, not per frame.** Erik Mooney, 1997, planning a Space Invaders
  whose invaders are playfield: the invader rows run in two-scanline blocks with *"about one spare
  cycle"*, and for the shields — *"If the shields are 12 lines high (twice the invader height - this
  matches the arcade), I can make the kernel in there operate in 3-scanline blocks instead of two, and I'll
  have enough time to write player graphics registers four times."* Two more moves in the same message:
  taking up Glenn Saunders's marching "legs" 〔stella-list `199704/msg00115`〕, one animation applied on a different line per
  row — *"bottom two do the "legs" as their bottom line, middle two rows do the "legs" as the middle
  line, and top row does it as the top line - this makes the "heads" of the top row thinner, just like
  the arcade"* — so the rows look like different creatures for no extra data; and *"five rows of six
  bytes"* of RAM laid out with *"a two-byte gap in RAM between each row, so the offset between rows is 8
  which is a power of two.. I am using those gaps for other data"* 〔stella-list `199704/msg00117`〕.
  **Cited only, not verified** — the band split and the legs are plans (*"I might be able to"*); what
  the finished game does has not been checked.

## Forms (ours vs the general one)
- **Static zones (this demo):** fixed bands, exactly P0+P1 per band, positions in RAM. Simple, deterministic,
  no per-frame sorting. Good when objects live in known rows (Frogger lanes).
  **Vanguard's display is this form at full stretch** (Nukey Shay, 2010): *"Vanguard's display is just
  a loop, either drawing the current enemy sprite or horizontal repositioning to draw the next enemy
  sprite (via HMOVE)"* — still two players on any scanline — so *"the only limit to the number of enemy
  sprites is the number of those "bands" between HMOVE lines visible on the left border."* The price is
  motion, twice: *"sprites are not allowed to move between those HMOVE line sections"*, and *"The band
  height is a bit larger than the sprites in Vanguard, so enemies have limited vertical movement as
  well."* More bands, more enemies, less room for each to move. The general form below makes the
  opposite trade — it chooses which objects to draw beforehand, so they can move over the whole display
  〔AtariAge `topic/170088`〕. **Cited only, not verified** — Vanguard is in this repository only as a
  measurement target (`docs/visual-ceiling.md`); its kernel has not been read here.
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
- **Motion decides where the bands go, not the picture.** Dave C, 2023, to someone building a tool that
  splits a still screen into zones: *"deciding the ranges of vertical and horizontal motion determines
  when and where you would potentially need to reposition a sprite (unless you use a multisprite kernel
  … in which case you use one big zone for everything)"* 〔AtariAge `topic/346095`〕. A band boundary is a
  reposition, and a reposition is needed only where some object's range of motion ends — so static zones
  are designed from each object's motion range, which a still mock-up does not contain. **Cited only, not
  verified.**
- **One sprite across several zone kernels.** When the frame is split into mini-kernels run in sequence,
  a sprite crossing a boundary has to be carried from one to the next. A 2007 thread gives two ways: one
  Y counter and one graphics pointer shared by every kernel (Pitfall!'s — Harry starts in one kernel and
  continues in the next), or a counter and pointer per kernel, easier to follow but heavy on RAM. It names
  the symptom of getting it wrong — a sprite that turns into a big block at a zone boundary means its
  pointer was not updated between kernels — and a third route, batari's FlipDraw, which needs no line
  counter and can sit anywhere in several mini-kernels as long as the sprite data does not cross a
  page — or even if it does, when each kernel is synced with `sta WSYNC` — and its branches do not
  cross one 〔AtariAge `topic/112133`〕. **Cited only, not verified** — the thread is held here as distilled notes,
  not its text, and FlipDraw's mechanics are not in it; the same thread's last resort (precompute each
  line's `GRPx` into RAM) is the RAM-strip route above.
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
- DaveC's `landscape.asm` (`reference/files-dave/`) — the "zone" form studied here.
