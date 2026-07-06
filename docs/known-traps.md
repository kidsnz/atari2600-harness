# Known traps — "passes in the emulator, breaks on real hardware" (and other silent killers)

A catalog of the Atari 2600 traps mined from AtariAge that **do not show up as a failure in a naive
emulator run** but corrupt the result on real hardware (or in a stricter emulator). Past Pong attempts died
on exactly this class of bug ("unverified timing / positioning"). This is the **pre-flight checklist** the
authoring loop runs before shipping a kernel, and the **spec for `scripts/check_traps.py`** (the future
trap linter, [[feedback-authoring-loop-system]]).

Detection column: **static** = a source-text linter can flag it · **runtime** = needs a sim assert /
`breakif` / `trace_clocks` · **manual** = judgment / pixel compare.

> Provenance: every row cites the mined thread(s) it came from. Raw notes in `reference/atariage/<id>-*/notes.ja.md`.

## A. Timing / sync (the Pong killers)

| trap | what happens | fix | detect | source |
|---|---|---|---|---|
| **RIOT timer wraparound write** | writing `TIM64T`/`TIM1024T` on the exact wraparound cycle silently drops the divider to **1T** → frame length collapses → rolls on real HW, Stella passes | **double-write** the timer; or never write on the wrap cycle | runtime (`breakif` timer-write@wrap) | 303277, 316035, 327383, 202736, 323670 |
| **HMOVE then HMxx within 24 cy** | writing any `HMxx` within 24 CPU cycles after an `HMOVE` strobe → unpredictable motion on HW | keep ≥24 cy between HMOVE and the next HMxx write | runtime (trace_clocks) | 307641 |
| **mid-line HMOVE / cycle-73-74 strobe** | the "no black comb bar" trick (HMOVE at cy73/74) only moves **left**, and `WSYNC`-then-`HMOVE` becomes unusable; behaviour differs by TIA revision | use only if you accept left-only; pin TIA revision when verifying | runtime + manual | 183219, 210361, 162520, 133205, 280660, 319456 |
| **total scanlines not constant** | frame line count varies between frames → jitter / roll even if each frame is "near 262" | keep total scanlines **identical every frame** (variable work absorbed in a fixed budget) | runtime (`ntsc_frame_lines`) | 171270, 303750, 306318, 76728 |
| **GRP/COLUP written outside HBLANK** | `GRP*` must finish within the ~22-cy HBLANK or the sprite's left edge corrupts; same for objects updated mid-visible | do graphics register writes in HBLANK | runtime/manual | 166749, 63229, 74225 |
| **PF band's first row partial / the "eaten line"** | a full-width `PFx` band whose writes don't finish in its first row's HBLANK (e.g. PF set on the *previous* line's trailing, no leading `WSYNC`) renders that row partly black — the left edge is unfilled because PF fills as the beam crosses (same root as GRP-outside-HBLANK). And if an object's last row shares that line, the PF write eats it → you get the object's last row **xor** a full-width PF row, not both | set the band's PF early in the **HBLANK of its first line** (after that line's `WSYNC`); to keep BOTH an object's last row AND a full-width PF row, give the band **1 extra line** | manual / `read_row` the band's first row | in-house: PONG static-repro 2026-06-19 |
| **RESx on a drawing line** | `RESxx` resets immediately; striking it on a visible/draw line glitches | reposition on a dedicated line, not while drawing | manual | 67525, 74225 |
| **HMOVE comb on a visible line** | every `HMOVE` right after WSYNC blanks color clocks 0-7 of THAT line (the comb) — if a drawn band's first row shares the HMOVE line, its left 8px go black (e.g. a top-wall row-1 notch) | land every HMOVE on a **black/blank line**; give the band's first row a line with no HMOVE (PF set in its own HBLANK) | `read_row` the band's first row = full 160 | in-house: PONG pf2 top-wall notch 2026-07-01 |
| **N-edge coincidence = the real worst path** | a per-line kernel with several `cpy <edge>` branches (ball top/bottom, paddle tops/bottoms…) gets budgeted on "typical" frames, but the true worst case is **all edges landing on the SAME Y** (+~5cy per extra hit) — a 1cy overrun that free-run testing may miss for hundreds of frames, then rolls once | hand-place the coincidence: poke every edge variable to one Y and `assert_line_budget`; add it as a standing fuzz case | poked-coincidence assert; `beamtrace` shows the stacked writes on one line | in-house: PONG PlayF 77cy (3 edges + INPT poll) 2026-07-02 |
| **velocity-negate bounce without position clamp = sign-flip trap** | upgrading fixed-value reflection (`DY:=-1`) to angle-preserving `DY:=-DY` silently drops the *idempotence* the old code relied on: if the object can sit at/past the wall row for 2+ frames (re-hit, \|DY\|≥2 overshoot), the sign flips every frame → the object oscillates in the wall zone forever | clamp the POSITION back to the last legal row on every wall reflection (top→0, bottom→max−h) | free-run + watch the position variable near walls | in-house: PONG english 182↔184 trap 2026-07-02 |
| **`bpl`/`bmi` clamp on a coordinate that legitimately exceeds 127** | using bit7 (`bpl`→"non-negative") to detect underflow on a value whose valid range is 0..N with N>127 (e.g. a playfield Y of 0..182): every legit value ≥128 has bit7 set and is misread as "negative" → silently clamped to 0. A target of Y=150 becomes 0 = paddle snaps to top | detect underflow by the **subtraction borrow / wrap magnitude**, not bit7: after the signed add, `cmp #<wrap-threshold>` (e.g. `#200`) to separate a wrapped-underflow (≈247-255) from a legit high value (≤~180). **The threshold is range-dependent** — set it above the max legit value: a lead/extrapolation like `BallRow + 8·DY` reaches ~202, so it needs `cmp #220` (not `#200`, which would misread 202 as underflow) | poke the coord to a >127 value and read the derived target | in-house: PONG AI target Y clamp 2026-07-03 (threshold refinement: v3 lead AI 2026-07-06) |
| **`cmp` clobbers Z between a load and a test-for-zero `bne`/`beq`** | to convert "0 → 1" you `lda v / … / bne keep / lda #1`, but if a `cmp #k` sits between the `lda v` and the `bne`, the branch tests `v==k`, **not** `v==0` (cmp overwrote Z). The clamp `lda v / cmp #hi / bcc / … / bne set` silently mis-handles `v==0` because the `bne` reads the cmp's result. Sibling of the immediate-`ld` clobber below; here it's `cmp` | do the **zero test before any `cmp`** that reuses the register (`lda v / beq zero / cmp #hi / …`), mirroring the working pattern in the serve-angle clamp | poke the input to 0 and read the output; it comes out 0 instead of the intended 1 | in-house: PONG serve-angle continuation clamp 2026-07-04 |
| **immediate `ldx`/`lda`/`ldy` clobbers N/Z between a flag-set and its branch** | a load-immediate sets N/Z from the loaded constant. Placing `ldx #80` after `lda dir` (to hold a value for the taken branch) destroys the sign flag the following `bpl`/`bmi` was meant to test → the branch reads the constant's sign, not `dir`'s. `sta` preserves flags (safe), but any `ld_` does not | branch **before** any `ld_`, or avoid the branch entirely with arithmetic (`dir` ∈ {\$01,\$FF} → `X = dir + 79` gives 80/78 branchlessly, no flag dependence) | step through and read the branch's actual target | in-house: PONG serve-side select 2026-07-03 |

## B. Positioning ground-truth (why "the X doesn't match")

| fact | value | source |
|---|---|---|
| **internal draw delay after RESxx** | player draws **+5 CLK** late, missile/ball **+4 CLK** late (RESx resets instantly but the object appears later) — *first suspect when target X is off by ~5px* | 294398, 283075, 305780, 75335-cluster |
| RESx strobe granularity | 3 color-clocks; RESP0 finishing at cy46 → X≈75 | 172089, 137739, 329611, 304182 |
| HMOVE range | ±8 px / scanline; pulse count = upper nibble of `HMxx EOR $80` | 319456 |
| coarse÷15 + fine HMOVE | `eor #7` → 4×ASL, ~30 cy; small real-HW latch differences | 304182, 284554, 160645 |
| ÷15 / X(N) is kernel-specific | the absolute offset includes the prologue cycle count → **measure `read_tia` HmovedPixel, don't hardcode N** | (CLAUDE.md) + 294398 |
| **positioning ÷15 loop crossing a page → judder** | if the `sbc #15` / `bcs` divide loop straddles a page boundary, the **taken `bcs` costs +1 cycle** → each iteration is 6cy(18px) not 5cy(15px); the coarse step no longer tiles with the HMOVE fine (±~7) → the object **judders ~3px at every 15px while moving** AND the X(N) offset inflates by +1cy/iteration. A *static* object hides it (only shows when it MOVES across cells). **Keep the divide loop on one page** (page-align it). Found: PONG smooth-ball, the loop sat at $F0FE/$F100 — moving it to $F100 (whole loop on one page) made the ball perfectly smooth (read_motion jerk→0). detect: runtime (read_motion jerk_rms) / listing (loop address vs page boundary) | in-house: PONG 2026-06-19 |

## C. Cartridge / bank / RAM

| trap | fix | detect | source |
|---|---|---|---|
| **F8/F6/F4 boot in a random bank** | every bank's reset entry must `JMP` to a common init in bank 0 | static (check each bank start) | 194935, 293970, 261488 |
| `NOP $00` / `BIT $00` as a skip on 3F/X07 carts | triggers an unintended bankswitch → use `NOP $80` / `.byte $2C` on a safe address | static (grep `nop $00`/`bit $00`) | 139089 |
| **read of a write-only hotspot / SuperChip write-port** | undefined on HW (varies by console) → never read it | static/manual | 169114, 285759, 204819, 111536 |
| **STA to ROM** | no R/W line to the cart → bus contention, never persists; RAM is `$80–$FF` only | static | 148390, 62852, 293816 |
| bank-move misaligns code → page-cross | split heavy moves across frames / `ALIGN 256` | static (ALIGN) | 307854, 339019, 133720 |
| variable placed at `$FF` | `JSR` push lands on `$0100` mirror and clobbers it → keep vars in `seg.u` from `$80` | static (var ≥ $FB warn) | 302998, 301766, 290790 |

## D. CPU / 6502

| trap | fix | detect | source |
|---|---|---|---|
| missing `CLD` | BCD math gives garbage; D is undefined at power-up | static (`CLD` in init) | 318346, (CLAUDE.md) |
| `ADC` without `CLC` / `SBC` without `SEC` | carry contaminates the result | manual | 157598, 75335, 63853, 63389 |
| **post-reset SP / RAM / flags undefined** | uninitialised everything → `CLEAN_START` is mandatory (proven on 5 consoles) | static (CLEAN_START) | 261488, 312005, 316071 |
| page-cross `+1` cycle on reads | reads (not stores) take +1 across a page; throws off cycle budget | static (CHECKPAGE / ALIGN) | 132913, 125755, 147642 |
| unstable illegal opcodes | `LAX/SAX/SBX/DCP/ASR` are HW-stable; **`LXA/XAA` are not** (chip/temp dependent) | static (opcode allowlist) | 168616, 132496, 139505 |

## E. TIA read / audio / emulator-fidelity

| trap | note | source |
|---|---|---|
| **reading a write-only TIA register** | returns bus float; only **bits 6/7 are driven** (collision/INPT) — don't rely on the full byte | 319781, 328451, 63342 |
| **2-voice audio phase interference** | two voices can partially cancel to near-silence; Gopher2600's noise (old TIASound) differs from real HW | 294766, 272769, 326549 |
| **AUDF-change propagation latency** | TIA tone is an LFSR-pair (5+4 stage), not a table; **lowering AUDF can take up to ~32 cycles (~1ms)** before the next output clock (the up-counter must wrap) — pitch changes lag, don't assume instant | blog 1116/1134/1140 |
| **mid-line NUSIZ double/quad copy trick** | renders on real HW but **not in Gopher2600** → ROMs using it won't pixel-match our oracle | 181903 |
| **player-width change → 1-clk right shift** | changing NUSIZ player width shifts start by 1 color-clock (missile/ball unaffected) | 143781, 64251, 290654 |
| **state isn't all in RAM** | sprite X positions live in TIA internal counters → full-state capture needs `read_tia_registers`, not just `read_ram` | 301766 |
| smooth horizontal PF scroll | **hardware-impossible** (RSYNC won't help); use ball/missile edge or delayed/tile scroll | 178574 |
| PAL: odd-scanline color / interlace | PAL color circuitry fails on odd scanlines; line-dropping kills color | 124445, 200197, 293881 |

---

**Use:** before shipping a kernel, walk this list; the static-detectable rows are the first targets for
`scripts/check_traps.py`. The runtime rows become `breakif`/scenario asserts. For Pong specifically, the
A-section (timing/sync) is the historical cause of every past abandonment.
