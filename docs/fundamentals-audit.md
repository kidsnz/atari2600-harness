# Fundamentals audit — what we know, what we assume, what we don't (2026-06)

A systematic audit of Atari 2600 fundamentals **before** absorbing more techniques. Method: six parallel
research passes over (a) the primary specs and tutorial corpus held locally in `reference/` (Stella
Programmer's Guide, woodgrain wiki, Andrew Davie's *Newbies*, SpiceWare's *Collect/Let's Make a Game*,
8bitworkshop samples, real-game disassemblies, DaveC's samples), (b) ~22 owner-supplied links (AtariAge
threads, 6502.org, Slocum's music guide, Stella debugger docs, Pitfall analyses, nanochess/za2600 repos),
and (c) independent web research (Andrew Towers' *TIA Hardware Notes*, Stolberg's frequency guide). Every
constant-level claim was cross-checked against ≥2 sources or flagged.

**Legend** — ✅ **verified**: measured by our litmus ROMs, locked in CI. 📖 **documented**: stated by a
primary spec or ≥2 independent sources, *not yet measured by us*. ⬜ **unknown**: no authoritative source
found, or sources conflict — measure it ourselves. ⚠️ **caution**: a trap, contradiction, or correction.

The actionable follow-ups (now delivered — see `CHANGELOG.md`) and any remaining gaps live in the single live
backlog `capability-gap-audit.md`. Verified facts remain cataloged in `verified-coverage.md`.

---

## 1. Frame & timing
- ✅ NTSC 262 (3/37/192/30), PAL 312; 1 line = 228 clocks = 76 CPU cycles; cycle-counting invariant.
  📖 **PAL's breakdown, from the vendor's own table** — `reference/docs_atari/stella_programmers_guide.html`,
  quoted verbatim (2026-09-04): VBLANK **40 / 48**, KERNEL **192 / 228**, OVERSCAN **30 / 36**,
  FRAME **262 / 312** lines, and 16686 / 20055 µs. Only the totals were here before. **PAL's kernel is
  228 lines, not 240** — which matters wherever "how many visible lines" is the free variable, and
  `design-principles.md`'s aspect-ratio note is exactly such a place. Marked 📖: this is what Atari
  specified, not what we measured, and the two are different claims. Found by the mailing-list
  distillation (helper-1), from a 2003 post pointing at page 16 of the Guide.
  📖 **Where 192 came from, and what drawing more lines costs.** Glenn Saunders, 2002: *"When the VCS
  was first being designed, Larry Wagner studied TVs that were on the market at the time to measure how
  much they displayed"*, and *"That's where the magic 192 scanlines count comes in. This has nothing to
  do with the NTSC spec. It's just a best-practice"*; his own rule is 200 (*"This is of course a matter
  of debate"*), and *"You can probably go all the way to 204 or so depending on how bold you want to
  be"* 〔stella-list `200212/msg00083`〕. Eric Ball, the same day: *"it is possible to have up to 240
  active lines"*, with 4 VBLANK lines before and 15 after the 3 of VSYNC — *"This is for NTSC, of
  course"* (`200212/msg00089`). The cost was put in 1997 by Eckhard Stolberg: *"You can take a couple of
  lines from overscan to display graphics, but not all TVs might be able to show that many. But usually
  you wouldn't want that, because the VBLANK and the Overscan lines are the only time, where you can use
  the processor fully for gamelogic"* (`199709/msg00229`); in the same thread Erik Mooney: *"some games
  do display more than 192 lines. Missile Command draws 222"* (`199709/msg00226`). The size of those two
  rooms is measured in the work-placement ✅ below. **Cited only, not verified** — no picture taller
  than 192 lines was built here.
  The same trade run the other way, Stolberg in 1999: *"If you don't need the full 192 scanlines to
  display your game graphics, you can blank out some more scanlines and use them for game calculations
  as well"* 〔stella-list `199905/msg00048`〕. Saunders had given the history in 2001, a year and a
  half before his 2002 post above: *"I think the 192 line standard they used on the 2600 and 8-bit
  were based on studies of televisions taken way back in early 1976"*, *"Most of Atari's peers
  settled with a 200 line standard"*, and *"These days I think maybe you can push it to 208-210 as
  long as you center the scanline usage but you might have some cropping at the edges on some TVs"*
  (`200105/msg00135`). In
  the same thread, on taking the extra lines from VBLANK, Chris Wilkson: *"you should be able to do
  this, no problem. Note that if you use these lines, you may get weird results with TVs that are
  capable of closed caption decode"* — *"line 21 I think it is"*, and *"something special about line
  17 and a couple of others, but I don't remember the details"* (`200105/msg00134`). Thomas Jentzsch,
  2004, to an author whose frame had too many visible lines: *"The upper limit is around 200
  scanlines (the Stella guide suggests 192)"* (`200404/msg00122`). **Cited only, not verified.**
- 📖 **VSYNC procedure: set D1, wait ≥2 lines, clear** (Stella PG ~§3). **Split by measurability
  2026-09-03** — the procedure and the threshold are different claims and only one of them is ours
  to measure. The *shape* (set D1, hold, clear, and the frame is accepted) is measurable here. The
  **≥2 is not**: `television.go` accepts a frame when
  `vsync.activeScanlineCount >= env.Prefs.TV.VSYNCscanlines`, and that preference defaults to **2**
  (`preferences/television.go` `SetDefaults`), with nothing in this repo setting it. A litmus that
  sweeps 0..4 lines and reports "1 fails, 2 passes" would be reading back the number we supplied.
  See `known-traps.md` section E; it is the sharper twin of the SuperChip SARA entry, because there
  the default disables a feature and the green looks odd, while here the default **agrees with the
  literature** and the green looks like corroboration. What can be measured is that a step exists
  and that there is only one, with a Go-side control moving `VSYNCscanlines` to 3 so the test says
  out loud that the threshold is an input.
  📖 **Where VSYNC is raised in the line decides how many lines it is held.** A 2002 frame raised
  VSYNC part-way along a line and then waited three `WSYNC`s; Thomas Jentzsch: *"The first WSYNC
  happens (according to z26) after about 1/3 of the line. Then you are doing two more WSYNCs. So you
  only do 2 2/3 lines of VSYNC, but you should do at least 3 full lines"* — *"But I'm not 100% sure"*
  〔stella-list `200204/msg00020`〕. The cartridge was unreadable on its author's TV one day and
  readable, its background pulsing, the next (`200204/msg00023`); Jentzsch, agreeing with the author,
  put that down to 2⅔ lines being *"probably very close to an acceptable timing"* (`200204/msg00025`),
  and Paul Slocum had it working *"fine on my TV with the Cuttle Cart or on an EPROM"*, at 271 lines
  (`200204/msg00029`). The shape that avoids it, from the 2003 "Turbo?" ROM, which Manuel
  Polik disassembled and recognised as Jentzsch's by this routine: `LDA #$02 / STA WSYNC / STA VSYNC /
  STA WSYNC / STA WSYNC / LSR / LDX #$37 / STA WSYNC / STA VSYNC / STX TIM64T` 〔`200304/msg00025`〕 —
  both VSYNC writes are the first store after a `WSYNC`, so on and off land at the same point of the
  line three lines apart (counted from the listing, not measured), and the timer is armed right after.
  Stella complains about the short form: Jentzsch, 2025, to another author whom he had pointed at its
  developer mode earlier in the thread, *"your VSYNC is too short (and may vary), because there is no
  WSYNC before enabling VSYNC. That's why Stella is complaining"*; JetSetIlly answered that *"The start
  and end of VSYNC look consistent too me"*, agreed it was under three lines, and named Asteroids as a
  vintage game that generates fewer than three 〔AtariAge `topic/382726`〕.
  The symptom and the fix, with more cases, are in `known-traps.md`'s VSYNC row. **Cited only, not
  verified.**
  📖 **The oldest form of the rule on the list has two conditions.** Eckhard Stolberg, 1997, on a demo
  that *"looks good on the emulator, but produces a rolling picture on my PAL TV"*: *"This is a problem,
  that a small number of TVs seems to have, when the vertical syncronisation isn't done like"*
  `STA WSYNC / STA VSYNC / STA WSYNC / STA WSYNC / STA WSYNC` — *"If you don't wait for exacly three
  scanlines or access any of the VCSs registers during those three lines, the TV can't sync correctly"*
  — adding that a recent RGVC discussion had NTSC TVs with the same problem 〔stella-list
  `199710/msg00081`〕. His own listing writes `WSYNC` inside those lines, so his "any" leaves that one
  out (read here, not stated there). Seven months earlier, in his NTSC-to-PAL guide, the oldest reason
  to break it: *"Do a Vsync in every frame. … Early games only did the Vsync once and then went on
  counting 262 lines for each frame. While this works on NTSC systems and saves 3 lines for game logic,
  it only produces rolling pictures on PAL VCSs"* (`199703/msg00171`). The ≥2 above is the Guide's
  threshold and the engine's input; "exactly three, untouched" is his account of what a few TVs need.
  **Cited only, not verified.**
  📖 **Only D1 of `VSYNC` is read, so the value loaded for `VBLANK` can be stored to it unchanged.**
  Thomas Jentzsch, 2001, shortening a frame routine that loads `#D1+D6` for `VBLANK`, commented out
  the `lda #D1` before `sta VSYNC`: *"just use the old contents D1+D6, VSYNC doesn't have a valid bit
  6"*; the routine's author was *"glad to get your confirmation that this is OK to do"* 〔stella-list
  `200103/msg00003`, `200103/msg00015`〕. The engine reads the same one bit —
  `tia.sig.VSync = reg.Value&0x02 == 0x02` in `Gopher2600/hardware/tia/tia.go` (read from the source,
  not run). **Cited only, not verified** on hardware.
- ✅ **RIOT timers TIM1T/8T/64T/1024T ($294–7)** — verified `litmus_timer` (v0.47.0),
  regression-locked `roms/litmus/scenarios/timer.json`, table row `docs/verified-coverage.md:26`.
  Write 1–255; the counter decrements 1/cycle; **after underflow it continues from $FF, still
  1/cycle** ($94=$EF then $96=$E1). TIMINT $285 D7 = expired ($93=$C0, D7+D6 set).
  ✅ **$C0's two bits are not both measurements** (2026-09-05, `litmus_timint_pa7`). The 6532 gives
  D7 = timer IRQ and D6 = **PA7** IRQ as independent flags (Rockwell's table, transcribed to the
  list in 199708), and the engine implements them as two separate booleans — but `Timer.Reset()`
  opens with `tmr.pa7 = true` unconditionally, before the `RandomState` branch. So D6 is set at
  power-on in a ROM that never touches PA7. Measured by reading TIMINT before writing any timer:
  **$40 at boot** (D6 alone), **$00** on the next read (the access clears it), **$80** after a real
  expiry (D7 alone — D6 does not come back without an edge). ★`scenarios/timer.json` pins the SUM
  as `ram.0x93 == 192`, which cannot report which half moved; `scenarios/timint_pa7.json` pins the
  halves separately. Found by the mailing-list distillation (helper-1), who asked why D6 was set.
  **Reading INTIM clears TIMINT** — $95=$00 after the INTIM read at $94; the scenario pins
  `ram.0x93 == 192` and `ram.0x95 == 0`, so the clear is a regression, not an observation.
  📖 Still documented-only (Stella PG PIA §2.3): **"INTIM holds 0 for one interval before the
  $FF wrap"** — the ROM steps straight through expiry and never samples the 0 interval.
  📖 **What the 0 interval does to a wait loop, and the fix the list gave.** Dennis Debro, 2002, timed a
  192-line kernel with `TIM64T` 228 and got 262 lines in z26, then with `T1024T` 14 and got 246. Eckhard
  Stolberg: *"I suppose you are checking if INTIM has reached zero, when you want to know if the timer
  has expired, right? The T1024T timer will be in this state for 1024 cycles, so your "waiting to
  expire" loop might catch it way too early. This would explain the 16 lines difference. You just need
  to increase your timer value by one"* 〔stella-list `200209/msg00149`, `200210/msg00003`〕. Only the
  reaching-zero side is measured here, for `TIM64T`: written 43, INTIM first reads zero 2692 cycles
  later — about 42 intervals, not 43 (`internal/emu/tim64tzero_test.go`). How long the zero then holds
  is the documented-only line above. **Cited only, not verified.**
  📖 **Deriving the load from a line count, as a 2000 post gives the recipe.** John K. Harvey, following
  Nick Bensema's *How to Draw a PF*: lines × 76, then subtract *"5 cycles timer set"*, *"3 cycles
  WSYNC"* and *"6 cycles for checking loop"*, and divide by 64; he asked whether a table of loads for 1
  to 37 lines existed, and the thread holds no reply 〔stella-list `200007/msg00039`〕. The post's
  worked figures do not start from 37 × 76 = 2812, so they are not repeated here, and the recipe has
  not been checked against the one point measured above. **Cited only, not verified.**
  ✅ **The other way to use the timer — ask without waiting — measured 2026-09-04.** Every INTIM site
  in this repository *waits* (`lda INTIM / bne loop`). ⚠ **That said "five of them" until 2026-09-07 and the count was stale** — re-measured, the tree reads INTIM at **55 sites across 25 files**, of which **18, in 18 different files, are the waiting shape**; the conclusion held while the number rotted. Re-count with `git grep -n 'lda INTIM' -- roms/` and look at the line after each, since it is the branch that makes it a wait. stella-list 2002 polls instead:
  `lda #$FC / and INTIM / beq NoTime / <work> / jmp back` — ask whether there is room for one more
  unit, and if not, drop it. **The mask is what makes the question cheap and also what makes it
  lossy: the bits it hides are budget the program can no longer see.** Over a 20-unit `TIM64T`
  interval, the waste is exactly **(2^k − 1) × 64 cycles** — `$FE` 64, `$FC` **192 (2.5 scanlines)**,
  `$F8` 448, `$F0` 960. The 2002 post's own `$FC` gives away **15 % of the interval** to save the two
  cycles a full compare costs. **And past a threshold the idiom silently does nothing**: `$E0` cannot
  resolve below 32 units, so with a 20-unit interval it answers "no time" on the first read and
  completes **zero** work — while the frame is still 262 lines and everything else looks right. The
  rule, which nothing here stated because nothing here used the idiom: **the mask must resolve a
  value smaller than the interval** (`2^k ≤ INTIM at the start`).
  `→ roms/litmus/litmus_askdontwait.asm` / `internal/emu/askdontwait_test.go`, regression-locked
  `roms/litmus/scenarios/askdontwait.json`. Found by the mailing-list distillation (helper-1).
  📖 **A third shape, for logic that may not fit in one frame: poll the timer around the kernel
  call.** supercat (AtariAge `topic/104777`, 2007) sets `TIM64T` to 124 plus the delay wanted and
  waits for it to count down to 124; in Strat-O-Gems the loop is
  `VLoop: bit INTIM / bmi VNoKernel / jsr KERNEL_V / VNoKernel:` — 7 cycles more per pass, and no
  pass has to be counted as long as it stays under about 200 cycles. A down-counter decremented
  every frame keeps the game running at a constant speed: when a gem lands he loads it, does all the
  computation, and calls the kernel until it reaches 0. **Cited only, not verified** — the thread was
  distilled, not kept, so the numbers are the distillation's paraphrase of his post.
  ✅ **And WHERE in the frame the work goes is about capacity, not ordering — a plausible mechanism
  tested and rejected, 2026-09-04.** The archive treats the move as a fix: Andrew Davie, 2001,
  *"**I moved the routine from the overscan to the vertical bl[ank]**"* to stop a game losing vertical
  sync; Dennis Debro, 2004, moved a console-switch check **the other way** for the same class of
  symptom. The obvious reading is ordering — VBLANK runs before the picture, overscan after it, so
  overscan work "should" reach the screen a frame late. **It does not.** `litmus_workplacement` runs the
  identical work in each region and the kernel uses the same value on the same frame either way,
  because it reads the variable at a fixed moment and cannot tell when the write happened. **What
  actually differs is room: 37 lines against 30 — 7 lines, 532 cycles.** So "move it to VBLANK" is
  not a latency trick, it is *use the bigger of the two rooms*, and the corollary is that a routine
  which does not fit in 37 will not fit in 30 either. At that point the remaining moves are the ones
  the same threads name: spread the work over frames, gate it on the timer (above), or blank a frame.
  `→ roms/litmus/litmus_workplacement.asm` / `internal/emu/workplacement_test.go`, regression-locked
  `roms/litmus/scenarios/workplacement.json`. Hypothesis proposed — and marked as unmeasured — by the
  mailing-list distillation (helper-3).
  📖 **Spreading the work over frames, with a limit one author reported.** Bob Colbert, 2001, among
  hints from his unfinished *Sabotage* clone: *"You can get a TON of extra program time by multiplexing
  the overscan area. By this I mean counting the frames... Lets say you have 3 sections of code, on
  frame 1 you call section 1, frame 2 you call section 2, and frame 3 you call section 3. I was able to
  put, for instance, collision detection and joystick reading in frame 1, object movement in frame 2,
  and still had frame three relatively empty. If you go more than 3 frames the game response
  suffered."* 〔stella-list `200101/msg00079`〕 **Cited only, not verified** — the limit is his
  experience with one game, and nothing here measures response.
  Two months later he described the split differently: *"You can double or triple your overscan
  cycles by dividing your code among frames. I did it with my code across 3 frames. I found that even
  reading the joystick every 3 frames was still very responsive!"*, and the next day, *"The screen gets
  drawn every frame, and the objects get moved every frame, but some of the logic doesn't get called
  every frame. My example of the joystick was one of them. I even had my collision detection routines
  running every 3rd frame"* 〔`200103/msg00248`, `200103/msg00274`〕 — movement every frame, where the
  January post, about the *Sabotage* clone, put object movement in one frame of three; the March posts
  do not name a game. The first March post drew an exchange about how much delay goes unnoticed. Rob
  Kudla, replying to it: *"I've heard that humans can't detect delays less than 70ms"*
  〔`200103/msg00254`〕 (his figure for 3 frames is not 3/60 s — our arithmetic — and is left out). Chris
  Wilkson, answering him: *"Most people will react to a single frame glitch as "what? what was
  that???""*, and *"people who are accustomed to watching video, will be able to conciously read things
  that are there for only a single frame"* 〔`200103/msg00257`〕 — the post that Colbert's *"The screen
  gets drawn every frame"* above answers. Kudla then narrowed his claim — *"I was
  only referring to delays in control mechanisms. Most people can't tell, when they press a button or
  turn a wheel, whether the device responded in 0ms or 70ms."* — said it was hearsay (*"I haven't tried
  this"*; he had it from *"the FPS people (Carmack et al.)"*), and granted the other point: *"Visually,
  you're right."* 〔`200103/msg00269`〕 So Kudla, in the end, set a delay in the controls apart from a
  single frame on the screen; Wilkson's later posts in the thread do not take that distinction up, and
  neither quantity is measured here. **Cited only, not verified.** Thomas Jentzsch, 2002, to a
  programmer running out of CPU time in his object handler: *"you could call some subroutines only in
  odd and other only in even frames. This is what I'm doing (though a bit more complicated) in Thrust
  too"*; Christopher Tumber, replying: *"you often don't really need to do all collision detections
  every frame (unless things are moving REALLY quickly and/or objects are REALLY small)"*, and *"In
  Tsunami, only half the shots (players and enemies - there can be a lot of them) are
  drawn/moved/collision detected every frame (odd numbered shots are handled on odd numbered frames,
  even numbered shots on even frames...)"* 〔`200211/msg00064`, `200211/msg00070`〕. The same post
  gives the price of the other extreme: BIG DIG runs its block-removal chain one step per frame,
  *"Which is why "gameplay" is so "start and stop""*, which he expected to improve *"now that I know
  to put code in the Overscan"*. **Cited only, not verified** — none of these games was run here.
  ⬜ **Exact first-decrement offset.** The countdown band pins three successive reads at
  $3C/$35/$2E from TIM1T=$40 — that fixes the rate (−7 per `lda abs`+`sta zp` iteration) and the
  value 4 ticks after the write **for that instruction sequence**; it does not isolate how many
  cycles after the store the first decrement lands.
  ✅ **What a program actually reads is pinned, though**: writing **20** to `TIM64T` and reading INTIM
  in the next instruction gives **19** — the first decrement has already happened by then
  (`litmus_askdontwait`, asserted). That is the value any caller sees; it does not settle where inside
  the store the decrement falls.
  ⚠ **And that 19 is a modelling choice, not a measurement of the machine.** `Timer.Update` ends with
  `tmr.ticksRemaining = 0`, above a comment from the engine's own author: *"the ticks remaining value
  should be zero or one for accurate timing … **I'm not sure which value is correct** so setting at zero
  until there's a good reason to do otherwise … **to match the debugging values in stella a value of 2 is
  required**"* (`Gopher2600/hardware/riot/timer/timer.go`). So the write does not leave a residual phase
  here — it resets it — and two emulators disagree about that number. **The Stella oracle cannot arbitrate:
  `TIARegNames` has 37 entries and every one of them is TIA (`internal/oracle/stella_tia.go`); INTIM is not
  among them.** This ⬜ is therefore not a gap a litmus ROM can close — it needs real hardware, and until
  then the honest shape is the one `internal/emu/timerdiv_test.go` already uses for the twin question:
  report the hazard, and state that the positive case has no witness rather than implying one with a
  passing test. Found by the mailing-list distillation (helper-2); the three claims were re-measured here
  (the comment read verbatim, the 37 counted from the array rather than from the prose that describes it).
  📖 **The list asked this in 2002 and left it open.** Andrew Towers, timing `TIM64T` = 43 in a Z26
  trace, found the wait *"always drops out of WaitForVblankEnd in the middle of line 39 (cycle 43)"*
  where he expected cycle 25–30 of line 40 — *"out by around 64 cycles (give or take 5)"* — and gave as
  one explanation that *"The RIOT (or the Z26 emulation) doesn't reset it's div-by-64 counter when
  TIM64T is written"*; 44 gave him the exit he wanted 〔stella-list `200208/msg00224`〕. No reply in the
  thread takes that up. It is an emulator trace, not hardware, and a timer whose first decrement lands
  at once — as this engine's does (20 → 19 above) — also exits about one interval before N × 64 with
  the divider reset (read here, not stated there). **Cited only, not verified.**
- ⬜ SECAM; real-game variable line counts (we already treat 262 as a range).
  ⚠ **And two engine fields disagree in a way nothing here notices.** `SpecPAL60.HorizontalScanRate`
  is **15625.00**, but the engine computes that spec's `RefreshRate` from **NTSC's 15734.26** — the
  only one of the five specs that does not divide its own fields. The gap is **0.4170 Hz (0.70%)**.
  **Which of the two is wrong depends on the console the spec describes** (corrected 2026-10-02: this
  line said the literal, because **PAL-M, the same 262-line geometry, declares 15734.26**). A PAL60
  game played on a PAL console runs on that console's clock — Eckhard Stolberg: *"a PAL60/NTSC game
  will run a little slower on a PAL VCS than the same game would on a NTSC VCS"* 〔stella-list
  `200408/msg00030`〕; Erik Mooney put it at about 0.8 %, from line rates of 15750 and 15625 Hz that he
  gave as *"IIRC"* (`200408/msg00032`) — so for that console the rate taken from NTSC is the half that
  does not fit, and PAL-M's figure says nothing about a PAL console's clock (our reading). Neither figure
  is a 2600's own line rate: the clocks in `pkg/audio` over 228 colour clocks give 15699.8 Hz NTSC and
  15556.6 Hz PAL, and 262 lines at the PAL figure is 59.38 Hz (computed here, not measured).
  **Cited only, not verified.** It costs nothing here only because
  `HorizontalScanRate` and `RefreshRate` are read **nowhere** in `internal/`, `pkg/` or `cmd/`;
  timing in this harness comes from scanline counts, not hertz. `internal/ceiling/palette.go`
  resolves `SpecPAL60`, but only to reach its colour generator. That insulation is an accident, not
  a decision, so `internal/emu/pal60rate_test.go` pins both halves and says what to do if either
  moves.
  📖 **One report of a colour effect on SECAM.** Eckhard Stolberg, on the SECAM build of Andrew
  Davie's Interleaved ChronoColour demo (2003): *"It seems that SECAM isn't the best environment for
  this effect"* — he *"could swear that every other line is green in the cap of the bouncing Mario"*,
  and saw *"occasional red flashes directly to the right of both Marios"* 〔stella-list
  `200303/msg00059`〕. The post does not say what he ran it on. **Cited only, not verified.**
  📖 **SECAM's colour/B&W switch, and where B&W lives instead.** Software sees the switch as B&W on a
  SECAM console (the Stella Programmer's Guide, quoted in `techniques/game-states.md` and `ingest.md`);
  Eckhard Stolberg, 2001, put it more exactly: *"The SECAM B/W switch isn't really hardwired to B/W, but
  the B/W switch input on the RIOT is. Therefore a game can't use that switch. But you can still turn
  the colours off on the SECAM VCS. But this is not left to the game, like on PAL and NTSC. They must be
  doing it in hardware."* 〔stella-list `200108/msg00444`; a 2022 thread says the same, AtariAge
  `topic/336537`, of which only the distillation notes are held here〕. On NTSC and PAL the switch only sets `SWCHB` D3, and B&W is whatever the game then
  writes to its colour registers (`design-principles.md`, the colour/B&W entries). **Cited only, not
  verified** — SECAM is not modelled here.
  📖 **What PAL games actually do, by one author's count.** Thomas Jentzsch, 2002, suggesting that one
  cartridge might carry NTSC, PAL60 and, with some blank lines added, PAL: *"You must not do exactly 312
  lines then"* (read here as *need not*) — *"The Atari PAL library ranges from ~284..342 (Acid Drop)
  lines, so ~300 should be enough"* 〔stella-list
  `200211/msg00097`〕. What he had shipped: *"Thrust: NTSC/PAL60 (270 lines)"*, *"Jammed: NTSC (270
  lines)/PAL(300 lines)"*, *"Both switchable with right difficulty"* (`200211/msg00109`); asked about
  Thrust's 262, *"The public binary does, but the cart owners get 8 extra lines"* (`200211/msg00124`).
  **Cited only, not verified** — the range is his count, not one made here.
  📖 **An NTSC count further out.** A 2020 thread reports Andrew Davie using **276** lines in Boulder
  Dash, shown by nearly every television, and puts the requirement on the count being the same every
  frame rather than on 262 〔AtariAge `topic/303750`; only the distillation notes are held here, so
  this is not his wording〕. **Cited only, not verified.**
  📖 **How an emulator decided which region a ROM was, in 2002.** Eckhard Stolberg: *"The only thing we
  do is that we try to autodetect the TV type. But for that we only check the number of scanlines
  during the first couple of frames. If the game does a stable line number of more than 285 lines or
  so, we assume that the game is meant to be PAL. But something like that could probably be added to
  Stella too quite easyly"* 〔stella-list `200201/msg00068`〕 — the "we" reads as z26, which the
  sentences before it set against Stella (our reading). Stella took the format from its `stella.pro`
  properties file, whose entries are made per binary by MD5 checksum: *"NTSC is only the default setting
  for games that don't have a "Display.Format" setting in their stella.pro entry, or that aren't
  included in the stella.pro file at all"* (`200201/msg00043`). Both describe 2002 emulators, not the
  engine run here. **Cited only, not verified.**
  📖 **A program cannot ask the console the same way.** Asked in 2001 why a timer run across two
  vertical blanks would not tell NTSC from PAL, Manuel Polik: *"That's because the vertical blank is
  just as long as you make it, totally indepent from the TV system the console is hooked to"*
  〔stella-list `200108/msg00073`〕. The month before, Christopher Rydberg had written that *"A complete
  PAL frame uses only 99.23% of the cycles that a complete NTSC frame uses"* (`200107/msg00050`);
  Eckhard Stolberg: *"Detection routines based on the frame timing won't work, because both consoles
  should react the same to whatever frame you make them draw"*. What he offered instead was a
  positioning trick inside the 24 cycles after `HMOVE`, from Kool-Aid Man's score display, which on
  all his PAL consoles puts the players partly on top of each other where on the NTSC ones the game
  works — *"But I'm not sure how SECAM consoles would react. And the emulators would also fail this
  test"* (`200107/msg00063`). Rydberg's figure is 312 × 50 / (262 × 60) = 0.99237 to the digits he
  gives — a second of 50 frames of 312 lines against 60 of 262 (computed here); on the two clocks in
  `pkg/audio` the PAL console runs 0.91 % fewer cycles a second (the svolli entry below). Whether a
  program needs to know, Stolberg again: *"There is no need for this. Most PAL TVs can handle 60 Hz VCS
  output quite nicely. The picture will be vertically centered with black bars above and below. It's
  only important that you do 3 uninterrupted scanlines of VSYNC. And it would be better, if you'd turn
  on VBLANK after the last line of display to make sure that the black bars really are black"*
  (`200108/msg00081`). **Cited only, not verified.**
- 📖 **A PAL console is different hardware, not the same console running a different game.** svolli,
  correcting a reply that said only the cartridges differ: *"The chips differ EVEN IN THE PINOUT"* —
  look for AUD1 on the TIA pinout — *"Not only is the TIA different, but the CLOCK of the TIA/CPU is
  also slightly differ[ent]"* 〔AtariAge `topic/203273`〕. **Cited only, not verified** (no pinout was
  read here). The clock half is already in this repository as numbers: `pkg/audio`'s `BaseClockNTSC`
  and `BaseClockPAL` divide **3579545** and **3546894** Hz, so the PAL CPU (colour clock / 3) runs
  **0.91 % slower**. That moves pitch (§6, 15.9 cents) and anything counted in seconds; nothing counted
  in scanlines or cycles moves.
  📖 **What sets that clock is a crystal, and nothing on the power side.** Asked in 1998 whether supply
  voltage, current, RF on the power line or 50/60 Hz mains could move the console's timing, Bob Colbert
  answered *"No, no, no, no!"*, that a *"crystal"* controls it, and that *"the only thing that will make
  a difference is the accuracy of the rating on the crystal that controls the clock of the 6507. I think
  they are pretty accurate"* 〔stella-list `199809/msg00015`〕. Eckhard Stolberg, of one console: *"There
  were two quartz crystals in the PAL VCS, that I sent to Chris Wilkson"* (`199809/msg00016`).
  **Cited only, not verified** — no console was opened here.
- 📖 **One source, two regions: a numeric flag and `IF/ELSE/ENDIF`.** Medieval Mayhem's DASM source
  (`NTSC = 0`, `PAL = 1`, `COMPILE_VERSION = NTSC`, then `IF COMPILE_VERSION = NTSC … ELSE … ENDIF`)
  keeps **every colour constant twice** (red `$44` NTSC / `$64` PAL) and **rescales every frame-counted
  delay by the frame rate** (`FIREBALL_DELAY` 70 NTSC / 59 PAL, i.e. ×60/64 against ×50/64)
  〔AtariAge `topic/156147`〕. **Cited only, not verified** — the thread was distilled, not kept, and no
  value was assembled here. The consequence for PAL60 follows from the ⚠ above, not from the thread:
  PAL60 keeps 60 Hz timing, so a PAL60 build swaps the colours and **no** frame-counted delay.
  📖 **Write the PAL build so it still fits NTSC.** Eckhard Stolberg's 1997 conversion guide, on going
  from PAL to NTSC: *"NTSC systems can only display a smaller number of lines. Therefore you should
  limit yourself to 192 lines of displayed graphics in PAL mode too. Or you should create your graphics
  in a way that allows you to remove enough lines to fit on a NTSC screen"*, and *"Also the number of
  lines that can be used for gamelogic is smaller on NTSC systems. That is why you should do cycle
  counting for the worst case, if you are using any timeconsuming calculation loops"* 〔stella-list
  `199703/msg00171`〕. **Cited only, not verified.**

## 2. Horizontal positioning & HMOVE
- ✅ X(N)=3N−55 (missile/ball), player +1px; slope 3 px/cycle; divide-by-15 coarse; **no leftmost-X constant** (retracted 2026-07-30; it is kernel-specific) /
  missile 2; all 16 HMOVE nibbles (+7..−8, positive = left) right after WSYNC.
- 📖 **The TIA holds one scanline's worth of state, and repeats it until told otherwise.** Eckhard
  Stolberg, 1999: *"The VCS has a line based architecture. It has only enough video memory to hold the
  data for one scanline. You can change the data at any time, but if you don't, the VCS will continue
  to output this scanline over and over again until you blank out display with the VBLANK command. So
  if you don't want to reposition an object througout the screen, positioning it once per frame every
  time the position has changed is enough."* 〔stella-list `199905/msg00095`〕 **Cited only, not
  verified.**
- ⚠️ `reference/docs_atari/cycle_counting_guide.html` uses `X=(CYCLES−20)*3` and "round to 15" — both are
  tutorial approximations. **Never cite it for positioning**; our calibrated formula is more precise.
- ✅ **Do not write HMxx within 24 CPU cycles after HMOVE** — measured `litmus_hmxx_freeze` (v1.53.0):
  on Gopher2600, HMxx is **latched at the HMOVE strobe** — rewrites at +6/+15/+33 cy never alter the
  in-flight movement (right-8 stayed +8/frame in all three windows). Keep the 24-cycle rule as a
  REAL-HARDWARE portability constraint ("unpredictable" on silicon, Stella PG 5×), but our oracle is
  deterministic and write-inert; the rule costs nothing to follow (HMCLR after SLEEP 24, as score6 does).
  📖 **The window covers `RESxx` too, and one effect is made inside it.** Eckhard Stolberg, 1999,
  explaining why River Raid strobes `RESP1` late and corrects it with an extra `HMOVE`: *"If you access
  RESxx or HMxx within 24 cycles after a HMOVE command, you might get different positions for the
  objects than normal. Z26 emulates this effect except for most of the variations on the Cosmic Ark
  starfield effect, which works by doing just this."* 〔stella-list `199908/msg00095`〕 Thomas Jentzsch,
  replying: *"i didn't know, that there are the same restrictions for RESxx as for HMxx"*
  (`199908/msg00104`). The measurement above rewrote `HMP0` at three points and found this engine
  inert there; whether the engine reproduces the starfield, and what a `RESxx` inside the window does
  here, were not measured. **Cited only, not verified.**
- ✅ **HMOVE mechanism** (Towers, *TIA Hardware Notes*) — measured 2026-09-03; this line was
  documented-only until then, and `roms/litmus/litmus_hmove_side.asm` had recorded the numbers in its
  header since V2-2 while **nothing graded them** (the ROM was carried only as ceiling corpus).
  HMOVE struck right after WSYNC **extends HBLANK by 8 colour clocks** → the left-side comb, painted
  with **every HMxx at zero** (16 of band A's 32 lines, strictly alternating). HMOVE struck
  **mid-visible displaces nothing and paints no comb** (0 of 32 lines; P0 holds clock 9 across all 64
  lines of bands A–C). HMOVE struck **at the end of the line adds 8 to the nibble**: HMP0 = $10 asks
  for one clock left and delivers **nine** — measured over 14 uniform strobes, 151 → 34. The loop-exit
  strobe moves **−8** instead, because `bne` falls through there and the strobe lands one cycle
  earlier; that is recorded rather than dropped. `→ internal/emu/hmoveside_test.go` (4 gradings,
  2 negative controls: removing the +8 fails on every step; claiming the comb in the mid-visible
  bands fails by name)
- ✅ **RESPx / RESMx / RESBL reset phase** — measured 2026-09-03; this line was documented-only until then. The player's first visible pixel lands **+5 colour clocks** past the strobe's own end clock (the value Towers' *TIA Hardware Notes* states), and the **missile and the ball land +4** — a one-clock difference between an 8-clock object and a 1-clock object that the document does not carry. One extra CPU cycle before the strobe moves any of them exactly **+3** clocks. Offsets are read against the strobe instruction's own beam position (`TraceClocks`, visible coordinates), so nothing is derived from cycle arithmetic. **Scope, added 2026-09-04:** this is the phase of a TIA with the engine's eight revision bugs
  switched off, which is the default and which nothing here has ever changed. Checked specifically —
  `internal/emu/tiarevision_test.go` renders this ROM under each of the eight and none of them moves
  it, and the reason is in `video/player.go`: `RESPxHBLANK` applies only to a strobe at the very end
  of HBLANK and `LateRESPx` only inside HBLANK during a starting HMOVE ripple, while this ROM strobes
  in the visible area. **So +5/+4 holds for the case measured and says nothing about those two.**
  The engine cites stella-list `199901/msg00089` for `RESPxHBLANK`, where the author of the five-pixel
  figure writes *"maybe I need to revise my 5 pixel delay theory again"* — the same number, doubted by
  its own source, twenty-seven years earlier.
  〔Towers, *TIA Hardware Notes*, RESPx pipeline〕 `→ roms/litmus/litmus_respx_phase.asm` / `internal/emu/respxphase_test.go` (3 gradings, 2 negative controls: the player's offset forced to 4 fails by name; a flat sweep trips the slope control)
  point (explains our verified +5 family offsets). **RESBL re-emits START (ball restartable mid-line);
  RESPx does not** (player needs a 160-clock wrap). ✅ **double-strobe measured 2026-09-05** (`litmus_hmove_double`, `internal/emu/hmovedouble_test.go`). With `HMP0 = $70` (one strobe = -7) and the same `RESP0` each time: one strobe **3 -> 156**; two **back to back** `3 -> 4` (**+1** — neither one move nor two, a strobe inside the running ripple is a third outcome); two **24 cycles apart** `3 -> 156` (**the second adds nothing**); the same with `HMCLR` between, `3 -> 156` (control). ★So within one scanline this engine does **not** accumulate, which is what AtariAge `198577` reports of real hardware. ★★It does not settle the `known-traps.md` warning about strobes on DIFFERENT scanlines with positioning code between — that is a different experiment and is still open. Found by the mailing-list distillation (helper-2), cross-checking the two corpora against each other.
  The mechanism, as stated on the forum and not checked against the schematic here: *"HMOVE is in and of
  itself a sort of delay line, which is highly dependent against the timing of the horizontal blank"*,
  so it cannot usefully be struck more than once a line — which is why a ball re-struck with `RESBL` for
  a second copy on the same line cannot be moved separately 〔AtariAge `topic/257405`, tschak909〕.
  **Cited only, not verified.**
- 📖 **The Cosmic Ark starfield moves its missile 17 pixels a line, and its glitch repeats every four
  lines.** Eckhard Stolberg, 2005, correcting the 15 of his own 1997 post 〔stella-list
  `199705/msg00061`〕: *"I later found out that the shift actually is 17 pixels to the left in each
  scanline"*; the HMOVE logic *"will continue to send a shifting pulse every 4 pixels. During the
  horizontal blank this will result in a 17 pixel shift to the left"*, while in the visible part *"on
  most VCSs the extra HMOVE pulses have no effect"* (`200503/msg00062`). Andrew Towers, replying, on
  the doubled and missing pixels: *"the effect repeats itself every four lines"* — *"There are 68
  clocks of HBLANK time, and each HMOVE moves the missile 17 pixels left. If I were to do this four
  times, I would have used up 68 clocks (17*4=68) so I'm back where I started. The position counter
  counts once for every four clocks, and each line I add 17 -- so I actually add 4 and 1/4 counts to the
  position counter each line. This puts the position counter 1/4 out of phase with the previous line
  each time"*, and *"This does not explain the doubling/missing effect, but it lays the groundwork"*
  (`200503/msg00064`). The 17 has a second source in `design-principles.md` (crispy, 2017); the doubled
  and missing pixels are `known-traps.md`'s Cosmic Ark row. Nothing here measures the starfield, and
  the engine's TIA-revision flags are all off by default (`known-traps.md`'s TIA-revision row), so
  **Cited only, not verified.**
- ✅ **missile-locked-to-player (RESMP D1)** — the ⬜ was stale: `roms/litmus/litmus_resmp.asm` +
  `scenarios/resmp.json` already lock the offset at **+4** (player0.hmoved_pixel 24, missile0 28) and
  confirm it follows an HMOVE'd player. What that fixture could not answer is the word **"centered"**,
  which is a claim about width. Measured 2026-09-03 across three widths: **+4 at NUSIZ 1x** (an 8-clock
  player, so that IS the centre), **+6 at 2x** (16 clocks; the centre would be +8) and **+10 at 4x**
  (32 clocks; the centre would be +16). Centred holds at 1x only. The snap fires when the player's scan
  counter reaches a particular pixel — 2 at 1x, 4 at 2x, 5 at 4x
  (`Gopher2600 hardware/tia/video/player.go:776`) — so it tracks a pixel index, not a width. The lock
  must be **held for a full scanline**; locking and releasing inside one line never snaps.
  `roms/techniques/bullets.asm:3` states +4 and is right for the 1x it uses.
  〔Stella PG for the mechanism; the width dependence is ours〕
  `→ roms/litmus/litmus_resmp_width.asm` / `internal/emu/resmpwidth_test.go` (4 gradings,
  3 negative controls: calling 2x the centre fails by name; a lock released inside one line fails three
  of the four; a missile that does not track the sweep fails)

## 3. Sprites (players)
- ✅ GRP bit order (D7 left), row order, NUSIZ double/quad/3-copies, REFP, P0+P1 16px combine.
- ✅ **VDEL exact semantics** (Stella PG §6.D — the load-bearing mechanism) — measured 2026-09-03;
  this line was documented-only until then. Each GRP has new+old copies. **Writing GRP0 copies P1's
  new→old; writing GRP1 copies P0's new→old, and also ENABL's new→old.** VDELPx/VDELBL D0=1 selects the
  *old* copy for display. All three confirmed: with VDELBL set and ENABL's new copy on, the ball stays
  **dark** after a GRP0 write and **lights** after a GRP1 write — two bands one instruction apart. Both
  players show their old byte, each latched by the **other** register.
  〔Stella PG §6.D; engine `hardware/tia/video/video.go:234-238`〕
  `→ roms/litmus/litmus_vdel_cross.asm` / `internal/emu/vdelcross_test.go` (3 gradings, 2 negative
  controls). The fixture latches every old copy to zero on entry: without that, the ball's old copy
  survives from an earlier frame and band B passes on stale state — measured, and the reason the
  first negative control did not fire.
  📖 **"New" and "old" are Atari's own words.** Kevin Horton, 2001, after a first look at five sheets
  of the TIA schematics: *"I saw the player graphics registers, specifically the "new" and "old" copies
  (their term)"*; how they are built he only guessed and left open — *"I \*think\* they use a 2 level
  deep \* 8 bit shift register for this (will need to do more reading later)"* 〔stella-list
  `200109/msg00291`〕. So the names used above are the schematic's. **Cited only, not
  verified** — no schematic was read here.
  📖 **What that costs a kernel that writes each GRP only on its own player's lines.** Roger
  Williams, 2001, of such a kernel: *"If you turn on VDEL for a player in this kernal, it doesn't
  appear at all if it's not positioned within the other player 's and it bleeds down to the bottom of
  the screen if its bottom is below that of the other player"*; a kernel where VDEL works is, in his
  words, one where *"you have to write both GRPx all the way down the screen"* 〔stella-list
  `200110/msg00294`〕. Both symptoms are what the cross-latch above predicts when the other GRP is
  not being written (read here, not stated there). **Cited only, not verified** — no such kernel was built here.
- ✅ **Missiles have no vertical delay** — measured 2026-09-03; this line was documented-only until
  then. Read against the ball, which does have one: with every VDEL bit set and both objects enabled on
  the same line and no GRP write after, the **missile lights and the ball stays dark**. Two controls make
  that readable — with VDELBL clear both light (so the fixture does enable both), and with VDELBL set
  plus a GRP1 write both light (so the ball was waiting on a latch, not broken). There is no VDELM
  register and no new/old pair for a missile, which is why in a 2LK it starts on the line it is enabled.
  〔Stella PG; pairs with `litmus_vdel_cross`〕 `→ roms/litmus/litmus_missile_novdel.asm` /
  `internal/emu/missilenovdel_test.go` (3 gradings, 2 negative controls)
  📖 **What it costs a kernel with flickering missiles.** Thomas Jentzsch, January 2003, on the Death
  Derby kernel he was writing, asked whether the HMOVE comb could be kept to every other line so the
  sprites under it show: *"Since there is no VDEL for the missiles, I have to do the HMOVE exactly in
  sync with the missile pattern. With flickering missiles you still would get up and down jittering
  HMOVE blanks, which IMO would look quite bad. Hm, maybe by replacing one missile with the ball (using
  VDELBL)? Must think about that...."* 〔stella-list `200301/msg00222`; the question is
  `200301/msg00217`〕. The comb is measured (§2, HMOVE mechanism); the jitter and the ball idea are his,
  for that kernel. **Cited only, not verified.**
- ✅ Moveable-object writes are shear-safe at CPU cycles 0–22 of the line — closed by derivation from
  verified constants (any write completing by cy 22 precedes every draw start: (X+68)/3 ≥ 22.67 even at
  X=0) plus litmus_48px6's measured mid-line GRP choreography (writes landing in copy gaps).
  📖 **Shearing in shipped cartridges.** spiceware's definition is wider than this window: it *"can occur
  when a TIA register is updated during the visible part of the scanline"*, any register. His example
  is a colour register — the right edge of Air-Sea Battle's background gradient. The thread's own case
  is Video Olympics games 9–12, **and only while the paddle is in the lower half of the screen**; a
  second poster saw *"something similar"* on a plane's wing in Skydiver 〔AtariAge `topic/300648`〕.
  **Cited only, not verified** — none of the three was run here. The
  position condition matters for reproducing Video Olympics: the original shears there too, so a
  shear in the same place is not by itself a defect of the reproduction.
  📖 **What a `GRP` write in the middle of a copy looks like, in one report.** B. Watson, 2001, on a
  six-digit score kernel he had not yet got right: *"one of my STA GRP1's happens in the middle of
  drawing player 1, so the left half is the old data and the right half is the new"* 〔stella-list
  `200108/msg00595`〕 — the shape §4 records for a late playfield write (old bits left, new bits right),
  here on a player. He does not say whether he saw it on a console or an emulator, and no `GRP` write
  inside a copy was graded at the pixel here (`litmus_48px6` lands its writes in the gaps between
  copies). **Cited only, not verified.**
- ⬜ 48px kernel GRP write windows: **no local source documents the cycle map** — derive ourselves (the
  recipe exists in score6.asm: NUSIZ=3-close, RESP0/RESP1 3 cycles apart at ~cycle 26+, HMP1=$10, VDELP both
  on, 6-store choreography, font `align $100`).
  📖 **Correction, 2026-09-30: a local source does document it.** Erik Mooney's 1997 walk-through of
  Okie Dokie's routine gives the map instruction by instruction — cycle, pixel, and the contents of all
  four registers (new and old copy of each) after every store 〔stella-list `199704/msg00137`〕. P0 at
  pixel 123 and P1 at 131, counted **including HBLANK** (visible 55 and 63 in our coordinates); the three
  preloaded digits go in at cycles 71 (previous line), 8 and 16, and the last four stores complete at
  **44 / 47 / 50 / 53**, landing 1, 2, 3 and 4 pixels after digits 2, 3, 4 and 5 start to draw. The
  fourth store's value is irrelevant — it exists only to copy GRP1 into GRP1's old register.
  **Cited only, not verified**: his pixel column is cycle × 3, which ignores the TIA's write delay, so
  the margins are his arithmetic, not a measurement.

## 4. Playfield
- ✅ PF0/PF1/PF2 bit order; CTRLPF D0 repeat/reflect; per-scanline colors.
- 📖 **Asymmetric-PF rewrite windows — definitive tables exist** (woodgrain `Playfield_Timing.html`,
  derived from AtariAge thread 149228). Conservative windows (CPU cycle after the store completes,
  WSYNC=0; `*`=previous line): repeated mode — LPF0 53\*–21, LPF1 64\*–27, LPF2 75\*–37, RPF0 27–48,
  RPF1 37–53, RPF2 48–64. Reflected mode — **RPF2 must complete exactly at cycle 48**. Mid-register late
  writes split *per pixel* (old bits left, new bits right) — well-defined, great litmus predicate.
  A second, independent source for the reflected-mode 48, with cycle annotations in shipped code:
  Stay Frosty's kernel, `stx PF2 ; 3 48 <- must be at 48` — *"Any sooner or later and the display will be
  incorrect"*. spiceware moved that game from repeated to reflected asymmetric **to save cycles and RAM**
  〔AtariAge `topic/254684`〕. **Cited only, not verified.**
  📖 **The other two reflected-mode rewrites, from one console.** Glenn Saunders, 2001: a posted kernel
  switched to reflected mode *"didn't work for me on a real VCS by just doing that"* — he added NOPs to
  find the PF2 point and moved the PF0 and PF1 rewrites ahead of it — where Roger Williams had seen the
  unmodified code come out right, on StellaX as his reply makes clear (`200109/msg00331`,
  `200109/msg00368`). Saunders: *"According to my math, in an asymmetrical reflected playfield: The STA
  PF2b had to start exactly on cycle 45, done by cycle 47. STA PF0b can be rewritten as early as cycle
  23. In my mod it's at 31. STA PF1b can be rewritten as early as cycle 29. In my mod it's at 38."*
  〔stella-list `200109/msg00333`〕 His PF2 point is the table's 48 if his 47 is the store's last cycle
  and the table's 48 the cycle after it (our reading); for PF0 and PF1 he does not say whether the cycle
  is where the store starts or ends, the ambiguity he raises himself in the same post. **Cited only,
  not verified.**
  📖 **What a late write costs, in pixels, cycle by cycle.** Brad Mott, 1998: *"The delay is either 2,
  3, 4 or 5 pixels depending on when the register is hit. Looks like the delay can be calculated as
  delay[cycle mod 4], where the delay array is given as delay[4] = {4, 5, 2, 3}"*, with one row per
  cycle from 22 (*"No display problems"*) to 75 saying how many pixels of PF0, PF1 or PF2 still show
  the old value, and the caveat *"I'm pretty sure these were for an unreflected playfield"*
  〔stella-list `199805/msg00153`〕. The windows above say whether a write is in time; this says how
  much is lost when it is not. One row meets a measurement here: his cycle 33 reads *"First 20 pixels
  of PF1 not changed"* — 5 of PF1's 8 bits — and `litmus_pf_async`'s write completing at cycle 33
  leaves 5 old bits and 3 new (`docs/verified-coverage.md`). His post does not say whether "cycle"
  is where the store completes, as ours is, so that agreement is one point, not a check of the
  table. **Cited only, not verified.**
- ⚠️ Internal discrepancy found: SpiceWare Step 3 says the left-PF1 window opens at cycle ~66 of the prior
  line; Step 7 annotates ~71. Resolve by measurement; trust the harness.
- ✅ **CTRLPF D1 SCORE / D2 PFP priority / D4–5 ball width** — verified `litmus_ctrlpf` (v1.53.0),
  table row `docs/verified-coverage.md:66`.
  SCORE (D1): left half→COLUP0, right→COLUP1, split at clock 80. Priority (D2): with D2 clear P0
  draws over PF; with D2 set PF draws over P0. **Ball width D4–5 = 00/01/10/11 → 1/2/4/8 px**,
  read back per band (`read_row` rows 106/114/122/128).
  ✅ **SCORE×PFP interaction measured** (v1.53.0): **PFP dominates** — with D2 set, D1 has no
  effect (PF renders in COLUPF on BOTH halves, with priority over players); $02→halves colored,
  $04 and $06→identical COLUPF rendering.
  ⬜ **WHEN a ball-width write takes effect is not measured — and a 1997 source disagrees with our
  engine.** `litmus_ctrlpf` fixes the four widths (`rg -i "delay|latch|mid.?line"` over it returns
  nothing), so we have the values and not the timing. Gopher2600 applies the write immediately
  (`hardware/tia/video/ball.go`: `bs.Size = (value & 0x30) >> 4`). A stella post from 1997 reports
  the opposite — a width write not taking effect **for about eighty colour clocks**, and a width
  changed part-way through a draw rendering as `X......X` rather than as either width. Which is
  right is open: neither has been measured here, and the 1997 report was made against real
  hardware while ours is a reading of the emulator's source.
  Note also that `design-principles.md`'s register-timing rules cover PF writes (2-3 colour clocks
  late) and colour writes (immediate) and say nothing about CTRLPF, so a reader has no reason to
  suspect a difference. To settle it: change the width mid-visible and read back the first x where
  the drawn run changes, with the same change made during HBLANK as the control (2026-09-03).
- ✅ **Asymmetric PF under reflection: writing PF0 twice in one line does show different values
  at the two edges** — verified `litmus_pf0_reflect`, regression-locked
  `roms/litmus/scenarios/pf0_reflect.json`, graded by `internal/emu/pf0reflect_test.go`.
  Under reflection PF0 draws at cols 0-3 and again at 36-39, and a second write between them
  changes the right edge alone. **The window is bounded on the right by the line itself**: the
  right copy is drawn at cy ~70.7-75.7 and the line ends at 76, so no store lands after it —
  the last usable point lands *inside* the copy and splits it old|new, which is where the
  measured step is (`0 0 0 0 0 0 1` over seven five-cycle steps). Two negative controls, both
  fired: widening the probe makes it read both copies (the TIA counter wraps at 160), and
  removing its HMOVE fine-adjust puts it past the split and the step disappears.
  📖 **Still documented-only: that real games do it** (DaveC's Random-Dungeon `_room_loop`) —
  that is a fact about someone else's source, not about the hardware, and the line had the two
  claims under one mark (2026-09-03).

## 5. Collisions
- ✅ 3 of 15 pairs (BL-PF, P0-P1, M0-P0), sticky latches, CXCLR.
- ⬜ remaining 12 pairs.
- 📖 read idiom: one `BIT CXxx` yields two pairs via N and V. The flag semantics are settled in
  the engine's source, not by a litmus: `Gopher2600/hardware/cpu/cpu.go:1262 "case instructions.BIT"`
  loads M, sets `Sign` and `Overflow` from it, and only then ANDs `A` — so N and V are M's bit7/bit6
  (`registers/data.go:73 "IsNegative"` masks `0x80`, `:83 "IsBitV"` masks `0x40`) and do not depend
  on `A` at all. Z is the only flag `A` reaches. That is engine source rather than a measurement, so
  the mark stays 📖 until a litmus sweeps `A` and shows N and V do not move: a single value of `A`
  cannot tell "independent of A" apart from "happened to agree".
- ⚠️ **Read it with `A = $C0`, not `$FF`.** Only D7/D6 of a collision register are driven; the rest
  of the byte is the last value the CPU put on the bus (`Gopher2600/hardware/memory/memory.go:189
  "data |= mem.LastCPUData & ^mem.DataBusDriven"`), which is why
  `roms/litmus/scenarios/litmus_cxclr.json` asserts 178 and 50 rather than 128 and 0 (the low bits are the
  last byte on the bus before the read, here its zero-page address `$32`). With `A = $FF`,
  Z answers "is the whole byte zero" and so moves when the address the read uses changes, with no
  change in TIA behaviour at all. With `A = $C0` the residue is masked and Z becomes a third useful
  predicate — "neither of these two pairs collided" — so one `BIT` yields three tests, not two.
  With `A = $00`, Z is always 1 and carries nothing.
- ✅ **flicker collision attribution** (za2600 `EN_LAST_DRAWN`) — verified `litmus_flicker_attrib`,
  regression-locked `roms/litmus/scenarios/flicker_attrib.json`, graded by
  `internal/emu/flickerattrib_test.go`. With **CXCLR strobed every frame**, the latch read in a
  frame belongs to the object drawn in **that** frame: over eight alternating frames the latch
  column and the ROM's own record of what it drew agree cell for cell, and inverting the phase
  inverts the latches, so frame parity is not the cause. **Without CXCLR the attribution is lost** —
  the same eight frames all read set. The line said "a verifiable pattern *once we do flicker*"
  while `flicker_multiplex` had existed since technique #10 and touched no collision register at
  all: **the condition had been met and the sentence had not noticed** (2026-09-03).
  ⬜ The control above needs a latch to survive a frame boundary, which nothing measured —
  `litmus_cxclr` takes all three of its snapshots inside one frame and strobes CXCLR every frame,
  so a latch never gets the chance there. This ROM measures it first, in its own group 1, so the
  control rests on our measurement. What stays open is whether **real hardware** holds a latch
  across a frame boundary; this is Gopher2600's behaviour.

## 6. Audio
- ✅ AUDC/AUDF/AUDV register readback; audio digest golden.
- 📖 **Complete AUDC table consolidated** (Slocum guide v1.02 — held locally, authoritative; Stolberg's
  frequency/waveform guide; Stella PG): usable voices — Square(4), Bass(6), Pitfall(7), Noise(8),
  Buzz(15), Lead(12), Saw(1), Engine(3). Pitch: `f = base/(AUDF+1)/D`, base ≈ 31,399.5 Hz NTSC
  (clock/114, 2 samples/line), CPU-clock modes (12–15) ÷3; D = 2/31/31/511/93/6/15/465. PAL ≈15.9 cents
  flatter (from the two clocks in `pkg/audio`). Slocum's three tuning setups (which (AUDC,AUDF) pairs are in tune) are transcription-ready
  for `pkg/audio`.
  ✅ **The "duplicates" are two different things — measured** (`docs/verified-coverage.md:108`): the
  sources list {0,11} {4,5} {6,10} {7,9} {12,13} as one set of duplicates. That is right about
  **tuning** and wrong about **samples**: only {0,11} {4,5} {12,13} are sample-identical, while
  **{6,10} and {7,9} are inverted twins** — same period and tuning, complementary hi/lo duty, so
  identical to the ear but a different sample sequence (`pkg/audio/audio.go:48-50`; the assertion is
  the duty-sum check in `internal/emu/emu_audiocap_test.go:134`). Consequence: any sample-level
  comparison — a golden audio digest, a waveform diff — reports 6 vs 10 and 7 vs 9 as **different**,
  and that difference is correct, not a defect. `audio.Canonical` folds all five pairs for
  classification; it does not make the samples equal.
  ✅ **Pitch formula measured** (`docs/verified-coverage.md:109`): `base/(AUDF+1)/D` confirmed by
  raw-sample capture (square 30/62, lead 90, bass 310).
- 📖 SFX recipes (Slocum): kick=Buzz@30, hi-hat=Noise@0 for 1 frame, snare=Noise@~8 or Buzz@~6
  (*"The snare pitch is flexible"*; Buzz@~6 is **Cited only, not verified**); arpeggio/echo/
  portamento patterns. Driver economics: ~400–500 cycles/frame, 600–2000 bytes ROM (Sequencer Kit).
- ✅ **The pitch table is measured against the machine at 330 of its 512 (AUDC,AUDF) points**
  (`TestEveryPitchTheHardwareHasMatchesTheFormula`, 2026-08-11). Two-sided: an exact sample-for-sample
  repeat at `(AUDF+1)×D` (which alone cannot fail a formula returning a multiple) plus autocorrelation
  finding nothing shorter (which alone is a similarity, not an equality). Skipped and counted: 96 pitchless
  (DC, noise), 86 too long to hold 8 cycles in 30 frames. Negative control: divisor 31→30 fails 128/330.
- ✅ **All nine pitched waveforms characterised, and AUDF proved a pure time scaling**
  (`TestAUDFScalesTheWaveformAndNeverChangesIt`, 2026-08-11). Run lengths within one cycle,
  normalised by `(AUDF+1)` — identical at every AUDF up to rotation, exact integer equality,
  pinned as a golden. Shapes (summing to the divisor by construction):

  | AUDC | name | D | runs | shape |
  |---|---|---|---|---|
  | 4 | square | 2 | 2 | `1 1` — a true 50% square |
  | 12 | lead | 6 | 2 | `3 3` — a true 50% square |
  | 6 | bass | 31 | 2 | `13 18` — an **asymmetric** 41.9% pulse |
  | 14 | low bass | 93 | 2 | `49 44` — an **asymmetric** 52.7% pulse |
  | 1 | saw | 15 | 8 | `4 3 1 2 2 1 1 1` |
  | 2 | rumble | 465 | 16 | `62 44 18 31 31 13 18 13 62 49 13 31 31 18 13 18` |
  | 7 | pitfall | 31 | 16 | `2 1 3 1 1 1 1 4 1 2 1 1 2 2 5 3` |
  | 15 | buzz | 93 | 16 | `5 6 4 5 10 5 3 7 4 10 6 3 6 4 9 6` |
  | 3 | engine | 465 | 128 | (in the test log) |

  Consequence for choosing an instrument: only two of the nine are symmetric squares, and
  the asymmetry of 6 and 14 is why they have their own character rather than being a
  quieter square.
- 📖 **Why the table has this shape: AUDC is two 2-bit fields.** Ron Fries' notes to his TIA sound
  emulator (1997, posted to the list by Eckhard Stolberg; *"From my observations"*): **D1D0 pick a
  clock modifier** — `00`/`01` none, `10` divide by 31 (*"in essence, a 5-bit polynomial with only two
  bits set. The resulting square wave actually has a 13:18 ratio"*), `11` the 5-bit polynomial — and
  **D3D2 pick the source it clocks** — `00` the 4-bit polynomial, `10` the 5-bit, `01`/`11` a pure
  toggle, with `11` also dropping the input clock from 3.58 MHz/114 to 1.19 MHz/114. Two exceptions:
  AUDC 0, where he believes the output is set equal to the volume, and AUDC 8, which clocks the 9-bit
  polynomial. Two of the duplicates fall out of it: in `A` the source and the modifier are both 31
  long, so *"Entry 'A' will then reduce to a pure 'div by 31' output which is identical to entry
  '6'"* — the same period and tuning, though the samples are inverted (the ✅ above measures that) —
  and in `B` *"the output will always be 1"* 〔stella-list `199703/msg00207`〕. Adam Wozniak, 2003,
  spelled out the doubly-polynomial mode 3: clock the 5-bit polynomial every tick, clock the 4-bit one
  *"Each time the 5 bit polynomial changes from 0 to 1 or 1 to 0"*, output the 4-bit one — with run
  lengths `1 2 2 1 1 1 4 3` for the 4-bit polynomial and `5 3 2 1 3 1 1 1 1 4 1 2 1 1 2 2` for the 5-bit
  〔`200311/msg00207`〕. **Those two lists are the same cycles as the shapes measured above for AUDC 1
  and AUDC 7**, differing only in where they start (`internal/emu/audioshape_test.go`), and his mode-3
  construction, computed here over one cycle (no emulator), gives period 465 with 128 runs — AUDC 3's
  D and run count in the table. His mode-15 post the same day keeps the 5-bit polynomial, which he
  writes as `[5 3]` (the 4-bit one is `[4 3]`), and swaps the 4-bit polynomial for *"the divide by 6
  thing"* 〔`200311/msg00208`〕; computed the same way it gives period 93 with 16 runs, and the runs are
  AUDC 15's row in the table up to rotation (a divide-by-4 in place of the divide-by-6, or the 4-bit
  polynomial in place of the 5-bit one, does not match). Both are agreement with this engine, not with
  hardware; the rest of the decomposition is **Cited only, not verified**.
- ⚠️ **`audio.MeasurePeriod` is square-like only, and fails silently.** Mean transition interval × 2 is the
  period only with two transitions per cycle — AUDC 4 and 12, nothing else. On the poly waveforms it returns
  a clean fraction — exactly (runs per cycle)/2, i.e. 4× for saw, 8× for rumble/pitfall/buzz, 64× for engine — that looks like an ordinary number.
  Use `audio.MeasureFundamental`. The four spot checks that stood as this table's verification were three
  squares plus AUDC 6, the one poly waveform whose transition count coincides with its period — by luck
  exactly the cases the broken measure could handle, which is why they stayed green.
- ⚠️ **The audio digest cannot verify pitch** (it's a hash, not a measurement); the sweep above is what does.
- ⚠️ slocum-tracker's default export has a comment/data mismatch (Engine slot emits 14 not 3) — check
  `soundTypeArray` on imported songs.

## 7. Input
- ✅ SWCHA joystick bits (P0 high nibble R/L/D/U, 0=pushed) — verified `litmus_input` (v0.42.0).
  **P1 is the low nibble, same order** — from the engine's source, not measured: `litmus_input` drives
  P0 only, and `Gopher2600/hardware/riot/ports/ports.go` shifts player 1's data `>> 4` into bits 3–0.
  **Not verified.** Consequence: one `lda SWCHA` serves both players, so changing one player's stick
  (a one-player hack) means finding which nibble each read path keeps — easy if SWCHA is read once per
  player, hard if not 〔AtariAge `topic/279174`〕.
- ✅ **SWCHB console switches** — verified `litmus_swchb` (v1.46.0): D0 RESET / D1 SELECT
  (active-low), D3 color/BW, D6/D7 P0/P1 difficulty. Driven via `SetPanel`
  (reset/select/color/p0pro/p1pro) + scenario panel inputs.
- ✅ INPT4/5 fire: D7, 0=pressed; **VBLANK D6=1 latch mode** — verified `litmus_input` (v0.42.0).
  Test with N flag, never Z (bus noise in low bits).
  📖 **The latch is not a debouncer.** B. Watson, 2001, whose colour-picker ROM's fire button *"still
  acts wonky on the Atari"* once he had an EPROM cart: *"What's the normal way to debounce the fire
  button? I had been trying to use latched input mode, but I found this was worse, not better
  (apparently, when I clear the latch, then set latch mode, it latches the bounce!). This version just
  checks the raw input line every 16 frames, no latching going on at all."* 〔stella-list
  `200109/msg00222`〕 Reading the button once a frame and comparing it with the last is in
  `docs/techniques/game-states.md`. The latch measured above holds a press; no bouncing contact was
  simulated here, and the cause is his *"apparently"*. **Cited only, not verified.**
- ✅ **The RIOT data registers decode A0 and A1 and ignore A3 and A4** — verified
  `litmus_riot_mirror` (`internal/emu/riotmirror_test.go`, 2026-09-06). `$0288`, `$0290` and
  `$0298` all read SWCHA, and `$028A` reads SWCHB, in every input state. So an address inside
  the RIOT window that looks like a register nobody documents is a **mirror of one of the four**,
  not a discovery. This matches the only hardware measurement of it the list produced: Eckhard
  Stolberg reported `$288` reading back as the port on a 7800 in 2600 mode, after the person
  asking said an emulator would not settle it — found by the mailing-list distillation.
  **Measured in four input states, and the reason is in the ROM's header:** the first version read
  the ports once at reset, before any input was applied, and reported `$FF` at every address —
  which is what a working mirror and a dead decoder both look like from one sample. The test's
  control is that **A1 still discriminates** (SWCHA `$FF`/`$BF`/`$DF`/`$F7` against SWCHB `$3F`),
  so the agreement is a fact about A3 and A4 rather than about a decoder that stopped answering.
  Negative controls: pointing one mirror at a real other register (`$0284` INTIM) fails it, and
  making SWCHA constant by turning SWACNT to outputs trips the distinct-values guard.
  📖 **The decode rule the mirrors follow — the 6532 data sheet's operation table**, which Bradford
  Mott posted in 1999 to answer what `$029C` and `$029D` are 〔stella-list `199901/msg00141`〕. With
  RS' = 1 (I/O, not RAM): **A2 = 0 selects the ports** — A0 = 1 a data-direction register, A0 = 0 an
  output register, A1 = 1 port B, A1 = 0 port A, A3 and A4 don't care; **A2 = 1 selects the timer** —
  a write with A4 = 1 sets it, A1A0 choosing 1T / 8T / 64T / 1024T and A3 its interrupt enable; a read
  with A0 = 0 is *Read Timer* (A3 again the interrupt enable), with A0 = 1 *Read Interrupt Flag*; a
  write with A4 = 0 is *Write Edge Detect Ctrl* (A1 the PA7 interrupt enable, A0 negative or positive
  edge). `litmus_riot_mirror` measured the port-register reads (A3 and A4 ignored, above), and the
  timer addresses used in this file — `$0284` INTIM, `$0285` TIMINT, `$0294`–`$0297` — fit the table;
  the rest is **Cited only, not verified**.
  📖 **The two timer read addresses are also written, and the flag behind one of them reaches no pin.**
  Chris Wilkson's 2002 memory map: `$0284` is *"INTIM (read), edge detect control (write)"* and `$0285`
  *"read interrupt flag (read), edge detect control (write)"* — the write side being what the table
  above gives for A4 = 0 — and *"PA7 of the RIOT can be used to generate an interrupt. Unfortunately,
  the 6507 has no interrupt inputs. So the /INT pin of the RIOT is left unconnected. But an internal
  flag is still generated and can be read and cleared normally"* 〔stella-list `200207/msg00178`〕. That
  flag is the D6 that §1 reads at power-on; no ROM in this repository writes `$0284` or `$0285`
  (`rg -i '(st[axy]|sax)\s+(\$0?28[45]|INTIM|TIMINT)\b' --glob '*.asm'` → 0 on 2026-10-02).
  **Cited only, not verified.**
- ✅ Paddles INPT0–3 dump/charge — verified `litmus_paddle` (v0.54.0; transfer curve measured).
- 📖 **SWACNT/SWBCNT DDRs** — documented only. **"Rarely game-relevant" was withdrawn 2026-09-03:**
  it was true of *our* ROMs and not of the games. A commercial title writes one: Combat stores
  `#$10` into `SWBCNT` at start-up (`A9 10 8D 83 02` at $F00A of `sandbox/studies/combat/Combat.bin`). `docs/casebook.md`
  once read that ROM as gating the joysticks through the DDR; it does not (corrected 2026-09-28), but the DDR write is real. What is true is the narrower statement: **no ROM here wrote
  SWACNT or SWBCNT at all** (`rg -l "SWACNT|SWBCNT" roms --glob "*.asm"` → 0 on 2026-09-03; the same command returns 3 on
  2026-09-28 — `litmus_swacnt.asm`, `litmus_swacnt_delay.asm`, `litmus_riot_mirror.asm`, added since), and none needed to,
  because the engine resets the RIOT chip memory to zero (`hardware/memory/vcs/riot.go` `Reset`),
  which is all-inputs, and `deriveSWCHA` then returns the peripheral value unchanged. So a DDR
  litmus cannot confirm existing practice — it has to **drive a port as an output**, which no ROM
  here did then.
  **The list went further and we could not follow it, 2026-09-04.** A 2004 post reads the Stella
  Programmer's Guide saying SWCHB *"is hardwired to be input only"* and asks the obvious question —
  *"then why would they have the SWBCNT register?"* — then reports **Air-Sea Battle setting D4 of
  SWCHB as output**, and **Combat doing the same with its comments mislabelled**: *"it says this
  stops the response from the joysticks but it doesn't"*. If that holds, "rarely game-relevant" was
  wrong about two of the best-known cartridges there are.
  ✅ **Combat verified from its own bytes, 2026-09-04.** `sandbox/studies/combat/Combat.bin`, first
  eighteen bytes: `78 D8 A2 FF 9A A2 5D 20 BD F5 A9 10 8D 83 02 …` — `SEI / CLD / LDX #$FF / TXS /
  LDX #$5D / JSR $F5BD /` **`LDA #$10 / STA $0283`**. One write to SWBCNT, at file+`$000C`, of
  **`$10` — D4 alone**, which is the same bit the 2004 post reports Air-Sea Battle setting. Zero
  writes to SWACNT (`8D 81 02`) anywhere in the 4K image. Decoded from the bytes here; no annotated
  disassembly was opened, which keeps this inside the clean-room line — a ROM image carries no
  interpretation, and reading meaning out of it is the skill.
  📖 **What the DDR write is FOR is still open.** The 2004 thread's question — *"does anyone know if
  SWBCNT has any use?"* — got only a general answer, about spare bits, that does not name either game
  (`200407/msg00042`, below), and the asking post reports Combat's own comment claiming
  the write stops joystick response **when it does not**. So we know two commercial titles do it and
  we do not know why, which is a sharper open question than the one this line started with.
  📖 **One reason given for input-only bits is the board, not the chip.** Mark De Smet, 2000,
  describing the PIA: *"2  8-bit input/output ports(all configureable in/out, but the 2600 circuit board
  requires some to be input to prevent contention)"* 〔stella-list `200006/msg00086`〕. He does not say
  which pins, and the Star Raiders account below has all eight of port A driven. **Cited only, not
  verified** — no schematic was read here.
  📖 **One use is on the list, for a third cartridge.** Manuel Polik, reverse-engineering Star Ship in
  2002, found the same `LDA #$10 / STA SWBCNT` at start-up — commented, across the two lines, *"Useless port
  direction instructions..."* on the `LDA` and *"...on the 2600 (?)"* on the `STA` — and, later, a value stored to `SWCHB` and read back.
  Eckhard Stolberg: *"There are three unused bits in SWCHB. If you set these to output you
  can read back whatever you write to SWCHB. So you could consider them as three bits more RAM, if you
  will. In the upper example it seems that the state of "gameOffBool" is stored in bit D4 of SWCHB"*
  〔stella-list `200208/msg00253`, `200208/msg00269`〕; Glenn Saunders, surprised by that answer:
  *"Wow. I thought you couldn't get away with using any of the VCS registers as general RAM storage. I
  remember asking about this a long time ago and the general concensus was that it wouldn't work"*
  (`200208/msg00279`).
  The one reply in the 2004 SWBCNT thread says the same, with a test run: zu03776, *"You can store #$34
  into SWBCNT and get three extra bits of RAM in the unused bits of SWCHB"*. His ROM stores the
  scanline counter into `SWCHB` on every line and draws the read-back as `GRP0`: on Stella 1.2 *"the
  P0 sprite is solid"*, and *"After compiling MakeWav for the Mac, and putting it on the Supercharger on
  my Atari 7800, I received a line with holes in it, as expected"* 〔stella-list `200407/msg00042`〕 — a
  7800, not a 2600.
  The three are **D2, D4 and D5** — the bits the
  switch list above leaves out. A 2010 post keeps two flags of its own (a 7800 detected at power-up,
  and the B&W switch's flip state) in those three bit positions of the RAM byte that holds the
  previous switch state, masking with `#$34` and `#$CB`, and adds in one line that *"That could be
  shorter if you are redefining some of SWCHB as usable ram (via SWBCNT)"*, without showing that
  version 〔AtariAge `topic/168437`, Nukey Shay〕. Nothing here writes `SWBCNT` and reads `SWCHB` back
  (`rg -i 'SWBCNT|\$0?283' internal roms cmd pkg` finds only a comment; `litmus_swacnt` drives port
  A), and the D4 reading is Stolberg's *"it seems"* about Star Ship, not an answer for Combat or
  Air-Sea Battle. **Cited only, not verified.**
  📖 **Driving a bit that a switch is wired to is the case to keep out of a port-B output test.** Kevin
  Horton, 2001, answering whether setting the console-switch port to output could damage the console:
  *"there is no protection against it, though I doubt it'd do any damage. The 6532 will get hotter than
  a pistol tho if you have console switches on and the port set to output trying to output a logic "1".
  (The switches pull to ground only) So it isn't recommended."* 〔stella-list `200111/msg00482`〕
  groovybee's 2012 warning about a difficulty switch in B 〔AtariAge `topic/197100`〕, quoted in full in
  `known-traps.md` (*"A port bit driven as an output…"*), describes the same mechanism and
  ends *"This isn't good for RIOT and is best avoided"* rather than doubting the harm. Star Raiders,
  in the next entry, drives all of port A high against joystick switches that ground it — the
  arrangement Horton describes, shipped (our reading). So the litmus this entry says a DDR test needs
  belongs on D2, D4 or D5 (our reading). **Cited only, not verified.**
  📖 **Port A (SWACNT) as an output does have known uses, each for a peripheral** — the answer to a
  2004 thread asking whether anything but a development tool drives it 〔stella-list
  `200404/msg00412`〕: the **keyboard controller** is an x-y grid, the four RIOT lines drive x[3:0] and
  the trigger plus two paddle inputs read y[2:0], one x line at a time 〔`200404/msg00419`, Chris
  Wilkson〕; **Kid Vid** pauses its tape, **Mindlink** is told to send new data, the **Compumate**
  keyboard bank-switches its cartridge through the joystick port, and **Star Raiders** sets all eight
  pins to output — it still reads its joystick because it drives the pins high and a pressed direction
  grounds one, which reads the same as driving it low 〔`200404/msg00421`, Eckhard Stolberg; the Star
  Raiders account again in `200409/msg00070`〕. **The negative example: paddles do not use the port as
  an output** — they dump their capacitors through a VBLANK bit (`200404/msg00421`). The same thread
  also claims **Combat** drives port A; the byte count above found **zero** SWACNT writes in Combat (its one DDR
  write is to SWBCNT), so that claim does not survive. **Cited only, not verified.** None of this says what Combat's and
  Air-Sea Battle's **SWBCNT** D4 is for, which stays open.
  **A search note:** this was recorded as unverifiable an hour earlier because `reference/` holds no
  Combat ROM. It is in `sandbox/`, a sibling repository — **the search was scoped to one of the four
  and the conclusion was stated as if it covered all of them.**
  ✅ **The truth table is measured, 2026-09-04** — `roms/litmus/litmus_swacnt.asm`,
  `internal/emu/swacnt_test.go`, regression-locked `roms/litmus/scenarios/swacnt.json`. This is the
  first ROM here that drives a port as an output, which is what the line above said was missing:

  | SWACNT | wrote | reads back | |
  |---|---|---|---|
  | `$00` | — | `$FF` | the peripheral owns the port (idle stick) |
  | `$FF` | `$A5` | `$A5` | every bit driven; the write reads back |
  | `$F0` | `$5A` | `$5F` | **split** — high nibble from the latch, low from the peripheral |
  | `$00` | — | `$FF` | handing it back is clean; the latch does not linger |

  The third row is the one that goes beyond dumping: **half the port can be an output while the
  other half is still an input**, which is what a two-console link needs. The fourth says the
  direction is reversible — a program that drives the port can give it back. Neither is in the
  Programmer's Guide's one-line description of the register. Negative controls: not setting the
  direction makes the `$A5` write vanish; making band 3 fully output removes the split.
  ⚠️ **This is the engine's answer, not the console's, and the engine says so itself.** The
  Programmer's Guide requires **400 µs — 477 cycles, 6.3 scanlines — between writing this port and
  reading it**, and every band above reads on the very next instruction (~4 cycles).
  `Gopher2600/hardware/peripherals/controllers/keypad.go`: *"We're not emulating this here … I'm not
  sure what's supposed to happen if the 400ms is not adhered to. **!!TODO: Consider adding 400ms
  delay for SWACNT settings to take effect.**"* Band 5 repeats band 2 *with* the wait and gets the
  same byte, which is the evidence that no settling time exists here. The list answers the engine's
  TODO where the engine could not — Chad Schell, running serial off the port at 38.4 kbps
  (`200111/msg00194`): *"If you only read the port, and thus don't change it's configuration, the
  400 uS delay does not apply"* — so **the constraint is about changing the direction**, which is
  exactly what every band does. Found by the distillation (helper-1).
  **What drove this was a working report, not a document.** stella-list `poor-man-s-cart-dumper`
  (2005-08) is a cartridge dumper in which the 2600 talks serial out of a joystick port at 1200 baud,
  and a third party reports getting it running on hardware. Only the fact that it writes SWACNT was
  taken from that post; no code was (clean-room). Found by the mailing-list distillation (helper-2).
  📖 **The bit time is counted in scanlines.** A send-only routine posted in 2005 (Glenn Saunders,
  trimmed from a 7800 dumper's source) waits on `WSYNC` per bit, with the comment *"6.5 lines = approx
  one 2400 baud bit"* / *"13 lines = approx one 1200 baud bit"*, and makes 2400 by alternating 7 and 6
  lines 〔stella-list `200508/msg00061`〕. Computed here from `pkg/audio`'s clocks (76 cycles a line, the
  CPU at colour clock ÷ 3; no emulator): 13 NTSC lines are 828.0 µs, 1207.7 baud, 0.64 % fast; on
  PAL's clock 835.7 µs, 1196.7 baud, 0.28 % slow. **Cited only, not verified** — no serial line was
  driven here.
  📖 **Where the fourth row of the SWACNT table above has been seen to differ: z26.** Alex Herbert,
  2005, found while working with the AtariVox that MGD's high-score table *"should not show without an AtariVox
  connected - works fine on the real hardware and in Stella, but it still displays in Z26"*, and gave a
  detector: `lda #$00 / sta SWCHA / ldx #$ff / stx SWACNT / sta SWACNT / lda SWCHA / beq z26_detected`
  — *"That's not exactly what I have in my code, but if I understand correctly why it happens that
  should do the trick"* 〔stella-list `200507/msg00017`〕. Read against the table: a port handed back to
  the peripheral after its latch was written reads the latch on z26 and not on the console, and the
  engine gives the console's answer in the fourth row (read here, not stated there). The seven
  instructions were not run here. **Cited only, not verified.**
  📖 **A peripheral that cares how fast the pins change.** The AtariVox keeps high scores in an I²C
  EEPROM that a ROM drives through the RIOT. Bob Montgomery, 2005, could not get a save to stick —
  every read came back `$FF` or close to it, and nothing he wrote changed it — using the AtariVox driver
  pack's `i2c.inc` in a kernel of its own with `VBLANK` left on, seven bytes from EEPROM address
  `$00C0` up, tested on a Cuttle Cart 〔stella-list `200505/msg00203`〕. The driver's author, Alex Herbert:
  *"It's pretty timing sensitive"* — *"if it goes SCL_1 --> SCL_0 (or SCA_IN --> SCA_OUT) too quickly,
  the i2c chip doesn't always see it. (The rise time is longer than the fall time because the RIOT only
  has single-transistor outputs.)"*, and *"I thought we'd found that 12 clock cycles between state
  changes on each pin was enough, but maybe it needs more"* (`200506/msg00039`). Aaron, whose Fall Down
  used the same routines: *"it might be that Fall Down works only by some strange coincidence"*
  (`200506/msg00003`). The thread ends without a confirmed fix, and nothing here measures a pin's rise
  time. **Cited only, not verified.**
  ⬜ **The power-up value is still the engine's choice, not a measurement.** `Reset` zeroes the RIOT
  memory explicitly; whether a real 6532 clears its DDR on RES is not established here. The table
  above measures what a *write* does, which is a different question from what reset leaves behind.
  📖 **The list disagreed about it and did not settle it — cited only, not verified.** Eckhard
  Stolberg: *"While the 6507 will be reset at power-up, the RIOT will not. It will remain it's state for
  a short while after you turn the console off"* 〔stella-list `200409/msg00060`〕. Alex Herbert replied
  that the RIOT shares the CPU's reset line, quoting the 6532 datasheet: *"a low /RES input causes a
  zeroing of all four I/O registers. This in t[u]rn causes all I/O busses to act as inputs"*
  〔`200409/msg00119`〕. The last word in the thread qualifies rather than decides: *"When turning power
  off and on quickly reset may not get low"* 〔`200409/msg00121`, Edwin Blink〕. So the engine's zero
  agrees with the datasheet as quoted, and whether a real console's RIOT is reset at power-up is still
  disputed — the ⬜ stays. Stolberg's advice for a binary that
  may start after a 7800 BIOS, a Supercharger or a Cuttle Cart loader has already run is the same in
  general form: initialise everything your code depends on 〔`200409/msg00070`〕.
  📖 **On a 7800 the RAM is not random at power-up either.** Eckhard Stolberg, 2002: *"On the 7800 you
  always go through the BIOS before the cart gets access to the machine. And all 7800 BIOS versions I
  have seen use the same code in RIOT RAM to switch the 7800 into 2600 mode. So on a 7800 you can't get
  random values from RIOT RAM."* 〔stella-list `200211/msg00183`〕 This engine starts RAM at zero
  unless its `RandomState` preference is set (`internal/emu/randomstate_test.go`). **Cited only, not
  verified.**

## 8. 6502/6507 precision
- ✅ cycle accounting (76/line; WSYNC-stall exclusion).
  📖 **The write to `WSYNC` has to end by cycle 76.** C. Bond, 2003, asked for the last cycle at which
  a `WSYNC` is still honoured on the current line; Thomas Jentzsch: *"The write to WSYNC must \*end\* at
  cycle 76."* Bond: *"a STA Z.P. (3 cycles) should start at cycle 74, but a STA Z.P.,X should start at
  cycle 73. Correct?"* — Jentzsch: *"Yes, yes and yes. :-)"* 〔stella-list `200309/msg00188`,
  `200309/msg00192`, `200309/msg00196`〕. Their numbers count where the store starts and where it ends;
  they are not converted to any convention used elsewhere in this file, and no late `WSYNC` was run
  here. **Cited only, not verified.**
- 📖 **Why 6502 tables are used for the 6507.** Asked in 2002 whether the cores are *"100% identical
  including any non documented opcodes"*, Chris Wilkson: *"I believe the silicon is the same, just some
  of the pads are not bonded out to the IC package pins"* 〔stella-list `200212/msg00192`〕; Roger
  Williams: *"Yes, they are identical. The 6507 package was created strictly to save space on the PCB,
  in those days it did not make sense to create a new die and mask for such an "improvement" since this
  was much more expensive than the package design"* (`200212/msg00216`). Mark Graybill corrected one
  detail of that reply — the C64's chip is a 6510, so the comparison is with the 6502 *"in an AppleII,
  Atari 800, or Vic-20"* (`200212/msg00217`). That is the ground this section stands on when it takes
  6502.org's numbers for the 6507. **Cited only, not verified** — `cmd/cpucheck` compares the engine with
  a 6502 netlist, not with a 6507.
- 📖 Page-cross +1 applies to **reads** (abs,X / abs,Y / (ind),Y); **stores are fixed** (STA abs,X
  always 5, (ind),Y always 6); RMW abs,X fixed 7. Branches: 2, +1 taken, +1 page-cross **measured
  from the next instruction's address**. (6502.org.)
  ✅ **Measured for the cases the litmus covers** (`docs/verified-coverage.md:90-91`, `litmus_6502`
  v0.44.0, regression-locked `roms/litmus/scenarios/cpu6502.json`): **LDA abs,X** 4cy → 5cy on a
  page cross (+1); **STA abs,X** 5cy on both sides (**stores really are fixed** — the basis for
  kernel determinism); **BNE** 2 / 3 / 4 (not taken / taken / taken+page-cross); **DCP zp** 5cy
  (which also proves illegal-opcode support). The ROM measures each with a TIM1T=$80 window.
  ✅ **The remaining three, settled from the engine's own instruction table** (2026-09-04,
  `Gopher2600/hardware/cpu/instructions/definitions.json`, 256 entries, grouped by
  `addressingMode` AND `bytes`). This is the table the CPU executes from, so it is a reading of the
  machine we run, not a second opinion about hardware — the litmus stays the authority on what the
  silicon does, and these three agree with it:
  **`(ind),Y`** 16 entries — read 8, **all page-sensitive**, base 5 (→6 on a cross); write 2, fixed
  6; **modify 6, fixed 8**. So "always 6" is true of `STA`/`SHA` and **not** of the illegal RMW
  forms (`slo`/`rla`/`sre`/`rra`/`dcp`/`isc`), which are a flat 8.
  **RMW `abs,X`** 12 entries, `cycles` exactly `[7]`, page-sensitive on none — **fixed 7 confirmed**.
  **Reads through `abs,Y`** 10 entries, **all page-sensitive**, base 4.
  The grouping is the whole trick: the table names both `$B6 ldx zp,Y` and true `abs,Y` as
  `absolutey` and separates them **only by `bytes`**. Counted by mode name alone, `abs,Y` reads look
  like 24 entries with 10 sensitive — "some `abs,Y` reads are insensitive", which is false. Found by
  the mailing-list distillation (helper-1), who **published the wrong count and then corrected it**
  from the same table; re-run here independently and matching in every cell.
  📖 **Still documented-only**: nothing above is a *measurement of hardware* — it is what our engine
  believes. `litmus_6502` covers `LDA abs,X`, `STA abs,X`, `BNE` and `DCP zp` against Stella; the
  other three modes have no litmus band.
  📖 **What the read rule buys a kernel, in one case.** Thomas Jentzsch, 2002, to Ben Larson, whose
  asymmetric-playfield kernel for *Incoming!* counted `LDA (P1Offset),Y` as 5–6 cycles: *"If you align
  (something like: align 256 + 120) the data P1/P2Offset are pointing to, then you can avoid the extra
  cycles for crossing a page. Then all code in your main kernel has a constant timing, which would
  allow you to remove WSYNC and get some more cycles. And that should allow you to add code for one
  missile (perhaps even both), and avoid flicker."* 〔stella-list `200201/msg00061`〕 On 1 February:
  *"It appears I will indeed be able to make the game flicker-less. After fitting the tank graphic
  into one page and making some kernel modifications to accomodate this, I was able to remove the
  WSYNCS in the kernel as well and get enough space for the missile-drawing algorithm"*
  (`200202/msg00011`). **Cited only, not verified.**
- 📖 **NMOS decimal mode: only the C flag is valid** after ADC/SBC (never branch on Z/N/V); D is
  unknown at power-up and survives interrupts → `CLD` in init is mandatory. BCD idiom:
  SED/CLC/ADC…/CLD; multi-byte chains keep the carry.
  **Subtraction is the mirror pair, SED/SEC/SBC…/CLD**: `SBC` subtracts one more when C is clear, so a
  `CLC` carried over from the addition idiom makes every subtraction one too large; with `SEC`, one off
  the tens digit is `sbc #$10` 〔AtariAge `topic/301365`, JetSetIlly's correction of `clc / sbc #9`〕.
  **Not verified** — `litmus_6502` measures the `ADC` side only.
  ✅ **The flag half is measured** (`docs/verified-coverage.md:88`, `litmus_6502` v0.44.0):
  $99+$01 under SED gives **A=$00 (correct)** and the pushed status $BD = **C=1 (correct), Z=0 and
  N=1 (both wrong for the decimal result)** — so "do not branch on Z or N" is our own measurement,
  not just the source's. **V is recorded (0) but nothing asserts it.**
  📖 **Not measured by us**: that D is undefined at power-up and survives interrupts. The `CLD`
  rule is *enforced* rather than measured — `scripts/check_traps.py` errors on an init with
  neither `CLD` nor `CLEAN_START` (see `docs/known-traps.md`), which is a lint, not a hardware fact.
- ✅ **JMP ($xxFF) page bug** — verified `litmus_6502` (v0.44.0), table row
  `docs/verified-coverage.md:89`: the indirect vector's high byte is fetched from **$xx00**, not
  $xx+1:00. `jmp ($F3FF)` lands on the buggy path and the ROM records the marker $92=$A5;
  regression-locked `roms/litmus/scenarios/cpu6502.json`.
- ⚠️ 📖 **BIT-as-NOP reads can strike TIA strobe mirrors — audit `.byte $2C` tricks.** Not measured,
  and **not caught by the linter either**: `scripts/check_traps.py` matches mnemonics
  (`READ_OP`), so a skip written as a raw `.byte $2C` / `.byte $0C` is invisible to it. The one
  such skip in our own tree is `roms/techniques/tia_pcm.asm:89`.
- ⬜ RMW double-write bus behavior on TIA strobes (6502.org silent; needs visual6502/64doc as source).
  📖 **The colour-register side has one report:** *"I tested LSR COLUBK this morning and it produces a 3
  pixel wide line"*, with the use named beforehand — a 3-pixel object inside the playfield, 1 or 2 pixels
  if timed across a pixel boundary 〔AtariAge `topic/238310`, zackattack〕. Whether that was hardware or an
  emulator is not stated. The engine does perform the extra write — the `Modify` path in
  `Gopher2600/hardware/cpu/cpu.go` writes the value it read back before the modified one ("phantom
  write") — so the first colour on the line is **whatever the read returned**, and a read of `$09` is a
  TIA **read** register (INPT1). The poster's own next step was to force that read to `$FF` so the
  two writes that follow could be overridden with any colour — a bus-stuffing plan. **Cited only, not
  verified**; the strobe side stays ⬜.
  📖 **The read registers have a shipped case.** Paul Slocum rebuilt Donkey Kong from a disassembly
  whose `vcs.h` put the TIA read base at `$00`, where the original reads through `$30`; six bytes
  differed, and only after moving the base to `$30` did it work on a real 2600. Thomas Jentzsch found
  the line, `asl CXP1FB`: *"If the base is at $00 this writes to RSYNC!"* 〔stella-list
  `200404/msg00049`〕. Every read register sits at `$00`–`$0D` (the read side `scripts/check_traps.py`
  describes), where the write side is a different register, `VSYNC` … `PF0`, so any RMW on a
  collision or input register through base `$00` writes one of them (read here, not stated there).
  Whether the TIA sees one write there or two is the ⬜ above. **Cited only, not verified.**
  📖 **The strobe side has one report, for `WSYNC`.** Fred Quimby, 2005: *"I had the idea that doing an
  INC WSYNC should give two scanlines, but on a real 2600 it didn't appear to work - I could only see
  one"*; his explanation, *"the READY line on the 6507 only halts it during read cycles. Therefore the
  second WSYNC gets executed before the CPU halts, thus you only get one scanline"*, which he extends
  to a `BRK` or `JSR` with SP at `$02` and qualifies: *"I haven't verified that the above is 100%
  true"* 〔stella-list `200507/msg00190`〕. Hours later: *"I just proved the theory in my last post by
  writing some code that will run on a real 2600 but will crash emulators"* — `sta WSYNC / lda #$FF /
  sta TIM1T / inc WSYNC / lda INTIM / .1 bpl .1` (`200507/msg00191`). In April of the same year he had
  written the opposite inside a VSYNC trick — *"INC does two writes to WSYNC, giving you two
  scanlines!"* — with *"I haven't actually tried this on real hardware, so use at your own risk!"*
  (`200504/msg00010`), so the July report reads as his own correction (our reading). On AtariAge,
  asked whether `INC WSYNC` waits two HBLANKs, rybags suspected the same mechanism: the CPU *"will
  allow any pending writes to occur before recognising it"*, which is *"deliberate to allow such
  read/modify write instructions to be able to complete properly before the CPU is snoozed"*
  〔AtariAge `topic/238310`〕. What this engine does with
  `INC WSYNC` was not measured here. **Cited only, not verified.**
- ✅ **skipdraw/DoDraw is 17 or 20 cycles, not a constant 18** — measured 2026-09-03; this line said
  "constant-18-cycle draw" and added "worth a cycle litmus", which was an accurate self-assessment.
  Timed WSYNC→GRP0 over eight frames of `roms/techniques/vertical_pos_dcp.asm`: **20 cycles on the 80
  lines that draw** (the range branch taken, then `ldx sprDraw` / `lda ArtRev,x`) and **17 on the 1,686
  that skip**. The ROM's own comment already read `~17-20`; the audit line did not. A kernel budgeted at
  a constant loses three cycles on exactly the lines that draw — the tightest ones. The illegal `dcp`
  costs 5 and the emulator runs it, which this fixture also exercises.
  `→ internal/emu/skipdraw_test.go` (1 grading, 1 negative control: asserting 18/18 fails on both paths)
  ✅ **And the three cycles can be bought back — measured 2026-09-04, with the price the source never
  stated.** The list has the answer (2005-02, Thomas Jentzsch, *"NOT skipdraw"*; the follow-up thread
  names it **SwitchDraw**) but recorded only *"it has some disadvantages"*. `litmus_switchdraw` removes
  the branch by letting the TABLE absorb the range test — `sprDraw` walks the whole byte range, so a
  256-entry table with the art at 0..H−1 and zero elsewhere makes "am I in range" an array index:
  `lda #H-1 / DCP sprDraw / ldx sprDraw / lda Art256,x / sta GRP0` = **17 cycles on every line,
  drawing or not.** Equal to the old skip path, three better than the old draw path, and — the point
  for beam racing — *the same number twice*.
  **The price, now stated: 248 bytes.** A 256-byte table instead of 8, **6.1 % of a 4K cartridge**, per
  sprite, not shared between sprites with different art. Against it: 3 cycles × 192 lines = **576
  cycles a frame, 7.6 scanlines' worth**. Which side wins is a budget question and depends on what is
  binding; this file states both numbers and takes no side.
  `→ roms/litmus/litmus_switchdraw.asm` / `internal/emu/switchdraw_test.go`, regression-locked
  `roms/litmus/scenarios/switchdraw.json`. Negative control is the strongest available kind: **the
  same measurement code run on the branching fixture**, which must still report 20 and 17 — if it
  cannot see the difference, the agreement is a property of the measurement and not of the kernel.
  Found by the mailing-list distillation (helper-2 and helper-1, who also found that the first post's
  code is broken — its wait loop branches to itself — so a reader who finds only that message copies
  something that hangs).
  📖 **The source's own numbers, for its branching version.** The first post says the code *"works
  even faster than the original routine (18 cycles for constant heights, 19 for variable heights)"*
  〔stella-list `200502/msg00058`〕. Its corrected listing annotates **15** cycles on each of its three
  paths (`.cont` reached directly, through `.wait`, and through `.switch`) — so as posted it too is one
  number on every line, by balanced branches rather than a 256-byte table; it *"Works only for
  kernels where y (or x) is the row counter (counting down) and doesn't start larger than 127"* (`200502/msg00062`).
  The post naming it SwitchDraw adds that, when the draw routine is longer (*"e.g. updating colors
  too"*), the test shrinks to *"only 5(!) cycles for determining if you have to draw the player or
  not (and saves 4-5 cycles compared to SkipDraw)"* (`200502/msg00077`). **Not verified** — the counts
  are his annotations, summed here, and the posts do not say how the 15 relates to the 18/19.
  📖 **And 17/20 is a property of where the branch goes, not of skipdraw.** `vertical_pos_dcp.asm` puts
  the draw path on the taken branch (`bcs VDraw`). Put the draw path on the fall-through instead, send the
  skip path out of line and back with a `BEQ`, and the two paths cost the same with no table: **19/19**
  for the DCP form, where the skip path loads its zero with `LDA temp1` (a zero-page byte holding 0,
  3 cycles) instead of `LDA #0` (2) to burn the one cycle that balances it, for one byte of RAM; the
  `SEC/SBC/ADC` form posted in the same thread adds up to **20/20** 〔AtariAge `topic/191440`, reveng〕.
  **Not verified** — the counts are the posts' own annotations, including `STA GRP0`, and assume that
  neither branch crosses a page, which a branch to code outside the kernel makes easy to break.
  The list had the polarity move in 2001. Glenn Saunders worried that a skip path which clears GRPx
  forces a `JMP` after the draw path's store; Thomas Jentzsch: the skip path can branch out of the
  kernel and jump back, which *"only works, if the cycles of the branches differ by at least 4 cycles (1
  for the taken branch and 3 for the additional jump)"*, and otherwise *"you can change bcc .skipDraw
  into bcs .doDraw and reorganize your code. This only adds one cycle to the long branch"* 〔stella-list
  `200110/msg00273`〕. So `.doDraw` there is skipdraw with the branch inverted, not a separate routine
  (our reading). **Cited only, not verified.**
- 📖 Mirror templates (woodgrain Memory_Map): TIA at $xyz0 (x even, z∈{0,4}); RAM $80–$FF mirrored
  at **$0180–$01FF — which is why the stack works**, and the mechanism is that the 6507's stack
  pointer is **only eight bits wide** while the address bus is thirteen, so the processor supplies
  `$01` as the upper bits on every stack access — the programmer has no say in it, and the PIA
  being mapped into both pages is what makes the two views the same memory 〔stella 1999-08〕; ROM $1000–$1FFF mirrored at every odd $x000
  (incl $F000).
  ✅ **Two of the three measured** (`docs/verified-coverage.md:27`, `litmus_mirror` v0.49.0,
  regression-locked `roms/litmus/scenarios/mirror.json`): the **RAM mirror holds in both
  directions** — write $5A to $0180, read $5A at $0080; write $A5 to $0080, read $A5 at $0180 —
  and **one TIA mirror**, $0049 → COLUBK, checked by rendering ($84 blue at `read_row(100)`).
  📖 **Not measured by us**: the TIA template as a rule ($xyz0 for x even, z∈{0,4}) — one mirror is
  not the pattern — and the ROM mirroring of $1000–$1FFF at every odd $x000.
  📖 **The chip-select wiring behind those mirrors.** Eric Ball, 2004, answering what `$029E` is: *"The
  TIA !CS0 and !CS3 are connected to A12 and A7, and it fully decodes A0-5. So the TIA is mapped to
  address %0xxxx0x?????? The RIOT CS1, !CS2 and !RS (RAM select) are connected to A7, A12 and A9
  respectively and it fully decodes A0-6. So the RIOT is mapped to addresses %0xxRx1???????"* — so
  `$029E`, with A7 and A9 high, is a RIOT register, which he reads from the 6532 data sheet as `TIM8T`
  with the timer interrupt enabled 〔stella-list `200412/msg00028`〕. That `TIM8T` does not fit the §7
  table: `$1E` is A4 = 1, A3 = 1, A1A0 = `10`, which is **64T** with the interrupt enabled, and the
  engine folds a write to `$029E` onto `$0296`, `TIM64T` (`0x29E & 0x297`, the write mask in
  `memorymap.MapAddress`; read from the source, not run); Alex Herbert, later in the thread, also sets
  `$29e` against `$296` (`200412/msg00035`). The engine's
  `memorymap.MapAddress` sorts addresses the same way — A12 for the cartridge, then A9 with A7 for the
  RIOT's registers, A7 alone for RAM, everything else TIA (read from the source, not run). Inside the
  RIOT's register window the port registers still ignore A3 and A4 (§7, measured), so his "fully
  decodes A0-6" is about which lines reach the chip, not about every combination being a different
  register (our reading). **Cited only, not verified.**
- ✅ **Convention: stack from $FF down (`LDX #$FF/TXS`), variables from $80 up — and the gap is now
  measured rather than hoped for.** `internal/ramtrace`'s activity report prints the stack
  low-water mark and the observed SP range, and it had never been run for this. Our own technique
  ROMs, 2026-09-03 (`go run ./cmd/ramtrace activity -rom <rom>`; SP points at the next free byte,
  so usage is `$FF − low`):

  | ROM | SP low | bytes |
  |---|---|---|
  | `bullets`, `flicker_multiplex`, `two_line_kernel`, `score6`, `paddle_demo`, `procgen_demo` | `$FD` | **2** |
  | `game_states`, `dyn_multisprite` | `$FB` | **4** |
  | `rts_dispatch` | `$F5` | **10** |

  `rts_dispatch` is the outlier by construction — it pushes return addresses as its dispatch
  mechanism, so its stack use *is* the technique. Everything else sits at two or four.
  Shipped games agree, from the list: **Space Instigators uses none, Fade Out and Marble Craze two**,
  and 6-8 is offered as enough for two or three levels of nesting 〔stella 2004〕. So a variable at
  `$F8` is safe in every kernel here except the one that dispatches through the stack — which is
  exactly the kind of thing a convention phrased as "hoping" cannot tell you.
  📖 **Not measured: the reverse trick** — deliberately using the stack region as scratch. The list
  offers it with its own caveat (*"But then you have to be carefully* [sic] *about which temp
  variables your subroutines use."* 〔stella-list `200401/msg00013`〕); `known-traps.md` covers a variable at `$FF` being clobbered by a `JSR` push and says
  nothing about going the other way.
  The move the same thread makes first costs no scratch at all: Paul Slocum, *"a lot of times you can
  use JMP's instead of JSR's. In Marble Craze I only have 2 bytes for the stack"*, and *"Usually I start
  off programs using more JSR's since it's a little easier, then convert them to JMP's as the program
  gets better defined and memory gets tight"* 〔stella-list `200401/msg00003`〕. **Cited only, not
  verified.**
  (Stella PG). Real-game RAM budgets: Pitfall ≈ all 128 bytes (world = 1 byte!), Random-Dungeon ≈45 with
  aliased overlays, za2600 overflows into cart RAM. ⬜ a RAM-map audit feature (symbols → read/write
  coverage) would catch dead variables (Pitfall's `cxHarry` is stored, never read).
  Keyed by symbol, that audit would mislead on overlays. Andrew Davie's DASM idiom declares `temp ds 8`
  and then several sections each opening with `org temp`, so `overlayvar1` and `linecounter` both name
  `temp`, and *"the same routine (or section of code) CANNOT use variables in overlay section 1 AND
  overlay section 2"* 〔stella-list `200102/msg00024`〕. A traced access carries an address, not a name,
  so with one byte under several names a per-name table cannot say which name was used, and a
  dead-variable report keyed by name can be wrong either way (our reading). `defuse` keys by address
  and is not misled by names, but its own test says it proves the read set, not the ordering that
  "these two variables may share a byte" needs (`internal/cyclebound/defuse_test.go`). **Cited only, not verified.**
  📖 **One multi-level data point.** Manuel Rotschkar, 2004, porting Jumpman with 25 of 33 analysed
  levels transferred: *"Highest RAM usage: 42 Bytes for Dragonslayer."* 〔stella-list
  `200410/msg00178`〕 The post does not say whether 42 counts the whole game's RAM or that level's own.
  **Cited only, not verified.**
  📖 **Where the 128 bytes go, by one author's rule of thumb.** Paul Slocum, 2004, on how large an
  AtariVox save file needs to be: *"I doubt many programs are going to need to store more than 117
  bytes considering there are only 128 bytes in the Atari. Most programs that use the file system will
  probably have text routines which generally require at least 20 bytes of temp RAM. Plus there's
  stack, controller handling variables, generic temp variables, etc."* 〔stella-list `200411/msg00068`,
  quoted in `200411/msg00071`, where Eckhard Stolberg's reply disputes the 117 for saved levels, not
  the RAM〕. **Cited only, not verified** — no text routine's RAM was counted here.
  📖 **One game's RAM, variable by variable.** Ben Larson, 2002, with *"RAM is pretty tight also"*, gave
  a *"rundown of total usage so far"* for his game *Incoming*: *"75 bytes for the terrain (the
  biggest chunk)"*; 12 *"to store memory index locations for the digit graphic lookups in the 6-digit
  E-P-W readout"*; 8 for shot x/y position and velocity (each such variable *"stored using 2 bytes, to
  simulate floating point numbers (actually fixed point)"*); 4 for the two players' x/y positions; 4
  for the player graphic lookups; 4 for the players' elevation and power; 4 for *"the psuedo random
  number register"*; 2 for health; and 1 each for wind, for *"shot wrapping off the top of the screen"*,
  for game status and for score. *"So that leaves 12 bytes. 8 as it is right now, because I need 4 for
  the stack on a certain 2-deep subroutine call"* 〔stella-list `200201/msg00063`〕. The twelve items add
  to 117, which would leave 11 of 128 rather than his 12 (our arithmetic). Thomas Jentzsch, replying,
  suggested three cuts: *"I don't think that you need a 4 byte random generator, 2 bytes should be enough
  here"*; *"perhaps you can share some bytes for different purposes: E.g. the 4 bytes for the player
  graphic lookups and the 12 bytes for the digit graphic lookups are temporary and never used a the
  same time"* — the overlay condition quoted above; and *"perhaps, you don't
  always need all bits (e.g wind or game status) so you could merge those bytes"* 〔`200201/msg00066`〕.
  A budget from a game still being written, and neither post says which cuts he made. **Cited only,
  not verified.**

## 10. Bank switching
- 📖 Scheme landscape (Horton's doc + woodgrain + threads): F8 8K ($1FF8/9) → F6 16K ($1FF6–9) → F4 32K
  ($1FF4–B), +SC 128B RAM variants; 3F/3E(+) for big data; DPC+/CDFJ need ARM (Melody/Harmony).
  **Community recommendation: F8 first** (max compatibility, cheapest PCBs, identical idiom scaling to
  F6/F4) — notably thread 338980 was started by DaveC himself.
- ✅ **The same-address trampoline runs here** (corrected 2026-10-02; this line had it documented-only).
  `roms/litmus/litmus_bank.asm` calls into the other bank and back through a hotspot read every frame
  (`roms/litmus/scenarios/bank.json`: `$80` = `$B1`, the counters `$81`/`$82` ≥ 4), and
  `roms/techniques/banked_game.asm` loads its level data through the `$FF80` trampoline
  (`roms/techniques/scenarios/banked_game.json`; `docs/techniques/bankswitching.md`).
- 📖 Best practices: vectors in **every** bank; identical reset stub per bank — both ROMs above carry
  them, but that is read from their source, not run. `TestEveryBankCanBeBootedInto`
  (`internal/emu/bootbank_test.go`) checks from the bytes that a bank's reset path selects a bank and
  passes when **at most one** bank omits it, so on a two-bank F8 a missing or broken stub in one bank
  still passes; nothing compares the stubs for being identical. With `RandomState` off the engine
  starts in bank 0 (`mapper_atari.go` `SetBank`: the first bank whose reset vector points into
  cartridge space below the vectors), so the other bank's stub and vectors never run; with it on
  (`internal/emu/randomstate_test.go`) the starting bank is one draw from a generator
  `internal/emu/emu.go` seeds with 0, and which bank that draw gives was not read here. **Not
  verified.**
  Each bank's RORG at an **odd** 4K segment ($1000/$3000/…) — TJ's reason is that
  otherwise *"you might access the TIA, RAM etc."*, and he adds that *"some standard is to use the last
  possible addresses, so you should use $d000 and $f000 for 8K games"* 〔stella-list `200306/msg00095`,
  quoting him〕; a different origin per bank is also how *"Many bank-switched games"* were made
  debuggable *"on an ICE-based development system with a full 6502 and RAM"* 〔AtariAge `topic/174668`,
  Bruce Tomlin〕 (both **Cited only, not verified**); why it is `RORG` and not `ORG`, in Jentzsch's own
  post: the plain-`ORG` version *"would create 12K instead of 8K files, because you skip the $2000 or $e000
  bank and the assembler will fill that with $FF"*, while `RORG` is *"only a logical origin and not a
  also physical one"* — *"Note that you \*always\* must use ORG and RORG then"* 〔stella-list
  `200306/msg00093`; **Cited only, not verified**〕, the same physical/logical split that leaves DASM's
  listing column on physical offsets (`capability-gap-audit.md`, `srcmap`; our reading); don't put
  code/data in the last bytes before vectors (accidental hotspot hits); SC RAM has separate write/read
  ports (no RMW; phantom reads on page-crossing indexed stores corrupt it).
- ✅(infra) **Gopher2600 supports all schemes we'd use** (F8/F6/F4±SC, FA, FE, E0, E7, 3F, 3E+, DPC(+),
  CDF*; not 0840) and **AUTO fingerprints a plain 8K dasm binary as F8** — our harness can verify
  bankswitching *today* with zero code changes. Bonus: `Cartridge.GetBank()` exposes the live bank →
  a tiny `read_bank` MCP tool is a natural addition.
- ✅(verified 2026-08-04, G1) **`read_bank` now has a witness beyond F8/F6/F4.** `roms/carts` holds a fixture
  per scheme and the bank count is asserted on each: **F6SC 4, F4SC 8, 3E 4 banks of 2048, 3E+ 4 banks of
  1024 at four origins, DPC 2 banks of 4096 plus 2048 bytes of graphics in no bank.** Bank SIZE is the part
  the harness used to assume: two of those five are not 4K. Every one of them is REFUSED by
  `internal/cyclebound`, naming its mapper and the reason, because in each the cartridge window is not the
  image. **Not verified: DPC+, CDF*, ELF/ACE and bus stuffing** — see `docs/capability-gap-audit.md` §G1
  for what specifically blocks each.

## 11. Procedural generation (new domain)
- ✅ **Pitfall's bidirectional LFSR — computed, not run** (samiam blog + disassembly; settled here by
  enumerating all 256 byte values, `scratchpad/lfsr_check.py`, no emulator involved). 1 byte = the
  world. **The step named for the direction the world scrolls, not for the direction the register
  shifts** — that distinction is the whole trap:
  **"right" = shift LEFT**, inserting `bit3⊕4⊕5⊕7` at bit 0;
  **"left" = shift RIGHT**, inserting `bit0⊕4⊕5⊕6` at bit 7.
  It cannot be read the other way and still work: **a shift right loses bit 0, so its tap has to
  contain bit 0; a shift left loses bit 7, so its tap has to contain bit 7.** The two tap sets in the
  sources each contain exactly one of those, and each fits exactly one direction. Read literally
  ("right step" = shift right with `bit3⊕4⊕5⊕7`) the function is **not even a bijection** and the
  orbit from $C4 is 34 long, with cycle lengths {1,2,3,4,31,32,33,34} over the 256 seeds.
  Read correctly: **both steps are permutations of 0–255, `left∘right` and `right∘left` are both the
  identity, 0 is the fixed point, and every one of the other 255 bytes lies on a single cycle** — so
  "period exactly 255" is exact, not approximate. **The disassembly's `bit1` is wrong**: `shr{1,4,5,6}`
  is not a bijection and fails to invert the right step for **128 of the 256** values.
  From seed **$C4** the world runs `$C4 $89 $12 $25 $4B $97 $2E $5C $B8 $70 $E0 $C0 $81 $03 $06 $0C …`
  and ends `… $11 $23 $47 $8E $1C $38 $71 $E2`; the "left" sequence is that one reversed.
  Regression handles: sha256[:16] of the 255-byte forward sequence = **751c0803eae3c1d4**, of the
  reverse = **62b12e47b5a03b55**.
  📖 **What reversibility saves, in the post that named it.** Thomas Jentzsch, 2001, calling it a
  *"bidirectional LFSR"* — *"the process of generating the next random number is reversible"* — sets
  it against River Raid: *"The two byte LFSR used in River Raid for the same purpose seems to be
  different, so Carol needs some extra RAM to store the previous random number, which is necessary to
  restart a scene when the player dies."* 〔stella-list `200109/msg00030`〕 **Cited only, not verified**
  — his *"seems"*; River Raid was not examined here.
  📖 **His question in the same post — *"if somebody knows a good bidirectional two byte LFSR, please
  let me know"* — has no reply in the thread, and the answer is any maximal-length one** (computed
  here, 2026-10-02, no emulator). A step that puts every non-zero value on one cycle is a permutation,
  so it has an inverse, and for a shift register the inverse is the opposite shift with the lost bit
  solved from the others — a register whose feedback polynomial is the forward one's **reciprocal**.
  Pitfall's pair above is exactly that: `x^8+x^4+x^3+x^2+1` for the "right" step and
  `x^8+x^6+x^5+x^4+1` for the "left" (exponents {0,2,3,4,8} and {0,4,5,6,8}, each 8 − the other), and
  deriving the backward taps from the forward ones gives `bit0⊕4⊕5⊕6` — with `bit1` in place of `bit0`
  it does not invert (the disassembly's error above). Over all 16 maximal-length 8-bit tap sets (every
  left-shift tap set containing bit 7, each run from seed 1 for its period) and the first 12
  maximal-length four-tap 16-bit ones (tap sets containing bit 15, taken in lexical order), each
  checked at all 2^8 or 2^16 states, the derived backward step undoes the forward one at every
  state, so it walks the same single cycle backwards; the 8-bit ones were also run to their period of
  255. The reciprocal relation beyond the cases counted is the textbook one, **Not verified** here; and
  whether River Raid's generator is such a shift register was not examined.
- 📖 **DaveC's Random-Dungeon** (read in full): 2-byte room codes (walls/interior indices into ROM strip
  libraries); **exit-wall code spliced into the next room's entry wall** = infinite consistent dungeon with
  zero map storage; curated room-code tables (validity by construction); 8-bit Galois LFSR `eor #$8E`
  (period 255, confirmed) → later 16-bit; pacing counter for special rooms; 3 kernels dispatched per frame.
  His landscape evolved to 10 zones × per-zone x/y/tile arrays = 20 independent objects + per-line COLUPx.
- ✅ **LFSR hygiene (SpiceWare Step 10) — computed, not run** (`scratchpad/lfsr_check.py`, all 256
  values enumerated, no emulator): `lsr A / bcc + / eor #$B4` is a **permutation of 0–255**; **$00 is a
  fixed point** (hence "never seed 0" — it is not a caution, it is the only way the generator can
  fail); every other byte lies on **one cycle of length 255**, so the period claim is exact.
  From seed $01: `$01 $B4 $5A $2D $A2 $51 $9C $4E …`, ending `… $69 $80 $40 $20 $10 $08 $04 $02`;
  sha256[:16] of the 255-byte sequence = **1cc3384d72331258**. Seeding from INTIM is untested here —
  that is about *where the seed comes from*, not about the generator, and INTIM can read 0.
  ✅ **`$B4` and `$8E` are two of sixteen — computed, not run** (2026-09-30, every EOR constant through
  the same `lsr / bcc / eor`, every seed, no emulator): exactly **16** constants make the step a
  permutation with one 255-long cycle — `$8E $95 $96 $A6 $AF $B1 $B2 $B4 $B8 $C3 $C6 $D4 $E1 $E7 $F3 $FA`,
  the same sixteen Thomas Jentzsch listed beside the routine in 2004 〔stella-list `200401/msg00222`〕.
  Because each of them puts all 255 non-zero bytes on one cycle, a second seed with the **same**
  constant only replays the same sequence from another point; a second generator that should not track
  the first wants a different constant from the list.
  📖 **The INTIM recipe — offered as a generator in place of the LFSR, not as a seed.** Thomas
  Jentzsch, 2001, *"a different idea about a random number generator, which doesn't cost you a lot of
  cycles and RAM space"*: *"1. load a by one decreased value into TIM64T and let the loop countdown
  not to 0 but to negative values 2. make the checking loop 8 or 16 cycles long 3. get the lower bits
  (up to 3 or 4) from the last load of INTIM"* — *"Those bits shoud be nearly random, but only if
  your code before doesn't have a very constant timing (uses branches etc.) You have to test..."*
  〔stella-list `200102/msg00155`〕. **Cited only, not verified** — nothing here has measured how those
  bits vary.
## 12. Harness/tooling implications
- 📖 **Stella IS automatable for F-4** (debugger doc + installed Stella 7.0 verified): `<rom>.script`
  auto-runs at `-debug` startup (`frame N / tia / riot / dump 80 ff 7 / saveSnap / saveSes`); `saveSes`
  writes the whole session to a text file; `-ss1x -sssingle` raw snapshots. Limits: GUI window always opens
  (no headless), no quit command (kill externally), no input timelines. **v1 design: RAM + TIA register
  compare at frame N** (exact, palette-free); image compare v2 (Stella doubles pixels horizontally; map
  palettes to TIA indices first). Needs a one-time frame-numbering calibration probe.
  (This is the plan as first written; the oracle as built changed part of every step — see
  `docs/stella-oracle.md`, Design.)
- ⚠️ AtariAge blocks direct fetching (Cloudflare 403) — use the Wayback Machine; randomterrain mirrors
  Davie/SpiceWare content. Disassembly corpus is ISO-8859+CRLF — `grep -a`.
- 📖 Davie's *Newbies* Revised PDF = editorial consolidation of Sessions 1–25 + opcode appendix; no new
  material; it **never covers** 6-digit score/paddles/BCD-display/random/sound — those live in SpiceWare
  Steps 3/10/13, score6.asm, and the Stella PG.
  📖 **A copy of a session is a copy of one version of it.** Davie corrected the posted sessions in
  place: *"I do actually go back and correct the errors in the original - so its not strictly necessary
  to use the errata - just make sure you have the latest and greatest of each lesson"* — his reply in
  2003 to Ron Corcoran, a reader who had posted a PDF of the lessons 〔stella-list
  `200305/msg00111`, `200305/msg00108`〕. The PDF held locally
  (`reference/docs_atari/Atari_2600_Programming_for_Newbies.pdf`) is a later compilation; it says of
  itself *"Edited by Dion Olsthoorn – April 2018"*, from tutorials posted *"between May 2003 and April
  2012"*. **Cited only, not verified** — no session was compared between versions.

---

## Corrections adopted into our docs (the audit's ⚠️ list)
1. `cycle_counting_guide.html` positioning math = approximation; do not cite for positions.
2. Pitfall disassembly `LeftRandom` comment is wrong (bit0, not bit1) — carry the corrected formula.
3. SpiceWare Step 3 vs Step 7 left-PF1 window numbers conflict — to be settled by litmus.
4. The HMOVE comb / late-HMOVE behavior exists in **no** local source — Towers' TIA Hardware Notes was
   adopted as the authority and has since been **corroborated by our own measurement**:
   `litmus_hmove_side` (comb = left 8 px blanked on strobe-after-WSYNC lines even with HMxx=0;
   mid-visible strobe ~cyc 39 = a no-op; line-end strobe ~cyc 74 = left by HM+8 px with no comb),
   fixed by `roms/litmus/scenarios/hmove_side.json` and `internal/emu/hmoveside_test.go`. What is
   still open is narrower, and it is recorded where the measurement was made rather than here: the
   numbers are emulator-verified and the Stella cross-check is pending
   (`docs/verified-coverage.md:42`). **Line corrected 2026-09-03** — it had said "pending our own
   measurement" while sitting on top of the evidence that the measurement had been made.
5. Add to constants: 24-cycle HMxx freeze after HMOVE; NMOS-BCD C-only; stores never take page-cross
   penalties (deterministic kernel timing); CLD mandatory at init.

## Where the follow-ups live
The prioritized work items distilled from this audit (new litmus ROMs, `read_bank`, audio sample capture,
Stella oracle automation, `pkg/audio` tables) were tracked in the v2 backlog — now **delivered (see
`CHANGELOG.md`)**, with any remaining gaps folded into the single live backlog **`capability-gap-audit.md`**.

## Mid-line HMOVE — verified (2026-06-12, litmus_hmove_mid)

Strobing HMOVE outside the post-WSYNC slot, with **all HM registers cleared** (HMCLR'd):
measured on Gopher2600 with pixel-level confirmation (bar edge above/below the strobe line):

| strobe completion (visible clock) | shift |
|---|---|
| 13  | 0 px |
| 85  | 0 px |
| 142 | **−5 px (left)** |
| (control: no strobe) | 0 px |

*(Clocks corrected in v1.32.0: the original ≈1/73/130 were hand-counted estimates; `trace_clocks`
measured the actual strobe completions — rule 2, "get cycles from the simulator", applies to
clocks too.)*

The folk rule "objects move right ~1px/4CLK" did **not** reproduce at these sample points — the
shift is a non-monotonic function of strobe time (consistent with Towers' per-cycle tables being
more complex than the summary line). Regression-pinned in `scenarios/hmove_mid.json`. For
authoring: keep HMOVE in the post-WSYNC slot unless deliberately exploiting the quirk, and if
exploiting it, measure your exact strobe cycle with this litmus pattern first.
