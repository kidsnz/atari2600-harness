; litmus_48px — 48ピクセルスプライト（"Six-Digit Score Trick" / Staugas kernel, V2-13）
; score6.asm の正確な位置決めレシピをクリーンルームで再現:
;   NUSIZ=3コピー → WSYNC → SLEEP 26 → RESP0 → RESP1(+9clk) → HMP1=$10(-1→+8) → WSYNC → HMOVE → SLEEP 24 → HMCLR。
; P0コピー(0,16,32)＋P1コピー(8,24,40) が interleave = 8px×6 = 48px 連続。
; ここでは静的 GRP=$FF（VDEL off）で「48px 連続バー」の幾何を裏取り。動的6書換は後続（同じ位置決めを使う）。
; 実機裏取り済（Gopher2600, v0.51.0）: read_row(8)=clock 24-71 が白 len【48】＝隙間ゼロの48px連続バー。
; P0=24 / P1=32（=+8 ちょうど, read_tia）。score6 の精密 SLEEP レシピで前回失敗(ループ版の1px隙間)を克服。
; 回帰固定=scenarios/p48.json。動的6書換（コピー毎に別グラフィック）は VDEL(V2-1済) を使う後続。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B

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
        sta $2C
        lda #$0E
        sta COLUP0
        sta COLUP1
        lda #0
        sta COLUBK
        lda #$03
        sta NUSIZ0
        sta NUSIZ1          ; 3コピー close
        lda #$FF
        sta GRP0
        sta GRP1

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
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- 48px 位置決め（score6 レシピ厳密）---
        sta WSYNC
        ds 13, $EA          ; SLEEP 26 cycles（NOP×13）
        sta RESP0           ; P0 ストローブ
        sta RESP1           ; +3cy=9clk → P1 粗=P0+9
        lda #$10
        sta HMP1            ; -1 → P1=P0+8
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24（HMOVE と HMxx 書込の間に必須）
        sta HMCLR

        ; --- 48px バーを 16 行描画（GRP=$FF 静的）---
        ldy #16
Draw:   sta WSYNC
        dey
        bne Draw

        lda #0
        sta GRP0
        sta GRP1
        ldy #174
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        lda #$FF
        sta GRP0
        sta GRP1
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
