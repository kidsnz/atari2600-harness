# Changelog

The change history of this project. Format follows [Keep a Changelog](https://keepachangelog.com/);
versions follow [Semantic Versioning](https://semver.org/).

> Entries from v0.17.0 and earlier are condensed; the full detailed history (in Japanese) is kept locally
> in `CHANGELOG.ja.md`.

### Fixed — `check_provenance` rewrote a commercial ROM's `.bin` to a `.asm` that will never exist (2026-09-04)

Citing `sandbox/studies/combat/Combat.bin` turned the gate red locally. The rule it hit is a good one:
a `.bin` we **build** is gitignored, so it is absent from a fresh checkout and present on any machine
that has run the build — its existence is a fact about the working copy, not about the citation — so
the check resolves it through the `.asm` beside it.

**A commercial ROM image is not a build product.** There is no `Combat.asm` beside it and there never
will be: not having Atari's source is the clean-room line, so the rewrite was asking for a file whose
absence is deliberate. `_resolves` now tries the path as cited first and falls back to the `.asm`.
Negative control: `Combat.bin` resolves, `NOSUCH.bin` does not, and `roms/techniques/bullets.bin` still
resolves through its source.

The file's own docstring counts the times this check has gone red for citing something real — the
umbrella, the commercial corpus, the build-product rewrite. **This is the fourth**, and it is recorded
there.

Worth noting how it surfaced. The pre-push hook mirrors CI in a throwaway worktree, where `sandbox/`
does not exist, so `_umbrella_present()` short-circuits and umbrella citations are skipped entirely —
the push went through and CI stayed green. **The gate is stricter on a working machine than in CI**,
by design, and it caught something CI structurally cannot see.

### Added — Stella cross-check for both new litmus ROMs, 37/37 each; the queue is empty again (2026-09-04)

`litmus_flicker_attrib` and `litmus_pf0_reflect` were queued yesterday because capturing drives Stella's
GUI and takes the screen for about thirteen seconds per ROM, and the machine was in use. Captured now,
with the author's go-ahead:

    PASS: 37/37 write-only TIA registers agree (litmus_flicker_attrib.bin @ frame 5)
    PASS: 37/37 write-only TIA registers agree (litmus_pf0_reflect.bin  @ frame 5)

Both queue lines removed; `CAPTURE_QUEUE` holds no ROMs. The oracle covers the whole corpus again.

Worth noting what this does and does not add. It says Gopher2600 and Stella agree on every write-only
TIA register these ROMs touch — two independent implementations reaching the same state. It does not
make either of them hardware, which is the line the ⬜ entries in `fundamentals-audit.md` keep.

### Fixed — "unverifiable" was wrong: the Combat ROM is in `sandbox/`, and it does write SWBCNT (2026-09-04)

An hour ago this recorded the list's DDR counterexamples as impossible to check, *"neither ROM is in
`reference/`"*. **The search was scoped to one of the four repositories and the conclusion was written as
if it covered all of them.** `sandbox/studies/combat/Combat.bin` has been there the whole time.

Decoded from its bytes: `78 D8 A2 FF 9A A2 5D 20 BD F5 A9 10 8D 83 02 …` — `SEI / CLD / LDX #$FF / TXS /
LDX #$5D / JSR $F5BD /` **`LDA #$10 / STA $0283`**. One write to SWBCNT, at file+`$000C`, of **`$10`,
D4 alone** — the same bit the 2004 post reports Air-Sea Battle setting. **Zero** writes to SWACNT
anywhere in the 4K image.

No annotated disassembly was opened for this. A ROM image carries no interpretation; deriving meaning
from it is the skill, and that is where our clean-room line sits.

What the write is *for* stays 📖, and is now a sharper question than the line started with: the 2004
thread's *"does anyone know if SWBCNT has any use?"* went unanswered, and the same post reports Combat's
own comment claiming the write stops joystick response **when it does not**. Two commercial titles do it;
nobody has said why.

The attribution dispute this entry recorded an hour ago also dissolves — the distillation measured it
themselves rather than arguing about who had measured it before, which is the right way to end that kind
of disagreement. Found and measured by helper-1; re-derived here from the same bytes.

### Added — two assembler traps from the list, checked and NOT applicable here (2026-09-04)

Both were reported as costing someone a day, and both are version-specific. Ours is not that version,
and saying so is the point: a trap that cannot reach us should not become a gate.

**1997 — an assembler accepted `jmp (abs,x)`**, a 65C02 addressing mode the 6507 does not have, so the
program assembled and misbehaved. DASM 2.20.14.1 here **rejects it outright**: *"error: Illegal
Addressing mode 'jmp ($F800,x)'"*, fatal, source not resolvable.

**2003 — DASM silently promoted `LDX zp,Y` to `abs,Y`** (+1 cycle, no error) while *rejecting*
`LAX zp,Y`, so a documented and an undocumented instruction with the same addressing mode were handled
differently, and half a day went into "why does this not assemble". Ours assembles `lax tbl,y` to
**`b7 80`** — two bytes, no promotion, no error — matching `ldx tbl,y` at `b6 80`.

Recorded in `known-traps.md` section D with the fixtures, because "checked and not applicable" is worth
as much as a trap: it stops the next reader adding a check for something that cannot happen, and says
what to re-run if the toolchain moves.

Relevant to yesterday's `multicolor48` note, which recommends `LAX` to fuse `LDA`+`TAX`: the objection
raised on the list against `LAX` addressing does not apply to this toolchain.

From the mailing-list distillation (helper-2), who raised both and could not test either.

### Changed — the question that cost an afternoon has a tool, documented in the file loaded at launch (2026-09-04)

The expensive question yesterday was *"which copy of the playfield is this probe standing on?"*, and it
was answered by writing a calibration band into a ROM. `decompose_row` answers it directly. Its own
description reads *"WHICH TIA OBJECT drew each pixel … the attribution sibling of read_row"*, and
**`CLAUDE.md` carries a worked example of the same shape** — *"`decompose_row` shows P0 occupying clock
2..9"* — in the file that is loaded into every session at launch.

Neither fixture written yesterday calls it. Zero occurrences in `litmus_pf0_reflect.asm`,
`pf0reflect_test.go`, `flickerattrib_test.go`.

The mechanism is in the tool's own sentence: the question was about **attribution** and the tool reached
for was the one about **colour** (`read_row`), whose sibling this is. One call would have shown a
quad-width `P1` starting at clock 0 and ended the confusion in a minute.

**This is the fifth of these in two days and the first of its kind.** The other four were facts sitting
inside code — `calibrate`'s exclusion of saturated points, `ramtrace`'s stack low-water mark and its
`collisions_seen`, `sprite-placement`'s rule 12. This one was in the always-loaded contract file, with
an example. **Writing it down where it is always read is not sufficient either.**

`invisible-probe.md` now says so at the top, next to the placement pointer added yesterday for the same
reason.

A smaller instance of the same thing while checking this: the quote above was searched for by hand and
missed, because a backtick was dropped when copying it out of a message. **Today's own rule — extract
verbatim mechanically, never type it — broken while verifying a report about not using the tools we
have.**

Found by the mailing-list distillation (helper-3), who could not run the tool themselves and named the
real failure as not having said "call `decompose_row`" out loud.

### Changed — the DDR line's counterexamples may be Combat and Air-Sea Battle, and we cannot check (2026-09-04)

Yesterday "rarely game-relevant" was withdrawn for the port DDRs on the strength of one line in
`casebook.md`. The list goes further. A 2004 post quotes the Stella Programmer's Guide saying SWCHB
*"is hardwired to be input only"*, asks *"then why would they have the SWBCNT register?"*, and reports
**Air-Sea Battle setting D4 of SWCHB as output** and **Combat doing the same with its comments
mislabelled** — *"it says this stops the response from the joysticks but it doesn't"*.

If that holds, the dismissal was wrong about two of the best-known cartridges there are.

**Recorded as unverified, with the reason.** Neither ROM is in `reference/`: the only `Combat.bin` here
is a twelve-byte fragment. Reading a commercial ROM's *bytes* would sit inside our clean-room line —
bytes carry no interpretation, and deriving meaning from them is the skill — but we do not have them.
The 2004 thread also records the question going **unanswered**, so the list does not settle it either.

Also noted for the record: the distillation attributed a Combat SWBCNT verification to this session,
and **there is no such measurement**. Checked before repeating it, which is why the entry says
"unverified" rather than citing a check that did not happen.

Separately, and usefully negative: **the NMOS-decimal rule (only C is valid after ADC/SBC) is not in the
corpus.** Three differently-shaped queries, the tightest returning nothing and the other two returning
hits that are all other senses. `CLAUDE.md` files that rule under "constants you must never get wrong",
and the mailing list will not corroborate it — so it stays where it is, resting on our own measurement
of the flags.

Found by the mailing-list distillation (helper-1).

### Changed — `collide_all`'s asserts are not vacuous, and the tool that said so was already there (2026-09-04)

Yesterday's note said `litmus_collide_all` cannot serve as a sensor, because every object overlaps every
other and its scenario asserts fifteen `== 1` with no `== 0`. True, and the wording was loose enough to
read as "the fifteen asserts prove nothing".

`ramtrace activity` prints **"collisions that occurred"**, and for this ROM it prints all fifteen pairs.
**Every assert really fires.** The gap is narrower and sharper than it was written: it is precisely the
missing `== 0` case, not the fifteen `== 1`s.

The same command distinguishes fixtures usefully — `litmus_flicker_attrib` fires only `p0_pf`, `bullets`
only `m0_p1` — so it also answers *which pairs a fixture never exercises*. **An assert on a pair that
never fires passes without measuring anything**, which is the vacuous case the original note was
reaching for and had put in the wrong place.

That makes three in two days where a tool was already reporting the answer: `calibrate`'s exclusion of
saturated points, `ramtrace`'s stack low-water mark, and now this. The audit that surfaced it was a
deliberate sweep for the pattern — every `json:` tag the Go code exports, filtered to those no document
mentions, **then re-checked by concept rather than by name**, which dropped three false hits that were
described in prose under other words.

Found by the mailing-list distillation (helper-2), who added that fourth step after watching a
name-only search produce a wrong zero earlier the same day.

### Changed — the `SEI` correction reached one corpus note this morning and five more were waiting (2026-09-03)

This morning's fix removed *"Needs `SEI` (no IRQ)"* from `missiles-bullets.md`, and the distillation
pointed at one reference note still quoting the old line. It was pointed at, so it was fixed. **The
search was never run.**

Run now: **seven occurrences across five notes**, and several of them are notes that had *found the
error* — *"`SEI` does nothing on the 2600 (no interrupt line, and `BRK` cannot be masked)"*, *"this
disagrees with harness head-on"*, *"the 6507 has no IRQ pin, and yet…"*. **The corpus had flagged this
repeatedly and harness was corrected this morning.** With harness fixed and the notes untouched, every
one of them now reads as a live disagreement that no longer exists.

The quotes are left in place — they are accurate records of what this repository said at the time — and
each carries a note that the line has since been corrected, with the current wording. Same treatment as
the half-stale criticism in `scanline-counts` earlier today: strike the resolved half, keep the rest.

**The rule this breaks is one I wrote into my own memory this morning**, after the first instance:
*when you correct a claim in harness, shoot `rg -F '<the old verbatim>' reference/` in the same turn.*
I fixed what I was shown and did not run the search — which is how the count went from one to seven.

Separately, and in the other direction: the `SEI` finding gains **external corroboration**, which is the
fourth kind of relationship a source can have with us. Erik Mooney, 1999: *"There are no interrupts on
the 2600."* In 2001 someone asks whether the `SEI` at the top of most games is therefore unnecessary,
and Eckhard Stolberg answers by drawing exactly the distinction we drew from the engine — the RIOT's own
timer-interrupt flag against anything the 6507 can see. The derivation here was independent and landed
on the same split.

Found by the mailing-list distillation (helper-3), who also declined to open the duplicate they spotted
because it belongs to another session's thread key, and named it instead.

### Changed — "hoping the two never meet" now has numbers, measured with a tool we already had (2026-09-03)

The stack/variable convention was stated as *"stack from $FF down, variables from $80 up, hoping the two
never meet"*. `internal/ramtrace`'s activity report prints a stack low-water mark and the observed SP
range, and **had never been run for this question**. Run now over our own technique ROMs:

    $FD (2 bytes)   bullets, flicker_multiplex, two_line_kernel, score6, paddle_demo, procgen_demo
    $FB (4 bytes)   game_states, dyn_multisprite
    $F5 (10 bytes)  rts_dispatch

`rts_dispatch` is the outlier by construction: it pushes return addresses as its dispatch mechanism, so
its stack use *is* the technique. Everything else is two or four. The list agrees from the other side —
**Space Instigators uses no stack, Fade Out and Marble Craze two bytes**, with 6-8 offered as enough for
two or three levels of nesting 〔stella 2004〕. A variable at `$F8` is therefore safe in every kernel here
except the one that dispatches through the stack, **which is what a convention phrased as "hoping"
cannot tell you.**

Also added: the *mechanism* behind "$0180–$01FF is why the stack works", which the line stated as a
conclusion. The 6507's stack pointer is **eight bits** against a thirteen-bit address bus, so the
processor supplies `$01` as the upper bits on every stack access — no software choice involved — and the
PIA being mapped into both pages is what makes the two addresses the same memory 〔stella 1999-08〕.

And marked 📖: the reverse trick, deliberately using the stack region as scratch, which the list offers
with its own caveat. `known-traps.md` covers a variable at `$FF` being clobbered by a `JSR` push and says
nothing about going the other way.

Found by the mailing-list distillation (helper-1), whose point was the sharp one: **the numbers were
missing because nobody ran the tool, not because the tool was missing.** A 2004 post proposes measuring
exactly this by watching the stack pointer in a tracer; we have had that since `ramtrace`.

### Added — a benefit `subpixel-velocity.md` had and never claimed (2026-09-03)

The Stella Programmer's Guide sentence about PAL conversion is normally quoted as an argument for 8.8
fixed-point position. Its condition is wider than that: *"if the NTSC version is designed with 2 byte
fractional addition techniques **(or anything not based on frames per second)**"*. A DDA carries its
fraction in the velocity rather than the position and converts the same way — swap the increment table.

So the argument usually made against this page's approach is an argument for it, and **the page had
never mentioned PAL, NTSC or a refresh rate at all** — the benefit was there and unwritten. It is now,
with the factor computed from our own constants: NTSC `15734.26/262` = 60.0544 Hz, PAL `15625.00/312`
= 50.0801 Hz, so a PAL increment must be **83.39%** of the NTSC one, 0.06 points off the nominal 50/60.

Worth the precision, because the thread shows what happens without it. The author wrote *"just ensure
the NTSC m to be ~80%"* and then asked, parenthetically, *"can someone provide the correct value?
83,4%?"*. **The confident number was 3.4 points out; the hesitant one was right to two decimal places.**

Also carried across as ⬜: that the zero flag can replace the carry for 0-2 px/frame and the overflow
flag for 0-4. The source gives no mechanism and the distillation did not reproduce it.

From the mailing-list distillation (helper-3), who noted this is the rarer direction — a source that
**strengthens** a choice already made here rather than correcting it — and that no query would have
found it, because the answer is in a subordinate clause of a sentence quoted for something else.

### Fixed — an open item in `litmus-results.md` had an answer waiting in a 2004 post (2026-09-03)

The file's "Unverified (optional, low priority)" list asked for *"an exact explanation of the boundary
artifact for deep-HBLANK strobes (DELAY 0–2)"*. Eric Ball gave it on the list in 2004: a horizontal
positioning routine *"doesn't work for X less than or equal to 7"* because **`RESP0` completes at cycle
21, which is still inside HBLANK**. Two `NOP`s at the head fix it. The table he was correcting had been
published in 1998 and stood wrong for six years — **the game it came from never placed anything in that
range**, so nobody hit it.

Marked answered-from-a-source rather than resolved: we have not measured it.

The part worth keeping is what looking for it turned up. **Our own tooling already routes around this
boundary without naming it**: `internal/calibrate`'s `Fit` least-squares *"only the longest contiguous
run … (excluding saturated points)"*, saturation being *"the strobe pinning to the left edge outside its
valid range"*. The tool knows; no document said so. **A tool that avoids a boundary is not a boundary
that has been written down**, which is exactly how a wrong table survived six years.

Also: `zone-multiplexing.md` gains a **fourth** way past two players on a line — use a missile as a
*character*, varying its width and colour and `HMOVE`ing it every line so it draws a shape rather than a
dot 〔stella 1997, Erik Mooney〕. That line listed two escapes this morning and now lists four.

From the mailing-list distillation (helper-2), who measured three states for each claim — what the
litmus covers, what the code does, what the docs say — and separately withdrew one of their own earlier
findings: their "PAL odd-scanline failure mode is absent" rested on a query requiring the words
`black|b/w|monochrome`, and harness says it as *"PAL color circuitry fails on odd scanlines"*. The zero
was true of the query and false as a conclusion.

### Changed — seven technique pages now point at the placement table, chosen by measurement (2026-09-03)

Thirty-six technique pages touch a placement register and three cited `sprite-placement.md` — two of
those three being the index and the roadmap, so **one actual technique**. Adding the pointer to all
thirty-six would have been noise; the question is which pages have a **fixture that can actually be
bitten**.

The criterion was measured rather than chosen: a fixture qualifies if it strobes `RESxx` **and** writes
a `NUSIZ` that makes copies (low bits 1,2,3,4,6), widens (5,7 or the size field), or is table-driven.
Strobing alone gives sixteen of twenty-two — rule 12 is about a *copy* passing clock 160, so without
copies or width it cannot fire. That leaves **seven**: `nusiz-shaping` (table-driven, sweeps every mode,
the most exposed), `rts-dispatch` (`$06`, three copies at the wide spacing, so a base past ~96 goes over
the edge), and `bitmap48`, `multicolor48`, `score-kernel`, `text12`, `text24` (all `$03`, rightmost copy
at base+32).

The placement litmus itself writes `$20`/`$26` — missile width — which is why width joined copies in the
criterion, and is the same shape as the quad-width probe that started this.

Two measurement notes, both about the same defect. The distillation's first pass disagreed with its own
`grep` on `text24`, and opening the file settled it: a fixed three-line window before `sta NUSIZ` was
picking up an earlier `lda #0` shared by three stores. **Checking their `litmus_sprite_place` claim here,
a two-line window missed the `$20`/`$26` writes entirely** — the same defect in the opposite direction,
found while verifying the report of it.

Selected by the mailing-list distillation (helper-1), who declined all three criteria offered and
derived one.

### Changed — the wrap at clock 160 was measured thirteen days ago, and we derived it again anyway (2026-09-03)

Most of an afternoon went into finding that a quad-width probe parked near the right edge was also
drawing at the left, because the TIA's horizontal counter wraps at 160. It is rule 12 in
`sprite-placement.md`, measured on 2026-08-21, fixed by `litmus_restrobe_objects`, graded by
`internal/emu/restrobeobjects_test.go`, and cross-checked against Stella 37/37.

**That file's own preamblepredicted this:** *"none of it said where they land, so the numbers were
re-derived from throwaway probes twice in one session. This file is the fix."* Today was the third
time, and the file was **named in this session's startup context** — `STATUS.md` lists it as technique
#35, with its litmus and its Stella check.

So the lesson is not "write it down". It was written down, verified, locked in CI and announced at
launch. **Knowing a table exists is not the same as reaching for it**, and what decided it was subject:
the work was about collision latches, so a document about *placement* was never opened. That is the
fourth instance this week of a tool being missed because of what was being built just before.

Both ends now carry it. The preamble records the third derivation and asks to be cited from any
technique whose fixture places an object; rule 12 carries the concrete failure, because a rule stated
abstractly reads as somebody else's problem — *a quad-width P1 at x≈150 wrapped to 0-22 and collided
with a playfield copy at the other end, which read as "the probe is on both copies"*. And
`invisible-probe.md`, whose fixture caused it, opens by pointing at the table.

Found by the mailing-list distillation (helper-3), who went looking for something else and recognised
our afternoon in a row of a table they had not written.

 (2026-09-03)

Which device a game can use is decided by where the cost of reading it lands, not by preference. The
joystick is 40-44 cycles **once a frame**, in VBLANK, where there is slack. A paddle is 8-16 cycles
**per visible line**, in the kernel, where there is not — over 192 lines that is 35× to 77× the
joystick's total, and 7.7% to 15.4% of the whole frame, spent on the side that has nothing to spare.

That is why the list's answer to "why is there no proportional trackball game" is a cycle count rather
than a taste: *"the 2600 doesn't have much CPU time left over to read controller ports … that's why
trackballs have a joystick emulation mode"* 〔stella 1997, Glenn Saunders〕. Keypads are worse again and
stay ⬜ — the ledger has `keypad-read-delay` and `reading-trackball` as titles only, so the table can be
finished by mining them.

`①lint` and `②techniques/litmus` were both empty for this: choosing an input device was in neither our
rules nor our hardware verification, and `paddle.md` was the only input technique we had.

**Two measurement notes came with it, both worth more than the page.** The engine's instruction table
**does not distinguish zero-page from absolute by mode name** — `LDA` appears twice as `absolute`, at 2
bytes/3 cycles and 3 bytes/4 cycles — so a query by mode alone silently counts a zero-page access as
four cycles. (`branch-always`'s figures were addressed by opcode and are unaffected; checked.) And
`paddle.md`'s "~12 cycles" does not match 8 or 16 from that table, which is now recorded on the line as
⬜ rather than quietly corrected, because neither number has been run.

Also added, to `zone-multiplexing.md`: a **third** way past two players on a line, next to flicker and
the missile/ball/PF — interleave both players at a wide `NUSIZ` so each sits in the other's gaps, six
figures and no flicker, paid for in width instead of in frames 〔stella 2000, Glenn Saunders〕. The line
had listed two escapes and there are three.

From the mailing-list distillation (helper-2).

### Added — the TIA manual's "one line after" is a consequence written as a mechanism (2026-09-03)

The manual says a VDELx object's second graphics bit is loaded *"one line after the first was loaded
from the data bus"*. A 2005 post calls that **"baloney"** — the registers *"look for writes to GRPx;
they don't monitor the scanline counter"*.

**Both are right, and that is the interesting part.** The same post continues: *"when that was written,
you were expected to write to GRP0 and GRP1 every other line during your kernel (as Combat does). If you
actually do that, all three VDEL registers behave exactly as advertised."* The manual describes the
consequence of an idiom that was universal when it was written, in the words of a mechanism. Do anything
else and the delay follows your writes, not your lines.

Our engine is on the writes side, read from source: `ball.go`'s `setEnableDelay` is called from exactly
two places, both `case cpubus.GRP1`, and nothing in that file reads a scanline counter — its two
mentions of the word are drawing comments. No fixture yet, and the entry says so: **source semantics are
not a measurement**, the same line held today for `BIT CXxx` and for the ball-width disagreement.

This is a third kind, and worth naming because the first two are being counted: **the source is wrong**
(ASR's stability, skipdraw's "constant 18"), **we misread the source** (the interlace rule, the
indirect-jump attribution, the 3CC derivation), and now **the source is right under a premise it does not
state**. The test that separates the third from the first is whether the person denying it goes on to
say when it does hold. Mooney does, so the manual is not to be discarded — it is to be quoted with its
premise attached.

Found by the mailing-list distillation (helper-1), who was asked to check whether harness took a side and
came back with a better answer than the question assumed. They also confirmed, from the same `case`, our
existing claim that a GRP1 write copies both P0's graphics and the ball's enable — a line they had
previously flagged as unsupported by its 1998 quote.

### Added — `invisible-probe.md`: the hardware computes a hit test if you can hide the instrument (2026-09-03)

Park a missile or the ball on a region the hardware has no register for, read the collision latch, and
hide the object so the player never sees it. Described on the list in 1998, with three ways to hide it;
**the engine carries the mechanism and the catalogue had nothing.** Searching `docs/techniques/` for the
idea returns nothing — five apparent hits are the word "invisible" in unrelated sentences.

The value the page adds over the source is **what each way of hiding costs**, worked out from the
engine's priority chain rather than assumed:

    default             P0 > M0 > P1 > M1 > BL > PF > BG
    CTRLPF D2 set       PF/BL > P0/M0 > P1/M1 > BG

So *"put the ball under the player"* is **free by default** — and its price is `CTRLPF` D2, because with
D2 set the ball rises above both players. **A kernel that uses playfield priority and a kernel that
hides a probe by priority cannot be the same kernel.** Colour-matching a missile to its player is free
too, but the probe reappears over the *other* player, and that missile is no longer available as a
bullet. Setting `COLUPF = COLUBK` hides it everywhere and costs the playfield everywhere.

It also states something the distillation noticed about its own week: **a litmus probe and a game probe
have opposite requirements.** A litmus probe may be visible (nobody is looking at the picture) and must
be exact and calibrated in both directions; a game probe must be invisible and may be approximate.
Which is why the same technique reads as two different ones depending on who wrote it down — and why
`collide_all` asserting fifteen 1s and no 0s is survivable for a game and not for a measurement.

Hiding is not measured here. Written by the mailing-list distillation (helper-3).

### Changed — we measured what a ball-width write does, never when, and a 1997 source disagrees (2026-09-03)

`litmus_ctrlpf` fixes the four ball widths, and searching it for `delay|latch|mid.?line` returns
nothing: **the values are verified and the timing has never been touched.** Gopher2600 applies the
write immediately (`ball.go`: `bs.Size = (value & 0x30) >> 4`). A stella post from 1997 reports the
opposite — a width write not taking effect for about **eighty colour clocks**, and a width changed
part-way through a draw rendering as `X......X` rather than as either width.

Recorded as ⬜ rather than resolved. Neither side has been measured here: the 1997 report was made
against real hardware, ours is a reading of the emulator's source, and reading a source is not a
measurement — the same distinction that kept `BIT CXxx` at 📖 earlier today.

Worth noting why nobody would have suspected it: `design-principles.md`'s register-timing rules cover
PF writes (2-3 colour clocks late) and colour writes (immediate) and **say nothing about CTRLPF at
all**, so the table gives a reader no reason to look. The entry carries a way to settle it — change
the width mid-visible, read back the first x where the drawn run changes, with the same change during
HBLANK as the control.

Found by the mailing-list distillation (helper-2), who measured all three states — what the litmus
covers, what the engine does, what the doc says — before concluding anything, and said plainly that
they cannot tell which side is right.

### Changed — `flicker-multiplexing.md` now says why the list told you not to do this (2026-09-03)

The page had **nothing about collisions** — zero mentions of a collision register or `CXCLR` — while
the standing advice on the list, for twenty-eight years, was that a flickered slot cannot use the
hardware at all:

> Obviously, you can't use the hardware collision registers … it'd just be a check to see if the
> "hot-spot" … is within a rectangular area — Erik Mooney, stella `199811/msg00037`

The reason is this page's own subject: on any frame at most one of a flickered pair is drawn, so a
pair that never share a frame can never latch. `litmus_flicker_attrib` measures the fix, and now the
page says so, with the ordering intact — **the 1998 advice was not wrong, it was practical.** Software
rectangles need no per-frame discipline and survive an author who forgets one; the hardware route is
cheaper and conditional, and the condition is what to write down.

Found by the mailing-list distillation (helper-3), who noticed that a fixture built here today has a
prehistory that says the opposite, and that both statements are true.

**Also measured and left open: the catalogue has no entry for placing an INVISIBLE object as a
collision probe.** A 1998 post describes a missile in the fist and a ball on the hit areas, hidden
three different ways — same colour as the player, priority beneath it, or PF and BG set alike. The
engine carries the mechanism (`video.go`: *"priority 1 (ball is same color as playfield)"*, *"priority 2
(missile 0 is same color as player 0)"*), and searching `docs/techniques/` for the idea returns
nothing — five apparent hits are the word "invisible" in unrelated sentences. Worth a page; not
written yet.

### Added — `blank-a-frame.md`, and a shipped precedent for a tightening we called untried (2026-09-03)

**`blank-a-frame`** is the third answer to a setup that will not fit, next to trimming the work and
spending more lines. It is the one that changes nothing about the kernel, and it applies exactly when
the other two do not: **when the cost is per-event rather than per-frame** — a respawn, a level load, a
state transition. The page is explicit that the danger is not slowness but that **the frame length
changes**, and carries forward what this week measured about detecting that: a scenario reading
`ntsc_frame_lines` samples *one* frame, so a one-off overrun at frame 40 is invisible to an assert at
frame 4. Use it with `frame_lines_stable`. Hardware basis is stated as not measured here — the source
proposes the approach rather than demonstrating it.

**Separately: `multicolor48.md` proposed `LAX` to fuse `LDA`+`TAX` "verify on Gopher2600 first",
and it had shipped in 2000.** Thomas Jentzsch, in Thrust, for the same purpose this line wants it —
*"i'm using `LAX (ptr),Y` to get the data faster into the X-register (5 vs 7 clocks). This gives me the
time to do the color-cycling"* 〔stella `200006/msg00037`〕. His numbers check against our own
instruction table: `LDA (zp),Y` 5 cy / 2 bytes plus `TAX` 2 cy / 1 byte against `LAX (zp),Y` 5 cy /
2 bytes — two cycles and one byte. `LAX` is on our hardware-stable list, unlike `LXA`/`XAA`.

The caution stays; what changed is that the idea is no longer untried, which is different information
from "it should work".

Both from the mailing-list distillation (helper-3), who also reported an error of their own worth
recording: they hand-wrote two thread manifests and got both wrong — and on the second, **the message
count matched while the contents did not**. `sl_extract.py` has a `manifest()` function that exists to
stop exactly that, in a file they had read this session. Their own diagnosis: they were writing prose,
so they filled the manifest in as prose.

### Added — `flicker-collision-attribution.md`, and a hand-written count removed from `branch-always` (2026-09-03)

`litmus_flicker_attrib` settles what the latches do. It does not carry **why the problem exists, what
it costs a shipped game, or the idioms the list converged on**, and those are what the page adds — with
the failure stated by an author who hit it (*"you can swing the pod through all other objects, except
the playfield … I'm not sure if i can fix this"*), the mechanism from the message that explains it, and
both idioms marked ⬜ unverified here.

The exception in that quote is the whole shape: the playfield is not flickered, so PF collisions never
miss, and everything that *is* flickered can pass through everything else that is.

**Separately: `branch-always.md` opened with "The third entry in this catalogue that corrects its own
source", followed by three hand-written items.** Two things wrong with that. Nothing reads the number
(one occurrence, in the file that writes it) — a count with no invalidation mechanism, which is the
failure this repository names at the top of its own `CLAUDE.md`. And it was **already wrong when
written**: the distillation enumerated the family and found five, not three, because two more
corrections had landed without anyone updating a sentence nobody was watching.

It also mixed two families that read identically from inside and should not be added together:
**the source was wrong** (page-cross, the byte economics, the LFSR step direction, ASR's stability,
skipdraw's "constant 18") versus **we misread the source** (the interlace-colour rule, the indirect-jump
attribution, the 3CC derivation). The line now states the fact without counting it and points at
`CHANGELOG.md`, where both kinds are recorded where they were found.

Page written by the mailing-list distillation (helper-1), who also enumerated the two families.

### Fixed — "Irreducible" in `missiles-bullets.md`, contradicted two lines below it (2026-09-03)

The missile-strobe kernel's cost was written as *"20 cy, both missiles, no branches. **Irreducible:** the
two `PLA` (8 cy) are the mandatory SP restore."* The next sentence but one already named the way out —
the X-pin trick that collapses `PLA;PLA` to a two-cycle `TXS` — while framing it as a different
technique from a different file.

It is not a different technique. It is the same one with the line counter in `Y`: compare with `CPY`
instead of `CPX` and `X` stays free to hold `$1E` across the line, so the restore is `TXS`. **`CPY`
appears nowhere in this file or in `two-line-kernel.md`**, which is how an alternative written in terms
of the other index register read as something else entirely.

The shape is **Thomas Jentzsch, stella 1999-11**, whose own comment reads `php ;3      got this trick
from Combat` — the same game this file derives from in-house, twenty-seven years apart. That is a
provenance addition, not a new technique: an independent, dated, external corroboration of a
derivation, which is the strongest form a `— in-house:` line can acquire.

What is corrected is the word. The cycle saving is arithmetic on paper and is not claimed as measured;
what is measured is that "irreducible" was false, since our own file names the escape.

Found by the mailing-list distillation (helper-3), who also stated the fair version: had Combat's
kernel needed `Y` for something else, `X` would have been the only register left, so the derivation is
sound and only the word overreached.

### Added — `litmus_pf0_reflect`: two PF0 writes in one line, and where the window ends (2026-09-03)

The audit line held two claims under one mark: *"asymmetric PF under reflection via double PF0 rewrite
per line is real-game practice"*. Whether games do it is a fact about someone else's source. Whether it
works, and where its boundaries are, is ours. Split; the second half is now measured.

**Most of the work was in the instrument.** Four calibration points show the probes respond to PF0 in
both directions — and cannot say *which* copy a probe is on, because they set PF0 for the whole line,
so both copies always agree and a probe on either returns the same 1,0,1,0. That is the failure this
week keeps naming, inside the fixture built to avoid it. Two more points (E and F) write in the blind
gap between the copies so the copies disagree, and the probe's answer names where it is standing.

It mattered immediately. A first version gave P1 quad width to be sure of reaching the right copy at
pixels 144-159; **it reached the copy at the other end too, because the TIA's horizontal counter wraps
at 160** — a 32-px object at 151 draws 151-159 and then 0-22, straight across the left copy. E and F
read 1,1. The earlier sweep had been tracking the *left* boundary at cy ~27.7 while the header claimed
the right one.

**The window is bounded on the right by the scanline itself.** The right copy is drawn at cy ~70.7-75.7
and the line ends at 76, so there is no room to place a store after it: the last usable point lands
*inside* the copy and splits it old|new. The measured column is `0 0 0 0 0 0 1` over seven five-cycle
steps, and the step being at the last index is the result, not a defect. An earlier design swept cy
40-55 — the blind gap — and would have found no step at all.

Also measured, and stated narrowly on purpose: **an `sta HMCLR` about two cycles after `sta HMOVE`
killed the move.** P1 needed a seven-pixel fine adjust, which div-15 cannot give (it quantises to 15),
and the HMOVE did nothing until HMCLR was moved to the next line.

**"Same line" is not the rule, and saying so would contradict a fixture already in CI.**
`litmus_hmxx_freeze` puts `sta HMCLR` about fifty cycles after its `sta HMOVE`, on the same line, and
`scenarios/hmxx_freeze.json` pins the player moving +8 a frame right through it. The plausible reading
is the ✅ rule this file already carries — **do not write HMxx within 24 cycles of HMOVE** — with the
addition that **HMCLR counts as an HMxx write**, since it clears them all. That is a reading, not a
measurement: our case changed two variables at once (two cycles instead of fifty, and HMCLR instead of
a single-register rewrite), so which one did the work is not separated. Two small bands would separate
them and neither exists yet.

The scenario checks `ntsc_frame_lines == 262` **and** `frame_lines_stable`, because they catch different
things: this ROM ran at a rock-steady 251 lines for several iterations, which the stability check passes
happily. Negative controls, both fired: widening the probe makes E,F read 1,1; removing the HMOVE makes
the step disappear.

Designed by the mailing-list distillation (helper-3), who also found the two defects in their own design
once it met the emulator — a calibration band that could not discriminate, and a sweep range that was
impossible inside a line.

### Changed — the new litmus is queued for a Stella capture, because the gate caught that it was not (2026-09-03)

Pushing `litmus_flicker_attrib` was blocked by the pre-push run, and correctly: the Stella oracle
reported `1 corpus ROM(s) have no Stella capture and are not queued`. Adding a ROM to the corpus adds
it to what the oracle is expected to cover, and I had not done the second half.

Queued rather than captured, which is the path the queue exists for — capturing drives Stella's GUI
and takes over the screen for about thirteen seconds per ROM, and the machine is in use. The queue is
not an exemption: every line prints on every run and the test fails once the queue outgrows its cap.

### Changed — `litmus_collide_all` cannot be used as a sensor, and now says so (2026-09-03)

Found while trying to *use* it: the ROM overlaps every object with every other, so all fifteen pairs
read true, and `scenarios/collide_all.json` asserts exactly that — **fifteen asserts, all `== 1`, not
one `== 0`.**

That pins the bit assignment, which is what it was written for. It does not distinguish *"each field
reports its own pair"* from *"any overlap sets every field"*, because nothing in the fixture is ever
apart. So anyone reading a collision field as a **sensor** — "is P0 over the playfield right now?" —
is relying on a direction nothing here measures: **no fixture puts two objects apart and requires the
field to read 0.**

Recorded in the ROM's own header rather than fixed, because fixing it is a separate ROM. Until then
the instruction is the one this week keeps arriving at: establish the negative in your own fixture
before relying on it, the way `litmus_flicker_attrib`'s group 1 establishes frame-crossing stickiness
before its control depends on it.

Found by the mailing-list distillation (helper-3), who needed exactly the unmeasured direction and
checked before assuming it. Comment-only; binary byte-identical to HEAD.

### Added — what actually makes a code path change a frame's length (2026-09-03)

Building `litmus_flicker_attrib` cost three attempts at a constant frame, so the lesson goes in
`known-traps.md` section A where the next author will hit it.

I first generalised it as "a routine whose length varies cannot share a line". **That is too broad**,
and the distillation produced the counterexample from our own tree: `litmus_pf_async` has bands seven
cycles apart (`cyc 40` against `cyc 33`) and holds 262 on every frame. Re-measured here — 262 across
nineteen frames, one 35-line boot frame aside.

The accurate statement is narrower. Only two things make a path change a frame's length: **crossing a
line boundary**, or **exceeding 76 cycles**. A `jsr` started part-way through a line hits the first,
because whichever branch it takes decides whether it spills — so the frame grows by the *case*, not by
the code. Bracketing the call with WSYNC on both sides fixes that, and is still not always enough:
with brackets in place the ROM rendered 263 in three groups and 264 in the fourth, because one
routine's group-2 path was longer and spilled anyway. **A routine whose length varies by case needs a
line of its own.**

The detection side is the same lesson as elsewhere this week: `ntsc_frame_lines` samples one frame and
passed both broken versions. `frame_lines_stable` is the ∀-over-frames check and catches them.

### Added — `litmus_flicker_attrib`: a flickered slot's collision latch belongs to the frame you read it (2026-09-03)

`fundamentals-audit` called this "a verifiable pattern **once we do flicker**". We have had flicker since
technique #10, and `flicker_multiplex.asm` touches no collision register at all — **the condition had been
met and the sentence had not noticed.** A different kind of stale than a wrong number, and one nothing
here was looking for.

The claim worth locking is not "a collision latches" (`litmus_cxclr` has that). It is that when two
objects share a slot on alternate frames, the latch you read belongs to whichever was drawn **that**
frame — and that this holds *only because* CXCLR runs every frame. Proving the "only because" needs a
control where CXCLR does not run, and that control's expected result depends on a latch surviving a frame
boundary. **Nothing had measured that**: `litmus_cxclr` takes all three snapshots inside one frame and
strobes CXCLR every frame, so a latch never gets the chance there. So group 1 of this ROM measures it
first, and the control rests on our own measurement instead of an assumption.

    group 1 ($90-$93)  1 1 1 0   a latch survives into the next frame; only CXCLR clears it
    group 2 ($A0/$B0)  1,0,1,0…  latch and the ROM's own record of what it drew agree cell for cell
    group 3 ($C0)      1,1,1,1…  without CXCLR, attribution is lost
    group 4 ($D0)      0,1,0,1…  phase inverted, so frame parity is not the cause

Two things the fixture does on purpose. **Collision is switched by blanking the playfield, not by moving
P0** — `litmus_cxclr`'s header records losing a day to a positioning bug, and its lesson is to take the
positioning out of the answer. **Every stored byte is normalised to 0 or 1**: raw CXP0FB would pin the
preceding instruction too, which is why that ROM's scenario reads 130 and 2 rather than 128 and 0.

**The frame length took three attempts and is measured, not derived.** A `jsr` that starts mid-line
leaks its own branch structure into the scanline count: the first version rendered 261 lines in some
frames and 262 in others, and bracketing the calls with WSYNC on *both* sides was not enough while one
subroutine's group-2 path was longer than the others — group 2's frames stayed a line long until the
per-frame recording was split into two subroutines, each called from its own line. The scenario therefore
uses `frame_lines_stable` (∀ over 34 frames) rather than `ntsc_frame_lines`, which samples one frame and
would have passed the broken versions.

Negative controls, both fired and both recovered: removing CXCLR from group 2 makes it read
`1 1 1 1 1 1 1 1`, identical to its own control; removing the phase inversion from group 4 makes it match
group 2. Designed by the mailing-list distillation (helper-3); built, measured and graded here.

### Changed — `BIT CXxx` returns three predicates, not two, and the mark stays 📖 anyway (2026-09-03)

The audit line said one `BIT CXxx` yields two collision pairs via N and V. True, and incomplete in a way
that matters: **read it with `A = $C0`, not `$FF`.**

Only D7/D6 of a collision register are driven. Every other bit is the last value the CPU put on the bus
(`memory.go`: `data |= mem.LastCPUData & ^mem.DataBusDriven`) — which is exactly why
`scenarios/litmus_cxclr.json` pins 130 and 2 rather than 128 and 0. So with `A = $FF`, Z answers "is the
whole byte zero" and **moves when the preceding instruction changes, with no change in TIA behaviour at
all**. With `A = $C0` the residue is masked and Z becomes a third useful predicate — "neither pair
collided" — so one `BIT` yields three tests. With `A = $00` it yields nothing: Z is always 1.

The flag semantics come from the engine's source (`cpu.go` `case instructions.BIT` loads M, sets Sign and
Overflow from it, and only then ANDs A; `data.go` masks `0x80` and `0x40`), which settles that N and V do
not depend on A.

**The mark stays 📖.** This file's own Legend defines ✅ as *measured by our litmus ROMs, locked in CI*,
and reading engine source is not a litmus — it establishes semantics, not measurement. Raising the mark
would need a ROM that sweeps `A` and shows N and V do not move, because a single value of `A` cannot tell
"independent of A" apart from "happened to agree" on that one value.

Found by the mailing-list distillation (helper-3), who declined to promote their own finding on the
Legend's own terms.

### Changed — the 23 silent always-taken branches now carry their invariant, on the seed (2026-09-03)

`branch-always` recorded that only 5 of our 28 always-taken branches named their invariant. The other
23 now do, and the note goes **on the seed** rather than on the branch:

    lda #0          ; A=0 here is also the beq's condition below - change one, check both

That placement is the point. All five pre-existing notes sit on the *branch* — the line that is safe.
The person who breaks this edits the *seed* and has no reason to look two lines down, so the warning
was on the side nobody reads.

The one site that already had it — `litmus_6502`, whose seed line says `Z=0` — is pointed at rather
than duplicated. It is also the evidence: of 28 sites, the single one where someone wrote the note on
the seed is the single one that reads correctly today.

No label names in the text, deliberately: a label is one more thing that can go stale, and the branch
is one or two lines below with nothing ambiguous between. Comment-only — all 14 affected ROMs assemble
byte-identical to their previous binaries, checked pair by pair. Six gates pass.

Written by the mailing-list distillation (helper-2), who supplied each line's verbatim text alongside
its number so the patch could be matched on content — the numbers had already moved once by then.

### Added — the VSYNC "≥2 lines" threshold is our own setting, and its default is the answer (2026-09-03)

`television.go` accepts a frame as synced when `vsync.activeScanlineCount >= env.Prefs.TV.VSYNCscanlines`,
and `preferences/television.go` `SetDefaults` sets that to **2**. Nothing in this repo changes it.

So a litmus that sweeps 0..4 VSYNC lines and reports "1 fails, 2 passes" has read back the number it was
given. **This is the sharper twin of the SuperChip SARA entry**: there the default switches a feature
*off*, so a green result looks odd enough to investigate; here the default **agrees with the literature**,
so the same worthless green looks like corroboration.

`fundamentals-audit.md` now splits the item by what is ours to measure — the procedure's shape is, the
threshold is not — and `known-traps.md` section E carries the trap. What a litmus can honestly show is
that a step exists and that there is only one, with a Go-side control that moves `VSYNCscanlines` to 3
and watches the step move, so the test states its own input.

Found by the mailing-list distillation (helper-3), who reached "this cannot be settled here" rather than
designing the ROM they were asked for. They also noticed that `television.go` sets `failedVSYNC = true`
on the branch where VSYNC *succeeded*, against its own comment eleven lines up — **and then established
that the field is never read** (six occurrences: a comment, the declaration, two resets, one assignment,
one comment), so nothing behaves differently and it is reported rather than touched. Upstream code, not
ours.

### Fixed — a line that said "pending our own measurement" while sitting on the measurement (2026-09-03)

`fundamentals-audit.md` listed the HMOVE comb / late-HMOVE behaviour as adopted from Towers'
notes "pending our own measurement". We measured it: `litmus_hmove_side`, two rows in
`verified-coverage.md`, a scenario, a golden and `internal/emu/hmoveside_test.go`. What is still
pending is narrower — the Stella cross-check — and that is already recorded where the measurement
was made, so nothing is lost by correcting the line.

Also annotates `litmus_cxclr.asm`, whose scenario pins `130 / 130 / 2` where a reader expects
`128 / 128 / 0`. CXP0FB drives only D7 and D6; Gopher2600 fills the floating pins from the last
value the CPU put on the bus (`memory.go`: `data |= mem.LastCPUData & ^mem.DataBusDriven`), and the
last such value is the `2` from the `lda #2 / sta VBLANK` on the next line. **So that scenario pins
the collision latch and the instruction that last drove the bus, together** — reordering those
instructions harmlessly, changing nothing the TIA does, fails it. Not a bug, but not readable from
the values either. Comment-only; the assembled binary is byte-identical to HEAD.

Found by the mailing-list distillation (helper-3), who also correctly declined to reclassify a
neighbouring line: `:129` says a collision pattern is verifiable "once we do flicker", and we do
have flicker — but our flicker ROM touches no collision register, so the claim is right and only
its stated reason is wrong. The mark stays.

### Added — `branch-always` / simulated BRA, the third catalogue entry that corrects its own source (2026-09-03)

A conditional branch whose flag the preceding instruction has already fixed, replacing a 3-byte
`JMP abs`. Used **28 times across 17 files** here and absent from the catalogue until now.

**The source states a one-byte saving without its condition, and the condition does all the work.**
Measured against this repository's own instruction table
(`Gopher2600/hardware/cpu/instructions/definitions.json`):

    seed needed anyway → branch only     2 bytes / 3 cy    −1 byte, ±0 cycles   ✓
    seed added: `lda #imm` + branch      4 bytes / 5 cy    +1 byte, +2 cycles   ✗
    seed added: `lsr` + branch           3 bytes / 5 cy    ±0 bytes, +2 cycles  ✗

So there is no configuration in which *adding* a seed wins — with `lsr` it costs the same bytes as the
`jmp` it replaced while running two cycles slower. Of our 28 uses, **25 free-ride on a flag that
already existed** and the other 3 are litmus ROMs built to measure this shape. **None of the 25 could
use the source's cheaper `lsr` seed**, because in every one the accumulator holds the value being
stored and `lsr` would destroy it.

Page-crossing costs the branch a cycle, and of the 28 sites **exactly one crosses** — `litmus_6502:128`,
where the crossing is the thing being measured. The rule and the corpus are both stated, because
either alone is false.

**Only 5 of the 28 name the invariant in a comment.** The other 23 are live, undocumented hazards:
the branch's unconditionality is a property of the instruction *above* it, so changing `lda #0` to
`lda mask` silently makes it conditional and moves the line's cycle count with the data.

Written by the mailing-list distillation (helper-2), who derived the classifier from this repository's
CPU rather than asserting it — the operators that may sit between seed and branch are computed from
`cpu.go`, and two independent derivations (Z-writers, N-writers) agree on the same 43. Their own three
corrections are kept in the working notes: a file count written without counting, a parser defect that
inflated 28 to 45, and this entry's economics, first reported as a draw because only bytes were
compared.

### Fixed — promoting the PAL rule left the claim in two places; now it is in one (2026-09-03)

`899069c` gave the PAL even-scanline rule its own name and left the original parenthetical in the
scrolling-background bullet, on the reasoning that it read correctly where it was. That was wrong for a
reason this repository has been bitten by before and says so at the top of its own `CLAUDE.md`: **two
copies of one claim is not redundancy, it is a second thing to keep in sync.**

It also broke the fix it was meant to enable. The distillation could not resolve 21 references because
the claim now had two homes and no way to choose between them — and separately measured that the two
places where addressing-by-rule-name fails in this file (`:178`/`:187`) are **also** a duplicated name.
Duplication is the failure mode of that method, and I had just created one.

The parenthetical now points at the rule instead of restating it, and the rule's name resolves to
exactly one line (`rg -F 'PAL frames must have an even scanline count'` → 1 hit). The old wording
"PAL must be even" is gone from the file, which is deliberate: references quoting it were **already**
all broken — the distillation measured every judgeable one of the 73 in that cluster as pointing at the
wrong line, with zero pointing at what the line holds now.

Found by helper-1, who was blocked by it and said so rather than picking one.

### Fixed — "rarely game-relevant" was true of our ROMs and not of the games (2026-09-03)

`fundamentals-audit.md` dismissed the SWACNT/SWBCNT data-direction registers as "rarely
game-relevant". **The counterexample was already in this repository, one file away:**
`docs/casebook.md:98` reads a commercial title as *"gate joysticks through the port DDR"*.

The measurable statement underneath is narrower and now stated: **no ROM here writes SWACNT or
SWBCNT at all**, and none needs to, because the engine resets RIOT chip memory to zero — all inputs —
and `deriveSWCHA` then returns the peripheral value unchanged. That is a fact about our corpus, and it
was generalised into a claim about games.

Consequence for anyone writing that litmus: it cannot confirm existing practice, because there is no
existing practice to confirm. It has to **drive a port as an output**, which nothing here does.

Also marked ⬜ rather than left implied: **the power-up value is the engine's choice, not a
measurement.** `Reset` zeroes the memory explicitly; whether a real 6532 clears its DDR on RES is not
established here. The thing to measure is what *writing* SWACNT does — the truth table, which is
hardware logic — not the default, which is ours. Same shape as the SuperChip SARA entry above.

Found by the mailing-list distillation (helper-3). One claim of theirs did not survive checking: they
reported the engine's SWCHA truth table as missing the `SWCHA_W=0, SWACNT=1` row. It is present — the
table has eight rows and all four combinations; only three carry the derivation comment beside them.

### Fixed — Pitfall's bidirectional LFSR cannot be read the way its sources are written (2026-09-03)

Two audit items settled by **enumeration, with no emulator**: both are pure arithmetic over 256 byte
values, so an emulator would only have been a slower way to be less sure.

The sources say "right step inserts `bit3⊕4⊕5⊕7`, left step inserts `bit0⊕4⊕5⊕6`". Implemented
literally — "right step" as a shift right — the function **is not even a bijection**: the orbit from
$C4 is 34 long and cycle lengths over the 256 seeds come out {1,2,3,4,31,32,33,34}. Nothing about
"period exactly 255" survives.

The correct reading is forced by the structure rather than chosen: **a shift right loses bit 0, so its
tap must contain bit 0; a shift left loses bit 7, so its tap must contain bit 7.** Each of the two tap
sets contains exactly one of those, and so fits exactly one direction. **"right" and "left" name the
direction the world scrolls, not the direction the register shifts.** Read that way every claim holds:
both steps are permutations, `left∘right` and `right∘left` are both the identity, $00 is the fixed
point, and the other 255 bytes lie on one cycle — so "period exactly 255" is exact. The disassembly's
`bit1` variant is wrong: `shr{1,4,5,6}` fails to invert for **128 of 256** values.

`eor #$B4` (SpiceWare) likewise: permutation, $00 the fixed point — "never seed 0" is not a caution but
the only way the generator can fail — and one cycle of 255. Sequences are pinned by sha256 prefix so a
re-run compares without eyeballing 255 values.

**The method was validated against ground truth before being trusted.** The same twenty lines were run
against `eor #$8E`, which `litmus_lfsr` already measures on the emulator and `scenarios/lfsr.json`
pins in CI, and reproduced `ram.0x90..0x97 == 1,142,71,173,216,108,54,27` byte for byte. Re-run here
and confirmed.

Found and computed by the mailing-list distillation (helper-3). Both entries say "computed, not run",
and the failing reading is kept in the text so the next reader does not fall into it.

### Changed — the claim with the worst reference-rot in the file had no name, so it got one (2026-09-03)

"PAL must be even" lived as a parenthetical inside the scrolling-PF-background bullet — a different
subject, under a different name. The distillation measured what that cost: **eight corpus references
cite this claim, and every one of them cites a line number that no longer holds it.** No other claim in
`design-principles.md` comes close.

The cause is structural rather than careless. A reference can only be as stable as the thing it points
at, and a claim buried in a subordinate clause has no name to point at — only a line number, which the
next edit moves. Promoted to its own rule so it has an address that `rg -F` finds.

While promoting it: `design.ScrollScanlinesConstant` carries **two** checks under one name — the count is
constant frame to frame, *and* it is even under PAL (`pkg/design/pf.go:60`). That is now stated where
someone reading the PAL rule will see it. The original parenthetical is left in place; it reads correctly
where it is.

This also bounds the distillation's own proposal — addressing references by rule name instead of line
number, which they measured at 100% stable against 55% for line numbers. It works only for claims that
*have* names. Found by helper-1, who found the limit of their own method.

### Added — a trap we would otherwise have measured wrong: SuperChip phantom reads are off by default (2026-09-03)

Gopher2600 does model the SARA phantom-read recovery — `mapper_atari.go` has `saraCycles = 2` and the
guard `cart.env.Prefs.Cartridge.EmulateSARA.Get().(bool)` — but
`preferences/cartridge_preferences.go` `SetDefaults` sets **`EmulateSARA` to `false`**, and nothing in
this repo sets it (`rg 'EmulateSARA|SARA' internal cmd pkg` returns nothing).

So a litmus written for phantom reads today would come back **green because the feature is off**, and
the green would say nothing about hardware — it would be measuring our own default. That is the shape
this project keeps hitting: an instrument that answers a different question than the one asked, with
no outward sign. Recorded in `known-traps.md` section E with both ways out — pin the preference and
say so, or record the behaviour as not modelled.

The other half of SuperChip needs no preference and is measurable as it stands: the write/read port
split ($F000-$F07F write-only, $F080-$F0FF read-only). Nothing in `verified-coverage.md` covers
SuperChip today (`rg -i 'superchip|sc ram' docs/verified-coverage.md` → 0 hits).

Found by the mailing-list distillation (helper-3) while designing that litmus — before writing it,
which is the point. Verified against the engine source here. Six gates pass.

### Changed — six audit items split into what we measured and what we did not, and a litmus header that pointed the reader backwards (2026-09-03)

Six `fundamentals-audit.md` items were marked 📖 (documented, not measured by us) while a measurement
for them already existed in `verified-coverage.md`. Three convert outright — **RIOT timers**,
**CTRLPF SCORE/priority/ball-width**, **JMP ($xxFF)**. Three do not, and saying so is the point:

- **page-cross** — we measured `LDA abs,X` +1, `STA abs,X` fixed, `BNE` 2/3/4, `DCP zp` 5. We did
  **not** measure `(ind),Y`, RMW `abs,X`, or reads through `abs,Y`.
- **NMOS decimal** — the flag half is ours ($99+$01 under SED: A=$00, C=1, Z=0, N=1, so "never branch
  on Z or N" is measured, not quoted). D undefined at power-up and surviving interrupts is not; the
  `CLD` rule is *enforced* by `check_traps.py`, which is a lint, not a hardware fact.
- **mirrors** — the RAM mirror holds both directions and one TIA mirror renders. One mirror is not
  the $xyz0 pattern, and ROM mirroring at every odd $x000 is untested.

`JMP ($xxFF)` also split in two: the page bug is ✅, but the `.byte $2C` skip trick beside it stays 📖
**and is invisible to the linter** — `check_traps.py` matches mnemonics, so a skip written as a raw
`.byte $2C` never reaches `READ_OP`. Our own instance is `roms/techniques/tia_pcm.asm:89`.

**`litmus_timer.asm` line 5 named the RAM cells backwards** — it said $93=INTIM $94=TIMINT when the
code at :48-55 stores TIMINT→$93 and INTIM→$94. Three things already disagreed with it: the code, the
recorded values ($94=$EF is impossible for TIMINT, which only defines D7/D6), and
`scenarios/timer.json` (`ram.0x93 == 192`, `ram.0x94 == 239`). Comment-only: the assembled binary is
byte-identical, checked against HEAD.

Note the counting. **The 📖 lines went 23 → 25 while this landed** — splitting a claim into a measured
half and an unmeasured half adds a 📖. Progress here cannot be tracked by counting 📖.

Found by the mailing-list distillation (helper-3), who also withdrew their own first classification:
they had checked whether a measurement existed on the topic, not whether it covered the whole claim.
All citations checked verbatim; six gates pass.

### Fixed — the AUDC "duplicates" list was right about tuning and wrong about samples (2026-09-03)

`fundamentals-audit.md` carried the sources' consolidated AUDC table with "duplicates {0,11} {4,5}
{6,10} {7,9} {12,13}" as a single flat set, still marked 📖 (documented, not measured by us). We had
already measured it — `verified-coverage.md:108` says only {0,11} {4,5} {12,13} are sample-identical
and **{6,10} and {7,9} are inverted twins**: same period and tuning, complementary hi/lo duty.

The two lines had sat eight sections apart, disagreeing, since V2-14.

Note what is *not* wrong here. The distillation first reported this as "the documents are wrong" and
then withdrew that after reading `pkg/audio/audio.go:50`, which already says it: *the documents'
"duplicates" are correct in the tuning sense, but at the sample level they split into two kinds.*
The sources are right; the audit line was missing which level it was talking about. Fixed by splitting
the claim by level rather than deleting half of it, and the flat set now appears once, in the ✅.

Also states the consequence, which nothing said anywhere: a golden audio digest or a waveform diff
reports 6 vs 10 and 7 vs 9 as **different**, and that is correct rather than a defect —
`audio.Canonical` folds all five pairs for classification but does not make the samples equal. Without
that sentence the next person to swap those voices suspects the golden.

The base constant (≈31,399.5 Hz) stays 📖: `verified-coverage.md:109` verified the *formula*, not that
constant by name. Found by the mailing-list distillation (helper-3); all four citations checked
verbatim before landing. Six gates pass.

### Fixed — a false-positive count stated without a date, and the corpus it was measured on has grown 48 files (2026-09-03)

`check_traps.py` justified the write-only-TIA-read detector with "Zero false positives measured: 0 hits
across the 123 files in roms/techniques + roms/litmus". **That sentence carried no date**, so it read as
a claim about the corpus as it stands rather than a historical measurement. It is not: 123 was measured
on 2026-07-30, and the corpus is now **171 files** (31 techniques + 140 litmus). Forty-eight litmus ROMs
had been added since and nobody had re-run it.

**Re-measured today over all 171 and the claim survives** — 0 ERROR, 1 warn:

    python3 scripts/check_traps.py roms/techniques/*.asm roms/litmus/*.asm
    traps OK — no emu-passes/HW-fails static traps in 171 asm file(s).

So this is not a bug in the detector; it is a count with no invalidation mechanism, which is the failure
mode the umbrella `CLAUDE.md` names. The sibling comment eight lines up already carried "Measured
2026-07-30" and was therefore fine — the two sat side by side and only one was falsifiable.

Also documented at the top of the file: **the no-argument default scans `roms/techniques/*.asm` only —
31 files, not the corpus.** Both the "123" and "171" figures come from passing the two directories
explicitly, and nothing said so, which is how a reader could take the routine gate to be the measurement.

Found by the mailing-list distillation (helper-3), who also correctly did *not* flag the five "123 ROMs"
figures in `capability-gap-audit.md`: those sit under headings dated 2026-07-30 and are records of what
was true then, not claims about now. Negative control: a `lda GRP0` probe appended to a technique ROM
makes the gate exit 1; removing it returns exit 0.

### Fixed — `SEI` finally removed from the missiles technique, and a doc comment I fused the same day (2026-09-03)

Two corrections, one of them mine from earlier the same day.

**`missiles-bullets.md` still said "Needs `SEI` (no IRQ)".** That error was found and recorded this
morning and never actually edited — the distillation caught it a second time and pointed out that
`reference/atariage/63334-plp-php/notes.ja.md:19` quotes the wrong line verbatim, so the mistake had
already propagated. `SEI` does nothing here: `CPU.Interrupt()` is reached from five call sites in the
vendored engine and all five are `mem.arm.Interrupt()`, the ARM coprocessor in ELF/ACE carts. The RIOT's
PA7 flag is a status bit software polls and never reaches the CPU. 139 of 173 ROMs here still open with
`sei` and that is fine as convention; calling it a requirement of this technique was not.

**Adding `RAM2600` fused two doc comments.** No blank line between the new constant's comment and
`ScrollScanlinesConstant`'s, so `go doc RAM2600` printed the function's description and
`go doc ScrollScanlinesConstant` printed nothing at all. `go build` exits 0 either way, which is why it
survived a full test run. Found with Go's own tool.


### Added — the skipdraw is 17 or 20 cycles; "constant 18" was wrong twice (2026-09-03)

`fundamentals-audit.md` carried "skipdraw/DoDraw **constant**-18-cycle draw" as documented-only and added
"worth a cycle litmus", which was an accurate self-assessment. Timed over eight frames of
`vertical_pos_dcp.asm`: **20 cycles from WSYNC to GRP0 on the 80 lines that draw**, **17 on the 1,686 that
skip**. Not constant, and neither figure is 18.

The ROM's own comment already read `~17-20`. The audit line did not, and a kernel budgeted at a constant
loses three cycles on exactly the lines that draw — the tightest ones it has.

The two paths are separated by the branch's own cycle count (taken 3, fallthrough 2) rather than by
reading sprite state, so the test does not need to know which lines are supposed to draw. Negative
control: asserting 18 for both paths fails on both, and on the "they must differ" clause as well.

Found by the Stella distillation, which reported the corpus's several different 18s and noted that
harness's own ROM had the right numbers in a comment nobody graded.


### Changed — four doc lines corrected against the sources they cite (2026-09-03)

Four separate findings from the same audit pass, all "harness says X, the note harness cites says
something else":

**The ~18 colour bands do not come from the 3CC grid.** `design-principles.md` read as if 3 colour clocks
produced the band count; 160 ÷ 3 is 53, not 18. The 18 comes from the width of the STORE that paints a
band — `writeCycles × 3`, so 9 px for `STA zp` — which is exactly what `design.MinColorBandWidthPx`
computes and `color_test.go` pins at 9/12/18. The code was right and the prose was a factor of three out.

**"No division" costs four instructions.** The proportional-homing line said `(target−pos)/16` needs no
division, which is true, and left out what keeping the sign costs: a `cmp #$80` before each of the four
`ror`s 〔107024:16〕.

**The pixel-aspect spread now names its free variables.** The line said the 1.67–1.82 answers differ "by
display assumption" and named only overscan. The sources name two more: an NTSC display expects 227.5
colour clocks a line and the 2600 emits 228 〔169128:12〕, and the machine is 240p progressive at full
refresh rather than half 〔208810:9〕. Neither number appears anywhere else here — 227.5 gets zero hits
across five layers, and every 228 in the tree is cycle budget, a different quantity.

**The ball cannot be replicated horizontally**, and only the solver knew. `internal/place/place.go:194-197`
gives players and missiles the full NUSIZ pattern set and the ball `{0x00}` alone; the technique catalogue
said nothing. Now `restrobe-copies.md` does 〔190154:40〕.


### Changed — `InterlaceColorsSafe` called "safe" the pairs its own source says are unusable (2026-09-03)

The function returned `Luminance(a) == Luminance(b)` and the doc line told authors to give both temporal-
mix colours the SAME luminance. The note harness cites for that rule carries the rebuttal on the same
page: seagtgruff points out that telling a dark green from a light green needs *some* luminance
difference, and 〔176987:37〕 says it outright — 完全均一は不可, the craft is "the smallest luminance
difference that still reads, with the largest hue difference".

So the function was returning true for exactly the pairs the source says a player may not be able to tell
apart, under a name that claimed a judgement. Renamed to **`SameLuminance`**, which is what it computes.
The doc now carries the trade-off and says plainly that the threshold — how small a difference still
reads — is measured nowhere in this tree (five layers, zero hits), so it is picked by eye and recorded.

Found by the Stella distillation reading the cited note far enough to reach the rebuttal.


### Added — a scrolling background's three layers were prescribed without asking whether they fit (2026-09-03)

`design-principles.md` describes a scrolling PF background as three layers — board RAM, display buffer,
delta update — and pointed at `design.ScrollScanlinesConstant`, which looks at line counts and PAL
evenness and nothing else. Nothing asked whether the three fit in 128 bytes. The source harness itself
cites says they usually do not: a world rewritten at run time needs SuperChip/CBS RAM, because internal
RAM only holds a **120-byte-class** malleable world 〔200972:14〕.

`design.ScrollBackgroundFitsRAM(board, buffer, delta, stack)` and `design.RAM2600` close it, with the
stack counted because that is what actually tips a plausible budget over.

Found by auditing harness claims against the sources harness cites — the same pass that produced the ASR
correction. Fourteen candidate pairs yielded two real findings; most of the rest were the audit tool's own
false positives, which is the expected shape.


### Added — missiles have no delay path, measured against the ball that does (2026-09-03)

The line "Missiles have no vertical delay (so in a 2LK they start only on even lines)" was
documented-only, and all three harness layers returned 0 for it. The claim only says something next to an
object that DOES have the delay, so the fixture puts the missile beside the ball under identical enables:
every VDEL bit set, both enabled on one line, no GRP write after. The missile lights; the ball stays
dark. Two controls make that readable — VDELBL clear lights both (the fixture does enable the ball), and
VDELBL set plus a GRP1 write lights both (the ball was waiting on a latch, not broken).

Fifth item closed by auditing harness against its own declared gaps. 24 documented-only lines remain,
from 31 this morning.


### Added — grade VDEL's cross-copy, including the ball half nobody had checked (2026-09-03)

`fundamentals-audit.md` carried VDEL as documented-only while calling it "the load-bearing mechanism".
The claim (Stella PG §6.D): writing GRP0 copies P1's new→old; writing GRP1 copies P0's new→old **and also
ENABL's new→old**. The vendored engine does exactly that (`hardware/tia/video/video.go:234-238`), and
nothing asserted it.

The ball half is the strange one — a write to a *player* register moving the *ball's* enable — and it
gives a clean binary reading. Two bands one instruction apart: with VDELBL set and ENABL's new copy on,
the ball stays dark after `sta GRP0` and lights after `sta GRP1`. Both players show their old byte, each
latched by the other register.

The fixture latches every old copy to zero on entry, and that is not decoration. Without it the ball's
old copy survives from an earlier frame, band B passes on stale state, and the negative control that
should destroy the effect (writing GRP0 where GRP1 belongs) does not fire. That is how the defect was
found: the control was run, did not fire, and the fixture was wrong rather than the claim.

Negative controls now fire on both halves. Stella agrees 37/37. Fourth item closed by auditing harness
against its own declared gaps; that file is down to 25 documented-only lines from 31 this morning.


### Added — the RESMP lock offset is not the centre once the player is wide (2026-09-03)

`fundamentals-audit.md` carried "release leaves M centered on P (Stella PG). ⬜ exact lock offset". The ⬜
was stale: `litmus_resmp.asm` + `scenarios/resmp.json` have locked the offset at +4 since v1.47.0. What
they could not answer is the word "centered", which is a claim about width and needs the width changed.

Across three widths: **+4 at NUSIZ 1x** (an 8-clock player, so that is the centre), **+6 at 2x** (16
clocks, centre +8) and **+10 at 4x** (32 clocks, centre +16). Centred holds at 1x only. The snap fires
when the player's scan counter reaches a particular pixel - 2, 4 and 5 respectively
(`Gopher2600 hardware/tia/video/player.go:776`) - so it follows a pixel index, not a width. A bullet
spawned from a double-width shooter appears two clocks left of where "centre" predicts, six at 4x.

The lock has to be held for a full scanline; locking and releasing inside one line never snaps. That
cause was itself got wrong once and corrected in the generator: the first note said the player had to be
DRAWN while locked, and removing the graphics byte changes nothing while removing the held line breaks
three of the four gradings.

Negative controls: calling 2x the centre fails by name; a lock released inside one line fails three of
four; a missile that does not track the player's sweep fails. Stella agrees 37/37.


### Added — grade the HMOVE side effects a ROM had been measuring silently since V2-2 (2026-09-03)

`litmus_hmove_side.asm` has recorded three numbers in its header since V2-2 and nothing checked any of
them: the ROM was carried only as ceiling corpus, so those lines were a comment. `fundamentals-audit.md`
marked the mechanism documented-only and `design-principles.md` marked the comb half `(needs litmus)`.
Both markers were accurate.

Now graded. A strobe right after WSYNC extends HBLANK and paints the left-side comb with **every HMxx at
zero** — 16 of band A's 32 lines, strictly alternating, which is what makes it the extended blank rather
than an object that walked left. A strobe mid-visible paints **no comb on any of 32 lines** and displaces
nothing: P0 holds clock 9 across all 64 lines of bands A–C. A strobe at the end of the line **adds 8 to
the nibble**: HMP0 = $10 asks for one clock left and delivers nine, measured over 14 uniform strobes,
151 → 34.

The loop-exit strobe moves −8 rather than −9, because `bne` falls through on the last iteration and the
strobe lands one cycle earlier. That is asserted separately instead of trimmed away — a fixture whose
last band is quietly excluded is a fixture nobody can reproduce.

Negative controls: dropping the +8 (grading the nibble alone at −1) fails on every step; claiming the
comb across the mid-visible bands as well fails by name.

This is the second item closed by auditing harness against its own declared gaps rather than by reading
the corpus. `fundamentals-audit.md` is down to 27 documented-only lines and three `needs litmus` markers
remain.


### Added — the reset-strobe phase, and the one clock between an 8-clock object and a 1-clock one (2026-09-03)

`fundamentals-audit.md` carried the RESPx pipeline as 📖, and that file's own legend defines 📖 as "stated
by a document, NOT measured by our litmus ROMs". The statement was Towers': counter reset, then the first
visible copy appears 5px right of the reset. Nothing in the tree graded it — checked with all five layers,
including the ledger and `reference/` - while `litmus_jmpind_pos` and `plan_sprite_placement` each measured
their own x0 empirically, precisely because this constant was never pinned.

It is 5, for the player. The missile and the ball land at 4. That one clock between an 8-clock object and a
1-clock object is a number the document does not carry, and it is the number a placement routine gets wrong
if it treats the three alike.

The offsets are read against the strobe instruction's own beam position rather than derived from cycle
arithmetic: `TraceClocks` reports each instruction's start and end in visible coordinates, so the strobe's
end clock and the pixel it produces are two readings of the same frame. That matters because a three-cycle
`sta` spans nine colour clocks and any derivation has to assume which of them latches the reset - an
assumption this fixture does not need to make.

`litmus_respx_phase.asm` sweeps sixteen strobe cycles per object, one CPU cycle apart, so consecutive bands
are exactly three colour clocks apart; the slope grading requires an unbroken run of twelve such steps,
which stops the offset assertion from being satisfiable by a fixture that never moved. Stella agrees on
37/37 write-only registers.

Three faults in the fixture were caught by its own probe before any grading existed, and each is written
into the generator rather than silently fixed: a delay of exactly one CPU cycle is not constructible from
`nop`/`bit` and is now refused instead of rounded; `ldx #0` is a 256-iteration loop, so the filler is
omitted when the bands already fill the picture (the first version produced a 35-scanline frame); and
`RESM1` is $13, not $12 - with $12 and only ENAM1 set, nothing visible moved and the probe read a constant
across all sixteen missile bands, which is exactly what a wrong strobe address looks like.

Found by auditing harness against its own declared gaps rather than by reading the corpus: of eleven harness
findings the day before, six needed no corpus at all.


### Added — indirect-jump positioning, and a 2002 attribution this file had eight years wrong (2026-09-02)

`design-principles.md` carried one bullet holding two unrelated claims: that striking HMOVE at cycle
73–74 suppresses the left-edge comb, and that indirect-jump positioning is "Omegamatrix's ... the
HMPx low nibble doubles as the jump index". Both sat under one `(needs litmus)`. They are now two
bullets, and the second is backed.

The technique is older and a different shape than the credit said. Erik Mooney posted it to the
Stella mailing list on 2002-07-21 (`200207/msg00330`, expanded in `msg00334`): prepare N hardcoded
positioning lines, compute a pointer off-screen, `JMP (ptr)` into the right one. No low-nibble
folding — the nibble stays free for movement. `litmus_jmpind_pos.asm` measures nine variants five
CPU cycles apart and finds the object exactly 15 colour clocks apart each time, in a deliberately
non-monotone table order so a band-index artefact cannot pass; the sixteen HMPx nibbles land at
−signed4(nibble) from the unmoved position, reaching −7..+8.

That makes Manuel Polik's arithmetic in the same thread (`msg00332`) checkable for the first time:
"I'd only need 9 kernel parts, as there are only 128 horizontal positions." Nine parts reach **136
contiguous positions**, so the claim holds — but the reason is not in the thread. The set is
contiguous only because the fine range (16 values) is at least the coarse step (15 clocks): the
intervals [15k−7, 15k+8] and [15(k+1)−7, 15(k+1)+8] meet exactly at their endpoint. One fewer fine
value and nine parts leave holes. `TestJmpIndPosCoverageIsContiguous` derives that from the measured
step and the measured range, not from the constants, so a change to either fails it by name.

The comb half of the old bullet is still unbacked and now says so precisely: `litmus_hmove_side.asm`
band D does strobe at ~cycle 74 and its header records "no comb", but no test grades it — the ROM is
only carried as ceiling corpus.

Two negative controls were run against the fixture itself: pointing every table entry at variant 0
collapses all nine bands onto one clock and fails eight gradings, and sorting the table order trips
the monotonicity control by name.


### Changed — technique #36 was generalising from one strobe spacing (2026-08-26)

`restrobe-copies.md` measured the re-strobe ladder with strobes eight cycles apart, found 3 + k, and
then said in as many words that "the eight-cycle spacing in the fixture is not special; any spacing
at or above three cycles works". Sweeping the spacing says otherwise, and not at the edges:

    spacing   3   4   5   6   7   8  10  12
    k=5       4   -   4   8   8   8   6   -

At **three and five cycles the ladder is FLAT at four** — every strobe after the first buys nothing.
At **twelve it climbs faster than 3 + k** (4, 6, 8, 9). And the doc's stated consequence that "an
added copy is never at x = 0 (mod 3)", which it called a rule that "binds every kernel built on
this", is false at spacing six, where the copies are 45, 63, 81 and 99.

The fixture now sweeps eight spacings x k=1..5, and `restrobe_test.go` grades all 35 as a table plus
a thirty-sixth band that measures **two players reaching sixteen slots** — the doc's opening claim,
which until now was 8x2 done in the head while the same doc's only real-world datapoint was eight
slots a scanline. Sixteen holds: P0 and P1 draw sixteen objects at sixteen distinct positions, three
of them clipped to 7 px because at that schedule the copies sit 9 and 7 apart in alternation.

The mechanism behind the spacing dependence is NOT explained, and the doc says so rather than
offering a third model — two were written down during this work and each is refuted by a row of the
same table.

Also in this pass: the doc's citation of `reference/atariage/311795` resolved nowhere (the directory
is `311795-576-1008-characters`) so `check_provenance.py` was red; `litmus_restrobe.bin` had reached
the corpus with neither a Stella capture nor a queue line, so `TestStellaAgreesWithHarnessOnWriteOnlyTIARegisters` was red — captured at 37/37, then retired again here because the ROM changed and a
capture records no hash, so a stale one would compare silently rather than fail. `pf_wraps.bin`,
queued since 2026-08-23, was captured in the same pass at 37/37.


## [Unreleased]

### Added
- **Indirect-jump object positioning** (`docs/design-principles.md`,
  `roms/litmus/litmus_jmpind_pos.asm`, `internal/emu/jmpindpos_test.go`,
  `scripts/gen_litmus_jmpind_pos.py`). `JMP (ptr)` into a table of hardcoded positioning lines
  replaces the cycle-counting delay loop: each variant defers the RESPx strobe by 5 CPU cycles = 15
  colour clocks, and the HMPx nibble applied by the following HMOVE fills in between. Nine variants
  reach 128 contiguous positions (measured 136) and are contiguous only because the fine range is at
  least the coarse step. Pays ROM for cycles plus >= 2 bytes of RAM per object. Sourced to Erik
  Mooney and Manuel Polik, Stella list 2002-07, correcting a later attribution.
- **Technique #36 — RESPx re-strobing: more copies a scanline, and how many depends on the spacing**
  (`docs/techniques/restrobe-copies.md`, `roms/litmus/litmus_restrobe.asm`,
  `internal/emu/restrobe_test.go`, `scripts/gen_litmus_restrobe.py`). A player in a NUSIZ copy mode
  draws more copies on one scanline with each mid-line RESP strobe — 3 + k at spacings of 6, 7 or 8
  cycles, so one player reaches eight shaped slots and two reach sixteen. This bullet first said
  "eight copies a scanline, not three" and stated 3 + k without qualification; the spacing sweep in
  the entry above shows the ladder is flat at 3 and 5 cycles and steeper at 12, and both entries are
  still unreleased, so this one is corrected in place rather than left to contradict it.
  `reference/atariage/180632` had filed this as solidcorp's unverified candidate ⑨ since 2011.
  Two consequences are graded with it: an added copy is **never at x ≡ 0 (mod 3)** (a base is
  `3c − 60`, so its surviving copies land at ≡ 1 and ≡ 2), and the leading two copies cannot be
  moved. The mechanism is `sprite-placement.md` rule 8 read from the other side — each strobe costs
  the copy at its own base and the one pending from the old base, and buys the two the new base
  makes, hence exactly +1 per strobe.

### Changed
- **`sprite-placement.md` rule 5 was over-general and is now scoped.** "A strobe does not draw the
  new position on the line it runs on" is true of the FIRST copy; later NUSIZ copies of the new base
  do draw, on that same line — which band 10 of the same fixture had already measured ("56 and 88
  but not 24") without the one-line rule saying so. Read as general, it makes six shaped slots a
  scanline look like a hardware ceiling, and it did: three probes in the work that prompted this each
  tried to check it with a ONE-COPY player, where "the first copy" and "the only copy" are the same
  thing.

### Note — a correction that was nearly made and should not have been
  The measurement that produced the ladder first read rules 1, 2 and 9 as **wrong by three pixels**.
  They are not. The new fixture labelled its strobes one cycle high — padding to the store's FIRST
  cycle and calling that the write cycle — where this catalogue counts the store's LAST
  (`scripts/gen_litmus_sprite_place.py:strobe` pads to `want-2`). Same measurement, different origin.
  The fixture now uses the catalogue's convention and rule 1 reproduces exactly. Nothing in
  `internal/place` changed.

### Added
- **`cmd/bandsplit -files` — several WAVs side by side, for stems rather than bands.** The same
  page answered the question twice on one job: the band page got the author to "B" and the stem
  page got him to "the bass stem", and only the second was the sound he meant. It prints each
  source's level BEFORE levelling, because a stem 25 dB under the others is one the separator
  found nothing for rather than a quiet instrument — which is exactly what `other` was.
- **`cmd/keyfit -waves` and `-fine` — one waveform, and a tonic off the semitone grid.**
  `-one-voice` only REPORTS what a single waveform would cost and cannot be told WHICH, and the
  best tonic for a single voice is generally not on the grid: for AUDC 6 over
  {0,3,4,5,10,12,15} it is 38.4 Hz, D#1 minus 21 cents. Both gaps had to be hand-rolled twice in
  one session, prompted by "one type of sound only, please" after a build that changed timbre
  mid-figure. Run on that figure the tool also found a better saw tonic than the hand search
  had (+24.1 cents against +25.4) — the hand grid had stepped past it.

- **`cmd/gridfind` — measures what every other audio tool here takes as an INPUT.** `audioingest`
  takes `-from`, `drumfit` takes `-t0`, and drumfit's own documentation says to read that value
  off audioingest, so one wrong start propagates through the whole chain with nothing to catch
  it. Measured cost: two delivered mp3s each carried ~233 ms of digital silence; a grid built
  without accounting for it sat two sixteenths out of phase for two days, and four-on-the-floor
  read as "the bass is on the offbeat" — every note reading coherent and wrong.
  - **T0 is the first HIT after the silence, not the beat phase.** Phase is known only modulo a
    beat, so it says where the beats are and not which one opens the bar. On the real file the
    two disagreed by most of a beat and the phase was wrong; the phase is now printed as the
    CHECK, with an explicit warning when they part company.
  - **Pattern length is measured per BAND, and the whole-mix answer is the DRUMS' answer.** On
    the same file the full band says 1 bar and `-band 85,1000` says 2 (correlation 0.963 against
    0.681), because the drums repeat every bar and the lead every two. Run on the whole mix it
    prints that warning unprompted. It reports the smallest true period, since a two-bar pattern
    also correlates at four and eight.
  - Run on the material it was written for it reproduced, from one command, a two-bar finding
    that had previously taken a multi-agent analysis.
- **`cmd/voicefit` — which of the machine's waveforms sounds most like THIS.** The timbre half of
  choosing a voice, where `keyfit` is the tuning half; the two routinely disagree. Measured: a
  lead was fitted by tuning alone, landed on AUDC 12, and the author's first words on hearing it
  were that the timbre was nothing like the record — AUDC 12 is a squarewave with NO even
  harmonics and the line rolls off 1.00 .47 .16 .07, making it the FURTHEST of the eight. Run on
  the octave-up build, `mixmatch` actually ranked AUDC 12 BEST, because band balance and harmonic
  structure are different questions and transposing an octave can satisfy the first while failing
  the second. Both are now printed and the disagreement is the finding.
- **`cmd/bandsplit` — which sound do you MEAN?** The step before every measurement here, and the
  one that actually blocked the work: an author asked for "the most prominent melodic sound" to be
  reproduced, guessing produced a bass reproduction that was not it, and a hand-written band-split
  page settled it in one exchange — then was thrown away. Bands are normalised to the same peak,
  because they differ by tens of dB and comparing them at natural levels asks which is loudest
  rather than which holds the part. The page states in its own footer that a band is not an
  instrument, since on this very record the author's correct answer of "B" pointed at a band that
  excluded the part's fundamental.
- **`cmd/keyfit -hz` — fit MEASURED pitches instead of degrees of a tonic.** Real music does not
  sit on a semitone grid: the line this was added for reads 97.12 and 115.73 Hz, is 14 cents flat
  of A=440, and its minor third is 303.5 cents rather than 300 — all of which rounding to degrees
  throws away before the search starts. It ranks by worst PAIRWISE INTERVAL error rather than
  absolute, because a figure moved bodily is still the same music and one whose thirds have
  changed size is not. `-waves` restricts the search to what `voicefit` picked, which is how the
  two halves of choosing a voice are made to meet. On the material it was added for it found a
  pairing at 1.0 cents where a hand search had stopped at 1.7.
- **`pkg/audio.MeasuredSpectra`, `SpectrumDistance` and `HarmonicsF`.** The measured harmonic
  series of all eight pitched waveforms lived in `internal/emu/audioshape_test.go`, where **no
  tool could import them**: choosing a voice by timbre — the thing an author actually does — had
  numbers behind it that only a test could see. That is the third time something real here has
  been unreachable (`internal/keyfit` and `internal/mixmatch` had no CLI; six commands were
  missing from CLAUDE.md). The table moves to the package the tools import and the test becomes
  what it should always have been: the check that the pinned table still matches the hardware.
  `HarmonicsF` puts a reference RECORDING on the same axis as the machine's own output, with the
  arithmetic in one copy so the two cannot drift.

### Removed
- **`cmd/mixcheck` and `keyfit -hz` were written today and deleted today, after an audit.** Both
  are recorded here rather than silently dropped, because the reasons are the useful part.
  - **`mixcheck`** was to answer "is this file one source or a mixture", and its insight is
    right: band-limiting selects a frequency range and not an instrument. The implementation
    could not support it. Its verdict flipped with the analysis band on the SAME file — a
    separated bass stem read 99.6% "one source" over 30-400 Hz and 44.1% "a mixture" over
    60-1000 — and it called another record's full mix a single source at 65.4%. The thresholds
    were fitted on the four files it was developed against. More decisive than any of that: if
    the right move is always to separate, and at about ten seconds it is, then a tool that tells
    you whether to separate has no decision to inform. **The knowledge moved into the doc
    comments of `cmd/audioingest` and `cmd/f0check`, with the demucs recipe**, which is where it
    always belonged.
  - **`keyfit -hz`** fitted measured absolute pitches instead of scale degrees. On the only
    figure it was ever used on it returned **47.3 cents where `-degrees` returned 25.4**, because
    it fitted each note's measurement noise as though it were music. A measured figure that sits
    within 0.3 of a semitone of a scale IS on that scale, and degrees are then the right unit.
    It nearly shipped a worse ROM.
  - Also removed: a duplicate `dft` in `internal/audioingest` that shadowed the one already in
    the package's own tests, added along with `mixcheck` and unnoticed until the audit.

### Fixed
- **Four one-off investigation programs were published on GitHub, and the gate that should have
  named them had a silent exemption.** `cmd/_c`, `cmd/_s`, `cmd/_shape` and `cmd/_v` went in on
  2026-08-09 and 2026-08-11 as three commits' worth of "ついで", 3.6 KB in total, with hard-coded RAM
  addresses, fixed frame counts, `panic(err)` for error handling and not one line of documentation.
  `check_wiring` skipped any directory starting with `_` — in FOUR places, and the convention was
  written down nowhere: `grep -n 'cmd/_' CLAUDE.md docs/ scripts/` found it only inside the gate
  itself. So the gate reported "all 44 commands are named in CLAUDE.md" while silently excluding
  four of forty-eight, which is the pf_deadlines disease with the count hidden instead of printed.
  - The author's own standing rule covers this: investigation scripts and PoCs do not go in a
    repository unasked, and **a public repository is to be treated as visible the moment something
    lands in it**. Measured 2026-08-23: `gh repo view` says PUBLIC and all four returned HTTP 200
    from `raw.githubusercontent.com`. Nothing referenced them (0 hits outside their own directories).
  - The exemption is removed rather than documented. Writing it down would have published a recipe
    for evading the gate. With it gone, the four were named immediately — the negative control ran
    itself — and after deleting them the count reads 44 again, this time out of 44.
  - **Deleting does not un-publish.** They remain in the history and on GitHub's commit pages;
    rewriting history to remove 3.6 KB would break every clone, which is not a trade worth making.
    The effect is that nothing new is confused by them, not that they were never there.
- **The playfield-deadline check went GREENER the harder the kernel was broken.** Colour clocks
  fold back every 228, and `clockAt` folds `MaxClock` with them, so a write pushed a whole scanline
  late reappears as a small clock in the next line's HBLANK and compares as comfortably early.
  Measured by the other session, adding nops at the head of a play region: +10 nops (96 cycles over
  a 76 budget) gave 6 of 23 LATE, +26 gave 3, **+40 gave "all land in time"**. The worst kernel was
  the green one.
  - A write whose `MaxAbs` reaches 228 is being measured against the deadlines of a line it is not
    on — whether a defect put it there or the region legitimately spans two lines, the table
    describes ONE line either way — so it goes to `Unjudged` instead of being compared. The
    predicate is `MaxAbs`, **not** `CrossesLine`: that flag is `minAbs/228 != maxAbs/228`, false for
    any EXACT window, and a run of nops is exact. Measured on the new witness: with `CrossesLine`
    it sets nothing aside and judges the wrapped writes anyway (10 checked, 2 late, 0 unjudged);
    with `MaxAbs` it sets 7 aside and judges the 3 that stayed in their line.
  - `roms/litmus/pf_wraps.asm` is the third witness beside `pf_ontime` and `pf_late`, and two
    corpus sweeps enrolled it the moment it existed: `TestNoRomBreathesAcrossFrames` (a kernel that
    overruns cannot hold a frame length — now a named exclusion with its measured distribution) and
    the Stella TIA oracle (now queued for capture). Neither had to be remembered.
  - **Measured across the corpus, which is the point of splitting the count**: of the 83 scenarios
    that set `pf_deadlines`, zero have an unjudged playfield write under either predicate. This
    closes a hole the tree does not currently contain — and the next kernel to fall into it will be
    told rather than congratulated.
  - `cmd/cyclebound` still declines the check entirely on an uncertified kernel while `cmd/scenario`
    runs it, so the two tools answer the same question differently. That difference is now the only
    one left, and it is written down here rather than left to be rediscovered.
- **Deleting a branch ran the full 5 min 41 s pre-push gate on a commit nobody was pushing.**
  `git push --delete branch` sends an all-zero local sha; the loop skipped it, `SHA` stayed empty,
  and the fallback reached for HEAD — so the hook built and tested the working tree's HEAD, which
  is not what the push contained, and could refuse a deletion on the strength of an unrelated
  failure. A push whose refs are all deletions now says so and exits 0. The fallback stays for the
  case it was written for: a hook invoked by hand with no refs at all, where HEAD is the only
  candidate. Found by the other session; confirmed against all three shapes (deletion only, no
  refs, ordinary push).
- **`pf_deadlines` printed one number for two opposite facts.** The verdict ended in "(N write(s)
  had no rule and were NOT checked)", which borrows the language of skipping a check that was due.
  A careful reader took it for a coverage hole in the playfield check on 2026-08-23, spent an
  afternoon on it and retracted. The count is three things: a register the playfield rules do not
  govern (GRP0, ENAM1, NUSIZ…), a second write to COLUPF/COLUBK, and a THIRD write to PF0/PF1/PF2
  — and only the last is dangerous, because it is a playfield write this model genuinely cannot
  judge. Summed into one figure, the reader cannot tell "not our business" from "our business, not
  checked", which are opposite conclusions.
  - The first replacement said "non-playfield write(s)" and was **false** — a third PF0 write is a
    playfield write — and the existing test caught it within the minute. Replacing a misleading
    sentence with a wrong one is not a repair.
  - The count is now two: `NotOurs` prints as an aside, `Unjudged` prints in brackets and capitals
    as "PLAYFIELD write(s) NOT JUDGED … this verdict is silent about them", because a rare failure
    formatted like a routine one gets read like a routine one. `isPlayfieldReg` is the classifier
    and a test pins it, including the case that started this: a third PF0 write must land in
    `Unjudged`, never in `NotOurs`.
  - **Measured across the corpus, which the split is what made possible**: of the 83 scenarios
    that set `pf_deadlines`, **zero** have an unjudged playfield write; all 83 carry only the
    not-our-business kind. The dangerous case exists in the model and does not exist in the tree —
    a thing that is true because it was measured, not because it was assumed.
- **A server that started current reported "not stale" forever.** `analyzerStamp` computed the
  whole stamp inside one `sync.Once`, on the reasoning that "HEAD moving under a live server is
  exactly the case being reported, so re-reading it per call would let the warning disappear on
  its own". That does not hold: the only way a fresh read silences the warning is HEAD returning
  to the revision the binary was built from, at which point the binary really is current. What the
  caching bought was the opposite failure — measured by the other session by launching a server
  with nothing stale, moving HEAD under it, and calling again: same answer, no warning, forever.
  This is a long-lived MCP server the author reconnects to rarely, so "forever" is days.
  - The split is now the honest one: the build half (version, revision, build time, dirty) is
    stamped at link time and cannot change, so it stays cached; HEAD can change, so it is read per
    call. `headRevisionFn` is a seam rather than a convenience — the half of this file that broke
    was the half no test could reach, and a test now asserts the repository is read once per call
    and that the build half does not move between them. Confirmed by putting the stale computation
    back inside the `Once` and watching the test go red.
- **Four commands the CHANGELOG already announced had never been committed.** `cmd/bandsplit`,
  `cmd/f0check`, `cmd/gridfind` and `cmd/voicefit` are named seven times across the entries above
  and in `CLAUDE.md`, with no code in the tree: this public repository has been describing tools a
  clone does not contain. They are adopted here, with the two repairs the gates asked for — a
  calibration test for `FirstOnset` and `HarmonicsF` (synthetic input, literal answer, DC as the
  negative control), and three tests in `internal/scenario` that asserted only that their fixtures
  loaded, now asserting the fact each one is named after. `check_tests` then caught a test THIS
  change had just added with no failure path at all; it was rewritten to go red if the anchor
  regresses.
- **The stale-binary warning was dead in the deployment it was written for.** `headRevision`
  walked up from the WORKING DIRECTORY, and `.mcp.json` sets no `cwd`, so the server inherits the
  client's: the umbrella directory holding harness/, roms/ and sandbox/, which belongs to no
  repository ON PURPOSE — that placement is what makes publishing `reference/` structurally
  impossible. The walk reached `/` without finding a `.git`, returned `""`, and `staleNote` opens
  with `if built == "" || head == ""`, so BOTH sentences it can raise died there. Measured
  2026-08-23: `bin/harness` had been built at 845656c while the repository sat at 2817b25, and a
  full day of static-analysis answers came back with no `stale` field at all. The `dirty` FLAG
  survived because it is stamped at build time — so the skimmable form lived and the full
  sentence, written that way precisely so it could not be skimmed past, did not.
  - `headRevision` now anchors to `os.Executable()` (symlinks resolved), which is inside the
    repository it was built from whatever the working directory is. **There is deliberately no
    working-directory fallback**: from `roms/` the walk finds `roms/.git` and would compare a
    harness build revision against a different repository's HEAD, reporting STALE forever — the
    false positive this file's own comment forbids. Under `go run` and `go test` the binary sits
    in a temp directory and the warning stays silent, which is the same silence as before and the
    safe direction.
  - **The walk now takes its directory as an ARGUMENT (`headRevisionFrom`), and that is the
    point.** The test that covered it called `headRevision()` and said, in its own failure
    message, that the warning "can never fire in practice" — a sentence it could never print,
    because `go test` fixes the working directory inside the repository, so the condition it
    named was structurally false. `TestHeadRevisionIsSilentFromTheUmbrella` stages the real
    layout (two repositories beside each other, none above) and pins all three answers; a second
    test goes red if the working-directory fallback is ever restored. Verified end to end: run
    from the umbrella, the built server now reports the sentence it had been swallowing, and a
    copy of the same binary placed outside the repository stays silent rather than crying STALE.
- **`cmd/bandsplit` no longer has a default output path.** It defaulted to `bandsplit.html` in the
  working directory, and this repository is PUBLIC: running it from inside the tree writes a
  multi-megabyte page **with the source recording embedded** straight into the working tree, one
  `git add -A` from publishing a client's unreleased master. `-out` is now required, and `*.html`
  is gitignored as a second net. The umbrella's rule that other people's material never enters a
  repository is structural, and a convenient default that quietly breaks it is worse than no
  default.

### Changed
- **A `golden_audio` / `golden_mix` check that hashes nothing is now refused at load.** The run
  length is not "frames" alone — it is the larger of "frames" and the highest `at_frame` in
  inputs/asserts — so a scenario with an audio golden and NEITHER runs one frame and matches
  whatever it was recorded against. Measured on a real work: such a scenario stayed green after
  its hi-hats were deleted from the ROM entirely. Under two frames is now an error; under sixty
  prints a warning and still runs, because coverage here varies with how a scenario was written
  rather than with intent — in this repository's own net `roms/techniques/sound_driver.json`
  reaches frame 70 through its asserts while `roms/litmus/audio.json` reaches 3, and turning that
  warning into an error is a decision about 7 existing scenarios, not about this code.

- **`cmd/f0check` — is the pitch you measured the fundamental, or a harmonic of one your band
  excluded?** Every other audio tool here takes the analysis band as an input and reports what it
  finds inside it. That is the right contract and it is a trap: a band that excludes the
  fundamental produces a confident wrong answer indistinguishable from a right one. Measured on
  real material — a lead line read 194 Hz over 110-800 Hz for two days; its fundamental is
  96.9 Hz, and 194 Hz was the second harmonic. Two independent analyses were needed to catch it.
  - Prints the autocorrelation answer AND the naive FFT peak together, because they fail
    differently and their disagreement is the finding. On that material the FFT peak was the one
    that lied; autocorrelation proved the tougher of the two.
  - Searches BELOW the band for a better period rather than testing integer multiples of what it
    found: a squarewave read from above its fundamental leaves partials at no simple ratio to it,
    which broke the first implementation and is now a test.
  - Reports the lower period's LEVEL in dB. A sub an octave down at -19 dB is a sub-oscillator
    inside one instrument; one at -3 dB is the note itself; both correlate equally well, so
    correlation alone cannot tell an author what to do.
  - `-strict` exits 1, so it can stand in a script or a gate.
  - Run on the material it was written for, it independently reproduced a finding that had
    previously taken a five-agent analysis: the line carries a sine sub exactly one octave down.

### Fixed
- **`internal/audioingest.F0` could return a frequency outside the range it was given** — three
  separate defects, all found by writing the tests for `f0check`. **Scope, measured rather than
  asserted:** `F0` is called from three places, all inside `internal/audioingest` itself
  (`BassNotes`, `F0Checked`, `MixCheck`). An earlier draft of this entry claimed the fixes
  reached `drumfit` and `mixmatch` as well; `grep` says those two use only `DecodeWAV`, and the
  claim was written without checking. The defects were real; their blast radius was smaller than
  advertised:
  - a peak landing on the EDGE of the search range has a slope rather than a summit under it, so
    the parabolic interpolation's denominator went to nothing and the correction ran away:
    measured, a window whose true period lay outside `loHz..hiHz` returned **-488 Hz**. A negative
    frequency is not a near miss; it is a value no caller can defend against.
  - clamping that correction to the half sample it is entitled to still returned **809 Hz** from a
    search told to stop at 800. Interpolation is now refused at the edge, where the integer lag is
    the honest answer and `F0Checked` is what says the answer is suspect.
  - the lag range truncated the wrong way at the short end: `int(44100/800)` is 55 and `44100/55`
    is 801.8 Hz, so the search could return above its own `hiHz`. Both ends now round so the
    answer falls inside the range the caller asked for.

### Changed
- **The fourth repository is retired, and the count in the shutdown sweep is replaced by an
  enumeration.** `260811_cover-demos` published a browser-playable page of the technojacket
  builds and had served its purpose; the repository is being taken down. The page **cannot be
  rebuilt** — `tools/mkpage.py` embeds the ROMs at generation time and three ROMs landed after
  it was made — so it is archived in the roms repo at
  `technojacket/_archive/2026-08-11-cover-demos/` with the exact served bytes (cmp-verified),
  the retired repo's README and robots.txt, a `git bundle --all` of its one-commit history
  (verified by cloning it back), and a `PROVENANCE.md` holding the restore procedure.
  - **The bytes were nearly lost to a deliberate `.gitignore`.** `tools/preview.html` is
    byte-identical to the published page, and lines 31-32 exclude it and `tools/index.html`
    from the roms repo on purpose. Those bytes had therefore never entered any git history, so
    deleting the cover-demos repository would have left the only copy of a PUBLISHED artifact
    untracked on one machine. The archive lives outside `tools/`, where the ignore rules do
    not reach; checked with `git check-ignore` before staging.
  - The checklist no longer says "all FOUR repositories". **It enumerates.** The figure was
    wrong for two days, and the session that first measured it wrote the wrong one into five
    places — including the page that was correcting the problem.
- **The CI headroom figure was a single sample, and the fastest one.** `docs/system-weight.md`
  claimed "currently 10m24, 4.5 minutes of headroom" against its 15-minute ceiling. Measured
  over the five runs since the growth landed, on essentially the same workload: **623, 623,
  721, 790, 795 s** — a 10m23-13m15 range, so the real headroom is **~1m45 at the worst run
  seen**. The spread is runner variance and not code: the 790 s run differs from the 624 s run
  by two sub-second calibrations. **A CI budget has to be set against the distribution, because
  the run that breaches the ceiling is the slow one** — and taking one sample, at the
  favourable end, is precisely the defect `check_instruments.py` was extended to forbid on the
  same day. Consequently the parked sweep optimisation is no longer optional-looking: it is the
  first place to come back to.

### Fixed
- **The shutdown checklist shipped with a step that could not fail.** Step 3 enumerated the
  repositories with `find /Users/.../2D -name .git 2>/dev/null`. On this machine macOS refuses
  to list that directory, and `find` prints the refusal to **stderr while exiting 0** — so with
  stderr discarded the step printed an empty list under a successful exit, indistinguishable
  from "there are no other repositories". A step that cannot tell *nothing is there* from *I was
  not allowed to look* is worse than the recited number it replaced, because it looks like
  evidence. Caught by running the checklist on the session that wrote it.
  - Fixed by dropping the redirect, so the sweep fails loudly (`Operation not permitted`,
    exit 1) and the refusal is read as an unfinished step. The umbrella-internal sweep is
    separate and works regardless.
  - **The sibling line is the one that mattered**: `260811_cover-demos` lived BESIDE the
    umbrella, so an enumeration that only descends from the umbrella root would have missed it
    exactly as every handoff did.

### Added
- **`docs/gate-ledger.md` — what each gate has actually caught, and one of them was running
  nowhere.** Six `check_*.py` gates existed and nothing recorded a single defect any of them
  had found, so a gate that earns its place could not be told from one that only looks like
  it does. A gate with no catches is not free: the green tick is read as "this class of
  defect is covered", which is a claim about the future it has never supported.
  - **`check_memory.py` was invoked by nothing** — not `ci.yml`, not the pre-push hook. 275
    lines, three real catches to its name (including one where it caught its own author's
    silently-failed edit), and its only mention outside its own source was a line in an
    ARCHIVED status file. Wired into the pre-push hook, which is its only possible home: it
    reads `~/.claude/.../memory`, which a CI checkout has not got and where it skips.
  - The ledger separates **catches** (a defect already in the tree) from **compliance** (new
    work blocked until it complied) from **self-inflicted** (the gate was wrong and cost a
    session or a red CI). Collapsing those three is how a gate's value gets overstated.
    Totals: instruments 10, tests 7, provenance 6 (with 3 self-inflicted CI reds), wiring 3,
    memory 3, **traps 0**.
  - **`check_traps.py` has never failed on a defect** — every measurement it has reported was
    a clean corpus. Kept, with the grounds written down rather than left to inference: it
    costs 0.10 s, it is the only mechanical thing holding three `@rom-write-ok` declarations
    in place, and it is a PRE-FLIGHT linter whose best case is stopping Claude mid-build,
    which never reaches a commit and which this ledger therefore cannot see. That is an
    argument for keeping it and explicitly not a claim that it works.
  - **All six gates together cost 2.15 s against a 444 s suite**, so none of them is a
    CI-time problem and none should be cut to save time.
  - Machine-checked, because a prose table rots: `check_wiring.py` gains a fourth inspection
    — every `scripts/check_*.py` must have a row, and the row's "Runs in" must match
    `ci.yml` and `scripts/git-hooks/pre-push`. **Free-text parsing was tried first and was
    wrong within the hour**: the reason beside check_memory's row reads "absent in CI, skips
    there", and a bare search for the word matched it, so a gate that runs nowhere near CI
    was read as claiming CI. The column is now backticked tokens (`ci` / `pre-push` /
    `none`) — a column a checker has to interpret is one it will interpret wrongly.
- **`docs/system-weight.md` — a CI wall-clock budget, and the end-of-session debris sweep.**
  Every gate here measures whether the code is right; nothing measured whether the system was
  getting heavier faster than it was getting better.
  - **Ceiling: 15 minutes of job wall-clock**, currently 10m24 (4.5 min of headroom). Chosen
    against the only constraint that binds — a push-to-green loop long enough that the author
    context-switches is a loop that stops being run before pushing. When a run exceeds it the
    next commit must make the heavy thing faster, drop it to `-short` **and record what CI
    stopped checking**, or raise the ceiling with a reason. Trimming a sweep's point count is
    explicitly not on the list: this project sweeps 512 pairs because four spot checks passed
    for a year against a broken instrument.
  - **82% of CI is one step** (`go build`+`vet`+`test -p 1`), and it took **+50% in a single
    session** (342 s → 512 s) while nothing else moved.
  - **The recorded reason for that growth was wrong.** It had been written down as "almost
    all of it is the audio sweep". Measured per package at the two commits: audio is **54%**
    of the growth and 18% of the suite. `internal/cyclebound` (+31.2 s), `internal/scenario`
    (+17.3 s), `behavmatch` and `ceiling` account for the other 46%, none of it audio. The
    512-point pitch sweep alone is 32% of the growth, so the instinct about that TEST was
    right and the generalisation to "the audio work" would have aimed the first cut at the
    wrong half.
  - **The comparison run needed correcting twice, both times in the house style.** `go test
    ./...` served most packages from CACHE, so the first per-package timings measured
    nothing — CI has no cache, so `-count=1` is the only honest local mirror. Then the
    baseline worktree lacked `bin/p6502step` and the umbrella tree, so `internal/cpudiff` and
    `internal/ramtrace` SKIPPED there and appeared to have grown 60× — **+56 s of pure
    artefact in two packages whose diff between the commits is empty**. Both were caught by
    asking why a number was surprising, not by reading carefully.
  - **The first attempt at making the heavy thing faster was measured and NOT shipped.** The
    pitch sweep is 330 independent emulator runs writing only into their own `t.TempDir()`,
    so none of the shared-`.bin` races that keep CI on `-p 1` apply. Taken concurrently:
    **41.8 s → 9.7 s, same 330 pairs, agreeing with the serial run pair for pair.** Reverted
    on two findings. (a) `go test -race` reported a race on the first run and has not
    reproduced since — 2 further sweep runs plus 15 runs of an 8-emulator probe were clean,
    and the report was captured through a `tail` that swallowed the WARNING block, so the
    addresses are unknown. **One positive detection is not cancelled by clean runs**; the
    detector does not invent happens-before violations. (b) Independently fatal: the loop
    body calls `t.Fatal` from a worker goroutine via `warmupStable`/`buildAudioROM`, which
    calls `runtime.Goexit()` and so kills the WORKER, not the test — the deferred
    `wg.Done()` still fires and the pair is left unmeasured while the aggregation counts it
    as measured. A concurrency change whose failure mode is a silently short sweep is the
    wrong change to make to the one test that exists because a short sweep passed for a
    year. There was no pressure to ship it: 10m24 against a 15-minute ceiling.
  - **The debris rule, from a measured incident**: a subagent's 34 MB git worktree was left
    INSIDE the harness directory, and `check_instruments.py` walked it as a second copy of
    the repository and reported three uncalibrated instruments that do not exist. A
    measurement worktree goes outside the repo; the baseline run above used one under
    `/private/tmp` and removed it afterwards.
  - **There are FOUR repositories, not three.** Every handoff counts harness / roms /
    sandbox. `../260811_cover-demos` (2026-08-11, pushed, `robots.txt` + `noindex`) carries
    every build of the jacket piece as base64 beside a javatari.js emulator. It is the one
    artefact here with an audience, and it appeared in no board, no index and no memory file
    until it was found by accident. Its `index.html` is GENERATED and embeds the ROMs at
    generation time, so a stale page is indistinguishable from a current one to a visitor.
- **`scripts/verify_claims.py` — re-run the command behind a number before believing it.**
  Twice in one session a subagent reported a measurement that did not survive reproduction:
  "proved on 73 of 76 regions" (the `pf_deadlines` check had never been executed at all) and
  "four points agree" (one had been measured; the other three were asserted from the shape of
  the first). Both were caught by reaching the number again, neither by reading the report
  carefully, and care does not scale with the number of claims.
  - Takes a JSON list of `{claim, command, expect}` and reports OK / MISMATCH / UNVERIFIABLE
    per entry. `expect` holds LITERALS rather than patterns on purpose — `\d+/\d+` would have
    passed the 73/76 report. An entry with no `command` is UNVERIFIABLE and fails the run,
    because that is the state both failures were actually in.
  - **What it does not catch is stated in its own docstring**: re-execution is not
    independence. If the instrument is wrong, this agrees with it — that is the class that
    cost a year, and `check_instruments.py` is the mechanical half of it. For a DERIVED
    number a structurally different second route is still required.
  - `--selftest` covers all four verdicts. Dogfooded on six of this session's own figures,
    6/6 reproduced.
- **`scripts/check_instruments.py` — a measurement that has never been checked against a
  known answer is not a measurement.** `check_tests.py` forbids a test that cannot fail;
  this is the same rule one level down. Every exported function whose first parameter is a
  slice of samples and which returns a number must have a test that feeds it an input the
  TEST BUILDS and asserts a literal answer. CI-gated, in the pre-push hook, and stated in
  `CLAUDE.md`.
  - **Checking a reader against the machine it reads proves the two agree; only a known
    answer proves it right**, and the difference cost a year. `audio.MeasurePeriod` passed
    four machine spot-checks — which happened, by luck, to be three squares and the one
    polynomial waveform whose transition count coincides with its period — while returning
    a clean fraction of the period on five of the TIA's nine waveforms.
  - **It found five uncalibrated instruments on its first run, two of them written the same
    session** (`Harmonics`, `MeasureFundamental`).
  - **And the calibrations found two real defects immediately.** `MeasureFundamental`
    returned **1** for a square of period 310 when the search started at lag 1: a two-level
    signal of long runs correlates with itself at r=0.987 at lag 1, so every real waveform
    answers 1 there. Every caller happened to pass a sensible lower bound, so the machine
    tests never showed it; `lo < 2` is now refused with the reason recorded.
  - **A third fell out of fixing the second.** Refusing `lo < 2` broke the 512-point pitch
    sweep on eight pairs: it passes `want/4`, and the shortest periods on this machine
    (AUDC 4 at AUDF 0 is two samples) put that below 2. The caller had been relying on the
    loose behaviour. Three defects in one function in one session, and **none of them ever
    appeared in a machine test** — every real call happened to pass a sensible bound.
  - **The second defect was in this project's own description of the first.** "MeasurePeriod
    breaks on asymmetric pulses" was written repeatedly today and is wrong: it returns 31.00
    for a 13:18 pulse of period 31, correctly. Asymmetry is not the problem — TRANSITIONS
    PER CYCLE are. Mean-interval-times-two is the period whenever a cycle has exactly two
    runs, however lopsided, and is off by (runs/2) when it has more: saw 8, pitfall and buzz
    16, engine 128. The calibration corrected the claim, not just the code.
  - **SECOND AXIS (2026-08-13): the STATE.** One calibration point is not a calibration, and
    this project paid for that in three separate places in one session — a Video Olympics
    reading taken only in the attract screen, a frame measurement taken only at frame 1, a
    pitch table checked at a single AUDF. Each was true where it was measured and wrong about
    everything else, so a calibration must now exercise its instrument at **two or more
    constructed inputs**. Counted mechanically: call sites across all of an instrument's
    calibrations, where a call inside a `for ... range` body counts as many, so two separate
    single-call tests (`EstimateTempo` on a click track and on noise) are two states.
  - **Measured before the rule shipped, so it is neither vacuous nor a flood: 9 of 11
    instruments already passed, and BOTH of the 2 that did not were hiding a live mutant.**
    - `DominantFreq` was calibrated only at 8192 samples, a power of two — the one length
      class for which `sampleRate/nextPow2(len)` and `sampleRate/len` agree. Production is
      `cmd/audiospec`, which passes an emulator capture: **30,955 samples on a 60-frame run,
      and never a power of two.** The mutant `float64(n)` → `float64(len(samples))` is EXACT
      at the calibrated state and reads 249.1 Hz as 263.7 — **100 cents** — at the production
      one, while passing every test in the package. Now a four-length table, asserted in
      CENTS: 50 Hz absolute is a quarter of a bin at 4000 and 348 cents at 249, and under the
      mutant the low case passed on the absolute window while its two neighbours failed.
    - `BeatPhase` was calibrated on one click track, asserting only that the answer fell
      outside the band 0.08..0.42 — which **`return 0` satisfies**, so a BeatPhase that never
      searched at all passed the whole package (verified by mutation). Replaced with a
      constructed-offset test: silence of known length is prepended, and the reading must
      follow one-for-one. **A constant ~27 ms EARLY bias is pinned rather than assumed away**
      — that is the group delay of the 512-sample flux envelope — with the slope, the spread
      (within two search steps across five offsets) and the sign all asserted.
    - Recorded and not fixed, because deleting it is a separate decision: **`BeatPhase` has
      zero production callers.** `cmd/audioingest`'s phase check is `census.go`.
  - **`--selftest`, and the parse it pins was wrong twice.** Both earlier versions of the
    state counter mis-read a `for ... range` header containing a composite literal — first
    `for[^{]*range[^{]*\{` could not see past `[]float64{`, then a bracket-depth scan took
    the literal's brace because `[]` had already closed. Each reported a four-case
    table-driven calibration as single-state, and neither was caught by running the gate:
    it printed a plausible complaint about a real function. The block brace is the LAST `{`
    on the header line, and seven snippets with known answers now hold that.

### Fixed
- **`framegen`: the per-zone calibration's slow convergence was a seed outside the
  actuator's working range, not a delicate loop.** RL-8c was blocked on it: "until
  `z1P0 want 9` reads 9 on the first iteration rather than the fourth, partial following
  will keep trading background for misplaced sprites."
  - **The zone actuator saturates**, measured over its whole domain — **198 of 198 inputs**:
    `read = 6*max(in/6, 9) + (in mod 6) - 51`. `zoneCoarseFine` splits the input on "one nop
    is six colour clocks" and `6*(in/6) + in%6` is exactly `in`, so on paper the map has
    slope 1 everywhere. Below **ten** nops the RESxx strobe lands at CPU cycle `2n+3 ≤ 21`,
    inside HBLANK, and an object cannot go left of the screen: **inputs 0–59 all land in the
    same six pixels**, moved only by the fine nibble. Pinned as a golden; negative control —
    a naive `in - 51` mismatches 54 of 198.
  - **Every object was seeded at input 40, dead centre of that flat region**, while the
    update rule `zin += want - have` assumes slope 1. The first correction therefore carried
    the whole saturation error and was thrown away.
  - Seeding from the inverse (`zoneInputFor`) takes `zone_multiplex` from convergence at
    iteration 4 to **iteration 2**, with **ten of twelve zone objects already exact at
    iteration 0**. Still pixel-exact. Zone 0 and the no-zone case keep the old seed: they
    are placed by the prologue's div-15 routine, a different actuator with a different map.

- **`framegen`: a zone's anchor could be stolen by a line the zone is not pinned at.** The
  zone's position comes from `bx` and its anchor from `lx`, and those are two different
  measurements — `bx` can read notDrawn on a line where `lx` still reports a leftmost run,
  so one line belonging to a band the zone does NOT reproduce could win the anchor by being
  the smallest, and every `GRP` byte read through it was wrong. Measured on Fishing Derby
  with partial following on: z1 (L27-213) is pinned at 134 and came out anchored at **29**,
  the position of a band the object had already been retired from (P1 25 → 58 with the tie).
  - `zoneLeftmost` now skips any line whose `bx` disagrees with the zone's pinned X. Witness
    plus two controls: a zone whose object drifts inside its own band keeps all of those
    lines, and a zone with no pin recorded behaves exactly as before.
  - **On today's code it changes nothing** — without partial following `pin` guarantees every
    line in a zone agrees with it — so it is witnessed at the function rather than at the
    picture. An anchor that can disagree with its own pin is wrong whether or not anything
    currently reads it that way, and RL-8c's partial following removes the guarantee.
  - **The calibration's feedback loop is located and deliberately untouched.** The same
    function is called again on the CLONE's measurements, and that call is the "read" of the
    want/read pair; it takes the minimum over the zone, so a sprite drawn in the wrong place
    on any line drags the read down and the next correction with it — the recorded
    `z1P0 want 9, read 3 → 7 → 9`. Testing the clone's position against itself would close
    the loop rather than break it.

### Added
- **The TIA pitch table is now measured against the machine at 330 of its 512 points,
  where it stood at 4.** `pkg/audio` derives every note this project plays from
  `f = base/(AUDF+1)/D`, and its verification was four spot checks — (4,14) (4,30)
  (12,14) (6,9). Four cannot catch a divisor that is right across most of a waveform's
  range and wrong at one end, which is this project's recurring failure shape.
  - Sweeping with the existing measurement reported **145 of 338 pairs off by up to
    7200 cents** — and that was the MEASUREMENT, not the table. `MeasurePeriod` takes
    the mean interval between transitions × 2, which is the period only when there are
    exactly two transitions per cycle: true of AUDC 4 and 12 and of nothing else the
    TIA has. On the polynomial waveforms it returns a clean fraction — exactly
    (runs per cycle)/2, which once the shapes were measured came out as predicted to
    three figures: 4× for saw (8 runs), 8× for rumble/pitfall/buzz (16), 64× for
    engine (128), against observed 4.00, 8.07, 8.05, 8.05 and 64.01.
  - **The four spot checks were not a random four.** Three are square-like; the fourth
    is AUDC 6, the one polynomial waveform whose transition count happens to coincide
    with its period. They were, by luck, exactly the cases the measurement could handle,
    which is why a year of green never revealed its blind spot.
  - **`audio.MeasureFundamental`** (autocorrelation) measures the period regardless of
    how convoluted one cycle is. It reproduces `(AUDF+1)×D` exactly on all nine pitched
    waveforms.
  - `TestEveryPitchTheHardwareHasMatchesTheFormula` proves it **two-sidedly**, because
    either half alone is weak: `IsPeriodic` requires an exact sample-for-sample repeat
    at the formula's period (but cannot fail a formula returning a MULTIPLE), and
    `MeasureFundamental` requires nothing shorter to correlate better (but is a
    similarity, not an equality). 330 measured; 96 pitchless (DC, noise) and 86 too
    long to hold 8 cycles in 30 frames, both skipped by a stated rule and counted.
  - The first cycle is discarded, as a measurement rather than a fudge: without it 14
    pairs fail, all AUDC 6/10 at long periods. The run-length histogram shows AUDC=6
    AUDF=31 emitting runs of 416 and 576 — summing to exactly the formula's 992 — plus
    one run of 490 at the start, because the capture begins part-way through a cycle.
    Skipping one period makes all 14 exact for the whole remainder of the stream.
  - Negative control: `Divisor` 31 → 30 fails 128 of 330.
- **All nine pitched TIA waveforms characterised, and AUDF proved to be a pure time
  scaling.** The pitch sweep proves the PERIOD is `(AUDF+1)×D` but says nothing about the
  shape inside it, so a waveform that changed character across its range would have passed
  everything this project has — and the instrument tables in every driver here pick a
  waveform once and then vary only AUDF, which is exactly that premise.
  - `TestAUDFScalesTheWaveformAndNeverChangesIt` normalises each run length by `(AUDF+1)`
    and requires the sequence to be identical at every AUDF up to rotation, with exact
    integer equality. The measured shapes are pinned as a golden, because self-consistency
    across AUDF would still pass if the emulator changed every waveform the same way.
  - **Only AUDC 4 and 12 are true 50% squares** (`[1 1]`, `[3 3]`). **AUDC 6 is a 13:18
    pulse and AUDC 14 a 49:44 one** — two-level like a square but asymmetric, a different
    harmonic series, and the reason "bass" does not sound like "square". The rest are
    polynomial shapes: saw 8 runs, rumble/pitfall/buzz 16, engine 128.
  - The 13:18 came out of the fourteen pairs that resisted the exact-repeat check: AUDC 6
    emits 130+180 at AUDF 9, 286+396 at 21, 377+522 at 28, 416+576 at 31 — every one
    exactly 13:18 after dividing by `(AUDF+1)`, ratio 0.41935 with no residual.
  - Comparing position-for-position produced a second false alarm of the same family as
    the one above: AUDC 7 "changed" at every AUDF, until the sequences turned out to be
    rotations of each other. A capture begins wherever it begins; a waveform has no first
    run. The comparison is cyclic.
- **`audio.Harmonics` — the spectrum, so a voice can be chosen by a number.** `Freq` says
  where a waveform's fundamental sits and nothing about whether the fundamental is the
  loudest thing in it, and on this machine it very often is not. Measured on all nine
  pitched waveforms at AUDF 9, pinned as a golden, and derived independently a second way
  (an ideal two-level reconstruction from the measured run lengths) agreeing to three
  decimals. Three consequences an author needs before picking a voice:
  - **AUDC 2 (rumble) has its 2nd harmonic six times its 1st** (.228 against .037), because
    its waveform nearly repeats at half period — its sixteen runs split 230 and 235. A note
    written for it from `Freq` sounds about an octave above where it was put.
  - **AUDC 4 and 12 are the same timbre**, differing only in divisor (2 against 6). "Square"
    and "lead" are one instrument in two registers, not two instruments.
  - **Only AUDC 6 and 14 pair a strong fundamental (.476, .512) with a bass divisor**, which
    is why a bass line lands on them and sounds thin on saw (.149) or pitfall (.130), whose
    spectra are flat enough to have no fundamental to speak of.
- **`golden_mix` — the audio golden could not hear the second channel, and nothing here had
  ever touched the mixer.** This started as a check of an AtariAge claim that emulators
  derived from Ron Fries' 1997 audio code ignore interference between the two TIA channels,
  which would have meant our audio verification was green against the wrong machine. **That
  concern is void**: Gopher2600's audio is Chris Brenner's circuit-derived implementation
  (`Gopher2600/hardware/tia/audio/doc.go:16-18` says so outright, citing pinned Stella and
  6502.ts commits), and the Fries tables still in `polynomials.go` are dead code no caller
  references. The investigation found three larger holes on our side instead:
  - **`golden_audio` hashes `AudioChannel0` alone** (`Gopher2600/digest/audio.go:78`).
    Measured on a ROM holding channel 0 fixed and sweeping channel 1 silent / half / full:
    the hash is `44cc324ba5783a68` at all three, while the control moves correctly when
    channel 0 changes. Seven scenarios gate on it; none can see half the sound.
  - **Nothing outside Gopher2600 called the `mix` package.** Every audio tool here reads the
    raw pre-mix 4-bit channels, so the output stage was exercised nowhere — and it is not a
    sum. `mix.Mono` indexes `mono[c0+c1]` into a hyperbolic curve: superposition fails by up
    to 25% (`Mono(15,15)`=16383 against 21844), and a full channel 1 cuts channel 0's
    contribution to 48% of what it is in silence. That squashing IS the interference the
    thread is about, and Gopher2600 models it.
  - Our own `pitchdither_test.go` sums the two channels **linearly** — the very assumption
    the thread warns against, in our code rather than the engine's.
  - `EnableMixDigest` hashes both channels through `mix.Mono`, closing the first two
    together. Witness at both levels: the three ch1 volumes give three distinct mix hashes
    against one audio hash, and on `roms/litmus/scenarios/audio.json` changing AUDV1 from 8
    to 15 leaves `golden_audio` passing while `golden_mix` fails. Controls: deterministic
    across runs, and still sensitive to channel 0.
- **`keyfit.SweepDetuned` — a piece does not have to start on a semitone, and here that is
  worth a quarter tone.** `Sweep` tried twelve tonics an octave, because that is where a
  keyboard's notes are. The TIA is not a keyboard: measured on AUDC 6, its rungs sit 182.4
  cents apart at AUDF 8→9 and still 55.0 apart at their tightest across AUDF 8..31, and
  nothing obliges a cartridge to anchor to A440. On the F# minor bass figure this project
  reproduced, searching tonics continuously instead of by semitone takes the worst degree
  from **28.0 to 16.7 cents** on a single voice. Control: the detuned range contains every
  semitone, so it can never lose — asserted over four figures.
- **Just intonation is refuted for this machine, and recorded as a test so it is not
  proposed again.** The ear prefers just ratios; the TIA cannot act on the preference. The
  largest 12-TET-to-just difference is 17.6 cents (the minor seventh) against a rung
  spacing of 55 to 182, so the target moves and the chosen register does not — measured,
  all six degrees of the real figure pick the identical `(AUDC, AUDF)` under both tunings.
- **`audio.FundamentalStrength` + `Fit.OneVoiceFundamental` — the most in-tune voice for a
  bass line has no bass in it.** Asked which single waveform plays that figure most
  accurately, `keyfit` answers AUDC 1 at 16.7 cents. AUDC 1's spectrum is `.149 .146 .141
  .133`, flat enough to have no fundamental at all, so the line would be in tune and
  inaudible as a pitch. The answer is correct and useless. "In tune" and "audible as a
  pitch" are two questions and the tool answered one; both numbers are now returned
  together, and the trap is pinned by its own test.
- **`audio.Loudness` — AUDV is not a volume control, and every driver here wrote it as one.**
  The output stage is a function of the SUM of the two 4-bit volumes through a hyperbolic
  curve, so half of AUDV 15's amplitude is reached at **AUDV 6, not 7.5**; one channel at 15
  delivers **66.7%** of the two-channel maximum; and adding a second voice at the same volume
  adds **50%, not 100%**. A balance set so the numbers look right is not the balance a
  listener hears, and the error is worst exactly where two voices overlap.
  - Cross-check, stated for what it is: Thomas Jentzsch's remark that AUDV 15 "sounds only
    about twice as loud as 6" comes out at exactly **2.0000** on this curve — a real outside
    confirmation, since the curve is otherwise only ever checked against the emulator that
    implements it. Two further percentage figures attributed to the same source in our mined
    notes do **not** reproduce (a quoted +32.1% computes as +75.0%); they are recorded as
    unreproduced rather than dropped. One confirmation is one, not four.
- **`still -frame N`** — render a ROM at a named frame, for a picture with NO moving
  state. The `-trigger` mechanism picks a frame by watching a RAM byte change, which a
  still picture has none of, so it fails (correctly) with "pick a trigger byte that
  actually moves" and there was no way to get a PNG out of `still-mine`/`still-dither`
  at all.
  - Naming a frame reopens the trap this command exists to prevent, so `-frame` carries
    its own guard, and **the obvious guard is the wrong one**. This file's own package
    comment quotes a mean luminance of 6.00 for the undrawn frame against 52.39 for the
    picture, which reads like a brightness test — but 6.00 is not that ROM's number.
    `litmus_pf_allcols` and `litmus_48px` BOTH read exactly 6.00 at frame 1: it is the
    emulator's undrawn frame, identical for every ROM. A brightness floor is therefore
    either under it (and passes the very frame it was written to catch — the first
    version, at 2.0, did) or over it (and rejects any dark picture).
  - The guard is on the SPREAD instead. An undrawn frame is uniform: sd 0.00 on both
    ROMs, against 52.49 and 39.58 once the picture is there. Floor 0.5.
  - Witness + negative control + a test asserting the undrawn frame is ROM-independent,
    which is the fact that rules the brightness measure out.

### Fixed
- **`behavmatch` compared a paddle with a ball and called it eight mechanic
  differences.** Video Olympics puts the ball on the BALL object and the paddles on the
  two players; `sandbox/practice/pong` puts the ball on PLAYER 0 and the paddles on the
  two missiles. Comparing by object INDEX therefore compared unrelated things, and every
  axis line came back `**MECHANIC DIFF**` — a table of falsehoods in the exact shape of
  a finding. Nothing was wrong with the measurements; the assumption that an object index
  means the same thing in two programs was wrong, and the tool had no way to say so.
  - `ClassifyRoles` derives a role per object from what it DOES — absent / static /
    vertical / horizontal / free — and `CompareRoles` gates `CompareTraces`. When the
    roles disagree the report says NOT COMPARABLE, names the objects, and **does not
    print the per-object table at all**.
  - On the real pair it turns out THREE of the five objects differ, not one: Video
    Olympics never draws either missile, and the reproduction draws both.
  - Absence is read from the trace's `Present`, not from the metric: a never-drawn
    object still yields a zero-range metric, which classified as "static" and read as
    agreement with an object that is merely parked. That was caught by its own test.
  - Negative control: two ROMs that agree on every role still get the full comparison,
    and a ROM compared with itself is clean on every line.

### Measured
- **Both remaining unforced refusal classes were forced, and only one is real.** The
  sole-blocker table's two biggest untested rows, run by removing the refusal and
  re-measuring the corpus against a 309/626 = 49.4% baseline:
  - **multiple back-edges: claimed +6.7 pt, worth +0.0.** 309/626, unchanged to the
    address. This was the row the audit called "the largest BROAD class" — 42 addresses
    over twelve cartridges — and not one of them becomes provable; every one meets a
    second obstacle immediately, exactly as WSYNC-in-body did.
  - **unresolved bank switch: claimed +23.1 pt, worth +15.3.** 403/623 = 64.7%. Real,
    and smaller than claimed. Concentrated in FOUR cartridges of sixteen — Vanguard +56,
    Aquaventure +16, Pressure Cooker +11, Donald Duck's Speedboat +11, and the other
    twelve gain nothing. Vanguard alone is 60% of it.
  - **Two of the three axes ever forced came back at zero**, so the table predicts the
    wrong answer more often than the right one. It now says so in its own output.
  - ⚠️ The forcing is UNSOUND: it certifies regions that may leave for a bank the
    analysis did not follow. 15.3 is what a correct model could be worth, before any of
    the work of building one.

### Fixed
- **The pitch-dither note said the wrong thing, and it is corrected.** It claimed that
  swapping AUDF every frame FAILS and that two frames is required. That came from one ROM
  at one write position, where the per-frame swap measured 40.00 Hz; adding an unrelated
  `sta WSYNC` elsewhere in that ROM's VBLANK moved the same measurement to 41.17. Sweeping
  the store across five scanlines settles it: **the per-frame swap is the most stable of
  the three rates** (0.4 c spread against the two-frame swap's 14.3 c on F#2), and it is
  what the piece uses. The litmus now takes the waveform, the rung pair, the swap rate and
  the scanline of the store from RAM, so a claim cannot again rest on a single operating
  point. A second error rode along: the F0 estimator returned exact subharmonics for the
  melody register (D2's dither read as 36.8 Hz, half of 73.5) until the search was narrowed
  around the note — a confident wrong octave looks identical to a confident right one.

### Added
- **The TIA can play a pitch it has no register for.** `roms/litmus/litmus_pitchdither.asm`
  + `internal/audioingest/pitchdither_test.go` + `docs/techniques/pitch-dither.md`.
  Alternating between two adjacent AUDF values every **two frames** puts the mean PERIOD
  on the target: E1 goes from -26.9 cents (the nearest rung) to **+8.8**, measured on the
  machine, with the out-of-note energy indistinguishable from a steady tone (0.065 against
  0.063). Applied to "Bassline" in the record's own key the worst degree falls from +45.7
  cents to +14.2, which is the difference between that key being playable and not.
  - **The obvious implementation is worse than doing nothing.** Swapping every FRAME lands
    at 40.00 Hz, 41.7 cents below E1 and outside both rungs, because a frame is 16.7 ms
    and E1's period is 24.2 -- neither value ever completes a cycle. The rule is that the
    alternation period must EXCEED the note's, and it is a window, not a direction.
    That case is a test, not a footnote.
  - Detuning two channels -- the other obvious idea -- does NOT fuse: two separate spectral
    peaks, 3.5x the out-of-note energy, and it costs both channels. Ruled out with numbers.
  - Five modes live in ONE ROM, selected from RAM `$80`, so every case comes off the same
    machine instead of five builds that are assumed to be alike.
- `audioingest.SlotCensus` + `cmd/audioingest -census` — does a part EXIST in this record,
  and where. Per sixteenth, per section, per band, over the whole file.
  - Measured on "Bassline": the offbeat-eighth hat is real but **enters at bar 24 (0:47)
    and is fully open by bar 32 (1:02)**; in the opening 46 s the 6-14 kHz band is a flat
    sixteenth texture. The ROM's hat had been an admitted invention; it turns out to be
    right about the slots and wrong about the section.
  - **Two defects found by running it on a real record rather than a fixture.** A breakdown
    scored 1.09 and was reported as the file's best section -- every slot in it read 0.01
    to 0.05, and a ratio of two near-zero numbers is not a measurement (`AudibleFloor`).
    And the first metric divided the offbeat eighths by the DOWNBEATS, which assumes the
    downbeat is where the drum is; a sidechained mix ducks there, so it read 4.44 for a
    section with no offbeat part at all (`EighthLift` compares against the neighbouring
    sixteenths instead, where the ducking cancels).
  - `KickSlot` checks the PHASE against the drum that defines it, and `cmd/audioingest`
    now runs it on every census and prints the correction. The first real run was two
    sixteenths out and produced a coherent, entirely false reading of the high band.
- `cmd/still` — render a ROM to a PNG, picking the frame by a zero-page RAM byte
  (`-trigger/-lo/-hi`) instead of by index. Promoted from a throwaway because both of
  the obvious ways to grab a frame are wrong and both reached the author before being
  measured: frame 1 is not the picture (band luminance 6.00 at frame 1 against 52.39
  from frame 7 on — a naive grab writes a near-black PNG, which is what was shown),
  and selecting by a RAM byte only works when the byte moves across frame boundaries
  (the first version watched one reading $00 at every boundary, so it took frame 1
  every time and never found its second frame). It now fails loudly when the trigger
  never reaches its value, and labels its pixel diff as colour PLUS geometry — the
  clean control reports 6136 differing pixels because COLUPF follows the drum envelope
  in every build.
- `cyclebound.CheckPFDeadlines` + scenario check `pf_deadlines` + a verdict line in
  `cmd/cyclebound` — **fitting in 76 cycles does not mean landing in time.** A playfield
  write has a DEADLINE (PF0 by colour clock 0, PF1 by 16, PF2 by 48; the right half's
  rewrites by 80 / 96 / 128), and a region can prove 75 of 76 cycles, certify, report 262
  stable lines, and still draw the picture in the wrong place. That is not hypothetical: it
  shipped in technojacket's `cover-tear-speck`, where three cycles of index arithmetic at
  the top of the line pushed PF0's store to cycle 26 against a 22.67-cycle deadline and the
  picture sat two and a half columns right with the previous line wrapping in at the left.
  **Nothing in the repo asked the question**, though `BeamIntervals` had been computing the
  windows it needs since v1.114.0. The author found it by eye.
  - Witness pair `roms/litmus/pf_ontime.asm` / `pf_late.asm`: the same kernel, the same
    data, the same 40 columns, the arithmetic moved from the line's tail to its head.
    Measured — both CERTIFIED at 71 cy of 76, both 262 lines, `pf_ontime` clean and
    `pf_late` 3 of 10 writes late, worst PF1 at clock 31 against 16, quantified in the
    report as a 4-column shift. The negative control is the point: a check that flagged the
    shape of an asymmetric playfield rather than its timing would condemn every cover ROM
    in the tree.
  - Reports what it CANNOT judge instead of absorbing it: a third write to a register in
    one line, or a register with no column rule, is counted in a `NOT checked` figure that
    prints with the verdict, and a declined analysis fails rather than passing quietly.
  - The table test found a real crash — `pfDeadlineFor("PF0", 3)` indexed a 2-element table
    and panicked. No kernel in the tree writes a playfield register three times in a line,
    so only the test could have found it.
  - Both witnesses are queued in `internal/oracle/testdata/stella_tia/CAPTURE_QUEUE`
    (Stella capture takes over the screen; not run mid-session).
- `cmd/keyfit`, `cmd/mixmatch`, `cmd/drumfit` (+ their `internal/` packages) — the three
  questions that reproducing a record on this machine keeps asking, each hand-rolled
  once before being written down.
  - **keyfit** — which key can the TIA play a figure in? Its tests pin the findings that
    three ROMs rest on: the source key is unusable, D and E are outside 25 cents in every
    bass octave, exactly three registers hold a four-note line in tune on one waveform,
    and only three pitch classes repeat across octaves within 8 cents. Writing that last
    one loosely ("the TIA has three pitch classes") was wrong and had already reached a
    ROM comment; the test is written the strict way so it cannot drift back.
  - **mixmatch** — per-band dB error between a reference recording and a ROM's own mixer
    output, with weights so a band the mix cannot reach (a fixed waveform's harmonics)
    can be discounted rather than chased. A Hann window is not cosmetic here: without one
    a 45 Hz sine left −19.5 dB in the 3–14 kHz band, which reads as "the record has hats"
    when it has silence.
  - **drumfit** — measures a drum over many onsets and fits `EnvV`/`EnvF`. Amplitude and
    pitch need different windows and the package says so: per-frame pitch below 120 Hz is
    worthless (one frame is under two cycles) while the amplitude from the same pass is
    clean, so `MeasureWin` takes a separate pitch window and every point carries a
    confidence. Negative control: white noise must not decay, or the fitter would invent
    an envelope out of anything.


- `cmd/audioingest` + `internal/audioingest` — reference recording -> TIA bassline data.
  This closes a real hole rather than adding a convenience: every audio tool here
  (`audiospec`, `pcmcheck`, `golden_audio`, `read_audio_trace`) grades a build against
  something and so needs the build to exist first. There was no audio counterpart to
  `analyze_image`, which meant reproducing a piece of music depended on transcribing it
  by ear — not a capability this harness has. The tool reports tempo (onset-flux
  autocorrelation), the sixteenth grid, and each step's fundamental mapped to the
  nearest (AUDC, AUDF), with confidence and cents error both printed because both are
  findings: low confidence means no bass note was heard, a large cents figure means the
  hardware cannot play the note that is there.
  Graded on synthesised fixtures whose answer is known exactly — pitches and rests
  recovered within 50 cents at >=0.3 confidence, tempo within 3 BPM at 110/124/140 —
  plus a negative control: white noise must report tempo strength <0.15, because
  autocorrelation always has a maximum and reporting it unqualified would turn noise
  into a tempo.

### Fixed
- **`prove_line_budget` and `cmd/cyclebound` could not see a blank-region overrun.**
  cyclebound's `Certified` covers the VISIBLE regions only — a documented, deliberate
  split — but the CLI headline and the scenario gate both passed that field straight
  through. So a ROM with a 77-cycle VBLANK line printed `CERTIFIED: ... worst region 71
  cy` and its scenario went green, while the report's own `BlankOver` list held the
  violation. A blank overrun is not a visible tear; it is worse. The WSYNC after it
  waits for the next line and the frame gains a scanline.
  Found 2026-08-09 on `roms/260809_technojacket`: two instructions added to a VBLANK line took
  one path to 77 cycles, 5 frames in 300 came out at 263 lines, and `ntsc_frame_lines` /
  `frame_lines_stable` were the only checks that noticed. The ∀-over-all-paths gate —
  whose whole claim is that it does not need to catch the bad frame in a sample — was
  green on it.
  Both now fail on a blank overrun and say which kind it is. Witness:
  `TestProveLineBudgetFailsOnABlankOverrun` (verified red with the fix reverted), with
  `TestProveLineBudgetStillPassesWhenTheBlankRegionFits` as the negative control on the
  annotated twin. `Certified` itself is unchanged, so nothing that depends on its
  visible-only meaning moves.
- `roms/techniques/scenarios/multicolor48.golden` regenerated. It had been stale since
  the kernel was corrected to draw one data row per scanline; the recorded picture was
  the broken one (every scanline drawing the same 16 pixels three times). The new
  picture is verified independently of the golden: CERTIFIED at 74 cy, 262 lines,
  `frame_lines_stable` 262x130, and rendered.

### Fixed
- **The sole-blocker table I added an hour ago is an upper bound whose gap to reality can be TOTAL, and the
  correction is measured rather than argued.** That table says WSYNC-in-body is worth **+5.7 pt**. Disabling
  the refusal and re-running leaves coverage at **309 of 629 = 49.1%, unchanged to the address** — the true
  value is **+0.0**, reproducing the old forcing experiment by a different method.
  **Not one of the 36 addresses became provable.** Every one hit a second obstacle immediately: trip count
  went 20 → **45**, branch-in-body 13 → 14, call/jump-in-body 11 → 13, other 6 → 14.
  So the same caution now applies to that table's own headline — "unresolved bank switch, 145 addresses,
  +23.1 pt" has not been forced and may be worth 23 points or nothing. The test says so in its own output
  rather than in a doc nobody re-reads, and `capability-gap-audit.md` carries the measurement.
- **It also demonstrates the non-independence the ceiling table only asserted.** Removing WSYNC-in-body did
  not free addresses, it EXPOSED 25 more trip-count blockages — which is exactly why the recorded ceiling
  reads "47.1 alone / 47.1 alone / 60.2 together". The 6.1 points exist only when the obstacles go together,
  and here the mechanism is visible instead of inferred.
- **Queue item E (WSYNC inside loop body) is closed as not-worth-doing on two independent measurements, and
  nothing was written for it.**

### Added
- **The two collision GAP entries are recovered, and one of them carries a constraint this repo had nowhere:
  flicker multiplexing DISABLES the TIA's hardware collision detection.** Fetched Wayback-first
  (`aa_blog_fetch.py 8429 8431`, 84KB and 97KB, both HTTP 200) and distilled; dev-blogs **150 -> 152, gaps
  5 -> 3**.
  The reason is a MISS, not an inaccuracy: two objects that are colliding **may never be drawn on the same
  frame**, so `CXPPMM` and its siblings never latch, and the player experiences "I hit it and nothing
  happened" — silent and intermittent, the worst shape a collision bug takes. Choosing flicker therefore
  chooses software collision too. Cheapest first step from the same source: test collisions **only for a
  sprite that MOVED this frame**, since most are stationary; the price is that an overlap already present when
  a screen appears goes unnoticed until something moves.
  Promoted out of the note and into `design-principles.md` plus `design.HardwareCollisionUsable`, with a test,
  because this is the kind of rule that otherwise gets rediscovered as a bug.
- **The pair also records two authors reaching OPPOSITE conclusions on the same question, which is worth more
  than either alone.** `684-collision-detection` (Chris Walton, Prince of Persia) implemented pixel-perfect
  collision, measured it as too slow, and fell back to bounding boxes. `8431` (SpiceWare, Frantic) got
  pixel-perfect working — on a Harmony cartridge, and the same author's `10777` says outright that Frantic
  spends ARM headroom on its sprite driver. **So the 2600-alone feasibility cannot be concluded from it**, and
  the note says so rather than filing it as "pixel-perfect is possible".

### Fixed
- **`framegen`'s per-line NUSIZ table carried only the player's copy mode, so every missile was reproduced at
  one fixed width.** It recorded `SizeAndCopies` — NUSIZ's low three bits — and the MISSILE width lives in
  bits 4-5. On `litmus_objsizes`, the ROM whose whole job is to exercise every missile and ball width, all 214
  lines read `nz0 = 0`, the table looked constant, no NUSIZ replay block was emitted, and the missiles came
  out drawn on **1544 and 1536 cells against targets of 728 and 720**. Those 1632 extra cells land on
  background, and 1632 was exactly the background shortfall the report showed. The kernel had five spare write
  blocks at the time (4 of 9 used), so this was never a budget problem.
  Measuring the full NUSIZ byte takes the ROM from **2568 mismatched cells to 160** — M1 exact (720/720), M0
  seven cells out, and the 146 the ball still needs. `nusizWidth`/`nusizStartShift` already mask with `&0x07`,
  so the player paths are unchanged. No regression: 22/31 technique ROMs pixel-exact, 31/31 at 262 scanlines.
- **The test that pinned 2568 had never once read it.** `TestGeneratedCloneCellCounts` scraped
  `"N of 34240 visible cells"`, a phrase only the "differences remain" verdict prints — for a "partial
  reproduction" the count stayed 0 and the `maxCells` comparison could not fail. So the number dropped from
  2568 to 160 and the test noticed nothing, which is the "passes while covering nothing" defect this repo
  keeps finding, sitting in the file written to stop exactly that. It now computes the mismatch from the
  per-element table (visible area minus every `matched` figure), which works for both verdicts, and errors
  outright if it cannot find the area to compute from. Negative control: reverting the NUSIZ fix now fails by
  name with **1778 mismatched cells against the pinned 160**.

### Added
- **The prover's ceiling table was missing its two biggest obstacles, and the census that says so is now a
  gate.** The recorded ceiling measured three axes and put the combined figure at 60.2%. Classifying all
  **320 unbounded regions across 16 commercial cartridges** by the refusal each reports, and counting the
  ADDRESSES whose only remaining blocker is that class, says those three are not the big ones:
  **unresolved bank switch 145 addresses (+23.1 pt) and multiple back-edges 42 (+6.7 pt)** — neither on the
  table — against **trip count 20 (+3.2 pt)**, which is the axis this work had been queued to attack.
  Baseline in this counting: 309 of 629 = 49.1%.
  **The concentration matters and is stated with the number**: bank switch is FIVE cartridges and Vanguard
  alone is 69 of the 145, so it is the largest figure and the narrowest cause. Multiple back-edges is the
  largest BROAD class — 42 addresses over TWELVE cartridges — worth double the trip-count axis.
  `TestRefusalClassesAccountForEveryUnboundedAddress` fails when more than 12 unbounded addresses land in
  "other", i.e. when the prover grows a refusal this classification cannot name — which is exactly how a
  145-address class stayed unmeasured. Negative control: dropping the bank-switch case takes "other" from
  6 to 151 and the test fails by name.
- **The queue item that sent me here is retired by its own measurement.** "Attack the remaining 4.7 points of
  the trip-count axis" — measured, that axis has 3.2 points left in total (20 addresses), and two unmeasured
  classes are larger. The 4.7 came from the forcing experiment and 3.2 is the sole-blocker figure; both are
  true and both imply the same ordering.

### Fixed
- **`framegen`'s zone planner was refusing on a rule that was wrong about the target, not merely strict.** It
  required BACKGROUND-ONLY lines for a repositioning block, and every zone failure on the corpus reported
  `have=0` — real kernels draw something on every line. But a positioning block does not need a blank target:
  the replay loop is stopped during it, so GRP0/GRP1/PF hold what the last replayed line left, and the target
  matches exactly when those lines REPEAT it. At Fishing Derby's line-27 boundary the old test saw **0 usable
  lines where 7 exist**. `heldRun` states the real condition and generalises the old one (an all-background run
  IS a run of identical lines), pinned by `TestHeldRunGeneralisesTheBlankRule`. `blankLine`/`blankRun` were
  left with no callers and deleted.
- **A second candidate fix was measured and deliberately NOT built.** `zonePosLines` charges one line per
  PLACEABLE object rather than per object that actually moves at the boundary, so a one-object move pays for
  four. Every failure has `have=0` under the old rule, so cheapening the block would have unlocked **nothing**
  on its own — measured before writing it, which is the third time this session that a plausible fix was
  checked against the corpus first and two of those came back zero.
- **The wall that remains is named with its numbers, and it is not the predicate.** With `heldRun` the line-27
  boundary fits, and the picture is still unchanged, because `planZones` is ALL-OR-NOTHING per object: P1
  changes X at line 27 **and** at line 195, the second does not fit (1 holdable line against 6), so the object
  is dropped whole — giving up the **33-line band that was achievable** to avoid the 7-line one that was not.
  The fix is partial following, which is a change to the zone model rather than to a predicate (boundaries are
  global; "stopped following" is per object), recorded as RL-8c rather than attempted in a hurry.
  No regression: 22/31 technique ROMs pixel-exact, 31/31 at 262 scanlines.

### Fixed
- **`design-principles.md`'s three remaining ambiguity flags are resolved — two by finding the answer, one by
  admitting the claim is not checkable here.** They were left as `<!-- TODO: ambiguous original -->` comments
  on 2026-08-04 rather than guessed at, which was right; this is the pass that settles them.
  - **The colour-band rule welded two different numbers with an `=` that is false** ("PF-aligned colours come
    in multiples of 4 colour clocks (= 12px)"). They are separate quantities and BOTH are already
    machine-locked in this repo: WHERE a PF boundary may fall is 4 colour clocks (one PF pixel; 40 columns ×
    4 = 160), pinned by `TestEveryPlayfieldColumnLandsWhereTheTableSays` over all 20 column positions; HOW WIDE
    the narrowest band can be is 3 colour clocks per CPU cycle, pinned by `cmd/calibrate` at R² = 1.000000.
    They compose, which is what the original was reaching for: **12 is a multiple of 4, so a 4-cycle `STx.w`
    band is exactly 3 PF pixels and lands on the grid, while a 3-cycle `STA zp` buys 9 and cannot.**
    `TestPFAlignedBandsAreMultiplesOfFour` pins that consequence, so "use the absolute form for a PF-aligned
    band" is now a checked fact rather than a style note.
  - **The dangling "at line 38" was a line number into an earlier revision of the same file.** Re-pointed at
    the rule by name (the asymmetric-reflected-PF "PF2 at exactly cy45" deadline), with the note that line
    numbers do not survive editing.
  - **The Overscan/VBLANK surplus rule reads unambiguously in English already** — the Japanese original's word
    order was the problem — so nothing about the WORDING was left to decide. What is left is that **the claim
    cannot be verified here and is no longer implied to be**: "no picture" and "jitter" are behaviours of a
    real television, and Gopher2600 renders an over-long Overscan and an over-long VBLANK alike. The quantity
    this harness CAN see is the frame's line count, which is a different thing and is gated separately.
  Side effect: `ColorClockPerColumn` was one of the 13 exports measured as referenced by nothing; the new test
  reads it, because the alignment argument rests on it.

### Fixed
- **`framegen` was capping its kernel against a prover limitation that had been fixed four days earlier, and
  the fix turned out to be worth measuring rather than taking.** `kernProvedBlockCost = 8` carried the comment
  "the static prover cannot assume the alignment, bounds `lda abs,y` at 5" — true when written, and repaired in
  `pagePenalty` on 2026-08-01 (a base with a zero low byte cannot be crossed by an 8-bit index, no index
  analysis needed). Re-measured before touching anything: the eight-block Fishing Derby clone certifies at
  **worst 66 cycles = 3+7*8+7**, so the prover was already counting blocks at 7 and the constant was the only
  thing still counting them at 8.
- **Raising the cap to the true ceiling of 9 made the picture WORSE on Fishing Derby, which is why the count is
  now SEARCHED.** Nine blocks bought M1 outright (**0 of 43 cells to 43 of 43**) and cost 2,025 background
  cells, because the ninth slot let a playfield write be scheduled past the beam it governs — PF drew 8,829
  cells against a target of 6,888. Net element match fell **33,637 -> 31,680**. More blocks is not
  monotonically better, so the count joins the content shift, the VBLANK top and the frame length as something
  this tool calibrates against the target: plan at each cap, render, score, keep the best picture. Both
  candidates certify (66 and 73 cycles against 76), so the choice is purely about the picture.
  **Ties go to the smaller kernel** — same picture, 7 more cycles of headroom for the author.
- **Two ways this search could have been silently vacuous, both caught by measurement.** `planKernel` consumes
  the cap and runs ONCE before the search, storing its answer in `fd.kern`, so the first version re-emitted the
  already-chosen kernel and scored every candidate identically (31,680 six times) — the plan has to be redone
  per candidate, not just the emit. And a cap above what the target needs plans the same kernel, so candidates
  are now skipped unless they change the plan: two renders instead of six.
  The verdict line also conflated the cap with the kernel — it printed "chosen kernel blocks: 9" for `bullets`,
  whose kernel is 8 blocks at 66 cycles whatever the cap is, which reads as a regression that did not happen.
  It now reports both.
- **No regression on the corpus, measured rather than assumed**: sweeping all 31 technique ROMs gives
  **22/31 pixel-exact and 31/31 at 262 scanlines**, the same as the recorded figures, in 99s.

### Fixed
- **A past session implemented ARCADE Pong's numbers in a VIDEO OLYMPICS reproduction, and the guard against
  that is now mechanical.** There is no standalone Pong cartridge for the 2600 — Pong is one variant inside
  **Video Olympics (CX2621, 1977)**, which is what `sandbox/practice/pong` reproduces against the real ROM.
  The 1972 arcade machine is discrete logic and a different game. This repo's own primary source (a self-made
  distella + live-observation analysis) records BOTH, and even labels the arcade section "文脈知識" (context):
  **VO accelerates every 4/8/16 hits** via a mask table `$F6BC{$FF,$03,$07,$0F}`, +/-1 steps capped at +/-4;
  **the arcade accelerates at volley 4 and 12**, three speeds. The reproduction's comments cited
  "原典アーケード仕様" — the context section, implemented as if it were the spec — and the code matched
  **neither**: 4/8/**12** in four fractional tiers.
  Every "原典" label is gone; the source now names Video Olympics as the target, states VO's measured
  behaviour, and records 4/8/12 as a DELIBERATE deviation (it exists to kill a +100% speed shock) with the
  exact edit that would make it VO-faithful. Two further claims — `11点先取` and "loser receives" — are marked
  **unverified against VO** rather than left looking measured. No behaviour changed; both scenarios still pass.
- **`check_provenance.py` now fails if a blog note discusses the arcade machine without the
  target-confusion banner.** Comments rot and banners get dropped; this does not. All 12 arcade notes carry it,
  and the guard names the file when one loses it (negative control run). The invariant is also burned into
  `CLAUDE.md`, which is loaded every session: **where the arcade spec and a VO measurement disagree, VO wins.**

### Added
- **The blog mine's PONG core was the part that had never been distilled, and the harvest had been dropping
  every article body.** The ledger said "157 items mined". Measured: **155 directories — 117 distilled, 38
  raw-only, 2 completely empty.** Worse, every one of those 38 `entry.txt` files contained **only reader
  comments**; the article itself was missing, so distilling from them would have produced notes about what
  commenters said, attributed to the author. The page is Invision Community — body and comments are all
  `ipsType_richText` blocks and the FIRST is the body, which the original extractor skipped.
  **Nothing needed re-fetching: 26 of the 38 bodies were already on disk.** `reference/atariage/_tools/aa_blog_body.py`
  re-extracts them into BODY + comments, preserving the SOURCE/WAYBACK provenance lines.
  Dev-blog notes **117 -> 150**; `docs/mining-digest.md` regenerated from them, and this table's count in
  `CLAUDE.md` corrected with it.
- **What the 38 turned out to be is the finding.** They are almost entirely the PONG core: ball, paddle,
  score, collision. DanBoris's reverse-engineering of the **original arcade PONG circuit** gives mechanics
  ground truth in numbers, which is clean-room legitimate (a circuit analysis, not anyone's 2600 source):
  ball **4px x 4px**; horizontal motion is **per-scanline phase drift**, not a per-frame add (456/455/454/453
  counts against the line's 454); the paddle's **16px face is 8 regions of 2px** mapping to
  up-fast/medium/slow / none / none / down-slow/medium/fast, with hits on the top or bottom **reversing
  direction and keeping speed**; the vertical slip counter's load values **7..13 -> 248..242 counts against
  245 visible lines**, so **10 is exactly "no vertical motion"**; the ball **speeds up at volley 4 and 12 —
  three speeds, no more**; scoring is detected **not by collision but by "graphics present during HBLANK"**,
  because the ball is the only object that can leave the screen; and the game ends at **11 or 15** by switch.
  Also carried off: a **26-cycle, branchless, 22-byte hex->BCD 0-99** routine with its 7-byte table, and
  supercat's **26/32-cycle four-paddle read** plus the observation that **the paddle reset need not happen at
  a fixed point each frame** — time it so the timeout lands where the kernel can afford to poll.
- **Two entries disagree and the note says so instead of picking one.** `658` and `882` give opposite
  Aa/Bb-to-direction mappings, and `1744` is the author's own correction of `882` (he had `/HIT1` and `/HIT2`
  reversed — found by noticing the score was being credited to the wrong player, which is the same "check it
  against something downstream" discipline this repo runs on). Both are recorded, unresolved, to be settled by
  measurement when PONG is built.
- **The tail is now fully accounted for: 150 distilled, 5 gaps, 0 unexplained.** Six entries are marked
  deliberately out of scope with the reason (unboxings and collection posts), so they stop reading as unfinished
  work. The 5 remaining gaps carry a GAP.txt saying what is missing and why it is worth re-fetching — two of
  them (`8429-bounding-box`, `8431-pixel-perfect`) are the direct continuation of `684-collision-detection`,
  whose author had just measured pixel-perfect collision as too slow on the 2600 and fallen back to bounding
  boxes.
- **The Stella capture queue is empty for the first time since it was created.** Captured during a window when
  the author was away from the machine, which is the only time it may run — the write-only TIA registers live
  in Stella's debugger GUI, so each capture takes the screen for ~13s. `litmus_jsr_stack`, `framelines_trap`,
  `framelines_clean`, `litmus_divpre` and `litmus_divctx` each agree with Gopher2600 on **37/37** write-only
  registers at frame 5. Four of those are this session's new witnesses, so the fixtures now gating frame
  stability and the divide-loop bound are themselves cross-checked against the reference emulator rather than
  trusted. Corpus: **161 captured ROMs, 5,957 register readings.**
- **`zone_multiplex` was the flagship technique scored by a hash alone; it now states what it claims.**
  Its scenario held `ntsc_frame_lines`, `frame_lines_stable` and `golden_frame` — nothing that says the
  technique WORKS. A golden hash fails identically whether a zone moved one pixel or the multiplexer collapsed
  to a single sprite. Added 22 behavioural assertions derived from what the ROM claims (6 zones x P0/P1 = 12
  sprites on a 2-sprite machine, P0 drifting right and P1 left, X wrapping at 128 via `and #$7F`):
  **12 asserts** pinning every zone's X at frame 12 — measured, not derived, and all twelve values are
  distinct, which IS the technique; **6 invariants** that each X stays in 0..127 over 60 frames (a mask
  regression); **2 monotonic** (P0 zone 0 up, P1 zone 5 down — the movement claim, over a window chosen to
  precede either one's wrap); and **1 temporal** `eventually ram.0x84 < 10 within 20` for the wrap itself,
  which is a sequence property no per-frame invariant can state (held at frame 14).
  Negative control: reversing P0's drift to `sbc #1` fails three independent ways —
  `monotonic ram.0x80 up [broke@frame 1: 17->16]`, the pinned values, and the temporal wrap.
  The golden was regenerated because `frames: 60` lengthens the observed window; the ROM is untouched and its
  rendering byte-identical, verified by re-running the original short window against the OLD golden first.

### Fixed
- **K6 — the divide loop's entry value is now the CONTEXT's, and pizza-boy certifies. The umbrella's
  known-failing list is empty for the first time: 16 of 16 scenarios pass, 0 known-red.**
  `absStates` is keyed by SITE, so A at a shared routine's loop header is the join over every caller.
  `pizza_boy.asm` calls `SetXPos` five times — twice from VBLANK with a sprite coordinate out of RAM (Top)
  and three times from the HUD with a compile-time constant — so the constant callers inherited Top, the
  divide was bounded from the 255 floor at 19 iterations against the 6 they can reach, and the region was
  reported as a visible-line overrun on a path that cannot happen.
  **Full context-sensitive abstract interpretation was NOT needed**, which the previous entry had already
  measured: the region walk is per-context already. It knows its return site, the calling JSR sits at ret-3,
  and when nothing between the callee's entry and the loop header can touch A — `sec` and a WSYNC store, for
  the whole positioning idiom — the accumulator at the CALL is the accumulator at the header, exactly, for
  that context. `preservesA` is a whitelist and `accumulatorSurvives` refuses on anything it cannot prove
  (a nested JSR, an unresolvable successor set, a step limit); the flat walk passes -1 and is unchanged.
- **The per-context display classification, previously measured as a no-op and reverted, is back — and this
  time it is load-bearing.** It was a no-op *because* loop bounds were context-insensitive: every context
  cost the same, so ranking visible above blank could not change a verdict. With each context carrying its
  own entry value they differ, and pizza-boy needs both halves. Verified by removing them one at a time:
  **disable either and `litmus_divctx` is NOT CERTIFIED.**
- **Witnessed in the corpus, because the corpus did not witness it.** `divCtxEntryUsed` — a counter added for
  exactly this question — read **0 over 164 ROMs**: `litmus_divpre` has one call site, so the per-context walk
  never runs on it. `roms/litmus/litmus_divctx.asm` is the missing shape: one `SetXPos`, called from VBLANK
  with a RAM byte and from the visible region with a constant. It certifies at 62cy with K6 and fails without
  either half, and the counter now reads 1. Negative control on the guard: a callee that reloads A from RAM
  before the loop is refused and falls back to the 255 floor (`OVER 122>76`).
  Soundness re-graded with both changes in: **observed <= proven on 1408 regions across 173 ROMs, no
  exceptions.**

### Added
- **A corpus-wide frame-stability gate, because the scenarios cannot reach most of the corpus.** Measured:
  **36 of 164 ROMs carried `frame_lines_stable`, 128 carried nothing.** Wiring 128 scenarios by hand is more
  work and more to maintain than one sweep, so `TestNoRomBreathesAcrossFrames` walks `roms/**/*.bin` and
  requires every ROM to hold ONE frame length for 130 frames. **162 swept, all pass, 19s** — the serial sweep
  was 76s, which is why this was not added before; `t.Parallel()` per ROM is what made it affordable.
  The invariant is **single-valued, not 262**: 38 ROMs hold a deliberately different frame length and are
  stable at it, and a gate demanding 262 would fail them all and teach everyone to ignore it.
  Negative control: un-excluding `framelines_trap` fails it with `262x129 263x1`.
- **The gate found one ROM on its first run — `cart_f4sc`, in `roms/carts/`, which the earlier 156-ROM sweep
  never covered.** It is a bank-switch/superchip FINGERPRINT fixture, not a display ROM: all eight banks end
  `lda $FFF4 / jmp .reset`, handing back to bank 0 and re-entering the reset vector, so the machine
  ping-pongs between banks instead of driving frames. Over 130 frames it never produces a 262 at all
  (`1x1 2x1 3x1 4x4 … 350x22`). Excluded BY NAME with that measurement, alongside `framelines_trap`; its four
  siblings (`cart_3e`, `cart_3eplus`, `cart_dpc`, `cart_f6sc`) do carry frame loops and are swept normally,
  so this is one fixture's shape and not a blanket exemption.

### Fixed
- **`objIndex["M1"]` was asserted by nothing, and it is the one entry that could have been wrong.**
  `spritepos.Achieve` pokes the INDEX into the kernel and reads the object back BY NAME, so that loop is what
  verifies the table against the machine — and `TestSolveHitsTargets` ran `P0, P1, M0, BL`, leaving index 3
  unexercised. Added `M1`. Negative control: setting it to 1 fails with
  `M1 target 12: achieved 25 (inputA=67, off by +13)`.
- **D5 — "two incompatible object-index orderings" is NOT a defect, and the inventory called it the sweep's
  highest-consequence duplicate.** `emu.DrawnObjects`'s `P0,M0,P1,M1,BL` exists to line up with the
  `Markers()` literal in the same file and indexes nothing else; `spritepos.objIndex`'s `P0,P1,M0,M1,BL` is
  the TIA register layout ($10..$14 / $20..$24). They index different things and never meet. What was real is
  that neither was pinned: `drawnobjects_test` restated the order instead of reading it back, so it agreed
  with `idxOf` even if `idxOf` had stopped agreeing with `Markers()`. It now DERIVES the labels from
  `Markers()`, and both declarations carry a comment naming the other as the thing they are not.
- **Deleted `design.MaxMultiSprite` and `FitsMultiSprite`: the cited thread does not say what they claim.**
  The constant was 5, documented as bB's multisprite/flickersort ceiling and cited to AtariAge 107063.
  That thread never mentions bB or the number 5 — it is a 2007 raw-assembly discussion of Venetian blinds for
  chess pieces, and its actual numbers are "8 distinct sprites per line is infeasible" (supercat) and
  "4 pieces per line by rewriting GRP0/GRP1 mid-line" (hornpipe2, measured). It was also numerically identical
  to `MovableObjects = 5`, so nothing about the value distinguished the two concepts, and the test asserted
  only that the definition equals itself. A bB-derived ceiling is also the one thing this project has decided
  on purpose not to inherit. `gen_mining_digest.py`'s 107063 mapping updated to stop naming a dead symbol.
- **`design.VividMaxLuminance` keeps its value and loses its false provenance.** It is NOT dead — it is the
  threshold behind `WashoutRisk`, which the docs do reference. Thread 132561 establishes the phenomenon
  (luma desaturates on real hardware, NTSC's I/Q ceiling is why, blues 7-9 stop separating at the top) and
  advises "mid-to-low luminance"; **it never states a threshold.** The 5 is this project's cut, now said so
  in the comment instead of looking measured. The "unit-ambiguous" charge does not survive checking:
  `Luminance()` returns `(reg>>1)&7` and `WashoutRisk` takes its argument from there, so 5 is step 5 of 8.
- **`pkg/design` has an importer, contrary to the inventory.** `internal/cyclebound` imports it, and 25 of its
  38 exports are referenced from code or docs. The package's contract is the routing table's step 2b —
  executable feasibility checks run during authoring — not Go-level reuse, so "N exports, 0 importers" was
  the wrong measurement to act on. The 13 unreferenced exports are hardware facts that are correct
  (`ColorClockPerColumn`, `PixelsPerCycle`, `LuminanceLevels`, …); deleting right constants is not the goal,
  and they are left alone.
- **`pkg/sprite.DigitFont()`'s digit 9 was upside down, and the doc comment that would have caught it is now
  the test.** The comment claims the glyphs are identical to the `score6` technique's and that they are
  returned top-first, while `score6.asm` stores them bottom-first for a `Y=7->0` kernel — so the two tables
  must be exact reverses. Derived independently from `roms/techniques/score6.asm:193-202`: digits 0-8 match
  perfectly, **digit 9 held score6's raw bottom-first bytes**, so read as documented it draws a 9 with the
  bowl at the bottom — which reads as a 6. Corrected to `{0x78,0xCC,0xCC,0xCC,0x7C,0x0C,0x18,0x70}`.
  Nothing could have noticed: `pkg/sprite` has no importer, so no ROM renders it, and `TestDigitFont` only
  asserted low-2-bits-clear and not-blank, both of which a flipped glyph satisfies. `TestDigitFontMatchesScore6`
  now PARSES the `.asm` (rather than restating its bytes, which would be a third copy to drift) and asserts
  the reversal digit for digit. Negative control: restoring the old bytes fails it with
  `digit 9: DigitFont()=70180C7CCCCCCC78, reversed score6=78CCCCCC7C0C1870`.
- **Deleted `cyclebound.Lint`, a caller-less wrapper whose only effect was discarding a denominator.**
  `cmd/timinglint` and both test helpers go through `LintDetail`; nothing called `Lint`. It returned the
  warning list alone, throwing away `Banks`/`Instructions`/`PerBank`/`Declined` — and an empty warning list
  means either "clean" or "nothing was analysed", which are opposite answers. A signature that cannot tell
  them apart is the defect this package keeps finding elsewhere. `CLAUDE.md`'s pointer updated with it.
- **The divide loop's entry value now crosses the region boundary — and the backlog item that sent me looking
  was wrong in both of its premises, which is the more useful half of this entry.** The board said the prover
  mislabels pizza-boy's `SetXPos` as `kind=visible` because blank classification does not cross a JSR.
  - **It is not the JSR.** The same 116cy refusal reproduces with the loop INLINED and no call anywhere.
  - **`kind=visible` is not a mislabel.** `SetXPos` is called twice from VBLANK *and three times from the
    HUD*, which is visible (`pizza_boy.asm:320,336,339`). A visible context genuinely exists.
  What the probes actually found: two ROMs with identical machine behaviour get opposite verdicts depending
  on which side of a WSYNC a constant is written on. `lda #78` before the region's opening WSYNC → **122cy,
  NOT CERTIFIED**; the same `lda #78` after it → **60cy, CERTIFIED**. `determineBound`'s predecessor scan ran
  over the region subgraph only, and the ordinary positioning idiom (`lda #TARGET_X` then `jsr SetXPos`, whose
  first act is `sta WSYNC`) puts the deciding instruction on the far side of that boundary — so the scan found
  only the latch, which it correctly excludes, and fell through to the 255 floor: 19 iterations for a value
  of 78. Widened to the whole decoded program. Sound by construction (a maximum over MORE predecessors cannot
  be smaller) and graded: **observed <= proven on 1399 regions across 172 ROMs, no exceptions.**
- **The corpus does not witness that fix, and saying so is the point.** Sweeping all 157 ROMs before and
  after, **ZERO changed their proven bound** — every real kernel that shares a positioning routine passes it a
  RAM byte (Top whatever the scan sees) or is refused earlier for an unrelated reason (`shared_setxpos` dies
  at "no WSYNC reached from region start"). A fix whose only demonstration is a throwaway probe is one nothing
  would notice losing, so the probe is checked in as `roms/litmus/litmus_divpre.asm` + `scenarios/divpre.json`:
  CERTIFIED at 62cy with the change, `OVER 122>76` without.
- **A second change was written, measured, and thrown away.** Classifying each call CONTEXT's display state
  from its own call site — rather than from the callee's entry state, which is the join over all callers and
  therefore unknown the moment one caller is visible — is correct in principle and is exactly what "the blank
  classification does not cross a JSR" should have meant. Measured over 157 ROMs: **102 certified either way,
  and not one proven bound moved.** It cannot differ: the join is only unknown when some context is visible,
  and a visible context outranks a blank one in the verdict regardless. Reverted rather than shipped as
  unwitnessed complexity.
- **pizza-boy's `prove_line_budget` is still red, and now for a precisely stated reason.** The remaining
  imprecision is that loop bounds are **context-insensitive**: A's range at the divide header is the join over
  all five call sites, two of which pass a RAM byte, so the HUD contexts inherit 19 iterations even though
  they pass compile-time constants. Fixing that means context-sensitive abstract interpretation, which is a
  project and not a patch. `@amax` does not substitute for it — one declared ceiling cannot say "160 here and
  78 there".

- **All 4 breathing ROMs repaired: the corpus is now 156/156 single-valued, 0 breathing.** Every fix is the
  same lesson stated twice, because the two halves of the corpus broke it in different ways.
  - `banked_game` (+2 lines every 120th frame) and the two `lint_bank_*` fixtures (+3 every 120th) do their
    switch work AHEAD of a fixed `ldx #37 / sta WSYNC` loop, so the overflow was ADDED to the frame instead of
    absorbed by it. Fixed by paying the switch path's extra lines on BOTH paths and reducing the loop by the
    same amount — the frame is 262 whatever the counter does. Proved confined: the first 100 frames (the
    switch is at 120) hash IDENTICALLY before and after, so only the switch frame changed, and
    `banked_game.golden` was regenerated on that basis. Negative control: the pre-fix ROM against the new gate
    gives `262x199 264x1`, while `ntsc_frame_lines` stays green on the same run.
  - `exerciser` (+2 every 64th frame) was a different fault wearing the same symptom, and the first guess —
    the missile-fire path — was **wrong**: with no input the ROM never leaves scene 0, so that code never
    runs. `profile_line_budget` named the real lines instead of arguing about them: `$F1A0` **79cy** and
    `$F16E` **77cy**, both worst at **frame 65**, both spilling into 2 physical scanlines. The listing maps
    them to the ch1 and ch0 music ticks — each note-change path overran 76 by 3 and 1. Fixed by hoisting the
    constant `lda #8 / sta AUDVx` out of both note-change paths into the Title scene's entry-time music init
    (-5cy each): re-profiled at **74cy and 72cy, `worst_lines: 1`**, whole-ROM worst 74.
    The melody is intact — every audio assertion in `m7_music` still passes (freq 14/11/9/23/14, volume 8),
    and `audiospec` over 200 frames / 104,327 samples reports **identical dominant frequency on both
    channels** (1308.25 Hz ch0, 31.38 Hz ch1) with envelope distance 0.0006/0.0007. The residual spectral
    distance of 0.091 (ch0) and 0.0022 (ch1) is a sub-line sample-boundary shift, not a note change — the
    tool's calibration point for "a completely different pitch" is 0.998. 6 of the 7 exerciser goldens are
    byte-identical after the fix; only `m7_music`'s two were regenerated.
- **`frame_lines_stable` wired into everything that can carry it**: all 31 technique scenarios (banked_game
  included now that it passes), all 7 exerciser scenarios, and two NEW scenarios for `lint_bank_hazard` /
  `lint_bank_split`, which had no scenario at all and so had nothing standing between them and a regression.
  `TestLintReadsBothBanks` still pins their warning sets, so the padding did not disturb what they exist for.
- **The corpus-wide gate now needs no exclusion list** — that was the reason not to add one, and it is gone.
  What remains is only its cost: a 156-ROM sweep at 130 frames is **76s**. Still not added unilaterally.

### Added
- **`frame_lines_stable` — the ∀-over-frames sibling of `ntsc_frame_lines`, because sampling one frame is not
  a claim about the frame after it.** `ntsc_frame_lines` calls `StepFrame()` exactly once, so a ROM whose
  frame total *changes* between frames passes it whenever the sampled frame happens to be the right length.
  That is not hypothetical: the pizza-boy reproduction renders **261 lines on 482 of 600 frames and 262 on
  117** (plus a 40-line boot frame) while the original it reproduces holds **262 on 594 of 598**, and it
  passed every check in the suite for as long as it existed. The frame length tracks sprite X — the
  divide-by-15 positioning loop costs a whole extra line past X=105 (=7x15) and that cost sits **outside** the
  region whose length is fixed — so on a CRT the whole picture steps up and down by a line. No golden hash can
  see this: the hash is over rendered frames, not over their heights.
  The new check steps N frames, histograms the per-frame scanline count, and passes only when every frame
  agrees (`"frame_lines_stable": {"frames": 130, "lines": 262}`; `lines` optional). Verdicts print the whole
  histogram — `262x129 264x1 (2 distinct; first change at frame +116)` — so the measurement is in the output
  either way. Continues on the emulator the timeline drove, so a scenario that holds an input and then checks
  stability measures the played state.
- **Sweeping this repo's own corpus found the same defect in 4 of 156 ROMs, so the check ships with real
  witnesses rather than a synthetic fixture.** 130 frames after a 3-frame warmup, every `.bin` under `roms/`:
  **152 stable at 262, 4 breathing, 0 errors.** `roms/techniques/banked_game` 262x129 / **264x1**,
  `roms/exerciser/exerciser` 262x128 / **264x2**, `roms/litmus/lint_bank_hazard` and `lint_bank_split`
  262x129 / **265x1**. Every outlier is exactly periodic — banked_game at frames 120/240/360/480 over a
  500-frame run, matching the `cmp #120` level switch declared in its own source — and the cause is
  pizza-boy's: `banked_game.asm:51-63` does the cross-bank level load **ahead of** its fixed 37-line `ldx #37 /
  sta WSYNC` loop, so the switch frame's extra work leaks into the frame total instead of being absorbed by it.
  These four are left RED-capable and unfixed pending a decision; `banked_game.json` is the one technique
  scenario not given the check.
- `TestFrameLinesStable` locks four directions against real ROMs: a fixed-structure kernel passes,
  `banked_game` fails over a 130-frame window, **the same ROM passes over a 60-frame window** (the window is
  shorter than the 120-frame period — the vacuity is measured, not merely warned about, so nobody later
  "fixes" a red check by shrinking the window), and stable-at-the-wrong-number fails a declared `lines`.
- Wired into **30 of the 31** technique scenarios (all but `banked_game`) at `frames: 130`, the reference
  kernels being what `docs/authoring-protocol.md` step 3 tells an author to clone. Cost measured:
  `go run ./cmd/scenario roms/techniques/scenarios/*.json` **14s -> 29s**.

### Changed
- **DOC-EN closed: the canonical docs are English, and the count says how much was left rather than claiming
  "done".** The 2026-06-17 cleanup dropped the 13 `.ja.md` duplicates but left the Japanese *bodies* of three
  canonical docs untouched, because they were the only copy. Measured over `docs/**/*.md` (excluding `*.ja.md`
  and `mining-digest.md`): lines carrying Japanese **script** **210 → 5**, lines carrying **any** non-ASCII
  character **2491 → 2427**. `design-principles.md` 105 → 4, `casebook.md` 58 → 0, `build-to-learn.md` 35 → 0,
  `capability-gap-audit.md` 7 → 1, `fundamentals-audit.md` 2 → 0, plus one line each in `verified-coverage.md`,
  `cookbook.md` and `techniques/multicolor48.md`.
- **The non-ASCII residue is not a shortfall.** 2427 lines still hold a non-ASCII character and every one of
  them should: em-dashes, `★`/`⚠`/`✅`, `≈`/`÷`/`§`/`⅔`, and the `〔…〕` provenance bracket that
  `scripts/check_provenance.py`'s `MARKERS` regex matches on. Translating `出典` to `Source:` keeps the marker;
  deleting the bracket would fail the gate. Provenance resolves the same citation set before and after
  (61 skipped for the absent umbrella on both runs), so no citation was broken or invented.
- **Five Japanese lines are kept ON PURPOSE, and each one is a quotation.** Four are
  `<!-- TODO: ambiguous original: … -->` comments in `design-principles.md` where the source sentence
  contradicts itself and a confident English rendering would have invented a measurement: the colour-band
  minimum width ("4 colour clocks" vs "= 12px, `STx.w`" — 4 colour clocks is 4px), the bare "line 38"
  back-reference next to the cy45 write deadline, the Overscan-vs-VBLANK surplus rule (reads as "not Overscan
  … do not absorb in VBLANK"), and the pixel aspect (`≒ 1/2` says tall, `≈ 2:1` and the ⚠ note say wide).
  Translated literally and flagged; all four remain open for a measurement pass. The fifth is
  `capability-gap-audit.md`'s verbatim quote of `banked_game.asm:110`, where the claim *is* about that exact
  line's bytes. `mining-digest.md` stays excluded — it is generated from Japanese-source thread data and
  translating it would break source fidelity.

### Fixed
- **"The analysis cannot pin A" is not "A has no bound".** The `sec / sbc #N / bcs` divide's trip count comes
  from the accumulator entering the loop, and `determineBound` refused whenever any predecessor carried a Top
  A — correctly, since SD-9's proxy guessed one and under-approximated by 40x. But refusing EVERY unpinned
  entry conflates *"this analysis does not know the value"* with *"this value has no upper bound"*, and the
  second is false about a 6502 accumulator: **A is eight bits, so it is at most 255** — a fact about the
  hardware, not a range inferred from the program, which is why it does not reopen the door SD-9 closed (that
  failure was reading a number off the wrong INSTRUCTION, not reading a register's width off the datasheet).
- **Found on the project's own ROM, as the second half of a failure whose first half was fixed the same day.**
  `pizza_boy.asm` positions sprites through `lda px / jsr SetXPos`, where `SetXPos` opens with `sta WSYNC` and
  divides A by 15. `px` is a RAM byte, Top by construction, at **all five call sites** — so every call context
  died on this line and the region came back **"no WSYNC reached from region start"**, a symptom four steps
  downstream of its cause.
- Corpus: **4 regions gained, 0 lost** (all four in Panda Chase), and `TestProvenWorstIsNeverExceededOnCorpus`
  stays green. `litmus_amax_floor.asm` grades the new bound against the machine (proven 103, machine 23) and
  carries the control that matters: a row whose A the scan CAN pin must stay **tighter** (43), or the floor has
  replaced the scan rather than standing under it. Both negative controls fire.
- **`@amax` did not lose its point — the refusal became an overrun.** `cb_blank_noamax` used to be *unbounded*;
  it is now bounded at **107 cycles against a 76-cycle budget**, while its annotated twin proves **67** and
  fits. "This line runs 107 cycles" is a fact a builder can act on; "I cannot tell" is not. The witness test
  now pins that the annotated twin must be strictly tighter.
- **The fixture took three attempts, and the failures are recorded in it.** `lda SWCHB / and #$0F` does not
  produce a Top accumulator — the interpreter follows the mask and knows [0,15]. Two different `sta $90` on
  the arms of an undecidable branch does not either — it joins them into [7,200]. Both were bounded BEFORE
  the change and would have proved nothing. The value has to come from hardware that moves on its own, so the
  row reads INTIM while the RIOT is counting.

### Added
- **G9 closed — the two craft patterns a reconstruction missed now have fixtures, graded tests and docs.**
  The Fishing Derby casebook recorded that Claude's sealed rebuild had no counterpart for (a) reshaping ONE
  player per scanline into an irregular silhouette wider than 8px, and (b) drawing an arbitrary-angle 1px
  line with a missile or ball. Both were prose in a gap list; neither had a ROM, a number, or a doc.
  New: `roms/litmus/litmus_nusiz_shape.asm` + `internal/emu/nusizshape_test.go` +
  `docs/techniques/nusiz-shaping.md`, and `roms/litmus/litmus_hmove_slope.asm` +
  `internal/emu/hmoveslope_test.go` + `docs/techniques/hmove-slope.md`.
- **The intended shape is stated where the harness cannot reach it, because grading output against output
  is worth nothing.** (a) states the silhouette as a table of drawn runs, generated in the test from the
  band table plus two hardware rules — the drawn pixels match on **40 of 40 scanlines**, 840 px of ink
  against 840 intended, plus **120 of 120** control rows. (b) states an equation,
  `x(n) = x(0) ± floor(n·NUM/256)`, and the drawn x of the 1-pixel object matches it with **max error 0 px
  over 160 scanlines**, on two slopes in opposite directions (3/8, and 85/256 chosen precisely because it is
  not dyadic — that angle is unreachable by any doubling scheme).
- **(a) is graded a second time with no table at all.** The 40-line kernel runs four times over the same
  tables with only two zero-page masks changed, so the shaped block must equal the NUSIZ-only block
  translated by the HMOVE-only block's displacement. It holds on all 8 bands, and it catches the two axes
  interfering — which no comparison against a table can.

### Fixed
- **`sta HMOVE` at CPU cycle 10 instead of cycle 0 adds a phantom +1 clock per line, with `HM=$00`.** Found
  in the first build of `litmus_nusiz_shape`: 39 clocks of drift over 40 lines, under a silhouette that
  still looked plausible band by band, and it corrupted every slope in the fixture equally. **A slope graded
  relative to its own first line cannot see it** — only a deliberately motionless object can. Both fixtures
  now carry one (block 3 of the shape ROM, M0 of the slope ROM), asserted static on 40/40 and 160/160
  scanlines. The rule is now written down in both technique docs rather than living in the kernel's shape.
- **Five negative controls run and reported, not assumed.** Deleting the single `sta NUSIZ0` → 40 matching
  scanlines becomes 5; zeroing one HM table entry → 15; `$60` → `$61` in the accumulator, a one-bit change →
  max error 1 px, 120 of 160 exact, travel +60 against +59; giving the static control a move → 157 of 160
  lines moved; breaking the width oracle → the table tests stay green and the metamorphic relation fails on
  7 of 8 bands, which is the case that relation exists for.
- **Cross-checked against the original cartridge by pixels, never by disassembly.** `emu.DecomposeRow` on
  Fishing Derby (umbrella-only, absent from CI): P0 drawn on 103 scanlines with **13 distinct per-row ink
  widths**, reaching a **28-clock extent on one line out of an 8-bit graphics register**, the copy count
  changing inside four scanlines and the left edge stepping 44 → 43 → 42; and the right-hand line drawn as
  the **ball over 110 consecutive scanlines at −0.0826 px/scanline**, holding each x for 8 to 15 lines
  rather than on a fixed period — the observable signature of an accumulator, not a divider.
- **A hardware rule the shape table would otherwise have had to invent, re-measured on the fixture it came
  from.** Double and quad width start ONE clock later than the 1x modes (`litmus_nusiz_all`: modes 0-4 and 6
  ink from clock 24, modes 5 and 7 from clock 25). `TestNusizWidthModesStartOneClockLate` re-measures it on
  that ROM every run, so this package's generator cannot quietly acquire a rule only it believes.
- **The provenance check then turned CI red for the SAME reason, twice more.** Resolving citations against
  both roots is right on a machine that has both; a CI checkout has only one. Three rounds of it: (1) 11
  citations into `sandbox/` and `reference/`, which are the umbrella's and are never fetched; (2) cited
  `.bin` files, which are gitignored build products absent from any fresh checkout — a cited `.bin` now
  resolves through the `.asm` beside it, since the source is what a reader can follow; (3) `scripts/`, which
  exists on BOTH sides, so keying on the root prefix was not enough. The rule is now: **when the umbrella is
  absent, anything the harness alone cannot resolve is COUNTED and passed over**, because "does not resolve"
  then carries no information — it cannot distinguish a broken trail from an unfetched tree. The count is
  printed (`58 citation(s) NOT checked`) so a run that verified everything and a run that skipped a third of
  it do not look the same.
- **Verified by cloning the repository outside the umbrella and running the real CI steps there** — assemble
  150 ROMs, `go vet`, `go test -p 1 ./...`, the five gates, and the 102 scenarios — rather than by running
  the same commands on a machine that has everything. That is the check the phrase "CI mirror" was standing
  in for, and it was not the same check.
- **The coverage test turned real CI red while the local "CI mirror" stayed green.** `TestProverCoverage...`
  demanded 16 commercial cartridges unconditionally, and those cartridges live in the umbrella `reference/`
  tree for licensing reasons — a GitHub Actions checkout has **none**. The very first push after it landed
  failed (`run #551`, `b2f2584`), and it was the USER who noticed, from the Actions page, while this session
  had reported "CI mirror green" four times. Running the same commands on a machine that HAS the corpus is
  not the same check: it is a proxy, and this is the proxy-versus-artifact mistake the project keeps writing
  down, committed by the file whose whole purpose is to measure coverage honestly. The test now follows the
  rule the rest of the package already used — **zero cartridges is a different environment (skip, loudly,
  naming why); any other shortfall is a corpus that shrank (fail)** — and both branches are verified by
  hiding the corpus and re-running.

### Fixed
- **A branch whose flag is already decided has ONE successor, and walking the other arm decoded a data table
  as instructions.** `collectRegion` took both arms of every branch and `longest` costed both. Found on the
  project's own ROM rather than on the corpus: `pizza_boy.asm` has `lda #0 / sta Dx / beq .cexit` — Z is 1 by
  construction and a store leaves the flags alone, so the fall-through cannot happen — immediately followed by
  the `Alley3A` snap table. Collection took the impossible arm, decoded the table, and the `$00` at **$F490**,
  a byte of level data, decoded as **BRK**. The region was refused for "BRK in region": an instruction the
  machine never executes, at an address that holds graphics. **That refusal is what made the project's own
  `phase0` scenario fail.** Both `collectRegion` and `longest` now apply `refineBranch`, the same test
  `absSuccessors` has always used, which prunes only when the flag is KNOWN.
- **The prune had to go in BOTH passes or they disagree.** Adding it to collection alone cost **5 proven
  regions** (M.A.S.H. $F126, Bermuda Triangle $F4F8, Star Wars $F649 and its bank-1 image, Planet of the Apes
  $F86C) — not because pruning was wrong but because `longest` still asked for the cost of an arm collection
  had left out, and a walk into an uncollected site reports the whole region unbounded.
- **Corpus effect over 626 addresses: 0 lost, 25 bounds LOWERED, 10 raised** — and the standing gate
  (`TestProvenWorstIsNeverExceededOnCorpus`, proven vs machine over the whole corpus) stays green, so the
  lowered bounds are over-approximation removed rather than soundness lost.
- **The BRK refusal now names its address.** It read exactly "BRK in region" with no location; the region it
  belongs to began at $F075 while the BRK was at $F490, four hundred bytes away. Diagnosing it meant
  instrumenting the prover and re-running.
- **The prune closed the only known route to `determineBound`'s "predecessor with no abstract state" guard**,
  which `cb_deadpred.asm` existed to witness. That route was the pruned arm itself. The guard is kept — a
  missing state entry means either proven-unreachable (which the prune now removes earlier, correctly) or
  never-analysed, and `computeStatesWith` can still return `converged=false` with work outstanding — and the
  fixture now records the closure and says out loud that the guard is unwitnessed, the same treatment
  `overlaps` and the body-range check received.

### Added
- **G3 — a PCM speech stream is now graded on TIME as well as on VALUES, and the harness had only ever
  graded values.** The mined recipe (topic/234209, iesposta + spiceware) parks AUDC and writes the 4-bit
  volume register AUDV0 as a DAC at a fixed rate (3900–4000 Hz for voice) from samples packed two nibbles
  per byte — and its stated failure mode is temporal: the older Berzerk speech hack made the TV **roll**
  because the playback loop ate the scanline budget. Measured before this change, on a 144-sample/frame
  fixture: `read_audio` yields **1 reading of 144 (0.69%)**, `read_audio_trace` **1 per frame**, and across
  the whole 150-ROM corpus traced 5 frames each **only 5 ROMs wrote AUDV at all**, the maximum being 4
  writes in one frame — **nothing in the corpus emitted a per-scanline stream**.
- **The pre-existing raw audio path was not blind to the samples — it was blind to their time, and the way
  it recovered them is why.** `emu.EnableAudioCapture` (524 samples/frame) does contain all 144 values, but
  only findable by searching **236 offsets × 2 phases** for the best fit, and the same search fits a stream
  shifted by a whole scanline **equally perfectly, 144/144**. A fitted anchor absorbs exactly the drift the
  check exists to find, so `internal/pcm` grades against a **declared** slot grid instead.
- **Two independent axes, one denominator.** VALUE pairs the k-th write with the k-th intended sample (a
  uniform shift cannot move it); TIMING compares the absolute scanline with `StartLine + k·pitch` (a
  corrupted value cannot move it); a clock histogram catches a write that wanders inside its line. New:
  `internal/pcm`, `cmd/pcmcheck`, fixture `roms/litmus/litmus_pcm.asm` (144 samples/frame, one per
  scanline, high nibble first, first sample on scanline 37, 262 lines) + its scenario. The intended
  waveform is **parsed out of the ROM's own source** between `; PCM_TABLE_BEGIN` / `; PCM_TABLE_END`, so
  player and grader read the same bytes and a typo in either cannot cancel out.
- **Fixture result: 3/3 frames `144/144 captured, 144/144 values exact, 144/144 in slot, mean pitch 1.000
  lines/sample, all writes at beam clock −23`. Every negative control fired.** One-line shift →
  `0/144 in slot` with values still `144/144`; dropped sample → `143/144 captured, 63/144 in slot`; one bad
  value → `143/144 values, 144/144 in slot`; drift of one line per 32 samples → `32/144 in slot, mean pitch
  1.028`; intra-line jitter → 2 clock buckets, other axes clean; nibble order swapped → `96/144 values
  differ, 144/144 in slot`. Two controls are **ROM-level**, assembled from a rewritten copy of the fixture
  rather than a doctored capture: an extra `sta WSYNC` per loop → `1/144 in slot, mean pitch 1.503` with
  values still perfect, and `PACKED = 71` → `142/144 captured`.
- **One control did NOT fire, and it defines what this is not.** Corrupting a byte of the fixture's sample
  table (`$FF` → `$F1`) still graded `144/144`, because the table IS the declared intent — editing it moves
  the ROM and the expectation together. So this grades *"does the ROM deliver the waveform it declares, in
  time"*, never *"is that the right waveform"*, and a ROM-level value defect has to break the **player**:
  narrowing the low-nibble mask to `and #$07` gives `107/144 values exact with 144/144 still in slot`.
  The controls are themselves witnessed — forcing `LineError` to 0 in the grader turns **4** tests red,
  forcing every value to count exact turns a different **4** red, and the positive fixture test was seen
  red twice.
- **The AtariAge queue was already empty, and the threads worth having were in the reject pile.** The roadmap
  still said 761 queued / 212 mined; measured against the filesystem — a thread counts as mined when
  `reference/atariage/<id>-<slug>/notes.ja.md` exists — **761 of 761 were done**, and the 850 mined
  directories exceeded the queue because 89 predate it. What remained unmined was one directory,
  `255863-wip-battle-pong`, abandoned after page 1. It is the closest thing in the corpus to the PONG
  capstone, and it carries the finding the milestone most needs: **`LDA (ptr),Y` costs 6 cycles instead of 5
  across a page boundary**, which in a 2-line kernel lands as a late `COLUPF` write or a `WSYNC` at cycle 74,
  i.e. a scanline that appears only on some rows. The fix is to move the data, not to buy the time.
- **Eleven more threads, chosen by the milestone rather than by the old triage.** The queue being exhausted,
  the remaining seam was the REJECT bucket, re-read for PONG mechanics (score kernels, paddle, ball/missile,
  collision, 2LK). **Three of eleven overturned the rejection**: `145747` — filed as "one-off collision debug",
  is a 6502 Pong whose answer is that **the collision registers latch and must be cleared with `CXCLR`**,
  the value written being irrelevant; `291730` — the BCD carry chain for a 6-digit score, where the upper
  bytes take `adc #0` and nothing else; `283840` — filed as "Game WIP", the most-viewed thread in forum 50,
  which shows that **strobing `HMOVE` during VBLANK removes the left-edge comb entirely**, freeing the Ball
  object that is otherwise spent smearing over it — for PONG, that is the ball. The other eight confirmed the
  triage and were distilled anyway, because a thread with no `notes.ja.md` is a thread that gets fetched again.
  The lesson is about the ledger, not the threads: **a triage is relative to the milestone that wrote it**, so
  the reject bucket is worth re-reading by keyword whenever the milestone changes.
- **A timer spin is named rather than called uncounted.** `determineBound` needs a counted `dex`/`dey` or the
  `sbc` divide idiom, and of the loops it refuses for having neither, **twenty of twenty-one are the same
  thing**: `lda $0284 / bne` — INTIM, the RIOT's interval timer, polled until it reaches zero. The trip count
  is not a property of any register the analysis tracks; it is whatever the hardware has left to count down,
  and no counter will ever appear however much the analysis is strengthened. The refusal was right and the
  reason was wrong: "needs a counted dex/dey" sends a builder looking for an analysis gap that is not there.
  The detector fires on 12 loops and reaches the reported reason on 5; **no region changes verdict and no
  bound is invented** — 0 keys moved on the corpus.
- The detector asks whether the **last instruction in the body to touch Z** is a load of INTIM, not whether
  the body contains one: the corpus holds `sta $002A / lda $0284 / bne`, where a store leaves the flags alone,
  while a body that loads INTIM and then tests something else is not spinning on the timer at all.
  `litmus_timerwait.asm` carries both controls, and only the canonical `$0284` is recognised — matching the
  6532's mirrors would mean reproducing its address decode, and the cost of matching too narrowly is the
  message a builder already gets, while the cost of matching too widely is a false claim about the ROM.
- **A capture queue for the Stella oracle, so adding a ROM does not have to interrupt the user.** Capturing
  the write-only TIA registers needs Stella's GUI and takes over the screen for ~13 s per ROM. A ROM added
  mid-session can now be listed in `internal/oracle/testdata/stella_tia/CAPTURE_QUEUE` and captured in a
  batch later. **It is not an exemption list and is built so it cannot become one**: every queued line is
  printed on every run, and the test fails once the queue passes six entries — a queue that stops being
  drained gets louder rather than quieter.
- **G1 — the advanced cartridge schemes were never unsupported; they were unverified, and DPC/DPC+ were one
  edit away from being analysed WRONG.** Measured over the 493 cartridge images under the umbrella (478 load:
  335 4K, 61 F6, 29 F8, 16 2K, 12 F4SC, 7 DPC+, 5 3E, 4 F4, 3 E0, 3 F8SC, 1 AR, 1 F6SC, 1 FA) plus five
  authored fixtures: **7 of 8 schemes load, 7 of 8 report a bank count and all 7 are right, 0 of 7 produce a
  cycle bound (all refused), and `cmd/dissect` decoded 3 of 7 correctly.**
- **The trap was the two schemes that look most like Atari's.** DPC and DPC+ clear *every* geometric gate
  `internal/cyclebound` has — banks of exactly 4096 at exactly origin $F000, parseable `BANK0..BANKn`
  hotspots, `GetBank` reporting `IsRAM: false` everywhere — and their bank-switch rule genuinely **is** the
  address-only Atari rule the package models. The only thing declining them was the absence of their ID from
  `verifiedEdgeSemantics`, i.e. one source-reading away from being removed by someone who checked the switch
  and was right about it. They would still have been wrong about the cartridge: $1000-$107F is the
  data-fetcher / RNG / music register file, so a value range folded from the image there bounds a loop on
  data the hardware never holds.
- **The fix asks the engine, not a list.** New `emu.CartridgeWindowNotImage` reports whether the CPU can read
  something other than the image in $1000-$1FFF, and names why, from the engine's own bus interfaces — RAM
  bus / static-data bus / register bus / coprocessor. Measured: 4K, F8 and **E0** answer no on all four (E0 is
  the load-bearing negative — a segmented mapper whose window really is image bytes); F8SC/F6SC/F4SC/FA/3E/3E+
  via the RAM bus; AR via RAM + registers; DPC via static + registers; DPC+ via static + registers +
  coprocessor. `MapsCartridgeRAM` delegates to it, so the refusal now rests on the fundamental objection.
- **New corpus `roms/carts/`** — five fixtures (F6SC 4 banks, F4SC 8, 3E 4×2K, 3E+ 4×1K at four origins,
  DPC 2×4K + 2K graphics), each the smallest image the engine fingerprints as its scheme that still boots and
  runs. **5 of 5 refused by `Prove`, each naming its mapper AND its reason.** They are a separate directory
  from `roms/litmus` on purpose: the Stella TIA oracle covers litmus + techniques and would demand five GUI
  capture sessions (~13 s of the user's screen each) for five ROMs that paint one flat colour. They are still
  in the regression net — `scripts/check_wiring.py` now scans `roms/carts` alongside `roms/litmus` (127 ROMs:
  57 via scenario, 70 via a test or tool, 0 orphaned).
- **`cmd/dissect` was stating a wrong bank count as a fact.** Its banking came from `len(rom)/4096`, which is
  the Atari family's arithmetic and nobody else's: 3E's 4 banks of 2K were printed as "2 banks of 4K"
  (DeathMerchant's 24 as 12), 3E+ as not banked at all, DPC+'s 6 banks as 8, and DPC's 2K of graphics — which
  belongs to no bank — was attributed to a third bank of a two-bank cartridge. The bank number was not just a
  header; it labelled every matched table, so a table at file offset $1800 of a 3E cartridge was reported as
  "ROM bank 1 $F800" when it is bank 3 and a 3E bank maps at $1000-$17FF. Geometry now comes from the mapper,
  and where the layout is not "N banks of 4K at $F000" the tool prints file offsets and says why rather than
  computing a bank number it cannot justify.
- **Every negative control fired.** Removing the static/register/coprocessor arms → DPC reads
  `window-not-image = false` and its `Prove` refusal falls back to the weaker "not among the mappers whose
  bank-switch rule has been checked" wording; making the guard refuse any banked cartridge → four control
  ROMs (F8, F6, F4, banked_game) report "the guard has spread to cartridges the static analysis was
  handling"; reverting the geometry to `len/4096` → `cart_3e` reads `{banks:2 atari4K:true}` against
  `{banks:4 bankSize:2048 atari4K:false}`; hiding one fixture `.bin` → all three suites FAIL with
  "4 of 5 fixtures are assembled" rather than skipping; hiding the tests → `check_wiring.py` reports
  `roms/carts/cart_f4sc.asm` and `cart_f6sc.asm` as orphans (the other three are named in Go doc comments,
  which that checker's substring scan counts as a reference — a pre-existing property of it).
- **Not closed, and stated:** DPC+/CDF/ACE have no in-repo fixture (real ARM Thumb driver code, not four
  fingerprint bytes), so their refusal is exercised only on out-of-repo cartridges. **ELF and bus stuffing
  cannot be tested at all** — bus stuffing is implemented only by the ELF and ACE mappers, no ELF or ACE image
  exists on this machine, and a synthesised ELF header is rejected by the engine
  (`mismatched ELF version 'EV_NONE'`).

### Changed

- **A loop body with an if/else in it is still a counted loop.** `loopShape` walked the body as a straight
  chain and refused the moment it met a branch. Measured across the sixteen-cartridge corpus, of the branches
  that tripped that refusal: **89 are a forward skip whose target is still inside the body**, 29 are an early
  exit, and 1 is an inner loop. Three quarters are a skip — most often `bcc` (64 of 118), the "add, and if it
  did not carry, skip the fixup" idiom. Such a body is a small acyclic graph with two ways through that rejoin
  before the latch, and it has a **longest path**, which is a sound cost for one iteration because whichever
  way the machine goes it cannot spend more. `litmus_branchbody.asm` proves **72 cycles against a machine that
  spends 72**. A chain body has exactly one path, so the walk reproduces the old running sum instruction for
  instruction: over 958 regions, **0 lost, 0 raised, 0 lowered**.
- The body walk is now two passes. Pass 1 collects and validates; pass 2 computes the longest path **in
  address order**, which is a topological order because every surviving edge goes forward. Relaxing distances
  inside pass 1's work queue would not be correct — a site can be dequeued before a longer path into it has
  been found, and its successors would then carry a stale distance.

### Fixed
- **A quarter of the per-bank source lines named a line that assembles nothing, and the count of answers was
  being read as a count of correct answers.** `srcmap.BankMap` took a line number from any DASM listing row
  that PRINTED an address, and DASM prints one on rows that put no byte in the image: a comment, an `=`
  equate, an `ORG`, a bare label, and a macro expansion listed under the macro body's own line numbers
  restarting at 0. Before the first `ORG` the address it prints is offset `$0000` — bank 0's first byte — so
  on `litmus_bank_f4` **`bank 0 $F000` resolved to line 1, the file's opening comment**, and the equates on
  lines 6-8, which start in column 1, were placed as bank 0 LABELS at `$F000` besides. The commit that built
  the map quoted `bank 1 B1Work (banked_game.asm:110)` as its proof; line 110 of that file is the comment
  `; ===== bank 1 =====`. Measured over the 11 bank-switched images the analysis accepts: **256 of 1671**
  resolved (bank,address) line numbers named a line that assembles nothing, and of the pairs the MACHINE
  actually executes over 300 frames, **91 of 878**. After: **0 of 1617** and **0 of 878**, with the executed
  denominator unchanged — 878 of 1004 executed pairs carry a line before and after, so 54 fabrications were
  removed and no real line was lost.
- **The cost was the one this project rates worst: a confidently wrong location sends the author to read a
  line that has nothing to do with the address.** `cyclebound -asm roms/exerciser/exerciser.asm` reported a
  region at `bank 1 ScoreLoop (exerciser.asm:990)`, which is the bare label `ScoreLoop:`; it is now line 992,
  the `sta WSYNC` that actually assembles there. `litmus_bank_f4` bank 1 `$FF03/$FF05/$FF07` now name lines
  70/71/72 = `lda #$B1` / `sta $90` / `inc $91`, spot-checked against the source rather than the listing.
  A row now defines a line number only when it EMITTED BYTES, defines a label when it merely holds a
  POSITION (a label alone on its line has an address and no bytes), and does neither when DASM marked it
  `????`. **Flat ROMs cannot be affected and were measured, not assumed: `ParseBanked` is only built when the
  image has more than one bank, and all 137 flat images produce byte-identical reports.**
- `TestBankedSourceLinesNameALineThatAssemblesThere` grades every resolved (bank,address) against the SOURCE
  FILE — a comment, an equate, a directive or a bare label cannot be the line that assembles somewhere —
  which is a different authority from the listing being parsed, so the test cannot agree with the parser by
  construction. It discovers the bank-switched corpus by walking `roms/**` for a `.bin` over 4K, so a bank
  ROM added later is covered without editing it, and it fails if nothing resolves at all. Negative controls,
  each restored after: reverting `banked.go` fails it with **256 of 1671**; dropping the macro rule alone,
  **14 of 1631**; dropping the equate rule alone, **1 of 1618** (`bank 0 $FE10` → line 814,
  `ZHmove = ZHmoveEnd - 256`); dropping the emitted-bytes rule, `bank 0 $F000` → line 119, `ORG $0000`.
  Dropping the `????` rule does NOT trip the line check — the emitted-bytes rule catches those rows on its
  own — it trips the LABEL assertion instead (`LabelBank(VSYNC) = bank 0`), which is the job that rule still
  does alone. `litmus_superchip` resolves nothing and is recorded by name with the reason rather than
  skipped: it `org`s at `$D000`, so its listing column is not a 0-based offset, and inferring the base from
  the lowest address seen would put every line in the wrong bank on a source that leaves its first bank
  empty. Nothing is lost — the analysis declines F8SC before a map is built for it.
- **The branch wall hid a raft of larger walls, and that is the finding.** Allowing a branch in the body moved
  exactly **one loop** on the corpus. The 89 skips were not miscounted: a body walk stops at its FIRST
  obstacle, so a census of refusal reasons is a census of what is NEAREST, not of what is blocking. With the
  branch wall gone the walks run further and hit what was always behind it. Measured over single-latch loops
  after the change: **105 bodies fully understood** (just 1 of which needed the graph), **53 understood but
  with an unknown trip count**, **41 WSYNC inside loop body**, 13 branch (the early exits and inner loops that
  stay refused), 13 call or jump. `branch inside loop body` fell from 118 first-hits to 13 while `WSYNC inside
  loop body` rose to 41 — the same loops, failing further along. The largest obstacle is no longer a body
  shape at all.
- **This is the third refusal in a row measured to be a name rather than a cause** (`multiple back-edges`
  before it, SD-9's proxy before that), and the pattern is worth more than any of them: **removing the first
  obstacle mostly reveals the second**. Both repairs are correct, both are pinned by fixtures that grade
  against the machine, and both bought approximately nothing — which is only knowable by measuring the corpus
  before and after rather than by counting reasons.
- **A second guard with no negative control, recorded rather than deleted.** Removing the `[header, latch]`
  test on successors leaves both fixture controls refused exactly as before: an early exit is caught because
  the walk follows the escape and a region is bounded by WSYNC strobes, and a backward escape is caught
  because a branch below the header IS a back edge, making the region a two-latch one. Both are accidents of
  other checks. The guard states the premise pass 2 rests on — every site inside [header, latch], which is
  what makes ascending address a topological order — and a walk that wandered below the header would produce
  a wrong answer rather than a refusal.


- **Two loops in a region are not necessarily nested, and the refusal was named after
  the rarest of the three shapes.** `foldLoops` refused any region with more than one back edge as
  "multiple back-edges (nested/complex loops)". Measured across the sixteen-cartridge corpus, of the regions
  carrying exactly two latches: **22 are siblings** whose intervals do not overlap, **9 overlap irreducibly,
  and exactly 1 is nested** — the name describes the rarest one. Nesting is rare for a reason that is obvious
  once stated: a region is one WSYNC-to-WSYNC interval, so a nest would have to fit two levels of iteration
  inside a scanline. Siblings are two plain counted loops one after the other, each a fold the code already
  computes into a map that is keyed by header and holds as many as it is given. `litmus_siblingloops.asm`
  proves **40 cycles against a machine that spends 40**, and both controls stay refused.

### Fixed
- **The diagnosis is worth more than the change: lifting the latch-count limit gained ZERO regions, because
  the graph shape was never the obstacle.** Every multi-latch region still fails, and the census of why says
  `branch inside loop body` by a large margin at every latch count, then `trip count unknown` (14 of the
  two-latch regions), then `WSYNC inside loop body`, then `call or jump inside loop body`. So
  "multiple back-edges" was a refusal **named after a property that is real but not load-bearing**, and the
  49 regions behind it are waiting on a different repair. Corpus effect: 0 gained, **0 lost, 0 lowered,
  0 raised**.
- **Reporting the specific body reason for a multi-latch region cost 6 proven regions** (Barnstorming $F3D4,
  Chopper Command $FA78 and $FAEC, Planet of the Apes $F8B9, Seaquest $F1EC, Stampede $F1A5). It is more
  informative than "multiple back-edges" and it is the wrong answer: `multipleBackEdges` is the ONE refusal
  the DAG walk is allowed to override, matched **by identity**, so a more precise string is a string the
  override does not match. Every multi-latch failure now rounds to it, and a test pins that.
- **`overlaps` is unreachable today, and the negative control measuring that is written down rather than
  taken as permission to delete it.** Disabling the guard entirely leaves the fixture's nested and
  overlapping rows refused exactly as before: if two loops share an instruction then one body holds the
  other's latch, a latch is a branch, and the body walk refuses a branch first. It is kept because the very
  next repair — `branch inside loop body`, the largest refusal left — removes the check that hides it, and
  it then becomes the only thing standing between a nest and a fold that charges an inner loop once for
  iterations the machine runs many times. Pinned directly by a unit test, since no ROM can reach it.

### Added

- **Stella oracle v3 — the write-only TIA registers are compared, not inferred (G4).** RAM (128/128 bytes) and
  pixels (100%) already agreed with Stella; COLUPF / NUSIZ / CTRLPF / REFP / HMxx did not, and **pixels cannot
  settle them** — an object whose graphics byte is `$00` renders identically whatever its NUSIZ says.
  `scripts/stella_oracle.sh <rom> <frames> tia` captures Stella's `tia` debugger output, `cmd/stellacheck
  -session` grades it offline, and `internal/oracle` re-grades every capture on every `go test`. **147 captures
  (all 114 `roms/litmus` + all 31 `roms/techniques` + 2 probes) x 37 registers = 5,439 readings, 19
  disagreements, 0 divergences**, with all 37 registers taking more than one value across the corpus.
- **Each disagreement is classified from measurement, never from assertion.** 7 are sub-frame phase — our side
  holds Stella's exact value at some scanline of the next frame (`litmus_hmxx_freeze` sets `HMP0=$80` right
  after VSYNC and `HMCLR`s it later in the same frame, so the two frame boundaries fall either side of one
  store). 10 are undefined at power-on: `litmus_cycles` and `uninit_trap` contain no `HMxx` or `HMCLR` write at
  all and read Gopher2600's power-on nibble 8 against Stella's 0, where a real TIA leaves the register
  undefined and **neither is right**. 2 are power-on RAM, and there **Stella is not reproducible**: two
  consecutive captures of `uninit_trap` at the same frame gave COLUBK `$fc` and then `$02`.
- **Stella's `tia` text is decoded by probe ROMs written for the purpose, not by reading Gopher2600.** Taking
  the field conventions off our own emulator would have made the comparison circular, so
  `internal/oracle/testdata/tiaprobe.asm` fixes from ROM source that `HM=$7` is the raw nibble, `size=#N` the
  raw two-bit field, `GR=` the **new** VDEL copy, and `PF0` pre-shifted.
- **RESMP1 is a Stella 7.0 defect, so it is named rather than compared.** `tiaprobe.asm` writes RESMP0=`$02` /
  RESMP1=`$00` and Stella prints the reset flag SET on both missile lines; the mirrored `tiaprobe2.asm` writes
  the opposite and Stella prints it CLEAR on both. Stella's M1 flag equals RESMP0 in both cases and RESMP1 in
  neither. `TestStella70MisreportsRESMP1` locks this, so a fixed Stella turns the test red and the register
  comes back into the comparison.
- **The channels that would have made this headless were ruled out by measurement, not assumption**
  (Stella 7.0): `dump 00 3f 1` returns the TIA *read* ports mirrored every `$10`; `saveState` from autoexec
  writes no file anywhere; the debugger expression language exposes no TIA-register accessor; and
  `tia` + `saveSes` inside `autoexec.script` yields a **0-byte file**, because `Debugger::exec()` keeps only
  the `Executed N commands` summary and discards each command's output.
- **The GUI dependency is loud rather than silent.** A corpus ROM with no capture fails the test by name, and
  it fired for real when `litmus_bccdiv.bin` was added mid-run. Adding a litmus or technique ROM now requires
  `scripts/stella_oracle.sh <rom> 5 tia`.

### Fixed
- **CLAUDE.md named a Stella flag that does not exist.** The settled-architecture line listed `-dbg.script` as
  how debugger commands reach Stella. `Stella -help` (7.0) has no such option and the real mechanism has always
  been `~/Library/Application Support/Stella/autoexec.script`. A wrong name in the one document that is loaded
  every session is worse than no name.

### Fixed
- **BCC counts UP, so the divide bound used the wrong variable — and this closes the nine unsound bounds
  the `determineBound` audit found.** `sbc #N / bcs` loops while A >= N and A falls, so a larger entry value
  means more iterations and `amax` bounds it. `sbc #N / bcc` loops while A < N and the subtraction **wraps**,
  so A rises by (255−N) until it reaches N — a larger entry value means FEWER iterations and `amax` bounds
  nothing at all. One formula was applied to both; it agrees only while N is small. Measured on the new
  `litmus_bccdiv.asm` at N=200: **proven 16, machine 31 — 1.9x under**, with `certified: true`. The BCC
  bound is `ceil(N/(255−N)) + 2`, and N=255 is refused rather than bounded, since 255−255 = 0 leaves A where
  it was and the loop never ends. Corpus effect over 155 images: 2 bounds raised (both the fixture's), 0
  lost, 0 lowered — all 18 divide folds in the corpus are BCS, which is the only reason none was wrong.
  **Two things generalise from the seven repairs**: four of them *deleted a divergence* rather than adding a
  rule (`transfer` vs `absSuccessors` twice, the fall-through filter vs `successors`, and one formula split
  in two where the machine always had two behaviours); and every census that cleared a defect was accurate
  about what it counted and wrong about the exposure.

### Added
- **`visual_ceiling` / `cmd/ceiling` / `internal/ceiling` — a denominator for a picture.** `vismatch`
  compares a build against another ROM, so a wrong picture could not be separated into "the kernel is
  wrong" and "the hardware cannot do this". The ceiling supplies the missing half: the best any 2600 kernel
  could reach for a target frame under a **stated constraint set**. A ceiling is a property of *(image,
  constraint set)*, never of an image, so the output is a **ladder** and the **deltas are the
  deliverable** — C1 playfield-only / C2 + one 8-clock object / C3 no column grid. Measured on five
  commercial frames the grid costs 7.09 rmse on Barnstorming and 8.58 on Vanguard against 3.13 on Chopper
  Command; one sprite is worth 8.88 on Pressure Cooker. C1/C3 are exhaustive over all 8256 colour-pair
  cases per line (true optima, not heuristics that could understate the machine), C2 exact by
  branch-and-bound, ~20 ms a frame. **The palette is derived from the renderer, never transcribed** —
  `PaletteFor` calls the same `GetColor` that paints each pixel, and a test proves that table equals what
  `litmus_palette.bin` actually draws on all 128 entries. That was the trap: the prototype read 9.95 on a
  frame achievable by construction because it used Stella's palette on Gopher2600 frames. Self-test: 5
  in-tree playfield-only ROMs score C1 **exactly 0**; both directions checked (sprite frames 23.06–40.92);
  planted wrong palettes break it on 5 of 5. **Limitation stated rather than implied: no rung emits a
  cartridge**, so none is validated against the 76-cycle budget — C1 rests on one prototype demonstration
  (66 cycles certified, 0/29440 pixels differing), C2 has none, C3 is unreachable by design.
  `docs/visual-ceiling.md`.

### Fixed
- **SD-9's address proxy was still live on the divide path, and nine real folds were resting on it.** The
  BCS/BCC path found A's entry bound with textual fall-through plus address order — the heuristic SD-9
  deleted from the dex/dey path — with an `lda #imm` guess behind it. Measured on the new
  `litmus_divpred.asm`, all three with `certified: true`: a predecessor arriving by `jmp` was invisible
  (**27 vs 87**), the proxy answered when nothing was adjacent (**28 vs 87**), and a `jmp` merely sitting
  before the header was read as a predecessor (**29 vs 89**). **The proxy's "0 uses" counter was a fact
  about eight hand-listed ROMs**: across the corpus nine folds were bounded by it, and it reads `lda #80`
  while ignoring the `adc #XCAL` two instructions later — 7 iterations where the sound bound is 19. They
  sat above the machine by luck.
  Removing it exposed the precision gap that had made it necessary: **`adcRange` returned Top on wrap**,
  and `XCAL = -5` assembles to `$FB`, so `lda #80 / clc / adc #XCAL` computes 331 and gave up. A wrapped
  sum is still a byte, so `[0,255]` is true and *useful* where Top is true and useless. Over 155 images:
  **0 bounds lost, 0 lowered, 12 raised** — the nine go from 53-63 to 118, from resting on an ignored
  instruction to proven. Gate green on 1243 regions across 158 ROMs. The scan is now the dex/dey path's:
  ask `absSuccessors` which edges reach the header and read A from the edge's state — deleting a
  divergence rather than adding a rule, for the third time in this function.

- **A `jsr` inside a folded loop body was costed at six cycles, dropping the callee once per iteration —
  and the tree contains a live instance.** `IsBranch()` is `Relative && Flow`, so a JSR (Absolute,
  Subroutine) and a JMP (Absolute, Flow) both sailed through the body walk. Measured on the new
  `litmus_callinloop.asm` with a twelve-`nop` callee: **proven 48, machine 168 across 3 scanlines — 3.5x
  under**, with `certified: true`. **The worse case is not the arithmetic**: a callee containing
  `sta WSYNC` makes the walk step over a REGION BOUNDARY, so the machine's interval ends at that strobe and
  the proof's does not. `roms/techniques/shared_setxpos.asm` $F054 is that shape — `jsr SetXPos` into a
  routine whose second instruction is `sta WSYNC` — and read **proven 98 against a machine 36**. Neither
  number was wrong; they were about different spans of time, and the 62-cycle "slack" was never slack.
  That is a **third** way for a bound to be wrong alongside too-low and too-high — *about the wrong
  interval* — and `observed <= proven` cannot detect it, because both readings pass while measuring
  different things. Corpus effect over 155 images: 2 folds lost (the fixture's own, and $F054 which was
  already over budget), zero bounds lowered, no certification lost.

- **A loop entered PAST its header carried a counter value nobody had scanned.** `determineBound` maximises
  the entry value over the predecessors of the header, which is the right set only if every execution
  reaching the back edge passed through the header — and nothing anywhere stated that premise. Measured on
  the new `litmus_midentry.asm`, where a `jmp` lands one instruction past the header with X=$50 while the
  header's only scanned predecessor loads X=2: **proven 40 cycles against a machine that spends 733 across
  10 scanlines. 18.3x under**, with `certified: true`. The guard collects the body's sites during the walk
  that was already happening and refuses an edge from outside into any of them **other than the header** —
  the exclusion is the whole subtlety, since several predecessors of the header are sound (the scan sees
  them all and takes the maximum). A guard keyed on "more than one predecessor" would pass the danger case
  and refuse a common shape; the fixture's control row proves it, and with the header included in the check
  both controls fail. **Precision cost over 155 images: zero** — the only fold lost is the fixture's own.

- **A loop counter's entry value now comes from the EDGE into the header, not from the instruction's own
  effect — and the two were computed by different functions in the same package.** `determineBound` used
  `State.transfer`, which models what an instruction does to the machine; for a JSR that is only the push,
  leaving X and Y at their **pre-call** values. `absSuccessors`, which defines what flows along each edge,
  correctly resets a JSR's return point to Top because the callee is not modelled. Measured on the new
  `litmus_jsrentry.asm` — `ldx #$02 / jsr SetBig` where the callee does `ldx #$50` — the scan saw X=2 and
  answered **36 cycles against a machine that spent 738 across 10 scanlines. 20.5x under**, with
  `certified: true`. The repair reads the edge state, which **deletes the divergence instead of adding a
  rule**: a JSR predecessor now yields `X.Top` and the existing unknown-entry refusal fires. **Precision
  cost over 155 images: zero** — the only folds lost are the fixture's own, since no corpus ROM has a call
  between a counter's load and its loop. The fixture keeps a control whose callee provably does not touch
  X as an *asserted refusal*, so the missing callee summary is a measured gap rather than an unexamined
  side effect; the test names it as the row that should become bounded if one is ever added.

- **SD-11 closed: a `dex; bne` counter that can enter at ZERO wraps for 256 iterations, and the proof was
  answering with the range's upper bound.** The trip count is `v` for v > 0 and **256** for v = 0, so it is
  not monotone and `Hi` is not the maximum when zero is reachable. Measured on the new
  `litmus_bnezero.asm`, whose join gives the header X in `[0,5]` and whose machine takes the zero arm:
  **proven 60, machine 2319 across 31 scanlines — 38.7x under**, with `certified: true` and
  `roll_free: true`. The repair returns 256 rather than refusing, so the region stays bounded and the
  author gets a number instead of silence; verified against the machine at **2319 == 2319**.
  **Why it was left alone last time is the interesting part.** When the `bpl` sibling was fixed this hazard
  was censused over the five commercial cartridges the gate then graded — 3 instances, none violating — and
  filed rather than fixed, on the stated principle that a change without a witness is how the previous bug
  got in. That reasoning was right and the corpus was wrong: re-censused after the gate stopped grading a
  hand-picked five, it is **14 folds across three shipped cartridges** (Seaquest x3, Bermuda Triangle x6,
  Vanguard x5, all `[0,15]`). Only the denominator changed. Corpus effect over **155 images**: 15 bounds
  raised, **0 lowered, 0 lost**, and all 14 pre-existing ones were already over budget, so no certification
  was lost. Controls: a joined range of `[3,5]` must stay exact (the fix must not fire on any join) and a
  plain `ldx #5` countdown must stay exact (it must not fire on any BNE) — without the latter, a blanket
  repair reports 2315 for a loop the machine finishes in 56.

- **`determineBound` audited on purpose rather than by accident: 9 unsound bounds measured, the largest
  fixed, and the gate was grading a third of the cartridges on disk.** Two of this package's three known
  unsound bounds were in this one function and both were found while investigating something else, so it
  was audited deliberately — 20 premises enumerated, 11 fail unsoundly, 9 probed with a real cartridge.
  Every probe ran (`Count = 12`, none a refusal on dead code) and every one reported `certified: true`:
  counter written in the body **22 vs 2290 (104x)**, BNE range including zero 67 vs 2326, `transfer(JSR)`
  36 vs 738, mid-body entry 40 vs 733, a call in the body 48 vs 168, and four more.
  **Fixed: the 104x.** SD-13 guarded the window *after* the decrement with `preservesZN`; the window
  *before* it was open, and a write there changes the COUNT rather than which flags are read. `writesX` /
  `writesY` now require the counter's register to be written by exactly one instruction, the decrement.
  The fixture caught a bug in the repair itself — keying on "any index register" refuses every loop that
  walks two pointers, and `OtherCtl` (`iny` inside a `dex` loop) failed at once. Corpus effect over **155
  images**: 4 folds lost, **all four already over budget**, so no certification was lost; zero bounds
  lowered.
  **And the gate's corpus was a hand-written list of 5 while 15 images sat in the same directories.** It is
  discovered by glob now — the same repair the scenario runner needed when 38 of 95 scenario files turned
  out to be run by nothing. **5 cartridges / 66 pairs / 1022 regions → 16 / 234 / 1190 across 152 ROMs**,
  still green. Extending it is what turned the counter-write hazard from "zero corpus instances" into a
  real one (`Pressure Cooker $D801`) and tripled the SD-11 count.

- **A loop's latch must read the counter's own flags — it was never checked, and the prover certified a
  region the machine takes 201x longer to run.** `determineBound` derives a trip count from "the counter
  decrements to zero and the branch exits there", which is reasoning about the DECREMENT's Z/N; anything
  writing those flags in between substitutes its own condition. Measured on the new
  `roms/litmus/litmus_latchflags.asm`, whose DangerRow is `ldx #1 / ... / dex / cpx #$02 / bne`: **proven
  19 cycles, machine 3829 across 51 scanlines**, reported with `certified: true` and `roll_free: true`.
  After the decrement X is 0, the compare against 2 clears Z, and X wraps through `$FF` for 255 iterations.
  Two controls in the same ROM isolate the cause and rule out the cheap repair — SafeRow (`nop` instead of
  the `cpx`) and StoreRow (`stx`, which writes memory not flags) both stay **exact at 21 and 47**, and
  demanding the decrement be ADJACENT to the latch would break both. **Why 140 images hid it:** reverse the
  inequality and the bug is an over-approximation, sound by luck — `exerciser.asm $F0C9` is that shape.
  Census: 757 dex/dey folds, **720 adjacent**, and of the 37 with a gap the instructions are `cpx` 19,
  `inx` 7, `adc` 5, `jsr` 5 — all flag-writing, no store anywhere. The fix is a whitelist rather than a
  blacklist because the engine's instruction table records memory effects, not flags, and the safe default
  with no table is to refuse. Corpus effect: **one region changes, the unsound one**; zero bounds lost,
  zero lowered, gate green on 1022 regions across 141 ROMs.

### Added
- **`get_screen_annotated` takes `raw=true` and returns the bare frame — 160 x visible-height, one pixel
  per TIA pixel, no grid, labels, markers or upscale.** The annotated image serves one direction of the
  pixel-art loop: the user points at a coordinate and Claude turns it into registers. The other direction
  runs the opposite way — the user opens the frame in Photoshop and paints dots, and Claude samples the
  file back into `.byte` rows — and that needs the file's pixel grid to BE the machine's. There was no way
  to get one; every screenshot carried annotations, which in that direction are not decoration but foreign
  pixels inside the artwork. Scale is deliberately ignored in this mode rather than applied, because an
  upscaled "raw" image is a file that lies about its own units. It writes to a separate `*_raw.png` so it
  cannot overwrite the annotated file the user keeps open in a reloading previewer. The height is whatever
  the frame actually rendered, read rather than assumed at 192 — measured on the sunset kernel it is 214.

### Fixed
- **The annotated screenshot no longer draws markers for objects that are not on screen.** `Markers` read
  the TIA's position registers and returned all five movable objects unconditionally — but a TIA object is
  a counter, it always HAS a position, and that says nothing about whether it painted anything. On a
  playfield-only kernel the result was **five labelled vertical lines for five objects that drew zero
  pixels**, over an image CLAUDE.md calls the primary channel the user reads a picture through. That makes
  a phantom marker a false statement about the ROM rather than a cosmetic blemish. New `emu.DrawnObjects`
  answers from the frame's own per-pixel attribution buffer — the one `decompose_row` reads — so the
  question "did P0 appear" is settled by looking for P0 in the picture instead of reasoning about GRP0,
  NUSIZ, VDEL and the priority rules and hoping the reasoning matches the hardware. The JSON still lists
  every object, now with `drawn`, because a position is real and sometimes wanted; only the image drops
  them. Verified end to end over MCP: the sunset kernel returns five objects all `drawn:false` and an image
  with no markers, `litmus_pos` returns `P0 drawn:true` and keeps its line. Both directions are tested,
  since a function that returned false for everything would satisfy the playfield case alone and silently
  erase every real sprite. **One test expectation was written wrong and the measurement caught it**: the
  `objsizes` litmus was assumed from its name to cover players, and it does not — it sweeps missile and
  ball widths only. The two independent readings of the buffer agreed with each other and disagreed with
  the author.

### Added
- **`prove_line_budget`, `defuse` and `beam_intervals` now say which build answered — and shout when the
  source has moved since.** A static analysis is a claim about source; the server answering is whatever
  binary the session connected to, and editing the analyser does not change it. Measured 2026-08-01, twice
  in one session and both times on a fix that was already correct: `prove_line_budget` returned worst **74**
  for a kernel the current source proves at **66**, and reported the DAG-first witness ROM as refused when
  the current source bounded it at **26**. Both read as "the change did not work"; the honest response to
  each would have been to revert a correct change. Go already embeds the answer with no build flags — the
  running binary carried `vcs.revision=bb3b0f8` while the tree sat at `30b492d`, four commits later, and
  nothing read it. Stamping alone would not have sufficed, since a stamp only helps a reader who thinks to
  compare, so the server reads HEAD itself and puts a full sentence in the result. It stays SILENT when a
  guess would be wrong (no build revision, unreadable repository), because a false STALE trains a reader to
  ignore the real one; a build from an uncommitted tree gets its own milder note. Note that `version.Harness`
  would NOT have caught this: it read `2.0.0` on both binaries, because the source moved and the release
  number did not.

- **A page-aligned table cannot be crossed, and the proof now says so — measured on a kernel the corpus
  did not contain.** `pagePenalty` reached its conservative `+1` whenever the index range was unprovable,
  which is exactly when a kernel aligns its tables in the first place. The rule that settles it needs no
  index analysis: a 6502 index register holds 0..255, so `$NN00 + idx` is at most `$NNFF` and never leaves
  the base's page. **Across the 135 ROMs in `roms/` this case fires ZERO times**, and that is a fact about
  the corpus rather than the case — 24 of the 31 technique kernels draw no playfield, so not one of them
  is a table-driven picture kernel, and a picture kernel is what aligns tables. The first one written
  produced **8 wasted charges on its first run**: proven worst 74 against a machine that takes 66, two
  cycles of headroom reported where there were ten. Witness `litmus_pagealign.asm`, whose aligned region
  now proves 34 against a measured 34 — equality, because a bound that is merely safe sends an author
  trimming work that was never over budget. Negative controls both ways: removing the shortcut puts the
  aligned region back at 38 vs 34, and widening it to ignore the base's low byte makes the same fixture
  report 44 for an interval the machine takes 48 — caught by the corpus gate, along with
  `litmus_pagecross`. **The fixture's first draft proved nothing and its own premise check said so**: the
  index was written as a constant, and the abstract interpreter tracks RAM well enough to pin it, so all
  four reads took the already-free path. It reads the joystick port now.

- **The prover stops calling a DAG a loop — and the first attempt at it published a bound BELOW the
  machine, which the corpus gate caught.** `foldLoops` decided what a back edge was by ADDRESS ORDER: a
  branch counted as one when its target sat at a lower address and was not a WSYNC sink. That is not
  reachability, and a region whose graph is acyclic was refused as *"multiple back-edges"* for having no
  edges back. Four regions across the corpus are exactly that, all commercial: Seaquest `$F1EC` (proven
  59, machine 53), Chopper Command `$FA78` (74, 72) and `$FAEC` (103, 97 over two scanlines),
  Barnstorming `$F3D4` (95).
  **The order is the whole finding.** Running the longest-path walk FIRST and accepting whenever it met
  no cycle bounded 49 regions — and 45 of them were bypassing legitimate refusals, because `foldLoops`
  refuses for eight reasons and only one describes the graph. The other seven describe the loop BODY,
  and a loop whose body holds a WSYNC is **invisible to the walk**: the WSYNC is a sink, so the walk
  stops there and never traverses the edge back, leaving a subgraph that looks perfectly acyclic.
  VideoOlympics `$F5CA` refuses for *"WSYNC inside loop body"*; the walk answered **148 cycles for an
  interval the machine takes 163**, and `$F61F` did the same at 155. Folding first and overriding only
  the back-edge refusal leaves exactly the 4 intended regions, 0 bounds lowered, 0 lost, and no region
  reclassified. Witness: `litmus_dag_region.asm`, since all four real specimens are cartridges this repo
  does not contain. Negative controls both ways — removing the override fails the witness on its own
  premise check (0 multi-latch regions reached), widening it past the back-edge case fails the invariant
  test (not one body-shape refusal survives in 31 ROMs).
  **The gate caught this because it had been extended hours earlier.** VideoOlympics was only graded at
  all because the commercial images were added to `TestProvenWorstIsNeverExceededOnCorpus` the same day;
  before that the corpus was the corpus we happened to write, and a well-argued soundness claim would
  have shipped an unsound bound.


### Fixed
- **The cycle-budget prover under-approximated every `dex`/`dey` countdown latched by `BPL` by one whole
  iteration — proven 66 where the machine takes 75.** `determineBound` accepted `BNE` and `BPL` as the latch
  of a decrement countdown and returned the same trip count for both. They do not end on the same iteration:
  `dex; bne L` from X=6 leaves when the decrement produces **zero** (6 iterations), while `dex; bpl L` leaves
  only when it produces a **negative** value, so it runs the body once more with X=0 and exits on X=$FF
  (**7 iterations**). The bound was short by one body plus one taken branch, every time. Found on the real
  **Seaquest** cartridge, region **$F1FC** — `sta HMOVE / sta COLUBK / lda #$FF / ldx #$06 / L: sta $B0,x /
  dex / bpl L` between two WSYNCs: **proven 66, machine 75, slack −9**, carried out on `Bounded=true`. A
  proven worst case the hardware **exceeds** is the one answer this package must never give; an author
  reading it would have believed a 75-cycle line had 10 cycles spare. Fix: the trip count of the `bpl` form
  is `best+1`. It is **sound and exact**, not a cushion — `loopCost` is monotone in the iteration count, and
  for the bpl exit condition the count is `v+1` for entry values `v <= 128` and 1 for `v >= 129`, so `best+1`
  bounds it everywhere and equals it for the loops an author actually writes.
- **Why the standing corpus gate never saw it, measured rather than guessed.**
  `TestProvenWorstIsNeverExceededOnCorpus` globbed only `roms/{techniques,litmus,exerciser}` — the corpus we
  happened to write. Censused over the 140 images analysed: **7 `bpl` folds, only 4 of them in our own
  kernels, and not one of the 4 could expose it.** `rts_dispatch`'s region $F036 and `zone_multiplex`'s
  $F033 produce **no `ProfileLineWorst` row at all**, so the gate compared nothing there; `shared_setxpos`'s
  $F054 was proven 83 against a measured 36 — **47 cycles of slack** to absorb the 15 cycles of error. Slack
  hides an under-approximation exactly as well as it hides nothing. The gate now also grades **five
  commercial cartridges** (VideoOlympics, Adventure, Seaquest, Chopper Command, Empire Strikes Back), which
  live in the umbrella `reference/` tree outside this repo: **absent → skipped with the reason logged;
  present → 5/5 graded, 63 region↔row pairs**, and *anything in between fails*, because a ROM that is
  present, loads, and then quietly compares nothing is a skip wearing a pass. Commercial images are profiled
  over 30 frames, not 6 — measured, Chopper Command yields **0** rows at 6 and **18** at 30. Nothing is read
  from these cartridges but their own bytes.
- **Witness: `roms/litmus/litmus_bpl_trip.asm`**, two single-scanline regions holding nothing but the
  countdown — `ldx #6 / sta $B0,x / dex / bpl` and `ldy #6 / sty COLUBK / dey / bpl`, 262 lines. The test
  asserts **equality**, proven == machine == **75** and **68** over 950 and 960 intervals, because tightness
  is the point: a merely-safe bound would send an author trimming a line that was never over budget. It
  checks its own premise first, via a measurement counter incremented at the corrected line, so it cannot
  pass over a region that never reached the fold. **Negative control:** reverting the `+1` fails it naming
  the **9**-cycle (dex) and **8**-cycle (dey) gaps, and with the bug in place the extended gate goes red on
  Seaquest `$F1FC` as well. **Second control:** re-proving the whole corpus before and after moved **6 of
  1226 regions**, all upward, all containing a `bpl` fold — Seaquest $F12C 102→107, $F1FC 66→75, $F419
  105→110; `rts_dispatch` $F036 55→69, `shared_setxpos` $F054 83→98, `zone_multiplex` $F033 181→214. Nothing
  else moved. The `dey`/`BPL` sibling is the same code path and is covered by the same fix and its own
  fixture region; `BMI` as a latch is *refused* rather than bounded (`determineBound` accepts only
  BNE/BPL/BCS/BCC), so it was already safe.
- **Two of five subjects of `TestDecodeReachesCodeInCommercialROMs` had been silently skipping.** Their paths
  used two levels of `..` where the umbrella tree is three up, so `VideoOlympics` and `Stampede` resolved to
  a `harness/reference/` that does not exist and took `t.Skipf` on every run while the test stayed green —
  the exact failure that file's own doc comment warns about ("an analysis that finds no instructions does not
  look wrong, it looks like a clean ROM"). Fixed; both now decode (**644** and **858** instructions), and
  Stampede's label is corrected from 4K to its measured **2048 bytes**.

## [2.0.0] - 2026-07-31

**MAJOR, because two MCP tools changed what a caller gets back.** `CLAUDE.md` says this project follows
SemVer and that MAJOR means 互換破壊, so the number is produced by the rule, not chosen: `beamtrace` changed
the SHAPE of its result and `breakif` changed the MEANING of its stop condition, and a caller written against
`1.117.0` is wrong about both. Neither is a mistake being undone — each is a capability that could not be
reached through the old contract (four of five traced frames were unreturnable; the whole visible region was
unstoppable). Everything else in this release is additive or a fix; `### Breaking` below is the complete list
of incompatibilities, and both entries are restated with their measurements under `### Changed`.

### Breaking
- **`beamtrace` returns `frames[]`; there is no top-level `rows`.** The result was a single `{frame, rows}`
  and is now `{frames: [{frame, rows}, ...]}`, one entry per traced frame. **A caller must change `rows` to
  `frames[0].rows` and `frame` to `frames[0].frame`** — reading `rows` at the top level now finds nothing
  there. Why it had to change: the tool traced `frames` frames and advanced the machine past all of them,
  then returned the earliest one alone (measured over the wire: `frames=4` starting at frame 5 left the
  machine at frame 9 and handed back frame 5), so the other frames were unreachable by any route. Pass
  `scanline` to keep the larger payload narrow.
- **`breakif` halts at or past the requested beam position, not on an exact match.** It now stops at the
  first instruction boundary **at or past** `(until_scanline, until_clock)`, so a caller that relied on
  "stops only on the exact value" now stops **earlier** — measured, asking for clock 80 halts at 82 — and
  must read the returned `coords` instead of assuming them. Why it had to change: the machine is only
  observed at instruction boundaries, the CPU advances 3 colour clocks per cycle so only one phase in three
  is ever observable, and a WSYNC kernel narrows it much further — measured on `motion_xclamp`, a visible
  scanline is observed at **7 clocks, every one of them inside HBLANK**, so the visible region 0..159 could
  not be stopped inside at all. An exact-match request for a position in the picture ran to `max_frames` and
  returned `halted=false` with no error, which is indistinguishable from "not yet". Second incompatibility in
  the same tool: an out-of-range clock is now an **error**. The old tag advertised "0-227" while the
  coordinate system is HBLANK −68..−1 / visible 0..159, so 68 of the advertised values did not exist; a
  caller passing one of them used to get a silent never-halt and now gets a rejection.

### Added
- **`LoadROM` no longer dies on a truncated image — and 2 of the 542 `.bin` files on this machine
  killed it.** Found by sweeping every image under the umbrella through the loader: a 12-byte
  `Combat.bin` and a 5-byte `skeleton_test.bin`, both partial downloads in a mined reference archive,
  **panicked** instead of failing to load. The fault is upstream — `hasSuperchip`
  (Gopher2600 `fingerprint.go`) compares `d[:0x80]` against `d[0x80:]` on every 4K window with no
  length check — but the consequence is ours: `load_rom` and `assemble_and_load` take a path from the
  MCP caller and `cmd/fieldtest -inbox` walks a directory the **user** drops files into, so this ended
  the server rather than one call. A truncated download is the most likely malformed input there is,
  and it was the one input that could not be reported. Two layers, verified independently by disabling
  each: a length precheck that can say what is wrong ("this file is 12 bytes"), and a `recover` backstop
  for any other fault the fingerprinter can take on bytes it did not anticipate. **The first version of
  the test proved nothing** — it wrote zero-filled files of the same sizes, and with the guard removed
  those load fine instead of crashing; the fixtures now carry the observed bytes, which do panic.
- **The edge-semantics whitelist is now re-read from the engine's source by a test, not by a human once.**
  `verifiedEdgeSemantics` claims seven mappers select the bank from the address alone, each entry citing
  the file and method where that was read — and nothing checked that the file exists, that the method
  exists, or that it still reads that way. Gopher2600 is a `replace` dependency that gets updated. The
  new gate parses the cited method out of the cited file with `go/ast` and requires that a mapper claimed
  as address-only never reads its data-bus parameter; the mirror assertion requires that the mappers
  recorded as data-driven **do** read it, so the check cannot pass by matching nothing (2 witnesses:
  WF8, FA). AST rather than grep, because CBS quotes the patent's "data line D0" in a comment six times.
  Negative controls: whitelisting FA fails it, and so does a citation to a method that does not exist.
  What it proves is one failure mode — data-bus selection, the WF8 trap — not all of them.

### Changed
- **Four more mappers moved from "unchecked" to "checked, and here is how they differ"**
  (`knownDifferentEdgeSemantics`), each read out of the engine's source: **FA** gates the entire switch
  on data-bus bit 0 (the CBS patent's requirement), so `lda $1FF9` switches or does nothing depending on
  what is on the bus; **FA2** guards `$0FFB` with `len(banks) > 6`, so the published `$1FFB:BANK6` edge
  does not exist on a 6-bank image, and spends `$0FF4` on NVRAM file I/O; **E0** does not switch a bank
  at all but assigns one of three 1K **segments**, leaving the fourth quarter — the one holding the
  hotspots and the vectors — permanently fixed; **E7** spends four hotspots on RAM rather than banks and
  reduces the result by `bank %= NumBanks()`, so its address-to-bank map depends on the image size.
  The refusal was already correct for all four; now it says why. **Only E0's message can actually print**,
  and that was measured rather than assumed: `bankedUnits` refuses a RAM-mapping cartridge before it
  consults this table, and FA/FA2/E7 all carry cartridge RAM by construction, so those three are
  permanently shadowed by the coarser refusal — the same shape as the `foldLoops` finding. E0 has
  `IsRAM: false` on every segment and does reach it, witnessed on three real cartridges (Montezuma's
  Revenge Trainer, Super Cobra, Swtagrc). The shadowed entries still earn their place: they exist to stop
  a future reader from pattern-matching published hotspots onto the Atari rule and promoting a mapper into
  `verifiedEdgeSemantics`, where being wrong invents edges the machine never takes.
- **Measured, and it is the number nobody had: `Prove` gives an ANSWER to 0 of the 33 exotic-mapper images
  on this machine.** Every one is refused — DPC+ (7), F4SC (10), 3E (5), F8SC (3), E0 (3), F6SC/FA/AR (1
  each) — so the analysis has no silent-wrongness path on cartridges outside the F8/F6/F4 families, which
  had never been checked end to end. G1 in the audit is the work of *adding* support; this is the prior
  question of whether its absence is honest, and it is.

- **`cb_pushdisplay` / `cb_pushsafe` — the twin fixtures that witness `pushMissesDisplay`'s
  "SP can reach the display" branch**, which had run 0 times across 129 ROMs. A `PHA` writes to `$0100|SP`
  and page 1 mirrors the console's addresses, so a program that points SP at the bottom of the stack turns
  a push into a VSYNC/VBLANK write — the Stack Trick, and the entire reason the prover tracks SP here
  instead of calling every push display-touching. Nothing exercised the YES side: the one ROM that reached
  the predicate took the "proved to miss" path, and `litmus_stack_trick` — written for this very hazard —
  never reached it at all. The danger twin pushes with **SP = 1** (`$0101` = VBLANK) inside an overscan
  region that would otherwise be classified BLANK and skipped; the safe twin changes **one immediate**
  (`$01FF`, stack RAM). Measured: `visible` vs `blank`, both 16 cy, blank-region count differing by exactly
  one. The branch was confirmed directly (`reaches-display sp=1 ma=$01`), not inferred. Negative controls:
  every-push-safe and every-push-dangerous each fail the test. A fixture defect was caught before shipping —
  the first version's region ran past `jmp Main` into the next frame's `sta VSYNC`, so both twins came back
  visible and it proved nothing.
- **`cb_deadpred` / `cb_deadpred_live` — the twin fixtures that finally witness `determineBound`'s
  "a predecessor we know nothing about" refusal**, a guard that had run 0 times across 123 ROMs and had
  never been shown unreachable either. Two fixtures failed first, and both are recorded: code hopped over
  by a `jmp` is never decoded, so it never becomes a predecessor at all (measured — the scan listed 9
  candidates and the dead address was not among them), and dead code placed in the region above the header
  is not seen because the scan is per-region. What reaches it is the **not-taken edge of a statically known
  branch**: decoded, and then given no abstract state at all, because `absSuccessors` emits only edges
  whose refined state is still valid. Measured over 129 ROMs, the neighbouring `!st.valid` condition fires
  **zero** times anywhere — pruned nodes never acquire a state to be invalid — and the missing-entry
  condition cannot be relaxed, because a fixpoint that hits its iteration cap also leaves nodes with no
  entry. The twins
  differ by that one edge, so the refusal is attributable — the dead one leaves its visible region
  unbounded, the twin bounds all 7 regions with a dearer worst (33 vs 17 cy). Negative control: removing
  the guard makes the dead ROM come back fully bounded.
- **`spritey` and `read_motion` now report `stillness`** — how far the object travelled on each axis over
  the window they measured, and whether any RAM byte changed — so a measurement carries the evidence for
  whether it measured anything. Travel of 0 is flagged as a **CONSTANT**, the shape behind both of this
  week's bad measurements. Multi-frame only: `frames=1` has no window, and the tool will not advance the
  machine to manufacture one. **The note deliberately draws no conclusion about the program, because the
  first version did and was wrong** — it classified "no travel and no RAM change" as *STUCK: the program
  is not running*, and measured before shipping, `litmus_pos` and `smoke` run a full kernel, draw their
  sprite every frame and never write RAM after init, so a live ROM got a confident diagnosis of dead.
  Whether a program REACTS cannot be established without injecting an input, which is why that question
  stays with `set_input`. Three tests; negative controls: restoring the STUCK verdict, and reporting one
  axis's travel into the other, each fail.
- **`motion_xclamp` litmus + the witness for `spritey`'s multi-frame mode.** The mode returns a per-frame
  sample carrying BOTH X and Y and always has, but nothing ever checked that the X in those samples MOVES:
  a build reporting a constant X, or reporting one axis into the other, passed every test in this repo.
  The new ROM is the horizontal mirror of `motion_glide` (which moves Y and pins X) — P0 glides right
  +2px/frame, CLAMPS, holds, and then the round ends and it snaps back to the start, all with a fixed Y
  band. Measured: X 13→91 in +2 steps, plateau at 91 for 80 frames, reset to 11 at frame 121, Y fixed at
  80–119, 262 scanlines. Two tests in `cmd/harness` pin it, and the reset half pins WHY the trajectory is
  worth preferring: a single read at frame 130 returns 29 while the trajectory over the same span peaks at
  91. That is the Outlaw failure (hold "right" 700 frames, read once, get x=7 near the LEFT edge because
  the round had ended) reproduced with known constants. Negative controls: pinning X to a constant,
  reporting Y into X, and letting Y drift with X each fail the tests. Note the trap liveness does NOT
  cover — the program is reacting the whole time; liveness answers "is it running", not "am I still in the
  situation I set up".

### Changed
- **`checks.motion` certified nothing on its own, and now says so in its own output.** `jerk_rms` is the
  RMS of the position's 2nd difference: 0 for constant velocity, and 0 for an object that never moves —
  measured, `{"axis":"x","max_jerk_rms":0.5}` **PASSES on `litmus_pos`**, whose P0 is pinned at one X for
  the whole run, so the judder regression the gate exists to catch and a completely dead kernel were
  indistinguishable to it. `motion.Stats` now carries `span` = max(pos) − min(pos), the scenario check
  **prints it unconditionally** (a scenario quietly gating a frozen object reports "span 0"), and the new
  `min_span` gates it. Applied to both motion scenarios: `motion_glide` span 39 ≥ 30, `motion_xclamp` x
  span 58 ≥ 50. Negative control: forcing `span` to 0 fails a genuinely gliding object.
- **`beam_intervals`' `crosses_line` was wrong 81% of the time it spoke.** The flag was computed as
  `(minAbs+68)/228 != (maxAbs+68)/228`, which places the scanline boundary at clock 92 of each line
  instead of at the line's start — `MinAbs`/`MaxAbs` are already measured from a WSYNC, i.e. from a
  boundary, so no shift belongs there. Measured over 127 ROMs / 1016 proven writes before the fix: **43
  flags raised, 35 of them false, and 11 real crossings missed**; after, 19 flags with 0 wrong in either
  direction. Concretely, `bullets.asm $F108 GRP0` proves to `[130..-20]` — a window that runs off the end
  of the line and folds into an inverted pair — and was NOT flagged, while `$F0FB GRP0` at `[82..154]`,
  entirely inside one line, WAS. The regression test uses the one direction checkable without restating
  the formula: folding preserves order within a line, so an inverted window is a proof of a crossing and
  must be flagged (13 such windows in the corpus; non-vacuity asserted). Negative control: restoring the
  old expression fails both tests.
- **`breakif` now halts when the beam REACHES a position, instead of silently never halting.** It required
  an exact `(scanline, clock)` match, and observations only happen at instruction boundaries: the CPU
  advances 3 colour clocks per cycle, so **only one phase in three is ever observable**, and a WSYNC kernel
  narrows it much further — measured on `motion_xclamp`, a visible scanline is observed at **7 clocks, every
  one of them inside HBLANK**, so the whole visible region 0..159 could not be stopped on at all. Asking for
  a position in the picture ran to `max_frames` and returned `halted=false` with no error, which is
  indistinguishable from "not yet". It now stops at the first instruction boundary at or past the target
  (measured: asking for clock 80 halts at 82), a position already passed in the current frame is caught on
  the next frame, and an out-of-range clock is an **error** — the tag used to advertise "0-227" while the
  coordinate system is HBLANK −68..−1 / visible 0..159, so 68 of the advertised values did not exist.
  Three tests pin it; negative controls: restoring the equality match, and arming unconditionally, each fail.
- **`beamtrace` now returns every frame it traced, not just the first.** It traced `frames` frames,
  advanced the emulator by all of them, and returned the EARLIEST one alone — measured over the wire:
  `frames=4` starting at frame 5 left the machine at frame 9 and handed back frame 5. The discarded
  frames were unreachable by any other route, because a second call advances the machine again, so
  frame-to-frame comparison — flicker, multiplexed sprites, a first frame that is atypical after setup —
  was impossible in a tool whose description promised the frames. Output is now `frames[]`, each entry a
  `{frame, rows}` pair (was a single top-level `frame`/`rows`). Pass `scanline` to keep the payload narrow.
  Witnessed by `TestBeamtraceReturnsEveryFrameItPaidFor`, which reads a register that provably changes
  every frame (motion_xclamp stages HMP0 = 96, 64, 32 as P0 walks) so that N copies of one frame cannot
  pass; negative controls: truncating to the first frame, and repeating one frame N times, both fail it.

### Fixed
- **`defuse` reports `writes_into_code`** — SD-3's "a store landing in decoded code space is a fact, not a
  guess". Every reachable write whose target set intersects addresses the decoder read as INSTRUCTIONS,
  with the writer's PC and source location. Each entry carries `exact`, because an exact store into code is
  a **fact** while a may-set that merely reaches code is a **possibility** — an indexed store spans up to
  256 addresses and a 4K image is mostly code, so collapsing the two would drown the first in the second.
  **Shipped with a planted fixture rather than a corpus witness, deliberately**: measured first, 133 ROMs
  and **zero** that write into the cartridge window at all, and a detector whose branch nothing reaches is
  not a check. `litmus_smc` plants one; `litmus_smc_clean` aims the same store at RAM. Measured after:
  **123 analysable ROMs including four commercial cartridges, exactly one report — the planted one.** The
  test gates both halves. Negative control: aiming the planted store at RAM fails it.
- **`cover -drive explore`** — cycles SELECT through the game variations, presses RESET, then rotates the
  stick, instead of holding a fixed input. Added because of a measurement: a 2600 attract mode runs the
  **game loop** with synthetic input, so a cartridge left alone already covers most of what playing it
  covers — on Chopper Command, RESET plus a dozen rounds of stick moved the executed count by **4
  instructions out of 2358**. What sitting there does not cover are the other game **variations**, behind
  SELECT: **Seaquest 51% → 60%** of its decoded instructions, Adventure 61% → 67%, Chopper Command
  46% → 49% — all four measured at the SAME driving budget, so they compare drivings rather than state a
  ceiling; given more frames every ROM saturates near 68–78%. The report now carries `drive`, because a coverage percentage is a property of a ROM **and a
  driving**, and two numbers taken under different drivings are not comparable. Panel switches go through
  `SetPanel`, not `SetInput` — ignoring that error is how an earlier measurement "pressed" RESET without
  pressing anything.
- **`prove_line_budget` / `cyclebound.Prove` now accept a raw `.bin`** — the entry point that was missing.
  SD-0c taught the *decoder* to read real cartridges (Outlaw's 2K went from 0 instructions to **931**,
  Combat's from 0 to **838**), but nothing public took a raw image: `Prove` and `timinglint` assemble their
  input, so every commercial ROM came back as *"Unknown Mnemonic"* — measured on Adventure, Seaquest,
  Chopper Command, VideoOlympics and Empire Strikes Back. The capability existed and was unreachable, which
  blocked the casebook line, where the ROMs are commercial by definition. Now measured on real cartridges:
  **VideoOlympics 8 regions, Adventure 14, Seaquest 49, Chopper Command 29, all converged.** A raw image
  loses only what SOURCE carries — `@lines`/`@amax` annotations and label locations — and `srcmap` is
  nil-safe throughout for exactly that. **The `.asm` path is byte-identical** (6 ROMs including a banked
  one, whole JSON output compared), and a test pins that the same ROM yields the same region count and
  worst case through both routes.
- **The collision latches' three unlocked claims are now locked** (`litmus_cxclr` +
  `TestHmclearDoesNotClearCollisions`). The D7/D6 map had a pure-function test; "sticky", "CXCLR clears"
  and "**HMCLR does not**" did not — stickiness appeared only in a comment and the HMCLR distinction was
  checked nowhere, which is the one a reader can actually get wrong, the names differing by two letters
  while both read as "clear something". Measured: CXP0FB is `$82` after the collision, **`$82` still after
  HMCLR**, `$02` after CXCLR. Negative control: swapping the two strobes in the ROM fails the test. The
  fixture also records its own first failure — it lit `PF1` only and positioned P0 with a div-15 loop that
  was never given a target value, so P0 missed the band and all three snapshots read "no collision"; the
  playfield is now lit solid so the answer does not depend on positioning at all.
- **The 6502 page-cross rules are now machine-locked** (`TestPageCrossPenaltyRules`, 11 cases). They were
  cited from 6502.org and never re-measured here, while `cyclebound`'s entire per-scanline proof rests on
  "stores never take the penalty" — and its page-cross costing was where a real under-approximation was
  found earlier the same day. Measured one instruction at a time through the silicon-differential harness:
  STA abs,X is 5 crossing or not, STA (ind),Y is 6 either way, LDA abs,X 4→5 and LDA (ind),Y 5→6 on a
  cross, branches 2 / 3 / 4. **All as documented.** The test also records a trap it fell into itself: a
  FORWARD branch from `$F802` cannot cross a page at all (the largest offset `$7F` reaches `$F881`), so the
  crossing case has to branch backwards — the first version asserted 4 cycles for a branch that never left
  its page.
- **All 20 playfield columns are now verified, not 3** (`litmus_pf_allcols` +
  `TestEveryPlayfieldColumnLandsWhereTheTableSays`). `CLAUDE.md` lists the column→register→bit map under
  "constants you must never get wrong" and cited `litmus_pf`, which lights **columns 0, 4 and 12** — the
  leftmost bit of each register, three of twenty positions and the three easiest. The new ROM draws the
  whole map in one frame (20 bands of 9 scanlines, band k lighting only column k) so every entry is
  checked, including that nothing else lights up — which is what catches a bit landing in the *wrong*
  column rather than in none. Measured: all 20 land on `4k..4k+3` and repeat at `80+4k`, confirming the
  repeat rule and the half boundary at clock 80 at the same time. **The table is correct.** Negative
  control: reversing PF2's byte order fails 8 columns.
- **The 16-nibble HMOVE table is now machine-locked** (`TestAllSixteenHmoveNibblesMoveByOnePixelEach`).
  `CLAUDE.md` lists it under "constants you must never get wrong" and cited a hand verification from
  **v0.4.0** — the existing HMOVE tests cover the ripple counter and the idle/unrecorded distinction, and
  `litmus_hmove` has no scenario, so nothing had held the table true since. Re-measured: all 16 match
  (`$70`=−7 … `$00`=0 … `$F0`=+1 … `$80`=+8), and the test asserts each nibble at the **drawn pixel** via
  `DecomposeRow` as well as at `HmovedPixel`, so a readout that stops describing the picture fails too.
  Negative controls: flipping the sign convention fails 7 nibbles; shifting `DecomposeRow`'s clock by one
  fails the drawn-position half.
- **`scripts/check_tests.py` — a gate on tests that cannot fail** (CI- and pre-push-gated, with a
  `--selftest`, like `check_traps`). A `func TestXxx` must either assert on `t` or hand `t` to a helper that
  does; anything else runs code and draws no conclusion, and `go test` will never say so. Measured when it
  was written: **344 test functions, exactly one with no failure path** — `TestZZProbe`, a scratch probe that
  printed to stdout, asserted nothing, and referenced absolute paths outside the repo. It was swept into a
  docs commit on 2026-07-29 and had been contributing a meaningless green tick since. Deleted; the tree is
  now at 343/343 able to fail. The detector clears delegation (`helper(t, ...)`, `t.Run`) — verified against
  a delegating test that would otherwise have been a false positive.
- **`cmd/dissect` carried two copies of the offset→address mapping and only one knew about banks.**
  `fmtRange` used `off/4096` and `$F000 + off%4096`; the step that matches an annotation to a DiStella
  label by address used the naive `0x10000 - len(rom) + off`, which on an 8K image resolves **every offset
  in bank 0 to `$Exxx`** — an address the 2600 never fetches — so those annotations matched no label and
  were dropped without a word. Both now call one `romAddrOf`, unit-tested across 4K/2K/8K/16K including the
  boundary bytes of each bank; negative control: removing the bank branch fails it.
- **A temporary ROM patch silently patched the wrong bank of an 8K image.** `assert_line_budget`'s
  `patch=` resolved an address to a file offset with `base = 0x10000 - len(rom)`, which puts an 8K
  cartridge's base at **$E000** — an address the 2600 never fetches from — and then resolves every patch
  into the SECOND bank: `$F123` became file offset `$1123`, inside the file, past the bounds check, with
  the range in the error text quoted as "$E000-$FFFF". A measurement taken on a ROM patched in the bank
  that was not running is worse than no measurement, because it is reported as one. Now **declined** with
  a message that says why, the way `defuse` and `beam_intervals` decline banked images; flat 4K/2K images
  are unaffected. Patching by bank is filed, not implemented — `PatchSpec` would need to carry one.
- **`mutate -covered`'s "honest kill rate" mutated the wrong half of a banked ROM.** `CoveredOffsets`
  mapped an executed PC to a file offset with `addr & (len(rom)-1)`, which on an 8K image folds every
  `$Fxxx` into the LAST 4K image whichever bank actually ran. Measured before the fix: on the exerciser
  **all 278 covered offsets landed in `$1000-$1FFF` and not one in `$0000-$0FFF`** — so "restrict fault
  injection to code that actually executed" was injecting into bank 1's bytes while bank 0 was the half
  executing, producing mutants that cannot be killed for precisely the reason `-covered` exists to avoid.
  Now `bank*4K + (addr & 0x0FFF)`, via the new `Coverage.SeenSites()`. The exerciser goes to 315 offsets
  with **49 in bank 0's image**; a 4K ROM is the one-bank case and is unchanged (`smoke.bin` still 38, so
  the published "2% naive vs 68% covered" figure does not move). Negative control: restoring the old fold
  fails the new test.
- **Coverage was bank-blind, and it flattered.** `internal/emu/coverage.go` keyed executed instructions,
  branches and both edge sets on a bare address, but two banks of an 8K image decode the same addresses.
  It failed in both directions at once: as a count it under-reported distinct executed instructions
  (exerciser, 200k instructions: 319 pairs executed, `PCCount` reported **282**, 37 addresses run in BOTH
  banks), and as a query `Seen(addr)` answered "covered" for the twin in the bank that never ran — the
  flattering direction, which VV-3's coverage percentage and `mutate -covered`'s "honest kill rate" both
  rest on. Now keyed on `(bank, address)`, with the fetch bank captured **before** the step (a hotspot
  access changes the mapping as it completes, so asking afterwards attributes the switching instruction to
  the bank it switched to). `Signature()` includes the bank too, so guided fuzzing on an 8K image can tell
  the halves of an address apart. `Seen(addr)` deliberately keeps its meaning ("in SOME bank") and
  `SeenIn(bank, addr)` is added beside it. **Flat ROMs are unaffected — `cmd/cover`'s whole JSON output is
  byte-identical 5/5**; only banked images move (exerciser `pc_executed` 268 → 297).
- **The prover's most important soundness check — "the machine never exceeds the proven worst case" — was
  bank-blind and ran on 31 ROMs.** It keyed proven regions on address alone while `LineWorst` has carried
  `Bank`/`BankValid` all along, so on an 8K image a region proven in one bank could be paired with a
  measured row from the other. The dangerous direction is the quiet one: an accidental pairing that happens
  to satisfy `observed <= proven` **hides** a real gap, which is the failure this test exists to catch, and
  `banked_game` is in its corpus. Now keyed on `(bank, address)` and run over the whole tree:
  **896 measured regions across 128 ROMs, no exceptions** (was 228 across 31), for 5.5s.
- **The two headline soundness gradings ran on 31 of ~129 ROMs, and extending them roughly tripled the
  evidence for free.** `defuse`'s "9055/9055 observed (pc,addr) pairs inside their predicted sets" and
  `beam_intervals`' "7117/7117 observed writes inside their proven window" were both measured over
  `roms/techniques/*.asm` alone — a denominator neither number stated. Run over the whole corpus they read
  **32655/32655** and **19143/19143**, still zero violations, costing 4.7s and 3.2s. The quoted figures in
  `CLAUDE.md` now name their corpus. A side effect: `defuse`'s CFG-reach-gap report now also names
  `litmus_6502.asm` (66 writes from instructions the decoder never reached), which had been invisible.
- **The blank-classification grading ran on 32 of 129 ROMs, and two defects in the grading itself were
  what kept it there.** It is the one verdict in the package that can hide a real scanline tear, so its
  corpus matters. (1) Its `blank` map was keyed on a region's **address alone**, and an 8K image decodes the
  same addresses in both banks — so on a banked cartridge it matched whatever sat at `$Fxxx` in the *other*
  bank and graded unrelated code. `banked_game` is in the original corpus, so part of the old number was
  aimed at the wrong instructions. (2) It sampled the display state **at** the region's opening `sta WSYNC`,
  but a TIA register write is delayed (`futureVblank`), so a `sta VBLANK` issued one instruction earlier has
  not reached the signal yet — measured, `DisplayOff()` is false at the strobe and true one instruction
  later for a region that is genuinely blanked. Both fixed; the corpus now runs the litmus and exerciser
  ROMs too: **128 ROMs, 133,684 executions of blank-region entry points, 0 disagreements** (was 32 ROMs).
  The per-ROM budget went 400k → 120k instructions in the same change, measured: 180s → 57s, with *blank
  regions never reached* — the number that says what this grading does not see — unchanged at **1**.
- **`emu.DisplayOff()` ignored VSYNC.** It read only `sig.VBlank`, while the prover's own `displayOff()` is
  `VSync || VBlank` — and VSYNC blanks the picture as surely as VBLANK does. Measured while extending the
  blank-classification grading past the technique corpus: **6730 "disagreements" appeared, every one on a
  frame's VSYNC lines** in a ROM that raises VSYNC without also raising VBLANK. The prover was right and the
  oracle was short a term. The error direction was false ALARMS, never missed detections, so nothing unsound
  had shipped; what it had done was silently cap that grading to ROMs which happen to raise VBLANK during
  VSYNC — the one verdict in the package that can hide a real scanline tear. Its only consumer is the
  grading test, which still passes unchanged (144,568 executions across 32 ROMs, 0 disagreements). The
  corpus extension is not shipped: 776 disagreements survive, and the evidence points at the oracle's
  sampling point rather than the prover, but that is filed as a hypothesis with its numbers, not acted on.
- **Four more descriptions made to match measured behaviour** (from the 38-tool sweep; no behaviour change).
  `step_scanline` said "CPU cycles consumed across that scanline" for a figure that **excludes WSYNC stall
  time** — measured 8 on twelve consecutive 76-cycle lines, so the remainder read as headroom that does not
  exist. `assert_line_budget`'s `line_cycles` is `scanlines × 76`, a quantised figure and never a measured
  count, so subtracting the budget from it yields a number of cycles to cut that means nothing.
  `read_audio` returns `note0`/`note1` = {note, cents} and said so nowhere, although it is the only
  register→pitch conversion in the whole tool surface. `analyze_image` told the reader "one screenshot is
  one frame of truth (flicker objects appear partially)" while accepting `paths[]` and running a
  multi-frame pipeline with an explicit flicker report — the description denied the one capability a
  flicker-hunter would search for, and its three inputs shipped with **no descriptions at all** on the wire.
- **`spritey`'s description advertised half of what the tool returns.** Its multi-frame mode was documented
  as returning "the per-frame Y trajectory", but every `SpriteYSample` has carried `X` (HmovedPixel) since
  the tool existed. Someone looking for a HORIZONTAL trajectory therefore did not find the tool that already
  had one. Measured cost on 2026-07-30: both attempts to pin down Outlaw's horizontal clamp were hand-rolled
  against `read_tia` instead, and one of them read the position once after holding an input for 700 frames —
  long enough for the round to end and the sprite to be reset — producing a confident, stable, wrong number.
  The description now names the X trajectory and warns against the single late read. No behaviour change.

## [1.117.0] - 2026-07-30

**Release-hygiene cut.** Everything that had accumulated under `[Unreleased]` is versioned here as one
**MINOR** release. It adds backward-compatible capability (new MCP tools and new `cmd/` tools), carries one
prover **soundness fix** (a cycle-cost under-approximation in `cyclebound`), and adds tests and litmus ROMs.
Not MAJOR: no exported Go API and no MCP tool was removed or re-signatured — the soundness fix changes the
numbers `prove_line_budget` reports, not its contract, and that contract always was "a sound upper bound over
all paths", so making it finally hold is a fix. Not PATCH: new functionality ships here. The number is
`1.117.0` (not `1.108.0`) because `internal/version/version.go` already ships `const Harness = "1.117.0"`,
and that constant is stamped into the MCP `serverInfo` and into `ramtrace` provenance headers — the CHANGELOG
is matched to the artifacts already produced rather than the other way round.

**Tag/CHANGELOG drift measured at this cut** — 170 tags vs 174 released sections:
- `v1.104.0` / `v1.105.0` / `v1.106.0` / `v1.107.0` were **tagged but never given a `##` section**. Their
  entries are folded in below and keep their inline `(v1.10x.0)` markers, so which work shipped when is not
  lost. `v1.105.0` (`read_ram_trace`; tag at `aab7ab7`, 2026-07-21) has **no CHANGELOG text at all** — it is
  recorded here as a known gap rather than reconstructed after the fact.
- `1.108.0`–`1.115.0` were named inline inside `[Unreleased]` but were never sectioned and never tagged; they
  ship here as part of `1.117.0`.
- `1.80.0`–`1.102.0` (23 versions), plus `0.5.1` and `0.6.0`, have `##` sections but **no tag**. Deliberately
  **not** tagged retroactively: a tag must point at the commit that shipped that version, and that mapping is
  not recoverable from the CHANGELOG alone.
- `1.74.0` and `1.116.0` were skipped entirely — no section, no tag, no mention anywhere.

### Added
- **`cyclebound` proves a WSYNC-to-WSYNC region that CROSSES A BANK SWITCH** (SD-11, stage 3 of bank
  support). Stage 2 closed the DECODE over bank switches; the flow was still refused, so the region where a
  bank-switched kernel does all of its cross-bank work had no number at all. It now has one, and the number
  is graded against the emulator rather than against the prover's own arithmetic.

  | ROM (mapper, banks) | crossing region | before | proven after | machine measured |
  |---|---|---|---|---|
  | `litmus_bank` (F8, 2) | bank 0 `$F02B` | REFUSED | **54** | **54** / 1 line |
  | `litmus_bank_f6` (F6, 4) | bank 0 `$F02B` | REFUSED | **72** | **72** / 1 line |
  | `litmus_bank_f4` (F4, 8) | bank 0 `$F02B` | REFUSED | **128** (violation at 76) | **128** / 2 lines |
  | `banked_game` (F8, 2) | bank 0 `$F01B` | REFUSED (switch) | still unbounded, **different reason** | 28 / 1 line |

  The three litmus kernels are deterministic, so proven EQUALS measured on all three — not merely
  proven >= measured. `litmus_bank` and `litmus_bank_f6` now come back `certified:true`; `litmus_bank_f4`'s
  chain genuinely spends more than one scanline (its source compensates with `ldx #29`), so it is reported
  as a stated 128-cycle budget violation rather than a refusal.

  - **Code identity is now `(bank, address)`, everywhere.** Every bank of a bank-switched cartridge is
    mapped into the same `$F000-$FFFF` window, so a bare address cannot tell two banks apart — measured,
    `litmus_bank_f4` has **1399 of 1427 decoded addresses claimed by 2+ banks**. DATA addresses stay flat on
    purpose: RAM `$80-$FF`, TIA/RIOT `$00-$3F` and the page-1 stack mirrors are not banked, so a bank in a
    data key would split one physical cell into N.
  - **THE EDGE comes from the engine, not from folklore.** An instruction whose DATA access reaches a
    bank-switch hotspot continues at the SAME ADDRESS IN THE TARGET BANK: the Atari mapper switches on the
    access and does not touch PC (`mapper_atari.go` runs `bankswitch(addr)` and then returns
    `cart.banks[cart.state.bank][addr]`). The switching instruction is charged its ordinary cost and the
    edge is charged nothing. When the access is EXACT the intra-bank fall-through is REPLACED, which is what
    makes the numbers exact rather than merely safe; a wide footprint keeps the fall-through as well.
  - **One oracle decides both the edge and the refusal.** `switchModel.switchEdges` is the single point;
    `residualSwitchRefusal` and every walker (collect, longest, the abstract interpreter, the beam pass,
    `determineBound`'s predecessor scan) are driven off it, because a successor function modelling a switch
    the refusal does not guard is silently unsound.
  - **Still refused, counted in `unmodelled_switches`, still blocking `certified`:** an instruction whose
    OWN BYTES span a hotspot (the opcode comes from the new bank, so the decoded instruction is not the one
    that executes), a `jmp`/`jsr` INTO a hotspot, an unresolvable indirect access under a hotspot-bearing
    mapper, a hotspot symbol that does not name a bank (`B0S0`, `RAM0`), a target bank outside the analysed
    set, and a landing address outside cartridge space. New `modelled_switch_edges` prints beside the
    refusal count, because "0 refused" is also what a cartridge that crossed nothing reports.
  - **The geometry is checked, not assumed.** `analysisUnits` declines any mapper whose banks are not the
    whole 4K window at `$F000` (`len(Data) == 4096`, `Origins == [$F000]`). M-Network is the trap that
    parses: it publishes `BANK0..BANK6` as bank-switch hotspots while its banks are 2K at TWO origins, so
    "the same address in the target bank" is false there — and under stage 3 a wrong seed is no longer just
    an over-decode, it is a CFG edge the longest-path walk follows, and a wrong edge can SHORTEN the
    longest path.
  - **`romTableRange` is routed by bank and refuses a hotspot byte.** On a merged program a flat reader
    would fold whichever bank was bound, so a `lda table,x` in bank 1 could be bounded by bank 0's table —
    a narrow, confident, wrong range feeding a trip count. A footprint containing a hotspot is `Top`,
    because the hardware switches first and returns the TARGET bank's byte there.
  - **`determineBound` keeps SD-9's guarantee across a bank boundary.** Its predecessor scan follows
    cross-bank edges and returns 0 unless the predecessor set is complete (an incomplete set
    under-approximates the entry value, hence the trip count, hence the worst case); both address-order
    filters are now SAME-BANK comparisons, because addresses in different banks have no order at all; and
    the `lda #imm` address proxy still live on the BCS/BCC path is same-bank-gated and COUNTED — measured
    **0 hits** across all 31 technique ROMs, all 12 `cb_*` litmus, `litmus_bound_proxy` and every bank ROM.
    `foldLoops` refuses outright any loop whose body contains a switching instruction: the folded cost
    assumes every iteration executes the same bytes, and after a switch iteration 2 does not.
  - **An unmodelled switch WIDENS its possible landing sites to `Top`.** The value domain is a
    whole-program fixpoint while a refusal is per-region, so refusing the region that contains a switch does
    not protect the region that contains its landing. `switch_widened_sites` + `switch_widen_reasons` report
    it: measured **5 sites on `litmus_bank` and 5 on `banked_game`**, all from decoded-but-never-executed
    filler bytes at bank 1 `$FFF6/$FFF8/$FFF9`, and **0 on `litmus_bank_f6`/`_f4`**.
  - **Merged fixpoint, measured before and after:** `litmus_bank` 151 sites / 17 ms, `litmus_bank_f6` 4171 /
    24 ms, `litmus_bank_f4` **9763 / 19 ms**, `banked_game` 134 / 47 ms — `converged:true` on all four, with
    the iteration cap now scaling with the program (`iterCap`) instead of being a fixed 300000 sized for
    ~1.4k per-bank nodes.
  - **Three new litmus ROMs, each closing a hole a test could not otherwise see.**
    `litmus_bank_shared_addr.asm` makes two banks execute DIFFERENT code with DIFFERENT costs at ONE address
    inside one region (measured: 38 executed `(bank,pc)` pairs over 35 distinct PCs — the first ROM in the
    corpus that can catch a flat-keyed instrument at all; proven 58 = machine 58).
    `litmus_bank_bound.asm` puts a counted loop in bank 1 whose ONLY initialiser is in bank 0, so an
    intra-bank predecessor scan loses the bound (proven 70 = machine 70).
    `litmus_bank_unmodelled.asm` keeps a switch that can NEVER be modelled (an `sta (ptr),y` whose target no
    address analysis can pin down), so the certification gate still has a witness — without it the gate
    would pass with the gate deleted.
  - **`DefUse` now DECLINES a bank-switched image** instead of computing MayWrite/Writes/Regions from the
    flat 8K fold while suppressing only the uninitialised-read pass. Measured on `litmus_bank`, that path
    produced `may_write: []` for a cartridge that demonstrably writes `$80/$81/$82` — empty by LUCK, because
    the flat fold decodes almost nothing, which is exactly the shape SD-7 condemned in `Prove`.
  - **`srcmap`'s package doc corrected to what the code does.** It claimed banked ROMs return an empty
    string; measured, they return a label-only string built from the `.sym` file. DASM's listing address
    column is the PHYSICAL ROM OFFSET on a banked image, so bank 0's line numbers are dropped and banks
    1..n's offsets are stored as if they were CPU addresses — which makes `@lines`/`@amax` INERT on every
    bank-switched kernel. New `source_annotations` says so out loud, because `litmus_bank_f4`'s 128-cycle
    violation is over budget only for want of an `@lines 2` the map cannot read.
  - `ProverVersion` -> `cyclebound/3 (VV-2 abstract-interp WCET + @lines + cross-bank flow)`.
  - **Golden diff, mandatory:** cyclebound JSON for all 31 `roms/techniques/*.asm`, all 12
    `roms/litmus/cb_*.asm`, `litmus_bound_proxy` and `litmus_superchip`, before vs after: **44 of 44 FLAT
    ROMs byte-identical**; only the 4 bank-switched images changed. `litmus_bound_proxy` still proves 1015
    (the SD-9 lock) and `litmus_superchip` is still declined.
  - **What this does NOT do.** `banked_game`'s crossing region is still unbounded — but for a different and
    unrelated reason, now stated: its bank-1 loader uses an `iny`/`cpy #8`/`bne` trip count this prover does
    not recognise, so the refusal moved from "region can switch banks" to "loop bound unknown", with a
    conditional obligation naming *the loop at bank 1 `$F00A`*. Its other unbounded region
    (`KRow+0`, "WSYNC inside loop body") is untouched. `BeamIntervals` and `Lint` still decline a
    bank-switched image rather than presenting bank-0-only windows as the cartridge's. `foldLoops` still
    refuses a region containing more than one back edge, which a real cross-bank kernel with a loop in each
    bank will hit.

- **`cyclebound` closes its decode over bank switches** (SD-8b, stage 2 of bank support). Stage 1 decoded each
  bank only from its OWN reset/NMI/IRQ vectors, but a worker bank is entered by the trampoline that switched
  to it. Measured residue, executed `(bank,pc)` pairs absent from the decode: **`litmus_bank` 4 of 36**
  (bank 1 `$FF03/$FF05/$FF07/$FF09`), **`banked_game` 1 of 61** (bank 1 `$FF83`), `litmus_bank_f6` 0 of 41,
  `litmus_bank_f4` 0 of 57 — **now 0 on all four**.
  - An instruction whose memory access reaches a bank-switch hotspot continues at the FOLLOWING address in
    the bank that hotspot names, and since each bank is already analysed as its own 4K image, that address is
    just another decode entry point: **no `map[uint16]Instr` key changes**. A read switches as a write does
    (`lda $FFF9`), targets fold through `cartHotspotKey` so `$FFF9`/`$1FF9`/`$3FF9` are one hotspot, and the
    fixpoint (seeding B can reveal a switch in B that seeds C) closes in **2 rounds on all four ROMs** against
    a cap of 8, with `cross_bank_seed_capped` so a capped run cannot read as a closed one.
  - **The target bank is parsed from the mapper's own symbol, never guessed.** `emu.BankSwitchHotspots()`,
    measured: F8 `$1FF8=BANK0 $1FF9=BANK1`, F6 `$1FF6..$1FF9`, F4 `$1FF4..$1FFB`. The whole symbol must be
    `BANK<digits>`, because Parker Bros publishes `B0S0` (bank-in-segment) and M-Network `RAM0` (cartridge
    RAM), for which "the same address in the other bank" is not where execution lands; those are reported in
    `unresolved_hotspots` and seed nothing. An access whose target cannot be resolved at all has no symbol
    and no bank, so it is counted in `unresolvable_switch_accesses` rather than guessed.
  - **This improves the DECODE, not the flow model.** `hotspotRefusal` is unchanged, `UnmodelledSwitches`
    still gates `Certified`, and all four bank ROMs still report `unmodelled_switches: 1, certified: false`.
  - New report fields (`cross_bank_seeds`, `cross_bank_seed_rounds`, `cross_bank_seed_capped`,
    `unresolved_hotspots`, `unresolvable_switch_accesses`, `bank_coverage[].seeded_entries`) are all
    bank-only. **Golden diff over 31 technique + 12 `cb_*` litmus ROMs: 42/43 byte-identical**, the one
    change being `banked_game`, the only banked image in the set.
- **`framegen` follows a sprite that moves down the frame — a zone-structured kernel** (RL-8b, the last RL-7
  limit). It carried one reset X per object and strobed RESxx once; it now emits a replay loop per zone with
  RESxx/HMOVE placement in the target's own blank lines. **`zone_multiplex` 380 → 0 cells, pixel-exact**
  (BG 33808/33808, P0 228/228, P1 204/204).
  - **Scope is measured, not assumed.** The extractor records the per-line reset X as a series and folds it
    into bands with the gap before each. A boundary costs one scanline per object placed + 1 HMOVE line + 1
    replayed blank line: `zone_multiplex`'s six bands per player have gaps 11/9/9/9/9 against a need of 4 and
    fit; `dyn_multisprite`'s P0 changes 48 → 78 at line 142 with **gap 0** and `road`'s M0 takes 27 bands of
    which 25 have gap 0, so both are refused with the counted reason instead of approximated.
  - **Five of the eight "placement differs" ROMs were never a per-zone problem** — `rts_dispatch`, `bitmap48`,
    `score6`, `text12`, `text24` all measure **1 reset X per player**, and `hscroll` draws no player at all.
    RL-7's "the 8 share one cause" was wrong about them.
  - Three defects found by measuring the clone: the prologue's div-15 `SetXPos` needs `k=11` to reach reset X 4
    and then spends TWO scanlines (263-line frame, 72 cells wrong that were not a positioning error) — replaced
    with a branch-free fixed-cost block strobing at `2n+3`; the reset marker is up to a pixel from the drawn
    window (same 8-px line reads X 49 span 49..56, X 117 span 116..123), so both sides are now anchored on the
    leftmost drawn pixel (12 cells); and the historical block order puts `GRP1` at visible clock +37, too late
    for a band at X 4 (64 cells).
  - **Regression gate held:** 22/31 technique ROMs + Outlaw and Combat (with and without `-reset`) pixel-exact,
    **262 scanlines on 35/35 runs**, `cyclebound certified:true` on 35/35 (74/76, the zoned one 66/76), and the
    generated source **byte-identical on all 34 non-zone runs** (`dyn_multisprite`/`road` gain only the new
    `NOT REPRODUCED: per-zone X` note).

### Fixed
- **`framegen` printed a cause it had not measured, and replayed a single frame-final NUSIZ** (RL-7c). On
  `roms/litmus/litmus_nusiz_all.bin` it reported 2666 of 34240 cells wrong and explained them with a fixed
  sentence — *"this is placement, not omission (one X per player cannot follow a per-zone multiplexed
  target)"* — printed unconditionally on every non-exact run. That ROM places both players once before the
  frame loop and never moves them (measured: **1 distinct reset X each**, over 191 and 190 drawn lines), so
  the sentence was false there and was never evidence anywhere.
  - **The obvious suspect was falsified first.** `nusizWidth` returns 1 for the five NUSIZ COPY modes, which
    looks like a bug and is not: the copies are hardware replication of the same 8-bit byte. Eight probe
    ROMs, one per mode held CONSTANT for a whole frame, reproduce **pixel-exact in all eight** (P0 cells
    864/864, 1728/1728, 1728/1728, 2592/2592, 1728/1728, 1728/1728, 2592/2592, 3456/3456 for modes 0..7).
    The real cause: `extract` read NUSIZ once at the end of the rendered frame, and litmus_nusiz_all ends in
    mode $07, so all 214 lines came out quad-width.
  - **Diagnosis.** Per visible line, for each player, the extractor now measures the NUSIZ in force, the reset
    position and the number of separate runs `DecomposeRow` reports — and takes the same measurement off the
    **clone**. The RESULT line names a cause only when the number proving it was counted: copies (`the target
    orders up to 3 copies (NUSIZ $06 on 37 lines) and the clone draws up to 1`), multiplexing (**only** when
    distinct reset X > 1, listing the positions and their line counts), or a late write (the kernel's own
    store landing past the object's leftmost pixel, arithmetic on the emitted block schedule). When none is
    measurable it says so. The NUSIZ size shift is removed before counting positions — HmovedPixel moves ±1
    on a 1x↔2x change without the sprite moving (measured: 24 for modes 0,1,2,3,4,6 and 25 for 5 and 7).
  - **Reproduction.** A varying per-line NUSIZ is now replayed from a table. Room is made by dropping
    playfield writes the target provably does not need, decided per PF register (both halves 0 on every line,
    or right half equal to left on every line). A left write is never dropped alone.
  - **Result: 2666 → 2 cells** (P0 3616/3616, P1 3120/3120). The 2 are reported, not absorbed: the target
    clears GRP1 part-way along scanline 228 leaving a 10-pixel P1 run, and a kernel writing each register
    once per line in HBLANK can draw only 8 or 12 there at quad width.
  - **Where it gives up, with numbers.** `rts_dispatch` would need 9 write blocks. Nine run on hardware
    (3+7·9+7 = 73 of 76; the 9-block clone measured 262 scanlines and 376→8 cells), but `cyclebound` bounds
    `lda abs,y` at 5 because it cannot assume the tables' `align 256`, scoring it 82 against 76 and refusing
    to certify. The kernel is capped at the certifiable 8 blocks and the tool reports what it dropped and why.
  - **No regression.** With NUSIZ constant down the frame — 30 of 31 technique ROMs, Outlaw and Combat — the
    historical eight-block layout is emitted unchanged (verified by diffing the generated sources; only the
    new per-block deadline comments differ). Corpus after: **21 pixel-exact / 8 differ / 2 partial, 262
    scanlines on 31/31**, every cell count identical to before; Outlaw and Combat still pixel-exact with and
    without `-reset`.
- **`prove_line_budget` called VBLANK-time code a visible-line tear whenever a subroutine had two call
  sites** (v1.115.0). Found while running a generated clone through the prover: the ordinary two-sprite
  shape — both players placed through one shared `SetXPos` — came back `certified:false` with the routine
  classified `visible`, while the *identical kernel with one call site* certified. That is the shape every
  two-sprite kernel has, including this project's own Outlaw and Pizza Boy builds.
  - Cause: `absSuccessors` reset a JSR's return point to full Top, discarding facts the callee cannot
    change. VBLANK went unknown at the *second* call site, that unknown flowed into the shared subroutine,
    joined with the known-on state from the first, and the routine's own entry state came out unknown.
  - Fix: keep VSYNC/VBLANK across a call when the callee provably cannot write them. The rule is one-sided —
    an unresolvable store, a push whose SP range can reach $0100/$0101 (page 1 mirrors the console's own
    addresses, so the Stack Trick is a real display write), or a nested call it has already visited all
    answer "not preserved". Indexed stores are resolved through the index range, which needs ranges that do
    not exist on the first pass, so `computeStates` now runs twice; the second pass only adds a fact
    justified by a sound first-pass range.
  - **A second, pre-existing hole fell out of the litmus**: `regionTouchesDisplay` tested the raw operand, so
    it saw only non-indexed writes — `sta VSYNC,x` is AbsoluteX and returned no address at all, letting a
    region that writes VBLANK be skipped as blank. Both checks now share one resolution path.
  - Corpus effect (31 technique ROMs): three false positives removed (`game_states` now certifies;
    `bullets` ×2 and `sfx_demo` reclassified to blank — each verified by reading the call context, e.g.
    `bullets` calls `PosObj` twice between VBLANK-on and VBLANK-off), and one region moved the *conservative*
    way (`rts_dispatch`, 55 ≤ 76, no violation) because indexed stores are no longer invisible to the check.
    The real 89-cycle interval in the generated clone is not lost — it moves to `blank_over`, i.e. frame-line
    drift rather than a torn line, which the `ntsc_frame_lines` check owns.
  - Graded against the machine, not against itself: new `blankclass_test.go` runs every corpus ROM and asks
    the television (`GetLastSignal().VBlank`, via new `emu.DisplayOff()`) whether the beam is really blanked
    each time execution reaches a blank-classified region's opening WSYNC — **129,936 executions across 31
    ROMs, 0 disagreements**, with the 1 never-reached region reported as not covered. Negative control:
    forcing `displayOff()` to true makes it fail with 28 disagreements, so the test can fail.
  - New twin litmus `roms/litmus/litmus_jsr_display.asm`: three routines of identical shape differing in one
    store (`sta COLUP0,x` / `sta VSYNC,x` / `sta VBLANK`), all called from the same place with the same index
    values, so a rule that answers the same way for all three is wrong whichever answer it gives.

### Changed
- **`framegen` now reports what it did NOT reproduce** (v1.115.0, audit RL-7). The field evaluation of
  `framegen` found the generator sound but the *report* misleading: its only output was a single
  `element match N / 34240` line, which on Fishing Derby read **96.9%** followed by `wrote clone.asm` — while
  the fisherman was 11% correct (P0 75/665 cells) and the hook and line were absent entirely. Background is
  77% of the visible area, so it carries the headline number and buries everything the reproduction is for.
  The cause is structural, not a tuning error: the emitted kernel writes PF + GRP0/GRP1 and **no
  `ENAM0`/`ENAM1`/`ENABL` at all** (`grep -c` over every generated clone: 0), and it carries one X per player
  for the whole frame, so a per-zone multiplexed sprite cannot be followed.
  - Per-element coverage is now measured against the clone's own rendered frame and reported in three places:
    the terminal, a `; NOT REPRODUCED:` block burned into the generated `.asm` banner (the file outlives the
    terminal it came from), and the **exit code** — 1 when incomplete, matching `vismatch`/`behavmatch`.
  - Structural absence (`clone 0` cells) is reported separately from misplacement (`clone > 0`, wrong cells):
    different causes, different fixes.
  - Field results, `-frames 28`: **21/31 technique ROMs pixel-exact**; 8 misplaced (`zone_multiplex` loses 190
    cells per player); 2 missing elements (`shared_setxpos` M0 1712 / M1 1712 / BL 428, `road` M0/M1/BL).
    Cartridges: **Outlaw and Combat pixel-exact, Fishing Derby partial.** Verdict recorded: a sound
    **BG/PF/P0/P1 validator for single-position kernels**, not a whole-frame reproducer.
  - Also fixed: a tie in the vertical-shift search resolved to the first candidate scanned, so "no offset
    explains anything" came out as "shift the picture up 4 lines" (`motion_glide` scored 34232 at all nine
    offsets and chose −4). Ties now resolve to 0.
  - Follow-up filed as **RL-8**: missile/ball replay and per-zone sprite X.
- **`framegen`: the last visible scanline no longer loses its sprites, and generated frames are 262 lines**
  (v1.115.0, audit RL-7b). Both faults were found by running the generated clone through `cyclebound` and
  `beamtrace` instead of only looking at its picture — a pixel comparison structurally cannot see either.
  - `cyclebound` put the `Kern` region at **97 cycles against a 76-cycle budget**; the loop body is 66 and the
    missing 31 are the loop-exit cleanup falling through *before* the next WSYNC. `beamtrace` on the clone
    shows it landing at `clk +133 GRP0` and `clk +142 GRP1` on the last visible line, so a sprite pixel right
    of clock 133 survives 213 lines and vanishes on the 214th. Fixed with a `sta WSYNC` before the cleanup;
    the clears now land in the next line's HBLANK (clocks −53..−17) and `Kern`'s worst drops **97 → 66**.
  - Proving it needed two litmus attempts, and the failed one is worth recording: a full-width *playfield*
    exposes nothing, because PF2 — the only PF register covering clocks 128-159 — is cleared after the line
    has ended. Only GRP0/GRP1 are early enough to bite. New `roms/litmus/litmus_lastline.asm` parks a player
    near the right edge of every visible line instead, sized to fill the 214-line snapshot window so the last
    extracted line is a drawn one.
  - Frame length: the pre-fix generator emitted **267 scanlines on 30 of 31 corpus ROMs and 268 on the other —
    262 on none**, five to six lines out of NTSC spec, which rolls on a real television. Invisible to every
    existing check because the *picture* was pixel-exact. Overscan ignored `vblankAdj`, and more
    fundamentally no formula can be right: `SetXPos` is a div-15 subtract loop, so a player far to the right
    costs more prologue than one on the left (Combat, P1 at clock 145, spends one line more than Outlaw).
    Frame length now **self-calibrates against `StepFrame()`** like X and VBLANK already did, is reported every
    run, and exits 1 when wrong. **After: 262 on 31/31**, pixel results unchanged (21 exact / 8 misplaced /
    2 missing). Locked by `roms/litmus/scenarios/lastline.json`.

### Added
- **Static program analysis — def-use, proven beam windows, conditional bounds, and the tools to check them**
  (v1.114.0). A night of work whose theme turned out to be that several existing tools were answering
  confidently and wrongly, and that only the machine could say so.
  - **`defuse` (MCP + `internal/cyclebound`)** — which instruction writes which address, over ALL paths, per
    WSYNC-to-WSYNC region, with may/must separated. Targets resolve through the EFFECTIVE address, so an
    indexed store is attributed to the register it reaches and a push lands wherever SP points. Also reports
    reads of RAM no path from reset has definitely written. Soundness is graded against the emulator:
    9055/9055 observed (pc,addr) pairs inside their predicted sets across the corpus.
  - **`beam_intervals` (MCP + `internal/cyclebound`)** — the forall version of `beamtrace`: every TIA write
    with the earliest and latest beam clock it can land at. 7117/7117 observed writes inside their proven
    window; 327 bounded writes, 106 exactly positioned, mean window 8.7 colour clocks. Nothing in the 2600
    ecosystem computes this; the state of the art is hand-counting one path.
  - **Conditional cycle bounds** — of 29 unbounded regions, 15 fail only because a loop's trip count is
    unknown, so the largest count that fits the budget is computable: *"within 76 cycles provided the loop at
    $F126 runs at most 11 times"*. Checked for tightness by re-deriving both edges. Never certifies.
  - **Stack-pointer tracking** — `TXS/TSX/PHA/PHP/PLA/PLP/JSR/RTS`, so the 2600 stack trick (SP aimed at a
    TIA register, PHA as the store) is visible to both the static and dynamic sides for the first time. They
    now agree to the clock.
  - **Call-context resolution** — a region opened by a WSYNC inside a subroutine is analysed once per call
    site and the worst taken; unbounded regions 29 -> 24.
  - **Sweep-loop recognition** — the `ldx #$FF / sta $00,x / dex / bne` idiom is a must-write of its swept
    range at the loop EXIT (minding the fencepost: `bne` leaves before storing at index 0). Uninitialised-read
    false positives 3783 -> 0 while the planted case still fires.
  - **Raw `.bin` support** — `program.canon` folds an address to a cartridge offset through the memory map,
    so a 2K cartridge decodes at every mirror the console sees it at.

### Fixed
- **`ProfileLineWorst` missed 44% of WSYNC strobes** (v1.114.0). It detected a strobe by the CPU stalling,
  and a WSYNC whose stall is shorter than one instruction step never shows that transition — so the interval
  it should have closed ran on to the next visible strobe. Measured: `$F0D0` in bitmap48 executes 192 times
  in 8 frames and 108 intervals were counted; the longest bogus interval spanned 13 lines and 987 cycles,
  which made the cycle prover look unsound. Detecting the WSYNC store fixes it (restricted to steps that
  retired an instruction, since `LastResult` is unchanged during a stall). Now 184 = 192 minus the 8 dropped
  at frame boundaries, worst 87 cycles over 2 lines, inside the proven 93.
- **`LastTIAWrite` attributed indexed stores to the base register** (v1.114.0). `sta COLUP0,x` was reported
  as COLUP0 whatever x held. On our own `shared_setxpos` kernel, five objects collapsed into player 0 over
  two frames; they now separate correctly. New litmus `litmus_indexed_tia.asm` turns the background green
  through an indexed store, so the screen arbitrates.
- **The abstract interpreter was not sound** (v1.114.0). An indexed or indirect store left previously-tracked
  cells standing, so a later load read a stale value into loop bounding and branch refinement — a "proven"
  worst case could sit BELOW the machine's. Stores now kill their may-set. No verdict changed on the 31
  technique kernels, so past certificates stand.
- **A capped fixpoint was silent** (v1.114.0). `computeStates` stopped at its iteration cap without saying
  so; nothing derived from an unconverged run may certify.
- **`cover` divided by branches OBSERVED** (v1.114.0), so an unreached branch left the arithmetic and the
  percentage rose as the test got worse. `divtable` reported 100% edge coverage with 12 of its 17 branches
  never executed. It now divides by the branches the program has, names the unreached ones, and says when
  the decoder itself is incomplete.

### Decisions
- **Check the instrument before believing what it says about the thing under test.** Twice in one night a
  faulty `ProfileLineWorst` nearly cost something real: it made the cycle prover look unsound (recorded as
  the top open defect, then retracted), and it caused a CORRECT improvement to be reverted as unsound
  (text12/text24 measured 143 against a proven 110; with the profiler fixed the same region measures 104).
  Both errors have the same shape. Recorded in `docs/capability-gap-audit.md`.
- **A detector is only worth its reports if it can stay silent.** Uninitialised-read detection was written,
  measured at 3783 false positives on one kernel, and deliberately NOT shipped until sweep-loop recognition
  brought it to zero — with a pair of litmus ROMs differing in one operand to prove both directions. The
  same rule sent an SMC detector back to the backlog: zero stores land on decoded code across 31 kernels and
  two commercial cartridges, so there is nothing to demonstrate it on.
- **`ramtrace` + the RAM-equivalence gate — the measurement half of behavioural reproduction** (v1.113.0).
  `vismatch` asks whether a build LOOKS like the target and `behavmatch`'s trajectory diff asks whether it
  MOVES like it; this answers the prior question — what the machine's 128 bytes of state are doing — so a
  commercial game's logic can be re-authored one rule at a time and each rule gated numerically.
  - **`emu.CurrentRAM`** reads all 128 bytes ($80-$FF) in one call. The point is not speed: it removes the
    need to DECLARE which addresses are interesting, which is precisely what is being measured.
  - **`emu.StartFrameWatch`/`FrameWatch`/`FrameWatchSPRange`** accumulate, inside the frame, every collision
    that OCCURRED (via the per-videocycle event, independent of `CXCLR`) and the range the stack pointer
    travelled. Observation-only, proven not to change a RAM byte or a cycle count.
  - **`cmd/ramtrace`** — `record` (full per-frame series + held input + collisions + SP range, as
    provenance-stamped JSON), `activity` (per-byte descriptive statistics, fitting nothing), `arity` (the
    smallest feature set that determines each byte's next value, with the LOCATIONS of any contradicting
    transitions and `-skip` to separate power-on initialisation from gameplay).
  - **`behavmatch -ram-gate`** reports the first frame and address where a build's RAM stops matching the
    target's — a debugging address instead of a downstream symptom. It compares a mask, never all 128
    bytes, because two correct implementations legitimately differ in scratch and leftovers; every verdict
    prints what was excluded and why, and a pass over nothing is labelled VACUOUS.
  - **Scenario library rewritten** as ROM-agnostic scripts covering both players, tap-vs-hold fire,
    diagonals, aimed fire, simultaneous fire, a 900-frame duel and the console switches. Scripts can no
    longer name a game variable, so they can no longer name a wrong one.
  - **`internal/version`** is now the single source of the harness version (it had drifted between the
    CHANGELOG and the MCP serverInfo twice; a tool that stamps a wrong version into a provenance block makes
    its artifacts untraceable).
  Docs: `docs/reproduce-loop.md`.

- **`framegen` — from-scratch full-frame reproduction generator** (v1.112.0). Reads a target ROM and emits
  a NEW, self-contained DASM source that reproduces its static visible frame **pixel-exactly** — including
  the players. It renders the target, reads which TIA object drew each pixel per visible scanline
  (`emu.DecomposeRow`), re-encodes the playfield into left/right PF register bytes and the two players into
  GRP0/GRP1 bytes, reads colours/NUSIZ/positions, and writes a data-driven per-scanline PF(L/R)+GRP0/GRP1
  replay kernel. Then it **self-calibrates** three things by assembling + rendering its own output in a loop:
  the two sprite X inputs (`SetXPos` landing offset is kernel-specific), the VBLANK line count (clone's
  visible top matches the target's), and a residual content vertical shift (±lines, chosen by element-match).
  Validated: on Outlaw it produced `clone/outlaw_clone.asm` that `vismatch` reports **pixel-exact (band diffs:
  none)** across all 214 visible scanlines — gunmen (2×-wide, P1 reflected), asymmetric cactus, score, bars,
  borders — with the target's exact TIA colours. `go run ./cmd/framegen -rom Outlaw.bin -reset -out clone.asm`.
- **`vismatch` + `behavmatch` — the automated reproduction-diff loop** (v1.111.0). Two CLI tools that
  close the "reproduce a commercial screen/mechanic pixel- and behaviour-exact" loop so a builder never
  again sparse-samples, mis-measures a band boundary by 1-2px, and iterates by hand. Both diff a TARGET
  ROM against your build.
  - **`cmd/vismatch` (`internal/vismatch`)** — PALETTE-INDEPENDENT visual diff. Renders both frames, reads
    WHICH TIA object drew each pixel (`emu.DecomposeRow` → BG/PF/P0/P1/M0/M1/BL) on every visible scanline,
    and reports every element-level difference plus a per-element **band diff** naming the exact scanline
    range and lit clock-spans where shapes disagree (e.g. `PF 162-165 | target 80-83 | mine 72-83` — a
    fat-by-4px playfield bar, pinpointed in one pass). `-diff` writes an object-attribution overlay PNG
    (green=match / red=target-only / blue=mine-only). `-genpf` **auto-generates the correct playfield tables
    from the target**: measures the cactus/PF bands and emits paste-ready `CACTOP/CACBOT` + `CacLTbl/CacRTbl`
    `ds` runs (validated: reproduced Outlaw's hand-derived cactus tables exactly). Palette independence is
    the point — two ROMs use different palettes, so object attribution, not RGB, is the ground truth.
  - **`cmd/behavmatch` (`internal/behavmatch`)** — behavioural diff. Drives both ROMs through identical
    scripted input scenarios (`internal/behavmatch/scenarios.go`: 4-way walk speed/clamps, fire→freeze
    coupling), records every object's per-frame trajectory (`emu.ObjectYExtent`/`Markers`/`PeekRAM`), and
    reports where a MECHANIC diverges as numbers — separating speed/travel-span (mechanic) from absolute
    rest position (calibration), plus a "no-Getaway" frozen-while-bullet coupling check. On Outlaw it
    confirmed horizontal (0.5px/f) and vertical (4px/4f) speeds match and surfaced two real build bugs
    (left-walk range too small; a fire-while-right bullet-trajectory divergence). **`-target-warmup N` /
    `-mine-warmup N`** (RL-5, field-driven): run N no-input frames before the scenario to skip a title
    screen that auto-advances to gameplay — without it a title-advance game (Pizza Boy) is measured on its
    title, not gameplay. Field evaluation of the whole loop + a 7-item enhancement backlog (RL-*) live in
    `docs/capability-gap-audit.md`.
  Both are thin layers over existing `emu` primitives (`New`/`LoadROM`/`RunFrames`/`SetInput`/`SetPanel`/
  `Snapshot`/`ReadRow`/`DecomposeRow`/`ObjectYExtent`/`Markers`/`PeekRAM`) + `build.Assemble` (accept `.asm`,
  auto-build). Docs: `docs/reproduce-loop.md`.
- **`decompose_row` — per-pixel TIA-object attribution of a scanline** (v1.110.0, AT-5). The attribution
  sibling of `read_row` (colours) and `beamtrace` (register writes): decomposes one visible scanline into
  run-length runs `{clock,len,element}` where element ∈ `{BG,PF,P0,P1,M0,M1,BL}` — answers "is THIS part of
  the picture the playfield, a player, a missile, or the ball?". The decisive tool for reverse-engineering how
  a running commercial ROM composes its screen (which TIA object draws which visual element, per line).
  Demand-driven while decoding Outlaw's asymmetric cactus: `read_row` showed `clk72-75 + clk80-83` lit but not
  that BOTH are Playfield (repeat-mode mid-line PF rewrites), and that the sprite/missile budget is spent
  elsewhere (gunmen at the sides, disjoint from the centre cactus). Implementation: Gopher2600's
  `reflection`/`video.Element` already computes per-pixel attribution but was unplumbed — `emu.EnableElementCapture`
  drives a per-color-clock callback (`VCS.Step(elemCB)`) recording `TIA.Video.LastElement` into `elemBuf`
  indexed by `signal.Index` (full 228×scanline space; visible clock x → `scanline*228+68+x`, same mapping as
  `ReadRow`); `emu.DecomposeRow` RLE-encodes it. Capture is on by default (observation-only — never changes
  colours/cycles; overhead is one array write per color clock). New: `emu.ElemRun`, `emu.DecomposeRow`,
  `emu.EnableElementCapture` + the `decompose_row` MCP tool. Same absolute-scanline coordinate as `read_row`.
- **`spritey` — numeric vertical (Y) position of a TIA object** (v1.109.0). Reports an object's drawn
  scanline extent (`y_top`/`y_bot`/`height`, grid-y) + X, found by matching the object's OWN colour at its
  X column — filling the gap `read_tia` (X only) and `read_motion` (rendered-top, which latches onto the
  playfield border for a 1-2px missile) leave. `frames=1` reports the current frame; `frames>1` advances and
  returns the per-frame Y trajectory — **tracing a bullet's ricochet as numbers** (`y_top` rises then falls at
  each top/bottom bounce). Surfaced demand-driven while observing Outlaw's signature ricochet: `read_motion`
  reported the bullet's Y as a constant 65 (the border) vs the true ~85. New: `emu.ObjectColorRGBA` (palette
  map via `capture.colorRGBA`→`Spec.GetColor`) + `emu.ObjectYExtent` (colour-matched column scan) + the
  `spritey` MCP tool + `TestObjectYExtentTracksBall` (a glide must descend, non-vacuous). Caveat: colour-match
  widens the extent when a same-colour object overlaps within ~8px (a just-fired missile over its player)
  until they separate. read_motion untouched (no regression). `go test ./...` green; MCP smoke on Outlaw green.
- **`docs/integration-density-playbook.md` — the composition/integration skill, distilled** (v1.108.0). The
  design-time reference for the *fit* problem (2 KB ROM · 128 B RAM · 76 cy/line · 262 lines interlocking).
  Distilled from a broad cross-domain research pass (demoscene/size-coding · WCET/embedded real-time ·
  deliberate-practice science · software product-line engineering · systemic game design) and **adversarially
  filtered against the real 2600 budget**. Contents: 8 rated transferable principles (adopted/adapted), an
  explicit **kill list** (bytebeat/PCM, heavy runtime synthesis, generic packers, compile-time `#ifdef`
  variants, "≤76 is enough", GC/heaps — all rejected with the reason), a measurable **Density Scorecard**
  (functionality-per-byte · WCET-slack/line · RAM-byte duty · feature-count-per-K · kernel byte-density ·
  table-leverage · dead-weight), and a 6-rung **deliberate-practice ladder** with per-rung scorecard gates.
  Wired into `docs/authoring-protocol.md` step 1 (Retrieve); provenance in §F. `check_wiring` / `check_provenance`
  green. No binary/behavior change (docs + one authoring-protocol reference only). First distilled from the
  interrupted deep-research run's 71 cached results, then **reconciled against the completed run** (25 claims
  3-vote tested → 20 confirmed / 0 refuted / 5 infra-unverified): added the master-move framing, concrete
  anchors (Pitfall seed 0xC4 / ~50 B; Combat 27 / ~28 B), two scorecard axes (generation-ratio, data-share),
  an anti-gaming caveat, and **§G** (verify-on-harness list + the 3 open research questions).
  **Rung-1 self-verification done** (§G): the 3-color-clocks-per-CPU-cycle coupling (`trace_clocks`:
  Δclock = exactly 3 × cycles across a 2/3/4/5/6-cy mix; `spritepos` x=80 exact) and the BIT-absolute
  skip-next idiom ($2C, 3 B / 4 cy, A/X/Y preserved) both confirmed on-harness — the playbook's §1/§6
  now rest on measured ground, not citations.
- **PONG-C3 / VV-2b — per-line WORST cycle count + blank-region ∀ accounting** (v1.106.0). `prove_line_budget`
  used to SKIP every VSYNC/VBLANK/overscan ("blank") region — `analyzeRegion` returned `Worst=0` for them and
  `Prove` `continue`d — so its `certified`/`max_worst` covered only visible lines, and a blank WSYNC-region
  that overruns 76cy (which adds a scanline = frame-line drift / "screen dip" / roll) was invisible to the ∀
  proof, delegated entirely to the runtime ∃ `ntsc_frame_lines`/`max_line_budget`. Surfaced while auditing the
  sandbox Combat clone: its ÷15 coarse-positioner (worst **73cy**, hand-verified) and its overscan-AI lines
  (up to **179cy**) were bounded-or-unbounded internally but reported as `Worst=0`. Now:
  - **① blank regions are computed and reported** — new `Report` fields `blank_lines` / `blank_max_worst` /
    `blank_over` (worst > budget×@lines = roll risk) / `blank_unbounded`. The existing `certified` / `max_worst`
    stay **visible-only** (backward-compatible: no existing scenario/litmus verdict changes).
  - **② `; @amax N` annotation** (sibling of `@lines`) — declares the proven upper bound of a divide-loop
    accumulator, so a ÷N coarse-positioner whose input is a RAM byte (abstract range Top → previously
    "loop bound unknown") can be bounded. `determineBound` uses it when the abstract range is Top.
  - **③ `roll_free`** — the ∀ roll-freedom verdict: EVERY region (blank AND visible) is bounded AND within its
    budget×@lines span. Stricter than `certified`; a blank overrun or an un-`@amax`'d divide loop makes it
    false, honestly (vs the old silent `Worst=0`). Litmus: `cb_blank_amax` (annotated → `roll_free`) /
    `cb_blank_noamax` (identical, unannotated → the blank divide loop is honestly `blank_unbounded`), test
    `TestProveBlankRegionAmax`. Complements the runtime side `emu.ProfileLineWorst` (∃ measured per-line worst,
    blank lines included) + the static `Report.Lines` complete per-region table. **Requires MCP reconnect** for
    the new report fields to surface through the `prove_line_budget` tool.
- **`read_audio_trace`** (v1.104.0) — trace the TIA audio registers (AUDC control / AUDF freq / AUDV volume)
  for both channels over N frames, returning the per-frame `control[]/freq[]/volume[]` time-series. The audio
  analog of `read_motion`: captures a whole sound envelope (a fire/explosion attack-decay, an engine pitch
  change) in one call instead of stepping frame-by-frame with `read_audio` by hand. ADVANCES the emulator N
  frames, so trigger the sound first. Motivated by the sandbox Combat clean-room sound pass, where capturing
  each of engine/fire/explosion took ~30 manual step+read_audio calls. `cmd/harness` handler +
  `AudioTraceOut`; smoke-tested (`initialize OK 1.104.0`). **Requires MCP reconnect** to become callable.
- **State snapshots + RAM-semantics probe** (v1.107.0) — three new MCP tools, `save_state` / `restore_state` /
  `probe_ram_semantics`, plus a `-snapshot` mode for `cmd/guidedfuzz`. Motivated by a study of
  [kisonecat/deep-atari](https://github.com/kisonecat/deep-atari) (a GAN that predicts the 2600 screen from RAM);
  the GAN itself was rejected, but two of its ingredients were worth taking.
  - **`save_state`/`restore_state`** (`internal/emu/state.go`) wrap Gopher2600's `hardware.VCS.Snapshot()` +
    `rewind.Plumb()` so the harness can branch-search: try N inputs or N RAM values from the SAME frame instead
    of replaying from `load_rom`. Slots are named and reusable; a snapshot costs ~3.9 KB (measured, 200 kept).
    ⚠️ **`television.Plumb()` does not touch the PixelRenderers** — `capture.Reset()` is never called on restore,
    so the framebuffer keeps the picture drawn on the *diverged* path (measured: hash matches the diverged frame,
    not the saved one). `State` therefore carries the framebuffer + crop rect + the CPU-cycle counters, and
    `TestSaveRestoreRoundTrip` fails if that copy is removed. Not covered, by design: video/audio digests,
    coverage and audio capture are append-only recorders and do not rewind.
  - **`probe_ram_semantics`** (`internal/emu/ramprobe.go`) answers "what is $XX?" for a ROM with no source:
    save → poke $XX=V → run `frames` → diff the frame against the un-poked baseline → restore, for every address
    and probe value; classifies from how the changed-region centroid travels (`x_position`/`y_position`/
    `appearance`/`none`). Non-destructive. Graded against litmus ground truth both ways (litmus_pos `$80`=DELAY
    → x_position; motion_glide `$80`=posY → y_position; unused addresses → `none`, no false positives) and
    audited against the published Combat disassembly (full sweep in 3.1s; `$A4`/`$A5` = TankY0/TankY1,
    `$DC` = KLskip, `$88`/`$A3` = GameOn/GAMVAR repaint everything, `$BE`-`$CC` even = the HIRES sprite buffer).
    Default `frames` is **3, not 1**, because Fishing Derby's score bytes reach the screen only after a
    BCD→digit-graphics conversion and were invisible at `frames=1` (measured 0 → 1 → 2 detected at frames 1/2/3).
    Known blind spot: a byte the kernel recomputes every frame before use reads as `none`.
  - **`guidedfuzz -snapshot`** reuses one emulator and restores a post-warmup snapshot per evaluation instead of
    reloading the ROM. Identical coverage signatures (`TestSnapshotEvaluatorMatchesReload`), and at `warmup=200`
    **~100x faster** (625.9ms → 6.2ms per evaluation; CLI end-to-end 23.80s → 0.86s at warmup=120, both
    markers=36). New `Coverage.Reset()` cuts each evaluation since restore does not rewind coverage.
  - Reference material: `reference/ale-ram-maps/` (umbrella, local-only) — RAM addresses for **104 commercial
    games** distilled from the Arcade Learning Environment's `RomSettings` sources, with provenance and the
    `(offset & 0x7F) + 0x80` address convention verified against ALE's `RomUtils.cpp`. Used as an independent
    answer key for `probe_ram_semantics`, not as a design input.
  **Requires MCP reconnect** to become callable.

### Fixed
- **Collision and stack sampling happened at frame boundaries, where both are already gone** (v1.113.0).
  Games clear `CXxx` every frame and SP is back at `$FF`, so boundary sampling could neither prove a game
  uses collisions nor tell which RAM the stack had trampled. Measured against a real cartridge, the SP
  low-water mark came out `$FF` on every single frame — excluding exactly zero bytes and silently turning
  the RAM gate's stack mask into a no-op. Watching inside the frame then invalidated the rule as well: the
  target's SP sweeps `$FF` down to `$1C` every frame (a `TXS` aiming at TIA register space), under which
  "exclude everything at or above the lowest SP" excludes all 128 bytes and the gate passes unconditionally
  while reporting green. A pointer descending past an address is not a write to it; stack exclusion needs
  write attribution and is not attempted until that exists.

### Decisions
- **The arity probe reports memorisation as memorisation.** A free-running frame counter takes a fresh
  value every frame, so keying on it "explains" every other byte perfectly — the first version of the probe
  reported that all of RAM had arity 1. Frame-counter-like bytes are now identified and tried last, and any
  resolution in which every key was seen exactly once is flagged `MEMORISING`: consistent with the data,
  and evidence of nothing about states the scenarios never visited. A model that only reproduces its own
  recording is the failure this whole system exists to avoid, so the tool has to be able to say so.

### Docs
- **Combat deep-read (round 2) absorbed** — a 5-lens pass (game-design/6502-craft/audio/anti-patterns/clone-novelties)
  over the original Wagner Combat.asm surfacing learnings the efficiency comparison structurally could not see,
  concentrated in **audio** and **design-intent**. `design-principles.md` gains a "Combat deep-read: design-intent,
  audio model & AI-nav primitives" section (difficulty=self-handicap-on-the-winner; curate+reskin content strategy;
  invisible-stealth self-betrayal; consequence-beat+board-reset; diegetic end-game UI; control-overload;
  **sound-priority = last-writer-wins on a 1-object-per-channel bus**; vector-slot-data 2K→4K trap; + the clone's
  emulator-verified **AI-nav primitives** (octant-seek overflow-guard, mod-16 shortest-arc turn, map-free
  stall/wall-slide/aim-gate/scatter-decoy) flagged PONG-capstone material). Two new **technique reference docs**:
  `techniques/audio-envelope-idioms.md` (counter-IS-the-audio-register, self-clearing SFX counter, per-player detune,
  gear-shift pitch curve) and `techniques/kernel-micro-idioms.md` (HMP low-nibble 2nd axis, `$FF`/`$00` AND-mask blank,
  −4 pointer bias, PF mirror via counter-EOR, compare-via-EOR A=0) — reference-only (reimplement+CI = TODO), wired into
  `techniques/README.md` (#30/#31). `casebook.md` gains two anti-pattern cases (unclamped-input-index UB; don't-cargo-cult
  a master's cruft). `capability-gap-audit.md` registers **CMB-4** (CMB-AUDIO temporal-audio assertion), **CMB-5** (MD5
  gap-fill $FF-PROM model), **CMB-6** (assembler warn on data over $FFFA-$FFFF), **CMB-7** (no-SMC calibration boundary).
  `check_wiring`+`check_provenance` green. Docs-only.
- **Combat (1977 Wagner 2K) structure/efficiency learnings absorbed** from the sandbox clean-room comparison
  study (`studies/combat/comparison-structure-vs-original.ja.md`, `diff-gaps.ja.md`), skipping what was already
  in the harness (stack-trick/two-line-kernel/score-kernel/div-15). `casebook.md` gains a **Combat** section
  (PF-only dual score via `CTRLPF #$02` + recycled PF1; multi-frame `MxPFcount` wall-normal bounce solver;
  `StirTimer` hit-reaction state machine; `VARMAP` 27-variant bit-packed selector + DDR input-gating) + an
  index row. `design-principles.md` gains 8 **integration-under-budget** rules (one `,X`-indexed path for all
  objects vs per-object inlining; time-sliced momentum; rotation-shape RAM precompute; interleaved single HIRES
  buffer; multi-duty phase-locked byte fan-out; `INTIM`/`TIM64T` VBLANK load-leveling vs fixed-count+pad;
  one-loop-four-ranges `ClearMem`; self-audit-your-own-cargo-cult). `capability-gap-audit.md` registers three
  candidates: **CMB-1** structural-efficiency lint (inline→`,X` in blanked time, ~250-400 B recoverable),
  **CMB-2** `INTIM` fixed-picture-start advisor, **CMB-3** collision-face/wall-normal estimation aid. `check_wiring`
  + `check_provenance` green. Clean-room recorded (disassembly read post-build; casebook contract). Docs-only.
- Knowledge captured from the sandbox PONG feel-pass (pf2-06, 2026-07-03): technique **#29 sub-pixel velocity
  (DDA error accumulator)** — fractional speed while the position stays a 1-byte integer
  (`docs/techniques/subpixel-velocity.md`); two `known-traps` rows (`bpl`/`bmi` clamp on a coordinate that
  legitimately exceeds 127 → use wrap-magnitude not bit7; immediate `ld_` clobbering N/Z between a flag-set
  and its branch → branch first or go branchless); backlog **PONG-C3** (per-line WORST cycle count, not just
  pass/fail — the highest-leverage tool gap the feel-pass surfaced). Docs-only; no tool/behavior change.
- More knowledge from the PONG serve-refinement + AI-variants work (2026-07-04/06): a `known-traps` row for
  **`cmp` clobbering Z between a load and a test-for-zero branch** (sibling of the immediate-`ld` clobber;
  hit as a real serve-clamp bug where `0→1` silently failed); a **range-dependent-threshold** note on the
  bit7-clamp trap (a `BallRow + 8·DY` lead reaches ~202 so the wrap threshold must be `#220`, not `#200`);
  and reinforcing evidence on backlog **PONG-C3** (building 3 swappable AI kernels hit the same
  guess-and-assert budget loop — design estimate 61cy vs real ~78cy — confirming it recurs on every
  budget-tight kernel). Docs-only; no tool/behavior change.
- Knowledge captured from the PONG AI-variants + objective-benchmark work (2026-07-06..11): two 6502 idioms
  in `design-principles.md` — sign-preserving ×2^n via repeated `asl` on two's-complement values (with the
  widened-range clamp caveat: bit7 stops being usable as a sign, so clamp by value range), and packed-BCD
  bytes comparing correctly with a plain `cmp` (binary order = decimal order; but binary *differences*
  overstate decimal ones across a digit boundary — bucket/saturate before using them as magnitudes). Plus a
  new in-house PONG section in `casebook.md`: the four classic paddle-AI paradigms with designed-in
  beatability, imperfection tuning via error/delay rather than speed, the exclusive-path shared-tail-skip
  (`jmp OverEnt`) budget-carving pattern, and the measured **non-transitivity of AI strength** (a
  single-baseline ranking refuted by a round-robin: the baseline's "strong" tracker is actually the weakest
  head-to-head). Docs-only; no tool/behavior change.
- Backlog **PONG-C4** registered (`capability-gap-audit.md`): gameplay-behavior verification — a headless
  match harness (declared actor interface + parameterized scripted opponent + match rules → per-pairing
  scores and an N×N tournament matrix) plus behavioral-invariant fuzz (speed bounds, score monotonicity,
  serve fairness). Generalizes the hand-built C1 bench/round-robin ROMs; the C1-measured non-transitivity
  is baked in as a design constraint (tournament matrix, not a scalar rank; opponent model = explicit
  parameter). Registration only — implementation is a separate approval.

## [1.105.0] - 2026-07-21

> **★backfilled 2026-07-30 from the commit diff.** This version was tagged (`aab7ab7`) and shipped with
> **no CHANGELOG entry at all** — the gap was found by counting 170 tags against 174 released sections.
> Everything below is read off the diff, which is fact. **The rationale is not recorded**: nobody wrote
> down why it was added at that moment, and inventing one after the fact would put a sentence in this
> file that nothing supports. Left blank deliberately.

### Added
- **`read_ram_trace` MCP tool** (`cmd/harness/main.go`, +61/−1, one file). Traces **1–16 RAM addresses**
  (`$80`–`$FF`) over **1–4000 frames** (default 60) and returns `traces[i][f]` — the per-frame value of
  each address, indexed from the call. Out-of-range addresses, an empty list, more than 16 addresses and
  a frame count outside 1–4000 are each rejected by name.
- It **ADVANCES the emulator** by `frames`, and input set with `set_input` persists across the trace —
  both stated in the tool's own description, so a caller is not surprised by the side effect.
- Purpose per that description: collapse a manual `step_frame` + `peek` loop into one call, to measure
  **as numbers** how a byte evolves — a tank's X/Y, an AI mode or timer, a score, frames-to-escape a
  region, a decay curve, a stuck oscillation.

## [1.103.0] - 2026-07-03

Interactive rollout of the two PONG-campaign capabilities (backlog PONG-C1/C2), live-proven on the real
PONG ROM: C1 rediscovers a byte-faithful replica of the historical 77cy 3-edge-coincidence bug
(fail at PFp1, 152cy) and passes the fixed kernel across 139 consistent alignments (offsets-coupled sweep);
C2 runs the whole lightweight-table budget ritual in one call with the original ROM verified byte-identical
afterwards. The rollout itself exposed and fixed a latent flaw in the budget guard (Fixed below). Also
includes the accumulated PONG-dogfooding items below (framesim normalization etc.).

### Removed
- **AtariAge fetch tooling relocated out of the repo.** `scripts/aa_fetch.py`, `scripts/aa_index.py`, and
  `scripts/aa_manifest.py` (the forum thread/index crawler — Wayback-first, with an optional cookie-based
  direct fallback) were moved to local-only research tooling outside the published repo. Rationale: it is
  ingestion scaffolding, not part of the verification harness (the deliverable), and a ToS-adjacent scraper
  has no reason to ship in the public engine. The distilled knowledge it produced (`docs/mining-digest.md`
  and the technique/casebook corpus) stays — that is the value; the scraper is not. `gen_mining_digest.py`
  remains (it only distills an existing local `MINED.csv`; it never fetched anything).

### Fixed
- **`RunUntilBudget` (assert_line_budget core) silently ate poked-state frames in its warmup — found by the
  PONG-C1 rollout (2026-07-03).** The unconditional 2-frame stabilization run consumed the very frame a caller
  had just poked into a worst-case alignment, so `poke → assert_line_budget` could NEVER observe a
  single-frame overrun — the historical PONG 77cy coincidence bug was un-reproducible by direct poke for
  exactly this reason (only persistent-state trajectories tripped it, by luck). The warmup now runs only on a
  fresh boot (Frame<2); mid-session calls start monitoring immediately. Ground truth: a byte-faithful 77cy
  replica (NetTbl load → 3×NOP, +2cy) with poked 3-edge alignment now reports over=true/152cy; the fixed
  kernel reports over=false. Locked by `TestBudgetGuardNoWarmupWhenRunning` (frames consumed must equal
  max_frames exactly when already running).

### Added
- **`assert_line_budget` temporary ROM patch (`patch`/`pokes` params) — PONG-C2 (v1.103.0).** During the PONG
  campaign every budget run required hand-editing the positioning table to lightweight values, assembling,
  asserting, restoring, re-assembling (~15×; one forgotten restore = shipping a wrong ROM). The tool now takes
  `patch: [{symbol|addr, bytes}]` — applied to a COPY of the loaded ROM (symbol resolved via the last
  `assemble_and_load` listing, new `srcmap.Symbol()`), fresh-booted for the measurement, and the original ROM
  is ALWAYS reloaded afterwards (deferred restore = the forget-to-restore failure mode is structurally gone).
  `pokes: [{addr,value}]` seeds RAM after the patched boot for trajectory reproduction.
- **`assert_edge_coincidence` — worst-path fuzz for edge-compare kernels — PONG-C1 (v1.103.0).** The PONG
  PlayF kernel hid a 77-cycle line that only fires when ALL edge variables (ball bottom + paddle top + paddle
  bottom) land on the SAME Y — free-run testing missed it for hundreds of frames (known-traps "N-edge
  coincidence", found 2026-07-02). The tool pokes every listed zero-page edge variable to one Y, runs
  `frames_per_y` frames under budget-guard semantics, sweeps Y over a range, and reports every failing
  alignment (`fail_ys`, first at/cycles). Optional `patch` (auto-restored) combines with a lightweight
  positioning table. Claim-level proof: rediscovers the historical 77cy bug on the pre-fix PONG binary,
  passes on the fixed one.
- **`framesim` scale-normalized comparison (`framesim.Resize` + `NormalizeSize`).** `framesim -a rom.bin -b
  screenshot.png` previously errored on a bounds mismatch (a 1× ROM render, 160×N, vs a 2× Stella screenshot,
  320×M), so a ROM could not be compared to a target screenshot. Both inputs are now downscaled (nearest-neighbor,
  per-axis min) to a common raster before SSIM/pHash, and the CLI reports the `normalized` size. Found and fixed
  during the PONG dogfooding campaign (it blocked the Phase-1 "framesim matches the target screenshot" metric).
  `TestNormalizeSizeRescales` locks it (an image vs its own 2× upscale scores ~1.0). Vertical-framing alignment
  (differing VBLANK/overscan margins) is handled separately by `-align` below.
- **`framesim` content-bbox alignment (`framesim.ContentBBox` + `NormalizeAligned` + `framesim -align`).**
  Scale-normalization alone still misaligned a ROM render against a screenshot whose lit content sits at a
  different vertical offset (the ROM's 214-row frame vs the target's 228-row frame put walls/net/scores on
  different rows), so the diff was dominated by spurious whole-row mismatches and was untrustworthy. `-align`
  first crops BOTH inputs to their lit-content bounding box (luma>128 = wall-to-wall, net-to-net) and only then
  scale-normalizes, so content is compared content-to-content regardless of margins. On the PONG campaign frame
  this took the diff from 3164→1078 mismatched px and SSIM 0.105→0.192 (the remaining diff is now REAL: score
  glyphs, net phase, paddles — not framing noise), unblocking the convergence loop. `TestNormalizeAligned` locks
  it (the same content at different positions in different-size frames aligns to ~1.0). Fix along the way: the
  bbox seed used `image.Rect(max,min)`, which sorts its args and collapsed the inverted seed back to full
  bounds (ContentBBox always returned the whole frame) — now seeded with plain ints.
- **`framesim` difference localizer (`framesim.Diff` + `framesim -diff out.png`).** SSIM gives one global score
  and the single worst 8×8 block; for the "reproduce a target screenshot" loop you need to see EVERY differing
  region. `Diff` classifies each pixel (match / A-only=red / B-only=blue) into a diff image and per-row stats,
  and the CLI prints the differing row-bands ("rows 37-59: 510 diff px" = a band the target draws that the ROM
  doesn't). Turns "compare → localize what's wrong → fix → repeat" into a measured loop. `TestDiffLocalizes`
  locks it (identical = 0 mismatch; a lit block over black = B-only localized to its rows).

- **`framesim` max-normalization (`framesim.NormalizeSizeMax` / `NormalizeAlignedUp` + `framesim -up`).**
  `NormalizeSize` downscales both inputs to the per-axis MIN; comparing a 1× ROM render to a 2× screenshot
  thus downscales the screenshot, blurring its thin features (net dash, glyph edge) so the SSIM/diff is *more
  forgiving* but fuzzy. `-up` instead rescales to the per-axis MAX — upscaling the ROM (nearest-neighbor, stays
  sharp) and leaving the screenshot native — for a sharp-vs-sharp comparison at the screenshot's resolution.
  Found during the PONG campaign while chasing the static-match residual: `-up` is the STRICTER, more honest
  metric (it doesn't blur real ~1px differences away), confirming the remaining residual is genuine fine detail
  (net-edge/phase, fenceposts), not a downscale artifact. `TestNormalizeSizeMaxUpscales` locks it (an image vs
  its 2× upscale max-normalizes to the 2× size and scores ~1.0).
- **`framesim` per-element ruler (`framesim.ContentRowSpans` + `Span`/`SpansEqual` + `framesim -spans`), and an
  `-align` height-mismatch warning.** The global SSIM/diff buries 1-row/1-px element errors (a score-bottom row,
  one net dash, a partial wall edge) — the PONG campaign proved it: the static frame read "done" at the global
  level while three elements (score bottom, ball squareness, paddle height) were each off by a row, and the fix
  hunt was slow because there was no standing tool to measure each element's exact extent. `-spans` prints, for
  every content-aligned row, the lit runs in CLOCK coords (clk = x / scale, so a 1× ROM and a 2× screenshot read
  on the same 0..159 axis) for A and B side-by-side, marking rows that differ — measured in each frame's OWN
  content crop at native resolution, so it keeps the screenshot's precision and sidesteps the resize that makes
  `-align` ±1-row sensitive. It is the exact ruler that complements the tolerant SSIM/diff: on the campaign frame
  it pinpointed exactly two differing rows (a net dash and the partial wall edge), including a net-dash gap below
  the diff's row-band threshold. The `-align` path now also warns when the two content heights differ (the
  resized diff then carries ~Δpx of edge noise → use `-spans`). `TestContentRowSpans` locks the clock-coord
  mapping at 1× and 2×. Replaces the ad-hoc per-row measurement script hand-built mid-campaign.

### Fixed
- **CI: run `go test -p 1 ./...` (serial package tests).** Several packages assemble/read the SAME shared ROM
  `.bin` fixtures (`roms/litmus`, `roms/techniques`) during their tests; under `go test ./...` (parallel test
  binaries) one process can truncate a `.bin` mid-assemble while another loads it → a flaky panic (e.g.
  `TestCoverageThroughRun` / `TestTrajdiffSelfTest`, "index out of range … length 0"). This was a long-standing
  test-isolation flake (also red on old commits like VV-9 @105932d), surfaced more often after the AT-* sprint
  added more assembling tests. Serial package tests fix it deterministically; the engine is single-emu in
  production, so this is a test-only concern. (No server-code change → server version unchanged.)

## [1.102.0] - 2026-06-18

### Added
- **MCP exposure of the interactive authoring aids (AT-5), batched into one reconnect.** Three new `cmd/harness`
  tools so the timeline/solver are usable in the authoring loop, not just the CLI:
  - **`beamtrace`** — write→visible-pixel timeline for the loaded ROM (per scanline, each TIA write's beam clock,
    register name/kind, value, and governed visible span). Advances the emulator.
  - **`beam_race`** — advisory beam-race map (per pixel-data write, object X + in-time/late). Factual, no verdict;
    paired with the existing scenario `checks.no_beam_race`. Advances the emulator.
  - **`spritepos`** — forward sprite-position solver (target X → routine input + decomposition + snippet +
    emulator-verified achieved X). Self-contained: builds its own calibration kernel, does not disturb the loaded ROM.
- `scripts/mcp_smoke.py` extended to call all three over stdio (smoke now covers beamtrace/beam_race/spritepos).

### Notes
- The static linter (AT-1, `cmd/timinglint`) stays CLI/CI-only — no MCP tool, by design (proactive source check,
  no live emulator state needed). **This release adds MCP tool schema → requires one `bin/harness` rebuild +
  client reconnect** (smoke-tested green at v1.102.0 first). Concludes the authoring-tools sprint (AT-1..AT-5).

## [1.101.0] - 2026-06-18

### Added
- **`cmd/spritepos` + `internal/spritepos` — forward sprite-position solver (authoring aid, AT-4).** Given a
  target X (0..159) it returns the routine input, the div-15-coarse / HMOVE-fine decomposition, a paste-able
  `SetXPos` snippet, and — the part that makes it trustworthy — the position the hardware **actually reaches**
  (HmovedPixel), measured by running the kernel. Built clean-room on the verified
  `roms/techniques/shared_setxpos.asm` idiom (div-15 coarse via RESPx strobe timing + remainder→HMOVE nibble by
  the `eor #7; asl×4` trick). Per CLAUDE.md the X(N) offset is kernel-specific, so `Solve` never trusts the
  arithmetic — it measures the offset against the emulator, inverts it, and re-runs to confirm. `-all` solves
  every X for an object; `-json` for tooling.
- **Self-tests:** `TestDecompose` (pure coarse/fine arithmetic), `TestAchieveSweepLog` (records X(A)),
  `TestSolveHitsTargets` (P0/P1/M0/BL × 7 targets land EXACTLY, emulator-verified), `TestAchieveDiscriminates`
  (a deliberately-wrong input must miss — the guarantee isn't vacuous).

### Notes
- **Measured: X(A) == A exactly across the whole range** for this calibrated routine (slope 1, offset 0), and
  `spritepos -object BL -all` lands **160/160 targets exactly**. Found + fixed a bug in the pure `Decompose`
  helper (loop-exit used bit-7 instead of the 6502 carry, so X≥128 broke immediately) — caught because the
  emulator ground truth disagreed with the math; the verified positions were never affected. Pure Go, CLI only.

## [1.100.0] - 2026-06-18

### Added
- **Beam-race / too-late-write detection (authoring aid, AT-3) — a SOUND dual, not a blanket detector.**
  `internal/beamrace` + scenario `checks.no_beam_race` + `cmd/beamtrace -race`, plus a thin `emu.ObjectX`
  accessor (player/missile/ball HmovedPixel). A write to an object's pixel-data register
  (GRP0/GRP1/ENAM0/ENAM1/ENABL) at beam clock C while the object sits at X reaches the beam in time iff C ≤ X;
  otherwise that line draws the previous value (a one-line lag).
  - **`cmd/beamtrace -race` — advisory report (automatic, factual, NO verdict):** per object, every pixel-data
    write with clock vs object X marked in-time / LATE. Cannot false-positive because it asserts nothing.
  - **`checks.no_beam_race` — verdict the author OPTS INTO:** `{object, line_from, line_to}` declares "object O
    must be updated before the beam on these scanlines"; the check fails on any late write. Sound because the
    intent is supplied, not guessed. Generalises the hardware-fixed `no_hmove_hazard` gate.
- **Litmus + self-tests:** `roms/litmus/beamrace_clean.asm` (P0 updated in HBLANK → in-time) and
  `beamrace_late.asm` (P0 graphics written deep in the visible line → one-line lag). `TestCheckEvalPure`,
  `TestBeamraceCleanPasses`, `TestBeamraceLateFails` (beamrace) + `TestBeamRaceScenario` (both directions
  through the scenario engine) + `roms/litmus/scenarios/beamrace_clean.json` in the regression set.

### Notes
- **Why no fully-automatic verdict (measured/reasoned, on the record):** whether a late write is a *bug* depends
  on author intent — the same late `sta GRP0` is correct when it pre-loads the NEXT line and wrong when meant for
  THIS line. Validated on the real `multicolor48` kernel: P0 at X=87, the 48px technique's right-side GRP0
  rewrites land at clk +139/+157 = "LATE" — **correct facts, not bugs**. An automatic verdict would
  false-positive there, violating the zero-false-positive bar; hence the advisory (no verdict) + opt-in check
  (intent supplied). A heuristic auto-detector is **deferred, not closed** (see audit AT-3) per the user's
  request to keep it on the books.
- Pre-existing flake noted (not from this change): `internal/trajdiff` `TestTrajdiffSelfTest` panics rarely under
  the fully-parallel `go test ./...` (a latent gopher2600 lazy-init data race); passes deterministically alone
  and under `go test -p 1`. Tracked for a later look.

## [1.99.0] - 2026-06-18

### Added
- **`cmd/beamtrace` + `internal/beamtrace` — write→visible-pixel timeline (authoring aid, AT-2).** Runs a ROM
  instruction-by-instruction and tabulates, per scanline, every TIA write with the beam clock it lands at and
  the visible-pixel span it governs — answering "where on the line does this `sta GRP0` actually paint?". The
  causal map the runtime tools (`trace_clocks`/`read_row`) only show piecemeal. States only what is sound: a
  write at clock C can affect a pixel only if rendered at clock ≥ C, and a later write to the **same** register
  supersedes it — so the governed span is `[C, next-same-reg-write)`. Register name+kind table
  (color/graphics/position/motion/control/audio/strobe); pure strobes (WSYNC/RESPx/HMOVE/HMCLR/CXCLR/RSYNC)
  report no value. New thin `emu.LastTIAWrite` accessor (same detection as `WatchHMOVEHazard`). Pure Go, CLI only.
- **Self-tests:** `TestTimelineSpans` (pure span logic on synthetic writes: ordering, same-reg supersede, HBLANK
  clamp, empty span when superseded in HBLANK) and `TestTraceGRP0Marker` (fixture `roms/litmus/beamtrace_grp0.asm`
  writes GRP0=$A5 once per frame → surfaced with right value/kind, localized to its scanline, deterministic).

### Notes
- Validated against the real `multicolor48` kernel: the timeline reproduces the staggered GRP0/GRP1 rewrites
  with correctly interleaved spans, and a write superseded during HBLANK correctly shows an empty `[0,0)` span.
- Interpreting whether a write is *too late* (the effect window is fully passed) is the next tool's job (AT-3
  beam-race detector); beamtrace only lays out the facts.

## [1.98.0] - 2026-06-18

### Added
- **`cmd/timinglint` + `cyclebound.Lint` — static TIA-timing linter (authoring aid, T1 of the authoring-tools
  sprint).** Reads a kernel and warns *before* you run it about high-confidence horizontal-motion timing
  pitfalls, complementing the runtime checks (`assert_line_budget`, VV-10 HMOVE hazard) by being proactive.
  Three rules, each validated both directions:
  - **`hmove-without-hmxx`** — HMOVE is strobed but no HMP0/HMP1/HMM0/HMM1/HMBL is ever written (the fine
    motion is always 0).
  - **`hmxx-without-hmove`** — a **provably non-zero** motion is staged but HMOVE is never strobed (the motion
    is never applied). Value-aware via the prover's abstract interpreter: a defensive `lda #0; sta HMPx` clear
    (proven 0) or any unknown/computed value never warns — only motion proven non-zero does.
  - **`hmove-hazard`** — an HMxx/HMCLR write starts <24 CPU cycles after an HMOVE on a straight-line path
    (motion undefined, Stella PG). The standard `sta HMOVE; ds 12,$EA; sta HMCLR` idiom (HMCLR at exactly 24cy)
    is correctly treated as safe.
- **Litmus fixtures + self-tests:** `roms/litmus/lint_r1_hmove_nohmxx.asm` / `lint_r2_hmxx_nohmove.asm` /
  `lint_r3_hazard.asm` (each fires exactly its rule) and `lint_clean.asm` (the canonical correct idiom, silent).
  `TestLintTrapsFire` / `TestLintCleanSilent` lock both directions; `TestLintNoFalsePositivesOnTechniques`
  is the corpus guard.

### Notes
- **Quality bar met (measured): zero false positives on all 31 known-good technique kernels.** The first sweep
  surfaced 6 apparent warnings, all run down to two detector gaps and fixed (the rules themselves held):
  `storeTIA` missed **indexed** HMxx stores (`sta HMP0,x` / `sta HMM0,y` — how shared positioning code stages
  several objects), and `hmxx-without-hmove` wrongly flagged a benign zero-clear (now value-aware). Also fixed a
  latent false-negative in the hazard cycle-accounting (the gap is now measured to the *start* of the HMxx write,
  so a 22-cycle `ds 11` gap is correctly flagged while the 24-cycle idiom is not). Pure Go, CGO-free; CLI only
  (no MCP tool — no reconnect).

## [1.97.0] - 2026-06-18

### Added
- **Value-range absint enhancements for the cycle-budget prover (VV-2 "array-range" arc).** Three sound,
  composable building blocks, each litmus-locked both directions:
  - **3A — AND/ORA #imm range.** `and #m` ⇒ [0,min(A.Hi,m)], `ora #m` ⇒ [max(A.Lo,m),255] (EOR stays Top); and
    `determineBound` now reads a divide loop's entry value from the fall-through predecessor's post-state (the loop
    header is polluted to Top by the final wrapping subtraction on the back-edge). Litmus `cb_andloop.asm`.
  - **3B — zero-page RAM array-element range.** `State.ZPVal` = join of all values stored to RAM ($80–$FF), seeded
    to the recognised clear value; an indexed RAM load (`lda arr,x`) returns it (sound over-approx of any element;
    $00–$7F TIA/RIOT excluded). Litmus `cb_arrloop.asm`.
  - **3D — ROM data-table value range.** An indexed load from a ROM address returns the table's actual byte range
    over the proven index range (constant data, read from the binary). Litmus `cb_romtable.asm`.

### Notes
- **Honest measured outcome: +0 real kernels (still 14/31 certified, 0 false-positive violations).** All three are
  sound and certify their clean litmus, but real kernels need a *cascade* of further precision: their loop counters
  and array indices are **loop-carried**, so the abstract interpreter over-approximates them to Top at the loop
  header (the dec/sbc wraps on the exit edge). Recovering the in-loop counter range (narrowing it on the loop
  branch) — and tight table extents — is the recurring root limitation; it is an open-ended precision tail with
  diminishing per-kernel payoff (a kernel only flips once its *entire* chain is closed). The building blocks are in
  for when that root fix is tackled. ②Z3 / ④external-ROMs remain inapplicable to what's left.

## [1.96.0] - 2026-06-18

### Added
- **Interprocedural cycle-budget proving (VV-14 2A).** `internal/cyclebound` now FOLLOWS subroutine calls
  instead of reporting "JSR in region — unbounded". `longest()` threads a single-level return address (memo keyed
  by `(addr, ret)`): a JSR descends into the callee with the return point threaded, an RTS/RTI returns to it, and
  the callee's own WSYNC remains a region sink. Sound by construction — a nested call or an RTS with no caller in
  context sets the region UNBOUNDED rather than under-estimating. Locked by `roms/litmus/cb_jsr.asm` +
  `TestProveInterproceduralJSR` (a JSR'd subroutine certifies; a tight budget flips the region with the callee on
  the worst path = its cycles are counted).
- **Divide-by-15 / sbc-counter loop bounding (VV-14 2B).** `determineBound` now bounds the coarse-positioning
  idiom (`sec; sbc #const; bcs/bcc`) from A's proven loop-entry range: iterations ≤ floor(Amax/const)+2. The
  entry bound comes from the closest immediate `lda #imm` before the loop (the in-loop join is polluted to Top by
  the final wrapping subtraction), falling back to a non-Top tracked range. Sound: over-approximates the count;
  unknown range / non-constant subtrahend ⇒ stays unbounded. Locked by `roms/litmus/cb_divloop.asm` +
  `TestProveDivideLoopBounded`.

### Changed
- **VV-14 2C — last false-positive violations cleared.** After 2A exposed them, four stable-262 kernels showed a
  genuine multi-line region as a violation; `@lines` declares the true span (sfx_demo Vis ×3 ⇒ now CERTIFIED;
  shared_setxpos/text12/text24 positioning-setup ×2 ⇒ violation cleared, other regions stay honestly UNBOUNDED).
  Result: **no false-positive violation remains in any technique kernel**; certified 13 → 14 / 31.

### Notes
- Honest measured outcome: 2A/2B add real, self-tested prover capabilities, but raised the kernel certify count by
  only +1 — the remaining uncertified kernels are blocked by a *combination* of hard/honest issues (no-WSYNC,
  multi-call-site RTS context, nested loops, WSYNC-in-loop, and divide loops whose counter lives in untracked
  indexed RAM, plus bank-switched display). Those UNBOUNDED verdicts are the correct honest scope limit, not false
  alarms; reducing them further needs larger absint work (indexed-memory range tracking, multi-context returns) —
  diminishing returns, deferred along with ②Z3/④external-ROMs.

## [1.95.0] - 2026-06-18

### Added
- **Citable cycle-budget certificate (VV-14, ③).** New `cmd/cpucert` + `cyclebound.Certify`. Wraps the VV-2 static
  prover in a reproducible, attestable proof artifact: per-region proven worst-case + verdict, the `@lines`
  declarations the proof relies on, and full provenance — prover version, Gopher2600 pin, DASM version, and SHA-256
  of both the `.asm` and the assembled ROM. Text or `-json`; exit 1 when not certified. Self-test both directions:
  smoke certifies with a deterministic ROM-core + hashes; litmus_overrun is rejected; multicolor48's cert records
  the `@lines 2` lemma it relies on; distinct ROMs hash distinctly (tamper-evident).

### Changed
- **VV-14 ① prover precision — applied `@lines` to real kernels.** Empirically (sweeping all 30 technique kernels),
  the prover's over-warnings are **multi-line-region** false positives, not infeasible-path ones (0 kernels). Each
  affected kernel runs at a verified-stable 262 scanlines/frame, so an over-budget region (worst W>76) legitimately
  spans ⌈W/76⌉ scanlines; declaring that with `@lines` is the sound fix. Annotated 9 kernels (multicolor48, score6,
  hscroll, bitmap48, two_line_vdel, zone_multiplex, tia_pcm, bullets, rpgmap): 5 now fully certify, 4 clear their
  false-positive violation (other regions stay honestly UNBOUNDED). No prover-code change; existing cyclebound
  self-tests stay green (no-false-negative preserved).

### Notes
- **Assessed and deliberately NOT built (measured 0/low payoff on real kernels):** display-off region
  reclassification (the VSYNC→VBLANK transition has VBLANK provably-unknown — first frame can run with display on —
  so skipping would be unsound; reverted), value-range loop bounding (the unbounded loops are nested / hardware-timer
  waits / JSR-RTS subroutine timing, which a divide-by-15 bounder cannot fix), and infeasible-branch pruning (no
  real kernel needs it). The remaining UNBOUNDED verdicts are the correct honest outcome; tightening them needs
  subroutine-timing modeling — a larger future lever, kept deferred along with ②Z3/SMT and ④external silicon-TIA ROMs.
- `.gitignore`: ignore stray `/cpucert`.

## [1.94.0] - 2026-06-18

### Added
- **Frequency-domain audio comparison (VV-13).** New `internal/audiospec` + `cmd/audiospec`. The `golden_audio`
  scenario check hashes the audio register chain and an RMS envelope says "how loud over time"; neither separates
  two sounds with the same loudness contour but different pitch/timbre ("inverted twins"). audiospec adds the
  spectral modality: a pure-Go radix-2 FFT magnitude spectrum with a cosine **spectral distance**, alongside an
  **RMS-envelope distance** and a dominant-frequency readout, over the captured PCM stream (`emu.AudioSamples`).
  `cmd/audiospec` compares two ROMs' audio on a chosen channel, prints a JSON report, and exits 1 above `-max`.

### Notes
- Self-test demonstrates the axis numerically: two equal-amplitude tones at different pitch score **envelope
  distance 0.0000 vs spectral distance 0.9980** — the spectral axis out-resolves the envelope. FFT recovers a known
  tone within bin resolution; identity is zero; a real capture (sfx_demo) runs the full pipeline. Pure Go, no
  reconnect. `.gitignore`: ignore stray `/audiospec`.
- **Phase C complete** — Tier-3 VV-11/12/13 all done; VV-14 (Z3/ILP prover upgrade + external silicon-TIA ROMs)
  remains deliberately deferred per the audit until a kernel demands it.

## [1.93.0] - 2026-06-18

### Added
- **Tolerant frame compare: SSIM + perceptual hash (VV-12).** New `internal/framesim` + `cmd/framesim`. The exact
  `golden_frame` scenario check answers a boolean "identical?"; framesim answers "how wrong, and where". Windowed
  **SSIM** over 8×8 luma blocks gives a magnitude (mean, 1.0 = identical) plus locality (the worst-matching block);
  a **DCT perceptual hash** gives a shift-tolerant Hamming distance. This complements — does not replace — the
  exact golden: a 1-pixel jitter that flips the exact rendering hash still scores SSIM ~1.0, while a genuinely
  corrupted frame scores far lower (multicolor48 vs smoke ≈ 0.08). `cmd/framesim` compares two frames (each a
  rendered `.bin` or a `.png`), prints a JSON report (ssim mean/worst, worst block, pHash distance), and exits 1
  when SSIM falls below `-min` (a tolerant regression gate).

### Notes
- Self-test both directions: identical ⇒ SSIM 1.0 / pHash 0; a 1-pixel change stays >0.99 (tolerant) but < 1;
  inverted < 0.5; SSIM monotonic in damage; the worst block localises injected damage; real cross-ROM frames score
  measurably below self. Pure Go, no reconnect. `.gitignore`: ignore stray `/framesim`.

## [1.92.0] - 2026-06-18

### Added
- **State-coverage matrix (VV-11, part 1).** New `internal/statecov` + `cmd/statecov`: a coverage axis orthogonal
  to PC/branch coverage (VV-3). Instead of "which instructions ran", it answers "which TIA *modes* did the test
  exercise" — NUSIZ copies, missile/ball size, VDELP0/P1/BL, playfield reflect/score/priority, and bank switches —
  by sampling `emu.ReadTIARegisters` + `Bank` once per scanline over a multi-frame run. An axis stuck at its reset
  value is a verification blind spot. `cmd/statecov` reports distinct values + a coverage fraction per axis (JSON).
- **Coverage-filtered mutation = honest kill rate (VV-11, part 2).** `mutate.EvalRandomCovered` (and
  `cmd/mutate -covered -frames N`) restricts fault injection to ROM offsets that a baseline run actually executes
  (PC coverage via `emu.SeenPCs`). Naive mutation dilutes the kill rate with mutations in never-executed code that
  can never be killed; the covered variant measures the suite against live code only. On `smoke.bin` (mostly
  unexecuted 4K padding) the same suite scores **2% naive vs 68% covered** — closing the testing-playbook's
  misleading 5–20% kill-rate thread.

### Notes
- Self-tests both directions: the matrix must distinguish a mode-exercising ROM (multicolor48) from one that never
  moves that mode (smoke), and a bank-switching ROM from a flat one; the covered kill rate must exceed the naive
  rate and be non-vacuous + deterministic. Pure Go, no reconnect.
- `.gitignore`: ignore the remaining stray repo-root cmd binaries (`/statecov`, `/mutate`, `/cover`,
  `/guidedfuzz`, `/trajdiff`).

## [1.91.0] - 2026-06-17

### Added
- **perfect6502 silicon CPU differential (VV-7).** New `internal/cpudiff` + `cmd/cpucheck`: a hardware-grade
  differential of the embedded Gopher2600 CPU core against the perfect6502 transistor netlist (mist64/perfect6502,
  the visual6502 model), one instruction at a time. This is a **CPU-layer** oracle — perfect6502 has no TIA/RIOT
  and cannot run a 2600 ROM, so it is **not** a member of the full-system RAM vote (`cmd/oraclevote`). Its value:
  catching a CPU bug that Gopher2600 and MAME (both software) could share, and covering undocumented / decimal
  opcodes that the fixed Tom Harte corpus (VV-1) excludes.
  - `internal/cpudiff/p6502step/p6502step.c` (first-party): runs exactly one instruction on the netlist. Register
    injection via a `measure.c`-style prologue (perfect6502 exposes no register writers); the single-instruction
    boundary is taken from the **SYNC line** (node 539), making it robust even when control flow returns to the
    instruction (e.g. a branch with offset −2); writes captured as a memory diff. Cycle count and PC pinned
    empirically against known answers.
  - `internal/cpudiff`: **symmetric** execution — both engines run the identical 64K image from the same prologue,
    reaching identical pre-instruction state by construction (`buildImage` mirrors the C harness). Differ masks P
    bits 4/5 (B/unused — convention-only). Seeded deterministic vector generator. Empirically established
    **allow-list** of the only opcodes permitted to diverge: 11 illegal/unstable ones (ANC `0B`/`2B`, ALR `4B`,
    ARR `6B`, ANE `8B`, LXA `AB`, SH* `93`/`9B`/`9C`/`9E`/`9F`, LAS `BB`).
  - `cmd/cpucheck`: CLI (`-seed`/`-n`/`-opcodes all|smoke`), JSON summary, exit 1 on any **unexpected** divergence
    (a documented-opcode disagreement = a real CPU bug or a harness artifact). Gated on `bin/p6502step`.
  - `scripts/install_perfect6502.sh`: fetch the pinned clone (`09fc542`, MIT) + build `bin/p6502step`. The
    perfect6502 source is gitignored, never vendored — mirroring how the Gopher2600 clone is handled.
- Self-tests: always-on differ-logic (planted-mutant, both directions, no binary needed) locks the comparator in
  CI; gated silicon differential confirms 0 documented-opcode divergences across many seeds + determinism.

### Notes
- Main build remains **CGO-free** (`CGO_ENABLED=0 go build ./...`): perfect6502 is an external binary, shelled out.
- `.gitignore`: added `/third_party/perfect6502/`, MAME scratch (`/cfg/`, `/snap/`), and stray root binaries
  `/cpucheck`, `/oraclevote`.

## [1.90.0] - 2026-06-17

### Added
- **MAME headless cross-oracle (VV-6).** New `internal/oracle` package: an `Oracle` interface (`DumpRAM`: run a
  ROM from power-on for N frames → RAM $80-$FF), the embedded `Gopher2600` member, `Diff`, and `Vote` (majority
  RAM dump + named dissenters). Extracted from `cmd/stellacheck` (which now reuses `oracle.Gopher`). `oracle.Mame`
  runs MAME's a2600 driver with `-video none -skip_gameinfo` and a lua autoboot script that dumps RAM after N
  frames — a genuinely independent, **fully hands-free** third emulator (unlike the Stella oracle's human
  keypress), CGO-free (shells out to the `mame` binary). New `cmd/oraclevote` runs every available oracle
  (Gopher2600 always, MAME if installed) and reports a majority verdict + dissenters (exit 1 on dissent) =
  "all software agrees but the hardware-grade member disagrees" made visible — the suite's reason to exist.
  Self-test (gated on MAME present): MAME reads smoke's ram.0x80==66 and agrees with Gopher2600 on all 128 RAM
  bytes, voting unanimously; `TestVoteDissent` proves a planted lone dissenter is named. VV-7 (perfect6502
  silicon-netlist CPU oracle) will plug into the same `cmd/oraclevote`. **Src:** MAME luascript docs.

## [1.89.0] - 2026-06-17

### Added
- **VV-2 green-ification: `@lines N` per-region budget for 2-line kernels.** A legitimate 2-line kernel does
  ~2 scanlines of CPU work between WSYNCs (~152cy), which the fixed 1-line budget (76) wrongly flags. A
  `; @lines N` note on the source line that opens a WSYNC region now sets that region's budget to N*76, greening
  real 2-line kernels (multicolor48 / score6 / tia_pcm / exerciser) without weakening the proof — an
  un-annotated over-76 region still flags. `srcmap` gained an exported `Line(pc)`; `cyclebound.Prove` reads
  `@lines` from the region opener's source line (scanning the mapped line + the next, since DASM maps a labeled
  WSYNC to its label line). Sound: the annotation only scales a specific region's budget, never disables a
  check. Planted/clean litmus `cb_2line` (`@lines 2`, region 139cy → certified) vs `cb_2line_noann` (same
  kernel, no note → 139>76 flagged); `TestTwoLineBudgetAnnotation` locks both directions. Applying the
  annotations to the actual game ROMs is a roms-repo follow-on. **Src:** Li&Malik IPET DAC'95.

## [1.88.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-3: uninitialized-RAM read — VV-10 complete.** `Emu.WatchUninitRead` flags the
  first read of a RAM byte ($80-$FF) never written since reset = the passes-in-emu (deterministic value) /
  fails-on-HW (power-up garbage) hazard. The enabler is `Emu.effectiveAddr`, which resolves the true memory
  address of every operand mode — Absolute (zero-page folded in via `Defn.Bytes==2`), AbsoluteX/Y (with zp
  wrap), and `(ind,X)`/`(ind),Y` via pointer dereference — so an indexed clear loop (`sta $00,x` writing 128
  RAM bytes in one instruction) is fully tracked and **not** a false positive. Stack push/pull are implied (no
  operand) and so fall outside this operand-based tracker, self-consistently. Exposed as the scenario check
  `checks.no_uninit_read`, run on a **fresh emu from reset** (uninit-read is a from-reset property, unlike the
  per-frame T-1/T-2). Planted/clean litmus (`uninit_trap` reads $90 with no clear = hit; `uninit_clean` indexed-
  clears then reads = no hit) with `TestUninitReadDetector` locking both directions (proving the indexed clear
  is not a false positive). **VV-10 is now complete (T-1 timer-wrap / T-2 HMOVE-latch / T-3 uninit-RAM-read).**
  **Src:** known-traps.md §A/§D; Valgrind Memcheck (shadow memory).

## [1.87.0] - 2026-06-17

### Added
- **Score OCR semantic oracle (VV-9).** `internal/ocr` reads the RENDERED digit pixels (not the registers) and
  decodes a displayed 2-digit packed-BCD score, matching each glyph against templates rendered from a
  ground-truth font (the spec — PF1=MSB-first / PF2=LSB-first per the verified playfield bit order). It asserts
  displayed == `decode(RAM)`, tying the display back to program meaning — catching display-kernel / BCD-split /
  font-index bugs that an exact frame hash would pass (a hash also accepts a consistently-wrong glyph). The band
  is located by detecting its top then sampling at the kernel's fixed row spacing (robust to blank glyph rows).
  Exposed as the scenario check `checks.score_equals_ram` (ground-truth font from a `<scenario>.font` sibling
  file, like golden files; no MCP tool, no reconnect). Litmus `score2.asm` renders RAM $80 (packed BCD '42') via
  PF1/PF2. Self-test `TestScoreOCRSelfTest`: the genuine ROM decodes 42 == RAM; a font-index mutation (glyph 8
  copied over glyph 4 in the ROM, RAM untouched) is caught as displayed≠RAM. **Src:** pHash Hamming primitive.

## [1.86.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-2: HMOVE-then-HMxx-within-24cy (VV-10).** `Emu.WatchHMOVEHazard` flags the
  first write to a motion register (HMP0/HMP1/HMM0/HMM1/HMBL or HMCLR) within 24 CPU cycles of an HMOVE strobe —
  the documented "unpredictable motion" hazard (Stella PG). The 24-cycle window is measured in **color clocks**
  (72 = 24 CPU cy) via `Coords`, not the executed-cycle counter (which excludes WSYNC stalls), so a clean
  kernel that separates HMOVE from HMxx with a WSYNC is correctly judged outside the window. Exposed as the
  scenario check `checks.no_hmove_hazard`. Planted/clean litmus pair (`hmove_trap` writes HMP0 ~3cy after HMOVE
  = hit; `hmove_clean` sets HMxx in VBLANK and strobes HMOVE right after a WSYNC = no hit) with
  `TestHMOVEHazardDetector` locking both directions. VV-10's T-3 (uninitialized-RAM read) remains a follow-on:
  a correct shadow-memory detector needs full effective-address resolution for indexed/`(ind),Y` writes.
  **Src:** Stella Programmer's Guide (HMOVE timing); known-traps.md §A/§D.

## [1.85.0] - 2026-06-17

### Added
- **HW-divergence trap detector T-1: RIOT timer-wrap / G8 (VV-10, partial).** `Emu.TimerState` exposes the RIOT
  timer (INTIM/TIMINT/Expired/Divider/ticksRemaining) and `Emu.WatchTimerWrap` flags the first time a program
  **reads INTIM while the timer has already underflowed/wrapped (Expired set)** — the G8 hazard (too-small
  interval, or a poll loop that stepped over 0 and is consuming post-wrap values). Exposed as the scenario check
  `checks.no_timer_wrap` (frames to watch) — no MCP tool, no reconnect.
- **Key finding (measured):** the audit's one-line spec "flag the wrap" is too naive and would false-positive on
  a *correct* kernel — `cb_timer` (which polls INTIM to 0 properly) also lets the timer wrap later in the frame,
  but nothing reads INTIM then. So the trap is narrowed to *read-after-wrap*, and `Expired` is sampled BEFORE
  each instruction because reading INTIM clears it (the timer's reversion). Planted/clean litmus pair
  (`timerwrap_trap` TIM1T poll overshoots 0 = hit; `timerwrap_clean` TIM64T polled to 0 = no hit) with
  `TestTimerWrapDetector` locking both directions. T-2 (HMOVE-then-HMxx<24cy) and T-3 (uninitialized-RAM read)
  remain follow-ons. **Src:** known-traps.md §A/§D; AtariAge 303277; Valgrind Memcheck (shadow memory).

## [1.84.0] - 2026-06-17

### Added
- **Behavioral trajectory diff (VV-8).** `internal/trajdiff` + `cmd/trajdiff` step an original and a candidate
  ROM in lockstep on the same input timeline and report the first frame+field where their observable state
  diverges, or MATCH. The default trajectory is the 128-byte RAM each frame (`emu.PeekRAM`); custom fields reuse
  `scenario.ResolveField`. It compares **behavior over time, not bytes**, so a dead/cosmetic byte difference is
  a MATCH while a real behavioral change is caught at the exact frame — the strongest oracle for a reproduction
  task (and a step beyond `refdiff`'s static snapshot). Pure Go, no external dependency, no reconnect. Self-test
  (`TestTrajdiffSelfTest`): identity = MATCH (determinism guard), a corrupted reset vector diverges, a
  behaviorally dead-byte flip = MATCH. The CLI exits 1 on divergence, 0 on MATCH. **Src:** Martignoni TOSEM'13;
  EXAMINER ASPLOS'22; McKeeman 1998.

## [1.83.0] - 2026-06-17

### Added
- **PC/branch coverage + coverage-guided fuzzing (VV-3).** Closes the last Tier-★1 gap — the test-adequacy
  axis plus AFL-style feedback fuzzing (the existing scenario `fuzz` is blind).
  - **Coverage recorder** (`internal/emu.Coverage`): an opt-in hook in `stepInstr` (at instruction completion,
    reading `LastResult.Address`/`Defn.IsBranch()`/`BranchSuccess`) records executed instruction addresses and
    per-branch taken/fall-through edges. Exposes `PCCount`/`BranchCount`/`EdgeCount`/`OneSidedBranches`
    (a branch whose other side was never exercised)/`Seen`/`Signature`. Nil until `EnableCoverage` = zero cost.
  - **`cmd/cover`**: drives a ROM and reports reached coverage + one-sided branches. On `cyclebound_branch` it
    flags `0xF036` — the same path VV-2 statically proves overruns (101>76) but runtime never takes, an
    independent cross-check between the two tools.
  - **`internal/guidedfuzz` + `cmd/guidedfuzz`**: AFL-style search that keeps a corpus of input sequences and
    grows it whenever a mutation reveals a new coverage marker (`Coverage.Signature`), climbing toward
    deeply-guarded states blind fuzz essentially never reaches. The search core is decoupled from the emulator
    via `Evaluator`, so it is unit-testable; `EmuEvaluator` wires it to a fresh deterministic emu per run.
  - Self-test: `TestCoverageLogic` (one-sided detection; an unrecorded address reads as uncovered),
    `TestGuidedBeatsBlind` (synthetic staircase oracle — guided reaches full depth = 9 markers while blind
    stalls at 4 on the same 6000-iteration budget, deterministic and ROM-independent), plus emu-wiring
    integration tests. Scope (honest): full dead-code over the decodable universe is a follow-on; today's map
    is reached-coverage + one-sided branches. **Src:** Zalewski AFL whitepaper; Go native fuzzing.

## [1.82.0] - 2026-06-17

### Added
- **Temporal-logic trace assertions (VV-5).** New scenario `temporal` block for bounded-temporal-logic
  properties over the **frame sequence** — things an instantaneous `assert` or a per-frame `invariant` cannot
  express. Three monitor kinds (`always P` stays as the existing `invariant`, not duplicated):
  - **`eventually`** — P must hold within `within` frames of the run start (bounded liveness).
  - **`response`** — whenever trigger A holds at frame *f*, P must hold within `within` frames (*f*..*f*+within).
  - **`never_for`** — P must not hold for `n` consecutive frames (safety).
  Implemented in `internal/scenario` by reusing the existing condition vocabulary (`resolve` + `condPass` +
  `condDesc`): each monitor's proposition (and the response trigger) is observed into a per-frame boolean trace
  inside the run loop, then the verdict is computed off the trace. Liveness whose window is not fully observed
  reports **INCONCLUSIVE** (`Pass:false`) so it can never be a vacuous green. **Scenario-only — no MCP tool,
  hence no server rebuild / reconnect** (a deliberate low-friction choice this session). Self-test:
  `TestEvalTemporal` fixes pass/fail/inconclusive for all three operators on planted boolean traces
  (frame-base independent) and `TestTemporalThroughRun` proves the resolve→observe→eval wiring end-to-end on
  `smoke.bin` (plus inconclusive-is-not-green and invalid-definition rejection). Sample
  `roms/litmus/scenarios/temporal.json`. Docs: `docs/scenarios.md`, `docs/testing-playbook.md`. **Src:**
  Bauer/Leucker/Schallhart TOSEM 2011 (LTL₃); STL RV'15.

## [1.81.0] - 2026-06-17

### Changed
- **VV-2 prover precision S0–S3 (fewer false positives, same proof strength).** The static per-scanline
  cycle-budget prover gained an abstract-interpretation layer so it stops flagging sound kernels:
  - **S0 — abstract-interpretation engine** (`internal/cyclebound/absint.go`): tracks a per-address
    value-range state (registers / known constants) by forward dataflow from the reset/IRQ/NMI entries.
  - **S1 — region recognition**: VSYNC/VBLANK and timer-driven (TIM64T) intervals are classified and skipped,
    so a legitimately long blank region is no longer reported as over-budget.
  - **S3 — page-cross precision**: an `abs,X`/`abs,Y` read's +1 page-cross penalty is now resolved from the
    proven index range — if `[base+lo, base+hi]` provably stays inside one 256-byte page the penalty is 0;
    an unknown index, or a pointer-based `(ind),Y` whose base we don't track, stays conservative (+1). The
    abstract state is wired into the solver (`solver.absStates`) and applied via `baseCost()+pagePenalty()`.
    Loop-body costing keeps the conservative `nodeCost` (sound, over-approximating).
- The proof stays **sound**: every relaxation is on the false-positive side only. `TestCycleboundSelfTest`
  (planted-discrepancy, "no false-negatives") stays green, and prove⇔assert agreement was re-verified on the
  litmus set (`cb_clean`/`cb_timer` certified; `cb_roll`/`cyclebound_branch` (101>76)/`litmus_overrun` (108)
  flagged).

### Notes
- **Finding (recorded to memory `feedback-verification-standard`):** a *small* per-scanline overrun (one heavy
  line = 262→263 scanlines) is **visually invisible** — the TV's auto-sync absorbs a one-line slip, so
  `cb_roll` and `cb_clean` render pixel-identically. This is precisely why a static ∀-prover exists: the
  defect is unseeable, only the numbers differ. Visual verification is unfit for this class of timing defect.

## [1.80.0] - 2026-06-17

### Added
- **Static per-scanline cycle-budget PROVER (VV-2).** Proves the worst-case CPU cycles of every
  `STA WSYNC`-to-`STA WSYNC` region over **ALL reachable paths** (∀) — the static sibling of the runtime
  `assert_line_budget` (which observes only one run, ∃) and the flagship attack on gap B (timing).
  `internal/cyclebound` recursive-descent-decodes the ROM from its reset/IRQ/NMI vectors (so inline data
  isn't misdecoded), costs each instruction from the in-tree exact table (`instructions.Definitions`: cycles
  + branch-taken/page penalties), cuts the CFG at every `STA WSYNC` ($02), and proves each region's DAG
  longest path ≤ budget (default 76, no solver). Counted loops (`ldx/ldy #N` + `dex/dey` + `bne/bpl`) are
  folded by their bound; JSR / indirect JMP / unbounded loops are reported honestly as out-of-scope, never
  silently passed; over-budget regions return a cycle-by-cycle worst path + source location. Shipped 3 ways:
  the `cmd/cyclebound` CLI, the **`prove_line_budget` MCP tool** (run it before executing a kernel), and the
  scenario **`checks.prove_line_budget`** regression gate (`cyclebound_safe.json` certifies smoke). New litmus
  `cyclebound_branch` overruns only on one branch (~101cy) so a live run is a lucky pass yet the proof flags
  it; the planted-discrepancy self-test (`TestCycleboundSelfTest`) also bounds `litmus_overrun`'s counted
  delay loop (108cy), certifies `smoke` (worst 19cy), flips smoke to a violation under a tight budget
  (non-vacuous), and checks the certified bound holds at runtime (observed-within-proven dual). The only
  ∀-claim member of the suite. Scope (v1, honest over guessing): single-bank flat 2K/4K; only the
  `ldx/ldy #N`+`dex/dey`+`bne/bpl` loop idiom is bounded (divide-by-15 positioning and other A-reg/memory-counter
  loops report unbounded rather than risk a false violation); page-sensitive reads charged a conservative +1; a
  0-WSYNC ROM is reported unbounded, never vacuously certified. Src: Li & Malik IPET (DAC'95); Ballabriga &
  Cassé (WCET'08).

## [1.79.0] - 2026-06-17

### Added
- **Motion-smoothness / jerk metric (VV-4).** Turns "does this object judder / ブルブル" into numbers:
  `internal/motion` tracks a TIA object's exact X (`Markers().HmovedPixel`) and rendered top over N frames →
  velocity (1st diff), acceleration (2nd diff), and **jerk_rms** (RMS of the 2nd difference; 0 = constant
  velocity) plus `max_accel`/`monotonic` (a real glitch/snap vs a benign integer-pixel staircase). Shipped as
  the `cmd/motion` CLI, the **`read_motion` MCP tool** (interactive — automates the hand frame-by-frame trace),
  and the scenario **`checks.motion`** regression gate. New litmus `motion_glide` (clean +1/frame → jerk 0) and
  `motion_stutter` (+2,0,+2,0 → jerk 2); the planted-discrepancy self-test (`TestMotionSelfTest` +
  `scenarios/motion_glide.json`) locks that the stutter scores above the glide, so the metric can't be vacuous.
  Used live on the Breakout ball (vertical jerk 0, horizontal jerk 1 = the benign 1px/2-frame staircase) and
  validated against the user's own perception (motion_stutter in Stella reproduced their reported judder).
  First Tier-1 perceptual oracle of the verification-variety backlog. Src: Flash & Hogan 1985 (minimum-jerk).

## [1.78.0] - 2026-06-17

### Added
- **CPU-core conformance gate in CI (VV-1).** The embedded Gopher2600 CPU is now certified against two
  external authoritative suites already vendored in the clone but never run by harness CI: **Klaus Dormann**
  6502 functional+decimal (embedded `.bin`, always-on) and a **Tom Harte / SingleStepTests 65x02** subset
  (per-cycle bus addr/data/read-write + final state + cycle count; 12-opcode smoke fetched on demand, MIT,
  not vendored; full 256 is local-only, ~1GB). Suites run via the full import path
  `go test github.com/jetsetilly/gopher2600/hardware/cpu/tests/{klaus2m5,thomharte}/...` (`replace`-resolved).
  New `scripts/check_cpu_conformance.sh` (+ `--selftest`) and two CI steps. The gate is **self-validated** by a
  planted-discrepancy: a corrupted expected value must make the run go RED (proven live, not vacuous). First
  Tier-1 pilot of the verification-variety backlog (`docs/capability-gap-audit.md`). Src: Klaus2m5 repo;
  SingleStepTests/65x02.

## [1.77.0] - 2026-06-17

### Changed
- **Verification discipline consolidated into a single canonical standard** (`feedback-verification-standard`,
  "MAX"). Iron rule 1 (`CLAUDE.md`) and the authoring-protocol Verify step (step 5) now **reference** that
  standard's MAX checklist instead of restating it: trace frame-by-frame, read the full object window (no
  partial reads), cross-check derived formulas against raw pixels, kill each hypothesis with data, prove the
  negative, present the measured table. Born from the Breakout ball-judder investigation (proved "not a bug"
  purely by `read_row` measurement).
- **Rule-base de-duplicated to "1 rule = 1 source of truth."** Memory feedback rules merged 18→10 (the
  verification cluster collapsed 5→1; goal 3→1; execution 2→1; work-tracking 2→1). Stale `[[memory-links]]`
  in `docs/` (design-principles, build-to-learn, casebook, testing-playbook) repointed to the new canonical
  names. No behavioural code change; CI/wiring unaffected.

## [1.76.0] - 2026-06-16

### Added
- **`set_input` now drives the console panel switches** (`reset`/`select`/`color`/`p0pro`/`p1pro`), not just the
  joystick — routes to the existing `emu.SetPanel`. Lets Claude press GAME RESET to actually start a game (e.g.
  the real Breakout, which needs RESET to leave attract mode) so its sprites can be measured live. Motivated by
  a `refdiff` gap: the original's ball height couldn't be rendered/measured because the game wouldn't start.
- **`internal/refdiff` + `cmd/refdiff`** — differential layout check vs a reference ROM (the original = oracle):
  extracts a fingerprint (left/right wall clock, ball **width and height**) and diffs it against the original.
  `MeasureBall` starts the game (RESET) and tries both control styles (joystick fire / paddle) to render the
  ball in the open field. Catches "wrong vs the original" that golden self-regression can't (a wall inset from
  the edge, an undersized ball). Worked example: a user spotted my Breakout's left-wall gap + 1×1 ball by
  *playing*; refdiff went RED (wall 2 vs 0, ball 1×1 vs 2×4), drove the fix to MATCH. Wired into
  `docs/testing-playbook.md` (the differential-vs-original entry). (Differential testing.)

### Notes
- **1.76.0** (MINOR — additive: panel-switch input + refdiff with ball-size diff). The MCP server must be
  rebuilt and reconnected to use the panel switches (`set_input reset` etc.).

## [1.75.0] - 2026-06-16

### Added
- **`docs/testing-playbook.md`** (new) — imports the established software-testing discipline (the **oracle
  problem** → invariants/contracts, property-based, metamorphic, differential/golden, fuzzing, **deterministic
  simulation testing** à la FoundationDB/Antithesis, mutation testing, invariant mining, delta debugging) and
  maps each onto this harness, with a per-build verification checklist usable today via `run_scenario` + MCP.
  Wired into the CLAUDE.md routing (step 5b) + authoring-protocol step 5. Motivated by `feedback-verify-at-claim-level`
  (verify at the level of the claim — for emergent behaviour, demonstrate it; don't infer it from component checks).
  Backlog for the executable backers added to `docs/capability-gap-audit.md` as **G10–G14** (scenario
  `invariants`/`monotonic`/range, `fuzz`, `metamorphic`, `mutation`, `mine-invariants`). Provenance recorded
  (QuickCheck, Daikon, AFL, FoundationDB/Antithesis, Chen/Segura, Barr, DeMillo, Zeller).
- **Automated verification suite — G10–G14 all delivered** (the executable backers of the playbook):
  - `internal/scenario`: **`invariants`** (a condition checked every frame), **`monotonic`** (a field that
    only moves one way over the run), the **`in`** range operator, **`fuzz`** (seeded random input +
    per-frame invariant monitoring + CPU-jam detection, deterministic = replay by seed), and **`metrics`**
    (fields captured at end of run). `run_scenario` MCP gains all of these automatically.
  - `internal/mutate` + **`cmd/mutate`** — mutation testing (inject a ROM-byte fault; confirm the suite kills
    it, or flag a survivor = weak checks; seeded batch kill rate).
  - `internal/metamorphic` + **`cmd/metamorphic`** — assert a relation `A.field <rel> B.field` between two
    runs (oracle-free).
  - `internal/mine` + **`cmd/mine-invariants`** — Daikon-lite: observe a driven run, emit candidate
    `invariants`/`monotonic` as a spec draft (`scenario.ResolveField` exported for shared vocabulary).
  - `build.Assemble` now passes `-I<asm dir>` so `.asm` scenarios resolve includes (e.g. `vcs.h`) from any cwd.
  - **First catch:** the Breakout `fuzz` scenario exposed the frame was 264 lines, not the "262" claimed by
    eye (never measured); fixed to a true 262 in the roms repo. New litmus scenarios `invariants.json`,
    `fuzz.json`. Tests added for every package.
- **`docs/build-to-learn.md`** (new) — reusable methodology: reproduce a real game mechanic-by-mechanic to
  turn "can read" into "can author". Wired into the CLAUDE.md routing table (step 1c) + cross-linked with
  casebook. First worked example = **Breakout** (`roms/breakout/`, 8 rungs from stable frame to a playable
  single-player game, each verified numerically against the real ROM; per-rung snapshots in `steps/`).
- Methodology refinements (user-driven): **measure the original's dimensional layout in Phase 0** (before
  building — retrofitting layout in assembly is costly); **judge colour AND size by `read_row`, not by eye**
  (caught the paddle being mis-set to white-24px when the original is red-16px); **read_row = measurement /
  CXxx collision = runtime check + contact verification** (two-tool split).
- **`docs/casebook.md`** — Breakout entry (the build-to-learn worked example: multi-region PF kernel,
  RAM-driven destructible PF, BL/P0 positioning, joystick paddle, position-based collision, game-state loop).

### Notes
- **1.75.0** (MINOR — additive: build-to-learn + casebook docs, the testing-playbook, and the automated
  verification suite G10–G14; all backward-compatible).
- The MCP server (`cmd/harness`) must be **rebuilt** for `run_scenario` to pick up the new scenario features
  (`invariants`/`monotonic`/`fuzz`/`metrics`) — smoke-test with `scripts/mcp_smoke.py` then reconnect.
- The authored Breakout ROM + its scenarios live in the **roms** repo (`roms/breakout/`), not here.

## [1.73.0] - 2026-06-15

### Added
- **`docs/casebook.md`** (new) — the *situation → technique* canon, evidence-backed by real commercial-game
  disassemblies (companion to `cookbook.md`'s forward recipes). Wired into the CLAUDE.md routing table
  (step 1b) and the authoring-protocol retrieve step; `check_wiring`/`check_provenance` green.
  - First case study: **Fishing Derby (Activision 1980, David Crane / Dennis Debro disassembly)** — the
    3-layer casebook pilot (manual=spec × disassembly=impl × Claude reconstruction). Raw pairing lives
    study-only (non-repo) under `reference/disassemblies/_casestudies/fishing-derby/`.
- **design-principles.md** — three new principles distilled from the pilot (with provenance):
  per-scanline **NUSIZ+HMOVE shaping** of one player into an 8px-plus irregular sprite (shark);
  **fractional-HMOVE slope** drawing of an arbitrary-angle 1px line on a missile/ball (fishing line);
  background shimmer by streaming a PRNG's bits to `COLUBK` per scanline (near-zero cost).
- **capability-gap-audit.md** — G9: authoring-craft support for the two shaping/slope patterns the
  Claude-side reconstruction missed (concrete-driven, build when the next ROM needs it).

### Notes
- Methodology refinement (user, 2026-06-15): **Layer1 spec = manual + live ROM observation**, not the manual
  alone — every reconstruction error was corrected by running the real ROM (feedback-play-the-rom-not-just-manual).
- Proposed release: **1.73.0** (MINOR — additive knowledge). Tag + push deferred to user approval.

## [1.72.0] - 2026-06-15

### Added
- **Knowledge-activation architecture** — so the whole accumulated corpus *fires* at authoring time and nothing
  rots unused ([[knowledge-activation-architecture]]).
  - `docs/authoring-protocol.md` is now the single **START HERE** entry for building a ROM: the mined pro
    workflow (A–E: image-first → 14-step build order → ceiling → audio truths → cycle-budget craft) above the
    6-step loop. CLAUDE.md iron rule 5 points to it.
  - **CLAUDE.md routing reorganized into the authoring-flow order** (① building a ROM, in sequence · ② reference).
  - `scripts/check_wiring.py` (**CI-gated**) — fails if any `docs/*.md` is unreachable from the routing table or
    the protocol; structurally prevents knowledge from orphaning. Caught + wired `capability-gap-audit.md`.
  - `docs/mining-digest.md` now also indexes the **117 dev-blog entries** (generated by `gen_mining_digest.py`).

### Changed
- CI now runs three knowledge lints — `check_provenance` (origins) + `check_traps` (traps) + `check_wiring`
  (no orphans). Green means knowledge is traceable, sound, and reachable.

## [1.71.1] - 2026-06-15

### Changed
- Completed the dev-blog gold absorption into `design-principles.md` (the two >2-object flicker algorithms;
  drop-PF0 cycle/RAM trade; 2-zone complementary-height moving platforms; 8-byte self-modifying init +
  hotspot placement) and `known-traps.md` (AUDF-lowering ≤32-cycle propagation latency). Beyond-bB findings
  (DPC+/ARM/CDF data-exchange, Slick/Fast-Fetch kernels, wav2tia, INT2HEX/INT2BITS) logged as technique
  candidates. All sourced to mined blog entries.

### Planned (historical — resolved; formerly a second `## [Unreleased]` heading)
> This block was a stray **second** `## [Unreleased]` heading stranded here between 1.71.1 and 1.71.0. It
> entered the file on 2026-06-10 in `f8ae33d` ("docs: English CHANGELOG"), was never cleared, and every later
> release was prepended above it. Demoted to a dated note on 2026-07-30 so the file has exactly one
> `[Unreleased]`. The three items are kept verbatim; all three had already shipped by the time this block was
> buried:
- Real game authoring on top of the 1.0 base (1.x). — delivered across the 1.x line.
- Stella oracle v2 (TIA/pixel compare, full keystroke automation); Slocum note-table transcription for composing.
  — Stella oracle v2 delivered in **1.54.0** (`stellacheck -pixels` / `-snap`, F-4 closed, hands-free via
  `scripts/stella_oracle.sh`); Slocum note-table transcription delivered in **1.35.0**
  (`pkg/audio.NoteFreq/FindNote` + `cmd/jingle`).

## [1.71.0] - 2026-06-15

### Added
- **Authoring loop tooling (Track E).**
  - `scripts/check_traps.py` — static pre-flight linter for the `docs/known-traps.md` traps (unstable illegal
    opcodes, `NOP $00` bankswitch, stack-collision vars, missing CLD/CLEAN_START). Zero false positives on the
    31 technique ROMs; `--selftest` proves detectors fire; **CI-gated**.
  - `docs/authoring-protocol.md` — the 6-step loop (retrieve→plan→author→preflight→verify→**feedback**) run on
    every kernel; the feedback step makes each production strengthen the system.
  - `docs/cookbook.md` — intent→recipe (game-type → technique stack + traps + checks) + the canonical bottom-up
    14-step build order (from SpiceWare's "Collect" tutorial).
- **AtariAge dev-blog mining (expansion).** 117 dev-blog entries distilled (SpiceWare's *Collect / Stay Frosty /
  Frantic / Draconian* dev diaries + DPC+/ARM + TIA-audio internals), fetched **Wayback-only** (CDX enumeration
  of `blogs/entry/*`). Key absorptions into `design-principles.md`: the **Photoshop-mock→48px flicker-free 2-color
  title** path (the project's image→assembly route), the 21-cy mask-sprite draw, and the `sta.w`/`.FORCE`
  one-cycle RESP trim. Corpus + gaps recorded in `reference/atariage/RECOVERY-TODO.md`.

### Notes
- AtariAge automated direct-access suspended (the account hit IPS's login-throttle lockout); mining is
  Wayback-only henceforth. Remaining gaps (15 blog + a few forum, all re-fetchable) listed in RECOVERY-TODO.

## [1.70.0] - 2026-06-15

### Added
- **AtariAge forum-50/31 mining campaign complete + absorbed.** All 761 `TO_MINE` threads from the
  Programming + Newbies forums deep-mined (850 total in `reference/atariage/MINED.csv`); 1727 threads
  triaged (provenance/checklists live under the umbrella `reference/`, not committed).
  - `scripts/gen_mining_digest.py` keeps `docs/mining-digest.md` (850 threads → principle/function it feeds)
    in sync from `MINED.csv`.
  - `scripts/aa_fetch.py` gained `-direct-first` (AtariAge cookie lane) so parallel mining splits load across
    two backends (Wayback + Cloudflare) without contending.
  - Heavy `docs/design-principles.md` absorption: **positioning ground-truth** (RESxx internal draw delay
    player+5 / missile·ball+4 CLK, multi-object cyc23 rule, X≤134 spill), RIOT timer-wraparound roll trap,
    illegal-opcode stability map, TIA-revision pixel-match caveat, mid-scanline GRP-rewrite multiplexing,
    resource triangle + TJ register convention, subpixel/ballistic physics, pixel-aspect source-spread
    (1.67–1.82; codified 2.0 flagged as over — pending one Stella measurement).
- **`docs/known-traps.md`** — catalog of "passes in the emulator, breaks on real hardware" traps (timer
  wraparound, HMOVE-24cy, NOP-$00 bankswitch, page-cross, illegal-opcode stability, TIA read floats,
  mid-line NUSIZ emu-only…), each sourced. The kernel pre-flight checklist and the spec for the future
  `check_traps.py`. Directly targets the timing class of bug that killed past Pong attempts.
- **Provenance enforcement.** `scripts/check_provenance.py` lints that every technique doc / `pkg/design`
  function / design rule cites its origin (CI-gated); `--list` regenerates `docs/provenance.md` (every
  element → its source). Rule recorded so a production issue can always be traced to the original thread.

## [1.69.0] - 2026-06-14

### Added
- **Seven new verified techniques** (built in parallel from mined technique-candidates, each clean-room
  implemented + locked by a CI scenario; all 31 technique scenarios pass, `ntsc_frame_lines:262` + golden):
  - **`road`** (㉓) — pseudo-3D road: M0/M1 shoulders + BL dashed centre, widening per perspective band
    (fills the only gap from the 8bitworkshop cross-check).
  - **`maze`** (⑲) — Entombed-style procedural playfield maze: LFSR bits doubled to 2px cells, scrolled, reflected.
  - **`tia_pcm`** (㉑) — digitized sample playback via AUDV (AUDC=0), 1-bit ADPCM, pseudo-5-bit 2-channel DAC; audio golden.
  - **`shared_setxpos`** (㉒) — position all 5 movable objects with one indexed `RESPx,x`/`HMPx,x` loop.
  - **`divtable`** (⑮) — constant divide ÷3/7/10/15 (corrected reciprocal-multiply, exact over 0..255 + remainder; Go-model exhaustive).
  - **`multicolor48`** (⑯) — 48-px graphic with per-row COLUPx color (~73/76 cy line budget).
  - **`rts_dispatch`** (⑱) — RTS-stack modular kernel dispatch: data-driven vertical zones at ~6cy/transition.
  - Catalog updated (`docs/techniques/README.md`); each has `docs/techniques/<name>.md`.

## [1.68.0] - 2026-06-14

### Added
- **Forum-50 mining run absorbed (checklist + deep-mine + knowledge).**
  - `scripts/gen_mining_digest.py`: idempotent generator for `docs/mining-digest.md` from
    `reference/atariage/MINED.csv` (keyword + curated-override category inference). Digest now 89 threads.
  - Deep-mined 12 high-value forum-50 threads into `design-principles.md`: the **RIOT 6532 timer-wraparound
    roll trap** (double-write `TIM64T`; the rare "passes-in-Stella / rolls-on-hardware" trap, diagnosed
    in-thread by the Gopher2600 author), early-HMOVE don't-move value = 8, HMOVE cy73-74 comb avoidance,
    div15 fine-motion range, flicker luminance tuning, and a **pixel-aspect refinement** (190154 5:3 ≈
    169128 12:7, both ≈1.7 vs the codified 2:1 — flagged for measurement, *not* codified, per verification-first).
  - `capability-gap-audit.md` **G8**: candidate RIOT timer-wraparound roll detector (sibling to `assert_line_budget`).
- **8bitworkshop sample cross-check** (`docs/8bitworkshop-crosscheck.md`). Steven Hugg's "Making Games for
  the Atari 2600" examples assembled in our toolchain (**26/26**, DASM bundled `vcs.h`/`macro.h`) and run in
  the harness; `multisprite3` verified in depth (8 multiplexed sprites read back at `fidelity:1`). Maps each
  sample to our technique library: **25/26 covered**, one gap — `road` (pseudo-3D road via 2 missiles + ball),
  logged as a technique candidate. External audit confirms `roms/techniques/` covers the standard curriculum.

### Notes
- The triage checklists (`reference/atariage/triage-forum50.csv` ~1317 rows, `triage-forum31.csv` 410 rows;
  1727 threads triaged with reasons) and the per-thread `notes.ja.md` live under the umbrella `reference/`
  (provenance, not committed). batari Basic (forum 65) is intentionally out of scope.

## [1.67.1] - 2026-06-14

### Changed
- **Reframed docs around the post-pivot direction** (TIA Studio canvas editor is frozen; the primary
  consumer of `pkg/design` and the design rules is now Claude's own authoring loop, not the editor).
  Updated `docs/design-principles.md` (intro, craft rules, "implementation" section), `docs/capability-gap-audit.md`
  (frozen banner, G2 marked done in v1.67.0), and the `pkg/design` package doc. Research notes under
  `tools/` are kept as the frozen project's historical record. No code behavior change.

## [1.67.0] - 2026-06-14

### Added
- **Absorbed the accumulated design knowledge into the authoring loop (gap-audit G2 completed).**
  - `pkg/design` now codifies the remaining numeric design-principles rules, not just the first six:
    color-register decomposition + judgment (`Hue`/`Luminance`/`HueName`/`WashoutRisk`/`GradientSameHue`/
    `InterlaceColorsSafe`), coarse÷15 + fine-HMOVE positioning (`PositionSplit`/`CoarseIterations`/
    `HMoveReachable`), PF helpers (`PFTotalColorClocks` reusing `playfield.FullWidth`, `ScoreModeTwoColor`,
    `ScrollScanlinesConstant`), multiplex (`FitsMultiSprite`/`NeedsEmptyYLane`/`RepositionCostScanlines`),
    and craft (`PixelAspectRatio`/`ScanlinesForSquare`/`WalkFrame`/`BackgroundSpec.Feasible`). All table-driven tested.
  - `docs/mining-digest.md`: a self-contained index of the 77 mined AtariAge threads (generated from
    `reference/atariage/MINED.csv`), each mapped to the design-principles section / `pkg/design` function /
    technique candidate it feeds. Raw thread captures stay in the umbrella `reference/` as provenance.
- **Routed the knowledge so authoring uses it.** `CLAUDE.md` gains an iron rule ("design before asm") and
  routing-table rows for `docs/design-principles.md`, `pkg/design/`, and `docs/mining-digest.md`.

### Changed
- `docs/design-principles.md`: every rule now carries a disposition — codifiable rules cross-reference their
  `pkg/design` function (`→ func`), and a new "machine-uncheckable judgment rules" section collects the
  qualitative ones (glyph misreads, thumbnail readability, role split…) with the reason they stay doc-only.
  Recorded the `colorPerRow[]` data-model lesson from the TIA Studio research.

### Decisions
- The raw 77-thread forum captures (3 MB HTML) are **not** moved into the harness; the harness keeps the
  distilled, citable digest while `reference/atariage/` keeps provenance. Keeps concerns separated and the
  published repo English-only.
- TIA Studio learnings: durable, authoring-relevant findings are folded into design-principles; tool-impl
  knowledge (spritemate data model, per-scanline-color UI) is intentionally not absorbed (no effect on
  writing assembly) and stays preserved in the frozen `tia-studio/` repo and research notes.

## [1.66.0] - 2026-06-13

### Added
- **`pkg/design` feasibility checker (gap-audit G2).** Codifies the *hard/numeric* rules from
  `docs/design-principles.md` into executable checks, so TIA Studio (and Claude) can answer "does
  this layout fit?" in code rather than prose: color-band minimum width (`MinColorBandWidthPx` /
  `CheckColorBands`), text capacity by technique (`MaxChars` / `FitsText`), 76cy line budget
  (`LineBudget` / `RemainingCycles`), asymmetric-PF right-half write windows (`AsymRightWindow` /
  `FitsAsymRightWrite`), and multiplex / sprites-per-line limits (`NeedsFlicker`). Soft craft (taste,
  readability) intentionally stays prose. Foundation for milestone M4 (budget feasibility).
- **`docs/capability-gap-audit.md`** — a mined-technique × harness-capability gap audit (G1–G7) with
  a prioritized strengthening backlog (G2 → G1 → G4). Also brought `docs/design-principles.md` and the
  `tools/` TIA Studio research corpus (research-w1..w11, build-readiness, M3/M5 prototype design)
  under version control.
- **`aa_fetch.py` direct AtariAge fetch via `curl_cffi`** (Cloudflare bypass). When `AA_COOKIE`
  (the browser Cookie header incl. `cf_clearance`) is set **and `curl_cffi` is installed**, fetches
  the live forum directly by impersonating Chrome's TLS (JA3) fingerprint — plain `curl` 403s even
  with a valid cookie because Cloudflare fingerprints TLS, not just UA/cookie. Adds `direct_get()` /
  `direct_enabled()`, **live page-count discovery** (`discover_live_pages`, **topic_id-based** so a
  wrong/short slug still resolves) to fill Wayback page gaps, a direct fallback when a Wayback page
  fetch fails, and **direct binary attachment download** (the `.bin`/ROM files Wayback never
  archived). Falls back cleanly to Wayback-only when cookie/`curl_cffi` absent (back-compatible).
  New optional dep: `pip install curl_cffi`.

## [1.65.0] - 2026-06-13

### Added
- **AtariAge mining manifest management** (`scripts/aa_manifest.py`): the single source of truth for
  "which threads are already mined", **regenerated idempotently from the filesystem** (a thread is
  mined iff `reference/atariage/<topic_id>-*/notes.ja.md` exists) → `reference/atariage/MINED.csv`.
  `--check <url|topic_id>` reports MINED (exit 1) / NEW (exit 0). Dedup keys on **topic_id**, so a
  different slug for the same thread is still caught. Stops re-mining the same thread across
  sessions/agents without relying on a hand-kept list.
- **`aa_fetch.py` auto-dedup**: skips a thread (no fetch, no stray dirs) if its topic_id is already
  mined; `-force` overrides. Mining is now mechanically dedup-enforced, not by memory.

### Changed
- `.gitignore`: ignore stray standalone `cmd/*` binaries built to the repo root
  (`/rammap` `/jingle` `/dissect` `/fieldtest` `/calibrate` `/scenario` `/stellacheck` `/ingest`) —
  build to `/bin` instead. (Removed a stray 12 MB `rammap` binary from the working tree.)

## [1.64.0] - 2026-06-13

### Added
- **Technique: instrument-envelope music driver** (`roms/techniques/music_driver.asm` +
  `docs/techniques/music-driver.md`): the step up from the constant-volume sound driver — AUDV is
  driven by a **per-instrument volume envelope every frame** (attack/decay → sustain, or
  decay-to-silence for plucks) and **each note selects its own instrument**. Data is the
  TIATracker model reduced clean-room: instrument `{AUDC, env offset, sustain}` over a flat `Env`
  table, parallel `Notes/Inst/Durs` patterns, looping song, `Env[0]=0` silence cell for rests.
  10 bytes of zero-page state; tick in overscan under TIM64T. Distilled from TIATracker
  (kylearan, forums.atariage.com/topic/250014 → `reference/atariage/250014-tiatracker/`).
  CI: `scenarios/music_driver.json` (envelope ramps 15→12→10→8 / 11→9→7, sustain holds, per-note
  instrument switch to pluck, pluck decay-to-silence with bass sustaining independently, song loop
  back to C5, 262 lines, audio golden). Hardware-calibrated via read_audio. = technique candidate ⑦.

### Mined (research, non-repo `reference/atariage/`, clean-room)
- AtariAge deep-dive run (depth-first, 11 threads distilled to `notes.ja.md`): TIATracker (⑦),
  fast-divide-by-seven (⑮), bus-stuffing (scope-out), raycasting (⑫ split a/b),
  48px-positioning (⑯), screen-resolution (constants cross-check), disassembling (dissect notes),
  castlevania-port (⑰), modular-kernel (⑱ RTS-stack dispatch), pointer-optimization,
  tiatracker-plus. Ledger updated; only ⑦ has been implemented+verified so far.

## [1.63.0] - 2026-06-12

### Added
- **Technique: room-based map navigation** (`roms/techniques/rpgmap.asm` +
  `docs/techniques/rpgmap.md`): the RPG/adventure backbone — a 2×2 world where each room is a
  wall table, the player walks (SWCHA + PosObject), and edge crossings transition rooms
  (`room ^= 1`/`^= 2` with wrap). Adding rooms is pure data. Distilled from za2600's
  kworld/rs/spr (`reference/2600-technique-sources/za2600/`, from the legacy ATARI AR folder).
  CI: `scenarios/rpgmap.json` (walk right→room 1, down→room 3, reflect, 262, golden).

### Note
- This completes the AR-folder technique trio (text24 ⑩ / hscroll ⑪ / rpgmap ⑬) studied from
  the recovered za2600 + sidescroll sources. Candidate ⑫ (raycasting) still lacks a source.
## [1.62.0] - 2026-06-12

### Added
- **Technique: horizontal playfield scroll** (`roms/techniques/hscroll.asm` +
  `docs/techniques/hscroll.md`): coarse 4px scroll via an 8-phase precomputed (PF0,PF1,PF2)
  table (PF bit-order quirks baked in), reflect mode, scrollSpeed-paced. Studied from the legacy
  ATARI AR Side-Scroll source. CI: `scenarios/hscroll.json` (phase progression, reflect, 262,
  golden); read_row confirms 4px-per-tick stripe motion.

## [1.61.0] - 2026-06-12

### Added
- **Technique: 24-character text line** (`roms/techniques/text24.asm` + `docs/techniques/text24.md`):
  doubles text12 to 24 chars by alternating two 12-char blocks across frames (left block P0=39,
  right block P0=87 = +48px contiguous) at 50% flicker. Studied from za2600's text24.asm
  (`reference/2600-technique-sources/za2600/`, recovered from the legacy ATARI AR folder).
  CI: `scenarios/text24.json` (block positions, packed buffers, 262, golden).

## [1.60.1] - 2026-06-12

### Fixed
- `aa_index.py` parser rewritten against the real IPB4 markup (verified on a live snapshot):
  title inside the nested span, `data-stattype` for replies/views, row split on the actual
  item class. One index page now yields ~49 clean topics (was 0-2 with polluted titles).

## [1.60.0] - 2026-06-12

### Added
- **`scripts/aa_index.py` (functional WIP)** — forum-wide topic catalog from Wayback index-page
  snapshots (title/author/replies/views CSV, views-sorted = digging-value ranking). The CDX
  enumeration and fetch loop work (50 archived index pages of the 2600 Programming forum);
  the IPB list parser only captures a fraction of rows and pollutes some titles — **parser
  iteration is the named next step** (recorded in reference/atariage/README.md).

## [1.59.0] - 2026-06-12

### Added
- **Technique: 48px bitmap zone with window scrolling** (`roms/techniques/bitmap48.asm` +
  `docs/techniques/bitmap48.md`): six bottom-up column tables + per-frame `ColK+offset`
  pointers = a logo/message band that scrolls through a taller bitmap (RevEng's Bitmap
  Minikernel idea, own implementation). Completes the 48px family: one verified choreography,
  three data feeds (digits / packed text / bitmap window). CI: offset animation incl. bounce,
  262, golden.

## [1.58.0] - 2026-06-12

### Added
- **Technique: 12-character text line** (`roms/techniques/text12.asm` +
  `docs/techniques/text12.md`): flicker-free text via the verified 48px 6-store choreography
  with a 4×5 font packed two characters per player byte (column-major zp buffer, strings
  pre-encoded as glyph indices). The catalog's biggest gap (menus/messages) closed at the
  sweet spot of the width ladder researched in the AtariAge 32-character thread (12 needs no
  RESP re-strobing and no flicker). CI: `scenarios/text12.json` (packed-buffer bytes, positions,
  262, golden). Wider variants (24 column-flicker / 32 interleaved) recorded as candidates with
  measured constraints.

## [1.57.1] - 2026-06-12

### Changed
- `aa_fetch.py` defaults to **lean storage**: raw HTML cache is deleted after parsing (Wayback
  itself is the permanent archive — re-fetchable anytime), attachments are listed in thread.md
  but only downloaded with `-attachments`, and `-keep-raw` opts back into caching. Keeps only
  the distillate (thread.md / gaps.md / notes). Demonstrated: a 2-page topic harvests to 80KB.

## [1.57.0] - 2026-06-12

### Added
- **`scripts/aa_fetch.py` — AtariAge thread-mining pipeline** (Wayback-first): the live forum
  sits behind a Cloudflare bot challenge, so the tool enumerates snapshots via the CDX API
  (both old/new domains), caches raw pages, parses IPB posts into a single `thread.md`
  (author/date/body), and recovers attachments (attachment.php redirects need
  status-filterless CDX + replay-URL following with retries). Gaps are reported for cookie/
  manual fallback (`AA_COOKIE` env supported; no passwords). First run: Medieval Mayhem topic —
  17/17 pages, 400 posts, dev-build ROMs recovered and analyzed with fieldtest/dissect
  (analysis artifacts stay in the non-repo reference/ area per the clean-room policy).

## [1.56.0] - 2026-06-12

### Changed
- Strengthening-run U wrap-up: summary section in `docs/improvement-roadmap.md` (P1-P4, 13
  harness releases + starshot v1.0 dogfood in the roms repo). Techniques catalog now covers the
  full real-game skeleton: score, SFX, sound driver, game states, bullets, paddle, procgen,
  bank template — every entry with a verified ROM + scenario + golden.

## [1.55.0] - 2026-06-12

### Added
- **`cmd/rammap`** (V2-18 closed): per-frame RAM diff over N frames → markdown usage map
  (address, change rate, value range, constant/per-frame hints). Feeds `docs/ram-maps.md` and
  audits our own ROMs' RAM budgets.
- **`scripts/check_gopher_pin.sh`** (F-2 closed): verifies the local Gopher2600 clone matches the
  CI-pinned SHA. Hardening-roadmap statuses updated (A-1/S-4/F-2/F-4/V2-18 all ✅).

## [1.54.0] - 2026-06-12

### Added
- **Stella oracle v2 — pixel compare** (`stellacheck -pixels` / `-snap`, F-4 closed): captures a
  Stella debugger `savesnap` PNG and compares it against Gopher2600's frame as TIA color codes,
  using a **measured Stella NTSC palette** (`internal/ingest/palette_stella.go`,
  `NewStellaNTSCQuantizer`) captured live from the new `litmus_palette.bin` (white marker + all
  128 colors, one per line). A shared quantizer misreads Stella's slightly-different RGB as
  ±1-luma errors (86.5%); with the measured palette: **100.00% agreement on litmus_pf
  (34,240 cells)**. `scripts/stella_oracle.sh <rom> <frames> pixels` runs it hands-free.

## [1.53.0] - 2026-06-12

### Added
- **Verification sweep — four documented-but-unverified facts closed** (`docs/fundamentals-audit.md`
  updated to ✅, each with a litmus + scenario):
  - `litmus_hmxx_freeze`: on Gopher2600, **HMxx is latched at the HMOVE strobe** — post-HMOVE
    rewrites (+6/+15/+33 cy) never alter in-flight movement. The 24-cycle rule stays as a
    real-hardware portability constraint.
  - `litmus_score_pfp`: **PFP dominates SCORE** — CTRLPF $06 renders identically to $04
    (PF in COLUPF on both halves, priority over players); SCORE coloring only without PFP.
  - `litmus_vdel_2lk`: the 2LK alignment relation pixel-exact — **VDELP0=1 shifts P0 +1 line**
    to align with odd-line-written P1 (read_row 137→138).
  - Shear-safe write window (cycles 0–22) closed by derivation from verified beam constants +
    litmus_48px6's measured mid-line choreography.

## [1.52.0] - 2026-06-12

### Added
- **`read_audio` note names** (A-1 closed): each channel now reports `note`/`cents` via
  `pkg/audio.NearestNote` — audio state is discussable by name ("ch0 is C5 +0.2¢"), not just
  raw AUDC/AUDF. Verified against the sound-driver ROM (C5/C4 exactly as composed).
- **Sprite shape in the annotated screenshot** (S-4 closed): `get_screen_annotated` draws the
  *current GRP bit pattern* (REFP-reflected, NUSIZ-width-scaled) at each player's marker
  position — mid-frame stops show exactly what byte the TIA is holding, cross-checked against
  `read_tia_registers.gfx_new` ($CC ⇒ the visible 2-2-2 pattern).

## [1.51.0] - 2026-06-12

### Added
- **Source-line debugging** (`internal/srcmap`, U-M9): `assemble_and_load` now assembles with
  DASM `-l`/`-s` and builds a PC → (nearest label + offset, source file:line) map. Tool outputs
  gain an `at` field: `assert_line_budget` (the overrunning code's location — e.g.
  `Burn+5 (litmus_overrun.asm:66)`), `trace_clocks` (every instruction), `watch_ram` (the
  writing instruction), `read_cpu` (current PC). `.bin` direct loads are unaffected (no map).
  Unit-tested parser + end-to-end coverage in `scripts/mcp_smoke.py` (overrun must report its
  source line). Flat 2K/4K only (banked ROMs return no `at`).

## [1.50.0] - 2026-06-12

### Added
- **Technique: bank-switched game structure** (`roms/techniques/banked_game.asm` +
  `docs/techniques/bankswitching.md`): the F8 template — per-bank reset stubs/vectors, a
  reusable `jsr $FF80` cross-bank trampoline, and the data-bank pattern (bank-1 loader copies
  level tables into zero page; bank-0 kernel renders from RAM). CI: `scenarios/banked_game.json`
  (load contents byte-exact, level switch, bank.number==0 at frame boundaries, golden).
  Recorded trap: **instruction fetch on $FFF8/$FFF9 switches banks** — placing the trampoline's
  `rts` on a hotspot caused a reboot loop (350-line frames); diagnosed via `watch_ram` writer PCs.

## [1.49.0] - 2026-06-12

### Added
- **Technique: procedural generation** (`roms/techniques/procgen_demo.asm` +
  `docs/techniques/procedural.md`): event-driven Galois LFSR (the litmus_lfsr form) mapped to
  spawn positions by mask+offset, with the sequence cross-checked against an off-target
  reference implementation. CI: `scenarios/procgen_demo.json` — four spawns assert RAM state
  AND rendered X exactly ($5A → $2D,$98,$4C,$26 / X 61,40,92,54), golden. Same seed = same world.

## [1.48.0] - 2026-06-12

### Added
- **Technique: paddle input** (`roms/techniques/paddle_demo.asm` + `docs/techniques/paddle.md`):
  the dump/charge/per-line-count kernel (VBLANK=$82 discharge → release at visible start →
  count lines until INPT0 D7) with the value mapped to a PosObject-placed bar. CI:
  `scenarios/paddle_demo.json` — paddle 0.1/0.25/0.5 measure exactly 0/63/170 lines (litmus
  transfer curve, shifted by the dump-release line) and the bar X follows (clamped), golden.

## [1.47.0] - 2026-06-12

### Added
- **Technique: missiles as bullets** (`roms/techniques/bullets.asm` +
  `docs/techniques/missiles-bullets.md`): RESMP spawn-at-player, sentinel-encoded row-range
  flight (kernel stays under the line budget on the active path), CXM0P hit handling.
  CI: `scenarios/bullets.json` (spawn at ship+4, flight, latch, hit bookkeeping, golden).
- **`litmus_resmp` — RESMP verified**: unlock places the missile at **player+4px** (1x center),
  follows HMOVE moves, and the lock must be **held ≥1 frame** (same-pass lock+unlock does not
  move the missile). Plus three recorded traps: collision *read* addresses decode the low nibble
  ($32 reads CXP0FB, not CXM0P=$30); PosObject fine adjust is `eor #7` (not `eor #$FF`); active-
  path-only line-budget overruns show up as frame-length changes (350-line frames).

## [1.46.0] - 2026-06-12

### Added
- **Technique: game state machine** (`roms/techniques/game_states.asm` +
  `docs/techniques/game-states.md`): title/play/game-over skeleton with edge-detected console
  switches, SELECT variants, difficulty-dependent round timing, attract mode, deterministic
  state entry, frame logic under TIM64T. CI: full-lifecycle scenario (~1100 frames, golden).
  Dogfooded: `fieldtest -auto` detects this ROM's title via `auto-start: reset`.
- **`litmus_swchb` — SWCHB read side verified** (D0/D1 active-low, D3 color, D6/D7 difficulty):
  `emu.SetPanel` extended with `color`/`p0pro`/`p1pro`, and scenario `inputs[]` now accepts panel
  actions (`reset`/`select`/`color`/`p0pro`/`p1pro`). `docs/fundamentals-audit.md` input section
  updated to verified.

## [1.45.0] - 2026-06-12

### Added
- **Technique: in-game sound driver** (`roms/techniques/sound_driver.asm` +
  `docs/techniques/sound-driver.md`): looping 2-voice music from jingle-compatible tables with
  **SFX preemption of channel 1 and automatic restore**; driver tick runs in overscan under
  TIM64T (constant calibrated by scenario line-count sweep). Verified by `dissect -audio`
  round-trip (transcription == composition on both voices) and frame-exact preemption/restore
  asserts. CI: `scenarios/sound_driver.json` (+ audio golden).

## [1.44.0] - 2026-06-12

### Added
- **Technique: sound effects** (`roms/techniques/sfx_demo.asm` + `docs/techniques/sound-effects.md`):
  SFX as frame tables (2 bytes/frame) generated by new `pkg/audio` helpers `PitchSweep` /
  `NoiseBurst` / `Blip` / `Arpeggio` / `EmitSFX` (unit-tested). Five standard recipes (laser,
  explosion, pickup, bounce, engine) + a ~40-cycle overscan player. CI: `scenarios/sfx_demo.json`
  — 14 register-exact asserts across all five effects (all passed first run) + audio-digest golden.

## [1.43.0] - 2026-06-12

### Added
- **Technique: 6-digit score kernel** (`roms/techniques/score6.asm` + `docs/techniques/score-kernel.md`):
  BCD 3-byte score + per-frame font-pointer build + the litmus_48px6 VDEL 6-store choreography with
  `(zp),y` fetches (stores at 55/58/61/64 cy → whole block repositioned +63px to P0=87/P1=95; gap
  relations preserved). `pkg/sprite.DigitFont()` for Go-side reuse. CI: `scenarios/score6.json`
  (positions, BCD carry at frames 99/150, 262 lines, golden).

## [1.42.0] - 2026-06-12

### Added
- **Music transcription** (`cmd/dissect -audio N`): samples TIA audio registers (AUDC/AUDF/AUDV,
  both channels) at frame granularity from reset and emits each channel as jingle notation
  ("D6:80 F6:40 R:6 ..."), with per-note AUDF/cents. New `pkg/audio.NearestNote` (12-TET inverse
  of `FindNote`, unit-tested). **Round-trip verified**: transcribing our own single- and two-voice
  fanfare ROMs reproduces the input melodies note-for-note on both channels (repeated equal
  pitches merge legato — register-identical, acoustically the same). Demo: a commercial title's
  theme transcribed with names + frame durations (output kept in inbox per clean-room policy).

## [1.41.0] - 2026-06-12

### Added
- **`cmd/dissect` bank-aware matching (F8/F6/F4)**: for carts >4K, matches are reported as
  "bank N $Fxxx-$Fxxx" (bank-relative in the $F000-$FFFF window) instead of a wrong flat address.
  Ground-truth verified with a purpose-built F8 ROM (Art table planted in bank 1 at $F200 →
  reported exactly as "bank 1 $F200-$F207"); field-checked on a commercial 8K title (asset tables
  resolved per bank, computed wireframe data correctly left unmatched). DiStella annotation is
  skipped with a note for banked carts (DiStella v2.10 supports 2K/4K only).

## [1.40.0] - 2026-06-12

### Changed
- **All generated output is now English**: ingest text reports (`internal/ingest/textreport.go`),
  fieldtest/dissect/stellacheck CLI messages, and jingle-generated ASM comments. Go source comments
  stay as-is (repo convention); only user-visible output strings changed. Existing inbox artifacts
  were regenerated/rewritten in English (reports, summaries, READMEs).

## [1.39.0] - 2026-06-12

### Added
- **`cmd/jingle` two-voice support** (`-notes2`/`-vol2`/`-type2`): both TIA channels driven
  independently (AUDC1/AUDF1/AUDV1, per-voice auto-picked sound type, automatic rest padding for
  loop sync). Verified numerically via `read_audio`: both channels sound the expected harmony
  pair (e.g. F6/A5) at the expected frames. Generated-ASM comments and CLI output are English.

## [1.38.0] - 2026-06-12

### Added
- **`cmd/dissect` — runtime trace × ROM byte matching** (disassembly-driven asset extraction; the
  preferred path when the ROM exists, superseding pixel analysis): instruction-steps N frames recording
  every TIA graphics-register store (GRP/PF/COLU) with PC + scanline, groups them into streams, and
  locates each table's **ROM address** (trying trimmed-blank / run-length-collapsed / reversed variants),
  rendering sprites as ASCII art. Constant streams are reported as immediates (false-positive guard).
  `-distella` merges `; dissect:` annotations into a DiStella disassembly at the nearest preceding label.
  Validated on ground truth (vertical_pos art table found at its exact address) and on a commercial
  title (player sprite incl. reversed storage + per-row color table + PF table; output kept local per
  the clean-room policy). Research notes + future ideas: `docs/improvement-roadmap.md`.
- `internal/emu`: CPU register accessors `PC`/`A`/`XReg`/`YReg` and `PeekROM` (memory peek without
  side effects) to support instruction-level tracing.

## [1.37.0] - 2026-06-12

### Added
- **fieldtest v2**: console panel switches (`emu.SetPanel` reset/select; `-press reset@30`),
  **auto-start escalation** (`-auto`: capture → if no dynamic objects, RESET → fire →
  fire+hold-right, reporting which attempt started the game — verified live: E.T. needed RESET,
  Outlaw needed fire+hold-right), and **inbox organize mode** (`-inbox dir`: each X.bin moves
  into X/ with overlay/report.txt/report.json inside — the standing structure, documented in
  inbox/README.txt). Batch-ran 9 ROMs end-to-end.

## [1.36.0] - 2026-06-12

### Added
- Recovery-run wrap-up: routing table entries (ram-maps, dynamic-multisprite), mcp_smoke now
  exercises all five new tools end-to-end, serverInfo version bump added to the release
  checklist in CLAUDE.md, open-backlog ledger CLEARED (remaining items are single user
  actions, each fully prepared). Summary at inbox/recovery_report.txt.

## [1.35.0] - 2026-06-12

### Added
- **Composing-session groundwork**: `pkg/audio.NoteFreq/FindNote` (12-TET note names →
  best (AUDC,AUDF) with cents error, Slocum tuning) and **`cmd/jingle`** — melody notation
  (`"C5:30 E5:30 G5:30 C6:60 R:30"`) → a playable looping ROM in one command (auto-picks the
  sound type that fits the whole melody within ±60 cents; assembles via dasm when present;
  per-note cents annotated in the generated source). Verified: register sequence
  AUDF 29→23→19→14 matches the documented C6 spot value; 262 lines held. The joint session is
  now "hum it → ROM in 30 seconds → listen together in Stella".

## [1.34.0] - 2026-06-12

### Added
- **`cmd/fieldtest` — ROM self-driving field tests (input contract v3).** Given a ROM file, the
  harness runs it in Gopher2600, captures K frames (with optional input injection
  `-press right@60,fire@90`), and emits the full multi-frame analysis (overlay/report.txt/json).
  Screenshots are no longer required when a ROM exists — F12 becomes the fallback. Verified
  end-to-end on dyn_multisprite (4 frames, fidelity ~100%).

## [1.33.0] - 2026-06-12

### Added
- **`scripts/stella_oracle.sh` — the Stella cross-check, hands-free.** Launches stellacheck and
  sends the debugger key to Stella via AppleScript in parallel; preflights the one-time
  Accessibility permission and prints setup instructions when missing (the manual-keypress flow
  remains as fallback). The last human step in the oracle loop is now a single one-time
  permission grant.

## [1.32.0] - 2026-06-12

### Added
- **MCP `trace_clocks`** — sub-instruction beam anatomy: each of the next N instructions with
  PC, opcode, CPU cycles, and start/end (scanline, color clock). The practical recovery of the
  parked step_clock (observation without suspension). **First catch:** the mid-line HMOVE
  table's strobe clocks were hand-estimates (≈1/73/130); trace_clocks measured 13/85/142 —
  fundamentals-audit corrected. Rule 2 extended to clocks.

## [1.31.0] - 2026-06-12

### Added
- **Ingest R3 — mid-scanline COLUPF as a first-class citizen.** Bands whose lit columns change
  color mid-half now carry `color_writes` ([{clock,color}] — faithful timed-write register
  semantics, exactly how you'd author it), the renderer replays them, and the text report prints
  them as `; COLUPF timed write: clock N -> $XX`. The previously "documented limit" is now
  modeled: **Pitfall's static layer 98.56% → 99.90%** (8 bands gained writes). Synthetic CI
  proof: a two-color half extracts write@clock48 with fidelity 100%. Half-boundary-only changes
  (score mode) still use ColorLeft/Right — no churn for existing data. inbox reports regenerated.

## [1.30.0] - 2026-06-12

### Changed
- **dyn_multisprite polish**: all five objects now have distinct X (DelTbl 1..5 — enabled by a
  −2-cycle state-flag dispatch: draw state = $80 so one `bmi` replaces cmp/beq); the documented
  position mapping now matches measurement (X = 33+15d on slot A, 36+15d on slot B; the 3px
  slot difference is the A/B dispatch asymmetry, now documented); scenario asserts strengthened
  with deterministic ys at two fixed frames; goldens regenerated.

## [1.29.0] - 2026-06-12

### Fixed
- **Exerciser scene-entry line transients eradicated** (debt since v1.2.0): title entry 263
  (music init moved into the half-empty HMCLR line), zone entry 264 (the 6-element X-table
  copies ran ~82 cycles — split 3+3 across the init's six lines), gradient entry 263 + a 263
  every 4th frame (the kick envelope's every-4th-frame branch jitter — now a branchless
  per-frame `AUDV0 = sfxTmr>>2` with identical envelope, and the entry-frame kick register
  writes moved past the first WSYNC, flagged by sfxTmr==40). **All five scenes now hold 262
  on every frame including entry** (full per-scene map probed). Goldens regenerated.

## [1.28.0] - 2026-06-12

### Added
- **V2-18 RAM-map audit** — `docs/ram-maps.md`, auto-extracted zero-page equates per ROM.
- CLAUDE.md tool list updated (analyze_screen / run_scenario / watch_ram; parked items noted);
  MCP serverInfo now tracks releases (1.28.0). Open-backlog ledger updated with v1.19–v1.28
  results. Overnight summary at `inbox/overnight_report.txt`.

## [1.27.0] - 2026-06-12

### Decided
- **Ingest M-I — static-layer residual diagnosed and documented, not papered over.** Pitfall's
  98.6% static reconstruction loses its 1.4% in canopy-fringe rows where two colors share one
  playfield half on one scanline = the game writes COLUPF mid-scanline. The band model keeps
  one color per half on purpose (per-column colors would misrepresent register semantics);
  the documented guidance is to author such rows as timed-write kernels. Diff-row histogram
  methodology recorded in docs/ingest.md.

## [1.26.0] - 2026-06-12

### Added
- **Ingest M-H — position-continuity union tracks + animated-PF hints.** The union links
  sprites across frames by proximity (≤20px; Pitfall's Harry runs up to 18px/frame) and shared
  colors — an animating mover is now ONE track with a `poses` count (Harry: 1 track, 4 poses,
  not flicker). `flicker` is redefined to "blinking in place across skipped frames" only
  (the four flicker balls keep their flag; vanished/appeared tracks count as gaps). Fully
  grid-aligned dynamic cells get an `animated_pf?` hint — the Exerciser's scrolling starfield
  is CI ground truth (mountains stay static reflect bands).

## [1.25.0] - 2026-06-12

### Added
- **Technique #10b — dynamic multi-sprite kernel, the full form** (`dynamic-multisprite.md`,
  demo `dyn_multisprite.asm`; suite now 50). 5 crossing objects through 2 players: 9-comparator
  sorting network (deterministic cycles), dynamic 2-of-N slot queues with 0-sentinels and
  per-frame fairness flip, mid-screen timed-RESP repositioning on the coarse grid, and a
  **TIM64T-managed VBLANK** (sort+assign vary 60–160 cycles by path — un-paddable; the
  real-game idiom now verified here). Zero visible budget spills over 10 frames by
  instruction-level interval enumeration; all 5 object colors proven rendered via multi-frame
  ingest. War stories recorded: a POSITION path at exactly 76 cycles (the closing WSYNC itself
  crossed) fixed by a fall-through reorder worth −3 cycles.

## [1.24.0] - 2026-06-12

### Added
- **VDEL odd/even verified** — `two_line_vdel.asm`: in a 2-line kernel (GRP0 on line A, GRP1 on
  line B), setting `VDELP0 = y&1` parks the GRP0 write in the shadow register until the GRP1
  write — the sprite starts on odd scanlines with the kernel unmodified. CI pixel proof: top
  edge moves exactly +1 scanline per frame (TestVDELOddEven). Suite now 49.

## [1.23.0] - 2026-06-12

### Added
- **skipDraw (DCP) verified** — `vertical_pos_dcp.asm`: the classic undocumented-opcode vertical
  trigger (`lda #H-1 / DCP sprDraw / bcs`), encoded via `.byte $C7` (DASM has no illegal
  mnemonics). Measured against the compare version on the same kernel: max line 40→38 cycles,
  sprite line 31→30 — modest here; the idiom's real value is freeing Y. Pixel-identical motion,
  CI-locked. Suite now 48.

## [1.22.0] - 2026-06-12

### Added
- **litmus_hmove_mid — mid-line HMOVE measured** (documented→verified). With HM registers
  cleared, strobes completing at visible clocks ≈1 and ≈73 shift nothing; ≈130 shifts **−5 px
  left**; no-strobe control 0. Pixel-confirmed (bar edge above/below the strobe line). The folk
  "right 1px/4CLK" summary did not reproduce — recorded as a non-monotonic function of strobe
  time in docs/fundamentals-audit.md; pinned in scenarios/hmove_mid.json. Suite now 47.

## [1.21.0] - 2026-06-12

### Added
- **litmus_bank_f6 / litmus_bank_f4 — F6 (16K/4-bank) and F4 (32K/8-bank) bankswitching
  hardware-verified** (generalizing the proven F8 pattern: vectors + identical reset stub in
  every bank, a byte-exact switch-zone chain at $FF00 visiting bank0→1→…→N→0 each frame).
  Each bank stamps its ID and counter; scenarios assert the last bank's mark, equal counters,
  and bank.number==0 at the frame boundary. Suite now 46. The F4 chain (~130 cycles) spills one
  overscan line — compensated explicitly (ldx #29) to keep 262.
- CLAUDE.md: bank constants note updated (F8/F6/F4 all verified).

## [1.20.0] - 2026-06-12

### Added
- **MCP `watch_ram`** — run until RAM[addr] changes; returns old/new value and the PC of the
  writing instruction (bounded by max_frames). Granularity is per-instruction; same-value
  stores are invisible (documented).

### Decided
- **step_clock parked with findings** (docs/mcp-tools.md): Gopher2600's colorClockCallback can
  observe but not suspend mid-instruction; a color-clock quantum needs an upstream CPU
  micro-instruction refactor. RunUntilBeam/read_cycles/assert_line_budget/watch_ram cover the
  practical cases.

## [1.19.0] - 2026-06-12

### Added
- **MCP `run_scenario`** — the regression runner's verdict callable from the live loop
  (paths[], returns pass/fail with failing-assertion details).
- **MCP `analyze_screen`** — the ingest analyzer applied to the *current emulator frame*
  (no file round-trip): PF bytes, sprite GRP + per-row colors, groups, fidelity, grid overlay.
  Supersedes the long-parked read_sprite_shape idea.
- `scripts/mcp_smoke.py` — sequential MCP smoke driver (the go-sdk serves tool calls
  concurrently; piping a batch races load_rom vs later calls — cost one debugging round).

## [1.18.0] - 2026-06-12

### Added
- **`report.txt` — the human-readable report is now an official tool output** (the author asked
  why the nice ASCII format was one-off). `cmd/ingest` writes it next to `report.json`/
  `overlay.png`: sprite ASCII art with per-row TIA color codes (duplicate rows compressed xN,
  NUSIZ stretch expanded), group list, playfield band table with 40-column previews and
  repaired/SCORE flags, and the DASM snippets. Multi-frame runs get the layered version
  (per-frame dynamic sprites + union + static layer).

## [1.17.0] - 2026-06-12

### Added
- **Image ingestion M9 — multi-frame everywhere.** `cmd/ingest -in a.png,b.png,c.png` and
  `analyze_image {paths: [...]}` run the M8 separation end-to-end; static objects carry
  interpretation hints (`pf_fringe?` when the color matches an adjacent PF band,
  `parked_object?` otherwise); input contract v2 documented (2-3 consecutive F12 shots for
  scenes with movement; N=3 recommended). MultiReport uses a named `static` field (Go embedded
  structs and the MCP schema generator don't mix — second schema gotcha after []uint8).

## [1.16.0] - 2026-06-12

### Added
- **Image ingestion M8 — multi-frame separation** (the author's architectural point: M7's
  reference-pattern repair doesn't generalize; this does). Feed N screenshots of the same scene:
  per-pixel voting builds the **static layer** (playfield/background/parked objects — leaf
  fringes, pit holes, ladders land here correctly as `static_*`), per-frame diffs give the
  **dynamic layer** (real sprites). No repeating-structure assumption. Bonus: **union across
  frames with flicker detection** — 30 Hz multiplexed objects read completely from 2 shots.
  N=2 ties fill from row background (recorded in `unresolved_share`); N=3 recommended.
- CI proofs from our own ROMs: flicker_multiplex 2 frames → all 4 balls in the union, each
  flagged flicker, per-frame fidelity 100%; sprite_anim → walker tracked moving +1px/frame, not
  misflagged; pf_modes static scene → bands identical to single-frame analysis, dynamic layer
  empty, unresolved 0.

## [1.15.0] - 2026-06-12

### Added
- **Image ingestion M7 — overlap repair (sprite-guided PF inpainting).** Where sprites cross
  playfield, ownership is locally undecidable; a clean reference band (the same structure
  repeating elsewhere) resolves it both ways: sprite pixels absorbed into PF return to the
  sprite's art, PF bits hidden under the sprite restore from the reference. Conservative: no
  reference → no touch. Synthetic CI proof: a frame sprite over a 3-cycle building pattern
  extracts bit-perfect with all bands repaired and fidelity 1.0.

### Fixed
- Context demotion (M6) demoted whole thin bands, dragging clean columns into the sprite layer
  (caught by the synthetic overlap test) — now **per-column** with per-column color matching.

### Result
- Pizza Boy: **fidelity 100.0%** (from 99.93%), zero contaminated/asymmetric bands; the pizza
  slice's body rows and the courier's belt row recovered exactly (author's two remaining
  complaints). All sprite/PF colors were already real TIA codes (COLUxx values) per row/band.

## [1.14.1] - 2026-06-12

### Fixed
- **annotate grid drew pink artifacts over bright backgrounds** — a latent bug since v0.5.0:
  the semi-transparent grid colors were invalid premultiplied `color.RGBA` values
  (channels > alpha, e.g. {255,255,255,30}); harmless over black (most 2600 screens) but the
  compositor produced pink streaks over bright areas (visible on Pizza Boy's cyan buildings;
  in hindsight also faint on the zone scene). Grid lines now use non-premultiplied
  `dc.SetRGBA`. Affects `get_screen_annotated` and all ingest overlays.

## [1.14.0] - 2026-06-12

### Added
- **Image ingestion M6 — context arbitration, stretch decomposition, grouping.** Thin "playfield"
  rows vertically touching same-colored sprite pixels are sprite strokes (the score digits'
  top/bottom bars) — they demote and the rings reassemble whole (synthetic 3-ring CI test).
  Components 9-16/17-32 px wide try NUSIZ 2x/4x hypotheses (≥90% row conformance) before
  empty-column splitting and 8px-window composites — everything gets GRP data now. Row-groups
  bundle score/gauge runs; identical shapes share an id. Overlay draws numbered bounding boxes.
- **Pizza Boy acceptance (author's checklist): all six criteria met.** Courier = one complete
  sprite (detached hand re-merged; 10 art rows × 2-line kernel), life gauge = one 3-copy entry,
  pizza = standalone sprite, **both cabs = player_2x with identical shape id (GRP'd)**, score =
  one row-group of complete digits, **fidelity 99.93%** (own-ROM suites stay at 100%).

## [1.13.0] - 2026-06-12

### Added
- **Image ingestion M5 — fidelity metric + fragment merging** (author feedback: "if the accuracy
  is too low to use, it's pointless" — so accuracy became a number first).
  - **Reconstruction fidelity**: the report (per-row background + PF bands + sprites) renders
    back to a 160×H plane and is pixel-compared with the normalized input; `fidelity` is in
    every report. CI asserts **100% on our own ROMs** (an extractor that can't reconstruct its
    own renderer's output is buggy); pf_modes allows 0.999 (sprite-over-PF assumption vs the
    priority region).
  - **Fragment merging**: connected components within a 2px gap sharing colors fuse before
    classification (the courier's detached hand, the cab's wheel, multi-part icons). Pizza Boy:
    16 components → 6 objects, fidelity **99.25%** (the remainder is exactly the still-GRP-less
    large objects = M6's job).

## [1.12.0] - 2026-06-12

### Added
- **Image ingestion M4 — `analyze_image` MCP tool.** The full pipeline (normalize → quantize →
  playfield bands → sprite candidates → DASM snippets) callable live; returns the structured
  report plus the TIA-grid overlay inline and at `$ATARI2600_INGEST_PATH`. Found and fixed a
  go-sdk structured-output gotcha: `[]uint8` marshals as base64 (Go `[]byte`) and fails the
  generated array schema — byte sequences in tool outputs are `[]int` now.
- docs/ingest.md (+ja) extended with the extraction layers and MCP usage; README section;
  CLAUDE.md routing + tool list. MCP serverInfo.version now 1.12.0.
- Field test: Pizza Boy F12 shot → 29 playfield bands + 16 sprite candidates end-to-end through
  the MCP tool (full report delivered to the author separately).

## [1.11.0] - 2026-06-12

### Added
- **Image ingestion M3 — sprite extraction.** 8-connected components over the residual layer
  (non-background, non-playfield) classified as player (width ≤8: GRP bytes in pkg/sprite bit
  order + per-row color table), missile/ball (≤4 solid), or large_object (low confidence);
  equal-shape groups at 16/32/64 spacing fold into one NUSIZ entry. DASM GRP tables emitted.
- **PF↔sprite reconciliation:** a grid-aligned sprite (the bouncing ball at x=80) was claimed by
  the playfield layer and fragmented — tiny PF bands (height ≤2, lit columns ≤2) now demote back
  to the sprite layer. Genuine 1-line playfield (starfields) survives via column count.
- Round-trip CI proofs: ball GRP == Art bit-for-bit with canonical colors; walker GRP matches
  phase art through the row-quadrupled kernel (32 rows); litmus_nusiz_copies folds to one
  3-copy/16-spacing entry.

## [1.10.0] - 2026-06-12

### Added
- **Image ingestion M2 — playfield extraction.** Per-row background estimation (global mode color
  with per-row fallback for COLUBK gradients — naive per-row mode inverted figure/ground on rows
  more than half-filled, caught by the mountain round-trip), 4-clock-aligned column folding,
  repeat/reflect/asymmetric half classification, score-mode flagging (same pattern, two colors),
  band compression, and DASM `byte` table emission reusing `pkg/playfield`'s verified bit order.
- Round-trip CI proofs: litmus_pf bands == $10/$80/$01 exactly; pf_modes score band ($66,
  $44-left/$86-right) and wall band ($10) found; Exerciser mountain bands match the live RAM
  band triples (PF0 masked to its displayed upper nibble) with reflect detected.
- Palette canonicalization: codes with identical RGB (e.g. $0C≡$0E here) report as the lowest
  code (`Quantizer.Canonical`).
- CI now assembles roms/techniques + roms/exerciser before `go test` (ingest tests use them as
  ground truth).
- Field result: Pizza Boy buildings extract as repeat-mode PF bands (blue $9E) with concrete
  PF0/PF1/PF2 bytes per 4-line band.

## [1.9.0] - 2026-06-12

### Added
- **Image ingestion M1 — screenshot → TIA raster** (`internal/ingest`, `cmd/ingest`,
  `docs/ingest.md`). The reverse pipeline begins: integer-scale auto-detection (any multiple of
  the 160-clock raster — decided with the author; 320×228 Stella F12 → 2×1), cell-majority
  normalization, palette quantization against the same Gopher2600 `Spec.GetColor` table the
  harness renders with (distance reported; Stella inputs show the expected small constant),
  TIA-coordinate grid overlay reusing `internal/annotate`. Round-trip CI tests: an emulator
  Snapshot upscaled 2×1/2×2 normalizes back **pixel-identical** with distance 0.
- **Image input contract** (docs/ingest.md + CLAUDE.md): grade A = Stella F12 PNG, unmodified,
  TV effects off (integer scale guaranteed, Retina-proof); OS screenshots = conversation grade,
  processed with warnings; hand-off point = umbrella `inbox/` (belongs to no repo).
- Real-image smoke test: Pizza Boy F12 shot → scale 2×1 detected, full color inventory
  (bg $00 79%, buildings $9E, score $FE, courier $CE, …).

## [1.8.0] - 2026-06-12

### Added
- **Technique #12 — Venetian Blinds** (`docs/techniques/venetian-blinds.md`, demo
  `roms/techniques/venetian.asm`, CI-locked; suite now 44). Intra-frame line interleaving: a white
  diamond and a red frame coexist in one 64-line zone through P0 alone — even lines draw A, odd
  lines B, shape *and* color swapped per line before the display window. Zero flicker (60 Hz
  stable), striped look — the Video Chess (Whitehead, 1979) technique. Adjacent rows pixel-verified
  (`[83+2 white]` ↔ `[80+8 red]`).

### Milestone
- **Techniques roadmap complete: 12 of 12 verified.** #1 zones, #2 animation, #3 vertical
  positioning, #4 2-line kernel, #5 48px+score, #6 sound driver, #7 LFSR, #8 PF modes,
  #9 ball+missiles, #10 flicker multiplexing, #11 F8 bank switching, #12 Venetian Blinds —
  each with a CI-locked demo or verified inside the Exerciser. Documented refinements (VDEL
  odd/even, dynamic Y-sort allocation, DCP skipDraw, F6+) remain on call for real games.

## [1.7.0] - 2026-06-12

### Added
- **Technique #10 — flicker multiplexing** (`docs/techniques/flicker-multiplexing.md`, demo
  `roms/techniques/flicker_multiplex.asm`, CI-locked; suite now 43). Four color-coded bouncing
  balls share two players by frame-parity subset rotation (30 Hz each) — the Pac-Man-ghost
  technique; overlap-safe since slots use the any-Y compare kernel (#3 ×2, ~49 cy/line) with
  per-subset colors and one shared HMOVE. **The alternation itself is CI-asserted** across three
  consecutive frames. The full dynamic form (Y-sort + 2-of-N allocation + fairness rotation)
  is documented for when a game needs it.

## [1.6.0] - 2026-06-12

### Added
- **Technique #8 completed — playfield score mode & priority** (`docs/techniques/pf-modes.md`,
  demo `roms/techniques/pf_modes.asm`, CI-locked; suite now 42). Three regions switch CTRLPF
  mid-frame; pixel-verified by read_row: in score mode the same PF1=$66 pattern reads back
  COLUP0-red on the left half and COLUP1-blue on the right; with priority off the red P0 column
  fully covers the yellow wall, with D2 set the wall splits the sprite (62+2/64+4/68+2).
  Together with the already-verified asymmetric PF and reflect, #8 is done.

## [1.5.0] - 2026-06-12

### Added
- **Technique #4 — 2-line kernel** (`docs/techniques/two-line-kernel.md`, demo
  `roms/techniques/two_line_kernel.asm`, CI-locked; suite now 41). Each art row spans two
  scanlines; line A carries P0's vertical compare + a COLUBK gradient, line B carries P1 +
  loop control — the standard headroom structure of real games. Two players staged then moved
  by **one shared HMOVE** (strobing per positioning line re-applies the earlier HMxx — a +3 px
  bug caught by read_tia and documented). Carry-hygiene note: an `adc` inheriting the sprite
  compare's flags jittered the gradient until it became an `ora`. VDEL odd/even (1-px vertical
  granularity inside a 2LK) left documented-only.

## [1.4.0] - 2026-06-12

### Added
- **Technique #3 — Vertical positioning** (`docs/techniques/vertical-positioning.md`, demo
  `roms/techniques/vertical_pos.asm`, CI-locked; suite now 40). Vertical has no hardware — the
  kernel compares `line − sprY` against the sprite height every scanline and feeds GRP0 art or
  zero (single unsigned `cmp` covers above *and* below via underflow; both paths converge on one
  store at ~21 cy). Demo bounces a ball Y 4⇔180 at X=80; pixel rows verified **bit-for-bit**
  against the art via `read_row`. DCP/skipDraw variant documented for cycle-starved kernels.
  Re-confirmed: **position calibration is kernel-specific** (`lda #imm` vs `lda zp` prologue =
  1 cy = 3 px; this ROM's XCAL is −5 where sprite_anim's is −8) — never copy constants, re-measure.

### Fixed
- **`read_row` y-coordinate was off by `visibleTop` (~29 lines)** from the annotated-grid labels
  the tool promises to match (grid = `visibleTop + image row`; the implementation indexed the
  cropped image directly). Static playfield checks were self-consistent, but grid-coordinate
  round-trips missed. `ReadRow` now subtracts `visibleTop` — the y you see on the grid is the y
  you pass. Found while pixel-verifying this technique's demo.
- MCP server `serverInfo.version` was stuck at "0.9.0"; now tracks releases (1.4.0).

## [1.3.0] - 2026-06-11

### Added
- **Technique #2 — Sprite animation** (`docs/techniques/sprite-animation.md`, demo
  `roms/techniques/sprite_anim.asm`, CI-locked by `scenarios/sprite_anim.json` + golden; suite now 39).
  4-phase walk cycle (frame-divided clock, `frameBase` staged in VBLANK, row-quadrupled kernel),
  ping-pong X with **free REFP0 horizontal flip** (asymmetric art so the flip reads), divide-by-15 +
  HMOVE-table positioner **calibrated to `pos(v) = v` exactly** (`XCAL=-8`, organic full-range sweep).
  Documented measurement subtlety: frame-boundary `hmoved_pixel` reads lag one frame (xpos∓1 by
  direction) — observation artifact, not a positioning error; and **calibrate with organic runs, not
  pokes** (poke timing vs frame-boundary anatomy mis-measured ±2 px twice).

### Changed
- `docs/techniques/roadmap.md` synced with reality: the Exerciser had already verified **#5 48px+score**,
  **#6 sound/music driver**, **#7 LFSR**, **#9 ball+missiles**, **#11 bank switching (F8)** (and parts of
  #8; VDEL prereq of #3 now ✅) — 7 of 12 techniques done. Next open items: #3 vertical positioning,
  #4 2-line kernel, #10 general multi-sprite kernel.

## [1.2.0] - 2026-06-11

### Changed
- **Exerciser Procedural scene redesigned: starfield over mountains** (author feedback: the old
  fixed-mask output "looks like a scrolling barcode" — the one-byte-seed magic wasn't visible).
  - Top 111 lines: sparse starfield — draw = (pair of LFSR steps ANDed) & previous line's pair
    (~6% density, any column), scrolling every frame. The old `and #$88/$11` masks confined stars
    to four fixed columns, which is what read as barcode.
  - Bottom 80 lines: a mirrored mountain ridge generated at scene entry from a one-byte seed by an
    AND-cascade (`band[b] = band[b+1] & (r1|r2)`, 10 bands of 8 lines; harsher `r1&r2` masks for the
    top bands, and the top two bands forced empty — consecutive LFSR steps are correlated, which
    otherwise lets a lucky column survive to the ceiling as a tower). Zero picture bytes in ROM.
  - The scene now owns all 192 scanlines explicitly (1+111+80). The old version only strobed 191
    WSYNCs and silently relied on the dispatch line spilling past 76 cycles for the 262 total; the
    rewrite's lighter pre-section broke that assumption (261 lines) before being caught by the
    line-count probe. Generation is spread across entry-frame lines (≤75 cycles each, one extra
    cycle over budget in an early draft was caught by the per-frame probe and moved to its own line).
- docs/exerciser.md: scene-4 row rewritten accordingly. 38 scenarios pass; goldens regenerated.

## [1.1.0] - 2026-06-11

### Changed
- **Exerciser polish from the author's play-test (three QA reports, all confirmed and fixed).**
  1. *Title logo & score were left of center* — the 48px blocks sat at the verified-recipe default (X=24).
     Now centered (P0=56/P1=64), which required **recalibrating the six-store choreography for the new
     display window** (timed stores 44/47/50/53 instead of 34/37/40/43) and rebalancing the kernel: B0/B1
     loads moved into the head, the tail slimmed to `dec row` + B5 staging, and the exit-line cleanups moved
     after their closing WSYNC (the combined exit line ran 77 cycles and spilled a scanline — caught by the
     line-budget probe).
  2. *Zone sprites never reached the right edge* — the drift wrap was `and #$7F` (0–127), inherited from the
     techniques demo. Now wraps properly at 0–159 (full width), with the drift loop re-split two zones per
     line to stay inside the 76-cycle budget.
  3. *The starfield's "reorganize every 64 frames" read as nothing happening* — one LFSR step per second
     only shifted the pattern a single line. The seed now advances every frame: a continuous upward-scrolling
     starfield. 38 scenarios pass; goldens regenerated.

## [1.0.1] - 2026-06-11

### Fixed
- **Exerciser: fire/scene-advance was dead in Stella — paddle scene removed.** Field report (the author,
  playing in Stella): Space did nothing, though it worked before M5 and every Gopher2600 scenario passes,
  including a real-user input-pattern probe. Root cause: **Stella's controller auto-detection** sees the
  ROM's INPT0 reads (the paddle scene), plugs paddles into the left port — and plugged paddles **hold INPT4
  permanently high**, so the joystick fire can never register (the property is also persisted per-ROM,
  which is why `-lc JOYSTICK` didn't rescue the first binary). Per the author's call, the paddle scene is
  removed from the Exerciser (5 scenes; paddle capability remains verified in `litmus_paddle` and the
  harness paddle input path). 38 scenarios pass.

## [1.0.0] - 2026-06-11

**The harness is 1.0.** The declared bar — a trustworthy loop (gaps A–E), a sourced fundamentals audit with
the unknowns measured, a verified techniques catalog, a two-emulator oracle, and **one artifact composing
every capability** — is met:

- **The Exerciser ROM is complete** (M1–M8, v0.56.0–v0.62.0): an 8K F8 cartridge whose six scenes compose
  the 48px six-store kernel + live BCD score + a 2-channel music driver, zone multiplexing over an
  asymmetric playfield, an interactive collision playground, paddle reading, per-scanline color + SFX, and
  LFSR procedural generation — all driven by input-timeline scenarios, locked by video/audio goldens, and
  green in CI on every push (39 scenarios; every scene provably inside the 76-cycle line budget via its
  262-line assertion).
- **Verification surface**: 26 litmus ROMs; the v2 fundamentals backlog closed (Tier 1–3, incl. VDEL, HMOVE
  side effects, asymmetric-PF windows, inputs incl. paddles, F8 bankswitching + `read_bank`, 6502/BCD
  precision, all 15 collision pairs, RIOT timers, mirrors, LFSR, audio sample capture + `pkg/audio`).
- **Cross-emulator agreement**: `cmd/stellacheck` RAM cross-checks PASS against Stella for `smoke` and the
  `litmus_6502` measurement suite (128/128 bytes each). The Exerciser cross-check additionally showed all
  structural state agreeing, with only per-frame counters phase-shifted by the emulators' differing
  frame-boundary cut points — measured and documented in `docs/stella-oracle.md` (sub-frame alignment = v2).
- **Docs**: routing-tabled deep dives (`fundamentals-audit`, `techniques/`, `exerciser`, `stella-oracle`,
  `verified-coverage`), each fact tagged verified/documented with sources.

## [0.50.0] - 2026-06-11

### Added
- **RESBL vs RESPx mid-line re-strobe litmus (v2 V2-11).** `litmus_resp_edge.asm` confirms Towers'
  TIA_HW_Notes: strobing **RESBL twice on one scanline draws two balls** (clocks 38 and 140 — the ball
  re-emits START, the multi-ball trick), while strobing **RESP0 twice draws a single 8px player** at the
  last position only (clock 107 — the player does not re-emit START until the 160-clock wrap). Locked by
  `scenarios/resp_edge.json` (position asserts + golden). 28 scenarios pass.

## [0.49.0] - 2026-06-11

### Added
- **Address-mirror litmus (v2 V2-12).** `litmus_mirror.asm` proves the memory map's mirroring: writing $5A to
  $0180 reads back at $0080 (and the reverse) — i.e. RAM $80–$FF is mirrored at $0180–$01FF, **which is why
  the stack works**; and setting the background through the TIA mirror $0049 colours COLUBK ($84 blue in
  `read_row`). Locked by `scenarios/mirror.json`. 27 scenarios pass.

## [0.48.0] - 2026-06-11

### Added
- **All 15 collision pairs verified in one ROM (v2 V2-8).** `litmus_collide_all.asm` overlaps P0/P1/M0/M1/BL
  (missiles width-8, ball width-8) with a lit PF0 at the left edge so every CXxx pair fires at once;
  `scenarios/collide_all.json` asserts all 15 (`p0_p1, m0_m1, m0_p0, m0_p1, m1_p0, m1_p1, p0_pf, p0_bl,
  p1_pf, p1_bl, m0_pf, m0_bl, m1_pf, m1_bl, bl_pf`) true — superseding the three single-pair litmus in
  coverage. 26 scenarios pass.

## [0.47.0] - 2026-06-11

### Added
- **RIOT timer litmus — answers the audit's open INTIM question (v2 V2-10).** `litmus_timer.asm` records
  INTIM/TIMINT snapshots to RAM: TIM1T counts down 1/cycle (consecutive reads −7 = the read-loop cost);
  after underflow INTIM wraps into the $FF range and keeps decrementing 1/cycle; **TIMINT D7 (timer-expired)
  is set before INTIM is read ($C0), and reading INTIM clears TIMINT ($00 afterward)** — the audit's open
  "does reading INTIM clear D7?" is now answered **yes**. Locked by `scenarios/timer.json`. 25 scenarios pass.

## [0.46.0] - 2026-06-11

### Added
- **LFSR litmus — procedural-generation foundation (v2 V2-9).** `litmus_lfsr.asm` runs an 8-bit Galois LFSR
  (`lsr / bcc / eor #$8E`, the form in DaveC's Random-Dungeon and common game RNGs) and proves its math
  numerically (pure `read_ram`, no rendering): the first 8 values from seed $01 are
  `01,8E,47,AD,D8,6C,36,1B` (matches hand calculation), it **never decays to $00** across a full sweep, and
  its **period is exactly 255** (returns to the seed). Locked by `scenarios/lfsr.json`. 24 scenarios pass.

## [0.45.0] - 2026-06-11

### Added
- **CTRLPF litmus — SCORE / priority / ball width, incl. the audit's open SCORE×PFP question (v2 V2-7).**
  `litmus_ctrlpf.asm` verifies five regimes: SCORE ($02) paints the left half COLUP0 / right half COLUP1
  (split at clock 80); default priority ($00) draws P0 over the playfield; PFP ($04) draws the playfield over
  P0 (player hidden); **SCORE+PFP ($06) renders the playfield as COLUPF — the SCORE colour substitution is
  *suppressed* under PFP — with the player hidden** (this corner is unspecified in the docs and a likely
  emulator-divergence point; recorded as a Gopher2600 measurement, flagged for the Stella oracle cross-check
  V2-17); ball width D4–5 doubles 1/2/4/8 px. Locked by `scenarios/ctrlpf.json`. 23 scenarios pass.

### Fixed
- **`smoke.asm` now clears collisions after init (CXCLR) — removes platform-dependent CI flakiness.** The
  zero-page clear loop incidentally strobes the TIA strobe registers (RESxx, HMOVE) whose effect depends on
  the power-on TIA state and reset beam timing, leaving sticky collision latches that differed across
  platforms (CI caught `TestReadCollisionsNoSprites` reporting M1-PF / BL-PF on the runner while it passed
  locally). A single CXCLR after init forces a clean, deterministic baseline; rendering (hence all goldens)
  is unchanged.

## [0.44.1] - 2026-06-11

### Changed (docs)
- **README reframed to match the evolved goal.** The project is no longer just "a loop to build games" with
  the five gaps A–E closed (phase 1); it is now a **general, verified 2600 capability base** (phase 2) — a
  fundamentals audit + a techniques catalog, each kept honest by the same numeric loop. Updated the opening
  and the gap-analysis section to name these two living documents and the current scope (20 tools, 20+
  regression scenarios), and to state the aim as *general verified competence, not any one game*.

## [0.44.0] - 2026-06-11

### Added
- **6502/6507 precision litmus — Tier 1 of the v2 backlog complete (V2-6).** `litmus_6502.asm` measures
  instruction facts *on the machine itself* via RIOT TIM1T (1 cycle/tick) and pins them in
  `scenarios/cpu6502.json`, all matching 6502.org exactly: **NMOS BCD** $99+$01 → A=$00 with C=1 correct
  while **Z=0 lies** (the documented NMOS unreliability, recorded); **JMP ($xxFF)** takes the page-bug path;
  **LDA abs,X** 4→5 cycles on page cross while **STA abs,X stays 5 fixed** (why store timing in kernels is
  deterministic); **BNE** 2/3/4 (not taken / taken / taken+cross); illegal **DCP zp = 5 cycles** (also
  certifies illegal-opcode support). 22 scenarios pass.

## [0.43.0] - 2026-06-11

### Added
- **F8 bankswitching verified + `read_bank` MCP tool + `bank.*` scenario fields (v2 backlog V2-5).**
  `litmus_bank.asm` is a best-practices 8K F8 ROM (vectors + an identical reset stub in *both* banks, a
  same-address switch zone whose instruction stream stays valid across the hotspot): every frame bank 0
  marks RAM and hotspot-reads $FFF9 → bank 1 writes its own sentinel and returns via $FFF8. Verified:
  Gopher2600 AUTO fingerprints the plain 8K dasm binary as F8; $80 ends every frame as bank 1's sentinel;
  both per-bank frame counters advance in lockstep; the kernel executes in bank 0 at the frame boundary.
  New `read_bank` MCP tool (20 tools now; `Cartridge.GetBank` at PC, with `is_ram`) and `bank.number` /
  `bank.is_ram` scenario fields; `bin/harness` rebuilt and smoke-tested (initialize + tools/list, no panic).
  Locked by `scenarios/bank.json`. 21 scenarios pass.

## [0.42.0] - 2026-06-11

### Added
- **Input-port litmus with an input-timeline scenario (v2 backlog V2-4).** `litmus_input.asm` samples
  SWCHA/INPT4 to RAM every frame; `scenarios/input.json` drives a press/release timeline and asserts the
  numeric readback: no input = SWCHA $FF, INPT4 $BC (D7=1 + open-bus noise — the documented reason to test
  with N only); P0 left = $BF (D6→0); fire = INPT4 $3C (D7→0); **the VBLANK D6 latch holds INPT4 at $3C
  frames after fire is released** while directions release immediately (the control). 20 scenarios pass.
  Paddle charge-timing verification split off as **V2-4b** (needs a paddle path in `set_input`).

## [0.41.0] - 2026-06-11

### Added
- **Asymmetric-playfield write-window litmus (v2 backlog V2-3).** `litmus_pf_async.asm` verifies woodgrain's
  `Playfield_Timing` tables to the pixel: **(A)** early PF1=$AA (cyc 5) + PF1=$55 at cyc 40 renders a true
  asymmetric playfield — left bits at clocks 16–43, right bits at 100–127, exactly as predicted;
  **(B)** a late write completing at cycle 33 while left PF1 is being drawn splits **per pixel**: the first
  5 bits show the old $FF (clocks 16–35 lit) and the last 3 the new $00 — reproducing woodgrain's worked
  example verbatim. Locked by `scenarios/pf_async.json`. 19 scenarios pass.

## [0.40.0] - 2026-06-11

### Added
- **HMOVE side-effects litmus (v2 backlog V2-2).** `litmus_hmove_side.asm` measures three regimes in one
  frame: **(a)** HMOVE right after WSYNC blanks the left 8px **even with all HMxx=0** (the comb — alternating
  strobe/no-strobe lines compared by `read_row`), confirming Towers' HBLANK+8CLK extension; **(b)** HMOVE
  mid-visible (~cycle 39) produces **zero displacement and no comb** for both HM=0 and HM=$10;
  **(c)** HMOVE at line end (~cycle 74) with HMP0=$10 (+1) moves P0 **left 9px per strobe = value+8**
  (the classic late-HMOVE +8 rule, measured numerically) with no comb. (b)/(c) are recorded as
  Gopher2600-measured values pending the Stella oracle cross-check (V2-17). Locked by
  `scenarios/hmove_side.json` (cumulative-position asserts + golden). 18 scenarios pass.

## [0.39.0] - 2026-06-11

### Added
- **VDEL litmus — verifies vertical delay's write-triggered shadow copies (v2 backlog V2-1).**
  `litmus_vdel.asm` proves all three paths in one frame, exactly as Stella PG §6.D describes:
  with VDELP0=1 a fresh GRP0=$FF stays hidden until **a GRP1 write copies P0's new→old** (then P0 renders
  $FF at X=3); with VDELBL=1 ENABL=on stays hidden until a GRP1 write (ball appears at X=2); with VDELP1=1
  GRP1=$3C stays hidden until **a GRP0 write copies P1's new→old** ($3C renders as 4px at clock 41).
  Locked by `scenarios/vdel.json` (vertical_delay asserts + golden). 17 scenarios pass. This is the
  prerequisite for the 48px score kernel and 2-line-kernel vertical positioning.

## [0.38.0] - 2026-06-11

### Added (docs)
- **Fundamentals audit — `docs/fundamentals-audit.md`.** Six parallel research passes over the local corpus
  (Stella Programmer's Guide, woodgrain wiki, Davie's *Newbies*, SpiceWare's Collect, 8bitworkshop,
  21 real-game disassemblies, DaveC's Random-Dungeon), ~22 owner-supplied links (AtariAge threads, 6502.org,
  Slocum's music guide, Stella debugger docs, Pitfall analyses), and independent web research (Towers'
  *TIA Hardware Notes*, Stolberg). Every domain is classified **verified / documented / unknown / caution**
  with sources. Headline corrections: the local cycle-counting guide's position math is approximate (never
  cite); Pitfall disassembly's LeftRandom comment is wrong (bit0, proven by simulation); SpiceWare Step 3
  vs 7 PF-window discrepancy (to settle by measurement); HMOVE comb/late-HMOVE behavior absent from the
  local shelf (Towers adopted as authority). Headline finds: VDEL's write-triggered cross-copy semantics;
  woodgrain's definitive asymmetric-PF write-window tables; Slocum's complete AUDC/tuning data (the parked
  audio-authoring blocker was already on our shelf); F8-first bankswitching consensus + Gopher2600 already
  auto-fingerprints 8K as F8 and exposes `GetBank()` (a `read_bank` tool candidate); Stella debugger is
  scriptable for automated oracle cross-checks (F-4 design v1).
- **`hardening-roadmap.md` § v2 backlog** — 18 prioritized follow-ups (V2-1…V2-18) in three tiers
  (VDEL, HMOVE side effects, asymmetric-PF windows, input, bankswitch + `read_bank`, 6502 precision; then
  matrix completion; then capabilities: audio sample capture, `pkg/audio`, Stella oracle automation).
- **CLAUDE.md constants hardened**: 24-cycle HMxx freeze after HMOVE; stores never pay page-cross
  penalties; NMOS decimal mode C-only; CLD mandatory at init; cycle-counting-guide caution. Routing tables
  link the audit.

## [0.37.1] - 2026-06-11

### Changed (docs)
- **Roadmap reframed as a general-capability TODO (de-anchored from any single game).** The main goal is a
  general, verified, reusable technique toolkit — not one specific game. `docs/techniques/roadmap.md` now
  prioritizes by **general/foundational value × difficulty × prereqs-verified** (instead of "relevance to a
  particular game"), and is an explicit checklist (`- [ ]`) ordered foundational/easy-wins first
  (animation → vertical positioning/VDEL → 2-line kernel → 48-px score → sound → …). A concrete game can be
  picked flexibly as a per-technique testbed; it is no longer the organizing principle.

## [0.37.0] - 2026-06-11

### Added / Changed (docs)
- **Technique #1 promoted to its formal name + a sourced techniques roadmap.** Researched AtariAge / the
  local `reference/docs_atari/` corpus and Wikipedia: confirmed the formal name is **sprite multiplexing**
  (the loop is a **multi-sprite kernel**); DaveC's "zone" is the common vertical-band term, and our demo is
  the *static-zones* form of the general *sort/position/display + flicker* kernel. Rewrote
  `docs/techniques/zone-multiplexing.md` with a formal-name/taxonomy section, a "Refinements & limits"
  section (2-per-line limit, flicker, single- vs 2-line kernel, positioning cost), See-also (48-px sprite,
  Venetian Blinds), and a sourced References list — marking *documented* vs *verified*. Added
  `docs/techniques/roadmap.md`: a prioritized survey of ~12 next techniques (48-px score, 2-line kernel,
  vertical positioning/VDEL, sound, animation, playfield tricks, LFSR, general flicker kernel, Venetian
  Blinds, bank switching) ranked by North-Star (Frogger) value, difficulty, and prereq-verified status.
  Catalog index links the roadmap. Docs-only; no code change (tests/scenarios unchanged).

## [0.36.1] - 2026-06-11

### Fixed
- **Deterministic emulator power-on state — eliminates CI test flakiness at the root.** Gopher2600
  randomizes the CPU/RAM power-on state (`vcs.Env.Random`, used by `CPU.Reset`), so a fresh `emu.New`
  varied run-to-run; cycle/timing tests (`TestCycleCounterExcludesWsyncStall`, `TestStepScanline`) passed
  locally but flaked in CI. `emu.New` now calls Gopher2600's official `vcs.Env.Normalise()`
  (`Random.ZeroSeed = true` + prefs defaults), the method intended for regression testing, before the
  cartridge-attach reset. Result: identical initial state every run (verified 5×/10× stable). Goldens are
  unaffected (the ROMs clear RAM on boot).

## [0.36.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 gets per-zone background colors — a landscape look.** Each zone sets `COLUBK` from a
  `ZoneBG` table (sky-blue → cyan → green → brown), set in HBLANK so it doesn't disturb the per-zone
  positioning, giving 6 colored bands behind the 12 moving sprites. Golden regenerated; 262 lines preserved.

## [0.35.0] - 2026-06-11

### Changed
- **Zone multiplexing #1 now animates — 12 *moving* sprites.** Each zone's X moved from ROM tables into RAM
  (`zx0`/`zx1`) and is updated every frame (P0 drifts right, P1 left, wrapping `and #$7F`), so all 12 sprites
  animate. Demonstrates RAM-backed motion verifiable purely by `read_ram` (the position bytes change frame to
  frame). VBLANK line count retuned to keep the frame at 262 (the per-frame update loop is absorbed). The
  scenario now locks the frame by `golden_frame` only (robust to the moving positions); all scenarios pass.

## [0.34.0] - 2026-06-11

### Added
- **Techniques catalog + #1 Zone (vertical) sprite multiplexing.** Establishes a repeatable pipeline for
  absorbing 2600 authoring techniques: learn (from `reference/`, local) → clean-room implement
  (`roms/techniques/`) → verify numerically (harness) → cross-check (Stella) → lock in (scenario + golden +
  CI) → optionally promote to `pkg/`. First entry: `roms/techniques/zone_multiplex.asm` puts **12 sprites**
  on screen (6 zones × P0+P1) from a 2-player machine by repositioning P0/P1 per zone (divide-by-15 + HMOVE,
  the harness-verified method). Verified on Gopher2600 + cross-checked in Stella; locked by
  `scenarios/zone_multiplex.json`. CI now runs `roms/techniques/scenarios/` too. Catalog index at
  `docs/techniques/`, linked from the routing tables.

## [0.33.0] - 2026-06-10

### Added
- **Coverage batch: NUSIZ quad-width + missile-player collision.**
  - `litmus_nusiz_quad.asm` (`NUSIZ0=$07`, QuadWidth) → `read_row` shows a **32px** continuous span (8px ×4),
    completing the NUSIZ width modes (double/quad) and copy modes (close/three).
  - `litmus_collide_mp.asm` overlaps an 8px-wide missile0 with player0 → `read_collisions` reports
    **`m0_p0=true`** (CXM0P), extending collision coverage to the missile-player pair. (Also documents the
    1px left-edge offset between missile clamp X=2 and player clamp X=3.)
  - Locked by `scenarios/nusiz_quad.json` and `scenarios/collide_mp.json`. 15 litmus scenarios pass.

## [0.32.0] - 2026-06-10

### Added
- **P0-P1 collision litmus (CXPPMM) — extends collision coverage.** `roms/litmus/litmus_collide_pp.asm`
  overlaps player0 and player1 (both clamped to X=3 via HBLANK strobes) drawing `$FF`; `read_collisions`
  reports **`p0_p1=true`**. Verifies the player-player pair the Frogger `OnPad` check actually uses (previously
  only BL-PF was litmus-verified). Locked by `scenarios/collide_pp.json`. 13 litmus pass.

## [0.31.0] - 2026-06-10

### Added
- **REFP (reflected sprite) litmus — rounds out the sprite track.** `roms/litmus/litmus_refp.asm` draws the
  asymmetric ramp with `REFP0=$08`; `read_tia_registers` shows `player0.reflected=true` and `read_row` shows
  the ramp mirrored (row0 `0x80` lights clock 10 = the right end; row4 `0xF8` lights clock 6–10 = right 5px) —
  the mirror image of the non-reflected `litmus_sprite`. Confirms REFP and `pkg/sprite.Reflect` (data-side
  mirror) are equivalent. Locked by `scenarios/refp.json`. 12 litmus pass.

## [0.30.0] - 2026-06-10

### Added
- **Missile/ball position litmus.** `roms/litmus/litmus_missile.asm` enables and positions missile0 and the
  ball in the visible region; `read_tia` reads **missile0=38 / ball=140** and `read_row` shows a 1px vertical
  line at each clock — verifying the harness reads the missile/ball object-position family (the `X = 3N − 55`
  side, complementing the player `X = 3N − 54` litmus_pos). Locked by `scenarios/missile.json`. 11 litmus pass.

## [0.29.1] - 2026-06-10

### Fixed
- **Flaky `TestStepScanline` (surfaced by CI).** The test asserted every single scanline step consumes
  >0 CPU cycles, but a scanline can legitimately be a pure WSYNC-stall pass-through (0 instructions executed)
  depending on beam-phase alignment — not an invariant. Relaxed to assert the **cumulative** cycles across
  40 scanlines is >0 (the CPU makes progress), which is robust. Keeps the CI badge reliable.

## [0.29.0] - 2026-06-10

### Added
- **NUSIZ multi-copy litmus coverage (extends S-2).** `roms/litmus/litmus_nusiz_copies.asm` renders an 8px
  solid sprite at `NUSIZ0=$03` (ThreeCopiesClose); `read_row` confirms **three 8px white spans at clock
  3/19/35 (16px copy spacing)**. Locked by `scenarios/nusiz_copies.json` (golden + `player0.nusiz=3`).
  Deepens verified coverage of the NUSIZ helper beyond double-width. 10 litmus scenarios now pass.

## [0.28.0] - 2026-06-10

### Added
- **CI via GitHub Actions (hardening-roadmap F-1).** `.github/workflows/ci.yml` runs on every push/PR:
  Ubuntu + Go (from `go.mod`) + DASM, clones Gopher2600 at the pinned commit `5d532e88` into `./Gopher2600`
  (the `replace` target), assembles the litmus ROMs (`.bin` are gitignored), then `CGO_ENABLED=0`
  build/vet/test and runs all litmus regression scenarios. No SDL needed — the harness only imports the
  SDL-free Gopher2600 packages, so a static (cgo-off) build covers it. A CI badge is on the README.
  Verified green on Actions (build/vet/test + 9 scenarios, ~1m).

## [0.27.0] - 2026-06-10

### Added
- **PAL frame verification (hardening-roadmap F-3).** `roms/litmus/litmus_pal.asm` emits a proper PAL frame
  (VSYNC 3 / VBLANK 45 / visible 228 / Overscan 36 = 312 lines) and `scenarios/pal.json` (with
  `tv_spec: "PAL"`) asserts the harness drives/counts it as **312 lines** (plus a RAM sentinel). Confirms the
  harness is not NTSC-only; `ntsc_frame_lines` counts the actual per-frame line total (312 for PAL).

## [0.26.0] - 2026-06-10

### Added
- **Golden-audio regression `checks.golden_audio` (hardening-roadmap A-2).** Mirrors the video golden for
  sound: a sha1 audio-chain (Gopher2600 `digest.Audio`) over the timeline is compared against
  `<scenario>.audio.golden`. `internal/emu` gains `EnableAudioDigest`/`ResetAudioDigest`/`AudioHash`
  (symmetric to the video digest); `internal/scenario`'s golden eval is generalized to share video/audio.
  Verified with `roms/litmus/scenarios/audio.json` on `litmus_audio.asm` (deterministic record→match, plus
  numeric AUDC/AUDF/AUDV asserts). All 8 litmus scenarios pass. CLI only; MCP binary unchanged.

## [0.25.0] - 2026-06-10

### Added
- **`pkg/sprite` NUSIZ helper (hardening-roadmap S-2).** `PlayerSize` (OneCopy … DoubleWidth … QuadWidth) /
  `MissileSize` enums and `NUSIZ(player, missile)` / `NUSIZPlayer(player)` compose a NUSIZx byte from intent
  instead of raw bits. **Verified on Gopher2600** with `roms/litmus/litmus_nusiz.asm`: an 8px solid sprite at
  `NUSIZ0=$05` (DoubleWidth) renders **16px wide** (`read_row` clock 4–19 = white len 16) and
  `read_tia_registers` shows `player0.nusiz=5`. Locked by `scenarios/nusiz.json`. Completes the sprite
  authoring trio (S-1 encoder + S-2 NUSIZ + S-3 P0+P1 combine).

## [0.24.0] - 2026-06-10

### Added
- **`pkg/sprite.SplitWide` + P0+P1 16px combine litmus (hardening-roadmap S-3 — flagship).** Split a
  16-wide ASCII design into P0 (left 8) + P1 (right 8) GRP tables, then place P1 exactly +8px to the right
  of P0 for a seamless up-to-16px (or multicolor) character. `roms/litmus/litmus_p0p1.asm` positions the two
  sprites by strobing RESP0→RESP1 three cycles apart in the visible region (= +9px; an HBLANK strobe would
  clamp both to the left edge) then HMOVE P1 left 1 → exactly +8px.
  **Verified on Gopher2600:** `read_tia` shows player0=69 / player1=77 (exactly +8); `read_row` shows the
  solid-16 rows as a **single continuous 16px white run (clock 69–84, no seam gap/overlap)**, with P0-only /
  P1-only / far-edge rows byte-exact. Locked for regression by `scenarios/p0p1.json` (position asserts 69/77
  + golden frame). This proves sprite placement is as numerically trustworthy as playfield — the headline
  capability of the sprite track.

## [0.23.0] - 2026-06-10

### Added
- **`pkg/sprite` — ASCII → player GRP encoder (hardening-roadmap S-1).** A mirror of `pkg/playfield` for
  player graphics: 8-wide ASCII rows → GRP bytes (`EncodeRow`/`Encode`, D7 = leftmost = standard TIA bit
  order), plus `Reflect` for REFP-less mirroring / P0+P1 right halves. Reuses `playfield.ParseASCIIRow`.
  Unit-tested, including that `..XXXX..` = `0x3C` matches the existing hand-coded Monet Frogger lily-pad byte.
- **`roms/litmus/litmus_sprite.asm` + `scenarios/sprite.json` (+golden) — numeric hardware proof.** An
  asymmetric ramp sprite (top `0x80` 1px → bottom `0xFF` 8px) rendered by player0 at X=3. Verified on
  Gopher2600 via `read_row`: the white span widens 1→2→…→8 px from clock 3 (visible lines 96–103), proving
  D7 = leftmost and top→bottom row order are byte-exact. Locked for regression with a golden-frame scenario;
  all litmus scenarios PASS. First step of the sprite track toward the P0+P1 16px flagship (S-3).

### Added
- **Strengthening roadmap (`docs/hardening-roadmap.md`).** A prioritized roadmap for the next phase —
  making the harness stronger beyond gap-closing. Theme A: deepen authoring + verification into the thin
  domains (S = sprites, incl. `pkg/sprite` ASCII→GRP, NUSIZ helper, and the ★ P0+P1 two-sprite combine for
  up to 16px / multicolor characters placed numerically via the X(N) calibration; A = audio, incl. note/
  timbre names in `read_audio` via Gopher2600 `tracker`, a `digest.Audio` golden, and a `pkg/audio` SFX
  helper). Theme B: harden the foundation (★ CI via GitHub Actions, optional Gopher2600 version pin,
  PAL/SECAM verification, Stella oracle cross-check, completing `step_clock`/`watch|trap`/`run_scenario`).
  Theme C: wire upstream Gopher2600 libraries (`recorder`/`regression`/`reflection`). Each item lists where
  to touch + how to verify + size. Cross-linked from the routing tables in CLAUDE.md / README /
  improvement-roadmap. No code changes (implementation in separate sessions).

## [0.22.1] - 2026-06-10

### Added
- **GPL-3.0 `LICENSE`.** The harness embeds Gopher2600 (GPL-3.0) as a library, so the combined work is
  GPL-3.0-or-later. Added copyright and an Acknowledgements section to the README.

### Changed
- **Public-readiness: the published repo is now English-only.** Translated the public surface
  (README + `docs/`×7 + CHANGELOG + CLAUDE.md) to English. The author works in Japanese, so Japanese copies
  are kept locally as `*.ja.md` sidecars (gitignored, never published). Calibrated the prior-art wording to
  "no Atari 2600 MCP found in a public search (2026-06; Atari Lynx = gearlynx exists)" rather than claiming
  "first". Removed the README provenance section. No code changes; build/vet/test green.

## [0.22.0] - 2026-06-10

### Changed
- **Physical spinoff: split the base into a standalone repo `atari2600-harness` (game ROMs move to a
  separate repo `atari2600-roms`).** Under an umbrella folder `260609_atari2600-dev/`, place `harness/`
  (this repo, history preserved) and `roms/` (new repo) as siblings, bound by `go.work`. Remove
  `roms/260610_frogger` from the harness (moved to the roms repo); `roms/litmus` stays as the harness's own
  verification ROMs. **Eradicate the harness→game dependency:** repoint the scenario/emu unit tests from
  frogger ROMs to litmus, and add a new fixture `roms/litmus/scenarios/golden.json` (+`.golden`).
  `.mcp.json`/`.claude` move up to the umbrella (read at Claude Code's project root). Updated CLAUDE.md's
  structure/dev sections to the post-spinoff reality. Verified: harness `go vet`/`go test` green, 4 litmus
  scenarios PASS; on the roms side `gen` + 3 frogger scenarios PASS.
- **Renamed the Go module `github.com/kidsnz/atari2600-dev` → `github.com/kidsnz/atari2600-harness`
  (spinoff prep).** `go.mod` and 9 import files replaced. build/vet/test green, all scenarios PASS.
- **Promoted `internal/playfield` → `pkg/playfield` (spinoff prep).** Go can't import `internal/` across
  modules, so the playfield encoder (universal Atari 2600 knowledge) became a public package. Updated the
  only cross-package importer (`roms/260610_frogger/gen`). Regenerated all scenes (header-comment-only diffs).
  Verified green; all scenarios (3 frogger + 3 litmus) PASS.
- **Documentation freshness audit (spinoff preamble).** Rewrote `README.md` to v0.21.0 reality (old diagram
  = `cmd/probe` + `internal/emu` only → 4 cmds, 6 internals, roms/<game>, 19 MCP tools, gaps A–E all
  closed; fixed the smoke.asm path to `roms/litmus/`). Fixed minor staleness in `improvement-roadmap`,
  `mcp-tools`, `tool-landscape`, and a stale `cmd/genpf` comment in `roms/260610_frogger/gen/asmgen.go`.

### Added
- **Improvement roadmap document (`docs/improvement-roadmap.md`).** Prioritizes next moves to make authoring
  more accurate, from every angle. Central observation = the position litmus is closed but the timing
  *budget* verification is open (gap B is the biggest hole in the real loop). P0 = cycle exposure +
  per-scanline budget guard, P1 = TIA shadow / collision register reads, P2 = verification automation,
  P3 = build-loop shortening, each annotated with verified Gopher2600 API symbols
  (`CPU.LastResult.Cycles`, `TIA.Video.*`, `Collisions`). Also added "untapped reference veins" (R-1 Freeway
  architecture port, R-2 audio recipes, R-3 cycle-cost table, R-4 real-game structure index) and "external
  research" (the biggest finding: Gopher2600 already implements the hardest items as libraries —
  `recorder`/`regression`/`tracker`/`reflection`/`digest`/`rewind` are usable standalone, shrinking P2/R-2
  from "build" to "wire"; License = GPL-3.0; an Atari 2600 MCP was not found in a public search = no known
  prior art, not a claim of being first; G-2 C64 MCPs, G-3 test DSLs, G-4 authoring-tool integration).

## [0.21.0] - 2026-06-10

### Added / Changed
- **A `.asm` source can be specified directly as a scenario `rom` (gap E fully closed).** If a scenario's
  `rom` is `.asm`, it is assembled with dasm before running = "one source → assemble → run → numeric
  asserts → verdict" in one command (`go run ./cmd/scenario foo.json`). Gap E reaches its ideal form.
- **Consolidated dasm invocation into `internal/build` (DRY).** `assemble_and_load` (harness) and the
  scenario `.asm` feature share `build.Assemble`/`build.BinPathFor`. Assemble failures are returned as
  errors (dasm output including the failing line), not swallowed. Sample: `roms/litmus/scenarios/smoke_src.json`.

## [0.20.0] - 2026-06-10

### Added
- **Automatic calibration of horizontal X(N) (B-4 / gap B fully closed).** Turns litmus from a one-off
  manual job into a reproducible sweep→auto-fit. A cooperating ROM (`litmus_pos`: delay `DELAY=$80`,
  SBC/BCS = 5 CPU cycles/unit) is poked across delays, `player0.ResetPixel` is measured each frame, and a
  linear regression recovers slope and offset numerically. Implementation: `internal/calibrate` (`Sweep`,
  `Fit` — robust to the 160 wrap and left-edge saturation via median-delta unwrapping of the longest
  consistent run). Result on litmus_pos: **slope = 3.0000 px/CPU-cycle** (matches the authoritative 3),
  R²=1.0, kernel offset = −18. Verified in `calibrate_test.go`.

## [0.19.0] - 2026-06-10

### Added
- **Golden-frame regression (P2 D-3 / gap D fully closed).** Adding `checks.golden_frame: true` compares the
  timeline's **rendered frame-chain hash** against `<scenario>.golden` = pixel-level regression detection of
  rendering (complements the D-1/D-2 logic/timing regression). Implementation: wire Gopher2600's exported
  `digest.Video` into `internal/emu` (`EnableVideoDigest`/`ResetVideoDigest`/`VideoHash`); `internal/scenario`
  enables it for golden scenarios, resets after warmup (deterministic), and compares to `.golden`.
  `cmd/scenario -update` records/updates the baseline. Sample: `roms/260610_frogger/scenarios/golden.json` +
  committed `golden.golden`. CLI only; `bin/harness` (MCP) unchanged.

## [0.18.0] - 2026-06-10

### Added
- **Scenario runner (P2 / gap D = first step of verification automation. D-1 assertions + D-2 input replay).**
  Declares an "input timeline + numeric assertions" in one JSON and auto-passes/fails it against a ROM.
  `go run ./cmd/scenario <file.json> ...` (exit 0 on all pass, 1 on failure) = a regression base that runs
  in CI **without MCP**. Key design: the assertion vocabulary (`field` strings) maps one-to-one to
  `internal/emu`'s read methods (dogfooding the observation tools as the regression vocabulary). Unknown
  fields are an error (no swallowing typos). Whole-run measurements with side effects are separated into
  `checks{ntsc_frame_lines, max_line_budget}`. Structure: `internal/scenario` (parse + vocab + Run,
  ROM-agnostic) / `cmd/scenario` (thin CLI). Samples under `roms/litmus/scenarios/` and
  `roms/260610_frogger/scenarios/` (including `hop` = `up` input drives FrogY 144→128). CLI only; MCP unchanged.

## [0.17.0] - 2026-06-10
### Added
- **`read_audio` MCP tool (R-2 / audio verification path).** Returns the current TIA audio registers
  AUDC/AUDF/AUDV for both channels as numbers (extends rule 1 "verify with numbers" to audio). Uses
  Gopher2600's exported `Audio.PeekChannels()`. Verification ROM `roms/litmus/litmus_audio.asm`; exact match
  in `emu_audio_test.go`.

## [0.16.0] - 2026-06-10
### Added
- **`assemble_and_load` MCP tool (P3 / build-loop shortening).** Takes an asm path, runs `dasm -f3` via
  `os/exec`, and loads the output `.bin` on success — collapsing `edit→dasm→load_rom`. On failure returns a
  structured `ok=false` + `dasm_output` (failing line) instead of an MCP error, so the model can fix in place.

## [0.15.0] - 2026-06-10
### Added
- **`step_instruction` / `step_scanline` MCP tools (B-2 / intra-frame granularity).** `step_instruction`
  runs exactly one CPU instruction (returns its cycles + coords); `step_scanline` runs until scanline +1
  (returns cycles consumed). A color-clock-granular `step_clock` is unimplemented (`Step` is per-instruction).

## [0.14.0] - 2026-06-10
### Added
- **`read_tia_registers` MCP tool (P1 / closes the rest of gap A).** Returns current values of write-only TIA
  registers directly from Gopher2600 internals (measure instead of inferring color from `read_row`). Confirmed
  PF0=$F0 (upper-nibble-only) behavior.
- **`read_collisions` MCP tool (P1).** Structures the 8 collision latches (CXxx, $30–$37) into named boolean
  pairs. Bit assignment verified against Gopher2600's `collisions.go`; BL-PF positive on `litmus_collide.asm`.

## [0.13.0] - 2026-06-10
### Added
- **`assert_line_budget` MCP tool (the crux of gap B / B-3 = per-scanline cycle budget guard).** Numerically
  catches the failure that silently killed Pong v2 (per-scanline overrun → screen roll). Detection: a WSYNC
  strobe = a `RdyFlg` true→false transition; the scanline delta between strobes = physical lines consumed by
  that logical line. Implemented with exported `RdyFlg` + beam coords in `internal/emu`'s own step loop
  (no debugger driver). Verified with `roms/litmus/litmus_overrun.asm` (`over=true`, `line_cycles=152`); no
  false positives on smoke / frogger.

## [0.12.1] - 2026-06-10
### Fixed
- **`read_cycles` double-counted spinning during a WSYNC stall (v0.12.0 bug).** During a WSYNC stall the CPU
  doesn't execute but leaves `LastResult` in place, so the old per-boundary accumulation over-counted on any
  WSYNC-using ROM. Fix: unify progress through a `stepInstr()` primitive and accumulate only when `RdyFlg`
  was true before the Step (i.e. a real instruction ran). Regression test `TestCycleCounterExcludesWsyncStall`.

## [0.12.0] - 2026-06-10
### Added
- **`read_cycles` MCP tool (gap B = wiring timing into the real loop, P0 step 1 / B-1).** Gets CPU cycles
  from the simulator numerically (first embodies rule 2 outside litmus). Returns `last_instruction_cycles`,
  `cycles_since_mark`, `total_cycles`. Source = `CPU.LastResult.Cycles` accumulated at instruction
  boundaries across all progress paths. Verified via the invariant "executed cycles × 3 == color clocks" on
  WSYNC-free `litmus_cycles.asm` (1 frame = 263×76 = `total_cycles 19988`).

## [0.11.0] - 2026-06-10
### Changed
- **Monorepo reorg: root = harness base / `roms/<game>/` = ROMs (spinoff Phase 1).** Demonstrated the
  game→harness one-way dependency and separated without surgery. Moved game-specific kernel generation
  (`cmd/genpf` + asmgen) into `roms/260610_frogger/gen/` (package main importing `playfield`); litmus under
  `roms/litmus/`. All builds/tests green; `litmus_pf` read_row identical after the reorg.

## [0.10.1] - 2026-06-10
### Added
- **Frogger polish.** Game over / restart (Lives→0 resets Lives=3/Score=0); visual zones (top = goal band,
  bottom = start bank, middle = Monet water).

## [0.10.0] - 2026-06-10
### Added
- **🎉 Playable Monet Frogger (M5).** A frog crosses a river on flowing lily pads over Monet water. A full
  game kernel (`GenerateFroggerASM`) handles ride/drown/win/lives via a state machine; collisions via CXPPMM.
  The model **played it itself** (set_input + peek/read_tia) to numerically verify every mechanic — and found
  and fixed a fatal landing-frame timing bug that way (1-frame grace via `PrevY`).

## [0.9.3] - 2026-06-09
### Added
- **Frog vertical hop.** player0 drawn at variable scanline `FrogY`; edge-detected up/down jumps it ±16 (one
  lane) on press (no auto-repeat). The model operates/observes/judges in a closed headless loop.

## [0.9.2] - 2026-06-09
### Added
- **Collision check (the Frogger core: on a pad vs in the water).** Per-frame `CXCLR` strobe; CXPPMM read via
  `peek $37` (no new tool needed). Set/clear verified frame-by-frame.

## [0.9.1] - 2026-06-09
### Added
- **Full-scene integration.** Flowing lily (player0) + controllable frog (player1) coexist over Monet water
  (per-scanline COLUBK), with separate motion applied to both via one HMOVE.

## [0.9.0] - 2026-06-09
### Added
- **`set_input` tool = joystick injection.** `poke` doesn't work for input (RIOT redrives SWCHA each frame),
  so inject via Gopher2600's `Ports.HandleInputEvent`. Control ROM verifies "input → frog moves" headlessly.
### Fixed
- A `set_input` jsonschema tag starting with `0=…`/`true=…` made go-sdk panic in AddTool; reworded the tags.

## [0.8.1] - 2026-06-09
### Added
- **Monet water + flowing lily sprite integration (M3 step 2).** Per-scanline COLUBK (water) + per-scanline
  GRP0 (lily) both resolved in HBLANK to dodge cycle criticality; drift via per-frame HMOVE.

## [0.8.0] - 2026-06-09
### Added
- **M2/M3 animation groundwork.** Per-frame color-table animation (`GenerateAsymmetricShimmerASM`, with
  TIM64T-timed VBLANK/Overscan) and smooth sprite horizontal motion = water flow (`sprite_flow.asm`, +1px/frame
  via per-frame HMOVE) — establishing that smooth horizontal motion on the 2600 is the sprite's (HMOVE) job.

## [0.7.2] - 2026-06-09
### Changed
- **Promoted the Monet still (M1) to an asymmetric version** (left/right-independent playfield + per-row water
  color). Per-row water (COLUBK) + constant lily (COLUPF), since the asymmetric loop has budget for only one
  per-row color channel.

## [0.7.1] - 2026-06-09
### Added
- **Asymmetric (left/right-independent) playfield capability, hardware-verified.** Transcribed ABB's "repeated"
  asymmetric kernel (72 cy/line, `tay`/`sty` timing). read_row proves one-sided lighting (impossible with reflect).

## [0.7.0] - 2026-06-09
### Added
- **M1 "quiet pond" — the rendering pipeline opens end to end.** First milestone of the north-star ROM. Path:
  ASCII art + color → EncodeSymmetric → asmgen(kernel) → dasm → load_rom → read_row check. `GenerateSymmetricASM`
  generates a self-contained reflect-playfield still with per-row COLUBK water.

## [0.6.0] - 2026-06-09
### Added
- **`read_row` tool (read playfield-lit columns / per-scanline color numerically).** RLE `{clock,len,hex}` of a
  visible scanline. Playfield bit-order litmus (`litmus_pf.asm`) and per-scanline color litmus (`litmus_color.asm`)
  pass; the verified bit-order table is burned into `docs/resources.md` / `CLAUDE.md`. The `internal/playfield`
  package (`EncodeSymmetric`/`EncodeAsymmetric`) self-verifies against the real litmus values in go test.

## [0.5.1] - 2026-06-09
### Added
- **`get_screen_annotated` also saves the PNG to a file** (env `ATARI2600_SCREEN_PATH`) so clients that don't
  render inline images (CLI terminals) can still open the latest frame; VS Code auto-reloads on change. Returns
  `png_path` in the structured Out.

## [0.5.0] - 2026-06-09
### Added
- **`get_screen_annotated` implemented (the user↔model comms channel), as a first-class citizen.** Captures the
  frame to `image.RGBA` (PixelRenderer), draws a TIA-coordinate XY grid + axis labels + sprite markers (Fixed
  Debug Colors) at ×3 nearest-neighbor, and returns **image (ImageContent PNG) + numbers together**. Enables the
  "user points on the image → model translates to registers" round trip.

## [0.4.1] - 2026-06-09
### Changed
- **Distilled core constants into CLAUDE.md (Phase 4):** the beam-coord convention `Clock` = HBLANK −68..−1 /
  visible 0..159, horizontal position (3px/cycle, coarse 15px, 160 wrap, leftmost X=3; offset is kernel-specific,
  final verdict via `read_tia.HmovedPixel`), the fully hardware-verified HMOVE table, and the annotated screenshot
  redefined as the primary user↔model channel.

## [0.4.0] - 2026-06-09
### Added
- **Litmus test fully passed (Phase 3) — the harness proven real, numerically (rule #4).** Coarse
  (`litmus_pos.asm`): 1 loop = 5 CPU cycles = 15px, linear over DELAY 3–11 (`ResetPixel = 15·DELAY − 18`),
  160 wrap, leftmost X=3. Fine (`litmus_hmove.asm`): all 16 HMP0 nibbles match the CLAUDE.md HMOVE table at 1px
  granularity. Coarse 15px + fine 1px = any X numerically predictable/placeable/verifiable. Detoxes Pong's
  failures #1/#3 (gap B).

## [0.3.0] - 2026-06-09
### Added
- **Harness plumbing verified (Phase 2.1)** — Gopher2600 embedded as a library, driven fully headless and
  numerically on a real ROM. `internal/emu` driver wrapper; `cmd/probe` numeric CLI; `roms/smoke.asm` confirms
  262 lines / RAM `$80`=$42 / PC.
- **Minimal MCP prototype (Phase 2.2)** — `cmd/harness` exposes 8 tools over stdio; JSON-RPC confirmed numerically.
  Official `modelcontextprotocol/go-sdk` v1.6.1, typed Out auto-generates JSON Schema. Spec in `docs/mcp-tools.md`.
### Decisions
- **Drive via direct `hardware.VCS` embedding, not terminal/PushedFunction** — `hardware`/`television`/`setup` are
  pure Go (no SDL/cgo), so library embedding is more deterministic/simple/fast. The terminal driving the research
  docs assumed was unnecessary.
- **★ Beam clock convention settled on hardware:** `GetCoords().Clock` = HBLANK −68..−1 / visible 0..159 (the
  spec's tentative "0–227" was wrong); same coordinate system as `HmovedPixel`.

## [0.2.0] - 2026-06-09
### Added
- **macOS / Apple Silicon environment set up.** Go 1.26.4, cc65/sim65, pkgconf, Gopher2600 built
  (`go build -tags=release .`), DASM / Stella / SDL2.

## [0.1.0] - 2026-06-09
### Added
- **Project founded.** Defined the goal as "an environment where the model can author the Atari 2600 in 6502
  assembly accurately." Initial `docs/gap-analysis.md` (gaps A–E from the past-Pong post-mortem),
  `docs/tool-landscape.md`, `docs/resources.md` (horizontal formula `X = 3N − 55`, HMOVE table, frame budget,
  collision registers), README, CHANGELOG.
### Decisions
- **Engine = Gopher2600** (the only high-accuracy 2600 emulator drivable at CPU + color-clock granularity on
  macOS), wrapped in a thin Go MCP. **BizHawk not adopted** (no macOS). Regression layer = sim65 / 6502profiler;
  oracle = Stella; top-priority gap = B (timing). MCP SDK = official `modelcontextprotocol/go-sdk`; design follows
  mcp-gameboy. Image overlay in-house Go (no ImageMagick shell-out). Regression around Gopher2600's record/replay
  + `regress`.
### Changed
- Renamed the directory from `Stella-MCP` to `atari2600-dev` (the engine isn't limited to Stella, and the
  deliverable is a whole environment, not a single MCP).
