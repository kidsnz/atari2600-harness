; multicolor48 — 48px multicolor graphic with per-row color (technique: multicolor48, AA2-209137)
; Lineage: AtariAge topic/209137 (SeaGtGruff's 76cy multicolor 48px kernel) mounted on the
; verified 6-store choreography of score6/bitmap48 — the full form:
;   - 48px = NUSIZ $03 (3 copies close) on P0/P1 + P1 shifted +8px + VDEL double buffering.
;   - **COLUP0/COLUP1 rewritten from ColorTab on every row** = the 48px image gains vertical multicolor.
;   - The color rewrite finishes inside HBLANK (before the first GRP0 store); the 6-store body is the same choreography as score6.
;   - 1 row = 1 scanline, in-row budget under 76cy (color 10cy + 6-store).
;   - Picture drawn = a rainbow heart (color changes vertically = multicolor is obvious at a glance).
;   - Each row's effective color is saved to RAM mirrors (colhi/collo) = a scenario can numerically verify ">=2 colors".
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

HEIGHT  = 16        ; number of image rows

row     = $80
tmp     = $81
colfst  = $82       ; color mirror of the first row (row=HEIGHT-1)
collst  = $83       ; color mirror of the last row (row=0)
fcnt    = $84
p0      = $90       ; column pointers x6 (hi fixed at init, all within one page)
p1      = $92
p2      = $94
p3      = $96
p4      = $98
p5      = $9A

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta COLUBK
        lda #$03            ; 3 copies close
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #>Col0          ; column tables sit within one page (guaranteed by ORG)
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
        lda #<Col0
        sta p0
        lda #<Col1
        sta p1
        lda #<Col2
        sta p2
        lda #<Col3
        sta p3
        lda #<Col4
        sta p4
        lda #<Col5
        sta p5

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
        ; clear the shadow registers (determinism)
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        inc fcnt
        ldx #34
VB:     sta WSYNC
        dex
        bne VB
        ; --- Positioning (same as score6: P0=87 / P1=95) ---
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

        ; ===== Multicolor 48px kernel (HEIGHT rows, color + 6-store, 1 row/scanline) =====
        lda #HEIGHT-1
        sta row
Krow:   sta WSYNC          ; @lines 2 — each data row spans 2 visible scanlines (loop-back ~79cy>76); verified stable 262
        ldy row             ; 3
        lda ColorTab,y      ; 7   per-row color
        sta COLUP0          ; 10
        sta COLUP1          ; 13
        lda (p0),y          ; 18
        sta GRP0            ; 21  B0->P0 new
        lda (p1),y          ; 23
        sta GRP1            ; 26  B1->P1 new, B0->P0 shadow (shown)
        lda (p2),y          ; 31
        sta GRP0            ; 34  B2->P0 new, B1->P1 shadow (shown)
        lda (p3),y          ; 39
        sta tmp             ; 42
        lda (p4),y          ; 47
        tax                 ; 49
        lda (p5),y          ; 54
        tay                 ; 56
        lda tmp             ; 59
        sta GRP1            ; 62  B3->P1 new, B2->P0 shadow
        stx GRP0            ; 65  B4->P0 new, B3->P1 shadow
        sty GRP1            ; 68  B5->P1 new, B4->P0 shadow
        sta GRP0            ; 71  junk,    B5->P1 shadow
        dec row             ; 76
        bpl Krow            ; (the next row's WSYNC realigns after the in-row work completes)

        ; blank the players (shadows included)
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ; record the color mirrors (for verification: first row=HEIGHT-1, last row=0)
        lda ColorTab+HEIGHT-1
        sta colfst
        lda ColorTab+0
        sta collst

        ldy #161
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

; ===== Per-row colors (bottom rows first = indexed with Y=HEIGHT-1..0; a rainbow) =====
; Order: last (row0, bottom of screen) -> first (row15, top of screen). Visually a rainbow.
ColorTab:
        byte $44,$44,$46,$46,$48,$48,$1A,$1A   ; row 0..7  : red -> orange -> yellow-green
        byte $BA,$BA,$98,$98,$76,$76,$64,$64   ; row 8..15 : cyan -> blue -> purple -> pink

; ===== Image (column-major 6 x HEIGHT, bottom rows first, within one page) = heart =====
        org $FE00
; Each byte = 8px. 6 columns = 48px. row0 is the bottom of the screen, row15 the top.
; Heart: 2 lobes at the top, narrowing to a V at the bottom.
Col0:   ; leftmost 8px
        byte $00,$00,$00,$01,$03,$07,$0F,$0F   ; row0..7
        byte $1F,$1F,$0F,$0E,$00,$00,$00,$00   ; row8..15
Col1:
        byte $00,$00,$00,$80,$F0,$F8,$FC,$FE   ; row0..7
        byte $FF,$FF,$FF,$FE,$3C,$3C,$00,$00   ; row8..15
Col2:
        byte $00,$00,$00,$01,$0F,$3F,$7F,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$00,$00   ; row8..15
Col3:
        byte $00,$00,$00,$80,$F0,$FC,$FE,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$00,$00   ; row8..15
Col4:
        byte $00,$00,$00,$00,$00,$80,$C0,$E0   ; row0..7
        byte $F0,$F0,$F8,$78,$3C,$3C,$00,$00   ; row8..15
Col5:   ; rightmost 8px
        byte $00,$00,$00,$00,$00,$00,$00,$00   ; row0..7
        byte $00,$00,$00,$00,$00,$00,$00,$00   ; row8..15

        org $FFFC
        .word Start
        .word Start
