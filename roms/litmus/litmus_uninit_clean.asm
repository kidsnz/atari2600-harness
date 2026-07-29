; litmus_uninit_clean.asm — the same ROM with the clear done properly.
;
; The negative half of the pair. Byte for byte the same as litmus_uninit_read.asm
; except that the sweep clears the whole zero page, so the `lda $8A` reads a cell
; the ROM itself wrote.
;
; It exists because a detector is only worth its reports if it stays SILENT here.
; Anything that fires on both members of the pair is reacting to the shape of the
; code rather than to whether the cell was initialised.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs

        ; Clear the whole zero page, so $8A is written before it is read.
        lda #0
        ldx #$FF
Clr:    sta $00,x
        dex
        bne Clr

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

        ; $8A was cleared above, so this read is initialised.
        lda $8A
        sta COLUBK

        ldx #36
vb:     sta WSYNC
        dex
        bne vb

        lda #0
        sta VBLANK
        ldx #192
vis:    sta WSYNC
        dex
        bne vis

        lda #2
        sta VBLANK
        ldx #30
os:     sta WSYNC
        dex
        bne os

        jmp Frame

        org $FFFA
        .word Reset
        .word Reset
        .word Reset
