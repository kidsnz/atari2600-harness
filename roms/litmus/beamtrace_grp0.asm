; beamtrace_grp0.asm — beamtrace self-test fixture.
; Writes GRP0 = $A5 exactly ONCE per frame, on the first visible scanline, at a
; visible beam clock (after a short delay past HBLANK). The self-test asserts the
; timeline shows that single write with the right value/kind, localized to its
; scanline, deterministically. No other GRP0 write exists in the frame.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
GRP0    = $1B
COLUBK  = $09

        org $F000
Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clear:  sta $00,x
        dex
        bne Clear

Main:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

        sta WSYNC           ; first visible line begins
        ldx #8
Delay:  dex                 ; burn ~40 cycles to push the write into the visible region
        bne Delay
        lda #$A5
        sta GRP0            ; <-- the marker: GRP0=$A5, once, at a visible clock

        ldx #191
Visible:
        sta WSYNC
        dex
        bne Visible

        lda #2
        sta VBLANK
        ldx #30
Over:   sta WSYNC
        dex
        bne Over
        jmp Main

        org $FFFC
        .word Reset
        .word Reset
