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

## Verified here (pixel-level, Gopher2600, locked in CI)
Adjacent rows read back alternating `[83+2 FFFFFE]` (diamond row $18, white) and `[80+8 AC1212]`
(frame row $FF, red) — two figures, one player, no flicker. Position, last-line color register,
262 lines and golden frame asserted in CI.
