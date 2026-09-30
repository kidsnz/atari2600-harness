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
  (ch0 lead / ch1 bass), each with independent state.
- **Silence cell**: `Env[0] = 0` is reserved; a rest points its envelope offset there with
  `sus = 0`, so the gate-off is expressed without a separate flag.

State is 5 zero-page bytes per channel (`dur, idx, eoff, eidx, sus`) = 10 bytes, in the same
ballpark as TIATracker's replayer (~9 permanent). The tick runs in **overscan under TIM64T** so the
line count never depends on the code path (same pattern as `sound-driver.md` / the dynamic kernel).

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

## Integration notes / extensions

- **Release tail**: this minimal version models attack/decay→sustain and decay-to-silence; a
  separate release phase on note-off (TIATracker's release) is a straightforward extension — give
  the gate-off path its own `eidx` walk past the sustain index.
- **Pitch guides** (candidate ⑭): TIA's 5-bit divider gives unevenly spaced pitches; choosing an
  A4 base that maximises in-tune notes per AUDC is a separate, composable layer (`cmd/jingle`
  extension), not part of the replayer.
- **Authoring**: compose externally (Furnace now targets TIA) or extend `cmd/jingle`; the byte
  format here (instrument table + flat `Env` + Notes/Inst/Durs) is intentionally simple to emit.
- For the full canonical envelope byte layout, see the TIATracker manual's "For the coder" section
  and the GitHub player source (recorded in the mining notes).
- **More voices than channels.** The software-mixing route is costed in `capability-gap-audit.md`
  (*Multi-voice software audio*: four voices or a 40-column picture, not both; not scheduled). A
  posted 4 KiB game does it: DigiBeatz (cardboardbox, 2021) *"features software-generated sound to
  split the console's 2 audio channels into 5"*, with 7 tracks of 5 charts each 〔AtariAge
  `topic/323039`〕. The posts give no mixing code; the one hint is the author's, in the same thread: the
  in-game rate is 3.9 kHz, *"once every 4 scanlines"* for `AUDV0`. **Cited only, not verified.**
- **Named note values.** Give pitches and lengths `equ` names (`midD equ $1A`, `crotchet equ 12`,
  `minim equ 24`) and hand-written song data reads as a score, `db midD, crotchet` 〔AtariAge
  `topic/264216`, derek-andrews〕. It changes nothing in the ROM. **Cited only, not verified.**
- **A square wave timed by the CPU** (same thread, just-jeff): toggle `AUDV0` between `$0F` and `$00`
  from a delay loop (with `AUDC0 = 0` that is the volume-as-level output of `tia-pcm.md`). At the NTSC
  CPU clock (1,193,182 Hz, `resources.md`) C5 = 523.25 Hz is 2280 cycles a period, 1140 a half-wave,
  about 142 passes of an 8-cycle loop. It holds the CPU for as long as it sounds, and in the thread the
  tuning was still off after the DASM syntax fix it was given. **Cited only, not verified.**

## Budgets and layouts from other drivers (cited)

This driver states only its RAM (10 bytes, above). Figures from elsewhere, all **Cited only, not
verified**:

- **Cycles and RAM.** Paul Slocum, 2003: *"my music driver generally uses 2-3 bytes of persistent RAM
  plus 3-4 temp RAM locations and I think a max of about 400 cycles (5 lines) per frame. You just
  call the driver once per frame in Vblank or Overscan."* 〔`200301/msg00390`〕 400 cycles is 5.3 lines
  of 76. A year earlier he had cut the driver he fitted into Combat to *"2 bytes plus 3 temp bytes"*
  〔`200202/msg00029`〕; what that gave up is not stated.
- **ROM.** Driver plus song: 700 bytes in the 2003 demo (17% of 4K), *"but it's an older version of
  the driver"*, whose successor *"could probably do the same demo in about 400 bytes"*
  〔`200301/msg00390`〕; 1.5K in the Combat hack, *"but the song's pretty long"* 〔`200202/msg00029`〕.
  Before either, a 1997 plan: a 96-note riff at 2 bytes a note (pitch + distortion) is 192 bytes
  without drums, and the aim was 512 for the whole piece 〔`199704/msg00005`, Nick S Bensema〕.
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
- **Change the frame length**: *"reduce the number of scanlines to (312 / 5 * 4 =) ~250, then the
  beats would be (almost 100%) identical too"* 〔same〕.
- **Space the beats by a pattern** — MLdB's "Dopey fix" for Slocum's song player (2018): for each
  `TEMPODELAY` 1..10 a 6-bit pattern of extra `tempoCount` increments over six beats (bit 7 = one on
  every beat). Counted from the posted table, delay D gets D extra increments (none for D = 1), so six
  beats take 5D frames instead of 6D, the 50:60 ratio, and no beat is dropped — MLdB contrasts it with
  moderntimes99's 2006 PAL revision, used in SpiceWare's Medieval Mayhem, which by his reading skips
  beats 〔AtariAge `topic/280139`〕.
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
