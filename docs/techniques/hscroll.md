# Technique — horizontal playfield scroll (coarse 4px)

**Goal:** scroll a playfield pattern horizontally — the foundation of side-scrollers — verifiably,
at the natural 4-px (one-PF-pixel) coarse granularity.

Demo: `roms/techniques/hscroll.asm` (4px stripes, 32px period, scrolling in reflect mode).
CI: `scenarios/hscroll.json` (phase progression, reflect, 262, golden).
Lineage: studied from the legacy ATARI AR `Side-Scroll/scroll.asm`
(`reference/2600-technique-sources/sidescroll/`).

## The technique
- The PF pattern is stored as **8 precomputed phases** of (PF0, PF1, PF2) — each phase is the
  stripe pattern shifted by one PF pixel (the PF bit-order quirks — PF0 nibble, PF1 reversed,
  PF2 normal — are baked into the table so a clean shift falls out).
- Each scroll tick (every `scrollSpeed` frames) advances the phase by 1 → the stripes move 4px.
  `CTRLPF` reflect mirrors the left half, so both halves scroll symmetrically.
- Per scanline the kernel just holds the current phase's PF (vertical stripes).

## Verified
read_row shows the stripe edges advancing 4px per scroll tick (e.g. edge 28→24 across one tick);
reflect on; phase variable progresses 0→7 wrapping; 262 lines; golden-pinned.

## Notes / variants
- **1-px fine scroll** needs bus stuffing or asymmetric per-line PF rewrites (candidate, harder) —
  the AtariAge "Bus Stuffing Demos" (index-forum50.csv) is the route, noted in the source comments.
- For scrolling *graphics* (not stripes) the phase table generalizes to any 40-bit pattern; a
  longer level scrolls by streaming new columns into the table edge.
- Vertical scroll is independent (shift the row pointer — see bitmap48's window).
- **Hiding the 4-px step — three routes on record, none measured here.** *Space:* cover the moving
  edge with an object — `known-traps.md`'s *smooth horizontal PF scroll* row ("use ball/missile edge
  or delayed/tile scroll"). Glenn Saunders proposed a version of this as a guess about the HERO /
  Seaquest / Megamania energy bars (*"This may not be how it works"*): on a non-reflected playfield, a
  *"filler"* object the same colour as the playfield — *"This could even be the ball sprite if you make
  it quad width"* — positioned to fill the gap of 1 to 3 pixels. He added that it *"COULD be used,
  theoretically, for a SMOOTH HORIZONTALLY SCROLLING playfield"*, at the cost of sprites and *"for very
  simple backgrounds"* 〔`200009/msg00007`〕.
  *Time:* step often enough — Thomas Jentzsch: *"you can create the illusion
  of smoothness. You just have to make sure that you scroll at least at 30Hz, even if it is 4 pixel at
  a time"*; mr-sql named KC and StarBlitz as scrolling *"at 30 FPS like Television"*. *Speed:* reveng
  on Thrust: *"the snap scroll used in Thrust is pretty much the state of the art, if a game can
  accommodate a non-continuous scroll. The speed of the eventual scroll masks the granularity of the
  PF pixels nicely."* 〔AtariAge `topic/249185`〕 **Cited only, not verified** — none of those games
  was run here, the filler object was only proposed in the thread, and the 30 Hz threshold is a
  statement, not a measurement.
- **Or do not scroll the playfield at all.** Glenn Saunders drew the line between *"scrolling with
  playfield graphics in tow"* and *"just sprites acting as background ala Stargate"*
  〔`199912/msg00009`〕, and later: *"Stargate is a good example of smooth horizontal scrolling using
  sprite elements for the background. This has obvious limitations, but it's doable."*
  〔`200104/msg00062`〕 **Cited only, not verified** — Stargate was not run here.
- **Half-screen tiles, chained by edge height.** Christopher Tumber's tile-based coarse scroller,
  *"similar to the one in Vanguard so it's a little jerky"*: *"Each tile is half a screen in width"*,
  21 of them (numbered 0 to 20), grouped by where the left edge starts and the right edge ends (low,
  middle, high) *"so that tiles can be strung together to create a smooth terrain"*. He had meant it to
  become a smooth scroll, *"but that's not really practical with this implementation"*, and had since
  come up with a different, smooth one 〔`200301/msg00018`〕. **Cited only, not verified.**
- **A four-way layout, as one author sized it.** gemintronic: *"In my 4 way scrolling engine each
  element stores 8 horizontal playfield pixels. Since I can only address 256 elements that makes my
  levels 16 rows deep and 128 columns wide"* — 16 elements of 8 pixels per row (our arithmetic) — and
  the next problem he names is joining one 256-element array to the next. In the same thread mr-sql
  suggested *"an x,y addressable camera object so you can easily pan the playfield around"*
  〔AtariAge `topic/221625`〕. **Cited only, not verified.**
- **Two slopes in one PF byte: AND, then EOR.** Thomas Jentzsch's horizontal-scrolling cave combines
  segments of different gradients inside the kernel, with a minimum segment width of 7 pixels *"so that
  there are no more than two segments in one PF block"*. *"Simple ORing or ANDing didn't work"*: the
  kernel loads the first segment's byte through a pointer, ANDs the second segment's, EORs a value
  (*"EORing the result when necessary"*) and stores to the PF register. The work-in-progress used no
  PF0 and four gradients 〔`200306/msg00114`〕. **Cited only, not verified.**
- **Hide the side edges under black playfield.** Manuel Rotschkar, on the first day of a project:
  *"In order to move sprites smoothly in and out of the screen on the sides, I'll simply have 2 black
  PF pixels on both sides of the screen. Then I can always scoll one sprite *under* this and after 8
  pixels I swap my sprite pointers accordingly."* 〔`200312/msg00184`〕 A plan, not a result.
  **Cited only, not verified.**

## Using the bit order instead of tabling it away (cited, 2026-10-05)

The eight-phase table bakes the PF bit order in — PF0 `D4..D7`, PF1 `D7..D0`, PF2 `D0..D7`, all 20
columns measured by `TestEveryPlayfieldColumnLandsWhereTheTableSays` (repeat mode). The list also used
the order directly. **None of what follows was run here**; the column order it is read against is
measured, and the reflected-mode readings additionally rest on the mirror rule, which that test does
not measure column by column.

- **One value into all three registers.** Thomas Jentzsch, answering a complaint about the reversed
  middle register: *"The reversed middle register can be used to draw nice looking asymetrical (and
  scrollable) upper and lower borders, simply by putting the same value into all 3 PF registers (like
  in Vanguard, Laser Gates, Exocet etc.). This makes the kernel a _lot_ simplier."* — and *"I don't
  know, if that was the original intention."* 〔`200109/msg00392`〕 Our reading: one byte is drawn
  LSB-first twice (PF0's upper nibble, PF2) and reversed once (PF1), so a single value gives an uneven
  20-column edge with no table. **Cited only, not verified.**
- **Rotating in place, repeat mode.** Erik Mooney's INV moves an asymmetric repeated-mode playfield
  through both copies of all the PF registers (labels `MoveEachRow0` / `MoveEachRow1`): *"The only
  tricky part is moving bit 7 of PF2 into bit 4 of the second copy of PF0"*. His code ROLs the PF2
  byte, carries the bit into bit 3 with four ROLs, ORs in the second PF0 byte and ROLs once more so it
  lands in bit 4; he then posted a shorter form that tests the carry and ORs bit 3 in directly
  〔`200405/msg00114`〕. This agrees with the measured order — column 19 is PF2 `D7`, column 20 the
  second copy's PF0 `D4`. **Cited only, not verified.**
- **Rotating in place, reflected mode.** Manuel Rotschkar (the same cybergoth address as the 2002
  routine in the next section), for *"scrolling a full width landscape, if the PF is in reflected
  mode"*: after ROR PF0Left, ROL PF1Left, ROR PF2Left, *"just continue"* with ROL PF2Right,
  ROR PF1Right, ROL PF0Right — a palindrome over six RAM bytes 〔`200405/msg00102`〕. Our reading
  against the measured order: every one of the six moves pixels left, and for the carry to hand
  each pixel to its left neighbour they must execute from PF0Right back to PF0Left — the reverse of
  the order as quoted, and the order his 2002 RotateLeft uses (PF2, PF1, PF0). **Not verified.**
- **Two lookup tables for four writes.** Schwerin, for a scrolling maze, PF1 and PF2 in reflected mode
  (*"PF0 is never updated"*): *"each register is affected by exactly 2 flags. We only need two lookup
  tables"* — PF1 and the right-hand PF2 share one, PF2 and the right-hand PF1 the other, each reached
  through a zero-page pointer set once per frame to the table plus `shift*4` 〔`199902/msg00019`〕. A
  design posted for comment, not a running kernel. Piero Cavina in 1997 absorbed the order in the
  data instead: 16 tiles per row stored as two bytes, *"the order of the bits was choosen so that they
  could be easily put into the playfield control registers"*, split with alternate-bit AND masks, a
  blank column between tiles, PF0 unused 〔`199704/msg00133`〕. **Cited only, not verified.**
- **Reflected PF1/PF2 repeat every 16 columns.** Manuel Rotschkar: with the playfield reflected the
  registers read *"u r u r u r"* (unreversed / reversed), *"So if you use PF1 and PF2 _only_, you have
  "r u" repeating twice!"* — so *"you should get away with one 16 byte AND table"*, indexed by
  x/4 − 4 with `AND #$0F`, and *"you should be able to erase the "right" block horizontally for _any_
  possible position"* (his use: clearing a destroyed block) 〔`200403/msg00317`〕. His follow-up the
  same evening ends *"its all untested brainstorms from me"* 〔`200403/msg00318`〕. The alternation
  follows from the measured order. Our reading: the table as posted clears `D0` at index 0, while
  the measured order puts PF1 `D7` at column 4, so check its direction against `litmus_pf_allcols`
  before using it. **Not verified.**

## The eight-phase table is a periodic stripe, not a rotation (2026-09-07)

It looks like this technique spends 24 bytes of ROM to avoid a runtime rotation — Manuel Polik posted
one in 2002, alternating `ROR`/`ROL` across PF2/PF1/PF0 with a wrap at the end 〔`200209/msg00126`〕.
**The two are not the same operation.**

Read out of `hscroll.asm` and reassembled into the twenty bits as the beam paints them (PF0 `D4..D7`,
PF1 `D7..D0`, PF2 `D0..D7`), measured by `internal/emu/hscrollphase_test.go`:

- each phase **is** the previous shifted left by one, and
- the bit shifted **in** alternates — `0,0,0,0,1,1,1` across the seven steps — so it is not fed back
  from the bit that left, which is what a rotation does;
- rotating the twenty-bit ring by eight does **not** return phase 0;
- the stripe's period is **8**, and **8 does not divide 20**.

★So the table holds **eight phases of an eight-periodic stripe**, not eight rotations of a playfield.
A true ring rotation repeats after twenty steps; this repeats after eight, which is why `and #7` is
right here and would be wrong for an arbitrary picture.

★★**The trade is therefore not the one it looks like.** 24 bytes buys eight phases of **one periodic**
pattern. An arbitrary twenty-bit playfield tabled the same way needs twenty phases — **60 bytes** — or
a runtime rotation and its cycles. Anyone simplifying this table into a rotation gets a different
picture, silently, for every pattern whose period does not divide 20. Raised by the mailing-list
distillation (helper-1) as a cycles-versus-ROM question; the answer is that the two sides are not
doing the same thing.

## The trade, as it was once counted (cited, 2026-10-05)

The other side of that question was counted once on the stella list — for scrolling *text* drawn
with players, not the playfield, so it bears on the trade and not on this kernel (the first three
bullets); the last bullet is from AtariAge:

- John K. Harvey's text cart stored the text pre-shifted. Kurt Woloch, after disassembling it:
  *"every bit that has to go through the scrolling process requires a whole byte"*, using *"only 5
  bytes off the VCS's internal RAM"* 〔`199906/msg00103`〕. Harvey's own count: *"24 bytes of $00s on
  each side of the text"* so that it scrolls offscreen, and *"about 96 characters"* in 4K
  〔`199906/msg00091`〕. A viewer: *"the gap where the words scroll in is just a bit short"*
  〔`199906/msg00129`〕. Two readings, neither checked: the 24-byte padding, or the narrow display
  window — the same post quotes Harvey asking whether a 6-digit display would be better than his
  4-digit one.
- Eckhard Stolberg built the other side — scrolling *"with ASL/ROLs as Kurt described it"* behind a
  six-sprite display: *"3045 characters"* at single-scanline resolution, 3004 at double, made because
  single was *"a bit hard to read on a real VCS"* 〔`199907/msg00007`〕. About 31 times Harvey's
  count (our arithmetic; Stolberg does not state his cartridge size in that post).
- Kurt's proposal updates the buffer *"while the beam is offscreen"* 〔`199906/msg00102`〕. Our
  reading: the usual reason to precompute — cycles inside the beam race — does not bind while the
  rotation fits in the blank; whether it fits is the measurement to make.
  The buffer as he laid it out: seven RAM bytes per scanline of the letters, six shown like a 6-digit
  score and the seventh (`$96` in his example) *"not visible"*, shifted with one instruction per byte
  rather than a loop — `ROL $96,X` / `ROL $95,X` / `ROL $94,X` and on, *"where X would be decreased
  by 7 after each line"* — *"56 bytes of RAM"* for 8-pixel-high letters; he was unsure of the opcode
  (*"I don't know if ROL was the right command, but I think so"*) 〔`199906/msg00102`〕. Our reading:
  each `ROL` hands the byte's top bit through the carry to bit 0 of the byte on its left, so that
  line moves one pixel, and the hidden seventh byte is where the next character comes in — the part
  the C-64 intro he analysed gave to an offscreen buffer refilled every 8 shifts. Four days later he
  added that with *"ROLling"* the zeros on each side *"could maybe"* go too 〔`199906/msg00123`〕.
- **Every rotation from one table, when what rotates is whole bytes.** shazz rotated an 88-entry
  colour list in RAM each frame (one colour per scanline) and was at *"still 1222 cycles"*, *"16
  scanlines"*. SeaGtGruff: *"You don't need to double the ROM table, just almost-double it. If you make
  the table 88+87 = 175 bytes (or 2N-1)"*, the first 87 bytes repeated after the 88, *"then you can get
  every possible rotation depending on your starting index value"*; he credited the idea to enthusi's
  earlier loop, adding the table setup 〔AtariAge `topic/215618`〕. Our
  reading: this turns a per-frame rotation of a byte list into a choice of start index; it does not
  shrink the 60-byte table above, because a playfield rotation moves bits inside bytes.

**Cited only, not verified** — the quoted figures are the posters' own (only the ratio is ours), and
no cartridge was run.
