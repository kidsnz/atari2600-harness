; cb_divloop.asm — VV-14 2B self-test ROM (divide-by-15 / sbc-counter loop).
; One visible line runs the classic coarse-positioning loop: A is reduced by a
; constant until borrow (`sec; sbc #15; bcs`). The static prover must bound the
; iteration count from A's PROVEN range (here A=#90 is an immediate, so the range
; is exact) and certify the region; before 2B this was "loop bound unknown".
; A tight budget must flip it (proving the loop's cycles are actually counted).
; Self-contained (no include).

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

Main:
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
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

; --- Visible top: 95 normal lines ---
        ldx #95
Top:
        sta WSYNC
        dex
        bne Top

; --- the line under test: a divide-by-15 coarse-position loop ---
        sta WSYNC           ; opens the region
        lda #90             ; A in a known range [90,90]
        sec
DWait:
        sbc #15             ; 2 — coarse 15px step
        bcs DWait           ; 2/3 — loop until borrow  (<= 90/15+1 = 7 iterations)
        sta COLUBK

; --- Visible bottom: 95 normal lines ---
        ldx #95
Bottom:
        sta WSYNC
        dex
        bne Bottom

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
