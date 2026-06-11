; zone_multiplex — 縦ゾーン多重化のクリーンルーム・デモ（technique #1, docs/techniques/zone-multiplexing.md）
; 6 ゾーン、各ゾーンで player0/player1 を別 X に再配置して 8px スプライトを描く＝2スプライト機で 2×6＝12 個に見える。
; （DaveC の Zone を学んだ自前実装。位置決めは divide-by-15＋HMOVE テーブル＝harness 裏取り済みの方式）
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
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- 6 ゾーン × (P0,P1) ---
        ldx #0                  ; zone index
ZoneLoop:
        ; P0 位置決め
        sta WSYNC
        lda ZoneX0,x
        sec
Div0:   sbc #15
        bcs Div0
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        ; P1 位置決め
        sta WSYNC
        lda ZoneX1,x
        sec
Div1:   sbc #15
        bcs Div1
        tay
        lda HMOVE_LUT,y
        sta HMP1
        sta RESP1
        ; 微調整適用
        sta WSYNC
        sta HMOVE
        ; スプライト 8 ライン（P0/P1 同形・別色）
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
        ; ゾーン残り 5 ライン消灯
        ldy #5
Blz:    sta WSYNC
        dey
        bne Blz
        inx
        cpx #NZONES
        bne ZoneLoop

        ; 残り可視を埋める（6*16=96 使用 → 96 ライン消灯）
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

ZoneX0:
        byte 20, 50, 90, 120, 140, 70    ; P0 の各ゾーン目標 X
ZoneX1:
        byte 100, 130, 30, 60, 95, 150   ; P1 の各ゾーン目標 X（P0 と交互にバラける）
Sprite:
        byte $18,$3C,$7E,$FF,$FF,$7E,$3C,$18

; divide-by-15 の余り（Y=$F1..$FF）→ HMOVE ニブル変換テーブル（DaveC 流の負インデックス）
HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
