; frog_hop.asm — M3/C: カエルの縦ジャンプ（hop）を隔離検証。
; player0 をカエルとして「可変 scanline FrogY」に描く。SWCHA の上(D4)/下(D5)をエッジ検出し、
; 押した瞬間に FrogY を ±16（1レーン）離散ジャンプ。連続入力で1回だけ動く（Frogger 的）。
; ハーネス検証: set_input up/down（押下→解除→押下）で peek FrogY($81) が 16 ずつ変化。screenshot で縦移動。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
SWCHA   = $0280

FrogY   = $81           ; カエルの縦位置（可視 scanline 上端）
PrevU   = $82           ; 直前の up 状態（エッジ検出用、$10=離す/$00=押下）
PrevD   = $83           ; 直前の down 状態（$20/$00）

        org $F000
Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #$00
        sta COLUBK
        lda #$1C
        sta COLUP0      ; カエル 黄緑
        lda #92
        sta FrogY       ; 初期位置
        lda #$10
        sta PrevU       ; 離した状態
        lda #$20
        sta PrevD

        sta WSYNC
        ldx #30
Xd:     dex
        bne Xd
        sta RESP0       ; X 位置（中央寄り・固定）

MainLoop:
; --- VSYNC ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- 上下エッジ検出 → FrogY ±16 ---
        lda SWCHA
        and #$10        ; up (D4)
        cmp PrevU
        beq UDone       ; 変化なし
        sta PrevU
        cmp #$00
        bne UDone       ; 今 離した（=$10）→ 何もしない
        ; up を今 押した瞬間
        lda FrogY
        cmp #24
        bcc UDone       ; 上限（これ以上 上に行かない）
        sbc #16
        sta FrogY
UDone:
        lda SWCHA
        and #$20        ; down (D5)
        cmp PrevD
        beq DDone
        sta PrevD
        cmp #$00
        bne DDone       ; 今 離した→何もしない
        ; down を今 押した瞬間
        lda FrogY
        cmp #168
        bcs DDone       ; 下限
        clc
        adc #16
        sta FrogY
DDone:

; --- VBLANK 37 ---
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible 192: 可変 FrogY にカエルを描く ---
        ldy #0
VL:     sta WSYNC
        tya
        sec
        sbc FrogY
        cmp #8
        bcs NF          ; (y-FrogY) >= 8 → 帯の外
        tax
        lda FrogGfx,x
        jmp SF
NF:     lda #0
SF:     sta GRP0
        iny
        cpy #192
        bne VL
        lda #0
        sta GRP0

; --- Overscan 30 ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp MainLoop

FrogGfx:
        .byte %00100100
        .byte %01111110
        .byte %11111111
        .byte %11111111
        .byte %10111101
        .byte %01111110
        .byte %00100100
        .byte %01000010

        org $FFFC
        .word Reset
        .word Reset
