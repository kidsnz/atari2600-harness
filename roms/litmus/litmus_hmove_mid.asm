; litmus_hmove_mid — 行中 HMOVE（HBLANK 外ストローブ）の実機裏取り（overnight M-D）
; Towers TIA_HW_Notes の documented 挙動「HBLANK 外の HMOVE は HM=0 でもオブジェクトを右へ
; （ストローブ時刻依存の量だけ）動かす」（Cosmic Ark の星のトリックの土台）を数値化する。
;
; 方式: 毎フレーム頭に P0 を標準作法（粗+微→WSYNC 直後 HMOVE→HMCLR）で X=60 へ再配置。
; frameCt&3 のパリティで可視行 100 の途中（遅延 0/20/40/60cy 付近）に HM 全クリア状態の
; HMOVE を打つ（k=0 は打たない＝コントロール）。フレーム末の hmoved_pixel が
; 「60 + そのストローブ時刻のシフト量」になる＝パリティ別に表が取れる。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

frameCt = $80

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
        lda #$0E
        sta COLUP0
        lda #$FF
        sta GRP0            ; 8px バー常時表示（hmoved の根拠を可視化）

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
        inc frameCt
        sta WSYNC           ; VB 1
        ; --- VB行2: P0 を X=60 へ（lda #imm 型 → XCAL=-5 校正系）---
        lda #60
        clc
        adc #-5
        sec
Dv:     sbc #15
        bcs Dv
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 2
        sta HMOVE
        sta WSYNC           ; VB 3（HMOVE 後 24cy を跨いでから HMCLR）
        sta HMCLR           ; HM 全クリア＝以後の HMOVE は「シフト量 0」のはず（HBLANK 内なら）
        ldx #33             ; VBLANK 残り（1+1+1+33+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 可視 0-99 ---
        ldy #100
Top:    sta WSYNC
        dey
        bne Top
        ; --- 行 100: パリティで行中 HMOVE（HM=0）---
        lda frameCt         ; 3
        and #3              ; 5
        beq KNone           ; 7/8
        cmp #1              ; 9
        beq K20             ; 11/12
        cmp #2              ; 13
        beq K40             ; 15/16
        ds 10, $EA          ; k=3: +20cy
K40:    ds 10, $EA          ; k=2: +20cy（k=3 はここを通って +40 に）
K20:    ds 4, $EA           ; k=1: +8cy
        sta HMOVE           ; 行中ストローブ（完了時刻はパリティで 3 段階）
KNone:  sta WSYNC           ; 行 101 へ
        ; --- 可視 101-191 ---
        ldy #91
Bot:    sta WSYNC
        dey
        bne Bot

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+(100+1+91)+30 = 262

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
