; litmus_paint_as_background — is "painted the background colour" the same as "not drawn"?
;
; Ruffin Bailey, 1998, on a game he was playing:
;   "My question is why can I see a silhouetted tank over on the far right side of the ground
;    in one player games?"
; Piero Cavina, who wrote it:
;   "Hmm.. it might be the second player's sprite, which is always there, but painted in black
;    in 1-player games.  But I don't see it here.."
;   〔stella-list 199801/msg00038 and msg00045〕
;
; The author could not reproduce what a third party saw. That is the signature of something that
; depends on the display rather than on the program, and it is exactly the direction this harness
; is blind in.
;
; What CAN be settled here is what the program does, and whether the instruments can see it:
;
;   band A   P1 drawn in a visible colour       -- the control: the object exists and shows
;   band B   P1 drawn in COLUBK's colour        -- the case in question
;   band C   P1 not drawn at all (GRP1 = 0)     -- the control for "actually absent"
;
; B and C are pixel-identical. Whether anything can tell them apart is the measurement.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP1   = $11
GRP1    = $1C
HMCLR   = $2B

BG      = $84       ; the background colour used throughout

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
        lda #BG
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
        sta WSYNC
        ldx #12
Pa:     dex
        bne Pa
        sta RESP1
        ldx #30
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ---- band A: a visible player ----
        lda #$0E
        sta COLUP1
        lda #$FF
        sta GRP1
        ldy #20
Arow:   sta WSYNC
        dey
        bne Arow

        ; ---- band B: the same player, painted the background colour ----
        lda #BG
        sta COLUP1
        ldy #20
Brow:   sta WSYNC
        dey
        bne Brow

        ; ---- band C: no player at all ----
        lda #0
        sta GRP1
        ldy #20
Crow:   sta WSYNC
        dey
        bne Crow

        ldx #120
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
