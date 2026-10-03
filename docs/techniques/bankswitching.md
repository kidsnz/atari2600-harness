# Technique — bank-switched game structure (F8 template)

**Where the whole problem comes from** (added 2026-09-04): *"Atari limited the cartridge
connector to 24 pins, omitting read-write and clock lines for RAM, as well as lines for
addresses greater than 4096."* Hotspots, the split read/write ports of cartridge RAM and the
4K ceiling all descend from that one choice — and the designers disowned it: *"Mr. Miner and
Mr. Decuir agreed in retrospect that this decision was a mistake, since a 30-pin connector
would have cost only 50 cents for each VCS and 10 cents a cartridge."* 〔Perry & Wallich, "Design case history: the Atari Video Computer System", IEEE Spectrum 1983-03 pp.45-51〕

**Goal:** the structural template for games larger than 4K: per-bank reset stubs + vectors, a
reusable cross-bank call trampoline, and the data-bank pattern (level assets loaded from
another bank into RAM).

Demo: `roms/techniques/banked_game.asm` (F8 8K; bank 1 holds level data + loader, bank 0 runs
the game and switches levels every 120 frames).
CI: `scenarios/banked_game.json` (cross-bank load contents, level switch, bank.number at frame
boundary, 262, golden).
Hardware basis: `litmus_bank` / `_f6` / `_f4` (v0.43.0; hotspots, AUTO fingerprint, per-bank
vectors all verified).

## What A12 actually is, and what every scheme is answering

The connector omitted *"lines for addresses greater than 4096"*, and the consequence is concrete:
**on a plain 4K cartridge, A0–A11 go straight to the ROM and A12 goes to the ROM's chip-enable.**
The 6507 has thirteen address lines; the thirteenth is not an address as far as the cartridge is
concerned, it is *"are you being talked to."* Chris Wilkson, stella-list 1999, adding the detail that
turns this into wiring: **mask ROMs have active-HIGH CE/OE and a standard EPROM (2716) has active-LOW
/CE and /OE**, so building a cartridge from an EPROM means **putting A12 through an inverter**.

That single fact is behind a set of otherwise unrelated observations from the archive: why homebrew
cartridge PCBs carry a **7404 hex inverter** and nothing else logical; why **2532 wants OE high while
2732 and EEPROMs want it low**; and why a "double-ender" — one board, two 4K games — works by
**tying A12 to Vcc on one edge connector and to GND on the other**.

**And it reframes bank switching itself.** A larger ROM needs address pins A12…A(n) that the console
does not provide, so *every* scheme is an answer to one question: **who supplies the value for those
extra pins?**

| scheme | who supplies it |
|---|---|
| F8/F6/F4 | the ROM itself, by touching a **hotspot address** |
| FE | the **stack** — `$01FE` on the bus, whatever instruction put it there (a JSR does, and so does an RTS), then the data bus one cycle later (no hotspot at all; AtariAge `topic/268780`). The engine (`mapper_scabs.go`) reads it as `data >> 5`: `%111` selects bank 0, `%110` bank 1, and ignores any other value. A 2019 hardware test on a Space Shuttle cart reports that bit 5 alone decides — set selects bank 0, clear bank 1, so `%000` selects bank 1 too (AtariAge `topic/293982`; the patent latches the top three bits as A13–A15, AtariAge `topic/266200`) — **Cited only, not verified**. If so, the engine can differ from the cart when a JSR or RTS lands below `$C000`, e.g. code running from RAM. |
| Supercharger | a **stateful arming sequence**: `$F0xx` arms it and latches that low byte as the value; the write lands on the **fifth subsequent address transition**, not the next access (the engine sets `Delay = 6`, decrements it only when the address actually changes, and commits at `Delay == 1`) |
| double-ender | **the connector**, wired once and never changed |

> **The Supercharger's "fifth" is derived, not traced.** It follows from the engine's arithmetic
> (`Delay` is set to 6, decremented once per address *transition*, and the write commits at
> `Delay == 1` — five decrements) and from alex_79's 2013 description on AtariAge `topic/215572`,
> which cites Kevin Horton's notes and the Cuttle Cart manual and says the value lands on "the
> fifth address touched afterwards". Two independent readings of the engine and one independent
> reading of the arithmetic agree. **What nobody has done is follow an execution**: whether
> `registers.Listen` (the decrement) or `Access` (the `Delay == 1` commit) runs first within a
> single bus event is unverified, and an off-by-one could hide there. `roms/litmus/litmus_superchip.asm`
> is the pattern for settling it; until then this row is the best available inference, not a measurement.

**The Supercharger's control byte has three bits that are not the game's.** Eckhard Stolberg's advice
on AtariAge `topic/221197`: the SC BIOS works out the write-pulse delay at start-up and leaves it, with
the current bank configuration, at `$80` when it hands over, so keep the top three bits of that byte
and OR the bank configuration into it instead of writing them as zero. It was offered as *"maybe it
might be worth a try"*, and the same thread later traced that bug to the value read from a hotspot
(below). Advice, not a measurement: **Cited only, not verified.** The engine decodes those bits as `WriteDelay` (`registers.go`:
`r.WriteDelay = int((v >> 5) & 0x07)`) but nothing reads the field — the delay is the fixed `Delay = 6`
above — and its fast-load puts the load block's own config byte at `$80`, so a ROM that zeroes the bits
runs the same here and cannot be caught by it. Read from the code; **Not verified**.

**The memory that sequence writes is three 2K banks, shown two at a time.** Chris Salomon, adding Starpath
support to Stella in 1997 from the documents he had: *"StarPath 6K cartridge, 4 banks possible, 3 banks
(RAM-Module) used,"* *"1 bank Starpath ROM (4th bank), 2 K each bank!"*, with bits D4–D2 of the control byte
choosing which bank appears at `$F000` and which at `$F800` — eight combinations, the ROM only ever in the
`$F800` slot — and of whether all 6K is RAM, *"dunno for sure, since some documents try to tell us
otherwise"* 〔stella-list `199703/msg00213`〕. **Cited only, not verified.** The engine has the same shape:
`supercharger/state.go` holds `ram [3][]uint8`, each `bankSize = 2048` (`supercharger.go`: *"supercharger
has 6k of RAM in total"*), and `GetBank` maps the eight values of `BankingMode` (`(v >> 2) & 0x07`) to the
same pairs as his table, the BIOS only ever in the `$F800` half. Read from the code; **Not verified**.

**A scheme can also have an input with no defined result.** A Rentacom 2-in-1 cartridge that no
emulator ran was worked out from its board — two 74LS10s forming a NAND SR latch — by alex_79 on
AtariAge `topic/293883`: any address with A12=0, A9=1, A6=0, A5=1 selects bank 0; A12=0, A9=1, A6=1,
A5=0 selects bank 1; and A12=0, A9=1, A6=1, A5=1 is the latch's undefined state, so it *"may or not
cause a bankswitch."* A list of hotspots cannot state that third row. **Cited only, not verified.**
The same post notes that these addresses include the ones UA games use. The engine's UA mapper
decodes the same four lines — `mapper_ua.go` `AccessPassive`: `switch addr & 0x1260 {` — and switches
only on `$0220` (bank 0) and `$0240` (bank 1), so the third combination (`$0260`) never switches there:
the engine picks one answer to a question the latch leaves open. Read from the code; **Not verified**.

**F8's two hotspots, `$1FF8` and `$1FF9`, are Kevin Horton's, and a dumper's builder doubted them.** Horton's `sizes.txt`
(V6.00, 1997-04-18): *"Accessing 1FF8 switches in the first 4K, and accessing 1FF9 switches in the last 4K."*
The engine decodes exactly those two (`mapper_atari.go`, `atari8k.bankswitch`:
`if addr >= 0x0ff8 && addr <= 0x0ff9 {`, `$0FF8` to bank 0 and `$0FF9` to bank 1; read from the code).
In 2003 Adam Wozniak, building a cartridge dumper, wrote that *"$1FF8 and $1FF9 do not seem to be the only hot spots on F8 carts; $1FF7 also
seems to be active (and many others). I don't think the hardware looks at the whole address bus to switch
banks"* 〔stella-list `200301/msg00118`〕. Eckhard Stolberg replied: *"In F8 bankswitching only $1FF8 and
$1FF9 are supposed to be hotspots. But for some carts the timing is a bit sensitive"* — on his 7800-based
reader some F8 carts *"would switch banks at random addresses"* at 7800 speed, until he read them with a
routine running at 2600 speed 〔`200301/msg00121`〕. **Cited only, not verified**; how a cart decodes its
hotspots is not something this engine can show.

**The FE row replaced a documented mechanism, and stephena said its unpicturable hardware should have been the first clue.**
tomson's FE cart crashed Decathlon and Space Shuttle on real consoles until he set aside the
documentation he had read — opcodes detected on the bus, the previous or next cycle's address — for
*"the most easy thing to have implemented"* in hardware, waiting for `$1FE` and latching the next
cycle's data bit; that ran every FE game he had on all his consoles, the 7800 included. dirtyhairy
called Kevin Horton's canonical document *"just plain wrong on this one"*, and stephena, who then
rewrote Stella's FE, said of it: *"While I understand what it is saying, I couldn't picture how the
hardware was actually implementing it. And that should have been the first clue that docs are
incorrect."* (AtariAge `topic/268780`, 2017). **Cited only, not verified.**

**FE as the list described it in 2002, and the cases it worried about.** Eckhard Stolberg: *"Games that use
FE bankswitching have one 4K bank compiled for $F000 and one 4K bank compiled for $D000"*, so from the
`$F000` bank *"you could do a "JSR $D123""*, and get back with a `JSR $F456` or an `RTS` 〔stella-list
`200212/msg00185`〕. The engine's rule in the row above agrees: `$D1` is `%110`, bank 1, and `$F4` is `%111`,
bank 0 (read from the code). His cart-side account then was not the row's: wait for an address in the
stack area `$0100-$01FF`, check that the next is there too, then read D5 〔`200212/msg00182`〕. Christopher
Tumber named two other ways a JSR/RTS pair turns up — *"to waste 12 cycles"*, and a subroutine built in RAM
that ends up as nothing but an `RTS` — and Thomas Jentzsch added *"that BRK-to-subroutine trick used at
least in several Parker games"* 〔`200212/msg00184`, `200212/msg00183`〕. Stolberg's answer was that in an
FE image every `JSR` and `RTS` is meant to switch, and that FE games use no `BRK` or `RTI`
〔`200212/msg00185`〕. **Cited only, not verified.** Read here, not stated there: the RAM case is the one the
row above flags, a JSR or RTS whose address lies below `$C000`.

**Some 7800s break FE.** Eckhard Stolberg, 2002: early 7800s had problems with some SuperChip games, so
*"Atari added a little circuit to the later models of the 7800 that changes the signal timing on the bus.
This made the superchip games more reliable, but broke the Activision games with FE bankswitching for
example"* 〔stella-list `200211/msg00187`〕. In 2004 he said the fix *"broke compatibility with Activision's
FE bankswitching and the Supercharger"*, and that *"some people disabled the extra hardware (only requires
removing a certain capacitor)"* 〔`200409/msg00115`〕. Neither
gives a year or a model, so "later" is known only by this behaviour; tomson's FE cart above ran on his 7800,
model not stated. **Cited only, not verified.**

**Above A12 there is nothing at all, and that is a resource.** The 6507 has thirteen address lines,
so A13–A15 of a 16-bit pointer are **never emitted** — measured 2026-09-04, the same ROM byte reads
back from all eight odd 4K windows:

```
$1000 $3000 $5000 $7000 $9000 $B000 $D000 $F000   -> all $78
```

Three people put the three halves of this on the list in 1997, and it takes all three to make it
safe to use. Erik Mooney gave the ladder — *"the upper 3 bits are completely unused by the 6507 and
the bus"*. Eckhard Stolberg gave the convention — *"setting all unused bits to 1 is the VCS
standard"*, which is why everyone writes `$F000`. And Greg Miller gave the guarantee, which is the
part that matters: **the upper bits are not merely ignored, they are never driven**, so *"no other
part of the system can detect the difference."*

That is what makes the trick safe: **a 16-bit pointer in RAM has three bits you may use for anything**
— flags, a small counter, an object type — and they cost **no instruction to strip**, because the
hardware never looks at them. Unlike ordinary bit-packing, there is nothing to mask off on the way
out. (Portability caveat: the 7800's 2600 mode does give those bits meaning.) One published use is a
**bank number**: SvOlli's `bankjsr`/`bankjmp`/`bankrts` macros (from *Bang!*) carry the target bank in
the top three bits of the destination address, and a back end copied to the same address in every bank
shifts it out and masks it — `and #%111` for F4, `%011` for F6, `%001` for F8 (AtariAge `topic/319899`;
**Cited only, not verified**).

A caution the same thread supplies: **the two A12s are different pins.** The console's A12 is a
chip-enable *output* as far as the cartridge sees it; a bank-switched cartridge's A12 is an address
*input* on a bigger ROM, driven by the decoder. Same name, opposite role — the fourth same-name
collision the mailing-list distillation has catalogued this week (`asr`/`alr` as spellings,
`absolutex` covering zp and abs, two files called `fingerprint.go`, and now A12).

Found by the mailing-list distillation (helper-1), who assembled it from four separate threads and
then found the author had stated the conclusion himself further down the one they were reading.

**A fifth for that catalogue, of another kind: a name borrowed from another machine.** Virtual World BASIC calls a
routine a *"display list interrupt"*; its manual draft says they *"run during the vertical blank"*,
and a program calls one with `gosub DLI` to update a few tile rows of the screen (AtariAge
`topic/251949`). Read here, not stated there: the 2600 has no display list, and its 6507 hears no
interrupt (`docs/capability-gap-audit.md`, the RIOT timer-wrap item), so the name fits neither half of
what runs. Even on the Atari 8-bit, where the name comes from, unbibium noted that the effect *"can be
done with an ordinary display list, and not an interrupt."* **Cited only, not verified**; the demo was
not run.

## The three standard pieces

1. **Identical reset stub + vectors in every bank** (`$FFE0: lda $FFF8 / jmp $F000`,
   vectors → $FFE0): whichever bank is mapped at power-on, you boot into bank 0.
   *Without a stub* (tjoppen, F4, AtariAge `topic/195113`): put the start code in the last bank, point
   every other bank's reset vector at `$1FFB`, and put `$4C` (`JMP abs`) there. Fetching that opcode
   touches the last bank's hotspot, so the operand at `$1FFC-$1FFD` is read from the last bank and names
   its start — the same path as waking up there; stated cost 18 bytes per bank. batari's variant, which
   works on edge- and level-triggered carts alike: `BRK` at `$1FF3` with the reset vector pointing at
   it; BRK's discarded fetch of `$1FF4` — an F4 hotspot, not an F8 or F6 one — selects the first bank,
   whose BRK/IRQ vector names the start. Both put code on hotspot addresses, which the same thread
   calls safe only when that byte is identical in every bank. **Cited only, not verified.** Read from
   the code; **Not verified**: `internal/emu.TestEveryBankCanBeBootedInto` would fail such a ROM
   although it boots — a false alarm — because `stubSelectsBank` looks only for an absolute
   `lda`/`sta`/`bit` of a hotspot after the reset vector.
   *Some carts put code on the hotspots, and the byte read there is not settled.* TomSon, who implemented
   the mappers in software for his Tiara cart, names Man Goes Down and Ms. Hack as putting code in the
   bank registers with the reset vector pointing at it, and reports that hotspot accesses are not one
   kind: most carts read without caring for the value, some write as the trigger, some mix both, and a
   few need the byte read — one from a specific bank — though whether that byte comes from the bank
   before the switch or after it, or is valid at all, is on simple hardware *"a coin toss"* (AtariAge
   `topic/266200`). A real case: Stella and Harmony returned the byte under a Supercharger hotspot
   (`$00`) where the real cart did not, and a `beq` after the call broke only on hardware (AtariAge
   `topic/221197`). **Cited only, not verified.** The engine's F8/F6/F4 read switches first and then
   returns that address in the new bank (`mapper_atari.go`: `cart.bankswitch(addr)` before
   `return cart.banks[cart.state.bank][addr]`) — one answer to the coin toss. Read from the code;
   **Not verified**.
2. **Cross-bank trampoline at `$FF80`** (callable as a plain `jsr $FF80` from bank 0):
   ```
   bank0 $FF80: lda $FFF9    ; select bank1 → next fetch $FF83 comes from bank1
   bank1 $FF83: jmp B1Work   ; bank1's entry dispatcher
   ...work...   jmp $FF86
   bank1 $FF86: lda $FFF8    ; select bank0 → next fetch $FF89 comes from bank0
   bank0 $FF89: rts          ; back to the caller (stack is shared RAM, unaffected)
   ```
   *Hidden by a macro* (Thomas Jentzsch, AtariAge `topic/285891`): `DEF_LBL Foo` records `Foo_BANK`;
   `JMP_LBL Foo` assembles to a plain `jmp Foo` when that is the current bank, and otherwise to
   `ldy #Foo_BANK / lda #>(Foo-1) / ldx #<(Foo-1) / jmp SwitchBank`, where `SwitchBank`
   (`pha / txa / pha / lda $fff4,y / rts`) sits at the same address in every bank. The caller never
   names a bank; the price is that A, X and Y are all spent, so nothing is passed in a register.
   **Cited only, not verified.**
   *Named like the instructions they replace* (Robert M's ARPM macros, AtariAge `topic/180561`):
   `JMB` for `JMP`, `JSB` for `JSR`, `RTB` for `RTS`, and `BS_END_BANK` reports the space left in the
   bank; indexed jumps from the same package, not described as crossing banks, are `JIX`/`JIY`
   (`ON X GOTO`) and `SIX`/`SIY` (`ON X GOSUB`). The target bank comes from the
   target's own address, `ldx #[>[{1}-1]]>>5` — the top-three-bits convention of `bankjsr` above.
   The zip was not fetched: **Cited only, not verified.**
   *A jump table at the top of every bank* (SpiceWare's Medieval Mayhem, 32K, retold on AtariAge
   `topic/273680`): the same table at `$F000` in every bank, each entry `nop SelectBankN` then
   `jmp XxxCode` — the `jmp` is fetched from bank N, so it must be there too — and every bank ends
   with the hotspot bytes and the vectors (`RORG $FFF4`). For EF's 16 banks that end block moves to
   `$1FE0`. Many banks make the table long; Thomas Jentzsch's alternative, `bit SelectBank1,y` then
   `jmp (vector)`, is slower but fits in a macro. **Cited only, not verified.** The engine's EF switches
   on any access to `$0FE0-$0FEF` and takes the bank from the low nibble (`mapper_atari_ef.go`:
   `cart.state.bank = int(addr & 0x000f)`). Read from the code; **Not verified**.
   *By `BRK`, from any bank to any bank* (vdub_bobby, AtariAge `topic/122388`): a call is `brk` followed
   by `.word Target`; every bank's BRK/IRQ vector points at the same routine at the same address,
   which reads the target from the two bytes after the `BRK` through the pushed return address,
   shifts its high byte right five times to get the bank, switches with `nop $1FF4,x` (F4) and
   `jmp (ptr)`; the return leg does the same with the caller's return address. It needs each bank
   `RORG`'d at an odd 4K boundary (`$1000`, `$3000`, `$5000`, …) so that the top three bits are the
   bank number. **Cited only, not verified.** That `BRK` pushes BRK+2 is measured in
   `docs/integration-density-playbook.md`.
   *Generated by the compiler* (Kylearan on K65, AtariAge `topic/252613`, 2016): functions are put in
   banks by `bank` statements, and `far doMagic` makes K65 *"automagically generate the necessary code
   stubs in both banks"* that switch to the callee's bank, call it, switch back and continue — the
   trampoline above, generated instead of written (read here, not stated there). Move both functions into one bank, remove `far`, and it becomes
   *"a traditional JSR/RTS subroutine"*. The bank of each function is still chosen by hand, the
   built-in switching is the one part whose overhead *"you cannot directly control"*, and K65 has its
   own syntax. **Cited only, not verified**; the generated stubs were not read.
3. **Data bank + RAM buffer**: bank 1 owns the level tables and the loader; the loader copies
   the selected level (8 PF bytes here) into zero page during VBLANK; bank 0's kernel renders
   only from RAM. Shared zero page is the contract between banks.
   *Moving variables into cartridge RAM* (djmips, AtariAge `topic/106769`): write and debug with them
   in zero page, then declare each one twice — `enemyXW = $1000`, `enemyXR = enemyXW+128` on a
   SuperChip — and turn every store into the `W` name and every load into the `R` name. CBS RAM+ (`FA`)
   has 256 bytes, written at `$1000-$10FF` and read at `$1100-$11FF` (the engine's `mapper_cbs.go`
   has the same map), so there the offset is 256. Read-modify-write does not survive the split
   (`docs/fundamentals-audit.md`): Pat Brady, *"you can't use INC or DEC instructions with on-cart RAM
   (regardless of bankswitching method)"*, and since it is not zero page every access takes a cycle
   more (AtariAge `topic/326419`). The procedure is **Cited only, not verified**.
   *The kernel itself in RAM*: PitKat gives E7's 1K RAM bank wholly to its 8×8 tile display kernel —
   code that runs from RAM, whose rewritten parts are the tile addresses and colours; no co-processor
   (the author, AtariAge `topic/308669`). **Cited only, not verified**; the ROM was not run.

## The trap that bit us (now baked into the template)

**Never place an instruction on the hotspot addresses.** A first draft put the return `rts` at
`$FFF9` — but instruction *fetch* is a read, and **reading $FFF8/$FFF9 switches banks**, so
returning from the trampoline flipped to bank 1, executed garbage, and hit the reset vector:
the ROM sat in a reboot loop (symptoms: 350-line TV frames, RAM cyclically re-cleared,
level stuck at 0). Diagnosed in minutes with `watch_ram` (the buffer's writer PC alternated
between the loader and the boot-time `Clr` loop). Trampoline at $FF80 keeps a safe distance.

**Not on the hotspot, and not just before it either, on F6.** Eckhard Stolberg, against a plan to put `BRK` on the
hotspots: *"Remember that the 6502 always reads at least two bytes for every instruction. So a BRK at $FFF8
will trigger both $FFF8 and $FFF9 with a one cycle gap"* 〔stella-list `200212/msg00000`〕. One byte lower
helps only while that byte is not a hotspot too: one `BRK` *"at $FFF7"* would do on F8, he went on, but not
if the game moves to F6 — where `$FFF7` is itself a hotspot (the engine's F6 switches on `$0FF6`-`$0FF9`).
**Cited only, not verified.** The engine makes the same read: an implied-mode instruction reads the byte
after its opcode (`cpu.go`, the `// phantom read` under `case instructions.Implied:`; `BRK` reads it as its
padding byte), and that read reaches the mapper like any other. `internal/cyclebound` refuses an instruction
whose own bytes cover a hotspot, and `rts`, `rti` and `brk` are one byte in `definitions.json`, so the byte
such an instruction reads after itself is not among the ones it checks. Both read from the code; **Not
verified**.

**The same read, used on purpose.** Fred Quimby, 2005, for F8: an `RTS` at `$FFF8` in bank 0 and at `$FFF7`
in bank 1; push the target minus one and `JMP $FFF8` (or `JMP $FFF7` to come back). Kroko's account:
fetching the `RTS` at `$FFF8` selects bank 0, where the code already is; *"After RTS is fetched, a dummy
fetch takes place which brings FFF9 to the bus, which then causes the bankswitching logic to switch to bank
1. The RTS then returns to the location you pushed on the stack, but after the switch to bank 1"*
〔stella-list `200503/msg00036`, `200503/msg00041`〕. It removes the fixed landing address the trampoline
above has to keep aligned. Quimby pushes `#>ROUTINE1` and `#<ROUTINE1-1` separately and notes that this
fails when the low byte of the address is zero; Manuel Rotschkar's form takes the target from a table
〔`200503/msg00038`〕 — `docs/techniques/rts-dispatch.md` does the table half and never mentions banks:
```
LDA JumpTable+1,Y
PHA
LDA JumpTable,Y
PHA
JMP $FFF8
Jumptable
	.word ROUTINE1-1, ROUTINE2-1, ROUTINE3-1
```
Quimby reported it working in Stella, z26 and on real hardware on a Kroko cart. Thomas Jentzsch was *"not
100% sure it works on real hardware"*: a similar trick in his first Battlezone TC hack *"worked on CC and
emulators, but not on real hardware"*. Quimby suspected the 0.1 µF capacitor on the boards he had seen,
Kroko argued that one cycle is all any hotspot access gets, and the thread ends with no test on such a board
〔`200503/msg00037`, `200503/msg00040`, `200503/msg00042`〕. **Cited only, not verified.** In the engine it
follows from the implied-mode read above (read from the code; **Not verified**); nothing here runs it.

**Placement bugs show up outside this repo too, and this engine has caught one.** Flap Ninja reset to
its title screen on real consoles (Harmony and PlusCart, PAL and NTSC) but not in Stella, even in
developer mode; the author called it *"some bug with the bank switching that Stella doesn't
recognize"*. A player found that Gopher2600 *"behaves like a console"* with it; the author rebuilt until
it ran there, and the new ROM reached gameplay on a PlusCart. He had met the bug before and *"resolved
it by simply rearranging the code"* (AtariAge `topic/367912`). What was moved is not in the thread.
**Cited only, not verified**; the ROM was not run here.

**Touching a hotspot on purpose, and nothing else.** `lda $1FF9` spends A and `bit` spends the flags.
SpiceWare's answer on AtariAge `topic/239091` is the undocumented `NOP` absolute: `nop SelectBank4`
assembles to `0c fa ff` and *"works just fine"* (Collect2 had used `CMP`). He gave the same answer on
`topic/243115` in 2015 — `sta`, `lda`, `cmp` and `nop` all switch, and he would use `nop` because it
changes no register and no flag — and `topic/273680` repeats it against `CMP`. In the engine `$0C` is
`nop`, absolute, 3 bytes, 4 cycles, category `read` (`Gopher2600/hardware/cpu/instructions/definitions.json`),
and the `NOP` case in `cpu.go` does nothing, so no register or flag changes; that it switches banks on
hardware is **Cited only, not verified**. `STA` to the hotspot was only guessed in the same thread to
cause bus contention, and the same thread reports that Centipede switches banks with `STA`. On E7,
where the cartridge sees no R/W line, `STA` is the recommended form because it leaves A
intact (AtariAge `topic/340351`; **Cited only, not verified**) — and this page's own trampoline,
`lda $FFF9`, spends A on every call. The opposite hazard — a `NOP`/`BIT` skip switching a 3F cart by accident — is in
`docs/known-traps.md`.

## Verified
- Loader contents land exactly ($81,$42,… for level 0; $FF,$7E,… after the switch).
- `bank.number == 0` at every frame boundary (the kernel never runs banked-in code).
- F6/F4 generalize by adding stubs/vectors per bank and more hotspots (verified in litmus).

## What crossing a bank costs, in stores per scanline (measured 2026-09-07)

Andrew Davie's 2003 packing tool imposed a rule and never said what it bought:

> a) For any frame, **ALL of its sprites must be in a single bank**
> b) The matrix definition for the frame must be in the same bank as the [sprites]
> 〔stella-list `200301/msg00229`〕

This page had the mechanism and no price. Measured by growing the store count in a kernel row until
every line of the band started taking two:

| how the graphic is reached | stores per scanline |
|---|---|
| same bank | **9** |
| one switch per line, fetches batched inside it | **8** |
| a switch on each side of every fetch | **4** |

★**That is the numeric reason for rule (a).** Reaching across banks per sprite costs **more than half**
the line's drawing capacity; batching the switch to once per line costs **exactly one store**. So the
rule is not conservatism — a frame whose sprites are split across banks either draws less than half as
much per line, or has to sort its fetches so that one switch serves them all, which is the packing
problem the tool existed to solve.

★★**The hotspot access is stood in for.** A real F8 switch is `sta $1FF9`, a 4-cycle absolute store;
`roms/litmus/litmus_bank_capacity.asm` is 4K and uses `lda $A0,x`, also 4 cycles and one memory
access, with no effect on a zeroed page. What is measured is the **time** a switch costs, which is
what the rule is about. The switching itself is covered by `litmus_bank`, `litmus_bank_f4` and
`litmus_bank_f6`. Guarded by `internal/emu/bankcapacity_test.go`, which runs all three bands at their
maxima (262 scanlines) and requires one more store to break it.

★★★**Two readings had to be corrected on the way here, both of them mine.** At one store past the
maximum the frame grows by exactly **one line**; only at two past does every line of the band take
two. Reading the +1 as the boundary made every maximum come out one too low. And the first three runs
of the sweep reported stale numbers, because the generated ROMs lived outside the module and `go test`
served **cached results** across two regenerations — `-count=1` is not optional when the fixtures are
somewhere the cache cannot see. Raised by the mailing-list distillation (helper-2).

**A second price, counted in registers.** Thomas Jentzsch, on a proposed scheme that needs two writes
per switch: *"The problem are not only the extra CPU cycles for the writes, but that you have to use 2
registers. So that only one register is left to work with."* When a subroutine needs two, one goes
through the stack or zero page (AtariAge `topic/279113`). The table above counts time; nothing here
counted registers. **Cited only, not verified.**

**And a switch can stand in for the loop.** In a thread on the Boulder Dash kernel: *"The very LAST
bank just has a 'rts' instead of continuation of the code - so it does the looping/branching for 8
lines without a single cycle of cost. It's essentially inline code, spread over 8 banks"* (AtariAge
`topic/256688`). Read here, not stated there: each bank runs on into the next at the next address —
the next-fetch rule of the trampoline — so there is no loop counter and no branch, where the RTS jump of
`docs/techniques/rts-dispatch.md` costs 6 cycles per use. What the switches themselves cost is not in
the copy, which holds 8 of the thread's 33 posts and leaves the author of that line unclear.
**Cited only, not verified.**

## Dividing a game between banks

- **Split by function, not by play mode** (AtariAge `topic/319899`). A bank per mode needs the
  sprites, tables and positioning routines in both. ben_larson's 16K *Panky* puts kernel, main loop,
  in-game logic and graphics in one bank, room set-up and room data in a second, title/ending kernels
  in a third and music in a fourth, so no graphics are duplicated. **Cited only, not verified.**
- **Or by screen — the opposite of the bullet above, in two plans from the list.** Erik Mooney's 16K RPG
  engine (1997): *"the world maps and their display kernel"* in one bank, text messages in a second, *"the
  monster graphics and code for battles"* in a third, and everything else — *"status bar display kernel,
  processing player movement/console switches, etc"* — in the last 〔stella-list `199707/msg00036`〕.
  Manuel Rotschkar's Beach Head (2005): title, map and high-score screens / torpedo run / naval battle and
  tank finale / tank movement, and *"I can probably create Banks 2-3 as separate solo 4K games"*, with room
  for another programmer to *"contribute 1 or 2 of the banks"* 〔stella-list `200507/msg00162`〕. Both
  were plans when posted. **Cited only, not verified.**
- **Or by calculation and drawing.** Nick Bensema, 1997, on how he would use 8K *"if I ever did need that
  much space"*: *"The first bank would have all the game calculation bits, the second bank would have all the
  screen drawing bits"* 〔stella-list `199703/msg00113`〕. Panky above keeps logic and kernel in one bank,
  and this page's demo splits data from code. A plan, not a game: **Cited only, not verified.**
- **Or put the code in both banks.** BiiggerBoing26 (2003) is an F8 image whose *"Code is mirrored in first
  1K of both banks"*, so *"this is effectively a 7K cart. It's like having two 3K 'banks' of unique data"*;
  its author called the switching *"'el cheapo'"* 〔stella-list `200307/msg00008`, `200307/msg00010`〕. The
  posted code switches with `BIT $1FF9` or `BIT $1FF8` to reach the bank that holds an animation frame and
  carries straight on — read here: the next instruction is the same in either bank, the next-fetch rule of
  the trampoline with the whole routine as the landing site. He assembled the 1K separately, emitted a byte
  at `$3FF` *"so that it would come out to 1024 bytes"*, and `incbin`'d it twice. **Cited only, not
  verified.**
- **Call a bank like a subfunction** (brpocock, AtariAge `topic/82141`): load a bank ID and a function
  ID into registers and `jmp bank_switch`; the selected bank's common entry reads the function ID. The
  shared RAM area is zeroed during the switch, which happens in VBLANK, and for a short script *"a
  second switch could happen almost immediately, without the player seeing anything at all."* **Cited only, not verified.**
- **What duplication costs**: the same author puts his 64K game at *"like 48k or so of unique data"*
  (about 25% duplicated), because every bank holding map data needs its own copy of the travel kernel —
  no switch is fast enough to fetch several tiles per row, and 128 bytes shared with game state cannot
  cache them. An estimate for a game that was not finished, not a measurement: **Cited only, not
  verified**. The stores-per-scanline table above is the measured half of the same reason.
- **The limit is the bank's, not the image's.** mzxrules' 3E+ Zelda port leaves bombable walls looking
  like plain walls, partly because *"the rom bank that handles that is near max capacity"* (AtariAge
  `topic/320907`). **Cited only, not verified.**
- **Which bank is which is ROM too.** In a Defender II hack nukey-shay swapped the two banks so that
  `$F000` is the first and `$D000` the second — *"this takes advantage of the hotspot jumps to save a
  little Romspace"* — and added a switch at the end of each bank to drop a duplicated score routine from
  the display bank (AtariAge `topic/289892`). How many bytes is not stated: **Cited only, not verified.**
  `docs/design-principles.md` puts hotspots at the highest addresses; that is not which bank holds what.

## Choosing the cartridge type

- **From what the game needs — for these two, RAM.** `docs/fundamentals-audit.md` records the community
  recommendation *"F8 first"*, for compatibility. andrew-davie, in a 2019 thread on F6: the scheme
  *"depends on the NEEDS of the game you want to write"*, and *"my games these days like having lots of RAM
  to play with. So I choose a bankswitch scheme that allows that"* (AtariAge `topic/294463`). grafixbmp's
  Beyond Castlevania began hoping that *"extra RAM wouldn't be necessary"*, found as the design grew that
  *"this didn't seem possible"*, and chose F4SC for *"the extra 128 bytes"* as a frame buffer for sprite
  data (AtariAge `topic/143390`). **Cited only, not verified.**
- **What each scheme adds in RAM.** Asked about 256 bytes, a 2022 thread pointed to an external table of
  the schemes with the ROM and RAM each provides, and added that most games with extra RAM used the
  SuperChip, *"which provided 128 bytes of RAM"*; svolli: it combines with 4K, F8, F6 and F4, and *"it
  "only" costs you 256 bytes of ROM per bank"* (AtariAge `topic/340856`; **Cited only, not verified**).
  The engine's mappers give:

  | scheme | ROM | extra RAM | where |
  |---|---|---|---|
  | F8SC / F6SC / F4SC | 8K / 16K / 32K | 128 bytes | `mapper_atari.go`, `superchipSize = 128` |
  | FA (CBS RAM+) | 12K | 256 bytes | `mapper_cbs.go`, `cbsRAMsize = 256` |
  | E7 (M-Network) | 8K / 16K | 1K plus four 256-byte banks | `mapper_mnetwork.go`; `fingerprint8k` and `fingerprint16k` both return `"E7"` |
  | CV (CommaVid) | 2K | 1K | `mapper_commavid.go`, `commavidRAMsize = 1024` |
  | 3E | from the file | up to 32 banks of 1K | `mapper_3e.go`, `ram [32][]uint8`, `ramSize = 1024` |
  | Supercharger | 2K BIOS | 6K | `supercharger/state.go`, above |

  Read from the code; **Not verified**.
- **The type is part of the budget.** svolli's 512-byte plasma demo runs on CommaVid *"so I can
  pre-calculate some data"*; the Supercharger version is over 512 bytes *"since RAM access there
  requires more effort"* (AtariAge `topic/354722`). The same picture costs different code on different
  carts, so a size limit applies after the cart is chosen. How much the precalculation saves is not
  stated: **Cited only, not verified.** The engine's CommaVid is 1K RAM read at `$F000-$F3FF` and written
  at `$F400-$F7FF`, with 2K ROM above it (`mapper_commavid.go`), whose comment names another svolli 512-byte demo
  (AtariAge `topic/342021`) as the reason it was implemented; the Supercharger's arming sequence is in
  the table above. Read from the code; **Not verified**.

## The image: size, joining, recognition

- **At least 2K.** *"A 2600 game must be at least 2K. If you want to do a 1K game then you should fill
  the first 1K with #$00"* (debro, AtariAge `topic/63395`); the 1K image had run in PCAE and shown
  nothing in z26. Why 1K fails is not said; what the engine does with 1K is **Not verified**.
- **F8 is two 4K images end to end**: `COPY /B /Y bank1.bin + bank2.bin f8game.bin` (AtariAge
  `topic/352624`; `cat` on macOS, **Not verified**). A switch from the first game lands at the next
  address in the second, not at its start — the next-fetch rule of the trampoline above. A proposed fix
  for the resulting soft lock, from the same thread (no result is reported): put `JMP ($FFFC)` in the second image at the address
  right after the `LDA $1FFx`, so the second game starts from its own reset vector — a hand-off that
  does not return, unlike the trampoline. **Cited only, not verified.**
- **Where each bank is assembled.** `docs/fundamentals-audit.md` records each bank's origin at an odd 4K
  segment and `$D000`/`$F000` for 8K. For 16K, Thomas Jentzsch moved Thrust's banks so that they *"now start
  at the adresses $9000, $b000, $d000 and $f000, like all later 16k atari games do. (This should help to
  check the ROM with real hardware)"* 〔stella-list `200007/msg00055`〕. Read here, not stated there: the
  eight odd windows measured above return the same byte, so the 6507 cannot tell these origins apart; they
  differ only to something that sees more address lines. **Cited only, not verified.**
- **On a 7800 RAM cart, only the odd 4K segments.** Eckhard Stolberg, to an F8 game prepared for one: *"In
  2600 mode the 7800 maps in the TIA and RIOT in all even 4K segments of the 6502 memory map. Therefore you
  can only use the odd 4K segments for a game. You have to double the segments in the ROM and compile the
  later halves for $D000 and $F000"* 〔stella-list `200304/msg00244`〕. The rebuilt image came back with
  *"This may not have been a fair test though"* and no result 〔`200304/msg00246`〕. In 2004 he answered a
  question about mirroring with *"In 2600 mode the memory map is exactly like on a real VCS"*
  〔`200409/msg00115`〕; read here, the two fit if what differs is only what a cart that sees all sixteen
  lines is handed — the kind of difference the portability caveat in the A13–A15 section names.
  **Cited only, not verified.**
- **A flash cart can assume a layout the scheme leaves open.** Eckhard Stolberg, to a 3F demo: copy *"the
  last 2K bank in the ROM (which is fixed at $1800-$1FFF)"* to *"the 4th 2K bank in the ROM"*, so that it
  runs on a Cuttle Cart, which *"is set up for the normal 8K versions of 3F bankswitching, so it always uses
  the 4th bank for the fixed area"* 〔stella-list `200301/msg00151`〕. **Cited only, not verified.** The
  engine fixes the last 2K of the image whatever its size (`mapper_tigervision.go`: *"the last 2K always
  points to the last 2K of the image"*; read from the code, **Not verified**), so for an image larger than
  8K it and such a cart disagree — read here.
- **Recognising the scheme from outside.** A dumper sees 4K at first, so it cannot fingerprint the
  whole image as Stella does and has to trigger hotspots and look (Thomas Jentzsch, AtariAge
  `topic/354336`). The Retron 77 dumper touches `$1FF6-$1FF9` one at a time and compares checksums of
  3.5K, skipping the first 256 bytes (SuperChip RAM) and the last 256 (hotspots): a change on
  `$1FF6/7` means F6, on `$1FF8/9` F8, none 4K; F4 is not detected. Reading each bank, it re-selects
  before every byte at `$FFF0` and above. A two-part dumper (Teensy++ 2.0 and a connector) lists what it
  reads as *"2k/4k and F8/F6/F4"* — sizes and schemes on one list (AtariAge `topic/305662`). **Cited
  only, not verified.** The engine works from the whole file (`fingerprint.go`, `fingerprint()`): it
  first tests signatures that do not depend on size — ELF, ACE, CDF, DPC+, DevCard, Supercharger
  fast-load, 3E+, 3E — and only then switches on the size, testing byte patterns within some sizes.
- **Byte fingerprints fire on operands.** Disc Match, a 64K EFSC ROM, was detected by Stella as 3F
  because `LDY $3F85,X` assembles to `$BC $85 $3F` — its operand is the bytes of `STA $3F` — and it
  occurred in several places; Stella's `isProbably3F` wants two. One padding byte before the graphic
  moved the address and fixed it (AtariAge `topic/384121`; **Cited only, not verified**). In the engine,
  8K, 16K and 32K images are tested for 3F before E0/E7 and the F8/F6/F4 default (after WF8 on 8K
  and FA2 on 32K), and 3F wants more than five `85 3F` anywhere in the file (`fingerprintTigervision`);
  its 64K path tries only EF. Read from the code; **Not verified**.
- **Signing the image so that it is recognised.** An author keeps `STA $3E` / `LDA #$00` in his code only
  so that Stella detects 3E (AtariAge `topic/249429`) — exactly the engine's 3E fingerprint
  (`fingerprint3e`: `0x85, 0x3e, 0xa9, 0x00`), which `roms/carts/cart_3e.asm` carries for the same
  reason. On Harmony the way round a wrong guess is the file name: renaming to `.3E` overrides its
  detection (Omegamatrix, same thread), and the author reported that it *"works better"*; Stella had no
  such override then (stephena wanted to add one). **Cited only, not verified.** The
  engine does the same: `internal/emu` loads with `"AUTO"`, and `cartridgeloader/loader.go` then takes
  any extension on its explicit list (`.3E`, `.F8`, `.EF`, …) as the mapping, so the file is never
  fingerprinted. Read from the code; **Not verified**.
