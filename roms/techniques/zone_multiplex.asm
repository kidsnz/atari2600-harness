; zone_multiplex — 縦ゾーン多重化のクリーンルーム・デモ（technique #1, docs/techniques/zone-multiplexing.md）
; 6 ゾーン × player0/player1 を各ゾーンで別 X に再配置＝2スプライト機で 12 個。
; さらに各ゾーンの X を毎フレーム更新（P0=右へ / P1=左へ流す）＝12 個がうねうね動く。
; （DaveC の Zone を学んだ自前実装。位置決めは divide-by-15＋HMOVE テーブル＝harness 裏取り済み）
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B

NZONES   = 6
SPRITE_H = 8

; RAM: 各ゾーンの現在 X（毎フレーム更新するので ROM でなく RAM）
zx0     = $80          ; player0 X × NZONES
zx1     = $86          ; player1 X × NZONES

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$0E
        sta COLUP0          ; P0 白
        lda #$48
        sta COLUP1          ; P1 赤系
        lda #0
        sta COLUBK
        sta NUSIZ0
        sta NUSIZ1
        ; 初期 X を ROM から RAM へコピー
        ldx #NZONES-1
InitX:  lda ZoneX0_init,x
        sta zx0,x
        lda ZoneX1_init,x
        sta zx1,x
        dex
        bpl InitX

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ; --- 位置更新（P0 右へ +1 / P1 左へ -1、0..127 で巻く）---
        ldx #NZONES-1
Upd:    lda zx0,x
        clc
        adc #1
        and #$7F
        sta zx0,x
        lda zx1,x
        sec
        sbc #1
        and #$7F
        sta zx1,x
        dex
        bpl Upd
        ; --- VBLANK 残りを WSYNC で埋める（更新ぶんを引いて全体を 262 に保つ）---
        ldx #35
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- 6 ゾーン × (P0,P1) ---
        ldx #0
ZoneLoop:
        sta WSYNC
        lda zx0,x
        sec
Div0:   sbc #15
        bcs Div0
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC
        lda zx1,x
        sec
Div1:   sbc #15
        bcs Div1
        tay
        lda HMOVE_LUT,y
        sta HMP1
        sta RESP1
        sta WSYNC
        sta HMOVE
        ldy #0
Spr:    sta WSYNC
        lda Sprite,y
        sta GRP0
        sta GRP1
        iny
        cpy #SPRITE_H
        bne Spr
        lda #0
        sta GRP0
        sta GRP1
        ldy #5
Blz:    sta WSYNC
        dey
        bne Blz
        inx
        cpx #NZONES
        bne ZoneLoop

        ldy #96
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

ZoneX0_init:
        byte 20, 50, 90, 120, 110, 70
ZoneX1_init:
        byte 100, 70, 30, 60, 95, 120
Sprite:
        byte $18,$3C,$7E,$FF,$FF,$7E,$3C,$18

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
