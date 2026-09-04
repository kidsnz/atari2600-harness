# Technique #12 — Venetian Blinds (intra-frame line interleaving)

**Goal:** two (or more) objects coexist in the *same vertical zone* through **one** player — with
zero flicker. Where flicker multiplexing (#10) time-slices across *frames* (30 Hz shimmer),
Venetian Blinds time-slices across *scanlines within one frame*: even lines draw object A, odd
lines object B, every frame, rock-stable at 60 Hz. The cost is the look: each object is striped
("blinds") at half vertical density. Bob Whitehead built *Video Chess* (1979) on this — 32 pieces
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

## Verified here (pixel-level, Gopher2600, locked in CI)
Adjacent rows read back alternating `[83+2 FFFFFE]` (diamond row $18, white) and `[80+8 AC1212]`
(frame row $FF, red) — two figures, one player, no flicker. Position, last-line color register,
262 lines and golden frame asserted in CI.
