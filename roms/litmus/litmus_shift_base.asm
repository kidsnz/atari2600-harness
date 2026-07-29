; litmus_shift_base.asm — a fixed playfield at a known vertical position.
;
; Pairs with litmus_shift_down8.asm, which is byte-for-byte the same work with
; the visible content moved down EIGHT scanlines (eight lines taken from the
; visible area and given to VBLANK, so the frame still totals 262). The pair
; exists so a vertical-offset detector can be graded against a number that is
; known by construction rather than inferred from the answer it produces.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
CTRLPF  = $0A
COLUPF  = $08
COLUBK  = $09
PF0     = $0D
PF1     = $0E
PF2     = $0F

SHIFT   = 0

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
clr:    sta $00,x
        dex
        bne clr

        lda #$0E
        sta COLUPF
        lda #$00
        sta COLUBK
        lda #1
        sta CTRLPF          ; reflect

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
        ldx #(37+SHIFT)
vb:     sta WSYNC
        dex
        bne vb

        lda #0
        sta VBLANK

        ; 40 blank lines, then a 24-line bar, then the rest blank.
        ldx #40
top:    sta WSYNC
        dex
        bne top

        lda #$F0
        sta PF0
        lda #$FF
        sta PF1
        lda #$FF
        sta PF2
        ldx #24
bar:    sta WSYNC
        dex
        bne bar
        lda #0
        sta PF0
        sta PF1
        sta PF2

        ldx #(127-SHIFT)
bot:    sta WSYNC
        dex
        bne bot

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
