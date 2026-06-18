; beamrace_late.asm — beamrace POSITIVE fixture (P0 graphics miss the beam).
; Same left-positioned P0, but over the visible band GRP0 is written only after a
; long delay, landing near clock ~120 — far to the RIGHT of P0.X. Each line's P0
; therefore draws with the PREVIOUS value (a one-line lag). A no_beam_race check
; over the band must FAIL, and every band event is BeforeBeam == false.

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
        sta COLUP0

        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        sta WSYNC
        sta RESP0           ; P0 near the left edge
        lda #0
        sta VBLANK

        ldx #20             ; visible band: update P0 LATE every line
Band:
        sta WSYNC
        ldy #12
Delay:  dey                 ; ~59 cy => push the write deep into the visible line
        bne Delay
        lda #$FF
        sta GRP0            ; lands ~clk +120, right of P0 => too late (one-line lag)
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
