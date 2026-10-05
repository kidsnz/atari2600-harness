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
  **What it cost one game, itemised — and why he declined it** (Manuel Polik, 2001, on Bob Colbert's
  multiplexer for the bullets of Gunfight 2600). RAM first: eleven arrays of `MAXSPRITE + 1` bytes, so
  *"he uses 22 bytes for the first sprite, another 11 for every additional one. For six bullets this would be
  77 bytes, which is already more than I have, even without any shootable obstacles"* 〔stella-list
  `200103/msg00190`〕 — 11 × (6 + 1) = 77, so the "+1" is a slot beyond the sprite count (our reading).
  Glenn Saunders answered that it is *"just one example of sprite multiplexing"*, pointing at Solaris (no
  RAM in the cart) and, he thought, Stargate 〔`200103/msg00197`〕. Two days later Polik listed his reasons not
  to merge it 〔`200103/msg00244`〕: the routine *"can do a repositioning every 4th line, i.e. one would come
  down to 1/4th of the vertical resolution for positioning"*; *"at least two additional tables"* of RAM (the
  sorted vertical positions and per-sprite flags); he did not think he could also paint *"2 sprites with two color
  changes + 2 PF values"* while repositioning; the kernel ROM — *"I assume that'll at least quadruple"*; and
  it was *"eating nearly all the time of the vertical blank"* and the overscan, which with the RAM held him
  to six bullets. Colbert's reply: *"You can double or triple your overscan cycles by dividing your code
  among frames"* — he ran his across three, and *"even reading the joystick every 3 frames was still very
  responsive"* 〔`200103/msg00248`〕. Polik's own test of where it fits: *"a non-flickering player vertical
  separated from all other objects floating around"* 〔`200103/msg00251`〕. A month later he put the scaling
  in two lines: *"Every bullet added would eat up some 5 or more Bytes in the RAM"* and *"Every bullet added
  would add lots of cycles in the sort routines, it'd add more than a linear function"* 〔`200104/msg00058`〕.
  **Cited only, not verified.**
  **A blind spot at the top edge** (Polik again, 2002, a flicker multiplexer for ships that enter from
  the top): *"I don't get a smooth transition done between the point where still the half sprite is done
  and where finally the kernel kicks in. Any object starting at a certain range inbetween these two points
  get's invisible. So there's a blind spot for any ship of any size"* 〔stella-list `200210/msg00037`〕. He
  was *"only able to shift that blind spot"*; the attempt cost *"~100 bytes"* of ROM and *"~ 5 scannlines"* and
  *"still looked crappy"*, so he discarded it 〔`200210/msg00030`〕, and the *"2 extra lines for the kernel
  to determine wether the next ship is ready for repositioning"* he took to be part of it 〔`200210/msg00032`〕.
  Thomas Jentzsch disagreed: *"I still see no technical reason, why you can't show objects that startt
  before the top"* 〔`200210/msg00035`〕 and *"Many 2600 games have proven that this shouldn't be such a big
  problem"* 〔`200210/msg00038`〕. He posted the source with the problem still in it 〔`200210/msg00052`〕, and the
  thread ends with the question open. **Cited only, not verified.**

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
- **Without reuse, positioning can stay out of the visible lines.** Sohl: *"If you don't have
  to reuse any of the movable TIA objects as multiple game entities, you can do all of the positioning
  when the CRT beam is above (VBlank period) or below (Overscan period) the visible portion"*; reusing
  one as a different entity at a different horizontal position puts the repositioning in the visible
  portion, the cost in the entry above (AtariAge `topic/337214`). **Cited only, not verified.**
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
  line's `GRPx` into RAM) is the RAM-strip route above. For bullets crossing a band boundary, see "Objects that cross a
  boundary" below.
- **Flicker** is the accepted way past the 2-per-line wall: alternate which objects get P0/P1 each frame; a
  priority counter gives the longest-unshown object precedence so motion stays legible.
- **2-line kernel** is usually worth it (CPU headroom for game logic); cost is half vertical sprite resolution.
- **Page alignment** of the kernel and the HMOVE table matters (a mid-loop page cross adds a cycle and shears
  the picture); a timer (`TIM64T`) keeps VBLANK stable regardless of per-frame work.
  **Where the kernel sits in the source follows from that.** Christopher Tumber, 2003, on how 2600
  sources are laid out: because *"the display kernal often needs to be page aligned for timing
  purposes"*, it is often put after the data or as the very first code, *"So that <display kernal> can
  be anchored (with an ORG or because it's the very first part of code) so that it doesn't move around
  every time you add/remove new code. Game data (bitmaps and such) are often also anchored in this way
  for the same reason."* Those layouts call the kernel with `JSR`; he himself kept the code linear
  because of *"unnesessary JSR/RTS combinations (12 wasted cycles!)"*, and says the anchored layouts
  are *"probably a lot more common, particularly among hombrewers"* 〔stella-list `200304/msg00089`〕. The 12 is
  `JSR` 6 plus `RTS` 6, held by the static budget prover in `internal/cyclebound/jsrplacement_test.go`; the anchoring advice
  is **Cited only, not verified**.

## Band boundaries and kernel splits — cases from the list and AtariAge (the cases themselves not measured here)
- **Objects that cross a boundary.** Manuel Polik, 2001, weighing a sideways Gunfight cut into five
  horizontal segments: *"You'd have to do some repositioning whenever reaching a segment border. Now - how
  display the bullets, when they cross the border?"* 〔stella-list `200102/msg00326`〕. Glenn Saunders pointed
  at Air-Sea Battle and Canyon Bomber 〔`200102/msg00329`〕; Polik's reading of the first: *"The bullet from
  Air Sea battle"* jumps *"in a certain patern and it's three or four pixels wide, so ENAMx is just hit right
  before & after the repositioning"* — not open to him, *"since I'd have two sprites to reposition every
  segment, needing at least four complete lines to reposition"* — and *"Canyon Bomber IIRC doesn't need any
  repositioning of sprites"* 〔`200102/msg00330`〕. The thread leaves it open; carrying one sprite across zone
  kernels is the "One sprite across several zone
  kernels" entry above. **Cited only, not verified.**
- **Never reposition: `HMOVE` every line.** Glenn Saunders, 2005, asking how to scroll two groups of stars
  in opposite directions, guessed the scroll itself works *"similar to how Combat animates, via applying
  movement every frame without resetting position, and that will take care of the wraparound, so the only
  challenge is the repositioning in mid-screen"* 〔stella-list `200508/msg00154`〕. Thomas Jentzsch: *"Maybe
  you shouldn't repostion inside the kernel at all. If there is enough vertical space between the stars of
  one direction, you could just HMOVE every scanline. IIRC Starmaster does that."* 〔`200508/msg00157`〕
  No reposition means no reposition line, and the wrap comes from the motion; the price is the vertical
  spacing (our reading). Three years earlier he described the same game's stars as a split: *"The stars in
  Starmaster are also separated into upper and lower parts and you hardly notice that. The big difference
  is, that the border between those parts is slightly movable up and down"* — answering Polik, who found
  Star Raiders' upper/lower split left its sprites *"trapped"* in their part 〔`200207/msg00336`〕.
  The two descriptions are not reconciled here; Starmaster's kernel has not been read. **Cited only, not
  verified.**
- **A playfield drawn in blocks quantises the sprites.** Nick S Bensema, 1997, on a loop that loads the
  playfield once per 8-line block and then only waits on `WSYNC`: it *"will make it difficult to add
  players, unless you want your players to only be positionable every 8 scanlines or so, as Centipede and
  both versions of Frogger do"*, and *"Instead of spitting out eight WSYNCs, you could use that time to
  position or draw a sprite. Perhaps there isn't time to do both if you want the full eight scanlines."*
  〔stella-list `199703/msg00182`〕 **Cited only, not verified** — those games' kernels have not been read.
- **The block height is the time budget.** Chris Wilkson, 2003, to a kernel whose wall bricks are 11 lines
  tall: *"if your blocks are 11 scanlines tall, then you can break the processing into 11 pieces and do a
  little bit on each line (of course you have to do if for both players). So...for each individual block,
  you effectively have 5.5 scanlines to use, minus the processing for the bats and the ball"*; and,
  tentatively, merge the two walls into alternating bits of one byte, so *"you only have to fetch and rotate
  one byte per level instead of 2"* 〔stella-list `200309/msg00055`〕. The asker went the other way, to RAM:
  *"given my simple game concept, RAM and ROM are cheap compared to kernal time"* 〔`200309/msg00067`〕 — the
  thread the RAM-strip price above quotes. **Cited only, not verified.**
- **Ragged band heights.** Piero Cavina, 1998, for a ship that sinks one line at a time, where a four-line loop
  body would need an exit test on every line: *"break the kernel that does the ship body in two
  parts: one for the upper part, made only of groups of four lines, where you won't have to care of "exit
  points", and one for the scrolling-end, where you'll draw 1,2 or 3 lines only, according to the sink
  level. Maybe you won't have time for sprites in these last lines, but the various elements might be
  arranged so that this is not a big problem"* 〔stella-list `199806/msg00085`〕. He posted it rebuilt that
  way the same day, *"in groups of 4 scanlines, plus from 1 to 3 final scalines"* 〔`199806/msg00088`〕.
  **Cited only, not verified.**
- **One crowded line gets its own kernel.** Christopher Tumber, 2002, for a display whose shots run to a
  crosshair line at the horizon: split the kernel into the top half, *"the crosshair's scanline only (and
  enemy ships)"*, and the bottom half — *"This should free up a bunch of cycles on crosshair scanline"*, and
  because *"the shots terminate at the horizon … you could treat the upper and lower halves of the screen as
  completely seperate entities as far as the missiles go"*. The cost: *"later changes to the kernal more
  difficult since you have to change all three parts"*; a gain: *"you could just push new values into
  COLUPF only on that one scanline"* 〔stella-list `200210/msg00066`〕. The author's reply cut both ways: *"elegance and
  ROM space certainly are already very good reasons, plus add that I like the current solution"*, and of the
  split itself, *"This sounds like more economic way of doing it. At least it'll be less than 1/2 K"*
  〔`200210/msg00068`〕. The mechanism, self-contained zone routines, is `rts-dispatch.md`'s; the motive here
  is one line's budget, not the number of zone types. **Cited only, not verified.**
- **Hiding the transition.** Thomas Jentzsch on his game Bottom: *"The game kernel is split into five
  vertical stripes (top power-ups, top obstacles, road, bottom obstacles, bottom power-ups) and transitions
  between the stripes. Between the stripes, several player and missile repositionings are required. These
  happen during the transitions. The code makes sure that there are no visible gaps between the stripes and
  no visible HMOVE blanks (often using "early HMOVEs")."* 〔AtariAge `topic/343591`〕 How an early `HMOVE`
  hides the blank is not in the post; the shifted `HMxx` table it brings is in `docs/known-traps.md` (the
  cycle-73/74 `HMOVE` row). **Cited only, not verified.**
- **Reposition on the lines that have time.** boutell asked on AtariAge whether Ms. Pac-Man, whose dots are
  playfield, draws the dot lines with an asymmetrical playfield and the rest symmetrical, repositioning
  sprites *"only on the repeated symmetrical playfield scanlines"*. Dennis Debro: *"I haven't totally
  disassembled Ms. Pac-man but from what I've seen you're right. The dots are an asymmetrical PF and the maze
  is symmetrical. I haven't gone as far as seeing where and how the positioning is done … but looking at the maze
  resolution you seem to be right on"* — so the playfield split is reported from a partial disassembly, and
  where the repositioning happens is his inference from the maze, not something he found in the code. Thomas
  Jentzsch's alternative for a playfield
  that must be asymmetrical everywhere: *"Striped playfield graphics like in Dig Dug, Mr. Do or Thrust may be
  an option for you"* and *"very efficient repositioning code"* 〔AtariAge `topic/56658`〕. **Cited only, not verified.**
- **One object, five times — and then not.** Erik Mooney's Space Invaders (the game quoted above; that
  passage is the later two-scanline kernel), the 1997 alpha: *"the ball is used for the invader bombs, and it
  can be recycled up to 5 times to display 5
  bombs simultaneously (as always, no two bombs in the same vertical zone.) Everything is displayed every
  frame"*; a bomb due to start on an invader line was skipped for that frame, *"because there's no time to
  set RESBL during a scanline in which I'm writing to the playfield registers six times"* 〔stella-list
  `199704/msg00061`〕, and repositioning the ball *"needs a full scanline"* 〔`199704/msg00066`〕. Two days
  after the alpha he dropped it: *"I rewrote the kernel to not reposition the ball (bombs), so it can only
  handle one bomb at a time, or two on alternating frames"*, and *"The kernel now
  barely fits within two scanlines within the invader block"* 〔`199704/msg00102`〕. **Cited only, not verified.**
- **A kernel version per horizontal half.** Glenn Saunders, 2001, with Y the line counter and X the
  playfield index, found it *"hard to do all the writes bunched up close
  together"* (the `GRP0`/`GRP1` writes and the missiles' motion and size writes), and proposed: *"I could have
  different versions of the kernel that update at different spots depending on the X position of
  the objects in order to avoid stomping on the sprite while it's being drawn. If I evaluate the screen as
  two halves and have two different kernels depending on whether the sprite is on the left or right half,
  then that's 16 possible combinations if done for all 4 objects"* 〔stella-list `200110/msg00185`〕. Thomas
  Jentzsch: *"that's a nice idea and it should work"*, but optimise the single kernel first — *"And maybe you
  won't need all 16 combinations, maybe 2 or 4 a sufficient"*; and on how much time a playfield border buys,
  *"3 pixels give you 1 cycle, so 4 playfield pixels (=16 pixels) will give you about 5 cycles"*
  〔`200110/msg00201`〕. **Cited only, not verified.**
- **How many objects one line can test.** Ruffin Bailey, 2002, testing two players and two missiles by Y on
  every line: *"Though skipDraw is quick, you unfortunately still can't get in four checks per scan, much
  less four checks per line with writes to the ENAMx's and GRPx's"*. His alternative is Kirk Israel's buffer, which
  he adopts — a RAM byte per object, loaded into `GRP0` at the start of the line and refilled for the next
  line before `WSYNC` 〔`200207/msg00037`〕: *"LDA from zero page takes 3 cycles and STA into zero page also
  takes 3. That's 6 per object checked, making 24 for four objects. Since I can "carry over" one value from
  the preceding scan line, that's 24-3 = 21"* 〔stella-list `200207/msg00041`〕. Kirk Israel: *"Why only one
  "carry over"? … you should be able to populate both A and X with a value"* 〔`200207/msg00046`〕, which
  would make it 18 (our arithmetic). Here skipdraw is measured at 17 or 20 cycles from `WSYNC` to the `GRP0`
  store for one object on one fixture (`docs/fundamentals-audit.md`), so four would be 68–80 of the line's
  76 before anything else (our arithmetic); `sta GRP0` at three cycles is measured in
  `internal/emu/ramstrip_test.go`. The buffer budget is **Cited only, not verified.**
- **A frame as a stack of kernels.** ZackAttack, 2023, proposing a framework for the ELF support of
  UCA-based cartridges: *"Each frame can be composed of one or more display kernels. Frames and kernels would
  be created once at the start. Each kernel will have a set of functions that can be used to change its
  appearance"*, and *"The kernels would stack vertically to produce a full 192+ lines of visible screen"*
  〔AtariAge `topic/347047`〕 — pseudocode only in the thread. On a stock cartridge the same composition, with
  the zone order as data in RAM, is `rts-dispatch.md`. **Cited only, not verified.**

## Cycle-level craft (verified in our build)
- HMOVE table placed so the lookup on the positioning line ALWAYS crosses a page (`LOOKUP = TABLE_END - 256`,
  negative index), which makes the read a constant 5 cycles whatever the remainder. This line used to say the
  placement *avoids* a page-cross; it does the opposite. After the `sbc #15` loop `Y` is 241–255, and in the
  assembled `zone_multiplex.bin` the read is `lda $EFF1,y` (table end `$F0F1`), so every effective address is
  `$F0E2`–`$F0F0`, one page above the base: always crossed, so always the +1 — measured for an `abs,Y` read
  that crosses in `internal/emu/ramstrip_test.go` (`lda Cross,y / sta GRP0` = 8 against 7 uncrossed). It holds
  while the table's end sits at least 15 bytes into its page. The same move,
  named as such, is in the `bzoneRepos` routine on the list: *"Consume 5 cycles by guaranteeing we cross a
  page boundary"* 〔stella-list `200506/msg00061`〕. **Not verified** by a cycle trace of this kernel — read
  from the binary and the page-cross rule.
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
