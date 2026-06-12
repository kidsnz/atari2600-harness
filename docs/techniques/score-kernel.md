# Technique — 6-digit score kernel (48px, BCD)

**Goal:** the standard score display every real game needs: a 6-digit decimal score rendered with
the two players in 3-copies-close mode + VDEL 6-store choreography (48px), updated from BCD bytes.

Demo: `roms/techniques/score6.asm` (score auto-increments each frame).
CI: `scenarios/score6.json` (positions, BCD carry chain at frames 99/150, 262 lines, golden).
Foundation: `litmus_48px6` (hardware-verified 6-store choreography, v0.52.0) + `litmus_6502`
(NMOS BCD behavior, v0.44.0).

## The technique

**Score state = 3 BCD bytes** (`score0` = high pair … `score2` = low pair). Adding is a normal
`SED` carry chain (`ADC #1` / `ADC #0` ×2 / `CLD`) — verified NMOS rule: only C is valid, and
`CLD` is mandatory.

**VBLANK: build 6 font pointers.** Each digit's glyph lives at `Font + digit*8`:

```
lda score0 / and #$F0 / lsr        ; hi nibble<<4 → >>1 = digit*8
sta p0
lda score0 / and #$0F / asl ×3     ; lo nibble*8
sta p1                              ; … same for score1→p2,p3 / score2→p4,p5
```

Pointer high bytes are set once at init (font fits in one page → no page-cross penalty, so
`lda (p),y` is a fixed 5 cycles — store timing stays deterministic).

**Kernel row (8 lines, Y=7→0):** the litmus_48px6 choreography with `(zp),y` fetches:

```
Krow:   sta WSYNC
        ldy row        ; 3
        lda (p0),y     ; 8     sta GRP0  ; 11   B0
        lda (p1),y     ; 16    sta GRP1  ; 19   B1
        lda (p2),y     ; 24    sta GRP0  ; 27   B2
        lda (p3),y     ; 32    sta tmp   ; 35
        lda (p4),y     ; 40    tax       ; 42
        lda (p5),y     ; 47    tay       ; 49
        lda tmp        ; 52
        sta GRP1 ; 55   stx GRP0 ; 58   sty GRP1 ; 61   sta GRP0 ; 64 (junk)
        dec row        ; 69
        bpl Krow       ; 72  (< 76 — fits in one line)
```

**Position follows the store times.** The 4-burst completes at 55/58/61/64 cy = **+21 cy** vs
litmus_48px6's 34/37/40/43, so the whole sprite block shifts **+63 px**: position P0=87, P1=95
(prologue = litmus recipe + SLEEP 21). The gap relations between copies are preserved exactly
because everything moves together. Verified: `read_tia` hmoved_pixel 87/95, digits render
byte-exact ("000004" readable on the annotated screen, `read_row` shows the 2px-pair pattern of
the `$CC` glyph rows at the expected clocks).

**Font:** 6px glyphs + 2 blank right columns (copies abut at 8px pitch, so inter-digit spacing
is built into the font). Stored bottom-row-first because the kernel walks Y=7→0.
Reusable from Go: `pkg/sprite.DigitFont()` (top-down order; reverse when emitting for this kernel).

## Notes / variants
- Score color: set `COLUP0/COLUP1` before the rows (one color for all 6 digits), or stage
  per-frame for flash effects.
- For score + lives/level on one line, SCORE mode (CTRLPF D1, verified litmus_ctrlpf) colors the
  PF halves differently — independent of this sprite-based kernel.
- Row height ×2: replace `dec row/bpl` with a 2-line repeat (budget allows: 72 cy used).
