; litmus_collide.asm — read_collisions（P1）の陽性検出 検証用 ROM
; 目的: 衝突ラッチが実際に立つ状況を決定的に作り、ReadCollisions が BL-PF を true で拾うことを裏取り。
; 仕掛け: playfield を全点灯（PF0/1/2=$FF）＋ボール有効（ENABL）にして可視全域を描く。
;   ボールと点灯 PF が必ず重なる → CXBLPF(D7) が立つ。プレイヤーは未描画なので P-PF 等は立たない。
; include は使わず自己完結。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESBL   = $14
ENABL   = $1F
COLUBK  = $09
COLUPF  = $08

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

        lda #$FF
        sta PF0
        sta PF1
        sta PF2         ; playfield 全点灯
        lda #$30
        sta CTRLPF      ; ball size = 8 clocks（重なりを確実に）
        lda #$02
        sta ENABL       ; ball 有効
        lda #$0E
        sta COLUPF

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
        sta WSYNC
        sta RESBL       ; ball をラインの左寄りに配置（可視域）

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
