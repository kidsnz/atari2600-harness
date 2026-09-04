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


---
