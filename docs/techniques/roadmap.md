# Techniques roadmap — what to absorb next

A prioritized survey of 2600 authoring techniques to fold into the catalog (each will go through the
pipeline in [`README.md`](README.md): learn → clean-room → verify → Stella → lock in CI → maybe promote to
`pkg/`). Sourced from the local corpus (`reference/docs_atari/`) + AtariAge/web. **Listing a technique here is
"documented intent", not "verified"** — verification happens when we build its demo.

**Priority axes:** ① contribution to the North Star (Monet Frogger: lanes of cars + lily pads, frog hops,
score, per-scanline color, hop/splash SFX) · ② difficulty (★ easy … ★★★ hard) · ③ are the prerequisites
already hardware-verified in our litmus set.

| # | Technique | Formal name / aka | One line | Diff | Frogger | Prereqs verified? | Key sources |
|---|---|---|---|---|---|---|---|
| 1 | **Sprite multiplexing (vertical zones)** | multi-sprite kernel | many players via per-band P0/P1 reuse | ★★ | **high** | ✅ pos/hmove/sprite | ✅ **done** (catalog #1) |
| 2 | **48-px sprite + 6-digit score** | "Six-Digit Score Trick" / Staugas kernel | wide hi-res graphic via 3-copy + VDEL shadow, 6 timed GRP writes | ★★ | **high** (score) | ✅ NUSIZ/sprite; VDEL ⬜ | `bigsprite.asm`,`score6.asm`,`6digit.inc`; Bumbershoot 48px |
| 3 | **2-line (double-line) kernel** | — | repeat each sprite line over 2 scanlines → CPU headroom | ★★ | **high** (foundation) | ✅ cycles/budget | spiceware Step 4; `multisprite.inc` |
| 4 | **Vertical positioning + VDEL** | — | place a sprite at any Y smoothly; vertical-delay shadow | ★★ | **high** | ⬜ VDEL | spiceware Step 5; Davie S23 |
| 5 | **Sound FX / music driver** | (Slocum) | AUDC/AUDF/AUDV envelopes; hop/splash/win | ★★★ | **high** | ✅ read_audio/golden | spiceware Step 13; resumes parked **A-3** |
| 6 | **Sprite animation** | — | cycle GRP frames per N frames (frog hop, car wheels) | ★ | **high** | ✅ sprite | spiceware Step 14 |
| 7 | **Playfield tricks** | asymmetric / score-mode / priority | non-mirrored PF, CTRLPF priority/score | ★★ | med | ✅ playfield (`pkg/playfield`) | `playfield.asm`; spiceware Step 7 |
| 8 | **Ball + missiles as objects** | — | use BL/M0/M1 as extra small movers | ★ | med | ✅ missile/collide | spiceware Step 11–12; `missiles.asm` |
| 9 | **LFSR pseudo-random** | — | cheap repeatable randomness (traffic/spawn) | ★ | med | n/a | spiceware Step 10; randomterrain |
| 10 | **General multi-sprite kernel** | sort/position/display + **flicker** | dynamic Y-sort + 2-of-N allocation, flicker past 2/line | ★★★ | med | ✅ (extends #1) | `multisprite2/3.asm` |
| 11 | **Venetian Blinds** | (Bob Whitehead, *Video Chess* 1979) | horizontal reuse + vertical interlace (striped/flicker) | ★★ | low | ✅ sprite | Video Chess; AtariAge |
| 12 | **Bank switching** | F8/F6/… schemes | break the 4K ROM ceiling | ★★★ | low (later) | n/a | `bankswitching.asm`; `bankswitch_sizes.txt` |

## Suggested order (for the North Star)
**#6 animation** (easy, high payoff) → **#2 48-px score** (Frogger needs a score) → **#3 2-line kernel**
(foundation that makes #2/#4 cleaner) → **#4 vertical positioning/VDEL** → **#5 sound** (with you, by ear) →
then the rest as Frogger demands them. #10–12 are "when the game needs it."

> Note: `reference/docs_atari/spiceware_tutorial/` (Darrell Spice Jr., *Let's Make a Game*, Steps 1–14) is
> essentially a ready-made curriculum that touches most of #2–#9 in build order — a strong execution guide.
