# Technique — the invisible probe (a hidden missile or ball as a hit box)

> **Before placing a probe, read `sprite-placement.md`'s rule table.** Rule 12 — a copy past clock
> 160 wraps to the left edge and draws on the same line — was measured and CI-locked on 2026-08-21
> and rediscovered from scratch while this page's own fixture was being built. Placement has its own
> document precisely so that a page about something else does not have to derive it again.
>
> **And before building a band to discover WHICH object drew a pixel, call `decompose_row`.** It
> returns run-length runs of `{clock, len, element}` across visible clocks 0..159 and describes
> itself as *"the attribution sibling of read_row"*. The calibration band in `litmus_pf0_reflect`
> exists because a probe's identity was unknown; one `decompose_row` call would have shown the
> quad-width `P1` starting at clock 0 immediately. Neither fixture built here calls it — the
> question asked was about attribution and the tool reached for was the one about colour.

**Goal:** get a hit test the hardware computes for you, on a region the hardware has no register
for. Park a missile or the ball on the region you want to test, read the collision latch, and hide
the object so the player never sees the instrument.

Demo: TODO — no standalone demo.
CI: TODO — no gate yet. Proposed litmus under "How to verify".
Hardware basis: **the hiding is not measured here.** The priority chain below is read from the
vendored engine's source, not from a litmus; whether a probe is *actually* invisible in a given
kernel is a per-kernel question and nothing in this repository answers it yet.

## Why it works

The 2600 gives you fifteen collision pairs and no way to ask "did the fist reach the head". But
missiles and the ball are **positionable one-clock objects that no colour register of their own**:

| object | takes its colour from | so it disappears over |
|---|---|---|
| `M0` | `COLUP0` (player 0) | player 0, and anything else drawn in `COLUP0` |
| `M1` | `COLUP1` (player 1) | player 1 |
| `BL` | `COLUPF` (playfield) | lit playfield, and the background if `COLUBK == COLUPF` |

〔`Gopher2600/hardware/tia/video/video.go:344` "priority 2 (missile 0 is same color as player 0)";
`:336` "priority 1 (ball is same color as playfield)"〕

So a missile riding on its own player is invisible **by construction** — there is no register to
set wrong. That is the whole trick.

**Why not test with the sprite itself: its hit shape is its picture, frame by frame.** bigmessowires
(AtariAge `topic/347059`, 2023) tested his player graphic against a playfield maze: *"for the same x,y
location of the character, some bitmaps may collide with the playfield and some don't"*, so turning
from vertical to horizontal movement swapped the bitmap and left the character *"partly stuck inside
the wall"*. One of his own options was this page's — *"a second TIA object under the main character
sprite that's square and the same color as the background, so it's invisible"* — which he called
*"Wasteful."* He first removed the sticking by redrawing every sprite, then moved the movement test to
a CPU bounding-box check — *"so much smoother and better"*, though it *"requires a ton of CPU
cycles"* where the TIA's is free — keeping hardware collision for *"determining when to activate or open or
destroy something that the player touched, but not for constraining the player's movement."*
**Cited only, not verified.**

**Or skip the hardware when the region is fixed.** A newcomer planning a hockey game asked how a
puck inside the net would be told from one hitting the boards 〔stella-list `200102/msg00325`〕.
Manuel Polik: *"Just forget about collision detection. You know the coordinates of the puck, you
know the coordinates of your net, just calculate if the puck is in or not. Whith clever use of an
offset table, you can even calculate if it's within a round shape"* — his example is one offset per
row of a curved edge (`0, 2, 3, 3, 3, 2, 0`), and *"Of course you don't need to do the zeros"*
〔stella-list `200102/msg00327`〕. Andrew Davie agreed: *"I wouldn't touch the hardware collision
registers with a barge-pole. Much more accurate to do it with simple software collision checking."*
〔stella-list `200102/msg00338`〕
**Cited only, not verified.**

**The software test itself needs no `CMP`.** The Stella mailing-list FAQ (updated October 2002 by
Mark Graybill) reprints Manuel Polik's check for *"whether a single pixel hit an 8\*11 square"*. Each
axis is two subtract-and-branch steps — `LDA bulletHorPos,Y` / `ADC #$08` / `SBC horPosP0,X` /
`BMI NoHit` / `SBC #$08` / `BPL NoHit` — and the vertical pair is `LDA verPosP0,X` /
`SBC bulletVerPos,Y` / `BMI NoHit` / `SBC #$0B` / `BPL NoHit`. For an invader only 6 pixels wide but
aligned on the left of the sprite, *"all that was required to change in this code was adopting the
height from #$0B to the invader height and replacing the hardcoded #$08 with a temporary variable."*
〔stella-list `200210/msg00268`〕 Our reading of the excerpt, not stated in the FAQ: it contains no
`CLC` or `SEC`, but traced one instruction at a time over positions 0–159 only the carry on entry is
undetermined; every later one follows from the code. With bullet minus player as `d`, the horizontal
window is `d` = −7..0 with that carry clear and −8..−1 with it set, so the whole 8-pixel window shifts
by one. The vertical pair always starts with the carry clear (it is reached only when `SBC #$08` went
negative, which clears it) and accepts player-minus-bullet 1..11, so the 11 rows do not shift.
**Cited only, not verified.**

## The three ways to hide a probe, and what each costs

**1 — Same colour (a missile on its own player).** Free: `M0` cannot be a different colour from
`P0`. *Cost:* the missile is only hidden where `COLUP0` is what is being drawn. Over the **other**
player, or over a playfield of a different hue, it shows as a one-to-eight-clock dot. And that
missile is now spent — it cannot also be a bullet.

**2 — Priority (the ball under a player).** Also free, because it is the default. The engine's
normal chain is

```
P0 > M0 > P1 > M1 > BL > PF > BG        〔video.go, the "normal priority" branch〕
```

so a ball underneath either player is covered by it. *Cost:* **you give up `CTRLPF` D2.** Setting
the playfield-priority bit reorders the chain to `PF/BL > P0/M0 > P1/M1 > BG` — the ball rises
**above both players** and your instrument becomes a visible dot on everyone's chest. A technique
that hides by priority and a kernel that wants PF priority cannot share a screen.

**3 — `COLUPF == COLUBK`.** The ball shares the playfield colour, so making the playfield the same
colour as the background hides it everywhere. *Cost:* this is the expensive one — **the playfield
becomes invisible too**, everywhere on the screen, for every line where it holds. Worth it for a
game whose background is a flat colour; ruinous for one that draws with the playfield.

The coupling runs the other way too: hiding the playfield hides whatever else is drawn in `COLUPF`.
Erik Mooney hit it in *INV+* (2004), where the invisible-invaders game also made the bombs invisible.
His partial fix: *"On the scanlines \*between\* invaders, I can set COLUPF to gray to make the bomb
visible. It looks a bit ugly, but it's better than not seeing the bombs at all."* 〔stella-list
`200404/msg00197`〕 That the invaders and the bombs both take `COLUPF` is our reading of that fix.
**Cited only, not verified.**

**Hidden by colour is still drawn.** A colour changes what an object looks like, not whether the TIA
draws it, so it keeps its place in the priority chain (our reading of the chain above). Erik Mooney's
*INV+* changelog, under *"Fixed a bug"*: *"in a two-player game, if player 0 got game over, his sprite
continued to be drawn in black which would obscure player 1 if P1 moved to the spot where P0 died."*
〔stella-list `200405/msg00002`〕 That the black was the background colour, so P0 itself was unseen, is
our reading. **Cited only, not verified.**

**A fourth, implicit in the source: enable the probe only on the lines it is testing.** A hit box
is a few scanlines tall, so `ENAM0`/`ENABL` is set for those lines and clear for the rest. This is
not really a hiding method — it is what makes the other three cheap, because the exposure is a
handful of pixels rather than a whole sprite.

**Not a hiding method: `VBLANK`.** The latches are set only while `VBLANK` is off (`known-traps.md`,
the row *"collision latches are not set while `VBLANK` is on"*; `verified-coverage.md` under
Collisions), so blanking a probe also blinds it. ZackAttack, sketching how a ball's position could be
read back in overscan: *"Don't forget collisions are only detected when vblank is disabled. So you'd
need to color everything black during the detection phase to avoid visible artifacts."* (AtariAge
`topic/279317`, 2018). **Cited only, not verified** — the engine gates the latches on `!vblank` in
`video.go`, and no litmus here overlaps two objects under `VBLANK`.

## The same colour sharing, used for drawing

- **A black missile as a mask over the playfield.** Glenn Saunders, with no time to rewrite the
  playfield between the left and right half of the screen for a gear indicator beside his score:
  *"I may try dropping a black missile over part of the playfield"* 〔stella-list `200508/msg00167`〕;
  in his next build, *"I added the missiles in. Since they are a mask, you only see the copy of each
  missile that is on top of the playfield."* 〔stella-list `200508/msg00174`〕 Our reading: this is
  method 1's cost used on purpose — the missile shows only where what lies under it is another
  colour — and since a missile takes its player's colour, that player is black on those lines too.
  **Cited only, not verified.**
- **The ball as a finer playfield edge.** sohl, replying in a thread about a Bruce Lee mockup: *"The
  ball is only available once per scanline, and is the same color as the playfield foreground
  (COLUPF), but can be a few different widths. Ball width of one or two color clocks (= 1 or 2 sprite
  pixels) can help smooth out playfield "pixels", which are 4 color clocks wide each."* johnnywc, in
  the same thread, suggested the missiles (*"3 copies each"*) to smooth the mountain tops and the ball
  for the playfield mountain (AtariAge `topic/347106`, 2023-01-27). **Cited only, not verified.**

## The same technique has different requirements in a litmus and in a game

This repository uses the identical idiom for a different purpose, and the requirements barely
overlap:

| | in a **litmus** | in a **game** |
|---|---|---|
| must the probe be invisible? | **no** — nobody is looking at the picture | **yes**, or it is a visible bug |
| must its position be exact? | **yes** — a probe one column off measures the wrong thing | roughly — a hit box is a design choice |
| must its response be calibrated? | **yes, in both directions** (1 when it should be, **0 when it should not**) | rarely — a wrong hit box is a gameplay complaint, not a wrong number |

**Both halves are load-bearing in their own context and dead weight in the other**, which is why
the technique reads as two different techniques depending on who is writing. `roms/litmus/litmus_pf0_reflect.asm`
carries a probe that is deliberately visible and calibrated in both directions before it is trusted
(its band 0); a game wants the opposite trade.

The calibration point is the one worth carrying across: **nothing in this repository measures a
collision field returning 0 when the objects do not overlap.** `roms/litmus/scenarios/collide_all.json`
asserts all fifteen pairs `== 1` with everything overlapped at the left edge and has no `== 0`
assert at all, so a probe's negative direction is unverified. A game can live with that; a
measurement cannot.

## Beyond hit boxes: the latches and the screen as instruments

A probe answers "did these overlap". The collision latches have also been used to answer questions
the CPU has no register for, and the picture itself to show what a register write did.

- **Reading a counter the CPU cannot read.** JeremiahK, 2017, wanting the power-on object positions as a
  random seed: *"the objects' positions are determined by timers which you can only reset, not read, you
  would have to use collision detection against the playfield."* His test program: *"enabling only one
  "pixel" in the playfield, and also one "pixel" in Player 1. Then I shift Player 1 to the right by 1
  color clock over and over until a collision is detected, keeping track of the number of shifts."* With
  the playfield copied that gave *"only a value from 1-80, not 1-160"*; telling the halves apart by
  re-checking with the mirrored playfield is a step he described, not one he posted. He called it
  *"kind of pointless"* next to seeding from `INTIM` (AtariAge `topic/273214`). **Cited only, not
  verified.**
- **A property of the console, and the trap in it.** Christopher Tumber's 2002 PAL/NTSC detector
  runs *Kool-Aid Man*'s score code for one frame and takes `CXPPMM` D7 as the answer 〔stella-list
  `200211/msg00098`, `200211/msg00100`〕. It did not measure what it was named for. Eckhard Stolberg
  replied that the effect it reads *"only affects certain TIAs"* and *"happens on PAL and NTSC
  consoles alike"*; to him all PAL 7800s *"seem to have the problem"* and 6- and 4-switch consoles
  *"seem to be unaffected"* 〔stella-list `200211/msg00116`〕. Yet a PAL woody (a model he counts
  as unaffected) and a PAL 7800 both came up NTSC 〔stella-list `200211/msg00110`,
  `200211/msg00131`–`00132`〕. The quotes and the other testers' reports are in
  `design-principles.md`, the bullet "Detecting the standard from inside the ROM was tried in 2002,
  and it did not work". **Cited only, not verified.**
- **The screen itself as the instrument.** Andrew Davie, on sprite writes that had *"gone wonky on
  the 2nd sprite"* in *Qb* — *"If I can't fix it, I won't mind too much"* — pointed to *"a few
  switches at the top of the code which allow you to set the destination for sprite data - set it to
  COLUBK to see timing in colour on the screen itself"* 〔stella-list `200102/msg00256`〕. Reading it
  needs no emulator tool, only the picture (our reading). **Cited only, not verified.**

## How to verify (proposed — not yet run)

1. **Litmus for the hiding, not the sensing.** Place `M0` on `P0` and `BL` under `P1`, run a frame,
   and assert with `read_row` that the probe columns are **identical to the same columns with the
   probes disabled**. That is the claim "invisible" actually makes, and it is checkable.
2. **Negative control for each hiding method.** (a) move `M0` off `P0` → the row must differ;
   (b) set `CTRLPF` D2 → the ball must appear; (c) set `COLUPF != COLUBK` → the ball must appear.
   Three controls for three methods; a method whose control does not fire was never doing anything.
3. **What this cannot settle.** Whether a probe is invisible *on a television* — the emulator's
   pixel equality is stricter than a CRT, so passing here is necessary and not sufficient.

## Sources

- **Chris Cracknell, Stella mailing list, 11 Nov 1998** (`reference/stella-list/199811/msg00041.html`),
  on Eckhard Stolberg's fighting-game demo:

  > "maybe you could put a missile graphic in the player's fist and a ball graphic in the three areas
  > of the opponent's body that would score a hit. A missle/ball collision would count as a hit. The
  > missle graphic would be the same colour as the player so it wouldn't show up, and if the priority
  > of the ball was under the player and the PF and BG were set to the same colour it wouldn't show
  > up either."

  〔distilled at `reference/stella-list/threads/beat-em-up-08-0c15/notes.ja.md`〕

- The priority chain and the shared colour registers are read from
  `Gopher2600/hardware/tia/video/video.go`; the costs above are derived from that chain, not measured.
