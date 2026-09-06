# The visual ceiling — the denominator for a picture

`cmd/ceiling` · `internal/ceiling` · MCP tool `visual_ceiling`

## What it is for

`vismatch` compares a build against another ROM, so when a picture is wrong there is no way to separate
*"the kernel is wrong"* from *"the hardware cannot do this"* — the missing-denominator defect
`docs/capability-gap-audit.md` records over and over, in visual form. A **ceiling** supplies the missing
half: given a target picture, the best any 2600 kernel could reach for it under a **stated constraint set**.

Origin: the Dithertron reading of 2026-08-01 (`docs/8bitworkshop-crosscheck.md`, the two Dithertron
sections), which supplied the constraint-cell formulation — one whole scanline of 40 playfield columns,
two colours. Design settled by measurement 2026-08-03. No Dithertron source is used or vendored; the
algorithm here is a different one (an exhaustive per-line optimum, not its iterated
histogram-and-diffuse fixpoint).

## The one thing to get right

**A ceiling is a property of (image, constraint set), not of an image.** Scoring Chopper Command under a
playfield-only ceiling gives a number that says nothing about its kernel, because Chopper does not draw
with the playfield alone. A denominator that does not match the constraints the kernel works under is a
lie dressed as a percentage.

So the tool reports a **LADDER**, and **the differences between rungs are the deliverable**:

| rung | constraint set | what it isolates |
|---|---|---|
| **C1** | playfield only, 2 colours/line (COLUBK+COLUPF), 40 columns x 4 colour clocks | the asymmetric-PF kernel's own ceiling |
| **C2** | C1 + one 8-clock object supplying a third colour (per-clock control inside the window) | what one sprite buys |
| **C3** | 2 colours/line, **no column grid** — not 2600-achievable | what the 4-clock grid itself costs |

`C1 -> C2` answers *"what would one sprite buy on this picture"*. `C1 -> C3` answers *"how much is the
column grid costing here"*. Plus `flat` — one flat colour per line, the weakest picture the machine can
draw — to normalise against.

**Rejected: detecting the constraint set from the build.** `emu.DrawnObjects` could do it, but it makes
the author's own choices the denominator, so the score is high by construction and never says "you left a
resource unused".

## Measured

Ladder on five commercial frames (frame 60, NTSC, 160x214 = 34240 px, rmse in RGB units):

| frame | flat | C1 | C2 | C3 | C1->C2 (one sprite) | C1->C3 (the grid) |
|---|---:|---:|---:|---:|---:|---:|
| Chopper Command | 33.25 | 13.24 | 9.99 | 10.11 | 3.25 | 3.13 |
| Seaquest | 27.38 | 14.17 | 11.18 | 11.04 | 3.00 | 3.13 |
| Barnstorming | 29.82 | 13.63 | 10.85 | 6.53 | 2.77 | **7.09** |
| Pressure Cooker | 43.50 | 25.03 | 16.14 | 21.41 | **8.88** | 3.62 |
| Vanguard | 21.98 | 8.58 | 7.13 | 0.00 | 1.45 | **8.58** |

The ordering is the finding, not the absolute values: the grid costs most where the content is fine
sprite detail (Barnstorming 7.09, Vanguard 8.58) and least where it is landscape (Chopper 3.13). One
sprite is worth most on Pressure Cooker (8.88), whose picture is dominated by a few large objects.
Absolute values move with the frame you grade — the prototype measured Chopper 15.07 / Barnstorming 15.95
on different frames — so **compare rungs within a frame, never rungs across frames**.

`-out` renders a rung as a picture, which is how the ladder decision was actually made: on Chopper the C1
ceiling brings the landscape back nearly intact while the helicopter, the score digits and the ACTIVISION
logo collapse into 4-clock smears. The bound says, in a picture, "the playfield can do the scenery and
cannot do the actors".

## The palette, which is the whole thing

The self-test is: **a frame a real 2600 ROM produced is achievable by construction, so it must score
perfectly under a constraint set that describes it.** A playfield-only kernel must score exactly 0 under
C1.

The prototype first returned rmse **9.95** there, because it quantised Gopher2600 frames against
`internal/ingest/palette_stella.go` — Stella's measured palette. **7 of the frame's 14 colours were not in
that table at all**, off by up to 40 RGB units, the same order as the signal being measured.

So this implementation hardcodes nothing. `ceiling.PaletteFor(spec)` asks Gopher2600's own colour
generator (`specification.Spec.GetColor`) — literally the function `internal/emu`'s capture calls to paint
each pixel. `ceiling.HarvestPalette` is the empirical twin: it runs `roms/litmus/litmus_palette.bin` (4
white marker scanlines, then 128 lines of COLUBK = `$00,$02,..,$FE`) and reads the colours back off the
rendered frame. `TestHarvestedPaletteEqualsDerivedPalette` asserts the two agree on **all 128 entries
exactly**, so a renderer change that the derived table would silently follow into the wrong answer turns
the suite red instead.

`ceiling -palette-spec PAL` deliberately quantises an NTSC frame against the PAL palette, and prints a
warning saying the numbers are wrong on purpose. It is there so the failure mode can be seen rather than
described.

**And PAL is not NTSC with different timing — it is a smaller box of colours.** Measured 2026-09-06
(`internal/ceiling/palpalette_test.go`), with the NTSC table as the control in every comparison:

| | grey hues | reddest entry |
|---|---|---|
| NTSC | **1** of 16 (hue 0) | `$46` → RGB(236, 51, 51) — a red |
| PAL | **4** of 16 (hues 0, 1, 14, 15, all RGB(154,154,154)) | `$46` → RGB(215, 106, 38) — an **orange** |

And the count that falls out of it — **the table is 128 entries on every spec; what differs is how
many of them are the same colour twice**:

| spec | distinct colours from 128 entries |
|---|---|
| NTSC | 126 |
| PAL | **104** (12 hues × 8 + one grey column) |
| SECAM | **8** |

★The test asserts this as an **order**, not as three pinned numbers: the NTSC count came out 126 here
and 127 on another machine earlier in this project — a rounding difference in the renderer's
conversion, not a fact about the hardware — and pinning it turned CI red. `NTSC > PAL > SECAM` is the
claim that survives the arithmetic.

So a PAL kernel picks from **twelve** hues rather than fifteen, and **the same TIA code that paints a
red on NTSC paints an orange on PAL** — a picture whose subject IS red does not port, and no other
entry rescues it, because `$46` is already the reddest thing in the table. Both facts were reported on
the list in 1997 by someone who burned an EPROM to check them on a real machine rather than an
emulator: *"the first and last two colours are the same grey. What is surprising is that the TIA has
many nice colours but there isn't a bright, intense RED - at least in PAL"* 〔`199704/msg00150`,
1997-04-17〕. **There should be 16 hues on both systems, but there is not necessarily a corresponding
hue for each hue in the other system** — the same post, and the reason a spec conversion cannot be a
table lookup. Found by the mailing-list distillation (helper-2).

## Verification

Run `go test ./internal/ceiling/`. Denominators are stated in every test's log line.

| what | how it is checked | measured |
|---|---|---|
| self-test | 5 in-tree playfield-only ROMs (`litmus_pf_allcols`, `litmus_pf`, `litmus_pf_async`, `score2`, `litmus_title_then_play` — each qualifies because its `.asm` contains **zero** references to GRP/ENAM/ENABL/RESPx/RESMx/RESBL/COLUP0/COLUP1/NUSIZ) | C1 `sum_sq` exactly **0** on all 5 |
| both directions | sprite-drawn frames must score materially worse | `litmus_missile` C1 **23.06**, `litmus_collide_mp` **30.47**, `litmus_nusiz_all` **40.92**, against **0.00** for playfield-only |
| planted defect | quantise against a wrong palette | PAL-on-NTSC 0.17–0.35; NTSC+40 36.08–39.15; NTSC−40 9.43–17.80 (correct palette 0.0000 on all five) |
| palette provenance | derived table vs the rendered `litmus_palette.bin` sweep | 128/128 entries identical |
| render == report | each rendered rung re-scored against the target | exact match on 3 frames x 3 rungs |
| ladder nesting | flat ≥ C1 ≥ C2 and flat ≥ C1 ≥ C3 must hold | **113** litmus frames, no violation |
| commercial corpus | grid cost ordering | 5 frames; Barnstorming 7.09 > Chopper 3.13. Skips **with a stated count** when `reference/` is absent, and **fails** if the tree exists but ROMs have gone missing |

`litmus_pf_allcols` qualifies as the primary self-test subject because it is the fixture that exercises
the whole playfield map — 20 bands, one of the 20 columns lit per band — so a frame that scores 0 under C1
has had every column position of the grid checked, not just an easy one.

## What this does NOT do — read before quoting a number

1. **It is not a score for a kernel.** It never looks at a ROM, only at a picture. It cannot tell you
   whether your kernel is good; it tells you how much of the remaining error is the hardware's.
2. **No rung is validated by emitting a cartridge.** This is the honest gap. The transferable rule from
   2026-08-03 is *"a rung that cannot produce a cartridge inside 76 cycles is not a ceiling"*, and this
   implementation does not enforce it. What exists is one demonstration, done by hand in the prototype:
   a generator emitted a cartridge from a Chopper Command C1 ceiling, `prove_line_budget` certified it at
   **66 cycles** against 76, and it reproduced the ceiling image with **0 of 29440 pixels differing** —
   after a `sta WSYNC` before the epilogue, because the region from the kernel's last WSYNC to the first
   overscan WSYNC carries the epilogue and the proof charges it to the last visible scanline (the first
   emission proved at 83, and a hand-written kernel had hit the same trap at 91). **C2 has no such
   evidence at all**, and **C3 is unreachable by design** — it is a diagnostic reference, not a ceiling.
   Until a rung emits a cartridge, "the machine could reach this" is arithmetic for C1 and a claim for C2.
3. **C1 measures the picture, not the mechanism.** A single 8-pixel player sitting on a 4-clock boundary
   is expressible as playfield, so C1 can be 0.00 for a frame drawn with a sprite (measured:
   `litmus_pos.bin`, flat 52.53, C1 0.00). That is the bound being correct, not the bound being fooled.
4. **C2's object window is aligned to the column grid** (39 positions) rather than free to start at any of
   the 153 colour clocks. A restricted search can only make the computed error larger, so this can only
   understate the machine, never overstate it. The size of the restriction is measured, not assumed —
   `TestColumnAlignedObjectWindowCostsThisMuchVersusAFreeStart`, holding the playfield pair fixed:
   0.00 rmse (`litmus_missile`, `litmus_collide_mp`), 0.41 (`litmus_sprite`), 0.80 (`litmus_objsizes`),
   1.09 (`litmus_hmove_side`), 1.36 (`litmus_nusiz_all`).
5. **C2 models ONE object.** A real kernel has two players, two missiles and a ball, and can reposition
   and reuse them mid-line. C2 is deliberately the smallest step above C1, so a large `C1->C2` means
   "objects are where the picture is", not "one sprite is enough".
6. **The error metric is squared RGB distance**, which is crude perceptually. It only has to be consistent
   between the bound and the build, not psychophysically right.
7. **No temporal modelling.** One frame at a time; flicker and multiplexing (which buy real colour
   resolution on hardware) are invisible to it, so a flickered picture's true ceiling is below what C1
   reports.

## Exactness, and what was traded for speed

C1 and C3 are **exhaustive** over all 8256 colour-pair cases per line (8128 distinct pairs + the 128
single-colour cases), so each is a true optimum rather than a heuristic that could understate the machine.
C2 is exact too: its branch-and-bound uses a bound that is valid rather than heuristic — an 8-clock object
can only reduce the error inside the columns it covers, and at best to zero — and in practice the worst
line needed **2 of 8256** pairs evaluated.

The shipped C1 is also **tighter than the prototype's**, at the same cost. The prototype collapsed each
column to its mean colour and chose both the pair and the per-column pick by the error of those means;
this one minimises the actual pixel error directly (`cellCost[c][v]` is the true per-column price). The
gap is small on TIA output, because a 4-clock column is usually one flat colour and then the two
formulations coincide — measured over 5 commercial frames the gap in raw squared error was 0, 0, 0, **560**
(Pressure Cooker) and 0. Small, but in the direction of blaming the hardware less.

Cost: a 160x214 frame with all three rungs takes **~15-25 ms** (identical scanlines are solved once; a
commercial frame has 40-60 distinct lines out of 214).

## Usage

```sh
# ladder for a ROM's frame
go run ./cmd/ceiling -target roms/litmus/litmus_pf_allcols.bin

# a PNG, two rungs, and render the C1 ceiling to look at
go run ./cmd/ceiling -target shot.png -rungs C1,C3 -out ceiling_c1.png -rung-out C1

# cross-check the palette by MEASURING it instead of deriving it
go run ./cmd/ceiling -target frame.bin -harvest-palette

# show what a wrong palette does (prints a warning; the numbers are wrong on purpose)
go run ./cmd/ceiling -target frame.bin -palette-spec PAL
```

MCP: `visual_ceiling` grades the **current emulator frame** (`load_rom` + `step_frame` first) or a `.png`.
A `.bin` is refused there, because loading it would disturb the machine you are working on. It does not
advance the emulator. It refuses a framebuffer that has never been drawn to — pure `(0,0,0)` is a value
the renderer never writes, since its own blank is `(6,6,6)` = colour code `$00`, and grading it returns
rmse 6.00 on every rung, which looks exactly like an answer.
