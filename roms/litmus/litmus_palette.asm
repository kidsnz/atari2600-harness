; litmus_palette — 全 128 色の掃引（U-M12: Stella パレット量子化表の実測採取用）
; 行 0-3: 白 $0E マーカー（スナップショットの縦オフセット検出用）
; 行 4-131: COLUBK = (行-4)*2 — 偶数コード $00..$FE を 1 行ずつ
; 行 132+: 黒
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

ln      = $80

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

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
        lda #0
        sta ln
KLine:  sta WSYNC
        lda ln
        cmp #4
        bcs KPal
        lda #$0E            ; マーカー白
        sta COLUBK
        jmp KNext
KPal:   cmp #132
        bcs KBlack
        sec
        sbc #4
        asl                 ; (行-4)*2
        sta COLUBK
        jmp KNext
KBlack: lda #0
        sta COLUBK
KNext:  inc ln
        lda ln
        cmp #192
        bne KLine
        lda #0
        sta COLUBK
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
