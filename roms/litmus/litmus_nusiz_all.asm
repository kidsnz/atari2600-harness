; litmus_nusiz_all — all eight NUSIZ copy/size modes, on BOTH players.
;
; Chosen by measurement: `statecov` over the whole 31-ROM corpus reported
; nusiz0_copies at 4/8 (modes 1, 2, 4 and 7 never appeared) and nusiz1_copies at
; 2/8 — P1 was almost never given copies or a size at all. Every one of those is
; a mode the positioning maths, the object attribution and the width tables all
; claim to handle, and none had ever been run.
;
; Eight bands of 24 visible lines, band n running NUSIZ0 = NUSIZ1 = n:
;
;   0 one copy          1 two copies close     2 two copies medium
;   3 three close       4 two copies wide      5 double width
;   6 three medium      7 quad width
;
; Both players carry $FF, so each band's run pattern IS the mode: the number of
; runs gives the copy count and each run's length gives the width. Nothing needs
; to be inferred from a formula, which is the point — a wrong width table shows
; up as a wrong run length in `DecomposeRow`, directly.
;
; The two players sit at different X so their copies cannot be confused, and the
; NUSIZ writes happen during the band's first HBLANK so no copy is half-drawn.
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
COLUBK = $09
CTRLPF = $0A
RESP0  = $10
RESP1  = $11
GRP0   = $1B
GRP1   = $1C

band    = $80
lineCnt = $81

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

    lda #$0E
    sta COLUP0
    lda #$46
    sta COLUP1
    lda #0
    sta COLUBK
    sta CTRLPF

    ; Place the players once, far apart, so even the quad-width mode's copies
    ; stay distinguishable from the other player's.
    sta WSYNC
    ldx #5
d0: dex
    bne d0
    sta RESP0
    sta WSYNC
    ldx #17
d1: dex
    bne d1
    sta RESP1

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

BandTop:
    sta WSYNC               ; set the mode in HBLANK, never mid-copy
    lda band
    sta NUSIZ0
    sta NUSIZ1
    lda #$FF
    sta GRP0
    sta GRP1
    lda #23                 ; 23 more lines; the mode-setting line is the 24th
    sta lineCnt

BandLine:
    sta WSYNC
    dec lineCnt
    bne BandLine

    inc band
    lda band
    cmp #8
    bne BandTop

    lda #0
    sta GRP0
    sta GRP1
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
