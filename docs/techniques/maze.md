# Technique — Entombed-style procedural playfield maze

**Source:** mined from AtariAge topic 296383 (US Games *Entombed* maze-generation investigation;
`reference/atariage/296383-*/notes.ja.md`). The famous "32-byte table" was a reverse-engineering red
herring — the maze is generated procedurally (technique candidate ⑲).

## Goal
Generate a scrolling maze straight into the playfield, the way US Games' *Entombed* does: take
pseudo-random bytes, distribute their nibbles across PF1/PF2, **expand each random bit to 2px**
(double `rol`) so passages are wide enough to walk through, and **shift a row buffer** to scroll
the maze vertically. The famous "32-byte table" the 2019 archaeology paper agonized over was a
disassembly artifact (a red herring); the real generation is plain procedural code.

## Demo
`roms/techniques/maze.asm` (standalone 4K, DASM). Structure:
- **Fixed-seed LFSR** — 8-bit Galois LFSR (`lsr` / `eor #$8E`, period 255, never-zero), same form
  as `litmus_lfsr`, seeded `$5A`. A fixed seed makes the whole maze deterministic (the golden
  depends on it).
- **2px bit-expansion (`GenRow`)** — pull one random bit with `lsr src`, push it into the output
  twice with `rol dest`; four source bits become eight output bits = four 2px wall/passage cells.
  This is the heart of the Entombed kernel (`rol`-twice per bit). `GenRow` keeps its bit counter in
  a zero-page byte (`gcnt`) so it does **not** clobber the caller's X index.
- **Row buffer + scroll** — `$90..$9F` = 16 PF1 rows, `$A0..$AF` = 16 PF2 rows. Each frame the
  buffer is shifted down one row (Entombed's `$8F,x → $90,x` copy) and a fresh row is generated into
  row 0, so the maze flows downward. A `scroll` counter (`$81`) advances +1 per frame.
- **Symmetric walls** — only the left half is generated; `CTRLPF` D0=1 (reflect) mirrors it to the
  right, giving the symmetric Entombed-style maze for free.
- **Frame** — explicit WSYNC accounting: VSYNC 3 + VBLANK 37 (compute runs here, then padded) +
  visible 192 (16 rows × 12 scanlines) + overscan 30 = **262**.

A sample generated maze (left half shown; reflect mirrors it right), `##` = wall, blank = passage,
every cell exactly 2px wide:

```
########            ############
########    ########    ########
    ########    ####    ########
            ############
####    ############
        ########    ########
```

## CI
`roms/techniques/scenarios/maze.json` (golden-pinned). Run:
`go run ./cmd/scenario roms/techniques/scenarios/maze.json` → exit 0.

- `ntsc_frame_lines == 262`, `golden_frame` pinned (`maze.golden`).
- `ram.0x81 == 13` at frame 10 and `== 33` at frame 30 → **scroll offset advances** exactly +1/frame.
- `ram.0x80 == 195` at frame 10 → **deterministic LFSR state** from the fixed seed.
- PF buffer bytes non-zero across several rows (`$90`, `$97`, `$9F`, `$A0`, `$AF` all `!= 0`; two
  pinned exactly) → **the maze rendered, not a blank screen**, and varies per row.

## Verified facts
- 2px expansion is exact: every generated PF byte has its bits in adjacent pairs (e.g. `207 =
  11001111`, `240 = 11110000`, `60 = 00111100`), so each maze cell is exactly 2 color clocks wide —
  a walkable passage width.
- Same seed ⇒ same maze (scenario pins LFSR state + specific PF bytes; the golden hash pins the
  full rendered frame).
- Frame is a clean 262 NTSC lines with the compute folded into the 37-line VBLANK budget.
- Hardware basis: LFSR taps from `litmus_lfsr` (v0.46.0); playfield bit order (`read_row`-verified,
  v0.6.0) — PF1 MSB-first, PF2 LSB-first, reflect mirrors the left half.

## Notes / scaling
- Bidirectional scrolling (Pitfall's left/right-stepping LFSR) lets the maze grow both ways; step
  the LFSR per cell instead of per row for finer structure.
- **What a counter-generated world cannot do.** Glenn Saunders, on a large world in little RAM: *"It
  can be done without much RAM by using polynomial counters ala Pitfall … With the counter, it has two
  directions, forward and back. One dimensional. The problem with this is if you back up, the monsters
  would have to reset. There is no way you can store the state of every monster in a world this big."*
  〔stella-list `199907/msg00023`〕 The counter regenerates terrain in either direction for free; it
  cannot remember anything that changed there, and stepping it forward and back walks a line, not a
  grid. Decide before relying on it whether the player can return somewhere and expect it changed.
  (He also suggested a random seed at every reset; this demo's fixed seed is the opposite choice.)
- **Choosing the column count: walls and passages add up to 40.** The playfield is 40 pixels wide,
  so `walls × wall width + passages × passage width = 40`. SeaGtGruff, with 1-pixel walls and one more
  wall than passages: all passages 2 wide gives `(n + 1) + 2n = 40` → **13 passages, 14 walls**; the
  most passages gives **20 walls and 19 passages**, all 1 pixel wide except the centre one at 2
  〔AtariAge `topic/224797`, 2014〕. Arithmetic only (14 + 26 = 40, 20 + 19 + 1 = 40).
- **One byte per row, not per scanline.** This demo keeps one byte per row by nesting a 12-line band
  loop inside the row loop (`maze.asm`, `RowLoop` / `Band`). A kernel that has to stay one flat
  per-scanline loop — an asymmetric playfield rewriting PF1/PF2 mid-line, say — can get the same
  saving with a second counter. SplendidNut's insert, placed after the line's playfield writes, with X
  preloaded to the last row:
  ```
  TYA            ; scanline counter
  AND #7
  BNE notNextRow
  DEX            ; X = row index into the PF tables
  notNextRow:
  ```
  X moves once every 8 lines and the tables shrink to one byte per row 〔AtariAge `topic/360395`,
  2024〕. The posted comment calls `AND #7` *"essentially (Y mod 7)"*; it is **mod 8**, and a single
  `AND` only works for a power-of-two row height. By the opcode table it costs 7 cycles when the branch
  is taken and 8 when it falls through (same page) — **Not verified** in a kernel.
- **Scrolling by sweeping an index the kernel already reads.** This demo scrolls by copying the row
  buffer down every frame. SpiceWare's alternative, on Collect: the kernel picks which arena to draw by
  an offset into the playfield tables (`ArenaOffset`), whose only two values are 0 and 22 — *"If,
  instead, you vary the value over time from 0,1,...,21,22 you'll end up with a scrolling playfield"*.
  No copy and no new kernel code, only another value in an index it already uses. He warned it is
  *"chunky scrolling"* 〔AtariAge `topic/267694`, 2017〕. **Cited only, not verified**.
- Carve guaranteed-solvable passages by forcing one open column per row (mask a passage bit before
  storing), the way Entombed guarantees a path.
  ★**Not implemented — and measured 2026-09-07 to be unnecessary at this size.** The generator is an
  8-bit Galois LFSR of period 255 stepped per row from a fixed seed, so the seed picks the whole maze
  and there are exactly **255** of them. All 255 were generated and flood-filled
  (`internal/emu/mazesolvable_test.go`): **every one is traversable from the top edge to the bottom by
  an 8-pixel-wide sprite**, with 22–91 px of eight-wide opening at the top edge and 93 at the bottom.
  ★★**The width is the part that nearly went missing**: a fill on single pixels asks whether a 1-px
  path exists, which no player is, and requiring eight contiguous columns roughly halves the open
  space. The answer survives it. ★★★**Exhaustion is not construction** — nothing in the ROM prevents a
  blocked maze, it simply happens that no seed produces one, and that is a property of this
  generator at this height with this step rate. Change any of the three and it is a question again;
  the carve above is what would make it structural.
- Combine with a player sprite (`dynamic-multisprite`) + collision (`CXPFB`) for wall collision to
  turn this skeleton into a playable maze game.

## Two of the three maze rules are local; the third is not (2026-09-07)

Thomas Jentzsch, who wrote Robot City, on generating by **adding** walls rather than removing them:

> there are at least two ways to build a maze: remove or add walls. In Robot City I'm adding …
> **1. there must be at least one wall at each 'wall connection point'**
> **2. dead ends are not allowed**
> **3. all area must be connected**
>
> #1 is **very easy (and fast)** and after some thinking and try-and-error, I found **a fast way for
> #2** too that still accepts the first rule. But **#3 requires either some heavy restrictions while
> adding walls (which I don't like) or some kind of floodfill**, that checks, if both sides of the
> wall are still connected. That part is **most time consuming now and the duration can be quite
> variable**.
> 〔`200208/msg00283`〕

★**The split is what to keep: rules 1 and 2 can be decided by looking at a cell's neighbours, and rule
3 cannot.** Connectivity is a property of the whole grid, so it needs a flood fill — and a flood fill
costs a variable amount of time, which on a machine with a fixed frame budget is the expensive kind of
cost. It is not that connectivity is hard; it is that it cannot be answered locally, and everything
this machine does cheaply is local.

★★**This repository's generator sidesteps rule 3 entirely and gets away with it.** It carves by LFSR
rather than by adding walls, and all 255 seeds turn out traversable (above) — so the flood fill was
never needed. That is the same trade Jentzsch names as the alternative he did not like: *"heavy
restrictions while adding walls"*, which is what a fixed carve pattern is. ★★★Worth knowing before
adding rules to it: **the moment a generator has to decide connectivity rather than inherit it, its
cost stops being predictable**, and a 2600 kernel cannot pay a variable cost at a fixed deadline.
Found by the mailing-list distillation (helper-1).
