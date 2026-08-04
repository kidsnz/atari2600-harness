; litmus_nusiz_shape — ONE player, reshaped every scanline, covering an outline no
; 8-pixel graphic can hold.
;
; WHAT THE FIXTURE IS FOR. `litmus_nusiz_all` already runs all eight NUSIZ modes, but
; it sets each mode ONCE and holds it for a 24-line band, and it never moves the
; object. That answers "does mode n draw n copies", which is a fact about the mode.
; It does not answer the question the Fishing Derby casebook asked
; (`docs/casebook.md`, capability gap G9(a) in `docs/capability-gap-audit.md`): can a
; SINGLE player be made to cover an irregular silhouette WIDER than eight pixels, by
; rewriting NUSIZ and strobing HMOVE on every line of the kernel? That is a fact
; about the two registers TOGETHER, per line, and nothing in this repo measured it.
;
; GRP0 IS $FF ON EVERY LINE OF THE SHAPE. Deliberately: if the graphic varied, the
; outline could be credited to the graphic. Here the bitmap is a constant 8 solid
; bits and the whole silhouette — its width per line AND its left edge per line —
; comes from NUSIZ0 and HMP0 alone. A real sprite paints art on top; this fixture
; removes the art so the mechanism is the only thing left to explain the shape.
;
; THE INTENDED OUTLINE (this is the claim; the test grades the drawn pixels against
; it, not against whatever the harness happened to produce). Eight bands of five
; scanlines. Two hardware rules and one anchor generate every row of it:
;
;   ANCHOR   X0 = 42. The positioning kernel below is the instruction sequence of
;            `roms/litmus/litmus_pos.asm` with delay unit 4, whose sweep
;            TestCoarseAdjustSweepIsWhatTheDocSays pins at HmovedPixel 42, and a
;            plain player inks its first clock exactly there (measured on that same
;            ROM: ObjectX 42, run [42,50)).
;   RULE 1   NUSIZ copies sit 16 / 32 / 64 clocks apart for close / medium / wide,
;            and double and quad width are 16 and 32 clocks of solid ink.
;   RULE 2   double and quad width START ONE CLOCK LATER than the 1x modes. Measured
;            on `roms/litmus/litmus_nusiz_all.asm`, which never moves anything: modes
;            0-4 and 6 ink from clock 24, modes 5 and 7 from clock 25. This fixture
;            did not discover that rule and does not get to choose it — it is in the
;            table below because the shape would otherwise be predicted 1px wrong on
;            25 of its 40 lines.
;
;   band  NUSIZ0  mode              HM at band top  cum.  size  drawn runs (absolute)  ink
;    0     $01    two copies close   $00  (0)         +0    +0   [42,50) [58,66)        16
;    1     $01    two copies close   $C0  (right 4)   +4    +0   [46,54) [62,70)        16
;    2     $07    quad width         $C0  (right 4)   +8    +1   [51,83)                32
;    3     $07    quad width         $C0  (right 4)  +12    +1   [55,87)                32
;    4     $07    quad width         $C0  (right 4)  +16    +1   [59,91)                32
;    5     $05    double width       $80  (right 8)  +24    +1   [67,83)                16
;    6     $05    double width       $C0  (right 4)  +28    +1   [71,87)                16
;    7     $00    one copy           $80  (right 8)  +36    +0   [78,86)                 8
;
; Read top to bottom it is a fish: a forked tail (two separate 8px runs), a 32px body
; that walks right as it descends, a taper, and an 8px snout. Total extent 49 clocks
; out of one player. The left edge moves 36 clocks over the 40 lines, so no static
; placement reproduces it either.
;
; THE ROWS AND THE CONTROLS. The same 40-line kernel runs FOUR times, 48 lines per
; block (1 housekeeping line + 1 positioning line + 40 shape lines + 6 blank
; separator lines). Only two zero-page masks differ, so the four blocks are the same
; instructions over the same tables and the only variable is which of the two
; registers is allowed through:
;
;   block 0  SHAPED     nusiz mask $FF, hmove mask $FF   the row the technique is for.
;   block 1  NUSIZ ONLY nusiz mask $FF, hmove mask $00   every band keeps its width and
;              every band's left edge must be X0. This is the WIDTH oracle for block 0.
;   block 2  HMOVE ONLY nusiz mask $00, hmove mask $FF   every band is 8px wide and only
;              the left edge steps. This is the LEFT-EDGE oracle for block 0, and it is
;              what the per-line HMOVE contributes on its own.
;   block 3  FLAT       both masks $00                   40 identical 8px rows at X0.
;              This is what one player gives you for free, and the whole point of the
;              fixture is that block 0 does not look like it. It is also the fixture's
;              sharpest control, because it is the one that says HMOVE with HM=$00
;              COSTS NOTHING: during development this block drifted +1 clock per line
;              (39 over 40 lines) purely because `sta HMOVE` sat at CPU cycle 10 of the
;              line instead of cycle 0. Every slope in blocks 0 and 2 was wrong by the
;              same amount and every band still looked plausible. Nothing but a
;              deliberately motionless row catches that.
;
; The decomposition is what makes the grading non-circular twice over: the test checks
; block 0 against the table above, AND checks that block 0's runs equal block 1's runs
; translated by block 2's left edge. A wrong table would have to be wrong in exactly
; the same way in three independently-driven blocks.
;
; HMOVE IS THE FIRST INSTRUCTION AFTER WSYNC, on every one of the 40 shape lines,
; including the four lines per band whose move is zero — so the 8-clock HMOVE comb is
; uniform across all 40 rows rather than ragged, and (see block 3) so the move is the
; nibble and nothing else. The consequence is that HMP0 for line k+1 has to be
; prepared on line k, after the 24-cycle window in which an HMxx write following an
; HMOVE strobe is a known hazard (`roms/litmus/lint_r3_hazard.asm`); the three `nop`s
; below are that spacing, measured, not decorative.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
CTRLPF  = $0A
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

posDelay = $80          ; ÷15 coarse unit; 4 = HmovedPixel 42 = first inked clock 42
nMask    = $81          ; $FF lets NUSIZ0 through, $00 forces one plain copy
hMask    = $82          ; $FF lets HMP0 through, $00 forces a zero move
block    = $83

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #4
        sta posDelay
        lda #$0E
        sta COLUP0              ; white — the only non-background colour on screen
        lda #0
        sta COLUBK
        sta CTRLPF

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible: 4 blocks x 48 lines = 192 ---
        lda #0
        sta block

BlockTop:
        ldx block
        lda MaskN,x
        sta nMask
        lda MaskH,x
        sta hMask
        sta WSYNC               ; ends the housekeeping line

        ; --- positioning line: the litmus_pos ÷15 kernel, delay unit 4.
        ;     GRP0 is armed at cycle 49 (colour clock 147), long after the beam has
        ;     passed clock 42, so this line itself stays blank. ---
        sta HMCLR
        lda posDelay
        sec
PD:     sbc #1
        bcs PD
        sta RESP0
        ldy #0
        lda HmTab,y             ; prime the first HMOVE with band 0's move ($00)
        and hMask
        sta HMP0
        lda #$FF
        sta GRP0                ; eight solid bits, and they never change again
        lda #0
        sta NUSIZ0

        ; --- 40 shape lines ---
SL:     sta WSYNC
        sta HMOVE               ; 0-2    prepared on the previous line
        lda NusizTab,y          ; 3-6
        and nMask               ; 7-9
        sta NUSIZ0              ; 10-12  colour clock 30, ahead of the beam at 42
        iny                     ; 13-14
        lda HmTab,y             ; 15-18  the NEXT line's move
        and hMask               ; 19-21
        nop                     ; 22-23  \
        nop                     ; 24-25   > clear of the 24-cycle post-HMOVE hazard
        nop                     ; 26-27  /
        sta HMP0                ; 28-30
        cpy #40                 ; 31-32
        bne SL                  ; 33-35

        ; --- 6 blank separator lines: nothing is drawn, so a mis-indexed row shows
        ;     up as ink where the fixture promises none. ---
        sta WSYNC
        lda #0
        sta GRP0
        sta NUSIZ0
        sta HMP0
        ldx #6
SEP:    sta WSYNC
        dex
        bne SEP

        inc block
        lda block
        cmp #4
        bne BlockTop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

; block -> which register is allowed through
MaskN:  .byte $FF, $FF, $00, $00
MaskH:  .byte $FF, $00, $FF, $00

        org $FE00               ; page-aligned: no indexed page-cross wobble

; NUSIZ0 per shape line (5 lines per band).
NusizTab:
        .byte $01,$01,$01,$01,$01       ; band 0 — two copies close (forked tail)
        .byte $01,$01,$01,$01,$01       ; band 1
        .byte $07,$07,$07,$07,$07       ; band 2 — quad width (body)
        .byte $07,$07,$07,$07,$07       ; band 3
        .byte $07,$07,$07,$07,$07       ; band 4
        .byte $05,$05,$05,$05,$05       ; band 5 — double width (taper)
        .byte $05,$05,$05,$05,$05       ; band 6
        .byte $00,$00,$00,$00,$00       ; band 7 — one copy (snout)
        .byte $00                       ; index 40: read once, after the last line

; HMP0 per shape line. The move lands on the band's FIRST line and is zero for the
; other four, so all five lines of a band share one left edge.
;   $C0 = right 4, $80 = right 8, $00 = no move.
HmTab:
        .byte $00,$00,$00,$00,$00       ;  +0
        .byte $C0,$00,$00,$00,$00       ;  +4
        .byte $C0,$00,$00,$00,$00       ;  +8
        .byte $C0,$00,$00,$00,$00       ; +12
        .byte $C0,$00,$00,$00,$00       ; +16
        .byte $80,$00,$00,$00,$00       ; +24
        .byte $C0,$00,$00,$00,$00       ; +28
        .byte $80,$00,$00,$00,$00       ; +36
        .byte $00                       ; index 40: read once, after the last line

        org $FFFC
        .word Reset
        .word Reset
