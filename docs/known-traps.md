# Known traps — "passes in the emulator, breaks on real hardware" (and other silent killers)

A catalog of the Atari 2600 traps mined from AtariAge that **do not show up as a failure in a naive
emulator run** but corrupt the result on real hardware (or in a stricter emulator). Past Pong attempts died
on exactly this class of bug ("unverified timing / positioning"). This is the **pre-flight checklist** the
authoring loop runs before shipping a kernel, and the **spec for `scripts/check_traps.py`** (the future
trap linter, [[knowledge-activation-architecture]]).

Detection column: **static** = a source-text linter can flag it · **runtime** = needs a sim assert /
`breakif` / `trace_clocks` · **manual** = judgment / pixel compare.

> **★coverage, measured 2026-07-30 — "static" says a linter COULD flag it, not that one does.** 10 rows below
> are marked static; `scripts/check_traps.py` implements **7 detectors** covering: unstable illegal opcodes,
> `LAX #imm`, `NOP $00`/`BIT $00`, variables in the `$F8-$FF` stack-collision zone, missing `CLD`/`CLEAN_START`
> (which is also the post-reset-undefined row), and **new today**, reads of a write-only TIA register and
> **`STA` to ROM**. Page-cross `+1` is handled by `cyclebound`'s abstract interpreter rather than here. Still **unimplemented**:
> bank-move page-cross misalignment. **F8/F6/F4 random boot bank is now checked at ROM level** (see below). **Read of a write-only cartridge
> hotspot / SuperChip write port is NOT statically decidable** and its row is re-marked below.
> The parenthetical below calling `check_traps.py` "the future trap linter" is stale — it exists and runs
> in CI on every push; what is future is that one row.

**New static trap (2026-07-30): reading a write-only TIA register.** The TIA answers reads only at
`$00-$0D` (`CXM0P`..`INPT5`; verified in Gopher2600 `cpubus.go TIAReadRegisters`). Everything from `$0E` (PF1)
to `$2C` (CXCLR) is write-only, so `lda GRP0` returns bus residue — which an emulator can make look stable and
repeatable while real hardware does not. Measured false positives: **0 across 123 known-good ROMs**
(31 techniques + 92 litmus), and the detector is not merely silent — the same scan matches 509 read-operand
pairs in that corpus, so it is looking at something. Bait-tested in `--selftest`; negative control: disabling
the rule fails the selftest by name.

**A setup database can patch our ROMs, and its path is relative — measured 2026-09-05.**
`internal/emu` attaches every cartridge through `setup.AttachCartridge`, and the engine's
`setup/doc.go` says what that is for: *"Toggling of panel switches / **Apply patches to cartridge** /
Television specification ... `<DB Key>, patch, <SHA-1 Hash>, <patch file>, <notes>`"*. An entry keyed
by a ROM's SHA-1 can flip the console switches, **rewrite the cartridge's bytes**, or change the TV
standard, and `setup.go` will *"silently ignore absence of setup database"*.

★The path is not one well-known location. `resources/dev_path.go` is `//go:build !release` and
returns the **relative** string `.gopher2600`; this repository builds without the `release` tag. So
which database is consulted depends on the directory the command ran from — and `CLAUDE.md` requires
that variation: *"Run commands from each repo's root."* **Two mandated working directories, two
possible databases.**

Measured: four `.gopher2600` directories exist (umbrella root, `roms/`, `harness/`, `sandbox/`),
**all four empty**; `~/.gopher2600` does not exist. `resources.JoinPath` creates the folder just for
being asked the path, which is why they are there at all. **The mechanism is live and the data is
absent** — and absence is the only reason this has never mattered.

★★`internal/emu/setupdb_test.go` asserts the absence. The `.gitignore` files already carry
`/.gopher2600/`, which says "do not commit this" and says nothing about what it can do. Negative
control: touching one file in any of them fails the test. Found by the mailing-list distillation
(helper-2), who closed the population of engine defaults and then followed `AttachCartridge` into
this.

**`VSYNCsyncedOnStart` hides the first frames' vertical instability, and 42% of our measurement
points are inside that window — measured 2026-09-05, and it hides almost nothing.** The engine will
not move the picture's vertical origin until the frame is Stable (`stabilityThreshold = 6`), and
`VSYNCsyncedOnStart` — **default true, never set here** — is the gate. The distillation (helper-2)
closed the population of engine defaults, found five preferences named nowhere in this repository,
and counted the exposure: **722 time references across 122 scenarios, 303 of them at frame < 6**.

Measured across 192 ROMs rendered twice at six read points:

| after | differ | | after | differ |
|---|---|---|---|---|
| 2 frames | 0 | | 5 frames | **1** — `cart_f4sc` |
| 3 frames | 0 | | 6 frames | **1** — `pf_wraps` |
| 4 frames | 0 | | 8 frames | 0 |

**At most one ROM differs at any point, which one depends on the point, and by frame 8 everything
agrees.** The effect is real, transient and self-correcting; the 42% is exposure, not damage.

★The first version of this measurement read at six frames only and said "one ROM differs:
`pf_wraps`". That was **one sample of a moving quantity**, and helper-2 caught it — six is the
threshold itself, so reading there can miss what the flag does. `internal/emu/vsyncstart_test.go`
now sweeps all six points and pins the witness list at each. ★★They also predicted `cb_roll` would
differ; it does not, at any point.

★★★Family: this is the fifth entry of *"TRUE OF HARDWARE, AND NOT OF WHAT WE MEASURE WITH"* —
after `RandomState`, the random boot bank, the VSYNC threshold and SuperChip SARA. **All of them,
and nine more, are now gathered in "The configuration surface" below**, which is where to look
first; the prose entries keep the per-trap advice, the table keeps the roster.

**The 3F hotspot is a 74LS173, and that explains two things the condition alone does not, 2026-09-05.**
The engine states the condition (`addr&0x10c0 == 0x0000`, quoting alex_79) but not the circuit. Adam
Wozniak posted the wiring in 200506 — *"`/G1`→A7, `/G1`→A6, `CLK`→enable/A12 ... When A7 and A6 are
low, followed by a high transition on A12, the 74LS173 loads its flip-flops."* Two consequences follow
that the address condition does not state on its own:

- **You cannot bankswitch from code running in RAM.** The load needs a LOW-to-HIGH transition on A12,
  and code executing out of RAM never drives A12 high. *"This also means we can't switch banks from
  code running in RAM."* — which is the constraint behind the separate `running-code-in-ram` idea of
  putting the switch routine in RAM: it works for other mappers and cannot work for this one.
- **The scheme scales by paralleling the latch**: one gives *"16 2K segments = 32K"*, two in parallel
  *"256 2K segments = 512K"*. That is where the 2003 request for *"3F images that are greater than 8K
  (up to 512 K)"* comes from — the size is not part of "3F", so a 3F ROM's size tells you nothing.

Found by the mailing-list distillation (helper-1). ★Not measured here: this repository has no 3F
fixture to run it against, so the circuit is recorded as the source's, not as ours.

**"Bus residue" names three different models, and this engine picks one, 2026-09-05.** The row above
says a read of a write-only register "returns bus residue", which is true and does not say *what*. The
engine's own source names two candidates and B. Watson's 200508 post names the third:

| model | what the floating bits become | who |
|---|---|---|
| the ADDRESS | bits 0-6 read as the address being read from | **Stella and z26, 2005** — *"in both emulators, the disconnected bits will read as the address being read from, so bits 0-6 will be `$2B`, which has bit 0 set"* |
| the LAST BUS BYTE | `data \|= mem.LastCPUData & ^mem.DataBusDriven` | **this engine, by default** — its comment says the pattern matches a PlusCart, and *"a different bit pattern can be seen on the Harmony"* |
| a fixed pseudo-random pattern | `data \|= Random.Rewindable(0xff) & ^mem.DataBusDriven` | this engine with `RandomPins` |

**Measured, not read off the source**: `litmus_floatbits` reads register `$0B` twice, once where the
last bus byte is the address (`lda $2B`) and once where it is not (`lda $00,X`, X=`$2B`), and the two
answers differ — bit 0 set then clear. Under the address model they would agree. So **this engine is
the last-bus-byte model and is NOT doing what Stella and z26 did.**

That matters because Eckhard Stolberg (200110) names three commercial ROMs whose behaviour turns on
it: Video Pinball's ball *"won't bounce off of the paddles properly"*, Dodge'em gets *"a reversed score
display"* if bit 0 comes back 1, and Berzerk enables a missile at `$F093`. **None of the three is in
this tree**, so the model is pinned here and the consequences are not.

**Does the difference reach a game? On the one vector we have, no.** Nicolas Olhaberry posted the
exact CPU state Haunted House reaches, because his own emulator disagreed there (200107/msg00044):
`A=$02`, carry set, `SBC $0F`. *"Both PCAE and Z26, after executing this opcode, leave the
accumulator with `$F3`... In my emu, since is subtracting zero, the carry remains set."* Measured
here: **`$F3`, carry clear** — PCAE's and z26's answer. `$0F` happens to be an address where the
two models agree (the operand byte and the address are the same value), so this is not a second
discriminator; it is the separate and more useful statement that **the model difference does not
reach this game.** Whether it reaches Video Pinball, Dodge'em or Berzerk is still unmeasured, and
will stay that way while their ROMs are not in this tree.

★And `RandomPins` does not mean unpredictable: it draws from `Random.Rewindable`, so it produces a
*different fixed* pattern, reproducible run to run. Measured — the default gives (1, 0) every time and
`RandomPins` gives (0, 1) every time. A test written expecting noise there would be wrong.

**`$00-$0D` is right about the TIA and wrong about the machine, 2026-09-04.** Those addresses answer
because the TIA decodes only the low bits — but the *page* they sit in is not exclusively the TIA's,
and something else in the address space can win. Darrell Spice Jr., 2003-08, tested **135 titles on a
Supercharger and found three broken** — Air-Sea Battle (*"shots either don't register"*), Canyon
Bomber and Code Breaker (*"screen rolls"*, differently) — and noted the same collision fault in
Haunted House, Space War and Ghost Manor. **Air-Sea Battle reads the collision registers at
`$00/$01`, not `$30/$31`.**

**Corrected 2026-09-04, hours after this row first landed, and the correction matters.** The proposed
fix — *"patching the code to use the images at `$3x` makes it work on the supercharger"*, with byte
offsets — is from **2000-08** (`200008/msg00038`), not 2003. The 2003 post quotes it in full **and
then reports that it does not work**: *"**I tried this, but no luck.** Running Air-Sea battle thru
Distella it looks like the `004a` should have been `024a`, but changing that didn't help either."*
This file cited the refutation as if it were the confirmation.

**Read collisions at `$30/$31` anyway** — but for reasons that survive the retraction, because the
"it fixes Air-Sea Battle" claim does not. The engine folds TIA reads to the low nibble
(`maskReadTIA = 0x000f`) and `$00-$2F` overlaps the write side; neither depends on a Supercharger
being present, and the two mirrors are identical on a bare console. **So the rule stands and the
evidence for it does not include a working repair.** What is actually established: a shipped game
reads through `$0x`, and it is one of three in 135 that a Supercharger breaks. Why remains open —
the 2000 reporter says "maybe", the 2003 reporter could not reproduce the cure, and nobody has
checked whether ROM revision, cartridge individual or modification accounts for the difference. **This is the false-green
direction** — the emulator passes it and a real configuration does not — the same side as the F8
hotspot decoding elsewhere in this file. Found by the mailing-list distillation (helper-2).

**New static trap (2026-07-30): `STA` into cartridge ROM.** The write is discarded, so a program that treats a
ROM address as storage reads back whatever was assembled there. Bank-switch hotspots and the SuperChip write
port are the legitimate exceptions, and both are deliberate — so intent is **declared, not inferred**: a line
carrying `@rom-write-ok` is allowed, anything else is an ERROR. Measured over 123 ROMs: exactly **2** stores
into cartridge space exist, both in `litmus_6502`, both aiming at ROM on purpose to time `STA abs,X` across a
page boundary (its own comment already said so). Those two lines now carry the declaration — comment-only, ROM
hash `7c16b7f…` identical before and after — and the corpus is clean. Negative control: removing one
declaration brings the ERROR back.

**Re-marked 2026-07-30: read of a write-only hotspot / SuperChip write port is not static.** What makes a
read of `$F000-$F07F` a trap is the cartridge MAPPING RAM there, and the mapper is not in the `.asm`
text — the same source line is ordinary code on a plain 4K image. Measured over the 123 ROMs the trap
linter scans: exactly **one** maps cartridge RAM (`litmus_superchip`), and the only source line reading
into that range (`litmus_6502.asm:52`, `lda $F010,x`) belongs to a plain 4K ROM where it is legitimate.
A naive static rule would score **1 false positive and 0 true positives**. The check belongs at ROM level,
where `emu.MapsCartridgeRAM` answers first; filed, not built. `internal/emu.TestCartridgeRAMIsRareAndNamed`
guards the premise — it fails if a second ROM starts mapping RAM (the trap becomes reachable and this row
needs re-deciding) or if `litmus_superchip` stops (cyclebound's SD-8c decline loses its witness).

**Checked at ROM level 2026-07-30: F8/F6/F4 random boot bank.** The console's power-on bank is undefined, so a
bank-switched cartridge must end up in the same place whichever bank it wakes in. This is decidable from the
IMAGE (unlike the write-port row above): read each bank's reset vector at `$FFFC` and look for a bank-select
hotspot in the stub. `internal/emu.TestEveryBankCanBeBootedInto` does it for every multi-bank ROM.
**The obvious rule is wrong, and measuring caught it before the check shipped.** "Every bank's stub must
select" flags `litmus_superchip`, which is correct — its bank 1 does `lda $FFF8` and jumps, and bank 0, the
destination, has no reason to select itself. `banked_game` selects in both banks, which is merely redundant.
So the rule is **at most ONE bank may omit the select**, and byte-identical banks are exempt outright.
Measured: 10 multi-bank ROMs, all pass (9 select in every bank, `litmus_superchip` in 1 of 2). Negative
controls: the too-strict rule fails `litmus_superchip` by name; blinding the hotspot scan fails all 10.

> Provenance: every row cites the mined thread(s) it came from. Raw notes in `reference/atariage/<id>-*/notes.ja.md`.

## The configuration surface — what our answer depends on besides the ROM

Every row here is TRUE OF HARDWARE AND NOT OF WHAT WE MEASURE WITH, or the reverse. They are not
bugs in Gopher2600; they are choices, and the choice is ours because nothing in this repository
sets any of them. This table exists because the family kept being discovered one member at a time
— `RandomState`, the random boot bank, the VSYNC threshold, SuperChip SARA, `VSYNCsyncedOnStart` —
each in a different section, each read as a one-off. **They are one population, and it is closed.**

Counted 2026-09-05 (`rg 'func .*SetDefaults' Gopher2600 --include='*.go' | rg -v _test`): `SetDefaults`
exists in **12** functions and all twelve have been opened. Four are hardware (`preferences.go`,
`cartridge_preferences.go`, `television.go`, `revision_preferences.go`) and hold **17** values; a
fifth file, `television/colourgen/colourgen.go`, holds **14** more that every pixel passes through
— **31** in all. The 17 are accounted for exactly: 2 + 2 + 5 + 8, and rows 1–7 and 9 below name all
of them. Of the remaining seven `SetDefaults`, **two reach outside this machine** and are rows 13–14;
the other five are the ARM coprocessor, the disassembly listing, rewind granularity, and two for the
GUI, which a headless run does not build.

**Three kinds, and they need different answers:**

| kind | what it is | what to do |
|---|---|---|
| **A — a switch that is off** | the engine models the behaviour and is not asked to | flip it and MEASURE the difference; the size of the difference is the finding |
| **B — no model at all** | the engine cannot show it however it is configured | document it; a litmus here can only measure our own settings |
| **C — something outside the repository** | a file, or a network, changes the answer | assert its absence, or read it and record what it said |

| # | Setting / mechanism | Default | Kind | Measured? |
|---|---|---|---|---|
| 1 | `RandomState` — INTIM and all 128 bytes of RAM | **false** (read in 18 places) | A | partly — the consequence is recorded (§C), the difference is not |
| 2 | F8/F6/F4 boot bank (`Random.Intn`, gated by 1) | fixed `Loader.Bank` | A | no — a litmus would measure our setting |
| 3 | `VSYNCscanlines` — the "frame is synced" threshold | **2** | A | no |
| 4 | `EmulateSARA` — SuperChip phantom reads | **false** | A | no |
| 5 | `VSYNCsyncedOnStart` — hides vertical drift before `stabilityThreshold` (**6**) frames | **true** | A | **yes, 2026-09-05** — 192 ROMs × 6 read points: at most **1** differs at any point, and by frame 8 all agree. Transient and self-correcting. 42% of our measurement points sit inside the window: exposure, not damage |
| 6 | `RandomPins` — floating-bus model | **false** | A | yes — used as a negative control in `internal/emu/floatbits_test.go` |
| 7 | The eight TIA-revision flags | **all false** | A | **yes** — 7 of 31 technique ROMs change under them; the catalogue table listed only 18 of the 31, now gated by `check_wiring.py` |
| 8 | `colourgen`: `LegacyEnabled` **true** plus 13 adjustment values (Brightness 1.196, Saturation 0.963, NTSCPhase 0.0; the non-legacy set differs on all three) | legacy generator | A | **no — and the existing guard cannot catch it**: `PaletteFor` and `HarvestPalette` BOTH go through `Spec.GetColor`, so a generator change moves both twins and `TestHarvestedPaletteEqualsDerivedPalette` stays green. So does C1==0 on the five playfield-only ROMs. **Closed 2026-09-05**: `internal/ceiling/palettegolden_test.go` pins the 128 measured RGB triples and checks BOTH paths against them instead of against each other. Negative control: moving `Saturation` from 0.963 to 0.500 shifts `$46` from `[236 51 51]` to `[157 69 69]` and fails the golden while the twin test still reports `ok` (probe applied, measured, reverted) |
| 9 | `VSYNCimmedateSync` / `HaltChangedVBLANK` / `HaltChangedVSYNC` / `UnwrapACE` | false/false/false/true | A | **no — not named anywhere in this repository before this table** |
| 10 | **PAL colour-loss** (odd scanline count → the set goes black and white) | — | **B** | Gopher2600 has **no implementation at all** — not a preference that is off, absent. Stella has the setting; it was off by default in 2011 and its maintainer said a developer should always have it on. Our own trap row for this warns the author about something our instrument cannot show 〔atariage 190917〕 |
| 11 | The composite waveform — sync separation, DC reference. **A missing layer, not a setting** | — | **B** | our television takes a digital sync flag, so no VBLANK-interferes-with-sync effect can appear here 〔stella-list "262 scanline kernel is rolling"; atariage 113732, 126450, 254640, 189414〕 |
| 12 | `setupDB` + `patches/` — a SHA-1-keyed database that can toggle the panel, PATCH THE CARTRIDGE BYTES, or change the TV spec | absent here | **C** | `setup.AttachCartridge` consults it on every load and the engine's own comment says it will "silently ignore absence of setup database". In a dev build the path is **relative**, so which database is consulted depends on the directory the command ran from. Measured 2026-09-05: **38** `.gopher2600` directories exist under the umbrella, **all empty** — 32 of them in Go package directories, because `go test` runs each package with the package directory as its working directory and `resources.JoinPath` mints the folder just for being asked the path. Asserted by `internal/emu/setupdb_test.go`, which **walks** rather than listing: an earlier version listed four roots and reported "four" |
| 13 | **PlusROM outbound HTTP** | no off switch | **C** | `plusrom/network.go` POSTs to a URL carried in the ROM itself (`http.NewRequest("POST", …)`), and no preference disables it — the gate is a **fingerprint**. `cartridge/fingerprint.go`'s `fingerprintPlusROM` searches the WHOLE cartridge image for the bytes of `STA $1FF1` (`8d f1 1f`); the comment says an earlier version searched only the first 1024 bytes and missed a real PlusROM after it. A false positive is caught one step later — `NewPlusROM()` validates the URL — so the byte match alone does not transmit. Measured 2026-09-05: **0 of 1157 `.bin` files** under the umbrella carry the sequence, so the second check is never even reached. Re-run the three-byte scan if the corpus grows |
| 14 | `FestivalEnabled` (AtariVox speech) — can launch an **external binary** | **true** | **C** | **not measured.** Same family as 13: an engine default that reaches outside the process. Whether any path in our headless use ever reaches it is unknown |

**The general rule.** An emulator implements the specification; what is not in the specification
cannot be modelled, only lost. **Kind A is not that** — kind A is in the specification and switched
off, so it is measurable today and the measurement is cheap. Do not write "hardware does X" from a
run that never asked the engine to model X.

**Provenance.** Rows 1–4 and 10–12 were found by the mailing-list distillation (helper-2), who
closed the population of `SetDefaults` and then followed `AttachCartridge` and `fingerprint.go` out
of it; rows 5, 7 and the counts in 12 and 13 were measured here. Rows 13 and 14 are the two that
leave the machine, and only 13 has been counted.

## A. Timing / sync (the Pong killers)

| trap | what happens | fix | detect | source |
|---|---|---|---|---|
| **the TIA manual's "one line after" for VDELx** | It reads as a mechanism and is a consequence. The manual says the second graphics bit is loaded *"one line after the first was loaded from the data bus"*; the registers actually **watch for writes to GRPx and never look at the scanline counter**. Under the era's idiom — write GRP0 and GRP1 every other line, as Combat does — the two are the same thing, which is why the wording survived. Do anything else and the delay tracks your writes, not your lines. | Think in writes, not lines: a VDELx object changes when GRP1 is next written. Ours agrees with the writes reading: `ball.go`'s `setEnableDelay` is called from exactly two places, both `case cpubus.GRP1`, and nothing in that file reads a scanline counter (its two mentions of the word are drawing comments). | No fixture yet — write GRPx at an irregular cadence and show the delay follows the writes. Source semantics are not a measurement. | 〔stella `200506/msg00063`, `msg00070`〕; engine read 2026-09-03 |
| **a subroutine called mid-line** | Whichever branch it takes decides whether it spills into the next line, so the frame grows by the case rather than by the code. Only two things make a path change a frame's length: **crossing a line boundary** or **exceeding 76 cycles** — path length alone does not, and `litmus_pf_async` proves it, holding 262 with bands 7 cycles apart (`cyc 40` vs `cyc 33`). | Give the call a line of its own: `sta WSYNC` before it as well as after. **Bracketing is not always enough** — `litmus_flicker_attrib` went 260/261 mixed, then 263 in three groups and 264 in the fourth *with* brackets, because one routine's group-2 path was longer and still spilled. A routine whose length varies by case needs its own line, so it was split in two. | `frame_lines_stable` (∀ over frames). **`ntsc_frame_lines` samples ONE frame and passed both broken versions.** | measured 2026-09-03 |
| **RIOT timer wraparound write** | writing `TIM64T`/`TIM1024T` on the exact wraparound cycle silently drops the divider to **1T** → frame length collapses → rolls on real HW, Stella passes | **double-write** the timer; or never write on the wrap cycle | runtime (`breakif` timer-write@wrap) | 303277, 316035, 327383, 202736, 323670 |
| **HMOVE then HMxx within 24 cy** | writing any `HMxx` within 24 CPU cycles after an `HMOVE` strobe → unpredictable motion on HW | keep ≥24 cy between HMOVE and the next HMxx write | runtime (trace_clocks) | 307641 |
| **mid-line HMOVE / cycle-73-74 strobe** | the "no black comb bar" trick (HMOVE at cy73/74) only moves **left**, and `WSYNC`-then-`HMOVE` becomes unusable; behaviour differs by TIA revision | use only if you accept left-only; pin TIA revision when verifying | runtime + manual | 183219, 210361, 162520, 133205, 280660, 319456 |
| **total scanlines not constant** | frame line count varies between frames → jitter / roll even if each frame is "near 262" | keep total scanlines **identical every frame** (variable work absorbed in a fixed budget) | runtime (`ntsc_frame_lines`) | 171270, 303750, 306318, 76728 |
| **GRP/COLUP written outside HBLANK — deadline = the object's leftmost pixel clock, not just "HBLANK end"** | `GRP*` must finish **before the sprite's first visible pixel**: a sprite parked at clock 11 (near the left wall) is corrupted by a `STA GRP0` that lands at clock 13 → its left 2 px show the *previous* line's GRP. Any **per-line prefix before the `STA GRP` eats that margin** — a parity branch (`TXA;AND#1;BNE`, +6cy≈+18px), a PF-store-before-sprite on a band's first line, or a slower address mode — pushes the write past the object | keep the low-X sprite's `STA GRP0` as the **first thing after WSYNC**; on a band's first line write GRP *before* the PF stores; give the earliest-clock object the least-delayed slot | runtime/manual (`read_row` the drawn line's left edge) | 166749, 63229, 74225 + in-house: Combat band-boundary GRP0 & parity-prefix left-edge corruption 2026-07-18/19 |
| **PF band's first row partial / the "eaten line"** | a full-width `PFx` band whose writes don't finish in its first row's HBLANK (e.g. PF set on the *previous* line's trailing, no leading `WSYNC`) renders that row partly black — the left edge is unfilled because PF fills as the beam crosses (same root as GRP-outside-HBLANK). And if an object's last row shares that line, the PF write eats it → you get the object's last row **xor** a full-width PF row, not both | set the band's PF early in the **HBLANK of its first line** (after that line's `WSYNC`); to keep BOTH an object's last row AND a full-width PF row, give the band **1 extra line** | manual / `read_row` the band's first row | in-house: PONG static-repro 2026-06-19 |
| **RESx on a drawing line** | `RESxx` resets immediately; striking it on a visible/draw line glitches | reposition on a dedicated line, not while drawing | manual | 67525, 74225 |
| **HMOVE comb on a visible line** | every `HMOVE` right after WSYNC blanks color clocks 0-7 of THAT line (the comb) — if a drawn band's first row shares the HMOVE line, its left 8px go black (e.g. a top-wall row-1 notch). **★It is an OVERSIGHT, not a specification, and that is why the cy73/74 workaround above is revision-dependent.** Glenn Saunders in 1998: *"The reason the 2600 positions objects as it does is because **poly-counter logic is cheaper to implement than digital logic**. Believe me, they did weigh the alternatives … they developed a software conversion routine and were anticipating that this routine, albeit CPU-hungry, **would only be necessary once at the top of the frame for each sprite (hence not knowing about the HMOVE comb bug)**"*. A second voice in the same thread: *"Refreshing to see someone refer to the HMOVE effect as a **bug** instead of a **feature**. My guess is that it was Activision who turned it into a feature. Too bad really, **it might have gotten fixed**."* So the comb lives in behaviour nobody specified, which is the reason our own measurement finds it moving under a TIA-revision flag (`LostMOTCK` changes `litmus_hmove_mid` and `litmus_hmove_side`, and nothing else in 145 litmus ROMs) rather than an unexplained coincidence. | land every HMOVE on a **black/blank line**; give the band's first row a line with no HMOVE (PF set in its own HBLANK) | `read_row` the band's first row = full 160 | in-house: PONG pf2 top-wall notch 2026-07-01; the design history from stella-list `199804/msg00146` (Glenn Saunders, 1998-04-17) and `199804/msg00151` (John Saeger), recovered verbatim from the raw archive by the mailing-list distillation (helper-2) |
| **N-edge coincidence = the real worst path** | a per-line kernel with several `cpy <edge>` branches (ball top/bottom, paddle tops/bottoms…) gets budgeted on "typical" frames, but the true worst case is **all edges landing on the SAME Y** (+~5cy per extra hit) — a 1cy overrun that free-run testing may miss for hundreds of frames, then rolls once | hand-place the coincidence: poke every edge variable to one Y and `assert_line_budget`; add it as a standing fuzz case | poked-coincidence assert; `beamtrace` shows the stacked writes on one line | in-house: PONG PlayF 77cy (3 edges + INPT poll) 2026-07-02 |
| **velocity-negate bounce without position clamp = sign-flip trap** | upgrading fixed-value reflection (`DY:=-1`) to angle-preserving `DY:=-DY` silently drops the *idempotence* the old code relied on: if the object can sit at/past the wall row for 2+ frames (re-hit, \|DY\|≥2 overshoot), the sign flips every frame → the object oscillates in the wall zone forever | clamp the POSITION back to the last legal row on every wall reflection (top→0, bottom→max−h) | free-run + watch the position variable near walls | in-house: PONG english 182↔184 trap 2026-07-02 |
| **`bpl`/`bmi` clamp on a coordinate that legitimately exceeds 127** | using bit7 (`bpl`→"non-negative") to detect underflow on a value whose valid range is 0..N with N>127 (e.g. a playfield Y of 0..182): every legit value ≥128 has bit7 set and is misread as "negative" → silently clamped to 0. A target of Y=150 becomes 0 = paddle snaps to top | detect underflow by the **subtraction borrow / wrap magnitude**, not bit7: after the signed add, `cmp #<wrap-threshold>` (e.g. `#200`) to separate a wrapped-underflow (≈247-255) from a legit high value (≤~180). **The threshold is range-dependent** — set it above the max legit value: a lead/extrapolation like `BallRow + 8·DY` reaches ~202, so it needs `cmp #220` (not `#200`, which would misread 202 as underflow) | poke the coord to a >127 value and read the derived target | in-house: PONG AI target Y clamp 2026-07-03 (threshold refinement: v3 lead AI 2026-07-06) |
| **`cmp` clobbers Z between a load and a test-for-zero `bne`/`beq`** | to convert "0 → 1" you `lda v / … / bne keep / lda #1`, but if a `cmp #k` sits between the `lda v` and the `bne`, the branch tests `v==k`, **not** `v==0` (cmp overwrote Z). The clamp `lda v / cmp #hi / bcc / … / bne set` silently mis-handles `v==0` because the `bne` reads the cmp's result. Sibling of the immediate-`ld` clobber below; here it's `cmp` | do the **zero test before any `cmp`** that reuses the register (`lda v / beq zero / cmp #hi / …`), mirroring the working pattern in the serve-angle clamp | poke the input to 0 and read the output; it comes out 0 instead of the intended 1 | in-house: PONG serve-angle continuation clamp 2026-07-04 |
| **immediate `ldx`/`lda`/`ldy` clobbers N/Z between a flag-set and its branch** | a load-immediate sets N/Z from the loaded constant. Placing `ldx #80` after `lda dir` (to hold a value for the taken branch) destroys the sign flag the following `bpl`/`bmi` was meant to test → the branch reads the constant's sign, not `dir`'s. `sta` preserves flags (safe), but any `ld_` does not | branch **before** any `ld_`, or avoid the branch entirely with arithmetic (`dir` ∈ {\$01,\$FF} → `X = dir + 79` gives 80/78 branchlessly, no flag dependence) | step through and read the branch's actual target | in-house: PONG serve-side select 2026-07-03 |
| **coarse-positioning region overruns the scanline at large X** | the `sta WSYNC`→`sta WSYNC` region wrapping the ÷15 divide loop + `eor`/ASL fine tail grows with the target X; at max X its **worst case can exceed 76cy** → RESP0 strobes on the *next* line → **roll**, while a lucky small-X playtest passes. The positioning line is the kernel's **tightest** region | prove over ∀ (`prove_line_budget`) with `; @amax N` on the **region-opening `sta WSYNC`** (not the `sbc` line); then **cap sprite travel** or replace the loop with a **÷15 lookup table** (constant-time). Bounding the loop alone isn't enough — the fine tail also counts toward the 76 | static (`prove_line_budget` `roll_free`) | in-house: density-ladder rung2 2026-07-24 |
| **multiple `HMOVE` strobes in one frame (multi-object positioning)** | positioning each object with its own `sta HMOVE` (e.g. P0-block → P1-block → M0-block, each ending in HMOVE) **re-applies every current `HMxx` to ALL objects on every strobe** → the object positioned first gets its fine motion applied N times (looks like slope-Nx: +1 X-input moved the sprite ~3px) and a stale `HMxx` left in another object's register bleeds onto it → the sprite **never settles**: it drifts a few px every frame even with **zero input**, and wobbles / "loops left-right within a range while moving." Passes a single-frame screenshot and stays `roll_free`, so it hides until you watch it MOVE. RESxx re-strobing every frame does NOT save you — the extra HMOVEs act after the strobe | strobe all coarse positions (`RESxx`) first, set all `HMxx`, then **one** `sta HMOVE` for the whole frame — deferring HMOVE across the intervening `sta WSYNC` cut lines is fine (HMxx persist). Alternatively `HMCLR` between per-object strobes. Watch the budget: removing the per-object `sta WSYNC` too absorbs the next block's prologue into the div region (→ over-budget) — keep the WSYNC cuts, drop only the HMOVEs. **★A third fix, for a kernel where you cannot remove an instruction at all: REPLACE the strobe rather than drop it.** In a generated, fully-timed kernel every write below is timed from the top of the line, so deleting three cycles moves everything after it. `bit $80` costs the same three cycles, touches RAM instead of the TIA, and is what this project's own first work uses — `roms/260816_transistor/tools/mk_transistor_ms.py:1023-1027` rewrites a repeat pass's `sta HMOVE` into ``bit $80   ; was HMOVE -- the repeat must not nudge again``, having measured the trap first (*"pass 1 drew at x=19 and its repeat at x=20, every letter on the line one pixel right of the first"*). **The catch, and it is not in that comment: `BIT` is not free.** It sets N and V from the memory byte and Z from `A AND m` (measured, `internal/emu/bitflags_test.go`), so the substitution is only safe where the flags are dead. If they are not, no legal three-cycle filler exists that touches nothing — `PHP`/`PLP` is seven cycles in two bytes (measured, `internal/emu/oddsleep_test.go`) | runtime (`read_row` a **constant-input** frame N times in a row → the object's left edge must be byte-identical; any per-frame drift = this bug). **The static half is `cmd/timinglint`'s `hmxx-without-hmove`**, and pointing it at the works for the first time on 2026-09-05 found the other end of this same fix: two generated bodies in which EVERY strobe had been quieted, leaving a `lda #$F0` staging block with no consumer at all (`sta HMOVE` = 0, `bit $80` = 11 and 10). Dead rather than wrong, but the comment above it claims a purpose the ROM does not have. **9 flagged ROMs, 0 false positives** — the rule is a whole-file predicate (`internal/cyclebound/timinglint.go:224`, `haveHmxxNonzero && !sawHMOVE`), so a ROM that strobes anywhere is never flagged | in-house: Outlaw gunman horizontal wobble 2026-07-25; the replace-don't-drop fix and its measurement from `roms/260816_transistor/tools/mk_transistor_ms.py`, found by the mailing-list distillation (helper-2), verified here 2026-09-05 |

| **`VBLANK` written on the LAST picture line blanks that line** | a picture loop that exits with the CPU at the START of the final line, then writes `VBLANK` there, blanks the line it is standing on. It is **invisible on a dark background** and shows as a black line across the bottom of a light one — so it survives every build until somebody inverts the palette. | `sta WSYNC` before the `VBLANK` write so it lands in the overscan; the overscan loop gives that line back, so the frame is still 262 | build the same kernel with foreground and background swapped and compare the drawn row count; `read_row` the last picture line | measured 2026-08-26 in a piece in the private `roms/` repository (its ledger names this file) |

## B. Positioning ground-truth (why "the X doesn't match")

| fact | value | source |
|---|---|---|
| **a two-pixel stroke needs ≥5 source columns** | stem 2 + gap 1 + stem 2. Doubled, that row is **10 px** wide (12 px from a 6-column source), while the ball and the missile can only be **1/2/4/8 px**. So **no solid-block object can stand in for the full-width row of a two-stem letter at any scale**, and "just narrow the bars" cannot rescue a letterform that needs two stems on one row — it deletes the gap instead. | measured 2026-08-26 in a piece in the private `roms/` repository |
| **internal draw delay after RESxx** | player draws **+5 CLK** late, missile/ball **+4 CLK** late (RESx resets instantly but the object appears later) — *first suspect when target X is off by ~5px* | 294398, 283075, 305780, 75335-cluster |
| RESx strobe granularity | 3 color-clocks; RESP0 finishing at cy46 → X≈75 | 172089, 137739, 329611, 304182 |
| HMOVE range | ±8 px / scanline; pulse count = upper nibble of `HMxx EOR $80` | 319456 |
| coarse÷15 + fine HMOVE | `eor #7` → 4×ASL, ~30 cy; small real-HW latch differences | 304182, 284554, 160645 |
| ÷15 / X(N) is kernel-specific | the absolute offset includes the prologue cycle count → **measure `read_tia` HmovedPixel, don't hardcode N** | (CLAUDE.md) + 294398 |
| **positioning ÷15 loop crossing a page → judder** | if the `sbc #15` / `bcs` divide loop straddles a page boundary, the **taken `bcs` costs +1 cycle** → each iteration is 6cy(18px) not 5cy(15px); the coarse step no longer tiles with the HMOVE fine (±~7) → the object **judders ~3px at every 15px while moving** AND the X(N) offset inflates by +1cy/iteration. A *static* object hides it (only shows when it MOVES across cells). **Keep the divide loop on one page** (page-align it). Found: PONG smooth-ball, the loop sat at $F0FE/$F100 — moving it to $F100 (whole loop on one page) made the ball perfectly smooth (read_motion jerk→0). detect: runtime (read_motion jerk_rms) / listing (loop address vs page boundary) | in-house: PONG 2026-06-19 |
| **÷15 DELAY 1→2 boundary is HBLANK-compressed → a moving sprite hitches ~2px ONCE near the left wall** | distinct from the page-cross judder: even with the loop on one page, the FIRST coarse cell is nonlinear because a low-iteration strobe lands deep in HBLANK. `litmus-results.md:15-17,31`: coarse step is **+9px for DELAY 1→2** vs a clean **+15px for DELAY≥2** (linear `ResetPixel=15·DELAY−18` only from DELAY 3). So a sprite moving across the **DELAY 1↔2** boundary (target X ≈ clk 12-16, i.e. the leftmost ~15px) steps ~2px BACKWARD once, while every higher ÷15 boundary (DELAY≥2) is smooth. It's HW nonlinearity, not a code bug; a *static* sprite there is fine — only a MOVING one shows it. **Fix for a moving sprite: clamp the ÷15 input so it never uses DELAY 1** (keep coarse ≥2 iterations ⇒ leftmost target ≳ clk14). The sprite loses ~4px of left reach but moves perfectly smoothly (`read_motion` x monotonic, no backward step). Verified: Outlaw left gunman — floored P0X≥15, seam gone | measured (`litmus-results.md:15-17,31`) + in-house: Outlaw gunman left-edge seam 2026-07-25 |

## C. Cartridge / bank / RAM

| trap | fix | detect | source |
|---|---|---|---|
| **`RandomState` governs far more than the boot bank — INTIM and all of RAM, and it is off** | The row below records one consequence of `RandomState` being false. Counted 2026-09-04: the preference is read in **18 places**, and the two that matter most are not cartridge banks. `riot/timer/timer.go:101` randomises the **divider and INTIM** at reset (`ChipWrite(INTIM, Random.Intn(0xff))`); `memory/vcs/ram.go:51` fills **all 128 bytes of RAM** with random values instead of zero. Nothing in `internal`, `cmd` or `pkg` sets it (`rg RandomState` → 0). **A bank-switching quirk touches bank-switching ROMs; a deterministic INTIM and a zeroed RAM touch every ROM there is.** The corpus names the damage: Thomas Jentzsch, 2003 — **Berzerk seeds its maze from INTIM**, so a console that always reads the same value draws the same maze every time. | Treat "it behaves the same on every run" as a property of this harness, not of the console, whenever a ROM reads RAM or INTIM before writing it. `defuse`'s `ReadBeforeWrite` finds the RAM half statically; the INTIM half has no detector. For a ROM of our own the rule is unchanged — initialise everything — but for **studying a commercial ROM**, determinism here can hide the very behaviour being studied. | `rg 'RandomState' Gopher2600 --include='*.go'` (18 hits) and `rg 'RandomState' internal cmd pkg` (0). | `Gopher2600/hardware/riot/timer/timer.go`, `hardware/memory/vcs/ram.go`; stella-list 2003 (Berzerk/INTIM); found by the mailing-list distillation (helper-1) |
| **F8/F6/F4 boot in a random bank — TRUE OF HARDWARE, AND NOT OF WHAT WE MEASURE WITH.** `mapper_atari.go`'s `reset()` picks `Random.Intn(len(cart.banks))` **only when the `RandomState` preference is set**, and `preferences.go` `SetDefaults` sets it to **false**, with nothing in this repository changing it — so our boot bank is `SetBank(Loader.Bank)`, deterministic. A litmus for "the boot bank is random" would show it fixed, and the fixed result would be about our settings. The list draws the same distinction first-hand: *"what if the cart startup is **random (I think so)**"* about hardware, and in the same message *"from testing my games, I think Stella always starts in the last bank of an image. Not randomly"* about the emulator 〔stella `199801/msg00368`, Greg Troutman〕. Third of these found in two days, after SuperChip SARA and the VSYNC threshold. The advice below is unchanged — it is what hardware does that matters. |  every bank's reset entry must `JMP` to a common init in bank 0 | static (check each bank start) | 194935, 293970, 261488 |
| **`NOP`/`BIT` reading `$00`-`$3F`** as a skip on 3F/X07 carts | triggers an unintended bankswitch. The condition is **A6 and A7 both low** (`addr&0x10c0 == 0`), which in zero page is all sixty-four of `$00`-`$3F` — `nop $04` and `bit $2C` are as dangerous as `nop $00`. `$40`-`$7F` sets A6, `$80`-`$BF` sets A7, so **`NOP $80` is safe for a reason**, not by convention. ★Also emitted by **`SLEEP` with an ODD value** (`nop $00`, or `bit $00` under `NO_ILLEGAL_OPCODES` — the switch changes the opcode and keeps the address), and by a **raw-byte skip** (`.byte $2C`), neither of which a mnemonic grep sees. For an odd delay use `PHP`/`PLP`: 7 cycles, 2 bytes, all flags restored, no address touched. | static: `nop`/`bit` with an operand in `$00`-`$3F`, `SLEEP <odd>`, and `.byte` NOP/BIT forms (the last scoped to files targeting a TIA-decoding mapper) | 139089; the address range and the `SLEEP` bytes measured 2026-09-05, `internal/emu/oddsleep_test.go` |
| **read of a write-only hotspot / SuperChip write-port** | undefined on HW (varies by console) → never read it | **runtime/ROM-level** (was static/manual — see below) | 169114, 285759, 204819, 111536 |
| **STA to ROM** | no R/W line to the cart → bus contention, never persists; RAM is `$80–$FF` only. **This one line is the CAUSE of three other entries in this repository, 2026-09-04.** The cartridge connector carries address and data and *no direction signal*, so a cartridge that wants writable memory must encode direction in the address itself. Everything downstream follows: **(a)** a SuperChip's 128 bytes occupy **256 addresses** — the first 128 write, the second 128 read — which is why its write port is not statically decidable and why a naive dumper reads back whatever it just wrote (`Gopher2600/hardware/memory/cartridge/fingerprint.go:584`: *"because there is no r/w line in the cartridge bus, there 256 addresses related to the superchip"*); **(b)** the Supercharger needs a stateful arming sequence instead — touch `$F0xx` and the next `$Fxxx` access becomes a write regardless of the instruction, with the engine's `Delay = 6`; **(c)** every "is this address a read or a write" question in the mappers is answered by convention rather than by a signal. Erik Mooney and Eckhard Stolberg, 1998, stated the fork plainly: with no R/W line you either **spend an address line as the direction bit** (halving the RAM) or **do something more complicated, like the Supercharger**. | static | 148390, 62852, 293816; cause traced by the mailing-list distillation (helper-1) |
| bank-move misaligns code → page-cross | split heavy moves across frames / `ALIGN 256` | static (ALIGN) | 307854, 339019, 133720 |
| variable placed at `$FF` | `JSR` push lands on `$0100` mirror and clobbers it → keep vars in `seg.u` from `$80` | static (var ≥ $FB warn) | 302998, 301766, 290790 |

## D. CPU / 6502

| trap | fix | detect | source |
|---|---|---|---|
| **`rg -rn` is not "recursive + line numbers"** | ripgrep's `-r` is `--replace` and **takes an argument**, so `-rn` means *replace every match with the letter `n`*. The output looks like a normal search result with the searched-for words missing and no line numbers, and the exit status is 0. ripgrep recurses by default, so `-r` was never needed. It silently corrupted two searches here on 2026-09-04, one of which had already been written into a technique page as a claim about how many places call `CPU.Interrupt()`. | Use `rg -n`. If a result reads oddly — a quoted line missing the very phrase you searched for — check the flags before believing the content. | Re-run with `rg -n` and compare. A `--replace` run and a plain run differ visibly on any file that matches. |
| **two assembler traps from the list that our toolchain does NOT have — checked 2026-09-04** | 1997: an assembler accepted `jmp (abs,x)`, a 65C02 addressing mode the 6507 does not have, and the author lost time to a program that assembled and misbehaved. 2003: DASM silently promoted `LDX zp,Y` to `abs,Y` (+1 cycle, no error) while *rejecting* `LAX zp,Y`, so a documented and an undocumented instruction with the same addressing mode were treated differently; half a day lost. | Nothing to do. **DASM 2.20.14.1 here rejects `jmp ($F800,x)` outright** — *"error: Illegal Addressing mode"*, fatal — and assembles `lax tbl,y` to `b7 80`, two bytes, no promotion and no error, matching `ldx tbl,y` at `b6 80`. | Re-run the two one-line fixtures if the toolchain version changes; both traps are version-specific. |
| missing `CLD` | BCD math gives garbage; D is undefined at power-up | static (`CLD` in init) | 318346, (CLAUDE.md) |
| `ADC` without `CLC` / `SBC` without `SEC` | carry contaminates the result | manual | 157598, 75335, 63853, 63389 |
| **Depending on a power-up value, not just failing to initialise one** | The rows below say *initialise everything*, and `design-principles.md` gives the eight-byte sequence that does it. Neither says **do not read a power-up value on purpose** — and someone did. stella-list `200111/msg00309`, Chad Schell talking a homebrew author out of it: *"I think **relying on the state the carry bit comes up in** is a little risky myself. You don't know how much hardware will have a problem running it, **both now and in the future**."* The author's aim was to make the game refuse to run on a particular cartridge — deliberate use of an undefined value as a copy-protection device — and the argument that stopped him was not correctness but the cost of maintaining it. **Worst possible direction**: green in every emulator, broken on somebody else's console, and the author never finds out. | Treat every register, flag and RAM byte as garbage until this program wrote it. If you want a machine to behave differently on different hardware, choose a value you can *read* (`SWCHB` switches, a cartridge signature) rather than one that is merely *undefined*. | Statically decidable from `internal/cyclebound`'s CFG — `defuse`'s `ReadBeforeWrite` already reports RAM reads no path wrote; the flag equivalent is not implemented. | stella-list `200111/msg00309`; found by the mailing-list distillation (helper-2) |
| **"Frying" — players deliberately corrupt the power-up state, and it works** | The row above says do not *depend* on a power-up value. This is the same fact from the other side, and it is why that row is not theoretical. From the list, 1998: *"**'frying'… means turning your 2600 on in such a way that it doesn't boot up quite right**"* — used to reach the *"fast walkers"* in The Empire Strikes Back — *"**this is probably a method reserved for the OLD STYLE Atari 2600s, since the newer versions had slightly different switches**."* Players were corrupting RAM on purpose, getting reproducible-enough results to trade as technique, and finding that it depended on which console you owned. A 2005 thread describes the mechanism precisely enough to emulate: bits are ANDed away at pseudo-random addresses and **never set**, so power-up corruption only *clears*. | Two consequences. **For a ROM**: an uninitialised value is not merely undefined, it is something a player can reach into and change — `CLEAN_START` is protection against a person, not only against chance. **For a fixture**: a "fried" state is a legitimate thing to test against, and `save_state`/`restore_state` make one reproducible (B. Watson, 2005: *"once it produces a 'fried' effect, we can save the state of the emulator … loading that state file will produce the same fried state"*). | `rg -i '\bfry(ing)?\b'` over docs/internal/cmd/roms/scripts returns nothing; `internal/mutate` is ROM-byte fault injection, a different thing from corrupting live RAM and TIA at run time. | stella-list, 1998 (player technique) and 2005 (mechanism); found by the mailing-list distillation (helper-1 and helper-2 independently) |
| **post-reset SP / RAM / flags undefined** | uninitialised everything → `CLEAN_START` is mandatory (proven on 5 consoles) | static (CLEAN_START) | 261488, 312005, 316071 |
| page-cross `+1` cycle on reads | reads (not stores) take +1 across a page; throws off cycle budget | static (CHECKPAGE / ALIGN) | 132913, 125755, 147642 |
| unstable illegal opcodes | `LAX/SAX/SBX/DCP` are HW-stable **on original NMOS silicon, which is not the same as "on every machine that plays 2600 cartridges"** — ★`SBX` and `ARR` are reported as not working on the **Flashback 2**, a chip-level reimplementation rather than a 6507 (AtariAge `113732-clean-assembly`, found by the mailing-list distillation 2026-09-05). This row already scopes `ASR` to *late Taiwanese Atari Jr units*; the same scoping applies here and was missing. ★★Unmeasured by us: we have no Flashback 2, and the engine models a 6507, so this is recorded as the source's claim.; **`LXA/XAA` are not** (chip/temp dependent); **`ASR`/`ALR` is NOT stable** — fails on late Taiwanese Atari Jr units (official hardware), Thunderground score corruption, confirmed independently by omegamatrix on real hardware | static (opcode allowlist) | 168616 (both the stable list AND the ASR caveat), 294471 §32, 132496, 139505 |
| **signed subtraction of two positions that can be >127 apart** | `dx = A − B` as a signed byte **overflows** when \|A−B\|>127 (two sprites on a 160-px field): −132 wraps to +124, mis-read as *positive* → a sign/direction test aims the wrong way (and a ±1-frame drift as the objects close within the signed range). The `V` flag catches it but is rarely tested. Sibling of the bit7-clamp traps in §A, but here it's the **difference of two wide-range values**, not one | take the **sign from an unsigned `CMP`** (is A≥B?) and the **magnitude from larger−minus−smaller** (always ≤255); never trust bit7 of a raw signed difference | poke the two coords >127 apart, read the derived sign/direction | in-house: Combat enemy-AI aim, `dx=TankX0−TankX1` 2026-07-19 |

| **Opposing joystick directions cannot be tested here at all** | Real `SWCHA` is four independent switch lines, and a **dance pad, a homebrew controller or a worn stick can close up and down together** — nothing in the RIOT prevents it. Gopher2600's peripherals prevent it, deliberately, and **it is not specific to the stick**: `gamepad.go` carries the same four `cancel()` calls with the same comment, and `keypad.go`/`paddle.go` have no directions to cancel — so **every direction-carrying peripheral the engine has** enforces the exclusivity. `Gopher2600/hardware/peripherals/controllers/stick.go` — *"we don't want to allow impossible positions for the stick. for example, holding left and right at the same time is impossible. the cancel function cancels any existing position that is in opposition to the new axis"*. Measured 2026-09-04: `SetInput(up)` then `SetInput(down)` reads `$DF` (down only, up cancelled); all four pressed reads `$5F` (down+right). **`Poke($0280, …)` does not get around it** — the peripheral rewrites SWCHA on the next update, so a poked `$CF` reads back as `$FF`. There is no `UpDown` axis constant, so `DataStickSet` cannot express it either. | Do not test opposing-direction handling through `SetInput`; it silently gives you a *different* input than the one you asked for, and the ROM sees a legal single direction. Write the branch defensively (treat `up&&down` as a real case) and note in the scenario that it is unverified here. **This is the dangerous direction of divergence**: the tool is *narrower* than the hardware, so a ROM that mishandles the state passes and fails on someone's pad. | Press two opposing directions and read `$0280`: if the result has only one direction bit clear, the cancel ran. Found by the mailing-list distillation (helper-1) from `discotech` (2003), a homemade dance pad. | `Gopher2600/hardware/peripherals/controllers/stick.go` (the engine's own comment); stella-list `discotech` thread |

| **The 400 µs port-direction settling time is not modelled** | The Programmer's Guide requires **400 µs between writing SWACNT/SWBCNT and reading the port** — 477 cycles at 1.19318 MHz, **6.3 scanlines**, which is a whole kernel's worth of budget. Gopher2600 does not implement it and says so in `hardware/peripherals/controllers/keypad.go`: *"We're not emulating this here … I'm not sure what's supposed to happen if the 400ms is not adhered to. **!!TODO: Consider adding 400ms delay for SWACNT settings to take effect.**"* Measured 2026-09-04 (`litmus_swacnt` band 5): reading 480 cycles after the write gives the same byte as reading 4 cycles after it. **This is the opposite direction from the joystick trap above** — there the tool is *narrower* than the hardware, here it is *looser*: a ROM that skips the settling time passes and may not on a console. | If you drive a port, wait 400 µs before reading it, and put the wait in the kernel's budget — it is 6.3 lines, not a rounding error. **The figure is a rule of thumb, not a specification.** The oldest post on it (`200011`, a year before the two below) does the arithmetic — *"1194720 cycles per second times 0.0004 seconds = 477.888 cycles = 6.288 scanlines?"* — and immediately hedges it: *"**I assume 400 microseconds is an upper bound as a general rule of thumb**."* Nobody in the thread had measured it; the question it opens with is *"has anyone written a small program to **verify this**?"* — which is the question `litmus_swacnt` answers 26 years later, and answers only for this engine. Budget the full 6.3 lines until someone measures the real settling time on hardware. **The list answers the engine's TODO**: Chad Schell, running serial off the port at 38.4 kbps (`200111/msg00194`), *"If you only read the port, and thus don't change it's configuration, the 400 uS delay does not apply"* — so the constraint binds **when you change the direction**, not on every read. | Grep the ROM for a `SWACNT`/`SWBCNT` write followed by a port read within ~477 cycles. Nothing here detects it automatically; both ROMs in this repository that write a DDR are litmus fixtures that violate it deliberately. | Stella Programmer's Guide, quoted in the engine's own source; **stella-list `200011` (`more-keyboard-nonsense`) is the earliest — it states the arithmetic and calls 400 µs an upper bound**, and separately settles a wrong theory in passing: the delay is *not* switch debounce, because *"what if the user presses the button a short while **after** you do the output?"*, and *"the better way to debounce is simply to read slowly … which is a different thing than waiting between the reads and writes"* — the same separation Chad Schell arrives at a year later; then `200111/msg00192` and `200111/msg00194`. Found by the mailing-list distillation (helper-1 for the 2001 posts, helper-2 for the 2000 one) |

| **`BPL`/`BMI` after `CMP` is a SIGNED test on an unsigned compare** | `CMP` sets **carry** for the unsigned comparison; `BPL`/`BMI` read **N**, bit 7 of the difference, which flips across `$80`. `LDA #$7F / CMP #$80 / BPL bigger` takes the branch although $7F < $80. **2600 kernels compare scanline counts and screen positions**, which cross $80 routinely, so this is not a corner case here. Caught once on the list by reading, not by running — Greg Miller on someone else's code 〔`199710/msg00043`, Greg Miller, 1997-10-03 — **recovered 2026-09-06 after this row spent a day saying the citation could not be found**〕: *"is there a good reason why you're not prepping the carry bit before the SBC? … **You sure this is working with `bpl`?**"*, answered *"Ummm.... because I forgot."* | Use **`BCC`/`BCS`** for unsigned comparisons and reserve `BPL`/`BMI` for genuinely signed values or for testing bit 7 of a byte you loaded. Prep carry with `SEC` before `SBC`, `CLC` before `ADC` — the same post caught both faults in one reading. | **Nothing here detects it.** `prove_line_budget` and `defuse` look at cycles and at read/write order, not at what a branch *means*; a wrong branch costs the same cycles as a right one. Read the comparison and ask which flag answers the question you asked. | stella-list 〔`199710/msg00043`〕. **The number first recorded here pointed into `199712`, past the end of a month that stops at `msg00101`, and a search over all 18,900 messages then reported nothing.** That search asked for one post carrying **both** this quote and the `#` trap below; the two are in **different messages of the same thread**, so the condition excluded the answer it was looking for. Widening it to either quote alone finds both in a minute (`grep -l 'prepping the carry' */msg*.html`). A search that measures the wrong thing returns a confident nothing, and nothing is indistinguishable from absence — found by the mailing-list distillation (helper-2 for the quotes, helper-1 for the recovery) |
| **A forgotten `#` turns an immediate into a zero-page read** | `AND $f0` reads RAM `$F0`; `AND #$f0` masks the accumulator. The assembler accepts both, and on the 2600 the address form usually reads a byte nothing ever wrote — so it "works" in an emulator that zeroes RAM and does something else on hardware. Same thread, the author's own reply: *"Ummm.... because I forgot. … Also realized the hours didn't work because I was doing `AND $f0` instead of `AND #$f0`. DOH!"* 〔`199710/msg00045`, crackers@, 1997-10-03〕. | Grep new code for `and|ora|eor|cmp|adc|sbc|lda|ldx|ldy` followed by a bare `$xx` where a mask or a constant was meant. | **The detector exists but only covers one of three landing zones**, which is why this row is worth having. `internal/cyclebound/defuse.go`'s **`ReadBeforeWrite`** flags a read that some path reaches with no write to that address having definitely happened first. Where a dropped `#` lands decides whether that helps: **(a) a small constant** (`AND $0F`) reads `$0F`, a TIA *read* register — a real value, never flagged; **(b) `$0E`–`$2C`** reads a write-only TIA register and returns bus residue — the `check_traps` static rule catches that one; **(c) `$80`–`$FF`** is RAM, and `ReadBeforeWrite` only fires if *nothing* wrote it. `$F0` in the 1997 example is unwritten, so it is caught — but **a binary immediate with bit 7 set is always `$80`–`$FF`**, which is where live variables are, so the most idiomatic version of this mistake is the one that slips through. Second report, 2003, from someone who called it a habit: `ORA %10000000` ≠ `ORA #%10000000`. The capability was there; nothing pointed the reader at this use of it. (This is also the concrete use for the `Writers`/`Readers` pair previously noted as unused.) | stella-list 〔`199710/msg00043`〕. **The number first recorded here pointed into `199712`, past the end of a month that stops at `msg00101`, and a search over all 18,900 messages then reported nothing.** That search asked for one post carrying **both** this quote and the `#` trap below; the two are in **different messages of the same thread**, so the condition excluded the answer it was looking for. Widening it to either quote alone finds both in a minute (`grep -l 'prepping the carry' */msg*.html`). A search that measures the wrong thing returns a confident nothing, and nothing is indistinguishable from absence — found by the mailing-list distillation (helper-2 for the quotes, helper-1 for the recovery) |

| **The analogue glue — five boundaries this harness cannot reach, and their name** | A 2001 question to a homebrew author who had cut down a console (`200101/msg00093`) asks how anyone learns *"how do you know **when to use a capacitor**, and what size? … doesn't **cutting out the excess 2600 parts alter the voltage, line noise, and timing requirements**? … how do you learn all of the **'analogue glue'** that holds the digital design together?"* Five separate limits recorded across this file and `fundamentals-audit` turn out to be one category: **the TV's own contrast through RF**, **console-to-console variation**, **the TIA's propagation delays and dynamic logic** (a VHDL model built exactly from the schematic *oscillates*, and real silicon does not), **cable length and wiring**, and **operating temperature** (the engine's `RESPxHBLANK` calls `HeatThreshold()`). | Not a gap to close — a **frontier to name**. This project measures the digital side and can say so precisely: register values, cycle counts, pixel positions, frame structure. Everything downstream of the TIA's pins belongs to somebody with a console, a television and a soldering iron. Say "measured" for the first and "reported" for the second, and never let a green run stand in for a picture nobody looked at. | By construction, no. Each of the five has its own row or note; this one exists so they can be recognised as the same kind of thing. | stella-list `200101/msg00093`; found by the mailing-list distillation (helper-2) |
| **Emulators can agree with each other and disagree with the machine** | Cross-checking one emulator against others — what `cmd/oraclevote` does, and what "Stella 37/37" means — assumes the errors are independent. They are not always. stella-list `200211/msg00098`ff, on a PAL/NTSC auto-detect routine tested on real hardware and three emulators at once: *"It **does display NTSC on my hardware**. It **displays PAL on every emulator I've tried** (Of the emulators I've tried (StellaX, Pcaewin and z26) **only z26 displays Kool-Aid Man correctly. However z26 cheats!! It's not actually doing the emulation it's just detecting the presence of the Kool-Aid Man rom (presumably through checksum because changing even 1 byte in Kool-Aid Man makes z26 display incorrectly as per the other emus) and then compensates to the undocumented bahaviour.** Boy did I ever waste a bunch of time trying to extract the code and test it on z26 before realising this!! (smile))"* **Three emulators, one answer, and it was the wrong one.** A majority vote would have carried the error unanimously. **★And the fourth emulator is worse than wrong.** This passage was elided with `[…]` until 2026-09-06, and what it hides is the sharpest evidence the row has: z26 gave the right answer for that one ROM **because it recognised the ROM**, not because it modelled anything — *"it's just detecting the presence of the Kool-Aid Man rom … and then compensates"*. The author proved it by **changing one byte**, which made z26 fail like the others. That is a fourth way for an emulator to be wrong, and the one a majority vote handles worst: a right answer with no model behind it is indistinguishable from a right answer at the moment you take the vote, and it stops being right the moment your ROM differs by a byte. He fixed Kool-Aid Man's own trap-#67 violation four messages later 〔`200211/msg00102`〕 — the same person, the same month, the same ROM. | Treat emulator agreement as evidence that the emulators share a model, not that the model is right — especially where the thing under test is *how a real chip behaves at the margin*. Prefer a litmus whose claim can be checked against a picture or against documented silicon, and say "our engines agree" rather than "the hardware does" when that is all that was established. | Not detectable from inside. The 2002 thread found it because someone owned a console; nothing in this repository can substitute for that. | stella-list `200211/msg00098`–`00110` (Christopher Tumber, 2002-11-15); found by the mailing-list distillation (helper-1), the elided passage recovered by helper-2 and re-read from the archive here — the quotation above is verbatim to the author's own spelling of *"bahaviour"* |

| **Ten litmus ROMs measure a TIA that has no revision bugs, because ours is switched off** | The engine models **eight TIA-revision behaviours** — `LateVDELGRP0/1`, `LateRESPx`, `EarlyScancounter`, `LatePFx`, `LateColor`, `LostMOTCK`, `RESPxHBLANK` — and **all eight default to false**; nothing in this repository has ever set one. They are not hypothetical: `hardware/tia/revision/bugs.go` names shipped cartridges (`LostMOTCK` → *"Cosmic Ark (starfield)"*, `LateColor` → *"QuickStep"*, and *"some TIAs that are on the edge of tolerance can also exhibit this … such as an RGB MOD"*, `LatePFx` → *"Pesco"*), and `RESPxHBLANK` adds that the effect *"seems to be affected by operating temperature"*. **That last explanation outruns its own citation, checked 2026-09-04.** The comment points at stella-list `199901/msg00089`; that post contains **zero occurrences of "HBLANK" and zero of "temperature"** (counted). What it does contain is a console-to-console discrepancy — *"I get only two copies at Hit5 … **what version of the VCS are you using?**"* — and its author's own hedge, *"**maybe I need to revise my 5 pixel delay theory again**."* The citation is genuine (same phenomenon, same test ROM) and the mechanism is not in it: HBLANK and temperature must come from later work the code does not name. **So the flag models a real effect on an unstated basis**, which is a second reason not to read our flag-off numbers as hardware. **Measured over all 145 litmus ROMs, ten render differently with a flag on, each under exactly one flag**: `LateColor` → `litmus_bpl_trip`, `litmus_dag_region`, `litmus_deadbranch`, `litmus_pagealign`, `litmus_pcm`; `LatePFx` → `litmus_pf0_reflect`, `pf_late`, `pf_wraps`; `LostMOTCK` → `litmus_hmove_mid`, `litmus_hmove_side`. | Not a defect to fix — a **scope** to state. Those ten facts are true of a TIA without those bugs, and a ROM that depends on them may behave differently on a console that has them. When a measurement lands in the ten, say which TIA it describes. **`litmus_respx_phase` was checked specifically and is NOT affected**: `RESPxHBLANK` applies only to a strobe at `hsync == 16 \|\| 18` on rising phi2, and `LateRESPx` only inside HBLANK during a starting HMOVE ripple, and that ROM strobes in the visible area — so the **+5/+4** is safe for the case it measures and unmeasured for the two those flags govern. | `internal/emu/tiarevision_test.go` renders each affected ROM with the flag on and off and fails if the dependency changes in either direction. Five flags move nothing here: the corpus has no fixture reaching their conditions. | `Gopher2600/hardware/tia/revision/bugs.go`, which cites stella-list `199901/msg00089` — *"maybe I need to revise my 5 pixel delay theory again"* — a post inside the archive being distilled; found by the mailing-list distillation (helper-2) |

| **A shipped game DEPENDS on bus residue — the third direction** | The two rows above say *do not read an undriven address*. A 2001 report says a commercial cartridge cannot run unless you do. Someone writing their own emulator found Haunted House (4K PAL) looping forever on `SBC $0F / BCS`: *"Both PCAE and Z26 … leave the accumulator with `$F3`, so **the value subtracted was `$F`**. In my emu, since [it] is subtracting zero … the game **enters into an endless loop**. Since the address `$000F` **can't be read, the data bus isn't updated**, so the subtract is made with `$0F` which is **the last value loaded into the bus**."* Gopher2600 models it — `memory/memory.go:189`, `data \|= mem.LastCPUData & ^mem.DataBusDriven` — so the ROM runs here. | Do not "fix" residue reads out of a corpus ROM you are studying: on some titles the residue **is** the value. Two separate facts follow. **(a) The residue is the last byte the CPU put on the bus**, which is why `INPT4`'s low seven bits read as the previous data rather than as anything about the trigger — `INPT4` drives D7 only. **(b) The pattern is not universal.** The engine's own comment: *"this pattern is good for replicating what we see on the **pluscart** … a **different bit pattern can be seen on the Harmony**"*, and `RandomPins` (default **off**, never set here) exists for the superchip case where the pins are *"more indeterminate"*. So a ROM that leans on residue is portable across a narrower set of hardware than its author knew. | `mining-digest.md:814` already points at a Haunted House disassembly, so the dependency can be confirmed from bytes without opening anyone's commented source. Nothing detects the pattern automatically. | stella-list `200107/msg00044`; `Gopher2600/hardware/memory/memory.go`; found by the mailing-list distillation (helper-2) |

| **`ROL`/`ROR` rotate THROUGH the carry, so two of them are not two of one** | `ROL` shifts bit 7 into C and C into bit 0. Do it once and the carry you started with lands in bit 0; do it twice and the *first* rotation's bit 7 comes back round. A 2004 report reads like a hardware fault and is not: *"1 time gives a good result. **2 times makes a total mess.** First line only → good. Second line only → good. **Both lines and it's a total mess!**"* — answered on the spot, *"is the carry bit messing you up? **The rotate instructions rotate data through the carry bit.**"* The symptom points away from the cause: each half works, so the pair looks like a timing or alignment problem. | `CLC` (or `SEC`) before the first rotate when you want a defined bit in, and remember the second rotate inherits the first's output. If you want a plain shift, `ASL`/`LSR` do not involve C on the way in. | **Do not make this a static check.** The chain is used deliberately here — `maze.md` turns four source bits into eight output bits precisely by rotating twice through carry — so a rule would fire on correct code. "Suspicious, read it" is the right strength. Third of a family this file already has: `cmp` clobbering Z between a load and a zero test, and an immediate `ld_` clobbering N/Z. | stella-list `200404/msg00370`; found by the mailing-list distillation (helper-2) |

### The delay table, measured — and the row that has been wrong since 1998

"The shortest code that wastes exactly N cycles" is a daily 2600 tool, and the table everyone
quotes has a defect its own authors flagged and never fixed.

Andrew Davie posted it 〔`199805/msg00090`, 1998-05-09 11:07:03〕 under the constraint that makes it
useful — *"these delays are designed to be NON-destructive of memory. Ie: they have no effect other
than delay or the accumulator and/or flags"* — and corrected one row **eight minutes later**
〔`199805/msg00091`, 11:15:37〕: *"That should be 5@3. I can't figure a 2 byte non-destructive 5
cycle delay :("*. He fixed the row he was looking at. **`11@4` contains the same `STA $8000,X` and
was not fixed.** Paul Slocum carried the table into the 2600 Cookbook six years later
〔`200404/msg00246`〕 still reading `11@4`, under a heading that says so out loud:

    WASTING CYCLES
    Christopher Tumbler, Chris Wilkson, Andrew Davie
     Todo: Verify Andrew's

That Todo is discharged here. All 21 rows measured 2026-09-06 against the engine, cycles from the
CPU counter and bytes from DASM's own symbol table (`roms/litmus/litmus_delaytable.asm`,
`internal/emu/delaytable_test.go`). **Twenty rows match. One does not:**

| cycles | bytes | code | |
|---|---|---|---|
| 2 | 1 | `nop` | |
| 3 | 2 | `lda $80` | |
| 3 | 1 | `pha` | 2004 |
| 3 | 2 | `dop $80` | 2004, **ILLEGAL opcode** |
| 4 | 1 | `pla` | 2004 |
| 4 | 2 | `nop / nop` | |
| 4 | 2 | `lda $80,x` | 2004 — **+1 cycle for ZERO extra bytes** over `lda $80` |
| 4 | 3 | `lda.w $80` | 2004 |
| 5 | 2 | `dec $2D` | 2004 — ★see the hazard below |
| 5 | 3 | `sta $8000,x` | Davie's own 8-minute correction |
| 6 | 2 | `lda ($80,x)` | |
| 7 | 2 | `pha / pla` | |
| 7 | 3 | `rol $8000,x` | |
| 8 | 3 | `lda ($80,x) / nop` | |
| 9 | 3 | `pha / pla / nop` | |
| 9 | 4 | `lda ($80,x) / lda $80` | |
| 10 | 4 | `rol $80 / ror $80` | |
| 10 | 4 | `dec $2D / dec $2D` | 2004 |
| **11** | **5** | `sta $8000,x / lda ($80,x)` | ★**published as `11@4` for 26 years. It is 5.** |
| 12 | 3 | `jsr` + shared `rts` | 3 = the caller's bytes |
| 12 | 4 | `lda ($80,x) / lda ($80,x)` | |

**The correction is stronger than "the byte count is off by one."** Shortening that row to four bytes
means using `sta $80,x` instead of `sta $8000,x`, which also drops it to **ten** cycles — measured as
a negative control. There is no eleven-cycle four-byte form; the published row names something that
does not exist.

**And `STA $8000,X` is not a write to ROM.** Davie closed the post asking *"Any comments on the
danger of 'writing' to ROM?"* and the thread never answered. There is no ROM at `$8000` on a 2600:
the address bus is 13 bits and A12 selects the cartridge, so `$8000` folds to `$0000` — the TIA. The
litmus makes that visible rather than arguing it: with `x = $09` the "harmless" write sets **COLUBK**
and the background changes colour. Which register it hits is whatever `X` happens to hold, so this
idiom is non-destructive only by luck. `x = $02` folds to **WSYNC** and halts the CPU to the end of
the line — a delay of a completely different length, from an instruction whose whole purpose was to
have a known one.

**`dec $2D` is the 2004 answer to Davie's 1998 lament, and it carries the hazard in the row above.**
The trick is not a cleverer instruction but a better address: *"locations $2D-$3F do nothin and
aren't decoded"* 〔`200404/msg00246`〕 — the instruction is destructive, and there is nothing there to
destroy. Verified here for **writes** (a separate axis from the read-side folding): writing `$FF` to
seven addresses in that range moves no write-only TIA register, with a write to `$09` as the negative
control. But `$2D` sits inside `$00`-`$3F`, so on a **3F/X07 cart it is a bankswitch hotspot** by the
row above — and `scripts/check_traps.py` matches only `nop`/`bit`, so it will not flag `dec $2D`.
The same paragraph in the Cookbook says as much (*"The only case where this might be a problem is if
you're using an unusual bankswitching setup"*), and the engine's own mapper agrees from the other
side: *"tigervision cartridges use mirror addresses to write to the TIA"*
(`Gopher2600/hardware/memory/cartridge/mapper_tigervision.go`).


## E. TIA read / audio / emulator-fidelity

| trap | note | source |
|---|---|---|
| **reading a write-only TIA register** | returns bus float; only **bits 6/7 are driven** (collision/INPT) — don't rely on the full byte | 319781, 328451, 63342 |
| **`save_state`/`restore_state` is bit-exact for the picture and NOT for the sound** | Two causes, additive, and probing each in the vendored engine accounts for all of it (`tia_pcm.bin`, save at a frame boundary, 20-frame detour: 34 → 33 → 1 → **0**). **`Television.audioSignals`**: the TV batches one sample-set per scanline and flushes at `TotalScanlines` (`television.go:489-507`), but the field lives on `Television` and **not on `State`**, so the partial batch is neither saved nor restored. **`Audio.sampleSum`**: `audio.go`'s `Snapshot()` is `n := *au`, so the snapshot shares the averager's backing array; 20 of the 92 `Snapshot` functions under `hardware/` call `copy()`, so deep-copying is the convention and Audio is the exception — blast radius one averaging window, hence exactly one sample. **★How bad depends on WHERE you saved, and the first write-up of this got that wrong by measuring one ROM at one position.** Swept over 3 ROMs × 4 save positions: **at a frame boundary the damage is a short head** (6, 19 or 34 samples) **and heals**; **one scanline later it is not a head at all** and reaches the end of the capture — worst measured **530 of 786 samples, last at 780**. | **Save at a frame boundary and discard the head, or do not restore when measuring sound.** "Discard the first N" is a workaround only for a frame-boundary save. Both probes were applied, measured and reverted — the vendored engine has never been modified here, which is what lets "our engine does X" mean "upstream does X". `internal/emu/audiosnapshot_test.go` asserts the defect, asserts that a boundary save stays a head (so the advice above keeps earning its place), and asserts that a mid-frame save does not. | engine read (mailing-list distillation, helper-2, who predicted the 1-sample half before it was run) + measured 2026-09-05 |
| **the audio clock is FREE-RUNNING, and it is not a picture coordinate** | `hardware/tia/audio/audio.go` keeps its own `clock228`, advances it by **3 per `Step()`** and wraps it at 228; measured 2026-09-05, the identifier appears **nowhere else in the engine** (`rg clock228 Gopher2600` → 6 hits, all in that one file), the `Audio` type has **no `Reset` and no HSYNC hook** (8 methods, none of them), and nothing outside ever writes it. It lines up with the scanline only because a line is also 228 colour clocks, so **anything that changes a line's length — `RSYNC`, a desynced frame — shifts the audio phase against the picture permanently**. Two events per line each: the waveform advances (`phase0`) at clock228 **9** and **81**, and the volume average closes (`phase1`) at **36** and **150** — 114 colour clocks = **38 CPU cycles** apart, which is the window `Vol0`/`Vol1` are averaged over. `Snapshot()` is `n := *au`, a value copy, so save/restore **does** carry the phase. Consequence for authoring a measurement: compare **two runs of the same instruction stream**, never an absolute position within a line. Same family as "a frame index is not a machine quantity" — a quantity that looks synchronised to the picture and is not. | engine read + measured 2026-09-05; found by the mailing-list distillation (helper-2) |
| **2-voice audio phase interference** | two voices can partially cancel to near-silence; Gopher2600's noise (old TIASound) differs from real HW | 294766, 272769, 326549 |
| **AUDF-change propagation latency** | TIA tone is an LFSR-pair (5+4 stage), not a table; **lowering AUDF can take up to ~32 cycles (~1ms)** before the next output clock (the up-counter must wrap) — pitch changes lag, don't assume instant | blog 1116/1134/1140 |
| **mid-line NUSIZ double/quad copy trick** | renders on real HW but **not in Gopher2600** → ROMs using it won't pixel-match our oracle | 181903 |
| **player-width change → 1-clk right shift** | changing NUSIZ player width shifts start by 1 color-clock (missile/ball unaffected) | 143781, 64251, 290654 |
| **The VSYNC "≥2 lines" threshold is an engine setting, and its default is the answer** | `television.go` decides a frame synced with `vsync.activeScanlineCount >= env.Prefs.TV.VSYNCscanlines`, and `preferences/television.go` sets that to **2** in `SetDefaults`. Nothing here changes it (`rg 'VSYNCscanlines' internal cmd pkg` → 0 hits). **A litmus that sweeps 0..4 VSYNC lines and reports "1 fails, 2 passes" has confirmed the number we fed it, not the hardware** — and this is worse than the SuperChip case above, because there the feature is off and the green looks odd, while here the default *matches the literature* so the green looks like corroboration. What is measurable is the shape (does a step exist, and only one?), not the threshold; a Go-side control that sets `VSYNCscanlines` to 3 and shows the step move is what makes the input visible. | engine source, measured 2026-09-03 |
| **SuperChip phantom reads are OFF by default in our engine** | Gopher2600 models the SARA phantom-read recovery (`mapper_atari.go`, `saraCycles = 2`, guarded by `cart.env.Prefs.Cartridge.EmulateSARA`), but `preferences/cartridge_preferences.go` sets **`EmulateSARA` to `false`** in `SetDefaults`, and nothing in this repo sets it (`rg 'EmulateSARA|SARA' internal cmd pkg` → 0 hits). **A litmus written for phantom reads today would come back green because the feature is off, not because the hardware behaves** — it would be measuring our own default. Pin the preference explicitly and say so, or record the behaviour as not modelled. The write/read port split ($F000-$F07F write-only, $F080-$F0FF read-only) needs no preference and *is* measurable as-is. | engine source, measured 2026-09-03 |
| **state isn't all in RAM** | sprite X positions live in TIA internal counters → full-state capture needs `read_tia_registers`, not just `read_ram` | 301766 |
| smooth horizontal PF scroll | **hardware-impossible** (RSYNC won't help); use ball/missile edge or delayed/tile scroll | 178574 |
| PAL: odd-scanline color / interlace | PAL color circuitry fails on odd scanlines; line-dropping kills color | 124445, 200197, 293881 |

---

**Use:** before shipping a kernel, walk this list; the static-detectable rows are the first targets for
`scripts/check_traps.py`. The runtime rows become `breakif`/scenario asserts. For Pong specifically, the
A-section (timing/sync) is the historical cause of every past abandonment.
