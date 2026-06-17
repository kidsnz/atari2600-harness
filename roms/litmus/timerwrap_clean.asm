; timerwrap_clean — VV-10 T-1 CLEAN twin: a correct RIOT-timer VBLANK wait.
; Sets TIM64T and polls INTIM until it reaches 0, exiting the loop BEFORE the
; timer underflows/wraps. The divider (64) is far larger than the ~5-cycle poll
; loop, so the loop never misses 0. WatchTimerWrap must report NO hit: the
; program never reads INTIM while the timer is in its post-wrap (Expired) state.
; (The timer does wrap later, during the visible kernel, but nothing reads INTIM
;  then — which is exactly why "did it ever wrap" is the wrong test.)

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
TIM64T  = $0296
INTIM   = $0284

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
        lda #43
        sta TIM64T
TWait:  lda INTIM
        bne TWait           ; exits at INTIM==0, BEFORE the wrap (divider 64 >> loop)
        sta WSYNC
        lda #0
        sta VBLANK

        lda #$1E
        sta COLUBK
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
