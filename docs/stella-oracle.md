# Stella oracle automation (V2-17) — design + status

**Goal (F-4):** automated cross-checks between Gopher2600 (our engine) and Stella (the reference emulator):
run the same ROM to frame N in both, compare RAM ($80–$FF) and TIA state numerically. This upgrades
"emulator-verified" facts (HMOVE side effects, SCORE×PFP, late-HMOVE +8…) toward "two independent
implementations agree".

## Design (validated against the Stella 7.0 docs + installed binary)
1. Place a debugger script next to the ROM (`frame N / tia / riot / dump 80 ff 7 / saveSes`).
2. Launch `Stella -debug -userdir <tmp> <rom>`; the script auto-executes at debugger entry.
3. Poll for the `saveSes` session text file; kill the Stella process (no quit command exists).
4. Parse the RAM dump block from the session file → byte-compare vs the harness `read_ram` at frame N.
   (TIA/RIOT register compare = same parse; pixel compare = v2, needs palette→TIA-index mapping and
   horizontal 2:1 downsampling of Stella's `-ss1x` snapshots.)
5. One-time calibration: a probe ROM writing its frame counter to RAM aligns Stella's `_fCount` with
   Gopher2600 frame numbering.

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
| power-on RAM | 2 | `uninit_trap` and `litmus_uninit_read` feed COLUBK from RAM reset never wrote. Stella randomises power-on RAM (`-plr.ramrandom`, on by default) and is therefore not reproducible: two consecutive captures of the same ROM at the same frame gave COLUBK `$fc` and `$02`. Stella is the one closer to hardware; our defined value is exactly the hazard those ROMs exist to demonstrate |

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

## Automation (v1.33.0)

`scripts/stella_oracle.sh <rom.bin> [frames]` runs the whole loop hands-free: it launches
stellacheck and, in parallel, sends the backquote key to Stella via AppleScript (System Events).
**One-time setup:** grant your terminal Accessibility permission
(System Settings → Privacy & Security → Accessibility). The script preflights the permission and
prints instructions if missing — until then the manual-keypress flow keeps working unchanged.
