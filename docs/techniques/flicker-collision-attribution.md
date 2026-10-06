# Technique — attributing a collision latch when objects are flickered or multiplexed

**Goal:** know *which* entity a set collision bit belongs to, when one TIA object is drawn as
several game entities. The hardware gives fifteen latched pairs and no way to ask "which copy?",
so attribution is something the kernel has to build.

**Status:** the *behaviour* is ✅ measured — `roms/litmus/litmus_flicker_attrib.asm`,
graded by `internal/emu/flickerattrib_test.go`, settles what the latches do across frames.
This page adds what that fixture does not carry: **why the problem exists, what it costs a
shipped game, and the three idioms the list converged on.** All three are ⬜ unverified here.

**Source:** Stella mailing list, four messages from **2000-05 to 2004-12** (`200005/msg00038` Mark De Smet, `200005/msg00043` Piero Cavina, `200007/msg00140` Thomas Jentzsch, `200412/msg00026` Nick Bensema). ★This line said *"eight years apart"* until 2026-09-06; the dates are in the archive and the span is **4.6 years**. A number that the document's own citations can settle should not be written out — write the endpoints and let the reader subtract.
`200005/msg00038` (Mark De Smet, 2000-05-04) states the mechanism; `200007/msg00140`
(Thomas Jentzsch, 2000-07-31) is the symptom in a finished game; `200412/msg00026`
(Nick Bensema quoting Lee Fastenau, 2004-12-02) is the temporal idiom; `200005/msg00043`
(Piero Cavina, 2000-05-05) is the coordinate idiom.
The section *Before 2000* below adds five messages from 1997-12 (`199712/msg00021`, `199712/msg00023`, `199712/msg00025`,
`199712/msg00026`, `199712/msg00028`), older than every message above.

## The failure, in a shipped homebrew

A player reported that a towed object could be swung **through** a solid one. The author's reply
is the whole problem in two sentences:

> I know, you can swing the pod through all other objects (**except the playfield**).
> I'm not sure, if i can fix this.
> — `200007/msg00140`

The exception matters: the playfield is not flickered, so PF collisions never miss. Everything
that *is* flickered can pass through everything else that is flickered, because on any given
frame at most one of the pair is being drawn.

## Why: the latches are blind to where you are in the frame

> The latches are **not in anyway linked to where in the frame drawing process you are**. …
> The latches will be set **every** time the two objects are on at the same time. They will stay
> set until you punch CXCLR. CXCLR clears the latches no matter what is going on, or when you do it.
> — `200005/msg00038`

and the consequence, stated in the same message:

> If you do a CXCLR once per frame … there will be **no way you can tell if the objects collided
> once, or 50 times, or even where in the screen they collided**. All you know is if they collided
> at least once.

So a once-per-frame CXCLR throws away exactly the information a multiplexed kernel needs. The
latch is a frame-wide OR, and the kernel is the only thing that can narrow it.

## Idiom A — partition the SCREEN (spatial)

> You can theoretically do different collision check every scan line, but you are of course limited
> by the cycles available. So, if you want to check for collisions seperately in the top of the
> screen, and at the bottom, you simply read off the colision registers in the middle, and do a
> CXCLR.
> — `200005/msg00038`

Read-then-CXCLR at a zone boundary and the latch belongs to the zone just finished. This is the
natural fit for a zone-multiplexed kernel (`zone-multiplexing.md`), where the boundary already
exists: the read costs one `LDA`/`BIT` per register per boundary, and the CXCLR one store.

## Idiom B — partition the FRAME (temporal)

> If you update the player position **every other frame**, then you can set **two collision bits,
> one per frame**. Right now, two high bits means platform collision. One high bit would mean
> ladder collision. Two low bits means no collision.
> — `200412/msg00026`

Here the flicker is not the problem, it is the channel: frame parity says which entity was on
screen, so one latched pair carries two questions. The cost is that a collision is answered at
30 Hz rather than 60 Hz, and that the two entities must never need to be tested on the same frame.

## Idiom C — do not ask the latch WHERE (coordinate)

Answering a version of Idiom A that reads `CXM0P`/`CXM1P` every 16 lines down an Air-Sea Battle
screen:

> There's a simpler way of doing this: remember that in a game like Air-Sea Battle each
> (pseudo)sprite has a fixed vertical position. You can check for collision between the missile and
> P0 just once in a frame, and select which target was hit just from the y-coordinate of the
> missile! misslY/16=index of the target that has been hit.
> — `200005/msg00043`

The latch answers *whether*; the missile's own Y, which the game already holds, answers *which*. One
read and one CXCLR per frame instead of one per band, so nothing is spent inside the visible region.
What it needs that Idiom A does not: every copy lives in a **fixed** band, so a band index is a
function of Y. The 16 is the earlier poster's guess (*"let's say, I didn't count"*), not a
measurement of Air-Sea Battle, and he did not know whether that game uses collisions at all. Cavina
adds *"I used a more elaborate version of this concept for Oystron"*.

## Before 2000 — the latch as a gate, a position test as the answer (Cited only, not verified)

Greg Troutman, 1997, on his game Rescue, answering Bob Colbert, whose shots were tested in software
against each sprite's rectangle (Colbert: it *"does not check to see if a particular pixel in the
rectangular area is "on""*, `199712/msg00021`):

> Rescue does both collision tests.  If the 2600 doesn't report the type of collision I'm testing, it
> just skips along, but if it does, then I bounds test to isolate which sprites/platforms are involved.
> — `199712/msg00023`

He thought that would give Colbert *"the full pixel-level collision checking you need"* without too much
extra work. Asked what a bounds test is, he answered the next day:

> So, when I say "bounds test" I mean I wait until an entire section of the screen is drawn, before
> checking the  collision register.  If it shows I collided, I still don't know which object it
> collided with, since it just passed through an area where one of the player graphic registers (and
> the missile and playfield graphic registers) are re-used multiple times.  In that event, I then test
> to see where the lander craft actually is in that frame to determine what it crashed into (or, where
> the lander's missile is located to determine which enemy it hit)...
> — `199712/msg00025`

That is Idiom A's section read followed by Idiom C's coordinate test, with the coordinate test run only
when the latch has fired. The same message gives the reason not to read inside the section — checking
*"repeatedly during the drawing of your screen"* tells you *"for a fact which copy"* was involved, but
*"when you've got a bunch of copies, and you're short of cpu cycles ... extra non-display code like
that is undesirable"* — and, in Colbert's kernel, the failure described above: objects whose graphics
are turned off on frames where more than one share a scanline are not reported on those frames,
*"So, the 2600's built-in collision detection is potentially unreliable."*
Piero Cavina replied that Oystron *"does a mixed hardware and bounds collision detection too"*: the
registers cleared before each of its 8 zones, checked after each zone's 16 scanlines and ORed into
memory, a VBLANK routine that handles the objects in a zone that hit, and an X-bounds check to find
which copy of a multiple-copy object was hit 〔`199712/msg00028`〕.

The same exchange has a third form, on paper only. Ruffin Bailey, who had asked what a bounds test is,
described his planned game when Troutman asked for details: a player sprite *"eight or nine bits
high"*, and *"placing the blocks that he's trying to collect at least 5 scan lines away from one
another (vertically) so that I only have to check the y-coordinate of my player to determine which
block it is with which he had collided"* 〔`199712/msg00026`〕. That is Idiom C with the layout chosen so
that Y alone decides. He had not yet assembled anything, and nobody in the thread assessed the plan.

## What this page does not settle

- **No idiom is measured here.** The litmus fixture covers what the latches do, not whether
  any of the three schemes survives a real kernel's cycle budget, and Oystron was not looked at.
- **The zone read costs cycles inside the visible region**, which is where a multiplexed kernel has
  none to spare; no budget for it has been proved with `prove_line_budget`.
- **Idiom B's 30 Hz answer** was not measured against a game's input latency requirement.
- The 2000 message says a per-scanline check is possible "theoretically"; nothing here shows a
  kernel that does it.
