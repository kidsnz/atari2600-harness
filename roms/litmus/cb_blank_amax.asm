; cb_blank_amax.asm — VV-2b self-test: a divide-by-15 coarse-position loop that
; runs in VBLANK (a "blank" region) over a RAM accumulator of UNKNOWN range.
; The accumulator posX ($80) is seeded from a read-only TIA input, so its proven
; range is Top and the divide loop cannot be bounded from the abstract state.
; The `@amax N` annotation on the region-opening WSYNC declares the accumulator's
; proven upper bound (N=145), so determineBound bounds the loop, the blank region
; is proven <=76, and roll_free flips to TRUE. It also proves ①: the blank region
; is no longer hidden as worst=0 — it appears in blank_lines with a real worst.
; Pairs with cb_blank_noamax (identical minus the annotation). No include.

        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C           ; read-only TIA input => unknown range to the prover

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
; --- VSYNC: 3 lines (display off through VSYNC+VBLANK) ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
; --- seed posX with an UNKNOWN value => accumulator range Top ---
        lda INPT4
        sta $80
; --- the blank region under test: a divide-by-15 loop over posX ---
        sta WSYNC           ; @amax 145  -- opens a VBLANK region; posX proven <=145
        lda $80
        sec
DW:     sbc #15
        bcs DW
        sta COLUBK
; --- pad the rest of VBLANK ---
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
; --- visible: 192 lines ---
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
; --- overscan: 30 lines ---
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
