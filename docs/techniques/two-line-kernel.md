# Technique #4 — The 2-line kernel

**Goal:** buy CPU headroom inside the visible region. A single-line kernel must finish *all* TIA
updates in 76 cycles per line — with two sprites, a playfield and colors, you run out. The 2-line
kernel stretches each art row over **two scanlines** and splits the work: line A updates one set
of objects, line B the other. Each job now has a whole 76-cycle line to itself. This is the
backbone of most real games (combined with #1/#10 multiplexing). The price: vertical resolution
halves — positions move in 2-line steps.

Learned from (clean-room): Darrell Spice Jr. *Let's Make a Game* Step 4; `multisprite.inc`
discussions. Demo: `roms/techniques/two_line_kernel.asm`, locked in CI by
`scenarios/two_line_kernel.json`.

## The technique

96 pairs × 2 lines = 192. Per pair (Y = pair index, sprite coords in *pair units*):

- **Line A:** vertical-compare + GRP0 store for P0 (#3's idiom, ~21 cy), then a background
  gradient `COLUBK` update (~14 cy) — two jobs, still half the budget free.
- **Line B:** vertical-compare + GRP1 store for P1, loop control. In a real game this is where
  game logic or the missile/ball updates go.

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

### Every object of a line is written in the same loop
spiceware, 2019, to a beginner planning `JSR Kernel` for the playfield followed by `JSR DrawSprites`
for the players: that cannot work, because the picture is generated as the beam passes — once the
playfield loop has run, those lines are already on the screen. Players, playfield and colours for each
line go into one kernel loop, and adding a sprite means weaving its GRP writes into that loop
〔AtariAge `topic/291513`〕. **Cited only, not verified** (held here from the distillation note; the
thread itself is not on disk here).

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

### A variant: a blank "logic" line instead of a second drawing line
Both lines of the A/B split above draw. ScumSoft, 2011, alternates a **draw** line with a **logic**
line that blanks the graphics and spends its 76 cycles on computation — *"96 Scanlines of visible
graphics"*, *"96 scanlines for logic"* — and swaps the two phases every frame (a 193-line pass
alternates with a 192-line one), so each line is drawn on every other frame; he reports *"minimal
flicker"*, and later reported a much better way to interlace the frames (not shown)
〔AtariAge `topic/178066`〕.
It pays the same halved vertical resolution as the 2LK and adds 30 Hz on every line (our reading of
the frame swap). **Cited only, not verified.**

## Verified here (Gopher2600, locked in CI)
- P0 (diamond, X=60) and P1 (frame, X=100) bounce independently in pair units over a striped
  gradient; RAM/`hmoved_pixel` asserted at fixed frames; 262 lines every frame; budget clean
  (A ≈ 45 cy / B ≈ 40 cy); golden frame.
- **Odd-row thin barrel (Combat, in-house):** re-cut East/West shape to 7 content rows → `read_row`
  measures the rendered barrel as exactly **2 scanlines**, matching the original ROM's East tank
  (body 4 / neck 2 / barrel 2 / neck 2 / body 4), with both missiles (double-push) still in budget.
