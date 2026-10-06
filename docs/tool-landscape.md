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

**8bitworkshop's built-in emulator was, in 2023, an old Javatari.** On a `RESPx` multi-sprite trick that
drew wrongly there and correctly in Stella, jetsetilly: *"8bitworkshop is awesome but these more advanced
methods aren't always displayed correctly"*, and his advice was *"to use Stella (or Gopher2600 or 6502.ts)
when exploring these more advanced ROMs"*; Al_Nafuur put it on Javatari v0.91 against a current v5.0.4
(AtariAge `topic/346786`; **Cited only, not verified**). What 5.0.4 still drew wrongly is in the "browser
preview beats against your monitor" row of `known-traps.md`.

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

**The accuracy is paid for in speed.** Its author, JetSetIlly, 2024: *"It's a slower emulator in general
than Stella, so if you're machine is low powered it will struggle to run at full speed"* — said to a
developer who could not find how to speed it up, in the thread where it reproduced a bank-switching
failure that Stella's developer mode did not (`stella-oracle.md`; AtariAge `topic/367912`). Not timed
against Stella here: **Cited only, not verified.**

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

The finer check people have asked for is an expected cycle per instruction. Thomas Jentzsch proposed it
for Stella (about 2010): hints in comments such as `=@48`, `>@24`, `<@60`, *"Stella would check that
information during runtime and react to it"*. Nukey Shay's example shows what a mismatch narrows to: at
his label the count should be 15, and if not, *"either the BCS branch between the two segments crosses a
page break … or the Y index when added to vector "p1GFX" crosses one"* — two causes, for that code
(AtariAge `topic/164572`; **Cited only, not verified**; whether Stella shipped it is not in the thread).
This repository checks per WSYNC interval, not per instruction: `prove_line_budget` proves each
interval's worst case with branch and index page-crossing penalties counted (`internal/cyclebound`),
`profile_line_budget` measures it, and the comments the prover reads are declarations, not
expected-cycle stamps: `@lines N` says an interval spans N scanlines (budget N × 76), `@amax N` bounds a
divide loop's accumulator. A declaration is not trusted: `TestLinesDeclarationIsNotSlack`
(`internal/cyclebound`) runs every annotated corpus ROM and fails when N is more lines than the
machine's worst run of that interval occupies, because the spare line lets an overrun certify.

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
    Section 8.0 of the HTML copy here repeats the 15-clock premise and then reads *"Objects can not
    be placed at any color clock position across the screen"* — one of the Sinnett reconstruction's
    spelling errors that dionoid, who used that reconstruction as the source of a printed booklet,
    calls *"actually misleading"*: *"can now"* is meant (`205774`). He adds that *"the information
    on the VDEL registers is incorrect"*, without saying which part (**Cited only, not verified**).
    No other VDEL statement in this tree cites the Guide (`rg -i VDEL`, filtered for
    `programmer|SPG|guide`, finds only this note, 2026-10-01).
  All 26 citations of the Guide outside `CHANGELOG.md` reference its prose, its register tables or
  its 400 us rule; **none reference the timing diagrams**, so the redrawing risk does not reach any
  claim here. Verified 2026-09-10. Outside this entry they name the Guide but not its edition, and
  the editions differ in their errors (above). **Name the edition when citing a document, and the
  release or commit when describing an emulator's behaviour** — this repository's rule; the document
  half is drawn from the editions above, not stated in any thread. Gopher2600's author suggested the
  emulator half, after finding an AI-written emulation note that listed two `QUANTUM` modes where
  the engine has three: *"include version information about the emulators in the md file. I use
  semantic versioning for gopher2600 and try to log breaking changes when they occur"* (JetSetIlly,
  `391096`, 2026; `docs/resources.md` carried the same kind of error — see its Stream A section).
- **Guide to Cycle Counting** (Nick Bensema) `cycle_counting_guide.html` — ★ the core of B/C
- **Programming for Newbies** (Andrew Davie) `Atari_2600_Programming_for_Newbies.{pdf,txt}` — especially Session 22 (horizontal position)
- **woodgrain wiki** `Playfield_Timing.html` / `Clock_Speeds.html` / `Memory_Map.html` / `Bank_Switching.html` / `Sound.html`
- **MiniDig — Best of Stella** (`http://www.qotile.net/minidig/`) — an earlier distillation of the
  mailing list this tree's `stella-list` citations come from. kisrael's reading list on AtariAge
  (`topic/320754`): *"The Stella mailing list was the previous core of Atari homebrew, this is a
  distillation of disassemblies, tricks, etc"*. Not collected here; its edition of the Guide is the
  one with the VSYNC typos above. **Cited only, not verified.**
- **vcs.h / macro.h** — TIA register name definitions. In 2018 the copies DASM ships were in its
  **source** download under `machines/atari2600` (the directory `8bitworkshop-crosscheck.md` passes with
  `-I`); someone who had not found them in the SourceForge distribution had to ask, once Andrew Davie
  stopped hosting his DASM site (AtariAge `topic/283030`, answered by Karl G; **Cited only, not
  verified**); the same thread also points to SvOlli's `vcs.inc` as another version.
- **the correct horizontal positioning** `8bitworkshop_samples/sethorizpos.asm` (divide-by-15 routine)
- **real-game disassemblies** `game_disassembly/` (adventure, pitfall, kaboom and 21 others), `za2600/` (Zelda reimplementation)
- **samples** `8bitworkshop_samples/`, `nanochess_samples/`, `spiceware_tutorial/`
- **6502 reference** `6502_reference.md`, `vcs_reference.md`, `tia_colors_ntsc.md`, `2600_music_guide.txt`

---

## E friction — iteration reduction / scaffolding

- **DASM** — the standard assembler. `brew install dasm`. build: `dasm x.asm -f3 -ox.bin`
  - *How full is the image?* The final `ORG $FFFA` pads the file to 4K, so the size hides it. Erik Mooney,
    2005: *"In the source, you can put a line: ECHO * before the final ORG. When DASM assembles the
    source, it'll print out the current address when it gets there"*; Jacob Rose: *"Cool, that works
    great"*. Thomas Jentzsch's alternative sums what alignment leaves free:
    `FREE SET FREE + 256 - (. % 256)` before each `ALIGN 256` 〔stella-list `200501/msg00011`,
    `200501/msg00012`, `200501/msg00021`〕. **Cited only, not verified.**
  - *Which part of `#<` / `#>` is DASM's own.* `LDA #>TABLE` loads the high byte of an address and `LDA #<TABLE` the
    low byte (Piero Cavina's example). Greg Troutman, 1998: *"The # sign means immediate data, which is
    the main distinction, and is standard across all 6502 assemblers I've seen. The < or > symbols are
    the HI vs. LO 8 bit portion of the 16 bit address and this part is handled differently from one
    assembler to the next"* (his order; in DASM `<` is the low byte). When Ruffin Bailey tried his source with
    the `#`s removed, RASM rejected each `<` and `>`; Cavina bet that they do exist in RASM, and Bailey replied he was *"pretty sure that I've found
    the similar commands"* 〔stella-list `199801/msg00037`, `199801/msg00040`, `199801/msg00042`,
    `199801/msg00053`〕. **Cited only, not
    verified.** This repository writes the DASM form (e.g. `docs/techniques/rts-dispatch.md`).
- **Atari Dev Studio** (VS Code) — bundles dasm+Stella+batari. Unsuited for MCP but useful as a **source of
  correct macOS binaries**
- **batari Basic** — takes over kernel timing. A scaffold / comparison point when pure asm gets stuck
- **The editor as a check.** Two habits from AtariAge. Write sprite bytes so they look like the picture:
  Thomas Jentzsch attached a file in 2015 (`graphics.zip`) that lets you write `.byte zz_XXX____` in
  place of a binary literal (`topic/233343`). And let syntax colouring see what the eye cannot:
  SpiceWare's jEdit mode makes graphics written as binary numbers easy to see, and colours register
  names, so `RESPO` typed with a letter O instead of `RESP0` shows in a different colour before anything
  is assembled (`topic/230320`, 2014). **Cited only, not verified.**
- **Page-constrained placement, as asked for in 2017, was mostly done by hand.** Kylearan (2017) wanted a linker
  that places sections marked `align` or `nocross` (*"that part must not cross a page boundary"*); of
  the alternatives offered (XA, ca65, KickAssembler, K2asm, ACME), *"From a quick glance … none of them
  seems to support the "nocross" declaration in conjunction with a linker"*, KK's k65 *"more or less has
  such a linker"* without standard mnemonics, and he started writing his own. He had recommended K65
  (devkk.net) the year before as a framework that *"abstracts a lot of things (functions, loops, inlining,
  bank switching etc.), optimizes the placing of code and data segments for you during linking
  (aligned/unaligned)"*, and in a later post *"K65 sure has some questionable design, I have been annoyed by it more than once.
  But its greatest strength is providing a good layer of abstraction while at the same time leaving the
  developer in full control of everything if need be"* (AtariAge `topic/251145`, 2016;
  **Cited only, not verified**). Inside DASM the check is a
  macro instead: same-page branch macros (`sbne` and the rest, credited to John Payson, posted by
  SpiceWare) that stop the assembly when a branch target is on another page (AtariAge `topic/264527`;
  **Cited only, not verified**). A taken branch that crosses a page costs a cycle (`docs/techniques/branch-always.md`).
- **Keeping 6507 and ARM definitions in step (DPC+).** joe-musashi (2015): assemble once for the `.sym`
  (no binary yet, since the ARM code is not built), turn the `DD_`-prefixed symbols into a C header with
  `grep` and `awk`, build the ARM C, assemble again. Under `RORG` a symbol lands at the wrong address for
  the C side — SpiceWare's font sat at `$Fxxx` in the symbol file where C needed `$4xxx`, handled with
  DASM `echo "#define FONT ", [[Font & $fff] + $4000]d` or an `awk` `substr` — and constants are kept
  apart from addresses by prefix (`DD_CONST_` / `DD_ADDR_`) (AtariAge `topic/236931`; **Cited only, not
  verified**). This repository builds no ARM code; it reads `.sym` only to resolve symbols
  (`internal/srcmap`).
- **Labels from ca65 in Stella.** gauauu's 2018 script turns ld65's label file (`-Ln labels.txt`) into
  DASM `.sym` lines (`<label> <addr> (R )`), so Stella shows labels for a ca65-built ROM; Stephen Anthony
  said then that reading ca65 directly was on Stella's list but not scheduled (AtariAge `topic/278754`;
  **Cited only, not verified**). This repository assembles with DASM only (`internal/build`), so its own
  symbol reader never sees ca65 output.
- **Why the 2600 long had no IDE.** Asked in 2005 why the 2600 had none, the answers were that the
  parts existed and people wired them into their own editor (Andrew Davie: Visual Studio → DASM → a
  Krokodile Cart; Cybergoth: TextPad with DASM and z26), and that the missing part was a debugger —
  batari: *"all anyone here needs is a debugger. The ability to set breakpoints, single-step and all that
  would be nice"*. Stephen Anthony announced Stella's integrated debugger for 2.0 in the same thread, and
  not everyone wanted one (Cybergoth argued that working without it teaches more) (AtariAge
  `topic/68569`; **Cited only, not verified**).
- **How it was done at the time.** Asked in 2016, tschak909 described one setup of several (*"there were
  a lot of ways to do it"*), starting *"circa 1977"*: code written on paper and keyed in by an operator, or later at
  your own terminal, on a minicomputer (a DEC PDP-11 in his example) that ran a cross assembler, and a development system that
  *"was typically an Atari VCS, which has been opened, the CPU has been removed or bypassed, and attached
  to a homegrown box"* (*"if you want a commercial equivalent, look up the HP 64000"*), with *"a specially
  made cartridge that contained RAM and circuitry to allow for that RAM to be written to either via
  RS-232, or an 8-bit parallel port. Lots of different names for such a device, such as a
  "Romulator""*. His example of *"A common configuration"* sent the object code to it through the
  pass-through EXTENSION port of an ADM-3A terminal. His variations include Garry Kitchen's Donkey Kong, written on an Apple ][ Plus whose modified
  language card was mapped onto a romulator umbilicus in the cartridge slot, and Atari 400/800 hosts at
  Apollo around 1981-82 (AtariAge `topic/253565`). Bruce-Robert Pocock, 2022: *"My understanding is that
  the original Atari devs in 1976-77 were using a timesharing minicomputer that they had terminal access
  into"* (`topic/340930`). Our reading: the 2600 was cross-developed from the start. The 2021 thread
  titled "ROMulator" (`topic/321808`) is about a present-day product, not this device. **Cited only, not
  verified.**
- **Browser drawing aids for the playfield and for kernel timing.** gauauu's playfield editor (2015,
  `tolberts.net/pf.html`, a remake of Kirk Israel's Playfield Pal) paints by click-and-drag, toggles a
  grid, and *"you can edit the code or the graphics, and the other immediately updates"*; *"It's
  still missing support for different colors"*, and karl-g later wished for *"async playfield support"*
  (AtariAge `topic/237824`). Kernel Paint (yvar-de-goffau, 2018,
  `github.com/Yvar-deGoffau/Kernel-Paint`) is *"a tool for drawing out kernel code"* to *"ease out
  development of correctly timed drawing code, using a simple drag-and-drop tool"*; sheldon-sims made a
  variant he called one *"for reflected playfields (PF0/PF1/PF2/PF0/PF1/PF2)"* that, in his words,
  *"repeats the playfield order instead of mirroring it"*, and said *"This really takes the guesswork out
  of the cycle counts, and figuring out when it is safe to re-write to the playfield registers"* (AtariAge
  `topic/276534`). Neither tool was opened here: **Cited only, not verified.** The nearest check here runs
  after the code exists, not while drawing, and covers less: `cyclebound.CheckPFDeadlines` looks only at
  the first and second write to each of `PF0`-`PF2` in a region (the first only for `COLUPF`/`COLUBK`) and
  asks whether each lands no later than its deadline (later writes are reported as unchecked); it does not check that a rewrite comes too
  early, which is the other half of "when it is safe to re-write".
- **A listing scanner for one console family's faults.** SpiceWare, 2016: *"during the development of
  Stay Frosty 2 we had numerous issues with DPC+'s Fast Fetch mode and some 7800s. We were able to figure
  out trigger conditions so I wrote evil7800, which analyzes the listing created by dasm to spot known
  issues"*. It finds the triggers it knows: the next day a DPC+ demo failed on his 7800 and *"evil7800
  didn't spot any issues with it"* (AtariAge `topic/183085`; the tool was not fetched — **Cited only, not
  verified**). `scripts/check_traps.py` is the same kind of check, run on `.asm` source for the statically
  decidable traps of `known-traps.md`; it has no 7800 rule.
- **A disassembly that will not reassemble may be the disassembler's options.** mayday (2005) assembled
  a working game, disassembled the binary with `distella -a -p`, and the output no longer assembled correctly; Cybergoth said to use at least `-paf`, and mayday reported that the switch seemed to clear
  it up (AtariAge `topic/78859`; **Cited only, not verified**). `cmd/dissect` runs `distella -a` only when
  given `-distella`, and only on a 2K/4K image, for an annotated listing to read; nothing here reassembles that listing or checks that it rebuilds the same
  bytes.

---

## Compatibility — the cartridge scheme can limit where a ROM runs

An emulator-based console may not run the newer formats. On AtGames' Flashback emulator in 2016, the
homebrews that did not work looked to one poster to be, in many cases, *"those that use
newer developments like DPC+ and the Melody board, things that Stella supports"* — from someone who had
*"only skimmed the compatibility list"*, so an impression, not a count; Thomas Jentzsch's position in the
same thread: *"For me the only reference is the real existing hardware"* (AtariAge `topic/260084`). A
mapper alone can be enough: the Myst port on an E7 cartridge (16K ROM, 2K RAM), 2023, *"due to the mapper
it won't work on a 2600+"* (deater78, AtariAge `topic/338659`). E7 had already split emulators in his 2022
thread: Stella ran it; MAME switched the 1K banks but not the 256-byte RAM, because it ignored
`-cartslot a26_e7` and detected the scheme itself, and it worked once E7's identifying bytes were put in the
image; javatari ran for a while and then corrupted memory (AtariAge `topic/340351`). **Cited only, not
verified.** `internal/oracle` starts MAME as `a2600 -cart <rom>` with no slot option (read from the code); our
reading is that on a bank-switched image `cmd/oraclevote`'s MAME vote runs whatever scheme MAME detects
(Not verified). For the
2600+ as an emulator, see the "Emulators can agree with each other" row of `known-traps.md`.

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
