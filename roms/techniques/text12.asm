; text12 — 12 文字テキスト行（technique: text12, AA2-B2, v1.58.0）
; 系譜: David Crane の 12 文字/行ルーチン（Basic Programming 1979）の現代版を自前実装。
; 方式: score6 と同一の 48px VDEL 6-store 振付に、**4×5 フォントを 2 文字/バイトに合成**した
; RAM バッファ（列優先 6 列 × 5 行 = 30 バイト/行）を流す。フリッカーレス 12 文字。
;   ・フォントは下位ニブル 4bit（bit3=最左）。VBLANK で (左<<4 | 右) に合成
;   ・グリフは下の行から格納（kernel は Y=4..0 降順）
;   ・行は 2 走査線ずつ（4×5 フォント → 12 文字 × 10 走査線/テキスト行）
; デモ: 上段 "HELLO WORLD!" 下段 "ATARI 2600.."。
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
HMP1    = $21
VDELP0  = $25
VDELP1  = $26
HMOVE   = $2A
HMCLR   = $2B
TIM64T  = $0296
INTIM   = $0284

row     = $80
tmp     = $81       ; 汎用（カーネルの 6-store 用）
strPtr  = $82       ; BuildBuf: 文字列ポインタ（2B $82-83）
outBase = $84       ; BuildBuf: 出力 zp 先頭
fL      = $85       ; 左フォント基点
fR      = $86       ; 右フォント基点
rowT    = $87       ; 現在行
nib     = $88       ; 左ニブル合成用
val     = $89       ; 合成値
pairIx  = $8A
bufA    = $90       ; 上段バッファ 30B（列優先: col*5+row）$90-$AD
bufB    = $B0       ; 下段バッファ 30B $B0-$CD
p0      = $D0       ; カーネル用ポインタ ×6（2B each）$D0-$DB
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
        lda #$03            ; 3 copies close
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        ; --- バッファ展開（静的文字列なので一度だけ）---
        lda #<TextA
        sta strPtr
        lda #>TextA
        sta strPtr+1
        ldy #<bufA
        jsr BuildBuf
        lda #<TextB
        sta strPtr
        lda #>TextB
        sta strPtr+1
        ldy #<bufB
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
        ; ポインタ hi（ゼロページ）は 0 固定
        lda #0
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
VBwait: lda INTIM
        bne VBwait
        ; --- 48px 位置決め（score6 と同一校正: P0=87/P1=95）---
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

        ; ===== 上段テキスト（10 行） =====
        ldx #<bufA
        jsr TextLine
        ; ギャップ 8 行
        ldy #8
G1:     sta WSYNC
        dey
        bne G1
        ; ===== 下段テキスト（10 行） =====
        ldx #<bufB
        jsr TextLine

        ; 残り可視（192 - 2 - 10 - 8 - 10 - PosObj2 = 162）+ 帳尻は OS 側で調整
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ldy #161
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

; ===== TextLine: X=バッファ先頭（zp オフセット）。5 行×2 走査線の 6-store =====
TextLine:
        txa                 ; 6 ポインタ = buf + col*5
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
KrowA:  sta WSYNC           ; ---- 1 行目 ----
        ldy row             ; 3
        lda (p0),y          ; 8
        sta GRP0            ; 11
        lda (p1),y          ; 16
        sta GRP1            ; 19
        lda (p2),y          ; 24
        sta GRP0            ; 27
        lda (p3),y          ; 32
        sta tmp             ; 35
        lda (p4),y          ; 40
        tax                 ; 42
        lda (p5),y          ; 47
        tay                 ; 49
        lda tmp             ; 52
        sta GRP1            ; 55
        stx GRP0            ; 58
        sty GRP1            ; 61
        sta GRP0            ; 64
        sta WSYNC           ; ---- 2 行目（同一データ再ストア）----
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
        dec row             ; 69
        bpl KrowA           ; 72
        lda #0              ; 消灯（影まで）
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

TextA:  byte $08,$05,$0C,$0C,$0F,$00,$17,$0F,$12,$0C,$04,$25   ; HELLO WORLD!
TextB:  byte $01,$14,$01,$12,$09,$00,$1D,$21,$1B,$1B,$26,$26   ; ATARI 2600..

; ===== 4×5 フォント（下の行から・下位ニブル・bit3=最左） =====
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
