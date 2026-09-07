; litmus_swacnt_delay — is the Stella Programmer's Guide's 400 microseconds modelled?
;
; The guide says, quoted on the list in 2001 by Nicolás Olhaberry:
;   "a delay of 400 microseconds is necessary between writing to this port and reading the TIA
;    input ports"  〔stella-list 200111/msg00192〕
;
; Chad Schell, who had a working 38.4 kbps serial link through the port, bounds it:
;   "If you only read the port, and thus don't change it's configuration, the 400 uS delay does
;    not apply."  〔200111/msg00194〕
;
; So the constraint is about CHANGING the direction, not about reading. The vendored engine says
; it does not implement it and does not know what happens if it is broken
; (`peripherals/controllers/keypad.go`):
;
;   "We're not emulating this here because as far as I can tell there is no need to.  More over,
;    I'm not sure what's supposed to happen if the 400ms is not adhered to.
;    !!TODO: Consider adding 400ms delay for SWACNT settings to take effect."
;
; This ROM makes the emulator's answer measurable instead of read from a comment. Two bands write
; the SAME direction byte to SWACNT and then read SWCHA -- one immediately, one after more than 400
; microseconds (400 us is about 477 CPU cycles at 1.19 MHz, so eight WSYNCs is comfortably past it).
; If the values agree, the delay is not modelled and a ROM that violates it is green here.
;
;   $80  SWCHA read immediately after writing SWACNT
;   $81  SWCHA read after the wait
;   $82  the direction byte that was written (so the test knows the write happened)
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHA   = $280
SWACNT  = $281

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
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK

        ; --- band A: change the direction and read at once ---
        lda #0
        sta SWACNT          ; all four port-A pins back to INPUT
        lda #$F0
        sta SWACNT          ; upper nibble becomes OUTPUT
        sta $82
        lda SWCHA           ; the very next instruction
        sta $80

        ; --- band B: the same change, then wait past 400 us ---
        lda #0
        sta SWACNT
        lda #$F0
        sta SWACNT
        ldx #8
Wait:   sta WSYNC           ; 8 x 76 cycles = 608 > 477
        dex
        bne Wait
        lda SWCHA
        sta $81

        ldx #28
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
        ldx #24
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
