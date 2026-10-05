# Technique — instrument-envelope music driver (TIATracker-derived)

**Goal:** the next step up from `sound-driver.md` — a music engine where **volume is driven by a
per-instrument envelope every frame** (attack/decay → sustain, or decay-to-silence for plucks) and
**each note picks its own instrument**. This is the "volume/gated music driver" that real games and
demos use, distilled clean-room from **TIATracker** (kylearan, forums.atariage.com/topic/250014;
mining notes in `reference/atariage/250014-tiatracker/`).

Demo: `roms/techniques/music_driver.asm`. CI: `scenarios/music_driver.json` (envelope ramps,
sustain holds, per-note instrument switch, pluck decay-to-silence, independent channels, song loop,
262 lines, audio golden).

## Data model (TIATracker's instrument / pattern / song, reduced)

- **Instrument** = `{ AUDC, envelope start offset, sustain index }`. The envelope is a list of
  4-bit volumes (`Env` table). On a note trigger `envIdx = 0`; each frame the driver writes
  `AUDV = Env[eoff + eidx]`, then advances `eidx` until it reaches `sus`, where it **holds**
  (attack/decay → sustain). An instrument whose sustain cell is `0` is a **pluck/percussion**:
  it decays to silence and holds there — a gated note.
- **Pattern** = parallel `Notes / Inst / Durs` arrays. A note is an AUDF value (or `$FF` = rest),
  an instrument id, and a duration in frames.
- **Song** = the pattern played per channel, looping at the end (goto = index wrap). Two channels
  (ch0 lead / ch1 bass), each with independent state. *Lead* and *bass* here name the part a channel
  plays, not its AUDC value: in Slocum's naming, which `pkg/audio.Name` follows, the lead's AUDC 4 is
  "square", the bass's AUDC 12 is "lead", and his "bass" is AUDC 6 〔stella-list `200301/msg00492`〕.
- **Silence cell**: `Env[0] = 0` is reserved; a rest points its envelope offset there with
  `sus = 0`, so the gate-off is expressed without a separate flag.

State is 5 zero-page bytes per channel (`dur, idx, eoff, eidx, sus`) = 10 bytes, in the same
ballpark as TIATracker's replayer (~9 permanent). The tick runs in **overscan under TIM64T** so the
line count never depends on the code path (same pattern as `sound-driver.md` / the dynamic kernel).
Paul Slocum's Synthcart (2002) instead spent one scanline of the picture on the sound registers,
because its measure/beat counter advanced twice a frame, *"to ensure smoothness of the tempo"*; he
added that *"your ear really can't tell the difference"* and that the update could be moved out of the
screen draw 〔stella-list `200202/msg00012`〕. A once-a-frame tick like this one gives up that
half-frame step (our reading). **Cited only, not verified.**

## Verified (scenario, numeric — hardware-calibrated via read_audio)

- **Lead envelope** (instrument 0, AUDC 4): `15 → 12 → 10 → 8` then holds 8 (sustain).
- **Bass envelope** (instrument 1, AUDC 12): `11 → 9 → 7` then holds 7, **advancing independently**
  of ch0.
- **Per-note instrument switch**: ch0 alternates instrument 0 (lead) and instrument 2 (pluck) per
  note; at the second note AUDC stays 4 but the envelope becomes the pluck `15 → 10 → 6 → 3 → 1 → 0`.
- **Gate**: the pluck reaches volume 0 and holds — the lead falls silent while the bass keeps
  sustaining (asserted at the same frame).
- **Pitch sequencing + loop**: C5 → E5 → … and after the 8-note pattern the song wraps back to C5
  (`c0idx` = 0, `c0eoff` = lead offset asserted from RAM).

## Relation to the existing sound driver

`sound-driver.md` writes a **constant** volume per voice and adds SFX preemption. This driver
replaces the constant volume with an **envelope read each frame** and adds **per-note instrument
selection** — the two features that make TIA music sound shaped rather than flat. The overscan
tick, the rest = `$FF` convention, and the looping pattern tables are shared, so the two are
compatible designs; a real game would merge them (envelope volume + SFX channel-1 preemption).

**Which voice gives way.** Merged that way, an effect takes channel 1, the bass. B. Watson, a bass
player planning a game with one voice on bass and the other on melody, asked in 2004 which one a
balloon-pop should take: to him the song would sound more continuous if the melody dropped out for a beat,
but he suspected *"most people would notice more if the guitar (the melody) were to cut out"*, and
once he had code he could *"post 2 binaries and let other people listen & decide"* 〔stella-list `200401/msg00167`,
`200401/msg00174`〕. One reply points him to Manuel Polik's TFXM for sound priorities
〔`200401/msg00170`, Dennis Debro〕; none says which voice. Slocum's guide time-shares a voice inside the music
itself: bass on one voice, melody on the other, and the percussion on the bass voice — *"If you just
play the kick and snare for one step in the sequence then go back to bass, you usually won't notice that
the bass is cutting out for a moment"*; the hi-hat *"often works"* on the up-beat only, his auto-hi-hat
plays for 1/60 s, and *"For most songs, a step in the sequence is 3/60ths to 4/60ths of a second"*
〔stella-list `200301/msg00492`〕. **Cited only, not verified.**

## Integration notes / extensions

- **Release tail**: this minimal version models attack/decay→sustain and decay-to-silence; a
  separate release phase on note-off (TIATracker's release) is a straightforward extension — give
  the gate-off path its own `eidx` walk past the sustain index.
- **Pitch guides** (candidate ⑭): TIA's 5-bit divider gives unevenly spaced pitches; choosing an
  A4 base that maximises in-tune notes per AUDC is a separate, composable layer (`cmd/jingle`
  extension), not part of the replayer.
- **Authoring**: compose externally (Furnace now targets TIA) or extend `cmd/jingle`; the byte
  format here (instrument table + flat `Env` + Notes/Inst/Durs) is intentionally simple to emit.
  Composing aids named in a 2020 AtariAge thread on music tools: TIATracker, reveng's Perceptual
  Tuning Primer, Miditari, and *"RT's online music page with the available notes you can click and hear
  shown on the keyboards for the different wave forms"*; kisrael preferred being shown which notes are
  available to webTune2600's approach of *"put in some notes and we'll hunt for matches"* 〔AtariAge
  `topic/307911`〕. **Cited only, not verified.**
- For the full canonical envelope byte layout, see the TIATracker manual's "For the coder" section
  and the GitHub player source (recorded in the mining notes).
- **Lifting a tune from someone else's ROM.** A 2010–2013 thread on a capture format for 2600 music set
  aside the SID/NSF route (rip the player code and its data, then call it): the 2600 has no standard
  composing tool or player, only scattered ones, and its code is more size-optimised, so detecting a
  player and extracting its data would work for very few games. It favoured a dump of the sound-register
  writes with cycle timestamps instead, because many games write those registers more than once a frame
  〔AtariAge `topic/166162`〕. `cmd/dissect -audio N` takes the dump route without decoding any player,
  but it reads the registers once, at the end of each frame (`transcribe` in `cmd/dissect/main.go`), so
  an earlier write to the same register in that frame is not in what it reports. **Cited only, not verified.**
- **A dropped one-frame hold speeds the song up.** Furnace's experimental 2600 ROM export (dave-c,
  2022): a test song played *"considerably faster"* than in the tracker. The cause was a write to
  `AUDCx` or `AUDVx` that had to be held for one frame, which the exporter's C code did not encode
  correctly and the assembly did not decode correctly, so frames were dropped. After the fix he
  suspected a frame was now being added somewhere, and the thread also named a possible second cause:
  the published branch did not compute the tempo correctly, and every song tried had been slightly off
  〔AtariAge `topic/342168`〕. With lengths counted in frames, as `Durs` is here, every lost frame
  shortens the song (our reading). **Cited only, not verified.**
- **Percussion, and notes kept apart from the instrument.** karri, composing for the same exporter,
  listed *"a few easy ways to make good sounding music for the TIA"*; two of them are *"Use
  percussions"* and writing the melody without the instrument, so that its sections can be reused for a
  different instrument — in his track the melody changes instrument midway on the same notes 〔AtariAge
  `topic/342168`〕. He does not say why percussion helps; our reading is that an unpitched sound cannot
  be out of tune. The parallel `Notes / Inst / Durs` arrays here already keep the instrument in its own
  array, so one `Notes` could be played against another `Inst`; this driver, with one pattern per
  channel, does not do it. **Cited only, not verified.**
- **A tick when a voice is set to 0.** A 2020 question: a small tick or pop each time a note's `AUDV0`
  or `AUDC0` was set to 0, whatever order the three registers were written in; the thread has no reply
  〔AtariAge `topic/307285`〕. A rest here sets `AUDV` to 0 in one write from wherever the envelope
  stands, while the pluck reaches 0 through 1. Whether either clicks, on hardware or in the engine, is
  not measured. **Cited only, not verified.**
- **More voices than channels.** The software-mixing route is costed in `capability-gap-audit.md`
  (*Multi-voice software audio*: four voices or a 40-column picture, not both; not scheduled). A
  posted 4 KiB game does it: DigiBeatz (cardboardbox, 2021) *"features software-generated sound to
  split the console's 2 audio channels into 5"*, with 7 tracks of 5 charts each 〔AtariAge
  `topic/323039`〕. The posts give no mixing code; the one hint is the author's, in the same thread: the
  in-game rate is 3.9 kHz, *"once every 4 scanlines"* for `AUDV0`. **Cited only, not verified.**
- **Three software voices, with the sequencer cut into states** (Andy Mucho, 2003). He summed three
  pulse-width-modulated voices into `AUDV` *"in a style very close to Pitfall2"*: the core *"eats 84
  cycles per samples generated which includes loading it into AUDV and the call overheads"*, one sample
  every two scanlines (*"only a 7.8Khz replay rate"*), with no sequencing inside the visible frame. In
  the overscan the sample still has to come every two lines, so the sequencer is broken into *"lots of
  little states"*, each *"written to not consume more than 40cycles"*: fetching the next pattern and its
  first event takes 7 states, a note-on 5 〔stella-list `200307/msg00172`〕. Two days later his build had
  envelopes and arpeggios and a little hard-coded pulse-width modulation on the lead, and still made no
  proper frame — *"it makes no attempt to at the moment because it makes developing this such a pain"* —
  so he asked for it to be tried on hardware or on his SoundSim, a simple emulator he had written that
  picks up only `WSYNC` and `AUDVx` writes 〔`200308/msg00026`, `200307/msg00172`〕. **Cited only, not
  verified.**
- **Named note values.** Give pitches and lengths `equ` names (`midD equ $1A`, `crotchet equ 12`,
  `minim equ 24`) and hand-written song data reads as a score, `db midD, crotchet` 〔AtariAge
  `topic/264216`, derek-andrews〕. It changes nothing in the ROM. **Cited only, not verified.**
  Musical lengths were asked for in 1997: Piero Cavina wanted Eckhard Stolberg's sound editor to take
  step lengths not as frames but as *"a relative duration (1/16, 1/8, 1/4...)"*; Stolberg answered that
  it was a tool for sound effects and little tunes, and that for a music program he would have used *"a
  Soundmaster 64 or Soundtracker like interface"* 〔stella-list `199707/msg00007`, `199707/msg00012`〕.
  The conversion to frames can itself go wrong: in 2003 Andy Mucho's work-in-progress
  had *"The actual note lengths specified in the music data are correct, but the way they convert them
  into frame times to wait are, erm , borked"*, and he meant to redo the lookup into *"proper 8ths 16ths
  etc"* 〔`200308/msg00026`〕. **Cited only, not verified.**
- **A square wave timed by the CPU** (same thread, just-jeff): toggle `AUDV0` between `$0F` and `$00`
  from a delay loop (with `AUDC0 = 0` that is the volume-as-level output of `tia-pcm.md`). At the NTSC
  CPU clock (1,193,182 Hz, `resources.md`) C5 = 523.25 Hz is 2280 cycles a period, 1140 a half-wave,
  about 142 passes of an 8-cycle loop. It holds the CPU for as long as it sounds, and in the thread the
  tuning was still off after the DASM syntax fix it was given. **Cited only, not verified.**
- **AtariVox oscillators as a voice** (reveng, AtariAge `topic/279099`, 2018): as far as he knew
  nobody had released anything driving them directly. Notes are possible, *"but there's a lot of data
  to feed to setup the oscillators, and with 1 byte a frame, you need to account for the delay in note
  timing"*; his suggestion: *"you could setup the oscillator in advance, and just give it volume as a
  "note on" a few frames early"*. Chords are *"a bit tough"*. For 21 Blue he tuned voice phonemes to
  sing along with the intro tune instead, which he found easier. Where the one byte a frame comes from
  is not stated. **Cited only, not verified.**

## Budgets and layouts from other drivers (cited)

This driver states only its RAM (10 bytes, above). Figures from elsewhere, all **Cited only, not
verified**:

- **Cycles and RAM.** Paul Slocum, 2003: *"my music driver generally uses 2-3 bytes of persistent RAM
  plus 3-4 temp RAM locations and I think a max of about 400 cycles (5 lines) per frame. You just
  call the driver once per frame in Vblank or Overscan."* 〔`200301/msg00390`〕 400 cycles is 5.3 lines
  of 76. A year earlier he had cut the driver he fitted into Combat to *"2 bytes plus 3 temp bytes"*
  〔`200202/msg00029`〕; what that gave up is not stated. A little
  before it (1 February in his time zone, 2 February in UTC), his first pattern-based song player had
  needed *"12 bytes of RAM, but 3 of those can be temp storage"*, and no work mid-screen
  〔`200202/msg00020`〕.
- **Two bytes of RAM, the countdown as the volume.** Manuel Polik's driver from Gunfight (2001), built
  on Kurt Woloch's "ship demo" driver and posted as *"The Sound Machine"*: *"Two bytes RAM needed!"* —
  `soundCount` (the step) and `decayCount`. Each frame `decayCount` drops by 2 from 16, and the count is
  written straight to `AUDV0` (halved with `LSR` for `AUDV1`), so the decay needs no envelope table and a
  step lasts 8 frames (our reading of the posted code). One byte a step carries both channels, a nibble
  each, so *"We're limited to 16 different notes per channel"*; a 0 nibble is skipped by a `BEQ`, so it
  holds the last note, and any other indexes a frequency table and a distortion table, which gives the
  *"(well - almost) silent note"* (`AUDF` 0, `AUDC` 4) for free as one of the 16. For the song,
  Ring of Fire: *"256 notes + 32 channel1 decodings +22 channel2 decodings - 4 dummy bytes = 306 bytes
  ROM!!!"* 〔stella-list `200109/msg00111`〕.
- **ROM.** Driver plus song: 700 bytes in the 2003 demo (17% of 4K), *"but it's an older version of
  the driver"*, whose successor *"could probably do the same demo in about 400 bytes"*
  〔`200301/msg00390`〕; 1.5K in the Combat hack, *"but the song's pretty long"* 〔`200202/msg00029`〕.
  Before either, a 1997 plan: a 96-note riff at 2 bytes a note (pitch + distortion) is 192 bytes
  without drums, and the aim was 512 for the whole piece 〔`199704/msg00005`, Nick S Bensema〕.
  Slocum's first pattern-based song player (2002): *"250 bytes for the song player code and another 250
  bytes for the song data. I guess 500 bytes isn't too bad, but it's a short song"*
  〔`200202/msg00020`〕; about six hours before, the same day in his time zone, he had guessed *"the
  player and a song would cost you around 300-500 bytes"*, one Synthcart beat/pattern being 36 bytes
  〔`200202/msg00012`〕. Heartbreak's title tune had only what was left: in 2013 cybearg, finishing that
  homebrew, had 250–300 bytes for one, and
  utz, offering to write it, called 300 bytes a real challenge 〔AtariAge `topic/208829`〕. At the other
  end, a cartridge that is all music: the demo group Flush's 32K cartridge holds 32 tracks, *"about an
  hour of music"*, made with TIATracker by the musician Glafouk; kylearan, posting it in 2017, called it
  *"impossible"* in the sense that most people would expect a TIA music collection not to be worth
  listening to 〔AtariAge `topic/269094`〕. That averages at most 1 KiB a track, player and display
  included (our arithmetic); the thread gives no per-track size.
- **Order list.** Here a song is one pattern per channel, wrapped. Slocum's 2002 song is a list of
  pattern numbers per voice (`song1`, `song2`, played together) indexing a `patternArray` of
  pointers; `255` ends a list, and a second list (the intro) follows that `255` in the same table.
  Each pattern is 32 note bytes plus 4 accent bytes, the note byte being the 3-bit sound type + 5-bit
  pitch that `pkg/audio`'s `NoteByte` already packs 〔`200202/msg00029`〕.
- **One byte per note, duration included.** A 2009 design thread: 3 RAM bytes a channel (song
  pointer, note pointer, duration countdown) and a note byte `12223333` — 1 bit voice (one of two
  AUDC values), 3 bits duration (an index into an 8-entry length table, so lengths need not be powers
  of two), 4 bits pitch (0 = rest) 〔AtariAge `topic/146399`, TROGDOR〕. Against the 3 bytes a note of
  `Notes/Inst/Durs` it trades the instrument id for one bit and AUDF's 32 values for 15 pitches and a
  rest. A proposal in
  the thread, not a shipped driver.
- **Bits per beat, when the song lives in RAM.** Andrew Schwerin, 1999, planning a composing program for
  his four-voice software engine (`capability-gap-audit.md`) in 4K of ROM and 128 bytes of RAM, left the
  composer 64 bytes. His test piece, the US national anthem, is 33 measures of 6 beats, 198 beats; at 4
  bits a voice and four voices, 16 bits a beat, that is 396 bytes, *"6.1:1 compression"* to fit. For 64
  bytes: uncompressed, 16 bits, 32 beats; 3 bits a voice plus a 2-bit duration, 14 bits, 36 beats; 3
  bits a voice, 12 bits, 42 changes. The goal: *"an average size of 2.5 bits"* a beat. A beat is the
  smallest interval the composer needs, and there are no rests. The 32 pitches (B2 to F#4, his naming,
  middle C = C3) can be given out as 16 overlapping values a voice or 8 each (Bass 0–7 … Soprano 24–31):
  *"I really like the 3 bit system when it comes to saving memory space, but it is much more
  constrictive that the 4 bit system for creative options"* 〔stella-list `199903/msg00055`〕.
- **Relative pitch did not pay.** Chris Wilkson suggested storing each voice as a chromatic step from
  its last note, at 3.5 bits; Andrew Schwerin counted 23 steps (same, up or down 1..11 half steps):
  *"I'll buy that 0-22 is 4.5 bits but not 3.5"*, and *"4.5 x 4:That's 18 bits per beat, which more
  than my "16 bits uncompressed""* 〔`199904/msg00005`, `199904/msg00006`〕 (log2 23 = 4.52). The loss is
  against his 4-bit absolute pitch per voice; against a 5-bit AUDF it would be 4.52 bits to 5, so the
  verdict depends on the baseline.

## PAL: the tempo moves, not only the pitch

`Durs` counts frames, so at 50 Hz every note lasts 6/5 as long (**Not verified** — frame-rate
arithmetic). `BaseClockPAL` in `pkg/audio` models the pitch only, as one uniform shift, which is
Thomas Jentzsch's point about PAL notes: *"In relation to each other, the notes are 100% identical out
of tune. Just the overall pitch is slightly different."* 〔AtariAge `topic/270574`〕 In that thread, on
porting a PAL demo to NTSC, the music rather than the 50/60 Hz timing or the colours was expected to be
the main effort. Fixes on record, all **Cited only, not verified**:

- **Advance the music in fractional frames** — Jentzsch calls it *"(superior!)"*; TIATracker did not
  support it in 2017 as far as he knew, but *"the required code change is just minimal"* 〔`topic/270574`〕.
  Its cost, from his own fractional-math version of Paul Slocum's driver: it *"allows very fine tuning
  of the music speed, but makes it a bit more complicated to have other stuff in sync with music"*
  〔AtariAge `topic/236117`, 2015〕.
- **Change the frame length**: *"reduce the number of scanlines to (312 / 5 * 4 =) ~250, then the
  beats would be (almost 100%) identical too"* 〔same〕.
- **Space the beats by a pattern** — MLdB's "Dopey fix" for Slocum's song player (2018): for each
  `TEMPODELAY` 1..10 a 6-bit pattern of extra `tempoCount` increments over six beats (bit 7 = one on
  every beat). Counted from the posted table, delay D gets D extra increments (none for D = 1), so six
  beats take 5D frames instead of 6D, the 50:60 ratio, and no beat is dropped — MLdB contrasts it with
  moderntimes99's 2006 PAL revision, used in SpiceWare's Medieval Mayhem, which by his reading skips
  beats 〔AtariAge `topic/280139`〕.
- **Switch the playback speed at assembly time** — Medieval Mayhem's copy of Slocum's driver
  (`songplay.h`) is *"slightly modified from the original so the music will play at the correct speed
  for both NTSC and PAL frame rates"*, in an `IF/ELSE/ENDIF` block on `COMPILE_VERSION`; SpiceWare
  credits Erik Ehrling, who made the change *"after hearing the music in the PAL version"*, and adds
  *"Knowing what I know now, I'd have made the PAL version of Medieval Mayhem a PAL60 build"*
  〔AtariAge `topic/236117`, 2015〕. In `topic/280139` (2018) SpiceWare credits Medieval Mayhem's PAL
  revision to moderntimes99, October 2006; neither thread says whether that is Erik Ehrling.
- **Ship both**: the demo's 4K cartridge carries both builds and the Color/B·W switch picks one, *"as
  auto-detection needs a melody board, which is over the top for a 4k ROM"* 〔`topic/270574`, svolli,
  2018〕.

## How well a tune fits is about HOW MANY pitches, not WHICH (2026-09-07)

Manuel Polik had SID-to-TIA conversion working in 2003 — *"No. manual. tweaking."* — and stated its
limit himself: *"Basically it does any SID tune. They will just sound **more or less horrible** ;-)
**Hubbard doesn't do to well**"* 〔`200308/msg00134`〕. The workflow was convert, listen, judge.

`cmd/keyfit` answers it **before** any conversion. Give it the figure as semitones above a tonic
(`-degrees 0,4,7`) and it reports, per tonic, how far each degree lands from where it should. Measured
over three octaves from 55 Hz, best tonic, worst degree in cents
(`internal/keyfit/pitchcount_test.go`):

| distinct pitches | figure | best tonic | worst |
|---|---|---|---|
| 3 | `0,4,7` | F2 | **2.7c** |
| 3 | `0,1,2` | A2 | 4.7c |
| 3 | `0,6,11` | B2 | 4.7c |
| 5 | `0,2,4,7,9` | C#2 | 13.9c |
| 5 | `0,1,6,7,11` | F2 | 13.4c |
| 7 | `0,2,4,5,7,9,11` | C#2 | 13.9c |
| 12 | `0…11` | E3 | **19.3c** |

★**Within a size the intervals barely matter; across sizes they matter a lot.** Three pitches land
within 2.7–4.7 cents whatever they are; five land at 13.4–13.9 whatever they are. **The count of
distinct pitches predicts the fit; the choice of pitches does not.**

★★That is a testable explanation for a subjective remark made twenty-three years ago: **Hubbard's
tunes use more distinct pitches**, so they land worse — nothing about the style, just the size of the
set. And it is answerable from a score.

★★★**For the rule that a cover version may not be out of tune — transpose if you have to — this is
the number that decides.** A three-note figure is free to sit almost anywhere; a chromatic one is 19
cents out at its best tonic, and no key rescues it. Found by the mailing-list distillation (helper-1).

Polik's converter had a second lever: which voices to keep. In September 2003 its mixer gave five ways
to fold the SID's three voices into the TIA's two: *"Unmixed"*, channel to channel — *"It's 100%, but only 2 out
of three channels"* — then three modes that leave one voice alone and mix the other two, and one that mixes
all three. The mixing is *"intelligent"*: a voice silent in a frame leaves the TIA voice to the others,
and when none is silent a priority picks the voice(s) or the split is *"even"*. With it, 30 seconds of a
Rob Hubbard song played *"in almost acceptable quality"* 〔stella-list `200309/msg00005`〕. **Cited
only, not verified.**
