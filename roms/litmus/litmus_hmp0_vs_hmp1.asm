; litmus_hmp0_vs_hmp1 — do HMP0 and HMP1 mean the same thing?
;
; Brad Mott, 1998, in one sentence and never corroborated in the archive:
;   "The only bad thing I see is that the meaning of HMP0 isn't quite the same as it is for
;    HMP1.  When the high nibble of HMP0 is F it moves P0 by -8 instead of -7 :-("
;   〔stella-list 199804/msg00193〕
;
; known-traps.md already records that mid-line HMOVE behaviour differs by TIA REVISION. This is a
; different axis: the same chip, the two players disagreeing with each other.
;
; Both players are placed identically, then given the SAME HM nibble and one HMOVE inside HBLANK.
; If the two registers mean the same thing the players stay level; if Mott is right, $F separates
; them by one pixel and the other nibbles do not.
;
;  The nibble lives in RAM $81 so the harness can sweep all sixteen values by poking it.
;  $00 is the control that must leave both players exactly where RESPx put them.
;
; The reading is `tia.player0.hmoved_pixel` against `tia.player1.hmoved_pixel`, so nothing depends
; on what the picture looks like.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
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
hmval   = $81      ; the value written to BOTH HMP0 and HMP1; poke it to sweep

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
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #$B6
        sta COLUP1
        lda #$00
        sta COLUBK
        lda #$FF
        sta GRP0
        sta GRP1

Frame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK

        ; Place both players at the same coarse position on the same line, so any later
        ; difference is the HM nibble's doing and not the strobe's.
        sta WSYNC
        ldx #10
Pa:     dex
        bne Pa
        sta RESP0
        sta RESP1
        sta WSYNC
        sta HMCLR

        ; The nibble comes from RAM so the harness can sweep all sixteen by poking $81.
        lda hmval
        sta HMP0
        sta HMP1
        sta WSYNC
        sta HMOVE           ; inside HBLANK, the ordinary way
        ldx #40
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ldx #180
Vis:    sta WSYNC
        dex
        bne Vis

        lda #2
        sta VBLANK
        sta HMCLR
        ldx #34
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
