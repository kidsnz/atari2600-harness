# Technique — in-game sound driver (music + SFX priority)

**Goal:** the audio architecture every real game uses: looping 2-voice music from data tables,
with channel 1 **preempted by SFX** and restored when the effect ends — all inside a normal game
frame (driver tick in overscan, timer-managed so the line count never depends on code paths).

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
