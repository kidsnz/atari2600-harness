# Techniques catalog — verified Atari 2600 authoring techniques

A growing, **hardware-verified** library of 2600 techniques: each is studied (from references / real games),
**re-implemented clean-room**, **verified numerically** on the embedded Gopher2600, cross-checked in Stella,
and **locked by a regression scenario run in CI**. The goal is a toolkit the author + the model can compose
with confidence, from basics to advanced tricks.

## Pipeline (every technique follows this)
1. **Learn** — from `reference/` (local, third-party study material) or AtariAge/web. Learn the *idea*.
2. **Implement clean-room** — `roms/techniques/<name>.asm` (our own code; never copy the reference verbatim).
3. **Verify numerically** — load in the harness; `read_tia`/`read_row`/`read_collisions`/… confirm it does
   what it claims (not eyeballing).
4. **Cross-check** — the author confirms in Stella (independent oracle): the visual/audio matches the numbers.
5. **Lock in** — `roms/techniques/scenarios/<name>.json` (numeric asserts + golden frame) → CI runs it on every
   push; write up the technique here.
6. **Promote (optional)** — a stable, reusable kernel/generator graduates to `pkg/` (like `pkg/playfield` / `pkg/sprite`).

## Catalog

| # | Technique | Level | Doc | Demo ROM | Status |
|---|---|---|---|---|---|
| 1 | Sprite multiplexing (vertical zones) — many players past the 2-per-line limit | intermediate | [zone-multiplexing.md](zone-multiplexing.md) | `roms/techniques/zone_multiplex.asm` | ✅ 12 moving sprites, CI-locked |
| 2 | Sprite animation — GRP frame cycling + free REFP flip | easy | [sprite-animation.md](sprite-animation.md) | `roms/techniques/sprite_anim.asm` | ✅ 4-phase walker, pos(v)=v calibrated, CI-locked |
| 3 | Vertical positioning — any-Y placement, per-line compare | intermediate | [vertical-positioning.md](vertical-positioning.md) | `roms/techniques/vertical_pos.asm` | ✅ bouncing ball, art verified bit-for-bit, CI-locked |
| 4 | 2-line kernel — art rows over 2 scanlines = CPU headroom | intermediate | [two-line-kernel.md](two-line-kernel.md) | `roms/techniques/two_line_kernel.asm` | ✅ 2 sprites + gradient, shared HMOVE, CI-locked |
| 8 | Playfield modes — score mode & PF priority | intermediate | [pf-modes.md](pf-modes.md) | `roms/techniques/pf_modes.asm` | ✅ both modes pixel-verified, CI-locked |
| 10 | Flicker multiplexing — N objects through 2 players | advanced | [flicker-multiplexing.md](flicker-multiplexing.md) | `roms/techniques/flicker_multiplex.asm` | ✅ 4 balls @30Hz, alternation CI-asserted |
| 12 | Venetian Blinds — 2 figures through 1 player, zero flicker | intermediate | [venetian-blinds.md](venetian-blinds.md) | `roms/techniques/venetian.asm` | ✅ alternating rows pixel-verified, CI-locked |
| 10b | Dynamic multi-sprite kernel — Y-sort + 2-of-N + mid-screen reposition | advanced | [dynamic-multisprite.md](dynamic-multisprite.md) | `roms/techniques/dyn_multisprite.asm` | ✅ 5 crossing objects, zero budget spills, CI-locked |
| M | Instrument-envelope music driver — per-frame volume envelopes + per-note instrument (TIATracker-derived) | advanced | [music-driver.md](music-driver.md) | `roms/techniques/music_driver.asm` | ✅ envelopes/sustain/pluck-gate/loop CI-locked, audio golden |

What to absorb next (prioritized, with sources): **[roadmap.md](roadmap.md)** — e.g. 48-px score / 2-line
kernel / vertical positioning / sound / animation / playfield tricks / general flicker kernel / bank switching.
