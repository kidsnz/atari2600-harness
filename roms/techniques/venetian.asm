; venetian — Venetian Blinds のクリーンルーム・デモ（technique #12, docs/techniques/venetian-blinds.md）
; Bob Whitehead（Video Chess, 1979）の古典。flicker（#10）がフレーム間の時分割なのに対し、
; こちらは**フレーム内の行分割**: 同じ縦ゾーンの偶数行に図形A・奇数行に図形Bを 1 つの player で
; 交互に描く＝2体が縞（ブラインド）状に**点滅ゼロ（60Hz 安定）**で共存。代償=縦密度が半分の縞見た目。
; 色も行毎に乗せ替え（A=白 / B=赤）＝ 1 レジスタで 2 色。
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

XP      = 80
ZTOP    = 60        ; ゾーン開始行
sent    = $9E

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
        lda #$F7
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
        sta WSYNC           ; VB 1
        lda #XP
        clc
        adc #XCAL
        sec
Dv:     sbc #15
        bcs Dv
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

        ; --- 行 0-59: 空 ---
        ldy #60
Top:    sta WSYNC
        dey
        bne Top
        ; --- 行 60-123: ブラインド・ゾーン（64 行 = 絵 8 行 × 8 行ずつ、偶=A/奇=B）---
        ldx #0
Zone:   sta WSYNC
        txa                 ; 2
        and #1              ; 4   行パリティ
        bne ZOdd            ; 6/7
        txa                 ; 8
        lsr
        lsr
        lsr                 ; 14  絵の行 = s/8
        tay                 ; 16
        lda ArtA,y          ; 20
        sta GRP0            ; 23  （表示窓 ~49cy より前）
        lda #$0E            ; 25  A=白
        sta COLUP0          ; 28
        jmp ZNext           ; 31
ZOdd:   txa                 ; 9
        lsr
        lsr
        lsr                 ; 15
        tay                 ; 17
        lda ArtB,y          ; 21
        sta GRP0            ; 24
        lda #$42            ; 26  B=赤
        sta COLUP0          ; 29
ZNext:  inx
        cpx #64
        bne Zone            ; ~38
        ; --- 行 124-191: 空 ---
        lda #0
        sta GRP0
        ldy #68
Bot:    sta WSYNC
        dey
        bne Bot

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+(60+64+68)+30 = 262 を明示所有

XCAL = -5                   ; lda #imm プロローグ。実測で確認すること

ArtA:   ; 菱形（白で出る方・自作）
        byte %00011000
        byte %00111100
        byte %01111110
        byte %11111111
        byte %11111111
        byte %01111110
        byte %00111100
        byte %00011000
ArtB:   ; 枠（赤で出る方・自作）
        byte %11111111
        byte %10000001
        byte %10000001
        byte %10000001
        byte %10000001
        byte %10000001
        byte %10000001
        byte %11111111

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
