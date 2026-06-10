; frog_control.asm — M3: カエルの操作を隔離検証。
; player0 をカエルとして表示し、SWCHA（P0 ジョイスティック）を読んで左右へ動かす。
; 移動は HMP0＋毎フレーム HMOVE strobe（右=$E0=右2px / 左=$20=左2px / 入力なし=0）。
; ハーネス検証: poke SWCHA で入力を与え（D7=右, D6=左, アクティブLOW）、read_tia の
; player0.HmovedPixel が右入力で増・左入力で減・無入力で不変、を数値確認する。

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
SWCHA   = $0280

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
        sta COLUBK      ; 黒背景（カエルを隔離）
        lda #$1C
        sta COLUP0      ; カエル 黄緑
        lda #$00
        sta NUSIZ0      ; 単体

        sta WSYNC
        ldx #20
D:      dex
        bne D
        sta RESP0       ; 初期 X（中央寄り）

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

; --- 入力 → HMP0 ---
        ldx #$00        ; 既定: 動かない
        lda SWCHA
        and #$80        ; P0 right (D7, 0=押下)
        bne ChkLeft
        ldx #$E0        ; 右 2px
ChkLeft:
        lda SWCHA
        and #$40        ; P0 left (D6, 0=押下)
        bne SetHM
        ldx #$20        ; 左 2px
SetHM:
        stx HMP0

; --- VBLANK: HMOVE を line 先頭で ---
        sta WSYNC
        sta HMOVE
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible: top 92 / frog 8 / bottom 92 = 192 ---
        ldx #92
TB:     sta WSYNC
        dex
        bne TB

        ldx #0
FR:     sta WSYNC
        lda FrogGfx,x
        sta GRP0
        inx
        cpx #8
        bne FR
        lda #0
        sta GRP0

        ldx #92
BB:     sta WSYNC
        dex
        bne BB

; --- Overscan ---
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
