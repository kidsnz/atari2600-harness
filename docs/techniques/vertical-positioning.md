# Technique #3 — Vertical positioning (any-Y sprite placement)

**Goal:** put a sprite at *any* vertical position, smoothly, 1 px per frame. Horizontal position
has hardware (RESPx + HMOVE); **vertical has none** — the TIA only knows "what's in GRP0 right
now". Vertical position is therefore a *software illusion*: every scanline the kernel asks
"is the beam inside my sprite?" and feeds GRP0 either an art row or zero.

Learned from (clean-room, ideas only): Darrell Spice Jr. *Let's Make a Game* Step 5; the
`skipDraw` idiom discussion (Davie/AtariAge). Demo: `roms/techniques/vertical_pos.asm`
(a ball bouncing Y 4⇔180 at fixed X=80), locked in CI by `scenarios/vertical_pos.json`.

## The technique

Per visible line (Y register = line number):

```
        tya             ; current line
        sec
        sbc sprY        ; A = row inside the sprite
        cmp #SPRITE_H   ; carry clear ⇔ 0 <= row < height
        bcc .draw
        lda #0          ; outside: blank
        beq .store      ; (always taken — keeps both paths near-equal cycles)
.draw:  tax
        lda Art,x       ; inside: art row
.store: sta GRP0
```

- The subtraction underflows to large values above the sprite, so a single unsigned `cmp`
  covers both "above" and "below" — no second compare.
- Both paths re-converge on one `sta GRP0` (~21 cycles in) — early enough in the line for any
  sprite X ≳ 8, and the line stays far under the 76-cycle budget (~30 cycles total).
- Movement logic runs once per frame in VBLANK; the kernel only reads `sprY`.

### skipDraw / DCP variant — **verified** (v1.23.0, `vertical_pos_dcp.asm`)
The classic undocumented-opcode idiom: per line `lda #H-1` / `DCP sprDraw` ($C7 zp = DEC+CMP) /
`bcs draw`; `sprDraw` initialized to `sprY+H` each frame counts down through 0..H-1 for exactly
H lines; art is stored bottom-up and indexed by the counter. DASM has no illegal mnemonics —
encode with `.byte $C7`. **Measured on this kernel: max line 40→38 cycles, sprite line 31→30**
(2–3 cycles — modest here; the idiom's real value is freeing A/X/Y pressure: no `tya`, so Y
stays available for other per-line work). Pixel-identical to the compare version (CI-locked).
- **Pointer pre-offset:** set `sprPtr = Art − sprY` in VBLANK and `lda (sprPtr),y` in-kernel;
  pairs naturally with masking tables for tall sprites.
- Combine with #2 (animation): `Art` becomes `Frames + frameBase`.

## Verified here (Gopher2600, locked in CI)

- Ball bounces Y 4⇔180; `sprY`/direction asserted by RAM at fixed frames; X pinned at 80 via
  `tia.player0.hmoved_pixel`; 262 lines; line budget clean; golden frame.
- Pixel-level: `read_row` confirms 8 contiguous rows of P0 color at the expected grid rows,
  matching the art **bit-for-bit** (the `%11011011` row reads back as 2-2-2 runs).
- **Calibration is kernel-specific, again:** this ROM's positioning prologue is `lda #imm`
  (2 cy) where sprite_anim's is `lda zp` (3 cy) — 1 CPU cycle = 3 px, so `XCAL` here is −5,
  not −8. Never copy a calibration constant between kernels; re-measure (`read_tia`).

## Harness fix shipped with this technique

`read_row`'s y-coordinate was off by `visibleTop` (~29 lines) from the annotated-grid labels it
promises to match — static-content checks (playfield) were self-consistent, but cross-referencing
a screenshot coordinate missed. Fixed in v1.4.0 (`internal/emu/emu.go ReadRow` now subtracts
`visibleTop`); the grid y you see is now exactly what you pass.
