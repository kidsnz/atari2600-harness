# 8bitworkshop sample cross-check

Cross-check of Steven Hugg's **"Making Games for the Atari 2600"** example ROMs (the 8bitworkshop
curriculum) against this harness: do they assemble in our toolchain, run in our emulator, and does our
technique library (`roms/techniques/`, `pkg/*`) cover what the book teaches? Treats the book examples as a
**second, canonical opinion** on each technique.

Sources: `reference/docs_atari/8bitworkshop_samples/*.asm` (32 files; 26 standalone `.asm`, the rest are
`.inc`/`.h` helpers). 8bitworkshop is GPL-3.0 (compatible) and is the project owner's actual dev environment.

## Method & result

- **Assemble:** `dasm <sample>.asm -f3 -I<dasm>/machines/atari2600 -I<samples-dir>`. DASM ships the
  `vcs.h`/`macro.h` the samples need; `xmacro.h`/`xtimer.h`/`*.inc` are local. **26/26 assemble cleanly**
  (25 × 4K, `bankswitching` 8K). → our toolchain handles canonical book code as-is.
- **Run:** loaded into the harness (`load_rom` NTSC → `step_frame`). Representative ROMs run with a stable
  VSYNC frame boundary. **`multisprite3` verified in depth:** 60 frames stable, and `analyze_screen` reads
  back **8 distinct multiplexed player sprites** (per-row colors, `fidelity:1`) — i.e. the canonical
  multi-sprite kernel renders correctly *and* our ingest reverses it. That is the cross-check working end to
  end.

## Technique map (sample → our equivalent)

| sample | teaches | our equivalent | status |
|---|---|---|---|
| hello | minimal kernel / CLEAN_START / bg color | fundamentals | ✓ trivial |
| vsync | VSYNC/VBLANK/overscan frame structure | CLAUDE.md frame constants | ✓ |
| playfield | PF0/1/2 per-scanline bitmap | `pkg/playfield`, `pf_modes.asm` | ✓ match |
| lines | changing PF each scanline | `pf_modes.asm` | ✓ |
| sprite | single player GRP0 | `pkg/sprite`, `sprite_anim.asm` | ✓ match |
| colorsprites | per-scanline GRP0+COLUP0 LUT | `sprite_anim.asm` (row colors) | ✓ match |
| bigsprite | 48-px sprite via async multi-write | `bitmap48.asm`, `pkg/sprite.SplitWide` | ✓ match |
| sethorizpos | coarse ÷15 + fine HMOVE positioning | `litmus_pos`, `design.PositionSplit` | ✓ match |
| controls | div15 positioning + joystick read | `design.PositionSplit`, `paddle_demo.asm` | ✓ match |
| missiles | missile objects | `bullets.asm` | ✓ match |
| collisions | CXxx collision detection | `litmus_collide*`, `read_collisions` | ✓ match |
| multisprite1 | 2-line kernel multi-sprite (basic) | `two_line_kernel.asm` | ✓ match |
| multisprite2 | sorted / more sprites | `dyn_multisprite.asm` | ✓ match |
| multisprite3 | full multiplexed N sprites | `dyn_multisprite.asm` | ✓✓ **verified** (8 sprites, fidelity 1) |
| scoreboard | score display kernel | `score6.asm` | ✓ match |
| score6 | 6-digit BCD score (48px) | `score6.asm` | ✓ match |
| tinyfonts | 48×5 text from char table | `text12.asm`/`text24.asm`, 48px | ✓ match |
| bitmap | asymmetric / bitmap PF | `bitmap48.asm` | ✓ match |
| complexscene | full-screen PF + sprite overlay | `zone_multiplex.asm`, `rpgmap.asm` | ✓ |
| complexscene2 | larger composite scene | `zone_multiplex.asm` | ✓ |
| timing1 | cycle timing / WSYNC | CLAUDE.md timing, `assert_line_budget` | ✓ |
| timing2 | **timer-based timing (TIM64T)** | CLAUDE.md timing | ✓ (ties to **G8** RIOT-timer trap) |
| brickgame | breakout: PF+ball+paddle+collision | integration (`game_states.asm`) | ✓ integration |
| fullgame | complete game skeleton / states | `game_states.asm` | ✓ match |
| bankswitching | F8 4K→8K bankswitch | `banked_game.asm` | ✓ match |
| road | **pseudo-3D road: 2 missiles + ball as the shoulders + dashed centre line** | — | ✗ **gap** |

## Findings

- **Coverage is strong.** 25/26 book techniques have a verified equivalent in our library. The book is a
  good external audit and it confirms our `roms/techniques/` set is complete for the standard curriculum.
- **One gap → candidate:** `road` — a **pseudo-3D road drawn with the two missiles + ball as widening
  lines** (the racing-game primitive; an application of the "missile = line" design rule). We have no
  equivalent. Logged as a technique candidate.
- **`timing2`** uses the RIOT timer (`TIM64T`) for kernel timing — the same hardware path as the
  **G8 timer-wraparound roll trap** mined today (303277). Good test fixture if/when G8 is implemented.

## Reproduce

```sh
DASMINC=$(dirname $(which dasm))/../share/dasm/machines/atari2600   # or brew Cellar machines/atari2600
cd reference/docs_atari/8bitworkshop_samples
dasm multisprite3.asm -f3 -I"$DASMINC" -I. -o /tmp/ms3.bin
# then load_rom /tmp/ms3.bin (NTSC) → step_frame → analyze_screen
```

---

## Dithertron (same author) — read 2026-08-01

[Dithertron](https://github.com/sehugg/dithertron) is Steven Hugg's image converter: an arbitrary picture
in, a picture legal for a chosen retro machine out. **GPL-3.0**, with *exported data* released **CC0**.
It is the inverse of `analyze_image`, which ingests a picture that is already 2600-legal; nothing here
does the hard direction.

**Its three VCS modes** (`src/settings/systems.ts`) are exactly the shapes this repo cares about:

| id | grid | converter |
|---|---|---|
| `vcs` | 40 x 192, 2 colours | `DitheringCanvas`, `reduce: 2` |
| `vcs.color` | 40 x 192 | `VCSColorPlayfield_Canvas` |
| `vcs.48` | 48 x 48 | 48-pixel sprite bitmap (`litmus_48px6` territory) |

**The algorithm, and the transferable part.** `VCSColorPlayfield_Canvas` is `TwoColor_Canvas` with
`w = 40, h = 1` — the constraint cell is *one entire scanline of 40 playfield columns, two colours*, which
is the 2600's COLUPF/COLUBK limit stated exactly. Colour choice and dithering are mutually dependent, so it
alternates to a fixed point: `guessParam` histograms the line, weighting the **currently assigned** colour
100 and the error-diffused nearest match 1, and takes the top two; `update` then rounds each pixel within
*only those two* and diffuses the residual. The 100:1 weighting plus an `errorThreshold` are the damping —
"changing a pixel costs evidence, unless its colour just became illegal" — which is the practical answer to
a formulation that otherwise oscillates.

**Their display kernel** (`src/export/asm/vcs.asm`) is a second canonical opinion on the asymmetric-playfield
kernel, and it differs from ours in ways worth recording:

| | Dithertron | our `horizon` (practice tree) |
|---|---|---|
| per-line colour | none — one pair for the whole frame | COLUBK + COLUPF every line |
| loop cost | 56 cy | 66 cy |
| reaching the right-half window | `nop` x3 | natural instruction spacing |
| table layout | one 1152-byte blob, `equ .+192*n` + `incbin` | each table `align 256` |
| closing the last visible line | `SLEEP 14` before clearing PF | a `sta WSYNC` before the epilogue |

Two of those earned their place immediately. **`SLEEP 14` is the same repair we needed**: written the
obvious way, the region from the kernel's last WSYNC to the first overscan WSYNC runs long — measured at 91
cycles against a budget of 76 — and both kernels pad it. Arriving at it independently is the confirmation
that the fix is idiomatic rather than a patch. And **their contiguous layout genuinely can cross a page**
where ours provably cannot, which is precisely the distinction `pagePenalty` could not draw until the
page-aligned-base shortcut was added; their kernel would be a good specimen for the conservative side of
that pair.

**What is worth absorbing, ranked honestly.**

1. *Nothing, for making pictures.* Dithertron already exists, is free, and its output is CC0. Writing our
   own dithering engine is not this project's goal.
2. *The measurement it implies, which is.* Dithertron computes **the best this hardware can do for this
   image**. Turned around, that is a denominator we do not have: `vismatch` compares a build against
   another ROM, so when a picture is wrong there is no way to separate "the kernel is wrong" from "the
   hardware cannot do this". A theoretical-best reference converts that into a percentage.
   **Design settled 2026-08-03 by prototype and measurement**; see
   `sandbox/experiments/visual-ceiling/` (local-only) and the summary below. Not implemented.
3. *The cross-check above*, which is done.

**Licence note.** Reading the design and reimplementing is fine; vendoring their GPL-3.0 source into this
repo is not, and nothing here does. The comparison kernel in the practice tree is written from scratch.

### The ceiling metric — design settled by measurement (2026-08-03)

An upper bound is **not a property of an image; it is a property of (image, constraint set)**. Measured on
real frames, rmse against the target:

| frame | C1 (PF grid, 2 colours/line) | C3 (no grid, 2 colours/line) | difference = the 4-clock grid's own cost |
|---|---:|---:|---:|
| `horizon` (playfield only, hand-authored) | **0.00** | 0.00 | 0 |
| Chopper Command | 15.07 | 11.82 | 3.25 |
| Seaquest | 13.36 | 9.38 | 3.98 |
| Barnstorming | 15.95 | 7.00 | **8.95** |

Scoring Chopper Command under C1 gives a low number that says nothing about its kernel — Chopper is not
trying to draw with the playfield alone. **A denominator that does not match the constraints the kernel
works under is a lie dressed as a percentage.** Rendering the C1 ceiling makes that legible: the landscape
survives nearly intact while the helicopter, the score digits and the ACTIVISION logo collapse into
4-clock smears. The bound says, in a picture, "the playfield can do the scenery and cannot do the actors".

**Decision: report a LADDER (C1 → C2 → C3), and treat the DIFFERENCES as the deliverable.** C1→C2 answers
"what would one sprite buy here"; C1→C3 answers "how much is the column grid costing" — 8.95 on
Barnstorming, whose content is fine sprite detail, against 3.25 on Chopper, whose content is landscape.
That is an authoring aid rather than a grade. Detecting the constraint set from the build was rejected
because it makes the author's own choices the denominator, so the score is high by construction and never
says "you left a resource unused".

**A trap the self-test caught, worth repeating anywhere this is rebuilt.** The test is: a frame a real ROM
produced is achievable by construction, so it must score perfectly under a set that describes it —
`horizon` is playfield-only, so C1 must return 0. It returned **9.95**, because the frames come from
Gopher2600's renderer while the quantiser used `internal/ingest/palette_stella.go`. **7 of the frame's 14
colours were not in that palette at all**, off by up to 40 RGB units, the same order as the signal.
Harvesting the real palette with `roms/litmus/litmus_palette.bin` took it to **0.00**. A percentage
computed against the wrong palette is noise with a decimal point.
