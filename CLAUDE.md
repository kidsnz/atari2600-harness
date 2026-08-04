# CLAUDE.md — atari2600-harness

This file is **the only always-on context, auto-loaded in full every session**. Put only "invariant
premises, settled decisions, constants you must never get wrong, and which doc to read for which task" here.
Deep dives go in `docs/` (routing table below). Assume anything not here is unread. Don't put facts that
must always hold *only* in a doc — burn them here or into memory.

> **Language policy:** docs are **English-only**. The author works/communicates in Japanese, but the former
> per-doc `*.ja.md` copies were **dropped (2026-06-17)** to avoid duplicate, drift-prone files — the English
> `*.md` is the single source of truth. (A few `.md` bodies still contain Japanese; full English-ization is
> tracked as **DOC-EN** in `docs/capability-gap-audit.md`.)
> **Talk to the user in Japanese.** The English-only policy is about *repo artifacts* (docs/code/CHANGELOG) —
> it does **not** apply to conversation. The user works in Japanese; answer in Japanese. Don't conflate
> "the repo is English" with "reply in English" (drift caught 2026-06-17).

## Invariant premises
- Goal: build a **verification harness** so Claude can author the Atari 2600 in 6502 assembly accurately
  (not a game-generation app).
- **The primary author is Claude.** The user doesn't read assembly. The environment optimizes Claude's
  authoring-loop precision and speed.
- **Top priority is gap B (timing).** Every past-Pong abandonment died on "unverified timing / positioning."

## Iron rules (follow every time)
1. **Judgment is numeric; screenshots are a supplement.** The final horizontal verdict is the TIA register
   value; vertical is the integer scanline. Don't decide by eyeballing pixel counts. When investigating any
   timing/rendering/behaviour claim, follow the **verification standard** (memory `feedback-verification-standard`)
   — its MAX checklist: trace frame-by-frame, read the full object window (no partial reads), cross-check derived
   formulas against raw pixels, kill each hypothesis with data, prove the negative, present the measured table.
2. **Get cycles from the simulator** (Gopher2600 / sim65). Don't trust the DASM listing or mental arithmetic.
3. **Small steps.** edit → assemble → run → numeric check → commit. Revert to the previous step on failure.
   No bulk changes.
4. **litmus test:** place a sprite at an arbitrary X / move it 1px and have it match `X = 3N − 55`. If this
   passes, the environment is real.
5. **Use every bit of the artifact you can decode YOURSELF; never read someone else's answer.**
   Decoding a binary is a capability, and it applies to your own kernels as much as to anyone's — the
   vendored `Gopher2600/disassembly` package (`FromCartridge` / `bless` flow-following code-vs-data /
   `jmpTargets` / `EntryLevel` = Decoded/Blessed/**Executed**), `cmd/dissect`, DiStella's machine output and
   `internal/cyclebound`'s CFG are all fair game and under-used. What is off limits is *someone else's
   interpretation*: a published labelled/commented `.asm`, a third-party RAM map, or treating one of our own
   builds as ground truth. The line is **whether interpretation is baked in** — raw bytes carry none
   (`A9 48` is just `LDA #$48`), and deriving meaning from them is the skill being built. A rule about not
   reading published disassemblies was once over-generalised into avoiding decoding altogether, which left
   the engine's own disassembler unused for months; do not repeat that. Full statement: memory
   `feedback-goal-standard`.
6. **Author by the protocol.** When building a ROM/kernel, follow `docs/authoring-protocol.md` — the 6-step
   loop (retrieve→plan→author→preflight→verify→**feedback**). It is the single entry that activates the whole
   knowledge base in order (cookbook → design-principles/`pkg/design` checks → known-traps/`check_traps` →
   nearest technique → verify). Don't reach for techniques you haven't budgeted; reject unworkable layouts on
   paper. The feedback step feeds every failure back into the rules/checks so the next build is better.

## Settled architecture
- Engine = **Gopher2600** (Go) **embedded in-process as a library**, wrapped by a thin **Go MCP** (official
  `modelcontextprotocol/go-sdk` v1.6.1, stdio). `hardware`/`television`/`setup` are pure Go (no SDL), so
  headless numeric driving works. terminal/PushedFunction turned out unnecessary (settled v0.3.0).
- Every tool returns results as **numbers (typed JSON, with Coords)**. The image (`get_screen_annotated`) is
  a special case = the annotated screenshot below.
- Regression = **Gopher2600's `regress` + record/replay**. Pure-6502 cycles = sim65 / 6502profiler.
- Reference oracle = **Stella** (`-sssingle -ss1x`, `-tia.dbgcolors roygbp`). Debugger commands are delivered
  by writing `~/Library/Application Support/Stella/autoexec.script` and entering the debugger, **not** by a
  command-line flag: `-dbg.script` does not exist in Stella 7.0 (`-help` has no such option) and never did any
  work here. `dump` covers RAM; the **write-only TIA registers** are reachable only through the debugger's
  `tia` command, which needs the GUI — hence the captured sessions in `internal/oracle/testdata/stella_tia/`.
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
  **`read_audio_trace`** (v1.104.0, the read_motion of sound: traces AUDC/AUDF/AUDV per-frame over N frames = a whole sound envelope in one call instead of hand-stepping read_audio) /
  **`read_ram_trace`** (v1.105.0, the read_motion of arbitrary game state: traces up to 16 RAM addresses ($80-$FF) per-frame over N frames = measure how a tank's X/Y, an AI mode/timer, or a score evolves as numbers — frames-to-escape, decay curves, stuck oscillation — in one call instead of a manual step_frame+peek loop) /
  **`read_bank`** (v0.43.0, current cartridge bank at PC + is_ram; **F8/F6/F4 verified** (litmus_bank, _f6, _f4), scenario fields `bank.number`/`bank.is_ram`) /
  **`analyze_image`** (v1.12.0+, screenshot→TIA data; multi-frame `paths[]` = static/dynamic separation + union tracks + flicker; `docs/ingest.md`) /
  **`analyze_screen`** (v1.19.0, ingest on the current emulator frame) / **`run_scenario`** (v1.19.0, regression verdicts live) /
  **`watch_ram`** (v1.20.0, RAM-change trap with writing PC) /
  **`read_motion`** (v1.79.0, VV-4: object motion-smoothness / jerk_rms over N frames = judder/ブルブル as a number) /
  **`prove_line_budget`** (v1.80.0, VV-2: STATIC per-scanline budget PROVER over ALL paths = the ∀ sibling of `assert_line_budget`; `cmd/cyclebound`+`internal/cyclebound`) /
  **`beamtrace`** (v1.102.0, AT-2: write→visible-pixel timeline = per scanline, each TIA write's beam clock + the visible span it governs) /
  **`beam_race`** (v1.102.0, AT-3: advisory object-graphics-vs-beam map, factual/no-verdict; paired with scenario `checks.no_beam_race`) /
  **`spritepos`** (v1.102.0, AT-4: forward sprite-position solver = target X → SetXPos input + decomposition + snippet + emulator-verified achieved X) /
  **`save_state`** / **`restore_state`** (v1.107.0, whole-machine snapshot into a named, reusable slot — CPU/RAM/TIA/RIOT/cart/TV **plus the rendered framebuffer and the cycle counters**; branch-search from one position instead of replaying from load_rom. ~3.9 KB per snapshot. Does NOT rewind the append-only recorders: video/audio digests, coverage, audio capture) /
  **`defuse`** (v1.114.0, SD-1: which instruction writes which address over ALL paths — the ∀ sibling of watch_ram/read_ram_trace. Per WSYNC region: each address's writer/reader PCs with source locations, plus the whole-program may-write set; targets resolved through the EFFECTIVE address so an indexed store is attributed to the register it reaches and a PHA lands wherever SP points. Also reports UNINITIALISED READS (RAM no path from reset definitely wrote = power-on rubbish on hardware, a defined value in the emulator). Soundness graded against the machine over the WHOLE ROM corpus (128 images, not just the 31 technique kernels): **32655/32655** observed (pc,addr) pairs inside their predicted sets. Declines a bank-switched image) /
  **`beam_intervals`** (v1.114.0, SD-2: PROVES where each TIA write lands on the scanline over ALL paths — the ∀ sibling of `beamtrace`. Earliest/latest beam clock per write in read_row coordinates, `exact` when the position is path-independent, `crosses_line` when even the scanline depends on the path. A wide window IS the finding. **19143/19143** observed writes inside their proven window, graded over the whole corpus (was 7117 on the technique kernels alone); 106 of 327 exact; mean window 8.7 colour clocks. Nothing else in the 2600 ecosystem computes this) /
  **`probe_ram_semantics`** (v1.107.0, "what is $XX?" for a ROM with no source: poke each RAM byte with each probe value, diff the frame against the un-poked baseline, classify from how the changed-region centroid travels = x_position / y_position / appearance / none. Non-destructive. Default frames=3 — at 1 a byte that reaches the screen via a multi-frame conversion, e.g. a BCD score, reads as no-effect. Cross-check its answers against `reference/ale-ram-maps/`) /
  **`visual_ceiling`** (VC-1: the DENOMINATOR for a picture — the smallest error any kernel could reach for a target frame under three STATED constraint sets: C1 playfield-only, 2 colours/line on the 40x4-clock grid / C2 plus one 8-clock object / C3 no column grid (NOT 2600-achievable; it isolates what the grid costs), plus the flat-colour reference. **Read the DELTAS, not the rungs** — C1->C2 answers "what would one sprite buy here", C1->C3 "what is the grid costing" (measured: the grid costs 7.09 rmse on Barnstorming against 3.13 on Chopper Command). A ceiling is a property of (image, constraint set) and never of an image, so it **grades a PICTURE, not a ROM** — scoring a sprite-drawn game under C1 says nothing about its kernel. The palette is derived from the renderer, never transcribed. **No rung is validated by emitting a cartridge**, so no bound here has been shown to fit 76 cycles: C1 rests on one demonstration, C2 on none, C3 is unreachable by design. `docs/visual-ceiling.md`). `step_clock`/`watch(bus)` parked (docs/mcp-tools.md).
  

## Constants you must never get wrong (source: `docs/resources.md`)
**Frame** — 1 line = 228 color clocks (HBLANK 68 + visible 160) = **76 CPU cycles** (3 clocks/cycle).
NTSC **262** = VSYNC 3 / VBLANK 37 / visible **192** / Overscan 30. PAL · SECAM 312 = 3/45/228/36.
Real games deviate, so don't hardcode "exactly 262" — handle as a range + warning.

**Beam coords (Gopher2600 `GetCoords`, hardware-verified v0.3.0)** — the `Clock` convention is
**HBLANK = −68..−1 / visible = 0..159** (first visible pixel = clock 0). **Same coordinate system** as a
sprite's `ResetPixel`/`HmovedPixel` = directly comparable. `Scanline` is a 0-based integer.

**Horizontal position** — missile/ball `X = 3N − 55`, **player is +1px → `X = 3N − 54`**
(N = CPU cycles from the sync point to the RESPx strobe).
The offset is TIA's ~5-color-clock delay + HBLANK 68. Granularity 3px. Coarse adjust is divide-by-15
(5-cycle loop). **litmus-verified:** slope 3px/CPU-cycle (R²=1.000000), coarse 15px/5cy, 160 wrap.
The formula's **offset constant is kernel-specific** (includes the prologue's cycle count)
→ don't hardcode the absolute N; make the final position verdict by measuring **`read_tia`'s `HmovedPixel`**
(visible 0–159). When HMOVE hasn't fired, it equals `ResetPixel`.
⚠️ **There is no "leftmost X" constant, and this line used to state one.** It read "Leftmost X=2 (player 3)"
and "leftmost X=3" — two sentences after warning that the offset is kernel-specific, which is exactly what a
leftmost position is. Re-measured 2026-07-30 on `litmus_pos` with `cmd/calibrate` and confirmed at the
pixel: sweeping DELAY past the wrap, **a PLAYER draws from clock 2** (`DELAY=12` → `reset_pixel` 2,
`hmoved_pixel` 2, `decompose_row` shows P0 occupying clock 2..9), then 3 at `DELAY=13` and 14. So the stated
player minimum of 3 was wrong for this kernel, and "the leftmost" is a property of the positioning code, not
of the machine. Measure it for the kernel you have. (The missile/ball figure was not re-measured.)

**HMOVE** — upper nibble only, two's complement, **positive = left / negative = right**, range +7 to −8.
Moves only at the HMOVE strobe. HMOVE is **right after WSYNC**. (All 16 nibbles: `$70`=left7 … `$00`=0 …
`$F0`=right1 … `$80`=right8, 1px granularity — re-measured 2026-07-30 and now **machine-locked** by
`TestAllSixteenHmoveNibblesMoveByOnePixelEach`, which checks each nibble at the DRAWN pixel as well as at
the register, since "verified in v0.4.0" is a claim about a version and nothing had held it true since.)
**Do not write HMxx within 24 CPU cycles after HMOVE** (Stella PG; unpredictable motion). HMOVE-after-WSYNC
extends HBLANK by 8 clocks = the left-side 8px blank on HMOVE lines; mid-line HMOVE moves objects RIGHT
~1px/4CLK (Towers TIA_HW_Notes; documented, not yet litmus-verified — see `docs/fundamentals-audit.md`).

**6502 timing/BCD (source: 6502.org; re-measured 2026-07-30)** — **stores never take page-cross penalties**
(STA abs,X always 5, (ind),Y always 6) = kernel store timing is deterministic; reads take +1 on page cross
(LDA abs,X 4→5, (ind),Y 5→6); branches 2 not-taken / 3 taken / 4 taken across a page. All eleven cases are
machine-locked by `TestPageCrossPenaltyRules`. ⚠️ A forward branch from a given PC often **cannot** cross —
from $F802 the largest offset $7F only reaches $F881 — so a crossing case must branch backwards. **NMOS decimal mode: only the C flag is valid** after ADC/SBC; D is unknown at
power-up → `CLD` in init is mandatory. ⚠️ `reference/docs_atari/cycle_counting_guide.html`'s position
math is approximate — never cite it for positions; use our calibrated X(N).

**Collisions (CXxx)** — two latches in each D7/D6, sticky. `BIT CXxx` → `BMI`(D7)/`BVS`(D6).
**CXCLR** = clear all collisions; **HMCLR** = clear the motion registers (a different thing).
All four re-measured 2026-07-30 and machine-locked: the D7/D6 map by `TestDecodeCollisions`, and sticky /
CXCLR-clears / **HMCLR-does-not** by `TestHmclearDoesNotClearCollisions` + `litmus_cxclr` (CXP0FB reads
`$82` after the collision, `$82` still after HMCLR, `$02` after CXCLR).

**playfield (bit order)** — 40 columns left→right, each 4 color clocks wide. **Two
sources (ABB/falukropp) agree**, and all **20** column positions are re-measured at the pixel and
machine-locked by `TestEveryPlayfieldColumnLandsWhereTheTableSays` (`litmus_pf_allcols` lights one column
per band, 20 bands in one frame). The older `litmus_pf` covers columns 0/4/12 only — the leftmost bit of
each register, three of twenty. `PF0` = upper nibble only, col0→D4..col3→D7 / `PF1` =
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

### ① Building a ROM — read in this order (the activation sequence)
| Step | Read |
|---|---|
| **0. START HERE** — the authoring loop (retrieve→plan→author→preflight→verify→feedback) | **`docs/authoring-protocol.md`** |
| 1. pick the recipe — game-type → technique stack + traps + checks + 14-step build order | `docs/cookbook.md` |
| 1b. case studies — how real commercial games solved a situation (situation → technique, evidence-backed by disassemblies) | `docs/casebook.md` |
| 1c. build-to-learn — reusable template for reproducing a real game mechanic-by-mechanic to turn "can read" into "can author" | `docs/build-to-learn.md` |
| 1d. the reproduction LOOP — automated visual (`vismatch`) + behavioural (`behavmatch`) diff of a target ROM vs your build, plus `ramtrace` (what the target's 128 bytes of state DO, and what each byte's next value depends on). Palette-independent object-attribution + PF-table generation + a RAM-equivalence gate. Use from the START of a reproduction, not by-eye | `docs/reproduce-loop.md` |
| 2. the design rules (color budget, layout, positioning ground-truth, the distilled rules) | `docs/design-principles.md` |
| 2b. run the executable feasibility checks (color bands, line budget, multiplex, PF windows, craft) | `pkg/design/` |
| 3. clone the nearest verified technique | `docs/techniques/` (+ catalog `README.md`) |
| 4. pre-flight the kernel against the "emu-passes / HW-fails" traps | `docs/known-traps.md` → `scripts/check_traps.py` |
| 5. verify — scenario format / litmus position+HMOVE data / what each litmus proves | `docs/scenarios.md` · `docs/litmus-results.md` · `docs/verified-coverage.md` |
| 5b. how to *know* it's correct — the testing discipline (oracle problem, invariants, property/metamorphic/fuzz/mutation, claim-level demo) + per-build checklist | `docs/testing-playbook.md` |
| search the whole mined corpus (forum 850 + dev-blogs 117) → principle/function it feeds | `docs/mining-digest.md` |

### ② Reference — look up when needed
| Task | Read |
|---|---|
| Why this design / anatomy of failure | `docs/gap-analysis.md` |
| Tool selection rationale / alternatives | `docs/tool-landscape.md` |
| Implementation spec (Gopher2600 API / MCP / Stella flags) / source of constants | `docs/resources.md` |
| MCP tool implementation spec (go-sdk API, per-tool I/O) | `docs/mcp-tools.md` |
| harness backlog — capability gaps (G1–G14) + verification-variety (VV-*); **the single live backlog** (ex improvement/hardening-roadmap folded in) | `docs/capability-gap-audit.md` |
| 8bitworkshop sample cross-check (book techniques vs our library) | `docs/8bitworkshop-crosscheck.md` |
| the visual CEILING — best the hardware could do for a picture (the denominator `vismatch` lacks) | `docs/visual-ceiling.md` |
| provenance map (every technique/rule → its origin) | `docs/provenance.md` (gen: `check_provenance.py --list`) |
| fundamentals audit (verified vs documented vs unknown, with sources) | `docs/fundamentals-audit.md` |
| Exerciser ROM (integration showcase, 6 scenes; v1.0.0 keystone) | `docs/exerciser.md` |
| Stella oracle cross-check usage | `docs/stella-oracle.md` |
| Image ingestion (screenshot/ROM → TIA data) + input contract v3 | `docs/ingest.md` |
| RAM maps per ROM (auto-extracted audit) | `docs/ram-maps.md` |
| RAM maps for **commercial** ROMs (external answer key for `probe_ram_semantics`; 104 games, ALE-derived) | umbrella `reference/ale-ram-maps/` (local-only, not part of this repo) |
| Decision history and changelog (delivered-work log) | `CHANGELOG.md` |

> **Anti-rot:** every `docs/*.md` must be reachable from this table or the authoring protocol — `scripts/check_wiring.py`
> (CI-gated) fails on orphaned knowledge, so nothing accrued ever rots unused.
> **Anti-vacuity:** every `func TestXxx` must be able to FAIL — assert on `t`, or hand `t` to a helper that does.
> `scripts/check_tests.py` (CI-gated, with a `--selftest`) fails on a test with no failure path. It exists
> because the extreme case of this project's recurring defect — a check that passes while covering nothing —
> is a test that cannot fail at all, and one had been sitting in the suite since 2026-07-29.

## Repository layout (v0.22.0 spinoff, standalone repo)
**This repo = the harness base only (general-purpose, reused across all games).** Game ROM artifacts are
split into a **separate repo**. Dependency is **one-way game → harness** (the harness has zero dependence on
any game; even its tests reference only its own `roms/litmus`).
- Module = `github.com/kidsnz/atari2600-harness`. Gopher2600 via `go.mod` `replace => ./Gopher2600`.
- Physical layout (under the umbrella folder `260609_atari2600-dev/`, three sibling repos bound by `go.work`):
  ```
  260609_atari2600-dev/        ← umbrella (the folder Claude Code opens; .mcp.json/.claude live here)
  ├── go.work                   ← binds harness + roms + sandbox locally
  ├── harness/                  ← this repo (atari2600-harness) = verification engine
  ├── roms/                     ← separate repo (atari2600-roms) = deliverables only (frogger; future pong/pizza-boy)
  └── sandbox/                  ← local-only repo = practice / experiments / studies (skill-building; not pushed)
  ```
- Base contents: `cmd/harness` (MCP server) / `cmd/probe` (plumbing) / `cmd/scenario` (regression runner CLI) /
  `cmd/calibrate` (horizontal X(N) sweep-fit) /
  `cmd/fieldtest` (ROM self-drive + multi-frame analysis; `-inbox` batch, `-auto` start escalation) /
  `cmd/dissect` (runtime trace × ROM matching → asset table addresses + annotated DiStella disassembly) /
  `cmd/jingle` (melody notation → playable ROM) / `cmd/rammap` (RAM usage audit → markdown map) /
  `cmd/cover` (VV-3 PC/branch coverage + one-sided branches) / `cmd/guidedfuzz` (VV-3 coverage-guided AFL-style fuzz) /
  `cmd/statecov` (VV-11 TIA state-coverage matrix: which NUSIZ/size/VDEL/PF-mode/bank the test exercised; `internal/statecov`) /
  `cmd/mutate` (mutation testing → kill rate; `-covered` = VV-11 honest kill rate over executed code only) /
  `cmd/framesim` (VV-12 tolerant frame compare: SSIM + perceptual-hash distance, "how wrong & where"; `internal/framesim`) /
  `cmd/audiospec` (VV-13 frequency-domain audio compare: FFT spectral + RMS-envelope distance, separates inverted twins; `internal/audiospec`) /
  `cmd/cpucert` (VV-14 citable cycle-budget certificate: VV-2 proof + @lines lemmas + provenance/hashes; `cyclebound.Certify`) /
  `cmd/timinglint` (static TIA-timing linter — proactive authoring aid: warns BEFORE running about HMOVE-without-HMxx / non-zero-HMxx-without-HMOVE / HMxx-write-<24cy-after-HMOVE; zero false positives on the known-good technique corpus; `cyclebound.Lint`) /
  `cmd/beamtrace` (write→visible-pixel timeline — authoring aid: per scanline, every TIA write with the beam clock it lands at + the visible-pixel span it governs (until the next write to the same reg); `internal/beamtrace`, `emu.LastTIAWrite`. `-race` = advisory beam-race report (object graphics vs beam, factual/no-verdict; `internal/beamrace`, `emu.ObjectX`) — paired with scenario `checks.no_beam_race` = author-declared "object updated before beam on lines A..B" gate (AT-3; a fully-automatic verdict can't be sound — see audit)) /
  `cmd/spritepos` (forward sprite-position solver — authoring aid: target X 0..159 → routine input + div-15-coarse/HMOVE-fine decomposition + paste-able SetXPos snippet + the position the hardware ACTUALLY reaches (HmovedPixel, emulator-verified — the formula offset is kernel-specific so the answer is measured, not trusted); `internal/spritepos`) /
  `cmd/trajdiff` (VV-8 behavioral trajectory diff vs a reference ROM) /
  `cmd/vismatch` (reproduction loop: PALETTE-INDEPENDENT visual diff of a target ROM vs your build via per-pixel object attribution `emu.DecomposeRow` — per-element band diffs naming exact scanline range + clock-spans, object-attribution overlay PNG, and `-genpf` = auto-GENERATE the correct `CacLTbl/CacRTbl`+`CACTOP/CACBOT` from the target's PF bands; `internal/vismatch`. See `docs/reproduce-loop.md`) /
  `cmd/behavmatch` (reproduction loop: behavioural diff — drives target ROM + your build through identical ROM-AGNOSTIC scripted input scenarios (`internal/behavmatch/scenarios.go`: both players, every direction, tap-vs-hold fire, diagonals, aimed fire, simultaneous fire, a long duel, console switches), records per-frame object trajectories, reports mechanic divergences (speed/clamp) separated from calibration (rest position) + fire→freeze coupling. **`-ram-gate`** = the RAM-equivalence gate: the first frame+address where your build's RAM stops matching the target's. It compares a MASK, never all 128 bytes (two correct implementations legitimately differ in scratch/leftovers), and every verdict prints what was excluded and why; a pass over nothing says VACUOUS. `internal/behavmatch`) /
  `cmd/ramtrace` (reproduction loop: records a target's RAM as a per-frame time series — all 128 bytes plus the held input, the collisions that OCCURRED (accumulated inside the frame, so a game's own CXCLR cannot erase the evidence) and the stack-pointer range — then describes it. `activity` = per-byte statistics that fit nothing; `arity` = the smallest feature set (self + input + companions) that determines each byte's next value, reporting unresolved as unresolved with the LOCATIONS of the contradicting transitions, flagging frame-counter-like bytes that can key anything, and flagging a "resolution" where every key was seen once as MEMORISING. ROM-blind by construction: input is a .bin and an input script, never a symbol map. `internal/ramtrace`) /
  `cmd/framegen` (reproduction loop: FROM-SCRATCH full-frame reproduction — reads a target ROM's per-scanline object attribution (`emu.DecomposeRow`) and emits a NEW self-contained DASM source that replays PF(left/right)+GRP0/GRP1+missiles/ball per scanline, in ONE loop or in per-zone loops with RESxx/HMOVE placement between them when a sprite moves down the frame (RL-8a/RL-8b); self-calibrates sprite X + VBLANK + content vertical-shift + frame length by assembling+rendering its own output. **Reports what it did NOT reproduce** — per-element coverage in the terminal, a `; NOT REPRODUCED:` block in the generated `.asm`, and exit 1 — with the counted reason: copies the 8-block kernel does not order, a slot dropped to stay inside 76 cycles, or a position change with too few background-only lines to pay for the move (one scanline per object placed + 1 HMOVE line + 1 replayed blank line). Field-measured: **pixel-exact on 22/31 technique ROMs + Outlaw + Combat, 262 scanlines on 35/35 runs**; partial on Fishing Derby (96.9% overall match while P0 is 11% correct — background is 77% of the area and buries the number, so read the per-element table). See `docs/reproduce-loop.md`, audit RL-7/RL-8) /
  `cmd/oraclevote` (VV-6 N-oracle majority RAM vote: Gopher2600 + MAME headless; `internal/oracle`) /
  `cmd/cpucheck` (VV-7 silicon CPU differential: Gopher2600 CPU vs perfect6502 netlist per-instruction; `internal/cpudiff`; needs `bin/p6502step` via `scripts/install_perfect6502.sh`) /
  `cmd/stellacheck` (Stella oracle: RAM + pixel compare; hands-free via scripts/stella_oracle.sh) / `internal/emu` (driving) / `internal/annotate` (annotation) /
  `internal/scenario` (scenario regression = input timeline + numeric assertions, ROM-agnostic) /
  `internal/calibrate` (position calibration = poke sweep + linear regression) /
  **`pkg/playfield`** (public encoder `EncodeSymmetric` etc. = universal Atari 2600 knowledge; the roms-side `gen` imports it).
- Verification ROMs: `roms/litmus/` (litmus_* / smoke / golden) = **the base's own property**, kept in this repo.
- Game deliverables (separate repo `atari2600-roms`): `<game>/` (`*.asm`/`*.bin`) + `<game>/gen/` (scene
  definitions + kernel generation, importing `atari2600-harness/pkg/playfield`) + `<game>/scenarios/*.json`.
  Example: `frogger/` (Monet Frogger). **roms = deliverables only.**
- Skill-building (local-only repo `sandbox/`, not pushed): `practice/` (practice ROMs), `experiments/`
  (throwaway spikes: frogger-spikes/, monet-frogger/), `studies/` (commercial-game reconstructions), plus
  `ROADMAP.md`/`EVALUATION.md`. Same `go.work`/`replace` wiring as roms; run scenarios from `sandbox/`
  (rom paths like `practice/<game>/...`).
- Add new games (deliverables) under the roms repo as `<name>/` (+`gen/`). Promote kernels you want to generalize to `pkg/`
  (like the encoder; YAGNI).

## Development environment (macOS / Apple Silicon)
`brew install dasm cc65 pkg-config go` / Stella: `brew install --cask stella` / MAME (VV-6 cross-oracle, optional): `brew install mame`.
Clone Gopher2600 into the **harness/** root (untracked, referenced via `go.mod` `replace`).
**Run commands from each repo's root** (harness's own from `harness/`, ROMs from `roms/`). `go.work` assumed.
- ROM build: `dasm x.asm -f3 -ox.bin`.
- Plumbing check (harness/): `go run ./cmd/probe`. MCP server: `go build -o bin/harness ./cmd/harness`.
- litmus regression (harness/): `go run ./cmd/scenario roms/litmus/scenarios/*.json` (exit 0 on all pass).
  **★2026-07-30: this command is no longer the gate, because it never was the whole one.** Measured: 95
  scenario files exist — 57 under `roms/litmus`, 31 under `roms/techniques`, 7 under `roms/exerciser` — and
  this line named 57 of them. The other 38 (40%) were written and never run by anything. All of them pass,
  so nothing was hiding; nothing would have noticed if they stopped. `internal/scenario.TestEveryScenarioRuns`
  now walks `roms/**/scenarios/*.json` and runs all 95 inside `go test ./...`, discovering the directories
  rather than listing them, so a fourth one is covered without editing a command line. ~19s.
- Calibration (harness/): `go run ./cmd/calibrate` (sweeps litmus_pos → reproduces slope 3 px/CPU-cycle).
- ROM generation (roms/): `go run ./<game>/gen [scene]`.
- ROM regression (roms/): `go run github.com/kidsnz/atari2600-harness/cmd/scenario <game>/scenarios/*.json`.

## Version control
For each meaningful change, append to `CHANGELOG.md` (Keep a Changelog) and tag with SemVer. Record decisions
in the CHANGELOG's "Decisions" section. **When tagging, also bump `Version:` in cmd/harness/main.go**
(serverInfo) — it has drifted twice; treat it as part of the release checklist.

**Before pushing, mirror CI — don't trust a plain local build+test.** A local `go test` runs under the
umbrella `go.work` and effectively serially, which HIDES what CI hits. Run CI's actual invocation:
`GOWORK=off CGO_ENABLED=0 go vet ./... && go test -p 1 ./...` (CI uses `-p 1`. The race that made that
necessary is FIXED — `build.Assemble` writes to a per-process name and renames, so two test packages
assembling the same kernel can no longer read a half-written ROM — but `-p 1` is kept as the CI invocation
because it is what CI actually runs, and mirroring CI is the point), then the scenario + `check_*` steps. **Lesson (2026-06-18):** a docs-only push
went CI-red on a flaky parallel-test file race that local build+test never showed — CI is the gate, so verify
against it, not a proxy, before every push. A tracked **pre-push hook** automates this: `scripts/git-hooks/pre-push`
runs the mirror (vet + `test -p 1` + the fast `check_*`) and blocks a red push — enable once per clone with
`git config core.hooksPath scripts/git-hooks` (emergency bypass: `git push --no-verify`).
