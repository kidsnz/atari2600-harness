# Cookbook — intent → recipe (game-type → technique stack + traps + checks)

The "retrieve" step of the [authoring protocol](authoring-protocol.md): given what you want to build, this
maps it to the verified technique stack (`docs/techniques/`), the design rules (`docs/design-principles.md`),
the traps to avoid (`docs/known-traps.md`), and the `pkg/design` checks to run. So a veteran's "oh, you want
X — here's the standard approach and the gotchas" is reproducible.

## Canonical build order (bottom-up — do it in this sequence)
SpiceWare's *Let's Make a Game! ("Collect")* 14-step order — the proven sequence from a blank kernel to a
finished 2600 game. Build and verify each step before the next.
1. stable display (VSYNC/VBLANK/overscan, 262 NTSC) → 2. timers (TIM64T frame budget) → 3. score
(`score6`) → 4. **2-line kernel** (`two_line_kernel`) → 5. VDEL alignment → 6. playfield (`pf_modes`,
`pkg/playfield`) → 7. switches/input (`paddle_demo`, SWCHA/SWCHB) → 8. game variations (`game_states`) →
9. RNG (LFSR, `litmus_lfsr`) → 10. ball → 11. missiles (`bullets`) → 12. sound (`sfx`, `sound_driver`,
`music_driver`) → 13. animation (`sprite_anim`) → 14. polish.
〔出典: 採掘 reference/atariage/blogs/ — SpiceWare "Collect" tutorial index〕

## Game-type recipes

| want to build | technique stack (`roms/techniques/`) | key rules | watch these traps | pre-build checks |
|---|---|---|---|---|
| **shooter** (player + bullets + enemies) | `bullets` (missiles), `dyn_multisprite`/`flicker_multiplex` (enemy waves), `shared_setxpos`, `score6`, `sfx` | mid-scanline GRP rewrite for enemy rows; missile=line | RESxx +5/+4 draw delay; GRP in HBLANK; flicker >2 needs empty Y lane | `design.NeedsFlicker`, `LineBudget`, `MinColorBandWidthPx` |
| **maze** (Pac-type) | `maze` (procedural PF) or `rpgmap`, `dyn_multisprite`, collision | cell-grid movement (`SWCHA AND cellmask`, ÷16); separate logic-collision map from draw | total-scanlines constant; PF write delay (cy48 reflect) | `design.ScrollScanlinesConstant`, `PFTotalColorClocks` |
| **racer** (driving) | `road` (M0/M1 shoulders + BL centre, widening), `divtable` (÷15), `shared_setxpos` | pseudo-3D = 1-px objects as lines; bands widen toward foreground | HMOVE-24cy rule; total lines constant | `design.PositionSplit`, `LineBudget` |
| **platformer** | `two_line_kernel`, `multicolor48`, `pf_modes` (reflect), `bullets`, subpixel physics | fixed-point 8.8 movement (gravity/friction = carry-driven); reflected-PF lets player drop both edges | RESxx draw delay; player-width 1-clk shift | `design.BackgroundSpec`, `ScanlinesForSquare` |
| **puzzle/board** | `pf_modes` (score-mode 2-color), `text12`/`text24`, `score6`, `game_states` | PF as the board; RAM-pack the state (overlay RORG) | vars not at $FF (stack); missing CLEAN_START | `design.ScoreModeTwoColor`, `MaxChars` |
| **title / logo from a Photoshop mock** | `multicolor48` (per-row color 48px), `bitmap48`, `text12` | **Photoshop mock first → 48px image → flicker-free 2-color 48px kernel** (SpiceWare SF2 logo path); 2:1-ish pixel aspect | pixel-aspect (1.67–1.82, measure in Stella); color non-RGB | `design.PixelAspectRatio`, `MinColorBandWidthPx` |
| **music / sound** | `music_driver`, `sound_driver`, `sfx`, `tia_pcm` | TIA = LFSR-pair voices (not a table); AUDF lowering has ≤32cy latency; out-of-tune scale → pick fitting notes | 2-voice phase interference → silence; Gopher2600 noise differs from real HW | `read_audio`, golden_audio |

## Beyond-bB (advanced, future)
DPC/DPC+/CDFJ/ARM (bigger ROM, writable gfx RAM, 3-voice ARM music) — `reference/atariage/blogs/` (SpiceWare
SF2/Frantic/Draconian) + `docs/design-principles.md` Bitmap section. Out of scope until the vanilla path is solid.

## How to use
Pick the row, clone the listed `roms/techniques/*.asm`, read the linked rules, run the checks, then follow the
[authoring protocol](authoring-protocol.md) loop. The traps column is your `check_traps.py` / manual pre-flight.
