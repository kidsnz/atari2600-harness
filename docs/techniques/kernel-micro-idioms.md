# Technique — 6502 / kernel micro-idioms (Combat instruction-level tricks)

**Source:** studied clean-room from the annotated *Combat* disassembly (Roger Williams' `Combat.asm`), deep-read harvest 2026-07-23 〔Combat.asm kernel + movement + score routines〕. **Reference idioms** (label names + generalized prose only) — *not yet reimplemented / CI-locked*; standalone demo + scenario = TODO. Each is an instruction-level packing / aliasing / branchless trick that saves cycles or bytes in the tightest loops.

## 1. Smuggle a second axis in the LOW nibble of an HMP byte
TIA's `HMPx` latch samples only bits 4-7 (the **high** nibble), so the low nibble is inert to hardware — free storage for a second datum in the same byte.
- A heading table (`Xoffsets` / `HDGTBL`) is **one byte per direction that drives BOTH motion axes**: `STA HMP0,X` sets horizontal HMOVE motion (high nibble), then the *same* byte is reused `AND #$0F / SEC / SBC #$08` to turn the low nibble into a **signed vertical step (−8..+7)** added to the object's Y. One table, one byte, both axes 〔PhMove, Xoffsets, HDGTBL〕.
- **Idiom.** Any TIA register that ignores some bits is spare storage — pack another field there and mask it out (`AND`) when you consume it.

## 2. Store a boolean as `$FF`/`$00` and spend it directly as an AND-mask (branchless blank)
Choose a flag's **representation** so it can be `AND`ed with data to pass-or-zero with no branch: `$FF`/`$00` (byte) or `$0F`/`$00` (nibble).
- `LDA glyph,Y / AND SHOWSCR` blanks the right-hand score when `SHOWSCR=$00`, passes it when `$0F` — no branch in the score kernel.
- The **same** flag pattern masks across subsystems: `AND GameOn` (`$FF` playing / `$00` attract) zeroes engine volume to mute during attract, and gates a timer's increment. One boolean, reused as a universal branchless gate over score, sound, and input 〔VSCOR AND SHOWSCR; DOMOTOR / NoNewGM AND GameOn〕.
- **Idiom.** Match a boolean's bit pattern to how it's consumed; an all-ones / all-zeros flag is a free conditional-blank (0 cycles of control flow).

## 3. Pre-bias a table pointer by the loop's fixed offset to delete a per-scanline add
If a kernel indexes a table with a counter that is always a **constant** higher than the data row, bake that constant into the stored pointer instead of adding it every line.
- The playfield column pointers (`PLFPNT`) each point **4 bytes before** the real data (`PF0_0-4`, `PF1_0-4`, `PF2_0-4`). The kernel's `LDA (LORES),Y` uses a Y that is naturally `+4` (the top 4 skipped lines); the two cancel, so the tightest loop never pays a `+4` add. (Author's note: "these addresses point 4 bytes before the real start of data.") 〔PLFPNT, LORES, InitPF〕
- **Idiom.** Fold a constant index bias into the **pointer constant**, not into the loop body.

## 4. Vertical playfield mirror by EOR-ing the scanline counter
Draw one PF map symmetrically top **and** bottom by reflecting the *index*, not duplicating data. The scanline counter's sign bit picks the half; for the bottom half `EOR #$F8` mirrors the fetch index so the same column data is read symmetrically:
```
LDA ScanLine
BPL VvRefl        ; top half: use index as-is
EOR #$F8          ; bottom half: reflect the index
VvRefl: … LDA (LORES),Y
```
〔Vfield BPL VvRefl / EOR #$F8〕
- **Idiom.** Vertical symmetry costs **one EOR on the loop counter**, not a second table. (Distinct from sprite 180° mirror via `REFP` + reverse-copy — this is the playfield index-reflect mechanism.)

## 5. Compare-via-EOR at a loop exit to harvest A=0 for the teardown clear
When a loop terminates on a specific counter value, exit with `EOR #value` instead of `CMP #value`: at the terminating value the EOR yields `0` and falls through with **A already zero**, which the next block spends directly to clear registers — saving the `LDA #0` before the tear-down.
```
EOR #$EC
BNE Vfield        ; not the last line → keep looping
; falls through with A = 0:
STA ENAM0 / STA ENAM1 / STA GRP0 / STA GRP1 / STA PF0 / STA PF1 / STA PF2
```
〔VnoPF EOR #$EC → STA ENAM0..PF2 clear block〕
- **Idiom.** If a loop's exit value is a constant **and** the next thing you do is zero registers, `EOR`-compare hands you the `0` for free — `CMP` would leave A holding the old counter value.
