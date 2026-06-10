; sprite_flow.asm — M3 ステップ1: スプライトで「滑らかな水流」を実証。
; player0 を小さな睡蓮パッドとして縦の一帯に表示し、HMOVE で毎フレーム +1px 右へドリフト。
; HMP0=$F0（右1px/strobe）を一度設定し、毎フレーム HMOVE を 1 回 strobe＝累積で連続移動。
; NUSIZ0=$03 で 3 コピー（レーンに並ぶ葉）。HMOVE は VBLANK 内で撃つ（comb を可視域に出さない）。
; 検証: read_tia の player0.HmovedPixel が毎フレーム +1、screenshot で横移動。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

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

        lda #$1A
        sta COLUBK      ; 水（青緑）背景・単色（スプライトを隔離検証）
        lda #$C8
        sta COLUP0      ; 睡蓮 緑
        lda #$03
        sta NUSIZ0      ; 3 コピー・標準幅

        ; 初期 X 位置: WSYNC 同期後に少し遅延して RESP0（中央寄り）。以後は HMOVE 累積に任せる。
        sta WSYNC
        ldx #12
D:      dex
        bne D
        sta RESP0
        lda #$F0
        sta HMP0        ; 右 1px / HMOVE

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 1 line で HMOVE、残り 36 ---
        sta WSYNC
        sta HMOVE       ; 累積モーション適用（毎フレーム右 1px）
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible: top blank 90 / pad 8 / bottom 94 = 192 ---
        ldx #90
TB:     sta WSYNC
        dex
        bne TB

        ldx #0
PD:     sta WSYNC
        lda PadGfx,x
        sta GRP0
        inx
        cpx #8
        bne PD
        lda #0
        sta GRP0

        ldx #94
BB:     sta WSYNC
        dex
        bne BB

; --- Overscan: 30 ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp MainLoop

PadGfx:
        .byte %00111100
        .byte %01111110
        .byte %11111111
        .byte %11111111
        .byte %11111111
        .byte %11111111
        .byte %01111110
        .byte %00111100

        org $FFFC
        .word Reset
        .word Reset
