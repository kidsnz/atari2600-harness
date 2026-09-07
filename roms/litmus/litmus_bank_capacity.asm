; litmus_bank_capacity — what Andrew Davie's packing rule costs, in stores per scanline.
;
; His 2003 tool 'Banker' imposed a rule and never said what it bought:
;   "a) For any frame, ALL of its sprites must be in a single bank
;    b) The matrix definition for the frame must be in the same bank as the [sprites]"
;   〔stella-list 200301/msg00229〕
;
; bankswitching.md carries the mechanism and no price. Measured 2026-09-07 by growing the
; store count until the frame stopped holding its line count:
;
;   same bank                9 stores per line
;   one switch per line      8            <- the switch pair costs exactly one store
;   a switch per fetch       4            <- less than half the line
;
; That is the numeric reason for rule (a): reaching across banks per sprite halves what a
; scanline can draw, and batching the switch to once per line costs only one store.
;
; NOTE ON THE STAND-IN. A real F8 hotspot access is `sta $1FF9` — a 4-cycle absolute store.
; This ROM is 4K and has no second bank, so it uses `lda $A0,x`, also 4 cycles, also a single
; memory access, with no side effect on a zeroed page. What is measured here is the TIME a
; switch costs, which is the quantity rule (a) is about; the switching itself is covered by
; litmus_bank, litmus_bank_f4 and litmus_bank_f6.
;
; The three bands run at their measured maxima, so the frame is exactly 262 scanlines. Add one
; store to any band and it is not.
        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUP0=$06
COLUBK=$09
RESP0=$10
GRP0=$1B
HMCLR=$2B
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #0
        sta COLUBK
        sta RESP0
Frame:  lda #2
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
        ldx #0

        ; ---- band A: 8 stores, same bank ----
        ldy #31
Arow:   sta WSYNC
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        dey
        bpl Arow

        ; ---- band B: 7 stores, one switch per line ----
        ldy #31
Brow:   sta WSYNC
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda Tbl,y
        sta GRP0
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        dey
        bpl Brow

        ; ---- band C: 4 stores, a switch on each side of every fetch ----
        ldy #31
Crow:   sta WSYNC
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda Tbl,y
        sta GRP0
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda Tbl,y
        sta GRP0
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda Tbl,y
        sta GRP0
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        lda Tbl,y
        sta GRP0
        lda $A0,x           ; 4cy — stands in for a hotspot access; see the note above
        dey
        bpl Crow
        dey
        bpl Crow

        lda #0
        sta GRP0
        lda #2
        sta VBLANK
        ldx #125
OS:     sta WSYNC
        dex
        bne OS
        ldx #0
        jmp Frame
Tbl:    .byte $18,$3C,$7E,$FF,$7E,$3C,$18,$00
        org $FFFC
        .word Start
        .word Start
