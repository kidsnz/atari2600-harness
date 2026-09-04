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
| FE | the **stack** — `$01FE` on the bus after a JSR, then `data >> 5` (no hotspot at all) |
| Supercharger | a **stateful arming sequence** (`$F0xx`, then the next `$Fxxx` is a write) |
| double-ender | **the connector**, wired once and never changed |

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
2. **Cross-bank trampoline at `$FF80`** (callable as a plain `jsr $FF80` from bank 0):
   ```
   bank0 $FF80: lda $FFF9    ; select bank1 → next fetch $FF83 comes from bank1
   bank1 $FF83: jmp B1Work   ; bank1's entry dispatcher
   ...work...   jmp $FF86
   bank1 $FF86: lda $FFF8    ; select bank0 → next fetch $FF89 comes from bank0
   bank0 $FF89: rts          ; back to the caller (stack is shared RAM, unaffected)
   ```
3. **Data bank + RAM buffer**: bank 1 owns the level tables and the loader; the loader copies
   the selected level (8 PF bytes here) into zero page during VBLANK; bank 0's kernel renders
   only from RAM. Shared zero page is the contract between banks.

## The trap that bit us (now baked into the template)

**Never place an instruction on the hotspot addresses.** A first draft put the return `rts` at
`$FFF9` — but instruction *fetch* is a read, and **reading $FFF8/$FFF9 switches banks**, so
returning from the trampoline flipped to bank 1, executed garbage, and hit the reset vector:
the ROM sat in a reboot loop (symptoms: 350-line TV frames, RAM cyclically re-cleared,
level stuck at 0). Diagnosed in minutes with `watch_ram` (the buffer's writer PC alternated
between the loader and the boot-time `Clr` loop). Trampoline at $FF80 keeps a safe distance.

## Verified
- Loader contents land exactly ($81,$42,… for level 0; $FF,$7E,… after the switch).
- `bank.number == 0` at every frame boundary (the kernel never runs banked-in code).
- F6/F4 generalize by adding stubs/vectors per bank and more hotspots (verified in litmus).
