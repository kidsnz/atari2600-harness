# The reproduction loop — `vismatch` + `behavmatch`

Two tools that make clean-room reproduction of a commercial ROM a **build → measure → diff → fix**
closed loop, so you never again sparse-sample a screen, misjudge a playfield-band boundary by 1–2px,
and iterate by hand. Both compare a **target** ROM (the original) against **your build** (a `.bin` or a
`.asm`, which is auto-assembled).

> Why: reproducing Outlaw's cactus by eye cost many rounds of "still a bit off" because 4–8-line sampling
> read band boundaries wrong, and the wrong measurement became the build's target. Object-attribution
> diffing at full resolution catches every difference in one pass.

## `vismatch` — visual diff (palette-independent)

Renders both frames and reads **which TIA object drew each pixel** (`emu.DecomposeRow` →
BG/PF/P0/P1/M0/M1/BL) on every visible scanline. Comparing object attribution — not RGB — is exact even
though the two ROMs use different colour palettes.

```
# every element difference + per-element band diff (exact scanline range & clock-spans)
go run ./cmd/vismatch -target Outlaw.bin -target-reset -target-frames 28 \
    -mine <game>/build/game.asm -mine-frames 12 -elem PF -diff overlay.png
```

Output: per-element missing/extra cell counts, then **band diffs** like
`PF 162-165  target 80-83 | mine 72-83` (a playfield bar 4px too tall, pinpointed). `-diff` writes an
overlay PNG: green=match, red=target-only, blue=mine-only, amber=object-type mismatch. Exit 1 on any
difference (a reproduction gate).

Key flags: `-target-reset`/`-mine-reset` (press console RESET to start — most originals need it),
`-target-frames`/`-mine-frames`, `-elem PF|P0|…` (filter band diffs), `-scale`.

### `-genpf` — generate the playfield tables FROM the target

```
go run ./cmd/vismatch -target Outlaw.bin -target-reset -genpf -region 122-186 -arena-top 74
```

Measures the target's playfield bands in the scanline `-region` and emits **paste-ready** `CACTOP/CACBOT`
+ `CacLTbl/CacRTbl` `ds` runs (the asymmetric repeat-mode PF encoding: left-half PF2 for clk72-79,
right-half PF0 for clk80-95). `-arena-top` maps scanline→arena line. This turns "derive the cactus by
hand" into one command — validated to reproduce Outlaw's hand-derived tables byte-for-byte.

## `behavmatch` — behavioural diff

Drives both ROMs through **identical scripted input scenarios** and records every object's per-frame
trajectory (`emu.ObjectYExtent`/`Markers`/`PeekRAM`), so a mechanic that differs is a number, not a hunch.

```
go run ./cmd/behavmatch -target Outlaw.bin -mine <game>/build/game.asm -scenario all
```

It separates the **mechanic** (speed + travelled span) from **calibration** (absolute rest position): a
`step≈1.0` (0.5px/frame) walk with a 3px start offset reads as "mechanic ok [pos offset +3]", not a false
divergence. The fire scenario adds a **freeze-coupling** check (shooter frozen while its bullet is out =
the "no-Getaway" rule). Scenarios live in `internal/behavmatch/scenarios.go` (add your own: an input
timeline `At[frame]` + objects/RAM to trace). Exit 1 on any mechanic divergence.

## `framegen` — reproduce a whole frame FROM SCRATCH

Emits a NEW, self-contained DASM source that reproduces a target ROM's static visible frame pixel-exactly,
**including the players** — a full clone, not just the playfield.

```
go run ./cmd/framegen -rom Outlaw.bin -reset -frames 28 -out clone/outlaw_clone.asm
vismatch -target Outlaw.bin -target-reset -mine clone/outlaw_clone.asm   # → pixel-exact
```

It reads which TIA object drew each pixel per visible scanline (`emu.DecomposeRow`), re-encodes the
playfield into left/right PF register bytes and the two players into GRP0/GRP1 bytes, reads
colours/NUSIZ/positions, and writes a data-driven per-scanline `PF(L/R)+GRP0/GRP1` replay kernel. It then
**self-calibrates** by assembling + rendering its own output in a loop: the two `SetXPos` inputs (the
landing offset is kernel-specific), the VBLANK line count (clone's visible top matches the target's), and a
residual content vertical shift (±lines, chosen by element-match). Validated: produced a pixel-exact Outlaw
clone (gunmen at 2× width with P1 reflected, asymmetric cactus, score, bars, borders) with the target's
exact TIA colours. The output is stand-alone (register equates inline, no includes).

## Scope (honest)

These automate **measuring and generating the DATA** (PF tables, positions, band boundaries) and
**verifying** visual+behavioural match. They do not synthesise an arbitrary kernel from scratch: new
sprite artwork still comes from the Photoshop round-trip, and a genuinely new kernel scaffold (a new
per-scanline object arrangement) is still hand-authored — the tools then measure it against the target and
tell you exactly what is off. The recurring 1px / speed / freeze iteration pain is fully removed.

## Building blocks reused (`internal/emu`)

`New`/`LoadROM`/`RunFrames`/`StepFrame`/`SetInput`/`SetPanel`/`Snapshot`/`ReadRow`/`DecomposeRow`
/`ObjectYExtent`/`Markers`/`PeekRAM`; `internal/build.Assemble` (auto-build `.asm`). No emulator changes.
