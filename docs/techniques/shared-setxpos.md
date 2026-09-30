# Technique — `shared_setxpos`: position all 5 objects with one shared routine

**Source:** AtariAge topic 115690 *New way for repositioning all 5 objects in 1 shared code* (`reference/atariage/115690-*/notes.ja.md`; technique candidate ㉒).

**Goal:** Place all five movable TIA objects (P0, P1, M0, M1, BL) at arbitrary X with a
**single shared code path** instead of five hand-written positioning routines. The standard
initial-placement idiom of real 2600 games (AtariAge #115690, LS_Dracon / robert-m, 2007).

**Demo:** `roms/techniques/shared_setxpos.asm` — five color-coded objects at distinct X
(P0=20, P1=55, M0=84, M1=109, BL=139), all positioned by one `SetXPos` indexed by object
number. CI-locked by `scenarios/shared_setxpos.json` + `.golden`.

## Why one routine can serve all five

The TIA register file is laid out so the relevant registers are **consecutive**:

| obj # | strobe (RESxx) | motion (HMxx) |
|---|---|---|
| 0 P0 | `$10` | `$20` |
| 1 P1 | `$11` | `$21` |
| 2 M0 | `$12` | `$22` |
| 3 M1 | `$13` | `$23` |
| 4 BL | `$14` | `$24` |

So with the object number in `X`, `RESP0,x` selects the right strobe and `HMP0,x` the right
motion register — **one code path positions any object**. (robert-m's correction in the thread:
the X register's true role is *object selection*, not a cycle-filler.)

## The shared loop

```asm
; X coords in RAM, laid out in RESxx order: P0,P1,M0,M1,BL @ $80..$84
PositionAllObjects:
        ldx #4              ; BL down to P0
PosLoop:
        lda xpos,x          ; this object's intended X
        jsr SetXPos         ; the ONE shared routine
        dex
        bpl PosLoop
        sta WSYNC
        sta HMOVE           ; apply all 5 fine adjustments at once
        rts

SetXPos:                    ; A = target X, X = object number (0..4)
        sec
        sta WSYNC           ; fresh line => deterministic coarse timing
WaitObj:
        sbc #15             ; 2  div15 coarse step (15px granularity)
        bcs WaitObj         ; 2/3 loop until borrow = coarse position
        eor #7              ; 2  remainder -> HMOVE nibble correction
        asl                 ; 2 \ shift low nibble to high (HMxx uses upper nibble)
        asl                 ; 2  |
        asl                 ; 2  |
        asl                 ; 2 /
        sta HMP0,x          ; 4  stage fine adjustment for THIS object (indexed)
        sta.w RESP0,x       ; 5  strobe RESxx (coarse). .w forces 5cy fixed timing
        rts
```

**Design rule:** lay the X coordinates in RAM at **consecutive addresses in RESxx order**
(P0,P1,M0,M1,BL). Then a `DEX/BPL` loop + one trailing `WSYNC`+`HMOVE` positions all five.
Each `SetXPos` does its own `WSYNC`, so it costs **one scanline per object** (5 lines for 5
objects) plus the final HMOVE line.

**The one-line cost holds only while the next `WSYNC` is reached in time.** The wait grows by 5
cycles per 15 px, and `SetXPos` returns right after its strobe, so it is the *next* call's `WSYNC`
that must still land on the same line. RMundschau's 2004 routine, which ends with its own `WSYNC`,
states the contract in its header: *"If control comes on or before cycle 73 then 1 scanline is
consumed. If control comes after cycle 73 then 2 scanlines are consumed"* and *"control is returned
on cycle 6 of the next scanline"* 〔`200403/msg00260`〕. Counted with the cycle table
(`Gopher2600/hardware/cpu/instructions/definitions.json`: `sec`/`sbc #`/`eor #`/`asl`/`dex` 2,
`bcs`/`bpl` taken 3, `sta zp,x`/`lda zp,x` 4, `sta abs,x` 5, `jsr`/`rts` 6), the strobe ends at cycle
5q+23 after the `WSYNC` (q = ⌊input/15⌋, the value passed in A) and the next call's `sta WSYNC`
starts at 5q+46 — past 73 from q = 6, input ≥ 90, where that object costs two lines. Measured on the
demo with `trace_clocks` (2026-09-30; the `.asm` passes inputs 20/55/85/110/140, which land at the
`hmoved_pixel` 20/55/84/109/139 above): the next `sta WSYNC` starts at cycle 15 of the following line
after BL (input 140) and at cycle 5 of the following line after M1 (input 110), but at cycle 71 after
M0 (input 85) and 61 after P1 (input 55). The frame is 3 VSYNC + the line the loop is called from + 7
+ 29 VBLANK (the first of them the HMOVE line) + 192 + 30 = 262, matching the scenario — the loop
takes 7 lines for 5 objects, not 5. `cmd/framegen` measured the same effect on its copy of
`SetXPos` — *"Combat (P1 at clock 145, input 139) spends one line more than Outlaw does"* — and
calibrates the frame length for it. Shrek met the inline form in 2004: with code added to the
positioning lines, *"if a sprite was too far on the right, it didn't fit and dropped on the next
line"*; he moved the code, and it came back when a monster walked past its rightmost point,
*"prolonging the positioning line and throwing the following few instructions to the next line, thus
shifting the platform one pixel down"* 〔`200410/msg00142`, `200410/msg00144`〕. **Cited only, not
verified.**

**Why `ldx #4`, not `#5`.** The five objects are indices 0..4. A sixth pass writes the next
register in each run instead: `RESP0+5` = `$15` = `AUDC0` and `HMP0+5` = `$25` = `VDELP0`
(`internal/beamtrace/beamtrace.go`) — an audio register and P0's vertical delay, not a position. It
has happened on AtariAge at least twice: `ldx #5` in Nosehair (karl-g: *"when the X register is 5,
you are stomping on AUDC0"* 〔AtariAge `topic/312285`〕), and a `cpx #$06` up-count whose sixth pass, per
Omegamatrix, *"ultimately writes to VDELP0 and AUDC0!!"* 〔AtariAge `topic/255357`〕. The second case adds the
other half of the design rule: that loop read `PlayerX0,x`, so the RAM *declaration order* decides
which variable positions which object, and the sixth byte it read was `PlayerY0S`, a variable that
had simply been declared next. **Cited only, not verified.**

## div15 math
- `sbc #15` loop = the known divide-by-15 coarse step (15px granularity).
  Why 15: one taken pass of `sbc #15`/`bcs` is 2 + 3 = 5 CPU cycles = 15 colour clocks, the time the
  beam takes to cross 15 pixels, so the same loop divides X and waits out the distance. RMundschau:
  *"our timing loop takes 5 cycles * 3 color clocks = 15 pixels. SBC #15 takes 2 cycles the same as
  DEY or DEX"* 〔`200403/msg00260`〕; SpiceWare's `PosObject` says it in a comment: *"each time thru
  this loop takes 5 cycles, which is the same amount of time it takes to draw 15 pixels"* 〔AtariAge
  `topic/342613`〕.
- The leftover remainder is mapped to the HMOVE nibble via `eor #7` then `asl`×4 (HMxx uses the
  upper nibble; two's-complement, positive = left).
- The nibbles this gives run from +6 (left 6) to −8 (right 8), not ±7: after the loop A is −15..−1,
  its low nibble 1..15, and `eor #7` maps that to 6..0 then −1..−8. RMundschau's table, and the
  table-free form he gives beside it (`eor #%00000111` / `adc #1` / `asl`×4; the carry is clear after
  the loop, so `adc #1` adds exactly 1), give +7..−7 〔`200403/msg00260`〕. Both are 15 consecutive
  steps, so either reaches every pixel once calibrated (`road.md`: *"Calibrate, don't copy"*); they
  differ by a one-pixel offset: both of RMundschau's forms use left 7 (his table's first entry is
  *"Left 7"*), and only `eor #7` alone uses right 8. **Not verified** in the emulator — arithmetic over
  the 15 remainders.
- HMOVE fires **once at the end**, applying every object's pre-latched HMxx simultaneously.
- `sta.w RESP0,x` forces the 16-bit absolute form = 5 fixed cycles, stabilizing the strobe.
- That depends on the assembler keeping the index. In 2017 two DASM builds both numbered 2.20.11
  disagreed: kylearan's (20140304) assembled `sta.w RESP0,x` to `STA.wx RESP0,x`, Thomas Jentzsch's
  (*"DASM 2.20.11 unofficial RevEng 20140124"*) to `STA.w RESP0` — the `,x` dropped without a word,
  so the strobe always hit `RESP0` and *"the ball and P1 are moving horizontally between frames"*.
  kylearan's suggestion for that build was to write `sta.wx RESP0,x` 〔AtariAge `topic/264527`〕. The
  opcodes differ (`$9D` `sta abs,x`, 5 cycles, against `$8D` `sta abs`, 4). Nothing here reads the
  opcode, and CI installs whatever `dasm` apt provides (`.github/workflows/ci.yml`); but a dropped
  `,x` would send every strobe to `RESP0`, so the demo's four `hmoved_pixel` asserts for P1, M0, M1
  and BL should catch it indirectly (inference, not tried; the local `.bin` has `$9D`).
  **Cited only, not verified** for the DASM builds.

## Variants

- **Moving by a difference instead of placing at X** 〔AtariAge `topic/212690`, svolli〕: `sec` /
  `lda oldpos` / `sbc newpos` / `asl`×4 / `sta HMxx`. old − new is positive for a move to the left,
  the sign HMxx uses, so the shifted difference is the nibble — no table, no re-strobe. It does not
  handle the wrap at the screen edge (old 2 → new 158 wants left 4), and one HMOVE reaches only +7
  left .. −8 right (`known-traps.md`, *HMOVE range*), so a longer move is split across frames.
  **Cited only, not verified.**
- **Two players 8 px apart from one X, on one line** 〔AtariAge `topic/119524`, cybergoth's
  Seawolf code as quoted there〕: after the `sbc #$0F`/`bcs` wait, `and #$0F`/`tax`/`lda hmovetab,x` is P1's nibble
  (a 16-entry table), and `sbc #$0F` on it — the carry is clear after the loop, so this subtracts
  `$10`, one step further right — is P0's; then `sta RESP0`/`sta RESP1` back to back. The strobes land
  9 colour clocks apart and the one-step nibble difference closes that to 8 — the correction
  `sprite-placement.md` (*What rule 3 buys*) takes from two shifted tables, here derived from one. The
  code needs a `clc` ahead of the wait loop; without it the pair shifts one pixel right when it moves.
  **Cited only, not verified** — taken from the mining notes; the thread's own text is not in the
  corpus.

## CI

`go run ./cmd/scenario roms/techniques/scenarios/shared_setxpos.json` (exit 0):
five `tia.<obj>.hmoved_pixel` asserts (one per object), `ntsc_frame_lines:262`,
`golden_frame:true`.

## Verified facts
- One shared `SetXPos` (indexed by object number via the consecutive `RESP0,x`/`HMP0,x` layout)
  positioned all five objects at **distinct intended X**: P0=20, P1=55, M0=84, M1=109, BL=139
  (measured `hmoved_pixel`).
- Players land at X (= 3N−54), missiles/ball at X−... (3N−55); the 1px player offset shows as
  the +1 difference between the player targets and the missile/ball targets that share the same
  coarse math — exactly the documented hardware offset.
- All five are visibly rendered at distinct X with distinct colors (`read_row` at scanline 100:
  P0 yellow @20, P1 red @55, M0 @84, M1 @109, BL cyan @139).
- 262 lines/frame; golden frame hash stable.
