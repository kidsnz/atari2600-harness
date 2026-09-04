# Fundamentals audit — what we know, what we assume, what we don't (2026-06)

A systematic audit of Atari 2600 fundamentals **before** absorbing more techniques. Method: six parallel
research passes over (a) the primary specs and tutorial corpus held locally in `reference/` (Stella
Programmer's Guide, woodgrain wiki, Andrew Davie's *Newbies*, SpiceWare's *Collect/Let's Make a Game*,
8bitworkshop samples, real-game disassemblies, DaveC's samples), (b) ~22 owner-supplied links (AtariAge
threads, 6502.org, Slocum's music guide, Stella debugger docs, Pitfall analyses, nanochess/za2600 repos),
and (c) independent web research (Andrew Towers' *TIA Hardware Notes*, Stolberg's frequency guide). Every
constant-level claim was cross-checked against ≥2 sources or flagged.

**Legend** — ✅ **verified**: measured by our litmus ROMs, locked in CI. 📖 **documented**: stated by a
primary spec or ≥2 independent sources, *not yet measured by us*. ⬜ **unknown**: no authoritative source
found, or sources conflict — measure it ourselves. ⚠️ **caution**: a trap, contradiction, or correction.

The actionable follow-ups (now delivered — see `CHANGELOG.md`) and any remaining gaps live in the single live
backlog `capability-gap-audit.md`. Verified facts remain cataloged in `verified-coverage.md`.

---

## 1. Frame & timing
- ✅ NTSC 262 (3/37/192/30), PAL 312; 1 line = 228 clocks = 76 CPU cycles; cycle-counting invariant.
- 📖 **VSYNC procedure: set D1, wait ≥2 lines, clear** (Stella PG ~§3). **Split by measurability
  2026-09-03** — the procedure and the threshold are different claims and only one of them is ours
  to measure. The *shape* (set D1, hold, clear, and the frame is accepted) is measurable here. The
  **≥2 is not**: `television.go` accepts a frame when
  `vsync.activeScanlineCount >= env.Prefs.TV.VSYNCscanlines`, and that preference defaults to **2**
  (`preferences/television.go` `SetDefaults`), with nothing in this repo setting it. A litmus that
  sweeps 0..4 lines and reports "1 fails, 2 passes" would be reading back the number we supplied.
  See `known-traps.md` section E; it is the sharper twin of the SuperChip SARA entry, because there
  the default disables a feature and the green looks odd, while here the default **agrees with the
  literature** and the green looks like corroboration. What can be measured is that a step exists
  and that there is only one, with a Go-side control moving `VSYNCscanlines` to 3 so the test says
  out loud that the threshold is an input.
- ✅ **RIOT timers TIM1T/8T/64T/1024T ($294–7)** — verified `litmus_timer` (v0.47.0),
  regression-locked `roms/litmus/scenarios/timer.json`, table row `docs/verified-coverage.md:26`.
  Write 1–255; the counter decrements 1/cycle; **after underflow it continues from $FF, still
  1/cycle** ($94=$EF then $96=$E1). TIMINT $285 D7 = expired ($93=$C0, D7+D6 set).
  **Reading INTIM clears TIMINT** — $95=$00 after the INTIM read at $94; the scenario pins
  `ram.0x93 == 192` and `ram.0x95 == 0`, so the clear is a regression, not an observation.
  📖 Still documented-only (Stella PG PIA §2.3): **"INTIM holds 0 for one interval before the
  $FF wrap"** — the ROM steps straight through expiry and never samples the 0 interval.
  ⬜ **Exact first-decrement offset.** The countdown band pins three successive reads at
  $3C/$35/$2E from TIM1T=$40 — that fixes the rate (−7 per `lda abs`+`sta zp` iteration) and the
  value 4 ticks after the write **for that instruction sequence**; it does not isolate how many
  cycles after the store the first decrement lands.
- ⬜ SECAM; real-game variable line counts (we already treat 262 as a range).

## 2. Horizontal positioning & HMOVE
- ✅ X(N)=3N−55 (missile/ball), player +1px; slope 3 px/cycle; divide-by-15 coarse; **no leftmost-X constant** (retracted 2026-07-30; it is kernel-specific) /
  missile 2; all 16 HMOVE nibbles (+7..−8, positive = left) right after WSYNC.
- ⚠️ `reference/docs_atari/cycle_counting_guide.html` uses `X=(CYCLES−20)*3` and "round to 15" — both are
  tutorial approximations. **Never cite it for positioning**; our calibrated formula is more precise.
- ✅ **Do not write HMxx within 24 CPU cycles after HMOVE** — measured `litmus_hmxx_freeze` (v1.53.0):
  on Gopher2600, HMxx is **latched at the HMOVE strobe** — rewrites at +6/+15/+33 cy never alter the
  in-flight movement (right-8 stayed +8/frame in all three windows). Keep the 24-cycle rule as a
  REAL-HARDWARE portability constraint ("unpredictable" on silicon, Stella PG 5×), but our oracle is
  deterministic and write-inert; the rule costs nothing to follow (HMCLR after SLEEP 24, as score6 does).
- ✅ **HMOVE mechanism** (Towers, *TIA Hardware Notes*) — measured 2026-09-03; this line was
  documented-only until then, and `roms/litmus/litmus_hmove_side.asm` had recorded the numbers in its
  header since V2-2 while **nothing graded them** (the ROM was carried only as ceiling corpus).
  HMOVE struck right after WSYNC **extends HBLANK by 8 colour clocks** → the left-side comb, painted
  with **every HMxx at zero** (16 of band A's 32 lines, strictly alternating). HMOVE struck
  **mid-visible displaces nothing and paints no comb** (0 of 32 lines; P0 holds clock 9 across all 64
  lines of bands A–C). HMOVE struck **at the end of the line adds 8 to the nibble**: HMP0 = $10 asks
  for one clock left and delivers **nine** — measured over 14 uniform strobes, 151 → 34. The loop-exit
  strobe moves **−8** instead, because `bne` falls through there and the strobe lands one cycle
  earlier; that is recorded rather than dropped. `→ internal/emu/hmoveside_test.go` (4 gradings,
  2 negative controls: removing the +8 fails on every step; claiming the comb in the mid-visible
  bands fails by name)
- ✅ **RESPx / RESMx / RESBL reset phase** — measured 2026-09-03; this line was documented-only until then. The player's first visible pixel lands **+5 colour clocks** past the strobe's own end clock (the value Towers' *TIA Hardware Notes* states), and the **missile and the ball land +4** — a one-clock difference between an 8-clock object and a 1-clock object that the document does not carry. One extra CPU cycle before the strobe moves any of them exactly **+3** clocks. Offsets are read against the strobe instruction's own beam position (`TraceClocks`, visible coordinates), so nothing is derived from cycle arithmetic. 〔Towers, *TIA Hardware Notes*, RESPx pipeline〕 `→ roms/litmus/litmus_respx_phase.asm` / `internal/emu/respxphase_test.go` (3 gradings, 2 negative controls: the player's offset forced to 4 fails by name; a flat sweep trips the slope control)
  point (explains our verified +5 family offsets). **RESBL re-emits START (ball restartable mid-line);
  RESPx does not** (player needs a 160-clock wrap). ⬜ double-strobe behavior unmeasured.
- ✅ **missile-locked-to-player (RESMP D1)** — the ⬜ was stale: `roms/litmus/litmus_resmp.asm` +
  `scenarios/resmp.json` already lock the offset at **+4** (player0.hmoved_pixel 24, missile0 28) and
  confirm it follows an HMOVE'd player. What that fixture could not answer is the word **"centered"**,
  which is a claim about width. Measured 2026-09-03 across three widths: **+4 at NUSIZ 1x** (an 8-clock
  player, so that IS the centre), **+6 at 2x** (16 clocks; the centre would be +8) and **+10 at 4x**
  (32 clocks; the centre would be +16). Centred holds at 1x only. The snap fires when the player's scan
  counter reaches a particular pixel — 2 at 1x, 4 at 2x, 5 at 4x
  (`Gopher2600 hardware/tia/video/player.go:776`) — so it tracks a pixel index, not a width. The lock
  must be **held for a full scanline**; locking and releasing inside one line never snaps.
  `roms/techniques/bullets.asm:3` states +4 and is right for the 1x it uses.
  〔Stella PG for the mechanism; the width dependence is ours〕
  `→ roms/litmus/litmus_resmp_width.asm` / `internal/emu/resmpwidth_test.go` (4 gradings,
  3 negative controls: calling 2x the centre fails by name; a lock released inside one line fails three
  of the four; a missile that does not track the sweep fails)

## 3. Sprites (players)
- ✅ GRP bit order (D7 left), row order, NUSIZ double/quad/3-copies, REFP, P0+P1 16px combine.
- ✅ **VDEL exact semantics** (Stella PG §6.D — the load-bearing mechanism) — measured 2026-09-03;
  this line was documented-only until then. Each GRP has new+old copies. **Writing GRP0 copies P1's
  new→old; writing GRP1 copies P0's new→old, and also ENABL's new→old.** VDELPx/VDELBL D0=1 selects the
  *old* copy for display. All three confirmed: with VDELBL set and ENABL's new copy on, the ball stays
  **dark** after a GRP0 write and **lights** after a GRP1 write — two bands one instruction apart. Both
  players show their old byte, each latched by the **other** register.
  〔Stella PG §6.D; engine `hardware/tia/video/video.go:234-238`〕
  `→ roms/litmus/litmus_vdel_cross.asm` / `internal/emu/vdelcross_test.go` (3 gradings, 2 negative
  controls). The fixture latches every old copy to zero on entry: without that, the ball's old copy
  survives from an earlier frame and band B passes on stale state — measured, and the reason the
  first negative control did not fire.
- ✅ **Missiles have no vertical delay** — measured 2026-09-03; this line was documented-only until
  then. Read against the ball, which does have one: with every VDEL bit set and both objects enabled on
  the same line and no GRP write after, the **missile lights and the ball stays dark**. Two controls make
  that readable — with VDELBL clear both light (so the fixture does enable both), and with VDELBL set
  plus a GRP1 write both light (so the ball was waiting on a latch, not broken). There is no VDELM
  register and no new/old pair for a missile, which is why in a 2LK it starts on the line it is enabled.
  〔Stella PG; pairs with `litmus_vdel_cross`〕 `→ roms/litmus/litmus_missile_novdel.asm` /
  `internal/emu/missilenovdel_test.go` (3 gradings, 2 negative controls)
- ✅ Moveable-object writes are shear-safe at CPU cycles 0–22 of the line — closed by derivation from
  verified constants (any write completing by cy 22 precedes every draw start: (X+68)/3 ≥ 22.67 even at
  X=0) plus litmus_48px6's measured mid-line GRP choreography (writes landing in copy gaps).
- ⬜ 48px kernel GRP write windows: **no local source documents the cycle map** — derive ourselves (the
  recipe exists in score6.asm: NUSIZ=3-close, RESP0/RESP1 3 cycles apart at ~cycle 26+, HMP1=$10, VDELP both
  on, 6-store choreography, font `align $100`).

## 4. Playfield
- ✅ PF0/PF1/PF2 bit order; CTRLPF D0 repeat/reflect; per-scanline colors.
- 📖 **Asymmetric-PF rewrite windows — definitive tables exist** (woodgrain `Playfield_Timing.html`,
  derived from AtariAge thread 149228). Conservative windows (CPU cycle after the store completes,
  WSYNC=0; `*`=previous line): repeated mode — LPF0 53\*–21, LPF1 64\*–27, LPF2 75\*–37, RPF0 27–48,
  RPF1 37–53, RPF2 48–64. Reflected mode — **RPF2 must complete exactly at cycle 48**. Mid-register late
  writes split *per pixel* (old bits left, new bits right) — well-defined, great litmus predicate.
- ⚠️ Internal discrepancy found: SpiceWare Step 3 says the left-PF1 window opens at cycle ~66 of the prior
  line; Step 7 annotates ~71. Resolve by measurement; trust the harness.
- ✅ **CTRLPF D1 SCORE / D2 PFP priority / D4–5 ball width** — verified `litmus_ctrlpf` (v1.53.0),
  table row `docs/verified-coverage.md:66`.
  SCORE (D1): left half→COLUP0, right→COLUP1, split at clock 80. Priority (D2): with D2 clear P0
  draws over PF; with D2 set PF draws over P0. **Ball width D4–5 = 00/01/10/11 → 1/2/4/8 px**,
  read back per band (`read_row` rows 106/114/122/128).
  ✅ **SCORE×PFP interaction measured** (v1.53.0): **PFP dominates** — with D2 set, D1 has no
  effect (PF renders in COLUPF on BOTH halves, with priority over players); $02→halves colored,
  $04 and $06→identical COLUPF rendering.
  ⬜ **WHEN a ball-width write takes effect is not measured — and a 1997 source disagrees with our
  engine.** `litmus_ctrlpf` fixes the four widths (`rg -i "delay|latch|mid.?line"` over it returns
  nothing), so we have the values and not the timing. Gopher2600 applies the write immediately
  (`hardware/tia/video/ball.go`: `bs.Size = (value & 0x30) >> 4`). A stella post from 1997 reports
  the opposite — a width write not taking effect **for about eighty colour clocks**, and a width
  changed part-way through a draw rendering as `X......X` rather than as either width. Which is
  right is open: neither has been measured here, and the 1997 report was made against real
  hardware while ours is a reading of the emulator's source.
  Note also that `design-principles.md`'s register-timing rules cover PF writes (2-3 colour clocks
  late) and colour writes (immediate) and say nothing about CTRLPF, so a reader has no reason to
  suspect a difference. To settle it: change the width mid-visible and read back the first x where
  the drawn run changes, with the same change made during HBLANK as the control (2026-09-03).
- ✅ **Asymmetric PF under reflection: writing PF0 twice in one line does show different values
  at the two edges** — verified `litmus_pf0_reflect`, regression-locked
  `roms/litmus/scenarios/pf0_reflect.json`, graded by `internal/emu/pf0reflect_test.go`.
  Under reflection PF0 draws at cols 0-3 and again at 36-39, and a second write between them
  changes the right edge alone. **The window is bounded on the right by the line itself**: the
  right copy is drawn at cy ~70.7-75.7 and the line ends at 76, so no store lands after it —
  the last usable point lands *inside* the copy and splits it old|new, which is where the
  measured step is (`0 0 0 0 0 0 1` over seven five-cycle steps). Two negative controls, both
  fired: widening the probe makes it read both copies (the TIA counter wraps at 160), and
  removing its HMOVE fine-adjust puts it past the split and the step disappears.
  📖 **Still documented-only: that real games do it** (DaveC's Random-Dungeon `_room_loop`) —
  that is a fact about someone else's source, not about the hardware, and the line had the two
  claims under one mark (2026-09-03).

## 5. Collisions
- ✅ 3 of 15 pairs (BL-PF, P0-P1, M0-P0), sticky latches, CXCLR.
- ⬜ remaining 12 pairs.
- 📖 read idiom: one `BIT CXxx` yields two pairs via N and V. The flag semantics are settled in
  the engine's source, not by a litmus: `Gopher2600/hardware/cpu/cpu.go:1262 "case instructions.BIT"`
  loads M, sets `Sign` and `Overflow` from it, and only then ANDs `A` — so N and V are M's bit7/bit6
  (`registers/data.go:73 "IsNegative"` masks `0x80`, `:83 "IsBitV"` masks `0x40`) and do not depend
  on `A` at all. Z is the only flag `A` reaches. That is engine source rather than a measurement, so
  the mark stays 📖 until a litmus sweeps `A` and shows N and V do not move: a single value of `A`
  cannot tell "independent of A" apart from "happened to agree".
- ⚠️ **Read it with `A = $C0`, not `$FF`.** Only D7/D6 of a collision register are driven; the rest
  of the byte is the last value the CPU put on the bus (`Gopher2600/hardware/memory/memory.go:189
  "data |= mem.LastCPUData & ^mem.DataBusDriven"`), which is why
  `roms/litmus/scenarios/litmus_cxclr.json` asserts 130 and 2 rather than 128 and 0. With `A = $FF`,
  Z answers "is the whole byte zero" and so moves when the preceding instruction changes, with no
  change in TIA behaviour at all. With `A = $C0` the residue is masked and Z becomes a third useful
  predicate — "neither of these two pairs collided" — so one `BIT` yields three tests, not two.
  With `A = $00`, Z is always 1 and carries nothing.
- ✅ **flicker collision attribution** (za2600 `EN_LAST_DRAWN`) — verified `litmus_flicker_attrib`,
  regression-locked `roms/litmus/scenarios/flicker_attrib.json`, graded by
  `internal/emu/flickerattrib_test.go`. With **CXCLR strobed every frame**, the latch read in a
  frame belongs to the object drawn in **that** frame: over eight alternating frames the latch
  column and the ROM's own record of what it drew agree cell for cell, and inverting the phase
  inverts the latches, so frame parity is not the cause. **Without CXCLR the attribution is lost** —
  the same eight frames all read set. The line said "a verifiable pattern *once we do flicker*"
  while `flicker_multiplex` had existed since technique #10 and touched no collision register at
  all: **the condition had been met and the sentence had not noticed** (2026-09-03).
  ⬜ The control above needs a latch to survive a frame boundary, which nothing measured —
  `litmus_cxclr` takes all three of its snapshots inside one frame and strobes CXCLR every frame,
  so a latch never gets the chance there. This ROM measures it first, in its own group 1, so the
  control rests on our measurement. What stays open is whether **real hardware** holds a latch
  across a frame boundary; this is Gopher2600's behaviour.

## 6. Audio
- ✅ AUDC/AUDF/AUDV register readback; audio digest golden.
- 📖 **Complete AUDC table consolidated** (Slocum guide v1.02 — held locally, authoritative; Stolberg's
  frequency/waveform guide; Stella PG): usable voices — Square(4), Bass(6), Pitfall(7), Noise(8),
  Buzz(15), Lead(12), Saw(1), Engine(3). Pitch: `f = base/(AUDF+1)/D`, base ≈ 31,399.5 Hz NTSC
  (clock/114, 2 samples/line), CPU-clock modes (12–15) ÷3; D = 2/31/31/511/93/6/15/465. PAL ≈13 cents
  flatter. Slocum's three tuning setups (which (AUDC,AUDF) pairs are in tune) are transcription-ready
  for `pkg/audio`.
  ✅ **The "duplicates" are two different things — measured** (`docs/verified-coverage.md:108`): the
  sources list {0,11} {4,5} {6,10} {7,9} {12,13} as one set of duplicates. That is right about
  **tuning** and wrong about **samples**: only {0,11} {4,5} {12,13} are sample-identical, while
  **{6,10} and {7,9} are inverted twins** — same period and tuning, complementary hi/lo duty, so
  identical to the ear but a different sample sequence (`pkg/audio/audio.go:48-50`; the assertion is
  the duty-sum check in `internal/emu/emu_audiocap_test.go:134`). Consequence: any sample-level
  comparison — a golden audio digest, a waveform diff — reports 6 vs 10 and 7 vs 9 as **different**,
  and that difference is correct, not a defect. `audio.Canonical` folds all five pairs for
  classification; it does not make the samples equal.
  ✅ **Pitch formula measured** (`docs/verified-coverage.md:109`): `base/(AUDF+1)/D` confirmed by
  raw-sample capture (square 30/62, lead 90, bass 310).
- 📖 SFX recipes (Slocum): kick=Buzz@30, hi-hat=Noise@0 for 1 frame, snare=Noise@~8; arpeggio/echo/
  portamento patterns. Driver economics: ~400–500 cycles/frame, 600–2000 bytes ROM (Sequencer Kit).
- ✅ **The pitch table is measured against the machine at 330 of its 512 (AUDC,AUDF) points**
  (`TestEveryPitchTheHardwareHasMatchesTheFormula`, 2026-08-11). Two-sided: an exact sample-for-sample
  repeat at `(AUDF+1)×D` (which alone cannot fail a formula returning a multiple) plus autocorrelation
  finding nothing shorter (which alone is a similarity, not an equality). Skipped and counted: 96 pitchless
  (DC, noise), 86 too long to hold 8 cycles in 30 frames. Negative control: divisor 31→30 fails 128/330.
- ✅ **All nine pitched waveforms characterised, and AUDF proved a pure time scaling**
  (`TestAUDFScalesTheWaveformAndNeverChangesIt`, 2026-08-11). Run lengths within one cycle,
  normalised by `(AUDF+1)` — identical at every AUDF up to rotation, exact integer equality,
  pinned as a golden. Shapes (summing to the divisor by construction):

  | AUDC | name | D | runs | shape |
  |---|---|---|---|---|
  | 4 | square | 2 | 2 | `1 1` — a true 50% square |
  | 12 | lead | 6 | 2 | `3 3` — a true 50% square |
  | 6 | bass | 31 | 2 | `13 18` — an **asymmetric** 41.9% pulse |
  | 14 | low bass | 93 | 2 | `49 44` — an **asymmetric** 52.7% pulse |
  | 1 | saw | 15 | 8 | `4 3 1 2 2 1 1 1` |
  | 2 | rumble | 465 | 16 | `62 44 18 31 31 13 18 13 62 49 13 31 31 18 13 18` |
  | 7 | pitfall | 31 | 16 | `2 1 3 1 1 1 1 4 1 2 1 1 2 2 5 3` |
  | 15 | buzz | 93 | 16 | `5 6 4 5 10 5 3 7 4 10 6 3 6 4 9 6` |
  | 3 | engine | 465 | 128 | (in the test log) |

  Consequence for choosing an instrument: only two of the nine are symmetric squares, and
  the asymmetry of 6 and 14 is why they have their own character rather than being a
  quieter square.
- ⚠️ **`audio.MeasurePeriod` is square-like only, and fails silently.** Mean transition interval × 2 is the
  period only with two transitions per cycle — AUDC 4 and 12, nothing else. On the poly waveforms it returns
  a clean fraction — exactly (runs per cycle)/2, i.e. 4× for saw, 8× for rumble/pitfall/buzz, 64× for engine — that looks like an ordinary number.
  Use `audio.MeasureFundamental`. The four spot checks that stood as this table's verification were three
  squares plus AUDC 6, the one poly waveform whose transition count coincides with its period — by luck
  exactly the cases the broken measure could handle, which is why they stayed green.
- ⚠️ **The audio digest cannot verify pitch** (it's a hash, not a measurement); the sweep above is what does.
- ⚠️ slocum-tracker's default export has a comment/data mismatch (Engine slot emits 14 not 3) — check
  `soundTypeArray` on imported songs.

## 7. Input
- ✅ SWCHA joystick bits (P0 high nibble R/L/D/U, 0=pushed) — verified `litmus_input` (v0.42.0).
- ✅ **SWCHB console switches** — verified `litmus_swchb` (v1.46.0): D0 RESET / D1 SELECT
  (active-low), D3 color/BW, D6/D7 P0/P1 difficulty. Driven via `SetPanel`
  (reset/select/color/p0pro/p1pro) + scenario panel inputs.
- ✅ INPT4/5 fire: D7, 0=pressed; **VBLANK D6=1 latch mode** — verified `litmus_input` (v0.42.0).
  Test with N flag, never Z (bus noise in low bits).
- ✅ Paddles INPT0–3 dump/charge — verified `litmus_paddle` (v0.54.0; transfer curve measured).
- 📖 **SWACNT/SWBCNT DDRs** — documented only. **"Rarely game-relevant" was withdrawn 2026-09-03:**
  it was true of *our* ROMs and not of the games. `docs/casebook.md:98` reads a commercial title
  as **"gate joysticks through the port DDR"**, so the counterexample was already in this
  repository, one file away. What is true is the narrower statement: **no ROM here writes
  SWACNT or SWBCNT at all** (`rg -l "SWACNT|SWBCNT" roms --glob "*.asm"` → 0), and none needs to,
  because the engine resets the RIOT chip memory to zero (`hardware/memory/vcs/riot.go` `Reset`),
  which is all-inputs, and `deriveSWCHA` then returns the peripheral value unchanged. So a DDR
  litmus cannot confirm existing practice — it has to **drive a port as an output**, which no ROM
  here does.
  **The list went further and we could not follow it, 2026-09-04.** A 2004 post reads the Stella
  Programmer's Guide saying SWCHB *"is hardwired to be input only"* and asks the obvious question —
  *"then why would they have the SWBCNT register?"* — then reports **Air-Sea Battle setting D4 of
  SWCHB as output**, and **Combat doing the same with its comments mislabelled**: *"it says this
  stops the response from the joysticks but it doesn't"*. If that holds, "rarely game-relevant" was
  wrong about two of the best-known cartridges there are.
  ✅ **Combat verified from its own bytes, 2026-09-04.** `sandbox/studies/combat/Combat.bin`, first
  eighteen bytes: `78 D8 A2 FF 9A A2 5D 20 BD F5 A9 10 8D 83 02 …` — `SEI / CLD / LDX #$FF / TXS /
  LDX #$5D / JSR $F5BD /` **`LDA #$10 / STA $0283`**. One write to SWBCNT, at file+`$000C`, of
  **`$10` — D4 alone**, which is the same bit the 2004 post reports Air-Sea Battle setting. Zero
  writes to SWACNT (`8D 81 02`) anywhere in the 4K image. Decoded from the bytes here; no annotated
  disassembly was opened, which keeps this inside the clean-room line — a ROM image carries no
  interpretation, and reading meaning out of it is the skill.
  📖 **What the DDR write is FOR is still open.** The 2004 thread's question — *"does anyone know if
  SWBCNT has any use?"* — went unanswered, and the same post reports Combat's own comment claiming
  the write stops joystick response **when it does not**. So we know two commercial titles do it and
  we do not know why, which is a sharper open question than the one this line started with.
  **A search note:** this was recorded as unverifiable an hour earlier because `reference/` holds no
  Combat ROM. It is in `sandbox/`, a sibling repository — **the search was scoped to one of the four
  and the conclusion was stated as if it covered all of them.**
  ⬜ **The power-up value is the engine's choice, not a measurement.** `Reset` zeroes the memory
  explicitly; whether a real 6532 clears its DDR on RES is not established here. Measure **what
  writing SWACNT does**, which is the truth table (`riot/ports/ports.go`, 8 rows, all four
  SWCHA_W×SWACNT combinations present), not the default.

## 8. 6502/6507 precision
- ✅ cycle accounting (76/line; WSYNC-stall exclusion).
- 📖 Page-cross +1 applies to **reads** (abs,X / abs,Y / (ind),Y); **stores are fixed** (STA abs,X
  always 5, (ind),Y always 6); RMW abs,X fixed 7. Branches: 2, +1 taken, +1 page-cross **measured
  from the next instruction's address**. (6502.org.)
  ✅ **Measured for the cases the litmus covers** (`docs/verified-coverage.md:90-91`, `litmus_6502`
  v0.44.0, regression-locked `roms/litmus/scenarios/cpu6502.json`): **LDA abs,X** 4cy → 5cy on a
  page cross (+1); **STA abs,X** 5cy on both sides (**stores really are fixed** — the basis for
  kernel determinism); **BNE** 2 / 3 / 4 (not taken / taken / taken+page-cross); **DCP zp** 5cy
  (which also proves illegal-opcode support). The ROM measures each with a TIM1T=$80 window.
  ✅ **The remaining three, settled from the engine's own instruction table** (2026-09-04,
  `Gopher2600/hardware/cpu/instructions/definitions.json`, 256 entries, grouped by
  `addressingMode` AND `bytes`). This is the table the CPU executes from, so it is a reading of the
  machine we run, not a second opinion about hardware — the litmus stays the authority on what the
  silicon does, and these three agree with it:
  **`(ind),Y`** 16 entries — read 8, **all page-sensitive**, base 5 (→6 on a cross); write 2, fixed
  6; **modify 6, fixed 8**. So "always 6" is true of `STA`/`SHA` and **not** of the illegal RMW
  forms (`slo`/`rla`/`sre`/`rra`/`dcp`/`isc`), which are a flat 8.
  **RMW `abs,X`** 12 entries, `cycles` exactly `[7]`, page-sensitive on none — **fixed 7 confirmed**.
  **Reads through `abs,Y`** 10 entries, **all page-sensitive**, base 4.
  The grouping is the whole trick: the table names both `$B6 ldx zp,Y` and true `abs,Y` as
  `absolutey` and separates them **only by `bytes`**. Counted by mode name alone, `abs,Y` reads look
  like 24 entries with 10 sensitive — "some `abs,Y` reads are insensitive", which is false. Found by
  the mailing-list distillation (helper-1), who **published the wrong count and then corrected it**
  from the same table; re-run here independently and matching in every cell.
  📖 **Still documented-only**: nothing above is a *measurement of hardware* — it is what our engine
  believes. `litmus_6502` covers `LDA abs,X`, `STA abs,X`, `BNE` and `DCP zp` against Stella; the
  other three modes have no litmus band.
- 📖 **NMOS decimal mode: only the C flag is valid** after ADC/SBC (never branch on Z/N/V); D is
  unknown at power-up and survives interrupts → `CLD` in init is mandatory. BCD idiom:
  SED/CLC/ADC…/CLD; multi-byte chains keep the carry.
  ✅ **The flag half is measured** (`docs/verified-coverage.md:88`, `litmus_6502` v0.44.0):
  $99+$01 under SED gives **A=$00 (correct)** and the pushed status $BD = **C=1 (correct), Z=0 and
  N=1 (both wrong for the decimal result)** — so "do not branch on Z or N" is our own measurement,
  not just the source's. **V is recorded (0) but nothing asserts it.**
  📖 **Not measured by us**: that D is undefined at power-up and survives interrupts. The `CLD`
  rule is *enforced* rather than measured — `scripts/check_traps.py` errors on an init with
  neither `CLD` nor `CLEAN_START` (see `docs/known-traps.md`), which is a lint, not a hardware fact.
- ✅ **JMP ($xxFF) page bug** — verified `litmus_6502` (v0.44.0), table row
  `docs/verified-coverage.md:89`: the indirect vector's high byte is fetched from **$xx00**, not
  $xx+1:00. `jmp ($F3FF)` lands on the buggy path and the ROM records the marker $92=$A5;
  regression-locked `roms/litmus/scenarios/cpu6502.json`.
- ⚠️ 📖 **BIT-as-NOP reads can strike TIA strobe mirrors — audit `.byte $2C` tricks.** Not measured,
  and **not caught by the linter either**: `scripts/check_traps.py` matches mnemonics
  (`READ_OP`), so a skip written as a raw `.byte $2C` / `.byte $0C` is invisible to it. The one
  such skip in our own tree is `roms/techniques/tia_pcm.asm:89`.
- ⬜ RMW double-write bus behavior on TIA strobes (6502.org silent; needs visual6502/64doc as source).
- ✅ **skipdraw/DoDraw is 17 or 20 cycles, not a constant 18** — measured 2026-09-03; this line said
  "constant-18-cycle draw" and added "worth a cycle litmus", which was an accurate self-assessment.
  Timed WSYNC→GRP0 over eight frames of `roms/techniques/vertical_pos_dcp.asm`: **20 cycles on the 80
  lines that draw** (the range branch taken, then `ldx sprDraw` / `lda ArtRev,x`) and **17 on the 1,686
  that skip**. The ROM's own comment already read `~17-20`; the audit line did not. A kernel budgeted at
  a constant loses three cycles on exactly the lines that draw — the tightest ones. The illegal `dcp`
  costs 5 and the emulator runs it, which this fixture also exercises.
  `→ internal/emu/skipdraw_test.go` (1 grading, 1 negative control: asserting 18/18 fails on both paths)
- 📖 Mirror templates (woodgrain Memory_Map): TIA at $xyz0 (x even, z∈{0,4}); RAM $80–$FF mirrored
  at **$0180–$01FF — which is why the stack works**, and the mechanism is that the 6507's stack
  pointer is **only eight bits wide** while the address bus is thirteen, so the processor supplies
  `$01` as the upper bits on every stack access — the programmer has no say in it, and the PIA
  being mapped into both pages is what makes the two views the same memory 〔stella 1999-08〕; ROM $1000–$1FFF mirrored at every odd $x000
  (incl $F000).
  ✅ **Two of the three measured** (`docs/verified-coverage.md:27`, `litmus_mirror` v0.49.0,
  regression-locked `roms/litmus/scenarios/mirror.json`): the **RAM mirror holds in both
  directions** — write $5A to $0180, read $5A at $0080; write $A5 to $0080, read $A5 at $0180 —
  and **one TIA mirror**, $0049 → COLUBK, checked by rendering ($84 blue at `read_row(100)`).
  📖 **Not measured by us**: the TIA template as a rule ($xyz0 for x even, z∈{0,4}) — one mirror is
  not the pattern — and the ROM mirroring of $1000–$1FFF at every odd $x000.
- ✅ **Convention: stack from $FF down (`LDX #$FF/TXS`), variables from $80 up — and the gap is now
  measured rather than hoped for.** `internal/ramtrace`'s activity report prints the stack
  low-water mark and the observed SP range, and it had never been run for this. Our own technique
  ROMs, 2026-09-03 (`go run ./cmd/ramtrace activity -rom <rom>`; SP points at the next free byte,
  so usage is `$FF − low`):

  | ROM | SP low | bytes |
  |---|---|---|
  | `bullets`, `flicker_multiplex`, `two_line_kernel`, `score6`, `paddle_demo`, `procgen_demo` | `$FD` | **2** |
  | `game_states`, `dyn_multisprite` | `$FB` | **4** |
  | `rts_dispatch` | `$F5` | **10** |

  `rts_dispatch` is the outlier by construction — it pushes return addresses as its dispatch
  mechanism, so its stack use *is* the technique. Everything else sits at two or four.
  Shipped games agree, from the list: **Space Instigators uses none, Fade Out and Marble Craze two**,
  and 6-8 is offered as enough for two or three levels of nesting 〔stella 2004〕. So a variable at
  `$F8` is safe in every kernel here except the one that dispatches through the stack — which is
  exactly the kind of thing a convention phrased as "hoping" cannot tell you.
  📖 **Not measured: the reverse trick** — deliberately using the stack region as scratch. The list
  offers it with its own caveat (*"just be careful about which temp variables each subroutine
  uses"*); `known-traps.md` covers a variable at `$FF` being clobbered by a `JSR` push and says
  nothing about going the other way.
  (Stella PG). Real-game RAM budgets: Pitfall ≈ all 128 bytes (world = 1 byte!), Random-Dungeon ≈45 with
  aliased overlays, za2600 overflows into cart RAM. ⬜ a RAM-map audit feature (symbols → read/write
  coverage) would catch dead variables (Pitfall's `cxHarry` is stored, never read).

## 10. Bank switching
- 📖 Scheme landscape (Horton's doc + woodgrain + threads): F8 8K ($1FF8/9) → F6 16K ($1FF6–9) → F4 32K
  ($1FF4–B), +SC 128B RAM variants; 3F/3E(+) for big data; DPC+/CDFJ need ARM (Melody/Harmony).
  **Community recommendation: F8 first** (max compatibility, cheapest PCBs, identical idiom scaling to
  F6/F4) — notably thread 338980 was started by DaveC himself.
- 📖 Best practices: vectors in **every** bank; identical reset stub per bank; same-address trampoline;
  TJ's distinct-RORG-per-bank ($1000/$3000/…) for debugger sanity; don't put code/data in the last bytes
  before vectors (accidental hotspot hits); SC RAM has separate write/read ports (no RMW; phantom reads on
  page-crossing indexed stores corrupt it).
- ✅(infra) **Gopher2600 supports all schemes we'd use** (F8/F6/F4±SC, FA, FE, E0, E7, 3F, 3E+, DPC(+),
  CDF*; not 0840) and **AUTO fingerprints a plain 8K dasm binary as F8** — our harness can verify
  bankswitching *today* with zero code changes. Bonus: `Cartridge.GetBank()` exposes the live bank →
  a tiny `read_bank` MCP tool is a natural addition.
- ✅(verified 2026-08-04, G1) **`read_bank` now has a witness beyond F8/F6/F4.** `roms/carts` holds a fixture
  per scheme and the bank count is asserted on each: **F6SC 4, F4SC 8, 3E 4 banks of 2048, 3E+ 4 banks of
  1024 at four origins, DPC 2 banks of 4096 plus 2048 bytes of graphics in no bank.** Bank SIZE is the part
  the harness used to assume: two of those five are not 4K. Every one of them is REFUSED by
  `internal/cyclebound`, naming its mapper and the reason, because in each the cartridge window is not the
  image. **Not verified: DPC+, CDF*, ELF/ACE and bus stuffing** — see `docs/capability-gap-audit.md` §G1
  for what specifically blocks each.

## 11. Procedural generation (new domain)
- ✅ **Pitfall's bidirectional LFSR — computed, not run** (samiam blog + disassembly; settled here by
  enumerating all 256 byte values, `scratchpad/lfsr_check.py`, no emulator involved). 1 byte = the
  world. **The step named for the direction the world scrolls, not for the direction the register
  shifts** — that distinction is the whole trap:
  **"right" = shift LEFT**, inserting `bit3⊕4⊕5⊕7` at bit 0;
  **"left" = shift RIGHT**, inserting `bit0⊕4⊕5⊕6` at bit 7.
  It cannot be read the other way and still work: **a shift right loses bit 0, so its tap has to
  contain bit 0; a shift left loses bit 7, so its tap has to contain bit 7.** The two tap sets in the
  sources each contain exactly one of those, and each fits exactly one direction. Read literally
  ("right step" = shift right with `bit3⊕4⊕5⊕7`) the function is **not even a bijection** and the
  orbit from $C4 is 34 long, with cycle lengths {1,2,3,4,31,32,33,34} over the 256 seeds.
  Read correctly: **both steps are permutations of 0–255, `left∘right` and `right∘left` are both the
  identity, 0 is the fixed point, and every one of the other 255 bytes lies on a single cycle** — so
  "period exactly 255" is exact, not approximate. **The disassembly's `bit1` is wrong**: `shr{1,4,5,6}`
  is not a bijection and fails to invert the right step for **128 of the 256** values.
  From seed **$C4** the world runs `$C4 $89 $12 $25 $4B $97 $2E $5C $B8 $70 $E0 $C0 $81 $03 $06 $0C …`
  and ends `… $11 $23 $47 $8E $1C $38 $71 $E2`; the "left" sequence is that one reversed.
  Regression handles: sha256[:16] of the 255-byte forward sequence = **751c0803eae3c1d4**, of the
  reverse = **62b12e47b5a03b55**.
- 📖 **DaveC's Random-Dungeon** (read in full): 2-byte room codes (walls/interior indices into ROM strip
  libraries); **exit-wall code spliced into the next room's entry wall** = infinite consistent dungeon with
  zero map storage; curated room-code tables (validity by construction); 8-bit Galois LFSR `eor #$8E`
  (period 255, confirmed) → later 16-bit; pacing counter for special rooms; 3 kernels dispatched per frame.
  His landscape evolved to 10 zones × per-zone x/y/tile arrays = 20 independent objects + per-line COLUPx.
- ✅ **LFSR hygiene (SpiceWare Step 10) — computed, not run** (`scratchpad/lfsr_check.py`, all 256
  values enumerated, no emulator): `lsr A / bcc + / eor #$B4` is a **permutation of 0–255**; **$00 is a
  fixed point** (hence "never seed 0" — it is not a caution, it is the only way the generator can
  fail); every other byte lies on **one cycle of length 255**, so the period claim is exact.
  From seed $01: `$01 $B4 $5A $2D $A2 $51 $9C $4E …`, ending `… $69 $80 $40 $20 $10 $08 $04 $02`;
  sha256[:16] of the 255-byte sequence = **1cc3384d72331258**. Seeding from INTIM is untested here —
  that is about *where the seed comes from*, not about the generator, and INTIM can read 0.
## 12. Harness/tooling implications
- 📖 **Stella IS automatable for F-4** (debugger doc + installed Stella 7.0 verified): `<rom>.script`
  auto-runs at `-debug` startup (`frame N / tia / riot / dump 80 ff 7 / saveSnap / saveSes`); `saveSes`
  writes the whole session to a text file; `-ss1x -sssingle` raw snapshots. Limits: GUI window always opens
  (no headless), no quit command (kill externally), no input timelines. **v1 design: RAM + TIA register
  compare at frame N** (exact, palette-free); image compare v2 (Stella doubles pixels horizontally; map
  palettes to TIA indices first). Needs a one-time frame-numbering calibration probe.
- ⚠️ AtariAge blocks direct fetching (Cloudflare 403) — use the Wayback Machine; randomterrain mirrors
  Davie/SpiceWare content. Disassembly corpus is ISO-8859+CRLF — `grep -a`.
- 📖 Davie's *Newbies* Revised PDF = editorial consolidation of Sessions 1–25 + opcode appendix; no new
  material; it **never covers** 6-digit score/paddles/BCD-display/random/sound — those live in SpiceWare
  Steps 3/10/13, score6.asm, and the Stella PG.

---

## Corrections adopted into our docs (the audit's ⚠️ list)
1. `cycle_counting_guide.html` positioning math = approximation; do not cite for positions.
2. Pitfall disassembly `LeftRandom` comment is wrong (bit0, not bit1) — carry the corrected formula.
3. SpiceWare Step 3 vs Step 7 left-PF1 window numbers conflict — to be settled by litmus.
4. The HMOVE comb / late-HMOVE behavior exists in **no** local source — Towers' TIA Hardware Notes was
   adopted as the authority and has since been **corroborated by our own measurement**:
   `litmus_hmove_side` (comb = left 8 px blanked on strobe-after-WSYNC lines even with HMxx=0;
   mid-visible strobe ~cyc 39 = a no-op; line-end strobe ~cyc 74 = left by HM+8 px with no comb),
   fixed by `roms/litmus/scenarios/hmove_side.json` and `internal/emu/hmoveside_test.go`. What is
   still open is narrower, and it is recorded where the measurement was made rather than here: the
   numbers are emulator-verified and the Stella cross-check is pending
   (`docs/verified-coverage.md:42`). **Line corrected 2026-09-03** — it had said "pending our own
   measurement" while sitting on top of the evidence that the measurement had been made.
5. Add to constants: 24-cycle HMxx freeze after HMOVE; NMOS-BCD C-only; stores never take page-cross
   penalties (deterministic kernel timing); CLD mandatory at init.

## Where the follow-ups live
The prioritized work items distilled from this audit (new litmus ROMs, `read_bank`, audio sample capture,
Stella oracle automation, `pkg/audio` tables) were tracked in the v2 backlog — now **delivered (see
`CHANGELOG.md`)**, with any remaining gaps folded into the single live backlog **`capability-gap-audit.md`**.

## Mid-line HMOVE — verified (2026-06-12, litmus_hmove_mid)

Strobing HMOVE outside the post-WSYNC slot, with **all HM registers cleared** (HMCLR'd):
measured on Gopher2600 with pixel-level confirmation (bar edge above/below the strobe line):

| strobe completion (visible clock) | shift |
|---|---|
| 13  | 0 px |
| 85  | 0 px |
| 142 | **−5 px (left)** |
| (control: no strobe) | 0 px |

*(Clocks corrected in v1.32.0: the original ≈1/73/130 were hand-counted estimates; `trace_clocks`
measured the actual strobe completions — rule 2, "get cycles from the simulator", applies to
clocks too.)*

The folk rule "objects move right ~1px/4CLK" did **not** reproduce at these sample points — the
shift is a non-monotonic function of strobe time (consistent with Towers' per-cycle tables being
more complex than the summary line). Regression-pinned in `scenarios/hmove_mid.json`. For
authoring: keep HMOVE in the post-WSYNC slot unless deliberately exploiting the quirk, and if
exploiting it, measure your exact strobe cycle with this litmus pattern first.
