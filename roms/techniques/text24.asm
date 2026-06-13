; text24 — 24 文字テキスト @50%フリッカー（technique: text24, U-AR ⑩, za2600 参照）
; 系譜: za2600（Zelda 移植）の text24.asm を読み解いた自前実装。
; 方式: text12（フリッカーレス12・v1.58.0）の 6-store 振付を **2フレーム交互**に走らせ、
;   ・偶数フレーム= 前半12字を**左ブロック**（P0≈39）
;   ・奇数フレーム= 後半12字を**右ブロック**（P0≈87, 左+48px）
;   → 1 行に 24 文字が左→右に並ぶ（各ブロックは隔フレーム＝50%フリッカー・supercat 2群方式）。
; text12 と同じ 4×5 フォント・列優先バッファ・(zp),y 6-store。NUSIZ は close のまま
;   （24字ぶんの横幅は 2 フレームの 8px オフセットで確保）。
; デモ: "ABCDEFGHIJKLMNOPQRSTUVWX"（24字）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
VDELP0  = $25
VDELP1  = $26
HMOVE   = $2A
HMCLR   = $2B
TIM64T  = $0296
INTIM   = $0284

frame   = $80
row     = $81
tmp     = $82
strPtr  = $83
outBase = $85
fL      = $86
fR      = $87
rowT    = $88
nib     = $89
val     = $8A
pairIx  = $8B
bufE    = $90       ; 偶数フレーム 12字（列優先 30B）$90-$AD
bufO    = $B0       ; 奇数フレーム 12字 30B $B0-$CD
p0      = $D0
p1      = $D2
p2      = $D4
p3      = $D6
p4      = $D8
p5      = $DA

        org $F000
Start:  sei
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
        lda #$03
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        ; --- バッファ展開（偶 index / 奇 index に分割）---
        lda #<TextEven
        sta strPtr
        lda #>TextEven
        sta strPtr+1
        ldy #<bufE
        jsr BuildBuf
        lda #<TextOdd
        sta strPtr
        lda #>TextOdd
        sta strPtr+1
        ldy #<bufO
        jsr BuildBuf

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
        lda #43
        sta TIM64T
        inc frame
        lda #0
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
VBwait: lda INTIM
        bne VBwait
        ; --- 位置決め: 偶=左ブロック(P0≈39) / 奇=右ブロック(P0≈87) ---
        sta WSYNC
        ds 5, $EA           ; SLEEP 10（共通プロローグ）
        lda frame
        and #1
        bne PosRight
        ds 4, $EA           ; 偶: 追加 SLEEP 8 → 計 ~5cy 後 RESP（左）
        jmp PosDo
PosRight:
        ds 13, $EA          ; 奇: 追加で右 48px（実測校正: P0 39→87）
PosDo:  lda $80
        sta RESP0
        sta RESP1
        lda #$10            ; P1 を P0+8
        sta HMP1
        lda #0
        sta HMP0
        sta WSYNC
        sta HMOVE
        ds 12, $EA
        sta HMCLR
        lda #0
        sta VBLANK

        ; ===== テキスト行（フレームでバッファ選択）=====
        lda frame
        and #1
        beq UseEven
        ldx #<bufO
        bne DrawLine
UseEven:
        ldx #<bufE
DrawLine:
        jsr TextLine

        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ldy #180
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ===== TextLine（text12 と同一）=====
TextLine:
        txa
        sta p0
        clc
        adc #5
        sta p1
        adc #5
        sta p2
        adc #5
        sta p3
        adc #5
        sta p4
        adc #5
        sta p5
        lda #4
        sta row
KrowA:  sta WSYNC
        ldy row
        lda (p0),y
        sta GRP0
        lda (p1),y
        sta GRP1
        lda (p2),y
        sta GRP0
        lda (p3),y
        sta tmp
        lda (p4),y
        tax
        lda (p5),y
        tay
        lda tmp
        sta GRP1
        stx GRP0
        sty GRP1
        sta GRP0
        sta WSYNC
        ldy row
        lda (p0),y
        sta GRP0
        lda (p1),y
        sta GRP1
        lda (p2),y
        sta GRP0
        lda (p3),y
        sta tmp
        lda (p4),y
        tax
        lda (p5),y
        tay
        lda tmp
        sta GRP1
        stx GRP0
        sty GRP1
        sta GRP0
        dec row
        bpl KrowA
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        rts

; ===== BuildBuf: strPtr=文字列(12B グリフ番号), Y=出力 zp 先頭。列優先 30B を合成 =====
; buf[pair*5+row] = Font[strL*5+row]<<4 | Font[strR*5+row]
BuildBuf:
        sty outBase
        lda #0
        sta pairIx
BBpair: lda pairIx
        asl                 ; 文字 index = pair*2
        tay
        lda (strPtr),y      ; 左の文字（グリフ番号）
        tax
        lda Mul5,x
        sta fL
        iny
        lda (strPtr),y      ; 右
        tax
        lda Mul5,x
        sta fR
        ldy #4
BBrow:  sty rowT
        lda fL
        clc
        adc rowT
        tax
        lda Font,x          ; 左 4bit → 上位ニブルへ
        asl
        asl
        asl
        asl
        sta nib
        lda fR
        clc
        adc rowT
        tax
        lda Font,x          ; 右 4bit
        ora nib
        sta val
        ldx pairIx          ; 出力先 = outBase + pair*5 + row
        lda Mul5,x
        clc
        adc outBase
        adc rowT
        tax
        lda val
        sta $00,x
        ldy rowT
        dey
        bpl BBrow
        inc pairIx
        lda pairIx
        cmp #6
        bcc BBpair
        rts


Mul5:   byte 0,5,10,15,20,25,30,35,40,45,50,55,60,65,70,75,80,85,90,95
        byte 100,105,110,115,120,125,130,135,140,145,150,155,160,165,170,175,180,185,190,195

; 24字 "ABCDEFGHIJKLMNOPQRSTUVWX" を偶index/奇indexに分割（グリフ番号: A=1..）
; A=1 B=2 ... X=24
TextEven: byte 1,2,3,4,5,6,7,8,9,10,11,12       ; ABCDEFGHIJKL（左ブロック）
TextOdd:  byte 13,14,15,16,17,18,19,20,21,22,23,24 ; MNOPQRSTUVWX（右ブロック）

Font:
        byte $0,$0,$0,$0,$0   ; space
        byte $9,$9,$F,$9,$6   ; A
        byte $E,$9,$E,$9,$E   ; B
        byte $7,$8,$8,$8,$7   ; C
        byte $E,$9,$9,$9,$E   ; D
        byte $F,$8,$E,$8,$F   ; E
        byte $8,$8,$E,$8,$F   ; F
        byte $7,$9,$B,$8,$7   ; G
        byte $9,$9,$F,$9,$9   ; H
        byte $7,$2,$2,$2,$7   ; I
        byte $6,$9,$1,$1,$1   ; J
        byte $9,$A,$C,$A,$9   ; K
        byte $F,$8,$8,$8,$8   ; L
        byte $9,$9,$F,$F,$9   ; M
        byte $9,$9,$B,$D,$9   ; N
        byte $6,$9,$9,$9,$6   ; O
        byte $8,$8,$E,$9,$E   ; P
        byte $5,$A,$9,$9,$6   ; Q
        byte $9,$A,$E,$9,$E   ; R
        byte $E,$1,$6,$8,$7   ; S
        byte $2,$2,$2,$2,$7   ; T
        byte $6,$9,$9,$9,$9   ; U
        byte $6,$6,$9,$9,$9   ; V
        byte $9,$F,$F,$9,$9   ; W
        byte $9,$6,$6,$6,$9   ; X
        byte $2,$2,$2,$5,$5   ; Y
        byte $F,$8,$6,$1,$F   ; Z
        byte $6,$D,$9,$B,$6   ; 0
        byte $7,$2,$2,$6,$2   ; 1
        byte $F,$4,$2,$9,$6   ; 2
        byte $E,$1,$6,$1,$E   ; 3
        byte $1,$1,$F,$9,$9   ; 4
        byte $E,$1,$E,$8,$F   ; 5
        byte $6,$9,$E,$8,$6   ; 6
        byte $4,$4,$2,$1,$F   ; 7
        byte $6,$9,$6,$9,$6   ; 8
        byte $6,$1,$7,$9,$6   ; 9
        byte $2,$0,$2,$2,$2   ; bang
        byte $2,$0,$0,$0,$0   ; dot

        org $FFFC
        .word Start
        .word Start
