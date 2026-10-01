# Sub-pixel velocity (DDA error accumulator)

**Problem.** An object's position is a 1-byte integer, so the only speeds you can express by
`pos += vel` are whole pixels per frame: 1, 2, 3… The jump from 1 to 2 px/frame is **+100%** — a
speed doubling the player feels as a lurch. Every classic that ramps ball speed (arcade Pong 1972,
Breakout 1978, faithful ports like djmips APong) avoids this by running **sub-pixel** speeds
(0.5 / 0.75 / 1.25 / 1.5 …). The naïve fix — widen position to 16-bit 8.8 fixed-point — forces you
to rewrite every consumer of the position (here: the ÷15 coarse-positioning loop and its HMOVE
fine-adjust, which read the integer X directly). Too invasive.

**The other integer route is slower, but it steps.** Moving 1 px once every N frames gives speeds
below 1 px/frame, still in whole-pixel jumps. A 2024 beginner tried both: *"Double
incrementing/decrementing the x position per update is easy and doubles the speed"*, and a counter
that waits frames before moving *"works okay at every other frame, but once you get to updating only
every 4 frames the movement gets choppy"*. The answer he got was that speed lives in *"how you change
the value of the player position variable, not about the divide loop"* 〔AtariAge `topic/359910`〕.
So integers alone offer whole multiples of 1 px/frame, or 1 px every N frames. A fraction adds the
speeds in between (0.75, 1.25, …); a speed of exactly 1/N px/frame comes out of the DDA as the same one
step every N frames, so for that case the fraction changes nothing. **Not verified** — the choppiness at N = 4 is his report; nothing here has measured it.

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

**That is the size of a step; the interval between steps is a second axis this table does not touch.**
Thomas Jentzsch, 2016, on playfield scrolling, which normally moves in 4-pixel steps: *"Depending on
the game, you can create the illusion of smoothness. You just have to make sure that you scroll at
least at 30Hz, even if it is 4 pixel at a time."* The same thread credits two homebrews' smoothness to
scrolling *"at 30 FPS like Television"* (mr-sql), and has a poster holding that smooth playfield
scrolling is not reachable at all (maiki) — answered in the same thread with a 1-pixel playfield
scroll built from four kernels (iesposta, linking AtariAge `topic/224946`), which zackattack reports
working on a regular CRT and misplacing one or more of the four on newer sets that "correct" the signal
〔AtariAge `topic/249185`〕. **Not verified** — no step size
or rate has been put on a screen here; the test would be 4-pixel steps at 30 Hz through `cmd/crtview`.

**Friction caps the speed without a clamp.** Each frame, subtract the velocity shifted right by n
(`vel −= vel >> n`) as well as adding thrust or gravity. The loss grows with speed, so the two balance
at `vel = force · 2^n` and the object stops accelerating there by itself — a top speed nobody wrote as
a constant. Thomas Jentzsch, 2006, from Thrust, SWOOPS! and Cave1K: with friction off, the
helicopter-style control becomes practically unplayable 〔AtariAge `topic/90087`〕. **Cited only, not
verified** — that thread is held here as distilled notes, not its text; the balance point is
arithmetic on the idealised form, and an integer shift truncates, so a real top speed settles near it,
not on it.

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

**What the swapped table holds, from 2003.** Thomas Jentzsch, reviewing a climbing game, suggested a
fractional per-frame counter: `clc / sbc currentSpeed / sta playerMotion`, moving on the frames where
the subtraction borrows — `SBC #$60` borrows on 3 frames in 8, a speed of $60/$100 〔stella-list
`200304/msg00221`〕. His table: *"NTSC: $40-1, $55-1, $80-1, $100-1 ; for PAL use 6/5 of the NTSC
value PAL: $4c-1, $66-1, $9a-1, ???"* 〔`200304/msg00200`〕. The `-1` pays for the `clc` (*"I am using
CLC to be able to subtract $100 (by using $ff)"*, 〔`200304/msg00221`〕), and the `???` is the catch:
6/5 of `$100` does not fit in a byte, so the fastest NTSC speed has no PAL entry. In this form the
speed is the subtracted value over 256, so the hex table's PAL/NTSC ratios (computed: 1.1875, 1.2000,
1.2031) are speed ratios and point the right way. His second form — *"subtact a constant value (e.g.
30) from playerMotion and add the level speeds (*30)"*, with `30*4-1 … 30*1-1` against `36*4-1 …
36*1-1`, and *"the conversion to PAL works best when it can be divided by 5"* 〔`200304/msg00221`〕 — does
not carry over the same way. As described, the object moves once per `level × 30` subtracted, i.e. every
n frames on NTSC, and the PAL values make that every 36n/30 = 1.2n frames: **slower on PAL, not faster**
(a simulation of the described loop gives 0.83–0.86× the NTSC rate). Whether the 36 belongs on the
subtracted constant instead is not settled in the thread. 6/5 is the nominal 60/50. **Cited only, not
verified** — no ROM was built with either table.

**The conversion factor, from the 2600's own clocks** (the colour clock over 228 colour clocks per
line — `pkg/audio`'s `BaseClockNTSC` and `BaseClockPAL` × 114, which agree with the engine's
`hardware/clocks` CPU clocks × 3): NTSC is `3579545 / 228 / 262` = **59.9227 Hz**, PAL is
`3546894 / 228 / 312` = **49.8607 Hz** (the PAL colour clock is **Cited only, not verified**: `pkg/audio`
records no source for 3546894, and the engine's `hardware/clocks` only cites a taswegian.com page for its
`PAL = 1.182298`), so an NTSC increment must be **83.21%** of the PAL one to move at
the same speed per second (a PAL increment is 120.18% of the NTSC one) — 0.12 points from the nominal
50/60. Until 2026-10-01 this page took the engine's `television/specification/specifications.go` rates,
`15734.26 / 262` = 60.0544 Hz and `15625.00 / 312` = 50.0801 Hz, and gave 83.39%: those are broadcast
line rates over the 2600's line counts, and the 2600's lines are 15,699.8 and 15,556.6 Hz (NTSC's broadcast line is 227.5 colour
clocks, the 2600's 228). Worth stating precisely, because the list did not: the author who raised it wrote *"just ensure
the NTSC m to be ~80%"* and then, parenthetically and unsurely, *"can someone provide the correct
value? 83,4%?"*. **The confident figure was 3.2 points out and the hesitant one 0.2.**

★**That factor is for CONSTANT VELOCITY. Anything that accelerates is off by the SQUARE of it.** With
`vel += g` and `pos += vel` once per frame, distance goes as the square of the frame count, so the
same code travels **1.4443×** further per second on NTSC than on PAL — not 1.2018×. A 2004 author felt
it as gravity and shipped a second build rather than retune: *"THE GRAVITY IN THE NTSC VERSION IS
EFFECTIVELY **1.4x GREATER**. IT'S THE ONE CONSTANT I COULDN'T CHANGE… So anyway, I'VE INCLUDED A
PAL60 VERSION"* 〔`200409/msg00309`〕. So **a PAL game ported to NTSC by scaling every velocity constant by
83.21% will still fall wrong**, and the acceleration constant needs 69.24% (83.21% squared).

★★**And the premise both numbers rest on is now measured, not assumed**
(`internal/emu/palphysics_test.go`, `roms/litmus/litmus_pal_physics.asm`): the same ROM produces
**byte-identical positions per frame** under NTSC and PAL at 50, 100 and 150 frames. PAL's extra fifty
scanlines change how long a frame lasts and nothing about what happens inside one, so the whole
difference is temporal — which is what lets a rate ratio stand in for the physics at all.

⬜ Untested here, from the same thread: that for speeds of 0-2 px/frame the **zero** flag can replace
the carry, and for 0-4 px the **overflow** flag. No mechanism is given in the source and it has not
been reproduced.

## What a region conversion actually costs, in bytes (2026-09-07)

The sections above say what has to change when a game moves between 50 Hz and 60 Hz. This is what it
came to in practice, measured on two conversions that ship all three builds at 16,384 bytes each:

| | NTSC vs PAL | NTSC vs PAL60 | **PAL vs PAL60** |
|---|---|---|---|
| Double Dragon 2b | 1333 | 1324 | **9** |
| Montezuma's Revenge 2B | 188 | 180 | **8** |

★**The time conversion is eight or nine bytes. Everything else is colour and data.** PAL→PAL60 changes
only the frame's length; NTSC→PAL changes that *and* the palette, and the difference between the two
columns is what the palette costs — **179 bytes in one game and 1324 in the other**, so it is not a
fixed price, it is however much of the ROM happens to be colour.

★★**And those eight bytes are three constants, patched once per bank.** In Montezuma the same three
values recur: `$1B→$19` (−2) three times, `$32→$15` (−29) three times, `$38→$1B` (−29) twice — an F6
cartridge holds a copy of the frame code in several banks and each copy needs the same edit. Double
Dragon has two such constants (`$41→$21`, `$52→$36`) plus a seven-byte run of `$FF`/`$00` flips at
`$0C0A`–`$0C15`. **The values are all line-count sized.** So the conversion is cheap to *make* and easy
to get *partly* right: miss one bank and the game runs at two different speeds depending where it is.

★★★**What this is NOT.** These are conversions from `reference/disassemblies`' hacks collection —
somebody's port, not Atari's shipped PAL release. What was measured is **what a converter changed**,
which is a different question from **how two official versions differ**. Byte counts only; no
disassembly was read. Found by the mailing-list distillation (helper-2) and re-measured here.

## Three rungs, and the game with no fraction to rescale (2026-09-30)

spiceware, 2013, laid conversion out as a ladder with a different price on each rung: *"The simplest
is to create a PAL60 game by only changing the colors. More complex is to also change the scan line
count, to achieve PAL's 50 frames per second, and to implement fractional positioning (if you haven't
already) so that the game plays at the same rate it does now. If you change the scan lines w/out the
fractional positioning then the game will be slower when played on a PAL system."* He also names what
keeps people on the first rung: *"The hassle of Fractional Positioning is part of why it's common to
leave the PAL version running at NTSC's 60 frames per seconds"* 〔AtariAge `topic/213265`〕. The
asker's price was RAM — *"I'm not using fractional positioning. I'm not sure I can free up that much
extra RAM, I would need 17 more bytes of data"* — so on a 128-byte machine keeping the speed can be a
RAM decision. **Cited only, not verified.**

**The first rung, made cheap at assembly time.** spiceware again: *"use color constants everywhere …
Then set COMPILE_VERSION to NTSC or PAL to select which build you're going to make. You'll end up with
a PAL60 version"* 〔AtariAge `topic/238183`〕. The byte table above is why this rung is the one worth
automating: the palette is most of what differs between the builds. **Cited only, not verified** — no
build here is assembled that way.

**The third rung when speed lives in a frame counter, not a fraction.** Everything above assumes a
fraction to rescale. Thomas Jentzsch, 2017: *"many developers back then and even today choose to base
their updates (movement, animation etc.) on a counter which is updated every frame … a NTSC to PAL-50
conversion based on a frame counter will slow down by ~17%."* His fix runs the counter faster —
*"Every 5th frame the counter has to be increased twice"* — without skipping the frames the game keys
on: Pitfall! updates every 2nd, 4th, 8th and 128th frame. His code increments again whenever the
counter lands on 83, on 19 mod 128, on 3 mod 32 or on 7 mod 8, in *"no extra RAM and 24 bytes ROM"* (his
figure; summed here from the instruction sizes it is 26, which fits Omegamatrix's *"25 bytes"* for a
one-byte-shorter version — **Not verified**) 〔AtariAge `topic/267100`〕. Counted here from those comparisons (a script over the 256 counter values,
not a ROM): 43 of every 256 values are skipped, so the counter advances 256/213 = 1.2019 per frame.
Over 500 frames it advances 598 to 603 depending on the starting value (601 most often, from all 256
starts) against 6/5's 600 — the size of the *"off by just 2"* he reports, which is one start's result. **Every value it
skips is odd**, so an update that fires on a multiple of 2^n is never skipped. What it does not catch,
by his own list: updates that run every frame, and updates not on a 2^n boundary. Omegamatrix's `asl`
variant in the same thread is a byte shorter; the same script finds it skips the same 43 values.
**Cited only, not verified** — the PAL-50 build he attached was not run here, and *"99.66...%
identical"* is his figure.
