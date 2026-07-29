; litmus_uninit_read.asm — a deliberate read of RAM nobody wrote.
;
; The bait for the uninitialised-read analysis. It clears only PART of the zero
; page and then reads a cell outside that range, so the value it paints with is
; whatever the hardware powered up holding. On an emulator that is a defined
; value and the ROM looks fine every single time; on a real console it is
; rubbish, and differently rubbish on each power-up.
;
; The partial clear is the point. A ROM with no clear at all would be caught by
; anything; catching this one requires knowing exactly WHICH cells the sweep
; loop reached — $01..$3F here, so $8A is untouched.
;
; Pairs with litmus_uninit_clean.asm, which does the same work after clearing
; everything. A detector that fires on both is not detecting anything.
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

        ; Clear only $01..$3F. $80-$FF is left exactly as the console powered up.
        lda #0
        ldx #$3F
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

        ; THE BAIT: $8A was never written by anything above.
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
