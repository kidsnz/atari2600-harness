# Technique — game state machine (title / play / game-over, console switches, attract)

**Goal:** the structural skeleton every real game ships with: a state machine driving
title → play → game-over → title, console-switch handling (RESET / SELECT / difficulty), and an
attract mode — all verifiable frame-by-frame.

Demo: `roms/techniques/game_states.asm`.
CI: `scenarios/game_states.json` (full lifecycle over ~1100 frames, golden).
New hardware verification: `litmus_swchb` + `scenarios/swchb.json` — **SWCHB read side verified**
(D0 RESET / D1 SELECT active-low, D3 color/BW, D6/D7 difficulty), driven by the harness's
extended `SetPanel` (now also `color` / `p0pro` / `p1pro`) and scenario panel inputs.

## Structure

- **One state byte** (`state` = 0 title / 1 play / 2 over) asserted directly in scenarios.
  - The other layout keeps the state in **bit 7** (a second one in bit 6): `BIT state` then
    `BMI`/`BPL` or `BVS`/`BVC`, no `cmp` and no mask, and the low six bits stay free — SpiceWare:
    *"allowing them to be used for something else such as which game variation the player has
    selected"* (AtariAge `topic/292204`). That `BIT` copies the memory's bit 7 into N and bit 6 into V
    is measured here (`internal/emu/bitflags_test.go`); the layout itself is **Cited only, not
    verified** — the demo uses the values 0/1/2.
  - **A counter beside the flags, and a flag that is also an offset.** Lee Fastenau, 2004, in *Reflex*:
    *"a cool gameMode byte that contains a 3-bit counter and 5 mode flags"*
    〔stella-list `200404/msg00109`〕, and, in a later post where he is *"trying really hard to reuse
    variables and compacting flags"* for RAM, of one of those flags: *"the joystick/driving controller flag
    is dual purpose. It's stored as bit D3 in the gameMode variable, which equates to decimal 8, which
    just happens to be the offset of the joystick and steering wheel icon graphics"*
    〔`200404/msg00113`〕 — masking the byte with `#$08` leaves 0 or 8, usable as the table offset
    with no shift (our reading; his attached reflex16.asm is not held here). **Cited only, not verified.**
- **Input snapshots + edge detection**: read INPT4 + SWCHB once per frame into "current" cells,
  compare with "previous" cells; transitions fire on edges only (hold-to-repeat bugs gone).
  - **RESET needs this too.** An init that runs on every frame RESET is down keeps re-initialising for
    as long as it is held; in one 2019 homebrew that showed as a screen roll with artifacts, which its
    author wondered might be his bankswitching (*"Perhaps"*; AtariAge `topic/287799` — a diagnosis
    by two replies, not confirmed in the thread). nukey-shay's two cures there: a RAM bit remembering the switch (what
    `prevRe` does here), or the cold start placed at the top of overscan so a whole frame is drawn
    before the reset test runs again. **Cited only, not verified.**
  - **Shorter ways to write the same test** — the demo keeps one previous cell per input
    (`prevFi` / `prevRe` / `prevSe`) and tests them one by one. **Cited only, not verified:**
    - `EOR` the current read against the saved one and a 1 is left exactly where something changed:
      every direction bit of SWCHA in one test, `beq` when nothing moved (SpiceWare, AtariAge
      `topic/306641`); on SWCHB, whether any console switch moved at all — nukey-shay: *"you can
      clear the score and whatever if ANY switch has changed position"* (`topic/168437`).
    - One button's previous state can live in one bit and travel through the carry (nukey-shay,
      `topic/286661`): `lsr flag` / `lda INPT4` / `bcc held` / `bmi up` / *(on press)* /
      `held:` `up:` `rol` / `rol flag`. The carry is clear afterwards; for a flag in bit 7, the first
      `lsr` becomes `asl` and the last `rol` becomes `ror`. The middle `rol` takes the button from bit 7
      of A, so the press code must not leave D7 set — **Not verified**. The same
      thread's other form keeps INPT4's masked D7 in a byte and `cmp`s it, which needs masking if that
      byte's other bits are in use.
  - **Read every frame, the fire button hardly needs the latch.** Eckhard Stolberg, 2002: *"For reading the
    joystick buttons the latches are pretty much unnessesary. As long as you check the joystick at
    least once per frame, it's better to leave bit D6 of VBLANK zero all the time"*; with D6 set, a
    press keeps D7 of INPT4 at 0 *"even after the button has been released again"*, and a new press
    shows only after D6 is dropped to zero and set again 〔stella-list `200207/msg00300`〕. The case
    for the latch, as Ruffin Bailey relayed what Stolberg had told him (*"as I understood him"*): a
    kernel that checks the button only every third screen or so — *"If you throw down the latch, you
    can preserve the button press until you deal with it"* 〔`200207/msg00298`〕. That the latch holds
    a press after release is measured here (`litmus_input`, `docs/verified-coverage.md`: INPT4 stays
    pressed ≥3 frames after release). The demo reads INPT4 every frame and its VBLANK writes are only
    `#2` and `#0`, so D6 is never set. When to use the latch is **Cited only, not verified.**
  - **The archive also holds D6 the wrong way round, under a lesson's title.** Ruffin Bailey, July 2002,
    *"Reading joystick buttons & VBLANK -- a lesson for a newbie"*: an Erik Mooney post from 1997, as he
    read it, *"said VBLANK needs to have a 1 in D6 if you want to be able to read the joystick buttons"*,
    so he changed the VBLANK write in Nick Bensema's *How to Draw a Playfield* to `#%01000000`
    〔stella-list `200207/msg00034`〕. The 1997 post does not say that; its button test ran *"with zero in
    bit 6 of VBLANK, so I'm not worrying about latching"* 〔`199703/msg00328`〕. Eckhard Stolberg's reply:
    *"Actually it's the other way around. If you want to be able to read the joystick buttons, you need
    to put a 0 in D6 of VBLANK. Otherwise the button ports get latched"* 〔`200207/msg00045`〕. Of that,
    `litmus_input` measures only the latched side: its frame loop writes VBLANK only as `$42` and
    `$40`, D6 set, and INPT4 stays pressed after release; it reads no button with D6 = 0. The
    correction is a separate reply, so a search that finds only Ruffin's `200207/msg00034` gets D6
    backwards (our reading). The rest is **Cited only, not verified.**
  - **Where an ARM runs the game, one call can both draw and read.** ZackAttack, 2023, sketching an API
    for the ELF support of UCA-based cartridges, had `draw_frame()` draw the frame and capture input.
    splendidnut: *"I would decouple the frame drawing from the input gathering. They are two different
    things. I know that's probably a convenient way of doing it due to switching between the 6507 and
    the ARM, but there's got to be a better way to handle that."* ZackAttack: *"Yeah, that's why I combined them. That way the game code never puts itself
    in a position where it can get the two out of sync"*, offering `set_input_type()` and
    `get_last_input()` as the other shape (AtariAge `topic/347047`). An argument about that API's
    design; nothing was run. **Cited only, not verified.**
- **The colour/B&W switch as an input — and where it is not one.** The demo reads D0/D1/D6 only;
  D3 is read by `litmus_swchb` above.
  - **As a selector, read by its edges.** In the *INV* thread, 2004, Erik Mooney wished the switch had
    a third state, for both PAL-50 and PAL-60; Adam Wozniak: *"Three states: NTSC, PAL50, PAL60"* /
    *"Transition of the Color-B/W switch from 0 to 1 or 1 to 0 causes a state transition."* 〔stella-list
    `200404/msg00016`〕 Mooney used it in *INV* on one direction only: *"Cycling the video mode is
    done when the switch changes from B/W to Color ... Essentially, this treats the Color/BW switch as a
    momentary switch like Select and Reset"*, so *"NTSC users (the most common case) don't have to
    worry about anything at all, since the game will always just boot up in NTSC and stay there"*
    〔`200408/msg00005`〕. The concern Mooney was answering is Lee Fastenau's, who had meant to use
    Wozniak's method and was now proposing to read a joystick, or SELECT/RESET, at power-on instead:
    *"That's generally the problem with the non-toggle console switches, including the
    difficulty switches: it requires the user to alter the state of their system from what they might
    consider the "normal" state"* 〔`200408/msg00001`〕; he then took both the power-on hold and the
    cycle 〔`200408/msg00010`〕 (power-on reads: `design-principles.md`, *"The switches can also be read
    once, at power-on"*). The demo's `prevSe` test does not carry over as is: it
    fires when the bit goes 1→0 (SELECT is active-low), which on D3 (1 = Color) is Color→B/W, the
    opposite of Mooney's direction, so the snapshot's sense must be inverted; Wozniak's both-edges form
    fires whenever the bit differs from the saved one (read from `roms/techniques/game_states.asm`; the
    D3 versions are not built here). **Cited only, not verified.**
  - **On a 7800 the switch does not stay put.** Kirk Israel in the same *INV* thread: *"will the
    color/b-w switch idea work well with a 7800? Didn't they turn it into more of a 'momentary contact'
    kind of thing?"* 〔`200404/msg00014`〕 — not answered there. The 1997 answer (Bob Colbert, Chris
    Wilkson's software latch, Piero Cavina) is in `known-traps.md`, *"A colour/B&W switch read as a
    position is a button on a 7800, and it cannot be held"*. Andrew Davie's
    *"On the 7800 it's a momentary switch"* is in `design-principles.md` (*"The same switch is a
    different switch on a 7800"*). **Cited only, not verified** — the 7800 is not modelled here.
  - **A pause on the switch that also works on a 7800, without detecting the console.** Dionoid, in
    the thread whose Lode Runner routine `design-principles.md` cites, posted an earlier one: toggle the
    pause at once when the switch (or the 7800's button) changes, *"but delay storing its state (i.e.,
    BIT3 of SWCHB) by 32 frames, which is around 0.5 second on NTSC. This allows for the 7800 pause
    button to return to its original state as the user releases the button again"*; the cost, in his
    words: *"pushing the pause button twice within half a second isn't handled"* (AtariAge
    `topic/194119`). His code counts the frames in the pause byte's low bits until bit 6 sets, and
    notes 64 frames (about a second) as the alternative. **Cited only, not verified.**
  - **On SECAM there is nothing to read.** The Stella Programmer's Guide (*PAL/SECAM conversions*): a
    SECAM console *"takes the PAL software, but the console color/black & white switch is hardwired as
    black & white"* (`ingest.md` quotes it for the colours). So an edge-cycled mode never steps there,
    and a level-read selector — Nick Bensema, 1997: *"turn the B&W switch into a PAL/NTSC switch"*
    〔stella-list `199703/msg00166`〕 — always sees B&W (our reading). **Cited only, not verified** —
    SECAM is not modelled here.
- **Frame logic under TIM64T in VBLANK** (the dynamic-multisprite pattern): state branches have
  wildly different lengths; the timer keeps the frame at 262 lines regardless.
- **title**: SELECT cycles the game variant (0-3); RESET *or* fire starts; 300 idle frames turn
  on attract (background pulse), any input clears it.
- **play**: a drifting sprite (HMP0=$F0 + one HMOVE per frame — the cheapest motion); the round
  timer counts double when **P0 difficulty = A/Pro** (SWCHB D6), so Pro rounds are half as long —
  the scenario discriminates this by timing (B: over at ~320f, Pro: ~160f).
- **game-over**: 120 frames back to title; RESET restarts immediately.
- **Deterministic state entry**: `EnterPlay` strobes RESP0 at a fixed cycle after WSYNC, so the
  sprite X is identical on every entry (golden-stable).
- **What an entry resets, and what must not come back.** Two bugs from the list, of opposite kinds.
  Paul Slocum, 2002, on *Space Treat*: *"your ship slows when you're energy gets low. But if your
  energy gets low and then you die, your next ship will have full energy but will still be slow"*
  〔stella-list `200212/msg00347`〕; the author: *"Ouch thanks! I must've broken it recently...!"*
  〔`200212/msg00348`〕 — the new life reset one value but not one that followed it (our reading).
  Erik Mooney, 2004, on *INV*: *"if the lowest row of invaders reaches the shields, the shields
  disappear correctly, but then if invaders are shot so that there are no longer any invaders down
  that low, the shields reappear. Fixed it to permanently remove the shields for that wave in that
  case"* 〔`200404/msg00003`〕 — the shields followed the current lowest row instead of a record that
  they had been destroyed (our reading). In the same *Space Treat* thread Albert Yarusso on the
  convention the title state here follows: *"My experience shows that Reset usually starts a new
  game, and oftentimes (depending on the game) the joystick button will do the same"*, where that
  game's Reset went to the splash screen 〔`200212/msg00355`〕. **Cited only, not verified.**
- **A flag set while an effect is still running, carried into a state that never writes it** (our
  reading) — the same kind as the *Space Treat* bug above, from AtariAge. In *Assault* (also *Sky
  Alien*) a video showed a kill screen at Wave 13, with every enemy invisible and unkillable. Thomas Jentzsch,
  2021, *"After looking into the code"*: *"The bug occurs if you hit the last alien just before it
  becomes invisible (so that the invisible flag is set during its explosion) and the next wave is with
  enemies jumping up and down."* *"During the jumping wave, the invisible flag isn't updated (or reset)
  so that the invaders stay invisible forever."* He posted the wave table and wrote that it *"can
  happen earliest in wave 13, then 15, 17, 19..."*, and of his patch: *"The attached ROM should fix the
  bug. It resets the invisible flag during jumping waves."* 〔AtariAge `topic/325222`〕 Nobody in the
  thread reports trying the patch. **Cited only, not verified** — the game was not run or disassembled here.

## Verified
- Full lifecycle (11 asserts over ~1100 frames): variant select, both start paths, B vs Pro round
  lengths, game-over timeout, return to title, attract flag.
- **Dogfood**: `fieldtest -auto` on this ROM reports `auto-start: reset` — the harness's title-
  screen escalation detects exactly the start method this technique implements.

## Integration notes
- A real game replaces the play-state body and keeps the dispatcher/edges/timer shell as is.
- SELECT-cycled `variant` is where game options live (number of players, speed class …).
- Attract can swap to a self-playing demo; the flag + idle counter here is the hook. Nick Bensema,
  1997, after describing the colour-cycling form: *"Not all games use the color cycling method as an
  attract mode. Some games have a "demo game" running, while others run a cute animation sequence"*
  〔stella-list `199707/msg00021`〕. The colour form, with the B&W switch folded into the same store,
  is in `design-principles.md` (*"Attract-mode colour cycling and the B&W switch can share one store
  path"*, with the lookup-table form beside it) and `kernel-micro-idioms.md`; this demo pulses the
  background instead and does not read D3. **Cited only, not verified.**
- **For a computer opponent, the advice was a state machine, and the worry was RAM.** In a 2003
  thread about making Combat one-player, Chris Wilkson: *"If you start with the combat disassembly,
  you've got 2k of ROM space to work with even before you start bankswitching. That's a lot."* —
  *"The real trick will be fitting the AI in RAM"* 〔stella-list `200302/msg00041`〕; Andrew Davie:
  *"without question you should be considering a FSM for your AI"*, *"You can do an awful lot of
  intelligent-seeming things with simple state-machines. And they remove the need for lots of
  conditional code"* 〔`200302/msg00043`〕. Advice to one project at its start. **Cited only, not
  verified.**
- **Adding a menu to a game built without one** — SpiceWare's order, from a 2019 thread about old
  games (AtariAge `topic/291009`): make room in ROM (easy from 2K; beyond that it means adding
  bankswitching), find the RAM that holds the variation and the RAM the menu can borrow, write the
  menu for that game, point the cartridge's init at the menu, give the game a new init that
  re-initialises the borrowed RAM, and send game-over back to the menu — and let the last score stay
  visible, since a menu that replaces it at once hides it. A wrapper that only hands values to an
  unmodified game cannot take over SELECT, because the game tracks SELECT every frame.
  **Cited only, not verified.**
