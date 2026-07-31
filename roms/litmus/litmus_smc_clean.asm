; litmus_smc_clean.asm — the TWIN of litmus_smc: the same kernel with the planted
; store aimed at RAM instead of at code.
;
; It exists so a report that fires on litmus_smc is attributable. The two ROMs differ
; by ONE operand — the store's target — so "this write lands in code" can only be that
; operand. This one must produce NO writes-into-code entry at all.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

scratch = $80

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

        ; The same store, aimed at RAM.
        lda #$EA
        sta scratch

; --- Visible: 192 lines ---
        ldx #192
Vis:    sta WSYNC
Target: stx COLUBK      ; <- decoded, executed, and the store above aims at it
        dex
        bne Vis

; --- Overscan: 30 lines ---
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
