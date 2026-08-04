# Technique — TIA PCM (digitized sample playback via volume)

**Goal:** play a digitized waveform **without TIA's tone generators**. Setting
`AUDC0 = AUDC1 = 0` makes each channel emit a steady "1" stream, so the **volume**
register (`AUDV`, 0–15) becomes the instantaneous amplitude = raw 4-bit PCM.
Summing both channels yields a **pseudo-5-bit DAC**: `AUDV0 + AUDV1` spans 0–30
(`log2(31) = 4.95` bit). Source: seagtgruff / Tjoppen / batari, AtariAge thread
#184034 (distilled in `reference/atariage/184034-*/notes.ja.md`).

Demo: `roms/techniques/tia_pcm.asm` (a fixed, looped sample plays continuously;
background color tracks the level for visible feedback).
CI: `scenarios/tia_pcm.json` (16 asserts: `AUDC0==0` and `AUDC1==0`, AUDV0/AUDV1
modulating to specific values across frames, plus golden frame + golden audio).

## How it works

1. **Silence the tone generators.** `AUDC=0` on both channels → AUDV is the raw
   amplitude, not a tone volume. This is the whole trick.
2. **Decode a sample per frame.** A small **1-bit ADPCM** decoder walks a fixed,
   looped bitstream. Each bit indexes a 16-entry state LUT
   (`nextState = StateLUT[(state<<1)|bit]`), and the state maps to a 0–30 level
   (`LevelLUT[state]`). One sample is decoded per frame (in overscan), so output
   is fully deterministic and the AUDV pair for any frame is fixed.
3. **Split the level across both AUDV (pseudo-5-bit), branchless, 12 cycles**
   (seagtgruff's optimum):

   ```
   lda level    ; 0..30
   lsr          ; A = level/2, carry = low bit
   sta AUDV0
   adc #0       ; add the low bit back
   sta AUDV1
   ```

   The value is halved into both channels; the low bit is folded into one side
   via the carry. No branch, no table.

## Sample-rate facts (NTSC, from #184034)

- Scanline rate = `3579575 Hz / 228 CC = 15699.89 Hz`.
- TIA emits **two audio clock pulses per line** (A-φ1, A-φ2) → native
  `≈ 31400 Hz`. Pulse spacing is uneven (112 / 116 CC) → **average 114 CC/sample**.
- Update rate sets the playback rate: 1×/line = 15700 Hz, 2×/line = 31400 Hz
  (TIA native), 3× = 47100 Hz, 4× = 62800 Hz. The demo updates **once per frame**
  (an envelope, not a high-rate voice) so the AUDV sequence is easy to assert; a
  real voice updates in the kernel (1–2×/line when drawing, more if blanked).
- `AUDV0 + AUDV1` mixes to a pseudo-5-bit DAC (0–30, ≈4.95 bit).

## CI / verified facts

- `AUDC0 == 0` and `AUDC1 == 0` across the run — pure PCM, no tone generator.
- AUDV **modulates over time** (rises to 15/15 ≈ peak, falls to 1/1 ≈ floor, then
  rises again — an audible, time-varying envelope, not a constant). Measured
  sequence includes frame 1 = (11,11), frame 5 = (15,15), frame 24 = (1,1),
  frame 30 = 4. The two AUDV channels split the level (`level/2` each + carry).
- `ntsc_frame_lines == 262`, `golden_frame: true`, `golden_audio: true` all pass.
- Background color = `level<<3 | hue`, so the screen is never black and pulses
  with the sample = visible companion to the audio.

## Grading a real stream — `internal/pcm` + `cmd/pcmcheck` (G3, 2026-08-04)

The demo above updates **once per frame**, which is why per-frame asserts are enough
for it. Digitised **speech** does not: the mined recipe (topic/234209, iesposta +
spiceware) writes AUDV0 **once per slot at a fixed rate**, 3900–4000 Hz for voice,
from samples packed **two 4-bit nibbles per byte** — and the thread's loudest warning
is not about values but about TIME (the old Berzerk speech hack made the TV **roll**
during playback because the loop ate the scanline budget).

So the check has two independent axes, both with the same denominator = the number of
intended samples:

| axis | pairing | what moves it | what does NOT |
|---|---|---|---|
| **value** | k-th write ↔ k-th intended sample | corrupt sample, wrong nibble order, short table | a uniform time shift |
| **timing** | absolute scanline vs `StartLine + k·LinesPerSample` | shift, dropped line, accumulating drift | a corrupted value |

plus a **clock histogram** — the intra-line beam clock of every write — because a
write that wanders inside its scanline is invisible at scanline resolution.

The anchor is **declared, never fitted.** The raw mixer capture
(`emu.EnableAudioCapture`) already contained every sample — measured: **144/144** of
the fixture's stream, recoverable from the 524-sample/frame mixer stream — but only
by searching 236 offsets for the best fit, and the same search fits a stream shifted
by a whole scanline equally perfectly (**144/144** again). A fitted anchor absorbs
exactly the drift the check exists to find.

Fixture: `roms/litmus/litmus_pcm.asm` — 144 samples/frame, one per scanline, high
nibble first, first sample on scanline 37 (3 VSYNC + 37 VBLANK), 262 lines. Its
sample table lives between `; PCM_TABLE_BEGIN` / `; PCM_TABLE_END` markers and is
**parsed out of the source** by the grader, so the player and the grader read the same
bytes and a typo in either cannot cancel out.

```
go run ./cmd/pcmcheck -rom roms/litmus/litmus_pcm.bin -asm roms/litmus/litmus_pcm.asm \
    -start 37 -pitch 1 -frames 3
frame 4: 144/144 samples captured; 144/144 values exact; 144/144 land in their slot,
144/144 within one line; mean pitch 1.000 lines/sample (declared 1); all writes at beam clock -23
```

Falsification (all in `internal/pcm/pcm_test.go`, all seen RED): a one-line shift →
`0/144 in slot` with values still `144/144`; a dropped sample → `143/144 captured,
63/144 in slot`; one corrupted value → `143/144 values, 144/144 in slot`; drift of one
line per 32 samples → `32/144 in slot, mean pitch 1.028`; intra-line jitter → two clock
buckets with both other axes clean; and two **ROM-level** mutants assembled from a
rewritten copy of the fixture — an extra `sta WSYNC` in the loop → `1/144 in slot,
mean pitch 1.503`, and `PACKED = 71` → `142/144 captured`.

**A control that did not fire, and what it means.** Editing a byte of the fixture's
table (`$FF` → `$F1`) left the grade at a perfect `144/144`: the table is the declared
intent, so changing it moves ROM and expectation together. This check answers *"does
the ROM deliver the waveform it declares, on time"*, never *"is that the right
waveform"*. A ROM-level value defect therefore has to break the **player** — narrowing
the low-nibble mask to `and #$07` gives `107/144 values exact, 144/144 still in slot`.

## Caveats

- A per-frame update is a slow "envelope," chosen for deterministic, readable
  asserts. For real digitized speech you stream samples in the kernel (1–2×/line),
  which trades display time for fidelity (#184034: prioritize **sample rate** and
  **compression** over bit depth). `litmus_pcm` is the per-line case.
- `pcmcheck` grades a stream on ONE volume register. The pseudo-5-bit variant above
  splits a level across AUDV0+AUDV1; grading that means running it twice, once per
  register, and the two halves are not independently meaningful.
- ADPCM here is a compact didactic LUT (16 states, 0–30 levels). Tjoppen's
  production codec is a 62-byte table tuned by an encoder against a WAV; same
  shape (`next = ADPCMTable[(sample<<1)|bit]`), better fit.
