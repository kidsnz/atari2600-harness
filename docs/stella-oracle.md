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
alignment for per-frame-mutating RAM; TIA write-register compare.

## Automation (v1.33.0)

`scripts/stella_oracle.sh <rom.bin> [frames]` runs the whole loop hands-free: it launches
stellacheck and, in parallel, sends the backquote key to Stella via AppleScript (System Events).
**One-time setup:** grant your terminal Accessibility permission
(System Settings → Privacy & Security → Accessibility). The script preflights the permission and
prints instructions if missing — until then the manual-keypress flow keeps working unchanged.
