# The Exerciser ROM — every verified capability, composed in one cartridge

`roms/exerciser/exerciser.bin` (8K, F8) is the harness's **integration showcase**: each scene composes
capabilities that were first hardware-verified in isolation (litmus ROMs + the techniques catalog), proving
they survive **composition** — shared 76-cycle lines, 128 bytes of RAM, and a bankswitched layout.
Completion of this ROM marks **v1.0.0**.

Fire advances scenes (edge-detected, wraps). Every scene is a subroutine consuming exactly 192 scanlines;
the bank-0 framework owns VSYNC/VBLANK/overscan and input.

| # | Scene | Composes (verified parts) |
|---|---|---|
| 0 | **Title** | 48px "EXRCSR" (six-store VDEL kernel, one glyph/copy) + live 6-digit BCD score (indirect digit pointers = nibble×16 into a page-aligned 16-byte-stride font) + **2-channel music driver** (Sequencer-Kit note codec, per-frame tick) |
| 1 | **Zone landscape** | Asymmetric-PF mountains (pf_async write windows) + 12 sprites via zone multiplexing with per-frame drift + per-zone colors |
| 2 | **Playground** | Joystick-driven P0, auto-firing missile (per-frame HMOVE drift), ball pole, PF walls; **collision latches → live color feedback** (read-then-CXCLR) |
| 3 | **Gradient + SFX** | Per-scanline COLUBK rainbow + scene-entry kick drum (Slocum recipe, decaying AUDV) |
| 4 | **Procedural** | Per-scanline starfield from the math-verified Galois LFSR; world seed evolves every 64 frames |

> A paddle scene existed briefly (v0.60.0–v1.0.0) but was removed in v1.0.1: Stella's controller
> auto-detection sees the ROM's INPT0 reads, plugs paddles into the left port, and **paddles hold INPT4
> high** — making the joystick fire (scene advance) dead on real-world players. Paddle capability remains
> fully verified in `litmus_paddle` + the harness's paddle input path.

## How it is verified (CI, every push)
`roms/exerciser/scenarios/`: a navigation scenario cycles all five scenes by input timeline; per-scene
scenarios assert sentinels, positions, colors, paddle counts, collision feedback, the **music note timeline
register-by-register**, and lock video goldens (+ an audio golden for the title music). Every scenario also
asserts a 262-line frame — which doubles as proof that no scene ever exceeds the 76-cycle line budget.

## Composition lessons it caught (why an integration ROM is not redundant)
Unclosed HMOVE lines merging with computation (263-line frames), per-line work exceeding budget only on
collision frames, VDEL shadows replaying stale graphics on staging lines, page-cross shears from unaligned
tables, scene-entry state initialization (lastScene latch), and end-of-frame CXCLR starving the next
frame's collision check. Each is the kind of bug that only appears when verified parts are combined.

## Cross-emulator check
`cmd/stellacheck -rom roms/exerciser/exerciser.bin -frames N` compares RAM against Stella (one human
keypress to enter Stella's debugger; see `docs/stella-oracle.md`).
