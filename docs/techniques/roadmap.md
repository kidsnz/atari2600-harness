# Techniques roadmap — what to absorb next (the TODO)

A prioritized, **tracked** list of 2600 authoring techniques to fold into the catalog. The main goal is
**general capability** — a verified, reusable toolkit — not any one game. Each item goes through the pipeline
in [`README.md`](README.md): learn → clean-room → verify → Stella → lock in CI → maybe promote to `pkg/`.
Sourced from the local corpus (`reference/docs_atari/`) + AtariAge/web. **Listing here = "documented intent",
not "verified"** — verification happens when its demo is built.

**Priority axes:** ① **general / foundational value** (how much it unlocks; how commonly needed) ·
② difficulty (★ easy … ★★★ hard) · ③ are the prerequisites already hardware-verified in our litmus set.
(A concrete game can be picked *flexibly* later as a testbed — it is not the organizing principle.)

**STATUS: all 12 items verified (2026-06-12, v1.8.0).** The roadmap is complete — every technique
below has a CI-locked demo or is verified inside the Exerciser. Future additions go through the
same pipeline; "documented" refinements noted per item remain available when a game needs them.

## TODO (ordered: foundational + easy wins first)

- [x] **#1 Sprite multiplexing (vertical zones)** — many players past the 2-per-line limit. ★★ · prereqs ✅ · **done** (catalog #1).
- [x] **#2 Sprite animation** — cycle GRP frames every N frames + free REFP flip. ★ · **done** (catalog: `sprite-animation.md`; demo `sprite_anim.asm` with pos(v)=v calibrated positioner, locked by scenario+golden in CI).
- [x] **#3 Vertical positioning** — any-Y placement via per-line compare kernel. ★★ · **done** (catalog: `vertical-positioning.md`; demo `vertical_pos.asm`, pixel rows verified bit-for-bit vs art; DCP/skipDraw variant documented for cycle-starved kernels).
- [x] **#4 2-line (double-line) kernel** — each art row over 2 scanlines → CPU headroom. ★★ · **done** (catalog: `two-line-kernel.md`; demo `two_line_kernel.asm`: P0+gradient on line A / P1 on line B, shared single HMOVE for both players — double-strobe pitfall documented; VDEL odd/even refinement documented-only).
- [x] **#5 48-px sprite + 6-digit score** — wide hi-res graphic via 3-copy + VDEL shadow, 6 timed GRP writes ("Six-Digit Score Trick" / Staugas kernel). ★★ · **done** (Exerciser title: 48px "EXRCSR" + live BCD score, timed stores recalibrated for centered X; locked by `m2_title` golden in CI; litmus_48px/_48px6).
- [x] **#6 Sound FX / music driver** — AUDC/AUDF/AUDV envelopes & note tables. ★★★ · **done** (Exerciser: 2-channel music driver with Sequencer-Kit note codec + kick-drum SFX, `pkg/audio` tuning math, locked by `m7_music` golden_audio). *Composing real tunes by ear with the author remains a future joint session.*
- [x] **#7 LFSR pseudo-random** — cheap, repeatable randomness (spawns, patterns). ★ · **done** (litmus_lfsr; applied in the Exerciser procedural scene v1.2.0 — starfield + AND-cascade mountain ridge from one-byte seeds; documented caveat: consecutive steps correlate, cap or decorrelate when masking).
- [x] **#8 Playfield tricks** — asymmetric ✅ (pf_async/zone) + reflect ✅ (proc mountains) + **score-mode & priority done** (catalog: `pf-modes.md`; demo `pf_modes.asm` — same PF pattern reads back COLUP0-left/COLUP1-right, and the wall flips from behind to in front of P0, all pixel-verified).
- [x] **#9 Ball + missiles as objects** — use BL/M0/M1 as extra small movers (bullets, dots). ★ · **done** (Exerciser playground: auto-firing missile with per-frame HMOVE drift + ball pole + collision-latch color feedback, locked by `m4_playground`; litmus_collide_all).
- [x] **#10 Multi-sprite** — flicker core (`flicker-multiplexing.md`) **and the full dynamic form verified** (catalog: `dynamic-multisprite.md`; demo `dyn_multisprite.asm`: sorting network + dynamic 2-of-N slot queues with sentinel + mid-screen timed-RESP repositioning + TIM64T VBLANK; zero visible budget spills, all 5 objects proven by ingest).
- [x] **#11 Bank switching (F8)** — break the 4K ROM ceiling. ★★★ · **done for F8** (litmus_bank pattern: vectors+reset stub in every bank, same-location switch zones; the whole Exerciser is a live F8 2-bank cart, `bank.number` asserted in CI). F6/F4/larger schemes remain documented-only.
- [x] **#12 Venetian Blinds** — intra-frame line interleaving: 2 figures through 1 player, zero flicker, striped look (Whitehead, *Video Chess* 1979). ★★ · **done** (catalog: `venetian-blinds.md`; demo `venetian.asm` — alternating rows pixel-verified).

## Notes
- `reference/docs_atari/spiceware_tutorial/` (Darrell Spice Jr., *Let's Make a Game*, Steps 1–14) is a
  ready-made **general curriculum** that touches most of #2–#9 in build order — a strong execution guide,
  independent of any particular game.
- When a technique matures into reusable code, promote a generator to `pkg/` (like `pkg/playfield` / `pkg/sprite`).
- Pick a small concrete demo/testbed per technique **flexibly**; don't anchor the whole roadmap to one game.
