# Changelog

The change history of this project. Format follows [Keep a Changelog](https://keepachangelog.com/);
versions follow [Semantic Versioning](https://semver.org/).

> Entries from v0.17.0 and earlier are condensed; the full detailed history (in Japanese) is kept locally
> in `CHANGELOG.ja.md`.

## [Unreleased]

### Planned
- Real game authoring (production use of the harness; e.g. a Pong rematch).
- Extending the `step_scanline|clock` / `watch|trap` tools.

## [0.50.0] - 2026-06-11

### Added
- **RESBL vs RESPx mid-line re-strobe litmus (v2 V2-11).** `litmus_resp_edge.asm` confirms Towers'
  TIA_HW_Notes: strobing **RESBL twice on one scanline draws two balls** (clocks 38 and 140 — the ball
  re-emits START, the multi-ball trick), while strobing **RESP0 twice draws a single 8px player** at the
  last position only (clock 107 — the player does not re-emit START until the 160-clock wrap). Locked by
  `scenarios/resp_edge.json` (position asserts + golden). 28 scenarios pass.

## [0.49.0] - 2026-06-11

### Added
- **Address-mirror litmus (v2 V2-12).** `litmus_mirror.asm` proves the memory map's mirroring: writing $5A to
  $0180 reads back at $0080 (and the reverse) — i.e. RAM $80–$FF is mirrored at $0180–$01FF, **which is why
  the stack works**; and setting the background through the TIA mirror $0049 colours COLUBK ($84 blue in
  `read_row`). Locked by `scenarios/mirror.json`. 27 scenarios pass.

## [0.48.0] - 2026-06-11

### Added
- **All 15 collision pairs verified in one ROM (v2 V2-8).** `litmus_collide_all.asm` overlaps P0/P1/M0/M1/BL
  (missiles width-8, ball width-8) with a lit PF0 at the left edge so every CXxx pair fires at once;
  `scenarios/collide_all.json` asserts all 15 (`p0_p1, m0_m1, m0_p0, m0_p1, m1_p0, m1_p1, p0_pf, p0_bl,
  p1_pf, p1_bl, m0_pf, m0_bl, m1_pf, m1_bl, bl_pf`) true — superseding the three single-pair litmus in
  coverage. 26 scenarios pass.

## [0.47.0] - 2026-06-11

### Added
- **RIOT timer litmus — answers the audit's open INTIM question (v2 V2-10).** `litmus_timer.asm` records
  INTIM/TIMINT snapshots to RAM: TIM1T counts down 1/cycle (consecutive reads −7 = the read-loop cost);
  after underflow INTIM wraps into the $FF range and keeps decrementing 1/cycle; **TIMINT D7 (timer-expired)
  is set before INTIM is read ($C0), and reading INTIM clears TIMINT ($00 afterward)** — the audit's open
  "does reading INTIM clear D7?" is now answered **yes**. Locked by `scenarios/timer.json`. 25 scenarios pass.

## [0.46.0] - 2026-06-11

### Added
- **LFSR litmus — procedural-generation foundation (v2 V2-9).** `litmus_lfsr.asm` runs an 8-bit Galois LFSR
  (`lsr / bcc / eor #$8E`, the form in DaveC's Random-Dungeon and common game RNGs) and proves its math
  numerically (pure `read_ram`, no rendering): the first 8 values from seed $01 are
  `01,8E,47,AD,D8,6C,36,1B` (matches hand calculation), it **never decays to $00** across a full sweep, and
  its **period is exactly 255** (returns to the seed). Locked by `scenarios/lfsr.json`. 24 scenarios pass.

## [0.45.0] - 2026-06-11

### Added
- **CTRLPF litmus — SCORE / priority / ball width, incl. the audit's open SCORE×PFP question (v2 V2-7).**
  `litmus_ctrlpf.asm` verifies five regimes: SCORE ($02) paints the left half COLUP0 / right half COLUP1
  (split at clock 80); default priority ($00) draws P0 over the playfield; PFP ($04) draws the playfield over
  P0 (player hidden); **SCORE+PFP ($06) renders the playfield as COLUPF — the SCORE colour substitution is
  *suppressed* under PFP — with the player hidden** (this corner is unspecified in the docs and a likely
  emulator-divergence point; recorded as a Gopher2600 measurement, flagged for the Stella oracle cross-check
  V2-17); ball width D4–5 doubles 1/2/4/8 px. Locked by `scenarios/ctrlpf.json`. 23 scenarios pass.

### Fixed
- **`smoke.asm` now clears collisions after init (CXCLR) — removes platform-dependent CI flakiness.** The
  zero-page clear loop incidentally strobes the TIA strobe registers (RESxx, HMOVE) whose effect depends on
  the power-on TIA state and reset beam timing, leaving sticky collision latches that differed across
  platforms (CI caught `TestReadCollisionsNoSprites` reporting M1-PF / BL-PF on the runner while it passed
  locally). A single CXCLR after init forces a clean, deterministic baseline; rendering (hence all goldens)
  is unchanged.

## [0.44.1] - 2026-06-11

### Changed (docs)
- **README reframed to match the evolved goal.** The project is no longer just "a loop to build games" with
  the five gaps A–E closed (phase 1); it is now a **general, verified 2600 capability base** (phase 2) — a
  fundamentals audit + a techniques catalog, each kept honest by the same numeric loop. Updated the opening
  and the gap-analysis section to name these two living documents and the current scope (20 tools, 20+
  regression scenarios), and to state the aim as *general verified competence, not any one game*.

## [0.44.0] - 2026-06-11

### Added
- **6502/6507 precision litmus — Tier 1 of the v2 backlog complete (V2-6).** `litmus_6502.asm` measures
  instruction facts *on the machine itself* via RIOT TIM1T (1 cycle/tick) and pins them in
  `scenarios/cpu6502.json`, all matching 6502.org exactly: **NMOS BCD** $99+$01 → A=$00 with C=1 correct
  while **Z=0 lies** (the documented NMOS unreliability, recorded); **JMP ($xxFF)** takes the page-bug path;
  **LDA abs,X** 4→5 cycles on page cross while **STA abs,X stays 5 fixed** (why store timing in kernels is
  deterministic); **BNE** 2/3/4 (not taken / taken / taken+cross); illegal **DCP zp = 5 cycles** (also
  certifies illegal-opcode support). 22 scenarios pass.

## [0.43.0] - 2026-06-11

### Added
- **F8 bankswitching verified + `read_bank` MCP tool + `bank.*` scenario fields (v2 backlog V2-5).**
  `litmus_bank.asm` is a best-practices 8K F8 ROM (vectors + an identical reset stub in *both* banks, a
  same-address switch zone whose instruction stream stays valid across the hotspot): every frame bank 0
  marks RAM and hotspot-reads $FFF9 → bank 1 writes its own sentinel and returns via $FFF8. Verified:
  Gopher2600 AUTO fingerprints the plain 8K dasm binary as F8; $80 ends every frame as bank 1's sentinel;
  both per-bank frame counters advance in lockstep; the kernel executes in bank 0 at the frame boundary.
  New `read_bank` MCP tool (20 tools now; `Cartridge.GetBank` at PC, with `is_ram`) and `bank.number` /
  `bank.is_ram` scenario fields; `bin/harness` rebuilt and smoke-tested (initialize + tools/list, no panic).
  Locked by `scenarios/bank.json`. 21 scenarios pass.

## [0.42.0] - 2026-06-11

### Added
- **Input-port litmus with an input-timeline scenario (v2 backlog V2-4).** `litmus_input.asm` samples
  SWCHA/INPT4 to RAM every frame; `scenarios/input.json` drives a press/release timeline and asserts the
  numeric readback: no input = SWCHA $FF, INPT4 $BC (D7=1 + open-bus noise — the documented reason to test
  with N only); P0 left = $BF (D6→0); fire = INPT4 $3C (D7→0); **the VBLANK D6 latch holds INPT4 at $3C
  frames after fire is released** while directions release immediately (the control). 20 scenarios pass.
  Paddle charge-timing verification split off as **V2-4b** (needs a paddle path in `set_input`).

## [0.41.0] - 2026-06-11

### Added
- **Asymmetric-playfield write-window litmus (v2 backlog V2-3).** `litmus_pf_async.asm` verifies woodgrain's
  `Playfield_Timing` tables to the pixel: **(A)** early PF1=$AA (cyc 5) + PF1=$55 at cyc 40 renders a true
  asymmetric playfield — left bits at clocks 16–43, right bits at 100–127, exactly as predicted;
  **(B)** a late write completing at cycle 33 while left PF1 is being drawn splits **per pixel**: the first
  5 bits show the old $FF (clocks 16–35 lit) and the last 3 the new $00 — reproducing woodgrain's worked
  example verbatim. Locked by `scenarios/pf_async.json`. 19 scenarios pass.

## [0.40.0] - 2026-06-11

### Added
- **HMOVE side-effects litmus (v2 backlog V2-2).** `litmus_hmove_side.asm` measures three regimes in one
  frame: **(a)** HMOVE right after WSYNC blanks the left 8px **even with all HMxx=0** (the comb — alternating
  strobe/no-strobe lines compared by `read_row`), confirming Towers' HBLANK+8CLK extension; **(b)** HMOVE
  mid-visible (~cycle 39) produces **zero displacement and no comb** for both HM=0 and HM=$10;
  **(c)** HMOVE at line end (~cycle 74) with HMP0=$10 (+1) moves P0 **left 9px per strobe = value+8**
  (the classic late-HMOVE +8 rule, measured numerically) with no comb. (b)/(c) are recorded as
  Gopher2600-measured values pending the Stella oracle cross-check (V2-17). Locked by
  `scenarios/hmove_side.json` (cumulative-position asserts + golden). 18 scenarios pass.

## [0.39.0] - 2026-06-11

### Added
- **VDEL litmus — verifies vertical delay's write-triggered shadow copies (v2 backlog V2-1).**
  `litmus_vdel.asm` proves all three paths in one frame, exactly as Stella PG §6.D describes:
  with VDELP0=1 a fresh GRP0=$FF stays hidden until **a GRP1 write copies P0's new→old** (then P0 renders
  $FF at X=3); with VDELBL=1 ENABL=on stays hidden until a GRP1 write (ball appears at X=2); with VDELP1=1
  GRP1=$3C stays hidden until **a GRP0 write copies P1's new→old** ($3C renders as 4px at clock 41).
  Locked by `scenarios/vdel.json` (vertical_delay asserts + golden). 17 scenarios pass. This is the
  prerequisite for the 48px score kernel and 2-line-kernel vertical positioning.

## [0.38.0] - 2026-06-11

### Added (docs)
- **Fundamentals audit — `docs/fundamentals-audit.md`.** Six parallel research passes over the local corpus
  (Stella Programmer's Guide, woodgrain wiki, Davie's *Newbies*, SpiceWare's Collect, 8bitworkshop,
  21 real-game disassemblies, DaveC's Random-Dungeon), ~22 owner-supplied links (AtariAge threads, 6502.org,
  Slocum's music guide, Stella debugger docs, Pitfall analyses), and independent web research (Towers'
  *TIA Hardware Notes*, Stolberg). Every domain is classified **verified / documented / unknown / caution**
  with sources. Headline corrections: the local cycle-counting guide's position math is approximate (never
  cite); Pitfall disassembly's LeftRandom comment is wrong (bit0, proven by simulation); SpiceWare Step 3
  vs 7 PF-window discrepancy (to settle by measurement); HMOVE comb/late-HMOVE behavior absent from the
  local shelf (Towers adopted as authority). Headline finds: VDEL's write-triggered cross-copy semantics;
  woodgrain's definitive asymmetric-PF write-window tables; Slocum's complete AUDC/tuning data (the parked
  audio-authoring blocker was already on our shelf); F8-first bankswitching consensus + Gopher2600 already
  auto-fingerprints 8K as F8 and exposes `GetBank()` (a `read_bank` tool candidate); Stella debugger is
  scriptable for automated oracle cross-checks (F-4 design v1).
- **`hardening-roadmap.md` § v2 backlog** — 18 prioritized follow-ups (V2-1…V2-18) in three tiers
  (VDEL, HMOVE side effects, asymmetric-PF windows, input, bankswitch + `read_bank`, 6502 precision; then
  matrix completion; then capabilities: audio sample capture, `pkg/audio`, Stella oracle automation).
- **CLAUDE.md constants hardened**: 24-cycle HMxx freeze after HMOVE; stores never pay page-cross
  penalties; NMOS decimal mode C-only; CLD mandatory at init; cycle-counting-guide caution. Routing tables
  link the audit.

## [0.37.1] - 2026-06-11

### Changed (docs)
- **Roadmap reframed as a general-capability TODO (de-anchored from any single game).** The main goal is a
  general, verified, reusable technique toolkit — not one specific game. `docs/techniques/roadmap.md` now
  prioritizes by **general/foundational value × difficulty × prereqs-verified** (instead of "relevance to a
  particular game"), and is an explicit checklist (`- [ ]`) ordered foundational/easy-wins first
  (animation → vertical positioning/VDEL → 2-line kernel → 48-px score → sound → …). A concrete game can be
  picked flexibly as a per-technique testbed; it is no longer the organizing principle.

## [0.37.0] - 2026-06-11

### Added / Changed (docs)
- **Technique #1 promoted to its formal name + a sourced techniques roadmap.** Researched AtariAge / the
  local `reference/docs_atari/` corpus and Wikipedia: confirmed the formal name is **sprite multiplexing**
  (the loop is a **multi-sprite kernel**); DaveC's "zone" is the common vertical-band term, and our demo is
  the *static-zones* form of the general *sort/position/display + flicker* kernel. Rewrote
  `docs/techniques/zone-multiplexing.md` with a formal-name/taxonomy section, a "Refinements & limits"
  section (2-per-line limit, flicker, single- vs 2-line kernel, positioning cost), See-also (48-px sprite,
  Venetian Blinds), and a sourced References list — marking *documented* vs *verified*. Added
  `docs/techniques/roadmap.md`: a prioritized survey of ~12 next techniques (48-px score, 2-line kernel,
  vertical positioning/VDEL, sound, animation, playfield tricks, LFSR, general flicker kernel, Venetian
  Blinds, bank switching) ranked by North-Star (Frogger) value, difficulty, and prereq-verified status.
  Catalog index links the roadmap. Docs-only; no code change (tests/scenarios unchanged).

## [0.36.1] - 2026-06-11

### Fixed
- **Deterministic emulator power-on state — eliminates CI test flakiness at the root.** Gopher2600
  randomizes the CPU/RAM power-on state (`vcs.Env.Random`, used by `CPU.Reset`), so a fresh `emu.New`
  varied run-to-run; cycle/timing tests (`TestCycleCounterExcludesWsyncStall`, `TestStepScanline`) passed
  locally but flaked in CI. `emu.New` now calls Gopher2600's official `vcs.Env.Normalise()`
  (`Random.ZeroSeed = true` + prefs defaults), the method intended for regression testing, before the
  cartridge-attach reset. Result: identical initial state every run (verified 5×/10× stable). Goldens are
  unaffected (the ROMs clear RAM on boot).

## [0.36.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 gets per-zone background colors — a landscape look.** Each zone sets `COLUBK` from a
  `ZoneBG` table (sky-blue → cyan → green → brown), set in HBLANK so it doesn't disturb the per-zone
  positioning, giving 6 colored bands behind the 12 moving sprites. Golden regenerated; 262 lines preserved.

## [0.35.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 now animates — 12 *moving* sprites.** Each zone's X moved from ROM tables into RAM
  (`zx0`/`zx1`) and is updated every frame (P0 drifts right, P1 left, wrapping `and #$7F`), so all 12 sprites
  animate. Demonstrates RAM-backed motion verifiable purely by `read_ram` (the position bytes change frame to
  frame). VBLANK line count retuned to keep the frame at 262 (the per-frame update loop is absorbed). The
  scenario now locks the frame by `golden_frame` only (robust to the moving positions); all scenarios pass.

## [0.34.0] - 2026-06-11

### Added
- **Techniques catalog + #1 Zone (vertical) sprite multiplexing.** Establishes a repeatable pipeline for
  absorbing 2600 authoring techniques: learn (from `reference/`, local) → clean-room implement
  (`roms/techniques/`) → verify numerically (harness) → cross-check (Stella) → lock in (scenario + golden +
  CI) → optionally promote to `pkg/`. First entry: `roms/techniques/zone_multiplex.asm` puts **12 sprites**
  on screen (6 zones × P0+P1) from a 2-player machine by repositioning P0/P1 per zone (divide-by-15 + HMOVE,
  the harness-verified method). Verified on Gopher2600 + cross-checked in Stella; locked by
  `scenarios/zone_multiplex.json`. CI now runs `roms/techniques/scenarios/` too. Catalog index at
  `docs/techniques/`, linked from the routing tables.

## [0.33.0] - 2026-06-10

### Added
- **Coverage batch: NUSIZ quad-width + missile-player collision.**
  - `litmus_nusiz_quad.asm` (`NUSIZ0=$07`, QuadWidth) → `read_row` shows a **32px** continuous span (8px ×4),
    completing the NUSIZ width modes (double/quad) and copy modes (close/three).
  - `litmus_collide_mp.asm` overlaps an 8px-wide missile0 with player0 → `read_collisions` reports
    **`m0_p0=true`** (CXM0P), extending collision coverage to the missile-player pair. (Also documents the
    1px left-edge offset between missile clamp X=2 and player clamp X=3.)
  - Locked by `scenarios/nusiz_quad.json` and `scenarios/collide_mp.json`. 15 litmus scenarios pass.

## [0.32.0] - 2026-06-10

### Added
- **P0-P1 collision litmus (CXPPMM) — extends collision coverage.** `roms/litmus/litmus_collide_pp.asm`
  overlaps player0 and player1 (both clamped to X=3 via HBLANK strobes) drawing `$FF`; `read_collisions`
  reports **`p0_p1=true`**. Verifies the player-player pair the Frogger `OnPad` check actually uses (previously
  only BL-PF was litmus-verified). Locked by `scenarios/collide_pp.json`. 13 litmus pass.

## [0.31.0] - 2026-06-10

### Added
- **REFP (reflected sprite) litmus — rounds out the sprite track.** `roms/litmus/litmus_refp.asm` draws the
  asymmetric ramp with `REFP0=$08`; `read_tia_registers` shows `player0.reflected=true` and `read_row` shows
  the ramp mirrored (row0 `0x80` lights clock 10 = the right end; row4 `0xF8` lights clock 6–10 = right 5px) —
  the mirror image of the non-reflected `litmus_sprite`. Confirms REFP and `pkg/sprite.Reflect` (data-side
  mirror) are equivalent. Locked by `scenarios/refp.json`. 12 litmus pass.

## [0.30.0] - 2026-06-10

### Added
- **Missile/ball position litmus.** `roms/litmus/litmus_missile.asm` enables and positions missile0 and the
  ball in the visible region; `read_tia` reads **missile0=38 / ball=140** and `read_row` shows a 1px vertical
  line at each clock — verifying the harness reads the missile/ball object-position family (the `X = 3N − 55`
  side, complementing the player `X = 3N − 54` litmus_pos). Locked by `scenarios/missile.json`. 11 litmus pass.

## [0.29.1] - 2026-06-10

### Fixed
- **Flaky `TestStepScanline` (surfaced by CI).** The test asserted every single scanline step consumes
  >0 CPU cycles, but a scanline can legitimately be a pure WSYNC-stall pass-through (0 instructions executed)
  depending on beam-phase alignment — not an invariant. Relaxed to assert the **cumulative** cycles across
  40 scanlines is >0 (the CPU makes progress), which is robust. Keeps the CI badge reliable.

## [0.29.0] - 2026-06-10

### Added
- **NUSIZ multi-copy litmus coverage (extends S-2).** `roms/litmus/litmus_nusiz_copies.asm` renders an 8px
  solid sprite at `NUSIZ0=$03` (ThreeCopiesClose); `read_row` confirms **three 8px white spans at clock
  3/19/35 (16px copy spacing)**. Locked by `scenarios/nusiz_copies.json` (golden + `player0.nusiz=3`).
  Deepens verified coverage of the NUSIZ helper beyond double-width. 10 litmus scenarios now pass.

## [0.28.0] - 2026-06-10

### Added
- **CI via GitHub Actions (hardening-roadmap F-1).** `.github/workflows/ci.yml` runs on every push/PR:
  Ubuntu + Go (from `go.mod`) + DASM, clones Gopher2600 at the pinned commit `5d532e88` into `./Gopher2600`
  (the `replace` target), assembles the litmus ROMs (`.bin` are gitignored), then `CGO_ENABLED=0`
  build/vet/test and runs all litmus regression scenarios. No SDL needed — the harness only imports the
  SDL-free Gopher2600 packages, so a static (cgo-off) build covers it. A CI badge is on the README.
  Verified green on Actions (build/vet/test + 9 scenarios, ~1m).

## [0.27.0] - 2026-06-10

### Added
- **PAL frame verification (hardening-roadmap F-3).** `roms/litmus/litmus_pal.asm` emits a proper PAL frame
  (VSYNC 3 / VBLANK 45 / visible 228 / Overscan 36 = 312 lines) and `scenarios/pal.json` (with
  `tv_spec: "PAL"`) asserts the harness drives/counts it as **312 lines** (plus a RAM sentinel). Confirms the
  harness is not NTSC-only; `ntsc_frame_lines` counts the actual per-frame line total (312 for PAL).

## [0.26.0] - 2026-06-10

### Added
- **Golden-audio regression `checks.golden_audio` (hardening-roadmap A-2).** Mirrors the video golden for
  sound: a sha1 audio-chain (Gopher2600 `digest.Audio`) over the timeline is compared against
  `<scenario>.audio.golden`. `internal/emu` gains `EnableAudioDigest`/`ResetAudioDigest`/`AudioHash`
  (symmetric to the video digest); `internal/scenario`'s golden eval is generalized to share video/audio.
  Verified with `roms/litmus/scenarios/audio.json` on `litmus_audio.asm` (deterministic record→match, plus
  numeric AUDC/AUDF/AUDV asserts). All 8 litmus scenarios pass. CLI only; MCP binary unchanged.

## [0.25.0] - 2026-06-10

### Added
- **`pkg/sprite` NUSIZ helper (hardening-roadmap S-2).** `PlayerSize` (OneCopy … DoubleWidth … QuadWidth) /
  `MissileSize` enums and `NUSIZ(player, missile)` / `NUSIZPlayer(player)` compose a NUSIZx byte from intent
  instead of raw bits. **Verified on Gopher2600** with `roms/litmus/litmus_nusiz.asm`: an 8px solid sprite at
  `NUSIZ0=$05` (DoubleWidth) renders **16px wide** (`read_row` clock 4–19 = white len 16) and
  `read_tia_registers` shows `player0.nusiz=5`. Locked by `scenarios/nusiz.json`. Completes the sprite
  authoring trio (S-1 encoder + S-2 NUSIZ + S-3 P0+P1 combine).

## [0.24.0] - 2026-06-10

### Added
- **`pkg/sprite.SplitWide` + P0+P1 16px combine litmus (hardening-roadmap S-3 — flagship).** Split a
  16-wide ASCII design into P0 (left 8) + P1 (right 8) GRP tables, then place P1 exactly +8px to the right
  of P0 for a seamless up-to-16px (or multicolor) character. `roms/litmus/litmus_p0p1.asm` positions the two
  sprites by strobing RESP0→RESP1 three cycles apart in the visible region (= +9px; an HBLANK strobe would
  clamp both to the left edge) then HMOVE P1 left 1 → exactly +8px.
  **Verified on Gopher2600:** `read_tia` shows player0=69 / player1=77 (exactly +8); `read_row` shows the
  solid-16 rows as a **single continuous 16px white run (clock 69–84, no seam gap/overlap)**, with P0-only /
  P1-only / far-edge rows byte-exact. Locked for regression by `scenarios/p0p1.json` (position asserts 69/77
  + golden frame). This proves sprite placement is as numerically trustworthy as playfield — the headline
  capability of the sprite track.

## [0.23.0] - 2026-06-10

### Added
- **`pkg/sprite` — ASCII → player GRP encoder (hardening-roadmap S-1).** A mirror of `pkg/playfield` for
  player graphics: 8-wide ASCII rows → GRP bytes (`EncodeRow`/`Encode`, D7 = leftmost = standard TIA bit
  order), plus `Reflect` for REFP-less mirroring / P0+P1 right halves. Reuses `playfield.ParseASCIIRow`.
  Unit-tested, including that `..XXXX..` = `0x3C` matches the existing hand-coded Monet Frogger lily-pad byte.
- **`roms/litmus/litmus_sprite.asm` + `scenarios/sprite.json` (+golden) — numeric hardware proof.** An
  asymmetric ramp sprite (top `0x80` 1px → bottom `0xFF` 8px) rendered by player0 at X=3. Verified on
  Gopher2600 via `read_row`: the white span widens 1→2→…→8 px from clock 3 (visible lines 96–103), proving
  D7 = leftmost and top→bottom row order are byte-exact. Locked for regression with a golden-frame scenario;
  all litmus scenarios PASS. First step of the sprite track toward the P0+P1 16px flagship (S-3).

### Added
- **Strengthening roadmap (`docs/hardening-roadmap.md`).** A prioritized roadmap for the next phase —
  making the harness stronger beyond gap-closing. Theme A: deepen authoring + verification into the thin
  domains (S = sprites, incl. `pkg/sprite` ASCII→GRP, NUSIZ helper, and the ★ P0+P1 two-sprite combine for
  up to 16px / multicolor characters placed numerically via the X(N) calibration; A = audio, incl. note/
  timbre names in `read_audio` via Gopher2600 `tracker`, a `digest.Audio` golden, and a `pkg/audio` SFX
  helper). Theme B: harden the foundation (★ CI via GitHub Actions, optional Gopher2600 version pin,
  PAL/SECAM verification, Stella oracle cross-check, completing `step_clock`/`watch|trap`/`run_scenario`).
  Theme C: wire upstream Gopher2600 libraries (`recorder`/`regression`/`reflection`). Each item lists where
  to touch + how to verify + size. Cross-linked from the routing tables in CLAUDE.md / README /
  improvement-roadmap. No code changes (implementation in separate sessions).

## [0.22.1] - 2026-06-10

### Added
- **GPL-3.0 `LICENSE`.** The harness embeds Gopher2600 (GPL-3.0) as a library, so the combined work is
  GPL-3.0-or-later. Added copyright and an Acknowledgements section to the README.

### Changed
- **Public-readiness: the published repo is now English-only.** Translated the public surface
  (README + `docs/`×7 + CHANGELOG + CLAUDE.md) to English. The author works in Japanese, so Japanese copies
  are kept locally as `*.ja.md` sidecars (gitignored, never published). Calibrated the prior-art wording to
  "no Atari 2600 MCP found in a public search (2026-06; Atari Lynx = gearlynx exists)" rather than claiming
  "first". Removed the README provenance section. No code changes; build/vet/test green.

## [0.22.0] - 2026-06-10

### Changed
- **Physical spinoff: split the base into a standalone repo `atari2600-harness` (game ROMs move to a
  separate repo `atari2600-roms`).** Under an umbrella folder `260609_atari2600-dev/`, place `harness/`
  (this repo, history preserved) and `roms/` (new repo) as siblings, bound by `go.work`. Remove
  `roms/frogger` from the harness (moved to the roms repo); `roms/litmus` stays as the harness's own
  verification ROMs. **Eradicate the harness→game dependency:** repoint the scenario/emu unit tests from
  frogger ROMs to litmus, and add a new fixture `roms/litmus/scenarios/golden.json` (+`.golden`).
  `.mcp.json`/`.claude` move up to the umbrella (read at Claude Code's project root). Updated CLAUDE.md's
  structure/dev sections to the post-spinoff reality. Verified: harness `go vet`/`go test` green, 4 litmus
  scenarios PASS; on the roms side `gen` + 3 frogger scenarios PASS.
- **Renamed the Go module `github.com/kidsnz/atari2600-dev` → `github.com/kidsnz/atari2600-harness`
  (spinoff prep).** `go.mod` and 9 import files replaced. build/vet/test green, all scenarios PASS.
- **Promoted `internal/playfield` → `pkg/playfield` (spinoff prep).** Go can't import `internal/` across
  modules, so the playfield encoder (universal Atari 2600 knowledge) became a public package. Updated the
  only cross-package importer (`roms/frogger/gen`). Regenerated all scenes (header-comment-only diffs).
  Verified green; all scenarios (3 frogger + 3 litmus) PASS.
- **Documentation freshness audit (spinoff preamble).** Rewrote `README.md` to v0.21.0 reality (old diagram
  = `cmd/probe` + `internal/emu` only → 4 cmds, 6 internals, roms/<game>, 19 MCP tools, gaps A–E all
  closed; fixed the smoke.asm path to `roms/litmus/`). Fixed minor staleness in `improvement-roadmap`,
  `mcp-tools`, `tool-landscape`, and a stale `cmd/genpf` comment in `roms/frogger/gen/asmgen.go`.

### Added
- **Improvement roadmap document (`docs/improvement-roadmap.md`).** Prioritizes next moves to make authoring
  more accurate, from every angle. Central observation = the position litmus is closed but the timing
  *budget* verification is open (gap B is the biggest hole in the real loop). P0 = cycle exposure +
  per-scanline budget guard, P1 = TIA shadow / collision register reads, P2 = verification automation,
  P3 = build-loop shortening, each annotated with verified Gopher2600 API symbols
  (`CPU.LastResult.Cycles`, `TIA.Video.*`, `Collisions`). Also added "untapped reference veins" (R-1 Freeway
  architecture port, R-2 audio recipes, R-3 cycle-cost table, R-4 real-game structure index) and "external
  research" (the biggest finding: Gopher2600 already implements the hardest items as libraries —
  `recorder`/`regression`/`tracker`/`reflection`/`digest`/`rewind` are usable standalone, shrinking P2/R-2
  from "build" to "wire"; License = GPL-3.0; an Atari 2600 MCP was not found in a public search = no known
  prior art, not a claim of being first; G-2 C64 MCPs, G-3 test DSLs, G-4 authoring-tool integration).

## [0.21.0] - 2026-06-10

### Added / Changed
- **A `.asm` source can be specified directly as a scenario `rom` (gap E fully closed).** If a scenario's
  `rom` is `.asm`, it is assembled with dasm before running = "one source → assemble → run → numeric
  asserts → verdict" in one command (`go run ./cmd/scenario foo.json`). Gap E reaches its ideal form.
- **Consolidated dasm invocation into `internal/build` (DRY).** `assemble_and_load` (harness) and the
  scenario `.asm` feature share `build.Assemble`/`build.BinPathFor`. Assemble failures are returned as
  errors (dasm output including the failing line), not swallowed. Sample: `roms/litmus/scenarios/smoke_src.json`.

## [0.20.0] - 2026-06-10

### Added
- **Automatic calibration of horizontal X(N) (B-4 / gap B fully closed).** Turns litmus from a one-off
  manual job into a reproducible sweep→auto-fit. A cooperating ROM (`litmus_pos`: delay `DELAY=$80`,
  SBC/BCS = 5 CPU cycles/unit) is poked across delays, `player0.ResetPixel` is measured each frame, and a
  linear regression recovers slope and offset numerically. Implementation: `internal/calibrate` (`Sweep`,
  `Fit` — robust to the 160 wrap and left-edge saturation via median-delta unwrapping of the longest
  consistent run). Result on litmus_pos: **slope = 3.0000 px/CPU-cycle** (matches the authoritative 3),
  R²=1.0, kernel offset = −18. Verified in `calibrate_test.go`.

## [0.19.0] - 2026-06-10

### Added
- **Golden-frame regression (P2 D-3 / gap D fully closed).** Adding `checks.golden_frame: true` compares the
  timeline's **rendered frame-chain hash** against `<scenario>.golden` = pixel-level regression detection of
  rendering (complements the D-1/D-2 logic/timing regression). Implementation: wire Gopher2600's exported
  `digest.Video` into `internal/emu` (`EnableVideoDigest`/`ResetVideoDigest`/`VideoHash`); `internal/scenario`
  enables it for golden scenarios, resets after warmup (deterministic), and compares to `.golden`.
  `cmd/scenario -update` records/updates the baseline. Sample: `roms/frogger/scenarios/golden.json` +
  committed `golden.golden`. CLI only; `bin/harness` (MCP) unchanged.

## [0.18.0] - 2026-06-10

### Added
- **Scenario runner (P2 / gap D = first step of verification automation. D-1 assertions + D-2 input replay).**
  Declares an "input timeline + numeric assertions" in one JSON and auto-passes/fails it against a ROM.
  `go run ./cmd/scenario <file.json> ...` (exit 0 on all pass, 1 on failure) = a regression base that runs
  in CI **without MCP**. Key design: the assertion vocabulary (`field` strings) maps one-to-one to
  `internal/emu`'s read methods (dogfooding the observation tools as the regression vocabulary). Unknown
  fields are an error (no swallowing typos). Whole-run measurements with side effects are separated into
  `checks{ntsc_frame_lines, max_line_budget}`. Structure: `internal/scenario` (parse + vocab + Run,
  ROM-agnostic) / `cmd/scenario` (thin CLI). Samples under `roms/litmus/scenarios/` and
  `roms/frogger/scenarios/` (including `hop` = `up` input drives FrogY 144→128). CLI only; MCP unchanged.

## [0.17.0] - 2026-06-10
### Added
- **`read_audio` MCP tool (R-2 / audio verification path).** Returns the current TIA audio registers
  AUDC/AUDF/AUDV for both channels as numbers (extends rule 1 "verify with numbers" to audio). Uses
  Gopher2600's exported `Audio.PeekChannels()`. Verification ROM `roms/litmus/litmus_audio.asm`; exact match
  in `emu_audio_test.go`.

## [0.16.0] - 2026-06-10
### Added
- **`assemble_and_load` MCP tool (P3 / build-loop shortening).** Takes an asm path, runs `dasm -f3` via
  `os/exec`, and loads the output `.bin` on success — collapsing `edit→dasm→load_rom`. On failure returns a
  structured `ok=false` + `dasm_output` (failing line) instead of an MCP error, so the model can fix in place.

## [0.15.0] - 2026-06-10
### Added
- **`step_instruction` / `step_scanline` MCP tools (B-2 / intra-frame granularity).** `step_instruction`
  runs exactly one CPU instruction (returns its cycles + coords); `step_scanline` runs until scanline +1
  (returns cycles consumed). A color-clock-granular `step_clock` is unimplemented (`Step` is per-instruction).

## [0.14.0] - 2026-06-10
### Added
- **`read_tia_registers` MCP tool (P1 / closes the rest of gap A).** Returns current values of write-only TIA
  registers directly from Gopher2600 internals (measure instead of inferring color from `read_row`). Confirmed
  PF0=$F0 (upper-nibble-only) behavior.
- **`read_collisions` MCP tool (P1).** Structures the 8 collision latches (CXxx, $30–$37) into named boolean
  pairs. Bit assignment verified against Gopher2600's `collisions.go`; BL-PF positive on `litmus_collide.asm`.

## [0.13.0] - 2026-06-10
### Added
- **`assert_line_budget` MCP tool (the crux of gap B / B-3 = per-scanline cycle budget guard).** Numerically
  catches the failure that silently killed Pong v2 (per-scanline overrun → screen roll). Detection: a WSYNC
  strobe = a `RdyFlg` true→false transition; the scanline delta between strobes = physical lines consumed by
  that logical line. Implemented with exported `RdyFlg` + beam coords in `internal/emu`'s own step loop
  (no debugger driver). Verified with `roms/litmus/litmus_overrun.asm` (`over=true`, `line_cycles=152`); no
  false positives on smoke / frogger.

## [0.12.1] - 2026-06-10
### Fixed
- **`read_cycles` double-counted spinning during a WSYNC stall (v0.12.0 bug).** During a WSYNC stall the CPU
  doesn't execute but leaves `LastResult` in place, so the old per-boundary accumulation over-counted on any
  WSYNC-using ROM. Fix: unify progress through a `stepInstr()` primitive and accumulate only when `RdyFlg`
  was true before the Step (i.e. a real instruction ran). Regression test `TestCycleCounterExcludesWsyncStall`.

## [0.12.0] - 2026-06-10
### Added
- **`read_cycles` MCP tool (gap B = wiring timing into the real loop, P0 step 1 / B-1).** Gets CPU cycles
  from the simulator numerically (first embodies rule 2 outside litmus). Returns `last_instruction_cycles`,
  `cycles_since_mark`, `total_cycles`. Source = `CPU.LastResult.Cycles` accumulated at instruction
  boundaries across all progress paths. Verified via the invariant "executed cycles × 3 == color clocks" on
  WSYNC-free `litmus_cycles.asm` (1 frame = 263×76 = `total_cycles 19988`).

## [0.11.0] - 2026-06-10
### Changed
- **Monorepo reorg: root = harness base / `roms/<game>/` = ROMs (spinoff Phase 1).** Demonstrated the
  game→harness one-way dependency and separated without surgery. Moved game-specific kernel generation
  (`cmd/genpf` + asmgen) into `roms/frogger/gen/` (package main importing `playfield`); litmus under
  `roms/litmus/`. All builds/tests green; `litmus_pf` read_row identical after the reorg.

## [0.10.1] - 2026-06-10
### Added
- **Frogger polish.** Game over / restart (Lives→0 resets Lives=3/Score=0); visual zones (top = goal band,
  bottom = start bank, middle = Monet water).

## [0.10.0] - 2026-06-10
### Added
- **🎉 Playable Monet Frogger (M5).** A frog crosses a river on flowing lily pads over Monet water. A full
  game kernel (`GenerateFroggerASM`) handles ride/drown/win/lives via a state machine; collisions via CXPPMM.
  The model **played it itself** (set_input + peek/read_tia) to numerically verify every mechanic — and found
  and fixed a fatal landing-frame timing bug that way (1-frame grace via `PrevY`).

## [0.9.3] - 2026-06-09
### Added
- **Frog vertical hop.** player0 drawn at variable scanline `FrogY`; edge-detected up/down jumps it ±16 (one
  lane) on press (no auto-repeat). The model operates/observes/judges in a closed headless loop.

## [0.9.2] - 2026-06-09
### Added
- **Collision check (the Frogger core: on a pad vs in the water).** Per-frame `CXCLR` strobe; CXPPMM read via
  `peek $37` (no new tool needed). Set/clear verified frame-by-frame.

## [0.9.1] - 2026-06-09
### Added
- **Full-scene integration.** Flowing lily (player0) + controllable frog (player1) coexist over Monet water
  (per-scanline COLUBK), with separate motion applied to both via one HMOVE.

## [0.9.0] - 2026-06-09
### Added
- **`set_input` tool = joystick injection.** `poke` doesn't work for input (RIOT redrives SWCHA each frame),
  so inject via Gopher2600's `Ports.HandleInputEvent`. Control ROM verifies "input → frog moves" headlessly.
### Fixed
- A `set_input` jsonschema tag starting with `0=…`/`true=…` made go-sdk panic in AddTool; reworded the tags.

## [0.8.1] - 2026-06-09
### Added
- **Monet water + flowing lily sprite integration (M3 step 2).** Per-scanline COLUBK (water) + per-scanline
  GRP0 (lily) both resolved in HBLANK to dodge cycle criticality; drift via per-frame HMOVE.

## [0.8.0] - 2026-06-09
### Added
- **M2/M3 animation groundwork.** Per-frame color-table animation (`GenerateAsymmetricShimmerASM`, with
  TIM64T-timed VBLANK/Overscan) and smooth sprite horizontal motion = water flow (`sprite_flow.asm`, +1px/frame
  via per-frame HMOVE) — establishing that smooth horizontal motion on the 2600 is the sprite's (HMOVE) job.

## [0.7.2] - 2026-06-09
### Changed
- **Promoted the Monet still (M1) to an asymmetric version** (left/right-independent playfield + per-row water
  color). Per-row water (COLUBK) + constant lily (COLUPF), since the asymmetric loop has budget for only one
  per-row color channel.

## [0.7.1] - 2026-06-09
### Added
- **Asymmetric (left/right-independent) playfield capability, hardware-verified.** Transcribed ABB's "repeated"
  asymmetric kernel (72 cy/line, `tay`/`sty` timing). read_row proves one-sided lighting (impossible with reflect).

## [0.7.0] - 2026-06-09
### Added
- **M1 "quiet pond" — the rendering pipeline opens end to end.** First milestone of the north-star ROM. Path:
  ASCII art + color → EncodeSymmetric → asmgen(kernel) → dasm → load_rom → read_row check. `GenerateSymmetricASM`
  generates a self-contained reflect-playfield still with per-row COLUBK water.

## [0.6.0] - 2026-06-09
### Added
- **`read_row` tool (read playfield-lit columns / per-scanline color numerically).** RLE `{clock,len,hex}` of a
  visible scanline. Playfield bit-order litmus (`litmus_pf.asm`) and per-scanline color litmus (`litmus_color.asm`)
  pass; the verified bit-order table is burned into `docs/resources.md` / `CLAUDE.md`. The `internal/playfield`
  package (`EncodeSymmetric`/`EncodeAsymmetric`) self-verifies against the real litmus values in go test.

## [0.5.1] - 2026-06-09
### Added
- **`get_screen_annotated` also saves the PNG to a file** (env `ATARI2600_SCREEN_PATH`) so clients that don't
  render inline images (CLI terminals) can still open the latest frame; VS Code auto-reloads on change. Returns
  `png_path` in the structured Out.

## [0.5.0] - 2026-06-09
### Added
- **`get_screen_annotated` implemented (the user↔model comms channel), as a first-class citizen.** Captures the
  frame to `image.RGBA` (PixelRenderer), draws a TIA-coordinate XY grid + axis labels + sprite markers (Fixed
  Debug Colors) at ×3 nearest-neighbor, and returns **image (ImageContent PNG) + numbers together**. Enables the
  "user points on the image → model translates to registers" round trip.

## [0.4.1] - 2026-06-09
### Changed
- **Distilled core constants into CLAUDE.md (Phase 4):** the beam-coord convention `Clock` = HBLANK −68..−1 /
  visible 0..159, horizontal position (3px/cycle, coarse 15px, 160 wrap, leftmost X=3; offset is kernel-specific,
  final verdict via `read_tia.HmovedPixel`), the fully hardware-verified HMOVE table, and the annotated screenshot
  redefined as the primary user↔model channel.

## [0.4.0] - 2026-06-09
### Added
- **Litmus test fully passed (Phase 3) — the harness proven real, numerically (rule #4).** Coarse
  (`litmus_pos.asm`): 1 loop = 5 CPU cycles = 15px, linear over DELAY 3–11 (`ResetPixel = 15·DELAY − 18`),
  160 wrap, leftmost X=3. Fine (`litmus_hmove.asm`): all 16 HMP0 nibbles match the CLAUDE.md HMOVE table at 1px
  granularity. Coarse 15px + fine 1px = any X numerically predictable/placeable/verifiable. Detoxes Pong's
  failures #1/#3 (gap B).

## [0.3.0] - 2026-06-09
### Added
- **Harness plumbing verified (Phase 2.1)** — Gopher2600 embedded as a library, driven fully headless and
  numerically on a real ROM. `internal/emu` driver wrapper; `cmd/probe` numeric CLI; `roms/smoke.asm` confirms
  262 lines / RAM `$80`=$42 / PC.
- **Minimal MCP prototype (Phase 2.2)** — `cmd/harness` exposes 8 tools over stdio; JSON-RPC confirmed numerically.
  Official `modelcontextprotocol/go-sdk` v1.6.1, typed Out auto-generates JSON Schema. Spec in `docs/mcp-tools.md`.
### Decisions
- **Drive via direct `hardware.VCS` embedding, not terminal/PushedFunction** — `hardware`/`television`/`setup` are
  pure Go (no SDL/cgo), so library embedding is more deterministic/simple/fast. The terminal driving the research
  docs assumed was unnecessary.
- **★ Beam clock convention settled on hardware:** `GetCoords().Clock` = HBLANK −68..−1 / visible 0..159 (the
  spec's tentative "0–227" was wrong); same coordinate system as `HmovedPixel`.

## [0.2.0] - 2026-06-09
### Added
- **macOS / Apple Silicon environment set up.** Go 1.26.4, cc65/sim65, pkgconf, Gopher2600 built
  (`go build -tags=release .`), DASM / Stella / SDL2.

## [0.1.0] - 2026-06-09
### Added
- **Project founded.** Defined the goal as "an environment where the model can author the Atari 2600 in 6502
  assembly accurately." Initial `docs/gap-analysis.md` (gaps A–E from the past-Pong post-mortem),
  `docs/tool-landscape.md`, `docs/resources.md` (horizontal formula `X = 3N − 55`, HMOVE table, frame budget,
  collision registers), README, CHANGELOG.
### Decisions
- **Engine = Gopher2600** (the only high-accuracy 2600 emulator drivable at CPU + color-clock granularity on
  macOS), wrapped in a thin Go MCP. **BizHawk not adopted** (no macOS). Regression layer = sim65 / 6502profiler;
  oracle = Stella; top-priority gap = B (timing). MCP SDK = official `modelcontextprotocol/go-sdk`; design follows
  mcp-gameboy. Image overlay in-house Go (no ImageMagick shell-out). Regression around Gopher2600's record/replay
  + `regress`.
### Changed
- Renamed the directory from `Stella-MCP` to `atari2600-dev` (the engine isn't limited to Stella, and the
  deliverable is a whole environment, not a single MCP).
