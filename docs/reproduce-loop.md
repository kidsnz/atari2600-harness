# The reproduction loop — `vismatch` + `behavmatch` + `ramtrace`

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

## `ramtrace` — what is this ROM's state, and how does each byte change?

The prior question to both of the above. `vismatch` asks whether it looks the same and `behavmatch`'s
trajectory diff asks whether it moves the same; `ramtrace` asks what the machine's 128 bytes of state are
doing, so a game's logic can be re-authored one rule at a time instead of guessed at wholesale.

```
ramtrace record   -rom target.bin -scenario duel-long -out trace.json   # full per-frame series
ramtrace activity -rom target.bin -scenario all                          # what each byte did
ramtrace arity    -rom target.bin -scenario all -skip 20                 # what each byte depends on
```

Every frame it records all 128 RAM bytes, the **held input** (reconstructed from the script's press/release
edges), the **collisions that occurred** and the **stack-pointer range**. Artifacts carry a provenance block
(ROM md5, engine, harness version, scenario), so a measurement can always be re-derived.

It is **ROM-blind by construction** — the input is a `.bin` and an input script, never a symbol map or a
disassembly. That ordering is the seal: an observation made before reading anyone's source is evidence; the
same observation made afterwards only confirms what you already believed. It is also not theoretical. The
scenario library used to declare a target's RAM addresses; measuring the real cartridge showed those
addresses were guesses from our own reconstruction and wrong. Scripts now cannot name an address, so they
cannot name a wrong one.

- **`activity`** describes and concludes nothing: distinct values, change cadence, the set of per-frame
  deltas, which inputs the byte moved under. Fitting a model on top is a separate, later step, and keeping
  the two apart is what stops a fitted story from being mistaken for an observation.
- **`arity`** answers whether a byte's next value is decided by a small number of things — itself, the
  input, and one or two companions. This is the premise that makes per-byte reconstruction a small
  regression rather than an intractable search, so it is measured rather than assumed. It is a determinism
  check, not a fit: if two transitions agree on the features but disagree on the result, the feature set is
  too small. Unresolved is reported as unresolved, with the residual count, the **locations** of the
  contradicting transitions (`scenario:frame`), and the search bound named.

Two traps it is built to survive, both found by measuring rather than by review:

- **A free-running frame counter explains everything.** A byte that takes a fresh value every frame
  identifies the frame, so keying on it reproduces the whole recording — the first version of the probe
  duly reported that all of RAM had arity 1. Such bytes are now identified, tried last, and any resolution
  that leans on one is flagged, as is any resolution where every key was seen exactly once (`MEMORISING`:
  consistent with the data, evidence of nothing about unvisited states).
- **Power-on initialisation is not a per-byte rule.** It is a bulk RAM clear, and folding it into the same
  model makes bytes look mysterious for reasons unrelated to gameplay. `-skip` separates the two; the
  conflict locations are what made this visible in one read.

### Sampling inside the frame, not at its edge

Collisions and the stack pointer are recorded **during** the frame, not at the boundary, because at the
boundary both are already gone: games clear `CXxx` every frame, and SP is back at `$FF`. Boundary sampling
of SP on a real target reported a low-water mark of `$FF` on every frame — a number that excluded exactly
zero bytes and quietly turned the RAM gate's stack mask into a no-op.

Watching inside the frame then killed the mask rule itself, which matters more. The target's SP sweeps
`$FF` down to `$1C` on every frame — a `TXS` aiming the pointer at TIA register space, not a 227-byte
stack. Under the rule "exclude every address at or above the lowest SP", all 128 bytes drop out and the
gate passes unconditionally while reporting a clean green. **A pointer descending past an address is not a
write to it.** Stack exclusion needs write attribution, which does not exist yet, so nothing is excluded on
stack grounds and the SP range is reported as evidence instead.

## Scope (honest)

These automate **measuring and generating the DATA** (PF tables, positions, band boundaries, per-byte
state behaviour) and **verifying** visual+behavioural match. They do not synthesise an arbitrary kernel from scratch: new
sprite artwork still comes from the Photoshop round-trip, and a genuinely new kernel scaffold (a new
per-scanline object arrangement) is still hand-authored — the tools then measure it against the target and
tell you exactly what is off. The recurring 1px / speed / freeze iteration pain is fully removed.

## Building blocks reused (`internal/emu`)

`New`/`LoadROM`/`RunFrames`/`StepFrame`/`SetInput`/`SetPanel`/`Snapshot`/`ReadRow`/`DecomposeRow`
/`ObjectYExtent`/`Markers`/`PeekRAM`/`CurrentRAM`/`StartFrameWatch`+`FrameWatch`; `internal/build.Assemble`
(auto-build `.asm`). The only emulator addition is observation-only: whole-RAM read, and in-frame
accumulation of collision events and the SP range through the existing per-color-clock callback — proven
not to change a single RAM byte or cycle count.
