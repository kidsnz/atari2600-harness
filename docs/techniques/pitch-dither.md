# Technique — pitch dither (play a note the TIA has no register for)

**Goal:** reach a pitch that falls BETWEEN two AUDF rungs, by alternating between them **every
frame** so the mean period lands on the target. In the bass this is the difference between a
key being playable and not.

**Status:** measured on the machine, `roms/litmus/litmus_pitchdither.asm` +
`internal/audioingest/pitchdither_test.go`. The ROM takes the waveform, the rung pair, the swap
rate and the scanline of the store from RAM, so one machine produces every case instead of
several builds assumed to be alike. **In use**: `roms/technojacket` `cover-fsharp` plays
"Bassline" in the record's own key, measured on the ROM at the piece's real seven-frame note
length -- D2 -0.9 c, E2 -6.0 c, F#2 +14.7 c, where the nearest single registers are -27.1,
+29.8 and -25.8.
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

Write one AUDF this frame, the adjacent one next frame, and repeat. The tone's period
alternates between the two rungs and the result sits at the **mean period**:

```
    mean = 2 / (1/f_lo + 1/f_hi)        NOT (f_lo + f_hi) / 2
```

Measured on the machine, over four notes spanning the bass and the melody register, each at
five different scanlines for the store so the result cannot be an accident of where the write
lands:

| note | nearest single rung | **swapped every frame** | spread over 5 write positions |
|---|---|---|---|
| E1 41.203 Hz (AUDC 6, 24/23) | −26.9 c | **+9.1 c** | 0.4 c |
| D2 73.416 Hz (AUDC 1, 28/27) | −27.1 c | **+3.3 c** | 0.5 c |
| E2 82.407 Hz (AUDC 1, 25/24) | +29.8 c | **−3.6 c** | 1.5 c |
| F#2 92.499 Hz (AUDC 1, 22/21) | −25.8 c | **+13.8 c** | 0.4 c |

Swapping every **two** frames also lands near the target but is markedly less stable — F#2
moved 14.3 cents across the same five write positions, against 0.4 for the per-frame swap.
Every three frames is stable again but no more accurate. **Use one frame.**

Applied to the whole figure of "Bassline" in its own key, the worst degree goes from +45.7
cents to +14.2.

Playing both rungs at once, one per channel, does NOT fuse: the spectrum keeps two separate
peaks, the estimator locks to one of them, and it costs both channels.

## ⚠️ A first version of this page said the opposite, and was wrong

It claimed that swapping every frame fails and that two frames is required, with a rule about
the swap period having to exceed the note's own. That came from ONE ROM at ONE write position,
where the per-frame swap measured 40.00 Hz. Adding an unrelated `sta WSYNC` elsewhere in that
ROM's VBLANK moved the same measurement to 41.17 Hz, and sweeping the store across five
scanlines showed the per-frame swap is in fact the **most** stable of the three rates.

Two lessons, both cheap to state and both expensive to have skipped:

- a mechanism measured at a single operating point is not measured. The swap rate, the note,
  and the position of the store are three axes and the first version varied none of them.
- an F0 estimator returns subharmonics. The same sweep read D2's dither as 36.8 Hz — exactly
  half — until the search range was narrowed around the note. A confident wrong octave looks
  identical to a confident right one.

## ⚠️ A DUTY RATIO does not work, and it is the obvious next idea

A 1:1 swap can only reach the midpoint of a pair of rungs. Where the target does not sit
near that midpoint — D3 at 146.8 Hz is 119 cents from its neighbour and lands nowhere
useful — the obvious fix is to hold one rung twice as long and land a third of the way.
The arithmetic says that puts D3 at −7 cents.

Measured, it does not. Holding the sharp rung two frames to the flat one's one gives
**+30.6 cents**, which is the sharp rung itself (+33.6); the reverse gives −84.8, which
is the flat rung (−85.5). At that pitch a frame holds two and a half cycles, so each
frame is heard as its own pitch and the longer one simply wins. There is no weighted
mean.

| D3 146.832 Hz, AUDC 1, rungs 14/13 | measured | spread over 4 write positions |
|---|---|---|
| 1:1 | −20.8 c | 0.2 c |
| 2:1 (sharp held longer) | +30.6 c | 0.1 c |
| 1:2 (flat held longer) | −84.8 c | 1.0 c |

So the technique gives **one** in-between pitch per pair, not a continuum, and it is the
midpoint. A note that needs something else needs a different key or a different octave —
`technojacket`'s `cover-fs-hi` went up a fifth rather than up an octave for exactly this.

## It costs nothing audible

A modulation puts energy either side of the note. Measured as the fraction of spectral energy
outside a fifth either side of the note:

| | outside the note |
|---|---|
| steady tone (E1) | 0.063 |
| dithered (E1, every frame) | 0.064 |
| two channels detuned (E1) | 0.175 |

The working mechanism is indistinguishable from a steady tone. The detune — the other obvious
idea — is 3.5× noisier **and** costs both channels for one note, which on a machine with two
channels means the drums or the bass has to go.

## Cost

One bit of frame parity, in VBLANK, and the note table has to carry a flag saying which notes
are dithered. No kernel cost at all: the kernel never sees this.

⚠️ **If a picture is driven from the AUDF byte** — as `technojacket`'s piano roll is, where the
column is `(COLBASE − AUDF) × 2` — the alternation makes the drawn column flicker between two
neighbours. Drive the picture from the note's BASE value, not from what was last written.

## Where to look

- `roms/litmus/litmus_pitchdither.asm` — the instrument; `$80` mode, `$82` AUDC, `$83` the flat
  rung, `$84` the swap rate, `$87` the scanline the store lands on
- `internal/audioingest/pitchdither_test.go` — the measurement, including the detune that does
  not fuse and the roughness figure
- `internal/keyfit` — which key a figure can be played in, single rungs only; this technique is
  not yet folded into it
