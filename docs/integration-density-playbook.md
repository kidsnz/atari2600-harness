# Integration & Density Playbook

*How to compose maximum functionality inside the Atari 2600's interlocking budgets
(~2 KB ROM · 128 B RAM · 76 CPU cycles/scanline · fixed 262-line NTSC frame), where
ROM bytes, RAM bytes, per-line cycles, and total scanlines all trade against one another.*

This is the **composition / integration** skill — spending a fixed, four-way-interlocked
budget so every byte and every cycle buys a *viable* option. It is a distinct capability
from "own more tools": more harness tools raise *verification coverage*; density is what
turns a working ROM into a **dense** one. This doc is the design-time reference for that skill.

> **Provenance.** Distilled (2026-07-24) from a broad cross-domain research pass —
> demoscene/size-coding, WCET/embedded real-time, deliberate-practice science, software
> product-line engineering, and systemic game design — then **adversarially filtered against
> the 2600's real budget** (ideas that silently assume abundant resources are killed in §C).
> Key sources are listed in §F. Grounded against our own Combat clean-room build and the
> `casebook.md` disassembly mining.

---

## A. The eight transferable principles (adopted · adapted · rated)

Rating = transfer value to a real 2 KB / 128 B / 76 cy build. ★★★★★ = load-bearing.

> **The two that carry the rest.** Of the eight, the completed research elevates **one master move** —
> *store a generator + a seed/table, not the data* (#3) — and **one discipline** — *prove the worst case
> statically, don't sample it* (#1). Everything else is how you make those two pay under byte/cycle scarcity.

### 1. Prove the worst case — don't sample it. ★★★★★
*(WCET / abstract interpretation → `prove_line_budget`)*
A test run only characterises the inputs you happened to exercise; it can **never** guarantee
the true worst-case path was hit. Safe timing/space bounds come from static analysis over
**all** paths. On the 6507 the hardware model is trivial (no cache/pipeline), so the whole
problem is **control-flow bounding** — *almost*. See the kill in §C: 6502 timing is **not**
purely control-flow (page-crossing and branch-taken add data-dependent cycles), and a kernel
needs **exact equal-cycle paths**, not merely `≤ 76`. Adapt: prove every line statically;
page-align data tables and balance branch arms so every path costs the *same*.
*Source: Wilhelm et al., WCET survey (ACM TECS 2008); AbsInt abstract-interpretation papers.*

### 2. One resource, many duties. ★★★★★
*(register/byte/table multi-use — the core density move)*
Give one scarce resource several simultaneous jobs. A 17-byte NTSC kernel (vs 60–80 B naïve)
uses **one loop counter** as both the exact 256-line count *and* the memory-init index, and
holds VSYNC bits, a shift counter, and colour in the **A** register at once. For RAM: if two
variables are provably **never live at the same time**, overlay them on the same byte
(the stack/RAM-minimisation result from real-time systems). Density metric: *RAM-byte duty* (§D).
*Source: 8bitworkshop "Tiny VCS kernels" (2025); real-time stack-minimisation (AbsInt/RTAS).*

### 3. Generate from a seed — but only with a CHEAP generator. ★★★★ (conditional)
*(procedural-from-seed, adversarially bounded)*
Trade storage for a *tiny* amount of compute. Pitfall! synthesises all 255 screens from a
polynomial-counter **seed** (an 8-bit LFSR seeded at **0xC4**, ~50 bytes of generator, a few

**Why the hardware counts this way at all** (added 2026-09-04): a polynomial counter was cheaper
silicon. The designers: *"A polynomial counter occupies one-fourth the silicon area of an equivalent
binary counter, but, unlike a binary counter, it does not count in any simple order."* 〔Perry & Wallich, "Design case history: the Atari Video Computer System", IEEE Spectrum 1983-03 pp.45-51〕
Every LFSR in this repository — the audio dividers, Pitfall's world, `eor #$B4` — is that one
economy, and "does not count in any simple order" is the bill, paid by the programmer ever since.
cycles *per screen* — not per pixel). Entombed builds its whole maze from a
**32-byte table + a small algorithm + symmetry** (20-bit half-row → fixed 4-bit wall → 16 →
bit-duplication → only **8 bits** actually selected). **KILL (§C):** the demoscene "everything
is a function" reflex (64 kB runtime mesh synthesis, `bytebeat` audio = *f(t)*) assumes runtime
compute the 2600 does not have. The rule: **the generator must fit the per-line/per-frame cycle
slack, or be precomputed offline.** LFSR/tiny-table = yes; heavy synthesis = no.
*Source: David Crane on Pitfall! (Hackaday 2013); Aycock & Copplestone, Entombed (arXiv 1811.02035).*

### 4. Runtime parametric variability from a compact config table. ★★★★
*(software product lines — CORRECTED for 8-bit)*
Ship many curated variants from one artifact. **Not** the compile-time `#ifdef` model
(that yields *N separate binaries* and each compositional "feature module" costs hook-method
bytes). The 2600 form is a **third mechanism**: one ROM, config bits (console switches / a RAM
byte) **index a small parameter table** at runtime → many variants from shared code
(Combat: **27 variants from a ~28-byte table**). Treat each config bit as a *modeled feature with constraints* so only
valid/curated combinations are reachable. Density metric: *feature-count-per-K* (§D).
*Source: Kang et al. FODA; Kästner & Apel (annotative vs compositional); Combat 27-variant table.*

### 5. Orthogonal composition beats accretion. ★★★
*(systemic design / effective complexity — guides WHAT to build)*
State-space grows **multiplicatively** with interactions, additively with parts. Depth comes
from giving each element/mechanic more *connections and roles*, and from differences of **type,
not degree** (orthogonal differentiation) — one radically-distinct ability is enough. Elegance =
depth ÷ complexity: cut comprehension/tracking cost, keep the depth. The design-side statement of
"reuse one routine for many effects": spend a byte only where it opens a *new viable option*.
*Source: "Complexity vs Depth" (Game Developer); Orthogonal Differentiation (gdp3, Chalmers).*

### 6. Byte-level idioms are the composition primitives. ★★★★★
*(the "shortest correct sequence / shared subexpression" analog, in raw 6502)*
Concrete, directly adoptable: **ASL-to-zero** clean start (5 B vs 11 B CLEAN_START — 8 shifts
drive any power-on value to 0); **BIT-opcode skip-next** to delete a branch (−1 B/use, no
register clobber); **shared tables** — overlap sound envelopes across effects varying only TIA
distortion (24 B recovered in *Dominant Amber*), share glyph bytes (0/6/8/9); **init↔frame-loop
fusion** (17 B vs 60–80 B). These are the atoms every other principle is built from.
*Source: "Dominant Amber" 1KB byte-saving log (Hackaday.io); 8bitworkshop tiny kernels; sizecoding.org.*

### 7. Tight, VALID numeric feedback is the master training lever. ★★★★★
*(deliberate practice / feedback loops — HOW to get better)*
The single highest-leverage lever on skill growth is minimising feedback latency: faster correct
feedback = more experiments per unit time. **Two guards** the literature is emphatic about:
(a) *signal validity is a prerequisite* — rapid iteration optimises hard toward whatever the
signal rewards, so a biased/proxy signal makes you worse fast; (b) *immediate feedback on every
attempt can degrade retention/transfer* (the guidance hypothesis) — so **fade** the feedback as
competence grows. Here the **harness is the instrument** (`prove_line_budget`, `spritepos`,
`read_row`): it must measure the user-observable truth, not a proxy.
*Source: siboehm "tight feedback loops"; Ericsson & Harwell (Frontiers 2019); motor-learning guidance hypothesis.*

### 8. Isolate sub-skills, then compose. ★★★★
*(chunking / progressive overload)*
Decompose the integrative skill into **per-axis drills** (cycles/line · ROM bytes · RAM bytes ·
scanline count), train one with full focus + immediate feedback + repeated revised attempts, then
train the **composition itself** on a representative task (integrative skills resist clean
isolation, so you must also practice the interlock). Progressive overload = tighten one axis at a
time once the current target is met.
*Source: Ericsson strict DP definition (Frontiers 2019); competitive-programming training practice.*

### Highest-leverage adoptions (ranked)
1. **Prove every line statically, exact-cycle** (#1) — the correctness floor.
2. **RAM-byte overlay + register multi-use** (#2) — the biggest raw-space win.
3. **Byte-idiom vocabulary** (#6) — the primitives to reach for reflexively.
4. **Cheap seed/table generation** (#3) — storage→compute, *bounded*.
5. **Runtime config-table variants** (#4) — functionality multiplier per byte.
6. **Harness-as-valid-feedback + fade** (#7) — the training instrument done right.
7. **Per-axis drills → compose** (#8) — the practice structure.
8. **Orthogonal, option-buying spend** (#5) — the design filter over all of the above.

---

## C. Adversarial kills (ideas that assume abundant resources — rejected)

| Idea | Why it dies on the 2600 |
|---|---|
| **`bytebeat` / PCM audio = f(t)** | No DAC/sample path (TIA = 2 register/LFSR channels); ≈149 cy/sample vs **76 cy/line**. |
| **Heavy runtime procedural synthesis** (64 kB-intro mesh gen) | No spare per-line compute; the storage→compute trade needs cycles the beam race has already spent. Use offline precompute or a cheap LFSR/table only. |
| **Generic compressors/packers** at sub-2 KB | Decompressor overhead can exceed the savings; hand-rolled packing or shared tables win. |
| **Compile-time `#ifdef` "feature modules"** | Yields N binaries; compositional hooks cost bytes. Use **runtime table indexing** (§4) instead. |
| **"`≤ 76` cycles is fine"** | A kernel needs **exact equal-path** timing; a stray +1 (page-cross/branch) desyncs the beam. |
| **GC / virtual memory / heaps / "just add a library"** | Irrelevant at 128 B RAM / 2 KB ROM. |

---

## D. Density Scorecard (measure a ROM against a reference)

Each metric is measurable *with the harness*, and compared to a reference ROM (e.g. Combat @ 2 KB).

| Metric | Definition | How to measure | Target |
|---|---|---|---|
| **Functionality-per-byte** | shipped features (variants · mechanics · screens) ÷ ROM bytes used | count features / `size` | ≥ reference (Combat 27 / ~28 B; Pitfall 255 screens / ~50 B gen) |
| **WCET slack per line** | `76 − proven_WCET(line)`, reported as the **minimum across all line-types** | `prove_line_budget` (all paths) — **proven, never `profile_line_budget` sampled** | small **positive & uniform** (tight ≠ loose ≠ negative) |
| **RAM-byte duty** | avg distinct live-purposes per RAM byte over a frame (overlays where non-overlap is proven) | `probe_ram_semantics` / `read_ram_trace` + liveness | > 1 |
| **Feature-count-per-K** | curated variants derivable ÷ config bytes | count / table size | ≥ reference (Combat 27 / ~28 B) |
| **Kernel byte-density** | visible pixel-rows produced ÷ kernel bytes | `read_row` × kernel size | ≥ reference |
| **Generation ratio** | bytes of rendered content (screens · mazes · objects) ÷ bytes of stored generator + seed | count outputs × output-size / gen bytes | maximise (Pitfall: 255 screens / ~50 B) |
| **Data-share ratio** | table bytes consumed by **≥ 2** users ÷ total table bytes | trace table readers | maximise (shared envelope/glyph tables) |
| **Dead-weight** | bytes/cycles that buy **no** viable option (complexity without depth) | design audit | → 0 |
| **RAM footprint** | RAM bytes a program precisely writes or reads — **a lower bound, not a price** | `cyclebound.RAMFootprintOf` (added 2026-09-06) | as low as the design allows |
| **Flicker area** | pixels whose **drawing object** (BG/PF/P0/P1/M0/M1/BL) differs between adjacent frames | `max_flicker_area` in a scenario, or `emu.FlickerArea` (added 2026-09-06) | author-set, once, having looked |
| **Frame-parity duty** | how many *meanings* the one frame-parity bit is asked to carry | **not computable** — the bit's value says nothing; only the author knows what it means | exactly 1 |
| **Sustained-viewing cost** | whether the picture is still comfortable after minutes | **no instrument exists here and none can** — the longest scenario in this repository runs 1200 frames (20 s) | — |

> **Anti-gaming caveat (open question).** The axes interlock — you can *trade* one for another (spend cycles to
> save bytes, overlay RAM at the cost of a branch). So the scorecard is a **vector, not a single score**:
> progress = moving one axis toward target **without regressing** the others. Collapsing them into one index
> that can't be gamed by a resource trade is unsolved (§G).

**Four rows added 2026-09-06, and two of them are honest about having no instrument.** The
distillation of the stella-list archive produced a classification of the resources a 2600 design
spends, and it does not fit the shape the rest of this table assumes. Some resources a tool can
prove over all paths (cycles, via `prove_line_budget`); some it can only check against a number the
author declares (RAM, via `ram_budget`); some are statically countable but nothing counted them
(ROM bytes); **one has no observable at all** — the frame-parity bit's *value* says nothing, because
the resource is which MEANING the author assigned to it, and that is a declaration, not a state;
**and one cannot be measured even by asking a person.** The last is the sharpest thing the archive
gave us. Manuel Polik put the flicker budget of Gunfight 2600 to a public vote in 2001, was
*"talked everybody into giving me 9 bullets"*, built it, *"watch[ed] it for two minutes"*, got a
headache, and shipped **six** 〔stella-list `200103/msg00099`〕 — killing the three-way shot in the
process. Thomas Jentzsch, independently: static coarseness stops bothering you the longer you play,
while *"any flicker … gives you some headache to soon"* 〔`200102/msg00271`〕. **A green 20-second
scenario is not "comfortable after two minutes", and no arrangement of this repository's tools makes
it one.** That row exists so nobody looks for the check.

The flicker-area row is the counter-example that makes the other two bearable: the archive judges
flicker by area — *"an area as large as an Arkanoid wall is going to be hard on the eyes even at
30 Hz flicker"* 〔`200108/msg00315`〕 — and area IS measurable. The threshold still is not, so the
check asks the author for a ceiling once and keeps it thereafter. Found by the mailing-list
distillation (helper-3); the two instruments built and calibrated here.

★**And flicker is judged on a different axis from resolution, which the same archive says outright.**
Glenn Saunders, explaining why a 2600 credit-roll had to hit twelve characters a line WITHOUT flicker
because he was aiming it at cable television: *"Because I don't think the cable networks will tolerate
flicker.  **Low res CG is one thing, but flicker is another.**"* 〔`199708/msg00139`〕. **Coarse gets
through; flickering does not.** A picture is not graded on one quality scale with flicker as its low
end — the two are separate judgements, and the judge in that case was a broadcaster rather than a
player. So `max_flicker_area` is not a proxy for "how good does it look"; it is its own gate, and a
kernel may be as blocky as it likes on the other side of it.

★★The quote was recovered by accident and the accident is worth recording: it was first transcribed
as *"will tole[rate]"*, cut mid-word with an editorial bracket, and **the bracket hid the sentence that
follows** — the one that carries the whole finding. Nothing goes inside a quotation that the author did
not write (`docs/provenance.md`); this is what it costs when something does.

★★★A second observer in the same thread read the flicker's SHAPE off the screen rather than its area:
*"It was pretty obvious from the way it flickered, though, that you were drawing every other character
every other frame"* 〔`199708/msg00129`, Lee Seitz〕. `FlickerArea` returns how many pixels changed, not
how they are arranged — and "every other character" and "one contiguous block" of the same area do not
look alike. **That is a gap in the instrument, stated by someone who separated the two by eye in 1997.**

The scorecard is a *criterion-referenced* instrument (per the deliberate-practice measurement
literature), not a vanity number: each row is an objective, reproducible target, and progress =
moving a chosen row toward its target **without regressing** the others (the interlock).

---

## E. Deliberate-practice loop (progressively harder 2 KB builds)

Per build:
1. **Pick a representative task at the edge of ability** with one objective criterion (a target
   scorecard number for this rung).
2. **Design first, then predict the numbers** (WCET/line, ROM bytes, RAM bytes) *before* coding.
3. **Author asm; measure immediately** with the harness; **compare to the prediction** — the gap
   is the lesson (this is the tight-feedback loop; keep the signal valid, §7).
4. **Log each miss** with a one-line "why"; review before the next rung.
5. **Progressive overload:** tighten exactly **one axis** at a time.
6. **Fade the feedback** as competence grows — stop leaning on the prover for numbers you can now
   predict (guidance hypothesis, §7).

### The ladder (each rung gated by a scorecard target)
1. **Stable minimal kernel** — 262 lines proven, every line exact-cycle. Gate: `prove_line_budget` clean.
2. **One moving object** — coarse position + HMOVE, still exact-cycle. Gate: `spritepos` exact, no line over budget.
3. **RAM diet** — re-implement rung 2 with ≥1 proven RAM-byte overlay. Gate: RAM-byte duty > 1.
4. **Byte diet** — shave the ROM via §6 idioms with zero feature loss. Gate: functionality-per-byte ↑.
5. **Add a variant free** — one runtime config bit → a second curated mode, **same** byte budget. Gate: feature-count-per-K ↑.
6. **Compose under interlock** — add a mechanic that forces a real four-way trade; keep all scorecard rows ≥ target. Gate: no row regresses.

---

## F. Provenance (key sources)

- **Size-coding / demoscene:** sizecoding.org; Ctrl-Alt-Test "Procedural 3D mesh generation in a 64kB intro" (2023); "Dominant Amber" 1KB byte-saving log (Hackaday.io); Mathieu Acher 256-byte build.
- **Atari 2600 practitioner:** Nick Bensema "Guide to Cycle Counting on the Atari 2600"; 8bitworkshop "Tiny VCS kernels" (2025); Bumbershoot "48px kernel"; David Crane on Pitfall! (Hackaday 2013); Aycock & Copplestone, *Entombed* (arXiv 1811.02035).
- **WCET / embedded real-time:** Wilhelm et al. "The WCET Problem — Overview & Survey of Tools" (ACM TECS 2008); AbsInt aiT / abstract-interpretation (arXiv 0710.4753); real-time stack-minimisation.
- **Deliberate practice:** Ericsson, Krampe & Tesch-Römer (1993); Ericsson & Harwell (Frontiers 2019); siboehm "tight feedback loops"; guidance-hypothesis motor-learning literature.
- **Software product lines:** Kang et al. FODA; Apel/Batory/Kästner/Saake FOSPL; Kästner & Apel annotative-vs-compositional; Combat 27-variant table.
- **Systemic game design:** "Complexity vs Depth" (Game Developer); Orthogonal Differentiation (gdp3, Chalmers); effective-complexity writeups.

*Adversarial verification note.* The full research pass later completed and 3-vote refutation-tested
25 claims: **20 confirmed, 0 refuted, 5 could not be verified** (verifier infra errored — *not*
refutations; see §G). Confirmed transfers: seed/table generation, symmetry/bit-duplication,
parameterized shared substrate, WCET-over-all-paths (safe **and** tight), Ericsson-strict practice.
**Bounded / killed:** `bytebeat` and heavy runtime synthesis (no 2600 compute path — §C); "6502 WCET
is purely control-flow" (page-crossing/branch data-dependence, §1); "Combat variants = annotative
`#ifdef`" (it is runtime table indexing — §4).

---

## G. Open agenda (verify-on-harness + unsolved questions)

**Verify on OUR harness before trusting** — these were near-certain textbook facts the research pass
could not independently re-verify (its verifier agents hit an infra limit, an *error* not a refutation).
Re-verifying them is the first practice task (§E rung 1). Status:

- ✓ **VERIFIED (2026-07-24, rung 1).** The cycle↔pixel coupling **3 color-clocks per CPU cycle** (the real
  fact behind `X = (CYCLES − 20) × 3`). `trace_clocks` on a 2/3/4/5/6-cycle instruction mix: color-clock
  advance = **exactly 3 × cycles** every time (6/9/12/15/18), no exceptions. Positioning reality
  cross-checked: `spritepos` x=80 → achieved_x=80 exact (div-15 loop = 5 cy = 15 clocks = the RESP granularity).
  *Note:* a `sta WSYNC` stall is attributed to the **following** instruction (Gopher2600 observation
  granularity) — a live reminder that a kernel needs exact-cycle timing, not just `≤ 76` (§1).
- ✓ **VERIFIED (2026-07-24, rung 1).** **BIT-absolute skip-next.** Raw `$2C` = 3 bytes / **4 cycles**;
  falling into it absorbs the next 2-byte instruction (`$F00D` → next PC `$F010`, skipping `lda #$22`) and
  leaves **A/X/Y intact** (A=$11, X=$BB, Y=$CC), moving only N/V/Z. Register-safety confirmed.
- ☐ **Still to verify** (lower priority, deferred): **BRK-as-1-byte-call** (vs 3-byte `JSR`) and the
  **shared envelope/glyph table** 24-byte saving (*Dominant Amber*) — reproduce with `assemble_and_load`
  + byte count when a build actually reaches for them.

**Unsolved questions (the research agenda this playbook opens):**
1. **Compute↔storage crossover.** At what *generation cost per row/screen* does procedural-from-seed stop
   paying, given 76 cy per visible line **plus** the larger off-screen VBLANK/overscan budget? (Measure it.)
2. **Single density index without gaming.** How to weight/normalise the scorecard's axes into one comparable
   number that cannot be gamed by trading one interlocking resource for another (cycles↔bytes↔RAM)? (§D caveat.)
3. **"Dense" baseline.** What is the *measured* functionality-per-byte of the references (Combat 27 / ~28 B;
   Pitfall 255 / ~50 B) vs a modern hand-built ROM — i.e. what ratio counts as dense, making the scorecard
   absolute rather than merely relative?
