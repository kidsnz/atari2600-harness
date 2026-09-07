# Technique #10 — Flicker multiplexing (N objects through 2 players)

**Goal:** show more than two player objects *on the same scanlines*. Vertical zones (#1) reuse
players down the screen, but the hard wall remains: 2 players per line. Flicker breaks it in
*time* instead of space: each frame draws a different subset, alternating fast enough (30 Hz per
object with 2 subsets) that persistence of vision merges them. This is how Pac-Man's four ghosts
share two player slots — the famous 2600 ghost flicker is this technique.

★**A longer cycle can flicker LESS, and the number this repository gates on cannot see it.** Andrew
Davie, 2003, proposing a three-frame Fuji over a two-frame one: *"**three frames**, first frame with
the outer bars, second with the left two, third with the right two. It gives you an **11-pixel
Fuji**… This one is **less 'flickery' in my opinion, because any of the bars is displayed two out of
every 3 frames**"* 〔`200309/msg00154`〕. The period went **up** and the flicker went **down**, because
what the eye tracks is the **duty ratio** — 2/3 rather than 1/2 — and the wider sprite came free.

★★Measured on two ROMs identical but for their duty (`internal/emu/flickerduty_test.go`):

| shown | max flicker area | mean | unchanged pairs |
|---|---|---|---|
| 1 frame of every 2 | 120 | 120.0 | 0 of 12 |
| 2 frames of every 3 | **120** | **80.0** | 4 of 12 |

★★★**The maxima are equal**, so a `max_flicker_area` ceiling ranks Davie's calmer design exactly level
with the one it replaces. The difference is entirely in *how often* the worst pair happens, and the
mean carries it — 80/120 is 2/3, the duty itself. This is not an argument for changing the gate: a
ceiling on the worst pair is the right shape for *"no single transition may be too violent"*. It is
an argument for knowing what the number cannot say — **a design that flickers less often, but just as
hard when it does, is invisible to it.** Choose the duty by eye, then let the gate hold the worst
case.

★**There is a second use, and this page had only the first: flicker makes COLOURS.** Manuel Polik,
watching Star Ship in 2002: *"I mean the crosshair. It's done with both missiles. But the two enemy
sprites have different colors. Now, **the flicker is used to give the crosshair a unique look!** The
missiles are just swapped constantly, so the **two colors \*melt\* into one**"* 〔`200201/msg00014`〕 —
and the same ROM uses flicker for **both** purposes at once, the ball multiplexing a starfield while
the missiles compose a colour.

★★**Measured** (`internal/ceiling/flickercolour_test.go`, engine NTSC palette, 128 codes):

| pairs alternated | distinct colours >16 RGB units from anything static |
|---|---|
| all 8128 pairs | 6225 |
| **the 960 SAME-LUMINANCE pairs** | **776** |

★★★**The same-luminance row is the usable one.** The TIA's luminance is D3..D1, so two codes sharing
it differ only in hue: the eye tracks a constant brightness and the hues melt instead of flickering.
That is the same axis `design.SameLuminance` names for multiplexing, used here for the opposite
purpose — there it decides which objects can share a slot without the swap being seen; here it decides
which pairs blend rather than blink. **So ~776 colours are reachable that no register can hold**, for
two `COLUPx` writes a frame.

★★★★**What is not measured: whether the eye agrees.** The midpoint is an arithmetic model of temporal
integration. What 30 Hz alternation looks like on a television is the frontier `known-traps.md` names
as this harness's harshest blind spot — the numbers above say which colours are *arithmetically* out
of reach, not which ones look right. Pick the pair here; judge it on a screen. Found by the
mailing-list distillation (helper-2).

Learned from (clean-room): `multisprite2/3.asm` discussions (8bitworkshop), AtariAge flicker
threads. Demo: `roms/techniques/flicker_multiplex.asm` — four bouncing color-coded balls, two
drawn per frame by frame parity — locked in CI by `scenarios/flicker_multiplex.json`.

## The technique
1. **Slots, not objects.** The kernel knows only two "slots" (P0, P1) with Y/X/color staged in
   VBLANK; it draws them with the any-Y compare kernel (#3 ×2 ≈ 49 cy/line — overlap-safe,
   no zone restrictions).
2. **Subset rotation.** Each frame, `frame & 1` picks objects {0,1} or {2,3} into the slots —
   positions, then colors, then one shared HMOVE (#4's staging discipline). Every object is
   visible 30 times a second.
3. **Per-object color rides the slot:** COLUP0/COLUP1 are re-staged with the subset, so four
   distinctly-colored objects coexist through two registers.

### The full form (documented, build when a game needs it)
Real engines improve on fixed pairs: **sort objects by Y each frame**, walk the screen assigning
the next-starting object to whichever player is free (re-positioning a player mid-screen after

> **★A full sort is not what the technique costs.** Roger Williams, stella-list 2002-04, describing
> what he named *FlickerSort*: it is a **single bubble pass per displayed frame**, not a sort —
> **O(n)**, and run **outside the kernel**. Over successive frames the list converges toward Y order
> and stays there while objects move slowly, which is all the technique needs. He also withdrew the
> obvious extension himself: doing it **inside** the kernel *"increases flicker and gains little."*
> The distinction is not cosmetic for anything using `prove_line_budget` — one pass is a fixed,
> provable cost; a sort is not.
>
> **This also pushes the source back five years.** This page cites AtariAge 107063 (2007) and bB; the
> 2002 thread is where the name was coined (Manuel Polik). Recorded because `pkg/design/multiplex.go`
> already carries a post-mortem on the opposite failure — *a citation that does not support the claim
> is worse than none*. Found by the mailing-list distillation (helper-1).
its previous object ends), and only flicker the objects that actually collide on the same lines —
with a rotation counter so no object starves. Fixed-parity pairs (this demo) are the verified
core; sort + dynamic 2-of-N allocation + fairness rotation is the documented extension
(`multisprite.inc` family).

## Verified here (Gopher2600, locked in CI)
- Four objects (two vertical bouncers at X=40/120, two horizontal at Y=60/120), all four
  trajectories deterministic; 262 lines every frame; budget clean.
- **The flicker itself is asserted:** three consecutive frames read
  P0/P1 = (53,97) → (40,120) → (55,95) — odd frames carry the moving horizontal pair, even
  frames the fixed-X vertical pair.

## Collisions across a flickered slot — and why the list said not to (added 2026-09-03)

Nothing above touches a collision register, and for twenty-eight years the standing advice was that
it could not:

> Obviously, you can't use the hardware collision registers … it'd just be a check to see if the
> "hot-spot" for the punching player's fist is within a rectangular area that the other player is in.
> — Erik Mooney, stella `199811/msg00037`

The reason is in this page's own subject. On any frame at most one of a flickered pair is drawn, so
a pair that never share a frame can never latch — the objects pass through each other. An author who
shipped it put it more plainly: *"you can swing the pod through all other objects (except the
playfield). I'm not sure, if i can fix this"* 〔`200007/msg00140`〕. The playfield is the exception
because the playfield is not flickered.

**It is fixable, and `litmus_flicker_attrib` measures the fix.** With `CXCLR` strobed every frame the
latch you read belongs to whatever was drawn in *that* frame, so a flickered slot can be given its
own attribution — and without the per-frame clear it cannot, which the same fixture shows by leaving
the strobe out and watching every frame read set. See `docs/techniques/flicker-collision-attribution.md`
for what it costs and the two idioms, and `internal/emu/flickerattrib_test.go` for the grading.

The 1998 advice was not wrong; it was practical. Software rectangles need no per-frame discipline and
survive an author who forgets one. The hardware route is cheaper and conditional, and the condition is
the thing to write down.
