# Technique #10 — Flicker multiplexing (N objects through 2 players)

**Goal:** show more than two player objects *on the same scanlines*. Vertical zones (#1) reuse
players down the screen, but the hard wall remains: 2 players per line. Flicker breaks it in
*time* instead of space: each frame draws a different subset, alternating fast enough (30 Hz per
object with 2 subsets) that persistence of vision merges them. This is how Pac-Man's four ghosts
share two player slots — the famous 2600 ghost flicker is this technique.

★**How much flicker, not just whether: `design.FlickerRateHz`.** `NeedsFlicker` answers yes or no and
returns the same answer for three objects and for twenty. The ladder is the frame rate over the number
of subsets, and the archive names both of its ends. Glenn Saunders, 1997: *"Some of the most impressive
2600 games have flicker (Solaris, Radar Lock, Stargate, Star Wars: The Arcade Game, even Adventure). It
frees up the 2600 to do more independently moving sprites… **It's never really necessary to drop below
30hz**"* 〔`199709/msg00139`〕. Piero Cavina, five days later and not as a compliment: *"**'Adventure'
must be the king of flicker**… you could put all the objects (dot included) in the same room and get an
incredible amount of flicker"* 〔`199709/msg00218`〕.

`design.SubsetsFor` gives the first column's answer and `design.FlickerRateHz` the last:

| objects sharing the two slots | subsets | each drawn at |
|---|---|---|
| 1–2 | 1 | 60.05 Hz — no flicker |
| 3–4 | 2 | **30.03 Hz** — the rate Saunders calls sufficient |
| 12 | 6 | 10.01 Hz |
| 24 | 12 | **5.00 Hz** — Cavina's *"5hZ, maybe?"* |

★★**The last row is arithmetic meeting an eyewitness.** Twenty-four objects sharing two slots comes to
exactly the number he guessed at, and neither side was derived from the other. ★★★There is no hardware
limit to return here — this is a judgement — but the number exists so the judgement is made against
one, and `HardwareCollisionUsable` is the other half of it: past two subsets the TIA's collision
latches stop being trustworthy, so a high N costs more than looks.

★**Does a longer cycle flicker less? The two sources disagree, and the number this repository gates on
cannot see the difference either way.** Andrew
Davie, 2003, proposing a three-frame Fuji over a two-frame one: *"**three frames**, first frame with
the outer bars, second with the left two, third with the right two. It gives you an **11-pixel
Fuji**… This one is **less 'flickery' in my opinion, because any of the bars is displayed two out of
every 3 frames**"* 〔`200309/msg00154`〕. His reading: the period went up and the flicker went down
because the eye tracks the **duty ratio** — 2/3 rather than 1/2. **Tested on real CRTs in 2021, the
ranking ran the other way**: several people found 1/2 the calmest and 3/4 the worst, and the reason
offered was the flicker frequency (30, 20 and 15 Hz), not the duty (AtariAge `topic/315322`; one
participant added that the preferred rate differs from person to person). The wider sprite came free
either way.

★★Measured on two ROMs identical but for their duty (`internal/emu/flickerduty_test.go`):

| shown | max flicker area | mean | unchanged pairs |
|---|---|---|---|
| 1 frame of every 2 | 120 | 120.0 | 0 of 12 |
| 2 frames of every 3 | **120** | **80.0** | 4 of 12 |

★★★**The maxima are equal**, so a `max_flicker_area` ceiling ranks Davie's three-frame design exactly level
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

★**A third use: flicker that mixes depth order.** Thomas Jentzsch, 2022, to the author of *Raptor*:
*"you are using the same PF priority flicker trick for the shield which I came up with for the clouds
in Aardvark. And in your game it is a key element."* (AtariAge `topic/332187`). The post names the
trick and nothing more. Our reading is that `CTRLPF` D2 (playfield priority, `pf-modes.md`) is
toggled frame by frame, so an object is in front of the cloud or shield on one frame and behind it on
the next. **Cited only, not verified** — neither ROM has been run here, and the D2 reading is ours.

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
its previous object ends), and only flicker the objects that actually collide on the same lines —
with a rotation counter so no object starves. Fixed-parity pairs (this demo) are the verified
core; sort + dynamic 2-of-N allocation + fairness rotation is the documented extension
(`multisprite.inc` family).

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

**Which form to build: supercat's ladder** (AtariAge `topic/78021`, 2005). The axis underneath is
memory against sorting.
- **Multiplex one player, keep the other fixed, and never let the multiplexed objects cross.** Most
  2600 titles do this, he says: the kernel stays simple, and the records keep a fixed order because
  moving on screen never reorders them. His advice to a newcomer is to start here.
- **Let them cross** and it gets much harder: keep the records fixed and copy them into a sorted
  structure before each frame, or re-sort them in memory as they move. This page's full form and
  `dynamic-multisprite.md` are this rung.
- **A row array** (the asker's design): for each sector of the screen (24 lines, say), a table of what
  GRP0 and GRP1 show. Plain code, and it extends to 30 Hz flicker easily; the price is RAM — 27 bytes
  plus 1 per sprite in the asker's version — and a simple version loses objects past five on one row.
  A bug reported in that version: on the frame where a sector drops back to two objects and flicker
  turns off, GRP0 and GRP1 can both draw the same sprite (seen as one extra frame of flicker).
- **Y-sorted alternation**, when every sprite is (or can be treated as) the same height: list the
  frame's sprites in Y order and hand them out P0, P1, P0, …; equal heights end in the order they
  start, so strict alternation holds. He calls adding flicker to it awkward.

He names *Dig Dug* as doing this with extra RAM on the cartridge. **Cited only, not verified** — read
through our distillation note; the thread itself is not on disk here.

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

## Choosing what flickers, how often, and how evenly (added 2026-09-30)

Everything above decides *how many* subsets. Six threads decide the rest, and none of them was
measured; each is **Cited only, not verified**, and none of the ROMs named has been run here.

**How often is set by the motion, not only by the count.** `SubsetsFor` takes the number of objects
on the same lines at one moment; how long they stay there decides how often the flicker happens at
all. RevEng, on drawing every orbiting electron of an atom: *"With the orbiting motion, the time each
electron shares scanlines with the others is fairly limited. It probably wouldn't be too bad with an
intelligent flicker routine."* Karl G's answer was to ask for uranium's 92 (AtariAge `topic/302929`,
2020).

**But flicker that comes and goes has its own cost.** The full form above flickers only the objects
that collide on the same lines, which makes the flicker intermittent. Thomas Jentzsch: *"Personally I
find on and off flicker more noticeable and annoying than high frequency, constant flicker"*
(AtariAge `topic/243516`, 2015). That is a third axis beside the duty and the frequency above, and
the gate reads it backwards: an on-and-off pattern has the same worst pair as a constant one and
more unchanged pairs, so `max_flicker_area` ties them and the mean prefers the one he finds worse.
That reading follows from the duty table's definitions; **Not verified** by a ROM.

**The duty need not be the same for every object.** Kirk Israel, 2004, planning a pterodactyl
("Pterry") between the two JoustPong players: alternate `[1 2]`, `[1 P]`, `[1 2]`, `[P 2]`, so
*"each player is shown 3 out of 4 frames, and Pterry is shown every other frame"*, against the even
split where Pterry is solid and each player shows half the time; he worried only about missing hits
〔`200402/msg00068`〕. The favoured objects get the 3-of-4 pattern — off one frame in four, the 15 Hz
gap that the 2021 CRT comparison above ranked worst. Glenn Saunders answered by not flickering at all:
draw Pterry with one missile 〔`200402/msg00069`〕, which Thomas Jentzsch said needs only relative
repositioning — `HMMx` and `HMOVE`, no timed `RESMx` 〔`200402/msg00073`〕.

**Which object flickers is a choice, and the threads choose by different rules.**
- *Never the hero.* johnnywc on a *Bruce Lee* mock-up: *"My recommendation would be to have Bruce
  never flicker and have the enemies flicker at 30hz, or you could flicker all 3 at 40hz. Of course
  they would only flicker when all 3 are on the same line"* (AtariAge `topic/347106`, 2023-01-27).
  His 40 Hz counts frames shown, two in three; the gap still recurs at 20 Hz. splendidnut's prototype
  two weeks later puts the two enemies on one player object, and they flicker.
- *By what lies underneath.* Thomas Jentzsch on *Pac-Line*: *"maybe it is better to flicker ghosts
  and player. Because the player will never move over white playfield pellets. These make flicker
  very obvious. For ghosts this is fine, but not for fruits."* The author found his own eyes were on
  the ghost more than on Pac-Man, and hardware collision settled it: flicker Pac-Man against the
  ghost and nobody dies, against the bonus and nothing is collected, so only ghost and bonus could
  share (AtariAge `topic/364115`). That is the collision section above deciding a design.
- *Inside something that already blinks.* In the same thread CapitanClassic suggested flickering the
  power pellet, since the arcade one is lit *"approximately every 2 out of 3 frames"*. Two costs came
  back: the pellet was eaten by hardware collision, so it could not be eaten while not drawn; and,
  Thomas Jentzsch, it is drawn with the ball and the playfield, so it *"cannot be used for drawing
  complex sprites"*.

**Brighten what flickers.** SpiceWare, who flickers the player's character "when needed" in Space Rocks,
Draconian, Frantic and Timmy: *"One thing that does help is to LumaBoost flickering objects - basically increase the color
values by 2 for any object that is flickering. Thomas suggested that on Dec 1, 2012 during the
development of Stay Frosty 2"* (AtariAge `topic/243516`). With luminance in D3..D1, +2 is one
luminance step, and at the top step it carries into the hue nibble. How much of the dimming it repays
is not measured.

**The other end is none.** Karl G, 2021: *"I wanted to see if I could make a 4-player maze game with
no sprite flicker and distinct player/object/maze colors and fit it into 2K, and this is the result"*
(QuadTari, `chaser.bin`, AtariAge `topic/317525`). The thread does not say how the four are drawn.
