; litmus_objsizes — every missile and ball WIDTH, and the ball's vertical delay.
;
; Chosen by measurement, not by taste: `statecov` over the whole 31-ROM corpus
; reported missile0_size and missile1_size at 2/4 (only the 1px and 8px widths
; ever appeared), ball_size at 2/4, and vdelbl never set at all. Those are widths
; the tools claim to handle and no test had ever exercised, which is the shape a
; capability gap has right before something quietly gets it wrong.
;
; Layout: four bands down the visible area. Band n draws M0, M1 and BL at width
; 2^n (1, 2, 4, 8 colour clocks) at fixed, well-separated X positions, so the
; width is readable straight off `read_row` / `DecomposeRow` as a run length —
; no arithmetic, and a wrong NUSIZ/CTRLPF encoding shows up as the wrong run.
;
; VDELBL is asserted for the lower half of the frame. The delay only bites when
; ENABL is written on alternating lines, so the ball is toggled per line there:
; with the delay off that draws every other line, with it on it draws the line
; after each write. The two halves therefore differ, which is what makes the bit
; observable at all rather than merely set.
;
; 262 lines: 3 VSYNC + 37 VBLANK + 192 visible + 30 overscan.
    processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
NUSIZ0 = $04
NUSIZ1 = $05
COLUP0 = $06
COLUP1 = $07
COLUPF = $08
COLUBK = $09
CTRLPF = $0A
RESM0  = $12
RESM1  = $13
RESBL  = $14
ENAM0  = $1D
ENAM1  = $1E
ENABL  = $1F
VDELBL = $27

band    = $80               ; 0..3, which width band we are in
lineCnt = $81               ; lines left in this band
toggle  = $82               ; alternating bit for the VDELBL half

    org $F000

Reset:
    sei
    cld
    ldx #$FF
    txs
    lda #0
Clear:
    sta $00,x
    dex
    bne Clear

    lda #$0E                ; three distinguishable colours: M0 white,
    sta COLUP0              ; M1 mid, BL playfield colour
    lda #$46
    sta COLUP1
    lda #$96
    sta COLUPF
    lda #0
    sta COLUBK

    ; Position the three objects once, well apart, so a width change is the only
    ; thing that can alter a run. Coarse placement by delay is enough here — the
    ; test reads the run LENGTH, never the absolute X.
    sta WSYNC
    ldx #6
d0: dex
    bne d0
    sta RESM0               ; leftish
    sta WSYNC
    ldx #13
d1: dex
    bne d1
    sta RESM1               ; middle
    sta WSYNC
    ldx #19
d2: dex
    bne d2
    sta RESBL               ; rightish

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
    ldx #37
VB: sta WSYNC
    dex
    bne VB

    lda #0
    sta VBLANK
    sta band
    sta VDELBL
    lda #2                  ; all three on for the first two bands
    sta ENAM0
    sta ENAM1
    sta ENABL

BandTop:
    ; NUSIZ missile width lives in bits 4-5; the ball's is CTRLPF bits 4-5.
    ; band 0->%00 (1px), 1->%01 (2px), 2->%10 (4px), 3->%11 (8px).
    lda band
    asl
    asl
    asl
    asl
    sta NUSIZ0
    sta NUSIZ1
    sta CTRLPF              ; reflect off (D0=0); only the size bits are set
    lda #48
    sta lineCnt

BandLine:
    sta WSYNC
    ldx band
    cpx #2
    bcc NoDelay             ; bands 0-1: plain, ball drawn every line
    lda #1
    sta VDELBL              ; bands 2-3: delay on, ENABL toggled per line
    lda toggle
    eor #2
    sta toggle
    sta ENABL
NoDelay:
    dec lineCnt
    bne BandLine

    inc band
    lda band
    cmp #4
    bne BandTop

    lda #0
    sta ENAM0
    sta ENAM1
    sta ENABL
    sta VDELBL
    lda #2
    sta VBLANK
    ldx #30
OS: sta WSYNC
    dex
    bne OS
    jmp Frame

    org $FFFA
    .word Reset
    .word Reset
    .word Reset
