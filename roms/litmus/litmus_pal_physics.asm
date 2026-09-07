; litmus_pal_physics — does the same ROM move the same pixels PER FRAME on PAL and NTSC?
;
; subpixel-velocity.md already carries the linear conversion: a PAL increment must be 83.39%
; of the NTSC one to travel the same distance per SECOND. What it does not carry is the
; consequence for anything that accelerates, and what nothing here had checked is the premise
; underneath both: that a ROM's per-frame arithmetic is untouched by the television standard.
;
; A 2004 author found out the hard way, shipping a PAL60 build rather than retune:
;   "THE GRAVITY IN THE NTSC VERSION IS EFFECTIVELY 1.4x GREATER. IT'S THE ONE CONSTANT I
;    COULDN'T CHANGE"  〔stella-list 200409/msg00309〕
; 1.4 is not a coincidence: with `vel += g` and `pos += vel` every frame, distance goes as the
; SQUARE of the frame count, and (60/50)^2 = 1.44.
;
; Two objects, both 8.8 fixed point, both stepped once per frame:
;   P0  constant velocity      pos += v            -> distance is linear in frames
;   P1  constant acceleration  vel += g; pos += vel -> distance is quadratic in frames
;
; The frame counter and both positions are parked in RAM so the harness can read them without
; looking at the picture (the picture is there to make the ROM legible, not to be measured).
;
;   $80 frame counter low   $81 frame counter high
;   $82 P0 pos low (sub)    $83 P0 pos high (pixels)
;   $84 P1 pos low          $85 P1 pos high
;   $86 P1 vel low          $87 P1 vel high
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
HMCLR   = $2B

VEL     = 96               ; P0: 0.375 px/frame
GRAV    = 6                ; P1: 0.0234 px/frame^2

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

        ; --- frame counter ---
        inc $80
        bne NoCarry
        inc $81
NoCarry:

        ; --- P0: constant velocity ---
        clc
        lda $82
        adc #VEL
        sta $82
        lda $83
        adc #0
        sta $83

        ; --- P1: constant acceleration ---
        clc
        lda $86
        adc #GRAV
        sta $86
        lda $87
        adc #0
        sta $87
        clc
        lda $84
        adc $86
        sta $84
        lda $85
        adc $87
        sta $85

        ldx #35
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis

        lda #2
        sta VBLANK
        ldx #28
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
