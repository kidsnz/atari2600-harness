# Changelog

The change history of this project. Format follows [Keep a Changelog](https://keepachangelog.com/);
versions follow [Semantic Versioning](https://semver.org/).

> Entries from v0.17.0 and earlier are condensed; the full detailed history (in Japanese) is kept locally
> in `CHANGELOG.ja.md`.

## [Unreleased]

### Fixed
- **BCC counts UP, so the divide bound used the wrong variable — and this closes the nine unsound bounds
  the `determineBound` audit found.** `sbc #N / bcs` loops while A >= N and A falls, so a larger entry value
  means more iterations and `amax` bounds it. `sbc #N / bcc` loops while A < N and the subtraction **wraps**,
  so A rises by (255−N) until it reaches N — a larger entry value means FEWER iterations and `amax` bounds
  nothing at all. One formula was applied to both; it agrees only while N is small. Measured on the new
  `litmus_bccdiv.asm` at N=200: **proven 16, machine 31 — 1.9x under**, with `certified: true`. The BCC
  bound is `ceil(N/(255−N)) + 2`, and N=255 is refused rather than bounded, since 255−255 = 0 leaves A where
  it was and the loop never ends. Corpus effect over 155 images: 2 bounds raised (both the fixture's), 0
  lost, 0 lowered — all 18 divide folds in the corpus are BCS, which is the only reason none was wrong.
  **Two things generalise from the seven repairs**: four of them *deleted a divergence* rather than adding a
  rule (`transfer` vs `absSuccessors` twice, the fall-through filter vs `successors`, and one formula split
  in two where the machine always had two behaviours); and every census that cleared a defect was accurate
  about what it counted and wrong about the exposure.

### Added
- **`visual_ceiling` / `cmd/ceiling` / `internal/ceiling` — a denominator for a picture.** `vismatch`
  compares a build against another ROM, so a wrong picture could not be separated into "the kernel is
  wrong" and "the hardware cannot do this". The ceiling supplies the missing half: the best any 2600 kernel
  could reach for a target frame under a **stated constraint set**. A ceiling is a property of *(image,
  constraint set)*, never of an image, so the output is a **ladder** and the **deltas are the
  deliverable** — C1 playfield-only / C2 + one 8-clock object / C3 no column grid. Measured on five
  commercial frames the grid costs 7.09 rmse on Barnstorming and 8.58 on Vanguard against 3.13 on Chopper
  Command; one sprite is worth 8.88 on Pressure Cooker. C1/C3 are exhaustive over all 8256 colour-pair
  cases per line (true optima, not heuristics that could understate the machine), C2 exact by
  branch-and-bound, ~20 ms a frame. **The palette is derived from the renderer, never transcribed** —
  `PaletteFor` calls the same `GetColor` that paints each pixel, and a test proves that table equals what
  `litmus_palette.bin` actually draws on all 128 entries. That was the trap: the prototype read 9.95 on a
  frame achievable by construction because it used Stella's palette on Gopher2600 frames. Self-test: 5
  in-tree playfield-only ROMs score C1 **exactly 0**; both directions checked (sprite frames 23.06–40.92);
  planted wrong palettes break it on 5 of 5. **Limitation stated rather than implied: no rung emits a
  cartridge**, so none is validated against the 76-cycle budget — C1 rests on one prototype demonstration
  (66 cycles certified, 0/29440 pixels differing), C2 has none, C3 is unreachable by design.
  `docs/visual-ceiling.md`.

### Fixed
- **SD-9's address proxy was still live on the divide path, and nine real folds were resting on it.** The
  BCS/BCC path found A's entry bound with textual fall-through plus address order — the heuristic SD-9
  deleted from the dex/dey path — with an `lda #imm` guess behind it. Measured on the new
  `litmus_divpred.asm`, all three with `certified: true`: a predecessor arriving by `jmp` was invisible
  (**27 vs 87**), the proxy answered when nothing was adjacent (**28 vs 87**), and a `jmp` merely sitting
  before the header was read as a predecessor (**29 vs 89**). **The proxy's "0 uses" counter was a fact
  about eight hand-listed ROMs**: across the corpus nine folds were bounded by it, and it reads `lda #80`
  while ignoring the `adc #XCAL` two instructions later — 7 iterations where the sound bound is 19. They
  sat above the machine by luck.
  Removing it exposed the precision gap that had made it necessary: **`adcRange` returned Top on wrap**,
  and `XCAL = -5` assembles to `$FB`, so `lda #80 / clc / adc #XCAL` computes 331 and gave up. A wrapped
  sum is still a byte, so `[0,255]` is true and *useful* where Top is true and useless. Over 155 images:
  **0 bounds lost, 0 lowered, 12 raised** — the nine go from 53-63 to 118, from resting on an ignored
  instruction to proven. Gate green on 1243 regions across 158 ROMs. The scan is now the dex/dey path's:
  ask `absSuccessors` which edges reach the header and read A from the edge's state — deleting a
  divergence rather than adding a rule, for the third time in this function.

- **A `jsr` inside a folded loop body was costed at six cycles, dropping the callee once per iteration —
  and the tree contains a live instance.** `IsBranch()` is `Relative && Flow`, so a JSR (Absolute,
  Subroutine) and a JMP (Absolute, Flow) both sailed through the body walk. Measured on the new
  `litmus_callinloop.asm` with a twelve-`nop` callee: **proven 48, machine 168 across 3 scanlines — 3.5x
  under**, with `certified: true`. **The worse case is not the arithmetic**: a callee containing
  `sta WSYNC` makes the walk step over a REGION BOUNDARY, so the machine's interval ends at that strobe and
  the proof's does not. `roms/techniques/shared_setxpos.asm` $F054 is that shape — `jsr SetXPos` into a
  routine whose second instruction is `sta WSYNC` — and read **proven 98 against a machine 36**. Neither
  number was wrong; they were about different spans of time, and the 62-cycle "slack" was never slack.
  That is a **third** way for a bound to be wrong alongside too-low and too-high — *about the wrong
  interval* — and `observed <= proven` cannot detect it, because both readings pass while measuring
  different things. Corpus effect over 155 images: 2 folds lost (the fixture's own, and $F054 which was
  already over budget), zero bounds lowered, no certification lost.

- **A loop entered PAST its header carried a counter value nobody had scanned.** `determineBound` maximises
  the entry value over the predecessors of the header, which is the right set only if every execution
  reaching the back edge passed through the header — and nothing anywhere stated that premise. Measured on
  the new `litmus_midentry.asm`, where a `jmp` lands one instruction past the header with X=$50 while the
  header's only scanned predecessor loads X=2: **proven 40 cycles against a machine that spends 733 across
  10 scanlines. 18.3x under**, with `certified: true`. The guard collects the body's sites during the walk
  that was already happening and refuses an edge from outside into any of them **other than the header** —
  the exclusion is the whole subtlety, since several predecessors of the header are sound (the scan sees
  them all and takes the maximum). A guard keyed on "more than one predecessor" would pass the danger case
  and refuse a common shape; the fixture's control row proves it, and with the header included in the check
  both controls fail. **Precision cost over 155 images: zero** — the only fold lost is the fixture's own.

- **A loop counter's entry value now comes from the EDGE into the header, not from the instruction's own
  effect — and the two were computed by different functions in the same package.** `determineBound` used
  `State.transfer`, which models what an instruction does to the machine; for a JSR that is only the push,
  leaving X and Y at their **pre-call** values. `absSuccessors`, which defines what flows along each edge,
  correctly resets a JSR's return point to Top because the callee is not modelled. Measured on the new
  `litmus_jsrentry.asm` — `ldx #$02 / jsr SetBig` where the callee does `ldx #$50` — the scan saw X=2 and
  answered **36 cycles against a machine that spent 738 across 10 scanlines. 20.5x under**, with
  `certified: true`. The repair reads the edge state, which **deletes the divergence instead of adding a
  rule**: a JSR predecessor now yields `X.Top` and the existing unknown-entry refusal fires. **Precision
  cost over 155 images: zero** — the only folds lost are the fixture's own, since no corpus ROM has a call
  between a counter's load and its loop. The fixture keeps a control whose callee provably does not touch
  X as an *asserted refusal*, so the missing callee summary is a measured gap rather than an unexamined
  side effect; the test names it as the row that should become bounded if one is ever added.

- **SD-11 closed: a `dex; bne` counter that can enter at ZERO wraps for 256 iterations, and the proof was
  answering with the range's upper bound.** The trip count is `v` for v > 0 and **256** for v = 0, so it is
  not monotone and `Hi` is not the maximum when zero is reachable. Measured on the new
  `litmus_bnezero.asm`, whose join gives the header X in `[0,5]` and whose machine takes the zero arm:
  **proven 60, machine 2319 across 31 scanlines — 38.7x under**, with `certified: true` and
  `roll_free: true`. The repair returns 256 rather than refusing, so the region stays bounded and the
  author gets a number instead of silence; verified against the machine at **2319 == 2319**.
  **Why it was left alone last time is the interesting part.** When the `bpl` sibling was fixed this hazard
  was censused over the five commercial cartridges the gate then graded — 3 instances, none violating — and
  filed rather than fixed, on the stated principle that a change without a witness is how the previous bug
  got in. That reasoning was right and the corpus was wrong: re-censused after the gate stopped grading a
  hand-picked five, it is **14 folds across three shipped cartridges** (Seaquest x3, Bermuda Triangle x6,
  Vanguard x5, all `[0,15]`). Only the denominator changed. Corpus effect over **155 images**: 15 bounds
  raised, **0 lowered, 0 lost**, and all 14 pre-existing ones were already over budget, so no certification
  was lost. Controls: a joined range of `[3,5]` must stay exact (the fix must not fire on any join) and a
  plain `ldx #5` countdown must stay exact (it must not fire on any BNE) — without the latter, a blanket
  repair reports 2315 for a loop the machine finishes in 56.

- **`determineBound` audited on purpose rather than by accident: 9 unsound bounds measured, the largest
  fixed, and the gate was grading a third of the cartridges on disk.** Two of this package's three known
  unsound bounds were in this one function and both were found while investigating something else, so it
  was audited deliberately — 20 premises enumerated, 11 fail unsoundly, 9 probed with a real cartridge.
  Every probe ran (`Count = 12`, none a refusal on dead code) and every one reported `certified: true`:
  counter written in the body **22 vs 2290 (104x)**, BNE range including zero 67 vs 2326, `transfer(JSR)`
  36 vs 738, mid-body entry 40 vs 733, a call in the body 48 vs 168, and four more.
  **Fixed: the 104x.** SD-13 guarded the window *after* the decrement with `preservesZN`; the window
  *before* it was open, and a write there changes the COUNT rather than which flags are read. `writesX` /
  `writesY` now require the counter's register to be written by exactly one instruction, the decrement.
  The fixture caught a bug in the repair itself — keying on "any index register" refuses every loop that
  walks two pointers, and `OtherCtl` (`iny` inside a `dex` loop) failed at once. Corpus effect over **155
  images**: 4 folds lost, **all four already over budget**, so no certification was lost; zero bounds
  lowered.
  **And the gate's corpus was a hand-written list of 5 while 15 images sat in the same directories.** It is
  discovered by glob now — the same repair the scenario runner needed when 38 of 95 scenario files turned
  out to be run by nothing. **5 cartridges / 66 pairs / 1022 regions → 16 / 234 / 1190 across 152 ROMs**,
  still green. Extending it is what turned the counter-write hazard from "zero corpus instances" into a
  real one (`Pressure Cooker $D801`) and tripled the SD-11 count.

- **A loop's latch must read the counter's own flags — it was never checked, and the prover certified a
  region the machine takes 201x longer to run.** `determineBound` derives a trip count from "the counter
  decrements to zero and the branch exits there", which is reasoning about the DECREMENT's Z/N; anything
  writing those flags in between substitutes its own condition. Measured on the new
  `roms/litmus/litmus_latchflags.asm`, whose DangerRow is `ldx #1 / ... / dex / cpx #$02 / bne`: **proven
  19 cycles, machine 3829 across 51 scanlines**, reported with `certified: true` and `roll_free: true`.
  After the decrement X is 0, the compare against 2 clears Z, and X wraps through `$FF` for 255 iterations.
  Two controls in the same ROM isolate the cause and rule out the cheap repair — SafeRow (`nop` instead of
  the `cpx`) and StoreRow (`stx`, which writes memory not flags) both stay **exact at 21 and 47**, and
  demanding the decrement be ADJACENT to the latch would break both. **Why 140 images hid it:** reverse the
  inequality and the bug is an over-approximation, sound by luck — `exerciser.asm $F0C9` is that shape.
  Census: 757 dex/dey folds, **720 adjacent**, and of the 37 with a gap the instructions are `cpx` 19,
  `inx` 7, `adc` 5, `jsr` 5 — all flag-writing, no store anywhere. The fix is a whitelist rather than a
  blacklist because the engine's instruction table records memory effects, not flags, and the safe default
  with no table is to refuse. Corpus effect: **one region changes, the unsound one**; zero bounds lost,
  zero lowered, gate green on 1022 regions across 141 ROMs.

### Added
- **`get_screen_annotated` takes `raw=true` and returns the bare frame — 160 x visible-height, one pixel
  per TIA pixel, no grid, labels, markers or upscale.** The annotated image serves one direction of the
  pixel-art loop: the user points at a coordinate and Claude turns it into registers. The other direction
  runs the opposite way — the user opens the frame in Photoshop and paints dots, and Claude samples the
  file back into `.byte` rows — and that needs the file's pixel grid to BE the machine's. There was no way
  to get one; every screenshot carried annotations, which in that direction are not decoration but foreign
  pixels inside the artwork. Scale is deliberately ignored in this mode rather than applied, because an
  upscaled "raw" image is a file that lies about its own units. It writes to a separate `*_raw.png` so it
  cannot overwrite the annotated file the user keeps open in a reloading previewer. The height is whatever
  the frame actually rendered, read rather than assumed at 192 — measured on the sunset kernel it is 214.

### Fixed
- **The annotated screenshot no longer draws markers for objects that are not on screen.** `Markers` read
  the TIA's position registers and returned all five movable objects unconditionally — but a TIA object is
  a counter, it always HAS a position, and that says nothing about whether it painted anything. On a
  playfield-only kernel the result was **five labelled vertical lines for five objects that drew zero
  pixels**, over an image CLAUDE.md calls the primary channel the user reads a picture through. That makes
  a phantom marker a false statement about the ROM rather than a cosmetic blemish. New `emu.DrawnObjects`
  answers from the frame's own per-pixel attribution buffer — the one `decompose_row` reads — so the
  question "did P0 appear" is settled by looking for P0 in the picture instead of reasoning about GRP0,
  NUSIZ, VDEL and the priority rules and hoping the reasoning matches the hardware. The JSON still lists
  every object, now with `drawn`, because a position is real and sometimes wanted; only the image drops
  them. Verified end to end over MCP: the sunset kernel returns five objects all `drawn:false` and an image
  with no markers, `litmus_pos` returns `P0 drawn:true` and keeps its line. Both directions are tested,
  since a function that returned false for everything would satisfy the playfield case alone and silently
  erase every real sprite. **One test expectation was written wrong and the measurement caught it**: the
  `objsizes` litmus was assumed from its name to cover players, and it does not — it sweeps missile and
  ball widths only. The two independent readings of the buffer agreed with each other and disagreed with
  the author.

### Added
- **`prove_line_budget`, `defuse` and `beam_intervals` now say which build answered — and shout when the
  source has moved since.** A static analysis is a claim about source; the server answering is whatever
  binary the session connected to, and editing the analyser does not change it. Measured 2026-08-01, twice
  in one session and both times on a fix that was already correct: `prove_line_budget` returned worst **74**
  for a kernel the current source proves at **66**, and reported the DAG-first witness ROM as refused when
  the current source bounded it at **26**. Both read as "the change did not work"; the honest response to
  each would have been to revert a correct change. Go already embeds the answer with no build flags — the
  running binary carried `vcs.revision=bb3b0f8` while the tree sat at `30b492d`, four commits later, and
  nothing read it. Stamping alone would not have sufficed, since a stamp only helps a reader who thinks to
  compare, so the server reads HEAD itself and puts a full sentence in the result. It stays SILENT when a
  guess would be wrong (no build revision, unreadable repository), because a false STALE trains a reader to
  ignore the real one; a build from an uncommitted tree gets its own milder note. Note that `version.Harness`
  would NOT have caught this: it read `2.0.0` on both binaries, because the source moved and the release
  number did not.

- **A page-aligned table cannot be crossed, and the proof now says so — measured on a kernel the corpus
  did not contain.** `pagePenalty` reached its conservative `+1` whenever the index range was unprovable,
  which is exactly when a kernel aligns its tables in the first place. The rule that settles it needs no
  index analysis: a 6502 index register holds 0..255, so `$NN00 + idx` is at most `$NNFF` and never leaves
  the base's page. **Across the 135 ROMs in `roms/` this case fires ZERO times**, and that is a fact about
  the corpus rather than the case — 24 of the 31 technique kernels draw no playfield, so not one of them
  is a table-driven picture kernel, and a picture kernel is what aligns tables. The first one written
  produced **8 wasted charges on its first run**: proven worst 74 against a machine that takes 66, two
  cycles of headroom reported where there were ten. Witness `litmus_pagealign.asm`, whose aligned region
  now proves 34 against a measured 34 — equality, because a bound that is merely safe sends an author
  trimming work that was never over budget. Negative controls both ways: removing the shortcut puts the
  aligned region back at 38 vs 34, and widening it to ignore the base's low byte makes the same fixture
  report 44 for an interval the machine takes 48 — caught by the corpus gate, along with
  `litmus_pagecross`. **The fixture's first draft proved nothing and its own premise check said so**: the
  index was written as a constant, and the abstract interpreter tracks RAM well enough to pin it, so all
  four reads took the already-free path. It reads the joystick port now.

- **The prover stops calling a DAG a loop — and the first attempt at it published a bound BELOW the
  machine, which the corpus gate caught.** `foldLoops` decided what a back edge was by ADDRESS ORDER: a
  branch counted as one when its target sat at a lower address and was not a WSYNC sink. That is not
  reachability, and a region whose graph is acyclic was refused as *"multiple back-edges"* for having no
  edges back. Four regions across the corpus are exactly that, all commercial: Seaquest `$F1EC` (proven
  59, machine 53), Chopper Command `$FA78` (74, 72) and `$FAEC` (103, 97 over two scanlines),
  Barnstorming `$F3D4` (95).
  **The order is the whole finding.** Running the longest-path walk FIRST and accepting whenever it met
  no cycle bounded 49 regions — and 45 of them were bypassing legitimate refusals, because `foldLoops`
  refuses for eight reasons and only one describes the graph. The other seven describe the loop BODY,
  and a loop whose body holds a WSYNC is **invisible to the walk**: the WSYNC is a sink, so the walk
  stops there and never traverses the edge back, leaving a subgraph that looks perfectly acyclic.
  VideoOlympics `$F5CA` refuses for *"WSYNC inside loop body"*; the walk answered **148 cycles for an
  interval the machine takes 163**, and `$F61F` did the same at 155. Folding first and overriding only
  the back-edge refusal leaves exactly the 4 intended regions, 0 bounds lowered, 0 lost, and no region
  reclassified. Witness: `litmus_dag_region.asm`, since all four real specimens are cartridges this repo
  does not contain. Negative controls both ways — removing the override fails the witness on its own
  premise check (0 multi-latch regions reached), widening it past the back-edge case fails the invariant
  test (not one body-shape refusal survives in 31 ROMs).
  **The gate caught this because it had been extended hours earlier.** VideoOlympics was only graded at
  all because the commercial images were added to `TestProvenWorstIsNeverExceededOnCorpus` the same day;
  before that the corpus was the corpus we happened to write, and a well-argued soundness claim would
  have shipped an unsound bound.


### Fixed
- **The cycle-budget prover under-approximated every `dex`/`dey` countdown latched by `BPL` by one whole
  iteration — proven 66 where the machine takes 75.** `determineBound` accepted `BNE` and `BPL` as the latch
  of a decrement countdown and returned the same trip count for both. They do not end on the same iteration:
  `dex; bne L` from X=6 leaves when the decrement produces **zero** (6 iterations), while `dex; bpl L` leaves
  only when it produces a **negative** value, so it runs the body once more with X=0 and exits on X=$FF
  (**7 iterations**). The bound was short by one body plus one taken branch, every time. Found on the real
  **Seaquest** cartridge, region **$F1FC** — `sta HMOVE / sta COLUBK / lda #$FF / ldx #$06 / L: sta $B0,x /
  dex / bpl L` between two WSYNCs: **proven 66, machine 75, slack −9**, carried out on `Bounded=true`. A
  proven worst case the hardware **exceeds** is the one answer this package must never give; an author
  reading it would have believed a 75-cycle line had 10 cycles spare. Fix: the trip count of the `bpl` form
  is `best+1`. It is **sound and exact**, not a cushion — `loopCost` is monotone in the iteration count, and
  for the bpl exit condition the count is `v+1` for entry values `v <= 128` and 1 for `v >= 129`, so `best+1`
  bounds it everywhere and equals it for the loops an author actually writes.
- **Why the standing corpus gate never saw it, measured rather than guessed.**
  `TestProvenWorstIsNeverExceededOnCorpus` globbed only `roms/{techniques,litmus,exerciser}` — the corpus we
  happened to write. Censused over the 140 images analysed: **7 `bpl` folds, only 4 of them in our own
  kernels, and not one of the 4 could expose it.** `rts_dispatch`'s region $F036 and `zone_multiplex`'s
  $F033 produce **no `ProfileLineWorst` row at all**, so the gate compared nothing there; `shared_setxpos`'s
  $F054 was proven 83 against a measured 36 — **47 cycles of slack** to absorb the 15 cycles of error. Slack
  hides an under-approximation exactly as well as it hides nothing. The gate now also grades **five
  commercial cartridges** (VideoOlympics, Adventure, Seaquest, Chopper Command, Empire Strikes Back), which
  live in the umbrella `reference/` tree outside this repo: **absent → skipped with the reason logged;
  present → 5/5 graded, 63 region↔row pairs**, and *anything in between fails*, because a ROM that is
  present, loads, and then quietly compares nothing is a skip wearing a pass. Commercial images are profiled
  over 30 frames, not 6 — measured, Chopper Command yields **0** rows at 6 and **18** at 30. Nothing is read
  from these cartridges but their own bytes.
- **Witness: `roms/litmus/litmus_bpl_trip.asm`**, two single-scanline regions holding nothing but the
  countdown — `ldx #6 / sta $B0,x / dex / bpl` and `ldy #6 / sty COLUBK / dey / bpl`, 262 lines. The test
  asserts **equality**, proven == machine == **75** and **68** over 950 and 960 intervals, because tightness
  is the point: a merely-safe bound would send an author trimming a line that was never over budget. It
  checks its own premise first, via a measurement counter incremented at the corrected line, so it cannot
  pass over a region that never reached the fold. **Negative control:** reverting the `+1` fails it naming
  the **9**-cycle (dex) and **8**-cycle (dey) gaps, and with the bug in place the extended gate goes red on
  Seaquest `$F1FC` as well. **Second control:** re-proving the whole corpus before and after moved **6 of
  1226 regions**, all upward, all containing a `bpl` fold — Seaquest $F12C 102→107, $F1FC 66→75, $F419
  105→110; `rts_dispatch` $F036 55→69, `shared_setxpos` $F054 83→98, `zone_multiplex` $F033 181→214. Nothing
  else moved. The `dey`/`BPL` sibling is the same code path and is covered by the same fix and its own
  fixture region; `BMI` as a latch is *refused* rather than bounded (`determineBound` accepts only
  BNE/BPL/BCS/BCC), so it was already safe.
- **Two of five subjects of `TestDecodeReachesCodeInCommercialROMs` had been silently skipping.** Their paths
  used two levels of `..` where the umbrella tree is three up, so `VideoOlympics` and `Stampede` resolved to
  a `harness/reference/` that does not exist and took `t.Skipf` on every run while the test stayed green —
  the exact failure that file's own doc comment warns about ("an analysis that finds no instructions does not
  look wrong, it looks like a clean ROM"). Fixed; both now decode (**644** and **858** instructions), and
  Stampede's label is corrected from 4K to its measured **2048 bytes**.

## [2.0.0] - 2026-07-31

**MAJOR, because two MCP tools changed what a caller gets back.** `CLAUDE.md` says this project follows
SemVer and that MAJOR means 互換破壊, so the number is produced by the rule, not chosen: `beamtrace` changed
the SHAPE of its result and `breakif` changed the MEANING of its stop condition, and a caller written against
`1.117.0` is wrong about both. Neither is a mistake being undone — each is a capability that could not be
reached through the old contract (four of five traced frames were unreturnable; the whole visible region was
unstoppable). Everything else in this release is additive or a fix; `### Breaking` below is the complete list
of incompatibilities, and both entries are restated with their measurements under `### Changed`.

### Breaking
- **`beamtrace` returns `frames[]`; there is no top-level `rows`.** The result was a single `{frame, rows}`
  and is now `{frames: [{frame, rows}, ...]}`, one entry per traced frame. **A caller must change `rows` to
  `frames[0].rows` and `frame` to `frames[0].frame`** — reading `rows` at the top level now finds nothing
  there. Why it had to change: the tool traced `frames` frames and advanced the machine past all of them,
  then returned the earliest one alone (measured over the wire: `frames=4` starting at frame 5 left the
  machine at frame 9 and handed back frame 5), so the other frames were unreachable by any route. Pass
  `scanline` to keep the larger payload narrow.
- **`breakif` halts at or past the requested beam position, not on an exact match.** It now stops at the
  first instruction boundary **at or past** `(until_scanline, until_clock)`, so a caller that relied on
  "stops only on the exact value" now stops **earlier** — measured, asking for clock 80 halts at 82 — and
  must read the returned `coords` instead of assuming them. Why it had to change: the machine is only
  observed at instruction boundaries, the CPU advances 3 colour clocks per cycle so only one phase in three
  is ever observable, and a WSYNC kernel narrows it much further — measured on `motion_xclamp`, a visible
  scanline is observed at **7 clocks, every one of them inside HBLANK**, so the visible region 0..159 could
  not be stopped inside at all. An exact-match request for a position in the picture ran to `max_frames` and
  returned `halted=false` with no error, which is indistinguishable from "not yet". Second incompatibility in
  the same tool: an out-of-range clock is now an **error**. The old tag advertised "0-227" while the
  coordinate system is HBLANK −68..−1 / visible 0..159, so 68 of the advertised values did not exist; a
  caller passing one of them used to get a silent never-halt and now gets a rejection.

### Added
- **`LoadROM` no longer dies on a truncated image — and 2 of the 542 `.bin` files on this machine
  killed it.** Found by sweeping every image under the umbrella through the loader: a 12-byte
  `Combat.bin` and a 5-byte `skeleton_test.bin`, both partial downloads in a mined reference archive,
  **panicked** instead of failing to load. The fault is upstream — `hasSuperchip`
  (Gopher2600 `fingerprint.go`) compares `d[:0x80]` against `d[0x80:]` on every 4K window with no
  length check — but the consequence is ours: `load_rom` and `assemble_and_load` take a path from the
  MCP caller and `cmd/fieldtest -inbox` walks a directory the **user** drops files into, so this ended
  the server rather than one call. A truncated download is the most likely malformed input there is,
  and it was the one input that could not be reported. Two layers, verified independently by disabling
  each: a length precheck that can say what is wrong ("this file is 12 bytes"), and a `recover` backstop
  for any other fault the fingerprinter can take on bytes it did not anticipate. **The first version of
  the test proved nothing** — it wrote zero-filled files of the same sizes, and with the guard removed
  those load fine instead of crashing; the fixtures now carry the observed bytes, which do panic.
- **The edge-semantics whitelist is now re-read from the engine's source by a test, not by a human once.**
  `verifiedEdgeSemantics` claims seven mappers select the bank from the address alone, each entry citing
  the file and method where that was read — and nothing checked that the file exists, that the method
  exists, or that it still reads that way. Gopher2600 is a `replace` dependency that gets updated. The
  new gate parses the cited method out of the cited file with `go/ast` and requires that a mapper claimed
  as address-only never reads its data-bus parameter; the mirror assertion requires that the mappers
  recorded as data-driven **do** read it, so the check cannot pass by matching nothing (2 witnesses:
  WF8, FA). AST rather than grep, because CBS quotes the patent's "data line D0" in a comment six times.
  Negative controls: whitelisting FA fails it, and so does a citation to a method that does not exist.
  What it proves is one failure mode — data-bus selection, the WF8 trap — not all of them.

### Changed
- **Four more mappers moved from "unchecked" to "checked, and here is how they differ"**
  (`knownDifferentEdgeSemantics`), each read out of the engine's source: **FA** gates the entire switch
  on data-bus bit 0 (the CBS patent's requirement), so `lda $1FF9` switches or does nothing depending on
  what is on the bus; **FA2** guards `$0FFB` with `len(banks) > 6`, so the published `$1FFB:BANK6` edge
  does not exist on a 6-bank image, and spends `$0FF4` on NVRAM file I/O; **E0** does not switch a bank
  at all but assigns one of three 1K **segments**, leaving the fourth quarter — the one holding the
  hotspots and the vectors — permanently fixed; **E7** spends four hotspots on RAM rather than banks and
  reduces the result by `bank %= NumBanks()`, so its address-to-bank map depends on the image size.
  The refusal was already correct for all four; now it says why. **Only E0's message can actually print**,
  and that was measured rather than assumed: `bankedUnits` refuses a RAM-mapping cartridge before it
  consults this table, and FA/FA2/E7 all carry cartridge RAM by construction, so those three are
  permanently shadowed by the coarser refusal — the same shape as the `foldLoops` finding. E0 has
  `IsRAM: false` on every segment and does reach it, witnessed on three real cartridges (Montezuma's
  Revenge Trainer, Super Cobra, Swtagrc). The shadowed entries still earn their place: they exist to stop
  a future reader from pattern-matching published hotspots onto the Atari rule and promoting a mapper into
  `verifiedEdgeSemantics`, where being wrong invents edges the machine never takes.
- **Measured, and it is the number nobody had: `Prove` gives an ANSWER to 0 of the 33 exotic-mapper images
  on this machine.** Every one is refused — DPC+ (7), F4SC (10), 3E (5), F8SC (3), E0 (3), F6SC/FA/AR (1
  each) — so the analysis has no silent-wrongness path on cartridges outside the F8/F6/F4 families, which
  had never been checked end to end. G1 in the audit is the work of *adding* support; this is the prior
  question of whether its absence is honest, and it is.

- **`cb_pushdisplay` / `cb_pushsafe` — the twin fixtures that witness `pushMissesDisplay`'s
  "SP can reach the display" branch**, which had run 0 times across 129 ROMs. A `PHA` writes to `$0100|SP`
  and page 1 mirrors the console's addresses, so a program that points SP at the bottom of the stack turns
  a push into a VSYNC/VBLANK write — the Stack Trick, and the entire reason the prover tracks SP here
  instead of calling every push display-touching. Nothing exercised the YES side: the one ROM that reached
  the predicate took the "proved to miss" path, and `litmus_stack_trick` — written for this very hazard —
  never reached it at all. The danger twin pushes with **SP = 1** (`$0101` = VBLANK) inside an overscan
  region that would otherwise be classified BLANK and skipped; the safe twin changes **one immediate**
  (`$01FF`, stack RAM). Measured: `visible` vs `blank`, both 16 cy, blank-region count differing by exactly
  one. The branch was confirmed directly (`reaches-display sp=1 ma=$01`), not inferred. Negative controls:
  every-push-safe and every-push-dangerous each fail the test. A fixture defect was caught before shipping —
  the first version's region ran past `jmp Main` into the next frame's `sta VSYNC`, so both twins came back
  visible and it proved nothing.
- **`cb_deadpred` / `cb_deadpred_live` — the twin fixtures that finally witness `determineBound`'s
  "a predecessor we know nothing about" refusal**, a guard that had run 0 times across 123 ROMs and had
  never been shown unreachable either. Two fixtures failed first, and both are recorded: code hopped over
  by a `jmp` is never decoded, so it never becomes a predecessor at all (measured — the scan listed 9
  candidates and the dead address was not among them), and dead code placed in the region above the header
  is not seen because the scan is per-region. What reaches it is the **not-taken edge of a statically known
  branch**: decoded, and then given no abstract state at all, because `absSuccessors` emits only edges
  whose refined state is still valid. Measured over 129 ROMs, the neighbouring `!st.valid` condition fires
  **zero** times anywhere — pruned nodes never acquire a state to be invalid — and the missing-entry
  condition cannot be relaxed, because a fixpoint that hits its iteration cap also leaves nodes with no
  entry. The twins
  differ by that one edge, so the refusal is attributable — the dead one leaves its visible region
  unbounded, the twin bounds all 7 regions with a dearer worst (33 vs 17 cy). Negative control: removing
  the guard makes the dead ROM come back fully bounded.
- **`spritey` and `read_motion` now report `stillness`** — how far the object travelled on each axis over
  the window they measured, and whether any RAM byte changed — so a measurement carries the evidence for
  whether it measured anything. Travel of 0 is flagged as a **CONSTANT**, the shape behind both of this
  week's bad measurements. Multi-frame only: `frames=1` has no window, and the tool will not advance the
  machine to manufacture one. **The note deliberately draws no conclusion about the program, because the
  first version did and was wrong** — it classified "no travel and no RAM change" as *STUCK: the program
  is not running*, and measured before shipping, `litmus_pos` and `smoke` run a full kernel, draw their
  sprite every frame and never write RAM after init, so a live ROM got a confident diagnosis of dead.
  Whether a program REACTS cannot be established without injecting an input, which is why that question
  stays with `set_input`. Three tests; negative controls: restoring the STUCK verdict, and reporting one
  axis's travel into the other, each fail.
- **`motion_xclamp` litmus + the witness for `spritey`'s multi-frame mode.** The mode returns a per-frame
  sample carrying BOTH X and Y and always has, but nothing ever checked that the X in those samples MOVES:
  a build reporting a constant X, or reporting one axis into the other, passed every test in this repo.
  The new ROM is the horizontal mirror of `motion_glide` (which moves Y and pins X) — P0 glides right
  +2px/frame, CLAMPS, holds, and then the round ends and it snaps back to the start, all with a fixed Y
  band. Measured: X 13→91 in +2 steps, plateau at 91 for 80 frames, reset to 11 at frame 121, Y fixed at
  80–119, 262 scanlines. Two tests in `cmd/harness` pin it, and the reset half pins WHY the trajectory is
  worth preferring: a single read at frame 130 returns 29 while the trajectory over the same span peaks at
  91. That is the Outlaw failure (hold "right" 700 frames, read once, get x=7 near the LEFT edge because
  the round had ended) reproduced with known constants. Negative controls: pinning X to a constant,
  reporting Y into X, and letting Y drift with X each fail the tests. Note the trap liveness does NOT
  cover — the program is reacting the whole time; liveness answers "is it running", not "am I still in the
  situation I set up".

### Changed
- **`checks.motion` certified nothing on its own, and now says so in its own output.** `jerk_rms` is the
  RMS of the position's 2nd difference: 0 for constant velocity, and 0 for an object that never moves —
  measured, `{"axis":"x","max_jerk_rms":0.5}` **PASSES on `litmus_pos`**, whose P0 is pinned at one X for
  the whole run, so the judder regression the gate exists to catch and a completely dead kernel were
  indistinguishable to it. `motion.Stats` now carries `span` = max(pos) − min(pos), the scenario check
  **prints it unconditionally** (a scenario quietly gating a frozen object reports "span 0"), and the new
  `min_span` gates it. Applied to both motion scenarios: `motion_glide` span 39 ≥ 30, `motion_xclamp` x
  span 58 ≥ 50. Negative control: forcing `span` to 0 fails a genuinely gliding object.
- **`beam_intervals`' `crosses_line` was wrong 81% of the time it spoke.** The flag was computed as
  `(minAbs+68)/228 != (maxAbs+68)/228`, which places the scanline boundary at clock 92 of each line
  instead of at the line's start — `MinAbs`/`MaxAbs` are already measured from a WSYNC, i.e. from a
  boundary, so no shift belongs there. Measured over 127 ROMs / 1016 proven writes before the fix: **43
  flags raised, 35 of them false, and 11 real crossings missed**; after, 19 flags with 0 wrong in either
  direction. Concretely, `bullets.asm $F108 GRP0` proves to `[130..-20]` — a window that runs off the end
  of the line and folds into an inverted pair — and was NOT flagged, while `$F0FB GRP0` at `[82..154]`,
  entirely inside one line, WAS. The regression test uses the one direction checkable without restating
  the formula: folding preserves order within a line, so an inverted window is a proof of a crossing and
  must be flagged (13 such windows in the corpus; non-vacuity asserted). Negative control: restoring the
  old expression fails both tests.
- **`breakif` now halts when the beam REACHES a position, instead of silently never halting.** It required
  an exact `(scanline, clock)` match, and observations only happen at instruction boundaries: the CPU
  advances 3 colour clocks per cycle, so **only one phase in three is ever observable**, and a WSYNC kernel
  narrows it much further — measured on `motion_xclamp`, a visible scanline is observed at **7 clocks, every
  one of them inside HBLANK**, so the whole visible region 0..159 could not be stopped on at all. Asking for
  a position in the picture ran to `max_frames` and returned `halted=false` with no error, which is
  indistinguishable from "not yet". It now stops at the first instruction boundary at or past the target
  (measured: asking for clock 80 halts at 82), a position already passed in the current frame is caught on
  the next frame, and an out-of-range clock is an **error** — the tag used to advertise "0-227" while the
  coordinate system is HBLANK −68..−1 / visible 0..159, so 68 of the advertised values did not exist.
  Three tests pin it; negative controls: restoring the equality match, and arming unconditionally, each fail.
- **`beamtrace` now returns every frame it traced, not just the first.** It traced `frames` frames,
  advanced the emulator by all of them, and returned the EARLIEST one alone — measured over the wire:
  `frames=4` starting at frame 5 left the machine at frame 9 and handed back frame 5. The discarded
  frames were unreachable by any other route, because a second call advances the machine again, so
  frame-to-frame comparison — flicker, multiplexed sprites, a first frame that is atypical after setup —
  was impossible in a tool whose description promised the frames. Output is now `frames[]`, each entry a
  `{frame, rows}` pair (was a single top-level `frame`/`rows`). Pass `scanline` to keep the payload narrow.
  Witnessed by `TestBeamtraceReturnsEveryFrameItPaidFor`, which reads a register that provably changes
  every frame (motion_xclamp stages HMP0 = 96, 64, 32 as P0 walks) so that N copies of one frame cannot
  pass; negative controls: truncating to the first frame, and repeating one frame N times, both fail it.

### Fixed
- **`defuse` reports `writes_into_code`** — SD-3's "a store landing in decoded code space is a fact, not a
  guess". Every reachable write whose target set intersects addresses the decoder read as INSTRUCTIONS,
  with the writer's PC and source location. Each entry carries `exact`, because an exact store into code is
  a **fact** while a may-set that merely reaches code is a **possibility** — an indexed store spans up to
  256 addresses and a 4K image is mostly code, so collapsing the two would drown the first in the second.
  **Shipped with a planted fixture rather than a corpus witness, deliberately**: measured first, 133 ROMs
  and **zero** that write into the cartridge window at all, and a detector whose branch nothing reaches is
  not a check. `litmus_smc` plants one; `litmus_smc_clean` aims the same store at RAM. Measured after:
  **123 analysable ROMs including four commercial cartridges, exactly one report — the planted one.** The
  test gates both halves. Negative control: aiming the planted store at RAM fails it.
- **`cover -drive explore`** — cycles SELECT through the game variations, presses RESET, then rotates the
  stick, instead of holding a fixed input. Added because of a measurement: a 2600 attract mode runs the
  **game loop** with synthetic input, so a cartridge left alone already covers most of what playing it
  covers — on Chopper Command, RESET plus a dozen rounds of stick moved the executed count by **4
  instructions out of 2358**. What sitting there does not cover are the other game **variations**, behind
  SELECT: **Seaquest 51% → 60%** of its decoded instructions, Adventure 61% → 67%, Chopper Command
  46% → 49% — all four measured at the SAME driving budget, so they compare drivings rather than state a
  ceiling; given more frames every ROM saturates near 68–78%. The report now carries `drive`, because a coverage percentage is a property of a ROM **and a
  driving**, and two numbers taken under different drivings are not comparable. Panel switches go through
  `SetPanel`, not `SetInput` — ignoring that error is how an earlier measurement "pressed" RESET without
  pressing anything.
- **`prove_line_budget` / `cyclebound.Prove` now accept a raw `.bin`** — the entry point that was missing.
  SD-0c taught the *decoder* to read real cartridges (Outlaw's 2K went from 0 instructions to **931**,
  Combat's from 0 to **838**), but nothing public took a raw image: `Prove` and `timinglint` assemble their
  input, so every commercial ROM came back as *"Unknown Mnemonic"* — measured on Adventure, Seaquest,
  Chopper Command, VideoOlympics and Empire Strikes Back. The capability existed and was unreachable, which
  blocked the casebook line, where the ROMs are commercial by definition. Now measured on real cartridges:
  **VideoOlympics 8 regions, Adventure 14, Seaquest 49, Chopper Command 29, all converged.** A raw image
  loses only what SOURCE carries — `@lines`/`@amax` annotations and label locations — and `srcmap` is
  nil-safe throughout for exactly that. **The `.asm` path is byte-identical** (6 ROMs including a banked
  one, whole JSON output compared), and a test pins that the same ROM yields the same region count and
  worst case through both routes.
- **The collision latches' three unlocked claims are now locked** (`litmus_cxclr` +
  `TestHmclearDoesNotClearCollisions`). The D7/D6 map had a pure-function test; "sticky", "CXCLR clears"
  and "**HMCLR does not**" did not — stickiness appeared only in a comment and the HMCLR distinction was
  checked nowhere, which is the one a reader can actually get wrong, the names differing by two letters
  while both read as "clear something". Measured: CXP0FB is `$82` after the collision, **`$82` still after
  HMCLR**, `$02` after CXCLR. Negative control: swapping the two strobes in the ROM fails the test. The
  fixture also records its own first failure — it lit `PF1` only and positioned P0 with a div-15 loop that
  was never given a target value, so P0 missed the band and all three snapshots read "no collision"; the
  playfield is now lit solid so the answer does not depend on positioning at all.
- **The 6502 page-cross rules are now machine-locked** (`TestPageCrossPenaltyRules`, 11 cases). They were
  cited from 6502.org and never re-measured here, while `cyclebound`'s entire per-scanline proof rests on
  "stores never take the penalty" — and its page-cross costing was where a real under-approximation was
  found earlier the same day. Measured one instruction at a time through the silicon-differential harness:
  STA abs,X is 5 crossing or not, STA (ind),Y is 6 either way, LDA abs,X 4→5 and LDA (ind),Y 5→6 on a
  cross, branches 2 / 3 / 4. **All as documented.** The test also records a trap it fell into itself: a
  FORWARD branch from `$F802` cannot cross a page at all (the largest offset `$7F` reaches `$F881`), so the
  crossing case has to branch backwards — the first version asserted 4 cycles for a branch that never left
  its page.
- **All 20 playfield columns are now verified, not 3** (`litmus_pf_allcols` +
  `TestEveryPlayfieldColumnLandsWhereTheTableSays`). `CLAUDE.md` lists the column→register→bit map under
  "constants you must never get wrong" and cited `litmus_pf`, which lights **columns 0, 4 and 12** — the
  leftmost bit of each register, three of twenty positions and the three easiest. The new ROM draws the
  whole map in one frame (20 bands of 9 scanlines, band k lighting only column k) so every entry is
  checked, including that nothing else lights up — which is what catches a bit landing in the *wrong*
  column rather than in none. Measured: all 20 land on `4k..4k+3` and repeat at `80+4k`, confirming the
  repeat rule and the half boundary at clock 80 at the same time. **The table is correct.** Negative
  control: reversing PF2's byte order fails 8 columns.
- **The 16-nibble HMOVE table is now machine-locked** (`TestAllSixteenHmoveNibblesMoveByOnePixelEach`).
  `CLAUDE.md` lists it under "constants you must never get wrong" and cited a hand verification from
  **v0.4.0** — the existing HMOVE tests cover the ripple counter and the idle/unrecorded distinction, and
  `litmus_hmove` has no scenario, so nothing had held the table true since. Re-measured: all 16 match
  (`$70`=−7 … `$00`=0 … `$F0`=+1 … `$80`=+8), and the test asserts each nibble at the **drawn pixel** via
  `DecomposeRow` as well as at `HmovedPixel`, so a readout that stops describing the picture fails too.
  Negative controls: flipping the sign convention fails 7 nibbles; shifting `DecomposeRow`'s clock by one
  fails the drawn-position half.
- **`scripts/check_tests.py` — a gate on tests that cannot fail** (CI- and pre-push-gated, with a
  `--selftest`, like `check_traps`). A `func TestXxx` must either assert on `t` or hand `t` to a helper that
  does; anything else runs code and draws no conclusion, and `go test` will never say so. Measured when it
  was written: **344 test functions, exactly one with no failure path** — `TestZZProbe`, a scratch probe that
  printed to stdout, asserted nothing, and referenced absolute paths outside the repo. It was swept into a
  docs commit on 2026-07-29 and had been contributing a meaningless green tick since. Deleted; the tree is
  now at 343/343 able to fail. The detector clears delegation (`helper(t, ...)`, `t.Run`) — verified against
  a delegating test that would otherwise have been a false positive.
- **`cmd/dissect` carried two copies of the offset→address mapping and only one knew about banks.**
  `fmtRange` used `off/4096` and `$F000 + off%4096`; the step that matches an annotation to a DiStella
  label by address used the naive `0x10000 - len(rom) + off`, which on an 8K image resolves **every offset
  in bank 0 to `$Exxx`** — an address the 2600 never fetches — so those annotations matched no label and
  were dropped without a word. Both now call one `romAddrOf`, unit-tested across 4K/2K/8K/16K including the
  boundary bytes of each bank; negative control: removing the bank branch fails it.
- **A temporary ROM patch silently patched the wrong bank of an 8K image.** `assert_line_budget`'s
  `patch=` resolved an address to a file offset with `base = 0x10000 - len(rom)`, which puts an 8K
  cartridge's base at **$E000** — an address the 2600 never fetches from — and then resolves every patch
  into the SECOND bank: `$F123` became file offset `$1123`, inside the file, past the bounds check, with
  the range in the error text quoted as "$E000-$FFFF". A measurement taken on a ROM patched in the bank
  that was not running is worse than no measurement, because it is reported as one. Now **declined** with
  a message that says why, the way `defuse` and `beam_intervals` decline banked images; flat 4K/2K images
  are unaffected. Patching by bank is filed, not implemented — `PatchSpec` would need to carry one.
- **`mutate -covered`'s "honest kill rate" mutated the wrong half of a banked ROM.** `CoveredOffsets`
  mapped an executed PC to a file offset with `addr & (len(rom)-1)`, which on an 8K image folds every
  `$Fxxx` into the LAST 4K image whichever bank actually ran. Measured before the fix: on the exerciser
  **all 278 covered offsets landed in `$1000-$1FFF` and not one in `$0000-$0FFF`** — so "restrict fault
  injection to code that actually executed" was injecting into bank 1's bytes while bank 0 was the half
  executing, producing mutants that cannot be killed for precisely the reason `-covered` exists to avoid.
  Now `bank*4K + (addr & 0x0FFF)`, via the new `Coverage.SeenSites()`. The exerciser goes to 315 offsets
  with **49 in bank 0's image**; a 4K ROM is the one-bank case and is unchanged (`smoke.bin` still 38, so
  the published "2% naive vs 68% covered" figure does not move). Negative control: restoring the old fold
  fails the new test.
- **Coverage was bank-blind, and it flattered.** `internal/emu/coverage.go` keyed executed instructions,
  branches and both edge sets on a bare address, but two banks of an 8K image decode the same addresses.
  It failed in both directions at once: as a count it under-reported distinct executed instructions
  (exerciser, 200k instructions: 319 pairs executed, `PCCount` reported **282**, 37 addresses run in BOTH
  banks), and as a query `Seen(addr)` answered "covered" for the twin in the bank that never ran — the
  flattering direction, which VV-3's coverage percentage and `mutate -covered`'s "honest kill rate" both
  rest on. Now keyed on `(bank, address)`, with the fetch bank captured **before** the step (a hotspot
  access changes the mapping as it completes, so asking afterwards attributes the switching instruction to
  the bank it switched to). `Signature()` includes the bank too, so guided fuzzing on an 8K image can tell
  the halves of an address apart. `Seen(addr)` deliberately keeps its meaning ("in SOME bank") and
  `SeenIn(bank, addr)` is added beside it. **Flat ROMs are unaffected — `cmd/cover`'s whole JSON output is
  byte-identical 5/5**; only banked images move (exerciser `pc_executed` 268 → 297).
- **The prover's most important soundness check — "the machine never exceeds the proven worst case" — was
  bank-blind and ran on 31 ROMs.** It keyed proven regions on address alone while `LineWorst` has carried
  `Bank`/`BankValid` all along, so on an 8K image a region proven in one bank could be paired with a
  measured row from the other. The dangerous direction is the quiet one: an accidental pairing that happens
  to satisfy `observed <= proven` **hides** a real gap, which is the failure this test exists to catch, and
  `banked_game` is in its corpus. Now keyed on `(bank, address)` and run over the whole tree:
  **896 measured regions across 128 ROMs, no exceptions** (was 228 across 31), for 5.5s.
- **The two headline soundness gradings ran on 31 of ~129 ROMs, and extending them roughly tripled the
  evidence for free.** `defuse`'s "9055/9055 observed (pc,addr) pairs inside their predicted sets" and
  `beam_intervals`' "7117/7117 observed writes inside their proven window" were both measured over
  `roms/techniques/*.asm` alone — a denominator neither number stated. Run over the whole corpus they read
  **32655/32655** and **19143/19143**, still zero violations, costing 4.7s and 3.2s. The quoted figures in
  `CLAUDE.md` now name their corpus. A side effect: `defuse`'s CFG-reach-gap report now also names
  `litmus_6502.asm` (66 writes from instructions the decoder never reached), which had been invisible.
- **The blank-classification grading ran on 32 of 129 ROMs, and two defects in the grading itself were
  what kept it there.** It is the one verdict in the package that can hide a real scanline tear, so its
  corpus matters. (1) Its `blank` map was keyed on a region's **address alone**, and an 8K image decodes the
  same addresses in both banks — so on a banked cartridge it matched whatever sat at `$Fxxx` in the *other*
  bank and graded unrelated code. `banked_game` is in the original corpus, so part of the old number was
  aimed at the wrong instructions. (2) It sampled the display state **at** the region's opening `sta WSYNC`,
  but a TIA register write is delayed (`futureVblank`), so a `sta VBLANK` issued one instruction earlier has
  not reached the signal yet — measured, `DisplayOff()` is false at the strobe and true one instruction
  later for a region that is genuinely blanked. Both fixed; the corpus now runs the litmus and exerciser
  ROMs too: **128 ROMs, 133,684 executions of blank-region entry points, 0 disagreements** (was 32 ROMs).
  The per-ROM budget went 400k → 120k instructions in the same change, measured: 180s → 57s, with *blank
  regions never reached* — the number that says what this grading does not see — unchanged at **1**.
- **`emu.DisplayOff()` ignored VSYNC.** It read only `sig.VBlank`, while the prover's own `displayOff()` is
  `VSync || VBlank` — and VSYNC blanks the picture as surely as VBLANK does. Measured while extending the
  blank-classification grading past the technique corpus: **6730 "disagreements" appeared, every one on a
  frame's VSYNC lines** in a ROM that raises VSYNC without also raising VBLANK. The prover was right and the
  oracle was short a term. The error direction was false ALARMS, never missed detections, so nothing unsound
  had shipped; what it had done was silently cap that grading to ROMs which happen to raise VBLANK during
  VSYNC — the one verdict in the package that can hide a real scanline tear. Its only consumer is the
  grading test, which still passes unchanged (144,568 executions across 32 ROMs, 0 disagreements). The
  corpus extension is not shipped: 776 disagreements survive, and the evidence points at the oracle's
  sampling point rather than the prover, but that is filed as a hypothesis with its numbers, not acted on.
- **Four more descriptions made to match measured behaviour** (from the 38-tool sweep; no behaviour change).
  `step_scanline` said "CPU cycles consumed across that scanline" for a figure that **excludes WSYNC stall
  time** — measured 8 on twelve consecutive 76-cycle lines, so the remainder read as headroom that does not
  exist. `assert_line_budget`'s `line_cycles` is `scanlines × 76`, a quantised figure and never a measured
  count, so subtracting the budget from it yields a number of cycles to cut that means nothing.
  `read_audio` returns `note0`/`note1` = {note, cents} and said so nowhere, although it is the only
  register→pitch conversion in the whole tool surface. `analyze_image` told the reader "one screenshot is
  one frame of truth (flicker objects appear partially)" while accepting `paths[]` and running a
  multi-frame pipeline with an explicit flicker report — the description denied the one capability a
  flicker-hunter would search for, and its three inputs shipped with **no descriptions at all** on the wire.
- **`spritey`'s description advertised half of what the tool returns.** Its multi-frame mode was documented
  as returning "the per-frame Y trajectory", but every `SpriteYSample` has carried `X` (HmovedPixel) since
  the tool existed. Someone looking for a HORIZONTAL trajectory therefore did not find the tool that already
  had one. Measured cost on 2026-07-30: both attempts to pin down Outlaw's horizontal clamp were hand-rolled
  against `read_tia` instead, and one of them read the position once after holding an input for 700 frames —
  long enough for the round to end and the sprite to be reset — producing a confident, stable, wrong number.
  The description now names the X trajectory and warns against the single late read. No behaviour change.

## [1.117.0] - 2026-07-30

**Release-hygiene cut.** Everything that had accumulated under `[Unreleased]` is versioned here as one
**MINOR** release. It adds backward-compatible capability (new MCP tools and new `cmd/` tools), carries one
prover **soundness fix** (a cycle-cost under-approximation in `cyclebound`), and adds tests and litmus ROMs.
Not MAJOR: no exported Go API and no MCP tool was removed or re-signatured — the soundness fix changes the
numbers `prove_line_budget` reports, not its contract, and that contract always was "a sound upper bound over
all paths", so making it finally hold is a fix. Not PATCH: new functionality ships here. The number is
`1.117.0` (not `1.108.0`) because `internal/version/version.go` already ships `const Harness = "1.117.0"`,
and that constant is stamped into the MCP `serverInfo` and into `ramtrace` provenance headers — the CHANGELOG
is matched to the artifacts already produced rather than the other way round.

**Tag/CHANGELOG drift measured at this cut** — 170 tags vs 174 released sections:
- `v1.104.0` / `v1.105.0` / `v1.106.0` / `v1.107.0` were **tagged but never given a `##` section**. Their
  entries are folded in below and keep their inline `(v1.10x.0)` markers, so which work shipped when is not
  lost. `v1.105.0` (`read_ram_trace`; tag at `aab7ab7`, 2026-07-21) has **no CHANGELOG text at all** — it is
  recorded here as a known gap rather than reconstructed after the fact.
- `1.108.0`–`1.115.0` were named inline inside `[Unreleased]` but were never sectioned and never tagged; they
  ship here as part of `1.117.0`.
- `1.80.0`–`1.102.0` (23 versions), plus `0.5.1` and `0.6.0`, have `##` sections but **no tag**. Deliberately
  **not** tagged retroactively: a tag must point at the commit that shipped that version, and that mapping is
  not recoverable from the CHANGELOG alone.
- `1.74.0` and `1.116.0` were skipped entirely — no section, no tag, no mention anywhere.

### Added
- **`cyclebound` proves a WSYNC-to-WSYNC region that CROSSES A BANK SWITCH** (SD-11, stage 3 of bank
  support). Stage 2 closed the DECODE over bank switches; the flow was still refused, so the region where a
  bank-switched kernel does all of its cross-bank work had no number at all. It now has one, and the number
  is graded against the emulator rather than against the prover's own arithmetic.

  | ROM (mapper, banks) | crossing region | before | proven after | machine measured |
  |---|---|---|---|---|
  | `litmus_bank` (F8, 2) | bank 0 `$F02B` | REFUSED | **54** | **54** / 1 line |
  | `litmus_bank_f6` (F6, 4) | bank 0 `$F02B` | REFUSED | **72** | **72** / 1 line |
  | `litmus_bank_f4` (F4, 8) | bank 0 `$F02B` | REFUSED | **128** (violation at 76) | **128** / 2 lines |
  | `banked_game` (F8, 2) | bank 0 `$F01B` | REFUSED (switch) | still unbounded, **different reason** | 28 / 1 line |

  The three litmus kernels are deterministic, so proven EQUALS measured on all three — not merely
  proven >= measured. `litmus_bank` and `litmus_bank_f6` now come back `certified:true`; `litmus_bank_f4`'s
  chain genuinely spends more than one scanline (its source compensates with `ldx #29`), so it is reported
  as a stated 128-cycle budget violation rather than a refusal.

  - **Code identity is now `(bank, address)`, everywhere.** Every bank of a bank-switched cartridge is
    mapped into the same `$F000-$FFFF` window, so a bare address cannot tell two banks apart — measured,
    `litmus_bank_f4` has **1399 of 1427 decoded addresses claimed by 2+ banks**. DATA addresses stay flat on
    purpose: RAM `$80-$FF`, TIA/RIOT `$00-$3F` and the page-1 stack mirrors are not banked, so a bank in a
    data key would split one physical cell into N.
  - **THE EDGE comes from the engine, not from folklore.** An instruction whose DATA access reaches a
    bank-switch hotspot continues at the SAME ADDRESS IN THE TARGET BANK: the Atari mapper switches on the
    access and does not touch PC (`mapper_atari.go` runs `bankswitch(addr)` and then returns
    `cart.banks[cart.state.bank][addr]`). The switching instruction is charged its ordinary cost and the
    edge is charged nothing. When the access is EXACT the intra-bank fall-through is REPLACED, which is what
    makes the numbers exact rather than merely safe; a wide footprint keeps the fall-through as well.
  - **One oracle decides both the edge and the refusal.** `switchModel.switchEdges` is the single point;
    `residualSwitchRefusal` and every walker (collect, longest, the abstract interpreter, the beam pass,
    `determineBound`'s predecessor scan) are driven off it, because a successor function modelling a switch
    the refusal does not guard is silently unsound.
  - **Still refused, counted in `unmodelled_switches`, still blocking `certified`:** an instruction whose
    OWN BYTES span a hotspot (the opcode comes from the new bank, so the decoded instruction is not the one
    that executes), a `jmp`/`jsr` INTO a hotspot, an unresolvable indirect access under a hotspot-bearing
    mapper, a hotspot symbol that does not name a bank (`B0S0`, `RAM0`), a target bank outside the analysed
    set, and a landing address outside cartridge space. New `modelled_switch_edges` prints beside the
    refusal count, because "0 refused" is also what a cartridge that crossed nothing reports.
  - **The geometry is checked, not assumed.** `analysisUnits` declines any mapper whose banks are not the
    whole 4K window at `$F000` (`len(Data) == 4096`, `Origins == [$F000]`). M-Network is the trap that
    parses: it publishes `BANK0..BANK6` as bank-switch hotspots while its banks are 2K at TWO origins, so
    "the same address in the target bank" is false there — and under stage 3 a wrong seed is no longer just
    an over-decode, it is a CFG edge the longest-path walk follows, and a wrong edge can SHORTEN the
    longest path.
  - **`romTableRange` is routed by bank and refuses a hotspot byte.** On a merged program a flat reader
    would fold whichever bank was bound, so a `lda table,x` in bank 1 could be bounded by bank 0's table —
    a narrow, confident, wrong range feeding a trip count. A footprint containing a hotspot is `Top`,
    because the hardware switches first and returns the TARGET bank's byte there.
  - **`determineBound` keeps SD-9's guarantee across a bank boundary.** Its predecessor scan follows
    cross-bank edges and returns 0 unless the predecessor set is complete (an incomplete set
    under-approximates the entry value, hence the trip count, hence the worst case); both address-order
    filters are now SAME-BANK comparisons, because addresses in different banks have no order at all; and
    the `lda #imm` address proxy still live on the BCS/BCC path is same-bank-gated and COUNTED — measured
    **0 hits** across all 31 technique ROMs, all 12 `cb_*` litmus, `litmus_bound_proxy` and every bank ROM.
    `foldLoops` refuses outright any loop whose body contains a switching instruction: the folded cost
    assumes every iteration executes the same bytes, and after a switch iteration 2 does not.
  - **An unmodelled switch WIDENS its possible landing sites to `Top`.** The value domain is a
    whole-program fixpoint while a refusal is per-region, so refusing the region that contains a switch does
    not protect the region that contains its landing. `switch_widened_sites` + `switch_widen_reasons` report
    it: measured **5 sites on `litmus_bank` and 5 on `banked_game`**, all from decoded-but-never-executed
    filler bytes at bank 1 `$FFF6/$FFF8/$FFF9`, and **0 on `litmus_bank_f6`/`_f4`**.
  - **Merged fixpoint, measured before and after:** `litmus_bank` 151 sites / 17 ms, `litmus_bank_f6` 4171 /
    24 ms, `litmus_bank_f4` **9763 / 19 ms**, `banked_game` 134 / 47 ms — `converged:true` on all four, with
    the iteration cap now scaling with the program (`iterCap`) instead of being a fixed 300000 sized for
    ~1.4k per-bank nodes.
  - **Three new litmus ROMs, each closing a hole a test could not otherwise see.**
    `litmus_bank_shared_addr.asm` makes two banks execute DIFFERENT code with DIFFERENT costs at ONE address
    inside one region (measured: 38 executed `(bank,pc)` pairs over 35 distinct PCs — the first ROM in the
    corpus that can catch a flat-keyed instrument at all; proven 58 = machine 58).
    `litmus_bank_bound.asm` puts a counted loop in bank 1 whose ONLY initialiser is in bank 0, so an
    intra-bank predecessor scan loses the bound (proven 70 = machine 70).
    `litmus_bank_unmodelled.asm` keeps a switch that can NEVER be modelled (an `sta (ptr),y` whose target no
    address analysis can pin down), so the certification gate still has a witness — without it the gate
    would pass with the gate deleted.
  - **`DefUse` now DECLINES a bank-switched image** instead of computing MayWrite/Writes/Regions from the
    flat 8K fold while suppressing only the uninitialised-read pass. Measured on `litmus_bank`, that path
    produced `may_write: []` for a cartridge that demonstrably writes `$80/$81/$82` — empty by LUCK, because
    the flat fold decodes almost nothing, which is exactly the shape SD-7 condemned in `Prove`.
  - **`srcmap`'s package doc corrected to what the code does.** It claimed banked ROMs return an empty
    string; measured, they return a label-only string built from the `.sym` file. DASM's listing address
    column is the PHYSICAL ROM OFFSET on a banked image, so bank 0's line numbers are dropped and banks
    1..n's offsets are stored as if they were CPU addresses — which makes `@lines`/`@amax` INERT on every
    bank-switched kernel. New `source_annotations` says so out loud, because `litmus_bank_f4`'s 128-cycle
    violation is over budget only for want of an `@lines 2` the map cannot read.
  - `ProverVersion` -> `cyclebound/3 (VV-2 abstract-interp WCET + @lines + cross-bank flow)`.
  - **Golden diff, mandatory:** cyclebound JSON for all 31 `roms/techniques/*.asm`, all 12
    `roms/litmus/cb_*.asm`, `litmus_bound_proxy` and `litmus_superchip`, before vs after: **44 of 44 FLAT
    ROMs byte-identical**; only the 4 bank-switched images changed. `litmus_bound_proxy` still proves 1015
    (the SD-9 lock) and `litmus_superchip` is still declined.
  - **What this does NOT do.** `banked_game`'s crossing region is still unbounded — but for a different and
    unrelated reason, now stated: its bank-1 loader uses an `iny`/`cpy #8`/`bne` trip count this prover does
    not recognise, so the refusal moved from "region can switch banks" to "loop bound unknown", with a
    conditional obligation naming *the loop at bank 1 `$F00A`*. Its other unbounded region
    (`KRow+0`, "WSYNC inside loop body") is untouched. `BeamIntervals` and `Lint` still decline a
    bank-switched image rather than presenting bank-0-only windows as the cartridge's. `foldLoops` still
    refuses a region containing more than one back edge, which a real cross-bank kernel with a loop in each
    bank will hit.

- **`cyclebound` closes its decode over bank switches** (SD-8b, stage 2 of bank support). Stage 1 decoded each
  bank only from its OWN reset/NMI/IRQ vectors, but a worker bank is entered by the trampoline that switched
  to it. Measured residue, executed `(bank,pc)` pairs absent from the decode: **`litmus_bank` 4 of 36**
  (bank 1 `$FF03/$FF05/$FF07/$FF09`), **`banked_game` 1 of 61** (bank 1 `$FF83`), `litmus_bank_f6` 0 of 41,
  `litmus_bank_f4` 0 of 57 — **now 0 on all four**.
  - An instruction whose memory access reaches a bank-switch hotspot continues at the FOLLOWING address in
    the bank that hotspot names, and since each bank is already analysed as its own 4K image, that address is
    just another decode entry point: **no `map[uint16]Instr` key changes**. A read switches as a write does
    (`lda $FFF9`), targets fold through `cartHotspotKey` so `$FFF9`/`$1FF9`/`$3FF9` are one hotspot, and the
    fixpoint (seeding B can reveal a switch in B that seeds C) closes in **2 rounds on all four ROMs** against
    a cap of 8, with `cross_bank_seed_capped` so a capped run cannot read as a closed one.
  - **The target bank is parsed from the mapper's own symbol, never guessed.** `emu.BankSwitchHotspots()`,
    measured: F8 `$1FF8=BANK0 $1FF9=BANK1`, F6 `$1FF6..$1FF9`, F4 `$1FF4..$1FFB`. The whole symbol must be
    `BANK<digits>`, because Parker Bros publishes `B0S0` (bank-in-segment) and M-Network `RAM0` (cartridge
    RAM), for which "the same address in the other bank" is not where execution lands; those are reported in
    `unresolved_hotspots` and seed nothing. An access whose target cannot be resolved at all has no symbol
    and no bank, so it is counted in `unresolvable_switch_accesses` rather than guessed.
  - **This improves the DECODE, not the flow model.** `hotspotRefusal` is unchanged, `UnmodelledSwitches`
    still gates `Certified`, and all four bank ROMs still report `unmodelled_switches: 1, certified: false`.
  - New report fields (`cross_bank_seeds`, `cross_bank_seed_rounds`, `cross_bank_seed_capped`,
    `unresolved_hotspots`, `unresolvable_switch_accesses`, `bank_coverage[].seeded_entries`) are all
    bank-only. **Golden diff over 31 technique + 12 `cb_*` litmus ROMs: 42/43 byte-identical**, the one
    change being `banked_game`, the only banked image in the set.
- **`framegen` follows a sprite that moves down the frame — a zone-structured kernel** (RL-8b, the last RL-7
  limit). It carried one reset X per object and strobed RESxx once; it now emits a replay loop per zone with
  RESxx/HMOVE placement in the target's own blank lines. **`zone_multiplex` 380 → 0 cells, pixel-exact**
  (BG 33808/33808, P0 228/228, P1 204/204).
  - **Scope is measured, not assumed.** The extractor records the per-line reset X as a series and folds it
    into bands with the gap before each. A boundary costs one scanline per object placed + 1 HMOVE line + 1
    replayed blank line: `zone_multiplex`'s six bands per player have gaps 11/9/9/9/9 against a need of 4 and
    fit; `dyn_multisprite`'s P0 changes 48 → 78 at line 142 with **gap 0** and `road`'s M0 takes 27 bands of
    which 25 have gap 0, so both are refused with the counted reason instead of approximated.
  - **Five of the eight "placement differs" ROMs were never a per-zone problem** — `rts_dispatch`, `bitmap48`,
    `score6`, `text12`, `text24` all measure **1 reset X per player**, and `hscroll` draws no player at all.
    RL-7's "the 8 share one cause" was wrong about them.
  - Three defects found by measuring the clone: the prologue's div-15 `SetXPos` needs `k=11` to reach reset X 4
    and then spends TWO scanlines (263-line frame, 72 cells wrong that were not a positioning error) — replaced
    with a branch-free fixed-cost block strobing at `2n+3`; the reset marker is up to a pixel from the drawn
    window (same 8-px line reads X 49 span 49..56, X 117 span 116..123), so both sides are now anchored on the
    leftmost drawn pixel (12 cells); and the historical block order puts `GRP1` at visible clock +37, too late
    for a band at X 4 (64 cells).
  - **Regression gate held:** 22/31 technique ROMs + Outlaw and Combat (with and without `-reset`) pixel-exact,
    **262 scanlines on 35/35 runs**, `cyclebound certified:true` on 35/35 (74/76, the zoned one 66/76), and the
    generated source **byte-identical on all 34 non-zone runs** (`dyn_multisprite`/`road` gain only the new
    `NOT REPRODUCED: per-zone X` note).

### Fixed
- **`framegen` printed a cause it had not measured, and replayed a single frame-final NUSIZ** (RL-7c). On
  `roms/litmus/litmus_nusiz_all.bin` it reported 2666 of 34240 cells wrong and explained them with a fixed
  sentence — *"this is placement, not omission (one X per player cannot follow a per-zone multiplexed
  target)"* — printed unconditionally on every non-exact run. That ROM places both players once before the
  frame loop and never moves them (measured: **1 distinct reset X each**, over 191 and 190 drawn lines), so
  the sentence was false there and was never evidence anywhere.
  - **The obvious suspect was falsified first.** `nusizWidth` returns 1 for the five NUSIZ COPY modes, which
    looks like a bug and is not: the copies are hardware replication of the same 8-bit byte. Eight probe
    ROMs, one per mode held CONSTANT for a whole frame, reproduce **pixel-exact in all eight** (P0 cells
    864/864, 1728/1728, 1728/1728, 2592/2592, 1728/1728, 1728/1728, 2592/2592, 3456/3456 for modes 0..7).
    The real cause: `extract` read NUSIZ once at the end of the rendered frame, and litmus_nusiz_all ends in
    mode $07, so all 214 lines came out quad-width.
  - **Diagnosis.** Per visible line, for each player, the extractor now measures the NUSIZ in force, the reset
    position and the number of separate runs `DecomposeRow` reports — and takes the same measurement off the
    **clone**. The RESULT line names a cause only when the number proving it was counted: copies (`the target
    orders up to 3 copies (NUSIZ $06 on 37 lines) and the clone draws up to 1`), multiplexing (**only** when
    distinct reset X > 1, listing the positions and their line counts), or a late write (the kernel's own
    store landing past the object's leftmost pixel, arithmetic on the emitted block schedule). When none is
    measurable it says so. The NUSIZ size shift is removed before counting positions — HmovedPixel moves ±1
    on a 1x↔2x change without the sprite moving (measured: 24 for modes 0,1,2,3,4,6 and 25 for 5 and 7).
  - **Reproduction.** A varying per-line NUSIZ is now replayed from a table. Room is made by dropping
    playfield writes the target provably does not need, decided per PF register (both halves 0 on every line,
    or right half equal to left on every line). A left write is never dropped alone.
  - **Result: 2666 → 2 cells** (P0 3616/3616, P1 3120/3120). The 2 are reported, not absorbed: the target
    clears GRP1 part-way along scanline 228 leaving a 10-pixel P1 run, and a kernel writing each register
    once per line in HBLANK can draw only 8 or 12 there at quad width.
  - **Where it gives up, with numbers.** `rts_dispatch` would need 9 write blocks. Nine run on hardware
    (3+7·9+7 = 73 of 76; the 9-block clone measured 262 scanlines and 376→8 cells), but `cyclebound` bounds
    `lda abs,y` at 5 because it cannot assume the tables' `align 256`, scoring it 82 against 76 and refusing
    to certify. The kernel is capped at the certifiable 8 blocks and the tool reports what it dropped and why.
  - **No regression.** With NUSIZ constant down the frame — 30 of 31 technique ROMs, Outlaw and Combat — the
    historical eight-block layout is emitted unchanged (verified by diffing the generated sources; only the
    new per-block deadline comments differ). Corpus after: **21 pixel-exact / 8 differ / 2 partial, 262
    scanlines on 31/31**, every cell count identical to before; Outlaw and Combat still pixel-exact with and
    without `-reset`.
- **`prove_line_budget` called VBLANK-time code a visible-line tear whenever a subroutine had two call
  sites** (v1.115.0). Found while running a generated clone through the prover: the ordinary two-sprite
  shape — both players placed through one shared `SetXPos` — came back `certified:false` with the routine
  classified `visible`, while the *identical kernel with one call site* certified. That is the shape every
  two-sprite kernel has, including this project's own Outlaw and Pizza Boy builds.
  - Cause: `absSuccessors` reset a JSR's return point to full Top, discarding facts the callee cannot
    change. VBLANK went unknown at the *second* call site, that unknown flowed into the shared subroutine,
    joined with the known-on state from the first, and the routine's own entry state came out unknown.
  - Fix: keep VSYNC/VBLANK across a call when the callee provably cannot write them. The rule is one-sided —
    an unresolvable store, a push whose SP range can reach $0100/$0101 (page 1 mirrors the console's own
    addresses, so the Stack Trick is a real display write), or a nested call it has already visited all
    answer "not preserved". Indexed stores are resolved through the index range, which needs ranges that do
    not exist on the first pass, so `computeStates` now runs twice; the second pass only adds a fact
    justified by a sound first-pass range.
  - **A second, pre-existing hole fell out of the litmus**: `regionTouchesDisplay` tested the raw operand, so
    it saw only non-indexed writes — `sta VSYNC,x` is AbsoluteX and returned no address at all, letting a
    region that writes VBLANK be skipped as blank. Both checks now share one resolution path.
  - Corpus effect (31 technique ROMs): three false positives removed (`game_states` now certifies;
    `bullets` ×2 and `sfx_demo` reclassified to blank — each verified by reading the call context, e.g.
    `bullets` calls `PosObj` twice between VBLANK-on and VBLANK-off), and one region moved the *conservative*
    way (`rts_dispatch`, 55 ≤ 76, no violation) because indexed stores are no longer invisible to the check.
    The real 89-cycle interval in the generated clone is not lost — it moves to `blank_over`, i.e. frame-line
    drift rather than a torn line, which the `ntsc_frame_lines` check owns.
  - Graded against the machine, not against itself: new `blankclass_test.go` runs every corpus ROM and asks
    the television (`GetLastSignal().VBlank`, via new `emu.DisplayOff()`) whether the beam is really blanked
    each time execution reaches a blank-classified region's opening WSYNC — **129,936 executions across 31
    ROMs, 0 disagreements**, with the 1 never-reached region reported as not covered. Negative control:
    forcing `displayOff()` to true makes it fail with 28 disagreements, so the test can fail.
  - New twin litmus `roms/litmus/litmus_jsr_display.asm`: three routines of identical shape differing in one
    store (`sta COLUP0,x` / `sta VSYNC,x` / `sta VBLANK`), all called from the same place with the same index
    values, so a rule that answers the same way for all three is wrong whichever answer it gives.

### Changed
- **`framegen` now reports what it did NOT reproduce** (v1.115.0, audit RL-7). The field evaluation of
  `framegen` found the generator sound but the *report* misleading: its only output was a single
  `element match N / 34240` line, which on Fishing Derby read **96.9%** followed by `wrote clone.asm` — while
  the fisherman was 11% correct (P0 75/665 cells) and the hook and line were absent entirely. Background is
  77% of the visible area, so it carries the headline number and buries everything the reproduction is for.
  The cause is structural, not a tuning error: the emitted kernel writes PF + GRP0/GRP1 and **no
  `ENAM0`/`ENAM1`/`ENABL` at all** (`grep -c` over every generated clone: 0), and it carries one X per player
  for the whole frame, so a per-zone multiplexed sprite cannot be followed.
  - Per-element coverage is now measured against the clone's own rendered frame and reported in three places:
    the terminal, a `; NOT REPRODUCED:` block burned into the generated `.asm` banner (the file outlives the
    terminal it came from), and the **exit code** — 1 when incomplete, matching `vismatch`/`behavmatch`.
  - Structural absence (`clone 0` cells) is reported separately from misplacement (`clone > 0`, wrong cells):
    different causes, different fixes.
  - Field results, `-frames 28`: **21/31 technique ROMs pixel-exact**; 8 misplaced (`zone_multiplex` loses 190
    cells per player); 2 missing elements (`shared_setxpos` M0 1712 / M1 1712 / BL 428, `road` M0/M1/BL).
    Cartridges: **Outlaw and Combat pixel-exact, Fishing Derby partial.** Verdict recorded: a sound
    **BG/PF/P0/P1 validator for single-position kernels**, not a whole-frame reproducer.
  - Also fixed: a tie in the vertical-shift search resolved to the first candidate scanned, so "no offset
    explains anything" came out as "shift the picture up 4 lines" (`motion_glide` scored 34232 at all nine
    offsets and chose −4). Ties now resolve to 0.
  - Follow-up filed as **RL-8**: missile/ball replay and per-zone sprite X.
- **`framegen`: the last visible scanline no longer loses its sprites, and generated frames are 262 lines**
  (v1.115.0, audit RL-7b). Both faults were found by running the generated clone through `cyclebound` and
  `beamtrace` instead of only looking at its picture — a pixel comparison structurally cannot see either.
  - `cyclebound` put the `Kern` region at **97 cycles against a 76-cycle budget**; the loop body is 66 and the
    missing 31 are the loop-exit cleanup falling through *before* the next WSYNC. `beamtrace` on the clone
    shows it landing at `clk +133 GRP0` and `clk +142 GRP1` on the last visible line, so a sprite pixel right
    of clock 133 survives 213 lines and vanishes on the 214th. Fixed with a `sta WSYNC` before the cleanup;
    the clears now land in the next line's HBLANK (clocks −53..−17) and `Kern`'s worst drops **97 → 66**.
  - Proving it needed two litmus attempts, and the failed one is worth recording: a full-width *playfield*
    exposes nothing, because PF2 — the only PF register covering clocks 128-159 — is cleared after the line
    has ended. Only GRP0/GRP1 are early enough to bite. New `roms/litmus/litmus_lastline.asm` parks a player
    near the right edge of every visible line instead, sized to fill the 214-line snapshot window so the last
    extracted line is a drawn one.
  - Frame length: the pre-fix generator emitted **267 scanlines on 30 of 31 corpus ROMs and 268 on the other —
    262 on none**, five to six lines out of NTSC spec, which rolls on a real television. Invisible to every
    existing check because the *picture* was pixel-exact. Overscan ignored `vblankAdj`, and more
    fundamentally no formula can be right: `SetXPos` is a div-15 subtract loop, so a player far to the right
    costs more prologue than one on the left (Combat, P1 at clock 145, spends one line more than Outlaw).
    Frame length now **self-calibrates against `StepFrame()`** like X and VBLANK already did, is reported every
    run, and exits 1 when wrong. **After: 262 on 31/31**, pixel results unchanged (21 exact / 8 misplaced /
    2 missing). Locked by `roms/litmus/scenarios/lastline.json`.

### Added
- **Static program analysis — def-use, proven beam windows, conditional bounds, and the tools to check them**
  (v1.114.0). A night of work whose theme turned out to be that several existing tools were answering
  confidently and wrongly, and that only the machine could say so.
  - **`defuse` (MCP + `internal/cyclebound`)** — which instruction writes which address, over ALL paths, per
    WSYNC-to-WSYNC region, with may/must separated. Targets resolve through the EFFECTIVE address, so an
    indexed store is attributed to the register it reaches and a push lands wherever SP points. Also reports
    reads of RAM no path from reset has definitely written. Soundness is graded against the emulator:
    9055/9055 observed (pc,addr) pairs inside their predicted sets across the corpus.
  - **`beam_intervals` (MCP + `internal/cyclebound`)** — the forall version of `beamtrace`: every TIA write
    with the earliest and latest beam clock it can land at. 7117/7117 observed writes inside their proven
    window; 327 bounded writes, 106 exactly positioned, mean window 8.7 colour clocks. Nothing in the 2600
    ecosystem computes this; the state of the art is hand-counting one path.
  - **Conditional cycle bounds** — of 29 unbounded regions, 15 fail only because a loop's trip count is
    unknown, so the largest count that fits the budget is computable: *"within 76 cycles provided the loop at
    $F126 runs at most 11 times"*. Checked for tightness by re-deriving both edges. Never certifies.
  - **Stack-pointer tracking** — `TXS/TSX/PHA/PHP/PLA/PLP/JSR/RTS`, so the 2600 stack trick (SP aimed at a
    TIA register, PHA as the store) is visible to both the static and dynamic sides for the first time. They
    now agree to the clock.
  - **Call-context resolution** — a region opened by a WSYNC inside a subroutine is analysed once per call
    site and the worst taken; unbounded regions 29 -> 24.
  - **Sweep-loop recognition** — the `ldx #$FF / sta $00,x / dex / bne` idiom is a must-write of its swept
    range at the loop EXIT (minding the fencepost: `bne` leaves before storing at index 0). Uninitialised-read
    false positives 3783 -> 0 while the planted case still fires.
  - **Raw `.bin` support** — `program.canon` folds an address to a cartridge offset through the memory map,
    so a 2K cartridge decodes at every mirror the console sees it at.

### Fixed
- **`ProfileLineWorst` missed 44% of WSYNC strobes** (v1.114.0). It detected a strobe by the CPU stalling,
  and a WSYNC whose stall is shorter than one instruction step never shows that transition — so the interval
  it should have closed ran on to the next visible strobe. Measured: `$F0D0` in bitmap48 executes 192 times
  in 8 frames and 108 intervals were counted; the longest bogus interval spanned 13 lines and 987 cycles,
  which made the cycle prover look unsound. Detecting the WSYNC store fixes it (restricted to steps that
  retired an instruction, since `LastResult` is unchanged during a stall). Now 184 = 192 minus the 8 dropped
  at frame boundaries, worst 87 cycles over 2 lines, inside the proven 93.
- **`LastTIAWrite` attributed indexed stores to the base register** (v1.114.0). `sta COLUP0,x` was reported
  as COLUP0 whatever x held. On our own `shared_setxpos` kernel, five objects collapsed into player 0 over
  two frames; they now separate correctly. New litmus `litmus_indexed_tia.asm` turns the background green
  through an indexed store, so the screen arbitrates.
- **The abstract interpreter was not sound** (v1.114.0). An indexed or indirect store left previously-tracked
  cells standing, so a later load read a stale value into loop bounding and branch refinement — a "proven"
  worst case could sit BELOW the machine's. Stores now kill their may-set. No verdict changed on the 31
  technique kernels, so past certificates stand.
- **A capped fixpoint was silent** (v1.114.0). `computeStates` stopped at its iteration cap without saying
  so; nothing derived from an unconverged run may certify.
- **`cover` divided by branches OBSERVED** (v1.114.0), so an unreached branch left the arithmetic and the
  percentage rose as the test got worse. `divtable` reported 100% edge coverage with 12 of its 17 branches
  never executed. It now divides by the branches the program has, names the unreached ones, and says when
  the decoder itself is incomplete.

### Decisions
- **Check the instrument before believing what it says about the thing under test.** Twice in one night a
  faulty `ProfileLineWorst` nearly cost something real: it made the cycle prover look unsound (recorded as
  the top open defect, then retracted), and it caused a CORRECT improvement to be reverted as unsound
  (text12/text24 measured 143 against a proven 110; with the profiler fixed the same region measures 104).
  Both errors have the same shape. Recorded in `docs/capability-gap-audit.md`.
- **A detector is only worth its reports if it can stay silent.** Uninitialised-read detection was written,
  measured at 3783 false positives on one kernel, and deliberately NOT shipped until sweep-loop recognition
  brought it to zero — with a pair of litmus ROMs differing in one operand to prove both directions. The
  same rule sent an SMC detector back to the backlog: zero stores land on decoded code across 31 kernels and
  two commercial cartridges, so there is nothing to demonstrate it on.
- **`ramtrace` + the RAM-equivalence gate — the measurement half of behavioural reproduction** (v1.113.0).
  `vismatch` asks whether a build LOOKS like the target and `behavmatch`'s trajectory diff asks whether it
  MOVES like it; this answers the prior question — what the machine's 128 bytes of state are doing — so a
  commercial game's logic can be re-authored one rule at a time and each rule gated numerically.
  - **`emu.CurrentRAM`** reads all 128 bytes ($80-$FF) in one call. The point is not speed: it removes the
    need to DECLARE which addresses are interesting, which is precisely what is being measured.
  - **`emu.StartFrameWatch`/`FrameWatch`/`FrameWatchSPRange`** accumulate, inside the frame, every collision
    that OCCURRED (via the per-videocycle event, independent of `CXCLR`) and the range the stack pointer
    travelled. Observation-only, proven not to change a RAM byte or a cycle count.
  - **`cmd/ramtrace`** — `record` (full per-frame series + held input + collisions + SP range, as
    provenance-stamped JSON), `activity` (per-byte descriptive statistics, fitting nothing), `arity` (the
    smallest feature set that determines each byte's next value, with the LOCATIONS of any contradicting
    transitions and `-skip` to separate power-on initialisation from gameplay).
  - **`behavmatch -ram-gate`** reports the first frame and address where a build's RAM stops matching the
    target's — a debugging address instead of a downstream symptom. It compares a mask, never all 128
    bytes, because two correct implementations legitimately differ in scratch and leftovers; every verdict
    prints what was excluded and why, and a pass over nothing is labelled VACUOUS.
  - **Scenario library rewritten** as ROM-agnostic scripts covering both players, tap-vs-hold fire,
    diagonals, aimed fire, simultaneous fire, a 900-frame duel and the console switches. Scripts can no
    longer name a game variable, so they can no longer name a wrong one.
  - **`internal/version`** is now the single source of the harness version (it had drifted between the
    CHANGELOG and the MCP serverInfo twice; a tool that stamps a wrong version into a provenance block makes
    its artifacts untraceable).
  Docs: `docs/reproduce-loop.md`.

- **`framegen` — from-scratch full-frame reproduction generator** (v1.112.0). Reads a target ROM and emits
  a NEW, self-contained DASM source that reproduces its static visible frame **pixel-exactly** — including
  the players. It renders the target, reads which TIA object drew each pixel per visible scanline
  (`emu.DecomposeRow`), re-encodes the playfield into left/right PF register bytes and the two players into
  GRP0/GRP1 bytes, reads colours/NUSIZ/positions, and writes a data-driven per-scanline PF(L/R)+GRP0/GRP1
  replay kernel. Then it **self-calibrates** three things by assembling + rendering its own output in a loop:
  the two sprite X inputs (`SetXPos` landing offset is kernel-specific), the VBLANK line count (clone's
  visible top matches the target's), and a residual content vertical shift (±lines, chosen by element-match).
  Validated: on Outlaw it produced `clone/outlaw_clone.asm` that `vismatch` reports **pixel-exact (band diffs:
  none)** across all 214 visible scanlines — gunmen (2×-wide, P1 reflected), asymmetric cactus, score, bars,
  borders — with the target's exact TIA colours. `go run ./cmd/framegen -rom Outlaw.bin -reset -out clone.asm`.
- **`vismatch` + `behavmatch` — the automated reproduction-diff loop** (v1.111.0). Two CLI tools that
  close the "reproduce a commercial screen/mechanic pixel- and behaviour-exact" loop so a builder never
  again sparse-samples, mis-measures a band boundary by 1-2px, and iterates by hand. Both diff a TARGET
  ROM against your build.
  - **`cmd/vismatch` (`internal/vismatch`)** — PALETTE-INDEPENDENT visual diff. Renders both frames, reads
    WHICH TIA object drew each pixel (`emu.DecomposeRow` → BG/PF/P0/P1/M0/M1/BL) on every visible scanline,
    and reports every element-level difference plus a per-element **band diff** naming the exact scanline
    range and lit clock-spans where shapes disagree (e.g. `PF 162-165 | target 80-83 | mine 72-83` — a
    fat-by-4px playfield bar, pinpointed in one pass). `-diff` writes an object-attribution overlay PNG
    (green=match / red=target-only / blue=mine-only). `-genpf` **auto-generates the correct playfield tables
    from the target**: measures the cactus/PF bands and emits paste-ready `CACTOP/CACBOT` + `CacLTbl/CacRTbl`
    `ds` runs (validated: reproduced Outlaw's hand-derived cactus tables exactly). Palette independence is
    the point — two ROMs use different palettes, so object attribution, not RGB, is the ground truth.
  - **`cmd/behavmatch` (`internal/behavmatch`)** — behavioural diff. Drives both ROMs through identical
    scripted input scenarios (`internal/behavmatch/scenarios.go`: 4-way walk speed/clamps, fire→freeze
    coupling), records every object's per-frame trajectory (`emu.ObjectYExtent`/`Markers`/`PeekRAM`), and
    reports where a MECHANIC diverges as numbers — separating speed/travel-span (mechanic) from absolute
    rest position (calibration), plus a "no-Getaway" frozen-while-bullet coupling check. On Outlaw it
    confirmed horizontal (0.5px/f) and vertical (4px/4f) speeds match and surfaced two real build bugs
    (left-walk range too small; a fire-while-right bullet-trajectory divergence). **`-target-warmup N` /
    `-mine-warmup N`** (RL-5, field-driven): run N no-input frames before the scenario to skip a title
    screen that auto-advances to gameplay — without it a title-advance game (Pizza Boy) is measured on its
    title, not gameplay. Field evaluation of the whole loop + a 7-item enhancement backlog (RL-*) live in
    `docs/capability-gap-audit.md`.
  Both are thin layers over existing `emu` primitives (`New`/`LoadROM`/`RunFrames`/`SetInput`/`SetPanel`/
  `Snapshot`/`ReadRow`/`DecomposeRow`/`ObjectYExtent`/`Markers`/`PeekRAM`) + `build.Assemble` (accept `.asm`,
  auto-build). Docs: `docs/reproduce-loop.md`.
- **`decompose_row` — per-pixel TIA-object attribution of a scanline** (v1.110.0, AT-5). The attribution
  sibling of `read_row` (colours) and `beamtrace` (register writes): decomposes one visible scanline into
  run-length runs `{clock,len,element}` where element ∈ `{BG,PF,P0,P1,M0,M1,BL}` — answers "is THIS part of
  the picture the playfield, a player, a missile, or the ball?". The decisive tool for reverse-engineering how
  a running commercial ROM composes its screen (which TIA object draws which visual element, per line).
  Demand-driven while decoding Outlaw's asymmetric cactus: `read_row` showed `clk72-75 + clk80-83` lit but not
  that BOTH are Playfield (repeat-mode mid-line PF rewrites), and that the sprite/missile budget is spent
  elsewhere (gunmen at the sides, disjoint from the centre cactus). Implementation: Gopher2600's
  `reflection`/`video.Element` already computes per-pixel attribution but was unplumbed — `emu.EnableElementCapture`
  drives a per-color-clock callback (`VCS.Step(elemCB)`) recording `TIA.Video.LastElement` into `elemBuf`
  indexed by `signal.Index` (full 228×scanline space; visible clock x → `scanline*228+68+x`, same mapping as
  `ReadRow`); `emu.DecomposeRow` RLE-encodes it. Capture is on by default (observation-only — never changes
  colours/cycles; overhead is one array write per color clock). New: `emu.ElemRun`, `emu.DecomposeRow`,
  `emu.EnableElementCapture` + the `decompose_row` MCP tool. Same absolute-scanline coordinate as `read_row`.
- **`spritey` — numeric vertical (Y) position of a TIA object** (v1.109.0). Reports an object's drawn
  scanline extent (`y_top`/`y_bot`/`height`, grid-y) + X, found by matching the object's OWN colour at its
  X column — filling the gap `read_tia` (X only) and `read_motion` (rendered-top, which latches onto the
  playfield border for a 1-2px missile) leave. `frames=1` reports the current frame; `frames>1` advances and
  returns the per-frame Y trajectory — **tracing a bullet's ricochet as numbers** (`y_top` rises then falls at
  each top/bottom bounce). Surfaced demand-driven while observing Outlaw's signature ricochet: `read_motion`
  reported the bullet's Y as a constant 65 (the border) vs the true ~85. New: `emu.ObjectColorRGBA` (palette
  map via `capture.colorRGBA`→`Spec.GetColor`) + `emu.ObjectYExtent` (colour-matched column scan) + the
  `spritey` MCP tool + `TestObjectYExtentTracksBall` (a glide must descend, non-vacuous). Caveat: colour-match
  widens the extent when a same-colour object overlaps within ~8px (a just-fired missile over its player)
  until they separate. read_motion untouched (no regression). `go test ./...` green; MCP smoke on Outlaw green.
- **`docs/integration-density-playbook.md` — the composition/integration skill, distilled** (v1.108.0). The
  design-time reference for the *fit* problem (2 KB ROM · 128 B RAM · 76 cy/line · 262 lines interlocking).
  Distilled from a broad cross-domain research pass (demoscene/size-coding · WCET/embedded real-time ·
  deliberate-practice science · software product-line engineering · systemic game design) and **adversarially
  filtered against the real 2600 budget**. Contents: 8 rated transferable principles (adopted/adapted), an
  explicit **kill list** (bytebeat/PCM, heavy runtime synthesis, generic packers, compile-time `#ifdef`
  variants, "≤76 is enough", GC/heaps — all rejected with the reason), a measurable **Density Scorecard**
  (functionality-per-byte · WCET-slack/line · RAM-byte duty · feature-count-per-K · kernel byte-density ·
  table-leverage · dead-weight), and a 6-rung **deliberate-practice ladder** with per-rung scorecard gates.
  Wired into `docs/authoring-protocol.md` step 1 (Retrieve); provenance in §F. `check_wiring` / `check_provenance`
  green. No binary/behavior change (docs + one authoring-protocol reference only). First distilled from the
  interrupted deep-research run's 71 cached results, then **reconciled against the completed run** (25 claims
  3-vote tested → 20 confirmed / 0 refuted / 5 infra-unverified): added the master-move framing, concrete
  anchors (Pitfall seed 0xC4 / ~50 B; Combat 27 / ~28 B), two scorecard axes (generation-ratio, data-share),
  an anti-gaming caveat, and **§G** (verify-on-harness list + the 3 open research questions).
  **Rung-1 self-verification done** (§G): the 3-color-clocks-per-CPU-cycle coupling (`trace_clocks`:
  Δclock = exactly 3 × cycles across a 2/3/4/5/6-cy mix; `spritepos` x=80 exact) and the BIT-absolute
  skip-next idiom ($2C, 3 B / 4 cy, A/X/Y preserved) both confirmed on-harness — the playbook's §1/§6
  now rest on measured ground, not citations.
- **PONG-C3 / VV-2b — per-line WORST cycle count + blank-region ∀ accounting** (v1.106.0). `prove_line_budget`
  used to SKIP every VSYNC/VBLANK/overscan ("blank") region — `analyzeRegion` returned `Worst=0` for them and
  `Prove` `continue`d — so its `certified`/`max_worst` covered only visible lines, and a blank WSYNC-region
  that overruns 76cy (which adds a scanline = frame-line drift / "screen dip" / roll) was invisible to the ∀
  proof, delegated entirely to the runtime ∃ `ntsc_frame_lines`/`max_line_budget`. Surfaced while auditing the
  sandbox Combat clone: its ÷15 coarse-positioner (worst **73cy**, hand-verified) and its overscan-AI lines
  (up to **179cy**) were bounded-or-unbounded internally but reported as `Worst=0`. Now:
  - **① blank regions are computed and reported** — new `Report` fields `blank_lines` / `blank_max_worst` /
    `blank_over` (worst > budget×@lines = roll risk) / `blank_unbounded`. The existing `certified` / `max_worst`
    stay **visible-only** (backward-compatible: no existing scenario/litmus verdict changes).
  - **② `; @amax N` annotation** (sibling of `@lines`) — declares the proven upper bound of a divide-loop
    accumulator, so a ÷N coarse-positioner whose input is a RAM byte (abstract range Top → previously
    "loop bound unknown") can be bounded. `determineBound` uses it when the abstract range is Top.
  - **③ `roll_free`** — the ∀ roll-freedom verdict: EVERY region (blank AND visible) is bounded AND within its
    budget×@lines span. Stricter than `certified`; a blank overrun or an un-`@amax`'d divide loop makes it
    false, honestly (vs the old silent `Worst=0`). Litmus: `cb_blank_amax` (annotated → `roll_free`) /
    `cb_blank_noamax` (identical, unannotated → the blank divide loop is honestly `blank_unbounded`), test
    `TestProveBlankRegionAmax`. Complements the runtime side `emu.ProfileLineWorst` (∃ measured per-line worst,
    blank lines included) + the static `Report.Lines` complete per-region table. **Requires MCP reconnect** for
    the new report fields to surface through the `prove_line_budget` tool.
- **`read_audio_trace`** (v1.104.0) — trace the TIA audio registers (AUDC control / AUDF freq / AUDV volume)
  for both channels over N frames, returning the per-frame `control[]/freq[]/volume[]` time-series. The audio
  analog of `read_motion`: captures a whole sound envelope (a fire/explosion attack-decay, an engine pitch
  change) in one call instead of stepping frame-by-frame with `read_audio` by hand. ADVANCES the emulator N
  frames, so trigger the sound first. Motivated by the sandbox Combat clean-room sound pass, where capturing
  each of engine/fire/explosion took ~30 manual step+read_audio calls. `cmd/harness` handler +
  `AudioTraceOut`; smoke-tested (`initialize OK 1.104.0`). **Requires MCP reconnect** to become callable.
- **State snapshots + RAM-semantics probe** (v1.107.0) — three new MCP tools, `save_state` / `restore_state` /
  `probe_ram_semantics`, plus a `-snapshot` mode for `cmd/guidedfuzz`. Motivated by a study of
  [kisonecat/deep-atari](https://github.com/kisonecat/deep-atari) (a GAN that predicts the 2600 screen from RAM);
  the GAN itself was rejected, but two of its ingredients were worth taking.
  - **`save_state`/`restore_state`** (`internal/emu/state.go`) wrap Gopher2600's `hardware.VCS.Snapshot()` +
    `rewind.Plumb()` so the harness can branch-search: try N inputs or N RAM values from the SAME frame instead
    of replaying from `load_rom`. Slots are named and reusable; a snapshot costs ~3.9 KB (measured, 200 kept).
    ⚠️ **`television.Plumb()` does not touch the PixelRenderers** — `capture.Reset()` is never called on restore,
    so the framebuffer keeps the picture drawn on the *diverged* path (measured: hash matches the diverged frame,
    not the saved one). `State` therefore carries the framebuffer + crop rect + the CPU-cycle counters, and
    `TestSaveRestoreRoundTrip` fails if that copy is removed. Not covered, by design: video/audio digests,
    coverage and audio capture are append-only recorders and do not rewind.
  - **`probe_ram_semantics`** (`internal/emu/ramprobe.go`) answers "what is $XX?" for a ROM with no source:
    save → poke $XX=V → run `frames` → diff the frame against the un-poked baseline → restore, for every address
    and probe value; classifies from how the changed-region centroid travels (`x_position`/`y_position`/
    `appearance`/`none`). Non-destructive. Graded against litmus ground truth both ways (litmus_pos `$80`=DELAY
    → x_position; motion_glide `$80`=posY → y_position; unused addresses → `none`, no false positives) and
    audited against the published Combat disassembly (full sweep in 3.1s; `$A4`/`$A5` = TankY0/TankY1,
    `$DC` = KLskip, `$88`/`$A3` = GameOn/GAMVAR repaint everything, `$BE`-`$CC` even = the HIRES sprite buffer).
    Default `frames` is **3, not 1**, because Fishing Derby's score bytes reach the screen only after a
    BCD→digit-graphics conversion and were invisible at `frames=1` (measured 0 → 1 → 2 detected at frames 1/2/3).
    Known blind spot: a byte the kernel recomputes every frame before use reads as `none`.
  - **`guidedfuzz -snapshot`** reuses one emulator and restores a post-warmup snapshot per evaluation instead of
    reloading the ROM. Identical coverage signatures (`TestSnapshotEvaluatorMatchesReload`), and at `warmup=200`
    **~100x faster** (625.9ms → 6.2ms per evaluation; CLI end-to-end 23.80s → 0.86s at warmup=120, both
    markers=36). New `Coverage.Reset()` cuts each evaluation since restore does not rewind coverage.
  - Reference material: `reference/ale-ram-maps/` (umbrella, local-only) — RAM addresses for **104 commercial
    games** distilled from the Arcade Learning Environment's `RomSettings` sources, with provenance and the
    `(offset & 0x7F) + 0x80` address convention verified against ALE's `RomUtils.cpp`. Used as an independent
    answer key for `probe_ram_semantics`, not as a design input.
  **Requires MCP reconnect** to become callable.

### Fixed
- **Collision and stack sampling happened at frame boundaries, where both are already gone** (v1.113.0).
  Games clear `CXxx` every frame and SP is back at `$FF`, so boundary sampling could neither prove a game
  uses collisions nor tell which RAM the stack had trampled. Measured against a real cartridge, the SP
  low-water mark came out `$FF` on every single frame — excluding exactly zero bytes and silently turning
  the RAM gate's stack mask into a no-op. Watching inside the frame then invalidated the rule as well: the
  target's SP sweeps `$FF` down to `$1C` every frame (a `TXS` aiming at TIA register space), under which
  "exclude everything at or above the lowest SP" excludes all 128 bytes and the gate passes unconditionally
  while reporting green. A pointer descending past an address is not a write to it; stack exclusion needs
  write attribution and is not attempted until that exists.

### Decisions
- **The arity probe reports memorisation as memorisation.** A free-running frame counter takes a fresh
  value every frame, so keying on it "explains" every other byte perfectly — the first version of the probe
  reported that all of RAM had arity 1. Frame-counter-like bytes are now identified and tried last, and any
  resolution in which every key was seen exactly once is flagged `MEMORISING`: consistent with the data,
  and evidence of nothing about states the scenarios never visited. A model that only reproduces its own
  recording is the failure this whole system exists to avoid, so the tool has to be able to say so.

### Docs
- **Combat deep-read (round 2) absorbed** — a 5-lens pass (game-design/6502-craft/audio/anti-patterns/clone-novelties)
  over the original Wagner Combat.asm surfacing learnings the efficiency comparison structurally could not see,
  concentrated in **audio** and **design-intent**. `design-principles.md` gains a "Combat deep-read: design-intent,
  audio model & AI-nav primitives" section (difficulty=self-handicap-on-the-winner; curate+reskin content strategy;
  invisible-stealth self-betrayal; consequence-beat+board-reset; diegetic end-game UI; control-overload;
  **sound-priority = last-writer-wins on a 1-object-per-channel bus**; vector-slot-data 2K→4K trap; + the clone's
  emulator-verified **AI-nav primitives** (octant-seek overflow-guard, mod-16 shortest-arc turn, map-free
  stall/wall-slide/aim-gate/scatter-decoy) flagged PONG-capstone material). Two new **technique reference docs**:
  `techniques/audio-envelope-idioms.md` (counter-IS-the-audio-register, self-clearing SFX counter, per-player detune,
  gear-shift pitch curve) and `techniques/kernel-micro-idioms.md` (HMP low-nibble 2nd axis, `$FF`/`$00` AND-mask blank,
  −4 pointer bias, PF mirror via counter-EOR, compare-via-EOR A=0) — reference-only (reimplement+CI = TODO), wired into
  `techniques/README.md` (#30/#31). `casebook.md` gains two anti-pattern cases (unclamped-input-index UB; don't-cargo-cult
  a master's cruft). `capability-gap-audit.md` registers **CMB-4** (CMB-AUDIO temporal-audio assertion), **CMB-5** (MD5
  gap-fill $FF-PROM model), **CMB-6** (assembler warn on data over $FFFA-$FFFF), **CMB-7** (no-SMC calibration boundary).
  `check_wiring`+`check_provenance` green. Docs-only.
- **Combat (1977 Wagner 2K) structure/efficiency learnings absorbed** from the sandbox clean-room comparison
  study (`studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`), skipping what was already
  in the harness (stack-trick/two-line-kernel/score-kernel/div-15). `casebook.md` gains a **Combat** section
  (PF-only dual score via `CTRLPF #$02` + recycled PF1; multi-frame `MxPFcount` wall-normal bounce solver;
  `StirTimer` hit-reaction state machine; `VARMAP` 27-variant bit-packed selector + DDR input-gating) + an
  index row. `design-principles.md` gains 8 **integration-under-budget** rules (one `,X`-indexed path for all
  objects vs per-object inlining; time-sliced momentum; rotation-shape RAM precompute; interleaved single HIRES
  buffer; multi-duty phase-locked byte fan-out; `INTIM`/`TIM64T` VBLANK load-leveling vs fixed-count+pad;
  one-loop-four-ranges `ClearMem`; self-audit-your-own-cargo-cult). `capability-gap-audit.md` registers three
  candidates: **CMB-1** structural-efficiency lint (inline→`,X` in blanked time, ~250-400 B recoverable),
  **CMB-2** `INTIM` fixed-picture-start advisor, **CMB-3** collision-face/wall-normal estimation aid. `check_wiring`
  + `check_provenance` green. Clean-room recorded (disassembly read post-build; casebook contract). Docs-only.
- Knowledge captured from the sandbox PONG feel-pass (pf2-06, 2026-07-03): technique **#29 sub-pixel velocity
  (DDA error accumulator)** — fractional speed while the position stays a 1-byte integer
  (`docs/techniques/subpixel-velocity.md`); two `known-traps` rows (`bpl`/`bmi` clamp on a coordinate that
  legitimately exceeds 127 → use wrap-magnitude not bit7; immediate `ld_` clobbering N/Z between a flag-set
  and its branch → branch first or go branchless); backlog **PONG-C3** (per-line WORST cycle count, not just
  pass/fail — the highest-leverage tool gap the feel-pass surfaced). Docs-only; no tool/behavior change.
- More knowledge from the PONG serve-refinement + AI-variants work (2026-07-04/06): a `known-traps` row for
  **`cmp` clobbering Z between a load and a test-for-zero branch** (sibling of the immediate-`ld` clobber;
  hit as a real serve-clamp bug where `0→1` silently failed); a **range-dependent-threshold** note on the
  bit7-clamp trap (a `BallRow + 8·DY` lead reaches ~202 so the wrap threshold must be `#220`, not `#200`);
  and reinforcing evidence on backlog **PONG-C3** (building 3 swappable AI kernels hit the same
  guess-and-assert budget loop — design estimate 61cy vs real ~78cy — confirming it recurs on every
  budget-tight kernel). Docs-only; no tool/behavior change.
- Knowledge captured from the PONG AI-variants + objective-benchmark work (2026-07-06..11): two 6502 idioms
  in `design-principles.md` — sign-preserving ×2^n via repeated `asl` on two's-complement values (with the
  widened-range clamp caveat: bit7 stops being usable as a sign, so clamp by value range), and packed-BCD
  bytes comparing correctly with a plain `cmp` (binary order = decimal order; but binary *differences*
  overstate decimal ones across a digit boundary — bucket/saturate before using them as magnitudes). Plus a
  new in-house PONG section in `casebook.md`: the four classic paddle-AI paradigms with designed-in
  beatability, imperfection tuning via error/delay rather than speed, the exclusive-path shared-tail-skip
  (`jmp OverEnt`) budget-carving pattern, and the measured **non-transitivity of AI strength** (a
  single-baseline ranking refuted by a round-robin: the baseline's "strong" tracker is actually the weakest
  head-to-head). Docs-only; no tool/behavior change.
- Backlog **PONG-C4** registered (`capability-gap-audit.md`): gameplay-behavior verification — a headless
  match harness (declared actor interface + parameterized scripted opponent + match rules → per-pairing
  scores and an N×N tournament matrix) plus behavioral-invariant fuzz (speed bounds, score monotonicity,
  serve fairness). Generalizes the hand-built C1 bench/round-robin ROMs; the C1-measured non-transitivity
  is baked in as a design constraint (tournament matrix, not a scalar rank; opponent model = explicit
  parameter). Registration only — implementation is a separate approval.

## [1.105.0] - 2026-07-21

> **★backfilled 2026-07-30 from the commit diff.** This version was tagged (`aab7ab7`) and shipped with
> **no CHANGELOG entry at all** — the gap was found by counting 170 tags against 174 released sections.
> Everything below is read off the diff, which is fact. **The rationale is not recorded**: nobody wrote
> down why it was added at that moment, and inventing one after the fact would put a sentence in this
> file that nothing supports. Left blank deliberately.

### Added
- **`read_ram_trace` MCP tool** (`cmd/harness/main.go`, +61/−1, one file). Traces **1–16 RAM addresses**
  (`$80`–`$FF`) over **1–4000 frames** (default 60) and returns `traces[i][f]` — the per-frame value of
  each address, indexed from the call. Out-of-range addresses, an empty list, more than 16 addresses and
  a frame count outside 1–4000 are each rejected by name.
- It **ADVANCES the emulator** by `frames`, and input set with `set_input` persists across the trace —
  both stated in the tool's own description, so a caller is not surprised by the side effect.
- Purpose per that description: collapse a manual `step_frame` + `peek` loop into one call, to measure
  **as numbers** how a byte evolves — a tank's X/Y, an AI mode or timer, a score, frames-to-escape a
  region, a decay curve, a stuck oscillation.

## [1.103.0] - 2026-07-03

Interactive rollout of the two PONG-campaign capabilities (backlog PONG-C1/C2), live-proven on the real
PONG ROM: C1 rediscovers a byte-faithful replica of the historical 77cy 3-edge-coincidence bug
(fail at PFp1, 152cy) and passes the fixed kernel across 139 consistent alignments (offsets-coupled sweep);
C2 runs the whole lightweight-table budget ritual in one call with the original ROM verified byte-identical
afterwards. The rollout itself exposed and fixed a latent flaw in the budget guard (Fixed below). Also
includes the accumulated PONG-dogfooding items below (framesim normalization etc.).

### Removed
- **AtariAge fetch tooling relocated out of the repo.** `scripts/aa_fetch.py`, `scripts/aa_index.py`, and
  `scripts/aa_manifest.py` (the forum thread/index crawler — Wayback-first, with an optional cookie-based
  direct fallback) were moved to local-only research tooling outside the published repo. Rationale: it is
  ingestion scaffolding, not part of the verification harness (the deliverable), and a ToS-adjacent scraper
  has no reason to ship in the public engine. The distilled knowledge it produced (`docs/mining-digest.md`
  and the technique/casebook corpus) stays — that is the value; the scraper is not. `gen_mining_digest.py`
  remains (it only distills an existing local `MINED.csv`; it never fetched anything).

### Fixed
- **`RunUntilBudget` (assert_line_budget core) silently ate poked-state frames in its warmup — found by the
  PONG-C1 rollout (2026-07-03).** The unconditional 2-frame stabilization run consumed the very frame a caller
  had just poked into a worst-case alignment, so `poke → assert_line_budget` could NEVER observe a
  single-frame overrun — the historical PONG 77cy coincidence bug was un-reproducible by direct poke for
  exactly this reason (only persistent-state trajectories tripped it, by luck). The warmup now runs only on a
  fresh boot (Frame<2); mid-session calls start monitoring immediately. Ground truth: a byte-faithful 77cy
  replica (NetTbl load → 3×NOP, +2cy) with poked 3-edge alignment now reports over=true/152cy; the fixed
  kernel reports over=false. Locked by `TestBudgetGuardNoWarmupWhenRunning` (frames consumed must equal
  max_frames exactly when already running).

### Added
- **`assert_line_budget` temporary ROM patch (`patch`/`pokes` params) — PONG-C2 (v1.103.0).** During the PONG
  campaign every budget run required hand-editing the positioning table to lightweight values, assembling,
  asserting, restoring, re-assembling (~15×; one forgotten restore = shipping a wrong ROM). The tool now takes
  `patch: [{symbol|addr, bytes}]` — applied to a COPY of the loaded ROM (symbol resolved via the last
  `assemble_and_load` listing, new `srcmap.Symbol()`), fresh-booted for the measurement, and the original ROM
  is ALWAYS reloaded afterwards (deferred restore = the forget-to-restore failure mode is structurally gone).
  `pokes: [{addr,value}]` seeds RAM after the patched boot for trajectory reproduction.
- **`assert_edge_coincidence` — worst-path fuzz for edge-compare kernels — PONG-C1 (v1.103.0).** The PONG
  PlayF kernel hid a 77-cycle line that only fires when ALL edge variables (ball bottom + paddle top + paddle
  bottom) land on the SAME Y — free-run testing missed it for hundreds of frames (known-traps "N-edge
  coincidence", found 2026-07-02). The tool pokes every listed zero-page edge variable to one Y, runs
  `frames_per_y` frames under budget-guard semantics, sweeps Y over a range, and reports every failing
  alignment (`fail_ys`, first at/cycles). Optional `patch` (auto-restored) combines with a lightweight
  positioning table. Claim-level proof: rediscovers the historical 77cy bug on the pre-fix PONG binary,
  passes on the fixed one.
- **`framesim` scale-normalized comparison (`framesim.Resize` + `NormalizeSize`).** `framesim -a rom.bin -b
  screenshot.png` previously errored on a bounds mismatch (a 1× ROM render, 160×N, vs a 2× Stella screenshot,
  320×M), so a ROM could not be compared to a target screenshot. Both inputs are now downscaled (nearest-neighbor,
  per-axis min) to a common raster before SSIM/pHash, and the CLI reports the `normalized` size. Found and fixed
  during the PONG dogfooding campaign (it blocked the Phase-1 "framesim matches the target screenshot" metric).
  `TestNormalizeSizeRescales` locks it (an image vs its own 2× upscale scores ~1.0). Vertical-framing alignment
  (differing VBLANK/overscan margins) is handled separately by `-align` below.
- **`framesim` content-bbox alignment (`framesim.ContentBBox` + `NormalizeAligned` + `framesim -align`).**
  Scale-normalization alone still misaligned a ROM render against a screenshot whose lit content sits at a
  different vertical offset (the ROM's 214-row frame vs the target's 228-row frame put walls/net/scores on
  different rows), so the diff was dominated by spurious whole-row mismatches and was untrustworthy. `-align`
  first crops BOTH inputs to their lit-content bounding box (luma>128 = wall-to-wall, net-to-net) and only then
  scale-normalizes, so content is compared content-to-content regardless of margins. On the PONG campaign frame
  this took the diff from 3164→1078 mismatched px and SSIM 0.105→0.192 (the remaining diff is now REAL: score
  glyphs, net phase, paddles — not framing noise), unblocking the convergence loop. `TestNormalizeAligned` locks
  it (the same content at different positions in different-size frames aligns to ~1.0). Fix along the way: the
  bbox seed used `image.Rect(max,min)`, which sorts its args and collapsed the inverted seed back to full
  bounds (ContentBBox always returned the whole frame) — now seeded with plain ints.
- **`framesim` difference localizer (`framesim.Diff` + `framesim -diff out.png`).** SSIM gives one global score
  and the single worst 8×8 block; for the "reproduce a target screenshot" loop you need to see EVERY differing
  region. `Diff` classifies each pixel (match / A-only=red / B-only=blue) into a diff image and per-row stats,
  and the CLI prints the differing row-bands ("rows 37-59: 510 diff px" = a band the target draws that the ROM
  doesn't). Turns "compare → localize what's wrong → fix → repeat" into a measured loop. `TestDiffLocalizes`
  locks it (identical = 0 mismatch; a lit block over black = B-only localized to its rows).

- **`framesim` max-normalization (`framesim.NormalizeSizeMax` / `NormalizeAlignedUp` + `framesim -up`).**
  `NormalizeSize` downscales both inputs to the per-axis MIN; comparing a 1× ROM render to a 2× screenshot
  thus downscales the screenshot, blurring its thin features (net dash, glyph edge) so the SSIM/diff is *more
  forgiving* but fuzzy. `-up` instead rescales to the per-axis MAX — upscaling the ROM (nearest-neighbor, stays
  sharp) and leaving the screenshot native — for a sharp-vs-sharp comparison at the screenshot's resolution.
  Found during the PONG campaign while chasing the static-match residual: `-up` is the STRICTER, more honest
  metric (it doesn't blur real ~1px differences away), confirming the remaining residual is genuine fine detail
  (net-edge/phase, fenceposts), not a downscale artifact. `TestNormalizeSizeMaxUpscales` locks it (an image vs
  its 2× upscale max-normalizes to the 2× size and scores ~1.0).
- **`framesim` per-element ruler (`framesim.ContentRowSpans` + `Span`/`SpansEqual` + `framesim -spans`), and an
  `-align` height-mismatch warning.** The global SSIM/diff buries 1-row/1-px element errors (a score-bottom row,
  one net dash, a partial wall edge) — the PONG campaign proved it: the static frame read "done" at the global
  level while three elements (score bottom, ball squareness, paddle height) were each off by a row, and the fix
  hunt was slow because there was no standing tool to measure each element's exact extent. `-spans` prints, for
  every content-aligned row, the lit runs in CLOCK coords (clk = x / scale, so a 1× ROM and a 2× screenshot read
  on the same 0..159 axis) for A and B side-by-side, marking rows that differ — measured in each frame's OWN
  content crop at native resolution, so it keeps the screenshot's precision and sidesteps the resize that makes
  `-align` ±1-row sensitive. It is the exact ruler that complements the tolerant SSIM/diff: on the campaign frame
  it pinpointed exactly two differing rows (a net dash and the partial wall edge), including a net-dash gap below
  the diff's row-band threshold. The `-align` path now also warns when the two content heights differ (the
  resized diff then carries ~Δpx of edge noise → use `-spans`). `TestContentRowSpans` locks the clock-coord
  mapping at 1× and 2×. Replaces the ad-hoc per-row measurement script hand-built mid-campaign.

### Fixed
- **CI: run `go test -p 1 ./...` (serial package tests).** Several packages assemble/read the SAME shared ROM
  `.bin` fixtures (`roms/litmus`, `roms/techniques`) during their tests; under `go test ./...` (parallel test
  binaries) one process can truncate a `.bin` mid-assemble while another loads it → a flaky panic (e.g.
  `TestCoverageThroughRun` / `TestTrajdiffSelfTest`, "index out of range … length 0"). This was a long-standing
  test-isolation flake (also red on old commits like VV-9 @105932d), surfaced more often after the AT-* sprint
  added more assembling tests. Serial package tests fix it deterministically; the engine is single-emu in
  production, so this is a test-only concern. (No server-code change → server version unchanged.)

## [1.102.0] - 2026-06-18

### Added
- **MCP exposure of the interactive authoring aids (AT-5), batched into one reconnect.** Three new `cmd/harness`
  tools so the timeline/solver are usable in the authoring loop, not just the CLI:
  - **`beamtrace`** — write→visible-pixel timeline for the loaded ROM (per scanline, each TIA write's beam clock,
    register name/kind, value, and governed visible span). Advances the emulator.
  - **`beam_race`** — advisory beam-race map (per pixel-data write, object X + in-time/late). Factual, no verdict;
    paired with the existing scenario `checks.no_beam_race`. Advances the emulator.
  - **`spritepos`** — forward sprite-position solver (target X → routine input + decomposition + snippet +
    emulator-verified achieved X). Self-contained: builds its own calibration kernel, does not disturb the loaded ROM.
- `scripts/mcp_smoke.py` extended to call all three over stdio (smoke now covers beamtrace/beam_race/spritepos).

### Notes
- The static linter (AT-1, `cmd/timinglint`) stays CLI/CI-only — no MCP tool, by design (proactive source check,
  no live emulator state needed). **This release adds MCP tool schema → requires one `bin/harness` rebuild +
  client reconnect** (smoke-tested green at v1.102.0 first). Concludes the authoring-tools sprint (AT-1..AT-5).

## [1.101.0] - 2026-06-18

### Added
- **`cmd/spritepos` + `internal/spritepos` — forward sprite-position solver (authoring aid, AT-4).** Given a
  target X (0..159) it returns the routine input, the div-15-coarse / HMOVE-fine decomposition, a paste-able
  `SetXPos` snippet, and — the part that makes it trustworthy — the position the hardware **actually reaches**
  (HmovedPixel), measured by running the kernel. Built clean-room on the verified
  `roms/techniques/shared_setxpos.asm` idiom (div-15 coarse via RESPx strobe timing + remainder→HMOVE nibble by
  the `eor #7; asl×4` trick). Per CLAUDE.md the X(N) offset is kernel-specific, so `Solve` never trusts the
  arithmetic — it measures the offset against the emulator, inverts it, and re-runs to confirm. `-all` solves
  every X for an object; `-json` for tooling.
- **Self-tests:** `TestDecompose` (pure coarse/fine arithmetic), `TestAchieveSweepLog` (records X(A)),
  `TestSolveHitsTargets` (P0/P1/M0/BL × 7 targets land EXACTLY, emulator-verified), `TestAchieveDiscriminates`
  (a deliberately-wrong input must miss — the guarantee isn't vacuous).

### Notes
- **Measured: X(A) == A exactly across the whole range** for this calibrated routine (slope 1, offset 0), and
  `spritepos -object BL -all` lands **160/160 targets exactly**. Found + fixed a bug in the pure `Decompose`
  helper (loop-exit used bit-7 instead of the 6502 carry, so X≥128 broke immediately) — caught because the
  emulator ground truth disagreed with the math; the verified positions were never affected. Pure Go, CLI only.

## [1.100.0] - 2026-06-18

### Added
- **Beam-race / too-late-write detection (authoring aid, AT-3) — a SOUND dual, not a blanket detector.**
  `internal/beamrace` + scenario `checks.no_beam_race` + `cmd/beamtrace -race`, plus a thin `emu.ObjectX`
  accessor (player/missile/ball HmovedPixel). A write to an object's pixel-data register
  (GRP0/GRP1/ENAM0/ENAM1/ENABL) at beam clock C while the object sits at X reaches the beam in time iff C ≤ X;
  otherwise that line draws the previous value (a one-line lag).
  - **`cmd/beamtrace -race` — advisory report (automatic, factual, NO verdict):** per object, every pixel-data
    write with clock vs object X marked in-time / LATE. Cannot false-positive because it asserts nothing.
  - **`checks.no_beam_race` — verdict the author OPTS INTO:** `{object, line_from, line_to}` declares "object O
    must be updated before the beam on these scanlines"; the check fails on any late write. Sound because the
    intent is supplied, not guessed. Generalises the hardware-fixed `no_hmove_hazard` gate.
- **Litmus + self-tests:** `roms/litmus/beamrace_clean.asm` (P0 updated in HBLANK → in-time) and
  `beamrace_late.asm` (P0 graphics written deep in the visible line → one-line lag). `TestCheckEvalPure`,
  `TestBeamraceCleanPasses`, `TestBeamraceLateFails` (beamrace) + `TestBeamRaceScenario` (both directions
  through the scenario engine) + `roms/litmus/scenarios/beamrace_clean.json` in the regression set.

### Notes
- **Why no fully-automatic verdict (measured/reasoned, on the record):** whether a late write is a *bug* depends
  on author intent — the same late `sta GRP0` is correct when it pre-loads the NEXT line and wrong when meant for
  THIS line. Validated on the real `multicolor48` kernel: P0 at X=87, the 48px technique's right-side GRP0
  rewrites land at clk +139/+157 = "LATE" — **correct facts, not bugs**. An automatic verdict would
  false-positive there, violating the zero-false-positive bar; hence the advisory (no verdict) + opt-in check
  (intent supplied). A heuristic auto-detector is **deferred, not closed** (see audit AT-3) per the user's
  request to keep it on the books.
- Pre-existing flake noted (not from this change): `internal/trajdiff` `TestTrajdiffSelfTest` panics rarely under
  the fully-parallel `go test ./...` (a latent gopher2600 lazy-init data race); passes deterministically alone
  and under `go test -p 1`. Tracked for a later look.

## [1.99.0] - 2026-06-18

### Added
- **`cmd/beamtrace` + `internal/beamtrace` — write→visible-pixel timeline (authoring aid, AT-2).** Runs a ROM
  instruction-by-instruction and tabulates, per scanline, every TIA write with the beam clock it lands at and
  the visible-pixel span it governs — answering "where on the line does this `sta GRP0` actually paint?". The
  causal map the runtime tools (`trace_clocks`/`read_row`) only show piecemeal. States only what is sound: a
  write at clock C can affect a pixel only if rendered at clock ≥ C, and a later write to the **same** register
  supersedes it — so the governed span is `[C, next-same-reg-write)`. Register name+kind table
  (color/graphics/position/motion/control/audio/strobe); pure strobes (WSYNC/RESPx/HMOVE/HMCLR/CXCLR/RSYNC)
  report no value. New thin `emu.LastTIAWrite` accessor (same detection as `WatchHMOVEHazard`). Pure Go, CLI only.
- **Self-tests:** `TestTimelineSpans` (pure span logic on synthetic writes: ordering, same-reg supersede, HBLANK
  clamp, empty span when superseded in HBLANK) and `TestTraceGRP0Marker` (fixture `roms/litmus/beamtrace_grp0.asm`
  writes GRP0=$A5 once per frame → surfaced with right value/kind, localized to its scanline, deterministic).

### Notes
- Validated against the real `multicolor48` kernel: the timeline reproduces the staggered GRP0/GRP1 rewrites
  with correctly interleaved spans, and a write superseded during HBLANK correctly shows an empty `[0,0)` span.
- Interpreting whether a write is *too late* (the effect window is fully passed) is the next tool's job (AT-3
  beam-race detector); beamtrace only lays out the facts.

## [1.98.0] - 2026-06-18

### Added
- **`cmd/timinglint` + `cyclebound.Lint` — static TIA-timing linter (authoring aid, T1 of the authoring-tools
  sprint).** Reads a kernel and warns *before* you run it about high-confidence horizontal-motion timing
  pitfalls, complementing the runtime checks (`assert_line_budget`, VV-10 HMOVE hazard) by being proactive.
  Three rules, each validated both directions:
  - **`hmove-without-hmxx`** — HMOVE is strobed but no HMP0/HMP1/HMM0/HMM1/HMBL is ever written (the fine
    motion is always 0).
  - **`hmxx-without-hmove`** — a **provably non-zero** motion is staged but HMOVE is never strobed (the motion
    is never applied). Value-aware via the prover's abstract interpreter: a defensive `lda #0; sta HMPx` clear
    (proven 0) or any unknown/computed value never warns — only motion proven non-zero does.
  - **`hmove-hazard`** — an HMxx/HMCLR write starts <24 CPU cycles after an HMOVE on a straight-line path
    (motion undefined, Stella PG). The standard `sta HMOVE; ds 12,$EA; sta HMCLR` idiom (HMCLR at exactly 24cy)
    is correctly treated as safe.
- **Litmus fixtures + self-tests:** `roms/litmus/lint_r1_hmove_nohmxx.asm` / `lint_r2_hmxx_nohmove.asm` /
  `lint_r3_hazard.asm` (each fires exactly its rule) and `lint_clean.asm` (the canonical correct idiom, silent).
  `TestLintTrapsFire` / `TestLintCleanSilent` lock both directions; `TestLintNoFalsePositivesOnTechniques`
  is the corpus guard.

### Notes
- **Quality bar met (measured): zero false positives on all 31 known-good technique kernels.** The first sweep
  surfaced 6 apparent warnings, all run down to two detector gaps and fixed (the rules themselves held):
  `storeTIA` missed **indexed** HMxx stores (`sta HMP0,x` / `sta HMM0,y` — how shared positioning code stages
  several objects), and `hmxx-without-hmove` wrongly flagged a benign zero-clear (now value-aware). Also fixed a
  latent false-negative in the hazard cycle-accounting (the gap is now measured to the *start* of the HMxx write,
  so a 22-cycle `ds 11` gap is correctly flagged while the 24-cycle idiom is not). Pure Go, CGO-free; CLI only
  (no MCP tool — no reconnect).

## [1.97.0] - 2026-06-18

### Added
- **Value-range absint enhancements for the cycle-budget prover (VV-2 "array-range" arc).** Three sound,
  composable building blocks, each litmus-locked both directions:
  - **3A — AND/ORA #imm range.** `and #m` ⇒ [0,min(A.Hi,m)], `ora #m` ⇒ [max(A.Lo,m),255] (EOR stays Top); and
    `determineBound` now reads a divide loop's entry value from the fall-through predecessor's post-state (the loop
    header is polluted to Top by the final wrapping subtraction on the back-edge). Litmus `cb_andloop.asm`.
  - **3B — zero-page RAM array-element range.** `State.ZPVal` = join of all values stored to RAM ($80–$FF), seeded
    to the recognised clear value; an indexed RAM load (`lda arr,x`) returns it (sound over-approx of any element;
    $00–$7F TIA/RIOT excluded). Litmus `cb_arrloop.asm`.
  - **3D — ROM data-table value range.** An indexed load from a ROM address returns the table's actual byte range
    over the proven index range (constant data, read from the binary). Litmus `cb_romtable.asm`.

### Notes
- **Honest measured outcome: +0 real kernels (still 14/31 certified, 0 false-positive violations).** All three are
  sound and certify their clean litmus, but real kernels need a *cascade* of further precision: their loop counters
  and array indices are **loop-carried**, so the abstract interpreter over-approximates them to Top at the loop
  header (the dec/sbc wraps on the exit edge). Recovering the in-loop counter range (narrowing it on the loop
  branch) — and tight table extents — is the recurring root limitation; it is an open-ended precision tail with
  diminishing per-kernel payoff (a kernel only flips once its *entire* chain is closed). The building blocks are in
  for when that root fix is tackled. ②Z3 / ④external-ROMs remain inapplicable to what's left.

## [1.96.0] - 2026-06-18

### Added
- **Interprocedural cycle-budget proving (VV-14 2A).** `internal/cyclebound` now FOLLOWS subroutine calls
  instead of reporting "JSR in region — unbounded". `longest()` threads a single-level return address (memo keyed
  by `(addr, ret)`): a JSR descends into the callee with the return point threaded, an RTS/RTI returns to it, and
  the callee's own WSYNC remains a region sink. Sound by construction — a nested call or an RTS with no caller in
  context sets the region UNBOUNDED rather than under-estimating. Locked by `roms/litmus/cb_jsr.asm` +
  `TestProveInterproceduralJSR` (a JSR'd subroutine certifies; a tight budget flips the region with the callee on
  the worst path = its cycles are counted).
- **Divide-by-15 / sbc-counter loop bounding (VV-14 2B).** `determineBound` now bounds the coarse-positioning
  idiom (`sec; sbc #const; bcs/bcc`) from A's proven loop-entry range: iterations ≤ floor(Amax/const)+2. The
  entry bound comes from the closest immediate `lda #imm` before the loop (the in-loop join is polluted to Top by
  the final wrapping subtraction), falling back to a non-Top tracked range. Sound: over-approximates the count;
  unknown range / non-constant subtrahend ⇒ stays unbounded. Locked by `roms/litmus/cb_divloop.asm` +
  `TestProveDivideLoopBounded`.

### Changed
- **VV-14 2C — last false-positive violations cleared.** After 2A exposed them, four stable-262 kernels showed a
  genuine multi-line region as a violation; `@lines` declares the true span (sfx_demo Vis ×3 ⇒ now CERTIFIED;
  shared_setxpos/text12/text24 positioning-setup ×2 ⇒ violation cleared, other regions stay honestly UNBOUNDED).
  Result: **no false-positive violation remains in any technique kernel**; certified 13 → 14 / 31.

### Notes
- Honest measured outcome: 2A/2B add real, self-tested prover capabilities, but raised the kernel certify count by
  only +1 — the remaining uncertified kernels are blocked by a *combination* of hard/honest issues (no-WSYNC,
  multi-call-site RTS context, nested loops, WSYNC-in-loop, and divide loops whose counter lives in untracked
  indexed RAM, plus bank-switched display). Those UNBOUNDED verdicts are the correct honest scope limit, not false
  alarms; reducing them further needs larger absint work (indexed-memory range tracking, multi-context returns) —
  diminishing returns, deferred along with ②Z3/④external-ROMs.

## [1.95.0] - 2026-06-18

### Added
- **Citable cycle-budget certificate (VV-14, ③).** New `cmd/cpucert` + `cyclebound.Certify`. Wraps the VV-2 static
  prover in a reproducible, attestable proof artifact: per-region proven worst-case + verdict, the `@lines`
  declarations the proof relies on, and full provenance — prover version, Gopher2600 pin, DASM version, and SHA-256
  of both the `.asm` and the assembled ROM. Text or `-json`; exit 1 when not certified. Self-test both directions:
  smoke certifies with a deterministic ROM-core + hashes; litmus_overrun is rejected; multicolor48's cert records
  the `@lines 2` lemma it relies on; distinct ROMs hash distinctly (tamper-evident).

### Changed
- **VV-14 ① prover precision — applied `@lines` to real kernels.** Empirically (sweeping all 30 technique kernels),
  the prover's over-warnings are **multi-line-region** false positives, not infeasible-path ones (0 kernels). Each
  affected kernel runs at a verified-stable 262 scanlines/frame, so an over-budget region (worst W>76) legitimately
  spans ⌈W/76⌉ scanlines; declaring that with `@lines` is the sound fix. Annotated 9 kernels (multicolor48, score6,
  hscroll, bitmap48, two_line_vdel, zone_multiplex, tia_pcm, bullets, rpgmap): 5 now fully certify, 4 clear their
  false-positive violation (other regions stay honestly UNBOUNDED). No prover-code change; existing cyclebound
  self-tests stay green (no-false-negative preserved).

### Notes
- **Assessed and deliberately NOT built (measured 0/low payoff on real kernels):** display-off region
  reclassification (the VSYNC→VBLANK transition has VBLANK provably-unknown — first frame can run with display on —
  so skipping would be unsound; reverted), value-range loop bounding (the unbounded loops are nested / hardware-timer
  waits / JSR-RTS subroutine timing, which a divide-by-15 bounder cannot fix), and infeasible-branch pruning (no
  real kernel needs it). The remaining UNBOUNDED verdicts are the correct honest outcome; tightening them needs
  subroutine-timing modeling — a larger future lever, kept deferred along with ②Z3/SMT and ④external silicon-TIA ROMs.
- `.gitignore`: ignore stray `/cpucert`.

## [1.94.0] - 2026-06-18

### Added
- **Frequency-domain audio comparison (VV-13).** New `internal/audiospec` + `cmd/audiospec`. The `golden_audio`
  scenario check hashes the audio register chain and an RMS envelope says "how loud over time"; neither separates
  two sounds with the same loudness contour but different pitch/timbre ("inverted twins"). audiospec adds the
  spectral modality: a pure-Go radix-2 FFT magnitude spectrum with a cosine **spectral distance**, alongside an
  **RMS-envelope distance** and a dominant-frequency readout, over the captured PCM stream (`emu.AudioSamples`).
  `cmd/audiospec` compares two ROMs' audio on a chosen channel, prints a JSON report, and exits 1 above `-max`.

### Notes
- Self-test demonstrates the axis numerically: two equal-amplitude tones at different pitch score **envelope
  distance 0.0000 vs spectral distance 0.9980** — the spectral axis out-resolves the envelope. FFT recovers a known
  tone within bin resolution; identity is zero; a real capture (sfx_demo) runs the full pipeline. Pure Go, no
  reconnect. `.gitignore`: ignore stray `/audiospec`.
- **Phase C complete** — Tier-3 VV-11/12/13 all done; VV-14 (Z3/ILP prover upgrade + external silicon-TIA ROMs)
  remains deliberately deferred per the audit until a kernel demands it.

## [1.93.0] - 2026-06-18

### Added
- **Tolerant frame compare: SSIM + perceptual hash (VV-12).** New `internal/framesim` + `cmd/framesim`. The exact
  `golden_frame` scenario check answers a boolean "identical?"; framesim answers "how wrong, and where". Windowed
  **SSIM** over 8×8 luma blocks gives a magnitude (mean, 1.0 = identical) plus locality (the worst-matching block);
  a **DCT perceptual hash** gives a shift-tolerant Hamming distance. This complements — does not replace — the
  exact golden: a 1-pixel jitter that flips the exact rendering hash still scores SSIM ~1.0, while a genuinely
  corrupted frame scores far lower (multicolor48 vs smoke ≈ 0.08). `cmd/framesim` compares two frames (each a
  rendered `.bin` or a `.png`), prints a JSON report (ssim mean/worst, worst block, pHash distance), and exits 1
  when SSIM falls below `-min` (a tolerant regression gate).

### Notes
- Self-test both directions: identical ⇒ SSIM 1.0 / pHash 0; a 1-pixel change stays >0.99 (tolerant) but < 1;
  inverted < 0.5; SSIM monotonic in damage; the worst block localises injected damage; real cross-ROM frames score
  measurably below self. Pure Go, no reconnect. `.gitignore`: ignore stray `/framesim`.

## [1.92.0] - 2026-06-18

### Added
- **State-coverage matrix (VV-11, part 1).** New `internal/statecov` + `cmd/statecov`: a coverage axis orthogonal
  to PC/branch coverage (VV-3). Instead of "which instructions ran", it answers "which TIA *modes* did the test
  exercise" — NUSIZ copies, missile/ball size, VDELP0/P1/BL, playfield reflect/score/priority, and bank switches —
  by sampling `emu.ReadTIARegisters` + `Bank` once per scanline over a multi-frame run. An axis stuck at its reset
  value is a verification blind spot. `cmd/statecov` reports distinct values + a coverage fraction per axis (JSON).
- **Coverage-filtered mutation = honest kill rate (VV-11, part 2).** `mutate.EvalRandomCovered` (and
  `cmd/mutate -covered -frames N`) restricts fault injection to ROM offsets that a baseline run actually executes
  (PC coverage via `emu.SeenPCs`). Naive mutation dilutes the kill rate with mutations in never-executed code that
  can never be killed; the covered variant measures the suite against live code only. On `smoke.bin` (mostly
  unexecuted 4K padding) the same suite scores **2% naive vs 68% covered** — closing the testing-playbook's
  misleading 5–20% kill-rate thread.

### Notes
- Self-tests both directions: the matrix must distinguish a mode-exercising ROM (multicolor48) from one that never
  moves that mode (smoke), and a bank-switching ROM from a flat one; the covered kill rate must exceed the naive
  rate and be non-vacuous + deterministic. Pure Go, no reconnect.
- `.gitignore`: ignore the remaining stray repo-root cmd binaries (`/statecov`, `/mutate`, `/cover`,
  `/guidedfuzz`, `/trajdiff`).

## [1.91.0] - 2026-06-17

### Added
- **perfect6502 silicon CPU differential (VV-7).** New `internal/cpudiff` + `cmd/cpucheck`: a hardware-grade
  differential of the embedded Gopher2600 CPU core against the perfect6502 transistor netlist (mist64/perfect6502,
  the visual6502 model), one instruction at a time. This is a **CPU-layer** oracle — perfect6502 has no TIA/RIOT
  and cannot run a 2600 ROM, so it is **not** a member of the full-system RAM vote (`cmd/oraclevote`). Its value:
  catching a CPU bug that Gopher2600 and MAME (both software) could share, and covering undocumented / decimal
  opcodes that the fixed Tom Harte corpus (VV-1) excludes.
  - `internal/cpudiff/p6502step/p6502step.c` (first-party): runs exactly one instruction on the netlist. Register
    injection via a `measure.c`-style prologue (perfect6502 exposes no register writers); the single-instruction
    boundary is taken from the **SYNC line** (node 539), making it robust even when control flow returns to the
    instruction (e.g. a branch with offset −2); writes captured as a memory diff. Cycle count and PC pinned
    empirically against known answers.
  - `internal/cpudiff`: **symmetric** execution — both engines run the identical 64K image from the same prologue,
    reaching identical pre-instruction state by construction (`buildImage` mirrors the C harness). Differ masks P
    bits 4/5 (B/unused — convention-only). Seeded deterministic vector generator. Empirically established
    **allow-list** of the only opcodes permitted to diverge: 11 illegal/unstable ones (ANC `0B`/`2B`, ALR `4B`,
    ARR `6B`, ANE `8B`, LXA `AB`, SH* `93`/`9B`/`9C`/`9E`/`9F`, LAS `BB`).
  - `cmd/cpucheck`: CLI (`-seed`/`-n`/`-opcodes all|smoke`), JSON summary, exit 1 on any **unexpected** divergence
    (a documented-opcode disagreement = a real CPU bug or a harness artifact). Gated on `bin/p6502step`.
  - `scripts/install_perfect6502.sh`: fetch the pinned clone (`09fc542`, MIT) + build `bin/p6502step`. The
    perfect6502 source is gitignored, never vendored — mirroring how the Gopher2600 clone is handled.
- Self-tests: always-on differ-logic (planted-mutant, both directions, no binary needed) locks the comparator in
  CI; gated silicon differential confirms 0 documented-opcode divergences across many seeds + determinism.

### Notes
- Main build remains **CGO-free** (`CGO_ENABLED=0 go build ./...`): perfect6502 is an external binary, shelled out.
- `.gitignore`: added `/third_party/perfect6502/`, MAME scratch (`/cfg/`, `/snap/`), and stray root binaries
  `/cpucheck`, `/oraclevote`.

## [1.90.0] - 2026-06-17

### Added
- **MAME headless cross-oracle (VV-6).** New `internal/oracle` package: an `Oracle` interface (`DumpRAM`: run a
  ROM from power-on for N frames → RAM $80-$FF), the embedded `Gopher2600` member, `Diff`, and `Vote` (majority
  RAM dump + named dissenters). Extracted from `cmd/stellacheck` (which now reuses `oracle.Gopher`). `oracle.Mame`
  runs MAME's a2600 driver with `-video none -skip_gameinfo` and a lua autoboot script that dumps RAM after N
  frames — a genuinely independent, **fully hands-free** third emulator (unlike the Stella oracle's human
  keypress), CGO-free (shells out to the `mame` binary). New `cmd/oraclevote` runs every available oracle
  (Gopher2600 always, MAME if installed) and reports a majority verdict + dissenters (exit 1 on dissent) =
  "all software agrees but the hardware-grade member disagrees" made visible — the suite's reason to exist.
  Self-test (gated on MAME present): MAME reads smoke's ram.0x80==66 and agrees with Gopher2600 on all 128 RAM
  bytes, voting unanimously; `TestVoteDissent` proves a planted lone dissenter is named. VV-7 (perfect6502
  silicon-netlist CPU oracle) will plug into the same `cmd/oraclevote`. **Src:** MAME luascript docs.

## [1.89.0] - 2026-06-17

### Added
- **VV-2 green-ification: `@lines N` per-region budget for 2-line kernels.** A legitimate 2-line kernel does
  ~2 scanlines of CPU work between WSYNCs (~152cy), which the fixed 1-line budget (76) wrongly flags. A
  `; @lines N` note on the source line that opens a WSYNC region now sets that region's budget to N*76, greening
  real 2-line kernels (multicolor48 / score6 / tia_pcm / exerciser) without weakening the proof — an
  un-annotated over-76 region still flags. `srcmap` gained an exported `Line(pc)`; `cyclebound.Prove` reads
  `@lines` from the region opener's source line (scanning the mapped line + the next, since DASM maps a labeled
  WSYNC to its label line). Sound: the annotation only scales a specific region's budget, never disables a
  check. Planted/clean litmus `cb_2line` (`@lines 2`, region 139cy → certified) vs `cb_2line_noann` (same
  kernel, no note → 139>76 flagged); `TestTwoLineBudgetAnnotation` locks both directions. Applying the
  annotations to the actual game ROMs is a roms-repo follow-on. **Src:** Li&Malik IPET DAC'95.

## [1.88.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-3: uninitialized-RAM read — VV-10 complete.** `Emu.WatchUninitRead` flags the
  first read of a RAM byte ($80-$FF) never written since reset = the passes-in-emu (deterministic value) /
  fails-on-HW (power-up garbage) hazard. The enabler is `Emu.effectiveAddr`, which resolves the true memory
  address of every operand mode — Absolute (zero-page folded in via `Defn.Bytes==2`), AbsoluteX/Y (with zp
  wrap), and `(ind,X)`/`(ind),Y` via pointer dereference — so an indexed clear loop (`sta $00,x` writing 128
  RAM bytes in one instruction) is fully tracked and **not** a false positive. Stack push/pull are implied (no
  operand) and so fall outside this operand-based tracker, self-consistently. Exposed as the scenario check
  `checks.no_uninit_read`, run on a **fresh emu from reset** (uninit-read is a from-reset property, unlike the
  per-frame T-1/T-2). Planted/clean litmus (`uninit_trap` reads $90 with no clear = hit; `uninit_clean` indexed-
  clears then reads = no hit) with `TestUninitReadDetector` locking both directions (proving the indexed clear
  is not a false positive). **VV-10 is now complete (T-1 timer-wrap / T-2 HMOVE-latch / T-3 uninit-RAM-read).**
  **Src:** known-traps.md §A/§D; Valgrind Memcheck (shadow memory).

## [1.87.0] - 2026-06-17

### Added
- **Score OCR semantic oracle (VV-9).** `internal/ocr` reads the RENDERED digit pixels (not the registers) and
  decodes a displayed 2-digit packed-BCD score, matching each glyph against templates rendered from a
  ground-truth font (the spec — PF1=MSB-first / PF2=LSB-first per the verified playfield bit order). It asserts
  displayed == `decode(RAM)`, tying the display back to program meaning — catching display-kernel / BCD-split /
  font-index bugs that an exact frame hash would pass (a hash also accepts a consistently-wrong glyph). The band
  is located by detecting its top then sampling at the kernel's fixed row spacing (robust to blank glyph rows).
  Exposed as the scenario check `checks.score_equals_ram` (ground-truth font from a `<scenario>.font` sibling
  file, like golden files; no MCP tool, no reconnect). Litmus `score2.asm` renders RAM $80 (packed BCD '42') via
  PF1/PF2. Self-test `TestScoreOCRSelfTest`: the genuine ROM decodes 42 == RAM; a font-index mutation (glyph 8
  copied over glyph 4 in the ROM, RAM untouched) is caught as displayed≠RAM. **Src:** pHash Hamming primitive.

## [1.86.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-2: HMOVE-then-HMxx-within-24cy (VV-10).** `Emu.WatchHMOVEHazard` flags the
  first write to a motion register (HMP0/HMP1/HMM0/HMM1/HMBL or HMCLR) within 24 CPU cycles of an HMOVE strobe —
  the documented "unpredictable motion" hazard (Stella PG). The 24-cycle window is measured in **color clocks**
  (72 = 24 CPU cy) via `Coords`, not the executed-cycle counter (which excludes WSYNC stalls), so a clean
  kernel that separates HMOVE from HMxx with a WSYNC is correctly judged outside the window. Exposed as the
  scenario check `checks.no_hmove_hazard`. Planted/clean litmus pair (`hmove_trap` writes HMP0 ~3cy after HMOVE
  = hit; `hmove_clean` sets HMxx in VBLANK and strobes HMOVE right after a WSYNC = no hit) with
  `TestHMOVEHazardDetector` locking both directions. VV-10's T-3 (uninitialized-RAM read) remains a follow-on:
  a correct shadow-memory detector needs full effective-address resolution for indexed/`(ind),Y` writes.
  **Src:** Stella Programmer's Guide (HMOVE timing); known-traps.md §A/§D.

## [1.85.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-1: RIOT timer-wrap / G8 (VV-10, partial).** `Emu.TimerState` exposes the RIOT
  timer (INTIM/TIMINT/Expired/Divider/ticksRemaining) and `Emu.WatchTimerWrap` flags the first time a program
  **reads INTIM while the timer has already underflowed/wrapped (Expired set)** — the G8 hazard (too-small
  interval, or a poll loop that stepped over 0 and is consuming post-wrap values). Exposed as the scenario check
  `checks.no_timer_wrap` (frames to watch) — no MCP tool, no reconnect.
- **Key finding (measured):** the audit's one-line spec "flag the wrap" is too naive and would false-positive on
  a *correct* kernel — `cb_timer` (which polls INTIM to 0 properly) also lets the timer wrap later in the frame,
  but nothing reads INTIM then. So the trap is narrowed to *read-after-wrap*, and `Expired` is sampled BEFORE
  each instruction because reading INTIM clears it (the timer's reversion). Planted/clean litmus pair
  (`timerwrap_trap` TIM1T poll overshoots 0 = hit; `timerwrap_clean` TIM64T polled to 0 = no hit) with
  `TestTimerWrapDetector` locking both directions. T-2 (HMOVE-then-HMxx<24cy) and T-3 (uninitialized-RAM read)
  remain follow-ons. **Src:** known-traps.md §A/§D; AtariAge 303277; Valgrind Memcheck (shadow memory).

## [1.84.0] - 2026-06-17

### Added
- **Behavioral trajectory diff (VV-8).** `internal/trajdiff` + `cmd/trajdiff` step an original and a candidate
  ROM in lockstep on the same input timeline and report the first frame+field where their observable state
  diverges, or MATCH. The default trajectory is the 128-byte RAM each frame (`emu.PeekRAM`); custom fields reuse
  `scenario.ResolveField`. It compares **behavior over time, not bytes**, so a dead/cosmetic byte difference is
  a MATCH while a real behavioral change is caught at the exact frame — the strongest oracle for a reproduction
  task (and a step beyond `refdiff`'s static snapshot). Pure Go, no external dependency, no reconnect. Self-test
  (`TestTrajdiffSelfTest`): identity = MATCH (determinism guard), a corrupted reset vector diverges, a
  behaviorally dead-byte flip = MATCH. The CLI exits 1 on divergence, 0 on MATCH. **Src:** Martignoni TOSEM'13;
  EXAMINER ASPLOS'22; McKeeman 1998.

## [1.83.0] - 2026-06-17

### Added
- **PC/branch coverage + coverage-guided fuzzing (VV-3).** Closes the last Tier-★1 gap — the test-adequacy
  axis plus AFL-style feedback fuzzing (the existing scenario `fuzz` is blind).
  - **Coverage recorder** (`internal/emu.Coverage`): an opt-in hook in `stepInstr` (at instruction completion,
    reading `LastResult.Address`/`Defn.IsBranch()`/`BranchSuccess`) records executed instruction addresses and
    per-branch taken/fall-through edges. Exposes `PCCount`/`BranchCount`/`EdgeCount`/`OneSidedBranches`
    (a branch whose other side was never exercised)/`Seen`/`Signature`. Nil until `EnableCoverage` = zero cost.
  - **`cmd/cover`**: drives a ROM and reports reached coverage + one-sided branches. On `cyclebound_branch` it
    flags `0xF036` — the same path VV-2 statically proves overruns (101>76) but runtime never takes, an
    independent cross-check between the two tools.
  - **`internal/guidedfuzz` + `cmd/guidedfuzz`**: AFL-style search that keeps a corpus of input sequences and
    grows it whenever a mutation reveals a new coverage marker (`Coverage.Signature`), climbing toward
    deeply-guarded states blind fuzz essentially never reaches. The search core is decoupled from the emulator
    via `Evaluator`, so it is unit-testable; `EmuEvaluator` wires it to a fresh deterministic emu per run.
  - Self-test: `TestCoverageLogic` (one-sided detection; an unrecorded address reads as uncovered),
    `TestGuidedBeatsBlind` (synthetic staircase oracle — guided reaches full depth = 9 markers while blind
    stalls at 4 on the same 6000-iteration budget, deterministic and ROM-independent), plus emu-wiring
    integration tests. Scope (honest): full dead-code over the decodable universe is a follow-on; today's map
    is reached-coverage + one-sided branches. **Src:** Zalewski AFL whitepaper; Go native fuzzing.

## [1.82.0] - 2026-06-17

### Added
- **Temporal-logic trace assertions (VV-5).** New scenario `temporal` block for bounded-temporal-logic
  properties over the **frame sequence** — things an instantaneous `assert` or a per-frame `invariant` cannot
  express. Three monitor kinds (`always P` stays as the existing `invariant`, not duplicated):
  - **`eventually`** — P must hold within `within` frames of the run start (bounded liveness).
  - **`response`** — whenever trigger A holds at frame *f*, P must hold within `within` frames (*f*..*f*+within).
  - **`never_for`** — P must not hold for `n` consecutive frames (safety).
  Implemented in `internal/scenario` by reusing the existing condition vocabulary (`resolve` + `condPass` +
  `condDesc`): each monitor's proposition (and the response trigger) is observed into a per-frame boolean trace
  inside the run loop, then the verdict is computed off the trace. Liveness whose window is not fully observed
  reports **INCONCLUSIVE** (`Pass:false`) so it can never be a vacuous green. **Scenario-only — no MCP tool,
  hence no server rebuild / reconnect** (a deliberate low-friction choice this session). Self-test:
  `TestEvalTemporal` fixes pass/fail/inconclusive for all three operators on planted boolean traces
  (frame-base independent) and `TestTemporalThroughRun` proves the resolve→observe→eval wiring end-to-end on
  `smoke.bin` (plus inconclusive-is-not-green and invalid-definition rejection). Sample
  `roms/litmus/scenarios/temporal.json`. Docs: `docs/scenarios.md`, `docs/testing-playbook.md`. **Src:**
  Bauer/Leucker/Schallhart TOSEM 2011 (LTL₃); STL RV'15.

## [1.81.0] - 2026-06-17

### Changed
- **VV-2 prover precision S0–S3 (fewer false positives, same proof strength).** The static per-scanline
  cycle-budget prover gained an abstract-interpretation layer so it stops flagging sound kernels:
  - **S0 — abstract-interpretation engine** (`internal/cyclebound/absint.go`): tracks a per-address
    value-range state (registers / known constants) by forward dataflow from the reset/IRQ/NMI entries.
  - **S1 — region recognition**: VSYNC/VBLANK and timer-driven (TIM64T) intervals are classified and skipped,
    so a legitimately long blank region is no longer reported as over-budget.
  - **S3 — page-cross precision**: an `abs,X`/`abs,Y` read's +1 page-cross penalty is now resolved from the
    proven index range — if `[base+lo, base+hi]` provably stays inside one 256-byte page the penalty is 0;
    an unknown index, or a pointer-based `(ind),Y` whose base we don't track, stays conservative (+1). The
    abstract state is wired into the solver (`solver.absStates`) and applied via `baseCost()+pagePenalty()`.
    Loop-body costing keeps the conservative `nodeCost` (sound, over-approximating).
- The proof stays **sound**: every relaxation is on the false-positive side only. `TestCycleboundSelfTest`
  (planted-discrepancy, "no false-negatives") stays green, and prove⇔assert agreement was re-verified on the
  litmus set (`cb_clean`/`cb_timer` certified; `cb_roll`/`cyclebound_branch` (101>76)/`litmus_overrun` (108)
  flagged).

### Notes
- **Finding (recorded to memory `feedback-verification-standard`):** a *small* per-scanline overrun (one heavy
  line = 262→263 scanlines) is **visually invisible** — the TV's auto-sync absorbs a one-line slip, so
  `cb_roll` and `cb_clean` render pixel-identically. This is precisely why a static ∀-prover exists: the
  defect is unseeable, only the numbers differ. Visual verification is unfit for this class of timing defect.

## [1.80.0] - 2026-06-17

### Added
- **Static per-scanline cycle-budget PROVER (VV-2).** Proves the worst-case CPU cycles of every
  `STA WSYNC`-to-`STA WSYNC` region over **ALL reachable paths** (∀) — the static sibling of the runtime
  `assert_line_budget` (which observes only one run, ∃) and the flagship attack on gap B (timing).
  `internal/cyclebound` recursive-descent-decodes the ROM from its reset/IRQ/NMI vectors (so inline data
  isn't misdecoded), costs each instruction from the in-tree exact table (`instructions.Definitions`: cycles
  + branch-taken/page penalties), cuts the CFG at every `STA WSYNC` ($02), and proves each region's DAG
  longest path ≤ budget (default 76, no solver). Counted loops (`ldx/ldy #N` + `dex/dey` + `bne/bpl`) are
  folded by their bound; JSR / indirect JMP / unbounded loops are reported honestly as out-of-scope, never
  silently passed; over-budget regions return a cycle-by-cycle worst path + source location. Shipped 3 ways:
  the `cmd/cyclebound` CLI, the **`prove_line_budget` MCP tool** (run it before executing a kernel), and the
  scenario **`checks.prove_line_budget`** regression gate (`cyclebound_safe.json` certifies smoke). New litmus
  `cyclebound_branch` overruns only on one branch (~101cy) so a live run is a lucky pass yet the proof flags
  it; the planted-discrepancy self-test (`TestCycleboundSelfTest`) also bounds `litmus_overrun`'s counted
  delay loop (108cy), certifies `smoke` (worst 19cy), flips smoke to a violation under a tight budget
  (non-vacuous), and checks the certified bound holds at runtime (observed-within-proven dual). The only
  ∀-claim member of the suite. Scope (v1, honest over guessing): single-bank flat 2K/4K; only the
  `ldx/ldy #N`+`dex/dey`+`bne/bpl` loop idiom is bounded (divide-by-15 positioning and other A-reg/memory-counter
  loops report unbounded rather than risk a false violation); page-sensitive reads charged a conservative +1; a
  0-WSYNC ROM is reported unbounded, never vacuously certified. Src: Li & Malik IPET (DAC'95); Ballabriga &
  Cassé (WCET'08).

## [1.79.0] - 2026-06-17

### Added
- **Motion-smoothness / jerk metric (VV-4).** Turns "does this object judder / ブルブル" into numbers:
  `internal/motion` tracks a TIA object's exact X (`Markers().HmovedPixel`) and rendered top over N frames →
  velocity (1st diff), acceleration (2nd diff), and **jerk_rms** (RMS of the 2nd difference; 0 = constant
  velocity) plus `max_accel`/`monotonic` (a real glitch/snap vs a benign integer-pixel staircase). Shipped as
  the `cmd/motion` CLI, the **`read_motion` MCP tool** (interactive — automates the hand frame-by-frame trace),
  and the scenario **`checks.motion`** regression gate. New litmus `motion_glide` (clean +1/frame → jerk 0) and
  `motion_stutter` (+2,0,+2,0 → jerk 2); the planted-discrepancy self-test (`TestMotionSelfTest` +
  `scenarios/motion_glide.json`) locks that the stutter scores above the glide, so the metric can't be vacuous.
  Used live on the Breakout ball (vertical jerk 0, horizontal jerk 1 = the benign 1px/2-frame staircase) and
  validated against the user's own perception (motion_stutter in Stella reproduced their reported judder).
  First Tier-1 perceptual oracle of the verification-variety backlog. Src: Flash & Hogan 1985 (minimum-jerk).

## [1.78.0] - 2026-06-17

### Added
- **CPU-core conformance gate in CI (VV-1).** The embedded Gopher2600 CPU is now certified against two
  external authoritative suites already vendored in the clone but never run by harness CI: **Klaus Dormann**
  6502 functional+decimal (embedded `.bin`, always-on) and a **Tom Harte / SingleStepTests 65x02** subset
  (per-cycle bus addr/data/read-write + final state + cycle count; 12-opcode smoke fetched on demand, MIT,
  not vendored; full 256 is local-only, ~1GB). Suites run via the full import path
  `go test github.com/jetsetilly/gopher2600/hardware/cpu/tests/{klaus2m5,thomharte}/...` (`replace`-resolved).
  New `scripts/check_cpu_conformance.sh` (+ `--selftest`) and two CI steps. The gate is **self-validated** by a
  planted-discrepancy: a corrupted expected value must make the run go RED (proven live, not vacuous). First
  Tier-1 pilot of the verification-variety backlog (`docs/capability-gap-audit.md`). Src: Klaus2m5 repo;
  SingleStepTests/65x02.

## [1.77.0] - 2026-06-17

### Changed
- **Verification discipline consolidated into a single canonical standard** (`feedback-verification-standard`,
  "MAX"). Iron rule 1 (`CLAUDE.md`) and the authoring-protocol Verify step (step 5) now **reference** that
  standard's MAX checklist instead of restating it: trace frame-by-frame, read the full object window (no
  partial reads), cross-check derived formulas against raw pixels, kill each hypothesis with data, prove the
  negative, present the measured table. Born from the Breakout ball-judder investigation (proved "not a bug"
  purely by `read_row` measurement).
- **Rule-base de-duplicated to "1 rule = 1 source of truth."** Memory feedback rules merged 18→10 (the
  verification cluster collapsed 5→1; goal 3→1; execution 2→1; work-tracking 2→1). Stale `[[memory-links]]`
  in `docs/` (design-principles, build-to-learn, casebook, testing-playbook) repointed to the new canonical
  names. No behavioural code change; CI/wiring unaffected.

## [1.76.0] - 2026-06-16

### Added
- **`set_input` now drives the console panel switches** (`reset`/`select`/`color`/`p0pro`/`p1pro`), not just the
  joystick — routes to the existing `emu.SetPanel`. Lets Claude press GAME RESET to actually start a game (e.g.
  the real Breakout, which needs RESET to leave attract mode) so its sprites can be measured live. Motivated by
  a `refdiff` gap: the original's ball height couldn't be rendered/measured because the game wouldn't start.
- **`internal/refdiff` + `cmd/refdiff`** — differential layout check vs a reference ROM (the original = oracle):
  extracts a fingerprint (left/right wall clock, ball **width and height**) and diffs it against the original.
  `MeasureBall` starts the game (RESET) and tries both control styles (joystick fire / paddle) to render the
  ball in the open field. Catches "wrong vs the original" that golden self-regression can't (a wall inset from
  the edge, an undersized ball). Worked example: a user spotted my Breakout's left-wall gap + 1×1 ball by
  *playing*; refdiff went RED (wall 2 vs 0, ball 1×1 vs 2×4), drove the fix to MATCH. Wired into
  `docs/testing-playbook.md` (the differential-vs-original entry). (Differential testing.)

### Notes
- **1.76.0** (MINOR — additive: panel-switch input + refdiff with ball-size diff). The MCP server must be
  rebuilt and reconnected to use the panel switches (`set_input reset` etc.).

## [1.75.0] - 2026-06-16

### Added
- **`docs/testing-playbook.md`** (new) — imports the established software-testing discipline (the **oracle
  problem** → invariants/contracts, property-based, metamorphic, differential/golden, fuzzing, **deterministic
  simulation testing** à la FoundationDB/Antithesis, mutation testing, invariant mining, delta debugging) and
  maps each onto this harness, with a per-build verification checklist usable today via `run_scenario` + MCP.
  Wired into the CLAUDE.md routing (step 5b) + authoring-protocol step 5. Motivated by `feedback-verify-at-claim-level`
  (verify at the level of the claim — for emergent behaviour, demonstrate it; don't infer it from component checks).
  Backlog for the executable backers added to `docs/capability-gap-audit.md` as **G10–G14** (scenario
  `invariants`/`monotonic`/range, `fuzz`, `metamorphic`, `mutation`, `mine-invariants`). Provenance recorded
  (QuickCheck, Daikon, AFL, FoundationDB/Antithesis, Chen/Segura, Barr, DeMillo, Zeller).
- **Automated verification suite — G10–G14 all delivered** (the executable backers of the playbook):
  - `internal/scenario`: **`invariants`** (a condition checked every frame), **`monotonic`** (a field that
    only moves one way over the run), the **`in`** range operator, **`fuzz`** (seeded random input +
    per-frame invariant monitoring + CPU-jam detection, deterministic = replay by seed), and **`metrics`**
    (fields captured at end of run). `run_scenario` MCP gains all of these automatically.
  - `internal/mutate` + **`cmd/mutate`** — mutation testing (inject a ROM-byte fault; confirm the suite kills
    it, or flag a survivor = weak checks; seeded batch kill rate).
  - `internal/metamorphic` + **`cmd/metamorphic`** — assert a relation `A.field <rel> B.field` between two
    runs (oracle-free).
  - `internal/mine` + **`cmd/mine-invariants`** — Daikon-lite: observe a driven run, emit candidate
    `invariants`/`monotonic` as a spec draft (`scenario.ResolveField` exported for shared vocabulary).
  - `build.Assemble` now passes `-I<asm dir>` so `.asm` scenarios resolve includes (e.g. `vcs.h`) from any cwd.
  - **First catch:** the Breakout `fuzz` scenario exposed the frame was 264 lines, not the "262" claimed by
    eye (never measured); fixed to a true 262 in the roms repo. New litmus scenarios `invariants.json`,
    `fuzz.json`. Tests added for every package.
- **`docs/build-to-learn.md`** (new) — reusable methodology: reproduce a real game mechanic-by-mechanic to
  turn "can read" into "can author". Wired into the CLAUDE.md routing table (step 1c) + cross-linked with
  casebook. First worked example = **Breakout** (`roms/breakout/`, 8 rungs from stable frame to a playable
  single-player game, each verified numerically against the real ROM; per-rung snapshots in `steps/`).
- Methodology refinements (user-driven): **measure the original's dimensional layout in Phase 0** (before
  building — retrofitting layout in assembly is costly); **judge colour AND size by `read_row`, not by eye**
  (caught the paddle being mis-set to white-24px when the original is red-16px); **read_row = measurement /
  CXxx collision = runtime check + contact verification** (two-tool split).
- **`docs/casebook.md`** — Breakout entry (the build-to-learn worked example: multi-region PF kernel,
  RAM-driven destructible PF, BL/P0 positioning, joystick paddle, position-based collision, game-state loop).

### Notes
- **1.75.0** (MINOR — additive: build-to-learn + casebook docs, the testing-playbook, and the automated
  verification suite G10–G14; all backward-compatible).
- The MCP server (`cmd/harness`) must be **rebuilt** for `run_scenario` to pick up the new scenario features
  (`invariants`/`monotonic`/`fuzz`/`metrics`) — smoke-test with `scripts/mcp_smoke.py` then reconnect.
- The authored Breakout ROM + its scenarios live in the **roms** repo (`roms/breakout/`), not here.

## [1.73.0] - 2026-06-15

### Added
- **`docs/casebook.md`** (new) — the *situation → technique* canon, evidence-backed by real commercial-game
  disassemblies (companion to `cookbook.md`'s forward recipes). Wired into the CLAUDE.md routing table
  (step 1b) and the authoring-protocol retrieve step; `check_wiring`/`check_provenance` green.
  - First case study: **Fishing Derby (Activision 1980, David Crane / Dennis Debro disassembly)** — the
    3-layer casebook pilot (manual=spec × disassembly=impl × Claude reconstruction). Raw pairing lives
    study-only (non-repo) under `reference/disassemblies/_casestudies/fishing-derby/`.
- **design-principles.md** — three new principles distilled from the pilot (with provenance):
  per-scanline **NUSIZ+HMOVE shaping** of one player into an 8px-plus irregular sprite (shark);
  **fractional-HMOVE slope** drawing of an arbitrary-angle 1px line on a missile/ball (fishing line);
  background shimmer by streaming a PRNG's bits to `COLUBK` per scanline (near-zero cost).
- **capability-gap-audit.md** — G9: authoring-craft support for the two shaping/slope patterns the
  Claude-side reconstruction missed (concrete-driven, build when the next ROM needs it).

### Notes
- Methodology refinement (user, 2026-06-15): **Layer1 spec = manual + live ROM observation**, not the manual
  alone — every reconstruction error was corrected by running the real ROM (feedback-play-the-rom-not-just-manual).
- Proposed release: **1.73.0** (MINOR — additive knowledge). Tag + push deferred to user approval.

## [1.72.0] - 2026-06-15

### Added
- **Knowledge-activation architecture** — so the whole accumulated corpus *fires* at authoring time and nothing
  rots unused ([[knowledge-activation-architecture]]).
  - `docs/authoring-protocol.md` is now the single **START HERE** entry for building a ROM: the mined pro
    workflow (A–E: image-first → 14-step build order → ceiling → audio truths → cycle-budget craft) above the
    6-step loop. CLAUDE.md iron rule 5 points to it.
  - **CLAUDE.md routing reorganized into the authoring-flow order** (① building a ROM, in sequence · ② reference).
  - `scripts/check_wiring.py` (**CI-gated**) — fails if any `docs/*.md` is unreachable from the routing table or
    the protocol; structurally prevents knowledge from orphaning. Caught + wired `capability-gap-audit.md`.
  - `docs/mining-digest.md` now also indexes the **117 dev-blog entries** (generated by `gen_mining_digest.py`).

### Changed
- CI now runs three knowledge lints — `check_provenance` (origins) + `check_traps` (traps) + `check_wiring`
  (no orphans). Green means knowledge is traceable, sound, and reachable.

## [1.71.1] - 2026-06-15

### Changed
- Completed the dev-blog gold absorption into `design-principles.md` (the two >2-object flicker algorithms;
  drop-PF0 cycle/RAM trade; 2-zone complementary-height moving platforms; 8-byte self-modifying init +
  hotspot placement) and `known-traps.md` (AUDF-lowering ≤32-cycle propagation latency). Beyond-bB findings
  (DPC+/ARM/CDF data-exchange, Slick/Fast-Fetch kernels, wav2tia, INT2HEX/INT2BITS) logged as technique
  candidates. All sourced to mined blog entries.

### Planned (historical — resolved; formerly a second `## [Unreleased]` heading)
> This block was a stray **second** `## [Unreleased]` heading stranded here between 1.71.1 and 1.71.0. It
> entered the file on 2026-06-10 in `f8ae33d` ("docs: English CHANGELOG"), was never cleared, and every later
> release was prepended above it. Demoted to a dated note on 2026-07-30 so the file has exactly one
> `[Unreleased]`. The three items are kept verbatim; all three had already shipped by the time this block was
> buried:
- Real game authoring on top of the 1.0 base (1.x). — delivered across the 1.x line.
- Stella oracle v2 (TIA/pixel compare, full keystroke automation); Slocum note-table transcription for composing.
  — Stella oracle v2 delivered in **1.54.0** (`stellacheck -pixels` / `-snap`, F-4 closed, hands-free via
  `scripts/stella_oracle.sh`); Slocum note-table transcription delivered in **1.35.0**
  (`pkg/audio.NoteFreq/FindNote` + `cmd/jingle`).

## [1.71.0] - 2026-06-15

### Added
- **Authoring loop tooling (Track E).**
  - `scripts/check_traps.py` — static pre-flight linter for the `docs/known-traps.md` traps (unstable illegal
    opcodes, `NOP $00` bankswitch, stack-collision vars, missing CLD/CLEAN_START). Zero false positives on the
    31 technique ROMs; `--selftest` proves detectors fire; **CI-gated**.
  - `docs/authoring-protocol.md` — the 6-step loop (retrieve→plan→author→preflight→verify→**feedback**) run on
    every kernel; the feedback step makes each production strengthen the system.
  - `docs/cookbook.md` — intent→recipe (game-type → technique stack + traps + checks) + the canonical bottom-up
    14-step build order (from SpiceWare's "Collect" tutorial).
- **AtariAge dev-blog mining (expansion).** 117 dev-blog entries distilled (SpiceWare's *Collect / Stay Frosty /
  Frantic / Draconian* dev diaries + DPC+/ARM + TIA-audio internals), fetched **Wayback-only** (CDX enumeration
  of `blogs/entry/*`). Key absorptions into `design-principles.md`: the **Photoshop-mock→48px flicker-free 2-color
  title** path (the project's image→assembly route), the 21-cy mask-sprite draw, and the `sta.w`/`.FORCE`
  one-cycle RESP trim. Corpus + gaps recorded in `reference/atariage/RECOVERY-TODO.md`.

### Notes
- AtariAge automated direct-access suspended (the account hit IPS's login-throttle lockout); mining is
  Wayback-only henceforth. Remaining gaps (15 blog + a few forum, all re-fetchable) listed in RECOVERY-TODO.

## [1.70.0] - 2026-06-15

### Added
- **AtariAge forum-50/31 mining campaign complete + absorbed.** All 761 `TO_MINE` threads from the
  Programming + Newbies forums deep-mined (850 total in `reference/atariage/MINED.csv`); 1727 threads
  triaged (provenance/checklists live under the umbrella `reference/`, not committed).
  - `scripts/gen_mining_digest.py` keeps `docs/mining-digest.md` (850 threads → principle/function it feeds)
    in sync from `MINED.csv`.
  - `scripts/aa_fetch.py` gained `-direct-first` (AtariAge cookie lane) so parallel mining splits load across
    two backends (Wayback + Cloudflare) without contending.
  - Heavy `docs/design-principles.md` absorption: **positioning ground-truth** (RESxx internal draw delay
    player+5 / missile·ball+4 CLK, multi-object cyc23 rule, X≤134 spill), RIOT timer-wraparound roll trap,
    illegal-opcode stability map, TIA-revision pixel-match caveat, mid-scanline GRP-rewrite multiplexing,
    resource triangle + TJ register convention, subpixel/ballistic physics, pixel-aspect source-spread
    (1.67–1.82; codified 2.0 flagged as over — pending one Stella measurement).
- **`docs/known-traps.md`** — catalog of "passes in the emulator, breaks on real hardware" traps (timer
  wraparound, HMOVE-24cy, NOP-$00 bankswitch, page-cross, illegal-opcode stability, TIA read floats,
  mid-line NUSIZ emu-only…), each sourced. The kernel pre-flight checklist and the spec for the future
  `check_traps.py`. Directly targets the timing class of bug that killed past Pong attempts.
- **Provenance enforcement.** `scripts/check_provenance.py` lints that every technique doc / `pkg/design`
  function / design rule cites its origin (CI-gated); `--list` regenerates `docs/provenance.md` (every
  element → its source). Rule recorded so a production issue can always be traced to the original thread.

## [1.69.0] - 2026-06-14

### Added
- **Seven new verified techniques** (built in parallel from mined technique-candidates, each clean-room
  implemented + locked by a CI scenario; all 31 technique scenarios pass, `ntsc_frame_lines:262` + golden):
  - **`road`** (㉓) — pseudo-3D road: M0/M1 shoulders + BL dashed centre, widening per perspective band
    (fills the only gap from the 8bitworkshop cross-check).
  - **`maze`** (⑲) — Entombed-style procedural playfield maze: LFSR bits doubled to 2px cells, scrolled, reflected.
  - **`tia_pcm`** (㉑) — digitized sample playback via AUDV (AUDC=0), 1-bit ADPCM, pseudo-5-bit 2-channel DAC; audio golden.
  - **`shared_setxpos`** (㉒) — position all 5 movable objects with one indexed `RESPx,x`/`HMPx,x` loop.
  - **`divtable`** (⑮) — constant divide ÷3/7/10/15 (corrected reciprocal-multiply, exact over 0..255 + remainder; Go-model exhaustive).
  - **`multicolor48`** (⑯) — 48-px graphic with per-row COLUPx color (~73/76 cy line budget).
  - **`rts_dispatch`** (⑱) — RTS-stack modular kernel dispatch: data-driven vertical zones at ~6cy/transition.
  - Catalog updated (`docs/techniques/README.md`); each has `docs/techniques/<name>.md`.

## [1.68.0] - 2026-06-14

### Added
- **Forum-50 mining run absorbed (checklist + deep-mine + knowledge).**
  - `scripts/gen_mining_digest.py`: idempotent generator for `docs/mining-digest.md` from
    `reference/atariage/MINED.csv` (keyword + curated-override category inference). Digest now 89 threads.
  - Deep-mined 12 high-value forum-50 threads into `design-principles.md`: the **RIOT 6532 timer-wraparound
    roll trap** (double-write `TIM64T`; the rare "passes-in-Stella / rolls-on-hardware" trap, diagnosed
    in-thread by the Gopher2600 author), early-HMOVE don't-move value = 8, HMOVE cy73-74 comb avoidance,
    div15 fine-motion range, flicker luminance tuning, and a **pixel-aspect refinement** (190154 5:3 ≈
    169128 12:7, both ≈1.7 vs the codified 2:1 — flagged for measurement, *not* codified, per verification-first).
  - `capability-gap-audit.md` **G8**: candidate RIOT timer-wraparound roll detector (sibling to `assert_line_budget`).
- **8bitworkshop sample cross-check** (`docs/8bitworkshop-crosscheck.md`). Steven Hugg's "Making Games for
  the Atari 2600" examples assembled in our toolchain (**26/26**, DASM bundled `vcs.h`/`macro.h`) and run in
  the harness; `multisprite3` verified in depth (8 multiplexed sprites read back at `fidelity:1`). Maps each
  sample to our technique library: **25/26 covered**, one gap — `road` (pseudo-3D road via 2 missiles + ball),
  logged as a technique candidate. External audit confirms `roms/techniques/` covers the standard curriculum.

### Notes
- The triage checklists (`reference/atariage/triage-forum50.csv` ~1317 rows, `triage-forum31.csv` 410 rows;
  1727 threads triaged with reasons) and the per-thread `notes.ja.md` live under the umbrella `reference/`
  (provenance, not committed). batari Basic (forum 65) is intentionally out of scope.

## [1.67.1] - 2026-06-14

### Changed
- **Reframed docs around the post-pivot direction** (TIA Studio canvas editor is frozen; the primary
  consumer of `pkg/design` and the design rules is now Claude's own authoring loop, not the editor).
  Updated `docs/design-principles.md` (intro, craft rules, "implementation" section), `docs/capability-gap-audit.md`
  (frozen banner, G2 marked done in v1.67.0), and the `pkg/design` package doc. Research notes under
  `tools/` are kept as the frozen project's historical record. No code behavior change.

## [1.67.0] - 2026-06-14

### Added
- **Absorbed the accumulated design knowledge into the authoring loop (gap-audit G2 completed).**
  - `pkg/design` now codifies the remaining numeric design-principles rules, not just the first six:
    color-register decomposition + judgment (`Hue`/`Luminance`/`HueName`/`WashoutRisk`/`GradientSameHue`/
    `InterlaceColorsSafe`), coarse÷15 + fine-HMOVE positioning (`PositionSplit`/`CoarseIterations`/
    `HMoveReachable`), PF helpers (`PFTotalColorClocks` reusing `playfield.FullWidth`, `ScoreModeTwoColor`,
    `ScrollScanlinesConstant`), multiplex (`FitsMultiSprite`/`NeedsEmptyYLane`/`RepositionCostScanlines`),
    and craft (`PixelAspectRatio`/`ScanlinesForSquare`/`WalkFrame`/`BackgroundSpec.Feasible`). All table-driven tested.
  - `docs/mining-digest.md`: a self-contained index of the 77 mined AtariAge threads (generated from
    `reference/atariage/MINED.csv`), each mapped to the design-principles section / `pkg/design` function /
    technique candidate it feeds. Raw thread captures stay in the umbrella `reference/` as provenance.
- **Routed the knowledge so authoring uses it.** `CLAUDE.md` gains an iron rule ("design before asm") and
  routing-table rows for `docs/design-principles.md`, `pkg/design/`, and `docs/mining-digest.md`.

### Changed
- `docs/design-principles.md`: every rule now carries a disposition — codifiable rules cross-reference their
  `pkg/design` function (`→ func`), and a new "machine-uncheckable judgment rules" section collects the
  qualitative ones (glyph misreads, thumbnail readability, role split…) with the reason they stay doc-only.
  Recorded the `colorPerRow[]` data-model lesson from the TIA Studio research.

### Decisions
- The raw 77-thread forum captures (3 MB HTML) are **not** moved into the harness; the harness keeps the
  distilled, citable digest while `reference/atariage/` keeps provenance. Keeps concerns separated and the
  published repo English-only.
- TIA Studio learnings: durable, authoring-relevant findings are folded into design-principles; tool-impl
  knowledge (spritemate data model, per-scanline-color UI) is intentionally not absorbed (no effect on
  writing assembly) and stays preserved in the frozen `tia-studio/` repo and research notes.

## [1.66.0] - 2026-06-13

### Added
- **`pkg/design` feasibility checker (gap-audit G2).** Codifies the *hard/numeric* rules from
  `docs/design-principles.md` into executable checks, so TIA Studio (and Claude) can answer "does
  this layout fit?" in code rather than prose: color-band minimum width (`MinColorBandWidthPx` /
  `CheckColorBands`), text capacity by technique (`MaxChars` / `FitsText`), 76cy line budget
  (`LineBudget` / `RemainingCycles`), asymmetric-PF right-half write windows (`AsymRightWindow` /
  `FitsAsymRightWrite`), and multiplex / sprites-per-line limits (`NeedsFlicker`). Soft craft (taste,
  readability) intentionally stays prose. Foundation for milestone M4 (budget feasibility).
- **`docs/capability-gap-audit.md`** — a mined-technique × harness-capability gap audit (G1–G7) with
  a prioritized strengthening backlog (G2 → G1 → G4). Also brought `docs/design-principles.md` and the
  `tools/` TIA Studio research corpus (research-w1..w11, build-readiness, M3/M5 prototype design)
  under version control.
- **`aa_fetch.py` direct AtariAge fetch via `curl_cffi`** (Cloudflare bypass). When `AA_COOKIE`
  (the browser Cookie header incl. `cf_clearance`) is set **and `curl_cffi` is installed**, fetches
  the live forum directly by impersonating Chrome's TLS (JA3) fingerprint — plain `curl` 403s even
  with a valid cookie because Cloudflare fingerprints TLS, not just UA/cookie. Adds `direct_get()` /
  `direct_enabled()`, **live page-count discovery** (`discover_live_pages`, **topic_id-based** so a
  wrong/short slug still resolves) to fill Wayback page gaps, a direct fallback when a Wayback page
  fetch fails, and **direct binary attachment download** (the `.bin`/ROM files Wayback never
  archived). Falls back cleanly to Wayback-only when cookie/`curl_cffi` absent (back-compatible).
  New optional dep: `pip install curl_cffi`.

## [1.65.0] - 2026-06-13

### Added
- **AtariAge mining manifest management** (`scripts/aa_manifest.py`): the single source of truth for
  "which threads are already mined", **regenerated idempotently from the filesystem** (a thread is
  mined iff `reference/atariage/<topic_id>-*/notes.ja.md` exists) → `reference/atariage/MINED.csv`.
  `--check <url|topic_id>` reports MINED (exit 1) / NEW (exit 0). Dedup keys on **topic_id**, so a
  different slug for the same thread is still caught. Stops re-mining the same thread across
  sessions/agents without relying on a hand-kept list.
- **`aa_fetch.py` auto-dedup**: skips a thread (no fetch, no stray dirs) if its topic_id is already
  mined; `-force` overrides. Mining is now mechanically dedup-enforced, not by memory.

### Changed
- `.gitignore`: ignore stray standalone `cmd/*` binaries built to the repo root
  (`/rammap` `/jingle` `/dissect` `/fieldtest` `/calibrate` `/scenario` `/stellacheck` `/ingest`) —
  build to `/bin` instead. (Removed a stray 12 MB `rammap` binary from the working tree.)

## [1.64.0] - 2026-06-13

### Added
- **Technique: instrument-envelope music driver** (`roms/techniques/music_driver.asm` +
  `docs/techniques/music-driver.md`): the step up from the constant-volume sound driver — AUDV is
  driven by a **per-instrument volume envelope every frame** (attack/decay → sustain, or
  decay-to-silence for plucks) and **each note selects its own instrument**. Data is the
  TIATracker model reduced clean-room: instrument `{AUDC, env offset, sustain}` over a flat `Env`
  table, parallel `Notes/Inst/Durs` patterns, looping song, `Env[0]=0` silence cell for rests.
  10 bytes of zero-page state; tick in overscan under TIM64T. Distilled from TIATracker
  (kylearan, forums.atariage.com/topic/250014 → `reference/atariage/250014-tiatracker/`).
  CI: `scenarios/music_driver.json` (envelope ramps 15→12→10→8 / 11→9→7, sustain holds, per-note
  instrument switch to pluck, pluck decay-to-silence with bass sustaining independently, song loop
  back to C5, 262 lines, audio golden). Hardware-calibrated via read_audio. = technique candidate ⑦.

### Mined (research, non-repo `reference/atariage/`, clean-room)
- AtariAge deep-dive run (depth-first, 11 threads distilled to `notes.ja.md`): TIATracker (⑦),
  fast-divide-by-seven (⑮), bus-stuffing (scope-out), raycasting (⑫ split a/b),
  48px-positioning (⑯), screen-resolution (constants cross-check), disassembling (dissect notes),
  castlevania-port (⑰), modular-kernel (⑱ RTS-stack dispatch), pointer-optimization,
  tiatracker-plus. Ledger updated; only ⑦ has been implemented+verified so far.

## [1.63.0] - 2026-06-12

### Added
- **Technique: room-based map navigation** (`roms/techniques/rpgmap.asm` +
  `docs/techniques/rpgmap.md`): the RPG/adventure backbone — a 2×2 world where each room is a
  wall table, the player walks (SWCHA + PosObject), and edge crossings transition rooms
  (`room ^= 1`/`^= 2` with wrap). Adding rooms is pure data. Distilled from za2600's
  kworld/rs/spr (`reference/2600-technique-sources/za2600/`, from the legacy ATARI AR folder).
  CI: `scenarios/rpgmap.json` (walk right→room 1, down→room 3, reflect, 262, golden).

### Note
- This completes the AR-folder technique trio (text24 ⑩ / hscroll ⑪ / rpgmap ⑬) studied from
  the recovered za2600 + sidescroll sources. Candidate ⑫ (raycasting) still lacks a source.
## [1.62.0] - 2026-06-12

### Added
- **Technique: horizontal playfield scroll** (`roms/techniques/hscroll.asm` +
  `docs/techniques/hscroll.md`): coarse 4px scroll via an 8-phase precomputed (PF0,PF1,PF2)
  table (PF bit-order quirks baked in), reflect mode, scrollSpeed-paced. Studied from the legacy
  ATARI AR Side-Scroll source. CI: `scenarios/hscroll.json` (phase progression, reflect, 262,
  golden); read_row confirms 4px-per-tick stripe motion.

## [1.61.0] - 2026-06-12

### Added
- **Technique: 24-character text line** (`roms/techniques/text24.asm` + `docs/techniques/text24.md`):
  doubles text12 to 24 chars by alternating two 12-char blocks across frames (left block P0=39,
  right block P0=87 = +48px contiguous) at 50% flicker. Studied from za2600's text24.asm
  (`reference/2600-technique-sources/za2600/`, recovered from the legacy ATARI AR folder).
  CI: `scenarios/text24.json` (block positions, packed buffers, 262, golden).

## [1.60.1] - 2026-06-12

### Fixed
- `aa_index.py` parser rewritten against the real IPB4 markup (verified on a live snapshot):
  title inside the nested span, `data-stattype` for replies/views, row split on the actual
  item class. One index page now yields ~49 clean topics (was 0-2 with polluted titles).

## [1.60.0] - 2026-06-12

### Added
- **`scripts/aa_index.py` (functional WIP)** — forum-wide topic catalog from Wayback index-page
  snapshots (title/author/replies/views CSV, views-sorted = digging-value ranking). The CDX
  enumeration and fetch loop work (50 archived index pages of the 2600 Programming forum);
  the IPB list parser only captures a fraction of rows and pollutes some titles — **parser
  iteration is the named next step** (recorded in reference/atariage/README.md).

## [1.59.0] - 2026-06-12

### Added
- **Technique: 48px bitmap zone with window scrolling** (`roms/techniques/bitmap48.asm` +
  `docs/techniques/bitmap48.md`): six bottom-up column tables + per-frame `ColK+offset`
  pointers = a logo/message band that scrolls through a taller bitmap (RevEng's Bitmap
  Minikernel idea, own implementation). Completes the 48px family: one verified choreography,
  three data feeds (digits / packed text / bitmap window). CI: offset animation incl. bounce,
  262, golden.

## [1.58.0] - 2026-06-12

### Added
- **Technique: 12-character text line** (`roms/techniques/text12.asm` +
  `docs/techniques/text12.md`): flicker-free text via the verified 48px 6-store choreography
  with a 4×5 font packed two characters per player byte (column-major zp buffer, strings
  pre-encoded as glyph indices). The catalog's biggest gap (menus/messages) closed at the
  sweet spot of the width ladder researched in the AtariAge 32-character thread (12 needs no
  RESP re-strobing and no flicker). CI: `scenarios/text12.json` (packed-buffer bytes, positions,
  262, golden). Wider variants (24 column-flicker / 32 interleaved) recorded as candidates with
  measured constraints.

## [1.57.1] - 2026-06-12

### Changed
- `aa_fetch.py` defaults to **lean storage**: raw HTML cache is deleted after parsing (Wayback
  itself is the permanent archive — re-fetchable anytime), attachments are listed in thread.md
  but only downloaded with `-attachments`, and `-keep-raw` opts back into caching. Keeps only
  the distillate (thread.md / gaps.md / notes). Demonstrated: a 2-page topic harvests to 80KB.

## [1.57.0] - 2026-06-12

### Added
- **`scripts/aa_fetch.py` — AtariAge thread-mining pipeline** (Wayback-first): the live forum
  sits behind a Cloudflare bot challenge, so the tool enumerates snapshots via the CDX API
  (both old/new domains), caches raw pages, parses IPB posts into a single `thread.md`
  (author/date/body), and recovers attachments (attachment.php redirects need
  status-filterless CDX + replay-URL following with retries). Gaps are reported for cookie/
  manual fallback (`AA_COOKIE` env supported; no passwords). First run: Medieval Mayhem topic —
  17/17 pages, 400 posts, dev-build ROMs recovered and analyzed with fieldtest/dissect
  (analysis artifacts stay in the non-repo reference/ area per the clean-room policy).

## [1.56.0] - 2026-06-12

### Changed
- Strengthening-run U wrap-up: summary section in `docs/improvement-roadmap.md` (P1-P4, 13
  harness releases + starshot v1.0 dogfood in the roms repo). Techniques catalog now covers the
  full real-game skeleton: score, SFX, sound driver, game states, bullets, paddle, procgen,
  bank template — every entry with a verified ROM + scenario + golden.

## [1.55.0] - 2026-06-12

### Added
- **`cmd/rammap`** (V2-18 closed): per-frame RAM diff over N frames → markdown usage map
  (address, change rate, value range, constant/per-frame hints). Feeds `docs/ram-maps.md` and
  audits our own ROMs' RAM budgets.
- **`scripts/check_gopher_pin.sh`** (F-2 closed): verifies the local Gopher2600 clone matches the
  CI-pinned SHA. Hardening-roadmap statuses updated (A-1/S-4/F-2/F-4/V2-18 all ✅).

## [1.54.0] - 2026-06-12

### Added
- **Stella oracle v2 — pixel compare** (`stellacheck -pixels` / `-snap`, F-4 closed): captures a
  Stella debugger `savesnap` PNG and compares it against Gopher2600's frame as TIA color codes,
  using a **measured Stella NTSC palette** (`internal/ingest/palette_stella.go`,
  `NewStellaNTSCQuantizer`) captured live from the new `litmus_palette.bin` (white marker + all
  128 colors, one per line). A shared quantizer misreads Stella's slightly-different RGB as
  ±1-luma errors (86.5%); with the measured palette: **100.00% agreement on litmus_pf
  (34,240 cells)**. `scripts/stella_oracle.sh <rom> <frames> pixels` runs it hands-free.

## [1.53.0] - 2026-06-12

### Added
- **Verification sweep — four documented-but-unverified facts closed** (`docs/fundamentals-audit.md`
  updated to ✅, each with a litmus + scenario):
  - `litmus_hmxx_freeze`: on Gopher2600, **HMxx is latched at the HMOVE strobe** — post-HMOVE
    rewrites (+6/+15/+33 cy) never alter in-flight movement. The 24-cycle rule stays as a
    real-hardware portability constraint.
  - `litmus_score_pfp`: **PFP dominates SCORE** — CTRLPF $06 renders identically to $04
    (PF in COLUPF on both halves, priority over players); SCORE coloring only without PFP.
  - `litmus_vdel_2lk`: the 2LK alignment relation pixel-exact — **VDELP0=1 shifts P0 +1 line**
    to align with odd-line-written P1 (read_row 137→138).
  - Shear-safe write window (cycles 0–22) closed by derivation from verified beam constants +
    litmus_48px6's measured mid-line choreography.

## [1.52.0] - 2026-06-12

### Added
- **`read_audio` note names** (A-1 closed): each channel now reports `note`/`cents` via
  `pkg/audio.NearestNote` — audio state is discussable by name ("ch0 is C5 +0.2¢"), not just
  raw AUDC/AUDF. Verified against the sound-driver ROM (C5/C4 exactly as composed).
- **Sprite shape in the annotated screenshot** (S-4 closed): `get_screen_annotated` draws the
  *current GRP bit pattern* (REFP-reflected, NUSIZ-width-scaled) at each player's marker
  position — mid-frame stops show exactly what byte the TIA is holding, cross-checked against
  `read_tia_registers.gfx_new` ($CC ⇒ the visible 2-2-2 pattern).

## [1.51.0] - 2026-06-12

### Added
- **Source-line debugging** (`internal/srcmap`, U-M9): `assemble_and_load` now assembles with
  DASM `-l`/`-s` and builds a PC → (nearest label + offset, source file:line) map. Tool outputs
  gain an `at` field: `assert_line_budget` (the overrunning code's location — e.g.
  `Burn+5 (litmus_overrun.asm:66)`), `trace_clocks` (every instruction), `watch_ram` (the
  writing instruction), `read_cpu` (current PC). `.bin` direct loads are unaffected (no map).
  Unit-tested parser + end-to-end coverage in `scripts/mcp_smoke.py` (overrun must report its
  source line). Flat 2K/4K only (banked ROMs return no `at`).

## [1.50.0] - 2026-06-12

### Added
- **Technique: bank-switched game structure** (`roms/techniques/banked_game.asm` +
  `docs/techniques/bankswitching.md`): the F8 template — per-bank reset stubs/vectors, a
  reusable `jsr $FF80` cross-bank trampoline, and the data-bank pattern (bank-1 loader copies
  level tables into zero page; bank-0 kernel renders from RAM). CI: `scenarios/banked_game.json`
  (load contents byte-exact, level switch, bank.number==0 at frame boundaries, golden).
  Recorded trap: **instruction fetch on $FFF8/$FFF9 switches banks** — placing the trampoline's
  `rts` on a hotspot caused a reboot loop (350-line frames); diagnosed via `watch_ram` writer PCs.

## [1.49.0] - 2026-06-12

### Added
- **Technique: procedural generation** (`roms/techniques/procgen_demo.asm` +
  `docs/techniques/procedural.md`): event-driven Galois LFSR (the litmus_lfsr form) mapped to
  spawn positions by mask+offset, with the sequence cross-checked against an off-target
  reference implementation. CI: `scenarios/procgen_demo.json` — four spawns assert RAM state
  AND rendered X exactly ($5A → $2D,$98,$4C,$26 / X 61,40,92,54), golden. Same seed = same world.

## [1.48.0] - 2026-06-12

### Added
- **Technique: paddle input** (`roms/techniques/paddle_demo.asm` + `docs/techniques/paddle.md`):
  the dump/charge/per-line-count kernel (VBLANK=$82 discharge → release at visible start →
  count lines until INPT0 D7) with the value mapped to a PosObject-placed bar. CI:
  `scenarios/paddle_demo.json` — paddle 0.1/0.25/0.5 measure exactly 0/63/170 lines (litmus
  transfer curve, shifted by the dump-release line) and the bar X follows (clamped), golden.

## [1.47.0] - 2026-06-12

### Added
- **Technique: missiles as bullets** (`roms/techniques/bullets.asm` +
  `docs/techniques/missiles-bullets.md`): RESMP spawn-at-player, sentinel-encoded row-range
  flight (kernel stays under the line budget on the active path), CXM0P hit handling.
  CI: `scenarios/bullets.json` (spawn at ship+4, flight, latch, hit bookkeeping, golden).
- **`litmus_resmp` — RESMP verified**: unlock places the missile at **player+4px** (1x center),
  follows HMOVE moves, and the lock must be **held ≥1 frame** (same-pass lock+unlock does not
  move the missile). Plus three recorded traps: collision *read* addresses decode the low nibble
  ($32 reads CXP0FB, not CXM0P=$30); PosObject fine adjust is `eor #7` (not `eor #$FF`); active-
  path-only line-budget overruns show up as frame-length changes (350-line frames).

## [1.46.0] - 2026-06-12

### Added
- **Technique: game state machine** (`roms/techniques/game_states.asm` +
  `docs/techniques/game-states.md`): title/play/game-over skeleton with edge-detected console
  switches, SELECT variants, difficulty-dependent round timing, attract mode, deterministic
  state entry, frame logic under TIM64T. CI: full-lifecycle scenario (~1100 frames, golden).
  Dogfooded: `fieldtest -auto` detects this ROM's title via `auto-start: reset`.
- **`litmus_swchb` — SWCHB read side verified** (D0/D1 active-low, D3 color, D6/D7 difficulty):
  `emu.SetPanel` extended with `color`/`p0pro`/`p1pro`, and scenario `inputs[]` now accepts panel
  actions (`reset`/`select`/`color`/`p0pro`/`p1pro`). `docs/fundamentals-audit.md` input section
  updated to verified.

## [1.45.0] - 2026-06-12

### Added
- **Technique: in-game sound driver** (`roms/techniques/sound_driver.asm` +
  `docs/techniques/sound-driver.md`): looping 2-voice music from jingle-compatible tables with
  **SFX preemption of channel 1 and automatic restore**; driver tick runs in overscan under
  TIM64T (constant calibrated by scenario line-count sweep). Verified by `dissect -audio`
  round-trip (transcription == composition on both voices) and frame-exact preemption/restore
  asserts. CI: `scenarios/sound_driver.json` (+ audio golden).

## [1.44.0] - 2026-06-12

### Added
- **Technique: sound effects** (`roms/techniques/sfx_demo.asm` + `docs/techniques/sound-effects.md`):
  SFX as frame tables (2 bytes/frame) generated by new `pkg/audio` helpers `PitchSweep` /
  `NoiseBurst` / `Blip` / `Arpeggio` / `EmitSFX` (unit-tested). Five standard recipes (laser,
  explosion, pickup, bounce, engine) + a ~40-cycle overscan player. CI: `scenarios/sfx_demo.json`
  — 14 register-exact asserts across all five effects (all passed first run) + audio-digest golden.

## [1.43.0] - 2026-06-12

### Added
- **Technique: 6-digit score kernel** (`roms/techniques/score6.asm` + `docs/techniques/score-kernel.md`):
  BCD 3-byte score + per-frame font-pointer build + the litmus_48px6 VDEL 6-store choreography with
  `(zp),y` fetches (stores at 55/58/61/64 cy → whole block repositioned +63px to P0=87/P1=95; gap
  relations preserved). `pkg/sprite.DigitFont()` for Go-side reuse. CI: `scenarios/score6.json`
  (positions, BCD carry at frames 99/150, 262 lines, golden).

## [1.42.0] - 2026-06-12

### Added
- **Music transcription** (`cmd/dissect -audio N`): samples TIA audio registers (AUDC/AUDF/AUDV,
  both channels) at frame granularity from reset and emits each channel as jingle notation
  ("D6:80 F6:40 R:6 ..."), with per-note AUDF/cents. New `pkg/audio.NearestNote` (12-TET inverse
  of `FindNote`, unit-tested). **Round-trip verified**: transcribing our own single- and two-voice
  fanfare ROMs reproduces the input melodies note-for-note on both channels (repeated equal
  pitches merge legato — register-identical, acoustically the same). Demo: a commercial title's
  theme transcribed with names + frame durations (output kept in inbox per clean-room policy).

## [1.41.0] - 2026-06-12

### Added
- **`cmd/dissect` bank-aware matching (F8/F6/F4)**: for carts >4K, matches are reported as
  "bank N $Fxxx-$Fxxx" (bank-relative in the $F000-$FFFF window) instead of a wrong flat address.
  Ground-truth verified with a purpose-built F8 ROM (Art table planted in bank 1 at $F200 →
  reported exactly as "bank 1 $F200-$F207"); field-checked on a commercial 8K title (asset tables
  resolved per bank, computed wireframe data correctly left unmatched). DiStella annotation is
  skipped with a note for banked carts (DiStella v2.10 supports 2K/4K only).

## [1.40.0] - 2026-06-12

### Changed
- **All generated output is now English**: ingest text reports (`internal/ingest/textreport.go`),
  fieldtest/dissect/stellacheck CLI messages, and jingle-generated ASM comments. Go source comments
  stay as-is (repo convention); only user-visible output strings changed. Existing inbox artifacts
  were regenerated/rewritten in English (reports, summaries, READMEs).

## [1.39.0] - 2026-06-12

### Added
- **`cmd/jingle` two-voice support** (`-notes2`/`-vol2`/`-type2`): both TIA channels driven
  independently (AUDC1/AUDF1/AUDV1, per-voice auto-picked sound type, automatic rest padding for
  loop sync). Verified numerically via `read_audio`: both channels sound the expected harmony
  pair (e.g. F6/A5) at the expected frames. Generated-ASM comments and CLI output are English.

## [1.38.0] - 2026-06-12

### Added
- **`cmd/dissect` — runtime trace × ROM byte matching** (disassembly-driven asset extraction; the
  preferred path when the ROM exists, superseding pixel analysis): instruction-steps N frames recording
  every TIA graphics-register store (GRP/PF/COLU) with PC + scanline, groups them into streams, and
  locates each table's **ROM address** (trying trimmed-blank / run-length-collapsed / reversed variants),
  rendering sprites as ASCII art. Constant streams are reported as immediates (false-positive guard).
  `-distella` merges `; dissect:` annotations into a DiStella disassembly at the nearest preceding label.
  Validated on ground truth (vertical_pos art table found at its exact address) and on a commercial
  title (player sprite incl. reversed storage + per-row color table + PF table; output kept local per
  the clean-room policy). Research notes + future ideas: `docs/improvement-roadmap.md`.
- `internal/emu`: CPU register accessors `PC`/`A`/`XReg`/`YReg` and `PeekROM` (memory peek without
  side effects) to support instruction-level tracing.

## [1.37.0] - 2026-06-12

### Added
- **fieldtest v2**: console panel switches (`emu.SetPanel` reset/select; `-press reset@30`),
  **auto-start escalation** (`-auto`: capture → if no dynamic objects, RESET → fire →
  fire+hold-right, reporting which attempt started the game — verified live: E.T. needed RESET,
  Outlaw needed fire+hold-right), and **inbox organize mode** (`-inbox dir`: each X.bin moves
  into X/ with overlay/report.txt/report.json inside — the standing structure, documented in
  inbox/README.txt). Batch-ran 9 ROMs end-to-end.

## [1.36.0] - 2026-06-12

### Added
- Recovery-run wrap-up: routing table entries (ram-maps, dynamic-multisprite), mcp_smoke now
  exercises all five new tools end-to-end, serverInfo version bump added to the release
  checklist in CLAUDE.md, open-backlog ledger CLEARED (remaining items are single user
  actions, each fully prepared). Summary at inbox/recovery_report.txt.

## [1.35.0] - 2026-06-12

### Added
- **Composing-session groundwork**: `pkg/audio.NoteFreq/FindNote` (12-TET note names →
  best (AUDC,AUDF) with cents error, Slocum tuning) and **`cmd/jingle`** — melody notation
  (`"C5:30 E5:30 G5:30 C6:60 R:30"`) → a playable looping ROM in one command (auto-picks the
  sound type that fits the whole melody within ±60 cents; assembles via dasm when present;
  per-note cents annotated in the generated source). Verified: register sequence
  AUDF 29→23→19→14 matches the documented C6 spot value; 262 lines held. The joint session is
  now "hum it → ROM in 30 seconds → listen together in Stella".

## [1.34.0] - 2026-06-12

### Added
- **`cmd/fieldtest` — ROM self-driving field tests (input contract v3).** Given a ROM file, the
  harness runs it in Gopher2600, captures K frames (with optional input injection
  `-press right@60,fire@90`), and emits the full multi-frame analysis (overlay/report.txt/json).
  Screenshots are no longer required when a ROM exists — F12 becomes the fallback. Verified
  end-to-end on dyn_multisprite (4 frames, fidelity ~100%).

## [1.33.0] - 2026-06-12

### Added
- **`scripts/stella_oracle.sh` — the Stella cross-check, hands-free.** Launches stellacheck and
  sends the debugger key to Stella via AppleScript in parallel; preflights the one-time
  Accessibility permission and prints setup instructions when missing (the manual-keypress flow
  remains as fallback). The last human step in the oracle loop is now a single one-time
  permission grant.

## [1.32.0] - 2026-06-12

### Added
- **MCP `trace_clocks`** — sub-instruction beam anatomy: each of the next N instructions with
  PC, opcode, CPU cycles, and start/end (scanline, color clock). The practical recovery of the
  parked step_clock (observation without suspension). **First catch:** the mid-line HMOVE
  table's strobe clocks were hand-estimates (≈1/73/130); trace_clocks measured 13/85/142 —
  fundamentals-audit corrected. Rule 2 extended to clocks.

## [1.31.0] - 2026-06-12

### Added
- **Ingest R3 — mid-scanline COLUPF as a first-class citizen.** Bands whose lit columns change
  color mid-half now carry `color_writes` ([{clock,color}] — faithful timed-write register
  semantics, exactly how you'd author it), the renderer replays them, and the text report prints
  them as `; COLUPF timed write: clock N -> $XX`. The previously "documented limit" is now
  modeled: **Pitfall's static layer 98.56% → 99.90%** (8 bands gained writes). Synthetic CI
  proof: a two-color half extracts write@clock48 with fidelity 100%. Half-boundary-only changes
  (score mode) still use ColorLeft/Right — no churn for existing data. inbox reports regenerated.

## [1.30.0] - 2026-06-12

### Changed
- **dyn_multisprite polish**: all five objects now have distinct X (DelTbl 1..5 — enabled by a
  −2-cycle state-flag dispatch: draw state = $80 so one `bmi` replaces cmp/beq); the documented
  position mapping now matches measurement (X = 33+15d on slot A, 36+15d on slot B; the 3px
  slot difference is the A/B dispatch asymmetry, now documented); scenario asserts strengthened
  with deterministic ys at two fixed frames; goldens regenerated.

## [1.29.0] - 2026-06-12

### Fixed
- **Exerciser scene-entry line transients eradicated** (debt since v1.2.0): title entry 263
  (music init moved into the half-empty HMCLR line), zone entry 264 (the 6-element X-table
  copies ran ~82 cycles — split 3+3 across the init's six lines), gradient entry 263 + a 263
  every 4th frame (the kick envelope's every-4th-frame branch jitter — now a branchless
  per-frame `AUDV0 = sfxTmr>>2` with identical envelope, and the entry-frame kick register
  writes moved past the first WSYNC, flagged by sfxTmr==40). **All five scenes now hold 262
  on every frame including entry** (full per-scene map probed). Goldens regenerated.

## [1.28.0] - 2026-06-12

### Added
- **V2-18 RAM-map audit** — `docs/ram-maps.md`, auto-extracted zero-page equates per ROM.
- CLAUDE.md tool list updated (analyze_screen / run_scenario / watch_ram; parked items noted);
  MCP serverInfo now tracks releases (1.28.0). Open-backlog ledger updated with v1.19–v1.28
  results. Overnight summary at `inbox/overnight_report.txt`.

## [1.27.0] - 2026-06-12

### Decided
- **Ingest M-I — static-layer residual diagnosed and documented, not papered over.** Pitfall's
  98.6% static reconstruction loses its 1.4% in canopy-fringe rows where two colors share one
  playfield half on one scanline = the game writes COLUPF mid-scanline. The band model keeps
  one color per half on purpose (per-column colors would misrepresent register semantics);
  the documented guidance is to author such rows as timed-write kernels. Diff-row histogram
  methodology recorded in docs/ingest.md.

## [1.26.0] - 2026-06-12

### Added
- **Ingest M-H — position-continuity union tracks + animated-PF hints.** The union links
  sprites across frames by proximity (≤20px; Pitfall's Harry runs up to 18px/frame) and shared
  colors — an animating mover is now ONE track with a `poses` count (Harry: 1 track, 4 poses,
  not flicker). `flicker` is redefined to "blinking in place across skipped frames" only
  (the four flicker balls keep their flag; vanished/appeared tracks count as gaps). Fully
  grid-aligned dynamic cells get an `animated_pf?` hint — the Exerciser's scrolling starfield
  is CI ground truth (mountains stay static reflect bands).

## [1.25.0] - 2026-06-12

### Added
- **Technique #10b — dynamic multi-sprite kernel, the full form** (`dynamic-multisprite.md`,
  demo `dyn_multisprite.asm`; suite now 50). 5 crossing objects through 2 players: 9-comparator
  sorting network (deterministic cycles), dynamic 2-of-N slot queues with 0-sentinels and
  per-frame fairness flip, mid-screen timed-RESP repositioning on the coarse grid, and a
  **TIM64T-managed VBLANK** (sort+assign vary 60–160 cycles by path — un-paddable; the
  real-game idiom now verified here). Zero visible budget spills over 10 frames by
  instruction-level interval enumeration; all 5 object colors proven rendered via multi-frame
  ingest. War stories recorded: a POSITION path at exactly 76 cycles (the closing WSYNC itself
  crossed) fixed by a fall-through reorder worth −3 cycles.

## [1.24.0] - 2026-06-12

### Added
- **VDEL odd/even verified** — `two_line_vdel.asm`: in a 2-line kernel (GRP0 on line A, GRP1 on
  line B), setting `VDELP0 = y&1` parks the GRP0 write in the shadow register until the GRP1
  write — the sprite starts on odd scanlines with the kernel unmodified. CI pixel proof: top
  edge moves exactly +1 scanline per frame (TestVDELOddEven). Suite now 49.

## [1.23.0] - 2026-06-12

### Added
- **skipDraw (DCP) verified** — `vertical_pos_dcp.asm`: the classic undocumented-opcode vertical
  trigger (`lda #H-1 / DCP sprDraw / bcs`), encoded via `.byte $C7` (DASM has no illegal
  mnemonics). Measured against the compare version on the same kernel: max line 40→38 cycles,
  sprite line 31→30 — modest here; the idiom's real value is freeing Y. Pixel-identical motion,
  CI-locked. Suite now 48.

## [1.22.0] - 2026-06-12

### Added
- **litmus_hmove_mid — mid-line HMOVE measured** (documented→verified). With HM registers
  cleared, strobes completing at visible clocks ≈1 and ≈73 shift nothing; ≈130 shifts **−5 px
  left**; no-strobe control 0. Pixel-confirmed (bar edge above/below the strobe line). The folk
  "right 1px/4CLK" summary did not reproduce — recorded as a non-monotonic function of strobe
  time in docs/fundamentals-audit.md; pinned in scenarios/hmove_mid.json. Suite now 47.

## [1.21.0] - 2026-06-12

### Added
- **litmus_bank_f6 / litmus_bank_f4 — F6 (16K/4-bank) and F4 (32K/8-bank) bankswitching
  hardware-verified** (generalizing the proven F8 pattern: vectors + identical reset stub in
  every bank, a byte-exact switch-zone chain at $FF00 visiting bank0→1→…→N→0 each frame).
  Each bank stamps its ID and counter; scenarios assert the last bank's mark, equal counters,
  and bank.number==0 at the frame boundary. Suite now 46. The F4 chain (~130 cycles) spills one
  overscan line — compensated explicitly (ldx #29) to keep 262.
- CLAUDE.md: bank constants note updated (F8/F6/F4 all verified).

## [1.20.0] - 2026-06-12

### Added
- **MCP `watch_ram`** — run until RAM[addr] changes; returns old/new value and the PC of the
  writing instruction (bounded by max_frames). Granularity is per-instruction; same-value
  stores are invisible (documented).

### Decided
- **step_clock parked with findings** (docs/mcp-tools.md): Gopher2600's colorClockCallback can
  observe but not suspend mid-instruction; a color-clock quantum needs an upstream CPU
  micro-instruction refactor. RunUntilBeam/read_cycles/assert_line_budget/watch_ram cover the
  practical cases.

## [1.19.0] - 2026-06-12

### Added
- **MCP `run_scenario`** — the regression runner's verdict callable from the live loop
  (paths[], returns pass/fail with failing-assertion details).
- **MCP `analyze_screen`** — the ingest analyzer applied to the *current emulator frame*
  (no file round-trip): PF bytes, sprite GRP + per-row colors, groups, fidelity, grid overlay.
  Supersedes the long-parked read_sprite_shape idea.
- `scripts/mcp_smoke.py` — sequential MCP smoke driver (the go-sdk serves tool calls
  concurrently; piping a batch races load_rom vs later calls — cost one debugging round).

## [1.18.0] - 2026-06-12

### Added
- **`report.txt` — the human-readable report is now an official tool output** (the author asked
  why the nice ASCII format was one-off). `cmd/ingest` writes it next to `report.json`/
  `overlay.png`: sprite ASCII art with per-row TIA color codes (duplicate rows compressed xN,
  NUSIZ stretch expanded), group list, playfield band table with 40-column previews and
  repaired/SCORE flags, and the DASM snippets. Multi-frame runs get the layered version
  (per-frame dynamic sprites + union + static layer).

## [1.17.0] - 2026-06-12

### Added
- **Image ingestion M9 — multi-frame everywhere.** `cmd/ingest -in a.png,b.png,c.png` and
  `analyze_image {paths: [...]}` run the M8 separation end-to-end; static objects carry
  interpretation hints (`pf_fringe?` when the color matches an adjacent PF band,
  `parked_object?` otherwise); input contract v2 documented (2-3 consecutive F12 shots for
  scenes with movement; N=3 recommended). MultiReport uses a named `static` field (Go embedded
  structs and the MCP schema generator don't mix — second schema gotcha after []uint8).

## [1.16.0] - 2026-06-12

### Added
- **Image ingestion M8 — multi-frame separation** (the author's architectural point: M7's
  reference-pattern repair doesn't generalize; this does). Feed N screenshots of the same scene:
  per-pixel voting builds the **static layer** (playfield/background/parked objects — leaf
  fringes, pit holes, ladders land here correctly as `static_*`), per-frame diffs give the
  **dynamic layer** (real sprites). No repeating-structure assumption. Bonus: **union across
  frames with flicker detection** — 30 Hz multiplexed objects read completely from 2 shots.
  N=2 ties fill from row background (recorded in `unresolved_share`); N=3 recommended.
- CI proofs from our own ROMs: flicker_multiplex 2 frames → all 4 balls in the union, each
  flagged flicker, per-frame fidelity 100%; sprite_anim → walker tracked moving +1px/frame, not
  misflagged; pf_modes static scene → bands identical to single-frame analysis, dynamic layer
  empty, unresolved 0.

## [1.15.0] - 2026-06-12

### Added
- **Image ingestion M7 — overlap repair (sprite-guided PF inpainting).** Where sprites cross
  playfield, ownership is locally undecidable; a clean reference band (the same structure
  repeating elsewhere) resolves it both ways: sprite pixels absorbed into PF return to the
  sprite's art, PF bits hidden under the sprite restore from the reference. Conservative: no
  reference → no touch. Synthetic CI proof: a frame sprite over a 3-cycle building pattern
  extracts bit-perfect with all bands repaired and fidelity 1.0.

### Fixed
- Context demotion (M6) demoted whole thin bands, dragging clean columns into the sprite layer
  (caught by the synthetic overlap test) — now **per-column** with per-column color matching.

### Result
- Pizza Boy: **fidelity 100.0%** (from 99.93%), zero contaminated/asymmetric bands; the pizza
  slice's body rows and the courier's belt row recovered exactly (author's two remaining
  complaints). All sprite/PF colors were already real TIA codes (COLUxx values) per row/band.

## [1.14.1] - 2026-06-12

### Fixed
- **annotate grid drew pink artifacts over bright backgrounds** — a latent bug since v0.5.0:
  the semi-transparent grid colors were invalid premultiplied `color.RGBA` values
  (channels > alpha, e.g. {255,255,255,30}); harmless over black (most 2600 screens) but the
  compositor produced pink streaks over bright areas (visible on Pizza Boy's cyan buildings;
  in hindsight also faint on the zone scene). Grid lines now use non-premultiplied
  `dc.SetRGBA`. Affects `get_screen_annotated` and all ingest overlays.

## [1.14.0] - 2026-06-12

### Added
- **Image ingestion M6 — context arbitration, stretch decomposition, grouping.** Thin "playfield"
  rows vertically touching same-colored sprite pixels are sprite strokes (the score digits'
  top/bottom bars) — they demote and the rings reassemble whole (synthetic 3-ring CI test).
  Components 9-16/17-32 px wide try NUSIZ 2x/4x hypotheses (≥90% row conformance) before
  empty-column splitting and 8px-window composites — everything gets GRP data now. Row-groups
  bundle score/gauge runs; identical shapes share an id. Overlay draws numbered bounding boxes.
- **Pizza Boy acceptance (author's checklist): all six criteria met.** Courier = one complete
  sprite (detached hand re-merged; 10 art rows × 2-line kernel), life gauge = one 3-copy entry,
  pizza = standalone sprite, **both cabs = player_2x with identical shape id (GRP'd)**, score =
  one row-group of complete digits, **fidelity 99.93%** (own-ROM suites stay at 100%).

## [1.13.0] - 2026-06-12

### Added
- **Image ingestion M5 — fidelity metric + fragment merging** (author feedback: "if the accuracy
  is too low to use, it's pointless" — so accuracy became a number first).
  - **Reconstruction fidelity**: the report (per-row background + PF bands + sprites) renders
    back to a 160×H plane and is pixel-compared with the normalized input; `fidelity` is in
    every report. CI asserts **100% on our own ROMs** (an extractor that can't reconstruct its
    own renderer's output is buggy); pf_modes allows 0.999 (sprite-over-PF assumption vs the
    priority region).
  - **Fragment merging**: connected components within a 2px gap sharing colors fuse before
    classification (the courier's detached hand, the cab's wheel, multi-part icons). Pizza Boy:
    16 components → 6 objects, fidelity **99.25%** (the remainder is exactly the still-GRP-less
    large objects = M6's job).

## [1.12.0] - 2026-06-12

### Added
- **Image ingestion M4 — `analyze_image` MCP tool.** The full pipeline (normalize → quantize →
  playfield bands → sprite candidates → DASM snippets) callable live; returns the structured
  report plus the TIA-grid overlay inline and at `$ATARI2600_INGEST_PATH`. Found and fixed a
  go-sdk structured-output gotcha: `[]uint8` marshals as base64 (Go `[]byte`) and fails the
  generated array schema — byte sequences in tool outputs are `[]int` now.
- docs/ingest.md (+ja) extended with the extraction layers and MCP usage; README section;
  CLAUDE.md routing + tool list. MCP serverInfo.version now 1.12.0.
- Field test: Pizza Boy F12 shot → 29 playfield bands + 16 sprite candidates end-to-end through
  the MCP tool (full report delivered to the author separately).

## [1.11.0] - 2026-06-12

### Added
- **Image ingestion M3 — sprite extraction.** 8-connected components over the residual layer
  (non-background, non-playfield) classified as player (width ≤8: GRP bytes in pkg/sprite bit
  order + per-row color table), missile/ball (≤4 solid), or large_object (low confidence);
  equal-shape groups at 16/32/64 spacing fold into one NUSIZ entry. DASM GRP tables emitted.
- **PF↔sprite reconciliation:** a grid-aligned sprite (the bouncing ball at x=80) was claimed by
  the playfield layer and fragmented — tiny PF bands (height ≤2, lit columns ≤2) now demote back
  to the sprite layer. Genuine 1-line playfield (starfields) survives via column count.
- Round-trip CI proofs: ball GRP == Art bit-for-bit with canonical colors; walker GRP matches
  phase art through the row-quadrupled kernel (32 rows); litmus_nusiz_copies folds to one
  3-copy/16-spacing entry.

## [1.10.0] - 2026-06-12

### Added
- **Image ingestion M2 — playfield extraction.** Per-row background estimation (global mode color
  with per-row fallback for COLUBK gradients — naive per-row mode inverted figure/ground on rows
  more than half-filled, caught by the mountain round-trip), 4-clock-aligned column folding,
  repeat/reflect/asymmetric half classification, score-mode flagging (same pattern, two colors),
  band compression, and DASM `byte` table emission reusing `pkg/playfield`'s verified bit order.
- Round-trip CI proofs: litmus_pf bands == $10/$80/$01 exactly; pf_modes score band ($66,
  $44-left/$86-right) and wall band ($10) found; Exerciser mountain bands match the live RAM
  band triples (PF0 masked to its displayed upper nibble) with reflect detected.
- Palette canonicalization: codes with identical RGB (e.g. $0C≡$0E here) report as the lowest
  code (`Quantizer.Canonical`).
- CI now assembles roms/techniques + roms/exerciser before `go test` (ingest tests use them as
  ground truth).
- Field result: Pizza Boy buildings extract as repeat-mode PF bands (blue $9E) with concrete
  PF0/PF1/PF2 bytes per 4-line band.

## [1.9.0] - 2026-06-12

### Added
- **Image ingestion M1 — screenshot → TIA raster** (`internal/ingest`, `cmd/ingest`,
  `docs/ingest.md`). The reverse pipeline begins: integer-scale auto-detection (any multiple of
  the 160-clock raster — decided with the author; 320×228 Stella F12 → 2×1), cell-majority
  normalization, palette quantization against the same Gopher2600 `Spec.GetColor` table the
  harness renders with (distance reported; Stella inputs show the expected small constant),
  TIA-coordinate grid overlay reusing `internal/annotate`. Round-trip CI tests: an emulator
  Snapshot upscaled 2×1/2×2 normalizes back **pixel-identical** with distance 0.
- **Image input contract** (docs/ingest.md + CLAUDE.md): grade A = Stella F12 PNG, unmodified,
  TV effects off (integer scale guaranteed, Retina-proof); OS screenshots = conversation grade,
  processed with warnings; hand-off point = umbrella `inbox/` (belongs to no repo).
- Real-image smoke test: Pizza Boy F12 shot → scale 2×1 detected, full color inventory
  (bg $00 79%, buildings $9E, score $FE, courier $CE, …).

## [1.8.0] - 2026-06-12

### Added
- **Technique #12 — Venetian Blinds** (`docs/techniques/venetian-blinds.md`, demo
  `roms/techniques/venetian.asm`, CI-locked; suite now 44). Intra-frame line interleaving: a white
  diamond and a red frame coexist in one 64-line zone through P0 alone — even lines draw A, odd
  lines B, shape *and* color swapped per line before the display window. Zero flicker (60 Hz
  stable), striped look — the Video Chess (Whitehead, 1979) technique. Adjacent rows pixel-verified
  (`[83+2 white]` ↔ `[80+8 red]`).

### Milestone
- **Techniques roadmap complete: 12 of 12 verified.** #1 zones, #2 animation, #3 vertical
  positioning, #4 2-line kernel, #5 48px+score, #6 sound driver, #7 LFSR, #8 PF modes,
  #9 ball+missiles, #10 flicker multiplexing, #11 F8 bank switching, #12 Venetian Blinds —
  each with a CI-locked demo or verified inside the Exerciser. Documented refinements (VDEL
  odd/even, dynamic Y-sort allocation, DCP skipDraw, F6+) remain on call for real games.

## [1.7.0] - 2026-06-12

### Added
- **Technique #10 — flicker multiplexing** (`docs/techniques/flicker-multiplexing.md`, demo
  `roms/techniques/flicker_multiplex.asm`, CI-locked; suite now 43). Four color-coded bouncing
  balls share two players by frame-parity subset rotation (30 Hz each) — the Pac-Man-ghost
  technique; overlap-safe since slots use the any-Y compare kernel (#3 ×2, ~49 cy/line) with
  per-subset colors and one shared HMOVE. **The alternation itself is CI-asserted** across three
  consecutive frames. The full dynamic form (Y-sort + 2-of-N allocation + fairness rotation)
  is documented for when a game needs it.

## [1.6.0] - 2026-06-12

### Added
- **Technique #8 completed — playfield score mode & priority** (`docs/techniques/pf-modes.md`,
  demo `roms/techniques/pf_modes.asm`, CI-locked; suite now 42). Three regions switch CTRLPF
  mid-frame; pixel-verified by read_row: in score mode the same PF1=$66 pattern reads back
  COLUP0-red on the left half and COLUP1-blue on the right; with priority off the red P0 column
  fully covers the yellow wall, with D2 set the wall splits the sprite (62+2/64+4/68+2).
  Together with the already-verified asymmetric PF and reflect, #8 is done.

## [1.5.0] - 2026-06-12

### Added
- **Technique #4 — 2-line kernel** (`docs/techniques/two-line-kernel.md`, demo
  `roms/techniques/two_line_kernel.asm`, CI-locked; suite now 41). Each art row spans two
  scanlines; line A carries P0's vertical compare + a COLUBK gradient, line B carries P1 +
  loop control — the standard headroom structure of real games. Two players staged then moved
  by **one shared HMOVE** (strobing per positioning line re-applies the earlier HMxx — a +3 px
  bug caught by read_tia and documented). Carry-hygiene note: an `adc` inheriting the sprite
  compare's flags jittered the gradient until it became an `ora`. VDEL odd/even (1-px vertical
  granularity inside a 2LK) left documented-only.

## [1.4.0] - 2026-06-12

### Added
- **Technique #3 — Vertical positioning** (`docs/techniques/vertical-positioning.md`, demo
  `roms/techniques/vertical_pos.asm`, CI-locked; suite now 40). Vertical has no hardware — the
  kernel compares `line − sprY` against the sprite height every scanline and feeds GRP0 art or
  zero (single unsigned `cmp` covers above *and* below via underflow; both paths converge on one
  store at ~21 cy). Demo bounces a ball Y 4⇔180 at X=80; pixel rows verified **bit-for-bit**
  against the art via `read_row`. DCP/skipDraw variant documented for cycle-starved kernels.
  Re-confirmed: **position calibration is kernel-specific** (`lda #imm` vs `lda zp` prologue =
  1 cy = 3 px; this ROM's XCAL is −5 where sprite_anim's is −8) — never copy constants, re-measure.

### Fixed
- **`read_row` y-coordinate was off by `visibleTop` (~29 lines)** from the annotated-grid labels
  the tool promises to match (grid = `visibleTop + image row`; the implementation indexed the
  cropped image directly). Static playfield checks were self-consistent, but grid-coordinate
  round-trips missed. `ReadRow` now subtracts `visibleTop` — the y you see on the grid is the y
  you pass. Found while pixel-verifying this technique's demo.
- MCP server `serverInfo.version` was stuck at "0.9.0"; now tracks releases (1.4.0).

## [1.3.0] - 2026-06-11

### Added
- **Technique #2 — Sprite animation** (`docs/techniques/sprite-animation.md`, demo
  `roms/techniques/sprite_anim.asm`, CI-locked by `scenarios/sprite_anim.json` + golden; suite now 39).
  4-phase walk cycle (frame-divided clock, `frameBase` staged in VBLANK, row-quadrupled kernel),
  ping-pong X with **free REFP0 horizontal flip** (asymmetric art so the flip reads), divide-by-15 +
  HMOVE-table positioner **calibrated to `pos(v) = v` exactly** (`XCAL=-8`, organic full-range sweep).
  Documented measurement subtlety: frame-boundary `hmoved_pixel` reads lag one frame (xpos∓1 by
  direction) — observation artifact, not a positioning error; and **calibrate with organic runs, not
  pokes** (poke timing vs frame-boundary anatomy mis-measured ±2 px twice).

### Changed
- `docs/techniques/roadmap.md` synced with reality: the Exerciser had already verified **#5 48px+score**,
  **#6 sound/music driver**, **#7 LFSR**, **#9 ball+missiles**, **#11 bank switching (F8)** (and parts of
  #8; VDEL prereq of #3 now ✅) — 7 of 12 techniques done. Next open items: #3 vertical positioning,
  #4 2-line kernel, #10 general multi-sprite kernel.

## [1.2.0] - 2026-06-11

### Changed
- **Exerciser Procedural scene redesigned: starfield over mountains** (author feedback: the old
  fixed-mask output "looks like a scrolling barcode" — the one-byte-seed magic wasn't visible).
  - Top 111 lines: sparse starfield — draw = (pair of LFSR steps ANDed) & previous line's pair
    (~6% density, any column), scrolling every frame. The old `and #$88/$11` masks confined stars
    to four fixed columns, which is what read as barcode.
  - Bottom 80 lines: a mirrored mountain ridge generated at scene entry from a one-byte seed by an
    AND-cascade (`band[b] = band[b+1] & (r1|r2)`, 10 bands of 8 lines; harsher `r1&r2` masks for the
    top bands, and the top two bands forced empty — consecutive LFSR steps are correlated, which
    otherwise lets a lucky column survive to the ceiling as a tower). Zero picture bytes in ROM.
  - The scene now owns all 192 scanlines explicitly (1+111+80). The old version only strobed 191
    WSYNCs and silently relied on the dispatch line spilling past 76 cycles for the 262 total; the
    rewrite's lighter pre-section broke that assumption (261 lines) before being caught by the
    line-count probe. Generation is spread across entry-frame lines (≤75 cycles each, one extra
    cycle over budget in an early draft was caught by the per-frame probe and moved to its own line).
- docs/exerciser.md: scene-4 row rewritten accordingly. 38 scenarios pass; goldens regenerated.

## [1.1.0] - 2026-06-11

### Changed
- **Exerciser polish from the author's play-test (three QA reports, all confirmed and fixed).**
  1. *Title logo & score were left of center* — the 48px blocks sat at the verified-recipe default (X=24).
     Now centered (P0=56/P1=64), which required **recalibrating the six-store choreography for the new
     display window** (timed stores 44/47/50/53 instead of 34/37/40/43) and rebalancing the kernel: B0/B1
     loads moved into the head, the tail slimmed to `dec row` + B5 staging, and the exit-line cleanups moved
     after their closing WSYNC (the combined exit line ran 77 cycles and spilled a scanline — caught by the
     line-budget probe).
  2. *Zone sprites never reached the right edge* — the drift wrap was `and #$7F` (0–127), inherited from the
     techniques demo. Now wraps properly at 0–159 (full width), with the drift loop re-split two zones per
     line to stay inside the 76-cycle budget.
  3. *The starfield's "reorganize every 64 frames" read as nothing happening* — one LFSR step per second
     only shifted the pattern a single line. The seed now advances every frame: a continuous upward-scrolling
     starfield. 38 scenarios pass; goldens regenerated.

## [1.0.1] - 2026-06-11

### Fixed
- **Exerciser: fire/scene-advance was dead in Stella — paddle scene removed.** Field report (the author,
  playing in Stella): Space did nothing, though it worked before M5 and every Gopher2600 scenario passes,
  including a real-user input-pattern probe. Root cause: **Stella's controller auto-detection** sees the
  ROM's INPT0 reads (the paddle scene), plugs paddles into the left port — and plugged paddles **hold INPT4
  permanently high**, so the joystick fire can never register (the property is also persisted per-ROM,
  which is why `-lc JOYSTICK` didn't rescue the first binary). Per the author's call, the paddle scene is
  removed from the Exerciser (5 scenes; paddle capability remains verified in `litmus_paddle` and the
  harness paddle input path). 38 scenarios pass.

## [1.0.0] - 2026-06-11

**The harness is 1.0.** The declared bar — a trustworthy loop (gaps A–E), a sourced fundamentals audit with
the unknowns measured, a verified techniques catalog, a two-emulator oracle, and **one artifact composing
every capability** — is met:

- **The Exerciser ROM is complete** (M1–M8, v0.56.0–v0.62.0): an 8K F8 cartridge whose six scenes compose
  the 48px six-store kernel + live BCD score + a 2-channel music driver, zone multiplexing over an
  asymmetric playfield, an interactive collision playground, paddle reading, per-scanline color + SFX, and
  LFSR procedural generation — all driven by input-timeline scenarios, locked by video/audio goldens, and
  green in CI on every push (39 scenarios; every scene provably inside the 76-cycle line budget via its
  262-line assertion).
- **Verification surface**: 26 litmus ROMs; the v2 fundamentals backlog closed (Tier 1–3, incl. VDEL, HMOVE
  side effects, asymmetric-PF windows, inputs incl. paddles, F8 bankswitching + `read_bank`, 6502/BCD
  precision, all 15 collision pairs, RIOT timers, mirrors, LFSR, audio sample capture + `pkg/audio`).
- **Cross-emulator agreement**: `cmd/stellacheck` RAM cross-checks PASS against Stella for `smoke` and the
  `litmus_6502` measurement suite (128/128 bytes each). The Exerciser cross-check additionally showed all
  structural state agreeing, with only per-frame counters phase-shifted by the emulators' differing
  frame-boundary cut points — measured and documented in `docs/stella-oracle.md` (sub-frame alignment = v2).
- **Docs**: routing-tabled deep dives (`fundamentals-audit`, `techniques/`, `exerciser`, `stella-oracle`,
  `verified-coverage`), each fact tagged verified/documented with sources.

## [0.50.0] - 2026-06-11

### Added
- **RESBL vs RESPx mid-line re-strobe litmus (v2 V2-11).** `litmus_resp_edge.asm` confirms Towers'
  TIA_HW_Notes: strobing **RESBL twice on one scanline draws two balls** (clocks 38 and 140 — the ball
  re-emits START, the multi-ball trick), while strobing **RESP0 twice draws a single 8px player** at the
  last position only (clock 107 — the player does not re-emit START until the 160-clock wrap). Locked by
  `scenarios/resp_edge.json` (position asserts + golden). 28 scenarios pass.

## [0.49.0] - 2026-06-11

### Added
- **Address-mirror litmus (v2 V2-12).** `litmus_mirror.asm` proves the memory map's mirroring: writing $5A to
  $0180 reads back at $0080 (and the reverse) — i.e. RAM $80–$FF is mirrored at $0180–$01FF, **which is why
  the stack works**; and setting the background through the TIA mirror $0049 colours COLUBK ($84 blue in
  `read_row`). Locked by `scenarios/mirror.json`. 27 scenarios pass.

## [0.48.0] - 2026-06-11

### Added
- **All 15 collision pairs verified in one ROM (v2 V2-8).** `litmus_collide_all.asm` overlaps P0/P1/M0/M1/BL
  (missiles width-8, ball width-8) with a lit PF0 at the left edge so every CXxx pair fires at once;
  `scenarios/collide_all.json` asserts all 15 (`p0_p1, m0_m1, m0_p0, m0_p1, m1_p0, m1_p1, p0_pf, p0_bl,
  p1_pf, p1_bl, m0_pf, m0_bl, m1_pf, m1_bl, bl_pf`) true — superseding the three single-pair litmus in
  coverage. 26 scenarios pass.

## [0.47.0] - 2026-06-11

### Added
- **RIOT timer litmus — answers the audit's open INTIM question (v2 V2-10).** `litmus_timer.asm` records
  INTIM/TIMINT snapshots to RAM: TIM1T counts down 1/cycle (consecutive reads −7 = the read-loop cost);
  after underflow INTIM wraps into the $FF range and keeps decrementing 1/cycle; **TIMINT D7 (timer-expired)
  is set before INTIM is read ($C0), and reading INTIM clears TIMINT ($00 afterward)** — the audit's open
  "does reading INTIM clear D7?" is now answered **yes**. Locked by `scenarios/timer.json`. 25 scenarios pass.

## [0.46.0] - 2026-06-11

### Added
- **LFSR litmus — procedural-generation foundation (v2 V2-9).** `litmus_lfsr.asm` runs an 8-bit Galois LFSR
  (`lsr / bcc / eor #$8E`, the form in DaveC's Random-Dungeon and common game RNGs) and proves its math
  numerically (pure `read_ram`, no rendering): the first 8 values from seed $01 are
  `01,8E,47,AD,D8,6C,36,1B` (matches hand calculation), it **never decays to $00** across a full sweep, and
  its **period is exactly 255** (returns to the seed). Locked by `scenarios/lfsr.json`. 24 scenarios pass.

## [0.45.0] - 2026-06-11

### Added
- **CTRLPF litmus — SCORE / priority / ball width, incl. the audit's open SCORE×PFP question (v2 V2-7).**
  `litmus_ctrlpf.asm` verifies five regimes: SCORE ($02) paints the left half COLUP0 / right half COLUP1
  (split at clock 80); default priority ($00) draws P0 over the playfield; PFP ($04) draws the playfield over
  P0 (player hidden); **SCORE+PFP ($06) renders the playfield as COLUPF — the SCORE colour substitution is
  *suppressed* under PFP — with the player hidden** (this corner is unspecified in the docs and a likely
  emulator-divergence point; recorded as a Gopher2600 measurement, flagged for the Stella oracle cross-check
  V2-17); ball width D4–5 doubles 1/2/4/8 px. Locked by `scenarios/ctrlpf.json`. 23 scenarios pass.

### Fixed
- **`smoke.asm` now clears collisions after init (CXCLR) — removes platform-dependent CI flakiness.** The
  zero-page clear loop incidentally strobes the TIA strobe registers (RESxx, HMOVE) whose effect depends on
  the power-on TIA state and reset beam timing, leaving sticky collision latches that differed across
  platforms (CI caught `TestReadCollisionsNoSprites` reporting M1-PF / BL-PF on the runner while it passed
  locally). A single CXCLR after init forces a clean, deterministic baseline; rendering (hence all goldens)
  is unchanged.

## [0.44.1] - 2026-06-11

### Changed (docs)
- **README reframed to match the evolved goal.** The project is no longer just "a loop to build games" with
  the five gaps A–E closed (phase 1); it is now a **general, verified 2600 capability base** (phase 2) — a
  fundamentals audit + a techniques catalog, each kept honest by the same numeric loop. Updated the opening
  and the gap-analysis section to name these two living documents and the current scope (20 tools, 20+
  regression scenarios), and to state the aim as *general verified competence, not any one game*.

## [0.44.0] - 2026-06-11

### Added
- **6502/6507 precision litmus — Tier 1 of the v2 backlog complete (V2-6).** `litmus_6502.asm` measures
  instruction facts *on the machine itself* via RIOT TIM1T (1 cycle/tick) and pins them in
  `scenarios/cpu6502.json`, all matching 6502.org exactly: **NMOS BCD** $99+$01 → A=$00 with C=1 correct
  while **Z=0 lies** (the documented NMOS unreliability, recorded); **JMP ($xxFF)** takes the page-bug path;
  **LDA abs,X** 4→5 cycles on page cross while **STA abs,X stays 5 fixed** (why store timing in kernels is
  deterministic); **BNE** 2/3/4 (not taken / taken / taken+cross); illegal **DCP zp = 5 cycles** (also
  certifies illegal-opcode support). 22 scenarios pass.

## [0.43.0] - 2026-06-11

### Added
- **F8 bankswitching verified + `read_bank` MCP tool + `bank.*` scenario fields (v2 backlog V2-5).**
  `litmus_bank.asm` is a best-practices 8K F8 ROM (vectors + an identical reset stub in *both* banks, a
  same-address switch zone whose instruction stream stays valid across the hotspot): every frame bank 0
  marks RAM and hotspot-reads $FFF9 → bank 1 writes its own sentinel and returns via $FFF8. Verified:
  Gopher2600 AUTO fingerprints the plain 8K dasm binary as F8; $80 ends every frame as bank 1's sentinel;
  both per-bank frame counters advance in lockstep; the kernel executes in bank 0 at the frame boundary.
  New `read_bank` MCP tool (20 tools now; `Cartridge.GetBank` at PC, with `is_ram`) and `bank.number` /
  `bank.is_ram` scenario fields; `bin/harness` rebuilt and smoke-tested (initialize + tools/list, no panic).
  Locked by `scenarios/bank.json`. 21 scenarios pass.

## [0.42.0] - 2026-06-11

### Added
- **Input-port litmus with an input-timeline scenario (v2 backlog V2-4).** `litmus_input.asm` samples
  SWCHA/INPT4 to RAM every frame; `scenarios/input.json` drives a press/release timeline and asserts the
  numeric readback: no input = SWCHA $FF, INPT4 $BC (D7=1 + open-bus noise — the documented reason to test
  with N only); P0 left = $BF (D6→0); fire = INPT4 $3C (D7→0); **the VBLANK D6 latch holds INPT4 at $3C
  frames after fire is released** while directions release immediately (the control). 20 scenarios pass.
  Paddle charge-timing verification split off as **V2-4b** (needs a paddle path in `set_input`).

## [0.41.0] - 2026-06-11

### Added
- **Asymmetric-playfield write-window litmus (v2 backlog V2-3).** `litmus_pf_async.asm` verifies woodgrain's
  `Playfield_Timing` tables to the pixel: **(A)** early PF1=$AA (cyc 5) + PF1=$55 at cyc 40 renders a true
  asymmetric playfield — left bits at clocks 16–43, right bits at 100–127, exactly as predicted;
  **(B)** a late write completing at cycle 33 while left PF1 is being drawn splits **per pixel**: the first
  5 bits show the old $FF (clocks 16–35 lit) and the last 3 the new $00 — reproducing woodgrain's worked
  example verbatim. Locked by `scenarios/pf_async.json`. 19 scenarios pass.

## [0.40.0] - 2026-06-11

### Added
- **HMOVE side-effects litmus (v2 backlog V2-2).** `litmus_hmove_side.asm` measures three regimes in one
  frame: **(a)** HMOVE right after WSYNC blanks the left 8px **even with all HMxx=0** (the comb — alternating
  strobe/no-strobe lines compared by `read_row`), confirming Towers' HBLANK+8CLK extension; **(b)** HMOVE
  mid-visible (~cycle 39) produces **zero displacement and no comb** for both HM=0 and HM=$10;
  **(c)** HMOVE at line end (~cycle 74) with HMP0=$10 (+1) moves P0 **left 9px per strobe = value+8**
  (the classic late-HMOVE +8 rule, measured numerically) with no comb. (b)/(c) are recorded as
  Gopher2600-measured values pending the Stella oracle cross-check (V2-17). Locked by
  `scenarios/hmove_side.json` (cumulative-position asserts + golden). 18 scenarios pass.

## [0.39.0] - 2026-06-11

### Added
- **VDEL litmus — verifies vertical delay's write-triggered shadow copies (v2 backlog V2-1).**
  `litmus_vdel.asm` proves all three paths in one frame, exactly as Stella PG §6.D describes:
  with VDELP0=1 a fresh GRP0=$FF stays hidden until **a GRP1 write copies P0's new→old** (then P0 renders
  $FF at X=3); with VDELBL=1 ENABL=on stays hidden until a GRP1 write (ball appears at X=2); with VDELP1=1
  GRP1=$3C stays hidden until **a GRP0 write copies P1's new→old** ($3C renders as 4px at clock 41).
  Locked by `scenarios/vdel.json` (vertical_delay asserts + golden). 17 scenarios pass. This is the
  prerequisite for the 48px score kernel and 2-line-kernel vertical positioning.

## [0.38.0] - 2026-06-11

### Added (docs)
- **Fundamentals audit — `docs/fundamentals-audit.md`.** Six parallel research passes over the local corpus
  (Stella Programmer's Guide, woodgrain wiki, Davie's *Newbies*, SpiceWare's Collect, 8bitworkshop,
  21 real-game disassemblies, DaveC's Random-Dungeon), ~22 owner-supplied links (AtariAge threads, 6502.org,
  Slocum's music guide, Stella debugger docs, Pitfall analyses), and independent web research (Towers'
  *TIA Hardware Notes*, Stolberg). Every domain is classified **verified / documented / unknown / caution**
  with sources. Headline corrections: the local cycle-counting guide's position math is approximate (never
  cite); Pitfall disassembly's LeftRandom comment is wrong (bit0, proven by simulation); SpiceWare Step 3
  vs 7 PF-window discrepancy (to settle by measurement); HMOVE comb/late-HMOVE behavior absent from the
  local shelf (Towers adopted as authority). Headline finds: VDEL's write-triggered cross-copy semantics;
  woodgrain's definitive asymmetric-PF write-window tables; Slocum's complete AUDC/tuning data (the parked
  audio-authoring blocker was already on our shelf); F8-first bankswitching consensus + Gopher2600 already
  auto-fingerprints 8K as F8 and exposes `GetBank()` (a `read_bank` tool candidate); Stella debugger is
  scriptable for automated oracle cross-checks (F-4 design v1).
- **`hardening-roadmap.md` § v2 backlog** — 18 prioritized follow-ups (V2-1…V2-18) in three tiers
  (VDEL, HMOVE side effects, asymmetric-PF windows, input, bankswitch + `read_bank`, 6502 precision; then
  matrix completion; then capabilities: audio sample capture, `pkg/audio`, Stella oracle automation).
- **CLAUDE.md constants hardened**: 24-cycle HMxx freeze after HMOVE; stores never pay page-cross
  penalties; NMOS decimal mode C-only; CLD mandatory at init; cycle-counting-guide caution. Routing tables
  link the audit.

## [0.37.1] - 2026-06-11

### Changed (docs)
- **Roadmap reframed as a general-capability TODO (de-anchored from any single game).** The main goal is a
  general, verified, reusable technique toolkit — not one specific game. `docs/techniques/roadmap.md` now
  prioritizes by **general/foundational value × difficulty × prereqs-verified** (instead of "relevance to a
  particular game"), and is an explicit checklist (`- [ ]`) ordered foundational/easy-wins first
  (animation → vertical positioning/VDEL → 2-line kernel → 48-px score → sound → …). A concrete game can be
  picked flexibly as a per-technique testbed; it is no longer the organizing principle.

## [0.37.0] - 2026-06-11

### Added / Changed (docs)
- **Technique #1 promoted to its formal name + a sourced techniques roadmap.** Researched AtariAge / the
  local `reference/docs_atari/` corpus and Wikipedia: confirmed the formal name is **sprite multiplexing**
  (the loop is a **multi-sprite kernel**); DaveC's "zone" is the common vertical-band term, and our demo is
  the *static-zones* form of the general *sort/position/display + flicker* kernel. Rewrote
  `docs/techniques/zone-multiplexing.md` with a formal-name/taxonomy section, a "Refinements & limits"
  section (2-per-line limit, flicker, single- vs 2-line kernel, positioning cost), See-also (48-px sprite,
  Venetian Blinds), and a sourced References list — marking *documented* vs *verified*. Added
  `docs/techniques/roadmap.md`: a prioritized survey of ~12 next techniques (48-px score, 2-line kernel,
  vertical positioning/VDEL, sound, animation, playfield tricks, LFSR, general flicker kernel, Venetian
  Blinds, bank switching) ranked by North-Star (Frogger) value, difficulty, and prereq-verified status.
  Catalog index links the roadmap. Docs-only; no code change (tests/scenarios unchanged).

## [0.36.1] - 2026-06-11

### Fixed
- **Deterministic emulator power-on state — eliminates CI test flakiness at the root.** Gopher2600
  randomizes the CPU/RAM power-on state (`vcs.Env.Random`, used by `CPU.Reset`), so a fresh `emu.New`
  varied run-to-run; cycle/timing tests (`TestCycleCounterExcludesWsyncStall`, `TestStepScanline`) passed
  locally but flaked in CI. `emu.New` now calls Gopher2600's official `vcs.Env.Normalise()`
  (`Random.ZeroSeed = true` + prefs defaults), the method intended for regression testing, before the
  cartridge-attach reset. Result: identical initial state every run (verified 5×/10× stable). Goldens are
  unaffected (the ROMs clear RAM on boot).

## [0.36.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 gets per-zone background colors — a landscape look.** Each zone sets `COLUBK` from a
  `ZoneBG` table (sky-blue → cyan → green → brown), set in HBLANK so it doesn't disturb the per-zone
  positioning, giving 6 colored bands behind the 12 moving sprites. Golden regenerated; 262 lines preserved.

## [0.35.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 now animates — 12 *moving* sprites.** Each zone's X moved from ROM tables into RAM
  (`zx0`/`zx1`) and is updated every frame (P0 drifts right, P1 left, wrapping `and #$7F`), so all 12 sprites
  animate. Demonstrates RAM-backed motion verifiable purely by `read_ram` (the position bytes change frame to
  frame). VBLANK line count retuned to keep the frame at 262 (the per-frame update loop is absorbed). The
  scenario now locks the frame by `golden_frame` only (robust to the moving positions); all scenarios pass.

## [0.34.0] - 2026-06-11

### Added
- **Techniques catalog + #1 Zone (vertical) sprite multiplexing.** Establishes a repeatable pipeline for
  absorbing 2600 authoring techniques: learn (from `reference/`, local) → clean-room implement
  (`roms/techniques/`) → verify numerically (harness) → cross-check (Stella) → lock in (scenario + golden +
  CI) → optionally promote to `pkg/`. First entry: `roms/techniques/zone_multiplex.asm` puts **12 sprites**
  on screen (6 zones × P0+P1) from a 2-player machine by repositioning P0/P1 per zone (divide-by-15 + HMOVE,
  the harness-verified method). Verified on Gopher2600 + cross-checked in Stella; locked by
  `scenarios/zone_multiplex.json`. CI now runs `roms/techniques/scenarios/` too. Catalog index at
  `docs/techniques/`, linked from the routing tables.

## [0.33.0] - 2026-06-10

### Added
- **Coverage batch: NUSIZ quad-width + missile-player collision.**
  - `litmus_nusiz_quad.asm` (`NUSIZ0=$07`, QuadWidth) → `read_row` shows a **32px** continuous span (8px ×4),
    completing the NUSIZ width modes (double/quad) and copy modes (close/three).
  - `litmus_collide_mp.asm` overlaps an 8px-wide missile0 with player0 → `read_collisions` reports
    **`m0_p0=true`** (CXM0P), extending collision coverage to the missile-player pair. (Also documents the
    1px left-edge offset between missile clamp X=2 and player clamp X=3.)
  - Locked by `scenarios/nusiz_quad.json` and `scenarios/collide_mp.json`. 15 litmus scenarios pass.

## [0.32.0] - 2026-06-10

### Added
- **P0-P1 collision litmus (CXPPMM) — extends collision coverage.** `roms/litmus/litmus_collide_pp.asm`
  overlaps player0 and player1 (both clamped to X=3 via HBLANK strobes) drawing `$FF`; `read_collisions`
  reports **`p0_p1=true`**. Verifies the player-player pair the Frogger `OnPad` check actually uses (previously
  only BL-PF was litmus-verified). Locked by `scenarios/collide_pp.json`. 13 litmus pass.

## [0.31.0] - 2026-06-10

### Added
- **REFP (reflected sprite) litmus — rounds out the sprite track.** `roms/litmus/litmus_refp.asm` draws the
  asymmetric ramp with `REFP0=$08`; `read_tia_registers` shows `player0.reflected=true` and `read_row` shows
  the ramp mirrored (row0 `0x80` lights clock 10 = the right end; row4 `0xF8` lights clock 6–10 = right 5px) —
  the mirror image of the non-reflected `litmus_sprite`. Confirms REFP and `pkg/sprite.Reflect` (data-side
  mirror) are equivalent. Locked by `scenarios/refp.json`. 12 litmus pass.

## [0.30.0] - 2026-06-10

### Added
- **Missile/ball position litmus.** `roms/litmus/litmus_missile.asm` enables and positions missile0 and the
  ball in the visible region; `read_tia` reads **missile0=38 / ball=140** and `read_row` shows a 1px vertical
  line at each clock — verifying the harness reads the missile/ball object-position family (the `X = 3N − 55`
  side, complementing the player `X = 3N − 54` litmus_pos). Locked by `scenarios/missile.json`. 11 litmus pass.

## [0.29.1] - 2026-06-10

### Fixed
- **Flaky `TestStepScanline` (surfaced by CI).** The test asserted every single scanline step consumes
  >0 CPU cycles, but a scanline can legitimately be a pure WSYNC-stall pass-through (0 instructions executed)
  depending on beam-phase alignment — not an invariant. Relaxed to assert the **cumulative** cycles across
  40 scanlines is >0 (the CPU makes progress), which is robust. Keeps the CI badge reliable.

## [0.29.0] - 2026-06-10

### Added
- **NUSIZ multi-copy litmus coverage (extends S-2).** `roms/litmus/litmus_nusiz_copies.asm` renders an 8px
  solid sprite at `NUSIZ0=$03` (ThreeCopiesClose); `read_row` confirms **three 8px white spans at clock
  3/19/35 (16px copy spacing)**. Locked by `scenarios/nusiz_copies.json` (golden + `player0.nusiz=3`).
  Deepens verified coverage of the NUSIZ helper beyond double-width. 10 litmus scenarios now pass.

## [0.28.0] - 2026-06-10

### Added
- **CI via GitHub Actions (hardening-roadmap F-1).** `.github/workflows/ci.yml` runs on every push/PR:
  Ubuntu + Go (from `go.mod`) + DASM, clones Gopher2600 at the pinned commit `5d532e88` into `./Gopher2600`
  (the `replace` target), assembles the litmus ROMs (`.bin` are gitignored), then `CGO_ENABLED=0`
  build/vet/test and runs all litmus regression scenarios. No SDL needed — the harness only imports the
  SDL-free Gopher2600 packages, so a static (cgo-off) build covers it. A CI badge is on the README.
  Verified green on Actions (build/vet/test + 9 scenarios, ~1m).

## [0.27.0] - 2026-06-10

### Added
- **PAL frame verification (hardening-roadmap F-3).** `roms/litmus/litmus_pal.asm` emits a proper PAL frame
  (VSYNC 3 / VBLANK 45 / visible 228 / Overscan 36 = 312 lines) and `scenarios/pal.json` (with
  `tv_spec: "PAL"`) asserts the harness drives/counts it as **312 lines** (plus a RAM sentinel). Confirms the
  harness is not NTSC-only; `ntsc_frame_lines` counts the actual per-frame line total (312 for PAL).

## [0.26.0] - 2026-06-10

### Added
- **Golden-audio regression `checks.golden_audio` (hardening-roadmap A-2).** Mirrors the video golden for
  sound: a sha1 audio-chain (Gopher2600 `digest.Audio`) over the timeline is compared against
  `<scenario>.audio.golden`. `internal/emu` gains `EnableAudioDigest`/`ResetAudioDigest`/`AudioHash`
  (symmetric to the video digest); `internal/scenario`'s golden eval is generalized to share video/audio.
  Verified with `roms/litmus/scenarios/audio.json` on `litmus_audio.asm` (deterministic record→match, plus
  numeric AUDC/AUDF/AUDV asserts). All 8 litmus scenarios pass. CLI only; MCP binary unchanged.

## [0.25.0] - 2026-06-10

### Added
- **`pkg/sprite` NUSIZ helper (hardening-roadmap S-2).** `PlayerSize` (OneCopy … DoubleWidth … QuadWidth) /
  `MissileSize` enums and `NUSIZ(player, missile)` / `NUSIZPlayer(player)` compose a NUSIZx byte from intent
  instead of raw bits. **Verified on Gopher2600** with `roms/litmus/litmus_nusiz.asm`: an 8px solid sprite at
  `NUSIZ0=$05` (DoubleWidth) renders **16px wide** (`read_row` clock 4–19 = white len 16) and
  `read_tia_registers` shows `player0.nusiz=5`. Locked by `scenarios/nusiz.json`. Completes the sprite
  authoring trio (S-1 encoder + S-2 NUSIZ + S-3 P0+P1 combine).

## [0.24.0] - 2026-06-10

### Added
- **`pkg/sprite.SplitWide` + P0+P1 16px combine litmus (hardening-roadmap S-3 — flagship).** Split a
  16-wide ASCII design into P0 (left 8) + P1 (right 8) GRP tables, then place P1 exactly +8px to the right
  of P0 for a seamless up-to-16px (or multicolor) character. `roms/litmus/litmus_p0p1.asm` positions the two
  sprites by strobing RESP0→RESP1 three cycles apart in the visible region (= +9px; an HBLANK strobe would
  clamp both to the left edge) then HMOVE P1 left 1 → exactly +8px.
  **Verified on Gopher2600:** `read_tia` shows player0=69 / player1=77 (exactly +8); `read_row` shows the
  solid-16 rows as a **single continuous 16px white run (clock 69–84, no seam gap/overlap)**, with P0-only /
  P1-only / far-edge rows byte-exact. Locked for regression by `scenarios/p0p1.json` (position asserts 69/77
  + golden frame). This proves sprite placement is as numerically trustworthy as playfield — the headline
  capability of the sprite track.

## [0.23.0] - 2026-06-10

### Added
- **`pkg/sprite` — ASCII → player GRP encoder (hardening-roadmap S-1).** A mirror of `pkg/playfield` for
  player graphics: 8-wide ASCII rows → GRP bytes (`EncodeRow`/`Encode`, D7 = leftmost = standard TIA bit
  order), plus `Reflect` for REFP-less mirroring / P0+P1 right halves. Reuses `playfield.ParseASCIIRow`.
  Unit-tested, including that `..XXXX..` = `0x3C` matches the existing hand-coded Monet Frogger lily-pad byte.
- **`roms/litmus/litmus_sprite.asm` + `scenarios/sprite.json` (+golden) — numeric hardware proof.** An
  asymmetric ramp sprite (top `0x80` 1px → bottom `0xFF` 8px) rendered by player0 at X=3. Verified on
  Gopher2600 via `read_row`: the white span widens 1→2→…→8 px from clock 3 (visible lines 96–103), proving
  D7 = leftmost and top→bottom row order are byte-exact. Locked for regression with a golden-frame scenario;
  all litmus scenarios PASS. First step of the sprite track toward the P0+P1 16px flagship (S-3).

### Added
- **Strengthening roadmap (`docs/hardening-roadmap.md`).** A prioritized roadmap for the next phase —
  making the harness stronger beyond gap-closing. Theme A: deepen authoring + verification into the thin
  domains (S = sprites, incl. `pkg/sprite` ASCII→GRP, NUSIZ helper, and the ★ P0+P1 two-sprite combine for
  up to 16px / multicolor characters placed numerically via the X(N) calibration; A = audio, incl. note/
  timbre names in `read_audio` via Gopher2600 `tracker`, a `digest.Audio` golden, and a `pkg/audio` SFX
  helper). Theme B: harden the foundation (★ CI via GitHub Actions, optional Gopher2600 version pin,
  PAL/SECAM verification, Stella oracle cross-check, completing `step_clock`/`watch|trap`/`run_scenario`).
  Theme C: wire upstream Gopher2600 libraries (`recorder`/`regression`/`reflection`). Each item lists where
  to touch + how to verify + size. Cross-linked from the routing tables in CLAUDE.md / README /
  improvement-roadmap. No code changes (implementation in separate sessions).

## [0.22.1] - 2026-06-10

### Added
- **GPL-3.0 `LICENSE`.** The harness embeds Gopher2600 (GPL-3.0) as a library, so the combined work is
  GPL-3.0-or-later. Added copyright and an Acknowledgements section to the README.

### Changed
- **Public-readiness: the published repo is now English-only.** Translated the public surface
  (README + `docs/`×7 + CHANGELOG + CLAUDE.md) to English. The author works in Japanese, so Japanese copies
  are kept locally as `*.ja.md` sidecars (gitignored, never published). Calibrated the prior-art wording to
  "no Atari 2600 MCP found in a public search (2026-06; Atari Lynx = gearlynx exists)" rather than claiming
  "first". Removed the README provenance section. No code changes; build/vet/test green.

## [0.22.0] - 2026-06-10

### Changed
- **Physical spinoff: split the base into a standalone repo `atari2600-harness` (game ROMs move to a
  separate repo `atari2600-roms`).** Under an umbrella folder `260609_atari2600-dev/`, place `harness/`
  (this repo, history preserved) and `roms/` (new repo) as siblings, bound by `go.work`. Remove
  `roms/frogger` from the harness (moved to the roms repo); `roms/litmus` stays as the harness's own
  verification ROMs. **Eradicate the harness→game dependency:** repoint the scenario/emu unit tests from
  frogger ROMs to litmus, and add a new fixture `roms/litmus/scenarios/golden.json` (+`.golden`).
  `.mcp.json`/`.claude` move up to the umbrella (read at Claude Code's project root). Updated CLAUDE.md's
  structure/dev sections to the post-spinoff reality. Verified: harness `go vet`/`go test` green, 4 litmus
  scenarios PASS; on the roms side `gen` + 3 frogger scenarios PASS.
- **Renamed the Go module `github.com/kidsnz/atari2600-dev` → `github.com/kidsnz/atari2600-harness`
  (spinoff prep).** `go.mod` and 9 import files replaced. build/vet/test green, all scenarios PASS.
- **Promoted `internal/playfield` → `pkg/playfield` (spinoff prep).** Go can't import `internal/` across
  modules, so the playfield encoder (universal Atari 2600 knowledge) became a public package. Updated the
  only cross-package importer (`roms/frogger/gen`). Regenerated all scenes (header-comment-only diffs).
  Verified green; all scenarios (3 frogger + 3 litmus) PASS.
- **Documentation freshness audit (spinoff preamble).** Rewrote `README.md` to v0.21.0 reality (old diagram
  = `cmd/probe` + `internal/emu` only → 4 cmds, 6 internals, roms/<game>, 19 MCP tools, gaps A–E all
  closed; fixed the smoke.asm path to `roms/litmus/`). Fixed minor staleness in `improvement-roadmap`,
  `mcp-tools`, `tool-landscape`, and a stale `cmd/genpf` comment in `roms/frogger/gen/asmgen.go`.

### Added
- **Improvement roadmap document (`docs/improvement-roadmap.md`).** Prioritizes next moves to make authoring
  more accurate, from every angle. Central observation = the position litmus is closed but the timing
  *budget* verification is open (gap B is the biggest hole in the real loop). P0 = cycle exposure +
  per-scanline budget guard, P1 = TIA shadow / collision register reads, P2 = verification automation,
  P3 = build-loop shortening, each annotated with verified Gopher2600 API symbols
  (`CPU.LastResult.Cycles`, `TIA.Video.*`, `Collisions`). Also added "untapped reference veins" (R-1 Freeway
  architecture port, R-2 audio recipes, R-3 cycle-cost table, R-4 real-game structure index) and "external
  research" (the biggest finding: Gopher2600 already implements the hardest items as libraries —
  `recorder`/`regression`/`tracker`/`reflection`/`digest`/`rewind` are usable standalone, shrinking P2/R-2
  from "build" to "wire"; License = GPL-3.0; an Atari 2600 MCP was not found in a public search = no known
  prior art, not a claim of being first; G-2 C64 MCPs, G-3 test DSLs, G-4 authoring-tool integration).

## [0.21.0] - 2026-06-10

### Added / Changed
- **A `.asm` source can be specified directly as a scenario `rom` (gap E fully closed).** If a scenario's
  `rom` is `.asm`, it is assembled with dasm before running = "one source → assemble → run → numeric
  asserts → verdict" in one command (`go run ./cmd/scenario foo.json`). Gap E reaches its ideal form.
- **Consolidated dasm invocation into `internal/build` (DRY).** `assemble_and_load` (harness) and the
  scenario `.asm` feature share `build.Assemble`/`build.BinPathFor`. Assemble failures are returned as
  errors (dasm output including the failing line), not swallowed. Sample: `roms/litmus/scenarios/smoke_src.json`.

## [0.20.0] - 2026-06-10

### Added
- **Automatic calibration of horizontal X(N) (B-4 / gap B fully closed).** Turns litmus from a one-off
  manual job into a reproducible sweep→auto-fit. A cooperating ROM (`litmus_pos`: delay `DELAY=$80`,
  SBC/BCS = 5 CPU cycles/unit) is poked across delays, `player0.ResetPixel` is measured each frame, and a
  linear regression recovers slope and offset numerically. Implementation: `internal/calibrate` (`Sweep`,
  `Fit` — robust to the 160 wrap and left-edge saturation via median-delta unwrapping of the longest
  consistent run). Result on litmus_pos: **slope = 3.0000 px/CPU-cycle** (matches the authoritative 3),
  R²=1.0, kernel offset = −18. Verified in `calibrate_test.go`.

## [0.19.0] - 2026-06-10

### Added
- **Golden-frame regression (P2 D-3 / gap D fully closed).** Adding `checks.golden_frame: true` compares the
  timeline's **rendered frame-chain hash** against `<scenario>.golden` = pixel-level regression detection of
  rendering (complements the D-1/D-2 logic/timing regression). Implementation: wire Gopher2600's exported
  `digest.Video` into `internal/emu` (`EnableVideoDigest`/`ResetVideoDigest`/`VideoHash`); `internal/scenario`
  enables it for golden scenarios, resets after warmup (deterministic), and compares to `.golden`.
  `cmd/scenario -update` records/updates the baseline. Sample: `roms/frogger/scenarios/golden.json` +
  committed `golden.golden`. CLI only; `bin/harness` (MCP) unchanged.

## [0.18.0] - 2026-06-10

### Added
- **Scenario runner (P2 / gap D = first step of verification automation. D-1 assertions + D-2 input replay).**
  Declares an "input timeline + numeric assertions" in one JSON and auto-passes/fails it against a ROM.
  `go run ./cmd/scenario <file.json> ...` (exit 0 on all pass, 1 on failure) = a regression base that runs
  in CI **without MCP**. Key design: the assertion vocabulary (`field` strings) maps one-to-one to
  `internal/emu`'s read methods (dogfooding the observation tools as the regression vocabulary). Unknown
  fields are an error (no swallowing typos). Whole-run measurements with side effects are separated into
  `checks{ntsc_frame_lines, max_line_budget}`. Structure: `internal/scenario` (parse + vocab + Run,
  ROM-agnostic) / `cmd/scenario` (thin CLI). Samples under `roms/litmus/scenarios/` and
  `roms/frogger/scenarios/` (including `hop` = `up` input drives FrogY 144→128). CLI only; MCP unchanged.

## [0.17.0] - 2026-06-10
### Added
- **`read_audio` MCP tool (R-2 / audio verification path).** Returns the current TIA audio registers
  AUDC/AUDF/AUDV for both channels as numbers (extends rule 1 "verify with numbers" to audio). Uses
  Gopher2600's exported `Audio.PeekChannels()`. Verification ROM `roms/litmus/litmus_audio.asm`; exact match
  in `emu_audio_test.go`.

## [0.16.0] - 2026-06-10
### Added
- **`assemble_and_load` MCP tool (P3 / build-loop shortening).** Takes an asm path, runs `dasm -f3` via
  `os/exec`, and loads the output `.bin` on success — collapsing `edit→dasm→load_rom`. On failure returns a
  structured `ok=false` + `dasm_output` (failing line) instead of an MCP error, so the model can fix in place.

## [0.15.0] - 2026-06-10
### Added
- **`step_instruction` / `step_scanline` MCP tools (B-2 / intra-frame granularity).** `step_instruction`
  runs exactly one CPU instruction (returns its cycles + coords); `step_scanline` runs until scanline +1
  (returns cycles consumed). A color-clock-granular `step_clock` is unimplemented (`Step` is per-instruction).

## [0.14.0] - 2026-06-10
### Added
- **`read_tia_registers` MCP tool (P1 / closes the rest of gap A).** Returns current values of write-only TIA
  registers directly from Gopher2600 internals (measure instead of inferring color from `read_row`). Confirmed
  PF0=$F0 (upper-nibble-only) behavior.
- **`read_collisions` MCP tool (P1).** Structures the 8 collision latches (CXxx, $30–$37) into named boolean
  pairs. Bit assignment verified against Gopher2600's `collisions.go`; BL-PF positive on `litmus_collide.asm`.

## [0.13.0] - 2026-06-10
### Added
- **`assert_line_budget` MCP tool (the crux of gap B / B-3 = per-scanline cycle budget guard).** Numerically
  catches the failure that silently killed Pong v2 (per-scanline overrun → screen roll). Detection: a WSYNC
  strobe = a `RdyFlg` true→false transition; the scanline delta between strobes = physical lines consumed by
  that logical line. Implemented with exported `RdyFlg` + beam coords in `internal/emu`'s own step loop
  (no debugger driver). Verified with `roms/litmus/litmus_overrun.asm` (`over=true`, `line_cycles=152`); no
  false positives on smoke / frogger.

## [0.12.1] - 2026-06-10
### Fixed
- **`read_cycles` double-counted spinning during a WSYNC stall (v0.12.0 bug).** During a WSYNC stall the CPU
  doesn't execute but leaves `LastResult` in place, so the old per-boundary accumulation over-counted on any
  WSYNC-using ROM. Fix: unify progress through a `stepInstr()` primitive and accumulate only when `RdyFlg`
  was true before the Step (i.e. a real instruction ran). Regression test `TestCycleCounterExcludesWsyncStall`.

## [0.12.0] - 2026-06-10
### Added
- **`read_cycles` MCP tool (gap B = wiring timing into the real loop, P0 step 1 / B-1).** Gets CPU cycles
  from the simulator numerically (first embodies rule 2 outside litmus). Returns `last_instruction_cycles`,
  `cycles_since_mark`, `total_cycles`. Source = `CPU.LastResult.Cycles` accumulated at instruction
  boundaries across all progress paths. Verified via the invariant "executed cycles × 3 == color clocks" on
  WSYNC-free `litmus_cycles.asm` (1 frame = 263×76 = `total_cycles 19988`).

## [0.11.0] - 2026-06-10
### Changed
- **Monorepo reorg: root = harness base / `roms/<game>/` = ROMs (spinoff Phase 1).** Demonstrated the
  game→harness one-way dependency and separated without surgery. Moved game-specific kernel generation
  (`cmd/genpf` + asmgen) into `roms/frogger/gen/` (package main importing `playfield`); litmus under
  `roms/litmus/`. All builds/tests green; `litmus_pf` read_row identical after the reorg.

## [0.10.1] - 2026-06-10
### Added
- **Frogger polish.** Game over / restart (Lives→0 resets Lives=3/Score=0); visual zones (top = goal band,
  bottom = start bank, middle = Monet water).

## [0.10.0] - 2026-06-10
### Added
- **🎉 Playable Monet Frogger (M5).** A frog crosses a river on flowing lily pads over Monet water. A full
  game kernel (`GenerateFroggerASM`) handles ride/drown/win/lives via a state machine; collisions via CXPPMM.
  The model **played it itself** (set_input + peek/read_tia) to numerically verify every mechanic — and found
  and fixed a fatal landing-frame timing bug that way (1-frame grace via `PrevY`).

## [0.9.3] - 2026-06-09
### Added
- **Frog vertical hop.** player0 drawn at variable scanline `FrogY`; edge-detected up/down jumps it ±16 (one
  lane) on press (no auto-repeat). The model operates/observes/judges in a closed headless loop.

## [0.9.2] - 2026-06-09
### Added
- **Collision check (the Frogger core: on a pad vs in the water).** Per-frame `CXCLR` strobe; CXPPMM read via
  `peek $37` (no new tool needed). Set/clear verified frame-by-frame.

## [0.9.1] - 2026-06-09
### Added
- **Full-scene integration.** Flowing lily (player0) + controllable frog (player1) coexist over Monet water
  (per-scanline COLUBK), with separate motion applied to both via one HMOVE.

## [0.9.0] - 2026-06-09
### Added
- **`set_input` tool = joystick injection.** `poke` doesn't work for input (RIOT redrives SWCHA each frame),
  so inject via Gopher2600's `Ports.HandleInputEvent`. Control ROM verifies "input → frog moves" headlessly.
### Fixed
- A `set_input` jsonschema tag starting with `0=…`/`true=…` made go-sdk panic in AddTool; reworded the tags.

## [0.8.1] - 2026-06-09
### Added
- **Monet water + flowing lily sprite integration (M3 step 2).** Per-scanline COLUBK (water) + per-scanline
  GRP0 (lily) both resolved in HBLANK to dodge cycle criticality; drift via per-frame HMOVE.

## [0.8.0] - 2026-06-09
### Added
- **M2/M3 animation groundwork.** Per-frame color-table animation (`GenerateAsymmetricShimmerASM`, with
  TIM64T-timed VBLANK/Overscan) and smooth sprite horizontal motion = water flow (`sprite_flow.asm`, +1px/frame
  via per-frame HMOVE) — establishing that smooth horizontal motion on the 2600 is the sprite's (HMOVE) job.

## [0.7.2] - 2026-06-09
### Changed
- **Promoted the Monet still (M1) to an asymmetric version** (left/right-independent playfield + per-row water
  color). Per-row water (COLUBK) + constant lily (COLUPF), since the asymmetric loop has budget for only one
  per-row color channel.

## [0.7.1] - 2026-06-09
### Added
- **Asymmetric (left/right-independent) playfield capability, hardware-verified.** Transcribed ABB's "repeated"
  asymmetric kernel (72 cy/line, `tay`/`sty` timing). read_row proves one-sided lighting (impossible with reflect).

## [0.7.0] - 2026-06-09
### Added
- **M1 "quiet pond" — the rendering pipeline opens end to end.** First milestone of the north-star ROM. Path:
  ASCII art + color → EncodeSymmetric → asmgen(kernel) → dasm → load_rom → read_row check. `GenerateSymmetricASM`
  generates a self-contained reflect-playfield still with per-row COLUBK water.

## [0.6.0] - 2026-06-09
### Added
- **`read_row` tool (read playfield-lit columns / per-scanline color numerically).** RLE `{clock,len,hex}` of a
  visible scanline. Playfield bit-order litmus (`litmus_pf.asm`) and per-scanline color litmus (`litmus_color.asm`)
  pass; the verified bit-order table is burned into `docs/resources.md` / `CLAUDE.md`. The `internal/playfield`
  package (`EncodeSymmetric`/`EncodeAsymmetric`) self-verifies against the real litmus values in go test.

## [0.5.1] - 2026-06-09
### Added
- **`get_screen_annotated` also saves the PNG to a file** (env `ATARI2600_SCREEN_PATH`) so clients that don't
  render inline images (CLI terminals) can still open the latest frame; VS Code auto-reloads on change. Returns
  `png_path` in the structured Out.

## [0.5.0] - 2026-06-09
### Added
- **`get_screen_annotated` implemented (the user↔model comms channel), as a first-class citizen.** Captures the
  frame to `image.RGBA` (PixelRenderer), draws a TIA-coordinate XY grid + axis labels + sprite markers (Fixed
  Debug Colors) at ×3 nearest-neighbor, and returns **image (ImageContent PNG) + numbers together**. Enables the
  "user points on the image → model translates to registers" round trip.

## [0.4.1] - 2026-06-09
### Changed
- **Distilled core constants into CLAUDE.md (Phase 4):** the beam-coord convention `Clock` = HBLANK −68..−1 /
  visible 0..159, horizontal position (3px/cycle, coarse 15px, 160 wrap, leftmost X=3; offset is kernel-specific,
  final verdict via `read_tia.HmovedPixel`), the fully hardware-verified HMOVE table, and the annotated screenshot
  redefined as the primary user↔model channel.

## [0.4.0] - 2026-06-09
### Added
- **Litmus test fully passed (Phase 3) — the harness proven real, numerically (rule #4).** Coarse
  (`litmus_pos.asm`): 1 loop = 5 CPU cycles = 15px, linear over DELAY 3–11 (`ResetPixel = 15·DELAY − 18`),
  160 wrap, leftmost X=3. Fine (`litmus_hmove.asm`): all 16 HMP0 nibbles match the CLAUDE.md HMOVE table at 1px
  granularity. Coarse 15px + fine 1px = any X numerically predictable/placeable/verifiable. Detoxes Pong's
  failures #1/#3 (gap B).

## [0.3.0] - 2026-06-09
### Added
- **Harness plumbing verified (Phase 2.1)** — Gopher2600 embedded as a library, driven fully headless and
  numerically on a real ROM. `internal/emu` driver wrapper; `cmd/probe` numeric CLI; `roms/smoke.asm` confirms
  262 lines / RAM `$80`=$42 / PC.
- **Minimal MCP prototype (Phase 2.2)** — `cmd/harness` exposes 8 tools over stdio; JSON-RPC confirmed numerically.
  Official `modelcontextprotocol/go-sdk` v1.6.1, typed Out auto-generates JSON Schema. Spec in `docs/mcp-tools.md`.
### Decisions
- **Drive via direct `hardware.VCS` embedding, not terminal/PushedFunction** — `hardware`/`television`/`setup` are
  pure Go (no SDL/cgo), so library embedding is more deterministic/simple/fast. The terminal driving the research
  docs assumed was unnecessary.
- **★ Beam clock convention settled on hardware:** `GetCoords().Clock` = HBLANK −68..−1 / visible 0..159 (the
  spec's tentative "0–227" was wrong); same coordinate system as `HmovedPixel`.

## [0.2.0] - 2026-06-09
### Added
- **macOS / Apple Silicon environment set up.** Go 1.26.4, cc65/sim65, pkgconf, Gopher2600 built
  (`go build -tags=release .`), DASM / Stella / SDL2.

## [0.1.0] - 2026-06-09
### Added
- **Project founded.** Defined the goal as "an environment where the model can author the Atari 2600 in 6502
  assembly accurately." Initial `docs/gap-analysis.md` (gaps A–E from the past-Pong post-mortem),
  `docs/tool-landscape.md`, `docs/resources.md` (horizontal formula `X = 3N − 55`, HMOVE table, frame budget,
  collision registers), README, CHANGELOG.
### Decisions
- **Engine = Gopher2600** (the only high-accuracy 2600 emulator drivable at CPU + color-clock granularity on
  macOS), wrapped in a thin Go MCP. **BizHawk not adopted** (no macOS). Regression layer = sim65 / 6502profiler;
  oracle = Stella; top-priority gap = B (timing). MCP SDK = official `modelcontextprotocol/go-sdk`; design follows
  mcp-gameboy. Image overlay in-house Go (no ImageMagick shell-out). Regression around Gopher2600's record/replay
  + `regress`.
### Changed
- Renamed the directory from `Stella-MCP` to `atari2600-dev` (the engine isn't limited to Stella, and the
  deliverable is a whole environment, not a single MCP).
