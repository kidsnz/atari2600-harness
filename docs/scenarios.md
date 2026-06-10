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

  "checks": {                           // whole-run properties (measurements with side effects; evaluated after the timeline)
    "ntsc_frame_lines": 262,            // StepFrame() == 262
    "max_line_budget": 76,              // budget guard is never exceeded (equivalent to assert_line_budget)
    "golden_frame": true               // D-3: compare the rendered frame-chain hash against <scenario>.golden
  }
}
```

- Operators `op`: `==` `!=` `<` `<=` `>` `>=`.
- `value` is an integer (bool fields are 0/1).
- `at_frame` / `inputs.frame` are 0-based frame numbers **after warmup**. Frame f = "apply input → run one frame → evaluate asserts".

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

`checks` (whole run): `ntsc_frame_lines` (`StepFrame`) / `max_line_budget` (`RunUntilBudget`) /
`golden_frame` (render-chain hash, below).

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

## Out of scope (next step)
- An MCP-tool variant `run_scenario` (could share logic with the CLI).
- Range operators, scanline-targeted asserts, audio golden (`digest.Audio`).
