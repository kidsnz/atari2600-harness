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

## Caveats

- A per-frame update is a slow "envelope," chosen for deterministic, readable
  asserts. For real digitized speech you stream samples in the kernel (1–2×/line),
  which trades display time for fidelity (#184034: prioritize **sample rate** and
  **compression** over bit depth).
- ADPCM here is a compact didactic LUT (16 states, 0–30 levels). Tjoppen's
  production codec is a 62-byte table tuned by an encoder against a WAV; same
  shape (`next = ADPCMTable[(sample<<1)|bit]`), better fit.
