# Stella oracle automation (V2-17) — design + status

**Goal (F-4):** automated cross-checks between Gopher2600 (our engine) and Stella (the reference emulator):
run the same ROM to frame N in both, compare RAM ($80–$FF) and TIA state numerically. This upgrades
"emulator-verified" facts (HMOVE side effects, SCORE×PFP, late-HMOVE +8…) toward "two independent
implementations agree".

## Design (as built in `cmd/stellacheck` and `scripts/stella_oracle.sh`, on Stella 7.0)
The first plan followed the Stella 7.0 manual (`fundamentals-audit.md` §12). The 2026-06-11 session
and the v3 work of 2026-08-03 (both below) found part of every step did not hold on this setup. These
are the steps the code takes:
1. Write the debugger commands to `~/Library/Application Support/Stella/autoexec.script`:
   `reset / frame N / dump 80 ff 7`, with `savesnap` before the `dump` under `-pixels`
   (`cmd/stellacheck`, which backs up any existing file and restores it afterwards), and only
   `reset / frame N` in `tia` mode (`scripts/stella_oracle.sh`, which overwrites the file and leaves
   it that way). No per-ROM script is written. The manual puts
   `"<rom_filename>.script"` *"in the same directory as the ROM"*, but all 205 captures in
   `internal/oracle/testdata/stella_tia/` record Stella looking for `~/Desktop/<rom>.script` (not
   found) after `Executed 2 commands from` autoexec. There is no `-dbg.script` flag either (Stella
   7.0's `-help` has no such option; harness `CLAUDE.md`).
2. Launch Stella with the ROM path (`cmd/stellacheck`: `exec.Command(stellaBin, romPath)`; `tia` mode
   adds only `-dbg.res` and `-dbg.fontsize`). No `-debug`: it did not enter the debugger on this setup.
   No `-userdir`: it redirected neither the autoexec nor the `saveSes` file. The debugger is entered with
   the backquote key, pressed by a person or sent through AppleScript by `scripts/stella_oracle.sh`,
   and the script auto-executes at debugger entry.
3. RAM and pixel modes (`cmd/stellacheck`) poll for the new `~/Desktop/<rom>_dbg_*.dump` written by
   `dump`, then kill the Stella process. `tia` mode polls for nothing: it pastes `tia` and `saveSes` at
   the debugger prompt (output of commands run from the autoexec is discarded, v3 below), waits fixed
   sleeps, kills the Stella process, and only then takes the one `~/Desktop/session_*.txt` that was not
   there before launch. The code never asks Stella to exit; the nearest command in the 7.0 command list,
   `exitRom`, is *"Exit emulator, return to ROM launcher"*.
4. Parse the RAM rows of the `.dump` file → byte-compare vs the harness RAM at frame N
   (`oracle.Gopher{}.DumpRAM`). The other comparisons use other files: the write-only TIA registers
   come from the session text (`oracle.ParseStellaSession`, v3), and pixels from the `savesnap` PNG,
   which `ingest.Normalize` scales down horizontally and the measured Stella palette maps to TIA
   colour codes (v2).
5. Frame alignment: `reset` at the head of the script makes the snapshot N frames from power-on,
   whenever the debugger is entered. No probe aligning Stella's `_fCount` with Gopher2600's frame
   numbering was built. The two emulators still cut "frame N" at different points within the frame
   (Frame-boundary phase, below). The phase probe that exists, `litmus_framephase`
   (`TestOracleSamplingPhaseIsMeasured`), measures Gopher2600 against MAME; no Stella RAM dump of it
   is kept.

## Status — ✅ WORKING (v1, one human keypress) — `cmd/stellacheck`
The interactive session (2026-06-11) resolved every unknown:
- **Auto-script location**: `~/Library/Application Support/Stella/autoexec.script` (runs at *debugger
  entry* — observed `autoExec(): Executed 3 commands`). The `-userdir` flag does **not** redirect this.
- **`-debug` does not enter the debugger** on this setup; entry needs the debugger key/button once.
  **Frame alignment is solved with `reset` + `frame N` in the script** — the snapshot is exactly N frames
  from power-on regardless of when the human enters the debugger.
- **`dump 80 ff 7` writes a file directly**: `~/Desktop/<rom>_dbg_<hash>.dump` (RAM rows + CPU `XC:` row +
  switches/input `XS:` row); `saveSes` writes `~/Desktop/session_<timestamp>.txt`. Both readable by the
  harness. `exec <path>` also works from the prompt.
- Launching Stella from the harness's sandboxed shell does not reliably show a window; the working flow is
  **the author launches Stella (one command) and presses the debugger key once** — everything else is
  automated by `cmd/stellacheck` (script setup, dump polling, parsing, comparison).

### Results (2026-06-11)
- `smoke.bin` @ frame 5: **RAM $80–$FF all 128 bytes match** (sentinel $42 + zeros).
- `litmus_6502.bin` @ frame 5: **all 128 bytes match** — i.e. the NMOS BCD results (incl. the unreliable Z),
  the JMP ($xxFF) bug path marker, the TIM1T-windowed cycle measurements (read +1 page-cross, store fixed-5,
  branch 2/3/4, illegal DCP=5) and timer behavior are **agreed by two independent emulator implementations**.

### Frame-boundary phase (measured 2026-06-11, exerciser cross-check)
The two emulators cut "frame N" at different points *within* the frame: comparing the Exerciser at
`-frames 5`, **127/128 bytes match** and the only diff is the frame counter (+1); at `-frames 4` the
counter matches and instead the four per-frame-mutating bytes differ by one step the other way. All
structural state agrees — the diffs are boundary phase, not divergence. Conclusion: the oracle's proven
scope today is **frame-stable RAM** (`smoke` and `litmus_6502`: 128/128 PASS); ROMs with per-frame
counters need sub-frame alignment (v2).

**Where line 0 is can be an emulator setting.** In January 2000 Eckhard Stolberg reported that Stella said a
demo drew *"265 scanlines"*; its author replied that PCAE put him *"on line 232"* at each VSYNC and asked how
Stella counts. Erik Mooney took PCAE's line 0 to be the start of the displayable area and estimated 232+37 = 269
(*"probably"*). Eckhard answered that he thought PCAE's line 0 was *"the first line that is displayed on the PC
monitor"*, movable at runtime by shifting the screen, and that with autocenter on *"different games will start
displaying at different scanlines (counting from the VSYNC)"* 〔stella-list `200001/msg00002`,
`200001/msg00004`, `200001/msg00009`, `200001/msg00010`〕. The thread left the three figures unreconciled.
**Cited only, not verified.** The pixel compare does not assume both sides start on the same row (the v2
offset search below).

### v2 — ✅ pixel compare WORKING (v1.54.0)
`stellacheck -pixels` (or `scripts/stella_oracle.sh <rom> <frames> pixels`) adds `savesnap` to the
debugger autoexec, captures Stella's frame PNG, and compares it cell-by-cell against Gopher2600's
frame **as TIA color codes**: the Stella snapshot is quantized with a **measured Stella palette**
(`internal/ingest/palette_stella.go`, all 128 colors captured live from `litmus_palette.bin` via
savesnap — Stella's NTSC RGB differs slightly from Gopher2600's, which a shared quantizer
misreads as ±1-luma code errors), the Gopher frame with the Gopher palette, and the grids matched
over a ±8-line vertical-offset search. **Result: 100.00% agreement on litmus_pf (34,240 cells,
offset +7)**. Offline re-checks: `stellacheck -snap <png>`. Still future: sub-frame boundary
alignment for per-frame-mutating RAM.

### v3 — ✅ TIA WRITE-register compare WORKING (G4)
RAM and pixels agreeing did not settle the write-only registers, and pixels never can: an object
whose graphics are `0` renders identically whatever its NUSIZ, so a wrong reading of
`read_tia_registers` could hide behind a right picture indefinitely. v3 compares the registers
themselves.

**How Stella can and cannot be asked** (all measured on Stella 7.0, 2026-08-03):

| channel | result |
|---|---|
| `dump 00 3f 1` (writes a file, autoexec-safe) | **does not reach them** — returns the TIA *read* ports (collisions/INPT) mirrored every `$10`: `00: 00 00 80 00 …` repeated at `10:`/`20:`/`30:` |
| `saveState`/`saveStateIf` from autoexec | wrote no file at all |
| debugger expression language (`print`, `ram`) | no accessor: the pseudo-registers are `_bank/_cClocks/_cyclesHi/_cyclesLo/_fCount/_fCycles/_iCycles/_scan/_scanEnd/_vBlank/_vSync`… — none is a TIA register |
| `tia` command ("Display text-based output of the contents of the TIA tab") | **reports them**, but only to the prompt widget |
| `tia` + `saveSes` inside `autoexec.script` | **0-byte file** — `Debugger::exec()` keeps only the `Executed N commands` summary and discards each command's output |

So the only working channel is *typing at the debugger prompt*. `scripts/stella_oracle.sh <rom>
<frames> tia` does that: it launches Stella, presses `` ` ``, then pastes `tia` and `saveSes` via
the clipboard (a plain `keystroke` is eaten by the Japanese IME) and re-activates Stella before
every keypress so a browser stealing focus cannot receive them. ~13 s per ROM. The session is
moved straight out of `~/Desktop` (Stella's user dir; `-userdir` does **not** redirect it) into
`internal/oracle/testdata/stella_tia/<rom>.txt` with a `# rom:`/`# frames:` provenance header.
`cmd/stellacheck -session <file>` then re-grades a capture offline, and
`internal/oracle.TestStellaAgreesWithHarnessOnWriteOnlyTIARegisters` re-grades every capture
against a fresh Gopher2600 run on every `go test`.

**What each field of that text means** was fixed by `internal/oracle/testdata/tiaprobe.asm` and its
mirror `tiaprobe2.asm`, which write one distinct constant to every register and then stop touching
TIA — reading the conventions off Gopher2600 would have made the comparison circular. Established
that way: `HM=$7` is the **raw HMxx nibble** (`$70` → `$7`), a missile/ball `size=#N` is the **raw
2-bit field** (not a pixel width), `GR=%…` and the ball's `ENABLED` are the **NEW** copy of the
VDEL-shadowed registers (probe has GRP0 new `$A5` / old `$22` with VDELP0=1 and Stella prints
`$A5`), `PF0` is printed already shifted down (`$B0` → `$0b`), and UPPERCASE spells a set flag.

**37 registers are compared per ROM**: COLUP0/1, COLUPF, COLUBK, GRP0/1, the NUSIZ player mode and
missile size for both objects, CTRLPF's reflect/score/priority/ball-size, REFP0/1, VDELP0/1/BL,
ENAM0/1, ENABL, RESMP0, PF0/1/2, HMP0/HMP1/HMM0/HMM1/HMBL and AUDC/AUDF/AUDV on both channels.

**Corpus result (2026-08-03).** 147 captures — every one of the 114 `roms/litmus` and 31
`roms/techniques` ROMs plus the two probes — at frame 5: **5,439 register readings, 19
disagreements, 0 divergences**. All 37 registers take more than one value across the corpus, so the
denominator is not a constant compared with a constant. The test prints all of those counts and
fails if any corpus ROM has no capture, because partial coverage that looks like a pass is the
failure mode this repo keeps finding.

The 19 disagreements are classified from measurement by `oracle.ClassifyTIADiffs`, never by
assertion, and every one is printed either way:

| class | count | what was measured |
|---|---|---|
| sub-frame phase | 7 | our side holds Stella's exact value at some scanline of the next frame — `litmus_hmxx_freeze` sets `HMP0=$80` right after VSYNC and `HMCLR`s it later in the same frame, so the two emulators' frame boundaries fall either side of one store; likewise `shared_setxpos` (5 HM registers) and `two_line_vdel` (VDELP0) |
| undefined at power-on | 10 | `litmus_cycles` and `uninit_trap` contain no `HMxx` or `HMCLR` write at all, and all five motion registers read Gopher2600's power-on nibble 8 (`HMxx=$80`, its zero-valued `(v^$80)>>4` field) against Stella's 0 — a real TIA leaves them undefined, so neither is the right answer |
| power-on RAM | 2 | `uninit_trap` and `litmus_uninit_read` feed COLUBK from RAM reset never wrote. Stella randomises power-on RAM (`-plr.ramrandom`, on by default) and is therefore not reproducible: two consecutive captures of the same ROM at the same frame gave COLUBK `$fc` and `$02`. Stella's random bytes are closer to a heavy sixer, our zeros to a 2600 Jr., and neither matches a 7800; our defined value is exactly the hazard those ROMs exist to demonstrate. In 2002 a ROM that puts RAM on screen at power-up, burned to EPROM, gave different bytes across power cycles on a heavy sixer (five dumps; the first two rows usually alike), all `00` over 20 power cycles on a 2600 Jr., and the same bytes every time on a 7800 modified with a dev OS (Albert Yarusso), and zeroes on a PAL Junior except where the program's own variables sit (Matthias Domin); Eckhard Stolberg on the 7800: *"all 7800 BIOS versions I have seen use the same code in RIOT RAM to switch the 7800 into 2600 mode. So on a 7800 you can't get random values from RIOT RAM."* 〔stella-list `200211/msg00175`, `200211/msg00181`, `200211/msg00183`, `200211/msg00191`〕. This engine starts all 128 bytes at 0 (`RAM.Reset` with `RandomState` false, which no harness tool sets; `internal/emu/randomstate_test.go` turns it on as a witness). **Cited only, not verified** on any console |

The classifier is itself planted against: `TestTheClassifierCannotExcuseAPlantedDefect` feeds it a
Stella value our side holds at no instant of the frame and requires the verdict `divergence`.

**Not covered, by name** (`oracle.TIARegsNotReported`): VSYNC and VBLANK (Stella prints only a
blanking flag, and `emu.TIARegisters` has no VBLANK at all); the raw NUSIZ and CTRLPF bytes (both
sides report the decoded fields, so the TIA-unused bits 3/6/7 are not compared); the *old* copies
of GRP0/GRP1/ENABL (Stella's text prints only the new one); the strobes, which hold no value; and
**RESMP1**.

**RESMP1 is a Stella defect, not ours.** `tiaprobe.asm` writes `RESMP0=$02 / RESMP1=$00` and Stella
prints the reset flag **set on both** missile lines; `tiaprobe2.asm` writes the mirror
(`RESMP0=$00 / RESMP1=$02`) and Stella prints it **clear on both**. Stella's M1 flag equals RESMP0
in both cases and RESMP1 in neither, so it is not a usable oracle for that register; our own
reading matches what each ROM writes. `TestStella70MisreportsRESMP1` locks the behaviour so a fixed
Stella makes the test fail and the register can be put back.

**An outside case in the other direction.** The power-on RAM row above has Stella closer to a heavy
sixer and our zeros closer to a 2600 Jr.; here Gopher2600 was the one closer to hardware. Flap Ninja's demo went back to its title screen whenever the button was
pressed, on a PAL light sixer with a Harmony cart (Bomberman94). MarcoJ found that Gopher2600 *"behaves like a
console"* on that ROM, while *"Stella with developer mode"* could not be made to; the author (kikipdph) put it
down to bank switching, replaced the binaries with ones that worked on Gopher2600, and MarcoJ confirmed they
reached gameplay on a PlusCart — fails on hardware, reproduced in one emulator, fixed, confirmed on hardware
(AtariAge `topic/367912`). **Cited only, not verified** — the ROM was not run here, and one case says nothing
about which emulator is closer in general.

## Automation (v1.33.0)

`scripts/stella_oracle.sh <rom.bin> [frames]` runs the whole loop hands-free: it launches
stellacheck and, in parallel, sends the backquote key to Stella via AppleScript (System Events).
**One-time setup:** grant your terminal Accessibility permission
(System Settings → Privacy & Security → Accessibility). The script preflights the permission and
prints instructions if missing — until then the manual-keypress flow keeps working unchanged.

## Stella facilities this oracle does not use

Collected from forum threads. Where Stella 7.0's bundled manual (`Stella.app/Contents/Resources/docs/`
`index.html` and `debugger.html`) names the same thing, that is said; **none of it was run here**.

- **The TV format can come from the ROM's file name — a hazard for this oracle.** A PAL60 ROM under TV
  Format = Auto Detect is detected as NTSC and its colours come out wrong (jamtex, AtariAge `topic/308669`);
  a name containing `PAL60`, `PAL-60`, `PAL_60` or `PAL 60` gets the right format (thomas-jentzsch, same
  thread). The patterns inside the Stella 7.0 binary (`strings`) are `[ _\-(\[<]+PAL[ _-]?60` for PAL60 —
  which admits the underscore form, and needs a separator before `PAL` — and `[ _\-(\[<]+PAL[ _\-)\]>.]` for
  PAL; the manual's filename table lists only `PAL60, PAL 60, PAL-60`, and its colour-based `-detectpal60` is
  described as *"not very reliable"*. `cmd/stellacheck` launches Stella with the ROM path alone
  (`exec.Command(stellaBin, romPath)`), so on the command line the file name is the only format hint Stella is
  given. No capture in `internal/oracle/testdata/stella_tia/` fits the PAL60 pattern; two fit the PAL one —
  `litmus_pal` and `litmus_pal_physics` — if Stella matches regardless of case, which was not checked
  (**Not verified**), and the captures do not record which format Stella chose. The engine's own PAL60
  inconsistency is a separate matter, pinned by `internal/emu/pal60rate_test.go`.
  Stella also keys the format to the ROM's MD5. Asked in 2014 how a game could make Stella use PAL60, SpiceWare
  answered that the author sends stephena the final ROM and settings, and the next release then uses *"the ROM's
  MD5 value to automatically use PAL60"*; until then a user sets Game Properties → Display → Format from
  Auto-detect to PAL60, and *"any time that ROM (as identified by the MD5 value) is loaded it will use PAL60"*
  (AtariAge `topic/224967`). Our reading: a Format saved that way in the Stella on this machine would apply to
  an oracle run without appearing on the command line `cmd/stellacheck` builds, and would not follow a rebuilt
  ROM whose bytes, and so MD5, changed. **Cited only, not verified.**
- **Jitter/roll.** The 7.0 manual: `-plr.tv.jitter` / `-dev.tv.jitter` — *"Enable TV jitter/roll effect, when
  there are too many or too few scanlines per frame"* (Alt+J / Cmd+J). stephena suggested it to an author whose
  game held steady on hardware except for a jump at the moment a wave was cleared, and barely showed it in
  Stella: make the failure visible first, then read the scanline count. SpiceWare's Frame Stats (Alt+L /
  Cmd+L) showed 272, and 283 with RESET held — *"the difference of 11 will cause the screen to jump"* (AtariAge `topic/281093`). This
  harness counts instead of showing: scenario `frame_lines_stable` (`docs/scenarios.md`); `internal/crt` has
  no vertical axis for it (`rg -i 'jitter|roll' internal/crt` → 0). **Not verified** — the effect was not
  turned on here.
- **Developer settings first.** thomas-jentzsch to an author, in his first post, testing an 8K game only in
  Stella: *"When testing with Stella, please make sure that you have the developer options set. This will
  help you to identify most of the errors."* The author had not known the developer menu and found *"one problem to
  fix already"* (AtariAge `topic/327208`, 2021). The 7.0 manual: `-dev.settings <1|0>` *"Select developer
  (1) or player (0) set"*, toggled with Alt+D / Cmd+D. "Most" carries no count, and the Flap Ninja case
  (Status, above) is a hardware fault Stella with developer mode could not be made to reproduce.
  `cmd/stellacheck` passes no `-dev.settings`. **Cited only, not verified.**
- **Undriven TIA bits.** reveng: run Stella with `-dev.settings 1` and `-dev.tiadriven 1` to make a read bug
  such as `lda 0` written for `lda #0` obvious (AtariAge `topic/298427`). The 7.0 manual: *"Set unused TIA pins
  to be randomly driven high or low on a read/peek. If disabled, use the last databus value for those pins
  instead."* This engine returns the last bus byte (`internal/emu/floatbits_test.go`) — the model under which
  that bug usually reads back the intended value. **Not verified** — `-dev.tiadriven` was not run here.
  reveng gave the flag alone in 2019, to a first-time homebrew author: *"It looks like you're relying on undriven bits to be
  a certain value, which may work on most consoles, but isn't great for compatibility reasons. Run the game
  in stella with the `-dev.tiadriven 1` option, which changes the undriven bits randomly, to see what I
  mean."* (AtariAge `topic/283352`; held here only as distilled notes; year and wording not re-checked).
  **Cited only, not verified.** This
  engine's counterpart is the `RandomPins` preference, off by default and set by no harness tool;
  `internal/emu/floatbits_test.go` turns it on only as a negative control, asserting that `litmus_floatbits`
  then reads different bytes, and its comment records the same bytes on every run — a fixed pattern, not a
  fresh draw.
- **Four standing guards** (Bruce-Robert Pocock, AtariAge `topic/353053`):

  ```
  breakIf { _scanEnd < #262 && _scan == 0 && _fCount > 1 }
  breakIf { _scan > #262 }
  breakIf { sp < $f0 }
  breakIf { pc < $f000 }
  ```

  A short frame (seen only once the next frame's first line is produced, hence `_scan == 0`, then walked back
  with Stella's rewind), an overrun, the stack pointer below the top of the variables (`$f0` for variables at
  `$80…$EF`), and a jump out of a ROM based at `$F000`. He suggests a file such as `~/fly29.script` to have
  them loaded automatically. Where Stella 7.0 looks for a per-ROM script is itself in doubt: its manual says
  `"<rom_filename>.script" (located in the same directory as the ROM)`, but every capture in
  `internal/oracle/testdata/stella_tia/` records `script file '~/Desktop/<rom>.script' not found`, after
  running `autoexec.script` (Status, above).
  **Not verified** — the guards were not run.
- **Trapping a strobe.** `trapwrite RESP1` stops on every write to it: on Dragon Fire it showed RESP1 written
  four times a frame and HMOVE used for only three of those positionings (SpiceWare, AtariAge `topic/219525`).
  The conditional forms stephena announced for Stella's git, `trapreadif`/`trapwriteif` (AtariAge
  `topic/271098`), are in the 7.0 command list as `trapReadIf`/`trapWriteIf`, *"On <condition> trap write
  access to address(es) xx [yy]"*. For a ROM with no source this engine's counterpart is
  `cmd/beamtrace -rom <bin>`, which lists every RESP0/1 and HMOVE write per scanline with its beam clock.
  **Not verified** — neither trap was run here.
- **Positions are the emulator's to show, not the program's to read.** The HPos Stella's debugger displays —
  like this harness's `read_tia` `ResetPixel`/`HmovedPixel` — is internal state; there is no way to ask the TIA
  for it, so a game that needs an X keeps it in a RAM byte and repositions from it every frame (Karl G,
  SpiceWare, AtariAge `topic/304108`). **Cited only, not verified.**
- **Snapshots in place of an input log.** Stella has no input recording; `saveStateIf` saves an extra state
  whenever a condition holds, e.g. when a given input happens (thomas-jentzsch, AtariAge `topic/330559`), so
  a rare input-dependent bug is reproduced from a state rather than a replayed input. The v3 table above
  records that `saveState`/`saveStateIf` issued from `autoexec.script` wrote no file here; a call that does
  work has not been found. **Cited only, not verified.**
- **ARM cartridges: Stella's line count can be wrong.** alex_79 put a logic analyser on a 4-switch's TIA
  output pins and read 263 lines from a DPC+ demo that Stella reported as 262; SpiceWare: Stella's
  ARM emulation reports how many instructions ran, not how long, so Stella counts ARM code as 0 cycles,
  *"for DPC+ as well"* (AtariAge `topic/183085`, 2016; later Stella versions not checked). **Cited only, not
  verified.** Every capture here is of a `roms/litmus`, `roms/techniques` or probe ROM (their `# rom:`
  headers), and none of those uses DPC+ (`rg -il 'dpc' roms/litmus roms/techniques --glob '*.asm'` finds only
  an ADPCM comment).
- **ROMs that break emulators**, thomas-jentzsch's list for a new emulator's author in 2015: Galaxians,
  Meltdown, Pole Position, Kool-Aid Man, Swoops! (AtariAge `topic/241103`). In the same thread DirtyHairy
  says Meltdown's left/right asymmetry was fixed in Stella and 6502.ts by better modelling of NUSIZ during
  draw/decode (stella-emu/stella issue #63; an earlier, less accurate fix in #56). Kool-Aid Man is already in
  `known-traps.md` for another reason (z26 recognised the ROM rather than emulating it). None of the five has
  been captured. **Cited only, not verified.**
