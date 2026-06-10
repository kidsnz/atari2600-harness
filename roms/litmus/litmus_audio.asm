; litmus_audio.asm — read_audio（R-2）の検証用 ROM
; 目的: TIA 音声レジスタ（AUDC/AUDF/AUDV）に既知値を書き、read_audio が両チャンネルを正しく読むことを裏取り。
; 仕掛け: 起動時に ch0=(AUDC0=$0C, AUDF0=$14, AUDV0=$0A) / ch1=(AUDC1=$04, AUDF1=$1F, AUDV1=$08) を設定。
;   以後フレームを回すだけ（音声レジスタは書き換えないので値が残る）。
; include は使わず自己完結。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:
        sta $00,x
        dex
        bne Clr

        ; ch0
        lda #$0C
        sta AUDC0
        lda #$14
        sta AUDF0
        lda #$0A
        sta AUDV0
        ; ch1
        lda #$04
        sta AUDC1
        lda #$1F
        sta AUDF1
        lda #$08
        sta AUDV1

Main:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:
        sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldx #192
Vis:
        sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:
        sta WSYNC
        dex
        bne OS
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
