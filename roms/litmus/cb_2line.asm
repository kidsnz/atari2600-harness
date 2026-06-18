; cb_2line — VV-2 green-ification litmus: a legitimate 2-LINE kernel. Each visible
; "row" spans TWO scanlines with a single WSYNC at its top, so the WSYNC-to-WSYNC
; region does ~2 lines' worth of CPU work (~150cy). That is correct for a 2-line
; design, but the default 1-line budget (76) would flag it. The `@lines 2` note on
; the opening WSYNC tells the prover this region's budget is 2*76=152, so it
; CERTIFIES. Strip the note (cb_2line_noann) and the same region is over budget.

        processor 6502
VSYNC  = $00
VBLANK = $01
WSYNC  = $02
COLUBK = $09

        org $F000
Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
Main:
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
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; visible: 96 rows x 2 lines = 192 lines
        ldy #96
VRow:
        sta WSYNC          ; @lines 2
        ldx #24
DL:     dex
        bne DL             ; ~24*5 = 120cy of work (2nd scanline, no WSYNC)
        nop
        nop
        dey
        bne VRow

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Main
        org $FFFC
        .word Reset
        .word Reset
