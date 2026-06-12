; bullets — ミサイル弾の完全形（technique: missiles-bullets, U-M5）
; 「自機から撃つ・飛ぶ・当たる」の標準形:
;   ・発射 = RESMP0 ロックを 1 フレーム保持→解除（M0 はプレイヤー+4px＝中央に出現。
;     ★同一フレーム内の lock→unlock では動かない: ロック同期はプレイヤーカウンタが
;     走る間に行われるため、最低 1 フレームのロック保持が必要 — 実測知見）
;   ・飛翔 = bulY を毎フレーム −4、kernel は [bulY, bulY+4) の行だけ ENAM0
;     （非アクティブ番兵 = 200: 行範囲外なら分岐不要＝カーネル痩身。$FF はロック中）
;   ・命中 = CXM0P D7（M0×P1）。hit でカウンタ++・弾消滅・敵を +24px 先へ再配置・CXCLR
; 自機 P0（ジョイスティック左右・PosObject 配置）、敵 P1（上部・PosObject 配置）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
ENAM0   = $1D
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
RESMP0  = $28
CXM0P   = $30        ; 読みアドレス（D7=M0×P1, D6=M0×P0）
CXCLR   = $2C
SWCHA   = $0280
INPT4   = $3C
TIM64T  = $0296
INTIM   = $0284

shipX   = $80       ; 自機 X（実 X と一致・PosObject 校正済み）
bulY    = $81       ; 弾の行（0=非アクティブ）
prevFi  = $82
hits    = $83       ; 命中数
enemyX  = $84       ; 敵 X
ln      = $85
posT    = $86       ; PosObj 一時

SHIPROW = 150
ENEMYROW = 30

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #60
        sta shipX
        sta enemyX
        lda #200            ; 弾 非アクティブ
        sta bulY
        lda #$3E            ; 自機=黄
        sta COLUP0
        lda #$44            ; 敵=赤
        sta COLUP1

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
        lda #47
        sta TIM64T

        ; ===== ロジック =====
        ; 自機移動
        lda SWCHA
        and #$80
        bne NoR
        lda shipX
        cmp #148
        bcs NoR
        inc shipX
NoR:    lda SWCHA
        and #$40
        bne NoL
        lda shipX
        cmp #5
        bcc NoL
        dec shipX
NoL:
        ; 弾の状態遷移（★fire エッジより先に処理 — 同一パスで即解除しないため。
        ;  ロックはここで「次のフレームに」解除される＝丸 1 フレーム保持）
        lda bulY
        cmp #$FF
        bne BulFly
        lda #0              ; 1 フレーム経過 → 解除＝自機+4px に出現
        sta RESMP0
        lda #SHIPROW-4
        sta bulY
        jmp BulDone
BulFly: cmp #200
        beq BulDone
        sec
        sbc #4
        cmp #8
        bcs BulSave
        lda #200
BulSave:
        sta bulY
BulDone:
        ; fire エッジ → 発射（弾が無いときだけ）
        ldx #0
        lda INPT4
        bmi FiSnap
        ldx #1
FiSnap: cpx #1
        bne FiUpd
        lda prevFi
        bne FiUpd
        lda bulY
        cmp #200
        bne FiUpd
        lda #2              ; ロック開始（1 フレーム保持）
        sta RESMP0
        lda #$FF
        sta bulY
FiUpd:  stx prevFi
        ; 命中判定（M0×P1 = CXM0P D7）
        bit CXM0P
        bpl NoHit
        inc hits
        lda #200
        sta bulY
        lda enemyX
        clc
        adc #24
        cmp #140
        bcc EnOK
        lda #20
EnOK:   sta enemyX
NoHit:  sta CXCLR           ; 毎フレームクリア（sticky 対策）

        ; ===== 配置（VBLANK 内・PosObject ×2） =====
VBwait: lda INTIM
        bne VBwait
        lda enemyX
        ldx #1
        jsr PosObj
        lda shipX
        ldx #0
        jsr PosObj
        sta WSYNC
        lda #0
        sta VBLANK

        ; ===== 可視 184 行（X=行カウンタ・番兵方式で全行 ~62cy） =====
        ldx #0
KLine:  sta WSYNC
        txa                 ; 敵（行 30-37）
        sec
        sbc #ENEMYROW
        cmp #8
        bcc EDraw
        lda #0
        beq EStore
EDraw:  tay
        lda EArt,y
EStore: sta GRP1
        txa                 ; 弾（[bulY, bulY+4)・非アクティブは番兵 200/255 で自然に圏外）
        sec
        sbc bulY
        cmp #4
        bcc BOn
        lda #0
        beq BStore
BOn:    lda #2
BStore: sta ENAM0
        txa                 ; 自機（行 150-157）
        sec
        sbc #SHIPROW
        cmp #8
        bcc SDraw
        lda #0
        beq SStore
SDraw:  tay
        lda Ship,y
SStore: sta GRP0
        inx
        cpx #184
        bne KLine

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        sta GRP1
        sta ENAM0
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ===== PosObj: A=目標 X, X=0(P0)/1(P1)。2 行使用。eor #7 形＋実測校正 =====
        align 16
PosObj: clc
        adc #3              ; 校正: 実X = A−3（indexed store 形, read_tia 裏取り 2026-06-12）
        sec
        sta WSYNC
PDiv:   sbc #15
        bcs PDiv
        eor #7
        asl
        asl
        asl
        asl
        sta HMP0,x
        sta RESP0,x
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; HMOVE 後 24cy は HMxx 禁止
        sta HMCLR
        rts

Ship:   byte %00011000
        byte %00011000
        byte %00111100
        byte %00111100
        byte %01111110
        byte %11111111
        byte %11011011
        byte %10011001

EArt:   byte %10000001
        byte %01011010
        byte %00111100
        byte %01111110
        byte %11111111
        byte %01100110
        byte %01000010
        byte %10100101

        org $FFFC
        .word Start
        .word Start
