; cb_andloop.asm — VV-2 3A self-test ROM (AND-mask value range).
; A divide loop is fed by an UNKNOWN value (indexed load => the prover cannot
; know it) that is then masked with `and #$3F`. Only by modeling AND (result in
; [0,63]) can the prover bound the loop (<= 63/15+1 = 5 iterations) and certify
; the region. Without the AND-range model the source stays Top => unbounded.
; A tight budget must flip it (the loop's cycles are counted). Self-contained.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
scratch = $90

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

; --- the line under test: divide loop fed by a masked UNKNOWN value ---
        sta WSYNC           ; opens the region
        ldx #0
        lda scratch,x       ; indexed => value unknown to the prover (Top)
        and #$3F            ; 3A: mask => A in [0,63] regardless of source
        sec
AWait:
        sbc #15             ; 2
        bcs AWait           ; 2/3 — <= 5 iterations once A is known <= 63
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
