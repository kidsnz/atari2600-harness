# Technique — free audio envelopes from gameplay counters (Combat idioms)

**Source:** studied clean-room from the annotated *Combat* disassembly (Roger Williams' `Combat.asm`), deep-read harvest 2026-07-23 〔Combat.asm sound routines: MOTORS / BoomSnd / SNDP〕. These are **reference idioms distilled from the original ROM** (label names + generalized prose only) — *not yet reimplemented / CI-locked* in this harness; standalone demo + scenario = TODO. Complements the three verified audio docs: `sound-effects.md` (frame-table SFX), `sound-driver.md` (music + SFX priority), `music-driver.md` (per-note instrument envelopes). Those are about **driver architecture**; this doc is about **reusing gameplay state as audio state** to get envelopes for zero extra bytes.

> Companion mental model — *sound-channel arbitration is last-writer-wins on a 1-object-per-channel bus* (no mixer; precedence is branch order): see `design-principles.md` "Combat deep-read".

## 1. The counter IS the audio register (free envelope)
Alias a monotonic gameplay counter you already keep **straight into AUDF/AUDV**; the sound's envelope falls out for free, with zero dedicated audio state.
- **Rising-pitch rally.** A wall-bounce counter (`BounceCount`, init `$1F`, `DEC BounceCount,X` per bounce) is written straight to pitch: `LDA BounceCount,X / STA AUDF0,X`. Lower AUDF = higher pitch, so every successive bounce lifts the tone — the ascending "pong-pong-pong" of a long rally, with **no separate sound counter**. Timbre fixed (AUDC `$04` pure, AUDV `$07`). The same byte simultaneously serves as a gameplay gate (the billiard "must-bounce-first" flag) 〔BounceCount, COLMPF, MOTORS pong branch〕.
- **Decay-to-silence explosion.** A physics countdown (`StirTimer`, the hit-spin timer, dec by 2/frame) becomes a linearly decaying volume via `LDA StirTimer / LSR / LSR / LSR / STA AUDV0` (÷8 to fit the 4-bit register). A `BPL` gate **holds full volume until the timer crosses halfway**, then lets the ramp begin → held-then-linear-fade (delayed-onset decay) with no envelope machinery. Pitch fixed (AUDC `$08` noise) 〔BoomSnd LSR×3 envelope〕.
- **Idiom.** Bind pitch/volume to a counter the gameplay already maintains and the "tension rises with the rally / boom fades over time" arc is automatic. **`LSR`×n rescales** the counter into the register's 4/5-bit range; a **sign-bit `BPL` gate** buys a delay before the ramp.

## 2. Give a triggered SFX its own short self-clearing counter
Decouple a sound's **audible length** from the physics event that spawned it: a tiny up-counter that self-resets, so the tone plays a fixed few frames no matter how long the underlying event lingers.
- New trigger does `INC AltSnd`; thereafter `CMP #$04 / INC AltSnd / BCC … else STA #$00` counts `0→$04` then clears. While nonzero it also **steals the channel** (suppresses the engine tone). So the bounce SFX is a fixed ~4 frames even though the missile can stay physically stuck in a wall for many frames (the collision solver `MxPFcount` lingers) 〔AltSnd, COLTNK, COLMPF INC〕.
- **Idiom.** Author SFX duration in a dedicated short counter, **not** off the event's own lifetime — otherwise a long-lived physics state drones the sound. The same counter can gate channel ownership while it runs.

## 3. Per-player detune for source separation on a mono console
Two objects that share timbre are made distinguishable by a **constant pitch offset keyed to player index** — pseudo-stereo separation on a 1-speaker machine.
- Engine: `AUDF = (X<<1) + curve[…]` via `TXA / ASL / ADC SNDP,Y` — player 1's engine is always **+2 AUDF** above player 0's at identical speed/craft. Explosion: a 2-entry per-player pitch table (`AudPitch = $0F,$11`) splits the two booms by 2 steps 〔MOTORS TXA/ASL/ADC SNDP,X; AudPitch $0F/$11〕.
- **Idiom.** Add a small index-keyed AUDF bias so the player can tell "my sound" from "their sound."

## 4. Speed→pitch via a 2D (craft × velocity) curve with gear-shift plateaus
Map a continuous state (speed) to pitch through an **indexed curve** rather than arithmetic, so the "feel" is hand-tunable per object — and **repeat table entries on purpose** to get stepped tone plateaus.
- Engine AUDF comes from a 3×12 table `SNDP` (3 craft rows × 12 velocity columns): add a per-craft stride (`#$0C` per craft step) to land on the row, then `ADC velocity` to pick the column. The curves are non-linear and quantized — e.g. a fast-craft row `$00,$00,$00,$00,$00,$12,$10,$10,$0C,$0C,$07,$07`: silent-ish at rest, then AUDF steps **down** (pitch up) in repeated pairs → discrete **"gear-shift" tone steps** as it accelerates, not a smooth glide. Base timbre/volume also differ per craft (`SNDC` / `SNDV`) 〔MOTORS MOPIT0 loop; SNDP/SNDC/SNDV tables〕.
- **Idiom.** An indexed speed→pitch curve makes acceleration feel **authorable per object**; adjacent repeated entries buy stepped plateaus for free.

## Verify (when reimplemented)
All four are **temporal** invariants the harness can already trace with `read_audio_trace`: AUDF descends monotonically across a rally, AUDV decays monotonically after a boom trigger, engine AUDF is a pure function of (craft, velocity, player) with the +2 detune, and the two channels never carry the same sound at once (single-channel arbitration). A dedicated `assert_audio_envelope`-style helper is filed as **CMB-4 (CMB-AUDIO)** in `capability-gap-audit.md`.
