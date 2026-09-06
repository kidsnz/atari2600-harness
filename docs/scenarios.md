# Scenario regression (P2 / gap D) — format reference

A regression mechanism that declares an "input timeline + numeric assertions" in a single JSON and
auto-passes/fails it against a ROM (`internal/scenario` + `cmd/scenario`, v0.18.0). No MCP required, so
it runs in CI. All judgments are numeric (rule 1).

```
go run ./cmd/scenario <scenarios>/*.json   # exit 0 if all pass, exit 1 on failure
```

Scenarios live under a `scenarios/` directory; the ROM path is relative to the directory you run from
(the harness's own scenarios are under `roms/litmus/scenarios/`, run from the harness root).

## Schema

```jsonc
{
  "rom": "roms/litmus/smoke.bin",      // required. If a .asm is given it is assembled with dasm first (source -> verdict in one command)
  "tv_spec": "NTSC",                    // default NTSC
  "warmup_frames": 2,                   // free-run before measuring (boot settling; default 2)

  "inputs": [                           // D-2: input timeline (frame is 0-based AFTER warmup)
    {"frame": 1, "player": 0, "action": "up", "pressed": true}
    // action: left|right|up|down|fire|center / applied before that frame is run
  ],

  "asserts": [                          // D-1: instantaneous numeric conditions at frame end
    {"at_frame": 0, "field": "ram.0x81", "op": "==", "value": 144},
    {"at_frame": 1, "field": "ram.0x81", "op": "==", "value": 128}
  ],

  "frames": 600,                        // run length for whole-run monitoring (default = max input/assert frame)

  "fuzz": {                             // deterministic simulation testing: seeded random input over N frames
    "seed": 7, "frames": 600, "actions": ["left", "right", "fire"], "player": 0
  },

  "metrics": ["ram.0x85"],              // fields captured at end of run (for cmd/metamorphic comparison)

  "invariants": [                       // property: a condition that must hold at EVERY frame (first break reported)
    {"field": "ram.0x9B", "op": "in", "lo": 0, "hi": 5}
  ],

  "monotonic": [                        // property: a field that only moves one way over the run
    {"field": "ram.0x85", "direction": "up"},    // score never decreases
    {"field": "ram.0x9B", "direction": "down"}   // lives never increase
  ],

  "temporal": [                         // VV-5: bounded temporal logic over the FRAME SEQUENCE
    {"kind": "eventually", "field": "ram.0x9B", "op": "==", "value": 3, "within": 60},        // P holds within K frames (bounded liveness)
    {"kind": "response", "a_field": "ram.0x80", "a_op": "==", "a_value": 1,                    // whenever A holds...
                         "field": "ram.0x85", "op": ">", "value": 0, "within": 10},           // ...P must hold within K frames
    {"kind": "never_for", "field": "ram.0x90", "op": "==", "value": 0, "n": 120}              // P must not hold for N consecutive frames
  ],

  "checks": {                           // whole-run properties (measurements with side effects; evaluated after the timeline)
    "ntsc_frame_lines": 262,            // StepFrame() == 262 (ONE frame)
    "frame_lines_stable": {"frames": 130, "lines": 262},  // every frame in the window has the SAME count (∀ sibling; `lines` optional)
    "ram_budget": {"board": 15, "buffer": 60, "delta": 8, "stack": 6},
                                        // the declared RAM layout fits in 128 bytes AND is no
                                        // smaller than what the program actually writes (.asm only)
    "max_flicker_area": 1500,           // pixels whose DRAWING OBJECT changes between two frames
    "motion": {"object": "P0", "axis": "top", "frames": 40, "max_jerk_rms": 0.5},
                                        // VV-4: how SMOOTH an object's motion is, over frames
    "no_beam_race": {"object": "P0", "line_from": 40, "line_to": 90},
                                        // AT-3: every pixel-data write for that object lands
                                        // before the beam reaches it, on every line in the range
    "max_line_budget": 76,              // budget guard is never exceeded (equivalent to assert_line_budget)
    "golden_frame": true,              // D-3: compare the rendered frame-chain hash against <scenario>.golden
    "golden_audio": true,              // A-2: compare the audio-chain hash against <scenario>.audio.golden
    "golden_mix": true,                // BOTH channels through the TIA's non-linear output stage,
                                       //   vs <scenario>.mix.golden. golden_audio hashes AudioChannel0
                                       //   ALONE, so a ROM's second voice is invisible to it (measured:
                                       //   byte-identical with channel 1 silent, half and full). Use this
                                       //   whenever a ROM sounds two voices.
    "no_timer_wrap": 3,                // VV-10 T-1: watch N frames; fail if INTIM is read after the timer wrapped (G8)
    "no_hmove_hazard": 2,              // VV-10 T-2: watch N frames; fail if HMxx is written within 24cy of HMOVE
    "score_equals_ram": {"ram": "ram.0x80", "frames": 4}, // VV-9: OCR the rendered 2-digit BCD score == RAM (font from <scenario>.font)
    "no_uninit_read": 2                // VV-10 T-3: from reset, fail if a RAM byte is read before it was ever written
  }
}
```

- Operators `op`: `==` `!=` `<` `<=` `>` `>=` and `in` (range; uses `lo`/`hi`, inclusive — for `asserts` and `invariants`).
- `value` is an integer (bool fields are 0/1).
- `at_frame` / `inputs.frame` are 0-based frame numbers **after warmup**. Frame f = "apply input → run one frame → evaluate asserts".
- **`invariants`** = property-based: each condition is checked at the end of **every** frame (0..`frames`); only the **first** break is recorded (with the frame number). A held invariant is reported as `… held [N frames]`. (QuickCheck / contracts — `docs/testing-playbook.md`.)
- **`monotonic`** = a field that may only move one way over the run: `up` = non-decreasing, `down` = non-increasing. Catches "score went down" / "lives went up". First violation reported with `prev->got`.
- **`frames`** extends the run beyond the last input/assert frame so `invariants`/`monotonic`/`temporal` are monitored over a meaningful window.
- **`temporal`** (VV-5) = bounded-temporal-logic properties over the **frame sequence** (things a per-frame `invariant` can't say). `"always P"` is just an `invariant` — not duplicated here. Three `kind`s:
  - **`eventually`** — `P` (the `field`/`op`/`value`/`lo`/`hi` condition) must hold within `within` frames of the run start (bounded liveness). If the run ends before the window fully elapses and `P` never held, the result is **INCONCLUSIVE** (a failure, *not* a vacuous pass) — set `frames` large enough to cover the window.
  - **`response`** — whenever the trigger `A` (`a_field`/`a_op`/`a_value`/`a_lo`/`a_hi`) holds at frame *f*, `P` must hold within `within` frames (*f*..*f*+`within`). An obligation whose window extends past the run end is INCONCLUSIVE.
  - **`never_for`** — `P` must not hold for `n` consecutive frames (safety; fully decidable on the observed trace).
  Scenario-side only (runs via `cmd/scenario`, no MCP tool). Self-test: `TestEvalTemporal` (planted traces) + `TestTemporalThroughRun`. Sample: `roms/litmus/scenarios/temporal.json`. **Src:** Bauer/Leucker/Schallhart TOSEM 2011 (LTL₃); STL RV'15.

## Field vocabulary (`field`)

The assertion vocabulary maps one-to-one to the read methods on `internal/emu` (the observation tools are
reused as-is for regression). **Unknown fields are an error** (typos are not swallowed).

| field | source |
|---|---|
| `frame` / `scanline` / `clock` | `Coords()` |
| `cycles_total` | `TotalCycles()` |
| `cpu.pc\|a\|x\|y\|sp` | `VCS.CPU` |
| `ram.0xNN` (hex/decimal) | `PeekRAM` |
| `tia.<obj>.reset_pixel\|hmoved_pixel` (obj=player0/1, missile0/1, ball) | `read_tia` equivalent |
| `tiareg.player0\|player1.color\|nusiz\|reflected\|vertical_delay` | `ReadTIARegisters` |
| `tiareg.playfield.pf0\|pf1\|pf2\|foreground_color\|background_color\|ctrlpf\|reflected` | ditto |
| `tiareg.ball.color\|enabled` | ditto |
| `collisions.<pair>` (p0_p1, m0_p0, p0_pf, bl_pf …) | `ReadCollisions` |
| `audio.ch0\|ch1.control\|freq\|volume` | `ReadAudio` |

- **`motion`** = the smoothness of one object's movement, as `jerk_rms` over a window of frames
  (`object` P0/P1/M0/M1/BL, `axis` `"top"` for the rendered vertical or `"x"`, plus `frames`,
  `warmup` and a `y_top`/`y_bot` search window). A position that jumps rather than glides fails it.
  Undocumented until 2026-09-06, when `check_wiring.py` started requiring every check in the schema
  to be named here and found it.
- **`no_beam_race`** = for one object over a range of scanlines, every write of its pixel data lands
  **before the beam reaches it**, on every line. The author declares the object and the range
  (`line_from`/`line_to`), which is what makes it sound: the tool checks a stated intention rather
  than guessing one. Also found by that gate.
- **`ram_budget`** = the declared RAM layout, checked twice. First the arithmetic
  (`board + buffer + delta + stack ≤ 128`, via `design.ScrollBackgroundFitsRAM`), then the
  declaration against **what the program actually writes** — `cyclebound.DefUse`'s static write set,
  every address any reachable instruction might touch over all paths. Numbers an author writes into
  a scenario are a claim about the program, and arithmetic on a claim grades the claim; the second
  half is what makes it grade the ROM. Refuses to grade rather than under-report when a write target
  cannot be pinned down, and says **SKIPPED** when `rom` is a `.bin` with no source. How to arrive at
  `board`: cells × bits-per-cell ÷ 8 (see `pkg/design/pf.go`, which cites five worked examples from
  the list at bit widths 2, 3, 4 and 8).
- **`max_flicker_area`** = pixels whose **drawing object** (BG/PF/P0/P1/M0/M1/BL) differs between two
  consecutive frames. Not a pixel diff: a colour register sweeping every frame contributes nothing,
  and a static picture reads exactly **0**. Calibrated on the first work's 134 ROMs — the ten the
  author named `*flick*` all read non-zero, 115 read zero, and the flickering variants sit between
  2.6% and 8.0% of the picture. **It does not set a threshold**: the archive describes the limit in
  words ("an area as large as an Arkanoid wall"), the quantity is a judgement about eyes, and this
  check exists so the author can decide a ceiling once and have a machine keep it. It also does not
  separate movement from blinking — a meter bar changing height reads as flicker.
- **`frame_lines_stable`** = the ∀-over-frames sibling of `ntsc_frame_lines`. That check samples **one**
  frame and therefore certifies nothing about the next one; this one steps `frames` frames (continuing on the
  emulator the timeline drove) and requires every frame to report the same scanline count, optionally equal to
  `lines`. A frame total that changes between frames rolls the whole picture by that many lines on a CRT —
  invisible to a single-frame check and to a golden hash, both of which happily stayed green while a
  reproduction breathed 261/262. Verdicts print the full histogram (`262x129 264x1`), so the measurement is
  in the output whether it passes or fails.
  **A pass covers only the frames it measured, and that is not a formality:** `roms/techniques/banked_game.bin`
  runs 264 lines on every 120th frame, so a 60-frame window passes it and a 130-frame window catches it
  (`TestFrameLinesStable` pins both). Size the window past the ROM's slowest periodic event — bank switches,
  scene changes, respawn timers.

- **`pf_deadlines`** (`true`) = every playfield write lands **before the beam reaches the columns it
  governs**, proven over all paths (`.asm` source only; a `.bin` scenario prints `skipped`). This is a
  DIFFERENT question from `prove_line_budget`, and the pair `roms/litmus/pf_ontime.asm` /
  `roms/litmus/pf_late.asm` exists to make that concrete: both draw the same 40-column asymmetric
  playfield, both fit in 76 cycles, both are CERTIFIED, and both report 262 lines — the only check that
  separates them is this one. `pf_late` puts three cycles of index arithmetic at the *top* of the line
  instead of in its tail, so PF1 lands at clock 31 against a deadline of 16 and the picture comes out
  shifted right by four columns with the previous line's right edge wrapping in at the left. That shipped
  (technojacket `cover-tear-speck`, 2026-08-09, measured at 75 of 76 cycles) and the author found it by eye.
  The deadlines are the machine's geometry: PF0 by clock 0, PF1 by 16, PF2 by 48, and in repeat mode the
  right half's rewrites by 80 / 96 / 128; `COLUPF`/`COLUBK` by 0.
  **What it cannot judge it says so about rather than passing:** a third write to the same register in one
  line, or a register with no column rule, is counted in the `outside these rules` figure printed with the verdict,
  and a declined analysis fails instead of passing silently.

`checks` (whole run): `ntsc_frame_lines` (`StepFrame`) / `frame_lines_stable` (`StepFrame` × N, histogram) /
`pf_deadlines` (`cyclebound.CheckPFDeadlines`, static ∀) /
`max_line_budget` (`RunUntilBudget`) /
`golden_frame` (render-chain hash, below) / `golden_audio` (audio-chain hash, same mechanism via
Gopher2600 `digest.Audio`, compared against `<scenario>.audio.golden`).

## Golden-frame regression (D-3, v0.19.0)

With `checks.golden_frame: true`, the **rendered frame-chain hash** of the timeline (excluding warmup) —
a sha1 chain of Gopher2600's `digest.Video` — is compared against `<scenario>.golden` (a sibling file),
i.e. regression detection of the rendered pixels.

```
go run ./cmd/scenario -update <scenarios>/foo.json   # record/update the baseline .golden
go run ./cmd/scenario         <scenarios>/foo.json   # compare against the baseline (fail on mismatch)
```

If `.golden` is missing or `-update` is given, the current hash is recorded. The hash is deterministic
with warmup excluded (reproducible for the same ROM + same input + same frame count). It guards
**rendering itself**, a layer separate from logic/timing regression (D-1/D-2). `.golden` is git-tracked
(commit it as the baseline).

## Bundled samples (in this repo)

- `roms/litmus/scenarios/smoke.json` — `ram.0x80==$42` + 262 lines + no budget overrun.
- `roms/litmus/scenarios/collide.json` — `collisions.bl_pf==1` (ball × fully-lit PF).
- `roms/litmus/scenarios/golden.json` (+ `golden.golden`) — render frame-chain hash regression.

Game repositories add their own under `<game>/scenarios/` — e.g. Frogger's `boot.json` (initial FrogY
144 + 3 lives + 262 lines), `hop.json` (`up` input drives FrogY 144→128), `golden.json`.

- **`fuzz`** = deterministic simulation testing: seeded random inputs from `actions` over `frames`, with
  `invariants`/`monotonic` monitored every frame and CPU jam (crash) detected. Deterministic, so a found
  failure reproduces by re-running the same seed (replay). (AFL / FoundationDB-Antithesis.)
- **`metrics`** = fields captured at the end of the run (before side-effecting checks), exposed in
  `Result.Metrics`, for `cmd/metamorphic` to relate two runs.

## The wider testing suite (CLIs over scenarios)
The scenario fields above (`invariants`/`monotonic`/range/`fuzz`/`metrics`) run via `cmd/scenario` and the
`run_scenario` MCP tool. Two more disciplines operate *over* scenarios as separate CLIs:
- **`cmd/mutate`** — mutation testing: inject a ROM-byte fault, confirm the scenario suite catches it (kill).
  Single mode exits 1 on a SURVIVOR; batch mode reports a seeded kill rate. Grades the tests.
- **`cmd/metamorphic`** — assert a relation `A.field <rel> B.field` between two scenario runs (oracle-free).
- **`cmd/mine-invariants`** — Daikon-lite: drive a ROM and emit candidate `invariants`/`monotonic` as a spec draft.
See `docs/testing-playbook.md` for when to use each.

## Bundled samples (added)
- `roms/litmus/scenarios/invariants.json` — `invariants` (`ram.0x80 == 66`, range `in [60,70]`) + `monotonic`
  (`frame` up) + the `in` range assert, over `frames: 5`.
- `roms/litmus/scenarios/fuzz.json` — `fuzz` (seeded random input) + an invariant + 262 lines.

## Out of scope (next step)
- Scanline-targeted asserts, audio golden (`digest.Audio`).
