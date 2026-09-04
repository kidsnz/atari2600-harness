# Technique — attributing a collision latch when objects are flickered or multiplexed

**Goal:** know *which* entity a set collision bit belongs to, when one TIA object is drawn as
several game entities. The hardware gives fifteen latched pairs and no way to ask "which copy?",
so attribution is something the kernel has to build.

**Status:** the *behaviour* is ✅ measured — `roms/litmus/litmus_flicker_attrib.asm`,
graded by `internal/emu/flickerattrib_test.go`, settles what the latches do across frames.
This page adds what that fixture does not carry: **why the problem exists, what it costs a
shipped game, and the two idioms the list converged on.** Both idioms are ⬜ unverified here.

**Source:** Stella mailing list, three messages eight years apart.
`200005/msg00038` (Mark De Smet, 2000-05-04) states the mechanism; `200007/msg00140`
(Thomas Jentzsch, 2000-07-31) is the symptom in a finished game; `200412/msg00026`
(Nick Bensema quoting Lee Fastenau, 2004-12-02) is the temporal idiom.

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

## What this page does not settle

- **Neither idiom is measured here.** The litmus fixture covers what the latches do, not whether
  either partitioning scheme survives a real kernel's cycle budget.
- **The zone read costs cycles inside the visible region**, which is where a multiplexed kernel has
  none to spare; no budget for it has been proved with `prove_line_budget`.
- **Idiom B's 30 Hz answer** was not measured against a game's input latency requirement.
- The 2000 message says a per-scanline check is possible "theoretically"; nothing here shows a
  kernel that does it.
