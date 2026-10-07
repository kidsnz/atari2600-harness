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
| `lsr` (accumulator) | N = 0 (a 0 is shifted into bit 7) | `bpl` |
| `lda #<non-zero immediate>` | Z = 0 | `bne` |

`asl` is **not** a seed: after it, N is the old bit 6 (`$40` → `$80`, N = 1), so a following `bpl`
can fall through — *"if you shift and THEN do a BMI, you're actually checking D6"*
〔stella-list `199806/msg00116`〕. The engine agrees: `Gopher2600/hardware/cpu/cpu.go` sets `Sign`
from the shifted value.

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

**A carry seed, in an arm written for constant timing.** Glenn Saunders, 2001, checking a missile
routine of Thomas Jentzsch's for *"a constant 30 cycle timing"*, ended its two out-of-line arms with
branches back to `.continue`: `.enable` with `lda #2 / sta ENAMx / bne .continue` (the table's
`lda #<non-zero>` → `bne`, one `sta` between), `.disable` with
`lda #0 / sta ENAMx / sec / nop / bcs .continue`, the `sec` commented *"keep carry state constant"*
and the `nop` *"waste 2 cycles for constant timing"*; he counts each closing branch `+3`
〔stella-list `200111/msg00155`〕. `sec` → `bcs` is a fourth seed (C = 1), not one of the three in the
table above. The arm wanted the padding, so the `sec` is a seed needed anyway: `sec` + `bcs` is 3
bytes and 5 cycles against 4 bytes and 5 cycles for `nop` + `jmp` (our reading). `.disable` is
entered only by a taken `bcc .disable` and `lda` / `sta` leave C, so the carry is already clear
there and a `bcc .continue` would be unconditional without the `sec`; what the `sec` adds is the
same carry the `.enable` arm leaves — that arm is entered by a taken `beq` after `sbc`, so C = 1
there and nothing in it writes C — while the normal path's two `asl` leave C at bit 6 of the byte it
loaded, so C is still not known after `.continue` (our reading). Jentzsch: *"At the start of .enable
you put 14 cycles, but you only need 10 to get there"*; dropping the normal path's `nop` and
*"removing the SEC (which I was counting as +2)"* gives *"Result: 26 cycles"* — which `sec` is not
said, and the routine has one before `sbc M0_Y` too. Glenn: *"I'm not confident removing the SECs
right now. I don't know what the condition of the carry flag will be in all cases. So it will have
to stay at 28 cycles for now."* Jentzsch: *"Just do your kernel(s) with SEC and we will help you
removing them"* 〔`msg00157`, `msg00159`, `msg00160`〕. **Cited only, not verified.**

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

**A seed with no other job: `clv` → `bvc` (V = 0).** A 2009 thread on how branches assemble gives `CLV`
/ `BVC` as the stand-in for the unconditional branch the 6502 lacks — 5 cycles, and position independent
as long as the branch and its target are moved together in one block, since a relative branch carries no
absolute address — and, as our notes put it, one byte smaller than `JMP`; Nukey Shay's use is a display
or positioning kernel that has exactly 2 cycles to burn, where it takes the place of a `NOP` and a `JMP`
and saves a byte (our reading of the notes) 〔AtariAge `topic/140713`; held here only as distilled notes,
so the wording is not checked〕. Against a bare `jmp abs` the table above gives no byte: `clv` is a seed
added for the branch, 1 byte and 2 cycles in `definitions.json`, so `clv` + `bvc` is 3 bytes and 5
cycles against 3 and 3 — the `lsr` row's ±0 bytes, +2 cycles. Against `nop` + `jmp`, 4 bytes and 5
cycles, it is −1 byte at the same 5: when the 2 cycles are wanted, the `clv` is padding and so a seed
needed anyway, as the `sec` in the `sec` → `bcs` arm above is (our arithmetic). Those 5 cycles are for a
`bvc` that stays on its page; taken across one it is 4 (the crossing row of the first table), so the
pair is 6, a cycle dearer than `nop` + `jmp` (our arithmetic). V stays clear only until an instruction
that writes it — `ADC`, `SBC`, `BIT`, `PLP`, `RTI`, and some undocumented opcodes in
`Gopher2600/hardware/cpu/cpu.go` — comes between (our reading). **Cited only, not verified.**

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

**The seed can be a whole path away.** Wade Brown, 2003, ended a routine with
`BCS NextByte ; ALWAYS Taken`. Andrew Davie: *"The last comment appears to be incorrect. I can see
situations where the carry will be clear at this point."* The path Davie gives runs through
`AND #$C0 / CLC / ROL / ROL / ROL`, after which *"the carry would be clear (since you roll off the
top two bits, and then one you have guaranteed to be 0)"*, and on that path *"the BCS at the end
\*won't\* be taken and you will instead execute the next function StorePatVal inadvertently."* His
moral: *"If you're going to use unconditional branches be really really really sure of your
"uncondition"!"* Wade had asked about a bug that *"causes the alien to drift slowly downward"*;
Davie: *"The above probably isn't the problem, but it is A problem"*, and the thread does not come
back to the drift 〔stella-list `200305/msg00032`, `msg00033`〕. As posted, that path then re-reads
the same non-zero table byte with the same X, so in that pass it does not reach the `BCS` (our
reading); the conclusion stands on the other paths in. No single instruction above that
branch is its seed: the carry is whatever the last carry-writing instruction on each path into it
left (our reading). **Cited only, not verified.**

## When the crossing is the part

Everything above treats the page-crossing cycle as a cost. Three of these sources use it, or a skip
byte, as a part, and one asks for it; two of them use it to make two paths cost the same. **Cited
only, not verified** — none of it was assembled here.

- **Branching over a 2-cycle instruction.** SeaGtGruff, 2016, allowed that the extra cycle *"could
  be used to advantage"* but thought it *"probably so rare and so difficult to manage ... that it's
  probably best to forget about entertaining such thoughts."* Nukey Shay, in the next post: *"Taking
  advantage of the added cycle is not so rare when you need to decide whether to branch over a
  2-cycle instruction. That leaves either case at 4 cycles. Handy for display kernels when cycle
  time must be precise."* By the cycle table under *Economics* and the instruction-table note below
  it: taken across a page, 4; not taken, 2 + the skipped instruction's 2, also 4 (our count; that
  the taken branch is the one that crosses is our reading) (AtariAge `topic/250652`).
- **Making the assembler insist on the crossing.** The same-page macros that stop the assembly when
  a branch leaves its page (`sbne` and the rest, John Payson's, posted by SpiceWare) are in
  `tool-landscape.md` and `known-traps.md`. In the `topic/264527` post that `tool-landscape.md`
  cites, SpiceWare also gave the inverse set, `dbcc` … `dbvs`, credited to himself: the branch, then
  `if ((* ^ {1}) & $FF00) = 0` / `echo "SAME PAGE","WARNING ",{1}," at ",*` / `err` / `endif`, so
  the build stops when the target is on the *same* page — *"I made a variation set of them as dXXX
  to confirm branch to a different page as I needed the extra cycle to occur one time."* He was
  answering enthusi, who wanted *"100% control of all bytes"* and disliked macros; enthusi's reply:
  *"admittedly that is a nice solution but it affects the source code"* — he would rather patch the
  assembler to honour a `;nocross` comment — *"Then again, your version works, mine is just a 'what
  if' in my head"* (AtariAge `topic/264527`, 2017).
- **Asking for it as a constraint.** Kylearan, 2017, in a thread on the `BOUNDARY` macro in
  `macro.h`, wished for *"an assembler/linker that takes care of code and data placement for you,
  observing such constraints as "align 256 offset PLAYER_HEIGHT" or "must not cross a page boundary
  but otherwise can be anywhere" or "this conditional branch must cross a page boundary""*, then
  *"(Almost there...)"*, which the thread does not explain. Thomas Jentzsch replied *"See above:
  COND_ALIGN_FREE(_LBL)"* — his macro from earlier in the thread, which aligns a block only when it
  would otherwise run onto the next page; that meets the do-not-cross constraint, not the must-cross
  one (our reading of the macro as posted) (AtariAge `topic/267367`).
- **Skipping instead of branching: `$0C`.** Eric Ball, 2003, on the RobotCity skipdraw Dennis Debro
  quoted (`bcs .doDrawP0 / lda #0 / NOP_W / .doDrawP0: lda (ptrP0),y / sta.w GRP0`): *"The key is
  the NOP_W, which is an equate for $0C, which is the opcode NOP abs. (An illegal/undocumented
  version of NOP. The documented way is to use $2C which is BIT abs) So if the bcs is not taken the
  lda (ptrP0),y gets absorbed into the NOP and is not executed."* His count: not taken 2 + 2 + 4 =
  8, taken 3 + 5 = 8 — *"always the same number of cycles"* 〔stella-list `200309/msg00060`〕. The
  taken count assumes neither the `bcs` nor the `(ptrP0),y` load crosses a page (`definitions.json`
  marks both page-sensitive); with a crossing it is 9 or 10, while the not-taken path pays neither
  (an untaken branch is 2 wherever its target is; `NOP abs` is not page-sensitive) and stays 8 (our
  reading). Jentzsch's own account of the trick, the next day, is *"the setup of yPosP0 (and ptrP0!)
  and the arrangement of the data (starting \*inside\* a page) in the ROM"* 〔`msg00069`〕. The two
  skip bytes differ in the flags: the engine's `BIT` case writes N, V and Z, and its `NOP` case does
  nothing (`Gopher2600/hardware/cpu/cpu.go`), so `$0C` is the one to use when N, V or Z set before
  the skip is read after it (`BIT` does not write C) — as in Jentzsch's *Thrust*
  (`integration-density-playbook.md`, 〔`200103/msg00052`〕). Nothing in the quoted lines, which end
  at `sta.w GRP0`, reads N, V or Z. The `$2C` skip is measured in `integration-density-playbook.md`
  §G; `$0C` is not.

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

A third seed and a third source, cited under *Economics* above, not under "both" here:

- **`clv` → `bvc`** (V): AtariAge `topic/140713` (2009), held here only as distilled notes, so the
  wording is not checked; the byte it saves holds against `nop` + `jmp`, not a bare `jmp`.
