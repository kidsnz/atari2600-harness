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
| 1–2 | 1 | 59.92 Hz — no flicker |
| 3–4 | 2 | **29.96 Hz** — the rate Saunders calls sufficient |
| 12 | 6 | 9.99 Hz |
| 24 | 12 | **4.99 Hz** — Cavina's *"5hZ, maybe?"* |

★★**The last row is arithmetic meeting an eyewitness.** Twenty-four objects sharing two slots comes to
the number he guessed at, and neither side was derived from the other. ★★★There is no hardware
limit to return here — this is a judgement — but the number exists so the judgement is made against
one, and `HardwareCollisionUsable` is the other half of it: past two subsets the TIA's collision
latches stop being trustworthy, so a high N costs more than looks.

**The table is NTSC only, and so is `FlickerRateHz`.** A PAL 2600 runs at 49.86 Hz (3546894/228/312;
`subpixel-velocity.md`, where the PAL colour clock itself is **Cited only, not verified**), so two
subsets are drawn at 24.93 Hz, not 29.96. Eckhard Stolberg, on his own TV, about a demo of two large
sprites interlaced over time: *"The 60Hz NTSC flicker is a borderline decission at best, but the 50Hz
PAL flicker is unbearable. It is too noticable with such large objects."* 〔`200302/msg00246`〕 Two
variables, then, the rate and the size of what flickers; `design-principles.md`'s *"Never over a large
area"* is the size half. **Cited only, not verified** — one viewer, one demo.

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

One answer from the forums on how far the eye forgives, which is a claim about perception and not a
measurement: RevEng, advising on a sprite flickered over a coloured background, *"don't get too caught
up on the absolute purity of the color of the guy's shirt and pants. When you flicker with a colored
background, color constancy kicks in and people perceive the colors as relatively pure even though they
aren't in the absolute sense."* (AtariAge `topic/172888`). **Cited only, not verified** — read through
our distillation note; the thread itself is not on disk here.

**One case where the colour use took the frames from the object use.** Andrew Davie, 2003, posting a
ChronoColour frame of a creature drawn with the large-sprite system of *Fu Kung!*: *"Only one creatre
possible, though - as both alternate frames are used to generate the chronocolour. There are, however,
enough cycles/line to change BOTH P0 and P1 colour. So that's another way to change the colour."*
〔`200301/msg00161`〕 Our reading: the alternation that could have carried a second creature is spent on
colour, so that object cannot also be one of a flickered set. Star Ship above does both at once, but
on different objects. Later that day he posted a last version: *"this is probably the last we'll see of
this monster. Consider this the termination of an exploratory branch of the capabilities of the
system."* 〔`200301/msg00164`〕 **Cited only, not verified.**

**The colour use, in the playfield.** Andrew Davie, 2002, to Billy Eno, whose 16x16 field of tiles
needed four states for each tile: *"Have you considered the "4-colour playfield" system I posted so
long ago? Basically this alternates playfield pattern and colour to achieve an effective 3-colours +
BG. You could use this effectively to get the display you want.... at the expense of 30Hz flicker, but
you'd get solid squares."* 〔stella-list `200208/msg00050`〕 "Alternates" is all the post says of the
mechanism; that it alternates frame by frame is our reading of the 30 Hz. Eno *"found only one bin in
the archive, and no source (the link to it is dead.)"* and put his understanding as a question — *"My
impression is that you create the 4 colors using the two alternating playfield colors, the color the
playfields create when overlapped, and the background?"* 〔`200208/msg00075`〕 — which no post in the
thread answers. The demo's reception in 1998 is in `design-principles.md` and
`integration-density-playbook.md`. No playfield doing this has been built here. **Cited only, not
verified.**

**The colour use, for a whole picture.** In February 2003 Andrew Davie posted cover art converted to a
*"colour bitmap for display"* — *"Takes me about 1 minute to convert an image for display"* — and
Thomas Jentzsch answered with a demo of his own (*"let's see how it looks compared to my demo"*). Rob,
replying to that post, wrote that *"this looks better in Stella"*, not yet having tried it on the
Cuttle. Davie wrote *"it looks like Thomas has killed my new technique within a day of its creation"*
and converted the clown picture of Jentzsch's demo as well, with a caveat: the original graphics were
*"in a strange RGB line format - so this isn't strictly a like to like"*. Jentzsch credited the idea to
ZylonBane on AtariAge. Both build the picture from three colour channels with one colour code chosen
per channel: Jentzsch's *"($42,$c6,$86)"*, Davie's *"$34, $D6, $70"*. Rob, quoting that caveat, found
Davie's clown *"more colorful somehow, albeit more noticeably flickery"*; Jentzsch's reply was that
this *"may come from the different RGB color values Andrew and I ($42,$c6,$86) choose"*, and that
differing emulator palettes may be a reason too. When Davie posted versions made from the original
images, Jentzsch wrote *"I'll try to make such a demo myself too"*, adding that *"the channel splitting
of the Clown seems to be a little incorrect right now"*; he later added that picture quality could
improve *"by using different ways to generate the three color channels"*. Rob set the originals apart
from versions *"munged into rolling RGB"*. 〔stella-list `200302/msg00069`, `200302/msg00071`,
`200302/msg00073`, `200302/msg00074`, `200302/msg00075`, `200302/msg00078`, `200302/msg00079`,
`200302/msg00080`, `200302/msg00081`, `200302/msg00085`, `200302/msg00087`〕 The words "RGB line format"
and "rolling RGB" appear, but none of these posts explains how the channels are laid out over frames
and lines. For the image in the next thread he started, "The Demo Image Series #0", Davie explained the
layout on 12 February: *"each individual scanline alternatively displays RGB, GBR, BRG... and the lines
are out-of-synch with each other"*, so that *"while one line is displaying the red values for that
line, the next line is displaying it's green values, and the one after that displaying blue values"*
〔stella-list `200302/msg00106`〕. None of these demos has been run here. **Cited only, not verified.**

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
> Manuel Polik's *FlickerSort* 〔`200204/msg00010`〕: it is a **single bubble pass per displayed frame**, not a sort —
> **O(n)**, and run **outside the kernel**. Over successive frames the list converges toward Y order
> and stays there while objects move slowly, which is all the technique needs. He also withdrew the
> obvious extension himself, doing it **inside** the kernel: *"My idea would have made the sprites
> flicker a lot more, to not much overall benefit"* 〔`200204/msg00011`〕.
> The distinction is not cosmetic for anything using `prove_line_budget` — one pass is a fixed,
> provable cost; a sort is not.
>
> **This also pushes the source back five years.** This page cites AtariAge 107063 (2007) and bB; the
> 2002 thread is where the name was coined (Manuel Polik). Recorded because `pkg/design/multiplex.go`
> already carries a post-mortem on the opposite failure — *a citation that does not support the claim
> is worse than none*. Found by the mailing-list distillation (helper-1).
>
> **What the withdrawn version would have cost, from the proposal itself** (Roger Williams,
> 2002-04-16, 〔`200204/msg00008`〕, with *"about half the code half-written"*). A state machine spreads
> the move to the next sprite over several lines, which *"will make each multiplex sprite several lines
> higher than it 'really is' for flicker purposes"* — his estimate, *"about four extra (2-scanline) lines,
> making the typical square player about double-height for flicker purposes."* The list is sorted
> indirectly, so *"2 of those bytes are X and Y positions which can double as game position data, since
> objects don't have to move around in the list."* RAM: about 10 bytes for the sorting code, about 18 for
> the non-multiplexed objects, and 5 per multiplexed copy of P0, leaving *"at least 2000 cycles free
> between frames"*. Polik's FlickerSort demo answered it, and the withdrawal quoted above came the
> next day. Later that same day he argued part of it back: a sort run outside the kernel can count a sprite as drawn when its RESPx/HMPx setup
> carried the kernel past it, and the sprite *"disappears completely"*, whereas in the kernel *"you know
> for sure whether a sprite was actually displayed"* 〔`200204/msg00022`〕. **Cited only, not verified** —
> none of these programs has been run here.

**The rotation can come out of the sort.** Thomas Jentzsch, 2002, on the "intelligent flicker" in
JtzBall 〔`200203/msg00079`〕: his bubble sort *"stops when all elements are sorted"*, which comes early
because each element moves one row a frame, and *"keeps the order of rows which have the same Y-value.
That is very important for intelligent flicker."* Duplicates on a row go to the end of the array and
out of the displayed count: *"By removing those duplicates with the lowest indexes, I'm getting
automatically the intelligent flicker I need."* His worked example, three atoms on one row moving the
same way, shows each displayed once in three frames. Our reading: nothing in it counts turns — the
stable sort does the job the rotation counter does above. He had not yet checked it *"for elements
which are moving in opposite Y-directions"*. **Cited only, not verified.**

**Or keep the order instead of restoring it.** Erik Mooney, 2002, in a thread on which sort to use:
*"Couldn't you just insert the object at the beginning of the list if it's entering at the top of the
screen, and insert it at the end of the list if it's entering at the bottom of the screen?"* — and, for
the general case, an insert routine that walks the list: *"That can't take more than n time. That almost
changes the paradigm from a sorted array to a priority queue, which when you think about it is what your
2600 display kernel really is..."* 〔`200203/msg00102`〕. Adam Wozniak, on finding the place: *"log2(n)
if you binary search"* 〔`200203/msg00109`〕. `dynamic-multisprite.md` goes the other way for its five
objects (a fixed sorting network, not insertion sort); this is the case of objects entering at the
screen's edges. **Cited only, not verified.**

**Heights feed the overlap test.** Bob Colbert, 1997, on his multi-sprite engine: *"I just eliminated a
nasty bug that sometimes caused a sprite to mysteriously disappear. It turns out that when I modified my
code to allow for variable height sprites, I neglected to modify a portion of the conflict detection
routine."* 〔`199710/msg00111`〕 Our reading: in the full form above that is the routine deciding which
objects "actually collide on the same lines" (this page's words), so whatever changes how heights are stored has to reach it too.
**Cited only, not verified.**

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

**Assuming the worst case saves ROM.** Asked why Atari's *Pac-Man* flickers its ghosts
constantly even when none of them share a scanline, pacmanplus answered: *"It takes less ROM to always
assume the worst-case scenario that all objects are on the same line, than to try and compute where each
object is and see which objects you need to flicker"*. SpiceWare gave the other side's bill: flicker
management, *"the logic used to reduce flicker by reusing object(s) multiple times as the screen is
drawn"*, *"takes up space in ROM, plus require RAM to keep track of things"*. His extreme example is
Draconian: the routines that reposition player0 while also drawing all five objects use 483 bytes
(*"though they're about to be rewritten"*), and adding the routines for the other four objects brings
it to 2462 — extreme, he says, because the ARM's speed lets it reuse all five (AtariAge `topic/264347`,
2017). Our reading: this page's fixed-parity demo is the worst-case
assumption and the full form above is the management. **Cited only, not verified** — no ROM size was
measured here.

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

**Two more conditions, from 2003: how small and fast the objects are, and how the subsets are split.**
Thomas Jentzsch suggested a two-player, two-missile kernel with constant 30 Hz flicker for four players
and four missiles 〔`200310/msg00051`〕; Kirk Israel asked whether that would miss hits — the
pass-through described above 〔`200310/msg00052`〕 — and Jentzsch answered: *"That depends on the
size and speed of the objects. The smaller and faster the objects are, the more problems you will have
with hardware collision detection."* Bounding rectangles, the other option, can also miss some
collisions when a missile *"moves faster than the size of the bounding rectangles"*
〔`200310/msg00053`〕. Kirk added that the split decides which pairs can meet at all: *"if you always
displayed player and bullets 1 + 2 on one frame then 3+4 on the alternate, 1's bullets could never hit 3
I think, and vice versa."* 〔`200310/msg00054`〕 `HardwareCollisionUsable` takes only whether an object
is flickered; neither the split nor size and speed is an input to it. **Cited only, not verified.**

## Choosing what flickers, how often, and how evenly (added 2026-09-30)

Everything above decides *how many* subsets. The threads below decide the rest, and none of them was
measured; each is **Cited only, not verified**, and none of the ROMs named has been run here.

**How often is set by the motion, not only by the count.** `SubsetsFor` takes the number of objects
on the same lines at one moment; how long they stay there decides how often the flicker happens at
all. RevEng, on drawing every orbiting electron of an atom: *"With the orbiting motion, the time each
electron shares scanlines with the others is fairly limited. It probably wouldn't be too bad with an
intelligent flicker routine."* Karl G's answer was to ask for uranium's 92 (AtariAge `topic/302929`,
2020).

**Small objects that rarely meet are a different claim from many objects.** Glenn Saunders, on Manuel
Polik's demo of bullets driven by Bob Colbert's multiplexer: *"Vertical separation is pretty easy on
missiles because they are so small. Even with six they rarely overlap, at least in that demo where they
are going in a diagonal trajectory."* 〔`200103/msg00208`〕 Manuel answered with a challenge: *"Name just
one game, that's having eight totally independent moving, nearly non flickering bullets/stars/whatever"*,
and *"The Stars in Stargate, Solaris, Starmaster & Star Voyager are definitely not capable to
overlap..."* 〔`200103/msg00211`〕. Our reading: before a commercial ROM is cited as many objects with
little flicker, check whether its objects can share a line at all.

**Starfields, as one programmer read them in 2002.** Manuel Polik, after *"two hours analyzing how
other Starfields are made"* 〔`200207/msg00331`〕: SW-TAG re-uses *"two particles à four times -> 15Hz
flicker"*; Gyruss draws the field *"using only 1 particle most of the time and should flicker extrem. I
wonder why it doesn't seem to be too annoying..."*; Star Voyager, *"Just like Starmaster"*, uses the
ball, *"wasting a complete scannline to reposition it"*; Star Raiders uses all three particles and
splits the screen in two with a mid-screen repositioning, so it *"can display six particles at once,
which is resulting in 12 stars with only 30Hz flicker"*. The Gyruss question is left open in the post.
**Cited only, not verified** — his reading of those ROMs, not ours.

**Why his own repositioned starfield flickered.** Earlier that day Polik had put a starfield back into
Star Fire's kernel and judged it *"not acceptable at all. The flicker is maddening, probably worse than
SW-TAG."* His reason: *"Repositioning an object takes a whole scannline. This \_can't\_ take place in a
line where I'm repositioning or drawing a sprite."* So the stars could go only *"in the vertical gaps
between the sprites"*, and *"A huge sprite can occupy enough vertical space to blank out 5 stars at
once"*. He would not continue with that approach, though he thought it *"a little better on the
TV"*, and floated nine stars from both missiles and the
ball cycled at 20Hz — *"but that'll flicker again, of course. A little less probably"* 〔`200207/msg00325`〕.
The field he went on to describe places each line's star 8 pixels along from the last instead
(`procedural.md`, `200207/msg00367`). **Cited only, not verified.**

**A rate one poster named, and what the copies cost a missile.** ZackAttack, 2015: *"I've been
trying to"* draw a 160x192 single-colour image with 30 Hz flicker — in Stella only, *"waiting on some
parts to arrive before I can test this on real hardware"*. He had all but 4 columns at 30 Hz, could
fill those only with the ball, at 15 Hz, and asked whether that could be overlooked. Omegamatrix: *"I
would keep trying to get 30Hz flicker."* Andromeda Stardust: *"IMO, flicker at less than 20Hz or so
gets annoying fast..."* — one opinion each, on the side of the frequency reading of the 2021 comparison
above (`topic/315322`). Asked whether he was out of missiles, ZackAttack: *"I'm using tripled players
NUSIZ setting for both players and thus also both missiles. In the case where I did use the missile I
had an extra write at just the right time to disable the missile before its 2 copies got drawn."* Two
hours later he wrote *"Think I'm going to table this until I have the hardware to test with. That way I
can be sure I'm not just fighting stella."*, and Omegamatrix answered *"I wouldn't use trust Stella
fully in this area either."* He did not stop there: he posted twice more over the next two days, still
with Stella results, of one of which he wrote *"I'm not really sure if stella is doing the right thing
here"* (AtariAge `topic/245146`; the posts run from 2015-11-01 to 11-04 by the page's own dates). That
a missile follows its player's NUSIZ copies is measured (`sprite-placement.md`, rule 4); the write that
removes the later copies is not. Our reading of the rest: that write is a result seen in Stella only,
by an author who doubted Stella on this, and since his kernel relied on bus stuffing the thread does
not show that the write fits in a line without it. **Cited only, not verified.**

**But flicker that comes and goes has its own cost.** The full form above flickers only the objects
that collide on the same lines, which makes the flicker intermittent. Thomas Jentzsch: *"Personally I
find on and off flicker more noticeable and annoying than high frequency, constant flicker"*
(AtariAge `topic/243516`, 2015). That is a third axis beside the duty and the frequency above, and
the gate reads it backwards: an on-and-off pattern has the same worst pair as a constant one and
more unchanged pairs, so `max_flicker_area` ties them and the mean prefers the one he finds worse.
That reading follows from the duty table's definitions; **Not verified** by a ROM.

**Thomas Jentzsch had said it in 2002, in a thread that drew the conclusion for multiplexers.** Clay Halliwell,
watching a Star Fire beta: *"when a sprite goes from solid to flickering, it catches your eye more than
when it goes from flickering to flickering more. So I wonder if it would be a good idea if multiplexing
routines were written so that, even if they have the opportunity to display a sprite solid, they flicker
it anyway."* 〔`200210/msg00025`〕 That is the reverse of the full form above. Thomas Jentzsch agreed —
*"constant rate flicker is less annoying than often changing on and off flicker (e.g compare Pac-Man
flicker with Robot City flicker)"* 〔`200210/msg00028`〕 — said it *"should be also tested on a TV"* and
that *"If you add constant flicker, the difference between the various flicker rates get's reduced
drastically"* 〔`200210/msg00029`〕, and later *"I think constant 50% flicker is hardly noticable, the
objects only get a bit darker"* 〔`200210/msg00031`〕. Manuel Polik, who had just cut his own flicker
down to visible vertical collisions, was not persuaded: *"Right now I'd prefer reducing flicker
situations at best, instead of going the SW-TAG way."* 〔`200210/msg00030`〕 The thread did not settle
it. **Cited only, not verified.**

**Eight months later, a fix to invisible objects was felt to reduce Star Fire's flicker.** Manuel
Polik, in an update to his third release candidate: *"Fixed any possibillities of deadlocks through
troubles with invisible objects. I finally found the ultimate solution of getting rid of them. I feel
this also significantly reduced flicker."* 〔`200306/msg00031`〕 Rob, running it in Stella: *"It did seem
a bit less flickery, and I didn't even really notice the flickering before."* 〔`200306/msg00032`〕 The
post does not say how invisible objects reached the flicker, and "them" can mean the invisible objects
or the deadlocks. Our reading takes it as the objects: an object nobody sees was
still being counted among the objects sharing the slots, so the count `SubsetsFor` is given should be of
the objects actually drawn. **Cited only, not verified** — a feeling and one player's impression, no
count.

**The duty need not be the same for every object.** Kirk Israel, 2004, planning a pterodactyl
("Pterry") between the two JoustPong players: alternate `[1 2]`, `[1 P]`, `[1 2]`, `[P 2]`, so
*"each player is shown 3 out of 4 frames, and Pterry is shown every other frame"*, against the even
split where Pterry is solid and each player shows half the time; he worried only about missing hits
〔`200402/msg00068`〕. The favoured objects get the 3-of-4 pattern — off one frame in four, the 15 Hz
gap that the 2021 CRT comparison above ranked worst. Glenn Saunders answered by not flickering at all:
draw Pterry with one missile 〔`200402/msg00069`〕, which Thomas Jentzsch said needs only relative
repositioning — `HMMx` and `HMOVE`, no timed `RESMx` 〔`200402/msg00073`〕.

The opposite choice is the even share. grafixbmp, to the author of *Taxi Panic*, about three player
sprites on the screen: *"instead of one on 60 frames and the other on 30 they could share the load and be
on 40 frames by being off every 3rd frame. Just a passing thought..."* (AtariAge `topic/249398`).
johnnywc's *"all 3 at 40hz"* below is the same split. **Cited only, not verified.**

**One object, three jobs.** Greg Troutman, 1997, planned to *"duplex one missile register at 30fps,
alternating the bad guy missiles with the player's shots"* 〔`199709/msg00195`〕; asked for retro-rocket
fire as well, he answered *"I should be able to split up one missile between the two player shots, and
the retro rockets, but divvying up the frames three ways gives me a priority problem. I'll try and make
that happen and just see how it ends up looking, I guess."* 〔`199709/msg00200`〕 He does not say what the
priority problem is. **Cited only, not verified.**

**The mean hides the longest gap.** Manuel Polik, 2001, four objects through two slots, cycling all six
pairs (`0011 0101 1010 0110 1001 1100`): *"No matter how you arrange it, you'd always have sprites
blanked for two frames, but the average display time is still 50% of the time..."* 〔`200102/msg00261`〕
The arithmetic is ours: inside a cycle that shows every pair, each object is on three frames of six, and
never being off twice running would put it on every other frame, which forces one pair onto all the odd
frames — so the claim holds for that cycle. Fixed pairs (this page's demo) never blank an object twice
running, at the price of never drawing some pairs together. No column of the duty table above reads the
longest gap; `never_for` (`scenarios.md`) can state "not off for N consecutive frames" over a RAM field,
and the demo's scenario does not use it.

**Which object flickers is a choice, and the threads choose by different rules.**
- *Never the hero.* johnnywc on a *Bruce Lee* mock-up: *"My recommendation would be to have Bruce
  never flicker and have the enemies flicker at 30hz, or you could flicker all 3 at 40hz. Of course
  they would only flicker when all 3 are on the same line"* (AtariAge `topic/347106`, 2023-01-27).
  His 40 Hz counts frames shown, two in three; the gap still recurs at 20 Hz. splendidnut's prototype
  two weeks later puts the two enemies on one player object, and they flicker.
  The rule is older. Bob Colbert in 1997 wanted to mark one sprite as never flickering, *"even if
  others fall on the same line ... (for the player controlled sprite of course)"*, as a plan rather
  than a feature 〔`199709/msg00146`〕, and Thomas Jentzsch gave the reason in 1999 while planning
  *Thrust*: *"I don't want the ship to flicker, because this is the point where the human player is
  most focussed at. So i'll better let some of the other players flicker."* 〔`199911/msg00039`〕
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

**Whether to flicker at all depends on where the eye is held.** Glenn Saunders, 2001, on a two-player
gunfight: *"I am NOT against flicker, but when your eye is focused primarily on the two gunfighters
facing eachother off, flicker is going to be really obvious. Flicker is better for games that have fast
moving sprites filling up the screen, constantly overlapping and often becoming solid when vertically
separated. Stargate and Solaris are good examples. You don't mind so much that it flickers if the payoff
is a screen full of activity."* 〔`200102/msg00329`〕 **Cited only, not verified.**

**What alternates need not be an object.** *Hellway 2 Players Edition* alternates whole views: *"I am
doing this by drawing 2 screens, alternating between left and right side each frame. From all my tests,
the game is perfectly playable and a constant 30hz flicker totally ok."* The author ties it to the game
state being deterministic — *"the screen is just a view of a world"* — and keeps control at full rate:
*"I draw the game at 30Hz (one side per frame), but most of the physics run at 60hz"*. A dark
background was the default from the start (*"Since the screen is flickering and the colors have less
intensity, dark mode became the default"*), and then the only mode (*"Due to the flickering, all non
dark backgrounds looked bad"*); when he later restored the other mode, dark stayed the default, because
*"when I tried in other monitor, the effect was much more noticeable"* (AtariAge `topic/331474`). In a thread on
hacking *Defender II*, the suggestion for adding a scrolling city alternates kernels instead: low on the
screen, *"decide if to branch out of the existing kernel IF THE FRAME COUNT IS ODD OR EVEN. That is only
a few cycles"*, with a new kernel drawing the city on its frames, where *"you can IGNORE PF COLLISION of
sprites"* (AtariAge `topic/289892`; read through our distillation note, the thread is not on disk here,
and it is a suggestion, not a built kernel). **Cited only, not verified.**

**The order of alternation, chosen for how it looks.** The rotation counter of the full form orders
the turns so that no object starves. Christopher Tumber, 2003, tried ordering them for the picture. The
rings of a *"Star Castle type game"*, drawn with both missiles in triplicate, left too little time on
a scanline, *"So I thought if I went to single missle and alternated rings, but in a real pattern
rather than just flicker (like a strobe or ripple effect emanating from the center) it might look
pretty good."* His verdict on the result: *"Actually, it was kinda distracting"* — and he kept it as
*"the start of a tunnel type effect"* instead 〔stella-list `200304/msg00235`〕. Of a version giving
*"the effect of having two rings onscreen at a time"*: *"Any more than two rings actually hinders the
effect as it gets too busy and too dififcult to track the "motion" of a ring (the extra flicker
doesn't help..)."* 〔`200304/msg00237`〕 A day later he posted a cave-shaped one that *"uses a sequence
that gives the impression of 3 polygons on screen at once, with a saturated colour so with all the
movement I think flicker is not a problem.."* 〔`200304/msg00240`〕 These are his impressions of his own demos, and none of them has been run here. **Cited only, not verified.**

**Brighten what flickers.** SpiceWare, who flickers the player's character "when needed" in Space Rocks,
Draconian, Frantic and Timmy: *"One thing that does help is to LumaBoost flickering objects - basically increase the color
values by 2 for any object that is flickering. Thomas suggested that on Dec 1, 2012 during the
development of Stay Frosty 2"* (AtariAge `topic/243516`). With luminance in D3..D1, +2 is one
luminance step, and at the top step it carries into the hue nibble. How much of the dimming it repays
is not measured.

**The other end is none.** Karl G, 2021: *"I wanted to see if I could make a 4-player maze game with
no sprite flicker and distinct player/object/maze colors and fit it into 2K, and this is the result"*
(QuadTari, `chaser.bin`, AtariAge `topic/317525`). The thread does not say how the four are drawn.

**None in one band, paid for elsewhere.** SpiceWare's Draconian (2014) draws its radar without
flicker, after Ed Fries's Rally-X, with each kind of object in its own colour. The radar uses both
players, so the six-digit score routine cannot share that part of the screen, and the score and lives
are drawn in the playfield, the older way. Read from distilled notes, not the blog post (AtariAge blog
entry `10896`); **Cited only, not verified**.

**None was once the selling point.** Piero Cavina, 2000, answering a description of games that keep
sprites on horizontal rails as copies: *"Most Activision games follow these concepts, and don't have
flicker at all (peraphs Commando is the only one with a little flicker.. anyone can confirm this?).. one
side-effect is that many Activision games look the same :-)"*, and *"I think that this explains a part
of the succes of Activision games over Atari when they first came out; Atari programmers used a lot of
flicker without too much shame at the time, while the beauty of Activision graphics was easily
recognizable."* 〔`200005/msg00161`〕 An opinion on how the games were received, with its own cost
named. **Cited only, not verified.**

**Not every viewer is a television.** Glenn Saunders, 1999, on 2600 output through a video editing
system: *"the end result of going through the time base corrector is pretty strobey and removes
flickering sprites that are updated on particular field numbers, so the motion doesn't look quite
right."* 〔`199907/msg00166`〕 The same thread found a VCR's record circuit stricter than the TV:
Edtris and TPS would not tape, Oystron and Submarine Commander did 〔`199907/msg00149`〕, and Eckhard
Stolberg's explanation was the sync, not the flicker — TPS skips the WSYNC before VSYNC, so it *"might
actually generate the vertical sync signal only for two and a half scanlines or so"* 〔`199907/msg00162`〕.
Nothing in this repository models a recorder. **Cited only, not verified.**
