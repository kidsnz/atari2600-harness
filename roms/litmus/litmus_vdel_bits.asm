; litmus_vdel_bits — does VDELPx decode bits 1..7, or only bit 0?
;
; Thomas Jentzsch wrote a two-line kernel in 2002 whose comment claims it does not:
;
;       LDA P0_YFullres
;       EOR #1
;       STA VDELP0          ;don't care for the bits 1..7, VDEL ignores them
;
;   〔stella-list 200204/msg00067, 2002-04〕
;
; two-line-kernel.md tells authors to write `VDELP0 = y & 1`. If Jentzsch is right the
; mask is free to drop: one AND less per object per frame.
;
; Four bands, one player, one delayed write each. The reading is whether P0 shows the
; NEW graphic (VDEL off) or the OLD one (VDEL on):
;
;   A   VDELP0 = $00   bit0 clear, no other bits      -> NEW   (control: off is off)
;   B   VDELP0 = $01   bit0 set                       -> OLD   (control: on is on)
;   C   VDELP0 = $02   bit0 CLEAR, bit1 set           -> ?     if bits 1..7 are ignored, NEW
;   D   VDELP0 = $03   bit0 set, bit1 set             -> ?     if bits 1..7 are ignored, OLD
;
; C and D are the measurement; A and B are the negative controls that prove the band
; layout can express both answers at all. $FE and $FF widen it to the whole byte.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
VDELP0  = $25
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
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #$00
        sta COLUBK

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
        sta WSYNC
        ldx #8
Pa:     dex
        bne Pa
        sta RESP0
        ldx #30
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; Each band: park OLD := $00, then write NEW := $FF and read the result.
        ; With VDEL off the player shows $FF; with VDEL on it shows the parked $00.

        MAC vdelband        ; {1} = value written to VDELP0
        lda #0
        sta VDELP0          ; VDEL off while we park the old copy
        sta GRP0            ; new := 0
        sta GRP0            ; old := 0 (a second write shifts new into old)
        sta WSYNC
        lda #{1}
        sta VDELP0
        lda #$FF
        sta GRP0            ; new := $FF. Delayed => the player still draws $00.
        REPEAT 12
        sta WSYNC
        REPEND
        lda #0
        sta GRP0
        sta VDELP0
        sta WSYNC
        sta WSYNC           ; one blank line between bands
        ENDM

        vdelband $00        ; A
        vdelband $01        ; B
        vdelband $02        ; C
        vdelband $03        ; D
        vdelband $FE        ; E — every bit but bit 0
        vdelband $FF        ; F — every bit

        ldx #24
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
