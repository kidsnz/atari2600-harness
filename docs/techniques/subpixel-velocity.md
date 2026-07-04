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
