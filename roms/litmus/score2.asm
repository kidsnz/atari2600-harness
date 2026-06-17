; score2 — VV-9 litmus: display a 2-digit BCD "score" from RAM so OCR can verify
; that the rendered digits == decode(RAM). The score byte ($80) is packed BCD
; (e.g. $42 shows "4" then "2"). The tens nibble drives the LEFT digit via PF1,
; the ones nibble the RIGHT digit via PF2, each glyph 8 rows tall (8 scanlines
; per row = big, easy-to-OCR blocks). Glyphs need only be DISTINCT patterns —
; the OCR learns its templates from this same font, so it catches font-index /
; BCD-split / display-kernel bugs (the wrong glyph renders) without RAM changing.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
PF1     = $0E
PF2     = $0F
CTRLPF  = $0A
COLUBK  = $09
COLUPF  = $08

SCORE   = $80         ; packed BCD score (RAM)
TENS    = $82         ; tens glyph pointer (lo/hi at $82/$83)
ONES    = $84         ; ones glyph pointer (lo/hi at $84/$85)
ROW     = $86         ; current font row 0..7

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #$42        ; the score to display = "42"
        sta SCORE

        lda #$0E        ; bright playfield, dark background
        sta COLUPF
        lda #$00
        sta COLUBK
        lda #0
        sta CTRLPF      ; no reflect: digits live in the left half

Main:
; --- VSYNC 3 ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
; --- VBLANK 37 ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- compute glyph pointers from the BCD nibbles ---
        lda SCORE
        and #$0F        ; ones nibble
        asl
        asl
        asl             ; *8 (8 bytes per glyph)
        clc
        adc #<FONT
        sta ONES
        lda #>FONT
        adc #0
        sta ONES+1

        lda SCORE
        lsr
        lsr
        lsr
        lsr             ; tens nibble
        asl
        asl
        asl             ; *8
        clc
        adc #<FONT
        sta TENS
        lda #>FONT
        adc #0
        sta TENS+1

; --- visible 192: 64 top blank, 64 digit band (8 rows x 8 lines), 64 bottom ---
        ldx #64
TopPad: sta WSYNC
        dex
        bne TopPad

        lda #0
        sta ROW
Band:
        ldy ROW
        lda (TENS),y
        sta PF1
        lda (ONES),y
        sta PF2
        ; hold this glyph row for 8 scanlines
        ldx #8
Hold:   sta WSYNC
        dex
        bne Hold
        lda #0
        sta PF1
        sta PF2
        inc ROW
        lda ROW
        cmp #8
        bne Band

        ldx #64
BotPad: sta WSYNC
        dex
        bne BotPad

; --- overscan 30 ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Main

; --- font: 10 glyphs x 8 rows. Distinct, digit-ish bit patterns. ---
FONT:
        .byte $3C,$42,$46,$5A,$62,$42,$3C,$00 ; 0
        .byte $08,$18,$08,$08,$08,$08,$1C,$00 ; 1
        .byte $3C,$42,$02,$0C,$30,$40,$7E,$00 ; 2
        .byte $3C,$42,$02,$1C,$02,$42,$3C,$00 ; 3
        .byte $0C,$14,$24,$44,$7E,$04,$04,$00 ; 4
        .byte $7E,$40,$7C,$02,$02,$42,$3C,$00 ; 5
        .byte $1C,$20,$40,$7C,$42,$42,$3C,$00 ; 6
        .byte $7E,$02,$04,$08,$10,$10,$10,$00 ; 7
        .byte $3C,$42,$42,$3C,$42,$42,$3C,$00 ; 8
        .byte $3C,$42,$42,$3E,$02,$04,$38,$00 ; 9

        org $FFFC
        .word Reset
        .word Reset
