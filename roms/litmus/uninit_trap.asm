; uninit_trap — VV-10 T-3 PLANTED trap: reads a RAM byte that was NEVER written
; since reset (no clear loop, $90 is untouched). On a real 2600 this is power-up
; garbage; in the emulator it reads a deterministic value and "works" — exactly
; the passes-in-emu / fails-on-HW bug WatchUninitRead must FLAG.

        processor 6502
VSYNC = $00
WSYNC = $02
COLUBK = $09

        org $F000
Reset:
        sei
        cld
        ldx #$FF
        txs
        ; NO RAM clear
        lda $90            ; READ uninitialized RAM = HAZARD
        sta COLUBK
Main:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #100
L:      sta WSYNC
        dex
        bne L
        jmp Main
        org $FFFC
        .word Reset
        .word Reset
