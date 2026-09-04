# Sub-pixel velocity (DDA error accumulator)

**Problem.** An object's position is a 1-byte integer, so the only speeds you can express by
`pos += vel` are whole pixels per frame: 1, 2, 3… The jump from 1 to 2 px/frame is **+100%** — a
speed doubling the player feels as a lurch. Every classic that ramps ball speed (arcade Pong 1972,
Breakout 1978, faithful ports like djmips APong) avoids this by running **sub-pixel** speeds
(0.5 / 0.75 / 1.25 / 1.5 …). The naïve fix — widen position to 16-bit 8.8 fixed-point — forces you
to rewrite every consumer of the position (here: the ÷15 coarse-positioning loop and its HMOVE
fine-adjust, which read the integer X directly). Too invasive.

**Technique.** Keep the position an integer. Carry the *fraction of the velocity* in a separate
1-byte accumulator and let it spill an extra whole pixel every few frames (a 1-D DDA / Bresenham
error term):

```
; state: Vel_int (whole px/frame, signed), Vel_frac (0..255 = the /256 fraction), Err (accumulator)
        lda Err
        clc
        adc Vel_frac        ; accumulate the fractional speed
        sta Err             ; C = 1  ⇔ we owe one extra pixel this frame
        lda Pos             ; integer position, unchanged semantics
        ; add Vel_int (+ the carry, applied IN THE DIRECTION OF TRAVEL)
        ...                 ; see "sign" note below
        sta Pos
```

Average speed = `Vel_int + Vel_frac/256` px/frame, and **`Pos` always lands on an integer** — so
the ÷15 loop, HMOVE, and any edge/collision compare keep working untouched. Free bonus: because a
frame moves at most `Vel_int+1` px, choosing `Vel_int ≤ 1` makes tunnelling (skipping past a thin
paddle/wall in one frame) structurally impossible.

**The sign trap.** The carry adds `+1` *numerically*, but a leftward-moving object needs `−1`. You
cannot `adc #0` your way out of it: for `Vel = $FF` (−1) a carry gives `$FF + 1 = 0` = no move.
Branch on the direction (or keep a `±1` unit and `adc dir`). One clean form:

```
        lda Vel_int         ; here C already = the fractional carry
        bcc done            ; no carry → step = Vel_int
        bmi sub1            ; carry & moving left  → one more px left
        adc #0              ; carry & moving right → +1 (adc adds the set carry)  ← cheaper than clc/adc #1
        jmp done
sub1:   sbc #1              ; C is still 1 here → Vel_int − 1
done:   sta Step            ; signed per-frame step; position code does `Pos += Step`
```

**Budget placement.** The accumulate+sign is ~20-28cy. If the line that moves the ball is already
near the 76-cy wall (e.g. it also does collision + miss detection), don't inline it there — compute
**next** frame's `Step` on a slack housekeeping line (all paths converge there once per frame) so the
hot line stays just `Pos += Step` (same cost as the old `Pos += Vel`). One frame of latency on the
step is invisible. (In PONG the accumulator lives on physics row 5; row 2 only does `adc Step`.)

**Tier table (PONG rally, the shipped values).** Reset `Err` and `Vel_frac` on serve so every rally
starts at exactly 1.00.

| rally hits | px/frame | Vel_int | Vel_frac | step vs prev |
|---|---|---|---|---|
| 0–3 | 1.00 | 1 | `$00` (0)   | — |
| 4–7 | 1.25 | 1 | `$40` (64)  | +25% |
| 8–11 | 1.50 | 1 | `$80` (128) | +20% |
| 12+ | 2.00 | 2 | `$00` (0)   | +33% |

The old code went 1 → 2 → 3 (the 4th hit = +100%); this replaces the shock with a +20-25% ramp.
Human speed discrimination (Weber fraction) is ~10-25%, so +100% always reads as a "gear change"
while these steps read as smooth — matching what the arcade original actually did (it was never
integer-jumped; it ran 0.5 → 0.75 → 1.0).

**Verify it numerically.** Poke `Vel_int/Vel_frac/Err/Pos`, put the object in open space (no walls
/paddles on its path, `Vel_Y = 0`), step N frames, read `Pos`: the delta must equal
`round((Vel_int + Vel_frac/256) · N)` exactly, for both signs. Measured for pf2-06:
1.25 → +10 over 8 frames; 1.5 (left) → −12; 2.0 → +16. Exact.

**Origin.** 8bitworkshop `brickgame` DDA; the identical idiom is the fraction-then-carry propagation
in Breakout 1978 (`breakout.asm` 8.8 position, 2.6-packed speed) and djmips APong (8.8 throughout,
speed table `$80/$c0/$00` = 0.5/0.75/1.0). In-game verified in sandbox PONG
`steps/pong_top_paddle_pf2_06_feel-rally-ai-serve`. Standalone demo ROM + CI scenario: TODO (would
promote 🔶 → ✅).

## PAL/NTSC portability — a benefit this page did not claim (added 2026-09-03)

The Stella Programmer's Guide's line about conversion is usually quoted as an argument *for* 8.8
fixed-point position. Read to the end of its condition, it is not:

> If the NTSC version is designed with 2 byte fractional addition techniques **(or anything not based
> on frames per second)** to move objects, then PAL conversion can be as simple as changing the
> fraction tables.

**The condition is "not tied to frames per second", not "8.8".** A DDA carries its fraction in the
*velocity* instead of the *position*, and converts the same way — by swapping the increment table. So
the reason 8.8 is quoted here applies to this technique too, and this page had never said so: before
today it contained no mention of PAL, NTSC, or a refresh rate at all.

**The conversion factor, from our own constants** (`television/specification/specifications.go`):
NTSC is `15734.26 / 262` = **60.0544 Hz**, PAL is `15625.00 / 312` = **50.0801 Hz**, so a PAL increment
must be **83.39%** of the NTSC one to move at the same speed per second — 0.06 points from the nominal
50/60. Worth stating precisely, because the list did not: the author who raised it wrote *"just ensure
the NTSC m to be ~80%"* and then, parenthetically and unsurely, *"can someone provide the correct
value? 83,4%?"*. **The confident figure was 3.4 points out and the hesitant one was right to two
decimal places.**

⬜ Untested here, from the same thread: that for speeds of 0-2 px/frame the **zero** flag can replace
the carry, and for 0-4 px the **overflow** flag. No mechanism is given in the source and it has not
been reproduced.
