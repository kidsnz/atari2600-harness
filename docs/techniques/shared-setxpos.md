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

## div15 math
- `sbc #15` loop = the known divide-by-15 coarse step (15px granularity).
- The leftover remainder is mapped to the HMOVE nibble via `eor #7` then `asl`×4 (HMxx uses the
  upper nibble; two's-complement, positive = left).
- HMOVE fires **once at the end**, applying every object's pre-latched HMxx simultaneously.
- `sta.w RESP0,x` forces the 16-bit absolute form = 5 fixed cycles, stabilizing the strobe.

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
