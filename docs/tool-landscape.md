# Tool landscape — a map of tools/references against each gap

Maps tools and references onto gaps A–E from [`gap-analysis.md`](gap-analysis.md).
Verified (2026-06-09, macOS / Apple Silicon).

## Gap legend
A = execution results invisible / B = cycles & beam position uncountable / C = knowledge / D = regression
& reproducibility / E = iteration friction

---

## Comparison table (verified)

| Tool | Gaps filled | headless/scriptable | MCP-able | macOS install | License |
|--------|:---:|------|:---:|------|------|
| **Gopher2600** | A B C E | (settled at v0.3.0 = **embedded as a library**; terminal/`PushedFunction` turned out unnecessary) | **◎ adopted** | `brew install sdl2 pkg-config` → `go install` | GPL-3.0 |
| **Stella** | A B C E | `exec`/`autoexec`/`-dbg.script` + `dump` to file. **No socket, no headless rendering** | △ only via script + file | `brew install --cask stella` | GPL-2.0 |
| **BizHawk** | A B C D | has a Lua socket server | ✕ **not on macOS** | **deprecated (effectively impossible on Apple Silicon)** | MIT (mixed) |
| **8bitworkshop** | A C E | browser (Javatari); `make tsweb` for local | ✕ | clone + node | GPL-3.0 |
| **sim65** (cc65) | B E | CLI, `-c` outputs executed cycles | ○ wrappable | `brew install cc65` | zlib-ish |
| **6502profiler** | B D E | CLI, cycle measurement + Lua tests | ○ wrappable | `go install` | OSS |
| **6502_test_executor** | B D | CLI, JSON tests, cycle-count asserts | ○ wrappable | clone + make | OSS |
| **sim6502** (barryw) | B D E | CLI, deterministic + VICE backend | △ | .NET build | OSS |
| **DASM** | C E | assembler; `-l` for a listing (**no cycle annotation**) | n/a | `brew install dasm` | GPL-2.0 |
| **Atari Dev Studio** | C E | bundles dasm+Stella+batari; VS Code task-driven | ✕ IDE-coupled | VS Code extension | OSS |

---

## A perception / B timing — emulators

### Gopher2600 (the chosen engine)
The only high-accuracy 2600 emulator that can be driven programmatically on macOS. High-accuracy
6507/TIA/RIOT. **You can inspect and rewind state at CPU and color-clock (beam position) granularity** →
directly addresses racing-the-beam (gap B). The Go package `debugger/terminal` exposes **`PushedFunction`**
(pushing commands from your own Go process into the debugger goroutine); the terminal accepts a stdin pipe,
and `debuggerInit` provides a startup script.
→ **Wrapping it in a thin Go MCP server** is best. Exposure ideas: `load_rom` / `step_frame` /
`step_scanline` / `step_cycle` / `read_cpu` / `read_ram` / `read_tia` / `breakif` / `get_screen`
(framebuffer → image).

### Stella (human-facing visuals + reference oracle)
Confirmed to have **neither a socket nor headless rendering**. External control is only the snapshot style
"`-dbg.script` dumps to a file → read it". Unsuited as the live MCP engine.
Its roles are (1) human (pizza-boy-style) visual debugging and (2) the **final arbiter of accuracy** that
cross-checks Gopher2600's results.

### BizHawk (not adopted on macOS)
The Atari2600Hawk core + Lua socket server are powerful, but the **macOS port is deprecated** (no 64-bit
WinForms). Effectively unusable on Apple Silicon. **Not chosen in this environment.**

---

## B timing / D regression — pure 6502 sim/test (no TIA; for CI logic checks)

A layer that runs kernel timing math and pure-logic regression deterministically and fast. None of these
have TIA, so use them for **cycle counting and unit tests of separable 6502 routines** (whole-2600
verification is Gopher2600).

- **sim65** (bundled with cc65) — `-c` outputs executed cycle count. Instant via `brew install cc65`. Lightest.
- **6502profiler** — clock-cycle measurement + arrange/assert tests in Lua. Go build.
- **6502_test_executor** — JSON tests, cycle-count asserts, cc65-based.
- **sim6502** (barryw) — two backends: deterministic + VICE cycle-accurate. Commodore-leaning.
- **Klaus2m5 functional tests** — golden baseline for 6502 correctness (reference).

> **Important:** DASM's listing file **does not annotate cycle counts** (only line/address/bytes/source).
> Always get cycles from a simulator (sim65 / 6502profiler / Gopher2600).

---

## C knowledge — references (the primary sources to distill into CLAUDE.md)

**The owner already collected nearly all of these in `260304_Claude-Code-Pong/docs_atari/`** (→ gap C is a
distillation problem, not a collection problem).

- **Stella Programmer's Guide** `stella_programmers_guide.{html,pdf}` — the TIA bible, and **not the
  original document**. Steve Wright wrote it 12/03/79; the copy here says *"Reconstructed by Charles
  Sinnett 6/11/93"* and *"HTMLified by B. Watson 9/14/2001"*, and the PDF is a Word file
  (`/Author (Bob Colbert)`, 74 fonts, 9 images) — **a retyping, not a scan**. Three hands sit between
  Wright and this file, and retyping this document is known to change it: on AtariAge in 2024
  (`368057`) someone redrew its timing diagrams and found the hand-drawn lines did not line up with
  the OSC waveform; the answer, seven weeks later, was that they *"should actually be in the middle"* —
  **the untidiness was the information**. Three further facts, none recorded here before 2026-09-10:
  - **The trustworthy original is Atari Museum's `2600_Guide.pdf`** (identical to the archive.org
    copy), per spiceware in `321100`; the MiniDig edition carries VSYNC typos (both should read
    *"3 scanlines"*).
  - **The community's current text is dionoid's 2024-12-14 revision** (`278499`) — OCR errors fixed,
    missing content restored. A separate **1988-07-01 revision by Darryl May** circulates as a PDF
    (`368057`); nobody in that thread could say what it changed, and it is **not in this tree**.
  - **A substantive erratum**: the Guide's horizontal-positioning text (*"15 colour clocks"*) is
    wrong without its missing premise — a bare `RESx` strobe is limited to **3 colour clocks**
    (1 CPU cycle), corrected by seagtgruff in `172089`. This repository already uses the corrected
    value (`docs/techniques/sprite-placement.md`, `x = 3c - 60`) and never absorbed the wrong one.
  All 26 citations of the Guide outside `CHANGELOG.md` reference its prose, its register tables or
  its 400 us rule; **none reference the timing diagrams**, so the redrawing risk does not reach any
  claim here. Verified 2026-09-10.
- **Guide to Cycle Counting** (Nick Bensema) `cycle_counting_guide.html` — ★ the core of B/C
- **Programming for Newbies** (Andrew Davie) `Atari_2600_Programming_for_Newbies.{pdf,txt}` — especially Session 22 (horizontal position)
- **woodgrain wiki** `Playfield_Timing.html` / `Clock_Speeds.html` / `Memory_Map.html` / `Bank_Switching.html` / `Sound.html`
- **vcs.h / macro.h** — TIA register name definitions
- **the correct horizontal positioning** `8bitworkshop_samples/sethorizpos.asm` (divide-by-15 routine)
- **real-game disassemblies** `game_disassembly/` (adventure, pitfall, kaboom and 21 others), `za2600/` (Zelda reimplementation)
- **samples** `8bitworkshop_samples/`, `nanochess_samples/`, `spiceware_tutorial/`
- **6502 reference** `6502_reference.md`, `vcs_reference.md`, `tia_colors_ntsc.md`, `2600_music_guide.txt`

---

## E friction — iteration reduction / scaffolding

- **DASM** — the standard assembler. `brew install dasm`. build: `dasm x.asm -f3 -ox.bin`
- **Atari Dev Studio** (VS Code) — bundles dasm+Stella+batari. Unsuited for MCP but useful as a **source of
  correct macOS binaries**
- **batari Basic** — takes over kernel timing. A scaffold / comparison point when pure asm gets stuck

---

## Existing MCP / harnesses (prior art)

- **mcp-gameboy** (mario-andreschak) — a TS MCP wrapping `serverboy`. `load_rom` / inputs /
  `get_screen`→`ImageContent`. **The 2600 MCP design follows this** (return the screen as an image = gap A).
- **vice-mcp / ViceMCP** (barryw) — embeds an MCP into VICE's C core with 63 tools (break/step/registers/
  memory/VIC-II/SID/screenshot). **Builds on macOS.** The best proof that "the model fully drives an
  emulator + embedded MCP" works (though Commodore).
- **CTalkobt/sim6502** — a Node MCP (assemble/step/reg/mem/breakpoint). No TIA; currently proprietary.
- **An Atari 2600-specific MCP was not found** in a public search (GitHub/web, 2026-06). Emulator MCPs exist
  for other systems (C64 = vice-mcp, Game Boy = mcp-gameboy, Atari Lynx = gearlynx). This is not a proof of
  absence — if prior art exists, pointers are welcome. (`bradleylab/stella-mcp` is unrelated = for System
  Dynamics modeling.) → **this looks like open ground = the novelty of this project.**

---

## Settled architecture

```
[ Claude Code ]
   │  MCP
   ▼
[ Gopher2600-backed MCP server (Go) ]   ← A/B/C/E: load_rom, step_*, read_cpu/ram/tia, breakif, get_screen
   │
   ├─ DASM (brew)                         ← assemble
   ├─ sim65 / 6502profiler                ← B/D: cycle measurement & regression of separable logic (CI)
   └─ Stella (brew cask)                  ← reference oracle + human visual check (-dbg.script + dump)
```

- **Engine = Gopher2600** (the only one that can be driven at beam granularity on macOS). BizHawk not
  adopted (not on macOS).
- **Regression layer = sim65 / 6502profiler** (pure-6502 cycle counting and CI).
- **Oracle = Stella** (not used for the live MCP; for final accuracy checks and humans).
- **Novelty:** an MCP that understands the 2600/TIA was not found in a public search (GitHub/web, 2026-06)
  (other systems have vice-mcp=C64, mcp-gameboy=GB, gearlynx=Atari Lynx, etc.). If prior art exists,
  pointers are welcome. The design follows mcp-gameboy; the shape is proven by vice-mcp.
