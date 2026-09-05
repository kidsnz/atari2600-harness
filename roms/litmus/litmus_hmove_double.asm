; litmus_hmove_double — does a SECOND HMOVE strobe move the sprite again, and does the gap matter?
;
; `known-traps.md` warns that positioning each object with its own `sta HMOVE` "re-applies every
; current HMxx to ALL objects on every strobe" so the sprite "never settles" — measured here, in
; this engine, as a real +3px drift. `fundamentals-audit.md` marks the neighbouring question
; **⬜ double-strobe behavior unmeasured**.
;
; The AtariAge corpus says something that would make that warning an emulator artefact
; (`198577`, 2012): *"on real hardware a repeated HMOVE strobe only counts once (emulators appear to
; accumulate — which is wrong)"*.
;
; ★Both can be true if the GAP is what matters: a second strobe landing while the ripple is still
; running is not the same event as one landing tens of cycles later. Nobody had measured the gap,
; and the two claims were being compared as if they described the same experiment.
;
; So: same HMxx, same starting position, three treatments, and read where P0 ends up.
;
;	$80  x after ONE strobe                       (the baseline)
;	$81  x after TWO strobes back to back         (`sta HMOVE / sta HMOVE`)
;	$82  x after TWO strobes 24 cycles apart
;	$83  x after TWO strobes with HMCLR between   (the control: the second must do nothing)
;
; ★★Reading it: if $81 == $80 and $82 != $80, the gap is the variable and BOTH sources are right.
; If $81 != $80 the accumulation happens even back to back. If $82 == $80 too, then this engine
; does not accumulate at all and `known-traps.md:159` is describing something else.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
COLUP0  = $06
GRP0    = $1B
RESP0   = $10
HMP0    = $20
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

Frame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        lda #$FF
        sta GRP0            ; ★give P0 something to draw: the DRAWN left edge is the measurement
        lda #$0E
        sta COLUP0

        ; ===== 1) one strobe =====================================================
        sta WSYNC
        sta RESP0           ; coarse position, same every time
        sta WSYNC
        lda #$70            ; HMP0 = -1 (upper nibble 7 = +7 in two's complement bias)
        sta HMP0
        sta WSYNC
        sta HMOVE
        sta WSYNC
        sta WSYNC           ; let the ripple finish; the test reads P0's position here

        ; ===== 2) two strobes back to back ========================================
        sta WSYNC
        sta RESP0
        sta WSYNC
        lda #$70
        sta HMP0
        sta WSYNC
        sta HMOVE
        sta HMOVE           ; ★immediately after — the ripple is still running
        sta WSYNC
        sta WSYNC

        ; ===== 3) two strobes 24 cycles apart =====================================
        sta WSYNC
        sta RESP0
        sta WSYNC
        lda #$70
        sta HMP0
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; 24 cycles
        sta HMOVE
        sta WSYNC
        sta WSYNC

        ; ===== 4) two strobes with HMCLR between (control) ========================
        sta WSYNC
        sta RESP0
        sta WSYNC
        lda #$70
        sta HMP0
        sta WSYNC
        sta HMOVE
        ds 12, $EA
        sta HMCLR           ; clears every HMxx — the second strobe must do nothing
        sta HMOVE
        sta WSYNC
        sta WSYNC

        ldx #170
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
