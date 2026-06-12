; paddle_demo — パドル入力の定石（technique: paddle, U-M6）
; litmus_paddle（v0.54.0: ダンプ/充電カーブ実測済み）の authoring 側:
;   ・overscan〜VBLANK は VBLANK=$82（ブランク＋D7 ダンプ）でコンデンサを放電
;   ・可視開始で VBLANK=0 → 充電開始 → 毎行 INPT0 D7 を監視し、立った行数 = パドル値
;   ・値はフレーム末に確定 → 次フレームでバー（P0）の X に写像（クランプ 0-151）
; 検証: set_input paddle 0.1/0.25/0.5 → 行数が litmus カーブどおり・バー X が追従。
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
HMCLR   = $2B
INPT0   = $38
TIM64T  = $0296
INTIM   = $0284

padVal  = $80       ; 確定パドル値（前フレームの計測行数）
padNew  = $81       ; 計測中（$FF=未確定）
barX    = $82       ; バー X（padVal クランプ）

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$5E
        sta COLUP0

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #$82            ; ブランク＋ダンプ（放電）
        sta VBLANK
        lda #49
        sta TIM64T
        ; --- 前フレームの計測を確定 → バー X ---
        lda padNew
        cmp #$FF
        bne PadOK
        lda #191            ; 一度も立たず＝最大
PadOK:  sta padVal
        cmp #152
        bcc BarOK
        lda #151
BarOK:  sta barX
        lda #$FF
        sta padNew
        ; --- バー配置（PosObj 1 体） ---
VBwait: lda INTIM
        bne VBwait
        lda barX
        ldx #0
        jsr PosObj
        sta WSYNC
        lda #0              ; ダンプ解除＝充電開始＋可視開始
        sta VBLANK

        ; ===== 可視 186 行: INPT0 監視＋バー描画（行 80-119） =====
        ldx #0
KLine:  sta WSYNC
        lda padNew          ; 未確定なら INPT0 を見る
        cmp #$FF
        bne KDraw
        bit INPT0
        bpl KDraw           ; D7=0: まだ充電中
        stx padNew          ; 立った行数を記録
KDraw:  txa
        sec
        sbc #80
        cmp #40
        bcc KOn
        lda #0
        beq KStore
KOn:    lda #$7E            ; バー（6px 幅）
KStore: sta GRP0
        inx
        cpx #186
        bne KLine

        lda #0
        sta GRP0
        lda #$82            ; ブランク＋ダンプ再開
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ===== PosObj（bullets 技と同形・indexed・実測校正） =====
        align 16
PosObj: clc
        adc #3              ; 校正: 実X = A−3（read_tia 裏取り）
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
        ds 12, $EA
        sta HMCLR
        rts

        org $FFFC
        .word Start
        .word Start
