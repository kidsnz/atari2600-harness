; litmus_vdel_2lk — 2 行カーネルにおける VDEL の縦整列（+1 行）の実測（U-M11）
; 文書（SpiceWare 流 2LK 実践）: GRP0 を偶数行・GRP1 を奇数行に書く 2LK では、
; VDELP0=1 にすると P0 の表示更新が「次の GRP1 書込（奇数行）」まで遅延し、
; P0 の見かけの開始行が +1 される（P1 と縦が揃う）。litmus_vdel（v0.39.0）の
; レジスタ機構検証を、2LK の行整列という実用形で確かめる。
; 構成: P0(X=40)・P1(X=100)、同一アート 8 行を 2 行/行ペアで描く（バンド行 100-115）。
;   偶数行: GRP0=Art[row] / 奇数行: GRP1=Art[row]
;   フレーム 0-59: VDELP0=0（P0 は偶数行から表示＝行 100 開始）
;   フレーム 60-119: VDELP0=1（P0 は +1 行＝行 101 開始・P1 と揃う）… 以後 120f 周期
; 判定: read_row（行 100 で P0 の有無が位相で変わる・P1 は常に行 101 開始）＋ golden。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
VDELP0  = $25

fc      = $80
phase   = $81       ; 0=VDELP0 off / 1=on
ln      = $82
row     = $83

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$86
        sta COLUP0
        lda #$C6
        sta COLUP1
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        ds 9, $EA           ; +18cy
        sta RESP1

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
        inc fc
        lda fc
        cmp #60
        bcc PhSet
        lda #0
        sta fc
        lda phase
        eor #1
        sta phase
PhSet:  lda phase
        sta VDELP0
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ===== 可視 192 行: バンド行 100-115 に 2LK 書込 =====
        lda #0
        sta ln
KLine:  sta WSYNC
        lda ln
        sec
        sbc #100
        cmp #16
        bcs KOut
        lsr                 ; row = (ln-100)/2, C=奇偶
        tax
        bcs KOdd
        lda Art,x           ; 偶数行: GRP0
        sta GRP0
        jmp KNext
KOdd:   lda Art,x           ; 奇数行: GRP1
        sta GRP1
        jmp KNext
KOut:   lda #0
        sta GRP0
        sta GRP1
KNext:  inc ln
        lda ln
        cmp #192
        bne KLine

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

Art:    byte %11111111
        byte %10000001
        byte %10111101
        byte %10100101
        byte %10100101
        byte %10111101
        byte %10000001
        byte %11111111

        org $FFFC
        .word Start
        .word Start
