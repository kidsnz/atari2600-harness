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

## Map and level data elsewhere, and proposals from the list (cited, not built here)
None of this is in `rpgmap.asm`; each item is how a game or demo held its map or level data, or what
someone on the list proposed.

- **Fewer playfields than rooms.** From the thread on *Knight Guy in Low Res World* (vhzc, 2018):
  *"There is 28 screens/rooms in the game, but only 12 playfields, some playfields are simply
  re-utilized without change and the other elements are modified"* 〔AtariAge `topic/283728`; held
  here only as distilled notes, so the wording is not checked〕. In this demo's terms a room would hold
  a playfield index instead of its own 8 bands, so two rooms can share walls (our reading).
  **Cited only, not verified.**
- **A room as a record of indices.** Manuel Rotschkar on *Crazy Balloon* (2005), describing the level
  data to Kirk Israel for a level editor: *"Each level is identified simply by a number. Even the title
  screen is a standard level."* The number points to a 4-byte record — `layoutType`, `enemyType`,
  `spikesType`, `screenscroll` — *"So each level is pointing to a certain layout, enemy and spike
  object. And you can individually activate the scrolling for each level."* A layout holds 4 pointers
  to playfield stripes (each 26 lines high), the balloon's start position and its colour. He called the
  format *"not put in stone yet"* 〔stella-list `200503/msg00016`〕. Kirk Israel, after reading the code (9 March), granted in
  passing that levels share layouts well and asked whether the editor should allow free painting
  instead — *"while you make very good reuse of layout info and the like, I assume the editor should
  allow freeform "painting" of the board"*, a question in the original — and suggested, tentatively
  (*"I guess I'm thinking we should save any "chunk reuse" as a possible post-contest
  optimization"*), leaving reuse in the editor for later 〔`200503/msg00035`〕. **Cited only, not verified.**
- **The map as commands.** Paul Slocum's scrolling RPG demo (2003): the map compression is *"basically
  a map descriptor language that can fill areas with tiles and generate pseudo-random tree areas with
  one or two byte commands"*; his 70x70 tile map is *"only around 300 bytes"* where *"a simple packed
  format"* would take *"about 2.5K"*, and *"will probably be closer to 400 bytes once the gameplay
  information is added"*. Decompressing while scrolling cost time: *"I use two frames to completely
  render the new screen. This causes a little bit a flicker on the edge sometimes"* 〔stella-list
  `200305/msg00037`〕. Fabrizio Zavagli replied that he agreed *"the flicker is ok on the tv screen"*,
  with the caveat *"but of course I'm not free from bias since "Bounce!" flickers in a similar way"*
  〔`200305/msg00042`〕. The byte counts are
  Slocum's own figures. **Cited only, not verified.**
- **Working on packed data, not just unpacking it.** On packing a Game of Life grid into RAM (Andrew
  Davie's run-length idea, 2001), Thomas Jentzsch: serial unpacking *"shoudn't be a big problem"*, but
  *"\*working\* with this packed data, which can't be done just serial, requires a very special
  algorithm. This \*might\* be the real problem."* (the asterisks are his) 〔stella-list `200103/msg00088`〕 65 minutes later he
  offered what he called *"a quite simple solution"*: decode the stream at three positions that *"differ by exactly one row and the mid-row is
  the one that gets recalculated"*, needing *"a minimum of extra RAM for unpacking, but a fast
  algorithm"*; and: *"If you can find one or two dimensional patterns that you can
  store in ROM, which are only referenced by RAM, you might be able to increase compression even
  more."* 〔`200103/msg00089`〕 Erik Mooney answered the run-length idea that compression *"doesn't save
  you RAM in worst-case scenarios, which you have to support"* 〔`200103/msg00096`〕. Davie accepted the
  limit — *"Then my proposed implementation could not handle it. This is a limitation I was aware of,
  and am prepared to put up with"* 〔`200103/msg00097`〕 — and Mooney replied that he did not like Life
  being *"actively corrupted"* by the RAM of its simulator 〔`200103/msg00147`〕. Davie's packing and
  Jentzsch's three streams and ROM patterns stay proposals in the thread, not a build, and the data
  there is a RAM grid rather than a ROM map.
  **Cited only, not verified.**
- **Exits as data.** Piero Cavina (1998), in a thread on *Adventure*, answering a post about where its secret room
  is: someone could
  *"take a look at the source code, find the table with the map of the land, and try to "dig" a passage
  to the secret room changing a byte or two in the binary file"* 〔stella-list `199801/msg00235`〕 — a
  suggestion; nobody in the thread reports doing it. Here the neighbours are computed (`room ^= 1` /
  `room ^= 2`), which ties the world to its grid; a per-room exit table would let a byte or two rewire
  it (our reading). **Cited only, not verified.**
