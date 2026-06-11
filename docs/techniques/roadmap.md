# Techniques roadmap — what to absorb next (the TODO)

A prioritized, **tracked** list of 2600 authoring techniques to fold into the catalog. The main goal is
**general capability** — a verified, reusable toolkit — not any one game. Each item goes through the pipeline
in [`README.md`](README.md): learn → clean-room → verify → Stella → lock in CI → maybe promote to `pkg/`.
Sourced from the local corpus (`reference/docs_atari/`) + AtariAge/web. **Listing here = "documented intent",
not "verified"** — verification happens when its demo is built.

**Priority axes:** ① **general / foundational value** (how much it unlocks; how commonly needed) ·
② difficulty (★ easy … ★★★ hard) · ③ are the prerequisites already hardware-verified in our litmus set.
(A concrete game can be picked *flexibly* later as a testbed — it is not the organizing principle.)

## TODO (ordered: foundational + easy wins first)

- [x] **#1 Sprite multiplexing (vertical zones)** — many players past the 2-per-line limit. ★★ · prereqs ✅ · **done** (catalog #1).
- [ ] **#2 Sprite animation** — cycle GRP frames every N frames (walk cycles, blinking, wheels). ★ · prereqs ✅(sprite) · value **high** (ubiquitous). Src: spiceware Step 14.
- [ ] **#3 Vertical positioning + VDEL** — place a sprite at any Y smoothly; vertical-delay shadow registers. ★★ · prereqs: VDEL ⬜ · value **high** (foundational). Src: spiceware Step 5; Davie S23.
- [ ] **#4 2-line (double-line) kernel** — repeat each sprite line over 2 scanlines → CPU headroom for logic. ★★ · prereqs ✅(cycles/budget) · value **high** (foundation for complex kernels). Src: spiceware Step 4; `multisprite.inc`.
- [ ] **#5 48-px sprite + 6-digit score** — wide hi-res graphic via 3-copy + VDEL shadow, 6 timed GRP writes ("Six-Digit Score Trick" / Staugas kernel). ★★ · prereqs ✅(NUSIZ/sprite), VDEL ⬜ · value **high** (almost every game needs a score/title). Src: `bigsprite.asm`,`score6.asm`,`6digit.inc`; Bumbershoot 48px.
- [ ] **#6 Sound FX / music driver** — AUDC/AUDF/AUDV envelopes & note tables (resumes parked **A-3**). ★★★ · prereqs ✅(read_audio/golden) · value **high** (every game needs sound; best done with the author by ear). Src: spiceware Step 13; Slocum guide.
- [ ] **#7 LFSR pseudo-random** — cheap, repeatable randomness (spawns, patterns). ★ · prereqs n/a · value **med** (common utility). Src: spiceware Step 10; randomterrain.
- [ ] **#8 Playfield tricks** — asymmetric (non-mirrored) PF, score-mode, CTRLPF priority. ★★ · prereqs ✅(`pkg/playfield`) · value **med**. Src: `playfield.asm`; spiceware Step 7.
- [ ] **#9 Ball + missiles as objects** — use BL/M0/M1 as extra small movers (bullets, dots). ★ · prereqs ✅(missile/collide) · value **med**. Src: spiceware Step 11–12; `missiles.asm`.
- [ ] **#10 General multi-sprite kernel** — dynamic Y-sort + 2-of-N allocation + **flicker** past 2/line; the general form of #1. ★★★ · prereqs ✅(extends #1) · value **high** but advanced. Src: `multisprite2/3.asm`.
- [ ] **#11 Bank switching** — F8/F6/… schemes to break the 4K ROM ceiling. ★★★ · prereqs n/a · value **low now / needed once ROM grows**. Src: `bankswitching.asm`; `bankswitch_sizes.txt`.
- [ ] **#12 Venetian Blinds** — horizontal reuse + vertical interlacing (striped/flicker); Bob Whitehead, *Video Chess* 1979. ★★ · prereqs ✅(sprite) · value **low** (historical/curiosity). Src: Video Chess; AtariAge.

## Notes
- `reference/docs_atari/spiceware_tutorial/` (Darrell Spice Jr., *Let's Make a Game*, Steps 1–14) is a
  ready-made **general curriculum** that touches most of #2–#9 in build order — a strong execution guide,
  independent of any particular game.
- When a technique matures into reusable code, promote a generator to `pkg/` (like `pkg/playfield` / `pkg/sprite`).
- Pick a small concrete demo/testbed per technique **flexibly**; don't anchor the whole roadmap to one game.
