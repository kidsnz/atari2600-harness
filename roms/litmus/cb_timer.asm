; cb_timer — VV-2 S1 litmus: VBLANK timed by the RIOT timer (no WSYNC in the
; blank interval), then a clean visible kernel. The timer-wait region is
; display-OFF (VBLANK on) with no display store inside it, so VV-2 must SKIP it
; (blank) rather than flag it as a 1-line overrun OR an unbounded loop. The
; visible kernel is within budget, so the ROM must CERTIFY. Self-contained.

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
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: timed by the RIOT timer (display off, no WSYNC inside) ---
        lda #2
        sta VBLANK
        lda #43
        sta TIM64T
        sta WSYNC           ; region boundary: VBLANK setup done
TWait:  lda INTIM
        bne TWait           ; busy-wait, display OFF, no display store -> SKIP region
        sta WSYNC           ; region boundary: timer done

        lda #0
        sta VBLANK

; --- Visible: 192 clean lines ---
        lda #$1E
        sta COLUBK
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis

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
