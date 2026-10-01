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
- **A shift chain that is ×85 in disguise.** Alex Herbert's 8-bit ÷3 "from memory" — `sta temp /
  lsr / lsr`, then `clc / adc temp / ror / lsr` four times 〔stella-list `200412/msg00075`〕 — returns
  exactly `(A*85)>>8` for every input 0..255, so its error is the RECIP=85 row of the table above:
  −1 on 85 inputs, exact on 171. The 85 are precisely the multiples of 3 from 3 to 255 (3/3 reads 0):
  it is wrong on every input that divides evenly. David Galloway's reply in the same thread derives the
  same ×$55 from 1/3 = $0.55 〔`msg00076`〕. **Not verified** in the emulator — the chain and `×85`
  were modelled in Python over all 256 inputs, not assembled. When reading an old ÷3 chain, the sign of
  its error tells which reciprocal it is.
- **÷15 without a multiply: the nibble sum.** Because 16 ≡ 1 (mod 15), `n = 16h + l = 15h + (h + l)`,
  so `n div 15 = h + (h + l) div 15` and `n mod 15 = (h + l) mod 15`, with `h + l` at most 30. Bob
  Colbert's 1997 positioning routine does exactly this — `and #$0F` for `l`, four `lsr` for `h`, `adc`
  the two, then one `cmp #$0F / bcc / sbc #$0F / iny` 〔stella-list `199709/msg00006`〕. No table, no
  multiply. **One conditional subtract is exact for 0..254 but not for 255**, where `h + l = 30` needs
  two (it returns 16 r15 instead of 17 r0) — checked in Python over all 256 inputs. Colbert's routine
  increments its input first, so its failing input is 254. Cycle cost: **Not verified**.

## When the divisor is a runtime value

Everything above divides by a constant. Two shapes for a divisor known only at run time, neither
implemented here:

- **Binary long division — exact.** Shift the dividend's top bit through carry into the accumulator;
  if the divisor fits, subtract it. MLdB: in binary *"the quotient of each step is always 1 or 0 … so
  there's no division involved only a single subtract"*, and the carry from that step is the quotient
  bit, rolled into *"the same variable that is used for the dividend, shifting the dividend out and
  the solution in"* 〔AtariAge `topic/280991`, 2018〕. Repeated subtraction (Omegamatrix's loop in the same
  thread, 7 instructions in its shorter form) is smaller, but its time grows with the quotient. MLdB's own version first
  scales the divisor up to its largest power-of-two multiple that fits 8 bits, then steps back down; he reported
  49 cycles for `255/255` and `128/128`, 243 for `0/1`, and 259 for `255/1`. **Cited only, not
  verified** — nobody else in the thread ran any of the routines, and none was assembled here.
- **Log/exp tables — approximate, any divisor.** Roger Williams' polar-to-cartesian converter (three
  tables, *"a bit under 700 bytes"* including trig) notes that the log and exp tables also do *"general
  purpose scaling multiplication and division … with few errors greater than 1 out to results of 45 or
  so"* 〔stella-list `200110/msg00292`〕. Asked for the remainder (Thomas Jentzsch, `msg00313`), Chris
  Wilkson gave `x = log(a) − log(b); q = int(antilog(x)); x = log(b) + log(q); r = a − int(antilog(x))`
  〔`msg00319`〕. Williams drew the line himself: the trick *"works because I accept the rounding errors
  … you couldn't use it to do base conversion because there are a lot of +-1 errors even close in. But
  for drawing a display which can be approximate, it does the job"* 〔`msg00322`〕. If `q` is off by
  one, the remainder `r = a − b·q` is off by the whole divisor `b` (**Not verified** — our arithmetic,
  not the thread's), so it is for screen positions, not for score digits (base conversion is his own
  counter-example). **Cited only, not verified**.
