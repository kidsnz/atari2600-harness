# Technique — 6-digit score kernel (48px, BCD)

> **Placement: see `sprite-placement.md`'s rule table before trusting where anything lands.**
> Rule 12 in particular — a copy past clock 160 wraps to the left edge and draws there on the
> same line — applies here because its fixture writes `NUSIZ = $03`, so the rightmost copy sits at base+32 and a base past ~128 wraps. That rule was measured and CI-locked on 2026-08-21 and
> re-derived from scratch anyway on 2026-09-03, which is why these pointers exist.

**Goal:** the standard score display every real game needs: a 6-digit decimal score rendered with
the two players in 3-copies-close mode + VDEL 6-store choreography (48px), updated from BCD bytes.

**It takes both players for the lines it occupies, and the consequence is visible.** Andrew Davie,
stella-list (2001), on his own game: *"when you jump at the top of the screen, **your sprite
disappears**, as sprites are used for the high-score routine."* Nothing playable can enter the score
band — not the avatar, not a projectile, not an enemy — because there are no player objects left
there. The playfield and the ball are still yours; the players are not.

The design answer is a rule someone else on the same list had already written down. Erik Mooney,
`role-playing-game-development` (1999): **put status display in a band at the top or the bottom**,
where the play area does not reach. That is not decoration, it is what makes the constraint above
invisible to the player — and it is why almost every 2600 game looks the way it does. If the score
must float over the play area instead, budget a different mechanism (playfield digits at 4-clock
grain, or a sprite the score borrows only on lines the player cannot reach).

Demo: `roms/techniques/score6.asm` (score auto-increments each frame).
CI: `scenarios/score6.json` (positions, BCD carry chain at frames 99/150, 262 lines, golden).
Foundation: `litmus_48px6` (hardware-verified 6-store choreography, v0.52.0) + `litmus_6502`
(NMOS BCD behavior, v0.44.0).

## The technique

**Score state = 3 BCD bytes** (`score0` = high pair … `score2` = low pair). Adding is a normal
`SED` carry chain (`ADC #1` / `ADC #0` ×2 / `CLD`) — verified NMOS rule: only C is valid, and
`CLD` is mandatory.

**When a BCD byte must be a binary quantity** (an index, a speed — comparing does not need it, see
`design-principles.md` on `cmp`): `$XY` is 16X+Y, so subtract 6X. No branches and no indexed reads, so
constant time; 15 bytes with the `RTS` (omegamatrix's version, AtariAge `topic/294471`):

```
BCDtoBin: sta Temp / and #$F0 / lsr / sta Temp2   ; Temp2 = 8X
          lsr / lsr / adc Temp                    ; 2X + 16X+Y  (C is 0: only zeros shifted out)
          sec / sbc Temp2 / rts                   ; − 8X = 10X+Y
```

Measured in the engine 2026-09-30 with a throwaway ROM (not in the repository): 100/100 inputs
correct; without the `SEC` 100/100 wrong; called with D set 44/100 wrong, so it runs after `CLD`.
The thread's shorter `ASR` form uses the unstable illegal opcode in `known-traps.md`.

**Or hold the six glyph pointers instead of the digits** (Thomas Jentzsch 〔stella-list
`200308/msg00064`〕; that River Raid uses it is Cited only, not verified): a point adds 8 (one
glyph) to the lowest pointer; at 80 (ten glyphs) it becomes 0 and 8 is added to the next. That drops
the 3 BCD bytes and the per-frame pointer build below; the price is that one `cmp` per BCD byte (two
digits) becomes one per digit — each pointer is digit×8, so order is kept. Which wins depends on the
game. Measured in a throwaway ROM: over 1,234 points all six
pointers stayed equal to a BCD reference's nibble×8; wrapping at 72 instead breaks it.

**VBLANK: build 6 font pointers.** Each digit's glyph lives at `Font + digit*8`:

```
lda score0 / and #$F0 / lsr        ; hi nibble<<4 → >>1 = digit*8
sta p0
lda score0 / and #$0F / asl ×3     ; lo nibble*8
sta p1                              ; … same for score1→p2,p3 / score2→p4,p5
```

The table need not hold only digits. A thread on keeping one digit fixed pairs a counting upper nibble
with a lower nibble that indexes a non-digit glyph — its example shows `$10` as "0%" (AtariAge
`topic/301365`; Cited only, not verified). Add `$10`, not 1, and while the lower nibble is 0–9 it never
moves; a lower nibble above 9 does not survive decimal-mode addition (`$1A` + `$10` gives `$30`, measured).

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
        sta GRP1 ; 55   stx GRP0 ; 58   sty GRP1 ; 61   sta GRP0 ; 64 (value unused, write required: it copies GRP1 into P1's delayed register)
        dec row        ; 69
        bpl Krow       ; 72  (< 76 — fits in one line)
```

★**There are 4 cycles left and they cannot buy a seventh store — measured 2026-09-07.**
`roms/litmus/litmus_store7_overrun.asm` is this kernel with one extra `lda (zp),y` + `sta GRPn`,
eight cycles the line does not have. What happens is worth knowing in both directions:

| | `prove_line_budget` | rendered frame | band width |
|---|---|---|---|
| six stores | Krow 93 / 152 — certified | 262 scanlines | 46 px |
| seven stores | Krow 102 / 152 — **certified** | **269 scanlines** | 46 px |

★★**The prover says yes and the machine says no.** The region begins with `sta WSYNC`, so it is given
a two-line budget of 152, and an eight-cycle overrun of the *inner* line disappears into that
allowance. That is not a bug in the budget — a two-line region is a real thing — but it means
**`prove_line_budget` cannot be the only check on a loop whose region starts with a WSYNC**.
`ntsc_frame_lines` / `frame_lines_stable` is what catches it. `internal/cyclebound/twolineregion_test.go`
holds this as a standing negative control.

★★★**And the seventh store buys nothing anyway: the band stays 46 px.** The "6" was never a cycle
budget — it is two players × three NUSIZ copies, and there is no seventh place to put a seventh image.
Question raised by the mailing-list distillation (helper-2), who asked where the six stores sit inside
the 76 cycles and whether the remainder admits a seventh.

**Position follows the store times.** The 4-burst completes at 55/58/61/64 cy = **+21 cy** vs
litmus_48px6's 34/37/40/43, so the whole sprite block shifts **+63 px**: position P0=87, P1=95
(prologue = litmus recipe + SLEEP 21). The gap relations between copies are preserved exactly
because everything moves together. Verified: `read_tia` hmoved_pixel 87/95, digits render
byte-exact ("000004" readable on the annotated screen, `read_row` shows the 2px-pair pattern of
the `$CC` glyph rows at the expected clocks).

**Font:** 6px glyphs + 2 blank right columns (copies abut at 8px pitch, so inter-digit spacing
is built into the font). Stored bottom-row-first because the kernel walks Y=7→0.
Reusable from Go: `pkg/sprite.DigitFont()` (top-down order; reverse when emitting for this kernel).

## When the score does not appear

Two forum checklists (azure, AtariAge `topic/290691`; nukey-shay, `topic/293942`) merged. Already on
this page: `NUSIZ = $03`, `(zp),y` fetches from a one-page font, HMOVE right after WSYNC (`CLAUDE.md`),
and position set by the store times — a position copied from another kernel (that thread's 56/64)
is wrong for this one (87/95). The rest:

1. **Variables not in RAM.** `ds` outside a `seg.u` at `org $80` takes the current ROM address and
   DASM says nothing; stores then do nothing (`known-traps.md`, *STA to ROM*). Measured 2026-09-30:
   `Score ds 3` after `org $F000` became `$F000` and read back 0 after `sta`; under `seg.u` it was
   `$80` and read back 1.
2. **Drawn during vertical blank** (code before the VBLANK wait). Measured in the engine 2026-09-30:
   this page's kernel run with VBLANK still set leaves the picture band uniform; as shipped, 164 lit
   pixels.
3. **Font in another bank** from the kernel: the same address there holds unrelated bytes, so glyphs
   come out as garbage. Cited only, not verified.
4. **Players repositioned for the play area before the band.** Each player has one position; move
   them after the score. Cited only, not verified.
5. **A font that hides the fault.** Fabrizio Zavagli 〔stella-list `200308/msg00058`〕: a score
   routine that borrows the stack pointer drew the first row of the first sprite wrong — *"Or was it
   the last row? Can't remember"* — unseen because the sample font left columns blank. Not verified.
   This page's font is 6 px wide, so test with bytes that light all 8 columns.

## Notes / variants
- **Without VDEL, nearly twice as wide** (spiceware, AtariAge `topic/215193`): both players `NUSIZ = $06`
  (three copies 32 px apart), P1 16 px right of P0, VDEL off. Each store gets its own gap, so each
  digit is loaded and stored in turn with no preloading, and the gap between digits frees all 8 font
  columns; the band is 88 px, not 48. Measured 2026-09-30 in a throwaway ROM (P0=33, P1=49,
  full-width bytes): read from six fixed tables (`lda Fk,y`), all 48 bytes are right only when the
  first load starts 15–18 cycles after WSYNC, because 7-cycle pairs drift against a 5⅓-cycle digit
  pitch; read through the score's pointers (`lda (p),y`), only a start at cycle 12. The thread's extensions — 12 digits by
  interlacing with a mid-kernel HMOVE (flickers), GRP0 and GRP1 halves in separate colours, three
  characters in GRP0+GRP1's 16 bits (Double Dunk) — Cited only, not verified.
- Score color: set `COLUP0/COLUP1` before the rows (one color for all 6 digits), or stage
  per-frame for flash effects.
- For score + lives/level on one line, SCORE mode (CTRLPF D1, verified litmus_ctrlpf) colors the
  PF halves differently — independent of this sprite-based kernel.
- Row height ×2: replace `dec row/bpl` with a 2-line repeat (budget allows: 72 cy used).
