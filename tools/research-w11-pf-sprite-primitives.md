# PF/Sprite Primitive Catalog — TIA Studio Template Parts

**Purpose**: Distillation of small educational demos from `reference/disassemblies/bjars_site_archive/public_html/`
into generalized building-block descriptions for TIA Studio template authoring. These are component-level
primitives (not full game zone kernels; those are covered in `research-w9-real-kernel-patterns.md`).

Each section describes: what the primitive does, cycle/byte budget, constraints and failure modes, and
relationship to existing `harness/docs/design-principles.md` principles.

Sources (all under the bjars archive `source/` or `resources/` directory unless otherwise noted):
- `playfieldprimer.asm` — Williams/Saunders, asymmetric reflected PF
- `MirrorPlay.asm`, `SymmPlay.asm`, `AsymPlay.asm` — Bogey, PF mode economy comparison
- `Asym2scrol.asm` — Bogey, independent dual-side vertical scroll
- `BackScroll.asm` — Bogey, background colour cycling (colour-as-scroll)
- `Left_Scroll.asm`, `right_Scroll.asm` — Bogey, symmetrical PF horizontal scroll via RAM shift
- `bigspritedemo.asm` — 48-pixel sprite, NUSIZ$03 + VDEL double-buffer
- `movingmissile.asm` — Bailey/Ruffin, joystick-driven missile with HMOVE
- `resources/score3x2.asm` — 6-digit score display (P0+P1, NUSIZ$06, VDEL, SP-as-temp)
- `resources/13_plus2.asm` — Slocum, 13-char + 2 text renderer (SP-as-buffer)
- `resources/bigmove.asm` — same 48px sprite with sub-pixel coarse+fine horizontal move
- `original/CaveIn/score_graphics.asm` — compact digit ROM table (8 bytes per digit)
- `source/Heart_Color.asm` — Bogey, mirrored PF with per-scanline colour cycling
- `source/colors.asm` — Watson, color-picker tool (asymmetric PF title, main kernel with per-zone COLUPF/COLUBK)

---

## 1. Symmetric PF Primitive

### What it does
Displays a full-height (up to 192-line) playfield where the right half mirrors the left half.
CTRLPF D0 = 0 (repeat) or D0 = 1 (reflect). A single per-line table of three byte columns (PF0, PF1, PF2)
drives the entire frame. The kernel is a simple decrementing index loop: load three bytes, write PF0/PF1/PF2,
WSYNC, loop.

### Cycle budget
The inner loop body (load PF0, STA PF0, load PF1, STA PF1, load PF2, STA PF2, WSYNC, DEX, BNE) is
approximately 30 cycles, well within the 76-cycle line budget. Because WSYNC absorbs the tail, there is no
per-line timing pressure. The entire 192-line draw costs 192 WSYNCs.

### Byte budget
Three ROM tables × 192 bytes each = 576 bytes minimum for a full-height unique-per-line image. For
repeating backgrounds a much smaller table (e.g., 4–8 entries) is sufficient and the index is computed
modulo the table length.

### Constraints and failure modes
- PF0 data is upper-nibble only (D4–D7). Lower four bits are ignored by TIA. Forgetting this shifts
  the leftmost four columns right by 4 pixels visually.
- PF1 is MSB-first (D7 = leftmost col 4), PF2 is LSB-first (D0 = leftmost col 12 of left half).
  Bit-order confusion is the most common pixel-layout bug.
- Data tables must be indexed in reverse (high scanline first) because the loop decrements the index.
  `MirrorPlay.asm` and `SymmPlay.asm` both comment "Data lines are reversed".
- CTRLPF must be written before VBLANK end; writing it inside the kernel causes a mid-frame mode glitch.

### TIA Studio template parameters
- `height` (1–192 scanlines)
- `mode` (repeat | reflect; maps to CTRLPF D0)
- `color` (single COLUPF value, or a per-scanline colour table if colour cycling is needed)
- `pf_table` (the three-column bitmap table; generator encodes bit-order automatically)

### Relation to design-principles
Reinforces existing principle: "Horizontal resolution is fixed; vertical rhythm drives expressiveness."
No new principle needed here — this is the simplest PF form.

---

## 2. Asymmetric PF Primitive

### What it does
Displays a full 40-column playfield (20 left + 20 right) where the right half is independently specified.
Hardware constraint: TIA begins drawing the right half partway through the scanline, so PF0/PF1/PF2 must
be written twice per scanline — once for the left half (before the beam reaches it) and once for the
right half (while the left half is visible, targeting the exact cycle window when the right-half registers
latch).

The Bogey `AsymPlay.asm` uses a SLEEP macro: after writing the left-half values (PF0/PF1/PF2), it issues
SLEEP 4 then rewrites PF0 (right), SLEEP 4, PF1 (right), SLEEP 4, PF2 (right), then WSYNC. Six register
writes per scanline versus three for symmetric mode.

### Cycle budget
Left-half writes: 3 LDA + 3 STA = approximately 21 cycles (must complete before the left half is visible,
roughly before cycle 7 from the WSYNC boundary, which includes the PF0 window at ~cy7 of the visible
region). Right-half writes must land at the canonical deadlines:
- PF0 right: cycle 31 from WSYNC (7 cycles after visible midpoint ~cy24)
- PF1 right: cycle 38
- PF2 right: exactly cycle 45 — one extra NOP destroys alignment

This leaves 76 − 47 ≈ 29 cycles per line for non-PF work (sprite positioning, colour changes, etc.).
This is the same 45-cycle constraint documented in design-principles under "非対称PFの書込み締切".

### Byte budget
Six ROM tables × 192 bytes = 1152 bytes for a full-height asymmetric image. This is 28% of a standard
4 K ROM. For 2 K ROMs, asymmetric PF over the full height is often impractical without RAM self-modification
or a reduced-height zone.

### Constraints and failure modes
- The SLEEP between left and right writes must be cycle-precise. A single extra NOP after the PF2-right
  write was observed in `playfieldprimer.asm` to break the image (comment: "adding even this one nop breaks
  this kernel"). This is the tightest timing constraint in any standard PF kernel.
- PF0 has only a ~20-cycle window before it starts latching for the left half (per the design-principles
  note "PF0 window is ~20cy only"). This means the left-half PF0 write must happen very early in each line.
- If any branch or conditional logic varies by scanline, the cycle count can change and slide the right-half
  writes off-deadline, producing a visible tear.
- CTRLPF must have D0 = 0 (repeat mode, not reflect) for true asymmetry. Using reflect mode with two writes
  produces mirrored halves, not independent halves.

### TIA Studio template parameters
- `height` (1–192 scanlines)
- `left_pf_table` (3 × N bytes: PF0_L, PF1_L, PF2_L per scanline)
- `right_pf_table` (3 × N bytes: PF0_R, PF1_R, PF2_R per scanline)
- `spare_cycles` (budget assertion: must be ≤ 29 cy/line for non-PF operations on the same scanline)

### Relation to design-principles
This primitive directly instantiates the existing "非対称PFの書込み締切" principle (cy7/14/21 left;
cy31/38/45 right; 29 cy free). No new principle; the doc already has the precise numbers.

---

## 3. Reflected (Non-Symmetric) PF with Two Independent Half-Patterns

### What it does
A middle path between symmetric (D0=1 single write) and fully asymmetric (D0=0 with two timed writes):
using CTRLPF D0=1 (reflect) but writing PF registers twice per line with different values for each half.
The result is a mirrored layout where left columns 0–19 are drawn left-to-right and right columns 20–39
are drawn right-to-left — giving visual symmetry — but per-line colour changes via COLUPF still work.

`playfieldprimer.asm` implements this: CTRLPF D0=1, six data tables (PFData0/1/2 for left, PFData0b/1b/2b
for right), same 45-cycle right-half deadline applies. The result is "asymmetric content, reflected layout."

### Cycle budget
Same as asymmetric PF above. The reflection flag changes the visual output but not the timing requirement.

### Constraints and failure modes
Same as asymmetric PF. The additional subtlety is bit-order: in reflect mode, the right-half columns scan
in reverse order compared to the left half, so PFData2b bit D7 maps to column 39 (rightmost visible pixel),
not column 12. Designers must account for this reversal when computing bit patterns.

### TIA Studio template parameters
- Same as asymmetric PF, plus `reflect` flag (boolean)
- When reflect=true, the generator must bit-mirror the right-half tables before storing them in ROM.

---

## 4. Vertical PF Colour Cycling

### What it does
Changes COLUPF (or COLUBK) on every scanline inside the display loop, producing a gradient or animated
colour wash over a static PF bitmap. `Heart_Color.asm` increments a colour index register each frame
(DEX applied each WSYNC) and stores the result in COLUPF immediately before the next WSYNC. This is pure
vertical colour: the same PF pixel changes colour each scanline.

`BackScroll.asm` uses the same idiom for the background only (STX COLUBK, INX, WSYNC per loop iteration),
producing a scrolling colour rainbow with no PF bitmap at all.

### Cycle budget
One additional STA COLUPF (3 cycles) or STX COLUBK (3 cycles) per scanline. Cost is negligible.
Symmetric PF with per-scanline colour leaves 76 − (30 + 3) = 43 free cycles. Asymmetric PF with per-scanline
colour leaves 29 − 3 = 26 free cycles.

### Constraints and failure modes
- The write must occur before WSYNC, not after. Writing after WSYNC takes effect on the following scanline,
  shifting all colours by one row visually.
- COLUPF is shared between PF and ball object; if the ball is in use, its colour follows any COLUPF change.
  Use CTRLPF D1 (score mode) to decouple PF from player colors, but then COLUPF only affects PF while
  COLUP0/COLUP1 colour the left/right PF halves independently (useful for dual-colour PF without asymmetric
  writes).
- The frame-level colour offset (e.g., `Color_Start` incremented each frame) creates the animation. The
  hardware does not scroll colours; the CPU computes a new start value each frame.

### TIA Studio template parameters
- `color_table` (up to 192 entries) or a formula (start_color, delta_per_line, delta_per_frame)
- `animate` (boolean — whether to increment frame-offset each frame)

### Relation to design-principles
Reinforces existing principle: "多色は縦に足す = 走査線ごとに COLUPx 書換え". **New detail**: CTRLPF D1
(score bit) splits COLUPF into left-half (COLUP0) and right-half (COLUP1), enabling a two-colour PF without
asymmetric write timing. This nuance is not explicitly called out in design-principles.

---

## 5. Horizontal PF Scroll (Symmetric, Bit-Shift)

### What it does
Smoothly scrolls a PF bitmap left or right by rotating the bit patterns across PF0/PF1/PF2 each frame.
`Left_Scroll.asm` and `right_Scroll.asm` copy the PF tables to RAM at startup, then rotate the RAM image
each frame using shift-rotate chains (LSR/ROL/ROR across PF2→PF1→PF0 for left scroll; ASL/ROR/ROL for
right scroll), with carry propagation to pass bits across register boundaries. The rotated RAM image is
then output per-scanline during the kernel loop.

A `Scroll_Speed` constant (default 2) throttles the animation by skipping rotate updates on non-multiple
frames.

### Cycle budget
The rotation code runs during VBLANK (approximately 2785 free cycles available per frame after the TIM64T
is loaded). Four iterations of the 3-register shift loop cost approximately 12 × 5 = 60 cycles — trivially
affordable in VBLANK. Display kernel cost is identical to a symmetric PF kernel (no timing change).

### Byte budget
RAM: 4 lines × 3 registers = 12 bytes of RAM for the scrolling portion of the PF. The total PF RAM can be
larger for taller scrollable regions.

### Constraints and failure modes
- The three PF registers are not contiguous in TIA address space, but the demo maps corresponding
  RAM bytes contiguously (PF0_L1..PF0_L4 at $80–$83, PF1_L1..L4 at $84–$87, PF2_L1..L4 at $88–$8B).
  The shift loop accesses them as PF2_L1-1,X / PF1_L1-1,X / PF0_L1-1,X, using index arithmetic.
- Bit carry must bridge PF0→PF1→PF2 in the correct order, accounting for each register's bit-order
  reversal (PF1 is MSB-first, PF2 is LSB-first). Getting the carry direction wrong causes an
  apparent stutter or reverse scroll.
- Scroll speed 1 (every frame) is the maximum rate. Speed N updates every N frames. There is no sub-frame
  smoothing; horizontal position is always aligned to 4-color-clock (one PF column = 4 CLK) boundaries.
- For taller PF regions, the RAM and shift loop must be extended proportionally.

### TIA Studio template parameters
- `direction` (left | right)
- `speed` (1 = every frame, N = every N frames; integer)
- `height_rows` (number of 4-row groups, each 3 bytes in RAM)
- `initial_pattern` (the starting 3-register value per row)

### Relation to design-principles
**New for design-principles**: PF horizontal scroll granularity is 4 color clocks (one PF column); there is
no sub-column PF scroll. All PF movement snaps to 4-CLK grid. This is distinct from sprite HMOVE (1-CLK
granularity). Document explicitly.

---

## 6. Independent Dual-Side Vertical PF Scroll (Asymmetric)

### What it does
Scrolls the left and right PF halves independently at different speeds and directions within the same
asymmetric kernel. `Asym2scrol.asm` maintains two sets of RAM pattern registers (PF0Left/PF1Left/PF2Left
for the left side, PF0ADR/PF1ADR/PF2ADR for the right side). Each frame, a per-side countdown compares
against a speed constant; when it expires, the pattern byte is rotated (via the 80x86 ROL emulation
sequence: ASL/BCC+ORA for unsigned left-rotate or LSR/BCC+ORA for right-rotate). The kernel then writes
left-half values, waits (SLEEP 4), writes right-half values, per scanline.

### Cycle budget
VBLANK computation: two sets of pattern rotation + frame counter logic ≈ 80–120 cycles, well within 2785
free cycles. Display kernel: same as full asymmetric PF (left write ~21 cy, right write ending at cy47,
29 cy free). The demo uses `SLEEP 4` macros (8 NOP cycles) between register writes, which fits within the
asymmetric window.

### Byte budget
No ROM tables needed for pure pattern scroll; the content is generated from a single seed byte per register
by the rotate chain. RAM: approximately 12 bytes (3 per side × 2 sides + 2 frame counters + 2 ghost
temporaries = ~10 bytes). Very RAM-efficient.

### Constraints and failure modes
- The 6502 has no native ROL-without-carry instruction. The 80x86-style emulation (ASL; BCC skip; ORA #1)
  adds approximately 5–7 cycles per rotate. For three registers per side this is ~18–21 extra cycles in
  VBLANK, still affordable.
- The SLEEP 4 delay between left and right half writes is not cycle-counted from WSYNC; it is counted from
  the end of the left write sequence. This works because the left-half writes always take the same number
  of cycles (no branching). Any conditional logic inserted between left and right writes would break timing.
- The two sides scroll independently: left uses DEC/INC on RAM bytes (simple increment/decrement), right
  uses bit-rotate. Mixing strategies shows that any repeating function on a byte value is valid as a
  "scroll" — the visual result depends on the function, not on a physical displacement.

### TIA Studio template parameters
- `left_scroll_speed` (1 = fastest, N = every N frames)
- `right_scroll_speed` (same scale)
- `left_direction` (increment | decrement)
- `right_direction` (rotate-left | rotate-right | increment | decrement)
- `left_seed` (3-byte initial pattern: PF0, PF1, PF2)
- `right_seed` (same)
- `dual_color` (boolean — use CTRLPF D1 score bit to give left/right distinct colours via COLUP0/COLUP1)

### Relation to design-principles
**New for design-principles**: CTRLPF D1 (score bit, bit 1) gives the left PF half the color COLUP0 and
the right half COLUP1, independent of player position. This is a cheap two-color PF hack available even
in symmetric mode. It requires CTRLPF = $02 (or $03 for reflect+score). `Asym2scrol.asm` uses this explicitly.

---

## 7. 48-Pixel Wide Sprite Primitive

### What it does
Constructs an apparent 48-pixel wide sprite using two hardware players (P0 and P1), both set to
NUSIZ = $03 (three close copies), with VDELP0 and VDELP1 enabled. P1 is positioned 8 pixels to the right
of P0. The six GRP registers (GRP0, GRP1, via the VDEL pipeline) output six 8-bit columns across the
scan, producing a combined 48-pixel bitmap when all copies align.

`bigspritedemo.asm` / `bigmove.asm` show the full technique. The inner kernel loop loads bitmap data via
six zero-page indirect pointers (s1–s6), using the GRP0/GRP1 pipeline sequence:
LDA (s1),Y → STA GRP0; LDA (s2),Y → STA GRP1; LDA (s3),Y → STA GRP0;
LDA (s6),Y → Temp; LDA (s5),Y → TAX; LDA (s4),Y → LDY Temp;
STA GRP1; STX GRP0; STY GRP1; STA GRP0.

### Cycle budget
The 10-instruction GRP pipeline (per scanline, no WSYNC): approximately 40 cycles. This is consistent with
existing design-principles entry for bitmap48. No WSYNC inside the bitmap rows — the loop runs free for N
rows, then WSYNC to advance to the next "character row" boundary. The demos use a `DelayPTR` jump table
to achieve sub-3-cycle horizontal position granularity for the coarse positioning NOP sled.

### Byte budget
Six data tables × 10 bytes each (for a 10-row sprite) = 60 bytes of ROM per sprite frame. For animation,
multiply by the number of animation frames.

### Constraints and failure modes
- VDEL is mandatory. Without VDELP0/VDELP1, the GRP0/GRP1 update sequence produces a one-row vertical
  shift artifact because GRP0's write clobbers the currently-displayed GRP1 (they share the delayed
  register pipeline).
- The GRP write order is GRP0, GRP1, GRP0, then the three-way sequence GRP1/GRP0/GRP1/GRP0 for the last
  four columns. Reversing any write causes the wrong bitmap data to appear in the wrong column.
- NOP-sled positioning: horizontal placement is coarse (RESP0/RESP1 strobe at a fixed offset) + fine
  (HMOVE nibble). The `bigspritedemo.asm` demo pre-positions on a calibration NOP sled and uses a
  pointer-jump (`JMP (DelayPTR)`) to enter the sled at different offsets, effectively shifting the sprite
  right by 1–2 extra NOP cycles (3px each) before the RESP strobes fire. Each 1-NOP step = 3px shift.

### TIA Studio template parameters
- `width` (always 48 for this primitive; variant: 24 = NUSIZ$03 single player, or 16 = one 2x copy)
- `height` (bitmap rows, 1–N)
- `color` (COLUP0 = COLUP1 for monochrome; independent for two-color 48px)
- `x_pos` (coarse via NOP sled entry point; fine via HMP0/HMP1)
- `y_pos` (top scanline; controlled by TopDelay countdown in the demo)
- `bitmap_tables` (six pointers to per-column data)

### Relation to design-principles
Reinforces existing: "48px = NUSIZ$03 (3 copies) + P1 を 8px 右 + VDEL 二重バッファ". **New detail**:
the NOP-sled jump table technique for sub-3-cycle horizontal granularity beyond what HMOVE provides. This
is distinct from the standard coarse ÷15 + HMOVE approach and should be noted as an alternative for
large-sprite positioning.

---

## 8. Missile as Decorative Vertical Line / Indicator

### What it does
Uses a missile (M0 or M1) as a thin 1- or 2-pixel vertical element at a fixed horizontal position.
`movingmissile.asm` demonstrates joystick-driven missile movement: HMM0 is set to move left or right
($F0 = left 1, $10 = right 1), then a WSYNC + HMOVE fires the move. Vertical position is controlled by
counting scanlines (a RAM byte holds the target Y, incremented/decremented by joystick up/down); ENAM0
is set to $02 on the target scanline and cleared after a fixed number of rows (missile height in lines).

The SWCHA bit-unpacking pattern (ROL accumulator + BMI per bit) is a reusable joystick-read idiom.

### Cycle budget
ENAM0 enable/disable adds 3 cycles per scanline during the missile-visible region. Joystick read (in
VBLANK) costs approximately 30–40 cycles per player. HMOVE during VBLANK costs 3 (STA WSYNC) + 3 (STA HMOVE).

### Constraints and failure modes
- HMOVE must fire immediately after WSYNC (the demo comment: "DON'T FORGET TO HIT HMOVE or the object
  won't move"). Firing HMOVE mid-line instead of post-WSYNC moves the missile the wrong direction and
  magnitude (the mid-line HMOVE side-effect documented in design-principles: moves right ~1px/4CLK).
- HMxx registers must not be written within 24 CPU cycles after HMOVE or the motion is unpredictable.
  The demo handles this by not writing HMM0 within the visible kernel.
- The missile is 1 pixel wide (or 2/4/8 per NUSIZ M bits). A 2-scanline missile is toggled ENAM0 on
  for 2 WSYNC loops, then off.

### TIA Studio template parameters
- `missile_index` (0 | 1)
- `x_pos` (HMM value; coarse via RESMx position)
- `y_start`, `y_height` (scanline range; controlled by ENAM0 on/off in kernel)
- `width_px` (1 | 2 | 4 | 8 via NUSIZ missile bits)

### Relation to design-principles
Reinforces existing: "missile/ball = 線・縁・縦枠". No new principle.

---

## 9. Score / Digit Display Primitives

### 9a. 6-Digit Score (P0+P1, NUSIZ $06, VDEL, SP-as-Temp)

**What it does**: Displays six decimal or hex digits across the top of the screen using P0 and P1 each
set to NUSIZ = $06 (two medium copies, 2× wide). P0 shows three digits (left player), P1 shows three
(right player). VDELP0 and VDELP1 are enabled. During the score zone, the kernel sequences GRP writes
to emit each digit column using the VDEL pipeline.

`resources/score3x2.asm` is the canonical implementation. Key tricks:
- **SP as temporary register**: the stack pointer is repurposed to hold one of the six score pointers
  (`tsx`/`txs`) during the per-scanline loop, since all registers (A, X, Y) are consumed by the six
  `lda (ptr),y` + `sta GRP*` sequences. SP is restored from RAM after the score zone.
- **Overscan pointer setup**: score byte-to-digit-pointer conversion (BCD nibble extraction, multiply by
  bytes-per-glyph) runs during overscan, so the display kernel reads pre-computed pointers with no math.
- **COLUPF mid-loop**: colour is written inside the `.loopScore` per-scanline loop at precise cycle
  offsets (annotated @07, @15, @23 etc.) to produce two-tone digit colouring without extra scanlines.

**Cycle budget**: the inner loop body from `score3x2.asm` is tightly annotated. The GRP sequence for
three digit pairs costs approximately 60 cycles per scanline row, leaving 16 cycles for housekeeping
(DEY, BPL, PF border writes). The SP-as-temp technique saves 2–3 cycles vs. an extra RAM push/pop.

**Glyph table**: each digit 0–9 is 8 bytes tall (one byte per scanline row, 8-pixel wide). The table
is placed at a 256-byte page boundary (`align 256`). Zero (the sentinel for "blank") is first so that
an uninitialized pointer defaults to blank, not garbage.

### 9b. Compact Digit Table (8 bytes per digit)

**What it does**: `original/CaveIn/score_graphics.asm` provides an alternative digit ROM table (10 digits
× 8 bytes = 80 bytes) designed to be placed at $FF9C (just before the interrupt vectors). This is the
smallest possible digit encoding for a one-player 8-pixel-wide font.

**Byte budget**: 80 bytes ROM. No RAM overhead at display time (index lookup only).

### 9c. 13-Char Text Renderer (SP-as-Ring-Buffer)

**What it does**: `resources/13_plus2.asm` by Paul Slocum renders scrolling 5×5 pixel text using both
players (NUSIZ$03 three copies + NUSIZ$04 quad) plus missiles and ball as extra pixel columns. The
trick is using the hardware stack pointer as a ring-buffer pointer into RAM: the font data for each
character pair is PHA-pushed into RAM during VBLANK, then the kernel reads them out via SP. This
compresses what would otherwise be complex per-column GRP sequencing into a push/pop pattern.

**Cycle budget**: VBLANK text assembly (loading font, pushing to stack): approximately one-third of
available VBLANK cycles for a 13-character line. Display kernel: approximately 45 cycles per row.

**Constraints**: The stack cannot be used for subroutine calls while SP is repurposed as a buffer
pointer. The demo saves SP to RAM before the kernel (`stx savesp`) and restores it afterward. Any
interrupt or call that corrupts the stack during the kernel will corrupt the display.

### TIA Studio template parameters (9a)
- `num_digits` (2 | 4 | 6; determines NUSIZ and pointer count)
- `glyph_height` (rows, typically 5 or 8)
- `color` (or per-digit COLUPF cycling as in `score3x2.asm`)
- `bcd_values` (array of digit values 0–9)
- `y_zone` (which scanline band; the rest of the frame must not conflict with SP use)

### Relation to design-principles
The SP-as-temp-register technique is not documented in design-principles. **New principle candidate**:
"SP はゼロページレジスタとして転用できる（カーネル中のサブルーチン呼出しを禁止し、RAM にバックアップして
前後で復元する）。6-digit スコアカーネルで A/X/Y が全部埋まる状況での標準的な節約手法。"

---

## 10. Sub-Pixel Horizontal Positioning via NOP Sled

### What it does
`bigmove.asm` and `bigspritedemo.asm` use a pre-computed table of `CMP` opcodes (`$C9` = 2-cycle
immediate CMP) as a NOP-equivalent sled. A pointer (`DelayPTR`) points into this sled; `JMP (DelayPTR)`
enters at different offsets, consuming different amounts of time before the RESP0/RESP1 strobes fire.
Moving the pointer left by 1 entry = 2 fewer cycles = sprite shifts left by 6 pixels (2 cycles × 3
px/cycle). Moving right by 1 entry shifts right by 6 pixels.

HMOVE fine-tunes within the 6-pixel coarse steps. Combined, this achieves pixel-level control beyond
the 3-pixel HMOVE granularity limit.

### Cycle budget
One CMP #imm = 2 cycles = 6 pixels at 3px/cycle. Fine HMOVE granularity = 1 pixel per HMOVE unit.
The combined system can reach any horizontal position with 1-pixel precision.

### Constraints and failure modes
- The sled must be aligned on a page boundary or the pointer arithmetic breaks.
- When the pointer reaches either end of the sled, the code switches to the next coarse position
  (RESP0/RESP1 fired one cycle earlier or later), requiring synchronization of the MoveCount variable.
  The demo tracks this with a 3-frame `MoveCount` modulo counter.
- This technique consumes ROM (the sled is approximately 37 bytes in the demo) and RAM (the pointer).

### TIA Studio template parameters
- This is a sub-feature of sprite positioning, not a standalone template. The TIA Studio positioning
  system should abstract it behind an `x_pos` parameter and emit the appropriate sled automatically.

### Relation to design-principles
**New for design-principles**: Horizontal position beyond HMOVE granularity is achievable via NOP/CMP
sled entry-point variation. Standard approach (÷15 coarse + HMOVE fine) gives 3-pixel granularity;
sled approach + HMOVE gives 1-pixel granularity at the cost of ~40 ROM bytes and a RAM pointer.

---

## Summary: Principles to Merge into design-principles.md

The following are genuinely new observations (not yet covered by existing entries) that should be
merged into `harness/docs/design-principles.md`:

1. **CTRLPF D1 (score bit) = 自由な 2 色 PF**
   Setting CTRLPF bit 1 makes the left PF half use COLUP0 and the right half use COLUP1, regardless of
   player positions. This gives two independently controlled PF colors without asymmetric write timing.
   Available in both symmetric and reflect modes. (`Asym2scrol.asm` line 91: `LDA #%00000010 / STA CTRLPF`)

2. **PF 水平スクロールは 4 color-clock (1 PF column) 刻みが最小単位**
   PF horizontal scroll is achieved by bit-rotating RAM copies of the PF registers each frame; the
   minimum visible step is 4 color clocks (one PF column width). There is no sub-column PF scroll.
   Contrast with sprite HMOVE (1px = 1 color-clock granularity). Design choices that need smooth
   horizontal movement must use sprites, not PF.

3. **SP-as-temp-register は 6-digit スコアカーネルの標準手法**
   When A, X, Y are all consumed by GRP double-buffer sequences, the stack pointer SP can be repurposed
   as a seventh working register. Prerequisite: no subroutine calls during the kernel, SP backed up to
   RAM before entry and restored after exit. This is established practice in score kernels (see
   `score3x2.asm` and the `colors.asm` main kernel which also uses `txs` for the SP-as-counter trick).

---

## Residual Uncertainty

**Asymmetric PF right-half timing in SLEEP-macro implementations (Bogey demos) vs. cycle-counted
implementations (Williams/Saunders)**: The Bogey demos (`AsymPlay.asm`, `Asym2scrol.asm`) use a SLEEP
macro between left and right writes, not explicit cycle counts. The `SLEEP 4` between each write pair
is approximately correct for the right-half deadline, but the precise cycle offset from WSYNC depends on
how many cycles the left-half sequence consumed before the first SLEEP. This has not been verified against
`assert_line_budget` in the harness. **Risk**: the SLEEP-macro demos may be slightly off-deadline on the
hardware or differ by one cycle from the Williams/Saunders reference implementation. Before using Bogey's
asymmetric kernel as a TIA Studio template, its cycle counts should be verified with `read_cycles` or
`assert_line_budget` against a known-good reference.
