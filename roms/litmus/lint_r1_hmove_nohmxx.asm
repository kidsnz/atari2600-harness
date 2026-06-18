; lint_r1_hmove_nohmxx.asm — timinglint R1 self-test (POSITIVE).
; Strobes HMOVE every visible line but NEVER writes any HMP0/HMP1/HMM0/HMM1/HMBL,
; so the fine motion is always 0 — the HMOVE is a no-op. timinglint must emit
; rule "hmove-without-hmxx". (R2/R3 must stay silent: no HMxx exists.)

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
HMOVE   = $2A

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
        sta VBLANK

        ldx #192
Visible:
        sta WSYNC
        sta HMOVE           ; <-- HMOVE with no HMxx ever staged (R1)
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
