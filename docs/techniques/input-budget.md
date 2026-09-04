# Technique — choosing an input device by what reading it costs

**Goal:** on this machine the controller is not a preference, it is a line item. Which device a
game can use is decided by **where in the frame the read lands and how often it repeats** — not by
what the hardware can sense. Pick the device the kernel can afford, then design the controls.

Demo: `roms/techniques/bullets.asm` (joystick, two axes) · `roms/techniques/paddle_demo.asm` +
`docs/techniques/paddle.md` (paddle).
CI: `scenarios/paddle_demo.json` (three paddle positions → exact line counts).
Hardware basis: `litmus_paddle` (v0.54.0; INPT0 dump/charge transfer curve measured) ·
`litmus_swchb` (SWCHB read side verified). **No litmus exists for the keypad or the trackball.**

## Where the cost lands

| device | read shape | cost | where it lands |
|---|---|---|---|
| **digital joystick** | `lda SWCHA / and #bit / branch`, twice for two axes | **40–44 cycles** | **once per frame**, in VBLANK — where slack exists |
| **paddle** | count scanlines until `INPT0` D7 goes high | **8–16 cycles** | **every visible scanline** — inside the kernel, where slack does not exist |
| keypad | ⬜ not measured here | ⬜ | ⬜ |
| trackball | ⬜ not measured here | ⬜ | ⬜ |

Both numbers are summed from this repository's own instruction table
(`Gopher2600/hardware/cpu/instructions/definitions.json`, keying on operator + mode + **byte
count**, since that table distinguishes zero-page from absolute by size, not by mode name) over
code that is already in the corpus: the joystick block at `roms/techniques/bullets.asm` (both
directions, branches counted not-taken; each taken branch adds one) and the per-line paddle kernel
quoted in `docs/techniques/paddle.md`.

**The ratio is the point.** Over 192 visible lines the paddle costs **1,536–3,072 cycles a frame**
against the joystick's 40–44 — **35× to 77×** — and an NTSC frame holds 262 × 76 = 19,912
cycles in total. So the paddle spends **8–15% of the whole frame**, and it spends it in the one
region that has no slack. The joystick spends 0.2%, in the region that does.

## What that forces

- **The joystick is the default not because it is simplest but because its cost is once.** A
  once-per-frame read can be moved into VBLANK and forgotten; a per-line read is a term in every
  kernel line's budget and competes with drawing.
- **A paddle game's kernel is designed around the paddle**, not the other way round: `paddle.md`
  measures 0 / 63 / 170 lines for three positions, which is the count *being* the value — the
  kernel cannot also be doing something expensive on those lines.
- **Devices that need continuous sampling do not fit.** The list said so in 1997 with a consequence
  rather than an argument: *"there are no perportional trakball games. It's too hard to constantly
  read a trakball, which is why the Atari trakballs have a joystick emulation mode"* — the trackball
  ships a fallback because software could not keep up, not because the hardware could not sense.
  Same source on the keypad: *"Other controllers like the keypad require an insane amount of time
  to read. Play Star Raiders and FEEL the delay between a keypress and a response."*
  〔Stella list, `controllers`, 1997-09, Glenn Saunders〕

## Reading the shape, not the value

`docs/techniques/game-states.md` fixes the discipline that makes any of these affordable:
**snapshot the inputs once per frame and compare against the previous frame** — edges, not levels.
That converts a device read into a fixed per-frame cost and keeps hold-to-repeat bugs out. It is
also why the joystick's 40–44 cycles is the *whole* cost and not a per-line one.

## Not measured here (deliberately marked)

- **Keypad and trackball have no numbers in this repository.** The ledger names three untapped
  threads that would supply them, and nothing has been mined from any of them:
  - `docs/mining-digest.md` — `| [301035](…) | keypad-read-delay | Keypad read delay | reference/atariage/ |`
  - `docs/mining-digest.md` — `| [88663](…) | reading-trackball | Reading the Trackball | reference/atariage/ |`
  - `docs/mining-digest.md` — `| [119919](…) | keypad-joystick | Keypad + Joystick Together; Is it Possible? | reference/atariage/ |`
  **Mine those three and this table can be completed.** Until then the two ⬜ rows stay ⬜.
- `paddle.md` says its per-line kernel is "~12 cycles when already latched". Summing the same code
  from the instruction table gives **8** on the early-exit path and **16** with every branch falling
  through. The doc's figure sits between the two; **which variant it counted has not been checked
  here**, and no ROM was run to settle it.
- The engine implements exactly four peripherals — `Gopher2600/hardware/peripherals/controllers/`
  holds `stick.go`, `paddle.go`, `keypad.go`, `gamepad.go` — so the keypad *can* be driven; it has
  simply never been budgeted.
  **One number for it now exists, and it is large.** A keypad scan drives the port as an output and
  then reads it, and the Programmer's Guide asks for **400 µs between the write and the read** —
  `1194720 × 0.0004 = 477.888 cycles = 6.288 scanlines`, **2.4 % of a 262-line frame, per direction
  change**. That arithmetic is not ours; it is in stella-list `200011` (`more-keyboard-nonsense`),
  where it is also called *"an upper bound as a general rule of thumb"* rather than a specification.
  **How many waits a full scan needs was the open number, and 1998 answers it.** Eckhard Stolberg,
  `199804/msg00077`, on why keypad input felt slow: *"**They ARE that hard to read.** To read one
  keyboard controller you have to **write out which row to read, wait for 400ms** and then check all
  three buttons in that row from three different read ports. If you have to do that for **four rows
  each on two controllers**, that takes quite some processor cycles."* Four rows × two controllers =
  **eight waits**:

  | | scanlines | of a 262-line frame |
  |---|---|---|
  | one controller (4 rows) | 25.2 | **9.6 %** |
  | two controllers (8 rows) | **50.4** | **19.2 %** |

  `477.888 × 8 = 3,823` cycles against a frame's `262 × 76 = 19,912`.

  **Whether the wait really applies eight times is a reading of the sources, not a measurement, and
  this harness cannot settle it.** The exemption is Chad Schell's — *"if you only **read** the port,
  and thus don't change **its configuration**, the 400 uS delay does not apply"* — and the Guide's
  requirement is *"between **writing to this port** and reading the TIA input ports."* A row select
  **is** a write to the port, and Schell's exemption is explicitly for the read-only case, so the
  wait stands on all eight. The alternative reading — that only a `SWACNT` change costs, and the
  eight `SWCHA` writes are free — gives **6.3 lines instead of 50.4, an eight-fold difference**.
  A litmus cannot choose between them: `litmus_swacnt` band 5 measured that **this engine models no
  settling time at all**, so both readings produce identical output here. **Budget the 19 %** and
  treat it as the pessimistic reading it is. Found by the mailing-list distillation (helper-2), who
  proposed the litmus that would have settled it on hardware. **Gopher2600 does not model the delay at all** (its own `keypad.go`
  carries the TODO), so nothing here will make you pay it: see `known-traps.md`.
  **A missing fifth peripheral — but not the keypad's hole.** The **driving controller** is not in
  that list either (`controllers/` holds `stick.go`, `paddle.go`, `keypad.go`, `gamepad.go` and no
  `driving.go`; the engine folds its events into the stick's horizontal/vertical). The reason it is
  absent is *different*, and an earlier version of this line got that wrong: the keypad's cost is the
  **400 µs settling wait** because the port is driven as an output, while the driving controller is
  **read like a joystick** — a Gray code in the low two bits of a `SWCHA` nibble, sampled once a
  frame, no direction change and therefore no wait. Two peripherals missing for two reasons; the
  costs do not transfer. (Corrected 2026-09-04 — helper-1 caught the category error and supplied the
  earlier source: Eckhard Stolberg, 2000-08-14, two years before the wiring diagrams this file
  originally cited.)


---
