# Verified coverage — what the harness proves on real hardware

The harness's credibility comes from **numeric verification on the embedded Gopher2600 emulator**, not
assertion. Every behavior below is exercised by a litmus ROM under `roms/litmus/`, read back numerically
(`read_tia` / `read_tia_registers` / `read_row` / `read_collisions` / `read_audio` / cycle counters), and
locked for regression by a scenario under `roms/litmus/scenarios/` (run in CI on every push).

Legend: **ROM** = `roms/litmus/<x>.asm` · **Scenario** = `roms/litmus/scenarios/<x>.json` (golden where noted).

## Frame & timing
| Behavior | ROM | Evidence |
|---|---|---|
| NTSC 262-line frame (3/37/192/30) | `smoke` | `ntsc_frame_lines == 262`; RAM sentinel |
| RIOT timer: 1/cycle countdown, post-underflow $FF wrap, TIMINT D7 set, **INTIM read clears it** | `litmus_timer` | INTIM/TIMINT snapshots to RAM |
| Address mirrors: RAM $0180↔$0080 (stack page), TIA $0049→COLUBK | `litmus_mirror` | RAM readback + rendered bg color |
| PAL 312-line frame (3/45/228/36) | `litmus_pal` | `ntsc_frame_lines == 312` with `tv_spec: PAL` |
| Cycle counting invariant (exec×3 == color clocks) | `litmus_cycles` | white-box test; 1 frame = 263×76 = 19988 |
| Per-scanline budget guard (overrun → halt) | `litmus_overrun` | `over=true`, `line_cycles=152`; no false positive on smoke/frogger |

## Horizontal position
| Behavior | ROM | Evidence |
|---|---|---|
| Player coarse position (3 px/CPU-cycle, 15 px/loop, 160 wrap, leftmost X=3) | `litmus_pos` | `ResetPixel = 15·DELAY − 18`, slope 3.0000 (auto via `cmd/calibrate`) |
| HMOVE nibble table (all 16, +7…−8, 1 px) | `litmus_hmove` | `HmovedPixel` matches CLAUDE.md for every nibble |
| Missile / ball position family (`X = 3N − 55`) | `litmus_missile` | `read_tia` missile0=38 / ball=140; `read_row` 1px line at each |
| Missile clamp X=2 vs player clamp X=3 (1px edge offset) | `litmus_collide_mp` | documented during m0-p0 setup |
| RESBL re-emits START (2 balls on a re-strobed line); RESPx does not (1 player) | `litmus_resp_edge` | `read_row`: 2 ball runs vs 1 player run |
| HMOVE comb: left 8px blanked on strobe-after-WSYNC lines even with HMxx=0 | `litmus_hmove_side` | `read_row`: strobe lines clock 0–7 black, others not |
| Late HMOVE: mid-visible (~cyc 39) = no-op; line-end (~cyc 74) = left (HM+8) px, no comb | `litmus_hmove_side` | cumulative P0 drift −9px/strobe measured (emulator-verified; Stella cross-check pending) |

## Sprites (player graphics)
| Behavior | ROM | Evidence |
|---|---|---|
| GRP bit order (D7 = leftmost), row order (top→bottom) | `litmus_sprite` | ramp widens 1→8 px from clock 3 (`pkg/sprite.Encode`) |
| P0+P1 combine = seamless ≤16px / multicolor | `litmus_p0p1` | `read_tia` P0=69/P1=77 (+8); `read_row` continuous 16px (no seam) |
| NUSIZ double-width (16px) | `litmus_nusiz` | `read_row` len 16; `player0.nusiz=5` |
| NUSIZ quad-width (32px) | `litmus_nusiz_quad` | `read_row` len 32; `player0.nusiz=7` |
| NUSIZ three copies (close, 16px spacing) | `litmus_nusiz_copies` | `read_row` 3×8px spans at clock 3/19/35 |
| 48-pixel sprite (NUSIZ-3 × P0+P1 interleaved +8; score6 recipe) | `litmus_48px` | `read_row` 48 contiguous px (clock 24–71); P0=24/P1=32 |
| 48px **dynamic 6-store kernel** (VDEL choreography; distinct byte per copy) | `litmus_48px6` | all 48 bits match design (stores at cyc 34/37/40/43) |
| REFP reflect (mirror) == `pkg/sprite.Reflect` | `litmus_refp` | `reflected=true`; ramp mirrored (right-anchored) |
| VDEL write-triggered shadow copies (GRP0→P1 old, GRP1→P0+ENABL old; VDELxx shows old) | `litmus_vdel` | 6 bands: hidden until the cross-write, then appears (`read_row` per band) |

## Playfield
| Behavior | ROM | Evidence |
|---|---|---|
| PF bit order (PF0 upper nibble / PF1 MSB-first / PF2 LSB-first) | `litmus_pf` | `read_row` per-column lit positions (2 sources agree) |
| Per-scanline background color | `litmus_color` | `read_row` distinct single color per line |
| Asymmetric PF via double-write (windows per woodgrain) + per-pixel split on late writes | `litmus_pf_async` | left $AA / right $55 exact clocks; cyc-33 write → 5 old + 3 new bits |
| CTRLPF: SCORE (left=COLUP0/right=COLUP1), priority D2 (player↔PF), ball width 1/2/4/8 | `litmus_ctrlpf` | per-band `read_row`; SCORE+PFP→COLUPF flagged for Stella |

## Collisions (CXxx)
| Behavior | ROM | Evidence |
|---|---|---|
| Ball–playfield (CXBLPF) | `litmus_collide` | `read_collisions.bl_pf == true` |
| Player0–Player1 (CXPPMM) — the pair Frogger uses | `litmus_collide_pp` | `read_collisions.p0_p1 == true` |
| Missile0–Player0 (CXM0P) | `litmus_collide_mp` | `read_collisions.m0_p0 == true` |
| **All 15 pairs** at once (overlap P0/P1/M0/M1/BL + PF) | `litmus_collide_all` | every `read_collisions` field true |

## Input
| Behavior | ROM | Evidence |
|---|---|---|
| SWCHA joystick bits (P0 left → D6=0), no-input $FF | `litmus_input` | RAM-sampled readback under a scenario input timeline |
| INPT4 fire (D7, 0=pressed; low bits = open-bus noise) | `litmus_input` | $BC released / $3C pressed |
| VBLANK D6 latch: INPT4 stays pressed after release; directions don't latch | `litmus_input` | $3C persists ≥3 frames post-release; SWCHA returns to $FF |
| Paddle INPT0: dump (VBLANK D7) → charge; transfer curve 0.1→5 / 0.25→69 / 0.5→176 lines | `litmus_paddle` | scanline-count readback under a paddle input timeline |

## 6502/6507 precision
| Behavior | ROM | Evidence |
|---|---|---|
| NMOS BCD: A and C correct, Z unreliable ($99+$01 → $00, C=1, Z=0) | `litmus_6502` | self-measured, status pushed to RAM |
| JMP ($xxFF) page bug (high byte from $xx00) | `litmus_6502` | buggy-path marker |
| Reads +1 on page cross; **stores fixed** (STA abs,X always 5) | `litmus_6502` | TIM1T-windowed cycle deltas |
| Branch 2/3/4 (not taken / taken / +page-cross) ; illegal DCP zp = 5 | `litmus_6502` | TIM1T windows |

## Procedural generation
| Behavior | ROM | Evidence |
|---|---|---|
| 8-bit Galois LFSR (`eor #$8E`): exact sequence, never-zero, period 255 | `litmus_lfsr` | `read_ram` first-8 values + sweep flags |

## Bank switching
| Behavior | ROM | Evidence |
|---|---|---|
| F8 (8K): AUTO fingerprint, hotspots $FFF8/9, cross-bank instruction flow, per-bank vectors/stub | `litmus_bank` | per-bank RAM sentinels + lockstep frame counters + `bank.number==0` at frame end |

## Audio
| Behavior | ROM | Evidence |
|---|---|---|
| AUDC/AUDF/AUDV read-back (both channels) | `litmus_audio` | exact match to known writes |
| Audio-chain golden regression (`digest.Audio`) | `litmus_audio` | `checks.golden_audio` deterministic record→match |
| AUDC duplicates: {0,11}{4,5}{12,13} sample-exact; **{6,10}{7,9} inverted twins** | Go test (hand-built ROMs) | digest equality / exact periodicity + complementary duty |
| Pitch = base/(AUDF+1)/D verified by raw-sample capture (square 30/62, lead 90, bass 310) | Go test | `EnableAudioCapture` + period measurement |

## Not yet covered (open)
Playfield priority/score mode (CTRLPF D2/D1), remaining collision pairs, paddles (INPT0–3 charge timing),
SECAM, and a Stella-oracle cross-check of the rendered pixels.
See `docs/hardening-roadmap.md` § v2 backlog.
