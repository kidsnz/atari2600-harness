; cb_deadpred_live — the TWIN of cb_deadpred, identical except that the dead
; predecessor is gone: the delay loop is entered straight from `ldy #4`.
;
; It exists so the refusal in cb_deadpred is attributable. Without it, "this region
; is unbounded" only shows that SOMETHING about the region defeated the prover; with
; it, the two ROMs differ by one pruned branch edge and nothing else, and this one
; must come back BOUNDED.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

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
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible top: 96 striped lines. Each line runs a WSYNC-FREE delay loop whose
;     header is entered over a DEAD initialiser, so the dead instruction and the
;     header sit in the SAME WSYNC-to-WSYNC region (the predecessor scan is
;     per-region: a first attempt put the dead code in the region above and the
;     guard never saw it). The dead instruction is the NOT-TAKEN edge of a branch
;     whose condition is statically known, not code hopped over by a jmp: measured,
;     a jmp'd-over instruction is never decoded at all, so it never becomes a node
;     and the guard cannot see it either. A pruned branch edge IS decoded, and the
;     abstract interpreter marks its state invalid (S5 pruning) -- which is exactly
;     the "predecessor we know nothing about" the guard is written for.
        ldx #96
Top:    sta WSYNC
        stx COLUBK
        ldy #4
Delay:  dey
        bne Delay

        dex
        bne Top

; --- Visible bottom: 96 striped lines (ordinary, for contrast) ---
        ldx #96
Bot:    sta WSYNC
        stx COLUBK
        dex
        bne Bot

; --- Overscan: 30 lines ---
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
