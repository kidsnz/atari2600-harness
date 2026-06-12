# Technique #10b — Dynamic multi-sprite kernel (the full form)

**Goal:** N objects at arbitrary, *crossing* vertical positions through 2 players — the general
engine behind games with free-moving object sets. Extends the verified flicker-pairs core (#10)
with the three missing pieces: per-frame **Y-sort**, **dynamic 2-of-N slot allocation**, and
**mid-screen repositioning** of a player after its previous object ends.

Demo: `roms/techniques/dyn_multisprite.asm` — 5 color-coded objects bouncing at different rates
(orders cross constantly), CI-locked by `scenarios/dyn_multisprite.json` + an ingest-based color
proof (`TestDynMultisprite`).

## The architecture (what real engines do)

- **Sorting network, not insertion sort.** 9 fixed compare-swaps sort 5 objects with
  deterministic cycles — kernels and VBLANK budgets hate data-dependent timing.
- **Slot queues.** Sorted objects assign alternately to P0/P1 (alternation start flips each
  frame = fairness); an object only joins a slot if it starts ≥ previous-end + 2 pairs (the
  repositioning gap); fallback to the other slot, else dropped this frame. Queues end with a
  **0 sentinel** — the kernel's wait-state compares against it harmlessly (cheaper than a
  bounds check by 5 cycles, which mattered).
- **2-line kernel state machine.** Line A = P0's slot, line B = P1's: WAIT (stage the next
  object's color; compare trigger pair) → POSITION (timed RESP via per-object delay constants —
  X lands on the coarse 15-px grid, no HMOVE needed) → DRAW (art rows) → next queue entry.
- **TIM64T VBLANK.** Sort + assignment cost varies by path (~60–160 cycles); padding WSYNCs
  can't equalize that. The timer absorbs it — the real-game idiom, now verified here.

## Cycle war stories (all measured, all CI-guarded now)
- The assignment's worst path (double slot fallback) is ~160 cycles — uncountable in per-line
  WSYNC budgeting; this alone forced the TIM64T design.
- The B-line POSITION path landed on **exactly 76 cycles** — the closing WSYNC itself crossed
  the boundary. Moving the POSITION block to fall through to the loop tail (deleting one `jmp`,
  −3 cycles) fixed it. Enumerated-spill probing (every interval >76 with its PC) found both.
- Queue-advance was the other spiller until exhaustion checks moved into the sentinel.

## Verified
- 262 lines every frame, **zero visible-region budget spills over 10 frames** (instruction-level
  interval enumeration), all 5 object colors render across frames (multi-frame ingest proof),
  golden + RAM asserts in CI.
