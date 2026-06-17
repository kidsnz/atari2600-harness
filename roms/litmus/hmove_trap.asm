; hmove_trap — VV-10 T-2 PLANTED trap: writes a motion register WITHIN 24 CPU
; cycles of an HMOVE strobe (STA HMOVE immediately followed by STA HMP0, ~3
; cycles later). The Stella PG warns this gives unpredictable motion. The write
; happens long before 24 cycles elapse, so WatchHMOVEHazard must FLAG it.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
HMP0    = $20
HMOVE   = $2A

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

Main:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        lda #$1E
        sta COLUBK
        ldx #192
Vis:    sta WSYNC
        sta HMOVE           ; HMOVE strobe...
        lda #$10
        sta HMP0            ; ...HMxx written ~3 CPU cycles later = HAZARD
        dex
        bne Vis

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
