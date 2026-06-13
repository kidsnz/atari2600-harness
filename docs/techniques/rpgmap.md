# Technique — room-based map navigation (RPG/adventure)

**Goal:** the navigation backbone of adventure/RPG games: a world of rooms where each room is a
data table, the player walks around, and crossing a screen edge transitions to the adjacent room.

Demo: `roms/techniques/rpgmap.asm` (2×2 world of 4 rooms, joystick player, edge transitions).
CI: `scenarios/rpgmap.json` (walk right → room 0→1, walk down → 1→3, reflect, 262, golden).
Lineage: distilled from za2600 (Zelda port) `kworld.asm`/`rs/`/`spr/`
(`reference/2600-technique-sources/za2600/`, recovered from the legacy ATARI AR folder).

## The technique (the distilled core)
- **Each room = a wall table** (here 8 PF1 bands per room; za2600 uses PF1+PF2 per room plus
  enemy/item tables). `room` index = `roomY*2 + roomX`; the table pointer is `Room0 + room*8`.
- **Player** = P0 placed by PosObject (calibrated divide-by-15), moved by SWCHA (4 directions).
- **Edge transition**: when the player walks past an edge, flip the corresponding room-axis bit
  (`room ^= 1` for left/right, `room ^= 2` for up/down) and wrap the player to the opposite edge.
  Adding rooms is **pure data** — the engine never changes (the za2600 `rs/` philosophy).
- The kernel draws the current room's walls (reflect mode) band by band, lighting the player
  sprite in its band.

## Verified
Start room 0 / player X=76; walk right → room 1; then down → room 3 (the `room ^= 1` / `room ^= 2`
transitions); reflect on; 262 lines; golden-pinned (each room's PF1 walls differ, so the golden
hash captures the correct room rendering).

## Notes / scaling toward za2600
- Real adventure maps add: per-room **enemy/item tables** (za2600 `en/`, `spr/`), room **scripts**
  (`rs/` — locked doors, mazes, events via room flags), and asymmetric PF (PF1+PF2, mid-line
  rewrite) for richer walls. All bolt onto this table-driven skeleton.
- Doors/blocked exits = gate the edge transition on a room-flag byte before flipping `room`.
- Combine with `text24` for dialogue and `bitmap48` for a map/inventory screen.
