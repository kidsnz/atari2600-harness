# Technique — pitch dither (play a note the TIA has no register for)

**Goal:** reach a pitch that falls BETWEEN two AUDF rungs, by alternating between them every
**two frames** so the mean period lands on the target. In the bass this is the difference
between a key being playable and not.

**Status:** measured on the machine, `roms/litmus/litmus_pitchdither.asm` +
`internal/audioingest/pitchdither_test.go` (5 tests: two controls, the working case, the
failing case, and a roughness measurement). **Not yet used in a piece.**
**Source:** in-house, 2026-08-09, reproducing Satoshi Tomiie's "Bassline" — the record's key
was rejected because D and E are more than 23 cents out in every bass octave, and this is the
mechanism that would have let it be kept.

## The problem it solves

`freq = clock / divisor / (AUDF+1)`. The rungs are set by an integer divide, so they crowd
together at the top and spread out at the bottom. Measured in the bass, on every playable
waveform:

| note | best single rung | error |
|---|---|---|
| D1 36.708 Hz | AUDC 6 AUDF 27 = 36.221 | −23.1 c |
| E1 41.203 Hz | AUDC 6 AUDF 24 = 40.568 | −26.9 c |
| B1 61.735 Hz | AUDC 6 AUDF 15 = 63.39 | +45.7 c |
| E2 82.407 Hz | AUDC 1 AUDF 24 = 83.84 | +29.8 c |

25 cents is a quarter-tone. A figure with two of these in it does not sound like the record
in any key the machine can hold.

## The pattern

Write one AUDF for two frames, the adjacent one for the next two, and repeat. The tone's
period alternates between the two rungs and the ear integrates the **mean period**:

```
    mean = 2 / (1/f_lo + 1/f_hi)        NOT (f_lo + f_hi) / 2
```

Measured on the machine, E1 with AUDF 23/24 on AUDC 6:

| what the ROM does | measured | vs E1 |
|---|---|---|
| AUDF 24 held (control) | 40.57 Hz | −26.9 c |
| AUDF 23 held (control) | 42.26 Hz | +43.8 c |
| **alternate every 2 frames** | **41.40 Hz** | **+8.8 c** |
| alternate every frame | 40.00 Hz | −41.7 c |
| ch0 23 + ch1 24 at once | two separate peaks | does not fuse |

Applied to the whole figure of "Bassline" in its own key, the worst degree goes from
+45.7 cents to +14.2.

## ⚠️ The alternation period must EXCEED the note's period

This is the whole rule, and the obvious implementation breaks it. E1's period is 24.2 ms; a
frame is 16.7 ms. Swapping **every frame** changes AUDF in the middle of nearly every cycle,
so neither value ever completes one — and the result is not between the rungs, it is 41.7
cents BELOW BOTH, worse than not doing it at all. Two frames is 33.4 ms, and each value gets
a whole cycle to itself.

So this is a **window**, not a direction:

- too fast (period < the note's) → lands below both rungs, useless
- too slow → the two pitches separate and it is heard as a trill

At 60 Hz frames the window opens for notes below about 60 Hz on a 2-frame swap. A higher note
needs the swap inside a frame, which this does not do.

## It costs nothing audible

A modulation puts energy either side of the note. Measured as the fraction of spectral energy
in 5–35 Hz and 55–95 Hz against the note's own 35–55 Hz:

| | outside the note |
|---|---|
| steady tone | 0.063 |
| alternating every 2 frames | 0.065 |
| alternating every frame | 0.066 |
| two channels detuned | 0.218 |

The working mechanism is indistinguishable from a steady tone. The detune — the other obvious
idea — is 3.5× noisier **and** costs both channels for one note, which on a machine with two
channels means the drums or the bass has to go.

## Cost

One byte of state (which of the pair is current) and a 2-frame counter, in VBLANK. No kernel
cost at all: the kernel never sees this.

⚠️ **If a picture is driven from the AUDF byte** — as `technojacket`'s piano roll is, where the
column is `(COLBASE − AUDF) × 2` — the alternation makes the drawn column flicker between two
neighbours. Drive the picture from the note's BASE value, not from what was last written.

## Where to look

- `roms/litmus/litmus_pitchdither.asm` — five modes selected from RAM `$80`, so one machine
  produces every case instead of five builds that are assumed to be alike
- `internal/audioingest/pitchdither_test.go` — the measurement, including the negative control
  that fails and the roughness figure
- `internal/keyfit` — which key a figure can be played in, single rungs only; this technique is
  not yet folded into it
