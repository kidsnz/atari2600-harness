; uninit_clean — VV-10 T-3 CLEAN twin: clears all RAM ($80-$FF) with the standard
; indexed loop BEFORE reading any of it, then reads a RAM byte (write-then-read).
; WatchUninitRead must report NO hit. The clear writes 128 bytes via ONE indexed
; instruction per iteration (sta $00,x landing in RAM for x>=$80) — the detector's
; effective-address tracking must mark them all (no false positive).

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
        lda #0
Clr:    sta $00,x          ; clears RAM 0x80-0xFF (and TIA mirror below) — indexed
        dex
        bne Clr

        lda $90            ; READ a RAM byte that WAS cleared above = fine
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
