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

In DASM the overlay can be written where the RAM is declared, so the code uses real names instead
of `temp+1`. Andrew Davie reserves one scratch area and re-opens it under new names per section
(abridged):

```
    org $80
temp        ds 8        ; general area for variable overlays
    ; overlay section 1
    org temp
overlayvar1 ds 1        ; effectively 'temp'
overlayvar2 ds 2        ; effectively 'temp+1'
    ; overlay section 2
    org temp
linecounter ds 1        ; effectively 'temp'
```

He calls managing `temp+1` by hand *"prone to error"*, and the never-live-together condition above is
his rule too: *"the same routine (or section of code) CANNOT use variables in overlay section 1 AND
overlay section 2"*, plus *"Just be careful your overlays don't get bigger than the general area
allocated for each section"* 〔stella-list `200102/msg00024`〕. Both are left to the author. This
repository's `.asm` files declare RAM with equates and contain no `org`-back overlay. Cited only, not
verified (not assembled here).

**Start with one overlay block.** Davie, 2001, on the same system: it *"doesn't require you to remember
what's used where"* 〔stella-list `200108/msg00040`〕. Thomas Jentzsch, answering, had used *"several
(mini) blocks of renamed variables"* in *Thrust*, and shared variables bit by bit, because RAM was too
short for one block (his account of what that cost is quoted in `techniques/README.md`) — and still
concluded: *"using a single overlay area (as long as possible) is CLEARLY the best way to start!"*
〔`200108/msg00042`〕. RAM-byte duty (§D) counts purposes per byte and does not tell these forms apart.
**Cited only, not verified.**

**Two names for one byte need no overlay section.** just-jeff, 2016, asked whether a routine's
`.`-prefixed equates (`.lenLo = $96`) were *"so you can use different names for the same RAM"*. Thomas
Jentzsch: *"Yup. Quite useful if you have to use the same ZP-variable for different purposes. But you
could do that without . too. The . just prevents you from using that variable accidentally somewhere
else."* — `var1 = $96` / `var2 = $96`, *"All labels are just aliases for values."* A `.` name is visible
only *"until the next SUBROUTINE marker"* (`known-traps.md` has the assembler's side of that rule).
just-jeff had met the same thing as `G48 EQU pfMasks`, for a 48-pixel graphic and a 2×6-digit score that
*"use a fair amount of RAM, but never at the same time"* 〔AtariAge `topic/257563`〕; SpiceWare's
Medieval Mayhem menu, `EQU`'d onto the castle walls, is the larger example in `design-principles.md`
("Across game states the same bytes can be different variables"). **Cited only, not
verified.**

**Two fields in one byte, and two counters.** Asked whether the unused bits of a byte holding a small
value can hold something else, SpiceWare: *"yes, that's common practice"* (his bit-testing half is in
`techniques/kernel-micro-idioms.md`). The asker closed by wondering whether one could *"have multiple
timers going on the same byte to count different things"*, and the thread ends there; nobody said how
〔AtariAge `topic/344945`〕. A count in the low field that overflows carries into the high one unless the
code stops it (our reading; neither the thread nor this repository addresses it). **Not verified.**

**Run-length coding of state in RAM.** Andrew Davie, 2020, on a snake whose body is a list of moves held
in RAM: *"Use '0' bit to indicate no change, and follow that by (say) 4 bits indicating a counter. So,
'no change for 10 moves' for example. In that case, 5 bits for 10 moves total, compared to 10 bits."* The
thread ends without saying whether this was built; what the author did build was the step before it —
two bits per segment, *"storing movement values instead of locations"*, which took the snake from about
75 segments to *"200+"* 〔AtariAge `topic/308519` wip-2600-snakes; only the distillation notes are held
here〕. **Cited only, not verified.**

**A RAM diet, item by item.** Thomas Jentzsch's *Pac-Line Panic* (2024, 4K) needs 27 bytes for each of its
eight rows kept naively, 216 in all, and the post that says so prices each cut 〔AtariAge `topic/368501`〕.
The shared speed, the animation read off the position and the reserved position value are summarised in
`techniques/input-budget.md`; three more cuts are not. Pellets: 20 bits a row *"could fit into 2.5 bytes.
But we have to unpack them on-the-fly, because the rows follow closely upon each other. There simply is not
enough CPU time for 2.5 bytes"* — so 3 bytes, left and right pellets interleaved, at *"only 2 cycles per
PF-write for a simple AND #$55 or #$AA"*, and the 12 cycles a line that leaves short are paid by drawing the
sprites single-coloured on the pellet lines (24 bytes saved). The power-pellet timer *"runs in sync with the
(ghost) speed. The faster the ghosts, the less power time. So we do not need the fractional byte here too"*
(8 bytes). The row status began as one byte per flag with a bit per row — *"8 rows fit perfectly into 8
bits"* — and ended at seven such bytes. The death animation reuses the power-pellet timer, *"there cannot be
both at the same time"*: the never-live-together rule above. Every count is his. **Cited only, not
verified.**

**A position that is also a flag.** `techniques/missiles-bullets.md` keeps an inactive shot at row 200, so
the kernel draws nothing for it without testing a flag; *Pac-Line Panic* reserves a position value for "no
power pellet" instead of a status bit (above). kikipdph, 2023, used the parked value on the logic side: a
sprite that is shot is moved to *"y = 200"*, and its movement runs only *"when the sprite is in a section
where movement is needed"*, tested as `< 195` 〔AtariAge `topic/353984`〕. One range of the coordinate then
stands for "absent" wherever it is read (our reading). **Cited only, not verified.**

**Where a table lives decides what it can do.** Manuel Rotschkar, 2004, on *Crazy Balloon*: *"I was also
forced to switch the display of the level layout from RAM to ROM, as I just didn't get the "rest" of the
game working with 22 Bytes... This also means that I have to find some new way for those levels where
parts of the background move and - yet another kernel rewrite..."* 〔stella-list `200412/msg00089`〕 The
move bought RAM and put the moving backgrounds in doubt, because a layout in ROM cannot be rewritten (our
reading of his "this also means"). **Cited only, not verified.**

**Which budget binds depends on the cartridge.** The figures in this document's first line are a 2 KB
cartridge's. Chris Wilkson, 2001, on whether bytes should go to NTSC/PAL auto-detection: *"I think at this
point, RAM is a bigger concern because there are 32K ROM boards available."* Christopher Rydberg, replying,
was *"not worried about bytes at all at this point"* and *"concerned with RAM which so far has not been a
problem"* 〔stella-list `200108/msg00017`, `200108/msg00029`〕. Generating from a seed (§3) answers a
shortage of ROM; on a large cartridge the 128 bytes of RAM can be the axis that binds instead (our reading).
**Cited only, not verified.**

**A zero without a RAM byte.** Andrew Davie, 2001, indexed his playfield rows through a table whose blank
rows point at a location that must read 0: *"I \*HAD\* one byte of RAM allocated for this task (a guaranteed
0), but I figured this was wasteful"* 〔stella-list `200102/msg00071`〕. Erik Mooney's answer: after a
`CXCLR`, a collision register can stand in, provided no collision of its kind can happen before the read —
for `$x4` (missile 0 with playfield and with ball), *"which will be true if either you're not using missile
0, you're not using PF or ball, or you're in vblank"* 〔`200102/msg00070`〕. Davie: *"I can put a CXCLR just
before the ldy, so that'll do nicely."* Only D7/D6 of a collision register are driven, though: in this
repository's engine `litmus_cxclr` reads `$32`, not 0, after `CXCLR`, the low bits being the last byte on
the bus (`fundamentals-audit.md`, "Read it with `A = $C0`"). Whether the whole byte is 0 in Davie's
`lda 0,y` depends on which bus-residue model holds, and `known-traps.md` names three (our reading). Erik's
other guess, that `$xE` and `$xF` *"always return $FF, but I'm not sure"*, meets the same file: the TIA
answers reads only at `$00-$0D`, and a write-only register returns bus residue. **Not verified.**

### 3. Generate from a seed — but only with a CHEAP generator. ★★★★ (conditional)
*(procedural-from-seed, adversarially bounded)*
Trade storage for a *tiny* amount of compute. Pitfall! synthesises all 255 screens from a
polynomial-counter **seed** (an 8-bit LFSR seeded at **0xC4**, ~50 bytes of generator, a few
cycles *per screen* — not per pixel). Entombed builds its whole maze from a
**32-byte table + a small algorithm + symmetry** (20-bit half-row → fixed 4-bit wall → 16 →
bit-duplication → only **8 bits** actually selected). **KILL (§C):** the demoscene "everything
is a function" reflex (64 kB runtime mesh synthesis, `bytebeat` audio = *f(t)*) assumes runtime
compute the 2600 does not have. The rule: **the generator must fit the per-line/per-frame cycle
slack, or be precomputed offline.** LFSR/tiny-table = yes; heavy synthesis = no.
*Source: David Crane on Pitfall! (Hackaday 2013); Aycock & Copplestone, Entombed (arXiv 1811.02035).*

**Why the hardware counts this way at all** (added 2026-09-04): a polynomial counter was cheaper
silicon. The designers: *"A polynomial counter occupies one-fourth the silicon area of an equivalent
binary counter, but, unlike a binary counter, it does not count in any simple order."* 〔Perry & Wallich, "Design case history: the Atari Video Computer System", IEEE Spectrum 1983-03 pp.45-51〕
Every LFSR in this repository — the audio dividers, Pitfall's world, `eor #$B4` — is that one
economy, and "does not count in any simple order" is the bill, paid by the programmer ever since.

**Offline precompute covers decisions too.** Zach Matley, 2003, on Tank AI, his computer-controlled tank
for *Combat*: *"with only about 16 bytes of RAM free, the idea of searching in real time was basically
hopeless. As a few of you have probably guessed, I do the searches at compile time and store the results
in lookup tables. I made a C++ program to search through the maze for all the possible tank positions
(with the playfield divided into 108 sectors)."* The search is A* 〔stella-list `200305/msg00094`〕.
**Cited only, not verified.**

**The assembler can write the table.** Andrew Davie, 2003, answering a question about score-digit
pointers, gave as *"Another way"* a pointer table that DASM builds at assembly time: a `REPEAT 10` / `REPEND` block that emits `.byte <.OFFSET`
and then reassigns `.OFFSET SET .OFFSET + SIZEOFDIGITDATA` — *"put your calculations into tables, and save
the cycles. The above code is just snazzy use of assembler ability to auto-build tables of pointers, using
a temporary label which has a value reassigned inside a loop"* 〔stella-list `200308/msg00116`〕. The post
cannot be copied as it stands: its `ScorePtrHi` block also emits `<.OFFSET`, and its own listing shows both
tables as `$14, $19, $1e …`, and the high byte is stored to `pointerP0Score` rather than `pointerP0Score+1`
(our reading of that listing; the high table needs `>`). `known-traps.md` has a DASM hang on
`REPEAT`/`REPEND`. **Cited only, not verified.**

**Computing the table once, into RAM.** svolli, 2019, on *The Mating of the Colorworms*, a 512-byte demo:
*"The trick was to use extra RAM. I chose the CommaVid 1k of extra RAM to PRECALC some tables I need for
displaying the effects"* 〔AtariAge `topic/293632`; only the distillation notes are held here, and they say
neither which tables nor when they are computed〕. Unlike Pitfall!'s screens the result is kept, and unlike
an offline table it costs RAM instead of ROM (our reading) — here a cartridge's 1K, not the console's 128
bytes. **Cited only, not verified.**

**The same choice at the size of one shape.** Pre-shifted copies in ROM, or one copy buffered in RAM and
shifted at run time: cybergoth listed both among four ways to bring a sprite in at the left edge
〔AtariAge `topic/104777`〕, and a beginner reached the RAM form on his own, so that each animation frame
would not cost ROM, and was told `ASL`/`LSR`/`ROL`/`ROR` do the shifting 〔`topic/112533`; only the
distillation notes of both are held here〕. Neither thread prices the two; the one place this repository
prices a pre-shifted copy against shifting is the font in `techniques/text12.md`. **Cited only, not
verified.**

**A union and a mask, between storing and computing.** Manuel Polik, 2002, on playfield shots that travel
to the centre of the screen: *"(PF1 + PF2) * 64 lines * 8 frames = 1024 bytes data only, without any
source code overhead"* — a quarter of a 4K cartridge (our arithmetic) 〔stella-list `200210/msg00030`〕.
Shifting did not work either: *"The problem with pure shifting is the crosshair, I didn't find a general
rule to compute the whole PF appearance in a reasonable # of cycles on the fly."* His alternative: *"I
could draw one big "X" in the ROM, and just mask the lines that are invisible on the particular frame.
This'd reduce the data somewhat, but I'd still need 16 tables with the mask data. It'd shrink the data by
40%+ though."* 〔`200210/msg00050`〕 In the thread it stayed a proposal: two days later he liked
*"the current solution"* and did not *"consider the PF one a big improvement"* 〔`200210/msg00068`〕. **Not verified.**

### 4. Runtime parametric variability from a compact config table. ★★★★
*(software product lines — CORRECTED for 8-bit)*
Ship many curated variants from one artifact. **Not** the compile-time `#ifdef` model
(that yields *N separate binaries* and each compositional "feature module" costs hook-method
bytes). The 2600 form is a **third mechanism**: one ROM, config bits (console switches / a RAM
byte) **index a small parameter table** at runtime → many variants from shared code
(Combat: **27 variants from a ~28-byte table**). Treat each config bit as a *modeled feature with constraints* so only
valid/curated combinations are reachable. Density metric: *feature-count-per-K* (§D).
*Source: Kang et al. FODA; Kästner & Apel (annotative vs compositional); Combat 27-variant table.*

**Shared across games on one cartridge.** wickeycolumbus, 2013, on his *Mini Game Collection*: *"There are
several routines shared across the 3 games on the cartridge, including the score kernel, "blank" kernel,
joystick/switch reading, random number generation, and probably more that I'm forgetting."* To the idea of
making each game a list of routine numbers: *"Making the entire game a table of subroutine addresses
probably won't compress it too much. There are many cases where the code would take less space if it weren't
a subroutine."* roland-p added that a jump table stores two bytes per routine, and `LDX #subroutine_id` /
`JSR gotosubroutine` *"doesn't take less space than"* `JSR mysharedsubroutine`; he pointed to a BRK-based
call instead, which §G prices 〔AtariAge `topic/208200`〕. Doing the same for a larger multicart was only
proposed (`design-principles.md`, "The same move scaled to a multicart was proposed, not built"). **Cited
only, not verified.**

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

**Clearing by wrapping the stack.** Andrew Davie's tutorial, 2003, takes the power-on clear from 258 bytes
(one store per address) to an 11-byte loop, then to a 9-byte `sta 0,x / inx / bne` loop that clears TIA
and RAM together, and closes with a *"magical"* 9-byte version that also sets the stack pointer:
`LDX #0 / TXS / PHA / TXA / CLEAR PHA / DEX / BNE CLEAR`, commented *"DOES THIS BY "WRAPPING" THE STACK -
UNUSUAL"* and *"STACK POINTER NOW $FF, A=X==0"*. It was not his last: later in the thread he gave *"an
EIGHT byte solution"*, `ldx #0 / txa / Clear dex / txs / pha / bne Clear`, after which *"X=A=0, and all of
RAM and the TIA has been initialised to 0, and the stack pointer is initialised to $FF"* — the loop that
DASM 2.20.14.1's `macro.h` puts at the core of `CLEAN_START` (`.CLEAR_STACK dex` / `txs` / `pha` / `bne`,
after `sei`, `cld`, `lxa #0` — `ldx #0` / `txa` under `NO_ILLEGAL_OPCODES` — and `tay`; `known-traps.md`
notes that it writes every TIA register). Neither of Davie's versions runs `CLD`: Thomas Jentzsch, in the same thread, *"SEI isn't necessary, but CLD should be done"* (the
`missing CLD` row of `known-traps.md`). ericball, replying to the 9-byte version: *"in my experience is
it's not always necessary to clear all of the TIA registers & RAM"* 〔AtariAge `topic/27405`〕. **Cited
only, not verified** — not assembled here.

**Defaults inside the clear loop.** Edwin Blink, 2004, on variables that need a non-zero start value
(*"default game type or high score for example"*): copy them from ROM with a 10-byte `LDX` / `LDA
DEFAULTS,X` / `STA VARS,X` / `DEX` / `BPL` loop, and then *"One byte can be kicked out of the above code
again by integrating the code with Clean start code"* — the copy placed inside a `DEX` / `TXS` / `PHA` /
`BNE` clear like the one above, so that *"first all ram is cleared then when clearing the tia vars are
initialized with their defaults"*, worth it only for more than three of them (*"must be >3 to save
bytes"*) 〔stella-list `200409/msg00044`〕. As posted, the test is `CPX VARLENGTH+1` with no `#`, followed
by `BNE`, which copies at one value of X only, and once a default has been loaded A is no longer 0, so the
`PHA`s after it fill the rest of the TIA with that value (our reading; nobody in the thread discusses the
loop). **Not verified.**

**Entries instead of a parameter load.** Thomas Jentzsch, 2001, on an often-called subroutine whose
parameter *"is mostly one of only some different values"*: replace each `lda #n` / `jsr MySub` with `jsr
MySubN`, where the entries are `MySub0: lda #0` / `.byte $2c` / `MySub1: lda #1` / `.byte $2c` … falling
into `MySub` — each `.byte $2c` (`BIT` absolute) swallows the next `lda #` where a 2-byte branch would
otherwise jump: *"This is my optimal solution for that."* Where the flags must survive, his *Thrust* uses
`$0c`, an undocumented 3-byte `NOP` absolute, instead 〔stella-list `200103/msg00052`〕. Manuel Polik:
*"Saves memory, wastes cycles :-)"* 〔`200103/msg00058`〕. The `$2C` skip is measured in §G; `$0C` is not.
Ladders of entry points used as delays are in `techniques/kernel-micro-idioms.md`. **Cited only, not
verified.**

**Digits on an 8-byte stride.** With a 5-line font the digit index is ×5 (`casebook.md` has Combat's
`ASL`/`ASL`/`ADC`). Lee Fastenau, 2004, to someone *"very low on rom space"* 〔stella-list
`200405/msg00092`〕: *"are you certain you can't spare the 30 bytes to make each digit take up 8 bytes
instead of 5? Do you have any ALIGN 256's that you haven't optimized in a while?"* 〔`200405/msg00105`〕.
Andrew Davie, the same day: the gaps between digits can hold small tables, and a table that starts with a
digit's last bytes or ends with a digit's first bytes can overlap it; the cost is *"27 bytes at
worst"* (the last digit needs no gap), *"LESS the improvements in the actual x5 code, which would be
replaced by a simple x8 (which for the BCD high byte would be just an AND and a LSR)"* 〔`200405/msg00107`〕.
`techniques/asymmetric-pf-score.md` uses 8 bytes per digit. **Cited only, not verified.**

**Forward and mirrored digits in one byte.** JeremiahK, 2020, to an author drawing a 3×5 score with the
playfield: *"you have to store 2 copies of the digits, one being flipped"*, but *"Bits 765 and 321 hold
regular forward copies as usual. Bits 543 hold a mirrored version of the same graphics."* With columns `a b
c`, the byte reads `a b c b a b c x`: bit 5 is `c` and bit 3 is `a` in both copies that share it, so any
3-wide glyph fits (our check, on paper). The display must *"shift the copies into place before ORing them
together"*; with ten digits *"the table is 50 bytes, but with the extra overhead to handle the mirrored
copies, it would probably save you 30ish bytes"*, and that game's eleventh character *"wouldn't work with
this method"* 〔AtariAge `topic/311006`〕. `techniques/asymmetric-pf-score.md` has the nibble form of a
reversed copy; Ed Fries' 26-letter font in 28 bytes, overlapped both ways, and his verdict on it are in
`design-principles.md` ("Packing a table pays only when the table dominates"). **Cited only, not verified.**

**The bytes in front of an alignment are still ROM.** `BOUNDRY`, *"a new macro from Dennis"* in the
updated `macro.h` of 2004, as Edwin Blink explained it to Manuel Polik, who had asked how it differed from
`ORG $F500` / `ds 5`: *"with BOUNDRY you don't have to do the ORG $F500"*; *"Bytes waisted for the boundry
can still be used to do something usefull. In that case you put the usefull stuff before the boundry.
Instead of ORG$F500, ds 5 You could also use DS <(5-.) to do the same"* 〔stella-list `200409/msg00035`,
`200409/msg00036`〕. B. Watson, 2001, set a fixed cost against such gaps, the BRK call's handler in §G:
*"The 8 bytes of overhead aren't even really an issue, since I have a few small chunks of empty ROM at the end of various data
tables (where the table is <256 bytes, and the next table must start on a page boundary for timing
reasons)"* 〔`200112/msg00094`〕. Jentzsch's free-space macros in §D count such gaps, and `known-traps.md`
prices an `ALIGN 256` as a step. The `BOUNDRY` macro is not in this repository. **Cited only, not
verified.**

**Three hints from one byte review.** Thomas Jentzsch, 2001, to Erik Eid, whose 4K *Euchre* had run 276
bytes past an `org` 〔stella-list `200109/msg00043`〕: *"use bxx instead of jmp where possible"*; *"avoid
all subroutines that are only called once. (yes, the result will be spaghetti code, but you're always
wasting 4 bytes here, and sometimes you will discover unoptimized code easier)"*; and, in one of his
worked examples, `cpx`/`cpy` in place of `txa`/`tya` and `cmp` — *"Another two bytes saved here."*
〔`200109/msg00046`〕. The 4 bytes are the `JSR` and its `RTS` (our arithmetic). **Cited only, not
verified.**

**One diet, itemised.** Two months later Eid, *"a beginner"* by his own account, reported *"almost 260 bytes
of code"* gone from *Euchre* without losing *"the message display capability"* (the feature priced in §D),
in fifteen numbered changes; his stated total is 257, and the items add to 276 〔stella-list
`200111/msg00408`〕. Four swap code for table reads (20, 19, 6 and 24 bytes — *"The larger savings is due to
table reuse!"*), and one replaces a compare-branch section by computing the address of the right `jsr` and
taking an indirect `JMP` (30). Together 99 bytes, about 36% of the 276 (our arithmetic). The largest item
folds code *"repeated four times, differing only in the suit being examined"* into a loop (66). Dropping the
duplicate J, Q, K and A images by moving the rank images into the letter table — the glyph sharing named
above — saved 18. The instruction-level edits — an unneeded `clc`, an unneeded `sec` (*"If bcc is not taken,
carry must already be set"*), `lda`/`tax` and `lda`/`tay` folded into `ldx` and `ldy`, and an `eor` whose
result was already the value needed — came to 9 (items 6–9; our grouping). One beginner's program; that it
argues for restructuring before single instructions is our reading. **Cited only, not verified.**

**A carry you can prove needs no `CLC`.** cd-w, 2017: *"When writing 6502 code, you can often optimize out
the CLC instructions as you can infer that the carry will not be set at that point in the code, e.g. you
have already performed an addition that cannot overflow."* 〔AtariAge `topic/262178`〕 Two worked cases are in
`techniques/kernel-micro-idioms.md` ("Fold the borrow into the constant", "A carry the shifts already
cleared"), and Eid's `sec` above is the same inference after a branch. The inference stays the author's:
`internal/cyclebound`'s abstract interpreter follows C through `CLC`, `SEC`, `SBC`, `CMP`/`CPX`/`CPY` and
the two carry branches but sets it unknown after every `ADC` (`absint.go`), and it uses flags to price and
prune paths, not to report a flag instruction as removable. **Cited only, not verified.**

**When not to take a saving yet.** Thomas Jentzsch, reading a DiStella listing of Davie's *Qb*, pointed
at two places to save bytes — one a caller's `LDA #$08` / `JSR LFA60` where the instruction just before
`LFA60` is that same `LDA #$08` (*"And this happens more than once :)"*), the other `LDX #$00` followed by
`LDA #$00` 〔stella-list `200102/msg00346`〕. Davie: *"These are interesting byte-level optimisations which
I will hold-back on for now. They both create a dependency between the code and values used for creature types.
Not a serious dependency, but these things are to be avoided until the last possible moment."*
〔`200102/msg00349`〕 Jentzsch drew the line for RAM differently, early in Glenn Saunders' *DD*: asked
whether *"save 11 bytes"* meant RAM or ROM, *"RAM of course. It's far too early to optimize for just 11
bytes of ROM."* — and *"I think the whole game can be done with only about 6 bytes of stack"*
〔`200301/msg00392`〕. That is his estimate for one game; the 16 bytes in
`internal/emu/stackbudget_test.go` is this repository's guard. **Cited only, not verified.**

**Bytes or cycles: perhaps decided by how full the place is.** Dennis Debro, 2003: *"I'm no expert but I've
found that it depends. If I know I have plenty of time to execute my instructions in VBLANK or overscan, I
would choose bytes over cycles. In the display kernel it would depend on the situation. Cycles would be
important here so you would keep up with the raster."* 〔stella-list `200305/msg00024`〕 Retroware (signed
Bob), 1998, putting a sprite sort for his multi-game engine to the list — *"this piece of code executes in
the VBLANK section of my game"*, 666 cycles by his count — had asked the opposite of the same region:
*"Optimizing for time is necessary, not space, so if you can do it in fewer cycles that would be awesome
and larger code size is o.k."* 〔`199805/msg00112`〕. So the region's name does not settle it; how much of
the region is already spent does, which is the condition in Debro's *"If I know I have plenty of time"*
(our reading). §G's first unsolved question leans on the same off-screen budget. **Cited only, not
verified.**

**Dense code reads like obfuscation.** Asked in 2016 whether 2600 games were obfuscated against reverse
engineering, kylearan doubted it: techniques such as *"semantic NOPs, jump into the middle of instructions,
opaque predicates, VM packing etc. always come at the expense of ROM"*, decryption on the fly needs RAM,
*"so I heavily doubt they were used back then"*. Thomas Jentzsch: *"I have disassembled a number of old
Atari 2600 games and there is also some original source code available. But I haven't found any intentional
obfuscation. However there exist some optimizing tricks (e.g. using BIT to skip the next instruction), which
may look like obfuscation."* 〔AtariAge `topic/247638`〕 The `BIT` skip is measured in §G; kylearan's
`CMP #$C9` slide from the same thread (such slides *"in fact often confuse disassemblers in a similar way"*) is
in `design-principles.md`. **Cited only, not verified.**

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

**Hand-rolled packing, one worked form.** Andrew Davie, 2003, on the large sprites of his fighting game:
*"The large sprite is actually composed of a matrix of smaller sprites."* *"The compression comes from the
ability to re-use the small sprites in making the large sprite, and also in not needing to define the blank
areas (many frames in my fighting game will have lots of blank small-sprites)."* The frame shown, 48 pixels
wide (he wrote *"48 x 64"*, then *"the sprites are 48 x 80, not 48 x 60"*), *"uses rougly 128 bytes of
data"*, and he estimated *"almost 200 frames of animation"* 〔stella-list `200301/msg00106`,
`200301/msg00107`〕. Every number is his. **Cited only, not verified.**

**Hand-rolled packing in stages, the price moved off ROM.** piledriver, 2024, on how *Oh Shoot!* got its
screens. Stored plainly, PF0/PF1/PF2 for 22 rows is 66 bytes a screen, drawn straight from ROM: *"I think
this only allowed me 4 screens in my 4KB game? Don't remember."* Then *"used 4 unused bits in PF0 to store
run length value"*, *"identical adjacent rows now grouped into chunks"*, *"extracted on the fly in the
kernel code (not ideal but it worked)"*: 16 screens (the PF0 half is in `techniques/kernel-micro-idioms.md`,
"PF0 is a second carrier"). Then each screen became *"a list of indexes into a chunk palette"*, 2 to 15
one-byte indexes, *"limiting the number of times I can change the playfield values to 15 times per screen"*,
each chunk PF1, PF2 and a run length, *"I ditched PF0"*. That *"could no longer be decompressed on the
fly"*: *"22 rows x 2PF bytes of RAM"*, *"44 bytes of RAM required!!!"*, and *"one frame between rounds to
load/decompress the screen into RAM"*; *"about 640 bytes for all my screens"* held 64. The game then went to
32K (F4, by the thread's opening post), with four banks of level data for 1024 screens and a PC tool that
reports how much ROM each screen set uses 〔AtariAge `topic/367878`〕. Each 4K step bought screens with
something other than ROM — kernel time, then RAM, a frame, PF0 and changes per screen (our reading). The
counts are his; the ROM was not examined here. **Cited only, not verified.**

**An entropy coder and its pointer tables in 540 bytes.** Thomas Jentzsch, 2001, on fitting 600 *Jammed*
puzzles (Rush Hour on a 6×6 grid) into 4K 〔stella-list `200102/msg00199`〕: a puzzle is twelve strips, six
rows and six columns. Before coding a strip he counts the patterns that still fit its empty squares — *"22
(empty strip), 13, 8, 7, 4, 2 and 1 (only the empty pattern fits) and there's a special compression table
for each number"* — frequent indices get shorter codes, *"(If the number is 1, i don't need any bits at
all!)"*, and rows alternate with columns because *"The sooner i can reduce the number of free patterns, the
more efficent is the compression."* The content was fitted to the coder too: *"Then I tried all computed
defined pattersn, and choose those which I could compress best, optimized the compression tables and
iterated the process"*, reaching *"less than 2.3 bits/strip or about 2k for 600 puzzles"* at *"~7000
cycles/puzzle"*. The decoder is *"about 540 bytes (including decompression and level pointer tables)"*,
beside about 1300 bytes of other code 〔`200102/msg00233`〕. Eckhard Stolberg's version of the same game went
without an efficient packing method: *"I didn't need to come up with an efficient packing method. This means
that my version can only store about 250 levels in 4K. But on the other hand my version can handle levels
with two escape cars or levels where the escape car is 3 squares long."* 〔`200102/msg00170`〕 Where
Jentzsch's decode went in the frame is in `techniques/blank-a-frame.md`; the break-even is in §G. **Cited
only, not verified.**

**Packing does not shrink a RAM buffer.** Andrew Davie, 2001, planning Conway's *Life*, proposed run-length
packing of the board in RAM, since most of it is 0 during a game 〔stella-list `200103/msg00087`〕. Erik
Mooney: *"That's a nice compression for the average and best cases, but the kernel and algorithm have to be
able to handle the worst case"*, and *"Sure you can compress it, but it doesn't save you RAM in worst-case
scenarios, which you have to support."* Answering Clay Halliwell's point that the grid must be held twice,
current and next generation (〔`200103/msg00093`〕, quoted in the reply), Mooney: *"No cell can affect or be
affected by another nonadjacent cell. So we can go with as few as 2 rows of common storage"*
〔`200103/msg00096`〕; the 1997 *Life* thread's *"floating" update window* is in `design-principles.md`.
Packed ROM data has one size, fixed when it is built; a RAM buffer is reserved for the worst content the
game can reach (our reading). **Cited only, not verified.**

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
> that can't be gamed by a resource trade is unsolved (§G). For one flag a practitioner gave every arrow:
> a byte per flag is faster and uses less ROM, packed flags save RAM and cost slower, longer code
> (Christopher Tumber, quoted in `techniques/kernel-micro-idioms.md`, "The trade has three sides").
> **Cited only, not verified.**

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

**A window, not only a ceiling.** Eckhard Stolberg, 1998, on Andrew Davie's four-colour playfield demo,
whose extra colours come from flicker: *"This version flickers a lot less, than the last one, but that may
also be due to the fact, that you don't have big areas of playfield anymore. Unforunately the colours are a
bit hard to tell apart in finer structures. If you can manage to create playfield structures, where the
blocks are big enough to distinguish the colours, but small enough for not to flicker too much, I would
say, that this technique is good enough for being used in a game."* 〔`199805/msg00194`〕 `max_flicker_area`
holds the upper side as a number the author sets; nothing here checks the lower side. `pkg/design/color.go`
(`SameLuminance`) records the same trade on the luminance axis — least flicker where two colours stop
reading apart — and says its threshold is not measured in this tree. **Cited only, not verified.**

**ROM priced by the feature.** `size` in the table counts a whole image. Erik Eid, 2001, with *"only about
another 130 bytes or so to work with"* in a 4K *Euchre*, priced one feature — the message line — in three
parts: letter images *"29 (all letters, a space, a 2, and a 4) * 6 = 174 bytes"*, the messages *"25 * 6 =
150 bytes"*, and *"code sprinkled throughout the game regarding deciding what message to show; I make a
conservative estimate of it at 120 bytes. All this together is 444 bytes, or at least 10.8% of the
available space!"* The routines he would keep anyway for a couple of fixed messages he left out
〔stella-list `200111/msg00336`〕. Only the third part is an estimate. Manuel Polik, replying: *"I
somehow can't believe that 444 bytes of data are really a problem"* 〔`200111/msg00339`〕. **Cited only, not
verified.**

**ROM counted gap by gap, during assembly.** Thomas Jentzsch's free-space macros: `OUT_FREE` wraps an
`ALIGN` or `RORG`, measures the gap it opened, adds it to a bank sum (`FREE_BANK`, *"has to be set to 0
for each bank"*) and a total, and ECHOes the address, the gap, the bank sum and the total;
`COND_ALIGN_FREE` aligns only when the next given number of bytes would run onto the next page
〔AtariAge `topic/267367`〕. This repository reads the built image instead (`internal/build.ROMBytesUsed`:
a lower bound on the bytes used, and the `$FF` run at the end), and reports neither gap by gap nor bank by
bank. **Cited only, not verified** — the macros have not been assembled here.

**Bytes per file, and what the gaps are for.** Dionoid, 2022, splitting a game into include files, ends each
file with `BYTE_COUNT`, which ECHOes the file's name and the bytes emitted since a start label; his
`ALIGN_PAGE` ECHOes how many bytes `align 256` skipped, *"bytes free before"* the named block (where he used
it the post does not say); his third macro, `CHECK_PAGE`, is in `design-principles.md` 〔AtariAge
`topic/345618`〕. Thomas Jentzsch, 2016, on where such gaps come from, after a suggestion that a linker would
place routines in them: *"Usually those unused bytes result from avoiding page crossing penalties. And when
you have to align code to a new page, you have to manually look e.g. for a small table which fits into the
space. I doubt a compiler can help me here."* kylearan's answer survives in this copy only as its quotation;
Jentzsch's next post: *"Sounds like I have to give it a try."* 〔`topic/252613`〕 `tool-landscape.md` follows
the linker question into 2017. `ROMBytesUsed` above has one caller, its own test
(`internal/build/romsize_test.go`). **Cited only, not verified** — the macros have not been assembled here.

**Where half a K came from.** Andrew Davie, 2001, itemised what *Qb* v0.10 had won back, headlined as 500
bytes: *"about 200 bytes"* from converting the sprite routine to *"the system as used/suggested by Thomas
Jentzsch"* — *"it effectively involved a complete rewrite of the entire kernal"*, and the new kernel lost
the player's colour changes; *"another 50 bytes"* from storing the playfield as *"3 columns each 19 bytes
long"* rather than 19 rows of 3, after which X *"could count in ones... and it could be used to index tables
by row"* and *"the TargetIndex table was shrunk in size by 2/3rds (38 bytes)"*; and *"another 256 bytes or
thereabouts"* from folding three kernels into one — *"95% succeeded"*, all but flashing the target area's
top and bottom border. A switch at the top of his source sets where sprite data is written: *"set it to
COLUBK to see timing in colour on the screen itself"* 〔stella-list `200102/msg00256`〕. The figures are his;
200 + 50 + 256 is 506 (our arithmetic). **Cited only, not verified.**

**What the shortage put on screen.** Andrew Davie, 2001, releasing *Qb*'s first alpha: *"I ran out of ROM
long ago, and since then its been a matter of shaving, shaving, installing something, running out of
memory, shaving shaving, etc."* and *"Memory is very tight. I have none left! From now, to get extra
memory I have to drop features/capabilities."* Two things in that build he put down to memory: *"If you
lose your last life, the background changes red (yes, I'm that short of memory)"*, and *"a
flash/corruption of the top/bottom border of the target area display. This is due to the scarcity of
RAM"*. A title screen he estimated at *"about 160 bytes.... that's about 155 bytes more than I have right
now"* 〔stella-list `200102/msg00334`〕. His cut list from the same month is in `design-principles.md`.
**Cited only, not verified.**

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
- ✓ **BRK as a call is 2 bytes, not 1** (BRK+2 and the RTI return verified 2026-09-15 on a scratch ROM; cycles from the 6502 table): BRK pushes BRK+2 and RTI returns there, so
  a call is `BRK` + one skipped byte — **1 byte shorter than `JSR`, not 2** — and since RTI restores the pushed status the
  result comes back in A, never in flags; 13 cycles against JSR/RTS's 12. Shipped: Video Olympics' BRK vector is
  `$F438` = `0A 69 00` ×4 + `40` (a nibble swap), called as `00 EA` from `$F262`, `$F2C8`, `$F453` (the 2K image runs in the $F000 mirror).
- ☐ **The skipped byte as an argument (Cited only, not verified).** Paul Slocum, 2003, writing up a trick
  Thomas Jentzsch found in Mark Lesser's *Lord of the Rings* prototype: calls of the form `brk` /
  `.byte $0e ; id-byte`, and a BRK handler `plp` (*"remove flags from stack (not needed)"*) / `tsx` / `inx` /
  `dec $00,x` (*"adjust return address"*) / `lda ($00,x)` (*"read break-id..."*) / `tay`, falling into a
  subroutine that ends in `rts`. Against `ldy #value` / `jsr Subroutine`, *"it saves 3 bytes with each call
  and the overhead is only 8 bytes. After only 3 subroutine calls (Lord of the Rings has about 20) you are
  saving ROM space."* There the subroutine *"selected the sound effect to be played based on a priority
  system"* 〔stella-list `200302/msg00037`〕. Asked whether, without pulling the flags, the handler would
  need `rti`, Slocum: *"Yeah"* 〔`200302/msg00039`〕. Jentzsch's own post, 2001, from his disassembly, gives
  the rest of the handler: after `tay` it finds which of two bytes (`$ef`/`$f0`) holds the lower id and, if
  the new id is larger, overwrites it and loads two table values into `$f1,x`/`$f3,x` — *"First I thought,
  this was debugging code, but then I found the registers filled in that routine are used for... SOUND!
  (Each sound has it's own priority and only the highest two are played.)"* 〔stella-list
  `200112/msg00093`〕. That arbitrates by comparing ids, a different scheme from the write order Combat uses
  (`design-principles.md`, "SOUND PRIORITY"). B. Watson, replying, judged it not worth it for his game.
  The handler was no obstacle — the 8 bytes could go into empty ROM at the ends of his tables (quoted in §6) —
  but *"this may not be worth it, since I don't have any subroutines that take a parameter in a register, so
  I won't be saving the 2 bytes for the LDY #blah, and I think I only have maybe 3 JSR's in all the code. So
  I'd only save 3 bytes or so... maybe I won't bother after all."* 〔`200112/msg00094`〕 With the handler in
  space that was free anyway, those three calls would still have come out about 3 bytes ahead; what he
  declined was the trouble, for a small saving.
- ☐ **BRK as a trap, and where the unused vectors point (Cited only, not verified).** Eric Ball, 2003,
  reposting his AtariAge advice: *"Although the 6507 doesn't have NMI or IRQ pins, it's good programming
  practice to have those vectors point somewhere, either the same as RESET or an RTI instruction. (And to
  include an SEI instruction in your initialization section.) That way if the 6507 flakes out it won't
  crash horribly."* Thomas Jentzsch's addendum, quoted in the same post: *"you can check if a branch is
  always taken by adding a BRK just behind it and point the IRQ vector to a special routine or simply to
  the RESET address"*, and *"Some games (Parker Bros) even use BRK to call subroutines"* 〔stella-list
  `200308/msg00032`〕. Dennis Debro: *"if you just point the IRQ to RESET (which most of us do), then is it
  needed?"* 〔`200308/msg00033`〕. Manuel Polik called BRK an NMI; Jentzsch: *"No, BRK uses $xffe, which is
  the IRQ vector."* 〔`200308/msg00035`, `200308/msg00038`〕. On the `SEI` half, `techniques/missiles-bullets.md`
  has it doing nothing here — no path by which an IRQ can reach the 6507, checked in the vendored engine.
- ☐ **What BRK does to B and I (Cited only, not verified).** B. Watson, 2005, quoting Stella's CPU core:
  `B = true` before the three pushes, `I = true` after them, then the vector from `$FFFE`/`$FFFF`; z26's
  core does the same, so *"the I flag gets set after the PC and status are pushed, so an RTI restores
  its original state"* (his emphasis on "after" dropped) 〔stella-list `200507/msg00169`〕. Why B is set the thread did not settle: Eckhard
  Stolberg's account is that B is wired to the interrupt input, held high on the 6507, so *"on the VCS the
  B flag should always be set"* — *"the theory I think is most plausible"* 〔`200507/msg00172`,
  `200507/msg00180`〕; Dennis Debro, *"IIRC"*, has it set by BRK and cleared on IRQ/NMI
  〔`200507/msg00174`〕. Watson's and Stolberg's accounts both leave B set in the byte a BRK pushes here.
  The bundled engine sets I after pushing the status, and `Status.Load` sets B on every load, reset
  included (`Gopher2600/hardware/cpu/cpu.go` BRK case, `registers/status.go`; read, not run).
- ☐ **Still to verify** (lower priority, deferred): the
  **shared envelope/glyph table** 24-byte saving (*Dominant Amber*) — reproduce with `assemble_and_load`
  + byte count when a build actually reaches for it.

**Unsolved questions (the research agenda this playbook opens):**
1. **Compute↔storage crossover.** At what *generation cost per row/screen* does procedural-from-seed stop
   paying, given 76 cy per visible line **plus** the larger off-screen VBLANK/overscan budget? (Measure it.)
   Two priced cases from the archive, neither a generator from a seed. Decoding instead of storing:
   *Jammed*'s 540-byte decoder is a fixed cost, after which a level costs 2.3 × 12 / 8 ≈ 3.45 bytes by
   Jentzsch's own formula (§C); Stolberg's version, without an efficient packing method, works out at most
   about 12 bytes a level from his figures, so the decoder pays for itself after at least about 60 levels
   (both our arithmetic). Building each line's bytes during the line before: Manuel Polik, 2001, on
   *Outlaw*'s playfield score, *"the actual graphical data is 10 Bytes, but the RAM usage is only 2
   Bytes(!). There's two blank lines at the beginning, then every line creates the data for the next line on
   the fly"* 〔stella-list `200102/msg00021`〕. In the posted loop each digit row is drawn on two scanlines
   and the next row's two bytes are built during them, so the price is those lines' cycles plus the two
   blank lines (our reading of the code). **Cited only, not verified.**
2. **Single density index without gaming.** How to weight/normalise the scorecard's axes into one comparable
   number that cannot be gamed by trading one interlocking resource for another (cycles↔bytes↔RAM)? (§D caveat.)
3. **"Dense" baseline.** What is the *measured* functionality-per-byte of the references (Combat 27 / ~28 B;
   Pitfall 255 / ~50 B) vs a modern hand-built ROM — i.e. what ratio counts as dense, making the scorecard
   absolute rather than merely relative?
