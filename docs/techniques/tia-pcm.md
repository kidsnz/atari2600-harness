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

1. **Silence the tone generators.** `AUDC=0` **or `AUDC=11`** on both channels → AUDV is the raw
   amplitude, not a tone volume. This is the whole trick.
   **`AUDC=11` does the same job** — measured 2026-09-04, `litmus_audc_carrier` /
   `internal/emu/audccarrier_test.go`: of all sixteen settings, **exactly 0 and 11**
   hold one sample value at every AUDF tried, out to 1000 frames, and they hold it
   at the volume written. The other fourteen break within four frames. That gives a
   second silent carrier — the escape route when AUDC is wanted for something else
   on the channel being used as the DC output. Stated by Eckhard Stolberg in
   stella-list `199902/msg00036` (*"If you set the AUDCx register to 0 or 11, the
   output will always be high"*) and unmeasured here until now.

   ★**The technique is two years older than that citation.** Eckhard Stolberg posted
   working code for it on 1997-02-27: *"One of the things, that I wanted to add to it,
   but don't have enough space for, is **volume samples**. I thought some of you might
   want to use that in your games. So here are my test programs"* 〔`199702/msg00018`〕,
   with `say.a65` — the source, AUDV0/AUDV1 and all — in the same batch 〔`199702/msg00020`〕
   and a `wavconv` thread beside it 〔`199702/msg00017`〕. So the earliest record here was
   not the earliest record: **the 1999 message explains the silent CARRIER, and the 1997
   one is the technique itself, already running.** Found by the mailing-list distillation
   (helper-3), verified here against the raw archive.
   **Do not take "constant" from a short window.** The first measurement of this used
   sixty frames of a ROM that wrote AUDC once at boot and reported four more silent
   carriers (2, 6, 10, 14 at AUDF=31). All four were false: AUDC=2 holds for 89 frames
   in that ROM and breaks in **one** frame in a ROM that reaches the same register
   values by a different path. The polynomial counters carry history, so a value
   measured in one state cannot be told apart from a constant.
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

- Scanline rate = `3579545 Hz / 228 CC = 15699.76 Hz` (the colour subcarrier is 5 MHz × 63/88 = 3,579,545.45 Hz; `pkg/audio/audio.go` uses the same clock). #184034 used 3579575 Hz / 15699.89 Hz — the marking on one console's crystal, 8 ppm above nominal 〔stella-list `200311/msg00157`〕; the nominal is used here.
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

## Making the sample data

The demo's levels come from a LUT. A real voice starts as a recording, and what is done to it
before it becomes nibbles is part of the technique. Everything in this section is Cited only, not
verified: no ROM here is built from a recording.

**Filter before you quantise.** Glenn Saunders, in the list thread on Eckhard Stolberg's 1997
sampled voice ("Stella says ..."): *"One of the tricks to low bitdepth, low sample rate samples is to
equalize it during the sampling phase. This usually involves rolling off the high frequencies which
will wind up as noise anyway."* He set the ceiling conditionally: *"If the 2600 system is like the
Atari 8-bit, then we're talking about frequency responses that are at telephone quality at best. So
keep tones<5khz and filter out the upper harmonics"* 〔stella-list `199703/msg00002`〕. reveng, twenty
years later: *"The source sample should also be low-pass filtered to drop any frequencies over the
Nyquist Frequency"* 〔AtariAge `topic/272948`〕. The SoX `lowpass 2000 rate 4000` recipe named under
G3 in `docs/capability-gap-audit.md` is this step; nothing here measures what it buys.

**The conversion chain used for Draconian.** SpiceWare: *"Samples need to first be converted to
unsigned 8 bit raw format, which I used sox to create:"* `sox $file -b 8 -u $root.raw`, then
`raw_to_dpc $root.raw $root.pds` — a small C program he attached in answer to a request for *"a tool
to convert files to 4 bit (high and low nibbles in one byte)"* — and the result goes into the source
with `INCBIN` 〔AtariAge `topic/273769`, 2018〕. `.pds` is not a standard format: *"I made that up, PDS =
Packed Digital Samples"*. The chain broke on the asker's SoX: *"the switch -u was not recognized so I
used"* `-e unsigned-integer`. The thread does not say which nibble `raw_to_dpc` puts first
(`cmd/pcmcheck` takes either, `-low-first`), and `pcmcheck` cannot read an `INCBIN` table at all
(G3, `docs/capability-gap-audit.md`).

**Fit the ROM first, then set the rate — for some phrases.** Mike Mika, on the voices he added to
Berzerk, set the rate per phrase, not per game: *"Intruder Alert! Intruder Alert!"* — *"I stored the
one half of the phrase, and played it back every 4 scan lines like Eckhard's demo, twice"*; *"Chicken
Fight Like A Robot"* — *"sampled this at a bit less than 4000hz, and update about every 4.5 lines (I
believe)"*. Of "Humanoid must not escape" (and, by "Same thing here", Chicken Fight): *"in an effort
to make the samples fit, I downsampled below 4000hz until it could be stored in a bank + code, then
adjusted the playback rate."* One more phrase was dropped because *"it had to be so downsampled it
was too unrecognizable"* 〔stella-list `200208/msg00080`〕. `pcmcheck`'s `-pitch` is a
whole number of lines, so a 4.5-line phrase cannot be declared to it.

**One sample per line, and long clips.** rbairos, converting zackattack's demo: *"I simply took the
audio channels, averaged them, resampled to 60*262 HZ, scaled slightly, then remapped 0..15"* —
60 × 262 = 15720 Hz, a round figure for the 15699.76 Hz line rate above. That demo plays *"about 30
seconds of sampled audio"*; it was thrown together while testing *"a routine that runs in zeropage
memory during overscan and vblank"*, stores its samples *"in a very inefficient manner"* on purpose,
and *"If someone only cared about playing back audio it would be possible to fit more than a minute
into a single rom."* It is a 3E image — DirtyHairy: it runs in 6502.ts/Stellerator *"if you set the
cartridge type to "bank switched 3E (Tigervision + RAM)" manually --- it won't autodetect"*. His own
5-bit driver, also 3E, *"plays 510k of packed 5bit samples"*; *"the first three bytes of each bank
(=2048 byte block) are ignored, so there is room for 2045 * 255 * 8 / 5 = 834360 samples before it
loops"*, and he had not yet got his ROMs or zackattack's to run on a Harmony Encore 〔AtariAge `topic/272948`,
2017-12〕. 3E is in `docs/techniques/bankswitching.md` (`roms/carts/cart_3e.asm`); no ROM here
streams audio across banks.

## Caveats

- A per-frame update is a slow "envelope," chosen for deterministic, readable
  asserts. For real digitized speech you stream samples in the kernel (1–2×/line),
  which trades display time for fidelity (#184034: prioritize **sample rate** and
  **compression** over bit depth). `litmus_pcm` is the per-line case.
  Two plans on record pay that trade only while the sound plays, rather than for the whole game:
  Kevin Horton's software-mixed music (last section of this page) — *"The only downside is there can be no video
  while this is occurring (except for maybe flashing the screen or some other relatively static
  display)"* 〔stella-list `200109/msg00301`〕 — and kylearan's Space Taxi design: *"During speech,
  the game might show a more simplified version of the level (without colors for example, or without
  some objects), but it should be doable without having to switch the screen off"*, on the
  condition *"if the game will be bigger than 4K"* 〔AtariAge `topic/261054`〕. Both are plans, not
  shipped kernels — Cited only, not verified.
- **Blanking the screen for speech is not the same as dropping sync.** omegamatrix, on testing
  iesposta's speech strings for the Dr Who hack of Berzerk: *"You will run into trouble on so some
  modern TV's if you blank the display and let go of handling VSYNC. I have a Toshiba 55" LED TV, and
  when it looses sync it mutes the sound. There is no setting to stop that"* — he saw *"a black screen
  with no sound"*. The fix was *"a kernel that blanked the screen while still keeping sync"*: *"If
  you're updating the audio every line, or every second line, then it is easy to do. Dr Who was a
  little more work as it was every 4 lines, and 262 is not divisible by 4. I didn't want to do 260 or
  264 line game"* 〔AtariAge `topic/247859`, 2016-01〕. The muting rests on one television; the 4-line
  pitch is the one `litmus_pcm.asm` says a real voice would use. Cited only, not verified.
- `pcmcheck` grades a stream on ONE volume register. The pseudo-5-bit variant above
  splits a level across AUDV0+AUDV1; grading that means running it twice, once per
  register, and the two halves are not independently meaningful.
- **The pseudo-5-bit sum is not 31 equal steps of output.** The split above and the demo's
  `LevelLUT` are linear; the mixer is not. The engine's mixer table is a compressive curve in the
  SUM of the two volumes (`docs/known-traps.md`, "two voices at high volume squash each other";
  `internal/emu/mixnonlinear_test.go`), so how a level is split between AUDV0 and AUDV1 does not
  matter there, but a waveform quantised evenly onto 0–30 comes out bent. DirtyHairy built the same
  5-bit sine scale twice, one ROM *"created using naive quantization, the other (test_nonlinear)
  corrects for the TIAs nonlinear response. To my ears, the nonlinear version sounds cleaner, while
  the linear one exhibits a ringing effect"* — compared in Stellerator, not on a console. reveng
  thought the curve itself harmless (*"I don't think it's a big problem. The non-linearity still gives
  31 unique fairly-evenly distributed values"*) but the quantiser not: *"But in my mind, 5-bit is
  pointless without the non-linear resampling."* Sheddy, from POKEY: *"Without non-linear resampling,
  combining channels still gives a noticable improvement there"* 〔AtariAge `topic/272948`〕. The
  thread's formula is the one from `topic/271920`, which `known-traps.md` takes to be the engine's own
  curve (the equation itself is not in the copy held here), so the engine cannot confirm the hardware
  here. Cited only, not verified.
- ADPCM here is a compact didactic LUT (16 states, 0–30 levels). Tjoppen's
  production codec is a 62-byte table tuned by an encoder against a WAV; same
  shape (`next = ADPCMTable[(sample<<1)|bit]`), better fit.

## The silent carrier is two values, not one (2026-09-07)

Eckhard Stolberg, asked how to get one-bit sound out of the TIA: *"If you set the AUDCx register to
**0 or 11**, the output will always be high. You can generate complex waves by quickly changing the
AUDVx register for that voice"* 〔`199902/msg00036`〕. This page took the first half; `litmus_pcm.asm`
says *"AUDC0 = 0 and AUDF0 = 0 for the whole run"*. **11 appeared nowhere.**

Measured by rebuilding the litmus with five values of `AUDC0` and comparing the audio mix digest over
ten frames (`internal/emu/pcmcarrier_test.go`):

| AUDC | digest | |
|---|---|---|
| 0 | `d323059a…` | the reference |
| 1 | `1e6efd69…` | different |
| 4 | `d33c513f…` | different |
| **11** | **`d323059a…`** | **byte-identical** |
| 12 | `160c6c9a…` | different |

★**Exactly 0 and 11, with both of their neighbours differing** — a pair of points, not a range. So the
precondition is `AUDC ∈ {0, 11}` and a driver that already holds 11 there does not have to write
anything.

★★**The confound this nearly had**: the litmus feeds one `lda #0` to three stores — `AUDC0`, `AUDF0`
and `AUDV0` — so replacing that literal would have moved the frequency and the volume too, and the
digests would have differed for reasons unrelated to the tone generator. The test splits the load
first, and fails loudly if the setup block ever changes shape. Found by the mailing-list distillation
(helper-1).

## Pitch from a timed loop instead of AUDF (2017)

With the carrier silent, the CPU alone can set the pitch. BNE Jeff's first routine *"changes
frequency without changing the frequency register- AUDFx"* 〔AtariAge `topic/264918`〕. SpiceWare
recorded it on a console — *"Sounds OK to me"*, *"That was on my 2600"* — where the author had heard
*"2 or 3 little breaks"* in his Stella (4.7.3); on his Harmony it *"worked correctly"*. In the tune
version's posted source: write `#$0F` to `AUDV0`, spin a `dec`/`bne` loop (*"8 machine cycles per
loop through"*) for half a period, write `#$00`, spin again. The intended pitch is the table's loop count
(C5 = 141 passes per half-wave) — and the program does nothing else: no VSYNC,
no WSYNC, no picture, and AUDC0 is 0 only because its start-up loop clears `$01`–`$FF`. That version
came out *"way,way out of tune"*, and the thread ends without a cause. Cited only, not verified. Our
reading of that source: the duration loop jumps back to the high half-wave with the counter already at
0, so after a note's first period every high half-wave runs 256 passes whatever the note. Not verified.

## A fifth bit from one register, in time, as proposed (2005)

The pseudo-5-bit trick above sums two registers at once. Manuel Rotschkar (`cybergoth`), 2005, gave
the same arithmetic spread over time on one register: *"you can fake 5-Bit quality samples with the
TIAs 4-Bit volume register by dividing each 5-Bit sample in two 4-Bit samples and feeding them to the
TIA twice as fast as the 5-Bit sample-speed"* — his example is 23 (*"/2 = 11.5"*) played as 11 and 12 —
and asked *"Isn't that called "oversampling"?"* 〔stella-list `200508/msg00064`〕. It was his reply to
B. Watson's proposal to toggle two adjacent pitches on alternate frames (the pitch side is
`pitch-dither.md`): *"Techniques like that are normally rather used with the volume than with the
frequency"*. None of the thread's sixteen messages reports trying it. **Not verified** — the nearest
measurement here is the engine's, not a console's: `internal/emu/audvtwice_test.go` writes AUDV0 `$0F`
then `$00` on every scanline (AUDC=0) and finds the played sample is the two values averaged by how
long each held within a 38-cycle averaging window, not the last one written; an alternation like 11
and 12 at a sample rate was not tried.

## The other direction: one volume register split among several voices (2001-09)

The pseudo-5-bit trick above adds two registers into ONE sample. Kevin Horton proposed the reverse:
several software oscillators (a phase accumulator per voice stepping through an 8-byte waveform
table) summed into ONE register, so the register's 16 levels are divided among the voices —
*"Each "channel" can range from 0 to 5. This gives 5*3 or a maximum of 15 levels used. Since there
are two volume registers, 3*2 = 6."* He also meant to leave the TIA's own generators usable: *"I was
thinking of defaulting the channel volume to 1, so the TIA's sound regs could still be used to
generate things like percussion and SFX or something."* 〔stella-list `200109/msg00312`〕 Thomas
Jentzsch asked whether the same split goes the other ways — *"So, you could also produce 2 channels
with 8 volume values, 4 channels with 4 values, or 8 channes with 2 values (if there would be enough
cpu time). And the channels only loose some (or a lot of) dynamic. Correct?"* 〔`200109/msg00336`〕 —
and the thread's eight messages carry no answer. Horton posted fragments (the add chain, two
waveform tables) but no working player; the cost he named is the picture (Caveats, above).
Cited only, not verified.

Andrew Schwerin described a running engine in 1999 and posted its inner loop (*"I took out my
interface code"*): a "Quad" loop that adds two 3-bit
sine lookups (values 0–6) into each of AUDV0 and AUDV1, each voice a 16-bit fixed-point pointer
stepping through one 256-byte table — *"I have played chords on this engine"*. In the same message he
named the trade that removes the real-time mix: *"if the music is always the same, the music can be
presampled. Instead of reading a wavetable for each voice, read a wavetable for each channel. The
chords get mashed together at assembly time and not in real-time."* The cost he gave: *"The limitation
here is memory storage, and lack of appropriate tools to design music & soundtracks"* 〔stella-list
`199904/msg00006`〕. Chris Wilkson's suggestion just before, to store note data per channel rather than
per voice 〔`199904/msg00005`〕, is a different thing, and Schwerin said so: *"I don't see how I can
combine the musical information for two voices into one channel. (Other than wavetable
precomputation, which is a different matter than musical note data for a song)."* Cited only, not
verified.
