# Technique — constant-divisor division helpers (÷3, ÷7, ÷10, ÷15)

**Goal:** branchless-ish constant division without lookup tables or illegal opcodes, exact over the
full 0..255 input range. Used in positioning math (÷15 coarse position) and score / BCD digit
splitting. Each helper returns an exact integer quotient and remainder.

Demo: `roms/techniques/divtable.asm` (computes the divisions at init, stores results to RAM, then
renders a non-blank bar; ÷15 also drives an on-screen value).
CI: `scenarios/divtable.json` (13 exact RAM asserts for ÷3/÷7/÷10/÷15, 262 lines, golden).
Source studied: `reference/atariage/113254-fast-divide-by-seven/notes.ja.md` (Apple Assembly Line
1984-12 reciprocal-shift family) — generalized here to ÷3/÷10/÷15 with a provably exact correction.

## The method — reciprocal multiply + one-step correction

Constant division `A / d` is computed as:

```
q ≈ (A * RECIP) >> 8          RECIP = ceil(256 / d)   -- NOT round; see below
                              d=3 → 86   d=7 → 37   d=10 → 26   d=15 → 18
```

The high byte of `A * RECIP` is a quotient *estimate*. For these four divisors the estimate is
always within **±1** of the true quotient over the entire 0..255 range (exhaustively checked in Go
before writing the asm), so a single bounded correction makes it exact:

```
if A < q*d   { q-- }          ; estimate too high (e.g. ÷15 of 100 estimates 7, true 6)
rem = A - q*d
while rem >= d { rem -= d; q++ }   ; estimate too low
```

Both correction loops run **at most once** per call. The remainder is exact, so this also yields a
correct `A mod d` for free.

### The multiply (`MulHi8`)

The product high byte comes from a generic 8×8 shift-add multiply (MSB-first):

```
MulHi8:  lda #0 / sta prodlo / sta prodhi / ldx #8
mh_lp:   asl prodlo / rol prodhi      ; shift 16-bit accumulator up
         asl mul / bcc mh_no          ; next multiplier bit (MSB first)
         clc / lda prodlo / adc num / sta prodlo
         lda prodhi / adc #0 / sta prodhi
mh_no:   dex / bne mh_lp / rts        ; prodhi = (num*mul) >> 8
```

All operands are zero-page and all stores are deterministic (no page-cross penalty), so timing is
fully predictable. The same routine serves all four divisors — only the loaded `RECIP` constant
differs (`DivCalc` selects it from the divisor passed in X).

## CI — what the scenario proves

`scenarios/divtable.json` asserts the exact RAM results at frame 3:

| input | ÷3 | ÷7 | ÷10 | ÷15 |
|------:|---:|---:|----:|----:|
| 100   | 33 r1 | 14 r2 | 10 r0 | 6 r10 |

Plus a ÷15 sweep of `0,15,30,…,150` → quotients `0,1,2,…,10` (table at `$A0`), and a ÷15-derived
on-screen value at `$B0` (÷15 of 90 = 6). 13 exact RAM asserts in total, `ntsc_frame_lines:262`,
`golden_frame:true`. The visual is a solid playfield+player bar across the mid-screen (not blank).

## Verified facts

- **Exactness:** all four helpers reproduce `A/d` and `A%d` for **every** input 0..255 (verified in
  a Go reference model). The ±1 reciprocal bound is what makes the single-step correction sufficient.
- **Reciprocal constants:** `ceil(256/d)` — 86 / 37 / 26 / 18. **The formula said `round` and did not
  produce two of its own four constants** (corrected 2026-09-04): `round(256/3) = 85` but the table
  holds **86**, and `round(256/15) = 17` but the table holds **18**. `ceil` gives all four. ÷7 and ÷10
  are unaffected because 256/d already rounds up there.
  **Both choices are correct** — every one stays inside the ±1 the single-step correction absorbs, so
  this was a wrong description of right constants, not a wrong constant. **But `ceil` is not uniformly
  the better pick, and the earlier note's parenthetical implied it was.** Measured over all 256 inputs:

  | | corrections needed | error values |
  |---|---|---|
  | ÷3, RECIP=85 (round) | 85 | −1 or 0 |
  | ÷3, **RECIP=86 (ceil)** | **43** | 0 or +1 |
  | ÷15, **RECIP=17 (round)** | **17** | −1 or 0 |
  | ÷15, RECIP=18 (ceil) | 111 | 0 or +1 |

  So `ceil` halves the corrections for ÷3 and multiplies them by **6.5** for ÷15. What the shipped
  table actually is, is *ceil throughout*; **why** is not the correction count.
  **Nor is it the implementation, checked 2026-09-04.** `divtable.asm`'s correction runs **both
  ways** — it shrinks `quot` while `quot*divd > num` and then grows it while `rem >= divd`, and its
  own comment says so: *"the reciprocal estimate is within ±1 of the true quotient, so we correct in
  BOTH directions"*. An asymmetric corrector would have forced a one-sided reciprocal and explained
  everything; this one does not. **Either constant is equally correct here, and neither the code nor
  this page knows why ceil was chosen.** Recorded rather than rationalised — the alternative was to
  invent a reason, and a plausible reason that is not the real one is worse than an open question. (Found by the mailing-list distillation, helper-1, who
  measured ÷3 and read the `round`/`ceil` mismatch off it; the ÷15 half is re-run here and reverses
  the advantage, so the "ceil is better" reading did not survive checking.)
- **Cycle cost (NMOS timing, no page-cross), in=100:** ÷3 ≈ 873 cy, ÷7 ≈ 556 cy, ÷10 ≈ 497 cy,
  ÷15 ≈ 552 cy. `MulHi8` is a fixed ~150 cy; the variable cost is the `q*d` remainder term, computed
  here by **repeated addition** (Y = q iterations) for clarity, so cost grows with quotient size
  (÷3 range 347..3076 cy, ÷15 range 319..872 cy over 0..255).
- **Speed variant (not used here):** replace the repeated-add `q*d` with a second `MulHi8`-style
  multiply (or, for ÷7, the pure reciprocal-shift `LSR/ADC/ROR` chain from the reference, exact 0..255
  for ÷7 with no correction at ~40 cy). The table here optimizes for *provable exactness across all
  four divisors via one shared routine*, not minimum cycles. The reference's pure-shift ÷3/÷10/÷15
  forms drift in the upper range (÷3 first error at 129, ÷15 at 15), which is why the corrected
  reciprocal form is used for the general helper.
- **÷15 = the coarse-position divisor** (CLAUDE.md: coarse adjust is divide-by-15, 5-cycle loop). The
  demo wires ÷15 into a visible value to keep the technique anchored to its real use (positioning).

## Notes / caveats

- No illegal opcodes (the reference's 54-cycle Omegamatrix version uses `SBX`; we stay legal, as the
  harness has not litmus-verified illegal-op behavior in Gopher2600).
- `DivCalc` clobbers Y and the `tmp` scratch byte — callers that loop over inputs must keep loop
  state in their own zero-page bytes (the demo's sweep uses `swin`/`swix`, not Y/`tmp`).
- For a single fixed divisor in a hot path, inline the specific reciprocal-shift chain instead of the
  general `DivCalc`; this catalog entry is the *general, exact* helper.
