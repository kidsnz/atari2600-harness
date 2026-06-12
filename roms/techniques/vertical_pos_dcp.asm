; vertical_pos_dcp — skipDraw（未文書命令 DCP）変種（technique #3 の verified variant）
; compare 方式（vertical_pos.asm）と同じ見た目を、毎行
;   lda #H-1 / DCP sprDraw / bcs 圏内
; で実現する古典 idiom。DCP（$C7 zp）= DEC+CMP の複合。sprDraw は毎フレーム sprY+H に初期化し
; 毎行デクリメント＝「カウントダウンが 0..H-1 を通過する H 行だけ描く」。絵は逆順テーブル。
; DASM は未文書命令ニーモニック非対応 → .byte \$C7 で直接エンコード。
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
sprDraw  = $82      ; skipDraw カウンタ（毎フレーム sprY+H で初期化・毎行 DCP で減算）
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
        lda #$B4
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
MvDone: lda sprY
        clc
        adc #SPRITE_H
        sta sprDraw         ; skipDraw 初期化（可視行毎に1減 → 0..H-1 の H 行だけ圏内）
        sta WSYNC           ; VB 1
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

        ; --- 可視 192 行: skipDraw（DCP）。圏内 20cy / 圏外 17cy ＝ compare 方式より軽い ---
        ldy #0
Vis:    sta WSYNC
        lda #SPRITE_H-1     ; 2
        .byte $C7,sprDraw   ; 7   DCP sprDraw（M--; A と比較）
        bcs VDraw           ; 9/10 圏内（sprDraw ≤ H-1）
        lda #0              ; 11
        beq VStore          ; 14（必ず成立）
VDraw:  ldx sprDraw         ; 13
        lda ArtRev,x        ; 17  逆順テーブル（sprDraw は H-1→0 と降る）
VStore: sta GRP0            ; ~17-20
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

ArtRev: ; 8×8 ボール（自作・skipDraw 用に下→上の逆順格納）
        byte %00111100
        byte %01111110
        byte %11100111
        byte %11111111
        byte %11011011
        byte %11111111
        byte %01111110
        byte %00111100

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
