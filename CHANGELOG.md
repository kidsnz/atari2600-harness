# Changelog

The change history of this project. Format follows [Keep a Changelog](https://keepachangelog.com/);
versions follow [Semantic Versioning](https://semver.org/).

> Entries from v0.17.0 and earlier are condensed; the full detailed history (in Japanese) is kept locally
> in `CHANGELOG.ja.md`.

## [Unreleased]

### Planned
- Real game authoring (production use of the harness; e.g. a Pong rematch).
- Extending the `step_scanline|clock` / `watch|trap` tools.

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
