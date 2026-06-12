; score6 — 6桁BCDスコアカーネル（technique: score-kernel, U-M1）
; litmus_48px6（VDEL 6-store・実機裏取り済 v0.52.0）の振付を (zp),y フォント参照に拡張した
; 「全ゲーム共通のスコア表示」完全形:
;   ・スコア = BCD 3バイト（score0=上位2桁 … score2=下位2桁）。毎フレーム +1（デモ）。
;   ・VBLANK で 6 桁ぶんのフォントポインタ（Font + digit*8）を組む。
;   ・可視 8 行を 6-store 振付で描画: ストア完了 55/58/61/64cy（litmus の 34/37/40/43 から
;     +21cy）→ 位置も +63px 平行移動で P0=87/P1=95（振付の隙間関係は相似のまま保たれる）。
;   ・フォントは 6px 幅＋右2px 空白（コピーが 8px ピッチで隣接するため間隔をフォント側に内蔵）。
;     メモリ上は下の行が先（kernel は Y=7→0 で参照）。
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
VDELP0  = $25
VDELP1  = $26
HMOVE   = $2A
HMCLR   = $2B

score0  = $80       ; BCD 上位2桁
score1  = $81
score2  = $82       ; BCD 下位2桁
row     = $83
tmp     = $84
p0      = $90       ; フォントポインタ ×6（lo のみ毎フレーム更新・hi は初期化で固定）
p1      = $92
p2      = $94
p3      = $96
p4      = $98
p5      = $9A

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
        sta COLUP1
        lda #$03            ; 3 copies close
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #>Font          ; ポインタ hi は固定（フォントは 1 ページ内）
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1

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
        ; 影クリア（決定化）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ; --- スコア +1（BCD・3バイト桁上げ） ---
        sed
        clc
        lda score2
        adc #1
        sta score2
        lda score1
        adc #0
        sta score1
        lda score0
        adc #0
        sta score0
        cld
        ; --- 6 桁のフォントポインタ（Font + digit*8） ---
        lda score0
        and #$F0
        lsr                 ; 上位 nibble<<4 → >>1 = digit*8
        sta p0
        lda score0
        and #$0F
        asl
        asl
        asl
        sta p1
        lda score1
        and #$F0
        lsr
        sta p2
        lda score1
        and #$0F
        asl
        asl
        asl
        sta p3
        lda score2
        and #$F0
        lsr
        sta p4
        lda score2
        and #$0F
        asl
        asl
        asl
        sta p5
        ldx #33
VB:     sta WSYNC
        dex
        bne VB
        ; --- 位置決め（litmus_48px レシピ +21cy = +63px → P0=87, P1=95） ---
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        ds 9, $EA           ; SLEEP 18
        lda $80             ; +3 = 21cy
        sta RESP0
        sta RESP1
        lda #$10
        sta HMP1
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        lda #0
        sta VBLANK

        ; --- スコア 8 行（6-store 振付・ストア完了 55/58/61/64cy） ---
        lda #7
        sta row
Krow:   sta WSYNC
        ldy row             ; 3
        lda (p0),y          ; 8
        sta GRP0            ; 11  B0→P0新
        lda (p1),y          ; 16
        sta GRP1            ; 19  B1→P1新, B0→P0影(表示)
        lda (p2),y          ; 24
        sta GRP0            ; 27  B2→P0新, B1→P1影(表示)
        lda (p3),y          ; 32
        sta tmp             ; 35
        lda (p4),y          ; 40
        tax                 ; 42
        lda (p5),y          ; 47
        tay                 ; 49
        lda tmp             ; 52
        sta GRP1            ; 55  B3→P1新, B2→P0影（P0c1終→c2開の隙間）
        stx GRP0            ; 58  B4→P0新, B3→P1影
        sty GRP1            ; 61  B5→P1新, B4→P0影
        sta GRP0            ; 64  junk,    B5→P1影
        dec row             ; 69
        bpl Krow            ; 72（<76 で行内完結）

        ; 消灯（影まで）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ldy #184
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; --- フォント 0-9（8 行/桁・6px 幅＋右2px 空白・**下の行から**格納） ---
        org $FE00
Font:
        byte $78,$CC,$CC,$CC,$CC,$CC,$CC,$78   ; 0（上下対称なので向き不問）
        byte $FC,$30,$30,$30,$30,$30,$70,$30   ; 1
        byte $FC,$C0,$60,$30,$18,$0C,$CC,$78   ; 2
        byte $78,$CC,$0C,$0C,$38,$0C,$CC,$78   ; 3
        byte $0C,$0C,$0C,$FC,$CC,$6C,$3C,$1C   ; 4
        byte $78,$CC,$0C,$0C,$F8,$C0,$C0,$FC   ; 5
        byte $78,$CC,$CC,$CC,$F8,$C0,$60,$38   ; 6
        byte $60,$60,$60,$60,$30,$18,$0C,$FC   ; 7
        byte $78,$CC,$CC,$CC,$78,$CC,$CC,$78   ; 8
        byte $70,$18,$0C,$7C,$CC,$CC,$CC,$78   ; 9

        org $FFFC
        .word Start
        .word Start
