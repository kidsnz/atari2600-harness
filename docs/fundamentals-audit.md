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
- 📖 VSYNC procedure: set D1, wait ≥2 lines, clear (Stella PG ~§3).
- 📖 RIOT timers TIM1T/8T/64T/1024T ($294–7): write 1–255; **after expiry INTIM holds 0 for one interval,
  then flips to $FF and decrements 1/cycle** (lets you measure how late you are) (Stella PG PIA §2.3).
  TIMINT $285: D7 expired flag. ⬜ whether reading INTIM clears D7; exact first-decrement offset.
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
- 📖 Missiles have **no** vertical delay (so in a 2LK they start only on even lines).
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
- 📖 CTRLPF D1 SCORE (left half→COLUP0, right→COLUP1), D2 PFP priority (PF/BL above players),
  D4–5 ball width 1/2/4/8 (Stella PG). ✅ **SCORE×PFP interaction measured** `litmus_score_pfp`
  (v1.53.0): **PFP dominates** — with D2 set, D1 has no effect (PF renders in COLUPF on BOTH
  halves, with priority over players); $02→halves colored, $04 and $06→identical COLUPF rendering.
- 📖 Asymmetric PF under reflection via double PF0 rewrite per line is real-game practice
  (DaveC's Random-Dungeon `_room_loop`). ⬜ unverified by us.

## 5. Collisions
- ✅ 3 of 15 pairs (BL-PF, P0-P1, M0-P0), sticky latches, CXCLR.
- ⬜ remaining 12 pairs. 📖 read idiom: one `BIT CXxx` yields two pairs via N and V flags.
- 📖 flicker collision attribution (za2600 `EN_LAST_DRAWN`): alternating-frame entities must track whose
  collision the latch belongs to — a verifiable pattern once we do flicker.

## 6. Audio
- ✅ AUDC/AUDF/AUDV register readback; audio digest golden.
- 📖 **Complete AUDC table consolidated** (Slocum guide v1.02 — held locally, authoritative; Stolberg's
  frequency/waveform guide; Stella PG): duplicates {0,11} {4,5} {6,10} {7,9} {12,13}; usable voices —
  Square(4), Bass(6), Pitfall(7), Noise(8), Buzz(15), Lead(12), Saw(1), Engine(3). Pitch:
  `f = base/(AUDF+1)/D`, base ≈ 31,399.5 Hz NTSC (clock/114, 2 samples/line), CPU-clock modes (12–15)
  ÷3; D = 2/31/31/511/93/6/15/465. PAL ≈13 cents flatter. Slocum's three tuning setups (which
  (AUDC,AUDF) pairs are in tune) are transcription-ready for `pkg/audio`.
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
- 📖 SWACNT/SWBCNT DDRs — documented only (rarely game-relevant).

## 8. 6502/6507 precision
- ✅ cycle accounting (76/line; WSYNC-stall exclusion).
- 📖 Page-cross +1 applies to **reads** (abs,X / abs,Y / (ind),Y); **stores are fixed** (STA abs,X always 5,
  (ind),Y always 6); RMW abs,X fixed 7. Branches: 2, +1 taken, +1 page-cross **measured from the next
  instruction's address**. (6502.org.)
- 📖 **NMOS decimal mode: only the C flag is valid** after ADC/SBC (never branch on Z/N/V); D is unknown at
  power-up and survives interrupts → `CLD` in init is mandatory. BCD idiom: SED/CLC/ADC…/CLD; multi-byte
  chains keep the carry.
- 📖 JMP ($xxFF) page bug. ⚠️ BIT-as-NOP reads can strike TIA strobe mirrors — audit `.byte $2C` tricks.
- ⬜ RMW double-write bus behavior on TIA strobes (6502.org silent; needs visual6502/64doc as source).
- 📖 skipdraw/DoDraw constant-18-cycle draw, illegal `dcp`=5 cycles (Davie S23) — our emulator runs these;
  worth a cycle litmus to also certify illegal-opcode support.

## 9. Memory map, RAM & stack
- 📖 Mirror templates (woodgrain Memory_Map): TIA at $xyz0 (x even, z∈{0,4}); RAM $80–$FF mirrored at
  **$0180–$01FF — which is why the stack works**; ROM $1000–$1FFF mirrored at every odd $x000 (incl $F000).
- 📖 Convention: stack from $FF down (`LDX #$FF/TXS`), variables from $80 up "hoping the two never meet"
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
- 📖 **Pitfall's bidirectional LFSR** (samiam blog + disassembly, simulated & confirmed): 1 byte = the
  world; right step inserts bit3⊕4⊕5⊕7, left step inserts bit0⊕4⊕5⊕6 (**the disassembly's comment says
  bit1 — wrong; simulation proves bit0**); period exactly 255; left∘right = identity. Expected sequences
  from seed $C4 are computed and litmus-ready.
- 📖 **DaveC's Random-Dungeon** (read in full): 2-byte room codes (walls/interior indices into ROM strip
  libraries); **exit-wall code spliced into the next room's entry wall** = infinite consistent dungeon with
  zero map storage; curated room-code tables (validity by construction); 8-bit Galois LFSR `eor #$8E`
  (period 255, confirmed) → later 16-bit; pacing counter for special rooms; 3 kernels dispatched per frame.
  His landscape evolved to 10 zones × per-zone x/y/tile arrays = 20 independent objects + per-line COLUPx.
- 📖 LFSR hygiene (SpiceWare Step 10): `lsr/bcc/eor #$B4` (8-bit, period 255), seed from INTIM, never 0.

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
4. The HMOVE comb / late-HMOVE behavior exists in **no** local source — Towers' TIA Hardware Notes adopted
   as the authority, pending our own measurement.
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
