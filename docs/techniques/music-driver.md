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
