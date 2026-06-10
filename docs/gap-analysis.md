# Gap analysis — how an LLM fails at Atari 2600 assembly

This is the **specification bedrock** of the project. Before "building a custom tool," it identifies
*what must be supplied so the model can author more accurately* as a set of gaps.

The failures are not uneven ability; they come from **specific information not being given to the model**.
Five gaps. The first half (A–C) is "the truth is invisible"; the second half (D–E) is "the loop is slow /
breakage goes unnoticed".

---

## A. Execution results are invisible (perception gap)

The model writes code **blindfolded**. A single screenshot is only "one instant" — it captures neither
motion (between frames) nor draw timing (within a line).

- **What's needed:** not pixels but **per-frame numeric state**
  (128 bytes of RAM / A·X·Y·SP·PC·flags / TIA registers / inputs / scanline / cycle).
- Screenshots are a supplement; the basis for judgment is numbers.
- **Annotated screenshot (`get_screen_annotated`):** overlay an **XY grid + axis labels** calibrated to
  the 2600's real coordinate system onto a Stella 1x snapshot
  (**horizontal = 0–159 color clocks / vertical = 0–191 scanlines**; not X-only — always draw both axes).
  Also draw Stella's **Fixed Debug Colors** (PF/P0/P1/M0/M1/BL in fixed colors) and markers for the
  **(X position, Y = drawn scanline range) of objects** read from registers/kernel, **always keeping image
  and numbers consistent**. X and Y are fundamentally different on the 2600:
  - **X (horizontal)** = the beam position at the RESPx strobe (continuous value + hardware-specific
    offset). The grid shows the mismatch "written position value → actual displayed column" = the
    observable for **failure #1**, which led to brute-forcing magic constants in Pong.
  - **Y (vertical)** = on which scanline drawing was enabled (the kernel's line counting). The grid shows
    sprite vertical drift and **zone-boundary scanlines** = the observable for the **screen roll
    (failure #3)** caused by miscounting `sta WSYNC`.
  - Judgment basis: **the final X verdict is the Gopher2600 TIA register value, not pixel-counting the
    image** (analog rendering introduces error). **Y reads exactly as an integer scanline number**, so it
    depends little on the image. In all cases the basis is numbers; the grid is a visual aid (subordinate
    to B).

## B. Cycles can't be counted exactly in your head (timing gap) ★ most critical

This is the crux. One scanline = **76 CPU cycles / 160 visible pixels**, 1 CPU cycle = 3 pixels. The
kernel must hit this cycle count exactly or the screen breaks. Branch cycle counts are variable (+1 on a
page crossing), and the model estimates "roughly." Roughly = collapse.

**Horizontal sprite positioning is the canonical failure** (in the literature, *"the most infamous aspect
of the Atari 2600's hardware interface"*): horizontal position is set by "the beam position at the moment
RESPx is strobed", and the idiom is **a `/15` loop (one turn = 15 color clocks = 5 CPU cycles) for coarse
alignment, plus HMOVE for ±7 px fine adjust (HMOVE must come right after WSYNC)**. This can't be hit
without knowing the "exact beam position" along a branchy code path. The model isn't bad at arithmetic —
it simply **can't pin this value without running**.

- **What's needed:** a mechanism that returns "the actual cycle count of this code region" and "the
  (scanline, colorclock) at the strobe moment". Plus a conditional stop ("halt if the kernel exceeds the
  cycle budget", a breakIf equivalent).
- **litmus test:** place a sprite at an arbitrary X / move it 1 px at a time. If this passes, the
  environment is real.

## C. 6502/TIA behavior isn't fully internalized (knowledge gap)

Training data is thinner than for modern languages, so the model **quietly misremembers** NTSC/PAL line
counts, TIA register addresses / bit layouts, the VSYNC(3)/VBLANK(37)/visible(192)/overscan(30)
boundaries, the HMOVE comb effect, and so on.

> **Important correction:** gap C is not "there are no references."
> The owner already has nearly all references (Nick Bensema's cycle counting guide, the Stella
> Programmer's Guide, Andrew Davie's "Newbies", disassemblies of real games, the woodgrain wiki's
> Playfield_Timing, etc. → inventoried in section C of `docs/tool-landscape.md`).
> **The problem is: having references in a folder ≠ the correct constant being live in the model's context
> at the moment of authoring.**
> So the countermeasure for C is not "add more references" but
> **(1) distill the core constants into CLAUDE.md so they're always in context + (2) verify with B instead
> of trusting memory**.
> In other words C is not solvable alone; it is **subordinate to B (verification)**.

## D. No reproducibility / regression (verification gap)

Fixing A silently breaks B's timing. Manual play is not reproducible.

- **What's needed:** **deterministic input scripts + automatic assertions**
  (`scanline == 262` / kernel within cycle budget / sprite X == expected at frame N).
- Two layers: a 6502 unit-test base (deterministic tests of pure logic) and per-frame assertions on the
  emulator.

## E. High iteration cost (friction) ✅ closed (v0.21)

If assemble → run → inspect isn't **one command the model can invoke immediately**, the iteration count
is too low and accuracy doesn't rise.

- **What's needed:** glue that completes "assemble → run → collect state numerically" in one command / one
  tool call.
- **Solution:** `assemble_and_load` (asm→load in one shot, v0.16) + scenario regression (inputs + numeric
  asserts + golden, v0.18-19) + specifying a `.asm` directly as the scenario `rom` (v0.21) → with
  `go run ./cmd/scenario foo.json`, "one source file → assemble → run → collect numerically → pass/fail"
  reaches one command.

---

## Summary: what fills which layer

| Layer | What it fills | Plumbing |
|----|-----------|------|
| A perception / B timing | per-frame numeric state, cycle / beam position | **MCP / verification harness** |
| C knowledge | always-in-context core constants + verification | **distillation into CLAUDE.md** (subordinate to B) |
| D verification | deterministic inputs + automatic assertions | **6502 test base + frame assertions** |
| E friction | one command | **tool glue / IDE integration** |

- A Stella-only MCP fills only part of A; that's why the former name `Stella-MCP` was too narrow.
- **B is the top priority** (the crux, and it has the litmus test). C rides on B. A is a prerequisite for B.

---

## Empirical data from past Pong work

In the post-mortem of `260304_Claude-Code-Pong` (3 abandonments + 1 restart), the observed failures map
cleanly onto gaps A–E. **Every abandonment died on unverified timing / positioning.**
(Evidence is in that project's memory and archive.)

### B (timing) — most frequent and fatal

1. **Horizontal position relied on "empirical magic constants."** memory `feedback_asm_architecture.md`
   notes "the theoretical formula `P1 = 160 - P0 - width`, but in practice it's off by a few pixels
   (hardware-specific offset)", and the final values were found by trial and error: score `P0=48 / P1=124`
   (NUSIZ `$05`), paddle `P0=16 / P1=143`. → **evidence the model couldn't derive them and brute-forced.**
   This is exactly gap B, "can't pin it without running."
2. **Hand-placed NOP positioning** (`archive/claude.ai_ver/pong.asm` 243–281, 31 NOPs targeting
   "cycle ≈ 71") breaks if a preceding instruction changes by one. It regressed even though the correct
   `PosObject` (divide-by-15) exists.
3. **Scanline-count drift → screen roll.** In a multi-zone kernel, `sta WSYNC` inside `PosObject` wasn't
   counted in the visible-region line budget and overran (`archive/terminal_02/02_step08.asm`). Three
   `PosObject`s during VBLANK pressured the `TIM64T #43` budget (`archive/terminal_01/01_v2_step3.asm`).
   → a compound of gap A (invisible) + B (uncountable).

### D / E (verification / friction)

4. **Bulk changes made bugs un-bisectable.** terminal_01 v2 "didn't work well and was frozen"
   (`project_pong_status.md`). → small-step build→check, revert to the previous step on failure
   (`feedback_dev_process.md`).
5. **Verification depended on human eyeballing of screenshots.** Step 2 (net) unverified, step08 stalled
   unconfirmed. → **the harness itself must verify.**

### Reusable assets

- The proven `PosObject` (divide-by-15) and `docs_atari/8bitworkshop_samples/sethorizpos.asm` (the correct
  hand-placed-NOP version).
- The established position constants (above) — a starting point to avoid rediscovery.
- The `cc-pong.asm` Step 1 skeleton (TIM64T, exactly 262 lines).
- The game logic in `archive/terminal_01/01_v2_step3.asm` (AI / 3-zone bounce angles / collision
  `CXP0FB` / score / sound) — display was unstable but it's useful as an **algorithm port source**.
- The reference corpus (`docs_atari/`, `docs_pong/`) — gap C is already collected.

### Conclusion

**The greatest leverage is "having the tool itself verify horizontal position and scanline counts, instead
of eyeballing Stella screenshots."** → **a verification harness that closes gap B is the top priority**,
settled. C rides on B (distill + verify). A is a prerequisite for B.
