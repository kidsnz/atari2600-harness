; litmus_title_then_play.asm — a title screen that auto-advances on frame 30.
;
; Frames 0..29 draw one playfield ("title"); from frame 30 onward a different one
; ("gameplay"), forever. Most commercial cartridges behave this way, and a tool
; that drives both ROMs from frame 0 ends up measuring one game's title against
; another's gameplay — every scenario then reads as a mechanic difference for a
; reason that has nothing to do with the mechanics.
;
; The switch frame is 30 by construction, which is the point: an auto-detector
; graded against a frame inferred from its own output would agree with itself
; whatever it reported.
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

frameLo = $80           ; frames elapsed, saturating at 255

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

        ; saturating frame counter
        lda frameLo
        cmp #255
        beq noInc
        inc frameLo
noInc:

        ldx #37
vb:     sta WSYNC
        dex
        bne vb

        lda #0
        sta VBLANK

        ldx #60
top:    sta WSYNC
        dex
        bne top

        ; The band's shape is the only thing that changes at frame 30.
        lda frameLo
        cmp #30
        bcc title
        lda #$C0            ; gameplay pattern
        sta PF1
        lda #$03
        sta PF2
        jmp drawn
title:  lda #$FF            ; title pattern
        sta PF1
        lda #$FF
        sta PF2
drawn:
        lda #$F0
        sta PF0

        ldx #40
band:   sta WSYNC
        dex
        bne band
        lda #0
        sta PF0
        sta PF1
        sta PF2

        ldx #92
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
