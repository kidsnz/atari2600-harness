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

What to absorb next (prioritized, with sources): **[roadmap.md](roadmap.md)** — e.g. 48-px score / 2-line
kernel / vertical positioning / sound / animation / playfield tricks / general flicker kernel / bank switching.
