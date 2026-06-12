# Technique — procedural generation (LFSR-driven content)

**Goal:** deterministic "randomness" for spawns/terrain: an 8-bit Galois LFSR stepped on game
events, with the guarantee that **the same seed reproduces the same world** — and that the
sequence is verifiable against an off-target reference implementation.

Demo: `roms/techniques/procgen_demo.asm` (a marker spawns every 30 frames at an LFSR-derived X).
CI: `scenarios/procgen_demo.json` (RAM state + on-screen X match the reference sequence, golden).
Hardware basis: `litmus_lfsr` (v0.46.0; period 255, never-zero, `eor #$8E` taps verified).

## The pattern

```
        lda lfsr        ; step (Galois, right shift)
        lsr
        bcc NoTap
        eor #$8E
NoTap:  sta lfsr
```

- **Seed once, non-zero** (zero is the lock-up state; period is 255).
- **Step on events, not per frame** (here every 30 frames) so gameplay pacing controls the draw
  rate; step extra times for "discard" rolls when you need decorrelation.
- **Map, don't mod**: derive values by masking/offsetting (`and #$7F / adc #16` → X in 16..143).
  Masks keep the mapping branch-free and budget-friendly.
- **Reference-check the sequence**: the same LFSR in 5 lines of Python/Go gives
  `$5A → $2D, $98, $4C, $26, $13, …`; the scenario asserts RAM and the rendered marker X match
  exactly (verified: 45/61, 152/40, 76/92, 38/54 at spawns 1-4).

## Uses
- Enemy spawn positions/waves (starshot uses this for wave patterns).
- Terrain/maze generation: step per row/cell; bidirectional variants (Pitfall's left/right
  stepping LFSR) let you scroll both ways — documented in `docs/fundamentals-audit.md`.
- Attract-mode variety with a frame-counter-mixed seed at game start (keep the *gameplay* seed
  fixed if you want reproducible worlds).
