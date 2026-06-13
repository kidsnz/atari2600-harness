; rpgmap — 部屋制マップ・ナビゲーション（technique: rpgmap, U-AR ⑬, za2600 参照）
; 系譜: za2600（Zelda 移植）の世界マップ（kworld/rs/spr）を読み解いた核の蒸留。
; 方式: 2x2 の 4 部屋。各部屋は PF1 壁テーブル（8 バンド）。自機（P0）をジョイスティックで動かし、
;   画面端を越えると**隣室へ遷移**（room 番号変更＋自機を反対端へワープ）。
;   テーブル駆動＝部屋を増やすのはデータ追加だけ（za2600 の rs/ と同じ思想）。
; 検証: 端越えで room 変数が変わる・部屋ごとに PF1 が違う（golden）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF1     = $0E
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
SWCHA   = $0280
TIM64T  = $0296
INTIM   = $0284

room    = $80      ; 0..3（部屋番号 = roomY*2 + roomX）
px      = $81      ; 自機 X（4..150）
py      = $82      ; 自機 Y（バンド 0..7 の行）
band    = $83
roomPtr = $84      ; 現部屋テーブル先頭（lo）
posT    = $86

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$46
        sta COLUPF
        lda #$1E
        sta COLUP0
        lda #$01            ; reflect 壁
        sta CTRLPF
        lda #76
        sta px
        lda #4
        sta py

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
        lda #42
        sta TIM64T

        ; ===== 移動（SWCHA P0: D7=R D6=L D5=D D4=U, 0=押下）=====
        lda SWCHA
        asl                 ; D7→C (right)
        bcs MNoR
        inc px
MNoR:   asl                 ; D6→C (left)
        bcs MNoL
        dec px
MNoL:   asl                 ; D5→C (down)
        bcs MNoD
        inc py
MNoD:   asl                 ; D4→C (up)
        bcs MNoU
        dec py
MNoU:
        ; ===== 端越え → 部屋遷移 =====
        lda px
        cmp #4
        bcs ChkR
        lda room            ; 左端: roomX を 0→1 トグル（room ^= 1）
        eor #1
        sta room
        lda #150
        sta px
        jmp EdgeDone
ChkR:   lda px
        cmp #151
        bcc ChkU
        lda room
        eor #1
        sta room
        lda #5
        sta px
        jmp EdgeDone
ChkU:   lda py
        cmp #1
        bcs ChkD
        lda room            ; 上端: roomY を 0→2 トグル（room ^= 2）
        eor #2
        sta room
        lda #7
        sta py
        jmp EdgeDone
ChkD:   lda py
        cmp #8
        bcc EdgeDone
        lda room
        eor #2
        sta room
        lda #0
        sta py
EdgeDone:
        ; ===== 部屋テーブルのポインタ（room*8）=====
        lda room
        asl
        asl
        asl                 ; *8
        clc
        adc #<Room0
        sta roomPtr
        lda #>Room0
        sta roomPtr+1

        ldx #34
VB:     sta WSYNC
        dex
        bne VB
        ; --- 自機 X 配置（PosObject）---
        lda px
        jsr PosP0
        lda #0
        sta VBLANK

        ; ===== 可視: 8 壁バンド × 各 24 行 =====
        ldx #0              ; バンド 0..7
KBand:  txa
        tay
        lda (roomPtr),y     ; 部屋テーブル[band]
        sta PF1
        ; 自機 GRP0: py のバンドで点灯
        ldy #24
KRow:   sta WSYNC
        cpx py
        bne KNoMan
        cpy #16
        bcc KNoMan
        lda #$FF
        sta GRP0
        jmp KManDone
KNoMan: lda #0
        sta GRP0
KManDone:
        dey
        bne KRow
        inx
        cpx #8
        bne KBand
        lda #0
        sta PF1
        sta GRP0

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; PosObject（共通形）
        align 16
PosP0:  clc
        adc #1
        sec
        sta WSYNC
PDiv:   sbc #15
        bcs PDiv
        eor #7
        asl
        asl
        asl
        asl
        sta HMP0
        sta RESP0
        sta WSYNC
        sta HMOVE
        ds 12, $EA
        sta HMCLR
        rts

Room0: byte $FF,$81,$81,$80,$80,$81,$81,$FF
Room1: byte $FF,$81,$99,$99,$99,$99,$81,$FF
Room2: byte $FF,$C3,$81,$00,$00,$81,$C3,$FF
Room3: byte $FF,$81,$A5,$81,$81,$A5,$81,$FF
        org $FFFC
        .word Start
        .word Start
