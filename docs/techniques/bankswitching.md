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

**A scheme can also have an input with no defined result.** A Rentacom 2-in-1 cartridge that no
emulator ran was worked out from its board — two 74LS10s forming a NAND SR latch — by alex_79 on
AtariAge `topic/293883`: any address with A12=0, A9=1, A6=0, A5=1 selects bank 0; A12=0, A9=1, A6=1,
A5=0 selects bank 1; and A12=0, A9=1, A6=1, A5=1 is the latch's undefined state, so it *"may or not
cause a bankswitch."* A list of hotspots cannot state that third row. **Cited only, not verified.**
The same post notes that these addresses include the ones UA games use. The engine's UA mapper
decodes the same four lines — `mapper_ua.go` `AccessPassive`: `switch addr & 0x1260 {` — and switches
only on `$0220` (bank 0) and `$0240` (bank 1), so the third combination (`$0260`) never switches there:
the engine picks one answer to a question the latch leaves open. Read from the code; **Not verified**.

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
