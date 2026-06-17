; cb_clean — the GREEN counterpart of cb_roll: the same striped background but
; with NO heavy line. Every visible line is exactly 1 scanline (<=76cy) and the
; frame is a clean 262 lines. VV-2 certifies this. Compare side by side with
; cb_roll: they look pixel-identical even though cb_roll runs one scanline over
; — a small overrun is visually invisible (verified 2026-06-17), so the defect
; shows up only in VV-2's numbers, never to the eye.

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

; --- Visible top: 96 striped lines ---
        ldx #96
Top:    sta WSYNC
        stx COLUBK
        dex
        bne Top

; --- Visible bottom: 96 striped lines (NO heavy line — clean 1-scanline line) ---
        ldx #96
Bot:    sta WSYNC
        stx COLUBK
        dex
        bne Bot

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
