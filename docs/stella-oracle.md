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

## Status — PARKED (needs an interactive session)
Headless attempts (2026-06-11) did not trigger script auto-execution: `<rom>.script`, `<rom.bin>.script`
and `autoexec.script` (in `-userdir`) all produced no session file, with empty console logs; the debugger
window opens but the scripted commands appear not to run unattended. The remaining unknowns (exact
auto-script naming/location, where `saveSes` writes, whether the debugger needs focus/input) are best
resolved **with the author watching the debugger window** — type `exec <script>` / `saveSes` in the prompt
once, observe the output location, then encode that into the driver. The driver itself is mechanical once
the invocation is known.

Confirmed so far: Stella 7.0 launches into the debugger with `-debug`; `-userdir` relocates user files;
the prompt supports `frame/dump/tia/riot/saveSes/breakIf/trapWrite` and pseudo-registers (`_fCount`,
`_scan`…) per the official docs — the capability exists; only the unattended trigger is unresolved.
