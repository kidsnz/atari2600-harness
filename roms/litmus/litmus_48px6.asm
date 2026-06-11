; litmus_48px6 — 48px の【動的6書換】（VDEL 6-store kernel, V2-13 完結編）
; 各コピーに別バイトを表示: B0=$AA B1=$55 B2=$F0 B3=$0F B4=$CC B5=$33。
; 位置決め=litmus_48px と同一（P0=24, P1=+8）。VDELP0/P1=1（V2-1 で裏取りした影コピーを利用）。
; 毎行の振付（WSYNC=cycle0）:
;   preload: B0→GRP0, B1→GRP1(B0→P0影), B2→GRP0(B1→P1影), A/X/Y←B3/B4/B5  …21cy
;   SLEEP10 → 4連続ストア（完了 cycle 34/37/40/43）:
;     sta GRP1(B3; B2→P0影) / stx GRP0(B4; B3→P1影) / sty GRP1(B5; B4→P0影) / sta GRP0(junk; B5→P1影)
;   各 swap がコピー間の隙間（P0c1終33.0→c2開始36.0 等）に着地する設計。
; 実機裏取り済（Gopher2600, v0.52.0, read_row(8) 一発成功）: 全6コピーが設計バイト通り —
;  $AA→24,26,28,30 / $55→33,35,37,39 / $F0→40-43 / $0F→52-55 / $CC→56,57,60,61 / $33→66,67,70,71。
;  ＝VDEL 6-store 振付（ストア完了 34/37/40/43）が正確。回帰固定=scenarios/p48_6.json（golden）。
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
VDELP0  = $25
VDELP1  = $26

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
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1

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
        ldx #35
VB:     sta WSYNC
        dex
        bne VB
        ; 位置決め（litmus_48px と同一レシピ・VBLANK 内 2 行）
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
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

        ; --- 6書換カーネル 16 行 ---
        ldy #16
Krow:   sty $80             ; 行カウンタ退避（Y を B5 に使う）
        sta WSYNC
        lda #$AA            ; 2
        sta GRP0            ; 5   B0→P0新
        lda #$55            ; 7
        sta GRP1            ; 10  B1→P1新, B0→P0影
        lda #$F0            ; 12
        sta GRP0            ; 15  B2→P0新, B1→P1影
        lda #$0F            ; 17  A=B3
        ldx #$CC            ; 19  X=B4
        ldy #$33            ; 21  Y=B5
        ds 5, $EA           ; 31  SLEEP 10
        sta GRP1            ; 34  B3→P1新, B2→P0影（P0c1終33.0→c2開36.0 の隙間）
        stx GRP0            ; 37  B4→P0新, B3→P1影
        sty GRP1            ; 40  B5→P1新, B4→P0影
        sta GRP0            ; 43  junk,    B5→P1影
        ldy $80
        dey
        bne Krow

        ; 消灯（影まで）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ldy #176
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

        org $FFFC
        .word Start
        .word Start
