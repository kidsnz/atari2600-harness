# Resource plan — the references this project needs

`docs_atari/` (the learning corpus for Pong) was gathered for "learn by writing a game". This project
"**builds a verification harness + distills constraints**," so the kind of references needed changes.
Here we **re-inventory what's needed**, classify "already have / newly need", and set the research targets.

Legend: ✅ = already in docs_atari / 🔍 = new research / ⭐ = directly tied to a past failure (priority)

---

## Category 1 — core authoring constants (for distilling into CLAUDE.md)

The countermeasure for gap C is "distillation," not "collection." A concise, definitive version that
**stays in context without relying on memory** is needed.

- ⭐🔍 **The exact horizontal-position formula and hardware-specific offset** — the relation between RESPx
  strobe cycle and displayed pixel, why `P1 = 160 - P0 - width` doesn't hold, divide-by-15 coarse adjust,
  the HMxx nibble → ± pixel table. (= the core for never repeating Pong's failure #1, "brute-forcing magic
  constants." With authoritative numbers.)
- ⭐🔍 **HMOVE behavior** — the HMOVE comb (black line at the left edge), early HBLANK, HMOVE required right
  after WSYNC, per-line HMOVE.
- 🔍 **Definitive frame budget values** — total lines for NTSC/PAL/SECAM; VSYNC/VBLANK/visible/overscan; 228
  color clocks / 76 CPU cycles / 68 HBLANK.
- ✅🔍 A distillation cheat sheet of the **full TIA register bitmap** and the **6502/6507 instruction-cycle
  table** (including branch and page-crossing penalties). (The Stella Programmer's Guide has it, but a
  "one-pager" is needed.)
- 🔍 **How to read the collision registers (CXxx).**

## Category 2 — harness-building references (almost all new; not needed for Pong)

- 🔍 **Gopher2600 developer docs** — the Gopher2600-Docs wiki, all debugger terminal commands, programmatic
  driving via `PushedFunction`, framebuffer extraction, color-clock inspection.
- 🔍 **Go's MCP SDK** — the official go-sdk and mark3labs/mcp-go, the 2026 recommendation, tool exposure and stdio.
- 🔍 **Reference implementations** — the architectures of mcp-gameboy (returning the screen as an image) and
  vice-mcp (the embedded-MCP shape).
- 🔍 **Stella's exact spec** — CLI flags for a 1x single snapshot, the output format of `-dbg.script`/`dump`,
  how to enable **Fixed Debug Colors** (which color is which object).
- 🔍 **Image overlay technique** — compositing an XY grid + axis labels + markers onto a 160×192 snapshot
  (Go image vs ImageMagick).

## Category 3 — verification/test references (mostly new)

- 🔍 **Test ROMs and known-state references** — for calibrating and checking the correctness of the harness
  (emulator) itself.
- 🔍 **Deterministic input replay** — how to replay movies / input scripts in Stella / Gopher2600 (gap D).
- 🔍 **2600 regression-test examples** — automated testing approaches from the homebrew community.
- ✅🔍 **Concrete test-description examples for sim65 / 6502profiler.**

## Category 4 — re-evaluating existing references

- ✅ The `docs_atari/` inventory is post-mortemed (see `gap-analysis.md`).
  This plan's role is to fill "what's missing from the new viewpoint" via 1–3 above.

---

## Research streams (run in parallel)

- **Stream A (harness building) = category 2** — ✅ `done 2026-06-09` (end).
- **Stream B (domain constants + verification) = categories 1 & 3** — ✅ `done 2026-06-09` (below).

> Reflect results into this file and use them as input to the CLAUDE.md distillation (Phase 4) and the
> harness implementation (Phase 2).

---

## Stream B results (domain constants + verification) 2026-06-09

Settled values that can be dropped directly into CLAUDE.md / `docs/2600-constants.md` for distillation.

### ⭐ Horizontal position (the antidote to failure #1) = NEW, most critical
- **Formula:** missile/ball `X = 3N − 55` (N = CPU cycles from the sync point to the RESPx strobe),
  **player is +1px → `X = 3N − 54`**. Leftmost X = 2 (player 3).
- **What the offset is:** RESPx is a strobe; after the store completes there is **about a 5-color-clock
  delay** before the TIA starts drawing. This plus the **68-clock HBLANK** is why "`160 − P0 − width`
  doesn't hold." A strobe during HBLANK places the object at the far left.
- **Granularity:** 3 clocks / CPU cycle → RESPx is in **3 px steps**. The rest is closed with HMOVE.
- **divide-by-15 coarse adjust:** divide the target X by 15 and burn time with a **5-CPU-cycle (=15
  color-clock) loop**. `SBC #15 / BCS loop`. Use the remainder (0–14) to index the HMOVE fine value from a
  page-boundary-aligned table.

### ⭐ HMOVE nibble table (trap: positive = left / negative = right)
Upper nibble (D7–D4) only, **two's complement**, range **+7 (left 7) to −8 (right 8)**. Moves **only at the
HMOVE strobe**.
```
$70=left7 $60=left6 $50=left5 $40=left4 $30=left3 $20=left2 $10=left1 $00=0
$F0=right1 $E0=right2 $D0=right3 $C0=right4 $B0=right5 $A0=right6 $90=right7 $80=right8
```

### HMOVE comb / timing
- HMOVE **extends that line's HBLANK by 8 clocks** (LRHB decode) → the left 8 px go black, and look like a
  "comb" when uneven.
- **Per-line HMOVE** turns the comb into a solid **bar** over the left 8 px (Pitfall!, etc.). HMOVE at
  **cycle 73–74** erases the black line.
- Strobe HMOVE **right after WSYNC**. The low-level reference is Andrew Towers' "TIA Hardware Notes".

### Frame budget (settled values)
- **1 line = 228 color clocks (HBLANK 68 + visible 160) = 76 CPU cycles** (3 clocks/cycle).
- NTSC **262** = VSYNC 3 / VBLANK 37 / visible **192** / Overscan 30.
- PAL · SECAM **312** = 3 / 45 / visible **228** / 36.
- **Caution:** real games deviate (NTSC 248–286, etc.). The harness does not hardcode "exactly 262" (use a
  range + warning).

### Collision registers (CXxx)
8 read-only registers, each with two latches in **D7/D6**, **sticky**. Test with `BIT CXxx` →
`BMI`(D7)/`BVS`(D6).
```
CXM0P : D7=M0-P1 D6=M0-P0    CXM1P : D7=M1-P0 D6=M1-P1
CXP0FB: D7=P0-PF D6=P0-BL    CXP1FB: D7=P1-PF D6=P1-BL
CXM0FB: D7=M0-PF D6=M0-BL    CXM1FB: D7=M1-PF D6=M1-BL
CXBLPF: D7=BL-PF (D6 unused)  CXPPMM: D7=P0-P1 D6=M0-M1
```
- **CXCLR** (write strobe) = clear all collision latches. **HMCLR** (write strobe) = zero the motion
  registers (HMxx) (a different thing from collisions).

### ⭐ playfield bit order (verified on hardware, v0.6.0, `litmus_pf` / read_row)
The **two independent implementations** ABB (`kirkjerk/atari-background-builder`) and falukropp
(`vcs_playfield_editor`) **agree**, and it is further **numerically verified** via `read_row` on Gopher2600.
**40 columns left→right on screen, each 4 color clocks wide:**
```
col:  0 1 2 3 | 4 5 6 7 8 9 10 11 | 12 13 14 15 16 17 18 19
reg:  PF0     | PF1               | PF2
bit:  4 5 6 7 | 7 6 5 4 3 2 1  0  | 0  1  2  3  4  5  6  7
```
- **PF0** = upper nibble only. col0→D4 / col1→D5 / col2→D6 / col3→D7 (lower nibble unused).
- **PF1** = MSB first. col4→D7 … col11→D0.
- **PF2** = LSB first. col12→D0 … col19→D7.
- Left half = visible clock 0–79, right half = 80–159. **CTRLPF D0=0 → repeat (right half copies left) /
  D0=1 → reflect (mirror)**.
- litmus measurement (scanline 100): `PF0=$10`→clock 0-3 / `PF1=$80`→16-19 / `PF2=$01`→48-51, repeating in
  the right half. Each exactly 4 clocks wide.
- **Caution (poke quirk):** write-only TIA registers ($0D/$0E…) do not persist stably under `poke` (poke is
  for RAM). To change rendering, **`sta` from ROM/kernel** rather than poke. Same for position and color.

### Verification/test references (category 3) = NEW
- **Klaus Dormann 6502 functional test** — the gold standard for CPU correctness (on success PC halts at a
  known address). Gopher2600 uses it too.
- ⭐ **Gopher2600 has record/replay + a built-in `regress` (regression test DB)** — golden image diff of
  frame hashes, off the shelf. The prime candidate for gap D.
- **Differential testing = use Stella as the oracle** (Gopher2600's author also uses Stella as the accuracy
  baseline).
- **Visual6502 / perfect6502** (transistor level) = the final truth when facts disagree.
- Existing TIA test ROMs do **not** cover edges like the HMOVE comb, late-HMOVE, RESPx during HBLANK →
  calibrate with Stella + real-hardware captures.

### References (cheat sheets for distillation)
- 6502 instructions/cycles: masswerk `6502_instruction_set` (branch: not taken 2 / taken same page 3 / page
  cross 4; `abs,X`/`abs,Y`/`(ind),Y` crossing +1).
- TIA register table: Stella Programmer's Guide / Computer Archeology / NO\$ `2k6specs` / `vcs.h`.

### Needs manual confirmation (the subagent couldn't WebFetch)
- The bit notation of masswerk's HMOVE table (the summarizer dropped the sign bit. Substituted Stella Guide
  values, cross-checked across sources, but double-check to be safe).
- **The exact command syntax of Gopher2600 `regress`** (not collected in Stream A either; confirm when
  starting the regression layer).

---

## Stream A results (harness building) 2026-06-09

Implementation spec for Phases 1–2. Latest Gopher2600 **v0.56.0** (2026-06), official Apple Silicon support.

### Gopher2600 driving (engine)
- **terminal commands:** `STEP` / `QUANTUM` (`CPU` or `CLOCK` — ★`CLOCK` steps in **color-clock units** =
  beam granularity) / `SCANLINE` / `FRAME` / `PEEK` / `POKE` / `CPU` `RAM` `TIA` `RIOT` `TV` (display each
  subsystem) / `WATCH` (halt on read/write, symbols allowed) / `BREAK` / `TRAP` (halt on change) / `REWIND` /
  `SCRIPT` (record · replay) / `ONSTEP` · `ONHALT` · `ONTRACE` (auto-run each time). The startup script is
  **debuggerInit** in the config dir.
- **Go API:** `hardware.VCS{ CPU, Mem, RIOT, TIA, Input, TV, Clock }`.
  Beam position `vcs.TV.GetCoords() → coords{Frame, Scanline, Clock}`.
  Step `vcs.Step(onColorClock func(bool)error)` / `vcs.RunForFrameCount(n, ...)`.
  save/restore `Snapshot()/Plumb()` (note: **the TV beam state is not included**).
- **External driving:** implement your own (non-interactive) `debugger/terminal.Terminal` + push closures
  into `ReadEvents.PushedFunction` / `PushedFunctionImmediate` to read state and run commands in sync with
  the emulation loop.
- **Framebuffer:** implement a `PixelRenderer` and `tv.AddPixelRenderer`. Get a 2D bitmap via `SetPixels`,
  col-lum→RGB via `television/colourgen`, visible width ~160 → a 1x capture as `image.RGBA`.
- **Build (macOS):** `brew install sdl2`, Go (see go.mod for the minimum; the wiki's 1.16 is old),
  `go build -tags=release .`.

### MCP server (Go)
- **Adopt the official `github.com/modelcontextprotocol/go-sdk`** (stdio, typed tools, auto JSON schema from
  structs). `mcp-go` only when HTTP/SSE is needed.
- Pattern: `mcp.NewServer` → `mcp.AddTool[In,Out](server, &Tool{...}, handler)` → `server.Run(ctx, &mcp.StdioTransport{})`.
  Return the screen as `ImageContent{Data []byte, MIMEType:"image/png"}` (the SDK base64-encodes it).

### Reference-implementation patterns (followed)
- ⭐ **action-returns-screenshot** (mcp-gameboy): every tool returns the latest frame image → unifies "what
  you did" and "observing the result."
- Hide low-level driving (Gopher2600 terminal/PushedFunction) behind the MCP tools and return structured data
  (hex strings, registers).
- Fine-grained tools + an `execute_batch`-style bundled run to cut round-trips (ViceMCP ~10x).
- Map `BREAK`/`WATCH`/`TRAP` to checkpoint-style tools.

### Stella (oracle + annotated screenshot)
- 1x single: `stella -snapsavedir DIR -sssingle -ss1x -snapname rom ROM`.
- `-dbg.script FILE` (load order `autoexec.script` → `<rom>.script` → `-dbg.script`).
  `dump START [END] FLAGS` (**1=memory / 2=CPU / 4=inputs**, additive so `7`=all).
- **Fixed Debug Colors:** `-tia.dbgcolors roygbp` = **P0=red / M0=orange / P1=yellow / M1=green / PF=blue /
  BL=purple** (fixed order P0,M0,P1,M1,PF,BL).

### Image overlay
- **In-house Go** (`image`/`image/draw`/`image/png` + `fogleman/gg` for lines and text). **Do not shell out
  to ImageMagick** (avoid external dependency and nondeterminism = important for a verification harness).
  Scale 160×192 up by an integer factor (×3–4, nearest) for readability when drawing; keep the 1x original
  as the judgment basis.

### Needs confirmation (UNVERIFIED)
- The argument spelling of `QUANTUM` (CPU/CLOCK/VIDEO) and the condition grammar of `BREAK`/`TRAP` → in-app `HELP`.
- Gopher2600's minimum Go version → `go.mod`. / `regress` subcommand syntax → when starting the regression layer.
- Stella `dump`'s exact output layout and the manpage wording (option names and color order verified; wording not).
