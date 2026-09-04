# Technique — missiles as bullets (RESMP spawn, flight, hit)

**Goal:** the standard "fire / fly / hit" loop using a missile: spawn at the player via RESMP,
move vertically by row-range drawing, detect hits with the collision latches.

Demo: `roms/techniques/bullets.asm` (joystick ship, fire launches M0 at a P1 target; hit
respawns the target +24px).
CI: `scenarios/bullets.json` (spawn position, flight, latch, hit bookkeeping, 262, golden).
New hardware verification: `litmus_resmp` + `scenarios/resmp.json`.

## Verified hardware facts (litmus_resmp)
- **RESMP unlock places the missile at player+4px** (1x size player center): player at 24 →
  missile at 28; follows the player after HMOVE moves (32 → 36).
- **The lock needs at least one frame of held RESMP** before unlocking. The lock-to-center sync
  happens while the player counter scans during the frame — a lock+unlock in the same logic pass
  does NOT move the missile (measured; Gopher2600 syncs at the player's scan `Pixel==2`).

## The pattern

- **Spawn**: fire edge (and no live bullet) → `RESMP0=2`, mark state "locking" (`bulY=$FF`).
  *Next* frame's logic: `RESMP0=0`, `bulY=SHIPROW-4`. Order matters: process the locking→flying
  transition **before** the fire-edge check in the frame logic, or the same pass unlocks
  immediately (the bug we hit).
- **Flight**: `bulY -= 4` per frame. The kernel draws `ENAM0` on rows `[bulY, bulY+4)` with a
  branch-free compare (`txa / sbc bulY / cmp #4`). **Inactive = sentinel 200** (out of row
  range) — no "is active?" test in the kernel, which keeps the worst row ≈62 cycles.
- **Hit**: `BIT CXM0P` / `BMI` → D7 = M0×P1. **Read address $30** — collision reads decode the
  low nibble, so a sloppy `$32` silently reads CXP0FB instead (the second bug we hit; the latch
  was provably set via `read_collisions` while the ROM saw nothing). Clear with `CXCLR` every
  frame after the check.

## Hard-won notes
- **Kernel line budget**: the first version did per-row `lda bulY / beq …` gating and blew past
  76 cycles when the bullet was live → the TV frame stretched to 350 lines. Sentinel encoding +
  X-as-row-counter brought every row under budget. Symptom to remember: *line count changes only
  while an object is active* = per-row code over budget on its active path.
- `PosObject` (divide-by-15) fine adjust is **`eor #7`**, not `eor #$FF` (that reverses the
  fine-adjust direction and breaks linearity). With indexed stores (`sta RESP0,x`) the measured
  calibration here is real X = A−3; with absolute stores it was A−9. **Calibrate per kernel
  with `read_tia`, never copy constants.**

## The ENAM "stack trick" — branchless 1-line missile enable (Combat)
For a **1-scanline** missile, instead of the row-range compare you can enable ENAM branchlessly in
~10 cy using the stack as a TIA-write pointer. Page 1 (`$01xx`) mirrors the TIA (A7=0), so
`[$011D]=ENAM0`, `[$011E]=ENAM1`, `[$011F]=ENABL`. Point SP at the ENAM mirror and `PHP` writes the
processor status — whose **bit 1 is the Z flag** — straight into ENAM.D1 (the enable bit):
```
; once per frame, before the visible region:  LDX #$1D ; TXS   (SP → ENAM0 mirror)
; per kernel line:   CPX MissY0   ; Z = (this row == missile row)
                     PHP          ; [$011D]=ENAM0, D1=Z  → lit only on its row
                     PLA          ; restore SP=$1D
```
Needs **no `JSR` in the kernel/VBLANK/overscan** (SP is borrowed). **Corrected 2026-09-03:** this line read "Needs `SEI` (no IRQ)" and `SEI` does nothing here — **the 2600 has no path by which an IRQ can reach the 6507**. Measured in the vendored engine: `CPU.Interrupt()` is called from five places and all five are `mem.arm.Interrupt()`, the ARM coprocessor in ELF/ACE carts; the RIOT's PA7 flag is a status bit software polls (TIMINT) and never reaches the CPU.
**Corroborated externally, added 2026-09-03:** the list said it first and drew the same distinction.
Erik Mooney, 1999: *"There are no interrupts on the 2600."* Two years later someone asks *"is the
`SEI` at the beginning of most games unnecessary then?"*, and Eckhard Stolberg answers by separating
exactly what we separated from the engine — the RIOT's own flag (*"everytime the timer wraps from $00
to $FF the interrupt flag is set (if timer interrupts are enabled)"*) from anything the 6507 can see.
The derivation here was independent and reached the same split; this is the rarer sort of source, the
kind that **confirms** a decision rather than correcting one. Most ROMs here still open with `sei` (139 of 173 .asm files) and that is fine as convention — the error was calling it a requirement of this technique. Trap: **ENAM0=$1D,
ENAM1=$1E, ENABL=$1F** — mixing them lights the wrong object; verify with `read_motion` height, not the
annotated position marker (a proxy).

### Two missiles in one branchless burst — the descending double-push
Both players' missiles (M0 + M1) in one sequence: point SP at the **higher** mirror (`$1E`) and push
twice, descending, so each `PHP` lands in the next ENAM:
```
; once per frame:  LDX #$1E ; TXS
; per line:   CPX MissY1 ; PHP   ; [$011E]=ENAM1  (SP $1E→$1D)
             CPX MissY0 ; PHP   ; [$011D]=ENAM0  (SP $1D→$1C)
             PLA ; PLA          ; restore SP=$1E
```
= 20 cy, both missiles, no branches. The two `PLA` (8 cy) are the SP restore, and they are mandatory
**only while `X` is the line counter** — which is what makes `CPX` the comparison and leaves nothing to
hold `$1E` across the line.

**Corrected 2026-09-03: this line read "Irreducible", and the sentence two lines below already named
the escape.** Compare the line counter in `Y` instead and `X` stays free, so the restore is a 2-cycle
`TXS` rather than eight cycles of `PLA;PLA`:

```
; once per frame:  LDX #ENABL ; TXS
; per line:   CPY BLline ; PHP
             CPY M1line ; PHP
             TXS                 ; 2 cy, not 8
```

That shape is **Thomas Jentzsch, stella 1999-11 〔msg00039〕**, whose own comment reads
`php ;3      got this trick from Combat` — so the in-house derivation below and this post reach the
same trick from the same game, twenty-seven years apart. **`CPY` appears nowhere in this file or in
`two-line-kernel.md`**, which is why the alternative read as "a different technique" rather than as
the same one with a different register. The saving is arithmetic on paper here and has not been
measured; what is measured is that "irreducible" was too strong a word.

If the line still overruns, the fix is more budget (a 2-line kernel), or the graphics-pointer X-pin
trick that collapses `PLA;PLA` to a 2-cy `TXS` (`two-line-kernel.md`).
— in-house: Combat 2026-07-18, ∀-certified; independently in Jentzsch 1999-11 〔stella msg00039〕.

## Missile bounce / tank block off the playfield — CX?FB move-check-revert
The mover doesn't know where the maze walls are; use the hardware object-vs-playfield latches
(`CXP0FB`/`CXP1FB`/`CXM0FB`/`CXM1FB`, D7 = object∧PF, cleared each frame by `CXCLR`). A collision is
only known **after** the object rendered, so it's a 1-frame-delayed bump:
- **Tank block:** before moving, `BIT CXPxFB; BMI blocked`. On a hit, restore the last non-colliding
  position (saved when clear) **and** skip this frame's forward step — revert *alone* re-enters the wall
  next frame; skip *alone* leaves the tank stuck inside. Turning stays allowed so you can steer off.
- **Missile reflect (2-frame axis probe):** on `CXM?FB`, revert to the last clear position and reverse
  **X** (assume a vertical wall); if it still collides next frame it was a horizontal wall → reverse **Y**
  instead. 16-dir ring: reverse-X = `(8−dir)&15`, reverse-Y = `(−dir)&15`, reverse-both = `(dir+8)&15`.
- **Debug with `read_collisions`:** it separates `m0_pf` from `m0_p0`/`m0_p1`. A missile spawned *inside*
  its own tank reads `m0_p0=true, m0_pf=false` — don't mistake that for a wall hit (the bug that stalled
  the reflect prototype). — in-house: Combat 2026-07-18/19 (block shipped ∀-certified; reflect: vertical
  bounce verified via `read_motion`, full probe still has a spawn-inside-tank edge case).
