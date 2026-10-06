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

When one factor is a known constant the loop is not needed: write the constant in binary and add
shifted copies. Andrew Davie's ×10 (`%1010`) is `lda n / asl / sta temp / asl / asl / adc temp` —
*"a x10 with just 15 cycles of processor time, and 9 bytes"* — with the `CLC` left out, as he
explained for ×3, *"on the assumption that it would be cleared by the shift (asl) instruction
before"*, a bound he states himself (*"assuming n was < 127"* for ×3, *"assuming we won't have
overflow"* for ×10). The other end of the
same trade is a ROM table, `ldx n / lda times10,x`, which *"only took 7 cycles"* 〔stella-list
`199805/msg00155`〕. For two variable factors, Robin Harbron passed on the quarter-square identity from
The Fridge: with `f(x) = x^2/4`, *"a*b = f(a+b) - f(a-b)"*, two table reads and a subtract
〔`199806/msg00020`〕 — a table, which this page set out to avoid. A floored `f` stays exact, because
`a+b` and `a-b` have the same parity (our arithmetic). **Cited only, not verified** — none of these
was assembled here.

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
  increments its input first, so its failing input is 254. It was already in circulation: five months
  earlier Erik Mooney decoded the same steps in a routine Piero Cavina had posted as taken from
  Air-Sea Battle 〔`199704/msg00015`, `199704/msg00043`〕, and in 2001 Thomas Jentzsch
  answered Andrew Davie's reinvention 〔`200102/msg00088`〕 with *"I think it's an old standard
  routine which is based on the same idea"* 〔`200102/msg00091`〕. Cycle cost, as posted
  (**Cited only, not verified**): Mooney gave
  *"between 59 and 73 cycles plus the JSR and RTS"* for the Air-Sea Battle version (range check and
  HMxx packing included) against *"a range of 30 to 100"* for the subtract-15 loop — *"The average is
  about the same, but I like the new one because it's more consistent"* — and said that with the range
  check removed, as a macro, *"it will fit into one scanline if needed"* 〔`199704/msg00051`〕.
  Jentzsch's per-instruction comments add up to 46 or 52 cycles including the `RTS`; Davie moved the
  `ldy` ahead of the compare and swapped `inc tmpVar` for `iny`, *"a lousy three cycles better"*
  〔`200102/msg00096`〕 — on the path that subtracts, 52 to 49 by our count of his comments.
- **The coarse position is time, not a number.** Every ÷15 on this page — the reciprocal helper and
  the nibble sum alike — returns the coarse count as a value, and the beam still has to be walked
  there. Colbert's routine above follows `calcpos` with a `DEY/BPL` wait after `WSYNC`, and he says of
  it *"it wastes 2 entire scanline"* 〔`199709/msg00006`〕. The routine Manuel Polik posted from
  Battlezone runs the `SBC #$0F / BCS` loop itself right after `WSYNC / HMOVE`, so the subtract is
  also the wait (the mechanism `hmove-two-step.md` describes as *burning cycles proportional to X*)
  〔`200210/msg00281`〕; he thought it could *"in some cases probably save a whole scannline"*
  〔`200210/msg00284`〕 and later labelled the loop *"2 in 1 magic"* 〔`200211/msg00165`〕. So neither
  helper here replaces that loop on a positioning line; they are for when the quotient is wanted as a
  number. ROM can take over the calculation, though not the wait: Colbert noted a line could be saved
  by precalculating the coarse and fine values into a table 〔`199709/msg00006`〕, and Piero Cavina replaced
  *"the well known routine"* with a 160-byte table
  *"to save RAM and CPU time, things more important than ROM at the moment"* 〔`199704/msg00152`〕.
  A dedicated-line form of the loop (`sec / sta HMCLR / sta WSYNC`, the loop, `eor #7`, four `asl`,
  `sta.wx HMP0,X`, `sta RESP0,X`, `sta WSYNC / sta HMOVE`) is reported to finish exactly 79 cycles
  after the first `WSYNC` for every position up to 160 〔AtariAge `topic/125115`, 2008〕 — constant
  because the closing `WSYNC` absorbs the loop's variable length (our reading). The 152 cycles
  `hmove-two-step.md` measured at X≥150 (`assert_line_budget`) were on a shared line in the PONG
  kernel — one that also carries other work, by our reading of that kernel — which is a different
  setting from this dedicated line, not a measurement of it.
  **Cited only, not verified** — none of these routines was assembled here.
- **÷36 from a ÷3 result.** For a value 0..107 already divided by 3 (so 0..35, in A), an AtariAge
  thread finishes ÷36 with two compares, `tax / lda #0 / cpx #12 / rol / cpx #24 / adc #0` — 10 bytes,
  12 cycles, A = 0, 1 or 2 — used there to sort a sprite position into three screen areas 〔AtariAge
  `topic/271568`, 2017〕. The `rol` stands in for a first `adc #0` and saves a byte. It is exact on that
  range because `n div 36 = (n div 3) div 12` (our arithmetic). **Cited only, not verified**.
- **Rounding a constant at assembly time.** DASM evaluates expressions in integers and truncates:
  Dennis Debro's `[(3 * 256 / 16)* 6] / 5` came out 57 where he wanted 58, and a literal `2.5` was
  not accepted (*"unless I'm missing something"*) 〔stella-list `200412/msg00001`, `msg00003`〕.
  Adam Wozniak's general form: for `N/D` rounded to the nearest whole number,
  *"compute (2*N+D) / (2*D)"* 〔`msg00006`〕. The reciprocal
  constants above are typed in, not computed; written as `256/d` in DASM they would come out as the
  floor (85 / 36 / 25 / 17), which matches none of the four, and the `ceil` the table holds is
  `(256+d-1)/d` (our arithmetic). **Cited only, not verified** — not assembled here.
- **Binary to BCD without dividing by 10: double dabble.** The ÷10 here splits a byte into digits by
  quotient and remainder. Kevin Horton's 6502 routine converts a 16-bit value to packed BCD by shifting
  it left one bit at a time into three result bytes and, after every shift but the last, adding 3 to any
  BCD nibble of 5 or more (`adc #3 / and #8` tests the low nibble, `adc #$30 / and #$80` the high) —
  *"it requires no temp variable storage"*, in his words 〔stella-list `200102/msg00104`〕. Our reading
  of the posted code finds two things: it shifts the input bytes out, so the input does not survive; and
  the rotate chain runs `bcd_out2` → `bcd_out1` → `bcd_out0`, which makes `bcd_out0` the most
  significant byte, though its header comment says `bcd_out2`. **Not verified** — neither assembled nor
  run here; the post gives no cycle count.

## When the divisor is a runtime value

Everything above divides by a constant. Three shapes for a divisor known only at run time, none
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
  By Williams' account (2001) the converter runs *"in 93 to 114 machine cycles"* and *"has been
  tested in Z26, Stella, and on a real 2600"* 〔`msg00292`〕; **Cited only, not verified**.
- **A percentage, `n*100/t`, two ways.** Asked for the share of a level's secrets found (`t` up to 50,
  different every level), gauntman's direct route — `n*100` by shifts and adds, then a 16-by-8 division —
  took about 362–397 cycles (average 377) and is exact; groovybee's keeps a table of `(100<<3)/t`
  and adds one entry into a 16-bit accumulator each time a secret is found, so reading the percentage
  is three 16-bit right shifts, at the cost of rounding error that accumulates, put at up to 4%
  〔AtariAge `topic/139620`, 2009〕. **Cited only, not verified**.
