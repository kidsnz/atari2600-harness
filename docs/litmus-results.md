# Litmus test results — horizontal position (measured verification of gap B)

Measured values obtained by positioning player0 through the harness (`cmd/harness`) and reading
`read_tia`'s `ResetPixel` / `HmovedPixel` **numerically**. This is the authoritative measurement that
ensures we never repeat past-Pong failure #1 ("brute-forcing magic constants"). **Everything is real
emulator output (Gopher2600 nightly), not guesswork.**

ROM: `roms/litmus/litmus_pos.asm` (one coarse-adjust loop iteration = `SBC#1`+`BCS` = 5 CPU cycles =
15 color clocks). Procedure: poke `$80` (DELAY) → `step_frame` → `read_tia`. HMOVE is 0 via HMCLR.

## Coarse-adjust sweep (2026-06-09)

| DELAY | ResetPixel | HmovedPixel | Δ vs prev | note |
|------:|-----------:|------------:|-------:|------|
| 0 | 72 | 72 | — | minimal delay; boundary artifact of a deep-HBLANK strobe |
| 1 | 3 | 3 | — | **leftmost clamp (player minimum X=3)** |
| 2 | 12 | 12 | +9 | HBLANK→visible transition boundary (nonlinear) |
| 3 | 27 | 27 | +15 | ← linear from here |
| 4 | 42 | 42 | +15 | |
| 5 | 57 | 57 | +15 | |
| 6 | 72 | 72 | +15 | |
| 7 | 87 | 87 | +15 | |
| 8 | 102 | 102 | +15 | |
| 9 | 117 | 117 | +15 | |
| 10 | 132 | 132 | +15 | |
| 11 | 147 | 147 | +15 | ← linear up to here |
| 12 | 2 | 2 | (162) | **wraps at 160** (147+15=162 → mod 160 = 2) |

## Established facts

1. **Coarse adjust is perfectly linear:** for DELAY 3–11, `ResetPixel = 15·DELAY − 18`.
   One loop iteration = 5 CPU cycles = 15 color clocks = **15 px**. **3 px / CPU cycle**, confirmed on the emulator.
2. **Wraps at the 160-wide visible region** (mod 160). DELAY=12 rolls back to 2.
3. **Leftmost clamp = X=3** (player). Matches the constant in CLAUDE.md.
4. **`HmovedPixel == ResetPixel`** (motion registers are 0 via HMCLR, HMOVE not fired).
   → firing HMOVE should produce a difference (verified in the next step).
5. **Coordinate system:** `ResetPixel` / `HmovedPixel` / the beam's visible `Clock` are the same visible
   pixel coordinate 0–159.

## Relation to the CLAUDE.md formula

CLAUDE.md: player `X = 3N − 54` (N = CPU cycles from the sync point to the RESPx strobe). The −18 offset
here differs because it includes this ROM's prologue (HMCLR/LDA/SEC and the trailing SBC·BCS·STA) cycle
count, but the **slope 3 px/cycle matches**. The point is that "running it pins the value numerically" =
gap B is closed. The absolute value of N is kernel-dependent and is something you **measure**.

### Automatic reproduction (B-4, v0.20.0)

The manual sweep above can now be **reproduced automatically** with `cmd/calibrate`
(`internal/calibrate`). `go run ./cmd/calibrate` sweeps litmus_pos's DELAY, robustly excludes the wrap
and left-edge saturation, and does a linear regression → outputs **slope 3.0000 px/CPU-cycle, offset −18,
R²=1.0** (matching this table). Change the kernel pattern, swap in a cooperating ROM, recalibrate, and you
get that kernel's offset constant numerically every time.

## HMOVE fine-adjust sweep (2026-06-09)

ROM: `roms/litmus/litmus_hmove.asm`. Fix coarse position at DELAY=6 (ResetPixel=72), poke `$81` (HMVAL)
→ set `HMP0` → strobe `HMOVE` right after the next-line WSYNC → measure `read_tia.HmovedPixel`.

| HMP0 | ResetPixel | HmovedPixel | shift | CLAUDE.md expected | match |
|-----:|-----------:|------------:|------:|:--------------:|:----:|
| $00 | 72 | 72 | 0 | 0 | ✅ |
| $10 | 72 | 71 | −1 | left 1 | ✅ |
| $20 | 72 | 70 | −2 | left 2 | ✅ |
| $30 | 72 | 69 | −3 | left 3 | ✅ |
| $40 | 72 | 68 | −4 | left 4 | ✅ |
| $50 | 72 | 67 | −5 | left 5 | ✅ |
| $60 | 72 | 66 | −6 | left 6 | ✅ |
| $70 | 72 | 65 | −7 | left 7 | ✅ |
| $80 | 72 | 80 | +8 | right 8 | ✅ |
| $90 | 72 | 79 | +7 | right 7 | ✅ |
| $A0 | 72 | 78 | +6 | right 6 | ✅ |
| $B0 | 72 | 77 | +5 | right 5 | ✅ |
| $C0 | 72 | 76 | +4 | right 4 | ✅ |
| $D0 | 72 | 75 | +3 | right 3 | ✅ |
| $E0 | 72 | 74 | +2 | right 2 | ✅ |
| $F0 | 72 | 73 | +1 | right 1 | ✅ |

### Established facts (HMOVE)

6. **The HMOVE nibble table matches CLAUDE.md exactly** (all 16 values). Upper nibble only, two's
   complement, **positive = left (X decreases) / negative = right (X increases)**, range **+7 (left 7, $70)
   to −8 (right 8, $80)**.
7. Moves at **1 px granularity** (fine adjust works). `ResetPixel` stays fixed (72) = RESP0 and HMOVE are
   independent.
8. Coarse (15 px) + HMOVE (1 px) make **any X fully specifiable**. Rule #4 passes.

## Litmus test conclusion

**The harness is real.** Horizontal position can be numerically predicted, placed, and verified by
"coarse 15 px + fine 1 px". Past-Pong failures #1 (brute-forcing magic constants) and #3 (positioning
breakdown) are always backed by `read_tia` numbers on this harness. Gap B is closed.

## Unverified (optional, low priority)

- An exact explanation of the boundary artifact for deep-HBLANK strobes (DELAY 0–2).
- Per-kernel measurement of the absolute N in the missile/ball formula `X = 3N − 55` (player is −54).
