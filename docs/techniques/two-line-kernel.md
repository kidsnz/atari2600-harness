# Technique #4 — The 2-line kernel

**Goal:** buy CPU headroom inside the visible region. A single-line kernel must finish *all* TIA
updates in 76 cycles per line — with two sprites, a playfield and colors, you run out. The 2-line
kernel stretches each art row over **two scanlines** and splits the work: line A updates one set
of objects, line B the other. Each job now has a whole 76-cycle line to itself. This is the
backbone of most real games (combined with #1/#10 multiplexing). The price: vertical resolution
halves — positions move in 2-line steps.

The price is not always paid. Rob, 2003, answering Thomas Jentzsch's *"I assume a 2LK kernel is a better
idea"* for an Odyssey² port: most or all Odyssey² games appear to *"have only about 96 lines of vertical
resolution"*, so the 2LK *"seems like a reasonable solution"* 〔stella-list `200307/msg00115`〕 — a source
drawn at about 96 lines has nothing to lose to 96 pairs (our reading). And the doubled rows can be the
look you want: Kirk Israel, 2002, moved JoustPong to a 2LK when his kernel overran 76 cycles and found
that *"the two line kernal actually looks better than the single version did"* 〔stella-list
`200209/msg00108`〕 — held in full in `design-principles.md`, which also has the rule that a 2600
pixel is wide. **Cited only, not verified.**

Learned from (clean-room): Darrell Spice Jr. *Let's Make a Game* Step 4; `multisprite.inc`
discussions. Demo: `roms/techniques/two_line_kernel.asm`, locked in CI by
`scenarios/two_line_kernel.json`.

## The technique

96 pairs × 2 lines = 192. Per pair (Y = pair index, sprite coords in *pair units*):

- **Line A:** vertical-compare + GRP0 store for P0 (#3's idiom, ~21 cy), then a background
  gradient `COLUBK` update (~14 cy) — two jobs, still half the budget free.
- **Line B:** vertical-compare + GRP1 store for P1, loop control. In a real game this is where
  game logic or the missile/ball updates go.

A budget from the list, for comparison: Thomas Jentzsch, 2001, on Glenn Saunders' 2LK (Death Derby,
152 cycles per pair, a playfield of tombstones), put *"PF + HMOVE + loop = max. 55"*, the rest of the
pair being for the objects, and *"the optimal player drawing routine need 18 cycles"* — 20 *"when you
need SEC"*, while *"the illegal opcode version takes 18 cycles too (and doesn't depend on the carry
state)"* 〔stella-list `200111/msg00158`, `200111/msg00162`〕. Those are his figures for that kernel. His
18-cycle code is not in the posts, so whether it counts the same paths is not written; the skipdraw
measured here (`fundamentals-audit.md`, `vertical_pos_dcp.asm`) costs 20 on the lines that draw and 17 on
the lines that skip, not a constant 18. **Cited only, not verified.**

### Positioning two players: one shared HMOVE
Set `HMP0+RESP0` on one line, `HMP1+RESP1` on the next, then strobe **HMOVE once** after the
final WSYNC — it applies every loaded HMxx register simultaneously.
**Pitfall (cost us +3 px, found by `read_tia`):** strobing HMOVE after *each* positioning line
re-applies the earlier sprite's HMxx a second time (the registers keep their values until HMCLR
or rewrite). One strobe, after everything is staged.

### VDEL odd/even — **verified** (v1.24.0, `two_line_vdel.asm`)
The 2LK writes GRP0 on line A and GRP1 on line B — exactly the structure VDEL wants: with
**VDELP0=1, the GRP0 write parks in the shadow register and becomes visible at the GRP1 write**
(one line later). So `VDELP0 = y & 1` gives back 1-px vertical granularity with the kernel
unmodified. CI proof: P0's top edge moves **exactly +1 scanline per frame** through even and odd
positions (`TestVDELOddEven`, pixel-row measurement).
★**The mask itself is optional: `VDELPx` decodes bit 0 and ignores bits 1..7.** Measured
(`internal/emu/vdelbits_test.go`, `roms/litmus/litmus_vdel_bits.asm`): writing **`$FE`** — seven bits
set, bit 0 clear — behaves exactly like `$00`, and `$FF` exactly like `$01`. So the parity can go
straight into the register with no `AND #1`, saving **2 cycles and 1 byte per object per frame** where
cycles are scarcest. Thomas Jentzsch wrote it as a code comment in 2002 — *"don't care for the bits
1..7, VDEL ignores them"* 〔`200204/msg00067`〕 — and until now nothing here had checked it. Keep the
`& 1` only if the same value is also used as a number elsewhere.
- Carry hygiene in shared lines: an `adc` after the sprite compare inherits its carry/`lsr`
  residue — our gradient flickered at stripe edges until the add became an `ora` (valid since
  the operands can't overlap). Constant-input ops beat flag-dependent ones inside kernels.
  The same residue also crosses from one object to the other. reveng, 2011, on a kernel whose two
  compare routines ran without `SEC`/`CLC`: each player's arithmetic leaves a carry that depends on
  its own Y and hands it to the other's `SBC`/`ADC`, so the poster saw *"Everything is fine until I
  move player 0 up or down on the same scanlines as player 1. This messes up both players."* Adding
  `SEC` before both fixed it, at a cost in cycles 〔AtariAge `topic/191440`〕. The DCP skipdraw in
  `vertical-positioning.md` never reads the carry — DCP's compare sets it. **Cited only, not verified.**

### What VDEL is for, and the mechanism under "parks in the shadow register"
**Why it exists.** Eckhard Stolberg, 2002: in a two-line kernel with single-line positioning,
without VDEL *"the code for updating the two players, missiles and one playfield register would have
to fit into the HBLANK. With VDEL you can update one of the players at anytime you want during the
two scanlines."* 〔stella-list `200202/msg00196`〕 reveng gave a beginner the same advice in 2011
〔AtariAge `topic/191440`〕. So whether to use VDEL is a design decision: a kernel whose HBLANK already
holds every write gains no timing from it (our reading), and keeps only the 1-px granularity above.
**Cited only, not verified.**

**Two players without VDEL, in six cycles of HBLANK.** boutell, 2007, asked whether two freely moving
single-height players were practical without VDEL. Manuel Rotschkar (`cybergoth`) answered with a
sketch: get shape 1 into X and shape 2 into A first, then `STA WSYNC / STX GRP0 / STA GRP1` —
*"Voilà, both sprites set in only 6 cycles of HBLANK"*. The next day he gave the point of it: *"_before_
the STA WSYNC you have (almost) all the time in the world to prepare stuff - and that your loop doesn't
have to start with it."* The six are the two 3-cycle stores; the fetches run on the previous line. Its
price is a second register: vdub_bobby tends to keep X as another counter, or lacks the spare cycles.
boutell later reported *"This tactic actually works very well for me. A variation on it is doing the job
nicely"* 〔AtariAge `topic/100922`〕. **Cited only, not verified.**

**The mechanism.** Each player has two graphics registers, new and old. A write to GRPx stores the
value in that player's new register **and copies the OTHER player's new into its old**. That copy
happens on every write, VDEL on or off; VDEL only chooses which of the two is displayed — alex_79,
2018 〔AtariAge `topic/281539`〕, and Eckhard Stolberg in 2002 〔stella-list `200202/msg00205`〕. Neither
post covers the ball; that a GRP1 write also copies ENABL's new into its old (the VDELBL case) is this
repository's measurement and engine reading below, not theirs. Measured here in
`fundamentals-audit.md` §3 (`roms/litmus/litmus_vdel_cross.asm`, `internal/emu/vdelcross_test.go`), whose entry latch runs with
every VDEL bit clear and is what zeroes the old copies — without it band B passed on stale state. In
the vendored engine the copy sits in the GRP0/GRP1 write path with no VDEL test
(`Gopher2600/hardware/tia/video/video.go`: `UpdateSpritePixels`, and the delayed path in `Tick`).

**Clearing P0 and the ball under VDEL takes a second GRP1 write.** spiceware, 2016, blanking
objects in VSYNC so they stop wrapping round through the score: zero GRP1, GRP0, ENABL (and the
missiles), then `sta GRP1` again — *"yep, twice - this makes sure GRP0 and ENABL are zeroed if VDELP0 or VDELBL are
on"* 〔AtariAge `topic/253441`〕. The first GRP1 write copies whatever GRP0 and ENABL held before they
were zeroed; only the second copies the zeros. `litmus_vdel_cross.asm`'s entry latch is the same
sequence (`sta GRP1 ; latch old := 0 (ball + P0)`); the score symptom is **Cited only, not verified**.
P1's mirror is a third store: the Combat code discussed on the list clears with `STA GRP0 / STA GRP1 /
STA GRP0`, and Erik Mooney, 2002, explained the last one — *"VDELP1 might still be set, so another write
(of anything) to GRP0 is needed to make sure the zero in GRP1 is displayed"* 〔stella-list
`200203/msg00008`〕. The same entry latch's next line is that store (`sta GRP0 ; latch old := 0 (P1)`),
and `multicolor48.asm` blanks its players with the same three stores; Mooney's reading of Combat is
**Cited only, not verified**.

**Under VDEL, write both GRPx on every pass of the kernel, even where only one player is shown.**
Glenn Saunders, 2001: in his 2LK, with the two players at different starting Y, *"neither displays at
all"*, and *"When I comment out either the P1Loop or P0Loop code, it makes the other sprite not show
up."* Thomas Jentzsch: *"You are using VDELPx to position your sprites in a two line kernel with single
line resolution. So you *must* write to both GRPx registers, even when only one of the sprites is
displayed. The easiest way to do so, is to *always* write both registers during kernel."* — his rewrite
stores 0 when a player is out of range, for two more cycles. Manuel Polik's alternative in the same
thread was to drop VDEL: *"Why not just throw out all the VDELX stuff? Worked fine for me"*
〔stella-list `200110/msg00464`, `200110/msg00466`, `200110/msg00470`〕. It follows from the mechanism
above, where a parked GRP0 is shown only by a GRP1 write (our reading). **Cited only, not verified.**

**A 1-line kernel can keep VDEL on for the whole display.** spiceware's Frantic kernel writes GRP1
before cycle 22 on every line and stages GRP0 and ENABL later in the line — its cycle notes put them
at 50 and 63, *"any, on VDEL"* — because their display waits for the next GRP1 write; he names
Draconian's kernel as another 〔AtariAge `topic/257825`〕. Same thread, nukey-shay: in an
`(indirect),Y` skipdraw, replace the secondary branch with `.byte $2C` (`BIT abs`) so both arms cost
the same — the 4-cycle, 3-byte skip `integration-density-playbook.md` verifies; check the skipped
bytes against the BIT-as-NOP read hazard in `fundamentals-audit.md`. **Cited only, not verified.**

**The ball and missiles without quantising to four lines.** Missiles have no vertical delay
(`fundamentals-audit.md` §3), so the odd/even trick above cannot reach them. spiceware, 2016, to a
kernel that quantised the ball and missiles to 4 scanlines: keep two Y values per object, one for even
rows and one for odd, and *"copy/paste your 2LK, turning it into a 4LK, and revise it so that the
original 2LK uses one set of Y values and the cloned 2LK uses the other"* — supercat's suggestion;
spiceware's Medieval Mayhem converts a subpixel Y into those per-row values 〔AtariAge `topic/253441`〕.
**Cited only, not verified** — no 4LK was built here.

### When 1-line vertical steps are worth buying
VDEL and the even/odd Y values above buy back 1-line steps; a 2002 thread answered whether they are
worth it by the kind of motion. Glenn Saunders needed them *"beause the cars will accelerate and
decelerate so coarse vertical movement will be easily detectable as jerky, especially when cars
initially start moving vertically"*, and agreed *"it's okay for more constant animation where you are
moving 2+ scanlines to reach a certain minimum speed anyway."* Ruffin Bailey, the other way: his objects
moved two lines a frame, and at one line a frame *"things looked like molassas"*, so 2-line steps were
the way to go for his demo. Thomas Jentzsch for precision: Thrust's graphics are 2LK but *"the ship and
the pod move with single line precision. That looks *much* better."* Manuel Polik: *"depends on the
game"* 〔stella-list `200204/msg00063`, `200204/msg00065`, `200204/msg00066`, `200204/msg00068`〕.
**Cited only, not verified** — nothing here measured how 2-line steps look at low speed.

### Sprite thickness under 2-line — a symmetric centre feature is 2× too thick unless the row count is ODD
A 2-line kernel fetches one shape byte per **two** scanlines, so every feature is an even number of
scanlines. A top/bottom-**symmetric** sprite (e.g. an East/West tank whose gun barrel lies on the axis
of symmetry) lands its centre feature on the **centre pair** of rows when the sprite has an **even**
number of 2-line rows → the barrel comes out **4 scanlines**, twice the original's ~2. **Fix:** give the
sprite an **odd** number of 2-line rows so the centre is a **single** row = 2 scanlines. Measured on the
original Combat East tank (clean-room `read_row`): body `$FC` (4 sl) / neck `$38` (2 sl) / **barrel `$3F`
(2 sl)** / neck (2 sl) / body (4 sl) = **7 rows, odd**, barrel = one centre row. Reproduced by re-cutting
the shape to 7 content rows + 1 blank, still 2-line-paired (no parity shimmer). **Zero cycles, zero RAM.**
- **Corollary (a verification-standard instance):** don't *assume* "2-line forces a fat feature". The
  original is itself 2-line and thin — **measure the reference first**; the assumed constraint was a
  false dilemma ("thin tank *or* enemy missile") that a 5-min ROM measurement dissolved.
- Need BOTH full-resolution players AND two ENAM-stack-trick missiles and the A/B split still won't fit?
  → the **graphics-pointer 1-line kernel** (flip the line counter to Y, `LDA (Pxptr),Y` so X can stay
  pinned to `$1E` → missile reset becomes a 2-cy `TXS` instead of `PLA;PLA`). Researched, not yet built —
  memory `project-technique-candidates`. — in-house: Combat 2026-07-19/20.
- A built 1-line kernel from the forum, for scale: karl-g, 2020, writes GRP0, GRP1, ENAM0, ENAM1, ENABL,
  PF0, PF1 and PF2 on every line in 76 cycles — each through its own `(ptr),Y` pointer with one shared Y,
  the tables padded with zeros so no line branches, `LAX (P1Ptr),Y` loading X for the next pass's
  `stx GRP1`; a symmetric playfield, no colour changes, and the loop page-aligned and split into two
  83-line loops with their own pointers to keep page crossings out 〔AtariAge `topic/308505`〕.
  **Cited only, not verified** (held here from the distillation note; the thread itself is not on disk
  here).

### Every object of a line is written in the same loop
spiceware, 2019, to a beginner planning `JSR Kernel` for the playfield followed by `JSR DrawSprites`
for the players: that cannot work, because the picture is generated as the beam passes — once the
playfield loop has run, those lines are already on the screen. Players, playfield and colours for each
line go into one kernel loop, and adding a sprite means weaving its GRP writes into that loop
〔AtariAge `topic/291513`〕. **Cited only, not verified** (held here from the distillation note; the
thread itself is not on disk here).

### An asymmetric playfield shown on both lines of the pair is rewritten on both
The A/B split does not reach an asymmetric playfield where it shows on both lines (Aaron's 6-line kernel
below has its asymmetric playfield on lines 3 and 6 only). J Parlee, 2003, porting K.C. Munchkin with a 2LK,
asked how *"to keep the second playfield writes from bleeding onto the first on the second line"*; with
his right-half writes moved into Thomas Jentzsch's timing windows it still failed, and he asked whether
writing the playfield only every other line was the problem. Dennis Debro: *"If you only draw the
playfield every other line the right PF data will still be resident in the PF registers for the next
scanline. ... For an asymetrical playfield you're going to have to update the PF registers each
scanline."* 〔stella-list `200301/msg00478`, `200301/msg00486`, `200301/msg00487`〕 So both lines of
the pair carry the playfield writes and only the object work is left to split (our reading).
**Cited only, not verified.**

### Advancing a table slower than the line counter, without dividing
The pair index above already does this for one ratio (Y counts pairs, so a 2-line row needs no
shift), and `sprite-animation.md` derives an art row with `tya / lsr / lsr`. When the playfield
changes every 8 scanlines in a 2LK — every fourth loop pass — and there is no time to divide, two
forms appear in one 2021 thread 〔AtariAge `topic/317058`〕:
- **two counters** (splendidnut): an inner line counter that is reloaded when it runs out, and an
  outer playfield-row counter decremented on each reload — any period;
- **mask the loop counter** (SpiceWare's *Collect*): `tya / and #%11 / bne skip / inx` advances the
  playfield index in X once per four passes — a power-of-two period only. With X busy, keep the index
  in RAM (`ArenaIndex`) and load it only for the playfield writes.
2019's version of the question 〔AtariAge `topic/291513`〕 kept the line counter in RAM and took the
table index as counter `lsr` 1. **Cited only, not verified** — none of the three was built here, and
both threads are held from distillation notes (neither is on disk here).

### Deciding less often than once per line
The A/B split moves work between lines; three posts make a per-line job run on fewer lines instead.
- **Split the tables by parity** (Manuel Rotschkar — `cybergoth`, the Manuel Polik of the
  2001–03 posts above — 2005, Crazy Balloon): his 2LK drew the balloon on both lines at 1-line
  precision and ran the skipdraw test (`LDA #H-1 / DCP / BCC`) every scanline. He split the sprite and
  HMOVE tables into pairs — odd entries in one, even in the other (`1,3,5` / `2,4,6`), with two pointers —
  so the draw/no-draw decision runs once every two lines and the counter counts every other line. A
  binary that *"looks precisely like"* the previous one went from 10 free kernel cycles to 17. The cost
  was positioning precision, which he won back by padding the tables with zeros and shifting and
  swapping the two pointers (he describes it frame by frame and asks *"anyone following me?"*); the
  divide table halved, so it *"Didn't even cost much ROM"* 〔stella-list `200501/msg00014`〕.
- **Choose a pointer per band** (Aaron, 2004): in a 6-line kernel with an asymmetric playfield on lines
  3 and 6, a skipdraw variant (built on `ISB`) *"isn't used to actually draw the sprites, rather it
  selects which sprite pointer is to be used for the next 6 lines"*; every line then loads and stores
  both GRPx with no test, VDELP1 on 〔stella-list `200411/msg00005`, `200411/msg00015`〕.
- **Compute once, replay from RAM** (kylearan, 2017, Air Taxi): asked by cd-w about a masked playfield
  fetch (cd-w's reading: `lda (ptr),Y / and mask,X / sta PF2`, 12 cycles), he does it *"only once every four
  scanlines"* — a four-line kernel that stores the result in RAM, so the other three lines only do
  `lda tmp_pf1; sta PF1` 〔AtariAge `topic/261776`〕 (held here from the distillation note; the thread
  itself is not on disk here).

**Cited only, not verified** — none of the three was built here.

### A band drawn by the pair loop has an even height
davem, on Crossbeam: the aliens move down by growing a blank area above them and shrinking the one below
(`BottomArea`), and *"BottomArea must be an even number of lines. Since the missile objects alternate
lines on which they are drawn, the BottomArea loops in pairs of scanlines. So, any time the BottomArea is
an odd number, the logic will reduce it by 1, and add 1 to the bottom gap area between the rows of
aliens to compensate."* 〔AtariAge `topic/381984`〕 The frame total stays fixed because the odd line moves
to another band rather than being dropped (our reading). The extra scanlines he was chasing in that post
came from a repositioning loop that overran 76 cycles, not from this. **Cited only, not verified.**

### A variant: a blank "logic" line instead of a second drawing line
Both lines of the A/B split above draw. ScumSoft, 2011, alternates a **draw** line with a **logic**
line that blanks the graphics and spends its 76 cycles on computation — *"96 Scanlines of visible
graphics"*, *"96 scanlines for logic"* — and swaps the two phases every frame (a 193-line pass
alternates with a 192-line one), so each line is drawn on every other frame; he reports *"minimal
flicker"*, and later reported a much better way to interlace the frames (not shown)
〔AtariAge `topic/178066`〕.
It pays the same halved vertical resolution as the 2LK and adds 30 Hz on every line (our reading of
the frame swap). **Cited only, not verified.**

### A variant: only some registers every other line
Thomas Jentzsch, 2003, in a brainstorm on porting the Odyssey² game Smithereens (Paul Slocum's proposed
layout put the two small figures on the missiles), answered Manuel Polik's count of *"4 GRPX reads and
writes, Plus 2 * (NUSIZX + HMMX) reads and writes"* with *"How about updating NUSIZX and HMMX every
second line."* In the same post he counted a different plan, the castles in the playfield with NUSIZx
and HMMx still written on the line (~32 cycles of it), at 75 cycles, and came down for a 2LK — the reply
quoted at the top of this page 〔stella-list `200307/msg00106`, `200307/msg00110`, `200307/msg00111`〕.
Our reading of the suggestion: the graphics writes stay on every line and only NUSIZx and HMMx drop to
every other line. **Cited only, not verified.**

## Verified here (Gopher2600, locked in CI)
- P0 (diamond, X=60) and P1 (frame, X=100) bounce independently in pair units over a striped
  gradient; RAM/`hmoved_pixel` asserted at fixed frames; 262 lines every frame; budget clean
  (A ≈ 45 cy / B ≈ 40 cy); golden frame.
- **Odd-row thin barrel (Combat, in-house):** re-cut East/West shape to 7 content rows → `read_row`
  measures the rendered barrel as exactly **2 scanlines**, matching the original ROM's East tank
  (body 4 / neck 2 / barrel 2 / neck 2 / body 4), with both missiles (double-push) still in budget.
