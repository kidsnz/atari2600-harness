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
