# Technique — procedural generation (LFSR-driven content)

**Goal:** deterministic "randomness" for spawns/terrain: an 8-bit Galois LFSR stepped on game
events, with the guarantee that **the same seed reproduces the same world** — and that the
sequence is verifiable against an off-target reference implementation.

Demo: `roms/techniques/procgen_demo.asm` (a marker spawns every 30 frames at an LFSR-derived X).
CI: `scenarios/procgen_demo.json` (RAM state + on-screen X match the reference sequence, golden).
Hardware basis: `litmus_lfsr` (v0.46.0; period 255, never-zero, `eor #$8E` taps verified).

## The pattern

```
        lda lfsr        ; step (Galois, right shift)
        lsr
        bcc NoTap
        eor #$8E
NoTap:  sta lfsr
```

- **Seed once, non-zero** (zero is the lock-up state; period is 255).
- **To allow a zero seed, write `bcs` for `bcc`** (debro, AtariAge `topic/107010`; Cited only, not
  verified): 0 is then on the 255-step cycle and another value becomes the lock-up state — `$F4`
  with `$8E`, computed in Python, not run in the engine. Thomas Jentzsch's 2004 routine is this form
  with `eor #$b2` and still says *"initialize to non-zero at start"* 〔stella-list `200412/msg00044`〕;
  David Galloway answered that with `bcs` zero is in the sequence and 255 is not (`200412/msg00059`).
  The first half holds; the second does not: for `$B2` the value left out is `$DC` (computed in
  Python, not run in the engine). The value to avoid moves with the branch and the constant.
- **Or let the routine leave zero by itself.** Eric Ball puts a `beq` to the EOR ahead of the shift,
  so a zero state becomes the EOR constant and the ordinary sequence follows — two bytes, and the
  non-zero rule is gone 〔stella-list `200401/msg00251`; the same guard is in his 2001 routine,
  `200110/msg00131`〕. Manuel Polik's guard, on another routine, is a `bne` past `lda #$FF`
  (`200212/msg00001`). A guard built from EOR does the opposite: `eor #1` to keep a seed off zero
  makes zero when the seed is 1 (mldb), and the author of that code, just-jeff, moved to `ora #1`,
  leaving 128 odd seeds (AtariAge `topic/307790`). Cited only, not verified.
- **The lock-up can find the generator in a ROM you did not write.** Erik Mooney read two reported
  "frying" effects — Asteroids' rocks and saucers always starting from the same spots, Atlantis's
  ships all of one type flying one way — as *"almost certainly
  the random number routine getting stuck on producing all zeroes"* 〔stella-list `200504/msg00058`〕.
  Adam Wozniak, recalling that Pitfall! has 255 screens rather than 256 because it uses an LFSR,
  proposed setting its state to `$00` to get the same screen over and over, and for other games
  trying each RAM byte or pair at `$00` or `$FF`, since some keep the state complemented
  (`200504/msg00060`). A proposal; nobody in the thread reports running it. Cited only, not verified.
- **Step on events, not per frame** (here every 30 frames) so gameplay pacing controls the draw
  rate; step extra times for "discard" rolls when you need decorrelation.
  How many: Thomas Jentzsch ran `ent` over the River Raid and Suicide Mission routines and found the
  serial correlation *"is divided by two with each iteration"* 〔stella-list `200110/msg00136`〕;
  B. Watson's run of Suicide Mission's gave 0.500001 (`200110/msg00128`), and River Raid's at one
  iteration was *"quite similar"*. Cited only, not verified. It is not a rule of every LFSR: this
  page's `$8E` sequence, computed in Python, gives 0.49, 0.24 and 0.00 for one, two and three
  steps per value, then −0.16 at five. Measure the generator you use.
  Eric Ball: use the byte rather than just the carry and *"there are a couple of sequences where
  X(n+1) = X(n) * 2"* (his BASIC model shifts left; his 6502 routine, like this page's, shifts
  right, so each step that skips the EOR halves the value); he says shifting in a bit made from the
  XOR of several bits normally avoids it, *"but that requires more instructions"*
  〔`200110/msg00131`, corrected in `msg00132`〕. Our reading: that holds only when the routine is
  run eight times before the byte is read. River Raid's routine shifts in one such bit per call,
  so every call doubles the byte and adds a bit — the 0.5 above. Of `ent`'s other figures —
  entropy, chi-square, arithmetic mean, Monte Carlo π — Jentzsch called serial correlation *"maybe
  the most important test for us"*: *"a high serial correlation should be avoided in games,
  because that's something the human player will notice"* (`200110/msg00135`). Cited only, not
  verified.
- **Or step every frame, needed or not, when the game should not repeat.** The other choice from the
  one above: the player's timing picks the point in the sequence. SpiceWare's tutorial calls
  `Random` in VerticalBlank to *"impose an element outside of the Atari's control - namely the time
  it takes the human to do things"* (AtariAge `topic/273214`, `topic/159268`); in `159268` scitari
  reports that seeding from INTIM at start-up gave him the same seed in later games, and that
  stepping once per main loop fixed it. Kevin Horton gave the same advice in 2001 〔stella-list
  `200110/msg00121`〕, and noland suggests an extra step on each frame a button is pressed, before
  the game starts too (`topic/293694`). Against the assumption that a player cannot time a press to
  1/15 s, Erik Mooney reports hitting that window 90% of the time with a key repeat
  (`200110/msg00157`). Cited only, not verified.
- **Check the period of a routine you did not derive.** noland: *"applying multiple shift and XOR
  operations may actually shorten the series (period) produced"*, to *"just 40 values or so"*
  (AtariAge `topic/293694`); Cited only, not verified. A general warning: the routine in that
  thread, modelled in Python, does reach 255. Manuel Polik's guarded routine above has a longest
  cycle of 217 values (and cycles of 31 and 7 for other seeds), and River Raid's two (below) 217
  and 57,337. Computed in Python, not run in the engine.
- **Map, don't mod**: derive values by masking/offsetting (`and #$7F / adc #16` → X in 16..143).
  Masks keep the mapping branch-free and budget-friendly.
- **A range that is not a power of two: mask, then re-roll** what falls past the end. No fold of
  2ⁿ equally likely values onto 24 results can be uniform, since 24 does not divide 2ⁿ. Erik Eid's
  Euchre draws 0–31 and rejects 24–31 to shuffle 24 cards, after Thomas Jentzsch found that the
  earlier shuffle moved some cards less than others 〔stella-list `200209/msg00025`〕. The source in
  that message bounds the cost: after six rejections in one pass, a card whose number is rejected is
  not swapped. Re-rolling has no fixed cost, which is why TROGDOR would rather make the game need
  only powers of two; batari answered that an LFSR's sequence is finite, so its worst run of
  re-rolls is too — find it off-target — or else take one value a frame and act only on one in range
  (AtariAge `topic/159268`). reveng puts the worst case at the generator's width in bits and the
  average near two tries (`topic/333279`). For this page's `$8E` LFSR stepped once per try,
  `and #7` for 0–5 re-rolls at most twice running, `and #31` for 0–23 four times, and 0–128 from
  the whole byte eight times, about two tries on average (computed in Python, not run in the
  engine). Cited only, not verified.
- **Or scale, at a bounded cost.** `min + rand × (max − min + 1) / 256` is the high byte of an 8×8
  multiply, which a shift-and-add loop gives, stopping when the random byte's set bits run out —
  0 to 8 passes (robert-m, AtariAge
  `topic/110347`); Andrew Davie's `(rnd × N) >> 8` in `topic/333279` is the same, built from N
  additions. It is still a fold of 256 values, so by the line above not exactly uniform: 0–5 from
  the `$8E` LFSR's 255 values comes out 42, 43, 42, 43, 43, 42 times (computed in Python, not run
  in the engine). Cited only, not verified.
- **Reference-check the sequence**: the same LFSR in 5 lines of Python/Go gives
  `$5A → $2D, $98, $4C, $26, $13, …`; the scenario asserts RAM and the rendered marker X match
  exactly (verified: 45/61, 152/40, 76/92, 38/54 at spawns 1-4).

## Longer periods

Two bytes (batari, with supercat's design, AtariAge `topic/107010`; the same routine, one byte or
two by whether `rand16` is defined, in `topic/159268`):

```
        lda rand        ; rand shifts right, rand16 shifts left
        lsr
        rol rand16
        bcc NoEor
        eor #$B4
NoEor:  sta rand
        eor rand16      ; returned value; rand16 is state only
```

Any seed but both bytes zero. The thread gives period 65535; a Python model of these lines repeats
after 65,535 steps (computed, not run in the engine). supercat: with the halves shifting in
opposite directions any value can follow any other (except 0 after 0), where half of a plain 16-bit
LFSR allows only two next values; batari reports that the final `eor rand16` lowered the correlation
with the previous value. Cited only, not verified.

Four bytes, 31 bits (Erik Mooney 〔stella-list `199703/msg00296`〕): XOR bits 27 and 30, shift the
four bytes left one bit (`rol rand1` … `rol rand4`) with the new bit entering bit 0, and call it
eight times per random byte. He gives the period as 2³¹−1 and seeds it by generating one bit every
frame until the player presses Reset — *"whether you need it or not"*, so the register *"can have at
least 200 different values"* by then; he would not rely on the power-up state, *"especially because
that doesn't work on the emulators"*. Euchre uses this routine and counts about 5.6 scanlines per
byte in its source 〔`200209/msg00025`〕, which is why it blanks the screen for the eight frames of
a shuffle. He looked at a linear congruential generator first and dropped it: the multiplier is a
number like 25173, and the 6502 has no multiply instruction. Cited only, not verified.

Kevin Horton posted a four-byte register in 2001, with the taps the *TTL Cookbook* numbers 28 and
31, which he renumbers 27 and 30 counting from 0 〔stella-list `200110/msg00121`〕. Mooney, quoting
the taps and Horton's call-every-frame advice, replied that he had done exactly this in INV, with
a static seed, the bit every frame before start being enough (`200110/msg00125`). Cited only, not
verified. The lines as posted XOR bits 28 and 30, not 27 and 30 — two `rol a` where Mooney has
three `asl` — and that pair is not maximal: x³¹+x²+1 is reducible over GF(2) (computed in Python
from the posted lines, not run in the engine). Copy Mooney's, or count the shifts.

Two widths in one game: partway through disassembling River Raid in 2001, Thomas Jentzsch found
*"two quite simple LFSRs, one with one byte and one very similar, but with two bytes"*, and guessed
that the first served random objects and sound and the second the terrain 〔stella-list
`200108/msg00100`〕. Both take the (high) byte, shift it left three times, EOR it back in, shift
once more and rotate the new bit in; neither is maximal. A Python model of the posted lines gives
the one-byte routine a longest cycle of 217 (plus 31, 7 and a stuck 0) and the two-byte one
57,337 — the length of the River Raid sample Jentzsch later ran through `ent`
(`200110/msg00135`). Computed in Python, not run in the engine; the guess about which serves what
is Cited only, not verified.

## What generating does not save

A seed saves the ROM a stored map would take. It does not, by itself, save RAM.

- **What the player changes cannot come from the seed.** Reasoning from play in 1999, Jeremy C. Jack
  noted that Pitfall!'s treasure *"has to be stored apart from that function because once you snag
  a treasure, if you go back to the screen, it's gone"* 〔stella-list `199906/msg00117`〕. Our
  reading: the same holds for anything the player alters, and it is RAM on top of the generator.
  Cited only, not verified.
- **A generator that expands its output needs room for it.** Manuel Polik found that the 8-bit
  computer version he was reverse-engineering *"completely generates the dungeons out of seed values,
  relying on the giant amount of RAM of the 8-Bits"*; Mark Graybill answered that Apshai stored
  fairly large structures and Fargoal less, and that a 16-room level took about 80 bytes in the
  *Devil's Dungeon* format and about 104 in his own (computer games, against the 2600's 128 bytes;
  stella-list `200307/msg00169`, quoting Polik). Cited only, not verified.
- **The working copy is the update's reach, not the board.** For Conway's Life, against the
  objection that a second copy of the grid is needed, Erik Mooney: no cell affects a non-adjacent
  one, so the second copy can shrink to two rows of temporary storage 〔stella-list
  `200103/msg00096`〕. In the same message, packing the grid does not save RAM, because the kernel
  must still handle the worst case. Cited only, not verified.
- **Keep what changes in RAM and what does not in ROM.** In a 2001 Arkanoid thought experiment,
  Erik J. Eid priced 4 bits per brick (colour, hits, capsule) at 64 bytes for an 8×16 field
  〔stella-list `200108/msg00308`〕; Erik Mooney: *"half the 2600's RAM"*, and at 64 bytes a level,
  64 levels fit one 4K bank (`msg00315`). Eid's alternative: one bit per brick, read together with
  the original level table in ROM, which *"would prevent having to represent the entire level at
  one time"* (`msg00354`). A discussion; no game in the thread. Cited only, not verified.
- **Or pre-generate, and keep pointers.** Christopher Tumber, against generating an asymmetric
  playfield on the fly: *"So cheat. Use ROM."* Twenty five-row sections whose top and bottom rows
  fit each other, any four stacked, give 20⁴ maps for 600 bytes (500 with PF0 packed); a few groups
  with different seams give about 6×20⁴ in 4K; RAM holds only pointers to the ROM data
  〔stella-list `200405/msg00073`〕. *"Not random, but nobody's going to notice"*. Cited only, not
  verified.

## Uses
- Enemy spawn positions/waves (starshot uses this for wave patterns).
- Terrain/maze generation: step per row/cell; bidirectional variants (Pitfall's left/right
  stepping LFSR) let you scroll both ways — documented in `docs/fundamentals-audit.md`.
- Scrolling banks (a river or road edge in PF): rather than rotating bits in a loop, batari
  indexes an 8-entry table of pre-shifted masks (`%00000001` … `%11111111`) with the LFSR
  `and #7`; one table serves the other side through `EOR #$FF` (cybergoth). A new shift every few
  rows rather than every row makes the edge bend one way for a while before turning back, instead
  of zig-zagging (seagtgruff). AtariAge `topic/103236`; Cited only, not verified.
- A whole scrolling map from little ROM. Reading Thomas Jentzsch's River Raid disassembly in 2017,
  vidak was *"surprised at how little ROM data Carol Shaw used in setting up the entire map"* and
  meant to copy *"the idea of controlled randomness"* — while weighing the opposite, an entire
  playfield in ROM with bank switching, because ROM is cheaper now (AtariAge `topic/267694`). One
  reader's impression, not a byte count; the disassembly was not opened here. Cited only, not
  verified.
- **Random access: seed from the place, not the order.** A stepped LFSR reproduces a world only in
  the order it was generated, and the bidirectional form above only by stepping through the
  screens in between. In a 1999 thread Dan Knapp suggested computing the "random" choice so that
  *"a given set of coordinates will always yield a given one of the generic cells"*
  〔stella-list `199906/msg00099`〕; Amos Bannister agreed and added that *"the cell's
  co-ordinates could be the seed for a RNG"* for details such as trees, so *"someone visiting the
  same cell twice would see exactly the same thing"* (`199906/msg00100`). A proposal; no game in
  the thread. On AtariAge (`topic/297443`): seed an 8-bit LFSR with dungeon info EOR room
  coordinate EOR character seed and step it once, or use a 16-bit LFSR with the dungeon info in
  the high byte. When several bytes (a name) feed one seed, plain EOR clusters
  — letters, digits and symbols come to about 64 characters, 6 bits — so shift each position
  differently or hash with CRC-8. Cited only, not verified.
- Thinning a starfield so it stops looking regular. Thomas Jentzsch, on a Star Fire starfield that
  looked *"much to symmetric"*: vary the HMOVE values inside a frame or between odd and even frames,
  show only about every 4th star from different vertical offsets on odd and even frames, and give
  stars different colours 〔stella-list `200207/msg00349`〕. Manuel Polik answered that in his
  field the HMOVE change breaks vertical scrolling — stars sit 8 pixels further along on each line,
  so +8 is what moves the field up — and that colours depend on the cycles left (`200207/msg00364`,
  `msg00367`). The thinning has to repeat every frame, so not *"total randomness"*; he would not
  *"sacrifice any RAM for that"* and floated scanning *"bitwise somehow through a randomly chosen
  part of the ROM"*, or a sequence generator he feared would cost too much kernel time
  (`msg00364`). His stated plan was an ENABL pattern that switches some stars off, its offset shifted
  with the field when it scrolls vertically (`200207/msg00394`). Cited only, not verified. The
  exerciser's starfield (`docs/exerciser.md`) thins by ANDing two LFSR streams, for density.
- Why a field of single dots is affordable, and another way to draw one. Glenn Saunders, on the
  missiles: a starfield can fill the screen because it draws *"one dot per scanline"*, and *"you
  don't notice that so much because the stars have to be somewhat spaced out anyway"* 〔stella-list
  `200005/msg00169`〕. Starpath's Supercharger start-up screen uses the ball: 256 bytes of
  Supercharger RAM hold data that changes its colour and shift on every scanline, and most values
  give black, so the field appears to fall (Eckhard Stolberg, `200102/msg00351`). Cited only, not
  verified.
- Attract-mode variety with a frame-counter-mixed seed at game start (keep the *gameplay* seed
  fixed if you want reproducible worlds).
