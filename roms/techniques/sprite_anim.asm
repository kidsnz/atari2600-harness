; sprite_anim — スプライトアニメーションのクリーンルーム・デモ（technique #2, docs/techniques/sprite-animation.md）
; 8×8 の歩行キャラを 4 フェーズで循環（8 フレーム毎に切替）し、X 10⇔140 をピンポン移動。
; 方向転換で REFP0 を反転（左右反転はハード無料＝絵は1方向ぶんだけ持つ）。
; フレーム切替はポインタでなく「ベースインデックス（phase*8）＋行」の indexed 方式（4K 単バンクで十分）。
; 位置決めは divide-by-15＋HMOVE 負インデックス表（zone_multiplex と同型・harness 裏取り済み）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
REFP0   = $0B
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

ANIM_RATE = 8       ; フレーム/アニメフェーズ
X_MIN     = 10
X_MAX     = 140

phase    = $80      ; 0-3
animTmr  = $81      ; 0-7
xpos     = $82      ; 現在 X（意図座標）
dir      = $83      ; 0=右へ / 1=左へ
frameBase = $84     ; phase*8（kernel 用に前計算）
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
        lda #$1E
        sta COLUP0          ; 黄
        lda #0
        sta COLUBK
        sta NUSIZ0
        lda #X_MIN
        sta xpos
        lda #$A2
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
        ; --- VB行1: アニメ進行＋移動（ロジックのみ。REFP0/配置は次行以降）---
        inc animTmr
        lda animTmr
        cmp #ANIM_RATE
        bne NoPh
        lda #0
        sta animTmr
        inc phase
        lda phase
        and #3
        sta phase
NoPh:   lda dir
        bne MoveL
        inc xpos            ; 右へ
        lda xpos
        cmp #X_MAX
        bcc MoveDone
        lda #1
        sta dir
        jmp MoveDone
MoveL:  dec xpos            ; 左へ
        lda xpos
        cmp #X_MIN+1
        bcs MoveDone
        lda #0
        sta dir
MoveDone:
        sta WSYNC           ; VB 1
        ; --- VB行2: 向き反転＋kernel 前計算 ---
        lda dir
        beq FaceR
        lda #8              ; REFP0 D3=1 で左右反転
FaceR:  sta REFP0           ; dir=0 のとき A=0（beq 直前の lda dir の値）
        lda phase
        asl
        asl
        asl
        sta frameBase       ; phase*8
        sta WSYNC           ; VB 2
        ; --- VB行3: P0 を xpos へ（粗=divide-by-15 ＋ 微=HMOVE 表）---
        lda xpos
        clc
        adc #XCAL           ; 校正定数（read_tia の hmoved_pixel == xpos になるよう実測で決定）
        sec
Div:    sbc #15
        bcs Div
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 3
        sta HMOVE           ; WSYNC 直後の HMOVE（裏取り済みの作法）
        ldx #33             ; VBLANK 残り（3+33+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 可視 192 行: 80 blank + 32 行スプライト（8 行×4 倍）+ 80 blank ---
        ldy #80
Top:    sta WSYNC
        dey
        bne Top
        ldy #0
Spr:    sta WSYNC
        tya
        lsr
        lsr                 ; 行 = y/4（行4倍化＝8 行の絵を 32 行に）
        clc
        adc frameBase
        tax
        lda Frames,x
        sta GRP0
        iny
        cpy #32
        bne Spr
        lda #0
        sta GRP0
        ldy #80             ; 3+37+(80+32+80)+30 = 262 を WSYNC 勘定で明示所有
Bot:    sta WSYNC
        dey
        bne Bot

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

XCAL = -8                   ; 実測校正: 適用写像が pos(v)=v になる値（オーガニック掃引で確定）

; --- 歩行サイクル 4 フェーズ × 8 行（自作ドット・上→下）---
; 腕（行3）を前に出して左右非対称＝REFP0 の反転が見えるように。
Frames:
        byte %00011000      ; phase 0: 接地（脚を大きく開く）
        byte %00011000
        byte %00111100
        byte %01111000      ; 腕を前へ（非対称）
        byte %00111100
        byte %00100100
        byte %01000010
        byte %10000001
        byte %00011000      ; phase 1: 通過（脚を閉じていく）
        byte %00011000
        byte %00111100
        byte %01111000
        byte %00111100
        byte %00100100
        byte %00100100
        byte %01000010
        byte %00011000      ; phase 2: 直立（脚が揃う）
        byte %00011000
        byte %00111100
        byte %01111000
        byte %00111100
        byte %00011000
        byte %00011000
        byte %00100100
        byte %00011000      ; phase 3: 通過（開きはじめ）
        byte %00011000
        byte %00111100
        byte %01111000
        byte %00111100
        byte %00100100
        byte %00100100
        byte %01000010

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
