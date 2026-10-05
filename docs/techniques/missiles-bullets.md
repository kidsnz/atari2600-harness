# Technique — missiles as bullets (RESMP spawn, flight, hit)

**Goal:** the standard "fire / fly / hit" loop using a missile: spawn at the player via RESMP,
move vertically by row-range drawing, detect hits with the collision latches.

Demo: `roms/techniques/bullets.asm` (joystick ship, fire launches M0 at a P1 target; hit
respawns the target +24px).
CI: `scenarios/bullets.json` (spawn position, flight, latch, hit bookkeeping, 262, golden).
New hardware verification: `litmus_resmp` + `scenarios/resmp.json`.

## Verified hardware facts (litmus_resmp)
- **RESMP unlock places the missile at player+4px** (1x size player center): player at 24 →
  missile at 28; follows the player after HMOVE moves (32 → 36).
- **The lock needs at least one frame of held RESMP** before unlocking. The lock-to-center sync
  happens while the player counter scans during the frame — a lock+unlock in the same logic pass
  does NOT move the missile (measured; Gopher2600 syncs at the player's scan `Pixel==2`).

## What the RESMP lock also does (cited, not measured here)
`litmus_resmp` measures where the missile lands. Two more effects of the same bit are only cited:
- **A locked missile is not drawn.** For a missile that showed on only one scanline, Dennis Debro
  suggested, conditionally ("If I understand you correctly"), that `RESMP0=#$02` was being set: *"the
  missile will be locked to the center of it's corresponding player and also be disabled. If you use #$00
  then the missile can move independently of it's player and it will also be enabled"* 〔stella-list
  `200302/msg00138`, 2003〕. The asker confirmed it: *"The problem was completely related to my resetting
  RESMP0 to $02"* 〔stella-list `200302/msg00141`〕. Andrew Towers: *"No copies of
  the missile will ever be drawn while RESMP0 is set"* 〔stella-list `200309/msg00252`, 2003〕. The vendored
  engine draws no missile pixel while `ResetToPlayer` is set (`Gopher2600 hardware/tia/video/missile.go:443`,
  engine read). **Cited only, not verified**.
- **Only the main copy resets it.** With `NUSIZx` copies, *"the reset only takes place on the main copy of
  the player (close, medium and far copies have no effect)"* (Towers, same post). He adds, untested and
  "in theory", that `RESMP0` could then hide the missile's copies while the player's copies draw and be
  cleared before the main copy without moving the missile. The engine's `triggerMissileReset` returns
  false for every copy but the first (`player.go:776`, engine read). **Cited only, not verified**. The same
  post guesses the 2x snap at player pixel "8 or maybe 9"; `fundamentals-audit.md` measured +6 at 2x.

## A missile has no colour of its own

`M0` draws in `COLUP0` and `M1` in `COLUP1` — there is no `COLUM` register. **A bullet cannot be a
different colour from the player that fires it**, and changing `COLUPx` mid-line to recolour the
missile recolours the player on the same line.

This is stated elsewhere in this repository, but only where it is an *advantage*:
`invisible-probe.md` lists it under **Free** (a probe that cannot be told apart from its player is
exactly what that technique wants). Someone opening *this* page to build a coloured laser has no
reason to read that one, so the same fact is written here, where it is a cost.

The three-way choice, from Manuel Polik on Star Fire (stella-list `200209/msg00153`):

| object | own colour? | flickers? | horizontal grain |
|---|---|---|---|
| a second **player** | yes (`COLUP1`) | yes, if the line already carries two | 1 clock |
| a **missile** | **no** — borrows `COLUPx` | no | 1 clock |
| the **playfield** | yes (`COLUPF`) | no | **4 clocks** |

So a coloured bullet costs either a flickering object or a 4-clock-wide one. Pick before drawing:
this is the kind of constraint that a mockup cannot catch by itself (`design-principles.md`).

**Borrow the other player's missile.** When the two players' colours are each other's — Sohl's
example is Spy vs Spy, *"if your sprites have inverted colors relative each other"* — put `M1` (drawn
in `COLUP1`) on player 0 and `M0` on player 1: *"You'd use Missile 1 with Player 0, and Missile 0 with
Player 1"* 〔AtariAge `topic/337214`〕. Each player gets a second colour from a missile it does not own.
The cost: neither missile is free to be its owner's bullet, and the accent changes whenever the other
player's colour does. **Cited only, not verified**; it rests on the register fact at the top of this
section.

**Width from two objects is flat.** A paddle game short of objects can make its bat from the ball and
a missile side by side, both set to 8 clocks, 16 wide 〔AtariAge `topic/317569`, 2021〕. Besides the
two colours above (the ball draws in `COLUPF`), the bat is flat: neither object has a bitmap, so its
face is a rectangle unless the kernel reshapes it line by line (`hmove-slope.md` moves a missile or the
ball per line). **Cited only, not verified**.

**Nor a size register of its own.** A missile's width is in its player's `NUSIZx` (bits 4-5,
`zone-multiplexing.md`), so repositioning missiles as extra bullets rewrites a register the player
also draws with. SpiceWare's Draconian (2014) lets each reposition care only about the size of the
object it moves and then runs one `FixSizes` clean-up after all repositioning to set every size
right. Read from distilled notes, not the blog post (AtariAge blog entry `10896`); **Cited only, not
verified**.

**A reshaped missile costs three writes per line; a player costs one.** Glenn Saunders, fitting missiles
into his Death Derby kernel: a player needs only `GRPx`, data or zeroes, but for missiles *"I have to
selectively enable them, set fine control, and set width. That's six writes right there if I intend to
update both of them on the same scanline"* — `ENAMx`, `HMMx`, `NUSIZx` each 〔stella-list
`200111/msg00115`, 2001〕. That count is for a missile whose width and offset change every line; the
bullet in *The pattern* below writes only `ENAM0` per line. Keeping width and motion in one byte then
costs an `AND %00110000` before the `NUSIZx` write, *"otherwise I mess up the players in the process"*.
**Cited only, not verified**.

**Mirroring a missile-drawn shape.** Missiles and the ball have no `REFPx`. A shape drawn by moving a
missile every line can be stored as the offset *from the previous line* in the upper nibble, so each byte
goes straight into `HMMx`; to face the other way without a second table, Erik Mooney: *"I think doing
XOR #$F0 / CLC / ADC #$10 will work, and you may be able to skip the CLC if some code immediately previous
(usually a compare) leaves the carry in a known state"* 〔stella-list `200102/msg00100`, 2001〕 (`XOR` is
`EOR`). Our reading: negating the offset mirrors the left edge, which mirrors the shape only while the
width stays the same from line to line; where the `NUSIZx` width changes, the mirrored left edge must
also move by that change. **Cited only, not verified**.

**When the kernel is full, change only what is set off screen.** Erik Mooney on his Invaders kernel,
where nothing more fits on the invader lines: an explosion could be made by setting the missile *"to 8
clocks wide off the screen, then display it on alternating frames with cycling colors. This doesn't
involve changing the kernel during invader display - all it involves is setting the missile X and Y
coordinates, and changing the color register, all of which can be done offscreen"* 〔stella-list
`199801/msg00065`, 1998〕. A suggestion, not something he shipped there; the colour register is the
player's `COLUPx` (above). **Cited only, not verified**.

## The pattern

- **Spawn**: fire edge (and no live bullet) → `RESMP0=2`, mark state "locking" (`bulY=$FF`).
  *Next* frame's logic: `RESMP0=0`, `bulY=SHIPROW-4`. Order matters: process the locking→flying
  transition **before** the fire-edge check in the frame logic, or the same pass unlocks
  immediately (the bug we hit).
- **Flight**: `bulY -= 4` per frame. The kernel draws `ENAM0` on rows `[bulY, bulY+4)` with a
  branch-free compare (`txa / sbc bulY / cmp #4`). **Inactive = sentinel 200** (out of row
  range) — no "is active?" test in the kernel, which keeps the worst row ≈62 cycles.
- **Hit**: `BIT CXM0P` / `BMI` → D7 = M0×P1. **Read address $30** — collision reads decode the
  low nibble, so a sloppy `$32` silently reads CXP0FB instead (the second bug we hit; the latch
  was provably set via `read_collisions` while the ROM saw nothing). Clear with `CXCLR` every
  frame after the check.

## Hard-won notes
- **Kernel line budget**: the first version did per-row `lda bulY / beq …` gating and blew past
  76 cycles when the bullet was live → the TV frame stretched to 350 lines. Sentinel encoding +
  X-as-row-counter brought every row under budget. Symptom to remember: *line count changes only
  while an object is active* = per-row code over budget on its active path.
- `PosObject` (divide-by-15) fine adjust is **`eor #7`**, not `eor #$FF` (that reverses the
  fine-adjust direction and breaks linearity). With indexed stores (`sta RESP0,x`) the measured
  calibration here is real X = A−3; with absolute stores it was A−9. **Calibrate per kernel
  with `read_tia`, never copy constants.**
- **Write `ENAMx` before the visible line starts.** The picture begins about 22.7 CPU cycles after
  `WSYNC` (68 colour clocks of HBLANK ÷ 3). nukey-shay diagnosed a missile drawn skewed at the left edge
  as a `sta ENAM1` landing at cycles 22–25, already on screen. His fix: decide on the previous line's
  spare cycles, keep the result in a temp byte, and on the line itself only `lda temp / sta ENAM1`,
  done by cycle 18 〔AtariAge `topic/262272`, 2017〕. The same thread offers vertical delay for when even
  that does not fit; it does not reach missiles, which have no VDEL (`fundamentals-audit.md`, measured).
  **Cited only, not verified**.

## Many bullets from one missile: HMOVE it, no full reposition (River Rampage)
Erik Mooney's River Rampage (2004) draws the player's bullets with `P1` + `M1` and `NUSIZ1` copies.
P1 and M1 *"are never fully repositioned during the screen - they're only HMOVEd, so we can reposition
from one bullet to the next using only a few instructions instead of an entire scanline for each
object."* For gaps between bursts, *"the game keeps calculating and HMOVEing invisible bullets on all the
inbetween lines, even when you're not shooting, using GRP1 and ENAM1 to turn them off when appropriate. It
initializes a new bullet every frame whether or not the joystick button is pressed"* 〔stella-list
`200404/msg00269`, 2004〕. His bullets are 8 lines tall, positioned every 8 lines and advance 8 lines a frame;
enemies and their shots use `P0`/`M0`, which win priority over `P1`/`M1`, so a bullet never covers what
the player must dodge 〔stella-list `200404/msg00274`, 2004〕. **Cited only, not verified**.

## The ENAM "stack trick" — branchless 1-line missile enable (Combat)
For a **1-scanline** missile, instead of the row-range compare you can enable ENAM branchlessly in
~10 cy using the stack as a TIA-write pointer. Page 1 (`$01xx`) mirrors the TIA (A7=0), so
`[$011D]=ENAM0`, `[$011E]=ENAM1`, `[$011F]=ENABL`. Point SP at the ENAM mirror and `PHP` writes the
processor status — whose **bit 1 is the Z flag** — straight into ENAM.D1 (the enable bit):
```
; once per frame, before the visible region:  LDX #$1D ; TXS   (SP → ENAM0 mirror)
; per kernel line:   CPX MissY0   ; Z = (this row == missile row)
                     PHP          ; [$011D]=ENAM0, D1=Z  → lit only on its row
                     PLA          ; restore SP=$1D
```
Needs **no `JSR` in the kernel/VBLANK/overscan** (SP is borrowed). **Corrected 2026-09-03:** this line read "Needs `SEI` (no IRQ)" and `SEI` does nothing here — **the 2600 has no path by which an IRQ can reach the 6507**. Measured in the vendored engine: **`CPU.Interrupt()` is never called** — searching every `.go` under `Gopher2600/` for `.Interrupt(`, excluding its own definition, `InInterrupt`, and the ARM coprocessor's `mem.arm.Interrupt()`, returns **zero** call sites. The five `mem.arm.Interrupt()` calls belong to the ELF/ACE cartridges' ARM and are a different method on a different processor. **Restated 2026-09-04:** this line previously read "called from five places and all five are `mem.arm.Interrupt()`", which put the right number under the wrong subject — the five were never calls to `CPU.Interrupt()` at all, and the true count is zero, which is the stronger statement. Found while re-running the original query after learning that `rg -r` takes an argument: `rg -rn` is `--replace n`, so the earlier search had been silently rewriting each match to the letter `n` and printing no line numbers, at exit 0. the RIOT's PA7 flag is a status bit software polls (TIMINT) and never reaches the CPU.
**Corroborated externally, added 2026-09-03:** the list said it first and drew the same distinction.
Erik Mooney, 1999: *"There are no interrupts on the 2600."* Two years later someone asks *"is the
`SEI` at the beginning of most games unnecessary then?"*, and Eckhard Stolberg answers by separating
exactly what we separated from the engine — the RIOT's own flag (*"everytime the timer wraps from $00
to $FF the interrupt flag is set (if timer interrupts are enabled)"*) from anything the 6507 can see.
The derivation here was independent and reached the same split; this is the rarer sort of source, the
kind that **confirms** a decision rather than correcting one. Most ROMs here still open with `sei` (139 of 173 .asm files) and that is fine as convention — the error was calling it a requirement of this technique. Trap: **ENAM0=$1D,
ENAM1=$1E, ENABL=$1F** — mixing them lights the wrong object; verify with `read_motion` height, not the
annotated position marker (a proxy).

### Taller than one line: subtract, then mask
`CPX` sets Z on one line only. Subtract instead and mask the difference, and Z covers a run of lines
whose length is a power of two:
```
; A = MissileY - line
        AND #$FC     ; Z = 1 when A is 0..3: a 4-line missile
        PHP          ; Z lands in ENAMx D1
```
Jim Nitchals, 1997: this *"is what Combat does, but it's limited to powers of 2 for the missile height
(its missiles are 2 high so use AND #$FE.)"* 〔stella-list `199704/msg00057`〕. Erik Mooney, replying:
*"Combat's missiles are 2 high? It looked like 1 to me"* 〔`199704/msg00060`〕, so the height Combat uses is
disputed in the thread. In 2001 Erik passed it on for Gunfight — *"This only works if your missile height
is a power of two, though"* — and Manuel Polik answered *"So it _will_ be a power of two"* 〔stella-list
`200102/msg00213`, `200102/msg00232`〕. Erik's 1998 ball version is `TYA / SBC BallY / AND #$F8 / PHP /
PLA`, 14 cycles for an 8-line ball, `PLA` restoring SP. When Eckhard Stolberg objected that `AND` does not
affect the carry, Erik answered that the zero flag carries the answer, and conceded *"I should be doing a
CLC before the SBC, but there just aren't the cycles to do it"*, so the ball may sit a line off
〔stella-list `199804/msg00023`, `199804/msg00025`, `199804/msg00026`〕. (He remembered `ENABL` as `$19`; it
is `$1F`.) The subtraction leaves the position free; only the height is limited. **Cited only, not
verified**.

Dennis Debro's other route to a taller object keeps the single-line compare and runs it only every Nth
line; Bob Montgomery asked whether that also limits vertical position to N-line steps, and Dennis: *"Ha,
I didn't think about *moving* the ball"* 〔stella-list `200502/msg00088`, `200502/msg00089`,
`200502/msg00090`〕. **Cited only, not verified**.

**`EOR` instead of `SBC` quantises the position too.** Gunfight's kernel compared with `TYA / CLC / SBC
verPosMx / AND #$FF / PHP` (a 1-line object). Thomas Jentzsch suggested `eor verPosMx` in place of `CLC /
SBC`, *"This would save you 4 more cycles"*, and Manuel Polik: *"Hey, that worked, too. Even with other
powers of 2!"* 〔stella-list `200102/msg00257`, `200102/msg00259`, `200102/msg00261`〕. Our reading, not
from the thread: `(line EOR y) AND mask` is zero only when `line` and `y` agree in the masked bits, so a
2^n-line missile can only occupy an aligned block of 2^n lines. Height and vertical position are both
quantised, where the subtraction quantises only the height. **Cited only, not verified**.

**Any height: `DCP` puts the answer in the carry.** The illegal `DCP` (`DEC`+`CMP`; HW-stable on NMOS,
`known-traps.md`) decrements a per-object counter and compares it with A, and an add moves the carry into
D1. No stack and no line counter are involved. Thomas Jentzsch's version, corrected in the thread to
`#HEIGHT-1` 〔stella-list `200401/msg00149`, `200401/msg00151`, 2004〕:
```
        lda #4-1
        dcp torpedoyPos     ; counter decremented every line
        adc #2
        sta ENAM1           ; 12 cycles per missile
```
Manuel Rotschkar (posting from the `cybergoth` address that Manuel Polik's 2002 Star Fire posts use)
replaced his `LAX`/`SBC`/`AND #$FC`/`PHP` pair (30 cycles for two missiles) with it
and reported almost 100 bytes saved, *"one page less to cross"*, four scanlines won and X free throughout
the kernel 〔`200401/msg00144`, `200401/msg00153`〕. Seawolf draws its torpedoes with the same `LDA #3 /
DCP torpedoyPos / ADC #2 / STA ENABL` (Manuel, quoted in 〔stella-list `200407/msg00003`〕). Bob Montgomery's
ball version is `lda BallHeight / dcp BallYPosition / sbc BallAdjuster / sta ENABL`, 14 cycles, with
`BallAdjuster = BallHeight - 2`; his first post had `sbc BallHeight` and he withdrew it 〔stella-list
`200502/msg00082`, `200502/msg00085`〕. Our reading of the arithmetic: the constant after `DCP` decides
which carry sets D1. Bob's leaves A at 2 or 1 for any height. Thomas's `adc #2` gives 6 or 5 at height 4
(D1 set or clear), but at height 2 the same `adc #2` gives 4 or 3, which inverts it. `DCP` also rewrites
its counter, so the counter is reloaded every frame (`vertical-positioning.md` uses the same instruction to
skip-draw a player). Manuel's rule of thumb: the shortcuts differ by height, 6 is *"almost worst case.
Better would be any value from 1-4 or any power of 2"* 〔stella-list `200407/msg00009`〕. **Cited only, not
verified**.

### Two missiles in one branchless burst — the descending double-push
Both players' missiles (M0 + M1) in one sequence: point SP at the **higher** mirror (`$1E`) and push
twice, descending, so each `PHP` lands in the next ENAM:
```
; once per frame:  LDX #$1E ; TXS
; per line:   CPX MissY1 ; PHP   ; [$011E]=ENAM1  (SP $1E→$1D)
             CPX MissY0 ; PHP   ; [$011D]=ENAM0  (SP $1D→$1C)
             PLA ; PLA          ; restore SP=$1E
```
= 20 cy, both missiles, no branches. The two `PLA` (8 cy) are the SP restore, and they are mandatory
**only while `X` is the line counter** — which is what makes `CPX` the comparison and leaves nothing to
hold `$1E` across the line.

**Corrected 2026-09-03: this line read "Irreducible", and the sentence two lines below already named
the escape.** Compare the line counter in `Y` instead and `X` stays free, so the restore is a 2-cycle
`TXS` rather than eight cycles of `PLA;PLA`:

```
; once per frame:  LDX #ENABL ; TXS
; per line:   CPY BLline ; PHP
             CPY M1line ; PHP
             TXS                 ; 2 cy, not 8
```

That shape is **Thomas Jentzsch, stella 1999-11 〔msg00039〕**, whose own comment reads
`php ;3      got this trick from Combat` — so the in-house derivation below and this post reach the
same trick from the same game, twenty-seven years apart. **`CPY` appears nowhere in this file or in
`two-line-kernel.md`**, which is why the alternative read as "a different technique" rather than as
the same one with a different register. The saving is arithmetic on paper here and has not been
measured; what is measured is that "irreducible" was too strong a word.

**Three objects: start one higher.** Manuel Polik's Gunfight kernel points SP at `$1F` and pushes three
times — `CPY verPosBL : PHP`, `CPY verPosM1 : PHP`, `CPY verPosM0 : PHP` — lighting ENABL, ENAM1 and
ENAM0 from one branchless sequence 〔stella-list `200102/msg00367`, 2001〕. He re-points SP every line
with `LDX #$1F / TXS` before the `WSYNC`, because his X indexes the player graphics. By the opcode
table that is 18 cycles for the three pairs plus 4 for the reload, the 22 cycles nukey-shay quotes for
the same shape 〔AtariAge `topic/262272`〕. **Cited only, not verified**.
Manuel Polik's Star Fire (2002) uses the same three pushes for something other than shots — a
starfield drawn with the ball and both missiles, `LDY vline / CPY yposBackup+2 / PHP / CPY yposBackup+1 / PHP / CPY yposBackup / PHP` — and sets
SP without losing X by parking X in A: `TXA / LDX #$1F / TXS / TAX` (A is the register lost instead)
〔stella-list `200211/msg00190`〕. His comment calls `$1F` "RESBL"; it is the `ENABL` mirror. **Cited only,
not verified**.

If the line still overruns, the fix is more budget (a 2-line kernel), or the graphics-pointer X-pin
trick that collapses `PLA;PLA` to a 2-cy `TXS` (`two-line-kernel.md`).
— in-house: Combat 2026-07-18, ∀-certified; independently in Jentzsch 1999-11 〔stella msg00039〕.

### What else the stack can write
- **Page 1 is split by A7.** The trick uses `$0100-$017F`, where A7=0 selects the TIA. `$0180-$01FF`
  has A7=1 and selects the RAM, which is why an ordinary stack works at all — Thomas Mathys: the 128
  bytes are *"mapped in twice ... once at 0x0180-0x01ff, so that the stack can be used, because it is
  assumed to be in page 1"* 〔stella-list `200405/msg00053`, 2004〕. `$0180` is measured as a mirror of
  `$0080` (one address, `litmus_mirror`, `verified-coverage.md`). Our reading: both halves follow from one
  address line, so a stack that grows below `$0180` stops writing RAM and starts writing TIA registers.
  **Not verified**.
- **`PHP` writes all eight flags, not only Z.** `ENAMx` and `ENABL` read only D1, so the other bits do
  nothing there; pointed at a register that reads other bits, they land as well (our reading). Bit 5 is
  the unused flag; C. Bond, 2005: the documents say it is "usually" set to 1, *"but no other explanation
  is given"*, and nobody answered 〔stella-list `200508/msg00022`〕. The vendored engine always pushes it as
  1 (`Gopher2600 hardware/cpu/registers/status.go:119`, engine read). **Not verified**.
- **`JSR` strobes two registers in one instruction; `BRK` three.** Thomas Jentzsch, on AtariAge as quoted
  on the list: *"To feed two continous registers as fast a possible (only 6 cycles) you can abuse JSR
  (stack points at those registers). And 3 registers with BRK (7 cycles)."* The bytes are the return
  address (and, for `BRK`, the flags), not chosen data, so Christopher Tumber found little gain for registers
  whose value matters (one cycle at best, in his four-colour-register example). Jentzsch called it *"only brainstorming a little bit (too much)!"*, and to
  Tumber's guess at a one-shot "burst mode": *"Yes, that may work. Actually I was mainly thinking about
  registers like RESPx, where the values don't matter at all. So you could position two players very
  close together without using HMOVE."* 〔stella-list `200307/msg00030`,
  `200307/msg00037`, 2003〕. Two years later, to Manuel's note that his Worm Whomper demo used it *"(with
  JSR!)"*, Jentzsch replied *"So finally someone finds something where my idea becomes useful"*, and
  Manuel answered *"Yup! Two RESPs don't get two particles close enough together"* 〔stella-list
  `200508/msg00020`, `200508/msg00021`, 2005〕. One commercial use is reported: an emulator author found *"the trick where
  Pole Position whacks three registers in three clock cycles using the BRK instruction. It revealed a bug
  in the timing of my BRK emulation"* 〔AtariAge `topic/260569`, 2017〕. Pole Position is not in this
  repository and the attribution is unchecked. **Cited only, not verified**.
- **`PF2` sits next to `RESP0`.** Erik Mooney: *"PF2 and RESP0 are actually right next to each other in
  the TIA memory map at $0F and $10. So if you can come up with some way for the stack/JSR or BRK trick to
  actually write data into PF2, you could use that to write to both registers"* 〔stella-list
  `200405/msg00042`, 2004〕; nobody in the thread took it up. Our reading: `RESP0` ignores its value, so
  a `JSR` with SP at `$10` would strobe `RESP0` and then write the return address's low byte into `PF2`.
  **Not verified**.

## Or read the enable from a table instead
Not a stack trick: the compare can also be traded for memory.
- **A run of zeroes with one `$02`.** Billy Eno's Warring Worms (2002) keeps `ds 66 / dc $02 / ds 69` and
  per missile does `ldy missNindex / lda missile_enables,y / sta ENAMn`, incrementing each index on every
  line pair; a missile off screen gets an index into the zeroes. *"I found this all had to be on the same
  page in memory or when it rolled over to the next page, it would add a clock cycle to the access and
  mess up the display of my playfield"* 〔stella-list `200201/msg00062`〕. That is the read-side page-cross
  penalty `TestPageCrossPenaltyRules` (`internal/cpudiff/pagecross_test.go`) measures for `LDA abs,X` and
  `LDA (ind),Y`; his `abs,Y` read is not one of its cases.
- **One table per height.** `LDA (ballPointer),Y / STA ENABL` over a table with as many zeroes as the
  kernel is high around the enables (Manuel Rotschkar); Bob Montgomery's costs: a table for each height,
  and with more than about 128 lines the reads cross a page, so the timing is constant only while the
  ball keeps clear of the top and bottom 〔stella-list `200502/msg00091`, `200502/msg00093`,
  `200502/msg00096`〕. For a short bottom kernel (his example: *"lets say"* ten 2-line-kernel lines), Manuel (`cybergoth`) proposed *"a set of
  ENABL tables for all possible different vertical ball positions"*, so that kernel never decides whether
  to draw 〔stella-list `200307/msg00114`〕.
- **One byte, two registers.** Thomas Jentzsch suggested merging a ball's `HMBL` and `ENABL` tables, *"because
  HMBL and ENABLE can share the bits"*: `lda BallTab,y ; hhhhsse0`, then `sta HMBL` and `sta ENABL`. `HMBL`
  reads D7-D4 and `ENABL` reads D1. For 2 more cycles, `asl / asl / ora #5 ; hhsse101` feeds `CTRLPF`
  too, the size bits landing in D5-D4 〔stella-list `200303/msg00155`〕. The `#5` fixes `CTRLPF` D0 and D2
  as constants; in Fabrizio Zavagli's kernel, which he was answering, the whole `CTRLPF` byte came from a
  `BallSizeTab` table 〔stella-list `200303/msg00152`〕, and the thread does not say which D0/D2 values that
  game needs.

**Cited only, not verified**.

## Missile bounce / tank block off the playfield — CX?FB move-check-revert
The mover doesn't know where the maze walls are; use the hardware object-vs-playfield latches
(`CXP0FB`/`CXP1FB`/`CXM0FB`/`CXM1FB`, D7 = object∧PF, cleared each frame by `CXCLR`). A collision is
only known **after** the object rendered, so it's a 1-frame-delayed bump:
- **Tank block:** before moving, `BIT CXPxFB; BMI blocked`. On a hit, restore the last non-colliding
  position (saved when clear) **and** skip this frame's forward step — revert *alone* re-enters the wall
  next frame; skip *alone* leaves the tank stuck inside. Turning stays allowed so you can steer off.
- **Missile reflect (2-frame axis probe):** on `CXM?FB`, revert to the last clear position and reverse
  **X** (assume a vertical wall); if it still collides next frame it was a horizontal wall → reverse **Y**
  instead. 16-dir ring: reverse-X = `(8−dir)&15`, reverse-Y = `(−dir)&15`, reverse-both = `(dir+8)&15`.
- **Debug with `read_collisions`:** it separates `m0_pf` from `m0_p0`/`m0_p1`. A missile spawned *inside*
  its own tank reads `m0_p0=true, m0_pf=false` — don't mistake that for a wall hit (the bug that stalled
  the reflect prototype). — in-house: Combat 2026-07-18/19 (block shipped ∀-certified; reflect: vertical
  bounce verified via `read_motion`, full probe still has a spawn-inside-tank edge case).
