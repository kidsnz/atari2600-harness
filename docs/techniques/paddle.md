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
- **Resolution is the number of reads; range is the time they span; the two are independent.**
  Erik Mooney on Super Breakout's 128 positions: 128 evenly spaced reads give 128 steps whether they
  cover the whole screen (most of the knob's travel) or about 20 scanlines (perhaps 15–20 degrees of
  it) 〔stella-list `200006/msg00098`〕. The per-line count above is the one-read-per-line case. A
  two-line kernel that reads every other line halves the steps — Eric Ball's figure is 120 positions
  instead of 240 〔stella-list `200505/msg00102`〕. **Cited only, not verified.**
- Four paddles = INPT0-3 with the same pattern; pairs share a port.
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
  at power-on. **Cited only, not verified.**
- **More digital inputs through one paddle pin** 〔AtariAge `topic/77034`, a proposal for four
  joysticks〕: wire each direction and the fire button through its own resistor on base-2 values
  (8k/16k/32k/64k/128k, sum under 1 MΩ), so each combination gives a distinct paddle value, e.g.
  up+right+fire = 200k. A hardware trick; whether the count separates the smallest steps reliably is
  not shown. **Cited only, not verified.**
- **A clone console is not the hardware.** One owner reports paddles "basically useless" in Kaboom!
  on a Flashback X, where a 2600 on the same TV plays fine 〔AtariAge `topic/349443`, a single
  unanswered post〕. A paddle checked on a clone has not been checked on a 2600. **Cited only, not
  verified.**
