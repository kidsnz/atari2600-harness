; hmove_clean — VV-10 T-2 CLEAN twin: the CORRECT HMOVE pattern. The motion
; registers (HMP0..) are written during VBLANK, then HMOVE is strobed right
; after a WSYNC, and no HMxx is written again within 24 CPU cycles of that
; HMOVE. WatchHMOVEHazard must report NO hit.

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
        lda #$10
        sta HMP0            ; set motion EARLY (in VBLANK), far from any HMOVE
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
        sta HMOVE           ; HMOVE right after WSYNC; no HMxx written after it
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
