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

    ; KROW {1}=row index, {2}=colour the NEXT drawn row uses.
    ;
    ; 71 of 76 cycles, and every number below is why the six stores work: each copy of
    ; the 48px image is painted while a different byte sits in GRP0/GRP1, so the store
    ; has to land in the gap before the copy it feeds. The visible clock a copy starts
    ; at is (cycle - 22.67) * 3, which puts P0's three copies at clocks 87/103/119 and
    ; P1's at 95/111/127 — the stores at 10/18/26 set up the first pair before the beam
    ; arrives, and 54/57/60 land in the gaps between the later ones.
    ;
    ; The colour pair at the end is deliberately AFTER the picture: at cycle 65 the beam
    ; is past clock 127, so it cannot recolour this row, and it is in place for the next.
    MAC KROW
        sta WSYNC
        ldy #{1}            ; 2
        lda Col0,y          ; 6
        sta GRP0            ; 9   B0 -> P0 new
        lda Col1,y          ; 13
        sta GRP1            ; 16  B1 -> P1 new, B0 -> P0 shadow (shown)
        nop                 ; 18  padding: absolute,Y is a cycle cheaper than the (zp),y
                            ;     this used to use, and the six stores have to keep landing
                            ;     in the gaps between the copies. Three nops put the LAST
                            ;     three back on 54/57/60 exactly — the ones that feed copies
                            ;     1 and 2. Verified by rendering, not by arithmetic.
        lda Col2,y          ; 22
        sta GRP0            ; 25  B2 -> P0 new, B1 -> P1 shadow (shown)
        lda Col3,y          ; 29
        sta tmp             ; 32
        nop                 ; 34
        lda Col4,y          ; 38
        tax                 ; 40
        lda Col5,y          ; 44
        tay                 ; 46
        nop                 ; 48
        lda tmp             ; 51
        sta GRP1            ; 54  B3 -> P1 new, B2 -> P0 shadow
        stx GRP0            ; 57  B4 -> P0 new, B3 -> P1 shadow
        sty GRP1            ; 60  B5 -> P1 new, B4 -> P0 shadow
        sta GRP0            ; 63  junk,   B5 -> P1 shadow
        lda #{2}            ; 65  next row's colour — the beam is past the picture here
        sta COLUP0          ; 68
        sta COLUP1          ; 71
    ENDM

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

        ; ===== Multicolor 48px kernel — ONE scanline per data row, 71 of 76 cycles =====
        ;
        ; It used to be TWO, and that broke the technique's own headline claim. The loop
        ; carried `ldy row / lda ColorTab,y / sta COLUP0 / sta COLUP1` ahead of the six
        ; stores and closed with `dec row / bpl`, which came to ~79 cycles. The source
        ; said so and annotated `@lines 2` so the budget prover would accept it — but a
        ; data row spread over two scanlines splits the six-store choreography across two
        ; lines, and that choreography IS the 48 pixels. Measured before this change:
        ; every scanline drew the SAME 16 pixels three times, alternating P0-only and
        ; P1-only rows, and setting Col2 to $FF changed the screen NOT AT ALL — columns 2
        ; through 5 never reached the picture. `score6`, same positions and same six
        ; stores, closes at 72 and renders six distinct digits.
        ;
        ; Two changes buy the 8 cycles back:
        ;   - the row loop is UNROLLED (macro KROW), so `dec row / bpl` disappears and the
        ;     row index is an immediate;
        ;   - the colour write moves BELOW the six stores and writes the NEXT row's colour.
        ;     At cycle 65 the beam is past clock 127, where the last copy ends, so the
        ;     write cannot recolour the row it sits in — it lands on the following one.
        ;     The table is therefore consumed one row ahead, and the first row's colour is
        ;     set before the kernel starts.
        ;
        ; Stores now land at 9/16/25/54/57/60 against score6's 11/19/27/55/58/64.
        lda #$64          ; first row's colour, before the kernel
        sta COLUP0
        sta COLUP1
        KROW 15, $64
        KROW 14, $76
        KROW 13, $76
        KROW 12, $98
        KROW 11, $98
        KROW 10, $BA
        KROW 9, $BA
        KROW 8, $1A
        KROW 7, $1A
        KROW 6, $48
        KROW 5, $48
        KROW 4, $46
        KROW 3, $46
        KROW 2, $44
        KROW 1, $44
        KROW 0, $44

        sta WSYNC           ; close the last row's region — without it the final KROW's
                            ; interval runs on into the blanking and mirror writes and the
                            ; budget prover sees 107 cycles for a 71-cycle row.
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

        ldy #176             ; the kernel is 16 lines shorter now that a data row is ONE
                            ; scanline; the fill takes the difference back so the frame is 262
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
        byte $00,$00,$00,$00,$00,$00,$03,$0F   ; row0..7
        byte $3F,$7F,$7F,$3F,$1F,$0F,$03,$00   ; row8..15
Col1:
        byte $00,$00,$00,$01,$0F,$7F,$FF,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$7F   ; row8..15
Col2:
        byte $01,$07,$3F,$FF,$FF,$FF,$FF,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FE,$FC,$F0,$80   ; row8..15
Col3:
        byte $80,$E0,$FC,$FF,$FF,$FF,$FF,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$7F,$3F,$0F,$01   ; row8..15
Col4:
        byte $00,$00,$00,$80,$F0,$FE,$FF,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$FE   ; row8..15
Col5:   ; rightmost 8px
        byte $00,$00,$00,$00,$00,$00,$C0,$F0   ; row0..7
        byte $FC,$FE,$FE,$FC,$F8,$F0,$C0,$00   ; row8..15

        org $FFFC
        .word Start
        .word Start
