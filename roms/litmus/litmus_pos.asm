; litmus_pos.asm — horizontal-position litmus test ROM (verification of gap B)
; Positions player0 by "strobe RESP0 N CPU cycles after the WSYNC sync point".
; The delay units live in RAM $80, poke-able from outside -> N can be swept without reassembling.
;
; Coarse-adjust loop: 1 iteration = SBC(2) + BCS(3) = 5 CPU cycles = 15 color clocks = 15px.
; So incrementing $80 by 1 should move ResetPixel by 15px. Measured with the harness's read_tia.
; HMOVE is not used (HMCLR zeroes the motion registers). HMOVE verification of fine adjust (±1px) is the next stage.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
HMCLR   = $2B
GRP0    = $1B

DELAY   = $80           ; coarse-adjust loop count (rewritten via poke)

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

        lda #6
        sta DELAY       ; initial delay units (overwritten later via poke)
        lda #$0E
        sta COLUP0      ; player0 white
        lda #$FF
        sta GRP0        ; player0 all pixels lit (8px wide)
        lda #0
        sta NUSIZ0      ; standard size, 1 copy

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

; --- VBLANK: 37 lines. Position player0 on the last line ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        ; ---- positioning kernel ----
        ; WSYNC aligns the beam to the start of the next line (HBLANK start)
        sta WSYNC               ; sync point
        sta HMCLR               ; motion registers to 0 (3)
        lda DELAY               ; fetch delay units (3, zp)
        sec                     ; set carry (2)
DelayLoop:
        sbc #1                  ; 2
        bcs DelayLoop           ; 3 (taken, same page) -> 5 cycles per iteration
        sta RESP0               ; player0 position-latch strobe (3)
        lda #0
        sta VBLANK              ; into visible

; --- Visible: 192 lines ---
        ldx #192
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
