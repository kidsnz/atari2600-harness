; beamrace_clean.asm — beamrace NEGATIVE fixture (P0 reaches the beam in time).
; P0 is positioned at the left, then over a visible band GRP0 is written during
; HBLANK every line (beam clock < 0 ≤ P0.X). Every update beats the beam, so a
; no_beam_race check over the band passes and every event is BeforeBeam.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
RESP0   = $10
GRP0    = $1B

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
        lda #$0E
        sta COLUP0          ; make P0 visible

        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        sta WSYNC           ; last VBLANK line: position P0
        sta RESP0           ; P0 near the left edge
        lda #0
        sta VBLANK

        ldx #20             ; visible band: update P0 in HBLANK every line
Band:
        sta WSYNC
        lda #$FF
        sta GRP0            ; lands ~clk -53 (HBLANK) => before the beam
        dex
        bne Band

        ldx #172
Rest:   sta WSYNC
        dex
        bne Rest

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
