; litmus_jsr_display — twin subroutines for the "does this call touch the
; display?" rule that decides whether a region is classified blank or visible.
;
; The rule keeps VSYNC/VBLANK alive across a JSR when the callee provably cannot
; write them. Getting that wrong in the permissive direction is the dangerous
; one: a region that really does turn the display on would be classified blank
; and skipped by the budget proof. So this ROM contains BOTH shapes, differing
; only in the base address of one indexed store:
;
;   SafeCall  — `sta COLUP0,x` with x in {0,1}: reaches $06/$07. Cannot be the
;               display, and the prover must be able to see that (otherwise the
;               ordinary shared positioning routine stays unprovable).
;   PeriCall  — `sta VSYNC,x`  with x in {0,1}: reaches $00/$01 = VSYNC/VBLANK.
;               The prover must NOT keep the display bits across this one.
;   DirectCall— `sta VBLANK` outright, no index. The plain case, so the rule
;               cannot be satisfied by only reasoning about indexed stores.
;
; Both are called from the same place with the same index values, so a rule that
; answers the same way for both is wrong whichever answer it gives.
;
; The ROM itself just runs a stable 262-line frame; what it proves is read out of
; the static analysis, not out of the picture.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUP0 = $06
COLUBK = $09

    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clear:
    sta $00,x
    dex
    bne Clear

Frame:
    lda #2
    sta VSYNC
    sta WSYNC
    sta WSYNC
    sta WSYNC
    lda #0
    sta VSYNC
    lda #2
    sta VBLANK

    ; Two calls to the SAFE routine — the shape a two-sprite kernel uses to
    ; place P0 and P1 through one shared routine.
    ldx #0
    lda #$44
    jsr SafeCall
    ldx #1
    lda #$C6
    jsr SafeCall

    ; One call to the PERILOUS routine, whose indexed store lands on VSYNC/VBLANK.
    ldx #1
    lda #0
    jsr PeriCall

    lda #2
    jsr DirectCall

    ldx #37
VB:
    sta WSYNC
    dex
    bne VB

    lda #0
    sta VBLANK
    lda #$92
    sta COLUBK
    ldx #192
Vis:
    sta WSYNC
    dex
    bne Vis

    lda #2
    sta VBLANK
    ldx #30
OS:
    sta WSYNC
    dex
    bne OS
    jmp Frame

; SafeCall: one WSYNC then an indexed store that cannot reach $00/$01.
SafeCall:
    sta WSYNC
    sta COLUP0,x
    rts

; PeriCall: identical shape, but the indexed store CAN reach VSYNC/VBLANK.
PeriCall:
    sta WSYNC
    sta VSYNC,x
    rts

; DirectCall: the plain, non-indexed display write.
DirectCall:
    sta WSYNC
    sta VBLANK
    rts

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
