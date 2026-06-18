; lint_r2_hmxx_nohmove.asm — timinglint R2 self-test (POSITIVE).
; Stages a PROVABLY NON-ZERO fine motion (lda #$70 -> HMP0) but never strobes
; HMOVE, so the staged motion is never applied. timinglint must emit rule
; "hmxx-without-hmove". The value-awareness matters: a `lda #0; sta HMP0` clear
; (proven 0) must NOT warn — that negative is covered by the real dyn_multisprite
; kernel (zero warnings) in the corpus sweep.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
HMP0    = $20

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

        lda #$70            ; <-- non-zero fine motion ...
        sta HMP0            ; ... staged, but HMOVE is never strobed (R2)

        ldx #37
VBlank: sta WSYNC
        dex
        bne VBlank
        sta VBLANK

        ldx #192
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
