; motion_glide.asm — VV-4 self-test (clean reference).
; A 2px ball glides DOWN at a constant +1 scanline/frame over a uniform black
; background, fixed X. Constant velocity => the rendered top has zero 2nd
; difference => motion jerk_rms == 0. Pairs with motion_stutter.asm.
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
CTRLPF  = $0A
COLUPF  = $08
COLUBK  = $09
RESBL   = $14
ENABL   = $1F
HMCLR   = $2B
CXCLR   = $2C

posY    = $80

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
        sta CXCLR
        sta HMCLR
        lda #$10        ; CTRLPF: ball 2 clocks wide
        sta CTRLPF
        lda #$0E        ; ball/PF colour = white
        sta COLUPF
        lda #82
        sta posY        ; start mid-screen; +1/frame stays in range for ~100 frames

main:
; --- VSYNC 3 ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
; --- VBLANK 37 (strobe RESBL once at a fixed cycle => deterministic X) ---
        ldx #37
        sta WSYNC       ; vblank line 1
        nop
        nop
        nop
        nop
        nop
        nop
        sta RESBL       ; fixed X every frame
        dex
vbl:    sta WSYNC
        dex
        bne vbl
        lda #0
        sta VBLANK
; --- Visible 192: ball on when (line - posY) in [0,4) ---
        sta COLUBK      ; black (A==0)
        ldy #0
vis:    sta WSYNC
        tya
        sec
        sbc posY
        cmp #4
        lda #0
        bcs noball
        lda #2
noball: sta ENABL
        iny
        cpy #192
        bne vis
; --- Overscan 30 ---
        lda #2
        sta VBLANK
        lda #0
        sta ENABL
        ldx #30
ovs:    sta WSYNC
        dex
        bne ovs
; --- frame update: +1 (constant velocity) ---
        inc posY
        jmp main

        org $FFFC
        .word Reset
        .word Reset
