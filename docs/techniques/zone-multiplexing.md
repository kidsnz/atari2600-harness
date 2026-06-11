# Technique #1 — Zone (vertical) sprite multiplexing

**Goal:** show **more than the hardware's 2 players** on one frame by dividing the screen into vertical
**zones** and **re-using P0/P1 in every zone** (reposition X, reload graphics/color). N zones → up to 2·N
players. This is the standard 2600 trick behind crowded screens (rows of cars, enemies, lily pads).

Source studied: DaveC's `landscape.asm` (AtariAge; in `reference/dave/`). This write-up is **clean-room** —
the idea is universal 2600 knowledge; the implementation here is our own.

## How it works
Per-zone data lives in parallel arrays indexed by zone (`p0_x[]`, `p0_y[]`, `p0_tile[]`, same for P1,
`zone_color[]`, `zone_height[]`). The display kernel walks the zones top→bottom; for each zone it:

1. **Swap colors** for the zone (`COLUBK`/`COLUPF`).
2. **Reposition P0** to `p0_x[zone]` using the proven coarse+fine method the harness already verifies:
   - coarse: a divide-by-15 loop (`sec` then `sbc #15 / bcs`), 5 cycles/iteration = 15 color clocks = the
     coarse step (harness: `litmus_pos`, slope 3px/CPU-cycle).
   - fine: the remainder indexes an HMOVE-nibble table → `HMPx` + strobe `RESPx`; apply `HMOVE` just after a
     `WSYNC` (harness: `litmus_hmove`, all 16 nibbles).
   Repeat for P1.
3. **Draw the zone**: an inner per-scanline loop loads `GRP0`/`GRP1` (+`COLUP0/1`) from the tile's graphics,
   guarded by `cpy #SPRITE_H / bcs` so the sprite only paints within its height (free Y placement in the zone).

## Cycle-level craft (the parts that bite)
- **`align 256`** the kernel: a mid-kernel page crossing on a branch/`lda abs,Y` costs +1 cycle and shears
  the picture. Align so the hot loops don't straddle a page.
- The **HMOVE lookup table** is placed so the indexed read can't cross a page on the positioning line
  (the classic `LOOKUP = TABLE_END - 256`, indexed by the negative remainder).
- Use a **timer (`TIM64T`)** for VBLANK/overscan so timing doesn't depend on how much per-frame logic ran.
- Budget every line to 76 CPU cycles; positioning eats the first 1–2 lines of each zone.

## How the harness verifies it
The building blocks are already hardware-verified (`litmus_pos` = positioning, `litmus_hmove` = HMOVE,
`litmus_sprite` = GRP bit order). The **composite** is verified by a demo ROM under `roms/techniques/`:
`read_tia` / `read_row` confirm a sprite appears at the expected X **in each zone** (i.e. the same P0/P1 land
at different X on different scanlines), and `get_screen_annotated` + Stella show many sprites at once.

## Status — ✅ verified
- `roms/techniques/zone_multiplex.asm` puts **12 sprites on screen** (6 zones × P0+P1) from a 2-sprite
  machine, each repositioned per zone. Hardware-verified on Gopher2600 (`get_screen_annotated` shows all 12;
  `read_tia` reads the last zone's P0=69 / P1=149) and cross-checked in Stella. Locked by
  `roms/techniques/scenarios/zone_multiplex.json` (position asserts + golden frame), run in CI.
- Next: per-frame motion (move each zone's X), per-zone colors/tiles; then promote a reusable kernel
  generator to `pkg/` (like `pkg/playfield` / `pkg/sprite`).
