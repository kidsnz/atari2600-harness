; vertical_pos — 縦位置決めのクリーンルーム・デモ（technique #3, docs/techniques/vertical-positioning.md）
; 縦位置にはハード支援が無い（RESP は横だけ）。kernel の毎行で
;   行番号 − sprY を計算 → 0..H-1 なら絵の行、外なら 0 を GRP0 へ
; を流す＝任意の Y に 1px 単位で滑らかに置ける。本デモは Y 4⇔180 をピンポンするボール。
; 判定は文書化命令のみの compare 方式（DCP=skipDraw 変種は doc 参照）。
; 横は起動後毎フレーム固定 X=80 へ（divide-by-15＋HMOVE 表、pos(v)=v 校正済み・sprite_anim と同型）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A

SPRITE_H = 8
Y_MIN    = 4
Y_MAX    = 180
XPOS     = 80

sprY     = $80      ; スプライト上端の可視行（0-191）
vdir     = $81      ; 0=下へ / 1=上へ
sent     = $9E      ; 実行センチネル

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
        lda #$86
        sta COLUP0          ; 青
        lda #Y_MIN
        sta sprY
        lda #$B3
        sta sent

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
        ; --- VB行1: 縦移動（ピンポン）---
        lda vdir
        bne MoveUp
        inc sprY            ; 下へ
        lda sprY
        cmp #Y_MAX
        bcc MvDone
        lda #1
        sta vdir
        jmp MvDone
MoveUp: dec sprY            ; 上へ
        lda sprY
        cmp #Y_MIN+1
        bcs MvDone
        lda #0
        sta vdir
MvDone: sta WSYNC           ; VB 1
        ; --- VB行2: P0 を X=80 へ（粗+微・pos(v)=v 校正）---
        lda #XPOS
        clc
        adc #XCAL
        sec
Div:    sbc #15
        bcs Div
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 2
        sta HMOVE
        ldx #34             ; VBLANK 残り（1+1+34+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 可視 192 行: 毎行「行−sprY ∈ 0..H-1 ?」で絵 or 0（分岐両経路とも予算内）---
        ldy #0
Vis:    sta WSYNC
        tya                 ; 2   現在行
        sec                 ; 4
        sbc sprY            ; 7   = スプライト内の行
        cmp #SPRITE_H       ; 9
        bcc VDraw           ; 11/12
        lda #0              ; 13  圏外＝消す
        beq VStore          ; 16（必ず成立）
VDraw:  tax                 ; 14
        lda Art,x           ; 18
VStore: sta GRP0            ; ~21  可視窓（P0 は X=80 → ~49cy）より十分前
        iny
        cpy #192
        bne Vis

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+192+30 = 262 を明示所有

XCAL = -5                   ; 校正は kernel 固有: 本 ROM は lda #即値（2cy）で sprite_anim（lda zp=3cy）より
                            ; プロローグが 1cy 短い＝3px 左に出る → -8+3。実測 hmoved=80 で確認

Art:    ; 8×8 ボール（自作）
        byte %00111100
        byte %01111110
        byte %11111111
        byte %11011011
        byte %11111111
        byte %11100111
        byte %01111110
        byte %00111100

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
