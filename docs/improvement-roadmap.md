# Improvement roadmap — next moves to raise authoring accuracy

The verification harness has matured, and Monet Frogger is the **most accurate** build so far. This
document is a **prioritized research roadmap** for "making it even more accurate." Implementation happens in
separate sessions. Each item lists **where to touch (real files, verified Gopher2600 API symbols)** so work
can start without guesswork.

> **Status (v0.21.0):** the base-layer P0–P3 of this roadmap (the implementation of gaps A–E) are **all
> closed**. The remaining unstarted items are mainly the game-side R-1 (Freeway port) / R-2 (audio recipes)
> and extensions like `step_clock` (color-clock granularity).
>
> **Next phase → see [`hardening-roadmap.md`](hardening-roadmap.md)** for strengthening the harness beyond
> gap-closing: deepening authoring + verification into thin domains (sprites incl. the P0+P1 16px technique,
> audio) and hardening the foundation (CI, trust, completing stub tools).

## Open backlog (2026-06-12, post-ingest — the durable TODO ledger)

### Image ingestion (v1.9–1.18 shipped; field-testing continues)
- [ ] **Field-test across game styles** (user-requested): vertical scrollers (River Raid), mazes
  (Pac-Man = real flicker), wide-sprite games (Demon Attack), score-mode users, PAL titles.
  Record weaknesses here and fix.
- [ ] Union tracking by position continuity (an animating+moving object splits into per-pose
  union entries all flagged "flicker" — Pitfall Harry; distinguish true same-position flicker
  from animation+motion).
- [ ] Static-layer reconstruction residual (Pitfall 98.5% vs Pizza Boy 100%) — canopy-fringe
  modelling; consider PAL/Stella palette as a second quantizer table if avg_palette_dist
  ever hurts.
- [ ] Animated playfield (scrolling starfields) lands in the dynamic layer as objects —
  semantic noise; possible "PF-shaped dynamic rows" classifier.

### Techniques — documented refinements (build when a game needs them)
- [ ] VDEL odd/even trick: 1-px vertical granularity inside a 2-line kernel (#4 note).
- [ ] Full dynamic multi-sprite kernel: per-frame Y-sort + 2-of-N allocation + fairness rotation
  (#10's general form; the flicker-pairs core is verified).
- [ ] DCP skipDraw variant for cycle-starved kernels (#3 note).
- [ ] F6/F4/larger bankswitch schemes (#11 verified F8 only).
- [ ] Mid-line HMOVE (+1px/4CLK rightward) — documented in fundamentals-audit, not litmus-verified.
- [ ] Real music composition with the author by ear (Slocum note tables; #6's joint session).

### Harness (older parked items)
- [ ] Stella oracle v2: sub-frame alignment, TIA/pixel-level compare, keystroke automation.
- [ ] `step_clock` (color-clock stepping), `watch`/`trap` (budget case mostly covered by
  assert_line_budget).
- [ ] `run_scenario` as an MCP tool; `read_sprite_shape`.
- [ ] V2-18 RAM-map audit (low).

### The main arc
- [ ] **Real game authoring** — the techniques catalog (12/12) + ingest pipeline exist to serve
  this. Next session can start from "what do we build?".

## Central observation — position is closed, the timing budget is open

The declared top priority, **gap B (timing), remained the biggest hole in the real authoring loop**. The
*position* litmus is fully closed (`read_tia`'s `HmovedPixel` places any X numerically, see
`docs/litmus-results.md`). But verifying the *timing budget* was unstarted = **there was no numeric way to
ask "did this kernel region fit in 76 cycles?"** This connects directly to the one unclosed failure mode
that silently killed Pong v2 (per-scanline cycle overrun → screen roll, undetectable). Rule 2 "get cycles
from the simulator" was used in litmus but **not yet wired into the real authoring loop**.

### Gap taxonomy — current status (see `docs/gap-analysis.md`)

| Gap | Content | Status | Remaining |
|---|---|---|---|
| A perception | results invisible | **closed** | `read_tia_registers`/`read_collisions` (P1, v0.14.0) measure write-only registers and collisions. Color inference dropped |
| B timing | cycle math doesn't add up | **closed** | position & cycle exposure (B-1, v0.12), budget guard (B-3, v0.13), intra-frame granularity (B-2, v0.15), automatic kernel-constant calibration (B-4, v0.20) all implemented |
| C knowledge | 6502/TIA detail errors | **closed** | kernel-dependent constants (missile formula N, HBLANK boundary) not yet formalized |
| D verification | no reproduction/regression | **closed** | scenario runner (D-1 assertions v0.18 + D-2 input replay v0.18 + D-3 golden-frame regression v0.19) implemented |
| E friction | edit→run→inspect is multi-step | **closed** | `assemble_and_load` (P3, v0.16) + scenario regression (P2, v0.18-19) + `.asm` directly as scenario `rom` (v0.21) make "one source → verdict" one command |

---

## P0 — gap B: wire timing verification into the real authoring loop (top priority, highest impact)

### B-1. `read_cycles` tool (cycle exposure)  ✅ done v0.12.0 (bugfix v0.12.1)
- **Problem:** `step_frame` returns no cycles. The model can't know the kernel's weight numerically.
- **Proposal:** an MCP tool that returns the CPU cycles of the most recent instruction / interval.
- **Where to touch:** Gopher2600 **already tracks** `mc.LastResult.Cycles` (`LastResult execution.Result` at
  `Gopher2600/hardware/cpu/cpu.go:52`, fields around `cpu/instructions/definitions.go:31`). Add a method to
  `internal/emu/emu.go` reading `e.VCS.CPU.LastResult.Cycles` and expose it in `cmd/harness/main.go`.
- **Verification:** does the accumulation match on a known-cycle instruction-stream ROM (e.g. `LDA#`=2cy ×N)?
  `cpu/cpu_test.go:554` is precedent.
- **Size:** small. The minimal addition that first embodies rule 2 in the real loop.

### B-2. `step_scanline` / `step_clock` tools (intra-frame granularity)  ✅ step_scanline + step_instruction done v0.15.0 (step_clock at color-clock granularity unstarted)
- **Problem:** currently only frame-granular stepping. Can't peek the kernel's mid-state.
- **Proposal:** advance by scanline+1 / color-clock+1. Already noted as "unimplemented (planned)" in CLAUDE.md/CHANGELOG.
- **Where to touch:** build a thin wrapper that stops at Coords +1 on top of the existing
  `RunUntilBeam(maxFrames, scanline, clock)` (`emu.go:197`) and low-level `e.VCS.Step(nil)` (used by
  `StepFrame`, `emu.go:102`).
- **Verification:** after stepping, `Coords()` matches the expected scanline/clock.
- **Size:** small–medium.

### B-3. per-scanline cycle budget guard (the crux — directly detects the failure that killed Pong v2)  ✅ done v0.13.0 (`assert_line_budget`)
- **Problem:** when a visible line exceeds 76 CPU cycles, **the screen silently breaks** with no way to detect it.
- **Proposal:** measure cycles between WSYNCs (= one line) and **halt if > 76, reporting the scanline**.
  Extend `breakif` from a "beam-position condition" to a "budget-overrun condition" (the core use case of the
  unimplemented `watch|trap`).
- **Where to touch:** in `emu.go`'s step loop, monitor accumulated `LastResult.Cycles` between WSYNC strobes.
  Use TV scanline transitions (`GetCoords().Scanline` change) as the boundary.
- **Verification:** halt fires on a ROM with a deliberately heavy kernel line (cycle over), and does not fire on a normal ROM.
- **Size:** medium. **Highest impact** — continuously guarantees "is the timing right?" numerically during authoring.

### B-4. automatic calibration of kernel-dependent constants (connects to gap C)  ✅ done v0.20.0 (`cmd/calibrate` / `internal/calibrate`; reproduces slope 3 px/cyc, offset −18 on litmus_pos)
- **Problem:** in missile/ball's `X = 3N − 55`, the **absolute N is kernel-specific**, and DELAY 0–2 is
  nonlinear at the HBLANK boundary (an open point noted in `docs/litmus-results.md`). read_tia measurement is needed each time.
- **Proposal:** a helper that sweeps the current kernel's RESPx timing, auto-fits `X(N)`, and returns that kernel's offset constant.
- **Size:** medium. Turns litmus from "a one-off manual job" into "reproducible per kernel".

---

## P1 — TIA shadow-register reads (the rest of gap A)

### `read_tia_registers` (measure current values of write-only registers)  ✅ done v0.14.0
- **Problem:** COLUP0/1, COLUPF, COLUBK, NUSIZ0/1, CTRLPF, PF0/1/2, REFP, HMxx are **write-only** and `poke`
  doesn't persist (CLAUDE.md "poke quirk"). The model can only **infer from the color** in `read_row` =
  it can only indirectly confirm "did `sta COLUP0` actually take effect?".
- **Proposal:** **directly return** the current values Gopher2600 holds internally. Replace color inference with measurement.
- **Where to touch (verified symbols):** all present under `e.VCS.TIA.Video` (already accessed at `emu.go:60`):
  - Player: `Player0.Color` / `.Nusiz` / `.SizeAndCopies` / `.Reflected` / `.HmovedPixel`
    (`Gopher2600/hardware/tia/video/player.go:96-108`).
  - Playfield: `Playfield.PF0/.PF1/.PF2` / `.ForegroundColor` / `.BackgroundColor` / `.Reflected` /
    `.Priority` / `.Scoremode` (`tia/video/playfield.go:44-81`).
  - The Video struct has `Playfield *Playfield` / `Player0/1` / `Missile0/1` / `Ball` (`tia/video/video.go:78-85`).
- **Verification:** ROM does `sta COLUP0,#$1C` → `read_tia_registers` returns `0x1C`. Cross-check against `read_row` color.
- **Size:** small–medium. **Takes gap A to "zero inference."**

### `read_collisions` (structuring CXxx)  ✅ done v0.14.0
- **Problem:** Frogger's `OnPad` check reads CXPPMM via **raw peek ($30–$37)**.
- **Proposal:** return collision latches as structured JSON (pair booleans like P0-P1 / M0-P0).
- **Where to touch:** the `Collisions` struct at `Gopher2600/hardware/tia/video/collisions.go:25` / `chipbus.CXPPMM` etc.
- **Size:** small. Same family as P1; natural to implement together.

---

## P2 — gap D: verification automation (replacing manual MCP spamming)

### D-1. assertion spec file + runner  ✅ done v0.18.0 (`cmd/scenario` / `internal/scenario`, `docs/scenarios.md`)
- **Problem:** all checks are manual MCP calls. Regression depends on humans.
- **Proposal:** per-ROM declaration files (JSON) describing `scanline == 262` / `P0.HmovedPixel <= 159` /
  `FrogY ∈ [24,160]` etc. → auto-run, pass/fail report.
- **Where to touch:** extend the self-verification pattern of the existing `playfield_test.go` to the **ROM level**.
- **Size:** medium.

### D-2. input replay timeline  ✅ done v0.18.0 (scenario `inputs[]`; demonstrated by frogger hop)
- **Problem:** `set_input` (`emu.go:129`) is a **single event**. Scenarios can't be reproduced.
- **Proposal:** stream scripts like "up at frame N, fire at N+30" to turn hop→drown→win into reproducible tests.
- **Size:** medium.

### D-3. golden-frame regression  ✅ done v0.19.0 (`digest.Video` wiring, `checks.golden_frame`, `cmd/scenario -update`)
- **Proposal:** wire Gopher2600's `regress` + record/replay (frame hashes).
- **Unconfirmed:** `regress`'s CLI syntax (also flagged unconfirmed in `docs/resources.md`) → verify when starting.
- **Size:** medium–large.

---

## P3 — gap E: build-loop shortening  ✅ closed (v0.16 assemble_and_load + v0.18-19 scenario regression + v0.21 `.asm` directly as scenario rom = one source → verdict in one command)

### `assemble_and_load` (collapse the multi-step into one shot)  ✅ done v0.16.0
- **Problem:** the multi-step `edit asmgen` → `go run ./<game>/gen` → `dasm -f3` → `load_rom`. Friction cuts iteration speed.
- **Proposal:** take an asm path, run `dasm` via `os/exec` → **surface errors structured at the tool surface**
  (highlight the failing line) → load immediately on success. Self-contained in `cmd/harness`.
- **Size:** small. Minimizes the round-trip of iteration.

---

## Other angles (medium-term, watch)

- **`asmgen.go` monolith:** the five `Generate*ASM` (symmetric/asymmetric/sprite/full/frogger) hold kernel
  boilerplate redundantly. Move toward reusable kernel fragments/macros.
  → natural to tidy this together with `[[project-harness-spinoff-todo]]` (making the base a standalone repo).
- **Annotated-screenshot extensions:** currently only sprite-X markers. Overlaying **collision state,
  RAM-derived object positions (FrogY etc.), and a per-scanline cycle-budget "ruler"** would deepen the
  primary user↔model channel (a first-class citizen). Also a cross-check of Gopher2600 annotation pixels vs
  the Stella oracle (a weakness noted in `docs/gap-analysis.md`).
- **Audio (AUDC/AUDF/AUDV) entirely missing:** zero verification path. But the references (Slocum's guide + a
  working driver) are in hand **→ promoted from "out of scope" to "actionable" in R-2 below.**
- **Formalizing gap C:** add the HBLANK boundary (DELAY 0–2 nonlinearity) and the kernel-dependent N of the
  missile formula to `docs/litmus-results.md`.

---

## Untapped veins in the reference material (docs_atari — cataloged but unused at the Frogger stage)

Source: the past Pong project's reference trove `docs_atari/`. **These are not new discoveries** — they are
already cataloged in `docs/tool-landscape.md` (Bensema's cycle guide = "★ the core of B/C", `sethorizpos.asm`,
`game_disassembly/` "21 titles", `za2600/`, various samples). `docs/gap-analysis.md` also states "the owner
already has nearly all references." **The value is not new files but mining the "untapped veins" against the
Frogger stage.** They informed the design (litmus, positioning) but the following are not yet live in the
current north star.

### R-1. Freeway architecture port (top priority, directly tied to authoring)
- **Problem:** the current Frogger **reinvents** lanes / multiple objects / collisions. Freeway is the closest
  reference — a **proven commercial design** for the same genre (a crossing game) that ports over directly.
- **Material:** `game_disassembly/freeway.asm` (1683 lines). Confirmed core structures:
  `LaneNumber` / `CarMotionTimers[10]` / `CarMotions[10]` / `CarXCoords[10]` /
  `Chick0LaneCollide`·`Chick1LaneCollide` (**per-lane collision**) / `CarShapePtr` / `ZCarPatterns[10]` (car rows via NUSIZ).
- **Mining points:** ① a kernel that loops over per-lane object arrays in one pass ② per-object motion timers
  (speed differences) ③ per-lane collision via Y→lane inversion + X overlap (a check independent of CXxx)
  ④ "car rows / lily rows" via NUSIZ multiple copies.
- **Effect:** back the Frogger state machine (ride/drown/win) and lane motion with a real game's design.
- **Note:** learn only the **structure** of the disassembly (don't copy code = port ideas clean-room).

### R-2. audio recipes (promoted from "out of scope" to "actionable")  🔶 verification path `read_audio` done v0.17.0 (the audio "authoring" side = sound effects is still to come)
- **So far:** the roadmap marked audio "out of scope." But the materials are in hand.
- **Material:** `2600_music_guide.txt` (Paul Slocum: meaning of AUDC/AUDF/AUDV, the 8 common timbres =
  Saw / Engine / Square / Bass / Pitfall / Noise / Lead / Buzz, drum settings, pitch table),
  `za2600/audio.asm` (**a working driver**: `Tone`/`Freq`/`Vol`, seq notes/durs, `songCur`),
  8bitworkshop `musicplayer` / `wavetable` / `fracpitch`.
- **Mining points:** start from minimal sound effects (hop = short Square, drown = descending Noise, win =
  rising arpeggio). Hit AUDV/AUDF/AUDC in the spare cycles of VBLANK / Overscan.
- **Harness implication (important):** audio has **no numeric verification path** (`read_tia` is video-only).
  → bundle a **TIA Audio shadow read (current AUDC/AUDF/AUDV)** into P1's `read_tia_registers`.
  Confirmed: Gopher2600 has `TIA.Audio *audio.Audio` (`Gopher2600/hardware/tia/tia.go:75`), and each channel's
  `registers.Control / .Freq / .Volume` (`tia/audio/channels.go`, `tia/audio/registers.go`) correspond to
  AUDC/AUDF/AUDV. But `channel0/1` are unexported, so `read_audio` needs a small accessor added.
- **Effect:** add sound effects to Frogger and extend the principle (rule 1) "verify with numbers" into audio.

### R-3. distilling a cycle-cost table (reinforces the "writing" side of P0)
- **Complementarity:** P0 **measures** with Gopher2600. But being able to **predict** before writing cuts
  round-trips. The two are a dual wield.
- **Material:** `cycle_counting_guide.html` (Nick Bensema / Random Terrain). Per-category cycles
  (Fast math / Storage / Weenie / Slow math / Stack / Branch), `X=(CYCLES−20)×3`, the DEY-BNE loop,
  `(indirect),Y` page-cross +1, branch taken +1 / page-cross +2.
- **Mining point:** distill a quick cycle table of common instructions into CLAUDE.md or a doc, to consult at
  the moment of writing a kernel.
- **Effect:** keeps rule 2 "get cycles from the simulator" while raising first-draft timing accuracy and cutting round-trips.

### R-4. real-game structure in general (prone to sprawl, so keep it thin = an "index")
- **Material:** `spiceware_tutorial/` (completes Collect in 14 steps), `nanochess_samples/` (Programming Games
  book), `game_disassembly/` (Pitfall / Kaboom / RiverRaid / Adventure etc.), `8bitworkshop_samples/`
  (multisprite, complexscene, fullgame = LFSR random, score6/BCD, collisions).
- **Mining points (an index to pull when needed):** 6-digit score/BCD, LFSR random, multi-sprite multiplexing,
  title/text display (`za2600/text24.asm`, `*.chr` fonts).
- **Caution:** **high sprawl risk. Only pull the relevant file when the north star (Frogger) needs it**; do
  nothing beyond indexing.

---

## External research — ideas to evolve further (GitHub/web, 2026-06)

Results of a GitHub/web survey. **Two biggest findings:**
(1) the **Gopher2600 we embed already implements the hardest roadmap items as libraries**. Without breaking
the decision to embed `hardware.VCS` directly and drop the debugger driver (v0.3.0, deterministic/simple and
correct), the packages "below" it can be used standalone = many P2/R items become **"wiring"** rather than
**"building"**.
(2) other systems have several emulator-MCPs (C64 = vice-mcp, GB = mcp-gameboy, Atari Lynx = gearlynx, etc.),
but **an Atari 2600 one was not found in a public search (GitHub/web, 2026-06).** ※ Since "absence" can't be
proven, we don't claim "we're first" = **no known prior art**. Pointers welcome if any exist.

### G-1. "promote" Gopher2600's unused packages (top priority, most grounded, highest impact)
With the debugger driver still dropped, use these libraries standalone (exported APIs verified in real code):

| package | exported API (verified) | roadmap item it fills |
|---|---|---|
| `recorder` | `NewRecorder(transcript, *hardware.VCS)` / `NewPlayback(transcript)` | **D-2 input replay** |
| `regression` | `RegressAdd` / `RegressRun` (video-hash + Playback + Log, 3 test kinds) | **D-3 golden regression** (noted in CLAUDE.md, unwired) |
| `tracker` | `Entry`/`Distortion`/`MusicalNote`/`NoteToPianoKey` (`audio.Tracker` impl) | **R-2 audio verification** — convert AUDx to **note/timbre names** |
| `reflection` | `NewReflector(*hardware.VCS)` (per-video-step element attribution + `Hmove` comb) | **annotation extension** (which object drew what) |
| `digest` | `NewVideo(tv)` / `NewAudio(tv)` | frame/audio hashes = golden foundation |
| `rewind` | `PokeHook` (**deeppoke**) / `ComparisonState` | **resolves CLAUDE.md "poke quirk"** (persistent poke) + state diff |

- **Honest limit:** `debugger/halt_watches|traps|breakpoints` also exist, but their **types are unexported**
  and coupled to the debugger loop → `watch|trap` (P0 B-3) stays a **pattern reference** (not a drop-in like recorder).
- **Wiring cost differences:** `recorder`/`digest`/`regression` are nearly drop-in. `tracker`/`reflection`
  need wiring like per-video-cycle stepping or FrameTrigger registration.
- **License:** Gopher2600 = **GPL-3.0** (already vendored via `go.mod` `replace`, operated under the same terms). Mind the usage form.
- **Effect:** P2 (D-2/D-3) and R-2 shrink from "implementation" to "wiring an existing library" = regression,
  input replay, and audio verification land at minimal cost.

### G-2. borrowing from the C64 MCP ecosystem + positioning
- **Finding:** the C64 has several emulator-MCPs ——
  [`barryw/vice-mcp`](https://github.com/barryw/vice-mcp) (~17k lines of C embedded directly in VICE;
  breakpoint/sprite/**SID register reads**/screenshot/step via JSON-RPC),
  [`chrisgleissner/c64bridge`](https://github.com/chrisgleissner/c64bridge),
  [`axewater/mcp-vice-emu`](https://github.com/axewater/mcp-vice-emu),
  [`cliffhall/mcp-c64`](https://github.com/cliffhall/mcp-c64), and others.
  **An Atari 2600 one was not found in a public search = this looks like open ground (not a claim of being first).**
- **Borrowing ideas:**
  ① vice-mcp's "**SID register reads**" = backing for our TIA audio shadow read (R-2 / G-1 `tracker`) (reading sound from registers is standard).
  ② [`barryw/sim6502`](https://github.com/barryw/sim6502)'s **pluggable-backend test DSL** (switch fast pure-CPU
  and cycle-accurate via the same DSL) → make P2 a **two-layer "sim65 = fast pure CPU" + "Gopher2600 =
  cycle/TIA accurate"** (consistent with CLAUDE.md "pure 6502 = sim65").
  ③ the [LLM→6502 pipeline](https://hackaday.com/2024/11/07/using-ai-to-help-with-assembly/) (Amazon Q et al.,
  RAG corpus + automatic compiler feedback) → strengthen P3 `assemble_and_load`'s **structured DASM errors and
  immediate resubmission**. Our CLAUDE.md + docs are effectively the corpus.
- **Value of positioning:** a one-line note in `gap-analysis.md` / README that "2600 is unexplored; no known
  prior art" gives outward significance to spinning the harness out as its own project.

### G-3. prior art for test DSLs (don't reinvent P2)
- [`barryw/sim6502`](https://github.com/barryw/sim6502) (DSL + multiple backends) /
  [`64bites/64spec`](https://github.com/64bites/64spec) (KickAssembler describe-it spec) / sim65 (cc65, cycle
  display + trace) / [`AsaiYusuke/6502_test_executor`](https://github.com/AsaiYusuke/6502_test_executor)
  (cc65-based) / [`Klaus2m5/6502_65C02_functional_tests`](https://github.com/Klaus2m5/6502_65C02_functional_tests) (all opcodes).
- **Mining:** borrow these DSL shapes (`expect A == $1C` / `cycles <= 76`) for P2 D-1's assertion spec (don't
  reinvent). sim65 is in the settled architecture but **unwired** → run TIA-independent logic (score math, LFSR
  random, etc.) in sim65 for fast CI, and split TIA-related work to Gopher2600. Klaus2m5 can also guarantee
  Gopher2600's own CPU correctness.

### G-4. authoring-tool integration (medium-term, optional)
- [`PlayerPal 2.2`](https://atariage.com/forums/topic/318184-tool-update-playerpal-22/) (multicolor sprite
  editor → ASM/batari output) / [masswerk VCS tools](https://www.masswerk.at/vcs-tools/) / Tiny 8-bit sprite
  editor / batari's PF, sprite, and music editors.
- **Mining ideas:** ① import these ASM/data output formats into `<game>/gen`'s sprite tables. ② Ambitious:
  use the annotated screenshot **in reverse** = the user paints on the image → GRP/register data (extend
  CLAUDE.md's "primary channel" toward the **input direction** = a paint→register editor).
- **Caution:** only after the north star (Frogger) needs it. Sprawl risk, so keep it to indexing.

---

## Recommended order of attack

1. ~~**B-1 `read_cycles`** → **B-3 budget guard** (the crux)~~ ✅ v0.12.0–v0.13.0. Gap B closed in the real loop.
2. ~~**P1 `read_tia_registers` + `read_collisions`** (zero inference)~~ ✅ v0.14.0. Gap A closed.
3. ~~**B-2 `step_scanline`** (+ `step_instruction`)~~ ✅ v0.15.0 (`step_clock` at color-clock granularity unstarted).
4. ~~**P3 `assemble_and_load`** (friction reduction)~~ ✅ v0.16.0.
5. ~~**P2 verification automation** (the regression foundation)~~ ✅ D-1+D-2 (v0.18.0) + D-3 golden regression (v0.19.0). **Gap D fully closed.**
6. ~~**B-4 automatic kernel-constant calibration**~~ ✅ v0.20.0 (gap B fully closed). Remaining: **R-2 audio "authoring" side** / **R-1 Freeway port** / **making the harness standalone** (= into the authoring or spinoff phase).

> On the authoring (Frogger) side, **R-1 Freeway architecture** is the most immediately effective. Audio
> verification **R-2** is natural to add together by bundling an Audio shadow into P1 (`read_tia_registers`).
> **Don't self-implement P2/R-2 — wiring Gopher2600's existing packages from G-1 (`recorder`/`regression`/`tracker`)
> is the shortest path.**
> When implementing, follow CLAUDE.md "Smoke-test harness before reconnect": after modifying `bin/harness`,
> smoke-test with MCP `initialize` before asking to reconnect (to avoid an `AddTool` startup panic).
