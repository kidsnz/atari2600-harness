# Technique — 6502 / kernel micro-idioms (instruction-level tricks: Combat §1-5, mailing list and forums §6-23)

**Source:** studied clean-room from the annotated *Combat* disassembly (Roger Williams' `Combat.asm`), deep-read harvest 2026-07-23 〔Combat.asm kernel + movement + score routines〕. **Reference idioms** (label names + generalized prose only) — *not yet reimplemented / CI-locked*; standalone demo + scenario = TODO. Each is an instruction-level packing / aliasing / branchless trick that saves cycles or bytes in the tightest loops.

## 1. Smuggle a second axis in the LOW nibble of an HMP byte
TIA's `HMPx` latch samples only bits 4-7 (the **high** nibble), so the low nibble is inert to hardware — free storage for a second datum in the same byte.
- A heading table (`Xoffsets` / `HDGTBL`) is **one byte per direction that drives BOTH motion axes**: `STA HMP0,X` sets horizontal HMOVE motion (high nibble), then the *same* byte is reused `AND #$0F / SEC / SBC #$08` to turn the low nibble into a **signed vertical step (−8..+7)** added to the object's Y. One table, one byte, both axes 〔PhMove, Xoffsets, HDGTBL〕.
- **PF0 is a second carrier.** It also shows only its high nibble. Thomas Jentzsch drives two playfield registers from one RAM byte on the right half of a non-reflected PF — `lda PF02Data,x / sta PF0` (bits 4-7) `/ and #$0f` (+2 cycles) `/ sta PF2` (bits 0-3) — answering the worry that a non-reflected PF would add *"another 32 bytes to the RAM usage"* 〔stella-list `200405/msg00068`〕. piledriver put a run-length count in the same spare nibble, taking a 4K game from about 4 screens to 16 〔AtariAge `topic/367878`〕. **Cited only, not verified.** (Not the two-lines-per-byte PF0 packing further down: there the low nibble is the next line's PF0.)
- **Four registers from one byte.** mikes360's rail macro: `lda (railGfxPtr1),y / and (railMaskPtr),y / sta ENAM0 / sta HMM1 / rol / sta ENAM1 / rol / and #%00110000 / sta NUSIZ1`. It holds because *"HM's don't care about the low nybble. Likewise, ENA's don't care about anything except bit1"* (Nukey Shay), so one byte carries HMM1 in bits 4-7, ENAM0 in bit 1, ENAM1 in bit 0 (bit 1 after one `rol`) and the NUSIZ1 missile size in bits 2-3 (bits 4-5 after two) 〔AtariAge `topic/233300`〕. **Cited only, not verified** — the bit positions are traced by hand from the posted code.
- **Idiom.** Any TIA register that ignores some bits is spare storage — pack another field there and mask it out (`AND`) when you consume it.
- ★**Trap: stepping a packed byte with `INX`/`DEX`.** Darrell Spice kept a colour in the high nibble of X with the low nibble zero, and one joystick direction stopped registering 〔stella-list `200404/msg00356`〕: *"DEX decreased the high-nybble by one, but INX increased the low-nybble by one w/out affecting the high-nybble"* 〔`200404/msg00361`〕. From `$A0`, `DEX` gives `$9F` (high nibble `$A`→`$9`) and `INX` gives `$A1` (high nibble still `$A`). His fix: `LSR` four times before the value goes into X, `ASL` four times after it comes back out. The arithmetic is checked; his program is **Cited only, not verified**.

## 2. Store a boolean as `$FF`/`$00` and spend it directly as an AND-mask (branchless blank)
Choose a flag's **representation** so it can be `AND`ed with data to pass-or-zero with no branch: `$FF`/`$00` (byte) or `$0F`/`$00` (nibble).
- `LDA glyph,Y / AND SHOWSCR` blanks the right-hand score when `SHOWSCR=$00`, passes it when `$0F` — no branch in the score kernel.
- The **same** flag pattern masks across subsystems: `AND GameOn` (`$FF` playing / `$00` attract) zeroes engine volume to mute during attract, and gates a timer's increment. One boolean, reused as a universal branchless gate over score, sound, and input 〔VSCOR AND SHOWSCR; DOMOTOR / NoNewGM AND GameOn〕.
- **The loop index as the flag.** ENAMx reads only D1, so if a kernel's index runs through Y = 2,3,6,7,10,11… (graphics spread out in ROM to match), writing Y to ENAMx lights the missile with no test at all; Y = 0,1,4,5,8,9… turns it off the same way (Omegamatrix, for a score kernel) 〔AtariAge `topic/221811`〕. **Cited only, not verified.**
- **Where to keep a flag you test in a hurry: bit 7 or bit 6.** SpiceWare: *"use the top 2 bits for anything time critical as they can quickly be tested for"* — `BIT zp` then `BPL`/`BMI` (bit 7) or `BVC`/`BVS` (bit 6), which leaves A, X and Y untouched; any other bit takes `LDA zp / AND #mask` and a branch, *"the A register, and 2 extra cycles"* 〔AtariAge `topic/344945`〕. From `definitions.json`: `BIT zp` 2 bytes / 3 cycles (`$24`), `LDA zp` + `AND #` 4 bytes / 5 cycles (`$A5`, `$29`). `docs/resources.md` reads TIMINT and PA7 the same way (*BIT TIMINT / BMI expired / BVS pa7*); this is the same instruction turned into a rule for laying out your own RAM. **Cited only, not verified.**
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

## 6. Equal-cost branch arms: swallow the taken arm's instruction with one opcode byte
SpiceWare's paddle read from *Medieval Mayhem* costs the same whether or not the pot has tripped 〔AtariAge `topic/246976`〕:
```
lda INPT0          ; 3
bpl .save          ; 2 not taken / 3 taken
.byte $2D          ; 4 / -   AND abs: eats the next two bytes
.save sty Paddle1  ; - / 3
```
Not taken 2 + 4, taken 3 + 3: 9 cycles either way. The counts are from the engine's opcode table (`Gopher2600/hardware/cpu/instructions/definitions.json`: `$2D` 3 bytes / 4 cycles, `$84` 3 cycles, `$10` 2 cycles; a taken branch adds 1, and 1 more across a page). **Not verified** — the macro was not assembled or run.
- ★`$2D` is **`AND abs`, not `BIT abs` (`$2C`)**: it changes A and N/Z, which is harmless only as long as the code after the macro does not use them (the thread does not show that code). The address it reads is `$PP84` (`$84` = the `sty zp` opcode, PP = Paddle1's zero-page address). A7 of `$84` is 1, so TIA is never selected (the hazard in `docs/fundamentals-audit.md`, *BIT-as-NOP reads*); A12 is bit 4 of PP, so for PP = `$80`-`$8F`, `$A0`-`$AF`, `$C0`-`$CF` or `$E0`-`$EF` A12 is 0 and the read goes to RIOT — its RAM, or its I/O and timer when PP's bit 1 (A9) is set (e.g. `$284` INTIM). With A12 = 1 it reads cartridge space, where RAM ports and hotspots still have to be checked against the scheme. **Not verified** — derived, not measured.
- The harness has the principle (`docs/integration-density-playbook.md`, *"balance branch arms so every path costs the same"*) and the byte-skip for another purpose (`roms/techniques/tia_pcm.asm`, `BIT abs = skip the next 2-byte LDA #0`); this is the one-byte way to make two arms equal.

## 7. One extra cycle for no extra byte: a zero-page access indexed by a known X
When X holds a known constant, bias the address by it: `ldy zp,X` is 2 bytes / 4 cycles against `ldy zp`'s 2 bytes / 3 cycles (`definitions.json`: `$B4`, `$A4`).
```
ldx #LEFT_2                ; 2
ldy ts_loopCount-LEFT_2,X  ; 4 — reads ts_loopCount, one cycle later than `ldy ts_loopCount`
```
Omegamatrix: *"taking advantage of the X register if has a known value. You can easily add 1 cycle delay to the code when storing zeropage"* 〔AtariAge `topic/354058`〕. `design-principles.md`'s `sta.wx HMP0,x` buys a cycle by forcing absolute,X, which costs one more byte; this costs none but pins X. **Cited only, not verified.**

## 8. A branch that will not reach
DASM stops with `error: Branch out of range` (exit 3, measured — `internal/build/build.go`). Three fixes, from three independent replies in 1999:
1. **Invert the condition and `JMP`**: `bne TooFar` becomes `beq Next / jmp TooFar / Next:` 〔stella-list `199901/msg00016` Robin Harbron, `199901/msg00020` Eckhard Stolberg〕.
2. **Branch to a nearby trampoline that holds the `JMP`**: *"it may seem a bit sloppy, but it's an easy way to get more distance out of a branching command"* 〔`199901/msg00017` John Harvey〕. One trampoline can serve many branches — Thomas Jentzsch's size fix for Climber 5: *"replace all but one jmp OverscanWait with branches and branch to the remaining jmp"* 〔`200304/msg00193`〕.
3. **Move code** so the target comes within range: *"This happens a lot when you have no idea where to place all the subroutines"* 〔`199901/msg00017`〕.

What 1 and 2 cost depends on how often the branch is taken, so no single cycle figure is given. **Cited only, not verified.**

## 9. A read-modify-write on a strobe also reads the input at the same address
TIA decodes reads by the low 4 bits, so `$2B` is HMCLR when written and INPT3 when read (`roms/litmus/litmus_floatbits.asm`). vdub_bobby's `asl HMCLR` (2 bytes, 5 cycles) strobes HMCLR and shifts INPT3's D7 into carry, replacing `sta HMCLR / sec` (3 bytes, 5 cycles) 〔AtariAge `topic/74034`〕. The strobe half is measured in this engine for `lsr HMCLR` (`internal/emu/floatbits_test.go`); `asl` is **not verified**.
- ★The carry is only as good as INPT3's D7, a paddle input. Asked whether it can be counted on if VBLANK D7 is never set, supercat answered only *"I wouldn't"*. He also advised against the `lsr` form (D0, a floating bit), and there he gave reasons: bus capacitance, crosstalk, selector boxes, ribbon cables 〔same〕.

## 10. Negating a byte: `eor #$FE` is right only for odd numbers
Found in Climber 5's source as *"make the number negative"*: for even n it gives −n−2 (`2 eor $FE = $FC = -4`) 〔stella-list `200304/msg00193`, Thomas Jentzsch〕. Andrew Davie's two correct forms 〔`200304/msg00201`〕:
```
lda number / eor #$FF / clc / adc #1
sec / lda #0 / sbc number
```
★A bug that survives every test written with odd values. The arithmetic was checked over all 256 byte values when this was written (all 128 odd values right, every even value off by −2). **Not verified** in a ROM.
- **254 − x needs no RAM temp.** For B with A + B = 254, Andrew Davie's `sec / lda #254 / sbc value` needs `value` in memory. Thomas Jentzsch then offered the negation (`eor #$ff / clc / adc #1`) and Davie stopped it — *"That negates a value. That's not what was asked for."* — and Omegamatrix gave `eor #$FF / sec / sbc #1`, which stays in A and *"saves a byte of ram and 1 cycle over subtracting using a ZP ram location"* 〔AtariAge `topic/316989`〕. Davie: *"assuming you have the value in the accumulator to start with. That's cheating"*; Jentzsch: *"your code assumed the value in a variable."* Both are right: 6 cycles against 7 (5 bytes each) holds only when the value starts in A; read from RAM, the `eor` form needs `lda zp` first and is 9 cycles, 7 bytes against Davie's 7 cycles, 5 bytes (`definitions.json`: `$A5`, `$49`, `$38`, `$E9` against `$38`, `$A9`, `$E5`). The formula was checked over all 256 values (decimal mode off); **Not verified** in a ROM.

## 11. "More than one bit set?" in 8 cycles
`b & (b-1)` clears the lowest set bit, so it is non-zero exactly when b has two or more bits set (crispy). zackattack's version with unofficial opcodes 〔AtariAge `topic/280264`〕:
```
lax Input   ; 3  A = X = b
dex         ; 2  X = b-1
sax Output  ; 3  Output = A AND X
```
- ★Input 0 gives 0, which is the right answer to "more than one bit?". mldb first called it a false positive, then withdrew: *"I misread the assignment. I thought it said: if a single bit is set"* 〔same〕. So a zero result means "zero or one bit"; test 0 separately only if the question is "exactly one bit".
- `LAX`/`SAX` are in the family stable on real hardware, where "real hardware" means original NMOS silicon, and a reimplementation such as the Flashback 2 is reported to fail other illegal opcodes (`design-principles.md`, *Stability map for illegal (unofficial) opcodes*). The formula was checked over all 256 values; the code is **Not verified**.

## 12. A one-byte index where a two-byte pointer or a raw PF byte would go
- **Sprite pointers.** Nukey Shay on *Alien*: *"That game uses a single byte for each sprite as a pointer to 2 page long data tables. Slower than (ind),y but more Ram efficient. Another drawback is that sprites car [sic] limited to that 512-byte range"* 〔AtariAge `topic/291287`〕 — asked by someone spending 28 bytes of RAM on 14 pointers with the same high byte. **Cited only, not verified**: one byte reaches 256, and how the second page is reached is not in the thread.
- **Playfield maps.** nanochess, for a vertical scroller: use a mirrored PF so only PF0/PF1/PF2 change, and make each map line *"a single byte in map pointing to a table where to load the PF registers. So each line in your map would use one byte and you could make very long maps"* 〔AtariAge `topic/267694`〕. The byte is an index to a PF triple, not PF data as in the PF0 table below. **Cited only, not verified** — a suggestion, not a shipped kernel.

## 13. Macros: the seams, and the layout they can generate
- **Seams.** Thomas Jentzsch: *"You can save 4 cycles (TAY/TYA) when you call consecutive macros like e.g. Y_POS_ADV and X_POS_GAN"* 〔AtariAge `topic/263329`〕 — presumably (our inference, from TAY + TYA = 4 cycles) a register save and restore that cancel at the join. Read the expanded sequence, not each macro alone. **Cited only, not verified** — the macro bodies are not in the thread.
- **Layout.** To keep one sprite per source line but emit them interleaved in ROM (row 0 of every sprite, then row 1 …, as Bob Whitehead's games store fonts), Andrew Davie's DASM macro takes a row index plus one sprite's rows and emits only that row; a `REPEAT` over the rows produces the interleave, with no `ORG` (the first attempt with `ORG` hit *Origin Reverse-indexed*) 〔AtariAge `topic/298503`〕. The harness has interleaved data (`design-principles.md`, *One interleaved HIRES buffer can feed BOTH players*) and notes reversed `.byte` order (`pkg/sprite/sprite.go`), but nothing that turns a readable source into an interleaved layout. **Cited only, not verified.**

## 14. When nothing else fits in 76 cycles: code in RAM
Thomas Jentzsch on the score and timer display of *Boulder Dash* 2600: *"we used self modifying code in RAM. Else it doesn't fit into the 76 cycles available."* 〔AtariAge `topic/344323`〕 `design-principles.md` lists RAM self-modification among the compromises for an asymmetric PF (*An asymmetric PF is expensive*); this is a second case. The harness detects self-modifying code (`defuse`'s `writes_into_code`; `capability-gap-audit.md`, *Self-modifying code is detectable even when not resolvable*) but has no example of writing it. **Cited only, not verified** — the post does not say what was patched.
- **How big a RAM kernel gets.** DirtyHairy, sizing RAM for an extended 3E cartridge: *"An unrolled kernel is about 16k, double that for double buffering, and maybe double again if you are doing a variation of flicker blinds --- this gives 64k. Put another 32k on top for a huge dynamic world, and you end up with 96k"* 〔AtariAge `topic/279113`〕 — cartridge RAM, against the console's 128 bytes. What the 16k kernel draws is not said. **Cited only, not verified.**

## 15. Count down to zero, and the decrement is the loop test
Kernels store art bottom row first because they index it with a counter that runs down. The reason given for running it down: `DEY` sets Z and N itself, so `dey / bne loop` (or `bpl`, to include row 0) needs no compare, where counting up needs `iny / cpy #rows / bne loop`. The compare is `CPY #` — 2 bytes, and 2 cycles on every pass (`definitions.json`, `$C0`). Glenn Saunders, finding the upside-down data confusing: *"it seems to be the standard way to do it on the 2600, having all the data upside down, probably because BNE is a quicker way to end a loop than testing for a given maximum value"* 〔stella-list `200109/msg00210`〕. `score-kernel.md` and `vertical-positioning.md` state the bottom-up order without this reason. §5 keeps a compare (`EOR #` for `CMP #`) and gets A = 0 from it. The *"probably"* is his; **Not verified** — no kernel was timed for this.

## 16. Counting the set bits in a byte
trogdor needed the number of set bits across 28 bytes for *Stellar Fortress*. Forms posted in the thread 〔AtariAge `topic/90828`〕:

| form | cycles | bytes |
|---|---|---|
| 8 × `lsr / bcc + / iny`, total left in Y (trogdor) | 50 worst | 34 |
| `sta Temp / lda #0`, then 8 × `lsr Temp / adc #0` — no branch (djmips, fixed by vdub_bobby) | 61 always | 36 |
| each nibble looked up in a 16-byte table (trogdor) | about 38 | 39 |
| `PLA` plus a half-byte table, with the illegal `ALR` (supercat) | "about 30" as reported in the thread; not recounted | — |

djmips also posted the mask-and-add fold (`and #$55`, then `#$33`, then `#$0F`), with no count. The first two rows agree with `definitions.json` (`$4A`, `$90`, `$C8` plus a leading `ldy #0`, `$A0`; `$85`, `$A9`, `$46`, `$69`) when no branch crosses a page, and the second was checked over all 256 values with decimal mode off. A 256-byte table would be 6 cycles and was turned down as 1/16 of a 4K ROM. supercat's 16-byte table leaves gaps where other tables can nest (used in Strat-O-Gems for a packed gem-to-colour conversion). jbanes' point comes first: if the program sets the bits itself, keep the count as it sets and clears them. §11 answers the cheaper question, "more than one bit?". **Cited only, not verified.**

## 17. Branch into an operand
`design-principles.md` has Omegamatrix's 8-byte init, in which `bne .loop+1` lands on the immediate operand `$0A` and runs it as `ASL` (*Minimum-byte initialisation*). The same move makes a 5-byte delay of 62 or 63 cycles 〔AtariAge `topic/354058`〕:
```
lda #$0A        ; $0A = ASL
pha
bne *-2         ; tracks guy: 5 bytes, 63 cycles, 8 bytes pushed

lda #$2A        ; $2A = ROL
.loop: asl
bne .loop-1     ; Omegamatrix: 5 bytes, 62 cycles
```
The second touches no stack: *"When the code below is done A=0, Carry is clear, and X and Y are preserved."* Both counts were stepped through with `definitions.json` cycles (`$A9`, `$0A`, `$2A`, `$48`, `$D0` 2 / 3 taken), assuming the branch stays in its page. **Not verified** in a ROM.

## 18. `PLA` reads and advances: the stack pointer as a table index
Replying to someone with no room in the line for `INC palettePtr` (5 cycles, `$E6`), Omegamatrix: *"In these situations I often store one of the digits in ram, and then use PLA to fetch it in the loop. It saves many cycles as you don't have to store to a TEMP variable in the loop"* 〔AtariAge `topic/215618`〕. A later post of Omegamatrix's adds: *"PLA will naturally increment the stack pointer. Use TSX and you can then index your color table. You can place the color table in rom."* `PLA` is 1 byte / 4 cycles (`$68`), `TSX` 1 byte / 2 cycles (`$BA`). The price is SP: it ends each frame somewhere else and needs a `TXS` to put it back (the asker: *"those TXS are ugly but it works"*). `design-principles.md` borrows SP as a spare colour register and `missiles-bullets.md` points it at ENAMx for `PHP`; here it is a read pointer that the fetch advances. **Cited only, not verified.**

## 19. Keep the loop counter in RAM to free X and Y
glurk, for someone trying to fit ten `COLUPF` changes into each line: *"I used a RAM variable as the counter, and used X and Y registers as needed. It reads the colors from $90-$99 and makes a 10-color rainbow thing"* 〔AtariAge `topic/346865`〕. The counter then costs `DEC zp` (2 bytes, 5 cycles, `$C6`) where `DEY` costs 1 byte and 2 cycles — 3 cycles a pass (our arithmetic), with the branch still reading Z from the `DEC` — and both index registers are free for the line. **Cited only, not verified** — the demo was not read.

## 20. A subroutine's final `RTS` is a 12-cycle delay
SpiceWare's positioning routine ends `SLEEP12: rts`: *"That SLEEP12 label at the end is used for wasting 12 cycles by doing a jsr SLEEP12"* 〔AtariAge `topic/233692`〕. Any subroutine's closing `RTS` already is one; the label only names it. The cost — 12 cycles, 3 bytes — is the row `jsr` + shared `rts` in `known-traps.md` (*The delay table, measured*), measured in this engine by `internal/emu/delaytable_test.go`; the call also uses two bytes of stack, which that test does not check.

## 21. Initialise the top of RAM by pushing
jeremiahk found the bytes for music in his 512-byte demo *Acid Rain* by *"Putting values at the end of RAM so they can be initialized by pushing values on the stack"* (tjoppen's words) 〔AtariAge `topic/280442`〕. After a clear that leaves SP = `$FF` (the 8-byte init in `design-principles.md` does), `lda #v / pha` writes `$FF`, then `$FE`, and so on; `PHA` is 1 byte where `STA zp` is 2 (`$48`, `$85`). The variables then sit above SP, where later `JSR`s do not reach them (our reading). **Cited only, not verified** — the demo's source was not read.

## 22. Transpose the playfield buffer so every column shares one Y
Mr SQL's kernel read one line's six PF bytes from consecutive RAM — `lda $00A0,y / sta PFn / iny`, six times (PF0, PF1, PF2, then again) — and on each repeated line rewound Y with `tya / sec / sbc #$06 / tay`. Tjoppen: *"You should store the data transposed in RAM, so you can access each column of playfield with the same Y value"* 〔AtariAge `topic/203463`〕 — six tables `PF0DataL` … `PF2DataR`, each read `,Y` with the same Y, which takes six `INY` (12 cycles) and the 8-cycle rewind out of the line; he estimated *"20-25-ish cycles to spare"*. The price is the layout: Mr SQL's buffer engines wrote row by row, and transposing without rewriting them would have cost 30 more bytes of RAM. §3 folds a constant offset into a pointer; this folds a stride into the layout. **Cited only, not verified.**

## 23. The line counter's low bit as a draw/logic switch
ScumSoft gives alternate lines to drawing and to game logic by the low bit of the line counter 〔AtariAge `topic/178066`〕:
```
PF_LOOP:   tya
           eor #1
           and #$01
           beq PF_LOGIC   ; odd Y: logic, with the graphics blanked
           ...            ; even Y: draw
PF_Return: dey
           sta WSYNC
           bne PF_LOOP
```
In overscan a toggled byte starts Y at 191 on one frame and 192 on the next, so the two sets of lines trade places each frame, and `TIM64T` gets 33 or 34 to hold 262 lines (his comments). He calls it interlaced; it is not TV interlace (`design-principles.md`: true interlace is not in this repository) — each line is drawn every other frame (our reading; he says *"I find this gives minimal flicker"*). `eor #1` only inverts the test: `tya / and #$01 / bne PF_LOGIC` takes the same branch in 2 fewer bytes and 2 fewer cycles (our reading). Three days later ScumSoft withdrew it: *"never mind the first post here, I found a much better way to interlace the frames. Much of that code was not really needed at all"* 〔same〕; that post does not show the better way. **Cited only, not verified.**

## Packing PF0's nibbles: +3 cycles a line, or +19 written the obvious way (2026-09-07)

PF0 uses only its high nibble, so a two-line pair fits in one byte. `design-principles.md` has the
extreme — *"not drawing PF0 … frees 12cy per line + 18 bytes of RAM"* — and this page has nibble
packing as a general idiom. The middle had no line anywhere, though Ben Larson was doing it in 2002
and finding it awkward: *"The fact that I'm **packing both PF0 nibbles into one byte** probably
doesn't help the situation"* 〔`200210/msg00045`〕.

Measured over four kernels (`internal/cyclebound/pf0nibble_test.go`), the last two unrolled across two
scanlines:

| kernel | Krow cycles |
|---|---|
| one line per iteration, one byte per line | 22 per line |
| one line per iteration, packed, parity tested at run time | 41 per line — **+19** |
| two lines per iteration, one byte per line | 11 + 22 = 33 per pair |
| two lines per iteration, packed, odd nibble via a table | 11 + 28 = 39 per pair — **+6, so +3 a line** |

★**The same idea costs +19 or +3 depending on how it is written** — a factor of six, because unrolling
removes the parity test entirely: the two halves of the pair know which nibble they are.

★★**The unrolled control is what makes that readable.** The first version of this measurement compared
a single-line unpacked kernel against an unrolled packed one, and made packing look *cheaper* than not
packing. The saving was the loop overhead halving and had nothing to do with nibbles.

★★★**What it buys and costs:** 16 bytes of RAM for a 32-line band, +3 cycles a line, 256 bytes of ROM
for the shift table. Against the extreme — abandoning PF0 *gains* 12 cycles a line and 18 bytes — so
packing is only the right answer when PF0's content is actually needed. Found by the mailing-list
distillation (helper-1).
