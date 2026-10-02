# Technique — choosing an input device by what reading it costs

**Goal:** on this machine the controller is not a preference, it is a line item. Which device a
game can use is decided by **where in the frame the read lands and how often it repeats** — not by
what the hardware can sense. Pick the device the kernel can afford, then design the controls.

Demo: `roms/techniques/bullets.asm` (joystick, two axes) · `roms/techniques/paddle_demo.asm` +
`docs/techniques/paddle.md` (paddle).
CI: `scenarios/paddle_demo.json` (three paddle positions → exact line counts).
Hardware basis: `litmus_paddle` (v0.54.0; INPT0 dump/charge transfer curve measured) ·
`litmus_swchb` (SWCHB read side verified). **No litmus exists for the keypad or the trackball.**

## The axis before all the others: how many axes the device has at all

★**This page costs a controller in cycles. It does not cost it in DEGREES OF FREEDOM, and that is the
first thing a design loses.** A paddle gives **one** analogue axis, a driving controller **one**
relative axis, a joystick **two** discrete ones. Choosing the analogue smoothness of a paddle spends
the second axis, and that is not a setting to revisit later — it is the shape of the game.

Joe Grand, 2001, asked by Andrew Davie to *"allow some horizontal motion (perhaps with
inertia/acceleration) to allow me to shift the head back/forward"* 〔`200105/msg00041`〕, answered:
*"I will consider the horizontal motion of the drivehead.. I was thinking about that a bit, but **if I
use a paddle or driving controller, that wouldn't be possible**.. :/"* 〔`200105/msg00044`〕. ★★The
feature request was for movement; the reply is about the **connector**. Adding a direction was not a
feature at all — it was a change of device, and it would have taken the analogue feel with it.

★★★So read the sections below as *"what can be squeezed out of the device you chose"*, and read this
one as *"which device, and what it forecloses"*. `paddle.md` describes the paddle's read cost in
detail and this page describes the budget; **until now neither said that picking one costs an axis**.
Found by the mailing-list distillation (helper-2).

★**Packing the lines into a code costs the same thing from the other side.** grafixbmp, 2009, proposed
reading a stick's four direction lines as a 4-bit number — up+down and left+right never close together
in practice, so each player's nibble could carry 0–15. Thomas Jentzsch's objection: sixteen separate
actions can be defined that way, but they can no longer be combined — on a sixteen-button controller,
two keys pressed at once would read as one of the sixteen 〔AtariAge `topic/149883`; held here as
distilled notes, not the posts' text〕. A device spends axes when it is chosen; a code spends
simultaneity when it is designed. **Cited only, not verified.**

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

**The joystick figure is for the long form, and this repository ships both.** `bullets.asm`,
`road.asm` and `exerciser.asm` reload `SWCHA` for each direction (`lda SWCHA / and #bit / bne`);
`rpgmap.asm` reads it once and shifts each direction bit into carry (`lda SWCHA / asl / bcs` …). For
four directions that is 4 × (3+2+2) = **28 bytes** against 3 + 4 × (1+2) = **15**, and with no
direction held (every branch taken) 4 × (4+2+3) = 36 cycles against 4 + 4 × (2+3) = 24 — summed from the
same instruction table, not run. The short form reads diagonals only if each direction's code leaves
A alone: the shift form was first posted to the list with `bcc` jumps that could take only one
direction, and Erik Mooney's correction was `bcs` past each direction's code, *"do stuff for right
without changing A"* 〔stella-list `200207/msg00264`, `200207/msg00268`〕. `rpgmap.asm`'s `inc`/`dec` on
memory satisfy it.

**The ratio is the point.** Over 192 visible lines the paddle costs **1,536–3,072 cycles a frame**
against the joystick's 40–44 — **35× to 77×** — and an NTSC frame holds 262 × 76 = 19,912
cycles in total. So the paddle spends **8–15% of the whole frame**, and it spends it in the one
region that has no slack. The joystick spends 0.2%, in the region that does.

**The keypad row also has an ordering from the list** (its 400 µs number is below). grafixbmp, 2011,
ranked devices by the coding they take: joysticks *"minimal coding"*; paddles *"moderate to
considerable coding depending on amount used. Difficult because of time it takes for caps to discharge"*; keypads *"average to moderate
coding Cross-reading the buttons take some time to process"*; driving controllers *"Average coding.
Doing more than standard joysticks but quicker than paddles"*; Sega Genesis pads *"minimal coding,
quite similar to joysticks"* 〔AtariAge `topic/177790`〕. It ranks effort, not cycles, and has no
trackball entry. **Cited only, not verified.**

## What that forces

- **The joystick is the default not because it is simplest but because its cost is once.** A
  once-per-frame read can be moved into VBLANK and forgotten; a per-line read is a term in every
  kernel line's budget and competes with drawing.
- **A paddle game's kernel is designed around the paddle**, not the other way round: `paddle.md`
  measures 0 / 63 / 170 lines for three positions, which is the count *being* the value — the
  kernel cannot also be doing something expensive on those lines.
  Thomas Jentzsch, 2004, gives a reason that is not the budget: *"The time intervall between those
  checks should be (almost) identical, else the control will become "strange". Only because the
  kernel usually provides those constant time intervalls for a quite long time, the paddle checks are
  always(?) done inside the kernel."* Asked whether VBLANK would do: *"Only if you need only very few
  different values and are able to poll the hardware registers in constant intervalls."* In the same
  thread Christopher Tumber read four paddles in overscan for Quadraside with *"something like 13
  paddle positions"* 〔stella-list `200402/msg00230`, `200402/msg00238`〕. The trackball paragraph below
  records the opposite finding for the trackball. **Cited only, not verified.**
- **Devices that need continuous sampling do not fit.** The list said so in 1997 with a consequence
  rather than an argument: *"there are no perportional trakball games. It's too hard to constantly
  read a trakball, which is why the Atari trakballs have a joystick emulation mode"* — the trackball
  ships a fallback because software could not keep up, not because the hardware could not sense.
  Same source on the keypad: *"Other controllers like the keypad require an insane amount of time
  to read. Play Star Raiders and FEEL the delay between a keypress and a response."*
  〔Stella list, `controllers`, 1997-09, Glenn Saunders〕

  ★**And the 2000 answer to that delay is not a faster read — it is a design that needs fewer keys.**
  Manuel Polik laid out multi-tap text entry in one message: a 4×4 grid where each key carries four
  letters, and *"with repeated pressings you cycle through A>>B>>C>>D>>A>>B and so on. You just need a
  **simple counter and 'AND #03'** it in your code, when having 4 values on each key… If another key
  is pressed, just **reset the counter**. If it's the same, increase i[t]"* 〔`200011/msg00012`〕.
  ★★Erasing is the same mechanism with a blank in the cycle — *"it would cycle from
  BLANK>>X>>Y>>Z>>BLANK>>X"* — and the cursor is *"just **EOR #$FF** the graphics data that paints the
  char where the cursor is located"*, so neither costs a code path of its own. ★★★He even puts the
  letter frequencies in: *"you can rearange the above layout a bit, for example **starting the fifth
  key with 'S' as it is statistically more often used** than P,Q&R"*.
  ★★★★**Why it belongs on a page about budgets:** the expensive part of a keypad is finding *which*
  of sixteen keys is down, and multi-tap needs only *one key at a time plus whether it changed*. The
  design does not make the read cheaper — it makes the answer smaller. That is the shape to look for
  whenever a device's cost is in resolution: **spend the resolution on time instead of on the read.**
  ★★★★★**The list had the scheme three years earlier, with a variant that spends the console
  instead.** crackers, 1997, laid a keypad out like a telephone — *"Press the key once for the number,
  twice for the first letter, three times for the second letter, and four times for the third
  letter"*, with `#` as SPACE and `*` as ENTER — and then: *"Or if that's too much button pressing then
  use the select and reset switches for space and enter"*, over a layout with fewer letters to a key
  〔`199709/msg00327`〕. Both variants still need the keypad; the second moves two of its jobs onto
  console switches. **Cited only, not verified.**

  ★**The trackball half has a second source from 2015, and a witness against it.** Crispy:
  *"Polling the trackball requires a huge amount of CPU time. In order to get an accurate picture of
  what the trackball is doing, it has to be polled every few scan lines, and preferably, every scan
  line"* — and his answer moves the counting off the CPU: two 4-bit up/down counters clocked by the
  trackball's pulses, read *"from address $280 once every frame during blanking"* as two's-complement
  deltas 〔AtariAge `topic/239525`; CX-22-type signalling only〕. Against that, Thomas Jentzsch, whose
  Trak-Ball hacks poll only inside the kernel, found the poll *"surprisingly tolerant to irregular
  timing"* — every 8th scanline, gaps anywhere, *"A poll e.g. in 5, 14, 31 and 39 would work too"* —
  and put it down to the trackball being relative to the previous read, where a paddle starts from
  scratch every frame 〔AtariAge `topic/245239`〕. So "too hard to constantly read" holds for a
  *continuous* read; how sparse a read still works is an open number. **Cited only, not verified.**

## Reading the shape, not the value

`docs/techniques/game-states.md` fixes the discipline that makes any of these affordable:
**snapshot the inputs once per frame and compare against the previous frame** — edges, not levels.
That converts a device read into a fixed per-frame cost and keeps hold-to-repeat bugs out. It is
also why the joystick's 40–44 cycles is the *whole* cost and not a per-line one.

★**An edge has a cost the level did not: the player cannot see it.** Glenn Saunders, 2003, on *Death
Derby*, where a joystick on a Y-cable adapter sets the gear alongside a driving controller: *"Tap
forward to kick the car into forward gear, tap back to go into reverse. A visual cue is important
since I don't want to force people to hold the joystick forward or back to stay in forward or reverse
gear. The joystick is just a latched toggle switch. So I need something in the score to indicate
that. If I had used the difficulty switches, I wouldn't need the indicators as much, at least if you
were using a six-switcher ;)"*; that build showed the gear by *"toggling the blocks in the corners of
the screen"* 〔stella-list `200302/msg00212`〕. Held, the state is in the player's hand; latched on an
edge, it is in RAM and has to be drawn; on a console switch, it is on the machine (our reading). So
the read shape can cost picture as well as cycles. **Cited only, not verified.**

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
- **The trackball's ⬜ is a third kind.** `controllers/` has no trackball either
  (`ls Gopher2600/hardware/peripherals/controllers/` → `doc.go gamepad.go keypad.go paddle.go stick.go`):
  the keypad's ⬜ is a measurement not yet taken on a peripheral the engine has, the trackball's cannot
  be taken here at all. And *the trackball* is more than one device. Eckhard Stolberg, 1999, reported
  a CX80 (triangular buttons) working as an Atari ST mouse, *"while the round button versions are not.
  I wonder why Atari changed the protocol between the two models"* 〔stella-list `199902/msg00012`〕.
  Bob Colbert's round-button unit had worked under neither the ST nor the Amiga setting of his
  Stell-A-Sketch, which he wrote before having a trackball: *"It is a simple matter of using a
  different lookup table to get each device to work"* 〔`199902/msg00009`〕. Asked whether it had been
  in trak-ball mode: *"Yes, absolutely sure. I used to have a program that I used to determine what
  the code was for each device and determined that the trackball was indeed different from the ST
  mouse"* 〔`199902/msg00022`〕. On fitting one into an existing kernel: Oliver Scholz, 2001, having
  dumped and disassembled Missile Command, patched it for the trackball — *"the kernel is pretty much
  exhausted, and the few areas where there is space to insert something, are insufficient for smooth
  motion. It worked though"* 〔`200110/msg00525`〕. **Cited only, not verified.**


---

## The same machinery, three ways — and this page had two of them (2026-09-07)

Edge detection, deliberate throttling and deliberate repetition are one counter with three policies.
`game-states.md` records the first as *"hold-to-repeat bugs gone"* — a record of **removing**
repetition — and `design-principles.md` records the second as a turn-rate governor. The third had no
line anywhere, and a held direction needs exactly it.

Glenn Saunders, 1996, on a Tetris in progress: *"When moving the joystick left and right, **when you
HOLD the joystick, it should keep moving the piece. It should not require extra taps.** Perhaps some
extra **'grace time' when a piece is lying flat** so you can 'slide' L-shaped pieces into place after
they are on the ground"* 〔`199612/msg00012`〕.

Measured (`internal/emu/autorepeat_test.go`, `roms/litmus/litmus_autorepeat.asm`), both policies
running off the same button with `DELAY = 16` and `REPEAT = 8`:

| held for | edge-only | auto-repeat |
|---|---|---|
| 1 frame | 1 | 1 |
| 16 frames | 1 | 1 |
| **17 frames** | 1 | **2** |
| 25 frames | 1 | 3 |
| 60 frames | 1 | 7 |

★**Edge detection alone never repeats, however long the button is down** — correct for a menu, wrong
for a direction, and the two policies differ by nine lines of kernel.

★★**The two numbers are the design.** `DELAY` is how long a hold must last before the game decides it
was deliberate; `REPEAT` is how fast it goes after that. They are in RAM so a scenario can assert them
rather than a person judging them by feel.

★★★**The grace window is the same counter with a different trigger** — a state change rather than a
button edge — and it is *not* measured here, because this litmus has no landing to hang it on. Named
so it is not mistaken for covered. Found by the mailing-list distillation (helper-1).

★★★★**A third time axis, and it starts on the release.** sohl's *Sweep Shot* (2022, 4K) asks one
button *when* (*"you push the objects stronger if the beam activates on or very close to an object"*),
*how long* (*"press too long and it looses effect"*) and *how soon again*: *"After you release the
button, the beam will require a fraction of a second to recover before it can be utilized again"*
〔AtariAge `topic/345816`〕. The first is the edge above; the second is the held-frames count that
`DELAY` already keeps, used as a ceiling instead of a threshold; the third is the same counter started
on the **release** edge, gating the next press. **Cited only, not verified** — the ROM was not run, and
the thread does not say how it counts the recovery.

★★★★★**Where the counter starts matters as much as its two numbers.** B. Watson posted *Poker
Solitaire* in 2001 with *"Joystick movement is a little wonky"*; Roger Williams: *"You need to reset
the debounce counter at the moment the joystick is pushed in a new direction. Right now you're just
keying in to a slow background counter and the initial reaction could occur immediately or after a
time interval, depending. (Fire button also seems to have this defect, it should respond immediately
regardless of the background debounce timer's state.)"* 〔stella-list `200111/msg00224`〕. A week later
Watson's to-do list still read *"Joystick & trigger debounce, instead of blindly ignoring them for 15
frames between reads"* 〔`200111/msg00382`〕. `litmus_autorepeat.asm` starts its counter on the press —
it steps and loads `DELAY` on the edge (read from the source). `autorepeat_test.go` presses at one
fixed frame, so it does not show that the first step is independent of the counter's phase, which is
what Williams was pointing at; it has one button, so a counter shared by four directions is not
exercised either. **Not verified.**

## The keypad read, as one routine (2026-09-30)

The sections above price the keypad's wait; this is the read itself, from a 2012 routine
〔AtariAge `topic/204852`, wickeycolumbus〕. The keypad is 4 rows × 3 columns. Drive one row low through
`SWCHA`, wait, test the three columns on `INPT4`, `INPT1` and `INPT0` (bit 7 clear = pressed), and count
`X` down so it holds the key number when a column hits:

```
        lda #$FF
        ldx #12
        clc
.new_row
        ror                 ; walks a single 0 down from D7: rows on D7..D4, the left port's nibble
        sta SWCHA
        ldy #120
.wait   dey
        bne .wait
        bit INPT4
        bpl .keypressed
        dex
        bit INPT1
        bpl .keypressed
        dex
        bit INPT0
        bpl .keypressed
        dex
        bne .new_row
.keypressed                 ; X = 0 when no key is down
```

The wait is 120 × (`dey` 2 + `bne` 3) = 600 cycles, about 500 µs — longer than the Guide's 400,
because (per the thread) alex_79 raised it for ageing capacitors on hardware. **The thread also names
an emulator difference:** in Stella a `SWCHA` write pulls the `INPT0`/`INPT1` lines low at once, so the
routine works there with no wait at all — passing in Stella does not imply passing on hardware, only the
reverse. That is the same looseness this engine shows (`litmus_swacnt` band 5, above). **Cited only,
not verified** — the thread is held here as distilled notes, not its text; the routine was not run, and
the `SWACNT` setup that makes the nibble an output is not part of it.

**The list's first keypad read was a question, and what stopped it was `VBLANK`.** John Matthews,
October 1996 — the month the archive held here begins — *"roughly, and I don't know that
this is right"*: *"Set Port A to output / Put a #$10 in SwchA (checks row 1) / Wait a while (400 usec)
/ Get values of Inpt0, Input1, Input4 / Draw Screen / Repeat with #$20, #$40, and #$80 to check rows
2, 3, 4 respectively"* 〔stella-list `199610/msg00035`〕. His row values set one bit; the routine above
clears one. Three days later he *"can't seem to get the timing right or something"*
〔`199610/msg00038`〕, and two days after that he had found part of it: borrowed code *"still was
writing a 1 to D7 of VBlank. This was dumping Inpt0, Inpt1, Inpt2, Inpt3 to ground and thus I
couldn't read them properly"*; with that fixed, *"I am getting a reaction from columns 1 and 2"*
〔`199610/msg00039`〕. Two of the keypad's three columns are read on inputs that the paddle's dump bit
grounds, so a frame that keeps `VBLANK = $82` through blanking (`paddle.md`) cannot read them there —
derived, Not verified. The rest is **Cited only, not verified.**

## A port as a serial line (2026-09-30)

`fundamentals-audit.md` records the send side (a dumper that talks serial out of a joystick port). The
receive side, and how its timing was chosen, is in a 2016 routine by alex_79 that takes 19200 baud from
a PC on the right port 〔AtariAge `topic/256433`〕. The order of the design is the reusable part: fix the
tolerance first (*"max error allowed 2%"*); list the cycles one bit lasts on each machine (*"61.6 62.1
92.4 93.2"* for 2600 PAL, 2600 NTSC, 7800 PAL, 7800 NTSC); pick one integer per machine that fits both
regions (62 on the 2600, 93 on the 7800); write down the error it leaves (*"PAL: 52.44 us (error
+0.69%) NTSC: 51.96 us (error -0.23%)"*). Recomputed here from the colour clocks in `pkg/audio/audio.go`
(3579545 and 3546894 Hz, CPU = ÷ 3): 62 cycles is 51.96 µs on NTSC and 52.44 µs on PAL against a
52.083 µs bit — the same two errors. Each bit comes in with one instruction, `lsr SWCHA` (6 cycles):
the routine's input is `SWCHA` D0 (`BITIN = %00000001`), so the shift drops it straight into carry and
`ror buffer` assembles the byte. Its own comment states the condition — *"PORT A must be configured as
INPUT !!"* — and `lsr` is a read-modify-write, so it also writes `SWCHA` back. **Cited only, not
verified** — not run here.

The same ports carry the AtariVox and SaveKey (I2C EEPROMs on a controller port), and a 2022 copy
utility drives one on each port at once — the source in the left port, the destination in the right
〔AtariAge `topic/332726`〕. **Cited only, not verified.**

Here the direction of each line set how many consoles one cable could join. hornpipe2, 2009, linking 2600s
through the joystick ports, used a protocol for — in theory — up to four consoles both ways,
demonstrated with two: FIRE cannot take part because it is input-only, which leaves four digital
lines and so at most four consoles. batari, in the same thread: one-way should work, and a two-way
link has to watch for contention where one line serves as input and output 〔AtariAge `topic/153150`;
held here as distilled notes, not the posts' text〕. **Cited only, not verified.**

## When the players multiply, the cost leaves the read (2026-09-30)

Thomas Jentzsch's *Pac-Line Panic* (2024, 4K) takes up to eight players at once — *"up to 8 players
simultaneously (with QuadTari, else 4 players) using paddle buttons"* 〔AtariAge `topic/368501`〕; a
QuadTari also changes what `VBLANK` D7 means, see `known-traps.md`. One button per player is the
cheapest read on this page, and what eight players cost is **state**: *"The biggest programming
challenge in this game was not the 4K ROM limit, but the 128 bytes RAM."* Kept naively each of the
eight rows needs 27 bytes — 216 in all, more than the machine has — and the same post lists the cuts that
brought the rows to 100 bytes and left 28 for everything else: one shared speed because *"the speeds
are identical for all eight rows"*, animation derived from position instead of stored, a reserved
position value meaning "not shown" instead of a status bit. So a multi-player design is budgeted first
in **bytes per player × players**, and only then in cycles per read. **Cited only, not verified** — the
byte counts are his; nothing here was built.

## The driving controller's read (2026-10-02)

The table has no row for it; *Not measured here* above says why it is cheap — a Gray code in two
`SWCHA` bits, read once a frame. What it gives up is resolution. Lee Fastenau, 2004, adding it to
*Reflex*: *"It is not a potentiometer, but a series of switches with a very low resolution (only 16
updates per full rotation). This was unacceptably slow in my first implementation, so I now detect if
there was movement in the last frame and then multiply the current movement by two if there was."*
〔stella-list `200404/msg00426`〕

Decoding the two bits, Thomas Jentzsch, 2001, in three steps 〔`200110/msg00480`〕: let a table hold
the value expected *next* for a left turn and another for a right turn, and compare the reading with
both; notice that *"the right table values are the left values EOR %11"*, so one table serves, with
`eor` in place of `cmp` before `beq`; then drop the table:

```
        lda SWCHA
        and #%00000011      ; missing from the post; his correction, 200111/msg00133
        tay
        eor last            ; 01 or 10 (else: abrupt twist)
        sty last
        dey                 ; -> y = -1..2
        cpy #2
        sbc #1              ; -> a = -1, 0, 1
        beq .right
.left:
```

His own caveats: *"very hard to explain"*, *"comes very close to the optimum"*, *"You need some
additional code here to recognize abrupt twists of the wheel"*, and *"I haven't tested the code"*. On
8 November he explained it — `cpy #2` is unsigned, so `-1` sets the carry as well as `+2`, and the
result is 0 on every right-turn step and ±1 on every left-turn step 〔`200111/msg00102`〕 — and on 9
November he posted the missing mask 〔`200111/msg00133`〕. The 0 / ±1 table was recomputed here in
Python over his eight transitions (left 00→01→11→10, right 00→10→11→01): the arithmetic, not a run on
the 6507. The mask keeps the right port's two bits; for the left port, shift `SWCHA` down four first,
as Eckhard Stolberg's version of the read does 〔`200110/msg00472`; Jentzsch points to it in
`200110/msg00484`〕. **Cited only, not verified.**

## A second button, and telling pads apart (2026-10-02)

A Sega Genesis (Mega Drive) pad gives the 2600 two buttons with no rewiring: B reads on `INPT4`, where
the stick's fire button is, and C on `INPT1`; reveng's 2010 proof of concept paints one colour per
button 〔AtariAge `topic/158597`; held here as distilled notes of the opening post only〕. The engine's
`Gopher2600/hardware/peripherals/controllers/gamepad.go` models that device: on the left port it
writes the button to `INPT4` and the second button to `INPT1`, and holds `INPT0` high while plugged
(read from the source, not run). reveng again, 2011: a Genesis pad *"+ one 10 cent pull-up resistor
can get you 4 buttons too , maybe even 7"* 〔AtariAge `topic/177790`〕. **Cited only, not verified.**

The Booster Grip's two buttons also sit on the paddle inputs (Omega Race reads them on `INPT0` and
`INPT1`, `kernel-micro-idioms.md`), and are read far less often than a paddle. Glenn Saunders, 2004,
arguing that four driving controllers would disturb Indy 500's kernel less than paddles, counted *"the
booster grip stuff which as Thomas explained a while back only involves polling the pot lines at the
very top and bottom of the screen which doesn't add much overhead"* 〔stella-list `200402/msg00228`〕 —
Jentzsch's explanation at second hand. **Cited only, not verified.**

Telling which pad is plugged in can rest on a state a normal stick does not produce. dionoid, 2020: a
Hyperkin Ranger gamepad, for a short time after power reaches it, shows all four of the left stick's
direction switches closed, and the check run at start-up is `lda SWCHA / and #%11110000 /
beq RangerGamepad_detected`. The same state has a side effect: in Pitfall! the timer can start
counting down at once 〔AtariAge `topic/304283`; held here as distilled notes〕. `paddle.md` (*Paddle or
joystick, detected at power-on*) makes the same move on the paddle lines, with the Genesis pad as its
trap. This one cannot be tried here: the engine will not produce opposing directions
(`known-traps.md`, *Opposing joystick directions cannot be tested here at all*). **Cited only, not
verified.**

## One device per port (2026-10-02)

The two ports need not hold the same device. Asked by krayt88, 2008, whether one player could use a
joystick and another a paddle, seagtgruff: *"As long as one type of controller is plugged into one port, and
the other type of controller is plugged into the other port, and you know which type of controller is
plugged into which port, you can read each port separately"* — and since paddles come in pairs, *"three
players-- one player with a joystick, and two players with paddles"* 〔AtariAge `topic/119959`〕. *"You
know which type"* is the condition; `paddle.md`'s power-on check is one way to meet it. An example in a
prototype: Starpath's unfinished *Sweat!*, per PatMan, 2001 — *"you use the paddles on the left
controller input to select the event, and the joystick on the right in the same manner as Activision
Decathalon"* 〔stella-list `200106/msg00017`〕. By the table above the two reads land in different
places, the joystick's in VBLANK and the paddle's across the visible lines; neither post says how the
two were scheduled, or whether *Sweat!* reads both in one frame. **Cited only, not verified.**

## A read that takes the whole picture (2026-10-02)

A light gun prices its read in picture rather than cycles. Colin Hughes, 1999, on how light guns
work: *"The whole screen flashes white for a frame, and you get a hit when the beam update hits the
part of the screen that the gun is pointing at. ( If you have a light background you can sometimes get
away without a flash ) Simply count cycles and scanlines to work out the position... On the VCS you'd
be better of with the flash option - as you don't need to maintain a screen kernal for that frame -
allowing better accuracy for the gun."* 〔stella-list `199905/msg00052`〕 That is advice, not a 2600
build, and the same thread described the NES's Duck Hunt differently: *"The whole screen in Duck Hunt
(at least the blue background) flashes for a frame, and then the white boxes appear around the ducks
for a frame or two"* 〔`199905/msg00056`〕. The next day Eckhard Stolberg posted a VCS test program built
on Sentinel's gun-detection code: *"The Vertical resolution can be extended to 190, but horizontally
53 positions is all you can do."* 〔`199906/msg00030`〕 `known-traps.md` (*A light gun on an LCD*) has the
2017 CRT method — a black frame, then a white square per target — and its failure on LCDs. The engine
has no light gun (`controllers/`, above). **Cited only, not verified.**
