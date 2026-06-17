; motion_stutter.asm — VV-4 self-test (the planted judder).
; Same ball/kernel as motion_glide.asm, but it moves DOWN by +2,0,+2,0… — the
; same average speed but a deliberate stutter. The rendered top's velocity
; alternates 2,0,2,0 => its 2nd difference alternates ±2 => motion jerk_rms is
; well above the clean glide's 0. Proves the motion metric is live, not vacuous.

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
phase   = $81

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
        sta posY

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
        sta WSYNC
        nop
        nop
        nop
        nop
        nop
        nop
        sta RESBL
        dex
vbl:    sta WSYNC
        dex
        bne vbl
        lda #0
        sta VBLANK
; --- Visible 192: ball on when (line - posY) in [0,4) ---
        sta COLUBK
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
; --- frame update: +2,0,+2,0 (same average, deliberate stutter) ---
        lda phase
        bne odd
        lda posY
        clc
        adc #2
        sta posY
        lda #1
        sta phase
        jmp main
odd:
        lda #0
        sta phase
        jmp main

        org $FFFC
        .word Reset
        .word Reset
