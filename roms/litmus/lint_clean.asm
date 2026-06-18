; lint_clean.asm — timinglint NEGATIVE self-test (all rules silent).
; The canonical correct idiom: stage fine motion, WSYNC, HMOVE, wait the full
; 24 cycles (ds 12,$EA), then HMCLR. timinglint must emit ZERO warnings:
;   R1 silent — HMxx IS written;  R2 silent — HMOVE IS strobed;
;   R3 silent — HMCLR starts at exactly 24cy after HMOVE (the safe boundary).

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
        ds 12,$EA           ; 12 NOPs = 24 CPU cycles (safe boundary)
        sta HMCLR           ; starts at 24cy => NOT a hazard
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
