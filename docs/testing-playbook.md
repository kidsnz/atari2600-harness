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
| **Property-based testing** (Claessen & Hughes, *QuickCheck*, ICFP 2000) | properties over many inputs | assert a *property* not a value — `monotonic` (score ↑, lives ↓), range asserts |
| **Metamorphic testing** (Chen et al. 1998; Segura et al., *A Survey on Metamorphic Testing*, IEEE TSE 2016) | a *relation* between two runs (no oracle) | scenario `metamorphic`: base + input transform + relation on metrics |
| **Differential / golden master** (Feathers, *WELC*; characterization tests) | a trusted reference | `golden_frame`/`golden_audio` hashes (vs the ROM's *own* past); **`cmd/refdiff`** = a layout fingerprint (wall positions, ball size) diffed vs the **original** ROM (the oracle) |
| **Fuzzing** (Zalewski, *AFL*) | "no invariant breaks under any input" | scenario `fuzz`: seeded random inputs, invariants monitored every frame |
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

## Status / automation — all delivered (G10–G14)
- Scenario `invariants` / `monotonic` / range, `fuzz`, `metrics` → `internal/scenario`, run by `cmd/scenario`
  and the `run_scenario` MCP tool. (`docs/scenarios.md`.)
- `cmd/mutate` (mutation testing), `cmd/metamorphic` (oracle-free relations), `cmd/mine-invariants`
  (Daikon-lite spec drafts) — CLIs over scenarios, runnable in CI / by hand.
- `cmd/refdiff` — differential check vs the **original** ROM: extracts a layout fingerprint (left/right wall
  clock, ball width & height) and diffs it. Catches "wrong vs the original" (a wall inset from the edge, an
  undersized ball) that golden self-regression cannot. Second worked example: a user spotted my Breakout's
  left-wall gap + 1×1 ball by *playing*; refdiff went RED on both, drove the fix to MATCH (wall 0, ball 2×4).
