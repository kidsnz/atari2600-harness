; cb_blank_noamax.asm — the RED counterpart of cb_blank_amax: identical, minus
; the `@amax N` annotation. The divide-by-15 loop in VBLANK reads posX ($80),
; whose range is Top (seeded from a TIA input), so with no declared bound the
; static prover reports that blank region UNBOUNDED (blank_unbounded) and
; roll_free=FALSE — honestly, not silently as worst=0. Proves that the blank
; ∀-accounting is real: an un-boundable blank region is surfaced, not hidden.
; No include.

        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C

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
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda INPT4
        sta $80
        sta WSYNC           ; (no @amax) -- blank region with an un-bounded divide loop
        lda $80
        sec
DW:     sbc #15
        bcs DW
        sta COLUBK
        ldx #36
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
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
