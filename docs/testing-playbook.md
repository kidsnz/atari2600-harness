# Testing playbook — how to *know* an authored ROM is correct

The harness already enforces "judge by numbers, not by eye" (CLAUDE.md iron rule 1). This doc imports the
broader, well-established software-testing discipline so the authoring loop verifies at the **level of the
claim**, not just the level of the parts. It is the verification half of `docs/authoring-protocol.md`
(step 5 verify + step 6 feedback).

## The core problem: the test oracle
The hardest question in testing is **"what is the right answer, and how do you know?"** — the *oracle
problem* (Barr et al., "The Oracle Problem in Software Testing: A Survey", IEEE TSE 2015). For a 2600 kernel
there is rarely a ready oracle ("is this the correct frame?"). Almost every technique below is a different
*answer* to the oracle problem. Pick the cheapest oracle that covers the claim you are making.

**Lesson that motivated this doc:** component checks passing (per-cell collision works, tunnel pass-through
works) does *not* prove the emergent claim (the ball clears the back row from behind). Verify at the level of
what you assert. When you claim a high-level behaviour, demonstrate *that behaviour* — don't infer it from
parts. (memory: `feedback-verification-standard`.)

## Technique → harness mapping
| Technique (source) | Oracle strategy | In this harness |
|---|---|---|
| **Invariants / contracts** (Meyer, *Eiffel*; assertions) | "always true" | scenario `invariants` (every frame) + `assert_line_budget` + `ntsc_frame_lines` |
| **Runtime verification / bounded temporal logic** (Bauer/Leucker/Schallhart, LTL₃, TOSEM 2011; STL, RV 2015) | properties over a *sequence* of frames, with a deadline | scenario `temporal` (VV-5): `eventually`-within-K (bounded liveness), `response` (A⇒P within K), `never_for` N consecutive; liveness with an unobserved window reports **INCONCLUSIVE**, never a vacuous green |
| **Property-based testing** (Claessen & Hughes, *QuickCheck*, ICFP 2000) | properties over many inputs | assert a *property* not a value — `monotonic` (score ↑, lives ↓), range asserts |
| **Metamorphic testing** (Chen et al. 1998; Segura et al., *A Survey on Metamorphic Testing*, IEEE TSE 2016) | a *relation* between two runs (no oracle) | scenario `metamorphic`: base + input transform + relation on metrics |
| **Differential / golden master** (Feathers, *WELC*; characterization tests) | a trusted reference | `golden_frame`/`golden_audio` hashes (vs the ROM's *own* past); **`cmd/refdiff`** = a layout fingerprint (wall positions, ball size) diffed vs the **original** ROM (the oracle); **`cmd/trajdiff`** (VV-8) = time-extended behavioral diff (lockstep RAM trajectory, first-divergence frame+field) |
| **Fuzzing** (Zalewski, *AFL*) | "no invariant breaks under any input" | scenario `fuzz`: seeded random inputs, invariants monitored every frame |
| **Coverage / test adequacy + coverage-guided fuzzing** (Zalewski, *AFL*; Go native fuzzing) | "did the run actually reach the code; grow inputs toward new edges" | VV-3: `internal/emu.Coverage` (pc + branch edges, one-sided branches) via `cmd/cover`; `internal/guidedfuzz`+`cmd/guidedfuzz` keep a corpus and mutate toward new markers (vs blind `fuzz`) |
| **Deterministic simulation testing** (FoundationDB → Antithesis; Wilson, Strange Loop 2014) | seeded run + end-of-run guarantees, reproducible | the emulator is already deterministic → seeded `fuzz` + failure **replay** (seed+frame) |
| **Mutation testing** (DeMillo/Lipton/Sayward 1978; Offutt) | grade the *tests*, not the code | `mutation`: inject a fault, confirm the suite catches it (kill) or warn (survivor) |
| **Invariant mining** (Ernst et al., *Daikon*, SCP 2007) | learn likely invariants from runs | `mine-invariants`: observe fields → emit candidate `invariants`/`monotonic`/range as a spec draft |
| **Delta debugging / shrinking** (Zeller & Hildebrandt, IEEE TSE 2002) | minimize a failing input | reduce a failing input timeline to the smallest frames that still fail |

## Per-build verification checklist (run every kernel/feature)
Steps 1–3 are the existing baseline; 4–6 raise the rigour. All of it is runnable today via `run_scenario` +
the MCP tools; the automated `fuzz`/`mutation`/`metamorphic`/`mine-invariants` make 4–5 one command.

1. **Golden + frame budget** — a scenario with `ntsc_frame_lines: 262`, `max_line_budget: 76`, and
   `golden_frame: true`. Guards rendering, timing, and rolls in one shot.
2. **Instantaneous asserts** — known RAM/TIA values at specific frames (`asserts` with `at_frame`). The boot
   state, a serve, a hit.
3. **Differential vs the real ROM** (when reproducing one) — `read_row` byte-match at sampled scanlines; the
   real ROM is the oracle. (`docs/build-to-learn.md`.)
4. **Invariants + properties** — declare what must *always* hold (`invariants`) and what may only move one way
   (`monotonic`): lives non-increasing, score non-decreasing, frame lines in range, the ball never resolves
   on top of a lit brick cell. These catch classes of bugs, not single states.
5. **Stress the space** — `fuzz` (seeded random input, hundreds of frames) to find rolls/crashes/invariant
   breaks no scripted timeline would; `mutation` to confirm the checks above would actually *catch* a
   regression (a passing suite against a broken ROM means the checks are too weak); `metamorphic` for claims
   with no oracle (e.g. "carving more of a column never increases the bricks left").
6. **Claim-level demonstration** — for any *emergent* claim ("the tunnel-behind technique works", "this is
   playable"), demonstrate *that behaviour* directly: free-run from a realistic state with no `poke`
   intervention and observe the claimed phenomenon in the numbers + annotated frame. Do not infer it from
   component checks. (memory: `feedback-verification-standard`.)

## Habits from the 2600 archive
The checklist above runs in the emulator, outside the ROM. The mailing list and the forums kept habits of their
own, from before any of these tools. Each is cited here, not re-run.

- **Put a number on the screen from inside the ROM.** Erik Mooney, 1997, with his vertical-blank logic on
  `TIM64T`, stored `INTIM` into player 0's score before spinning out the timer, so *"player 0's score would show
  the number of 64-cycle intervals I had left for game logic"*, and *"I found that to be a very useful debugging
  device in general - once the score-display routine was done, I could write any value to the score and have it
  displayed"* 〔stella-list `199704/msg00191`〕. He was answering Piero Cavina, whose way was a useless
  `LDX #timetowaste` / `DEX` / `BPL` loop raised to *"the maximum value that does not cause the screen to jump"*
  〔stella-list `199704/msg00189`〕. MLdB, 2013, kept the minimum rather than one frame: his player-2 ammo display
  showed *"the lowest recorded value of the INTIM register right before entering the VBlank-wait routine. It
  should probably go no lower than $0A, which means I have about 10\*64=640 cycles left if in VBlank I'm not
  mistaken"* (AtariAge `topic/209800`). Paul Slocum, 2002, testing on a Cuttle Cart, wrote a variable straight
  into `PF2` under the score; it showed that paddle 1 *"is actually being read correctly and shows the exact same
  values as when Paddle 3 is set, but the marble just doesn't respond"*, so the fault lay after the read — he
  *"sort of figured out"* that it was his min/max speed checking, fixed by initialising the speed, though he was *"still not exactly sure why only paddle
  1 and 2 were affected"* 〔stella-list `200206/msg00005`, `200206/msg00006`〕. SpiceWare's version, `INTIM` at the
  end of vertical blank and of overscan shown in the score, is in `docs/known-traps.md` (the DPC+ ARM row).
  `read_ram` and `read_ram_trace` read such values from the emulator, outside the ROM; a readout built into the ROM is
  still there on a console, where they are not — our reading. Cited only, not verified.
- **Measure the headroom, not only the fit.** Paul Slocum asked the list how to avoid running out of
  vertical-blank and overscan time, and proposed: *"for testing, I'll throw a couple of extra WSYNCs into VBlank
  and Overscan and try to optimize the code so it still works. Then in the final release, I'll remove the WSYNCs
  and know that I have a little extra headroom in case a state comes up where extra cycles are used"*
  〔stella-list `200206/msg00198`〕. That post has no reply in the archive, but his later guide section
  "INSURANCE AGAINST TOO MANY VBLANK/OVERSCAN CYCLES" recommends the same, and adds *"This is also an easy way
  to estimate how much time you have left in VBlank and Overscan: keep adding WSYNCs (each line is 76 cycles)
  until the screen jumps"* 〔stella-list `200404/msg00246`〕. MLdB's minimum above is a measured form of the same
  question. The checklist above says whether a frame fits, not by how much — our reading. Cited only, not
  verified.
- **A count test with two outcomes.** Erik Mooney's test — pad a line to exactly 76 cycles without `WSYNC`, and a
  screen that gets more corrupt going down means the count is wrong — is in the `docs/known-traps.md` row
  "dropping `WSYNC` from a line timed to exactly 76 cycles buys three cycles, and puts every path's count into
  the picture". Its other half: *"If not, then it's correct and there's another problem."*
  〔stella-list `200109/msg00329`〕 Cited only, not verified.
- **When two emulators disagree, count your own frame first.** In a 2005 thread a playfield exercise flickered
  in Stella and looked fine in z26; the cause was neither emulator but the author's `ldx 192` for `ldx #192`,
  found when he counted 343 lines at 45 FPS (AtariAge `topic/63521`, held here only as distilled notes; the
  whole story is the `docs/known-traps.md` row "When it works here and not on the machine"). That is one case.
  This harness counts the frame in every scenario that sets `ntsc_frame_lines` (step 1); what the case adds is
  the order — our reading. Cited only, not verified.
- **A patch that cancels the symptom.** crackers, 1997, had a frame-counting clock losing a second every minute.
  Jim Nitchals asked whether the display had too many lines — if the frame does not add up to 262, *"your clock
  routine will be off"* — and said
  the routine *"sure looks like it'd count by 60's exactly"* 〔stella-list `199710/msg00055`〕. The author, whose
  vertical blank and overscan ran on `TIM64T` values he called *"the closest number"*, added a `DEC SECS` each
  minute instead; it seemed to match his watch, and when he came back later the program had crashed
  〔stella-list `199710/msg00060`〕. Later that day he reported a mistake in the patch's placement (after
  `STA MINS`, where it confused the `BEQ`) — *"Should have worked but it didn't"*; that this was what crashed it
  is our reading. With that fixed the clock gained a second every hour, so he added an
  `INC SECS` each hour — *"It might gain or drop a second every 60 hours now"*, good enough for his virtual pet
  〔stella-list `199710/msg00067`〕. He chose not to find the cause (*"Rather than mucking around trying to figure
  out what exactly was causing the clock to lose a second every minute"*), and whether his frame was 262 lines is
  not in the thread. Each patch needed another at the next scale, and the cause stayed where it was — our
  reading: measure the frame (step 1) before compensating for it. Cited only, not verified.
- **Isolation by transplant.** A 2003 kernel crashed whenever an index went above 50. The replies came as:
  Manuel Polik, *"Post the source."*; Thomas Jentzsch, a hypothesis — *"Are you using a branch directly after
  loading the data? Maybe you are relying on some flags here"*; Dennis Debro, who had not looked at the code, a
  suggestion to run a z26 trace; then Jentzsch again,
  *"I have put your code into my own framework and it works perfect. It \*must\* be something else"*, with his
  framework attached 〔stella-list `200301/msg00116`, `200301/msg00117`, `200301/msg00135`, `200301/msg00142`〕.
  The thread never found the cause: the trace log the poster made, once Eckhard Stolberg had explained z26's
  `-t` switch, *"revealed nothing about the problem"*, Erik Eid's hand cycle count found nothing that
  *"screams out"*, and the poster reported *"I moved some things around, and saved some time with some
  optimisations, and for the most part it's working again"* — *"I don't know how I did it though"*
  〔stella-list `200301/msg00150`, `200301/msg00155`, `200301/msg00153`, `200301/msg00154`〕. The transplant
  pointed away from the posted code, as the hand count also did; the shrinking row above narrows a failing
  input, not the code around it — our reading. Cited only, not verified.
- **The test art decides what a kernel test can see.** Kirk Israel, 2004: *"The kernal I ended up with is
  tighter than I even realized...when I substituted in Pterry for a player graphic, pixels were getting dropped
  (ones that the player graphics don't usually use)"* 〔stella-list `200402/msg00161`〕. Art that leaves a pixel
  unlit cannot show a kernel failing to draw it; test with art that lights every pixel the kernel is meant to
  draw. The harness's own playfield litmus had the same gap: `litmus_pf` covers columns 0/4/12 only, and
  `litmus_pf_allcols` all 20 (`TestEveryPlayfieldColumnLandsWhereTheTableSays`) — both the rule and the
  comparison are our reading. Cited only, not verified.

## Provenance
Imported, established techniques (not 2600-specific). Recorded per `feedback-provenance-always`:
- Oracle problem — Barr, Harman, McMinn, Shahbaz, Yoo, *The Oracle Problem in Software Testing: A Survey*,
  IEEE TSE 41(5), 2015.
- Property-based testing — Claessen & Hughes, *QuickCheck: A Lightweight Tool for Random Testing of Haskell
  Programs*, ICFP 2000.
- Metamorphic testing — Chen, Cheung, Yiu 1998; Segura, Fraser, Sanchez, Ruiz-Cortés, *A Survey on
  Metamorphic Testing*, IEEE TSE 42(9), 2016.
- Golden master / characterization — Feathers, *Working Effectively with Legacy Code*, 2004.
- Fuzzing — Zalewski, *American Fuzzy Lop (AFL)*.
- Deterministic simulation testing — FoundationDB; Will Wilson, *Testing Distributed Systems with
  Deterministic Simulation*, Strange Loop 2014; Antithesis (antithesis.com) DST/PBT resources.
- Mutation testing — DeMillo, Lipton, Sayward 1978; Offutt et al.
- Invariant mining — Ernst, Perkins, Guo, McCamant, Pacheco, Tschantz, Xiao, *The Daikon system for dynamic
  detection of likely invariants*, Science of Computer Programming 69, 2007.
- Delta debugging — Zeller & Hildebrandt, *Simplifying and Isolating Failure-Inducing Input*, IEEE TSE 28(2),
  2002.

## Worked example — the suite's first catch (Breakout, 2026-06-16)
On its first real run, the Breakout `fuzz` scenario (`ntsc_frame_lines: 262` + game-logic invariants over
600 frames of random paddle) reported **264 lines, not 262** — exposing that the "stable 262" verdict carried
through the whole Breakout build was an *eyeball guess, never measured* (it had silently drifted: rung8=262,
asymmetric PF +1 → 263, channel kernel +1 → 264). Fixed to a true 262 (overscan tuned 30→28) and locked by
the scenario. This is the playbook's whole point: a numeric, claim-level check finds what the eye certifies
as fine. Then `mutate` showed the fuzz scenario alone is a weak oracle (5% byte-flip kill rate); adding a
`golden_frame` scenario raised it to 20%.

**Honest kill rate (VV-11).** Those naive percentages are deflated by dead-code dilution: a 4K ROM is mostly
unexecuted padding, so most random byte-flips land where no scenario could ever catch them and survive by
construction. `cmd/mutate -covered` (and `mutate.EvalRandomCovered`) restricts fault injection to offsets a
baseline run actually executes (PC coverage), measuring the suite against *live* code only. On `smoke.bin` the
same suite scores **2% naive vs 68% covered** — the suite was never that weak; the low number was the metric
lying. Pair it with `cmd/statecov` (the TIA state-coverage matrix) to see whether the test even exercised the
modes worth mutating.

## Status / automation — all delivered (G10–G14)
- Scenario `invariants` / `monotonic` / range, `fuzz`, `metrics` → `internal/scenario`, run by `cmd/scenario`
  and the `run_scenario` MCP tool. (`docs/scenarios.md`.)
- `cmd/mutate` (mutation testing), `cmd/metamorphic` (oracle-free relations), `cmd/mine-invariants`
  (Daikon-lite spec drafts) — CLIs over scenarios, runnable in CI / by hand.
- `cmd/refdiff` — differential check vs the **original** ROM: extracts a layout fingerprint (left/right wall
  clock, ball width & height) and diffs it. Catches "wrong vs the original" (a wall inset from the edge, an
  undersized ball) that golden self-regression cannot. Second worked example: a user spotted my Breakout's
  left-wall gap + 1×1 ball by *playing*; refdiff went RED on both, drove the fix to MATCH (wall 0, ball 2×4).
