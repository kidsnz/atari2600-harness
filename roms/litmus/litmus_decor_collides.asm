; litmus_decor_collides — an object placed only as DECORATION still sets the collision latch.
;
; The TIA's collision registers are geometry, not intent. Two enabled objects that share a
; colour clock set the bit whether the program thinks of them as a bullet, an explosion, a
; shadow, or a piece of the scenery. Every "reuse one object per region" technique in this
; repository borrows an object for a second job, and the borrowed object keeps colliding.
;
; The list has the bug report, from the author of the game it happened in. Piero Cavina, on
; INV 〔stella-list `199801/msg00197`〕:
;
;       This must happen because the program is checking missile/invaders collisions even
;       when the missile is used for the explosion.
;
; The symptom he was explaining: shoot one of two adjacent invaders in the top row and BOTH
; explode. The missile had been repurposed as the explosion graphic and was still being
; tested against the invaders.
;
; Two bands, identical in every register except the one under test:
;   band A  M1 ENABLED and overlapping P0   -> the latch must SET
;   band B  M1 DISABLED, same position      -> the latch must stay CLEAR   ★negative control
; Without band B a TIA that latched unconditionally would pass.
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
RESM1   = $13
GRP0    = $1B
ENAM1   = $1E
CXM1P   = $31           ; read address ($31 mirror: VBLANK is $01)
CXCLR   = $2C

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #$0E
        sta COLUP0
        lda #$46
        sta COLUP1
        lda #$00
        sta COLUBK
        lda #$30            ; missile 1 -> 8 clocks wide, so the overlap cannot be a near miss
        sta NUSIZ1
        lda #$07            ; ★player 0 -> QUAD width (32 clocks). The first version read no
        sta NUSIZ0          ; ★collision at normal width, and `DecomposeRow` showed P0 at 3..10 and
                            ; ★M1 at 25..32 — but the cause was its RESM1 = $12, which is RESM0: M1
                            ; ★was strobed only by the init clear loop's write to $13, so 25..32 was
                            ; ★where init left it. With RESM1 = $13
                            ; ★(2026-09-29) both strobes land in HBLANK, M1 at 2..9 and P0 at 4..35 on
                            ; ★row 50, and a normal-width P0 overlaps too. Quad width is kept; it is no
                            ; ★longer what makes the overlap.

Frame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldy #37
VB:     sta WSYNC
        dey
        bne VB
        lda #0
        sta VBLANK
        sta CXCLR

; ---- band A: the missile is ENABLED and sits on the player ----
        sta WSYNC
        sta RESP0           ; both objects strobed on the same line, 3 cycles apart
        sta RESM1
        lda #$02
        sta ENAM1           ; ★enabled -- "just a picture", as far as the program is concerned
        lda #$FF
        sta GRP0
        ldy #40
BandA:  sta WSYNC
        dey
        bne BandA
        lda CXM1P
        sta $80             ; ★what the latch says while the decoration is on screen

; ---- band B: same geometry, missile DISABLED ----
        sta CXCLR
        lda #$00
        sta ENAM1           ; ★the only change
        ldy #40
BandB:  sta WSYNC
        dey
        bne BandB
        lda CXM1P
        sta $81             ; ★the control

        lda #$00
        sta GRP0
        ldy #111
Rest:   sta WSYNC
        dey
        bne Rest
        lda #2
        sta VBLANK
        ldy #30
OS:     sta WSYNC
        dey
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
