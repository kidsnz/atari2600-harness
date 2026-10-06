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
- **The overscan timer at its maximum, as posted: 35 NTSC, from Greg Troutman and Eckhard
  Stolberg; 42 PAL, from Stolberg.** Troutman, 1996, quoting matt's `Top:` / `LDA #$05` /
  `STA Tim64T`: *"There are 30 scan lines in overscan. Thus you need to count 2,280 (30 * 76) on the
  timer. Using the 64interval timer, you get 35 ticks. Not 5. Or something like that."*
  〔stella-list `199610/msg00016`〕. Stolberg, about 7½ hours later (the next day by both posts'
  dates): a loop in matt's code that waits for 32 lines could be replaced by *"a routine that sets
  up the Tim64T timer with #35 (for NTSC; #42 for PAL) and waits for it to expire. This way you
  could do some game logic between the setup and the waiting"* 〔`199610/msg00020`〕. His
  correction, posted the same day about 21½ hours later, moved the advice to the loop commented as
  the overscan loop: *"To get it to it's maximum you need to replace the first LDA #$05 line in
  the code with a LDA #35 line (LDA #42 for PAL), as someone allready mentioned"* (his spellings)
  〔`199610/msg00022`〕; an earlier mention in the archive is Troutman's, above. None of the three
  derives 42; the same calculation for the 36 PAL overscan lines of the vendor's table
  (`fundamentals-audit.md`) gives 36×76/64 = 42.75 → 42 (our arithmetic). This ROM's sweep landed
  on 37, not 35; matt's frame is not traced here, so the two are not compared. Use 35/42 as the
  first points of a sweep like the one above, not as the answer; this file has no PAL sweep.
  **Cited only, not verified.**

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
- **Writing for the chip is easier than porting.** Paul Slocum's music programming guide, 2003:
  *"it's \*much\* easier to write your own music for the 2600 than it is to port a song to the
  2600 and end up with something that sounds good. The pitch is so limited that often you will not
  be able to find notes that are reasonably in tune in the sound types you want, and this will
  lead to compromises. If you write your own music, you can write based on the notes you know that
  the Atari can play in-tune and what sounds best"* 〔stella-list `200301/msg00492`〕. Asked by
  Glenn Saunders how his music avoids out-of-tune notes, Slocum said that for his originals he
  selects sets of notes *"that are all pretty close to being in-tune and limit myself to composing
  with those notes"*, and that when porting he switches *"distortions and/or octaves to get the
  closest pitch possible"* — his driver lets the distortion change on each note — and of the C64
  *Thrust* music, *"Probably my most ambitious port so far"* 〔`200302/msg00182`,
  `200302/msg00189`〕. Harder, then, not ruled out. How far a given figure lands from in tune is
  measured in `music-driver.md` (*How well a tune fits*). **Cited only, not verified.**
- More voices/priorities (e.g. SFX queue, ducking instead of preemption) are straightforward
  extensions of `WriteM1` — add when a game needs them.
- **Two ways to pick the voice per effect** instead of fixing it, both posted in 2003. Manuel
  Polik's first, *"rudimentary"* version of TFXM (The FX Machine), an SFX driver: *"the code
  automatically detects the \*least-busy-channel\*"*, while *"The rest is pretty much random
  driven at the moment"* 〔stella-list `200307/msg00051`〕. Thomas Jentzsch replied the next day
  with his *Thrust* sound code, a `StartSound` subroutine with a cycle count on every instruction
  line: take the channel whose current effect has the lower priority (channel 0 on a tie), and
  start the new effect there only if its priority is not lower than that one's — his comment says
  *"higher"*, and the `bcc` also lets an equal one through (our reading of the code) — with *"By
  definition, my "least-busy channel" is determined by the priority of the FX which is identical
  to the offset into the sound data table"* 〔`200307/msg00052`〕. In this driver effects always
  take ch1. **Cited only, not verified.**
- **A split with no preemption at all** is on record: joe-musashi's pattern player (used in D.K. VCS)
  has an init and a play routine per channel — *"It is important that init and play routines have to
  be called separately for each channel. This makes it possible to use only one channel, e.g., if the
  other one is used for sound effects"*. In the same post, *"A song is made of a list of patterns and
  each pattern is a table of AUDF/AUDC/AUDV values plus duration"*, the timer *"decreased every time
  the play routine gets called (typically once per frame)"* 〔AtariAge `topic/247929`〕 — the order-list
  layer this driver does not have (see `music-driver.md`, *Order list*). Cited only, not verified.
- **A hi-hat played by the code, not stored in the song.** Paul Slocum, 2003: *"This is the first
  full song where I've added the high hat sound in code rather in the music data and it works
  great. It sounds good and frees up the voices for other stuff. Plus I think it saves ROM space
  in the long run"*; that song was 4 minutes long and *"only 1.6K for the driver and music data
  combined"* 〔stella-list `200302/msg00180`〕. His guide of 31 January describes an auto-high-hat
  in his new drivers — *"you don't have to put it in the music data"* — that plays for only 1/60 s
  〔`200301/msg00492`; the 1/60 s part is quoted in `music-driver.md`〕; that this is the same
  feature, and that it still takes a voice for that frame, is our reading. **Cited only, not
  verified.**
