# Technique #12 — Venetian Blinds (intra-frame line interleaving)

**Goal:** two (or more) objects coexist in the *same vertical zone* through **one** player — with
zero flicker. Where flicker multiplexing (#10) time-slices across *frames* (30 Hz shimmer),
Venetian Blinds time-slices across *scanlines within one frame*: even lines draw object A, odd
lines object B, every frame, rock-stable at 60 Hz. The cost is the look: each object is striped
("blinds") at half vertical density. Bob Whitehead built *Video Chess* (1979, with Larry Wagner, credited with the chess logic 〔stella-list `200011/msg00069`, `200312/msg00057`〕) on this — 32 pieces
on screen with two players and a lot of stripes.

Learned from (clean-room): Video Chess analyses, AtariAge history threads. Demo:
`roms/techniques/venetian.asm` — a white diamond and a red frame sharing one 64-line zone through
P0 alone — locked in CI by `scenarios/venetian.json`.

## The technique
Per zone line `s` (zone-local counter):
- parity `s & 1` picks the object: even → `GRP0 = ArtA[s/8]`, `COLUP0 = white`;
  odd → `GRP0 = ArtB[s/8]`, `COLUP0 = red`.
- Both stores land by ~29 cycles — before the display window — so color *and* shape swap cleanly
  per line: one player register pair renders two differently-colored figures.
- Art rows advance every 8 lines (`s>>3`), so each art row contributes 4 interleaved stripes.
- Why a power of two: it is what lets the row index be a shift (our reading; the source says
  *"Boolean math"*). Crackers (Chris Cracknell, 1997)
  weighed changing the playfield every 4, 6 or 8 lines (*"20x48 bits"*, *"20x32 bits"*,
  *"20x24 bits"*); Nick Bensema, in a reply Crackers forwarded to the list: *"Every six lines would
  be tricker than every four or eight lines. Six is not a power of two, so neat tricks with Boolean
  math won't work."* Crackers: *"Yeah, I kind of realized that last night"* 〔stella-list
  `199703/msg00147`, `199703/msg00153`〕. The rule is for deriving the row from the line counter, as
  `s>>3` does; a kernel that keeps its own count of lines per row is not bound by it (our reading).
  **Cited only, not verified.**

Trade-offs vs #10 flicker: blinds = stable but striped & half-density; flicker = full-bodied but
shimmering. Video Chess chose stripes; Pac-Man chose shimmer. Use blinds for static/dense scenes
(boards, HUDs), flicker for moving objects.

**They are not exclusive, and the third option is the one this page was missing.** Glenn Saunders,
stella-list `the-demo-image-series-9` (2003-02), on two objects too close to share a line:

```
XXXXXXXXXXXXX
                             XXXXXXXXXXXXXXXX
XXXXXXXXXXXXX
                             XXXXXXXXXXXXXXXX
```

> *"and then **alternate this even/odd pattern** so it would be like a **'closed venetian blind'**
> technique. That way **on every frame you'll have graphics on both sides**."*

Interleave the two objects by line — blinds — **and** swap which one owns the even lines each frame —
flicker. Neither object ever vanishes for a whole frame, which is what plain flicker does and what
the eye reads as blinking; what alternates instead is *which half of each object* is drawn. The
striping stays (half density, as above), but the shimmer moves from the object to its texture. Cost
is unchanged: it is the blinds kernel with one bit of frame parity added to the row test.

Untested here — no fixture combines them, and the claim above is a 2003 design sketch, not a
measurement. Recorded because the page previously read as a fork in the road.

**Interlace can comb a moving object at some speeds; the closed blind may too.** The case on the
list is interlace, not this page's kernel: Billy Eno's 2002 interlacing demo drew a ship whose graphic
*"gets 'torn' because of the way I am drawing it"*, and Glenn Saunders answered: *"If you move at
certain speeds you will get a comb effect because of the interlace. If you update the animation only
30 frames a second rather than 60 you'll be able to eliminate this since each frame the object will
be stationary and thus completely draw itself across both fields, but perhaps at the cost of
somewhat jerkier animation should the object need to move quickly."* Eno had drawn it at 30 fps
first and found it *"moving way too slow in the vertical direction"*; in the real game he would
*"probably just give up the smooth motion"* 〔stella-list `200208/msg00120`, `200208/msg00128`,
`200208/msg00129`〕. The closed blind above also draws each object's two halves on alternate frames,
so the same comb and the same 30 Hz fix should apply to it (our reading). **Cited only, not
verified.**

**A suggestion: let parity split jobs, not only objects.** Piero Cavina (1998), sketching a kernel for a
Jumpman-style game — the player on P0, prizes on a re-used P1 with varied NUSIZ, two dots on M0 and
the ball: *"you can't have lines where you draw the player AND reposition sprite 1 AND reload the
playfield AND activate the ball... so you'll need to think a lot in advance"*, and *"study carefully
a configuration where the evil dots don't have to be drawn in the same lines where you will
reposition Sprite1: for example, make sure that reposition-lines are always odd lines, and put the
dots in the other lines"* 〔stella-list `199803/msg00086`〕. That is this page's even/odd split with
jobs in place of objects (our reading); `restrobe-copies.md` names the same split and does not use
it. It was advice, not a build: Ruffin Bailey's reply took up the object allocation and not the
odd-line rule 〔`199803/msg00088`〕. **Cited only, not verified.**

## Verified here (pixel-level, Gopher2600, locked in CI)
Adjacent rows read back alternating `[83+2 FFFFFE]` (diamond row $18, white) and `[80+8 AC1212]`
(frame row $FF, red) — two figures, one player, no flicker. Position, last-line color register,
262 lines and golden frame asserted in CI.
