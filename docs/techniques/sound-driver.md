# Technique — in-game sound driver (music + SFX priority)

**Source:** standard 2600 music+SFX driver architecture; hardware basis `litmus_audio`, tuning math in `pkg/audio`; cf. `music-driver.md` (TIATracker, AtariAge topic 250014).

**Goal:** the audio architecture every real game uses: looping 2-voice music from data tables,
with channel 1 **preempted by SFX** and restored when the effect ends — all inside a normal game
frame (driver tick in overscan, timer-managed so the line count never depends on code paths).

## ⚠ What this repository can and cannot tell you about sound

**Everything here reads the audio REGISTERS.** `read_audio`, the audio golden, `pkg/audio`'s spectra,
the scenario asserts — all of them answer *what was written to `AUDC`/`AUDF`/`AUDV`, and when*. That
is a strong guarantee and it is not the same guarantee as *how it sounds*.

The list found the gap the hard way. Manuel Polik, `200405/msg00275`, on his own finished game:
*"[on Z26] brilliant and crystal clear … [on a Cuttle Cart, on hardware] horribly distorted … the
voice-line 'vibrated'."* He suspected his driver, read trace logs for hours, and concluded *"the
driver did precisely what I wanted it to do."* **A register-level check would have passed, because
the registers were right.** His minimum reproduction is six lines from `CLEAN_START`:

```asm
        lda #$3A
        sta AUDF1
        lda #$06
        sta AUDC1
        lda #$0F        ; <- the volume is the variable that breaks it
        sta AUDV1
```

The fix was *"turning the volume down"* — voice at 10, bass at 8. His hypothesis was that mixing
`AUDC` 6 and 12 at full volume distorts in a PAL console's mono summing; reproduced on a 6-switch,
a Jr. and a third person's machine. **We hold `AUDC` 6 and 12 individually** (`pkg/audio`'s
`MeasuredSpectra` covers `{1,2,4,6,7,12,14,15}`) **and nothing at all about mixing them loud.**

This is a limit of the approach, not a missing feature: an oracle that reads registers cannot see an
analogue summing problem downstream of them. Two consequences worth stating plainly:

- **Register-correct is the claim; audible-correct is not.** Say so when reporting sound results.
- **Somebody has to listen**, on as many outputs as can be reached, and full-volume combinations
  deserve the most suspicion. That is not a weakness of the pipeline; it is where the pipeline ends.

The same month, the same author noted that Stella and Z26 had *separate* sound implementations, both
with problems. Reading registers means never inheriting either one's bugs — and never seeing the
distortion either. Found by the mailing-list distillation (helper-1).

Demo: `roms/techniques/sound_driver.asm` (original 144-frame loop; fire triggers a laser).
CI: `scenarios/sound_driver.json` (music states, preemption, restore, 262 lines, audio golden).
Companions: `sound-effects.md` (the SFX tables) and `cmd/jingle` (compose → the same Notes/Durs
table format) / `cmd/dissect -audio` (transcribe back).

## Structure

- **ch0 = lead**: jingle-compatible `Notes0/Durs0` tables (AUDF per event, `$FF` = rest),
  advanced by a per-frame `dec dur / Adv` tick. AUDC/volume fixed per voice.
- **ch1 = bass + SFX**: same music tick, but the current note is kept in `m1f` and written
  through `WriteM1`. While `sfxOn`, the music tick **keeps advancing time but does not touch the
  registers**; the SFX player (frame-table format from the SFX technique) owns ch1. When the
  table ends, `WriteM1` restores AUDC/AUDF/AUDV to the in-progress music note.
- **Overscan via TIM64T** (the real-game pattern, same as the dynamic-multisprite kernel): set
  the timer at overscan start, run input + driver tick, then spin on INTIM. Code-path length no
  longer affects the line count — verified 262 every frame (timer constant 37 calibrated by
  scenario sweep: 36→261, 37→262, 38→263).

## Verified

- **Round-trip**: `dissect -audio 150` transcribes the running ROM back to exactly the composed
  melodies — ch0 `C5:16 E5:16 G5:16 C6:16 A5:16 G5:16 E5:16 G5:24 R:8`, ch1 `C4:32 F4:32 G4:32
  C4:48` (loop-boundary legato merge as expected).
- **Preemption**: at the fire frame, ch1 switches to the laser's AUDC=4 sweep while ch0 keeps
  playing untouched; 12 frames later ch1 is back to AUDC=12/vol 6 and `sfxOn`=0 (all asserted
  numerically in the scenario).

## Integration notes
- The whole driver is ~120 bytes of code + tables; tick worst case ≈ driver + SFX ≈ well under
  the overscan budget (timer absorbs the variance anyway).
- To compose: write the melody in jingle notation, run `cmd/jingle`, copy its `Notes/Durs`
  tables. To verify by ear and by data: Stella for ears, `dissect -audio` for the score.
- More voices/priorities (e.g. SFX queue, ducking instead of preemption) are straightforward
  extensions of `WriteM1` — add when a game needs them.
