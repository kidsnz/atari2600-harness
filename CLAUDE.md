# CLAUDE.md — atari2600-harness

This file is **the only always-on context, auto-loaded in full every session**. Put only "invariant
premises, settled decisions, constants you must never get wrong, and which doc to read for which task" here.
Deep dives go in `docs/` (routing table below). Assume anything not here is unread. Don't put facts that
must always hold *only* in a doc — burn them here or into memory.

> **Language policy:** the published repo is English-only. The author works in Japanese and communicates in
> Japanese; Japanese copies of each `*.md` are kept locally as `*.ja.md` (gitignored, never published). When
> editing docs, keep the English `*.md` as the source of truth for the public repo.

## Invariant premises
- Goal: build a **verification harness** so Claude can author the Atari 2600 in 6502 assembly accurately
  (not a game-generation app).
- **The primary author is Claude.** The user doesn't read assembly. The environment optimizes Claude's
  authoring-loop precision and speed.
- **Top priority is gap B (timing).** Every past-Pong abandonment died on "unverified timing / positioning."

## Iron rules (follow every time)
1. **Judgment is numeric; screenshots are a supplement.** The final horizontal verdict is the TIA register
   value; vertical is the integer scanline. Don't decide by eyeballing pixel counts.
2. **Get cycles from the simulator** (Gopher2600 / sim65). Don't trust the DASM listing or mental arithmetic.
3. **Small steps.** edit → assemble → run → numeric check → commit. Revert to the previous step on failure.
   No bulk changes.
4. **litmus test:** place a sprite at an arbitrary X / move it 1px and have it match `X = 3N − 55`. If this
   passes, the environment is real.

## Settled architecture
- Engine = **Gopher2600** (Go) **embedded in-process as a library**, wrapped by a thin **Go MCP** (official
  `modelcontextprotocol/go-sdk` v1.6.1, stdio). `hardware`/`television`/`setup` are pure Go (no SDL), so
  headless numeric driving works. terminal/PushedFunction turned out unnecessary (settled v0.3.0).
- Every tool returns results as **numbers (typed JSON, with Coords)**. The image (`get_screen_annotated`) is
  a special case = the annotated screenshot below.
- Regression = **Gopher2600's `regress` + record/replay**. Pure-6502 cycles = sim65 / 6502profiler.
- Reference oracle = **Stella** (`-sssingle -ss1x`, `-tia.dbgcolors roygbp`, `-dbg.script`+`dump`).
- Image overlay = **in-house Go** (`image/draw` + `fogleman/gg`). No shelling out to ImageMagick.
- Assembler = **DASM** (`-f3`). **BizHawk not adopted (not on macOS).**
- MCP tools (**implemented**, `cmd/harness`): `load_rom` / `step_frame` / `read_cpu` / `read_ram` /
  `read_tia` / `peek` / `poke` / `breakif` / **`get_screen_annotated`** (v0.5.0, image + numbers together) /
  **`read_cycles`** (v0.12.0, exposes CPU cycles = rule 2 into the real loop; last instruction/interval/total) /
  **`assert_line_budget`** (v0.13.0, per-scanline budget guard = halt when a WSYNC interval overruns = a roll cause) /
  **`read_tia_registers`** (v0.14.0, measures current write-only register values = drops color inference) /
  **`read_collisions`** (v0.14.0, structures CXxx into named boolean pairs) /
  **`step_scanline`** (v0.15.0, advance until scanline +1) / **`step_instruction`** (v0.15.0, one instruction at a time) /
  **`assemble_and_load`** (v0.16.0, dasm→load in one shot; on failure returns structured dasm output) /
  **`read_audio`** (v0.17.0, reads TIA audio AUDC/AUDF/AUDV numerically = verify sound with numbers too) /
  **`read_bank`** (v0.43.0, current cartridge bank at PC + is_ram; **F8/F6/F4 verified** (litmus_bank, _f6, _f4), scenario fields `bank.number`/`bank.is_ram`) /
  **`analyze_image`** (v1.12.0+, screenshot→TIA data; multi-frame `paths[]` = static/dynamic separation + union tracks + flicker; `docs/ingest.md`) /
  **`analyze_screen`** (v1.19.0, ingest on the current emulator frame) / **`run_scenario`** (v1.19.0, regression verdicts live) /
  **`watch_ram`** (v1.20.0, RAM-change trap with writing PC). `step_clock`/`watch(bus)` parked (docs/mcp-tools.md).
  

## Constants you must never get wrong (source: `docs/resources.md`)
**Frame** — 1 line = 228 color clocks (HBLANK 68 + visible 160) = **76 CPU cycles** (3 clocks/cycle).
NTSC **262** = VSYNC 3 / VBLANK 37 / visible **192** / Overscan 30. PAL · SECAM 312 = 3/45/228/36.
Real games deviate, so don't hardcode "exactly 262" — handle as a range + warning.

**Beam coords (Gopher2600 `GetCoords`, hardware-verified v0.3.0)** — the `Clock` convention is
**HBLANK = −68..−1 / visible = 0..159** (first visible pixel = clock 0). **Same coordinate system** as a
sprite's `ResetPixel`/`HmovedPixel` = directly comparable. `Scanline` is a 0-based integer.

**Horizontal position** — missile/ball `X = 3N − 55`, **player is +1px → `X = 3N − 54`**
(N = CPU cycles from the sync point to the RESPx strobe). Leftmost X=2 (player 3).
The offset is TIA's ~5-color-clock delay + HBLANK 68. Granularity 3px. Coarse adjust is divide-by-15
(5-cycle loop). **litmus hardware-verified (v0.4.0):** slope 3px/CPU-cycle, coarse 15px/5cy, 160 wrap,
leftmost X=3. But the formula's **offset constant is kernel-specific** (includes the prologue's cycle count)
→ don't hardcode the absolute N; make the final position verdict by measuring **`read_tia`'s `HmovedPixel`**
(visible 0–159). When HMOVE hasn't fired, it equals `ResetPixel`.

**HMOVE** — upper nibble only, two's complement, **positive = left / negative = right**, range +7 to −8.
Moves only at the HMOVE strobe. HMOVE is **right after WSYNC**. (All 16 nibbles hardware-verified in litmus
v0.4.0: `$70`=left7 … `$00`=0 … `$F0`=right1 … `$80`=right8. 1px granularity.)
**Do not write HMxx within 24 CPU cycles after HMOVE** (Stella PG; unpredictable motion). HMOVE-after-WSYNC
extends HBLANK by 8 clocks = the left-side 8px blank on HMOVE lines; mid-line HMOVE moves objects RIGHT
~1px/4CLK (Towers TIA_HW_Notes; documented, not yet litmus-verified — see `docs/fundamentals-audit.md`).

**6502 timing/BCD (source: 6502.org)** — **stores never take page-cross penalties** (STA abs,X always 5,
(ind),Y always 6) = kernel store timing is deterministic; reads take +1 on page cross; branches 2/+1
taken/+1 page-cross. **NMOS decimal mode: only the C flag is valid** after ADC/SBC; D is unknown at
power-up → `CLD` in init is mandatory. ⚠️ `reference/docs_atari/cycle_counting_guide.html`'s position
math is approximate — never cite it for positions; use our calibrated X(N).

**Collisions (CXxx)** — two latches in each D7/D6, sticky. `BIT CXxx` → `BMI`(D7)/`BVS`(D6).
**CXCLR** = clear all collisions; **HMCLR** = clear the motion registers (a different thing).

**playfield (bit order, hardware-verified v0.6.0)** — 40 columns left→right, each 4 color clocks wide. **Two
sources (ABB/falukropp) agree + `read_row` measured.** `PF0` = upper nibble only, col0→D4..col3→D7 / `PF1` =
MSB first, col4→D7..col11→D0 / `PF2` = LSB first, col12→D0..col19→D7. Left half = clock 0–79, right half =
80–159. `CTRLPF` D0: 0=repeat (right half copies left) / 1=reflect (mirror). Verify with `read_row` (numeric,
not by eye).

**Hardware** — 128 bytes of RAM. ROM `$F000` (4K), vectors `$FFFA`.
**poke quirk** — `poke` is for RAM. Write-only TIA registers ($0D PF0 etc.) don't persist stably under poke →
change rendering with a `sta` in the ROM/kernel.

**Image input contract (user → Claude)** — for pixel-exact extraction ask for **Stella F12 snapshots
(PNG, unmodified, TV effects off)** = guaranteed integer scale, Retina-proof. OS screenshots are
conversation-grade only (non-integer scale → warnings). Hand-off point = umbrella `inbox/` (belongs to no repo). Size = any integer multiple of 160 (auto-detected). **Best input = the ROM file itself** (`cmd/fieldtest` self-drives Gopher2600 → full multi-frame analysis; drop ROMs in `inbox/`). F12 shots (2-3 consecutive for movement) are the fallback. Details: `docs/ingest.md`.

**Annotated screenshot (`get_screen_annotated`)** — not a Claude-only aid but **the primary user↔Claude comms
channel** = a first-class citizen. The user looks at the image and gives data visually ("move P0 to clock
80") → Claude translates directly to registers, a round-trip loop. So the grid is **calibrated to TIA real
coordinates** (horizontal clock 0–159 / vertical scanline 0–191, both axes always) so the user's coordinates
map straight to register values. Burn the current position as a **numeric label**. Human readability first
(×3–4 scale, axis labels). Besides the inline image, **overwrite a file each call** (env
`ATARI2600_SCREEN_PATH`, default `preview/screen.png` in `.mcp.json`) = VS Code preview auto-reloads for the
round trip. Also return `png_path` in JSON.

## Routing table (read before working)
| Task | Read first |
|---|---|
| Why this design / anatomy of failure | `docs/gap-analysis.md` |
| Tool selection rationale / alternatives | `docs/tool-landscape.md` |
| Implementation spec (Gopher2600 API / MCP / Stella flags) / source of constants | `docs/resources.md` |
| MCP tool implementation spec (go-sdk API, per-tool I/O) | `docs/mcp-tools.md` |
| Scenario regression format (input timeline + numeric assertions) | `docs/scenarios.md` |
| litmus measurements (authoritative horizontal-position / HMOVE data) | `docs/litmus-results.md` |
| verified coverage (what each litmus proves on hardware) | `docs/verified-coverage.md` |
| techniques catalog (verified 2600 authoring techniques: zone multiplexing, …) | `docs/techniques/` |
| fundamentals audit (verified vs documented vs unknown, with sources; 2026-06) | `docs/fundamentals-audit.md` |
| Exerciser ROM (integration showcase, 6 scenes; v1.0.0 keystone) | `docs/exerciser.md` |
| Stella oracle cross-check usage | `docs/stella-oracle.md` |
| Image ingestion (screenshot → TIA data) + **image input contract** | `docs/ingest.md` |
| Roadmap / next moves (prioritized) | `docs/improvement-roadmap.md` |
| Strengthening roadmap (sprites / audio / CI hardening) | `docs/hardening-roadmap.md` |
| Decision history and changelog | `CHANGELOG.md` |

## Repository layout (v0.22.0 spinoff, standalone repo)
**This repo = the harness base only (general-purpose, reused across all games).** Game ROM artifacts are
split into a **separate repo**. Dependency is **one-way game → harness** (the harness has zero dependence on
any game; even its tests reference only its own `roms/litmus`).
- Module = `github.com/kidsnz/atari2600-harness`. Gopher2600 via `go.mod` `replace => ./Gopher2600`.
- Physical layout (under the umbrella folder `260609_atari2600-dev/`, two sibling repos bound by `go.work`):
  ```
  260609_atari2600-dev/        ← umbrella (the folder Claude Code opens; .mcp.json/.claude live here)
  ├── go.work                   ← binds harness + roms locally
  ├── harness/                  ← this repo (atari2600-harness)
  └── roms/                     ← separate repo (atari2600-roms); frogger etc. live here
  ```
- Base contents: `cmd/harness` (MCP server) / `cmd/probe` (plumbing) / `cmd/scenario` (regression runner CLI) /
  `cmd/calibrate` (horizontal X(N) sweep-fit) / `internal/emu` (driving) / `internal/annotate` (annotation) /
  `internal/scenario` (scenario regression = input timeline + numeric assertions, ROM-agnostic) /
  `internal/calibrate` (position calibration = poke sweep + linear regression) /
  **`pkg/playfield`** (public encoder `EncodeSymmetric` etc. = universal Atari 2600 knowledge; the roms-side `gen` imports it).
- Verification ROMs: `roms/litmus/` (litmus_* / smoke / golden) = **the base's own property**, kept in this repo.
- Game artifacts (separate repo `atari2600-roms`): `<game>/` (`*.asm`/`*.bin`) + `<game>/gen/` (scene
  definitions + kernel generation, importing `atari2600-harness/pkg/playfield`) + `<game>/scenarios/*.json`.
  Example: `frogger/` (Monet Frogger).
- Add new games under the roms repo as `<name>/` (+`gen/`). Promote kernels you want to generalize to `pkg/`
  (like the encoder; YAGNI).

## Development environment (macOS / Apple Silicon)
`brew install dasm cc65 pkg-config go` / Stella: `brew install --cask stella`.
Clone Gopher2600 into the **harness/** root (untracked, referenced via `go.mod` `replace`).
**Run commands from each repo's root** (harness's own from `harness/`, ROMs from `roms/`). `go.work` assumed.
- ROM build: `dasm x.asm -f3 -ox.bin`.
- Plumbing check (harness/): `go run ./cmd/probe`. MCP server: `go build -o bin/harness ./cmd/harness`.
- litmus regression (harness/): `go run ./cmd/scenario roms/litmus/scenarios/*.json` (exit 0 on all pass).
- Calibration (harness/): `go run ./cmd/calibrate` (sweeps litmus_pos → reproduces slope 3 px/CPU-cycle).
- ROM generation (roms/): `go run ./<game>/gen [scene]`.
- ROM regression (roms/): `go run github.com/kidsnz/atari2600-harness/cmd/scenario <game>/scenarios/*.json`.

## Version control
For each meaningful change, append to `CHANGELOG.md` (Keep a Changelog) and tag with SemVer. Record decisions
in the CHANGELOG's "Decisions" section.
