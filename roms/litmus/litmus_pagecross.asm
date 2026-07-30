; litmus_pagecross — the +1 page-cross cycle on an indexed READ, with a CONSTANT index.
;
; 出典: 自前の検証根拠。2026-07-30 に cyclebound の pagePenalty が過小評価していたのを
; 捕まえた fixture であり、その唯一の証人。コーパス 123 本のどれも「定数索引でページを
; 跨ぐ読み出し」を持たないので、この ROM が無いと修正が効いていることを示せない。
;
; 仕掛け: 2 つの WSYNC 領域が、それぞれ 4 回ずつ `lda Tbl,y`（y は定数 200）を実行する。
;   NearTbl は $F100 に置く → $F100+200 = $F1C8 ＝ base と同じページ → ペナルティなし
;   FarTbl  は $F0F8 に置く → $F0F8+200 = $F1C0 ＝ base の次のページ → 1 命令あたり +1
; 検証は「同じ ROM の FAR 領域の値が修正前後でどう変わるか」で行う。実測: FarRow 35 → 39
; （跨ぐ読み 4 本 × +1）、NearRow は 30 のまま。
; 注意: NEAR と FAR の絶対値を直接引き算しないこと。2 つの領域は命令列こそ同じでも配置が
; 違い、分岐のページ跨ぎ等で数サイクルずれる（修正前でも 30 対 35 で、差は 5 あった）。
; ここで意味があるのは同一領域の前後比較だけ。
;
; 旧実装は base+Lo と base+Hi を比べていた＝定数索引では常に等しく、ペナルティが 0 になった。
; 正しい条件は base のページと base+Hi のページの比較。固定 base に対して跨ぎは索引について
; 単調なので、最悪値は Hi だけで決まる。

        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- NEAR: 96 lines, table stays inside its own page ---
        ldx #96
NearRow:
        sta WSYNC
        ldy #200
        lda NearTbl,y
        sta COLUBK
        lda NearTbl,y
        lda NearTbl,y
        lda NearTbl,y
        dex
        bne NearRow

        ; --- FAR: 96 lines, identical code, table reaches the next page ---
        ldx #96
FarRow:
        sta WSYNC
        ldy #200
        lda FarTbl,y
        sta COLUBK
        lda FarTbl,y
        lda FarTbl,y
        lda FarTbl,y
        dex
        bne FarRow

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Main

        org $F0F8
FarTbl: ds 8, $1E          ; $F0F8..$F0FF — the read at +200 lands at $F1C0
        org $F100
NearTbl: ds 256, $3E       ; $F100..$F1FF — the read at +200 lands at $F1C8

        org $FFFC
        .word Start
        .word Start
