# Technique #10 — Flicker multiplexing (N objects through 2 players)

**Goal:** show more than two player objects *on the same scanlines*. Vertical zones (#1) reuse
players down the screen, but the hard wall remains: 2 players per line. Flicker breaks it in
*time* instead of space: each frame draws a different subset, alternating fast enough (30 Hz per
object with 2 subsets) that persistence of vision merges them. This is how Pac-Man's four ghosts
share two player slots — the famous 2600 ghost flicker is this technique.

Learned from (clean-room): `multisprite2/3.asm` discussions (8bitworkshop), AtariAge flicker
threads. Demo: `roms/techniques/flicker_multiplex.asm` — four bouncing color-coded balls, two
drawn per frame by frame parity — locked in CI by `scenarios/flicker_multiplex.json`.

## The technique
1. **Slots, not objects.** The kernel knows only two "slots" (P0, P1) with Y/X/color staged in
   VBLANK; it draws them with the any-Y compare kernel (#3 ×2 ≈ 49 cy/line — overlap-safe,
   no zone restrictions).
2. **Subset rotation.** Each frame, `frame & 1` picks objects {0,1} or {2,3} into the slots —
   positions, then colors, then one shared HMOVE (#4's staging discipline). Every object is
   visible 30 times a second.
3. **Per-object color rides the slot:** COLUP0/COLUP1 are re-staged with the subset, so four
   distinctly-colored objects coexist through two registers.

### The full form (documented, build when a game needs it)
Real engines improve on fixed pairs: **sort objects by Y each frame**, walk the screen assigning
the next-starting object to whichever player is free (re-positioning a player mid-screen after
its previous object ends), and only flicker the objects that actually collide on the same lines —
with a rotation counter so no object starves. Fixed-parity pairs (this demo) are the verified
core; sort + dynamic 2-of-N allocation + fairness rotation is the documented extension
(`multisprite.inc` family).

## Verified here (Gopher2600, locked in CI)
- Four objects (two vertical bouncers at X=40/120, two horizontal at Y=60/120), all four
  trajectories deterministic; 262 lines every frame; budget clean.
- **The flicker itself is asserted:** three consecutive frames read
  P0/P1 = (53,97) → (40,120) → (55,95) — odd frames carry the moving horizontal pair, even
  frames the fixed-X vertical pair.
