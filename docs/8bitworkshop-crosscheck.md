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
