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
  with `$8E`, computed in Python, not run in the engine.
- **Step on events, not per frame** (here every 30 frames) so gameplay pacing controls the draw
  rate; step extra times for "discard" rolls when you need decorrelation.
  How many: Thomas Jentzsch ran `ent` over the River Raid and Suicide Mission routines and found the
  serial correlation *"is divided by two with each iteration"* 〔stella-list `200110/msg00136`〕;
  B. Watson's run of Suicide Mission's gave 0.500001 (`200110/msg00128`), and River Raid's at one
  iteration was *"quite similar"*. Cited only, not verified. It is not a rule of every LFSR: this
  page's `$8E` sequence, computed in Python, gives 0.49, 0.24 and 0.00 for one, two and three
  steps per value, then −0.16 at five. Measure the generator you use.
- **Map, don't mod**: derive values by masking/offsetting (`and #$7F / adc #16` → X in 16..143).
  Masks keep the mapping branch-free and budget-friendly.
- **A range that is not a power of two: mask, then re-roll** what falls past the end. No fold of
  2ⁿ equally likely values onto 24 results can be uniform, since 24 does not divide 2ⁿ. Erik Eid's
  Euchre draws 0–31 and rejects 24–31 to shuffle 24 cards, after Thomas Jentzsch found that the
  earlier shuffle moved some cards less than others 〔stella-list `200209/msg00025`〕. The source in
  that message bounds the cost: after six rejections in one pass, a card whose number is rejected is
  not swapped. Cited only, not verified.
- **Reference-check the sequence**: the same LFSR in 5 lines of Python/Go gives
  `$5A → $2D, $98, $4C, $26, $13, …`; the scenario asserts RAM and the rendered marker X match
  exactly (verified: 45/61, 152/40, 76/92, 38/54 at spawns 1-4).

## Longer periods

Two bytes (batari, with supercat's design, AtariAge `topic/107010`):

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
frame until the player presses Reset. Euchre uses this routine and counts about 5.6 scanlines per
byte in its source 〔`200209/msg00025`〕, which is why it blanks the screen for the eight frames of
a shuffle. He looked at a linear congruential generator first and dropped it: the multiplier is a
number like 25173, and the 6502 has no multiply instruction. Cited only, not verified.

## Uses
- Enemy spawn positions/waves (starshot uses this for wave patterns).
- Terrain/maze generation: step per row/cell; bidirectional variants (Pitfall's left/right
  stepping LFSR) let you scroll both ways — documented in `docs/fundamentals-audit.md`.
- Scrolling banks (a river or road edge in PF): rather than rotating bits in a loop, batari
  indexes an 8-entry table of pre-shifted masks (`%00000001` … `%11111111`) with the LFSR
  `and #7`; one table serves the other side through `EOR #$FF` (cybergoth). A new shift every few
  rows rather than every row makes the edge bend one way for a while before turning back, instead
  of zig-zagging (seagtgruff). AtariAge `topic/103236`; Cited only, not verified.
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
  `msg00367`). His stated plan was an ENABL pattern that switches some stars off, its offset shifted
  with the field when it scrolls vertically (`200207/msg00394`). Cited only, not verified. The
  exerciser's starfield (`docs/exerciser.md`) thins by ANDing two LFSR streams, for density.
- Attract-mode variety with a frame-counter-mixed seed at game start (keep the *gameplay* seed
  fixed if you want reproducible worlds).
