# Technique — paddle input (dump / charge / per-line count)

**Goal:** the standard paddle-reading kernel: discharge the pot capacitor during blanking,
release at visible start, count scanlines until INPT0 D7 goes high — that count *is* the paddle
value, mapped to whatever the game controls.

Demo: `roms/techniques/paddle_demo.asm` (a P0 bar tracks the paddle).
CI: `scenarios/paddle_demo.json` (three paddle positions → exact line counts + bar X, golden).
Hardware basis: `litmus_paddle` (v0.54.0; INPT0 dump/charge transfer curve measured).

## The pattern

- **Overscan + VBLANK: `VBLANK = $82`** — D1 blanks the screen as usual, **D7 dumps the paddle
  caps** (discharge). Both functions live in one register; keep dump on through blanking.
- **Visible start: `VBLANK = 0`** — releases the dump, capacitor starts charging.
- **Kernel, each line** (cheap: ~12 cycles when already latched). ⬜ **Unresolved 2026-09-03:**
  adding the same instructions up from the engine's own table gives **8 or 16** depending on which
  form is counted, not 12. Neither figure has been run, so the doc keeps its number and records the
  disagreement rather than picking a side. The shape survives either way — see
  `input-budget.md`, whose argument is the ratio between a per-frame and a per-line read, not the
  exact count.
  ```
  lda padNew / cmp #$FF / bne done   ; already latched this frame?
  bit INPT0 / bpl done               ; D7 still 0 = charging
  stx padNew                         ; X = line counter → the paddle value
  ```
- End of frame: commit `padNew` → `padVal` (use $FF→191 for "never latched" = far end),
  clamp/map to the controlled quantity. The demo maps to bar X via PosObject (clamp 151).

## Verified numbers
With this frame structure, `set_input paddle` 0.1 / 0.25 / 0.5 measure **0 / 63 / 170 lines**
(matches the litmus transfer curve shifted by the dump-release line). Bar X follows exactly
(170 clamps to 151). Note the measured value depends on *when in the frame you release the
dump* — re-baseline the scenario after structural changes (ours shifted by 7 lines when the
VBLANK timer constant moved).

## Notes
- Full paddle range needs more lines than one visible frame at the far end — real paddle games
  either accept saturation (as here) or read across two frames. A third choice: a wrist turns only
  part of the knob's travel, so pick a range that always latches within one frame (Kirk Israel
  〔stella-list `200505/msg00092`〕). **Cited only, not verified.**
- **Which way the count runs with the knob: two posters say clockwise is longer, one emulator
  report read the other way, and the list did not settle it.** dee, May 1999: *"i'm not sure which way provides the most or least
  resistance, clockwise or counter-clockwise"* 〔stella-list `199905/msg00040`〕; Eckhard Stolberg:
  *"I'm not sure, but I think having the paddles turned all the way clockwise takes the longest to
  recharge."* 〔`199905/msg00047`〕 dee again, August 1999: *"when the paddle is turned clockwise,
  INPT# bit 7 is supposed to take more cycles before bit 7 is set to one, right?"*
  〔`199908/msg00007`〕 Stolberg, this time without the hedge: *"This is correct."* — with a `LDA
  INPTx/BMI/INC` loop *"the counter value would get bigger the further the paddle is turned
  clockwise"* 〔`199908/msg00008`〕. Erik Mooney, July 1999, in
  another thread: *"All the way counterclockwise should be very low resistance and therefore the
  capacitors would recharge immediately. All the way clockwise, it's actually quite long. I tested
  this a while back, don't remember the exact number but it was on the order of 1.5 frames
  (400-something scanlines.)"* 〔`199907/msg00146`〕 The report the
  other way is dee's own, earlier the same day as the August question: with a LDA-BMI-INC routine in
  the PCAE emulator set to paddle controllers, *"the counter is actually less the further i would have turned
  the pots clockwise"* 〔`199908/msg00006`〕. Stolberg asked whether the display routine's reference
  counter counted upwards too; the thread ends on his *"Please post the whole code."*
  〔`199908/msg00013`〕, so the two were not reconciled there. Nothing here
  measures the direction: `set_input paddle` takes a fraction, not a knob angle. **Cited only, not
  verified.**
- **Resolution is the number of reads; range is the time they span; the two are independent.**
  Erik Mooney on Super Breakout's 128 positions: 128 evenly spaced reads give 128 steps whether they
  cover the whole screen (most of the knob's travel) or about 20 scanlines (perhaps 15–20 degrees of
  it) 〔stella-list `200006/msg00098`〕. The per-line count above is the one-read-per-line case. A
  two-line kernel that reads every other line halves the steps — Eric Ball's figure is 120 positions
  instead of 240 〔stella-list `200505/msg00102`〕. Fewer values need fewer reads: *"If you only need
  e.g. 10 different values, you only have to check 10 times"*, against ~160 a frame for Kaboom!'s
  single-pixel positioning (Thomas Jentzsch 〔stella-list `200402/msg00230`〕). Erik J. Eid's card game,
  with at most five choices, needed *"about a dozen times or so"*; he took a paddle over a joystick for
  the menu because an absolute position does not need the current choice kept track of
  〔stella-list `200107/msg00016`〕. **Cited only, not verified.**
  In April 2002 Eid reported taking the paddle out again: *"I also dropped the paddle support in
  favor of a joystick. I found that with the type of display I have - not very repetitive - and only
  reading the paddle in a few locations, that the screen was never stable enough for my liking."*
  〔`200204/msg00006`〕 He does not say what about the screen was unstable. **Cited only, not
  verified.**
- **A short polling window narrows the arc and pins it.** Thomas Jentzsch: possible in VBLANK
  *"only if you need only very few different values and are able to poll the hardware registers in
  constant intervalls"* — uneven spacing makes the control "strange", which is why the reads usually
  live in the kernel. 50 polls in 500 cycles give 50 values *"(unless there are some hardware
  limitations I am unaware of)"*, but within a very small turn, *"and this
  area is on a fixed position, so left or right of that small angle the paddle won't react at all"*
  〔stella-list `200402/msg00230`〕. Christopher Tumber's Quadraside read all four in overscan: it writes
  `VBLANK = 2` at the top of overscan (his comment: blank *"and charge paddles"*), then makes 23 passes
  over INPT0-3, each read padded so both branch paths cost 10 cycles (`bne` not taken + `dec` + `jmp`,
  or `bne` taken + `nop` `nop` + a 3-cycle `sta` to an address the TIA does not use — our count and
  reading). The result was *"something like 13 paddle positions"*: not enough for Kaboom!, perhaps
  enough for a driving game's hard left … hard right 〔stella-list `200402/msg00238`〕. **Cited only, not
  verified.**
- Four paddles = INPT0-3 with the same pattern; pairs share a port.
- **Four paddles from one line count** (Erik Mooney 〔stella-list `199710/msg00006`〕): load the count
  into A once, then per paddle `bit INPTn / bmi chargedN / sta PotN`. `sta` is two cycles cheaper than
  `inc`, the count stays in A, and a skipped store costs 3 cycles instead of 5, so the line varies less.
  One branch per paddle and no "already latched" test: once D7 is set the branch skips the store, so
  `PotN` keeps the last line on which the cap was still charging — one less than this file's
  first-line-set count (our reading). He ended by asking for a time-invariant way to read all four;
  supercat's form below is branch-free. **Cited only, not verified.**
- **Not every paddle every frame.** With several paddles you can *"check the paddles only every 4th
  frame. This should work for almost all games, except maybe ... for very fast games like Kaboom! or
  SCSIcide"* — but the read then takes an index (`INPT0,x`), so it needs X or Y and A, and `bit` is no
  longer usable (Thomas Jentzsch 〔stella-list `200402/msg00230`〕; of the paddle games he had analysed,
  some polled the same paddle always, some each paddle every nth frame, one all paddles in one frame).
  Jim Nitchals' earlier version, quoted by Piero Cavina: alternating one paddle per frame gives new
  values only every 30th of a second, and alternating every 2 lines costs resolution instead
  〔stella-list `199710/msg00005`, where he is "Jim"; the surname from `199612/msg00025`〕. **Cited only,
  not verified.**
- **Equal paths with one skipped byte** (Thomas Jentzsch 〔stella-list `200301/msg00187`〕):
  `lda INPT0 / bmi p1 / .byte $2c / p1: sty padVal1`. Not taken, `$2C` makes the 2-byte `sty` the
  operand of a `bit` absolute (4 cycles); taken, the branch lands on the `sty` (3); 9 cycles either way
  (our count; the `$2C` skip itself is measured in `integration-density-playbook.md`, "BIT-absolute
  skip-next"). It replaced Paul Slocum's version that leaves the loop and branches back, which he could
  not use in Marble Craze because his kernel loops were so large the branch went out of range; he had
  evened the paths with `WSYNC` every few lines instead 〔stella-list `200301/msg00181`〕. Thomas's own
  2002 branch-out form hit the same range limit for Paul 〔stella-list `200301/msg00190`,
  `200301/msg00191`〕. As posted, the taken `bmi` lands on the store, so it stores on lines where D7 is
  set, while Paul's three-line `lda INPT0 / bmi paddles1 / sty padVal1` skips the store on those lines —
  choose the branch for the sense you need (our reading). **Cited only, not verified.**
- **All four in 26 or 32 cycles, without a branch** 〔supercat, AtariAge blog `entry/1073`〕. With X
  holding any value 64–127, `cpx INPTn` compares the whole byte and `ror` shifts the carry into A:
  `lda INPT0 / cpx INPT1 / ror / cpx INPT2 / ror / cpx INPT3 / ror`, then `eor pscratch+k /
  sta pscratch+k`, where *k* is the line's "magic index" — which bit of a Gray-code count flips on
  that line (`0 1 0 2 0 1 0 3 0 1 0 2 0 1 0 4 …`). 26 cycles when *k* is known for the line, 32 when
  it is not; every path costs the same, unlike `bit`/`bpl`. The counts are not usable as they stand:
  the post gives the decode, run outside the kernel (an EOR chain down `pscratch+6`…`pscratch`, then
  a loop that shifts one 7-bit value per paddle out of them). ★**This breaks the low-bits rule
  below.** D7 set always clears the carry; D7 clear sets it only when the byte read is at most X, and
  "any X from 64 to 127" holds only while the undriven low bits read 63 or less — here the operand's own address, under both the address and the last-bus-byte models
  of `known-traps.md` ("Bus residue" names three different models, and this engine picks one). A
  substitute cartridge can answer differently, as that section records. **Cited only, not verified.**
- Latch test uses **N flag (`bit`/`bpl`)**, per the verified input rules (never test low bits).
- **State the slack as a number.** Asked whether his Pong Wars (no player input yet) could go four
  players *"using the interleaved 4-color backgrounds"*, Thomas Jentzsch answered that it exceeds a
  stock 2600 and *"The current kernel, which is quite optimized, has exactly one cycle free."* He did
  not mention paddles; the link is the proposer's (CapitanClassic): *"I expected as much, since the
  paddle capacitors need to be read almost every display line for accuracy"* 〔AtariAge
  `topic/360379`〕. Here that number comes from `prove_line_budget`. **Cited only, not verified.**

## Moving the release instead of the reads (cited, not measured)
- **The dump need not end at the same line every frame** 〔supercat, AtariAge blog `entry/1152`〕. If
  only one band of the kernel can afford a per-line read, time the release so the paddle times out
  inside that band. His form, in a loop that runs once every 8 lines with X counting down:
  `cpx paddlecoarse / lda #0 / ror / sta VBLANK` — the carry lands in VBLANK D7, so the dump is
  switched on or off without a branch, 8 or 10 cycles once per 8 lines. Then read every line across
  a 32-line window, take 8 × `paddlecoarse` + the reading as the position, and nudge `paddlecoarse`
  when the reading falls within 10 lines of either end of the window. **Cited only, not verified.**
- The same idea as a "floating dump" 〔omegamatrix, AtariAge `topic/245239`〕: a paddle cannot go from
  rest to full speed in one frame, so poll a narrow window (his example: lines 42–72) and decide per
  segment of about 10 lines whether to strobe the dump. He did not ship it — reading the whole kernel
  was easier — and pots of different value (1M against 500K) complicate it. **Cited only, not
  verified.** This file's own measurement is the other face of the same freedom: the count moved
  7 lines when the VBLANK timer constant moved (Verified numbers, above).
- **Where the release falls also picks the usable part of the turn.** Reviewing a paddle demo, Eckhard
  Stolberg advised: *"you should stop grounding the paddles after the three lines of VSYNC. That way the
  usable part of the paddle will be about in the middle of the full possible turn"* 〔stella-list
  `200106/msg00098`〕. Releasing there lets the cap charge through VBLANK first, so the visible-line
  count starts partway along the turn; this file's pattern releases at visible start (our reading).
  **Cited only, not verified.**

## Jitter (cited, not measured)
- **A median of three "might be a better choice" than an average** (reveng's words): a one-frame
  spike is damped by averaging and removed by a 3-sample median; noise that lasts longer than a frame
  defeats both 〔AtariAge `topic/242625`; the median is suggested again in `topic/290927` — a few
  bytes of RAM〕. Hysteresis hides jitter without slowing the response 〔tep392 in `topic/242625`, on
  Castle Crisis〕. **Cited only, not verified.**
- **If you average anyway on the 6507:** two readings are `clc / lda / adc / lsr` (each up to 127),
  four take two `lsr` (each up to 63); counting the newest frame twice and dividing by four weights
  it. Dividing each reading by 3 before summing avoids overflow but piles up truncation — (4,4,4)
  averages to 3. A table-free divide by 3 is quoted at 18 bytes and 30 cycles (omegamatrix). All
  〔AtariAge `topic/242625`〕. **Cited only, not verified.**

## Hardware around the read (cited, not measured)
- **The capacitor is on the console board, not inside the TIA** — the .068 µF parts on the 2600
  schematic, one on each paddle input pin 〔AtariAge `topic/294749`〕. The pot is in the controller,
  and Atari bought pots off the shelf, so identical-looking paddles can hold different pots (same
  thread): the C belongs to the console, the R to whichever controller is plugged in. **Cited only,
  not verified.**
- **Paddle or joystick, detected at power-on** 〔AtariAge `topic/279570`〕: dump the caps for one line,
  release, wait (512 lines is reported to be enough), then read `INPT0` (and `INPT2`) D7 — set means
  a paddle charged it. Astroblast does the same in its `DetermineControllerType`, per the same thread.
  Trap: a Sega Genesis/Mega Drive pad reads as a paddle, because its pull-up on pin 5 keeps
  recharging the cap; the Harmony menu lets the player force joystick mode by holding the fire button
  at power-on. The same test was given in 1999 from the other side: with no paddle the line is an
  open circuit, *"infinite resistance being higher than any resistance a paddle can provide"*, so it
  reads higher than any paddle can; Erik Mooney guessed that Astroblast checks only at power-up, or
  maybe at game start 〔stella-list `199905/msg00018`〕. **Cited only, not verified.**
- **The paddle buttons are joystick bits.** *"the paddle buttons map to the same bits as joystick
  left/right"*, so one routine takes either controller — Jake Patterson used left/right (Game Select
  cycles the choices) to enter a mode for that reason 〔stella-list `200109/msg00224`〕. Paddle 0's
  button is PA7, the pin shared with joystick 0 right, and the RIOT's edge detector watches it: the
  interrupt pin is unconnected, but the flag can still be polled (TIMINT D6; the edge-control writes
  are in `fundamentals-audit.md`). Mark De Smet proposed using it *"in the same way as the latches on
  the joystick trigger buttons"*, to catch the fastest presses or only the release 〔stella-list
  `200005/msg00045`, `200005/msg00068`〕. In this engine the flag is set at power-on and the first
  TIMINT read clears it (`litmus_timint_pa7`). **Cited only, not verified.**
- **Booster Grip: two more buttons on the paddle lines.** Eckhard Stolberg, replying to a post on
  wiring a NES pad as a Booster Grip: *"the Booster Grip connects the two paddle lines with the joystick
  button line"* 〔stella-list `200103/msg00325`〕. Omega Race uses one extra button (Chris Pepin
  〔stella-list `200103/msg00316`〕), Thrust two (Thomas Jentzsch 〔stella-list `200103/msg00318`〕). A
  button only has to read as on or off, so it is *"nowhere near as processor-intensive as regular paddle
  reads"* (Glenn Saunders 〔stella-list `200301/msg00192`〕); Omega Race, as disassembled on the list,
  reads INPT0/INPT1 just before VSYNC 〔stella-list `200202/msg00146`〕. The dump still matters: asked
  whether bit 7 of VBLANK never has to be set, Thomas Jentzsch answered *"you have to set the bit
  sometimes, else (on some consoles) the condensators of the paddle controllers (which are used here)
  will charge even without input"*, and suggested setting it for some time right after reading —
  *"only a suggestion"* 〔stella-list `200202/msg00145`〕; Omega Race sets it through VSYNC and clears it
  at the end of VBLANK 〔stella-list `200202/msg00146`〕. He added that
  Thrust's support was still a bit buggy and z26 did not yet emulate it exactly 〔stella-list
  `200202/msg00145`〕. **Cited only, not
  verified.**
- **More digital inputs through one paddle pin** 〔AtariAge `topic/77034`, a proposal for four
  joysticks〕: wire each direction and the fire button through its own resistor on base-2 values
  (8k/16k/32k/64k/128k, sum under 1 MΩ), so each combination gives a distinct paddle value, e.g.
  up+right+fire = 200k. A hardware trick; whether the count separates the smallest steps reliably is
  not shown. **Cited only, not verified.**
- **A clone console is not the hardware.** One owner reports paddles "basically useless" in Kaboom!
  on a Flashback X, where a 2600 on the same TV plays fine 〔AtariAge `topic/349443`, a single
  unanswered post〕. A paddle checked on a clone has not been checked on a 2600. **Cited only, not
  verified.**
