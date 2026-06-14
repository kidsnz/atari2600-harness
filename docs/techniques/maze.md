# Technique — Entombed-style procedural playfield maze

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
- Carve guaranteed-solvable passages by forcing one open column per row (mask a passage bit before
  storing), the way Entombed guarantees a path.
- Combine with a player sprite (`dynamic-multisprite`) + collision (`CXPFB`) for wall collision to
  turn this skeleton into a playable maze game.
