; cb_pushsafe — the TWIN of cb_pushdisplay: the same kernel with SP left at the top
; of the stack, so the push lands at $01FF (ordinary stack RAM) and cannot reach the
; display.
;
; It exists so the reclassification in cb_pushdisplay is attributable. The two ROMs
; differ by ONE immediate operand — the value loaded into X before TXS — and nothing
; else, so "that region became visible" can only be the SP range. This one's region
; must stay BLANK.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

STACKTOP = $FF          ; <- the whole difference from cb_pushdisplay: $01FF = stack RAM

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

; --- Visible: 192 lines ---
        ldx #192
Vis:    sta WSYNC
        stx COLUBK
        dex
        bne Vis

; --- Overscan: 30 lines, the last of which carries the push ---
        lda #2
        sta VBLANK
        ldx #28
OS:     sta WSYNC
        dex
        bne OS

        sta WSYNC       ; opens the region under test: VBLANK on, no display stores
        ldx #STACKTOP
        txs             ; SP is now provably {STACKTOP}
        lda #2
        pha             ; $01FF — ordinary stack RAM, provably not the display
        ldx #$FF
        txs             ; hand the stack back before the next frame
        sta WSYNC       ; CLOSES it here. Without this the region ran on through
                        ; `jmp Main` into the next frame's `sta VSYNC`, which is a
                        ; display store, so it was classified visible in BOTH twins
                        ; and the fixture proved nothing (measured before shipping).
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
