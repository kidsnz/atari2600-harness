; lint_r3_hazard.asm — timinglint R3 self-test (POSITIVE).
; Writes HMCLR only ~4 CPU cycles after HMOVE (two NOPs), inside the 24-cycle
; window where motion is undefined (Stella PG). timinglint must emit rule
; "hmove-hazard". R1/R2 stay silent (HMxx IS staged and HMOVE IS strobed); only
; the <24cy spacing is wrong. The clean sibling (HMCLR at exactly 24cy) is in
; lint_clean.asm and must NOT warn.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

        org $F000
Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clear:  sta $00,x
        dex
        bne Clear

Main:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VBlank: sta WSYNC
        dex
        bne VBlank
        sta VBLANK

        ldx #192
Visible:
        lda #$70
        sta HMP0            ; stage fine motion
        sta WSYNC
        sta HMOVE           ; apply
        nop                 ; 2
        nop                 ; 4
        sta HMCLR           ; <-- only ~4cy after HMOVE (<24) => hazard (R3)
        dex
        bne Visible

        lda #2
        sta VBLANK
        ldx #30
Over:   sta WSYNC
        dex
        bne Over
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
