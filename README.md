# atari2600-harness

[![CI](https://github.com/kidsnz/atari2600-harness/actions/workflows/ci.yml/badge.svg)](https://github.com/kidsnz/atari2600-harness/actions/workflows/ci.yml)

A **verification harness** that lets an LLM (Claude) author the Atari 2600 in 6502 assembly *accurately*.
It is not a game-generation app — it is two things: the loop substrate the model hammers on every iteration
(**assemble → run → inspect numerically**), and a **growing, hardware-verified knowledge base** (a
fundamentals audit + a techniques catalog) so 2600 capability *accrues* instead of being re-derived each
session. The aim is general, verified competence on the machine — not any one game.

## Premises

- **The primary author is the model (Claude).** The project owner does not read assembly. So this
  environment optimizes the model's authoring loop — precision and speed — rather than human readability.
- Past attempts (Pong, etc.) showed the failures were **not** code-generation ability but the **lack of
  execution feedback and cycle-exact timing verification**.
- Therefore the thing to build is **not** a game generator but a **verification harness the model can
  invoke on every iteration**.

## Design backbone — the five gaps (A–E)

The ways the model fails at 2600 assembly decompose into five gaps; each is closed by a tool or document.
See [`docs/gap-analysis.md`](docs/gap-analysis.md). **All of A–E are closed as of v0.21.0.**

| | Gap | One line | Status |
|---|------|------|---|
| A | Perception | Execution results are invisible (numeric state is needed) | ✅ closed |
| B | Timing | Cycles / beam position can't be counted in your head (★ most critical) | ✅ closed |
| C | Knowledge | 6502/TIA constants and behavior get misremembered | ✅ closed (rides on B) |
| D | Verification | No reproducibility / regression tests | ✅ closed |
| E | Friction | build→run→inspect isn't one command | ✅ closed |

Closing A–E (**phase 1**) made the loop trustworthy. **Phase 2 turns it into a general, verified 2600
capability base**, two living documents kept honest by the same numeric loop:
- a **[fundamentals audit](docs/fundamentals-audit.md)** — every TIA/6502 behavior classified *verified /
  documented / unknown* with sources, and a prioritized backlog of what to nail down next;
- a **[techniques catalog](docs/techniques/)** — each authoring technique re-implemented clean-room,
  hardware-verified, and CI-locked (e.g. sprite multiplexing).

Today the harness ships **20 MCP tools** and **20+ litmus/technique regression scenarios** (VDEL, HMOVE side
effects, asymmetric-playfield write windows, input ports, F8 bankswitching, 6502 cycle/BCD precision, …),
all green in CI on every push.

## Architecture

The engine — **Gopher2600** (Go) — is **embedded as a library in-process** and wrapped by a thin
**Go MCP server** (`modelcontextprotocol/go-sdk` v1.6.1, stdio). Its `hardware` / `television` / `setup`
packages are pure Go (no SDL), so headless numeric driving works. Every tool returns results as
**numbers (typed JSON)**.

- Assembler = **DASM** (`-f3`); pure 6502 cycle counts = sim65 / 6502profiler.
- Reference oracle = **Stella**; image overlay = in-house Go (`image/draw` + `fogleman/gg`).

### MCP tools (20, in `cmd/harness`)

`load_rom` / `step_frame` / `step_scanline` / `step_instruction` / `assemble_and_load` /
`read_cpu` / `read_ram` / `read_tia` / `read_tia_registers` / `read_cycles` / `read_collisions` /
`read_row` / `read_audio` / `read_bank` / `peek` / `poke` / `breakif` / `set_input` / `assert_line_budget` /
**`get_screen_annotated`** (returns image + numbers together — the primary user↔model comms channel).

Implementation spec: [`docs/mcp-tools.md`](docs/mcp-tools.md).

## Layout

This repository is the **harness only** (the general-purpose, reusable base). Game ROM artifacts live in
a **separate repository** that depends on this one (dependency flows game → harness; the harness has zero
dependence on any game).

```
atari2600-harness/
├── CLAUDE.md               # always-loaded project constitution (premises, rules, fixed constants)
├── README.md / CHANGELOG.md / LICENSE
├── go.mod                  # Go module (replace -> ./Gopher2600 for the local engine clone)
├── Gopher2600/             # external dependency (untracked; clone per setup below)
├── cmd/                    # executables (system — reused across all games)
│   ├── harness/            #   MCP server (19 tools)
│   ├── probe/              #   plumbing-check CLI
│   ├── scenario/           #   scenario regression runner (input timeline + numeric asserts)
│   └── calibrate/          #   horizontal X(N) sweep-fit
├── internal/               # base libraries (zero dependence on any game)
│   ├── emu/                #   Gopher2600 driver wrapper (headless, numeric)
│   ├── annotate/           #   annotated-screenshot rendering
│   ├── build/              #   dasm invocation
│   ├── scenario/           #   scenario regression (ROM-agnostic)
│   └── calibrate/          #   position calibration
├── pkg/                    # public packages (importable by game repos)
│   └── playfield/          #   playfield encoder (universal Atari 2600 knowledge)
├── roms/litmus/            # the harness's own verification ROMs (litmus_* / smoke / golden)
└── docs/                   # deep-dive docs (routing in CLAUDE.md)
```

`*.bin` / `bin/` / `preview/` / `Gopher2600/` are untracked (`.gitignore`); regenerate them with
dasm / `go build` / scenarios.

## Setup (macOS / Apple Silicon)

```sh
git clone https://github.com/kidsnz/atari2600-harness.git
cd atari2600-harness

brew install dasm cc65 pkg-config go            # assembler, 6502 sim, build deps
brew install --cask stella                       # reference oracle (optional)
git clone https://github.com/JetSetIlly/Gopher2600.git   # engine, into the repo root (untracked)
go mod tidy

go build ./... && go test ./...                  # should be green
go run ./cmd/probe                               # plumbing check (numeric output)
go run ./cmd/scenario roms/litmus/scenarios/*.json   # regression scenarios (exit 0 on all pass)
go run ./cmd/calibrate                           # horizontal calibration (reproduces slope 3 px/CPU-cycle)
```

`Gopher2600/` is referenced via the `replace` directive, so clone it directly under the repo root.

### Using it as MCP tools from Claude Code

An `.mcp.json` registers the harness binary (`bin/harness`) as the MCP server. Build the binary, then
restart Claude Code, and tools such as `get_screen_annotated` become available.

```sh
go build -o bin/harness ./cmd/harness   # produce the binary referenced by .mcp.json
# → restart Claude Code to load the "atari2600" MCP server
```

## Documentation

| Topic | Read |
|---|---|
| Why this design / anatomy of failure | [`docs/gap-analysis.md`](docs/gap-analysis.md) |
| Tool selection rationale / alternatives | [`docs/tool-landscape.md`](docs/tool-landscape.md) |
| Implementation spec / source of constants | [`docs/resources.md`](docs/resources.md) |
| MCP tool implementation spec | [`docs/mcp-tools.md`](docs/mcp-tools.md) |
| Scenario regression format | [`docs/scenarios.md`](docs/scenarios.md) |
| Litmus measurements (horizontal position, HMOVE) | [`docs/litmus-results.md`](docs/litmus-results.md) |
| Verified coverage (what each litmus proves on hardware) | [`docs/verified-coverage.md`](docs/verified-coverage.md) |
| Techniques catalog (verified 2600 authoring techniques) | [`docs/techniques/`](docs/techniques/) |
| Fundamentals audit (verified vs documented vs unknown, sourced) | [`docs/fundamentals-audit.md`](docs/fundamentals-audit.md) |
| Roadmap / next moves | [`docs/improvement-roadmap.md`](docs/improvement-roadmap.md) |
| Strengthening roadmap (sprites / audio / CI) | [`docs/hardening-roadmap.md`](docs/hardening-roadmap.md) |
| Decisions and changelog | [`CHANGELOG.md`](CHANGELOG.md) |

As far as a public search (GitHub/web, 2026-06) shows, no MCP server for the Atari 2600 exists yet —
emulator MCPs exist for other systems (C64 = vice-mcp, Game Boy = mcp-gameboy, Atari Lynx = gearlynx).
This is not a proof of absence; if prior art exists, pointers are welcome.

## License

**GPL-3.0-or-later.** This harness embeds [Gopher2600](https://github.com/JetSetIlly/Gopher2600)
(GPL-3.0) as a library, so the combined work is licensed under the GNU General Public License v3.
See [`LICENSE`](LICENSE). Copyright (C) 2026 kidsnz.

## Acknowledgements

- **[Gopher2600](https://github.com/JetSetIlly/Gopher2600)** by JetSetIlly — the Atari 2600 emulator
  embedded as the driving engine.
- **[Stella](https://github.com/stella-emu/stella)** — used as the reference oracle.
- **DASM** — the assembler.
