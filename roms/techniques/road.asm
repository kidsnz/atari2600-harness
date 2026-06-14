; road — pseudo-3D road (technique: "missile = line", racing primitive)
; ---------------------------------------------------------------------------
; The two missiles M0/M1 are the LEFT/RIGHT shoulders of the road, the ball BL
; is the dashed centre line. All three converge at a vanishing point near the
; horizon and the two shoulders WIDEN toward the bottom of the screen to fake
; perspective (an application of "a 1-px-wide object = a vertical line").
;
; Structure (each visible band = a constant width row group):
;   centre X (curveX) is fixed per frame (steerable via joystick L/R).
;   For band b (0=horizon .. NBAND-1=foreground), halfWidth = widthTab[b].
;     leftX  = curveX - halfWidth   (drawn with M0)
;     rightX = curveX + halfWidth   (drawn with M1)
;     centre = curveX               (drawn with BL, dashed by a row counter)
;   Every band we RESPx-position the three objects (PosObj divide-by-15 + HMOVE)
;   then hold them lit for that band's scanlines. Widening table is monotone so
;   the road visibly opens out toward the bottom.
;
; Determinism: curveX lives in RAM ($82). The kernel writes per-band positions
; into RAM mirrors leftMir/rightMir/ctrMir ($88/$89/$8A) at the LAST band so a
; scenario can assert the foreground shoulder spread without screen-scraping.
; ---------------------------------------------------------------------------
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
RESM0   = $12
RESM1   = $13
RESBL   = $14
ENAM0   = $1D
ENAM1   = $1E
ENABL   = $1F
HMM0    = $22
HMM1    = $23
HMBL    = $24
HMOVE   = $2A
HMCLR   = $2B
SWCHA   = $0280
TIM64T  = $0296
INTIM   = $0284

curveX  = $82       ; centre-line X (road follows this)
band    = $83       ; current band index
half    = $84       ; current half-width
leftX   = $85       ; computed left shoulder X
rightX  = $86       ; computed right shoulder X
dashCt  = $87       ; dashed centre-line row counter
leftMir = $88       ; mirror: foreground left shoulder X
rightMir= $89       ; mirror: foreground right shoulder X
ctrMir  = $8A       ; mirror: foreground centre X
posT    = $8B

NBAND   = 8         ; number of perspective bands
BANDROWS = 18       ; hold rows per band (+ 6 positioning WSYNC = 24 lines/band)
CENTER  = 78        ; default centre X

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #CENTER
        sta curveX
        lda #$0E            ; shoulders = white
        sta COLUP0
        sta COLUP1
        lda #$1C            ; centre line = bright yellow
        sta COLUPF
        lda #$04            ; CTRLPF: ball size = 4 clocks wide
        sta CTRLPF

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
        lda #44
        sta TIM64T

        ; ----- steer the road centre with the joystick -----
        lda SWCHA
        and #$80
        bne NoRt
        lda curveX
        cmp #120
        bcs NoRt
        inc curveX
NoRt:   lda SWCHA
        and #$40
        bne NoLt
        lda curveX
        cmp #36
        bcc NoLt
        dec curveX
NoLt:

        ; ----- advance dash animation -----
        inc dashCt

VBwait: lda INTIM
        bne VBwait
        sta WSYNC
        lda #0
        sta VBLANK

        ; enable all three road objects for the whole visible frame
        lda #2
        sta ENAM0
        sta ENAM1
        sta ENABL

        ; ===== per-band road kernel =====
        ldx #0              ; band index 0..NBAND-1
        stx band
BandLoop:
        ldx band
        lda WidthTab,x      ; half-width for this band
        sta half
        ; leftX = curveX - half
        lda curveX
        sec
        sbc half
        sta leftX
        ; rightX = curveX + half
        lda curveX
        clc
        adc half
        sta rightX

        ; position M0 at leftX, M1 at rightX, BL at curveX
        lda leftX
        ldy #0
        jsr PosObj          ; M0
        lda rightX
        ldy #1
        jsr PosObj          ; M1
        lda curveX
        ldy #2
        jsr PosObj          ; BL

        ; mirror the foreground (last) band into RAM for assertions
        lda band
        cmp #NBAND-1
        bne NoMir
        lda leftX
        sta leftMir
        lda rightX
        sta rightMir
        lda curveX
        sta ctrMir
NoMir:

        ; hold this band lit for BANDROWS scanlines, dashing the centre line
        ldy #BANDROWS
HoldLoop:
        sta WSYNC
        ; dash the centre line: ENABL on for some rows, off for others,
        ; cadence shifts with dashCt so it animates (scrolls down)
        tya
        clc
        adc dashCt
        and #4              ; 4-on / 4-off vertical dash
        sta ENABL
        dey
        bne HoldLoop

        inc band
        lda band
        cmp #NBAND
        bne BandLoop

        ; ----- overscan -----
        lda #2
        sta VBLANK
        lda #0
        sta ENAM0
        sta ENAM1
        sta ENABL
        ldx #23
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ===== PosObj: A = target X, Y = 0(M0)/1(M1)/2(BL). divide-by-15 + HMOVE =====
; RESM0=$12, RESM1=$13, RESBL=$14 are consecutive -> index with Y.
; HMM0=$22, HMM1=$23, HMBL=$24 likewise.
        align 16
PosObj: sta posT
        sta WSYNC
        sec
PDiv:   sbc #15
        bcs PDiv
        eor #7              ; fine-adjust nibble (calibrate with read_tia)
        asl
        asl
        asl
        asl
        sta HMM0,y
        sta RESM0,y
        sta WSYNC
        sta HMOVE
        rts

; widening table: half-width per band, horizon (narrow) -> foreground (wide).
; monotone increasing so the road opens out toward the bottom.
WidthTab:
        byte 4              ; band 0 (horizon)
        byte 8
        byte 13
        byte 18
        byte 24
        byte 30
        byte 37
        byte 44             ; band 7 (foreground)

        org $FFFC
        .word Start
        .word Start
