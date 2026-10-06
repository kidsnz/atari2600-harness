# Technique — multicolor 48px graphic (per-row color)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** the 48px-wide image (the standard 3×NUSIZ + P1 offset + VDEL technique used by
`score-kernel` / `bitmap48`) but **multicolored vertically** — `COLUP0`/`COLUP1` are rewritten
every scanline from a color table, so a single 48px graphic spans many colors top-to-bottom
without any extra sprites.

Demo: `roms/techniques/multicolor48.asm` (a rainbow-banded heart icon, 48×16).
CI: `scenarios/multicolor48.json` (positions 87/95, two distinct per-row colors mirrored to RAM,
262 lines, golden).
Lineage: AtariAge topic/209137 (SeaGtGruff's 76-cycle multicolor 48px kernel) laid on top of the
hardware-verified 6-store choreography (`litmus_48px6`, v0.52.0).

## The technique

**48px construction** is unchanged from the family: `NUSIZ0=NUSIZ1=$03` (3 copies close),
`VDELP0=VDELP1=1` (double-buffer), P0 reset then P1 reset + `HMP1=$10` (P1 nudged +8px right) so
the six 8px slices abut into one 48px band. Positions land at **P0=87 / P1=95** (same prologue as
score6: litmus recipe + SLEEP 21).

**Per-row color = one table fetch in HBLANK.** The color rewrite is squeezed into the front of the
line, before the first GRP store, so it costs nothing in the visible store window:

```
Krow:   sta WSYNC
        ldy row        ; 3
        lda ColorTab,y ; 7
        sta COLUP0     ; 10
        sta COLUP1     ; 13   ← color set for this row (still in HBLANK)
        lda (p0),y     ; 18   sta GRP0 ; 21   B0
        lda (p1),y     ; 23   sta GRP1 ; 26   B1
        lda (p2),y     ; 31   sta GRP0 ; 34   B2
        lda (p3),y     ; 39   sta tmp  ; 42
        lda (p4),y     ; 47   tax      ; 49
        lda (p5),y     ; 54   tay      ; 56
        lda tmp        ; 59
        sta GRP1 ; 62  stx GRP0 ; 65  sty GRP1 ; 68  sta GRP0 ; 71 (value unused, write required: it copies GRP1 into P1's delayed register)
        dec row        ; 76
        bpl Krow       ; runs 3cy into HBLANK, halted by next WSYNC
```

**Per-line budget ≈ 73 cycles of work** before `dec row` completes at cycle 76 (the WSYNC
boundary); `bpl` spills harmlessly into the next HBLANK where the next `sta WSYNC` re-syncs. One
row = exactly one scanline, no overrun — confirmed by the frame staying at 262 lines (an overrun
would push it to 276). The 10-cycle color burst (`lda/sta/sta`) lives entirely inside HBLANK, so
the four GRP stores still complete at 21/26/34 then 62/65/68/71 — the same gap relations as the
monochrome kernel, just shifted +7 cy vs score6 because of the inserted color fetch.

**Why the stores cannot be moved earlier.** With two plain sprites a GRP write only has to land
before the beam reaches the object; here it cannot. azure: *"if you're dynamically repeating
sprites with NUSIZ0/NUSIZ1, such as using the 48 pixel trick, writing registers early doesn't work.
Each write must occur precisely within a narrow window of time, because the kernel is playing a
shell game with the GRP0 and GRP1 registers. That's the cost of using two sprite registers to
simulate six sprites."* And: *"If you insert a nop in the middle of the 48-pixel sprite algorithm,
you'll see rendering artifacts"* (AtariAge `topic/282244`). `litmus_48px6` measured that one
placement works — a different kernel, stores at cycles 34/37/40/43, all 48 bits matching
(`verified-coverage.md`); how narrow the window is, and that shifting the stores breaks it, was not
run here — **Cited only, not verified**. The window for an ordinary write, cycles 0–22, is
`fundamentals-audit.md`'s.

**The invariant the store order keeps.** Erik Mooney, working the routine out from Okie Dokie's
source in 1997, called each player's two graphics registers GRP0/GRP0A and GRP1/GRP1A (with both
VDELs set, the A registers are the ones drawn): *"When GRP0 is written to (with any value), GRP1A
receives the contents of GRP1. When GRP1 is written to, GRP0A becomes equal to GRP0"*, and of the
store sequence, *"Note that we always update GRP0A while GRP1A is being displayed, and vice versa"*
〔stella `199704/msg00137`〕. The copy itself is measured here (`litmus_vdel`, `verified-coverage.md`);
that this kernel's stores keep the alternation was not checked against its beam positions —
**Cited only, not verified**.

**Data layout:** six column tables (`Col0..Col5`, one per 8px slice) and a `ColorTab`, all stored
**bottom-row-first** (the kernel walks `row = HEIGHT-1 → 0`) within one ROM page (fixed pointer
high bytes → `lda (p),y` is a deterministic 5 cy). `ColorTab[row]` is the COLUPx value for that
scanline; vary it to taste (here a 16-step rainbow).

## Verified

- Positions `read_tia` hmoved_pixel **87 / 95**, VDEL on.
- **Multicolor proven numerically:** the kernel mirrors the top row's color ($64) and bottom row's
  color ($44) into RAM `$82`/`$83`; the scenario asserts they are distinct (`$82==100`, `$83==68`,
  `$82!=68`). `read_row` confirms different colors on different scanlines (e.g. `3F81FF` cyan at
  line 46, `FFFF2F` yellow at line 52).
- Recognizable, non-blank shape (rainbow heart band on the annotated screen), golden-pinned, 262
  lines every frame.

## Notes / variants

- The color table is independent of the graphic data — swap `ColorTab` for a flashing/cycling
  effect by offsetting the index per frame.
- **Turn the subject so its colour changes run down the screen.** supercat, on a sideways ray-cast
  view: *"another advantage of doing things sideways was that color may be assigned on a per-row
  basis. Thus, one would be able to have different wall panels with different colors"*. He would not
  have the player turn the set over: *"Instead, I'd set the game in a tall but narrow space station
  with nothing interesting on the left and right walls"* (AtariAge `topic/111473`; **Cited only, not
  verified**). Turning the television itself was proposed and rejected — `design-principles.md`.
- **The same per-row colour on a sprite that moves vertically.** In SpiceWare's MaskDraw as Verdant
  catalogued it, a colour pointer is offset like the pattern and mask pointers so that Y indexes all
  three, and `lda (Sprite_Colour_Pointer),y / sta COLUPx` is marked *"Optional for multicolour"*:
  13 cycles without it, 21 with (AtariAge `topic/363349`; the routine is in
  `vertical-positioning.md`). Verdant's first post says he had not yet assembled or run these
  routines; his later post in the thread (comment 5445148) attaches `drawrout.asm`, a DASM file
  that *"can be set to use any of the outlined drawing routines"*, with corrections to other
  routines and none to MaskDraw — **Cited only, not verified**.
- **In one thread a colour change inside the line was a placement problem, not a cycle problem.**
  bigmessowires, colouring a row of playfield shapes with ten `COLUPF` writes a line: *"There's actually enough CPU
  time to make all ten COLUPF changes on each line, but my problem was how they lined up with the
  playfield beans. No matter how I shifted things around, at least one of the COLUPF changes always
  fell inside of a visible bean"*. There it was solvable: glurk answered with a demo that did it
  (`kernel-micro-idioms.md` has how he freed the registers), and the asker replied *"THANK YOU! You
  were right"* (AtariAge `topic/346865`; **Cited only, not verified**). That kernel had cycles to
  spare; this one does not (the budget bullet below). It does not meet the placement question
  because both colours are written before the band starts, in HBLANK.
- **Two colours at any position: inverse graphics.** SpiceWare: *"It's done by using inverse
  graphics — the sprites are BLACK and the color comes from the background and the playfield/ball.
  VBLANK is turned on/off so that the color of the playfield doesn't show on either side of the 48
  pixels. VBLANK can't be changed at the exact cycle it needs to be, so the MISSILES are used to help
  hide the background on both sides"*. He adds *"The VBLANK trick does have a shortcoming"*, which he
  ties to a display whose *"brightness is adjusted incorrectly"* (AtariAge `topic/197100`; **Cited
  only, not verified** — read from distilled notes, not the thread). The fixed-split version — the score bit, left half `COLUP0`, right half
  `COLUP1` — is `design-principles.md`'s; `VBLANK` set and cleared inside a line as a mask is in
  `known-traps.md`.
- **Thomas Jentzsch, 2002: 48 pixels cannot be stretched, only mixed with playfield.** Asked in July 2002 whether the
  48-pixel routine could be stretched across the screen, Thomas Jentzsch: *"The highest resolution
  for patterns are 48 pixel (maybe 49 if you use the ball too). And you cannot stretch these. The
  only thing you can do is to "cheat" and mix some PF graphics (like the title screen of Marble
  Craze) into."* 〔stella `200207/msg00318`〕. B. Watson answered that, accepting flicker, two
  48-pixel patterns side by side on alternate frames double the width; he had done it with 40-pixel
  patterns 〔`200207/msg00319`〕. `text24.md` builds that route, with two 12-character blocks.
  **Cited only, not verified** (both quotes). Measured here, and pressing on Thomas's 48:
  `restrobe-copies.md` decomposes one real kernel, `36char.bin`, at eight 8-pixel slots a scanline
  from nine GRP writes and three `RESPx` strobes, the budget spent at cycle 73 of 76 — eight times
  eight is 64 (our arithmetic). Its sixteen-per-line figure is positions of copies, not sixteen
  pictures (*"Sixteen places is not sixteen full-width places"*), and its counts are for one TIA.
- **Data in the instruction stream.** The Tiara flash cartridge's 32-character menu kernel keeps its
  data in the operands of immediate loads. TomSon, its author (his reply survives here only as a
  quotation in two later posts; the attribution is from the order of the thread): *"All the data is put into register immediate loads (with cycles to
  spare)"*, through macros seeded with the text. SpiceWare: *"the use of register immediate load's
  the same thing we're doing with Fast Fetchers + datastreams in DPC+ and CDF"*, and that capacity
  is what limits colour — for DPC+ *"there's plenty of time to update color - problem is all 16
  datastreams are already in use"*, while *"CDF has 32 datastreams"* (AtariAge `topic/266200`;
  **Cited only, not verified**). `LDA #imm` is 2 cycles against 4 for this kernel's
  `LDA ColorTab,y` (`Gopher2600/hardware/cpu/instructions/definitions.json`). On a plain ROM an
  immediate is fixed when the source is assembled; the menu can change because the cartridge serving
  it is a processor — our reading of the thread, **Not verified**.
- For pixel-exact X placement of the band (1px instead of the 3px coarse grid), combine with the
  ÷3 coarse/fine table + clockslide from topic/209137 (technique ledger ⑯); orthogonal to the color trick.
  As the distilled notes give SvOlli's code there (the thread itself is not kept here), X÷3 and
  X mod 3 are packed into one byte of a 112-byte table (`%mmdddddd`), and the clockslide is code
  that discards an exact number of cycles to time the `RESPx` strobe. The nearest measured
  mechanism is `design-principles.md`'s `CMP #$C9` slide, measured there for delays of 2 to 6
  cycles (`internal/emu/jmptabledelay_test.go`). Whether that is the slide SvOlli used is **Not verified** — no copy of his code
  is kept here to read; the rest is **Cited only, not verified**.
- **Slack a loop already has can buy a playfield gradient.** Glenn Saunders asked for tombstones
  with *"a subtle fade-out gradient from white to dark grey"*, adding that it *"would also shade the
  "alley""* 〔stella `200301/msg00241`〕. Thomas Jentzsch: *"Actually I just tried that yesterday. I
  used the 3 free cycles for "sta COLUPF" and calculated the value for A from Y. There I used the two
  nops at the start of the loop. This gives quite a lot of possibilities. E.g. red tombstones, each
  line with new brightness, ranging from $2..$e"*, followed by `asl` / `ora #$40` / `sta COLUPF`
  〔`200301/msg00246`〕. The cycles were ones the loop was already wasting; this kernel's headroom
  is small (next bullet). **Cited only, not verified.**
- Budget headroom is small (~73 cy used). Adding more per-row work (e.g. a second color register
  for the PF) needs an illegal-opcode tightening (LAX to fuse LDA+TAX).
  **Shipped precedent, added 2026-09-03: this was not a proposal.** Thomas Jentzsch used it in
  Thrust and said why 〔stella `200006/msg00037`〕: *"i'm using `LAX (ptr),Y` to get the data faster
  into the X-register (5 vs 7 clocks). This gives me the time to do the color-cycling"* — the same
  purpose this line proposes it for, in June 2000. His figures check out against our own instruction
  table (`Gopher2600/hardware/cpu/instructions/definitions.json`): `LDA (zp),Y` 5 cy / 2 bytes plus
  `TAX` 2 cy / 1 byte against `LAX (zp),Y` 5 cy / 2 bytes, so **2 cycles and 1 byte**. `LAX` is
  hardware-stable — `known-traps.md` lists it with `SAX/SBX/DCP` ★on original NMOS silicon. `SBX` and `ARR` are reported dead on the **Flashback 2** (AtariAge `113732-clean-assembly`), which is a reimplementation rather than a 6507 — unmeasured here, and stated because this is a technique page telling an author something is safe, unlike `LXA/XAA`.
  "Verify on Gopher2600 first" still stands for our own kernel; what has changed is that the idea
  is no longer untried.
  - **The stack pointer as one more holding register, and what it bought.** In Thrust's score
    kernel Thomas Jentzsch parks one byte with `lax (ptr),y` / `txs`, loads a second with another
    `lax`, and takes the first back with `tsx`: *"I used them get some free time (16 cycles) for
    other effects"*, then spent it on a per-row colour fetch at the head of the loop,
    `lda (digcolPtr),y` / `and digcolMask` / `sta COLUP0` / `sta COLUP1`. The cost he names: *"The
    result is actually only a \*47\* pixel routine, because the right pixel of the first and the third
    digit (if i remember it correct) are always the same"* — acceptable to him because his score and
    the THRUST title are blank there, except for the long T-line 〔stella `200008/msg00024`〕. In
    September 2000, answering a list member's rewrite of the score routine, he gave the reason for
    the registers 〔`200009/msg00044`〕: *"The last two writes to both players must be done in not
    more than 8 cycles each, and they have to be done both"*, which needs values preloaded —
    *"that's why i'm using the stackpointer and the x register for it"*. Getting a value into X
    costs 2 extra cycles and into and out of the stack pointer 6, *"That makes 8 and you only have 6
    cylces left"*; `LAX` saves the two `TAX`es. **Cited only, not verified** — his cycle counts,
    not ours.
  - **A six-pointer kernel with `LAX` for the sixth byte, and per-row colour in its idle time.**
    Manuel Rotschkar posted Seawolf's state display in January 2004: the same six pointers as this
    kernel, with `LAX (spritePointer6),Y` loading the sixth byte into X before the third load and
    `STX GRP1` storing it last 〔stella `200401/msg00138`〕. Asked by Glenn Saunders whether the score
    could have three colours, he answered: replace its `SLEEP 11` with `LDA colortab,Y` /
    `STA COLUP0` / `STA.w COLUP1` *"Right after the WSYNC"* — *"I just did this for Seawolf"*
    〔`200401/msg00147`〕. That is this page's trick (4+3+4 = 11 cycles without a page cross, the length of the `SLEEP` it
    replaces — our arithmetic). Glenn had meant independent colours for the left, middle and right
    digit pairs; Manuel counted *"20 free cycles in this routine"* against 24 for three
    `LDA #color / STA COLUP0 / STA COLUP1`: *"So I fear it's not posible"* 〔`200401/msg00159`,
    `200401/msg00163`〕. **Cited only, not verified.**
