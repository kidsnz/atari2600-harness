# Atari 2600 visual design principles (design-principles)

The canon of "graphics-design principles that can be reduced to rules", obtained from mining (AtariAge) plus
web research. Purpose = (1) explicit rules for Claude's design judgement (roms/EVALUATION.md's ⑥craft)
(2) the basis of `pkg/design` feasibility judgements (also reusable for the frozen TIA Studio templates).
Detailed sources = `tools/research-w2-design.md` + `docs/mining-digest.md` (mined-thread index) + `reference/atariage/*/notes.ja.md`.

**The executable judgements are already "absorbed" into `pkg/design/`** (machine-checked before any asm is written).
Rules that can be quantified name their function at the end of the line with `→ func`. Judgement rules that
cannot be quantified are collected in the final section, "Judgement rules no machine can decide".
Mapping: colour bands / gradients / mixing = `color.go` / horizontal position = `position.go` / PF windows, 2-colour score, scrolling = `pf.go` /
multiplexing = `multiplex.go` / character count = `text.go` / budget = `budget.go` / drawing craft = `craft.go`.

## Colour (most important)
- **Hold colour as a register value / symbolic name (hue = upper nibble × lum = lower nibble), never as RGB**. Don't scatter raw hex.
  Luminance is effectively 8 steps (bit0 has no effect). Design goal for PAL/NTSC = two parallel sets (N_xx/P_xx) switchable in one line. 〔Davie S11, symbolic-color-names〕
- **Add colours VERTICALLY = rewrite COLUPx per scanline** (one colour horizontally). **Horizontal multi-colour is expensive** (only the fakes: PF score / Chronocolour / flicker / stacking). 〔Hugg, Davie S21〕
  - **Minimum width of a horizontal colour band = store-instruction cycles × 3 colour clocks**. An arbitrary
    colour costs ~6cy per band (about 8 bands per line is the ceiling). There is also the trick of borrowing SP
    (`txs`/`tsx`) as a 4th colour register. 〔170018 multiple-colors-per-scanline〕 `→ design.MinColorBandWidthPx/CheckColorBands`
  - **Two separate numbers govern a PF-aligned band, and the source welded them with an `=` that is false**
    (resolved 2026-08-06; it read "multiples of 4 colour clocks (= 12px)"). Both figures are right about their
    own thing, and both are already machine-locked in this repo rather than taken from the thread:
    - **WHERE a boundary can fall: multiples of 4 colour clocks.** One playfield pixel is 4 colour clocks wide
      (40 columns × 4 = 160), so a PF-aligned edge cannot land anywhere else. Pinned at the pixel by
      `TestEveryPlayfieldColumnLandsWhereTheTableSays` — `litmus_pf_allcols` lights one column per band and all
      20 column positions are re-measured, not the leftmost-bit-of-each-register sample the older `litmus_pf` took.
    - **HOW WIDE the narrowest band can be: 3 colour clocks per CPU cycle**, so a 4-cycle `STx.w` buys 12.
      Pinned by `cmd/calibrate`'s sweep of `litmus_pos`: slope 3px per CPU cycle, R² = 1.000000.
    **The two compose, which is what the original was reaching for**: 12 is a multiple of 4, so a band written
    with a 4-cycle `STx.w` is automatically on the PF grid — 12 colour clocks is exactly 3 PF pixels. A 3-cycle
    `STA zp` buys 9 clocks, which is NOT a multiple of 4 and therefore cannot start and end on the grid.
- **There is no "one correct RGB"**: Stella generates the palette from YIQ dynamically, so the same register value differs by a dozen up to 0x20 between emulators and settings.
  For us the running table `internal/ingest/palette_stella.go` is authoritative (100% match against Stella). 〔rgb-color-values, 118495〕
- **hue ↔ colour map**: hue1 = yellow / hue4 = red / hue8 = blue / hue12 = green (hue15 ≈ hue1). hue1 is the standard choice for yellow. 〔132561〕
- **The higher the luminance the lower the saturation — it washes out toward white** (bright blue in particular stops being identifiable) → **place colours you want to read as vivid at mid-to-low luminance**. Saturation and luminance trade off. 〔132561〕 `→ design.Hue/Luminance/WashoutRisk, HueName, GradientSameHue, SameLuminance`
- **The atom of the colour data model is "colour per scanline" = `colorPerRow[]`**: holding an array of scanline index → COLUPx value instead of a single `color` expresses vertical multi-colour (the cheapest multi-colour) directly. TIA Studio's M1 design decision converged on this too. 〔research w4 / `tools/research-w4-m1-open-questions.md`〕
- **Background "shimmer / noise texture" is just streaming bits of the random seed into `COLUBK` every scanline (no dedicated RAM)**: water shimmer, sandstorm, twinkling stars — copy bits of the LFSR/randomSeed you already run into `COLUBK` per band and get them at **almost zero cost**. 〔Fishing Derby `.colorWaterShimmer` = a water effect that streams randomSeed bits into per-line COLUBK〕

## Sprites (P0/P1)
- 8 dots wide, one register (GRP 8-bit, MSB = leftmost). Width via NUSIZ 1x/2x/4x. 〔2k6specs, Davie S21〕
- **Horizontal position = two stages**: coarse ÷15 (5cy loop) → fine HMOVE. Granularity 3px per CPU cycle (matches litmus). 〔Davie S22〕 `→ design.PositionSplit/CoarseIterations`
  - **The coarse-positioning line IS the kernel's tightest interval**: the `sta WSYNC`→`sta WSYNC` interval holds the ÷15 loop plus the fine tail (`eor` / ASL×4 / `sta`), so it **gets heavier in proportion to X**, and at maximum X the **worst case exceeds 76cy → the picture rolls** (a lucky test at small X passes; only a ∀ proof surfaces it). `prove_line_budget`'s `; @amax N` must sit on the line of the **`sta WSYNC` that OPENS the interval** to bind it (it has no effect on the `sbc` line). Ways to fit: **restrict the range of movement**, or **turn the ÷15 into a lookup table so it becomes constant-time** (tabulation is the standard density move). Constraining the loop alone still overruns while the fine tail remains. 〔density-ladder rung2 measured / prove_line_budget roll_free 2026-07-24〕
- **★RESxx's internal draw delay (first suspect in any position mismatch)**: the `RESxx` strobe resets the counter immediately, but **the object actually starts drawing later = player +5 / missile and ball +4 colour clocks** (if RESP0 completes at cycle 46, X ≈ 75). Demonstrated from the Stella source (`renderCounterOffset`). **When a target X is off by ~5px, suspect this first.** RESxx granularity is 3 colour clocks. 〔mining 294398, 283075, 305780, 172089, 137739, 329611, 304182〕 (this is the quantity that explains the codified `X=3N−54/55` from behind = **measure it against litmus_pos** before turning it into a constant)
- **Position formula and write window for multiple objects**: `RESxx` while the beam is visible is forbidden (the immediate reset bends the picture) = look ahead in HBLANK or on the previous line. A shared loop walks consecutive `RESP0,x`/`HMP0,x` with `DEX/BPL` (`design.shared_setxpos`, implemented). The right-edge overflow limit is X ≈ 134 (the real cause is "N objects = N+1 scanlines"). 〔mining 67045, 308513, 340965, 311795 (RESxx × HMOVE race = implemented in Gopher2600)〕
- **Burn one cycle to land RESP where you want it**: when coarse positioning has no NOP to spare, `sta.wx HMP0,x` (dasm `.w`/`.FORCE` forces Absolute,X = 5cy; ZP,X is 4cy) adds 1cy so the RESP0 strobe lands on **the cycle you intended**. 〔mining blog SpiceWare 12538〕
- **Masked sprite drawing = 21cy** (cheaper than DoDraw's 26cy): `lda (img),y / and (mask),y / sta GRPx / lda (color),y / sta COLUPx` = **clip the shape and update the per-row colour at the same time**. Zero-pad so different sizes share a single mask. 〔mining blog SpiceWare 10890, 339509〕
- **div15's fine-movement range is implementation-dependent**: a naive div15 gives **−6..8px**; symmetrising with `eor #15` + `adc #((8+1)<<4)` gives **−7..8px**. Origin = Decuir / Video Olympics. Separate from HMOVE's raw hardware range (−8..+7) = it is a property of the routine. 〔mining 286698〕 (needs litmus backing)
- **For early-HMOVE (HMOVE before WSYNC), the "do not move" value is HMPx $80 (= 8), not $00**: with $00, an object spanning the same scanline drifts 8px. The idiom positions with a dedicated 15px × 11 kernel. 〔mining 169471〕 (needs litmus)
- **Striking HMOVE at cycle 73–74 suppresses the left-edge comb (black line)**: the known Cosmic Ark-family trick. 〔mining 165428, 183219, 319456 "HMOVE Shuffle"〕 **Measured 2026-09-03** — `roms/litmus/litmus_hmove_side.asm` band D is now graded: a late HMOVE adds **8 to the nibble** (HMP0 = $10 delivers nine clocks left, not one) and paints **no comb**, while a strobe right after WSYNC paints the comb with every HMxx at zero. `→ internal/emu/hmoveside_test.go`
- **Indirect-jump positioning: `JMP (ptr)` into a table of hardcoded positioning lines replaces the delay loop.** Each extra variant defers the RESPx strobe by 5 CPU cycles = **15 colour clocks**; the HMPx nibble loaded before the dispatch and applied by the HMOVE after it fills in between. **Nine variants reach 128 contiguous positions** — measured 136 — and they are contiguous *only* because the fine range (16 values, −7..+8) is at least the coarse step (15): the reachable intervals [15k−7, 15k+8] and [15(k+1)−7, 15(k+1)+8] meet exactly at their endpoint, so one fewer fine value puts holes in the set. Pays ROM for cycles, plus ≥ 2 bytes of RAM per object (pointer + HMOVE value). 〔Stella list, `star fire - return of the starfield ?!?`, 2002-07: Erik Mooney 200207/msg00330 + msg00334 for the mechanism, Manuel Polik 200207/msg00332 for the nine-parts arithmetic〕 `→ roms/litmus/litmus_jmpind_pos.asm` / `internal/emu/jmpindpos_test.go` (5 gradings, 2 negative controls). **Provenance correction:** this file previously credited the idea to Omegamatrix. That attribution is both later and a *different* shape — Omegamatrix folds the jump index into the HMPx low nibble, Erik's 2002 form dispatches through a plain pointer computed off-screen and keeps the nibble for movement.
- **48px** = NUSIZ $03 (3 copies) + P1 shifted 8px right + VDEL double-buffering to swap GRP with a time offset. Reuse score/bitmap48. 〔48px-positioning〕 `→ pkg/sprite.SplitWide/NUSIZ / design.MaxChars(Text48px)`
- **Do not fix the picture first and then assign objects.** Order = colour budget → assignment table → negotiate any shortfall via "share colours / double up objects / change the layout".
- missile/ball = lines, edges, vertical frames; player = area via double width / multiple copies / 4x. Build one apparent shape by stacking several objects.
- **A single irregular shape wider than 8px = "shape ONE player with a per-scanline NUSIZ + HMOVE table" — don't fall back on flicker**: keep GRP small and switch NUSIZ (size 1/2/4/8, copy count) and HMOVE on every scanline, and one player "stretches" into an irregular shape ~40 colour clocks wide (fish / shark / ship / wide creature). Accept a single colour. No extra object, no flicker. Confirmed on a live run. 〔Fishing Derby (David Crane / Debro disassembly) SharkTraveling*NUSIZValues = a shark made of per-line NUSIZ + HMOVE; ~40-clock width confirmed by running build/fishing_derby.bin〕 `→ casebook.md "large irregular shapes"`
- **A 1px line at an arbitrary slope = missile/ball + fractional-HMOVE accumulation (Bresenham in HMOVE)**: take an M/BL drawn vertically, `adc` the slope held as integer + fraction on every scanline, and on carry apply a ±1px HMOVE through `HMMx`/`HMBL` — that yields the **diagonal lines** of a fishing line / tether / rope / laser (drop the assumption that only vertical and horizontal are possible). 〔Fishing Derby fishingLineSlope (Integer/Fraction) + HMOVE; right line = BL, left line = M1; the right line's slope confirmed on a live run〕 `→ casebook.md "diagonal lines"`

## Playfield
- 40px across × 4 clocks per px. Expressive power is earned through vertical rhythm. 〔Davie S13〕
- **A scrolling PF background is 3 layers — board RAM + display buffer + delta update** — plus tile-granular scrolling (avoids tearing). Iron rule = **keep the total scanline count constant from frame to frame** — the legal totals and the PAL parity constraint are the rule
  immediately below, stated once rather than twice. The scroll band is 10–16 lines top and bottom. 〔200972 tile-scrolling-engines, Boulder Dash style〕 `→ design.ScrollScanlinesConstant` **and `design.ScrollBackgroundFitsRAM`**. **Corrected 2026-09-03:** this line prescribed three layers and pointed only at the scanline check, which looks at line counts and PAL evenness and nothing else — so nothing asked whether the three fit. The same source says they usually do not: **a world you rewrite at run time needs SuperChip/CBS RAM, because the internal 128 bytes only hold a 120-byte-class malleable world** 〔200972:14〕. Budget the three layers plus the stack against `design.RAM2600` before choosing this structure. Found by auditing harness claims against the sources harness itself cites.
- **PAL frames must have an even scanline count** — an odd total is not a legal PAL frame, so a
  kernel that varies its line count must vary it in twos. Safe zone 262 (NTSC) / 264 (PAL).
  `→ design.ScrollScanlinesConstant`, which carries **two** checks under one name: the count is
  constant frame to frame, **and** it is even when `pal` is set (`pkg/design/pf.go:60`).
  **Promoted to its own rule 2026-09-03.** It had been living as a parenthetical inside the
  scrolling-background bullet above, which is a different subject and carries a different name.
  The distillation measured the cost of that: **eight corpus references cite this claim and every
  one of them cites a line number that no longer holds it** — the worst rate of any claim in this
  file. A claim with no name of its own has no address, so nothing can point at it and survive an
  edit. That is the general lesson, not a fact about PAL. 〔200972 tile-scrolling-engines〕
- **For HUD/text the character COUNT picks the technique**: 48px = 12 characters / venetian blinds = 32 characters (but only at 3px width). Either split the HUD into its own screen mode, or isolate it in a zone and reuse the score area for several purposes. 〔197162 text-hud〕 `→ design.MaxChars`
- **An asymmetric PF is expensive** (PF0/1/2 written twice mid-scanline; the PF0 window is only ~20cy). Compromises = central 32px / every other line with double height / venetian / RAM self-modification. 〔Davie S17, castlevania-port〕
- **Write deadlines for an asymmetric PF (measured cycles)**: when you display the left half and rewrite the right half on the same scanline, aim each write at the moment that PF is **no longer visible**. The classic kernel's actual values =
  first pass PF0[cy7] / PF1[cy14] / PF2[cy21] (for the left half — in time before it becomes visible) → then for the right half
  **PF0 rewritten at cy31 / PF1 at cy38 / PF2 at "exactly cy45"** (too early or too late and it breaks — adding a single nop destroys it).
  What remains, 76−47 ≈ **29cy per line, is the free budget for sprites and the like**. Judge whether a horizontally multi-coloured PF is feasible by asking "can we hit that single 45cy point without fail, and does the rest of the work fit in the remaining 29cy?". 〔Williams/Saunders "Asymmetric Reflected Playfield" tutorial〕
- **A free 2-colour PF = CTRLPF D1 (the score bit)**: set bit1 and **the left half of the PF takes COLUP0, the right half COLUP1**, independently coloured (no asymmetric write timing needed). The staple for score display, but also a cheap way to colour the left and right of a background differently. 〔w11/Asym2scrol〕 `→ design.ScoreModeTwoColor`
- **The saving trade of giving up PF0**: not drawing PF0 (on top platforms and the like) forces PF2 to be written at **exactly cycle 48** instead, but it frees **12cy per line + 18 bytes of RAM**, and lets the player fall off both edges of the screen (an advantage of the reflected PF). 〔mining blog SpiceWare Stay Frosty〕
- **Vertically moving platforms use two zones of complementary height**: build the upper and lower band heights so that "when one grows the other shrinks by the same amount" and the total line count stays constant = a stable picture (mismatched, you get motion blur). 〔mining blog SpiceWare〕
- **Visible delay on a PF register write**: an `sta` to PF0/PF1/PF2 takes effect **2–3 colour clocks late**
  (colour registers are immediate). Complete the centre boundary of a reflected PF at **exactly cycle 48**
  (measured in mining 149228; clone consoles are +1cy) — consistent with the **"PF2 at exactly cy45"** deadline
  in the asymmetric-reflected-PF rule above, the 3-clock write delay being the difference. Do the timing
  arithmetic for a horizontally multi-coloured PF with this delay included. 〔mining 149228 PF write-timing table〕
  (The dangling "at line 38" this sentence used to carry is resolved 2026-08-06: it was a line number into an
  earlier revision of THIS file, and the rule it pointed at is the cy45 one now cited by name. Line numbers do
  not survive editing; a reference has to name the rule.)

## Multiplexing and flicker
- **★Flicker multiplexing DISABLES the TIA's hardware collision detection, and the reason is a miss rather than
  an inaccuracy**: two objects that are colliding may never be drawn on the SAME FRAME, so `CXPPMM` and friends
  simply never latch. The player sees "I hit it and nothing happened". Choosing flicker therefore decides the
  collision architecture too — it has to move into software. Cheapest first step, from the same source: test
  collisions **only for a sprite that MOVED this frame**, since most sprites are stationary; the cost is that
  an overlap present the moment a screen appears goes unnoticed until something moves. 〔blogs 8429 SpiceWare/Frantic〕 `→ design.HardwareCollisionUsable`
- Beyond 2 objects, multiplex by Y band; a horizontal repositioning costs one scanline; **an empty Y lane is mandatory**; the price is 30Hz flicker. 〔Bumbershoot〕 `→ design.NeedsFlicker/NeedsEmptyYLane/RepositionCostScanlines`
- **Turn one sprite into many by rewriting GRP mid-scanline**: duplicate a single player with NUSIZ and re-`STA GRPx` just before each copy is drawn, and **every copy can be a different picture** (the shared basis of Space Invaders formations, 6-digit scores, and varied enemy rows). Keep `STA GRPx` strictly inside HBLANK. 〔mining 337131, 182923〕
- **Multi-kernel = reuse one object per region**: switch `REFP` / position / picture per Y band and reuse a single player for different purposes (Stay Frosty). Match a "never overlap on the same line" placement constraint with an AI that "never enters an occupied column" and flicker is zero. 〔mining 303364, 318140, 164247〕
- **Flicker is a last resort, and only for short-lived objects.** Never over a large area. Don't trust the emulator — verify by compositing several frames. 〔flicker-to-enhance-graphics〕
  **This is a POSITION, not a measurement, and the list held the opposite one.** It rested on a single
  source and stated no cost for following it. Glenn Saunders, then the list's administrator, 1997:
  *"Oystron and Rescue avoid flicker but constrain sprite placement and movement to do it. That's
  fine, but it narrows the horizon of what's possible on the 2600."* — *"flicker is another 2600
  programming strategy and it opens up a lot of territory."* His exemplars were **Solaris** and
  **Star Wars: The Arcade Game**, *"paragons of intelligent sprite flicker and reuse."*
  **The default here stays "last resort"**, because this project's own recorded preference is to cut
  rather than to add and not to let a technique show — a taste decision, made once, not re-argued per
  page. What changes is that the alternative is now written down with its advocate, and that **the
  cost of avoidance is named**: refusing to flicker forces objects apart in Y, and that is a
  measurable narrowing (`pkg/design/multiplex.go`'s `NeedsEmptyYLane` is the existing foothold —
  how many placements survive the constraint is a number, not an opinion).
  Two things remain matters for the eye and not for this file: whether a given flicker reads as
  motion or as damage, and how large an area is too large. Found by the mailing-list distillation
  (helper-1), who flagged it as belonging to the artist rather than to the harness.
- **Flickering more than 2 objects: list reordering REPLACED age-based, and it costs priority.**
  The older way is **age-based** — count how many times each object has been shown and display the
  oldest next. The newer one is **list reordering**: try each object in FLICKERLIST order, move the ones
  you drew to the end and the ones you could not to the front, so next frame's order falls out of this
  frame's. **It abolishes the age loop rather than sitting beside it** 〔blog SpiceWare 10777:8〕.
  **The price is that index order stops being priority, so the player's own ship starts flickering** —
  which is the thing an author notices last and minds most. **Put both players fully into the flicker
  pool and you reach ~24 objects** (Frantic is the real example). The design layer above
  `flicker_multiplex`/`dyn_multisprite`. **Corrected 2026-09-03:** this line offered the two as parallel
  choices and carried neither the supersession nor the cost; both were in the cited note.
  〔mining blog SpiceWare 10777:8, 11656〕
- **76cy per line is the ceiling.** Decide the line count first, then allocate features out of the remaining budget. 〔splendidnut〕 `→ design.LineBudget/RemainingCycles`
- **★RIOT 6532 timer wrap-around bug (the "Stella passes / real hardware rolls" trap)**: write `TIM64T`/`TIM1024T` on **exactly the cycle** the timer wraps around and the divider silently degenerates to **1T**, wrecking the frame length so the picture rolls on hardware. **The fix = a double write (double-write TIM64T).** Easy to miss because it is emulator-dependent = a direct hit on the harness's core mission (gap B). Diagnosed in that thread by Gopher2600's author (JetSetIlly). 〔mining 303277 "To Roll or not to Roll"〕 (harness-hardening candidate = an assert that detects a timer write on the wrap-around cycle)
- **Hard lower bounds on the vertical budget, and asymmetric failure modes**: given that the total scanline count is held constant, the lower bound of each region = VSYNC ≥ 3 / Overscan ≥ 3 / VBLANK ≥ 15 (even for PAL). **Why this is yours to do at all** (added 2026-09-04): the designers removed it on purpose —
  *"They also eliminated any provision for vertical synchronization and gave that task to the
  programmer."* Of a piece with the rest of the machine: *"making the software do as much of the
  work as possible, so that the hardware could be cheaper — silicon was very expensive in those
  days."* Their statement of the budget is *"must finish displaying a single frame in exactly the
  same time — 15.24 milliseconds"*; that is their round figure and not our measured refresh
  (NTSC 15734.26/262 = 60.0544 Hz = 16.65 ms), so do not carry it as a constant. 〔Perry & Wallich, IEEE Spectrum 1983-03〕 **Overstretching Overscan = no picture / overstretching VBLANK = jitter** — the failures show up differently, so do not absorb the surplus on the VBLANK side (a jitter source). 〔mining 171270〕
  (Ambiguity resolved 2026-08-06. The Japanese original's word order read as a contradiction; the sentence
  above is the only reading its own two failure modes support, so nothing is left to decide about the WORDING.
  **The CLAIM, though, is not verifiable here and is not treated as measured**: "no picture" and "jitter" are
  behaviours of a real television, and an emulator shows neither — Gopher2600 renders an over-long Overscan and
  an over-long VBLANK alike. What this harness can see is the frame's line count, which is a different
  quantity, gated by `frame_lines_stable` and `TestNoRomBreathesAcrossFrames`. Cited to 〔mining 171270〕 and
  left there.)
- **WSYNC semantics**: `sta WSYNC` halts the CPU until **the start of the next HBLANK** (68 colour clocks = 22⅔ CPU cycles). Choose where to write with the register-update delays in mind (colour = immediate / PF = 2-3 clocks / VBLANK = +1 line / note length = delayed). 〔mining 192183 register-update delay table〕
- State = one GameState variable + a kernel per state. A title picture is padding top and bottom + a central PF table, clearing GRP/PF at the end. 〔title-to-game-transition〕
- Cycle saving = the unofficial ISC/ISB opcodes + borrowing SP as a line counter (needs litmus backing). 〔5cycle-color-cycling, illegal-opcodes〕
- **Stability map for illegal (unofficial) opcodes**: **the ones that are stable on real hardware are the LAX/SAX/SBX/DCP family**. **LXA/XAA are unstable = do not use** (they depend on the individual chip and on temperature). **`ASR`/`ALR` is NOT in the stable set** — the very source this line cites reports it **failing on official hardware**: late Taiwanese-built Atari Jr units, with Thunderground's score corrupting, and a second independent report (omegamatrix, on real hardware) says the same. The one byte and two cycles it saves are not worth a unit-dependent failure. Gate opcode-level code generation on this allow/deny table.
  **Extended 2026-09-04 — the map was right and incomplete.** `definitions.json` carries a
  `stability` field on exactly 8 of 256 opcodes (it is `omitempty`, so an absent field means
  stable): **magic** on `$8B ane` and `$AB lax #imm` — the two this line already names as XAA/LXA —
  and **unstable** on six more that the map never mentioned, all of them *stores*:
  **`$93 sha (zp),Y`, `$9B tas abs,Y`, `$9C shy abs,X`, `$9E shx abs,Y`, `$9F sha abs,Y`,
  `$BB las abs,Y`**. They AND the high byte of the target address into the value, so what they write
  depends on where they write; treat them as denied.
  One sharp edge inside the map's own wording: **`LAX` and `LXA` are the same mnemonic in this
  table.** Seven `lax` entries exist and six carry no stability field; the seventh is `$AB lax
  #imm`, which IS `LXA` and IS magic. "The LAX family is stable" therefore holds for every
  addressing mode **except immediate**, and a generator that reads the family name rather than the
  opcode will emit the one unstable member. `SAX` (4), `SBX` (1) and `DCP` (7) carry no stability
  field in any mode, so the rest of the line stands. Counted from
  `Gopher2600/hardware/cpu/instructions/definitions.json` by the mailing-list distillation
  (helper-1) and re-run here independently. 〔engine instruction table〕
  **A shipped homebrew used `LAX`, and which one is an open question this repository cannot close.**
  Andrew Davie's release note for Qb v0.04, quoted on the list (`200102/msg00205`): *"Mac users
  should recompile the source, exchanging all **"lax"** instructions with **"lda"** — this will give
  a buggered score display, but that's all that will be different."* So the instruction was load
  bearing in released code, and swapping it degraded exactly one thing. **If those were the
  addressed forms the map is confirmed by practice; if any was `$AB` (immediate) the map says magic
  and the note says it shipped anyway, which would be the more interesting outcome.** It cannot be
  settled here: the source was a list attachment (other people's commented assembly — outside the
  clean-room line, not opened) and **no Qb ROM exists in this tree** — checked, 319 `.bin` images
  under `reference/`, none of them Qb. A raw byte scan would not settle it — `$AB` as data
  is indistinguishable from `$AB` as an opcode — but **disassembling from the entry point would**, and
  this tree has the machinery: `Gopher2600/disassembly`'s `FromCartridge`/`bless` follow flow and
  separate code from data, and `cmd/dissect` drives them. **The method exists; the ROM does not.**
  (Corrected the same day: this line first said a byte scan settles nothing and stopped there, which
  understated what is available — helper-1 caught it.) With a `.bin`, this is a question that can be
  closed without opening anyone's source.
  Recorded as an open question with its falsifier named. Found by the distillation (helper-1), who
  declined to quote it as evidence for the same reason. 〔mining 168616 illegal-opcode stability (ASR caveat in the same note); 294471 §32 for the independent second report〕 **Corrected 2026-09-02**: this line previously listed ASR as stable and claimed it was "already used in 48px / dyn_multisprite". Both were wrong — those three ROMs use no illegal opcode at all, and no ROM in the corpus uses ASR/ALR (measured with two structurally different expressions, both exit 1). `scripts/check_traps.py:73` had already omitted ASR from what it recommends, so the docs were the outlier.
- **The resource triangle + a register convention**: RAM (128B) / CPU (76cy) / ROM are mutually exclusive = growing one shrinks the others (plus the human cost). The Thomas Jentzsch convention = inside the kernel, pin the roles to **Y = scanline and sprite index, X = PF, A = everything else** and it runs faster. Use subroutines for code reuse only (the call cost is high). 〔mining 146817〕
- **The canonical kernel vocabulary** (Andrew Davie): "**N-scanline kernel**" (one picture row = N scanlines) plus 4 shape axes = sprite spacing / PF spacing / symmetry (sym/asym) / reflection (mirrored). Adopted as the harness's internal kernel vocabulary. 〔mining 320714〕
- **Movement = fixed-point subpixels**: hold position as 8.8 fixed point and add `vel` every frame → the carry moves the integer part = smooth slow motion, friction, gravity and wind in one framework. **A parabola = constant velocity in X × constant acceleration in Y** (no trigonometry). Enemy chasing = proportional homing from the sign-shift of `(target−pos)/16` (no division; 16 directions = octant + slope threshold — **but a signed shift is not free**: keeping the sign through `(target−pos)/16` costs a `cmp #$80` before each of the four `ror`s, so "no division" means four extra instructions, not none 〔107024:16〕). 〔mining 178177, 270373, 107024〕 (technique-candidate ㉕)
- **×2^n on a small signed value = repeated `asl` (no multiply, sign preserved)**: in two's complement `asl` is exactly ×2, so a signed velocity such as BallDY becomes ×2^n with n `asl`s (e.g. the lookahead target = BallRow + 4×BallDY = two `asl`s + one `adc`). But (a) **the result's range widens → bit7 can no longer serve as the sign test** = do clamp/wrap tests on the value range instead (if the extrapolated target maxes out around ~190 the threshold is `cmp #220`; an application of known-traps' "bit7 clamping is not usable"), and (b) an input that overflows into bit7 during the shift (|value| × 2^n ≥ 128) destroys the sign = check the input range first. 〔in-house: PONG ai-variants v3 lookahead 2026-07〕
- **A BCD score can be compared with `cmp` without decoding**: for a valid packed BCD byte, binary ordering = decimal ordering (the upper nibble dominates) → both `cmp #$11` (first to 11 points) and a ScoreR vs ScoreL comparison are correct as written. But **a binary difference is not a decimal difference** (it inflates by +6 across a digit boundary: $10−$09 = 7) → when the difference is used as a QUANTITY, bucket it with saturation so the coarseness is harmless (v4 rubberband's score difference → error-width modulation). The bit7 sign of a subtraction is valid only while |binary difference| < 128. 〔in-house: PONG ai-variants v4 2026-07〕
- **★TIA revision differences are a trap when cross-checking against hardware**: HMOVE's "extra clock" effect (the Cosmic Ark stars) **reverses behaviour on post-1989 TIAs**, among others — **the same ROM produces a different picture per revision**. The harness's pixel comparison must **pin the TIA revision / emulator** before comparing (and record which revision the verification used). 〔mining 191061 Cosmic Ark stars〕
- **Minimum-byte initialisation + hotspot placement**: in a tight 2K/4K, Omegamatrix's 8-byte self-modifying init (`bne .loop+1` jumps between operator and operand → `#$0A` executes as an ASL) yields A=0 / X=0 / SP=$FF / carry clear. Put bank hotspots **at the highest addresses (near the already-used interrupt vectors)** and the free chunk is maximised (a ZP hotspot = Tigervision 3F saves 1 ROM byte + 1cy per switch). 〔mining blog 12061, 11811〕
- **★The "lodging" pattern for physics lines (sharing a WSYNC line between mutually exclusive paths)**: splitting the Overscan physics into "one concern = one WSYNC line" runs out of lines, but **paths that are mutually exclusive within the same frame (normal / hit / miss / frozen …) may use the same line for different purposes** —— each path strobes line N's WSYNC itself and only the contents of the line are swapped (e.g. line 3 = paddle input normally / english computation on a hit / serve handling on a miss). Work that gets skipped (a frame's worth of paddle input not being applied, say) merely means "drawn with a value one frame old" = an invisible compromise. Keep **the total line count identical on every path** (offset the variable part with the number of filler lines). When a feature addition inflates the budget, first ask "which path is it exclusive with", and consider lodging before adding a dedicated line. Housekeeping that must run every frame (LFSR / counters / note length / switch polling) is safest gathered on **a dedicated line where all paths converge**. 〔in-house: PONG pf2 physics-line architecture 2026-07-02–03 (serve lodging → generalised to hit/miss → new line 5)〕

- **★Placing a row of shapes and WRITING them are different limits, and the writes bind first.** A line's
  placement capacity is a search over strobe cycles (`plan_sprite_placement`); its write capacity is the
  graphics stores that must fit in the same 76 cycles (`prove_line_budget`). They are not the same number and
  the second is smaller, so "the row fits" answered from placement alone is answered from the wrong half.
  Measured on a ten-slot row at a uniform 16 px pitch: **one scanline can PLACE all ten and can WRITE only
  eight**, at both shape widths tried, best schedule ending at cycle **73 of 76**. That is what forces a second
  line — and a shape drawn on one of two lines is lit on every other scanline, so **a striped look is a
  consequence of the budget, not a style**. Two more in the same "cost, not taste" direction: the two lines
  afford **12 shape-draws at 7 px and 13 at 6 px, while one solid word costs 20**, so a row mixing two solid
  shapes with eight striped ones is not an arrangement the budget offers; and **the phase is free** — every
  arrangement that schedules at a given width ends its heavier line on the same cycle, whether the split is
  5/5 or 4/6. **Ask placement and cycles separately and report them separately: a combined "no" cannot tell
  you which of the two said it.** 〔measured 2026-08-26 in a piece in the private `roms/` repository;
  **not re-measured here** — the solvers behind it are bound to that work's own kernel, and the reusable form
  is on `techniques/roadmap.md` waiting for a second caller〕

## Rules of thumb for "good graphics"
- Visual impact ≈ number of colours × sprite density. More colours are bought by adding hardware (Pitfall II = DPC). 〔Demon Attack, Stay Frosty/Draconian〕
- The exemplars = the AtariAge Homebrew Awards "Best Graphics" category. **The strongest ground truth = the homebrew "Pizza Boy", every pixel of which the user drew personally** (designed in Photoshop; constraints confirmed with DaveC). More accurate than mining external threads = put the design questions (colour bands / NUSIZ / flicker tolerance) to the author directly.
- **Endorsement from a real production (the Pizza Boy dissection)**: professional-grade visuals were achieved **by craft on top of a STANDARD kernel** (batari Basic multisprite = 5 moving objects, P1 flickersort + P0 + M0/M1/BL + a 6-digit score). Not exotic code tricks — what works is **role separation (buildings = static asymmetric PF / moving things = sprites) + window rhythm (alternating solid and window across PF rows = vertical window texture) + colour and density design**. → a real production endorses TIA Studio's premise that "the designer composes the screen on top of a standard kernel". Details `reference/pizza-boy/dissection.ja.md` 〔Pizza Boy, bB multisprite kernel〕
- Verify feasibility with a mockup before building (colour budget + scanline count + multiplexing on paper).
  **A mockup only checks the constraints you believed when you drew it.** Kurt Woloch, stella-list
  `199805/msg00187` era post in `new-members`, on the 2600 conversions he drew in 1984: *"I tried to
  figure out what the capabilities of graphics and sound were, roughly, and did some drawings …
  However, I did MISUNDERSTAND some of the constraints. I thought it would be allowed to have four
  colors on one scanline of playfield if you reduced the vertical resolution to double-scanline …
  which, I'm afraid, ISN'T POSSIBLE THIS WAY."* **The thing he got wrong was the colour budget** —
  the very item this line tells you to check — and the drawings looked good the whole way through.
  A mockup cannot catch a rule you do not know you are breaking, so the artist's constraints are only
  as good as whoever supplied them; get the budget from a measured page (`docs/techniques/`,
  `verified-coverage.md`), never from memory. *(And "trade vertical resolution for playfield colours"
  is not merely wrong here, it is plausible — swapping resolution for something else is true
  elsewhere on this machine. `known-traps.md` had nothing on it; see there.)*

## Drawing craft (making the sprite/character pictures = the concrete rules of ⑥craft)
- **Start from thumbnail legibility**: verify **first** that it is still identifiable when shrunk to about one dot, then add detail. Shrink without interpolation (nearest, halving each step). 〔326595, 106110〕
- **A 2600 pixel is WIDE — one pixel covers about twice as much width as it does height, so a shape needs about HALF as many pixels across as it does down**: do not trust a square-dot preview. Decide letterforms and pictures at the real hardware aspect (player = thin out 1px horizontally, PF = 3–4× vertically to buy density). **→ draw previews with non-square pixels.** 〔326595〕 (there is no constant for this — see the note below)
  <!-- The Japanese original read 「横 ≒ 縦の約 1/2・≈2:1」 and looked self-contradictory in translation, because its two halves count different things: 「横 ≒ 縦の約 1/2」 is about how many PIXELS a shape needs across versus down, while 「≈2:1」 is the aspect of ONE pixel. Both say the same thing — the pixel is wide — and the English above now states it once. Resolved 2026-08-04. -->
  - **⚠★ WHY THE SOURCES DISAGREE, and what "measure it" can and cannot settle (2026-08-04).** The spread
    1.67–1.82 is not measurement noise, it is the question being underspecified. A 2600 pixel's aspect is
    `(visible width / visible height)` divided by the display's own `4:3`, and **the visible height is the
    free variable**: 192 lines of a 262-line frame is not the same picture as 210 or 228, and every source
    picked a different one. 5:3 (1.67), 12:7 (1.71) and 20:11 (1.82) are the same physics with three
    different overscan assumptions. **A fourth datum, 2026-09-04, falls BELOW the range and widens it to
    1.60–1.82** 〔stella-list `paint-tool-for-screen-mock-ups`, 2001-10, Erik Mooney〕: *"The 2600's
    160 x 192 is actually at an aspect ratio of 5:8 horz:vert (**an object 8 pixels high and 5 pixels wide
    will be visually square**), and the 40 x 192 is 5:32."* That is 8:5 = **1.60**, and it is the first of
    these to come from the mailing list rather than AtariAge — a fourth independent overscan assumption.
    **This strengthens the decision below rather than weakening it**: a spread that grows as sources
    accumulate is not converging, which is what "underspecified" predicts.
    **A fifth datum breaks the range at the TOP, on a different axis, 2026-09-04.** Eric Ball, 2004:
    *"For NTSC **160x200** is very close to 4:3, for PAL **160x240** is very close to 4:3."* PAL's
    240 gives **2.00** — above everything above — and the reason is not another overscan guess, it is
    **the television standard**. So the spread now has *two* free variables, not one.
    **And PAL immediately disagrees with itself, which is the point.** Atari's own table
    (`reference/docs_atari/stella_programmers_guide.html`, quoted in `fundamentals-audit`) gives PAL a
    **228-line kernel**, not 240 — so the vendor's recommendation is **1.90** while filling the frame
    is **2.00**. Same standard, same free variable, two answers, from the manufacturer and from a
    2004 practitioner. A 45° line reads as **27.8°** under one and **26.6°** under the other.
    **And all five values come out of one expression**, which is worth stating plainly because it
    makes the disagreement legible rather than mysterious:
    `pixel aspect = (4:3) ÷ (160 ÷ visible lines) = visible_lines / 120`.
    192 → 1.600, 200 → 1.667, 205 → 1.708, 218 → 1.817, 240 → 2.000 — reproducing 8:5, 5:3, 12:7 and
    20:11 to three places. Every source is the same physics; each picked a different line count, and
    PAL picks a different one again.
    **The consequence is not only shape, it is angle.** A line drawn at 45° on a square-pixel canvas
    reads as `atan(1/aspect)`: **32° at 1.60, 31° at 1.67, 27° at 2.00**. Eric Ball arrived from
    exactly that symptom — *"when I try doing 16 degrees of movement assuming 1 pixel per frame
    up/down and 1 pixel per frame left/right, **the 45° diagonals don't seem quite right**."*
    **Do not confuse this with the diagonal correction already in this file.** Combat's frame gating
    (`MPace & $03`, move on 3 of 4 frames) corrects **√2 — the distance travelled diagonally** — and
    is needed on any display. The aspect correction is about **the angle the eye sees**, and is
    needed because a 2600 pixel is wide. Two different corrections; this repository has the first and,
    within the searches run, not the second. Found by the mailing-list distillation (helper-2), who
    flagged that the 2004 quote is a description they had not verified and the arithmetic is theirs —
    re-run here and matching. Mooney's phrasing is also the
    most useful one for an artist — **8 tall × 5 wide reads as a square** — so it is the form to hand to
    whoever is drawing (see `docs/ingest.md`). **The sources name two more free variables this line left implicit** 〔found 2026-09-03〕: an NTSC display expects **227.5 colour clocks per line and the 2600 emits 228** 〔169128:12〕, so the horizontal scale is already off a standards-conforming set by half a clock per line; and the 2600 is **240p progressive — the even field of 480i, but at full refresh rather than half** 〔208810:9〕, so "how many visible lines" asks about a signal with no interlaced partner to average against. Neither number lives anywhere else in this tree (227.5: five layers, zero hits; every 228 here is cycle budget, a different quantity). **So no single Stella measurement settles it either** — it settles what
    STELLA assumes, which is one more source, not an arbiter. (Checked 2026-08-04: Stella 7.0's `-help`
    offers no aspect-correction option at all — only `-tia.vsizeadjust <-5..5>` — so the "Stella 91%" figure
    cited above does not correspond to anything in the current build.)
    **HOW THIS WAS SETTLED (2026-08-04): the constant was DELETED, not corrected.** `pkg/design` carried
    `PixelAspectRatio = 2` and `ScanlinesForSquare(w) = w*2`, and 2.0 is above the entire 1.67–1.82 range, so
    it was wrong under every assumption. But it had **no caller anywhere** — not in the harness, not in the
    umbrella `sandbox/` tree holding the 54 authored PONG sources and the Pizza Boy reproduction. The only
    references were its own definition and a test asserting that definition. Dead code carrying a wrong
    constant is worse than no code, because the next reader trusts it.
    **The measurement is the part worth keeping, and it is the three derivations above.** The author draws in
    Photoshop on a 1:2 grid and has decided not to chase the remaining ~16%, which on a 2600 sprite is one dot
    either way — and real CRTs vary by more than the gap between 1.67 and 1.82.
  - **⚠ The precise value must be measured (the codified 2:1 is too large)**: several sources agree that "one 2600 px is wide" but **they disagree on the value** = 5:3 ≈ 1.67 (190154, 172161, 334673) / 12:7 ≈ 1.71 (169128) / 20:11 ≈ 1.82 (208810, Stella 91%). **The code used to carry `design.PixelAspectRatio = 2`, larger than every source and therefore too large under all of them; it was deleted on 2026-08-04 rather than corrected, because nothing called it** (see the note above). **The right answer splits over 1.67–1.82 depending on the display assumption**, and no single Stella measurement arbitrates that — it would settle what Stella assumes, which is one more source ([[feedback-verification-standard]]). Colour is likewise not RGB = the Stella palette is the comparison standard (306508, 300805). 〔mining 190154, 169128, 208810, 172161〕
- **★The canonical image→title route (a professional's real workflow)**: SpiceWare builds **the Photoshop mock FIRST and the kernel after it**. Logos and titles use a **flicker-free 2-colour 48px kernel** to turn "a designed 48px image" into "a stable on-screen display" (SF2 is the real example). = exactly this project's Photoshop→2600 path. `multicolor48`/`bitmap48` are its implementation basis. 〔mining blog SpiceWare 10640, 10515〕
  - **Mid-scanline colours sit on a 3CC grid, but a band is as wide as the STORE that paints it**:
    3 colour clocks is the CPU's granularity — one cycle — and it is not the band width. A band costs a
    whole store, so the floor is `writeCycles × 3` px: 9 px for `STA zp`, 12 for `STA abs`, 18 for a
    six-cycle write (`design.MinColorBandWidthPx`, and `color_test.go` pins all three). 160 ÷ 9 ≈ 18 is
    where the band count comes from — not 160 ÷ 3 ≈ 53. **Corrected 2026-09-03:** the sentence read as
    if the 3CC grid produced the ~18, which is a factor of three out; the code was right all along.
    Horizontal multi-colour tops out at ~18 bands / 3 colours (4 by borrowing SAX). Four arbitrary colours are impossible = substitute holes plus stacking. The SCORE bit (CTRLPF D1) splits the PF left/right. 〔mining 190154〕 `→ design.MinColorBandWidthPx, ScoreModeTwoColor`
- **Kill the misread letter pairs**: L/I/T · U/W · M/H/N · O/0/D. An author cannot notice their own misreadings → **verify with another person or by reading aloud**; the final adjustment is single-pixel. 〔294306, 326595 (confirmed twice = a strong principle)〕
- **In 8px monochrome, spend the entire budget on the silhouette**: concentrate on the single most identifying part (hat, moustache, etc.). If that is not enough, buy density with double width + venetian stripes. 〔106110〕
- **A walk cycle needs a minimum of 2 frames at 50:50**: one bit of the frame counter (`and #2^n`) gives even spacing with no reset, and runs **only while moving**. 〔301861〕 `→ design.WalkFrame`
- **Landscape gradients hold one hue and step only the luminance** (never mix hues). Depth from two layers: BG = far, PF = near. 〔160655〕 (consistent with the colour section's "high luminance → low saturation" rule)
- **Decide background art on 4 axes up front**: width (48/96px), colour count (1/2), PF symmetry (reflected/asymmetric), row height (1–16 lines per row = detail vs load). **These ARE the input parameters of the background template (`design.BackgroundSpec`)**. 〔319884 atari-background-builder (= the tool the user used on Pizza Boy)〕 `→ design.BackgroundSpec.Feasible`

## Judgement rules no machine can decide (doc-only — deliberately not landed in `pkg/design`)
These cannot be quantified and need a judgement from Claude, a person, or an image, so they are deliberately left uncoded and collected here (= every rule is given a disposition, which is what guarantees coverage).
- **Thumbnail legibility**: whether it is still identifiable when shrunk = needs an image and a human eye. Judge from `get_screen_annotated`'s reduced preview. 〔326595, 106110〕
- **Misread letter pairs** (L/I/T · U/W · M/H/N · O/0/D): an author cannot notice their own misreadings = verify with another person or by reading aloud. Hard to mechanise. 〔294306, 326595〕
- **In 8px monochrome, spend the entire budget on the silhouette**: which part carries the identity is a subject-dependent aesthetic judgement. 〔106110〕
- **The role split missile/ball = lines, player = areas**: which stack of objects builds one apparent shape is a composition judgement. 〔Davie〕
- **GameState = one variable + a kernel per state**: a structural pattern, not a numeric test. 〔title-to-game-transition〕
- **The ISC/ISB illegal opcodes + borrowing SP as a line counter**: a cycle-saving trick. Whether it is usable is backed by litmus measurement (guaranteed by verification, not by code). 〔illegal-opcodes〕
- **Symbolic naming / the two PAL-NTSC sets (N_xx/P_xx)**: a convention for how colour is held. `design.Hue/Luminance` can decompose the value, but the practice of "holding it under a symbolic name" is a convention, not something to check. 〔symbolic-color-names〕
- **Tool-implementation knowledge (the spritemate data model, implementing a per-scanline colour UI, and so on) is NOT absorbed**: it does not help authoring (writing asm). Preserving it in the frozen `tia-studio/` repo and the research notes is enough.

## Landing this in the implementation (`pkg/design` / the frozen TIA Studio)
- Feasibility = `pkg/design`'s static estimate plus live assert_line_budget / read_cycles / calibrate answers "does this layout fit inside 76cy?" immediately. Used as the gate before Claude writes asm.
- The defaults for the 4 feasibility axes (colour / scanlines / multiplexing / budget) are detailed at the end of `tools/research-w2-design.md`.
- The templates map onto the verified kernel techniques (zone_multiplex / dyn_multisprite / score6 / bitmap48 / two_line_kernel …).
- Note: TIA Studio (the canvas editor) is **frozen** ([[project-pivot-author-not-tool]]). These dimensions and judgements were originally aimed at its M4, but the main consumer now is Claude's authoring loop. The template set can be reused if it is revived.

## Structure & efficiency rules from the Combat (1977) disassembly comparison
Distilled from an efficiency/structure comparison of a self-authored Combat clone (`combat_mine`, 4K) vs the original Wagner 2K ROM (`sandbox/studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`). Clean-room: generalized prose + routine names only. These are **integration-under-budget** rules — how the original fits a whole 27-variant game in 2K.

- **Move ALL objects through ONE `,X`-indexed path over a bearings/state array — do NOT inline per object**: hold each object's dir/vel/pos in parallel arrays indexed by object (Combat's `DIRECTN[0..3]` drives both tanks AND both missiles down one `,X` loop with a 24-byte `MVtable`). The clone inlined friction+accel 4× (P0/P1 × X/Y ≈ +120–200 B of pure duplication). Decisive point: **movement runs in blanked overscan, so the 76cy/line budget does not apply — an index costs nothing off-beam, so the indexed loop is BOTH smaller AND free.** Before adding a per-object copy of any motion code, ask whether one indexed pass over an array does it. 〔Combat `DIRECTN`/`MVtable` — one `,X` path for 4 objects; comparison §2.4/§4/§7, diff-gaps GAP-3〕
- **Momentum = time-sliced increments, not a fractional velocity**: as an alternative to `pos += vel/frac`, dither the velocity across time. `FwdTimer` ($F0→$00, 16 steps) `ROL`s two 8-bit halves (`MVadjA`/`MVadjB`); the emerging bit nudges `XoffBase` by $10 for that one frame → faint analog acceleration over 16 frames, **no multiply**. Diagonal isotropy = **frame gating** (`MPace & $03` moves on 3 of 4 frames), not a √2 fraction (cheaper, VCS-idiomatic). A plain subpixel integrator moves correctly but can't reproduce that "faint inertia" texture — reach for time-slicing when the *feel* matters. 〔Combat `FwdTimer`/`MVadjA`/`MVadjB`/`MPace`; diff-gaps GAP-3, comparison §2.4〕
- **Rotation sprite = precompute the shape into a RAM buffer so the kernel reads a bare `LDA abs,Y` (zero per-line rotation math)**: store only **180° of shapes in ROM**; synthesize the other 180° as a **point-rotation = `REFP` hardware H-flip + a reverse-order byte copy (software V-flip)**, rendered in VBLANK into a RAM shape buffer; re-render only **one object per frame** (30 Hz each) to bound the VBLANK cost. General pattern: *don't compute in the kernel; stage the shape in VBLANK*; the table needs only 180° (symmetry supplies the rest). 〔Combat `ROT`/`SHAPES`+`REFP0/1`+reverse copy → 16B HIRES RAM; diff-gaps GAP-5, comparison §2.2〕
- **One interleaved HIRES buffer can feed BOTH players (P0 = even bytes / P1 = odd)**: a single 16-byte RAM buffer serves both sprites — pick a player's bytes with `AND #$FE` / `ORA #$01`, no shape math. Halves the RAM vs two separate buffers (~16 B) = a RAM-thrift move to hold in reserve for when 128 B is tight. 〔Combat shared 16B HIRES, P0/P1 interleaved; comparison §2.1/§2.2/§7〕
- **Fan one byte out to many duties, phase-locked, when RAM is tight**: `CLOCK` serves **5 roles** (frame timer / attract color / debounce pace / score-flash clock …) and `GameTimer` serves **3** (match clock + bit7 in-progress flag + attract period), sub-fields phase-locked so their uses never collide. Master-class RAM economy — but **only pay this when RAM is actually scarce**: packing with 43 B free just spends clarity for nothing (premature optimization). Know it; deploy it only under pressure. 〔Combat `CLOCK` (5-duty) / `GameTimer` (3-duty) / `VCNTRL`; comparison §2.7/§7〕
- **Load-level VBLANK with `TIM64T`/`INTIM` so the picture starts at a FIXED beam position — don't rely on a fixed WSYNC count + elastic filler**: arm a RIOT timer at VBLANK start, spin on `INTIM` until it expires, then begin the visible kernel = display-start **independent of how long the frame's logic ran**. A fixed WSYNC count + elastic `VBpad` tuned to today's code does NOT auto-absorb logic growth: add work and the picture dips (screen dip — the exact fragility the clone's positioner had to hand-engineer around). Prefer timer load-leveling when VBLANK work is variable or expected to grow. 〔Combat `VCNTRL`/`INTIM`/`TIM64T`; comparison §2.1/§6, diff-gaps (measure the VBLANK length with INTIM)〕 `→ techniques/sound-driver.md · game-states.md`
- **One wrap-around clear loop, reused with 4 seed values for 4 clear extents**: `ClearMem` is a single loop whose start index (X seed) is set 4 ways to wipe 4 regions — one routine, four callers, vs four clear loops. Cheap ROM-thrift for init/reset paths that wipe several ranges. 〔Combat `ClearMem`; comparison §2.8/§7〕
- **Audit your OWN hand-tuned code for cargo-cult — hand-tuned ≠ optimal, even in a 2K master ROM**: the annotated Combat disassembly honestly inventories its own cruft (a redundant double `STA GRP0`, a stray `WSYNC`, a self-flagged "why not `LDA MVtable+1,Y`?" 2-cycle miss). Model this: keep a written inventory of your ROM's own redundancy rather than assuming your tuned code is tight. (Applied to our clone, this surfaced ~250–400 B of recoverable duplication unrelated to its provability trade.) 〔Combat — Williams' annotations; comparison §7〕

## Combat deep-read: design-intent, audio model & AI-nav primitives
A second pass over Combat (1977) through 5 lenses BEYOND round-1's efficiency/structure comparison — design intent, the audio channel model, and AI-nav primitives our own clone added (the original has no AI). Clean-room: generalized prose + labels only. Where round-1 gave **integration-under-budget** rules, these are the **why / feel / balance** rules the structural pass could not see. The AI-nav block is flagged **PONG-capstone material**.

- **Difficulty = a per-player self-handicap on the WINNER, not AI scaling — and the lever means something different per vehicle.** Each player reads his OWN difficulty switch. "Pro" nerfs the strong player two ways: (1) shorter missile RANGE (early-killed at a higher remaining-life threshold), and (2) a SLOWER vehicle (subtract the vehicle-index off velocity — a jet loses speed while a tank loses none; tanks only lose range). Intent = a self-selected handicap so parent/child or expert/novice play the same 2-player match evenly: nerf the strong player instead of buffing an enemy. 〔Combat `NoStir` (DIFSWCH ASL) / `ChkVM`·`MisEZ` (CMP #$1C early-kill) / `FwdPro` (SBC GAMSHP); manual p.71-77; deep-read harvest 2026-07-23〕
- **Orthogonal mechanic axes buy cheap combinatorial breadth — but CURATE the cross-product and RESKIN shared mechanics with new fiction.** 27 variations = ~6 independent bitfields, each driving one flag, so combos are nearly free. Yet the designers shipped a hand-picked 27, NOT all 2^N (many mechanically-legal combos dropped as unfun), and the SAME playfield bytes are reskinned as "maze/barriers" for tanks vs "clouds" for planes — identical rendering, different fiction and tactics. Content strategy, distinct from the round-1 VARMAP bit-packing fact. 〔Combat `VARMAP` / `InitPF` flag decode / `PLFPNT` maze-vs-cloud reuse; deep-read harvest 2026-07-23〕
- **Stealth where your own useful/risky verbs repaint you.** In invisible variants the tank is painted the background color, but firing, bumping a wall, and scoring/being-hit all RESTORE the visible color — each merely re-writes the color register those events already touch. The hidden-information layer is self-defeating by design: attacking or moving recklessly lights you up = attack-vs-hide tension for free. 〔Combat `ChkVM` (ColorBK paint) / `BumpTank` (XColor restore); manual "Invisible Tank"; deep-read harvest 2026-07-23〕
- **Force composition — how MANY and how BIG each side's units are — is a distinct handicap axis (~2 NUSIZ bytes).** A widths table turns the plane games into 1v1 / 2v2 / 1-vs-3 / one quad-width "Bomber" vs three planes almost for free (formation units all fire on one trigger). Quantity/size asymmetry is a curated difficulty knob separate from stat tuning. 〔Combat `WIDTHS` / `LDSTEL` NUSIZ setup; manual games 19-27; deep-read harvest 2026-07-23〕
- **Input deliberately throttled for "heft" — spend real effort on feel nobody consciously sees.** Three governors make vehicles feel weighty, not twitchy: a turn-rate governor (N frames between each 22.5° rotate), a whipsaw-reversal inhibitor (block instant flips), and forward-speed dithered over 16 frames so momentum is "just barely noticeable." Rate-limit raw input to express a vehicle's mass. (Round-1 has the FwdTimer momentum; new here: the rotational governor + whipsaw inhibitor + the invisible-polish philosophy.) 〔Combat `CHKSW` (TurnTimer/LastTurn) + `FwdTimer`; deep-read harvest 2026-07-23〕
- **A scoring event earns a consequence beat that ALSO resets board geometry (anti-camping).** A hit doesn't just respawn: the loser's tank spins, explosion volume ramps down, the loser is knocked to a NEW position (direction off the winner's missile bearing), and the winner's engine is silenced. Re-orienting and shoving the loser prevents play resuming in the same lethal geometry = anti-instant-re-hit fairness. Give scoring events a consequence beat that also resets state. 〔Combat `COLDET` / `CHKSW` stir branch / `RushTank`·`BumpTank`; deep-read harvest 2026-07-23〕
- **Overload one control with a contextual second meaning (control economy on a 1-button machine).** In guided-missile variants, rotating your body continuously copies your CURRENT bearing into the missile's — so after firing you steer the missile by continuing to turn, no separate control. Trades aim for vulnerability (the same stick turns your body). Depth without extra buttons. 〔Combat `ROT` (BIT GUIDED / STY DIRECTN+2,X); manual Fig E; deep-read harvest 2026-07-23〕
- **Fixed short match + a diegetic end-game telegraph rendered THROUGH the score itself — no separate UI.** A ~2-minute timer ticks ~1/sec; the last ~1/8 is telegraphed by BLINKING the score (no timer widget). Short fixed sessions keep 2-player play snappy; communicate urgent state by animating an element you already draw. 〔Combat `GSGRCK` (GameTimer / CMP #$F0 / CLOCK&$30 flash / KLskip=$0E); deep-read harvest 2026-07-23〕
- **Minimal-UI: attract == menu == play, and the score doubles as the variation selector.** No separate menu — in attract, Select increments the variation number straight into SCORE, shown by the normal score kernel (right score hidden so only the game number reads); the idle match-timer drives a color-cycle anti-burn-in. Reuse gameplay display elements as menu/attract UI. 〔Combat `SelGO` (STA SCORE / SHOWSCR) / `LDSTEL` color cycle; deep-read harvest 2026-07-23〕
- **Rule-layering & productive imprecision as design moves.** (a) Billiard adds a scoring PRECONDITION (must-bounce-first) over the unchanged bounce engine → a bank-shot game with a higher skill ceiling from the same physics. (b) The faked Pong reflection is imprecise ON PURPOSE (guesses the wall normal, jiggers +22.5°) so bounces are never perfectly axis-aligned → livelier, unsolvable. (c) Removing "reverse" from tanks is control-limitation-as-identity. New modes come from preconditions/constraints/omissions, not new systems. 〔Combat `Launch`/`COLIS` billiard gate / `COLMPF` reflection SM / `CTRLTBL` "No reverse"; deep-read harvest 2026-07-23〕
- **★SOUND PRIORITY = last-writer-wins on a 1-object-per-channel bus — arbitration is BRANCH ORDER, not a mixer.** The core 2600 audio mental model (only 2 channels). Each object owns one channel; precedence (explosion > shot-boom > engine > pong) is decided purely by which routine writes `AUDx0,X` LAST, via the branch order of the sound dispatch. A state flag can "steal" a channel (nonzero → emit the bounce tone INSTEAD of engine). Decide precedence by ORDERING writes, not comparing volumes — zero bytes of priority logic. 〔Combat MisLife dispatch (MisFly/MotMis/BoomSnd order) / `MOTORS` AltSnd hijack; deep-read harvest 2026-07-23〕
- **★Live game data overlaid on the CPU IRQ/BRK vector slot = a 2K→4K port booby-trap.** A 2-byte pitch table sits at the IRQ/BRK vector address because a 2K cart mirrors $F000-$F7FF into $F800-$FFFF, so the "vector" bytes ARE read as an ordinary data table (`LDA table,X`). It survives only because the code SEIs and never takes BRK. A 2K→4K port silently breaks. Never overlay meaningful data on $FFFA-$FFFF unless you fully model the bank mirror. (harness-warn candidate → capgap CMB-6.) 〔Combat ORG $F7FC / AudPitch $0F,$11 at $F7FE; deep-read harvest 2026-07-23〕

**AI-nav primitives (PONG-capstone material — from our Combat clone `combat_mine.asm`; the original 2-player Combat has NO AI, so these are authored-new and emulator-verified):**
- **8-way octant seek from unsigned |dx|,|dy| + sign-first, with a >127px overflow guard.** Derive each axis's sign FIRST by unsigned CMP (a signed 8-bit subtract of two positions overflows once separation exceeds 127px: dx=-132 reads as +124), then form |dx|,|dy| by large-minus-small, then classify: 2·|dy|<|dx| → horizontal / 2·|dx|<|dy| → vertical / else diagonal (the ×2 via ASL+carry also handles 2·|d|>255). Picks one of 8 headings, division-free and overflow-proof on a 160px field. 〔clone `AiDxE`/`AiDyC`/`AiCls`; deep-read harvest 2026-07-23〕
- **Shortest-arc turn on a power-of-two direction ring.** To rotate toward a target heading the short way on a 16-step wrap ring: diff = (target − current) & $0F; if 0 done; CMP #9 → 1..8 turn CW (INC), 9..15 turn CCW (DEC). One compare picks the correct rotation sense across the wrap with no signed distance and no table. Generalizes via CMP #(N/2+1); rate-limit the turn. 〔clone `AiMove`/`AiCW`/`AiTe`; deep-read harvest 2026-07-23〕
- **Map-free navigation primitives (four independently-testable behaviors on a bare greedy seeker).** (1) Stall→180° reversal: every 32 frames sample horizontal headway; |Δ|<2px = wedged → about-face (gate OFF where an axis is intentionally frozen, else a legit vertical climb reads as "stuck"). (2) Reactive wall-slide: on wall-contact (CXP1FB) skip accel + rotate one notch + snap velocity to zero, so the heading sweeps off the wall — no normal, no map. (3) Ammo-gate fire-when-aligned: fire only when off-axis error < half a tank height; a hold sets a quick re-check WITHOUT charging the post-shot cooldown (hold ≠ fired). (4) Scatter-decoy target-swap: inside a concave pocket substitute hard-coded exit waypoints for the target (seek core unchanged) with the escape direction LATCHED against mid-corridor oscillation. Grow AI as named, separately-verifiable layers over "walk toward target." 〔clone `AiStk`/`P1Snap`/`AimOK`/`TgtEsc`; deep-read harvest 2026-07-23〕
