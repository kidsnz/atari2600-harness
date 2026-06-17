; timerwrap_trap — VV-10 T-1 PLANTED trap: the G8 timer-wrap hazard. Uses the
; 1-cycle divider (TIM1T) with a small interval, then polls INTIM in a ~7-cycle
; loop. Because the timer decrements every CPU cycle but the loop only samples
; every ~7 cycles, the poll STEPS OVER the exact 0 and keeps reading values in
; the post-wrap (Expired) regime (255, 248, ...). That read-after-wrap is the
; bug WatchTimerWrap must flag: the wait runs hundreds of cycles too long.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
TIM1T   = $0294
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
        lda #20
        sta TIM1T           ; 1-cycle divider: INTIM drops every cycle
TWait:  lda INTIM           ; ~7-cycle loop overshoots 0 -> reads post-wrap values
        bne TWait
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
