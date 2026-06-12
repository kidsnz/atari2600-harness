# Technique — bank-switched game structure (F8 template)

**Goal:** the structural template for games larger than 4K: per-bank reset stubs + vectors, a
reusable cross-bank call trampoline, and the data-bank pattern (level assets loaded from
another bank into RAM).

Demo: `roms/techniques/banked_game.asm` (F8 8K; bank 1 holds level data + loader, bank 0 runs
the game and switches levels every 120 frames).
CI: `scenarios/banked_game.json` (cross-bank load contents, level switch, bank.number at frame
boundary, 262, golden).
Hardware basis: `litmus_bank` / `_f6` / `_f4` (v0.43.0; hotspots, AUTO fingerprint, per-bank
vectors all verified).

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
