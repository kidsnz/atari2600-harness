; litmus_overrun.asm — ROM for verifying the per-scanline budget guard (B-3)
; Purpose: deliberately construct the situation "a visible line overruns the 76 CPU cycle budget and eats
;   into the next physical scanline", and confirm numerically that assert_line_budget catches that line
;   with over=true.
; Mechanism: the same well-formed frame structure as smoke, but with exactly one "heavy line" planted near
;   the middle of the visible area.
;   Heavy line = spin a busy loop of ~100 CPU cycles before WSYNC → work > 76cy → that logical line
;   consumes 2 physical scanlines (WSYNC eats into the next line) = a roll cause.
; No includes; self-contained.

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
ClearMem:
        sta $00,x
        dex
        bne ClearMem

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        lda #0
        sta VBLANK

; --- Visible: top half, 96 lines (normal) ---
        lda #$1E
        sta COLUBK
        ldx #96
TopLoop:
        sta WSYNC
        dex
        bne TopLoop

; --- one heavy line: waste ~100cy before WSYNC (overruns the budget of 76) ---
        ldy #20
Burn:
        dey             ; 2cy
        bne Burn        ; 3cy (when taken) → about 20*5 = 100cy > 76 = this logical line consumes 2 physical lines
        sta WSYNC

; --- Visible: bottom half, 95 lines (normal) ---
        ldx #95
BotLoop:
        sta WSYNC
        dex
        bne BotLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
