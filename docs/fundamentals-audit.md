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

The actionable follow-ups live in `hardening-roadmap.md` § "v2 backlog". Verified facts remain cataloged in
`verified-coverage.md`.

---

## 1. Frame & timing
- ✅ NTSC 262 (3/37/192/30), PAL 312; 1 line = 228 clocks = 76 CPU cycles; cycle-counting invariant.
- 📖 VSYNC procedure: set D1, wait ≥2 lines, clear (Stella PG ~§3).
- 📖 RIOT timers TIM1T/8T/64T/1024T ($294–7): write 1–255; **after expiry INTIM holds 0 for one interval,
  then flips to $FF and decrements 1/cycle** (lets you measure how late you are) (Stella PG PIA §2.3).
  TIMINT $285: D7 expired flag. ⬜ whether reading INTIM clears D7; exact first-decrement offset.
- ⬜ SECAM; real-game variable line counts (we already treat 262 as a range).

## 2. Horizontal positioning & HMOVE
- ✅ X(N)=3N−55 (missile/ball), player +1px; slope 3 px/cycle; divide-by-15 coarse; leftmost player 3 /
  missile 2; all 16 HMOVE nibbles (+7..−8, positive = left) right after WSYNC.
- ⚠️ `reference/docs_atari/cycle_counting_guide.html` uses `X=(CYCLES−20)*3` and "round to 15" — both are
  tutorial approximations. **Never cite it for positioning**; our calibrated formula is more precise.
- 📖 **Do not write HMxx within 24 CPU cycles after HMOVE** — "unpredictable motion" (Stella PG, stated 5×;
  also visible in score6.asm's deliberate HMCLR delay). Not yet in our constants — adopt.
- 📖 **HMOVE mechanism** (Towers, *TIA Hardware Notes*): HMOVE right after WSYNC **extends HBLANK by exactly
  8 color clocks** → the famous left-side 8px black comb on HMOVE lines; movement = clock stuffing (1px
  left per extra pulse). **Late HMOVE during the visible line "plugs" MOTCK pulses → moves objects RIGHT at
  1px/4CLK** (the Cosmic Ark starfield family). None of this is in our local spec shelf — Towers
  (https://www.atarihq.com/danb/files/TIA_HW_Notes.txt) is the adopted authority. ⬜ all of it unmeasured.
- 📖 **RESPx pipeline** (Towers): counter reset → first visible copy appears **5px right** of the reset
  point (explains our verified +5 family offsets). **RESBL re-emits START (ball restartable mid-line);
  RESPx does not** (player needs a 160-clock wrap). ⬜ double-strobe behavior unmeasured.
- 📖 missile-locked-to-player (RESMP D1): release leaves M centered on P (Stella PG). ⬜ exact lock offset.

## 3. Sprites (players)
- ✅ GRP bit order (D7 left), row order, NUSIZ double/quad/3-copies, REFP, P0+P1 16px combine.
- 📖 **VDEL exact semantics** (Stella PG §6.D — the load-bearing mechanism): each GRP has new+old copies.
  **Writing GRP0 copies P1's new→old; writing GRP1 copies P0's new→old, and also ENABL's new→old.**
  VDELPx/VDELBL D0=1 selects the *old* copy for display. This write-triggered cross-copy is what powers the
  2-line kernel alignment AND the 48px/6-digit score trick. Testable relation (SpiceWare Step 4): in a 2LK,
  VDELP0=1/VDELP1=0 aligns sprites at the same Y; 0/1 aligns at Y+1. ⬜ unverified — top backlog item.
- 📖 Missiles have **no** vertical delay (so in a 2LK they start only on even lines).
- 📖 Moveable-object writes are shear-safe at CPU cycles 0–22 of the line (HBLANK 68/3) (SpiceWare Step 4).
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
  D4–5 ball width 1/2/4/8 (Stella PG). ⬜ SCORE×PFP interaction is specified nowhere — measure.
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
- ⚠️ **The audio digest cannot verify pitch** (it's a hash, not a measurement). Gopher2600's
  `television.AddAudioMixer` exposes raw per-channel samples (2/scanline) — a capture hook + zero-crossing
  /autocorrelation measurement is the missing capability that makes the note tables falsifiable in CI.
  Cheap today with no new code: **duplicate-AUDC digest-equality scenarios** ({4,5} etc. must hash equal).
- ⚠️ slocum-tracker's default export has a comment/data mismatch (Engine slot emits 14 not 3) — check
  `soundTypeArray` on imported songs.

## 7. Input
- 📖 SWCHA joystick bits (P0 high nibble R/L/D/U, 0=pushed), SWCHB console switches, SWACNT/SWBCNT DDRs.
- 📖 INPT4/5 fire: D7, 0=pressed; **VBLANK D6=1 enables latch mode** (stays 0 once pressed; disabling
  resets to 1). Test with N flag, never Z (bus noise in low bits).
- 📖 Paddles INPT0–3: **VBLANK D7=1 dumps the caps**; clear → caps charge; count scanlines until D7=1.
- ⬜ none of this is litmus-verified; `set_input` exists but its paddle path is uncalibrated.

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
Stella oracle automation, `pkg/audio` tables) are tracked in **`hardening-roadmap.md` § v2 backlog**.

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
