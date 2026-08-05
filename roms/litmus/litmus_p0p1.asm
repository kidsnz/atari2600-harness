; litmus_p0p1 — a character up to 16px wide by joining P0+P1 (hardening-roadmap S-3 flagship)
; A 16-wide, 8-row design is split by pkg/sprite.SplitWide into P0 (left 8) and P1 (right 8), and P1 is placed adjacent at P0 +8px.
; With no seam a solid row comes out as a continuous 16px of white (1 run in read_row). P0/P1 are the same colour (white) so the seam continuity is what gets tested.
; Design (X = lit):
;   row0 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16 (seam test)
;   row1 XXXXXXXX........  P0=$FF P1=$00  P0 only
;   row2 ........XXXXXXXX  P0=$00 P1=$FF  P1 only
;   row3 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
;   row4 X..............X  P0=$80 P1=$01  both ends only
;   row5 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
;   row6 ........XXXXXXXX  P0=$00 P1=$FF  P1 only
;   row7 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
; Positioning: strobing RESP0→RESP1 3cy apart gives P1=P0+9px; HMP1=$10 (left 1) plus HMOVE pulls it in to P1=P0+8.
; Note: a strobe during HBLANK is clamped to the far left and the position collapses → delay until the beam is in the visible area, then strobe.
; HMOVE is done during VBLANK (so no comb appears in the visible area).
; Hardware-verified (Gopher2600): read_tia gives player0=69 / player1=77 (= exactly +8).
;   read_row(visible 96, a solid16 row)=clock 69-84 white, len16, one continuous run = zero seam.
;   (97)=left 8px (P0) / (98)=right 8px (P1, continuous from 77) / (100)=1px at each end. The joined P0+P1 16px holds with no seam.
;   Regression-locked = roms/litmus/scenarios/p0p1.json (position asserts 69/77 + golden).
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
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B

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
        lda #$0E          ; both P0 and P1 white (so seam continuity can be checked)
        sta COLUP0
        sta COLUP1
        lda #0
        sta NUSIZ0
        sta NUSIZ1
        sta COLUBK

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ; --- position within the first 2 of the 37 VBLANK lines ---
        ; A strobe during HBLANK is clamped to the far left (the position collapses) → delay into the visible area, then strobe.
        sta WSYNC         ; VBLANK line 1: beam=clock-68
        ldy #8
DelayP: dey
        bne DelayP        ; burns ~39cy → beam into the visible area
        sta RESP0         ; place P0 in the visible area
        sta RESP1         ; 3cy later → P1 = P0 + 9px
        lda #0
        sta HMP0
        lda #$10
        sta HMP1          ; P1 left 1 → +8
        sta WSYNC         ; VBLANK line 2
        sta HMOVE         ; applied right after WSYNC (the comb lands inside VBLANK)
        ldx #35
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

        ldx #192          ; 192 visible lines
Visible:
        sta WSYNC
        lda Gfx0Line-1,x
        sta GRP0
        lda Gfx1Line-1,x
        sta GRP1
        dex
        bne Visible
        lda #0
        sta GRP0
        sta GRP1

        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame

; GfxnLine[k] = the GRP for visible line (191-k). The sprite occupies (in kernel terms) visible 88..95 = Gopher2600 visible 96..103.
; The table runs bottom→top (idx96=row7 … idx103=row0).
;   P0 rows: FF FF 00 FF 80 FF 00 FF (row0..7) → reversed: FF 00 FF 80 FF 00 FF FF
;   P1 rows: FF 00 FF FF 01 FF FF FF (row0..7) → reversed: FF FF FF 01 FF FF 00 FF
Gfx0Line:
        ds 96, 0
        .byte $FF,$00,$FF,$80,$FF,$00,$FF,$FF
        ds 88, 0
Gfx1Line:
        ds 96, 0
        .byte $FF,$FF,$FF,$01,$FF,$FF,$00,$FF
        ds 88, 0

        org $FFFC
        .word Start
        .word Start
