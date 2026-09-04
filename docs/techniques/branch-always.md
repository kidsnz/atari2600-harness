# Technique — branch-always / "simulated BRA" (a conditional branch whose flag is already fixed)

*This entry corrects the source it cites* — the economics below are the correction. That happens often
enough to expect, and rarely enough to say out loud; **it is not the same thing as us having misread a
source**, which happens too and reads identically from inside. Both kinds are recorded in `CHANGELOG.md`
at the point they were found, which is where to count them if a count is ever wanted — **this line used
to carry one, written by hand, and it was already wrong when it was written.**

**Goal:** replace a 3-byte `JMP abs` with a 2-byte relative branch when the *preceding*
instruction has already fixed the flag the branch tests.

**It is a byte trick, and only when the flag comes free.** The branch alone is −1 byte and ±0
cycles on the same page, +1 cycle across one. In this repository the crossing has never been paid:
of the 28 uses **27 stay on the page and exactly one crosses**, and that one is `litmus_6502:128`,
where the crossing is *the thing being measured* (`org $F5F4` / `org $F601`). So the rule says a
cycle-counted kernel can lose by using this; the corpus says ours never has. Both are true; either
alone is false.

Demo: TODO — no standalone demo; the idiom appears inside 10 existing ROMs (below).
CI: TODO — no gate yet. Proposed gate + negative controls under "How to verify".
Hardware basis: **`litmus_6502`, pinned by regression** — not by its comments. The ROM saves each
measuring window as `lda INTIM / eor #$FF / sta $9x`, and `roms/litmus/scenarios/cpu6502.json`
fixes the three results:

| scenario line | value | branch case | ROM comment |
|---|---|---|---|
| `ram.0x97 == 135` | 135 | not taken | `; 不成立 (2cy) → 窓 = 2+2+4` |
| `ram.0x98 == 136` | 136 | taken, same page | `; 成立・同ページ (3cy) → 窓 = 2+3+4` |
| `ram.0x9a == 137` | 137 | taken, crossing | `; 成立+跨ぎ (4cy) → 窓 = 2+4+4` |

**The bridge between the two columns — write it down, because nothing else does.** The window is
started by `ldy #$80 / sty TIM1T`, so INTIM begins at 128 and falls one per CPU cycle; the ROM
stores `255 − INTIM`, which is `127 + elapsed`. The three windows are 8, 9 and 10 cycles, giving
135, 136, 137. **Read the differences, not the absolutes**: `fundamentals-audit.md:25` still marks
the timer's *exact first-decrement offset* ⬜ unverified, so the absolute value carries that
unknown — but 135→136→137 is exactly the 2→3→4 the branch costs, and that is what these scenarios
pin. `go test ./internal/scenario/ -run TestEveryScenarioRuns` passes (43.6 s).

## The pattern

Three seeds, each of which pins one flag so the following branch can never fall through:

| seed | flag it fixes | branch that is then unconditional |
|---|---|---|
| `lda #0` (`#$00`) | Z = 1 | `beq` |
| `lsr` / `asl` (accumulator) | N = 0 (bit 7 shifted out) | `bpl` |
| `lda #<non-zero immediate>` | Z = 0 | `bne` |

```
        lda #0              ; A = 0 AND Z = 1 — two results, both used
        beq VStore          ; always taken: the branch is free of a JMP's third byte
VDraw:  ldx sprDraw
        lda ArtRev,x
VStore: sta GRP0            ; both paths converge here
```
〔`roms/techniques/vertical_pos_dcp.asm:104-108`〕

The value is doubled when the seed is one you needed anyway: in the kernel above, `lda #0` is
loading the blank sprite byte, and the flag it happens to set buys the jump for nothing.

## Economics — and the condition the source leaves out

| | bytes | cycles |
|---|---|---|
| `jmp abs` | 3 | 3 |
| branch, taken, same page | 2 | 3 |
| branch, taken, **crossing a page** | 2 | **4** |

**The branch alone is −1 byte and ±0 cycles on the same page, +1 across one.** But that is only
half the sum. The seed has a size too, and whether it counts depends on one question:

**Was the seed there anyway?**

| | bytes | cycles | vs `jmp abs` (3 bytes / 3 cy) |
|---|---|---|---|
| seed needed anyway → branch only | 2 | 3 | **−1 byte, ±0 cycles** ✓ |
| seed added for the branch: `lda #imm` + branch | 2+2 = 4 | 2+3 = 5 | **+1 byte, +2 cycles** ✗ |
| seed added for the branch: `lsr` + branch | 1+2 = 3 | 2+3 = 5 | **±0 bytes, +2 cycles** ✗ |

(Sizes and base costs read from this repository's own instruction table,
`Gopher2600/hardware/cpu/instructions/definitions.json`: `LSR A` 1/2, any relative branch 2/2 —
+1 when taken, +2 when taken across a page — `JMP abs` 3/3, `LDA #imm` 2/2.)

So the idiom pays **only when the flag is a by-product of work you were doing regardless**. Added
for its own sake it loses on both axes with `lda`, and with `lsr` it costs the same bytes as the
`jmp` it replaced **while running two cycles slower** — there is no configuration in which adding a
seed wins. The source states the one-byte saving without this condition; measured here, the
condition is what does all the work.

**Measured in this repository:** of the 28 uses, **25 have a seed that was needed anyway** — the
branch free-rides on a flag that already exists — and the remaining **3 are all litmus ROMs whose
purpose is to build this exact shape for measurement** (`cb_deadpred:78`, `litmus_6502:88`, `:128`).
**No production use here could be rewritten with the source's cheaper `lsr` seed**, because in all
25 the accumulator value is the thing being stored; `lsr` would destroy it.

## The hazard

**The branch's unconditionality is a property of the instruction ABOVE it, not of the branch.**
Change `lda #0` to `lda mask` and the branch silently becomes conditional: the code still
assembles, still runs, and the line's cycle count changes by ±1 depending on data — which in a
kernel means the frame length moves and the picture rolls on hardware.

The source names the same hazard for the `lsr`/`bpl` seed and prescribes a comment
("assumes A < 128"). A comment is the weak form; see below for the machine form.

**Put the comment on the seed, not on the branch.** All five annotated sites here comment the
*branch* — the line that is safe. The person who breaks this edits the *seed* and has no reason to
look at the branch below it, so the warning sits on the side nobody reads. Write it as
`lda #0  ; the 0 is also the beq's condition below — change one, check the other`.

**And note what a comment-requiring gate cannot do.** A lint that finds seed→branch pairs and
demands a comment goes *silent* at exactly the moment it is needed: change `lda #0` to `lda mask`
and the pattern no longer matches, so nothing fires. It documents the hazard; it does not guard it.
What actually guards it today is the result side — a branch that becomes conditional takes the
other path, which moves the picture (golden frame) and the line's cycle count
(`prove_line_budget`). The proof route below is worth more than the comment route not because it is
stricter but because a re-derived fact cannot go stale, and comments here have.

## Where it is used here (measured 2026-09-03)

**28 sites across 17 files, all under `harness/roms` — none in the works themselves.**
17 have the branch on the very next instruction; 11 have exactly one `STA` in between (`STA`
writes no flag, so the invariant survives it). Forms: `lda #0 → beq` **18**, `lda #<non-zero> →
bne` **10**, `lsr/asl → bpl` **0** (the source's own seed is not used here at all).
Split: `roms/litmus` 9 · `roms/techniques` 19.

**Only 5 of the 28 name the invariant in a comment** (`cb_deadpred.asm:78`, `litmus_6502.asm:88`,
`litmus_deadbranch.asm:94`, `vertical_pos.asm:101`, `vertical_pos_dcp.asm:105`). **The other 23 are
silent** — each is a place where the hazard above is live and undocumented.

The classifier is derived, not asserted: the set of instructions that may sit between the seed and
the branch is **computed from this repository's own CPU** — every `case instructions.<OP>:` block in
`Gopher2600/hardware/cpu/cpu.go` that never assigns `mc.Status.Zero` / `mc.Status.Sign` and never
calls `mc.Status.Load` (which is how `PLP` and `RTI` restore the whole register). That yields 32
non-writing operators; removing the 13 that transfer control leaves the 19 that are safe to skip.
Two independent derivations — the Z-writers and the N-writers — come out as **the same 43
operators**, which is what the architecture says and is therefore a check on the derivation.

## How to verify (proposed — not yet run)

1. **Gate (weak form).** A `scripts/check_*` pass that finds the three seed→branch shapes and
   requires the branch's comment to name the invariant.
   *Negative controls:* strip the comment from `vertical_pos_dcp.asm:105` → must fail; insert a
   seed→branch pair with a comment → must pass; insert one without → must fail.
2. **Prover (strong form).** `internal/cyclebound` already carries abstract state across
   branches — `roms/litmus/cb_deadpred.asm` is its fixture for exactly this shape ("Z is
   statically true here"). If the abstract state at the branch pins the tested flag, the branch
   is *provably* unconditional and no comment is needed. This replaces a convention with a proof.
   *Negative control:* change the seed to a load from RAM → the prover must stop calling it
   unconditional.
3. **Page audit.** `internal/cyclebound` already counts page-cross (`pagecross_test.go`). For each
   of the 28 sites, report whether the branch crosses a page: **a crossing always-taken branch is
   one cycle dearer than the `jmp` it replaced**, and the author should be told, not corrected.

## Sources

Both seeds are the same technique with a different flag:

- **`lsr` → `bpl`** (N): AtariAge 146817 §16, which names the idiom **"simulated BRA — there is no
  BRA instruction, so it is simulated"**: "after an LSR, bit7 = 0 is guaranteed, so BPL can be
  diverted into an unconditional branch = 1 byte less than JMP (3 bytes). But if the LSR is later
  removed the BPL misbehaves → a comment such as '; assumes A < 128' is mandatory."
  〔`reference/atariage/146817-6502-programming-theory/notes.ja.md:16`; the English above is our
  translation of that note — the original thread text has not been re-read〕
- **`lda #0` → `beq`** (Z): our own kernels, e.g. `roms/techniques/vertical_pos_dcp.asm:105`
  ("14 (always taken)").
