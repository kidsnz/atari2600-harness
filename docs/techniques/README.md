# Techniques catalog — verified Atari 2600 authoring techniques

A growing, **hardware-verified** library of 2600 techniques: each is studied (from references / real games),
**re-implemented clean-room**, **verified numerically** on the embedded Gopher2600, cross-checked in Stella,
and **locked by a regression scenario run in CI**. The goal is a toolkit the author + the model can compose
with confidence, from basics to advanced tricks.

## Pipeline (every technique follows this)
1. **Learn** — from `reference/` (local, third-party study material) or AtariAge/web. Learn the *idea*.
2. **Implement clean-room** — `roms/techniques/<name>.asm` (our own code; never copy the reference verbatim).
3. **Verify numerically** — load in the harness; `read_tia`/`read_row`/`read_collisions`/… confirm it does
   what it claims (not eyeballing).
4. **Cross-check** — the author confirms in Stella (independent oracle): the visual/audio matches the numbers.
5. **Lock in** — `roms/techniques/scenarios/<name>.json` (numeric asserts + golden frame) → CI runs it on every
   push; write up the technique here.
6. **Promote (optional)** — a stable, reusable kernel/generator graduates to `pkg/` (like `pkg/playfield` / `pkg/sprite`).

## Promoting out of a work (roms/) into here

Written 2026-08-21, after doing it once and getting three of the five wrong on the way.

- **A measured fact goes up the day it is measured.** It cannot improve by waiting, only be
  forgotten — that session re-derived, from throwaway probes, a rule `internal/emu/hmovemid_test.go`
  already held. **Negative results count**: nobody writes down what did not work, which is exactly
  why the next session repeats the experiment.
- **A mechanism waits for its SECOND caller.** Promote a generator when something other than its
  first use needs it, not on first success — a tool generalised from one use case encodes that use
  case's accidents. `internal/keyfit` (502 lines) sat here with no CLI and no importer until
  `check_wiring`'s fifth check was written to catch exactly that.
- **This repository is PUBLIC and `roms/` is not.** The copy that lands here must not name the work:
  cite `roms/`, not `roms/<work>/`. The work's own notes name the artifacts that went up, so the
  trail exists for anyone holding both repositories and for nobody else.
- **Nothing lands without its litmus, its grading test and its catalogue row.** The gates enforce
  this — `check_provenance` refuses an uncited claim or a path that does not resolve,
  `check_wiring` refuses an unreachable doc, `check_tests` refuses a test that can only fail when
  its setup does.
- **Verify the COMMITTED tree, not the working one.** A detached `git worktree` of the commit,
  built and graded there. Measuring the working tree grades other sessions' unfinished work: the
  pre-push mirror blocked a push over two files that were not in the commit, and a run of the gates
  in the checkout said green while `go test` had never actually built.

## Catalog

| # | Technique | Level | Doc | Demo ROM | Status |
|---|---|---|---|---|---|
| 1 | Sprite multiplexing (vertical zones) — many players past the 2-per-line limit | intermediate | [zone-multiplexing.md](zone-multiplexing.md) | `roms/techniques/zone_multiplex.asm` | ✅ 12 moving sprites, CI-locked |
| 2 | Sprite animation — GRP frame cycling + free REFP flip | easy | [sprite-animation.md](sprite-animation.md) | `roms/techniques/sprite_anim.asm` | ✅ 4-phase walker, pos(v)=v calibrated, CI-locked |
| 3 | Vertical positioning — any-Y placement, per-line compare | intermediate | [vertical-positioning.md](vertical-positioning.md) | `roms/techniques/vertical_pos.asm` | ✅ bouncing ball, art verified bit-for-bit, CI-locked |
| 4 | 2-line kernel — art rows over 2 scanlines = CPU headroom | intermediate | [two-line-kernel.md](two-line-kernel.md) | `roms/techniques/two_line_kernel.asm` | ✅ 2 sprites + gradient, shared HMOVE, CI-locked |
| 8 | Playfield modes — score mode & PF priority | intermediate | [pf-modes.md](pf-modes.md) | `roms/techniques/pf_modes.asm` | ✅ both modes pixel-verified, CI-locked |
| 10 | Flicker multiplexing — N objects through 2 players | advanced | [flicker-multiplexing.md](flicker-multiplexing.md) | `roms/techniques/flicker_multiplex.asm` | ✅ 4 balls @30Hz, alternation CI-asserted |
| 12 | Venetian Blinds — 2 figures through 1 player, zero flicker | intermediate | [venetian-blinds.md](venetian-blinds.md) | `roms/techniques/venetian.asm` | ✅ alternating rows pixel-verified, CI-locked |
| 10b | Dynamic multi-sprite kernel — Y-sort + 2-of-N + mid-screen reposition | advanced | [dynamic-multisprite.md](dynamic-multisprite.md) | `roms/techniques/dyn_multisprite.asm` | ✅ 5 crossing objects, zero budget spills, CI-locked |
| M | Instrument-envelope music driver — per-frame volume envelopes + per-note instrument (TIATracker-derived) | advanced | [music-driver.md](music-driver.md) | `roms/techniques/music_driver.asm` | ✅ envelopes/sustain/pluck-gate/loop CI-locked, audio golden |
| 13 | Text on screen — 12 characters flicker-free, 24 with column flicker, and **the width ladder above them**: 12 (flicker-free) → 24 (column flicker) → 26 (supercat) → 28 (Jentzsch) → 32 (solidcorp 2011, interleaved RESP re-strobing) → 36 / 576 / 1008 (omegamatrix). **Read this row before starting a text kernel**: every rung above 24 is a re-strobe route, and the re-strobe itself is measured in #36 — a kernel designed without it will conclude that 12 or 24 is the ceiling | intermediate | [text12.md](text12.md) · [text24.md](text24.md) | `roms/techniques/text12.asm`, `roms/techniques/text24.asm` | ✅ both CI-locked (`scenarios/text12.json` / `text24.json`). The ladder ABOVE 24 is what the AtariAge 32-character-display thread records and is **not measured here**; #36 is the measured part |
| 15 | Constant divide (÷3/7/10/15) — corrected reciprocal-multiply, exact 0..255 + remainder | intermediate | [divtable.md](divtable.md) | `roms/techniques/divtable.asm` | ✅ Go-model exhaustive (0 errors), 13 RAM asserts, CI-locked |
| 16 | Multicolor 48-px — 3×NUSIZ+VDEL 48px with per-row COLUPx (~73/76 cy) | advanced | [multicolor48.md](multicolor48.md) | `roms/techniques/multicolor48.asm` | ✅ per-row color verified, CI-locked, golden |
| 18 | RTS-stack dispatch — data-driven zone chaining via push(addr-1)+RTS, ~6cy/zone | advanced | [rts-dispatch.md](rts-dispatch.md) | `roms/techniques/rts_dispatch.asm` | ✅ 4 zones, RAM-list driven, 262 held, CI-locked |
| 19 | Procedural maze PF — LFSR bits doubled to 2px cells, scrolled, reflected | intermediate | [maze.md](maze.md) | `roms/techniques/maze.asm` | ✅ deterministic seed, symmetric maze, CI-locked |
| 21 | TIA PCM — digitized sample via AUDV (AUDC=0), 1-bit ADPCM, pseudo-5bit DAC | advanced | [tia-pcm.md](tia-pcm.md) | `roms/techniques/tia_pcm.asm` | ✅ AUDV envelope + audio golden, CI-locked |
| 22 | Shared SetXPos — position all 5 objects via one indexed RESPx,x/HMPx,x loop | intermediate | [shared-setxpos.md](shared-setxpos.md) | `roms/techniques/shared_setxpos.asm` | ✅ 5 objects at distinct X (hmoved_pixel), CI-locked |
| 23 | Pseudo-3D road — M0/M1 shoulders + BL dashed centre, widening per band | advanced | [road.md](road.md) | `roms/techniques/road.asm` | ✅ widening wedge verified, steerable, CI-locked |
| 27 | HMOVE two-step — reach screen edges past the ÷15 budget wall (clamp input + 2nd HMOVE drift) | intermediate | [hmove-two-step.md](hmove-two-step.md) | (in-game: sandbox PONG `pf2_02_flyoff-right`) | 🔶 in-game verified (read_row edge pixels + budget); standalone demo/CI TODO |
| 28 | Asymmetric-PF two-digit score — mid-line PF1/PF2 rewrite = 4 independent digit fields | advanced | [asymmetric-pf-score.md](asymmetric-pf-score.md) | (in-game: sandbox PONG `pf2_score-2digit-playfield`+) | 🔶 in-game verified (all digits read_row); standalone demo/CI TODO |
| 29 | Sub-pixel velocity (DDA accumulator) — fractional speed while the POSITION stays a 1-byte integer | intermediate | [subpixel-velocity.md](subpixel-velocity.md) | (in-game: sandbox PONG `pf2_06_feel-rally-ai-serve`) | 🔶 in-game verified (per-tier px/frame measured exact); standalone demo/CI TODO |
| 30 | Audio-envelope idioms (Combat) — free envelopes from gameplay counters: counter-IS-register, self-clearing SFX counter, per-player detune, gear-shift pitch curve | idiom | [audio-envelope-idioms.md](audio-envelope-idioms.md) | (reference — `Combat.asm`) | 📖 studied from Combat disassembly (harvest 2026-07-23); reimplement + CI = TODO |
| 31 | Kernel micro-idioms (Combat) — HMP low-nibble 2nd axis, `$FF`/`$00` AND-mask blank, −4 pointer bias, PF mirror via counter-EOR, compare-via-EOR A=0 | idiom | [kernel-micro-idioms.md](kernel-micro-idioms.md) | (reference — `Combat.asm`) | 📖 studied from Combat disassembly (harvest 2026-07-23); reimplement + CI = TODO |
| 32 | Per-scanline NUSIZ+HMOVE shaping — one player into an irregular silhouette wider than 8px (G9a) | advanced | [nusiz-shaping.md](nusiz-shaping.md) | `roms/litmus/litmus_nusiz_shape.asm` | ✅ intended outline matched on 40/40 scanlines + 120/120 control rows, two-axis decomposition, CI-locked |
| 33 | Fractional-HMOVE slope — arbitrary-angle 1px line on a missile/ball via an error accumulator (G9b) | advanced | [hmove-slope.md](hmove-slope.md) | `roms/litmus/litmus_hmove_slope.asm` | ✅ max error 0 px vs the line equation over 160 scanlines × 2 slopes, static control 160/160, CI-locked |
| 34 | Pitch dither — play a note the TIA has NO register for by alternating two adjacent AUDF values every TWO frames, so the mean PERIOD lands on the target (E1: -26.9 c -> +8.8 c). Swapping every FRAME is worse than not doing it. | idiom | [pitch-dither.md](pitch-dither.md) | `litmus_pitchdither` | ✅ measured on the machine (5 tests incl. the failing case + a roughness figure); not yet used in a piece |
| 35 | Sprite placement physics — where RESP/RESM land an object (player x = 3c-60, missile x = 3c-61, clamped at x=3), the 9 px floor between two strobes, the missile-3-cycles-after-player trick that abuts a 4 px tail to an 8 px head with NO HMOVE, when a GRP write is too late (effective at x = 3w-64), and the fact that a strobe cancels only an object's FIRST copy | foundation | [sprite-placement.md](sprite-placement.md) | `roms/litmus/litmus_sprite_place.asm` | ✅ 11 bands, negative-controlled, CI-locked |
| 36 | RESPx RE-STROBING — how many shaped slots fit on ONE scanline, and **how many depends on the SPACING between strobes**. At 6, 7 or 8 cycles a player in a copy mode draws 3 + k with k mid-line strobes, so one player reaches **eight** and two players reach **sixteen** (measured on its own band, not doubled). At **3 and 5 cycles the ladder is FLAT** — every strobe after the first buys nothing — and at 12 it climbs faster than 3 + k. Where a copy lands moves with the spacing too: off the multiple-of-three grid at 8, **on** it at 6. The mechanism behind the spacing dependence is measured but NOT explained. Slots are not the binding limit though — BYTES are, at six cycles a write | advanced | [restrobe-copies.md](restrobe-copies.md) | `roms/litmus/litmus_restrobe.asm` | ✅ 8 spacings x k=1..5 plus a two-player band, 36 bands, CI-locked; source solidcorp AtariAge 180632 |

## Not a technique, but looked for here: game STATE

Nothing in this directory is about **state machines**, and someone designing a game will look here
first. The material exists, in `docs/casebook.md`, where it is written as what a commercial title
does rather than as something to reach for:

- `casebook.md:94` — *"reconstruct the wall normal over multiple frames with a **trial-and-error
  state machine** … **Lesson: when the hardware UNDER-REPORTS, replace the one-shot formula with an
  observe-then-choose state machine that SPENDS FRAMES to disambiguate.**"* The 2600 under-reports
  constantly — collisions say *that*, never *where* — so this is the general shape, not one game's
  trick.
- `casebook.md:96` — *"the hit reaction is **its own mode that suppresses input**"*: a mode is how
  a machine with one button says "not now".
- `casebook.md:19`, `casebook.md:29` — two more.

A pointer rather than a page, deliberately: writing a third copy is how the same fact ends up stated
three ways and corrected in one. **`sprite-placement.md` names this failure in its own opening** —
*"Knowing a table exists is not the same as reaching for it … a rule nobody reads is a rule that gets
measured again"* — and it earned that sentence by having a measured table re-derived from scratch an
afternoon later. Found by the mailing-list distillation (helper-2), who checked before claiming it
was missing and found it was not.

What to absorb next (prioritized, with sources): **[roadmap.md](roadmap.md)** — e.g. 48-px score / 2-line
kernel / vertical positioning / sound / animation / playfield tricks / general flicker kernel / bank switching.
